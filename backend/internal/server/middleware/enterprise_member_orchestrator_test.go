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

func TestOrchestrateEnterpriseMemberGroupsRetriesTypedCapacityExhausted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-test"}`))

	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "gpt-test")

	var groupIDs []int64
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, apiKey.GroupID)
		groupIDs = append(groupIDs, *apiKey.GroupID)
		if len(groupIDs) == 1 {
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapacityExhausted)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "capacity exhausted"})
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

func TestOrchestrateEnterpriseMemberGroupsDoesNotReplaceCapacityFailureWithLaterCapabilityMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.6-sol"}`))

	plan := testEnterpriseMemberGroupPlan()
	plan.candidates = append(plan.candidates, enterpriseMemberGroupCandidate{
		group:       service.Group{ID: 33, Platform: service.PlatformAnthropic, RateMultiplier: 1.3, Hydrated: true},
		memberIndex: 2,
	})
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "gpt-5.6-sol")

	var groupIDs []int64
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, apiKey.GroupID)
		groupIDs = append(groupIDs, *apiKey.GroupID)
		if len(groupIDs) == 1 {
			c.Header("Retry-After", "7")
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapacityExhausted)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "published route temporarily exhausted"})
			return
		}
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
		c.JSON(http.StatusNotFound, gin.H{"error": "model is not supported by this group"})
	})

	handler(c)

	require.Equal(t, []int64{11, 22, 33}, groupIDs)
	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, "7", recorder.Header().Get("Retry-After"))
	require.JSONEq(t, `{"error":"published route temporarily exhausted"}`, recorder.Body.String())
	active, ok := service.ActiveGroupFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, int64(11), active.GroupID, "terminal routing context must match the response selected for the client")
	attempts := service.OpsRoutingAttemptsFromContext(c)
	require.Len(t, attempts, 3)
	require.Equal(t, []int64{11, 22, 33}, []int64{attempts[0].GroupID, attempts[1].GroupID, attempts[2].GroupID})
	terminal, ok := service.GroupAttemptResultFromContext(c)
	require.True(t, ok)
	require.Equal(t, int64(11), terminal.GroupID)
	require.Equal(t, service.OpsGroupRetryReasonCapacityExhausted, terminal.Reason)
}

func TestOrchestrateEnterpriseMemberGroupsDoesNotReplaceTransientFailureWithLaterCapabilityMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.6-sol"}`))

	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "gpt-5.6-sol")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		if calls == 1 {
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonTransientUpstream)
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream connection closed"})
			return
		}
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
		c.JSON(http.StatusNotFound, gin.H{"error": "model is not supported by this group"})
	})

	handler(c)

	require.Equal(t, 2, calls)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.JSONEq(t, `{"error":"upstream connection closed"}`, recorder.Body.String())
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

