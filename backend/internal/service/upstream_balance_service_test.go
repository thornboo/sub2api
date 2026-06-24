package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type upstreamBalanceHTTPStub struct {
	response *http.Response
	err      error
	calls    int
	lastReq  *http.Request
}

func (s *upstreamBalanceHTTPStub) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return s.DoWithTLS(req, proxyURL, accountID, accountConcurrency, nil)
}

func (s *upstreamBalanceHTTPStub) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	s.calls++
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	if s.response == nil {
		return nil, fmt.Errorf("missing upstream balance stub response")
	}
	resp := *s.response
	return &resp, nil
}

func newUpstreamBalanceTestService(stub *upstreamBalanceHTTPStub) *AccountTestService {
	return &AccountTestService{
		httpUpstream: stub,
		cfg: &config.Config{
			Security: config.SecurityConfig{
				URLAllowlist: config.URLAllowlistConfig{
					Enabled:           false,
					AllowInsecureHTTP: true,
				},
			},
		},
	}
}

func newUpstreamBalanceTestAccount(extra map[string]any) *Account {
	return &Account{
		ID:          11,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "http://newapi.example/v1",
		},
		Extra: extra,
	}
}

func newUpstreamBalanceResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestFetchUpstreamBalanceNewAPICompatibleSuccess(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusOK, `{
			"success": true,
			"data": {
				"total_granted": 2500000,
				"total_used": "500000",
				"total_available": 2000000,
				"unlimited_quota": false,
				"expires_at": 1800000000
			}
		}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceProviderExtraKey:     UpstreamBalanceProviderNewAPICompatible,
	})

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "ok", snapshot.Status)
	require.Equal(t, http.StatusOK, snapshot.StatusCode)
	require.Equal(t, "http://newapi.example/api/usage/token/", snapshot.Endpoint)
	require.NotNil(t, snapshot.RawAvailable)
	require.Equal(t, float64(2000000), *snapshot.RawAvailable)
	require.NotNil(t, snapshot.RawUsed)
	require.Equal(t, float64(500000), *snapshot.RawUsed)
	require.NotNil(t, snapshot.AvailableUSD)
	require.Equal(t, float64(4), *snapshot.AvailableUSD)
	require.NotNil(t, snapshot.ExpiresAt)
	require.Equal(t, 1, stub.calls)
	require.NotNil(t, stub.lastReq)
	require.Equal(t, "Bearer sk-test", stub.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", stub.lastReq.Header.Get("Accept"))
}

func TestFetchUpstreamBalanceSub2APISuccess(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusOK, `{
			"mode": "unrestricted",
			"isValid": true,
			"remaining": 8.64,
			"balance": 8.64,
			"unit": "USD"
		}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
	})

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, UpstreamBalanceProviderSub2API, snapshot.Provider)
	require.Equal(t, "ok", snapshot.Status)
	require.Equal(t, "usd", snapshot.RawUnit)
	require.Equal(t, "http://newapi.example/v1/usage", snapshot.Endpoint)
	require.NotNil(t, snapshot.RawAvailable)
	require.Equal(t, 8.64, *snapshot.RawAvailable)
	require.NotNil(t, snapshot.AvailableUSD)
	require.Equal(t, 8.64, *snapshot.AvailableUSD)
	require.Equal(t, 1, stub.calls)
	require.NotNil(t, stub.lastReq)
	require.Equal(t, "Bearer sk-test", stub.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", stub.lastReq.Header.Get("Accept"))
}

func TestFetchUpstreamBalanceSub2APIUsesTopLevelRemaining(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusOK, `{
			"mode": "unrestricted",
			"isValid": true,
			"remaining": 8.64,
			"unit": "USD"
		}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
	})

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "ok", snapshot.Status)
	require.Equal(t, "usd", snapshot.RawUnit)
	require.NotNil(t, snapshot.RawAvailable)
	require.Equal(t, 8.64, *snapshot.RawAvailable)
	require.NotNil(t, snapshot.AvailableUSD)
	require.Equal(t, 8.64, *snapshot.AvailableUSD)
}

func TestFetchUpstreamBalanceSub2APIProfileSuccess(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusOK, `{
			"code": 0,
			"message": "success",
			"data": {
				"id": 7,
				"email": "user@example.com",
				"balance": 8.64,
				"total_recharged": 12.5
			}
		}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceEndpointExtraKey:     UpstreamBalanceSub2APIProfileEndpoint,
		upstreamBalanceAuthModeExtraKey:     UpstreamBalanceAuthModeBearerToken,
	})
	account.Credentials[upstreamBalanceAuthTokenCredKey] = "jwt-token"

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, UpstreamBalanceProviderSub2API, snapshot.Provider)
	require.Equal(t, "ok", snapshot.Status)
	require.Equal(t, "usd", snapshot.RawUnit)
	require.Equal(t, "http://newapi.example/api/v1/user/profile", snapshot.Endpoint)
	require.NotNil(t, snapshot.RawAvailable)
	require.Equal(t, 8.64, *snapshot.RawAvailable)
	require.NotNil(t, snapshot.AvailableUSD)
	require.Equal(t, 8.64, *snapshot.AvailableUSD)
	require.NotNil(t, snapshot.RawGranted)
	require.Equal(t, 12.5, *snapshot.RawGranted)
	require.Equal(t, 1, stub.calls)
	require.NotNil(t, stub.lastReq)
	require.Equal(t, "Bearer jwt-token", stub.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", stub.lastReq.Header.Get("Accept"))
}

