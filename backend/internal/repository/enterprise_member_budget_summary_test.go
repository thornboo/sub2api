//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetSummaryUsesLedgerPeriodForRequestFacts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	periodEnd := periodStart.AddDate(0, 1, 0)
	mock.ExpectQuery(`SELECT m\.monthly_limit_usd`).
		WithArgs(int64(44), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{
			"monthly_limit_usd", "used_usd", "reserved_usd",
			"rate_limit_5h", "rate_limit_1d", "rate_limit_7d",
			"usage_5h", "usage_1d", "usage_7d",
			"window_5h_start", "window_1d_start", "window_7d_start",
		}).AddRow(100, 20, 5, 0, 0, 0, 0, 0, 0, nil, nil, nil))
	mock.ExpectQuery(`(?s)FROM enterprise_member_budget_entries entry\s+JOIN usage_logs usage.*entry\.period_start = \$2.*entry\.kind = 'usage'`).
		WithArgs(int64(44), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{"count", "input_tokens", "output_tokens"}).AddRow(1, 10, 20))
	mock.ExpectQuery(`FROM enterprise_member_import_usage_baselines`).
		WithArgs(int64(44), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{
			"billed_usd", "total_tokens", "input_tokens", "output_tokens",
			"cache_tokens", "cache_creation_tokens", "cache_read_tokens",
		}).AddRow(12.34, "9223372036854775808.00", "200.11", "100.22", "50.33", "40.44", "30.55"))

	repo := &enterpriseMemberBudgetRepository{db: db}
	summary, err := repo.GetSummary(context.Background(), 44, periodStart, periodEnd)
	require.NoError(t, err)
	require.Equal(t, int64(1), summary.RequestCount)
	require.Equal(t, int64(10), summary.InputTokens)
	require.Equal(t, int64(20), summary.OutputTokens)
	require.Equal(t, 80.0, summary.RemainingUSD, "customer remaining budget must ignore diagnostic reservations")
	require.Equal(t, "9223372036854775808.00", summary.MigrationTotalTokens.String())
	require.Equal(t, "200.11", summary.MigrationInputTokens.String())
	require.Equal(t, "100.22", summary.MigrationOutputTokens.String())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberOwnerBudgetSummaryUsesLedgerPeriodForRequestFacts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	periodEnd := periodStart.AddDate(0, 1, 0)
	mock.ExpectQuery(`(?s)FROM enterprise_members m.*FROM enterprise_member_budget_entries entry.*entry\.period_start = \$2.*entry\.kind = 'usage'`).
		WithArgs(int64(7), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member_code", "name", "status", "monthly_limit_usd", "removed_at",
			"used_usd", "reserved_usd", "request_count", "input_tokens", "output_tokens",
			"billed_usd", "total_tokens", "migration_input_tokens", "migration_output_tokens",
			"cache_tokens", "cache_creation_tokens", "cache_read_tokens",
		}).AddRow(44, "member-44", "Member 44", "active", 100, nil, 20, 5, 1, 10, 20, 12.34, "421.63", "200.11", "100.22", "50.33", "40.44", "30.55"))

	repo := &enterpriseMemberBudgetRepository{db: db}
	summary, err := repo.GetOwnerUsageSummary(context.Background(), 7, periodStart, periodEnd)
	require.NoError(t, err)
	require.Equal(t, int64(1), summary.RequestCount)
	require.Equal(t, int64(10), summary.InputTokens)
	require.Equal(t, int64(20), summary.OutputTokens)
	require.Equal(t, "421.63", summary.MigrationTotalTokens.String())
	require.Len(t, summary.Members, 1)
	require.Equal(t, 80.0, summary.Members[0].RemainingUSD, "owner remaining budget must reflect actual spend only")
	require.Equal(t, "421.63", summary.Members[0].MigrationTotalTokens.String())
	require.NoError(t, mock.ExpectationsWereMet())
}
