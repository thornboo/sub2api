//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserAvailableChannel_Unauthenticated401(t *testing.T) {
	// 没有 AuthSubject 注入时，handler 应返回 401 且不触达 service 依赖。
	gin.SetMode(gin.TestMode)
	h := &AvailableChannelHandler{} // nil services — 401 路径不会调用它们
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/channels/available", nil)

	h.List(c)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFilterUserVisibleGroups_IntersectionOnly(t *testing.T) {
	// 渠道挂在 {g1, g2, g3}，用户只允许 {g1, g3} —— 响应必须仅含 g1/g3。
	groups := []service.AvailableGroupRef{
		{ID: 1, Name: "g1", Platform: "anthropic", AllowMessagesDispatch: true},
		{ID: 2, Name: "g2", Platform: "anthropic"},
		{ID: 3, Name: "g3", Platform: "openai"},
	}
	allowed := map[int64]struct{}{1: {}, 3: {}}

	visible := filterUserVisibleGroups(groups, allowed)
	require.Len(t, visible, 2)
	ids := []int64{visible[0].ID, visible[1].ID}
	require.ElementsMatch(t, []int64{1, 3}, ids)
	require.True(t, visible[0].AllowMessagesDispatch)
}

func TestToUserSupportedModels_FiltersByAllowedPlatforms(t *testing.T) {
	// 用户可访问分组只覆盖 anthropic；anthropic 平台的模型保留，openai 模型被剔除。
	src := []service.SupportedModel{
		{Name: "claude-sonnet-4-6", Platform: "anthropic", Pricing: nil},
		{Name: "gpt-4o", Platform: "openai", Pricing: nil},
	}
	allowed := map[string]struct{}{"anthropic": {}}
	out := toUserSupportedModels(src, allowed)
	require.Len(t, out, 1)
	require.Equal(t, "claude-sonnet-4-6", out[0].Name)
}

func TestToUserSupportedModels_NilAllowedPlatformsKeepsAll(t *testing.T) {
	// 显式传 nil allowedPlatforms 表示不做过滤。
	src := []service.SupportedModel{
		{Name: "a", Platform: "anthropic"},
		{Name: "b", Platform: "openai"},
	}
	require.Len(t, toUserSupportedModels(src, nil), 2)
}

func TestUserAvailableChannel_FieldWhitelist(t *testing.T) {
	// 通过序列化 userAvailableChannel 结构体验证响应形状：
	// 只有 name / description / platforms；不含管理端字段。
	row := userAvailableChannel{
		Name:        "ch",
		Description: "d",
		Platforms: []userChannelPlatformSection{
			{
				Platform:        "anthropic",
				Groups:          []userAvailableGroup{{ID: 1, Name: "g1", Platform: "anthropic"}},
				SupportedModels: []userSupportedModel{},
			},
		},
	}
	raw, err := json.Marshal(row)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(raw, &decoded))

	for _, key := range []string{"id", "status", "billing_model_source", "restrict_models"} {
		_, exists := decoded[key]
		require.Falsef(t, exists, "user DTO must not expose %q", key)
	}
	for _, key := range []string{"name", "description", "platforms"} {
		_, exists := decoded[key]
		require.Truef(t, exists, "user DTO must expose %q", key)
	}

	// 验证 section 的字段（platform / groups / supported_models）。
	rawSection, err := json.Marshal(row.Platforms[0])
	require.NoError(t, err)
	var sectionDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawSection, &sectionDecoded))
	for _, key := range []string{"platform", "groups", "supported_models"} {
		_, exists := sectionDecoded[key]
		require.Truef(t, exists, "platform section must expose %q", key)
	}

	// Group DTO 暴露区分专属/公开、订阅类型、默认倍率和高峰倍率规则所需的字段，
	// 前端据此渲染 GroupBadge 并与 API 密钥页保持一致的视觉。
	rawGroup, err := json.Marshal(row.Platforms[0].Groups[0])
	require.NoError(t, err)
	var groupDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawGroup, &groupDecoded))
	for _, key := range []string{"id", "name", "platform", "subscription_type", "rate_multiplier", "peak_rate_enabled", "peak_start", "peak_end", "peak_rate_multiplier", "is_exclusive"} {
		_, exists := groupDecoded[key]
		require.Truef(t, exists, "group DTO must expose %q", key)
	}
	_, exposesDispatchPolicy := groupDecoded["allow_messages_dispatch"]
	require.False(t, exposesDispatchPolicy, "group DTO must not expose internal endpoint policy fields")

	// pricing interval 白名单：不应暴露 id / sort_order。
	pricing := toUserPricing(&service.ChannelModelPricing{
		BillingMode: service.BillingModeToken,
		Intervals: []service.PricingInterval{
			{ID: 7, MinTokens: 0, MaxTokens: nil, SortOrder: 3},
		},
	})
	require.NotNil(t, pricing)
	require.Len(t, pricing.Intervals, 1)
	rawIv, err := json.Marshal(pricing.Intervals[0])
	require.NoError(t, err)
	var ivDecoded map[string]any
	require.NoError(t, json.Unmarshal(rawIv, &ivDecoded))
	for _, key := range []string{"id", "pricing_id", "sort_order"} {
		_, exists := ivDecoded[key]
		require.Falsef(t, exists, "user pricing interval must not expose %q", key)
	}
}

