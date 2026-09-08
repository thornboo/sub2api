package admin

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct {
	feedbackService *service.FeedbackService
}

type feedbackContentRequest struct {
	Content string `json:"content"`
}

type feedbackReadRequest struct {
	LastReadReplyID *int64 `json:"last_read_reply_id"`
}

type feedbackReadResponse struct {
	UnreadCount     int64 `json:"unread_count"`
	LastReadReplyID int64 `json:"last_read_reply_id"`
}

func NewFeedbackHandler(feedbackService *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{feedbackService: feedbackService}
}

func (h *FeedbackHandler) List(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin not found in context")
		return
	}
	items, result, err := h.feedbackService.List(c.Request.Context(), adminFeedbackPagination(c), feedbackFilters(c), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.AdminFeedback, 0, len(items))
	for i := range items {
		out = append(out, *dto.AdminFeedbackFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, result.Page, result.PageSize)
}

func (h *FeedbackHandler) Get(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	item, err := h.feedbackService.Get(c.Request.Context(), id, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminFeedbackFromService(item))
}

func (h *FeedbackHandler) ListMessages(c *gin.Context) {
	setFeedbackNoStore(c)
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	items, result, err := h.feedbackService.ListReplies(c.Request.Context(), id, adminFeedbackPagination(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.FeedbackReply, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackReplyFromService(&items[i]))
	}
	response.Paginated(c, out, result.Total, result.Page, result.PageSize)
}

func (h *FeedbackHandler) Reply(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	req, ok := bindFeedbackRequest(c)
	if !ok {
		return
	}
	result, err := h.feedbackService.ReplyAsAdmin(c.Request.Context(), id, subject.UserID, req.Content)
	if err != nil {
		writeFeedbackError(c, err)
		return
	}
	response.Created(c, gin.H{
		"message":     dto.FeedbackReplyFromService(result.Message),
		"retry_after": int(result.RetryAfter.Seconds()),
	})
}

func (h *FeedbackHandler) Close(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin not found in context")
		return
	}
	id, ok := feedbackIDParam(c)
	if !ok {
		return
	}
	if !bindEmptyJSONBody(c) {
		return
	}
	item, err := h.feedbackService.Close(c.Request.Context(), id, subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.AdminFeedbackFromService(item))
}

func (h *FeedbackHandler) MarkRead(c *gin.Context) {
	setFeedbackNoStore(c)
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Unauthorized(c, "Admin not found in context")
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
	state, err := h.feedbackService.MarkRead(c.Request.Context(), id, subject.UserID, *req.LastReadReplyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, feedbackReadResponse{UnreadCount: state.UnreadCount, LastReadReplyID: state.LastReadReplyID})
}

func setFeedbackNoStore(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
}

func adminFeedbackPagination(c *gin.Context) pagination.PaginationParams {
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	return pagination.PaginationParams{Page: page, PageSize: pageSize, SortBy: "updated_at", SortOrder: pagination.SortOrderDesc}
}

func feedbackFilters(c *gin.Context) service.FeedbackListFilters {
	return service.FeedbackListFilters{Status: strings.TrimSpace(c.Query("status"))}
}

func feedbackIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, service.ErrFeedbackInvalidID)
		return 0, false
	}
	return id, true
}

func bindFeedbackRequest(c *gin.Context) (feedbackContentRequest, bool) {
	var req feedbackContentRequest
	if !bindStrictJSON(c, 32<<10, &req) {
		return req, false
	}
	if _, err := service.NormalizeFeedbackContent(req.Content); err != nil {
		response.ErrorFrom(c, err)
		return req, false
	}
	return req, true
}

func bindFeedbackReadRequest(c *gin.Context) (feedbackReadRequest, bool) {
	var req feedbackReadRequest
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
