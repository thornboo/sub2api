package repository

import (
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetSummaryIncludesArchivedMembers(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	periodEnd := periodStart.AddDate(0, 1, 0)
	mock.ExpectQuery(`WHERE m\.id = \$1\s*$`).
		WithArgs(int64(11), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{
			"monthly_limit_usd", "used_usd", "reserved_usd",
			"rate_limit_5h", "rate_limit_1d", "rate_limit_7d",
			"usage_5h", "usage_1d", "usage_7d",
			"window_5h_start", "window_1d_start", "window_7d_start",
		}).AddRow(100, 30, 0, 25, 50, 75, 0, 0, 0, nil, nil, nil))
	mock.ExpectQuery(`(?s)FROM enterprise_member_budget_entries entry\s+JOIN usage_logs usage.*entry\.period_start = \$2.*entry\.kind = 'usage'`).
		WithArgs(int64(11), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{"count", "input_tokens", "output_tokens"}).AddRow(2, 100, 50))
	mock.ExpectQuery(regexp.QuoteMeta("FROM enterprise_member_import_usage_baselines WHERE member_id = $1 AND period_start = $2")).
		WithArgs(int64(11), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{
			"billed_usd", "total_tokens", "input_tokens", "output_tokens", "cache_tokens", "cache_creation_tokens", "cache_read_tokens",
		}).AddRow(0, 0, 0, 0, 0, 0, 0))

	repo := &enterpriseMemberBudgetRepository{db: db}
	summary, err := repo.GetSummary(t.Context(), 11, periodStart, periodEnd)
	require.NoError(t, err)
	require.Equal(t, 30.0, summary.UsedUSD)
	require.Equal(t, int64(2), summary.RequestCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberOwnerSummaryExcludesRemovedFactsFromCurrentTotalsAndItems(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	periodEnd := periodStart.AddDate(0, 1, 0)
	columns := []string{
		"id", "member_code", "name", "status", "monthly_limit_usd", "removed_at",
		"used_usd", "reserved_usd", "request_count", "input_tokens", "output_tokens",
		"billed_usd", "total_tokens", "migration_input_tokens", "migration_output_tokens",
		"cache_tokens", "cache_creation_tokens", "cache_read_tokens",
	}
	removedAt := periodStart.Add(24 * time.Hour)
	mock.ExpectQuery(`FROM enterprise_members m`).
		WithArgs(int64(7), "2026-07-01").
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(11, "~deleted~11", "Deleted member #11", "disabled", 100, removedAt, 30, 0, 2, 100, 50, 0, 0, 0, 0, 0, 0, 0).
			AddRow(12, "member-12", "Member 12", "active", 200, nil, 20, 0, 1, 40, 10, 0, 0, 0, 0, 0, 0, 0))

	repo := &enterpriseMemberBudgetRepository{db: db}
	summary, err := repo.GetOwnerUsageSummary(t.Context(), 7, periodStart, periodEnd)
	require.NoError(t, err)
	require.Equal(t, 20.0, summary.UsedUSD, "owner totals exclude facts from removed members")
	require.Equal(t, int64(1), summary.RequestCount)
	require.Equal(t, int64(40), summary.InputTokens)
	require.Equal(t, int64(10), summary.OutputTokens)
	require.Len(t, summary.Members, 1, "removed tombstones are not returned as manageable members")
	require.Equal(t, int64(12), summary.Members[0].MemberID)
	require.NoError(t, mock.ExpectationsWereMet())
}
