package admin

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountTestModelListItem struct {
	ID              string `json:"id"`
	DisplayName     string `json:"display_name"`
	UpstreamModelID string `json:"upstream_model_id"`
	IsPattern       bool   `json:"is_pattern,omitempty"`
	Disabled        bool   `json:"disabled,omitempty"`
}

type accountTestModelsHTTPUpstream struct {
	response *http.Response
	err      error
	calls    int
}

func (u *accountTestModelsHTTPUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	if u.err != nil {
		return nil, u.err
	}
	return u.response, nil
}

func (u *accountTestModelsHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, accountConcurrency)
}

func setupAvailableModelsRouterWithAccountTestService(adminSvc service.AdminService, accountTestSvc *service.AccountTestService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, accountTestSvc, nil, nil, nil, nil, nil)
	router.GET("/api/v1/admin/accounts/:id/models", handler.GetAvailableModels)
	router.GET("/api/v1/admin/accounts/:id/models/resolve", handler.ResolveAvailableModel)
	return router
}

func accountTestServiceWithOpenAIGateway(upstream service.HTTPUpstream) *service.AccountTestService {
	cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}}
	accountTestSvc := service.NewAccountTestService(nil, nil, nil, nil, nil, nil, cfg, nil)
	accountTestSvc.SetOpenAIGatewayService(service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, upstream, nil, nil, nil, nil, nil, nil, nil, nil,
	))
	return accountTestSvc
}

