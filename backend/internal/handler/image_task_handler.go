package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type AsyncImageHandler struct {
	tasks               *service.ImageTaskService
	openAI              *OpenAIGatewayHandler
	memberBudgetService *service.EnterpriseMemberBudgetService
	execute             func(platform string, c *gin.Context)
}

func NewAsyncImageHandler(tasks *service.ImageTaskService, openAI *OpenAIGatewayHandler) *AsyncImageHandler {
	h := &AsyncImageHandler{tasks: tasks, openAI: openAI}
	if openAI != nil {
		h.memberBudgetService = openAI.memberBudgetService
	}
	h.execute = h.executeWithGateway
	return h
}

// acceptingSubmissions reports whether this instance can safely accept a new
// image task. Polling uses the weaker task-store availability gate so accepted
// tasks remain visible during uploader or credential outages.
func (h *AsyncImageHandler) acceptingSubmissions() bool {
	return h != nil && h.tasks != nil && h.tasks.Enabled()
}

// pollable reports whether task lookups can be served. It is deliberately weaker
// than enabled(): results already written to Redis stay readable after the
// feature is switched off, so an in-flight task is never stranded.
func (h *AsyncImageHandler) pollable() bool {
	return h != nil && h.tasks != nil && h.tasks.Pollable()
}

// Submit accepts the same payload as the synchronous Images endpoint and
// returns before the upstream image generation begins.
func (h *AsyncImageHandler) Submit(c *gin.Context) {
	if !h.acceptingSubmissions() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	platform := ""
	if apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	if platform != service.PlatformOpenAI && platform != service.PlatformGrok {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "Images API is not supported for this platform")
		return
	}
	if !service.GroupAllowsImageGeneration(apiKey.Group) {
		imageTaskJSONError(c, http.StatusForbidden, "permission_error", service.ImageGenerationPermissionMessage())
		return
	}
	if h == nil || h.tasks == nil || h.execute == nil {
		imageTaskError(c, service.ErrImageTaskUnavailable)
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			imageTaskJSONError(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	if asyncImageRequestStreams(c.GetHeader("Content-Type"), body) {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", "streaming image requests cannot be submitted as asynchronous tasks")
		return
	}
	if err := h.validateRequest(c, platform, body); err != nil {
		imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if !h.checkSecurityAuditBeforeSubmit(c, apiKey, platform, body) {
		return
	}

	taskCtx, recorder, cancel := newAsyncImageContext(c, body, h.tasks.ExecutionTimeout())
	budgetLink := h.imageTaskBudgetLink(c)
	task, err := h.tasks.CreateWithBudget(
		c.Request.Context(),
		service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID},
		budgetLink,
	)
	if err != nil {
		cancel()
		imageTaskError(c, err)
		return
	}
	// From this point onward, every receipt transition must be fenced by this
	// task ID. The outer request middleware may still safely release failures
	// that occur before task creation, but it must not perform its generic
	// request-ID-only fallback after a task exists.
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.MemberBudgetAsyncTaskOwned, true))
	if budgetLink != nil {
		if err := h.memberBudgetService.AttachImageTask(c.Request.Context(), budgetLink.RequestID, task.ID); err != nil {
			budgetStatus := h.releaseMemberBudget(c, task.ID)
			h.failTask(task.ID, http.StatusServiceUnavailable, imageTaskErrorPayload("api_error", "image generation task could not be linked to its budget receipt"), budgetStatus)
			cancel()
			imageTaskError(c, service.ErrImageTaskUnavailable.WithCause(err))
			return
		}
	}

	pollURL := imageTaskPollURL(c.Request.URL.Path, task.ID)
	c.Header("Cache-Control", "no-store")
	c.Header("Location", pollURL)
	c.Header("Retry-After", "3")
	response := gin.H{
		"id":         task.ID,
		"task_id":    task.TaskID,
		"object":     task.Object,
		"status":     task.Status,
		"phase":      task.Phase,
		"created_at": task.CreatedAt,
		"expires_at": task.ExpiresAt,
		"poll_url":   pollURL,
	}
	if task.Budget != nil {
		response["budget"] = task.Budget
	}
	c.JSON(http.StatusAccepted, response)

	go h.run(task.ID, platform, taskCtx, recorder, cancel)
}

