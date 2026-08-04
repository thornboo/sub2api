//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFilterPublicStandardGroups_ExcludesExclusiveAndSubscriptionGroups(t *testing.T) {
	groups := []service.AvailableGroupRef{
		{ID: 1, Name: "public-standard", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard},
		{ID: 2, Name: "exclusive-standard", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard, IsExclusive: true},
		{ID: 3, Name: "public-subscription", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription},
		{ID: 4, Name: "legacy-empty-type", Platform: service.PlatformOpenAI},
	}

	visible := filterPublicStandardGroups(groups)

	require.Len(t, visible, 1)
	require.Equal(t, int64(1), visible[0].ID)
	require.False(t, visible[0].IsExclusive)
	require.Equal(t, service.SubscriptionTypeStandard, visible[0].SubscriptionType)
}

func TestBuildAvailableChannelCatalog_PublicUsesSharedDeliveryProjection(t *testing.T) {
	inputPrice := 0.000001
	imagePrice1K := 0.02
	channels := []service.AvailableChannel{
		{
			Name:   "deepseek",
			Status: service.StatusActive,
			Groups: []service.AvailableGroupRef{
				{
					ID: 1, Name: "public", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard,
					ImageRateIndependent: true, ImageRateMultiplier: 0.5, ImagePrice1K: &imagePrice1K,
				},
				{ID: 2, Name: "exclusive", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard, IsExclusive: true},
				{ID: 3, Name: "subscription", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeSubscription},
			},
			SupportedModels: []service.SupportedModel{{
				Name:     "deepseek-v4-flash",
				Platform: service.PlatformOpenAI,
				Pricing: &service.ChannelModelPricing{
					BillingMode: service.BillingModeToken,
					InputPrice:  &inputPrice,
				},
			}},
		},
		{
			Name:   "disabled-channel",
			Status: service.StatusDisabled,
			Groups: []service.AvailableGroupRef{{
				ID: 1, Name: "public", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard,
			}},
			SupportedModels: []service.SupportedModel{{Name: "hidden", Platform: service.PlatformOpenAI}},
		},
	}
	groupRepo := &availableDeliveryGroupRepoStub{
		groups: []service.Group{{
			ID: 1, Name: "public", Platform: service.PlatformOpenAI,
			Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		}},
		accountIDs: []int64{70},
	}
	accountRepo := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
		ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{1},
	}}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := service.NewModelProtocolCapabilityService(
		&availableDeliveryCapabilityRepoStub{itemsByAccount: map[int64][]service.AccountModelProtocolCapability{
			70: exactProtocolCapabilities("deepseek-v4-flash", service.ModelProtocolOpenAIChat),
		}},
		accountRepo,
		groupRepo,
		nil,
		cfg,
	)
	delivery := service.NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	out, err := buildAvailableChannelCatalog(
		context.Background(),
		channels,
		delivery,
		filterPublicStandardGroups,
	)

	require.NoError(t, err)
	require.Len(t, out, 1)
	require.Equal(t, "deepseek", out[0].Name)
	require.Len(t, out[0].Platforms, 1)
	section := out[0].Platforms[0]
	require.Len(t, section.Groups, 1)
	require.Equal(t, int64(1), section.Groups[0].ID)
	require.True(t, section.Groups[0].ImageRateIndependent)
	require.InDelta(t, 0.5, section.Groups[0].ImageRateMultiplier, 1e-12)
	require.NotNil(t, section.Groups[0].ImagePrice1K)
	require.InDelta(t, 0.02, *section.Groups[0].ImagePrice1K, 1e-12)
	require.Len(t, section.SupportedModels, 1)
	require.Equal(t, []int64{1}, section.SupportedModels[0].RouteGroupIDs)
	require.Equal(t, []userSupportedEndpoint{{
		Protocol: string(service.ModelProtocolOpenAIChat),
		Path:     "/v1/chat/completions",
		GroupIDs: []int64{1},
	}}, section.SupportedModels[0].SupportedEndpoints)
}

func TestModelPlazaResponse_UsesCustomerSafeChannelDTO(t *testing.T) {
	payload := modelPlazaResponse{
		Description: "public catalog",
		Channels: []userAvailableChannel{{
			Name:        "deepseek",
			Description: "public",
			Platforms: []userChannelPlatformSection{{
				Platform: service.PlatformOpenAI,
				Groups: []userAvailableGroup{{
					ID: 1, Name: "public", Platform: service.PlatformOpenAI,
					SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.8,
				}},
				SupportedModels: []userSupportedModel{{
					Name: "deepseek-v4-flash", Platform: service.PlatformOpenAI,
					RouteGroupIDs: []int64{1},
					SupportedEndpoints: []userSupportedEndpoint{{
						Protocol: string(service.ModelProtocolOpenAIChat),
						Path:     "/v1/chat/completions",
						GroupIDs: []int64{1},
					}},
				}},
			}},
		}},
	}

	raw, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Contains(t, decoded, "description")
	require.Contains(t, decoded, "channels")
	require.NotContains(t, decoded, "groups")
	serialized := string(raw)
	for _, forbidden := range []string{
		"user_rate_multiplier",
		"official_pricing",
		"account_id",
		"account_name",
		"base_url",
		"billing_model_source",
	} {
		require.NotContains(t, serialized, forbidden)
	}
	require.Contains(t, serialized, `"route_group_ids":[1]`)
	require.Contains(t, serialized, `"path":"/v1/chat/completions"`)
}

