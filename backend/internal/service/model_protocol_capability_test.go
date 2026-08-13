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

func TestScopeAccountModelProtocolCapabilitiesUsesMappedUpstreamModels(t *testing.T) {
	t.Parallel()

	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"minimax-m2.5": "minimax-m2.5",
				"minimax-m2.7": "MiniMax-M2.7",
			},
		},
	}
	items := []AccountModelProtocolCapability{
		{UpstreamModel: ModelProtocolWildcardModel, Protocol: ModelProtocolAnthropicMessages},
		{UpstreamModel: "glm-5", Protocol: ModelProtocolAnthropicMessages},
		{UpstreamModel: "minimax-m2.5", Protocol: ModelProtocolAnthropicMessages},
		{UpstreamModel: "MiniMax-M2.7", Protocol: ModelProtocolOpenAIChat},
	}

	scoped, models, restricted := ScopeAccountModelProtocolCapabilities(account, items)

	require.True(t, restricted)
	require.Equal(t, []string{"MiniMax-M2.7", "minimax-m2.5"}, models)
	require.Equal(t, []AccountModelProtocolCapability{
		{UpstreamModel: ModelProtocolWildcardModel, Protocol: ModelProtocolAnthropicMessages},
		{UpstreamModel: "minimax-m2.5", Protocol: ModelProtocolAnthropicMessages},
		{UpstreamModel: "MiniMax-M2.7", Protocol: ModelProtocolOpenAIChat},
	}, scoped)
}

func TestModelProtocolCapabilitySyncCatalogForAccountIgnoresUnmappedUpstreamModels(t *testing.T) {
	t.Parallel()

	repo := &modelProtocolCapabilityRepoStub{}
	svc := &ModelProtocolCapabilityService{repo: repo}
	account := &Account{
		ID:       9,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"minimax-m2.5": "minimax-m2.5",
				"minimax-m2.7": "MiniMax-M2.7",
			},
		},
	}

	result, err := svc.SyncCatalogForAccount(context.Background(), account, []UpstreamModelDescriptor{
		{ID: "glm-5", SupportedEndpointTypes: []string{"anthropic"}, EndpointTypesPresent: true, EndpointTypesComplete: true},
		{ID: "minimax-m2.5", SupportedEndpointTypes: []string{"anthropic"}, EndpointTypesPresent: true, EndpointTypesComplete: true},
		{ID: "MiniMax-M2.7", SupportedEndpointTypes: []string{"anthropic", "openai"}, EndpointTypesPresent: true, EndpointTypesComplete: true},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"MiniMax-M2.7", "minimax-m2.5"}, result.Models)
	require.NotEmpty(t, repo.observations)
	for _, observation := range repo.observations {
		require.NotEqual(t, "glm-5", observation.UpstreamModel)
		require.Contains(t, result.Models, observation.UpstreamModel)
	}
}

func TestModelProtocolCapabilityUpdateOverridesForAccountRejectsUnmappedModel(t *testing.T) {
	t.Parallel()

	repo := &modelProtocolCapabilityRepoStub{}
	svc := &ModelProtocolCapabilityService{repo: repo}
	account := &Account{
		ID:       9,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"minimax-m2.5": "minimax-m2.5",
			},
		},
	}

	err := svc.UpdateOverridesForAccount(context.Background(), account, []ModelProtocolOverride{
		{UpstreamModel: "glm-5", Protocol: ModelProtocolAnthropicMessages, State: ModelProtocolStateSupported},
	})

	var validationErr *ModelProtocolCapabilityValidationError
	require.ErrorAs(t, err, &validationErr)
	require.Contains(t, validationErr.Error(), "not configured in account model_mapping")
	require.Empty(t, repo.overrides)
}

func TestModelProtocolCapabilityUpdateOverridesForAccountAllowsMappedAndWildcardModels(t *testing.T) {
	t.Parallel()

	repo := &modelProtocolCapabilityRepoStub{}
	svc := &ModelProtocolCapabilityService{repo: repo}
	account := &Account{
		ID:       9,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"minimax-m2.5": "minimax-m2.5",
			},
		},
	}
	overrides := []ModelProtocolOverride{
		{UpstreamModel: ModelProtocolWildcardModel, Protocol: ModelProtocolOpenAIChat, State: ModelProtocolStateSupported},
		{UpstreamModel: "minimax-m2.5", Protocol: ModelProtocolAnthropicMessages, State: ModelProtocolStateSupported},
	}

	err := svc.UpdateOverridesForAccount(context.Background(), account, overrides)

	require.NoError(t, err)
	require.Equal(t, overrides, repo.overrides)
}