func TestOrchestrateEnterpriseMemberGroupsDoesNotReplayAfterSemanticSSEWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.6-sol","stream":true}`))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "gpt-5.6-sol")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		if calls > 1 {
			t.Fatal("a semantic streaming response must permanently lock the active group")
		}
		c.Header("Content-Type", "text/event-stream")
		_, err := c.Writer.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n"))
		require.NoError(t, err)
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonTransientUpstream)
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "text/event-stream", recorder.Header().Get("Content-Type"))
	require.Equal(t, "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n", recorder.Body.String())
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

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryGenericServerErrorWithoutTypedMarker(t *testing.T) {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generic upstream failure"})
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.JSONEq(t, `{"error":"generic upstream failure"}`, recorder.Body.String())
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

func TestOrchestrateEnterpriseMemberGroupsClearsAttemptLocalRoutingStateBeforeRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"public-alias"}`))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "public-alias")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		if calls == 1 {
			c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), service.CompositeRouteDecision{
				Matched:        true,
				GroupID:        11,
				PublicModel:    "public-alias",
				TargetPlatform: service.PlatformOpenAI,
				UpstreamModel:  "upstream-model",
			}))
			c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{Kind: "http_error", UpstreamResponseBody: "first upstream miss"}})
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": "first miss"})
			return
		}

		_, retryMarked := service.GroupAttemptResultFromContext(c)
		require.False(t, retryMarked, "retry marker from the previous group must not leak")
		_, platformResolved := service.ResolvedTargetPlatformFromContext(c.Request.Context())
		require.False(t, platformResolved, "composite target from the previous group must not leak")
		upstreamErrors, ok := c.Get(service.OpsUpstreamErrorsKey)
		require.True(t, ok, "historical upstream errors must remain available for Ops evidence")
		require.Len(t, upstreamErrors, 1)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	handler(c)

	require.Equal(t, 2, calls)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsRetryActivatesFullCandidateSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"public-alias"}`))
	ctx := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "req-parity")
	ctx = service.WithPrefetchedStickySession(ctx, 902, 22, true)
	c.Request = c.Request.WithContext(ctx)

	plan := testEnterpriseMemberGroupPlan()
	plan.candidates[0].group = service.Group{
		ID:                   11,
		Platform:             service.PlatformOpenAI,
		RateMultiplier:       1,
		Status:               service.StatusActive,
		Hydrated:             true,
		RequirePrivacySet:    false,
		RequireOAuthOnly:     false,
		ProfitControlEnabled: false,
	}
	plan.candidates[1].group = service.Group{
		ID:                   22,
		Platform:             service.PlatformOpenAI,
		RateMultiplier:       1.35,
		Status:               service.StatusActive,
		Hydrated:             true,
		RequirePrivacySet:    true,
		RequireOAuthOnly:     true,
		ProfitControlEnabled: true,
		ProfitMinMargin:      0.25,
		ProfitSafetyBuffer:   0.05,
		ModelRoutingEnabled:  true,
		ModelRouting:         map[string][]int64{"public-alias": []int64{902}},
	}
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "public-alias")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, requestKey.GroupID)
		if calls == 1 {
			require.Equal(t, int64(11), *requestKey.GroupID)
			c.Set("channel_pricing_temp_skip", true)
			c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), service.CompositeRouteDecision{
				Matched:        true,
				GroupID:        11,
				PublicModel:    "public-alias",
				TargetPlatform: service.PlatformGrok,
				UpstreamModel:  "grok-upstream",
			}))
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": "first miss"})
			return
		}

		require.Equal(t, int64(22), *requestKey.GroupID)
		require.NotNil(t, requestKey.Group)
		require.True(t, requestKey.Group.RequirePrivacySet)
		require.True(t, requestKey.Group.RequireOAuthOnly)
		require.True(t, requestKey.Group.ProfitControlEnabled)
		require.Equal(t, 0.25, requestKey.Group.ProfitMinMargin)
		require.Equal(t, 0.05, requestKey.Group.ProfitSafetyBuffer)
		require.Equal(t, []int64{902}, requestKey.Group.GetRoutingAccountIDs("public-alias"))
		require.Len(t, requestKey.Member.Groups, 2)
		require.Equal(t, []int64{11, 22}, requestKey.Member.GroupIDs)
		_, tempSkipLeaked := c.Get("channel_pricing_temp_skip")
		require.False(t, tempSkipLeaked)
		_, compositeLeaked := service.ResolvedTargetPlatformFromContext(c.Request.Context())
		require.False(t, compositeLeaked)
		prefetchedAccount, ok := service.PrefetchedStickyAccountIDFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, int64(902), prefetchedAccount)
		prefetchedGroup, ok := service.PrefetchedStickyGroupIDFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, int64(22), prefetchedGroup)
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, int64(22), active.GroupID)
		require.Equal(t, "req-parity:g22:a2", active.AttemptID)
		require.Equal(t, 1, active.CandidateIndex)
		require.Equal(t, 2, active.AttemptNumber)
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	handler(c)

	require.Equal(t, 2, calls)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"ok":true}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsRetryPreservesHistoricalAttemptsAndUpstreamErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"public-alias"}`))

	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "public-alias")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		if calls == 1 {
			c.Set(service.OpsUpstreamErrorsKey, []*service.OpsUpstreamErrorEvent{{Kind: "http_error", UpstreamResponseBody: "first miss"}})
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": "first miss"})
			return
		}

		_, retryMarked := service.GroupAttemptResultFromContext(c)
		require.False(t, retryMarked)
		upstreamErrors, ok := c.Get(service.OpsUpstreamErrorsKey)
		require.True(t, ok)
		require.Len(t, upstreamErrors, 1)
		attempts := service.OpsRoutingAttemptsFromContext(c)
		require.Len(t, attempts, 1, "historical routing attempts must remain available for Ops evidence")
		require.Equal(t, service.OpsRoutingAttemptStageActualAttempt, attempts[0].Stage)
		require.Equal(t, int64(11), attempts[0].GroupID)
		require.Equal(t, 1, attempts[0].AttemptNumber)
		require.Equal(t, string(service.GroupAttemptOutcomeTerminalFailure), attempts[0].Outcome)
		require.Equal(t, string(service.OpsGroupRetryReasonCapabilityMismatch), attempts[0].Reason)
		require.NotNil(t, attempts[0].SafeToReplay)
		require.True(t, *attempts[0].SafeToReplay)
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

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryExternalTaskAttempt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-test"}`))
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "gpt-image-test")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
		service.MarkEnterpriseMemberExternalTaskCommitted(c)
		c.JSON(http.StatusNotFound, gin.H{"error": "task already created"})
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.JSONEq(t, `{"error":"task already created"}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryCommittedWebSocketTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	plan := testEnterpriseMemberGroupPlan()
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "gpt-realtime-test")

	calls := 0
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		calls++
		service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonTransientUpstream)
		service.MarkEnterpriseMemberWSTurnCommitted(c)
		c.JSON(http.StatusBadGateway, gin.H{"error": "ws turn committed"})
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.JSONEq(t, `{"error":"ws turn committed"}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsDoesNotRetryCancelledClient(t *testing.T) {
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
		ctx, cancel := context.WithCancel(c.Request.Context())
		cancel()
		c.Request = c.Request.WithContext(ctx)
		c.JSON(http.StatusBadGateway, gin.H{"error": "client cancelled"})
	})

	handler(c)

	require.Equal(t, 1, calls)
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.JSONEq(t, `{"error":"client cancelled"}`, recorder.Body.String())
}

func TestOrchestrateEnterpriseMemberGroupsAttemptsEachGroupAtMostOnce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-test"}`))

	plan := testEnterpriseMemberGroupPlan()
	plan.candidates = []enterpriseMemberGroupCandidate{
		plan.candidates[0],
		{group: service.Group{ID: 11, Platform: service.PlatformOpenAI, RateMultiplier: 1, Hydrated: true}, memberIndex: 1},
		{group: service.Group{ID: 33, Platform: service.PlatformAnthropic, RateMultiplier: 1.3, Hydrated: true}, memberIndex: 2},
	}
	c.Set(enterpriseMemberGroupPlanKey, plan)
	activateEnterpriseMemberGroupCandidate(c, plan, 0, "claude-test")

	var groupIDs []int64
	handler := OrchestrateEnterpriseMemberGroups(func(c *gin.Context) {
		apiKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotNil(t, apiKey.GroupID)
		groupIDs = append(groupIDs, *apiKey.GroupID)
		if len(groupIDs) == 1 {
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": "first miss"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"group_id": *apiKey.GroupID})
	})

	handler(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"group_id":33}`, recorder.Body.String())
	require.Equal(t, []int64{11, 33}, groupIDs)
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
