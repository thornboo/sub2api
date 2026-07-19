package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseMemberBudgetRepository struct{ db *sql.DB }

func NewEnterpriseMemberBudgetRepository(db *sql.DB) service.EnterpriseMemberBudgetRepository {
	return &enterpriseMemberBudgetRepository{db: db}
}

func (r *enterpriseMemberBudgetRepository) Reserve(ctx context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, expiresAt time.Time) (_ *service.EnterpriseMemberBudgetReservation, err error) {
	return r.ReserveWithKind(ctx, requestID, memberID, groupID, payloadHash, amount, service.EnterpriseMemberReceiptKindLegacy, expiresAt)
}

func (r *enterpriseMemberBudgetRepository) ReserveWithKind(ctx context.Context, requestID string, memberID int64, groupID *int64, payloadHash string, amount float64, receiptKind string, expiresAt time.Time) (_ *service.EnterpriseMemberBudgetReservation, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	receiptKind = strings.TrimSpace(receiptKind)
	if !validEnterpriseMemberReceiptKind(receiptKind) {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var existing service.EnterpriseMemberBudgetReservation
	err = tx.QueryRowContext(ctx, `
		SELECT id, request_id, member_id, group_id, request_payload_hash, period_start, reserved_usd, actual_usd, status, receipt_kind, COALESCE(async_task_id, ''), COALESCE(async_task_phase, ''), usage_log_id, expires_at
		FROM enterprise_member_budget_reservations WHERE request_id = $1 FOR UPDATE`, requestID).
		Scan(&existing.ID, &existing.RequestID, &existing.MemberID, &existing.GroupID, &existing.PayloadHash, &existing.PeriodStart, &existing.ReservedUSD, &existing.ActualUSD, &existing.Status, &existing.ReceiptKind, &existing.TaskID, &existing.TaskPhase, &existing.UsageLogID, &existing.ExpiresAt)
	if err == nil {
		existingPayloadHash := strings.TrimSpace(existing.PayloadHash)
		amountMatches := mathAbs(existing.ReservedUSD-amount) <= 1e-8
		legacyReceipt := existing.ReceiptKind == "" || existing.ReceiptKind == service.EnterpriseMemberReceiptKindLegacy
		legacyPositiveSyncReceipt := receiptKind == service.EnterpriseMemberReceiptKindSync && legacyReceipt && mathAbs(amount) <= 1e-8 && existing.ReservedUSD > 1e-8
		if existing.Status != "reserved" ||
			existing.MemberID != memberID ||
			(!legacyReceipt && existing.ReceiptKind != receiptKind) ||
			(!amountMatches && !legacyPositiveSyncReceipt) ||
			!sameOptionalInt64(existing.GroupID, groupID) ||
			existingPayloadHash != strings.TrimSpace(payloadHash) {
			return nil, service.ErrEnterpriseMemberBudgetConflict
		}
		return &existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	periodStart, enforced, err := reserveEnterpriseMemberSpendingLimits(ctx, tx, memberID, amount, time.Now())
	if err != nil {
		return nil, err
	}
	if !enforced && amount > 0 {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	reservation := &service.EnterpriseMemberBudgetReservation{RequestID: requestID, MemberID: memberID, GroupID: groupID, PayloadHash: payloadHash, PeriodStart: periodStart, ReservedUSD: amount, Status: "reserved", ReceiptKind: receiptKind, ExpiresAt: expiresAt}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO enterprise_member_budget_reservations (request_id, member_id, group_id, request_payload_hash, period_start, reserved_usd, receipt_kind, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`, requestID, memberID, groupID, strings.TrimSpace(payloadHash), periodStart, amount, receiptKind, expiresAt).Scan(&reservation.ID); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, service.ErrEnterpriseMemberBudgetConflict
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return reservation, nil
}

func validEnterpriseMemberReceiptKind(kind string) bool {
	switch kind {
	case service.EnterpriseMemberReceiptKindLegacy, service.EnterpriseMemberReceiptKindSync,
		service.EnterpriseMemberReceiptKindAsyncImage, service.EnterpriseMemberReceiptKindAsyncVideo,
		service.EnterpriseMemberReceiptKindBatchImage:
		return true
	default:
		return false
	}
}

func (r *enterpriseMemberBudgetRepository) AttachAsyncTask(ctx context.Context, requestID, taskID string, expiresAt time.Time) error {
	if r == nil || r.db == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_budget_reservations
		SET async_task_id = $2, async_task_phase = 'queued', expires_at = $3, updated_at = NOW()
		WHERE request_id = $1 AND receipt_kind = 'async_image' AND status = 'reserved'
		  AND (async_task_id IS NULL OR async_task_id = $2)`, strings.TrimSpace(requestID), strings.TrimSpace(taskID), expiresAt)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	return nil
}

func (r *enterpriseMemberBudgetRepository) MarkAsyncTaskExecuting(ctx context.Context, requestID, taskID string) error {
	if r == nil || r.db == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_budget_reservations
		SET async_task_phase = 'executing', expires_at = NOW() + INTERVAL '2 hours', updated_at = NOW()
		WHERE request_id = $1 AND async_task_id = $2 AND receipt_kind = 'async_image'
		  AND status = 'reserved' AND async_task_phase IN ('queued', 'executing')`,
		strings.TrimSpace(requestID), strings.TrimSpace(taskID))
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	return nil
}

func (r *enterpriseMemberBudgetRepository) ReleaseAsyncTask(ctx context.Context, requestID, taskID string) (_ *service.EnterpriseMemberBudgetReservation, err error) {
	if r == nil || r.db == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	requestID = strings.TrimSpace(requestID)
	taskID = strings.TrimSpace(taskID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	receipt := &service.EnterpriseMemberBudgetReservation{RequestID: requestID}
	err = tx.QueryRowContext(ctx, `
		SELECT id, member_id, group_id, period_start, reserved_usd, actual_usd, status,
		       receipt_kind, COALESCE(async_task_id, ''), COALESCE(async_task_phase, ''), expires_at
		FROM enterprise_member_budget_reservations
		WHERE request_id = $1
		FOR UPDATE`, requestID).Scan(
		&receipt.ID, &receipt.MemberID, &receipt.GroupID, &receipt.PeriodStart,
		&receipt.ReservedUSD, &receipt.ActualUSD, &receipt.Status, &receipt.ReceiptKind,
		&receipt.TaskID, &receipt.TaskPhase, &receipt.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	if receipt.ReceiptKind != service.EnterpriseMemberReceiptKindAsyncImage ||
		(receipt.TaskID != "" && receipt.TaskID != taskID) {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	if receipt.Status != "reserved" {
		return receipt, nil
	}
	if receipt.TaskPhase != "" && receipt.TaskPhase != service.EnterpriseMemberAsyncTaskPhaseQueued &&
		receipt.TaskPhase != service.EnterpriseMemberAsyncTaskPhaseExecuting {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_periods
		SET reserved_usd = GREATEST(0, reserved_usd - $1), version = version + 1, updated_at = NOW()
		WHERE member_id = $2 AND period_start = $3`, receipt.ReservedUSD, receipt.MemberID, receipt.PeriodStart); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_reservations
		SET status = 'released', outcome_reason = 'released_before_completion', updated_at = NOW()
		WHERE request_id = $1`, requestID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	receipt.Status = "released"
	return receipt, nil
}

func (r *enterpriseMemberBudgetRepository) MarkAsyncTaskAmbiguous(ctx context.Context, requestID, taskID, outcomeReason string) (_ *service.EnterpriseMemberBudgetReservation, err error) {
	if r == nil || r.db == nil || strings.TrimSpace(requestID) == "" || strings.TrimSpace(taskID) == "" || strings.TrimSpace(outcomeReason) == "" {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	requestID = strings.TrimSpace(requestID)
	taskID = strings.TrimSpace(taskID)
	outcomeReason = strings.TrimSpace(outcomeReason)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	receipt := &service.EnterpriseMemberBudgetReservation{RequestID: requestID}
	err = tx.QueryRowContext(ctx, `
		SELECT id, member_id, reserved_usd, actual_usd, status, receipt_kind,
		       COALESCE(async_task_id, ''), COALESCE(async_task_phase, ''), expires_at
		FROM enterprise_member_budget_reservations
		WHERE request_id = $1
		FOR UPDATE`, requestID).Scan(
		&receipt.ID, &receipt.MemberID, &receipt.ReservedUSD, &receipt.ActualUSD,
		&receipt.Status, &receipt.ReceiptKind, &receipt.TaskID, &receipt.TaskPhase, &receipt.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	if receipt.ReceiptKind != service.EnterpriseMemberReceiptKindAsyncImage || receipt.TaskID != taskID {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	if receipt.Status == "reserved" {
		if _, err := tx.ExecContext(ctx, `
			UPDATE enterprise_member_budget_reservations
			SET status = 'ambiguous', outcome_reason = $1,
			    reconcile_attempts = reconcile_attempts + 1, last_reconcile_at = NOW(),
			    expires_at = NOW() + INTERVAL '10 minutes', updated_at = NOW()
			WHERE request_id = $2`, outcomeReason, requestID); err != nil {
			return nil, err
		}
		receipt.Status = "ambiguous"
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (r *enterpriseMemberBudgetRepository) GetReservation(ctx context.Context, requestID string) (*service.EnterpriseMemberBudgetReservation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(requestID) == "" {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	receipt := &service.EnterpriseMemberBudgetReservation{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, request_id, member_id, group_id, request_payload_hash, period_start,
		       reserved_usd, actual_usd, status, receipt_kind, COALESCE(async_task_id, ''), COALESCE(async_task_phase, ''), usage_log_id, expires_at
		FROM enterprise_member_budget_reservations
		WHERE request_id = $1`, strings.TrimSpace(requestID)).Scan(
		&receipt.ID, &receipt.RequestID, &receipt.MemberID, &receipt.GroupID,
		&receipt.PayloadHash, &receipt.PeriodStart, &receipt.ReservedUSD,
		&receipt.ActualUSD, &receipt.Status, &receipt.ReceiptKind, &receipt.TaskID, &receipt.TaskPhase, &receipt.UsageLogID, &receipt.ExpiresAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (r *enterpriseMemberBudgetRepository) GetReservationByTaskID(ctx context.Context, taskID string) (*service.EnterpriseMemberBudgetReservation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(taskID) == "" {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	receipt := &service.EnterpriseMemberBudgetReservation{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, request_id, member_id, group_id, request_payload_hash, period_start,
		       reserved_usd, actual_usd, status, receipt_kind, COALESCE(async_task_id, ''),
		       COALESCE(async_task_phase, ''), usage_log_id, expires_at, created_at
		FROM enterprise_member_budget_reservations
		WHERE async_task_id = $1`, strings.TrimSpace(taskID)).Scan(
		&receipt.ID, &receipt.RequestID, &receipt.MemberID, &receipt.GroupID,
		&receipt.PayloadHash, &receipt.PeriodStart, &receipt.ReservedUSD,
		&receipt.ActualUSD, &receipt.Status, &receipt.ReceiptKind, &receipt.TaskID,
		&receipt.TaskPhase, &receipt.UsageLogID, &receipt.ExpiresAt, &receipt.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (r *enterpriseMemberBudgetRepository) Release(ctx context.Context, requestID string) (err error) {
	if r == nil || r.db == nil || requestID == "" {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var memberID int64
	var periodStart time.Time
	var amount float64
	var status string
	var receiptKind string
	var taskID string
	err = tx.QueryRowContext(ctx, `SELECT member_id, period_start, reserved_usd, status, receipt_kind, COALESCE(async_task_id, '') FROM enterprise_member_budget_reservations WHERE request_id = $1 FOR UPDATE`, requestID).Scan(&memberID, &periodStart, &amount, &status, &receiptKind, &taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if receiptKind == service.EnterpriseMemberReceiptKindAsyncImage && taskID != "" {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	if status != "reserved" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET reserved_usd = GREATEST(0, reserved_usd - $1), version = version + 1, updated_at = NOW() WHERE member_id = $2 AND period_start = $3`, amount, memberID, periodStart); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_reservations SET status = 'released', outcome_reason = 'released_before_completion', updated_at = NOW() WHERE request_id = $1`, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *enterpriseMemberBudgetRepository) MarkAmbiguous(ctx context.Context, requestID, outcomeReason string) error {
	if r == nil || r.db == nil {
		return errors.New("enterprise member budget repository db is nil")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_budget_reservations
		SET status = 'ambiguous', outcome_reason = $1,
		    reconcile_attempts = reconcile_attempts + 1,
		    last_reconcile_at = NOW(), expires_at = NOW() + INTERVAL '10 minutes',
		    updated_at = NOW()
		WHERE request_id = $2 AND status = 'reserved'
		  AND NOT (receipt_kind = 'async_image' AND async_task_id IS NOT NULL)`, outcomeReason, requestID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		service.RecordEnterpriseMemberBudgetAmbiguous()
		return nil
	}
	var status, receiptKind, taskID string
	if err := r.db.QueryRowContext(ctx, `SELECT status, receipt_kind, COALESCE(async_task_id, '') FROM enterprise_member_budget_reservations WHERE request_id = $1`, requestID).Scan(&status, &receiptKind, &taskID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrEnterpriseMemberBudgetReceiptNotFound
		}
		return err
	}
	if receiptKind == service.EnterpriseMemberReceiptKindAsyncImage && taskID != "" {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	if status == "ambiguous" {
		return nil
	}
	return service.ErrEnterpriseMemberBudgetConflict
}

func (r *enterpriseMemberBudgetRepository) GetPeriod(ctx context.Context, memberID int64, periodStart time.Time) (float64, float64, error) {
	var used, reserved float64
	err := r.db.QueryRowContext(ctx, `SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods WHERE member_id = $1 AND period_start = $2`, memberID, periodStart).Scan(&used, &reserved)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil
	}
	return used, reserved, err
}

func (r *enterpriseMemberBudgetRepository) GetSummary(ctx context.Context, memberID int64, periodStart, periodEnd time.Time) (*service.EnterpriseMemberBudgetSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	summary := &service.EnterpriseMemberBudgetSummary{
		MemberID: memberID, PeriodStart: periodStart, PeriodEnd: periodEnd, Timezone: enterpriseBudgetTimezone(),
	}
	var window5h, window1d, window7d sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT m.monthly_limit_usd, COALESCE(p.used_usd, 0), COALESCE(p.reserved_usd, 0),
		       m.rate_limit_5h, m.rate_limit_1d, m.rate_limit_7d,
		       COALESCE(r.usage_5h, 0), COALESCE(r.usage_1d, 0), COALESCE(r.usage_7d, 0),
		       r.window_5h_start, r.window_1d_start, r.window_7d_start
		FROM enterprise_members m
		LEFT JOIN enterprise_member_budget_periods p ON p.member_id = m.id AND p.period_start = $2
		LEFT JOIN enterprise_member_rate_limit_periods r ON r.member_id = m.id
		WHERE m.id = $1`, memberID, periodStart.Format("2006-01-02")).
		Scan(&summary.LimitUSD, &summary.UsedUSD, &summary.ReservedUSD,
			&summary.RateLimit5h, &summary.RateLimit1d, &summary.RateLimit7d,
			&summary.Usage5h, &summary.Usage1d, &summary.Usage7d,
			&window5h, &window1d, &window7d)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberNotFound
	}
	if err != nil {
		return nil, err
	}
	if window5h.Valid && !service.IsWindowExpired(&window5h.Time, service.RateLimitWindow5h) {
		reset := window5h.Time.Add(service.RateLimitWindow5h)
		summary.Reset5hAt = &reset
	} else {
		summary.Usage5h = 0
	}
	if window1d.Valid && !service.IsWindowExpired(&window1d.Time, service.RateLimitWindow1d) {
		reset := window1d.Time.Add(service.RateLimitWindow1d)
		summary.Reset1dAt = &reset
	} else {
		summary.Usage1d = 0
	}
	if window7d.Valid && !service.IsWindowExpired(&window7d.Time, service.RateLimitWindow7d) {
		reset := window7d.Time.Add(service.RateLimitWindow7d)
		summary.Reset7dAt = &reset
	} else {
		summary.Usage7d = 0
	}
	if summary.LimitUSD <= 0 {
		summary.RemainingUSD = -1
	} else {
		summary.RemainingUSD = summary.LimitUSD - summary.UsedUSD
		if summary.RemainingUSD < 0 {
			summary.RemainingUSD = 0
		}
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(usage.input_tokens), 0), COALESCE(SUM(usage.output_tokens), 0)
		FROM enterprise_member_budget_entries entry
		JOIN usage_logs usage
		  ON usage.id = entry.usage_log_id
		 AND usage.member_id = entry.member_id
		WHERE entry.member_id = $1
		  AND entry.period_start = $2
		  AND entry.kind = 'usage'`, memberID, periodStart.Format("2006-01-02")).
		Scan(&summary.RequestCount, &summary.InputTokens, &summary.OutputTokens); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(billed_usd), 0), COALESCE(SUM(total_tokens), 0),
		       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0),
		       COALESCE(SUM(cache_tokens), 0), COALESCE(SUM(cache_creation_tokens), 0),
		       COALESCE(SUM(cache_read_tokens), 0)
		FROM enterprise_member_import_usage_baselines
		WHERE member_id = $1 AND period_start = $2`, memberID, periodStart.Format("2006-01-02")).
		Scan(&summary.MigrationBilledUSD, &summary.MigrationTotalTokens, &summary.MigrationInputTokens,
			&summary.MigrationOutputTokens, &summary.MigrationCacheTokens, &summary.MigrationCacheWriteTokens,
			&summary.MigrationCacheReadTokens); err != nil {
		return nil, err
	}
	return summary, nil
}

func (r *enterpriseMemberBudgetRepository) ListEntries(ctx context.Context, memberID int64, periodStart time.Time, limit, offset int) ([]service.EnterpriseMemberBudgetEntry, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("enterprise member budget repository db is nil")
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM enterprise_member_budget_entries WHERE member_id = $1 AND period_start = $2`, memberID, periodStart.Format("2006-01-02")).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, kind, request_id, amount_usd, usage_log_id, actor_user_id, note, created_at
		FROM enterprise_member_budget_entries
		WHERE member_id = $1 AND period_start = $2
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`, memberID, periodStart.Format("2006-01-02"), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	entries := make([]service.EnterpriseMemberBudgetEntry, 0, limit)
	for rows.Next() {
		var entry service.EnterpriseMemberBudgetEntry
		if err := rows.Scan(&entry.ID, &entry.Kind, &entry.RequestID, &entry.AmountUSD, &entry.UsageLogID, &entry.ActorUserID, &entry.Note, &entry.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry)
	}
	return entries, total, rows.Err()
}

func (r *enterpriseMemberBudgetRepository) CreateAdjustment(ctx context.Context, memberID int64, periodStart time.Time, amount float64, actorUserID int64, idempotencyKey, note string) (err error) {
	if r == nil || r.db == nil {
		return errors.New("enterprise member budget repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone)
		VALUES ($1, $2, $3) ON CONFLICT (member_id, period_start) DO NOTHING`, memberID, periodStart.Format("2006-01-02"), enterpriseBudgetTimezone()); err != nil {
		return err
	}
	var existingAmount float64
	var existingActor *int64
	var existingNote string
	err = tx.QueryRowContext(ctx, `SELECT amount_usd, actor_user_id, note FROM enterprise_member_budget_entries WHERE idempotency_key = $1 FOR UPDATE`, idempotencyKey).
		Scan(&existingAmount, &existingActor, &existingNote)
	if err == nil {
		if mathAbs(existingAmount-amount) > 1e-8 || existingActor == nil || *existingActor != actorUserID || existingNote != note {
			return service.ErrEnterpriseMemberBudgetConflict
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_periods
		SET used_usd = used_usd + $1, version = version + 1, updated_at = NOW()
		WHERE member_id = $2 AND period_start = $3 AND used_usd + $1 >= 0`, amount, memberID, periodStart.Format("2006-01-02"))
	if err != nil {
		return err
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return service.ErrEnterpriseMemberInvalid
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
		(member_id, period_start, kind, amount_usd, idempotency_key, actor_user_id, note)
		VALUES ($1, $2, 'manual_adjustment', $3, $4, $5, $6)`, memberID, periodStart.Format("2006-01-02"), amount, idempotencyKey, actorUserID, note); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return service.ErrEnterpriseMemberBudgetConflict
		}
		return err
	}
	return tx.Commit()
}

