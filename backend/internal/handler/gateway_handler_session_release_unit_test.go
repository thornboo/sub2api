//go:build unit

package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gatewaySessionReleaseCache struct {
	service.SessionLimitCache

	mu           sync.Mutex
	active       map[int64]map[string]struct{}
	registered   map[int64][]string
	unregistered map[int64][]string
}

func newGatewaySessionReleaseCache() *gatewaySessionReleaseCache {
	return &gatewaySessionReleaseCache{
		active:       make(map[int64]map[string]struct{}),
		registered:   make(map[int64][]string),
		unregistered: make(map[int64][]string),
	}
}

func (c *gatewaySessionReleaseCache) RegisterSession(_ context.Context, accountID int64, sessionUUID string, maxSessions int, _ time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.active[accountID] == nil {
		c.active[accountID] = make(map[string]struct{})
	}
	if _, exists := c.active[accountID][sessionUUID]; exists {
		return true, nil
	}
	if len(c.active[accountID]) >= maxSessions {
		return false, nil
	}
	c.active[accountID][sessionUUID] = struct{}{}
	c.registered[accountID] = append(c.registered[accountID], sessionUUID)
	return true, nil
}

func (c *gatewaySessionReleaseCache) UnregisterSession(_ context.Context, accountID int64, sessionUUID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.active[accountID], sessionUUID)
	c.unregistered[accountID] = append(c.unregistered[accountID], sessionUUID)
	return nil
}

func (c *gatewaySessionReleaseCache) registerNewSessionAllowed(accountID int64, sessionUUID string) bool {
	allowed, err := c.RegisterSession(context.Background(), accountID, sessionUUID, 1, time.Minute)
	return err == nil && allowed
}

func (c *gatewaySessionReleaseCache) unregisterCount(accountID int64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.unregistered[accountID])
}

func (c *gatewaySessionReleaseCache) registeredCount(accountID int64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.registered[accountID])
}

type gatewaySessionReleaseConcurrencyCache struct {
	fakeConcurrencyCache

	accountAcquired       bool
	accountAcquireErr     error
	accountWaitAllowed    bool
	accountWaitAllowedSet bool
	onAcquireAccountSlot  func()
	accountReleases       atomic.Int64
}

func (c *gatewaySessionReleaseConcurrencyCache) AcquireAccountSlot(context.Context, int64, int, string) (bool, error) {
	if c.accountAcquireErr != nil {
		return false, c.accountAcquireErr
	}
	if c.onAcquireAccountSlot != nil {
		c.onAcquireAccountSlot()
	}
	return c.accountAcquired, nil
}

func (c *gatewaySessionReleaseConcurrencyCache) ReleaseAccountSlot(context.Context, int64, string) error {
	c.accountReleases.Add(1)
	return nil
}

func (c *gatewaySessionReleaseConcurrencyCache) IncrementAccountWaitCount(context.Context, int64, int) (bool, error) {
	if c.accountWaitAllowedSet {
		return c.accountWaitAllowed, nil
	}
	return true, nil
}

