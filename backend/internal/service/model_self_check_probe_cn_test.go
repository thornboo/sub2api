//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayModelSelfCheckProbeExecutorCNProviderForwardPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		account    *Account
		model      string
		body       string
		wantURL    string
		wantStatus string
		wantCode   string
	}{
		{
			name:       "zhipu chat completions",
			account:    modelSelfCheckCNTestAccount(PlatformZhipu, APIProtocolChatCompletions, "http://upstream.example"),
			model:      "glm-4.7",
			body:       `{"id":"chatcmpl_self_check","object":"chat.completion","model":"glm-4.7","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			wantURL:    "http://upstream.example/v1/chat/completions",
			wantStatus: MonitorStatusOperational,
		},
		{
			name:       "kimi responses",
			account:    modelSelfCheckCNTestAccount(PlatformKimi, APIProtocolResponses, "http://upstream.example/v1"),
			model:      "k3-256k",
			body:       `{"error":{"type":"rate_limit_error","message":"rate limited"}}`,
			wantURL:    "http://upstream.example/v1/responses",
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
		{
			name:       "deepseek responses uses platform path",
			account:    modelSelfCheckCNTestAccount(PlatformDeepseek, APIProtocolResponses, "http://upstream.example"),
			model:      "deepseek-v4-pro",
			body:       `{"error":{"type":"rate_limit_error","message":"rate limited"}}`,
			wantURL:    "http://upstream.example/responses",
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
		{
			name:       "kimi anthropic protocol non 200",
			account:    modelSelfCheckCNTestAccount(PlatformKimi, APIProtocolAnthropic, "http://upstream.example/anthropic"),
			model:      "k3",
			body:       `{"error":{"type":"rate_limit_error","message":"rate limited"}}`,
			wantURL:    "http://upstream.example/anthropic/v1/messages",
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusCode := http.StatusOK
			if tt.wantCode != "" {
				statusCode = http.StatusTooManyRequests
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_self_check"}},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}}
			repo := &modelSelfCheckRateLimitRepo{}
			executor := &gatewayModelSelfCheckProbeExecutor{
				openAIGatewayService: &OpenAIGatewayService{
					cfg:          modelSelfCheckProbeTestConfig(),
					httpUpstream: upstream,
					accountRepo:  repo,
				},
			}

			result := executor.Probe(context.Background(), tt.account, tt.model)

			require.NotNil(t, upstream.lastReq)
			require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
			require.Equal(t, tt.wantURL, upstream.lastReq.URL.String())
			require.Equal(t, tt.model, gjson.GetBytes(upstream.lastBody, "model").String())
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, tt.wantCode, result.ErrorCode)
			require.Zero(t, repo.setErrorCalls)
			require.Zero(t, repo.setRateLimitedCalls)
			require.Zero(t, repo.setModelRateLimitCalls)
			require.Zero(t, repo.setTempCalls)
		})
	}
}

func modelSelfCheckCNTestAccount(platform string, protocol string, baseURL string) *Account {
	return &Account{
		ID:          1101,
		Name:        "model-self-check-cn",
		Platform:    platform,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":      "sk-cn-self-check",
			"api_protocol": protocol,
			"base_url":     baseURL,
		},
	}
}
