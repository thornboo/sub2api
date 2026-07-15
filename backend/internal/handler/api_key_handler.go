// Package handler provides HTTP request handlers for the application.
package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// APIKeyHandler handles API key-related requests
type APIKeyHandler struct {
	apiKeyService *service.APIKeyService
}

// NewAPIKeyHandler creates a new APIKeyHandler
func NewAPIKeyHandler(apiKeyService *service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
	}
}

// CreateAPIKeyRequest represents the create API key request payload
type CreateAPIKeyRequest struct {
	Name          string   `json:"name" binding:"required"`
	Tags          []string `json:"tags"`
	GroupID       *int64   `json:"group_id"`        // nullable
	CustomKey     *string  `json:"custom_key"`      // 可选的自定义key
	IPWhitelist   []string `json:"ip_whitelist"`    // IP 白名单
	IPBlacklist   []string `json:"ip_blacklist"`    // IP 黑名单
	Quota         *float64 `json:"quota"`           // 配额限制 (USD)
	ExpiresInDays *int     `json:"expires_in_days"` // 过期天数

	// Rate limit fields (0 = unlimited)
	RateLimit5h *float64 `json:"rate_limit_5h"`
	RateLimit1d *float64 `json:"rate_limit_1d"`
	RateLimit7d *float64 `json:"rate_limit_7d"`
}

type BatchCreateAPIKeysRequest struct {
	Count         *int     `json:"count"`
	NameTemplate  *string  `json:"name_template"`
	Names         []string `json:"names"`
	Tags          []string `json:"tags"`
	GroupID       *int64   `json:"group_id"`
	IPWhitelist   []string `json:"ip_whitelist"`
	IPBlacklist   []string `json:"ip_blacklist"`
	Quota         *float64 `json:"quota"`
	ExpiresInDays *int     `json:"expires_in_days"`
	RateLimit5h   *float64 `json:"rate_limit_5h"`
	RateLimit1d   *float64 `json:"rate_limit_1d"`
	RateLimit7d   *float64 `json:"rate_limit_7d"`
}

type BatchCreateAPIKeysResponse struct {
	Keys               []dto.APIKey `json:"keys"`
	Created            int          `json:"created"`
	MaxAllowed         int          `json:"max_allowed"`
	PlaintextAvailable bool         `json:"plaintext_available"`
}

type APIKeyBatchFiltersRequest struct {
	Search  string   `json:"search"`
	Status  string   `json:"status"`
	GroupID *int64   `json:"group_id"`
	Tags    []string `json:"tags"`
}

type BatchUpdateAPIKeysRequest struct {
	IDs     []int64                   `json:"ids"`
	ApplyTo string                    `json:"apply_to"`
	Filters APIKeyBatchFiltersRequest `json:"filters"`

	UpdateGroup bool   `json:"update_group"`
	GroupID     *int64 `json:"group_id"`

	UpdateStatus bool   `json:"update_status"`
	Status       string `json:"status"`

	UpdateQuota bool    `json:"update_quota"`
	QuotaMode   string  `json:"quota_mode"`
	QuotaValue  float64 `json:"quota_value"`

	UpdateExpiration bool    `json:"update_expiration"`
	ExpiresAt        *string `json:"expires_at"`

	UpdateRateLimit       bool     `json:"update_rate_limit"`
	RateLimit5h           float64  `json:"rate_limit_5h"`
	RateLimit1d           float64  `json:"rate_limit_1d"`
	RateLimit7d           float64  `json:"rate_limit_7d"`
	ResetRateLimitUsage   bool     `json:"reset_rate_limit_usage"`
	UpdateIPAccessControl bool     `json:"update_ip_access_control"`
	IPWhitelist           []string `json:"ip_whitelist"`
	IPBlacklist           []string `json:"ip_blacklist"`
	UpdateTags            bool     `json:"update_tags"`
	TagsMode              string   `json:"tags_mode"`
	Tags                  []string `json:"tags"`
}

type BatchUpdateAPIKeysResponse struct {
	Updated int `json:"updated"`
}

type BatchDeleteAPIKeysRequest struct {
	IDs     []int64                   `json:"ids"`
	ApplyTo string                    `json:"apply_to"`
	Filters APIKeyBatchFiltersRequest `json:"filters"`
}

type BatchDeleteAPIKeysResponse struct {
	Deleted int `json:"deleted"`
}

type PublicAPIKeyStatusRequest struct {
	Key string `json:"key" binding:"required"`
}

