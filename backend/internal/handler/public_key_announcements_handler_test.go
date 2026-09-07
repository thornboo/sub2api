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
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type publicKeyAnnouncementAPIKeyRepoStub struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *publicKeyAnnouncementAPIKeyRepoStub) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
	if r.key == nil || r.key.ID != id {
		return nil, service.ErrAPIKeyNotFound
	}
	key := *r.key
	return &key, nil
}

type publicKeyAnnouncementSessionCacheStub struct {
	service.APIKeyCache
	session           *service.PublicKeyUsageSession
	deleted           bool
	expectedTokenHash string
}

func (c *publicKeyAnnouncementSessionCacheStub) CreatePublicKeyUsageSession(_ context.Context, _ string, session *service.PublicKeyUsageSession, _ time.Duration) error {
	c.session = session
	return nil
}

func (c *publicKeyAnnouncementSessionCacheStub) GetPublicKeyUsageSession(_ context.Context, tokenHash string) (*service.PublicKeyUsageSession, error) {
	if c.expectedTokenHash != "" && tokenHash != c.expectedTokenHash {
		return nil, service.ErrPublicKeyUsageSessionInvalid
	}
	if c.session == nil {
		return nil, service.ErrPublicKeyUsageSessionInvalid
	}
	session := *c.session
	return &session, nil
}

func (c *publicKeyAnnouncementSessionCacheStub) RefreshPublicKeyUsageSession(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (c *publicKeyAnnouncementSessionCacheStub) DeletePublicKeyUsageSession(_ context.Context, _ string) error {
	c.deleted = true
	return nil
}

type publicKeyAnnouncementRepoStub struct {
	service.AnnouncementRepository
	item   service.Announcement
	active []service.Announcement
}

func (r *publicKeyAnnouncementRepoStub) GetByID(_ context.Context, id int64) (*service.Announcement, error) {
	if r.item.ID != id {
		return nil, service.ErrAnnouncementNotFound
	}
	item := r.item
	return &item, nil
}

func (r *publicKeyAnnouncementRepoStub) ListActive(context.Context, time.Time) ([]service.Announcement, error) {
	return r.active, nil
}

type publicKeyAnnouncementUserRepoStub struct {
	service.UserRepository
	user *service.User
}

func (r *publicKeyAnnouncementUserRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, service.ErrUserNotFound
	}
	user := *r.user
	return &user, nil
}

type publicKeyAnnouncementSubRepoStub struct {
	service.UserSubscriptionRepository
	subs []service.UserSubscription
}

func (r *publicKeyAnnouncementSubRepoStub) ListActiveByUserID(context.Context, int64) ([]service.UserSubscription, error) {
	return r.subs, nil
}

type publicKeyAnnouncementUserReadRepoStub struct {
	service.AnnouncementReadRepository
}

type publicKeyAnnouncementKeyReadRepoStub struct {
	service.AnnouncementKeyReadRepository
	readMap map[int64]time.Time
	marks   []publicKeyAnnouncementKeyReadMark
}

type publicKeyAnnouncementKeyReadMark struct {
	AnnouncementID int64
	APIKeyID       int64
}

func (r *publicKeyAnnouncementKeyReadRepoStub) MarkRead(_ context.Context, announcementID, apiKeyID int64, _ time.Time) error {
	r.marks = append(r.marks, publicKeyAnnouncementKeyReadMark{AnnouncementID: announcementID, APIKeyID: apiKeyID})
	return nil
}

func (r *publicKeyAnnouncementKeyReadRepoStub) GetReadMapByAPIKey(context.Context, int64, []int64) (map[int64]time.Time, error) {
	return r.readMap, nil
}

func TestPublicKeyAnnouncementsRequireCurrentSessionCookie(t *testing.T) {
	router, _, cache := newPublicKeyAnnouncementTestRouter(t, &publicKeyAnnouncementKeyReadRepoStub{})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/key/announcements", nil)

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s, want 401", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" || recorder.Header().Get("Pragma") != "no-cache" {
		t.Fatalf("missing no-store headers: %+v", recorder.Header())
	}
	if cache.deleted {
		t.Fatal("missing cookie should not delete an unrelated session")
	}
}

