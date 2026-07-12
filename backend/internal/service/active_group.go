package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
)

// ActiveGroupContext is the immutable request-level group decision consumed by
// routing, scheduling, usage, billing, and operational logging.
type ActiveGroupContext struct {
	LogicalRequestID string  `json:"logical_request_id"`
	AttemptID        string  `json:"attempt_id"`
	MemberID         int64   `json:"member_id"`
	MemberVersion    int64   `json:"member_version"`
	GroupID          int64   `json:"group_id"`
	Platform         string  `json:"platform"`
	RateMultiplier   float64 `json:"rate_multiplier"`
	SubscriptionType string  `json:"subscription_type"`
	Endpoint         string  `json:"endpoint"`
	RequestedModel   string  `json:"requested_model"`
	MappedModel      string  `json:"mapped_model"`
	CandidateIndex   int     `json:"candidate_index"`
	AttemptNumber    int     `json:"attempt_number"`
}

func ActiveGroupFromContext(ctx context.Context) (*ActiveGroupContext, bool) {
	if ctx == nil {
		return nil, false
	}
	value, ok := ctx.Value(ctxkey.ActiveGroup).(*ActiveGroupContext)
	return value, ok && value != nil
}
