package handler

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	publicKeyUsageSessionCookie = "sub2api_key_usage_session"
	publicKeyUsageSessionPath   = "/api/v1/key"
	publicKeyUsageMaxRangeDays  = 90
	publicKeyUsageExportLimit   = 5000
)

type publicKeyUsageSessionResponse struct {
	Valid             bool       `json:"valid"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	AbsoluteExpiresAt *time.Time `json:"absolute_expires_at,omitempty"`
}

type publicKeyUsageIdentity struct {
	Name          string     `json:"name"`
	KeyPrefix     string     `json:"key_prefix"`
	Status        string     `json:"status"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"created_at"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	IPAccessMode  string     `json:"ip_access_mode"`
	WhitelistSize int        `json:"whitelist_size"`
	BlacklistSize int        `json:"blacklist_size"`
	Member        *struct {
		Code   string `json:"code"`
		Name   string `json:"name"`
		Status string `json:"status"`
	} `json:"member,omitempty"`
}

type publicKeyUsageLimit struct {
	Limit     float64    `json:"limit"`
	Used      float64    `json:"used"`
	Remaining float64    `json:"remaining"`
	ResetAt   *time.Time `json:"reset_at,omitempty"`
}

type publicKeyUsageBudget struct {
	Quota  publicKeyUsageLimit `json:"quota"`
	Limit5 publicKeyUsageLimit `json:"limit_5h"`
	Limit1 publicKeyUsageLimit `json:"limit_1d"`
	Limit7 publicKeyUsageLimit `json:"limit_7d"`
}

type publicKeyUsageMemberBudget struct {
	PeriodStart  time.Time           `json:"period_start"`
	PeriodEnd    time.Time           `json:"period_end"`
	Timezone     string              `json:"timezone"`
	Monthly      publicKeyUsageLimit `json:"monthly"`
	SettledUSD   float64             `json:"settled_usd"`
	ReservedUSD  float64             `json:"reserved_usd"`
	RequestCount int64               `json:"request_count"`
	InputTokens  int64               `json:"input_tokens"`
	OutputTokens int64               `json:"output_tokens"`
	Limit5       publicKeyUsageLimit `json:"limit_5h"`
	Limit1       publicKeyUsageLimit `json:"limit_1d"`
	Limit7       publicKeyUsageLimit `json:"limit_7d"`
}

type publicKeyUsageAccessGroup struct {
	Name        string   `json:"name"`
	Platform    string   `json:"platform"`
	Status      string   `json:"status"`
	SortOrder   int      `json:"sort_order"`
	RPMLimit    int      `json:"rpm_limit"`
	Models      []string `json:"models"`
	ModelCount  int      `json:"model_count"`
	Description string   `json:"description,omitempty"`
}

type publicKeyUsageStats struct {
	TotalRequests            int64   `json:"total_requests"`
	TotalInputTokens         int64   `json:"total_input_tokens"`
	TotalOutputTokens        int64   `json:"total_output_tokens"`
	TotalCacheCreationTokens int64   `json:"total_cache_creation_tokens"`
	TotalCacheReadTokens     int64   `json:"total_cache_read_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	TotalActualCost          float64 `json:"total_actual_cost"`
	AverageDurationMs        float64 `json:"average_duration_ms"`
}

type publicKeyUsageTrendPoint struct {
	Date                string  `json:"date"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	ActualCost          float64 `json:"actual_cost"`
}

type publicKeyUsageModelStat struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	ActualCost          float64 `json:"actual_cost"`
}

type publicKeyUsageSummaryResponse struct {
	Identity              publicKeyUsageIdentity      `json:"identity"`
	KeyBudget             publicKeyUsageBudget        `json:"key_budget"`
	MemberBudget          *publicKeyUsageMemberBudget `json:"member_budget,omitempty"`
	AccessGroups          []publicKeyUsageAccessGroup `json:"access_groups"`
	Stats                 publicKeyUsageStats         `json:"stats"`
	Trend                 []publicKeyUsageTrendPoint  `json:"trend"`
	Models                []publicKeyUsageModelStat   `json:"models"`
	StartDate             string                      `json:"start_date"`
	EndDate               string                      `json:"end_date"`
	Timezone              string                      `json:"timezone"`
	ErrorRecordsAvailable bool                        `json:"error_records_available"`
}

