package handler

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// EnterpriseMemberHandler manages non-login members owned by an enterprise user.
type EnterpriseMemberHandler struct {
	service       *service.EnterpriseMemberService
	budgetService *service.EnterpriseMemberBudgetService
	importService *service.EnterpriseMemberImportService
	auditRepo     service.EnterpriseMemberAuditRepository
}

func NewEnterpriseMemberHandler(memberService *service.EnterpriseMemberService, budgetService *service.EnterpriseMemberBudgetService, importService *service.EnterpriseMemberImportService, auditRepo service.EnterpriseMemberAuditRepository) *EnterpriseMemberHandler {
	return &EnterpriseMemberHandler{service: memberService, budgetService: budgetService, importService: importService, auditRepo: auditRepo}
}

type enterpriseMemberStatusRequest struct {
	ExpectedVersion int64 `json:"expected_version" binding:"required"`
}

type enterpriseMemberBudgetAdjustmentRequest struct {
	AmountUSD float64 `json:"amount_usd" binding:"required"`
	Note      string  `json:"note" binding:"required"`
}

type enterpriseMemberImportCommitRequest struct {
	JobID           int64   `json:"job_id" binding:"required"`
	PreviewToken    string  `json:"preview_token" binding:"required"`
	SelectedRows    []int   `json:"selected_rows"`
	DefaultGroupIDs []int64 `json:"default_group_ids"`
	ActivateMembers bool    `json:"activate_members"`
}

type enterpriseMemberImportResultSecretsRequest struct {
	ResultToken string `json:"result_token" binding:"required"`
}

func (h *EnterpriseMemberHandler) List(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	includeArchived, err := strconv.ParseBool(c.DefaultQuery("include_archived", "false"))
	if err != nil {
		response.BadRequest(c, "Invalid include_archived value")
		return
	}
	members, err := h.service.List(c.Request.Context(), ownerID, includeArchived)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, members)
}

func (h *EnterpriseMemberHandler) Create(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	var req service.CreateEnterpriseMemberInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSON(c, "user.enterprise_members.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.service.Create(ctx, ownerID, req, c.GetHeader("Idempotency-Key"))
	})
}

func (h *EnterpriseMemberHandler) Get(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	includeArchived, err := strconv.ParseBool(c.DefaultQuery("include_archived", "false"))
	if err != nil {
		response.BadRequest(c, "Invalid include_archived value")
		return
	}
	member, err := h.service.Get(c.Request.Context(), ownerID, memberID, includeArchived)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, member)
}

func (h *EnterpriseMemberHandler) Update(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	var req service.UpdateEnterpriseMemberInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	member, err := h.service.Update(c.Request.Context(), ownerID, memberID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, member)
}

func (h *EnterpriseMemberHandler) Disable(c *gin.Context) {
	h.setStatus(c, service.EnterpriseMemberStatusDisabled)
}

func (h *EnterpriseMemberHandler) Enable(c *gin.Context) {
	h.setStatus(c, service.EnterpriseMemberStatusActive)
}

func (h *EnterpriseMemberHandler) Restore(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	var req enterpriseMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	member, err := h.service.Restore(c.Request.Context(), ownerID, memberID, req.ExpectedVersion)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, member)
}

func (h *EnterpriseMemberHandler) setStatus(c *gin.Context, status string) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	var req enterpriseMemberStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	member, err := h.service.SetStatus(c.Request.Context(), ownerID, memberID, req.ExpectedVersion, status)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, member)
}

func (h *EnterpriseMemberHandler) Delete(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	permanent, err := strconv.ParseBool(c.DefaultQuery("permanent", "false"))
	if err != nil {
		response.BadRequest(c, "Invalid permanent value")
		return
	}
	if permanent {
		result, deleteErr := h.service.DeletePermanently(c.Request.Context(), ownerID, memberID)
		if deleteErr != nil {
			response.ErrorFrom(c, deleteErr)
			return
		}
		response.Success(c, gin.H{
			"archived":            false,
			"permanently_deleted": true,
			"deletion_mode":       result.Mode,
		})
		return
	}
	expectedVersion, parseErr := strconv.ParseInt(c.Query("expected_version"), 10, 64)
	if parseErr != nil || expectedVersion <= 0 {
		response.BadRequest(c, "expected_version is required")
		return
	}
	if err := h.service.Archive(c.Request.Context(), ownerID, memberID, expectedVersion); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"archived": true, "permanently_deleted": false})
}

