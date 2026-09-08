package routes

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
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// These tests exercise production route registration with rejecting auth
// middleware. The auth implementations have their own credential tests; here
// the contract is that feedback cannot accidentally bypass those gates.
func TestFeedbackUserRouteRequiresJWT(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		APIKey:   handler.NewAPIKeyHandler(&service.APIKeyService{}),
		Feedback: handler.NewFeedbackHandler(nil, nil),
	}
	RegisterUserRoutes(router.Group("/api/v1"), handlers,
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Login required")
		}),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil,
	)

	for _, endpoint := range feedbackContractEndpoints("/api/v1/feedback", true) {
		for _, credential := range []string{"", "Bearer not-a-login-token"} {
			request := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", credential)
			request.AddCookie(&http.Cookie{Name: "sub2api_key_usage_session", Value: "a-query-cookie-is-not-a-login"})
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusUnauthorized, recorder.Code, endpoint.path)
		}
	}
}

type feedbackContractEndpoint struct{ method, path, body string }

func feedbackContractEndpoints(base string, allowCreate bool) []feedbackContractEndpoint {
	endpoints := []feedbackContractEndpoint{
		{http.MethodGet, base, ""},
		{http.MethodGet, base + "/7", ""},
		{http.MethodGet, base + "/7/messages", ""},
		{http.MethodPost, base + "/7/messages", `{"content":"需要帮助"}`},
		{http.MethodPost, base + "/7/close", `{}`},
		{http.MethodPost, base + "/7/read", `{"last_read_reply_id":0}`},
	}
	if allowCreate {
		endpoints = append(endpoints, feedbackContractEndpoint{http.MethodPost, base, `{"content":"需要帮助"}`})
	}
	return endpoints
}

func TestFeedbackAdminRoutesRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{Feedback: adminhandler.NewFeedbackHandler(nil)}}
	auth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			servermiddleware.AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "Login required")
			return
		}
		servermiddleware.AbortWithError(c, http.StatusForbidden, "FORBIDDEN", "Admin required")
	})
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, auth,
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil,
	)

	for _, endpoint := range feedbackContractEndpoints("/api/v1/admin/feedback", false) {
		for _, tc := range []struct {
			authorization string
			want          int
		}{
			{"", http.StatusUnauthorized},
			{"Bearer ordinary-user", http.StatusForbidden},
		} {
			request := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", tc.authorization)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, tc.want, recorder.Code, endpoint.path)
			require.NotContains(t, recorder.Body.String(), "user_email")
		}
	}
}