func (h *AsyncImageHandler) checkSecurityAuditBeforeSubmit(c *gin.Context, apiKey *service.APIKey, platform string, body []byte) bool {
	if h == nil || h.openAI == nil {
		return true
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		imageTaskJSONError(c, http.StatusInternalServerError, "api_error", "User context not found")
		return false
	}
	model := ""
	moderationBody := body
	if platform == service.PlatformGrok {
		parsed := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
		model, moderationBody = parsed.Model, parsed.ModerationBody()
	} else if h.openAI.gatewayService != nil {
		parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
		if err != nil {
			imageTaskJSONError(c, http.StatusBadRequest, "invalid_request_error", err.Error())
			return false
		}
		model, moderationBody = parsed.Model, parsed.ModerationBody()
	}
	if len(moderationBody) == 0 {
		c.Set(securityAuditCompletedContextKey, true)
		return true
	}
	reqLog := requestLogger(c, "handler.async_image.security_audit",
		zap.Int64("user_id", subject.UserID), zap.Int64("api_key_id", apiKey.ID), zap.String("model", model))
	decision := h.openAI.checkSecurityAudit(c, reqLog, apiKey, subject, service.ContentModerationProtocolOpenAIImages, model, moderationBody)
	if decision != nil && !decision.AllowNextStage {
		h.openAI.openAISecurityAuditError(c, decision)
		return false
	}
	return true
}

func (h *AsyncImageHandler) Get(c *gin.Context) {
	// Polling deliberately does not require the feature to be enabled, only that
	// the task store is reachable. Turning the switch off in the admin UI must not
	// strand tasks that were already accepted — their results are still in Redis
	// and their submitters are still polling.
	if !h.pollable() {
		imageTaskJSONError(c, http.StatusNotFound, "not_found_error", "async image tasks are not enabled")
		return
	}
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok || apiKey == nil || apiKey.UserID <= 0 || apiKey.ID <= 0 {
		imageTaskError(c, service.ErrImageTaskForbidden)
		return
	}
	task, err := h.tasks.Get(c.Request.Context(), service.ImageTaskOwner{UserID: apiKey.UserID, APIKeyID: apiKey.ID}, c.Param("task_id"))
	if err != nil {
		imageTaskError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	if task.Status == service.ImageTaskStatusProcessing {
		c.Header("Retry-After", "3")
	}
	c.JSON(http.StatusOK, task)
}

func (h *AsyncImageHandler) validateRequest(c *gin.Context, platform string, body []byte) error {
	if h.openAI == nil || h.openAI.gatewayService == nil {
		return nil
	}
	if platform == service.PlatformGrok {
		parsed := service.ParseGrokMediaRequest(c.GetHeader("Content-Type"), body)
		if strings.TrimSpace(parsed.Model) == "" {
			return errors.New("model is required")
		}
		return nil
	}
	parsed, err := h.openAI.gatewayService.ParseOpenAIImagesRequest(c, body)
	if err != nil {
		return err
	}
	if parsed.Stream {
		return errors.New("streaming image requests cannot be submitted as asynchronous tasks")
	}
	return nil
}

func (h *AsyncImageHandler) executeWithGateway(platform string, c *gin.Context) {
	if h.openAI == nil {
		imageTaskJSONError(c, http.StatusServiceUnavailable, "api_error", "image gateway is unavailable")
		return
	}
	if platform == service.PlatformGrok {
		h.openAI.GrokImages(c)
		return
	}
	h.openAI.Images(c)
}

func (h *AsyncImageHandler) run(taskID, platform string, taskCtx *gin.Context, recorder *httptest.ResponseRecorder, cancel context.CancelFunc) {
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().Error("image_task.execution_panicked", zap.String("task_id", taskID), zap.Any("panic", recovered))
			budgetStatus := h.markMemberBudgetOutcomeAmbiguous(taskCtx, taskID, "async_image_execution_panicked")
			h.failTask(taskID, http.StatusInternalServerError, imageTaskErrorPayload("api_error", "image generation task panicked"), budgetStatus)
		}
	}()
	if err := h.markMemberBudgetTaskExecuting(taskCtx, taskID); err != nil {
		logger.L().Error("image_task.mark_budget_executing_failed", zap.String("task_id", taskID), zap.Error(err))
		budgetStatus := h.releaseMemberBudget(taskCtx, taskID)
		h.failTask(taskID, http.StatusServiceUnavailable, imageTaskErrorPayload("api_error", "image generation task could not establish its durable execution fence; any task budget hold was released"), budgetStatus)
		return
	}
	if err := h.tasks.MarkExecuting(context.Background(), taskID); err != nil {
		logger.L().Error("image_task.mark_executing_failed", zap.String("task_id", taskID), zap.Error(err))
		budgetStatus := h.releaseMemberBudget(taskCtx, taskID)
		h.failTask(taskID, http.StatusServiceUnavailable, imageTaskErrorPayload("api_error", "image generation task could not be started; any task budget hold was released"), budgetStatus)
		return
	}

	h.execute(platform, taskCtx)
	body := bytes.TrimSpace(recorder.Body.Bytes())
	if err := taskCtx.Request.Context().Err(); err != nil && len(body) == 0 {
		budgetStatus := h.markMemberBudgetOutcomeAmbiguous(taskCtx, taskID, "async_image_execution_timeout")
		h.failTask(taskID, http.StatusGatewayTimeout, imageTaskErrorPayload("timeout_error", "image generation task timed out"), budgetStatus)
		return
	}
	var actualGroupID *int64
	if activeGroup, ok := service.ActiveGroupFromContext(taskCtx.Request.Context()); ok && activeGroup.GroupID > 0 {
		groupID := activeGroup.GroupID
		actualGroupID = &groupID
	}
	if err := h.tasks.MarkFinalizingWithGroup(context.Background(), taskID, actualGroupID); err != nil {
		logger.L().Error("image_task.mark_finalizing_failed", zap.String("task_id", taskID), zap.Error(err))
	}
	statusCode := recorder.Code
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		if len(body) == 0 || !json.Valid(body) {
			budgetStatus := h.markMemberBudgetOutcomeAmbiguous(taskCtx, taskID, "async_image_invalid_success_response")
			h.failTask(taskID, http.StatusBadGateway, imageTaskErrorPayload("api_error", "upstream returned an invalid image response"), budgetStatus)
			return
		}
		budgetStatus := h.settledMemberBudgetStatus(taskCtx)
		if service.IsEnterpriseMemberBudgetOutcomeAmbiguous(taskCtx) {
			budgetStatus = h.markMemberBudgetOutcomeAmbiguous(taskCtx, taskID, "async_image_upstream_outcome_unknown")
		}
		finalizeCtx, finalizeCancel := context.WithTimeout(context.Background(), h.tasks.FinalizationTimeout())
		defer finalizeCancel()
		if err := h.tasks.CompleteWithBudgetStatus(finalizeCtx, taskID, statusCode, json.RawMessage(body), budgetStatus); err != nil {
			logger.L().Error("image_task.complete_store_failed", zap.String("task_id", taskID), zap.Error(err))
		}
		return
	}
	budgetStatus := ""
	if service.IsEnterpriseMemberBudgetOutcomeAmbiguous(taskCtx) {
		budgetStatus = h.markMemberBudgetOutcomeAmbiguous(taskCtx, taskID, "async_image_upstream_outcome_unknown")
	} else {
		budgetStatus = h.releaseMemberBudget(taskCtx, taskID)
	}
	h.failTask(taskID, statusCode, extractImageTaskError(body), budgetStatus)
}

