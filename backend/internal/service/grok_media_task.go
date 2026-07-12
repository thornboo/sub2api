package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrGrokMediaTaskNotFound = errors.New("grok media task not found")

type GrokMediaTask struct {
	ID                int64
	UpstreamRequestID string
	UserID            int64
	APIKeyID          int64
	MemberID          *int64
	GroupID           int64
	AccountID         int64
}

type GrokMediaTaskRepository interface {
	Create(ctx context.Context, task *GrokMediaTask) error
	GetByRequestID(ctx context.Context, userID int64, memberID *int64, upstreamRequestID string) (*GrokMediaTask, error)
}

type grokMediaTaskRecorderKey struct{}

type GrokMediaTaskRecorder func(ctx context.Context, upstreamRequestID string, accountID int64) error

func WithGrokMediaTaskRecorder(ctx context.Context, recorder GrokMediaTaskRecorder) context.Context {
	if ctx == nil || recorder == nil {
		return ctx
	}
	return context.WithValue(ctx, grokMediaTaskRecorderKey{}, recorder)
}

func recordGrokMediaTask(ctx context.Context, upstreamRequestID string, accountID int64) error {
	upstreamRequestID = strings.TrimSpace(upstreamRequestID)
	if ctx == nil || upstreamRequestID == "" || accountID <= 0 {
		return nil
	}
	recorder, _ := ctx.Value(grokMediaTaskRecorderKey{}).(GrokMediaTaskRecorder)
	if recorder == nil {
		return nil
	}
	return recorder(ctx, upstreamRequestID, accountID)
}

// SelectGrokMediaTaskAccount returns only the account that created an async
// upstream task. Status checks must never fail over to a different account,
// because upstream task IDs are scoped to the original credential tenant.
func (s *OpenAIGatewayService) SelectGrokMediaTaskAccount(ctx context.Context, accountID, groupID int64) (*AccountSelectionResult, error) {
	if s == nil || s.accountRepo == nil || accountID <= 0 || groupID <= 0 {
		return nil, ErrGrokMediaTaskNotFound
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return nil, ErrGrokMediaTaskNotFound
	}
	if account.Platform != PlatformGrok || account.DeletedAt != nil || account.Status != StatusActive || !accountBelongsToGroup(account, groupID) {
		return nil, ErrGrokMediaTaskNotFound
	}
	acquired, err := s.tryAcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil {
		return nil, err
	}
	if acquired != nil && acquired.Acquired {
		return s.newAcquiredSelectionResult(ctx, account, acquired.ReleaseFunc)
	}
	cfg := s.schedulingConfig()
	maxWaiting := cfg.StickySessionMaxWaiting
	if maxWaiting <= 0 {
		maxWaiting = 3
	}
	timeout := cfg.StickySessionWaitTimeout
	if timeout <= 0 {
		timeout = cfg.FallbackWaitTimeout
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("grok media task account %d is busy", accountID)
	}
	return s.newSelectionResult(ctx, account, false, nil, &AccountWaitPlan{
		AccountID:      account.ID,
		MaxConcurrency: account.Concurrency,
		Timeout:        timeout,
		MaxWaiting:     maxWaiting,
	})
}

func accountBelongsToGroup(account *Account, groupID int64) bool {
	if account == nil {
		return false
	}
	for _, candidate := range account.GroupIDs {
		if candidate == groupID {
			return true
		}
	}
	for i := range account.AccountGroups {
		if account.AccountGroups[i].GroupID == groupID {
			return true
		}
	}
	return false
}
