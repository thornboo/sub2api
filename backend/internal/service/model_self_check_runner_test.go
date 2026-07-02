package service

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type modelSelfCheckRunnerSvcStub struct {
	tasks                []ModelSelfCheckProbeTask
	runCount             atomic.Int64
	snapshotRefreshCount atomic.Int64
	cleanupCount         atomic.Int64
	lastCleanupRetention atomic.Int64
	cleanupErr           error
	runCalled            chan ModelSelfCheckProbeTask
}

type modelSelfCheckRunnerSettingRepoStub struct {
	values map[string]string
}

func (s *modelSelfCheckRunnerSettingRepoStub) Get(ctx context.Context, key string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *modelSelfCheckRunnerSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *modelSelfCheckRunnerSettingRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *modelSelfCheckRunnerSettingRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (s *modelSelfCheckRunnerSettingRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *modelSelfCheckRunnerSettingRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *modelSelfCheckRunnerSettingRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func (s *modelSelfCheckRunnerSvcStub) ListProbeTasks(ctx context.Context) ([]ModelSelfCheckProbeTask, error) {
	return append([]ModelSelfCheckProbeTask(nil), s.tasks...), nil
}

func (s *modelSelfCheckRunnerSvcStub) RefreshStatusSnapshots(ctx context.Context) error {
	s.snapshotRefreshCount.Add(1)
	return nil
}

func (s *modelSelfCheckRunnerSvcStub) CleanupStatusSnapshotsWithRetention(ctx context.Context, retentionDays int) (int64, error) {
	s.cleanupCount.Add(1)
	s.lastCleanupRetention.Store(int64(retentionDays))
	if s.cleanupErr != nil {
		return 0, s.cleanupErr
	}
	return 0, nil
}

func (s *modelSelfCheckRunnerSvcStub) RunProbe(ctx context.Context, task ModelSelfCheckProbeTask) error {
	s.runCount.Add(1)
	if s.runCalled != nil {
		select {
		case s.runCalled <- task:
		default:
		}
	}
	return nil
}

func TestModelSelfCheckRunnerStartLoadsTasksAndRunsProbe(t *testing.T) {
	svc := &modelSelfCheckRunnerSvcStub{
		tasks: []ModelSelfCheckProbeTask{{
			Key:       modelSelfCheckTaskKey("gpt-4o", 7),
			Model:     "gpt-4o",
			AccountID: 7,
			Platform:  PlatformOpenAI,
		}},
		runCalled: make(chan ModelSelfCheckProbeTask, 1),
	}
	r := newModelSelfCheckRunner(svc, nil)
	r.Start()
	defer r.Stop()

	select {
	case task := <-svc.runCalled:
		if task.Model != "gpt-4o" || task.AccountID != 7 {
			t.Fatalf("run task = %#v, want gpt-4o account 7", task)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("model self-check runner did not trigger RunProbe")
	}
	if got := modelSelfCheckRunnerTaskCount(r); got != 1 {
		t.Fatalf("scheduled tasks = %d, want 1", got)
	}
	if got := svc.snapshotRefreshCount.Load(); got == 0 {
		t.Fatal("snapshot refresh count = 0, want runner refresh to persist status snapshots")
	}
	if got := svc.cleanupCount.Load(); got == 0 {
		t.Fatal("cleanup count = 0, want runner refresh to cleanup status snapshots")
	}
}

func TestModelSelfCheckRunnerInFlightAcquireRelease(t *testing.T) {
	r := newModelSelfCheckRunner(&modelSelfCheckRunnerSvcStub{}, nil)
	if !r.tryAcquireInFlight("gpt-4o:7") {
		t.Fatal("first acquire should succeed")
	}
	if r.tryAcquireInFlight("gpt-4o:7") {
		t.Fatal("second acquire without release must fail")
	}
	r.releaseInFlight("gpt-4o:7")
	if !r.tryAcquireInFlight("gpt-4o:7") {
		t.Fatal("acquire after release should succeed")
	}
	r.releaseInFlight("gpt-4o:7")
}

func TestLimitModelSelfCheckProbeTasks(t *testing.T) {
	tasks := []ModelSelfCheckProbeTask{
		{Key: "gpt-4o:1", Model: "gpt-4o", AccountID: 1},
		{Key: "gpt-4o:2", Model: "gpt-4o", AccountID: 2},
		{Key: "gpt-4o:3", Model: "gpt-4o", AccountID: 3},
	}

	limited, truncated := limitModelSelfCheckProbeTasks(tasks, 2)
	if !truncated {
		t.Fatal("expected task list to be truncated")
	}
	if len(limited) != 2 {
		t.Fatalf("limited tasks = %d, want 2", len(limited))
	}
	if limited[0].AccountID != 1 || limited[1].AccountID != 2 {
		t.Fatalf("limited tasks preserve order = %#v", limited)
	}

	notLimited, truncated := limitModelSelfCheckProbeTasks(tasks, 0)
	if truncated {
		t.Fatal("fallback limit should not truncate this small task list")
	}
	if len(notLimited) != len(tasks) {
		t.Fatalf("fallback-limited tasks = %d, want %d", len(notLimited), len(tasks))
	}
}

func TestModelSelfCheckRunnerSnapshotCleanupRunsOncePerInterval(t *testing.T) {
	svc := &modelSelfCheckRunnerSvcStub{}
	r := newModelSelfCheckRunner(svc, nil)

	r.reloadSchedule(context.Background())
	r.reloadSchedule(context.Background())
	if got := svc.cleanupCount.Load(); got != 1 {
		t.Fatalf("cleanup count after immediate reloads = %d, want 1", got)
	}

	r.mu.Lock()
	r.lastSnapshotCleanup = time.Now().UTC().Add(-modelSelfCheckSnapshotCleanupInterval - time.Second)
	r.mu.Unlock()

	r.reloadSchedule(context.Background())
	if got := svc.cleanupCount.Load(); got != 2 {
		t.Fatalf("cleanup count after interval elapsed = %d, want 2", got)
	}
}

func TestModelSelfCheckRunnerSnapshotCleanupUsesConfiguredRetention(t *testing.T) {
	svc := &modelSelfCheckRunnerSvcStub{}
	settings := NewSettingService(&modelSelfCheckRunnerSettingRepoStub{values: map[string]string{
		SettingKeyModelSelfCheckSnapshotRetentionDays: "180",
	}}, &config.Config{})
	r := newModelSelfCheckRunner(svc, settings)

	r.reloadSchedule(context.Background())
	if got := svc.cleanupCount.Load(); got != 1 {
		t.Fatalf("cleanup count = %d, want 1", got)
	}
	if got := svc.lastCleanupRetention.Load(); got != 180 {
		t.Fatalf("cleanup retention = %d, want 180", got)
	}
}

func TestModelSelfCheckRunnerSnapshotCleanupDisabledSkipsCleanup(t *testing.T) {
	svc := &modelSelfCheckRunnerSvcStub{}
	settings := NewSettingService(&modelSelfCheckRunnerSettingRepoStub{values: map[string]string{
		SettingKeyModelSelfCheckSnapshotRetentionDays: "0",
	}}, &config.Config{})
	r := newModelSelfCheckRunner(svc, settings)

	r.reloadSchedule(context.Background())
	if got := svc.cleanupCount.Load(); got != 0 {
		t.Fatalf("cleanup count = %d, want 0 when retention cleanup is disabled", got)
	}
}

func TestModelSelfCheckRunnerSnapshotCleanupRetriesAfterFailure(t *testing.T) {
	svc := &modelSelfCheckRunnerSvcStub{cleanupErr: errors.New("cleanup failed")}
	r := newModelSelfCheckRunner(svc, nil)

	r.reloadSchedule(context.Background())
	if got := svc.cleanupCount.Load(); got != 1 {
		t.Fatalf("cleanup count after failed reload = %d, want 1", got)
	}

	svc.cleanupErr = nil
	r.reloadSchedule(context.Background())
	if got := svc.cleanupCount.Load(); got != 2 {
		t.Fatalf("cleanup count after retry reload = %d, want 2", got)
	}
}

func modelSelfCheckRunnerTaskCount(r *ModelSelfCheckRunner) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}