func TestFeedbackKeyRouteRequiresQueryCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{
		Feedback: handler.NewFeedbackHandler(nil, &service.APIKeyService{}),
	}
	cache := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	RegisterAuthRoutes(router.Group("/api/v1"), handlers,
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), rdb, nil, nil,
	)

	// Neither a site's JWT, a raw API Key, nor browser state can stand in for
	// the existing HttpOnly query session on this endpoint.
	for _, endpoint := range feedbackContractEndpoints("/api/v1/key/feedback", true) {
		for _, credential := range []string{"", "Bearer user-token", "Bearer raw-api-key"} {
			request := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", credential)
			request.Header.Set("X-Has-Session", "true")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			require.Equal(t, http.StatusUnauthorized, recorder.Code, endpoint.path)
			require.NotContains(t, recorder.Body.String(), "user_id")
		}
	}
	for _, endpoint := range feedbackContractEndpoints("/api/v1/key/feedback", true) {
		if endpoint.method != http.MethodPost {
			continue
		}
		request := httptest.NewRequest(endpoint.method, endpoint.path, strings.NewReader(endpoint.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Sec-Fetch-Site", "cross-site")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusForbidden, recorder.Code, endpoint.path)
	}
}

// The repository fake keeps route tests focused on the authority/DTO contract.
// SQL locking, ownership predicates and pagination are tested in repository tests.
type feedbackContractRepository struct {
	item     service.Feedback
	messages []service.FeedbackReply
	params   pagination.PaginationParams
	filter   service.FeedbackListFilters
	reads    map[service.FeedbackReader]int64
}

func (r *feedbackContractRepository) Create(_ context.Context, input service.FeedbackCreateInput) (*service.Feedback, error) {
	r.item = service.Feedback{ID: 7, Content: input.Content, Source: input.Source, UserID: input.UserID, APIKeyID: input.APIKeyID, MemberID: input.MemberID, Status: service.FeedbackStatusOpen, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	return &r.item, nil
}

func (r *feedbackContractRepository) List(ctx context.Context, params pagination.PaginationParams, filters service.FeedbackListFilters) ([]service.Feedback, *pagination.PaginationResult, error) {
	return r.ListByScope(ctx, params, filters, service.FeedbackScope{Kind: service.FeedbackActorAdmin}, service.FeedbackReader{})
}

func (r *feedbackContractRepository) ListByScope(_ context.Context, params pagination.PaginationParams, filters service.FeedbackListFilters, scope service.FeedbackScope, reader service.FeedbackReader) ([]service.Feedback, *pagination.PaginationResult, error) {
	r.params, r.filter = params, filters
	items := []service.Feedback{}
	if r.matches(scope) && (filters.Status == "" || r.item.Status == filters.Status) {
		item := r.item
		item.UnreadCount = r.unread(reader)
		items = append(items, item)
	}
	return items, &pagination.PaginationResult{Total: int64(len(items)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *feedbackContractRepository) matches(scope service.FeedbackScope) bool {
	if scope.Kind == service.FeedbackActorAdmin {
		return true
	}
	if scope.Kind == service.FeedbackActorUser {
		return r.item.Source == service.FeedbackSourceUser && r.item.UserID == scope.UserID
	}
	if scope.Kind == service.FeedbackSourceKey {
		return r.item.Source == service.FeedbackSourceKey && r.item.UserID == scope.UserID && r.item.APIKeyID != nil && *r.item.APIKeyID == scope.APIKeyID && ((r.item.MemberID == nil && scope.MemberID == nil) || (r.item.MemberID != nil && scope.MemberID != nil && *r.item.MemberID == *scope.MemberID))
	}
	return false
}

func (r *feedbackContractRepository) Get(_ context.Context, id int64, scope service.FeedbackScope, reader service.FeedbackReader) (*service.Feedback, error) {
	if id != r.item.ID || !r.matches(scope) {
		return nil, infraerrors.NotFound("FEEDBACK_NOT_FOUND", "feedback not found")
	}
	item := r.item
	item.UnreadCount = r.unread(reader)
	return &item, nil
}

func (r *feedbackContractRepository) ListReplies(ctx context.Context, id int64, params pagination.PaginationParams, scope service.FeedbackScope) ([]service.FeedbackReply, *pagination.PaginationResult, error) {
	if _, err := r.Get(ctx, id, scope, service.FeedbackReader{}); err != nil {
		return nil, nil, err
	}
	messages := append([]service.FeedbackReply{}, r.messages...)
	return messages, &pagination.PaginationResult{Total: int64(len(messages)), Page: params.Page, PageSize: params.PageSize, Pages: 1}, nil
}

func (r *feedbackContractRepository) CreateReply(ctx context.Context, input service.FeedbackReplyInput) (*service.FeedbackReply, error) {
	if _, err := r.Get(ctx, input.FeedbackID, input.Scope, service.FeedbackReader{}); err != nil {
		return nil, err
	}
	if r.item.Status == service.FeedbackStatusClosed {
		return nil, service.ErrFeedbackClosed
	}
	message := service.FeedbackReply{ID: int64(len(r.messages) + 1), FeedbackID: input.FeedbackID, AuthorRole: input.AuthorRole, Content: input.Content, CreatedAt: time.Now().UTC()}
	r.messages = append(r.messages, message)
	r.item.UpdatedAt = message.CreatedAt
	return &message, nil
}

func (r *feedbackContractRepository) Close(ctx context.Context, id int64, closedBy string, scope service.FeedbackScope, reader service.FeedbackReader) (*service.Feedback, error) {
	if _, err := r.Get(ctx, id, scope, service.FeedbackReader{}); err != nil {
		return nil, err
	}
	if r.item.Status != service.FeedbackStatusClosed {
		now := time.Now().UTC()
		r.item.Status, r.item.ClosedBy, r.item.ClosedAt, r.item.UpdatedAt = service.FeedbackStatusClosed, &closedBy, &now, now
	}
	item := r.item
	item.UnreadCount = r.unread(reader)
	return &item, nil
}

func (r *feedbackContractRepository) unread(reader service.FeedbackReader) int64 {
	if reader.ID <= 0 {
		return 0
	}
	cursor, seenOpening := r.reads[reader]
	incoming := service.FeedbackActorAdmin
	var count int64
	if reader.Kind == service.FeedbackActorAdmin {
		incoming = service.FeedbackActorUser
		if !seenOpening {
			count++
		}
	}
	for _, message := range r.messages {
		if message.AuthorRole == incoming && message.ID > cursor {
			count++
		}
	}
	return count
}

func (r *feedbackContractRepository) MarkRead(ctx context.Context, id int64, scope service.FeedbackScope, reader service.FeedbackReader, cursor int64) (*service.FeedbackReadState, error) {
	if _, err := r.Get(ctx, id, scope, reader); err != nil {
		return nil, err
	}
	valid := cursor == 0
	for _, message := range r.messages {
		if message.ID == cursor {
			valid = true
		}
	}
	if !valid {
		return nil, service.ErrFeedbackInvalidReadID
	}
	if r.reads == nil {
		r.reads = make(map[service.FeedbackReader]int64)
	}
	if previous := r.reads[reader]; previous > cursor {
		cursor = previous
	}
	r.reads[reader] = cursor
	return &service.FeedbackReadState{UnreadCount: r.unread(reader), LastReadReplyID: cursor}, nil
}

type feedbackContractCooldown struct{ claimed map[string]bool }

func (c *feedbackContractCooldown) ClaimFeedbackCooldown(_ context.Context, identity string, _ time.Duration) (bool, time.Duration, error) {
	if c.claimed == nil {
		c.claimed = make(map[string]bool)
	}
	if c.claimed[identity] {
		return false, 60 * time.Second, nil
	}
	c.claimed[identity] = true
	return true, 0, nil
}

func newFeedbackContractRouter(t *testing.T) (*gin.Engine, *feedbackContractRepository, *feedbackContractCooldown) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	repo := &feedbackContractRepository{item: service.Feedback{
		ID: 7, Source: service.FeedbackSourceUser, UserID: 42, UserEmail: "private-owner@example.test",
		Content: "<img src=x>\n保留原始文字", Status: service.FeedbackStatusOpen,
		CreatedAt: time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC),
	}}
	cooldown := &feedbackContractCooldown{}
	svc := service.NewFeedbackService(repo, cooldown)
	handlers := &handler.Handlers{
		APIKey:   handler.NewAPIKeyHandler(&service.APIKeyService{}),
		Feedback: handler.NewFeedbackHandler(svc, nil),
		Admin:    &handler.AdminHandlers{Feedback: adminhandler.NewFeedbackHandler(svc)},
	}
	RegisterUserRoutes(router.Group("/api/v1"), handlers,
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) {
			c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: 42})
			c.Next()
		}), servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, nil,
	)
	RegisterAdminRoutes(router.Group("/api/v1"), handlers,
		servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
			adminID := int64(900)
			if c.GetHeader("X-Test-Admin") == "other" {
				adminID = 901
			}
			c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: adminID})
			c.Next()
		}),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() }), nil, nil,
	)
	return router, repo, cooldown
}

