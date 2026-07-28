package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrModelProtocolCapabilityUnavailable = errors.New("model protocol capability unavailable")

// SelectAccountWithSchedulerForProtocolDelivery preserves all existing live
// scheduler constraints and then applies the same stable per-model delivery
// decision used by catalog projection. The outer legacy selector is allowed
// only when strict protocol routing is disabled before an authoritative
// decision is made.
func (s *OpenAIGatewayService) SelectAccountWithSchedulerForProtocolDelivery(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	channelMappedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
	previousResponseCanMove bool,
	useUpstreamTokenCost bool,
	inboundProtocol ModelProtocol,
	platform string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, ModelDeliveryDecision, error) {
	return s.selectAccountWithSchedulerForProtocolDelivery(
		ctx, groupID, previousResponseID, sessionHash, requestedModel, channelMappedModel, excludedIDs,
		requiredTransport, requiredCapability, requireCompact, previousResponseCanMove, useUpstreamTokenCost,
		inboundProtocol, platform,
	)
}

func (s *OpenAIGatewayService) selectAccountWithSchedulerForProtocolDelivery(
	ctx context.Context,
	groupID *int64,
	previousResponseID string,
	sessionHash string,
	requestedModel string,
	channelMappedModel string,
	excludedIDs map[int64]struct{},
	requiredTransport OpenAIUpstreamTransport,
	requiredCapability OpenAIEndpointCapability,
	requireCompact bool,
	previousResponseCanMove bool,
	useUpstreamTokenCost bool,
	inboundProtocol ModelProtocol,
	platform string,
) (*AccountSelectionResult, OpenAIAccountScheduleDecision, ModelDeliveryDecision, error) {
	decision := OpenAIAccountScheduleDecision{}
	delivery := ModelDeliveryDecision{InboundProtocol: inboundProtocol}
	if s == nil {
		return nil, decision, delivery, ErrNoAvailableAccounts
	}
	var routingSettings NativeModelProtocolRoutingSettingReader
	if s.settingService != nil {
		routingSettings = s.settingService
	}
	if !nativeModelProtocolRoutingEnabled(ctx, routingSettings, s.cfg) {
		return nil, decision, delivery, ErrNoAvailableAccounts
	}
	if platform != PlatformOpenAI {
		// Per-model strict protocol capabilities currently belong to OpenAI
		// API-key upstreams. Other platform adapters retain their established
		// selector and conversion behavior.
		return nil, decision, delivery, ErrNoAvailableAccounts
	}
	if s.modelProtocolCapability == nil {
		delivery = blockModelDeliveryDecision(delivery, ModelDeliveryReasonCapabilityUnknown)
		return nil, decision, delivery, ErrModelProtocolCapabilityUnavailable
	}
	delivery = blockModelDeliveryDecision(delivery, ModelDeliveryReasonNoStableRoute)

	schedulerRequiredCapability := requiredCapability
	if inboundProtocol == ModelProtocolOpenAIResponses &&
		requiredCapability == OpenAIEndpointCapabilityResponses {
		// Exact per-model Responses capability is authoritative for strict API
		// key routing. Widen the account-level prefilter to generic text
		// generation, then let the delivery decision enforce the concrete
		// Responses endpoint. This prevents the legacy account-wide Responses
		// preference from hiding a proven per-model route.
		schedulerRequiredCapability = OpenAIEndpointCapabilityChatCompletions
	}

	effectiveExcluded := cloneExcludedAccountIDs(excludedIDs)
	var legacySelection *AccountSelectionResult
	var legacyScheduleDecision OpenAIAccountScheduleDecision
	var legacyDelivery ModelDeliveryDecision
	releaseLegacySelection := func() {
		if legacySelection != nil && legacySelection.ReleaseFunc != nil {
			legacySelection.ReleaseFunc()
		}
		legacySelection = nil
	}
	for {
		selection, nextDecision, err := s.selectAccountWithSchedulerForResolvedModel(
			ctx,
			groupID,
			previousResponseID,
			sessionHash,
			requestedModel,
			channelMappedModel,
			effectiveExcluded,
			requiredTransport,
			schedulerRequiredCapability,
			requireCompact,
			previousResponseCanMove,
			useUpstreamTokenCost,
			platform,
		)
		decision = nextDecision
		if err != nil || selection == nil || selection.Account == nil {
			if legacySelection != nil {
				s.bindProtocolDeliverySticky(ctx, groupID, sessionHash, legacySelection)
				return legacySelection, legacyScheduleDecision, allowLegacyUnknownProtocolDelivery(legacyDelivery), nil
			}
			return selection, decision, delivery, err
		}

		account := selection.Account
		strictAccount := strictOpenAIAPIKeyProtocolRouting(ModelDeliveryCandidateInput{
			Account:              account,
			NativeRoutingEnabled: true,
		})
		capabilities, capabilityErr := s.modelProtocolCapability.List(ctx, account.ID)
		if capabilityErr != nil {
			if selection.ReleaseFunc != nil {
				selection.ReleaseFunc()
			}
			if legacySelection != nil {
				s.bindProtocolDeliverySticky(ctx, groupID, sessionHash, legacySelection)
				return legacySelection, legacyScheduleDecision, allowLegacyUnknownProtocolDelivery(legacyDelivery), nil
			}
			delivery = blockModelDeliveryDecision(delivery, ModelDeliveryReasonCapabilityUnknown)
			return nil, decision, delivery, fmt.Errorf("%w: %v", ErrModelProtocolCapabilityUnavailable, capabilityErr)
		}
		delivery = EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
			Account:               account,
			PublicModel:           requestedModel,
			ChannelMappedModel:    channelMappedModel,
			GroupPlatform:         account.Platform,
			AllowMessagesDispatch: true,
			InboundProtocol:       inboundProtocol,
			NativeRoutingEnabled:  true,
			Capabilities:          capabilities,
		})
		if delivery.Eligible {
			releaseLegacySelection()
			s.bindProtocolDeliverySticky(ctx, groupID, sessionHash, selection)
			return selection, decision, delivery, nil
		}
		if modelDeliveryBlockedOnlyByCapabilityUnknown(delivery) && !strictAccount && legacySelection == nil {
			legacySelection = selection
			legacyScheduleDecision = decision
			legacyDelivery = delivery
		} else if selection.ReleaseFunc != nil {
			selection.ReleaseFunc()
		}

		if effectiveExcluded == nil {
			effectiveExcluded = make(map[int64]struct{})
		}
		if _, exists := effectiveExcluded[account.ID]; exists {
			releaseLegacySelection()
			return nil, decision, delivery, ErrNoAvailableAccounts
		}
		effectiveExcluded[account.ID] = struct{}{}
	}
}

