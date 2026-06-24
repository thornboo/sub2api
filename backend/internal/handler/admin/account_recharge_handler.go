package admin

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type upstreamRechargeService interface {
	ListUpstreamRechargeRecords(ctx context.Context, accountID int64) (*service.UpstreamRechargeRecordsResult, error)
	CreateUpstreamRechargeRecord(ctx context.Context, input service.UpstreamRechargeRecordInput) (*service.UpstreamRechargeRecord, error)
	UpdateUpstreamRechargeRecord(ctx context.Context, recordID int64, input service.UpstreamRechargeRecordInput) (*service.UpstreamRechargeRecord, error)
	DeleteUpstreamRechargeRecord(ctx context.Context, accountID, recordID int64) error
}

type upstreamRechargeRecordRequest struct {
	Type                   string  `json:"type"`
	PaidAmount             float64 `json:"paid_amount"`
	PaidCurrency           string  `json:"paid_currency"`
	ReceivedCreditAmount   float64 `json:"received_credit_amount"`
	ReceivedCreditCurrency string  `json:"received_credit_currency"`
	ReferenceFXRate        float64 `json:"reference_fx_rate"`
	RecordedAt             *string `json:"recorded_at"`
	Note                   *string `json:"note"`
}

type upstreamCostProfileRequest struct {
	RechargeCNYPerUSD *float64                    `json:"recharge_cny_per_usd"`
	ReferenceFXRate   *float64                    `json:"reference_fx_rate"`
	GroupMultiplier   *float64                    `json:"group_multiplier"`
	Note              *string                     `json:"note"`
	ModelFamilies     []upstreamCostFamilyRequest `json:"model_families"`
	BalanceEnabled    *bool                       `json:"balance_query_enabled"`
	BalanceProvider   *string                     `json:"balance_provider"`
	BalanceEndpoint   *string                     `json:"balance_endpoint"`
	BalanceAuthMode   *string                     `json:"balance_auth_mode"`
	BalanceAuthHeader *string                     `json:"balance_auth_header"`
}

type upstreamCostFamilyRequest struct {
	Family          string   `json:"family"`
	GroupMultiplier *float64 `json:"group_multiplier"`
	Note            *string  `json:"note"`
}

