package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMarkOpsGroupRetryCreatesTypedAttemptResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newGroupAttemptTestContext(t)

	MarkOpsGroupRetry(c, OpsGroupRetryReasonCapabilityMismatch)

	result, ok := GroupAttemptResultFromContext(c)
	require.True(t, ok)
	require.Equal(t, int64(42), result.GroupID)
	require.Equal(t, 2, result.AttemptNumber)
	require.Equal(t, GroupAttemptOutcomeTerminalFailure, result.Outcome)
	require.Equal(t, OpsGroupRetryReasonCapabilityMismatch, result.Reason)
	require.True(t, result.SafeToReplay)
	require.False(t, result.ResponseCommitted)
	require.False(t, result.OutcomeUnknown)
}

func TestGroupAttemptResultForReplayBlocksUnsafeOutcomes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name         string
		responseDone bool
		mark         func(*gin.Context)
		wantReason   GroupAttemptUnsafeReason
	}{
		{
			name:         "transactional response committed",
			responseDone: true,
			wantReason:   GroupAttemptUnsafeReasonResponseCommitted,
		},
		{
			name:       "service response committed",
			mark:       MarkResponseCommitted,
			wantReason: GroupAttemptUnsafeReasonResponseCommitted,
		},
		{
			name: "budget outcome ambiguous",
			mark: func(c *gin.Context) {
				MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "usage_persistence_failed")
			},
			wantReason: GroupAttemptUnsafeReasonOutcomeUnknown,
		},
		{
			name:       "typed outcome unknown",
			mark:       MarkOpsGroupAttemptOutcomeUnknown,
			wantReason: GroupAttemptUnsafeReasonOutcomeUnknown,
		},
		{
			name:       "external task committed",
			mark:       MarkEnterpriseMemberExternalTaskCommitted,
			wantReason: GroupAttemptUnsafeReasonExternalTask,
		},
		{
			name:       "websocket turn committed",
			mark:       MarkEnterpriseMemberWSTurnCommitted,
			wantReason: GroupAttemptUnsafeReasonWSTurnCommitted,
		},
		{
			name: "client cancelled",
			mark: func(c *gin.Context) {
				ctx, cancel := context.WithCancel(c.Request.Context())
				cancel()
				c.Request = c.Request.WithContext(ctx)
			},
			wantReason: GroupAttemptUnsafeReasonClientCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newGroupAttemptTestContext(t)
			MarkOpsGroupRetry(c, OpsGroupRetryReasonTransientUpstream)
			if tt.mark != nil {
				tt.mark(c)
			}

			result, ok := GroupAttemptResultForReplay(c, tt.responseDone)

			require.False(t, ok)
			require.False(t, result.SafeToReplay)
			require.Equal(t, tt.wantReason, result.UnsafeReason)
		})
	}
}

func TestGroupAttemptResultRequiresActiveGroupAndClosedReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil)

	MarkGroupAttemptResult(c, GroupAttemptResult{
		Outcome:      GroupAttemptOutcomeTerminalFailure,
		Reason:       OpsGroupRetryReason("client_error"),
		SafeToReplay: true,
	})

	result, ok := GroupAttemptResultForReplay(c, false)

	require.False(t, ok)
	require.False(t, result.SafeToReplay)
	require.Equal(t, GroupAttemptUnsafeReasonInvalidReason, result.UnsafeReason)

	MarkOpsGroupRetry(c, OpsGroupRetryReasonCapabilityMismatch)
	result, ok = GroupAttemptResultForReplay(c, false)

	require.False(t, ok)
	require.False(t, result.SafeToReplay)
	require.Equal(t, GroupAttemptUnsafeReasonMissingActiveGroup, result.UnsafeReason)
}

func newGroupAttemptTestContext(t *testing.T) *gin.Context {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest("POST", "/v1/messages", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.ActiveGroup, &ActiveGroupContext{
		GroupID:       42,
		AttemptNumber: 2,
	}))
	c.Request = req
	return c
}
