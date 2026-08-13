package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// EnterpriseMemberRouteReasonCode is the stable service-layer explanation for
// why an authorized member group was kept or pruned by the model-aware planner.
type EnterpriseMemberRouteReasonCode string

const (
	EnterpriseMemberRouteReasonEligible           EnterpriseMemberRouteReasonCode = "eligible"
	EnterpriseMemberRouteReasonModelUnpublished   EnterpriseMemberRouteReasonCode = "model_unpublished"
	EnterpriseMemberRouteReasonModelUnsupported   EnterpriseMemberRouteReasonCode = "model_unsupported"
	EnterpriseMemberRouteReasonEndpointCapability EnterpriseMemberRouteReasonCode = "endpoint_capability"
	EnterpriseMemberRouteReasonNoPersistentPool   EnterpriseMemberRouteReasonCode = "no_persistent_pool"
	EnterpriseMemberRouteReasonEvaluationFailed   EnterpriseMemberRouteReasonCode = "evaluation_failed"
)

type EnterpriseMemberRoutePublicationSource string

const (
	EnterpriseMemberRoutePublicationExactMapping                   EnterpriseMemberRoutePublicationSource = "exact_mapping"
	EnterpriseMemberRoutePublicationExactPricing                   EnterpriseMemberRoutePublicationSource = "exact_pricing"
	EnterpriseMemberRoutePublicationWildcardMappingPricedExpansion EnterpriseMemberRoutePublicationSource = "wildcard_mapping_priced_expansion"
)

type EnterpriseMemberRoutePublication struct {
	GroupID            int64
	ChannelID          int64
	PublicModel        string
	ChannelMappedModel string
	Source             EnterpriseMemberRoutePublicationSource
}

type EnterpriseMemberRouteInput struct {
	AuthorizedGroups []*Group
	Model            string
	Protocol         ModelProtocol
	Endpoint         string
	Body             []byte
}

type EnterpriseMemberRoutePlan struct {
	Model      string
	Protocol   ModelProtocol
	Source     EnterpriseMemberRoutePlanSource
	Candidates []EnterpriseMemberRouteCandidateDecision
	Rejected   []EnterpriseMemberRouteCandidateDecision
	Warnings   []string
}

type EnterpriseMemberRouteCandidateDecision struct {
	Group              *Group
	GroupID            int64
	Reason             EnterpriseMemberRouteReasonCode
	Publication        *EnterpriseMemberRoutePublication
	Delivery           *ModelDeliveryGroupProjection
	ChannelMappedModel string
	// RoutePlanSnapshotAgeMs is populated only for last-known-good candidates
	// restored from a bounded, non-expired snapshot.
	RoutePlanSnapshotAgeMs *int64
}

type EnterpriseMemberRoutePublicationResolver interface {
	ResolveEnterpriseMemberRoutePublications(ctx context.Context, groupIDs []int64, model string) (map[int64]EnterpriseMemberRoutePublication, error)
}

type EnterpriseMemberRouteDeliveryResolver interface {
	ResolveForEnterpriseMemberRoute(ctx context.Context, groups []*Group, model string, protocol ModelProtocol) (*ModelDeliveryProjection, error)
}

// EnterpriseMemberRoutePlanningService is the request-path contract consumed
// by enterprise member middleware. Keeping the middleware on this narrow
// interface makes shadow/enforce behavior testable without repositories or
// scheduler state.
type EnterpriseMemberRoutePlanningService interface {
	Plan(ctx context.Context, input EnterpriseMemberRouteInput) (*EnterpriseMemberRoutePlan, error)
}

// EnterpriseMemberRoutePublicationFilter is the low-cost request-path contract
// used before group failover. It consults only cached channel publication
// metadata and deliberately does not inspect accounts, scheduler capacity, or
// protocol capability repositories.
//
// Callers must treat an error or an empty result as absence of positive
// evidence, not as proof that every authorized group is invalid.
type EnterpriseMemberRoutePublicationFilter interface {
	ResolvePublishedGroupIDs(ctx context.Context, input EnterpriseMemberRouteInput) ([]int64, error)
}

