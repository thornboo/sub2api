package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

func TestSyncModelProtocolCapabilitiesReturnsCurrentSyncedObservations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := service.Account{
		ID:       44,
		Name:     "openai",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"api_key":  "key",
			"base_url": "https://provider.example/v1",
			"model_mapping": map[string]any{
				"public-a": "matched-model",
				"public-b": "missing-model",
			},
		},
	}
	adminSvc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: account}
	upstream := &syncUpstreamHTTPUpstream{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"data":[
			{"id":"matched-model","supported_endpoint_types":["openai"]},
			{"id":"unmapped-model","supported_endpoint_types":["anthropic"]}
		]}`)),
	}}
	repo := &modelProtocolHandlerCapabilityRepoStub{items: []service.AccountModelProtocolCapability{
		{
			AccountID:      account.ID,
			UpstreamModel:  "historic-only",
			Protocol:       service.ModelProtocolOpenAIChat,
			ObservedState:  service.ModelProtocolStateSupported,
			ObservedSource: "upstream_model_list",
		},
	}}
	accountTestSvc := service.NewAccountTestService(
		nil,
		nil,
		nil,
		nil,
		nil,
		upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		nil,
	)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil, nil)
	handler.SetModelProtocolCapabilityService(service.NewModelProtocolCapabilityService(repo, nil, nil, nil, nil))
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/model-protocol-capabilities/sync", handler.SyncModelProtocolCapabilities)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/44/model-protocol-capabilities/sync", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data struct {
			Models             []string                           `json:"models"`
			SyncedObservations []service.ModelProtocolObservation `json:"synced_observations"`
			Items              []service.AccountModelProtocolCapability
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, []string{"matched-model", "missing-model"}, resp.Data.Models)
	require.NotNil(t, resp.Data.SyncedObservations)
	require.Equal(t, repo.observations, resp.Data.SyncedObservations)

	states := make(map[string]service.ModelProtocolState)
	for _, observation := range resp.Data.SyncedObservations {
		require.Equal(t, "matched-model", observation.UpstreamModel)
		states[string(observation.Protocol)] = observation.State
	}
	require.Equal(t, service.ModelProtocolStateSupported, states[string(service.ModelProtocolOpenAIChat)])
	require.Equal(t, service.ModelProtocolStateUnsupported, states[string(service.ModelProtocolAnthropicMessages)])
	require.NotContains(t, rec.Body.String(), "historic-only")
	require.NotContains(t, rec.Body.String(), "unmapped-model")
}

func TestSyncModelProtocolCapabilitiesReturnsEmptySyncedObservations(t *testing.T) {
	gin.SetMode(gin.TestMode)
	account := service.Account{
		ID:          45,
		Name:        "openai",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Credentials: map[string]any{"api_key": "key", "base_url": "https://provider.example/v1"},
	}
	adminSvc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: account}
	upstream := &syncUpstreamHTTPUpstream{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"*","supported_endpoint_types":["openai"]}]}`)),
	}}
	repo := &modelProtocolHandlerCapabilityRepoStub{}
	accountTestSvc := service.NewAccountTestService(
		nil,
		nil,
		nil,
		nil,
		nil,
		upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		nil,
	)
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil, nil)
	handler.SetModelProtocolCapabilityService(service.NewModelProtocolCapabilityService(repo, nil, nil, nil, nil))
	router := gin.New()
	router.POST("/api/v1/admin/accounts/:id/model-protocol-capabilities/sync", handler.SyncModelProtocolCapabilities)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/accounts/45/model-protocol-capabilities/sync", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data struct {
			SyncedObservations []service.ModelProtocolObservation `json:"synced_observations"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Data.SyncedObservations)
	require.Empty(t, resp.Data.SyncedObservations)
}

type modelProtocolHandlerCapabilityRepoStub struct {
	items        []service.AccountModelProtocolCapability
	observations []service.ModelProtocolObservation
}

func (r *modelProtocolHandlerCapabilityRepoStub) ListByAccount(_ context.Context, _ int64) ([]service.AccountModelProtocolCapability, error) {
	return append([]service.AccountModelProtocolCapability(nil), r.items...), nil
}

func (r *modelProtocolHandlerCapabilityRepoStub) ListByAccountIDs(_ context.Context, accountIDs []int64) (map[int64][]service.AccountModelProtocolCapability, error) {
	result := make(map[int64][]service.AccountModelProtocolCapability, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = append([]service.AccountModelProtocolCapability(nil), r.items...)
	}
	return result, nil
}

func (r *modelProtocolHandlerCapabilityRepoStub) SyncObserved(_ context.Context, _ int64, observations []service.ModelProtocolObservation) error {
	r.observations = append([]service.ModelProtocolObservation(nil), observations...)
	return nil
}

func (r *modelProtocolHandlerCapabilityRepoStub) UpdateOverrides(_ context.Context, _ int64, _ []service.ModelProtocolOverride) error {
	return nil
}
