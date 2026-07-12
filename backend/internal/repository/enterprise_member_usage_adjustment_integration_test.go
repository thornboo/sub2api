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

func TestEnterpriseMemberCreatePersistsOpeningUsageAtomically(t *testing.T) {
	ctx := context.Background()
	suffix := fmt.Sprintf("member-opening-%d", time.Now().UnixNano())
	owner, err := integrationEntClient.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("integration-test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)
	group := mustCreateGroup(t, integrationEntClient, &service.Group{
		Name:           suffix + "-group",
		RateMultiplier: 1,
	})

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, `
			TRUNCATE TABLE enterprise_member_audit_logs, enterprise_member_import_jobs, enterprise_members CASCADE`)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM groups WHERE id = $1`, group.ID)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, owner.ID)
		require.NoError(t, cleanupErr)
	})

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	idempotencyKey := "member-opening:" + integrationHash(suffix)
	member := &service.EnterpriseMember{
		EnterpriseUserID: owner.ID,
		MemberCode:       "opening-member",
		Name:             "Opening Member",
		Status:           service.EnterpriseMemberStatusActive,
		MonthlyLimitUSD:  100,
		RateLimit5h:      25,
		RateLimit1d:      50,
		RateLimit7d:      75,
	}
	repo := NewEnterpriseMemberRepository(integrationEntClient, integrationDB)
	require.NoError(t, repo.Create(ctx, member, []int64{group.ID}, service.EnterpriseMemberOpeningUsage{
		PeriodStart:    periodStart,
		MonthlyUsedUSD: 30,
		Usage5h:        5,
		Usage1d:        10,
		Usage7d:        20,
		ActorUserID:    owner.ID,
		IdempotencyKey: idempotencyKey,
		Note:           "usage imported from finance system",
	}))
	require.NotZero(t, member.ID)
	require.Equal(t, 5.0, member.Usage5h)
	require.Equal(t, 10.0, member.Usage1d)
	require.Equal(t, 20.0, member.Usage7d)
	require.Equal(t, []int64{group.ID}, member.GroupIDs)

	var monthlyUsed, usage5h, usage1d, usage7d float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT p.used_usd, r.usage_5h, r.usage_1d, r.usage_7d
		FROM enterprise_member_budget_periods p
		JOIN enterprise_member_rate_limit_periods r ON r.member_id = p.member_id
		WHERE p.member_id = $1 AND p.period_start = $2`, member.ID, periodStart).
		Scan(&monthlyUsed, &usage5h, &usage1d, &usage7d))
	require.Equal(t, 30.0, monthlyUsed)
	require.Equal(t, 5.0, usage5h)
	require.Equal(t, 10.0, usage1d)
	require.Equal(t, 20.0, usage7d)
	var bindingOrder int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT sort_order FROM enterprise_member_group_bindings
		WHERE member_id = $1 AND group_id = $2`, member.ID, group.ID).Scan(&bindingOrder))
	require.Zero(t, bindingOrder)

	var kind, ledgerNote, auditSource, auditNote string
	var ledgerAmount, auditedMonthly float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT kind, amount_usd, note
		FROM enterprise_member_budget_entries
		WHERE member_id = $1 AND idempotency_key = $2`, member.ID, idempotencyKey).
		Scan(&kind, &ledgerAmount, &ledgerNote))
	require.Equal(t, "migration_opening", kind)
	require.Equal(t, 30.0, ledgerAmount)
	require.Equal(t, "usage imported from finance system", ledgerNote)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT (after_data->>'monthly_used_usd')::numeric,
		       metadata->>'source', metadata->>'note'
		FROM enterprise_member_audit_logs
		WHERE member_id = $1 AND action = 'member.usage_adjusted'`, member.ID).
		Scan(&auditedMonthly, &auditSource, &auditNote))
	require.Equal(t, 30.0, auditedMonthly)
	require.Equal(t, "member_create", auditSource)
	require.Equal(t, "usage imported from finance system", auditNote)

	// A late opening-ledger conflict must roll back the member row as well.
	conflicting := &service.EnterpriseMember{
		EnterpriseUserID: owner.ID,
		MemberCode:       "opening-member-conflict",
		Name:             "Opening Member Conflict",
		Status:           service.EnterpriseMemberStatusActive,
		MonthlyLimitUSD:  100,
	}
	require.Error(t, repo.Create(ctx, conflicting, nil, service.EnterpriseMemberOpeningUsage{
		PeriodStart:    periodStart,
		MonthlyUsedUSD: 1,
		ActorUserID:    owner.ID,
		IdempotencyKey: idempotencyKey,
		Note:           "conflicting opening",
	}))
	var conflictingCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_members
		WHERE enterprise_user_id = $1 AND member_code = 'opening-member-conflict'`, owner.ID).
		Scan(&conflictingCount))
	require.Zero(t, conflictingCount)
}

