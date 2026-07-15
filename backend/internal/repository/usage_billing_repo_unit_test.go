//go:build unit

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const (
	conditionalBalanceDeductSQL = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance`
	overdraftBalanceDeductSQL   = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL\s+RETURNING balance`
	reserveBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance - \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) \+ \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND balance >= \$1\s+RETURNING balance, frozen_balance`
	captureBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance \+ \$1 - \$2,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$3 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1\s+RETURNING balance, frozen_balance`
	releaseBatchImageHoldSQL    = `(?s)UPDATE users\s+SET balance = balance \+ \$1,\s+frozen_balance = COALESCE\(frozen_balance, 0\) - \$1,\s+updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL AND COALESCE\(frozen_balance, 0\) >= \$1\s+RETURNING balance, frozen_balance`
	userExistsForBillingSQL     = `(?s)SELECT 1\s+FROM users\s+WHERE id = \$1 AND deleted_at IS NULL`
)

func TestDeductUsageBillingBalance_UsesSufficientBalanceGuard(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.5))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 2.5)
	require.NoError(t, err)
	require.True(t, sufficient)
	require.InDelta(t, 7.5, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductUsageBillingBalance_RecordsOverdraftWhenGuardMisses(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, 42, 10)
	require.NoError(t, err)
	require.False(t, sufficient)
	require.InDelta(t, -5.0, newBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUsageBillingEffects_FlagsBalanceOverdraft(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(-5.0))
	mock.ExpectCommit()

	result := &service.UsageBillingApplyResult{Applied: true}
	err = (&usageBillingRepository{}).applyUsageBillingEffects(ctx, tx, &service.UsageBillingCommand{
		UserID:      42,
		BalanceCost: 10,
	}, result)
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, -5.0, *result.NewBalance, 0.000001)
	require.True(t, result.BalanceOverdrafted)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_PersistsMemberUsageAtomically(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	memberID := int64(44)
	createdAt := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT member_id, status\s+FROM enterprise_member_budget_reservations.*FOR UPDATE`).
		WithArgs("5:req-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)INSERT INTO enterprise_member_usage_settlement_outbox.*SET command_payload = enterprise_member_usage_settlement_outbox.command_payload`).
		WithArgs(int64(5), memberID, int64(7), "req-1", "5:req-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(71))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT member_id, period_start, reserved_usd, status\s+FROM enterprise_member_budget_reservations`).
		WithArgs("5:req-1").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d FROM enterprise_members`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d"}).AddRow(0, 0, 0, 0))
	mock.ExpectExec(`(?s)INSERT INTO enterprise_member_budget_periods`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO enterprise_member_budget_entries`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_periods\s+SET used_usd`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)INSERT INTO usage_logs`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(91, createdAt))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_entries entry\s+SET usage_log_id = usage.id`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations reservation\s+SET usage_log_id = usage.id`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)DELETE FROM enterprise_member_usage_settlement_outbox`).
		WithArgs(int64(5), "req-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	usageLog := &service.UsageLog{
		UserID:    7,
		APIKeyID:  5,
		AccountID: 9,
		RequestID: "req-1",
		Model:     "gpt-test",
		MemberID:  &memberID,
	}
	repo := &usageBillingRepository{db: db}
	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:             "req-1",
		APIKeyID:              5,
		UserID:                7,
		AccountID:             9,
		MemberID:              &memberID,
		MemberBudgetRequestID: "5:req-1",
		UsageLog:              usageLog,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.True(t, result.UsageLogPersisted)
	require.Equal(t, int64(91), usageLog.ID)
	require.Equal(t, createdAt, usageLog.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_MemberUsageWriteFailureRollsBackBilling(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	memberID := int64(44)
	writeErr := errors.New("usage log insert failed")
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT member_id, status\s+FROM enterprise_member_budget_reservations.*FOR UPDATE`).
		WithArgs("5:req-rollback").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)INSERT INTO enterprise_member_usage_settlement_outbox`).
		WithArgs(int64(5), memberID, int64(7), "req-rollback", "5:req-rollback", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(72))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT member_id, period_start, reserved_usd, status\s+FROM enterprise_member_budget_reservations`).
		WithArgs("5:req-rollback").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d FROM enterprise_members`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d"}).AddRow(0, 0, 0, 0))
	mock.ExpectExec(`(?s)INSERT INTO enterprise_member_budget_periods`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO enterprise_member_budget_entries`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_periods\s+SET used_usd`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`(?s)INSERT INTO usage_logs`).
		WillReturnError(writeErr)
	mock.ExpectRollback()

	repo := &usageBillingRepository{db: db}
	result, err := repo.Apply(ctx, &service.UsageBillingCommand{
		RequestID:             "req-rollback",
		APIKeyID:              5,
		UserID:                7,
		AccountID:             9,
		MemberID:              &memberID,
		MemberBudgetRequestID: "5:req-rollback",
		UsageLog: &service.UsageLog{
			UserID:    7,
			APIKeyID:  5,
			AccountID: 9,
			RequestID: "req-rollback",
			Model:     "gpt-test",
			MemberID:  &memberID,
		},
	})
	require.ErrorIs(t, err, writeErr)
	require.Nil(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_RejectsMismatchedMemberUsageIdentity(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*service.UsageBillingCommand)
	}{
		{name: "owner", mutate: func(cmd *service.UsageBillingCommand) { cmd.UsageLog.UserID++ }},
		{name: "api key", mutate: func(cmd *service.UsageBillingCommand) { cmd.UsageLog.APIKeyID++ }},
		{name: "member", mutate: func(cmd *service.UsageBillingCommand) { other := *cmd.MemberID + 1; cmd.UsageLog.MemberID = &other }},
		{name: "request", mutate: func(cmd *service.UsageBillingCommand) { cmd.UsageLog.RequestID = "different-request" }},
		{name: "budget request", mutate: func(cmd *service.UsageBillingCommand) { cmd.MemberBudgetRequestID = "5:different-request" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			memberID := int64(44)
			cmd := &service.UsageBillingCommand{
				RequestID:             "client:req-identity",
				APIKeyID:              5,
				UserID:                7,
				MemberID:              &memberID,
				MemberBudgetRequestID: "5:client:req-identity",
				UsageLog: &service.UsageLog{
					RequestID: "client:req-identity",
					APIKeyID:  5,
					UserID:    7,
					MemberID:  &memberID,
				},
			}
			tt.mutate(cmd)

			result, err := (&usageBillingRepository{db: db}).Apply(context.Background(), cmd)

			require.Nil(t, result)
			require.ErrorIs(t, err, service.ErrUsageBillingRequestConflict)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUsageBillingRepositoryReplayPendingSettlementCleansAlreadyAppliedCommand(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	memberID := int64(44)
	cmd := &service.UsageBillingCommand{
		RequestID:             "client:req-replay",
		APIKeyID:              5,
		UserID:                7,
		AccountID:             9,
		MemberID:              &memberID,
		MemberBudgetRequestID: "5:client:req-replay",
		UsageLog: &service.UsageLog{
			UserID: 7, APIKeyID: 5, AccountID: 9, RequestID: "client:req-replay", MemberID: &memberID,
		},
	}
	cmd.Normalize()
	payload, err := json.Marshal(enterpriseMemberSettlementPayload{Version: enterpriseMemberSettlementPayloadVersion, Command: cmd})
	require.NoError(t, err)

	mock.ExpectQuery(`(?s)SELECT id, command_payload\s+FROM enterprise_member_usage_settlement_outbox`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"id", "command_payload"}).AddRow(int64(81), payload))
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT member_id, status\s+FROM enterprise_member_budget_reservations.*FOR UPDATE`).
		WithArgs("5:client:req-replay").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "status"}).AddRow(memberID, "settled"))
	mock.ExpectQuery(`(?s)INSERT INTO enterprise_member_usage_settlement_outbox`).
		WithArgs(int64(5), memberID, int64(7), "client:req-replay", "5:client:req-replay", cmd.RequestFingerprint, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(81)))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup`).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`(?s)SELECT request_fingerprint\s+FROM usage_billing_dedup`).
		WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint"}).AddRow(cmd.RequestFingerprint))
	mock.ExpectExec(`(?s)DELETE FROM enterprise_member_usage_settlement_outbox`).
		WithArgs(int64(5), "client:req-replay", cmd.RequestFingerprint).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &usageBillingRepository{db: db}
	replayed, err := repo.ReplayPendingEnterpriseMemberSettlements(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, replayed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryStageMemberSettlementRejectsReleasedReceipt(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	memberID := int64(44)
	cmd := &service.UsageBillingCommand{
		RequestID:             "client:req-released",
		APIKeyID:              5,
		UserID:                7,
		MemberID:              &memberID,
		MemberBudgetRequestID: "5:client:req-released",
		UsageLog: &service.UsageLog{
			UserID: 7, APIKeyID: 5, RequestID: "client:req-released", MemberID: &memberID,
		},
	}
	cmd.Normalize()

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT member_id, status\s+FROM enterprise_member_budget_reservations.*FOR UPDATE`).
		WithArgs("5:client:req-released").
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "status"}).AddRow(memberID, "released"))
	mock.ExpectRollback()

	repo := &usageBillingRepository{db: db}
	err = repo.stageEnterpriseMemberSettlement(context.Background(), cmd)

	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberSettlementPayloadExcludesHydratedSecrets(t *testing.T) {
	memberID := int64(44)
	payload, err := json.Marshal(enterpriseMemberSettlementPayload{
		Version: enterpriseMemberSettlementPayloadVersion,
		Command: &service.UsageBillingCommand{
			RequestID: "req-secret-boundary",
			APIKeyID:  5,
			UserID:    7,
			MemberID:  &memberID,
			UsageLog: &service.UsageLog{
				RequestID: "req-secret-boundary",
				APIKey:    &service.APIKey{Key: "sk-must-not-enter-outbox"},
				Account:   &service.Account{Name: "upstream-account-must-not-enter-outbox"},
			},
		},
	})
	require.NoError(t, err)
	require.NotContains(t, string(payload), "sk-must-not-enter-outbox")
	require.NotContains(t, string(payload), "upstream-account-must-not-enter-outbox")
}

