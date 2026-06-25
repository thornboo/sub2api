//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

// modelRateLimitRepoStub 记录 SetModelRateLimit 调用，用于断言闸门是否真正打限流标记。
type modelRateLimitRepoStub struct {
	mockAccountRepoForGemini
	setCalls  int
	lastScope string
	lastReset time.Time
}

func (r *modelRateLimitRepoStub) SetModelRateLimit(_ context.Context, _ int64, scope string, resetAt time.Time, _ ...string) error {
	r.setCalls++
	r.lastScope = scope
	r.lastReset = resetAt
	return nil
}

// fakeModelFailCounter 是内存版 ModelFailCounterCache，用于控制失败计数。
type fakeModelFailCounter struct {
	counts         map[string]int64
	resetCalls     int
	lastResetScope string
}

func (f *fakeModelFailCounter) IncrementModelFailCount(_ context.Context, _ int64, scope string, _ int) (int64, error) {
	if f.counts == nil {
		f.counts = map[string]int64{}
	}
	f.counts[scope]++
	return f.counts[scope], nil
}

func (f *fakeModelFailCounter) ResetModelFailCount(_ context.Context, _ int64, scope string) error {
	f.resetCalls++
	f.lastResetScope = scope
	return nil
}

func newModelRateLimitService(accountRepo AccountRepository, settings *ModelRateLimitSettings, counter ModelFailCounterCache) *RateLimitService {
	settingRepo := newMockSettingRepo()
	if settings != nil {
		data, _ := json.Marshal(*settings)
		settingRepo.data[SettingKeyModelRateLimitSettings] = string(data)
	}
	svc := NewRateLimitService(accountRepo, nil, &config.Config{}, nil, nil)
	svc.SetSettingService(NewSettingService(settingRepo, &config.Config{}))
	if counter != nil {
		svc.SetModelFailCounterCache(counter)
	}
	return svc
}

func openAI429Body() []byte {
	return []byte(`{"error":{"type":"rate_limit_error","message":"Rate limit reached for gpt-5 in org"}}`)
}

func openAITestAccount() *Account {
	return &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
}

// 默认 Enabled=false 时保持历史行为：首次失败即限流。
func TestHandleOpenAIModelRateLimit_DefaultTripsImmediately(t *testing.T) {
	repo := &modelRateLimitRepoStub{}
	svc := newModelRateLimitService(repo, nil, &fakeModelFailCounter{})

	handled := svc.HandleOpenAIModelRateLimit(context.Background(), openAITestAccount(), "gpt-5", http.StatusTooManyRequests, http.Header{}, openAI429Body())
	require.True(t, handled)
	require.Equal(t, 1, repo.setCalls)
	require.Equal(t, "gpt-5", repo.lastScope)
}

// Enabled=true 且阈值=3 时，前两次不限流，第三次才限流。
func TestHandleOpenAIModelRateLimit_ThresholdDelaysTrip(t *testing.T) {
	repo := &modelRateLimitRepoStub{}
	settings := &ModelRateLimitSettings{Enabled: true, FailureThreshold: 3, WindowMinutes: 5, CooldownSeconds: 30}
	svc := newModelRateLimitService(repo, settings, &fakeModelFailCounter{})
	account := openAITestAccount()

	for i := 1; i <= 2; i++ {
		handled := svc.HandleOpenAIModelRateLimit(context.Background(), account, "gpt-5", http.StatusTooManyRequests, http.Header{}, openAI429Body())
		require.True(t, handled, "call %d should be handled (failover) but not rate-limited", i)
		require.Equal(t, 0, repo.setCalls, "call %d must not set rate limit yet", i)
	}

	handled := svc.HandleOpenAIModelRateLimit(context.Background(), account, "gpt-5", http.StatusTooManyRequests, http.Header{}, openAI429Body())
	require.True(t, handled)
	require.Equal(t, 1, repo.setCalls, "third failure should trip the model rate limit")
	// 冷却覆盖：无上游 reset 时应使用配置的 30 秒。
	require.WithinDuration(t, time.Now().Add(30*time.Second), repo.lastReset, 3*time.Second)
}

