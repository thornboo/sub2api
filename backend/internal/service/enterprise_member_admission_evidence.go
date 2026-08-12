package service

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const (
	EnterpriseMemberAdmissionEvidenceReasonReady                       = "admission_evidence_satisfied"
	EnterpriseMemberAdmissionEvidenceReasonUnavailable                 = "admission_evidence_unavailable"
	EnterpriseMemberAdmissionEvidenceReasonShadowCoverageInsufficient  = "shadow_full_day_coverage_insufficient"
	EnterpriseMemberAdmissionEvidenceReasonAliasReviewPending          = "legacy_success_new_pruned_requires_review"
	EnterpriseMemberAdmissionEvidenceReasonShadowKeptBaselineMissing   = "shadow_kept_success_baseline_missing"
	EnterpriseMemberAdmissionEvidenceReasonCanaryEvidenceMissing       = "enforce_canary_evidence_missing"
	EnterpriseMemberAdmissionEvidenceReasonCanarySuccessRateRegression = "enforce_canary_success_rate_regression"
	EnterpriseMemberAdmissionEvidenceReasonUnpublishedAttemptViolation = "unpublished_model_actual_attempt_violation"
	EnterpriseMemberAdmissionEvidenceReasonUnpublishedEvidenceMissing  = "unpublished_model_guard_evidence_missing"
	EnterpriseMemberAdmissionEvidenceReasonPlannerEvaluationFailed     = "planner_evaluation_failed_rate_exceeded"
	EnterpriseMemberAdmissionEvidenceReasonPlannerLatencyMissing       = "planner_latency_evidence_missing"
)

type EnterpriseMemberAdmissionEvidenceRepository interface {
	GetEnterpriseMemberAdmissionEvidence(ctx context.Context, input EnterpriseMemberAdmissionEvidenceInput) (*EnterpriseMemberAdmissionEvidenceSummary, error)
}

type EnterpriseMemberAdmissionEvidenceInput struct {
	Now                               time.Time
	ShadowFullDays                    int
	LookbackDays                      int
	MaxCanarySuccessRateDeclineBps    int64
	MaxPlannerEvaluationFailedRateBps int64
}

type EnterpriseMemberAdmissionEvidenceSummary struct {
	Ready       bool      `json:"ready"`
	Reason      string    `json:"reason"`
	GeneratedAt time.Time `json:"generated_at"`

	ShadowFullDays int       `json:"shadow_full_days"`
	FullDaysStart  time.Time `json:"full_days_start"`
	FullDaysEnd    time.Time `json:"full_days_end"`
	LookbackStart  time.Time `json:"lookback_start"`
	LookbackEnd    time.Time `json:"lookback_end"`

	DailyShadowEvidence []EnterpriseMemberAdmissionDailyShadowEvidence `json:"daily_shadow_evidence"`
	AliasAudit          EnterpriseMemberAdmissionAliasAuditEvidence    `json:"alias_audit"`
	ShadowKeptBaseline  EnterpriseMemberAdmissionShadowKeptBaseline    `json:"shadow_kept_baseline"`
	CanaryComparison    EnterpriseMemberAdmissionCanaryComparison      `json:"canary_comparison"`
	UnpublishedGuard    EnterpriseMemberAdmissionUnpublishedGuard      `json:"unpublished_guard"`
	PlannerHealth       EnterpriseMemberAdmissionPlannerHealth         `json:"planner_health"`
	LKG                 EnterpriseMemberAdmissionLKGEvidence           `json:"lkg"`
}

type EnterpriseMemberAdmissionDailyShadowEvidence struct {
	DayStart                         time.Time `json:"day_start"`
	ShadowSamples                    int64     `json:"shadow_samples"`
	LegacySuccessNewPruned           int64     `json:"legacy_success_new_pruned"`
	UnreviewedLegacySuccessNewPruned int64     `json:"unreviewed_legacy_success_new_pruned"`
}

