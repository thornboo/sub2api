package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type accountTestModelHTTPUpstream struct {
	requests []*http.Request
	bodies   [][]byte
}

func (u *accountTestModelHTTPUpstream) Do(req *http.Request, proxyURL string, accountID int64, accountConcurrency int) (*http.Response, error) {
	return u.record(req), nil
}

func (u *accountTestModelHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, profile *tlsfingerprint.Profile) (*http.Response, error) {
	return u.record(req), nil
}

func (u *accountTestModelHTTPUpstream) record(req *http.Request) *http.Response {
	var body []byte
	if req != nil && req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	u.requests = append(u.requests, req)
	u.bodies = append(u.bodies, body)
	return accountTestModelSuccessResponse(req)
}

func accountTestModelSuccessResponse(req *http.Request) *http.Response {
	body := "data: [DONE]\n\n"
	if req != nil {
		path := req.URL.Path
		switch {
		case strings.Contains(path, "/images/generations"):
			body = `{ "data": [{ "b64_json": "aGVsbG8=" }] }`
		case strings.Contains(path, "/chat/completions"):
			body = "data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
		case strings.Contains(path, "/responses"):
			body = "data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"ig_123\",\"type\":\"image_generation_call\",\"result\":\"aGVsbG8=\",\"output_format\":\"png\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n"
		}
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type accountTestModelTokenCache struct {
	GeminiTokenCache
	token string
}

func (c *accountTestModelTokenCache) GetAccessToken(context.Context, string) (string, error) {
	return c.token, nil
}

func newAccountTestModelService(upstream *accountTestModelHTTPUpstream, repo AccountRepository) *AccountTestService {
	tokenCache := &accountTestModelTokenCache{token: "test-access-token"}
	antigravityGateway := NewAntigravityGatewayService(
		repo,
		nil,
		nil,
		NewAntigravityTokenProvider(repo, tokenCache, nil),
		nil,
		upstream,
		nil,
		nil,
	)
	return NewAccountTestService(
		repo,
		NewGeminiTokenProvider(repo, tokenCache, nil),
		NewClaudeTokenProvider(repo, tokenCache, nil),
		nil,
		antigravityGateway,
		upstream,
		&config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		&TLSFingerprintProfileService{},
	)
}

func newAccountTestModelContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/test", nil)
	return ctx, rec
}

func TestOpenAIAccountConnectionUsesResolvedModelInActualRequest(t *testing.T) {
	for _, tt := range []struct {
		name        string
		account     *Account
		modelID     string
		wantURLPart string
		wantModel   string
	}{
		{
			name: "single hop only",
			account: accountTestModelActualAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{
				"a-model": "b-model",
				"b-model": "c-model",
			}, nil),
			modelID:     "a-model",
			wantURLPart: "/v1/responses",
			wantModel:   "b-model",
		},
		{
			name: "apikey trims mapped target like preview",
			account: accountTestModelActualAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{
				"spaced": " upstream-model ",
			}, nil),
			modelID:     "spaced",
			wantURLPart: "/v1/responses",
			wantModel:   "upstream-model",
		},
		{
			name: "rawchat passthrough still maps once",
			account: accountTestModelActualAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{
				"a-model": "b-model",
				"b-model": "c-model",
			}, map[string]any{
				"openai_passthrough":                true,
				openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
			}),
			modelID:     "a-model",
			wantURLPart: "/v1/chat/completions",
			wantModel:   "b-model",
		},
		{
			name: "true passthrough ignores normal mapping",
			account: accountTestModelActualAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{
				"a-model": "b-model",
			}, map[string]any{
				"openai_passthrough":                     true,
				openai_compat.ExtraKeyResponsesSupported: true,
			}),
			modelID:     "a-model",
			wantURLPart: "/v1/responses",
			wantModel:   "a-model",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &accountTestModelHTTPUpstream{}
			svc := newAccountTestModelService(upstream, nil)
			ctx, rec := newAccountTestModelContext()

			err := svc.testOpenAIAccountConnection(ctx, tt.account, tt.modelID, "", AccountTestModeDefault)

			require.NoError(t, err)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Len(t, upstream.requests, 1)
			require.Contains(t, upstream.requests[0].URL.String(), tt.wantURLPart)
			require.Equal(t, tt.wantModel, gjson.GetBytes(upstream.bodies[0], "model").String())
		})
	}
}

func TestOpenAIAccountConnectionRoutesMappedOAuthImageTarget(t *testing.T) {
	upstream := &accountTestModelHTTPUpstream{}
	svc := newAccountTestModelService(upstream, nil)
	ctx, rec := newAccountTestModelContext()
	account := accountTestModelActualAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"image-public": "gpt-image-2",
	}, nil)

	err := svc.testOpenAIAccountConnection(ctx, account, "image-public", "draw a cat", AccountTestModeDefault)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, chatgptCodexAPIURL, upstream.requests[0].URL.String())
	require.Equal(t, openAIImagesResponsesMainModel, gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.bodies[0], "tools.0.model").String())
}

