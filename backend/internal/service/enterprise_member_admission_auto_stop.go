package service

import (
	"context"
	"strings"
	"time"
)

const (
	EnterpriseMemberModelAdmissionAutoStopSourceManual  = "manual"
	EnterpriseMemberModelAdmissionAutoStopSourceMetrics = "metrics"

	EnterpriseMemberModelAdmissionAutoStopReasonManual                     = "manual_auto_stop"
	EnterpriseMemberModelAdmissionAutoStopReasonSuccessRateDrop            = "success_rate_drop"
	EnterpriseMemberModelAdmissionAutoStopReasonUnreviewedAliasActive      = "unreviewed_alias_active"
	EnterpriseMemberModelAdmissionAutoStopReasonEvaluationFailedElevated   = "evaluation_failed_elevated"
	EnterpriseMemberModelAdmissionAutoStopReasonLKGUnsafe                  = "lkg_unsafe"
	EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded     = "planner_latency_exceeded"
	EnterpriseMemberModelAdmissionAutoStopReasonUnpublishedActualAttempt   = "unpublished_actual_attempt"
	EnterpriseMemberModelAdmissionAutoStopReasonRestrictionBypass          = "restriction_bypass"
	EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay               = "unsafe_replay"
	EnterpriseMemberModelAdmissionAutoStopReasonExplicitAliasUnrecoverable = "explicit_alias_unrecoverable"
	EnterpriseMemberModelAdmissionAutoStopReasonEvidenceUnavailable        = "auto_stop_evidence_unavailable"
)

const (
	defaultEnterpriseMemberAutoStopMinSamples               int64 = 100
	defaultEnterpriseMemberAutoStopSuccessRateDropBasisPts  int64 = 500
	defaultEnterpriseMemberAutoStopEvaluationFailedPermille int64 = 10
	defaultEnterpriseMemberAutoStopPlannerP95BudgetMs       int64 = 5
	defaultEnterpriseMemberAutoStopPlannerP99BudgetMs       int64 = 20
)

// EnterpriseMemberModelAdmissionAutoStopEvidenceProvider returns low-cardinality
// rollout evidence only. Implementations must not include model names, member
// identifiers, API key IDs, group topology, or request IDs in summary fields.
type EnterpriseMemberModelAdmissionAutoStopEvidenceProvider interface {
	SummarizeEnterpriseMemberModelAdmissionAutoStopEvidence(ctx context.Context) (EnterpriseMemberModelAdmissionAutoStopEvidenceSummary, error)
}

type EnterpriseMemberModelAdmissionAutoStopEvidenceSummary struct {
	GeneratedAt time.Time `json:"generated_at,omitempty"`
	Window      string    `json:"window,omitempty"`

	EnforceSamples int64 `json:"enforce_samples,omitempty"`
	ControlSamples int64 `json:"control_samples,omitempty"`

	EnforceSuccessRatePermille int64 `json:"enforce_success_rate_permille,omitempty"`
	ControlSuccessRatePermille int64 `json:"control_success_rate_permille,omitempty"`

	UnreviewedAliasActiveCount         int64 `json:"unreviewed_alias_active_count,omitempty"`
	EvaluationFailedPermille           int64 `json:"evaluation_failed_permille,omitempty"`
	LKGGenerationMismatchCount         int64 `json:"lkg_generation_mismatch_count,omitempty"`
	LKGStaleHitCount                   int64 `json:"lkg_stale_hit_count,omitempty"`
	LKGWrongGroupAfterUseCount         int64 `json:"lkg_wrong_group_after_use_count,omitempty"`
	PlannerP95Ms                       int64 `json:"planner_p95_ms,omitempty"`
	PlannerP99Ms                       int64 `json:"planner_p99_ms,omitempty"`
	UnpublishedModelActualAttemptCount int64 `json:"unpublished_model_actual_attempt_count,omitempty"`
	UnauthorizedCandidateCount         int64 `json:"unauthorized_candidate_count,omitempty"`
	LKGRevokedGroupRestoreCount        int64 `json:"lkg_revoked_group_restore_count,omitempty"`
	RestrictionBypassCount             int64 `json:"restriction_bypass_count,omitempty"`
	UnsafeReplayCount                  int64 `json:"unsafe_replay_count,omitempty"`
	ExplicitAliasUnrecoverableCount    int64 `json:"explicit_alias_unrecoverable_count,omitempty"`
}