func TestPublicKeyUsageDisplaySessionIDIsStableAndCannotAuthorizeRequests(t *testing.T) {
	router, _, cache := newPublicKeyAnnouncementTestRouter(t, &publicKeyAnnouncementKeyReadRepoStub{})
	token := "query-cookie-token"
	hash := sha256.Sum256([]byte(token))
	cache.expectedTokenHash = hex.EncodeToString(hash[:])
	getSessionID := func(cookie string) (bool, string) {
		t.Helper()
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/key/usage-session", nil)
		req.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: cookie})
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", recorder.Code)
		}
		var envelope struct {
			Data publicKeyUsageSessionResponse `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		return envelope.Data.Valid, envelope.Data.SessionID
	}
	valid, id := getSessionID(token)
	if !valid || id == "" || id == token || id == cache.expectedTokenHash {
		t.Fatal("valid session should expose an independent non-credential identifier")
	}
	if _, restoredID := getSessionID(token); restoredID != id {
		t.Fatal("restoring the same cookie session must preserve its display identifier")
	}
	if valid, rejectedID := getSessionID(id); valid || rejectedID != "" {
		t.Fatal("the display identifier must not authorize a query session")
	}
	otherToken := "different-query-cookie-token"
	otherHash := sha256.Sum256([]byte(otherToken))
	cache.expectedTokenHash = hex.EncodeToString(otherHash[:])
	if valid, otherID := getSessionID(otherToken); !valid || otherID == id {
		t.Fatal("different cookie sessions must have separate display identifiers")
	}
}

func TestPublicKeyAnnouncementsRejectExpiredSessionAndClearCookie(t *testing.T) {
	router, _, cache := newPublicKeyAnnouncementTestRouter(t, &publicKeyAnnouncementKeyReadRepoStub{})
	cache.session.AbsoluteExpiresAt = time.Now().Add(-time.Minute)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/key/announcements", nil)
	req.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: "expired-token"})

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body=%s, want 401", recorder.Code, recorder.Body.String())
	}
	if !cache.deleted {
		t.Fatal("expired session should delete cached authority")
	}
	if cookies := recorder.Result().Cookies(); len(cookies) == 0 || cookies[0].MaxAge != -1 || cookies[0].Path != publicKeyUsageSessionPath {
		t.Fatalf("expired session should clear scoped cookie, got %+v", cookies)
	}
}

func TestPublicKeyAnnouncementsReturnSanitizedUserAnnouncementDTO(t *testing.T) {
	router, _, _ := newPublicKeyAnnouncementTestRouter(t, &publicKeyAnnouncementKeyReadRepoStub{})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/key/announcements", nil)
	req.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: "valid-token"})

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"status", "targeting", "created_by", "updated_by", "user_id", "api_key_id", "key_id"} {
		if strings.Contains(body, `"`+forbidden+`"`) {
			t.Fatalf("response contains forbidden field %q: %s", forbidden, body)
		}
	}
	var envelope struct {
		Data []struct {
			ID         int64      `json:"id"`
			Title      string     `json:"title"`
			Content    string     `json:"content"`
			NotifyMode string     `json:"notify_mode"`
			ReadAt     *time.Time `json:"read_at"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(envelope.Data) != 1 || envelope.Data[0].ID != 10 || envelope.Data[0].Title != "visible" || envelope.Data[0].NotifyMode != service.AnnouncementNotifyModePopup {
		t.Fatalf("unexpected announcement DTO: %+v", envelope.Data)
	}
}

func TestPublicKeyAnnouncementReadUsesSessionAPIKeyAndIgnoresSelectors(t *testing.T) {
	keyReadRepo := &publicKeyAnnouncementKeyReadRepoStub{}
	router, _, _ := newPublicKeyAnnouncementTestRouter(t, keyReadRepo)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/key/announcements/10/read?api_key_id=999&key_id=999&user_id=999", nil)
	req.AddCookie(&http.Cookie{Name: publicKeyUsageSessionCookie, Value: "valid-token"})

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s, want 200", recorder.Code, recorder.Body.String())
	}
	if len(keyReadRepo.marks) != 1 || keyReadRepo.marks[0].AnnouncementID != 10 || keyReadRepo.marks[0].APIKeyID != 101 {
		t.Fatalf("mark read used selector instead of session API key: %+v", keyReadRepo.marks)
	}
}

func newPublicKeyAnnouncementTestRouter(t *testing.T, keyReadRepo *publicKeyAnnouncementKeyReadRepoStub) (*gin.Engine, *service.APIKey, *publicKeyAnnouncementSessionCacheStub) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	apiKey := &service.APIKey{ID: 101, UserID: 1, Status: service.StatusAPIKeyExpired}
	sessionCache := &publicKeyAnnouncementSessionCacheStub{
		session: &service.PublicKeyUsageSession{
			APIKeyID:          apiKey.ID,
			UserID:            apiKey.UserID,
			AbsoluteExpiresAt: time.Now().Add(time.Hour),
		},
	}
	apiKeyService := service.NewAPIKeyService(
		&publicKeyAnnouncementAPIKeyRepoStub{key: apiKey},
		nil,
		nil,
		nil,
		nil,
		sessionCache,
		&config.Config{},
	)
	ann := service.Announcement{
		ID:         10,
		Title:      "visible",
		Content:    "content",
		Status:     service.AnnouncementStatusActive,
		NotifyMode: service.AnnouncementNotifyModePopup,
		CreatedAt:  time.Now().Add(-time.Hour),
		UpdatedAt:  time.Now().Add(-time.Hour),
	}
	announcementService := service.NewAnnouncementService(
		&publicKeyAnnouncementRepoStub{item: ann, active: []service.Announcement{ann}},
		&publicKeyAnnouncementUserReadRepoStub{},
		keyReadRepo,
		&publicKeyAnnouncementUserRepoStub{user: &service.User{ID: 1, Balance: 100}},
		&publicKeyAnnouncementSubRepoStub{},
	)
	handler := &GatewayHandler{
		apiKeyService:        apiKeyService,
		announcementService:  announcementService,
		settingService:       nil,
		memberBudgetService:  nil,
		opsService:           nil,
		usageService:         nil,
		billingCacheService:  nil,
		gatewayService:       nil,
		openAIGatewayService: nil,
	}
	router := gin.New()
	router.GET("/api/v1/key/usage-session", handler.GetPublicKeyUsageSession)
	router.GET("/api/v1/key/announcements", handler.ListPublicKeyAnnouncements)
	router.POST("/api/v1/key/announcements/:id/read", handler.MarkPublicKeyAnnouncementRead)
	return router, apiKey, sessionCache
}
