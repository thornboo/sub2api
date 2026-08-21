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
	enterpriseMemberRoutePlanner *service.EnterpriseMemberRoutePlanner,
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
	resolveMemberGroupAnthropic := middleware.ResolveEnterpriseMemberGroup(subscriptionService, enterpriseMemberRoutePlanner, settingService, cfg, middleware.AnthropicErrorWriter)
	resolveMemberGroupGoogle := middleware.ResolveEnterpriseMemberGroup(subscriptionService, enterpriseMemberRoutePlanner, settingService, cfg, middleware.GoogleErrorWriter)
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
		case service.PlatformOpenAI, service.PlatformGrok,
			service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek:
			// 国产 OpenAI 兼容供应商（kimi/zhipu/deepseek）与 openai/grok 一样经 OpenAI 网关转发。
			return true
		default:
			return false
		}
	}
	countTokensHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformOpenAI, service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek:
			h.OpenAIGateway.CountTokens(c)
		case service.PlatformGrok:
			h.OpenAIGateway.GrokCountTokens(c)
		default:
			h.Gateway.CountTokens(c)
		}
	}
	modelsHandler := func(c *gin.Context) {
		if c.Query("client_version") != "" {
			switch getGroupPlatform(c) {
			case service.PlatformOpenAI, service.PlatformComposite:
				h.OpenAIGateway.CodexModels(c)
				return
			}
		}
		h.Gateway.Models(c)
	}
	memberModelsHandler := orchestrateMemberGroups(modelsHandler)
	liveCreateHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformOpenAI {
			markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"type":    "not_found_error",
					"message": "Live is not supported for this platform",
				},
			})
			return
		}
		apiKey, ok := middleware.GetAPIKeyFromContext(c)
		if ok && apiKey != nil && apiKey.Group != nil && !apiKey.Group.AllowLive {
			markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusForbidden, gin.H{
				"error": gin.H{
					"type":    "permission_error",
					"message": "Live is not enabled for this group",
				},
			})
			return
		}
		h.OpenAIGateway.Live(c)
	}
	withCompositeLiveMemberGroups := func(next gin.HandlerFunc) gin.HandlerFunc {
		return orchestrateMemberGroups(compositeLiveTargetPlatformHandler(compositeResolver, next))
	}
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
			markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
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
		// Video status/content lookups below already allow Composite groups; keep
		// task creation aligned so composite keys that route to Grok accounts can
		// submit video generation jobs.
		if platform := getGroupPlatform(c); platform == service.PlatformGrok || platform == service.PlatformComposite {
			h.OpenAIGateway.GrokVideoGeneration(c)
			return
		}
		markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
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
		markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
	}
	videoExtensionHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoExtension(c)
			return
		}
		markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Videos API is not supported for this platform"}})
	}
	// /responses/*subpath 的子路径会被转发到上游同名端点之后，因此在入口就拒掉
	// 不可转发的子路径，不让它进入调度与转发流程。可转发的判定见
	// service.IsForwardableOpenAIResponsesRequestPath 及 upstream_path_guard.go。
	guardResponsesSubpath := func(next gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if !service.IsForwardableOpenAIResponsesRequestPath(c) {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalPolicyDenied)
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"error": gin.H{
						"type":    "not_found_error",
						"message": "Unsupported responses subpath",
					},
				})
				return
			}
			if service.IsOpenAIResponsesInputTokensRequestPath(c) && isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.ResponsesInputTokens(c)
				return
			}
			next(c)
		}
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
			if getGroupPlatform(c) == service.PlatformOpenAI {
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
		gateway.POST("/live", withCompositeLiveMemberGroups(liveCreateHandler))
		gateway.GET("/live/:call_id", h.OpenAIGateway.LiveSideband)
		// OpenAI Responses API: auto-route based on group platform
		gateway.POST("/responses", withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		}))
		gateway.POST("/responses/*subpath", guardResponsesSubpath(withCompositeMemberGroups(func(c *gin.Context) {
			if isOpenAIResponsesCompatibleGatewayPlatform(c) {
				h.OpenAIGateway.Responses(c)
				return
			}
			h.Gateway.Responses(c)
		})))
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
				markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
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
		// OpenAI-compatible clients may create through /videos; xAI receives the
		// canonical /videos/generations route inside the Grok media forwarder.
		gateway.POST("/videos", withCompositeMemberGroups(videoGenerationHandler))
		gateway.POST("/videos/generations", withCompositeMemberGroups(videoGenerationHandler))
		gateway.POST("/videos/edits", withCompositeMemberGroups(videoEditHandler))
		gateway.POST("/videos/extensions", withCompositeMemberGroups(videoExtensionHandler))
		gateway.GET("/videos/generations/:request_id/content", withCompositeMemberGroups(videoContentHandler))
		gateway.GET("/videos/edits/:request_id/content", withCompositeMemberGroups(videoContentHandler))
		gateway.GET("/videos/extensions/:request_id/content", withCompositeMemberGroups(videoContentHandler))
		gateway.GET("/videos/generations/:request_id", withCompositeMemberGroups(videoStatusHandler))
		gateway.GET("/videos/edits/:request_id", withCompositeMemberGroups(videoStatusHandler))
		gateway.GET("/videos/extensions/:request_id", withCompositeMemberGroups(videoStatusHandler))
		gateway.GET("/videos/:request_id", videoStatusHandler)
		gateway.GET("/videos/:request_id/content", videoContentHandler)

		// xAI Voice APIs (Grok platform only): HTTP TTS/STT + Realtime WS.
		// Not part of the creation-center product surface — gateway relay only.
		voiceHandler := func(endpoint string) gin.HandlerFunc {
			return func(c *gin.Context) {
				if getGroupPlatform(c) != service.PlatformGrok {
					service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
					c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
					return
				}
				h.OpenAIGateway.GrokVoice(c, endpoint)
			}
		}
		gateway.POST("/tts", voiceHandler("tts"))
		gateway.POST("/stt", voiceHandler("stt"))
		gateway.POST("/custom-voices", voiceHandler("custom-voices"))
		customVoicePathHandler := func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
				return
			}
			h.OpenAIGateway.GrokVoice(c, grokCustomVoiceEndpoint(c))
		}
		gateway.GET("/custom-voices", voiceHandler("custom-voices"))
		gateway.GET("/custom-voices/:voice_id/audio", customVoicePathHandler)
		gateway.GET("/custom-voices/:voice_id", customVoicePathHandler)
		gateway.PATCH("/custom-voices/:voice_id", customVoicePathHandler)
		gateway.DELETE("/custom-voices/:voice_id", customVoicePathHandler)
		gateway.GET("/realtime", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Realtime API is not supported for this platform"}})
				return
			}
			h.OpenAIGateway.GrokRealtime(c)
		})
		gateway.POST("/web_search", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Web Search API is not supported for this platform"}})
				return
			}
			h.Gateway.WebSearch(c)
		})
		gateway.POST("/x_search", func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "X Search API is not supported for this platform"}})
				return
			}
			h.Gateway.XSearch(c)
		})
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
	r.POST("/responses/*subpath", append(commonDirect, guardResponsesSubpath(withCompositeMemberGroups(responsesHandler)))...)
	r.POST("/alpha/search", append([]gin.HandlerFunc{textBodyLimit}, append(commonDirect[1:], withCompositeMemberGroups(h.OpenAIGateway.AlphaSearch))...)...)
	r.GET("/responses", append(commonDirect, withCompositeResolver(func(c *gin.Context) {
		h.OpenAIGateway.ResponsesWebSocket(c)
	}))...)
	r.GET("/models", append(commonDirect, memberModelsHandler)...)
	r.POST("/messages/count_tokens", append(commonDirect, withCompositeMemberGroups(countTokensHandler))...)

	codexDirect := r.Group("/backend-api/codex")
	codexDirect.Use(commonDirect...)
	{
		codexDirect.POST("/realtime/calls", withCompositeLiveMemberGroups(liveCreateHandler))
		codexDirect.GET("/:call_id", h.OpenAIGateway.LiveSideband)
		codexDirect.POST("/responses", withCompositeMemberGroups(responsesHandler))
		codexDirect.POST("/responses/*subpath", guardResponsesSubpath(withCompositeMemberGroups(responsesHandler)))
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
			markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
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
	r.POST("/videos", append(commonDirect, withCompositeMemberGroups(videoGenerationHandler))...)
	r.POST("/videos/generations", append(commonDirect, withCompositeMemberGroups(videoGenerationHandler))...)
	r.POST("/videos/edits", append(commonDirect, withCompositeMemberGroups(videoEditHandler))...)
	r.POST("/videos/extensions", append(commonDirect, withCompositeMemberGroups(videoExtensionHandler))...)
	r.GET("/videos/generations/:request_id/content", append(commonDirect, withCompositeMemberGroups(videoContentHandler))...)
	r.GET("/videos/edits/:request_id/content", append(commonDirect, withCompositeMemberGroups(videoContentHandler))...)
	r.GET("/videos/extensions/:request_id/content", append(commonDirect, withCompositeMemberGroups(videoContentHandler))...)
	r.GET("/videos/generations/:request_id", append(commonDirect, withCompositeMemberGroups(videoStatusHandler))...)
	r.GET("/videos/edits/:request_id", append(commonDirect, withCompositeMemberGroups(videoStatusHandler))...)
	r.GET("/videos/extensions/:request_id", append(commonDirect, withCompositeMemberGroups(videoStatusHandler))...)
	r.GET("/videos/:request_id", append(commonDirect, videoStatusHandler)...)
	r.GET("/videos/:request_id/content", append(commonDirect, videoContentHandler)...)

	rootVoiceHandler := func(endpoint string) gin.HandlerFunc {
		return func(c *gin.Context) {
			if getGroupPlatform(c) != service.PlatformGrok {
				service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
				return
			}
			h.OpenAIGateway.GrokVoice(c, endpoint)
		}
	}
	r.POST("/tts", append(commonDirect, withCompositeMemberGroups(rootVoiceHandler("tts")))...)
	r.POST("/stt", append(commonDirect, withCompositeMemberGroups(rootVoiceHandler("stt")))...)
	r.POST("/custom-voices", append(commonDirect, withCompositeMemberGroups(rootVoiceHandler("custom-voices")))...)
	rootCustomVoicePathHandler := func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Voice API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokVoice(c, grokCustomVoiceEndpoint(c))
	}
	r.GET("/custom-voices", append(commonDirect, withCompositeMemberGroups(rootVoiceHandler("custom-voices")))...)
	r.GET("/custom-voices/:voice_id/audio", append(commonDirect, withCompositeMemberGroups(rootCustomVoicePathHandler))...)
	r.GET("/custom-voices/:voice_id", append(commonDirect, withCompositeMemberGroups(rootCustomVoicePathHandler))...)
	r.PATCH("/custom-voices/:voice_id", append(commonDirect, withCompositeMemberGroups(rootCustomVoicePathHandler))...)
	r.DELETE("/custom-voices/:voice_id", append(commonDirect, withCompositeMemberGroups(rootCustomVoicePathHandler))...)
	r.GET("/realtime", append(commonDirect, withCompositeMemberGroups(func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Realtime API is not supported for this platform"}})
			return
		}
		h.OpenAIGateway.GrokRealtime(c)
	}))...)
	r.POST("/web_search", append(commonDirect, withCompositeMemberGroups(func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "Web Search API is not supported for this platform"}})
			return
		}
		h.Gateway.WebSearch(c)
	}))...)
	r.POST("/x_search", append(commonDirect, withCompositeMemberGroups(func(c *gin.Context) {
		if getGroupPlatform(c) != service.PlatformGrok {
			service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "X Search API is not supported for this platform"}})
			return
		}
		h.Gateway.XSearch(c)
	}))...)

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