func TestEvaluateModelDeliveryCandidateRequiresExactSupportedProtocolForOpenAIAPIKey(t *testing.T) {
	t.Parallel()
	account := &Account{
		ID:          9,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode:      string(openai_compat.ResponsesSupportModeAuto),
			openai_compat.ExtraKeyResponsesSupported: true,
		},
	}
	capabilities := []AccountModelProtocolCapability{
		{
			UpstreamModel:  "deepseek-v4-pro",
			Protocol:       ModelProtocolOpenAIChat,
			ObservedState:  ModelProtocolStateSupported,
			ObservedSource: "upstream_model_list",
		},
		{
			UpstreamModel:  "deepseek-v4-pro",
			Protocol:       ModelProtocolOpenAIResponses,
			ObservedState:  ModelProtocolStateUnsupported,
			ObservedSource: "upstream_model_list",
		},
	}

	chatDecision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
		Account:              account,
		PublicModel:          "deepseek-v4-pro",
		ChannelMappedModel:   "deepseek-v4-pro",
		GroupPlatform:        PlatformOpenAI,
		InboundProtocol:      ModelProtocolOpenAIChat,
		NativeRoutingEnabled: true,
		Capabilities:         capabilities,
	})
	require.True(t, chatDecision.Eligible)
	require.Equal(t, ModelProtocolOpenAIChat, chatDecision.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeNative, chatDecision.Mode)
	require.Equal(t, "upstream_model_list", chatDecision.CapabilitySource)

	responsesDecision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
		Account:              account,
		PublicModel:          "deepseek-v4-pro",
		ChannelMappedModel:   "deepseek-v4-pro",
		GroupPlatform:        PlatformOpenAI,
		InboundProtocol:      ModelProtocolOpenAIResponses,
		NativeRoutingEnabled: true,
		Capabilities:         capabilities,
	})
	require.False(t, responsesDecision.Eligible)
	require.Equal(t, ModelProtocolOpenAIResponses, responsesDecision.UpstreamProtocol)
	require.Contains(t, responsesDecision.ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
	require.Equal(t, "upstream_model_list", responsesDecision.CapabilitySource)
}

func TestEvaluateModelDeliveryCandidateStrictRoutingIgnoresLegacyChatResponsesPreference(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name            string
		mode            openai_compat.ResponsesSupportMode
		inboundProtocol ModelProtocol
	}{
		{
			name:            "force chat does not convert responses",
			mode:            openai_compat.ResponsesSupportModeForceChatCompletions,
			inboundProtocol: ModelProtocolOpenAIResponses,
		},
		{
			name:            "force responses does not convert chat",
			mode:            openai_compat.ResponsesSupportModeForceResponses,
			inboundProtocol: ModelProtocolOpenAIChat,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			account := &Account{
				ID:          9,
				Platform:    PlatformOpenAI,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
				Extra: map[string]any{
					openai_compat.ExtraKeyResponsesMode: string(test.mode),
				},
			}
			capabilities := []AccountModelProtocolCapability{
				{
					UpstreamModel: "deepseek-v4-pro",
					Protocol:      ModelProtocolOpenAIChat,
					OverrideState: ModelProtocolStateSupported,
				},
				{
					UpstreamModel: "deepseek-v4-pro",
					Protocol:      ModelProtocolOpenAIResponses,
					OverrideState: ModelProtocolStateSupported,
				},
			}

			decision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
				Account:              account,
				PublicModel:          "deepseek-v4-pro",
				ChannelMappedModel:   "deepseek-v4-pro",
				GroupPlatform:        PlatformOpenAI,
				InboundProtocol:      test.inboundProtocol,
				NativeRoutingEnabled: true,
				Capabilities:         capabilities,
			})

			require.True(t, decision.Eligible)
			require.Equal(t, test.inboundProtocol, decision.UpstreamProtocol)
			require.Equal(t, ModelDeliveryModeNative, decision.Mode)
		})
	}
}

