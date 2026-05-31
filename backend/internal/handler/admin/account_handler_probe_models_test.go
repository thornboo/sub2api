package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupProbeModelsRouter(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/accounts/probe-models", handler.ProbeModels)
	return router
}

func TestAccountHandlerProbeModelsRejectsPrivateBaseURL(t *testing.T) {
	router := setupProbeModelsRouter(newStubAdminService())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/accounts/probe-models",
		strings.NewReader(`{"base_url":"http://127.0.0.1:6379","api_key":"test-key"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Invalid base URL")
}

func TestAccountHandlerProbeModelsRejectsPlainHTTPBaseURL(t *testing.T) {
	router := setupProbeModelsRouter(newStubAdminService())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/accounts/probe-models",
		strings.NewReader(`{"base_url":"http://api.example.com","api_key":"test-key"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "Invalid base URL")
}

func TestAccountHandlerProbeModelsUsesStoredAccountKeyWhenRequestKeyBlank(t *testing.T) {
	adminSvc := &probeModelsStoredKeyService{account: &service.Account{
		ID:     7,
		Name:   "packycode",
		Status: service.StatusActive,
		APIKeys: []service.AccountAPIKey{
			{ID: 11, AccountID: 7, APIKey: "stored-secret"},
		},
	}}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{
		BaseURL:         "https://api.example.com",
		AccountID:       ptrInt64ForProbeModelsTest(7),
		AccountAPIKeyID: ptrInt64ForProbeModelsTest(11),
	})

	require.NoError(t, err)
	require.Equal(t, "stored-secret", apiKey)
}

func TestAccountHandlerProbeModelsRejectsBlankNewKey(t *testing.T) {
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{
		BaseURL: "https://api.example.com",
	})

	require.Error(t, err)
	require.Empty(t, apiKey)
}

func TestNewProbeModelsHTTPRequestSetsRelayCompatibleAuthHeaders(t *testing.T) {
	req, err := newProbeModelsHTTPRequest(context.Background(), "https://api.example.com/v1/models", "stored-secret")

	require.NoError(t, err)
	require.Equal(t, http.MethodGet, req.Method)
	require.Equal(t, "application/json", req.Header.Get("Accept"))
	require.Equal(t, "Bearer stored-secret", req.Header.Get("Authorization"))
	require.Equal(t, "stored-secret", req.Header.Get("X-Api-Key"))
}

func TestProbeModelsUpstreamStatusMessageIncludesStatus(t *testing.T) {
	message := probeModelsUpstreamStatusMessage(http.StatusUnauthorized)

	require.Equal(t, "Failed to fetch supported models: upstream /v1/models returned HTTP 401", message)
}

func TestBuildProbeModelsEndpoint(t *testing.T) {
	require.Equal(t, "https://api.example.com/v1/models", buildProbeModelsEndpoint("https://api.example.com"))
	require.Equal(t, "https://api.example.com/v1/models", buildProbeModelsEndpoint("https://api.example.com/v1"))
	require.Equal(t, "https://api.example.com/api/v1/models", buildProbeModelsEndpoint("https://api.example.com/api/v1/"))
}

type probeModelsStoredKeyService struct {
	stubAdminService
	account *service.Account
}

func (s *probeModelsStoredKeyService) GetAccount(ctx context.Context, id int64) (*service.Account, error) {
	if s.account != nil && s.account.ID == id {
		return s.account, nil
	}
	return s.stubAdminService.GetAccount(ctx, id)
}

func ptrInt64ForProbeModelsTest(value int64) *int64 {
	return &value
}
