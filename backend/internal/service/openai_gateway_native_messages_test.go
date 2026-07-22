package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardNativeAnthropicMessagesPreservesProtocolAndRewritesModel(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"req_native"}},
		Body: io.NopCloser(strings.NewReader(`{
			"id":"msg_1","type":"message","role":"assistant","model":"MiniMax-M3-upstream",
			"content":[{"type":"text","text":"ok"}],
			"usage":{"input_tokens":11,"output_tokens":3,"cache_read_input_tokens":2}
		}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID: 21, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 2,
		Credentials: map[string]any{
			"api_key":       "upstream-secret",
			"base_url":      "https://new-api.example.com/v1",
			"model_mapping": map[string]any{"MiniMax-M3": "MiniMax-M3-upstream"},
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"MiniMax-M3","stream":false,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`))
	c.Request.Header.Set("anthropic-beta", "tools-2025-01-01")

	result, err := svc.ForwardNativeAnthropicMessages(context.Background(), c, account, []byte(`{"model":"MiniMax-M3","stream":false,"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`), "MiniMax-M3")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "https://new-api.example.com/v1/messages", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer upstream-secret", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "tools-2025-01-01", upstream.lastReq.Header.Get("anthropic-beta"))
	require.Equal(t, "MiniMax-M3-upstream", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "MiniMax-M3", gjson.Get(w.Body.String(), "model").String())
	require.Equal(t, "/v1/messages", result.UpstreamEndpoint)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, 2, result.Usage.CacheReadInputTokens)
}

func TestForwardNativeAnthropicMessagesClassifiesMissingEndpointWithoutWritingResponse(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"route not found"}}`)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID: 22, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "secret", "base_url": "https://new-api.example.com"},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	result, err := svc.ForwardNativeAnthropicMessages(context.Background(), c, account, []byte(`{"model":"MiniMax-M3","stream":false,"messages":[]}`), "MiniMax-M3")
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.True(t, failoverErr.IsNativeProtocolUnavailable())
	require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
	require.Empty(t, w.Body.String())
}

func TestForwardNativeAnthropicMessagesClassifiesUnsupportedEndpointText(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	for _, status := range []int{http.StatusBadRequest, http.StatusNotImplemented} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"This endpoint is not supported by the upstream"}}`)),
			}}
			svc := &OpenAIGatewayService{
				cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
				httpUpstream: upstream,
			}
			account := &Account{
				ID: 22, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
				Credentials: map[string]any{"api_key": "secret", "base_url": "https://new-api.example.com"},
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

			result, err := svc.ForwardNativeAnthropicMessages(context.Background(), c, account, []byte(`{"model":"MiniMax-M3","stream":false,"messages":[]}`), "MiniMax-M3")
			require.Nil(t, result)
			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.True(t, failoverErr.IsNativeProtocolUnavailable())
			require.Equal(t, status, failoverErr.StatusCode)
			require.Empty(t, w.Body.String())
		})
	}
}

func TestRewriteNativeAnthropicSSEModel(t *testing.T) {
	t.Parallel()
	line := `data: {"type":"message_start","message":{"id":"msg_1","model":"MiniMax-M3-upstream"}}`
	rewritten := rewriteNativeAnthropicSSEModel(line, strings.TrimPrefix(line, "data: "), "MiniMax-M3", "MiniMax-M3-upstream")
	require.Equal(t, "MiniMax-M3", gjson.Get(strings.TrimPrefix(rewritten, "data: "), "message.model").String())
}

func TestStreamNativeAnthropicMessagesIncompleteBeforeWriteCanFailOver(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}

	result, err := svc.streamNativeAnthropicMessages(c, resp, "MiniMax-M3", "MiniMax-M3", "MiniMax-M3", time.Now())
	require.NotNil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, w.Body.String())
}

func TestStreamNativeAnthropicMessagesUpstreamErrorIsAlreadyTerminated(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	stream := "event: error\n" +
		`data: {"type":"error","error":{"type":"api_error","message":"upstream failed"}}` + "\n\n"
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(stream)),
	}

	result, err := svc.streamNativeAnthropicMessages(c, resp, "MiniMax-M3", "MiniMax-M3", "MiniMax-M3", time.Now())
	require.NotNil(t, result)
	require.ErrorIs(t, err, ErrNativeAnthropicStreamErrorForwarded)
	require.Equal(t, 1, strings.Count(w.Body.String(), "event: error"))
	require.Equal(t, 1, strings.Count(w.Body.String(), `"type":"error"`))
}
