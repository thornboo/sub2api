package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
)

type modelProtocolCapabilityRepoStub struct {
	items            []AccountModelProtocolCapability
	itemsByAccount   map[int64][]AccountModelProtocolCapability
	observations     []ModelProtocolObservation
	overrides        []ModelProtocolOverride
	batchListCalls   int
	listErr          error
	listErrByAccount map[int64]error
	listManyErr      error
}

func (r *modelProtocolCapabilityRepoStub) ListByAccount(_ context.Context, accountID int64) ([]AccountModelProtocolCapability, error) {
	if err := r.listErrByAccount[accountID]; err != nil {
		return nil, err
	}
	if r.listErr != nil {
		return nil, r.listErr
	}
	if r.itemsByAccount != nil {
		return append([]AccountModelProtocolCapability(nil), r.itemsByAccount[accountID]...), nil
	}
	return append([]AccountModelProtocolCapability(nil), r.items...), nil
}

func (r *modelProtocolCapabilityRepoStub) ListByAccountIDs(_ context.Context, accountIDs []int64) (map[int64][]AccountModelProtocolCapability, error) {
	r.batchListCalls++
	if r.listManyErr != nil {
		return nil, r.listManyErr
	}
	result := make(map[int64][]AccountModelProtocolCapability, len(accountIDs))
	for _, accountID := range accountIDs {
		if r.itemsByAccount != nil {
			result[accountID] = append([]AccountModelProtocolCapability(nil), r.itemsByAccount[accountID]...)
			continue
		}
		result[accountID] = append([]AccountModelProtocolCapability(nil), r.items...)
	}
	return result, nil
}

type modelProtocolCatalogAccountRepoStub struct {
	AccountRepository
	accounts   []*Account
	getByCalls int
}

