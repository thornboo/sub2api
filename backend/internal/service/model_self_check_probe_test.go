package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayModelSelfCheckProbeExecutorOpenAIForwardPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name        string
		statusCode  int
		body        string
		upstreamErr error
		wantStatus  string
		wantCode    string
	}{
		{
			name:       "upstream 200 operational",
			statusCode: http.StatusOK,
			body:       `{"id":"chatcmpl_self_check","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			wantStatus: MonitorStatusOperational,
		},
		{
			name:       "upstream 401 config error",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"type":"invalid_request_error","message":"invalid api key"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 403 config error",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"type":"permission_error","message":"forbidden"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 429 degraded",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"type":"rate_limit_exceeded","message":"rate limited"}}`,
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
		{
			name:       "upstream 500 failed",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":{"type":"server_error","message":"upstream unavailable"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorUpstream,
		},
		{
			name:        "connection failure",
			upstreamErr: errors.New("dial tcp: connect: connection refused"),
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorConnection,
		},
		{
			name:        "timeout failure",
			upstreamErr: context.DeadlineExceeded,
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{err: tt.upstreamErr}
			if tt.upstreamErr == nil {
				upstream.resp = &http.Response{
					StatusCode: tt.statusCode,
					Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_self_check"}},
					Body:       io.NopCloser(strings.NewReader(tt.body)),
				}
			}
			executor := &gatewayModelSelfCheckProbeExecutor{
				openAIGatewayService: &OpenAIGatewayService{
					cfg:          modelSelfCheckProbeTestConfig(),
					httpUpstream: upstream,
				},
			}

			result := executor.Probe(context.Background(), modelSelfCheckOpenAITestAccount(), "gpt-4o")

			require.NotNil(t, upstream.lastReq)
			require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
			require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
			require.Equal(t, "gpt-4o", gjson.GetBytes(upstream.lastBody, "model").String())
			require.EqualValues(t, 1, gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
			require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, tt.wantCode, result.ErrorCode)
			require.NotNil(t, result.LatencyMs)
			if tt.upstreamErr == nil {
				require.NotNil(t, result.HTTPStatus)
				require.Equal(t, tt.statusCode, *result.HTTPStatus)
			}
		})
	}
}

