package admin

import (
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
	modelStatusService *service.ModelSelfCheckService
}

func NewModelSelfCheckHandler(modelStatusService *service.ModelSelfCheckService) *ModelSelfCheckHandler {
	return &ModelSelfCheckHandler{modelStatusService: modelStatusService}
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
