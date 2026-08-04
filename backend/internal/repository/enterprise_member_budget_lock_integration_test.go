//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberBudgetLockFixture struct {
	ownerID     int64
	memberID    int64
	periodStart time.Time
	now         time.Time
	suffix      string
}

func newEnterpriseMemberBudgetLockFixture(t *testing.T, ctx context.Context, prefix string, monthlyLimit float64) enterpriseMemberBudgetLockFixture {
	t.Helper()

	suffix := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
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
		VALUES ($1, $2, $3, 'active', $4, 0, 0, 0, 1)
		RETURNING id`, owner.ID, suffix, suffix, monthlyLimit).Scan(&memberID)
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

	return enterpriseMemberBudgetLockFixture{
		ownerID:     owner.ID,
		memberID:    memberID,
		periodStart: periodStart,
		now:         now,
		suffix:      suffix,
	}
}

func TestEnterpriseMemberBudgetReservationLockAllowsSettlementForeignKeyCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fixture := newEnterpriseMemberBudgetLockFixture(t, ctx, "member-budget-lock", 0)

	settlementTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = settlementTx.Rollback() })
	var periodID int64
	err = settlementTx.QueryRowContext(ctx, `
		SELECT id
		FROM enterprise_member_budget_periods
		WHERE member_id = $1 AND period_start = $2
		FOR UPDATE`, fixture.memberID, fixture.periodStart).Scan(&periodID)
	require.NoError(t, err)
	require.NotZero(t, periodID)

	reservationTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reservationTx.Rollback() })
	var reservationPID int
	require.NoError(t, reservationTx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&reservationPID))

	reservationDone := make(chan error, 1)
	go func() {
		_, _, reserveErr := reserveEnterpriseMemberSpendingLimits(ctx, reservationTx, fixture.memberID, 0, fixture.now)
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
		VALUES ($1, $2, 'manual_adjustment', 0, $3, '')`, fixture.memberID, fixture.periodStart, fixture.suffix)
	require.NoError(t, err)
	require.NoError(t, settlementTx.Commit())
	require.NoError(t, <-reservationDone)
	require.NoError(t, reservationTx.Commit())
}

func TestEnterpriseMemberBatchAdjustmentLockAllowsSettlementForeignKeyCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fixture := newEnterpriseMemberBudgetLockFixture(t, ctx, "member-batch-lock", 100)
	batchDB, err := sql.Open("postgres", integrationPostgresDSN)
	require.NoError(t, err)
	batchDB.SetMaxOpenConns(1)
	batchDB.SetMaxIdleConns(1)
	require.NoError(t, batchDB.PingContext(ctx))
	t.Cleanup(func() { _ = batchDB.Close() })

	var batchPID int
	require.NoError(t, batchDB.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&batchPID))

	settlementTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = settlementTx.Rollback() })
	var periodID int64
	require.NoError(t, settlementTx.QueryRowContext(ctx, `
		SELECT id
		FROM enterprise_member_budget_periods
		WHERE member_id = $1 AND period_start = $2
		FOR UPDATE`, fixture.memberID, fixture.periodStart).Scan(&periodID))
	require.NotZero(t, periodID)

	type batchResult struct {
		updates []service.BatchEnterpriseMemberUsageUpdate
		err     error
	}
	batchDone := make(chan batchResult, 1)
	go func() {
		updates, batchErr := NewEnterpriseMemberBudgetRepository(batchDB).BatchAdjustUsage(
			ctx,
			fixture.ownerID,
			fixture.periodStart,
			[]service.EnterpriseMemberBatchTarget{{ID: fixture.memberID, ExpectedVersion: 1}},
			service.EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
			fixture.ownerID,
			"batch:"+fixture.suffix,
			"batch lock compatibility test",
		)
		batchDone <- batchResult{updates: updates, err: batchErr}
	}()

	var batchWaitingQuery string
	require.Eventually(t, func() bool {
		var waitEventType *string
		queryErr := integrationDB.QueryRowContext(ctx, `
			SELECT wait_event_type, query
			FROM pg_stat_activity
			WHERE pid = $1`, batchPID).Scan(&waitEventType, &batchWaitingQuery)
		return queryErr == nil && waitEventType != nil && *waitEventType == "Lock"
	}, 3*time.Second, 10*time.Millisecond, "batch adjustment should wait on the budget period lock")
	require.Contains(t, batchWaitingQuery, "enterprise_member_budget_periods")

	_, err = settlementTx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, amount_usd, idempotency_key, note)
		VALUES ($1, $2, 'manual_adjustment', 0, $3, '')`, fixture.memberID, fixture.periodStart, "settlement:"+fixture.suffix)
	require.NoError(t, err)
	require.NoError(t, settlementTx.Commit())

	result := <-batchDone
	require.NoError(t, result.err)
	require.Equal(t, []service.BatchEnterpriseMemberUsageUpdate{{ID: fixture.memberID, MonthlyUsedUSD: 1}}, result.updates)
}

func TestEnterpriseMemberBudgetReservationsRemainSerializedByMemberLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fixture := newEnterpriseMemberBudgetLockFixture(t, ctx, "member-reservation-serialization", 10)
	firstTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = firstTx.Rollback() })
	_, eligible, err := reserveEnterpriseMemberSpendingLimits(ctx, firstTx, fixture.memberID, 7, fixture.now)
	require.NoError(t, err)
	require.True(t, eligible)

	secondTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = secondTx.Rollback() })
	var secondPID int
	require.NoError(t, secondTx.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&secondPID))

	secondDone := make(chan error, 1)
	go func() {
		_, _, reserveErr := reserveEnterpriseMemberSpendingLimits(ctx, secondTx, fixture.memberID, 7, fixture.now)
		secondDone <- reserveErr
	}()

	var secondWaitingQuery string
	require.Eventually(t, func() bool {
		var waitEventType *string
		queryErr := integrationDB.QueryRowContext(ctx, `
			SELECT wait_event_type, query
			FROM pg_stat_activity
			WHERE pid = $1`, secondPID).Scan(&waitEventType, &secondWaitingQuery)
		return queryErr == nil && waitEventType != nil && *waitEventType == "Lock"
	}, 3*time.Second, 10*time.Millisecond, "second reservation should wait on the member mutex")
	require.Contains(t, secondWaitingQuery, "enterprise_members")

	require.NoError(t, firstTx.Commit())
	require.ErrorIs(t, <-secondDone, service.ErrEnterpriseMemberAsyncBudgetUnavailable)
	require.NoError(t, secondTx.Rollback())

	var reserved float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT reserved_usd
		FROM enterprise_member_budget_periods
		WHERE member_id = $1 AND period_start = $2`, fixture.memberID, fixture.periodStart).Scan(&reserved))
	require.Equal(t, 7.0, reserved)
}

