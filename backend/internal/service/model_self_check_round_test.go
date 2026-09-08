package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type modelSelfCheckRoundRepoStub struct {
	*modelSelfCheckRepoStub
	mu                 sync.Mutex
	rounds             []ModelSelfCheckProbeRound
	createdRounds      []ModelSelfCheckProbeRound
	updatedRounds      []ModelSelfCheckProbeRound
	deletedBefore      time.Time
	failCanceledUpdate bool
	failProgressUpdate bool
	createErr          error
}

func newModelSelfCheckRoundRepoStub(base *modelSelfCheckRepoStub) *modelSelfCheckRoundRepoStub {
	if base == nil {
		base = &modelSelfCheckRepoStub{}
	}
	return &modelSelfCheckRoundRepoStub{modelSelfCheckRepoStub: base}
}

func (s *modelSelfCheckRoundRepoStub) CreateProbeRound(ctx context.Context, round *ModelSelfCheckProbeRound) error {
	if s.createErr != nil {
		return s.createErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if round.ID == 0 {
		round.ID = int64(len(s.rounds) + 1)
	}
	cp := cloneModelSelfCheckProbeRound(round)
	s.rounds = append(s.rounds, cp)
	s.createdRounds = append(s.createdRounds, cp)
	return nil
}

func (s *modelSelfCheckRoundRepoStub) UpdateProbeRound(ctx context.Context, round *ModelSelfCheckProbeRound) error {
	if s.failCanceledUpdate && ctx.Err() != nil {
		return ctx.Err()
	}
	if s.failProgressUpdate && round.FinishedAt == nil {
		return errors.New("progress storage unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := cloneModelSelfCheckProbeRound(round)
	s.updatedRounds = append(s.updatedRounds, cp)
	for i := range s.rounds {
		if s.rounds[i].ID == round.ID {
			if s.rounds[i].FinishedAt != nil && round.FinishedAt != nil {
				return errors.New("already finished")
			}
			s.rounds[i] = cp
			return nil
		}
	}
	return errors.New("missing round")
}

func (s *modelSelfCheckRoundRepoStub) ListLatestProbeRounds(ctx context.Context, targets []ModelSelfCheckTarget) ([]ModelSelfCheckProbeRound, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ModelSelfCheckProbeRound, 0, len(targets))
	for _, target := range targets {
		var best *ModelSelfCheckProbeRound
		for i := range s.rounds {
			row := &s.rounds[i]
			if row.GroupID != target.GroupID || row.Model != target.Model {
				continue
			}
			if best == nil || row.StartedAt.After(best.StartedAt) || (row.StartedAt.Equal(best.StartedAt) && row.ID > best.ID) {
				best = row
			}
		}
		if best != nil {
			out = append(out, cloneModelSelfCheckProbeRound(best))
		}
	}
	return out, nil
}

func (s *modelSelfCheckRoundRepoStub) DeleteProbeRoundsBefore(ctx context.Context, before time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletedBefore = before
	var kept []ModelSelfCheckProbeRound
	var deleted int64
	for _, row := range s.rounds {
		if row.StartedAt.Before(before) {
			deleted++
			continue
		}
		kept = append(kept, row)
	}
	s.rounds = kept
	return deleted, nil
}

type sequencedSelfCheckProbeExecutor struct {
	mu       sync.Mutex
	results  map[int64]ModelSelfCheckProbeResult
	calls    []int64
	sessions []string
}

func (e *sequencedSelfCheckProbeExecutor) Probe(ctx context.Context, account *Account, model string) ModelSelfCheckProbeResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls = append(e.calls, account.ID)
	session, _ := ctx.Value(modelSelfCheckSessionKey{}).(string)
	e.sessions = append(e.sessions, session)
	if result, ok := e.results[account.ID]; ok {
		return result
	}
	return ModelSelfCheckProbeResult{Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(10)}
}

func TestRunProbeRoundOrdersByPriorityAndStopsOnFallbackSuccess(t *testing.T) {
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{GroupID: 10, GroupName: "ds", GroupPlatform: PlatformDeepseek, Model: "deepseek-pro"}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: PlatformDeepseek},
			{GroupID: 10, AccountID: 2, Platform: PlatformDeepseek},
			{GroupID: 10, AccountID: 3, Platform: PlatformDeepseek},
		},
	})
	accounts := map[int64]*Account{
		1: namedSelfCheckAccount(1, "A", PlatformDeepseek, 1, map[string]any{"deepseek-flash": "deepseek-flash"}),
		2: namedSelfCheckAccount(2, "B", PlatformDeepseek, 3, map[string]any{"deepseek-flash": "deepseek-flash", "deepseek-pro": "deepseek-pro"}),
		3: namedSelfCheckAccount(3, "C", PlatformDeepseek, 2, map[string]any{"deepseek-flash": "deepseek-flash", "deepseek-pro": "deepseek-pro", "deepseek-exp": "deepseek-exp"}),
	}
	executor := &sequencedSelfCheckProbeExecutor{results: map[int64]ModelSelfCheckProbeResult{
		3: {Status: MonitorStatusFailed, ErrorCode: modelSelfCheckErrorUpstream},
		2: {Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(42)},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: accounts}, executor)

	err := svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "deepseek-pro"})
	require.NoError(t, err)
	require.Equal(t, []int64{3, 2}, executor.calls)
	require.NotEmpty(t, executor.sessions[0])
	require.Equal(t, executor.sessions[0], executor.sessions[1])
	require.GreaterOrEqual(t, len(repo.updatedRounds), 5)
	require.Equal(t, modelSelfCheckProbeStepOutcomePending, repo.updatedRounds[0].Steps[1].Outcome)
	final := repo.updatedRounds[len(repo.updatedRounds)-1]
	require.Equal(t, MonitorStatusDegraded, final.Status)
	require.Equal(t, modelSelfCheckProbeReasonFallbackOK, final.ReasonCode)
	require.NotNil(t, final.WinnerAccountID)
	require.EqualValues(t, 2, *final.WinnerAccountID)
	require.Equal(t, []int64{1, 3, 2}, roundStepAccountIDs(final.Steps))
	require.Equal(t, modelSelfCheckProbeStepOutcomeSkipped, final.Steps[0].Outcome)
	require.Equal(t, modelSelfCheckProbeReasonUnsupported, final.Steps[0].ReasonCode)
	require.Equal(t, modelSelfCheckProbeStepOutcomeFailed, final.Steps[1].Outcome)
	require.Equal(t, modelSelfCheckErrorUpstream, final.Steps[1].ReasonCode)
	require.Equal(t, modelSelfCheckProbeStepOutcomeSucceeded, final.Steps[2].Outcome)
	require.Equal(t, modelSelfCheckProbeReasonOK, final.Steps[2].ReasonCode)
	require.Len(t, repo.created, 2)
}

