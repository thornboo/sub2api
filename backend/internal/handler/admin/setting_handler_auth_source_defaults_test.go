package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type settingHandlerRepoStub struct {
	values      map[string]string
	lastUpdates map[string]string
}

func (s *settingHandlerRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *settingHandlerRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if s.values != nil {
		if value, ok := s.values[key]; ok {
			return value, nil
		}
	}
	return "", nil
}

func (s *settingHandlerRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *settingHandlerRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *settingHandlerRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	s.lastUpdates = make(map[string]string, len(settings))
	for key, value := range settings {
		s.lastUpdates[key] = value
		if s.values == nil {
			s.values = map[string]string{}
		}
		s.values[key] = value
	}
	return nil
}

func (s *settingHandlerRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *settingHandlerRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

type failingAuthSourceSettingsRepoStub struct {
	values map[string]string
	err    error
}

func (s *failingAuthSourceSettingsRepoStub) Get(ctx context.Context, key string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *failingAuthSourceSettingsRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *failingAuthSourceSettingsRepoStub) Set(ctx context.Context, key, value string) error {
	panic("unexpected Set call")
}

func (s *failingAuthSourceSettingsRepoStub) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *failingAuthSourceSettingsRepoStub) SetMultiple(ctx context.Context, settings map[string]string) error {
	if _, ok := settings[service.SettingKeyAuthSourceDefaultEmailBalance]; ok {
		return s.err
	}
	for key, value := range settings {
		if s.values == nil {
			s.values = map[string]string{}
		}
		s.values[key] = value
	}
	return nil
}

func (s *failingAuthSourceSettingsRepoStub) GetAll(ctx context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *failingAuthSourceSettingsRepoStub) Delete(ctx context.Context, key string) error {
	panic("unexpected Delete call")
}

func TestSettingHandler_GetSettings_InjectsAuthSourceDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled:                 "true",
			service.SettingKeyPromoCodeEnabled:                    "true",
			service.SettingKeyAuthSourceDefaultEmailBalance:       "9.5",
			service.SettingKeyAuthSourceDefaultEmailConcurrency:   "8",
			service.SettingKeyAuthSourceDefaultEmailSubscriptions: `[{"group_id":31,"validity_days":15}]`,
			service.SettingKeyForceEmailOnThirdPartySignup:        "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)

	handler.GetSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, 9.5, data["auth_source_default_email_balance"])
	require.Equal(t, float64(8), data["auth_source_default_email_concurrency"])
	require.Equal(t, true, data["force_email_on_third_party_signup"])

	subscriptions, ok := data["auth_source_default_email_subscriptions"].([]any)
	require.True(t, ok)
	require.Len(t, subscriptions, 1)
}

func TestSettingHandler_UpdateSettings_PreservesOmittedAuthSourceDefaults(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled:                    "false",
			service.SettingKeyPromoCodeEnabled:                       "true",
			service.SettingKeyAuthSourceDefaultEmailBalance:          "9.5",
			service.SettingKeyAuthSourceDefaultEmailConcurrency:      "8",
			service.SettingKeyAuthSourceDefaultEmailSubscriptions:    `[{"group_id":31,"validity_days":15}]`,
			service.SettingKeyAuthSourceDefaultEmailGrantOnSignup:    "true",
			service.SettingKeyAuthSourceDefaultEmailGrantOnFirstBind: "false",
			service.SettingKeyForceEmailOnThirdPartySignup:           "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"registration_enabled":              true,
		"promo_code_enabled":                true,
		"auth_source_default_email_balance": 12.75,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "12.75000000", repo.values[service.SettingKeyAuthSourceDefaultEmailBalance])
	require.Equal(t, "8", repo.values[service.SettingKeyAuthSourceDefaultEmailConcurrency])
	require.Equal(t, `[{"group_id":31,"validity_days":15}]`, repo.values[service.SettingKeyAuthSourceDefaultEmailSubscriptions])
	require.Equal(t, "true", repo.values[service.SettingKeyForceEmailOnThirdPartySignup])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, 12.75, data["auth_source_default_email_balance"])
	require.Equal(t, float64(8), data["auth_source_default_email_concurrency"])
	require.Equal(t, true, data["force_email_on_third_party_signup"])
}

