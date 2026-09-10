//go:build integration

package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	svc "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type upstreamSupplierBalanceAdmin interface {
	CreateUpstreamSupplier(context.Context, svc.CreateUpstreamSupplierInput) (*svc.UpstreamSupplier, error)
	UpdateUpstreamSupplier(context.Context, svc.UpdateUpstreamSupplierInput) (*svc.UpstreamSupplier, error)
	ListUpstreamSuppliers(context.Context) ([]svc.UpstreamSupplier, error)
	RefreshUpstreamSupplierBalance(context.Context, int64) (*svc.UpstreamSupplier, error)
}

type supplierBalanceProvider struct {
	server *httptest.Server

	mu          sync.Mutex
	statusCalls int
	selfCalls   int
	lastAuth    string
	lastUser    string
	quota       string
	failSelf    bool
}

func newSupplierBalanceProvider(t *testing.T, quota string) *supplierBalanceProvider {
	t.Helper()
	provider := &supplierBalanceProvider{quota: quota}
	provider.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider.mu.Lock()
		defer provider.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/panel/api/status":
			provider.statusCalls++
			require.Empty(t, r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":250000}}`))
		case "/panel/api/user/self":
			provider.selfCalls++
			provider.lastAuth = r.Header.Get("Authorization")
			provider.lastUser = r.Header.Get("New-Api-User")
			if provider.failSelf {
				http.Error(w, `{"success":false,"message":"denied"}`, http.StatusUnauthorized)
				return
			}
			_, _ = fmt.Fprintf(w, `{"success":true,"data":{"quota":%s,"used_quota":999999}}`, provider.quota)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(provider.server.Close)
	return provider
}

func (p *supplierBalanceProvider) baseURL() string {
	return p.server.URL + "/panel/"
}

func (p *supplierBalanceProvider) setQuota(quota string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quota = quota
}

func (p *supplierBalanceProvider) setFailSelf(fail bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failSelf = fail
}

func (p *supplierBalanceProvider) requireRequests(t *testing.T, wantStatus, wantSelf int, wantAuth, wantUser string) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	require.Equal(t, wantStatus, p.statusCalls)
	require.Equal(t, wantSelf, p.selfCalls)
	require.Equal(t, wantAuth, p.lastAuth)
	require.Equal(t, wantUser, p.lastUser)
}

func (p *supplierBalanceProvider) requestCounts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.statusCalls, p.selfCalls
}

func TestUpstreamSupplierBalanceRefreshStoresAccountWalletFromNewAPI(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	provider := newSupplierBalanceProvider(t, "1250000")
	supplier := createSupplierWithBalanceConfig(t, admin, provider, "plain-access-token")

	refreshed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)

	require.NoError(t, err)
	require.NotNil(t, refreshed.BalanceSnapshot)
	require.Equal(t, "ok", refreshed.BalanceSnapshot.Status)
	require.NotNil(t, refreshed.BalanceSnapshot.BalanceUSD)
	require.Equal(t, float64(5), *refreshed.BalanceSnapshot.BalanceUSD)
	require.NotNil(t, refreshed.BalanceSnapshot.UpdatedAt)
	provider.requireRequests(t, 1, 1, "Bearer plain-access-token", "42")
	requireSupplierBalanceSampleCount(t, supplier.ID, requireSupplierBalanceRevision(t, supplier.ID), 1)

	ciphertext := requireSupplierBalanceCiphertext(t, supplier.ID)
	require.NotEmpty(t, ciphertext)
	require.NotContains(t, ciphertext, "plain-access-token")
	requireSupplierBalanceJSONDoesNotLeakToken(t, *refreshed, "plain-access-token", ciphertext)

	listed := requireListedSupplier(t, admin, supplier.ID)
	require.True(t, listed.BalanceConfig.HasAccessToken)
	require.NotContains(t, mustJSON(t, listed), "plain-access-token")
	require.NotContains(t, mustJSON(t, listed), ciphertext)
}