func TestEvaluateModelDeliveryCandidateOnlyTreatsAnthropicMessagesUpstreamAsNative(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		platform string
		wantMode ModelDeliveryMode
	}{
		{name: "anthropic", platform: PlatformAnthropic, wantMode: ModelDeliveryModeNative},
		{name: "gemini compatibility bridge", platform: PlatformGemini, wantMode: ModelDeliveryModeCompatibility},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			account := &Account{
				ID:          9,
				Platform:    test.platform,
				Type:        AccountTypeAPIKey,
				Status:      StatusActive,
				Schedulable: true,
			}

			decision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
				Account:            account,
				PublicModel:        "upstream-model",
				ChannelMappedModel: "upstream-model",
				GroupPlatform:      test.platform,
				InboundProtocol:    ModelProtocolAnthropicMessages,
			})

			require.True(t, decision.Eligible)
			require.Equal(t, ModelProtocolAnthropicMessages, decision.UpstreamProtocol)
			require.Equal(t, test.wantMode, decision.Mode)
		})
	}
}

func TestEvaluateModelDeliveryCandidateStrictChatIgnoresForceResponsesPreference(t *testing.T) {
	t.Parallel()
	account := &Account{
		ID:          10,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeAPIKey,
		Status:      StatusActive,
		Schedulable: true,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
		},
	}
	decision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
		Account:              account,
		PublicModel:          "deepseek-v4-pro",
		ChannelMappedModel:   "deepseek-v4-pro",
		GroupPlatform:        PlatformOpenAI,
		InboundProtocol:      ModelProtocolOpenAIChat,
		NativeRoutingEnabled: true,
		Capabilities: []AccountModelProtocolCapability{
			{
				UpstreamModel:  "deepseek-v4-pro",
				Protocol:       ModelProtocolOpenAIChat,
				ObservedState:  ModelProtocolStateSupported,
				ObservedSource: "upstream_model_list",
			},
			{
				UpstreamModel:  "deepseek-v4-pro",
				Protocol:       ModelProtocolOpenAIResponses,
				ObservedState:  ModelProtocolStateUnsupported,
				ObservedSource: "upstream_model_list",
			},
		},
	})

	require.True(t, decision.Eligible)
	require.Equal(t, ModelProtocolOpenAIChat, decision.UpstreamProtocol)
	require.Equal(t, ModelProtocolStateSupported, decision.CapabilityState)
	require.Equal(t, ModelDeliveryModeNative, decision.Mode)
	require.Empty(t, decision.ReasonCodes)
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

func TestSelectAccountWithSchedulerForNativeProtocolFallsBackOnlyToNonStrictAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	rejectedAPIKey := Account{
		ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
	}
	legacyOAuth := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"MiniMax-M3": "MiniMax-M3"},
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		81: {{
			UpstreamModel: "MiniMax-M3",
			Protocol:      ModelProtocolAnthropicMessages,
			OverrideState: ModelProtocolStateUnsupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{rejectedAPIKey, legacyOAuth}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForNativeProtocol(
		context.Background(), &groupID, "", "MiniMax-M3", "MiniMax-M3", nil,
		ModelProtocolAnthropicMessages, PlatformOpenAI,
	)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.Equal(t, ModelDeliveryModeCompatibility, delivery.Mode)
	require.Equal(t, ModelProtocolOpenAIResponses, delivery.UpstreamProtocol)
	require.Equal(t, "intrinsic", delivery.CapabilitySource)
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

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForNativeProtocol(
		context.Background(), &groupID, "", "MiniMax-M3", "MiniMax-M3", nil, ModelProtocolAnthropicMessages, PlatformOpenAI,
	)
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrModelProtocolCapabilityUnavailable)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnknown)
	require.False(t, ShouldUseLegacyProtocolDeliverySelector(err, delivery))
}

