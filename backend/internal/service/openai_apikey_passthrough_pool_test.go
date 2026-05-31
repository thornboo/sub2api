package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newOpenAIAPIKeyPoolAccountForTest() *Account {
	return &Account{
		ID:          701,
		Name:        "openai-apikey-pool",
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"api_key":       "legacy-openai-key",
			"base_url":      "https://relay.example.com",
			"model_mapping": map[string]any{"site-gpt": "relay-gpt-account"},
		},
		Extra:       map[string]any{"openai_passthrough": true},
		Status:      StatusActive,
		Schedulable: true,
	}
}

func TestOpenAIGatewayService_APIKeyPassthrough_KeyPoolFailoverCoolsOnlySelectedKeyModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := []byte(`{"model":"site-gpt","stream":false,"input":"hello"}`)
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusTooManyRequests,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-key-1"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limit on selected upstream key","type":"rate_limit_error","code":"rate_limit_exceeded"}}`)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-key-2"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"resp_1","model":"relay-gpt-account","output":[],"usage":{"input_tokens":3,"output_tokens":2}}`)),
			},
		},
	}
	repo := &accountAPIKeyRuntimeRepoRecorder{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		accountRepo:  repo,
	}
	account := newOpenAIAPIKeyPoolAccountForTest()
	account.APIKeys = []AccountAPIKey{
		{ID: 7101, AccountID: account.ID, Name: "key-1", APIKey: "upstream-openai-key-1", Priority: 1, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"relay-gpt-account": "relay-gpt-account"}},
		{ID: 7102, AccountID: account.ID, Name: "key-2", APIKey: "upstream-openai-key-2", Priority: 2, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"relay-gpt-account": "relay-gpt-account"}},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 2)
	require.Equal(t, "Bearer upstream-openai-key-1", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "Bearer upstream-openai-key-2", upstream.requests[1].Header.Get("Authorization"))
	require.Equal(t, "relay-gpt-account", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "relay-gpt-account", gjson.GetBytes(upstream.bodies[1], "model").String())
	require.Equal(t, "site-gpt", gjson.GetBytes(rec.Body.Bytes(), "model").String())
	require.Len(t, repo.cooldowns, 1)
	require.Equal(t, int64(7101), repo.cooldowns[0].keyID)
	require.Equal(t, "relay-gpt-account", repo.cooldowns[0].upstreamModel)
	require.Equal(t, http.StatusTooManyRequests, repo.cooldowns[0].statusCode)
	require.Len(t, repo.used, 2)
	require.Equal(t, int64(7101), repo.used[0].keyID)
	require.True(t, repo.used[0].failed)
	require.Equal(t, int64(7102), repo.used[1].keyID)
	require.False(t, repo.used[1].failed)
}

func TestOpenAIGatewayService_APIKeyPassthrough_KeyPoolUsesPerKeyModelMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := []byte(`{"model":"site-gpt","stream":false,"input":"hello"}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid-key-map"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_2","model":"relay-gpt-key-2","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}
	repo := &accountAPIKeyRuntimeRepoRecorder{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		accountRepo:  repo,
	}
	account := newOpenAIAPIKeyPoolAccountForTest()
	account.APIKeys = []AccountAPIKey{
		{ID: 7201, AccountID: account.ID, Name: "key-other-model", APIKey: "upstream-openai-other", Priority: 1, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "mapping", ModelMapping: map[string]string{"other-site-model": "relay-other"}},
		{ID: 7202, AccountID: account.ID, Name: "key-site-gpt", APIKey: "upstream-openai-site", Priority: 2, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "mapping", ModelMapping: map[string]string{"site-gpt": "relay-gpt-key-2"}},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer upstream-openai-site", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "relay-gpt-key-2", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Equal(t, "site-gpt", gjson.GetBytes(rec.Body.Bytes(), "model").String())
	require.Len(t, repo.used, 1)
	require.Equal(t, int64(7202), repo.used[0].keyID)
	require.False(t, repo.used[0].failed)
}

func TestOpenAIGatewayService_APIKeyPassthrough_KeyPoolDoesNotFailoverPlainBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := []byte(`{"model":"site-gpt","stream":false,"input":"hello"}`)
	respBody := `{"error":{"message":"Invalid value for 'input': expected an array","type":"invalid_request_error"}}`
	upstream := &httpUpstreamRecorder{
		responses: []*http.Response{
			{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(respBody)),
			},
			{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"id":"unexpected"}`)),
			},
		},
	}
	repo := &accountAPIKeyRuntimeRepoRecorder{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		accountRepo:  repo,
	}
	account := newOpenAIAPIKeyPoolAccountForTest()
	account.APIKeys = []AccountAPIKey{
		{ID: 7301, AccountID: account.ID, Name: "key-1", APIKey: "upstream-openai-key-1", Priority: 1, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"relay-gpt-account": "relay-gpt-account"}},
		{ID: 7302, AccountID: account.ID, Name: "key-2", APIKey: "upstream-openai-key-2", Priority: 2, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"relay-gpt-account": "relay-gpt-account"}},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Nil(t, result)
	require.Error(t, err)
	require.Len(t, upstream.requests, 1)
	require.Empty(t, repo.cooldowns)
	require.Empty(t, repo.used)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOpenAIGatewayService_APIKeyPassthrough_KeyPoolSkipsCooledKeyModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	body := []byte(`{"model":"site-gpt","stream":false,"input":"hello"}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"resp_3","model":"relay-gpt-account","output":[],"usage":{"input_tokens":1,"output_tokens":1}}`)),
	}}
	repo := &accountAPIKeyRuntimeRepoRecorder{}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{},
		httpUpstream: upstream,
		accountRepo:  repo,
	}
	account := newOpenAIAPIKeyPoolAccountForTest()
	account.APIKeys = []AccountAPIKey{
		{
			ID:                   7401,
			AccountID:            account.ID,
			Name:                 "key-cooled",
			APIKey:               "upstream-openai-cooled",
			Priority:             1,
			Status:               AccountAPIKeyStatusActive,
			ModelRestrictionMode: "whitelist",
			ModelMapping:         map[string]string{"relay-gpt-account": "relay-gpt-account"},
			ModelCooldowns: map[string]AccountAPIKeyModelCooldown{
				"relay-gpt-account": {UpstreamModel: "relay-gpt-account", CooldownUntil: time.Now().Add(time.Hour)},
			},
		},
		{ID: 7402, AccountID: account.ID, Name: "key-available", APIKey: "upstream-openai-available", Priority: 2, Status: AccountAPIKeyStatusActive, ModelRestrictionMode: "whitelist", ModelMapping: map[string]string{"relay-gpt-account": "relay-gpt-account"}},
	}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "Bearer upstream-openai-available", upstream.requests[0].Header.Get("Authorization"))
	require.Empty(t, repo.cooldowns)
	require.Len(t, repo.used, 1)
	require.Equal(t, int64(7402), repo.used[0].keyID)
}