func TestBuildPlatformSections_GroupsByPlatform(t *testing.T) {
	// 一个渠道横跨 anthropic / openai / 空平台：应该生成 2 个 section，
	// 按 platform 字母序排序，各自 groups 和 supported_models 只含同平台条目。
	ch := service.AvailableChannel{
		Name: "ch",
		SupportedModels: []service.SupportedModel{
			{Name: "claude-sonnet-4-6", Platform: "anthropic"},
			{Name: "gpt-4o", Platform: "openai"},
		},
	}
	visible := []userAvailableGroup{
		{ID: 1, Name: "g-openai", Platform: "openai"},
		{ID: 2, Name: "g-ant", Platform: "anthropic"},
		{ID: 3, Name: "g-empty", Platform: ""},
	}
	sections := buildPlatformSections(ch, visible)
	require.Len(t, sections, 2)
	require.Equal(t, "anthropic", sections[0].Platform)
	require.Equal(t, "openai", sections[1].Platform)
	require.Len(t, sections[0].Groups, 1)
	require.Equal(t, int64(2), sections[0].Groups[0].ID)
	require.Len(t, sections[0].SupportedModels, 1)
	require.Equal(t, "claude-sonnet-4-6", sections[0].SupportedModels[0].Name)
}

type availableDeliveryAccountRepoStub struct {
	service.AccountRepository
	accounts []*service.Account
}

func (r *availableDeliveryAccountRepoStub) GetByIDs(_ context.Context, _ []int64) ([]*service.Account, error) {
	return r.accounts, nil
}

type availableDeliveryGroupRepoStub struct {
	service.GroupRepository
	groups     []service.Group
	accountIDs []int64
}

type availableDeliveryCapabilityRepoStub struct {
	itemsByAccount map[int64][]service.AccountModelProtocolCapability
}

func (r *availableDeliveryCapabilityRepoStub) ListByAccount(_ context.Context, accountID int64) ([]service.AccountModelProtocolCapability, error) {
	return append([]service.AccountModelProtocolCapability(nil), r.itemsByAccount[accountID]...), nil
}

func (r *availableDeliveryCapabilityRepoStub) ListByAccountIDs(_ context.Context, accountIDs []int64) (map[int64][]service.AccountModelProtocolCapability, error) {
	result := make(map[int64][]service.AccountModelProtocolCapability, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = append([]service.AccountModelProtocolCapability(nil), r.itemsByAccount[accountID]...)
	}
	return result, nil
}

func (r *availableDeliveryCapabilityRepoStub) SyncObserved(_ context.Context, _ int64, _ []service.ModelProtocolObservation) error {
	return nil
}

func (r *availableDeliveryCapabilityRepoStub) UpdateOverrides(_ context.Context, _ int64, _ []service.ModelProtocolOverride) error {
	return nil
}

func (r *availableDeliveryGroupRepoStub) ListActive(_ context.Context) ([]service.Group, error) {
	return r.groups, nil
}

func (r *availableDeliveryGroupRepoStub) GetAccountIDsByGroupIDs(_ context.Context, _ []int64) ([]int64, error) {
	return r.accountIDs, nil
}

