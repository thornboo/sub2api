package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type supplierBalanceAdminStub struct {
	service.AdminService
	upstreamCostPoolService
	createInput service.CreateUpstreamSupplierInput
	updateInput service.UpdateUpstreamSupplierInput
	refreshID   int64
}

func (s *supplierBalanceAdminStub) CreateUpstreamSupplier(_ context.Context, input service.CreateUpstreamSupplierInput) (*service.UpstreamSupplier, error) {
	s.createInput = input
	return &service.UpstreamSupplier{ID: 8}, nil
}
func (s *supplierBalanceAdminStub) UpdateUpstreamSupplier(_ context.Context, input service.UpdateUpstreamSupplierInput) (*service.UpstreamSupplier, error) {
	s.updateInput = input
	return &service.UpstreamSupplier{ID: input.SupplierID}, nil
}
func (s *supplierBalanceAdminStub) RefreshUpstreamSupplierBalance(_ context.Context, id int64) (*service.UpstreamSupplier, error) {
	s.refreshID = id
	zero := 0.0
	return &service.UpstreamSupplier{ID: id, BalanceSnapshot: &service.UpstreamSupplierBalanceSnapshot{BalanceUSD: &zero, Status: "ok"}}, nil
}

func TestSupplierBalanceHandlerConfigurationAndRefresh(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &supplierBalanceAdminStub{}
	h := &AccountHandler{adminService: svc}
	r := gin.New()
	r.POST("/suppliers", h.CreateUpstreamSupplier)
	r.PATCH("/suppliers/:supplier_id", h.UpdateUpstreamSupplier)
	r.POST("/suppliers/:supplier_id/balance/refresh", h.RefreshUpstreamSupplierBalance)
	request := func(method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(rec, req)
		return rec
	}
	rec := request(http.MethodPost, "/suppliers", `{"name":"supplier","balance_config":{"enabled":true,"provider":"newapi","base_url":"https://upstream.example","user_id":7,"access_token":"private-token"}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, svc.createInput.BalanceConfig)
	require.Equal(t, "private-token", svc.createInput.BalanceConfig.AccessToken)
	require.Equal(t, int64(7), svc.createInput.BalanceConfig.UserID)
	require.NotContains(t, rec.Body.String(), "private-token")

	rec = request(http.MethodPatch, "/suppliers/8", `{"name":"renamed"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Nil(t, svc.updateInput.BalanceConfig)
	rec = request(http.MethodPatch, "/suppliers/8", `{"balance_config":{"enabled":true,"provider":"newapi","base_url":"https://upstream.example","access_token":""}}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Empty(t, svc.updateInput.BalanceConfig.AccessToken)

	rec = request(http.MethodPost, "/suppliers/8/balance/refresh", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, int64(8), svc.refreshID)
	var payload struct {
		Data service.UpstreamSupplier `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.NotNil(t, payload.Data.BalanceSnapshot.BalanceUSD)
	require.Zero(t, *payload.Data.BalanceSnapshot.BalanceUSD)
	svc.refreshID = 0
	for _, id := range []string{"0", "-1", "bad"} {
		rec = request(http.MethodPost, "/suppliers/"+id+"/balance/refresh", "")
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
	require.Zero(t, svc.refreshID)
}

func TestSupplierBalanceHandlerUnavailableUsesStandardReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AccountHandler{}
	r := gin.New()
	r.POST("/suppliers/:supplier_id/balance/refresh", h.RefreshUpstreamSupplierBalance)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/suppliers/8/balance/refresh", nil))
	require.Equal(t, http.StatusNotImplemented, rec.Code)
	var payload struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Equal(t, "UPSTREAM_COST_POOL_UNAVAILABLE", payload.Reason)
}
