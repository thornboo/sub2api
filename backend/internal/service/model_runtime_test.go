package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type modelRuntimeRepoStub struct {
	rows  []ModelRuntimeBucket
	err   error
	calls int
	start time.Time
	end   time.Time
}

func (r *modelRuntimeRepoStub) AggregateModelRuntime(_ context.Context, start, end time.Time) ([]ModelRuntimeBucket, error) {
	r.calls++
	r.start, r.end = start, end
	return r.rows, r.err
}

func TestModelRuntimeWeightedMetrics(t *testing.T) {
	end := time.Date(2026, 9, 9, 1, 23, 0, 0, time.UTC)
	var buckets [24]ModelRuntimeBucket
	buckets[0] = ModelRuntimeBucket{Successes: 1, Failures: 1, FirstTokenSumMS: 100, FirstTokenCount: 1, OutputTokens: 10, GenerationTimeMS: 1000}
	buckets[23] = ModelRuntimeBucket{Successes: 99, Failures: 1, FirstTokenSumMS: 14850, FirstTokenCount: 99, OutputTokens: 1980, GenerationTimeMS: 99000}
	metric := buildModelRuntimeMetrics(BillingModeToken, end, buckets)
	require.InDelta(t, 100.0/102, *metric.SuccessRate, 1e-9)
	require.InDelta(t, 149.5, *metric.AverageLatencyMS, 1e-9)
	require.InDelta(t, 19.9, *metric.ThroughputTokensPerSecond, 1e-9)
	require.Equal(t, "ready", metric.SampleState)
	require.Len(t, metric.Hours, 24)
	require.Equal(t, end.Add(-24*time.Hour), metric.Hours[0].StartedAt)
	require.Equal(t, end.Add(-time.Hour), metric.Hours[23].StartedAt)
	require.Equal(t, int64(2), metric.Hours[0].RequestVolume)
	require.Nil(t, metric.Hours[1].SuccessRate)
}

func TestModelRuntimeMissingSamplesAndNonTokenTiming(t *testing.T) {
	end := time.Now()
	var buckets [24]ModelRuntimeBucket
	empty := buildModelRuntimeMetrics(BillingModeToken, end, buckets)
	require.Equal(t, "empty", empty.SampleState)
	require.Nil(t, empty.SuccessRate)
	require.Nil(t, empty.AverageLatencyMS)
	require.Nil(t, empty.ThroughputTokensPerSecond)
	buckets[0] = ModelRuntimeBucket{Successes: 2, Failures: 1, DurationSumMS: 12000, DurationCount: 2}
	token := buildModelRuntimeMetrics(BillingModeToken, end, buckets)
	require.Equal(t, "low", token.SampleState)
	require.Nil(t, token.AverageLatencyMS, "missing stream timing must not become zero or whole-response duration")
	for _, tt := range []struct {
		mode BillingMode
		kind string
	}{{BillingModeImage, "generation"}, {BillingModePerRequest, "response"}} {
		metric := buildModelRuntimeMetrics(tt.mode, end, buckets)
		require.Equal(t, tt.kind, metric.LatencyKind)
		require.Equal(t, 6000.0, *metric.AverageLatencyMS)
		require.Nil(t, metric.ThroughputTokensPerSecond)
	}
}