func (s *OpenAIGatewayService) bindProtocolDeliverySticky(
	ctx context.Context,
	groupID *int64,
	sessionHash string,
	selection *AccountSelectionResult,
) {
	if s == nil || selection == nil || selection.Account == nil || strings.TrimSpace(sessionHash) == "" {
		return
	}
	_ = s.BindStickySession(ctx, groupID, sessionHash, selection.Account.ID)
}

// ShouldUseLegacyProtocolDeliverySelector limits the outer legacy selector to
// cases where the new policy never made an authoritative decision. In
// particular, an administrator's explicit unsupported override must never be
// bypassed by selecting the same account again through the legacy path.
func ShouldUseLegacyProtocolDeliverySelector(err error, delivery ModelDeliveryDecision) bool {
	if errors.Is(err, ErrModelProtocolCapabilityUnavailable) {
		return len(delivery.ReasonCodes) == 0
	}
	return errors.Is(err, ErrNoAvailableAccounts) && len(delivery.ReasonCodes) == 0
}

func modelDeliveryBlockedOnlyByCapabilityUnknown(delivery ModelDeliveryDecision) bool {
	return !delivery.Eligible &&
		len(delivery.ReasonCodes) == 1 &&
		delivery.ReasonCodes[0] == ModelDeliveryReasonCapabilityUnknown
}

func allowLegacyUnknownProtocolDelivery(delivery ModelDeliveryDecision) ModelDeliveryDecision {
	delivery.Eligible = true
	delivery.ReasonCodes = nil
	delivery.Mode = ModelDeliveryModeCompatibility
	if delivery.InboundProtocol == delivery.UpstreamProtocol {
		delivery.Mode = ModelDeliveryModeNative
	}
	delivery.CapabilitySource = "existing_gateway_contract"
	return delivery
}