func TestProtocolDeliveryUsesLegacySelectorOnlyWhenStrictRoutingIsDisabled(t *testing.T) {
	for _, test := range []struct {
		name           string
		enabled        bool
		platform       string
		wantLegacy     bool
		wantReasonCode ModelDeliveryReasonCode
	}{
		{
			name:       "disabled keeps legacy selector",
			enabled:    false,
			platform:   PlatformOpenAI,
			wantLegacy: true,
		},
		{
			name:           "enabled no-route stays authoritative",
			enabled:        true,
			platform:       PlatformOpenAI,
			wantLegacy:     false,
			wantReasonCode: ModelDeliveryReasonNoStableRoute,
		},
		{
			name:       "enabled non-openai platform keeps legacy selector",
			enabled:    true,
			platform:   PlatformGrok,
			wantLegacy: true,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			groupID := int64(808)
			cfg := &config.Config{}
			cfg.Gateway.NativeModelProtocolRoutingEnabled = test.enabled
			cfg.Gateway.Scheduling.LoadBatchEnabled = false
			svc := &OpenAIGatewayService{
				accountRepo:             schedulerTestOpenAIAccountRepo{},
				cache:                   &schedulerTestGatewayCache{},
				cfg:                     cfg,
				concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
				modelProtocolCapability: &ModelProtocolCapabilityService{repo: &modelProtocolCapabilityRepoStub{}},
			}

			selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
				context.Background(), &groupID, "", "", "deepseek-v4-pro", "deepseek-v4-pro", nil,
				OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
				false, false, true, ModelProtocolOpenAIChat, test.platform,
			)

			require.Nil(t, selection)
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.Equal(t, test.wantLegacy, ShouldUseLegacyProtocolDeliverySelector(err, delivery))
			if test.wantReasonCode != "" {
				require.Contains(t, delivery.ReasonCodes, test.wantReasonCode)
			} else {
				require.Empty(t, delivery.ReasonCodes)
			}
		})
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryUsesExactResponsesProtocol(t *testing.T) {
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
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.False(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIResponses, delivery.UpstreamProtocol)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
}

func TestSelectAccountWithSchedulerForProtocolDeliveryStrictResponsesIgnoresLegacyResponsesPrefilter(t *testing.T) {
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
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityResponses,
		false, false, true, ModelProtocolOpenAIResponses, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIResponses, delivery.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeNative, delivery.Mode)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryStrictResponsesSupportsWebSocketIngress(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
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
			UpstreamModel: "glm-5.2",
			Protocol:      ModelProtocolOpenAIResponses,
			OverrideState: ModelProtocolStateSupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	cfg.Gateway.OpenAIWS.Enabled = true
	cfg.Gateway.OpenAIWS.APIKeyEnabled = true
	cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{account}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "glm-5.2", "glm-5.2", nil,
		OpenAIUpstreamTransportResponsesWebsocketV2Ingress, OpenAIEndpointCapabilityResponses,
		false, false, true, ModelProtocolOpenAIResponses, PlatformOpenAI,
	)
	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIResponses, delivery.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeNative, delivery.Mode)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliveryUsesExactChatProtocol(t *testing.T) {
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
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.False(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIChat, delivery.UpstreamProtocol)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
}

func TestSelectAccountWithSchedulerForProtocolDeliverySkipsHigherPriorityWrongProtocolAccount(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	chatOnly := Account{
		ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	responses := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		81: {
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateUnsupported},
		},
		82: {
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateUnsupported},
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateSupported},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{chatOnly, responses}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "deepseek-v4-pro", "deepseek-v4-pro", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIResponses, PlatformOpenAI,
	)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIResponses, delivery.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeNative, delivery.Mode)
	if selection.ReleaseFunc != nil {
		selection.ReleaseFunc()
	}
}

func TestSelectAccountWithSchedulerForProtocolDeliverySkipsHigherPriorityResponsesOnlyAccountForChat(t *testing.T) {
	resetOpenAIAdvancedSchedulerSettingCacheForTest()
	groupID := int64(808)
	responsesOnly := Account{
		ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 10,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
		},
	}
	chat := Account{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceResponses),
		},
	}
	capRepo := &modelProtocolCapabilityRepoStub{itemsByAccount: map[int64][]AccountModelProtocolCapability{
		81: {
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateUnsupported},
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateSupported},
		},
		82: {
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIChat, OverrideState: ModelProtocolStateSupported},
			{UpstreamModel: "deepseek-v4-pro", Protocol: ModelProtocolOpenAIResponses, OverrideState: ModelProtocolStateUnsupported},
		},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	cfg.Gateway.Scheduling.LoadBatchEnabled = false
	svc := &OpenAIGatewayService{
		accountRepo:             schedulerTestOpenAIAccountRepo{accounts: []Account{responsesOnly, chat}},
		cache:                   &schedulerTestGatewayCache{},
		cfg:                     cfg,
		concurrencyService:      NewConcurrencyService(schedulerTestConcurrencyCache{}),
		modelProtocolCapability: &ModelProtocolCapabilityService{repo: capRepo},
	}

	selection, _, delivery, err := svc.SelectAccountWithSchedulerForProtocolDelivery(
		context.Background(), &groupID, "", "", "deepseek-v4-pro", "deepseek-v4-pro", nil,
		OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions,
		false, false, true, ModelProtocolOpenAIChat, PlatformOpenAI,
	)

	require.NoError(t, err)
	require.NotNil(t, selection)
	require.Equal(t, int64(82), selection.Account.ID)
	require.True(t, delivery.Eligible)
	require.Equal(t, ModelProtocolOpenAIChat, delivery.UpstreamProtocol)
	require.Equal(t, ModelDeliveryModeNative, delivery.Mode)
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