func TestOpenAIAccountConnectionRejectsInvalidMappedTargetBeforeNormalize(t *testing.T) {
	for _, tt := range []struct {
		name    string
		target  string
		wantErr string
	}{
		{name: "empty", target: "   ", wantErr: "empty upstream model"},
		{name: "wildcard", target: "bad-*", wantErr: "wildcard upstream model"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &accountTestModelHTTPUpstream{}
			svc := newAccountTestModelService(upstream, nil)
			ctx, _ := newAccountTestModelContext()
			account := accountTestModelActualAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{
				"public-model": tt.target,
			}, nil)

			err := svc.testOpenAIAccountConnection(ctx, account, "public-model", "", AccountTestModeDefault)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
			require.Empty(t, upstream.requests)
		})
	}
}

func TestOpenAIShadowAccountUsesChildMappingAndParentCodexCredentials(t *testing.T) {
	upstream := &accountTestModelHTTPUpstream{}
	parentID := int64(7001)
	parent := accountTestModelActualAccount(PlatformOpenAI, AccountTypeOAuth, nil, nil)
	parent.ID = parentID
	child := accountTestModelActualAccount(PlatformOpenAI, AccountTypeOAuth, map[string]any{
		"public-codex": "gpt-5.6-max",
	}, nil)
	child.ID = 7002
	child.ParentAccountID = &parentID
	repo := &accountTestModelRepo{accountsByID: map[int64]*Account{parentID: parent}}
	svc := newAccountTestModelService(upstream, repo)
	ctx, _ := newAccountTestModelContext()

	err := svc.testOpenAIAccountConnection(ctx, child, "public-codex", "", AccountTestModeDefault)

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer test-openai-token", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(upstream.bodies[0], "model").String())
}

func TestGeminiAccountConnectionUsesWildcardAndOAuthMappingInActualRequest(t *testing.T) {
	for _, tt := range []struct {
		name        string
		accountType string
		mapping     map[string]any
		modelID     string
		wantURLPart string
	}{
		{
			name:        "apikey wildcard",
			accountType: AccountTypeAPIKey,
			mapping:     map[string]any{"gemini-public-*": "gemini-2.5-pro"},
			modelID:     "gemini-public-alpha",
			wantURLPart: "/models/gemini-2.5-pro:streamGenerateContent",
		},
		{
			name:        "oauth exact",
			accountType: AccountTypeOAuth,
			mapping:     map[string]any{"gemini-public": "gemini-2.5-flash"},
			modelID:     "gemini-public",
			wantURLPart: "/models/gemini-2.5-flash:streamGenerateContent",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &accountTestModelHTTPUpstream{}
			svc := newAccountTestModelService(upstream, nil)
			ctx, _ := newAccountTestModelContext()
			account := accountTestModelActualAccount(PlatformGemini, tt.accountType, tt.mapping, nil)

			err := svc.testGeminiAccountConnection(ctx, account, tt.modelID, "")

			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			require.Contains(t, upstream.requests[0].URL.String(), tt.wantURLPart)
		})
	}
}

func TestAntigravityAPIKeyRoutesByMappedCrossProtocolTarget(t *testing.T) {
	upstream := &accountTestModelHTTPUpstream{}
	svc := newAccountTestModelService(upstream, nil)
	ctx, _ := newAccountTestModelContext()
	account := accountTestModelActualAccount(PlatformAntigravity, AccountTypeAPIKey, map[string]any{
		"claude-public": "gemini-2.5-pro",
	}, nil)

	err := svc.routeAntigravityTest(ctx, account, "claude-public", "")

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Contains(t, upstream.requests[0].URL.String(), "/models/gemini-2.5-pro:streamGenerateContent")
}

func TestAntigravityOAuthBuildsBodyFromMappedTargetProtocol(t *testing.T) {
	upstream := &accountTestModelHTTPUpstream{}
	svc := newAccountTestModelService(upstream, nil)
	ctx, _ := newAccountTestModelContext()
	account := accountTestModelActualAccount(PlatformAntigravity, AccountTypeOAuth, map[string]any{
		"claude-public": "gemini-2.5-pro",
	}, nil)

	err := svc.testAntigravityAccountConnection(ctx, account, "claude-public")

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "gemini-2.5-pro", gjson.GetBytes(upstream.bodies[0], "model").String())
}

func TestClaudeOAuthAccountConnectionUsesMappedModelInActualRequest(t *testing.T) {
	upstream := &accountTestModelHTTPUpstream{}
	svc := newAccountTestModelService(upstream, nil)
	ctx, _ := newAccountTestModelContext()
	account := accountTestModelActualAccount(PlatformAnthropic, AccountTypeOAuth, map[string]any{
		"claude-public": "claude-upstream",
	}, nil)

	err := svc.testClaudeAccountConnection(ctx, account, "claude-public")

	require.NoError(t, err)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, testClaudeAPIURL, upstream.requests[0].URL.String())
	require.Equal(t, "claude-upstream", gjson.GetBytes(upstream.bodies[0], "model").String())
}

