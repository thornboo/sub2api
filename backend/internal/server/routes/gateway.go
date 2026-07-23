package routes

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// RegisterGatewayRoutes 注册 API 网关路由（Claude/OpenAI/Gemini 兼容）
func RegisterGatewayRoutes(
	r *gin.Engine,
	h *handler.Handlers,
	apiKeyAuth middleware.APIKeyAuthMiddleware,
	apiKeyService *service.APIKeyService,
	memberBudgetService *service.EnterpriseMemberBudgetService,
	subscriptionService *service.SubscriptionService,
	opsService *service.OpsService,
	settingService *service.SettingService,
	compositeResolver *service.CompositeRouteResolver,
	cfg *config.Config,
) {
	bodyLimit := middleware.RequestBodyLimit(cfg.Gateway.MaxBodySize)
	textBodyLimit := middleware.RequestBodyLimit(cfg.Gateway.TextMaxBodySize)
	clientRequestID := middleware.ClientRequestID()
	opsErrorLogger := handler.OpsErrorLoggerMiddleware(opsService)
	endpointNorm := handler.InboundEndpointMiddleware()

	// 未分组 Key 拦截中间件（按协议格式区分错误响应）
	requireGroupAnthropic := middleware.RequireGroupAssignment(settingService, middleware.AnthropicErrorWriter)
	requireGroupGoogle := middleware.RequireGroupAssignment(settingService, middleware.GoogleErrorWriter)
	resolveMemberGroupAnthropic := middleware.ResolveEnterpriseMemberGroup(subscriptionService, cfg, middleware.AnthropicErrorWriter)
	resolveMemberGroupGoogle := middleware.ResolveEnterpriseMemberGroup(subscriptionService, cfg, middleware.GoogleErrorWriter)
	enforceMemberBudgetAnthropic := middleware.EnforceEnterpriseMemberBudget(memberBudgetService, cfg, middleware.AnthropicErrorWriter)
	enforceMemberBudgetGoogle := middleware.EnforceEnterpriseMemberBudget(memberBudgetService, cfg, middleware.GoogleErrorWriter)
	orchestrateMemberGroups := middleware.OrchestrateEnterpriseMemberGroups
	withCompositeMemberGroups := func(next gin.HandlerFunc) gin.HandlerFunc {
		return orchestrateMemberGroups(compositeTargetPlatformHandler(compositeResolver, next))
	}
	withCompositeGeminiMemberGroups := func(next gin.HandlerFunc) gin.HandlerFunc {
		return orchestrateMemberGroups(compositeGeminiTargetPlatformHandler(compositeResolver, next))
	}
	withCompositeResolver := func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			handler.AttachCompositeRouteResolver(c, compositeResolver)
			next(c)
		}
	}

	isOpenAIResponsesCompatibleGatewayPlatform := func(c *gin.Context) bool {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI, service.PlatformGrok:
			return true
		default:
			return false
		}
	}
	isOpenAIGatewayPlatform := func(c *gin.Context) bool {
		return getGroupPlatform(c) == service.PlatformOpenAI
	}
	countTokensHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI:
			h.OpenAIGateway.CountTokens(c)
		case service.PlatformGrok:
			h.OpenAIGateway.GrokCountTokens(c)
		default:
			h.Gateway.CountTokens(c)
		}
	}
	modelsHandler := func(c *gin.Context) {
		if isOpenAIGatewayPlatform(c) && c.Query("client_version") != "" {
			h.OpenAIGateway.CodexModels(c)
			return
		}
		h.Gateway.Models(c)
	}
	memberModelsHandler := orchestrateMemberGroups(modelsHandler)
	isOpenAIOnlyEndpointGatewayPlatform := func(c *gin.Context) bool {
		return getGroupPlatform(c) == service.PlatformOpenAI
	}
	imagesHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI:
			h.OpenAIGateway.Images(c)
		case service.PlatformGrok:
			h.OpenAIGateway.GrokImages(c)
		default:
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Images API is not supported for this platform",
				},
			})
		}
	}
	videoGenerationHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoGeneration(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoStatusHandler := func(c *gin.Context) {
		// Video status requests do not carry a model, so composite groups cannot
		// be resolved by compositeTargetPlatformMiddleware. Route them through
		// the Grok handler and let scheduler/account selection enforce capacity.
		if getGroupPlatform(c) == service.PlatformGrok || getGroupPlatform(c) == service.PlatformComposite {
			h.OpenAIGateway.GrokVideoStatus(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoContentHandler := func(c *gin.Context) {
		// Video content requests do not carry a model, so composite groups cannot
		// be resolved by compositeTargetPlatformMiddleware. Route them through
		// the Grok handler just like video status lookups.
		if getGroupPlatform(c) == service.PlatformGrok || getGroupPlatform(c) == service.PlatformComposite {
			h.OpenAIGateway.GrokVideoContent(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}
	videoEditHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoEdit(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
	}
	videoExtensionHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoExtension(c)
			return
		}
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
	}
	// API网关（Claude API兼容）
	gateway := r.Group("/v1")
	gateway.Use(bodyLimit)
	gateway.Use(clientRequestID)
	gateway.Use(opsErrorLogger)
	gateway.Use(endpointNorm)
	gateway.Use(gin.HandlerFunc(apiKeyAuth))
	gateway.GET("/sub2api/billing", h.Gateway.KeyBillingInfo)
	gateway.Use(resolveMemberGroupAnthropic)
	gateway.Use(enforceMemberBudgetAnthropic)
	gateway.Use(requireGroupAnthropic)
	{
		// /v1/messages: auto-route based on group platform
		gateway.POST("/messages", withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Messages(c)
				return
			}
			h.Gateway.Messages(c)
		}))
		// /v1/messages/count_tokens: OpenAI uses Anthropic-compat bridge; other
		// OpenAI-compatible platforms retain their provider-specific behavior.
		gateway.POST("/messages/count_tokens", withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIGatewayPlatform(c) {
				h.OpenAIGateway.CountTokens(c)
				return
			}
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				countTokensHandler(c)
				return
			}
			h.Gateway.CountTokens(c)
		}))
		// Codex CLI / Codex app refresh their model picker from the provider's
		// /models endpoint with a client_version query and expect the ChatGPT
		// Codex manifest format; other clients keep the OpenAI-style list.
		gateway.GET("/models", memberModelsHandler)
		gateway.GET("/usage", h.Gateway.Usage)
		// OpenAI Responses API: auto-route based on group platform
		gateway.POST("/responses", withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		}))
		gateway.POST("/responses/*subpath", withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		}))
		gateway.POST("/alpha/search", textBodyLimit, withCompositeMemberGroups(h.OpenAIGateway.AlphaSearch))
		gateway.GET("/responses", withCompositeResolver(func(c *gin.Context) {
			h.OpenAIGateway.ResponsesWebSocket(c)
		}))
		// OpenAI Chat Completions API: auto-route based on group platform
		gateway.POST("/chat/completions", withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.ChatCompletions(c)
				return
			}
			h.Gateway.ChatCompletions(c)
		}))
		gateway.POST("/embeddings", textBodyLimit, withCompositeMemberGroups(func(c *gin.Context) {
			if !isOpenAIOnlyEndpointGatewayPlatform(c) {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Embeddings API is not supported for this platform",
					},
				})
				return
			}
			h.OpenAIGateway.Embeddings(c)
		}))
		gateway.POST("/images/generations", withCompositeMemberGroups(imagesHandler))
		gateway.POST("/images/edits", withCompositeMemberGroups(imagesHandler))
		gateway.POST("/images/generations/async", withCompositeMemberGroups(h.AsyncImage.Submit))
		gateway.POST("/images/edits/async", withCompositeMemberGroups(h.AsyncImage.Submit))
		gateway.GET("/images/tasks/:task_id", h.AsyncImage.Get)
		gateway.POST("/images/batches", withCompositeMemberGroups(h.BatchImage.Submit))
		gateway.GET("/images/batches", h.BatchImage.List)
		gateway.GET("/images/batches/models", h.BatchImage.Models)
		gateway.GET("/images/batches/:id", h.BatchImage.Get)
		gateway.GET("/images/batches/:id/items", h.BatchImage.Items)
		gateway.GET("/images/batches/:id/items/:custom_id/content", h.BatchImage.ItemContent)
		gateway.GET("/images/batches/:id/download", h.BatchImage.Download)
		gateway.POST("/images/batches/:id/cancel", h.BatchImage.Cancel)
		gateway.DELETE("/images/batches/:id", h.BatchImage.DeleteRecord)
		gateway.DELETE("/images/batches/:id/outputs", h.BatchImage.DeleteOutputs)
		gateway.POST("/videos/generations", withCompositeMemberGroups(videoGenerationHandler))
		gateway.POST("/videos/edits", withCompositeMemberGroups(videoEditHandler))
		gateway.POST("/videos/extensions", withCompositeMemberGroups(videoExtensionHandler))
		gateway.GET("/videos/:request_id", videoStatusHandler)
		gateway.GET("/videos/:request_id/content", videoContentHandler)
	}

	// Gemini 原生 API 兼容层（Gemini SDK/CLI 直连）
	gemini := r.Group("/v1beta")
	gemini.Use(bodyLimit)
	gemini.Use(clientRequestID)
	gemini.Use(opsErrorLogger)
	gemini.Use(endpointNorm)
	gemini.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	gemini.Use(resolveMemberGroupGoogle)
	gemini.Use(enforceMemberBudgetGoogle)
	gemini.Use(requireGroupGoogle)
	{
		gemini.GET("/models", orchestrateMemberGroups(h.Gateway.GeminiV1BetaListModels))
		gemini.GET("/models/:model", withCompositeGeminiMemberGroups(h.Gateway.GeminiV1BetaGetModel))
		// Gin treats ":" as a param marker, but Gemini uses "{model}:{action}" in the same segment.
		gemini.POST("/models/*modelAction", withCompositeGeminiMemberGroups(h.Gateway.GeminiV1BetaModels))
	}

	// OpenAI Responses API（不带v1前缀的别名）— auto-route based on group platform
	responsesHandler := func(c *gin.Context) {
		if isOpenAIResponsesCompatibleGatewayPlatform(c) {
			h.OpenAIGateway.Responses(c)
			return
		}
		h.Gateway.Responses(c)
	}
	commonDirect := []gin.HandlerFunc{
		bodyLimit,
		clientRequestID,
		opsErrorLogger,
		endpointNorm,
		gin.HandlerFunc(apiKeyAuth),
		resolveMemberGroupAnthropic,
		enforceMemberBudgetAnthropic,
		requireGroupAnthropic,
	}
	r.POST("/responses", append(commonDirect, withCompositeMemberGroups(responsesHandler))...)
	r.POST("/responses/*subpath", append(commonDirect, withCompositeMemberGroups(responsesHandler))...)
	r.POST("/alpha/search", append([]gin.HandlerFunc{textBodyLimit}, append(commonDirect[1:], withCompositeMemberGroups(h.OpenAIGateway.AlphaSearch))...)...)
	r.GET("/responses", append(commonDirect, withCompositeResolver(func(c *gin.Context) {
		h.OpenAIGateway.ResponsesWebSocket(c)
	}))...)
	r.GET("/models", append(commonDirect, memberModelsHandler)...)
	r.POST("/messages/count_tokens", append(commonDirect, withCompositeMemberGroups(countTokensHandler))...)

	codexDirect := r.Group("/backend-api/codex")
	codexDirect.Use(commonDirect...)
	{
		codexDirect.POST("/responses", withCompositeMemberGroups(responsesHandler))
		codexDirect.POST("/responses/*subpath", withCompositeMemberGroups(responsesHandler))
		codexDirect.POST("/alpha/search", textBodyLimit, withCompositeMemberGroups(h.OpenAIGateway.AlphaSearch))
		codexDirect.GET("/responses", withCompositeResolver(func(c *gin.Context) {
			h.OpenAIGateway.ResponsesWebSocket(c)
		}))
		codexDirect.GET("/models", orchestrateMemberGroups(h.OpenAIGateway.CodexModels))
	}
	// OpenAI Chat Completions API（不带v1前缀的别名）— auto-route based on group platform
	r.POST("/chat/completions", append(commonDirect, withCompositeMemberGroups(func(c *gin.Context) {
		if isOpenAIResponsesCompatibleGatewayPlatform(c) {
			h.OpenAIGateway.ChatCompletions(c)
			return
		}
		h.Gateway.ChatCompletions(c)
	}))...)
	r.POST("/embeddings", append([]gin.HandlerFunc{textBodyLimit}, append(commonDirect[1:], withCompositeMemberGroups(func(c *gin.Context) {
		if !isOpenAIOnlyEndpointGatewayPlatform(c) {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Embeddings API is not supported for this platform",
				},
			})
			return
		}
		h.OpenAIGateway.Embeddings(c)
	}))...)...)
	r.POST("/images/generations", append(commonDirect, withCompositeMemberGroups(imagesHandler))...)
	r.POST("/images/edits", append(commonDirect, withCompositeMemberGroups(imagesHandler))...)
	r.POST("/images/generations/async", append(commonDirect, withCompositeMemberGroups(h.AsyncImage.Submit))...)
	r.POST("/images/edits/async", append(commonDirect, withCompositeMemberGroups(h.AsyncImage.Submit))...)
	r.GET("/images/tasks/:task_id", append(commonDirect, h.AsyncImage.Get)...)
	r.POST("/videos/generations", append(commonDirect, withCompositeMemberGroups(videoGenerationHandler))...)
	r.POST("/videos/edits", append(commonDirect, withCompositeMemberGroups(videoEditHandler))...)
	r.POST("/videos/extensions", append(commonDirect, withCompositeMemberGroups(videoExtensionHandler))...)
	r.GET("/videos/:request_id", append(commonDirect, videoStatusHandler)...)
	r.GET("/videos/:request_id/content", append(commonDirect, videoContentHandler)...)

	// Antigravity 模型列表
	r.GET("/antigravity/models", gin.HandlerFunc(apiKeyAuth), resolveMemberGroupAnthropic, enforceMemberBudgetAnthropic, requireGroupAnthropic, h.Gateway.AntigravityModels)

	// Antigravity 专用路由（仅使用 antigravity 账户，不混合调度）
	antigravityV1 := r.Group("/antigravity/v1")
	antigravityV1.Use(bodyLimit)
	antigravityV1.Use(clientRequestID)
	antigravityV1.Use(opsErrorLogger)
	antigravityV1.Use(endpointNorm)
	antigravityV1.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1.Use(gin.HandlerFunc(apiKeyAuth))
	antigravityV1.Use(resolveMemberGroupAnthropic)
	antigravityV1.Use(enforceMemberBudgetAnthropic)
	antigravityV1.Use(requireGroupAnthropic)
	{
		antigravityV1.POST("/messages", orchestrateMemberGroups(h.Gateway.Messages))
		antigravityV1.POST("/messages/count_tokens", orchestrateMemberGroups(h.Gateway.CountTokens))
		antigravityV1.GET("/models", h.Gateway.AntigravityModels)
		antigravityV1.GET("/usage", h.Gateway.Usage)
	}

	antigravityV1Beta := r.Group("/antigravity/v1beta")
	antigravityV1Beta.Use(bodyLimit)
	antigravityV1Beta.Use(clientRequestID)
	antigravityV1Beta.Use(opsErrorLogger)
	antigravityV1Beta.Use(endpointNorm)
	antigravityV1Beta.Use(middleware.ForcePlatform(service.PlatformAntigravity))
	antigravityV1Beta.Use(middleware.APIKeyAuthWithSubscriptionGoogle(apiKeyService, subscriptionService, cfg))
	antigravityV1Beta.Use(resolveMemberGroupGoogle)
	antigravityV1Beta.Use(enforceMemberBudgetGoogle)
	antigravityV1Beta.Use(requireGroupGoogle)
	{
		antigravityV1Beta.GET("/models", orchestrateMemberGroups(h.Gateway.GeminiV1BetaListModels))
		antigravityV1Beta.GET("/models/:model", orchestrateMemberGroups(h.Gateway.GeminiV1BetaGetModel))
		antigravityV1Beta.POST("/models/*modelAction", orchestrateMemberGroups(h.Gateway.GeminiV1BetaModels))
	}

}