func TestGatewayModelSelfCheckProbeExecutorAnthropicForwardPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []modelSelfCheckForwardCase{
		{
			name:       "upstream 200 operational",
			statusCode: http.StatusOK,
			body:       `{"id":"msg_self_check","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`,
			wantStatus: MonitorStatusOperational,
		},
		{
			name:       "upstream 401 config error",
			statusCode: http.StatusUnauthorized,
			body:       `{"type":"error","error":{"type":"authentication_error","message":"invalid api key"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 403 config error",
			statusCode: http.StatusForbidden,
			body:       `{"type":"error","error":{"type":"permission_error","message":"forbidden"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 429 degraded",
			statusCode: http.StatusTooManyRequests,
			body:       `{"type":"error","error":{"type":"rate_limit_error","message":"rate limited"}}`,
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
		{
			name:       "upstream 500 failed",
			statusCode: http.StatusInternalServerError,
			body:       `{"type":"error","error":{"type":"api_error","message":"upstream unavailable"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorUpstream,
		},
		{
			name:        "connection failure",
			upstreamErr: errors.New("dial tcp: connect: connection refused"),
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorConnection,
		},
		{
			name:        "timeout failure",
			upstreamErr: context.DeadlineExceeded,
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := modelSelfCheckUpstream(tt)
			executor := &gatewayModelSelfCheckProbeExecutor{
				gatewayService: &GatewayService{
					cfg:          modelSelfCheckProbeTestConfig(),
					httpUpstream: upstream,
				},
			}

			result := executor.Probe(context.Background(), modelSelfCheckAnthropicTestAccount(), "claude-3-5-sonnet-20241022")

			require.NotNil(t, upstream.lastReq)
			require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
			require.Equal(t, 1, len(upstream.requests))
			require.Equal(t, "http://upstream.example/v1/messages?beta=true", upstream.lastReq.URL.String())
			require.Equal(t, "claude-3-5-sonnet-20241022", gjson.GetBytes(upstream.lastBody, "model").String())
			require.EqualValues(t, 1, gjson.GetBytes(upstream.lastBody, "max_tokens").Int())
			require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, tt.wantCode, result.ErrorCode)
			require.NotNil(t, result.LatencyMs)
			if tt.upstreamErr == nil {
				require.NotNil(t, result.HTTPStatus)
				require.Equal(t, tt.statusCode, *result.HTTPStatus)
			}
		})
	}
}

func TestGatewayModelSelfCheckProbeExecutorGeminiForwardPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []modelSelfCheckForwardCase{
		{
			name:       "upstream 200 operational",
			statusCode: http.StatusOK,
			body:       `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}`,
			wantStatus: MonitorStatusOperational,
		},
		{
			name:       "upstream 401 config error",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"code":401,"status":"UNAUTHENTICATED","message":"invalid api key"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 403 config error",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"code":403,"status":"PERMISSION_DENIED","message":"forbidden"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 429 degraded",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"rate limited"}}`,
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
		{
			name:       "upstream 500 failed",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":{"code":500,"status":"INTERNAL","message":"upstream unavailable"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorUpstream,
		},
		{
			name:        "connection failure",
			upstreamErr: errors.New("dial tcp: connect: connection refused"),
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorConnection,
		},
		{
			name:        "timeout failure",
			upstreamErr: context.DeadlineExceeded,
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := modelSelfCheckUpstream(tt)
			repo := &modelSelfCheckRateLimitRepo{}
			executor := &gatewayModelSelfCheckProbeExecutor{
				geminiCompatService: &GeminiMessagesCompatService{
					cfg:              modelSelfCheckProbeTestConfig(),
					httpUpstream:     upstream,
					rateLimitService: NewRateLimitService(repo, nil, &config.Config{}, nil, nil),
				},
			}

			result := executor.Probe(context.Background(), modelSelfCheckGeminiTestAccount(), "gemini-2.5-flash")

			require.NotNil(t, upstream.lastReq)
			require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
			require.Equal(t, 1, len(upstream.requests))
			require.Equal(t, "http://upstream.example/v1beta/models/gemini-2.5-flash:generateContent", upstream.lastReq.URL.String())
			require.Contains(t, string(upstream.lastBody), "Reply with ok.")
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, tt.wantCode, result.ErrorCode)
			require.NotNil(t, result.LatencyMs)
			require.Zero(t, repo.setErrorCalls)
			require.Zero(t, repo.setRateLimitedCalls)
			require.Zero(t, repo.setModelRateLimitCalls)
			require.Zero(t, repo.setTempCalls)
			if tt.upstreamErr == nil {
				require.NotNil(t, result.HTTPStatus)
				require.Equal(t, tt.statusCode, *result.HTTPStatus)
			}
		})
	}
}

func TestGatewayModelSelfCheckProbeExecutorAntigravityForwardPath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []modelSelfCheckForwardCase{
		{
			name:       "upstream 200 operational",
			statusCode: http.StatusOK,
			body:       `data: {"response":{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}}` + "\n\n" + `data: [DONE]` + "\n\n",
			wantStatus: MonitorStatusOperational,
		},
		{
			name:       "upstream 401 config error",
			statusCode: http.StatusUnauthorized,
			body:       `{"error":{"code":401,"status":"UNAUTHENTICATED","message":"invalid token"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 403 config error",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"code":403,"status":"PERMISSION_DENIED","message":"forbidden"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorConfig,
		},
		{
			name:       "upstream 429 degraded",
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED","message":"rate limited"}}`,
			wantStatus: MonitorStatusDegraded,
			wantCode:   modelSelfCheckErrorRateLimit,
		},
		{
			name:       "upstream 500 failed",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":{"code":500,"status":"INTERNAL","message":"upstream unavailable"}}`,
			wantStatus: MonitorStatusFailed,
			wantCode:   modelSelfCheckErrorUpstream,
		},
		{
			name:        "connection failure",
			upstreamErr: errors.New("dial tcp: connect: connection refused"),
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorConnection,
		},
		{
			name:        "timeout failure",
			upstreamErr: context.DeadlineExceeded,
			wantStatus:  MonitorStatusFailed,
			wantCode:    modelSelfCheckErrorTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := modelSelfCheckUpstream(tt)
			repo := &stubAntigravityAccountRepo{}
			executor := &gatewayModelSelfCheckProbeExecutor{
				antigravityGatewayService: &AntigravityGatewayService{
					accountRepo:    repo,
					settingService: NewSettingService(&antigravitySettingRepoStub{}, &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}),
					tokenProvider:  &AntigravityTokenProvider{},
					httpUpstream:   upstream,
				},
			}

			result := executor.Probe(context.Background(), modelSelfCheckAntigravityTestAccount(), "claude-sonnet-4-5")

			require.NotNil(t, upstream.lastReq)
			require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
			require.Equal(t, 1, len(upstream.requests))
			require.Contains(t, upstream.lastReq.URL.String(), "streamGenerateContent")
			require.Contains(t, string(upstream.lastBody), "Reply with ok.")
			require.Equal(t, tt.wantStatus, result.Status)
			require.Equal(t, tt.wantCode, result.ErrorCode)
			require.NotNil(t, result.LatencyMs)
			require.Empty(t, repo.modelRateLimitCalls)
			require.Empty(t, repo.rateCalls)
			if tt.upstreamErr == nil {
				require.NotNil(t, result.HTTPStatus)
				require.Equal(t, tt.statusCode, *result.HTTPStatus)
			}
		})
	}
}