func feedbackContractRequest(router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func requireFeedbackCustomerProjection(t *testing.T, body string) {
	t.Helper()
	for _, privateField := range []string{"user_id", "user_email", "api_key_id", "key_name", "key_prefix", "member_id", "private-owner@example.test"} {
		require.NotContains(t, body, privateField)
	}
}

func TestFeedbackRoutesPersonalHistoryAndAdministratorReplies(t *testing.T) {
	router, repo, _ := newFeedbackContractRouter(t)
	list := feedbackContractRequest(router, http.MethodGet, "/api/v1/feedback?status=open&page_size=1000&user_id=999", "")
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	require.Equal(t, 100, repo.params.PageSize)
	require.Equal(t, service.FeedbackStatusOpen, repo.filter.Status)
	require.Contains(t, list.Body.String(), `"total":1`)
	requireFeedbackCustomerProjection(t, list.Body.String())
	require.Equal(t, "no-store", list.Header().Get("Cache-Control"))

	adminReply := feedbackContractRequest(router, http.MethodPost, "/api/v1/admin/feedback/7/messages", `{"content":" 管理员回复 "}`)
	require.Equal(t, http.StatusCreated, adminReply.Code, adminReply.Body.String())
	require.Contains(t, adminReply.Body.String(), `"author_role":"admin"`)
	require.Contains(t, adminReply.Body.String(), `"retry_after":0`)
	userReply := feedbackContractRequest(router, http.MethodPost, "/api/v1/feedback/7/messages", `{"content":"用户补充"}`)
	require.Equal(t, http.StatusCreated, userReply.Code, userReply.Body.String())
	require.Contains(t, userReply.Body.String(), `"author_role":"user"`)
	require.Contains(t, userReply.Body.String(), `"retry_after":60`)

	messages := feedbackContractRequest(router, http.MethodGet, "/api/v1/feedback/7/messages", "")
	require.Equal(t, http.StatusOK, messages.Code, messages.Body.String())
	require.Contains(t, messages.Body.String(), "管理员回复")
	require.Contains(t, messages.Body.String(), "用户补充")
	require.Contains(t, messages.Body.String(), `"total":2`)
	requireFeedbackCustomerProjection(t, messages.Body.String())
	var detail struct {
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	response := feedbackContractRequest(router, http.MethodGet, "/api/v1/feedback/7", "")
	require.Equal(t, http.StatusOK, response.Code)
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &detail))
	require.Equal(t, "<img src=x>\n保留原始文字", detail.Data.Content)
	requireFeedbackCustomerProjection(t, response.Body.String())

	retry := feedbackContractRequest(router, http.MethodPost, "/api/v1/feedback/7/messages", `{"content":"重复发送"}`)
	require.Equal(t, http.StatusTooManyRequests, retry.Code)
	require.Equal(t, "60", retry.Header().Get("Retry-After"))
	require.Len(t, repo.messages, 2)
}