func newGatewaySessionReleaseHandler(
	t *testing.T,
	group *service.Group,
	account *service.Account,
	sessionCache *gatewaySessionReleaseCache,
	schedulerConcurrency *gatewaySessionReleaseConcurrencyCache,
	handlerConcurrency *gatewaySessionReleaseConcurrencyCache,
) (*GatewayHandler, func()) {
	t.Helper()

	schedulerCache := &fakeSchedulerCache{accounts: []*service.Account{account}}
	schedulerSnapshot := service.NewSchedulerSnapshotService(schedulerCache, nil, nil, nil, nil)
	cfg := &config.Config{
		RunMode: config.RunModeSimple,
		Gateway: config.GatewayConfig{Scheduling: config.GatewaySchedulingConfig{
			FallbackWaitTimeout: 5 * time.Millisecond,
			FallbackMaxWaiting:  1,
			LoadBatchEnabled:    true,
		}},
	}

	gwSvc := service.NewGatewayService(
		nil,                          // accountRepo
		&fakeGroupRepo{group: group}, // groupRepo
		nil,                          // usageLogRepo
		nil,                          // usageBillingRepo
		nil,                          // userRepo
		nil,                          // userSubRepo
		nil,                          // userGroupRateRepo
		nil,                          // cache
		cfg,                          // cfg
		schedulerSnapshot,            // schedulerSnapshot
		service.NewConcurrencyService(schedulerConcurrency), // concurrencyService
		nil,          // billingService
		nil,          // rateLimitService
		nil,          // billingCacheService
		nil,          // identityService
		nil,          // httpUpstream
		nil,          // deferredService
		nil,          // claudeTokenProvider
		sessionCache, // sessionLimitCache
		nil,          // rpmCache
		nil,          // digestStore
		nil,          // settingService
		nil,          // tlsFPProfileService
		nil,          // channelService
		nil,          // resolver
		nil,          // compositeResolver
		nil,          // balanceNotifyService
		nil,          // userPlatformQuotaRepo
	)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)

	return &GatewayHandler{
			gatewayService:            gwSvc,
			billingCacheService:       billingCacheSvc,
			concurrencyHelper:         NewConcurrencyHelper(service.NewConcurrencyService(handlerConcurrency), SSEPingFormatClaude, 0),
			maxAccountSwitches:        1,
			maxAccountSwitchesGemini:  1,
			antigravityGatewayService: nil,
		}, func() {
			billingCacheSvc.Stop()
		}
}

func gatewaySessionReleaseAccount(id int64) *service.Account {
	now := time.Now()
	rate := 0.3
	return &service.Account{
		ID:             id,
		Name:           "anthropic-oauth",
		Platform:       service.PlatformAnthropic,
		Type:           service.AccountTypeOAuth,
		Concurrency:    1,
		Priority:       1,
		Status:         service.StatusActive,
		Schedulable:    true,
		RateMultiplier: &rate,
		Credentials: map[string]any{
			"access_token": "tok_test",
		},
		Extra: map[string]any{
			"max_sessions": 1,
			"upstream_billing_probe": map[string]any{
				"status":      service.UpstreamBillingProbeStatusOK,
				"received_at": now.Add(-time.Minute),
				"fresh_until": now.Add(time.Hour),
				"data": map[string]any{
					"billing_scope":            "token",
					"resolved_rate_multiplier": rate,
				},
			},
		},
	}
}

func runGatewaySessionReleaseMessage(t *testing.T, h *GatewayHandler, group *service.Group, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("POST", "/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.Group, group))
	c.Request = req

	apiKey := &service.APIKey{
		ID:      7101,
		UserID:  8101,
		GroupID: &group.ID,
		Status:  service.StatusActive,
		User:    &service.User{ID: 8101, Concurrency: 10, Balance: 100},
		Group:   group,
	}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: apiKey.UserID, Concurrency: 10})

	h.Messages(c)
	return rec
}

