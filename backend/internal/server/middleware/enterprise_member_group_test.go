package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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
	router.Use(ResolveEnterpriseMemberGroup(nil, &config.Config{RunMode: config.RunModeSimple}, AnthropicErrorWriter))
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		requestKey, ok := GetAPIKeyFromContext(c)
		require.True(t, ok)
		require.NotSame(t, key, requestKey)
		require.Equal(t, int64(12), *requestKey.GroupID)
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

func TestEnterpriseMemberGroupEligibleUsesBatchAndWebSocketCapabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	user := &service.User{ID: 3, Balance: 10}

	batchContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	batchContext.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", strings.NewReader(`{"model":"imagen"}`))
	geminiBatch := &service.Group{ID: 1, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true, AllowBatchImageGeneration: true}
	geminiDisabled := &service.Group{ID: 2, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true, AllowImageGeneration: true}
	openAI := &service.Group{ID: 3, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, AllowBatchImageGeneration: true}
	require.True(t, enterpriseMemberGroupEligible(batchContext, user, geminiBatch, "imagen"))
	require.False(t, enterpriseMemberGroupEligible(batchContext, user, geminiDisabled, "imagen"))
	require.False(t, enterpriseMemberGroupEligible(batchContext, user, openAI, "imagen"))

	wsContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	wsContext.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	require.True(t, enterpriseMemberGroupEligible(wsContext, user, openAI, ""))
	require.False(t, enterpriseMemberGroupEligible(wsContext, user, geminiBatch, ""))
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
	require.False(t, enterpriseMemberGroupEligible(images, user, openAIImagesDisabled, "gpt-image-1"))
	require.True(t, enterpriseMemberGroupEligible(images, user, openAIImagesEnabled, "gpt-image-1"))

	grokVideoDisabled := activeGroup(service.PlatformGrok)
	grokVideoEnabled := activeGroup(service.PlatformGrok)
	grokVideoEnabled.AllowImageGeneration = true
	videos := testContext(http.MethodPost, "/v1/videos/generations")
	require.False(t, enterpriseMemberGroupEligible(videos, user, grokVideoDisabled, "grok-imagine-video"))
	require.True(t, enterpriseMemberGroupEligible(videos, user, grokVideoEnabled, "grok-imagine-video"))

	openAIMessagesDisabled := activeGroup(service.PlatformOpenAI)
	openAIMessagesEnabled := activeGroup(service.PlatformOpenAI)
	openAIMessagesEnabled.AllowMessagesDispatch = true
	messages := testContext(http.MethodPost, "/v1/messages")
	require.False(t, enterpriseMemberGroupEligible(messages, user, openAIMessagesDisabled, "gpt-5"))
	require.True(t, enterpriseMemberGroupEligible(messages, user, openAIMessagesEnabled, "gpt-5"))

	embeddings := testContext(http.MethodPost, "/v1/embeddings")
	require.True(t, enterpriseMemberGroupEligible(embeddings, user, activeGroup(service.PlatformOpenAI), "text-embedding-3-large"))
	require.False(t, enterpriseMemberGroupEligible(embeddings, user, activeGroup(service.PlatformGrok), "text-embedding-3-large"))

	gemini := testContext(http.MethodPost, "/v1beta/models/gemini-2.5-pro:generateContent")
	require.True(t, enterpriseMemberGroupEligible(gemini, user, activeGroup(service.PlatformGemini), "gemini-2.5-pro"))
	require.True(t, enterpriseMemberGroupEligible(gemini, user, activeGroup(service.PlatformAntigravity), "gemini-2.5-pro"))
	require.False(t, enterpriseMemberGroupEligible(gemini, user, activeGroup(service.PlatformOpenAI), "gemini-2.5-pro"))
}

func TestActivateEnterpriseMemberGroupForModelUsesFirstMatchingSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(8)
	key := &service.APIKey{ID: 17, UserID: 3, MemberID: &memberID, Member: &service.EnterpriseMember{ID: memberID, Version: 2}}
	first := service.Group{ID: 11, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"gpt-4o"}}}
	second := service.Group{ID: 12, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true, ModelsListConfig: service.GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5"}}}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	plan := &enterpriseMemberGroupPlan{apiKey: key, current: 0, candidates: []enterpriseMemberGroupCandidate{{group: first}, {group: second}}}
	c.Set(enterpriseMemberGroupPlanKey, plan)

	require.True(t, ActivateEnterpriseMemberGroupForModel(c, "gpt-5"))
	requestKey, ok := GetAPIKeyFromContext(c)
	require.True(t, ok)
	require.Equal(t, int64(12), *requestKey.GroupID)
	active, ok := service.ActiveGroupFromContext(c.Request.Context())
	require.True(t, ok)
	require.Equal(t, "gpt-5", active.RequestedModel)
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