func (r *modelProtocolCatalogAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	for _, account := range r.accounts {
		if account != nil && account.ID == id {
			return account, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (r *modelProtocolCatalogAccountRepoStub) GetByIDs(_ context.Context, _ []int64) ([]*Account, error) {
	r.getByCalls++
	return r.accounts, nil
}

type modelProtocolCatalogGroupRepoStub struct {
	GroupRepository
	groups        []Group
	accountIDs    []int64
	listCalls     int
	accountIDCall int
}

func (r *modelProtocolCatalogGroupRepoStub) ListActive(_ context.Context) ([]Group, error) {
	r.listCalls++
	return append([]Group(nil), r.groups...), nil
}

func (r *modelProtocolCatalogGroupRepoStub) ListActiveByPlatform(_ context.Context, platform string) ([]Group, error) {
	r.listCalls++
	if platform != PlatformOpenAI {
		return nil, nil
	}
	return append([]Group(nil), r.groups...), nil
}

func (r *modelProtocolCatalogGroupRepoStub) GetAccountIDsByGroupIDs(_ context.Context, _ []int64) ([]int64, error) {
	r.accountIDCall++
	return append([]int64(nil), r.accountIDs...), nil
}

func (r *modelProtocolCapabilityRepoStub) SyncObserved(_ context.Context, _ int64, observations []ModelProtocolObservation) error {
	r.observations = append([]ModelProtocolObservation(nil), observations...)
	return nil
}

func (r *modelProtocolCapabilityRepoStub) UpdateOverrides(_ context.Context, _ int64, overrides []ModelProtocolOverride) error {
	r.overrides = append([]ModelProtocolOverride(nil), overrides...)
	return nil
}

func TestModelProtocolCapabilityResolvePrecedence(t *testing.T) {
	t.Parallel()
	repo := &modelProtocolCapabilityRepoStub{items: []AccountModelProtocolCapability{
		{UpstreamModel: "*", Protocol: ModelProtocolAnthropicMessages, OverrideState: ModelProtocolStateUnsupported, ObservedState: ModelProtocolStateSupported},
		{UpstreamModel: "MiniMax-M3", Protocol: ModelProtocolAnthropicMessages, OverrideState: ModelProtocolStateAuto, ObservedState: ModelProtocolStateSupported, ObservedSource: "upstream_model_list"},
	}}
	svc := &ModelProtocolCapabilityService{repo: repo}

	state, source, err := svc.Resolve(context.Background(), 7, "MiniMax-M3", ModelProtocolAnthropicMessages, false)
	require.NoError(t, err)
	require.Equal(t, ModelProtocolStateUnsupported, state)
	require.Equal(t, "admin_override", source)

	repo.items[0].OverrideState = ModelProtocolStateAuto
	svc.invalidate(7)
	state, source, err = svc.Resolve(context.Background(), 7, "MiniMax-M3", ModelProtocolAnthropicMessages, false)
	require.NoError(t, err)
	require.Equal(t, ModelProtocolStateSupported, state)
	require.Equal(t, "upstream_model_list", source)
}

func TestModelProtocolCapabilitySyncCatalogKeepsMissingAndUnknownSafe(t *testing.T) {
	t.Parallel()
	repo := &modelProtocolCapabilityRepoStub{}
	svc := &ModelProtocolCapabilityService{repo: repo}

	result, err := svc.SyncCatalog(context.Background(), 9, []UpstreamModelDescriptor{
		{ID: "Kimi-K2"},
		{ID: "MiniMax-M3", SupportedEndpointTypes: []string{"anthropic", "vendor-special"}, EndpointTypesPresent: true, EndpointTypesComplete: true},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Kimi-K2", "MiniMax-M3"}, result.Models)
	require.Len(t, result.Warnings, 2)
	require.Contains(t, result.Warnings[0]+result.Warnings[1], "did not declare supported_endpoint_types")

	states := make(map[string]ModelProtocolState)
	sources := make(map[string]string)
	for _, observation := range repo.observations {
		key := observation.UpstreamModel + ":" + string(observation.Protocol)
		states[key] = observation.State
		sources[key] = observation.Source
		require.WithinDuration(t, time.Now(), observation.ObservedAt, time.Second)
	}
	require.Equal(t, ModelProtocolStateSupported, states["MiniMax-M3:anthropic_messages"])
	require.Equal(t, ModelProtocolStateUnknown, states["MiniMax-M3:openai_chat_completions"])
	require.Equal(t, ModelProtocolStateUnknown, states["Kimi-K2:anthropic_messages"])
	require.Equal(t, "upstream_model_list_missing", sources["Kimi-K2:anthropic_messages"])
}

func TestModelProtocolCapabilitySyncCatalogKeepsEmptyEndpointTypesUnknown(t *testing.T) {
	t.Parallel()
	repo := &modelProtocolCapabilityRepoStub{}
	svc := &ModelProtocolCapabilityService{repo: repo}

	result, err := svc.SyncCatalog(context.Background(), 9, []UpstreamModelDescriptor{
		{ID: "MiniMax-M3", SupportedEndpointTypes: []string{}, EndpointTypesPresent: true, EndpointTypesComplete: false},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"MiniMax-M3"}, result.Models)
	require.Len(t, result.Warnings, 1)
	require.Contains(t, result.Warnings[0], "empty supported_endpoint_types array")
	require.Len(t, repo.observations, len(AllModelProtocols))
	for _, observation := range repo.observations {
		require.Equal(t, "MiniMax-M3", observation.UpstreamModel)
		require.Equal(t, ModelProtocolStateUnknown, observation.State)
		require.Equal(t, "upstream_model_list_empty", observation.Source)
	}
}

func TestModelProtocolCapabilityUpdateOverridesRejectsComplexWildcard(t *testing.T) {
	t.Parallel()
	repo := &modelProtocolCapabilityRepoStub{}
	svc := &ModelProtocolCapabilityService{repo: repo}

	err := svc.UpdateOverrides(context.Background(), 1, []ModelProtocolOverride{{
		UpstreamModel: "MiniMax-*",
		Protocol:      ModelProtocolAnthropicMessages,
		State:         ModelProtocolStateSupported,
	}})
	require.EqualError(t, err, "only the exact model name or * is supported")
	require.Empty(t, repo.overrides)
}

func TestSanitizeUnknownEndpointTypesBoundsAndRemovesControlCharacters(t *testing.T) {
	t.Parallel()
	values := make([]string, 0, 12)
	values = append(values, "vendor\nsecret")
	for i := 0; i < 11; i++ {
		values = append(values, "unknown-"+string(rune('a'+i))+"-"+strings.Repeat("x", 80))
	}

	sanitized, total := sanitizeUnknownEndpointTypes(values)
	require.Equal(t, 12, total)
	require.Len(t, sanitized, maxUnknownEndpointTypesPerWarning)
	for _, value := range sanitized {
		require.NotContains(t, value, "\n")
		require.LessOrEqual(t, utf8.RuneCountInString(strings.TrimSuffix(value, "...")), maxWarningValueRunes)
	}
}

func TestSelectAccountWithSchedulerForNativeProtocolSkipsHigherPriorityLegacyCandidate(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	legacy := Account{ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10}
	native := Account{ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{UpstreamModel: "MiniMax-M3", Protocol: ModelProtocolAnthropicMessages, OverrideState: ModelProtocolStateAuto, ObservedState: ModelProtocolStateSupported, ObservedSource: "upstream_model_list"}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{legacy, native}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForNativeProtocol(
		context.Background(), &groupID, "", "MiniMax-M3", "MiniMax-M3", nil, ModelProtocolAnthropicMessages, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.Equal(t, "upstream_model_list", delivery.CapabilitySource)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForNativeProtocolDoesNotBindRejectedCandidate(t *testing.T) {
	for _, tc := range []struct {
		name             string
		loadBatchEnabled bool
	}{
		{name: "legacy_non_batch", loadBatchEnabled: false},
		{name: "legacy_load_batch", loadBatchEnabled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			groupID := int64(808)
			account := Account{ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
			capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
				81: {{
					UpstreamModel: "MiniMax-M3",
					Protocol:      ModelProtocolAnthropicMessages,
					OverrideState: ModelProtocolStateUnsupported,
				}},
			}}
			cfg := &config.Config{}
			cfg.Gateway.NativeModelProtocolRoutingEnabled = true
			cfg.Gateway.Scheduling.LoadBatchEnabled = tc.loadBatchEnabled
			cache := &schedulerTestGatewayCache{}
			svc := &OpenAIGatewayService{
				accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
				cache:                   cache,
				cfg:                     cfg,
				concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
				modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
			}

			selection, _, _, err := svc.SelectAccountWithSchedulerForNativeProtocol(
				context.Background(), &groupID, "rejected-candidate", "MiniMax-M3", "MiniMax-M3", nil, ModelProtocolAnthropicMessages, PlatformOpenAI,
			)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Nil(t, selection)
			require.Empty(t, cache.sessionBindings, "capability-rejected candidates must not create sticky bindings")
		})
	}
}

func TestSelectAccountWithSchedulerForNativeProtocolDoesNotMutateExistingSticky(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	defer resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		81: {{
			UpstreamModel: "MiniMax-M3",
			Protocol:      ModelProtocolAnthropicMessages,
			OverrideState: ModelProtocolStateUnsupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:rejected-advanced": 81}}
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   cache,
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		rateLimitService:        newOpenAIAdvancedSchedulerRateLimitService("true"),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, _, err := svc.SelectAccountWithSchedulerForNativeProtocol(
		context.Background(), &groupID, "rejected-advanced", "MiniMax-M3", "MiniMax-M3", nil, ModelProtocolAnthropicMessages, PlatformOpenAI,
	)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, selection)
	require.Equal(t, int64(81), cache.sessionBindings["openai:rejected-advanced"])
	require.Empty(t, cache.deletedSessions, "candidate probing must not clear an existing sticky binding")
	require.Empty(t, cache.refreshedSessions, "candidate probing must not refresh an existing sticky binding")
}

func TestSelectAccountWithSchedulerForNativeMessagesIgnoresChatResponsesRoutePreference(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{
			openAIEndpointCapabilitiesCredentialKey: []string{},
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{
			UpstreamModel: "MiniMax-M3", Protocol: ModelProtocolAnthropicMessages,
			OverrideState: ModelProtocolStateSupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForNativeProtocol(
		context.Background(), &groupID, "", "MiniMax-M3", "MiniMax-M3", nil, ModelProtocolAnthropicMessages, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.Equal(t, "admin_override", delivery.CapabilitySource)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForNativeMessagesClassifiesCapabilityStoreFailure(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
	}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: &modelProtocolCapabilityRepoStub{listErr: errors.New("database unavailable")}},
	}

	selection, _, _, err := svc.SelectAccountWithSchedulerForNativeProtocol(
		context.Background(), &groupID, "", "MiniMax-M3", "MiniMax-M3", nil, ModelProtocolAnthropicMessages, PlatformOpenAI,
	)
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrModelProtocolCapabilityUnavailable)
}

func TestSelectAccountWithSchedulerForProtocolDeliveryUsesForceChatTransport(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateUnsupported},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIResponses, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIChat, delivery.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeCompatibility, delivery.Mode)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryUsesForceResponsesTransport(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateUnsupported},
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateSupported},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIResponses, delivery.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeCompatibility, delivery.Mode)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryKeepsPublicAndMappedModelsSeparate(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"glm-channel": "glm-upstream"},
		},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{
			UpstreamModel: "glm-upstream",
			Protocol:      ModelProtocolOpenAIChat,
			OverrideState: ModelProtocolStateSupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-public", "glm-channel", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, "glm-public", delivery.PublicModel)
	require.Equal(t, "glm-channel", delivery.ChannelMappedModel)
	require.Equal(t, "glm-upstream", delivery.UpstreamModel)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryPreservesLegacyOnlyForUnknownCapability(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	explicitlyUnsupported := Account{
		ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	unknown := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		81: {{
			UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat,
			OverrideState: ModelProtocolStateUnsupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{explicitlyUnsupported, unknown}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolStateUnknown, delivery.CapabilityState)
	require.Equal(t, "existing_gateway_contract", delivery.CapabilitySource)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryDoesNotBypassExplicitUnsupported(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{
			UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat,
			OverrideState: ModelProtocolStateUnsupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
	require.False(t, ShouldUseLegacyProtocolDeliverySelector(err, delivery))
}

func TestSelectAccountWithSchedulerForProtocolDeliveryDoesNotReopenBlockedAccountOnLaterStoreFailure(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	blocked := Account{
		ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	storeFailure := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{
		itemsByAccount: map[int64][]AccountModelProtocolCapability{
			81: {{
				UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat,
				OverrideState: ModelProtocolStateUnsupported,
			}},
		},
		listErrByAccount: map[int64]error{82: errors.New("database unavailable")},
	}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{blocked, storeFailure}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrModelProtocolCapabilityUnavailable)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
	require.False(t, ShouldUseLegacyProtocolDeliverySelector(err, delivery))
}

func TestSelectAccountWithSchedulerForProtocolDeliveryDoesNotBindRejectedPreviousResponseCandidate(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	ctx := context.Background()
	groupID := int64(808)
	account := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode:             string(openai_compat.ResponsesSupportModeForceChatCompletions),
			"openai_apikey_responses_websockets_v2_enabled": true,
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{
			UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat,
			OverrideState: ModelProtocolStateUnsupported,
		}},
	}}
	cache := &schedulerTestGatewayCache{}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.OAuthEnabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   cache,
		cfg:                     cfg,
		rateLimitService:        newOpenAIAdvancedSchedulerRateLimitService("true"),
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}
	require.NoError(t, svc.getOpenAIWSStateStore().BindResponseAccount(ctx, groupID, "resp_rejected", account.ID, time.Hour))

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		ctx, &groupID, "resp_rejected", "session_rejected", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, true, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
	require.NotContains(t, cache.sessionBindings, "openai:session_rejected")
}

func TestShouldUseLegacyProtocolDeliverySelectorOnlyBeforeAuthoritativeDecision(t *testing.T) {
	t.Parallel()
	require.True(t, ShouldUseLegacyProtocolDeliverySelector(ErrNoAvailableAccounts, ModelDeliveryDecision{}))
	require.False(t, ShouldUseLegacyProtocolDeliverySelector(ErrModelProtocolCapabilityUnavailable, ModelDeliveryDecision{
		ReasonCodes: []ModelDeliveryReasonCode{ModelDeliveryReasonCapabilityUnsupported},
	}))
	require.True(t, ShouldUseLegacyProtocolDeliverySelector(ErrModelProtocolCapabilityUnavailable, ModelDeliveryDecision{}))
	require.False(t, ShouldUseLegacyProtocolDeliverySelector(errors.New("unexpected"), ModelDeliveryDecision{}))
	require.False(t, ShouldUseLegacyProtocolDeliverySelector(ErrNoAvailableAccounts, ModelDeliveryDecision{
		ReasonCodes: []ModelDeliveryReasonCode{ModelDeliveryReasonCapabilityUnsupported},
	}))
}

func TestUseOpenAIResponsesForSelectedDeliveryHonorsCanonicalProtocol(t *testing.T) {
	t.Parallel()
	forceChat := &Account{
		ID: 1, Type: AccountTypeAPIKey,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	forceResponses := &Account{
		ID: 2, Type: AccountTypeAPIKey,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
		},
	}

	useResponses, err := useOpenAIResponsesForSelectedDelivery(forceChat, "")
	require.NoError(t, err)
	require.False(t, useResponses)

	useResponses, err = useOpenAIResponsesForSelectedDelivery(forceChat, ModelProtocolOpenAIResponses)
	require.NoError(t, err)
	require.True(t, useResponses)

	useResponses, err = useOpenAIResponsesForSelectedDelivery(forceResponses, ModelProtocolOpenAIChat)
	require.NoError(t, err)
	require.False(t, useResponses)

	_, err = useOpenAIResponsesForSelectedDelivery(&Account{ID: 3, Type: AccountTypeOAuth}, ModelProtocolOpenAIChat)
	require.Error(t, err)
}

func TestResolveNativeProtocolsForGroupsBatchesAccountsAndUsesFinalAccountModel(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups:     []Group{{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, AllowMessagesDispatch: true}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
		GroupIDs: []int64{10},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"MiniMax-M3": "MiniMax-M3-upstream"},
		},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{UpstreamModel: "MiniMax-M3-upstream", Protocol: ModelProtocolAnthropicMessages, OverrideState: ModelProtocolStateAuto, ObservedState: ModelProtocolStateSupported, ObservedSource: "upstream_model_list"}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	svc := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)

	result, err := svc.ResolveNativeProtocolsForGroups(context.Background(), []int64{10}, []string{"MiniMax-M3"})
	require.NoError(t, err)
	require.Equal(t, []int64{10}, result["MiniMax-M3"][ModelProtocolAnthropicMessages])
	require.Equal(t, 1, groupRepo.listCalls)
	require.Equal(t, 1, groupRepo.accountIDCall)
	require.Equal(t, 1, accountRepo.getByCalls)
	require.Equal(t, 1, capRepo.batchListCalls)
}

func TestModelDeliveryRequiresStableRouteAndPreservesCompatibilityMessages(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI 主线路", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: true,
		}},
		accountIDs: []int64{82},
	}
	future := time.Now().Add(time.Hour)
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Name: "new-api-A", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
		RateLimitResetAt: &future,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"MiniMax-M3": "MiniMax-M3-upstream"},
		},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{
			UpstreamModel: "MiniMax-M3-upstream", Protocol: ModelProtocolOpenAIChat,
			OverrideState: ModelProtocolStateAuto, ObservedState: ModelProtocolStateSupported,
			ObservedSource: "upstream_model_list",
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"MiniMax-M3"})
	require.NoError(t, err)
	group := projection.Group("MiniMax-M3", 10)
	require.NotNil(t, group)
	require.True(t, group.Deliverable(), "transient rate limiting must not remove stable catalog delivery")
	require.Len(t, group.Routes, 1)
	require.Equal(t, "MiniMax-M3-upstream", group.Routes[0].UpstreamModel)
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolAnthropicMessages])
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIChat])
	require.Equal(t, []int64{10}, projection.EndpointGroupIDs("MiniMax-M3", ModelProtocolAnthropicMessages))
}