type EnterpriseMemberRoutePlanner struct {
	publication EnterpriseMemberRoutePublicationResolver
	delivery    EnterpriseMemberRouteDeliveryResolver
	runtime     *RoutingEligibilityRuntime
}

func NewEnterpriseMemberRoutePlanner(publication EnterpriseMemberRoutePublicationResolver, delivery EnterpriseMemberRouteDeliveryResolver) *EnterpriseMemberRoutePlanner {
	return &EnterpriseMemberRoutePlanner{
		publication: publication,
		delivery:    delivery,
	}
}

func NewEnterpriseMemberRoutePlannerForChannel(channel *ChannelService, delivery *ModelDeliveryService) *EnterpriseMemberRoutePlanner {
	return NewEnterpriseMemberRoutePlanner(NewEnterpriseMemberChannelPublicationResolver(channel), delivery)
}

func (p *EnterpriseMemberRoutePlanner) SetRoutingEligibilityRuntime(runtime *RoutingEligibilityRuntime) {
	if p != nil {
		p.runtime = runtime
	}
}

// ResolvePublishedGroupIDs returns authorized groups that explicitly publish
// the requested public model, preserving member preference order. The channel
// resolver is backed by ChannelService's bounded cache and singleflight, so
// this path avoids the per-request account and capability queries performed by
// the full delivery planner.
func (p *EnterpriseMemberRoutePlanner) ResolvePublishedGroupIDs(ctx context.Context, input EnterpriseMemberRouteInput) ([]int64, error) {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		return nil, nil
	}
	orderedGroupIDs := make([]int64, 0, len(input.AuthorizedGroups))
	seen := make(map[int64]struct{}, len(input.AuthorizedGroups))
	for _, group := range input.AuthorizedGroups {
		if !IsGroupContextValid(group) || !group.IsActive() {
			continue
		}
		if _, ok := seen[group.ID]; ok {
			continue
		}
		seen[group.ID] = struct{}{}
		orderedGroupIDs = append(orderedGroupIDs, group.ID)
	}
	if len(orderedGroupIDs) == 0 {
		return nil, nil
	}
	if p == nil || p.publication == nil {
		return nil, fmt.Errorf("enterprise member route publication resolver is not configured")
	}
	publications, err := p.publication.ResolveEnterpriseMemberRoutePublications(ctx, orderedGroupIDs, model)
	if err != nil {
		return nil, fmt.Errorf("resolve enterprise member route publications: %w", err)
	}
	publishedGroupIDs := make([]int64, 0, len(publications))
	for _, groupID := range orderedGroupIDs {
		if _, ok := publications[groupID]; ok {
			publishedGroupIDs = append(publishedGroupIDs, groupID)
		}
	}
	return publishedGroupIDs, nil
}

