package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestModelSelfCheckChainRouteIsBehindAdminAuthentication(t *testing.T) {
	router := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{ModelSelfCheck: adminhandler.NewModelSelfCheckHandler(nil)}}
	auth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, 401, "UNAUTHORIZED", "Authorization required")
			return
		}
		servermiddleware.AbortWithError(c, 403, "FORBIDDEN", "Admin access required")
	})
	RegisterAdminRoutes(router.Group("/api/v1"), h, auth, servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil)
	for _, token := range []string{"", "Bearer user-token"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/v1/admin/model-self-check/chain?group_id=2&model=pro", nil)
		r.Header.Set("Authorization", token)
		router.ServeHTTP(w, r)
		if token == "" {
			require.Equal(t, 401, w.Code)
		} else {
			require.Equal(t, 403, w.Code)
		}
		require.NotContains(t, w.Body.String(), "account_id")
	}
}
