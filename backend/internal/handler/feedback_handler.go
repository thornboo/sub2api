package handler

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const feedbackRequestBodyMaxBytes int64 = 32 << 10

type FeedbackHandler struct {
	feedbackService *service.FeedbackService
	apiKeyService   *service.APIKeyService
}

func NewFeedbackHandler(feedbackService *service.FeedbackService, apiKeyService *service.APIKeyService) *FeedbackHandler {
	return &FeedbackHandler{feedbackService: feedbackService, apiKeyService: apiKeyService}
}

type submitFeedbackRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type feedbackContentRequest struct {
	Content string `json:"content"`
}

type markFeedbackReadRequest struct {
	LastReadReplyID *int64 `json:"last_read_reply_id"`
}

type submitFeedbackResponse struct {
	ID         int64     `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	RetryAfter int       `json:"retry_after"`
}

type feedbackReplyResponse struct {
	Message    dto.FeedbackReply `json:"message"`
	RetryAfter int               `json:"retry_after"`
}

type feedbackReadResponse struct {
	UnreadCount     int64 `json:"unread_count"`
	LastReadReplyID int64 `json:"last_read_reply_id"`
}

func (h *FeedbackHandler) SubmitUser(c *gin.Context) {
	setFeedbackNoStore(c)
	req, ok := bindFeedbackRequest(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	result, err := h.feedbackService.CreateForUser(c.Request.Context(), subject.UserID, req.Title, req.Content)
	if err != nil {
		writeFeedbackError(c, err)
		return
	}
	writeFeedbackCreated(c, result)
}

func (h *FeedbackHandler) ListUser(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	items, result, err := h.feedbackService.ListForUser(c.Request.Context(), feedbackPagination(c), feedbackFilters(c), subject.UserID)
	writeFeedbackList(c, items, result, err, false)
}

func (h *FeedbackHandler) GetUser(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	item, err := h.feedbackService.GetForUser(c.Request.Context(), id, subject.UserID)
	writeFeedbackItem(c, item, err, false)
}

func (h *FeedbackHandler) ListUserMessages(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	items, result, err := h.feedbackService.ListRepliesForUser(c.Request.Context(), id, feedbackPagination(c), subject.UserID)
	writeFeedbackMessages(c, items, result, err)
}

func (h *FeedbackHandler) ReplyUser(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	req, ok := bindFeedbackContentRequest(c)
	if !ok {
		return
	}
	result, err := h.feedbackService.ReplyForUser(c.Request.Context(), id, subject.UserID, req.Content)
	writeFeedbackReply(c, result, err)
}

func (h *FeedbackHandler) CloseUser(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	if !bindEmptyJSONBody(c) {
		return
	}
	item, err := h.feedbackService.CloseForUser(c.Request.Context(), id, subject.UserID)
	writeFeedbackItem(c, item, err, false)
}

func (h *FeedbackHandler) MarkUserRead(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	req, ok := bindFeedbackReadRequest(c)
	if !ok {
		return
	}
	state, err := h.feedbackService.MarkReadForUser(c.Request.Context(), id, subject.UserID, *req.LastReadReplyID)
	writeFeedbackRead(c, state, err)
}

func (h *FeedbackHandler) SubmitKey(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	req, ok := bindFeedbackRequest(c)
	if !ok {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	result, err := h.feedbackService.CreateForKey(c.Request.Context(), session, req.Title, req.Content)
	if err != nil {
		writeFeedbackError(c, err)
		return
	}
	writeFeedbackCreated(c, result)
}

func (h *FeedbackHandler) ListKey(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	items, result, err := h.feedbackService.ListForKey(c.Request.Context(), feedbackPagination(c), feedbackFilters(c), session)
	writeFeedbackList(c, items, result, err, false)
}

func (h *FeedbackHandler) GetKey(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	item, err := h.feedbackService.GetForKey(c.Request.Context(), id, session)
	writeFeedbackItem(c, item, err, false)
}

func (h *FeedbackHandler) ListKeyMessages(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	items, result, err := h.feedbackService.ListRepliesForKey(c.Request.Context(), id, feedbackPagination(c), session)
	writeFeedbackMessages(c, items, result, err)
}

func (h *FeedbackHandler) ReplyKey(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	req, ok := bindFeedbackContentRequest(c)
	if !ok {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	result, err := h.feedbackService.ReplyForKey(c.Request.Context(), id, session, req.Content)
	writeFeedbackReply(c, result, err)
}

func (h *FeedbackHandler) CloseKey(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	if !bindEmptyJSONBody(c) {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	item, err := h.feedbackService.CloseForKey(c.Request.Context(), id, session)
	writeFeedbackItem(c, item, err, false)
}

func (h *FeedbackHandler) MarkKeyRead(c *gin.Context) {
	setFeedbackNoStore(c)
	if rejectCrossSiteFeedback(c) {
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	req, ok := bindFeedbackReadRequest(c)
	if !ok {
		return
	}
	session, ok := h.publicKeyUsageSession(c)
	if !ok {
		return
	}
	state, err := h.feedbackService.MarkReadForKey(c.Request.Context(), id, session, *req.LastReadReplyID)
	writeFeedbackRead(c, state, err)
}

func (h *FeedbackHandler) publicKeyUsageSession(c *gin.Context) (*service.PublicKeyUsageSession, bool) {
	if h.apiKeyService == nil {
		response.ErrorFrom(c, service.ErrPublicKeyUsageSessionUnavailable)
		return nil, false
	}
	token, err := c.Cookie(publicKeyUsageSessionCookie)
	if err != nil || strings.TrimSpace(token) == "" {
		response.ErrorFrom(c, service.ErrPublicKeyUsageSessionInvalid)
		return nil, false
	}
	session, _, _, err := h.apiKeyService.ResolvePublicKeyUsageSession(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, service.ErrPublicKeyUsageSessionInvalid) {
			clearPublicKeyUsageCookie(c)
		}
		response.ErrorFrom(c, err)
		return nil, false
	}
	return session, true
}

func bindFeedbackRequest(c *gin.Context) (submitFeedbackRequest, bool) {
	var req submitFeedbackRequest
	if !bindStrictJSON(c, feedbackRequestBodyMaxBytes, &req) {
		return req, false
	}
	if _, err := service.NormalizeFeedbackContent(req.Content); err != nil {
		response.ErrorFrom(c, err)
		return req, false
	}
	return req, true
}

func bindFeedbackContentRequest(c *gin.Context) (feedbackContentRequest, bool) {
	var req feedbackContentRequest
	if !bindStrictJSON(c, feedbackRequestBodyMaxBytes, &req) {
		return req, false
	}
	if _, err := service.NormalizeFeedbackContent(req.Content); err != nil {
		response.ErrorFrom(c, err)
		return req, false
	}
	return req, true
}

func bindFeedbackReadRequest(c *gin.Context) (markFeedbackReadRequest, bool) {
	var req markFeedbackReadRequest
	if !bindStrictJSON(c, 4<<10, &req) {
		return req, false
	}
	if req.LastReadReplyID == nil || *req.LastReadReplyID < 0 {
		response.ErrorFrom(c, service.ErrFeedbackInvalidReadID)
		return req, false
	}
	return req, true
}

func bindEmptyJSONBody(c *gin.Context) bool {
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return true
	}
	var req struct{}
	return bindStrictJSON(c, 4<<10, &req)
}

func bindStrictJSON(c *gin.Context, maxBytes int64, out any) bool {
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "application/json") {
		response.BadRequest(c, "Content-Type must be application/json")
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.ErrorWithDetails(c, http.StatusRequestEntityTooLarge, "request body too large", "REQUEST_BODY_TOO_LARGE", nil)
			return false
		}
		response.BadRequest(c, "Invalid request: "+err.Error())
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			response.ErrorWithDetails(c, http.StatusRequestEntityTooLarge, "request body too large", "REQUEST_BODY_TOO_LARGE", nil)
			return false
		}
		response.BadRequest(c, "Invalid request: trailing JSON is not allowed")
		return false
	}
	return true
}

func writeFeedbackCreated(c *gin.Context, result *service.FeedbackCreateResult) {
	if result == nil || result.Feedback == nil {
		response.ErrorFrom(c, service.ErrFeedbackUnavailable)
		return
	}
	response.Created(c, submitFeedbackResponse{
		ID:         result.Feedback.ID,
		CreatedAt:  result.Feedback.CreatedAt,
		RetryAfter: int(result.RetryAfter.Seconds()),
	})
}

func writeFeedbackReply(c *gin.Context, result *service.FeedbackReplyResult, err error) {
	if err != nil {
		writeFeedbackError(c, err)
		return
	}
	if result == nil || result.Message == nil {
		response.ErrorFrom(c, service.ErrFeedbackUnavailable)
		return
	}
	response.Created(c, feedbackReplyResponse{
		Message:    *dto.FeedbackReplyFromService(result.Message),
		RetryAfter: int(result.RetryAfter.Seconds()),
	})
}

func writeFeedbackList(c *gin.Context, items []service.Feedback, result *pagination.PaginationResult, err error, admin bool) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result == nil {
		response.ErrorFrom(c, service.ErrFeedbackUnavailable)
		return
	}
	if admin {
		out := make([]dto.AdminFeedback, 0, len(items))
		for i := range items {
			out = append(out, *dto.AdminFeedbackFromService(&items[i]))
		}
		response.Paginated(c, out, result.Total, result.Page, result.PageSize)
		return
	}
	out := make([]dto.Feedback, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, result.Page, result.PageSize)
}

func writeFeedbackItem(c *gin.Context, item *service.Feedback, err error, admin bool) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if admin {
		response.Success(c, dto.AdminFeedbackFromService(item))
		return
	}
	response.Success(c, dto.FeedbackFromService(item))
}

func writeFeedbackMessages(c *gin.Context, items []service.FeedbackReply, result *pagination.PaginationResult, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if result == nil {
		response.ErrorFrom(c, service.ErrFeedbackUnavailable)
		return
	}
	out := make([]dto.FeedbackReply, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackReplyFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, result.Page, result.PageSize)
}

func writeFeedbackRead(c *gin.Context, state *service.FeedbackReadState, err error) {
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if state == nil {
		response.ErrorFrom(c, service.ErrFeedbackUnavailable)
		return
	}
	response.Success(c, feedbackReadResponse{
		UnreadCount:     state.UnreadCount,
		LastReadReplyID: state.LastReadReplyID,
	})
}

func writeFeedbackError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrFeedbackRateLimited) {
		metadata := map[string]string{"retry_after": "60"}
		app := infraerrors.FromError(err)
		if app != nil && app.Metadata != nil {
			metadata = app.Metadata
		}
		retryAfter := metadata["retry_after"]
		if retryAfter == "" {
			retryAfter = "60"
			metadata["retry_after"] = retryAfter
		}
		c.Header("Retry-After", retryAfter)
		response.ErrorWithDetails(c, http.StatusTooManyRequests, app.Message, app.Reason, metadata)
		return
	}
	response.ErrorFrom(c, err)
}

func feedbackIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, service.ErrFeedbackInvalidID)
		return 0, false
	}
	return id, true
}

func feedbackPagination(c *gin.Context) pagination.PaginationParams {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	return pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "updated_at", SortOrder: pagination.SortOrderDesc}
}

func feedbackFilters(c *gin.Context) service.FeedbackListFilters {
	return service.FeedbackListFilters{
		Status:      strings.TrimSpace(c.Query("status")),
		ReplyStatus: strings.TrimSpace(c.Query("reply_status")),
	}
}

func rejectCrossSiteFeedback(c *gin.Context) bool {
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("Sec-Fetch-Site")), "cross-site") {
		response.Forbidden(c, "Cross-site feedback requests are not allowed")
		return true
	}
	return false
}

func setFeedbackNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}
