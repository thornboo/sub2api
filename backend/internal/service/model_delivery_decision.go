package service

import (
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
)

// ModelDeliveryReasonCode is a stable administrator-facing explanation for a
// protocol decision. User-facing catalog DTOs must not expose these reasons or
// any account topology.
type ModelDeliveryReasonCode string

const (
	ModelDeliveryReasonNoStableRoute               ModelDeliveryReasonCode = "no_stable_route"
	ModelDeliveryReasonPlatformMismatch            ModelDeliveryReasonCode = "platform_mismatch"
	ModelDeliveryReasonModelUnsupported            ModelDeliveryReasonCode = "model_unsupported"
	ModelDeliveryReasonGroupProtocolDisabled       ModelDeliveryReasonCode = "group_protocol_disabled"
	ModelDeliveryReasonGlobalRoutingDisabled       ModelDeliveryReasonCode = "global_routing_disabled"
	ModelDeliveryReasonAccountTransportUnavailable ModelDeliveryReasonCode = "account_transport_unavailable"
	ModelDeliveryReasonCapabilityUnknown           ModelDeliveryReasonCode = "protocol_capability_unknown"
	ModelDeliveryReasonCapabilityUnsupported       ModelDeliveryReasonCode = "protocol_capability_unsupported"
)

// ModelDeliveryDecision is the canonical stable decision for one account,
// model and public protocol. It deliberately excludes transient concurrency,
// cooldown and rate-limit state; the scheduler still owns those concerns.
type ModelDeliveryDecision struct {
	Eligible           bool
	ReasonCodes        []ModelDeliveryReasonCode
	PublicModel        string
	ChannelMappedModel string
	UpstreamModel      string
	InboundProtocol    ModelProtocol
	UpstreamProtocol   ModelProtocol
	Mode               ModelDeliveryMode
	CapabilityState    ModelProtocolState
	CapabilitySource   string
}

type ModelDeliveryEvaluationPurpose string

const (
	ModelDeliveryEvaluationPurposeCatalog             ModelDeliveryEvaluationPurpose = "catalog"
	ModelDeliveryEvaluationPurposeEnterpriseAdmission ModelDeliveryEvaluationPurpose = "enterprise_admission"
)

type ModelDeliveryCandidateInput struct {
	Account               *Account
	PublicModel           string
	ChannelMappedModel    string
	GroupPlatform         string
	AllowMessagesDispatch bool
	InboundProtocol       ModelProtocol
	NativeRoutingEnabled  bool
	Capabilities          []AccountModelProtocolCapability
	Purpose               ModelDeliveryEvaluationPurpose
}

// EvaluateModelDeliveryCandidate is the single stable policy boundary shared
// by catalog projection and runtime candidate filtering. Execution code may
// still choose among eligible routes using live scheduler state.
func EvaluateModelDeliveryCandidate(input ModelDeliveryCandidateInput) ModelDeliveryDecision {
	decision := ModelDeliveryDecision{
		PublicModel:        strings.TrimSpace(input.PublicModel),
		ChannelMappedModel: strings.TrimSpace(input.ChannelMappedModel),
		InboundProtocol:    input.InboundProtocol,
		CapabilityState:    ModelProtocolStateUnknown,
	}
	account := input.Account
	if account == nil || !isStableDeliveryAccount(account) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonNoStableRoute)
	}
	if strings.TrimSpace(input.GroupPlatform) == "" || account.Platform != input.GroupPlatform {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
	}
	if decision.ChannelMappedModel == "" {
		decision.ChannelMappedModel = decision.PublicModel
	}
	if !accountSupportsDeliveryModel(account, decision.ChannelMappedModel) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonModelUnsupported)
	}
	decision.UpstreamModel = resolveFinalDeliveryModel(account, decision.ChannelMappedModel)

	if isEnterpriseNonTextAdmissionProtocol(input.InboundProtocol) {
		return evaluateNonTextDeliveryCandidate(input, decision)
	}
	if input.InboundProtocol == ModelProtocolAnthropicMessages {
		return evaluateMessagesDeliveryCandidate(input, decision)
	}
	if input.InboundProtocol != ModelProtocolOpenAIChat && input.InboundProtocol != ModelProtocolOpenAIResponses {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonCapabilityUnsupported)
	}
	enterpriseAdmission := input.Purpose == ModelDeliveryEvaluationPurposeEnterpriseAdmission
	if !input.NativeRoutingEnabled && !enterpriseAdmission {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonGlobalRoutingDisabled)
	}
	if enterpriseAdmission && account.IsGrok() {
		if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions) {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
		}
		decision.Eligible = true
		decision.UpstreamProtocol = ModelProtocolOpenAIResponses
		decision.Mode = ModelDeliveryModeCompatibility
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "existing_grok_gateway_contract"
		return decision
	}
	if enterpriseAdmission && supportsEstablishedGatewayTextContract(account.Platform) {
		decision.Eligible = true
		decision.Mode = ModelDeliveryModeCompatibility
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "existing_gateway_contract"
		if account.Platform == PlatformAnthropic {
			decision.UpstreamProtocol = ModelProtocolAnthropicMessages
		}
		return decision
	}
	if !account.IsOpenAI() {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
	}
	if strictOpenAIAPIKeyProtocolRouting(input) {
		decision.UpstreamProtocol = decision.InboundProtocol
		decision.CapabilityState, decision.CapabilitySource = resolveCapabilityFromItems(
			input.Capabilities,
			decision.UpstreamModel,
			decision.InboundProtocol,
			false,
		)
		if !accountSupportsStrictOpenAIProtocolTransport(account, decision.UpstreamProtocol) {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
		}
		if decision.CapabilityState != ModelProtocolStateSupported {
			return blockForCapabilityState(decision)
		}
		decision.Eligible = true
		decision.Mode = ModelDeliveryModeNative
		return decision
	}

	decision.UpstreamProtocol, decision.CapabilityState, decision.CapabilitySource = selectOpenAIUpstreamProtocolForModel(
		account,
		decision.UpstreamModel,
		input.Capabilities,
	)
	if !accountSupportsOpenAITransport(account, decision.UpstreamProtocol) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
	}
	if decision.CapabilityState != ModelProtocolStateSupported {
		if enterpriseAdmission && decision.CapabilityState == ModelProtocolStateUnknown {
			decision.Eligible = true
			decision.Mode = ModelDeliveryModeCompatibility
			decision.CapabilitySource = "existing_gateway_contract"
			return decision
		}
		return blockForCapabilityState(decision)
	}
	decision.Eligible = true
	decision.Mode = ModelDeliveryModeCompatibility
	if decision.InboundProtocol == decision.UpstreamProtocol {
		decision.Mode = ModelDeliveryModeNative
	}
	return decision
}