func compositeLiveTargetPlatformHandler(resolver *service.CompositeRouteResolver, next gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !resolveCompositeLiveTargetPlatform(c, resolver) {
			return
		}
		next(c)
	}
}

func resolveCompositeLiveTargetPlatform(c *gin.Context, resolver *service.CompositeRouteResolver) bool {
	if resolver == nil {
		resolver = service.NewCompositeRouteResolver(nil)
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.Group == nil || apiKey.Group.Platform != service.PlatformComposite {
		return true
	}
	if c.Request == nil || c.Request.Method != http.MethodPost {
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

	model, modelErr := service.ExtractLiveCallRequestModel(c.GetHeader("Content-Type"), body)
	if modelErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "Unable to parse Live session model"}})
		c.Abort()
		return false
	}
	if model != "" {
		decision, resolveErr := resolver.Resolve(c.Request.Context(), apiKey.Group.ID, model, compositeRouteEndpointForPath(c.Request.URL.Path))
		if resolveErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"type": "server_error", "message": "Failed to resolve composite model route"}})
			c.Abort()
			return false
		}
		if !decision.Matched {
			markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "No composite route matches this Live model"}})
			return false
		}
		c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
		if upstreamModel := strings.TrimSpace(decision.UpstreamModel); upstreamModel != "" && upstreamModel != model {
			if rewritten, rewrittenContentType, ok := rewriteLiveRequestModel(c.GetHeader("Content-Type"), body, upstreamModel); ok {
				body = rewritten
				if rewrittenContentType != "" {
					c.Request.Header.Set("Content-Type", rewrittenContentType)
				}
			}
		}
	}
	resetRequestBody(c, body)
	return true
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
			markEnterpriseMemberRouteRetry(c, service.OpsGroupRetryReasonCapabilityMismatch)
			c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"type": "not_found_error", "message": "No composite route matches this model and endpoint"}})
			return false
		}
		c.Request = c.Request.WithContext(service.WithCompositeRouteDecision(c.Request.Context(), decision))
		if upstreamModel := strings.TrimSpace(decision.UpstreamModel); upstreamModel != "" && upstreamModel != model && gjson.ValidBytes(body) {
			if _, modelPath := compositeJSONRequestModel(body); modelPath != "" {
				if rewritten, rewriteErr := sjson.SetBytes(body, modelPath, upstreamModel); rewriteErr == nil {
					body = rewritten
				}
			}
		}
	}
	resetRequestBody(c, body)
	return true
}