type EnterpriseMemberModelAdmissionAutoStopState struct {
	Stopped   bool     `json:"stopped"`
	Source    string   `json:"source,omitempty"`
	Reason    string   `json:"reason,omitempty"`
	Reasons   []string `json:"reasons,omitempty"`
	Details   string   `json:"details,omitempty"`
	Generated string   `json:"generated_at,omitempty"`
}

type EnterpriseMemberModelAdmissionAutoStopThresholds struct {
	MinSamples                 int64
	SuccessRateDropBasisPoints int64
	EvaluationFailedPermille   int64
	PlannerP95BudgetMs         int64
	PlannerP99BudgetMs         int64
}

func DefaultEnterpriseMemberModelAdmissionAutoStopThresholds() EnterpriseMemberModelAdmissionAutoStopThresholds {
	return EnterpriseMemberModelAdmissionAutoStopThresholds{
		MinSamples:                 defaultEnterpriseMemberAutoStopMinSamples,
		SuccessRateDropBasisPoints: defaultEnterpriseMemberAutoStopSuccessRateDropBasisPts,
		EvaluationFailedPermille:   defaultEnterpriseMemberAutoStopEvaluationFailedPermille,
		PlannerP95BudgetMs:         defaultEnterpriseMemberAutoStopPlannerP95BudgetMs,
		PlannerP99BudgetMs:         defaultEnterpriseMemberAutoStopPlannerP99BudgetMs,
	}
}

type noopEnterpriseMemberModelAdmissionAutoStopEvidenceProvider struct{}

func (noopEnterpriseMemberModelAdmissionAutoStopEvidenceProvider) SummarizeEnterpriseMemberModelAdmissionAutoStopEvidence(context.Context) (EnterpriseMemberModelAdmissionAutoStopEvidenceSummary, error) {
	return EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{}, nil
}

func NewNoopEnterpriseMemberModelAdmissionAutoStopEvidenceProvider() EnterpriseMemberModelAdmissionAutoStopEvidenceProvider {
	return noopEnterpriseMemberModelAdmissionAutoStopEvidenceProvider{}
}

func EvaluateEnterpriseMemberModelAdmissionAutoStop(
	summary EnterpriseMemberModelAdmissionAutoStopEvidenceSummary,
	thresholds EnterpriseMemberModelAdmissionAutoStopThresholds,
) EnterpriseMemberModelAdmissionAutoStopState {
	thresholds = normalizeEnterpriseMemberModelAdmissionAutoStopThresholds(thresholds)
	reasons := make([]string, 0, 8)

	if summary.EnforceSamples >= thresholds.MinSamples && summary.ControlSamples >= thresholds.MinSamples {
		dropBasisPoints := (summary.ControlSuccessRatePermille - summary.EnforceSuccessRatePermille) * 10
		if dropBasisPoints >= thresholds.SuccessRateDropBasisPoints {
			reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonSuccessRateDrop)
		}
	}
	if summary.UnreviewedAliasActiveCount > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonUnreviewedAliasActive)
	}
	if summary.EvaluationFailedPermille >= thresholds.EvaluationFailedPermille && summary.EvaluationFailedPermille > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonEvaluationFailedElevated)
	}
	if summary.LKGGenerationMismatchCount > 0 || summary.LKGStaleHitCount > 0 || summary.LKGWrongGroupAfterUseCount > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonLKGUnsafe)
	}
	if (thresholds.PlannerP95BudgetMs > 0 && summary.PlannerP95Ms > thresholds.PlannerP95BudgetMs) ||
		(thresholds.PlannerP99BudgetMs > 0 && summary.PlannerP99Ms > thresholds.PlannerP99BudgetMs) {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonPlannerLatencyExceeded)
	}
	if summary.UnpublishedModelActualAttemptCount > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonUnpublishedActualAttempt)
	}
	if summary.UnauthorizedCandidateCount > 0 ||
		summary.LKGRevokedGroupRestoreCount > 0 ||
		summary.RestrictionBypassCount > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonRestrictionBypass)
	}
	if summary.UnsafeReplayCount > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay)
	}
	if summary.ExplicitAliasUnrecoverableCount > 0 {
		reasons = append(reasons, EnterpriseMemberModelAdmissionAutoStopReasonExplicitAliasUnrecoverable)
	}

	state := EnterpriseMemberModelAdmissionAutoStopState{
		Stopped: len(reasons) > 0,
		Source:  EnterpriseMemberModelAdmissionAutoStopSourceMetrics,
		Reasons: reasons,
	}
	if !summary.GeneratedAt.IsZero() {
		state.Generated = summary.GeneratedAt.UTC().Format(time.RFC3339)
	}
	if state.Stopped {
		state.Reason = reasons[0]
		state.Details = enterpriseMemberModelAdmissionAutoStopDetails(summary)
	}
	return state
}