func TestSettingHandler_UpdateSettings_PreservesOmittedDevZZOperationalSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:                     "true",
			service.SettingKeyScheduleStrategy:                     service.ScheduleStrategyCostFirst,
			service.SettingKeyNativeModelProtocolRoutingEnabled:    "true",
			service.SettingKeyModelSelfCheckEnabled:                "true",
			service.SettingKeyModelSelfCheckDefaultIntervalSeconds: "900",
			service.SettingKeyModelSelfCheckMaxConcurrency:         "12",
			service.SettingKeyModelSelfCheckMaxTasksPerRound:       "1200",
			service.SettingKeyModelSelfCheckSnapshotRetentionDays:  "180",
			service.SettingKeyDisableKeysOnRateChange:              "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled": true,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.ScheduleStrategyCostFirst, repo.values[service.SettingKeyScheduleStrategy])
	require.Equal(t, "true", repo.values[service.SettingKeyNativeModelProtocolRoutingEnabled])
	require.Equal(t, "true", repo.values[service.SettingKeyModelSelfCheckEnabled])
	require.Equal(t, "900", repo.values[service.SettingKeyModelSelfCheckDefaultIntervalSeconds])
	require.Equal(t, "12", repo.values[service.SettingKeyModelSelfCheckMaxConcurrency])
	require.Equal(t, "1200", repo.values[service.SettingKeyModelSelfCheckMaxTasksPerRound])
	require.Equal(t, "180", repo.values[service.SettingKeyModelSelfCheckSnapshotRetentionDays])
	require.Equal(t, "true", repo.values[service.SettingKeyDisableKeysOnRateChange])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, service.ScheduleStrategyCostFirst, data["schedule_strategy"])
	require.Equal(t, true, data["native_model_protocol_routing_enabled"])
	require.Equal(t, "settings", data["native_model_protocol_routing_source"])
	require.Equal(t, true, data["model_self_check_enabled"])
	require.Equal(t, float64(900), data["self_check_default_interval_seconds"])
	require.Equal(t, float64(12), data["self_check_max_concurrency"])
	require.Equal(t, float64(1200), data["self_check_max_tasks_per_round"])
	require.Equal(t, float64(180), data["model_self_check_status_snapshot_retention_days"])
	require.Equal(t, true, data["disable_keys_on_rate_change"])
}

func TestSettingHandler_UpdateSettings_OmittedNativeRoutingKeepsConfigFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{"promo_code_enabled": true})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	_, persisted := repo.lastUpdates[service.SettingKeyNativeModelProtocolRoutingEnabled]
	require.False(t, persisted)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, data["native_model_protocol_routing_enabled"])
	require.Equal(t, "config", data["native_model_protocol_routing_source"])
}

func TestSettingHandler_UpdateSettings_RejectsEnterpriseMemberEnforceBeforeReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled":                     true,
		"enterprise_member_model_admission_mode": string(service.EnterpriseMemberModelAdmissionEnforcePublished),
		"enterprise_member_model_admission_rollout_policy": map[string]any{
			"enterprise_user_ids": []int64{42},
			"member_ids":          []int64{},
			"percentage":          0,
			"salt":                "test",
			"auto_stop":           false,
		},
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusConflict, rec.Code)
	_, persisted := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionMode]
	require.False(t, persisted)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "ENFORCE_NOT_READY", resp.Reason)
	require.Equal(t, service.EnterpriseMemberModelAdmissionEnforceBlockedReason, resp.Metadata["blocked_reason"])
}

