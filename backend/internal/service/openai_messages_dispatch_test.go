package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIMessagesDispatchModelConfig(t *testing.T) {
	t.Parallel()

	cfg := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   " gpt-5.4-high ",
		SonnetMappedModel: "gpt-5.3-codex",
		HaikuMappedModel:  " gpt-5.4-mini-medium ",
		ExactModelMappings: map[string]string{
			" claude-sonnet-4-5-20250929 ": " gpt-5.2-high ",
			"":                             "gpt-5.4",
			"claude-opus-4-6":              " ",
		},
	})

	require.Equal(t, "gpt-5.4", cfg.OpusMappedModel)
	require.Equal(t, "gpt-5.3-codex", cfg.SonnetMappedModel)
	require.Equal(t, "gpt-5.4-mini", cfg.HaikuMappedModel)
	require.Equal(t, map[string]string{
		"claude-sonnet-4-5-20250929": "gpt-5.2",
	}, cfg.ExactModelMappings)
}

func TestGroupResolveMessagesDispatchModel_GrokRequiresCrossClientMapping(t *testing.T) {
	original := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(original) })
	group := &Group{Platform: PlatformGrok}

	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{})
	require.Empty(t, group.ResolveMessagesDispatchModel("claude-sonnet-4-5"))

	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{
		DefaultText:          "grok-build-0.1",
		EnableCrossClientMap: true,
	})
	require.Equal(t, "grok-build-0.1", group.ResolveMessagesDispatchModel("claude-sonnet-4-5"))
	require.Equal(t, "grok-build-0.1", group.ResolveMessagesDispatchModel("claude-opus-4-6"))
	require.Equal(t, "grok-build-0.1", group.ResolveMessagesDispatchModel("claude-haiku-4-5"))
	require.Empty(t, group.ResolveMessagesDispatchModel("grok"))
	require.Empty(t, group.ResolveMessagesDispatchModel("gpt-5.3-codex"))
}

func TestResolveOpenAIMessagesDeliveryModelPrefersExplicitChannelMapping(t *testing.T) {
	t.Parallel()
	group := &Group{
		Platform: PlatformOpenAI,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			SonnetMappedModel: "group-sonnet",
		},
	}

	require.Equal(t, "channel-sonnet", ResolveOpenAIMessagesDeliveryModel(group, "claude-sonnet-4-5", ChannelMappingResult{
		Mapped:      true,
		MappedModel: "channel-sonnet",
	}))
	require.Equal(t, "group-sonnet", ResolveOpenAIMessagesDeliveryModel(group, "claude-sonnet-4-5", ChannelMappingResult{
		MappedModel: "claude-sonnet-4-5",
	}))
	require.Equal(t, "gpt-5.4", ResolveOpenAIMessagesDeliveryModel(group, "gpt-5.4", ChannelMappingResult{}))
}

func TestResolveOpenAIMessagesDeliveryModelSupportsExplicitPassThrough(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform: PlatformOpenAI,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			FamilyMappingMode: OpenAIMessagesDispatchFamilyMappingModePassthrough,
		},
	}

	require.Equal(t, "claude-sonnet-4-5", ResolveOpenAIMessagesDeliveryModel(
		group,
		"claude-sonnet-4-5",
		ChannelMappingResult{},
	))
}

func TestResolveOpenAIMessagesDeliveryModelSupportsPartialCustomFamilyMappings(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform: PlatformOpenAI,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			FamilyMappingMode: OpenAIMessagesDispatchFamilyMappingModeCustom,
			SonnetMappedModel: "minimax-m2.7",
		},
	}

	require.Equal(t, "minimax-m2.7", ResolveOpenAIMessagesDeliveryModel(
		group,
		"claude-sonnet-4-5",
		ChannelMappingResult{},
	))
	require.Equal(t, "claude-opus-4-6", ResolveOpenAIMessagesDeliveryModel(
		group,
		"claude-opus-4-6",
		ChannelMappingResult{},
	))
}

func TestResolveOpenAIMessagesDeliveryModelExactMappingWinsInPassThroughMode(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform: PlatformOpenAI,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			FamilyMappingMode: OpenAIMessagesDispatchFamilyMappingModePassthrough,
			ExactModelMappings: map[string]string{
				"claude-sonnet-4-5": "minimax-m2.7",
			},
		},
	}

	require.Equal(t, "minimax-m2.7", ResolveOpenAIMessagesDeliveryModel(
		group,
		"claude-sonnet-4-5",
		ChannelMappingResult{},
	))
}

func TestResolveOpenAIMessagesDeliveryModelFeedsAccountLogicalMapping(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform: PlatformOpenAI,
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			FamilyMappingMode: OpenAIMessagesDispatchFamilyMappingModeCustom,
			SonnetMappedModel: "minimax-m2.7",
		},
	}
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"minimax-m2.7": "MiniMax-M2.7",
			},
		},
	}

	deliveryModel := ResolveOpenAIMessagesDeliveryModel(
		group,
		"claude-sonnet-4-5",
		ChannelMappingResult{},
	)

	require.Equal(t, "minimax-m2.7", deliveryModel)
	require.True(t, accountSupportsDeliveryModel(account, deliveryModel))
	require.Equal(t, "MiniMax-M2.7", resolveFinalDeliveryModel(account, deliveryModel))
}

func TestValidateOpenAIMessagesDispatchModelConfigRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	require.NoError(t, validateOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{}))
	require.NoError(t, validateOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		FamilyMappingMode: OpenAIMessagesDispatchFamilyMappingModePassthrough,
	}))
	require.Error(t, validateOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		FamilyMappingMode: "automatic",
	}))
}

func TestSanitizeGroupMessagesDispatchFields_ClearsNonOpenAIPlatform(t *testing.T) {
	t.Parallel()

	group := &Group{
		Platform:              PlatformAnthropic,
		AllowMessagesDispatch: true,
		DefaultMappedModel:    "gpt-5.6-sol",
		MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
			SonnetMappedModel: "gpt-5.3-codex",
			ExactModelMappings: map[string]string{
				"claude-fable-5": "gpt-5.6-sol",
			},
		},
	}

	sanitizeGroupMessagesDispatchFields(group)

	require.False(t, group.AllowMessagesDispatch)
	require.Empty(t, group.DefaultMappedModel)
	require.Equal(t, OpenAIMessagesDispatchModelConfig{}, group.MessagesDispatchModelConfig)
}