func TestAttachSupportedEndpoints_PreservesMessagesOnlyForStableRoutes(t *testing.T) {
	channels := []userAvailableChannel{
		{
			Name: "legacy-contract",
			Platforms: []userChannelPlatformSection{
				{
					Platform: service.PlatformAnthropic,
					Groups: []userAvailableGroup{
						{ID: 3, Platform: service.PlatformAnthropic},
					},
					SupportedModels: []userSupportedModel{{Name: "claude-fable-5", Platform: service.PlatformAnthropic}},
				},
				{
					Platform: service.PlatformOpenAI,
					Groups: []userAvailableGroup{
						{ID: 7, Platform: service.PlatformOpenAI, AllowMessagesDispatch: true},
						{ID: 8, Platform: service.PlatformOpenAI, AllowMessagesDispatch: false},
					},
					SupportedModels: []userSupportedModel{{Name: "MiniMax-M3", Platform: service.PlatformOpenAI}},
				},
			},
		},
	}
	groupRepo := &availableDeliveryGroupRepoStub{
		groups: []service.Group{
			{ID: 3, Name: "Anthropic", Platform: service.PlatformAnthropic, Status: service.StatusActive},
			{ID: 7, Name: "OpenAI enabled", Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: true},
			{ID: 8, Name: "OpenAI disabled", Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: false},
		},
		accountIDs: []int64{30, 70},
	}
	accountRepo := &availableDeliveryAccountRepoStub{accounts: []*service.Account{
		{ID: 30, Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{3}},
		{ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{7, 8}},
	}}
	delivery := service.NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

	err := (&AvailableChannelHandler{modelDelivery: delivery}).attachSupportedEndpoints(context.Background(), channels)
	require.NoError(t, err)

	category := channels[0].Platforms
	require.Equal(t, []userSupportedEndpoint{{
		Protocol: string(service.ModelProtocolAnthropicMessages),
		Path:     "/v1/messages",
		GroupIDs: []int64{3},
	}}, category[0].SupportedModels[0].SupportedEndpoints)
	require.Equal(t, []int64{3}, category[0].SupportedModels[0].RouteGroupIDs)
	require.Equal(t, []userSupportedEndpoint{{
		Protocol: string(service.ModelProtocolAnthropicMessages),
		Path:     "/v1/messages",
		GroupIDs: []int64{7},
	}}, category[1].SupportedModels[0].SupportedEndpoints)
	require.Equal(t, []int64{7, 8}, category[1].SupportedModels[0].RouteGroupIDs)
}

func TestAttachSupportedEndpoints_RemovesPricingOnlyModelWithoutStableRoute(t *testing.T) {
	channels := []userAvailableChannel{{
		Name: "pricing-only",
		Platforms: []userChannelPlatformSection{{
			Platform:        service.PlatformOpenAI,
			Groups:          []userAvailableGroup{{ID: 7, Platform: service.PlatformOpenAI, AllowMessagesDispatch: true}},
			SupportedModels: []userSupportedModel{{Name: "orphan-model", Platform: service.PlatformOpenAI}},
		}},
	}}
	groupRepo := &availableDeliveryGroupRepoStub{
		groups: []service.Group{{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: true}},
	}
	delivery := service.NewModelDeliveryService(&availableDeliveryAccountRepoStub{}, groupRepo, nil, nil, &config.Config{})

	require.NoError(t, (&AvailableChannelHandler{modelDelivery: delivery}).attachSupportedEndpoints(context.Background(), channels))
	require.Empty(t, channels[0].Platforms[0].SupportedModels)
	require.Empty(t, pruneUndeliverableChannels(channels))
}

func TestAttachSupportedEndpoints_KeepsStableLegacyRouteWhenNativeRoutingDisabled(t *testing.T) {
	channels := []userAvailableChannel{{
		Name: "route-without-endpoint",
		Platforms: []userChannelPlatformSection{{
			Platform:        service.PlatformOpenAI,
			Groups:          []userAvailableGroup{{ID: 7, Platform: service.PlatformOpenAI}},
			SupportedModels: []userSupportedModel{{Name: "MiniMax-M3", Platform: service.PlatformOpenAI}},
		}},
	}}
	groupRepo := &availableDeliveryGroupRepoStub{
		groups:     []service.Group{{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive}},
		accountIDs: []int64{70},
	}
	accountRepo := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
		ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{7},
	}}}
	delivery := service.NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

	require.NoError(t, (&AvailableChannelHandler{modelDelivery: delivery}).attachSupportedEndpoints(context.Background(), channels))
	require.Len(t, channels[0].Platforms[0].SupportedModels, 1)
	require.Equal(t, []int64{7}, channels[0].Platforms[0].SupportedModels[0].RouteGroupIDs)
	require.Nil(t, channels[0].Platforms[0].SupportedModels[0].SupportedEndpoints)
}

