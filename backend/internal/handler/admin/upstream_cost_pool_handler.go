package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type upstreamCostPoolService interface {
	ListUpstreamSuppliers(ctx context.Context) ([]service.UpstreamSupplier, error)
	CreateUpstreamSupplier(ctx context.Context, input service.CreateUpstreamSupplierInput) (*service.UpstreamSupplier, error)
	UpdateUpstreamSupplier(ctx context.Context, input service.UpdateUpstreamSupplierInput) (*service.UpstreamSupplier, error)
	DeleteUpstreamSupplier(ctx context.Context, supplierID int64) error
	ListUpstreamCostPools(ctx context.Context) ([]service.UpstreamCostPool, error)
	GetUpstreamCostPool(ctx context.Context, poolID int64) (*service.UpstreamCostPool, error)
	ListUpstreamCostPoolAccounts(ctx context.Context, poolID int64) ([]service.UpstreamAccountCostBinding, error)
	ListUpstreamCostPoolRechargeRecords(ctx context.Context, poolID int64) (*service.UpstreamRechargeRecordsResult, error)
	CreateUpstreamCostPoolRechargeRecord(ctx context.Context, poolID int64, input service.UpstreamRechargeRecordInput) (*service.UpstreamRechargeRecord, error)
	UpdateUpstreamCostPoolRechargeRecord(ctx context.Context, poolID, recordID int64, input service.UpstreamRechargeRecordInput) (*service.UpstreamRechargeRecord, error)
	DeleteUpstreamCostPoolRechargeRecord(ctx context.Context, poolID, recordID int64) error
	GetAccountUpstreamCostBinding(ctx context.Context, accountID int64) (*service.UpstreamAccountCostBinding, error)
	UpdateAccountUpstreamCostBinding(ctx context.Context, input service.UpstreamCostBindingInput) (*service.UpstreamAccountCostBinding, error)
	UpdateAccountUpstreamSupplierBinding(ctx context.Context, input service.UpstreamSupplierBindingInput) (*service.UpstreamAccountCostBinding, error)
}

type upstreamCostPoolRechargeRecordRequest struct {
	AccountID              *int64  `json:"account_id"`
	Type                   string  `json:"type"`
	PaidAmount             float64 `json:"paid_amount"`
	PaidCurrency           string  `json:"paid_currency"`
	ReceivedCreditAmount   float64 `json:"received_credit_amount"`
	ReceivedCreditCurrency string  `json:"received_credit_currency"`
	ReferenceFXRate        float64 `json:"reference_fx_rate"`
	RecordedAt             *string `json:"recorded_at"`
	Note                   *string `json:"note"`
}

type upstreamCostBindingRequest struct {
	CostPoolID              int64                       `json:"cost_pool_id"`
	UpstreamGroupName       *string                     `json:"upstream_group_name"`
	UpstreamGroupMultiplier *float64                    `json:"upstream_group_multiplier"`
	DefaultMultiplier       *float64                    `json:"default_multiplier"`
	ModelFamilies           []upstreamCostFamilyRequest `json:"model_families"`
	Note                    *string                     `json:"note"`
}

type upstreamSupplierBindingRequest struct {
	SupplierID              *int64                      `json:"supplier_id"`
	SupplierName            *string                     `json:"supplier_name"`
	CostPoolID              *int64                      `json:"cost_pool_id"`
	UpstreamGroupName       *string                     `json:"upstream_group_name"`
	UpstreamGroupMultiplier *float64                    `json:"upstream_group_multiplier"`
	DefaultMultiplier       *float64                    `json:"default_multiplier"`
	ModelFamilies           []upstreamCostFamilyRequest `json:"model_families"`
	Note                    *string                     `json:"note"`
}

type upstreamSupplierRequest struct {
	Name                      string  `json:"name"`
	Note                      *string `json:"note"`
	DefaultEffectiveCNYPerUSD float64 `json:"default_effective_cny_per_usd"`
	DefaultReferenceFXRate    float64 `json:"default_reference_fx_rate"`
}