func TestSelectAccountWithSchedulerForProtocolDeliveryRejectsUnknownStrictCapability(t *testing.T) {
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
	require.Nil(t, selection)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.False(t, delivery.Eligible)
	require.Equal(t, ModelProtocolStateUnknown, delivery.CapabilityState)
	require.Contains(t, delivery.ReasonCodes, ModelDeliveryReasonCapabilityUnknown)
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

func TestModelDeliveryRequiresStableRouteAndRejectsUnknownMessagesCapability(t *testing.T) {
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
	_, hasMessagesEndpoint := group.Endpoints[ModelProtocolAnthropicMessages]
	require.False(t, hasMessagesEndpoint)
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIChat])
	require.Empty(t, projection.EndpointGroupIDs("MiniMax-M3", ModelProtocolAnthropicMessages))
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

func TestModelDeliveryStrictRoutingIgnoresForceChatCompatibilityPreference(t *testing.T) {
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
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIResponses])
	require.Equal(t, []int64{10}, projection.EndpointGroupIDs("glm-5.2", ModelProtocolOpenAIResponses))
	require.Equal(t, []int64{10}, projection.NativeEndpointGroupIDs("glm-5.2", ModelProtocolOpenAIResponses))
	require.Equal(t, []int64{10}, projection.NativeEndpointGroupIDs("glm-5.2", ModelProtocolOpenAIChat))
}

func TestModelDeliveryStrictRoutingIgnoresForceResponsesCompatibilityPreference(t *testing.T) {
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
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIChat])
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

func TestModelDeliveryStrictRoutingPublishesOnlyExactSupportedProtocol(t *testing.T) {
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
	require.NotContains(t, group.Endpoints, ModelProtocolAnthropicMessages)
	require.NotContains(t, group.Endpoints, ModelProtocolOpenAIChat)
	require.Equal(t, ModelDeliveryModeNative, group.Endpoints[ModelProtocolOpenAIResponses])
}

func TestModelDeliveryCapabilityLookupFailureDoesNotCreateStrictRoute(t *testing.T) {
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
	require.Empty(t, projection.EndpointGroupIDs("MiniMax-M3", ModelProtocolAnthropicMessages))
	require.Empty(t, projection.EndpointGroupIDs("MiniMax-M3", ModelProtocolOpenAIChat))
	require.Empty(t, projection.CallableGroupIDs("MiniMax-M3"))
}

func TestEnterpriseMemberModelDeliveryUsesAuthorizedSnapshotsAndLegacyCompatibility(t *testing.T) {
	t.Parallel()
	group := &Group{
		ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
		Status: StatusActive, Hydrated: true,
	}
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups:     []Group{{ID: 999, Name: "unrelated", Platform: PlatformOpenAI, Status: StatusActive}},
		accountIDs: []int64{82},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesMode: string(openai_compat.ResponsesSupportModeForceChatCompletions),
		},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"published-model": "upstream-model"},
		},
	}}}
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

	for _, protocol := range []ModelProtocol{ModelProtocolOpenAIChat, ModelProtocolOpenAIResponses} {
		projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "published-model", protocol)
		require.NoError(t, err)
		require.Contains(t, projection.Group("published-model", 10).Endpoints, protocol)
	}
	require.Zero(t, groupRepo.listCalls, "request-path projection must not scan all active groups")
}