type publicKeyUsageRecord struct {
	ID                  int64     `json:"id"`
	Kind                string    `json:"kind"`
	CreatedAt           time.Time `json:"created_at"`
	RequestID           string    `json:"request_id,omitempty"`
	Model               string    `json:"model"`
	InboundEndpoint     string    `json:"inbound_endpoint,omitempty"`
	GroupName           string    `json:"group_name,omitempty"`
	StatusCode          int       `json:"status_code"`
	RequestType         string    `json:"request_type,omitempty"`
	Stream              bool      `json:"stream"`
	InputTokens         int       `json:"input_tokens,omitempty"`
	OutputTokens        int       `json:"output_tokens,omitempty"`
	CacheCreationTokens int       `json:"cache_creation_tokens,omitempty"`
	CacheReadTokens     int       `json:"cache_read_tokens,omitempty"`
	TotalTokens         int       `json:"total_tokens,omitempty"`
	ActualCost          float64   `json:"actual_cost,omitempty"`
	DurationMs          *int      `json:"duration_ms,omitempty"`
	FirstTokenMs        *int      `json:"first_token_ms,omitempty"`
	IPAddress           string    `json:"ip_address,omitempty"`
	UserAgent           string    `json:"user_agent,omitempty"`
	Category            string    `json:"category,omitempty"`
	Platform            string    `json:"platform,omitempty"`
	Message             string    `json:"message,omitempty"`
	UpstreamStatusCode  *int      `json:"upstream_status_code,omitempty"`
}