func TestUpstreamSupplierBalanceDiscardsSupersededRefresh(t *testing.T) {
	for _, changeConfig := range []bool{true, false} {
		name := "newer snapshot"
		if changeConfig {
			name = "changed credentials"
		}
		t.Run(name, func(t *testing.T) {
			admin := newUpstreamSupplierBalanceAdmin(t)
			started := make(chan struct{})
			release := make(chan struct{})
			releaseFirst := sync.OnceFunc(func() { close(release) })
			var calls atomic.Int32
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					close(started)
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
					_, _ = w.Write([]byte(`{"remaining":1,"unit":"Credits"}`))
					return
				}
				_, _ = w.Write([]byte(`{"remaining":2,"unit":"Credits"}`))
			}))
			t.Cleanup(provider.Close)
			t.Cleanup(releaseFirst)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			supplier, err := admin.CreateUpstreamSupplier(ctx, svc.CreateUpstreamSupplierInput{
				Name: fmt.Sprintf("supplier-balance-superseded-%d", time.Now().UnixNano()),
				BalanceConfig: &svc.UpstreamSupplierBalanceInput{
					Enabled: true, Provider: "soleapi", BaseURL: provider.URL, AccessToken: "synthetic-original-key",
				},
			})
			require.NoError(t, err)
			type refreshResult struct {
				supplier *svc.UpstreamSupplier
				err      error
			}
			done := make(chan refreshResult, 1)
			go func() {
				updated, refreshErr := admin.RefreshUpstreamSupplierBalance(ctx, supplier.ID)
				done <- refreshResult{updated, refreshErr}
			}()
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("refresh did not reach the upstream fixture")
			}
			var latest *svc.UpstreamSupplier
			if changeConfig {
				latest, err = admin.UpdateUpstreamSupplier(ctx, svc.UpdateUpstreamSupplierInput{
					SupplierID: supplier.ID,
					BalanceConfig: &svc.UpstreamSupplierBalanceInput{
						Enabled: true, Provider: "soleapi", BaseURL: provider.URL, AccessToken: "synthetic-replacement-key",
					},
				})
			} else {
				latest, err = admin.RefreshUpstreamSupplierBalance(ctx, supplier.ID)
			}
			require.NoError(t, err)
			releaseFirst()
			select {
			case result := <-done:
				require.NoError(t, result.err, "discarding an outdated result is not a refresh error")
				require.Equal(t, latest.BalanceSnapshot, result.supplier.BalanceSnapshot)
			case <-ctx.Done():
				t.Fatal("superseded refresh did not finish")
			}
			stored := requireListedSupplier(t, admin, supplier.ID)
			require.Equal(t, latest.BalanceSnapshot, stored.BalanceSnapshot)
			if changeConfig {
				require.Nil(t, stored.BalanceSnapshot)
				requireSupplierBalanceSampleCount(t, supplier.ID, requireSupplierBalanceRevision(t, supplier.ID), 0)
			} else {
				require.NotNil(t, stored.BalanceSnapshot)
				require.Equal(t, "ok", stored.BalanceSnapshot.Status)
				require.NotNil(t, stored.BalanceSnapshot.BalanceUSD)
				require.Equal(t, float64(2), *stored.BalanceSnapshot.BalanceUSD)
				requireSupplierBalanceSampleCount(t, supplier.ID, requireSupplierBalanceRevision(t, supplier.ID), 1)
			}
		})
	}
}

func TestUpstreamSupplierBalanceDisabledConfigDoesNotRequireCredential(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	supplier, err := admin.CreateUpstreamSupplier(context.Background(), svc.CreateUpstreamSupplierInput{
		Name: fmt.Sprintf("supplier-balance-disabled-%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)
	require.False(t, supplier.BalanceConfig.Enabled)
	require.False(t, supplier.BalanceConfig.HasAccessToken)
	require.Empty(t, requireSupplierBalanceCiphertext(t, supplier.ID))

	listed := requireListedSupplier(t, admin, supplier.ID)
	require.False(t, listed.BalanceConfig.Enabled)
	require.False(t, listed.BalanceConfig.HasAccessToken)
	require.Nil(t, listed.BalanceSnapshot)

	refreshed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.Nil(t, refreshed)
	require.ErrorContains(t, err, "balance query is disabled or supplier is archived")
}

func TestUpstreamSupplierBalanceUpdatePreservesTokenWhenAccessTokenBlank(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	provider := newSupplierBalanceProvider(t, "750000")
	supplier := createSupplierWithBalanceConfig(t, admin, provider, "saved-token")
	originalCiphertext := requireSupplierBalanceCiphertext(t, supplier.ID)

	newName := fmt.Sprintf("supplier-balance-renamed-%d", time.Now().UnixNano())
	updated, err := admin.UpdateUpstreamSupplier(context.Background(), svc.UpdateUpstreamSupplierInput{
		SupplierID: supplier.ID,
		Name:       &newName,
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled:  true,
			Provider: "newapi",
			BaseURL:  provider.baseURL(),
			UserID:   42,
		},
	})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name)
	require.Equal(t, originalCiphertext, requireSupplierBalanceCiphertext(t, supplier.ID))

	refreshed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)

	require.NoError(t, err)
	require.Equal(t, float64(3), *refreshed.BalanceSnapshot.BalanceUSD)
	provider.requireRequests(t, 1, 1, "Bearer saved-token", "42")
}

