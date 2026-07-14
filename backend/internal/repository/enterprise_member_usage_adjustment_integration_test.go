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

func TestEnterpriseMemberBatchUpdateCanChangeStatusWithoutChangingLimits(t *testing.T) {
	ctx := context.Background()
	suffix := fmt.Sprintf("member-batch-status-%d", time.Now().UnixNano())
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

	repo := NewEnterpriseMemberRepository(integrationEntClient, integrationDB)
	members := make([]service.EnterpriseMember, 2)
	for i := range members {
		members[i] = service.EnterpriseMember{
			EnterpriseUserID: owner.ID,
			MemberCode:       fmt.Sprintf("batch-status-%d", i+1),
			Name:             fmt.Sprintf("Batch Status %d", i+1),
			Status:           service.EnterpriseMemberStatusDisabled,
			MonthlyLimitUSD:  100 + float64(i),
			RateLimit5h:      10 + float64(i),
			RateLimit1d:      20 + float64(i),
			RateLimit7d:      30 + float64(i),
		}
		require.NoError(t, repo.Create(ctx, &members[i], []int64{group.ID}, service.EnterpriseMemberOpeningUsage{}))
	}

	active := service.EnterpriseMemberStatusActive
	updated, err := repo.BatchUpdate(ctx, owner.ID, []service.EnterpriseMemberBatchTarget{
		{ID: members[0].ID, ExpectedVersion: members[0].Version},
		{ID: members[1].ID, ExpectedVersion: members[1].Version},
	}, service.BatchEnterpriseMemberPolicyPatch{
		Status:    &active,
		GroupMode: "keep",
	})
	require.NoError(t, err)
	require.Len(t, updated, 2)
	for i := range updated {
		require.Equal(t, service.EnterpriseMemberStatusActive, updated[i].Status)
		require.Equal(t, members[i].MonthlyLimitUSD, updated[i].MonthlyLimitUSD)
		require.Equal(t, members[i].RateLimit5h, updated[i].RateLimit5h)
		require.Equal(t, members[i].RateLimit1d, updated[i].RateLimit1d)
		require.Equal(t, members[i].RateLimit7d, updated[i].RateLimit7d)
		require.Equal(t, []int64{group.ID}, updated[i].GroupIDs)
	}
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

func TestEnterpriseMemberBatchAdjustUsageNormalizesExpiredWindowsAndReplays(t *testing.T) {
	ctx := context.Background()
	suffix := fmt.Sprintf("usage-batch-expired-%d", time.Now().UnixNano())
	owner, err := integrationEntClient.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("integration-test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)
	member, err := integrationEntClient.EnterpriseMember.Create().
		SetEnterpriseUserID(owner.ID).
		SetMemberCode("usage-batch-expired").
		SetName("Usage Batch Expired").
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

	location, err := time.LoadLocation(service.EnterpriseMemberBudgetTimezone)
	require.NoError(t, err)
	now := time.Now().In(location)
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	repo := NewEnterpriseMemberBudgetRepository(integrationDB)
	require.NoError(t, repo.SetUsage(ctx, owner.ID, member.ID, periodStart, 30, 80, 10, 20, owner.ID, "opening:"+integrationHash(suffix), "test opening"))
	_, err = integrationDB.ExecContext(ctx, `
		UPDATE enterprise_member_rate_limit_periods
		SET window_5h_start = NOW() - INTERVAL '6 hours',
		    window_1d_start = NOW(),
		    window_7d_start = NOW()
		WHERE member_id = $1`, member.ID)
	require.NoError(t, err)

	targets := []service.EnterpriseMemberBatchTarget{{ID: member.ID, ExpectedVersion: member.Version}}
	delta := service.EnterpriseMemberUsageDelta{MonthlyUsedUSD: 2, Usage5h: 10, Usage1d: 5}
	batchKey := "usage-batch:" + integrationHash(suffix)
	updated, err := repo.BatchAdjustUsage(ctx, owner.ID, periodStart, targets, delta, owner.ID, batchKey, "batch test")
	require.NoError(t, err)
	require.Equal(t, []service.BatchEnterpriseMemberUsageUpdate{{
		ID: member.ID, MonthlyUsedUSD: 32, Usage5h: 10, Usage1d: 15, Usage7d: 20,
	}}, updated)

	replayed, err := repo.BatchAdjustUsage(ctx, owner.ID, periodStart, targets, delta, owner.ID, batchKey, "batch test")
	require.NoError(t, err)
	require.Equal(t, updated, replayed)

	var batchAuditCount, batchLedgerCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_audit_logs
		WHERE member_id = $1 AND action = 'member.usage_adjusted'
		  AND metadata->>'batch_idempotency_key' = $2`, member.ID, batchKey).Scan(&batchAuditCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_budget_entries
		WHERE member_id = $1 AND idempotency_key = $2`, member.ID, batchKey+":"+fmt.Sprint(member.ID)).Scan(&batchLedgerCount))
	require.Equal(t, 1, batchAuditCount)
	require.Equal(t, 1, batchLedgerCount)
}

