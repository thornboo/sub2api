package service

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestParseUsageRequestType(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		input   string
		want    RequestType
		wantErr bool
	}

	cases := []testCase{
		{name: "unknown", input: "unknown", want: RequestTypeUnknown},
		{name: "sync", input: "sync", want: RequestTypeSync},
		{name: "stream", input: "stream", want: RequestTypeStream},
		{name: "ws_v2", input: "ws_v2", want: RequestTypeWSV2},
		{name: "case_insensitive", input: "WS_V2", want: RequestTypeWSV2},
		{name: "trim_spaces", input: "  stream  ", want: RequestTypeStream},
		{name: "invalid", input: "xxx", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUsageRequestType(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestRequestTypeNormalizeAndString(t *testing.T) {
	t.Parallel()

	require.Equal(t, RequestTypeUnknown, RequestType(99).Normalize())
	require.Equal(t, "unknown", RequestType(99).String())
	require.Equal(t, "sync", RequestTypeSync.String())
	require.Equal(t, "stream", RequestTypeStream.String())
	require.Equal(t, "ws_v2", RequestTypeWSV2.String())
}

func TestRequestTypeFromLegacy(t *testing.T) {
	t.Parallel()

	require.Equal(t, RequestTypeWSV2, RequestTypeFromLegacy(false, true))
	require.Equal(t, RequestTypeStream, RequestTypeFromLegacy(true, false))
	require.Equal(t, RequestTypeSync, RequestTypeFromLegacy(false, false))
}

func TestApplyLegacyRequestFields(t *testing.T) {
	t.Parallel()

	stream, ws := ApplyLegacyRequestFields(RequestTypeSync, true, true)
	require.False(t, stream)
	require.False(t, ws)

	stream, ws = ApplyLegacyRequestFields(RequestTypeStream, false, true)
	require.True(t, stream)
	require.False(t, ws)

	stream, ws = ApplyLegacyRequestFields(RequestTypeWSV2, false, false)
	require.True(t, stream)
	require.True(t, ws)

	stream, ws = ApplyLegacyRequestFields(RequestTypeUnknown, true, false)
	require.True(t, stream)
	require.False(t, ws)
}

func TestUsageLogSyncRequestTypeAndLegacyFields(t *testing.T) {
	t.Parallel()

	log := &UsageLog{RequestType: RequestTypeWSV2, Stream: false, OpenAIWSMode: false}
	log.SyncRequestTypeAndLegacyFields()

	require.Equal(t, RequestTypeWSV2, log.RequestType)
	require.True(t, log.Stream)
	require.True(t, log.OpenAIWSMode)
}

func TestUsageLogEffectiveRequestTypeFallback(t *testing.T) {
	t.Parallel()

	log := &UsageLog{RequestType: RequestTypeUnknown, Stream: true, OpenAIWSMode: true}
	require.Equal(t, RequestTypeWSV2, log.EffectiveRequestType())
}

func TestUsageLogEffectiveRequestTypeNilReceiver(t *testing.T) {
	t.Parallel()

	var log *UsageLog
	require.Equal(t, RequestTypeUnknown, log.EffectiveRequestType())
}

func TestUsageLogSyncRequestTypeAndLegacyFieldsNilReceiver(t *testing.T) {
	t.Parallel()

	var log *UsageLog
	log.SyncRequestTypeAndLegacyFields()
}

func TestApplyAPIKeyUsageAttributionKeepsMemberIDWithoutLoadedSnapshot(t *testing.T) {
	t.Parallel()

	memberID := int64(42)
	log := &UsageLog{}
	applyAPIKeyUsageAttribution(log, &APIKey{MemberID: &memberID})

	require.NotNil(t, log.MemberID)
	require.Equal(t, memberID, *log.MemberID)
	require.Nil(t, log.MemberCodeSnapshot)
	require.Nil(t, log.MemberNameSnapshot)
}

func TestUsageGroupIDPrefersRequestActiveGroupForMemberKey(t *testing.T) {
	t.Parallel()

	memberID := int64(42)
	staleGroupID := int64(10)
	ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
		MemberID: memberID,
		GroupID:  11,
	})

	got := usageGroupID(ctx, &APIKey{MemberID: &memberID, GroupID: &staleGroupID})
	require.NotNil(t, got)
	require.Equal(t, int64(11), *got)
}

func TestApplyUsageRoutingPlanEvidencePersistsLKGSnapshotAge(t *testing.T) {
	t.Parallel()

	snapshotAgeMs := int64(3456)
	ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
		ModelPlanApplied:       true,
		RoutePlanSource:        EnterpriseMemberRoutePlanSourceLastKnownGood,
		RoutePlanSnapshotAgeMs: &snapshotAgeMs,
	})
	usage := &UsageLog{}

	ApplyUsageRoutingPlanEvidence(ctx, usage)

	require.Equal(t, "last_known_good", usage.RoutePlanSource)
	require.NotNil(t, usage.RoutePlanSnapshotAgeMs)
	require.Equal(t, snapshotAgeMs, *usage.RoutePlanSnapshotAgeMs)
}

func TestApplyUsageRoutingPlanEvidenceLeavesLiveSnapshotAgeEmpty(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
		ModelPlanApplied: true,
		RoutePlanSource:  EnterpriseMemberRoutePlanSourceLive,
	})
	usage := &UsageLog{}

	ApplyUsageRoutingPlanEvidence(ctx, usage)

	require.Equal(t, "live", usage.RoutePlanSource)
	require.Nil(t, usage.RoutePlanSnapshotAgeMs)
}

