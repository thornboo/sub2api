package admin

import (
	"context"
	"net/http"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type upstreamBalanceFetcher interface {
	FetchUpstreamBalance(ctx context.Context, account *service.Account) (*service.UpstreamBalanceSnapshot, error)
}

// RefreshUpstreamBalance queries the configured upstream key quota endpoint and
// persists the latest snapshot to account.extra. The route name is kept for
// compatibility with the first version of the feature.
// POST /api/v1/admin/accounts/:id/upstream-balance/refresh
func (h *AccountHandler) RefreshUpstreamBalance(c *gin.Context) {
	h.refreshUpstreamBalanceSnapshot(
		c,
		func(ctx context.Context, fetcher upstreamBalanceFetcher, account *service.Account) (*service.UpstreamBalanceSnapshot, error) {
			return fetcher.FetchUpstreamBalance(ctx, account)
		},
		service.UpstreamBalanceSnapshotExtraKey,
		"empty upstream key quota result",
	)
}

func (h *AccountHandler) refreshUpstreamBalanceSnapshot(
	c *gin.Context,
	fetch func(context.Context, upstreamBalanceFetcher, *service.Account) (*service.UpstreamBalanceSnapshot, error),
	snapshotKey string,
	emptyError string,
) {
	accountID, ok := parseAccountIDParam(c)
	if !ok {
		return
	}
	fetcher, ok := h.upstreamBalanceFetcher(c)
	if !ok {
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	snapshot, err := fetch(c.Request.Context(), fetcher, account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if snapshot == nil {
		snapshot = &service.UpstreamBalanceSnapshot{
			Provider:  service.UpstreamBalanceDefaultProvider,
			Status:    "error",
			FetchedAt: time.Now().UTC(),
			Error:     emptyError,
		}
	}
	if err := h.adminService.UpdateAccountExtra(c.Request.Context(), accountID, map[string]any{
		snapshotKey: snapshot,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	updated, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), updated))
}

func (h *AccountHandler) upstreamBalanceFetcher(c *gin.Context) (upstreamBalanceFetcher, bool) {
	if h.accountTestService == nil {
		response.ErrorFrom(c, infraerrors.New(http.StatusNotImplemented, "UPSTREAM_BALANCE_UNAVAILABLE", "upstream balance service is unavailable"))
		return nil, false
	}
	return h.accountTestService, true
}
