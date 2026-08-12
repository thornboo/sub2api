package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIGatewayHandlerResponses_GrokPassiveImageToolDeclarationBypassesPermissionGate(t *testing.T) {
	body := `{"model":"grok-4.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto","input":"write code"}`
	rec := runOpenAIResponsesImagePermissionGateTest(t, service.PlatformGrok, body)

	require.NotEqual(t, http.StatusForbidden, rec.Code)
	require.NotContains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
}

func TestOpenAIGatewayHandlerResponses_GrokResponsesLiteImageToolDeclarationBypassesPermissionGate(t *testing.T) {
	body := `{"model":"grok-4.5","tool_choice":"auto","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]},{"type":"message","role":"user","content":"write code"}]}`
	rec := runOpenAIResponsesImagePermissionGateTest(t, service.PlatformGrok, body)

	require.NotEqual(t, http.StatusForbidden, rec.Code)
	require.NotContains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
}

func TestOpenAIGatewayHandlerResponses_ImagePermissionHardSignalsStillRejected(t *testing.T) {
	tests := []struct {
		name     string
		platform string
		body     string
	}{
		{
			name:     "Grok native image_generation declaration",
			platform: service.PlatformGrok,
			body:     `{"model":"grok-4.5","tools":[{"type":"image_generation"}],"input":"draw"}`,
		},
		{
			name:     "Grok explicit image_gen tool choice",
			platform: service.PlatformGrok,
			body:     `{"model":"grok-4.5","tools":[{"type":"namespace","name":"image_gen"}],"tool_choice":{"type":"namespace","name":"image_gen"},"input":"draw"}`,
		},
		{
			name:     "OpenAI native image_generation tool",
			platform: service.PlatformOpenAI,
			body:     `{"model":"gpt-5.5","tools":[{"type":"image_generation","model":"gpt-image-2"}],"input":"draw a cat"}`,
		},
		{
			name:     "OpenAI image model",
			platform: service.PlatformOpenAI,
			body:     `{"model":"gpt-image-2","input":"draw a cat"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := runOpenAIResponsesImagePermissionGateTest(t, tt.platform, tt.body)

			require.Equal(t, http.StatusForbidden, rec.Code)
			require.Contains(t, rec.Body.String(), service.ImageGenerationPermissionMessage())
		})
	}
}

func TestOpenAIGatewayHandlerResponses_PassiveNamespaceDoesNotTrigger403(t *testing.T) {
	passiveNamespace := `{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto","input":"write code"}`
	rec := runOpenAIResponsesImagePermissionGateTest(t, service.PlatformOpenAI, passiveNamespace)

	require.NotEqual(t, http.StatusForbidden, rec.Code,
		"passive image_gen namespace with tool_choice=auto should not trigger 403 (#4447)")
}

func TestOpenAIGatewayHandlerResponses_ImagePermissionGateMarksOnlyEnterpriseMemberRetry(t *testing.T) {
	body := `{"model":"gpt-image-2","input":"draw a cat"}`

	ordinaryRecorder, ordinaryContext := runOpenAIResponsesImagePermissionGateContextTest(t, service.PlatformOpenAI, body, false)
	require.Equal(t, http.StatusForbidden, ordinaryRecorder.Code)
	_, ordinaryRetry := service.GroupAttemptResultFromContext(ordinaryContext)
	require.False(t, ordinaryRetry, "ordinary keys must not gain enterprise cross-group replay state")

	memberRecorder, memberContext := runOpenAIResponsesImagePermissionGateContextTest(t, service.PlatformOpenAI, body, true)
	require.Equal(t, http.StatusForbidden, memberRecorder.Code)
	attempt, memberRetry := service.GroupAttemptResultFromContext(memberContext)
	require.True(t, memberRetry)
	require.Equal(t, service.OpsGroupRetryReasonCapabilityMismatch, attempt.Reason)
}

func TestOpenAIGatewayHandlerResponses_ImagePermissionGateKeepsSafeShadowFallback(t *testing.T) {
	body := `{"model":"gpt-image-2","input":"draw a cat"}`

	recorder, requestContext := runOpenAIResponsesImagePermissionGateContextTest(t, service.PlatformOpenAI, body, true, false)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	attempt, retryMarked := service.GroupAttemptResultFromContext(requestContext)
	require.True(t, retryMarked, "shadow planning must not suppress the legacy safe fallback contract")
	require.Equal(t, service.OpsGroupRetryReasonCapabilityMismatch, attempt.Reason)
}

func runOpenAIResponsesImagePermissionGateTest(t *testing.T, platform string, body string) *httptest.ResponseRecorder {
	recorder, _ := runOpenAIResponsesImagePermissionGateContextTest(t, platform, body, false)
	return recorder
}

func runOpenAIResponsesImagePermissionGateContextTest(t *testing.T, platform string, body string, enterpriseMember bool, modelPlanAppliedOverride ...bool) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	groupID := int64(6301)
	userID := int64(6302)
	apiKey := &service.APIKey{
		ID:      6303,
		GroupID: &groupID,
		Group: &service.Group{
			ID:                   groupID,
			Platform:             platform,
			AllowImageGeneration: false,
		},
		User: &service.User{ID: userID, Status: service.StatusActive},
	}
	if enterpriseMember {
		memberID := int64(6304)
		apiKey.MemberID = &memberID
		apiKey.Member = &service.EnterpriseMember{ID: memberID, Status: service.EnterpriseMemberStatusActive}
		modelPlanApplied := true
		if len(modelPlanAppliedOverride) > 0 {
			modelPlanApplied = modelPlanAppliedOverride[0]
		}
		mode := service.EnterpriseMemberModelAdmissionEnforcePublished
		if !modelPlanApplied {
			mode = service.EnterpriseMemberModelAdmissionShadowPublished
		}
		active := &service.ActiveGroupContext{
			GroupID:          groupID,
			AttemptNumber:    1,
			RoutePlanMode:    mode,
			ModelPlanApplied: modelPlanApplied,
		}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ActiveGroup, active))
	}
	c.Set(string(middleware2.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID, Concurrency: 1})

	h := &OpenAIGatewayHandler{
		gatewayService:      &service.OpenAIGatewayService{},
		billingCacheService: service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, &config.Config{RunMode: config.RunModeSimple}, nil),
		apiKeyService:       &service.APIKeyService{},
		concurrencyHelper: &ConcurrencyHelper{concurrencyService: service.NewConcurrencyService(
			&helperConcurrencyCacheStub{userSeq: []bool{true}},
		)},
		cfg:          &config.Config{},
		imageLimiter: &imageConcurrencyLimiter{},
	}

	h.Responses(c)
	return rec, c
}