func (h *GatewayHandler) CreatePublicKeyUsageSession(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	if h.apiKeyService == nil {
		response.ErrorFrom(c, service.ErrPublicKeyUsageSessionUnavailable)
		return
	}
	rawKey, ok := parseBearerCredential(c.GetHeader("Authorization"))
	if !ok {
		response.Unauthorized(c, "A valid API Key is required")
		return
	}
	created, err := h.apiKeyService.CreatePublicKeyUsageSession(c.Request.Context(), rawKey)
	if err != nil {
		if errors.Is(err, service.ErrAPIKeyNotFound) {
			response.Unauthorized(c, "A valid API Key is required")
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	setPublicKeyUsageCookie(c, created.Token, int(service.PublicKeyUsageSessionAbsoluteTTL.Seconds()))
	response.Created(c, publicKeyUsageSessionResponse{
		Valid:             true,
		ExpiresAt:         &created.ExpiresAt,
		AbsoluteExpiresAt: &created.Session.AbsoluteExpiresAt,
	})
}

func (h *GatewayHandler) GetPublicKeyUsageSession(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	token, err := c.Cookie(publicKeyUsageSessionCookie)
	if err != nil || strings.TrimSpace(token) == "" {
		response.Success(c, publicKeyUsageSessionResponse{Valid: false})
		return
	}
	session, _, expiresAt, err := h.apiKeyService.ResolvePublicKeyUsageSession(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, service.ErrPublicKeyUsageSessionInvalid) {
			clearPublicKeyUsageCookie(c)
			response.Success(c, publicKeyUsageSessionResponse{Valid: false})
			return
		}
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, publicKeyUsageSessionResponse{
		Valid:             true,
		ExpiresAt:         &expiresAt,
		AbsoluteExpiresAt: &session.AbsoluteExpiresAt,
	})
}

func (h *GatewayHandler) DeletePublicKeyUsageSession(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	token, _ := c.Cookie(publicKeyUsageSessionCookie)
	clearPublicKeyUsageCookie(c)
	if err := h.apiKeyService.DeletePublicKeyUsageSession(c.Request.Context(), token); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *GatewayHandler) GetPublicKeyUsageSummary(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	_, apiKey, _, ok := h.resolvePublicKeyUsageSession(c)
	if !ok {
		return
	}
	start, end, startDate, endDate, timezoneName, ok := parsePublicKeyUsageDateRange(c)
	if !ok {
		return
	}
	if h.usageService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("PUBLIC_KEY_USAGE_UNAVAILABLE", "usage data is temporarily unavailable"))
		return
	}

	status, err := h.apiKeyService.GetPublicStatusByID(c.Request.Context(), apiKey.ID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats, err := h.usageService.GetStatsByAPIKey(c.Request.Context(), apiKey.ID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	trend, err := h.usageService.GetAPIKeyUsageTrendForUser(c.Request.Context(), apiKey.UserID, apiKey.ID, start, end, "day", timezoneName)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	models, err := h.usageService.GetAPIKeyModelStats(c.Request.Context(), apiKey.UserID, apiKey.ID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := publicKeyUsageSummaryResponse{
		Identity:              buildPublicKeyUsageIdentity(apiKey, status),
		KeyBudget:             buildPublicKeyUsageBudget(status),
		AccessGroups:          h.buildPublicKeyUsageAccessGroups(c.Request.Context(), apiKey),
		Stats:                 mapPublicKeyUsageStats(stats),
		Trend:                 mapPublicKeyUsageTrend(trend),
		Models:                mapPublicKeyUsageModels(models),
		StartDate:             startDate,
		EndDate:               endDate,
		Timezone:              timezoneName,
		ErrorRecordsAvailable: h.settingService != nil && h.settingService.IsUserErrorViewAllowed(c.Request.Context()),
	}
	if apiKey.MemberID != nil {
		if h.memberBudgetService == nil {
			response.ErrorFrom(c, infraerrors.ServiceUnavailable("PUBLIC_KEY_MEMBER_BUDGET_UNAVAILABLE", "member budget is temporarily unavailable"))
			return
		}
		memberBudget, err := h.memberBudgetService.GetSummary(c.Request.Context(), *apiKey.MemberID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		result.MemberBudget = mapPublicKeyUsageMemberBudget(memberBudget)
	}
	response.Success(c, result)
}

func (h *GatewayHandler) ListPublicKeyUsageRecords(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	_, apiKey, _, ok := h.resolvePublicKeyUsageSession(c)
	if !ok {
		return
	}
	start, end, _, _, _, ok := parsePublicKeyUsageDateRange(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	kind := strings.ToLower(strings.TrimSpace(c.DefaultQuery("kind", "success")))
	switch kind {
	case "success":
		if h.usageService == nil {
			response.ErrorFrom(c, infraerrors.ServiceUnavailable("PUBLIC_KEY_USAGE_UNAVAILABLE", "usage data is temporarily unavailable"))
			return
		}
		filters := publicKeyUsageLogFilters(apiKey, start, end, c.Query("model"))
		params := pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}
		logs, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		items := make([]publicKeyUsageRecord, 0, len(logs))
		for i := range logs {
			items = append(items, mapPublicKeyUsageLog(&logs[i]))
		}
		response.Paginated(c, items, result.Total, page, pageSize)
	case "error":
		if !h.publicKeyUsageErrorsAllowed(c) {
			return
		}
		filter := publicKeyUsageErrorFilter(apiKey, start, end, c, page, pageSize)
		result, err := h.opsService.ListUserErrorRequests(c.Request.Context(), apiKey.UserID, filter)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		items := make([]publicKeyUsageRecord, 0, len(result.Items))
		for _, item := range result.Items {
			items = append(items, mapPublicKeyUsageError(item))
		}
		response.Paginated(c, items, int64(result.Total), result.Page, result.PageSize)
	default:
		response.BadRequest(c, "Invalid record kind")
	}
}

func (h *GatewayHandler) GetPublicKeyUsageRecordDetail(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	_, apiKey, _, ok := h.resolvePublicKeyUsageSession(c)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid record id")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(c.DefaultQuery("kind", "success")))
	switch kind {
	case "success":
		if h.usageService == nil {
			response.ErrorFrom(c, infraerrors.ServiceUnavailable("PUBLIC_KEY_USAGE_UNAVAILABLE", "usage data is temporarily unavailable"))
			return
		}
		log, err := h.usageService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if log.UserID != apiKey.UserID || log.APIKeyID != apiKey.ID {
			response.NotFound(c, "Usage record not found")
			return
		}
		response.Success(c, mapPublicKeyUsageLog(log))
	case "error":
		if !h.publicKeyUsageErrorsAllowed(c) {
			return
		}
		detail, err := h.opsService.GetUserAPIKeyErrorRequestDetail(c.Request.Context(), apiKey.UserID, apiKey.ID, id)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		item := mapPublicKeyUsageError(&detail.UserErrorRequest)
		item.UpstreamStatusCode = detail.UpstreamStatusCode
		response.Success(c, item)
	default:
		response.BadRequest(c, "Invalid record kind")
	}
}

func (h *GatewayHandler) ExportPublicKeyUsageRecords(c *gin.Context) {
	setPublicKeyUsageNoStore(c)
	_, apiKey, _, ok := h.resolvePublicKeyUsageSession(c)
	if !ok {
		return
	}
	start, end, startDate, endDate, _, ok := parsePublicKeyUsageDateRange(c)
	if !ok {
		return
	}
	kind := strings.ToLower(strings.TrimSpace(c.DefaultQuery("kind", "success")))
	if kind != "success" && kind != "error" {
		response.BadRequest(c, "Invalid record kind")
		return
	}
	if kind == "error" && !h.publicKeyUsageErrorsAllowed(c) {
		return
	}

	items, err := h.collectPublicKeyUsageExportRecords(c, apiKey, kind, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	filename := fmt.Sprintf("key-usage-%s-%s-to-%s.csv", kind, startDate, endDate)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Status(http.StatusOK)
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"time", "type", "request_id", "model", "endpoint", "group", "status", "input_tokens", "output_tokens", "cache_tokens", "total_tokens", "actual_cost", "duration_ms", "category", "message"})
	for _, item := range items {
		_ = writer.Write(publicKeyUsageCSVRow(item))
	}
	writer.Flush()
}

func (h *GatewayHandler) collectPublicKeyUsageExportRecords(c *gin.Context, apiKey *service.APIKey, kind string, start, end time.Time) ([]publicKeyUsageRecord, error) {
	if kind == "success" && h.usageService == nil {
		return nil, infraerrors.ServiceUnavailable("PUBLIC_KEY_USAGE_UNAVAILABLE", "usage data is temporarily unavailable")
	}
	allItems := make([]publicKeyUsageRecord, 0, min(publicKeyUsageExportLimit, 1000))
	for page := 1; len(allItems) < publicKeyUsageExportLimit; page++ {
		var items []publicKeyUsageRecord
		pageSize := 1000
		if kind == "success" {
			logs, _, err := h.usageService.ListWithFilters(c.Request.Context(), pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "created_at", SortOrder: "desc"}, publicKeyUsageLogFilters(apiKey, start, end, c.Query("model")))
			if err != nil {
				return nil, err
			}
			items = make([]publicKeyUsageRecord, 0, len(logs))
			for i := range logs {
				items = append(items, mapPublicKeyUsageLog(&logs[i]))
			}
		} else {
			filter := publicKeyUsageErrorFilter(apiKey, start, end, c, page, pageSize)
			result, err := h.opsService.ListUserErrorRequests(c.Request.Context(), apiKey.UserID, filter)
			if err != nil {
				return nil, err
			}
			items = make([]publicKeyUsageRecord, 0, len(result.Items))
			for _, item := range result.Items {
				items = append(items, mapPublicKeyUsageError(item))
			}
			if result.PageSize > 0 {
				pageSize = result.PageSize
			}
		}
		if len(items) == 0 {
			break
		}
		for _, item := range items {
			if len(allItems) >= publicKeyUsageExportLimit {
				break
			}
			allItems = append(allItems, item)
		}
		if !shouldContinuePublicKeyUsageExport(len(allItems), len(items), pageSize) {
			break
		}
	}
	return allItems, nil
}

func shouldContinuePublicKeyUsageExport(collected, itemCount, pageSize int) bool {
	return collected < publicKeyUsageExportLimit && pageSize > 0 && itemCount >= pageSize
}

func (h *GatewayHandler) resolvePublicKeyUsageSession(c *gin.Context) (*service.PublicKeyUsageSession, *service.APIKey, time.Time, bool) {
	if h.apiKeyService == nil {
		response.ErrorFrom(c, service.ErrPublicKeyUsageSessionUnavailable)
		return nil, nil, time.Time{}, false
	}
	token, err := c.Cookie(publicKeyUsageSessionCookie)
	if err != nil || strings.TrimSpace(token) == "" {
		response.ErrorFrom(c, service.ErrPublicKeyUsageSessionInvalid)
		return nil, nil, time.Time{}, false
	}
	session, apiKey, expiresAt, err := h.apiKeyService.ResolvePublicKeyUsageSession(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, service.ErrPublicKeyUsageSessionInvalid) {
			clearPublicKeyUsageCookie(c)
		}
		response.ErrorFrom(c, err)
		return nil, nil, time.Time{}, false
	}
	return session, apiKey, expiresAt, true
}