func EvaluateEnterpriseMemberModelAdmissionAutoStopFromProvider(
	ctx context.Context,
	provider EnterpriseMemberModelAdmissionAutoStopEvidenceProvider,
) EnterpriseMemberModelAdmissionAutoStopState {
	if ctx == nil {
		ctx = context.Background()
	}
	if provider == nil {
		return EnterpriseMemberModelAdmissionAutoStopState{Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics}
	}
	summary, err := provider.SummarizeEnterpriseMemberModelAdmissionAutoStopEvidence(ctx)
	if err != nil {
		return EnterpriseMemberModelAdmissionAutoStopState{
			Stopped: true,
			Source:  EnterpriseMemberModelAdmissionAutoStopSourceMetrics,
			Reason:  EnterpriseMemberModelAdmissionAutoStopReasonEvidenceUnavailable,
			Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonEvidenceUnavailable},
			Details: "evidence_provider_error",
		}
	}
	return EvaluateEnterpriseMemberModelAdmissionAutoStop(summary, DefaultEnterpriseMemberModelAdmissionAutoStopThresholds())
}

func ManualEnterpriseMemberModelAdmissionAutoStopState(enabled bool) EnterpriseMemberModelAdmissionAutoStopState {
	if !enabled {
		return EnterpriseMemberModelAdmissionAutoStopState{Source: EnterpriseMemberModelAdmissionAutoStopSourceManual}
	}
	return EnterpriseMemberModelAdmissionAutoStopState{
		Stopped: true,
		Source:  EnterpriseMemberModelAdmissionAutoStopSourceManual,
		Reason:  EnterpriseMemberModelAdmissionAutoStopReasonManual,
		Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonManual},
	}
}

func normalizeEnterpriseMemberModelAdmissionAutoStopThresholds(thresholds EnterpriseMemberModelAdmissionAutoStopThresholds) EnterpriseMemberModelAdmissionAutoStopThresholds {
	defaults := DefaultEnterpriseMemberModelAdmissionAutoStopThresholds()
	if thresholds.MinSamples <= 0 {
		thresholds.MinSamples = defaults.MinSamples
	}
	if thresholds.SuccessRateDropBasisPoints <= 0 {
		thresholds.SuccessRateDropBasisPoints = defaults.SuccessRateDropBasisPoints
	}
	if thresholds.EvaluationFailedPermille <= 0 {
		thresholds.EvaluationFailedPermille = defaults.EvaluationFailedPermille
	}
	if thresholds.PlannerP95BudgetMs <= 0 {
		thresholds.PlannerP95BudgetMs = defaults.PlannerP95BudgetMs
	}
	if thresholds.PlannerP99BudgetMs <= 0 {
		thresholds.PlannerP99BudgetMs = defaults.PlannerP99BudgetMs
	}
	return thresholds
}

func enterpriseMemberModelAdmissionAutoStopDetails(summary EnterpriseMemberModelAdmissionAutoStopEvidenceSummary) string {
	parts := make([]string, 0, 6)
	if summary.Window != "" {
		parts = append(parts, "window="+strings.TrimSpace(summary.Window))
	}
	if summary.EnforceSamples > 0 || summary.ControlSamples > 0 {
		parts = append(parts, "samples=enforce/control")
	}
	if summary.PlannerP95Ms > 0 || summary.PlannerP99Ms > 0 {
		parts = append(parts, "planner_latency_ms=p95/p99")
	}
	if len(parts) == 0 {
		return "metrics_threshold_exceeded"
	}
	return strings.Join(parts, " ")
}

func IsEnterpriseMemberModelAdmissionRolloutExpansion(current, next EnterpriseMemberModelAdmissionRolloutPolicy) bool {
	current, currentErr := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(current)
	next, nextErr := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(next)
	if currentErr != nil || nextErr != nil {
		return false
	}
	if next.AutoStop && !current.AutoStop {
		return false
	}
	if next.Percentage > current.Percentage {
		return true
	}
	return false
}
