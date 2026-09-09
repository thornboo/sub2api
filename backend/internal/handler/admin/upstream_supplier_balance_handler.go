package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"

	"github.com/gin-gonic/gin"
)

// RefreshUpstreamSupplierBalance refreshes an upstream login account's wallet.
func (h *AccountHandler) RefreshUpstreamSupplierBalance(c *gin.Context) {
	id, ok := parseSupplierIDParam(c)
	if !ok {
		return
	}
	svc, ok := h.upstreamCostPoolService(c)
	if !ok {
		return
	}
	supplier, err := svc.RefreshUpstreamSupplierBalance(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, supplier)
}