func TestUpstreamSupplierBalanceSoleAPIStoresCreditsAndKeepsFailedSnapshot(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	var mu sync.Mutex
	failed := false
	calls := 0
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		require.Equal(t, "/sole/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer sk-sole-integration", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		if failed {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"Key expired: sk-sole-integration","code":"key_expired"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"remaining":131.4,"used":69.6,"total":201,"unit":"Credits"}`))
	}))
	t.Cleanup(provider.Close)
	supplier, err := admin.CreateUpstreamSupplier(context.Background(), svc.CreateUpstreamSupplierInput{
		Name: fmt.Sprintf("supplier-soleapi-%d", time.Now().UnixNano()),
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled: true, Provider: "soleapi", BaseURL: provider.URL + "/sole", AccessToken: "sk-sole-integration",
		},
	})
	require.NoError(t, err)
	ciphertext := requireSupplierBalanceCiphertext(t, supplier.ID)
	require.NotEmpty(t, ciphertext)
	require.NotEqual(t, "sk-sole-integration", ciphertext)
	success, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ok", success.BalanceSnapshot.Status)
	require.Equal(t, 131.4, *success.BalanceSnapshot.BalanceUSD)
	require.Equal(t, "Credits", success.BalanceSnapshot.Unit)
	requireSupplierBalanceSampleCount(t, supplier.ID, requireSupplierBalanceRevision(t, supplier.ID), 1)
	requireSupplierBalanceJSONDoesNotLeakToken(t, *success, "sk-sole-integration", ciphertext)
	listed := requireListedSupplier(t, admin, supplier.ID)
	require.Equal(t, "Credits", listed.BalanceSnapshot.Unit)
	require.Equal(t, 131.4, *listed.BalanceSnapshot.BalanceUSD)

	mu.Lock()
	failed = true
	mu.Unlock()
	stale, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "error", stale.BalanceSnapshot.Status)
	require.Equal(t, 131.4, *stale.BalanceSnapshot.BalanceUSD)
	require.Equal(t, "Credits", stale.BalanceSnapshot.Unit)
	require.True(t, stale.BalanceSnapshot.UpdatedAt.Equal(*success.BalanceSnapshot.UpdatedAt))
	require.Contains(t, stale.BalanceSnapshot.Error, "HTTP 403: Key expired: [redacted]")
	requireSupplierBalanceSampleCount(t, supplier.ID, requireSupplierBalanceRevision(t, supplier.ID), 1)
	requireSupplierBalanceJSONDoesNotLeakToken(t, *stale, "sk-sole-integration", ciphertext)
	mu.Lock()
	require.Equal(t, 2, calls)
	mu.Unlock()
}

func TestUpstreamSupplierBalanceSub2APIStoresWalletAndRejectsKeyQuota(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	var mu sync.Mutex
	body := `{"mode":"unrestricted","isValid":true,"balance":36.5,"remaining":999,"unit":"USD"}`
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		require.Equal(t, "/sub2api/v1/usage", r.URL.Path)
		require.Equal(t, "Bearer sk-sub2api-integration", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("New-Api-User"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(provider.Close)
	supplier, err := admin.CreateUpstreamSupplier(context.Background(), svc.CreateUpstreamSupplierInput{
		Name: fmt.Sprintf("supplier-sub2api-%d", time.Now().UnixNano()),
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled: true, Provider: "sub2api", BaseURL: provider.URL + "/sub2api", AccessToken: "sk-sub2api-integration",
		},
	})
	require.NoError(t, err)
	ciphertext := requireSupplierBalanceCiphertext(t, supplier.ID)
	require.NotEmpty(t, ciphertext)
	require.NotEqual(t, "sk-sub2api-integration", ciphertext)
	success, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ok", success.BalanceSnapshot.Status)
	require.Equal(t, 36.5, *success.BalanceSnapshot.BalanceUSD)
	require.Equal(t, "USD", success.BalanceSnapshot.Unit)
	requireSupplierBalanceJSONDoesNotLeakToken(t, *success, "sk-sub2api-integration", ciphertext)
	listed := requireListedSupplier(t, admin, supplier.ID)
	require.Equal(t, "USD", listed.BalanceSnapshot.Unit)
	require.Equal(t, 36.5, *listed.BalanceSnapshot.BalanceUSD)

	mu.Lock()
	body = `{"mode":"quota_limited","remaining":99,"unit":"USD","quota":{"limit":100,"remaining":99}}`
	mu.Unlock()
	stale, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "error", stale.BalanceSnapshot.Status)
	require.Equal(t, 36.5, *stale.BalanceSnapshot.BalanceUSD)
	require.Equal(t, "USD", stale.BalanceSnapshot.Unit)
	require.True(t, stale.BalanceSnapshot.UpdatedAt.Equal(*success.BalanceSnapshot.UpdatedAt))
	require.Contains(t, stale.BalanceSnapshot.Error, "API Key limits, not the account wallet")
	requireSupplierBalanceJSONDoesNotLeakToken(t, *stale, "sk-sub2api-integration", ciphertext)

	mu.Lock()
	body = `{"mode":"unrestricted","isValid":true,"balance":0,"remaining":0,"unit":"USD"}`
	mu.Unlock()
	zero, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, "ok", zero.BalanceSnapshot.Status)
	require.NotNil(t, zero.BalanceSnapshot.BalanceUSD)
	require.Zero(t, *zero.BalanceSnapshot.BalanceUSD)
	require.Empty(t, zero.BalanceSnapshot.Error)
}

func TestUpstreamSupplierBalanceFailureKeepsPreviousSuccessfulBalance(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	provider := newSupplierBalanceProvider(t, "500000")
	supplier := createSupplierWithBalanceConfig(t, admin, provider, "failure-token")
	first, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, float64(2), *first.BalanceSnapshot.BalanceUSD)
	require.NotNil(t, first.BalanceSnapshot.UpdatedAt)

	provider.setFailSelf(true)
	failed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)

	require.NoError(t, err)
	require.NotNil(t, failed.BalanceSnapshot)
	require.Equal(t, "error", failed.BalanceSnapshot.Status)
	require.Contains(t, failed.BalanceSnapshot.Error, "upstream HTTP 401")
	require.Equal(t, float64(2), *failed.BalanceSnapshot.BalanceUSD)
	require.Equal(t, first.BalanceSnapshot.UpdatedAt, failed.BalanceSnapshot.UpdatedAt)
	require.True(t, failed.BalanceSnapshot.LastAttemptAt.After(first.BalanceSnapshot.LastAttemptAt) || failed.BalanceSnapshot.LastAttemptAt.Equal(first.BalanceSnapshot.LastAttemptAt))
	provider.requireRequests(t, 2, 2, "Bearer failure-token", "42")
}

func TestUpstreamSupplierBalanceZeroIsStoredAsRealBalance(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	provider := newSupplierBalanceProvider(t, "0")
	supplier := createSupplierWithBalanceConfig(t, admin, provider, "zero-token")

	refreshed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)

	require.NoError(t, err)
	require.NotNil(t, refreshed.BalanceSnapshot)
	require.Equal(t, "ok", refreshed.BalanceSnapshot.Status)
	require.NotNil(t, refreshed.BalanceSnapshot.BalanceUSD)
	require.Zero(t, *refreshed.BalanceSnapshot.BalanceUSD)
}

func TestUpstreamSupplierBalanceChangingSiteClearsSnapshotAndRequiresNewToken(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	firstProvider := newSupplierBalanceProvider(t, "1000000")
	supplier := createSupplierWithBalanceConfig(t, admin, firstProvider, "first-token")
	refreshed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.NotNil(t, refreshed.BalanceSnapshot)
	firstRevision := requireSupplierBalanceRevision(t, supplier.ID)
	requireSupplierBalanceSampleCount(t, supplier.ID, firstRevision, 1)

	secondProvider := newSupplierBalanceProvider(t, "250000")
	_, err = admin.UpdateUpstreamSupplier(context.Background(), svc.UpdateUpstreamSupplierInput{
		SupplierID: supplier.ID,
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled:  true,
			Provider: "newapi",
			BaseURL:  secondProvider.baseURL(),
			UserID:   42,
		},
	})
	require.ErrorContains(t, err, "enter a new token")

	updated, err := admin.UpdateUpstreamSupplier(context.Background(), svc.UpdateUpstreamSupplierInput{
		SupplierID: supplier.ID,
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled:     true,
			Provider:    "newapi",
			BaseURL:     secondProvider.baseURL(),
			UserID:      42,
			AccessToken: "second-token",
		},
	})
	require.NoError(t, err)
	require.Nil(t, updated.BalanceSnapshot)
	secondRevision := requireSupplierBalanceRevision(t, supplier.ID)
	require.Equal(t, firstRevision+1, secondRevision)
	requireSupplierBalanceSampleCount(t, supplier.ID, secondRevision, 0)

	refreshed, err = admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)
	require.NoError(t, err)
	require.Equal(t, float64(1), *refreshed.BalanceSnapshot.BalanceUSD)
	requireSupplierBalanceSampleCount(t, supplier.ID, secondRevision, 1)
	secondProvider.requireRequests(t, 1, 1, "Bearer second-token", "42")
}

func TestUpstreamSupplierBalancePollerAtomicClaimSkipsDisabledAndSystem(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	cfg := supplierBalanceIntegrationConfig()
	encryptor, err := repository.NewAESEncryptor(cfg)
	require.NoError(t, err)
	provider := newSupplierBalanceProvider(t, "1750000")
	active := createSupplierWithBalanceConfig(t, admin, provider, "poller-active-token")
	disabled, err := admin.CreateUpstreamSupplier(context.Background(), svc.CreateUpstreamSupplierInput{
		Name: fmt.Sprintf("supplier-poller-disabled-%d", time.Now().UnixNano()),
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled:     false,
			Provider:    "newapi",
			BaseURL:     provider.baseURL(),
			UserID:      42,
			AccessToken: "poller-disabled-token",
		},
	})
	require.NoError(t, err)
	system := createSupplierWithBalanceConfig(t, admin, provider, "poller-system-token")
	_, err = serviceIntegrationDB.ExecContext(context.Background(), `
UPDATE upstream_suppliers
SET is_system = TRUE, balance_next_poll_at = NOW()
WHERE id = $1`, system.ID)
	require.NoError(t, err)

	pollerA := svc.NewUpstreamSupplierBalancePollerWithOptions(serviceIntegrationEntClient, cfg, encryptor, time.Hour, time.Hour, 4, 4, 35*24*time.Hour)
	pollerB := svc.NewUpstreamSupplierBalancePollerWithOptions(serviceIntegrationEntClient, cfg, encryptor, time.Hour, time.Hour, 4, 4, 35*24*time.Hour)
	pollerA.Start()
	pollerB.Start()
	t.Cleanup(pollerA.Stop)
	t.Cleanup(pollerB.Stop)
	require.Eventually(t, func() bool {
		statusCalls, selfCalls := provider.requestCounts()
		return statusCalls == 1 && selfCalls == 1
	}, 10*time.Second, 20*time.Millisecond)
	activeRevision := requireSupplierBalanceRevision(t, active.ID)
	require.Eventually(t, func() bool {
		return supplierBalanceSampleCount(t, active.ID, activeRevision) == 1
	}, 10*time.Second, 20*time.Millisecond)
	pollerA.Stop()
	pollerB.Stop()

	provider.requireRequests(t, 1, 1, "Bearer poller-active-token", "42")
	requireSupplierBalanceSampleCount(t, active.ID, activeRevision, 1)
	requireSupplierBalanceSampleCount(t, disabled.ID, requireSupplierBalanceRevision(t, disabled.ID), 0)
	requireSupplierBalanceSampleCount(t, system.ID, requireSupplierBalanceRevision(t, system.ID), 0)
}

func TestUpstreamSupplierBalanceArchivedSupplierRejectsRefreshWithoutRequest(t *testing.T) {
	admin := newUpstreamSupplierBalanceAdmin(t)
	provider := newSupplierBalanceProvider(t, "1000000")
	supplier := createSupplierWithBalanceConfig(t, admin, provider, "archived-token")
	archived := "archived"
	_, err := admin.UpdateUpstreamSupplier(context.Background(), svc.UpdateUpstreamSupplierInput{
		SupplierID: supplier.ID,
		Status:     &archived,
	})
	require.NoError(t, err)

	refreshed, err := admin.RefreshUpstreamSupplierBalance(context.Background(), supplier.ID)

	require.Nil(t, refreshed)
	require.ErrorContains(t, err, "balance query is disabled or supplier is archived")
	provider.requireRequests(t, 0, 0, "", "")
}

func newUpstreamSupplierBalanceAdmin(t *testing.T) upstreamSupplierBalanceAdmin {
	t.Helper()
	cfg := supplierBalanceIntegrationConfig()
	encryptor, err := repository.NewAESEncryptor(cfg)
	require.NoError(t, err)
	accountRepo := repository.NewAdminAccountRepository(serviceIntegrationEntClient, serviceIntegrationDB, nil)
	adminService := svc.NewAdminService(
		cfg,
		nil,
		nil,
		accountRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		serviceIntegrationEntClient,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		encryptor,
	)
	admin, ok := adminService.(upstreamSupplierBalanceAdmin)
	require.True(t, ok)
	return admin
}

func supplierBalanceIntegrationConfig() *config.Config {
	return &config.Config{
		Totp: config.TotpConfig{EncryptionKey: strings.Repeat("42", 32)},
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}
}

func createSupplierWithBalanceConfig(t *testing.T, admin upstreamSupplierBalanceAdmin, provider *supplierBalanceProvider, token string) *svc.UpstreamSupplier {
	t.Helper()
	supplier, err := admin.CreateUpstreamSupplier(context.Background(), svc.CreateUpstreamSupplierInput{
		Name: fmt.Sprintf("supplier-balance-%d", time.Now().UnixNano()),
		BalanceConfig: &svc.UpstreamSupplierBalanceInput{
			Enabled:     true,
			Provider:    "newapi",
			BaseURL:     provider.baseURL(),
			UserID:      42,
			AccessToken: token,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, supplier)
	require.True(t, supplier.BalanceConfig.Enabled)
	require.True(t, supplier.BalanceConfig.HasAccessToken)
	require.Equal(t, "newapi", supplier.BalanceConfig.Provider)
	require.Equal(t, strings.TrimRight(provider.baseURL(), "/"), supplier.BalanceConfig.BaseURL)
	require.Nil(t, supplier.BalanceSnapshot)
	requireSupplierBalanceJSONDoesNotLeakToken(t, *supplier, token, requireSupplierBalanceCiphertext(t, supplier.ID))
	return supplier
}

func requireSupplierBalanceCiphertext(t *testing.T, supplierID int64) string {
	t.Helper()
	var ciphertext string
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT balance_access_token
FROM upstream_suppliers
WHERE id = $1`, supplierID).Scan(&ciphertext))
	return ciphertext
}

