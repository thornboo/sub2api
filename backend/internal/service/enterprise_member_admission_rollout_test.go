package service

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type readinessRuntimeFake struct {
	ready    bool
	mirrored bool
	scopes   []RoutingEligibilityScope
}

func (f readinessRuntimeFake) Ready() bool {
	return f.ready
}

func (f *readinessRuntimeFake) MirroredVersion(scopes []RoutingEligibilityScope) (RoutingEligibilityVersion, bool) {
	f.scopes = scopes
	if !f.mirrored {
		return RoutingEligibilityVersion{}, false
	}
	return NewRoutingEligibilityVersion([]RoutingEligibilityScopeRevision{{Scope: RoutingEligibilityScope{Type: RoutingEligibilityScopeChannel, ID: 0}, Revision: 1}}), true
}

func (f *readinessRuntimeFake) scopeSet() map[RoutingEligibilityScope]struct{} {
	set := make(map[RoutingEligibilityScope]struct{}, len(f.scopes))
	for _, scope := range f.scopes {
		set[scope] = struct{}{}
	}
	return set
}

type aliasReadinessRepoFake struct {
	summary *EnterpriseMemberAliasReviewReadinessSummary
	err     error
}

func (f aliasReadinessRepoFake) ListLegacySuccessNewPruned(context.Context, EnterpriseMemberAliasReviewListInput) ([]EnterpriseMemberAliasReviewItem, error) {
	return nil, nil
}

func (f aliasReadinessRepoFake) UpsertReview(context.Context, EnterpriseMemberAliasReviewUpsert) (*EnterpriseMemberAliasReviewRecord, error) {
	return nil, nil
}

func (f aliasReadinessRepoFake) GetReadinessSummary(context.Context, time.Time) (*EnterpriseMemberAliasReviewReadinessSummary, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.summary == nil {
		return &EnterpriseMemberAliasReviewReadinessSummary{}, nil
	}
	return f.summary, nil
}

func enterpriseMemberAdmissionSequentialIDs(start int64, count int) []int64 {
	ids := make([]int64, count)
	for i := range ids {
		ids[i] = start + int64(i)
	}
	return ids
}

func enterpriseMemberAdmissionRepeatedIDs(value int64, count int) []int64 {
	ids := make([]int64, count)
	for i := range ids {
		ids[i] = value
	}
	return ids
}

func TestNormalizeEnterpriseMemberModelAdmissionRolloutPolicyAcceptsBoundedPolicy(t *testing.T) {
	policy, err := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{
		EnterpriseUserIDs: enterpriseMemberAdmissionSequentialIDs(1, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2),
		MemberIDs:         enterpriseMemberAdmissionSequentialIDs(10_000, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2),
		Percentage:        100,
		Salt:              strings.Repeat("a", enterpriseMemberModelAdmissionMaxRolloutSaltBytes),
	})

	require.NoError(t, err)
	require.Len(t, policy.EnterpriseUserIDs, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2)
	require.Len(t, policy.MemberIDs, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2)
	require.Len(t, []byte(policy.Salt), enterpriseMemberModelAdmissionMaxRolloutSaltBytes)
}

