package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetReserveCreatesZeroAmountReceiptForUnlimitedMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	groupID := int64(7)
	requestID := "17:client:req-1"
	payloadHash := service.HashUsageRequestPayload([]byte(`{"model":"gpt-test"}`))
	expiresAt := time.Date(2026, time.July, 15, 14, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnError(sql.ErrNoRows)
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
	mock.ExpectQuery(`INSERT INTO enterprise_member_budget_reservations`).
		WithArgs(requestID, memberID, &groupID, payloadHash, sqlmock.AnyArg(), 0.0, service.EnterpriseMemberReceiptKindLegacy, expiresAt).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(91)))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.Reserve(context.Background(), requestID, memberID, &groupID, payloadHash, 0, expiresAt)

	require.NoError(t, err)
	require.Equal(t, int64(91), receipt.ID)
	require.Equal(t, &groupID, receipt.GroupID)
	require.Equal(t, payloadHash, receipt.PayloadHash)
	require.Zero(t, receipt.ReservedUSD)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetGetReservationSupportsAsyncTaskRecovery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requestID := "17:client:async-image"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash, period_start`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start",
			"reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at",
		}).AddRow(int64(91), requestID, int64(42), int64(7), "payload", periodStart, 6.0, 0.0, "ambiguous", service.EnterpriseMemberReceiptKindAsyncImage, "task-91", service.EnterpriseMemberAsyncTaskPhaseExecuting, nil, expiresAt))

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.GetReservation(context.Background(), requestID)
	require.NoError(t, err)
	require.Equal(t, int64(91), receipt.ID)
	require.Equal(t, "ambiguous", receipt.Status)
	require.Equal(t, 6.0, receipt.ReservedUSD)
	require.Equal(t, service.EnterpriseMemberReceiptKindAsyncImage, receipt.ReceiptKind)
	require.Equal(t, "task-91", receipt.TaskID)
	require.Equal(t, service.EnterpriseMemberAsyncTaskPhaseExecuting, receipt.TaskPhase)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetGetReservationByTaskIDSupportsRedisLossFallback(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requestID := "17:client:redis-key-lost"
	taskID := "imgtask_lost"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 20, 8, 0, 0, 0, time.UTC)
	createdAt := expiresAt.Add(-time.Hour)
	mock.ExpectQuery(`(?s)SELECT id, request_id, member_id.*FROM enterprise_member_budget_reservations.*WHERE async_task_id = \$1`).
		WithArgs(taskID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start",
			"reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase",
			"usage_log_id", "expires_at", "created_at",
		}).AddRow(int64(91), requestID, int64(42), int64(7), "payload", periodStart, 6.0, 0.0, "ambiguous",
			service.EnterpriseMemberReceiptKindAsyncImage, taskID, service.EnterpriseMemberAsyncTaskPhaseExecuting,
			nil, expiresAt, createdAt))

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.GetReservationByTaskID(context.Background(), taskID)
	require.NoError(t, err)
	require.Equal(t, requestID, receipt.RequestID)
	require.Equal(t, taskID, receipt.TaskID)
	require.Equal(t, createdAt, receipt.CreatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetPersistsAsyncTaskLifecycleFence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requestID := "17:client:async-image"
	taskID := "imgtask_91"
	expiresAt := time.Date(2026, time.July, 20, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET async_task_id = \$2, async_task_phase = 'queued'.*WHERE request_id = \$1`).
		WithArgs(requestID, taskID, expiresAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET async_task_phase = 'executing'.*WHERE request_id = \$1 AND async_task_id = \$2`).
		WithArgs(requestID, taskID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &enterpriseMemberBudgetRepository{db: db}
	require.NoError(t, repo.AttachAsyncTask(context.Background(), requestID, taskID, expiresAt))
	require.NoError(t, repo.MarkAsyncTaskExecuting(context.Background(), requestID, taskID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetReleaseAsyncTaskRequiresMatchingTaskID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 20, 10, 0, 0, 0, time.UTC)
	requestID := "17:client:async-image"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id, member_id, group_id, period_start.*WHERE request_id = \$1.*FOR UPDATE`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member_id", "group_id", "period_start", "reserved_usd", "actual_usd", "status",
			"receipt_kind", "async_task_id", "async_task_phase", "expires_at",
		}).AddRow(int64(91), int64(42), int64(7), periodStart, 4.0, 0.0, "reserved",
			service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_original", service.EnterpriseMemberAsyncTaskPhaseExecuting, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.ReleaseAsyncTask(context.Background(), requestID, "imgtask_reused_request")
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetGenericReleaseRejectsBoundAsyncImageTask(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:async-image"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status, receipt_kind, COALESCE\(async_task_id, ''\)`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id",
		}).AddRow(int64(42), periodStart, 4.0, "reserved", service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_original"))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	err = repo.Release(context.Background(), requestID)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetGenericReleaseAllowsUnboundAsyncImageReceipt(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	memberID := int64(42)
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:async-image-before-task"

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status, receipt_kind, COALESCE\(async_task_id, ''\)`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id",
		}).AddRow(memberID, periodStart, 4.0, "reserved", service.EnterpriseMemberReceiptKindAsyncImage, ""))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods SET reserved_usd`).
		WithArgs(4.0, memberID, periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_reservations SET status = 'released'`).
		WithArgs(requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	require.NoError(t, repo.Release(context.Background(), requestID))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetReleaseAsyncTaskReturnsAuthoritativeStatus(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 20, 10, 0, 0, 0, time.UTC)
	requestID := "17:client:async-image"
	taskID := "imgtask_91"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id, member_id, group_id, period_start.*WHERE request_id = \$1.*FOR UPDATE`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member_id", "group_id", "period_start", "reserved_usd", "actual_usd", "status",
			"receipt_kind", "async_task_id", "async_task_phase", "expires_at",
		}).AddRow(int64(91), int64(42), int64(7), periodStart, 4.0, 0.0, "reserved",
			service.EnterpriseMemberReceiptKindAsyncImage, taskID, service.EnterpriseMemberAsyncTaskPhaseExecuting, expiresAt))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_periods.*reserved_usd = GREATEST\(0, reserved_usd - \$1\)`).
		WithArgs(4.0, int64(42), periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET status = 'released'`).
		WithArgs(requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.ReleaseAsyncTask(context.Background(), requestID, taskID)
	require.NoError(t, err)
	require.Equal(t, "released", receipt.Status)
	require.Equal(t, taskID, receipt.TaskID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetMarkAsyncTaskAmbiguousRequiresMatchingTaskID(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	requestID := "17:client:async-image"
	expiresAt := time.Date(2026, time.July, 20, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT id, member_id, reserved_usd.*WHERE request_id = \$1.*FOR UPDATE`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member_id", "reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "expires_at",
		}).AddRow(int64(91), int64(42), 4.0, 0.0, "reserved", service.EnterpriseMemberReceiptKindAsyncImage,
			"imgtask_original", service.EnterpriseMemberAsyncTaskPhaseExecuting, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.MarkAsyncTaskAmbiguous(context.Background(), requestID, "imgtask_other", "outcome_unknown")
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetGenericAmbiguousRejectsBoundAsyncImageTask(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	requestID := "17:client:async-image"

	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*AND NOT \(receipt_kind = 'async_image' AND async_task_id IS NOT NULL\)`).
		WithArgs("outcome_unknown", requestID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT status, receipt_kind, COALESCE\(async_task_id, ''\)`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "receipt_kind", "async_task_id"}).
			AddRow("reserved", service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_original"))

	repo := &enterpriseMemberBudgetRepository{db: db}
	err = repo.MarkAmbiguous(context.Background(), requestID, "outcome_unknown")
	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetReserveReusesLegacyPositiveSyncReceiptAsZeroAmountRequest(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	groupID := int64(7)
	requestID := "17:client:req-legacy-sync"
	payloadHash := service.HashUsageRequestPayload([]byte(`{"model":"gpt-test"}`))
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 15, 14, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start",
			"reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at",
		}).AddRow(int64(91), requestID, memberID, groupID, payloadHash, periodStart, 253.38, 0.0, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", "", nil, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.ReserveWithKind(context.Background(), requestID, memberID, &groupID, payloadHash, 0, service.EnterpriseMemberReceiptKindSync, expiresAt)

	require.NoError(t, err)
	require.Equal(t, int64(91), receipt.ID)
	require.Equal(t, 253.38, receipt.ReservedUSD, "the legacy hold remains recoverable until settlement or release")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetReserveRejectsDifferentPositiveHoldForExistingRequest(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	groupID := int64(7)
	requestID := "17:client:req-async"
	payloadHash := service.HashUsageRequestPayload([]byte(`{"model":"gpt-image-2"}`))
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 15, 14, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start",
			"reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at",
		}).AddRow(int64(92), requestID, memberID, groupID, payloadHash, periodStart, 4.0, 0.0, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", "", nil, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.Reserve(context.Background(), requestID, memberID, &groupID, payloadHash, 5.0, expiresAt)

	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetMarkAmbiguousKeepsReservedProjectionHeld(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requestID := "17:client:req-ambiguous"
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET status = 'ambiguous'.*WHERE request_id = \$2 AND status = 'reserved'`).
		WithArgs("task_persistence_failed", requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &enterpriseMemberBudgetRepository{db: db}
	err = repo.MarkAmbiguous(context.Background(), requestID, "task_persistence_failed")

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetManualResolutionOnlyReleasesHold(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	receiptID := int64(91)
	memberID := int64(42)
	ownerID := int64(3)
	actorID := int64(7)
	groupID := int64(23)
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
	requestID := "17:client:req-ambiguous"

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT reservation.id, reservation.request_id.*WHERE reservation.id = \$1.*FOR UPDATE`).
		WithArgs(receiptID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "enterprise_user_id", "member_id", "member_code", "display_name",
			"group_id", "period_start", "reserved_usd", "receipt_kind", "async_task_id", "async_task_phase", "outcome_reason", "reconcile_attempts",
			"last_reconcile_at", "expires_at", "created_at", "updated_at", "status",
		}).AddRow(
			receiptID, requestID, ownerID, memberID, "member-42", "Member 42",
			groupID, periodStart, 4.5, service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_91", service.EnterpriseMemberAsyncTaskPhaseExecuting, "task_persistence_failed", 2,
			now, now.Add(10*time.Minute), now.Add(-time.Hour), now, "ambiguous",
		))
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM usage_logs WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3\)`).
		WithArgs(int64(17), "client:req-ambiguous", memberID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM usage_billing_dedup.*UNION ALL.*FROM usage_billing_dedup_archive`).
		WithArgs(int64(17), "client:req-ambiguous").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM enterprise_member_usage_settlement_outbox.*WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3`).
		WithArgs(int64(17), "client:req-ambiguous", memberID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM batch_image_jobs.*WHERE member_budget_request_id = \$1.*status NOT IN \('failed', 'cancelled', 'output_deleted'\)`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_periods.*reserved_usd = GREATEST\(0, reserved_usd - \$1\)`).
		WithArgs(4.5, memberID, periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET actual_usd = \$1, status = \$2, outcome_reason = \$3`).
		WithArgs(0, "released", "manual_release", receiptID, 2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)INSERT INTO enterprise_member_audit_logs.*member.budget_receipt_reconciled`).
		WithArgs(ownerID, memberID, actorID, receiptID, 4.5, "task_persistence_failed", 2, "released", 0, "manual_release", "release", "upstream confirmed no task exists").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.ResolveAmbiguousReceipt(context.Background(), receiptID, service.EnterpriseMemberAmbiguousReceiptResolution{
		Decision:                  service.EnterpriseMemberReceiptDecisionRelease,
		ExpectedReconcileAttempts: 2,
		Reason:                    "upstream confirmed no task exists",
	}, actorID)

	require.NoError(t, err)
	require.Equal(t, receiptID, receipt.ID)
	require.Equal(t, "manual_release", receipt.OutcomeReason)
	require.Equal(t, 3, receipt.ReconcileAttempts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetManualReleaseRejectsConcurrentUsageEvidence(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	receiptID := int64(91)
	memberID := int64(42)
	ownerID := int64(3)
	groupID := int64(23)
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT reservation.id, reservation.request_id.*WHERE reservation.id = \$1.*FOR UPDATE`).
		WithArgs(receiptID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "enterprise_user_id", "member_id", "member_code", "display_name",
			"group_id", "period_start", "reserved_usd", "receipt_kind", "async_task_id", "async_task_phase", "outcome_reason", "reconcile_attempts",
			"last_reconcile_at", "expires_at", "created_at", "updated_at", "status",
		}).AddRow(
			receiptID, "17:client:req-raced", ownerID, memberID, "member-42", "Member 42",
			groupID, periodStart, 4.5, service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_raced", service.EnterpriseMemberAsyncTaskPhaseExecuting, "usage_persistence_failed", 2,
			now, now.Add(10*time.Minute), now.Add(-time.Hour), now, "ambiguous",
		))
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM usage_logs WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3\)`).
		WithArgs(int64(17), "client:req-raced", memberID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.ResolveAmbiguousReceipt(context.Background(), receiptID, service.EnterpriseMemberAmbiguousReceiptResolution{
		Decision:                  service.EnterpriseMemberReceiptDecisionRelease,
		ExpectedReconcileAttempts: 2,
		Reason:                    "upstream confirmed no task exists",
	}, int64(7))

	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetManualReleaseRejectsPendingSettlementOutbox(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	receiptID := int64(91)
	memberID := int64(42)
	ownerID := int64(3)
	groupID := int64(23)
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT reservation.id, reservation.request_id.*WHERE reservation.id = \$1.*FOR UPDATE`).
		WithArgs(receiptID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "enterprise_user_id", "member_id", "member_code", "display_name",
			"group_id", "period_start", "reserved_usd", "receipt_kind", "async_task_id", "async_task_phase", "outcome_reason", "reconcile_attempts",
			"last_reconcile_at", "expires_at", "created_at", "updated_at", "status",
		}).AddRow(
			receiptID, "17:client:req-pending", ownerID, memberID, "member-42", "Member 42",
			groupID, periodStart, 4.5, service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_pending", service.EnterpriseMemberAsyncTaskPhaseExecuting, "usage_persistence_failed", 2,
			now, now.Add(10*time.Minute), now.Add(-time.Hour), now, "ambiguous",
		))
	mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM usage_logs WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3\)`).
		WithArgs(int64(17), "client:req-pending", memberID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM usage_billing_dedup.*UNION ALL.*FROM usage_billing_dedup_archive`).
		WithArgs(int64(17), "client:req-pending").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM enterprise_member_usage_settlement_outbox.*WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3`).
		WithArgs(int64(17), "client:req-pending", memberID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.ResolveAmbiguousReceipt(context.Background(), receiptID, service.EnterpriseMemberAmbiguousReceiptResolution{
		Decision:                  service.EnterpriseMemberReceiptDecisionRelease,
		ExpectedReconcileAttempts: 2,
		Reason:                    "upstream confirmed no task exists",
	}, int64(7))

	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetManualReleaseRejectsDurableSettlementEvidence(t *testing.T) {
	tests := []struct {
		name           string
		billingExists  bool
		outboxExists   bool
		batchJobExists bool
	}{
		{name: "archived billing dedup", billingExists: true},
		{name: "active or successful batch image settlement", batchJobExists: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			const (
				receiptID = int64(91)
				memberID  = int64(42)
				ownerID   = int64(3)
				groupID   = int64(23)
			)
			periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
			now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
			requestID := "17:client:req-durable-evidence"

			mock.ExpectBegin()
			mock.ExpectQuery(`(?s)SELECT reservation.id, reservation.request_id.*WHERE reservation.id = \$1.*FOR UPDATE`).
				WithArgs(receiptID).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "request_id", "enterprise_user_id", "member_id", "member_code", "display_name",
					"group_id", "period_start", "reserved_usd", "receipt_kind", "async_task_id", "async_task_phase", "outcome_reason", "reconcile_attempts",
					"last_reconcile_at", "expires_at", "created_at", "updated_at", "status",
				}).AddRow(
					receiptID, requestID, ownerID, memberID, "member-42", "Member 42",
					groupID, periodStart, 4.5, service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_evidence", service.EnterpriseMemberAsyncTaskPhaseExecuting, "usage_persistence_failed", 2,
					now, now.Add(10*time.Minute), now.Add(-time.Hour), now, "ambiguous",
				))
			mock.ExpectQuery(`SELECT EXISTS \(SELECT 1 FROM usage_logs WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3\)`).
				WithArgs(int64(17), "client:req-durable-evidence", memberID).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
			mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM usage_billing_dedup.*UNION ALL.*FROM usage_billing_dedup_archive`).
				WithArgs(int64(17), "client:req-durable-evidence").
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tt.billingExists))
			if !tt.billingExists {
				mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM enterprise_member_usage_settlement_outbox.*WHERE api_key_id = \$1 AND request_id = \$2 AND member_id = \$3`).
					WithArgs(int64(17), "client:req-durable-evidence", memberID).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tt.outboxExists))
			}
			if !tt.billingExists && !tt.outboxExists {
				mock.ExpectQuery(`(?s)SELECT EXISTS \(.*FROM batch_image_jobs.*WHERE member_budget_request_id = \$1.*status NOT IN \('failed', 'cancelled', 'output_deleted'\)`).
					WithArgs(requestID).
					WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tt.batchJobExists))
			}
			mock.ExpectRollback()

			repo := &enterpriseMemberBudgetRepository{db: db}
			_, err = repo.ResolveAmbiguousReceipt(context.Background(), receiptID, service.EnterpriseMemberAmbiguousReceiptResolution{
				Decision:                  service.EnterpriseMemberReceiptDecisionRelease,
				ExpectedReconcileAttempts: 2,
				Reason:                    "external evidence says no billable result exists",
			}, int64(7))

			require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEnterpriseMemberBudgetReserveRejectsRequestIDReuseWithDifferentPayload(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	groupID := int64(7)
	requestID := "17:client:req-1"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 15, 14, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start", "reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at"}).
			AddRow(int64(91), requestID, memberID, groupID, strings.Repeat("a", 64), periodStart, 0.0, 0.0, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", "", nil, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.Reserve(context.Background(), requestID, memberID, &groupID, strings.Repeat("b", 64), 0, expiresAt)

	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetReserveRejectsTerminalReceiptReuse(t *testing.T) {
	for _, status := range []string{"settled", "released", "expired", "ambiguous"} {
		t.Run(status, func(t *testing.T) {
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			memberID := int64(42)
			groupID := int64(7)
			requestID := "17:client:req-1"
			payloadHash := strings.Repeat("a", 64)
			periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
			expiresAt := time.Date(2026, time.July, 15, 14, 0, 0, 0, time.UTC)
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
				WithArgs(requestID).
				WillReturnRows(sqlmock.NewRows([]string{"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start", "reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at"}).
					AddRow(int64(91), requestID, memberID, groupID, payloadHash, periodStart, 0.0, 0.0, status, service.EnterpriseMemberReceiptKindLegacy, "", "", nil, expiresAt))
			mock.ExpectRollback()

			repo := &enterpriseMemberBudgetRepository{db: db}
			_, err = repo.Reserve(context.Background(), requestID, memberID, &groupID, payloadHash, 0, expiresAt)

			require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestEnterpriseMemberBudgetReserveValidatesEmptyPayloadReceiptGroup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	oldGroupID := int64(7)
	newGroupID := int64(8)
	requestID := "17:client:req-1"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, time.July, 15, 14, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start", "reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at"}).
			AddRow(int64(91), requestID, memberID, oldGroupID, "", periodStart, 0.0, 0.0, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", "", nil, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	_, err = repo.Reserve(context.Background(), requestID, memberID, &newGroupID, "", 0, expiresAt)

	require.ErrorIs(t, err, service.ErrEnterpriseMemberBudgetConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetReleasePropagatesLookupFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	lookupErr := errors.New("database unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs("17:client:req-1").
		WillReturnError(lookupErr)
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	err = repo.Release(context.Background(), "17:client:req-1")

	require.ErrorIs(t, err, lookupErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBatchImageMemberBudgetReleasePropagatesLookupFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	lookupErr := errors.New("database unavailable")
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs("batch:1").
		WillReturnError(lookupErr)
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = releaseBatchImageEnterpriseMemberBudget(context.Background(), tx, "batch:1")

	require.ErrorIs(t, err, lookupErr)
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLinkEnterpriseMemberBudgetUsageRequiresMatchingMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_entries entry.*entry\.member_id = \$4.*usage\.member_id = \$4`).
		WithArgs("17:client:req-1", "client:req-1", int64(17), memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations reservation.*reservation\.member_id = \$4.*usage\.member_id = \$4`).
		WithArgs("17:client:req-1", "client:req-1", int64(17), memberID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = linkEnterpriseMemberBudgetUsage(context.Background(), db, "client:req-1", 17, &memberID)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetRecoverExpiredKeepsUnknownOutcomeReserved(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:req-1"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT request_id, member_id, period_start, reserved_usd, status`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id", "async_task_phase"}).
			AddRow(requestID, int64(42), periodStart, 3.5, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", ""))
	mock.ExpectQuery(`SELECT id, actual_cost FROM usage_logs`).
		WithArgs(int64(17), "client:req-1", int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(int64(17), "client:req-1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET status = 'ambiguous'`).
		WithArgs(requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	recovered, err := repo.RecoverExpired(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetRecoverExpiredReleasesAsyncImageReceiptWithoutTaskLink(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:orphaned-async-image"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT request_id, member_id, period_start, reserved_usd, status`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id", "async_task_phase"}).
			AddRow(requestID, int64(42), periodStart, 3.5, "reserved", service.EnterpriseMemberReceiptKindAsyncImage, "", ""))
	mock.ExpectQuery(`SELECT id, actual_cost FROM usage_logs`).
		WithArgs(int64(17), "client:orphaned-async-image", int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(int64(17), "client:orphaned-async-image").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_periods.*reserved_usd = GREATEST\(0, reserved_usd - \$1\)`).
		WithArgs(3.5, int64(42), periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET status = 'released', outcome_reason = \$2`).
		WithArgs(requestID, "async_task_not_created").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	recovered, err := repo.RecoverExpired(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetRecoverExpiredReleasesLinkedAsyncImageStillQueued(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:lost-queued-task"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT request_id, member_id, period_start, reserved_usd, status`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id", "async_task_phase"}).
			AddRow(requestID, int64(42), periodStart, 3.5, "reserved", service.EnterpriseMemberReceiptKindAsyncImage, "imgtask_lost", service.EnterpriseMemberAsyncTaskPhaseQueued))
	mock.ExpectQuery(`SELECT id, actual_cost FROM usage_logs`).
		WithArgs(int64(17), "client:lost-queued-task", int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(int64(17), "client:lost-queued-task").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_periods.*reserved_usd = GREATEST\(0, reserved_usd - \$1\)`).
		WithArgs(3.5, int64(42), periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET status = 'released', outcome_reason = \$2`).
		WithArgs(requestID, "async_task_not_dispatched").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	recovered, err := repo.RecoverExpired(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetRecoverExpiredMarksMissingUsageAfterBillingAmbiguous(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:req-billed-without-usage"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT request_id, member_id, period_start, reserved_usd, status`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id", "async_task_phase"}).
			AddRow(requestID, int64(42), periodStart, 3.5, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", ""))
	mock.ExpectQuery(`SELECT id, actual_cost FROM usage_logs`).
		WithArgs(int64(17), "client:req-billed-without-usage", int64(42)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(int64(17), "client:req-billed-without-usage").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec(`(?s)UPDATE enterprise_member_budget_reservations.*SET status = 'ambiguous'`).
		WithArgs(requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	recovered, err := repo.RecoverExpired(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetRecoverExpiredSettlesKnownUnlimitedReceipt(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:req-known"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT request_id, member_id, period_start, reserved_usd, status`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id", "async_task_phase"}).
			AddRow(requestID, int64(42), periodStart, 0.0, "ambiguous", service.EnterpriseMemberReceiptKindLegacy, "", ""))
	mock.ExpectQuery(`SELECT id, actual_cost FROM usage_logs`).
		WithArgs(int64(17), "client:req-known", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actual_cost"}).AddRow(int64(99), 2.0))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(2.0, 0.0, int64(42), periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_rate_limit_periods`).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_rate_limit_periods`).
		WithArgs(2.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_reservations SET actual_usd`).
		WithArgs(2.0, int64(99), "settled_after_recovery", requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_entries`).
		WithArgs(int64(42), periodStart, requestID, 2.0, int64(99), "usage:"+requestID).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	recovered, err := repo.RecoverExpired(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberBudgetRecoverExpiredSettlesKnownUsageAtActualCost(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	requestID := "17:client:req-overrun"
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT request_id, member_id, period_start, reserved_usd, status`).
		WithArgs(100).
		WillReturnRows(sqlmock.NewRows([]string{"request_id", "member_id", "period_start", "reserved_usd", "status", "receipt_kind", "async_task_id", "async_task_phase"}).
			AddRow(requestID, int64(42), periodStart, 1.0, "reserved", service.EnterpriseMemberReceiptKindLegacy, "", ""))
	mock.ExpectQuery(`SELECT id, actual_cost FROM usage_logs`).
		WithArgs(int64(17), "client:req-overrun", int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actual_cost"}).AddRow(int64(99), 2.0))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(2.0, 1.0, int64(42), periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_rate_limit_periods`).
		WithArgs(2.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_member_budget_reservations SET actual_usd`).
		WithArgs(2.0, int64(99), "settled_after_overrun", requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_budget_entries`).
		WithArgs(int64(42), periodStart, requestID, 2.0, int64(99), "usage:"+requestID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberBudgetRepository{db: db}
	recovered, err := repo.RecoverExpired(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, 1, recovered)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleEnterpriseMemberBudgetAcceptsAmbiguousReceiptAndReleasesHold(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-1"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "period_start", "reserved_usd", "status"}).
			AddRow(memberID, periodStart, 5.0, "ambiguous"))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(2.0, 5.0, memberID, periodStart).
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
		MemberBudgetCost:      2,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleEnterpriseMemberBudgetHonorsUnlimitedReceiptAcrossLimitChange(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(42)
	requestID := "17:client:req-unlimited"
	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT member_id, period_start, reserved_usd, status`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{"member_id", "period_start", "reserved_usd", "status"}).
			AddRow(memberID, periodStart, 0.0, "reserved"))
	mock.ExpectExec(`UPDATE enterprise_member_budget_periods`).
		WithArgs(2.0, 0.0, memberID, periodStart).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO enterprise_member_rate_limit_periods`).
		WithArgs(memberID).
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
		MemberBudgetCost:      2,
	})
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
