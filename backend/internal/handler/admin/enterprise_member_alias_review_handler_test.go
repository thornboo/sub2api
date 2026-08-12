package admin

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAliasReviewHandlerListAndReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &aliasReviewHandlerRepoFake{
		items: []service.EnterpriseMemberAliasReviewItem{{
			PublicModel: "Alias", PublicModelNorm: "alias", LegacyOutcome: "success", PlannedOutcome: "pruned",
			RequestCount7d: 1, RequestCount30d: 2,
		}},
		summary: &service.EnterpriseMemberAliasReviewReadinessSummary{BlockingUnreviewedActive7d: 1},
	}
	handler := NewOpsHandler(nil)
	handler.SetEnterpriseMemberAliasReviewService(service.NewEnterpriseMemberAliasReviewService(repo, nil, nil, nil))
	router := gin.New()
	router.GET("/aliases", handler.ListEnterpriseMemberModelAliases)
	router.GET("/readiness", handler.GetEnterpriseMemberModelAliasReadiness)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/aliases?limit=25", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"public_model":"Alias"`)
	require.Equal(t, 25, repo.lastLimit)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/readiness", nil)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"ready":false`)
	require.Contains(t, w.Body.String(), "legacy_success_new_pruned_requires_review")
}

func TestEnterpriseMemberAliasReviewHandlerReviewRequiresAdminSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &aliasReviewHandlerRepoFake{}
	handler := NewOpsHandler(nil)
	handler.SetEnterpriseMemberAliasReviewService(service.NewEnterpriseMemberAliasReviewService(repo, nil, nil, nil))
	router := gin.New()
	router.PUT("/review", handler.ReviewEnterpriseMemberModelAlias)

	body := bytes.NewBufferString(`{"public_model":"Alias","status":"rejected_invalid"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/review", body)
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.False(t, repo.wrote)

	router = gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 77})
	})
	router.PUT("/review", handler.ReviewEnterpriseMemberModelAlias)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/review", bytes.NewBufferString(`{"public_model":"Alias","status":"rejected_invalid"}`))
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, repo.wrote)
	require.Equal(t, int64(77), repo.reviewedBy)
	require.Contains(t, w.Body.String(), `"status":"rejected_invalid"`)
}

type aliasReviewHandlerRepoFake struct {
	items      []service.EnterpriseMemberAliasReviewItem
	summary    *service.EnterpriseMemberAliasReviewReadinessSummary
	lastLimit  int
	wrote      bool
	reviewedBy int64
}

func (f *aliasReviewHandlerRepoFake) ListLegacySuccessNewPruned(_ context.Context, input service.EnterpriseMemberAliasReviewListInput) ([]service.EnterpriseMemberAliasReviewItem, error) {
	f.lastLimit = input.Limit
	return f.items, nil
}

func (f *aliasReviewHandlerRepoFake) UpsertReview(_ context.Context, input service.EnterpriseMemberAliasReviewUpsert) (*service.EnterpriseMemberAliasReviewRecord, error) {
	f.wrote = true
	f.reviewedBy = input.ReviewedBy
	now := time.Now().UTC()
	return &service.EnterpriseMemberAliasReviewRecord{
		ID: 1, PublicModel: input.PublicModel, PublicModelNorm: input.PublicModelNorm,
		Endpoint: input.Endpoint, Status: input.Status, ReviewNote: input.ReviewNote,
		ReviewedBy: &input.ReviewedBy, ReviewedAt: &now,
	}, nil
}

func (f *aliasReviewHandlerRepoFake) GetReadinessSummary(context.Context, time.Time) (*service.EnterpriseMemberAliasReviewReadinessSummary, error) {
	if f.summary != nil {
		return f.summary, nil
	}
	return &service.EnterpriseMemberAliasReviewReadinessSummary{}, nil
}