func markEnterpriseMemberRouteRetry(c *gin.Context, reason service.OpsGroupRetryReason) {
	if c == nil || c.Request == nil {
		return
	}
	apiKey, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.MemberID == nil {
		return
	}
	if _, ok := service.ActiveGroupFromContext(c.Request.Context()); !ok {
		return
	}
	service.MarkOpsGroupRetry(c, reason)
}

func compositeRequestModelFromBody(contentType string, body []byte) string {
	if model, _ := compositeJSONRequestModel(body); model != "" {
		return model
	}
	return compositeMultipartModelFromBody(contentType, body)
}

func compositeJSONRequestModel(body []byte) (string, string) {
	for _, path := range []string{"model", "session.model"} {
		model := gjson.GetBytes(body, path)
		if model.Type != gjson.String {
			continue
		}
		if value := strings.TrimSpace(model.String()); value != "" {
			return value, path
		}
	}
	return "", ""
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
		fieldName := part.FormName()
		if part.FileName() != "" || (fieldName != "model" && fieldName != "session") {
			continue
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return ""
		}
		switch fieldName {
		case "model":
			return strings.TrimSpace(string(data))
		case "session":
			if model, _ := compositeJSONRequestModel(data); model != "" {
				return model
			}
		}
	}
}

func rewriteLiveRequestModel(contentType string, body []byte, upstreamModel string) ([]byte, string, bool) {
	upstreamModel = strings.TrimSpace(upstreamModel)
	if upstreamModel == "" {
		return body, "", false
	}
	if gjson.ValidBytes(body) {
		rewritten, err := sjson.SetBytes(body, "session.model", upstreamModel)
		return rewritten, "", err == nil
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil || !strings.EqualFold(mediaType, "multipart/form-data") {
		return body, "", false
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return body, "", false
	}
	reader := multipart.NewReader(bytes.NewReader(body), boundary)
	var out bytes.Buffer
	writer := multipart.NewWriter(&out)
	rewrote := false
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			_ = writer.Close()
			return body, "", false
		}
		data, readErr := io.ReadAll(part)
		if readErr != nil {
			_ = part.Close()
			_ = writer.Close()
			return body, "", false
		}
		if strings.EqualFold(strings.TrimSpace(part.FormName()), "session") && part.FileName() == "" && gjson.ValidBytes(data) {
			if rewrittenSession, rewriteErr := sjson.SetBytes(data, "model", upstreamModel); rewriteErr == nil {
				data = rewrittenSession
				rewrote = true
			}
		}
		target, createErr := writer.CreatePart(part.Header)
		_ = part.Close()
		if createErr != nil {
			_ = writer.Close()
			return body, "", false
		}
		if _, writeErr := target.Write(data); writeErr != nil {
			_ = writer.Close()
			return body, "", false
		}
	}
	if closeErr := writer.Close(); closeErr != nil || !rewrote {
		return body, "", false
	}
	return out.Bytes(), writer.FormDataContentType(), true
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

