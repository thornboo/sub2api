package service

import (
	"context"
	"errors"
	"math"
	"sort"
	"testing"
	"time"
)

type modelSelfCheckRepoStub struct {
	targets          []ModelSelfCheckTarget
	accounts         []ModelSelfCheckTargetAccount
	latest           []ModelSelfCheckHistory
	history          []ModelSelfCheckHistory
	timeline         []ModelSelfCheckHistory
	snapshots        []ModelSelfCheckStatusSnapshot
	tokenUsage       []ModelSelfCheckTokenUsage
	tokenUsageSince  time.Time
	created          []ModelSelfCheckHistory
	createdSnapshots []ModelSelfCheckStatusSnapshot
}

func (s *modelSelfCheckRepoStub) ListStatusTargets(ctx context.Context) ([]ModelSelfCheckTarget, error) {
	return append([]ModelSelfCheckTarget(nil), s.targets...), nil
}

func (s *modelSelfCheckRepoStub) ListTargetAccounts(ctx context.Context, groupIDs []int64) ([]ModelSelfCheckTargetAccount, error) {
	allowed := map[int64]struct{}{}
	for _, id := range groupIDs {
		allowed[id] = struct{}{}
	}
	out := []ModelSelfCheckTargetAccount{}
	for _, account := range s.accounts {
		if _, ok := allowed[account.GroupID]; ok {
			out = append(out, account)
		}
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListLatestByModels(ctx context.Context, models []string) ([]ModelSelfCheckHistory, error) {
	allowed := map[string]struct{}{}
	for _, model := range models {
		allowed[model] = struct{}{}
	}
	out := []ModelSelfCheckHistory{}
	for _, row := range s.latest {
		if _, ok := allowed[row.Model]; ok {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListHistoriesSince(ctx context.Context, models []string, since time.Time) ([]ModelSelfCheckHistory, error) {
	allowed := map[string]struct{}{}
	for _, model := range models {
		allowed[model] = struct{}{}
	}
	out := []ModelSelfCheckHistory{}
	for _, row := range s.history {
		if _, ok := allowed[row.Model]; ok && !row.CheckedAt.Before(since) {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListRecentHistories(ctx context.Context, model string, accountIDs []int64, limit int) ([]ModelSelfCheckHistory, error) {
	allowed := map[int64]struct{}{}
	for _, id := range accountIDs {
		allowed[id] = struct{}{}
	}
	out := []ModelSelfCheckHistory{}
	for _, row := range s.timeline {
		if row.Model != model {
			continue
		}
		if _, ok := allowed[row.AccountID]; !ok {
			continue
		}
		out = append(out, row)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListRecentHistoriesBefore(ctx context.Context, model string, accountIDs []int64, before time.Time, limit int) ([]ModelSelfCheckHistory, error) {
	allowed := map[int64]struct{}{}
	for _, id := range accountIDs {
		allowed[id] = struct{}{}
	}
	out := []ModelSelfCheckHistory{}
	for _, row := range s.timeline {
		if row.Model != model {
			continue
		}
		if _, ok := allowed[row.AccountID]; !ok {
			continue
		}
		if !row.CheckedAt.Before(before) {
			continue
		}
		out = append(out, row)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListRecentStatusSnapshots(ctx context.Context, groupID int64, model string, limit int) ([]ModelSelfCheckStatusSnapshot, error) {
	out := []ModelSelfCheckStatusSnapshot{}
	for _, row := range s.snapshots {
		if row.GroupID != groupID || row.Model != model {
			continue
		}
		out = append(out, row)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListStatusSnapshotsSince(ctx context.Context, groupID int64, model string, since time.Time) ([]ModelSelfCheckStatusSnapshot, error) {
	out := []ModelSelfCheckStatusSnapshot{}
	for _, row := range s.snapshots {
		if row.GroupID != groupID || row.Model != model {
			continue
		}
		if row.CheckedAt.Before(since) {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *modelSelfCheckRepoStub) ListTokenUsageSince(ctx context.Context, since time.Time) ([]ModelSelfCheckTokenUsage, error) {
	s.tokenUsageSince = since
	return append([]ModelSelfCheckTokenUsage(nil), s.tokenUsage...), nil
}

func (s *modelSelfCheckRepoStub) CreateHistory(ctx context.Context, history *ModelSelfCheckHistory) error {
	if history != nil {
		s.created = append(s.created, *history)
	}
	return nil
}

func (s *modelSelfCheckRepoStub) CreateStatusSnapshot(ctx context.Context, snapshot *ModelSelfCheckStatusSnapshot) error {
	if snapshot != nil {
		s.createdSnapshots = append(s.createdSnapshots, *snapshot)
	}
	return nil
}

func (s *modelSelfCheckRepoStub) DeleteStatusSnapshotsBefore(ctx context.Context, before time.Time) (int64, error) {
	var kept []ModelSelfCheckStatusSnapshot
	var deleted int64
	for _, row := range s.snapshots {
		if row.CheckedAt.Before(before) {
			deleted++
			continue
		}
		kept = append(kept, row)
	}
	s.snapshots = kept
	return deleted, nil
}

type modelSelfCheckAccountRepoStub struct {
	accounts map[int64]*Account
}

func (s *modelSelfCheckAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	account := s.accounts[id]
	if account == nil {
		return nil, errors.New("account not found")
	}
	cp := *account
	return &cp, nil
}

func (s *modelSelfCheckAccountRepoStub) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		account := s.accounts[id]
		if account == nil {
			continue
		}
		cp := *account
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

type modelSelfCheckProbeExecutorStub struct {
	calls  []ModelSelfCheckProbeTask
	result ModelSelfCheckProbeResult
}

func (s *modelSelfCheckProbeExecutorStub) Probe(ctx context.Context, account *Account, model string) ModelSelfCheckProbeResult {
	s.calls = append(s.calls, ModelSelfCheckProbeTask{
		Key:       modelSelfCheckTaskKey(model, account.ID),
		Model:     model,
		AccountID: account.ID,
		Platform:  account.Platform,
	})
	return s.result
}

func TestListUserModelStatusAggregatesSelfCheckByGroupModel(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: "openai",
			Model:         "gpt-4o",
		}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: "openai"},
			{GroupID: 10, AccountID: 2, Platform: "openai"},
			{GroupID: 10, AccountID: 3, Platform: "anthropic"},
		},
		latest: []ModelSelfCheckHistory{
			{Model: "gpt-4o", AccountID: 1, Platform: "openai", Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(800), CheckedAt: now.Add(-2 * time.Minute)},
			{Model: "gpt-4o", AccountID: 2, Platform: "openai", Status: MonitorStatusFailed, CheckedAt: now.Add(-1 * time.Minute)},
			{Model: "gpt-4o", AccountID: 3, Platform: "anthropic", Status: MonitorStatusFailed, CheckedAt: now.Add(-1 * time.Minute)},
		},
		history: []ModelSelfCheckHistory{
			{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(800), CheckedAt: now.Add(-1 * time.Hour)},
			{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusDegraded, LatencyMs: modelStatusIntPtr(1200), CheckedAt: now.Add(-2 * time.Hour)},
			{Model: "gpt-4o", AccountID: 2, Status: MonitorStatusFailed, CheckedAt: now.Add(-3 * time.Hour)},
			{Model: "gpt-4o", AccountID: 3, Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(50), CheckedAt: now.Add(-1 * time.Hour)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	rows, err := svc.ListUserModelStatus(context.Background())
	if err != nil {
		t.Fatalf("ListUserModelStatus() error = %v", err)
	}

	row := findModelStatusRow(t, rows, 10, "gpt-4o")
	if row.Status != MonitorStatusDegraded {
		t.Fatalf("status = %q, want %q", row.Status, MonitorStatusDegraded)
	}
	if row.MessageCode != userModelMessagePartial {
		t.Fatalf("message = %q, want %q", row.MessageCode, userModelMessagePartial)
	}
	if row.LatestLatencyMs == nil || *row.LatestLatencyMs != 800 {
		t.Fatalf("latest latency = %v, want 800", row.LatestLatencyMs)
	}
	if row.AvgLatency24hMs == nil || *row.AvgLatency24hMs != 1000 {
		t.Fatalf("24h avg latency = %v, want 1000", row.AvgLatency24hMs)
	}
	assertFloatNear(t, row.Availability24h, 66.6666667)
	assertFloatNear(t, row.DegradedRatio24h, 33.3333333)
}

func TestListUserModelStatusIncludesRecentTimeline(t *testing.T) {
	now := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: PlatformOpenAI,
			Model:         "gpt-4o",
		}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI},
		},
		latest: []ModelSelfCheckHistory{
			{Model: "gpt-4o", AccountID: 1, Platform: PlatformOpenAI, Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(500), CheckedAt: now.Add(-30 * time.Second)},
		},
		snapshots: []ModelSelfCheckStatusSnapshot{
			{ID: 2, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, ReasonCode: modelSelfCheckSnapshotReasonOK, LatencyMs: modelStatusIntPtr(500), CheckedAt: now.Add(-1 * time.Minute)},
			{ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusFailed, ReasonCode: modelSelfCheckSnapshotReasonNoAvailableAccount, CheckedAt: now.Add(-2 * time.Minute)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	rows, err := svc.ListUserModelStatus(context.Background())
	if err != nil {
		t.Fatalf("ListUserModelStatus() error = %v", err)
	}

	row := findModelStatusRow(t, rows, 10, "gpt-4o")
	if len(row.Timeline) != 2 {
		t.Fatalf("timeline = %#v, want two snapshot points", row.Timeline)
	}
	if row.Timeline[0].Status != MonitorStatusOperational || row.Timeline[1].Status != MonitorStatusFailed {
		t.Fatalf("timeline statuses = %#v, want newest snapshot order", row.Timeline)
	}
}

func TestListUserModelStatusMarksUnknownWhenLatestIsStale(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: "openai", Model: "new-model"}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: "openai"},
		},
		latest: []ModelSelfCheckHistory{
			{Model: "new-model", AccountID: 1, Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(400), CheckedAt: now.Add(-30 * time.Minute)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	rows, err := svc.ListUserModelStatus(context.Background())
	if err != nil {
		t.Fatalf("ListUserModelStatus() error = %v", err)
	}
	row := findModelStatusRow(t, rows, 10, "new-model")
	if row.Status != UserModelStatusUnknown {
		t.Fatalf("status = %q, want %q", row.Status, UserModelStatusUnknown)
	}
	if row.MessageCode != userModelMessageNoData {
		t.Fatalf("message = %q, want %q", row.MessageCode, userModelMessageNoData)
	}
	if row.LastCheckedAt != nil {
		t.Fatalf("last checked = %v, want nil for stale latest", row.LastCheckedAt)
	}
}

func TestListUserModelStatusMarksFailedWhenNoAccountCanServeGroup(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{GroupID: 10, GroupName: "Pro", GroupPlatform: "openai", Model: "gpt-4o"}},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	rows, err := svc.ListUserModelStatus(context.Background())
	if err != nil {
		t.Fatalf("ListUserModelStatus() error = %v", err)
	}
	row := findModelStatusRow(t, rows, 10, "gpt-4o")
	if row.Status != MonitorStatusFailed {
		t.Fatalf("status = %q, want %q", row.Status, MonitorStatusFailed)
	}
	if row.MessageCode != userModelMessageUnavailable {
		t.Fatalf("message = %q, want %q", row.MessageCode, userModelMessageUnavailable)
	}
}

func TestGetUserModelStatusKeepsSameModelSeparatedByGroup(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{
			{GroupID: 10, GroupName: "Pro", GroupPlatform: "openai", Model: "gpt-4o"},
			{GroupID: 20, GroupName: "Team", GroupPlatform: "openai", Model: "gpt-4o"},
		},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: "openai"},
			{GroupID: 20, AccountID: 2, Platform: "openai"},
		},
		latest: []ModelSelfCheckHistory{
			{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(700), CheckedAt: now.Add(-1 * time.Minute)},
			{Model: "gpt-4o", AccountID: 2, Status: MonitorStatusFailed, CheckedAt: now.Add(-1 * time.Minute)},
		},
		timeline: []ModelSelfCheckHistory{
			{Model: "gpt-4o", AccountID: 2, Status: MonitorStatusFailed, CheckedAt: now.Add(-1 * time.Minute)},
			{Model: "gpt-4o", AccountID: 1, Status: MonitorStatusOperational, CheckedAt: now.Add(-1 * time.Minute)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	detail, err := svc.GetUserModelStatus(context.Background(), 20, "gpt-4o")
	if err != nil {
		t.Fatalf("GetUserModelStatus() error = %v", err)
	}
	if detail.GroupID != 20 {
		t.Fatalf("group_id = %d, want 20", detail.GroupID)
	}
	if detail.Status != MonitorStatusFailed {
		t.Fatalf("status = %q, want %q", detail.Status, MonitorStatusFailed)
	}
	if len(detail.Timeline) != 1 || detail.Timeline[0].Status != MonitorStatusFailed {
		t.Fatalf("timeline = %#v, want only group 20 account result", detail.Timeline)
	}
}

func TestRefreshStatusSnapshotsRecordsNoAvailableAccount(t *testing.T) {
	now := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: PlatformOpenAI,
			Model:         "gpt-4o",
		}},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	if err := svc.RefreshStatusSnapshots(context.Background()); err != nil {
		t.Fatalf("RefreshStatusSnapshots() error = %v", err)
	}
	if len(repo.createdSnapshots) != 1 {
		t.Fatalf("created snapshots = %#v, want one", repo.createdSnapshots)
	}
	got := repo.createdSnapshots[0]
	if got.GroupID != 10 || got.Model != "gpt-4o" {
		t.Fatalf("snapshot target = %#v, want group 10 gpt-4o", got)
	}
	if got.Status != MonitorStatusFailed || got.ReasonCode != modelSelfCheckSnapshotReasonNoAvailableAccount {
		t.Fatalf("snapshot status/reason = %q/%q, want failed/no_available_account", got.Status, got.ReasonCode)
	}
	if got.EligibleAccountCount != 0 || got.CheckedAccountCount != 0 {
		t.Fatalf("snapshot counts = eligible %d checked %d, want 0/0", got.EligibleAccountCount, got.CheckedAccountCount)
	}
	if !got.CheckedAt.Equal(now) {
		t.Fatalf("checked_at = %s, want %s", got.CheckedAt, now)
	}
}

func TestGetUserModelStatusUsesSnapshotsForTimelineAndDetailMetrics(t *testing.T) {
	now := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: PlatformOpenAI,
			Model:         "gpt-4o",
		}},
		snapshots: []ModelSelfCheckStatusSnapshot{
			{ID: 3, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, ReasonCode: modelSelfCheckSnapshotReasonOK, LatencyMs: modelStatusIntPtr(500), CheckedAt: now.Add(-1 * time.Minute)},
			{ID: 2, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusFailed, ReasonCode: modelSelfCheckSnapshotReasonNoAvailableAccount, CheckedAt: now.Add(-30 * time.Minute)},
			{ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, ReasonCode: modelSelfCheckSnapshotReasonOK, LatencyMs: modelStatusIntPtr(700), CheckedAt: now.Add(-2 * time.Hour)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	detail, err := svc.GetUserModelStatus(context.Background(), 10, "gpt-4o")
	if err != nil {
		t.Fatalf("GetUserModelStatus() error = %v", err)
	}
	if detail.Status != MonitorStatusFailed {
		t.Fatalf("current status = %q, want failed from no current account", detail.Status)
	}
	if len(detail.Timeline) != 3 {
		t.Fatalf("timeline = %#v, want three snapshot points", detail.Timeline)
	}
	if detail.Timeline[0].Status != MonitorStatusOperational || detail.Timeline[1].Status != MonitorStatusFailed {
		t.Fatalf("timeline statuses = %#v, want newest snapshot order", detail.Timeline)
	}
	assertFloatNear(t, detail.Availability24h, 66.6666667)
	if detail.AvgLatency24hMs == nil || *detail.AvgLatency24hMs != 600 {
		t.Fatalf("24h avg latency = %v, want 600", detail.AvgLatency24hMs)
	}
	if detail.LatestLatencyMs == nil || *detail.LatestLatencyMs != 500 {
		t.Fatalf("latest latency = %v, want latest snapshot latency 500", detail.LatestLatencyMs)
	}
	if detail.LastCheckedAt == nil || !detail.LastCheckedAt.Equal(now.Add(-1*time.Minute)) {
		t.Fatalf("last checked = %v, want latest snapshot time", detail.LastCheckedAt)
	}
}

func TestGetUserModelStatusSupplementsShortSnapshotTimelineFromLegacyHistory(t *testing.T) {
	now := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: PlatformOpenAI,
			Model:         "gpt-4o",
		}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI},
		},
		latest: []ModelSelfCheckHistory{
			{Model: "gpt-4o", AccountID: 1, Platform: PlatformOpenAI, Status: MonitorStatusOperational, LatencyMs: modelStatusIntPtr(500), CheckedAt: now.Add(-30 * time.Second)},
		},
		snapshots: []ModelSelfCheckStatusSnapshot{
			{ID: 1, GroupID: 10, Model: "gpt-4o", Status: MonitorStatusOperational, ReasonCode: modelSelfCheckSnapshotReasonOK, LatencyMs: modelStatusIntPtr(500), CheckedAt: now.Add(-1 * time.Minute)},
		},
		timeline: []ModelSelfCheckHistory{
			{ID: 4, Model: "gpt-4o", AccountID: 1, Status: MonitorStatusFailed, CheckedAt: now.Add(-30 * time.Second)},
			{ID: 3, Model: "gpt-4o", AccountID: 1, Status: MonitorStatusFailed, CheckedAt: now.Add(-2 * time.Minute)},
			{ID: 2, Model: "gpt-4o", AccountID: 1, Status: MonitorStatusDegraded, LatencyMs: modelStatusIntPtr(800), CheckedAt: now.Add(-3 * time.Minute)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	detail, err := svc.GetUserModelStatus(context.Background(), 10, "gpt-4o")
	if err != nil {
		t.Fatalf("GetUserModelStatus() error = %v", err)
	}
	if len(detail.Timeline) != 3 {
		t.Fatalf("timeline = %#v, want snapshot plus two older legacy points", detail.Timeline)
	}
	if detail.Timeline[0].Status != MonitorStatusOperational ||
		detail.Timeline[1].Status != MonitorStatusFailed ||
		detail.Timeline[2].Status != MonitorStatusDegraded {
		t.Fatalf("timeline statuses = %#v, want snapshot first and older legacy points after it", detail.Timeline)
	}
	if !detail.Timeline[0].CheckedAt.Equal(now.Add(-1 * time.Minute)) {
		t.Fatalf("first timeline checked_at = %s, want snapshot time", detail.Timeline[0].CheckedAt)
	}
	if detail.Timeline[1].CheckedAt.After(detail.Timeline[0].CheckedAt) {
		t.Fatalf("legacy timeline includes overlapping newer history: %#v", detail.Timeline)
	}
	if detail.LastCheckedAt == nil || !detail.LastCheckedAt.Equal(now.Add(-1*time.Minute)) {
		t.Fatalf("last checked = %v, want latest snapshot time", detail.LastCheckedAt)
	}
}

func TestCleanupStatusSnapshotsDeletesRowsPastRetention(t *testing.T) {
	now := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		snapshots: []ModelSelfCheckStatusSnapshot{
			{ID: 1, GroupID: 10, Model: "gpt-4o", CheckedAt: now.AddDate(0, 0, -modelSelfCheckSnapshotRetentionFallback).Add(-time.Minute)},
			{ID: 2, GroupID: 10, Model: "gpt-4o", CheckedAt: now.AddDate(0, 0, -modelSelfCheckSnapshotRetentionFallback).Add(time.Minute)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	deleted, err := svc.CleanupStatusSnapshots(context.Background())
	if err != nil {
		t.Fatalf("CleanupStatusSnapshots() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if len(repo.snapshots) != 1 || repo.snapshots[0].ID != 2 {
		t.Fatalf("remaining snapshots = %#v, want only fresh row", repo.snapshots)
	}
}

func TestCleanupStatusSnapshotsWithRetentionDisabledSkipsDelete(t *testing.T) {
	now := time.Date(2026, 7, 2, 9, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		snapshots: []ModelSelfCheckStatusSnapshot{
			{ID: 1, GroupID: 10, Model: "gpt-4o", CheckedAt: now.AddDate(-1, 0, 0)},
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return now }

	deleted, err := svc.CleanupStatusSnapshotsWithRetention(context.Background(), 0)
	if err != nil {
		t.Fatalf("CleanupStatusSnapshotsWithRetention() error = %v", err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	if len(repo.snapshots) != 1 || repo.snapshots[0].ID != 1 {
		t.Fatalf("snapshots = %#v, want unchanged", repo.snapshots)
	}
}

func TestListProbeTasksDedupesSharedAccountAndFiltersUnsupportedModels(t *testing.T) {
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{
			{GroupID: 10, GroupName: "Pro", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"},
			{GroupID: 20, GroupName: "Team", GroupPlatform: PlatformOpenAI, Model: "gpt-4o"},
		},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI},
			{GroupID: 20, AccountID: 1, Platform: PlatformOpenAI},
			{GroupID: 20, AccountID: 2, Platform: PlatformOpenAI},
			{GroupID: 20, AccountID: 3, Platform: PlatformAnthropic},
		},
	}
	accountRepo := &modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: activeSelfCheckAccount(1, PlatformOpenAI, map[string]any{"gpt-4o": "gpt-4o"}),
		2: activeSelfCheckAccount(2, PlatformOpenAI, map[string]any{"other-model": "other-model"}),
		3: activeSelfCheckAccount(3, PlatformAnthropic, map[string]any{"gpt-4o": "gpt-4o"}),
	}}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(accountRepo, &modelSelfCheckProbeExecutorStub{})

	tasks, err := svc.ListProbeTasks(context.Background())
	if err != nil {
		t.Fatalf("ListProbeTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks = %#v, want one deduped OpenAI account", tasks)
	}
	if tasks[0].Model != "gpt-4o" || tasks[0].AccountID != 1 || tasks[0].Platform != PlatformOpenAI {
		t.Fatalf("task = %#v, want gpt-4o account 1", tasks[0])
	}
}

func TestListProbeTasksFiltersChannelRestrictedModel(t *testing.T) {
	repo := &modelSelfCheckRepoStub{
		targets: []ModelSelfCheckTarget{{
			GroupID:       10,
			GroupName:     "Pro",
			GroupPlatform: PlatformOpenAI,
			Model:         "gpt-4o",
		}},
		accounts: []ModelSelfCheckTargetAccount{
			{GroupID: 10, AccountID: 1, Platform: PlatformOpenAI},
		},
	}
	accountRepo := &modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		1: activeSelfCheckAccount(1, PlatformOpenAI, nil),
	}}
	svc := NewModelSelfCheckService(repo)
	svc.SetProbeDependencies(accountRepo, &gatewayModelSelfCheckProbeExecutor{
		gatewayService: &GatewayService{
			channelService: modelSelfCheckChannelServiceWithRestrictedModel(10, PlatformOpenAI, "allowed-model"),
		},
	})

	tasks, err := svc.ListProbeTasks(context.Background())
	if err != nil {
		t.Fatalf("ListProbeTasks() error = %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("tasks = %#v, want none when channel pricing restricts target model", tasks)
	}
}

func TestRunProbeCallsExecutorAndRecordsHistory(t *testing.T) {
	repo := &modelSelfCheckRepoStub{}
	accountRepo := &modelSelfCheckAccountRepoStub{accounts: map[int64]*Account{
		7: activeSelfCheckAccount(7, PlatformOpenAI, map[string]any{"gpt-4o": "gpt-4o"}),
	}}
	latency := 123
	httpStatus := 200
	executor := &modelSelfCheckProbeExecutorStub{
		result: ModelSelfCheckProbeResult{
			Status:       MonitorStatusOperational,
			LatencyMs:    &latency,
			HTTPStatus:   &httpStatus,
			InputTokens:  12,
			OutputTokens: 1,
		},
	}
	svc := NewModelSelfCheckService(repo)
	svc.now = func() time.Time { return time.Date(2026, 6, 27, 12, 30, 0, 0, time.UTC) }
	svc.SetProbeDependencies(accountRepo, executor)

	err := svc.RunProbe(context.Background(), ModelSelfCheckProbeTask{
		Key:       modelSelfCheckTaskKey("gpt-4o", 7),
		Model:     "gpt-4o",
		AccountID: 7,
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
	got := repo.created[0]
	if got.Model != "gpt-4o" || got.AccountID != 7 || got.Platform != PlatformOpenAI {
		t.Fatalf("history target = %#v, want gpt-4o account 7 openai", got)
	}
	if got.Status != MonitorStatusOperational || got.LatencyMs == nil || *got.LatencyMs != latency {
		t.Fatalf("history result = %#v, want operational latency %d", got, latency)
	}
	if got.HTTPStatus == nil || *got.HTTPStatus != httpStatus {
		t.Fatalf("history http status = %v, want %d", got.HTTPStatus, httpStatus)
	}
	if got.InputTokens != 12 || got.OutputTokens != 1 {
		t.Fatalf("history tokens = %d/%d, want 12/1", got.InputTokens, got.OutputTokens)
	}
}

func TestListTokenUsageSinceCalculatesTotals(t *testing.T) {
	since := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := &modelSelfCheckRepoStub{
		tokenUsage: []ModelSelfCheckTokenUsage{
			{Model: "gpt-4o", InputTokens: 12, OutputTokens: 2},
			{Model: "claude-sonnet", InputTokens: 30, OutputTokens: 1, TotalTokens: 999},
		},
	}
	svc := NewModelSelfCheckService(repo)

	rows, err := svc.ListTokenUsageSince(context.Background(), since)
	if err != nil {
		t.Fatalf("ListTokenUsageSince() error = %v", err)
	}
	if !repo.tokenUsageSince.Equal(since) {
		t.Fatalf("since = %v, want %v", repo.tokenUsageSince, since)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want 2", rows)
	}
	if rows[0].TotalTokens != 14 {
		t.Fatalf("first total = %d, want 14", rows[0].TotalTokens)
	}
	if rows[1].TotalTokens != 31 {
		t.Fatalf("second total = %d, want recomputed 31", rows[1].TotalTokens)
	}
}

func findModelStatusRow(t *testing.T, rows []*UserModelStatusView, groupID int64, model string) *UserModelStatusView {
	t.Helper()
	for _, row := range rows {
		if row.GroupID == groupID && row.Model == model {
			return row
		}
	}
	t.Fatalf("group=%d model %q not found", groupID, model)
	return nil
}

func assertFloatNear(t *testing.T, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("float = nil, want %.4f", want)
	}
	if math.Abs(*got-want) > 0.0001 {
		t.Fatalf("float = %.8f, want %.8f", *got, want)
	}
}

func modelStatusIntPtr(v int) *int {
	return &v
}

func activeSelfCheckAccount(id int64, platform string, modelMapping map[string]any) *Account {
	return &Account{
		ID:          id,
		Platform:    platform,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": modelMapping,
		},
	}
}

func modelSelfCheckChannelServiceWithRestrictedModel(groupID int64, platform, allowedModel string) *ChannelService {
	channel := &Channel{
		ID:                 99,
		Status:             StatusActive,
		GroupIDs:           []int64{groupID},
		RestrictModels:     true,
		BillingModelSource: BillingModelSourceRequested,
	}
	pricing := &ChannelModelPricing{ChannelID: channel.ID, Platform: platform, Models: []string{allowedModel}}
	svc := &ChannelService{}
	svc.cache.Store(&channelCache{
		pricingByGroupModel: map[channelModelKey]*ChannelModelPricing{
			{groupID: groupID, platform: platform, model: allowedModel}: pricing,
		},
		wildcardByGroupPlatform: map[channelGroupPlatformKey][]*wildcardPricingEntry{},
		mappingByGroupModel:     map[channelModelKey]string{},
		wildcardMappingByGP:     map[channelGroupPlatformKey][]*wildcardMappingEntry{},
		channelByGroupID:        map[int64]*Channel{groupID: channel},
		groupPlatform:           map[int64]string{groupID: platform},
		byID:                    map[int64]*Channel{channel.ID: channel},
		loadedAt:                time.Now(),
	})
	return svc
}
