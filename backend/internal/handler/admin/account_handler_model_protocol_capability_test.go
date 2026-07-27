package admin

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelProtocolCapabilityResponseScopesItemsToAccountMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	handler := &AccountHandler{}
	account := &service.Account{
		ID:       7,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"minimax-m2.5": "minimax-m2.5",
				"minimax-m2.7": "MiniMax-M2.7",
			},
		},
	}
	items := []service.AccountModelProtocolCapability{
		{UpstreamModel: service.ModelProtocolWildcardModel, Protocol: service.ModelProtocolAnthropicMessages},
		{UpstreamModel: "glm-5", Protocol: service.ModelProtocolAnthropicMessages},
		{UpstreamModel: "minimax-m2.5", Protocol: service.ModelProtocolAnthropicMessages},
		{UpstreamModel: "MiniMax-M2.7", Protocol: service.ModelProtocolOpenAIChat},
	}

	payload := handler.modelProtocolCapabilityResponse(ctx, account, items, nil)

	require.Equal(t, int64(7), payload["account_id"])
	require.Equal(t, true, payload["mapping_restricted"])
	require.Equal(t, []string{"MiniMax-M2.7", "minimax-m2.5"}, payload["models"])
	require.Equal(t, []service.AccountModelProtocolCapability{
		{UpstreamModel: service.ModelProtocolWildcardModel, Protocol: service.ModelProtocolAnthropicMessages},
		{UpstreamModel: "minimax-m2.5", Protocol: service.ModelProtocolAnthropicMessages},
		{UpstreamModel: "MiniMax-M2.7", Protocol: service.ModelProtocolOpenAIChat},
	}, payload["items"])
}
