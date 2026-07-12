package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
			service.MarkOpsGroupFailoverEligible(c)
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
		service.MarkOpsGroupFailoverEligible(c)
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

func testEnterpriseMemberGroupPlan() *enterpriseMemberGroupPlan {
	memberID := int64(7)
	member := &service.EnterpriseMember{ID: memberID, Version: 3}
	apiKey := &service.APIKey{ID: 5, MemberID: &memberID, Member: member}
	return &enterpriseMemberGroupPlan{
		apiKey:  apiKey,
		current: -1,
		candidates: []enterpriseMemberGroupCandidate{
			{group: service.Group{ID: 11, Platform: service.PlatformAnthropic, RateMultiplier: 1, Hydrated: true}, memberIndex: 0},
			{group: service.Group{ID: 22, Platform: service.PlatformOpenAI, RateMultiplier: 1.2, Hydrated: true}, memberIndex: 1},
		},
	}
}
