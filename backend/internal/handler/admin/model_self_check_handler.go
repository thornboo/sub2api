package admin

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	modelSelfCheckTokenWindowToday = "today"
	modelSelfCheckTokenWindow7d    = "7d"
	modelSelfCheckTokenWindow30d   = "30d"
)

// ModelSelfCheckHandler exposes admin-only model self-check diagnostics.
type ModelSelfCheckHandler struct {
	modelStatusService modelSelfCheckDiagnostics
}

type modelSelfCheckDiagnostics interface {
	ListTokenUsageSince(context.Context, time.Time) ([]service.ModelSelfCheckTokenUsage, error)
	GetAdminProbeChain(context.Context, int64, string) (*service.ModelSelfCheckChainView, error)
}

func NewModelSelfCheckHandler(modelStatusService *service.ModelSelfCheckService) *ModelSelfCheckHandler {
	if modelStatusService == nil {
		return &ModelSelfCheckHandler{}
	}
	return &ModelSelfCheckHandler{modelStatusService: modelStatusService}
}

// GetProbeChain GET /api/v1/admin/model-self-check/chain?group_id=...&model=...
// Account identity is intentionally confined to this administrator route.
func (h *ModelSelfCheckHandler) GetProbeChain(c *gin.Context) {
	groupID, err := strconv.ParseInt(c.Query("group_id"), 10, 64)
	model := strings.TrimSpace(c.Query("model"))
	if err != nil || groupID <= 0 || model == "" || len(model) > 255 {
		response.BadRequest(c, "group_id and model are required")
		return
	}
	if h == nil || h.modelStatusService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Model self-check service is unavailable")
		return
	}
	chain, err := h.modelStatusService.GetAdminProbeChain(c.Request.Context(), groupID, model)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if chain == nil {
		response.NotFound(c, "Model self-check target not found")
		return
	}
	response.Success(c, chain)
}

type modelSelfCheckTokenUsageItem struct {
	Model        string `json:"model"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	TotalTokens  int64  `json:"total_tokens"`
}

// GetTokenUsage GET /api/v1/admin/model-self-check/token-usage?window=today|7d|30d
func (h *ModelSelfCheckHandler) GetTokenUsage(c *gin.Context) {
	window, since := resolveModelSelfCheckTokenUsageWindow(c.Query("window"), c.Query("timezone"))
	if h == nil || h.modelStatusService == nil {
		response.Success(c, gin.H{"window": window, "items": []modelSelfCheckTokenUsageItem{}})
		return
	}
	rows, err := h.modelStatusService.ListTokenUsageSince(c.Request.Context(), since)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]modelSelfCheckTokenUsageItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, modelSelfCheckTokenUsageItem{
			Model:        row.Model,
			InputTokens:  row.InputTokens,
			OutputTokens: row.OutputTokens,
			TotalTokens:  row.TotalTokens,
		})
	}
	response.Success(c, gin.H{"window": window, "items": items})
}

func resolveModelSelfCheckTokenUsageWindow(rawWindow, userTZ string) (string, time.Time) {
	now := timezone.NowInUserLocation(userTZ)
	switch strings.ToLower(strings.TrimSpace(rawWindow)) {
	case modelSelfCheckTokenWindow7d:
		return modelSelfCheckTokenWindow7d, now.AddDate(0, 0, -7)
	case modelSelfCheckTokenWindow30d:
		return modelSelfCheckTokenWindow30d, now.AddDate(0, 0, -30)
	case modelSelfCheckTokenWindowToday, "":
		fallthrough
	default:
		return modelSelfCheckTokenWindowToday, timezone.StartOfDayInUserLocation(now, userTZ)
	}
}