// getGroupPlatform extracts the group platform from the API Key stored in context.
func getGroupPlatform(c *gin.Context) string {
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey.Group == nil {
		return ""
	}
	if apiKey.Group.Platform == service.PlatformComposite {
		if platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
			return platform
		}
	}
	return apiKey.Group.Platform
}

func compositeTargetPlatformMiddleware(resolver *service.CompositeRouteResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveCompositeTargetPlatform(c, resolver) {
			return
		}
		c.Next()
	}
}

func compositeTargetPlatformHandler(resolver *service.CompositeRouteResolver, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveCompositeTargetPlatform(c, resolver) {
			return
		}
		next(c)
	}
}

func resolveCompositeTargetPlatform(c *gin.Context, resolver *service.CompositeRouteResolver) bool {
	if resolver == nil {
		resolver = service.NewCompositeRouteResolver(nil)
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return true
	}
	if c.Request == nil || c.Request.Method == http.MethodGet {
		return true
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		status := http.StatusBadRequest
		message := "Failed to read request body"
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			status = http.StatusRequestEntityTooLarge
			message = "Request body is too large"
		}
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
		c.Abort()
		return false
	}

	model := compositeRequestModelFromBody(c.GetHeader("Content-Type"), body)
	if model != "" {
		decision, resolveErr := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, compositeRouteEndpointForPath(c.Request.URL.Path))
		if resolveErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "server_error", "message": "Failed to resolve composite model route"}})
			c.Abort()
			return false
		}
		if !decision.Matched {
			service.MarkOpsGroupRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "No composite route matches this model and endpoint"}})
			return false
		}
		c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
		if upstreamModel := strings.TrimSpace(decision.UpstreamModel); upstreamModel != "" && upstreamModel != model && gjson.ValidBytes(body) {
			if rewritten, rewriteErr := sjson.SetBytes(body, "model", upstreamModel); rewriteErr == nil {
				body = rewritten
			}
		}
	}
	resetRequestBody(c, body)
	return true
}

