//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetReservationLockAllowsSettlementForeignKeyCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	suffix := fmt.Sprintf("member-budget-lock-%d", time.Now().UnixNano())
	owner, err := integrationEntClient.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("integration-test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)

	var memberID int64
	err = integrationDB.QueryRowContext(ctx, `
		INSERT INTO enterprise_members
			(enterprise_user_id, member_code, name, status,
			 monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, version)
		VALUES ($1, $2, $3, 'active', 0, 0, 0, 0, 1)
		RETURNING id`, owner.ID, suffix, suffix).Scan(&memberID)
	require.NoError(t, err)

	location, err := time.LoadLocation(enterpriseBudgetTimezone())
	require.NoError(t, err)
	now := time.Now().In(location)
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone)
		VALUES ($1, $2, $3)`, memberID, periodStart, enterpriseBudgetTimezone())
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, `
			TRUNCATE TABLE enterprise_member_audit_logs, enterprise_member_import_jobs, enterprise_members CASCADE`)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, owner.ID)
		require.NoError(t, cleanupErr)
	})

	settlementTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = settlementTx.Rollback() })
	var periodID int64
	err = settlementTx.QueryRowContext(ctx, `
		SELECT id
		FROM enterprise_member_budget_periods
		WHERE member_id = $1 AND period_start = $2
		FOR UPDATE`, memberID, periodStart).Scan(&periodID)
	require.NoError(t, err)
	require.NotZero(t, periodID)

	reservationTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reservationTx.Rollback() })
	var reservationPID int
	require.NoError(t, reservationTx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&reservationPID))

	reservationDone := make(chan error, 1)
	go func() {
		_, _, reserveErr := reserveEnterpriseMemberSpendingLimits(ctx, reservationTx, memberID, 0, now)
		reservationDone <- reserveErr
	}()

	var reservationWaitingQuery string
	require.Eventually(t, func() bool {
		var waitEventType *string
		queryErr := integrationDB.QueryRowContext(ctx, `
			SELECT wait_event_type, query
			FROM pg_stat_activity
			WHERE pid = $1`, reservationPID).Scan(&waitEventType, &reservationWaitingQuery)
		return queryErr == nil && waitEventType != nil && *waitEventType == "Lock"
	}, 3*time.Second, 10*time.Millisecond, "reservation transaction should wait on the budget period lock")
	require.Contains(t, reservationWaitingQuery, "enterprise_member_budget_periods")

	_, err = settlementTx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, amount_usd, idempotency_key, note)
		VALUES ($1, $2, 'manual_adjustment', 0, $3, '')`, memberID, periodStart, suffix)
	require.NoError(t, err)
	require.NoError(t, settlementTx.Commit())
	require.NoError(t, <-reservationDone)
	require.NoError(t, reservationTx.Commit())
}