func TestEnterpriseMemberSetUsagePersistsTypedAuditEvidence(t *testing.T) {
	ctx := context.Background()
	suffix := fmt.Sprintf("usage-adjustment-%d", time.Now().UnixNano())
	owner, err := integrationEntClient.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("integration-test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)

	member, err := integrationEntClient.EnterpriseMember.Create().
		SetEnterpriseUserID(owner.ID).
		SetMemberCode("usage-adjustment-member").
		SetName("Usage Adjustment Member").
		SetStatus(service.EnterpriseMemberStatusActive).
		SetMonthlyLimitUsd(100).
		Save(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, `
			TRUNCATE TABLE enterprise_member_audit_logs, enterprise_member_import_jobs, enterprise_members CASCADE`)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, owner.ID)
		require.NoError(t, cleanupErr)
	})

	periodStart := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	idempotencyKey := "usage-adjustment:" + integrationHash(suffix)
	repo := NewEnterpriseMemberBudgetRepository(integrationDB)
	require.NoError(t, repo.SetUsage(
		ctx, owner.ID, member.ID, periodStart,
		30, 1.25, 2.5, 3.75,
		owner.ID, idempotencyKey, "correct imported opening balance",
	))

	var monthlyUsed, usage5h, usage1d, usage7d float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT p.used_usd, r.usage_5h, r.usage_1d, r.usage_7d
		FROM enterprise_member_budget_periods p
		JOIN enterprise_member_rate_limit_periods r ON r.member_id = p.member_id
		WHERE p.member_id = $1 AND p.period_start = $2`, member.ID, periodStart).
		Scan(&monthlyUsed, &usage5h, &usage1d, &usage7d))
	require.Equal(t, 30.0, monthlyUsed)
	require.Equal(t, 1.25, usage5h)
	require.Equal(t, 2.5, usage1d)
	require.Equal(t, 3.75, usage7d)

	var afterMonthly, after5h, after1d, after7d float64
	var note string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT (after_data->>'monthly_used_usd')::numeric,
		       (after_data->>'usage_5h')::numeric,
		       (after_data->>'usage_1d')::numeric,
		       (after_data->>'usage_7d')::numeric,
		       metadata->>'note'
		FROM enterprise_member_audit_logs
		WHERE member_id = $1 AND action = 'member.usage_adjusted'`, member.ID).
		Scan(&afterMonthly, &after5h, &after1d, &after7d, &note))
	require.Equal(t, monthlyUsed, afterMonthly)
	require.Equal(t, usage5h, after5h)
	require.Equal(t, usage1d, after1d)
	require.Equal(t, usage7d, after7d)
	require.Equal(t, "correct imported opening balance", note)

	// The same key and payload must replay without a second ledger or audit write.
	require.NoError(t, repo.SetUsage(
		ctx, owner.ID, member.ID, periodStart,
		30, 1.25, 2.5, 3.75,
		owner.ID, idempotencyKey, "correct imported opening balance",
	))
	var auditCount, ledgerCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_audit_logs
		WHERE member_id = $1 AND action = 'member.usage_adjusted'`, member.ID).Scan(&auditCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_budget_entries
		WHERE member_id = $1 AND idempotency_key = $2`, member.ID, idempotencyKey).Scan(&ledgerCount))
	require.Equal(t, 1, auditCount)
	require.Equal(t, 1, ledgerCount)
}