func TestGatewayMessagesReleasesRegisteredSessionOnEarlyReturns(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newGroup := func() *service.Group {
		return &service.Group{ID: 7001, Hydrated: true, Platform: service.PlatformAnthropic, Status: service.StatusActive, ProfitControlEnabled: true, ProfitMinMargin: 0.5, RateMultiplier: 1}
	}
	messageBody := []byte(`{"model":"claude-sonnet-4-5","max_tokens":32,"metadata":{"user_id":"session-a"},"messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}]}`)
	warmupBody := []byte(`{"model":"claude-sonnet-4-5","max_tokens":32,"metadata":{"user_id":"session-a"},"messages":[{"role":"user","content":[{"type":"text","text":"Warmup"}]}]}`)

	t.Run("warmup intercept releases registered session", func(t *testing.T) {
		group := newGroup()
		account := gatewaySessionReleaseAccount(7201)
		account.Credentials["intercept_warmup_requests"] = true
		sessionCache := newGatewaySessionReleaseCache()
		schedulerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: true}
		handlerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: true}
		h, cleanup := newGatewaySessionReleaseHandler(t, group, account, sessionCache, schedulerConcurrency, handlerConcurrency)
		defer cleanup()

		rec := runGatewaySessionReleaseMessage(t, h, group, warmupBody)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, 1, sessionCache.registeredCount(account.ID))
		require.Equal(t, 1, sessionCache.unregisterCount(account.ID))
		require.True(t, sessionCache.registerNewSessionAllowed(account.ID, "session-b"))
	})

	t.Run("wait queue full releases registered session", func(t *testing.T) {
		group := newGroup()
		account := gatewaySessionReleaseAccount(7202)
		sessionCache := newGatewaySessionReleaseCache()
		schedulerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: false}
		handlerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: false, accountWaitAllowed: false, accountWaitAllowedSet: true}
		h, cleanup := newGatewaySessionReleaseHandler(t, group, account, sessionCache, schedulerConcurrency, handlerConcurrency)
		defer cleanup()

		rec := runGatewaySessionReleaseMessage(t, h, group, messageBody)

		require.Equal(t, http.StatusTooManyRequests, rec.Code)
		require.Contains(t, rec.Body.String(), gatewayQueueFullCode)
		require.Equal(t, 1, sessionCache.registeredCount(account.ID))
		require.Equal(t, 1, sessionCache.unregisterCount(account.ID))
		require.True(t, sessionCache.registerNewSessionAllowed(account.ID, "session-b"))
	})

	t.Run("wait acquire failure releases registered session", func(t *testing.T) {
		group := newGroup()
		account := gatewaySessionReleaseAccount(7203)
		sessionCache := newGatewaySessionReleaseCache()
		schedulerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: false}
		handlerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: false, accountAcquireErr: errors.New("redis acquire failed")}
		h, cleanup := newGatewaySessionReleaseHandler(t, group, account, sessionCache, schedulerConcurrency, handlerConcurrency)
		defer cleanup()

		rec := runGatewaySessionReleaseMessage(t, h, group, messageBody)

		require.Equal(t, http.StatusServiceUnavailable, rec.Code)
		require.Contains(t, rec.Body.String(), "Service temporarily unavailable")
		require.Equal(t, 1, sessionCache.registeredCount(account.ID))
		require.Equal(t, 1, sessionCache.unregisterCount(account.ID))
		require.True(t, sessionCache.registerNewSessionAllowed(account.ID, "session-b"))
	})

	t.Run("profit veto releases registered session", func(t *testing.T) {
		group := newGroup()
		account := gatewaySessionReleaseAccount(7204)
		expensiveRate := 0.9
		sessionCache := newGatewaySessionReleaseCache()
		schedulerConcurrency := &gatewaySessionReleaseConcurrencyCache{accountAcquired: false}
		handlerConcurrency := &gatewaySessionReleaseConcurrencyCache{
			accountAcquired: true,
			onAcquireAccountSlot: func() {
				account.RateMultiplier = &expensiveRate
				account.Extra["upstream_billing_probe"].(map[string]any)["data"].(map[string]any)["resolved_rate_multiplier"] = expensiveRate
				account.UpdatedAt = time.Now().Add(time.Second)
			},
		}
		h, cleanup := newGatewaySessionReleaseHandler(t, group, account, sessionCache, schedulerConcurrency, handlerConcurrency)
		defer cleanup()

		rec := runGatewaySessionReleaseMessage(t, h, group, messageBody)

		require.Equal(t, http.StatusBadGateway, rec.Code)
		require.Equal(t, int64(1), handlerConcurrency.accountReleases.Load())
		require.Equal(t, 1, sessionCache.registeredCount(account.ID))
		require.Equal(t, 1, sessionCache.unregisterCount(account.ID))
		require.True(t, sessionCache.registerNewSessionAllowed(account.ID, "session-b"))
	})
}
