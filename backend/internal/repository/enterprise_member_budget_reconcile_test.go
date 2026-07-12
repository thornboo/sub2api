package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetReconcileRepairsEvidenceAndProjection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, member_id, period_start, used_usd, reserved_usd`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "member_id", "period_start", "used_usd", "reserved_usd"}).
			AddRow(int64(9), int64(42), periodStart, 3.0, 1.0))
	mock.ExpectExec(`UPDATE enterprise_member_budget_entries entry`).
		WithArgs(int64(42), "2026-07-01").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_reservations reservation`).
		WithArgs(int64(42), "2026-07-01").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_entries`).
		WithArgs(int64(42), "2026-07-01", enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount_usd\), 0\)`).
		WithArgs(int64(42), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{"used_usd"}).AddRow(5.0))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(reserved_usd\), 0\)`).
		WithArgs(int64(42), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{"reserved_usd"}).AddRow(2.0))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(5.0, 2.0, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	result, err := repo.ReconcilePeriods(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, result.PeriodsChecked)
	require.Equal(t, 2, result.EvidenceLinksRepaired)
	require.Equal(t, 1, result.MissingEntriesCreated)
	require.Equal(t, 1, result.ProjectionsRebuilt)
	require.NoError(t, mock.ExpectationsWereMet())
}
