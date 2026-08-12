package routes

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberRouteAccountRepo struct {
	service.AccountRepository

	mu                sync.Mutex
	accountsByGroup   map[int64][]service.Account
	selectionGroupIDs []int64
}

func (r *enterpriseMemberRouteAccountRepo) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, accounts := range r.accountsByGroup {
		for i := range accounts {
			if accounts[i].ID == id {
				account := accounts[i]
				return &account, nil
			}
		}
	}
	return nil, service.ErrAccountNotFound
}

func (r *enterpriseMemberRouteAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, groupID int64, platform string) ([]service.Account, error) {
	r.mu.Lock()
	r.selectionGroupIDs = append(r.selectionGroupIDs, groupID)
	r.mu.Unlock()
	return r.accountsForGroupAndPlatforms(groupID, []string{platform}), nil
}

func (r *enterpriseMemberRouteAccountRepo) ListSchedulableByGroupIDAndPlatforms(_ context.Context, groupID int64, platforms []string) ([]service.Account, error) {
	r.mu.Lock()
	r.selectionGroupIDs = append(r.selectionGroupIDs, groupID)
	r.mu.Unlock()
	return r.accountsForGroupAndPlatforms(groupID, platforms), nil
}

func (r *enterpriseMemberRouteAccountRepo) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, _ bool) ([]service.Account, error) {
	if groupID == nil {
		return nil, nil
	}
	return r.accountsForGroupAndPlatforms(*groupID, platforms), nil
}

func (r *enterpriseMemberRouteAccountRepo) accountsForGroupAndPlatforms(groupID int64, platforms []string) []service.Account {
	allowed := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		allowed[platform] = struct{}{}
	}
	accounts := r.accountsByGroup[groupID]
	out := make([]service.Account, 0, len(accounts))
	for i := range accounts {
		if _, ok := allowed[accounts[i].Platform]; !ok {
			continue
		}
		out = append(out, accounts[i])
	}
	return out
}

func (r *enterpriseMemberRouteAccountRepo) selectedGroups() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.selectionGroupIDs...)
}

type enterpriseMemberRouteUpstream struct {
	service.HTTPUpstream

	mu         sync.Mutex
	accountIDs []int64
}

func (u *enterpriseMemberRouteUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.accountIDs = append(u.accountIDs, accountID)
	u.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(
			`{"id":"resp_enterprise_route_ok","object":"response","model":"gpt-5.6-sol","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`,
		)),
	}, nil
}

func (u *enterpriseMemberRouteUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func (u *enterpriseMemberRouteUpstream) calls() []int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]int64(nil), u.accountIDs...)
}

type enterpriseMemberRouteBillingRepo struct {
	service.UsageBillingRepository

	mu      sync.Mutex
	calls   int
	lastCmd *service.UsageBillingCommand
}

type enterpriseMemberRouteUserRepo struct {
	service.UserRepository
	user *service.User
}

func (r *enterpriseMemberRouteUserRepo) GetByID(_ context.Context, id int64) (*service.User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, service.ErrUserNotFound
	}
	user := *r.user
	return &user, nil
}

func (r *enterpriseMemberRouteBillingRepo) Apply(_ context.Context, cmd *service.UsageBillingCommand) (*service.UsageBillingApplyResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.lastCmd = cmd
	return &service.UsageBillingApplyResult{Applied: true, UsageLogPersisted: true}, nil
}

func (r *enterpriseMemberRouteBillingRepo) snapshot() (int, *service.UsageBillingCommand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls, r.lastCmd
}

func TestGatewayRoutesEnterpriseMemberResponsesRetriesCapabilityMismatchAndBillsSuccessfulGroupOnce(t *testing.T) {
	for _, requestPath := range []string{"/v1/responses", "/responses"} {
		t.Run(requestPath, func(t *testing.T) {
			router, accountRepo, upstream, billingRepo := newEnterpriseMemberResponsesRouteTestRouter(t)
			request := httptest.NewRequest(
				http.MethodPost,
				requestPath,
				bytes.NewBufferString(`{"model":"gpt-5.6-sol","input":"hello","stream":false}`),
			)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.Contains(t, response.Body.String(), "resp_enterprise_route_ok")
			require.Equal(t, []int64{11, 22}, accountRepo.selectedGroups())
			require.Equal(t, []int64{2200}, upstream.calls(), "the incompatible Anthropic group must not reach an upstream")

			billingCalls, billingCmd := billingRepo.snapshot()
			require.Equal(t, 1, billingCalls)
			require.NotNil(t, billingCmd)
			require.Equal(t, int64(2200), billingCmd.AccountID)
			require.NotNil(t, billingCmd.MemberID)
			require.Equal(t, int64(77), *billingCmd.MemberID)
			require.NotNil(t, billingCmd.UsageLog)
			require.NotNil(t, billingCmd.UsageLog.GroupID)
			require.Equal(t, int64(22), *billingCmd.UsageLog.GroupID)
			require.Equal(t, "gpt-5.6-sol", billingCmd.UsageLog.RequestedModel)
		})
	}
}