type modelPlazaChannelRepoStub struct {
	service.ChannelRepository
	channels []service.Channel
}

func (r *modelPlazaChannelRepoStub) ListAll(_ context.Context) ([]service.Channel, error) {
	return append([]service.Channel(nil), r.channels...), nil
}

func (r *modelPlazaChannelRepoStub) GetGroupPlatforms(
	_ context.Context,
	groupIDs []int64,
) (map[int64]string, error) {
	platforms := make(map[int64]string, len(groupIDs))
	for _, groupID := range groupIDs {
		platforms[groupID] = service.PlatformOpenAI
	}
	return platforms, nil
}

type modelPlazaSettingRepoStub struct {
	service.SettingRepository
	values map[string]string
}

func (r *modelPlazaSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[key] = r.values[key]
	}
	return values, nil
}

func TestModelPlazaHandler_AnonymousAndAuthenticatedResponsesAreIdentical(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupRepo := &availableDeliveryGroupRepoStub{
		groups: []service.Group{
			{
				ID: 1, Name: "public", Description: "public group", Platform: service.PlatformOpenAI,
				Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
				RateMultiplier: 0.8,
			},
			{
				ID: 2, Name: "exclusive", Platform: service.PlatformOpenAI,
				Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
				IsExclusive: true,
			},
			{
				ID: 3, Name: "subscription", Platform: service.PlatformOpenAI,
				Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeSubscription,
			},
		},
		accountIDs: []int64{70},
	}
	channelService := service.NewChannelService(
		&modelPlazaChannelRepoStub{channels: []service.Channel{{
			ID: 10, Name: "deepseek", Description: "public route", Status: service.StatusActive,
			GroupIDs: []int64{1, 2, 3},
			ModelMapping: map[string]map[string]string{
				service.PlatformOpenAI: {"deepseek-v4-flash": "deepseek-v4-flash"},
			},
		}}},
		groupRepo,
		nil,
		nil,
	)
	accountRepo := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
		ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{1},
	}}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := service.NewModelProtocolCapabilityService(
		&availableDeliveryCapabilityRepoStub{itemsByAccount: map[int64][]service.AccountModelProtocolCapability{
			70: exactProtocolCapabilities("deepseek-v4-flash", service.ModelProtocolOpenAIChat),
		}},
		accountRepo,
		groupRepo,
		nil,
		cfg,
	)
	delivery := service.NewModelDeliveryService(accountRepo, groupRepo, channelService, capability, cfg)
	settingService := service.NewSettingService(&modelPlazaSettingRepoStub{values: map[string]string{
		service.SettingKeyModelPlazaEnabled:     "true",
		service.SettingKeyModelPlazaRequireAuth: "false",
		service.SettingKeyModelPlazaDescription: "public catalog",
	}}, cfg)
	h := NewModelPlazaHandler(channelService, settingService, delivery)

	invoke := func(subject *middleware.AuthSubject) []byte {
		t.Helper()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)
		if subject != nil {
			c.Set(string(middleware.ContextKeyUser), *subject)
		}
		h.Get(c)
		require.Equal(t, http.StatusOK, w.Code)
		return w.Body.Bytes()
	}

	anonymous := invoke(nil)
	authenticated := invoke(&middleware.AuthSubject{UserID: 999})

	require.JSONEq(t, string(anonymous), string(authenticated))
	require.Contains(t, string(anonymous), `"description":"public group"`)
	require.Contains(t, string(anonymous), `"rate_multiplier":0.8`)
	require.Contains(t, string(anonymous), `"path":"/v1/chat/completions"`)
	require.NotContains(t, string(anonymous), `"exclusive"`)
	require.NotContains(t, string(anonymous), `"subscription"`)
	require.NotContains(t, string(anonymous), `"user_rate_multiplier"`)
}

func TestModelPlazaHandler_NilSettingServiceFailsClosed404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &ModelPlazaHandler{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/model-plaza", nil)

	h.Get(c)

	require.Equal(t, http.StatusNotFound, w.Code)
}