type PublicAPIKeyStatusResponse struct {
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	IsActive   bool       `json:"is_active"`
	GroupID    *int64     `json:"group_id"`
	GroupName  string     `json:"group_name,omitempty"`
	Platform   string     `json:"platform,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`

	Quota          float64 `json:"quota"`
	QuotaUsed      float64 `json:"quota_used"`
	QuotaRemaining float64 `json:"quota_remaining"`

	RateLimit5h float64    `json:"rate_limit_5h"`
	RateLimit1d float64    `json:"rate_limit_1d"`
	RateLimit7d float64    `json:"rate_limit_7d"`
	Usage5h     float64    `json:"usage_5h"`
	Usage1d     float64    `json:"usage_1d"`
	Usage7d     float64    `json:"usage_7d"`
	Reset5hAt   *time.Time `json:"reset_5h_at"`
	Reset1dAt   *time.Time `json:"reset_1d_at"`
	Reset7dAt   *time.Time `json:"reset_7d_at"`
}

// UpdateAPIKeyRequest represents the update API key request payload.
// The inactive status value is accepted only as a legacy alias and is normalized to disabled by the service layer.
// TODO(v1.3.0): remove inactive after migration 153 has shipped through the supported upgrade window.
type UpdateAPIKeyRequest struct {
	Name        string    `json:"name"`
	Tags        *[]string `json:"tags"`
	GroupID     *int64    `json:"group_id"`
	Status      string    `json:"status" binding:"omitempty,oneof=active disabled inactive"`
	IPWhitelist []string  `json:"ip_whitelist"` // IP 白名单
	IPBlacklist []string  `json:"ip_blacklist"` // IP 黑名单
	Quota       *float64  `json:"quota"`        // 配额限制 (USD), 0=无限制
	ExpiresAt   *string   `json:"expires_at"`   // 过期时间 (ISO 8601)
	ResetQuota  *bool     `json:"reset_quota"`  // 重置已用配额

	// Rate limit fields (nil = no change, 0 = unlimited)
	RateLimit5h         *float64 `json:"rate_limit_5h"`
	RateLimit1d         *float64 `json:"rate_limit_1d"`
	RateLimit7d         *float64 `json:"rate_limit_7d"`
	ResetRateLimitUsage *bool    `json:"reset_rate_limit_usage"` // 重置限速用量
}