func (h *EnterpriseMemberHandler) GetGroups(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	member, err := h.service.Get(c.Request.Context(), ownerID, memberID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"group_ids": member.GroupIDs, "version": member.Version})
}

func (h *EnterpriseMemberHandler) ReplaceGroups(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	var req service.ReplaceEnterpriseMemberGroupsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	member, err := h.service.ReplaceGroups(c.Request.Context(), ownerID, memberID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"group_ids": member.GroupIDs, "version": member.Version})
}

func (h *EnterpriseMemberHandler) BatchReplaceGroups(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	var req service.BatchReplaceEnterpriseMemberGroupsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	members, err := h.service.BatchReplaceGroups(c.Request.Context(), ownerID, req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, members)
}

func (h *EnterpriseMemberHandler) ListKeys(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	keys, err := h.service.ListKeys(c.Request.Context(), ownerID, memberID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.APIKey, 0, len(keys))
	for i := range keys {
		item := dto.APIKeyFromService(&keys[i])
		item.Key = maskAPIKeyForIdempotencyReplay(item.Key)
		out = append(out, *item)
	}
	response.Success(c, out)
}

func (h *EnterpriseMemberHandler) ListAdoptableKeys(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	keys, err := h.service.ListAdoptableKeys(c.Request.Context(), ownerID, memberID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.APIKey, 0, len(keys))
	for i := range keys {
		item := dto.APIKeyFromService(&keys[i])
		item.Key = maskAPIKeyForIdempotencyReplay(item.Key)
		out = append(out, *item)
	}
	response.Success(c, out)
}

func (h *EnterpriseMemberHandler) AdoptKey(c *gin.Context) {
	ownerID, memberID, keyID, ok := enterpriseMemberKeyIDs(c)
	if !ok {
		return
	}
	var req service.AdoptEnterpriseMemberKeyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	payload := struct {
		MemberID int64 `json:"member_id"`
		KeyID    int64 `json:"key_id"`
		service.AdoptEnterpriseMemberKeyInput
	}{MemberID: memberID, KeyID: keyID, AdoptEnterpriseMemberKeyInput: req}
	executeUserIdempotentJSON(c, "user.enterprise_members.keys.adopt", payload, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.service.AdoptKey(ctx, ownerID, memberID, keyID, req)
	})
}

func (h *EnterpriseMemberHandler) CreateKey(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	var req CreateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	svcReq := createAPIKeyRequestToService(req)
	executeUserIdempotentJSON(c, "user.enterprise_members.keys.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		key, err := h.service.CreateKey(ctx, ownerID, memberID, svcReq)
		if err != nil {
			return nil, err
		}
		return dto.APIKeyFromService(key), nil
	})
}

