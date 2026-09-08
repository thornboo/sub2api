package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type ModelRuntimeKey struct {
	GroupID int64
	Model   string
}

type ModelRuntimeRequest struct {
	ModelRuntimeKey
	BillingMode BillingMode
}

// ModelRuntimeBucket contains additive observations, never averages of averages.
// Only terminal user requests are counted; account retry attempts are not requests.
type ModelRuntimeBucket struct {
	ModelRuntimeKey
	Hour             int
	Successes        int64
	Failures         int64
	FirstTokenSumMS  float64
	FirstTokenCount  int64
	DurationSumMS    float64
	DurationCount    int64
	OutputTokens     float64
	GenerationTimeMS float64
}

type ModelRuntimeRepository interface {
	AggregateModelRuntime(context.Context, time.Time, time.Time) ([]ModelRuntimeBucket, error)
}

type ModelRuntimeHour struct {
	StartedAt        time.Time `json:"started_at"`
	SuccessRate      *float64  `json:"success_rate"`
	AverageLatencyMS *float64  `json:"average_latency_ms"`
	RequestVolume    int64     `json:"request_volume"`
}

type ModelRuntimeMetrics struct {
	WindowHours               int                `json:"window_hours"`
	UpdatedAt                 time.Time          `json:"updated_at"`
	SuccessRate               *float64           `json:"success_rate"`
	AverageLatencyMS          *float64           `json:"average_latency_ms"`
	LatencyKind               string             `json:"latency_kind"`
	SampleState               string             `json:"sample_state"`
	ThroughputTokensPerSecond *float64           `json:"throughput_tokens_per_second"`
	Hours                     []ModelRuntimeHour `json:"hours"`
}

type modelRuntimeSnapshot struct {
	end     time.Time
	buckets map[ModelRuntimeKey][24]ModelRuntimeBucket
}

// ModelRuntimeService shares one short-lived aggregate across both catalogs.
// It is independent of monitor settings, probes and scheduling. Handlers must
// supply only authorized catalog keys; no user/account details leave this service.
type ModelRuntimeService struct {
	repo               ModelRuntimeRepository
	now                func() time.Time
	mu                 sync.RWMutex
	cached             *modelRuntimeSnapshot
	refresh            singleflight.Group
	failureLogsEnabled func(context.Context) bool
}

func NewModelRuntimeService(repo ModelRuntimeRepository) *ModelRuntimeService {
	return &ModelRuntimeService{repo: repo, now: time.Now}
}

func (s *ModelRuntimeService) GetForModels(ctx context.Context, requests []ModelRuntimeRequest) (map[ModelRuntimeKey]ModelRuntimeMetrics, error) {
	out := make(map[ModelRuntimeKey]ModelRuntimeMetrics, len(requests))
	if len(requests) == 0 {
		return out, nil
	}
	if s == nil || s.repo == nil {
		return nil, errors.New("model runtime repository unavailable")
	}
	if s.failureLogsEnabled != nil && !s.failureLogsEnabled(ctx) {
		return nil, errors.New("model runtime metrics require request error logging")
	}
	snapshot, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	for _, request := range requests {
		key := request.ModelRuntimeKey
		key.Model = strings.ToLower(key.Model)
		// Only the internal lookup key is normalized; retain the catalog spelling.
		out[request.ModelRuntimeKey] = buildModelRuntimeMetrics(request.BillingMode, snapshot.end, snapshot.buckets[key])
	}
	return out, nil
}

func (s *ModelRuntimeService) freshSnapshot() *modelRuntimeSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cached != nil && s.now().Before(s.cached.end.Add(time.Minute)) {
		return s.cached
	}
	return nil
}