func (h *GatewayHandler) publicKeyUsageErrorsAllowed(c *gin.Context) bool {
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return false
	}
	if h.opsService == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("PUBLIC_KEY_ERROR_RECORDS_UNAVAILABLE", "error records are temporarily unavailable"))
		return false
	}
	return true
}

func parseBearerCredential(value string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return parts[1], true
}

func setPublicKeyUsageNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func publicKeyUsageCookieSecure(c *gin.Context) bool {
	if c.Request != nil && c.Request.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https")
}

func setPublicKeyUsageCookie(c *gin.Context, value string, maxAge int) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     publicKeyUsageSessionCookie,
		Value:    value,
		Path:     publicKeyUsageSessionPath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   publicKeyUsageCookieSecure(c),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearPublicKeyUsageCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     publicKeyUsageSessionCookie,
		Value:    "",
		Path:     publicKeyUsageSessionPath,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
		HttpOnly: true,
		Secure:   publicKeyUsageCookieSecure(c),
		SameSite: http.SameSiteStrictMode,
	})
}

func parsePublicKeyUsageDateRange(c *gin.Context) (time.Time, time.Time, string, string, string, bool) {
	timezoneName := strings.TrimSpace(c.DefaultQuery("timezone", timezone.Name()))
	if _, err := time.LoadLocation(timezoneName); err != nil {
		response.BadRequest(c, "Invalid timezone")
		return time.Time{}, time.Time{}, "", "", "", false
	}
	now := timezone.NowInUserLocation(timezoneName)
	start := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -29), timezoneName)
	endInclusive := timezone.StartOfDayInUserLocation(now, timezoneName)
	if raw := strings.TrimSpace(c.Query("start_date")); raw != "" {
		parsed, hasTime, err := timezone.ParseUserDateOrDateTime(raw, timezoneName)
		if err != nil || hasTime {
			response.BadRequest(c, "Invalid start_date")
			return time.Time{}, time.Time{}, "", "", "", false
		}
		start = parsed
	}
	if raw := strings.TrimSpace(c.Query("end_date")); raw != "" {
		parsed, hasTime, err := timezone.ParseUserDateOrDateTime(raw, timezoneName)
		if err != nil || hasTime {
			response.BadRequest(c, "Invalid end_date")
			return time.Time{}, time.Time{}, "", "", "", false
		}
		endInclusive = parsed
	}
	if endInclusive.Before(start) || !endInclusive.Before(start.AddDate(0, 0, publicKeyUsageMaxRangeDays)) {
		response.BadRequest(c, "Date range must be between 1 and 90 days")
		return time.Time{}, time.Time{}, "", "", "", false
	}
	end := endInclusive.AddDate(0, 0, 1)
	return start, end, start.Format("2006-01-02"), endInclusive.Format("2006-01-02"), timezoneName, true
}

