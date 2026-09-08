//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func configuredCatalogChannelsForTest() []service.AvailableChannel {
	price := 0.000002
	channel := &service.Channel{
		ModelPricing: []service.ChannelModelPricing{{
			Platform:    service.PlatformOpenAI,
			Models:      []string{"deepseek-v4-flash", "deepseek-v4-pro"},
			BillingMode: service.BillingModeToken,
			InputPrice:  &price,
		}},
	}
	return []service.AvailableChannel{{
		Name: "deepseek", Status: service.StatusActive,
		Groups: []service.AvailableGroupRef{
			{ID: 7, Name: "visible", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard},
			{ID: 9, Name: "also-visible", Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard},
			{ID: 11, Name: "private", Platform: service.PlatformOpenAI, IsExclusive: true},
		},
		SupportedModels: channel.SupportedModels(),
	}}
}

func configuredCatalogVisibleGroupsForTest(groups []service.AvailableGroupRef) []userAvailableGroup {
	return filterUserVisibleGroups(groups, map[int64]struct{}{7: {}, 9: {}})
}

func TestConfiguredChannelCatalog_KeepsPricedModelsIndependentOfAccounts(t *testing.T) {
	tests := []struct {
		name         string
		account      *service.Account
		capabilities []service.AccountModelProtocolCapability
	}{
		{name: "no accounts"},
		{name: "account disabled", account: &service.Account{Status: service.StatusDisabled, Schedulable: true}},
		{name: "scheduling disabled", account: &service.Account{Status: service.StatusActive}},
		{name: "model unsupported", account: &service.Account{Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{
			"model_mapping": map[string]any{"other-model": "other-model"},
		}}},
		{name: "protocol evidence unknown", account: &service.Account{Status: service.StatusActive, Schedulable: true}},
		{name: "protocols explicitly unsupported", account: &service.Account{Status: service.StatusActive, Schedulable: true}, capabilities: []service.AccountModelProtocolCapability{
			{UpstreamModel: "*", Protocol: service.ModelProtocolOpenAIChat, OverrideState: service.ModelProtocolStateUnsupported},
			{UpstreamModel: "*", Protocol: service.ModelProtocolOpenAIResponses, OverrideState: service.ModelProtocolStateUnsupported},
			{UpstreamModel: "*", Protocol: service.ModelProtocolAnthropicMessages, OverrideState: service.ModelProtocolStateUnsupported},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := &availableDeliveryGroupRepoStub{groups: []service.Group{
				{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive},
				{ID: 9, Platform: service.PlatformOpenAI, Status: service.StatusActive},
			}}
			accounts := &availableDeliveryAccountRepoStub{}
			if tt.account != nil {
				tt.account.ID, tt.account.Platform, tt.account.Type = 70, service.PlatformOpenAI, service.AccountTypeAPIKey
				tt.account.GroupIDs = []int64{7}
				accounts.accounts = []*service.Account{tt.account}
				groups.accountIDs = []int64{70}
			}
			cfg := &config.Config{}
			cfg.Gateway.NativeModelProtocolRoutingEnabled = true
			capability := service.NewModelProtocolCapabilityService(&availableDeliveryCapabilityRepoStub{
				itemsByAccount: map[int64][]service.AccountModelProtocolCapability{70: tt.capabilities},
			}, accounts, groups, nil, cfg)
			delivery := service.NewModelDeliveryService(accounts, groups, nil, capability, cfg)

			out, err := buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), delivery,
				configuredCatalogVisibleGroupsForTest, catalogModelsConfigured)
			require.NoError(t, err)
			require.Len(t, out, 1)
			require.Len(t, out[0].Platforms, 1)
			section := out[0].Platforms[0]
			require.Len(t, section.Groups, 2)
			require.Len(t, section.SupportedModels, 2)
			for _, model := range section.SupportedModels {
				require.Equal(t, []int64{7, 9}, model.CatalogGroupIDs)
				require.Empty(t, model.RouteGroupIDs)
				require.Empty(t, model.SupportedEndpoints)
				require.NotNil(t, model.Pricing)
				require.InDelta(t, 0.000002, *model.Pricing.InputPrice, 1e-12)
			}
			raw, err := json.Marshal(out)
			require.NoError(t, err)
			require.Contains(t, string(raw), `"catalog_group_ids":[7,9]`)
			require.NotContains(t, string(raw), "private")
			require.NotContains(t, string(raw), "account_id")

			public, err := buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), delivery,
				filterPublicStandardGroups, catalogModelsCallable)
			require.NoError(t, err)
			require.Empty(t, public, "public discovery keeps its existing callable-model policy")
		})
	}
}

func TestConfiguredChannelCatalog_EndpointEvidenceDoesNotRestrictPublication(t *testing.T) {
	groups := &availableDeliveryGroupRepoStub{groups: []service.Group{
		{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive},
		{ID: 9, Platform: service.PlatformOpenAI, Status: service.StatusActive},
	}, accountIDs: []int64{70}}
	resetAt := time.Now().Add(time.Hour)
	accounts := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
		ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{7}, RateLimitResetAt: &resetAt,
	}}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := service.NewModelProtocolCapabilityService(&availableDeliveryCapabilityRepoStub{
		itemsByAccount: map[int64][]service.AccountModelProtocolCapability{
			70: exactProtocolCapabilities("deepseek-v4-flash", service.ModelProtocolOpenAIChat),
		},
	}, accounts, groups, nil, cfg)
	delivery := service.NewModelDeliveryService(accounts, groups, nil, capability, cfg)

	out, err := buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), delivery,
		configuredCatalogVisibleGroupsForTest, catalogModelsConfigured)
	require.NoError(t, err)
	models := out[0].Platforms[0].SupportedModels
	require.Len(t, models, 2)
	require.Equal(t, []int64{7, 9}, models[0].CatalogGroupIDs)
	require.Equal(t, []int64{7}, models[0].RouteGroupIDs)
	require.Equal(t, []userSupportedEndpoint{{Protocol: string(service.ModelProtocolOpenAIChat), Path: "/v1/chat/completions", GroupIDs: []int64{7}}}, models[0].SupportedEndpoints)
	require.Equal(t, []int64{7, 9}, models[1].CatalogGroupIDs)
	require.Empty(t, models[1].RouteGroupIDs)
	require.Empty(t, models[1].SupportedEndpoints)
}

type unavailableCatalogAccountRepo struct {
	service.AccountRepository
}

func (unavailableCatalogAccountRepo) GetByIDs(context.Context, []int64) ([]*service.Account, error) {
	return nil, errors.New("account metadata unavailable")
}

func TestConfiguredChannelCatalog_MetadataFailureKeepsPublication(t *testing.T) {
	groups := &availableDeliveryGroupRepoStub{groups: []service.Group{{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive}}}
	delivery := service.NewModelDeliveryService(unavailableCatalogAccountRepo{}, groups, nil, nil, &config.Config{})
	out, err := buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), delivery,
		configuredCatalogVisibleGroupsForTest, catalogModelsConfigured)
	require.NoError(t, err)
	require.Len(t, out, 1)
	for _, model := range out[0].Platforms[0].SupportedModels {
		require.Equal(t, []int64{7, 9}, model.CatalogGroupIDs)
		require.Empty(t, model.SupportedEndpoints)
	}
	_, err = buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), delivery,
		filterPublicStandardGroups, catalogModelsCallable)
	require.ErrorContains(t, err, "account metadata unavailable")
}