func TestFeedbackRoutesEitherParticipantClosesConversationPermanently(t *testing.T) {
	for _, closer := range []string{"user", "admin"} {
		t.Run(closer, func(t *testing.T) {
			router, repo, cooldown := newFeedbackContractRouter(t)
			base := "/api/v1/feedback"
			if closer == "admin" {
				base = "/api/v1/admin/feedback"
			}
			before := feedbackContractRequest(router, http.MethodPost, "/api/v1/admin/feedback/7/messages", `{"content":"关闭前的回复"}`)
			require.Equal(t, http.StatusCreated, before.Code, before.Body.String())
			closed := feedbackContractRequest(router, http.MethodPost, base+"/7/close", `{}`)
			require.Equal(t, http.StatusOK, closed.Code, closed.Body.String())
			require.Contains(t, closed.Body.String(), `"status":"closed"`)
			require.Equal(t, closer, *repo.item.ClosedBy)
			closedAt := *repo.item.ClosedAt
			for _, participant := range []string{"/api/v1/feedback", "/api/v1/admin/feedback"} {
				reply := feedbackContractRequest(router, http.MethodPost, participant+"/7/messages", `{"content":"关闭后不允许发送"}`)
				require.Equal(t, http.StatusConflict, reply.Code, reply.Body.String())
				require.Contains(t, reply.Body.String(), "FEEDBACK_CLOSED")
				again := feedbackContractRequest(router, http.MethodPost, participant+"/7/close", `{}`)
				require.Equal(t, http.StatusOK, again.Code, again.Body.String())
				history := feedbackContractRequest(router, http.MethodGet, participant+"/7/messages", "")
				require.Equal(t, http.StatusOK, history.Code)
				require.Contains(t, history.Body.String(), "关闭前的回复")
				reopened := feedbackContractRequest(router, http.MethodPatch, participant+"/7", `{"status":"open"}`)
				require.Equal(t, http.StatusNotFound, reopened.Code)
			}
			require.Equal(t, closer, *repo.item.ClosedBy)
			require.Equal(t, closedAt, *repo.item.ClosedAt)
			require.Len(t, repo.messages, 1)
			require.Empty(t, cooldown.claimed, "closed messages must not consume customer cooldown")
		})
	}
}