func TestSettingHandler_UpdateSettings_RejectsEnterpriseMemberEnforceWithManualAutoStopBeforeReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled":                     true,
		"enterprise_member_model_admission_mode": string(service.EnterpriseMemberModelAdmissionEnforcePublished),
		"enterprise_member_model_admission_rollout_policy": map[string]any{
			"enterprise_user_ids": []int64{42},
			"member_ids":          []int64{},
			"percentage":          0,
			"salt":                "test",
			"auto_stop":           true,
		},
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.Nil(t, repo.lastUpdates)
	_, persistedMode := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionMode]
	require.False(t, persistedMode)
	_, persistedPolicy := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionRolloutPolicy]
	require.False(t, persistedPolicy)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "ENFORCE_NOT_READY", resp.Reason)
	require.Equal(t, service.EnterpriseMemberModelAdmissionEnforceBlockedReason, resp.Metadata["blocked_reason"])
}

func TestSettingHandler_UpdateSettings_RejectsOversizedEnterpriseMemberAdmissionRolloutPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	enterpriseUserIDs := make([]int64, 1025)
	for i := range enterpriseUserIDs {
		enterpriseUserIDs[i] = int64(i + 1)
	}
	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled":                     true,
		"enterprise_member_model_admission_mode": string(service.EnterpriseMemberModelAdmissionShadowPublished),
		"enterprise_member_model_admission_rollout_policy": map[string]any{
			"enterprise_user_ids": enterpriseUserIDs,
			"member_ids":          []int64{},
			"percentage":          0,
			"salt":                "test",
			"auto_stop":           false,
		},
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, repo.lastUpdates)
	_, persistedPolicy := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionRolloutPolicy]
	require.False(t, persistedPolicy)
}

func TestSettingHandler_UpdateSettings_RejectsOversizedRequestBeforeJSONDecode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &SettingHandler{}
	body := `{"home_content":"` + strings.Repeat("x", int(updateSettingsRequestBodyMaxBytes)) + `"}`
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "REQUEST_BODY_TOO_LARGE", resp.Reason)
	require.Equal(t, "4194304", resp.Metadata["limit_bytes"])
}

func TestSettingHandler_UpdateSettings_RejectsEnterpriseMemberEnforceWithoutRolloutTargetBeforeReadiness(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{
			name: "missing rollout",
			body: map[string]any{
				"promo_code_enabled":                     true,
				"enterprise_member_model_admission_mode": string(service.EnterpriseMemberModelAdmissionEnforcePublished),
			},
		},
		{
			name: "empty rollout",
			body: map[string]any{
				"promo_code_enabled":                     true,
				"enterprise_member_model_admission_mode": string(service.EnterpriseMemberModelAdmissionEnforcePublished),
				"enterprise_member_model_admission_rollout_policy": map[string]any{
					"enterprise_user_ids": []int64{},
					"member_ids":          []int64{},
					"percentage":          0,
					"salt":                "test",
					"auto_stop":           false,
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &settingHandlerRepoStub{values: map[string]string{
				service.SettingKeyPromoCodeEnabled: "true",
			}}
			cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
			svc := service.NewSettingService(repo, cfg)
			handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

			rawBody, err := json.Marshal(tc.body)
			require.NoError(t, err)
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
			c.Request.Header.Set("Content-Type", "application/json")

			handler.UpdateSettings(c)

			require.Equal(t, http.StatusConflict, rec.Code)
			require.Nil(t, repo.lastUpdates)
			_, persisted := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionMode]
			require.False(t, persisted)

			var resp response.Response
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.Equal(t, "ENFORCE_NOT_READY", resp.Reason)
			require.Equal(t, service.EnterpriseMemberModelAdmissionEnforceBlockedReason, resp.Metadata["blocked_reason"])
		})
	}
}

func TestSettingHandler_UpdateSettings_RejectsInvalidEnterpriseMemberModelAdmissionMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled":                     true,
		"enterprise_member_model_admission_mode": "not-a-mode",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, repo.lastUpdates)
	_, persisted := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionMode]
	require.False(t, persisted)
}

func TestSettingHandler_UpdateSettings_OmittedEnterpriseMemberAdmissionKeepsConfigFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(service.EnterpriseMemberModelAdmissionLegacyOrderOnly)
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{"promo_code_enabled": true})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	_, persisted := repo.lastUpdates[service.SettingKeyEnterpriseMemberModelAdmissionMode]
	require.False(t, persisted)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, string(service.EnterpriseMemberModelAdmissionLegacyOrderOnly), data["enterprise_member_model_admission_mode"])
	require.Equal(t, "config", data["enterprise_member_model_admission_source"])
}