func TestModelDeliveryUsesMessagesDispatchModelInProtocolDecision(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: true,
			MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{SonnetMappedModel: "glm-5.2"},
		}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"glm-5.2": "glm-upstream"},
		},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {{
			UpstreamModel: "glm-upstream", Protocol: ModelProtocolAnthropicMessages,
			OverrideState: ModelProtocolStateSupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"claude-sonnet-4-5"})
	require.NoError(t, err)
	group := projection.Group("claude-sonnet-4-5", 10)
	require.NotNil(t, group)
	require.True(t, group.StableRouteAvailable())
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolAnthropicMessages])
	require.Len(t, group.Routes, 1)
	messagesDecision := group.Routes[0].Decisions[ModelProtocolAnthropicMessages]
	require.Equal(t, "glm-5.2", messagesDecision.ChannelMappedModel)
	require.Equal(t, "glm-upstream", messagesDecision.UpstreamModel)
	require.Contains(t, group.Routes[0].Decisions[ModelProtocolOpenAIChat].ReasonCodes, ModelDeliveryReasonModelUnsupported)
}

func TestModelDeliveryForceChatPolicyKeepsAllPublicEndpointsWithTruthfulModes(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: true,
		}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolAnthropicMessages, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateSupported},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"glm-5.2"})
	require.NoError(t, err)
	group := projection.Group("glm-5.2", 10)
	require.NotNil(t, group)
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolAnthropicMessages])
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIChat])
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolOpenAIResponses])
}