func (s *ModelRuntimeService) snapshot(ctx context.Context) (*modelRuntimeSnapshot, error) {
	if cached := s.freshSnapshot(); cached != nil {
		return cached, nil
	}
	result := s.refresh.DoChan("24h", func() (any, error) {
		if cached := s.freshSnapshot(); cached != nil {
			return cached, nil
		}
		// One disconnected viewer must not cancel the shared refresh for others.
		queryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		end := s.now().UTC().Truncate(time.Minute)
		rows, err := s.repo.AggregateModelRuntime(queryCtx, end.Add(-24*time.Hour), end)
		if err != nil {
			return nil, err
		}
		snapshot := &modelRuntimeSnapshot{end: end, buckets: make(map[ModelRuntimeKey][24]ModelRuntimeBucket)}
		for _, row := range rows {
			if row.Hour < 0 || row.Hour >= 24 {
				continue
			}
			key := row.ModelRuntimeKey
			key.Model = strings.ToLower(key.Model)
			buckets := snapshot.buckets[key]
			// SQL returns original model spellings. Add case variants before
			// calculating ratios and averages, including variants in the same hour.
			bucket := &buckets[row.Hour]
			bucket.Successes += row.Successes
			bucket.Failures += row.Failures
			bucket.FirstTokenSumMS += row.FirstTokenSumMS
			bucket.FirstTokenCount += row.FirstTokenCount
			bucket.DurationSumMS += row.DurationSumMS
			bucket.DurationCount += row.DurationCount
			bucket.OutputTokens += row.OutputTokens
			bucket.GenerationTimeMS += row.GenerationTimeMS
			snapshot.buckets[key] = buckets
		}
		s.mu.Lock()
		s.cached = snapshot
		s.mu.Unlock()
		return snapshot, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-result:
		if result.Err != nil {
			return nil, result.Err
		}
		snapshot, ok := result.Val.(*modelRuntimeSnapshot)
		if !ok || snapshot == nil {
			return nil, errors.New("invalid model runtime snapshot")
		}
		return snapshot, nil
	}
}

func buildModelRuntimeMetrics(billingMode BillingMode, end time.Time, buckets [24]ModelRuntimeBucket) ModelRuntimeMetrics {
	kind := "firstToken"
	if billingMode == BillingModeImage {
		kind = "generation"
	} else if billingMode != "" && billingMode != BillingModeToken {
		kind = "response"
	}
	metric := ModelRuntimeMetrics{WindowHours: 24, UpdatedAt: end, LatencyKind: kind, SampleState: "empty", Hours: make([]ModelRuntimeHour, 24)}
	var successes, requests, latencyCount int64
	var latencySum, outputTokens, generationMS float64
	for i, bucket := range buckets {
		count := bucket.Successes + bucket.Failures
		sum, samples := bucket.FirstTokenSumMS, bucket.FirstTokenCount
		if kind != "firstToken" {
			sum, samples = bucket.DurationSumMS, bucket.DurationCount
		}
		metric.Hours[i] = ModelRuntimeHour{
			StartedAt: end.Add(time.Duration(i-24) * time.Hour), RequestVolume: count,
			SuccessRate:      modelRuntimeRatio(float64(bucket.Successes), float64(count)),
			AverageLatencyMS: modelRuntimeRatio(sum, float64(samples)),
		}
		successes += bucket.Successes
		requests += count
		latencySum += sum
		latencyCount += samples
		outputTokens += bucket.OutputTokens
		generationMS += bucket.GenerationTimeMS
	}
	metric.SuccessRate = modelRuntimeRatio(float64(successes), float64(requests))
	metric.AverageLatencyMS = modelRuntimeRatio(latencySum, float64(latencyCount))
	if kind == "firstToken" {
		metric.ThroughputTokensPerSecond = modelRuntimeRatio(outputTokens*1000, generationMS)
	}
	if requests >= 20 {
		metric.SampleState = "ready"
	} else if requests > 0 {
		metric.SampleState = "low"
	}
	return metric
}

func modelRuntimeRatio(numerator, denominator float64) *float64 {
	if denominator <= 0 {
		return nil
	}
	value := numerator / denominator
	return &value
}
