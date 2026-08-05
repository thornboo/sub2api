package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	EnterpriseMemberEvaluatorCoverageAlgorithmVersion = "enterprise_member_evaluator_coverage_v1"

	EnterpriseMemberEvaluatorCoverageReasonIncomplete              = "evaluator_coverage_incomplete"
	EnterpriseMemberEvaluatorCoverageReasonEndpointMissing         = "endpoint_family_not_registered"
	EnterpriseMemberEvaluatorCoverageReasonDuplicateEndpoint       = "duplicate_endpoint_family"
	EnterpriseMemberEvaluatorCoverageReasonProtocolMissing         = "protocol_not_registered"
	EnterpriseMemberEvaluatorCoverageReasonPlannerUnsupported      = "planner_protocol_unsupported"
	EnterpriseMemberEvaluatorCoverageReasonPlannerProtocolMismatch = "planner_protocol_mismatch"
	EnterpriseMemberEvaluatorCoverageReasonEvaluatorMissing        = "stable_delivery_evaluator_not_registered"
	EnterpriseMemberEvaluatorCoverageReasonAlgorithmMismatch       = "stable_delivery_evaluator_algorithm_mismatch"
	EnterpriseMemberEvaluatorCoverageReasonTypedGateMissing        = "typed_local_gate_not_registered"
	EnterpriseMemberEvaluatorCoverageReasonCompositePreviewMissing = "composite_explicit_preview_not_registered"
)

const (
	EnterpriseMemberEndpointFamilyChat                     = "chat"
	EnterpriseMemberEndpointFamilyResponses                = "responses"
	EnterpriseMemberEndpointFamilyMessages                 = "messages"
	EnterpriseMemberEndpointFamilyEmbeddings               = "embeddings"
	EnterpriseMemberEndpointFamilyImages                   = "images"
	EnterpriseMemberEndpointFamilyLive                     = "live"
	EnterpriseMemberEndpointFamilyBatchImages              = "batch_images"
	EnterpriseMemberEndpointFamilyVideo                    = "video"
	EnterpriseMemberEndpointFamilyGeminiNative             = "gemini_native"
	EnterpriseMemberEndpointFamilyCompositeExplicitPreview = "composite_explicit_preview"
)

const (
	EnterpriseMemberEvaluatorIDModelDeliveryCandidate   = "model_delivery_candidate"
	EnterpriseMemberEvaluatorIDCompositeExplicitPreview = "composite_explicit_preview"

	EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate   = "model_delivery_candidate_v2"
	EnterpriseMemberEvaluatorAlgorithmCompositeExplicitPreview = "composite_route_explicit_preview_v1"
)

const (
	EnterpriseMemberTypedLocalGateGroupAttemptResult         = "group_attempt_result_closed_reason"
	EnterpriseMemberTypedLocalGateImageGenerationIntent      = "responses_explicit_image_generation_intent"
	EnterpriseMemberTypedLocalGateOpenAIImagesCapability     = "openai_images_capability_mismatch"
	EnterpriseMemberTypedLocalGateOpenAIEmbeddingsCapability = "openai_embeddings_capability_mismatch"
	EnterpriseMemberTypedLocalGateOpenAILiveCapability       = "openai_live_capability_mismatch"
	EnterpriseMemberTypedLocalGateBatchImagesCapability      = "batch_images_capability_mismatch"
	EnterpriseMemberTypedLocalGateGrokVideoCapability        = "grok_video_capability_mismatch"
	EnterpriseMemberTypedLocalGateGeminiNativeCapability     = "gemini_native_capability_mismatch"
	EnterpriseMemberTypedLocalGateCompositeRoutePreview      = "composite_explicit_route_preview"
)

// EnterpriseMemberEvaluatorCoverageEntry is one production readiness row for a
// model-aware routing surface. It is intentionally static and low-cardinality:
// it describes endpoint families, protocols, platforms and evaluator contracts,
// never models, members, groups or routing mirror state.
type EnterpriseMemberEvaluatorCoverageEntry struct {
	EndpointFamily           string
	SampleEndpoint           string
	Protocol                 ModelProtocol
	Platforms                []string
	Intent                   EnterpriseMemberRouteIntentProfile
	EvaluatorID              string
	EvaluatorAlgorithm       string
	RequiredTypedLocalGates  []string
	CompositeExplicitPreview bool
}