func TestModelDeliveryForceResponsesPolicyKeepsAllPublicEndpointsWithTruthfulModes(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: true,
		}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
		},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolAnthropicMessages, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "glm-5.2", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateSupported},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"glm-5.2"})
	require.NoError(t, err)
	group := projection.Group("glm-5.2", 10)
	require.NotNil(t, group)
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolAnthropicMessages])
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolOpenAIChat])
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIResponses])
}

func TestModelDeliveryKeepsPublishedModelNonDeliverableWithoutEligibleAccount(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups:     []Group{{ID: 10, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive, AllowMessagesDispatch: true}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusDisabled, Schedulable: true, GroupIDs: []int64{10},
	}}}
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"orphan-model"})
	require.NoError(t, err)
	group := projection.Group("orphan-model", 10)
	require.NotNil(t, group, "admin diagnostics must retain the configured group even without routes")
	require.False(t, group.Deliverable())
	require.Empty(t, projection.DeliverableGroupIDs("orphan-model"))
	require.Empty(t, projection.EndpointGroupIDs("orphan-model", ModelProtocolAnthropicMessages))
}

func TestModelDeliveryDoesNotAdvertiseStableRouteWithoutPublicEndpoint(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: false,
		}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
	}}}
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"MiniMax-M3"})
	require.NoError(t, err)
	group := projection.Group("MiniMax-M3", 10)
	require.NotNil(t, group)
	require.True(t, group.StableRouteAvailable(), "the route remains visible to administrators for diagnosis")
	require.False(t, group.Deliverable(), "a route without a customer-callable endpoint is not deliverable")
	require.Empty(t, projection.DeliverableGroupIDs("MiniMax-M3"))
}

