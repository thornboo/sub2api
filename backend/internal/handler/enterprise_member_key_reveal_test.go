package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberKeyRevealRepoStub struct {
	service.EnterpriseMemberRepository
	key       *service.APIKey
	memberErr error
	keyErr    error
}

func (r *enterpriseMemberKeyRevealRepoStub) GetByOwnerAndID(_ context.Context, ownerID, memberID int64, includeArchived bool) (*service.EnterpriseMember, error) {
	if ownerID != 7 || memberID != 41 || includeArchived {
		return nil, service.ErrEnterpriseMemberNotFound
	}
	if r.memberErr != nil {
		return nil, r.memberErr
	}
	return &service.EnterpriseMember{ID: memberID, EnterpriseUserID: ownerID, Status: service.EnterpriseMemberStatusActive}, nil
}

func (r *enterpriseMemberKeyRevealRepoStub) GetKey(_ context.Context, ownerID, memberID, keyID int64) (*service.APIKey, error) {
	if ownerID != 7 || memberID != 41 || keyID != 28 {
		return nil, service.ErrAPIKeyNotFound
	}
	if r.keyErr != nil {
		return nil, r.keyErr
	}
	return r.key, nil
}

type enterpriseMemberKeyRevealOwnerRepoStub struct{ service.UserRepository }

func (r *enterpriseMemberKeyRevealOwnerRepoStub) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Role: service.RoleUser, AccountType: service.UserAccountTypeEnterprise, Status: service.StatusActive}, nil
}

type enterpriseMemberKeyRevealAuditRepoStub struct {
	service.EnterpriseMemberAuditRepository
	err   error
	calls [][4]int64
}

func (r *enterpriseMemberKeyRevealAuditRepoStub) RecordKeyReveal(_ context.Context, ownerID, memberID, actorUserID, keyID int64) error {
	r.calls = append(r.calls, [4]int64{ownerID, memberID, actorUserID, keyID})
	return r.err
}

func TestEnterpriseMemberRevealKeyAuditsBeforeReturningMinimalSecretPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auditRepo := &enterpriseMemberKeyRevealAuditRepoStub{}
	h := newEnterpriseMemberKeyRevealTestHandler(auditRepo)
	recorder := performEnterpriseMemberKeyRevealRequest(h, 7, "/enterprise/members/41/keys/28/reveal")

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, [][4]int64{{7, 41, 7, 28}}, auditRepo.calls)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "no-cache", recorder.Header().Get("Pragma"))

	var body struct {
		Data enterpriseMemberKeyRevealResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Equal(t, enterpriseMemberKeyRevealResponse{ID: 28, MemberID: 41, Key: "sk-plaintext-secret"}, body.Data)
	require.NotContains(t, recorder.Body.String(), "key-name")
	require.NotContains(t, recorder.Body.String(), "user_id")
	require.NotContains(t, recorder.Body.String(), "status")
}

func TestEnterpriseMemberRevealKeyFailsClosedWhenAuditWriteFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	auditRepo := &enterpriseMemberKeyRevealAuditRepoStub{err: errors.New("audit storage unavailable")}
	h := newEnterpriseMemberKeyRevealTestHandler(auditRepo)
	recorder := performEnterpriseMemberKeyRevealRequest(h, 7, "/enterprise/members/41/keys/28/reveal")

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "sk-plaintext-secret")
	require.Contains(t, recorder.Body.String(), "ENTERPRISE_MEMBER_KEY_REVEAL_UNAVAILABLE")
}

func TestEnterpriseMemberRevealKeyRejectsCrossMemberAndCrossOwnerAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name   string
		userID int64
		path   string
	}{
		{name: "different member", userID: 7, path: "/enterprise/members/42/keys/28/reveal"},
		{name: "different owner", userID: 8, path: "/enterprise/members/41/keys/28/reveal"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			auditRepo := &enterpriseMemberKeyRevealAuditRepoStub{}
			h := newEnterpriseMemberKeyRevealTestHandler(auditRepo)
			recorder := performEnterpriseMemberKeyRevealRequest(h, testCase.userID, testCase.path)

			require.Equal(t, http.StatusNotFound, recorder.Code)
			require.Empty(t, auditRepo.calls)
			require.NotContains(t, recorder.Body.String(), "sk-plaintext-secret")
		})
	}
}

func TestEnterpriseMemberRevealKeyRejectsArchivedMembersAndDeletedKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, testCase := range []struct {
		name string
		repo *enterpriseMemberKeyRevealRepoStub
	}{
		{name: "archived member", repo: &enterpriseMemberKeyRevealRepoStub{memberErr: service.ErrEnterpriseMemberNotFound}},
		{name: "deleted key", repo: &enterpriseMemberKeyRevealRepoStub{keyErr: service.ErrAPIKeyNotFound}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			auditRepo := &enterpriseMemberKeyRevealAuditRepoStub{}
			testCase.repo.key = enterpriseMemberKeyRevealFixture()
			h := newEnterpriseMemberKeyRevealTestHandlerWithRepo(testCase.repo, auditRepo)
			recorder := performEnterpriseMemberKeyRevealRequest(h, 7, "/enterprise/members/41/keys/28/reveal")

			require.Equal(t, http.StatusNotFound, recorder.Code)
			require.Empty(t, auditRepo.calls)
			require.NotContains(t, recorder.Body.String(), "sk-plaintext-secret")
		})
	}
}

func newEnterpriseMemberKeyRevealTestHandler(auditRepo service.EnterpriseMemberAuditRepository) *EnterpriseMemberHandler {
	memberRepo := &enterpriseMemberKeyRevealRepoStub{key: enterpriseMemberKeyRevealFixture()}
	return newEnterpriseMemberKeyRevealTestHandlerWithRepo(memberRepo, auditRepo)
}

func newEnterpriseMemberKeyRevealTestHandlerWithRepo(memberRepo service.EnterpriseMemberRepository, auditRepo service.EnterpriseMemberAuditRepository) *EnterpriseMemberHandler {
	memberService := service.NewEnterpriseMemberService(memberRepo, &enterpriseMemberKeyRevealOwnerRepoStub{}, nil, auditRepo)
	return NewEnterpriseMemberHandler(memberService, nil, nil, auditRepo)
}

func enterpriseMemberKeyRevealFixture() *service.APIKey {
	return &service.APIKey{ID: 28, UserID: 7, MemberID: int64Pointer(41), Name: "key-name", Key: "sk-plaintext-secret"}
}

func performEnterpriseMemberKeyRevealRequest(h *EnterpriseMemberHandler, userID int64, path string) *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/enterprise/members/:id/keys/:key_id/reveal", func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: userID})
		h.RevealKey(c)
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, nil)
	router.ServeHTTP(recorder, req)
	return recorder
}

func int64Pointer(value int64) *int64 { return &value }
