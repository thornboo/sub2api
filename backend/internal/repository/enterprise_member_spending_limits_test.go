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

func TestReserveEnterpriseMemberSpendingLimitsAggregatesPendingRequestsAcrossKeys(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 13, 3, 0, 0, 0, time.UTC)
	memberID := int64(42)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, status, deleted_at`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "status", "deleted_at"}).
			AddRow(0.0, 10.0, 0.0, 0.0, service.EnterpriseMemberStatusActive, nil))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg(), enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"used_usd", "reserved_usd"}).AddRow(0.0, 1.0))
	mock.ExpectExec(`INSERT INTO enterprise_member_rate_limit_periods`).
		WithArgs(memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT usage_5h, usage_1d, usage_7d, window_5h_start, window_1d_start, window_7d_start`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"usage_5h", "usage_1d", "usage_7d", "window_5h_start", "window_1d_start", "window_7d_start"}).
			AddRow(8.0, 0.0, 0.0, now.Add(-time.Hour), nil, nil))
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, _, err = reserveEnterpriseMemberSpendingLimits(context.Background(), tx, memberID, 2.0, now)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberAsyncBudgetUnavailable)
	require.Contains(t, err.Error(), "limit_window:5h")
	require.Contains(t, err.Error(), "settled_used_usd:8.000000")
	require.Contains(t, err.Error(), "active_task_holds_usd:1.000000")
	require.Contains(t, err.Error(), "requested_task_hold_usd:2.000000")
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveEnterpriseMemberSpendingLimitsZeroReceiptIgnoresHistoricalReservations(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	memberID := int64(42)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, status, deleted_at`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "status", "deleted_at"}).
			AddRow(300.0, 0.0, 0.0, 0.0, service.EnterpriseMemberStatusActive, nil))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg(), enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"used_usd", "reserved_usd"}).AddRow(39.64, 253.38))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, enforced, err := reserveEnterpriseMemberSpendingLimits(context.Background(), tx, memberID, 0, now)
	require.NoError(t, err)
	require.True(t, enforced)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveEnterpriseMemberSpendingLimitsZeroReceiptRejectsAlreadyExhaustedUsage(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	memberID := int64(42)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, status, deleted_at`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "status", "deleted_at"}).
			AddRow(300.0, 0.0, 0.0, 0.0, service.EnterpriseMemberStatusActive, nil))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg(), enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"used_usd", "reserved_usd"}).AddRow(300.0, 0.0))
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, _, err = reserveEnterpriseMemberSpendingLimits(context.Background(), tx, memberID, 0, now)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetExceeded)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveEnterpriseMemberSpendingLimitsPositiveHoldInitializesSettlementProjectionForMonthlyOnlyMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.July, 20, 3, 0, 0, 0, time.UTC)
	memberID := int64(42)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, status, deleted_at`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "status", "deleted_at"}).
			AddRow(300.0, 0.0, 0.0, 0.0, service.EnterpriseMemberStatusActive, nil))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg(), enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"used_usd", "reserved_usd"}).AddRow(39.64, 0.0))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(4.0, memberID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_rate_limit_periods`).
		WithArgs(memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, enforced, err := reserveEnterpriseMemberSpendingLimits(context.Background(), tx, memberID, 4.0, now)
	require.NoError(t, err)
	require.True(t, enforced)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleEnterpriseMemberBudgetRejectsRateLimitedMemberWithoutReservation(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-rate-only"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d FROM enterprise_members`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d"}).AddRow(0.0, 25.0, 0.0, 0.0))
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = settleEnterpriseMemberBudget(context.Background(), tx, &service.UsageBillingCommand{
		MemberID: &memberID, MemberBudgetRequestID: requestID, MemberBudgetCost: 1.25,
	})
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
