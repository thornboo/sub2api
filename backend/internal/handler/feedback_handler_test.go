package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type feedbackHandlerRepoStub struct {
	service.FeedbackRepository
	created []service.FeedbackCreateInput
}

func (r *feedbackHandlerRepoStub) Create(_ context.Context, input service.FeedbackCreateInput) (*service.Feedback, error) {
	r.created = append(r.created, input)
	return &service.Feedback{
		ID:          int64(len(r.created)),
		Title:       input.Title,
		Content:     input.Content,
		Source:      input.Source,
		Status:      service.FeedbackStatusOpen,
		ReplyStatus: service.FeedbackReplyStatusPending,
		UserID:      input.UserID,
		APIKeyID:    input.APIKeyID,
		MemberID:    input.MemberID,
		CreatedAt:   time.Unix(1776790020, 0).UTC(),
	}, nil
}

func (r *feedbackHandlerRepoStub) List(context.Context, pagination.PaginationParams, service.FeedbackListFilters) ([]service.Feedback, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *feedbackHandlerRepoStub) UpdateStatus(context.Context, int64, string) (*service.Feedback, error) {
	return nil, nil
}

type feedbackHandlerCooldownStub struct {
	claimed map[string]bool
}

func (s *feedbackHandlerCooldownStub) ClaimFeedbackCooldown(_ context.Context, identity string, _ time.Duration) (bool, time.Duration, error) {
	if s.claimed == nil {
		s.claimed = make(map[string]bool)
	}
	if s.claimed[identity] {
		return false, 60 * time.Second, nil
	}
	s.claimed[identity] = true
	return true, 0, nil
}

func TestFeedbackHandlerSubmitUserStrictJSONAndNoStore(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "ok", contentType: "application/json; charset=utf-8", body: `{"content":" hello "}`, wantStatus: http.StatusCreated},
		{name: "wrong content type", contentType: "text/plain", body: `{"content":"hello"}`, wantStatus: http.StatusBadRequest},
		{name: "ok with title", contentType: "application/json", body: `{"title":" 工单 标题 ","content":"hello"}`, wantStatus: http.StatusCreated},
		{name: "legacy content only", contentType: "application/json", body: `{"content":"hello"}`, wantStatus: http.StatusCreated},
		{name: "unknown field", contentType: "application/json", body: `{"content":"hello","image":"x"}`, wantStatus: http.StatusBadRequest},
		{name: "trailing json", contentType: "application/json", body: `{"content":"hello"} {}`, wantStatus: http.StatusBadRequest},
		{name: "non string content", contentType: "application/json", body: `{"content":1}`, wantStatus: http.StatusBadRequest},
		{name: "empty content", contentType: "application/json", body: `{"content":"   "}`, wantStatus: http.StatusBadRequest},
		{name: "nul content", contentType: "application/json", body: "{\"content\":\"bad\\u0000content\"}", wantStatus: http.StatusBadRequest},
		{name: "too large body", contentType: "application/json", body: `{"content":"` + strings.Repeat("x", int(feedbackRequestBodyMaxBytes)) + `"}`, wantStatus: http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			repo := &feedbackHandlerRepoStub{}
			h := NewFeedbackHandler(service.NewFeedbackService(repo, &feedbackHandlerCooldownStub{}), nil)
			router := gin.New()
			router.POST("/api/v1/feedback", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
				h.SubmitUser(c)
			})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			router.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d body=%s, want %d", rec.Code, rec.Body.String(), tt.wantStatus)
			}
			if rec.Header().Get("Cache-Control") != "no-store" || rec.Header().Get("Pragma") != "no-cache" {
				t.Fatalf("missing no-store headers: %+v", rec.Header())
			}
			if tt.wantStatus != http.StatusCreated && len(repo.created) != 0 {
				t.Fatalf("invalid request wrote feedback: %+v", repo.created)
			}
		})
	}
}