func compositeRequestModelFromBody(contentType string, body []byte) string {
	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); model != "" {
		return model
	}
	return compositeMultipartModelFromBody(contentType, body)
}

func compositeMultipartModelFromBody(contentType string, body []byte) string {
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return ""
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return ""
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return ""
		}
		if err != nil {
			return ""
		}
		if part.FormName() != "model" || part.FileName() != "" {
			continue
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
}

func compositeGeminiTargetPlatformMiddleware(resolver *service.CompositeRouteResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveCompositeGeminiTargetPlatform(c, resolver) {
			return
		}
		c.Next()
	}
}

func compositeGeminiTargetPlatformHandler(resolver *service.CompositeRouteResolver, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveCompositeGeminiTargetPlatform(c, resolver) {
			return
		}
		next(c)
	}
}

func resolveCompositeGeminiTargetPlatform(c *gin.Context, resolver *service.CompositeRouteResolver) bool {
	if resolver == nil {
		resolver = service.NewCompositeRouteResolver(nil)
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return true
	}
	model := compositeGeminiModelFromParams(c)
	if model != "" {
		decision, err := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, service.CompositeRouteEndpointGemini)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "server_error", "message": "Failed to resolve composite model route"}})
			c.Abort()
			return false
		}
		if decision.Matched {
			c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
		}
	}
	if _, resolved := service.ResolvedTargetPlatformFromContext(c.Request.Context()); !resolved {
		c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformGemini))
	}
	return true
}

