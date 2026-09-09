package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newSupplierBalanceQueryTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
				AllowPrivateHosts: true,
			},
		},
	}
}

func TestFetchNewAPISupplierBalanceSuccessWithNonDefaultScaleAndHeaders(t *testing.T) {
	var statusAuth, userAuth, userOverride string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/panel/api/status":
			statusAuth = r.Header.Get("Authorization")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":250000}}`))
		case "/panel/api/user/self":
			userAuth = r.Header.Get("Authorization")
			userOverride = r.Header.Get("New-Api-User")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota":1250000,"used_quota":999999}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	balance, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "newapi",
		BaseURL:  server.URL + "/panel/",
		UserID:   42,
	}, "secret-token")

	require.NoError(t, err)
	require.Equal(t, float64(5), balance)
	require.Empty(t, statusAuth)
	require.Equal(t, "Bearer secret-token", userAuth)
	require.Equal(t, "42", userOverride)
}

func TestFetchNewAPISupplierBalanceAcceptsZeroAndNegativeBalance(t *testing.T) {
	for name, quota := range map[string]string{
		"zero":     "0",
		"negative": "-250000",
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/status":
					_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
				case "/api/user/self":
					_, _ = fmt.Fprintf(w, `{"success":true,"data":{"quota":%s}}`, quota)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			balance, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "newapi",
				BaseURL:  server.URL,
			}, "secret-token")

			require.NoError(t, err)
			if quota == "0" {
				require.Zero(t, balance)
			} else {
				require.Equal(t, -0.5, balance)
			}
		})
	}
}

func TestFetchNewAPISupplierBalanceRejectsMissingNullAndInvalidNumbers(t *testing.T) {
	tests := []struct {
		name       string
		statusBody string
		userBody   string
		wantErr    string
	}{
		{
			name:       "missing quota per unit",
			statusBody: `{"success":true,"data":{}}`,
			userBody:   `{"success":true,"data":{"quota":1}}`,
			wantErr:    "missing quota_per_unit",
		},
		{
			name:       "null quota per unit",
			statusBody: `{"success":true,"data":{"quota_per_unit":null}}`,
			userBody:   `{"success":true,"data":{"quota":1}}`,
			wantErr:    "missing quota_per_unit",
		},
		{
			name:       "zero quota per unit",
			statusBody: `{"success":true,"data":{"quota_per_unit":0}}`,
			userBody:   `{"success":true,"data":{"quota":1}}`,
			wantErr:    "invalid quota_per_unit",
		},
		{
			name:       "missing quota",
			statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`,
			userBody:   `{"success":true,"data":{}}`,
			wantErr:    "missing quota",
		},
		{
			name:       "null quota",
			statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`,
			userBody:   `{"success":true,"data":{"quota":null}}`,
			wantErr:    "missing quota",
		},
		{
			name:       "nonfinite quota",
			statusBody: `{"success":true,"data":{"quota_per_unit":500000}}`,
			userBody:   `{"success":true,"data":{"quota":"NaN"}}`,
			wantErr:    "invalid quota",
		},
		{
			name:       "failed envelope",
			statusBody: `{"success":false,"message":"bad status"}`,
			userBody:   `{"success":true,"data":{"quota":1}}`,
			wantErr:    "upstream response indicates failure: bad status",
		},
		{
			name:       "missing success envelope",
			statusBody: `{"data":{"quota_per_unit":500000}}`,
			userBody:   `{"success":true,"data":{"quota":1}}`,
			wantErr:    "upstream response indicates failure",
		},
		{
			name:       "wrong success envelope type",
			statusBody: `{"success":"true","data":{"quota_per_unit":500000}}`,
			userBody:   `{"success":true,"data":{"quota":1}}`,
			wantErr:    "upstream response indicates failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/status":
					_, _ = w.Write([]byte(tt.statusBody))
				case "/api/user/self":
					_, _ = w.Write([]byte(tt.userBody))
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
				Provider: "newapi",
				BaseURL:  server.URL,
			}, "secret-token")

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
			require.NotContains(t, err.Error(), "secret-token")
		})
	}
}

func TestFetchNewAPISupplierBalanceKeepsSafeFailureMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":false,"message":"invalid user or token scope"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "newapi",
		BaseURL:  server.URL,
	}, "secret-token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "account query failed: upstream response indicates failure: invalid user or token scope")
	require.NotContains(t, err.Error(), "secret-token")
}

