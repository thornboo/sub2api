package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 捕获 ListUsers 入参、返回一个已删用户的 admin service 桩。
type searchUsersAdminStub struct {
	service.AdminService
	gotFilters service.UserListFilters
}

func (s *searchUsersAdminStub) ListUsers(ctx context.Context, page, pageSize int, filters service.UserListFilters, sortBy, sortOrder string) ([]service.User, int64, error) {
	s.gotFilters = filters
	ts := time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC)
	return []service.User{
		{ID: 1, Email: "active@test.com"},
		{ID: 2, Email: "deleted@test.com", DeletedAt: &ts},
	}, 2, nil
}

func TestAdminUsageSearchUsers_IncludesDeletedAndFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stub := &searchUsersAdminStub{}
	handler := NewUsageHandler(nil, nil, stub, nil)
	router := gin.New()
	router.GET("/admin/usage/search-users", handler.SearchUsers)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-users?q=test", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, stub.gotFilters.IncludeDeleted, "SearchUsers 必须请求 IncludeDeleted")

	var resp struct {
		Data []struct {
			ID      int64  `json:"id"`
			Email   string `json:"email"`
			Deleted bool   `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	require.False(t, resp.Data[0].Deleted)
	require.True(t, resp.Data[1].Deleted, "已删用户必须标记 deleted=true")
}

type searchAPIKeysRepoStub struct {
	service.APIKeyRepository
	gotIncludeDeleted bool
	gotAPIKeyID       int64
}

func (s *searchAPIKeysRepoStub) SearchAPIKeys(ctx context.Context, userID int64, keyword string, limit int) ([]service.APIKey, error) {
	return s.SearchAPIKeysIncludingDeleted(ctx, userID, keyword, limit, false)
}

func (s *searchAPIKeysRepoStub) SearchAPIKeysIncludingDeleted(ctx context.Context, userID int64, keyword string, limit int, includeDeleted bool) ([]service.APIKey, error) {
	s.gotIncludeDeleted = includeDeleted
	deletedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	return []service.APIKey{
		{ID: 7, UserID: userID, Name: "active-key"},
		{ID: 8, UserID: userID, Name: "deleted-key", DeletedAt: &deletedAt},
	}, nil
}

func (s *searchAPIKeysRepoStub) GetByIDIncludingDeleted(ctx context.Context, id int64) (*service.APIKey, error) {
	s.gotAPIKeyID = id
	deletedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	return &service.APIKey{ID: id, UserID: 4, Name: "deleted-key", DeletedAt: &deletedAt}, nil
}

func TestAdminUsageSearchAPIKeys_CanIncludeDeletedAndFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &searchAPIKeysRepoStub{}
	apiKeySvc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewUsageHandler(nil, apiKeySvc, nil, nil)
	router := gin.New()
	router.GET("/admin/usage/search-api-keys", handler.SearchAPIKeys)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-api-keys?user_id=4&q=key&include_deleted=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, repo.gotIncludeDeleted, "SearchAPIKeys must forward include_deleted")

	var resp struct {
		Data []struct {
			ID        int64      `json:"id"`
			Name      string     `json:"name"`
			UserID    int64      `json:"user_id"`
			Deleted   bool       `json:"deleted"`
			DeletedAt *time.Time `json:"deleted_at"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	require.Equal(t, int64(4), resp.Data[1].UserID)
	require.True(t, resp.Data[1].Deleted)
	require.NotNil(t, resp.Data[1].DeletedAt)
}

func TestAdminUsageSearchAPIKeys_ExactLookupCanIncludeDeleted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &searchAPIKeysRepoStub{}
	apiKeySvc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewUsageHandler(nil, apiKeySvc, nil, nil)
	router := gin.New()
	router.GET("/admin/usage/search-api-keys", handler.SearchAPIKeys)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-api-keys?user_id=4&api_key_id=8&include_deleted=true", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int64(8), repo.gotAPIKeyID, "api_key_id must use exact including-deleted lookup")

	var resp struct {
		Data []struct {
			ID      int64  `json:"id"`
			Name    string `json:"name"`
			UserID  int64  `json:"user_id"`
			Deleted bool   `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, int64(8), resp.Data[0].ID)
	require.Equal(t, int64(4), resp.Data[0].UserID)
	require.Equal(t, "deleted-key", resp.Data[0].Name)
	require.True(t, resp.Data[0].Deleted)
}

func TestAdminUsageSearchAPIKeys_ExactLookupHidesDeletedByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &searchAPIKeysRepoStub{}
	apiKeySvc := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewUsageHandler(nil, apiKeySvc, nil, nil)
	router := gin.New()
	router.GET("/admin/usage/search-api-keys", handler.SearchAPIKeys)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/search-api-keys?user_id=4&api_key_id=8", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Empty(t, resp.Data)
}