func buildPublicKeyUsageIdentity(apiKey *service.APIKey, status *service.APIKeyPublicStatus) publicKeyUsageIdentity {
	identity := publicKeyUsageIdentity{
		Name:          status.Name,
		KeyPrefix:     maskPublicKeyUsageKey(apiKey.Key),
		Status:        status.Status,
		Active:        status.IsActive,
		CreatedAt:     status.CreatedAt,
		LastUsedAt:    status.LastUsedAt,
		ExpiresAt:     status.ExpiresAt,
		IPAccessMode:  publicKeyUsageIPAccessMode(apiKey),
		WhitelistSize: len(apiKey.IPWhitelist),
		BlacklistSize: len(apiKey.IPBlacklist),
	}
	if apiKey.Member != nil {
		identity.Member = &struct {
			Code   string `json:"code"`
			Name   string `json:"name"`
			Status string `json:"status"`
		}{Code: apiKey.Member.MemberCode, Name: apiKey.Member.Name, Status: apiKey.Member.Status}
	}
	return identity
}

func publicKeyUsageIPAccessMode(apiKey *service.APIKey) string {
	if len(apiKey.IPWhitelist) > 0 {
		return "whitelist"
	}
	if len(apiKey.IPBlacklist) > 0 {
		return "blacklist"
	}
	return "unrestricted"
}

func maskPublicKeyUsageKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:min(8, len(key))] + "••••"
}

func buildPublicKeyUsageBudget(status *service.APIKeyPublicStatus) publicKeyUsageBudget {
	return publicKeyUsageBudget{
		Quota:  newPublicKeyUsageLimit(status.Quota, status.QuotaUsed, nil),
		Limit5: newPublicKeyUsageLimit(status.RateLimit5h, status.Usage5h, status.Reset5hAt),
		Limit1: newPublicKeyUsageLimit(status.RateLimit1d, status.Usage1d, status.Reset1dAt),
		Limit7: newPublicKeyUsageLimit(status.RateLimit7d, status.Usage7d, status.Reset7dAt),
	}
}

func newPublicKeyUsageLimit(limit, used float64, resetAt *time.Time) publicKeyUsageLimit {
	remaining := -1.0
	if limit > 0 {
		remaining = max(0, limit-used)
	}
	return publicKeyUsageLimit{Limit: limit, Used: used, Remaining: remaining, ResetAt: resetAt}
}

func mapPublicKeyUsageMemberBudget(summary *service.EnterpriseMemberBudgetSummary) *publicKeyUsageMemberBudget {
	if summary == nil {
		return nil
	}
	return &publicKeyUsageMemberBudget{
		PeriodStart:  summary.PeriodStart,
		PeriodEnd:    summary.PeriodEnd,
		Timezone:     summary.Timezone,
		Monthly:      publicKeyUsageLimit{Limit: summary.LimitUSD, Used: summary.UsedUSD + summary.ReservedUSD, Remaining: summary.RemainingUSD},
		SettledUSD:   summary.UsedUSD,
		ReservedUSD:  summary.ReservedUSD,
		RequestCount: summary.RequestCount,
		InputTokens:  summary.InputTokens,
		OutputTokens: summary.OutputTokens,
		Limit5:       newPublicKeyUsageLimit(summary.RateLimit5h, summary.Usage5h, summary.Reset5hAt),
		Limit1:       newPublicKeyUsageLimit(summary.RateLimit1d, summary.Usage1d, summary.Reset1dAt),
		Limit7:       newPublicKeyUsageLimit(summary.RateLimit7d, summary.Usage7d, summary.Reset7dAt),
	}
}

func (h *GatewayHandler) buildPublicKeyUsageAccessGroups(ctx context.Context, apiKey *service.APIKey) []publicKeyUsageAccessGroup {
	groups := make([]service.Group, 0)
	if apiKey.Member != nil {
		groups = append(groups, apiKey.Member.Groups...)
	} else if apiKey.Group != nil {
		groups = append(groups, *apiKey.Group)
	}
	result := make([]publicKeyUsageAccessGroup, 0, len(groups))
	for i := range groups {
		group := &groups[i]
		models := h.publicKeyUsageModelsForGroup(ctx, group)
		rpmLimit := group.RPMLimit
		if apiKey.User != nil && apiKey.User.RPMLimit > 0 && (rpmLimit == 0 || apiKey.User.RPMLimit < rpmLimit) {
			rpmLimit = apiKey.User.RPMLimit
		}
		result = append(result, publicKeyUsageAccessGroup{
			Name: group.Name, Platform: group.Platform, Status: group.Status, SortOrder: group.SortOrder,
			RPMLimit: rpmLimit, Models: models, ModelCount: len(models), Description: group.Description,
		})
	}
	return result
}