func (h *AsyncImageHandler) memberBudgetReceipt(c *gin.Context) (*service.EnterpriseMemberBudgetReservation, bool) {
	if h == nil || h.memberBudgetService == nil || c == nil || c.Request == nil {
		return nil, false
	}
	receipt, _ := c.Request.Context().Value(ctxkey.MemberBudgetReservation).(*service.EnterpriseMemberBudgetReservation)
	return receipt, receipt != nil && strings.TrimSpace(receipt.RequestID) != ""
}

func (h *AsyncImageHandler) imageTaskBudgetLink(c *gin.Context) *service.ImageTaskBudgetLink {
	receipt, ok := h.memberBudgetReceipt(c)
	if !ok {
		return nil
	}
	return &service.ImageTaskBudgetLink{
		RequestID: receipt.RequestID, MemberID: receipt.MemberID,
		GroupID: receipt.GroupID, HeldUSD: receipt.ReservedUSD,
	}
}

func (h *AsyncImageHandler) settledMemberBudgetStatus(c *gin.Context) string {
	if _, ok := h.memberBudgetReceipt(c); !ok {
		return ""
	}
	return service.ImageTaskBudgetStatusSettled
}

func (h *AsyncImageHandler) markMemberBudgetTaskExecuting(c *gin.Context, taskID string) error {
	receipt, ok := h.memberBudgetReceipt(c)
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.memberBudgetService.MarkImageTaskExecuting(ctx, receipt.RequestID, taskID)
}

