package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type roundProbeFunction func(context.Context, *Account, string) ModelSelfCheckProbeResult

func (f roundProbeFunction) Probe(ctx context.Context, a *Account, model string) ModelSelfCheckProbeResult {
	return f(ctx, a, model)
}

func completionRoundFixture() (*ModelSelfCheckService, *modelSelfCheckRoundRepoStub) {
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, Model: "pro", GroupPlatform: PlatformDeepseek}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformDeepseek}, {GroupID: 10, AccountID: 2, Platform: PlatformDeepseek}},
	})
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: namedSelfCheckAccount(1, "C", PlatformDeepseek, 2, nil),
		2: namedSelfCheckAccount(2, "B", PlatformDeepseek, 3, nil),
	}}, nil)
	return svc, repo
}

func TestProbeRoundCompletionFirstSuccessAndAllFailures(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first, second string
		want          string
		attempts      int
	}{
		{"first succeeds", MonitorStatusOperational, MonitorStatusFailed, MonitorStatusOperational, 1},
		{"all fail", MonitorStatusFailed, MonitorStatusFailed, MonitorStatusFailed, 2},
		{"timeout falls back", MonitorStatusFailed, MonitorStatusOperational, MonitorStatusDegraded, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo := completionRoundFixture()
			calls := 0
			svc.probeExecutor = roundProbeFunction(func(ctx context.Context, a *Account, _ string) ModelSelfCheckProbeResult {
				calls++
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				require.LessOrEqual(t, time.Until(deadline), 30*time.Second)
				if a.ID == 1 {
					return ModelSelfCheckProbeResult{Status: tc.first, ErrorCode: func() string {
						if tc.first == MonitorStatusFailed {
							return modelSelfCheckErrorTimeout
						}
						return ""
					}()}
				}
				return ModelSelfCheckProbeResult{Status: tc.second}
			})
			require.NoError(t, svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "pro"}))
			require.Equal(t, tc.attempts, calls)
			final := repo.rounds[0]
			require.Equal(t, tc.want, final.Status)
			require.NotNil(t, final.FinishedAt)
			if tc.attempts == 1 {
				require.Equal(t, modelSelfCheckProbeStepOutcomeNotAttempted, final.Steps[1].Outcome)
				require.Equal(t, modelSelfCheckProbeReasonPriorSuccess, final.Steps[1].ReasonCode)
				require.Nil(t, final.Steps[1].StartedAt)
			}
		})
	}
}

func TestProbeRoundCompletionDeadlineDuringAttemptPreservesRemainingSteps(t *testing.T) {
	svc, repo := completionRoundFixture()
	repo.failCanceledUpdate = true
	calls := 0
	svc.probeExecutor = roundProbeFunction(func(ctx context.Context, _ *Account, _ string) ModelSelfCheckProbeResult {
		calls++
		<-ctx.Done()
		return ModelSelfCheckProbeResult{Status: MonitorStatusFailed, ErrorCode: modelSelfCheckErrorTimeout}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.NoError(t, svc.RunProbeRound(ctx, ModelSelfCheckProbeTask{GroupID: 10, Model: "pro"}))
	require.Equal(t, 1, calls)
	final := repo.rounds[0]
	require.NotNil(t, final.FinishedAt)
	require.Equal(t, UserModelStatusUnknown, final.Status)
	require.Equal(t, modelSelfCheckProbeReasonRoundDeadline, final.ReasonCode)
	require.Equal(t, modelSelfCheckProbeStepOutcomeFailed, final.Steps[0].Outcome)
	require.Equal(t, modelSelfCheckProbeStepOutcomeNotAttempted, final.Steps[1].Outcome)
	require.Equal(t, modelSelfCheckProbeReasonRoundDeadline, final.Steps[1].ReasonCode)
	require.Nil(t, final.Steps[1].StartedAt)
}

type failingRoundHistoryRepository struct {
	*modelSelfCheckRoundRepoStub
	failure error
}

type progressHookRoundRepository struct {
	*modelSelfCheckRoundRepoStub
	beforeProgress func(context.Context)
}

func (r *progressHookRoundRepository) UpdateProbeRound(ctx context.Context, round *ModelSelfCheckProbeRound) error {
	if round.FinishedAt == nil && r.beforeProgress != nil {
		r.beforeProgress(ctx)
	}
	return r.modelSelfCheckRoundRepoStub.UpdateProbeRound(ctx, round)
}

func TestProbeRoundDeadlineDuringProgressDoesNotInventAttempt(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			svc, repo := completionRoundFixture()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			svc.repo = &progressHookRoundRepository{repo, func(progressCtx context.Context) {
				if mode == "cancel" {
					cancel()
				} else {
					<-ctx.Done()
				}
				require.ErrorIs(t, progressCtx.Err(), ctx.Err())
			}}
			executor := &sequencedSelfCheckProbeExecutor{}
			svc.probeExecutor = executor
			require.NoError(t, svc.RunProbeRound(ctx, ModelSelfCheckProbeTask{GroupID: 10, Model: "pro"}))
			require.Empty(t, executor.calls)
			require.Empty(t, repo.created)
			final := repo.rounds[0]
			require.Equal(t, UserModelStatusUnknown, final.Status)
			require.NotNil(t, final.FinishedAt)
			for _, step := range final.Steps {
				require.Equal(t, modelSelfCheckProbeStepOutcomeNotAttempted, step.Outcome)
				require.Equal(t, modelSelfCheckProbeReasonRoundDeadline, step.ReasonCode)
				require.Nil(t, step.StartedAt)
				require.Nil(t, step.FinishedAt)
			}
		})
	}
}