func TestSettingHandler_UpdateSettings_OmittedEnterpriseMemberAdmissionDoesNotPersistBlockedConfigEnforce(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(service.EnterpriseMemberModelAdmissionEnforcePublished)
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{"promo_code_enabled": true})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	_, persisted := repo.lastUpdates[service.SettingKeyEnterpriseMemberModelAdmissionMode]
	require.False(t, persisted)

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, string(service.EnterpriseMemberModelAdmissionShadowPublished), data["enterprise_member_model_admission_mode"])
	require.Equal(t, "rollout_shadow", data["enterprise_member_model_admission_source"])
}

func TestSettingHandler_UpdateSettings_PersistsEnterpriseMemberLegacyRetirementTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled": true,
		"enterprise_member_model_admission_legacy_retirement_target": "v1.8.0",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "v1.8.0", repo.values[service.SettingKeyEnterpriseMemberModelAdmissionLegacyRetirementTarget])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, "v1.8.0", data["enterprise_member_model_admission_legacy_retirement_target"])
	legacy, ok := data["enterprise_member_model_admission_legacy"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "scheduled", legacy["retirement_status"])
	require.Equal(t, false, legacy["phase5_ready"])
	require.Equal(t, service.EnterpriseMemberModelAdmissionPhase5GatePendingReason, legacy["phase5_reason"])
}

func TestSettingHandler_UpdateSettings_RejectsInvalidEnterpriseMemberLegacyRetirementTarget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{values: map[string]string{
		service.SettingKeyPromoCodeEnabled: "true",
	}}
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	svc := service.NewSettingService(repo, cfg)
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	rawBody, err := json.Marshal(map[string]any{
		"promo_code_enabled": true,
		"enterprise_member_model_admission_legacy_retirement_target": "later",
	})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	_, persisted := repo.values[service.SettingKeyEnterpriseMemberModelAdmissionLegacyRetirementTarget]
	require.False(t, persisted)
}

func TestDiffSettings_TracksNativeRoutingOverrideSourceChange(t *testing.T) {
	before := &service.SystemSettings{
		NativeModelProtocolRoutingEnabled: true,
		NativeModelProtocolRoutingSource:  "config",
	}
	after := &service.SystemSettings{
		NativeModelProtocolRoutingEnabled: true,
		NativeModelProtocolRoutingSource:  "settings",
	}

	changed := diffSettings(before, after, nil, nil, UpdateSettingsRequest{})

	require.Contains(t, changed, service.SettingKeyNativeModelProtocolRoutingEnabled)
}

func TestSettingHandler_UpdateSettings_InvalidOpenAIFastPolicyDoesNotPersistPartialSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const existingPolicy = `{"rules":[]}`
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:         "false",
			service.SettingKeyOpenAIFastPolicySettings: existingPolicy,
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled": true,
		"openai_fast_policy_settings": map[string]any{
			"rules": []map[string]any{{
				"service_tier": "priority",
				"action":       "pass",
				"scope":        "all",
				"user_ids":     []int64{0},
			}},
		},
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, repo.lastUpdates)
	require.Equal(t, "false", repo.values[service.SettingKeyPromoCodeEnabled])
	require.Equal(t, existingPolicy, repo.values[service.SettingKeyOpenAIFastPolicySettings])
}

func TestSettingHandler_UpdateSettings_PersistsOpenAIFastPolicyInSameBatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:         "false",
			service.SettingKeyOpenAIFastPolicySettings: `{"rules":[]}`,
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled": true,
		"openai_fast_policy_settings": map[string]any{
			"rules": []map[string]any{{
				"service_tier": "priority",
				"action":       "force_priority",
				"scope":        "all",
				"user_ids":     []int64{42},
			}},
		},
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "true", repo.lastUpdates[service.SettingKeyPromoCodeEnabled])
	require.JSONEq(t, `{"rules":[{"service_tier":"priority","action":"force_priority","scope":"all","user_ids":[42]}]}`, repo.lastUpdates[service.SettingKeyOpenAIFastPolicySettings])
}

