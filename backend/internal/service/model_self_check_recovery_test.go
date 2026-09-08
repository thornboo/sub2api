package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelfCheckTransientRoundsEscalateAndRecover(t *testing.T) {
	now := time.Now().UTC()
	repo := newModelSelfCheckRoundRepoStub(&modelSelfCheckRepoStub{
		targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupName: "OpenAI", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
	})
	executor := &sequencedSelfCheckProbeExecutor{results: map[int64]ModelSelfCheckProbeResult{
		1: {Status: MonitorStatusFailed, ErrorCode: modelSelfCheckErrorUpstream, Transient: true},
	}}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }
	svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{1: namedSelfCheckAccount(1, "A", PlatformOpenAI, 1, nil)}}, executor)
	task := ModelSelfCheckProbeTask{GroupID: 10, Model: "gpt-4o"}
	require.NoError(t, svc.RunProbeRound(context.Background(), task))
	require.Equal(t, MonitorStatusDegraded, repo.rounds[len(repo.rounds)-1].Status)
	require.Equal(t, "transient_probe_failed", repo.rounds[len(repo.rounds)-1].ReasonCode)
	now = now.Add(time.Minute)
	require.NoError(t, svc.RunProbeRound(context.Background(), task))
	require.Equal(t, MonitorStatusFailed, repo.rounds[len(repo.rounds)-1].Status)
	require.NotEmpty(t, executor.sessions[0])
	require.NotEqual(t, executor.sessions[0], executor.sessions[1])
	now = now.Add(time.Minute)
	executor.results[1] = ModelSelfCheckProbeResult{Status: MonitorStatusDegraded, Recovered: true, ErrorCode: "retry_succeeded", RetryCount: 1, InitialHTTPStatus: modelStatusIntPtr(503)}
	require.NoError(t, svc.RunProbeRound(context.Background(), task))
	final := repo.rounds[len(repo.rounds)-1]
	require.Equal(t, MonitorStatusDegraded, final.Status)
	require.Equal(t, "retry_succeeded", final.ReasonCode)
	require.EqualValues(t, 1, *final.WinnerAccountID)
	require.Equal(t, modelSelfCheckProbeStepOutcomeSucceeded, final.Steps[0].Outcome)
	require.Equal(t, 1, final.Steps[0].RetryCount)
	require.Equal(t, 503, *final.Steps[0].InitialHTTPStatus)
}