func TestModelRuntimeCacheAndGroupIsolation(t *testing.T) {
	key := ModelRuntimeKey{GroupID: 7, Model: "shared-model"}
	private := ModelRuntimeKey{GroupID: 9, Model: "shared-model"}
	repo := &modelRuntimeRepoStub{rows: []ModelRuntimeBucket{
		{ModelRuntimeKey: key, Hour: 23, Successes: 9, Failures: 1},
		{ModelRuntimeKey: private, Hour: 23, Successes: 1, Failures: 9},
	}}
	svc := NewModelRuntimeService(repo)
	now := time.Date(2026, 9, 9, 1, 23, 12, 0, time.UTC)
	svc.now = func() time.Time { return now }
	metrics, err := svc.GetForModels(context.Background(), []ModelRuntimeRequest{{ModelRuntimeKey: key}})
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.InDelta(t, 0.9, *metrics[key].SuccessRate, 1e-9)
	require.Equal(t, 24*time.Hour, repo.end.Sub(repo.start))
	require.Equal(t, now.Truncate(time.Minute), repo.end)
	metrics, err = svc.GetForModels(context.Background(), []ModelRuntimeRequest{{ModelRuntimeKey: private}})
	require.NoError(t, err)
	require.Len(t, metrics, 1)
	require.InDelta(t, 0.1, *metrics[private].SuccessRate, 1e-9)
	require.Equal(t, 1, repo.calls)
	now = now.Add(time.Minute)
	_, err = svc.GetForModels(context.Background(), []ModelRuntimeRequest{{ModelRuntimeKey: key}})
	require.NoError(t, err)
	require.Equal(t, 2, repo.calls)
}

func TestModelRuntimeMergesModelCaseVariants(t *testing.T) {
	repo := &modelRuntimeRepoStub{rows: []ModelRuntimeBucket{
		{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "claude-fable-5"}, Hour: 22,
			Successes: 1, Failures: 1, FirstTokenSumMS: 100, FirstTokenCount: 1,
			DurationSumMS: 2000, DurationCount: 1, OutputTokens: 10, GenerationTimeMS: 1000},
		{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "cLaude-faBlE-5"}, Hour: 22,
			Successes: 3, Failures: 1, FirstTokenSumMS: 900, FirstTokenCount: 3,
			DurationSumMS: 9000, DurationCount: 3, OutputTokens: 90, GenerationTimeMS: 3000},
		{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "CLAUDE-FABLE-5"}, Hour: 23,
			Successes: 6, FirstTokenSumMS: 3000, FirstTokenCount: 6,
			DurationSumMS: 18000, DurationCount: 6, OutputTokens: 600, GenerationTimeMS: 12000},
		{ModelRuntimeKey: ModelRuntimeKey{GroupID: 9, Model: "Claude-Fable-5"}, Hour: 22, Failures: 99},
		{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "claude-fable5"}, Hour: 22, Failures: 99},
		{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "claude-fable-6"}, Hour: 22, Failures: 99},
	}}
	svc := NewModelRuntimeService(repo)
	svc.now = func() time.Time { return time.Date(2026, 9, 9, 1, 23, 12, 0, time.UTC) }
	for _, model := range []string{"claude-fable-5", "Claude-Fable-5", "cLaude-faBlE-5", "CLAUDE-FABLE-5"} {
		t.Run(model, func(t *testing.T) {
			key := ModelRuntimeKey{GroupID: 7, Model: model}
			metrics, err := svc.GetForModels(context.Background(), []ModelRuntimeRequest{{ModelRuntimeKey: key}})
			require.NoError(t, err)
			require.Len(t, metrics, 1)
			metric, ok := metrics[key]
			require.True(t, ok, "the result must retain the catalog's original model spelling")
			require.NotNil(t, metric.SuccessRate)
			require.InDelta(t, 10.0/12, *metric.SuccessRate, 1e-9)
			require.InDelta(t, 400.0, *metric.AverageLatencyMS, 1e-9)
			require.InDelta(t, 43.75, *metric.ThroughputTokensPerSecond, 1e-9)
			require.Equal(t, int64(6), metric.Hours[22].RequestVolume)
			require.InDelta(t, 4.0/6, *metric.Hours[22].SuccessRate, 1e-9)
			require.InDelta(t, 250.0, *metric.Hours[22].AverageLatencyMS, 1e-9)
			require.Equal(t, int64(6), metric.Hours[23].RequestVolume)
			require.Equal(t, 1.0, *metric.Hours[23].SuccessRate)
			require.Zero(t, metric.Hours[21].RequestVolume)
		})
	}

	key := ModelRuntimeKey{GroupID: 7, Model: "Claude-Fable-5"}
	metrics, err := svc.GetForModels(context.Background(), []ModelRuntimeRequest{{ModelRuntimeKey: key, BillingMode: BillingModeImage}})
	require.NoError(t, err)
	require.NotNil(t, metrics[key].AverageLatencyMS)
	require.InDelta(t, 2900.0, *metrics[key].AverageLatencyMS, 1e-9, "whole-response durations must also merge by sample count")
	require.Nil(t, metrics[key].ThroughputTokensPerSecond)

	private := ModelRuntimeKey{GroupID: 9, Model: "claude-fable-5"}
	metrics, err = svc.GetForModels(context.Background(), []ModelRuntimeRequest{{ModelRuntimeKey: private}})
	require.NoError(t, err)
	require.NotNil(t, metrics[private].SuccessRate)
	require.Zero(t, *metrics[private].SuccessRate)
	require.Equal(t, int64(99), metrics[private].Hours[22].RequestVolume)
	require.Equal(t, 1, repo.calls, "case variants must reuse the same cached snapshot")
	require.Equal(t, "cLaude-faBlE-5", repo.rows[1].Model, "source observations must retain their original model spelling")
}

