//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotGroupRoundtripMatchesMemberGroupRoundtrip(t *testing.T) {
	group := authSnapshotCompleteGroupFixture()
	apiKey := &APIKey{
		ID: 1, UserID: 2, Status: StatusActive,
		User:  &User{ID: 2, Status: StatusActive, Role: RoleUser},
		Group: group,
		Member: &EnterpriseMember{
			ID:               10,
			EnterpriseUserID: 2,
			Status:           EnterpriseMemberStatusActive,
			Groups:           []Group{*group},
		},
	}
	svc := &APIKeyService{}

	snapshot := svc.snapshotFromAPIKey(context.Background(), apiKey)
	materialized := svc.snapshotToAPIKey("member-key", snapshot)

	require.NotNil(t, materialized.Group)
	require.Len(t, materialized.Member.Groups, 1)
	requireAuthSnapshotGroupFields(t, group, materialized.Group)
	requireAuthSnapshotGroupFields(t, group, &materialized.Member.Groups[0])
}

func TestAPIKeyServiceGetByKeyMemberDBFallbackPreservesLongContextPricing(t *testing.T) {
	memberID := int64(44)
	group := authSnapshotCompleteGroupFixture()
	repo := &authRepoStub{getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
		return &APIKey{
			ID:       5,
			UserID:   7,
			MemberID: &memberID,
			Status:   StatusActive,
			User: &User{
				ID:          7,
				Status:      StatusActive,
				Role:        RoleUser,
				AccountType: UserAccountTypeEnterprise,
				Balance:     12,
				Concurrency: 2,
			},
			Member: &EnterpriseMember{
				ID:               memberID,
				EnterpriseUserID: 7,
				Status:           EnterpriseMemberStatusActive,
				Groups:           []Group{*group},
			},
		}, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})

	got, err := svc.GetByKey(context.Background(), "member-key")
	require.NoError(t, err)
	require.NotNil(t, got.Member)
	require.Len(t, got.Member.Groups, 1)
	require.True(t, got.Member.Groups[0].LongContextPricingEnabled)
	require.Equal(t, group.ModelPricing, got.Member.Groups[0].ModelPricing)
	require.Nil(t, got.Group)
}

func TestAPIKeyAuthMemberGroupLongContextPricingBillsHighTier(t *testing.T) {
	group := authSnapshotOpenAIGroupWithAstraPricing()
	memberID := int64(44)
	apiKey := &APIKey{
		ID: 5, UserID: 7, MemberID: &memberID, Status: StatusActive,
		User: &User{ID: 7, Status: StatusActive, Role: RoleUser, AccountType: UserAccountTypeEnterprise},
		Member: &EnterpriseMember{
			ID:               memberID,
			EnterpriseUserID: 7,
			Status:           EnterpriseMemberStatusActive,
			Groups:           []Group{*group},
		},
	}
	svc := &APIKeyService{}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), apiKey)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))

	materialized, used, err := svc.applyAuthCacheEntry("member-key", &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.Len(t, materialized.Member.Groups, 1)
	activeGroup := materialized.Member.Groups[0]
	require.True(t, activeGroup.LongContextPricingEnabled)

	activated := *materialized
	activated.GroupID = &activeGroup.ID
	activated.Group = &activeGroup

	cost := authSnapshotAstraCost(t, &activated)
	require.InDelta(t, 13.267359, cost.ActualCost, 1e-9)
	require.InDelta(t, 13.267359, cost.TotalCost, 1e-9)
}

func TestCalculateTokenCostUsesWholeInputAndCacheContextForChannelIntervals(t *testing.T) {
	group := authSnapshotOpenAIGroupWithAstraPricing()
	apiKey := &APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group}

	tests := []struct {
		name       string
		input      int
		cacheRead  int
		wantActual float64
	}{
		{name: "below threshold", input: 271999, wantActual: 2.71999},
		{name: "at threshold", input: 272000, wantActual: 2.72},
		{name: "above threshold", input: 272001, wantActual: 5.44002},
		{name: "production usage includes cache in context and cost", input: 659468, cacheRead: 3712, wantActual: 13.267359},
		{name: "cache pushes context above threshold", input: 271999, cacheRead: 2, wantActual: 5.439984},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := authSnapshotAstraCostWithTokens(t, apiKey, UsageTokens{
				InputTokens:     tt.input,
				OutputTokens:    boolInt(tt.name == "production usage includes cache in context and cost", 941),
				CacheReadTokens: tt.cacheRead,
			})
			require.InDelta(t, tt.wantActual, cost.ActualCost, 1e-9)
		})
	}
}