func requireSupplierBalanceRevision(t *testing.T, supplierID int64) int64 {
	t.Helper()
	var revision int64
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT balance_revision
FROM upstream_suppliers
WHERE id = $1`, supplierID).Scan(&revision))
	return revision
}

func requireSupplierBalanceSampleCount(t *testing.T, supplierID, revision int64, want int) {
	t.Helper()
	require.Equal(t, want, supplierBalanceSampleCount(t, supplierID, revision))
}

func supplierBalanceSampleCount(t *testing.T, supplierID, revision int64) int {
	t.Helper()
	var count int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM upstream_supplier_balance_samples
WHERE supplier_id = $1 AND revision = $2`, supplierID, revision).Scan(&count))
	return count
}

func requireListedSupplier(t *testing.T, admin upstreamSupplierBalanceAdmin, supplierID int64) svc.UpstreamSupplier {
	t.Helper()
	suppliers, err := admin.ListUpstreamSuppliers(context.Background())
	require.NoError(t, err)
	for _, supplier := range suppliers {
		if supplier.ID == supplierID {
			return supplier
		}
	}
	t.Fatalf("supplier %d was not listed", supplierID)
	return svc.UpstreamSupplier{}
}

func requireSupplierBalanceJSONDoesNotLeakToken(t *testing.T, supplier svc.UpstreamSupplier, token, ciphertext string) {
	t.Helper()
	encoded := mustJSON(t, supplier)
	require.NotContains(t, encoded, token)
	if ciphertext != "" {
		require.NotContains(t, encoded, ciphertext)
	}
	require.NotContains(t, encoded, `"access_token"`)
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}
