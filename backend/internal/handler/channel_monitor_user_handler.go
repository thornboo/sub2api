package handler

import (
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// ChannelMonitorUserHandler 模型服务状态用户只读 handler。
type ChannelMonitorUserHandler struct {
	modelStatusService *service.ModelSelfCheckService
	settingService     *service.SettingService
}

// NewChannelMonitorUserHandler 创建 handler。
// settingService 用于每次请求前读取功能开关；关闭时 List/GetStatus 直接返回空/404。
func NewChannelMonitorUserHandler(
	modelStatusService *service.ModelSelfCheckService,
	settingService *service.SettingService,
) *ChannelMonitorUserHandler {
	return &ChannelMonitorUserHandler{
		modelStatusService: modelStatusService,
		settingService:     settingService,
	}
}

// featureEnabled 返回当前模型自检功能是否开启。
// settingService 为 nil（测试场景）视为启用。
func (h *ChannelMonitorUserHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return true
	}
	return h.settingService.GetModelSelfCheckRuntime(c.Request.Context()).Enabled
}

// --- Response ---

type userModelStatusListItem struct {
	GroupID          int64                          `json:"group_id"`
	GroupName        string                         `json:"group_name"`
	Model            string                         `json:"model"`
	DisplayName      string                         `json:"display_name"`
	Status           string                         `json:"status"`
	MessageCode      string                         `json:"message_code"`
	LatestLatencyMs  *int                           `json:"latest_latency_ms"`
	AvgLatency24hMs  *int                           `json:"avg_latency_24h_ms"`
	AvgLatency7dMs   *int                           `json:"avg_latency_7d_ms"`
	Availability24h  *float64                       `json:"availability_24h"`
	Availability7d   *float64                       `json:"availability_7d"`
	Availability30d  *float64                       `json:"availability_30d"`
	DegradedRatio24h *float64                       `json:"degraded_ratio_24h"`
	LastCheckedAt    *string                        `json:"last_checked_at"`
	Timeline         []userModelStatusTimelinePoint `json:"timeline,omitempty"`
}

type userModelStatusTimelinePoint struct {
	Status    string `json:"status"`
	LatencyMs *int   `json:"latency_ms"`
	CheckedAt string `json:"checked_at"`
}

func userModelStatusViewToItem(v *service.UserModelStatusView) userModelStatusListItem {
	return userModelStatusListItem{
		GroupID:          v.GroupID,
		GroupName:        v.GroupName,
		Model:            v.Model,
		DisplayName:      v.DisplayName,
		Status:           v.Status,
		MessageCode:      v.MessageCode,
		LatestLatencyMs:  v.LatestLatencyMs,
		AvgLatency24hMs:  v.AvgLatency24hMs,
		AvgLatency7dMs:   v.AvgLatency7dMs,
		Availability24h:  v.Availability24h,
		Availability7d:   v.Availability7d,
		Availability30d:  v.Availability30d,
		DegradedRatio24h: v.DegradedRatio24h,
		LastCheckedAt:    formatOptionalTime(v.LastCheckedAt),
		Timeline:         userModelTimelineToResponse(v.Timeline),
	}
}

func userModelTimelineToResponse(points []service.UserModelTimelinePoint) []userModelStatusTimelinePoint {
	out := make([]userModelStatusTimelinePoint, 0, len(points))
	for _, p := range points {
		out = append(out, userModelStatusTimelinePoint{
			Status:    p.Status,
			LatencyMs: p.LatencyMs,
			CheckedAt: p.CheckedAt.UTC().Format(time.RFC3339),
		})
	}
	return out
}

func formatOptionalTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	v := t.UTC().Format(time.RFC3339)
	return &v
}

// --- Handlers ---

// ListModelStatus GET /api/v1/model-status
func (h *ChannelMonitorUserHandler) ListModelStatus(c *gin.Context) {
	if !h.featureEnabled(c) {
		response.Success(c, gin.H{
			"items":      []userModelStatusListItem{},
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	if h.modelStatusService == nil {
		response.Success(c, gin.H{
			"items":      []userModelStatusListItem{},
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}
	views, err := h.modelStatusService.ListUserModelStatus(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]userModelStatusListItem, 0, len(views))
	for _, v := range views {
		items = append(items, userModelStatusViewToItem(v))
	}
	response.Success(c, gin.H{
		"items":      items,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// GetModelStatus GET /api/v1/model-status/detail?group_id=...&model=...
func (h *ChannelMonitorUserHandler) GetModelStatus(c *gin.Context) {
	if !h.featureEnabled(c) {
		response.ErrorFrom(c, service.ErrChannelMonitorNotFound)
		return
	}
	if h.modelStatusService == nil {
		response.ErrorFrom(c, service.ErrChannelMonitorNotFound)
		return
	}
	model := c.Query("model")
	groupID, err := parseOptionalGroupID(c.Query("group_id"))
	if err != nil {
		response.BadRequest(c, "invalid group_id")
		return
	}
	detail, err := h.modelStatusService.GetUserModelStatus(c.Request.Context(), groupID, model)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, userModelStatusViewToItem(&detail.UserModelStatusView))
}

func parseOptionalGroupID(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