func TestResolveUpstreamBalanceConfigMigratesLegacySub2APIProfileAccountKeyConfig(t *testing.T) {
	cfg := ResolveUpstreamBalanceConfig(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceProviderExtraKey:     UpstreamBalanceProviderSub2API,
		upstreamBalanceEndpointExtraKey:     UpstreamBalanceSub2APIProfileEndpoint,
		upstreamBalanceAuthModeExtraKey:     UpstreamBalanceAuthModeAccountAPIKey,
	})

	require.True(t, cfg.Enabled)
	require.Equal(t, UpstreamBalanceProviderSub2API, cfg.Provider)
	require.Equal(t, UpstreamBalanceDefaultEndpoint, cfg.Endpoint)
	require.Equal(t, UpstreamBalanceAuthModeAccountAPIKey, cfg.AuthMode)
}

func TestFetchUpstreamBalanceNewAPICompatibleKeepsUpstreamErrorAsSnapshot(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusUnauthorized, `{"error":{"message":"invalid token sk-secret"}}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceProviderExtraKey:     UpstreamBalanceProviderNewAPICompatible,
	})

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "error", snapshot.Status)
	require.Equal(t, http.StatusUnauthorized, snapshot.StatusCode)
	require.Contains(t, snapshot.Error, "upstream HTTP 401")
}

func TestFetchUpstreamBalanceUsesDedicatedBearerToken(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusOK, `{
			"success": true,
			"data": {"total_available": 500000, "unlimited_quota": false}
		}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceProviderExtraKey:     UpstreamBalanceProviderNewAPICompatible,
		upstreamBalanceAuthModeExtraKey:     UpstreamBalanceAuthModeBearerToken,
	})
	account.Credentials[upstreamBalanceAuthTokenCredKey] = "balance-token"

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, "ok", snapshot.Status)
	require.NotNil(t, stub.lastReq)
	require.Equal(t, "Bearer balance-token", stub.lastReq.Header.Get("Authorization"))
}

func TestFetchUpstreamBalanceUsesCustomHeader(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{
		response: newUpstreamBalanceResponse(http.StatusOK, `{
			"success": true,
			"data": {"total_available": 500000, "unlimited_quota": false}
		}`),
	}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceProviderExtraKey:     UpstreamBalanceProviderNewAPICompatible,
		upstreamBalanceAuthModeExtraKey:     UpstreamBalanceAuthModeCustomHeader,
		upstreamBalanceAuthHeaderExtraKey:   "X-Panel-Token",
	})
	account.Credentials[upstreamBalanceAuthTokenCredKey] = "panel-token"

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.Equal(t, "ok", snapshot.Status)
	require.NotNil(t, stub.lastReq)
	require.Empty(t, stub.lastReq.Header.Get("Authorization"))
	require.Equal(t, "panel-token", stub.lastReq.Header.Get("X-Panel-Token"))
}

func TestFetchUpstreamBalanceRejectsMissingDedicatedToken(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{
		upstreamBalanceQueryEnabledExtraKey: true,
		upstreamBalanceAuthModeExtraKey:     UpstreamBalanceAuthModeBearerToken,
	})

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, "error", snapshot.Status)
	require.Equal(t, "missing upstream_balance_auth_token", snapshot.Error)
	require.Equal(t, 0, stub.calls)
}

func TestFetchUpstreamBalanceRejectsDisabledAccount(t *testing.T) {
	stub := &upstreamBalanceHTTPStub{}
	svc := newUpstreamBalanceTestService(stub)
	account := newUpstreamBalanceTestAccount(map[string]any{})

	snapshot, err := svc.FetchUpstreamBalance(context.Background(), account)

	require.Error(t, err)
	require.Nil(t, snapshot)
	require.Equal(t, 0, stub.calls)
}
