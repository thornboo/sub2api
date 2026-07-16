package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberImportQueueCommitReturnsCommittedStateWithoutPostCommitRead(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberImportRepository{db: db}
	expiresAt := time.Now().Add(time.Hour)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, expires_at, idempotency_key_hash, selected_rows, default_group_ids, activate_members, import_policy_version FROM enterprise_member_import_jobs").
		WithArgs(int64(41), int64(7), "preview-token-hash").
		WillReturnRows(sqlmock.NewRows([]string{"status", "expires_at", "idempotency_key_hash", "selected_rows", "default_group_ids", "activate_members", "import_policy_version"}).
			AddRow("previewed", expiresAt, nil, []byte("[]"), []byte("[]"), false, service.EnterpriseMemberImportPolicyExplicitActivation))
	mock.ExpectExec("UPDATE enterprise_member_import_jobs SET status = \\$1").
		WithArgs(service.EnterpriseMemberImportStatusQueuedV2, service.EnterpriseMemberImportCommitProtocolPolicyV2, []byte("[2,4]"), []byte("[9,3]"), true, "commit-key-hash", int64(41)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	job, err := repo.QueueCommit(context.Background(), 7, 41, "preview-token-hash", []int{2, 4}, []int64{9, 3}, true, "commit-key-hash")
	require.NoError(t, err)
	require.Equal(t, int64(41), job.ID)
	require.Equal(t, int64(7), job.EnterpriseUserID)
	require.Equal(t, service.EnterpriseMemberImportStatusQueuedV2, job.Status)
	require.NoError(t, mock.ExpectationsWereMet(), "a committed queue operation must not perform a fallible post-commit read")
}

func TestEnterpriseMemberImportQueueCommitReplaysSameCommittedRequestWithoutPostCommitRead(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberImportRepository{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, expires_at, idempotency_key_hash, selected_rows, default_group_ids, activate_members, import_policy_version FROM enterprise_member_import_jobs").
		WithArgs(int64(41), int64(7), "preview-token-hash").
		WillReturnRows(sqlmock.NewRows([]string{"status", "expires_at", "idempotency_key_hash", "selected_rows", "default_group_ids", "activate_members", "import_policy_version"}).
			AddRow(service.EnterpriseMemberImportStatusQueuedV2, time.Now().Add(time.Hour), "commit-key-hash", []byte("[2,4]"), []byte("[9,3]"), true, service.EnterpriseMemberImportPolicyExplicitActivation))
	mock.ExpectCommit()

	job, err := repo.QueueCommit(context.Background(), 7, 41, "preview-token-hash", []int{2, 4}, []int64{9, 3}, true, "commit-key-hash")
	require.NoError(t, err)
	require.Equal(t, service.EnterpriseMemberImportStatusQueuedV2, job.Status)
	require.NoError(t, mock.ExpectationsWereMet(), "same-key retries must attach to the durable job without another read")
}

func TestEnterpriseMemberBatchReplaceGroupsReturnsTransactionResultWithoutPostCommitRead(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberRepository{db: db}
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectQuery("UPDATE enterprise_members").
		WithArgs(int64(12), int64(7), int64(3), true).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "updated_at"}).AddRow(int64(4), service.EnterpriseMemberStatusDisabled, now))
	mock.ExpectExec("DELETE FROM enterprise_member_group_bindings").
		WithArgs(int64(12)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()

	updated, err := repo.BatchReplaceGroups(context.Background(), 7, []service.BatchEnterpriseMemberGroupTarget{{
		ID: 12, ExpectedVersion: 3, GroupIDs: nil,
	}})
	require.NoError(t, err)
	require.Equal(t, []service.BatchEnterpriseMemberGroupUpdate{{
		ID: 12, Version: 4, GroupIDs: []int64{}, Status: service.EnterpriseMemberStatusDisabled, UpdatedAt: now,
	}}, updated)
	require.NoError(t, mock.ExpectationsWereMet(), "authorization updates must not depend on a post-commit list query")
}