func TestSettingsAuditChanges_IncludesOpenAIFastPolicy(t *testing.T) {
	settings := &service.SystemSettings{}
	changed := settingsAuditChanges(
		settings,
		settings,
		nil,
		nil,
		UpdateSettingsRequest{},
		service.SettingKeyOpenAIFastPolicySettings,
	)

	require.Contains(t, changed, service.SettingKeyOpenAIFastPolicySettings)
}

func TestEqualOpenAIFastPolicySettings_DetectsUserScopedChanges(t *testing.T) {
	before := &service.OpenAIFastPolicySettings{Rules: []service.OpenAIFastPolicyRule{{
		ServiceTier: "priority",
		Action:      "pass",
		Scope:       "all",
		UserIDs:     []int64{42},
	}}}
	after := &service.OpenAIFastPolicySettings{Rules: []service.OpenAIFastPolicyRule{{
		ServiceTier: "priority",
		Action:      "pass",
		Scope:       "all",
		UserIDs:     []int64{43},
	}}}

	require.False(t, equalOpenAIFastPolicySettings(before, after))
	require.True(t, equalOpenAIFastPolicySettings(before, before))
}

func TestSettingHandler_UpdateSettings_PersistsPaymentVisibleMethodsAndAdvancedScheduler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled":                                      true,
		"payment_visible_method_alipay_source":                    "easypay",
		"payment_visible_method_wxpay_source":                     "wxpay",
		"payment_visible_method_alipay_enabled":                   true,
		"payment_visible_method_wxpay_enabled":                    false,
		"openai_advanced_scheduler_enabled":                       true,
		"openai_oauth_scheduling_rate_multiplier":                 0.05,
		"openai_advanced_scheduler_subscription_priority_enabled": true,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, service.VisibleMethodSourceEasyPayAlipay, repo.values[service.SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, service.VisibleMethodSourceOfficialWechat, repo.values[service.SettingPaymentVisibleMethodWxpaySource])
	require.Equal(t, "true", repo.values[service.SettingPaymentVisibleMethodAlipayEnabled])
	require.Equal(t, "false", repo.values[service.SettingPaymentVisibleMethodWxpayEnabled])
	require.Equal(t, "true", repo.values["openai_advanced_scheduler_enabled"])
	require.Equal(t, "0.05", repo.values[service.SettingKeyOpenAIOAuthSchedulingRateMultiplier])
	require.Equal(t, "true", repo.values[service.SettingKeyOpenAIAdvancedSchedulerSubscriptionPriorityEnabled])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, service.VisibleMethodSourceEasyPayAlipay, data["payment_visible_method_alipay_source"])
	require.Equal(t, service.VisibleMethodSourceOfficialWechat, data["payment_visible_method_wxpay_source"])
	require.Equal(t, true, data["payment_visible_method_alipay_enabled"])
	require.Equal(t, false, data["payment_visible_method_wxpay_enabled"])
	require.Equal(t, true, data["openai_advanced_scheduler_enabled"])
	require.Equal(t, 0.05, data["openai_oauth_scheduling_rate_multiplier"])
	require.Equal(t, true, data["openai_advanced_scheduler_subscription_priority_enabled"])
}

func TestSettingHandler_UpdateSettings_PreservesLegacyBlankPaymentVisibleMethodSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:               "true",
			service.SettingPaymentVisibleMethodAlipayEnabled: "true",
			service.SettingPaymentVisibleMethodAlipaySource:  "",
			service.SettingPaymentVisibleMethodWxpayEnabled:  "false",
			service.SettingPaymentVisibleMethodWxpaySource:   "",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled": false,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "", repo.values[service.SettingPaymentVisibleMethodAlipaySource])
	require.Equal(t, "true", repo.values[service.SettingPaymentVisibleMethodAlipayEnabled])
}