type EnterpriseMemberAdmissionAliasAuditEvidence struct {
	ActiveLegacySuccessNewPruned30d int64 `json:"active_legacy_success_new_pruned_30d"`
	UnreviewedActive30d             int64 `json:"unreviewed_active_30d"`
}

type EnterpriseMemberAdmissionShadowKeptBaseline struct {
	Successes                int64 `json:"successes"`
	DistinctEndpoints        int64 `json:"distinct_endpoints"`
	DistinctEnterpriseOwners int64 `json:"distinct_enterprise_owners"`
}

type EnterpriseMemberAdmissionCanaryComparison struct {
	CanarySuccesses       int64 `json:"canary_successes"`
	CanaryErrors          int64 `json:"canary_errors"`
	CanaryTotal           int64 `json:"canary_total"`
	CanarySuccessRateBps  int64 `json:"canary_success_rate_bps"`
	ControlSuccesses      int64 `json:"control_successes"`
	ControlErrors         int64 `json:"control_errors"`
	ControlTotal          int64 `json:"control_total"`
	ControlSuccessRateBps int64 `json:"control_success_rate_bps"`
	DeclineBps            int64 `json:"decline_bps"`
	MaxAllowedDeclineBps  int64 `json:"max_allowed_decline_bps"`
}

type EnterpriseMemberAdmissionUnpublishedGuard struct {
	ModelUnpublishedNoCandidateErrors int64 `json:"model_unpublished_no_candidate_errors"`
	ActualAttemptViolations           int64 `json:"actual_attempt_violations"`
}

type EnterpriseMemberAdmissionPlannerHealth struct {
	Evaluations             int64 `json:"evaluations"`
	EvaluationFailed        int64 `json:"evaluation_failed"`
	EvaluationFailedRateBps int64 `json:"evaluation_failed_rate_bps"`
	MaxEvaluationFailedBps  int64 `json:"max_evaluation_failed_bps"`
	LatencySamples          int64 `json:"latency_samples"`
	P95Ms                   int64 `json:"p95_ms"`
	P99Ms                   int64 `json:"p99_ms"`
}

type EnterpriseMemberAdmissionLKGEvidence struct {
	LiveSuccesses         int64 `json:"live_successes"`
	LKGSuccesses          int64 `json:"lkg_successes"`
	LKGErrors             int64 `json:"lkg_errors"`
	LKGTotal              int64 `json:"lkg_total"`
	LKGSuccessRateBps     int64 `json:"lkg_success_rate_bps"`
	LKGMisses             int64 `json:"lkg_misses"`
	LKGStaleOrExpired     int64 `json:"lkg_stale_or_expired"`
	LKGGenerationMismatch int64 `json:"lkg_generation_mismatch"`
}

type EnterpriseMemberAdmissionEvidenceService struct {
	repo EnterpriseMemberAdmissionEvidenceRepository
	now  func() time.Time
}

func NewEnterpriseMemberAdmissionEvidenceService(repo EnterpriseMemberAdmissionEvidenceRepository) *EnterpriseMemberAdmissionEvidenceService {
	return &EnterpriseMemberAdmissionEvidenceService{repo: repo, now: time.Now}
}

type enterpriseMemberAdmissionEvidenceAutoStopProvider struct {
	service *EnterpriseMemberAdmissionEvidenceService
}

func NewEnterpriseMemberAdmissionEvidenceAutoStopProvider(service *EnterpriseMemberAdmissionEvidenceService) EnterpriseMemberModelAdmissionAutoStopEvidenceProvider {
	return enterpriseMemberAdmissionEvidenceAutoStopProvider{service: service}
}

func (p enterpriseMemberAdmissionEvidenceAutoStopProvider) SummarizeEnterpriseMemberModelAdmissionAutoStopEvidence(ctx context.Context) (EnterpriseMemberModelAdmissionAutoStopEvidenceSummary, error) {
	if p.service == nil {
		return EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{}, errors.New("enterprise member admission evidence service is not configured")
	}
	summary, err := p.service.GetSummary(ctx)
	if err != nil {
		return EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{}, err
	}
	if summary == nil {
		return EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{}, errors.New("enterprise member admission evidence summary is missing")
	}
	return enterpriseMemberAdmissionAutoStopEvidenceFromSummary(summary), nil
}