// grokCustomVoiceEndpoint derives the upstream Voice endpoint for the
// /custom-voices/:voice_id[/audio] routes.
//
// The /audio suffix must be decided from the matched route template, not from
// the raw URL path: a voice literally named "audio" makes GET
// /custom-voices/audio match /custom-voices/:voice_id, and a raw-path suffix
// check would rewrite it to custom-voices/audio/audio — turning a profile
// lookup into an audio download.
func grokCustomVoiceEndpoint(c *gin.Context) string {
	endpoint := "custom-voices/" + c.Param("voice_id")
	if strings.HasSuffix(c.FullPath(), "/:voice_id/audio") {
		endpoint += "/audio"
	}
	return endpoint
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
	case strings.Contains(path, "/responses"),
		strings.Contains(path, "/alpha/search"),
		strings.Contains(path, "/realtime/calls"),
		strings.HasSuffix(strings.TrimRight(path, "/"), "/live"):
		return service.CompositeRouteEndpointResponses
	case strings.Contains(path, "/chat/completions"):
		return service.CompositeRouteEndpointChatCompletions
	case strings.Contains(path, "/embeddings"):
		return service.CompositeRouteEndpointEmbeddings
	case strings.Contains(path, "/images/batches"):
		return service.CompositeRouteEndpointBatchImages
	case strings.Contains(path, "/images/"):
		return service.CompositeRouteEndpointImages
	case strings.Contains(path, "/live") || strings.Contains(path, "/realtime/calls"):
		return service.CompositeRouteEndpointLive
	case strings.Contains(path, "/videos/generations") || strings.Contains(path, "/videos/edits") || strings.Contains(path, "/videos/extensions"):
		return service.CompositeRouteEndpointVideo
	case strings.Contains(path, "/v1beta/"):
		return service.CompositeRouteEndpointGemini
	default:
		return service.CompositeRouteEndpointAny
	}
}