func TestNormalizeEnterpriseMemberModelAdmissionRolloutPolicyRejectsBounds(t *testing.T) {
	tests := []struct {
		name   string
		policy EnterpriseMemberModelAdmissionRolloutPolicy
		want   string
	}{
		{
			name:   "enterprise user ids per list",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{EnterpriseUserIDs: enterpriseMemberAdmissionSequentialIDs(1, enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1)},
			want:   "enterprise_user_ids",
		},
		{
			name:   "duplicate enterprise user ids before normalization",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{EnterpriseUserIDs: enterpriseMemberAdmissionRepeatedIDs(7, enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1)},
			want:   "enterprise_user_ids",
		},
		{
			name:   "invalid enterprise user ids before normalization",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{EnterpriseUserIDs: enterpriseMemberAdmissionRepeatedIDs(-1, enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1)},
			want:   "enterprise_user_ids",
		},
		{
			name:   "member ids per list",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: enterpriseMemberAdmissionSequentialIDs(1, enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1)},
			want:   "member_ids",
		},
		{
			name:   "duplicate member ids before normalization",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: enterpriseMemberAdmissionRepeatedIDs(42, enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1)},
			want:   "member_ids",
		},
		{
			name: "explicit target cardinality",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{
				EnterpriseUserIDs: enterpriseMemberAdmissionSequentialIDs(1, enterpriseMemberModelAdmissionMaxRolloutIDsPerList),
				MemberIDs:         enterpriseMemberAdmissionSequentialIDs(10_000, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets-enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1),
			},
			want: "explicit targets",
		},
		{
			name:   "salt bytes",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{Salt: strings.Repeat("界", enterpriseMemberModelAdmissionMaxRolloutSaltBytes/3+1)},
			want:   "salt",
		},
		{
			name: "serialized policy bytes",
			policy: EnterpriseMemberModelAdmissionRolloutPolicy{
				EnterpriseUserIDs: enterpriseMemberAdmissionSequentialIDs(math.MaxInt64-int64(enterpriseMemberModelAdmissionMaxRolloutExplicitTargets)+1, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2),
				MemberIDs:         enterpriseMemberAdmissionSequentialIDs(math.MaxInt64-int64(enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2)+1, enterpriseMemberModelAdmissionMaxRolloutExplicitTargets/2),
			},
			want: "policy",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(tc.policy)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestEnterpriseMemberModelAdmissionReadinessProviderAllowsEnforceWhenAllConditionsMet(t *testing.T) {
	runtime := &readinessRuntimeFake{ready: true, mirrored: true}
	aliasSvc := &EnterpriseMemberAliasReviewService{repo: aliasReadinessRepoFake{summary: &EnterpriseMemberAliasReviewReadinessSummary{
		BlockingUnreviewedActive7d:      0,
		BlockingUnreviewedActive30d:     0,
		LegacySuccessNewPrunedActive7d:  0,
		LegacySuccessNewPrunedActive30d: 0,
	}}}
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	evidenceRepo := &admissionEvidenceRepoFake{summary: completeAdmissionEvidenceSummary(now)}
	evidenceSvc := NewEnterpriseMemberAdmissionEvidenceService(evidenceRepo)
	evidenceSvc.now = func() time.Time { return now }

	provider := NewEnterpriseMemberModelAdmissionReadinessProvider(runtime, aliasSvc, evidenceSvc)
	readiness := provider.EvaluateEnterpriseMemberModelAdmissionReadiness(context.Background())

	require.Truef(t, readiness.Ready, "readiness=%+v", readiness)
	require.True(t, readiness.CanaryReady)
	require.True(t, readiness.ExpansionReady)
	require.Empty(t, readiness.Reason)
	require.Equal(t, EnterpriseMemberModelAdmissionInjectedReadinessSource, readiness.Source)
	require.Len(t, readiness.Conditions, 6)
	require.Equal(t, 1, evidenceRepo.calls)

	requiredScopes := map[RoutingEligibilityScope]struct{}{
		{Type: RoutingEligibilityScopeChannel, ID: 0}:   {},
		{Type: RoutingEligibilityScopeAccount, ID: 0}:   {},
		{Type: RoutingEligibilityScopeProtocol, ID: 0}:  {},
		{Type: RoutingEligibilityScopeComposite, ID: 0}: {},
	}
	require.Equal(t, requiredScopes, runtime.scopeSet())
}

func TestEnterpriseMemberModelAdmissionReadinessProviderBlocksWhenRuntimeUnavailable(t *testing.T) {
	provider := NewEnterpriseMemberModelAdmissionReadinessProvider(nil, &EnterpriseMemberAliasReviewService{repo: aliasReadinessRepoFake{summary: &EnterpriseMemberAliasReviewReadinessSummary{}}})
	readiness := provider.EvaluateEnterpriseMemberModelAdmissionReadiness(context.Background())

	require.False(t, readiness.Ready)
	require.Equal(t, "routing_runtime_unavailable", readiness.Conditions[0].Reason)
	require.True(t, readiness.Conditions[1].Ready)
	require.Equal(t, enterpriseMemberModelAdmissionReadinessConditionEval, readiness.Conditions[1].Name)
}

func TestEnterpriseMemberModelAdmissionReadinessProviderBlocksOnPendingAliasGate(t *testing.T) {
	runtime := &readinessRuntimeFake{ready: true, mirrored: true}
	aliasSvc := &EnterpriseMemberAliasReviewService{repo: aliasReadinessRepoFake{summary: &EnterpriseMemberAliasReviewReadinessSummary{
		BlockingUnreviewedActive7d:  2,
		BlockingUnreviewedActive30d: 4,
	}}}
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	evidenceSvc := NewEnterpriseMemberAdmissionEvidenceService(&admissionEvidenceRepoFake{summary: completeAdmissionEvidenceSummary(now)})
	evidenceSvc.now = func() time.Time { return now }
	provider := NewEnterpriseMemberModelAdmissionReadinessProvider(runtime, aliasSvc, evidenceSvc)
	readiness := provider.EvaluateEnterpriseMemberModelAdmissionReadiness(context.Background())

	require.False(t, readiness.Ready)
	require.Equal(t, "legacy_success_new_pruned_requires_review", readiness.Conditions[2].Reason)
	require.Equal(t, "alias_audit_clear", readiness.Conditions[2].Name)
	require.Equal(t, "evidence_pipeline_healthy", readiness.Conditions[3].Name)
}