func fetchAccountTestModels(t *testing.T, router *gin.Engine, accountID int64) []accountTestModelListItem {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/"+strconv.FormatInt(accountID, 10)+"/models", nil)
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp struct {
		Data []accountTestModelListItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return resp.Data
}

func resolveAccountTestModel(t *testing.T, router *gin.Engine, accountID int64, modelID string) (int, string) {
	t.Helper()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/accounts/"+strconv.FormatInt(accountID, 10)+"/models/resolve?model_id="+url.QueryEscape(modelID),
		nil,
	)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		return rec.Code, ""
	}
	var resp struct {
		Data struct {
			UpstreamModelID string `json:"upstream_model_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	return rec.Code, resp.Data.UpstreamModelID
}

func mappedAccountForHandlerTest(id int64, platform, accountType string, mapping map[string]any, extra map[string]any) service.Account {
	credentials := map[string]any{
		"model_mapping": mapping,
	}
	if platform == service.PlatformAnthropic && accountType == service.AccountTypeBedrock {
		credentials["aws_region"] = "us-west-2"
	}
	for key, value := range extra {
		if strings.HasPrefix(key, "aws_") {
			credentials[key] = value
		}
	}
	parentID := int64(9000 + id)
	account := service.Account{
		ID:          id,
		Name:        platform + "-" + accountType,
		Platform:    platform,
		Type:        accountType,
		Status:      service.StatusActive,
		Credentials: credentials,
		Extra:       extra,
	}
	if extra != nil {
		if shadow, _ := extra["shadow"].(bool); shadow {
			account.ParentAccountID = &parentID
			account.QuotaDimension = service.QuotaDimensionSpark
			delete(account.Extra, "shadow")
		}
	}
	return account
}

func standardAccountTestMappingForPlatform(platform, accountType string) map[string]any {
	if platform == service.PlatformAnthropic && accountType == service.AccountTypeBedrock {
		return map[string]any{
			"alpha-public": "claude-sonnet-4-5",
			"beta-public":  "claude-sonnet-4-5",
			"wild-*":       "claude-opus-4-6",
		}
	}
	if platform == service.PlatformOpenAI {
		return map[string]any{
			"alpha-public": "gpt-5.6-sol",
			"beta-public":  "gpt-5.6-sol",
			"wild-*":       "gpt-5.4",
		}
	}
	if platform == service.PlatformGemini || platform == service.PlatformAntigravity {
		return map[string]any{
			"alpha-public": "gemini-2.5-pro",
			"beta-public":  "gemini-2.5-pro",
			"wild-*":       "gemini-2.5-flash",
		}
	}
	if platform == service.PlatformGrok {
		return map[string]any{
			"alpha-public": "grok-4.5",
			"beta-public":  "grok-4.5",
			"wild-*":       "grok-imagine",
		}
	}
	return map[string]any{
		"alpha-public": platform + "-upstream",
		"beta-public":  platform + "-upstream",
		"wild-*":       platform + "-wild-upstream",
	}
}

func TestAccountHandlerGetAvailableModels_ExplicitMappingsEnumerateRawAliasesForAllAccountTypes(t *testing.T) {
	for idx, tt := range []struct {
		name        string
		platform    string
		accountType string
		extra       map[string]any
	}{
		{name: "anthropic api key", platform: service.PlatformAnthropic, accountType: service.AccountTypeAPIKey},
		{name: "anthropic oauth", platform: service.PlatformAnthropic, accountType: service.AccountTypeOAuth},
		{name: "anthropic setup token", platform: service.PlatformAnthropic, accountType: service.AccountTypeSetupToken},
		{name: "anthropic bedrock", platform: service.PlatformAnthropic, accountType: service.AccountTypeBedrock},
		{name: "anthropic service account", platform: service.PlatformAnthropic, accountType: service.AccountTypeServiceAccount},
		{name: "openai api key", platform: service.PlatformOpenAI, accountType: service.AccountTypeAPIKey},
		{name: "openai oauth", platform: service.PlatformOpenAI, accountType: service.AccountTypeOAuth},
		{name: "openai setup token", platform: service.PlatformOpenAI, accountType: service.AccountTypeSetupToken},
		{name: "openai shadow", platform: service.PlatformOpenAI, accountType: service.AccountTypeOAuth, extra: map[string]any{"shadow": true}},
		{name: "openai raw chat fallback passthrough", platform: service.PlatformOpenAI, accountType: service.AccountTypeAPIKey, extra: map[string]any{
			"openai_passthrough":                true,
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		}},
		{name: "gemini api key", platform: service.PlatformGemini, accountType: service.AccountTypeAPIKey},
		{name: "gemini oauth", platform: service.PlatformGemini, accountType: service.AccountTypeOAuth},
		{name: "gemini google one", platform: service.PlatformGemini, accountType: service.AccountTypeOAuth, extra: map[string]any{"oauth_type": "google_one"}},
		{name: "gemini service account", platform: service.PlatformGemini, accountType: service.AccountTypeServiceAccount},
		{name: "antigravity api key", platform: service.PlatformAntigravity, accountType: service.AccountTypeAPIKey},
		{name: "antigravity oauth", platform: service.PlatformAntigravity, accountType: service.AccountTypeOAuth},
		{name: "antigravity upstream", platform: service.PlatformAntigravity, accountType: service.AccountTypeUpstream},
		{name: "grok api key", platform: service.PlatformGrok, accountType: service.AccountTypeAPIKey},
		{name: "grok oauth", platform: service.PlatformGrok, accountType: service.AccountTypeOAuth},
		{name: "kimi api key", platform: service.PlatformKimi, accountType: service.AccountTypeAPIKey},
		{name: "zhipu api key", platform: service.PlatformZhipu, accountType: service.AccountTypeAPIKey},
		{name: "deepseek api key", platform: service.PlatformDeepseek, accountType: service.AccountTypeAPIKey},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mapping := standardAccountTestMappingForPlatform(tt.platform, tt.accountType)
			account := mappedAccountForHandlerTest(int64(7000+idx), tt.platform, tt.accountType, mapping, tt.extra)
			svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: account}
			router := setupAvailableModelsRouter(svc)

			models := fetchAccountTestModels(t, router, account.ID)

			require.Len(t, models, 3)
			require.Equal(t, []string{"alpha-public", "beta-public", "wild-*"}, []string{models[0].ID, models[1].ID, models[2].ID})
			require.True(t, models[2].IsPattern)
			require.False(t, models[0].Disabled)
			require.False(t, models[1].Disabled)
			require.False(t, models[2].Disabled)

			for _, model := range models {
				wantUpstream, err := service.ResolveAccountTestModel(&account, model.ID)
				require.NoError(t, err)
				require.Equal(t, wantUpstream, model.UpstreamModelID)
				if wantUpstream == model.ID {
					require.Equal(t, model.ID, model.DisplayName)
				} else {
					require.Equal(t, model.ID+" → "+wantUpstream, model.DisplayName)
				}
			}
			require.Equal(t, models[0].UpstreamModelID, models[1].UpstreamModelID, "duplicate upstream targets must keep separate aliases")
			require.NotEqual(t, "GPT-5.6 Sol", models[0].DisplayName)
		})
	}
}

func TestAccountHandlerGetAvailableModels_OpenAIExplicitMappingBypassesUpstreamDiscovery(t *testing.T) {
	account := mappedAccountForHandlerTest(7101, service.PlatformOpenAI, service.AccountTypeAPIKey, map[string]any{
		"configured-model": "gpt-5.6-sol",
	}, nil)
	upstream := &accountTestModelsHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"discovered-model"}]}`)),
	}}
	accountTestSvc := accountTestServiceWithOpenAIGateway(upstream)
	router := setupAvailableModelsRouterWithAccountTestService(
		&availableModelsAdminService{stubAdminService: newStubAdminService(), account: account},
		accountTestSvc,
	)

	models := fetchAccountTestModels(t, router, account.ID)

	require.Equal(t, 0, upstream.calls)
	require.Equal(t, []string{"configured-model"}, []string{models[0].ID})
}

