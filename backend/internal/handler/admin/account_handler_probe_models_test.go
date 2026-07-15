package admin

import (
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

func TestBuildProbeModelsEndpoint(t *testing.T) {
	require.Equal(t, "https://api.example.com/v1/models", buildProbeModelsEndpoint("https://api.example.com"))
	require.Equal(t, "https://api.example.com/v1/models", buildProbeModelsEndpoint("https://api.example.com/v1"))
	require.Equal(t, "https://api.example.com/api/v1/models", buildProbeModelsEndpoint("https://api.example.com/api/v1/"))
}