func (p *EnterpriseMemberRoutePlanner) Plan(ctx context.Context, input EnterpriseMemberRouteInput) (*EnterpriseMemberRoutePlan, error) {
	model := strings.TrimSpace(input.Model)
	protocol := normalizeEnterpriseMemberRouteProtocol(input.Protocol, input.Endpoint)
	plan := &EnterpriseMemberRoutePlan{Model: model, Protocol: protocol, Source: EnterpriseMemberRoutePlanSourceLive}
	if model == "" || protocol == "" {
		return plan, nil
	}

	orderedGroups := make([]*Group, 0, len(input.AuthorizedGroups))
	groupIDs := make([]int64, 0, len(input.AuthorizedGroups))
	seen := make(map[int64]struct{}, len(input.AuthorizedGroups))
	for _, group := range input.AuthorizedGroups {
		if !IsGroupContextValid(group) || !group.IsActive() {
			if group != nil && group.ID > 0 {
				plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonNoPersistentPool, nil, nil))
			}
			continue
		}
		if _, ok := seen[group.ID]; ok {
			continue
		}
		seen[group.ID] = struct{}{}
		orderedGroups = append(orderedGroups, group)
		groupIDs = append(groupIDs, group.ID)
	}
	if len(orderedGroups) == 0 {
		return plan, nil
	}
	if p == nil || p.publication == nil || p.delivery == nil {
		for _, group := range orderedGroups {
			plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEvaluationFailed, nil, nil))
		}
		return plan, fmt.Errorf("enterprise member route planner is not configured")
	}

	publications, err := p.publication.ResolveEnterpriseMemberRoutePublications(ctx, groupIDs, model)
	if err != nil {
		for _, group := range orderedGroups {
			plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEvaluationFailed, nil, nil))
		}
		return p.restoreLastKnownGood(ctx, input, orderedGroups, plan, fmt.Errorf("resolve enterprise member route publications: %w", err))
	}

	publishedGroups := make([]int64, 0, len(orderedGroups))
	publishedGroupSnapshots := make([]*Group, 0, len(orderedGroups))
	for _, group := range orderedGroups {
		if _, ok := publications[group.ID]; ok {
			publishedGroups = append(publishedGroups, group.ID)
			publishedGroupSnapshots = append(publishedGroupSnapshots, group)
		}
	}
	if len(publishedGroups) == 0 {
		for _, group := range orderedGroups {
			plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonModelUnpublished, nil, nil))
		}
		return plan, nil
	}

	projection, err := p.delivery.ResolveForEnterpriseMemberRoute(ctx, publishedGroupSnapshots, model, protocol)
	if err != nil {
		for _, group := range orderedGroups {
			if pub, ok := publications[group.ID]; ok {
				plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEvaluationFailed, &pub, nil))
			}
		}
		return p.restoreLastKnownGood(ctx, input, orderedGroups, plan, fmt.Errorf("resolve enterprise member route delivery: %w", err))
	}
	if projection != nil {
		plan.Warnings = append(plan.Warnings, projection.Warnings...)
	}

	explicitImageIntent := IsExplicitImageGenerationIntent(enterpriseMemberRouteEndpoint(input.Endpoint, protocol), model, input.Body)
	for _, group := range orderedGroups {
		pub, published := publications[group.ID]
		if !published {
			plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonModelUnpublished, nil, nil))
			continue
		}
		if explicitImageIntent && !GroupAllowsImageGeneration(group) {
			plan.Rejected = append(plan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEndpointCapability, &pub, nil))
			continue
		}
		groupProjection := projection.Group(model, group.ID)
		reason := enterpriseMemberRouteReasonForProjection(groupProjection, protocol)
		decision := enterpriseMemberRouteDecision(group, reason, &pub, groupProjection)
		if reason == EnterpriseMemberRouteReasonEligible {
			plan.Candidates = append(plan.Candidates, decision)
		} else {
			plan.Rejected = append(plan.Rejected, decision)
		}
	}
	p.storeLastKnownGood(input, plan)
	RecordEnterpriseMemberRoutePlanSource(EnterpriseMemberRoutePlanSourceLive, false)
	return plan, nil
}

func (p *EnterpriseMemberRoutePlanner) storeLastKnownGood(input EnterpriseMemberRouteInput, plan *EnterpriseMemberRoutePlan) {
	if p == nil || p.runtime == nil || plan == nil || plan.Source != EnterpriseMemberRoutePlanSourceLive || len(plan.Candidates) == 0 {
		return
	}
	store := p.runtime.SnapshotStore()
	if store == nil {
		return
	}
	intent := enterpriseMemberRouteSnapshotIntent(input, plan.Protocol)
	endpoint := enterpriseMemberRouteEndpoint(input.Endpoint, plan.Protocol)
	for _, candidate := range plan.Candidates {
		version, ok := p.runtime.MirroredVersion(enterpriseMemberRouteEligibilityScopes(candidate.GroupID))
		if !ok {
			continue
		}
		snapshotCandidate, ok := enterpriseMemberRouteSnapshotCandidateForDecision(candidate, plan.Protocol, plan.Model)
		if !ok {
			continue
		}
		store.Store(EnterpriseMemberRouteSnapshotPlan{
			Source: EnterpriseMemberRoutePlanSourceLive,
			Key: EnterpriseMemberRouteSnapshotKey{
				PublicModel:      plan.Model,
				Endpoint:         endpoint,
				InboundProtocol:  plan.Protocol,
				Intent:           intent,
				GroupID:          candidate.GroupID,
				Eligibility:      version,
				AlgorithmVersion: enterpriseMemberRouteSnapshotAlgorithmVersion,
			},
			Candidates: []EnterpriseMemberRouteSnapshotCandidate{snapshotCandidate},
		})
	}
}

