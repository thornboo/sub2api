package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRouteSnapshotOnlyStoresCompleteLivePlan(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))

	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLastKnownGood,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))
	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source: EnterpriseMemberRoutePlanSourceLive,
		Key:    key,
	}))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))
	require.Equal(t, 1, store.Len())
}

func TestRouteSnapshotFallbackCannotRefreshSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))

	now = now.Add(50 * time.Second)
	restored, ok := store.Restore(key, []int64{1})
	require.True(t, ok)
	require.False(t, store.Store(restored))

	now = now.Add(11 * time.Second)
	_, ok = store.Restore(key, []int64{1})
	require.False(t, ok)
}

func TestRouteSnapshotKeySeparatesModelEndpointIntentRevisionAndAlgorithm(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))

	for _, mutated := range []EnterpriseMemberRouteSnapshotKey{
		func() EnterpriseMemberRouteSnapshotKey {
			next := key
			next.PublicModel = "glm-5.2"
			return next
		}(),
		func() EnterpriseMemberRouteSnapshotKey {
			next := key
			next.Endpoint = "/v1/chat/completions"
			return next
		}(),
		func() EnterpriseMemberRouteSnapshotKey {
			next := key
			next.Intent = EnterpriseMemberRouteIntentImage
			return next
		}(),
		func() EnterpriseMemberRouteSnapshotKey {
			next := key
			next.Eligibility = routeSnapshotTestVersion(1, 11)
			return next
		}(),
		func() EnterpriseMemberRouteSnapshotKey {
			next := key
			next.AlgorithmVersion = "enterprise_member_route_lkg_v2"
			return next
		}(),
	} {
		_, ok := store.Restore(mutated, []int64{1})
		require.False(t, ok)
	}
	_, ok := store.Restore(key, []int64{1})
	require.True(t, ok)
}

func TestRouteSnapshotIntersectsCurrentAuthorizedGroups(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(3, routeSnapshotTestVersion(3, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(3),
	}))

	restored, ok := store.Restore(key, []int64{3, 1})
	require.True(t, ok)
	require.Equal(t, EnterpriseMemberRoutePlanSourceLastKnownGood, restored.Source)
	require.Equal(t, []int64{3}, routeSnapshotCandidateGroupIDs(restored.Candidates))
}

func TestRouteSnapshotNeverRestoresRevokedGroup(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(8, routeSnapshotTestVersion(8, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(8),
	}))

	_, ok := store.Restore(key, []int64{7})
	require.False(t, ok)
}

func TestRouteSnapshotRejectsCrossGroupOrIncompleteCandidates(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))

	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1, 2),
	}))
	incomplete := routeSnapshotTestCandidates(1)
	incomplete[0].UpstreamProtocol = ""
	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: incomplete,
	}))
}

func TestRouteSnapshotExpiresAtConfiguredTTL(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))

	now = now.Add(time.Minute)
	_, ok := store.Restore(key, []int64{1})
	require.False(t, ok)
}

func TestRouteSnapshotRestoreReportsBoundedNonNegativeAge(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))

	now = now.Add(1500 * time.Millisecond)
	restored, ok := store.Restore(key, []int64{1})
	require.True(t, ok)
	require.NotNil(t, restored.SnapshotAgeMs)
	require.Equal(t, int64(1500), *restored.SnapshotAgeMs)
	require.Less(t, *restored.SnapshotAgeMs, int64(time.Minute/time.Millisecond))
}

func TestRouteSnapshotRejectsFutureOrAlreadyExpiredCreatedAt(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))

	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
		CreatedAt:  now.Add(time.Millisecond),
	}))
	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
		CreatedAt:  now.Add(-time.Minute),
	}))
	require.Zero(t, store.Len())
}

func TestRouteSnapshotZeroTTLDisablesFallback(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(0, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))

	require.False(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))
	_, ok := store.Restore(key, []int64{1})
	require.False(t, ok)
}

func TestRouteSnapshotRevisionMismatchFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source:     EnterpriseMemberRoutePlanSourceLive,
		Key:        key,
		Candidates: routeSnapshotTestCandidates(1),
	}))

	nextRevision := key
	nextRevision.Eligibility = routeSnapshotTestVersion(1, 11)
	_, ok := store.Restore(nextRevision, []int64{1})
	require.False(t, ok)
}

func TestRouteSnapshotExplicitInvalidationRemovesAffectedScopes(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	firstVersion := routeSnapshotTestVersion(1, 10)
	firstVersion = NewRoutingEligibilityVersion(append(firstVersion.Items, RoutingEligibilityScopeRevision{
		Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 2}, Revision: 20,
	}))
	first := routeSnapshotTestKey(1, firstVersion)
	second := routeSnapshotTestKey(3, routeSnapshotTestVersion(3, 30))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{Source: EnterpriseMemberRoutePlanSourceLive, Key: first, Candidates: routeSnapshotTestCandidates(1)}))
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{Source: EnterpriseMemberRoutePlanSourceLive, Key: second, Candidates: routeSnapshotTestCandidates(3)}))

	removed := store.InvalidateScopes([]RoutingEligibilityScope{{Type: RoutingEligibilityScopeChannel, ID: 2}})
	require.Equal(t, 1, removed)
	_, ok := store.Restore(first, []int64{1})
	require.False(t, ok)
	_, ok = store.Restore(second, []int64{3})
	require.True(t, ok)
}

