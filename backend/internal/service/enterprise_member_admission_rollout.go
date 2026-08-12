package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"sync/atomic"
	"time"
)

const (
	EnterpriseMemberModelAdmissionEnforceBlockedReason        = "phase3_prerequisites_incomplete"
	EnterpriseMemberModelAdmissionRolloutMissReason           = "rollout_not_matched"
	EnterpriseMemberModelAdmissionRolloutNoTargetReason       = "rollout_no_enforce_target"
	EnterpriseMemberModelAdmissionAutoStopReason              = "rollout_auto_stop"
	EnterpriseMemberModelAdmissionRolloutInvalidReason        = "rollout_policy_invalid"
	EnterpriseMemberModelAdmissionDefaultReadinessSource      = "server_default"
	EnterpriseMemberModelAdmissionInjectedReadinessSource     = "injected_provider"
	enterpriseMemberModelAdmissionDefaultRolloutPolicySalt    = "enterprise-member-model-admission-v1"
	enterpriseMemberModelAdmissionReadinessConditionRevision  = "routing_revision_healthy"
	enterpriseMemberModelAdmissionReadinessConditionEval      = "evaluator_coverage_verified"
	enterpriseMemberModelAdmissionReadinessConditionAlias     = "alias_audit_clear"
	enterpriseMemberModelAdmissionReadinessConditionEvidence  = "evidence_pipeline_healthy"
	enterpriseMemberModelAdmissionReadinessConditionExpansion = "expansion_evidence_verified"
	enterpriseMemberModelAdmissionReadinessConditionStop      = "stop_clear"
	enterpriseMemberModelAdmissionMaxRolloutIDsPerList        = 1024
	enterpriseMemberModelAdmissionMaxRolloutExplicitTargets   = 1536
	enterpriseMemberModelAdmissionMaxRolloutSaltBytes         = 128
	enterpriseMemberModelAdmissionMaxRolloutPolicyBytes       = 30000
)

