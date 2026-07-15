package service

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsGroupRetryReasonRequiresExplicitRetryableFailure(t *testing.T) {
	for _, statusCode := range []int{
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	} {
		reason, ok := OpsGroupRetryReasonForStatus(statusCode)
		require.False(t, ok, "status %d alone cannot prove that replay is safe", statusCode)
		require.Empty(t, reason)
	}

	reason, ok := OpsGroupRetryReasonForFailoverError(&UpstreamFailoverError{StatusCode: http.StatusTooManyRequests})
	require.True(t, ok)
	require.Equal(t, OpsGroupRetryReasonCapacityExhausted, reason)

	reason, ok = OpsGroupRetryReasonForFailoverError(&UpstreamFailoverError{
		StatusCode: http.StatusBadGateway,
		Stage:      GatewayFailureStageAccountAuth,
	})
	require.True(t, ok)
	require.Equal(t, OpsGroupRetryReasonCapacityExhausted, reason)
}

func TestSafeUpstreamURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"strips query", "https://api.anthropic.com/v1/messages?beta=true", "https://api.anthropic.com/v1/messages"},
		{"strips fragment", "https://api.openai.com/v1/responses#frag", "https://api.openai.com/v1/responses"},
		{"strips both", "https://host/path?token=secret#x", "https://host/path"},
		{"no query or fragment", "https://host/path", "https://host/path"},
		{"empty string", "", ""},
		{"whitespace only", "  ", ""},
		{"query before fragment", "https://h/p?a=1#f", "https://h/p"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, safeUpstreamURL(tt.input))
		})
	}
}
