package service

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
)

// AccountTestUsesModelMapping reports whether the admin test picker should
// treat account model_mapping as the source of testable models.
func AccountTestUsesModelMapping(account *Account) bool {
	if account == nil {
		return false
	}
	if shouldForwardOpenAIResponsesViaRawChatCompletions(account) {
		return len(account.GetModelMapping()) > 0
	}
	if account.IsOpenAI() && account.IsOpenAIPassthroughEnabled() {
		return false
	}
	return len(account.GetModelMapping()) > 0
}

// ResolveAccountTestModel resolves the upstream model used by the default
// admin account-test path. It is pure and does not contact upstream services.
func ResolveAccountTestModel(account *Account, modelID string) (string, error) {
	if account == nil {
		return "", fmt.Errorf("account is nil")
	}
	requestedModel := strings.TrimSpace(modelID)
	if requestedModel == "" {
		requestedModel = defaultAccountTestModel(account)
	}

	var (
		resolvedModel string
		err           error
	)
	switch {
	case account.IsCNProvider():
		resolvedModel = resolveMappedAccountTestModel(account, requestedModel)
	case account.IsOpenAI():
		resolvedModel, err = resolveOpenAIAccountTestModel(account, requestedModel)
	case account.IsGemini():
		resolvedModel = resolveMappedAccountTestModel(account, requestedModel)
	case account.Platform == PlatformGrok:
		resolvedModel = resolveGrokDefaultAccountTestModel(account, requestedModel)
	case account.Platform == PlatformAntigravity:
		resolvedModel, err = resolveAntigravityAccountTestModel(account, requestedModel)
	case account.Platform == PlatformAnthropic:
		resolvedModel, err = resolveClaudeAccountTestModel(account, requestedModel)
	default:
		resolvedModel = resolveMappedAccountTestModel(account, requestedModel)
	}
	if err != nil {
		return "", err
	}
	return validateResolvedAccountTestModel(resolvedModel)
}

func defaultAccountTestModel(account *Account) string {
	if account == nil {
		return claude.DefaultTestModel
	}
	switch account.Platform {
	case PlatformOpenAI:
		return openai.DefaultTestModel
	case PlatformGemini:
		return geminicli.DefaultTestModel
	case PlatformGrok:
		return grokDefaultResponsesModel
	case PlatformAntigravity:
		return defaultAntigravityTestModel
	case PlatformKimi:
		return "kimi-k2.5"
	case PlatformZhipu:
		return "glm-4.7"
	case PlatformDeepseek:
		return "deepseek-v4-pro"
	default:
		return claude.DefaultTestModel
	}
}

func resolveMappedAccountTestModel(account *Account, requestedModel string) string {
	if AccountTestUsesModelMapping(account) {
		return account.GetMappedModel(requestedModel)
	}
	return requestedModel
}

func resolveClaudeAccountTestModel(account *Account, requestedModel string) (string, error) {
	if account.IsBedrock() {
		modelID, ok := ResolveBedrockModelID(account, requestedModel)
		if !ok {
			return "", fmt.Errorf("unsupported Bedrock model: %s", requestedModel)
		}
		return modelID, nil
	}
	if account.Type == AccountTypeServiceAccount {
		if mappedModel, matched := account.ResolveMappedModel(requestedModel); matched {
			return mappedModel, nil
		}
		return normalizeVertexAnthropicModelID(claude.NormalizeModelID(requestedModel)), nil
	}
	if AccountTestUsesModelMapping(account) {
		return account.GetMappedModel(requestedModel), nil
	}
	if account.Type == AccountTypeOAuth || account.Type == AccountTypeSetupToken {
		return claude.NormalizeModelID(requestedModel), nil
	}
	return requestedModel, nil
}

func resolveOpenAIAccountTestModel(account *Account, requestedModel string) (string, error) {
	resolvedModel := requestedModel
	if AccountTestUsesModelMapping(account) {
		resolvedModel = account.GetMappedModel(requestedModel)
		if _, err := validateResolvedAccountTestModel(resolvedModel); err != nil {
			return "", err
		}
	}
	if IsGPTImageGenerationModel(resolvedModel) {
		return strings.TrimSpace(resolvedModel), nil
	}
	if account.UsesOpenAICodexProtocol() {
		return normalizeOpenAIModelForUpstream(account, resolvedModel), nil
	}
	return strings.TrimSpace(resolvedModel), nil
}

func resolveGrokDefaultAccountTestModel(account *Account, requestedModel string) string {
	resolvedModel := strings.TrimSpace(resolveMappedAccountTestModel(account, requestedModel))
	if isGrokImageGenerationModel(resolvedModel) {
		return NormalizeGrokMediaModelForEndpoint(GrokMediaEndpointImagesGenerations, resolvedModel, false)
	}
	return resolvedModel
}

func resolveAntigravityAccountTestModel(account *Account, requestedModel string) (string, error) {
	mappedModel := mapAntigravityModel(account, requestedModel)
	if mappedModel == "" {
		return "", fmt.Errorf("model %s not in whitelist", requestedModel)
	}
	return mappedModel, nil
}

func validateResolvedAccountTestModel(modelID string) (string, error) {
	if strings.TrimSpace(modelID) == "" {
		return "", fmt.Errorf("account test model resolves to empty upstream model")
	}
	if strings.Contains(modelID, "*") {
		return "", fmt.Errorf("account test model resolves to wildcard upstream model: %s", modelID)
	}
	return modelID, nil
}