func TestEnterpriseMemberModelDeliveryAppliesStableGroupAccountPolicies(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		group    *Group
		accounts []*Account
		wantID   int64
	}{
		{
			name: "oauth only excludes API key",
			group: &Group{
				ID: 10, Name: "OpenAI", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RequireOAuthOnly: true,
			},
			accounts: []*Account{
				{ID: 81, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{10}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "api-key-upstream"}}},
				{ID: 82, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{10}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "oauth-upstream"}}},
			},
			wantID: 82,
		},
		{
			name: "privacy requirement excludes unset account",
			group: &Group{
				ID: 20, Name: "OpenAI privacy", Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true, RequirePrivacySet: true,
			},
			accounts: []*Account{
				{ID: 91, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{20}, Extra: map[string]any{"privacy_mode": "failed"}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "unset-upstream"}}},
				{ID: 92, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, GroupIDs: []int64{20}, Extra: map[string]any{"privacy_mode": PrivacyModeTrainingOff}, Credentials: map[string]any{"model_mapping": map[string]any{"published-model": "private-upstream"}}},
			},
			wantID: 92,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountIDs := make([]int64, 0, len(tt.accounts))
			for _, account := range tt.accounts {
				accountIDs = append(accountIDs, account.ID)
			}
			groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: accountIDs}
			accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: tt.accounts}
			svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

			projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{tt.group}, "published-model", ModelProtocolOpenAIResponses)
			require.NoError(t, err)
			groupProjection := projection.Group("published-model", tt.group.ID)
			require.NotNil(t, groupProjection)
			require.Len(t, groupProjection.Routes, 1)
			require.Equal(t, tt.wantID, groupProjection.Routes[0].AccountID)
		})
	}
}

func TestEnterpriseMemberModelDeliverySupportsGrokCompatibility(t *testing.T) {
	t.Parallel()
	group := &Group{
		ID: 20, Name: "Grok", Platform: PlatformGrok,
		Status: StatusActive, Hydrated: true,
	}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{92}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 92, Platform: PlatformGrok, Type: AccountTypeOAuth,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{20},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-route-model": "grok-upstream-model"},
		},
	}}}
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})

	for _, protocol := range []ModelProtocol{ModelProtocolOpenAIChat, ModelProtocolOpenAIResponses} {
		projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "grok-route-model", protocol)
		require.NoError(t, err)
		require.Contains(t, projection.Group("grok-route-model", 20).Endpoints, protocol)
	}
}

func TestCatalogModelDeliverySupportsGrokTextCompatibility(t *testing.T) {
	t.Parallel()
	groupRepo := &modelProtocolCatalogGroupRepoStub{
		groups: []Group{{
			ID: 20, Name: "Grok", Platform: PlatformGrok,
			Status: StatusActive,
		}},
		accountIDs: []int64{92},
	}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 92, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{20},
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-route-model": "grok-upstream-model"},
		},
	}}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, cfg)

	projection, err := svc.ResolveForGroups(context.Background(), []int64{20}, []string{"grok-route-model"})

	require.NoError(t, err)
	group := projection.Group("grok-route-model", 20)
	require.NotNil(t, group)
	require.True(t, group.Deliverable())
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolOpenAIChat])
	require.Equal(t, ModelDeliveryModeCompatibility, group.Endpoints[ModelProtocolOpenAIResponses])
	require.Equal(t, []int64{20}, projection.EndpointGroupIDs("grok-route-model", ModelProtocolOpenAIChat))
	require.Equal(t, []int64{20}, projection.EndpointGroupIDs("grok-route-model", ModelProtocolOpenAIResponses))
	for _, protocol := range []ModelProtocol{ModelProtocolOpenAIChat, ModelProtocolOpenAIResponses} {
		decision := group.Routes[0].Decisions[protocol]
		require.True(t, decision.Eligible)
		require.Equal(t, ModelProtocolOpenAIResponses, decision.UpstreamProtocol)
		require.Equal(t, "existing_grok_gateway_contract", decision.CapabilitySource)
	}
}

func TestCatalogModelDeliveryKeepsGrokTextBehindGlobalRoutingGate(t *testing.T) {
	t.Parallel()
	account := &Account{
		ID: 92, Platform: PlatformGrok, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true,
		Credentials: map[string]any{
			"model_mapping": map[string]any{"grok-route-model": "grok-upstream-model"},
		},
	}

	decision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
		Account:            account,
		PublicModel:        "grok-route-model",
		ChannelMappedModel: "grok-route-model",
		GroupPlatform:      PlatformGrok,
		InboundProtocol:    ModelProtocolOpenAIChat,
	})

	require.False(t, decision.Eligible)
	require.Contains(t, decision.ReasonCodes, ModelDeliveryReasonGlobalRoutingDisabled)
}