func TestAccountHandlerGetAvailableModels_OpenAIWithoutMappingUsesUpstreamDiscovery(t *testing.T) {
	account := service.Account{
		ID:       7102,
		Name:     "openai-discovered",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://provider.example/v1",
		},
	}
	upstream := &accountTestModelsHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"discovered-model"}]}`)),
	}}
	accountTestSvc := accountTestServiceWithOpenAIGateway(upstream)
	router := setupAvailableModelsRouterWithAccountTestService(
		&availableModelsAdminService{stubAdminService: newStubAdminService(), account: account},
		accountTestSvc,
	)

	models := fetchAccountTestModels(t, router, account.ID)

	require.Equal(t, 1, upstream.calls)
	require.Equal(t, []string{"discovered-model"}, []string{models[0].ID})
}

func TestAccountHandlerGetAvailableModels_OpenAIWithoutMappingFallsBackWhenDiscoveryFails(t *testing.T) {
	account := service.Account{
		ID:       7103,
		Name:     "openai-discovery-fallback",
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"api_key":  "test-key",
			"base_url": "https://provider.example/v1",
		},
	}
	upstream := &accountTestModelsHTTPUpstream{err: context.Canceled}
	accountTestSvc := accountTestServiceWithOpenAIGateway(upstream)
	router := setupAvailableModelsRouterWithAccountTestService(
		&availableModelsAdminService{stubAdminService: newStubAdminService(), account: account},
		accountTestSvc,
	)

	models := fetchAccountTestModels(t, router, account.ID)

	require.Equal(t, 1, upstream.calls)
	require.NotEmpty(t, models)
	require.Equal(t, "gpt-5.6-sol", models[0].ID)
}

func TestAccountHandlerGetAvailableModels_AntigravityConfiguredListDoesNotIncludeImplicitKeys(t *testing.T) {
	account := mappedAccountForHandlerTest(7104, service.PlatformAntigravity, service.AccountTypeOAuth, map[string]any{
		"only-configured": "gemini-2.5-pro",
	}, nil)
	svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: account}
	router := setupAvailableModelsRouter(svc)

	models := fetchAccountTestModels(t, router, account.ID)

	require.Len(t, models, 1)
	require.Equal(t, []string{"only-configured"}, []string{models[0].ID})
	require.NotContains(t, models[0].ID, "gemini-3.1-pro-preview")
}

func TestAccountHandlerGetAvailableModels_DisablesRowsWhenResolvedTargetIsInvalid(t *testing.T) {
	for _, tt := range []struct {
		name        string
		account     service.Account
		disabledIDs []string
	}{
		{
			name: "empty target",
			account: mappedAccountForHandlerTest(7200, service.PlatformAnthropic, service.AccountTypeAPIKey, map[string]any{
				"empty-target": "   ",
			}, nil),
			disabledIDs: []string{"empty-target"},
		},
		{
			name: "wildcard target",
			account: mappedAccountForHandlerTest(7201, service.PlatformAnthropic, service.AccountTypeAPIKey, map[string]any{
				"wildcard-target": "upstream-*",
			}, nil),
			disabledIDs: []string{"wildcard-target"},
		},
		{
			name: "unknown bedrock target",
			account: mappedAccountForHandlerTest(7202, service.PlatformAnthropic, service.AccountTypeBedrock, map[string]any{
				"unknown-bedrock": "not-a-bedrock-model",
			}, nil),
			disabledIDs: []string{"unknown-bedrock"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: tt.account}
			router := setupAvailableModelsRouter(svc)

			models := fetchAccountTestModels(t, router, tt.account.ID)

			require.Len(t, models, len(tt.disabledIDs))
			for i, model := range models {
				require.Equal(t, tt.disabledIDs[i], model.ID)
				require.True(t, model.Disabled)
			}
		})
	}
}

func TestAccountHandlerResolveAvailableModelRejectsInvalidResolvedTargets(t *testing.T) {
	for _, tt := range []struct {
		name    string
		account service.Account
		modelID string
	}{
		{
			name: "empty target",
			account: mappedAccountForHandlerTest(7300, service.PlatformAnthropic, service.AccountTypeAPIKey, map[string]any{
				"empty-target": "   ",
			}, nil),
			modelID: "empty-target",
		},
		{
			name: "wildcard target",
			account: mappedAccountForHandlerTest(7301, service.PlatformAnthropic, service.AccountTypeAPIKey, map[string]any{
				"wildcard-target": "upstream-*",
			}, nil),
			modelID: "wildcard-target",
		},
		{
			name: "unknown bedrock target",
			account: mappedAccountForHandlerTest(7302, service.PlatformAnthropic, service.AccountTypeBedrock, map[string]any{
				"unknown-bedrock": "not-a-bedrock-model",
			}, nil),
			modelID: "unknown-bedrock",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: tt.account}
			router := setupAvailableModelsRouter(svc)

			code, _ := resolveAccountTestModel(t, router, tt.account.ID, tt.modelID)

			require.Equal(t, http.StatusBadRequest, code)
		})
	}
}

func TestAccountHandlerResolveAvailableModelUsesSpecializedAccountResolvers(t *testing.T) {
	for _, tt := range []struct {
		name    string
		account service.Account
		modelID string
		want    string
	}{
		{
			name: "bedrock adjusts region",
			account: mappedAccountForHandlerTest(7400, service.PlatformAnthropic, service.AccountTypeBedrock, map[string]any{
				"bedrock-public": "claude-sonnet-4-5",
			}, nil),
			modelID: "bedrock-public",
			want:    "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		},
		{
			name: "vertex normalizes dated claude id",
			account: service.Account{
				ID:          7401,
				Name:        "vertex",
				Platform:    service.PlatformAnthropic,
				Type:        service.AccountTypeServiceAccount,
				Status:      service.StatusActive,
				Credentials: map[string]any{},
			},
			modelID: "claude-sonnet-4-5-20250929",
			want:    "claude-sonnet-4-5@20250929",
		},
		{
			name: "grok media alias resolves final image model",
			account: service.Account{
				ID:          7402,
				Name:        "grok-media",
				Platform:    service.PlatformGrok,
				Type:        service.AccountTypeOAuth,
				Status:      service.StatusActive,
				Credentials: map[string]any{},
			},
			modelID: "grok-imagine",
			want:    "grok-imagine-image-quality",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := &availableModelsAdminService{stubAdminService: newStubAdminService(), account: tt.account}
			router := setupAvailableModelsRouter(svc)

			code, upstream := resolveAccountTestModel(t, router, tt.account.ID, tt.modelID)

			require.Equal(t, http.StatusOK, code)
			require.Equal(t, tt.want, upstream)
		})
	}
}
