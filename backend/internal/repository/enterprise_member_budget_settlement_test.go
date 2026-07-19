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

func TestSettleEnterpriseMemberBudgetRecordsUnlimitedMemberWithoutReservation(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-1"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d FROM enterprise_members`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d"}).AddRow(0.0, 0.0, 0.0, 0.0))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg(), enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_entries`).
		WithArgs(memberID, sqlmock.AnyArg(), requestID, 1.25, "usage:"+requestID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(1.25, memberID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = settleEnterpriseMemberBudget(context.Background(), tx, &service.UsageBillingCommand{
		MemberID:              &memberID,
		MemberBudgetRequestID: requestID,
		MemberBudgetCost:      1.25,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleEnterpriseMemberBudgetRejectsLimitedMemberWithoutReservation(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-2"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d FROM enterprise_members`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d"}).AddRow(100.0, 0.0, 0.0, 0.0))
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = settleEnterpriseMemberBudget(context.Background(), tx, &service.UsageBillingCommand{
		MemberID:              &memberID,
		MemberBudgetRequestID: requestID,
		MemberBudgetCost:      1.25,
	})
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleEnterpriseMemberBudgetPersistsActualCostWhenEstimateIsExceeded(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-overrun"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "period_start", "reserved_usd", "status"}).
			AddRow(memberID, periodStart, 1.0, "reserved"))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(2.0, 1.0, memberID, periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_rate_limit_periods`).
		WithArgs(2.0, memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_reservations`).
		WithArgs(2.0, requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_entries`).
		WithArgs(memberID, periodStart, requestID, 2.0, "usage:"+requestID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = settleEnterpriseMemberBudget(context.Background(), tx, &service.UsageBillingCommand{
		MemberID:              &memberID,
		MemberBudgetRequestID: requestID,
		MemberBudgetCost:      2.0,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleEnterpriseMemberBudgetAllowsZeroReceiptToCrossLimitOnFinalRequest(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-final"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "period_start", "reserved_usd", "status"}).
			AddRow(memberID, periodStart, 0.0, "reserved"))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(0.30, 0.0, memberID, periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_rate_limit_periods`).
		WithArgs(memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_rate_limit_periods`).
		WithArgs(0.30, memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_reservations`).
		WithArgs(0.30, requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_entries`).
		WithArgs(memberID, periodStart, requestID, 0.30, "usage:"+requestID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = settleEnterpriseMemberBudget(context.Background(), tx, &service.UsageBillingCommand{
		MemberID:              &memberID,
		MemberBudgetRequestID: requestID,
		MemberBudgetCost:      0.30,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveBatchImageEnterpriseMemberBudgetCreatesZeroReceiptForUnlimitedMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, status, deleted_at FROM enterprise_members`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "status", "deleted_at"}).
			AddRow(0.0, 0.0, 0.0, 0.0, service.EnterpriseMemberStatusActive, nil))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg(), enterpriseBudgetTimezone()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods`).
		WithArgs(memberID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"used_usd", "reserved_usd"}).AddRow(0.0, 0.0))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_reservations`).
		WithArgs("batch:1", memberID, nil, "", sqlmock.AnyArg(), 0.0, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = reserveBatchImageEnterpriseMemberBudget(context.Background(), tx, &service.BatchImageBalanceHoldCommand{
		MemberID:              &memberID,
		MemberBudgetRequestID: "batch:1",
		HoldAmount:            10,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
