package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOrchestrateEnterpriseMemberGroupsRetriesUncommittedGroupFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "req-1")
	c.Request = c.Request.WithContext(ctx)

	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-test")

	var groupIDs []int64
	var bodies []string
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, apiKey.GroupID)
		groupIDs = append(groupIDs, *apiKey.GroupID)
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		bodies = append(bodies, string(body))
		if len(groupIDs) == 1 {
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonTransientUpstream)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "first group exhausted"})
			return
		}
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, int64(22), active.GroupID)
		require.Equal(t, "req-1:g22:a2", active.AttemptID)
		c.JSON(http.StatusOK, gin.H{"group_id": active.GroupID})
	})

	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"group_id":22}`, recorder.Body.String())
	require.Equal(t, []int64{11, 22}, groupIDs)
	require.Equal(t, []string{`{"model":"claude-test"}`, `{"model":"claude-test"}`}, bodies)
}

func TestOrchestrateEnterpriseMemberGroupsRetriesTypedModelCapabilityMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-8"}`))
	ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "req-opus-48")
	c.Request = c.Request.WithContext(ctx)

	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-opus-4-8")

	var groupIDs []int64
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, apiKey.GroupID)
		groupIDs = append(groupIDs, *apiKey.GroupID)
		if len(groupIDs) == 1 {
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "model_not_found"}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"group_id": *apiKey.GroupID})
	})

	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"group_id":22}`, recorder.Body.String())
	require.Equal(t, []int64{11, 22}, groupIDs)
}

func TestOrchestrateEnterpriseMemberGroupsReturnsFinalModelNotFoundAfterAllGroupsMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-typo"}`))

	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-typo")

	var groupIDs []int64
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, apiKey.GroupID)
		groupIDs = append(groupIDs, *apiKey.GroupID)
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "model_not_found",
				"message": "model is not supported by this group",
			},
		})
	})

	handler(c)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.JSONEq(t, `{"error":{"type":"model_not_found","message":"model is not supported by this group"}}`, recorder.Body.String())
	require.Equal(t, []int64{11, 22}, groupIDs)
}

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryCommittedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-test")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		c.Status(http.StatusOK)
		c.Writer.WriteHeaderNow()
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonTransientUpstream)
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-test")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"error":"invalid request"}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsRestoresBodyLengthMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	const originalBody = `{"model":"short"}`
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(originalBody))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(originalBody)))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "short")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		if calls == 1 {
			rewritten := `{"model":"a-much-longer-upstream-model"}`
			restoreRequestBody(c.Request, []byte(rewritten))
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": "retry"})
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, originalBody, string(body))
		require.Equal(t, int64(len(originalBody)), c.Request.ContentLength)
		require.Equal(t, strconv.Itoa(len(originalBody)), c.Request.Header.Get("Content-Length"))
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	handler(c)

	require.Equal(t, 2, calls)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryAmbiguousUpstreamOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-test")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonTransientUpstream)
		service.MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "upstream_outcome_unknown")
		c.JSON(http.StatusBadGateway, gin.H{"error": "unknown upstream outcome"})
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.JSONEq(t, `{"error":"unknown upstream outcome"}`, recorder.Body.String())
}

func testEnterpriseMemberGroupPlan() *enterpriseMemberGroupPlan {
	memberID := int64(7)
	member := &service.EnterpriseMember{ID: memberID, Version: 3}
	apiKey := &service.APIKey{ID: 5, MemberID: &memberID, Member: member}
	return &enterpriseMemberGroupPlan{
		apiKey:  apiKey,
		current: -1,
		candidates: []enterpriseMemberGroupCandidate{
			{group: service.Group{ID: 11, Platform: service.PlatformOpenAI, RateMultiplier: 1, Hydrated: true}, memberIndex: 0},
			{group: service.Group{ID: 22, Platform: service.PlatformAnthropic, RateMultiplier: 1.2, Hydrated: true}, memberIndex: 1},
		},
	}
}
