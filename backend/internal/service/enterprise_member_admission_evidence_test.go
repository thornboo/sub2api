package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type admissionEvidenceRepoFake struct {
	summary *EnterpriseMemberAdmissionEvidenceSummary
	err     error
	input   EnterpriseMemberAdmissionEvidenceInput
	calls   int
}

func (f *admissionEvidenceRepoFake) GetEnterpriseMemberAdmissionEvidence(_ context.Context, input EnterpriseMemberAdmissionEvidenceInput) (*EnterpriseMemberAdmissionEvidenceSummary, error) {
	f.calls++
	f.input = input
	if f.err != nil {
		return nil, f.err
	}
	return f.summary, nil
}

func TestEnterpriseMemberAdmissionEvidenceSummaryReadyOnlyWithCompleteEvidence(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	summary := EvaluateEnterpriseMemberAdmissionEvidenceSummary(completeAdmissionEvidenceSummary(now), EnterpriseMemberAdmissionEvidenceInput{Now: now})

	require.True(t, summary.Ready)
	require.Equal(t, EnterpriseMemberAdmissionEvidenceReasonReady, summary.Reason)
	require.Equal(t, int64(10000), summary.CanaryComparison.CanarySuccessRateBps)
	require.Equal(t, int64(0), summary.CanaryComparison.DeclineBps)
	require.Equal(t, int64(0), summary.PlannerHealth.EvaluationFailedRateBps)
	require.Equal(t, int64(10000), summary.LKG.LKGSuccessRateBps)
}

func TestEnterpriseMemberAdmissionEvidenceSummaryFailsClosedOnMissingFullDaySamples(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	summary := completeAdmissionEvidenceSummary(now)
	summary.DailyShadowEvidence = summary.DailyShadowEvidence[:6]

	got := EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary, EnterpriseMemberAdmissionEvidenceInput{Now: now})

	require.False(t, got.Ready)
	require.Equal(t, EnterpriseMemberAdmissionEvidenceReasonShadowCoverageInsufficient, got.Reason)
}

func TestEnterpriseMemberAdmissionEvidenceSummaryBlocksPendingAliasAndActualAttempt(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	summary := completeAdmissionEvidenceSummary(now)
	summary.DailyShadowEvidence[0].UnreviewedLegacySuccessNewPruned = 1

	got := EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary, EnterpriseMemberAdmissionEvidenceInput{Now: now})
	require.False(t, got.Ready)
	require.Equal(t, EnterpriseMemberAdmissionEvidenceReasonAliasReviewPending, got.Reason)

	summary = completeAdmissionEvidenceSummary(now)
	summary.UnpublishedGuard.ActualAttemptViolations = 1
	got = EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary, EnterpriseMemberAdmissionEvidenceInput{Now: now})
	require.False(t, got.Ready)
	require.Equal(t, EnterpriseMemberAdmissionEvidenceReasonUnpublishedAttemptViolation, got.Reason)
}

func TestEnterpriseMemberAdmissionEvidenceSummaryBlocksCanaryRegression(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	summary := completeAdmissionEvidenceSummary(now)
	summary.CanaryComparison.CanarySuccesses = 90
	summary.CanaryComparison.CanaryErrors = 10
	summary.CanaryComparison.ControlSuccesses = 100
	summary.CanaryComparison.ControlErrors = 0

	got := EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary, EnterpriseMemberAdmissionEvidenceInput{Now: now})

	require.False(t, got.Ready)
	require.Equal(t, EnterpriseMemberAdmissionEvidenceReasonCanarySuccessRateRegression, got.Reason)
	require.Equal(t, int64(1000), got.CanaryComparison.DeclineBps)
}