func TestApplyUsageRoutingShadowSuccessEvidencePersistsPrunedLegacySuccess(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
		MemberID: 42,
		GroupID:  11,
	})
	ctx = WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
		Mode:            EnterpriseMemberModelAdmissionShadowPublished,
		PlanSource:      EnterpriseMemberRoutePlanSourceLive,
		Model:           "minimax-m3",
		LegacyGroupIDs:  []int64{11, 12},
		PlannedGroupIDs: []int64{12},
		Rejected: []UsageRoutingShadowRejection{{
			GroupID: 11,
			Reason:  EnterpriseMemberRouteReasonModelUnpublished,
		}, {
			GroupID: 11,
			Reason:  EnterpriseMemberRouteReasonModelUnpublished,
		}},
	})
	usage := &UsageLog{}

	ApplyUsageRoutingShadowSuccessEvidence(ctx, usage)

	require.NotNil(t, usage.ScheduleMeta)
	require.Equal(t, UsageShadowDiffLegacySuccessNewPruned, usage.ScheduleMeta.ShadowDiffType)
	require.Equal(t, []string{"model_unpublished"}, usage.ScheduleMeta.ShadowReasonCodes)
	require.Equal(t, "live", usage.ScheduleMeta.ShadowPlanSource)
	require.Equal(t, 2, usage.ScheduleMeta.ShadowLegacyGroups)
	require.Equal(t, 1, usage.ScheduleMeta.ShadowPlannedGroups)
	require.Equal(t, 1, usage.ScheduleMeta.ShadowPrunedGroups)
}

func TestApplyUsageRoutingShadowSuccessEvidenceSkipsUnsafeCases(t *testing.T) {
	t.Parallel()

	base := func() context.Context {
		ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
			MemberID: 42,
			GroupID:  11,
		})
		return WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
			Mode:            EnterpriseMemberModelAdmissionShadowPublished,
			PlanSource:      EnterpriseMemberRoutePlanSourceLive,
			LegacyGroupIDs:  []int64{11, 12},
			PlannedGroupIDs: []int64{12},
			Rejected: []UsageRoutingShadowRejection{{
				GroupID: 11,
				Reason:  EnterpriseMemberRouteReasonModelUnpublished,
			}},
		})
	}

	tests := []struct {
		name   string
		ctx    context.Context
		usage  UsageLog
		mutate func(context.Context) context.Context
	}{
		{name: "legacy mode", mutate: func(ctx context.Context) context.Context {
			return WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
				Mode:            EnterpriseMemberModelAdmissionLegacyOrderOnly,
				LegacyGroupIDs:  []int64{11, 12},
				PlannedGroupIDs: []int64{12},
				Rejected: []UsageRoutingShadowRejection{{
					GroupID: 11,
					Reason:  EnterpriseMemberRouteReasonModelUnpublished,
				}},
			})
		}},
		{name: "enforce mode", mutate: func(ctx context.Context) context.Context {
			return WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
				Mode:            EnterpriseMemberModelAdmissionEnforcePublished,
				LegacyGroupIDs:  []int64{11, 12},
				PlannedGroupIDs: []int64{12},
				Rejected: []UsageRoutingShadowRejection{{
					GroupID: 11,
					Reason:  EnterpriseMemberRouteReasonModelUnpublished,
				}},
			})
		}},
		{name: "evaluation error", mutate: func(ctx context.Context) context.Context {
			return WithUsageRoutingShadowEvidence(ctx, UsageRoutingShadowEvidence{
				Mode:            EnterpriseMemberModelAdmissionShadowPublished,
				LegacyGroupIDs:  []int64{11, 12},
				PlannedGroupIDs: []int64{12},
				EvaluationError: true,
				Rejected: []UsageRoutingShadowRejection{{
					GroupID: 11,
					Reason:  EnterpriseMemberRouteReasonModelUnpublished,
				}},
			})
		}},
		{name: "ordinary key no active member", mutate: func(ctx context.Context) context.Context {
			return context.WithValue(ctx, ctxkey.ActiveGroup, &ActiveGroupContext{GroupID: 11})
		}},
		{name: "cyber non-success", usage: UsageLog{RequestType: RequestTypeCyberBlocked}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := base()
			if tt.mutate != nil {
				ctx = tt.mutate(ctx)
			}
			usage := tt.usage

			ApplyUsageRoutingShadowSuccessEvidence(ctx, &usage)

			require.Nil(t, usage.ScheduleMeta)
		})
	}
}

func TestUsageScheduleMetaFromOpenAIDecision(t *testing.T) {
	t.Parallel()

	got := UsageScheduleMetaFromOpenAIDecision(OpenAIAccountScheduleDecision{
		Layer:               openAIAccountScheduleLayerLoadBalance,
		CandidateCount:      3,
		TopK:                2,
		LatencyMs:           7,
		LoadSkew:            math.Inf(1),
		SelectedAccountID:   42,
		SelectedAccountType: AccountTypeAPIKey,
	})

	require.NotNil(t, got)
	require.Equal(t, "openai", got.Provider)
	require.Equal(t, openAIAccountScheduleLayerLoadBalance, got.Layer)
	require.Equal(t, 3, got.CandidateCount)
	require.Equal(t, 2, got.TopK)
	require.Equal(t, int64(7), got.LatencyMs)
	require.Zero(t, got.LoadSkew)
	require.Equal(t, int64(42), got.SelectedAccountID)
	require.Equal(t, AccountTypeAPIKey, got.SelectedAccountType)
}

func TestUsageScheduleMetaFromOpenAIDecisionEmpty(t *testing.T) {
	t.Parallel()

	require.Nil(t, UsageScheduleMetaFromOpenAIDecision(OpenAIAccountScheduleDecision{}))
}
