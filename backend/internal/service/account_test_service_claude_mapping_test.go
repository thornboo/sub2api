package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type claudeMappingHTTPUpstream struct {
	lastBody []byte
}

func (u *claudeMappingHTTPUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	if req != nil && req.Body != nil {
		body, _ := io.ReadAll(req.Body)
		u.lastBody = body
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("data: [DONE]\n\n")),
	}, nil
}

func (u *claudeMappingHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestClaudeAccountConnectionUsesMappedModelInRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tt := range []struct {
		name        string
		modelID     string
		mapping     map[string]any
		wantRequest string
	}{
		{
			name:        "same name",
			modelID:     "same-model",
			mapping:     map[string]any{"same-model": "same-model"},
			wantRequest: "same-model",
		},
		{
			name:        "single hop only",
			modelID:     "a-model",
			mapping:     map[string]any{"a-model": "b-model", "b-model": "c-model"},
			wantRequest: "b-model",
		},
		{
			name:        "wildcard to fixed target",
			modelID:     "claude-custom-20260909",
			mapping:     map[string]any{"claude-custom-*": "fixed-upstream"},
			wantRequest: "fixed-upstream",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &claudeMappingHTTPUpstream{}
			svc := NewAccountTestService(
				nil,
				nil,
				nil,
				nil,
				nil,
				upstream,
				&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
				nil,
			)
			account := &Account{
				ID:          901,
				Name:        "claude-apikey",
				Platform:    PlatformAnthropic,
				Type:        AccountTypeAPIKey,
				Concurrency: 1,
				Credentials: map[string]any{
					"api_key":       "test-key",
					"base_url":      "https://api.anthropic.com",
					"model_mapping": tt.mapping,
				},
			}

			rec := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(rec)
			ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test", nil)

			err := svc.testClaudeAccountConnection(ctx, account, tt.modelID)

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.NotEmpty(t, upstream.lastBody)
			var payload struct {
				Model string `json:"model"`
			}
			require.NoError(t, json.Unmarshal(upstream.lastBody, &payload))
			require.Equal(t, tt.wantRequest, payload.Model)
		})
	}
}