func TestRunProbeRoundTreats429AsFailureAndContinues(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}, {GroupID: 10, AccountID: 2, Platform: PlatformOpenAI}},
	})
	accounts := map[int64]*Account{
		1: namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil),
		2: namedSelfCheckAccount(2, "backup", PlatformOpenAI, 2, nil),
	}
	httpStatus := 429
	executor := &sequencedSelfCheckProbeExecutor{results: map[int64]ModelSelfCheckProbeResult{
		1: {Status: MonitorStatusDegraded, HTTPStatus: &httpStatus, ErrorCode: modelSelfCheckErrorRateLimit},
		2: {Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(30)},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: accounts}, executor)

	err := svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"})
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, executor.calls)
	final := repo.updatedRounds[len(repo.updatedRounds)-1]
	require.Equal(t, MonitorStatusDegraded, final.Status)
	require.Equal(t, modelSelfCheckErrorRateLimit, final.Steps[0].ReasonCode)
	require.Equal(t, modelSelfCheckProbeStepOutcomeSucceeded, final.Steps[1].Outcome)
}

func TestRunProbeRoundSkipsFreshlyIneligibleAccountBeforeAttempt(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}, {GroupID: 10, AccountID: 2, Platform: PlatformOpenAI}},
	})
	primary := namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil)
	backup := namedSelfCheckAccount(2, "backup", PlatformOpenAI, 2, nil)
	accountRepo := &modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{1: primary, 2: backup}}
	accountRepo.getByID = func(ctx context.Context, id int64, account *Account) (*Account, error) {
		cp := *account
		if id == 1 {
			cp.Schedulable = false
		}
		return &cp, nil
	}
	executor := &sequencedSelfCheckProbeExecutor{results: map[int64]ModelSelfCheckProbeResult{
		2: {Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(30)},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(accountRepo, executor)

	err := svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"})
	require.NoError(t, err)
	require.Equal(t, []int64{2}, executor.calls)
	final := repo.updatedRounds[len(repo.updatedRounds)-1]
	require.Equal(t, MonitorStatusOperational, final.Status)
	require.Equal(t, modelSelfCheckProbeStepOutcomeSkipped, final.Steps[0].Outcome)
	require.Equal(t, modelSelfCheckProbeReasonNotSchedulable, final.Steps[0].ReasonCode)
}

func TestListProbeTasksWithRoundRepoSchedulesTargetsWithoutEligibleAccounts(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{
			{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"},
			{GroupID: 20, GroupName: "Team", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"},
		},
	})
	svc := NewModelSelfCheckService(repo)

	tasks, err := svc.ListProbeTasks(context.Background())
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	require.Equal(t, int64(10), tasks[0].GroupID)
	require.Equal(t, int64(20), tasks[1].GroupID)
}

func TestRunProbeRoundPersistsNoEligibleRound(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
	})
	executor := &sequencedSelfCheckProbeExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{}, executor)

	err := svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"})
	require.NoError(t, err)
	require.Empty(t, executor.calls)
	require.Len(t, repo.createdRounds, 1)
	require.Equal(t, MonitorStatusFailed, repo.createdRounds[0].Status)
	require.Equal(t, modelSelfCheckProbeReasonNoEligible, repo.createdRounds[0].ReasonCode)
}

