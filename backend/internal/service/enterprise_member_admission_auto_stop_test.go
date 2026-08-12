package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberAdmissionAutoStopEvidenceStub struct {
	summary EnterpriseMemberModelAdmissionAutoStopEvidenceSummary
	err     error
}

func (s enterpriseMemberAdmissionAutoStopEvidenceStub) SummarizeEnterpriseMemberModelAdmissionAutoStopEvidence(context.Context) (EnterpriseMemberModelAdmissionAutoStopEvidenceSummary, error) {
	return s.summary, s.err
}

func TestEnterpriseMemberModelAdmissionAutoStopReasons(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		summary EnterpriseMemberModelAdmissionAutoStopEvidenceSummary
		reason  string
	}{
		{
			name: "success_rate_drop",
			summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{
				GeneratedAt:                now,
				Window:                     "15m",
				EnforceSamples:             100,
				ControlSamples:             100,
				EnforceSuccessRatePermille: 930,
				ControlSuccessRatePermille: 990,
			},
			reason: EnterpriseMemberModelAdmissionAutoStopReasonSuccessRateDrop,
		},
		{name: "unreviewed_alias", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{UnreviewedAliasActiveCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonUnreviewedAliasActive},
		{name: "evaluation_failed", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{EvaluationFailedPermille: 10}, reason: EnterpriseMemberModelAdmissionAutoStopReasonEvaluationFailedElevated},
		{name: "lkg_generation_mismatch", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{LKGGenerationMismatchCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonLKGUnsafe},
		{name: "lkg_stale", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{LKGStaleHitCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonLKGUnsafe},
		{name: "lkg_wrong_group", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{LKGWrongGroupAfterUseCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonLKGUnsafe},
		{name: "planner_p95", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{PlannerP95Ms: 6}, reason: EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded},
		{name: "planner_p99", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{PlannerP99Ms: 21}, reason: EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded},
		{name: "unpublished_actual_attempt", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{UnpublishedModelActualAttemptCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonUnpublishedActualAttempt},
		{name: "unauthorized_candidate", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{UnauthorizedCandidateCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonRestrictionBypass},
		{name: "lkg_revoked_restore", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{LKGRevokedGroupRestoreCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonRestrictionBypass},
		{name: "restriction_bypass", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{RestrictionBypassCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonRestrictionBypass},
		{name: "unsafe_replay", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{UnsafeReplayCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay},
		{name: "explicit_alias_unrecoverable", summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{ExplicitAliasUnrecoverableCount: 1}, reason: EnterpriseMemberModelAdmissionAutoStopReasonExplicitAliasUnrecoverable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := EvaluateEnterpriseMemberModelAdmissionAutoStop(tc.summary, DefaultEnterpriseMemberModelAdmissionAutoStopThresholds())

			require.True(t, state.Stopped)
			require.Equal(t, EnterpriseMemberModelAdmissionAutoStopSourceMetrics, state.Source)
			require.Contains(t, state.Reasons, tc.reason)
			require.Equal(t, tc.reason, state.Reason)
			if !tc.summary.GeneratedAt.IsZero() {
				require.Equal(t, now.Format(time.RFC3339), state.Generated)
			}
		})
	}
}

func TestEnterpriseMemberModelAdmissionAutoStopDebouncesSuccessRateUntilSamplesAreReady(t *testing.T) {
	state := EvaluateEnterpriseMemberModelAdmissionAutoStop(EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{
		EnforceSamples:             99,
		ControlSamples:             100,
		EnforceSuccessRatePermille: 0,
		ControlSuccessRatePermille: 1000,
	}, DefaultEnterpriseMemberModelAdmissionAutoStopThresholds())

	require.False(t, state.Stopped)
	require.Empty(t, state.Reason)
}

func TestEnterpriseMemberModelAdmissionAutoStopProviderErrorFailsClosed(t *testing.T) {
	state := EvaluateEnterpriseMemberModelAdmissionAutoStopFromProvider(context.Background(), enterpriseMemberAdmissionAutoStopEvidenceStub{err: errors.New("db down")})

	require.True(t, state.Stopped)
	require.Equal(t, EnterpriseMemberModelAdmissionAutoStopReasonEvidenceUnavailable, state.Reason)
}

func TestEnterpriseMemberModelAdmissionReadinessMergesMetricAutoStop(t *testing.T) {
	runtime := &readinessRuntimeFake{ready: true, mirrored: true}
	aliasSvc := &EnterpriseMemberAliasReviewService{repo: aliasReadinessRepoFake{summary: &EnterpriseMemberAliasReviewReadinessSummary{}}}
	provider := NewEnterpriseMemberModelAdmissionReadinessProvider(runtime, aliasSvc, enterpriseMemberAdmissionAutoStopEvidenceStub{
		summary: EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{UnpublishedModelActualAttemptCount: 1},
	})

	readiness := provider.EvaluateEnterpriseMemberModelAdmissionReadiness(context.Background())

	require.False(t, readiness.Ready)
	require.True(t, readiness.AutoStopped)
	require.True(t, readiness.AutoStop.Stopped)
	require.Equal(t, EnterpriseMemberModelAdmissionAutoStopReasonUnpublishedActualAttempt, readiness.Reason)
	require.Equal(t, "stop_clear", readiness.Conditions[len(readiness.Conditions)-1].Name)
	require.Equal(t, EnterpriseMemberModelAdmissionAutoStopReasonUnpublishedActualAttempt, readiness.Conditions[len(readiness.Conditions)-1].Reason)
}

func TestEnterpriseMemberModelAdmissionResolverDowngradesOnMetricAutoStop(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{
		readiness: EnterpriseMemberModelAdmissionEnforceReadiness{
			Ready:      false,
			Source:     "test",
			AutoStop:   EnterpriseMemberModelAdmissionAutoStopState{Stopped: true, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics, Reason: EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay, Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay}},
			Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{{Name: "auto_stop_clear", Ready: false, Reason: EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics}},
		},
	})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42})

	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
	require.Equal(t, "metric_auto_stop", runtime.Source)
	require.Equal(t, EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay, runtime.Readiness.AutoStop.Reason)
}

func TestEnterpriseMemberModelAdmissionRolloutExpansionDirection(t *testing.T) {
	current := EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 10, EnterpriseUserIDs: []int64{7}, MemberIDs: []int64{42}}

	require.True(t, IsEnterpriseMemberModelAdmissionRolloutExpansion(current, EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 11, EnterpriseUserIDs: []int64{7}, MemberIDs: []int64{42}}))
	require.False(t, IsEnterpriseMemberModelAdmissionRolloutExpansion(current, EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 10, EnterpriseUserIDs: []int64{7, 8}, MemberIDs: []int64{42}}))
	require.False(t, IsEnterpriseMemberModelAdmissionRolloutExpansion(current, EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 10, EnterpriseUserIDs: []int64{7}, MemberIDs: []int64{42, 43}}))
	require.False(t, IsEnterpriseMemberModelAdmissionRolloutExpansion(current, EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 9, EnterpriseUserIDs: []int64{7}, MemberIDs: []int64{42}}))
	require.False(t, IsEnterpriseMemberModelAdmissionRolloutExpansion(current, EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 10, EnterpriseUserIDs: []int64{}, MemberIDs: []int64{42}}))
	require.False(t, IsEnterpriseMemberModelAdmissionRolloutExpansion(current, EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 10, EnterpriseUserIDs: []int64{7}, MemberIDs: []int64{42}, AutoStop: true}))
}