func TestEnterpriseMemberAdmissionEvidenceServiceNormalizesInput(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	repo := &admissionEvidenceRepoFake{summary: completeAdmissionEvidenceSummary(now)}
	svc := NewEnterpriseMemberAdmissionEvidenceService(repo)
	svc.now = func() time.Time { return now }

	got, err := svc.GetSummary(context.Background())

	require.NoError(t, err)
	require.True(t, got.Ready)
	require.Equal(t, 7, repo.input.ShadowFullDays)
	require.Equal(t, 30, repo.input.LookbackDays)
	require.Equal(t, int64(200), repo.input.MaxCanarySuccessRateDeclineBps)
	require.Equal(t, int64(100), repo.input.MaxPlannerEvaluationFailedRateBps)
}

func TestEnterpriseMemberAdmissionEvidenceAutoStopProviderMapsBoundedMetrics(t *testing.T) {
	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	summary := completeAdmissionEvidenceSummary(now)
	summary.CanaryComparison.CanarySuccesses = 93
	summary.CanaryComparison.CanaryErrors = 7
	summary.CanaryComparison.ControlSuccesses = 99
	summary.CanaryComparison.ControlErrors = 1
	summary.AliasAudit.UnreviewedActive30d = 2
	summary.UnpublishedGuard.ActualAttemptViolations = 1
	summary.LKG.LKGGenerationMismatch = 1
	summary.LKG.LKGStaleOrExpired = 3
	repo := &admissionEvidenceRepoFake{summary: summary}
	svc := NewEnterpriseMemberAdmissionEvidenceService(repo)
	svc.now = func() time.Time { return now }

	got, err := NewEnterpriseMemberAdmissionEvidenceAutoStopProvider(svc).SummarizeEnterpriseMemberModelAdmissionAutoStopEvidence(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(100), got.EnforceSamples)
	require.Equal(t, int64(100), got.ControlSamples)
	require.Equal(t, int64(930), got.EnforceSuccessRatePermille)
	require.Equal(t, int64(990), got.ControlSuccessRatePermille)
	require.Equal(t, int64(2), got.UnreviewedAliasActiveCount)
	require.Equal(t, int64(1), got.UnpublishedModelActualAttemptCount)
	require.Equal(t, int64(1), got.LKGGenerationMismatchCount)
	require.Equal(t, int64(3), got.LKGStaleHitCount)
}

func completeAdmissionEvidenceSummary(now time.Time) *EnterpriseMemberAdmissionEvidenceSummary {
	fullEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	days := make([]EnterpriseMemberAdmissionDailyShadowEvidence, 0, 7)
	for i := 7; i > 0; i-- {
		days = append(days, EnterpriseMemberAdmissionDailyShadowEvidence{
			DayStart:                         fullEnd.AddDate(0, 0, -i),
			ShadowSamples:                    10,
			LegacySuccessNewPruned:           0,
			UnreviewedLegacySuccessNewPruned: 0,
		})
	}
	return &EnterpriseMemberAdmissionEvidenceSummary{
		DailyShadowEvidence: days,
		AliasAudit: EnterpriseMemberAdmissionAliasAuditEvidence{
			ActiveLegacySuccessNewPruned30d: 2,
			UnreviewedActive30d:             0,
		},
		ShadowKeptBaseline: EnterpriseMemberAdmissionShadowKeptBaseline{
			Successes:                70,
			DistinctEndpoints:        3,
			DistinctEnterpriseOwners: 5,
		},
		CanaryComparison: EnterpriseMemberAdmissionCanaryComparison{
			CanarySuccesses:  20,
			CanaryErrors:     0,
			ControlSuccesses: 70,
			ControlErrors:    0,
		},
		UnpublishedGuard: EnterpriseMemberAdmissionUnpublishedGuard{
			ModelUnpublishedNoCandidateErrors: 4,
			ActualAttemptViolations:           0,
		},
		PlannerHealth: EnterpriseMemberAdmissionPlannerHealth{
			Evaluations:      74,
			EvaluationFailed: 0,
			LatencySamples:   70,
			P95Ms:            4,
			P99Ms:            18,
		},
		LKG: EnterpriseMemberAdmissionLKGEvidence{
			LiveSuccesses: 19,
			LKGSuccesses:  1,
			LKGErrors:     0,
		},
	}
}
