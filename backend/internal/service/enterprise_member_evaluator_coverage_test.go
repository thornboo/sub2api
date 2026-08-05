package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultEnterpriseMemberEvaluatorCoverageRegistryIsProductionComplete(t *testing.T) {
	report := EvaluateEnterpriseMemberEvaluatorCoverageRegistry(DefaultEnterpriseMemberEvaluatorCoverageRegistry())

	require.Truef(t, report.Ready, "report=%+v", report)
	require.Empty(t, report.Reason)
	require.Equal(t, EnterpriseMemberEvaluatorCoverageAlgorithmVersion, report.AlgorithmVersion)

	byFamily := enterpriseMemberEvaluatorCoverageItemsByFamily(report.Items)
	require.Equal(t, map[string]struct{}{
		EnterpriseMemberEndpointFamilyChat:                     {},
		EnterpriseMemberEndpointFamilyResponses:                {},
		EnterpriseMemberEndpointFamilyMessages:                 {},
		EnterpriseMemberEndpointFamilyEmbeddings:               {},
		EnterpriseMemberEndpointFamilyImages:                   {},
		EnterpriseMemberEndpointFamilyLive:                     {},
		EnterpriseMemberEndpointFamilyBatchImages:              {},
		EnterpriseMemberEndpointFamilyVideo:                    {},
		EnterpriseMemberEndpointFamilyGeminiNative:             {},
		EnterpriseMemberEndpointFamilyCompositeExplicitPreview: {},
	}, enterpriseMemberEvaluatorCoverageFamilySet(report.Items))

	require.Equal(t, ModelProtocolOpenAIChat, byFamily[EnterpriseMemberEndpointFamilyChat].Protocol)
	require.Equal(t, ModelProtocolOpenAIResponses, byFamily[EnterpriseMemberEndpointFamilyResponses].Protocol)
	require.Equal(t, ModelProtocolAnthropicMessages, byFamily[EnterpriseMemberEndpointFamilyMessages].Protocol)
	require.Equal(t, ModelProtocolOpenAIEmbeddings, byFamily[EnterpriseMemberEndpointFamilyEmbeddings].Protocol)
	require.Equal(t, ModelProtocolOpenAIImages, byFamily[EnterpriseMemberEndpointFamilyImages].Protocol)
	require.Equal(t, ModelProtocolOpenAILive, byFamily[EnterpriseMemberEndpointFamilyLive].Protocol)
	require.Equal(t, ModelProtocolBatchImages, byFamily[EnterpriseMemberEndpointFamilyBatchImages].Protocol)
	require.Equal(t, ModelProtocolGrokVideo, byFamily[EnterpriseMemberEndpointFamilyVideo].Protocol)
	require.Equal(t, ModelProtocolGeminiNative, byFamily[EnterpriseMemberEndpointFamilyGeminiNative].Protocol)
	require.True(t, byFamily[EnterpriseMemberEndpointFamilyCompositeExplicitPreview].CompositeExplicitPreview)
}

func TestEnterpriseMemberEvaluatorCoverageCompletenessTracksAllModelProtocols(t *testing.T) {
	report := EvaluateEnterpriseMemberEvaluatorCoverageRegistry(DefaultEnterpriseMemberEvaluatorCoverageRegistry())
	registered := make(map[ModelProtocol]struct{}, len(report.Items))
	for _, item := range report.Items {
		if item.Protocol != "" {
			registered[item.Protocol] = struct{}{}
		}
	}

	for _, protocol := range AllModelProtocols {
		require.Containsf(t, registered, protocol, "new protocol %q must be registered in enterprise member evaluator coverage", protocol)
	}
	require.Len(t, registered, len(AllModelProtocols))
}

func TestEnterpriseMemberEvaluatorCoverageRequiresPlannerEvaluatorAndTypedGate(t *testing.T) {
	registry := DefaultEnterpriseMemberEvaluatorCoverageRegistry()
	registry.Entries[0].SampleEndpoint = "/v1/not-a-planned-endpoint"
	delete(registry.StableDeliveryEvaluators, EnterpriseMemberEvaluatorIDCompositeExplicitPreview)
	delete(registry.TypedLocalGates, EnterpriseMemberTypedLocalGateOpenAIImagesCapability)
	delete(registry.CompositePreviewers, EnterpriseMemberEndpointFamilyCompositeExplicitPreview)

	report := EvaluateEnterpriseMemberEvaluatorCoverageRegistry(registry)

	require.False(t, report.Ready)
	require.Equal(t, EnterpriseMemberEvaluatorCoverageReasonIncomplete, report.Reason)
	require.Contains(t, enterpriseMemberEvaluatorCoverageReasonsForFamily(report, EnterpriseMemberEndpointFamilyChat), EnterpriseMemberEvaluatorCoverageReasonPlannerUnsupported)
	require.Contains(t, enterpriseMemberEvaluatorCoverageReasonsForFamily(report, EnterpriseMemberEndpointFamilyImages), EnterpriseMemberEvaluatorCoverageReasonTypedGateMissing)
	require.Contains(t, enterpriseMemberEvaluatorCoverageReasonsForFamily(report, EnterpriseMemberEndpointFamilyCompositeExplicitPreview), EnterpriseMemberEvaluatorCoverageReasonEvaluatorMissing)
	require.Contains(t, enterpriseMemberEvaluatorCoverageReasonsForFamily(report, EnterpriseMemberEndpointFamilyCompositeExplicitPreview), EnterpriseMemberEvaluatorCoverageReasonCompositePreviewMissing)
}

