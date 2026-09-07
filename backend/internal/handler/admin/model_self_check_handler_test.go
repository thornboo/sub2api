package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type selfCheckDiagnosticsStub struct {
	calls   int
	groupID int64
	model   string
	view    *service.ModelSelfCheckChainView
	err     error
}

func (s *selfCheckDiagnosticsStub) ListTokenUsageSince(context.Context, time.Time) ([]service.ModelSelfCheckTokenUsage, error) {
	return nil, nil
}
func (s *selfCheckDiagnosticsStub) GetAdminProbeChain(_ context.Context, groupID int64, model string) (*service.ModelSelfCheckChainView, error) {
	s.calls++
	s.groupID, s.model = groupID, model
	return s.view, s.err
}

func TestModelSelfCheckChainHandlerValidatesTarget(t *testing.T) {
	for _, query := range []string{"", "?group_id=0&model=pro", "?group_id=-2&model=pro", "?group_id=invalid&model=pro", "?group_id=2", "?group_id=2&model=%20"} {
		t.Run(query, func(t *testing.T) {
			stub := &selfCheckDiagnosticsStub{}
			h := &ModelSelfCheckHandler{modelStatusService: stub}
			router := gin.New()
			router.GET("/chain", h.GetProbeChain)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/chain"+query, nil))
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Zero(t, stub.calls)
		})
	}
}

func TestModelSelfCheckChainHandlerReturnsEvidenceOrExplicitError(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stub   *selfCheckDiagnosticsStub
		status int
	}{
		{"success", &selfCheckDiagnosticsStub{view: &service.ModelSelfCheckChainView{GroupID: 2, Model: "pro", Candidates: []service.ModelSelfCheckChainCandidate{{AccountID: 7, AccountName: "C", Priority: 2, Order: 1, Eligible: true}}}}, 200},
		{"not found", &selfCheckDiagnosticsStub{}, 404},
		{"storage failure", &selfCheckDiagnosticsStub{err: errors.New("storage failure")}, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &ModelSelfCheckHandler{modelStatusService: tc.stub}
			router := gin.New()
			router.GET("/chain", h.GetProbeChain)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/chain?group_id=2&model=%20pro%20", nil))
			require.Equal(t, tc.status, w.Code)
			require.EqualValues(t, 2, tc.stub.groupID)
			require.Equal(t, "pro", tc.stub.model)
			if tc.status == 200 {
				require.Contains(t, w.Body.String(), `"account_name":"C"`)
				require.NotContains(t, w.Body.String(), "credentials")
			}
		})
	}
}
