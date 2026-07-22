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

type ModelDeliveryCandidateInput struct {
	Account               *Account
	PublicModel           string
	ChannelMappedModel    string
	GroupPlatform         string
	AllowMessagesDispatch bool
	DisableNativeMessages bool
	InboundProtocol       ModelProtocol
	NativeRoutingEnabled  bool
	Capabilities          []AccountModelProtocolCapability
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

	if input.InboundProtocol == ModelProtocolAnthropicMessages {
		return evaluateMessagesDeliveryCandidate(input, decision)
	}
	if input.InboundProtocol != ModelProtocolOpenAIChat && input.InboundProtocol != ModelProtocolOpenAIResponses {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonCapabilityUnsupported)
	}
	if !input.NativeRoutingEnabled {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonGlobalRoutingDisabled)
	}
	if !account.IsOpenAI() {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonPlatformMismatch)
	}

	decision.UpstreamProtocol = openAISelectedUpstreamProtocol(account)
	if !accountSupportsOpenAITransport(account, decision.UpstreamProtocol) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
	}
	decision.CapabilityState, decision.CapabilitySource = resolveCapabilityFromItems(
		input.Capabilities,
		decision.UpstreamModel,
		decision.UpstreamProtocol,
		accountIntrinsicProtocolSupport(account, decision.UpstreamProtocol),
	)
	if decision.CapabilityState != ModelProtocolStateSupported {
		return blockForCapabilityState(decision)
	}
	decision.Eligible = true
	decision.Mode = ModelDeliveryModeCompatibility
	if decision.InboundProtocol == decision.UpstreamProtocol {
		decision.Mode = ModelDeliveryModeNative
	}
	return decision
}

func evaluateMessagesDeliveryCandidate(input ModelDeliveryCandidateInput, decision ModelDeliveryDecision) ModelDeliveryDecision {
	account := input.Account
	if !input.AllowMessagesDispatch && input.GroupPlatform == PlatformOpenAI {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonGroupProtocolDisabled)
	}
	if !account.IsOpenAI() {
		decision.Eligible = true
		decision.UpstreamProtocol = ModelProtocolAnthropicMessages
		decision.Mode = ModelDeliveryModeCompatibility
		decision.CapabilitySource = "existing_gateway_contract"
		return decision
	}

	if input.NativeRoutingEnabled && !input.DisableNativeMessages && account.Type == AccountTypeAPIKey {
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
	decision.UpstreamProtocol = openAISelectedUpstreamProtocol(account)
	if !accountSupportsOpenAITransport(account, decision.UpstreamProtocol) {
		return blockModelDeliveryDecision(decision, ModelDeliveryReasonAccountTransportUnavailable)
	}
	decision.CapabilityState, decision.CapabilitySource = resolveCapabilityFromItems(
		input.Capabilities,
		decision.UpstreamModel,
		decision.UpstreamProtocol,
		accountIntrinsicProtocolSupport(account, decision.UpstreamProtocol),
	)
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

func accountSupportsOpenAITransport(account *Account, protocol ModelProtocol) bool {
	if account == nil {
		return false
	}
	switch protocol {
	case ModelProtocolOpenAIChat:
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions)
	case ModelProtocolOpenAIResponses:
		return account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityResponses)
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