func TestProbeRoundAttemptBudgetStartsAfterProgressWrite(t *testing.T) {
	svc, repo := completionRoundFixture()
	var progressFinished time.Time
	svc.repo = &progressHookRoundRepository{repo, func(context.Context) {
		progressFinished = time.Now()
	}}
	svc.probeExecutor = roundProbeFunction(func(ctx context.Context, _ *Account, _ string) ModelSelfCheckProbeResult {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.False(t, deadline.Before(progressFinished.Add(modelSelfCheckProbeAttemptTimeout)))
		return ModelSelfCheckProbeResult{Status: MonitorStatusOperational}
	})
	require.NoError(t, svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "pro"}))
}

func TestProbeRoundSkipsBackupRemovedFromGroupDuringFirstAttempt(t *testing.T) {
	svc, repo := completionRoundFixture()
	accounts := svc.accountRepo.(*modelSelfCheckAccountRepoStub).accounts
	var calls []int64
	svc.probeExecutor = roundProbeFunction(func(_ context.Context, a *Account, _ string) ModelSelfCheckProbeResult {
		calls = append(calls, a.ID)
		accounts[2].GroupIDs = []int64{20}
		return ModelSelfCheckProbeResult{Status: MonitorStatusFailed}
	})
	require.NoError(t, svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "pro"}))
	require.Equal(t, []int64{1}, calls)
	require.Len(t, repo.created, 1)
	step := repo.rounds[0].Steps[1]
	require.Equal(t, modelSelfCheckProbeStepOutcomeSkipped, step.Outcome)
	require.Equal(t, modelSelfCheckProbeReasonGroupRemoved, step.ReasonCode)
	require.Nil(t, step.StartedAt)
}

func (r *failingRoundHistoryRepository) CreateHistory(context.Context, *ModelSelfCheckHistory) error {
	return r.failure
}

func TestProbeRoundCompletionHistoryFailurePreservesActualOutcome(t *testing.T) {
	svc, repo := completionRoundFixture()
	failure := errors.New("history storage unavailable")
	svc.repo = &failingRoundHistoryRepository{repo, failure}
	svc.probeExecutor = &sequencedSelfCheckProbeExecutor{}
	require.ErrorIs(t, svc.RunProbeRound(context.Background(), ModelSelfCheckProbeTask{GroupID: 10, Model: "pro"}), failure)
	final := repo.rounds[0]
	require.NotNil(t, final.FinishedAt)
	require.Equal(t, UserModelStatusUnknown, final.Status)
	require.Equal(t, modelSelfCheckProbeReasonIncomplete, final.ReasonCode)
	require.Equal(t, modelSelfCheckProbeStepOutcomeSucceeded, final.Steps[0].Outcome)
	require.Nil(t, final.WinnerAccountID)
}