func compositeGeminiModelFromParams(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if model := strings.TrimSpace(c.Param("model")); model != "" {
		return model
	}
	modelAction := strings.TrimPrefix(strings.TrimSpace(c.Param("modelAction")), "/")
	if modelAction == "" {
		return ""
	}
	if idx := strings.LastIndex(modelAction, ":"); idx >= 0 {
		return strings.TrimSpace(modelAction[:idx])
	}
	return modelAction
}

func resetRequestBody(c *gin.Context, body []byte) {
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

func compositeRouteEndpointForPath(path string) string {
	switch {
	case strings.Contains(path, "/messages/count_tokens"):
		return service.CompositeRouteEndpointCountTokens
	case strings.Contains(path, "/messages"):
		return service.CompositeRouteEndpointMessages
	case strings.Contains(path, "/responses"):
		return service.CompositeRouteEndpointResponses
	case strings.Contains(path, "/chat/completions"):
		return service.CompositeRouteEndpointChatCompletions
	case strings.Contains(path, "/embeddings"):
		return service.CompositeRouteEndpointEmbeddings
	case strings.Contains(path, "/images/"):
		return service.CompositeRouteEndpointImages
	case strings.Contains(path, "/v1beta/"):
		return service.CompositeRouteEndpointGemini
	default:
		return service.CompositeRouteEndpointAny
	}
}