// ListUpstreamRechargeRecords handles listing recharge records for an account.
// GET /api/v1/admin/accounts/:id/recharge-records
func (h *AccountHandler) ListUpstreamRechargeRecords(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamRechargeService(c)
	if !ok {
		return
	}
	result, err := svc.ListUpstreamRechargeRecords(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// CreateUpstreamRechargeRecord handles creating an account recharge record.
// POST /api/v1/admin/accounts/:id/recharge-records
func (h *AccountHandler) CreateUpstreamRechargeRecord(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamRechargeService(c)
	if !ok {
		return
	}
	input, ok := bindUpstreamRechargeRecordInput(c, accountID)
	if !ok {
		return
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists && subject.UserID > 0 {
		input.CreatedBy = &subject.UserID
	}

	record, err := svc.CreateUpstreamRechargeRecord(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

// UpdateUpstreamRechargeRecord handles updating an account recharge record.
// PUT /api/v1/admin/accounts/:id/recharge-records/:record_id
func (h *AccountHandler) UpdateUpstreamRechargeRecord(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	recordID, ok := parseRecordIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamRechargeService(c)
	if !ok {
		return
	}
	input, ok := bindUpstreamRechargeRecordInput(c, accountID)
	if !ok {
		return
	}

	record, err := svc.UpdateUpstreamRechargeRecord(c.Request.Context(), recordID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

// DeleteUpstreamRechargeRecord handles soft-deleting an account recharge record.
// DELETE /api/v1/admin/accounts/:id/recharge-records/:record_id
func (h *AccountHandler) DeleteUpstreamRechargeRecord(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	recordID, ok := parseRecordIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamRechargeService(c)
	if !ok {
		return
	}
	if err := svc.DeleteUpstreamRechargeRecord(c.Request.Context(), accountID, recordID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Recharge record deleted successfully"})
}

// UpdateUpstreamCostProfile incrementally merges supplier cost fields into account extra.
// PATCH /api/v1/admin/accounts/:id/upstream-cost-profile
func (h *AccountHandler) UpdateUpstreamCostProfile(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}

	var req upstreamCostProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updates, ok := buildUpstreamCostProfileExtraUpdates(c, req)
	if !ok {
		return
	}
	if err := h.adminService.UpdateAccountExtra(c.Request.Context(), accountID, updates); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

func (h *AccountHandler) upstreamRechargeService(c *gin.Context) (upstreamRechargeService, bool) {
	svc, ok := h.adminService.(upstreamRechargeService)
	if !ok || svc == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusNotImplemented, "UPSTREAM_RECHARGE_UNAVAILABLE", "upstream recharge service is unavailable"))
		return nil, false
	}
	return svc, true
}

func parseAccountIDParam(c *gin.Context) (int64, bool) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "Invalid account ID")
		return 0, false
	}
	return accountID, true
}

func parseRecordIDParam(c *gin.Context) (int64, bool) {
	recordID, err := strconv.ParseInt(c.Param("record_id"), 10, 64)
	if err != nil || recordID <= 0 {
		response.BadRequest(c, "Invalid recharge record ID")
		return 0, false
	}
	return recordID, true
}

func bindUpstreamRechargeRecordInput(c *gin.Context, accountID int64) (service.UpstreamRechargeRecordInput, bool) {
	var req upstreamRechargeRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return service.UpstreamRechargeRecordInput{}, false
	}
	recordedAt, ok := parseOptionalRechargeRecordedAt(c, req.RecordedAt)
	if !ok {
		return service.UpstreamRechargeRecordInput{}, false
	}
	return service.UpstreamRechargeRecordInput{
		AccountID:              accountID,
		Type:                   req.Type,
		PaidAmount:             req.PaidAmount,
		PaidCurrency:           req.PaidCurrency,
		ReceivedCreditAmount:   req.ReceivedCreditAmount,
		ReceivedCreditCurrency: req.ReceivedCreditCurrency,
		ReferenceFXRate:        req.ReferenceFXRate,
		RecordedAt:             recordedAt,
		Note:                   req.Note,
	}, true
}

func parseOptionalRechargeRecordedAt(c *gin.Context, raw *string) (*time.Time, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		response.BadRequest(c, "Invalid recorded_at")
		return nil, false
	}
	return &parsed, true
}

func buildUpstreamCostProfileExtraUpdates(c *gin.Context, req upstreamCostProfileRequest) (map[string]any, bool) {
	updates := make(map[string]any)
	if req.RechargeCNYPerUSD != nil {
		value, ok := positiveFiniteField(c, "recharge_cny_per_usd", *req.RechargeCNYPerUSD)
		if !ok {
			return nil, false
		}
		updates["upstream_recharge_cny_per_usd"] = value
	}
	if req.ReferenceFXRate != nil {
		value, ok := positiveFiniteField(c, "reference_fx_rate", *req.ReferenceFXRate)
		if !ok {
			return nil, false
		}
		updates["upstream_reference_fx_rate"] = value
	}
	if req.GroupMultiplier != nil {
		value, ok := positiveFiniteField(c, "group_multiplier", *req.GroupMultiplier)
		if !ok {
			return nil, false
		}
		updates["upstream_group_multiplier"] = value
	}
	if req.Note != nil {
		if note := strings.TrimSpace(*req.Note); note != "" {
			updates["upstream_cost_note"] = note
		}
	}
	if len(req.ModelFamilies) > 0 {
		families, ok := normalizeUpstreamCostFamilies(c, req.ModelFamilies)
		if !ok {
			return nil, false
		}
		updates["upstream_cost_model_families"] = families
	}
	if req.BalanceEnabled != nil {
		updates["upstream_balance_query_enabled"] = *req.BalanceEnabled
		if *req.BalanceEnabled {
			balanceUpdates, ok := normalizeUpstreamBalanceProfile(c, req)
			if !ok {
				return nil, false
			}
			for key, value := range balanceUpdates {
				updates[key] = value
			}
		}
	}
	if len(updates) == 0 {
		response.BadRequest(c, "No upstream cost fields provided")
		return nil, false
	}
	return updates, true
}