func (p *EnterpriseMemberRoutePlanner) restoreLastKnownGood(
	ctx context.Context,
	input EnterpriseMemberRouteInput,
	groups []*Group,
	failedPlan *EnterpriseMemberRoutePlan,
	planningErr error,
) (*EnterpriseMemberRoutePlan, error) {
	if p == nil || p.runtime == nil || failedPlan == nil || len(groups) == 0 {
		return failedPlan, planningErr
	}
	allScopes := enterpriseMemberRouteGlobalEligibilityScopes()
	authorizedGroupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group == nil || group.ID <= 0 {
			continue
		}
		authorizedGroupIDs = append(authorizedGroupIDs, group.ID)
		allScopes = append(allScopes, RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: group.ID})
	}
	current, revisionReady := p.runtime.MirroredVersion(allScopes)
	if !revisionReady {
		RecordEnterpriseMemberRoutePlanSource("", true)
		return failedPlan, errors.Join(planningErr, ErrRoutingEligibilityRevisionUnavailable)
	}
	store := p.runtime.SnapshotStore()
	if store == nil {
		return failedPlan, planningErr
	}
	restoredPlan := &EnterpriseMemberRoutePlan{
		Model:    failedPlan.Model,
		Protocol: failedPlan.Protocol,
		Source:   EnterpriseMemberRoutePlanSourceLastKnownGood,
	}
	intent := enterpriseMemberRouteSnapshotIntent(input, failedPlan.Protocol)
	endpoint := enterpriseMemberRouteEndpoint(input.Endpoint, failedPlan.Protocol)
	for _, group := range groups {
		if group == nil || group.ID <= 0 {
			continue
		}
		version, ok := enterpriseMemberRouteVersionForGroup(current, group.ID)
		if !ok {
			restoredPlan.Rejected = append(restoredPlan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEvaluationFailed, nil, nil))
			continue
		}
		restored, ok := store.Restore(EnterpriseMemberRouteSnapshotKey{
			PublicModel:      failedPlan.Model,
			Endpoint:         endpoint,
			InboundProtocol:  failedPlan.Protocol,
			Intent:           intent,
			GroupID:          group.ID,
			Eligibility:      version,
			AlgorithmVersion: enterpriseMemberRouteSnapshotAlgorithmVersion,
		}, authorizedGroupIDs)
		if !ok || len(restored.Candidates) == 0 {
			restoredPlan.Rejected = append(restoredPlan.Rejected, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEvaluationFailed, nil, nil))
			continue
		}
		snapshot := restored.Candidates[0]
		publication := &EnterpriseMemberRoutePublication{
			GroupID:            group.ID,
			PublicModel:        snapshot.PublicModel,
			ChannelMappedModel: snapshot.ChannelMappedModel,
		}
		restoredPlan.Candidates = append(restoredPlan.Candidates, enterpriseMemberRouteDecision(group, EnterpriseMemberRouteReasonEligible, publication, nil))
		restoredPlan.Candidates[len(restoredPlan.Candidates)-1].RoutePlanSnapshotAgeMs = restored.SnapshotAgeMs
	}
	if len(restoredPlan.Candidates) == 0 {
		RecordEnterpriseMemberRoutePlanSource("", false)
		return failedPlan, planningErr
	}
	RecordEnterpriseMemberRoutePlanSource(EnterpriseMemberRoutePlanSourceLastKnownGood, false)
	return restoredPlan, nil
}