func TestModelDeliveryResponsesBridgeDoesNotDependOnChatCapability(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: true,
		}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		82: {
			{
				UpstreamModel: "MiniMax-M3", Protocol: ModelProtocolOpenAIChat,
				OverrideState: ModelProtocolStateUnsupported,
			},
			{
				UpstreamModel: "MiniMax-M3", Protocol: ModelProtocolOpenAIResponses,
				OverrideState: ModelProtocolStateSupported,
			},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"MiniMax-M3"})
	require.NoError(t, err)
	group := projection.Group("MiniMax-M3", 10)
	require.NotNil(t, group)
	require.True(t, group.Deliverable())
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolAnthropicMessages])
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolOpenAIChat])
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIResponses])
}

func TestModelDeliveryCapabilityLookupFailureKeepsProvableCompatibilityRoute(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
			Status: StatusActive, AllowMessagesDispatch: true,
		}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{listManyErr: errors.New("capability repository unavailable")}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{10}, []string{"MiniMax-M3"})
	require.NoError(t, err)
	require.NotEmpty(t, projection.Warnings)
	require.Equal(t, []int64{10}, projection.EndpointGroupIDs("MiniMax-M3", ModelProtocolAnthropicMessages))
	require.Empty(t, projection.EndpointGroupIDs("MiniMax-M3", ModelProtocolOpenAIChat))
}