func maskAPIKeyForIdempotencyReplay(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 12 {
		return "***"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func redactBatchCreateResponseForIdempotency(data any) (any, error) {
	resp, ok := data.(*BatchCreateAPIKeysResponse)
	if !ok || resp == nil {
		return data, nil
	}
	redacted := *resp
	redacted.PlaintextAvailable = false
	redacted.Keys = make([]dto.APIKey, len(resp.Keys))
	for i := range resp.Keys {
		redacted.Keys[i] = resp.Keys[i]
		redacted.Keys[i].Key = maskAPIKeyForIdempotencyReplay(resp.Keys[i].Key)
	}
	return &redacted, nil
}

func parseAPIKeyTagsParam(values ...string) []string {
	tags := make([]string, 0)
	for _, value := range values {
		for _, tag := range strings.FieldsFunc(value, func(r rune) bool {
			return r == ',' || r == '\n' || r == '\r'
		}) {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

func apiKeyBatchFiltersRequestToService(req APIKeyBatchFiltersRequest) service.APIKeyBatchFilters {
	return service.APIKeyBatchFilters{
		Search:  req.Search,
		Status:  req.Status,
		GroupID: req.GroupID,
		Tags:    req.Tags,
	}
}

// List handles listing user's API keys with pagination
// GET /api/v1/api-keys
func (h *APIKeyHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	// Parse filter parameters
	var filters service.APIKeyListFilters
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		if len(search) > 100 {
			search = search[:100]
		}
		filters.Search = search
	}
	filters.Status = c.Query("status")
	if groupIDStr := c.Query("group_id"); groupIDStr != "" {
		gid, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err == nil {
			filters.GroupID = &gid
		}
	}
	if rawTags := c.Query("tags"); rawTags != "" {
		filters.Tags = append(filters.Tags, parseAPIKeyTagsParam(rawTags)...)
	}
	if repeatedTags := c.QueryArray("tag"); len(repeatedTags) > 0 {
		filters.Tags = append(filters.Tags, parseAPIKeyTagsParam(repeatedTags...)...)
	}

	keys, result, err := h.apiKeyService.List(c.Request.Context(), subject.UserID, params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.APIKey, 0, len(keys))
	for i := range keys {
		out = append(out, *dto.APIKeyFromService(&keys[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// ListTags handles listing distinct API key tags for the current user.
// GET /api/v1/keys/tags
func (h *APIKeyHandler) ListTags(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	tags, err := h.apiKeyService.ListTags(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"tags": tags})
}

// GetByID handles getting a single API key
// GET /api/v1/api-keys/:id
func (h *APIKeyHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	key, err := h.apiKeyService.GetByID(c.Request.Context(), keyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Member keys are managed only through the enterprise-member surface. The
	// legacy API-key endpoint must not disclose their plaintext or internal
	// membership metadata even to the enterprise owner.
	if key.UserID != subject.UserID || key.MemberID != nil {
		response.NotFound(c, "API key not found")
		return
	}

	response.Success(c, dto.APIKeyFromService(key))
}

// Create handles creating a new API key
// POST /api/v1/api-keys
func (h *APIKeyHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.CreateAPIKeyRequest{
		Name:          req.Name,
		Tags:          req.Tags,
		GroupID:       req.GroupID,
		CustomKey:     req.CustomKey,
		IPWhitelist:   req.IPWhitelist,
		IPBlacklist:   req.IPBlacklist,
		ExpiresInDays: req.ExpiresInDays,
	}
	if req.Quota != nil {
		svcReq.Quota = *req.Quota
	}
	if req.RateLimit5h != nil {
		svcReq.RateLimit5h = *req.RateLimit5h
	}
	if req.RateLimit1d != nil {
		svcReq.RateLimit1d = *req.RateLimit1d
	}
	if req.RateLimit7d != nil {
		svcReq.RateLimit7d = *req.RateLimit7d
	}

	executeUserIdempotentJSON(c, "user.api_keys.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		key, err := h.apiKeyService.Create(ctx, subject.UserID, svcReq)
		if err != nil {
			return nil, err
		}
		return dto.APIKeyFromService(key), nil
	})
}

// BatchCreate handles creating API keys as one all-or-nothing batch.
// POST /api/v1/keys/batch
func (h *APIKeyHandler) BatchCreate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchCreateAPIKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.BatchCreateAPIKeysRequest{
		NameTemplate:  req.NameTemplate,
		Names:         req.Names,
		Tags:          req.Tags,
		GroupID:       req.GroupID,
		IPWhitelist:   req.IPWhitelist,
		IPBlacklist:   req.IPBlacklist,
		ExpiresInDays: req.ExpiresInDays,
	}
	if req.Count != nil {
		svcReq.Count = *req.Count
	}
	if req.Quota != nil {
		svcReq.Quota = *req.Quota
	}
	if req.RateLimit5h != nil {
		svcReq.RateLimit5h = *req.RateLimit5h
	}
	if req.RateLimit1d != nil {
		svcReq.RateLimit1d = *req.RateLimit1d
	}
	if req.RateLimit7d != nil {
		svcReq.RateLimit7d = *req.RateLimit7d
	}

	executeUserIdempotentJSONWithStoredResponse(
		c,
		"user.api_keys.batch_create",
		req,
		service.DefaultWriteIdempotencyTTL(),
		redactBatchCreateResponseForIdempotency,
		func(ctx context.Context) (any, error) {
			result, err := h.apiKeyService.BatchCreate(ctx, subject.UserID, svcReq)
			if err != nil {
				return nil, err
			}
			keys := make([]dto.APIKey, 0, len(result.Keys))
			for i := range result.Keys {
				keys = append(keys, *dto.APIKeyFromService(&result.Keys[i]))
			}
			return &BatchCreateAPIKeysResponse{
				Keys:               keys,
				Created:            result.Created,
				MaxAllowed:         result.MaxAllowed,
				PlaintextAvailable: true,
			}, nil
		},
	)
}

// BatchUpdate handles all-or-nothing user API key configuration updates.
// POST /api/v1/keys/batch-update
func (h *APIKeyHandler) BatchUpdate(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchUpdateAPIKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.BatchUpdateAPIKeysRequest{
		IDs:                   req.IDs,
		ApplyTo:               req.ApplyTo,
		Filters:               apiKeyBatchFiltersRequestToService(req.Filters),
		UpdateGroup:           req.UpdateGroup,
		GroupID:               req.GroupID,
		UpdateStatus:          req.UpdateStatus,
		Status:                req.Status,
		UpdateQuota:           req.UpdateQuota,
		QuotaMode:             req.QuotaMode,
		QuotaValue:            req.QuotaValue,
		UpdateExpiration:      req.UpdateExpiration,
		UpdateRateLimit:       req.UpdateRateLimit,
		RateLimit5h:           req.RateLimit5h,
		RateLimit1d:           req.RateLimit1d,
		RateLimit7d:           req.RateLimit7d,
		ResetRateLimitUsage:   req.ResetRateLimitUsage,
		UpdateIPAccessControl: req.UpdateIPAccessControl,
		IPWhitelist:           req.IPWhitelist,
		IPBlacklist:           req.IPBlacklist,
		UpdateTags:            req.UpdateTags,
		TagsMode:              req.TagsMode,
		Tags:                  req.Tags,
	}
	if req.UpdateExpiration && req.ExpiresAt != nil && strings.TrimSpace(*req.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			response.BadRequest(c, "Invalid expires_at format: "+err.Error())
			return
		}
		svcReq.ExpiresAt = &t
	}

	executeUserIdempotentJSON(c, "user.api_keys.batch_update", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result, err := h.apiKeyService.BatchUpdate(ctx, subject.UserID, svcReq)
		if err != nil {
			return nil, err
		}
		return &BatchUpdateAPIKeysResponse{Updated: result.Updated}, nil
	})
}

// BatchDelete handles all-or-nothing user API key soft deletion.
// POST /api/v1/keys/batch-delete
func (h *APIKeyHandler) BatchDelete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchDeleteAPIKeysRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeUserIdempotentJSON(c, "user.api_keys.batch_delete", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		result, err := h.apiKeyService.BatchDelete(ctx, subject.UserID, service.BatchDeleteAPIKeysRequest{
			IDs:     req.IDs,
			ApplyTo: req.ApplyTo,
			Filters: apiKeyBatchFiltersRequestToService(req.Filters),
		})
		if err != nil {
			return nil, err
		}
		return &BatchDeleteAPIKeysResponse{Deleted: result.Deleted}, nil
	})
}

// PublicStatus returns a read-only status summary for callers that only possess an API key.
// POST /api/v1/key/status
func (h *APIKeyHandler) PublicStatus(c *gin.Context) {
	var req PublicAPIKeyStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	status, err := h.apiKeyService.GetPublicStatusByKey(c.Request.Context(), req.Key)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, PublicAPIKeyStatusResponse{
		Name:           status.Name,
		Status:         status.Status,
		IsActive:       status.IsActive,
		GroupID:        status.GroupID,
		GroupName:      status.GroupName,
		Platform:       status.GroupPlatform,
		LastUsedAt:     status.LastUsedAt,
		CreatedAt:      status.CreatedAt,
		ExpiresAt:      status.ExpiresAt,
		Quota:          status.Quota,
		QuotaUsed:      status.QuotaUsed,
		QuotaRemaining: status.QuotaRemaining,
		RateLimit5h:    status.RateLimit5h,
		RateLimit1d:    status.RateLimit1d,
		RateLimit7d:    status.RateLimit7d,
		Usage5h:        status.Usage5h,
		Usage1d:        status.Usage1d,
		Usage7d:        status.Usage7d,
		Reset5hAt:      status.Reset5hAt,
		Reset1dAt:      status.Reset1dAt,
		Reset7dAt:      status.Reset7dAt,
	})
}

// Update handles updating an API key
// PUT /api/v1/api-keys/:id
func (h *APIKeyHandler) Update(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	var req UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	svcReq := service.UpdateAPIKeyRequest{
		Tags:                req.Tags,
		IPWhitelist:         req.IPWhitelist,
		IPBlacklist:         req.IPBlacklist,
		Quota:               req.Quota,
		ResetQuota:          req.ResetQuota,
		RateLimit5h:         req.RateLimit5h,
		RateLimit1d:         req.RateLimit1d,
		RateLimit7d:         req.RateLimit7d,
		ResetRateLimitUsage: req.ResetRateLimitUsage,
	}
	if req.Name != "" {
		svcReq.Name = &req.Name
	}
	svcReq.GroupID = req.GroupID
	if req.Status != "" {
		svcReq.Status = &req.Status
	}
	// Parse expires_at if provided
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			// Empty string means clear expiration
			svcReq.ExpiresAt = nil
			svcReq.ClearExpiration = true
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				response.BadRequest(c, "Invalid expires_at format: "+err.Error())
				return
			}
			svcReq.ExpiresAt = &t
		}
	}

	key, err := h.apiKeyService.Update(c.Request.Context(), keyID, subject.UserID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.APIKeyFromService(key))
}

// Delete handles deleting an API key
// DELETE /api/v1/api-keys/:id
func (h *APIKeyHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid key ID")
		return
	}

	err = h.apiKeyService.Delete(c.Request.Context(), keyID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "API key deleted successfully"})
}

// GetAvailableGroups 获取用户可以绑定的分组列表
// GET /api/v1/groups/available
func (h *APIKeyHandler) GetAvailableGroups(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.Group, 0, len(groups))
	for i := range groups {
		out = append(out, *dto.GroupFromService(&groups[i]))
	}
	response.Success(c, out)
}

// GetUserGroupRates 获取当前用户的专属分组倍率配置
// GET /api/v1/groups/rates
func (h *APIKeyHandler) GetUserGroupRates(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	rates, err := h.apiKeyService.GetUserGroupRates(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, rates)
}