func enterpriseMemberRouteGlobalEligibilityScopes() []RoutingEligibilityScope {
	return []RoutingEligibilityScope{
		{Type: RoutingEligibilityScopeChannel, ID: 0},
		{Type: RoutingEligibilityScopeAccount, ID: 0},
		{Type: RoutingEligibilityScopeProtocol, ID: 0},
		{Type: RoutingEligibilityScopeComposite, ID: 0},
	}
}

func enterpriseMemberRouteEligibilityScopes(groupID int64) []RoutingEligibilityScope {
	scopes := enterpriseMemberRouteGlobalEligibilityScopes()
	if groupID > 0 {
		scopes = append(scopes, RoutingEligibilityScope{Type: RoutingEligibilityScopeGroup, ID: groupID})
	}
	return scopes
}

func enterpriseMemberRouteVersionForGroup(current RoutingEligibilityVersion, groupID int64) (RoutingEligibilityVersion, bool) {
	wanted := normalizeRoutingEligibilityScopes(enterpriseMemberRouteEligibilityScopes(groupID))
	byScope := make(map[RoutingEligibilityScope]uint64, len(current.Items))
	for _, item := range current.Items {
		byScope[item.Scope] = item.Revision
	}
	items := make([]RoutingEligibilityScopeRevision, 0, len(wanted))
	for _, scope := range wanted {
		revision := byScope[scope]
		if revision == 0 {
			return RoutingEligibilityVersion{}, false
		}
		items = append(items, RoutingEligibilityScopeRevision{Scope: scope, Revision: revision})
	}
	return NewRoutingEligibilityVersion(items), true
}

func enterpriseMemberRouteSnapshotIntent(input EnterpriseMemberRouteInput, protocol ModelProtocol) EnterpriseMemberRouteIntentProfile {
	if IsExplicitImageGenerationIntent(enterpriseMemberRouteEndpoint(input.Endpoint, protocol), input.Model, input.Body) {
		return EnterpriseMemberRouteIntentImage
	}
	if protocol == ModelProtocolOpenAIImages || protocol == ModelProtocolBatchImages {
		return EnterpriseMemberRouteIntentImage
	}
	if protocol == ModelProtocolAnthropicMessages {
		return EnterpriseMemberRouteIntentMessages
	}
	return EnterpriseMemberRouteIntentText
}

func enterpriseMemberRouteSnapshotCandidateForDecision(
	candidate EnterpriseMemberRouteCandidateDecision,
	protocol ModelProtocol,
	publicModel string,
) (EnterpriseMemberRouteSnapshotCandidate, bool) {
	if candidate.GroupID <= 0 || candidate.Delivery == nil {
		return EnterpriseMemberRouteSnapshotCandidate{}, false
	}
	for _, route := range candidate.Delivery.Routes {
		decision, ok := route.Decisions[protocol]
		if !ok || !decision.Eligible {
			continue
		}
		channelMappedModel := candidate.ChannelMappedModel
		if channelMappedModel == "" {
			channelMappedModel = route.ChannelMappedModel
		}
		return EnterpriseMemberRouteSnapshotCandidate{
			GroupID:            candidate.GroupID,
			ReasonCode:         string(EnterpriseMemberRouteReasonEligible),
			PublicModel:        publicModel,
			ChannelMappedModel: channelMappedModel,
			UpstreamModel:      route.UpstreamModel,
			InboundProtocol:    protocol,
			UpstreamProtocol:   decision.UpstreamProtocol,
			DeliveryMode:       decision.Mode,
		}, true
	}
	return EnterpriseMemberRouteSnapshotCandidate{}, false
}

func enterpriseMemberRouteDecision(group *Group, reason EnterpriseMemberRouteReasonCode, publication *EnterpriseMemberRoutePublication, delivery *ModelDeliveryGroupProjection) EnterpriseMemberRouteCandidateDecision {
	var pub *EnterpriseMemberRoutePublication
	if publication != nil {
		cp := *publication
		pub = &cp
	}
	decision := EnterpriseMemberRouteCandidateDecision{
		Group:       group,
		Reason:      reason,
		Publication: pub,
		Delivery:    delivery,
	}
	if group != nil {
		decision.GroupID = group.ID
	}
	if pub != nil {
		decision.ChannelMappedModel = pub.ChannelMappedModel
	}
	return decision
}