type EnterpriseMemberEvaluatorCoverageItem struct {
	EnterpriseMemberEvaluatorCoverageEntry
	Ready   bool     `json:"ready"`
	Reasons []string `json:"reasons,omitempty"`
}

type EnterpriseMemberEvaluatorCoverageReport struct {
	Ready            bool                                    `json:"ready"`
	Reason           string                                  `json:"reason,omitempty"`
	AlgorithmVersion string                                  `json:"algorithm_version"`
	Items            []EnterpriseMemberEvaluatorCoverageItem `json:"items"`
}

type EnterpriseMemberEvaluatorCoverageProvider interface {
	EvaluateEnterpriseMemberEvaluatorCoverage(ctx context.Context) EnterpriseMemberEvaluatorCoverageReport
}

type enterpriseMemberEvaluatorCoverageProvider struct {
	registry EnterpriseMemberEvaluatorCoverageRegistry
}

type EnterpriseMemberEvaluatorCoverageRegistry struct {
	Entries                  []EnterpriseMemberEvaluatorCoverageEntry
	StableDeliveryEvaluators map[string]string
	TypedLocalGates          map[string]struct{}
	CompositePreviewers      map[string]string
}

func NewEnterpriseMemberEvaluatorCoverageProvider() EnterpriseMemberEvaluatorCoverageProvider {
	return enterpriseMemberEvaluatorCoverageProvider{registry: DefaultEnterpriseMemberEvaluatorCoverageRegistry()}
}

func (p enterpriseMemberEvaluatorCoverageProvider) EvaluateEnterpriseMemberEvaluatorCoverage(context.Context) EnterpriseMemberEvaluatorCoverageReport {
	return EvaluateEnterpriseMemberEvaluatorCoverageRegistry(p.registry)
}

func DefaultEnterpriseMemberEvaluatorCoverageRegistry() EnterpriseMemberEvaluatorCoverageRegistry {
	return EnterpriseMemberEvaluatorCoverageRegistry{
		Entries: defaultEnterpriseMemberEvaluatorCoverageEntries(),
		StableDeliveryEvaluators: map[string]string{
			EnterpriseMemberEvaluatorIDModelDeliveryCandidate:   EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			EnterpriseMemberEvaluatorIDCompositeExplicitPreview: EnterpriseMemberEvaluatorAlgorithmCompositeExplicitPreview,
		},
		TypedLocalGates: map[string]struct{}{
			EnterpriseMemberTypedLocalGateGroupAttemptResult:         {},
			EnterpriseMemberTypedLocalGateImageGenerationIntent:      {},
			EnterpriseMemberTypedLocalGateOpenAIImagesCapability:     {},
			EnterpriseMemberTypedLocalGateOpenAIEmbeddingsCapability: {},
			EnterpriseMemberTypedLocalGateOpenAILiveCapability:       {},
			EnterpriseMemberTypedLocalGateBatchImagesCapability:      {},
			EnterpriseMemberTypedLocalGateGrokVideoCapability:        {},
			EnterpriseMemberTypedLocalGateGeminiNativeCapability:     {},
			EnterpriseMemberTypedLocalGateCompositeRoutePreview:      {},
		},
		CompositePreviewers: map[string]string{
			EnterpriseMemberEndpointFamilyCompositeExplicitPreview: EnterpriseMemberEvaluatorAlgorithmCompositeExplicitPreview,
		},
	}
}