func TestMergeModelDeliveryModePreservesMixedAccountRoutes(t *testing.T) {
	t.Parallel()
	require.Equal(t, ModelDeliveryModeNative, mergeModelDeliveryMode("", ModelDeliveryModeNative))
	require.Equal(t, ModelDeliveryModeNative, mergeModelDeliveryMode(ModelDeliveryModeNative, ModelDeliveryModeNative))
	require.Equal(t, ModelDeliveryModeMixed, mergeModelDeliveryMode(ModelDeliveryModeNative, ModelDeliveryModeCompatibility))
}

type modelDeliveryChannelRepoStub struct {
	ChannelRepository
	channels  []Channel
	platforms map[int64]string
}

func (r *modelDeliveryChannelRepoStub) ListAll(_ context.Context) ([]Channel, error) {
	return append([]Channel(nil), r.channels...), nil
}

func (r *modelDeliveryChannelRepoStub) GetGroupPlatforms(_ context.Context, _ []int64) (map[int64]string, error) {
	return r.platforms, nil
}

func TestResolveAccountImpactsJoinsPublicAndFinalUpstreamModels(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{groups: []Group{{
		ID: 10, Name: "OpenAI 主线路", Platform: PlatformOpenAI, Status: StatusActive,
	}}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Name: "new-api-A", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"MiniMax-M3": "MiniMax-M3-upstream"},
		},
	}}}
	channelRepo := &modelDeliveryChannelRepoStub{
		channels: []Channel{
			{
				ID: 5, Name: "国内模型", Status: StatusActive, GroupIDs: []int64{10},
				ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"MiniMax-M3"}}},
			},
			{
				ID: 6, Name: "已停用目录", Status: StatusDisabled, GroupIDs: []int64{10},
				ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"hidden-model"}}},
			},
		},
		platforms: map[int64]string{10: PlatformOpenAI},
	}
	channel := NewChannelService(channelRepo, groupRepo, nil, nil)
	svc := NewModelDeliveryService(accountRepo, groupRepo, channel, nil, &config.Config{})

	impacts, err := svc.ResolveAccountImpacts(context.Background(), 82)
	require.NoError(t, err)
	require.Equal(t, []AccountPublicModelImpact{{
		UpstreamModel: "MiniMax-M3-upstream",
		PublicModel:   "MiniMax-M3",
		ChannelID:     5,
		ChannelName:   "国内模型",
		GroupID:       10,
		GroupName:     "OpenAI 主线路",
		Platform:      PlatformOpenAI,
	}}, impacts["MiniMax-M3-upstream"])
	require.NotContains(t, impacts, "hidden-model", "disabled channels must not appear as current public impact")
}