func TestDeductUsageBillingBalance_ReturnsUserNotFoundWhenNoUserUpdated(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(conditionalBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(overdraftBalanceDeductSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	_, _, err = deductUsageBillingBalance(ctx, tx, 42, 10)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_MovesAvailableToFrozen(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(2.5, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(7.5, 2.5))
	mock.ExpectCommit()

	result, err := reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 2.5})
	require.NoError(t, err)
	require.NotNil(t, result.NewBalance)
	require.NotNil(t, result.FrozenBalance)
	require.InDelta(t, 7.5, *result.NewBalance, 0.000001)
	require.InDelta(t, 2.5, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReserveUsageBillingBatchImageBalance_InsufficientBalance(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(reserveBatchImageHoldSQL).
		WithArgs(10.0, int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(userExistsForBillingSQL).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectRollback()

	_, err = reserveUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 10})
	require.ErrorIs(t, err, service.ErrBatchImageInsufficientBalance)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ReleasesRemainder(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(1.0, 0.25, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(9.75, 0.0))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 1, ActualAmount: 0.25})
	require.NoError(t, err)
	require.InDelta(t, 9.75, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptureUsageBillingBatchImageBalance_ChargesActualCostOverHold(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(captureBatchImageHoldSQL).
		WithArgs(0.5, 1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(8.5, 0.0))
	mock.ExpectCommit()

	result, err := captureUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, HoldAmount: 0.5, ActualAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 8.5, *result.NewBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_ReturnsFrozenToAvailable(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_release"), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"?column?"}).AddRow(1))
	mock.ExpectQuery(releaseBatchImageHoldSQL).
		WithArgs(1.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "frozen_balance"}).AddRow(10.0, 0.0))
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_release", HoldAmount: 1})
	require.NoError(t, err)
	require.InDelta(t, 10.0, *result.NewBalance, 0.000001)
	require.InDelta(t, 0.0, *result.FrozenBalance, 0.000001)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReleaseUsageBillingBatchImageBalance_SkipsWhenHoldNeverReserved(t *testing.T) {
	ctx := context.Background()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	// dedup 与归档表均无 hold claim：说明该 job 从未成功冻结，
	// 释放必须跳过，不得从他人冻结资金池中凭空生成余额。
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT 1\s+FROM usage_billing_dedup_archive\s+WHERE request_id = \$1 AND api_key_id = \$2`).
		WithArgs(service.BatchImageHoldRequestID("imgbatch_phantom"), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectCommit()

	result, err := releaseUsageBillingBatchImageBalance(ctx, tx, &service.BatchImageBalanceHoldCommand{UserID: 42, APIKeyID: 7, BatchID: "imgbatch_phantom", HoldAmount: 1})
	require.NoError(t, err)
	require.Nil(t, result.NewBalance)
	require.Nil(t, result.FrozenBalance)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