// Enabled=true 但未注入计数器时，降级为首次即限流。
func TestHandleOpenAIModelRateLimit_NilCounterTripsImmediately(t *testing.T) {
	repo := &modelRateLimitRepoStub{}
	settings := &ModelRateLimitSettings{Enabled: true, FailureThreshold: 5, WindowMinutes: 5, CooldownSeconds: 30}
	svc := newModelRateLimitService(repo, settings, nil)

	handled := svc.HandleOpenAIModelRateLimit(context.Background(), openAITestAccount(), "gpt-5", http.StatusTooManyRequests, http.Header{}, openAI429Body())
	require.True(t, handled)
	require.Equal(t, 1, repo.setCalls)
}

func TestModelRateLimitResetAtWithOverride(t *testing.T) {
	now := time.Now()

	// 无 header/body reset 时，override 作为回退生效。
	got := openAIModelRateLimitResetAtWithOverride(http.Header{}, []byte("{}"), 30*time.Second)
	require.WithinDuration(t, now.Add(30*time.Second), got, 2*time.Second)

	// override<=0 时回退到硬编码默认（1 分钟）。
	gotDefault := openAIModelRateLimitResetAtWithOverride(http.Header{}, []byte("{}"), 0)
	require.WithinDuration(t, now.Add(openAIModelRateLimitDefaultCooldown), gotDefault, 2*time.Second)

	// 上游 Retry-After 优先于 override。
	h := http.Header{}
	h.Set("Retry-After", "120")
	gotHeader := openAIModelRateLimitResetAtWithOverride(h, nil, 30*time.Second)
	require.WithinDuration(t, now.Add(120*time.Second), gotHeader, 5*time.Second)
}

func TestGetModelRateLimitSettings_DefaultsWhenNotSet(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	settings, err := svc.GetModelRateLimitSettings(context.Background())
	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, 1, settings.FailureThreshold)
	require.Equal(t, 5, settings.WindowMinutes)
	require.Equal(t, 60, settings.CooldownSeconds)
}

func TestGetModelRateLimitSettings_ClampOnRead(t *testing.T) {
	repo := newMockSettingRepo()
	data, _ := json.Marshal(ModelRateLimitSettings{Enabled: true, FailureThreshold: 999, WindowMinutes: 99999, CooldownSeconds: 99999})
	repo.data[SettingKeyModelRateLimitSettings] = string(data)
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetModelRateLimitSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 100, settings.FailureThreshold)
	require.Equal(t, 1440, settings.WindowMinutes)
	require.Equal(t, 7200, settings.CooldownSeconds)
}

func TestSetModelRateLimitSettings_EnabledRejectsOutOfRange(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	ctx := context.Background()

	require.Error(t, svc.SetModelRateLimitSettings(ctx, &ModelRateLimitSettings{Enabled: true, FailureThreshold: 0, WindowMinutes: 5, CooldownSeconds: 60}))
	require.Error(t, svc.SetModelRateLimitSettings(ctx, &ModelRateLimitSettings{Enabled: true, FailureThreshold: 101, WindowMinutes: 5, CooldownSeconds: 60}))
	require.Error(t, svc.SetModelRateLimitSettings(ctx, &ModelRateLimitSettings{Enabled: true, FailureThreshold: 3, WindowMinutes: 0, CooldownSeconds: 60}))
	require.Error(t, svc.SetModelRateLimitSettings(ctx, &ModelRateLimitSettings{Enabled: true, FailureThreshold: 3, WindowMinutes: 5, CooldownSeconds: 0}))

	// 合法值通过。
	require.NoError(t, svc.SetModelRateLimitSettings(ctx, &ModelRateLimitSettings{Enabled: true, FailureThreshold: 3, WindowMinutes: 10, CooldownSeconds: 120}))
}

func TestRateLimitService_ClearModelRateLimit(t *testing.T) {
	repo := &rateLimitClearRepoStub{}
	counter := &fakeModelFailCounter{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	svc.SetModelFailCounterCache(counter)
	ctx := context.Background()

	require.NoError(t, svc.ClearModelRateLimit(ctx, 7, "gpt-5"))
	require.Equal(t, 1, repo.clearSingleModelCalls)
	require.Equal(t, "gpt-5", repo.clearSingleModelScope)
	require.Equal(t, 1, counter.resetCalls)
	require.Equal(t, "gpt-5", counter.lastResetScope)

	// 空 scope 应报错且不调用仓储。
	require.Error(t, svc.ClearModelRateLimit(ctx, 7, "   "))
	require.Equal(t, 1, repo.clearSingleModelCalls)
}