func enterpriseMemberAdmissionAutoStopEvidenceFromSummary(summary *EnterpriseMemberAdmissionEvidenceSummary) EnterpriseMemberModelAdmissionAutoStopEvidenceSummary {
	if summary == nil {
		return EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{}
	}
	return EnterpriseMemberModelAdmissionAutoStopEvidenceSummary{
		GeneratedAt:                        summary.GeneratedAt,
		Window:                             fmt.Sprintf("%s..%s", summary.LookbackStart.UTC().Format(time.RFC3339), summary.LookbackEnd.UTC().Format(time.RFC3339)),
		EnforceSamples:                     summary.CanaryComparison.CanaryTotal,
		ControlSamples:                     summary.CanaryComparison.ControlTotal,
		EnforceSuccessRatePermille:         summary.CanaryComparison.CanarySuccessRateBps / 10,
		ControlSuccessRatePermille:         summary.CanaryComparison.ControlSuccessRateBps / 10,
		UnreviewedAliasActiveCount:         summary.AliasAudit.UnreviewedActive30d,
		EvaluationFailedPermille:           summary.PlannerHealth.EvaluationFailedRateBps / 10,
		LKGGenerationMismatchCount:         summary.LKG.LKGGenerationMismatch,
		LKGStaleHitCount:                   summary.LKG.LKGStaleOrExpired,
		PlannerP95Ms:                       summary.PlannerHealth.P95Ms,
		PlannerP99Ms:                       summary.PlannerHealth.P99Ms,
		UnpublishedModelActualAttemptCount: summary.UnpublishedGuard.ActualAttemptViolations,
	}
}

func (s *EnterpriseMemberAdmissionEvidenceService) GetSummary(ctx context.Context) (*EnterpriseMemberAdmissionEvidenceSummary, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("enterprise member admission evidence service is not configured")
	}
	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}
	input := NormalizeEnterpriseMemberAdmissionEvidenceInput(EnterpriseMemberAdmissionEvidenceInput{Now: now})
	summary, err := s.repo.GetEnterpriseMemberAdmissionEvidence(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("get enterprise member admission evidence: %w", err)
	}
	return EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary, input), nil
}

func NormalizeEnterpriseMemberAdmissionEvidenceInput(input EnterpriseMemberAdmissionEvidenceInput) EnterpriseMemberAdmissionEvidenceInput {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	} else {
		input.Now = input.Now.UTC()
	}
	if input.ShadowFullDays <= 0 {
		input.ShadowFullDays = 7
	}
	if input.LookbackDays <= 0 {
		input.LookbackDays = 30
	}
	if input.MaxCanarySuccessRateDeclineBps <= 0 {
		input.MaxCanarySuccessRateDeclineBps = 200
	}
	if input.MaxPlannerEvaluationFailedRateBps <= 0 {
		input.MaxPlannerEvaluationFailedRateBps = 100
	}
	return input
}

func EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary *EnterpriseMemberAdmissionEvidenceSummary, input EnterpriseMemberAdmissionEvidenceInput) *EnterpriseMemberAdmissionEvidenceSummary {
	input = NormalizeEnterpriseMemberAdmissionEvidenceInput(input)
	if summary == nil {
		summary = &EnterpriseMemberAdmissionEvidenceSummary{}
	}
	summary.GeneratedAt = input.Now
	summary.ShadowFullDays = input.ShadowFullDays
	summary.LookbackEnd = input.Now
	if summary.FullDaysEnd.IsZero() {
		summary.FullDaysEnd = time.Date(input.Now.Year(), input.Now.Month(), input.Now.Day(), 0, 0, 0, 0, time.UTC)
	}
	if summary.FullDaysStart.IsZero() {
		summary.FullDaysStart = summary.FullDaysEnd.AddDate(0, 0, -input.ShadowFullDays)
	}
	if summary.LookbackStart.IsZero() {
		summary.LookbackStart = input.Now.AddDate(0, 0, -input.LookbackDays)
	}
	summary.CanaryComparison.MaxAllowedDeclineBps = input.MaxCanarySuccessRateDeclineBps
	summary.PlannerHealth.MaxEvaluationFailedBps = input.MaxPlannerEvaluationFailedRateBps
	summary.CanaryComparison.CanaryTotal = summary.CanaryComparison.CanarySuccesses + summary.CanaryComparison.CanaryErrors
	summary.CanaryComparison.ControlTotal = summary.CanaryComparison.ControlSuccesses + summary.CanaryComparison.ControlErrors
	summary.CanaryComparison.CanarySuccessRateBps = rateBps(summary.CanaryComparison.CanarySuccesses, summary.CanaryComparison.CanaryTotal)
	summary.CanaryComparison.ControlSuccessRateBps = rateBps(summary.CanaryComparison.ControlSuccesses, summary.CanaryComparison.ControlTotal)
	summary.CanaryComparison.DeclineBps = summary.CanaryComparison.ControlSuccessRateBps - summary.CanaryComparison.CanarySuccessRateBps
	if summary.CanaryComparison.DeclineBps < 0 {
		summary.CanaryComparison.DeclineBps = 0
	}
	summary.PlannerHealth.EvaluationFailedRateBps = rateBps(summary.PlannerHealth.EvaluationFailed, summary.PlannerHealth.Evaluations)
	summary.LKG.LKGTotal = summary.LKG.LKGSuccesses + summary.LKG.LKGErrors
	summary.LKG.LKGSuccessRateBps = rateBps(summary.LKG.LKGSuccesses, summary.LKG.LKGTotal)

	summary.Ready = false
	switch {
	case !shadowFullDayEvidenceReady(summary.DailyShadowEvidence, input.ShadowFullDays):
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonShadowCoverageInsufficient
	case hasDailyUnreviewedLegacySuccessNewPruned(summary.DailyShadowEvidence):
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonAliasReviewPending
	case summary.AliasAudit.UnreviewedActive30d > 0:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonAliasReviewPending
	case summary.ShadowKeptBaseline.Successes == 0:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonShadowKeptBaselineMissing
	case summary.CanaryComparison.CanaryTotal == 0 || summary.CanaryComparison.ControlTotal == 0:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonCanaryEvidenceMissing
	case summary.CanaryComparison.DeclineBps > input.MaxCanarySuccessRateDeclineBps:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonCanarySuccessRateRegression
	case summary.UnpublishedGuard.ActualAttemptViolations > 0:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonUnpublishedAttemptViolation
	case summary.UnpublishedGuard.ModelUnpublishedNoCandidateErrors == 0:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonUnpublishedEvidenceMissing
	case summary.PlannerHealth.Evaluations == 0 || summary.PlannerHealth.EvaluationFailedRateBps > input.MaxPlannerEvaluationFailedRateBps:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonPlannerEvaluationFailed
	case summary.PlannerHealth.LatencySamples == 0:
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonPlannerLatencyMissing
	default:
		summary.Ready = true
		summary.Reason = EnterpriseMemberAdmissionEvidenceReasonReady
	}
	return summary
}

func shadowFullDayEvidenceReady(days []EnterpriseMemberAdmissionDailyShadowEvidence, want int) bool {
	if want <= 0 || len(days) != want {
		return false
	}
	for _, day := range days {
		if day.ShadowSamples <= 0 {
			return false
		}
	}
	return true
}

func hasDailyUnreviewedLegacySuccessNewPruned(days []EnterpriseMemberAdmissionDailyShadowEvidence) bool {
	for _, day := range days {
		if day.UnreviewedLegacySuccessNewPruned > 0 {
			return true
		}
	}
	return false
}

func rateBps(numerator, denominator int64) int64 {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	return numerator * 10000 / denominator
}
