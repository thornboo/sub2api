package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ModelPlazaHandler 处理「模型广场」查询。
//
// 广场路由挂 OptionalJWT 中间件：匿名可访问（除非 require_auth 开启）。
// 无论是否带 token，响应都固定为 active、standard、非专属的公开目录；
// 专属分组、订阅分组和用户专属倍率只属于登录后的「可用渠道」。
type ModelPlazaHandler struct {
	channelService *service.ChannelService
	settingService *service.SettingService
	modelDelivery  *service.ModelDeliveryService
}

// NewModelPlazaHandler 创建模型广场 handler。
func NewModelPlazaHandler(
	channelService *service.ChannelService,
	settingService *service.SettingService,
	modelDelivery *service.ModelDeliveryService,
) *ModelPlazaHandler {
	return &ModelPlazaHandler{
		channelService: channelService,
		settingService: settingService,
		modelDelivery:  modelDelivery,
	}
}

// modelPlazaResponse 广场页响应。
type modelPlazaResponse struct {
	Description string                 `json:"description"`
	Channels    []userAvailableChannel `json:"channels"`
}

// Get 返回模型广场数据。
// GET /api/v1/model-plaza
func (h *ModelPlazaHandler) Get(c *gin.Context) {
	if h.settingService == nil {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}
	rt := h.settingService.GetModelPlazaRuntime(c.Request.Context())
	if !rt.Enabled {
		response.NotFound(c, "Model plaza is not enabled")
		return
	}

	_, authed := middleware.GetAuthSubjectFromContext(c)
	if rt.RequireAuth && !authed {
		response.Unauthorized(c, "Authentication required")
		return
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out, err := buildAvailableChannelCatalog(
		c.Request.Context(),
		channels,
		h.modelDelivery,
		filterPublicStandardGroups,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, modelPlazaResponse{
		Description: rt.Description,
		Channels:    out,
	})
}