func (r *enterpriseMemberBudgetRepository) SetUsage(ctx context.Context, ownerID, memberID int64, periodStart time.Time, monthlyUsed, usage5h, usage1d, usage7d float64, actorUserID int64, idempotencyKey, note string) (err error) {
	if r == nil || r.db == nil {
		return errors.New("enterprise member budget repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var existingMonthly, existing5h, existing1d, existing7d float64
	var existingNote string
	err = tx.QueryRowContext(ctx, `
		SELECT (after_data->>'monthly_used_usd')::numeric,
		       (after_data->>'usage_5h')::numeric,
		       (after_data->>'usage_1d')::numeric,
		       (after_data->>'usage_7d')::numeric,
		       metadata->>'note'
		FROM enterprise_member_audit_logs
		WHERE action = 'member.usage_adjusted' AND metadata->>'idempotency_key' = $1
		FOR UPDATE`, idempotencyKey).Scan(&existingMonthly, &existing5h, &existing1d, &existing7d, &existingNote)
	if err == nil {
		if mathAbs(existingMonthly-monthlyUsed) > 1e-8 || mathAbs(existing5h-usage5h) > 1e-8 || mathAbs(existing1d-usage1d) > 1e-8 || mathAbs(existing7d-usage7d) > 1e-8 || existingNote != note {
			return service.ErrEnterpriseMemberBudgetConflict
		}
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	var actualOwnerID int64
	if err := tx.QueryRowContext(ctx, `SELECT enterprise_user_id FROM enterprise_members WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, memberID).Scan(&actualOwnerID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return service.ErrEnterpriseMemberNotFound
		}
		return err
	}
	if actualOwnerID != ownerID || actorUserID != ownerID {
		return service.ErrEnterpriseMemberNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone)
		VALUES ($1, $2, $3) ON CONFLICT (member_id, period_start) DO NOTHING`, memberID, periodStart, enterpriseBudgetTimezone()); err != nil {
		return err
	}
	var beforeMonthly float64
	if err := tx.QueryRowContext(ctx, `SELECT used_usd FROM enterprise_member_budget_periods WHERE member_id = $1 AND period_start = $2 FOR UPDATE`, memberID, periodStart).Scan(&beforeMonthly); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_member_rate_limit_periods (member_id) VALUES ($1) ON CONFLICT (member_id) DO NOTHING`, memberID); err != nil {
		return err
	}
	var before5h, before1d, before7d float64
	if err := tx.QueryRowContext(ctx, `SELECT usage_5h, usage_1d, usage_7d FROM enterprise_member_rate_limit_periods WHERE member_id = $1 FOR UPDATE`, memberID).Scan(&before5h, &before1d, &before7d); err != nil {
		return err
	}

	delta := monthlyUsed - beforeMonthly
	if mathAbs(delta) > 1e-8 {
		if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET used_usd = $1, version = version + 1, updated_at = NOW() WHERE member_id = $2 AND period_start = $3`, monthlyUsed, memberID, periodStart); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, amount_usd, idempotency_key, actor_user_id, note)
			VALUES ($1, $2, 'manual_adjustment', $3, $4, $5, $6)`, memberID, periodStart, delta, idempotencyKey, actorUserID, note); err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				return service.ErrEnterpriseMemberBudgetConflict
			}
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_rate_limit_periods
		SET usage_5h = $1, usage_1d = $2, usage_7d = $3,
		    window_5h_start = CASE WHEN ABS(usage_5h - $1) > 0.00000001 THEN NOW() ELSE window_5h_start END,
		    window_1d_start = CASE WHEN ABS(usage_1d - $2) > 0.00000001 THEN NOW() ELSE window_1d_start END,
		    window_7d_start = CASE WHEN ABS(usage_7d - $3) > 0.00000001 THEN NOW() ELSE window_7d_start END,
		    updated_at = NOW()
		WHERE member_id = $4`, usage5h, usage1d, usage7d, memberID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_audit_logs
		(enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data, metadata)
		VALUES ($1, $2, $3, 'member.usage_adjusted', 'member', $2,
			jsonb_build_object(
				'monthly_used_usd', CAST($4 AS NUMERIC),
				'usage_5h', CAST($5 AS NUMERIC),
				'usage_1d', CAST($6 AS NUMERIC),
				'usage_7d', CAST($7 AS NUMERIC)),
			jsonb_build_object(
				'monthly_used_usd', CAST($8 AS NUMERIC),
				'usage_5h', CAST($9 AS NUMERIC),
				'usage_1d', CAST($10 AS NUMERIC),
				'usage_7d', CAST($11 AS NUMERIC)),
			jsonb_build_object(
				'note', CAST($12 AS TEXT),
				'idempotency_key', CAST($13 AS TEXT)))`,
		ownerID, memberID, actorUserID, beforeMonthly, before5h, before1d, before7d,
		monthlyUsed, usage5h, usage1d, usage7d, note, idempotencyKey); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *enterpriseMemberBudgetRepository) BatchAdjustUsage(ctx context.Context, ownerID int64, periodStart time.Time, targets []service.EnterpriseMemberBatchTarget, delta service.EnterpriseMemberUsageDelta, actorUserID int64, idempotencyKey, note string) ([]service.BatchEnterpriseMemberUsageUpdate, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	ordered := append([]service.EnterpriseMemberBatchTarget(nil), targets...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize retries for the same batch key before checking durable audit evidence.
	// Without this lock, concurrent first attempts could both observe no audit row and
	// apply the same signed delta twice because usage writes do not advance policy versions.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, idempotencyKey); err != nil {
		return nil, err
	}

	replayedByID := make(map[int64]service.BatchEnterpriseMemberUsageUpdate, len(ordered))
	for _, target := range ordered {
		memberKey := fmt.Sprintf("%s:%d", idempotencyKey, target.ID)
		var replay service.BatchEnterpriseMemberUsageUpdate
		replay.ID = target.ID
		var monthlyDelta, usage5hDelta, usage1dDelta, usage7dDelta float64
		var expectedVersion int64
		err := tx.QueryRowContext(ctx, `
			SELECT (after_data->>'monthly_used_usd')::numeric,
			       (after_data->>'usage_5h')::numeric,
			       (after_data->>'usage_1d')::numeric,
			       (after_data->>'usage_7d')::numeric,
			       (metadata->>'monthly_used_delta')::numeric,
			       (metadata->>'usage_5h_delta')::numeric,
			       (metadata->>'usage_1d_delta')::numeric,
			       (metadata->>'usage_7d_delta')::numeric,
			       (metadata->>'expected_version')::bigint
			FROM enterprise_member_audit_logs
			WHERE enterprise_user_id = $1 AND member_id = $2
			  AND action = 'member.usage_adjusted'
			  AND metadata->>'idempotency_key' = $3
			FOR UPDATE`, ownerID, target.ID, memberKey).
			Scan(&replay.MonthlyUsedUSD, &replay.Usage5h, &replay.Usage1d, &replay.Usage7d,
				&monthlyDelta, &usage5hDelta, &usage1dDelta, &usage7dDelta, &expectedVersion)
		if err == nil {
			if expectedVersion != target.ExpectedVersion ||
				mathAbs(monthlyDelta-delta.MonthlyUsedUSD) > 1e-8 ||
				mathAbs(usage5hDelta-delta.Usage5h) > 1e-8 ||
				mathAbs(usage1dDelta-delta.Usage1d) > 1e-8 ||
				mathAbs(usage7dDelta-delta.Usage7d) > 1e-8 {
				return nil, service.ErrEnterpriseMemberBudgetConflict
			}
			replayedByID[target.ID] = replay
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	if len(replayedByID) > 0 {
		if len(replayedByID) != len(ordered) {
			return nil, service.ErrEnterpriseMemberBudgetConflict
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return orderBatchUsageUpdates(targets, replayedByID), nil
	}

	updatedByID := make(map[int64]service.BatchEnterpriseMemberUsageUpdate, len(ordered))
	for _, target := range ordered {
		var currentVersion int64
		if err := tx.QueryRowContext(ctx, `
			SELECT version
			FROM enterprise_members
			WHERE id = $1 AND enterprise_user_id = $2
			  AND deleted_at IS NULL AND removed_at IS NULL
			FOR UPDATE`, target.ID, ownerID).Scan(&currentVersion); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, service.ErrEnterpriseMemberNotFound
			}
			return nil, err
		}
		if currentVersion != target.ExpectedVersion {
			return nil, service.ErrEnterpriseMemberVersion
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone)
			VALUES ($1, $2, $3)
			ON CONFLICT (member_id, period_start) DO NOTHING`, target.ID, periodStart.Format("2006-01-02"), enterpriseBudgetTimezone()); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_rate_limit_periods (member_id)
			VALUES ($1) ON CONFLICT (member_id) DO NOTHING`, target.ID); err != nil {
			return nil, err
		}

		var beforeMonthly, before5h, before1d, before7d float64
		if err := tx.QueryRowContext(ctx, `
			SELECT used_usd
			FROM enterprise_member_budget_periods
			WHERE member_id = $1 AND period_start = $2
			FOR UPDATE`, target.ID, periodStart.Format("2006-01-02")).Scan(&beforeMonthly); err != nil {
			return nil, err
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT CASE
			         WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN 0
			         ELSE usage_5h
			       END,
			       CASE
			         WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '1 day' <= NOW() THEN 0
			         ELSE usage_1d
			       END,
			       CASE
			         WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN 0
			         ELSE usage_7d
			       END
			FROM enterprise_member_rate_limit_periods
			WHERE member_id = $1
			FOR UPDATE`, target.ID).Scan(&before5h, &before1d, &before7d); err != nil {
			return nil, err
		}

		update := service.BatchEnterpriseMemberUsageUpdate{
			ID:             target.ID,
			MonthlyUsedUSD: beforeMonthly + delta.MonthlyUsedUSD,
			Usage5h:        before5h + delta.Usage5h,
			Usage1d:        before1d + delta.Usage1d,
			Usage7d:        before7d + delta.Usage7d,
		}
		if update.MonthlyUsedUSD < -1e-8 || update.Usage5h < -1e-8 || update.Usage1d < -1e-8 || update.Usage7d < -1e-8 {
			return nil, service.ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{
				"field": "usage_delta", "reason": "negative_result", "member_id": fmt.Sprint(target.ID),
			})
		}
		if update.MonthlyUsedUSD > service.EnterpriseMemberMaxMonetaryValue ||
			update.Usage5h > service.EnterpriseMemberMaxMonetaryValue ||
			update.Usage1d > service.EnterpriseMemberMaxMonetaryValue ||
			update.Usage7d > service.EnterpriseMemberMaxMonetaryValue {
			return nil, service.ErrEnterpriseMemberInvalid.WithMetadata(map[string]string{
				"field": "usage_delta", "reason": "out_of_range", "member_id": fmt.Sprint(target.ID),
			})
		}
		update.MonthlyUsedUSD = clampBatchUsageZero(update.MonthlyUsedUSD)
		update.Usage5h = clampBatchUsageZero(update.Usage5h)
		update.Usage1d = clampBatchUsageZero(update.Usage1d)
		update.Usage7d = clampBatchUsageZero(update.Usage7d)

		memberKey := fmt.Sprintf("%s:%d", idempotencyKey, target.ID)
		if mathAbs(delta.MonthlyUsedUSD) > 1e-8 {
			if _, err := tx.ExecContext(ctx, `
				UPDATE enterprise_member_budget_periods
				SET used_usd = $1, version = version + 1, updated_at = NOW()
				WHERE member_id = $2 AND period_start = $3`, update.MonthlyUsedUSD, target.ID, periodStart.Format("2006-01-02")); err != nil {
				return nil, err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO enterprise_member_budget_entries
				(member_id, period_start, kind, amount_usd, idempotency_key, actor_user_id, note)
				VALUES ($1, $2, 'manual_adjustment', $3, $4, $5, $6)`,
				target.ID, periodStart.Format("2006-01-02"), delta.MonthlyUsedUSD, memberKey, actorUserID, note); err != nil {
				if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
					return nil, service.ErrEnterpriseMemberBudgetConflict
				}
				return nil, err
			}
		}
		if mathAbs(delta.Usage5h) > 1e-8 || mathAbs(delta.Usage1d) > 1e-8 || mathAbs(delta.Usage7d) > 1e-8 {
			if _, err := tx.ExecContext(ctx, `
				UPDATE enterprise_member_rate_limit_periods
				SET usage_5h = $1, usage_1d = $2, usage_7d = $3,
				    window_5h_start = CASE WHEN $4 THEN NOW() ELSE window_5h_start END,
				    window_1d_start = CASE WHEN $5 THEN NOW() ELSE window_1d_start END,
				    window_7d_start = CASE WHEN $6 THEN NOW() ELSE window_7d_start END,
				    updated_at = NOW()
				WHERE member_id = $7`, update.Usage5h, update.Usage1d, update.Usage7d,
				mathAbs(delta.Usage5h) > 1e-8, mathAbs(delta.Usage1d) > 1e-8, mathAbs(delta.Usage7d) > 1e-8, target.ID); err != nil {
				return nil, err
			}
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_audit_logs
			(enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data, metadata)
			VALUES ($1, $2, $3, 'member.usage_adjusted', 'member', $2,
				jsonb_build_object(
					'monthly_used_usd', CAST($4 AS NUMERIC),
					'usage_5h', CAST($5 AS NUMERIC),
					'usage_1d', CAST($6 AS NUMERIC),
					'usage_7d', CAST($7 AS NUMERIC)),
				jsonb_build_object(
					'monthly_used_usd', CAST($8 AS NUMERIC),
					'usage_5h', CAST($9 AS NUMERIC),
					'usage_1d', CAST($10 AS NUMERIC),
					'usage_7d', CAST($11 AS NUMERIC)),
				jsonb_build_object(
					'note', CAST($12 AS TEXT),
					'idempotency_key', CAST($13 AS TEXT),
					'batch_idempotency_key', CAST($14 AS TEXT),
					'monthly_used_delta', CAST($15 AS NUMERIC),
					'usage_5h_delta', CAST($16 AS NUMERIC),
					'usage_1d_delta', CAST($17 AS NUMERIC),
					'usage_7d_delta', CAST($18 AS NUMERIC),
					'expected_version', CAST($19 AS BIGINT)))`,
			ownerID, target.ID, actorUserID,
			beforeMonthly, before5h, before1d, before7d,
			update.MonthlyUsedUSD, update.Usage5h, update.Usage1d, update.Usage7d,
			note, memberKey, idempotencyKey,
			delta.MonthlyUsedUSD, delta.Usage5h, delta.Usage1d, delta.Usage7d, target.ExpectedVersion); err != nil {
			return nil, err
		}
		updatedByID[target.ID] = update
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return orderBatchUsageUpdates(targets, updatedByID), nil
}

