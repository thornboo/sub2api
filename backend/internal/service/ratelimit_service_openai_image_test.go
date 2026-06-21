//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestIsOpenAIImageRateLimitError(t *testing.T) {
	imageBody := []byte(`{"error":{"message":"Rate limit reached for gpt-image-2-codex (for limit gpt-image) in organization org on input-images per min: Limit 4000, Used 4000. Please try again in 467ms."}}`)
	textBody := []byte(`{"error":{"message":"Rate limit reached for gpt-5.4 in organization org on tokens per min: Limit 30000, Used 30000. Please try again in 1s."}}`)

	require.True(t, isOpenAIImageRateLimitError(http.StatusTooManyRequests, imageBody))
	require.False(t, isOpenAIImageRateLimitError(http.StatusTooManyRequests, textBody))
	require.False(t, isOpenAIImageRateLimitError(http.StatusBadRequest, imageBody))
}

func TestRateLimitService_HandleOpenAIImageRateLimit_ParsesTryAgainCooldown(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 201, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-image-2-codex (for limit gpt-image) on input-images per min. Please try again in 2s."}}`)

	before := time.Now()
	handled := svc.HandleOpenAIImageRateLimit(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body)

	require.True(t, handled)
	require.Len(t, repo.modelRateLimitCalls, 1)
	call := repo.modelRateLimitCalls[0]
	require.Equal(t, account.ID, call.accountID)
	require.Equal(t, openAIImageGenerationRateLimitKey, call.scope)
	require.Equal(t, openAIImageRateLimitReason, call.reason)
	require.WithinDuration(t, before.Add(2*time.Second), call.resetAt, time.Second)
}

func TestRateLimitService_HandleOpenAIImageRateLimit_DefaultsToOneMinute(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 202, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-image-2-codex (for limit gpt-image) on input-images per min."}}`)

	before := time.Now()
	handled := svc.HandleOpenAIImageRateLimit(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body)

	require.True(t, handled)
	require.Len(t, repo.modelRateLimitCalls, 1)
	call := repo.modelRateLimitCalls[0]
	require.Equal(t, openAIImageGenerationRateLimitKey, call.scope)
	require.Equal(t, openAIImageRateLimitReason, call.reason)
	require.WithinDuration(t, before.Add(time.Minute), call.resetAt, time.Second)
}

func TestRateLimitService_HandleOpenAIModelRateLimit_ParsesTryAgainCooldown(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 204, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-5.4 in organization org on tokens per min. Please try again in 2s."}}`)

	before := time.Now()
	handled := svc.HandleOpenAIModelRateLimit(context.Background(), account, "gpt-5.4", http.StatusTooManyRequests, http.Header{}, body)

	require.True(t, handled)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Zero(t, repo.rateLimitedCalls)
	call := repo.modelRateLimitCalls[0]
	require.Equal(t, account.ID, call.accountID)
	require.Equal(t, "gpt-5.4", call.scope)
	require.Equal(t, openAIModelRateLimitReason, call.reason)
	require.WithinDuration(t, before.Add(2*time.Second), call.resetAt, time.Second)
}

func TestRateLimitService_HandleOpenAIModelRateLimit_SkipsModelMismatch(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 205, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-5.3 in organization org on tokens per min. Please try again in 2s."}}`)

	handled := svc.HandleOpenAIModelRateLimit(context.Background(), account, "gpt-5.4", http.StatusTooManyRequests, http.Header{}, body)

	require.False(t, handled)
	require.Empty(t, repo.modelRateLimitCalls)
}

func TestRateLimitService_HandleOpenAIModelRateLimit_SkipsWhenScopeIsEmpty(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 216, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-5.4 in organization org on tokens per min. Please try again in 2s."}}`)

	handled := svc.HandleOpenAIModelRateLimit(context.Background(), account, "", http.StatusTooManyRequests, http.Header{}, body)

	require.False(t, handled)
	require.Empty(t, repo.modelRateLimitCalls)
}

func TestRateLimitService_HandleUpstreamError_OpenAIModel429UsesModelRateLimit(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 206, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-5.4 in organization org on tokens per min. Please try again in 1s."}}`)

	shouldFailover := svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body, "gpt-5.4")

	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Zero(t, repo.rateLimitedCalls)
	require.Equal(t, "gpt-5.4", repo.modelRateLimitCalls[0].scope)
}

func TestRateLimitService_HandleModelScopedFailure_Upstream500UsesModelCooldownAndFailover(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 208, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"server_error","message":"upstream model backend error"}}`)

	before := time.Now()
	handled, shouldFailover := svc.HandleModelScopedFailure(context.Background(), account, "gpt-5.4", http.StatusInternalServerError, http.Header{}, body)

	require.True(t, handled)
	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Zero(t, repo.rateLimitedCalls)
	call := repo.modelRateLimitCalls[0]
	require.Equal(t, account.ID, call.accountID)
	require.Equal(t, "gpt-5.4", call.scope)
	require.Equal(t, modelUpstreamFailureReason, call.reason)
	require.WithinDuration(t, before.Add(modelUpstreamFailureCooldown), call.resetAt, time.Second)
}

func TestRateLimitService_HandleModelScopedFailure_Generic400UsesModelCooldownAndFailover(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 213, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"upstream_error","message":"provider returned a model backend error"}}`)

	handled, shouldFailover := svc.HandleModelScopedFailure(context.Background(), account, "gpt-5.4", http.StatusBadRequest, http.Header{}, body)

	require.True(t, handled)
	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, "gpt-5.4", repo.modelRateLimitCalls[0].scope)
	require.Equal(t, modelUpstreamFailureReason, repo.modelRateLimitCalls[0].reason)
}

