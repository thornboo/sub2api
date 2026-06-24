package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newUpstreamCostProfileTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPatch, "/admin/accounts/1/upstream-cost-profile", nil)
	return c, w
}

func TestBuildUpstreamCostProfileExtraUpdatesIncludesBalanceDefaults(t *testing.T) {
	c, _ := newUpstreamCostProfileTestContext()
	enabled := true

	updates, ok := buildUpstreamCostProfileExtraUpdates(c, upstreamCostProfileRequest{
		BalanceEnabled: &enabled,
	})

	require.True(t, ok)
	require.Equal(t, true, updates["upstream_balance_query_enabled"])
	require.Equal(t, service.UpstreamBalanceProviderSub2API, updates["upstream_balance_provider"])
	require.Equal(t, service.UpstreamBalanceDefaultEndpoint, updates["upstream_balance_endpoint"])
	require.Equal(t, service.UpstreamBalanceAuthModeAccountAPIKey, updates["upstream_balance_auth_mode"])
	require.NotContains(t, updates, "upstream_balance_auth_header")
}

func TestBuildUpstreamCostProfileExtraUpdatesMigratesLegacySub2APIProfileEndpoint(t *testing.T) {
	c, _ := newUpstreamCostProfileTestContext()
	enabled := true
	provider := service.UpstreamBalanceProviderSub2API
	endpoint := service.UpstreamBalanceSub2APIProfileEndpoint
	authMode := service.UpstreamBalanceAuthModeAccountAPIKey

	updates, ok := buildUpstreamCostProfileExtraUpdates(c, upstreamCostProfileRequest{
		BalanceEnabled:  &enabled,
		BalanceProvider: &provider,
		BalanceEndpoint: &endpoint,
		BalanceAuthMode: &authMode,
	})

	require.True(t, ok)
	require.Equal(t, service.UpstreamBalanceDefaultEndpoint, updates["upstream_balance_endpoint"])
	require.Equal(t, service.UpstreamBalanceAuthModeAccountAPIKey, updates["upstream_balance_auth_mode"])
}

func TestBuildUpstreamCostProfileExtraUpdatesKeepsCustomBalanceHeader(t *testing.T) {
	c, _ := newUpstreamCostProfileTestContext()
	enabled := true
	provider := service.UpstreamBalanceProviderNewAPICompatible
	authMode := service.UpstreamBalanceAuthModeCustomHeader
	authHeader := "X-Panel-Token"

	updates, ok := buildUpstreamCostProfileExtraUpdates(c, upstreamCostProfileRequest{
		BalanceEnabled:    &enabled,
		BalanceProvider:   &provider,
		BalanceAuthMode:   &authMode,
		BalanceAuthHeader: &authHeader,
	})

	require.True(t, ok)
	require.Equal(t, service.UpstreamBalanceProviderNewAPICompatible, updates["upstream_balance_provider"])
	require.Equal(t, service.UpstreamBalanceNewAPIDefaultEndpoint, updates["upstream_balance_endpoint"])
	require.Equal(t, service.UpstreamBalanceAuthModeCustomHeader, updates["upstream_balance_auth_mode"])
	require.Equal(t, "X-Panel-Token", updates["upstream_balance_auth_header"])
}

func TestBuildUpstreamCostProfileExtraUpdatesCanDisableBalanceQuery(t *testing.T) {
	c, _ := newUpstreamCostProfileTestContext()
	enabled := false

	updates, ok := buildUpstreamCostProfileExtraUpdates(c, upstreamCostProfileRequest{
		BalanceEnabled: &enabled,
	})

	require.True(t, ok)
	require.Equal(t, false, updates["upstream_balance_query_enabled"])
	require.NotContains(t, updates, "upstream_balance_endpoint")
	require.NotContains(t, updates, "upstream_balance_auth_mode")
}
