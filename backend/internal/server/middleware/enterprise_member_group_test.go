package middleware

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberBudgetMiddlewareRepo struct {
	service.EnterpriseMemberBudgetRepository
	reservedRequestID  string
	markedRequestID    string
	markedReason       string
	releasedRequestIDs []string
	reserveErr         error
}

func (r *enterpriseMemberBudgetMiddlewareRepo) Reserve(_ context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, expiresAt time.Time) (*service.EnterpriseMemberBudgetReservation, error) {
	r.reservedRequestID = requestID
	if r.reserveErr != nil {
		return nil, r.reserveErr
	}
	return &service.EnterpriseMemberBudgetReservation{
		ID:          1,
		RequestID:   requestID,
		MemberID:    memberID,
		GroupID:     groupID,
		PayloadHash: payloadHash,
		ReservedUSD: amount,
		ExpiresAt:   expiresAt,
		Status:      "reserved",
	}, nil
}

func TestEnforceEnterpriseMemberBudgetExplainsAsyncTaskHoldRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{
		ID: 17, UserID: 3, MemberID: &memberID,
		Member: &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive},
	}
	repo := &enterpriseMemberBudgetMiddlewareRepo{reserveErr: service.ErrEnterpriseMemberAsyncBudgetUnavailable.WithMetadata(map[string]string{
		"limit_window": "monthly", "limit_usd": "300.000000", "settled_used_usd": "39.640000",
		"active_task_holds_usd": "253.380000", "requested_task_hold_usd": "20.000000",
	})}
	budgetService := service.NewEnterpriseMemberBudgetService(repo, nil, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "request-1"))
		c.Next()
	})
	router.Use(EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, AnthropicErrorWriter))
	router.POST("/v1/images/generations/async", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	request := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-1","prompt":"cat"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusTooManyRequests, response.Code)
	require.Equal(t, "ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE", response.Header().Get("X-Sub2API-Error-Code"))
	require.Equal(t, "253.380000", response.Header().Get("X-Sub2API-Budget-Active-Holds-USD"))
	require.Equal(t, "20.000000", response.Header().Get("X-Sub2API-Budget-Requested-Hold-USD"))
	require.Contains(t, response.Body.String(), `"code":"ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE"`)
	require.Contains(t, response.Body.String(), `"metadata":`)
	require.Contains(t, response.Body.String(), `"active_task_holds_usd":"253.380000"`)
	require.Contains(t, response.Body.String(), "settled usage US$39.640000")
	require.Contains(t, response.Body.String(), "active task holds US$253.380000")
	require.NotContains(t, response.Body.String(), "metadata=map")
}

// TestEnforceEnterpriseMemberBudgetReportsUnclassifiedFailureAsPlatformFault
// pins the incident signature: an infrastructure failure inside the reservation
// transaction must not be reported as a client error, and must never surface the
// bare infraerrors.UnknownMessage fallback to the caller.
func TestEnforceEnterpriseMemberBudgetReportsUnclassifiedFailureAsPlatformFault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		reserveErr error
	}{
		{"connection pool exhausted", sql.ErrConnDone},
		{"statement deadline exceeded", context.DeadlineExceeded},
		{"opaque driver failure", errors.New("pq: sorry, too many clients already")},
		{"wrapped begin transaction failure", fmt.Errorf("begin reservation tx: %w", sql.ErrConnDone)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			memberID := int64(8)
			key := &service.APIKey{
				ID: 17, UserID: 3, MemberID: &memberID,
				Member: &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive},
			}
			repo := &enterpriseMemberBudgetMiddlewareRepo{reserveErr: tc.reserveErr}
			budgetService := service.NewEnterpriseMemberBudgetService(repo, nil, nil)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyAPIKey), key)
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "request-1"))
				c.Next()
			})
			router.Use(EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, AnthropicErrorWriter))
			handlerReached := false
			router.POST("/v1/responses", func(c *gin.Context) {
				handlerReached = true
				c.Status(http.StatusNoContent)
			})

			request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hi"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.False(t, handlerReached, "request must abort in the middleware, matching the incident's NULL request_type")
			require.Equal(t, http.StatusInternalServerError, response.Code,
				"an infrastructure failure must be attributed to the platform, not the client")
			require.NotContains(t, response.Body.String(), "internal error",
				"the infraerrors.UnknownMessage fallback must not leak to clients")
		})
	}
}

