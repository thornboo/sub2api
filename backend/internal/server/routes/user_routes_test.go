package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newUserRoutesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			User:             &handler.UserHandler{},
			APIKey:           handler.NewAPIKeyHandler(&service.APIKeyService{}),
			Usage:            &handler.UsageHandler{},
			Redeem:           &handler.RedeemHandler{},
			Subscription:     &handler.SubscriptionHandler{},
			Announcement:     &handler.AnnouncementHandler{},
			ChannelMonitor:   &handler.ChannelMonitorUserHandler{},
			Totp:             &handler.TotpHandler{},
			AvailableChannel: &handler.AvailableChannelHandler{},
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) {
			c.Next()
		}),
		nil,
	)

	return router
}

func TestUserRoutesAPIKeyBatchPathsAreRegisteredBeforeIDRoute(t *testing.T) {
	router := newUserRoutesTestRouter()

	for _, path := range []string{"/api/v1/keys/batch", "/api/v1/keys/batch-update", "/api/v1/keys/batch-delete"} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		require.NotEqual(t, http.StatusNotFound, w.Code, path)
	}
}

func TestUserRoutesAPIKeyTagOptionsPathIsRegisteredBeforeIDRoute(t *testing.T) {
	router := newUserRoutesTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/keys/tags", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.NotEqual(t, http.StatusNotFound, w.Code)
}

func TestUserRoutesDoNotExposeLegacyChannelMonitorProbeEndpoints(t *testing.T) {
	router := newUserRoutesTestRouter()

	for _, path := range []string{"/api/v1/channel-monitors", "/api/v1/channel-monitors/1/status"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusNotFound, w.Code, path)
	}
}