func TestAPIKeyAuthMemberRecordUsageBillsLongContextThroughSettlement(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "non_stream", true: "stream"}[stream], func(t *testing.T) {
			apiKey := authSnapshotLoadAndActivateMemberKey(t)
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true, UsageLogPersisted: true}}
			userRepo := &openAIRecordUsageUserRepoStub{}
			subRepo := &openAIRecordUsageSubRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, userRepo, subRepo, &openAIUserGroupRateRepoStub{})
			svc.billingService = NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{}})
			svc.channelService = newChannelServiceWithPricings(*apiKey.GroupID, []ChannelModelPricing{authSnapshotAstraChannelPricing()})
			svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)

			ctx := context.WithValue(context.Background(), ctxkey.ActiveGroup, &ActiveGroupContext{
				LogicalRequestID: "client:e6d3deb3-a05b-40b6-8eab-1e655d8e174c",
				AttemptID:        "client:e6d3deb3-a05b-40b6-8eab-1e655d8e174c:g3:a1",
				MemberID:         *apiKey.MemberID,
				MemberVersion:    apiKey.Member.Version,
				GroupID:          *apiKey.GroupID,
				Platform:         apiKey.Group.Platform,
				RateMultiplier:   apiKey.Group.RateMultiplier,
				SubscriptionType: apiKey.Group.SubscriptionType,
				Endpoint:         "/v1/responses",
				RequestedModel:   "gpt-6-astra",
				MappedModel:      "gpt-6-astra",
				CandidateIndex:   0,
				AttemptNumber:    1,
			})

			err := svc.RecordUsage(ctx, &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{
					RequestID: "resp_member_astra_long_context",
					Usage: OpenAIUsage{
						// OpenAI reports total input including cache; RecordUsage
						// stores uncached input by subtracting cache tokens.
						InputTokens:          663180,
						OutputTokens:         941,
						CacheReadInputTokens: 3712,
					},
					Model:    "gpt-6-astra",
					Stream:   stream,
					Duration: time.Second,
				},
				OriginalModel:      "gpt-6-astra",
				ChannelMappedModel: "gpt-6-astra",
				InboundEndpoint:    "/v1/responses",
				UpstreamEndpoint:   "/v1/responses",
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            &Account{ID: 34, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
				PricingAt:          time.Date(2026, 9, 12, 13, 27, 24, 0, time.UTC),
			})

			require.NoError(t, err)
			require.Zero(t, usageRepo.calls, "enterprise member usage is persisted atomically by the billing command")
			require.Equal(t, 1, billingRepo.calls)
			require.NotNil(t, billingRepo.lastCmd)
			require.NotNil(t, billingRepo.lastCmd.UsageLog)
			require.NotNil(t, billingRepo.lastCmd.MemberID)
			require.Equal(t, *apiKey.MemberID, *billingRepo.lastCmd.MemberID)
			require.InDelta(t, 13.267359, billingRepo.lastCmd.MemberBudgetCost, 1e-9)
			require.InDelta(t, 13.267359, billingRepo.lastCmd.BalanceCost, 1e-9)

			log := billingRepo.lastCmd.UsageLog
			require.Equal(t, "gpt-6-astra", log.Model)
			require.Equal(t, "gpt-6-astra", log.RequestedModel)
			require.NotNil(t, log.GroupID)
			require.Equal(t, int64(3), *log.GroupID)
			require.Equal(t, 659468, log.InputTokens)
			require.Equal(t, 941, log.OutputTokens)
			require.Equal(t, 3712, log.CacheReadTokens)
			require.InDelta(t, 13.18936, log.InputCost, 1e-9)
			require.InDelta(t, 0.070575, log.OutputCost, 1e-9)
			require.InDelta(t, 0.007424, log.CacheReadCost, 1e-9)
			require.InDelta(t, 13.267359, log.TotalCost, 1e-9)
			require.InDelta(t, 13.267359, log.ActualCost, 1e-9)
			require.True(t, log.LongContextBillingApplied)
			require.True(t, log.Stream == stream)
			require.Equal(t, billingRepo.lastCmd.InputTokens, log.InputTokens)
			require.Equal(t, billingRepo.lastCmd.OutputTokens, log.OutputTokens)
			require.Equal(t, billingRepo.lastCmd.CacheReadTokens, log.CacheReadTokens)
		})
	}
}

