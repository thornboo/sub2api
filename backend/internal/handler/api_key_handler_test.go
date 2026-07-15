package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type apiKeyUpdateRepoStub struct {
	service.APIKeyRepository
	key service.APIKey
}

func (s *apiKeyUpdateRepoStub) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
	if s.key.ID != id {
		return nil, service.ErrAPIKeyNotFound
	}
	key := s.key
	return &key, nil
}

func (s *apiKeyUpdateRepoStub) Update(_ context.Context, key *service.APIKey) error {
	s.key = *key
	return nil
}

func TestAPIKeyHandlerUpdateAcceptsDisabledStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &apiKeyUpdateRepoStub{
		key: service.APIKey{
			ID:     1,
			UserID: 42,
			Key:    "sk-test",
			Name:   "test",
			Status: service.StatusAPIKeyActive,
		},
	}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewAPIKeyHandler(apiKeyService)
	router := gin.New()
	router.PUT("/keys/:id", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		handler.Update(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/keys/1", strings.NewReader(`{"status":"disabled"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, body = %s", w.Code, w.Body.String())
	}
	if got := repo.key.Status; got != service.StatusAPIKeyDisabled {
		t.Fatalf("api key status = %q, want %q", got, service.StatusAPIKeyDisabled)
	}
}

func TestAPIKeyHandlerGetByIDDoesNotExposeEnterpriseMemberKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(9)
	repo := &apiKeyUpdateRepoStub{
		key: service.APIKey{
			ID:       1,
			UserID:   42,
			MemberID: &memberID,
			Key:      "sk-member-secret",
			Name:     "member key",
			Status:   service.StatusAPIKeyActive,
		},
	}
	apiKeyService := service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	handler := NewAPIKeyHandler(apiKeyService)
	router := gin.New()
	router.GET("/keys/:id", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		handler.GetByID(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, body = %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if strings.Contains(w.Body.String(), "sk-member-secret") {
		t.Fatalf("member key leaked in response: %s", w.Body.String())
	}
}
