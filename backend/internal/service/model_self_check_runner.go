package service

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	modelSelfCheckScheduleRefreshInterval = time.Minute
	modelSelfCheckRunOneTimeout           = 90 * time.Second
	modelSelfCheckSnapshotCleanupInterval = 24 * time.Hour
)

type modelSelfCheckRunnerSvc interface {
	ListProbeTasks(ctx context.Context) ([]ModelSelfCheckProbeTask, error)
	RefreshStatusSnapshots(ctx context.Context) error
	CleanupStatusSnapshotsWithRetention(ctx context.Context, retentionDays int) (int64, error)
	RunProbe(ctx context.Context, task ModelSelfCheckProbeTask) error
}

type ModelSelfCheckRunner struct {
	svc            modelSelfCheckRunnerSvc
	settingService *SettingService

	parentCtx    context.Context
	parentCancel context.CancelFunc

	mu      sync.Mutex
	tasks   map[string]*scheduledModelSelfCheck
	wg      sync.WaitGroup
	started bool
	stopped bool

	inFlight   map[string]struct{}
	inFlightMu sync.Mutex

	workerSem           chan struct{}
	lastSnapshotCleanup time.Time
}

type scheduledModelSelfCheck struct {
	task     ModelSelfCheckProbeTask
	interval time.Duration
	jitter   time.Duration
	cancel   context.CancelFunc
}

func NewModelSelfCheckRunner(svc *ModelSelfCheckService, settingService *SettingService) *ModelSelfCheckRunner {
	return newModelSelfCheckRunner(svc, settingService)
}

func newModelSelfCheckRunner(svc modelSelfCheckRunnerSvc, settingService *SettingService) *ModelSelfCheckRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &ModelSelfCheckRunner{
		svc:            svc,
		settingService: settingService,
		parentCtx:      ctx,
		parentCancel:   cancel,
		tasks:          map[string]*scheduledModelSelfCheck{},
		inFlight:       map[string]struct{}{},
	}
}

func (r *ModelSelfCheckRunner) Start() {
	if r == nil || r.svc == nil {
		return
	}
	r.mu.Lock()
	if r.started || r.stopped {
		r.mu.Unlock()
		return
	}
	r.started = true
	r.workerSem = make(chan struct{}, r.runtime(context.Background()).MaxConcurrency)
	r.mu.Unlock()

	r.reloadSchedule(context.Background())

	r.wg.Add(1)
	go r.refreshLoop()
}

func (r *ModelSelfCheckRunner) Stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	r.stopped = true
	r.parentCancel()
	for _, task := range r.tasks {
		task.cancel()
	}
	r.tasks = nil
	r.mu.Unlock()
	r.wg.Wait()
}

func (r *ModelSelfCheckRunner) refreshLoop() {
	defer r.wg.Done()
	timer := time.NewTimer(modelSelfCheckScheduleRefreshInterval)
	defer timer.Stop()
	for {
		select {
		case <-r.parentCtx.Done():
			return
		case <-timer.C:
			r.reloadSchedule(r.parentCtx)
			timer.Reset(modelSelfCheckScheduleRefreshInterval)
		}
	}
}

func (r *ModelSelfCheckRunner) reloadSchedule(ctx context.Context) {
	if r == nil || r.svc == nil {
		return
	}
	runtime := r.runtime(ctx)
	if !runtime.Enabled {
		r.cancelAll()
		return
	}
	loadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	tasks, err := r.svc.ListProbeTasks(loadCtx)
	if err != nil {
		slog.Warn("model_self_check: load probe tasks failed", "error", err)
		return
	}
	if err := r.svc.RefreshStatusSnapshots(loadCtx); err != nil {
		slog.Warn("model_self_check: refresh status snapshots failed", "error", err)
	}
	r.cleanupStatusSnapshotsIfDue(loadCtx, runtime.SnapshotRetentionDays)
	var limited bool
	tasks, limited = limitModelSelfCheckProbeTasks(tasks, runtime.MaxTasksPerRound)
	if limited {
		slog.Warn(
			"model_self_check: probe task limit reached",
			"max_tasks_per_round", runtime.MaxTasksPerRound,
		)
	}
	interval := time.Duration(runtime.DefaultIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = modelSelfCheckIntervalFallback * time.Second
	}
	jitter := interval / 10
	if jitter > 30*time.Second {
		jitter = 30 * time.Second
	}

	desired := make(map[string]ModelSelfCheckProbeTask, len(tasks))
	for _, task := range tasks {
		if task.Key == "" {
			task.Key = modelSelfCheckTaskKey(task.Model, task.AccountID)
		}
		desired[task.Key] = task
	}

	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	if cap(r.workerSem) != runtime.MaxConcurrency {
		r.workerSem = make(chan struct{}, runtime.MaxConcurrency)
	}
	for key, current := range r.tasks {
		if _, ok := desired[key]; !ok || current.interval != interval {
			current.cancel()
			delete(r.tasks, key)
		}
	}
	toStart := make([]*scheduledModelSelfCheck, 0)
	for key, task := range desired {
		if _, ok := r.tasks[key]; ok {
			continue
		}
		taskCtx, taskCancel := context.WithCancel(r.parentCtx)
		scheduled := &scheduledModelSelfCheck{
			task:     task,
			interval: interval,
			jitter:   jitter,
			cancel:   taskCancel,
		}
		r.tasks[key] = scheduled
		toStart = append(toStart, scheduled)
		r.wg.Add(1)
		go r.runScheduled(taskCtx, scheduled)
	}
	r.mu.Unlock()

	if len(toStart) > 0 {
		slog.Info("model_self_check: schedule refreshed", "new_tasks", len(toStart), "total_tasks", len(desired))
	}
}