func accountTestModelActualAccount(platform, accountType string, mapping map[string]any, extra map[string]any) *Account {
	credentials := map[string]any{
		"api_key":      "test-api-key",
		"access_token": "test-openai-token",
		"base_url":     "https://api.openai.com",
		"project_id":   "test-project",
	}
	if platform == PlatformAnthropic {
		credentials["base_url"] = "https://api.anthropic.com"
	}
	if platform == PlatformGemini {
		credentials["base_url"] = "https://generativelanguage.googleapis.com"
		if accountType == AccountTypeOAuth {
			delete(credentials, "project_id")
		}
	}
	if platform == PlatformAntigravity {
		credentials["base_url"] = "https://generativelanguage.googleapis.com"
		credentials["project_id"] = "test-project"
	}
	if mapping != nil {
		credentials["model_mapping"] = mapping
	}
	return &Account{
		ID:          8001,
		Name:        platform + "-" + accountType,
		Platform:    platform,
		Type:        accountType,
		Concurrency: 1,
		Credentials: credentials,
		Extra:       extra,
	}
}

type accountTestModelRepo struct {
	AccountRepository
	accountsByID map[int64]*Account
}

func (r *accountTestModelRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	if account, ok := r.accountsByID[id]; ok {
		return account, nil
	}
	return nil, fmt.Errorf("account %d not found", id)
}

func (r *accountTestModelRepo) SetError(context.Context, int64, string) error            { return nil }
func (r *accountTestModelRepo) UpdateExtra(context.Context, int64, map[string]any) error { return nil }
func (r *accountTestModelRepo) BulkUpdate(context.Context, []int64, AccountBulkUpdate) (int64, error) {
	return 0, nil
}
func (r *accountTestModelRepo) SetRateLimited(context.Context, int64, time.Time) error { return nil }
func (r *accountTestModelRepo) ClearError(context.Context, int64) error                { return nil }
func (r *accountTestModelRepo) List(context.Context, pagination.PaginationParams) ([]Account, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func TestGrokAccountConnectionUsesPreviewTargetWithoutMappingTwice(t *testing.T) {
	for _, mode := range []string{AccountTestModeDefault, AccountTestModeGrokImage} {
		t.Run(mode, func(t *testing.T) {
			account := accountTestModelActualAccount(PlatformGrok, AccountTypeAPIKey, map[string]any{
				"image-public": "grok-imagine",
				"grok-imagine": "grok-imagine-video",
			}, nil)
			upstream := &accountTestModelHTTPUpstream{}
			svc := newAccountTestModelService(upstream, nil)
			ctx, _ := newAccountTestModelContext()

			err := svc.testGrokAccountConnection(ctx, account, "image-public", "a cat", mode, AccountTestOptions{})

			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			require.Contains(t, upstream.requests[0].URL.Path, "/images/generations")
			require.Equal(t, "grok-imagine-image-quality", gjson.GetBytes(upstream.bodies[0], "model").String())
		})
	}
}

func TestGrokAccountConnectionRejectsInvalidMappedTargets(t *testing.T) {
	for _, mode := range []string{AccountTestModeDefault, AccountTestModeGrokText, AccountTestModeGrokImage, AccountTestModeGrokVideo} {
		for _, target := range []string{"", "bad-*"} {
			t.Run(mode+"/"+target, func(t *testing.T) {
				account := accountTestModelActualAccount(PlatformGrok, AccountTypeAPIKey, map[string]any{"public": target}, nil)
				upstream := &accountTestModelHTTPUpstream{}
				svc := newAccountTestModelService(upstream, nil)
				ctx, _ := newAccountTestModelContext()

				err := svc.testGrokAccountConnection(ctx, account, "public", "", mode, AccountTestOptions{})

				require.Error(t, err)
				require.Empty(t, upstream.requests)
			})
		}
	}
}

func TestScheduledAccountConnectionMapsRequestIDOnce(t *testing.T) {
	account := accountTestModelActualAccount(PlatformOpenAI, AccountTypeAPIKey, map[string]any{
		"a-model": "b-model", "b-model": "c-model",
	}, nil)
	upstream := &accountTestModelHTTPUpstream{}
	repo := &accountTestModelRepo{accountsByID: map[int64]*Account{account.ID: account}}
	svc := newAccountTestModelService(upstream, repo)

	result, err := svc.RunTestBackground(context.Background(), account.ID, "a-model")

	require.NoError(t, err)
	require.Equal(t, "success", result.Status)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "b-model", gjson.GetBytes(upstream.bodies[0], "model").String())
}
