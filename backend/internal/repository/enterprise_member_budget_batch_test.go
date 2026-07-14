package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBatchAdjustUsageReplaysCompleteBatchInRequestOrder(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberBudgetRepository{db: db}
	delta := service.EnterpriseMemberUsageDelta{MonthlyUsedUSD: 2, Usage5h: -1}
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("batch-key").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM enterprise_member_audit_logs").
		WithArgs(int64(7), int64(11), "batch-key:11").
		WillReturnRows(sqlmock.NewRows([]string{
			"monthly_used_usd", "usage_5h", "usage_1d", "usage_7d",
			"monthly_used_delta", "usage_5h_delta", "usage_1d_delta", "usage_7d_delta", "expected_version",
		}).AddRow(11.0, 3.0, 4.0, 5.0, 2.0, -1.0, 0.0, 0.0, int64(3)))
	mock.ExpectQuery("SELECT .* FROM enterprise_member_audit_logs").
		WithArgs(int64(7), int64(22), "batch-key:22").
		WillReturnRows(sqlmock.NewRows([]string{
			"monthly_used_usd", "usage_5h", "usage_1d", "usage_7d",
			"monthly_used_delta", "usage_5h_delta", "usage_1d_delta", "usage_7d_delta", "expected_version",
		}).AddRow(22.0, 3.0, 4.0, 5.0, 2.0, -1.0, 0.0, 0.0, int64(4)))
	mock.ExpectCommit()

	updated, err := repo.BatchAdjustUsage(context.Background(), 7, time.Now(), []service.EnterpriseMemberBatchTarget{
		{ID: 22, ExpectedVersion: 4},
		{ID: 11, ExpectedVersion: 3},
	}, delta, 7, "batch-key", "system note")

	require.NoError(t, err)
	require.Equal(t, []int64{22, 11}, []int64{updated[0].ID, updated[1].ID})
	require.Equal(t, 22.0, updated[0].MonthlyUsedUSD)
	require.Equal(t, 11.0, updated[1].MonthlyUsedUSD)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBatchAdjustUsageRollsBackWhenDeltaWouldBecomeNegative(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberBudgetRepository{db: db}
	period := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("batch-key").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM enterprise_member_audit_logs").
		WithArgs(int64(7), int64(11), "batch-key:11").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT version FROM enterprise_members").
		WithArgs(int64(11), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(int64(3)))
	mock.ExpectExec("INSERT INTO enterprise_member_budget_periods").
		WithArgs(int64(11), "2026-07-01", enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO enterprise_member_rate_limit_periods").
		WithArgs(int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT used_usd FROM enterprise_member_budget_periods").
		WithArgs(int64(11), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{"used_usd"}).AddRow(3.0))
	mock.ExpectQuery("SELECT CASE WHEN window_5h_start").
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"usage_5h", "usage_1d", "usage_7d"}).AddRow(2.0, 4.0, 6.0))
	mock.ExpectRollback()

	_, err = repo.BatchAdjustUsage(context.Background(), 7, period, []service.EnterpriseMemberBatchTarget{{
		ID: 11, ExpectedVersion: 3,
	}}, service.EnterpriseMemberUsageDelta{MonthlyUsedUSD: -4}, 7, "batch-key", "system note")

	require.ErrorIs(t, err, service.ErrEnterpriseMemberInvalid)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBatchAdjustUsageAppliesDeltaToEffectiveWindowUsage(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberBudgetRepository{db: db}
	period := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("batch-key").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM enterprise_member_audit_logs").
		WithArgs(int64(7), int64(11), "batch-key:11").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("SELECT version FROM enterprise_members").
		WithArgs(int64(11), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"version"}).AddRow(int64(3)))
	mock.ExpectExec("INSERT INTO enterprise_member_budget_periods").
		WithArgs(int64(11), "2026-07-01", enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO enterprise_member_rate_limit_periods").
		WithArgs(int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT used_usd FROM enterprise_member_budget_periods").
		WithArgs(int64(11), "2026-07-01").
		WillReturnRows(sqlmock.NewRows([]string{"used_usd"}).AddRow(0.0))
	mock.ExpectQuery("SELECT CASE WHEN window_5h_start").
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"usage_5h", "usage_1d", "usage_7d"}).AddRow(0.0, 10.0, 20.0))
	mock.ExpectExec("UPDATE enterprise_member_rate_limit_periods").
		WithArgs(5.0, 10.0, 20.0, true, false, false, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO enterprise_member_audit_logs").
		WithArgs(
			int64(7), int64(11), int64(7),
			0.0, 0.0, 10.0, 20.0,
			0.0, 5.0, 10.0, 20.0,
			"system note", "batch-key:11", "batch-key",
			0.0, 5.0, 0.0, 0.0, int64(3),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.BatchAdjustUsage(context.Background(), 7, period, []service.EnterpriseMemberBatchTarget{{
		ID: 11, ExpectedVersion: 3,
	}}, service.EnterpriseMemberUsageDelta{Usage5h: 5}, 7, "batch-key", "system note")

	require.NoError(t, err)
	require.Equal(t, []service.BatchEnterpriseMemberUsageUpdate{{
		ID: 11, MonthlyUsedUSD: 0, Usage5h: 5, Usage1d: 10, Usage7d: 20,
	}}, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}