func TestEnterpriseMemberAdoptKeyLockAllowsSettlementForeignKeyCheck(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fixture := newEnterpriseMemberBudgetLockFixture(t, ctx, "member-adopt-key-lock", 100)
	apiKey, err := integrationEntClient.APIKey.Create().
		SetUserID(fixture.ownerID).
		SetKey("sk-" + fixture.suffix).
		SetName(fixture.suffix).
		SetStatus(service.StatusActive).
		SetMemberID(fixture.memberID).
		Save(ctx)
	require.NoError(t, err)

	adoptDB, err := sql.Open("postgres", integrationPostgresDSN)
	require.NoError(t, err)
	adoptDB.SetMaxOpenConns(1)
	adoptDB.SetMaxIdleConns(1)
	require.NoError(t, adoptDB.PingContext(ctx))
	t.Cleanup(func() { _ = adoptDB.Close() })

	var adoptPID int
	require.NoError(t, adoptDB.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&adoptPID))

	settlementTx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = settlementTx.Rollback() })
	_, err = settlementTx.ExecContext(ctx, `
		UPDATE api_keys
		SET quota_used = quota_used + 1, updated_at = NOW()
		WHERE id = $1`, apiKey.ID)
	require.NoError(t, err)

	adoptDone := make(chan error, 1)
	go func() {
		_, adoptErr := (&enterpriseMemberRepository{db: adoptDB}).AdoptKey(
			ctx, fixture.ownerID, fixture.memberID, apiKey.ID, 1,
		)
		adoptDone <- adoptErr
	}()

	var adoptWaitingQuery string
	require.Eventually(t, func() bool {
		var waitEventType *string
		queryErr := integrationDB.QueryRowContext(ctx, `
			SELECT wait_event_type, query
			FROM pg_stat_activity
			WHERE pid = $1`, adoptPID).Scan(&waitEventType, &adoptWaitingQuery)
		return queryErr == nil && waitEventType != nil && *waitEventType == "Lock"
	}, 3*time.Second, 10*time.Millisecond, "key adoption should wait on the API key row")
	require.Contains(t, adoptWaitingQuery, "FROM api_keys")

	_, err = settlementTx.ExecContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, request_id, amount_usd, idempotency_key, note)
		VALUES ($1, $2, 'usage', $3, 0, $4, '')`,
		fixture.memberID, fixture.periodStart, "settlement:"+fixture.suffix, "usage:"+fixture.suffix)
	require.NoError(t, err)
	require.NoError(t, settlementTx.Commit())
	require.ErrorIs(t, <-adoptDone, service.ErrEnterpriseMemberKeyNotAdoptable)
}