func enterpriseMemberRouteReasonForProjection(projection *ModelDeliveryGroupProjection, protocol ModelProtocol) EnterpriseMemberRouteReasonCode {
	if projection == nil {
		return EnterpriseMemberRouteReasonNoPersistentPool
	}
	if enterpriseMemberRouteProjectionSupportsProtocol(projection, protocol) {
		return EnterpriseMemberRouteReasonEligible
	}
	decision, ok := projection.Decisions[protocol]
	if !ok {
		if projection.StableRouteAvailable() {
			return EnterpriseMemberRouteReasonEndpointCapability
		}
		return EnterpriseMemberRouteReasonNoPersistentPool
	}
	if hasModelDeliveryReason(decision, ModelDeliveryReasonModelUnsupported) {
		return EnterpriseMemberRouteReasonModelUnsupported
	}
	if hasModelDeliveryReason(decision, ModelDeliveryReasonGroupProtocolDisabled, ModelDeliveryReasonGlobalRoutingDisabled, ModelDeliveryReasonAccountTransportUnavailable, ModelDeliveryReasonCapabilityUnknown, ModelDeliveryReasonCapabilityUnsupported, ModelDeliveryReasonPlatformMismatch) {
		return EnterpriseMemberRouteReasonEndpointCapability
	}
	return EnterpriseMemberRouteReasonNoPersistentPool
}

func enterpriseMemberRouteProjectionSupportsProtocol(projection *ModelDeliveryGroupProjection, protocol ModelProtocol) bool {
	if projection == nil || protocol == "" {
		return false
	}
	// Group-level endpoint and route aggregates are built independently. The
	// planner must prove that one concrete stable route can serve the requested
	// protocol; combining an endpoint from one account with a stable route from
	// another would create a false-positive candidate.
	for i := range projection.Routes {
		decision, ok := projection.Routes[i].Decisions[protocol]
		if ok && decision.Eligible {
			return true
		}
	}
	return false
}

func hasModelDeliveryReason(decision ModelDeliveryDecision, reasons ...ModelDeliveryReasonCode) bool {
	if len(decision.ReasonCodes) == 0 || len(reasons) == 0 {
		return false
	}
	want := make(map[ModelDeliveryReasonCode]struct{}, len(reasons))
	for _, reason := range reasons {
		want[reason] = struct{}{}
	}
	for _, reason := range decision.ReasonCodes {
		if _, ok := want[reason]; ok {
			return true
		}
	}
	return false
}

func normalizeEnterpriseMemberRouteProtocol(protocol ModelProtocol, endpoint string) ModelProtocol {
	if protocol != "" {
		return protocol
	}
	endpoint = strings.ToLower(strings.TrimSpace(endpoint))
	switch {
	case strings.Contains(endpoint, "/chat/completions"):
		return ModelProtocolOpenAIChat
	case strings.Contains(endpoint, "/responses"):
		return ModelProtocolOpenAIResponses
	case strings.Contains(endpoint, "/messages"):
		return ModelProtocolAnthropicMessages
	case strings.Contains(endpoint, "/embeddings"):
		return ModelProtocolOpenAIEmbeddings
	case strings.Contains(endpoint, "/images/batches"):
		return ModelProtocolBatchImages
	case strings.Contains(endpoint, "/images/generations") || strings.Contains(endpoint, "/images/edits"):
		return ModelProtocolOpenAIImages
	case strings.Contains(endpoint, "/live") || strings.Contains(endpoint, "/realtime/calls"):
		return ModelProtocolOpenAILive
	case strings.Contains(endpoint, "/videos/generations") || strings.Contains(endpoint, "/videos/edits") || strings.Contains(endpoint, "/videos/extensions"):
		return ModelProtocolGrokVideo
	case strings.Contains(endpoint, "/v1beta/models") || strings.Contains(endpoint, "/antigravity/v1beta/models"):
		return ModelProtocolGeminiNative
	default:
		return ""
	}
}

