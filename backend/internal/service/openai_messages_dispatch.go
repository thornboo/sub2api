package service

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

const (
	OpenAIMessagesDispatchFamilyMappingModePassthrough = "passthrough"
	OpenAIMessagesDispatchFamilyMappingModeCustom      = "custom"

	defaultOpenAIMessagesDispatchOpusMappedModel   = "gpt-5.4"
	defaultOpenAIMessagesDispatchSonnetMappedModel = "gpt-5.3-codex"
	defaultOpenAIMessagesDispatchHaikuMappedModel  = "gpt-5.4-mini"
)

func normalizeOpenAIMessagesDispatchMappedModel(model string) string {
	model = NormalizeOpenAICompatRequestedModel(strings.TrimSpace(model))
	return strings.TrimSpace(model)
}

func normalizeOpenAIMessagesDispatchModelConfig(cfg OpenAIMessagesDispatchModelConfig) OpenAIMessagesDispatchModelConfig {
	out := OpenAIMessagesDispatchModelConfig{
		FamilyMappingMode: strings.ToLower(strings.TrimSpace(cfg.FamilyMappingMode)),
		OpusMappedModel:   normalizeOpenAIMessagesDispatchMappedModel(cfg.OpusMappedModel),
		SonnetMappedModel: normalizeOpenAIMessagesDispatchMappedModel(cfg.SonnetMappedModel),
		HaikuMappedModel:  normalizeOpenAIMessagesDispatchMappedModel(cfg.HaikuMappedModel),
	}

	if len(cfg.ExactModelMappings) > 0 {
		out.ExactModelMappings = make(map[string]string, len(cfg.ExactModelMappings))
		for requestedModel, mappedModel := range cfg.ExactModelMappings {
			requestedModel = strings.TrimSpace(requestedModel)
			mappedModel = normalizeOpenAIMessagesDispatchMappedModel(mappedModel)
			if requestedModel == "" || mappedModel == "" {
				continue
			}
			out.ExactModelMappings[requestedModel] = mappedModel
		}
		if len(out.ExactModelMappings) == 0 {
			out.ExactModelMappings = nil
		}
	}

	if out.FamilyMappingMode == OpenAIMessagesDispatchFamilyMappingModePassthrough {
		out.OpusMappedModel = ""
		out.SonnetMappedModel = ""
		out.HaikuMappedModel = ""
	}

	return out
}

func normalizeNewOpenAIMessagesDispatchModelConfig(cfg OpenAIMessagesDispatchModelConfig) OpenAIMessagesDispatchModelConfig {
	out := normalizeOpenAIMessagesDispatchModelConfig(cfg)
	if out.FamilyMappingMode == "" &&
		out.OpusMappedModel == "" &&
		out.SonnetMappedModel == "" &&
		out.HaikuMappedModel == "" {
		out.FamilyMappingMode = OpenAIMessagesDispatchFamilyMappingModePassthrough
	}
	return out
}

func validateOpenAIMessagesDispatchModelConfig(cfg OpenAIMessagesDispatchModelConfig) error {
	switch strings.ToLower(strings.TrimSpace(cfg.FamilyMappingMode)) {
	case "", OpenAIMessagesDispatchFamilyMappingModePassthrough, OpenAIMessagesDispatchFamilyMappingModeCustom:
		return nil
	default:
		return fmt.Errorf(
			"family_mapping_mode must be %q or %q",
			OpenAIMessagesDispatchFamilyMappingModePassthrough,
			OpenAIMessagesDispatchFamilyMappingModeCustom,
		)
	}
}

func claudeMessagesDispatchFamily(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(normalized, "claude") {
		return ""
	}
	switch {
	case strings.Contains(normalized, "opus"):
		return "opus"
	case strings.Contains(normalized, "sonnet"):
		return "sonnet"
	case strings.Contains(normalized, "haiku"):
		return "haiku"
	default:
		return ""
	}
}

func (g *Group) ResolveMessagesDispatchModel(requestedModel string) string {
	if g == nil {
		return ""
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return ""
	}

	if g.Platform == PlatformGrok {
		if claudeMessagesDispatchFamily(requestedModel) == "" {
			return ""
		}
		opts := xai.RuntimeModelMappingOptions()
		if !opts.EnableCrossClientMap {
			return ""
		}
		return xai.ModelMappingWithOptions(opts)["claude-*"]
	}

	cfg := normalizeOpenAIMessagesDispatchModelConfig(g.MessagesDispatchModelConfig)
	if mappedModel := strings.TrimSpace(cfg.ExactModelMappings[requestedModel]); mappedModel != "" {
		return mappedModel
	}

	mode := strings.TrimSpace(cfg.FamilyMappingMode)
	if mode == OpenAIMessagesDispatchFamilyMappingModePassthrough {
		return ""
	}

	switch claudeMessagesDispatchFamily(requestedModel) {
	case "opus":
		if mappedModel := strings.TrimSpace(cfg.OpusMappedModel); mappedModel != "" {
			return mappedModel
		}
		if mode == OpenAIMessagesDispatchFamilyMappingModeCustom {
			return ""
		}
		return defaultOpenAIMessagesDispatchOpusMappedModel
	case "sonnet":
		if mappedModel := strings.TrimSpace(cfg.SonnetMappedModel); mappedModel != "" {
			return mappedModel
		}
		if mode == OpenAIMessagesDispatchFamilyMappingModeCustom {
			return ""
		}
		return defaultOpenAIMessagesDispatchSonnetMappedModel
	case "haiku":
		if mappedModel := strings.TrimSpace(cfg.HaikuMappedModel); mappedModel != "" {
			return mappedModel
		}
		if mode == OpenAIMessagesDispatchFamilyMappingModeCustom {
			return ""
		}
		return defaultOpenAIMessagesDispatchHaikuMappedModel
	default:
		return ""
	}
}

// ResolveOpenAIMessagesDeliveryModel applies the group-level Messages dispatch
// mapping as part of the same public -> delivery -> account model chain used by
// catalog projection and runtime routing. An explicit channel mapping wins over
// the legacy family fallback because it is the more specific product mapping.
func ResolveOpenAIMessagesDeliveryModel(group *Group, requestedModel string, channelMapping ChannelMappingResult) string {
	if channelMapping.Mapped && strings.TrimSpace(channelMapping.MappedModel) != "" {
		return NormalizeOpenAICompatRequestedModel(strings.TrimSpace(channelMapping.MappedModel))
	}
	if mapped := strings.TrimSpace(group.ResolveMessagesDispatchModel(requestedModel)); mapped != "" {
		return NormalizeOpenAICompatRequestedModel(mapped)
	}
	if mapped := strings.TrimSpace(channelMapping.MappedModel); mapped != "" {
		return NormalizeOpenAICompatRequestedModel(mapped)
	}
	return NormalizeOpenAICompatRequestedModel(strings.TrimSpace(requestedModel))
}

func sanitizeGroupMessagesDispatchFields(g *Group) {
	if g == nil || g.Platform == PlatformOpenAI {
		return
	}
	g.AllowMessagesDispatch = false
	g.DefaultMappedModel = ""
	g.MessagesDispatchModelConfig = OpenAIMessagesDispatchModelConfig{}
}