func TestRunProbeRoundSkipsWhenRoundAlreadyInProgress(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.createErr = ErrModelSelfCheckRoundInProgress
	executor := &sequencedSelfCheckProbeExecutor{}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil),
	}}, executor)

	err := svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"})
	require.NoError(t, err)
	require.Empty(t, executor.calls)
	require.Empty(t, repo.updatedRounds)
}

func TestRunProbeRoundFinalUpdateIgnoresParentCancellation(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.failCanceledUpdate = true
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil),
	}}, &sequencedSelfCheckProbeExecutor{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.RunProbeRound(ctx, ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"})
	require.NoError(t, err)
	final := repo.updatedRounds[len(repo.updatedRounds)-1]
	require.Equal(t, UserModelStatusUnknown, final.Status)
	require.Equal(t, modelSelfCheckProbeReasonRoundDeadline, final.ReasonCode)
	require.NotNil(t, final.FinishedAt)
}

func TestRunProbeRoundIgnoresProgressUpdateFailureAndFinalizes(t *testing.T) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.failProgressUpdate = true
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil),
	}}, &sequencedSelfCheckProbeExecutor{results: map[int64]ModelSelfCheckProbeResult{
		1: {Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(25)},
	}})

	err := svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"})
	require.NoError(t, err)
	require.NotEmpty(t, repo.updatedRounds)
	final := repo.updatedRounds[len(repo.updatedRounds)-1]
	require.Equal(t, MonitorStatusOperational, final.Status)
	require.NotNil(t, final.FinishedAt)
}

func TestSelfCheckCandidateReasonKeepsCanonicalEligibilityForAntigravityCreditLimit(t *testing.T) {
	resetAt := time.Now().Add(time.Hour).UTC()
	account := namedSelfCheckAccount(1, "ag", PlatformAntigravity, 1, nil)
	account.Extra = map[string]any{}
	setAccountModelRateLimitSnapshot(account, creditsExhaustedKey, resetAt, "credits exhausted", time.Now().UTC())
	svc := NewModelSelfCheckService(&modelSelfCheckRepoStub{})

	reason := svc.selfCheckCandidateReason(
		context.Background(),
		ModelSelfCheckTarget{GroupID: 10, GroupPlatform: PlatformAntigravity, Model: "claude-sonnet-4-5"},
		account,
		func(int64) *Account { return nil },
	)
	require.Empty(t, reason)
}

func TestModelStatusUsesFreshRoundAndInvalidatesIneligibleWinner(t *testing.T) {
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	finished := now.Add(-time.Minute)
	winnerID := int64(1)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}, {GroupID: 10, AccountID: 2, Platform: PlatformOpenAI}},
	})
	repo.rounds = []ModelSelfCheckProbeRound{{
		ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, ReasonCode: modelSelfCheckProbeReasonOK,
		WinnerAccountID: &winnerID, StartedAt: now.Add(-2 * time.Minute), FinishedAt: &finished,
		Steps: []ModelSelfCheckProbeStep{{AccountID: 1, Order: 1, Outcome: modelSelfCheckProbeStepOutcomeSucceeded, LatencyMs: modelStatusIntPtr(88)}},
	}}
	accountRepo := &modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "winner", PlatformOpenAI, 1, nil),
		2: namedSelfCheckAccount(2, "backup", PlatformOpenAI, 2, nil),
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	setModelSelfCheckVisibleGroups(svc, 10)
	svc.SetProbeDependencies(accountRepo, &sequencedSelfCheckProbeExecutor{})

	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	row := findModelStatusRow(t, rows, 10, "gpt-4o")
	require.Equal(t, MonitorStatusOperational, row.Status)
	require.NotNil(t, row.LatestLatencyMs)
	require.Equal(t, 88, *row.LatestLatencyMs)

	accountRepo.accounts[1].Schedulable = false
	rows, err = svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	row = findModelStatusRow(t, rows, 10, "gpt-4o")
	require.Equal(t, UserModelStatusUnknown, row.Status)
	require.Nil(t, row.LatestLatencyMs)
}

