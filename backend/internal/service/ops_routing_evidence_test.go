package service

import (
	"context"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestSanitizeOpsRoutingAttemptsForQueueBoundsAndRedacts(t *testing.T) {
	secret := "sk-proj-secret-token"
	entry := &OpsInsertErrorLogInput{
		RoutingPlanSource: "last_known_good",
		RoutingAttempts: []*OpsRoutingAttemptEvidence{
			{
				Stage:          OpsRoutingAttemptStageActualAttempt,
				Outcome:        string(GroupAttemptOutcomeTerminalFailure),
				GroupID:        12,
				AttemptNumber:  1,
				CandidateIndex: 0,
				Platform:       "openai\nBearer " + secret,
				RequestedModel: "gpt-5.6-terra\u0000",
				Reason:         string(OpsGroupRetryReasonCapabilityMismatch) + " " + secret,
				UnsafeReason:   string(GroupAttemptUnsafeReasonResponseCommitted) + " " + secret,
			},
			{
				Stage:   "invalid",
				GroupID: 99,
				Reason:  secret,
			},
		},
	}

	require.NoError(t, SanitizeOpsRoutingAttemptsForQueue(entry))
	require.Nil(t, entry.RoutingAttempts)
	require.NotNil(t, entry.RoutingAttemptsJSON)
	require.NotContains(t, *entry.RoutingAttemptsJSON, secret)
	require.NotContains(t, *entry.RoutingAttemptsJSON, "Bearer")
	require.NotContains(t, *entry.RoutingAttemptsJSON, "\\u0000")

	attempts, err := ParseOpsRoutingAttempts(*entry.RoutingAttemptsJSON)
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	require.Equal(t, int64(12), attempts[0].GroupID)
	require.Equal(t, OpsRoutingAttemptStageActualAttempt, attempts[0].Stage)
	require.Contains(t, attempts[0].Reason, string(OpsGroupRetryReasonCapabilityMismatch))
}

func TestSanitizeOpsRoutingAttemptsForQueueCapsEvents(t *testing.T) {
	entry := &OpsInsertErrorLogInput{}
	for i := 0; i < opsRoutingAttemptsMax+5; i++ {
		entry.RoutingAttempts = append(entry.RoutingAttempts, &OpsRoutingAttemptEvidence{
			Stage:   OpsRoutingAttemptStagePlannedCandidate,
			Outcome: OpsRoutingAttemptOutcomePlanned,
			GroupID: int64(i + 1),
		})
	}

	require.NoError(t, SanitizeOpsRoutingAttemptsForQueue(entry))
	require.NotNil(t, entry.RoutingAttemptsJSON)
	attempts, err := ParseOpsRoutingAttempts(*entry.RoutingAttemptsJSON)
	require.NoError(t, err)
	require.Len(t, attempts, opsRoutingAttemptsMax)
	require.Equal(t, int64(6), attempts[0].GroupID)
	require.LessOrEqual(t, len(*entry.RoutingAttemptsJSON), len(strings.Repeat("x", 8192)))
}

func TestApplyUsageRoutingShadowSuccessEvidencePersistsKeptBaseline(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
		MemberID: 7,
		GroupID:  12,
	})
	ctx = WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
		Mode:             EnterpriseMemberModelAdmissionShadowPublished,
		PlanSource:       EnterpriseMemberRoutePlanSourceLive,
		LegacyGroupIDs:   []int64{12, 34},
		PlannedGroupIDs:  []int64{12},
		PlannerLatencyMs: 9,
	})
	usage := &UsageLog{}

	ApplyUsageRoutingShadowSuccessEvidence(ctx, usage)

	require.NotNil(t, usage.ScheduleMeta)
	require.True(t, usage.ScheduleMeta.ShadowPlanEvaluated)
	require.True(t, usage.ScheduleMeta.ShadowGroupKept)
	require.Equal(t, int64(9), usage.ScheduleMeta.ShadowPlannerLatencyMs)
	require.Equal(t, "live", usage.ScheduleMeta.ShadowPlanSource)
	require.Equal(t, 2, usage.ScheduleMeta.ShadowLegacyGroups)
	require.Equal(t, 1, usage.ScheduleMeta.ShadowPlannedGroups)
	require.Equal(t, 1, usage.ScheduleMeta.ShadowPrunedGroups)
	require.Empty(t, usage.ScheduleMeta.ShadowDiffType)
}

func TestApplyUsageRoutingShadowSuccessEvidencePersistsLegacySuccessNewPruned(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
		MemberID: 7,
		GroupID:  34,
	})
	ctx = WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
		Mode:            EnterpriseMemberModelAdmissionShadowPublished,
		PlanSource:      EnterpriseMemberRoutePlanSourceLive,
		LegacyGroupIDs:  []int64{12, 34},
		PlannedGroupIDs: []int64{12},
		Rejected: []UsageRoutingShadowRejection{{
			GroupID: 34,
			Reason:  EnterpriseMemberRouteReasonModelUnpublished,
		}},
	})
	usage := &UsageLog{}

	ApplyUsageRoutingShadowSuccessEvidence(ctx, usage)

	require.NotNil(t, usage.ScheduleMeta)
	require.True(t, usage.ScheduleMeta.ShadowPlanEvaluated)
	require.False(t, usage.ScheduleMeta.ShadowGroupKept)
	require.Equal(t, UsageShadowDiffLegacySuccessNewPruned, usage.ScheduleMeta.ShadowDiffType)
	require.Equal(t, []string{"model_unpublished"}, usage.ScheduleMeta.ShadowReasonCodes)
}
