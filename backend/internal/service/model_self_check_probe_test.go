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
		wantInput   int
		wantOutput  int
	}{
		{
			name:       "upstream 200 operational",
			statusCode: http.StatusOK,
			body:       `{"id":"chatcmpl_self_check","object":"chat.completion","model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
			wantStatus: MonitorStatusOperational,
			wantInput:  1,
			wantOutput: 1,
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
			require.Equal(t, tt.wantInput, result.InputTokens)
			require.Equal(t, tt.wantOutput, result.OutputTokens)
			require.NotNil(t, result.LatencyMs)
			if tt.upstreamErr == nil {
				require.NotNil(t, result.HTTPStatus)
				require.Equal(t, tt.statusCode, *result.HTTPStatus)
			}
		})
	}
}

func TestGatewayModelSelfCheckProbeExecutorOpenAIAutoUsesModelCapabilityDecision(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_self_check","object":"chat.completion","model":"deepseek-v4-pro","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		)),
	}}
	account := modelSelfCheckOpenAITestAccount()
	account.Extra = map[string]any{
		openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeAuto),
		openai_compat.ExtraKeyResponsesSupported: true,
	}
	capabilityRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		account.ID: {
			{
				UpstreamModel:  "deepseek-v4-pro",
				Protocol:       ModelProtocolOpenAIChat,
				ObservedState:  ModelProtocolStateSupported,
				ObservedSource: "upstream_model_list",
			},
			{
				UpstreamModel:  "deepseek-v4-pro",
				Protocol:       ModelProtocolOpenAIResponses,
				ObservedState:  ModelProtocolStateUnsupported,
				ObservedSource: "upstream_model_list",
			},
		},
	}}
	cfg := modelSelfCheckProbeTestConfig()
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	executor := &gatewayModelSelfCheckProbeExecutor{
		openAIGatewayService: &OpenAIGatewayService{
			cfg:                     cfg,
			httpUpstream:            upstream,
			modelProtocolCapability: &ModelProtocolCapabilityService{repo: capabilityRepo},
		},
	}

	result := executor.Probe(context.Background(), account, "deepseek-v4-pro")

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "deepseek-v4-pro", gjson.GetBytes(upstream.lastBody, "model").String())
}

func TestGatewayModelSelfCheckProbeExecutorGrokUsesOpenAICompatiblePathWithoutStateMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":                   []string{"application/json"},
			"X-Ratelimit-Limit-Requests":     []string{"10"},
			"X-Ratelimit-Remaining-Requests": []string{"9"},
		},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_self_check","object":"chat.completion","model":"grok-4.5","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`,
		)),
	}}
	repo := &modelSelfCheckGrokAccountRepo{}
	executor := &gatewayModelSelfCheckProbeExecutor{
		openAIGatewayService: &OpenAIGatewayService{
			cfg:          modelSelfCheckProbeTestConfig(),
			httpUpstream: upstream,
			accountRepo:  repo,
		},
	}
	account := modelSelfCheckGrokTestAccount()

	result := executor.Probe(context.Background(), account, "grok-4.5")

	require.Equal(t, MonitorStatusOperational, result.Status)
	require.Empty(t, result.ErrorCode)
	require.NotNil(t, upstream.lastReq)
	require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
	require.Equal(t, "http://upstream.example/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "grok-4.5", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, 1, result.InputTokens)
	require.Equal(t, 1, result.OutputTokens)
	require.Zero(t, repo.updateCalls)
	require.Zero(t, repo.rateLimitedCalls)
	require.Zero(t, repo.modelRateLimitedCalls)
	require.Zero(t, repo.tempUnschedCalls)
	require.Zero(t, repo.recoveryClearCalls)
	require.False(t, executor.openAIGatewayService.isOpenAIAccountRuntimeBlocked(account))
}