func TestSettingHandler_UpdateSettings_PersistsExplicitFalseOIDCCompatibilityFlags(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:               "true",
			service.SettingKeyOIDCConnectEnabled:             "true",
			service.SettingKeyOIDCConnectProviderName:        "OIDC",
			service.SettingKeyOIDCConnectClientID:            "oidc-client",
			service.SettingKeyOIDCConnectClientSecret:        "oidc-secret",
			service.SettingKeyOIDCConnectIssuerURL:           "https://issuer.example.com",
			service.SettingKeyOIDCConnectAuthorizeURL:        "https://issuer.example.com/auth",
			service.SettingKeyOIDCConnectTokenURL:            "https://issuer.example.com/token",
			service.SettingKeyOIDCConnectUserInfoURL:         "https://issuer.example.com/userinfo",
			service.SettingKeyOIDCConnectJWKSURL:             "https://issuer.example.com/jwks",
			service.SettingKeyOIDCConnectScopes:              "openid email profile",
			service.SettingKeyOIDCConnectRedirectURL:         "https://example.com/api/v1/auth/oauth/oidc/callback",
			service.SettingKeyOIDCConnectFrontendRedirectURL: "/auth/oidc/callback",
			service.SettingKeyOIDCConnectTokenAuthMethod:     "client_secret_post",
			service.SettingKeyOIDCConnectUsePKCE:             "true",
			service.SettingKeyOIDCConnectValidateIDToken:     "true",
			service.SettingKeyOIDCConnectAllowedSigningAlgs:  "RS256",
			service.SettingKeyOIDCConnectClockSkewSeconds:    "120",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled":                true,
		"oidc_connect_enabled":              true,
		"oidc_connect_use_pkce":             false,
		"oidc_connect_validate_id_token":    false,
		"oidc_connect_allowed_signing_algs": "",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyOIDCConnectUsePKCE])
	require.Equal(t, "false", repo.values[service.SettingKeyOIDCConnectValidateIDToken])

	var resp response.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, data["oidc_connect_use_pkce"])
	require.Equal(t, false, data["oidc_connect_validate_id_token"])
}

func TestSettingHandler_UpdateSettings_DoesNotSolidifyImplicitOIDCSecurityDefaultsOnLegacyUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled:                "true",
			service.SettingKeyOIDCConnectEnabled:              "true",
			service.SettingKeyOIDCConnectProviderName:         "OIDC",
			service.SettingKeyOIDCConnectClientID:             "oidc-client",
			service.SettingKeyOIDCConnectClientSecret:         "oidc-secret",
			service.SettingKeyOIDCConnectIssuerURL:            "https://issuer.example.com",
			service.SettingKeyOIDCConnectAuthorizeURL:         "https://issuer.example.com/auth",
			service.SettingKeyOIDCConnectTokenURL:             "https://issuer.example.com/token",
			service.SettingKeyOIDCConnectUserInfoURL:          "https://issuer.example.com/userinfo",
			service.SettingKeyOIDCConnectJWKSURL:              "https://issuer.example.com/jwks",
			service.SettingKeyOIDCConnectScopes:               "openid email profile",
			service.SettingKeyOIDCConnectRedirectURL:          "https://example.com/api/v1/auth/oauth/oidc/callback",
			service.SettingKeyOIDCConnectFrontendRedirectURL:  "/auth/oidc/callback",
			service.SettingKeyOIDCConnectTokenAuthMethod:      "client_secret_post",
			service.SettingKeyOIDCConnectAllowedSigningAlgs:   "RS256",
			service.SettingKeyOIDCConnectClockSkewSeconds:     "120",
			service.SettingKeyOIDCConnectRequireEmailVerified: "false",
			service.SettingKeyOIDCConnectUserInfoEmailPath:    "",
			service.SettingKeyOIDCConnectUserInfoIDPath:       "",
			service.SettingKeyOIDCConnectUserInfoUsernamePath: "",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{
		Default: config.DefaultConfig{UserConcurrency: 5},
		OIDC: config.OIDCConnectConfig{
			Enabled:             true,
			ProviderName:        "OIDC",
			ClientID:            "oidc-client",
			ClientSecret:        "oidc-secret",
			IssuerURL:           "https://issuer.example.com",
			AuthorizeURL:        "https://issuer.example.com/auth",
			TokenURL:            "https://issuer.example.com/token",
			UserInfoURL:         "https://issuer.example.com/userinfo",
			JWKSURL:             "https://issuer.example.com/jwks",
			Scopes:              "openid email profile",
			RedirectURL:         "https://example.com/api/v1/auth/oauth/oidc/callback",
			FrontendRedirectURL: "/auth/oidc/callback",
			TokenAuthMethod:     "client_secret_post",
			UsePKCE:             true,
			ValidateIDToken:     true,
			AllowedSigningAlgs:  "RS256",
			ClockSkewSeconds:    120,
		},
	})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled":   true,
		"oidc_connect_enabled": true,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyOIDCConnectUsePKCE])
	require.Equal(t, "false", repo.values[service.SettingKeyOIDCConnectValidateIDToken])
}