// TestEnforceEnterpriseMemberBudgetKeepsClassifiedFailuresUnchanged guards the
// blast radius of the status remap: errors that carry a domain reason must keep
// the status and message they had before.
func TestEnforceEnterpriseMemberBudgetKeepsClassifiedFailuresUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name       string
		reserveErr error
		wantStatus int
		wantCode   string
	}{
		{"unbounded request", service.ErrEnterpriseMemberBudgetUnbounded, http.StatusBadRequest, "ENTERPRISE_MEMBER_BUDGET_UNBOUNDED_REQUEST"},
		{"request id conflict", service.ErrEnterpriseMemberBudgetConflict, http.StatusBadRequest, "ENTERPRISE_MEMBER_BUDGET_REQUEST_CONFLICT"},
		{"budget exhausted", service.ErrEnterpriseMemberBudgetExceeded, http.StatusTooManyRequests, "ENTERPRISE_MEMBER_BUDGET_EXCEEDED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			memberID := int64(8)
			key := &service.APIKey{
				ID: 17, UserID: 3, MemberID: &memberID,
				Member: &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive},
			}
			repo := &enterpriseMemberBudgetMiddlewareRepo{reserveErr: tc.reserveErr}
			budgetService := service.NewEnterpriseMemberBudgetService(repo, nil, nil)
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(ContextKeyAPIKey), key)
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "request-1"))
				c.Next()
			})
			router.Use(EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, AnthropicErrorWriter))
			router.POST("/v1/responses", func(c *gin.Context) { c.Status(http.StatusNoContent) })

			request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hi"}`))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, tc.wantStatus, response.Code)
			require.Equal(t, tc.wantCode, response.Header().Get(gatewayErrorCodeHeader))
		})
	}
}

func TestGoogleErrorWriterExposesStructuredBudgetDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Header(gatewayErrorCodeHeader, "ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE")
	c.Header(gatewayBudgetMetadataHeaders["limit_window"], "monthly")
	c.Header(gatewayBudgetMetadataHeaders["active_task_holds_usd"], "253.380000")

	GoogleErrorWriter(c, http.StatusTooManyRequests, "Asynchronous task budget is unavailable")

	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Contains(t, w.Body.String(), `"reason":"ENTERPRISE_MEMBER_ASYNC_BUDGET_UNAVAILABLE"`)
	require.Contains(t, w.Body.String(), `"limit_window":"monthly"`)
	require.Contains(t, w.Body.String(), `"active_task_holds_usd":"253.380000"`)
}

func (r *enterpriseMemberBudgetMiddlewareRepo) MarkAmbiguous(_ context.Context, requestID, outcomeReason string) error {
	r.markedRequestID = requestID
	r.markedReason = outcomeReason
	return nil
}

func (r *enterpriseMemberBudgetMiddlewareRepo) Release(_ context.Context, requestID string) error {
	r.releasedRequestIDs = append(r.releasedRequestIDs, requestID)
	return nil
}

func TestResolveEnterpriseMemberGroupSelectsOrderedEligibleGroupAndReplaysBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{
		ID: 17, UserID: 3, MemberID: &memberID,
		User: &service.User{ID: 3, Role: service.RoleUser, AccountType: service.UserAccountTypeEnterprise, Status: service.StatusActive, Balance: 10},
		Member: &service.EnterpriseMember{
			ID: 8, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive, Version: 4,
			Groups: []service.Group{
				{ID: 11, Platform: service.PlatformAnthropic, Status: service.StatusDisabled, Hydrated: true},
				{ID: 12, Name: "primary", Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, RateMultiplier: 1.2},
			},
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		c.Next()
	})
	router.Use(ResolveEnterpriseMemberGroup(nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}, AnthropicErrorWriter))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotSame(t, key, requestKey)
		require.Equal(t, int64(12), *requestKey.GroupID)
		require.NotSame(t, key.Member, requestKey.Member)
		require.Len(t, requestKey.Member.Groups, 1, "request snapshot must expose only currently authorized candidates")
		require.Equal(t, int64(12), requestKey.Member.Groups[0].ID)
		active, ok := service.ActiveGroupFromContext(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, int64(12), active.GroupID)
		require.Equal(t, "gpt-5", active.RequestedModel)
		var body map[string]any
		require.NoError(t, json.NewDecoder(c.Request.Body).Decode(&body))
		require.Equal(t, "gpt-5", body["model"])
		c.Status(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","messages":[]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusNoContent, response.Code)
	require.Nil(t, key.GroupID, "cached key must remain immutable")
}