func TestFeedbackHandlerSubmitUserRateLimitEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewFeedbackHandler(service.NewFeedbackService(&feedbackHandlerRepoStub{}, &feedbackHandlerCooldownStub{}), nil)
	router := gin.New()
	router.POST("/api/v1/feedback", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		h.SubmitUser(c)
	})
	for i, want := range []int{http.StatusCreated, http.StatusTooManyRequests} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader(`{"content":"hello"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("request %d status=%d body=%s, want %d", i+1, rec.Code, rec.Body.String(), want)
		}
		if want == http.StatusTooManyRequests {
			var envelope struct {
				Reason   string            `json:"reason"`
				Metadata map[string]string `json:"metadata"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if rec.Header().Get("Retry-After") != "60" || envelope.Reason != "FEEDBACK_RATE_LIMITED" || envelope.Metadata["retry_after"] != "60" {
				t.Fatalf("bad rate limit response headers=%+v body=%s", rec.Header(), rec.Body.String())
			}
		}
	}
}

func TestFeedbackHandlerSubmitKeyUsesSessionAuthorityAndRejectsCrossSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	memberID := int64(9)
	apiKey := &service.APIKey{ID: 101, UserID: 7, MemberID: &memberID, Status: service.StatusAPIKeyExpired}
	sessionCache := &publicKeyAnnouncementSessionCacheStub{
		session: &service.PublicKeyUsageSession{
			APIKeyID:          apiKey.ID,
			UserID:            apiKey.UserID,
			MemberID:          &memberID,
			AbsoluteExpiresAt: time.Now().Add(time.Hour),
		},
	}
	token := "valid-token"
	hash := sha256.Sum256([]byte(token))
	sessionCache.expectedTokenHash = hex.EncodeToString(hash[:])
	apiKeyService := service.NewAPIKeyService(
		&publicKeyAnnouncementAPIKeyRepoStub{key: apiKey},
		nil,
		nil,
		nil,
		nil,
		sessionCache,
		&config.Config{},
	)
	repo := &feedbackHandlerRepoStub{}
	h := NewFeedbackHandler(service.NewFeedbackService(repo, &feedbackHandlerCooldownStub{}), apiKeyService)
	router := gin.New()
	router.POST("/api/v1/key/feedback", h.SubmitKey)

	crossSite := httptest.NewRecorder()
	crossReq := httptest.NewRequest(http.MethodPost, "/api/v1/key/feedback", strings.NewReader(`{"content":"hello"}`))
	crossReq.Header.Set("Content-Type", "application/json")
	crossReq.Header.Set("Sec-Fetch-Site", "cross-site")
	crossReq.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: token})
	router.ServeHTTP(crossSite, crossReq)
	if crossSite.Code != http.StatusForbidden {
		t.Fatalf("cross-site status=%d body=%s, want 403", crossSite.Code, crossSite.Body.String())
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/key/feedback?api_key_id=999&user_id=999", strings.NewReader(`{"content":" key text "}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: token})
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", rec.Code, rec.Body.String())
	}
	if len(repo.created) != 1 || repo.created[0].Source != service.FeedbackSourceKey || repo.created[0].UserID != 7 || repo.created[0].APIKeyID == nil || *repo.created[0].APIKeyID != 101 {
		t.Fatalf("feedback did not use session authority: %+v", repo.created)
	}

	apiKey.MemberID = nil
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/key/feedback", strings.NewReader(`{"content":"after change"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: token})
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !sessionCache.deleted {
		t.Fatalf("changed authority status=%d deleted=%v body=%s, want 401 and cleared session", rec.Code, sessionCache.deleted, rec.Body.String())
	}
}

func TestBindFeedbackRequestOversizeTrailingReturns413(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewFeedbackHandler(service.NewFeedbackService(&feedbackHandlerRepoStub{}, &feedbackHandlerCooldownStub{}), nil)
	router := gin.New()
	router.POST("/api/v1/feedback", func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
		h.SubmitUser(c)
	})
	rec := httptest.NewRecorder()
	body := `{"content":"ok"}` + strings.Repeat(" ", int(feedbackRequestBodyMaxBytes))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s, want 413", rec.Code, rec.Body.String())
	}
}