func TestModelSelfCheckGinContextCarriesProbeMarker(t *testing.T) {
	ctx := withModelSelfCheckProbeContext(context.Background())
	c, _ := newModelSelfCheckGinContext(ctx, "/v1/chat/completions", []byte(`{"model":"gpt-4o"}`))

	require.NotNil(t, c.Request)
	require.True(t, isModelSelfCheckProbeContext(c.Request.Context()))
	require.Equal(t, "sub2api-model-self-check/1.0", c.Request.Header.Get("User-Agent"))
}

func TestOpenAISelfCheck429DoesNotRuntimeBlockAccount(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 42, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	shouldDisable := svc.handleOpenAIAccountUpstreamError(
		withModelSelfCheckProbeContext(context.Background()),
		account,
		http.StatusTooManyRequests,
		http.Header{},
		[]byte(`{"error":{"type":"rate_limit_exceeded","message":"rate limited"}}`),
		"gpt-4o",
	)

	require.False(t, shouldDisable)
	require.False(t, svc.isOpenAIAccountRuntimeBlocked(account))
}

type modelSelfCheckForwardCase struct {
	name        string
	statusCode  int
	body        string
	upstreamErr error
	wantStatus  string
	wantCode    string
}

type modelSelfCheckRateLimitRepo struct {
	AccountRepository
	setErrorCalls          int
	setRateLimitedCalls    int
	setModelRateLimitCalls int
	setTempCalls           int
}

func (r *modelSelfCheckRateLimitRepo) SetError(_ context.Context, _ int64, _ string) error {
	r.setErrorCalls++
	return nil
}

func (r *modelSelfCheckRateLimitRepo) SetRateLimited(_ context.Context, _ int64, _ time.Time) error {
	r.setRateLimitedCalls++
	return nil
}

func (r *modelSelfCheckRateLimitRepo) SetModelRateLimit(_ context.Context, _ int64, _ string, _ time.Time, _ ...string) error {
	r.setModelRateLimitCalls++
	return nil
}

func (r *modelSelfCheckRateLimitRepo) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, _ string) error {
	r.setTempCalls++
	return nil
}

func modelSelfCheckUpstream(tt modelSelfCheckForwardCase) *httpUpstreamRecorder {
	upstream := &httpUpstreamRecorder{err: tt.upstreamErr}
	if tt.upstreamErr == nil {
		upstream.resp = &http.Response{
			StatusCode: tt.statusCode,
			Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_self_check"}},
			Body:       io.NopCloser(strings.NewReader(tt.body)),
		}
	}
	return upstream
}

func modelSelfCheckOpenAITestAccount() *Account {
	return &Account{
		ID:          1001,
		Name:        "model-self-check-openai",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-self-check",
			"base_url": "http://upstream.example",
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
}

func modelSelfCheckAnthropicTestAccount() *Account {
	return &Account{
		ID:          1002,
		Name:        "model-self-check-anthropic",
		Platform:    PlatformAnthropic,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "sk-ant-self-check",
			"base_url": "http://upstream.example",
		},
	}
}

func modelSelfCheckGeminiTestAccount() *Account {
	return &Account{
		ID:          1003,
		Name:        "model-self-check-gemini",
		Platform:    PlatformGemini,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "gemini-self-check",
			"base_url": "http://upstream.example",
		},
	}
}

func modelSelfCheckAntigravityTestAccount() *Account {
	return &Account{
		ID:          1004,
		Name:        "model-self-check-antigravity",
		Platform:    PlatformAntigravity,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"access_token": "ag-self-check",
			"project_id":   "ag-project-self-check",
			"model_mapping": map[string]any{
				"claude-sonnet-4-5": "gemini-3-pro-high",
			},
		},
	}
}

func modelSelfCheckProbeTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
			},
		},
	}
}