func (h *GatewayHandler) publicKeyUsageModelsForGroup(ctx context.Context, group *service.Group) []string {
	if group == nil {
		return []string{}
	}
	groupID := group.ID
	available := []string(nil)
	if h.gatewayService != nil {
		available = h.gatewayService.GetAvailableModels(ctx, &groupID, group.Platform)
	}
	fallback := defaultModelIDsForPlatform(group.Platform)
	if group.CustomModelsListEnabled() {
		available = filterModelsByCustomList(customModelsListSource(group.Platform, available, fallback), fallback, group.ModelsListConfig.Models)
	} else if len(available) == 0 {
		available = fallback
	}
	seen := make(map[string]struct{}, len(available))
	models := make([]string, 0, len(available))
	for _, model := range available {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, model)
	}
	return models
}

func mapPublicKeyUsageStats(stats *service.UsageStats) publicKeyUsageStats {
	if stats == nil {
		return publicKeyUsageStats{}
	}
	return publicKeyUsageStats{
		TotalRequests: stats.TotalRequests, TotalInputTokens: stats.TotalInputTokens,
		TotalOutputTokens: stats.TotalOutputTokens, TotalCacheCreationTokens: stats.TotalCacheCreationTokens,
		TotalCacheReadTokens: stats.TotalCacheReadTokens, TotalTokens: stats.TotalTokens,
		TotalActualCost: stats.TotalActualCost, AverageDurationMs: stats.AverageDurationMs,
	}
}

func mapPublicKeyUsageTrend(items []usagestats.TrendDataPoint) []publicKeyUsageTrendPoint {
	result := make([]publicKeyUsageTrendPoint, 0, len(items))
	for _, item := range items {
		result = append(result, publicKeyUsageTrendPoint{
			Date: item.Date, Requests: item.Requests, InputTokens: item.InputTokens,
			OutputTokens: item.OutputTokens, CacheCreationTokens: item.CacheCreationTokens,
			CacheReadTokens: item.CacheReadTokens, TotalTokens: item.TotalTokens, ActualCost: item.ActualCost,
		})
	}
	return result
}

func mapPublicKeyUsageModels(items []usagestats.ModelStat) []publicKeyUsageModelStat {
	result := make([]publicKeyUsageModelStat, 0, len(items))
	for _, item := range items {
		result = append(result, publicKeyUsageModelStat{
			Model: item.Model, Requests: item.Requests, InputTokens: item.InputTokens,
			OutputTokens: item.OutputTokens, CacheCreationTokens: item.CacheCreationTokens,
			CacheReadTokens: item.CacheReadTokens, TotalTokens: item.TotalTokens, ActualCost: item.ActualCost,
		})
	}
	return result
}

func publicKeyUsageLogFilters(apiKey *service.APIKey, start, end time.Time, model string) usagestats.UsageLogFilters {
	return usagestats.UsageLogFilters{
		UserID: apiKey.UserID, APIKeyID: apiKey.ID, Model: strings.TrimSpace(model),
		ModelFilterSource: usagestats.ModelSourceRequested, StartTime: &start, EndTime: &end, ExactTotal: true,
	}
}

func publicKeyUsageErrorFilter(apiKey *service.APIKey, start, end time.Time, c *gin.Context, page, pageSize int) *service.OpsErrorLogFilter {
	apiKeyID := apiKey.ID
	filter := &service.OpsErrorLogFilter{
		StartTime: &start, EndTime: &end, APIKeyID: &apiKeyID,
		Model: strings.TrimSpace(c.Query("model")), Page: page, PageSize: pageSize,
	}
	if raw := strings.TrimSpace(c.Query("status_code")); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value >= 400 && value <= 599 {
			filter.StatusCodes = []int{value}
		}
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" {
		filter.ErrorPhasesAny, filter.ErrorTypesAny = service.CategoryToFilter(category)
	}
	filter.SetSort("created_at", "desc")
	return filter
}

