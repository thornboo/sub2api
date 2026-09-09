package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchSoleAPISupplierBalanceReadsRemainingCredits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		remaining any
		want      float64
	}{
		{"wallet including rewards", 131.4, 131.4},
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
				require.Equal(t, "Bearer sk-sole-test-credential", r.Header.Get("Authorization"))
				require.Empty(t, r.Header.Get("New-Api-User"))
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"remaining": tc.remaining, "used": 69.6, "total": 201, "unit": "Credits",
				})
			}))
			t.Cleanup(server.Close)
			balance, err := fetchSoleAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "soleapi", BaseURL: server.URL + "/panel/", UserID: 42,
			}, "sk-sole-test-credential")
			require.NoError(t, err)
			require.Equal(t, tc.want, balance)
			require.Equal(t, 1, calls)
		})
	}
}

func TestFetchSoleAPISupplierBalanceRejectsMissingInvalidOrFailedValues(t *testing.T) {
	for name, body := range map[string]string{
		"missing balance": `{"unit":"Credits","used":7,"total":20}`,
		"null balance":    `{"unit":"Credits","remaining":null}`,
		"invalid balance": `{"unit":"Credits","remaining":"NaN"}`,
		"missing unit":    `{"remaining":12}`,
		"wrong unit":      `{"unit":"CNY","remaining":12}`,
		"wrong envelope":  `{"success":true,"data":{"remaining":12,"unit":"Credits"}}`,
		"failed response": `{"success":false,"remaining":12,"unit":"Credits"}`,
		"error envelope":  `{"error":{"message":"permission denied"},"remaining":12,"unit":"Credits"}`,
		"html response":   `<html>Gateway unavailable</html>`,
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			t.Cleanup(server.Close)
			_, err := fetchSoleAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "soleapi", BaseURL: server.URL,
			}, "sk-sole-test")
			require.Error(t, err)
			require.NotContains(t, err.Error(), "sk-sole-test")
		})
	}
}

func TestFetchSoleAPISupplierBalancePreservesNestedErrorAndRedactsAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"API key expired: sk-sole-private","type":"invalid_request_error","code":"key_expired"}}`))
	}))
	t.Cleanup(server.Close)
	_, err := fetchSoleAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "soleapi", BaseURL: server.URL,
	}, "sk-sole-private")
	require.ErrorContains(t, err, "upstream HTTP 403: API key expired: [redacted]")
	require.NotContains(t, err.Error(), "sk-sole-private")
}

func TestFetchSoleAPISupplierBalanceDoesNotFollowRedirect(t *testing.T) {
	redirectCalls := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCalls++
	}))
	t.Cleanup(target.Close)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	t.Cleanup(server.Close)
	_, err := fetchSoleAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "soleapi", BaseURL: server.URL,
	}, "sk-sole-private")
	require.ErrorContains(t, err, "upstream HTTP 302")
	require.Zero(t, redirectCalls)
}