func TestRouteSnapshotExcludesKeyBodyCredentialsCapacityAndStickyState(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	store := newRouteSnapshotTestStore(2*time.Minute, &now)
	key := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	key.PublicModel = " GPT-5.6-TERRA "
	key.Endpoint = "v1/responses"
	require.True(t, store.Store(EnterpriseMemberRouteSnapshotPlan{
		Source: EnterpriseMemberRoutePlanSourceLive,
		Key:    key,
		Candidates: []EnterpriseMemberRouteSnapshotCandidate{{
			GroupID:            1,
			ReasonCode:         " ELIGIBLE ",
			PublicModel:        " GPT-5.6-TERRA ",
			ChannelMappedModel: " GPT-5.6-TERRA-UPSTREAM ",
			UpstreamModel:      " GPT-5.6-TERRA-UPSTREAM ",
			InboundProtocol:    ModelProtocolOpenAIResponses,
			UpstreamProtocol:   ModelProtocolOpenAIResponses,
			DeliveryMode:       ModelDeliveryModeNative,
		}},
	}))

	lookup := routeSnapshotTestKey(1, routeSnapshotTestVersion(1, 10))
	lookup.PublicModel = "gpt-5.6-terra"
	restored, ok := store.Restore(lookup, []int64{1})
	require.True(t, ok)
	require.Equal(t, "gpt-5.6-terra", restored.Key.PublicModel)
	require.Equal(t, "/v1/responses", restored.Key.Endpoint)
	require.Equal(t, []EnterpriseMemberRouteSnapshotCandidate{{
		GroupID:            1,
		ReasonCode:         "eligible",
		PublicModel:        "gpt-5.6-terra",
		ChannelMappedModel: "gpt-5.6-terra-upstream",
		UpstreamModel:      "gpt-5.6-terra-upstream",
		InboundProtocol:    ModelProtocolOpenAIResponses,
		UpstreamProtocol:   ModelProtocolOpenAIResponses,
		DeliveryMode:       ModelDeliveryModeNative,
	}}, restored.Candidates)
}

func newRouteSnapshotTestStore(ttl time.Duration, now *time.Time) *EnterpriseMemberRouteSnapshotStore {
	return NewEnterpriseMemberRouteSnapshotStoreWithClock(ttl, func() time.Time {
		return *now
	})
}

func routeSnapshotTestKey(groupID int64, version RoutingEligibilityVersion) EnterpriseMemberRouteSnapshotKey {
	return EnterpriseMemberRouteSnapshotKey{
		PublicModel:      "gpt-5.6-terra",
		Endpoint:         "/v1/responses",
		InboundProtocol:  ModelProtocolOpenAIResponses,
		Intent:           EnterpriseMemberRouteIntentText,
		GroupID:          groupID,
		Eligibility:      version,
		AlgorithmVersion: enterpriseMemberRouteSnapshotAlgorithmVersion,
	}
}

func routeSnapshotTestVersion(groupID int64, groupRevision uint64) RoutingEligibilityVersion {
	return NewRoutingEligibilityVersion([]RoutingEligibilityScopeRevision{
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 0}, Revision: 1},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeAccount, ID: 0}, Revision: 2},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeProtocol, ID: 0}, Revision: 3},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeComposite, ID: 0}, Revision: 4},
		{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: groupID}, Revision: groupRevision},
	})
}

func routeSnapshotTestCandidates(groupIDs ...int64) []EnterpriseMemberRouteSnapshotCandidate {
	candidates := make([]EnterpriseMemberRouteSnapshotCandidate, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		candidates = append(candidates, EnterpriseMemberRouteSnapshotCandidate{
			GroupID:            groupID,
			ReasonCode:         "eligible",
			PublicModel:        "gpt-5.6-terra",
			ChannelMappedModel: "gpt-5.6-terra",
			UpstreamModel:      "gpt-5.6-terra",
			InboundProtocol:    ModelProtocolOpenAIResponses,
			UpstreamProtocol:   ModelProtocolOpenAIResponses,
			DeliveryMode:       ModelDeliveryModeNative,
		})
	}
	return candidates
}

func routeSnapshotCandidateGroupIDs(candidates []EnterpriseMemberRouteSnapshotCandidate) []int64 {
	groupIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		groupIDs = append(groupIDs, candidate.GroupID)
	}
	return groupIDs
}
