//go:build unit

package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"
)

func TestIsAccountEligibleForSelfCheckUsesSchedulableForModel(t *testing.T) {
	now := time.Now().UTC()
	future := now.Add(time.Hour)
	past := now.Add(-time.Hour)

	tests := []struct {
		name    string
		account *Account
		model   string
		parents map[int64]*Account
		want    bool
	}{
		{
			name: "total quota exhausted",
			account: selfCheckEligibilityAccount(1, PlatformOpenAI, AccountTypeAPIKey, nil, map[string]any{
				"quota_limit": 10.0,
				"quota_used":  10.0,
			}),
		},
		{
			name: "daily quota exhausted",
			account: selfCheckEligibilityAccount(2, PlatformOpenAI, AccountTypeAPIKey, nil, map[string]any{
				"quota_daily_limit": 10.0,
				"quota_daily_used":  10.0,
				"quota_daily_start": now.Add(-time.Hour).Format(time.RFC3339),
			}),
		},
		{
			name: "weekly quota exhausted",
			account: selfCheckEligibilityAccount(3, PlatformOpenAI, AccountTypeAPIKey, nil, map[string]any{
				"quota_weekly_limit": 10.0,
				"quota_weekly_used":  10.0,
				"quota_weekly_start": now.Add(-24 * time.Hour).Format(time.RFC3339),
			}),
		},
		{
			name: "expired active account",
			account: func() *Account {
				account := selfCheckEligibilityAccount(4, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.AutoPauseOnExpired = true
				account.ExpiresAt = &past
				return account
			}(),
		},
		{
			name: "overload window",
			account: func() *Account {
				account := selfCheckEligibilityAccount(5, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.OverloadUntil = &future
				return account
			}(),
		},
		{
			name: "global rate limit window",
			account: func() *Account {
				account := selfCheckEligibilityAccount(6, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.RateLimitResetAt = &future
				return account
			}(),
		},
		{
			name: "temporary unschedulable window",
			account: func() *Account {
				account := selfCheckEligibilityAccount(7, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.TempUnschedulableUntil = &future
				return account
			}(),
		},
		{
			name:  "model a limited",
			model: "model-a",
			account: selfCheckEligibilityAccount(8, PlatformOpenAI, AccountTypeOAuth, nil, map[string]any{
				modelRateLimitsKey: map[string]any{
					"model-a": map[string]any{"rate_limit_reset_at": future.Format(time.RFC3339)},
				},
			}),
		},
		{
			name:  "model b remains eligible",
			model: "model-b",
			account: selfCheckEligibilityAccount(9, PlatformOpenAI, AccountTypeOAuth, nil, map[string]any{
				modelRateLimitsKey: map[string]any{
					"model-a": map[string]any{"rate_limit_reset_at": future.Format(time.RFC3339)},
				},
			}),
			want: true,
		},
		{
			name:  "antigravity model limit uses overages when credits available",
			model: "gemini-3-pro-preview",
			account: selfCheckEligibilityAccount(10, PlatformAntigravity, AccountTypeOAuth, map[string]any{"gemini-3-pro-preview": "gemini-3-pro-preview"}, map[string]any{
				"allow_overages": true,
				modelRateLimitsKey: map[string]any{
					antigravityGeminiModelRateLimitKey: map[string]any{"rate_limit_reset_at": future.Format(time.RFC3339)},
				},
			}),
			want: true,
		},
		{
			name:  "antigravity overages blocked when credits exhausted",
			model: "gemini-3-pro-preview",
			account: selfCheckEligibilityAccount(11, PlatformAntigravity, AccountTypeOAuth, map[string]any{"gemini-3-pro-preview": "gemini-3-pro-preview"}, map[string]any{
				"allow_overages": true,
				modelRateLimitsKey: map[string]any{
					antigravityGeminiModelRateLimitKey: map[string]any{"rate_limit_reset_at": future.Format(time.RFC3339)},
					creditsExhaustedKey: map[string]any{
						"rate_limited_at":     now.Format(time.RFC3339),
						"rate_limit_reset_at": future.Format(time.RFC3339),
					},
				},
			}),
		},
		{
			name: "spark shadow rejects missing parent lookup",
			account: func() *Account {
				parentID := int64(100)
				account := selfCheckEligibilityAccount(12, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.ParentAccountID = &parentID
				account.QuotaDimension = QuotaDimensionSpark
				return account
			}(),
		},
		{
			name: "spark shadow ignores parent global rate limit",
			account: func() *Account {
				parentID := int64(100)
				account := selfCheckEligibilityAccount(13, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.ParentAccountID = &parentID
				account.QuotaDimension = QuotaDimensionSpark
				return account
			}(),
			parents: map[int64]*Account{
				100: func() *Account {
					parent := selfCheckEligibilityAccount(100, PlatformOpenAI, AccountTypeOAuth, nil, nil)
					parent.RateLimitResetAt = &future
					return parent
				}(),
			},
			want: true,
		},
		{
			name: "spark shadow blocks parent credential cooldown",
			account: func() *Account {
				parentID := int64(100)
				account := selfCheckEligibilityAccount(14, PlatformOpenAI, AccountTypeOAuth, nil, nil)
				account.ParentAccountID = &parentID
				account.QuotaDimension = QuotaDimensionSpark
				return account
			}(),
			parents: map[int64]*Account{
				100: func() *Account {
					parent := selfCheckEligibilityAccount(100, PlatformOpenAI, AccountTypeOAuth, nil, nil)
					parent.TempUnschedulableUntil = &future
					return parent
				}(),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			model := tt.model
			if model == "" {
				model = "gpt-4o"
			}
			got := isAccountEligibleForSelfCheck(context.Background(), tt.account, model, func(id int64) *Account {
				return tt.parents[id]
			})
			if got != tt.want {
				t.Fatalf("isAccountEligibleForSelfCheck() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunProbeSkipsWhenAccountBecomesIneligible(t *testing.T) {
	until := time.Now().UTC().Add(time.Hour)
	account := selfCheckEligibilityAccount(21, PlatformOpenAI, AccountTypeOAuth, nil, nil)
	account.TempUnschedulableUntil = &until
	repo := &selfCheckEligibilityHistoryRepo{}
	executor := &selfCheckEligibilityExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&selfCheckEligibilityAccountRepo{accounts: map[int64]*Account{21: account}}, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Model:     "gpt-4o",
		AccountID: 21,
		Platform:  PlatformOpenAI,
	})
	if err != nil {
		t.Fatalf("RunProbe() error = %v", err)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %#v, want none", executor.calls)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created histories = %#v, want none", repo.created)
	}
}

func TestRunProbeSkipsWhenModelNoLongerSupported(t *testing.T) {
	repo := &selfCheckEligibilityHistoryRepo{}
	executor := &selfCheckEligibilityExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&selfCheckEligibilityAccountRepo{
		accounts: map[int64]*Account{
			22: selfCheckEligibilityAccount(22, PlatformOpenAI, AccountTypeOAuth, map[string]any{"other-model": "other-model"}, nil),
		},
	}, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Model:     "gpt-4o",
		AccountID: 22,
		Platform:  PlatformOpenAI,
	})
	if err != nil {
		t.Fatalf("RunProbe() error = %v", err)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %#v, want none", executor.calls)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created histories = %#v, want none", repo.created)
	}
}

func TestRunProbeAccountNotFoundSkipsWithoutHistory(t *testing.T) {
	repo := &selfCheckEligibilityHistoryRepo{}
	executor := &selfCheckEligibilityExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&selfCheckEligibilityAccountRepo{err: ErrAccountNotFound}, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Model:     "gpt-4o",
		AccountID: 23,
		Platform:  PlatformOpenAI,
	})
	if err != nil {
		t.Fatalf("RunProbe() error = %v", err)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %#v, want none", executor.calls)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created histories = %#v, want none", repo.created)
	}
}

func TestRunProbeReadErrorReturnsInternalErrorWithoutHistory(t *testing.T) {
	readErr := errors.New("read failed")
	repo := &selfCheckEligibilityHistoryRepo{}
	executor := &selfCheckEligibilityExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&selfCheckEligibilityAccountRepo{err: readErr}, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Model:     "gpt-4o",
		AccountID: 24,
		Platform:  PlatformOpenAI,
	})
	if !errors.Is(err, readErr) {
		t.Fatalf("RunProbe() error = %v, want wrapped read error", err)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %#v, want none", executor.calls)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created histories = %#v, want none", repo.created)
	}
}

func TestRunProbeUsesFreshParentForSparkShadow(t *testing.T) {
	parentID := int64(100)
	shadow := selfCheckEligibilityAccount(25, PlatformOpenAI, AccountTypeOAuth, map[string]any{"gpt-5.3-codex-spark": "gpt-5.3-codex-spark"}, nil)
	shadow.ParentAccountID = &parentID
	shadow.QuotaDimension = QuotaDimensionSpark
	parent := selfCheckEligibilityAccount(parentID, PlatformOpenAI, AccountTypeOAuth, nil, nil)
	until := time.Now().UTC().Add(time.Hour)
	parent.TempUnschedulableUntil = &until

	repo := &selfCheckEligibilityHistoryRepo{}
	executor := &selfCheckEligibilityExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&selfCheckEligibilityAccountRepo{accounts: map[int64]*Account{
		parentID: parent,
		25:       shadow,
	}}, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Model:     "gpt-5.3-codex-spark",
		AccountID: 25,
		Platform:  PlatformOpenAI,
	})
	if err != nil {
		t.Fatalf("RunProbe() error = %v", err)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %#v, want none", executor.calls)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created histories = %#v, want none", repo.created)
	}
}

func TestRunProbeRecordsRealProbeFailure(t *testing.T) {
	repo := &selfCheckEligibilityHistoryRepo{}
	executor := &selfCheckEligibilityExecutor{result: ModelSelfCheckProbeResult{
		Status:    MonitorStatusFailed,
		ErrorCode: modelSelfCheckErrorUpstream,
	}}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&selfCheckEligibilityAccountRepo{accounts: map[int64]*Account{
		26: selfCheckEligibilityAccount(26, PlatformOpenAI, AccountTypeOAuth, map[string]any{"gpt-4o": "gpt-4o"}, nil),
	}}, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Model:     "gpt-4o",
		AccountID: 26,
		Platform:  PlatformOpenAI,
	})
	if err != nil {
		t.Fatalf("RunProbe() error = %v", err)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("executor calls = %#v, want one", executor.calls)
	}
	if len(repo.created) != 1 {
		t.Fatalf("created histories = %#v, want one", repo.created)
	}
	if repo.created[0].Status != MonitorStatusFailed || repo.created[0].ErrorCode != modelSelfCheckErrorUpstream {
		t.Fatalf("created history = %#v, want real upstream failure", repo.created[0])
	}
}