func TestSettingHandler_UpdateSettings_RejectsInvalidPaymentVisibleMethodSource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerRepoStub{
		values: map[string]string{
			service.SettingKeyPromoCodeEnabled: "true",
		},
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"promo_code_enabled":                   true,
		"payment_visible_method_alipay_source": "bogus",
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.NotContains(t, repo.values, service.SettingPaymentVisibleMethodAlipaySource)
}

func TestSettingHandler_UpdateSettings_DoesNotPersistPartialSystemSettingsWhenAuthSourceDefaultsFail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &failingAuthSourceSettingsRepoStub{
		values: map[string]string{
			service.SettingKeyRegistrationEnabled:                 "false",
			service.SettingKeyPromoCodeEnabled:                    "true",
			service.SettingKeyAuthSourceDefaultEmailBalance:       "9.5",
			service.SettingKeyAuthSourceDefaultEmailConcurrency:   "8",
			service.SettingKeyAuthSourceDefaultEmailSubscriptions: `[{"group_id":31,"validity_days":15}]`,
		},
		err: errors.New("write auth source defaults failed"),
	}
	svc := service.NewSettingService(repo, &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}})
	handler := NewSettingHandler(svc, nil, nil, nil, nil, nil, nil)

	body := map[string]any{
		"registration_enabled":              true,
		"promo_code_enabled":                true,
		"auth_source_default_email_balance": 12.75,
	}
	rawBody, err := json.Marshal(body)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/v1/admin/settings", bytes.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.UpdateSettings(c)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "false", repo.values[service.SettingKeyRegistrationEnabled])
	require.Equal(t, "9.5", repo.values[service.SettingKeyAuthSourceDefaultEmailBalance])
}

func TestDiffSettings_IncludesAuthSourceDefaultsAndForceEmail(t *testing.T) {
	changed := diffSettings(
		&service.SystemSettings{},
		&service.SystemSettings{},
		&service.AuthSourceDefaultSettings{
			Email: service.ProviderDefaultGrantSettings{
				Balance:          0,
				Concurrency:      5,
				Subscriptions:    nil,
				GrantOnSignup:    true,
				GrantOnFirstBind: false,
			},
			ForceEmailOnThirdPartySignup: false,
		},
		&service.AuthSourceDefaultSettings{
			Email: service.ProviderDefaultGrantSettings{
				Balance:          12.5,
				Concurrency:      7,
				Subscriptions:    []service.DefaultSubscriptionSetting{{GroupID: 21, ValidityDays: 30}},
				GrantOnSignup:    false,
				GrantOnFirstBind: true,
			},
			ForceEmailOnThirdPartySignup: true,
		},
		UpdateSettingsRequest{},
	)

	require.Contains(t, changed, "auth_source_default_email_balance")
	require.Contains(t, changed, "auth_source_default_email_concurrency")
	require.Contains(t, changed, "auth_source_default_email_subscriptions")
	require.Contains(t, changed, "auth_source_default_email_grant_on_signup")
	require.Contains(t, changed, "auth_source_default_email_grant_on_first_bind")
	require.Contains(t, changed, "force_email_on_third_party_signup")
}
