package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// ModelProtocol identifies an upstream request/response contract.
type ModelProtocol string

const (
	ModelProtocolAnthropicMessages ModelProtocol = "anthropic_messages"
	ModelProtocolOpenAIChat        ModelProtocol = "openai_chat_completions"
	ModelProtocolOpenAIResponses   ModelProtocol = "openai_responses"
	ModelProtocolWildcardModel                   = "*"
)

var AllModelProtocols = []ModelProtocol{
	ModelProtocolAnthropicMessages,
	ModelProtocolOpenAIChat,
	ModelProtocolOpenAIResponses,
}

type ModelProtocolState string

const (
	ModelProtocolStateAuto        ModelProtocolState = "auto"
	ModelProtocolStateUnknown     ModelProtocolState = "unknown"
	ModelProtocolStateSupported   ModelProtocolState = "supported"
	ModelProtocolStateUnsupported ModelProtocolState = "unsupported"
)

type AccountModelProtocolCapability struct {
	ID              int64              `json:"id"`
	AccountID       int64              `json:"account_id"`
	UpstreamModel   string             `json:"upstream_model"`
	Protocol        ModelProtocol      `json:"protocol"`
	OverrideState   ModelProtocolState `json:"override_state"`
	ObservedState   ModelProtocolState `json:"observed_state"`
	EffectiveState  ModelProtocolState `json:"effective_state"`
	EffectiveSource string             `json:"effective_source,omitempty"`
	ObservedSource  string             `json:"observed_source,omitempty"`
	ObservedAt      *time.Time         `json:"observed_at,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type ModelProtocolObservation struct {
	UpstreamModel string
	Protocol      ModelProtocol
	State         ModelProtocolState
	Source        string
	ObservedAt    time.Time
}

type ModelProtocolOverride struct {
	UpstreamModel string             `json:"upstream_model"`
	Protocol      ModelProtocol      `json:"protocol"`
	State         ModelProtocolState `json:"state"`
}

type ModelProtocolCapabilityValidationError struct {
	Message string
}

func (e *ModelProtocolCapabilityValidationError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func newModelProtocolValidationError(message string) error {
	return &ModelProtocolCapabilityValidationError{Message: message}
}

type ModelProtocolCapabilityRepository interface {
	ListByAccount(ctx context.Context, accountID int64) ([]AccountModelProtocolCapability, error)
	ListByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64][]AccountModelProtocolCapability, error)
	SyncObserved(ctx context.Context, accountID int64, observations []ModelProtocolObservation) error
	UpdateOverrides(ctx context.Context, accountID int64, overrides []ModelProtocolOverride) error
}

type ModelProtocolCapabilityService struct {
	repo            ModelProtocolCapabilityRepository
	accountRepo     AccountRepository
	groupRepo       GroupRepository
	channel         *ChannelService
	routingSettings NativeModelProtocolRoutingSettingReader
	cfg             *config.Config
	mu              sync.RWMutex
	cache           map[int64]modelProtocolCapabilityCacheEntry
}

type modelProtocolCapabilityCacheEntry struct {
	items     []AccountModelProtocolCapability
	expiresAt time.Time
}

const modelProtocolCapabilityCacheTTL = 5 * time.Second

func NewModelProtocolCapabilityService(repo ModelProtocolCapabilityRepository, accountRepo AccountRepository, groupRepo GroupRepository, channel *ChannelService, cfg *config.Config) *ModelProtocolCapabilityService {
	return &ModelProtocolCapabilityService{repo: repo, accountRepo: accountRepo, groupRepo: groupRepo, channel: channel, cfg: cfg, cache: make(map[int64]modelProtocolCapabilityCacheEntry)}
}

func (s *ModelProtocolCapabilityService) SetNativeModelProtocolRoutingSettingReader(settings NativeModelProtocolRoutingSettingReader) {
	if s != nil {
		s.routingSettings = settings
	}
}

func IsValidModelProtocol(protocol ModelProtocol) bool {
	for _, candidate := range AllModelProtocols {
		if protocol == candidate {
			return true
		}
	}
	return false
}

func IsValidModelProtocolOverride(state ModelProtocolState) bool {
	return state == ModelProtocolStateAuto || state == ModelProtocolStateSupported || state == ModelProtocolStateUnsupported
}

func (s *ModelProtocolCapabilityService) List(ctx context.Context, accountID int64) ([]AccountModelProtocolCapability, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("model protocol capability service is not configured")
	}
	s.mu.RLock()
	cached, ok := s.cache[accountID]
	s.mu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return append([]AccountModelProtocolCapability(nil), cached.items...), nil
	}
	items, err := s.repo.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].EffectiveState, items[i].EffectiveSource = resolveCapabilityFromItems(items, items[i].UpstreamModel, items[i].Protocol, false)
	}
	s.mu.Lock()
	if s.cache == nil {
		s.cache = make(map[int64]modelProtocolCapabilityCacheEntry)
	}
	s.cache[accountID] = modelProtocolCapabilityCacheEntry{items: append([]AccountModelProtocolCapability(nil), items...), expiresAt: time.Now().Add(modelProtocolCapabilityCacheTTL)}
	s.mu.Unlock()
	return append([]AccountModelProtocolCapability(nil), items...), nil
}

func (s *ModelProtocolCapabilityService) invalidate(accountID int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.cache, accountID)
	s.mu.Unlock()
}

func normalizeCapabilityTarget(model string, protocol ModelProtocol) (string, error) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", errors.New("upstream_model is required")
	}
	if utf8.RuneCountInString(model) > 255 {
		return "", errors.New("upstream_model must not exceed 255 characters")
	}
	if strings.IndexFunc(model, unicode.IsControl) >= 0 {
		return "", errors.New("upstream_model must not contain control characters")
	}
	if model != ModelProtocolWildcardModel && strings.Contains(model, "*") {
		return "", errors.New("only the exact model name or * is supported")
	}
	if !IsValidModelProtocol(protocol) {
		return "", fmt.Errorf("unsupported protocol: %s", protocol)
	}
	return model, nil
}

func (s *ModelProtocolCapabilityService) UpdateOverrides(ctx context.Context, accountID int64, overrides []ModelProtocolOverride) error {
	if s == nil || s.repo == nil {
		return errors.New("model protocol capability service is not configured")
	}
	if len(overrides) == 0 {
		return newModelProtocolValidationError("items is required")
	}
	normalized := make([]ModelProtocolOverride, 0, len(overrides))
	seen := make(map[string]struct{}, len(overrides))
	for _, item := range overrides {
		model, err := normalizeCapabilityTarget(item.UpstreamModel, item.Protocol)
		if err != nil {
			return newModelProtocolValidationError(err.Error())
		}
		if !IsValidModelProtocolOverride(item.State) {
			return newModelProtocolValidationError(fmt.Sprintf("unsupported override state: %s", item.State))
		}
		key := model + "\x00" + string(item.Protocol)
		if _, ok := seen[key]; ok {
			return newModelProtocolValidationError(fmt.Sprintf("duplicate capability override for %s and %s", model, item.Protocol))
		}
		seen[key] = struct{}{}
		item.UpstreamModel = model
		normalized = append(normalized, item)
	}
	if err := s.repo.UpdateOverrides(ctx, accountID, normalized); err != nil {
		return err
	}
	s.invalidate(accountID)
	return nil
}

type ModelProtocolCapabilitySyncResult struct {
	Models   []string `json:"models"`
	Warnings []string `json:"warnings"`
}

const (
	maxUnknownEndpointTypesPerWarning = 10
	maxWarningValueRunes              = 64
	maxCapabilitySyncWarnings         = 50
)

func warningValue(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	count := 0
	truncated := false
	for _, r := range value {
		if count >= maxWarningValueRunes {
			truncated = true
			break
		}
		if unicode.IsControl(r) {
			r = ' '
		}
		_, _ = b.WriteRune(r)
		count++
	}
	value = strings.TrimSpace(b.String())
	if value == "" {
		return "<empty>"
	}
	if truncated {
		return value + "..."
	}
	return value
}

func sanitizeUnknownEndpointTypes(values []string) ([]string, int) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = warningValue(value)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	total := len(result)
	if len(result) > maxUnknownEndpointTypesPerWarning {
		result = result[:maxUnknownEndpointTypesPerWarning]
	}
	return result, total
}

func boundCapabilitySyncWarnings(warnings []string) []string {
	if len(warnings) == 0 {
		return nil
	}
	sort.Strings(warnings)
	result := warnings[:0]
	for _, warning := range warnings {
		if len(result) > 0 && result[len(result)-1] == warning {
			continue
		}
		result = append(result, warning)
	}
	if len(result) <= maxCapabilitySyncWarnings {
		return result
	}
	remaining := len(result) - maxCapabilitySyncWarnings
	result = append(result[:maxCapabilitySyncWarnings], fmt.Sprintf("%d additional sync warnings were omitted", remaining))
	return result
}

func (s *ModelProtocolCapabilityService) SyncCatalog(ctx context.Context, accountID int64, catalog []UpstreamModelDescriptor) (*ModelProtocolCapabilitySyncResult, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("model protocol capability service is not configured")
	}
	models := make([]string, 0, len(catalog))
	observations := make([]ModelProtocolObservation, 0, len(catalog)*len(AllModelProtocols))
	warnings := make([]string, 0)
	now := time.Now().UTC()
	for _, descriptor := range catalog {
		model := strings.TrimSpace(descriptor.ID)
		if model == "" {
			continue
		}
		if model == ModelProtocolWildcardModel || strings.Contains(model, "*") || utf8.RuneCountInString(model) > 255 || strings.IndexFunc(model, unicode.IsControl) >= 0 {
			warnings = append(warnings, fmt.Sprintf("ignored invalid upstream model id: %s", warningValue(model)))
			continue
		}
		models = append(models, model)
		if !descriptor.EndpointTypesPresent {
			for _, protocol := range AllModelProtocols {
				observations = append(observations, ModelProtocolObservation{UpstreamModel: model, Protocol: protocol, State: ModelProtocolStateUnknown, Source: "upstream_model_list_missing", ObservedAt: now})
			}
			warnings = append(warnings, fmt.Sprintf("%s did not declare supported_endpoint_types; prior observations were preserved", warningValue(model)))
			continue
		}
		if len(descriptor.SupportedEndpointTypes) == 0 {
			for _, protocol := range AllModelProtocols {
				observations = append(observations, ModelProtocolObservation{UpstreamModel: model, Protocol: protocol, State: ModelProtocolStateUnknown, Source: "upstream_model_list_empty", ObservedAt: now})
			}
			warnings = append(warnings, fmt.Sprintf("%s declared an empty supported_endpoint_types array; prior observations were preserved", warningValue(model)))
			continue
		}
		recognized := make(map[ModelProtocol]struct{}, len(descriptor.SupportedEndpointTypes))
		unknown := make([]string, 0)
		for _, endpointType := range descriptor.SupportedEndpointTypes {
			protocol, ok := modelProtocolFromUpstreamEndpointType(endpointType)
			if !ok {
				unknown = append(unknown, strings.TrimSpace(endpointType))
				continue
			}
			recognized[protocol] = struct{}{}
		}
		for protocol := range recognized {
			observations = append(observations, ModelProtocolObservation{UpstreamModel: model, Protocol: protocol, State: ModelProtocolStateSupported, Source: "upstream_model_list", ObservedAt: now})
		}
		if len(unknown) == 0 && descriptor.EndpointTypesComplete {
			for _, protocol := range AllModelProtocols {
				if _, ok := recognized[protocol]; ok {
					continue
				}
				observations = append(observations, ModelProtocolObservation{UpstreamModel: model, Protocol: protocol, State: ModelProtocolStateUnsupported, Source: "upstream_model_list", ObservedAt: now})
			}
		} else if len(unknown) > 0 {
			for _, protocol := range AllModelProtocols {
				if _, ok := recognized[protocol]; ok {
					continue
				}
				observations = append(observations, ModelProtocolObservation{UpstreamModel: model, Protocol: protocol, State: ModelProtocolStateUnknown, Source: "upstream_unknown_values", ObservedAt: now})
			}
			unknown, totalUnknown := sanitizeUnknownEndpointTypes(unknown)
			warning := fmt.Sprintf("%s declared unknown endpoint types: %s", warningValue(model), strings.Join(unknown, ", "))
			if totalUnknown > len(unknown) {
				warning += fmt.Sprintf(" (+%d more)", totalUnknown-len(unknown))
			}
			warnings = append(warnings, warning)
		}
	}
	models = dedupeAndSortModelIDs(models)
	warnings = boundCapabilitySyncWarnings(warnings)
	if err := s.repo.SyncObserved(ctx, accountID, observations); err != nil {
		return nil, err
	}
	s.invalidate(accountID)
	return &ModelProtocolCapabilitySyncResult{Models: models, Warnings: warnings}, nil
}

func modelProtocolFromUpstreamEndpointType(endpointType string) (ModelProtocol, bool) {
	switch strings.ToLower(strings.TrimSpace(endpointType)) {
	case "anthropic":
		return ModelProtocolAnthropicMessages, true
	case "openai":
		return ModelProtocolOpenAIChat, true
	case "openai-response":
		return ModelProtocolOpenAIResponses, true
	default:
		return "", false
	}
}

func UpstreamEndpointTypeForModelProtocol(protocol ModelProtocol) (string, bool) {
	switch protocol {
	case ModelProtocolAnthropicMessages:
		return "anthropic", true
	case ModelProtocolOpenAIChat:
		return "openai", true
	case ModelProtocolOpenAIResponses:
		return "openai-response", true
	default:
		return "", false
	}
}

// Resolve returns the six-level exact/wildcard result. Intrinsic support is only
// considered after stored manual and observed facts.
func (s *ModelProtocolCapabilityService) Resolve(ctx context.Context, accountID int64, upstreamModel string, protocol ModelProtocol, intrinsicSupported bool) (ModelProtocolState, string, error) {
	items, err := s.List(ctx, accountID)
	if err != nil {
		return ModelProtocolStateUnknown, "", err
	}
	upstreamModel = strings.TrimSpace(upstreamModel)
	var exact, wildcard *AccountModelProtocolCapability
	for i := range items {
		item := &items[i]
		if item.Protocol != protocol {
			continue
		}
		switch item.UpstreamModel {
		case upstreamModel:
			exact = item
		case ModelProtocolWildcardModel:
			wildcard = item
		}
	}
	for _, candidate := range []*AccountModelProtocolCapability{exact, wildcard} {
		if candidate != nil && (candidate.OverrideState == ModelProtocolStateSupported || candidate.OverrideState == ModelProtocolStateUnsupported) {
			return candidate.OverrideState, "admin_override", nil
		}
	}
	for _, candidate := range []*AccountModelProtocolCapability{exact, wildcard} {
		if candidate != nil && (candidate.ObservedState == ModelProtocolStateSupported || candidate.ObservedState == ModelProtocolStateUnsupported) {
			return candidate.ObservedState, candidate.ObservedSource, nil
		}
	}
	if intrinsicSupported {
		return ModelProtocolStateSupported, "intrinsic", nil
	}
	return ModelProtocolStateUnknown, "", nil
}

func (s *ModelProtocolCapabilityService) Supports(ctx context.Context, account *Account, upstreamModel string, protocol ModelProtocol) (bool, string, error) {
	if account == nil {
		return false, "", nil
	}
	intrinsic := accountIntrinsicProtocolSupport(account, protocol)
	state, source, err := s.Resolve(ctx, account.ID, upstreamModel, protocol, intrinsic)
	return state == ModelProtocolStateSupported, source, err
}

// ResolveNativeProtocolsForGroups batches public model capability aggregation.
// The result is keyed by public model, then protocol, with only visible group IDs.
func (s *ModelProtocolCapabilityService) ResolveNativeProtocolsForGroups(ctx context.Context, groupIDs []int64, models []string) (map[string]map[ModelProtocol][]int64, error) {
	result := make(map[string]map[ModelProtocol][]int64)
	if s == nil || s.accountRepo == nil || s.groupRepo == nil {
		return result, nil
	}
	delivery := NewModelDeliveryService(s.accountRepo, s.groupRepo, s.channel, s, s.cfg)
	delivery.SetNativeModelProtocolRoutingSettingReader(s.routingSettings)
	projection, err := delivery.ResolveForGroups(ctx, groupIDs, models)
	if err != nil {
		return nil, err
	}
	for _, model := range dedupeAndSortModelIDs(models) {
		for _, groupID := range groupIDs {
			group := projection.Group(model, groupID)
			if group == nil {
				continue
			}
			for _, route := range group.Routes {
				for _, endpoint := range route.Endpoints {
					if endpoint.Mode != ModelDeliveryModeNative {
						continue
					}
					if result[model] == nil {
						result[model] = make(map[ModelProtocol][]int64)
					}
					if !containsInt64(result[model][endpoint.Protocol], groupID) {
						result[model][endpoint.Protocol] = append(result[model][endpoint.Protocol], groupID)
					}
				}
			}
		}
	}
	for _, protocols := range result {
		for protocol := range protocols {
			sort.Slice(protocols[protocol], func(i, j int) bool { return protocols[protocol][i] < protocols[protocol][j] })
		}
	}
	return result, nil
}

func (s *ModelProtocolCapabilityService) listMany(ctx context.Context, accountIDs []int64) (map[int64][]AccountModelProtocolCapability, error) {
	result := make(map[int64][]AccountModelProtocolCapability, len(accountIDs))
	missing := make([]int64, 0, len(accountIDs))
	now := time.Now()
	s.mu.RLock()
	for _, accountID := range accountIDs {
		if cached, ok := s.cache[accountID]; ok && now.Before(cached.expiresAt) {
			result[accountID] = append([]AccountModelProtocolCapability(nil), cached.items...)
			continue
		}
		missing = append(missing, accountID)
	}
	s.mu.RUnlock()
	if len(missing) == 0 {
		return result, nil
	}
	loaded, err := s.repo.ListByAccountIDs(ctx, missing)
	if err != nil {
		return nil, fmt.Errorf("list model protocol capabilities for catalog: %w", err)
	}
	expiresAt := time.Now().Add(modelProtocolCapabilityCacheTTL)
	s.mu.Lock()
	if s.cache == nil {
		s.cache = make(map[int64]modelProtocolCapabilityCacheEntry)
	}
	for _, accountID := range missing {
		items := append([]AccountModelProtocolCapability(nil), loaded[accountID]...)
		for i := range items {
			items[i].EffectiveState, items[i].EffectiveSource = resolveCapabilityFromItems(items, items[i].UpstreamModel, items[i].Protocol, false)
		}
		result[accountID] = items
		s.cache[accountID] = modelProtocolCapabilityCacheEntry{items: append([]AccountModelProtocolCapability(nil), items...), expiresAt: expiresAt}
	}
	s.mu.Unlock()
	return result, nil
}

func resolveCapabilityFromItems(items []AccountModelProtocolCapability, upstreamModel string, protocol ModelProtocol, intrinsic bool) (ModelProtocolState, string) {
	var exact, wildcard *AccountModelProtocolCapability
	for i := range items {
		item := &items[i]
		if item.Protocol != protocol {
			continue
		}
		switch item.UpstreamModel {
		case upstreamModel:
			exact = item
		case ModelProtocolWildcardModel:
			wildcard = item
		}
	}
	for _, candidate := range []*AccountModelProtocolCapability{exact, wildcard} {
		if candidate != nil && (candidate.OverrideState == ModelProtocolStateSupported || candidate.OverrideState == ModelProtocolStateUnsupported) {
			return candidate.OverrideState, "admin_override"
		}
	}
	for _, candidate := range []*AccountModelProtocolCapability{exact, wildcard} {
		if candidate != nil && (candidate.ObservedState == ModelProtocolStateSupported || candidate.ObservedState == ModelProtocolStateUnsupported) {
			return candidate.ObservedState, candidate.ObservedSource
		}
	}
	if intrinsic {
		return ModelProtocolStateSupported, "intrinsic"
	}
	return ModelProtocolStateUnknown, ""
}

func accountIntrinsicProtocolSupport(account *Account, protocol ModelProtocol) bool {
	if account == nil {
		return false
	}
	if protocol == ModelProtocolAnthropicMessages && account.IsAnthropic() && !account.IsBedrock() && account.Type != AccountTypeServiceAccount {
		return true
	}
	return protocol == ModelProtocolOpenAIResponses && account.IsOpenAI() && account.Type == AccountTypeOAuth
}