func (r *ModelSelfCheckRunner) cleanupStatusSnapshotsIfDue(ctx context.Context, retentionDays int) {
	if retentionDays == 0 {
		return
	}
	now := time.Now().UTC()
	r.mu.Lock()
	if !r.lastSnapshotCleanup.IsZero() && now.Sub(r.lastSnapshotCleanup) < modelSelfCheckSnapshotCleanupInterval {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	deleted, err := r.svc.CleanupStatusSnapshotsWithRetention(ctx, retentionDays)
	if err != nil {
		slog.Warn("model_self_check: cleanup status snapshots failed", "error", err)
		return
	}
	r.mu.Lock()
	r.lastSnapshotCleanup = now
	r.mu.Unlock()
	if deleted > 0 {
		slog.Info("model_self_check: cleaned old status snapshots", "deleted", deleted)
	}
}

func (r *ModelSelfCheckRunner) cancelAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, task := range r.tasks {
		task.cancel()
		delete(r.tasks, key)
	}
}

func (r *ModelSelfCheckRunner) runScheduled(ctx context.Context, task *scheduledModelSelfCheck) {
	defer r.wg.Done()

	r.fire(ctx, task)

	timer := time.NewTimer(nextModelSelfCheckDelay(task.interval, task.jitter))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			r.fire(ctx, task)
			timer.Reset(nextModelSelfCheckDelay(task.interval, task.jitter))
		}
	}
}

func nextModelSelfCheckDelay(interval, jitter time.Duration) time.Duration {
	if interval <= 0 {
		interval = modelSelfCheckIntervalFallback * time.Second
	}
	if jitter <= 0 {
		return interval
	}
	offset := time.Duration(rand.Int64N(int64(2*jitter) + 1))
	delay := interval - jitter + offset
	if delay < time.Second {
		return time.Second
	}
	return delay
}

func limitModelSelfCheckProbeTasks(tasks []ModelSelfCheckProbeTask, max int) ([]ModelSelfCheckProbeTask, bool) {
	if max <= 0 {
		max = modelSelfCheckMaxTasksFallback
	}
	if len(tasks) <= max {
		return tasks, false
	}
	return tasks[:max], true
}

func (r *ModelSelfCheckRunner) fire(ctx context.Context, task *scheduledModelSelfCheck) {
	if task == nil {
		return
	}
	if !r.runtime(ctx).Enabled {
		return
	}
	key := task.task.Key
	if key == "" {
		key = modelSelfCheckTaskKey(task.task.Model, task.task.AccountID)
	}
	if !r.tryAcquireInFlight(key) {
		slog.Debug("model_self_check: skip already in-flight", "task", key)
		return
	}
	if !r.tryAcquireWorker() {
		r.releaseInFlight(key)
		slog.Warn("model_self_check: worker limit reached, skip submission", "task", key)
		return
	}
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.releaseWorker()
		defer r.releaseInFlight(key)
		r.runOne(ctx, task.task)
	}()
}

func (r *ModelSelfCheckRunner) runOne(parent context.Context, task ModelSelfCheckProbeTask) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, modelSelfCheckRunOneTimeout)
	defer cancel()
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("model_self_check: runner panic", "task", task.Key, "panic", rec)
		}
	}()
	if err := r.svc.RunProbe(ctx, task); err != nil {
		slog.Warn("model_self_check: probe failed", "task", task.Key, "error", err)
	}
}

func (r *ModelSelfCheckRunner) tryAcquireInFlight(key string) bool {
	r.inFlightMu.Lock()
	defer r.inFlightMu.Unlock()
	if _, ok := r.inFlight[key]; ok {
		return false
	}
	r.inFlight[key] = struct{}{}
	return true
}

func (r *ModelSelfCheckRunner) releaseInFlight(key string) {
	r.inFlightMu.Lock()
	delete(r.inFlight, key)
	r.inFlightMu.Unlock()
}

func (r *ModelSelfCheckRunner) tryAcquireWorker() bool {
	r.mu.Lock()
	sem := r.workerSem
	r.mu.Unlock()
	if sem == nil {
		return false
	}
	select {
	case sem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (r *ModelSelfCheckRunner) releaseWorker() {
	r.mu.Lock()
	sem := r.workerSem
	r.mu.Unlock()
	if sem == nil {
		return
	}
	select {
	case <-sem:
	default:
	}
}

func (r *ModelSelfCheckRunner) runtime(ctx context.Context) ModelSelfCheckRuntime {
	if r == nil || r.settingService == nil {
		return ModelSelfCheckRuntime{
			Enabled:                true,
			DefaultIntervalSeconds: modelSelfCheckIntervalFallback,
			MaxConcurrency:         modelSelfCheckConcurrencyFallback,
			MaxTasksPerRound:       modelSelfCheckMaxTasksFallback,
			SnapshotRetentionDays:  modelSelfCheckSnapshotRetentionFallback,
		}
	}
	runtime := r.settingService.GetModelSelfCheckRuntime(ctx)
	if runtime.DefaultIntervalSeconds <= 0 {
		runtime.DefaultIntervalSeconds = modelSelfCheckIntervalFallback
	}
	if runtime.MaxConcurrency <= 0 {
		runtime.MaxConcurrency = modelSelfCheckConcurrencyFallback
	}
	if runtime.MaxTasksPerRound <= 0 {
		runtime.MaxTasksPerRound = modelSelfCheckMaxTasksFallback
	}
	if runtime.SnapshotRetentionDays < 0 {
		runtime.SnapshotRetentionDays = modelSelfCheckSnapshotRetentionFallback
	}
	return runtime
}
