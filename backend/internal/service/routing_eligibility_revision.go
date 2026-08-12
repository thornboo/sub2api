package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// RoutingEligibilityScopeType names the stable configuration domain that can
// change enterprise-member route eligibility.
type RoutingEligibilityScopeType string

const (
	RoutingEligibilityScopeChannel   RoutingEligibilityScopeType = "channel"
	RoutingEligibilityScopeGroup     RoutingEligibilityScopeType = "group"
	RoutingEligibilityScopeAccount   RoutingEligibilityScopeType = "account"
	RoutingEligibilityScopeProtocol  RoutingEligibilityScopeType = "protocol"
	RoutingEligibilityScopeComposite RoutingEligibilityScopeType = "composite"
)

// RoutingEligibilityScope identifies one eligibility-affecting configuration
// scope. ID zero is reserved for future global scope use.
type RoutingEligibilityScope struct {
	Type RoutingEligibilityScopeType
	ID   int64
}

// RoutingEligibilityScopeRevision is the local mirror value observed for a
// scope at planning time.
type RoutingEligibilityScopeRevision struct {
	Scope    RoutingEligibilityScope
	Revision uint64
}

// RoutingEligibilityVersion is a deterministic request-plan version for the
// exact set of eligibility scopes read by the planner.
type RoutingEligibilityVersion struct {
	Digest string
	Items  []RoutingEligibilityScopeRevision
}

// RoutingEligibilityRevisionMirror is a process-local, high-availability mirror
// for eligibility configuration revisions. It is intentionally not a durable
// authority; database revision rows and cluster events must remain the source of
// truth for cross-instance propagation.
type RoutingEligibilityRevisionMirror struct {
	sequence  atomic.Uint64
	revisions sync.Map
}

func NewRoutingEligibilityRevisionMirror() *RoutingEligibilityRevisionMirror {
	return &RoutingEligibilityRevisionMirror{}
}

// NextLocalRevision allocates and applies a local monotonic revision for scope.
// It is useful for write paths after the durable configuration commit succeeds.
func (m *RoutingEligibilityRevisionMirror) NextLocalRevision(scope RoutingEligibilityScope) uint64 {
	if m == nil {
		return 0
	}
	normalized, ok := normalizeRoutingEligibilityScope(scope)
	if !ok {
		return 0
	}
	revision := m.sequence.Add(1)
	m.Apply(normalized, revision)
	return revision
}

// Apply records an observed revision when it is newer than the current mirror.
// Duplicate, stale, invalid, and zero revisions are ignored.
func (m *RoutingEligibilityRevisionMirror) Apply(scope RoutingEligibilityScope, revision uint64) bool {
	if m == nil || revision == 0 {
		return false
	}
	normalized, ok := normalizeRoutingEligibilityScope(scope)
	if !ok {
		return false
	}
	for {
		currentAny, loaded := m.revisions.Load(normalized)
		if loaded {
			current, ok := currentAny.(uint64)
			if !ok {
				m.revisions.Delete(normalized)
				continue
			}
			if revision <= current {
				return false
			}
			if m.revisions.CompareAndSwap(normalized, current, revision) {
				m.raiseSequence(revision)
				return true
			}
			continue
		}
		if _, loaded := m.revisions.LoadOrStore(normalized, revision); !loaded {
			m.raiseSequence(revision)
			return true
		}
	}
}

func (m *RoutingEligibilityRevisionMirror) raiseSequence(revision uint64) {
	for {
		current := m.sequence.Load()
		if current >= revision || m.sequence.CompareAndSwap(current, revision) {
			return
		}
	}
}

func (m *RoutingEligibilityRevisionMirror) Revision(scope RoutingEligibilityScope) uint64 {
	if m == nil {
		return 0
	}
	normalized, ok := normalizeRoutingEligibilityScope(scope)
	if !ok {
		return 0
	}
	value, ok := m.revisions.Load(normalized)
	if !ok {
		return 0
	}
	revision, ok := value.(uint64)
	if !ok {
		return 0
	}
	return revision
}