type selfCheckEligibilityAccountRepo struct {
	accounts map[int64]*Account
	err      error
}

func (r *selfCheckEligibilityAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.err != nil {
		return nil, r.err
	}
	account := r.accounts[id]
	if account == nil {
		return nil, ErrAccountNotFound
	}
	cp := *account
	return &cp, nil
}

func (r *selfCheckEligibilityAccountRepo) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		account := r.accounts[id]
		if account == nil {
			continue
		}
		cp := *account
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type selfCheckEligibilityExecutor struct {
	calls  []ModelSelfCheckProbeTask
	result ModelSelfCheckProbeResult
}

func (e *selfCheckEligibilityExecutor) Probe(ctx context.Context, account *Account, model string) ModelSelfCheckProbeResult {
	e.calls = append(e.calls, ModelSelfCheckProbeTask{
		Key:       modelSelfCheckTaskKey(model, account.ID),
		Model:     model,
		AccountID: account.ID,
		Platform:  account.Platform,
	})
	return e.result
}

type selfCheckEligibilityHistoryRepo struct {
	created []ModelSelfCheckHistory
}

func (r *selfCheckEligibilityHistoryRepo) ListStatusTargets(ctx context.Context) ([]ModelSelfCheckTarget, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListTargetAccounts(ctx context.Context, groupIDs []int64) ([]ModelSelfCheckTargetAccount, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListLatestByModels(ctx context.Context, models []string) ([]ModelSelfCheckHistory, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListHistoriesSince(ctx context.Context, models []string, since time.Time) ([]ModelSelfCheckHistory, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListRecentHistories(ctx context.Context, model string, accountIDs []int64, limit int) ([]ModelSelfCheckHistory, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListRecentHistoriesBefore(ctx context.Context, model string, accountIDs []int64, before time.Time, limit int) ([]ModelSelfCheckHistory, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListRecentStatusSnapshots(ctx context.Context, groupID int64, model string, limit int) ([]ModelSelfCheckStatusSnapshot, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListStatusSnapshotsSince(ctx context.Context, groupID int64, model string, since time.Time) ([]ModelSelfCheckStatusSnapshot, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListStatusSnapshotMetrics(ctx context.Context, targets []ModelSelfCheckTarget, now time.Time) ([]ModelSelfCheckStatusMetrics, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) ListTokenUsageSince(ctx context.Context, since time.Time) ([]ModelSelfCheckTokenUsage, error) {
	return nil, nil
}

func (r *selfCheckEligibilityHistoryRepo) CreateHistory(ctx context.Context, history *ModelSelfCheckHistory) error {
	if history != nil {
		r.created = append(r.created, *history)
	}
	return nil
}

func (r *selfCheckEligibilityHistoryRepo) CreateStatusSnapshot(ctx context.Context, snapshot *ModelSelfCheckStatusSnapshot) error {
	return nil
}

func (r *selfCheckEligibilityHistoryRepo) DeleteStatusSnapshotsBefore(ctx context.Context, before time.Time) (int64, error) {
	return 0, nil
}

func selfCheckEligibilityAccount(id int64, platform, accountType string, modelMapping map[string]any, extra map[string]any) *Account {
	if extra == nil {
		extra = map[string]any{}
	}
	if modelMapping == nil {
		modelMapping = map[string]any{
			"gpt-4o": "gpt-4o",
		}
	}
	return &Account{
		ID:          id,
		Platform:    platform,
		Type:        accountType,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": modelMapping,
		},
		Extra: extra,
	}
}

var _ ModelSelfCheckRepository = (*selfCheckEligibilityHistoryRepo)(nil)
var _ ModelSelfCheckAccountRepository = (*selfCheckEligibilityAccountRepo)(nil)
var _ ModelSelfCheckProbeExecutor = (*selfCheckEligibilityExecutor)(nil)
