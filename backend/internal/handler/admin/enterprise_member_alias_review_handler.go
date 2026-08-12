package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ListEnterpriseMemberModelAliases returns a bounded admin-only shadow alias
// migration list. It omits API keys, member identity, credentials, request
// bodies and upstream response payloads.
func (h *OpsHandler) ListEnterpriseMemberModelAliases(c *gin.Context) {
	if h.aliasReviewService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Enterprise member alias review service not available")
		return
	}
	limit := 100
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			response.BadRequest(c, "Invalid limit")
			return
		}
		limit = parsed
	}
	items, err := h.aliasReviewService.ListLegacySuccessNewPruned(c.Request.Context(), limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to list enterprise member model aliases")
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *OpsHandler) GetEnterpriseMemberModelAliasReadiness(c *gin.Context) {
	if h.aliasReviewService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Enterprise member alias review service not available")
		return
	}
	summary, err := h.aliasReviewService.GetReadinessSummary(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to get enterprise member model alias readiness")
		return
	}
	response.Success(c, summary)
}

// ReviewEnterpriseMemberModelAlias writes administrator disposition. Review
// state is an audit/readiness signal only; the planner never reads it.
func (h *OpsHandler) ReviewEnterpriseMemberModelAlias(c *gin.Context) {
	if h.aliasReviewService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Enterprise member alias review service not available")
		return
	}
	var req service.EnterpriseMemberAliasReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok || subject.UserID <= 0 {
		response.Error(c, http.StatusUnauthorized, "Unauthorized")
		return
	}
	record, err := h.aliasReviewService.Review(c.Request.Context(), req, subject.UserID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, record)
}
