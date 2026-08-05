package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAdmissionEvidenceRepositoryAggregatesBoundedReadinessEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	fullEnd := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	fullStart := fullEnd.AddDate(0, 0, -7)
	lookbackStart := now.AddDate(0, 0, -30)

	dailyRows := sqlmock.NewRows([]string{
		"day_start", "shadow_samples", "legacy_success_new_pruned", "unreviewed_legacy_success_new_pruned",
	})
	for i := 7; i > 0; i-- {
		dailyRows.AddRow(fullEnd.AddDate(0, 0, -i), int64(10), int64(0), int64(0))
	}
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionDailyShadowSQL)).
		WithArgs(fullStart, fullEnd).
		WillReturnRows(dailyRows)
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionAliasAuditSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"active_30d", "unreviewed_30d"}).AddRow(int64(2), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionShadowKeptBaselineSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"successes", "distinct_endpoints", "distinct_enterprise_owners"}).AddRow(int64(70), int64(3), int64(5)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionCanarySQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"canary_successes", "canary_errors", "control_successes", "control_errors"}).AddRow(int64(20), int64(0), int64(70), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionUnpublishedGuardSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"model_unpublished_no_candidate_errors", "actual_attempt_violations"}).AddRow(int64(4), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionPlannerHealthSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"evaluations", "evaluation_failed", "latency_samples", "p95_ms", "p99_ms"}).AddRow(int64(74), int64(0), int64(70), float64(12.1), float64(18.8)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionLKGSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"live_successes", "lkg_successes", "lkg_errors", "lkg_misses", "lkg_stale_or_expired", "lkg_generation_mismatch"}).AddRow(int64(19), int64(1), int64(0), int64(0), int64(0), int64(0)))

	repo := NewEnterpriseMemberAdmissionEvidenceRepository(db)
	summary, err := repo.GetEnterpriseMemberAdmissionEvidence(context.Background(), service.EnterpriseMemberAdmissionEvidenceInput{Now: now})

	require.NoError(t, err)
	require.True(t, summary.Ready)
	require.Len(t, summary.DailyShadowEvidence, 7)
	require.Equal(t, int64(70), summary.ShadowKeptBaseline.Successes)
	require.Equal(t, int64(20), summary.CanaryComparison.CanarySuccesses)
	require.Equal(t, int64(13), summary.PlannerHealth.P95Ms)
	require.Equal(t, int64(19), summary.PlannerHealth.P99Ms)
	require.Equal(t, int64(1), summary.LKG.LKGSuccesses)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberAdmissionEvidenceRepositoryDoesNotTreatZeroDailyRowsAsReady(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)
	fullEnd := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
	fullStart := fullEnd.AddDate(0, 0, -7)
	lookbackStart := now.AddDate(0, 0, -30)

	dailyRows := sqlmock.NewRows([]string{
		"day_start", "shadow_samples", "legacy_success_new_pruned", "unreviewed_legacy_success_new_pruned",
	})
	for i := 7; i > 0; i-- {
		dailyRows.AddRow(fullEnd.AddDate(0, 0, -i), int64(0), int64(0), int64(0))
	}
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionDailyShadowSQL)).
		WithArgs(fullStart, fullEnd).
		WillReturnRows(dailyRows)
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionAliasAuditSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"active_30d", "unreviewed_30d"}).AddRow(int64(0), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionShadowKeptBaselineSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"successes", "distinct_endpoints", "distinct_enterprise_owners"}).AddRow(int64(0), int64(0), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionCanarySQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"canary_successes", "canary_errors", "control_successes", "control_errors"}).AddRow(int64(0), int64(0), int64(0), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionUnpublishedGuardSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"model_unpublished_no_candidate_errors", "actual_attempt_violations"}).AddRow(int64(0), int64(0)))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionPlannerHealthSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"evaluations", "evaluation_failed", "latency_samples", "p95_ms", "p99_ms"}).AddRow(int64(0), int64(0), int64(0), nil, nil))
	mock.ExpectQuery(regexp.QuoteMeta(enterpriseMemberAdmissionLKGSQL)).
		WithArgs(lookbackStart, now).
		WillReturnRows(sqlmock.NewRows([]string{"live_successes", "lkg_successes", "lkg_errors", "lkg_misses", "lkg_stale_or_expired", "lkg_generation_mismatch"}).AddRow(int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)))

	repo := NewEnterpriseMemberAdmissionEvidenceRepository(db)
	summary, err := repo.GetEnterpriseMemberAdmissionEvidence(context.Background(), service.EnterpriseMemberAdmissionEvidenceInput{Now: now})

	require.NoError(t, err)
	require.False(t, summary.Ready)
	require.Equal(t, service.EnterpriseMemberAdmissionEvidenceReasonShadowCoverageInsufficient, summary.Reason)
	require.NoError(t, mock.ExpectationsWereMet())
}
