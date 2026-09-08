package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const selfCheckCompletedSSE = `data: {"type":"response.completed","response":{"id":"resp_test","model":"gpt-5.6-terra","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}],"usage":{"input_tokens":3,"output_tokens":1,"total_tokens":4}}}` + "\n\n"

func selfCheckResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestSelfCheckResponsesSessionsDoNotStickAcrossRounds(t *testing.T) {
	var keys, sessions []string
	upstream := &codexModelsHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		require.NoError(t, err)
		key := gjson.GetBytes(body, "prompt_cache_key").String()
		require.NotEmpty(t, key)
		require.Equal(t, "gpt-5.6-terra", gjson.GetBytes(body, "model").String())
		require.Equal(t, "/v1/responses", req.URL.Path)
		require.True(t, gjson.GetBytes(body, "stream").Bool())
		keys = append(keys, key)
		sessions = append(sessions, req.Header.Get("session_id"))
		// An upstream sticky bucket fails on reuse: independent checks must not
		// revisit it simply because their prompt is identical.
		if len(keys) > 1 && keys[len(keys)-2] == key {
			return selfCheckResponse(503, `{"error":{"message":"Servers are overloaded"}}`), nil
		}
		return selfCheckResponse(200, selfCheckCompletedSSE), nil
	}}
	executor := &gatewayModelSelfCheckProbeExecutor{openAIGatewayService: &OpenAIGatewayService{cfg: modelSelfCheckProbeTestConfig(), httpUpstream: upstream}}
	account := modelSelfCheckOpenAITestAccount()
	account.Extra = map[string]any{"openai_responses_supported": true}
	for range 2 {
		result := executor.Probe(context.Background(), account, "gpt-5.6-terra")
		require.Equal(t, MonitorStatusOperational, result.Status)
	}
	require.Len(t, keys, 2)
	require.NotEqual(t, keys[0], keys[1])
	require.NotEmpty(t, sessions[0])
	require.NotEqual(t, sessions[0], sessions[1])
}

func TestSelfCheckHTTPRetryIsBoundedAndKeepsSession(t *testing.T) {
	for _, tc := range []struct {
		name          string
		firstStatus   int
		firstBody     string
		repeat        bool
		wantCalls     int
		wantRecovered bool
	}{
		{"explicit overload recovers", 503, `{"error":{"message":"Servers are overloaded"}}`, false, 2, true},
		{"persistent overload stops", 503, `{"error":{"message":"Servers are overloaded"}}`, true, 2, false},
		{"authentication is not retried", 401, `{"error":{"message":"Invalid API key"}}`, true, 1, false},
		{"payment is not retried", 402, `{"error":{"message":"Payment required"}}`, true, 1, false},
		{"model missing is not retried", 404, `{"error":{"message":"Model not found"}}`, true, 1, false},
		{"stream error is not replayed", 200, "data: {\"type\":\"error\",\"error\":{\"type\":\"server_error\",\"message\":\"Servers are overloaded\"}}\n\n", true, 1, false},
		{"missing terminal is not replayed", 200, "data: {\"type\":\"response.created\"}\n\n", true, 1, false},
		{"incomplete terminal is not success", 200, "data: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"r\",\"status\":\"incomplete\",\"output\":[]}}\n\n", true, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var keys []string
			upstream := &codexModelsHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				keys = append(keys, gjson.GetBytes(body, "prompt_cache_key").String())
				if len(keys) == 1 || tc.repeat {
					return selfCheckResponse(tc.firstStatus, tc.firstBody), nil
				}
				return selfCheckResponse(200, selfCheckCompletedSSE), nil
			}}
			executor := &gatewayModelSelfCheckProbeExecutor{openAIGatewayService: &OpenAIGatewayService{cfg: modelSelfCheckProbeTestConfig(), httpUpstream: upstream}}
			account := modelSelfCheckOpenAITestAccount()
			account.Extra = map[string]any{"openai_responses_supported": true}
			result := executor.Probe(context.Background(), account, "gpt-5.6-terra")
			require.Len(t, keys, tc.wantCalls)
			require.Equal(t, tc.wantRecovered, result.Recovered)
			if tc.wantRecovered {
				require.Equal(t, MonitorStatusDegraded, result.Status)
				require.Equal(t, "retry_succeeded", result.ErrorCode)
				require.Equal(t, 1, result.RetryCount)
				require.Equal(t, 503, *result.InitialHTTPStatus)
			} else {
				require.NotEqual(t, MonitorStatusOperational, result.Status)
			}
			if len(keys) == 2 {
				require.Equal(t, keys[0], keys[1])
			}
		})
	}
}

func TestSelfCheckRetryRespectsDeadlineAndDisabledRetries(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "deadline", true: "disabled"}[disabled], func(t *testing.T) {
			calls := 0
			upstream := &codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
				calls++
				return selfCheckResponse(503, `{"error":{"message":"Servers are overloaded"}}`), nil
			}}
			executor := &gatewayModelSelfCheckProbeExecutor{openAIGatewayService: &OpenAIGatewayService{cfg: modelSelfCheckProbeTestConfig(), httpUpstream: upstream}}
			account := modelSelfCheckOpenAITestAccount()
			account.Extra = map[string]any{"openai_responses_supported": true}
			ctx := context.Background()
			if disabled {
				account.Credentials["pool_mode"] = true
				account.Credentials["pool_mode_retry_count"] = 0
			} else {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
			}
			result := executor.Probe(ctx, account, "gpt-5.6-terra")
			require.Equal(t, 1, calls)
			require.False(t, result.Recovered)
			require.Zero(t, result.RetryCount)
		})
	}
}

func TestSelfCheckUpstreamCancellationIsPreserved(t *testing.T) {
	for _, selfCheck := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		if selfCheck {
			ctx = withModelSelfCheckProbeContext(ctx)
		}
		upstream, release := detachUpstreamContext(ctx)
		stream, releaseStream := detachStreamUpstreamContext(ctx, true)
		cancel()
		for _, forwarded := range []context.Context{upstream, stream} {
			_, bounded := forwarded.Deadline()
			require.Equal(t, selfCheck, bounded)
			if selfCheck {
				require.ErrorIs(t, forwarded.Err(), context.Canceled)
			} else {
				require.NoError(t, forwarded.Err())
			}
		}
		release()
		releaseStream()
	}
}

func TestSelfCheckTransportStopsAtAttemptDeadline(t *testing.T) {
	calls := 0
	upstream := &codexModelsHTTPUpstreamStub{do: func(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		calls++
		_, bounded := req.Context().Deadline()
		require.True(t, bounded)
		<-req.Context().Done()
		return nil, req.Context().Err()
	}}
	executor := &gatewayModelSelfCheckProbeExecutor{openAIGatewayService: &OpenAIGatewayService{cfg: modelSelfCheckProbeTestConfig(), httpUpstream: upstream}}
	account := modelSelfCheckOpenAITestAccount()
	account.Extra = map[string]any{"openai_responses_supported": true}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := executor.Probe(ctx, account, "gpt-5.6-terra")
	require.Equal(t, 1, calls)
	require.NotEqual(t, MonitorStatusOperational, result.Status)
	require.Zero(t, result.RetryCount)
}