func TestModelRuntimeFailureIsNotEmptySuccess(t *testing.T) {
	repo := &modelRuntimeRepoStub{err: errors.New("database unavailable")}
	svc := NewModelRuntimeService(repo)
	requests := []ModelRuntimeRequest{{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "model"}}}
	metrics, err := svc.GetForModels(context.Background(), requests)
	require.Error(t, err)
	require.Nil(t, metrics)
	repo.err = nil
	metrics, err = svc.GetForModels(context.Background(), requests)
	require.NoError(t, err)
	require.Equal(t, "empty", metrics[requests[0].ModelRuntimeKey].SampleState)
	require.Nil(t, metrics[requests[0].ModelRuntimeKey].SuccessRate)
}

func TestModelRuntimeDisabledFailureRecordingDoesNotPublishSuccessRate(t *testing.T) {
	repo := &modelRuntimeRepoStub{}
	svc := NewModelRuntimeService(repo)
	enabled := false
	svc.failureLogsEnabled = func(context.Context) bool { return enabled }
	requests := []ModelRuntimeRequest{{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "model"}}}
	metrics, err := svc.GetForModels(context.Background(), requests)
	require.Error(t, err)
	require.Nil(t, metrics)
	require.Zero(t, repo.calls)
	enabled = true
	_, err = svc.GetForModels(context.Background(), requests)
	require.NoError(t, err)
	enabled = false
	metrics, err = svc.GetForModels(context.Background(), requests)
	require.Error(t, err, "cached metrics must not bypass disabled failure recording")
	require.Nil(t, metrics)
}

type modelRuntimeRepositoryFunc func(context.Context, time.Time, time.Time) ([]ModelRuntimeBucket, error)

func (f modelRuntimeRepositoryFunc) AggregateModelRuntime(ctx context.Context, start, end time.Time) ([]ModelRuntimeBucket, error) {
	return f(ctx, start, end)
}

func TestModelRuntimeCanceledViewerDoesNotCancelSharedRefresh(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	var calls atomic.Int64
	svc := NewModelRuntimeService(modelRuntimeRepositoryFunc(func(ctx context.Context, _, _ time.Time) ([]ModelRuntimeBucket, error) {
		calls.Add(1)
		close(started)
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))
	requests := []ModelRuntimeRequest{{ModelRuntimeKey: ModelRuntimeKey{GroupID: 7, Model: "model"}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first := make(chan error, 1)
	go func() { _, err := svc.GetForModels(ctx, requests); first <- err }()
	<-started
	cancel()
	require.ErrorIs(t, <-first, context.Canceled)
	second := make(chan error, 1)
	go func() { _, err := svc.GetForModels(context.Background(), requests); second <- err }()
	unblock()
	require.NoError(t, <-second)
	require.Equal(t, int64(1), calls.Load())
}