func TestEnterpriseMemberModelDeliveryFailsOnCapabilityDependencyError(t *testing.T) {
	t.Parallel()
	group := &Group{
		ID: 10, Name: "OpenAI", Platform: PlatformOpenAI,
		Status: StatusActive, Hydrated: true,
	}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{82}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 82, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{10},
	}}}
	capRepo := &modelProtocolCapabilityRepoStub{listManyErr: errors.New("capability repository unavailable")}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	capability := NewModelProtocolCapabilityService(capRepo, accountRepo, groupRepo, nil, cfg)
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, capability, cfg)

	projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "published-model", ModelProtocolOpenAIResponses)
	require.Error(t, err)
	require.Nil(t, projection)
	require.Contains(t, err.Error(), "load model protocol capabilities for enterprise admission")
}

func TestEnterpriseMemberModelDeliveryFailsClosedWhenCompositeEvaluatorIsUnavailable(t *testing.T) {
	t.Parallel()
	svc := NewModelDeliveryService(
		&modelProtocolCatalogAccountRepoStub{},
		&modelProtocolCatalogGroupRepoStub{},
		nil,
		nil,
		&config.Config{},
	)

	projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{{
		ID: 30, Name: "Composite", Platform: PlatformComposite,
		Status: StatusActive, Hydrated: true,
	}}, "published-model", ModelProtocolOpenAIResponses)
	require.Error(t, err)
	require.Nil(t, projection)
	require.Contains(t, err.Error(), "evaluator unavailable")
}

func TestEnterpriseMemberModelDeliveryCompositePreviewUsesPerCandidateTarget(t *testing.T) {
	t.Parallel()
	groups := []*Group{
		{ID: 30, Name: "Composite OpenAI", Platform: PlatformComposite, Status: StatusActive, Hydrated: true},
		{ID: 31, Name: "Composite Anthropic", Platform: PlatformComposite, Status: StatusActive, Hydrated: true},
	}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{301, 311}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{
		{
			ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true, GroupIDs: []int64{30},
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5-upstream"}},
		},
		{
			ID: 311, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true, GroupIDs: []int64{31},
			Credentials: map[string]any{"model_mapping": map[string]any{"claude-sonnet-4-6": "claude-upstream"}},
		},
	}}
	resolver := NewCompositeRouteResolver(compositeRouteRepoStub{routes: []CompositeModelRoute{
		{ID: 1, GroupID: 30, PublicModel: "shared-alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: CompositeRouteEndpointResponses, Enabled: true},
		{ID: 2, GroupID: 31, PublicModel: "shared-alias", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformAnthropic, UpstreamModel: "claude-sonnet-4-6", Endpoint: CompositeRouteEndpointResponses, Enabled: true},
	}})
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})
	svc.SetCompositeRoutePreviewer(resolver)
	ctx := WithCompositeRouteDecision(context.Background(), CompositeRouteDecision{
		Matched:        true,
		Source:         CompositeRouteSourceExplicit,
		GroupID:        999,
		PublicModel:    "stale-public",
		TargetPlatform: PlatformGemini,
		UpstreamModel:  "stale-upstream",
		Endpoint:       CompositeRouteEndpointResponses,
	})

	projection, err := svc.ResolveForEnterpriseMemberRoute(ctx, groups, "shared-alias", ModelProtocolOpenAIResponses)

	require.NoError(t, err)
	require.Zero(t, groupRepo.listCalls)
	openAIProjection := projection.Group("shared-alias", 30)
	require.NotNil(t, openAIProjection)
	require.True(t, enterpriseMemberRouteProjectionSupportsProtocol(openAIProjection, ModelProtocolOpenAIResponses))
	require.Equal(t, PlatformOpenAI, openAIProjection.Routes[0].TargetPlatform)
	require.Equal(t, "gpt-5-upstream", openAIProjection.Routes[0].UpstreamModel)
	anthropicProjection := projection.Group("shared-alias", 31)
	require.NotNil(t, anthropicProjection)
	require.True(t, enterpriseMemberRouteProjectionSupportsProtocol(anthropicProjection, ModelProtocolOpenAIResponses))
	require.Equal(t, PlatformAnthropic, anthropicProjection.Routes[0].TargetPlatform)
	require.Equal(t, "claude-upstream", anthropicProjection.Routes[0].UpstreamModel)
	platform, ok := ResolvedTargetPlatformFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, PlatformGemini, platform, "qualification must not mutate the caller's route decision")
}

