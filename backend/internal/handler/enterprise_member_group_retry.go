package handler

import (
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// markEnterpriseMemberGroupRetry records a closed, group-local failure for the
// outer enterprise-member orchestrator. Admission mode only decides how the
// candidate list is built; it must never disable the safe recovery contract
// that legacy and shadow requests already depended on. The orchestrator still
// rejects replay after a committed response, an ambiguous outcome, a committed
// external task, a committed WebSocket turn, or client cancellation.
func markEnterpriseMemberGroupRetry(c *gin.Context, apiKey *service.APIKey, reason service.OpsGroupRetryReason) {
	if c == nil || c.Request == nil || apiKey == nil || apiKey.MemberID == nil {
		return
	}
	if current, ok := middleware2.GetAPIKeyFromContext(c); !ok || current == nil || current.MemberID == nil {
		return
	}
	if _, ok := service.ActiveGroupFromContext(c.Request.Context()); !ok {
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
