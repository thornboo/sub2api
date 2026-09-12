package service

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetchOpenAIAccountModelsDisplayNames(t *testing.T) {
	gateway := newCodexModelsAPIKeyTestService(&codexModelsHTTPUpstreamStub{do: func(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
		return ordinaryModelsUpstreamResponse(`{"data":[{"id":"deepseek-chat"},{"id":"deepseek-reasoner","display_name":""},{"id":"blank-name","display_name":"  "},{"id":"named-model","display_name":"Custom Model"}]}`), nil
	}})
	svc := &AccountTestService{openaiGatewayService: gateway}
	account := newCodexModelsAPIKeyTestAccount("https://models.example/v1")

	models, err := svc.FetchOpenAIAccountModels(context.Background(), account)
	require.NoError(t, err)
	require.Len(t, models, 4)
	for i, expected := range []string{"deepseek-chat", "deepseek-reasoner", "blank-name", "Custom Model"} {
		require.Equal(t, expected, models[i].DisplayName)
	}
	require.Equal(t, "deepseek-chat", models[0].ID)
	require.Equal(t, "named-model", models[3].ID)

	// Picker labels must not rewrite the shared public model catalog.
	raw, err := gateway.FetchOpenAIModelsList(context.Background(), account)
	require.NoError(t, err)
	require.NotContains(t, string(raw.Body), `"display_name":"deepseek-chat"`)
}

func TestFetchOpenAIAccountModelsOAuthDisplayNames(t *testing.T) {
	newCodexModelsOAuthCacheServer(t, `{"models":[{"slug":"oauth-model"}]}`)
	gateway := &OpenAIGatewayService{}
	svc := &AccountTestService{openaiGatewayService: gateway}
	account := newCodexModelsTestAccount()
	models, err := svc.FetchOpenAIAccountModels(context.Background(), account)
	require.NoError(t, err)
	wantIDs := []string{"oauth-model", "gpt-image-1", "gpt-image-1.5", "gpt-image-2", "gpt-image-2.5-flare", "gpt-image-2.5-sunburst"}
	require.Len(t, models, len(wantIDs))
	for i, id := range wantIDs {
		require.Equal(t, id, models[i].ID)
		require.NotEmpty(t, models[i].DisplayName)
	}
	require.Equal(t, "oauth-model", models[0].ID)
	require.Equal(t, "oauth-model", models[0].DisplayName)
	raw, err := gateway.FetchOpenAIModelsList(context.Background(), account)
	require.NoError(t, err)
	require.NotContains(t, string(raw.Body), "gpt-image-", "picker-only image choices must not mutate the shared discovery cache")
}