func TestModelStatusIgnoresUnfinishedRoundEvenIfMarkedOperational(t *testing.T) {
	now := time.Date(2026, 9, 7, 9, 30, 0, 0, time.UTC)
	winnerID := int64(1)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.rounds = []ModelSelfCheckProbeRound{{
		ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, ReasonCode: modelSelfCheckProbeReasonOK,
		WinnerAccountID: &winnerID, StartedAt: now.Add(-time.Minute),
		Steps: []ModelSelfCheckProbeStep{{AccountID: 1, Order: 1, Outcome: modelSelfCheckProbeStepOutcomeSucceeded, LatencyMs: modelStatusIntPtr(88)}},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	setModelSelfCheckVisibleGroups(svc, 10)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "winner", PlatformOpenAI, 1, nil),
	}}, &sequencedSelfCheckProbeExecutor{})

	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	row := findModelStatusRow(t, rows, 10, "gpt-4o")
	require.Equal(t, UserModelStatusUnknown, row.Status)
	require.Equal(t, "checking", row.ReasonCode)
	require.Nil(t, row.LatestLatencyMs)
	require.NoError(t, svc.RefreshStatusSnapshots(context.Background()))
	require.Len(t, repo.createdSnapshots, 1)
	require.Equal(t, UserModelStatusUnknown, repo.createdSnapshots[0].Status)
	require.Nil(t, repo.createdSnapshots[0].LatencyMs)
}

func TestModelStatusReportsStaleReasonForExpiredRound(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	finished := now.Add(-modelSelfCheckFreshWindow).Add(-time.Second)
	winnerID := int64(1)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.rounds = []ModelSelfCheckProbeRound{{
		ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational,
		ReasonCode: modelSelfCheckProbeReasonOK, WinnerAccountID: &winnerID,
		StartedAt: now.Add(-20 * time.Minute), FinishedAt: &finished,
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	setModelSelfCheckVisibleGroups(svc, 10)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil),
	}}, &sequencedSelfCheckProbeExecutor{})

	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	row := findModelStatusRow(t, rows, 10, "gpt-4o")
	require.Equal(t, UserModelStatusUnknown, row.Status)
	require.Equal(t, "stale_probe", row.ReasonCode)
	require.NotNil(t, row.LastCheckedAt)
	require.Equal(t, finished, *row.LastCheckedAt)
}

func TestAdminProbeChainIncludesCandidateReasonsAndLatestRound(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	finished := now.Add(-time.Minute)
	winnerID := int64(2)
	lastChecked := now.Add(-2 * time.Minute)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI},
			{GroupID: 10, AccountID: 2, Platform: PlatformOpenAI},
			{GroupID: 10, AccountID: 3, Platform: PlatformOpenAI},
		},
		latest: []ModelSelfCheckHistory{{Model: "gpt-4o", AccountID: 2, Status: MonitorStatusOperational, CheckedAt: lastChecked}},
	})
	repo.rounds = []ModelSelfCheckProbeRound{{
		ID: 3, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusDegraded, ReasonCode: modelSelfCheckProbeReasonFallbackOK,
		WinnerAccountID: &winnerID, StartedAt: now.Add(-3 * time.Minute), FinishedAt: &finished,
		Steps: []ModelSelfCheckProbeStep{{AccountID: 1, AccountName: "limited", Priority: 1, Order: 1, Outcome: modelSelfCheckProbeStepOutcomeFailed}},
	}}
	future := time.Now().Add(time.Hour)
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "limited", PlatformOpenAI, 1, nil),
		2: namedSelfCheckAccount(2, "backup", PlatformOpenAI, 2, nil),
		3: func() *Account {
			account := namedSelfCheckAccount(3, "cooling", PlatformOpenAI, 3, nil)
			account.RateLimitResetAt = &future
			return account
		}(),
	}}, &sequencedSelfCheckProbeExecutor{})

	view, err := svc.GetAdminProbeChain(context.Background(), 10, "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Len(t, view.Candidates, 3)
	require.Equal(t, []int64{1, 2, 3}, chainCandidateAccountIDs(view.Candidates))
	require.True(t, view.Candidates[0].Eligible)
	require.True(t, view.Candidates[1].Eligible)
	require.False(t, view.Candidates[2].Eligible)
	require.Equal(t, modelSelfCheckProbeReasonRateLimited, view.Candidates[2].ReasonCode)
	require.NotNil(t, view.Candidates[1].LastCheckedAt)
	require.Equal(t, MonitorStatusOperational, view.Candidates[1].LastStatus)
	require.NotNil(t, view.LatestRound)
	require.EqualValues(t, 3, view.LatestRound.ID)
}