func normalizeUpstreamBalanceProfile(c *gin.Context, req upstreamCostProfileRequest) (map[string]any, bool) {
	provider := service.UpstreamBalanceDefaultProvider
	if req.BalanceProvider != nil {
		switch strings.TrimSpace(*req.BalanceProvider) {
		case service.UpstreamBalanceProviderSub2API, service.UpstreamBalanceProviderNewAPICompatible:
			provider = strings.TrimSpace(*req.BalanceProvider)
		default:
			response.BadRequest(c, "balance_provider is unsupported")
			return nil, false
		}
	}

	authMode := service.UpstreamBalanceAuthModeAccountAPIKey
	if req.BalanceAuthMode != nil {
		switch strings.TrimSpace(*req.BalanceAuthMode) {
		case service.UpstreamBalanceAuthModeAccountAPIKey,
			service.UpstreamBalanceAuthModeBearerToken,
			service.UpstreamBalanceAuthModeCustomHeader:
			authMode = strings.TrimSpace(*req.BalanceAuthMode)
		default:
			response.BadRequest(c, "balance_auth_mode is unsupported")
			return nil, false
		}
	}

	endpoint := defaultBalanceEndpointForProvider(provider)
	if req.BalanceEndpoint != nil {
		if value := strings.TrimSpace(*req.BalanceEndpoint); value != "" {
			endpoint = value
		}
	}

	cfg := service.ResolveUpstreamBalanceConfig(map[string]any{
		"upstream_balance_query_enabled": true,
		"upstream_balance_provider":      provider,
		"upstream_balance_endpoint":      endpoint,
		"upstream_balance_auth_mode":     authMode,
	})

	updates := map[string]any{
		"upstream_balance_provider":  cfg.Provider,
		"upstream_balance_endpoint":  cfg.Endpoint,
		"upstream_balance_auth_mode": cfg.AuthMode,
	}
	if cfg.AuthMode == service.UpstreamBalanceAuthModeCustomHeader {
		header := "Authorization"
		if req.BalanceAuthHeader != nil {
			if value := strings.TrimSpace(*req.BalanceAuthHeader); value != "" {
				header = value
			}
		}
		updates["upstream_balance_auth_header"] = header
	}
	return updates, true
}

func defaultBalanceEndpointForProvider(provider string) string {
	switch provider {
	case service.UpstreamBalanceProviderNewAPICompatible:
		return service.UpstreamBalanceNewAPIDefaultEndpoint
	default:
		return service.UpstreamBalanceDefaultEndpoint
	}
}

func normalizeUpstreamCostFamilies(c *gin.Context, input []upstreamCostFamilyRequest) ([]map[string]any, bool) {
	families := make([]map[string]any, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		family := strings.TrimSpace(item.Family)
		if family == "" || item.GroupMultiplier == nil {
			continue
		}
		key := strings.ToLower(family)
		if _, exists := seen[key]; exists {
			continue
		}
		multiplier, ok := positiveFiniteField(c, "model_families.group_multiplier", *item.GroupMultiplier)
		if !ok {
			return nil, false
		}
		entry := map[string]any{
			"family":           family,
			"group_multiplier": multiplier,
		}
		if item.Note != nil {
			if note := strings.TrimSpace(*item.Note); note != "" {
				entry["note"] = note
			}
		}
		seen[key] = struct{}{}
		families = append(families, entry)
	}
	return families, true
}

func positiveFiniteField(c *gin.Context, field string, value float64) (float64, bool) {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		response.BadRequest(c, field+" must be greater than 0")
		return 0, false
	}
	return value, true
}