func orderBatchUsageUpdates(targets []service.EnterpriseMemberBatchTarget, updates map[int64]service.BatchEnterpriseMemberUsageUpdate) []service.BatchEnterpriseMemberUsageUpdate {
	ordered := make([]service.BatchEnterpriseMemberUsageUpdate, 0, len(targets))
	for _, target := range targets {
		ordered = append(ordered, updates[target.ID])
	}
	return ordered
}

func clampBatchUsageZero(value float64) float64 {
	if mathAbs(value) <= 1e-8 {
		return 0
	}
	return value
}

func (r *enterpriseMemberBudgetRepository) GetUsageAnalytics(ctx context.Context, memberID int64, start, end time.Time) (*service.EnterpriseMemberUsageAnalytics, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	analytics := &service.EnterpriseMemberUsageAnalytics{Start: start, End: end, Trend: []service.EnterpriseMemberUsageTrendPoint{}, Models: []service.EnterpriseMemberUsageBreakdown{}, Groups: []service.EnterpriseMemberUsageBreakdown{}}
	trendRows, err := r.db.QueryContext(ctx, `
		SELECT TO_CHAR(created_at AT TIME ZONE $4, 'YYYY-MM-DD'), COUNT(*),
		       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(actual_cost), 0)
		FROM usage_logs WHERE member_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 1`, memberID, start, end, enterpriseBudgetTimezone())
	if err != nil {
		return nil, err
	}
	for trendRows.Next() {
		var item service.EnterpriseMemberUsageTrendPoint
		if err := trendRows.Scan(&item.Date, &item.RequestCount, &item.InputTokens, &item.OutputTokens, &item.ActualCost); err != nil {
			_ = trendRows.Close()
			return nil, err
		}
		analytics.Trend = append(analytics.Trend, item)
	}
	if err := trendRows.Close(); err != nil {
		return nil, err
	}

	modelRows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(NULLIF(requested_model, ''), model) AS model_name, COUNT(*),
		       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(actual_cost), 0)
		FROM usage_logs WHERE member_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY 1 ORDER BY 5 DESC, 1 LIMIT 100`, memberID, start, end)
	if err != nil {
		return nil, err
	}
	for modelRows.Next() {
		var item service.EnterpriseMemberUsageBreakdown
		if err := modelRows.Scan(&item.Key, &item.RequestCount, &item.InputTokens, &item.OutputTokens, &item.ActualCost); err != nil {
			_ = modelRows.Close()
			return nil, err
		}
		item.Name = item.Key
		analytics.Models = append(analytics.Models, item)
	}
	if err := modelRows.Close(); err != nil {
		return nil, err
	}

	groupRows, err := r.db.QueryContext(ctx, `
		SELECT COALESCE(ul.group_id::text, 'unassigned'), COALESCE(g.name, 'Unassigned'), COUNT(*),
		       COALESCE(SUM(ul.input_tokens), 0), COALESCE(SUM(ul.output_tokens), 0), COALESCE(SUM(ul.actual_cost), 0)
		FROM usage_logs ul LEFT JOIN groups g ON g.id = ul.group_id
		WHERE ul.member_id = $1 AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY 1, 2 ORDER BY 6 DESC, 2 LIMIT 100`, memberID, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = groupRows.Close() }()
	for groupRows.Next() {
		var item service.EnterpriseMemberUsageBreakdown
		if err := groupRows.Scan(&item.Key, &item.Name, &item.RequestCount, &item.InputTokens, &item.OutputTokens, &item.ActualCost); err != nil {
			return nil, err
		}
		analytics.Groups = append(analytics.Groups, item)
	}
	if err := groupRows.Err(); err != nil {
		return nil, err
	}
	return analytics, nil
}

func (r *enterpriseMemberBudgetRepository) GetOwnerUsageSummary(ctx context.Context, ownerID int64, periodStart, periodEnd time.Time) (*service.EnterpriseMemberOwnerUsageSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	summary := &service.EnterpriseMemberOwnerUsageSummary{PeriodStart: periodStart, PeriodEnd: periodEnd, Timezone: enterpriseBudgetTimezone(), Members: []service.EnterpriseMemberOwnerUsageItem{}}
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.id, m.member_code, m.name, m.status, m.monthly_limit_usd, m.removed_at,
		       COALESCE(p.used_usd, 0), COALESCE(p.reserved_usd, 0),
		       COALESCE(u.request_count, 0), COALESCE(u.input_tokens, 0), COALESCE(u.output_tokens, 0),
		       COALESCE(b.billed_usd, 0), COALESCE(b.total_tokens, 0), COALESCE(b.input_tokens, 0),
		       COALESCE(b.output_tokens, 0), COALESCE(b.cache_tokens, 0), COALESCE(b.cache_creation_tokens, 0),
		       COALESCE(b.cache_read_tokens, 0)
		FROM enterprise_members m
		LEFT JOIN enterprise_member_budget_periods p ON p.member_id = m.id AND p.period_start = $2
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS request_count,
			       COALESCE(SUM(usage.input_tokens), 0) AS input_tokens,
			       COALESCE(SUM(usage.output_tokens), 0) AS output_tokens
			FROM enterprise_member_budget_entries entry
			JOIN usage_logs usage
			  ON usage.id = entry.usage_log_id
			 AND usage.member_id = entry.member_id
			WHERE entry.member_id = m.id
			  AND entry.period_start = $2
			  AND entry.kind = 'usage'
		) u ON TRUE
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(billed_usd), 0) AS billed_usd, COALESCE(SUM(total_tokens), 0) AS total_tokens,
			       COALESCE(SUM(input_tokens), 0) AS input_tokens, COALESCE(SUM(output_tokens), 0) AS output_tokens,
			       COALESCE(SUM(cache_tokens), 0) AS cache_tokens, COALESCE(SUM(cache_creation_tokens), 0) AS cache_creation_tokens,
			       COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens
			FROM enterprise_member_import_usage_baselines baseline
			WHERE baseline.member_id = m.id AND baseline.period_start = $2
		) b ON TRUE
		WHERE m.enterprise_user_id = $1
		  AND m.removed_at IS NULL
		ORDER BY m.id`, ownerID, periodStart.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.EnterpriseMemberOwnerUsageItem
		var removedAt sql.NullTime
		if err := rows.Scan(&item.MemberID, &item.MemberCode, &item.MemberName, &item.Status, &item.LimitUSD,
			&removedAt,
			&item.UsedUSD, &item.ReservedUSD, &item.RequestCount, &item.InputTokens, &item.OutputTokens,
			&item.MigrationBilledUSD, &item.MigrationTotalTokens, &item.MigrationInputTokens,
			&item.MigrationOutputTokens, &item.MigrationCacheTokens, &item.MigrationCacheWriteTokens,
			&item.MigrationCacheReadTokens); err != nil {
			return nil, err
		}
		if removedAt.Valid {
			continue
		}
		if item.LimitUSD <= 0 {
			item.RemainingUSD = -1
		} else {
			item.RemainingUSD = item.LimitUSD - item.UsedUSD
			if item.RemainingUSD < 0 {
				item.RemainingUSD = 0
			}
		}
		summary.UsedUSD += item.UsedUSD
		summary.ReservedUSD += item.ReservedUSD
		summary.RequestCount += item.RequestCount
		summary.InputTokens += item.InputTokens
		summary.OutputTokens += item.OutputTokens
		summary.MigrationBilledUSD += item.MigrationBilledUSD
		summary.MigrationTotalTokens = summary.MigrationTotalTokens.Add(item.MigrationTotalTokens)
		summary.MigrationInputTokens = summary.MigrationInputTokens.Add(item.MigrationInputTokens)
		summary.MigrationOutputTokens = summary.MigrationOutputTokens.Add(item.MigrationOutputTokens)
		summary.MigrationCacheTokens = summary.MigrationCacheTokens.Add(item.MigrationCacheTokens)
		summary.MigrationCacheWriteTokens = summary.MigrationCacheWriteTokens.Add(item.MigrationCacheWriteTokens)
		summary.MigrationCacheReadTokens = summary.MigrationCacheReadTokens.Add(item.MigrationCacheReadTokens)
		summary.Members = append(summary.Members, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return summary, nil
}

