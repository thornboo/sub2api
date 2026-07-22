package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// ModelDeliveryMode describes how sub2api can fulfill a public endpoint on a
// concrete account route. It is administrator-only routing metadata.
type ModelDeliveryMode string

const (
	ModelDeliveryModeCompatibility ModelDeliveryMode = "compatibility"
	ModelDeliveryModeNative        ModelDeliveryMode = "native"
	ModelDeliveryModeMixed         ModelDeliveryMode = "mixed"
)

// ModelDeliveryEndpoint is one public protocol that a concrete account route
// can fulfill. Source is capability evidence and must not be exposed to users.
type ModelDeliveryEndpoint struct {
	Protocol ModelProtocol
	Mode     ModelDeliveryMode
	Source   string
}

// ModelDeliveryRoute is the administrator projection from a public model to
// one persistently eligible account and its final upstream model.
type ModelDeliveryRoute struct {
	PublicModel        string
	GroupID            int64
	GroupName          string
	GroupPlatform      string
	AccountID          int64
	AccountName        string
	ChannelMappedModel string
	UpstreamModel      string
	Endpoints          []ModelDeliveryEndpoint
	Decisions          map[ModelProtocol]ModelDeliveryDecision
}

// ModelDeliveryGroupProjection aggregates stable routes for one public model
// inside one group. Transient saturation, cooldown and rate-limit state are
// deliberately excluded from this catalog projection.
type ModelDeliveryGroupProjection struct {
	PublicModel string
	GroupID     int64
	GroupName   string
	Platform    string
	Routes      []ModelDeliveryRoute
	Endpoints   map[ModelProtocol]ModelDeliveryMode
	Decisions   map[ModelProtocol]ModelDeliveryDecision
}

func (p *ModelDeliveryGroupProjection) Deliverable() bool {
	return p != nil && len(p.Endpoints) > 0
}

func (p *ModelDeliveryGroupProjection) StableRouteAvailable() bool {
	return p != nil && len(p.Routes) > 0
}

// Callable reports whether the group/model pair still has a route the runtime
// may select. Published endpoints are intentionally stricter: unknown
// capability evidence can retain the established compatibility path without
// being advertised as a proven endpoint.
func (p *ModelDeliveryGroupProjection) Callable(nativeRoutingEnabled bool) bool {
	if !p.StableRouteAvailable() {
		return false
	}
	if !nativeRoutingEnabled {
		return true
	}
	for _, route := range p.Routes {
		for _, decision := range route.Decisions {
			if decision.Eligible || modelDeliveryBlockedOnlyByCapabilityUnknown(decision) {
				return true
			}
		}
	}
	return false
}

// ModelDeliveryProjection is keyed by the exact public model ID and group ID.
// Callers must still intersect group IDs with their own authorization scope.
type ModelDeliveryProjection struct {
	Models               map[string]map[int64]*ModelDeliveryGroupProjection
	Warnings             []string
	NativeRoutingEnabled bool
}

func (p *ModelDeliveryProjection) Group(publicModel string, groupID int64) *ModelDeliveryGroupProjection {
	if p == nil || p.Models == nil {
		return nil
	}
	return p.Models[publicModel][groupID]
}