func TestFeedbackRoutesRejectOtherOwnersAndLoginAccessToKeyTickets(t *testing.T) {
	for _, scope := range []string{"other-user", "own-key"} {
		t.Run(scope, func(t *testing.T) {
			router, repo, cooldown := newFeedbackContractRouter(t)
			if scope == "other-user" {
				repo.item.UserID = 43
			} else {
				keyID := int64(101)
				repo.item.Source, repo.item.APIKeyID = service.FeedbackSourceKey, &keyID
			}
			for _, endpoint := range feedbackContractEndpoints("/api/v1/feedback", false) {
				rec := feedbackContractRequest(router, endpoint.method, endpoint.path+"?user_id=43&api_key_id=101", endpoint.body)
				if endpoint.path == "/api/v1/feedback" {
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
					require.Contains(t, rec.Body.String(), `"total":0`)
				} else {
					require.Equal(t, http.StatusNotFound, rec.Code, endpoint.path+": "+rec.Body.String())
				}
				require.NotContains(t, rec.Body.String(), "保留原始文字")
				requireFeedbackCustomerProjection(t, rec.Body.String())
			}
			require.Empty(t, cooldown.claimed)
			require.Empty(t, repo.messages)
			require.Equal(t, service.FeedbackStatusOpen, repo.item.Status)
			admin := feedbackContractRequest(router, http.MethodGet, "/api/v1/admin/feedback/7", "")
			require.Equal(t, http.StatusOK, admin.Code)
			require.Contains(t, admin.Body.String(), `"user_id"`)
		})
	}
}

func TestFeedbackRoutesCreateAndRepliesShareCustomerCooldown(t *testing.T) {
	router, repo, _ := newFeedbackContractRouter(t)
	created := feedbackContractRequest(router, http.MethodPost, "/api/v1/feedback", `{"content":"新工单"}`)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	message := feedbackContractRequest(router, http.MethodPost, "/api/v1/feedback/7/messages", `{"content":"紧接着补充"}`)
	require.Equal(t, http.StatusTooManyRequests, message.Code)
	require.Empty(t, repo.messages)
	closed := feedbackContractRequest(router, http.MethodPost, "/api/v1/feedback/7/close", `{}`)
	require.Equal(t, http.StatusOK, closed.Code, "closing does not wait for submit cooldown")
}