func TestValidateEnterpriseMemberAPIKeyFailsClosedForDisabledMember(t *testing.T) {
	memberID := int64(8)
	key := &service.APIKey{
		UserID: 3, MemberID: &memberID,
		User:   &service.User{ID: 3, Role: service.RoleUser, AccountType: service.UserAccountTypeEnterprise, Status: service.StatusActive},
		Member: &service.EnterpriseMember{ID: 8, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusDisabled},
	}
	code, _, valid := validateEnterpriseMemberAPIKey(key)
	require.False(t, valid)
	require.Equal(t, "ENTERPRISE_MEMBER_DISABLED", code)
}

func TestEnterpriseMemberBudgetRequiredIncludesRateOnlyLimits(t *testing.T) {
	memberID := int64(8)
	key := &service.APIKey{
		MemberID: &memberID,
		Member: &service.EnterpriseMember{
			ID:          memberID,
			RateLimit5h: 25,
		},
	}

	require.True(t, enterpriseMemberBudgetRequired(key), "every member request must create a durable zero or non-zero receipt")
	key.Member.RateLimit5h = 0
	require.True(t, enterpriseMemberBudgetRequired(key), "unlimited members still need a zero-amount request receipt")
	key.MemberID = nil
	require.False(t, enterpriseMemberBudgetRequired(key))
}

func TestEnforceEnterpriseMemberBudgetKeepsAmbiguousOutcomeReservedEvenAfterSuccessStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{
		ID:       17,
		UserID:   3,
		MemberID: &memberID,
		Member:   &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive},
	}
	repo := &enterpriseMemberBudgetMiddlewareRepo{}
	budgetService := service.NewEnterpriseMemberBudgetService(repo, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		requestContext := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "request-1")
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
	})
	router.Use(EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, AnthropicErrorWriter))
	router.POST("/v1/videos/generations", func(c *gin.Context) {
		service.MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "task_persistence_failed")
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/videos/generations", strings.NewReader(`{"model":"grok-imagine-video"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "17:client:request-1", repo.reservedRequestID)
	require.Equal(t, "17:client:request-1", repo.markedRequestID)
	require.Equal(t, "task_persistence_failed", repo.markedReason)
	require.Empty(t, repo.releasedRequestIDs, "ambiguous upstream side effects must keep their reservation until reconciliation")
}

func TestEnforceEnterpriseMemberBudgetReleasesDefinitiveStreamFailureAfterSuccessStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{
		ID:       17,
		UserID:   3,
		MemberID: &memberID,
		Member:   &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive},
	}
	repo := &enterpriseMemberBudgetMiddlewareRepo{}
	budgetService := service.NewEnterpriseMemberBudgetService(repo, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		requestContext := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "request-1")
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
	})
	router.Use(EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, AnthropicErrorWriter))
	router.POST("/v1/responses", func(c *gin.Context) {
		service.MarkEnterpriseMemberBudgetDefinitiveFailureWithReason(c, "upstream_terminal_failure")
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hi","stream":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code, "SSE response headers may already be committed as 200")
	require.Equal(t, "17:client:request-1", repo.reservedRequestID)
	require.Equal(t, []string{"17:client:request-1"}, repo.releasedRequestIDs)
	require.Empty(t, repo.markedRequestID)
}

func TestEnforceEnterpriseMemberBudgetKeepsAmbiguousOutcomeOverDefinitiveFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{
		ID:       17,
		UserID:   3,
		MemberID: &memberID,
		Member:   &service.EnterpriseMember{ID: memberID, EnterpriseUserID: 3, Status: service.EnterpriseMemberStatusActive},
	}
	repo := &enterpriseMemberBudgetMiddlewareRepo{}
	budgetService := service.NewEnterpriseMemberBudgetService(repo, nil, nil)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(ContextKeyAPIKey), key)
		requestContext := context.WithValue(c.Request.Context(), ctxkey.ClientRequestID, "request-1")
		c.Request = c.Request.WithContext(requestContext)
		c.Next()
	})
	router.Use(EnforceEnterpriseMemberBudget(budgetService, &config.Config{RunMode: config.RunModeStandard}, AnthropicErrorWriter))
	router.POST("/v1/responses", func(c *gin.Context) {
		service.MarkEnterpriseMemberBudgetDefinitiveFailureWithReason(c, "upstream_terminal_failure")
		service.MarkEnterpriseMemberBudgetOutcomeAmbiguousWithReason(c, "usage_persistence_failed")
		c.Status(http.StatusOK)
	})

	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5.5","input":"hi","stream":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "17:client:request-1", repo.markedRequestID)
	require.Equal(t, "usage_persistence_failed", repo.markedReason)
	require.Empty(t, repo.releasedRequestIDs, "unknown accounting outcome must take precedence over a terminal upstream failure")
}