func newEnterpriseMemberResponsesRouteTestRouter(t *testing.T) (*gin.Engine, *enterpriseMemberRouteAccountRepo, *enterpriseMemberRouteUpstream, *enterpriseMemberRouteBillingRepo) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	accountRepo := &enterpriseMemberRouteAccountRepo{accountsByGroup: map[int64][]service.Account{
		11: {{
			ID:          1100,
			Name:        "anthropic-incompatible",
			Platform:    service.PlatformAnthropic,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"api_key":       "sk-anthropic-test",
				"model_mapping": map[string]any{"claude-sonnet-test": "claude-sonnet-test"},
			},
		}},
		22: {{
			ID:          2200,
			Name:        "openai-compatible",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeAPIKey,
			Status:      service.StatusActive,
			Schedulable: true,
			Credentials: map[string]any{
				"api_key":  "sk-openai-test",
				"base_url": "https://api.example.test",
			},
			Extra: map[string]any{"openai_passthrough": true},
		}},
	}}
	upstream := &enterpriseMemberRouteUpstream{}
	billingRepo := &enterpriseMemberRouteBillingRepo{}
	cfg := &config.Config{RunMode: config.RunModeStandard}
	cfg.Default.RateMultiplier = 1
	cfg.Gateway.MaxBodySize = 1024 * 1024
	cfg.Gateway.TextMaxBodySize = 1024 * 1024
	cfg.Gateway.MaxAccountSwitches = 2
	cfg.Gateway.Scheduling.LoadBatchEnabled = false

	user := &service.User{
		ID:          7,
		Role:        service.RoleUser,
		AccountType: service.UserAccountTypeEnterprise,
		Status:      service.StatusActive,
		Balance:     100,
	}
	userRepo := &enterpriseMemberRouteUserRepo{user: user}
	concurrencyService := service.NewConcurrencyService(nil)
	billingCacheService := service.NewBillingCacheService(nil, userRepo, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billingCacheService.Stop)
	billingService := service.NewBillingService(cfg, nil)
	deferredService := &service.DeferredService{}
	apiKeyService := service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg)
	openAIGatewayService := service.NewOpenAIGatewayService(
		accountRepo,
		nil,
		billingRepo,
		userRepo,
		nil,
		nil,
		nil,
		cfg,
		nil,
		concurrencyService,
		billingService,
		nil,
		billingCacheService,
		upstream,
		deferredService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	gatewayService := service.NewGatewayService(
		accountRepo,
		nil,
		nil,
		billingRepo,
		userRepo,
		nil,
		nil,
		nil,
		cfg,
		nil,
		concurrencyService,
		billingService,
		nil,
		billingCacheService,
		nil,
		upstream,
		deferredService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	gatewayHandler := handler.NewGatewayHandler(
		gatewayService,
		openAIGatewayService,
		nil,
		nil,
		nil,
		concurrencyService,
		billingCacheService,
		nil,
		apiKeyService,
		nil,
		nil,
		nil,
		nil,
		cfg,
		nil,
	)
	openAIGatewayHandler := handler.NewOpenAIGatewayHandler(
		openAIGatewayService,
		concurrencyService,
		billingCacheService,
		apiKeyService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
	)

	memberID := int64(77)
	memberGroups := []service.Group{
		{
			ID:       11,
			Name:     "anthropic-first",
			Platform: service.PlatformAnthropic,
			Status:   service.StatusActive,
			Hydrated: true,
		},
		{
			ID:       22,
			Name:     "openai-second",
			Platform: service.PlatformOpenAI,
			Status:   service.StatusActive,
			Hydrated: true,
		},
	}
	apiKey := &service.APIKey{
		ID:       700,
		UserID:   7,
		MemberID: &memberID,
		User:     user,
		Member: &service.EnterpriseMember{
			ID:               memberID,
			EnterpriseUserID: 7,
			Status:           service.EnterpriseMemberStatusActive,
			Version:          1,
			Groups:           memberGroups,
		},
	}
	auth := servermiddleware.APIKeyAuthMiddleware(func(c *gin.Context) {
		c.Set(string(servermiddleware.ContextKeyAPIKey), apiKey)
		c.Set(string(servermiddleware.ContextKeyUser), servermiddleware.AuthSubject{UserID: apiKey.UserID})
		c.Next()
	})

	router := gin.New()
	RegisterGatewayRoutes(
		router,
		&handler.Handlers{
			Gateway:       gatewayHandler,
			OpenAIGateway: openAIGatewayHandler,
			AsyncImage:    handler.NewAsyncImageHandler(nil, nil),
		},
		auth,
		apiKeyService,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		cfg,
	)
	return router, accountRepo, upstream, billingRepo
}