func TestEnterpriseMemberBatchUpdateReturnsTransactionResultWithoutPostCommitRead(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberRepository{db: db}
	now := time.Now()
	limit := 100.0
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, version FROM enterprise_members").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "version"}).AddRow(service.EnterpriseMemberStatusActive, int64(3)))
	mock.ExpectQuery(`SELECT group_id\s+FROM enterprise_member_group_bindings\s+WHERE member_id = \$1\s+ORDER BY sort_order, group_id`).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(9)))
	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM groups`).
		WithArgs(int64(7), pq.Array([]int64{9})).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("UPDATE enterprise_members").
		WithArgs(int64(12), int64(7), int64(3), limit, nil, nil, nil, service.EnterpriseMemberStatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "updated_at"}).
			AddRow(int64(4), service.EnterpriseMemberStatusActive, limit, 0.0, 0.0, 0.0, now))
	mock.ExpectCommit()

	updated, err := repo.BatchUpdate(context.Background(), 7, []service.EnterpriseMemberBatchTarget{{
		ID: 12, ExpectedVersion: 3,
	}}, service.BatchEnterpriseMemberPolicyPatch{MonthlyLimitUSD: &limit, GroupMode: "keep"})

	require.NoError(t, err)
	require.Equal(t, []service.BatchEnterpriseMemberUpdate{{
		ID: 12, Version: 4, Status: service.EnterpriseMemberStatusActive, MonthlyLimitUSD: limit,
		GroupIDs: []int64{9}, UpdatedAt: now,
	}}, updated)
	require.NoError(t, mock.ExpectationsWereMet(), "batch policy updates must return the committed transaction state")
}

func TestEnterpriseMemberBatchUpdateRollsBackEarlierRowsOnVersionConflict(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberRepository{db: db}
	limit := 100.0
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, version FROM enterprise_members").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "version"}).AddRow(service.EnterpriseMemberStatusActive, int64(3)))
	mock.ExpectQuery(`SELECT group_id\s+FROM enterprise_member_group_bindings\s+WHERE member_id = \$1\s+ORDER BY sort_order, group_id`).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(9)))
	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM groups`).
		WithArgs(int64(7), pq.Array([]int64{9})).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("UPDATE enterprise_members").
		WithArgs(int64(12), int64(7), int64(3), limit, nil, nil, nil, service.EnterpriseMemberStatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"version", "status", "monthly_limit_usd", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d", "updated_at"}).
			AddRow(int64(4), service.EnterpriseMemberStatusActive, limit, 0.0, 0.0, 0.0, time.Now()))
	mock.ExpectQuery("SELECT status, version FROM enterprise_members").
		WithArgs(int64(24), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "version"}).AddRow(service.EnterpriseMemberStatusActive, int64(9)))
	mock.ExpectRollback()

	_, err = repo.BatchUpdate(context.Background(), 7, []service.EnterpriseMemberBatchTarget{
		{ID: 24, ExpectedVersion: 4},
		{ID: 12, ExpectedVersion: 3},
	}, service.BatchEnterpriseMemberPolicyPatch{MonthlyLimitUSD: &limit, GroupMode: "keep"})

	require.ErrorIs(t, err, service.ErrEnterpriseMemberVersion)
	require.NoError(t, mock.ExpectationsWereMet(), "a later optimistic-lock conflict must roll back earlier member updates")
}