func TestEnterpriseMemberBatchAdjustUsageRollsBackEveryMemberOnLateValidationFailure(t *testing.T) {
	ctx := context.Background()
	suffix := fmt.Sprintf("usage-batch-rollback-%d", time.Now().UnixNano())
	owner, err := integrationEntClient.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("integration-test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)

	members := make([]*service.EnterpriseMember, 2)
	memberRepo := NewEnterpriseMemberRepository(integrationEntClient, integrationDB)
	for index := range members {
		members[index] = &service.EnterpriseMember{
			EnterpriseUserID: owner.ID,
			MemberCode:       fmt.Sprintf("usage-batch-rollback-%d", index+1),
			Name:             fmt.Sprintf("Usage Batch Rollback %d", index+1),
			Status:           service.EnterpriseMemberStatusDisabled,
			MonthlyLimitUSD:  100,
		}
		require.NoError(t, memberRepo.Create(ctx, members[index], nil, service.EnterpriseMemberOpeningUsage{}))
	}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, `
			TRUNCATE TABLE enterprise_member_audit_logs, enterprise_member_import_jobs, enterprise_members CASCADE`)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, owner.ID)
		require.NoError(t, cleanupErr)
	})

	location, err := time.LoadLocation(service.EnterpriseMemberBudgetTimezone)
	require.NoError(t, err)
	now := time.Now().In(location)
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	repo := NewEnterpriseMemberBudgetRepository(integrationDB)
	require.NoError(t, repo.SetUsage(ctx, owner.ID, members[0].ID, periodStart, 10, 0, 0, 0, owner.ID, "opening:first:"+integrationHash(suffix), "test opening"))
	require.NoError(t, repo.SetUsage(ctx, owner.ID, members[1].ID, periodStart, 0, 0, 0, 0, owner.ID, "opening:second:"+integrationHash(suffix), "test opening"))

	batchKey := "usage-batch:" + integrationHash(suffix)
	_, err = repo.BatchAdjustUsage(ctx, owner.ID, periodStart, []service.EnterpriseMemberBatchTarget{
		{ID: members[0].ID, ExpectedVersion: members[0].Version},
		{ID: members[1].ID, ExpectedVersion: members[1].Version},
	}, service.EnterpriseMemberUsageDelta{MonthlyUsedUSD: -5}, owner.ID, batchKey, "batch rollback test")
	require.ErrorIs(t, err, service.ErrEnterpriseMemberInvalid)

	var firstUsed, secondUsed float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT used_usd FROM enterprise_member_budget_periods WHERE member_id = $1 AND period_start = $2`, members[0].ID, periodStart).Scan(&firstUsed))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT used_usd FROM enterprise_member_budget_periods WHERE member_id = $1 AND period_start = $2`, members[1].ID, periodStart).Scan(&secondUsed))
	require.Equal(t, 10.0, firstUsed)
	require.Zero(t, secondUsed)

	var batchAuditCount, batchLedgerCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_audit_logs
		WHERE metadata->>'batch_idempotency_key' = $1`, batchKey).Scan(&batchAuditCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_budget_entries
		WHERE idempotency_key LIKE $1`, batchKey+":%").Scan(&batchLedgerCount))
	require.Zero(t, batchAuditCount)
	require.Zero(t, batchLedgerCount)
}
