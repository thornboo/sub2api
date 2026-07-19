package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type enterpriseMemberSpendingLimitState struct {
	monthlyLimit float64
	limit5h      float64
	limit1d      float64
	limit7d      float64
	monthlyUsed  float64
	reserved     float64
	usage5h      float64
	usage1d      float64
	usage7d      float64
	window5h     *time.Time
	window1d     *time.Time
	window7d     *time.Time
	periodStart  time.Time
}

// reserveEnterpriseMemberSpendingLimits serializes every member-level spending
// decision on the member row. A zero amount is a synchronous request receipt:
// it checks settled usage only and deliberately ignores historical holds. A
// positive amount is an asynchronous task hold and keeps strict reservation
// semantics until the task captures or releases it.
func reserveEnterpriseMemberSpendingLimits(ctx context.Context, tx *sql.Tx, memberID int64, amount float64, now time.Time) (time.Time, bool, error) {
	state, err := lockEnterpriseMemberSpendingLimitState(ctx, tx, memberID, now)
	if err != nil {
		return time.Time{}, false, err
	}
	if state.monthlyLimit <= 0 && state.limit5h <= 0 && state.limit1d <= 0 && state.limit7d <= 0 {
		return state.periodStart, false, nil
	}
	if amount <= 0 {
		if state.monthlyLimit > 0 && state.monthlyUsed >= state.monthlyLimit {
			return time.Time{}, false, service.ErrEnterpriseMemberBudgetExceeded
		}
		if state.limit5h > 0 && state.usage5h >= state.limit5h {
			return time.Time{}, false, service.ErrEnterpriseMemberRateLimit5hExceeded
		}
		if state.limit1d > 0 && state.usage1d >= state.limit1d {
			return time.Time{}, false, service.ErrEnterpriseMemberRateLimit1dExceeded
		}
		if state.limit7d > 0 && state.usage7d >= state.limit7d {
			return time.Time{}, false, service.ErrEnterpriseMemberRateLimit7dExceeded
		}
		return state.periodStart, true, nil
	}

	// Preserve the established exhaustion errors when settled usage alone has
	// consumed the limit. If only a new asynchronous hold cannot fit, return a
	// distinct explanation so clients do not mistake frozen task funds for
	// already billed usage.
	if state.monthlyLimit > 0 && state.monthlyUsed >= state.monthlyLimit {
		return time.Time{}, false, service.ErrEnterpriseMemberBudgetExceeded
	}
	if state.limit5h > 0 && state.usage5h >= state.limit5h {
		return time.Time{}, false, service.ErrEnterpriseMemberRateLimit5hExceeded
	}
	if state.limit1d > 0 && state.usage1d >= state.limit1d {
		return time.Time{}, false, service.ErrEnterpriseMemberRateLimit1dExceeded
	}
	if state.limit7d > 0 && state.usage7d >= state.limit7d {
		return time.Time{}, false, service.ErrEnterpriseMemberRateLimit7dExceeded
	}

	pending := state.reserved + amount
	if state.monthlyLimit > 0 && state.monthlyUsed+pending > state.monthlyLimit {
		return time.Time{}, false, enterpriseMemberAsyncBudgetUnavailable("monthly", state.monthlyLimit, state.monthlyUsed, state.reserved, amount)
	}
	if state.limit5h > 0 && state.usage5h+pending > state.limit5h {
		return time.Time{}, false, enterpriseMemberAsyncBudgetUnavailable("5h", state.limit5h, state.usage5h, state.reserved, amount)
	}
	if state.limit1d > 0 && state.usage1d+pending > state.limit1d {
		return time.Time{}, false, enterpriseMemberAsyncBudgetUnavailable("1d", state.limit1d, state.usage1d, state.reserved, amount)
	}
	if state.limit7d > 0 && state.usage7d+pending > state.limit7d {
		return time.Time{}, false, enterpriseMemberAsyncBudgetUnavailable("7d", state.limit7d, state.usage7d, state.reserved, amount)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_budget_periods
		SET reserved_usd = reserved_usd + $1, version = version + 1, updated_at = NOW()
		WHERE member_id = $2 AND period_start = $3`, amount, memberID, state.periodStart); err != nil {
		return time.Time{}, false, err
	}
	if state.limit5h <= 0 && state.limit1d <= 0 && state.limit7d <= 0 {
		// Settlement increments the shared rolling-usage projection even when
		// no rolling limit is configured. A fresh monthly-only asynchronous
		// hold must initialize that row before it can be captured.
		if err := ensureEnterpriseMemberRateLimitPeriod(ctx, tx, memberID); err != nil {
			return time.Time{}, false, err
		}
		return state.periodStart, true, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_rate_limit_periods
		SET usage_5h = $1, usage_1d = $2, usage_7d = $3,
		    window_5h_start = $4, window_1d_start = $5, window_7d_start = $6,
		    updated_at = NOW()
		WHERE member_id = $7`, state.usage5h, state.usage1d, state.usage7d,
		state.window5h, state.window1d, state.window7d, memberID); err != nil {
		return time.Time{}, false, err
	}
	return state.periodStart, true, nil
}

