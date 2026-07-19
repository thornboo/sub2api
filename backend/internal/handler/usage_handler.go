package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type userUsageFilters struct {
	Filters   usagestats.UsageLogFilters
	StartTime time.Time
	EndTime   time.Time
}

type userGroupStat struct {
	GroupID     int64   `json:"group_id"`
	GroupName   string  `json:"group_name"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
	ActualCost  float64 `json:"actual_cost"`
}

func parseUsageMemberFilters(c *gin.Context) (*int64, string, bool, string) {
	rawMemberID, hasMemberID := c.GetQuery("member_id")
	rawMemberScope, hasMemberScope := c.GetQuery("member_scope")
	if !hasMemberID && !hasMemberScope {
		return nil, "", false, ""
	}

	var memberID *int64
	if raw := strings.TrimSpace(rawMemberID); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return nil, "", true, "Invalid member_id"
		}
		memberID = &parsed
	} else if hasMemberID {
		return nil, "", true, "Invalid member_id"
	}

	memberScope := strings.ToLower(strings.TrimSpace(rawMemberScope))
	if memberScope == "" {
		memberScope = usagestats.MemberScopeAll
	}
	if !usagestats.IsValidMemberScope(memberScope) {
		return nil, "", true, "Invalid member_scope, allowed values are all, assigned, unassigned"
	}
	if memberID != nil && memberScope != usagestats.MemberScopeAll {
		return nil, "", true, "member_id cannot be combined with assigned or unassigned member_scope"
	}
	return memberID, memberScope, true, ""
}

func apiKeyMatchesMemberFilter(apiKey *service.APIKey, memberID *int64, memberScope string) bool {
	if apiKey == nil {
		return true
	}
	if memberID != nil {
		return apiKey.MemberID != nil && *apiKey.MemberID == *memberID
	}
	switch memberScope {
	case usagestats.MemberScopeAssigned:
		return apiKey.MemberID != nil
	case usagestats.MemberScopeUnassigned:
		return apiKey.MemberID == nil
	default:
		return true
	}
}

// UsageHandler handles usage-related requests
type UsageHandler struct {
	usageService   *service.UsageService
	apiKeyService  *service.APIKeyService
	opsService     *service.OpsService
	settingService *service.SettingService
}

// NewUsageHandler creates a new UsageHandler
func NewUsageHandler(
	usageService *service.UsageService,
	apiKeyService *service.APIKeyService,
	opsService *service.OpsService,
	settingService *service.SettingService,
) *UsageHandler {
	return &UsageHandler{
		usageService:   usageService,
		apiKeyService:  apiKeyService,
		opsService:     opsService,
		settingService: settingService,
	}
}

func (h *UsageHandler) parseUserUsageFilters(c *gin.Context, requireRange bool) (*userUsageFilters, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return nil, false
	}

	var apiKeyID int64
	var selectedAPIKey *service.APIKey
	if apiKeyIDStr := strings.TrimSpace(c.Query("api_key_id")); apiKeyIDStr != "" {
		id, err := strconv.ParseInt(apiKeyIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid api_key_id")
			return nil, false
		}
		if h.apiKeyService == nil {
			response.InternalError(c, "API key service not available")
			return nil, false
		}
		apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			return nil, false
		}
		if apiKey.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to access this API key's usage records")
			return nil, false
		}
		apiKeyID = id
		selectedAPIKey = apiKey
	}

	memberID, memberScope, memberFilterSet, memberErrMessage := parseUsageMemberFilters(c)
	if memberErrMessage != "" {
		response.BadRequest(c, memberErrMessage)
		return nil, false
	}
	if memberFilterSet {
		if err := h.usageService.ValidateEnterpriseUsageOwner(c.Request.Context(), subject.UserID); err != nil {
			response.ErrorFrom(c, err)
			return nil, false
		}
		if memberID != nil {
			if err := h.usageService.ValidateOwnerUsageMember(c.Request.Context(), subject.UserID, *memberID); err != nil {
				response.ErrorFrom(c, err)
				return nil, false
			}
		}
		if !apiKeyMatchesMemberFilter(selectedAPIKey, memberID, memberScope) {
			response.BadRequest(c, "api_key_id does not belong to the selected member scope")
			return nil, false
		}
	}

	var groupID int64
	if groupIDStr := strings.TrimSpace(c.Query("group_id")); groupIDStr != "" {
		id, err := strconv.ParseInt(groupIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "Invalid group_id")
			return nil, false
		}
		groupID = id
	}

	var requestType *int16
	var stream *bool
	if requestTypeStr := strings.TrimSpace(c.Query("request_type")); requestTypeStr != "" {
		parsed, err := service.ParseUsageRequestType(requestTypeStr)
		if err != nil {
			response.BadRequest(c, err.Error())
			return nil, false
		}
		value := int16(parsed)
		requestType = &value
	} else if streamStr := strings.TrimSpace(c.Query("stream")); streamStr != "" {
		val, err := strconv.ParseBool(streamStr)
		if err != nil {
			response.BadRequest(c, "Invalid stream value, use true or false")
			return nil, false
		}
		stream = &val
	}

	var billingType *int8
	if billingTypeStr := strings.TrimSpace(c.Query("billing_type")); billingTypeStr != "" {
		val, err := strconv.ParseInt(billingTypeStr, 10, 8)
		if err != nil {
			response.BadRequest(c, "Invalid billing_type")
			return nil, false
		}
		bt := int8(val)
		billingType = &bt
	}

	billingMode := strings.TrimSpace(c.Query("billing_mode"))
	if billingMode != "" && !service.BillingMode(billingMode).IsValidUsageFilter() {
		response.BadRequest(c, "Invalid billing_mode")
		return nil, false
	}

	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	var startTime, endTime time.Time
	var startPtr, endPtr *time.Time
	startStr := strings.TrimSpace(c.Query("start_time"))
	if startStr == "" {
		startStr = strings.TrimSpace(c.Query("start_date"))
	}

	if startStr != "" {
		t, _, err := timezone.ParseUserDateOrDateTime(startStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date/start_time format")
			return nil, false
		}
		startTime = t
		startPtr = &startTime
	}

	endStr := strings.TrimSpace(c.Query("end_time"))
	if endStr == "" {
		endStr = strings.TrimSpace(c.Query("end_date"))
	}
	if endStr != "" {
		t, hasTime, err := timezone.ParseUserDateOrDateTime(endStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date/end_time format")
			return nil, false
		}
		if !hasTime {
			// Date-only: half-open range [start, end+1d) to include the whole end date.
			t = t.AddDate(0, 0, 1)
		}
		endTime = t
		endPtr = &endTime
	}

	if requireRange {
		if startPtr == nil {
			switch c.DefaultQuery("period", "") {
			case "today":
				startTime = timezone.StartOfDayInUserLocation(now, userTZ)
			case "week":
				startTime = now.AddDate(0, 0, -7)
			case "month":
				startTime = now.AddDate(0, -1, 0)
			default:
				startTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -7), userTZ)
			}
			startPtr = &startTime
		}
		if endPtr == nil {
			if strings.TrimSpace(c.Query("period")) != "" {
				endTime = now
			} else {
				endTime = timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
			}
			endPtr = &endTime
		}
	}

	return &userUsageFilters{
		Filters: usagestats.UsageLogFilters{
			UserID:            subject.UserID,
			APIKeyID:          apiKeyID,
			GroupID:           groupID,
			MemberID:          memberID,
			MemberScope:       memberScope,
			Model:             strings.TrimSpace(c.Query("model")),
			ModelFilterSource: usagestats.ModelSourceRequested,
			RequestType:       requestType,
			Stream:            stream,
			BillingType:       billingType,
			BillingMode:       billingMode,
			StartTime:         startPtr,
			EndTime:           endPtr,
		},
		StartTime: derefTime(startPtr),
		EndTime:   derefTime(endPtr),
	}, true
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

// List handles listing usage records with pagination
// GET /api/v1/usage
func (h *UsageHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	parsed, ok := h.parseUserUsageFilters(c, false)
	if !ok {
		return
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "created_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}

	records, result, err := h.usageService.ListWithFilters(c.Request.Context(), params, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UsageLog, 0, len(records))
	for i := range records {
		out = append(out, *dto.UsageLogFromService(&records[i]))
	}
	response.Paginated(c, out, result.Total, page, pageSize)
}

// ListErrors handles listing the current user's failed requests (redacted).
// GET /api/v1/usage/errors
func (h *UsageHandler) ListErrors(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Visibility switch (fail-closed). Defense-in-depth: frontend also hides the tab.
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}

	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}

	filter := &service.OpsErrorLogFilter{Page: page, PageSize: pageSize}

	// Date range (half-open [start, end)). start_time/end_time (datetime) take precedence
	// over start_date/end_date; a datetime bound skips the whole-day end bump.
	userTZ := c.Query("timezone")
	startStr := c.Query("start_time")
	if startStr == "" {
		startStr = c.Query("start_date")
	}
	if startStr != "" {
		t, _, err := timezone.ParseUserDateOrDateTime(startStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid start_date/start_time format")
			return
		}
		filter.StartTime = &t
	}
	endStr := c.Query("end_time")
	if endStr == "" {
		endStr = c.Query("end_date")
	}
	if endStr != "" {
		t, hasTime, err := timezone.ParseUserDateOrDateTime(endStr, userTZ)
		if err != nil {
			response.BadRequest(c, "Invalid end_date/end_time format")
			return
		}
		if !hasTime {
			t = t.AddDate(0, 0, 1)
		}
		filter.EndTime = &t
	}

	filter.Model = strings.TrimSpace(c.Query("model"))

	var selectedAPIKey *service.APIKey
	if k := strings.TrimSpace(c.Query("api_key_id")); k != "" {
		n, err := strconv.ParseInt(k, 10, 64)
		if err != nil || n < 0 {
			response.BadRequest(c, "Invalid api_key_id")
			return
		}
		if n > 0 {
			filter.APIKeyID = &n
		}
	}

	memberID, memberScope, memberFilterSet, memberErrMessage := parseUsageMemberFilters(c)
	if memberErrMessage != "" {
		response.BadRequest(c, memberErrMessage)
		return
	}
	if memberFilterSet {
		if err := h.usageService.ValidateEnterpriseUsageOwner(c.Request.Context(), subject.UserID); err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if memberID != nil {
			if err := h.usageService.ValidateOwnerUsageMember(c.Request.Context(), subject.UserID, *memberID); err != nil {
				response.ErrorFrom(c, err)
				return
			}
		}
		if filter.APIKeyID != nil {
			if h.apiKeyService == nil {
				response.InternalError(c, "API key service not available")
				return
			}
			apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), *filter.APIKeyID)
			if err != nil {
				response.ErrorFrom(c, err)
				return
			}
			if apiKey.UserID != subject.UserID {
				response.Forbidden(c, "Not authorized to access this API key's error records")
				return
			}
			selectedAPIKey = apiKey
		}
		if !apiKeyMatchesMemberFilter(selectedAPIKey, memberID, memberScope) {
			response.BadRequest(c, "api_key_id does not belong to the selected member scope")
			return
		}
		filter.MemberID = memberID
		filter.MemberScope = memberScope
	}

	if sc := strings.TrimSpace(c.Query("status_code")); sc != "" {
		n, err := strconv.Atoi(sc)
		if err != nil || n < 0 {
			response.BadRequest(c, "Invalid status_code")
			return
		}
		filter.StatusCodes = []int{n}
	}

	if cat := strings.TrimSpace(c.Query("category")); cat != "" {
		phases, types := service.CategoryToFilter(cat)
		filter.ErrorPhasesAny = phases
		filter.ErrorTypesAny = types
	}

	// 排序对齐用量明细:列白名单与方向归一在 repo 层,非法值回退 created_at DESC。
	filter.SetSort(c.Query("sort_by"), c.Query("sort_order"))

	result, err := h.opsService.ListUserErrorRequests(c.Request.Context(), subject.UserID, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Items, int64(result.Total), result.Page, result.PageSize)
}

// GetErrorDetail handles fetching one of the current user's failed-request details (redacted).
// GET /api/v1/usage/errors/:id
func (h *UsageHandler) GetErrorDetail(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.settingService == nil || !h.settingService.IsUserErrorViewAllowed(c.Request.Context()) {
		response.Forbidden(c, "Error requests view is disabled")
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid id")
		return
	}
	detail, err := h.opsService.GetUserErrorRequestDetail(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// GetByID handles getting a single usage record
// GET /api/v1/usage/:id
func (h *UsageHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	usageID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid usage ID")
		return
	}

	record, err := h.usageService.GetByIDForOwner(c.Request.Context(), usageID, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Repository 已按 owner 和成员墓碑可见性过滤；保留二次校验防止错误实现泄露记录存在性。
	if record.UserID != subject.UserID {
		response.NotFound(c, "Usage record not found")
		return
	}

	response.Success(c, dto.UsageLogFromService(record))
}

// Stats handles getting usage statistics
// GET /api/v1/usage/stats
func (h *UsageHandler) Stats(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}

	stats, err := h.usageService.GetStatsWithFilters(c.Request.Context(), parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	stats.TotalAccountCost = nil
	stats.UpstreamEndpoints = nil
	stats.EndpointPaths = nil

	response.Success(c, stats)
}

const (
	defaultAPIKeyDailyUsageDays = 30
	maxAPIKeyDailyUsageDays     = 90
)

const (
	defaultAPIKeyUsageTrendGranularity = "day"
	apiKeyUsageTrendDateLayout         = "2006-01-02"
)

var apiKeyUsageTrendMaxDays = map[string]int{
	"hour":  31,
	"day":   366,
	"week":  7 * 104,
	"month": 31 * 60,
}

var ownerAPIKeyAnalyticsMaxDays = map[string]int{
	"hour":  31,
	"day":   180,
	"week":  7 * 104,
	"month": 31 * 60,
}

func parseAPIKeyDailyUsageDays(raw string) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return defaultAPIKeyDailyUsageDays, true
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days <= 0 || days > maxAPIKeyDailyUsageDays {
		return 0, false
	}
	return days, true
}

func apiKeyDailyUsageRange(days int, userTZ string) (time.Time, time.Time) {
	now := timezone.NowInUserLocation(userTZ)
	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -(days-1)), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)
	return startTime, endTime
}

func parseAPIKeyUsageTrendGranularity(raw string) (string, bool) {
	granularity := strings.TrimSpace(raw)
	if granularity == "" {
		return defaultAPIKeyUsageTrendGranularity, true
	}
	switch granularity {
	case "hour", "day", "week", "month":
		return granularity, true
	default:
		return "", false
	}
}

func apiKeyUsageTrendLocation(userTZ string) *time.Location {
	if trimmed := strings.TrimSpace(userTZ); trimmed != "" {
		if loc, err := time.LoadLocation(trimmed); err == nil {
			return loc
		}
	}
	return timezone.Location()
}

func apiKeyUsageTrendTimezoneName(userTZ string) string {
	if trimmed := strings.TrimSpace(userTZ); trimmed != "" {
		if _, err := time.LoadLocation(trimmed); err == nil {
			return trimmed
		}
	}
	name := timezone.Name()
	if name == "Local" {
		return "UTC"
	}
	return name
}

func apiKeyUsageTrendStartOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func apiKeyUsageTrendStartOfWeek(t time.Time, loc *time.Location) time.Time {
	start := apiKeyUsageTrendStartOfDay(t, loc)
	weekday := int(start.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return start.AddDate(0, 0, -weekday+1)
}

func apiKeyUsageTrendStartOfMonth(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, loc)
}

func defaultAPIKeyUsageTrendRange(granularity string, loc *time.Location) (time.Time, time.Time) {
	now := time.Now().In(loc)
	switch granularity {
	case "hour":
		start := apiKeyUsageTrendStartOfDay(now, loc)
		return start, start.AddDate(0, 0, 1)
	case "week":
		currentWeekStart := apiKeyUsageTrendStartOfWeek(now, loc)
		return currentWeekStart.AddDate(0, 0, -7*11), currentWeekStart.AddDate(0, 0, 7)
	case "month":
		currentMonthStart := apiKeyUsageTrendStartOfMonth(now, loc)
		return currentMonthStart.AddDate(0, -11, 0), currentMonthStart.AddDate(0, 1, 0)
	default:
		end := apiKeyUsageTrendStartOfDay(now.AddDate(0, 0, 1), loc)
		return end.AddDate(0, 0, -defaultAPIKeyDailyUsageDays), end
	}
}

func parseAPIKeyUsageTrendRange(startDate, endDate, granularity string, loc *time.Location) (time.Time, time.Time, string) {
	startTime, endTime := defaultAPIKeyUsageTrendRange(granularity, loc)
	if strings.TrimSpace(startDate) != "" {
		parsed, err := time.ParseInLocation(apiKeyUsageTrendDateLayout, startDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "Invalid start_date format, use YYYY-MM-DD"
		}
		startTime = parsed
	}
	if strings.TrimSpace(endDate) != "" {
		parsed, err := time.ParseInLocation(apiKeyUsageTrendDateLayout, endDate, loc)
		if err != nil {
			return time.Time{}, time.Time{}, "Invalid end_date format, use YYYY-MM-DD"
		}
		endTime = parsed.AddDate(0, 0, 1)
	}
	if !startTime.Before(endTime) {
		return time.Time{}, time.Time{}, "Invalid date range"
	}
	if !apiKeyUsageTrendRangeAllowed(startTime, endTime, granularity) {
		return time.Time{}, time.Time{}, "Date range exceeds maximum allowed span"
	}
	return startTime, endTime, ""
}

func apiKeyUsageTrendRangeAllowed(startTime, endTime time.Time, granularity string) bool {
	if granularity == "month" {
		return !endTime.After(startTime.AddDate(0, 60, 0))
	}
	maxDays, ok := apiKeyUsageTrendMaxDays[granularity]
	if !ok {
		return false
	}
	return !endTime.After(startTime.AddDate(0, 0, maxDays))
}

func ownerAPIKeyAnalyticsRangeAllowed(startTime, endTime time.Time, granularity string) bool {
	if granularity == "month" {
		return !endTime.After(startTime.AddDate(0, 60, 0))
	}
	maxDays, ok := ownerAPIKeyAnalyticsMaxDays[granularity]
	if !ok {
		return false
	}
	return !endTime.After(startTime.AddDate(0, 0, maxDays))
}

func parseOwnerAPIKeyAnalyticsRange(startDate, endDate, startTimeStr, endTimeStr, granularity, userTZ string, loc *time.Location) (time.Time, time.Time, string) {
	startTime, endTime := defaultAPIKeyUsageTrendRange(granularity, loc)

	// start_time/end_time（datetime）优先于 start_date/end_date（日期）；datetime 口径不做 +1 天补偿。
	startStr := strings.TrimSpace(startTimeStr)
	if startStr == "" {
		startStr = strings.TrimSpace(startDate)
	}
	if startStr != "" {
		parsed, _, err := timezone.ParseUserDateOrDateTime(startStr, userTZ)
		if err != nil {
			return time.Time{}, time.Time{}, "Invalid start_date/start_time format"
		}
		startTime = parsed
	}

	endStr := strings.TrimSpace(endTimeStr)
	if endStr == "" {
		endStr = strings.TrimSpace(endDate)
	}
	if endStr != "" {
		parsed, hasTime, err := timezone.ParseUserDateOrDateTime(endStr, userTZ)
		if err != nil {
			return time.Time{}, time.Time{}, "Invalid end_date/end_time format"
		}
		if hasTime {
			endTime = parsed
		} else {
			endTime = parsed.AddDate(0, 0, 1)
		}
	}

	if !startTime.Before(endTime) {
		return time.Time{}, time.Time{}, "Invalid date range"
	}
	if !ownerAPIKeyAnalyticsRangeAllowed(startTime, endTime, granularity) {
		return time.Time{}, time.Time{}, "Date range exceeds maximum allowed span"
	}
	return startTime, endTime, ""
}

func parseOwnerAnalyticsTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		tag := strings.ToLower(strings.TrimSpace(part))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func parseOwnerAnalyticsStatus(raw string) (string, bool) {
	status := strings.TrimSpace(raw)
	if status == "" {
		return "", true
	}
	switch status {
	case service.StatusAPIKeyActive, service.StatusAPIKeyDisabled, service.StatusAPIKeyQuotaExhausted, service.StatusAPIKeyExpired:
		return status, true
	case "inactive":
		return service.StatusAPIKeyDisabled, true
	default:
		return "", false
	}
}

func parseOwnerAnalyticsLimit(raw string) (int, bool) {
	if strings.TrimSpace(raw) == "" {
		return service.DefaultOwnerAPIKeyAnalyticsLimit, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > service.MaxOwnerAPIKeyAnalyticsLimit {
		return 0, false
	}
	return limit, true
}

func parseOwnerAPIKeyAnalyticsFilters(c *gin.Context, userID int64) (service.OwnerAPIKeyAnalyticsFilters, *time.Location, string) {
	granularity, ok := parseAPIKeyUsageTrendGranularity(c.DefaultQuery("granularity", ""))
	if !ok {
		return service.OwnerAPIKeyAnalyticsFilters{}, nil, "Invalid granularity, allowed values are hour, day, week, month"
	}
	userTZ := c.Query("timezone")
	loc := apiKeyUsageTrendLocation(userTZ)
	timezoneName := apiKeyUsageTrendTimezoneName(userTZ)
	startTime, endTime, errMessage := parseOwnerAPIKeyAnalyticsRange(c.Query("start_date"), c.Query("end_date"), c.Query("start_time"), c.Query("end_time"), granularity, userTZ, loc)
	if errMessage != "" {
		return service.OwnerAPIKeyAnalyticsFilters{}, nil, errMessage
	}

	var apiKeyID *int64
	if rawAPIKeyID := strings.TrimSpace(c.Query("api_key_id")); rawAPIKeyID != "" {
		parsed, err := strconv.ParseInt(rawAPIKeyID, 10, 64)
		if err != nil || parsed <= 0 {
			return service.OwnerAPIKeyAnalyticsFilters{}, nil, "Invalid api_key_id"
		}
		apiKeyID = &parsed
	}

	memberID, memberScope, memberFilterSet, memberErrMessage := parseUsageMemberFilters(c)
	if memberErrMessage != "" {
		return service.OwnerAPIKeyAnalyticsFilters{}, nil, memberErrMessage
	}

	var groupID *int64
	if rawGroupID := strings.TrimSpace(c.Query("group_id")); rawGroupID != "" {
		parsed, err := strconv.ParseInt(rawGroupID, 10, 64)
		if err != nil || parsed < 0 {
			return service.OwnerAPIKeyAnalyticsFilters{}, nil, "Invalid group_id"
		}
		groupID = &parsed
	}

	status, ok := parseOwnerAnalyticsStatus(c.Query("status"))
	if !ok {
		return service.OwnerAPIKeyAnalyticsFilters{}, nil, "Invalid status"
	}

	limit, ok := parseOwnerAnalyticsLimit(c.DefaultQuery("limit", ""))
	if !ok {
		return service.OwnerAPIKeyAnalyticsFilters{}, nil, "Invalid limit, allowed range is 1-100"
	}

	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		return service.OwnerAPIKeyAnalyticsFilters{}, nil, "Search is too long"
	}

	return service.OwnerAPIKeyAnalyticsFilters{
		UserID:          userID,
		APIKeyID:        apiKeyID,
		MemberID:        memberID,
		MemberScope:     memberScope,
		MemberFilterSet: memberFilterSet,
		StartTime:       startTime,
		EndTime:         endTime,
		TimezoneName:    timezoneName,
		Granularity:     granularity,
		GroupID:         groupID,
		Tags:            parseOwnerAnalyticsTags(c.Query("tags")),
		Status:          status,
		Search:          search,
		Limit:           limit,
	}, loc, ""
}

func (h *UsageHandler) validateOwnerAnalyticsMemberFilter(c *gin.Context, userID int64, filters service.OwnerAPIKeyAnalyticsFilters) bool {
	if !filters.MemberFilterSet {
		return true
	}
	if err := h.usageService.ValidateEnterpriseUsageOwner(c.Request.Context(), userID); err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	if filters.MemberID != nil {
		if err := h.usageService.ValidateOwnerUsageMember(c.Request.Context(), userID, *filters.MemberID); err != nil {
			response.ErrorFrom(c, err)
			return false
		}
	}
	if filters.APIKeyID == nil {
		return true
	}
	if h.apiKeyService == nil {
		response.InternalError(c, "API key service not available")
		return false
	}
	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), *filters.APIKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return false
	}
	if apiKey.UserID != userID {
		response.Forbidden(c, "Not authorized to access this API key's usage records")
		return false
	}
	if !apiKeyMatchesMemberFilter(apiKey, filters.MemberID, filters.MemberScope) {
		response.BadRequest(c, "api_key_id does not belong to the selected member scope")
		return false
	}
	return true
}

type userVisibleModelStat struct {
	Model               string  `json:"model"`
	Requests            int64   `json:"requests"`
	InputTokens         int64   `json:"input_tokens"`
	OutputTokens        int64   `json:"output_tokens"`
	CacheCreationTokens int64   `json:"cache_creation_tokens"`
	CacheReadTokens     int64   `json:"cache_read_tokens"`
	TotalTokens         int64   `json:"total_tokens"`
	ActualCost          float64 `json:"actual_cost"`
}

func toUserVisibleModelStats(stats []usagestats.ModelStat) []userVisibleModelStat {
	out := make([]userVisibleModelStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, userVisibleModelStat{
			Model:               stat.Model,
			Requests:            stat.Requests,
			InputTokens:         stat.InputTokens,
			OutputTokens:        stat.OutputTokens,
			CacheCreationTokens: stat.CacheCreationTokens,
			CacheReadTokens:     stat.CacheReadTokens,
			TotalTokens:         stat.TotalTokens,
			ActualCost:          stat.ActualCost,
		})
	}
	return out
}

// DashboardStats handles getting user dashboard statistics
// GET /api/v1/usage/dashboard/stats
func (h *UsageHandler) DashboardStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	stats, err := h.usageService.GetUserDashboardStats(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// DashboardTrend handles getting user usage trend data
// GET /api/v1/usage/dashboard/trend
func (h *UsageHandler) DashboardTrend(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}
	granularity := c.DefaultQuery("granularity", "day")

	trend, err := h.usageService.GetUsageTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, parsed.Filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"trend":       trend,
		"start_date":  parsed.StartTime.Format("2006-01-02"),
		"end_date":    parsed.EndTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity": granularity,
	})
}

// DashboardModels handles getting user model usage statistics
// GET /api/v1/usage/dashboard/models
func (h *UsageHandler) DashboardModels(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}

	modelSource := strings.TrimSpace(c.Query("model_source"))
	if modelSource != "" && modelSource != usagestats.ModelSourceRequested {
		response.BadRequest(c, "Invalid model_source, user usage only supports requested")
		return
	}

	stats, err := h.getUserDashboardModelStats(c, parsed)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"models":     toUserVisibleModelStats(stats),
		"start_date": parsed.StartTime.Format("2006-01-02"),
		"end_date":   parsed.EndTime.Add(-24 * time.Hour).Format("2006-01-02"),
	})
}

// DashboardSnapshotV2 returns usage-page chart data scoped to the current user.
// GET /api/v1/usage/dashboard/snapshot-v2
func (h *UsageHandler) DashboardSnapshotV2(c *gin.Context) {
	parsed, ok := h.parseUserUsageFilters(c, true)
	if !ok {
		return
	}

	granularity := strings.TrimSpace(c.DefaultQuery("granularity", "day"))
	if granularity != "hour" {
		granularity = "day"
	}
	includeTrend, ok := parseBoolQueryWithDefault(c, "include_trend", true)
	if !ok {
		return
	}
	includeModels, ok := parseBoolQueryWithDefault(c, "include_model_stats", true)
	if !ok {
		return
	}
	includeGroups, ok := parseBoolQueryWithDefault(c, "include_group_stats", false)
	if !ok {
		return
	}

	resp := gin.H{
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"start_date":   parsed.StartTime.Format("2006-01-02"),
		"end_date":     parsed.EndTime.Add(-24 * time.Hour).Format("2006-01-02"),
		"granularity":  granularity,
	}

	if includeTrend {
		trend, err := h.usageService.GetUsageTrendWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, granularity, parsed.Filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		resp["trend"] = trend
	}
	if includeModels {
		models, err := h.getUserDashboardModelStats(c, parsed)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		resp["models"] = toUserVisibleModelStats(models)
	}
	if includeGroups {
		groups, err := h.usageService.GetGroupStatsWithFilters(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		resp["groups"] = userGroupStatsFromUsageStats(groups)
	}

	response.Success(c, resp)
}

func (h *UsageHandler) getUserDashboardModelStats(c *gin.Context, parsed *userUsageFilters) ([]usagestats.ModelStat, error) {
	return h.usageService.GetModelStatsWithFiltersBySource(c.Request.Context(), parsed.StartTime, parsed.EndTime, parsed.Filters, usagestats.ModelSourceRequested)
}

func userGroupStatsFromUsageStats(stats []usagestats.GroupStat) []userGroupStat {
	out := make([]userGroupStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, userGroupStat{
			GroupID:     stat.GroupID,
			GroupName:   stat.GroupName,
			Requests:    stat.Requests,
			TotalTokens: stat.TotalTokens,
			Cost:        stat.Cost,
			ActualCost:  stat.ActualCost,
		})
	}
	return out
}

func parseBoolQueryWithDefault(c *gin.Context, key string, fallback bool) (bool, bool) {
	raw := c.Query(key)
	if strings.TrimSpace(raw) == "" {
		return fallback, true
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		response.BadRequest(c, "Invalid "+key+" value, use true or false")
		return false, false
	}
	return parsed, true
}

// BatchAPIKeysUsageRequest represents the request for batch API keys usage
type BatchAPIKeysUsageRequest struct {
	APIKeyIDs []int64 `json:"api_key_ids" binding:"required"`
}

// DashboardAPIKeysUsage handles getting usage stats for user's own API keys
// POST /api/v1/usage/dashboard/api-keys-usage
func (h *UsageHandler) DashboardAPIKeysUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req BatchAPIKeysUsageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.APIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	// Limit the number of API key IDs to prevent SQL parameter overflow
	if len(req.APIKeyIDs) > 100 {
		response.BadRequest(c, "Too many API key IDs (maximum 100 allowed)")
		return
	}

	validAPIKeyIDs, err := h.apiKeyService.VerifyOwnership(c.Request.Context(), subject.UserID, req.APIKeyIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if len(validAPIKeyIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	stats, err := h.usageService.GetBatchAPIKeyUsageStats(c.Request.Context(), validAPIKeyIDs, time.Time{}, time.Time{})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"stats": stats})
}

func ownerAnalyticsResponseMeta(filters service.OwnerAPIKeyAnalyticsFilters, loc *time.Location) gin.H {
	return gin.H{
		"start_date":  filters.StartTime.In(loc).Format(apiKeyUsageTrendDateLayout),
		"end_date":    filters.EndTime.In(loc).AddDate(0, 0, -1).Format(apiKeyUsageTrendDateLayout),
		"timezone":    filters.TimezoneName,
		"granularity": filters.Granularity,
	}
}

// ListOwnerUsageMembers returns the enterprise member directory used by usage filters.
// Archived members remain visible because historical usage facts keep their member identity.
// GET /api/v1/usage/members
func (h *UsageHandler) ListOwnerUsageMembers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	members, err := h.usageService.ListOwnerUsageMembers(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"members": members})
}

// GetOwnerAPIKeyAnalyticsSummary handles owner-level API Key usage summary.
// GET /api/v1/usage/analytics/summary
func (h *UsageHandler) GetOwnerAPIKeyAnalyticsSummary(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	summary, err := h.usageService.GetOwnerAPIKeyAnalyticsSummary(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["summary"] = summary
	response.Success(c, resp)
}

// GetOwnerAPIKeyAnalyticsLeaderboard handles owner-level employee Key ranking.
// GET /api/v1/usage/analytics/leaderboard
func (h *UsageHandler) GetOwnerAPIKeyAnalyticsLeaderboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	leaderboard, err := h.usageService.GetOwnerAPIKeyAnalyticsLeaderboard(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["items"] = leaderboard.Items
	resp["total"] = leaderboard.Total
	resp["total_actual_cost"] = leaderboard.TotalActualCost
	resp["displayed_actual_cost"] = leaderboard.DisplayedActualCost
	response.Success(c, resp)
}

// GetOwnerMemberAnalyticsLeaderboard handles enterprise member ranking and budget risk.
// GET /api/v1/usage/analytics/members
func (h *UsageHandler) GetOwnerMemberAnalyticsLeaderboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	leaderboard, err := h.usageService.GetOwnerMemberAnalyticsLeaderboard(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["items"] = leaderboard.Items
	resp["total"] = leaderboard.Total
	resp["member_count"] = leaderboard.MemberCount
	resp["budget_risk_member_count"] = leaderboard.BudgetRiskMemberCount
	resp["total_reserved_usd"] = leaderboard.TotalReservedUSD
	resp["total_actual_cost"] = leaderboard.TotalActualCost
	resp["displayed_actual_cost"] = leaderboard.DisplayedActualCost
	response.Success(c, resp)
}

// GetOwnerAPIKeyModelAnalytics handles owner-level requested-model distribution.
// GET /api/v1/usage/analytics/models
func (h *UsageHandler) GetOwnerAPIKeyModelAnalytics(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	models, err := h.usageService.GetOwnerAPIKeyModelAnalytics(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["models"] = models
	response.Success(c, resp)
}

// GetOwnerAPIKeyGroupAnalytics handles owner-level group usage split.
// GET /api/v1/usage/analytics/groups
func (h *UsageHandler) GetOwnerAPIKeyGroupAnalytics(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	groups, err := h.usageService.GetOwnerAPIKeyGroupAnalytics(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["groups"] = groups
	response.Success(c, resp)
}

// GetOwnerAPIKeyTagAnalytics handles owner-level tag attribution.
// GET /api/v1/usage/analytics/tags
func (h *UsageHandler) GetOwnerAPIKeyTagAnalytics(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	tags, err := h.usageService.GetOwnerAPIKeyTagAnalytics(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["tags"] = tags
	response.Success(c, resp)
}

// GetOwnerAPIKeyUsageTrend handles owner-level API Key usage trend.
// GET /api/v1/usage/analytics/trend
func (h *UsageHandler) GetOwnerAPIKeyUsageTrend(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	filters, loc, errMessage := parseOwnerAPIKeyAnalyticsFilters(c, subject.UserID)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}
	if !h.validateOwnerAnalyticsMemberFilter(c, subject.UserID, filters) {
		return
	}
	items, err := h.usageService.GetOwnerAPIKeyUsageTrend(c.Request.Context(), filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	resp := ownerAnalyticsResponseMeta(filters, loc)
	resp["items"] = items
	response.Success(c, resp)
}

// GetMyAPIKeyDailyUsage handles getting daily usage details for the current user's API key.
// GET /api/v1/user/api-keys/:id/usage/daily?days=30
func (h *UsageHandler) GetMyAPIKeyDailyUsage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	days, ok := parseAPIKeyDailyUsageDays(c.DefaultQuery("days", ""))
	if !ok {
		response.BadRequest(c, "Invalid days, allowed range is 1-90")
		return
	}

	if h.apiKeyService == nil {
		response.InternalError(c, "API key service is not configured")
		return
	}

	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if apiKey.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this API key's usage")
		return
	}

	userTZ := c.Query("timezone")
	startTime, endTime := apiKeyDailyUsageRange(days, userTZ)
	items, err := h.usageService.GetAPIKeyDailyUsage(c.Request.Context(), subject.UserID, apiKeyID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      items,
		"days":       days,
		"start_date": startTime.Format("2006-01-02"),
		"end_date":   endTime.AddDate(0, 0, -1).Format("2006-01-02"),
	})
}

// GetMyAPIKeyUsageTrend handles getting usage trend details for the current user's API key.
// GET /api/v1/user/api-keys/:id/usage/trend?granularity=day&start_date=2026-06-01&end_date=2026-06-14
func (h *UsageHandler) GetMyAPIKeyUsageTrend(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	granularity, ok := parseAPIKeyUsageTrendGranularity(c.DefaultQuery("granularity", ""))
	if !ok {
		response.BadRequest(c, "Invalid granularity, allowed values are hour, day, week, month")
		return
	}

	if h.apiKeyService == nil {
		response.InternalError(c, "API key service is not configured")
		return
	}

	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if apiKey.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this API key's usage")
		return
	}

	userTZ := c.Query("timezone")
	loc := apiKeyUsageTrendLocation(userTZ)
	timezoneName := apiKeyUsageTrendTimezoneName(userTZ)
	startTime, endTime, errMessage := parseAPIKeyUsageTrendRange(c.Query("start_date"), c.Query("end_date"), granularity, loc)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}

	items, err := h.usageService.GetAPIKeyUsageTrendForUser(c.Request.Context(), subject.UserID, apiKeyID, startTime, endTime, granularity, timezoneName)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":       items,
		"granularity": granularity,
		"start_date":  startTime.In(loc).Format(apiKeyUsageTrendDateLayout),
		"end_date":    endTime.In(loc).AddDate(0, 0, -1).Format(apiKeyUsageTrendDateLayout),
		"timezone":    timezoneName,
	})
}

// GetMyAPIKeyModelStats handles getting model distribution for the current user's API key.
// GET /api/v1/user/api-keys/:id/usage/models?start_date=2026-06-01&end_date=2026-06-14
func (h *UsageHandler) GetMyAPIKeyModelStats(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	apiKeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid API key ID")
		return
	}

	if h.apiKeyService == nil {
		response.InternalError(c, "API key service is not configured")
		return
	}

	apiKey, err := h.apiKeyService.GetByID(c.Request.Context(), apiKeyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if apiKey.UserID != subject.UserID {
		response.Forbidden(c, "Not authorized to access this API key's usage")
		return
	}

	userTZ := c.Query("timezone")
	loc := apiKeyUsageTrendLocation(userTZ)
	timezoneName := apiKeyUsageTrendTimezoneName(userTZ)
	startTime, endTime, errMessage := parseAPIKeyUsageTrendRange(c.Query("start_date"), c.Query("end_date"), "day", loc)
	if errMessage != "" {
		response.BadRequest(c, errMessage)
		return
	}

	stats, err := h.usageService.GetAPIKeyModelStats(c.Request.Context(), subject.UserID, apiKeyID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"models":     toUserVisibleModelStats(stats),
		"start_date": startTime.In(loc).Format(apiKeyUsageTrendDateLayout),
		"end_date":   endTime.In(loc).AddDate(0, 0, -1).Format(apiKeyUsageTrendDateLayout),
		"timezone":   timezoneName,
	})
}