func TestBuildSystemSettingsUpdatesAllowsShrinkDuringMetricAutoStopButRejectsExpansion(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{
		readiness: EnterpriseMemberModelAdmissionEnforceReadiness{
			Ready:      false,
			Source:     "test",
			AutoStop:   EnterpriseMemberModelAdmissionAutoStopState{Stopped: true, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics, Reason: EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded, Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded}},
			Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{{Name: "auto_stop_clear", Ready: false, Reason: EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics}},
		},
	})
	defer restore()
	currentPolicy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 20, MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: currentPolicy,
	}, &config.Config{})

	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		EnterpriseMemberModelAdmissionMode:   string(EnterpriseMemberModelAdmissionEnforcePublished),
		EnterpriseMemberModelAdmissionSource: "settings",
		EnterpriseMemberModelAdmissionRollout: EnterpriseMemberModelAdmissionRolloutState{Policy: EnterpriseMemberModelAdmissionRolloutPolicy{
			Percentage: 10,
			MemberIDs:  []int64{42},
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, updates)

	updates, err = svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		EnterpriseMemberModelAdmissionMode:   string(EnterpriseMemberModelAdmissionEnforcePublished),
		EnterpriseMemberModelAdmissionSource: "settings",
		EnterpriseMemberModelAdmissionRollout: EnterpriseMemberModelAdmissionRolloutState{Policy: EnterpriseMemberModelAdmissionRolloutPolicy{
			Percentage: 21,
			MemberIDs:  []int64{42},
		}},
	})
	require.Error(t, err)
	require.Nil(t, updates)
	require.Contains(t, err.Error(), "ENFORCE_AUTO_STOPPED")
}

func TestUpdateSettingsRejectsNewEnforceDuringMetricAutoStopWhenStoredModeIsNotEnforce(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{
		readiness: EnterpriseMemberModelAdmissionEnforceReadiness{
			Ready:      false,
			Source:     "test",
			AutoStop:   EnterpriseMemberModelAdmissionAutoStopState{Stopped: true, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics, Reason: EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded, Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded}},
			Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{{Name: "auto_stop_clear", Ready: false, Reason: EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics}},
		},
	})
	defer restore()
	currentPolicy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 20, MemberIDs: []int64{42}})
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		rawMode   string
		getErr    error
		rollout   string
		wantError string
	}{
		{name: "stored shadow", rawMode: string(EnterpriseMemberModelAdmissionShadowPublished), rollout: currentPolicy, wantError: "ENFORCE_NOT_READY"},
		{name: "stored missing", rollout: currentPolicy, wantError: "ENFORCE_NOT_READY"},
		{name: "stored read failure", getErr: errors.New("setting unavailable"), rollout: currentPolicy, wantError: "ENFORCE_NOT_READY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &enterpriseMemberAdmissionSettingRepoStub{
				value:        tc.rawMode,
				rolloutValue: tc.rollout,
				getErr:       tc.getErr,
			}
			svc := NewSettingService(repo, &config.Config{})

			err := svc.UpdateSettings(context.Background(), &SystemSettings{
				EnterpriseMemberModelAdmissionMode:   string(EnterpriseMemberModelAdmissionEnforcePublished),
				EnterpriseMemberModelAdmissionSource: "settings",
				EnterpriseMemberModelAdmissionRollout: EnterpriseMemberModelAdmissionRolloutState{Source: "settings", Policy: EnterpriseMemberModelAdmissionRolloutPolicy{
					Percentage: 10,
					MemberIDs:  []int64{42},
				}},
				NativeModelProtocolRoutingSource: "config",
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), tc.wantError)
			require.Nil(t, repo.updates)
		})
	}
}