func TestEnterpriseMemberEvaluatorCoverageBlocksUnregisteredFamiliesAndProtocols(t *testing.T) {
	registry := DefaultEnterpriseMemberEvaluatorCoverageRegistry()
	filtered := registry.Entries[:0]
	for _, entry := range registry.Entries {
		if entry.EndpointFamily == EnterpriseMemberEndpointFamilyEmbeddings {
			continue
		}
		if entry.Protocol == ModelProtocolGeminiNative {
			continue
		}
		filtered = append(filtered, entry)
	}
	registry.Entries = filtered

	report := EvaluateEnterpriseMemberEvaluatorCoverageRegistry(registry)

	require.False(t, report.Ready)
	require.Contains(t, enterpriseMemberEvaluatorCoverageReasonsForFamily(report, EnterpriseMemberEndpointFamilyEmbeddings), EnterpriseMemberEvaluatorCoverageReasonEndpointMissing)
	require.Contains(t, enterpriseMemberEvaluatorCoverageReasonsForFamily(report, "protocol:"+string(ModelProtocolGeminiNative)), EnterpriseMemberEvaluatorCoverageReasonProtocolMissing)
}

func TestEnterpriseMemberReadinessProviderConsumesEvaluatorCoverageNotRoutingMirror(t *testing.T) {
	runtime := &readinessRuntimeFake{ready: true, mirrored: true}
	aliasSvc := &EnterpriseMemberAliasReviewService{repo: aliasReadinessRepoFake{summary: &EnterpriseMemberAliasReviewReadinessSummary{}}}
	provider := &enterpriseMemberModelAdmissionReadinessProviderImpl{
		runtime:        runtime,
		aliasReviewSvc: aliasSvc,
		coverage: enterpriseMemberEvaluatorCoverageProvider{registry: EnterpriseMemberEvaluatorCoverageRegistry{
			Entries: defaultEnterpriseMemberEvaluatorCoverageEntries(),
			StableDeliveryEvaluators: map[string]string{
				EnterpriseMemberEvaluatorIDModelDeliveryCandidate: EnterpriseMemberEvaluatorAlgorithmModelDeliveryCandidate,
			},
			TypedLocalGates:     DefaultEnterpriseMemberEvaluatorCoverageRegistry().TypedLocalGates,
			CompositePreviewers: DefaultEnterpriseMemberEvaluatorCoverageRegistry().CompositePreviewers,
		}},
	}

	readiness := provider.EvaluateEnterpriseMemberModelAdmissionReadiness(context.Background())

	require.False(t, readiness.Ready)
	condition := enterpriseMemberAdmissionConditionByName(readiness, enterpriseMemberModelAdmissionReadinessConditionEval)
	require.False(t, condition.Ready)
	require.Equal(t, EnterpriseMemberEvaluatorCoverageReasonIncomplete, condition.Reason)
	require.Contains(t, condition.Details, EnterpriseMemberEvaluatorCoverageAlgorithmVersion)
}

func enterpriseMemberEvaluatorCoverageItemsByFamily(items []EnterpriseMemberEvaluatorCoverageItem) map[string]EnterpriseMemberEvaluatorCoverageItem {
	result := make(map[string]EnterpriseMemberEvaluatorCoverageItem, len(items))
	for _, item := range items {
		result[item.EndpointFamily] = item
	}
	return result
}

func enterpriseMemberEvaluatorCoverageFamilySet(items []EnterpriseMemberEvaluatorCoverageItem) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		result[item.EndpointFamily] = struct{}{}
	}
	return result
}

func enterpriseMemberEvaluatorCoverageReasonsForFamily(report EnterpriseMemberEvaluatorCoverageReport, family string) []string {
	for _, item := range report.Items {
		if item.EndpointFamily == family {
			return item.Reasons
		}
	}
	return nil
}

func enterpriseMemberAdmissionConditionByName(readiness EnterpriseMemberModelAdmissionEnforceReadiness, name string) EnterpriseMemberModelAdmissionReadinessCondition {
	for _, condition := range readiness.Conditions {
		if condition.Name == name {
			return condition
		}
	}
	return EnterpriseMemberModelAdmissionReadinessCondition{}
}
