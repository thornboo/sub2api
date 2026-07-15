package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type opsEnterpriseMemberRedactionRepo struct {
	service.OpsRepository
	item *service.OpsErrorLog
}

type opsEnterpriseMemberBudgetRedactionRepo struct {
	service.EnterpriseMemberBudgetRepository
	item service.EnterpriseMemberAmbiguousReceipt
}

func (r *opsEnterpriseMemberBudgetRedactionRepo) ListAmbiguousReceipts(context.Context, int, int) ([]service.EnterpriseMemberAmbiguousReceipt, int64, error) {
	return []service.EnterpriseMemberAmbiguousReceipt{r.item}, 1, nil
}

func (r *opsEnterpriseMemberRedactionRepo) ListErrorLogs(context.Context, *service.OpsErrorLogFilter) (*service.OpsErrorLogList, error) {
	return &service.OpsErrorLogList{Errors: []*service.OpsErrorLog{r.item}, Total: 1, Page: 1, PageSize: 20}, nil
}

func (r *opsEnterpriseMemberRedactionRepo) GetErrorLogByID(context.Context, int64) (*service.OpsErrorLogDetail, error) {
	return &service.OpsErrorLogDetail{OpsErrorLog: *r.item, ErrorBody: "safe error body"}, nil
}

func TestAdminOpsResponsesRedactEnterpriseMemberIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(91)
	repo := &opsEnterpriseMemberRedactionRepo{item: &service.OpsErrorLog{
		ID:                 7,
		Message:            "upstream failed",
		MemberID:           &memberID,
		MemberCodeSnapshot: "secret-member-code",
		MemberNameSnapshot: "Secret Member Name",
	}}
	handler := NewOpsHandler(service.NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil))
	router := gin.New()
	router.GET("/errors", handler.GetErrorLogs)
	router.GET("/errors/:id", handler.GetErrorLogByID)

	for _, path := range []string{"/errors", "/errors/7"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "upstream failed")
		require.NotContains(t, w.Body.String(), "secret-member-code")
		require.NotContains(t, w.Body.String(), "Secret Member Name")
		require.False(t, strings.Contains(w.Body.String(), `"member_id":91`), w.Body.String())
	}

	// Redaction must be response-local; the service object remains intact for
	// owner-scoped APIs and internal correlation.
	require.NotNil(t, repo.item.MemberID)
	require.Equal(t, "secret-member-code", repo.item.MemberCodeSnapshot)
}

func TestAdminOpsAmbiguousReceiptListExposesOnlyOperationalCorrelationFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
	groupID := int64(23)
	repo := &opsEnterpriseMemberBudgetRedactionRepo{item: service.EnterpriseMemberAmbiguousReceipt{
		ID:                51,
		RequestID:         "17:client:secret-request-id",
		EnterpriseUserID:  3,
		MemberID:          91,
		MemberCode:        "secret-member-code",
		MemberName:        "Secret Member Name",
		GroupID:           &groupID,
		PeriodStart:       now,
		ReservedUSD:       9.5,
		OutcomeReason:     "task_persistence_failed",
		ReconcileAttempts: 2,
		ExpiresAt:         now.Add(10 * time.Minute),
		CreatedAt:         now,
		UpdatedAt:         now,
	}}
	handler := NewOpsHandler(nil, service.NewEnterpriseMemberBudgetService(repo, nil, nil))
	router := gin.New()
	router.GET("/ambiguous", handler.ListEnterpriseMemberAmbiguousReceipts)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ambiguous", nil)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"id":51`)
	require.Contains(t, w.Body.String(), `"member_id":91`)
	require.Contains(t, w.Body.String(), "task_persistence_failed")
	for _, secret := range []string{"secret-request-id", "secret-member-code", "Secret Member Name", `"group_id"`, `"reserved_usd"`, `"enterprise_user_id"`, `"period_start"`} {
		require.NotContains(t, w.Body.String(), secret)
	}
}