func TestResolveAccountImpactsIncludesMessagesDispatchUpstreamModel(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{groups: []Group{{
		ID: 10, Name: "OpenAI 主线路", Platform: PlatformOpenAI, Status: StatusActive,
		AllowMessagesDispatch:       true,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{SonnetMappedModel: "glm-5.2"},
	}}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Name: "new-api-A", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"glm-5.2": "glm-upstream"},
		},
	}}}
	channelRepo := &modelDeliveryChannelRepoStub{
		channels: []Channel{{
			ID: 5, Name: "Claude 入口", Status: StatusActive, GroupIDs: []int64{10},
			ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"claude-sonnet-4-5"}}},
		}},
		platforms: map[int64]string{10: PlatformOpenAI},
	}
	channel := NewChannelService(channelRepo, groupRepo, nil, nil)
	svc := NewModelDeliveryService(accountRepo, groupRepo, channel, nil, &config.Config{})

	impacts, err := svc.ResolveAccountImpacts(context.Background(), 82)
	require.NoError(t, err)
	require.Equal(t, []AccountPublicModelImpact{{
		UpstreamModel: "glm-upstream",
		PublicModel:   "claude-sonnet-4-5",
		ChannelID:     5,
		ChannelName:   "Claude 入口",
		GroupID:       10,
		GroupName:     "OpenAI 主线路",
		Platform:      PlatformOpenAI,
	}}, impacts["glm-upstream"])
}