func (h *EnterpriseMemberHandler) UpdateKey(c *gin.Context) {
	ownerID, memberID, keyID, ok := enterpriseMemberKeyIDs(c)
	if !ok {
		return
	}
	var req UpdateAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	svcReq, err := updateAPIKeyRequestToService(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	key, err := h.service.UpdateKey(c.Request.Context(), ownerID, memberID, keyID, svcReq)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := dto.APIKeyFromService(key)
	out.Key = maskAPIKeyForIdempotencyReplay(out.Key)
	response.Success(c, out)
}

func (h *EnterpriseMemberHandler) DeleteKey(c *gin.Context) {
	ownerID, memberID, keyID, ok := enterpriseMemberKeyIDs(c)
	if !ok {
		return
	}
	if err := h.service.DeleteKey(c.Request.Context(), ownerID, memberID, keyID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (h *EnterpriseMemberHandler) GetBudget(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	if _, err := h.service.Get(c.Request.Context(), ownerID, memberID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	summary, err := h.budgetService.GetSummary(c.Request.Context(), memberID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *EnterpriseMemberHandler) ListBudgetEntries(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	if _, err := h.service.Get(c.Request.Context(), ownerID, memberID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	entries, total, err := h.budgetService.ListEntries(c.Request.Context(), memberID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": entries, "total": total, "page": page, "page_size": pageSize})
}

func (h *EnterpriseMemberHandler) ListAuditEvents(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	if _, err := h.service.Get(c.Request.Context(), ownerID, memberID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	page, pageSize := enterpriseMemberAuditPagination(c)
	items, total, err := h.auditRepo.ListByMember(c.Request.Context(), ownerID, memberID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *EnterpriseMemberHandler) ListOwnerAuditEvents(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	if _, err := h.service.List(c.Request.Context(), ownerID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	page, pageSize := enterpriseMemberAuditPagination(c)
	items, total, err := h.auditRepo.ListByOwner(c.Request.Context(), ownerID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func enterpriseMemberAuditPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func (h *EnterpriseMemberHandler) CreateBudgetAdjustment(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	if _, err := h.service.Get(c.Request.Context(), ownerID, memberID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req enterpriseMemberBudgetAdjustmentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSON(c, "user.enterprise_members.budget.adjust", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.budgetService.CreateAdjustment(ctx, memberID, ownerID, req.AmountUSD, c.GetHeader("Idempotency-Key"), req.Note); err != nil {
			return nil, err
		}
		return h.budgetService.GetSummary(ctx, memberID)
	})
}

func (h *EnterpriseMemberHandler) SetUsage(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	if _, err := h.service.Get(c.Request.Context(), ownerID, memberID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var req service.EnterpriseMemberUsageAdjustmentInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSON(c, "user.enterprise_members.usage.adjust", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		if err := h.budgetService.SetUsage(ctx, ownerID, memberID, req, c.GetHeader("Idempotency-Key")); err != nil {
			return nil, err
		}
		return h.budgetService.GetSummary(ctx, memberID)
	})
}

func (h *EnterpriseMemberHandler) GetUsageAnalytics(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	if _, err := h.service.Get(c.Request.Context(), ownerID, memberID, true); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	analytics, err := h.budgetService.GetUsageAnalytics(c.Request.Context(), memberID, days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, analytics)
}

func (h *EnterpriseMemberHandler) ListUsageRecords(c *gin.Context) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return
	}
	page, pageSize := enterpriseMemberAuditPagination(c)
	items, total, err := h.service.ListUsageRecords(c.Request.Context(), ownerID, memberID, page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *EnterpriseMemberHandler) GetOwnerUsageSummary(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	if _, err := h.service.List(c.Request.Context(), ownerID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	summary, err := h.budgetService.GetOwnerUsageSummary(c.Request.Context(), ownerID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, summary)
}

func (h *EnterpriseMemberHandler) GetOwnerUsageTrend(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	if _, err := h.service.List(c.Request.Context(), ownerID, false); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	trend, start, end, err := h.budgetService.GetOwnerUsageTrend(c.Request.Context(), ownerID, days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"start": start, "end": end, "trend": trend})
}

func (h *EnterpriseMemberHandler) ImportTemplate(c *gin.Context) {
	if _, ok := enterpriseOwnerID(c); !ok {
		return
	}
	format := strings.ToLower(strings.TrimSpace(c.DefaultQuery("format", "csv")))
	if format == "csv" {
		c.Header("Content-Disposition", enterpriseMemberImportTemplateContentDisposition(format))
		c.Data(http.StatusOK, "text/csv; charset=utf-8", service.EnterpriseMemberImportCSVTemplate())
		return
	}
	if format == "xlsx" {
		data, err := service.EnterpriseMemberImportXLSXTemplate()
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		c.Header("Content-Disposition", enterpriseMemberImportTemplateContentDisposition(format))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
		return
	}
	response.BadRequest(c, "format must be csv or xlsx")
}

func enterpriseMemberImportTemplateContentDisposition(format string) string {
	filename := "企业成员导入模板." + format
	return `attachment; filename="enterprise-members-template.` + format + `"; filename*=UTF-8''` + url.PathEscape(filename)
}

func (h *EnterpriseMemberHandler) ImportPreview(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "A CSV or XLSX file is required")
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, (10<<20)+1))
	if err != nil || len(data) > 10<<20 {
		response.BadRequest(c, "Import file exceeds the 10 MiB limit")
		return
	}
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(header.Filename)), ".")
	if requested := strings.ToLower(strings.TrimSpace(c.PostForm("format"))); requested != "" {
		format = requested
	}
	importPolicyVersion, ok := parseEnterpriseMemberImportPolicyVersion(c.PostForm("import_policy_version"))
	if !ok {
		response.BadRequest(c, "Unsupported enterprise member import policy version")
		return
	}
	preview, err := h.importService.PreviewWithPolicy(c.Request.Context(), ownerID, format, data, importPolicyVersion)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, preview)
}

func parseEnterpriseMemberImportPolicyVersion(raw string) (int, bool) {
	requested := strings.TrimSpace(raw)
	if requested == "" {
		return service.EnterpriseMemberImportPolicyExplicitActivation, true
	}
	parsed, err := strconv.Atoi(requested)
	return parsed, err == nil && parsed == service.EnterpriseMemberImportPolicyExplicitActivation
}

func (h *EnterpriseMemberHandler) ImportCommit(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	var req enterpriseMemberImportCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	executeUserIdempotentJSONWithStoredResponse(
		c,
		"user.enterprise_members.import.commit",
		req,
		service.DefaultWriteIdempotencyTTL(),
		redactEnterpriseMemberImportResultForReplay,
		func(ctx context.Context) (any, error) {
			return h.importService.QueueCommit(ctx, ownerID, req.JobID, req.PreviewToken, req.SelectedRows, req.DefaultGroupIDs, req.ActivateMembers, c.GetHeader("Idempotency-Key"))
		},
	)
}

func redactEnterpriseMemberImportResultForReplay(data any) (any, error) {
	result, ok := data.(*service.EnterpriseMemberImportResult)
	if !ok || result == nil {
		return data, nil
	}
	redacted := *result
	redacted.Keys = append([]service.EnterpriseMemberImportCreatedKey(nil), result.Keys...)
	for i := range redacted.Keys {
		redacted.Keys[i].Key = ""
	}
	return &redacted, nil
}

func (h *EnterpriseMemberHandler) GetImportJob(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	jobID, err := strconv.ParseInt(c.Param("job_id"), 10, 64)
	if err != nil || jobID <= 0 {
		response.BadRequest(c, "Invalid import job ID")
		return
	}
	job, err := h.importService.GetJob(c.Request.Context(), ownerID, jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"id": job.ID, "status": job.Status, "preview": job.Preview, "result": job.Result,
		"selected_rows": job.SelectedRows, "attempt_count": job.AttemptCount,
		"default_group_ids": job.DefaultGroupIDs, "activate_members": job.ActivateMembers,
		"error_code": job.ErrorCode, "error_summary": job.ErrorSummary,
		"expires_at": job.ExpiresAt, "created_at": job.CreatedAt, "queued_at": job.QueuedAt,
		"started_at": job.StartedAt, "updated_at": job.UpdatedAt, "completed_at": job.CompletedAt,
		"result_secrets_consumed_at": job.ResultSecretsConsumedAt,
	})
}

func (h *EnterpriseMemberHandler) ConsumeImportResultSecrets(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	jobID, err := strconv.ParseInt(c.Param("job_id"), 10, 64)
	if err != nil || jobID <= 0 {
		response.BadRequest(c, "Invalid import job ID")
		return
	}
	var req enterpriseMemberImportResultSecretsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	keys, err := h.importService.ConsumeResultSecrets(c.Request.Context(), ownerID, jobID, req.ResultToken)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"keys": keys})
}

func (h *EnterpriseMemberHandler) DownloadImportErrorReport(c *gin.Context) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return
	}
	jobID, err := strconv.ParseInt(c.Param("job_id"), 10, 64)
	if err != nil || jobID <= 0 {
		response.BadRequest(c, "Invalid import job ID")
		return
	}
	job, err := h.importService.GetJob(c.Request.Context(), ownerID, jobID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if job.Status != "failed" {
		response.ErrorFrom(c, service.ErrEnterpriseMemberImportPending)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="enterprise-member-import-errors.csv"`)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"row_number", "member_code", "error_code", "error_summary"})
	selected := make(map[int]bool, len(job.SelectedRows))
	for _, rowNumber := range job.SelectedRows {
		selected[rowNumber] = true
	}
	code, summary := "ENTERPRISE_MEMBER_IMPORT_FAILED", "enterprise member import transaction failed"
	if job.ErrorCode != nil {
		code = *job.ErrorCode
	}
	if job.ErrorSummary != nil {
		summary = *job.ErrorSummary
	}
	for _, row := range job.Preview.Rows {
		if selected[row.RowNumber] {
			_ = writer.Write([]string{strconv.Itoa(row.RowNumber), row.MemberCode, code, summary})
		}
	}
	writer.Flush()
}

func enterpriseOwnerID(c *gin.Context) (int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return 0, false
	}
	return subject.UserID, true
}

func enterpriseMemberIDs(c *gin.Context) (int64, int64, bool) {
	ownerID, ok := enterpriseOwnerID(c)
	if !ok {
		return 0, 0, false
	}
	memberID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || memberID <= 0 {
		response.BadRequest(c, "Invalid member ID")
		return 0, 0, false
	}
	return ownerID, memberID, true
}

func enterpriseMemberKeyIDs(c *gin.Context) (int64, int64, int64, bool) {
	ownerID, memberID, ok := enterpriseMemberIDs(c)
	if !ok {
		return 0, 0, 0, false
	}
	keyID, err := strconv.ParseInt(c.Param("key_id"), 10, 64)
	if err != nil || keyID <= 0 {
		response.BadRequest(c, "Invalid key ID")
		return 0, 0, 0, false
	}
	return ownerID, memberID, keyID, true
}

func createAPIKeyRequestToService(req CreateAPIKeyRequest) service.CreateAPIKeyRequest {
	out := service.CreateAPIKeyRequest{
		Name: req.Name, Tags: req.Tags, GroupID: req.GroupID, CustomKey: req.CustomKey,
		IPWhitelist: req.IPWhitelist, IPBlacklist: req.IPBlacklist, ExpiresInDays: req.ExpiresInDays,
	}
	if req.Quota != nil {
		out.Quota = *req.Quota
	}
	if req.RateLimit5h != nil {
		out.RateLimit5h = *req.RateLimit5h
	}
	if req.RateLimit1d != nil {
		out.RateLimit1d = *req.RateLimit1d
	}
	if req.RateLimit7d != nil {
		out.RateLimit7d = *req.RateLimit7d
	}
	return out
}

func updateAPIKeyRequestToService(req UpdateAPIKeyRequest) (service.UpdateAPIKeyRequest, error) {
	out := service.UpdateAPIKeyRequest{
		Tags: req.Tags, GroupID: req.GroupID, IPWhitelist: req.IPWhitelist, IPBlacklist: req.IPBlacklist,
		Quota: req.Quota, ResetQuota: req.ResetQuota, RateLimit5h: req.RateLimit5h, RateLimit1d: req.RateLimit1d,
		RateLimit7d: req.RateLimit7d, ResetRateLimitUsage: req.ResetRateLimitUsage,
	}
	if req.Name != "" {
		out.Name = &req.Name
	}
	if req.Status != "" {
		out.Status = &req.Status
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			out.ClearExpiration = true
		} else {
			value, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err != nil {
				return service.UpdateAPIKeyRequest{}, err
			}
			out.ExpiresAt = &value
		}
	}
	return out, nil
}