func defaultEnterpriseMemberEvaluatorCoverageEntries() []EnterpriseMemberEvaluatorCoverageEntry {
	return []EnterpriseMemberEvaluatorCoverageEntry{
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyChat, SampleEndpoint: "/v1/chat/completions",
			Protocol: ModelProtocolOpenAIChat, Platforms: []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformGrok},
			Intent: EnterpriseMemberRouteIntentText, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyResponses, SampleEndpoint: "/v1/responses",
			Protocol: ModelProtocolOpenAIResponses, Platforms: []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity, PlatformGrok},
			Intent: EnterpriseMemberRouteIntentText, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateImageGenerationIntent},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyMessages, SampleEndpoint: "/v1/messages",
			Protocol: ModelProtocolAnthropicMessages, Platforms: []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini, PlatformAntigravity},
			Intent: EnterpriseMemberRouteIntentMessages, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyEmbeddings, SampleEndpoint: "/v1/embeddings",
			Protocol: ModelProtocolOpenAIEmbeddings, Platforms: []string{PlatformOpenAI},
			Intent: EnterpriseMemberRouteIntentText, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateOpenAIEmbeddingsCapability},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyImages, SampleEndpoint: "/v1/images/generations",
			Protocol: ModelProtocolOpenAIImages, Platforms: []string{PlatformOpenAI, PlatformGrok},
			Intent: EnterpriseMemberRouteIntentImage, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateOpenAIImagesCapability},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyLive, SampleEndpoint: "/v1/live",
			Protocol: ModelProtocolOpenAILive, Platforms: []string{PlatformOpenAI},
			Intent: EnterpriseMemberRouteIntentText, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateOpenAILiveCapability},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyBatchImages, SampleEndpoint: "/v1/images/batches",
			Protocol: ModelProtocolBatchImages, Platforms: []string{PlatformGemini},
			Intent: EnterpriseMemberRouteIntentImage, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateBatchImagesCapability},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyVideo, SampleEndpoint: "/v1/videos/generations",
			Protocol: ModelProtocolGrokVideo, Platforms: []string{PlatformGrok},
			Intent: EnterpriseMemberRouteIntentVideo, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateGrokVideoCapability},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyGeminiNative, SampleEndpoint: "/v1beta/models/gemini-2.5-pro:generateContent",
			Protocol: ModelProtocolGeminiNative, Platforms: []string{PlatformGemini, PlatformAntigravity},
			Intent: EnterpriseMemberRouteIntentText, EvaluatorID: EnterpriseMemberEvaluatorIDModelDeliveryCandidate,
			EvaluatorAlgorithm:      EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			RequiredTypedLocalGates: []string{EnterpriseMemberTypedLocalGateGroupAttemptResult, EnterpriseMemberTypedLocalGateGeminiNativeCapability},
		},
		{
			EndpointFamily: EnterpriseMemberEndpointFamilyCompositeExplicitPreview, SampleEndpoint: "/v1/responses",
			Protocol: ModelProtocolOpenAIResponses, Platforms: []string{PlatformComposite},
			Intent: EnterpriseMemberRouteIntentText, EvaluatorID: EnterpriseMemberEvaluatorIDCompositeExplicitPreview,
			EvaluatorAlgorithm:       EnterpriseMemberEvaluatorAlgorithmCompositeExplicitPreview,
			RequiredTypedLocalGates:  []string{EnterpriseMemberTypedLocalGateCompositeRoutePreview},
			CompositeExplicitPreview: true,
		},
	}
}