func isEnterpriseNonTextAdmissionProtocol(protocol ModelProtocol) bool {
	switch protocol {
	case ModelProtocolOpenAIEmbeddings,
		ModelProtocolOpenAIImages,
		ModelProtocolOpenAILive,
		ModelProtocolBatchImages,
		ModelProtocolGrokVideo,
		ModelProtocolGeminiNative:
		return true
	default:
		return false
	}
}

func evaluateNonTextDeliveryCandidate(input ModelDeliveryCandidateInput, decision ModelDeliveryDecision) ModelDeliveryDecision {
	account := input.Account
	requiredCapability, requiresOpenAICapability := openAIEndpointCapabilityForModelProtocol(input.InboundProtocol)
	switch input.InboundProtocol {
	case ModelProtocolOpenAIEmbeddings:
		if input.GroupPlatform != PlatformOpenAI || account.Platform != PlatformOpenAI {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if !account.SupportsOpenAIEndpointCapability(requiredCapability) {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
		}
		decision.UpstreamProtocol = ModelProtocolOpenAIEmbeddings
		decision.Mode = ModelDeliveryModeNative
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "openai_endpoint_capability"
	case ModelProtocolOpenAIImages:
		if input.GroupPlatform != PlatformOpenAI && input.GroupPlatform != PlatformGrok {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if account.Platform != input.GroupPlatform {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if input.GroupPlatform == PlatformGrok {
			if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration) {
				return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
			}
			decision.CapabilitySource = "grok_media_generation_capability"
		} else {
			decision.CapabilitySource = "existing_openai_images_contract"
		}
		decision.UpstreamProtocol = ModelProtocolOpenAIImages
		decision.Mode = ModelDeliveryModeNative
		decision.CapabilityState = ModelProtocolStateSupported
	case ModelProtocolOpenAILive:
		if input.GroupPlatform != PlatformOpenAI || account.Platform != PlatformOpenAI {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if !account.SupportsOpenAIEndpointCapability(requiredCapability) {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
		}
		decision.UpstreamProtocol = ModelProtocolOpenAILive
		decision.Mode = ModelDeliveryModeNative
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "openai_endpoint_capability"
	case ModelProtocolBatchImages:
		if input.GroupPlatform != PlatformGemini {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if account.Platform != PlatformGemini {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		decision.UpstreamProtocol = ModelProtocolBatchImages
		decision.Mode = ModelDeliveryModeNative
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "batch_image_group_platform_contract"
	case ModelProtocolGrokVideo:
		if input.GroupPlatform != PlatformGrok || account.Platform != PlatformGrok {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityGrokMediaGeneration) {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
		}
		decision.UpstreamProtocol = ModelProtocolGrokVideo
		decision.Mode = ModelDeliveryModeNative
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "grok_media_generation_capability"
	case ModelProtocolGeminiNative:
		if input.GroupPlatform != PlatformGemini && input.GroupPlatform != PlatformAntigravity {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		if account.Platform != input.GroupPlatform {
			return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
		}
		decision.UpstreamProtocol = ModelProtocolGeminiNative
		decision.Mode = ModelDeliveryModeNative
		decision.CapabilityState = ModelProtocolStateSupported
		decision.CapabilitySource = "gemini_native_group_platform_contract"
	default:
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonCapabilityUnsupported)
	}
	if requiresOpenAICapability && decision.CapabilityState != ModelProtocolStateSupported {
		return blockForCapabilityState(decision)
	}
	decision.Eligible = true
	return decision
}

func evaluateMessagesDeliveryCandidate(input ModelDeliveryCandidateInput, decision ModelDeliveryDecision) ModelDeliveryDecision {
	account := input.Account
	if !input.AllowMessagesDispatch && (input.GroupPlatform == PlatformOpenAI || input.GroupPlatform == PlatformGrok) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonGroupProtocolDisabled)
	}
	if !account.IsOpenAI() {
		decision.Eligible = true
		decision.UpstreamProtocol = ModelProtocolAnthropicMessages
		decision.Mode = ModelDeliveryModeCompatibility
		if account.Platform == PlatformAnthropic {
			decision.Mode = ModelDeliveryModeNative
		}
		decision.CapabilitySource = "existing_gateway_contract"
		return decision
	}
	if strictOpenAIAPIKeyProtocolRouting(input) {
		decision.UpstreamProtocol = ModelProtocolAnthropicMessages
		decision.CapabilityState, decision.CapabilitySource = resolveCapabilityFromItems(
			input.Capabilities,
			decision.UpstreamModel,
			ModelProtocolAnthropicMessages,
			false,
		)
		if decision.CapabilityState != ModelProtocolStateSupported {
			return blockForCapabilityState(decision)
		}
		decision.Eligible = true
		decision.Mode = ModelDeliveryModeNative
		return decision
	}

	if input.NativeRoutingEnabled && account.Type == AccountTypeAPIKey {
		state, source := resolveCapabilityFromItems(
			input.Capabilities,
			decision.UpstreamModel,
			ModelProtocolAnthropicMessages,
			false,
		)
		if state == ModelProtocolStateSupported {
			decision.Eligible = true
			decision.UpstreamProtocol = ModelProtocolAnthropicMessages
			decision.Mode = ModelDeliveryModeNative
			decision.CapabilityState = state
			decision.CapabilitySource = source
			return decision
		}
	}

	// Existing Messages compatibility chooses Chat or Responses using the same
	// account route preference as the forwarding path. Unknown evidence keeps the
	// legacy bridge available; only explicit unsupported evidence blocks it.
	decision.UpstreamProtocol, decision.CapabilityState, decision.CapabilitySource = selectOpenAIUpstreamProtocolForModel(
		account,
		decision.UpstreamModel,
		input.Capabilities,
	)
	if !accountSupportsOpenAITransport(account, decision.UpstreamProtocol) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
	}
	if decision.CapabilityState == ModelProtocolStateUnsupported {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonCapabilityUnsupported)
	}
	decision.Eligible = true
	decision.Mode = ModelDeliveryModeCompatibility
	if decision.CapabilitySource == "" {
		decision.CapabilitySource = "existing_gateway_contract"
	}
	return decision
}

func openAIEndpointCapabilityForModelProtocol(protocol ModelProtocol) (OpenAIEndpointCapability, bool) {
	switch protocol {
	case ModelProtocolOpenAIEmbeddings:
		return OpenAIEndpointCapabilityEmbeddings, true
	case ModelProtocolOpenAILive:
		return OpenAIEndpointCapabilityLive, true
	default:
		return "", false
	}
}

func supportsEstablishedGatewayTextContract(platform string) bool {
	switch platform {
	case PlatformAnthropic, PlatformGemini, PlatformAntigravity:
		return true
	default:
		return false
	}
}

func strictOpenAIAPIKeyProtocolRouting(input ModelDeliveryCandidateInput) bool {
	account := input.Account
	return input.NativeRoutingEnabled &&
		account != nil &&
		account.Platform == PlatformOpenAI &&
		account.Type == AccountTypeAPIKey
}

func accountSupportsStrictOpenAIProtocolTransport(account *Account, protocol ModelProtocol) bool {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeAPIKey {
		return false
	}
	switch protocol {
	case ModelProtocolOpenAIChat, ModelProtocolOpenAIResponses:
		// The per-model protocol capability is authoritative for the concrete
		// upstream endpoint. The legacy account-wide Responses preference must
		// not override it; retain only the generic OpenAI HTTP capability gate.
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions)
	default:
		return false
	}
}

