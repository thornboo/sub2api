package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelSelfCheckStatusExcludesIneligibleAccountWithFreshSuccess(t *testing.T) {
	for _, scenario := range []string{"quota", "missing_account", "missing_repository"} {
		t.Run(scenario, func(t *testing.T) {
			now := time.Now().UTC()
			repo := &modelSelfCheckRepoStub{
				targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
				accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
				latest:   []ModelSelfCheckHistory{{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, CheckedAt: now.Add(-time.Minute)}},
			}
			svc := NewModelSelfCheckService(repo)
			setModelSelfCheckVisibleGroups(svc, 10)
			svc.now = func() time.Time { return now }
			account := activeSelfCheckAccount(1, PlatformOpenAI, nil)
			account.Type = AccountTypeAPIKey
			account.Extra = map[string]any{"quota_limit": 10.0, "quota_used": 10.0}
			svc.accountRepo = &modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{1: account}}
			if scenario == "missing_account" {
				svc.accountRepo = &modelSelfCheckAccountRepoStub{}
			}
			if scenario == "missing_repository" {
				svc.accountRepo = nil
			}
			rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
			require.NoError(t, err)
			require.Equal(t, MonitorStatusFailed, rows[0].Status)
			require.NoError(t, svc.RefreshStatusSnapshots(context.Background()))
			require.Equal(t, modelSelfCheckSnapshotReasonNoAvailableAccount, repo.createdSnapshots[0].ReasonCode)
			require.Zero(t, repo.createdSnapshots[0].EligibleAccountCount)
		})
	}
}

