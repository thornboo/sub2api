//go:build unit

package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleAccountCapabilityMismatchExhaustedReturnsStableClientError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	h := &OpenAIGatewayHandler{}
	h.handleAccountCapabilityMismatchExhausted(c, &service.AccountCapabilityMismatchError{
		AccountID: 42,
		Feature:   "hosted_tool_search",
		Message:   "hosted tool_search requires a Responses-capable account",
	}, false)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{
		"error": {
			"type": "unsupported_feature",
			"message": "hosted tool_search requires a Responses-capable account"
		}
	}`, recorder.Body.String())
}

func TestHandleOpenAIAccountAttemptsExhaustedPrefersUpstreamFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	h := &OpenAIGatewayHandler{}
	h.handleOpenAIAccountAttemptsExhausted(
		c,
		&service.UpstreamFailoverError{StatusCode: http.StatusInternalServerError},
		&service.AccountCapabilityMismatchError{
			AccountID: 42,
			Feature:   "hosted_tool_search",
			Message:   "hosted tool_search requires a Responses-capable account",
		},
		false,
	)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.JSONEq(t, `{
		"error": {
			"type": "upstream_error",
			"message": "Upstream service temporarily unavailable"
		}
	}`, recorder.Body.String())
}