func TestFeedbackRoutesDoNotAcceptClientSuppliedActorsOrStatus(t *testing.T) {
	router, repo, cooldown := newFeedbackContractRouter(t)
	for _, base := range []string{"/api/v1/feedback", "/api/v1/admin/feedback"} {
		for _, body := range []string{
			`{"content":"冒充回复","author_role":"admin"}`,
			`{"content":"替他人回复","user_id":43}`,
			`{"content":"图片","attachment":"data:image/png;base64,example"}`,
		} {
			reply := feedbackContractRequest(router, http.MethodPost, base+"/7/messages", body)
			require.Equal(t, http.StatusBadRequest, reply.Code, reply.Body.String())
		}
		for _, body := range []string{`{"status":"open"}`, `{"closed_by":"admin"}`, `{} {}`} {
			close := feedbackContractRequest(router, http.MethodPost, base+"/7/close", body)
			require.Equal(t, http.StatusBadRequest, close.Code, close.Body.String())
		}
		invalid := feedbackContractRequest(router, http.MethodGet, base+"?status=processed", "")
		require.Equal(t, http.StatusBadRequest, invalid.Code, invalid.Body.String())
	}
	require.Empty(t, repo.messages)
	require.Empty(t, cooldown.claimed)
	require.Equal(t, service.FeedbackStatusOpen, repo.item.Status)
}

type feedbackContractKeyRepository struct {
	service.APIKeyRepository
	key service.APIKey
}

func (r *feedbackContractKeyRepository) GetByID(_ context.Context, id int64) (*service.APIKey, error) {
	if r.key.ID != id {
		return nil, service.ErrAPIKeyNotFound
	}
	key := r.key
	return &key, nil
}

type feedbackContractSessionCache struct {
	service.APIKeyCache
	session service.PublicKeyUsageSession
	hash    string
	deleted bool
}

func (s *feedbackContractSessionCache) CreatePublicKeyUsageSession(context.Context, string, *service.PublicKeyUsageSession, time.Duration) error {
	return nil
}
func (s *feedbackContractSessionCache) GetPublicKeyUsageSession(_ context.Context, hash string) (*service.PublicKeyUsageSession, error) {
	if hash != s.hash || s.deleted {
		return nil, service.ErrPublicKeyUsageSessionInvalid
	}
	session := s.session
	return &session, nil
}
func (s *feedbackContractSessionCache) RefreshPublicKeyUsageSession(context.Context, string, time.Duration) error {
	return nil
}
func (s *feedbackContractSessionCache) DeletePublicKeyUsageSession(context.Context, string) error {
	s.deleted = true
	return nil
}