func blockForCapabilityState(decision ModelDeliveryDecision) ModelDeliveryDecision {
	if decision.CapabilityState == ModelProtocolStateUnsupported {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonCapabilityUnsupported)
	}
	return blockModelDeliveryDecision(decision, ModelDeliveryReasonCapabilityUnknown)
}

func blockModelDeliveryDecision(decision ModelDeliveryDecision, reasons ...ModelDeliveryReasonCode) ModelDeliveryDecision {
	decision.Eligible = false
	decision.Mode = ""
	decision.ReasonCodes = mergeModelDeliveryReasonCodes(decision.ReasonCodes, reasons)
	return decision
}

func mergeModelDeliveryReasonCodes(existing, additions []ModelDeliveryReasonCode) []ModelDeliveryReasonCode {
	seen := make(map[ModelDeliveryReasonCode]struct{}, len(existing)+len(additions))
	result := make([]ModelDeliveryReasonCode, 0, len(existing)+len(additions))
	for _, values := range [][]ModelDeliveryReasonCode{existing, additions} {
		for _, value := range values {
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func openAISelectedUpstreamProtocol(account *Account) ModelProtocol {
	if account != nil && account.Type == AccountTypeAPIKey && !openai_compat.ShouldUseResponsesAPI(account.Extra) {
		return ModelProtocolOpenAIChat
	}
	return ModelProtocolOpenAIResponses
}

// selectOpenAIUpstreamProtocolForModel applies the account-level route
// preference without letting an auto-detected, account-wide Responses flag
// override stronger model-level capability evidence. Forced modes remain
// authoritative. In auto mode, a supported alternate protocol wins when the
// preferred protocol is unknown or explicitly unsupported.
func selectOpenAIUpstreamProtocolForModel(
	account *Account,
	upstreamModel string,
	capabilities []AccountModelProtocolCapability,
) (ModelProtocol, ModelProtocolState, string) {
	preferred := openAISelectedUpstreamProtocol(account)
	preferredState, preferredSource := resolveCapabilityFromItems(
		capabilities,
		upstreamModel,
		preferred,
		accountIntrinsicProtocolSupport(account, preferred),
	)
	if account == nil || account.Type != AccountTypeAPIKey || openAIResponsesSupportMode(account) != openai_compat.ResponsesSupportModeAuto {
		return preferred, preferredState, preferredSource
	}

	alternate := ModelProtocolOpenAIChat
	if preferred == ModelProtocolOpenAIChat {
		alternate = ModelProtocolOpenAIResponses
	}
	alternateState, alternateSource := resolveCapabilityFromItems(
		capabilities,
		upstreamModel,
		alternate,
		accountIntrinsicProtocolSupport(account, alternate),
	)
	if preferredState != ModelProtocolStateSupported &&
		alternateState == ModelProtocolStateSupported &&
		accountSupportsOpenAITransport(account, alternate) {
		return alternate, alternateState, alternateSource
	}
	return preferred, preferredState, preferredSource
}

func openAIResponsesSupportMode(account *Account) openai_compat.ResponsesSupportMode {
	if account == nil || account.Extra == nil {
		return openai_compat.ResponsesSupportModeAuto
	}
	mode, _ := account.Extra[openai_compat.ExtraKeyResponsesMode].(string)
	return openai_compat.NormalizeResponsesSupportMode(mode)
}

func accountSupportsOpenAITransport(account *Account, protocol ModelProtocol) bool {
	if account == nil {
		return false
	}
	switch protocol {
	case ModelProtocolOpenAIChat:
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions)
	case ModelProtocolOpenAIResponses:
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses)
	case ModelProtocolOpenAIEmbeddings:
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings)
	case ModelProtocolOpenAILive:
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityLive)
	default:
		return false
	}
}

func useOpenAIResponsesForSelectedDelivery(account *Account, selectedProtocol ModelProtocol) (bool, error) {
	if account == nil {
		return false, fmt.Errorf("selected protocol requires an account")
	}
	if selectedProtocol == "" {
		return account.Type != AccountTypeAPIKey || openai_compat.ShouldUseResponsesAPI(account.Extra), nil
	}
	switch selectedProtocol {
	case ModelProtocolOpenAIResponses:
		return true, nil
	case ModelProtocolOpenAIChat:
		if account.Type != AccountTypeAPIKey {
			return false, fmt.Errorf("account %d cannot use selected upstream protocol %s", account.ID, selectedProtocol)
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported selected OpenAI upstream protocol %s", selectedProtocol)
	}
}
