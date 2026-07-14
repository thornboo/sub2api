package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseMemberBudgetRepository struct{ db *sql.DB }

func NewEnterpriseMemberBudgetRepository(db *sql.DB) service.EnterpriseMemberBudgetRepository {
	return &enterpriseMemberBudgetRepository{db: db}
}

func (r *enterpriseMemberBudgetRepository) Reserve(ctx context.Context, requestID string, memberID int64, amount float64, expiresAt time.Time) (_ *service.EnterpriseMemberBudgetReservation, err error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member budget repository db is nil")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var existing service.EnterpriseMemberBudgetReservation
	err = tx.QueryRowContext(ctx, `
		SELECT id, request_id, member_id, period_start, reserved_usd, actual_usd, status, usage_log_id, expires_at
		FROM enterprise_member_budget_reservations WHERE request_id = $1 FOR UPDATE`, requestID).
		Scan(&existing.ID, &existing.RequestID, &existing.MemberID, &existing.PeriodStart, &existing.ReservedUSD, &existing.ActualUSD, &existing.Status, &existing.UsageLogID, &existing.ExpiresAt)
	if err == nil {
		if existing.MemberID != memberID || mathAbs(existing.ReservedUSD-amount) > 1e-8 {
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
	if !enforced {
		return nil, service.ErrEnterpriseMemberBudgetConflict
	}
	reservation := &service.EnterpriseMemberBudgetReservation{RequestID: requestID, MemberID: memberID, PeriodStart: periodStart, ReservedUSD: amount, Status: "reserved", ExpiresAt: expiresAt}
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO enterprise_member_budget_reservations (request_id, member_id, period_start, reserved_usd, expires_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`, requestID, memberID, periodStart, amount, expiresAt).Scan(&reservation.ID); err != nil {
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
	err = tx.QueryRowContext(ctx, `SELECT member_id, period_start, reserved_usd, status FROM enterprise_member_budget_reservations WHERE request_id = $1 FOR UPDATE`, requestID).Scan(&memberID, &periodStart, &amount, &status)
	if errors.Is(err, sql.ErrNoRows) || status != "reserved" {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET reserved_usd = GREATEST(0, reserved_usd - $1), version = version + 1, updated_at = NOW() WHERE member_id = $2 AND period_start = $3`, amount, memberID, periodStart); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_reservations SET status = 'released', updated_at = NOW() WHERE request_id = $1`, requestID); err != nil {
		return err
	}
	return tx.Commit()
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
		WHERE m.id = $1 AND m.deleted_at IS NULL`, memberID, periodStart.Format("2006-01-02")).
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
		summary.RemainingUSD = summary.LimitUSD - summary.UsedUSD - summary.ReservedUSD
		if summary.RemainingUSD < 0 {
			summary.RemainingUSD = 0
		}
	}
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0)
		FROM usage_logs
		WHERE member_id = $1 AND created_at >= $2 AND created_at < $3`, memberID, periodStart, periodEnd).
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
		SELECT m.id, m.member_code, m.name, m.status, m.monthly_limit_usd,
		       COALESCE(p.used_usd, 0), COALESCE(p.reserved_usd, 0),
		       COALESCE(u.request_count, 0), COALESCE(u.input_tokens, 0), COALESCE(u.output_tokens, 0),
		       COALESCE(b.billed_usd, 0), COALESCE(b.total_tokens, 0), COALESCE(b.input_tokens, 0),
		       COALESCE(b.output_tokens, 0), COALESCE(b.cache_tokens, 0), COALESCE(b.cache_creation_tokens, 0),
		       COALESCE(b.cache_read_tokens, 0)
		FROM enterprise_members m
		LEFT JOIN enterprise_member_budget_periods p ON p.member_id = m.id AND p.period_start = $2
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS request_count, COALESCE(SUM(input_tokens), 0) AS input_tokens, COALESCE(SUM(output_tokens), 0) AS output_tokens
			FROM usage_logs ul WHERE ul.member_id = m.id AND ul.created_at >= $3 AND ul.created_at < $4
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
		ORDER BY m.id`, ownerID, periodStart.Format("2006-01-02"), periodStart, periodEnd)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.EnterpriseMemberOwnerUsageItem
		if err := rows.Scan(&item.MemberID, &item.MemberCode, &item.MemberName, &item.Status, &item.LimitUSD,
			&item.UsedUSD, &item.ReservedUSD, &item.RequestCount, &item.InputTokens, &item.OutputTokens,
			&item.MigrationBilledUSD, &item.MigrationTotalTokens, &item.MigrationInputTokens,
			&item.MigrationOutputTokens, &item.MigrationCacheTokens, &item.MigrationCacheWriteTokens,
			&item.MigrationCacheReadTokens); err != nil {
			return nil, err
		}
		if item.LimitUSD <= 0 {
			item.RemainingUSD = -1
		} else {
			item.RemainingUSD = item.LimitUSD - item.UsedUSD - item.ReservedUSD
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
		summary.MigrationTotalTokens += item.MigrationTotalTokens
		summary.MigrationInputTokens += item.MigrationInputTokens
		summary.MigrationOutputTokens += item.MigrationOutputTokens
		summary.MigrationCacheTokens += item.MigrationCacheTokens
		summary.MigrationCacheWriteTokens += item.MigrationCacheWriteTokens
		summary.MigrationCacheReadTokens += item.MigrationCacheReadTokens
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
		SELECT TO_CHAR(created_at AT TIME ZONE $4, 'YYYY-MM-DD'), COUNT(*),
		       COALESCE(SUM(input_tokens), 0), COALESCE(SUM(output_tokens), 0), COALESCE(SUM(actual_cost), 0)
		FROM usage_logs WHERE user_id = $1 AND member_id IS NOT NULL AND created_at >= $2 AND created_at < $3
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
		SELECT request_id, member_id, period_start, reserved_usd
		FROM enterprise_member_budget_reservations
		WHERE status = 'reserved' AND expires_at <= NOW()
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
	}
	items := make([]expiredReservation, 0, limit)
	for rows.Next() {
		var item expiredReservation
		if err := rows.Scan(&item.requestID, &item.memberID, &item.periodStart, &item.reservedUSD); err != nil {
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
		usageErr := tx.QueryRowContext(ctx, `SELECT id, actual_cost FROM usage_logs WHERE api_key_id = $1 AND request_id = $2`, apiKeyID, usageRequestID).Scan(&usageLogID, &actualUSD)
		if usageErr == nil {
			if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET used_usd = used_usd + $1, reserved_usd = GREATEST(0, reserved_usd - $2), version = version + 1, updated_at = NOW() WHERE member_id = $3 AND period_start = $4`, actualUSD, item.reservedUSD, item.memberID, item.periodStart); err != nil {
				return recovered, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_reservations SET actual_usd = $1, status = 'settled', usage_log_id = $2, updated_at = NOW() WHERE request_id = $3`, actualUSD, usageLogID, item.requestID); err != nil {
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
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM usage_billing_dedup WHERE api_key_id = $1 AND request_id = $2)`, apiKeyID, usageRequestID).Scan(&billed); err != nil {
			return recovered, err
		}
		if billed {
			if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_reservations SET expires_at = NOW() + INTERVAL '10 minutes', updated_at = NOW() WHERE request_id = $1`, item.requestID); err != nil {
				return recovered, err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_periods SET reserved_usd = GREATEST(0, reserved_usd - $1), version = version + 1, updated_at = NOW() WHERE member_id = $2 AND period_start = $3`, item.reservedUSD, item.memberID, item.periodStart); err != nil {
			return recovered, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_budget_reservations SET status = 'expired', updated_at = NOW() WHERE request_id = $1`, item.requestID); err != nil {
			return recovered, err
		}
		recovered++
	}
	if err := tx.Commit(); err != nil {
		return recovered, err
	}
	return recovered, nil
}

// ReconcilePeriods repairs the rebuildable monthly projection from immutable
// ledger entries and live reservations. A usage log that survived a crash
// without a corresponding ledger entry is recovered as an explicit
// reconciliation entry; it is never silently folded into the projection.
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
				(member_id, period_start, kind, amount_usd, idempotency_key, note)
			SELECT ul.member_id, $2, 'reconciliation', GREATEST(ul.actual_cost, 0),
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
				  AND reservation.status = 'reserved'
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
			WHERE member_id = $1 AND period_start = $2 AND status = 'reserved'`, period.memberID, period.periodStart.Format("2006-01-02")).Scan(&liveReserved); err != nil {
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