type upstreamSupplierUpdateRequest struct {
	Name                      *string  `json:"name"`
	Note                      *string  `json:"note"`
	Status                    *string  `json:"status"`
	DefaultEffectiveCNYPerUSD *float64 `json:"default_effective_cny_per_usd"`
	DefaultReferenceFXRate    *float64 `json:"default_reference_fx_rate"`
}

// ListUpstreamSuppliers handles listing upstream suppliers.
// GET /api/v1/admin/upstream-suppliers
func (h *AccountHandler) ListUpstreamSuppliers(c *gin.Context) {
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	items, err := svc.ListUpstreamSuppliers(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// CreateUpstreamSupplier handles creating a reusable upstream supplier.
// POST /api/v1/admin/upstream-suppliers
func (h *AccountHandler) CreateUpstreamSupplier(c *gin.Context) {
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}

	var req upstreamSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input := service.CreateUpstreamSupplierInput{
		Name:                      req.Name,
		Note:                      req.Note,
		DefaultEffectiveCNYPerUSD: req.DefaultEffectiveCNYPerUSD,
		DefaultReferenceFXRate:    req.DefaultReferenceFXRate,
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists && subject.UserID > 0 {
		input.CreatedBy = &subject.UserID
	}

	supplier, err := svc.CreateUpstreamSupplier(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, supplier)
}

// UpdateUpstreamSupplier renames, re-notes or archives an upstream supplier.
// PATCH /api/v1/admin/upstream-suppliers/:supplier_id
func (h *AccountHandler) UpdateUpstreamSupplier(c *gin.Context) {
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	supplierID, ok := parseSupplierIDParam(c)
	if !ok {
		return
	}

	var req upstreamSupplierUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	supplier, err := svc.UpdateUpstreamSupplier(c.Request.Context(), service.UpdateUpstreamSupplierInput{
		SupplierID:                supplierID,
		Name:                      req.Name,
		Note:                      req.Note,
		Status:                    req.Status,
		DefaultEffectiveCNYPerUSD: req.DefaultEffectiveCNYPerUSD,
		DefaultReferenceFXRate:    req.DefaultReferenceFXRate,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, supplier)
}

// DeleteUpstreamSupplier hard-deletes a clean upstream supplier.
// DELETE /api/v1/admin/upstream-suppliers/:supplier_id
func (h *AccountHandler) DeleteUpstreamSupplier(c *gin.Context) {
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	supplierID, ok := parseSupplierIDParam(c)
	if !ok {
		return
	}
	if err := svc.DeleteUpstreamSupplier(c.Request.Context(), supplierID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ListUpstreamCostPools handles listing upstream cost pools.
// GET /api/v1/admin/upstream-cost-pools
func (h *AccountHandler) ListUpstreamCostPools(c *gin.Context) {
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	items, err := svc.ListUpstreamCostPools(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// GetUpstreamCostPool handles fetching a cost pool detail.
// GET /api/v1/admin/upstream-cost-pools/:pool_id
func (h *AccountHandler) GetUpstreamCostPool(c *gin.Context) {
	poolID, ok := parsePoolIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	item, err := svc.GetUpstreamCostPool(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

// ListUpstreamCostPoolAccounts handles listing active account bindings for a cost pool.
// GET /api/v1/admin/upstream-cost-pools/:pool_id/accounts
func (h *AccountHandler) ListUpstreamCostPoolAccounts(c *gin.Context) {
	poolID, ok := parsePoolIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	items, err := svc.ListUpstreamCostPoolAccounts(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

// ListUpstreamCostPoolRechargeRecords handles listing pool-owned recharge records.
// GET /api/v1/admin/upstream-cost-pools/:pool_id/recharge-records
func (h *AccountHandler) ListUpstreamCostPoolRechargeRecords(c *gin.Context) {
	poolID, ok := parsePoolIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	result, err := svc.ListUpstreamCostPoolRechargeRecords(c.Request.Context(), poolID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// CreateUpstreamCostPoolRechargeRecord handles creating pool-owned recharge records.
// POST /api/v1/admin/upstream-cost-pools/:pool_id/recharge-records
func (h *AccountHandler) CreateUpstreamCostPoolRechargeRecord(c *gin.Context) {
	poolID, ok := parsePoolIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	input, ok := bindUpstreamCostPoolRechargeRecordInput(c)
	if !ok {
		return
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists && subject.UserID > 0 {
		input.CreatedBy = &subject.UserID
	}
	record, err := svc.CreateUpstreamCostPoolRechargeRecord(c.Request.Context(), poolID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

// UpdateUpstreamCostPoolRechargeRecord handles updating pool-owned recharge records.
// PUT /api/v1/admin/upstream-cost-pools/:pool_id/recharge-records/:record_id
func (h *AccountHandler) UpdateUpstreamCostPoolRechargeRecord(c *gin.Context) {
	poolID, ok := parsePoolIDParam(c)
	if !ok {
		return
	}
	recordID, ok := parseRecordIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	input, ok := bindUpstreamCostPoolRechargeRecordInput(c)
	if !ok {
		return
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists && subject.UserID > 0 {
		input.CreatedBy = &subject.UserID
	}

	record, err := svc.UpdateUpstreamCostPoolRechargeRecord(c.Request.Context(), poolID, recordID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, record)
}

// DeleteUpstreamCostPoolRechargeRecord handles soft-deleting pool-owned recharge records.
// DELETE /api/v1/admin/upstream-cost-pools/:pool_id/recharge-records/:record_id
func (h *AccountHandler) DeleteUpstreamCostPoolRechargeRecord(c *gin.Context) {
	poolID, ok := parsePoolIDParam(c)
	if !ok {
		return
	}
	recordID, ok := parseRecordIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	if err := svc.DeleteUpstreamCostPoolRechargeRecord(c.Request.Context(), poolID, recordID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Recharge record deleted successfully"})
}

// GetAccountUpstreamCostBinding handles fetching an account's active cost binding.
// GET /api/v1/admin/accounts/:id/upstream-cost-binding
func (h *AccountHandler) GetAccountUpstreamCostBinding(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	binding, err := svc.GetAccountUpstreamCostBinding(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, binding)
}

// UpdateAccountUpstreamCostBinding handles replacing an account's active cost binding.
// PUT /api/v1/admin/accounts/:id/upstream-cost-binding
func (h *AccountHandler) UpdateAccountUpstreamCostBinding(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}

	var req upstreamCostBindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	input := service.UpstreamCostBindingInput{
		AccountID:              accountID,
		CostPoolID:             req.CostPoolID,
		UpstreamGroupName:      req.UpstreamGroupName,
		DefaultMultiplier:      1,
		ModelFamilyMultipliers: make([]service.UpstreamCostModelFamilyMultiplier, 0, len(req.ModelFamilies)),
		Note:                   req.Note,
	}
	if req.DefaultMultiplier != nil {
		input.DefaultMultiplier = *req.DefaultMultiplier
	}
	if req.UpstreamGroupMultiplier != nil {
		input.DefaultMultiplier = *req.UpstreamGroupMultiplier
	}
	for _, item := range req.ModelFamilies {
		multiplier := 0.0
		if item.GroupMultiplier != nil {
			multiplier = *item.GroupMultiplier
		}
		input.ModelFamilyMultipliers = append(input.ModelFamilyMultipliers, service.UpstreamCostModelFamilyMultiplier{
			Family:          item.Family,
			GroupMultiplier: multiplier,
			Note:            item.Note,
		})
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists && subject.UserID > 0 {
		input.CreatedBy = &subject.UserID
	}

	binding, err := svc.UpdateAccountUpstreamCostBinding(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, binding)
}

// UpdateAccountUpstreamSupplierBinding handles replacing an account's active cost binding by supplier.
// PUT /api/v1/admin/accounts/:id/upstream-supplier-binding
func (h *AccountHandler) UpdateAccountUpstreamSupplierBinding(c *gin.Context) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}

	body, err := c.GetRawData()
	if err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var req upstreamSupplierBindingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(body, &raw)
	input := service.UpstreamSupplierBindingInput{
		AccountID:              accountID,
		DefaultMultiplier:      1,
		ModelFamilyMultipliers: make([]service.UpstreamCostModelFamilyMultiplier, 0, len(req.ModelFamilies)),
		Note:                   req.Note,
	}
	input.UpstreamGroupName = req.UpstreamGroupName
	if supplierIDRaw, ok := raw["supplier_id"]; ok && string(supplierIDRaw) == "null" {
		input.Clear = true
	}
	if req.SupplierID != nil {
		input.SupplierID = *req.SupplierID
		if *req.SupplierID <= 0 {
			input.Clear = true
		}
	}
	if req.SupplierName != nil {
		input.SupplierName = *req.SupplierName
	}
	if req.CostPoolID != nil {
		input.CostPoolID = *req.CostPoolID
	}
	if req.DefaultMultiplier != nil {
		input.DefaultMultiplier = *req.DefaultMultiplier
	}
	if req.UpstreamGroupMultiplier != nil {
		input.DefaultMultiplier = *req.UpstreamGroupMultiplier
	}
	for _, item := range req.ModelFamilies {
		multiplier := 0.0
		if item.GroupMultiplier != nil {
			multiplier = *item.GroupMultiplier
		}
		input.ModelFamilyMultipliers = append(input.ModelFamilyMultipliers, service.UpstreamCostModelFamilyMultiplier{
			Family:          item.Family,
			GroupMultiplier: multiplier,
			Note:            item.Note,
		})
	}
	if subject, exists := middleware.GetAuthSubjectFromContext(c); exists && subject.UserID > 0 {
		input.CreatedBy = &subject.UserID
	}

	binding, err := svc.UpdateAccountUpstreamSupplierBinding(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, binding)
}

func (h *AccountHandler) upstreamCostPoolService(c *gin.Context) (upstreamCostPoolService, bool) {
	svc, ok := h.adminService.(upstreamCostPoolService)
	if !ok || svc == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusNotImplemented, "UPSTREAM_COST_POOL_UNAVAILABLE", "upstream cost pool service is unavailable"))
		return nil, false
	}
	return svc, true
}

func parsePoolIDParam(c *gin.Context) (int64, bool) {
	raw := c.Param("pool_id")
	if raw == "" {
		raw = c.Param("id")
	}
	poolID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || poolID <= 0 {
		response.BadRequest(c, "Invalid upstream cost pool ID")
		return 0, false
	}
	return poolID, true
}

func parseSupplierIDParam(c *gin.Context) (int64, bool) {
	raw := c.Param("supplier_id")
	if raw == "" {
		raw = c.Param("id")
	}
	supplierID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || supplierID <= 0 {
		response.BadRequest(c, "Invalid upstream supplier ID")
		return 0, false
	}
	return supplierID, true
}

func bindUpstreamCostPoolRechargeRecordInput(c *gin.Context) (service.UpstreamRechargeRecordInput, bool) {
	var req upstreamCostPoolRechargeRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return service.UpstreamRechargeRecordInput{}, false
	}
	recordedAt, ok := parseOptionalRechargeRecordedAt(c, req.RecordedAt)
	if !ok {
		return service.UpstreamRechargeRecordInput{}, false
	}
	var accountID int64
	if req.AccountID != nil {
		if *req.AccountID <= 0 {
			response.BadRequest(c, "Invalid account ID")
			return service.UpstreamRechargeRecordInput{}, false
		}
		accountID = *req.AccountID
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