func TestFeedbackRoutesKeyHolderConversationAndSiblingIsolation(t *testing.T) {
	router, repo, cooldown := newFeedbackContractRouter(t)
	keyID, memberID := int64(101), int64(9)
	repo.item.Source, repo.item.APIKeyID, repo.item.MemberID = service.FeedbackSourceKey, &keyID, &memberID
	keyRepo := &feedbackContractKeyRepository{key: service.APIKey{ID: keyID, UserID: 42, MemberID: &memberID}}
	token := "feedback-contract-test-query-cookie"
	hash := sha256.Sum256([]byte(token))
	sessions := &feedbackContractSessionCache{hash: hex.EncodeToString(hash[:]), session: service.PublicKeyUsageSession{APIKeyID: keyID, UserID: 42, MemberID: &memberID, CreatedAt: time.Now(), AbsoluteExpiresAt: time.Now().Add(time.Hour)}}
	keyService := service.NewAPIKeyService(keyRepo, nil, nil, nil, nil, sessions, &config.Config{})
	handlers := &handler.Handlers{Feedback: handler.NewFeedbackHandler(service.NewFeedbackService(repo, cooldown), keyService)}
	cache := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	RegisterAuthRoutes(router.Group("/api/v1"), handlers,
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), rdb, nil, nil,
	)
	keyRequest := func(method, path, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer unrelated-login-must-not-replace-query-cookie")
		request.AddCookie(&http.Cookie{Name: "sub2api_key_usage_session", Value: token})
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	admin := feedbackContractRequest(router, http.MethodPost, "/api/v1/admin/feedback/7/messages", `{"content":"管理员向 Key 用户回复"}`)
	require.Equal(t, http.StatusCreated, admin.Code, admin.Body.String())
	for _, path := range []string{"/api/v1/key/feedback?user_id=999", "/api/v1/key/feedback/7", "/api/v1/key/feedback/7/messages"} {
		rec := keyRequest(http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		requireFeedbackCustomerProjection(t, rec.Body.String())
	}
	unread := keyRequest(http.MethodGet, "/api/v1/key/feedback/7", "")
	require.Contains(t, unread.Body.String(), `"unread_count":1`)
	read := keyRequest(http.MethodPost, "/api/v1/key/feedback/7/read", `{"last_read_reply_id":1}`)
	require.Equal(t, http.StatusOK, read.Code, read.Body.String())
	require.Equal(t, int64(1), repo.reads[service.FeedbackReader{Kind: "key", ID: keyID}])
	require.NotContains(t, read.Body.String(), "user_id")
	reply := keyRequest(http.MethodPost, "/api/v1/key/feedback/7/messages", `{"content":"Key 用户补充"}`)
	require.Equal(t, http.StatusCreated, reply.Code, reply.Body.String())
	require.True(t, cooldown.claimed["key:101"])
	require.False(t, cooldown.claimed["user:42"])
	closed := keyRequest(http.MethodPost, "/api/v1/key/feedback/7/close", `{}`)
	require.Equal(t, http.StatusOK, closed.Code, closed.Body.String())
	requireFeedbackCustomerProjection(t, closed.Body.String())
	require.Equal(t, "user", *repo.item.ClosedBy)
	after := keyRequest(http.MethodPost, "/api/v1/key/feedback/7/messages", `{"content":"关闭后拒绝"}`)
	require.Equal(t, http.StatusConflict, after.Code, after.Body.String())
	history := keyRequest(http.MethodGet, "/api/v1/key/feedback/7/messages", "")
	require.Contains(t, history.Body.String(), "管理员向 Key 用户回复")
	require.Contains(t, history.Body.String(), "Key 用户补充")

	// A newly validated sibling Key under the same owner/member has a separate
	// authority; knowledge of the ticket ID cannot transfer that history.
	keyRepo.key.ID, sessions.session.APIKeyID = 102, 102
	for _, endpoint := range feedbackContractEndpoints("/api/v1/key/feedback", false) {
		rec := keyRequest(endpoint.method, endpoint.path, endpoint.body)
		if endpoint.path == "/api/v1/key/feedback" {
			require.Equal(t, http.StatusOK, rec.Code)
			require.Contains(t, rec.Body.String(), `"total":0`)
		} else {
			require.Equal(t, http.StatusNotFound, rec.Code, endpoint.path+": "+rec.Body.String())
		}
		require.NotContains(t, rec.Body.String(), "Key 用户补充")
	}
	require.False(t, cooldown.claimed["key:102"])

	// Reassignment invalidates even a still-live cookie before exposing data.
	otherMember := int64(10)
	keyRepo.key.MemberID = &otherMember
	invalid := keyRequest(http.MethodGet, "/api/v1/key/feedback", "")
	require.Equal(t, http.StatusUnauthorized, invalid.Code, invalid.Body.String())
	require.True(t, sessions.deleted)
	require.NotContains(t, invalid.Body.String(), "保留原始文字")
}

func TestFeedbackRoutesUnreadBelongsToViewerAndOnlyAcknowledgesFetchedMessages(t *testing.T) {
	router, repo, cooldown := newFeedbackContractRouter(t)
	assertUnread := func(base string, want int64) {
		t.Helper()
		rec := feedbackContractRequest(router, http.MethodGet, base+"/7", "")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Data struct {
				Unread int64 `json:"unread_count"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, want, body.Data.Unread, base)
	}
	markRead := func(base, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		rec := feedbackContractRequest(router, http.MethodPost, base+"/7/read", body)
		require.Equal(t, want, rec.Code, rec.Body.String())
		return rec
	}
	const userBase = "/api/v1/feedback"
	const adminBase = "/api/v1/admin/feedback"
	assertUnread(userBase, 0) // The customer's own opening message is not unread.
	assertUnread(adminBase, 1)
	// GET/list/detail/message reads have no hidden write side effect.
	feedbackContractRequest(router, http.MethodGet, adminBase+"/7/messages", "")
	assertUnread(adminBase, 1)
	markRead(adminBase, `{"last_read_reply_id":0}`, http.StatusOK)
	assertUnread(adminBase, 0)

	for i := 0; i < 2; i++ {
		rec := feedbackContractRequest(router, http.MethodPost, adminBase+"/7/messages", `{"content":"administrator reply"}`)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	assertUnread(adminBase, 0) // Replies from the reader's own side never light the dot.
	assertUnread(userBase, 2)
	markRead(userBase, `{"last_read_reply_id":1}`, http.StatusOK)
	assertUnread(userBase, 1) // Reply 2 arrived after the displayed snapshot of reply 1.
	markRead(userBase, `{"last_read_reply_id":2}`, http.StatusOK)
	late := markRead(userBase, `{"last_read_reply_id":1}`, http.StatusOK)
	require.Contains(t, late.Body.String(), `"last_read_reply_id":2`)
	assertUnread(userBase, 0)
	require.Empty(t, cooldown.claimed, "read receipts do not consume customer submission cooldown")

	userReply := feedbackContractRequest(router, http.MethodPost, userBase+"/7/messages", `{"content":"customer followup"}`)
	require.Equal(t, http.StatusCreated, userReply.Code)
	assertUnread(userBase, 0)
	assertUnread(adminBase, 1)
	closed := feedbackContractRequest(router, http.MethodPost, adminBase+"/7/close", `{}`)
	require.Equal(t, http.StatusOK, closed.Code)
	require.Contains(t, closed.Body.String(), `"unread_count":1`)
	assertUnread(adminBase, 1)
	markRead(adminBase, `{"last_read_reply_id":3}`, http.StatusOK)
	assertUnread(adminBase, 0)
	require.Equal(t, service.FeedbackStatusClosed, repo.item.Status)
}

func TestFeedbackRoutesAdministratorsKeepIndependentReadReceipts(t *testing.T) {
	router, _, _ := newFeedbackContractRouter(t)
	first := feedbackContractRequest(router, http.MethodPost, "/api/v1/admin/feedback/7/read", `{"last_read_reply_id":0}`)
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/feedback/7", nil)
	request.Header.Set("X-Test-Admin", "other")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), `"unread_count":1`)
	mine := feedbackContractRequest(router, http.MethodGet, "/api/v1/admin/feedback/7", "")
	require.Contains(t, mine.Body.String(), `"unread_count":0`)
}

func TestFeedbackRoutesReadReceiptRejectsForgedIdentityAndInvalidCursor(t *testing.T) {
	router, repo, cooldown := newFeedbackContractRouter(t)
	for _, base := range []string{"/api/v1/feedback", "/api/v1/admin/feedback"} {
		for _, body := range []string{
			`{}`, `{"last_read_reply_id":-1}`, `{"last_read_reply_id":99999}`,
			`{"last_read_reply_id":1.5}`, `{"last_read_reply_id":"1"}`,
			`{"last_read_reply_id":0,"reader_id":901}`,
			`{"last_read_reply_id":0,"reader_kind":"admin"}`,
			`{"last_read_reply_id":0} {}`,
		} {
			rec := feedbackContractRequest(router, http.MethodPost, base+"/7/read", body)
			require.Equal(t, http.StatusBadRequest, rec.Code, body+": "+rec.Body.String())
		}
	}
	require.Empty(t, repo.reads)
	require.Empty(t, cooldown.claimed)
}