func TestRateLimitService_HandleModelScopedFailure_Generic403UsesModelCooldownAndFailover(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 214, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"provider_error","message":"model temporarily unavailable on this provider"}}`)

	handled, shouldFailover := svc.HandleModelScopedFailure(context.Background(), account, "gpt-5.4", http.StatusForbidden, http.Header{}, body)

	require.True(t, handled)
	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, "gpt-5.4", repo.modelRateLimitCalls[0].scope)
	require.Equal(t, modelUpstreamFailureReason, repo.modelRateLimitCalls[0].reason)
}

func TestRateLimitService_HandleModelScopedFailure_Ambiguous429UsesRequestedModel(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 209, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"upstream overloaded, retry later"}}`)

	handled, shouldFailover := svc.HandleModelScopedFailure(context.Background(), account, "gpt-5.4", http.StatusTooManyRequests, http.Header{}, body)

	require.True(t, handled)
	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, "gpt-5.4", repo.modelRateLimitCalls[0].scope)
	require.Equal(t, modelUpstreamFailureReason, repo.modelRateLimitCalls[0].reason)
}

func TestRateLimitService_HandleModelScopedFailure_BillingMessageFallsThrough(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &RateLimitService{accountRepo: repo}
	account := &Account{ID: 210, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"server_error","message":"credit balance exhausted"}}`)

	handled, shouldFailover := svc.HandleModelScopedFailure(context.Background(), account, "gpt-5.4", http.StatusInternalServerError, http.Header{}, body)

	require.False(t, handled)
	require.False(t, shouldFailover)
	require.Empty(t, repo.modelRateLimitCalls)
}

func TestOpenAIGatewayService_HandleOpenAIAccountUpstreamError_ImageRateLimitDoesNotBlockWholeAccount(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
	account := &Account{ID: 203, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-image-2-codex (for limit gpt-image) on input-images per min. Please try again in 1s."}}`)

	disabled := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body, "gpt-image-2")

	require.False(t, disabled)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, openAIImageGenerationRateLimitKey, repo.modelRateLimitCalls[0].scope)
	_, wholeAccountBlocked := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, wholeAccountBlocked)
}

func TestOpenAIGatewayService_HandleOpenAIAccountUpstreamError_TextModelRateLimitDoesNotBlockWholeAccount(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
	account := &Account{ID: 207, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-5.4 in organization org on tokens per min. Please try again in 1s."}}`)

	shouldFailover := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body, "gpt-5.4")

	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, openAIModelRateLimitReason, repo.modelRateLimitCalls[0].reason)
	_, wholeAccountBlocked := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, wholeAccountBlocked)
}

func TestOpenAIGatewayService_HandleOpenAIAccountUpstreamError_Upstream500FailsOverWithoutWholeAccountBlock(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
	account := &Account{ID: 211, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"server_error","message":"upstream model backend error"}}`)

	shouldFailover := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusInternalServerError, http.Header{}, body, "gpt-5.4")

	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, modelUpstreamFailureReason, repo.modelRateLimitCalls[0].reason)
	_, wholeAccountBlocked := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, wholeAccountBlocked)
}

func TestOpenAIGatewayService_HandleOpenAIAccountUpstreamError_Generic400FailsOverWithoutWholeAccountBlock(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
	account := &Account{ID: 215, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"provider_error","message":"model temporarily unavailable"}}`)

	shouldFailover := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusBadRequest, http.Header{}, body, "gpt-5.4")

	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, modelUpstreamFailureReason, repo.modelRateLimitCalls[0].reason)
	_, wholeAccountBlocked := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, wholeAccountBlocked)
}

func TestOpenAIGatewayService_HandleOpenAIAccountUpstreamError_Ambiguous429DoesNotBlockWholeAccount(t *testing.T) {
	repo := &modelNotFoundAccountRepoStub{}
	svc := &OpenAIGatewayService{rateLimitService: &RateLimitService{accountRepo: repo}}
	account := &Account{ID: 212, Platform: PlatformOpenAI, Type: AccountTypeOAuth}
	body := []byte(`{"error":{"type":"rate_limit_exceeded","message":"upstream overloaded, retry later"}}`)

	shouldFailover := svc.handleOpenAIAccountUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{}, body, "gpt-5.4")

	require.True(t, shouldFailover)
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, "gpt-5.4", repo.modelRateLimitCalls[0].scope)
	require.Equal(t, modelUpstreamFailureReason, repo.modelRateLimitCalls[0].reason)
	_, wholeAccountBlocked := svc.openaiAccountRuntimeBlockUntil.Load(account.ID)
	require.False(t, wholeAccountBlocked)
}

func TestOpenAIGatewayServiceForwardImages_ImageRateLimitReturnsFailoverAndCoolsCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &modelNotFoundAccountRepoStub{}
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a cat"}`)
	errorBody := `{"error":{"type":"rate_limit_exceeded","message":"Rate limit reached for gpt-image-2-codex (for limit gpt-image) in organization org on input-images per min: Limit 4000, Used 4000. Please try again in 1s."}}`

	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	svc := &OpenAIGatewayService{
		rateLimitService: &RateLimitService{accountRepo: repo},
		httpUpstream: &httpUpstreamRecorder{
			resp: &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"X-Request-Id": []string{"req_img_rate_limited"}},
				Body:       io.NopCloser(strings.NewReader(errorBody)),
			},
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	account := &Account{
		ID:       204,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "token-123",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	require.Contains(t, string(failoverErr.ResponseBody), "input-images per min")
	require.Len(t, repo.modelRateLimitCalls, 1)
	require.Equal(t, openAIImageGenerationRateLimitKey, repo.modelRateLimitCalls[0].scope)
}