func mapPublicKeyUsageLog(log *service.UsageLog) publicKeyUsageRecord {
	model := strings.TrimSpace(log.RequestedModel)
	if model == "" {
		model = log.Model
	}
	endpoint := ""
	if log.InboundEndpoint != nil {
		endpoint = *log.InboundEndpoint
	}
	groupName := ""
	if log.Group != nil {
		groupName = log.Group.Name
	}
	ipAddress := ""
	if log.IPAddress != nil {
		ipAddress = *log.IPAddress
	}
	userAgent := ""
	if log.UserAgent != nil {
		userAgent = *log.UserAgent
	}
	return publicKeyUsageRecord{
		ID: log.ID, Kind: "success", CreatedAt: log.CreatedAt, RequestID: log.RequestID,
		Model: model, InboundEndpoint: endpoint, GroupName: groupName, StatusCode: http.StatusOK,
		RequestType: log.RequestType.String(), Stream: log.Stream, InputTokens: log.InputTokens,
		OutputTokens: log.OutputTokens, CacheCreationTokens: log.CacheCreationTokens,
		CacheReadTokens: log.CacheReadTokens, TotalTokens: log.InputTokens + log.OutputTokens + log.CacheCreationTokens + log.CacheReadTokens,
		ActualCost: log.ActualCost, DurationMs: log.DurationMs, FirstTokenMs: log.FirstTokenMs,
		IPAddress: maskPublicKeyUsageIP(ipAddress), UserAgent: userAgent,
	}
}

func mapPublicKeyUsageError(item *service.UserErrorRequest) publicKeyUsageRecord {
	if item == nil {
		return publicKeyUsageRecord{Kind: "error"}
	}
	requestType := ""
	if item.RequestType != nil {
		requestType = service.RequestTypeFromInt16(*item.RequestType).String()
	}
	return publicKeyUsageRecord{
		ID: item.ID, Kind: "error", CreatedAt: item.CreatedAt, RequestID: item.RequestID, Model: item.Model,
		InboundEndpoint: item.InboundEndpoint, GroupName: item.GroupName, StatusCode: item.StatusCode,
		RequestType: requestType, Stream: item.Stream, IPAddress: maskPublicKeyUsageIP(item.ClientIP), UserAgent: item.UserAgent,
		Category: item.Category, Platform: item.Platform, Message: item.Message,
	}
}

func maskPublicKeyUsageIP(value string) string {
	parsed := net.ParseIP(strings.TrimSpace(value))
	if parsed == nil {
		return ""
	}
	if ipv4 := parsed.To4(); ipv4 != nil {
		return fmt.Sprintf("%d.%d.%d.*", ipv4[0], ipv4[1], ipv4[2])
	}
	ipv6 := parsed.To16()
	if ipv6 == nil {
		return ""
	}
	return fmt.Sprintf("%x:%x:%x:*", uint16(ipv6[0])<<8|uint16(ipv6[1]), uint16(ipv6[2])<<8|uint16(ipv6[3]), uint16(ipv6[4])<<8|uint16(ipv6[5]))
}

func publicKeyUsageCSVRow(item publicKeyUsageRecord) []string {
	duration := ""
	if item.DurationMs != nil {
		duration = strconv.Itoa(*item.DurationMs)
	}
	values := []string{
		item.CreatedAt.Format(time.RFC3339), item.Kind, item.RequestID, item.Model, item.InboundEndpoint,
		item.GroupName, strconv.Itoa(item.StatusCode), strconv.Itoa(item.InputTokens), strconv.Itoa(item.OutputTokens),
		strconv.Itoa(item.CacheCreationTokens + item.CacheReadTokens), strconv.Itoa(item.TotalTokens),
		strconv.FormatFloat(item.ActualCost, 'f', 8, 64), duration, item.Category, item.Message,
	}
	for i := range values {
		values[i] = protectCSVFormula(values[i])
	}
	return values
}

func protectCSVFormula(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}
