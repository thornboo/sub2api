package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchSub2APISupplierBalanceReadsAccountWallet(t *testing.T) {
	for _, tc := range []struct {
		name    string
		balance any
		want    float64
	}{
		{"wallet balance", 42.125, 42.125},
		{"real zero", 0, 0},
		{"negative balance", -0.25, -0.25},
		{"decimal string", "12.3456", 12.3456},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/panel/v1/usage", r.URL.Path)
				require.Empty(t, r.URL.RawQuery)
				require.Equal(t, "Bearer sk-sub2api-test", r.Header.Get("Authorization"))
				require.Empty(t, r.Header.Get("New-Api-User"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"mode": "unrestricted", "isValid": true, "unit": "USD",
					"balance": tc.balance, "remaining": 999, "usage": map[string]any{"cost": 456},
				})
			}))
			t.Cleanup(server.Close)
			balance, err := fetchSub2APISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "sub2api", BaseURL: server.URL + "/panel/", UserID: 42,
			}, "sk-sub2api-test")
			require.NoError(t, err)
			require.Equal(t, tc.want, balance)
			require.Equal(t, 1, calls)
		})
	}
}

func TestFetchSub2APISupplierBalanceRejectsKeyQuotasAndSubscriptionAllowances(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"key quota", `{"mode":"quota_limited","isValid":true,"remaining":99,"unit":"USD","quota":{"limit":100,"remaining":99}}`, "API Key limits"},
		{"periodic limits", `{"mode":"quota_limited","isValid":true,"rate_limits":[{"window":"5h","remaining":99}]}`, "API Key limits"},
		{"quota with extra balance", `{"mode":"quota_limited","balance":999,"unit":"USD"}`, "API Key limits"},
		{"subscription", `{"mode":"unrestricted","isValid":true,"remaining":99,"unit":"USD","subscription":{"daily_limit_usd":100}}`, "subscription allowances"},
		{"subscription with extra balance", `{"mode":"unrestricted","balance":999,"unit":"USD","subscription":{}}`, "subscription allowances"},
		{"subscription not loaded", `{"mode":"unrestricted","isValid":true,"planName":"Monthly","unit":"USD"}`, "account wallet balance"},
		{"remaining without balance", `{"mode":"unrestricted","remaining":99,"unit":"USD"}`, "account wallet balance"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)
			_, err := fetchSub2APISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "sub2api", BaseURL: server.URL,
			}, "sk-sub2api-test")
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestFetchSub2APISupplierBalanceRejectsInvalidOrFailedResponses(t *testing.T) {
	for name, body := range map[string]string{
		"missing mode":    `{"balance":12,"unit":"USD"}`,
		"unknown mode":    `{"mode":"custom","balance":12,"unit":"USD"}`,
		"null balance":    `{"mode":"unrestricted","balance":null,"unit":"USD"}`,
		"invalid balance": `{"mode":"unrestricted","balance":"NaN","unit":"USD"}`,
		"infinity":        `{"mode":"unrestricted","balance":"Inf","unit":"USD"}`,
		"boolean balance": `{"mode":"unrestricted","balance":false,"unit":"USD"}`,
		"missing unit":    `{"mode":"unrestricted","balance":12}`,
		"wrong unit":      `{"mode":"unrestricted","balance":12,"unit":"Credits"}`,
		"invalid key":     `{"mode":"unrestricted","balance":12,"unit":"USD","isValid":false}`,
		"wrong envelope":  `{"data":{"mode":"unrestricted","balance":12,"unit":"USD"}}`,
		"failed response": `{"success":false,"mode":"unrestricted","balance":12,"unit":"USD"}`,
		"error envelope":  `{"error":{"message":"permission denied"},"mode":"unrestricted","balance":12,"unit":"USD"}`,
		"html response":   `<html>Gateway unavailable</html>`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(server.Close)
			_, err := fetchSub2APISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "sub2api", BaseURL: server.URL,
			}, "sk-sub2api-test")
			require.Error(t, err)
			require.NotContains(t, err.Error(), "sk-sub2api-test")
		})
	}
}

func TestFetchSub2APISupplierBalancePreservesErrorAndRedactsAPIKey(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusForbidden} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"message":"API key expired: sk-sub2api-private","code":"key_expired"}}`))
		}))
		t.Cleanup(server.Close)
		_, err := fetchSub2APISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
			Provider: "sub2api", BaseURL: server.URL,
		}, "sk-sub2api-private")
		require.ErrorContains(t, err, "API key expired: [redacted]")
		require.NotContains(t, err.Error(), "sk-sub2api-private")
		if status == http.StatusForbidden {
			require.ErrorContains(t, err, "HTTP 403")
		}
	}
}

func TestFetchSub2APISupplierBalanceDoesNotFollowRedirect(t *testing.T) {
	redirectCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCalls++
	}))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	_, err := fetchSub2APISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "sub2api", BaseURL: server.URL,
	}, "sk-sub2api-private")
	require.ErrorContains(t, err, "upstream HTTP 302")
	require.Zero(t, redirectCalls)
}
