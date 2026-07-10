package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// UpdateModelRateLimitSettingsRequest 更新模型级限流策略配置请求
type UpdateModelRateLimitSettingsRequest struct {
	Enabled          bool `json:"enabled"`
	FailureThreshold int  `json:"failure_threshold"`
	WindowMinutes    int  `json:"window_minutes"`
	CooldownSeconds  int  `json:"cooldown_seconds"`
}

func (h *SettingHandler) GetModelRateLimitSettings(c *gin.Context) {
	settings, err := h.settingService.GetModelRateLimitSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ModelRateLimitSettings{
		Enabled:          settings.Enabled,
		FailureThreshold: settings.FailureThreshold,
		WindowMinutes:    settings.WindowMinutes,
		CooldownSeconds:  settings.CooldownSeconds,
	})
}

func (h *SettingHandler) UpdateModelRateLimitSettings(c *gin.Context) {
	var req UpdateModelRateLimitSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	settings := &service.ModelRateLimitSettings{
		Enabled:          req.Enabled,
		FailureThreshold: req.FailureThreshold,
		WindowMinutes:    req.WindowMinutes,
		CooldownSeconds:  req.CooldownSeconds,
	}

	if err := h.settingService.SetModelRateLimitSettings(c.Request.Context(), settings); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	updatedSettings, err := h.settingService.GetModelRateLimitSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ModelRateLimitSettings{
		Enabled:          updatedSettings.Enabled,
		FailureThreshold: updatedSettings.FailureThreshold,
		WindowMinutes:    updatedSettings.WindowMinutes,
		CooldownSeconds:  updatedSettings.CooldownSeconds,
	})
}