func enterpriseMemberAsyncBudgetUnavailable(window string, limit, used, held, requested float64) error {
	format := func(value float64) string {
		return strconv.FormatFloat(value, 'f', 6, 64)
	}
	return service.ErrEnterpriseMemberAsyncBudgetUnavailable.WithMetadata(map[string]string{
		"limit_window":            window,
		"limit_usd":               format(limit),
		"settled_used_usd":        format(used),
		"active_task_holds_usd":   format(held),
		"requested_task_hold_usd": format(requested),
	})
}

func lockEnterpriseMemberSpendingLimitState(ctx context.Context, tx *sql.Tx, memberID int64, now time.Time) (*enterpriseMemberSpendingLimitState, error) {
	state := &enterpriseMemberSpendingLimitState{}
	var status string
	var deletedAt *time.Time
	if err := tx.QueryRowContext(ctx, `
		SELECT monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, status, deleted_at
		FROM enterprise_members WHERE id = $1 FOR UPDATE`, memberID).
		Scan(&state.monthlyLimit, &state.limit5h, &state.limit1d, &state.limit7d, &status, &deletedAt); err != nil {
		return nil, err
	}
	if deletedAt != nil || status != service.EnterpriseMemberStatusActive {
		return nil, service.ErrEnterpriseMemberNotFound
	}
	location, err := time.LoadLocation(enterpriseBudgetTimezone())
	if err != nil {
		return nil, err
	}
	localNow := now.In(location)
	state.periodStart = time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, location)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone)
		VALUES ($1, $2, $3) ON CONFLICT (member_id, period_start) DO NOTHING`, memberID, state.periodStart, enterpriseBudgetTimezone()); err != nil {
		return nil, err
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT used_usd, reserved_usd FROM enterprise_member_budget_periods
		WHERE member_id = $1 AND period_start = $2 FOR UPDATE`, memberID, state.periodStart).
		Scan(&state.monthlyUsed, &state.reserved); err != nil {
		return nil, err
	}
	if state.monthlyLimit <= 0 && state.limit5h <= 0 && state.limit1d <= 0 && state.limit7d <= 0 {
		return state, nil
	}
	if state.limit5h <= 0 && state.limit1d <= 0 && state.limit7d <= 0 {
		return state, nil
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_rate_limit_periods (member_id)
		VALUES ($1) ON CONFLICT (member_id) DO NOTHING`, memberID); err != nil {
		return nil, err
	}
	var window5h, window1d, window7d sql.NullTime
	if err := tx.QueryRowContext(ctx, `
		SELECT usage_5h, usage_1d, usage_7d, window_5h_start, window_1d_start, window_7d_start
		FROM enterprise_member_rate_limit_periods WHERE member_id = $1 FOR UPDATE`, memberID).
		Scan(&state.usage5h, &state.usage1d, &state.usage7d, &window5h, &window1d, &window7d); err != nil {
		return nil, err
	}
	state.usage5h, state.window5h = normalizeEnterpriseMemberWindow(state.usage5h, window5h, state.limit5h, service.RateLimitWindow5h, now)
	state.usage1d, state.window1d = normalizeEnterpriseMemberWindow(state.usage1d, window1d, state.limit1d, service.RateLimitWindow1d, now)
	state.usage7d, state.window7d = normalizeEnterpriseMemberWindow(state.usage7d, window7d, state.limit7d, service.RateLimitWindow7d, now)
	return state, nil
}

func normalizeEnterpriseMemberWindow(usage float64, start sql.NullTime, limit float64, duration time.Duration, now time.Time) (float64, *time.Time) {
	if limit <= 0 {
		if start.Valid {
			value := start.Time
			return usage, &value
		}
		return usage, nil
	}
	if !start.Valid || !now.Before(start.Time.Add(duration)) {
		value := now
		return 0, &value
	}
	value := start.Time
	return usage, &value
}

func incrementEnterpriseMemberRateLimitUsage(ctx context.Context, tx *sql.Tx, memberID int64, amount float64) error {
	if amount <= 0 {
		return nil
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE enterprise_member_rate_limit_periods SET
			usage_5h = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN $1 ELSE usage_5h + $1 END,
			usage_1d = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN $1 ELSE usage_1d + $1 END,
			usage_7d = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN $1 ELSE usage_7d + $1 END,
			window_5h_start = CASE WHEN window_5h_start IS NULL OR window_5h_start + INTERVAL '5 hours' <= NOW() THEN NOW() ELSE window_5h_start END,
			window_1d_start = CASE WHEN window_1d_start IS NULL OR window_1d_start + INTERVAL '24 hours' <= NOW() THEN NOW() ELSE window_1d_start END,
			window_7d_start = CASE WHEN window_7d_start IS NULL OR window_7d_start + INTERVAL '7 days' <= NOW() THEN NOW() ELSE window_7d_start END,
			updated_at = NOW()
		WHERE member_id = $2`, amount, memberID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("enterprise member rate limit period is missing")
	}
	return nil
}

func ensureEnterpriseMemberRateLimitPeriod(ctx context.Context, tx *sql.Tx, memberID int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO enterprise_member_rate_limit_periods (member_id)
		VALUES ($1) ON CONFLICT (member_id) DO NOTHING`, memberID)
	return err
}