func authSnapshotCompleteGroupFixture() *Group {
	daily := 10.0
	weekly := 20.0
	monthly := 30.0
	image1K := 0.01
	image2K := 0.02
	image4K := 0.04
	video480 := 0.1
	video720 := 0.2
	video1080 := 0.3
	webSearch := 0.01
	search := 3.0
	audioRealtime := 0.5
	audioTTS := 1.2
	audioSTT := 2.3
	fallbackID := int64(60)
	invalidFallbackID := int64(61)
	inputPrice := 1e-6
	outputPrice := 2e-6
	cacheWritePrice := 1.25e-6
	cacheReadPrice := 0.1e-6
	return &Group{
		ID:                              50,
		Name:                            "complete-group",
		Platform:                        PlatformOpenAI,
		IsExclusive:                     true,
		Status:                          StatusActive,
		SubscriptionType:                SubscriptionTypeSubscription,
		RateMultiplier:                  1.7,
		DailyLimitUSD:                   &daily,
		WeeklyLimitUSD:                  &weekly,
		MonthlyLimitUSD:                 &monthly,
		AllowImageGeneration:            true,
		AllowBatchImageGeneration:       true,
		ImageRateIndependent:            true,
		ImageRateMultiplier:             0.8,
		ImagePrice1K:                    &image1K,
		ImagePrice2K:                    &image2K,
		ImagePrice4K:                    &image4K,
		VideoRateIndependent:            true,
		VideoRateMultiplier:             0.9,
		VideoPrice480P:                  &video480,
		VideoPrice720P:                  &video720,
		VideoPrice1080P:                 &video1080,
		VideoModelPrices:                map[string]map[string]float64{"veo": {"720p": 0.42}},
		WebSearchPricePerCall:           &webSearch,
		SearchPricePer1k:                &search,
		AudioRealtimePricePerMin:        &audioRealtime,
		AudioTTSPricePerMillionChars:    &audioTTS,
		AudioSTTPricePerHour:            &audioSTT,
		LongContextPricingEnabled:       true,
		ModelPricing:                    []ChannelModelPricing{{Models: []string{"gpt-6-astra"}, BillingMode: BillingModeToken, InputPrice: &inputPrice, OutputPrice: &outputPrice, CacheWritePrice: &cacheWritePrice, CacheReadPrice: &cacheReadPrice}},
		ClaudeCodeOnly:                  true,
		FallbackGroupID:                 &fallbackID,
		FallbackGroupIDOnInvalidRequest: &invalidFallbackID,
		ModelRouting:                    map[string][]int64{"gpt-*": {1, 2}},
		ModelRoutingEnabled:             true,
		MCPXMLInject:                    true,
		SupportedModelScopes:            []string{"claude", "gemini_text"},
		AllowMessagesDispatch:           true,
		AllowLive:                       true,
		ForceOpenAIFast:                 true,
		FreeOpenAIFast:                  true,
		DefaultMappedModel:              "gpt-6-astra",
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			FamilyMappingMode: OpenAIMessagesDispatchFamilyMappingModeCustom,
			SonnetMappedModel: "gpt-6-astra",
			ExactModelMappings: map[string]string{
				"claude-sonnet-4.5": "gpt-6-astra",
			},
		},
		ModelAllowlist:              GroupModelAllowlist{Enabled: true, Models: []string{"gpt-6-astra"}},
		CodexModelsManifestConfig:   GroupCodexModelsManifestConfig{Enabled: true, AccountIDs: []int64{10}},
		RPMLimit:                    123,
		MaxReasoningEffort:          "medium",
		MaxReasoningEffortOverLimit: ReasoningEffortOverLimitDeny,
		ReasoningEffortMappings:     []ReasoningEffortMapping{{From: "max", To: "xhigh"}},
		PeakRateEnabled:             true,
		PeakStart:                   "09:00",
		PeakEnd:                     "10:00",
		PeakRateMultiplier:          1.5,
		ProfitControlEnabled:        true,
		ProfitMinMargin:             0.2,
		ProfitSafetyBuffer:          0.05,
	}
}