func TestEnterpriseMemberBatchUpdateRejectsActiveMemberWithUnauthorizedExistingGroup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &enterpriseMemberRepository{db: db}
	limit := 100.0
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT status, version FROM enterprise_members").
		WithArgs(int64(12), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "version"}).AddRow(service.EnterpriseMemberStatusActive, int64(3)))
	mock.ExpectQuery(`SELECT group_id\s+FROM enterprise_member_group_bindings\s+WHERE member_id = \$1\s+ORDER BY sort_order, group_id`).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(9)))
	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM groups`).
		WithArgs(int64(7), pq.Array([]int64{9})).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()

	_, err = repo.BatchUpdate(context.Background(), 7, []service.EnterpriseMemberBatchTarget{{
		ID: 12, ExpectedVersion: 3,
	}}, service.BatchEnterpriseMemberPolicyPatch{MonthlyLimitUSD: &limit, GroupMode: "keep"})

	require.ErrorIs(t, err, service.ErrGroupNotAllowed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberImportEffectiveTokenTotalMatchesPersistedSummary(t *testing.T) {
	row := service.EnterpriseMemberImportRow{
		InputTokens: mustEnterpriseMemberTokenCount(t, "50"), OutputTokens: mustEnterpriseMemberTokenCount(t, "30"),
		CacheTokens: mustEnterpriseMemberTokenCount(t, "20"),
	}
	require.Equal(t, "80.00", enterpriseMemberImportSummaryTokens(row).String())

	row.TotalTokens = mustEnterpriseMemberTokenCount(t, "100")
	require.Equal(t, "100.00", enterpriseMemberImportSummaryTokens(row).String())

	row.TotalTokens = service.EnterpriseMemberTokenCount{}
	row.TotalTokensProvided = true
	require.True(t, enterpriseMemberImportSummaryTokens(row).IsZero(), "an explicit source zero must remain distinct from an omitted total")
}

func TestEnterpriseMemberImportPolicyPreservesLegacyActivationOnly(t *testing.T) {
	legacy := &service.EnterpriseMemberImportJob{ImportPolicyVersion: service.EnterpriseMemberImportPolicyLegacyAutoActivate}
	currentDisabled := &service.EnterpriseMemberImportJob{ImportPolicyVersion: service.EnterpriseMemberImportPolicyExplicitActivation}
	currentEnabled := &service.EnterpriseMemberImportJob{ImportPolicyVersion: service.EnterpriseMemberImportPolicyExplicitActivation, ActivateMembers: true}

	require.True(t, enterpriseMemberImportShouldActivate(legacy, []int64{9}))
	require.False(t, enterpriseMemberImportShouldActivate(currentDisabled, []int64{9}))
	require.True(t, enterpriseMemberImportShouldActivate(currentEnabled, []int64{9}))
	require.False(t, enterpriseMemberImportShouldActivate(legacy, nil))
}

func TestEnterpriseMemberImportEffectiveGroupsSeparateLegacyRowsFromPolicyV2(t *testing.T) {
	row := service.EnterpriseMemberImportRow{GroupIDs: []int64{5, 6}}

	legacy := &service.EnterpriseMemberImportJob{
		ImportPolicyVersion: service.EnterpriseMemberImportPolicyLegacyAutoActivate,
		DefaultGroupIDs:     []int64{1},
	}
	require.Equal(t, []int64{5, 6}, enterpriseMemberImportEffectiveGroups(legacy, row),
		"legacy jobs must continue to use the row-level groups accepted by old clients")

	currentReplace := &service.EnterpriseMemberImportJob{
		ImportPolicyVersion: service.EnterpriseMemberImportPolicyExplicitActivation,
		DefaultGroupIDs:     []int64{1, 2},
	}
	require.Equal(t, []int64{1, 2}, enterpriseMemberImportEffectiveGroups(currentReplace, row),
		"the owner-selected policy-v2 groups are authoritative for the whole import")

	currentNoAccess := &service.EnterpriseMemberImportJob{
		ImportPolicyVersion: service.EnterpriseMemberImportPolicyExplicitActivation,
		DefaultGroupIDs:     []int64{},
	}
	require.Empty(t, enterpriseMemberImportEffectiveGroups(currentNoAccess, row),
		"an empty policy-v2 selection means no authorization, not fallback to legacy row groups")
}

func TestEnterpriseMemberImportWriteErrorClassificationPreservesInfrastructureFailures(t *testing.T) {
	uniqueViolation := &pq.Error{Code: "23505", Constraint: "enterprise_members_owner_code_ci_unique"}
	require.ErrorIs(t, classifyEnterpriseMemberImportWriteError("insert member", uniqueViolation), service.ErrEnterpriseMemberImportConflict)

	foreignKeyViolation := &pq.Error{Code: "23503", Constraint: "enterprise_member_group_bindings_group_id_fkey"}
	require.ErrorIs(t, classifyEnterpriseMemberImportWriteError("insert group binding", foreignKeyViolation), service.ErrEnterpriseMemberImportConflict)

	unknownForeignKeyViolation := &pq.Error{Code: "23503", Constraint: "api_keys_member_owner_fk"}
	classified := classifyEnterpriseMemberImportWriteError("insert key", unknownForeignKeyViolation)
	require.False(t, errors.Is(classified, service.ErrEnterpriseMemberImportConflict),
		"unexpected foreign-key failures are internal invariant errors, not user-recoverable import races")
	require.ErrorIs(t, classified, unknownForeignKeyViolation)

	checkViolation := &pq.Error{Code: "23514", Constraint: "enterprise_members_limits_check"}
	classified = classifyEnterpriseMemberImportWriteError("insert member", checkViolation)
	require.False(t, errors.Is(classified, service.ErrEnterpriseMemberImportConflict),
		"unexpected database invariants must remain diagnosable instead of being flattened into a user conflict")
	require.ErrorIs(t, classified, checkViolation)

	classified = classifyEnterpriseMemberImportWriteError("insert member", context.DeadlineExceeded)
	require.ErrorIs(t, classified, context.DeadlineExceeded)
	require.False(t, errors.Is(classified, service.ErrEnterpriseMemberImportConflict))
}