func TestGatewayModelSelfCheckProbeExecutorGrokTransportErrorDoesNotUnscheduleAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{err: errors.New("dial tcp: connection refused")}
	repo := &modelSelfCheckGrokAccountRepo{}
	executor := &gatewayModelSelfCheckProbeExecutor{
		openAIGatewayService: &OpenAIGatewayService{
			cfg:          modelSelfCheckProbeTestConfig(),
			httpUpstream: upstream,
			accountRepo:  repo,
		},
	}
	account := modelSelfCheckGrokTestAccount()

	result := executor.Probe(context.Background(), account, "grok-4.5")

	require.Equal(t, MonitorStatusFailed, result.Status)
	require.NotNil(t, upstream.lastReq)
	require.True(t, isModelSelfCheckProbeContext(upstream.lastReq.Context()))
	require.Zero(t, repo.updateCalls)
	require.Zero(t, repo.rateLimitedCalls)
	require.Zero(t, repo.modelRateLimitedCalls)
	require.Zero(t, repo.tempUnschedCalls)
	require.Zero(t, repo.recoveryClearCalls)
	require.False(t, executor.openAIGatewayService.isOpenAIAccountRuntimeBlocked(account))
}

func TestOpenAISelfCheckUpstreamProtocolUsesExactModelCapability(t *testing.T) {
	for _, test := range []struct {
		name          string
		responsesMode openai_compat.ResponsesSupportMode
		capabilities  []AccountModelProtocolCapability
		wantProtocol  ModelProtocol
		wantStrict    bool
	}{
		{
			name:          "responses capability overrides force chat preference",
			responsesMode: openai_compat.ResponsesSupportModeForceChatCompletions,
			capabilities: []AccountModelProtocolCapability{{
				UpstreamModel: "deepseek-v4-pro",
				Protocol:      ModelProtocolOpenAIResponses,
				OverrideState: ModelProtocolStateSupported,
			}},
			wantProtocol: ModelProtocolOpenAIResponses,
			wantStrict:   true,
		},
		{
			name:          "messages capability overrides force responses preference",
			responsesMode: openai_compat.ResponsesSupportModeForceResponses,
			capabilities: []AccountModelProtocolCapability{{
				UpstreamModel: "deepseek-v4-pro",
				Protocol:      ModelProtocolAnthropicMessages,
				OverrideState: ModelProtocolStateSupported,
			}},
			wantProtocol: ModelProtocolAnthropicMessages,
			wantStrict:   true,
		},
		{
			name:          "unknown capabilities fail closed in strict mode",
			responsesMode: openai_compat.ResponsesSupportModeAuto,
			wantStrict:    true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			account := modelSelfCheckOpenAITestAccount()
			account.Extra = map[string]any{
				openai_compat.ExtraKeyResponsesMode: string(test.responsesMode),
			}
			cfg := modelSelfCheckProbeTestConfig()
			cfg.Gateway.NativeModelProtocolRoutingEnabled = true
			executor := &gatewayModelSelfCheckProbeExecutor{
				openAIGatewayService: &OpenAIGatewayService{
					cfg: cfg,
					modelProtocolCapability: &ModelProtocolCapabilityService{
						repo: &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
							account.ID: test.capabilities,
						}},
					},
				},
			}

			protocol, strict := executor.openAISelfCheckUpstreamProtocol(
				context.Background(),
				account,
				"deepseek-v4-pro",
			)
			require.Equal(t, test.wantProtocol, protocol)
			require.Equal(t, test.wantStrict, strict)
		})
	}
}

func TestGatewayModelSelfCheckProbeExecutorStrictUnknownDoesNotUseLegacyPreference(t *testing.T) {
	account := modelSelfCheckOpenAITestAccount()
	account.Extra = map[string]any{
		openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
	}
	upstream := &httpUpstreamRecorder{}
	cfg := modelSelfCheckProbeTestConfig()
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	executor := &gatewayModelSelfCheckProbeExecutor{
		openAIGatewayService: &OpenAIGatewayService{
			cfg:                     cfg,
			httpUpstream:            upstream,
			modelProtocolCapability: &ModelProtocolCapabilityService{repo: &modelProtocolCapabilityRepoStub{}},
		},
	}

	result := executor.Probe(context.Background(), account, "deepseek-v4-pro")

	require.Equal(t, MonitorStatusFailed, result.Status)
	require.Nil(t, upstream.lastReq)
}

func TestParseModelSelfCheckTokenUsageBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantInput  int
		wantOutput int
	}{
		{
			name:       "openai chat completions usage",
			body:       `{"usage":{"prompt_tokens":12,"completion_tokens":1,"total_tokens":13}}`,
			wantInput:  12,
			wantOutput: 1,
		},
		{
			name:       "anthropic messages usage",
			body:       `{"usage":{"input_tokens":10,"output_tokens":1}}`,
			wantInput:  10,
			wantOutput: 1,
		},
		{
			name:       "gemini usage metadata",
			body:       `{"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":1}}`,
			wantInput:  8,
			wantOutput: 1,
		},
		{
			name:       "antigravity sse usage metadata",
			body:       "data: {\"response\":{\"usageMetadata\":{\"promptTokenCount\":9,\"candidatesTokenCount\":1}}}\n\ndata: [DONE]\n\n",
			wantInput:  9,
			wantOutput: 1,
		},
		{
			name: "invalid body",
			body: `not-json`,
		},
		{
			name: "missing usage",
			body: `{"id":"msg_self_check","content":[]}`,
		},
		{
			name: "negative usage clamps to zero",
			body: `{"usage":{"input_tokens":-1,"output_tokens":-2}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseModelSelfCheckTokenUsageBody([]byte(tt.body))
			require.Equal(t, tt.wantInput, got.InputTokens)
			require.Equal(t, tt.wantOutput, got.OutputTokens)
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
			wantInput:  1,
			wantOutput: 1,
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
			require.Equal(t, tt.wantInput, result.InputTokens)
			require.Equal(t, tt.wantOutput, result.OutputTokens)
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
	wantInput   int
	wantOutput  int
}

// modelSelfCheckGrokAccountRepo records every account-state mutation the Grok
// self-check path could perform. Embedding AccountRepository keeps this probe
// focused on mutation behavior without depending on unit-tag-only test stubs.
type modelSelfCheckGrokAccountRepo struct {
	AccountRepository
	updateCalls           int
	rateLimitedCalls      int
	modelRateLimitedCalls int
	tempUnschedCalls      int
	recoveryClearCalls    int
}

func (r *modelSelfCheckGrokAccountRepo) UpdateExtra(_ context.Context, _ int64, _ map[string]any) error {
	r.updateCalls++
	return nil
}

func (r *modelSelfCheckGrokAccountRepo) SetRateLimited(_ context.Context, _ int64, _ time.Time) error {
	r.rateLimitedCalls++
	return nil
}

func (r *modelSelfCheckGrokAccountRepo) SetRateLimitedIfLater(ctx context.Context, id int64, resetAt time.Time) error {
	return r.SetRateLimited(ctx, id, resetAt)
}

func (r *modelSelfCheckGrokAccountRepo) SetModelRateLimit(_ context.Context, _ int64, _ string, _ time.Time, _ ...string) error {
	r.modelRateLimitedCalls++
	return nil
}

func (r *modelSelfCheckGrokAccountRepo) SetTempUnschedulable(_ context.Context, _ int64, _ time.Time, _ string) error {
	r.tempUnschedCalls++
	return nil
}

func (r *modelSelfCheckGrokAccountRepo) ClearRateLimitIfObserved(_ context.Context, _ int64, _, _ time.Time) (bool, error) {
	r.recoveryClearCalls++
	return false, nil
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

func modelSelfCheckGrokTestAccount() *Account {
	return &Account{
		ID:          1005,
		Name:        "model-self-check-grok",
		Platform:    PlatformGrok,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":  "xai-self-check",
			"base_url": "http://upstream.example",
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