func (h *AsyncImageHandler) releaseMemberBudget(c *gin.Context, taskID string) string {
	receipt, ok := h.memberBudgetReceipt(c)
	if !ok {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, err := h.memberBudgetService.ReleaseImageTaskReservationByRequestID(ctx, receipt.RequestID, taskID)
	if err != nil {
		logger.L().Error("image_task.member_budget_release_failed", zap.String("request_id", receipt.RequestID), zap.Error(err))
		return service.ImageTaskBudgetStatusNeedsReview
	}
	return imageTaskBudgetStatusFromReceipt(updated)
}

func (h *AsyncImageHandler) markMemberBudgetAmbiguous(c *gin.Context, taskID, reason string) string {
	receipt, ok := h.memberBudgetReceipt(c)
	if !ok {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	updated, err := h.memberBudgetService.MarkImageTaskReservationAmbiguousByRequestID(ctx, receipt.RequestID, taskID, reason)
	if err != nil {
		logger.L().Error("image_task.member_budget_ambiguous_failed", zap.String("request_id", receipt.RequestID), zap.String("reason", reason), zap.Error(err))
		return service.ImageTaskBudgetStatusNeedsReview
	}
	return imageTaskBudgetStatusFromReceipt(updated)
}

func (h *AsyncImageHandler) markMemberBudgetOutcomeAmbiguous(c *gin.Context, taskID, fallbackReason string) string {
	reason := strings.TrimSpace(fallbackReason)
	if service.IsEnterpriseMemberBudgetOutcomeAmbiguous(c) {
		if markedReason := service.EnterpriseMemberBudgetOutcomeAmbiguousReason(c); markedReason != "" {
			reason = markedReason
		}
	}
	if reason == "" {
		reason = "async_image_upstream_outcome_unknown"
	}
	return h.markMemberBudgetAmbiguous(c, taskID, reason)
}

func imageTaskBudgetStatusFromReceipt(receipt *service.EnterpriseMemberBudgetReservation) string {
	if receipt == nil {
		return service.ImageTaskBudgetStatusNeedsReview
	}
	switch receipt.Status {
	case "settled":
		return service.ImageTaskBudgetStatusSettled
	case "released", "expired":
		return service.ImageTaskBudgetStatusReleased
	case "reserved":
		return service.ImageTaskBudgetStatusHeld
	default:
		return service.ImageTaskBudgetStatusNeedsReview
	}
}

func (h *AsyncImageHandler) failTask(taskID string, statusCode int, taskErr json.RawMessage, budgetStatus string) {
	if err := h.tasks.FailWithBudgetStatus(context.Background(), taskID, statusCode, taskErr, budgetStatus); err != nil {
		logger.L().Error("image_task.failure_store_failed", zap.String("task_id", taskID), zap.Error(err))
	}
}

func newAsyncImageContext(c *gin.Context, body []byte, timeoutDuration time.Duration) (*gin.Context, *httptest.ResponseRecorder, context.CancelFunc) {
	base := context.WithoutCancel(c.Request.Context())
	executionCtx, cancel := context.WithTimeout(base, timeoutDuration)
	request := c.Request.Clone(executionCtx)
	request.Body = io.NopCloser(bytes.NewReader(body))
	request.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	request.ContentLength = int64(len(body))
	request.URL.Path = strings.TrimSuffix(request.URL.Path, "/async")

	taskCtx := c.Copy()
	recorder := httptest.NewRecorder()
	recorderCtx, _ := gin.CreateTestContext(recorder)
	taskCtx.Writer = recorderCtx.Writer
	taskCtx.Request = request
	return taskCtx, recorder, cancel
}

func asyncImageRequestStreams(contentType string, body []byte) bool {
	if isMultipartImagesContentType(contentType) {
		return false
	}
	var envelope struct {
		Stream bool `json:"stream"`
	}
	return json.Unmarshal(body, &envelope) == nil && envelope.Stream
}

func imageTaskPollURL(submitPath, taskID string) string {
	if strings.HasPrefix(submitPath, "/v1/") {
		return "/v1/images/tasks/" + taskID
	}
	return "/images/tasks/" + taskID
}

func extractImageTaskError(body []byte) json.RawMessage {
	if json.Valid(body) {
		var envelope struct {
			Error json.RawMessage `json:"error"`
		}
		if json.Unmarshal(body, &envelope) == nil && len(envelope.Error) > 0 && json.Valid(envelope.Error) {
			return envelope.Error
		}
		return json.RawMessage(body)
	}
	return imageTaskErrorPayload("api_error", "image generation failed")
}

func imageTaskErrorPayload(errorType, message string) json.RawMessage {
	data, _ := json.Marshal(gin.H{"type": errorType, "message": message})
	return data
}

func imageTaskError(c *gin.Context, err error) {
	status := infraerrors.Code(err)
	code := infraerrors.Reason(err)
	message := infraerrors.Message(err)
	if status <= 0 {
		status = http.StatusInternalServerError
	}
	if strings.TrimSpace(code) == "" {
		code = "IMAGE_TASK_ERROR"
	}
	imageTaskJSONError(c, status, code, message)
}

func imageTaskJSONError(c *gin.Context, status int, code, message string) {
	c.Header("Cache-Control", "no-store")
	c.JSON(status, gin.H{"error": gin.H{"type": code, "code": code, "message": message}})
}