func (m *RoutingEligibilityRevisionMirror) VersionFor(scopes []RoutingEligibilityScope) RoutingEligibilityVersion {
	if m == nil {
		return RoutingEligibilityVersion{}
	}
	normalized := normalizeRoutingEligibilityScopes(scopes)
	items := make([]RoutingEligibilityScopeRevision, 0, len(normalized))
	for _, scope := range normalized {
		items = append(items, RoutingEligibilityScopeRevision{
			Scope:    scope,
			Revision: m.Revision(scope),
		})
	}
	return NewRoutingEligibilityVersion(items)
}

func NewRoutingEligibilityVersion(items []RoutingEligibilityScopeRevision) RoutingEligibilityVersion {
	normalized := normalizeRoutingEligibilityScopeRevisions(items)
	builder := strings.Builder{}
	for i, item := range normalized {
		if i > 0 {
			_ = builder.WriteByte('|')
		}
		_, _ = builder.WriteString(string(item.Scope.Type))
		_ = builder.WriteByte(':')
		_, _ = builder.WriteString(fmt.Sprintf("%d=%d", item.Scope.ID, item.Revision))
	}
	sum := sha256.Sum256([]byte(builder.String()))
	return RoutingEligibilityVersion{
		Digest: hex.EncodeToString(sum[:16]),
		Items:  normalized,
	}
}

func (v RoutingEligibilityVersion) String() string {
	return v.Digest
}

func (v RoutingEligibilityVersion) Equal(other RoutingEligibilityVersion) bool {
	return v.Digest == other.Digest && equalRoutingEligibilityScopeRevisions(v.Items, other.Items)
}

func normalizeRoutingEligibilityScope(scope RoutingEligibilityScope) (RoutingEligibilityScope, bool) {
	scope.Type = RoutingEligibilityScopeType(strings.ToLower(strings.TrimSpace(string(scope.Type))))
	if scope.ID < 0 {
		return RoutingEligibilityScope{}, false
	}
	switch scope.Type {
	case RoutingEligibilityScopeChannel,
		RoutingEligibilityScopeGroup,
		RoutingEligibilityScopeAccount,
		RoutingEligibilityScopeProtocol,
		RoutingEligibilityScopeComposite:
		return scope, true
	default:
		return RoutingEligibilityScope{}, false
	}
}

func normalizeRoutingEligibilityScopes(scopes []RoutingEligibilityScope) []RoutingEligibilityScope {
	seen := make(map[RoutingEligibilityScope]struct{}, len(scopes))
	normalized := make([]RoutingEligibilityScope, 0, len(scopes))
	for _, scope := range scopes {
		item, ok := normalizeRoutingEligibilityScope(scope)
		if !ok {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		normalized = append(normalized, item)
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Type != normalized[j].Type {
			return normalized[i].Type < normalized[j].Type
		}
		return normalized[i].ID < normalized[j].ID
	})
	return normalized
}

func normalizeRoutingEligibilityScopeRevisions(items []RoutingEligibilityScopeRevision) []RoutingEligibilityScopeRevision {
	byScope := make(map[RoutingEligibilityScope]uint64, len(items))
	for _, item := range items {
		scope, ok := normalizeRoutingEligibilityScope(item.Scope)
		if !ok {
			continue
		}
		if item.Revision > byScope[scope] {
			byScope[scope] = item.Revision
		}
	}
	normalized := make([]RoutingEligibilityScopeRevision, 0, len(byScope))
	for scope, revision := range byScope {
		normalized = append(normalized, RoutingEligibilityScopeRevision{Scope: scope, Revision: revision})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Scope.Type != normalized[j].Scope.Type {
			return normalized[i].Scope.Type < normalized[j].Scope.Type
		}
		return normalized[i].Scope.ID < normalized[j].Scope.ID
	})
	return normalized
}

func equalRoutingEligibilityScopeRevisions(a, b []RoutingEligibilityScopeRevision) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