func requireAuthSnapshotGroupFields(t *testing.T, want, got *Group) {
	t.Helper()
	require.Equal(t, want.ID, got.ID)
	require.Equal(t, want.Name, got.Name)
	require.Equal(t, want.Platform, got.Platform)
	require.Equal(t, want.IsExclusive, got.IsExclusive)
	require.Equal(t, want.Status, got.Status)
	require.True(t, got.Hydrated)
	require.Equal(t, want.SubscriptionType, got.SubscriptionType)
	require.Equal(t, want.RateMultiplier, got.RateMultiplier)
	require.Equal(t, want.DailyLimitUSD, got.DailyLimitUSD)
	require.Equal(t, want.WeeklyLimitUSD, got.WeeklyLimitUSD)
	require.Equal(t, want.MonthlyLimitUSD, got.MonthlyLimitUSD)
	require.Equal(t, want.AllowImageGeneration, got.AllowImageGeneration)
	require.Equal(t, want.AllowBatchImageGeneration, got.AllowBatchImageGeneration)
	require.Equal(t, want.ImageRateIndependent, got.ImageRateIndependent)
	require.Equal(t, want.ImageRateMultiplier, got.ImageRateMultiplier)
	require.Equal(t, want.ImagePrice1K, got.ImagePrice1K)
	require.Equal(t, want.ImagePrice2K, got.ImagePrice2K)
	require.Equal(t, want.ImagePrice4K, got.ImagePrice4K)
	require.Equal(t, want.VideoRateIndependent, got.VideoRateIndependent)
	require.Equal(t, want.VideoRateMultiplier, got.VideoRateMultiplier)
	require.Equal(t, want.VideoPrice480P, got.VideoPrice480P)
	require.Equal(t, want.VideoPrice720P, got.VideoPrice720P)
	require.Equal(t, want.VideoPrice1080P, got.VideoPrice1080P)
	require.Equal(t, NormalizeVideoModelPrices(want.VideoModelPrices), got.VideoModelPrices)
	require.Equal(t, want.WebSearchPricePerCall, got.WebSearchPricePerCall)
	require.Equal(t, want.SearchPricePer1k, got.SearchPricePer1k)
	require.Equal(t, want.AudioRealtimePricePerMin, got.AudioRealtimePricePerMin)
	require.Equal(t, want.AudioTTSPricePerMillionChars, got.AudioTTSPricePerMillionChars)
	require.Equal(t, want.AudioSTTPricePerHour, got.AudioSTTPricePerHour)
	require.Equal(t, want.LongContextPricingEnabled, got.LongContextPricingEnabled)
	require.Equal(t, want.ModelPricing, got.ModelPricing)
	require.Equal(t, want.ClaudeCodeOnly, got.ClaudeCodeOnly)
	require.Equal(t, want.FallbackGroupID, got.FallbackGroupID)
	require.Equal(t, want.FallbackGroupIDOnInvalidRequest, got.FallbackGroupIDOnInvalidRequest)
	require.Equal(t, want.ModelRouting, got.ModelRouting)
	require.Equal(t, want.ModelRoutingEnabled, got.ModelRoutingEnabled)
	require.Equal(t, want.MCPXMLInject, got.MCPXMLInject)
	require.Equal(t, want.SupportedModelScopes, got.SupportedModelScopes)
	require.Equal(t, want.AllowMessagesDispatch, got.AllowMessagesDispatch)
	require.Equal(t, want.AllowLive, got.AllowLive)
	require.Equal(t, want.ForceOpenAIFast, got.ForceOpenAIFast)
	require.Equal(t, want.FreeOpenAIFast, got.FreeOpenAIFast)
	require.Equal(t, want.DefaultMappedModel, got.DefaultMappedModel)
	require.Equal(t, want.MessagesDispatchModelConfig, got.MessagesDispatchModelConfig)
	require.Equal(t, want.ModelAllowlist, got.ModelAllowlist)
	require.Equal(t, want.CodexModelsManifestConfig, got.CodexModelsManifestConfig)
	require.Equal(t, want.RPMLimit, got.RPMLimit)
	require.Equal(t, want.MaxReasoningEffort, got.MaxReasoningEffort)
	require.Equal(t, want.MaxReasoningEffortOverLimit, got.MaxReasoningEffortOverLimit)
	require.Equal(t, want.ReasoningEffortMappings, got.ReasoningEffortMappings)
	require.Equal(t, want.PeakRateEnabled, got.PeakRateEnabled)
	require.Equal(t, want.PeakStart, got.PeakStart)
	require.Equal(t, want.PeakEnd, got.PeakEnd)
	require.Equal(t, want.PeakRateMultiplier, got.PeakRateMultiplier)
	require.Equal(t, want.ProfitControlEnabled, got.ProfitControlEnabled)
	require.Equal(t, want.ProfitMinMargin, got.ProfitMinMargin)
	require.Equal(t, want.ProfitSafetyBuffer, got.ProfitSafetyBuffer)
}

