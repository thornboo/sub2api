package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type usageBillingRepository struct {
	db *sql.DB
}

const enterpriseMemberSettlementPayloadVersion = 1

type enterpriseMemberSettlementPayload struct {
	Version int                          `json:"version"`
	Command *service.UsageBillingCommand `json:"command"`
}

func NewUsageBillingRepository(_ *dbent.Client, sqlDB *sql.DB) service.UsageBillingRepository {
	return &usageBillingRepository{db: sqlDB}
}

func NewEnterpriseMemberUsageSettlementRepository(_ *dbent.Client, sqlDB *sql.DB) service.EnterpriseMemberUsageSettlementRepository {
	return &usageBillingRepository{db: sqlDB}
}

func (r *usageBillingRepository) Apply(ctx context.Context, cmd *service.UsageBillingCommand) (_ *service.UsageBillingApplyResult, err error) {
	if cmd == nil {
		return &service.UsageBillingApplyResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}

	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}
	if cmd.MemberID != nil {
		if err := validateEnterpriseMemberUsageBillingCommand(cmd); err != nil {
			return nil, err
		}
		if err := r.stageEnterpriseMemberSettlement(ctx, cmd); err != nil {
			return nil, err
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingKey(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if !applied {
		if err := r.deleteEnterpriseMemberSettlement(ctx, tx, cmd); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		tx = nil
		return &service.UsageBillingApplyResult{Applied: false}, nil
	}

	result := &service.UsageBillingApplyResult{Applied: true}
	if err := r.applyUsageBillingEffects(ctx, tx, cmd, result); err != nil {
		return nil, err
	}
	if cmd.MemberID != nil && cmd.UsageLog != nil {
		if _, err := createUsageLogSingle(ctx, tx, cmd.UsageLog); err != nil {
			return nil, err
		}
		result.UsageLogPersisted = true
	}
	if err := r.deleteEnterpriseMemberSettlement(ctx, tx, cmd); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func validateEnterpriseMemberUsageBillingCommand(cmd *service.UsageBillingCommand) error {
	if cmd == nil || cmd.MemberID == nil || cmd.UsageLog == nil || strings.TrimSpace(cmd.MemberBudgetRequestID) == "" {
		return service.ErrEnterpriseMemberUsagePersistenceUnavailable
	}
	usage := cmd.UsageLog
	if usage.MemberID == nil ||
		*usage.MemberID != *cmd.MemberID ||
		usage.UserID != cmd.UserID ||
		usage.APIKeyID != cmd.APIKeyID ||
		strings.TrimSpace(usage.RequestID) != cmd.RequestID ||
		strings.TrimSpace(cmd.MemberBudgetRequestID) != service.EnterpriseMemberBudgetRequestID(cmd.APIKeyID, cmd.RequestID) {
		return service.ErrUsageBillingRequestConflict
	}
	return nil
}

func (r *usageBillingRepository) stageEnterpriseMemberSettlement(ctx context.Context, cmd *service.UsageBillingCommand) error {
	if r == nil || r.db == nil || cmd == nil || cmd.MemberID == nil {
		return service.ErrEnterpriseMemberUsagePersistenceUnavailable
	}
	payload, err := json.Marshal(enterpriseMemberSettlementPayload{Version: enterpriseMemberSettlementPayloadVersion, Command: cmd})
	if err != nil {
		return fmt.Errorf("marshal enterprise member settlement command: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin enterprise member settlement staging: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Serialize creation of the durable settlement intent with every path that
	// may release the same budget receipt. Without this row lock, an operator
	// could observe an empty outbox, release the hold, and race a successful
	// request that stages its settlement immediately afterwards.
	var receiptMemberID int64
	var receiptStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT member_id, status
		FROM enterprise_member_budget_reservations
		WHERE request_id = $1
		FOR UPDATE`, cmd.MemberBudgetRequestID).Scan(&receiptMemberID, &receiptStatus)
	if err == nil {
		if receiptMemberID != *cmd.MemberID {
			return service.ErrEnterpriseMemberBudgetConflict
		}
		switch receiptStatus {
		case "reserved", "ambiguous", "settled":
			// Open receipts may be settled; a settled receipt may be replayed
			// idempotently and will be cleaned up by the billing dedup record.
		default:
			return service.ErrEnterpriseMemberBudgetConflict
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock enterprise member budget receipt for settlement staging: %w", err)
	}

	var id int64
	err = tx.QueryRowContext(ctx, `
			INSERT INTO enterprise_member_usage_settlement_outbox
			(api_key_id, member_id, enterprise_user_id, request_id, member_budget_request_id, request_fingerprint, command_payload)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (api_key_id, request_id) DO UPDATE
			SET command_payload = enterprise_member_usage_settlement_outbox.command_payload
			WHERE enterprise_member_usage_settlement_outbox.member_id = EXCLUDED.member_id
			  AND enterprise_member_usage_settlement_outbox.enterprise_user_id = EXCLUDED.enterprise_user_id
			  AND enterprise_member_usage_settlement_outbox.member_budget_request_id = EXCLUDED.member_budget_request_id
			  AND enterprise_member_usage_settlement_outbox.request_fingerprint = EXCLUDED.request_fingerprint
			  AND enterprise_member_usage_settlement_outbox.command_payload = EXCLUDED.command_payload
		RETURNING id`,
		cmd.APIKeyID, *cmd.MemberID, cmd.UserID, cmd.RequestID, cmd.MemberBudgetRequestID, cmd.RequestFingerprint, payload,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrUsageBillingRequestConflict
	}
	if err != nil {
		return fmt.Errorf("stage enterprise member settlement: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit enterprise member settlement staging: %w", err)
	}
	return nil
}

func (r *usageBillingRepository) deleteEnterpriseMemberSettlement(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	if cmd == nil || cmd.MemberID == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		DELETE FROM enterprise_member_usage_settlement_outbox
		WHERE api_key_id = $1 AND request_id = $2 AND request_fingerprint = $3`,
		cmd.APIKeyID, cmd.RequestID, cmd.RequestFingerprint,
	)
	return err
}

func (r *usageBillingRepository) ReplayPendingEnterpriseMemberSettlements(ctx context.Context, limit int) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("usage billing repository db is nil")
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, command_payload
		FROM enterprise_member_usage_settlement_outbox
		WHERE next_attempt_at <= NOW()
		ORDER BY next_attempt_at, id
		LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type pendingSettlement struct {
		id      int64
		payload []byte
	}
	pending := make([]pendingSettlement, 0, limit)
	for rows.Next() {
		var item pendingSettlement
		if err := rows.Scan(&item.id, &item.payload); err != nil {
			_ = rows.Close()
			return 0, err
		}
		pending = append(pending, item)
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	replayed := 0
	for _, item := range pending {
		var payload enterpriseMemberSettlementPayload
		if err := json.Unmarshal(item.payload, &payload); err != nil || payload.Version != enterpriseMemberSettlementPayloadVersion || payload.Command == nil {
			replayErr := err
			if replayErr == nil {
				replayErr = errors.New("unsupported enterprise member settlement payload")
			}
			if updateErr := r.recordEnterpriseMemberSettlementReplayFailure(ctx, item.id, replayErr); updateErr != nil {
				return replayed, updateErr
			}
			continue
		}
		if _, err := r.Apply(ctx, payload.Command); err != nil {
			if updateErr := r.recordEnterpriseMemberSettlementReplayFailure(ctx, item.id, err); updateErr != nil {
				return replayed, updateErr
			}
			continue
		}
		replayed++
	}
	return replayed, nil
}

func (r *usageBillingRepository) recordEnterpriseMemberSettlementReplayFailure(ctx context.Context, id int64, replayErr error) error {
	message := "unknown settlement replay error"
	if replayErr != nil {
		message = strings.TrimSpace(replayErr.Error())
	}
	if len(message) > 2000 {
		message = message[:2000]
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_usage_settlement_outbox
		SET attempt_count = attempt_count + 1,
		    last_error = $1,
		    next_attempt_at = NOW() + INTERVAL '1 minute',
		    updated_at = NOW()
		WHERE id = $2`, message, id)
	return err
}

func (r *usageBillingRepository) claimUsageBillingKey(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (bool, error) {
	return r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
}

func (r *usageBillingRepository) claimUsageBillingRequest(ctx context.Context, tx *sql.Tx, requestID string, apiKeyID int64, requestFingerprint string) (bool, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO usage_billing_dedup (request_id, api_key_id, request_fingerprint)
		VALUES ($1, $2, $3)
		ON CONFLICT (request_id, api_key_id) DO NOTHING
		RETURNING id
	`, requestID, apiKeyID, requestFingerprint).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		var existingFingerprint string
		if err := tx.QueryRowContext(ctx, `
			SELECT request_fingerprint
			FROM usage_billing_dedup
			WHERE request_id = $1 AND api_key_id = $2
		`, requestID, apiKeyID).Scan(&existingFingerprint); err != nil {
			return false, err
		}
		if strings.TrimSpace(existingFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var archivedFingerprint string
	err = tx.QueryRowContext(ctx, `
		SELECT request_fingerprint
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, requestID, apiKeyID).Scan(&archivedFingerprint)
	if err == nil {
		if strings.TrimSpace(archivedFingerprint) != strings.TrimSpace(requestFingerprint) {
			return false, service.ErrUsageBillingRequestConflict
		}
		return false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return true, nil
}

func (r *usageBillingRepository) ReserveBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, func(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
		result, err := reserveUsageBillingBatchImageBalance(ctx, tx, cmd)
		if err != nil {
			return nil, err
		}
		if err := reserveBatchImageEnterpriseMemberBudget(ctx, tx, cmd); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (r *usageBillingRepository) CaptureBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, func(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
		result, err := captureUsageBillingBatchImageBalance(ctx, tx, cmd)
		if err != nil {
			return nil, err
		}
		if cmd.MemberID != nil && strings.TrimSpace(cmd.MemberBudgetRequestID) != "" {
			if err := settleEnterpriseMemberBudget(ctx, tx, &service.UsageBillingCommand{
				MemberID: cmd.MemberID, MemberBudgetRequestID: cmd.MemberBudgetRequestID, MemberBudgetCost: cmd.ActualAmount,
			}); err != nil {
				return nil, err
			}
		}
		if cmd.UsageLog != nil {
			if _, err := createUsageLogSingle(ctx, tx, cmd.UsageLog); err != nil {
				return nil, err
			}
			result.UsageLogPersisted = true
		}
		return result, nil
	})
}

func (r *usageBillingRepository) ReleaseBatchImageBalance(ctx context.Context, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	return r.applyBatchImageBalanceHold(ctx, cmd, func(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
		result, err := releaseUsageBillingBatchImageBalance(ctx, tx, cmd)
		if err != nil {
			return nil, err
		}
		if err := releaseBatchImageEnterpriseMemberBudget(ctx, tx, cmd.MemberBudgetRequestID); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func reserveBatchImageEnterpriseMemberBudget(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) error {
	if cmd == nil || cmd.MemberID == nil || *cmd.MemberID <= 0 || strings.TrimSpace(cmd.MemberBudgetRequestID) == "" {
		return nil
	}
	periodStart, enforced, err := reserveEnterpriseMemberSpendingLimits(ctx, tx, *cmd.MemberID, cmd.HoldAmount, time.Now())
	if err != nil {
		return err
	}
	reservedAmount := cmd.HoldAmount
	if !enforced {
		reservedAmount = 0
	}
	expiresAt := cmd.MemberBudgetExpiresAt
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_reservations
			(request_id, member_id, group_id, request_payload_hash, period_start, reserved_usd, receipt_kind, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'batch_image', $7)`,
		cmd.MemberBudgetRequestID, *cmd.MemberID, cmd.GroupID, strings.TrimSpace(cmd.RequestPayloadHash), periodStart, reservedAmount, expiresAt)
	if err != nil {
		if isUniqueConstraintViolation(err) {
			return service.ErrEnterpriseMemberBudgetConflict
		}
		return err
	}
	return nil
}

func releaseBatchImageEnterpriseMemberBudget(ctx context.Context, tx *sql.Tx, requestID string) error {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return nil
	}
	var memberID int64
	var periodStart time.Time
	var amount float64
	var status string
	err := tx.QueryRowContext(ctx, `SELECT member_id, period_start, reserved_usd, status FROM enterprise_member_budget_reservations WHERE request_id = $1 FOR UPDATE`, requestID).Scan(&memberID, &periodStart, &amount, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != "reserved" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET reserved_usd = GREATEST(0, reserved_usd - $1), version = version + 1, updated_at = NOW() WHERE member_id = $2 AND period_start = $3`, amount, memberID, periodStart); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE enterprise_member_budget_reservations SET status = 'released', outcome_reason = 'batch_released', updated_at = NOW() WHERE request_id = $1`, requestID)
	return err
}

func (r *usageBillingRepository) applyBatchImageBalanceHold(
	ctx context.Context,
	cmd *service.BatchImageBalanceHoldCommand,
	apply func(context.Context, *sql.Tx, *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error),
) (_ *service.BatchImageBalanceHoldResult, err error) {
	if cmd == nil {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	if r == nil || r.db == nil {
		return nil, errors.New("usage billing repository db is nil")
	}
	cmd.Normalize()
	if cmd.RequestID == "" {
		return nil, service.ErrUsageBillingRequestIDRequired
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	applied, err := r.claimUsageBillingRequest(ctx, tx, cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint)
	if err != nil {
		return nil, err
	}
	if !applied {
		return &service.BatchImageBalanceHoldResult{Applied: false}, nil
	}

	result, err := apply(ctx, tx, cmd)
	if err != nil {
		return nil, err
	}
	if result == nil {
		result = &service.BatchImageBalanceHoldResult{}
	}
	result.Applied = true

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	tx = nil
	return result, nil
}

func (r *usageBillingRepository) applyUsageBillingEffects(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand, result *service.UsageBillingApplyResult) error {
	if cmd.SubscriptionCost > 0 && cmd.SubscriptionID != nil {
		if err := incrementUsageBillingSubscription(ctx, tx, *cmd.SubscriptionID, cmd.SubscriptionCost); err != nil {
			return err
		}
	}

	if cmd.BalanceCost > 0 {
		newBalance, sufficient, err := deductUsageBillingBalance(ctx, tx, cmd.UserID, cmd.BalanceCost)
		if err != nil {
			return err
		}
		result.NewBalance = &newBalance
		result.BalanceOverdrafted = !sufficient
	}

	if cmd.APIKeyQuotaCost > 0 {
		exhausted, err := incrementUsageBillingAPIKeyQuota(ctx, tx, cmd.APIKeyID, cmd.APIKeyQuotaCost)
		if err != nil {
			return err
		}
		result.APIKeyQuotaExhausted = exhausted
	}

	if cmd.APIKeyRateLimitCost > 0 {
		if err := incrementUsageBillingAPIKeyRateLimit(ctx, tx, cmd.APIKeyID, cmd.APIKeyRateLimitCost); err != nil {
			return err
		}
	}

	if cmd.AccountQuotaCost > 0 && (strings.EqualFold(cmd.AccountType, service.AccountTypeAPIKey) || strings.EqualFold(cmd.AccountType, service.AccountTypeBedrock)) {
		quotaState, err := incrementUsageBillingAccountQuota(ctx, tx, cmd.AccountID, cmd.AccountQuotaCost)
		if err != nil {
			return err
		}
		result.QuotaState = quotaState
	}

	if cmd.MemberID != nil && cmd.MemberBudgetRequestID != "" {
		if err := settleEnterpriseMemberBudget(ctx, tx, cmd); err != nil {
			return err
		}
	}

	return nil
}

func settleEnterpriseMemberBudget(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	var memberID int64
	var periodStart time.Time
	var reservedUSD float64
	var status string
	err := tx.QueryRowContext(ctx, `
		SELECT member_id, period_start, reserved_usd, status
		FROM enterprise_member_budget_reservations
		WHERE request_id = $1
		FOR UPDATE`, cmd.MemberBudgetRequestID).Scan(&memberID, &periodStart, &reservedUSD, &status)
	if errors.Is(err, sql.ErrNoRows) {
		// Unlimited members do not reserve budget, but every completed request —
		// including a zero-cost request — still needs a zero-amount ledger row so
		// request/token facts inherit the authoritative budget period. A member
		// with any positive limit must have reserved first and fails closed here.
		return settleUnlimitedEnterpriseMemberBudget(ctx, tx, cmd)
	}
	if err != nil {
		return err
	}
	if cmd.MemberID == nil || memberID != *cmd.MemberID {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	if status == "settled" {
		return nil
	}
	holdsReservation := status == "reserved" || status == "ambiguous"
	if !holdsReservation {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	reservationWasLimited := reservedUSD > 1e-8
	if reservationWasLimited && cmd.MemberBudgetCost > reservedUSD+1e-8 {
		service.RecordEnterpriseMemberBudgetSettlementOverrun()
		logger.LegacyPrintf(
			"repository.usage_billing",
			"enterprise member settlement exceeded reservation: member=%d reserved=%.8f actual=%.8f",
			memberID,
			reservedUSD,
			cmd.MemberBudgetCost,
		)
	}
	reservedDelta := reservedUSD
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_periods
		SET used_usd = used_usd + $1,
			reserved_usd = GREATEST(0, reserved_usd - $2),
			version = version + 1,
			updated_at = NOW()
		WHERE member_id = $3 AND period_start = $4`, cmd.MemberBudgetCost, reservedDelta, memberID, periodStart); err != nil {
		return err
	}
	if !reservationWasLimited {
		if err := ensureEnterpriseMemberRateLimitPeriod(ctx, tx, memberID); err != nil {
			return err
		}
	}
	if err := incrementEnterpriseMemberRateLimitUsage(ctx, tx, memberID, cmd.MemberBudgetCost); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_reservations
		SET actual_usd = $1, status = 'settled', outcome_reason = 'settled', updated_at = NOW()
		WHERE request_id = $2`, cmd.MemberBudgetCost, cmd.MemberBudgetRequestID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, request_id, amount_usd, idempotency_key, note)
		VALUES ($1, $2, 'usage', $3, $4, $5, '')
		ON CONFLICT (request_id) DO NOTHING`, memberID, periodStart, cmd.MemberBudgetRequestID, cmd.MemberBudgetCost, "usage:"+cmd.MemberBudgetRequestID)
	return err
}

func settleUnlimitedEnterpriseMemberBudget(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) error {
	if cmd == nil || cmd.MemberID == nil || *cmd.MemberID <= 0 {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	var monthlyLimit, limit5h, limit1d, limit7d float64
	if err := tx.QueryRowContext(ctx, `SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d FROM enterprise_members WHERE id = $1`, *cmd.MemberID).
		Scan(&monthlyLimit, &limit5h, &limit1d, &limit7d); err != nil {
		return err
	}
	// A positive limit must always have passed through a durable reservation.
	// Only the explicit zero-limit (unlimited) policy may settle without one.
	if monthlyLimit > 0 || limit5h > 0 || limit1d > 0 || limit7d > 0 {
		return service.ErrEnterpriseMemberBudgetConflict
	}
	location, err := time.LoadLocation(enterpriseBudgetTimezone())
	if err != nil {
		return err
	}
	now := time.Now().In(location)
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone)
		VALUES ($1, $2, $3)
		ON CONFLICT (member_id, period_start) DO NOTHING`, *cmd.MemberID, periodStart, enterpriseBudgetTimezone()); err != nil {
		return err
	}
	inserted, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, request_id, amount_usd, idempotency_key, note)
		VALUES ($1, $2, 'usage', $3, $4, $5, '')
		ON CONFLICT (request_id) DO NOTHING`, *cmd.MemberID, periodStart, cmd.MemberBudgetRequestID, cmd.MemberBudgetCost, "usage:"+cmd.MemberBudgetRequestID)
	if err != nil {
		return err
	}
	affected, err := inserted.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_periods
		SET used_usd = used_usd + $1, version = version + 1, updated_at = NOW()
		WHERE member_id = $2 AND period_start = $3`, cmd.MemberBudgetCost, *cmd.MemberID, periodStart)
	return err
}

func incrementUsageBillingSubscription(ctx context.Context, tx *sql.Tx, subscriptionID int64, costUSD float64) error {
	const updateSQL = `
		UPDATE user_subscriptions us
		SET
			daily_usage_usd = us.daily_usage_usd + $1,
			weekly_usage_usd = us.weekly_usage_usd + $1,
			monthly_usage_usd = us.monthly_usage_usd + $1,
			updated_at = NOW()
		FROM groups g
		WHERE us.id = $2
			AND us.deleted_at IS NULL
			AND us.group_id = g.id
			AND g.deleted_at IS NULL
	`
	res, err := tx.ExecContext(ctx, updateSQL, costUSD, subscriptionID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	return service.ErrSubscriptionNotFound
}

func deductUsageBillingBalance(ctx context.Context, tx *sql.Tx, userID int64, amount float64) (float64, bool, error) {
	var newBalance float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if err == nil {
		return newBalance, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}

	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING balance
	`, amount, userID).Scan(&newBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, service.ErrUserNotFound
	}
	if err != nil {
		return 0, false, err
	}
	return newBalance, false, nil
}

func reserveUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance - $1,
			frozen_balance = COALESCE(frozen_balance, 0) + $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND balance >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, service.ErrBatchImageInsufficientBalance
}

func captureUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 && cmd.ActualAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1 - $2,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.ActualAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

func releaseUsageBillingBatchImageBalance(ctx context.Context, tx *sql.Tx, cmd *service.BatchImageBalanceHoldCommand) (*service.BatchImageBalanceHoldResult, error) {
	if cmd.HoldAmount <= 0 {
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	// 释放前校验该 job 确实预留过 hold（hold request id 已被 claim），
	// 防止从未成功冻结的 job 触发"幻影释放"，从其他用户的冻结资金池中凭空生成余额。
	held, heldErr := batchImageHoldClaimExists(ctx, tx, service.BatchImageHoldRequestID(cmd.BatchID), cmd.APIKeyID)
	if heldErr != nil {
		return nil, heldErr
	}
	if !held {
		logger.LegacyPrintf("repository.usage_billing", "[BatchImage] release skipped, hold was never reserved: batch=%s", cmd.BatchID)
		return &service.BatchImageBalanceHoldResult{}, nil
	}
	var balance, frozen float64
	err := tx.QueryRowContext(ctx, `
		UPDATE users
		SET balance = balance + $1,
			frozen_balance = COALESCE(frozen_balance, 0) - $1,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL AND COALESCE(frozen_balance, 0) >= $1
		RETURNING balance, frozen_balance
	`, cmd.HoldAmount, cmd.UserID).Scan(&balance, &frozen)
	if err == nil {
		return &service.BatchImageBalanceHoldResult{NewBalance: &balance, FrozenBalance: &frozen}, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if exists, existsErr := userExistsForBilling(ctx, tx, cmd.UserID); existsErr != nil {
		return nil, existsErr
	} else if !exists {
		return nil, service.ErrUserNotFound
	}
	return nil, errors.New("batch image frozen balance is insufficient")
}

// batchImageHoldClaimExists 检查 hold request id 是否已在 dedup（或归档）表中被 claim，
// 即该 batch 的冻结操作确实成功提交过。
func batchImageHoldClaimExists(ctx context.Context, tx *sql.Tx, holdRequestID string, apiKeyID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	err = tx.QueryRowContext(ctx, `
		SELECT 1
		FROM usage_billing_dedup_archive
		WHERE request_id = $1 AND api_key_id = $2
	`, holdRequestID, apiKeyID).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func userExistsForBilling(ctx context.Context, tx *sql.Tx, userID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func incrementUsageBillingAPIKeyQuota(ctx context.Context, tx *sql.Tx, apiKeyID int64, amount float64) (bool, error) {
	var exhausted bool
	err := tx.QueryRowContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + $1,
			status = CASE
				WHEN quota > 0
					AND status = $3
					AND quota_used < quota
					AND quota_used + $1 >= quota
				THEN $4
				ELSE status
			END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING quota > 0 AND quota_used >= quota AND quota_used - $1 < quota
	`, amount, apiKeyID, service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted).Scan(&exhausted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, service.ErrAPIKeyNotFound
	}
	if err != nil {
		return false, err
	}
	return exhausted, nil
}

func incrementUsageBillingAPIKeyRateLimit(ctx context.Context, tx *sql.Tx, apiKeyID int64, cost float64) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE api_keys SET
			usage_5h = CASE WHEN window_5h_start IS NOT NULL AND window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NOT NULL AND window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NOT NULL AND window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN date_trunc('day', NOW()) ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN date_trunc('day', NOW()) ELSE window_7d_start END,
			updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
	`, cost, apiKeyID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrAPIKeyNotFound
	}
	return nil
}

func incrementUsageBillingAccountQuota(ctx context.Context, tx *sql.Tx, accountID int64, amount float64) (*service.AccountQuotaState, error) {
	rows, err := tx.QueryContext(ctx,
		`UPDATE accounts SET extra = (
			COALESCE(extra, '{}'::jsonb)
			|| jsonb_build_object('quota_used', COALESCE((extra->>'quota_used')::numeric, 0) + $1)
			|| CASE WHEN COALESCE((extra->>'quota_daily_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_daily_used',
					CASE WHEN `+dailyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_daily_used')::numeric, 0) + $1 END,
					'quota_daily_start',
					CASE WHEN `+dailyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_daily_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+dailyExpiredExpr+` AND `+nextDailyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_daily_reset_at', `+nextDailyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
			|| CASE WHEN COALESCE((extra->>'quota_weekly_limit')::numeric, 0) > 0 THEN
				jsonb_build_object(
					'quota_weekly_used',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN $1
					ELSE COALESCE((extra->>'quota_weekly_used')::numeric, 0) + $1 END,
					'quota_weekly_start',
					CASE WHEN `+weeklyExpiredExpr+`
					THEN `+nowUTC+`
					ELSE COALESCE(extra->>'quota_weekly_start', `+nowUTC+`) END
				)
				|| CASE WHEN `+weeklyExpiredExpr+` AND `+nextWeeklyResetAtExpr+` IS NOT NULL
				   THEN jsonb_build_object('quota_weekly_reset_at', `+nextWeeklyResetAtExpr+`)
				   ELSE '{}'::jsonb END
			ELSE '{}'::jsonb END
		), updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING
			COALESCE((extra->>'quota_used')::numeric, 0),
			COALESCE((extra->>'quota_limit')::numeric, 0),
			COALESCE((extra->>'quota_daily_used')::numeric, 0),
			COALESCE((extra->>'quota_daily_limit')::numeric, 0),
			COALESCE((extra->>'quota_weekly_used')::numeric, 0),
			COALESCE((extra->>'quota_weekly_limit')::numeric, 0)`,
		amount, accountID)
	if err != nil {
		return nil, err
	}

	var state service.AccountQuotaState
	if rows.Next() {
		if err := rows.Scan(
			&state.TotalUsed, &state.TotalLimit,
			&state.DailyUsed, &state.DailyLimit,
			&state.WeeklyUsed, &state.WeeklyLimit,
		); err != nil {
			_ = rows.Close()
			return nil, err
		}
	} else {
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		_ = rows.Close()
		return nil, service.ErrAccountNotFound
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	// 必须在执行下一条 SQL 前显式关闭 rows：pq 驱动在同一连接上
	// 不允许前一条查询的结果集未耗尽时启动新查询，否则会返回
	// "unexpected Parse response" 错误。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	// 任意维度额度在本次递增中从"未超"跨越到"已超"时，必须刷新调度快照，
	// 否则 Redis 中缓存的 Account 仍显示旧的 used 值，后续请求会继续选中本账号，
	// 最终观察到 daily_used / weekly_used 大幅超过配置的 limit。
	// 对于日/周额度，即使本次触发了周期重置（pre=0、post=amount），
	// 判定式 (post-amount) < limit 同样成立，逻辑与总额度保持一致。
	crossedTotal := state.TotalLimit > 0 && state.TotalUsed >= state.TotalLimit && (state.TotalUsed-amount) < state.TotalLimit
	crossedDaily := state.DailyLimit > 0 && state.DailyUsed >= state.DailyLimit && (state.DailyUsed-amount) < state.DailyLimit
	crossedWeekly := state.WeeklyLimit > 0 && state.WeeklyUsed >= state.WeeklyLimit && (state.WeeklyUsed-amount) < state.WeeklyLimit
	if crossedTotal || crossedDaily || crossedWeekly {
		if err := enqueueSchedulerOutbox(ctx, tx, service.SchedulerOutboxEventAccountChanged, &accountID, nil, nil); err != nil {
			logger.LegacyPrintf("repository.usage_billing", "[SchedulerOutbox] enqueue quota exceeded failed: account=%d err=%v", accountID, err)
			return nil, err
		}
	}
	return &state, nil
}