func (p *ModelDeliveryProjection) DeliverableGroupIDs(publicModel string) []int64 {
	if p == nil || p.Models == nil {
		return nil
	}
	groups := p.Models[publicModel]
	result := make([]int64, 0, len(groups))
	for groupID, group := range groups {
		if group.Deliverable() {
			result = append(result, groupID)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (p *ModelDeliveryProjection) StableRouteGroupIDs(publicModel string) []int64 {
	if p == nil || p.Models == nil {
		return nil
	}
	groups := p.Models[publicModel]
	result := make([]int64, 0, len(groups))
	for groupID, group := range groups {
		if group.StableRouteAvailable() {
			result = append(result, groupID)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (p *ModelDeliveryProjection) CallableGroupIDs(publicModel string) []int64 {
	if p == nil || p.Models == nil {
		return nil
	}
	groups := p.Models[publicModel]
	result := make([]int64, 0, len(groups))
	for groupID, group := range groups {
		if group.Callable(p.NativeRoutingEnabled) {
			result = append(result, groupID)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

func (p *ModelDeliveryProjection) EndpointGroupIDs(publicModel string, protocol ModelProtocol) []int64 {
	if p == nil || p.Models == nil {
		return nil
	}
	groups := p.Models[publicModel]
	result := make([]int64, 0, len(groups))
	for groupID, group := range groups {
		if group == nil {
			continue
		}
		if _, ok := group.Endpoints[protocol]; ok {
			result = append(result, groupID)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

type AccountPublicModelImpact struct {
	UpstreamModel string `json:"upstream_model"`
	PublicModel   string `json:"public_model"`
	ChannelID     int64  `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	GroupID       int64  `json:"group_id"`
	GroupName     string `json:"group_name"`
	Platform      string `json:"platform"`
}

// ModelDeliveryService owns the shared projection between channel catalog
// models, stable account routes, final upstream models and public endpoints.
// It stores no duplicate capability facts.
type ModelDeliveryService struct {
	accountRepo     AccountRepository
	groupRepo       GroupRepository
	channel         *ChannelService
	capability      *ModelProtocolCapabilityService
	routingSettings NativeModelProtocolRoutingSettingReader
	cfg             *config.Config
}

// SetNativeModelProtocolRoutingSettingReader attaches the runtime-backed global
// gate while preserving the constructor's config fallback for focused tests.
func (s *ModelDeliveryService) SetNativeModelProtocolRoutingSettingReader(settings NativeModelProtocolRoutingSettingReader) {
	if s != nil {
		s.routingSettings = settings
	}
}

func NewModelDeliveryService(
	accountRepo AccountRepository,
	groupRepo GroupRepository,
	channel *ChannelService,
	capability *ModelProtocolCapabilityService,
	cfg *config.Config,
) *ModelDeliveryService {
	return &ModelDeliveryService{
		accountRepo: accountRepo,
		groupRepo:   groupRepo,
		channel:     channel,
		capability:  capability,
		cfg:         cfg,
	}
}

// ResolveForGroups resolves the stable cross-product requested by catalog
// callers in one group/account/capability batch. Callers decide which model is
// actually published in which channel section before consuming the result.
func (s *ModelDeliveryService) ResolveForGroups(ctx context.Context, groupIDs []int64, models []string) (*ModelDeliveryProjection, error) {
	result := &ModelDeliveryProjection{Models: make(map[string]map[int64]*ModelDeliveryGroupProjection)}
	if s == nil || s.accountRepo == nil || s.groupRepo == nil {
		return result, nil
	}
	modelList := dedupeAndSortModelIDs(models)
	requestedGroups := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID > 0 {
			requestedGroups[groupID] = struct{}{}
		}
	}
	if len(requestedGroups) == 0 || len(modelList) == 0 {
		return result, nil
	}

	activeGroups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups for model delivery: %w", err)
	}
	groupsByID := make(map[int64]*Group, len(activeGroups))
	activeGroupIDs := make([]int64, 0, len(activeGroups))
	for i := range activeGroups {
		group := &activeGroups[i]
		if _, ok := requestedGroups[group.ID]; !ok || !group.IsActive() {
			continue
		}
		groupsByID[group.ID] = group
		activeGroupIDs = append(activeGroupIDs, group.ID)
	}
	if len(activeGroupIDs) == 0 {
		return result, nil
	}
	sort.Slice(activeGroupIDs, func(i, j int) bool { return activeGroupIDs[i] < activeGroupIDs[j] })

	accountIDs, err := s.groupRepo.GetAccountIDsByGroupIDs(ctx, activeGroupIDs)
	if err != nil {
		return nil, fmt.Errorf("list model delivery account ids: %w", err)
	}
	accounts, err := s.accountRepo.GetByIDs(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load model delivery accounts: %w", err)
	}
	accountsByGroup := make(map[int64][]*Account, len(activeGroupIDs))
	for _, account := range accounts {
		if account == nil || !isStableDeliveryAccount(account) {
			continue
		}
		for _, groupID := range activeGroupIDs {
			if accountBelongsToDeliveryGroup(account, groupID) {
				accountsByGroup[groupID] = append(accountsByGroup[groupID], account)
			}
		}
	}
	for groupID := range accountsByGroup {
		sort.SliceStable(accountsByGroup[groupID], func(i, j int) bool {
			return accountsByGroup[groupID][i].ID < accountsByGroup[groupID][j].ID
		})
	}

	nativeRoutingEnabled := s.nativeRoutingEnabled(ctx)
	result.NativeRoutingEnabled = nativeRoutingEnabled
	capabilitiesByAccount := make(map[int64][]AccountModelProtocolCapability)
	if nativeRoutingEnabled && s.capability != nil {
		capabilitiesByAccount, err = s.capability.listMany(ctx, accountIDs)
		if err != nil {
			slog.Warn("model_delivery_capability_evidence_unavailable", "error", err)
			result.Warnings = append(result.Warnings, "Native protocol evidence is temporarily unavailable; compatibility delivery is shown where it can still be proven")
			capabilitiesByAccount = make(map[int64][]AccountModelProtocolCapability)
		}
	}

	for _, model := range modelList {
		for _, groupID := range activeGroupIDs {
			group := groupsByID[groupID]
			projection := &ModelDeliveryGroupProjection{
				PublicModel: model,
				GroupID:     group.ID,
				GroupName:   group.Name,
				Platform:    group.Platform,
				Endpoints:   make(map[ModelProtocol]ModelDeliveryMode),
				Decisions:   make(map[ModelProtocol]ModelDeliveryDecision),
			}
			channelMapping, err := s.resolveChannelMapping(ctx, group.ID, model)
			if err != nil {
				return nil, fmt.Errorf("resolve channel model for group %d and model %q: %w", group.ID, model, err)
			}
			channelMappedModel := effectiveChannelMappedModel(model, channelMapping)
			messagesMappedModel := ResolveOpenAIMessagesDeliveryModel(group, model, channelMapping)
			for _, account := range accountsByGroup[groupID] {
				stableRoute := accountMatchesDeliveryPlatform(account, group.Platform) &&
					(accountSupportsDeliveryModel(account, channelMappedModel) ||
						(group.AllowMessagesDispatch && accountSupportsDeliveryModel(account, messagesMappedModel)))
				route := ModelDeliveryRoute{
					PublicModel:        model,
					GroupID:            group.ID,
					GroupName:          group.Name,
					GroupPlatform:      group.Platform,
					AccountID:          account.ID,
					AccountName:        account.Name,
					ChannelMappedModel: channelMappedModel,
					UpstreamModel:      resolveFinalDeliveryModel(account, channelMappedModel),
					Decisions:          make(map[ModelProtocol]ModelDeliveryDecision),
				}
				for _, protocol := range AllModelProtocols {
					protocolMappedModel := channelMappedModel
					if protocol == ModelProtocolAnthropicMessages {
						protocolMappedModel = messagesMappedModel
					}
					decision := EvaluateModelDeliveryCandidate(ModelDeliveryCandidateInput{
						Account:               account,
						PublicModel:           model,
						ChannelMappedModel:    protocolMappedModel,
						GroupPlatform:         group.Platform,
						AllowMessagesDispatch: group.AllowMessagesDispatch,
						InboundProtocol:       protocol,
						NativeRoutingEnabled:  nativeRoutingEnabled,
						Capabilities:          capabilitiesByAccount[account.ID],
					})
					route.Decisions[protocol] = decision
					projection.Decisions[protocol] = mergeModelDeliveryDecision(projection.Decisions[protocol], decision)
					if !decision.Eligible {
						continue
					}
					upsertDeliveryEndpoint(&route.Endpoints, ModelDeliveryEndpoint{
						Protocol: decision.InboundProtocol,
						Mode:     decision.Mode,
						Source:   decision.CapabilitySource,
					})
					projection.Endpoints[protocol] = mergeModelDeliveryMode(projection.Endpoints[protocol], decision.Mode)
				}
				if stableRoute {
					projection.Routes = append(projection.Routes, route)
				}
			}
			if len(projection.Routes) == 0 {
				for _, protocol := range AllModelProtocols {
					noRouteDecision := blockModelDeliveryDecision(ModelDeliveryDecision{
						PublicModel:     model,
						InboundProtocol: protocol,
					}, ModelDeliveryReasonNoStableRoute)
					projection.Decisions[protocol] = mergeModelDeliveryDecision(projection.Decisions[protocol], noRouteDecision)
				}
			}
			if result.Models[model] == nil {
				result.Models[model] = make(map[int64]*ModelDeliveryGroupProjection)
			}
			result.Models[model][groupID] = projection
		}
	}
	return result, nil
}

func (s *ModelDeliveryService) nativeRoutingEnabled(ctx context.Context) bool {
	return s != nil && nativeModelProtocolRoutingEnabled(ctx, s.routingSettings, s.cfg)
}

func (s *ModelDeliveryService) resolveChannelMapping(ctx context.Context, groupID int64, publicModel string) (ChannelMappingResult, error) {
	if s == nil || s.channel == nil {
		return ChannelMappingResult{MappedModel: strings.TrimSpace(publicModel)}, nil
	}
	return s.channel.ResolveChannelMappingStrict(ctx, groupID, publicModel)
}

func effectiveChannelMappedModel(publicModel string, mapping ChannelMappingResult) string {
	if mapped := strings.TrimSpace(mapping.MappedModel); mapped != "" {
		return mapped
	}
	return strings.TrimSpace(publicModel)
}

func mergeModelDeliveryMode(current, next ModelDeliveryMode) ModelDeliveryMode {
	if current == "" || current == next {
		return next
	}
	return ModelDeliveryModeMixed
}

func mergeModelDeliveryDecision(current, next ModelDeliveryDecision) ModelDeliveryDecision {
	if current.InboundProtocol == "" {
		return next
	}
	if next.Eligible {
		if !current.Eligible {
			return next
		}
		current.Mode = mergeModelDeliveryMode(current.Mode, next.Mode)
		if current.UpstreamProtocol != next.UpstreamProtocol {
			current.UpstreamProtocol = ""
		}
		if current.CapabilitySource != next.CapabilitySource {
			current.CapabilitySource = "mixed"
		}
		return current
	}
	if current.Eligible {
		return current
	}
	current.ReasonCodes = mergeModelDeliveryReasonCodes(current.ReasonCodes, next.ReasonCodes)
	return current
}

func upsertDeliveryEndpoint(items *[]ModelDeliveryEndpoint, endpoint ModelDeliveryEndpoint) {
	for i := range *items {
		if (*items)[i].Protocol != endpoint.Protocol {
			continue
		}
		if endpoint.Mode == ModelDeliveryModeNative {
			(*items)[i] = endpoint
		}
		return
	}
	*items = append(*items, endpoint)
	sort.SliceStable(*items, func(i, j int) bool {
		return modelProtocolSortIndex((*items)[i].Protocol) < modelProtocolSortIndex((*items)[j].Protocol)
	})
}

func modelProtocolSortIndex(protocol ModelProtocol) int {
	for index, candidate := range AllModelProtocols {
		if candidate == protocol {
			return index
		}
	}
	return len(AllModelProtocols)
}

func PublicPathForModelProtocol(protocol ModelProtocol) string {
	switch protocol {
	case ModelProtocolAnthropicMessages:
		return "/v1/messages"
	case ModelProtocolOpenAIChat:
		return "/v1/chat/completions"
	case ModelProtocolOpenAIResponses:
		return "/v1/responses"
	default:
		return ""
	}
}

func isStableDeliveryAccount(account *Account) bool {
	return account != nil && account.IsActive() && account.Schedulable
}

func accountBelongsToDeliveryGroup(account *Account, groupID int64) bool {
	if account == nil || groupID <= 0 {
		return false
	}
	for _, candidate := range account.GroupIDs {
		if candidate == groupID {
			return true
		}
	}
	for _, binding := range account.AccountGroups {
		if binding.GroupID == groupID {
			return true
		}
	}
	return false
}

func accountMatchesDeliveryPlatform(account *Account, platform string) bool {
	return account != nil && strings.TrimSpace(platform) != "" && account.Platform == platform
}

func accountSupportsDeliveryModel(account *Account, requestedModel string) bool {
	if account == nil {
		return false
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return false
	}
	if account.Platform == PlatformAntigravity {
		return mapAntigravityModel(account, requestedModel) != ""
	}
	if account.IsBedrock() {
		_, ok := ResolveBedrockModelID(account, requestedModel)
		return ok
	}
	if account.Platform == PlatformOpenAI && account.IsOpenAIPassthroughEnabled() {
		return true
	}
	if account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
		if account.Type == AccountTypeServiceAccount {
			requestedModel = normalizeVertexAnthropicModelID(claude.NormalizeModelID(requestedModel))
		} else {
			requestedModel = claude.NormalizeModelID(requestedModel)
		}
	}
	if account.Platform == PlatformOpenAI {
		requestedModel = NormalizeOpenAICompatRequestedModel(requestedModel)
	}
	return account.IsModelSupported(requestedModel)
}

func resolveFinalDeliveryModel(account *Account, requestedModel string) string {
	if account == nil {
		return requestedModel
	}
	if account.Platform == PlatformOpenAI {
		requestedModel = NormalizeOpenAICompatRequestedModel(requestedModel)
		return normalizeOpenAIModelForUpstream(account, resolveOpenAIForwardModel(account, requestedModel, ""))
	}
	if account.Platform == PlatformAntigravity {
		if mapped := mapAntigravityModel(account, requestedModel); mapped != "" {
			return mapped
		}
	}
	if account.IsBedrock() {
		if mapped, ok := ResolveBedrockModelID(account, requestedModel); ok {
			return mapped
		}
	}
	return account.GetMappedModel(requestedModel)
}

// ResolveAccountImpacts returns the public channel models that currently
// resolve to each final upstream model on one account. It is administrator-only
// diagnostic data and may include channel/account topology.
func (s *ModelDeliveryService) ResolveAccountImpacts(ctx context.Context, accountID int64) (map[string][]AccountPublicModelImpact, error) {
	result := make(map[string][]AccountPublicModelImpact)
	if s == nil || s.accountRepo == nil || s.channel == nil || accountID <= 0 {
		return result, nil
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	channels, err := s.channel.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	for _, channel := range channels {
		if channel.Status != StatusActive {
			continue
		}
		for _, group := range channel.Groups {
			if group.Platform != account.Platform || !accountBelongsToDeliveryGroup(account, group.ID) {
				continue
			}
			for _, model := range channel.SupportedModels {
				if model.Platform != group.Platform {
					continue
				}
				channelMapping, err := s.resolveChannelMapping(ctx, group.ID, model.Name)
				if err != nil {
					return nil, fmt.Errorf("resolve impacted channel model for group %d and model %q: %w", group.ID, model.Name, err)
				}
				deliveryModels := []string{effectiveChannelMappedModel(model.Name, channelMapping)}
				if group.AllowMessagesDispatch {
					deliveryModels = append(deliveryModels, ResolveOpenAIMessagesDeliveryModel(&Group{
						Platform:                    group.Platform,
						MessagesDispatchModelConfig: group.MessagesDispatchModelConfig,
					}, model.Name, channelMapping))
				}
				for _, mappedModel := range dedupeAndSortModelIDs(deliveryModels) {
					if !accountSupportsDeliveryModel(account, mappedModel) {
						continue
					}
					upstreamModel := resolveFinalDeliveryModel(account, mappedModel)
					key := fmt.Sprintf("%s\x00%d\x00%s", upstreamModel, group.ID, model.Name)
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					result[upstreamModel] = append(result[upstreamModel], AccountPublicModelImpact{
						UpstreamModel: upstreamModel,
						PublicModel:   model.Name,
						ChannelID:     channel.ID,
						ChannelName:   channel.Name,
						GroupID:       group.ID,
						GroupName:     group.Name,
						Platform:      group.Platform,
					})
				}
			}
		}
	}
	for upstreamModel := range result {
		sort.SliceStable(result[upstreamModel], func(i, j int) bool {
			left, right := result[upstreamModel][i], result[upstreamModel][j]
			if left.ChannelName != right.ChannelName {
				return left.ChannelName < right.ChannelName
			}
			if left.GroupName != right.GroupName {
				return left.GroupName < right.GroupName
			}
			return left.PublicModel < right.PublicModel
		})
	}
	return result, nil
}