// SupportsEnterpriseMemberRoutePlanning reports whether the current stable
// delivery projection has a protocol contract for this endpoint.
func SupportsEnterpriseMemberRoutePlanning(endpoint string) bool {
	return normalizeEnterpriseMemberRouteProtocol("", endpoint) != ""
}

func enterpriseMemberRouteEndpoint(endpoint string, protocol ModelProtocol) string {
	if strings.TrimSpace(endpoint) != "" {
		return strings.TrimSpace(endpoint)
	}
	return PublicPathForModelProtocol(protocol)
}

type enterpriseMemberChannelPublicationResolver struct {
	channel *ChannelService
}

func NewEnterpriseMemberChannelPublicationResolver(channel *ChannelService) EnterpriseMemberRoutePublicationResolver {
	return &enterpriseMemberChannelPublicationResolver{channel: channel}
}

func (r *enterpriseMemberChannelPublicationResolver) ResolveEnterpriseMemberRoutePublications(ctx context.Context, groupIDs []int64, model string) (map[int64]EnterpriseMemberRoutePublication, error) {
	result := make(map[int64]EnterpriseMemberRoutePublication)
	model = strings.TrimSpace(model)
	if r == nil || r.channel == nil || model == "" || len(groupIDs) == 0 {
		return result, nil
	}
	cache, err := r.channel.loadCache(ctx)
	if err != nil {
		return nil, err
	}
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		ch, ok := cache.channelByGroupID[groupID]
		if !ok || ch == nil || !ch.IsActive() {
			continue
		}
		platform := channelLookupPlatform(ctx, cache.groupPlatform[groupID])
		if publication, ok := resolveEnterpriseMemberChannelPublication(cache, ch, groupID, platform, model); ok {
			result[groupID] = publication
		}
	}
	return result, nil
}

func resolveEnterpriseMemberChannelPublication(cache *channelCache, channel *Channel, groupID int64, platform, model string) (EnterpriseMemberRoutePublication, bool) {
	modelLower := strings.ToLower(strings.TrimSpace(model))
	if cache == nil || channel == nil || groupID <= 0 || modelLower == "" {
		return EnterpriseMemberRoutePublication{}, false
	}
	for _, candidatePlatform := range matchingPlatforms(platform) {
		key := channelModelKey{groupID: groupID, platform: candidatePlatform, model: modelLower}
		if mapped, ok := cache.mappingByGroupModel[key]; ok {
			return EnterpriseMemberRoutePublication{
				GroupID:            groupID,
				ChannelID:          channel.ID,
				PublicModel:        model,
				ChannelMappedModel: effectiveChannelMappedModel(model, ChannelMappingResult{MappedModel: mapped}),
				Source:             EnterpriseMemberRoutePublicationExactMapping,
			}, true
		}
	}
	pricingModel := normalizeChannelPricingModelName(modelLower)
	for _, candidatePlatform := range matchingPlatforms(platform) {
		key := channelModelKey{groupID: groupID, platform: candidatePlatform, model: pricingModel}
		if _, ok := cache.pricingByGroupModel[key]; !ok {
			continue
		}
		source := EnterpriseMemberRoutePublicationExactPricing
		mapped := model
		if wildcardMapped := cache.matchWildcardMapping(groupID, candidatePlatform, modelLower); wildcardMapped != "" {
			source = EnterpriseMemberRoutePublicationWildcardMappingPricedExpansion
			mapped = wildcardMapped
		}
		return EnterpriseMemberRoutePublication{
			GroupID:            groupID,
			ChannelID:          channel.ID,
			PublicModel:        model,
			ChannelMappedModel: effectiveChannelMappedModel(model, ChannelMappingResult{MappedModel: mapped}),
			Source:             source,
		}, true
	}
	return EnterpriseMemberRoutePublication{}, false
}