func EvaluateEnterpriseMemberEvaluatorCoverageRegistry(registry EnterpriseMemberEvaluatorCoverageRegistry) EnterpriseMemberEvaluatorCoverageReport {
	entries := append([]EnterpriseMemberEvaluatorCoverageEntry(nil), registry.Entries...)
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].EndpointFamily < entries[j].EndpointFamily
	})

	report := EnterpriseMemberEvaluatorCoverageReport{
		Ready:            true,
		AlgorithmVersion: EnterpriseMemberEvaluatorCoverageAlgorithmVersion,
		Items:            make([]EnterpriseMemberEvaluatorCoverageItem, 0, len(entries)),
	}

	seenFamilies := make(map[string]struct{}, len(entries))
	registeredProtocols := make(map[ModelProtocol]struct{}, len(entries))
	for _, entry := range entries {
		item := EnterpriseMemberEvaluatorCoverageItem{EnterpriseMemberEvaluatorCoverageEntry: normalizeEnterpriseMemberEvaluatorCoverageEntry(entry), Ready: true}
		if item.EndpointFamily == "" {
			item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonEndpointMissing)
		} else if _, ok := seenFamilies[item.EndpointFamily]; ok {
			item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonDuplicateEndpoint)
		} else {
			seenFamilies[item.EndpointFamily] = struct{}{}
		}
		if item.Protocol == "" {
			item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonProtocolMissing)
		} else {
			registeredProtocols[item.Protocol] = struct{}{}
			if !SupportsEnterpriseMemberRoutePlanning(item.SampleEndpoint) {
				item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonPlannerUnsupported)
			} else if got := normalizeEnterpriseMemberRouteProtocol("", item.SampleEndpoint); got != item.Protocol {
				item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonPlannerProtocolMismatch)
			}
		}
		if got := strings.TrimSpace(registry.StableDeliveryEvaluators[item.EvaluatorID]); got == "" {
			item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonEvaluatorMissing)
		} else if got != item.EvaluatorAlgorithm {
			item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonAlgorithmMismatch)
		}
		for _, gate := range item.RequiredTypedLocalGates {
			if _, ok := registry.TypedLocalGates[gate]; !ok {
				item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonTypedGateMissing)
			}
		}
		if item.CompositeExplicitPreview {
			if got := strings.TrimSpace(registry.CompositePreviewers[item.EndpointFamily]); got == "" {
				item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonCompositePreviewMissing)
			} else if got != item.EvaluatorAlgorithm {
				item.Reasons = append(item.Reasons, EnterpriseMemberEvaluatorCoverageReasonAlgorithmMismatch)
			}
		}
		item.Reasons = normalizeEnterpriseMemberEvaluatorCoverageReasons(item.Reasons)
		item.Ready = len(item.Reasons) == 0
		if !item.Ready {
			report.Ready = false
		}
		report.Items = append(report.Items, item)
	}

	for _, required := range requiredEnterpriseMemberEvaluatorCoverageFamilies() {
		if _, ok := seenFamilies[required]; ok {
			continue
		}
		report.Ready = false
		report.Items = append(report.Items, EnterpriseMemberEvaluatorCoverageItem{
			EnterpriseMemberEvaluatorCoverageEntry: EnterpriseMemberEvaluatorCoverageEntry{EndpointFamily: required},
			Ready:                                  false,
			Reasons:                                []string{EnterpriseMemberEvaluatorCoverageReasonEndpointMissing},
		})
	}
	for _, protocol := range AllModelProtocols {
		if _, ok := registeredProtocols[protocol]; ok {
			continue
		}
		report.Ready = false
		report.Items = append(report.Items, EnterpriseMemberEvaluatorCoverageItem{
			EnterpriseMemberEvaluatorCoverageEntry: EnterpriseMemberEvaluatorCoverageEntry{
				EndpointFamily: fmt.Sprintf("protocol:%s", protocol),
				Protocol:       protocol,
			},
			Ready:   false,
			Reasons: []string{EnterpriseMemberEvaluatorCoverageReasonProtocolMissing},
		})
	}
	if !report.Ready {
		report.Reason = EnterpriseMemberEvaluatorCoverageReasonIncomplete
	}
	return report
}

func requiredEnterpriseMemberEvaluatorCoverageFamilies() []string {
	return []string{
		EnterpriseMemberEndpointFamilyChat,
		EnterpriseMemberEndpointFamilyResponses,
		EnterpriseMemberEndpointFamilyMessages,
		EnterpriseMemberEndpointFamilyEmbeddings,
		EnterpriseMemberEndpointFamilyImages,
		EnterpriseMemberEndpointFamilyLive,
		EnterpriseMemberEndpointFamilyBatchImages,
		EnterpriseMemberEndpointFamilyVideo,
		EnterpriseMemberEndpointFamilyGeminiNative,
		EnterpriseMemberEndpointFamilyCompositeExplicitPreview,
	}
}

func normalizeEnterpriseMemberEvaluatorCoverageEntry(entry EnterpriseMemberEvaluatorCoverageEntry) EnterpriseMemberEvaluatorCoverageEntry {
	entry.EndpointFamily = strings.TrimSpace(entry.EndpointFamily)
	entry.SampleEndpoint = strings.TrimSpace(entry.SampleEndpoint)
	entry.EvaluatorID = strings.TrimSpace(entry.EvaluatorID)
	entry.EvaluatorAlgorithm = strings.TrimSpace(entry.EvaluatorAlgorithm)
	entry.Platforms = normalizeEnterpriseMemberEvaluatorCoverageStrings(entry.Platforms)
	entry.RequiredTypedLocalGates = normalizeEnterpriseMemberEvaluatorCoverageStrings(entry.RequiredTypedLocalGates)
	return entry
}

func normalizeEnterpriseMemberEvaluatorCoverageStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func normalizeEnterpriseMemberEvaluatorCoverageReasons(values []string) []string {
	return normalizeEnterpriseMemberEvaluatorCoverageStrings(values)
}