func TestEnterpriseMemberGroupEligibleUsesBatchAndWebSocketCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 3, Balance: 10}

	batchContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	batchContext.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", strings.NewReader(`{"model":"imagen"}`))
	geminiBatch := &service.Group{ID: 1, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true, AllowBatchImageGeneration: true}
	geminiDisabled := &service.Group{ID: 2, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	openAI := &service.Group{ID: 3, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowBatchImageGeneration: true}
	require.True(t, enterpriseMemberGroupEligible(batchContext, user, geminiBatch))
	require.False(t, enterpriseMemberGroupEligible(batchContext, user, geminiDisabled))
	require.False(t, enterpriseMemberGroupEligible(batchContext, user, openAI))

	wsContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	wsContext.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	require.True(t, enterpriseMemberGroupEligible(wsContext, user, openAI))
	require.False(t, enterpriseMemberGroupEligible(wsContext, user, geminiBatch))
}

func TestEnterpriseMemberGroupEligibleEnforcesEndpointCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 3, Balance: 10}
	activeGroup := func(platform string) *service.Group {
		return &service.Group{ID: 1, Platform: platform, Status: service.StatusActive, Hydrated: true}
	}
	testContext := func(method, requestPath string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(method, requestPath, nil)
		return c
	}

	openAIImagesDisabled := activeGroup(service.PlatformOpenAI)
	openAIImagesEnabled := activeGroup(service.PlatformOpenAI)
	openAIImagesEnabled.AllowImageGeneration = true
	images := testContext(http.MethodPost, "/v1/images/generations")
	require.False(t, enterpriseMemberGroupEligible(images, user, openAIImagesDisabled))
	require.True(t, enterpriseMemberGroupEligible(images, user, openAIImagesEnabled))

	grokVideoDisabled := activeGroup(service.PlatformGrok)
	grokVideoEnabled := activeGroup(service.PlatformGrok)
	grokVideoEnabled.AllowImageGeneration = true
	videos := testContext(http.MethodPost, "/v1/videos/generations")
	require.False(t, enterpriseMemberGroupEligible(videos, user, grokVideoDisabled))
	require.True(t, enterpriseMemberGroupEligible(videos, user, grokVideoEnabled))

	openAIMessagesDisabled := activeGroup(service.PlatformOpenAI)
	openAIMessagesEnabled := activeGroup(service.PlatformOpenAI)
	openAIMessagesEnabled.AllowMessagesDispatch = true
	messages := testContext(http.MethodPost, "/v1/messages")
	require.False(t, enterpriseMemberGroupEligible(messages, user, openAIMessagesDisabled))
	require.True(t, enterpriseMemberGroupEligible(messages, user, openAIMessagesEnabled))

	embeddings := testContext(http.MethodPost, "/v1/embeddings")
	require.True(t, enterpriseMemberGroupEligible(embeddings, user, activeGroup(service.PlatformOpenAI)))
	require.False(t, enterpriseMemberGroupEligible(embeddings, user, activeGroup(service.PlatformGrok)))

	alphaSearch := testContext(http.MethodPost, "/v1/alpha/search")
	require.True(t, enterpriseMemberGroupEligible(alphaSearch, user, activeGroup(service.PlatformOpenAI)))
	require.False(t, enterpriseMemberGroupEligible(alphaSearch, user, activeGroup(service.PlatformGrok)))

	gemini := testContext(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent")
	require.True(t, enterpriseMemberGroupEligible(gemini, user, activeGroup(service.PlatformGemini)))
	require.True(t, enterpriseMemberGroupEligible(gemini, user, activeGroup(service.PlatformAntigravity)))
	require.False(t, enterpriseMemberGroupEligible(gemini, user, activeGroup(service.PlatformOpenAI)))
}