type EnterpriseMemberModelAdmissionReadinessCondition struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Layer   string `json:"layer,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Source  string `json:"source,omitempty"`
	Details string `json:"details,omitempty"`
}

// EnterpriseMemberModelAdmissionEnforceReadiness is the server-authoritative
// rollout gate for the final admission mode. UI confirmation is advisory only.
type EnterpriseMemberModelAdmissionEnforceReadiness struct {
	Ready           bool                                               `json:"ready"`
	Reason          string                                             `json:"reason"`
	Source          string                                             `json:"source"`
	FoundationReady bool                                               `json:"foundation_ready"`
	CanaryReady     bool                                               `json:"canary_ready"`
	ExpansionReady  bool                                               `json:"expansion_ready"`
	AutoStopped     bool                                               `json:"auto_stopped"`
	AutoStop        EnterpriseMemberModelAdmissionAutoStopState        `json:"auto_stop"`
	Conditions      []EnterpriseMemberModelAdmissionReadinessCondition `json:"conditions"`
	Snapshot        EnterpriseMemberModelAdmissionGateSnapshot         `json:"snapshot"`
}

type EnterpriseMemberModelAdmissionReadinessProvider interface {
	EvaluateEnterpriseMemberModelAdmissionReadiness(ctx context.Context) EnterpriseMemberModelAdmissionEnforceReadiness
}

type EnterpriseMemberModelAdmissionEligibilityRuntime interface {
	Ready() bool
	MirroredVersion(scopes []RoutingEligibilityScope) (RoutingEligibilityVersion, bool)
}

type enterpriseMemberModelAdmissionReadinessProviderImpl struct {
	runtime                       EnterpriseMemberModelAdmissionEligibilityRuntime
	aliasReviewSvc                *EnterpriseMemberAliasReviewService
	evidenceSvc                   *EnterpriseMemberAdmissionEvidenceService
	coverage                      EnterpriseMemberEvaluatorCoverageProvider
	autoStopEvidence              EnterpriseMemberModelAdmissionAutoStopEvidenceProvider
	autoStopUsesAdmissionEvidence bool
}

func NewEnterpriseMemberModelAdmissionReadinessProvider(
	runtime EnterpriseMemberModelAdmissionEligibilityRuntime,
	aliasReviewSvc *EnterpriseMemberAliasReviewService,
	optional ...any,
) EnterpriseMemberModelAdmissionReadinessProvider {
	var evidence EnterpriseMemberModelAdmissionAutoStopEvidenceProvider
	var admissionEvidence *EnterpriseMemberAdmissionEvidenceService
	for _, item := range optional {
		switch v := item.(type) {
		case EnterpriseMemberModelAdmissionAutoStopEvidenceProvider:
			evidence = v
		case *EnterpriseMemberAdmissionEvidenceService:
			admissionEvidence = v
		}
	}
	autoStopUsesAdmissionEvidence := evidence == nil && admissionEvidence != nil
	if autoStopUsesAdmissionEvidence {
		evidence = NewEnterpriseMemberAdmissionEvidenceAutoStopProvider(admissionEvidence)
	}
	return &enterpriseMemberModelAdmissionReadinessProviderImpl{
		runtime:                       runtime,
		aliasReviewSvc:                aliasReviewSvc,
		evidenceSvc:                   admissionEvidence,
		coverage:                      NewEnterpriseMemberEvaluatorCoverageProvider(),
		autoStopEvidence:              evidence,
		autoStopUsesAdmissionEvidence: autoStopUsesAdmissionEvidence,
	}
}

func ProvideEnterpriseMemberModelAdmissionReadinessProvider(
	runtime *RoutingEligibilityRuntime,
	aliasReviewSvc *EnterpriseMemberAliasReviewService,
	evidenceSvc *EnterpriseMemberAdmissionEvidenceService,
) EnterpriseMemberModelAdmissionReadinessProvider {
	provider := NewEnterpriseMemberModelAdmissionReadinessProvider(runtime, aliasReviewSvc, evidenceSvc)
	SetEnterpriseMemberModelAdmissionReadinessProvider(provider)
	return provider
}

func (p *enterpriseMemberModelAdmissionReadinessProviderImpl) EvaluateEnterpriseMemberModelAdmissionReadiness(ctx context.Context) EnterpriseMemberModelAdmissionEnforceReadiness {
	if p == nil {
		p = &enterpriseMemberModelAdmissionReadinessProviderImpl{}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	conditions := make([]EnterpriseMemberModelAdmissionReadinessCondition, 0, 6)

	if p.runtime == nil {
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionRevision, Ready: false, Layer: "foundation", Reason: "routing_runtime_unavailable", Source: EnterpriseMemberModelAdmissionInjectedReadinessSource},
		)
	} else {
		runtimeReady := p.runtime.Ready()
		if version, ok := p.runtime.MirroredVersion(enterpriseMemberRouteGlobalEligibilityScopes()); runtimeReady && ok {
			conditions = append(conditions, EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionRevision, Ready: true, Layer: "foundation", Source: EnterpriseMemberModelAdmissionInjectedReadinessSource, Details: "routing_revision=" + version.String()})
		} else {
			reason := "routing_revision_unavailable"
			if !runtimeReady {
				reason = "routing_runtime_not_ready"
			}
			conditions = append(conditions, EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionRevision, Ready: false, Layer: "foundation", Reason: reason, Source: EnterpriseMemberModelAdmissionInjectedReadinessSource})
		}
	}
	coverageProvider := p.coverage
	if coverageProvider == nil {
		coverageProvider = NewEnterpriseMemberEvaluatorCoverageProvider()
	}
	coverage := coverageProvider.EvaluateEnterpriseMemberEvaluatorCoverage(ctx)
	coverageDetails := fmt.Sprintf("algorithm=%s items=%d", coverage.AlgorithmVersion, len(coverage.Items))
	conditions = append(conditions, EnterpriseMemberModelAdmissionReadinessCondition{
		Name:    enterpriseMemberModelAdmissionReadinessConditionEval,
		Ready:   coverage.Ready,
		Layer:   "foundation",
		Reason:  coverage.Reason,
		Source:  EnterpriseMemberModelAdmissionInjectedReadinessSource,
		Details: coverageDetails,
	})

	var aliasSummary *EnterpriseMemberAliasReviewReadinessSummary
	var aliasErr error
	if p.aliasReviewSvc == nil {
		aliasErr = fmt.Errorf("enterprise member alias review service is not configured")
	} else {
		aliasSummary, aliasErr = p.aliasReviewSvc.GetReadinessSummary(ctx)
	}

	if aliasErr != nil {
		reason := aliasErr.Error()
		if reason == "" {
			reason = "alias_readiness_unavailable"
		}
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionAlias, Ready: false, Layer: "foundation", Reason: reason, Source: EnterpriseMemberModelAdmissionInjectedReadinessSource},
		)
	} else if aliasSummary == nil {
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionAlias, Ready: false, Layer: "foundation", Reason: "alias_readiness_summary_missing", Source: EnterpriseMemberModelAdmissionInjectedReadinessSource},
		)
	} else {
		aliasReady := aliasSummary.BlockingUnreviewedActive7d == 0
		aliasReason := ""
		if !aliasReady {
			aliasReason = aliasSummary.Reason
			if aliasReason == "" {
				aliasReason = EnterpriseMemberAdmissionEvidenceReasonAliasReviewPending
			}
		}
		aliasDetails := fmt.Sprintf("summary_generated_at=%s blocking_7d=%d blocking_30d=%d", aliasSummary.GeneratedAt.UTC().Format(time.RFC3339), aliasSummary.BlockingUnreviewedActive7d, aliasSummary.BlockingUnreviewedActive30d)
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{
				Name:    enterpriseMemberModelAdmissionReadinessConditionAlias,
				Ready:   aliasReady,
				Layer:   "foundation",
				Reason:  aliasReason,
				Source:  EnterpriseMemberModelAdmissionInjectedReadinessSource,
				Details: aliasDetails,
			},
		)
	}

	var evidenceSummary *EnterpriseMemberAdmissionEvidenceSummary
	var evidenceErr error
	if p.evidenceSvc == nil {
		evidenceErr = fmt.Errorf("enterprise member admission evidence service is not configured")
	} else {
		evidenceSummary, evidenceErr = p.evidenceSvc.GetSummary(ctx)
	}
	if evidenceErr != nil {
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionEvidence, Ready: false, Layer: "foundation", Reason: EnterpriseMemberAdmissionEvidenceReasonUnavailable, Source: EnterpriseMemberModelAdmissionInjectedReadinessSource, Details: "evidence_provider_error"},
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionExpansion, Ready: false, Layer: "expansion", Reason: EnterpriseMemberAdmissionEvidenceReasonUnavailable, Source: EnterpriseMemberModelAdmissionInjectedReadinessSource, Details: "evidence_provider_error"},
		)
	} else if evidenceSummary == nil {
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionEvidence, Ready: false, Layer: "foundation", Reason: "admission_evidence_summary_missing", Source: EnterpriseMemberModelAdmissionInjectedReadinessSource},
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionExpansion, Ready: false, Layer: "expansion", Reason: "admission_evidence_summary_missing", Source: EnterpriseMemberModelAdmissionInjectedReadinessSource},
		)
	} else {
		evidenceDetails := fmt.Sprintf("window=%s..%s shadow_days=%d canary_total=%d control_total=%d planner_p95_ms=%d planner_p99_ms=%d", evidenceSummary.FullDaysStart.UTC().Format(time.RFC3339), evidenceSummary.LookbackEnd.UTC().Format(time.RFC3339), len(evidenceSummary.DailyShadowEvidence), evidenceSummary.CanaryComparison.CanaryTotal, evidenceSummary.CanaryComparison.ControlTotal, evidenceSummary.PlannerHealth.P95Ms, evidenceSummary.PlannerHealth.P99Ms)
		conditions = append(conditions,
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionEvidence, Ready: true, Layer: "foundation", Source: EnterpriseMemberModelAdmissionInjectedReadinessSource, Details: evidenceDetails},
			EnterpriseMemberModelAdmissionReadinessCondition{Name: enterpriseMemberModelAdmissionReadinessConditionExpansion, Ready: evidenceSummary.Ready, Layer: "expansion", Reason: expansionEvidenceReason(evidenceSummary), Source: EnterpriseMemberModelAdmissionInjectedReadinessSource, Details: evidenceDetails},
		)
	}

	var autoStop EnterpriseMemberModelAdmissionAutoStopState
	if p.autoStopUsesAdmissionEvidence {
		switch {
		case evidenceErr != nil || evidenceSummary == nil:
			autoStop = EnterpriseMemberModelAdmissionAutoStopState{
				Stopped: true,
				Source:  EnterpriseMemberModelAdmissionAutoStopSourceMetrics,
				Reason:  EnterpriseMemberModelAdmissionAutoStopReasonEvidenceUnavailable,
				Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonEvidenceUnavailable},
				Details: "evidence_provider_error",
			}
		default:
			autoStop = EvaluateEnterpriseMemberModelAdmissionAutoStop(
				enterpriseMemberAdmissionAutoStopEvidenceFromSummary(evidenceSummary),
				DefaultEnterpriseMemberModelAdmissionAutoStopThresholds(),
			)
		}
	} else {
		autoStop = EvaluateEnterpriseMemberModelAdmissionAutoStopFromProvider(ctx, p.autoStopEvidence)
	}
	if autoStop.Stopped {
		conditions = append(conditions, EnterpriseMemberModelAdmissionReadinessCondition{
			Name:    enterpriseMemberModelAdmissionReadinessConditionStop,
			Ready:   false,
			Layer:   "stop",
			Reason:  autoStop.Reason,
			Source:  autoStop.Source,
			Details: autoStop.Details,
		})
	} else {
		conditions = append(conditions, EnterpriseMemberModelAdmissionReadinessCondition{
			Name:   enterpriseMemberModelAdmissionReadinessConditionStop,
			Ready:  true,
			Layer:  "stop",
			Source: autoStop.Source,
		})
	}

	return normalizeEnterpriseMemberModelAdmissionReadiness(EnterpriseMemberModelAdmissionEnforceReadiness{
		Ready:      true,
		Source:     EnterpriseMemberModelAdmissionInjectedReadinessSource,
		AutoStop:   autoStop,
		Conditions: conditions,
	})
}

func expansionEvidenceReason(summary *EnterpriseMemberAdmissionEvidenceSummary) string {
	if summary == nil || summary.Ready {
		return ""
	}
	return summary.Reason
}

type defaultEnterpriseMemberModelAdmissionReadinessProvider struct{}

func (defaultEnterpriseMemberModelAdmissionReadinessProvider) EvaluateEnterpriseMemberModelAdmissionReadiness(context.Context) EnterpriseMemberModelAdmissionEnforceReadiness {
	conditions := []EnterpriseMemberModelAdmissionReadinessCondition{
		{Name: enterpriseMemberModelAdmissionReadinessConditionRevision, Ready: false, Layer: "foundation", Reason: "revision_health_unverified", Source: EnterpriseMemberModelAdmissionDefaultReadinessSource},
		{Name: enterpriseMemberModelAdmissionReadinessConditionEval, Ready: false, Layer: "foundation", Reason: "evaluator_coverage_unverified", Source: EnterpriseMemberModelAdmissionDefaultReadinessSource},
		{Name: enterpriseMemberModelAdmissionReadinessConditionAlias, Ready: false, Layer: "foundation", Reason: "alias_audit_unverified", Source: EnterpriseMemberModelAdmissionDefaultReadinessSource},
		{Name: enterpriseMemberModelAdmissionReadinessConditionEvidence, Ready: false, Layer: "foundation", Reason: "admission_evidence_unverified", Source: EnterpriseMemberModelAdmissionDefaultReadinessSource},
		{Name: enterpriseMemberModelAdmissionReadinessConditionExpansion, Ready: false, Layer: "expansion", Reason: "expansion_evidence_unverified", Source: EnterpriseMemberModelAdmissionDefaultReadinessSource},
		{Name: enterpriseMemberModelAdmissionReadinessConditionStop, Ready: false, Layer: "stop", Reason: "auto_stop_evidence_unverified", Source: EnterpriseMemberModelAdmissionDefaultReadinessSource},
	}
	return EnterpriseMemberModelAdmissionEnforceReadiness{
		Ready:           false,
		Reason:          EnterpriseMemberModelAdmissionEnforceBlockedReason,
		Source:          EnterpriseMemberModelAdmissionDefaultReadinessSource,
		FoundationReady: false,
		CanaryReady:     false,
		ExpansionReady:  false,
		AutoStop:        EnterpriseMemberModelAdmissionAutoStopState{Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics},
		Conditions:      conditions,
	}
}

type enterpriseMemberModelAdmissionReadinessProviderHolder struct {
	provider EnterpriseMemberModelAdmissionReadinessProvider
}

var enterpriseMemberModelAdmissionReadinessProvider atomic.Value // *enterpriseMemberModelAdmissionReadinessProviderHolder

func currentEnterpriseMemberModelAdmissionReadinessProvider() EnterpriseMemberModelAdmissionReadinessProvider {
	if holder, ok := enterpriseMemberModelAdmissionReadinessProvider.Load().(*enterpriseMemberModelAdmissionReadinessProviderHolder); ok && holder != nil && holder.provider != nil {
		return holder.provider
	}
	return defaultEnterpriseMemberModelAdmissionReadinessProvider{}
}

func SetEnterpriseMemberModelAdmissionReadinessProvider(provider EnterpriseMemberModelAdmissionReadinessProvider) {
	if provider == nil {
		provider = defaultEnterpriseMemberModelAdmissionReadinessProvider{}
	}
	enterpriseMemberModelAdmissionReadinessProvider.Store(&enterpriseMemberModelAdmissionReadinessProviderHolder{provider: provider})
}

func SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider EnterpriseMemberModelAdmissionReadinessProvider) func() {
	previous, _ := enterpriseMemberModelAdmissionReadinessProvider.Load().(*enterpriseMemberModelAdmissionReadinessProviderHolder)
	SetEnterpriseMemberModelAdmissionReadinessProvider(provider)
	return func() {
		if previous == nil {
			SetEnterpriseMemberModelAdmissionReadinessProvider(nil)
			return
		}
		enterpriseMemberModelAdmissionReadinessProvider.Store(previous)
	}
}

func CurrentEnterpriseMemberModelAdmissionEnforceReadiness() EnterpriseMemberModelAdmissionEnforceReadiness {
	return EvaluateEnterpriseMemberModelAdmissionEnforceReadiness(context.Background())
}

func EvaluateEnterpriseMemberModelAdmissionEnforceReadiness(ctx context.Context) EnterpriseMemberModelAdmissionEnforceReadiness {
	if ctx == nil {
		ctx = context.Background()
	}
	readiness := currentEnterpriseMemberModelAdmissionReadinessProvider().EvaluateEnterpriseMemberModelAdmissionReadiness(ctx)
	return normalizeEnterpriseMemberModelAdmissionReadiness(readiness)
}

func normalizeEnterpriseMemberModelAdmissionReadiness(readiness EnterpriseMemberModelAdmissionEnforceReadiness) EnterpriseMemberModelAdmissionEnforceReadiness {
	if readiness.Source == "" {
		readiness.Source = EnterpriseMemberModelAdmissionInjectedReadinessSource
	}
	if readiness.AutoStopped && !readiness.AutoStop.Stopped {
		readiness.AutoStop = ManualEnterpriseMemberModelAdmissionAutoStopState(true)
		readiness.Ready = false
		if readiness.Reason == "" {
			readiness.Reason = EnterpriseMemberModelAdmissionAutoStopReason
		}
	}
	if readiness.AutoStop.Stopped {
		readiness.AutoStopped = true
		readiness.Ready = false
		if readiness.Reason == "" {
			readiness.Reason = readiness.AutoStop.Reason
		}
	}
	readiness.FoundationReady = true
	readiness.ExpansionReady = true
	stopClear := true
	if len(readiness.Conditions) == 0 {
		readiness.FoundationReady = false
		readiness.ExpansionReady = false
		stopClear = false
	}
	for _, condition := range readiness.Conditions {
		switch condition.Layer {
		case "foundation":
			if !condition.Ready {
				readiness.FoundationReady = false
			}
		case "expansion":
			if !condition.Ready {
				readiness.ExpansionReady = false
			}
		case "stop":
			if !condition.Ready {
				stopClear = false
			}
		}
	}
	if readiness.AutoStop.Stopped {
		stopClear = false
	}
	readiness.CanaryReady = readiness.FoundationReady && stopClear
	readiness.Ready = readiness.CanaryReady && readiness.ExpansionReady
	for _, condition := range readiness.Conditions {
		if !condition.Ready {
			if readiness.Reason == "" {
				readiness.Reason = conditionReason(condition, EnterpriseMemberModelAdmissionEnforceBlockedReason)
			}
			break
		}
	}
	if len(readiness.Conditions) == 0 {
		readiness.Ready = false
		if readiness.Reason == "" {
			readiness.Reason = EnterpriseMemberModelAdmissionEnforceBlockedReason
		}
		return readiness
	}
	if readiness.Ready {
		readiness.Reason = ""
		return readiness
	}
	if readiness.Reason == "" {
		readiness.Reason = EnterpriseMemberModelAdmissionEnforceBlockedReason
	}
	return readiness
}

func conditionReason(condition EnterpriseMemberModelAdmissionReadinessCondition, fallback string) string {
	if condition.Reason != "" {
		return condition.Reason
	}
	if condition.Name != "" {
		return condition.Name + "_not_ready"
	}
	return fallback
}

type EnterpriseMemberModelAdmissionRolloutPolicy struct {
	EnterpriseUserIDs []int64 `json:"enterprise_user_ids,omitempty"`
	MemberIDs         []int64 `json:"member_ids,omitempty"`
	Percentage        int     `json:"percentage,omitempty"`
	Salt              string  `json:"salt,omitempty"`
	AutoStop          bool    `json:"auto_stop,omitempty"`
}

type EnterpriseMemberModelAdmissionRolloutState struct {
	Policy            EnterpriseMemberModelAdmissionRolloutPolicy `json:"policy"`
	Source            string                                      `json:"source"`
	Valid             bool                                        `json:"valid"`
	Reason            string                                      `json:"reason,omitempty"`
	Matched           bool                                        `json:"matched,omitempty"`
	MatchedBy         string                                      `json:"matched_by,omitempty"`
	HashBucket        int                                         `json:"hash_bucket,omitempty"`
	StableHashPercent int                                         `json:"stable_hash_percent"`
	AutoStopped       bool                                        `json:"auto_stopped"`
	AutoStop          EnterpriseMemberModelAdmissionAutoStopState `json:"auto_stop"`
}

type EnterpriseMemberModelAdmissionRolloutInput struct {
	APIKeyID         int64
	EnterpriseUserID int64
	MemberID         int64
}

func DefaultEnterpriseMemberModelAdmissionRolloutPolicy() EnterpriseMemberModelAdmissionRolloutPolicy {
	return EnterpriseMemberModelAdmissionRolloutPolicy{
		Percentage: 0,
		Salt:       enterpriseMemberModelAdmissionDefaultRolloutPolicySalt,
	}
}

func ParseEnterpriseMemberModelAdmissionRolloutPolicy(raw string) (EnterpriseMemberModelAdmissionRolloutPolicy, error) {
	policy := DefaultEnterpriseMemberModelAdmissionRolloutPolicy()
	if strings.TrimSpace(raw) == "" {
		return policy, nil
	}
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("parse rollout policy: %w", err)
	}
	return NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(policy)
}

func NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(policy EnterpriseMemberModelAdmissionRolloutPolicy) (EnterpriseMemberModelAdmissionRolloutPolicy, error) {
	if policy.Percentage < 0 || policy.Percentage > 100 {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("rollout percentage must be between 0 and 100")
	}
	if len(policy.EnterpriseUserIDs) > enterpriseMemberModelAdmissionMaxRolloutIDsPerList {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("enterprise_user_ids must contain at most %d raw ids", enterpriseMemberModelAdmissionMaxRolloutIDsPerList)
	}
	if len(policy.MemberIDs) > enterpriseMemberModelAdmissionMaxRolloutIDsPerList {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("member_ids must contain at most %d raw ids", enterpriseMemberModelAdmissionMaxRolloutIDsPerList)
	}
	if len(policy.EnterpriseUserIDs)+len(policy.MemberIDs) > enterpriseMemberModelAdmissionMaxRolloutExplicitTargets {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("rollout explicit targets must contain at most %d raw ids", enterpriseMemberModelAdmissionMaxRolloutExplicitTargets)
	}
	if data, err := json.Marshal(policy); err != nil {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("marshal raw rollout policy for validation: %w", err)
	} else if len(data) > enterpriseMemberModelAdmissionMaxRolloutPolicyBytes {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("raw rollout policy must be at most %d bytes", enterpriseMemberModelAdmissionMaxRolloutPolicyBytes)
	}
	policy.EnterpriseUserIDs = normalizeInt64Set(policy.EnterpriseUserIDs)
	policy.MemberIDs = normalizeInt64Set(policy.MemberIDs)
	if len(policy.EnterpriseUserIDs) > enterpriseMemberModelAdmissionMaxRolloutIDsPerList {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("enterprise_user_ids must contain at most %d ids", enterpriseMemberModelAdmissionMaxRolloutIDsPerList)
	}
	if len(policy.MemberIDs) > enterpriseMemberModelAdmissionMaxRolloutIDsPerList {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("member_ids must contain at most %d ids", enterpriseMemberModelAdmissionMaxRolloutIDsPerList)
	}
	if len(policy.EnterpriseUserIDs)+len(policy.MemberIDs) > enterpriseMemberModelAdmissionMaxRolloutExplicitTargets {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("rollout explicit targets must contain at most %d ids", enterpriseMemberModelAdmissionMaxRolloutExplicitTargets)
	}
	if strings.TrimSpace(policy.Salt) == "" {
		policy.Salt = enterpriseMemberModelAdmissionDefaultRolloutPolicySalt
	} else {
		policy.Salt = strings.TrimSpace(policy.Salt)
	}
	if len([]byte(policy.Salt)) > enterpriseMemberModelAdmissionMaxRolloutSaltBytes {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("rollout salt must be at most %d bytes", enterpriseMemberModelAdmissionMaxRolloutSaltBytes)
	}
	if data, err := json.Marshal(policy); err != nil {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("marshal rollout policy for validation: %w", err)
	} else if len(data) > enterpriseMemberModelAdmissionMaxRolloutPolicyBytes {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), fmt.Errorf("rollout policy must be at most %d bytes", enterpriseMemberModelAdmissionMaxRolloutPolicyBytes)
	}
	return policy, nil
}

func MarshalEnterpriseMemberModelAdmissionRolloutPolicy(policy EnterpriseMemberModelAdmissionRolloutPolicy) (string, error) {
	normalized, err := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(policy)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal rollout policy: %w", err)
	}
	return string(data), nil
}

func EvaluateEnterpriseMemberModelAdmissionRollout(policy EnterpriseMemberModelAdmissionRolloutPolicy, input EnterpriseMemberModelAdmissionRolloutInput) EnterpriseMemberModelAdmissionRolloutState {
	normalized, err := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(policy)
	if err != nil {
		return EnterpriseMemberModelAdmissionRolloutState{
			Policy:            DefaultEnterpriseMemberModelAdmissionRolloutPolicy(),
			Source:            "runtime",
			Valid:             false,
			Reason:            EnterpriseMemberModelAdmissionRolloutInvalidReason,
			StableHashPercent: 0,
			AutoStopped:       false,
			AutoStop:          ManualEnterpriseMemberModelAdmissionAutoStopState(false),
		}
	}
	state := EnterpriseMemberModelAdmissionRolloutState{
		Policy:            normalized,
		Source:            "runtime",
		Valid:             true,
		StableHashPercent: normalized.Percentage,
		AutoStopped:       normalized.AutoStop,
		AutoStop:          ManualEnterpriseMemberModelAdmissionAutoStopState(normalized.AutoStop),
	}
	if normalized.AutoStop {
		state.Reason = EnterpriseMemberModelAdmissionAutoStopReasonManual
		return state
	}
	if containsEnterpriseMemberAdmissionInt64(normalized.EnterpriseUserIDs, input.EnterpriseUserID) {
		state.Matched = true
		state.MatchedBy = "enterprise_user_allowlist"
		return state
	}
	if containsEnterpriseMemberAdmissionInt64(normalized.MemberIDs, input.MemberID) {
		state.Matched = true
		state.MatchedBy = "member_allowlist"
		return state
	}
	state.HashBucket = stableEnterpriseMemberModelAdmissionBucket(normalized.Salt, input)
	if normalized.Percentage > 0 && state.HashBucket < normalized.Percentage {
		state.Matched = true
		state.MatchedBy = "stable_hash"
		return state
	}
	if len(normalized.EnterpriseUserIDs) == 0 && len(normalized.MemberIDs) == 0 && normalized.Percentage == 0 {
		state.Reason = EnterpriseMemberModelAdmissionRolloutNoTargetReason
		return state
	}
	state.Reason = EnterpriseMemberModelAdmissionRolloutMissReason
	return state
}

func stableEnterpriseMemberModelAdmissionBucket(salt string, input EnterpriseMemberModelAdmissionRolloutInput) int {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|enterprise:%d|member:%d|key:%d", salt, input.EnterpriseUserID, input.MemberID, input.APIKeyID)))
	return int(binary.BigEndian.Uint64(sum[:8]) % 100)
}

func normalizeInt64Set(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func containsEnterpriseMemberAdmissionInt64(values []int64, value int64) bool {
	if value <= 0 || len(values) == 0 {
		return false
	}
	i := sort.Search(len(values), func(i int) bool { return values[i] >= value })
	return i < len(values) && values[i] == value
}

type EnterpriseMemberModelAdmissionGateSnapshot struct {
	Version                  string    `json:"version"`
	GeneratedAt              time.Time `json:"generated_at"`
	ExpiresAt                time.Time `json:"expires_at"`
	AgeMs                    int64     `json:"age_ms"`
	Source                   string    `json:"source"`
	PolicyHash               string    `json:"policy_hash"`
	RoutingRevision          string    `json:"routing_revision,omitempty"`
	EvaluatorCoverageVersion string    `json:"evaluator_coverage_version,omitempty"`
	EvidenceWindow           string    `json:"evidence_window,omitempty"`
	FoundationReady          bool      `json:"foundation_ready"`
	CanaryReady              bool      `json:"canary_ready"`
	ExpansionReady           bool      `json:"expansion_ready"`
	StopClear                bool      `json:"stop_clear"`
	StopSource               string    `json:"stop_source,omitempty"`
	StopReason               string    `json:"stop_reason,omitempty"`
}

func BuildEnterpriseMemberModelAdmissionGateSnapshot(readiness EnterpriseMemberModelAdmissionEnforceReadiness, policy EnterpriseMemberModelAdmissionRolloutPolicy, source string, generatedAt, expiresAt time.Time) EnterpriseMemberModelAdmissionGateSnapshot {
	if generatedAt.IsZero() {
		generatedAt = time.Now()
	}
	generatedAt = generatedAt.UTC()
	if expiresAt.IsZero() || !expiresAt.After(generatedAt) {
		expiresAt = generatedAt
	}
	expiresAt = expiresAt.UTC()
	snapshot := EnterpriseMemberModelAdmissionGateSnapshot{
		Version:         "enterprise_member_model_admission_gate_v1",
		GeneratedAt:     generatedAt,
		ExpiresAt:       expiresAt,
		Source:          source,
		PolicyHash:      hashEnterpriseMemberModelAdmissionPolicy(policy),
		FoundationReady: readiness.FoundationReady,
		CanaryReady:     readiness.CanaryReady,
		ExpansionReady:  readiness.ExpansionReady,
		StopClear:       !readiness.AutoStop.Stopped,
		StopSource:      readiness.AutoStop.Source,
		StopReason:      readiness.AutoStop.Reason,
	}
	for _, condition := range readiness.Conditions {
		switch condition.Name {
		case enterpriseMemberModelAdmissionReadinessConditionRevision:
			if condition.Ready {
				snapshot.RoutingRevision = condition.Details
			}
		case enterpriseMemberModelAdmissionReadinessConditionEval:
			snapshot.EvaluatorCoverageVersion = condition.Details
		case enterpriseMemberModelAdmissionReadinessConditionEvidence, enterpriseMemberModelAdmissionReadinessConditionExpansion:
			if snapshot.EvidenceWindow == "" {
				snapshot.EvidenceWindow = condition.Details
			}
		}
	}
	return snapshot.WithAge(generatedAt)
}

func (s EnterpriseMemberModelAdmissionGateSnapshot) WithAge(now time.Time) EnterpriseMemberModelAdmissionGateSnapshot {
	if now.IsZero() {
		now = time.Now()
	}
	if !s.GeneratedAt.IsZero() {
		s.AgeMs = now.UTC().Sub(s.GeneratedAt.UTC()).Milliseconds()
		if s.AgeMs < 0 {
			s.AgeMs = 0
		}
	}
	return s
}

func hashEnterpriseMemberModelAdmissionPolicy(policy EnterpriseMemberModelAdmissionRolloutPolicy) string {
	normalized, err := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(policy)
	if err != nil {
		normalized = DefaultEnterpriseMemberModelAdmissionRolloutPolicy()
	}
	data, _ := json.Marshal(normalized)
	h := fnv.New64a()
	_, _ = h.Write(data)
	return fmt.Sprintf("fnv64a:%016x", h.Sum64())
}