func TestModelSelfCheckListAndDetailKeepSnapshotMetricsWhenAccountsDisappear(t *testing.T) {
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{GroupID: 10, GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		snapshots: []ModelSelfCheckStatusSnapshot{
			{GroupID: 10, Model: "gpt-4o", Status: MonitorStatusFailed, CheckedAt: now.Add(-time.Minute)},
			{GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(500), CheckedAt: now.Add(-time.Hour)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	setModelSelfCheckVisibleGroups(svc, 10)
	svc.now = func() time.Time { return now }
	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	detail, err := svc.GetUserModelStatus(context.Background(), modelSelfCheckTestUserID, 10, "gpt-4o")
	require.NoError(t, err)
	require.Equal(t, MonitorStatusFailed, rows[0].Status)
	assertFloatNear(t, rows[0].Availability24h, 50)
	require.Equal(t, rows[0].Availability24h, detail.Availability24h)
	require.Equal(t, rows[0].Availability7d, detail.Availability7d)
	require.Equal(t, rows[0].Availability30d, detail.Availability30d)
	require.Equal(t, rows[0].AvgLatency24hMs, detail.AvgLatency24hMs)
	require.Zero(t, repo.historyCalls, "snapshot-backed targets must not load raw account history")
	require.Equal(t, 2, repo.metricCalls, "one batched metrics query per request")
}

func TestModelSelfCheckSnapshotMetricsUnavailableDoNotInventLegacyAvailability(t *testing.T) {
	now := time.Now().UTC()
	repo := &modelSelfCheckRepoStub{
		targets:            []ModelSelfCheckTarget{{GroupID: 10, GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts:           []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
		latest:             []ModelSelfCheckHistory{{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, CheckedAt: now.Add(-time.Minute)}},
		history:            []ModelSelfCheckHistory{{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, CheckedAt: now.Add(-time.Minute)}},
		snapshotMetricsErr: errors.New("snapshot metrics unavailable"),
	}
	svc := NewModelSelfCheckService(repo)
	setModelSelfCheckVisibleGroups(svc, 10)
	svc.now = func() time.Time { return now }
	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	require.Equal(t, MonitorStatusOperational, rows[0].Status)
	require.Nil(t, rows[0].Availability24h)
	require.Zero(t, repo.historyCalls)
}

func TestModelSelfCheckOldSnapshotsDoNotFallBackToAccountHistory(t *testing.T) {
	now := time.Now().UTC()
	repo := &modelSelfCheckRepoStub{
		targets:   []ModelSelfCheckTarget{{GroupID: 10, GroupPlatform: PlatformOpenAI, Model: "gpt-4o"}},
		accounts:  []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
		history:   []ModelSelfCheckHistory{{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, CheckedAt: now.Add(-time.Minute)}},
		snapshots: []ModelSelfCheckStatusSnapshot{{GroupID: 10, Model: "gpt-4o", Status: MonitorStatusFailed, CheckedAt: now.AddDate(0, 0, -31)}},
	}
	svc := NewModelSelfCheckService(repo)
	setModelSelfCheckVisibleGroups(svc, 10)
	svc.now = func() time.Time { return now }
	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	require.Nil(t, rows[0].Availability30d)
	require.Zero(t, repo.historyCalls)
}

func TestModelSelfCheckHydratesShadowParentsOutsideCandidateAccounts(t *testing.T) {
	for _, parentState := range []string{"credential_healthy", "credential_cooling", "missing"} {
		t.Run(parentState, func(t *testing.T) {
			now := time.Now().UTC()
			model := "gpt-5.3-codex-spark"
			parentID := int64(99)
			shadow := activeSelfCheckAccount(1, PlatformOpenAI, map[string]any{model: model})
			shadow.ParentAccountID = &parentID
			parent := activeSelfCheckAccount(parentID, PlatformOpenAI, nil)
			parent.Type = AccountTypeOAuth
			parent.Schedulable = false
			until := now.Add(time.Hour)
			parent.RateLimitResetAt = &until
			if parentState == "credential_cooling" {
				parent.TempUnschedulableUntil = &until
			}
			accounts := map[int64]*Account{1: shadow, parentID: parent}
			if parentState == "missing" {
				delete(accounts, parentID)
			}
			repo := &modelSelfCheckRepoStub{
				targets:  []ModelSelfCheckTarget{{GroupID: 10, GroupPlatform: PlatformOpenAI, Model: model}},
				accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}},
				latest:   []ModelSelfCheckHistory{{Model: model, AccountID: 1, Status: MonitorStatusOperational, CheckedAt: now.Add(-time.Minute)}},
			}
			svc := NewModelSelfCheckService(repo)
			svc.SetProbeDependencies(&modelSelfCheckAccountRepoStub{accounts: accounts}, &modelSelfCheckProbeExecutorStub{})
			setModelSelfCheckVisibleGroups(svc, 10)
			svc.now = func() time.Time { return now }
			tasks, err := svc.ListProbeTasks(context.Background())
			require.NoError(t, err)
			rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
			require.NoError(t, err)
			if parentState == "credential_healthy" {
				require.Len(t, tasks, 1)
				require.Equal(t, MonitorStatusOperational, rows[0].Status)
			} else {
				require.Empty(t, tasks)
				require.Equal(t, MonitorStatusFailed, rows[0].Status)
			}
		})
	}
}

func TestModelSelfCheckSnapshotMetricsKeepGroupsSeparateAndFallbackOnlyUncoveredModels(t *testing.T) {
	now := time.Now().UTC()
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{
			{GroupID: 10, GroupPlatform: PlatformOpenAI, Model: "shared"},
			{GroupID: 20, GroupPlatform: PlatformOpenAI, Model: "shared"},
			{GroupID: 20, GroupPlatform: PlatformOpenAI, Model: "legacy"},
		},
		accounts: []ModelSelfCheckTargetAccount{{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI}, {GroupID: 20, AccountID: 1, Platform: PlatformOpenAI}},
		snapshots: []ModelSelfCheckStatusSnapshot{
			{GroupID: 10, Model: "shared", Status: MonitorStatusFailed, CheckedAt: now.Add(-time.Minute)},
			{GroupID: 20, Model: "shared", Status: MonitorStatusOperational, CheckedAt: now.Add(-time.Minute)},
		},
		history: []ModelSelfCheckHistory{{AccountID: 1, Model: "legacy", Status: MonitorStatusDegraded, CheckedAt: now.Add(-time.Minute)}},
	}
	svc := NewModelSelfCheckService(repo)
	setModelSelfCheckVisibleGroups(svc, 10, 20)
	svc.now = func() time.Time { return now }
	rows, err := svc.ListUserModelStatus(context.Background(), modelSelfCheckTestUserID)
	require.NoError(t, err)
	assertFloatNear(t, findModelStatusRow(t, rows, 10, "shared").Availability24h, 0)
	assertFloatNear(t, findModelStatusRow(t, rows, 20, "shared").Availability24h, 100)
	assertFloatNear(t, findModelStatusRow(t, rows, 20, "legacy").Availability24h, 100)
	require.Equal(t, []string{"legacy"}, repo.historyModels)
}