func authSnapshotLoadAndActivateMemberKey(t *testing.T) *APIKey {
	t.Helper()
	memberID := int64(113)
	repo := &authRepoStub{getByKeyForAuth: func(ctx context.Context, key string) (*APIKey, error) {
		return &APIKey{
			ID:       55,
			UserID:   4,
			MemberID: &memberID,
			Status:   StatusActive,
			User: &User{
				ID:          4,
				Status:      StatusActive,
				Role:        RoleUser,
				AccountType: UserAccountTypeEnterprise,
				Balance:     1000,
				Concurrency: 2,
			},
			Member: &EnterpriseMember{
				ID:               memberID,
				EnterpriseUserID: 4,
				MemberCode:       "soleapi-codex",
				Name:             "soleapi-codex",
				Status:           EnterpriseMemberStatusActive,
				Version:          7,
				Groups:           []Group{*authSnapshotOpenAIGroupWithAstraPricing()},
			},
		}, nil
	}}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})
	apiKey, err := svc.GetByKey(context.Background(), "member-key")
	require.NoError(t, err)
	require.NotNil(t, apiKey.Member)
	require.Len(t, apiKey.Member.Groups, 1)
	activeGroup := apiKey.Member.Groups[0]
	require.True(t, activeGroup.LongContextPricingEnabled)

	activated := *apiKey
	activated.GroupID = &activeGroup.ID
	activated.Group = &activeGroup
	member := *apiKey.Member
	member.Groups = []Group{activeGroup}
	member.GroupIDs = []int64{activeGroup.ID}
	activated.Member = &member
	return &activated
}

func authSnapshotOpenAIGroupWithAstraPricing() *Group {
	return &Group{
		ID:                        3,
		Name:                      "openai",
		Platform:                  PlatformOpenAI,
		Status:                    StatusActive,
		SubscriptionType:          SubscriptionTypeStandard,
		RateMultiplier:            1,
		LongContextPricingEnabled: true,
		ModelPricing:              nil,
	}
}

func authSnapshotAstraCost(t *testing.T, apiKey *APIKey) *CostBreakdown {
	t.Helper()
	return authSnapshotAstraCostWithTokens(t, apiKey, UsageTokens{
		InputTokens:     659468,
		OutputTokens:    941,
		CacheReadTokens: 3712,
	})
}

func authSnapshotAstraCostWithTokens(t *testing.T, apiKey *APIKey, tokens UsageTokens) *CostBreakdown {
	t.Helper()
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{}})
	resolver := NewModelPricingResolver(newChannelServiceWithPricings(apiKey.Group.ID, []ChannelModelPricing{authSnapshotAstraChannelPricing()}), billing)
	cost, err := billing.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "gpt-6-astra",
		GroupID:        apiKey.GroupID,
		Group:          apiKey.Group,
		Tokens:         tokens,
		RateMultiplier: apiKey.Group.RateMultiplier,
		PricingAt:      time.Date(2026, 9, 12, 13, 27, 24, 0, time.UTC),
		Resolver:       resolver,
	})
	require.NoError(t, err)
	return cost
}

func authSnapshotAstraChannelPricing() ChannelModelPricing {
	firstMax := 272000
	input1 := 10.0 / 1e6
	output1 := 50.0 / 1e6
	cacheWrite1 := 12.5 / 1e6
	cacheRead1 := 1.0 / 1e6
	input2 := 20.0 / 1e6
	output2 := 75.0 / 1e6
	cacheWrite2 := 25.0 / 1e6
	cacheRead2 := 2.0 / 1e6
	return ChannelModelPricing{
		Platform:        PlatformOpenAI,
		Models:          []string{"gpt-6-astra"},
		BillingMode:     BillingModeToken,
		InputPrice:      &input1,
		OutputPrice:     &output1,
		CacheWritePrice: &cacheWrite1,
		CacheReadPrice:  &cacheRead1,
		Intervals: []PricingInterval{
			{MinTokens: 0, MaxTokens: &firstMax, InputPrice: &input1, OutputPrice: &output1, CacheWritePrice: &cacheWrite1, CacheReadPrice: &cacheRead1},
			{MinTokens: 272000, InputPrice: &input2, OutputPrice: &output2, CacheWritePrice: &cacheWrite2, CacheReadPrice: &cacheRead2},
		},
	}
}

func boolInt(ok bool, value int) int {
	if ok {
		return value
	}
	return 0
}
