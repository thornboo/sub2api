package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type probeModelsAdminService struct {
	*stubAdminService
	account *service.Account
	err     error
	calls   int
}

func (s *probeModelsAdminService) GetAccount(_ context.Context, id int64) (*service.Account, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.account != nil && s.account.ID == id {
		account := *s.account
		return &account, nil
	}
	return nil, errors.New("account not found")
}

func setupProbeModelsRouter(adminSvc service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
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

func TestAccountHandlerProbeModelsRejectsMissingAPIKeyWithoutAccount(t *testing.T) {
	router := setupProbeModelsRouter(newStubAdminService())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/accounts/probe-models",
		strings.NewReader(`{"base_url":"https://api.example.com"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "API key is required")
}

func TestResolveProbeModelsAPIKeyPrefersProvidedKey(t *testing.T) {
	adminSvc := &probeModelsAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:          10,
			Credentials: map[string]any{"api_key": "stored-key"},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{
		APIKey:    "  typed-key  ",
		AccountID: 10,
	})

	require.NoError(t, err)
	require.Equal(t, "typed-key", apiKey)
	require.Zero(t, adminSvc.calls)
}

func TestResolveProbeModelsAPIKeyUsesStoredAPIKeyFromAccount(t *testing.T) {
	adminSvc := &probeModelsAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:          10,
			Credentials: map[string]any{"api_key": " stored-key "},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{AccountID: 10})

	require.NoError(t, err)
	require.Equal(t, "stored-key", apiKey)
	require.Equal(t, 1, adminSvc.calls)
}

func TestResolveProbeModelsAPIKeyFallsBackToStoredAccessToken(t *testing.T) {
	adminSvc := &probeModelsAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:          10,
			Credentials: map[string]any{"access_token": " stored-access-token "},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{AccountID: 10})

	require.NoError(t, err)
	require.Equal(t, "stored-access-token", apiKey)
	require.Equal(t, 1, adminSvc.calls)
}

func TestResolveProbeModelsAPIKeyRejectsInvalidAccountID(t *testing.T) {
	handler := NewAccountHandler(newStubAdminService(), nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{AccountID: -1})

	require.Empty(t, apiKey)
	require.ErrorIs(t, err, errProbeModelsInvalidAccount)
}

func TestResolveProbeModelsAPIKeyRejectsMissingAccount(t *testing.T) {
	adminSvc := &probeModelsAdminService{stubAdminService: newStubAdminService()}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{AccountID: 10})

	require.Empty(t, apiKey)
	require.ErrorIs(t, err, errProbeModelsAccountMissing)
	require.Equal(t, 1, adminSvc.calls)
}

func TestResolveProbeModelsAPIKeyRejectsAccountWithoutCredential(t *testing.T) {
	adminSvc := &probeModelsAdminService{
		stubAdminService: newStubAdminService(),
		account: &service.Account{
			ID:          10,
			Credentials: map[string]any{"base_url": "https://api.example.com"},
		},
	}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	apiKey, err := handler.resolveProbeModelsAPIKey(context.Background(), probeModelsRequest{AccountID: 10})

	require.Empty(t, apiKey)
	require.ErrorIs(t, err, errProbeModelsAPIKeyRequired)
	require.Equal(t, 1, adminSvc.calls)
}

func TestBuildProbeModelsEndpoint(t *testing.T) {
	require.Equal(t, "https://api.example.com/v1/models", buildProbeModelsEndpoint("https://api.example.com"))
	require.Equal(t, "https://api.example.com/v1/models", buildProbeModelsEndpoint("https://api.example.com/v1"))
	require.Equal(t, "https://api.example.com/api/v1/models", buildProbeModelsEndpoint("https://api.example.com/api/v1/"))
}
