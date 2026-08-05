package service

import (
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const enterpriseMemberRouteSnapshotAlgorithmVersion = "enterprise_member_route_lkg_v1"

type EnterpriseMemberRoutePlanSource string

const (
	EnterpriseMemberRoutePlanSourceLive          EnterpriseMemberRoutePlanSource = "live"
	EnterpriseMemberRoutePlanSourceLastKnownGood EnterpriseMemberRoutePlanSource = "last_known_good"
)

type EnterpriseMemberRouteIntentProfile string

const (
	EnterpriseMemberRouteIntentText     EnterpriseMemberRouteIntentProfile = "text"
	EnterpriseMemberRouteIntentImage    EnterpriseMemberRouteIntentProfile = "image"
	EnterpriseMemberRouteIntentVideo    EnterpriseMemberRouteIntentProfile = "video"
	EnterpriseMemberRouteIntentLive     EnterpriseMemberRouteIntentProfile = "live"
	EnterpriseMemberRouteIntentBatch    EnterpriseMemberRouteIntentProfile = "batch"
	EnterpriseMemberRouteIntentMessages EnterpriseMemberRouteIntentProfile = "messages"
)

// EnterpriseMemberRouteSnapshotKey identifies the exact stable eligibility
// envelope for a route snapshot. It intentionally excludes member/key identity,
// request bodies, credentials, scheduler capacity, and sticky state.
type EnterpriseMemberRouteSnapshotKey struct {
	PublicModel      string
	Endpoint         string
	InboundProtocol  ModelProtocol
	Intent           EnterpriseMemberRouteIntentProfile
	GroupID          int64
	Eligibility      RoutingEligibilityVersion
	AlgorithmVersion string
}

// EnterpriseMemberRouteSnapshotCandidate contains only stable qualification
// evidence. Runtime scheduling must still re-evaluate capacity, sticky,
// pricing, proxy, and account health after a snapshot hit.
type EnterpriseMemberRouteSnapshotCandidate struct {
	GroupID            int64
	ReasonCode         string
	PublicModel        string
	ChannelMappedModel string
	UpstreamModel      string
	InboundProtocol    ModelProtocol
	UpstreamProtocol   ModelProtocol
	DeliveryMode       ModelDeliveryMode
}

type EnterpriseMemberRouteSnapshotPlan struct {
	Source        EnterpriseMemberRoutePlanSource
	Key           EnterpriseMemberRouteSnapshotKey
	Candidates    []EnterpriseMemberRouteSnapshotCandidate
	CreatedAt     time.Time
	SnapshotAgeMs *int64
}

type EnterpriseMemberRouteSnapshotStore struct {
	mu       sync.RWMutex
	ttl      time.Duration
	clock    func() time.Time
	entries  map[string]enterpriseMemberRouteSnapshotEntry
	maxItems int
}

type enterpriseMemberRouteSnapshotEntry struct {
	key        EnterpriseMemberRouteSnapshotKey
	candidates []EnterpriseMemberRouteSnapshotCandidate
	createdAt  time.Time
	expiresAt  time.Time
}

func NewEnterpriseMemberRouteSnapshotStore(ttl time.Duration) *EnterpriseMemberRouteSnapshotStore {
	return NewEnterpriseMemberRouteSnapshotStoreWithClock(ttl, time.Now)
}

func NewEnterpriseMemberRouteSnapshotStoreWithClock(ttl time.Duration, clock func() time.Time) *EnterpriseMemberRouteSnapshotStore {
	if clock == nil {
		clock = time.Now
	}
	return &EnterpriseMemberRouteSnapshotStore{
		ttl:      ttl,
		clock:    clock,
		entries:  make(map[string]enterpriseMemberRouteSnapshotEntry),
		maxItems: 1024,
	}
}

func (s *EnterpriseMemberRouteSnapshotStore) Store(plan EnterpriseMemberRouteSnapshotPlan) bool {
	if s == nil || s.ttl <= 0 || plan.Source != EnterpriseMemberRoutePlanSourceLive {
		return false
	}
	key := normalizeEnterpriseMemberRouteSnapshotKey(plan.Key)
	if !enterpriseMemberRouteSnapshotKeyValid(key) {
		return false
	}
	candidates := normalizeEnterpriseMemberRouteSnapshotCandidates(plan.Candidates)
	// Snapshots are deliberately exact per group. Accepting a multi-group value
	// under one group's key would let a later lookup restore a different group
	// even though that group's generation was never part of the key.
	if len(candidates) != 1 || candidates[0].GroupID != key.GroupID {
		return false
	}
	now := s.clock()
	createdAt := plan.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	if createdAt.After(now) {
		return false
	}
	expiresAt := createdAt.Add(s.ttl)
	if !expiresAt.After(now) {
		return false
	}
	entry := enterpriseMemberRouteSnapshotEntry{
		key:        key,
		candidates: candidates,
		createdAt:  createdAt,
		expiresAt:  expiresAt,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[enterpriseMemberRouteSnapshotKeyString(key)] = entry
	s.evictExpiredLocked(now)
	s.evictOverflowLocked()
	return true
}

func (s *EnterpriseMemberRouteSnapshotStore) Restore(key EnterpriseMemberRouteSnapshotKey, authorizedGroupIDs []int64) (EnterpriseMemberRouteSnapshotPlan, bool) {
	if s == nil || s.ttl <= 0 {
		return EnterpriseMemberRouteSnapshotPlan{}, false
	}
	normalizedKey := normalizeEnterpriseMemberRouteSnapshotKey(key)
	if !enterpriseMemberRouteSnapshotKeyValid(normalizedKey) {
		return EnterpriseMemberRouteSnapshotPlan{}, false
	}
	now := s.clock()
	keyString := enterpriseMemberRouteSnapshotKeyString(normalizedKey)
	s.mu.RLock()
	entry, ok := s.entries[keyString]
	s.mu.RUnlock()
	if !ok || !entry.key.Eligibility.Equal(normalizedKey.Eligibility) || entry.createdAt.After(now) || !entry.expiresAt.After(now) {
		if ok {
			s.mu.Lock()
			if current, exists := s.entries[keyString]; exists && (current.createdAt.After(now) || !current.expiresAt.After(now)) {
				delete(s.entries, keyString)
			}
			s.mu.Unlock()
		}
		return EnterpriseMemberRouteSnapshotPlan{}, false
	}
	authorized := routeSnapshotAuthorizedGroupSet(authorizedGroupIDs)
	if len(authorized) == 0 {
		return EnterpriseMemberRouteSnapshotPlan{}, false
	}
	candidates := make([]EnterpriseMemberRouteSnapshotCandidate, 0, len(entry.candidates))
	for _, candidate := range entry.candidates {
		if _, ok := authorized[candidate.GroupID]; ok {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) == 0 {
		return EnterpriseMemberRouteSnapshotPlan{}, false
	}
	ageMs := now.Sub(entry.createdAt).Milliseconds()
	if ageMs < 0 {
		return EnterpriseMemberRouteSnapshotPlan{}, false
	}
	return EnterpriseMemberRouteSnapshotPlan{
		Source:        EnterpriseMemberRoutePlanSourceLastKnownGood,
		Key:           entry.key,
		Candidates:    candidates,
		CreatedAt:     entry.createdAt,
		SnapshotAgeMs: &ageMs,
	}, true
}

func (s *EnterpriseMemberRouteSnapshotStore) InvalidateScopes(scopes []RoutingEligibilityScope) int {
	if s == nil || len(scopes) == 0 {
		return 0
	}
	normalizedScopes := normalizeRoutingEligibilityScopes(scopes)
	if len(normalizedScopes) == 0 {
		return 0
	}
	scopeSet := make(map[RoutingEligibilityScope]struct{}, len(normalizedScopes))
	for _, scope := range normalizedScopes {
		scopeSet[scope] = struct{}{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for key, entry := range s.entries {
		if routeSnapshotEligibilityIntersects(entry.key.Eligibility, scopeSet) {
			delete(s.entries, key)
			removed++
		}
	}
	return removed
}

func (s *EnterpriseMemberRouteSnapshotStore) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func normalizeEnterpriseMemberRouteSnapshotKey(key EnterpriseMemberRouteSnapshotKey) EnterpriseMemberRouteSnapshotKey {
	key.PublicModel = normalizeEnterpriseMemberRouteSnapshotToken(key.PublicModel)
	key.Endpoint = normalizeEnterpriseMemberRouteSnapshotEndpoint(key.Endpoint)
	key.InboundProtocol = ModelProtocol(normalizeEnterpriseMemberRouteSnapshotToken(string(key.InboundProtocol)))
	key.Intent = EnterpriseMemberRouteIntentProfile(normalizeEnterpriseMemberRouteSnapshotToken(string(key.Intent)))
	key.AlgorithmVersion = strings.TrimSpace(key.AlgorithmVersion)
	if key.AlgorithmVersion == "" {
		key.AlgorithmVersion = enterpriseMemberRouteSnapshotAlgorithmVersion
	}
	key.Eligibility = NewRoutingEligibilityVersion(key.Eligibility.Items)
	return key
}

func enterpriseMemberRouteSnapshotKeyValid(key EnterpriseMemberRouteSnapshotKey) bool {
	return key.PublicModel != "" &&
		key.Endpoint != "" &&
		key.InboundProtocol != "" &&
		key.Intent != "" &&
		key.GroupID > 0 &&
		key.Eligibility.Digest != "" &&
		len(key.Eligibility.Items) > 0 &&
		enterpriseMemberRouteSnapshotEligibilityComplete(key.GroupID, key.Eligibility) &&
		key.AlgorithmVersion != ""
}

func enterpriseMemberRouteSnapshotEligibilityComplete(groupID int64, version RoutingEligibilityVersion) bool {
	if groupID <= 0 {
		return false
	}
	required := []RoutingEligibilityScope{
		{Type: RoutingEligibilityScopeChannel, ID: 0},
		{Type: RoutingEligibilityScopeAccount, ID: 0},
		{Type: RoutingEligibilityScopeProtocol, ID: 0},
		{Type: RoutingEligibilityScopeComposite, ID: 0},
		{Type: RoutingEligibilityScopeGroup, ID: groupID},
	}
	available := make(map[RoutingEligibilityScope]uint64, len(version.Items))
	for _, item := range version.Items {
		available[item.Scope] = item.Revision
	}
	for _, scope := range required {
		if available[scope] == 0 {
			return false
		}
	}
	return true
}

func enterpriseMemberRouteSnapshotKeyString(key EnterpriseMemberRouteSnapshotKey) string {
	return strings.Join([]string{
		key.PublicModel,
		key.Endpoint,
		string(key.InboundProtocol),
		string(key.Intent),
		strconv.FormatInt(key.GroupID, 10),
		key.Eligibility.Digest,
		key.AlgorithmVersion,
	}, "\x1f")
}

func normalizeEnterpriseMemberRouteSnapshotCandidates(candidates []EnterpriseMemberRouteSnapshotCandidate) []EnterpriseMemberRouteSnapshotCandidate {
	normalized := make([]EnterpriseMemberRouteSnapshotCandidate, 0, len(candidates))
	seen := make(map[int64]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate.GroupID <= 0 {
			continue
		}
		if _, exists := seen[candidate.GroupID]; exists {
			continue
		}
		seen[candidate.GroupID] = struct{}{}
		candidate.ReasonCode = normalizeEnterpriseMemberRouteSnapshotToken(candidate.ReasonCode)
		candidate.PublicModel = normalizeEnterpriseMemberRouteSnapshotToken(candidate.PublicModel)
		candidate.ChannelMappedModel = normalizeEnterpriseMemberRouteSnapshotToken(candidate.ChannelMappedModel)
		candidate.UpstreamModel = normalizeEnterpriseMemberRouteSnapshotToken(candidate.UpstreamModel)
		candidate.InboundProtocol = ModelProtocol(normalizeEnterpriseMemberRouteSnapshotToken(string(candidate.InboundProtocol)))
		candidate.UpstreamProtocol = ModelProtocol(normalizeEnterpriseMemberRouteSnapshotToken(string(candidate.UpstreamProtocol)))
		if candidate.ReasonCode != string(EnterpriseMemberRouteReasonEligible) ||
			candidate.PublicModel == "" ||
			candidate.UpstreamModel == "" ||
			candidate.InboundProtocol == "" ||
			candidate.UpstreamProtocol == "" ||
			(candidate.DeliveryMode != ModelDeliveryModeCompatibility && candidate.DeliveryMode != ModelDeliveryModeNative) {
			continue
		}
		normalized = append(normalized, candidate)
	}
	return normalized
}

func normalizeEnterpriseMemberRouteSnapshotToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeEnterpriseMemberRouteSnapshotEndpoint(endpoint string) string {
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	if endpoint == "" || strings.HasPrefix(endpoint, "/") {
		return endpoint
	}
	return "/" + endpoint
}

func routeSnapshotAuthorizedGroupSet(groupIDs []int64) map[int64]struct{} {
	authorized := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID > 0 {
			authorized[groupID] = struct{}{}
		}
	}
	return authorized
}

func routeSnapshotEligibilityIntersects(version RoutingEligibilityVersion, scopeSet map[RoutingEligibilityScope]struct{}) bool {
	for _, item := range version.Items {
		if _, ok := scopeSet[item.Scope]; ok {
			return true
		}
	}
	return false
}

func (s *EnterpriseMemberRouteSnapshotStore) evictExpiredLocked(now time.Time) {
	for key, entry := range s.entries {
		if !entry.expiresAt.After(now) {
			delete(s.entries, key)
		}
	}
}

func (s *EnterpriseMemberRouteSnapshotStore) evictOverflowLocked() {
	if s.maxItems <= 0 || len(s.entries) <= s.maxItems {
		return
	}
	type item struct {
		key       string
		expiresAt time.Time
	}
	items := make([]item, 0, len(s.entries))
	for key, entry := range s.entries {
		items = append(items, item{key: key, expiresAt: entry.expiresAt})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].expiresAt.Before(items[j].expiresAt)
	})
	for len(s.entries) > s.maxItems && len(items) > 0 {
		delete(s.entries, items[0].key)
		items = items[1:]
	}
}