func TestEnterpriseMemberModelDeliveryCompositePreviewHonorsEndpointAndDependencyFailures(t *testing.T) {
	t.Parallel()
	group := &Group{ID: 30, Name: "Composite", Platform: PlatformComposite, Status: StatusActive, Hydrated: true}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{301}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{30},
	}}}

	t.Run("endpoint mismatch is not an eligible route", func(t *testing.T) {
		resolver := NewCompositeRouteResolver(compositeRouteRepoStub{
			routes: []CompositeModelRoute{{
				ID: 1, GroupID: 30, PublicModel: "ambiguous-alias", MatchType: CompositeRouteMatchExact,
				TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: CompositeRouteEndpointChatCompletions, Enabled: true,
			}},
		})
		svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})
		svc.SetCompositeRoutePreviewer(resolver)

		projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "ambiguous-alias", ModelProtocolOpenAIResponses)

		require.NoError(t, err)
		groupProjection := projection.Group("ambiguous-alias", 30)
		require.NotNil(t, groupProjection)
		require.False(t, enterpriseMemberRouteProjectionSupportsProtocol(groupProjection, ModelProtocolOpenAIResponses))
	})

	t.Run("repository failure is a qualification dependency error", func(t *testing.T) {
		wantErr := errors.New("composite repository unavailable")
		resolver := NewCompositeRouteResolver(compositeRouteRepoStub{err: wantErr})
		svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})
		svc.SetCompositeRoutePreviewer(resolver)

		projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "ambiguous-alias", ModelProtocolOpenAIResponses)

		require.ErrorIs(t, err, wantErr)
		require.Nil(t, projection)
	})

	t.Run("missing target platform pool stays a persistent pool rejection", func(t *testing.T) {
		resolver := NewCompositeRouteResolver(compositeRouteRepoStub{
			routes: []CompositeModelRoute{{
				ID: 2, GroupID: 30, PublicModel: "target-without-pool", MatchType: CompositeRouteMatchExact,
				TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-5", Endpoint: CompositeRouteEndpointResponses, Enabled: true,
			}},
		})
		wrongPlatformAccounts := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
			ID: 302, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true, GroupIDs: []int64{30},
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5"}},
		}}}
		svc := NewModelDeliveryService(wrongPlatformAccounts, groupRepo, nil, nil, &config.Config{})
		svc.SetCompositeRoutePreviewer(resolver)

		projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "target-without-pool", ModelProtocolOpenAIResponses)

		require.NoError(t, err)
		decision := projection.Group("target-without-pool", 30).Decisions[ModelProtocolOpenAIResponses]
		require.Contains(t, decision.ReasonCodes, ModelDeliveryReasonNoStableRoute)
		require.NotContains(t, decision.ReasonCodes, ModelDeliveryReasonPlatformMismatch)
	})
}

func TestEnterpriseMemberModelDeliveryCompositePreviewRejectsDetectorOnlyMatch(t *testing.T) {
	t.Parallel()
	group := &Group{ID: 30, Name: "Composite", Platform: PlatformComposite, Status: StatusActive, Hydrated: true}
	groupRepo := &modelProtocolCatalogGroupRepoStub{accountIDs: []int64{301}}
	accountRepo := &modelProtocolCatalogAccountRepoStub{accounts: []*Account{{
		ID: 301, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, GroupIDs: []int64{30},
		Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-terra": "gpt-5.6-terra"}},
	}}}
	svc := NewModelDeliveryService(accountRepo, groupRepo, nil, nil, &config.Config{})
	svc.SetCompositeRoutePreviewer(NewCompositeRouteResolver(compositeRouteRepoStub{}))

	projection, err := svc.ResolveForEnterpriseMemberRoute(context.Background(), []*Group{group}, "gpt-5.6-terra", ModelProtocolOpenAIResponses)

	require.NoError(t, err)
	groupProjection := projection.Group("gpt-5.6-terra", 30)
	require.NotNil(t, groupProjection)
	require.False(t, enterpriseMemberRouteProjectionSupportsProtocol(groupProjection, ModelProtocolOpenAIResponses))
	require.Contains(t, groupProjection.Decisions[ModelProtocolOpenAIResponses].ReasonCodes, ModelDeliveryReasonCapabilityUnsupported)
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
