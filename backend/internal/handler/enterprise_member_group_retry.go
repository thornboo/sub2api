package handler

import (
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// markEnterpriseMemberGroupRetry marks a defensive local capability failure
// replayable only when enforce mode already replaced the legacy candidate list
// with a model-aware plan. Shadow/legacy modes and endpoint families without a
// stable planner must preserve their existing execution behavior.
func markEnterpriseMemberGroupRetry(c *gin.Context, apiKey *service.APIKey, reason service.OpsGroupRetryReason) {
	if c == nil || c.Request == nil || apiKey == nil || apiKey.MemberID == nil {
		return
	}
	if current, ok := middleware2.GetAPIKeyFromContext(c); !ok || current == nil || current.MemberID == nil {
		return
	}
	active, ok := service.ActiveGroupFromContext(c.Request.Context())
	if !ok || !active.ModelPlanApplied || active.RoutePlanMode != service.EnterpriseMemberModelAdmissionEnforcePublished {
		return
	}
	service.MarkOpsGroupRetry(c, reason)
}

func markEnterpriseMemberGroupRetryFromContext(c *gin.Context, reason service.OpsGroupRetryReason) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		return
	}
	markEnterpriseMemberGroupRetry(c, apiKey, reason)
}