func TestAttachSupportedEndpoints_KeepsUnknownLegacyRouteWithoutPublishingEndpoint(t *testing.T) {
	channels := []userAvailableChannel{{
		Name: "unknown-capability",
		Platforms: []userChannelPlatformSection{{
			Platform:        service.PlatformOpenAI,
			Groups:          []userAvailableGroup{{ID: 7, Platform: service.PlatformOpenAI}},
			SupportedModels: []userSupportedModel{{Name: "MiniMax-M3", Platform: service.PlatformOpenAI}},
		}},
	}}
	groupRepo := &availableDeliveryGroupRepoStub{
		groups:     []service.Group{{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive}},
		accountIDs: []int64{70},
	}
	accountRepo := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
		ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{7},
	}}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := service.NewModelProtocolCapabilityService(
		&availableDeliveryCapabilityRepoStub{}, accountRepo, groupRepo, nil, cfg,
	)
	delivery := service.NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	require.NoError(t, (&AvailableChannelHandler{modelDelivery: delivery}).attachSupportedEndpoints(context.Background(), channels))
	require.Len(t, channels[0].Platforms[0].SupportedModels, 1)
	model := channels[0].Platforms[0].SupportedModels[0]
	require.Equal(t, []int64{7}, model.RouteGroupIDs)
	require.Empty(t, model.SupportedEndpoints, "unknown evidence must not be advertised as a proven endpoint")
}

func TestAttachSupportedEndpoints_RemovesRouteWhenAllProtocolsExplicitlyUnsupported(t *testing.T) {
	channels := []userAvailableChannel{{
		Name: "explicitly-unsupported",
		Platforms: []userChannelPlatformSection{{
			Platform: service.PlatformOpenAI,
			Groups: []userAvailableGroup{{
				ID: 7, Platform: service.PlatformOpenAI, AllowMessagesDispatch: true,
			}},
			SupportedModels: []userSupportedModel{{Name: "MiniMax-M3", Platform: service.PlatformOpenAI}},
		}},
	}}
	groupRepo := &availableDeliveryGroupRepoStub{
		groups: []service.Group{{
			ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: true,
		}},
		accountIDs: []int64{70},
	}
	accountRepo := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
		ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{7},
	}}}
	items := make([]service.AccountModelProtocolCapability, 0, len(service.AllModelProtocols))
	for _, protocol := range service.AllModelProtocols {
		items = append(items, service.AccountModelProtocolCapability{
			UpstreamModel: "MiniMax-M3",
			Protocol:      protocol,
			OverrideState: service.ModelProtocolStateUnsupported,
		})
	}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := service.NewModelProtocolCapabilityService(
		&availableDeliveryCapabilityRepoStub{itemsByAccount: map[int64][]service.AccountModelProtocolCapability{70: items}},
		accountRepo, groupRepo, nil, cfg,
	)
	delivery := service.NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	require.NoError(t, (&AvailableChannelHandler{modelDelivery: delivery}).attachSupportedEndpoints(context.Background(), channels))
	require.Empty(t, channels[0].Platforms[0].SupportedModels)
}

func TestUpsertUserSupportedEndpoint_MergesDefaultAndConfirmedGroups(t *testing.T) {
	model := userSupportedModel{
		Name: "MiniMax-M3",
		SupportedEndpoints: []userSupportedEndpoint{{
			Protocol: string(service.ModelProtocolAnthropicMessages),
			Path:     "/v1/messages",
			GroupIDs: []int64{9, 3},
		}},
	}

	upsertUserSupportedEndpoint(&model, service.ModelProtocolAnthropicMessages, []int64{7, 3})
	upsertUserSupportedEndpoint(&model, service.ModelProtocolOpenAIChat, []int64{7})

	require.Equal(t, []userSupportedEndpoint{
		{Protocol: string(service.ModelProtocolAnthropicMessages), Path: "/v1/messages", GroupIDs: []int64{3, 7, 9}},
		{Protocol: string(service.ModelProtocolOpenAIChat), Path: "/v1/chat/completions", GroupIDs: []int64{7}},
	}, model.SupportedEndpoints)
}