func TestFetchNewAPISupplierBalanceDropsUnsafeFailureMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			_, _ = w.Write([]byte(`{"success":false,"message":"Access Token expired: echoed-token-value"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "newapi",
		BaseURL:  server.URL,
	}, "echoed-token-value")

	require.Error(t, err)
	require.Contains(t, err.Error(), "account query failed: upstream response indicates failure: Access Token expired: [redacted]")
	require.NotContains(t, err.Error(), "echoed-token-value")
}

func TestFetchNewAPISupplierBalanceIncludesNon2xxJSONMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"success":false,"message":"Access Token expired: echoed-token-value"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "newapi",
		BaseURL:  server.URL,
	}, "echoed-token-value")

	require.Error(t, err)
	require.Contains(t, err.Error(), "account query failed: upstream HTTP 401: Access Token expired: [redacted]")
	require.NotContains(t, err.Error(), "echoed-token-value")
}

func TestClassifyNewAPISupplierRequestErrorDistinguishesTimeout(t *testing.T) {
	err := classifyNewAPISupplierRequestError(context.DeadlineExceeded)

	require.EqualError(t, err, "upstream request timed out")
}

func TestRequestNewAPISupplierJSONRedactsCredentialsAndPreservesReadableErrors(t *testing.T) {
	for _, token := range []string{"abc", "plain-token-value", "token  with  spaces"} {
		t.Run(token, func(t *testing.T) {
			message := sanitizeNewAPIUpstreamFailureMessage("Access Token expired: "+token, []string{token})
			require.Equal(t, "Access Token expired: [redacted]", message)
		})
	}
	message := sanitizeNewAPIUpstreamFailureMessage(strings.Repeat("用户权限不足", 50), nil)
	require.True(t, utf8.ValidString(message))
	require.LessOrEqual(t, len([]rune(message)), 163)
	require.Empty(t, sanitizeNewAPIUpstreamFailureMessage("<html>Gateway error</html>", nil))
}

type supplierBalanceReadTimeoutTransport struct{}

func (supplierBalanceReadTimeoutTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusGatewayTimeout,
		Header:     http.Header{},
		Body:       io.NopCloser(iotest.ErrReader(context.DeadlineExceeded)),
	}, nil
}

func TestRequestNewAPISupplierJSONKeepsStatusOnResponseReadTimeout(t *testing.T) {
	client := &http.Client{Transport: supplierBalanceReadTimeoutTransport{}}
	_, err := requestNewAPISupplierJSON(context.Background(), client, "https://supplier.example/api/user/self", nil)
	require.EqualError(t, err, "upstream HTTP 504 response: upstream request timed out")
}

func TestFetchNewAPISupplierBalanceStatusEndpointUsesNoAuthAndFailureDoesNotCallUserEndpoint(t *testing.T) {
	userCalls := 0
	var statusAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			statusAuth = r.Header.Get("Authorization")
			http.Error(w, "server body should not leak", http.StatusServiceUnavailable)
		case "/api/user/self":
			userCalls++
			http.Error(w, "unexpected", http.StatusTeapot)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "newapi",
		BaseURL:  server.URL,
	}, "secret-token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "upstream HTTP 503")
	require.NotContains(t, err.Error(), "server body should not leak")
	require.NotContains(t, err.Error(), "secret-token")
	require.Empty(t, statusAuth)
	require.Zero(t, userCalls)
}

func TestFetchNewAPISupplierBalanceRejectsRedirectWithoutLeakingAuthorization(t *testing.T) {
	redirectTargetCalls := 0
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectTargetCalls++
		require.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(redirectTarget.Close)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			http.Redirect(w, r, redirectTarget.URL+"/capture", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), UpstreamSupplierBalanceConfig{
		Provider: "newapi",
		BaseURL:  server.URL,
	}, "secret-token")

	require.Error(t, err)
	require.Contains(t, err.Error(), "upstream HTTP 302")
	require.Zero(t, redirectTargetCalls)
}

func TestFetchNewAPISupplierBalanceRejectsUnsafeInput(t *testing.T) {
	tests := []struct {
		name   string
		cfg    UpstreamSupplierBalanceConfig
		token  string
		errMsg string
	}{
		{
			name:   "unsupported provider",
			cfg:    UpstreamSupplierBalanceConfig{Provider: "sub2api", BaseURL: "https://example.com"},
			token:  "secret",
			errMsg: "unsupported supplier balance provider",
		},
		{
			name:   "empty base URL",
			cfg:    UpstreamSupplierBalanceConfig{Provider: "newapi"},
			token:  "secret",
			errMsg: "supplier website is required",
		},
		{
			name:   "base URL with query",
			cfg:    UpstreamSupplierBalanceConfig{Provider: "newapi", BaseURL: "https://example.com?access_token=secret"},
			token:  "secret",
			errMsg: "must not contain credentials",
		},
		{
			name:   "empty token",
			cfg:    UpstreamSupplierBalanceConfig{Provider: "newapi", BaseURL: "https://example.com"},
			token:  " ",
			errMsg: "missing supplier Access Token",
		},
		{
			name:   "token newline",
			cfg:    UpstreamSupplierBalanceConfig{Provider: "newapi", BaseURL: "https://example.com"},
			token:  "secret\nx",
			errMsg: "invalid supplier Access Token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fetchNewAPISupplierBalance(context.Background(), newSupplierBalanceQueryTestConfig(), tt.cfg, tt.token)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMsg)
			require.NotContains(t, err.Error(), "secret")
		})
	}
}

func TestBuildNewAPISupplierBalanceURLPreservesSubdirectory(t *testing.T) {
	endpoint, err := buildNewAPISupplierBalanceURL("https://supplier.example/newapi", "/api/user/self")

	require.NoError(t, err)
	require.Equal(t, "https://supplier.example/newapi/api/user/self", endpoint)
}

func TestBuildNewAPISupplierBalanceURLDoesNotDoubleEncodeEscapedBasePath(t *testing.T) {
	endpoint, err := buildNewAPISupplierBalanceURL("https://supplier.example/site%20root", "/api/user/self")

	require.NoError(t, err)
	require.Equal(t, "https://supplier.example/site%20root/api/user/self", endpoint)
}

func TestRequestNewAPISupplierJSONRejectsPrivateResolvedIPWhenMarkedPublicOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("request should be rejected before reaching private test server")
	}))
	t.Cleanup(server.Close)

	_, err := requestNewAPISupplierJSON(WithHTTPUpstreamPublicHostsOnly(context.Background()), newUpstreamSupplierBalanceHTTPClient(), server.URL, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "upstream host rejected by URL security policy")
}

func TestRequestNewAPISupplierJSONLimitsResponseSize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000},"padding":"` + strings.Repeat("x", upstreamBalanceResponseLimit) + `"}`))
	}))
	t.Cleanup(server.Close)

	_, err := requestNewAPISupplierJSON(context.Background(), newUpstreamSupplierBalanceHTTPClient(), server.URL, nil)

	require.Error(t, err)
	require.Contains(t, err.Error(), "too large")
}
