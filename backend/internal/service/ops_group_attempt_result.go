package service

import (
	"context"

	"github.com/gin-gonic/gin"
)

type GroupAttemptOutcome string

const (
	GroupAttemptOutcomeTerminalFailure GroupAttemptOutcome = "terminal_failure"
)

type GroupAttemptUnsafeReason string

const (
	GroupAttemptUnsafeReasonInvalidReason      GroupAttemptUnsafeReason = "invalid_reason"
	GroupAttemptUnsafeReasonResponseCommitted  GroupAttemptUnsafeReason = "response_committed"
	GroupAttemptUnsafeReasonOutcomeUnknown     GroupAttemptUnsafeReason = "outcome_unknown"
	GroupAttemptUnsafeReasonExternalTask       GroupAttemptUnsafeReason = "external_task_committed"
	GroupAttemptUnsafeReasonWSTurnCommitted    GroupAttemptUnsafeReason = "ws_turn_committed"
	GroupAttemptUnsafeReasonClientCancelled    GroupAttemptUnsafeReason = "client_cancelled"
	GroupAttemptUnsafeReasonMissingActiveGroup GroupAttemptUnsafeReason = "missing_active_group"
)

// GroupAttemptResult is the replay contract for one enterprise-member group
// attempt. HTTP status codes are intentionally absent from this contract:
// handlers must opt in with one of the closed reasons before the orchestrator
// may consider another group.
type GroupAttemptResult struct {
	GroupID           int64                    `json:"group_id"`
	AttemptNumber     int                      `json:"attempt_number"`
	Outcome           GroupAttemptOutcome      `json:"outcome"`
	Reason            OpsGroupRetryReason      `json:"reason"`
	SafeToReplay      bool                     `json:"safe_to_replay"`
	ResponseCommitted bool                     `json:"response_committed"`
	OutcomeUnknown    bool                     `json:"outcome_unknown"`
	UnsafeReason      GroupAttemptUnsafeReason `json:"unsafe_reason,omitempty"`
}

func NewGroupAttemptResultFromContext(c *gin.Context, reason OpsGroupRetryReason) GroupAttemptResult {
	result := GroupAttemptResult{
		Outcome:      GroupAttemptOutcomeTerminalFailure,
		Reason:       reason,
		SafeToReplay: reason.Valid(),
	}
	if c == nil {
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonMissingActiveGroup
		return result
	}
	if c.Request != nil {
		if active, ok := ActiveGroupFromContext(c.Request.Context()); ok {
			result.GroupID = active.GroupID
			result.AttemptNumber = active.AttemptNumber
		}
	}
	result = finalizeGroupAttemptReplayDecision(c, result, IsResponseCommitted(c))
	return result
}

func MarkGroupAttemptResult(c *gin.Context, result GroupAttemptResult) {
	if c == nil {
		return
	}
	c.Set(OpsGroupAttemptResultKey, result)
	if result.Reason.Valid() {
		c.Set(OpsGroupRetryReasonKey, result.Reason)
	}
}

func GroupAttemptResultFromContext(c *gin.Context) (GroupAttemptResult, bool) {
	if c == nil {
		return GroupAttemptResult{}, false
	}
	if value, ok := c.Get(OpsGroupAttemptResultKey); ok {
		switch result := value.(type) {
		case GroupAttemptResult:
			return result, true
		case *GroupAttemptResult:
			if result != nil {
				return *result, true
			}
		}
	}
	if reason, ok := OpsGroupRetryReasonFromContext(c); ok {
		result := NewGroupAttemptResultFromContext(c, reason)
		return result, result.Valid()
	}
	return GroupAttemptResult{}, false
}

func GroupAttemptResultForReplay(c *gin.Context, responseCommitted bool) (GroupAttemptResult, bool) {
	result, ok := GroupAttemptResultFromContext(c)
	if !ok {
		return GroupAttemptResult{}, false
	}
	result = finalizeGroupAttemptReplayDecision(c, result, responseCommitted || result.ResponseCommitted)
	return result, result.SafeToReplay && result.Valid()
}

func (o GroupAttemptOutcome) Valid() bool {
	return o == GroupAttemptOutcomeTerminalFailure
}

func (r GroupAttemptResult) Valid() bool {
	return r.Outcome.Valid() && r.Reason.Valid()
}

func MarkEnterpriseMemberExternalTaskCommitted(c *gin.Context) {
	if c != nil {
		c.Set(EnterpriseMemberExternalTaskCommittedKey, true)
	}
}

func HasEnterpriseMemberExternalTaskCommitted(c *gin.Context) bool {
	return ginBool(c, EnterpriseMemberExternalTaskCommittedKey)
}

func MarkEnterpriseMemberWSTurnCommitted(c *gin.Context) {
	if c != nil {
		c.Set(EnterpriseMemberWSTurnCommittedKey, true)
	}
}

func HasEnterpriseMemberWSTurnCommitted(c *gin.Context) bool {
	return ginBool(c, EnterpriseMemberWSTurnCommittedKey)
}

func MarkOpsGroupAttemptOutcomeUnknown(c *gin.Context) {
	if c == nil {
		return
	}
	if result, ok := GroupAttemptResultFromContext(c); ok {
		result.OutcomeUnknown = true
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonOutcomeUnknown
		c.Set(OpsGroupAttemptResultKey, result)
		return
	}
	c.Set(EnterpriseMemberBudgetOutcomeAmbiguousKey, true)
}

func finalizeGroupAttemptReplayDecision(c *gin.Context, result GroupAttemptResult, responseCommitted bool) GroupAttemptResult {
	if !result.Reason.Valid() {
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonInvalidReason
		return result
	}
	if result.GroupID <= 0 || result.AttemptNumber <= 0 {
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonMissingActiveGroup
		return result
	}
	if c == nil {
		return result
	}
	if responseCommitted || IsResponseCommitted(c) {
		result.ResponseCommitted = true
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonResponseCommitted
		return result
	}
	if result.OutcomeUnknown || IsEnterpriseMemberBudgetOutcomeAmbiguous(c) {
		result.OutcomeUnknown = true
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonOutcomeUnknown
		return result
	}
	if HasEnterpriseMemberExternalTaskCommitted(c) {
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonExternalTask
		return result
	}
	if HasEnterpriseMemberWSTurnCommitted(c) {
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonWSTurnCommitted
		return result
	}
	if c.Request != nil && contextErr(c.Request.Context()) != nil {
		result.SafeToReplay = false
		result.UnsafeReason = GroupAttemptUnsafeReasonClientCancelled
		return result
	}
	return result
}

func ginBool(c *gin.Context, key string) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(key)
	if !ok {
		return false
	}
	marked, _ := value.(bool)
	return marked
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