func TestEnterpriseMemberGroupEligibleIgnoresDisplayOnlyModelsList(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 3, Balance: 10}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"claude-opus-4-8"}`))
	group := &service.Group{
		ID:                    11,
		Platform:              service.PlatformOpenAI,
		Status:                service.StatusActive,
		Hydrated:              true,
		AllowMessagesDispatch: true,
		ModelsListConfig: service.GroupModelsListConfig{
			Enabled: true,
			Models:  []string{"gpt-4o"},
		},
	}

	require.True(t, enterpriseMemberGroupEligible(c, user, group),
		"the custom /v1/models response list must not become a runtime scheduling authority")
}

func TestActivateEnterpriseMemberGroupForModelUsesFirstModelEligibleCandidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{ID: 17, UserID: 3, MemberID: &memberID, Member: &service.EnterpriseMember{ID: memberID, Version: 2}}
	first := service.Group{ID: 11, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"gpt-4o"}}}
	second := service.Group{ID: 12, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5"}}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	legacyCandidates := []enterpriseMemberGroupCandidate{{group: first, memberIndex: 0}, {group: second, memberIndex: 1}}
	snapshotAgeMs := int64(2500)
	plan := &enterpriseMemberGroupPlan{
		apiKey:           key,
		current:          0,
		legacyCandidates: legacyCandidates,
		candidates:       append([]enterpriseMemberGroupCandidate(nil), legacyCandidates...),
		planner: enterpriseMemberRoutePlannerStub{plan: &service.EnterpriseMemberRoutePlan{
			Model:  "gpt-5",
			Source: service.EnterpriseMemberRoutePlanSourceLastKnownGood,
			Candidates: []service.EnterpriseMemberRouteCandidateDecision{{
				GroupID:                12,
				Group:                  &second,
				Reason:                 service.EnterpriseMemberRouteReasonEligible,
				RoutePlanSnapshotAgeMs: &snapshotAgeMs,
			}},
		}},
		admissionMode: service.EnterpriseMemberModelAdmissionEnforcePublished,
	}
	c.Set(enterpriseMemberGroupPlanKey, plan)

	require.True(t, ActivateEnterpriseMemberGroupForModel(c, "gpt-5"))
	requestKey, ok := GetAPIKeyFromContext(c)
	require.True(t, ok)
	require.Equal(t, int64(12), *requestKey.GroupID, "only the planner-qualified candidate may activate")
	require.Len(t, requestKey.Member.Groups, 1)
	require.Equal(t, int64(12), requestKey.Member.Groups[0].ID)
	active, ok := service.ActiveGroupFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, "gpt-5", active.RequestedModel)
	require.Equal(t, 1, active.CandidateIndex, "the active context must preserve the original member binding index")
	require.Equal(t, service.EnterpriseMemberRoutePlanSourceLastKnownGood, active.RoutePlanSource)
	require.NotNil(t, active.RoutePlanSnapshotAgeMs)
	require.Equal(t, snapshotAgeMs, *active.RoutePlanSnapshotAgeMs)
}

type enterpriseMemberRoutePlannerStub struct {
	plan *service.EnterpriseMemberRoutePlan
	err  error
}

func (s enterpriseMemberRoutePlannerStub) Plan(context.Context, service.EnterpriseMemberRouteInput) (*service.EnterpriseMemberRoutePlan, error) {
	return s.plan, s.err
}

func TestActivateEnterpriseMemberGroupByIDRestoresOnlyAuthorizedCandidate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{ID: 17, UserID: 3, MemberID: &memberID, Member: &service.EnterpriseMember{ID: memberID, Version: 2}}
	first := service.Group{ID: 11, Platform: service.PlatformGrok, Status: service.StatusActive, Hydrated: true}
	second := service.Group{ID: 12, Platform: service.PlatformGrok, Status: service.StatusActive, Hydrated: true}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/video-123", nil)
	plan := &enterpriseMemberGroupPlan{apiKey: key, current: 0, candidates: []enterpriseMemberGroupCandidate{{group: first}, {group: second}}}
	c.Set(enterpriseMemberGroupPlanKey, plan)

	require.True(t, ActivateEnterpriseMemberGroupByID(c, 12))
	requestKey, ok := GetAPIKeyFromContext(c)
	require.True(t, ok)
	require.Equal(t, int64(12), *requestKey.GroupID)
	require.False(t, ActivateEnterpriseMemberGroupByID(c, 99), "revoked or unrelated groups must fail closed")
}