func (r *enterpriseMemberBudgetRepository) GetOwnerUsageTrend(ctx context.Context, ownerID int64, start, end time.Time) ([]service.EnterpriseMemberUsageTrendPoint, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT TO_CHAR(ul.created_at AT TIME ZONE $4, 'YYYY-MM-DD'), COUNT(*),
		       COALESCE(SUM(ul.input_tokens), 0), COALESCE(SUM(ul.output_tokens), 0), COALESCE(SUM(ul.actual_cost), 0)
		FROM usage_logs ul
		WHERE ul.user_id = $1
		  AND `+ownerVisibleEnterpriseMemberFactCondition("ul")+`
		  AND ul.created_at >= $2 AND ul.created_at < $3
		GROUP BY 1 ORDER BY 1`, ownerID, start, end, enterpriseBudgetTimezone())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	trend := make([]service.EnterpriseMemberUsageTrendPoint, 0)
	for rows.Next() {
		var item service.EnterpriseMemberUsageTrendPoint
		if err := rows.Scan(&item.Date, &item.RequestCount, &item.InputTokens, &item.OutputTokens, &item.ActualCost); err != nil {
			return nil, err
		}
		trend = append(trend, item)
	}
	return trend, rows.Err()
}

func enterpriseBudgetTimezone() string { return "Asia/Shanghai" }

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (r *enterpriseMemberBudgetRepository) ListAmbiguousReceipts(ctx context.Context, limit, offset int) ([]service.EnterpriseMemberAmbiguousReceipt, int64, error) {
	if r == nil || r.db == nil {
		return nil, 0, errors.New("enterprise member budget repository db is nil")
	}
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM enterprise_member_budget_reservations WHERE status = 'ambiguous'`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT reservation.id, reservation.request_id, member.enterprise_user_id,
		       reservation.member_id, member.member_code, member.display_name,
		       reservation.group_id, reservation.period_start, reservation.reserved_usd,
		       reservation.receipt_kind, COALESCE(reservation.async_task_id, ''), COALESCE(reservation.async_task_phase, ''),
		       reservation.outcome_reason, reservation.reconcile_attempts,
		       reservation.last_reconcile_at, reservation.expires_at,
		       reservation.created_at, reservation.updated_at
		FROM enterprise_member_budget_reservations reservation
		JOIN enterprise_members member ON member.id = reservation.member_id
		WHERE reservation.status = 'ambiguous'
		ORDER BY reservation.updated_at DESC, reservation.id DESC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.EnterpriseMemberAmbiguousReceipt, 0, limit)
	for rows.Next() {
		var item service.EnterpriseMemberAmbiguousReceipt
		if err := rows.Scan(
			&item.ID, &item.RequestID, &item.EnterpriseUserID,
			&item.MemberID, &item.MemberCode, &item.MemberName,
			&item.GroupID, &item.PeriodStart, &item.ReservedUSD,
			&item.ReceiptKind, &item.TaskID, &item.TaskPhase,
			&item.OutcomeReason, &item.ReconcileAttempts,
			&item.LastReconcileAt, &item.ExpiresAt,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *enterpriseMemberBudgetRepository) ResolveAmbiguousReceipt(ctx context.Context, receiptID int64, input service.EnterpriseMemberAmbiguousReceiptResolution, actorUserID int64) (receipt *service.EnterpriseMemberAmbiguousReceipt, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	receipt = &service.EnterpriseMemberAmbiguousReceipt{}
	var status string
	err = tx.QueryRowContext(ctx, `
		SELECT reservation.id, reservation.request_id, member.enterprise_user_id,
		       reservation.member_id, member.member_code, member.display_name,
		       reservation.group_id, reservation.period_start, reservation.reserved_usd,
		       reservation.receipt_kind, COALESCE(reservation.async_task_id, ''), COALESCE(reservation.async_task_phase, ''),
		       reservation.outcome_reason, reservation.reconcile_attempts,
		       reservation.last_reconcile_at, reservation.expires_at,
		       reservation.created_at, reservation.updated_at, reservation.status
		FROM enterprise_member_budget_reservations reservation
		JOIN enterprise_members member ON member.id = reservation.member_id
		WHERE reservation.id = $1
		FOR UPDATE`, receiptID).Scan(
		&receipt.ID, &receipt.RequestID, &receipt.EnterpriseUserID,
		&receipt.MemberID, &receipt.MemberCode, &receipt.MemberName,
		&receipt.GroupID, &receipt.PeriodStart, &receipt.ReservedUSD,
		&receipt.ReceiptKind, &receipt.TaskID, &receipt.TaskPhase,
		&receipt.OutcomeReason, &receipt.ReconcileAttempts,
		&receipt.LastReconcileAt, &receipt.ExpiresAt,
		&receipt.CreatedAt, &receipt.UpdatedAt, &status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberBudgetReceiptNotFound
	}
	if err != nil {
		return nil, err
	}
	if status != "ambiguous" || receipt.ReconcileAttempts != input.ExpectedReconcileAttempts {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}

	if input.Decision != service.EnterpriseMemberReceiptDecisionRelease {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	apiKeyID, usageRequestID, ok := splitEnterpriseBudgetRequestID(receipt.RequestID)
	if !ok {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	// The administrator's observation can be stale. Re-check local settlement
	// evidence while the receipt row is locked so a concurrent usage commit can
	// never race a manual release and refund a billable request.
	var usageExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM usage_logs WHERE api_key_id = $1 AND request_id = $2 AND member_id = $3)`, apiKeyID, usageRequestID, receipt.MemberID).Scan(&usageExists); err != nil {
		return nil, err
	}
	if usageExists {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	var billingExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM usage_billing_dedup WHERE api_key_id = $1 AND request_id = $2
			UNION ALL
			SELECT 1 FROM usage_billing_dedup_archive WHERE api_key_id = $1 AND request_id = $2
		)`, apiKeyID, usageRequestID).Scan(&billingExists); err != nil {
		return nil, err
	}
	if billingExists {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	var settlementPending bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM enterprise_member_usage_settlement_outbox
			WHERE api_key_id = $1 AND request_id = $2 AND member_id = $3
		)`, apiKeyID, usageRequestID, receipt.MemberID).Scan(&settlementPending); err != nil {
		return nil, err
	}
	if settlementPending {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	var batchImageMayStillSettle bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM batch_image_jobs
			WHERE member_budget_request_id = $1
			  AND (
				status NOT IN ('failed', 'cancelled', 'output_deleted')
				OR success_count > 0
				OR actual_cost IS NOT NULL
			  )
		)`, receipt.RequestID).Scan(&batchImageMayStillSettle); err != nil {
		return nil, err
	}
	if batchImageMayStillSettle {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_periods
		SET reserved_usd = GREATEST(0, reserved_usd - $1),
		    version = version + 1, updated_at = NOW()
		WHERE member_id = $2 AND period_start = $3`, receipt.ReservedUSD, receipt.MemberID, receipt.PeriodStart); err != nil {
		return nil, err
	}

	newStatus := "released"
	outcomeReason := "manual_release"
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_reservations
		SET actual_usd = $1, status = $2, outcome_reason = $3,
		    reconcile_attempts = reconcile_attempts + 1,
		    last_reconcile_at = NOW(), updated_at = NOW()
		WHERE id = $4 AND status = 'ambiguous' AND reconcile_attempts = $5`,
		0, newStatus, outcomeReason, receipt.ID, input.ExpectedReconcileAttempts); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_audit_logs
		(enterprise_user_id, member_id, actor_user_id, action, entity_type, entity_id, before_data, after_data, metadata)
		VALUES ($1, $2, $3, 'member.budget_receipt_reconciled', 'budget_receipt', $4,
			jsonb_build_object('status', 'ambiguous', 'reserved_usd', CAST($5 AS NUMERIC), 'outcome_reason', CAST($6 AS TEXT), 'reconcile_attempts', $7),
			jsonb_build_object('status', CAST($8 AS TEXT), 'actual_usd', CAST($9 AS NUMERIC), 'outcome_reason', CAST($10 AS TEXT), 'reconcile_attempts', $7 + 1),
			jsonb_build_object('decision', CAST($11 AS TEXT), 'reason', CAST($12 AS TEXT)))`,
		receipt.EnterpriseUserID, receipt.MemberID, actorUserID, receipt.ID,
		receipt.ReservedUSD, receipt.OutcomeReason, receipt.ReconcileAttempts,
		newStatus, 0, outcomeReason, input.Decision, input.Reason); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	now := time.Now()
	receipt.OutcomeReason = outcomeReason
	receipt.ReconcileAttempts++
	receipt.LastReconcileAt = &now
	receipt.UpdatedAt = now
	return receipt, nil
}

func (r *enterpriseMemberBudgetRepository) RecoverExpired(ctx context.Context, limit int) (recovered int, err error) {
	if r == nil || r.db == nil {
		return 0, errors.New("enterprise member budget repository db is nil")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `
		SELECT request_id, member_id, period_start, reserved_usd, status, receipt_kind,
		       COALESCE(async_task_id, ''), COALESCE(async_task_phase, '')
		FROM enterprise_member_budget_reservations
		WHERE status IN ('reserved', 'ambiguous') AND expires_at <= NOW()
		ORDER BY expires_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type expiredReservation struct {
		requestID   string
		memberID    int64
		periodStart time.Time
		reservedUSD float64
		status      string
		receiptKind string
		taskID      string
		taskPhase   string
	}
	items := make([]expiredReservation, 0, limit)
	for rows.Next() {
		var item expiredReservation
		if err := rows.Scan(&item.requestID, &item.memberID, &item.periodStart, &item.reservedUSD, &item.status, &item.receiptKind, &item.taskID, &item.taskPhase); err != nil {
			_ = rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	for _, item := range items {
		apiKeyID, usageRequestID, ok := splitEnterpriseBudgetRequestID(item.requestID)
		if !ok {
			return recovered, service.ErrEnterpriseMemberBudgetConflict
		}
		var usageLogID int64
		var actualUSD float64
		usageErr := tx.QueryRowContext(ctx, `SELECT id, actual_cost FROM usage_logs WHERE api_key_id = $1 AND request_id = $2 AND member_id = $3`, apiKeyID, usageRequestID, item.memberID).Scan(&usageLogID, &actualUSD)
		if usageErr == nil {
			if item.reservedUSD > 1e-8 && actualUSD > item.reservedUSD+1e-8 {
				service.RecordEnterpriseMemberBudgetSettlementOverrun()
				logger.LegacyPrintf(
					"repository.enterprise_member_budget",
					"recovered enterprise member usage exceeded reservation: member=%d reserved=%.8f actual=%.8f",
					item.memberID,
					item.reservedUSD,
					actualUSD,
				)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET used_usd = used_usd + $1, reserved_usd = GREATEST(0, reserved_usd - $2), version = version + 1, updated_at = NOW() WHERE member_id = $3 AND period_start = $4`, actualUSD, item.reservedUSD, item.memberID, item.periodStart); err != nil {
				return recovered, err
			}
			if item.reservedUSD <= 1e-8 {
				if err := ensureEnterpriseMemberRateLimitPeriod(ctx, tx, item.memberID); err != nil {
					return recovered, err
				}
			}
			if err := incrementEnterpriseMemberRateLimitUsage(ctx, tx, item.memberID, actualUSD); err != nil {
				return recovered, err
			}
			outcomeReason := "settled_after_recovery"
			if item.reservedUSD > 1e-8 && actualUSD > item.reservedUSD+1e-8 {
				outcomeReason = "settled_after_overrun"
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE enterprise_member_budget_reservations
				SET actual_usd = $1, status = 'settled', usage_log_id = $2,
				    outcome_reason = $3, reconcile_attempts = reconcile_attempts + 1,
				    last_reconcile_at = NOW(), updated_at = NOW()
				WHERE request_id = $4`, actualUSD, usageLogID, outcomeReason, item.requestID); err != nil {
				return recovered, err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_member_budget_entries (member_id, period_start, kind, request_id, amount_usd, usage_log_id, idempotency_key, note) VALUES ($1, $2, 'usage', $3, $4, $5, $6, 'recovered expired reservation') ON CONFLICT (request_id) DO UPDATE SET usage_log_id = EXCLUDED.usage_log_id`, item.memberID, item.periodStart, item.requestID, actualUSD, usageLogID, "usage:"+item.requestID); err != nil {
				return recovered, err
			}
			recovered++
			continue
		}
		if !errors.Is(usageErr, sql.ErrNoRows) {
			return recovered, usageErr
		}
		var billed bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM usage_billing_dedup WHERE api_key_id = $1 AND request_id = $2
				UNION ALL
				SELECT 1 FROM usage_billing_dedup_archive WHERE api_key_id = $1 AND request_id = $2
			)`, apiKeyID, usageRequestID).Scan(&billed); err != nil {
			return recovered, err
		}
		if billed {
			if _, err := tx.ExecContext(ctx, `
				UPDATE enterprise_member_budget_reservations
				SET status = 'ambiguous', outcome_reason = 'billing_without_usage',
				    reconcile_attempts = reconcile_attempts + 1, last_reconcile_at = NOW(),
				    expires_at = NOW() + INTERVAL '10 minutes', updated_at = NOW()
				WHERE request_id = $1`, item.requestID); err != nil {
				return recovered, err
			}
			if item.status == "reserved" {
				service.RecordEnterpriseMemberBudgetAmbiguous()
				recovered++
			}
			continue
		}
		if item.receiptKind == service.EnterpriseMemberReceiptKindAsyncImage &&
			(strings.TrimSpace(item.taskID) == "" || item.taskPhase == service.EnterpriseMemberAsyncTaskPhaseQueued) {
			// Async image execution is gated on the PostgreSQL executing fence.
			// A missing link or a row that is still queued proves upstream
			// dispatch never began, even if the Redis task key was lost.
			outcomeReason := "async_task_not_dispatched"
			if strings.TrimSpace(item.taskID) == "" {
				outcomeReason = "async_task_not_created"
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE enterprise_member_budget_periods
				SET reserved_usd = GREATEST(0, reserved_usd - $1), version = version + 1, updated_at = NOW()
				WHERE member_id = $2 AND period_start = $3`, item.reservedUSD, item.memberID, item.periodStart); err != nil {
				return recovered, err
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE enterprise_member_budget_reservations
				SET status = 'released', outcome_reason = $2,
				    reconcile_attempts = reconcile_attempts + 1, last_reconcile_at = NOW(), updated_at = NOW()
				WHERE request_id = $1`, item.requestID, outcomeReason); err != nil {
				return recovered, err
			}
			recovered++
			continue
		}
		// Absence of local billing evidence does not prove that the upstream
		// request failed. Preserve the reservation and mark the receipt for
		// reconciliation so a successful-but-interrupted request cannot silently
		// regain budget and disappear from member usage.
		if _, err := tx.ExecContext(ctx, `
			UPDATE enterprise_member_budget_reservations
			SET status = 'ambiguous', outcome_reason = 'outcome_unproven',
			    reconcile_attempts = reconcile_attempts + 1, last_reconcile_at = NOW(),
			    expires_at = NOW() + INTERVAL '10 minutes', updated_at = NOW()
			WHERE request_id = $1`, item.requestID); err != nil {
			return recovered, err
		}
		if item.status == "reserved" {
			service.RecordEnterpriseMemberBudgetAmbiguous()
			recovered++
		}
	}
	if err := tx.Commit(); err != nil {
		return recovered, err
	}
	return recovered, nil
}

// ReconcilePeriods repairs the rebuildable monthly projection from immutable
// ledger entries and live reservations. A usage log that survived a crash
// without a corresponding ledger entry is recovered as a usage entry linked
// to that immutable request fact, with a stable reconciliation idempotency key
// and note; it is never silently folded into the projection.
func (r *enterpriseMemberBudgetRepository) ReconcilePeriods(ctx context.Context, limit int) (result service.EnterpriseMemberBudgetReconciliationResult, err error) {
	if r == nil || r.db == nil {
		return result, errors.New("enterprise member budget repository db is nil")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, member_id, period_start, used_usd, reserved_usd
		FROM enterprise_member_budget_periods
		ORDER BY updated_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT $1`, limit)
	if err != nil {
		return result, err
	}
	type periodProjection struct {
		id          int64
		memberID    int64
		periodStart time.Time
		usedUSD     float64
		reservedUSD float64
	}
	periods := make([]periodProjection, 0, limit)
	for rows.Next() {
		var period periodProjection
		if err := rows.Scan(&period.id, &period.memberID, &period.periodStart, &period.usedUSD, &period.reservedUSD); err != nil {
			_ = rows.Close()
			return result, err
		}
		periods = append(periods, period)
	}
	if err := rows.Close(); err != nil {
		return result, err
	}

	for _, period := range periods {
		result.PeriodsChecked++
		linkedEntries, err := tx.ExecContext(ctx, `
			UPDATE enterprise_member_budget_entries entry
			SET usage_log_id = ul.id
			FROM usage_logs ul
			WHERE entry.member_id = $1
			  AND entry.period_start = $2
			  AND entry.kind = 'usage'
			  AND entry.usage_log_id IS NULL
			  AND ul.member_id = entry.member_id
			  AND entry.request_id = ul.api_key_id::text || ':' || ul.request_id
			  AND NOT EXISTS (
				SELECT 1 FROM enterprise_member_budget_entries claimed
				WHERE claimed.usage_log_id = ul.id
			  )`, period.memberID, period.periodStart.Format("2006-01-02"))
		if err != nil {
			return result, err
		}
		linkedReservations, err := tx.ExecContext(ctx, `
			UPDATE enterprise_member_budget_reservations reservation
			SET usage_log_id = ul.id, updated_at = NOW()
			FROM usage_logs ul
			WHERE reservation.member_id = $1
			  AND reservation.period_start = $2
			  AND reservation.status = 'settled'
			  AND reservation.usage_log_id IS NULL
			  AND ul.member_id = reservation.member_id
			  AND reservation.request_id = ul.api_key_id::text || ':' || ul.request_id
			  AND NOT EXISTS (
				SELECT 1 FROM enterprise_member_budget_reservations claimed
				WHERE claimed.usage_log_id = ul.id
			  )`, period.memberID, period.periodStart.Format("2006-01-02"))
		if err != nil {
			return result, err
		}
		for _, update := range []sql.Result{linkedEntries, linkedReservations} {
			if count, countErr := update.RowsAffected(); countErr == nil {
				result.EvidenceLinksRepaired += int(count)
			}
		}
		inserted, err := tx.ExecContext(ctx, `
			INSERT INTO enterprise_member_budget_entries
				(member_id, period_start, kind, request_id, amount_usd, usage_log_id, idempotency_key, note)
			SELECT ul.member_id, $2, 'usage', ul.api_key_id::text || ':' || ul.request_id,
			       GREATEST(ul.actual_cost, 0), ul.id,
			       'reconciliation:usage:' || ul.id::text,
			       'recovered usage evidence missing from member budget ledger'
			FROM usage_logs ul
			WHERE ul.member_id = $1
			  AND (ul.created_at AT TIME ZONE $3)::date >= $2
			  AND (ul.created_at AT TIME ZONE $3)::date < ($2::date + INTERVAL '1 month')::date
			  AND NOT EXISTS (
				SELECT 1 FROM enterprise_member_budget_entries entry
				WHERE entry.usage_log_id = ul.id
				   OR entry.request_id = ul.api_key_id::text || ':' || ul.request_id
				   OR entry.idempotency_key = 'reconciliation:usage:' || ul.id::text
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM enterprise_member_budget_reservations reservation
				WHERE reservation.request_id = ul.api_key_id::text || ':' || ul.request_id
				  AND reservation.status IN ('reserved', 'ambiguous')
			  )
			ON CONFLICT (idempotency_key) DO NOTHING`, period.memberID, period.periodStart.Format("2006-01-02"), enterpriseBudgetTimezone())
		if err != nil {
			return result, err
		}
		if count, countErr := inserted.RowsAffected(); countErr == nil {
			result.MissingEntriesCreated += int(count)
		}

		var ledgerUsed, liveReserved float64
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(amount_usd), 0)
			FROM enterprise_member_budget_entries
			WHERE member_id = $1 AND period_start = $2`, period.memberID, period.periodStart.Format("2006-01-02")).Scan(&ledgerUsed); err != nil {
			return result, err
		}
		if ledgerUsed < -1e-8 {
			return result, service.ErrEnterpriseMemberBudgetConflict
		}
		if ledgerUsed < 0 {
			ledgerUsed = 0
		}
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(reserved_usd), 0)
			FROM enterprise_member_budget_reservations
			WHERE member_id = $1 AND period_start = $2 AND status IN ('reserved', 'ambiguous')`, period.memberID, period.periodStart.Format("2006-01-02")).Scan(&liveReserved); err != nil {
			return result, err
		}
		if mathAbs(period.usedUSD-ledgerUsed) > 1e-8 || mathAbs(period.reservedUSD-liveReserved) > 1e-8 {
			result.ProjectionsRebuilt++
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE enterprise_member_budget_periods
			SET used_usd = $1, reserved_usd = $2, version = version + 1, updated_at = NOW()
			WHERE id = $3`, ledgerUsed, liveReserved, period.id); err != nil {
			return result, err
		}
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}

func splitEnterpriseBudgetRequestID(value string) (int64, string, bool) {
	keyPart, requestPart, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(requestPart) == "" {
		return 0, "", false
	}
	keyID, err := strconv.ParseInt(keyPart, 10, 64)
	if err != nil || keyID <= 0 {
		return 0, "", false
	}
	return keyID, requestPart, true
}