func TestAdminProbeChainProjectsExpiredUnfinishedRoundAsUnknown(t *testing.T) {
	now := time.Date(2026, 9, 7, 10, 30, 0, 0, time.UTC)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.rounds = []ModelSelfCheckProbeRound{{
		ID: 4, GroupID: 10, Model: "gpt-4o", Status: modelSelfCheckProbeRoundStatusChecking,
		ReasonCode: modelSelfCheckProbeReasonIncomplete, StartedAt: now.Add(-2 * modelSelfCheckProbeRoundTimeout),
		Steps: []ModelSelfCheckProbeStep{{AccountID: 1, AccountName: "primary", Priority: 1, Order: 1, Outcome: modelSelfCheckProbeStepOutcomePending}},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "primary", PlatformOpenAI, 1, nil),
	}}, &sequencedSelfCheckProbeExecutor{})

	view, err := svc.GetAdminProbeChain(context.Background(), 10, "gpt-4o")
	require.NoError(t, err)
	require.NotNil(t, view.LatestRound)
	require.Equal(t, UserModelStatusUnknown, view.LatestRound.Status)
	require.Equal(t, modelSelfCheckProbeReasonIncomplete, view.LatestRound.ReasonCode)
	require.Nil(t, view.LatestRound.FinishedAt)
	require.Equal(t, modelSelfCheckProbeStepOutcomeIncomplete, view.LatestRound.Steps[0].Outcome)
}

func TestLatestRoundFutureTimestampIsNotFreshEvidence(t *testing.T) {
	now := time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC)
	futureFinished := now.Add(time.Minute)
	winnerID := int64(1)
	round := &ModelSelfCheckProbeRound{
		GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational,
		WinnerAccountID: &winnerID, StartedAt: now.Add(-time.Minute), FinishedAt: &futureFinished,
	}
	got := latestRoundForTarget(ModelSelfCheckTarget{GroupID: 10, Model: "gpt-4o"}, map[modelSelfCheckTargetKey]ModelSelfCheckProbeRound{
		{groupID: 10, model: "gpt-4o"}: *round,
	}, now)
	require.Nil(t, got)
}

func TestRefreshStatusSnapshotUsesRefreshTimeWhenProjectingLatestRound(t *testing.T) {
	now := time.Date(2026, 9, 7, 11, 30, 0, 0, time.UTC)
	finished := now.Add(-time.Minute)
	winnerID := int64(1)
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	repo.rounds = []ModelSelfCheckProbeRound{{
		ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational,
		ReasonCode: modelSelfCheckProbeReasonOK, WinnerAccountID: &winnerID,
		StartedAt: now.Add(-2 * time.Minute), FinishedAt: &finished,
		Steps: []ModelSelfCheckProbeStep{{AccountID: 1, Order: 1, Outcome: modelSelfCheckProbeStepOutcomeSucceeded, LatencyMs: modelStatusIntPtr(88)}},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "winner", PlatformOpenAI, 1, nil),
	}}, &sequencedSelfCheckProbeExecutor{})

	err := svc.RefreshStatusSnapshots(context.Background())
	require.NoError(t, err)
	require.Len(t, repo.createdSnapshots, 1)
	require.Equal(t, now, repo.createdSnapshots[0].CheckedAt)
	require.Equal(t, MonitorStatusOperational, repo.createdSnapshots[0].Status)
	require.NotNil(t, repo.createdSnapshots[0].LatencyMs)
	require.Equal(t, 88, *repo.createdSnapshots[0].LatencyMs)
}

func namedSelfCheckAccount(id int64, name, platform string, priority int, modelMapping map[string]any) *Account {
	account := activeSelfCheckAccount(id, platform, modelMapping)
	account.Name = name
	account.Priority = priority
	account.Type = AccountTypeAPIKey
	account.Concurrency = 1
	account.GroupIDs = []int64{10}
	return account
}

func roundStepAccountIDs(steps []ModelSelfCheckProbeStep) []int64 {
	out := make([]int64, 0, len(steps))
	for _, step := range steps {
		out = append(out, step.AccountID)
	}
	return out
}

func chainCandidateAccountIDs(candidates []ModelSelfCheckChainCandidate) []int64 {
	out := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, candidate.AccountID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
