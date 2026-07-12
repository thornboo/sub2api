//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberImportClaimIsUniqueAcrossWorkers(t *testing.T) {
	ctx := context.Background()
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	job := createCommittedImportQueueFixture(t, ctx, "queued", nil, nil, 0)

	start := make(chan struct{})
	results := make(chan *service.EnterpriseMemberImportJob, 2)
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	for _, workerID := range []string{"worker-a", "worker-b"} {
		workers.Add(1)
		go func(id string) {
			defer workers.Done()
			<-start
			claimed, err := repo.ClaimNextCommitJob(ctx, id, 3*time.Minute)
			results <- claimed
			errs <- err
		}(workerID)
	}
	close(start)
	workers.Wait()
	close(results)
	close(errs)

	var claimed []*service.EnterpriseMemberImportJob
	for result := range results {
		if result != nil {
			claimed = append(claimed, result)
		}
	}
	var emptyClaims int
	for err := range errs {
		if err == nil {
			continue
		}
		require.ErrorIs(t, err, service.ErrEnterpriseMemberImportQueueEmpty)
		emptyClaims++
	}

	require.Len(t, claimed, 1, "only one worker may claim a queued job")
	require.Equal(t, job.ID, claimed[0].ID)
	require.NotNil(t, claimed[0].LockOwner)
	require.Contains(t, []string{"worker-a", "worker-b"}, *claimed[0].LockOwner)
	require.Equal(t, 1, claimed[0].AttemptCount)
	require.Equal(t, 1, emptyClaims)

	var status, lockOwner string
	var attemptCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, lock_owner, attempt_count
		FROM enterprise_member_import_jobs WHERE id = $1`, job.ID).
		Scan(&status, &lockOwner, &attemptCount))
	require.Equal(t, "processing", status)
	require.Equal(t, *claimed[0].LockOwner, lockOwner)
	require.Equal(t, 1, attemptCount)
}

func TestEnterpriseMemberImportLeaseTakeoverFencesStaleWorker(t *testing.T) {
	ctx := context.Background()
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	job := createCommittedImportQueueFixture(t, ctx, "queued", nil, nil, 0)

	staleWorkerJob, err := repo.ClaimNextCommitJob(ctx, "worker-stale", 3*time.Minute)
	require.NoError(t, err)
	require.NoError(t, setImportJobLeaseTime(ctx, job.ID, time.Now().Add(-10*time.Minute)))

	replacementJob, err := repo.ClaimNextCommitJob(ctx, "worker-replacement", 3*time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, replacementJob.ID)
	require.NotNil(t, replacementJob.LockOwner)
	require.Equal(t, "worker-replacement", *replacementJob.LockOwner)
	require.Equal(t, 2, replacementJob.AttemptCount)

	idempotencyHash := *staleWorkerJob.IdempotencyKeyHash
	_, err = repo.Commit(ctx, staleWorkerJob, staleWorkerJob.Preview.Rows, nil, idempotencyHash, "")
	require.ErrorIs(t, err, service.ErrEnterpriseMemberImportConflict,
		"the old lease holder must be fenced before it writes imported members")

	var memberCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_members WHERE enterprise_user_id = $1`, job.EnterpriseUserID).
		Scan(&memberCount))
	require.Zero(t, memberCount)

	require.NoError(t, repo.MarkCommitFailed(ctx, job.ID, "worker-stale", "STALE_FAILURE", "late failure"))
	assertImportJobLease(t, ctx, job.ID, "processing", "worker-replacement")

	require.NoError(t, repo.MarkCommitFailed(ctx, job.ID, "worker-replacement", "COMMIT_FAILED", "replacement failed"))
	assertImportJobLease(t, ctx, job.ID, "failed", "")
}

func TestEnterpriseMemberImportClaimRecoversProcessingJobWithoutLeaseTimestamp(t *testing.T) {
	ctx := context.Background()
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	oldOwner := "worker-with-missing-timestamp"
	job := createCommittedImportQueueFixture(t, ctx, "processing", &oldOwner, nil, 1)

	claimed, err := repo.ClaimNextCommitJob(ctx, "worker-recovery", 3*time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, claimed.ID)
	require.NotNil(t, claimed.LockOwner)
	require.Equal(t, "worker-recovery", *claimed.LockOwner)
	require.Equal(t, 2, claimed.AttemptCount)
}

func TestEnterpriseMemberImportLeaseRenewalPreventsPrematureTakeover(t *testing.T) {
	ctx := context.Background()
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	job := createCommittedImportQueueFixture(t, ctx, "queued", nil, nil, 0)

	claimed, err := repo.ClaimNextCommitJob(ctx, "worker-renewing", time.Second)
	require.NoError(t, err)
	require.NoError(t, setImportJobLeaseTime(ctx, job.ID, time.Now().Add(-10*time.Second)))

	renewed, err := repo.RenewCommitLease(ctx, job.ID, "different-worker")
	require.NoError(t, err)
	require.False(t, renewed, "a non-owner must not be able to extend another worker's lease")

	renewed, err = repo.RenewCommitLease(ctx, job.ID, "worker-renewing")
	require.NoError(t, err)
	require.True(t, renewed)

	_, err = repo.ClaimNextCommitJob(ctx, "worker-takeover", time.Second)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberImportQueueEmpty,
		"a freshly renewed processing job must not be reclaimed")

	require.NoError(t, setImportJobLeaseTime(ctx, job.ID, time.Now().Add(-2*time.Second)))
	takenOver, err := repo.ClaimNextCommitJob(ctx, "worker-takeover", time.Second)
	require.NoError(t, err)
	require.Equal(t, claimed.ID, takenOver.ID)
	require.NotNil(t, takenOver.LockOwner)
	require.Equal(t, "worker-takeover", *takenOver.LockOwner)
}

func TestEnterpriseMemberImportCommitHandlesMaximum5000Rows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	job := createCommittedImportQueueFixtureWithRows(t, ctx, "queued", nil, nil, 0, 5000)

	claimed, err := repo.ClaimNextCommitJob(ctx, "worker-capacity", 3*time.Minute)
	require.NoError(t, err)
	require.Len(t, claimed.Preview.Rows, 5000)
	require.NotNil(t, claimed.IdempotencyKeyHash)

	startedAt := time.Now()
	result, err := repo.Commit(ctx, claimed, claimed.Preview.Rows, nil, *claimed.IdempotencyKeyHash, "")
	require.NoError(t, err)
	require.Equal(t, "completed", result.Status)
	require.Equal(t, 5000, result.CreatedMembers)
	require.Zero(t, result.CreatedKeys)
	t.Logf("committed 5000 enterprise members in %s", time.Since(startedAt))

	var members, audits int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enterprise_members WHERE enterprise_user_id = $1`, job.EnterpriseUserID).Scan(&members))
	require.Equal(t, 5000, members)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM enterprise_member_audit_logs
		WHERE enterprise_user_id = $1 AND action = 'member.created'`, job.EnterpriseUserID).Scan(&audits))
	require.Equal(t, 5000, audits, "maximum-size import must retain an audit fact for every member")
}

func TestEnterpriseMemberImportReferenceValidationRejectsSoftDeletedKeyReuse(t *testing.T) {
	ctx := context.Background()
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	job := createCommittedImportQueueFixture(t, ctx, "queued", nil, nil, 0)
	historicalKey := "sk-deleted-import-conflict-" + integrationHash(t.Name())[:16]

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, status, deleted_at)
		VALUES ($1, $2, 'historical deleted key', 'disabled', NOW())`, job.EnterpriseUserID, historicalKey)
	require.NoError(t, err)

	state, err := repo.ValidateReferences(ctx, job.EnterpriseUserID, nil, []string{historicalKey}, nil)
	require.NoError(t, err)
	require.True(t, state.ExistingKeys[historicalKey],
		"soft-deleted credentials remain historical facts and must not be reusable by import")
}

func TestEnterpriseMemberImportCommitConnectionLossRollsBackAndAllowsTakeover(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	job := createCommittedImportQueueFixtureWithRows(t, ctx, "queued", nil, nil, 0, 5000)

	isolatedDB, err := sql.Open("postgres", integrationPostgresDSN)
	require.NoError(t, err)
	isolatedDB.SetMaxOpenConns(1)
	isolatedDB.SetMaxIdleConns(1)
	require.NoError(t, isolatedDB.PingContext(ctx))
	t.Cleanup(func() { _ = isolatedDB.Close() })

	var backendPID int
	require.NoError(t, isolatedDB.QueryRowContext(ctx, `SELECT pg_backend_pid()`).Scan(&backendPID))
	isolatedRepo := NewEnterpriseMemberImportRepository(isolatedDB)
	claimed, err := isolatedRepo.ClaimNextCommitJob(ctx, "worker-connection-loss", 3*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, claimed.IdempotencyKeyHash)

	commitErr := make(chan error, 1)
	go func() {
		_, commitError := isolatedRepo.Commit(ctx, claimed, claimed.Preview.Rows, nil, *claimed.IdempotencyKeyHash, "")
		commitErr <- commitError
	}()

	require.Eventually(t, func() bool {
		var active bool
		queryErr := integrationDB.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE pid = $1 AND state = 'active' AND query LIKE '%INSERT INTO enterprise_members%'
			)`, backendPID).Scan(&active)
		return queryErr == nil && active
	}, 15*time.Second, 5*time.Millisecond, "the test must terminate the connection during member inserts")

	var terminated bool
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT pg_terminate_backend($1)`, backendPID).Scan(&terminated))
	require.True(t, terminated)
	require.Error(t, <-commitErr, "the interrupted transaction must report failure")

	var memberCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM enterprise_members WHERE enterprise_user_id = $1`, job.EnterpriseUserID).Scan(&memberCount))
	require.Zero(t, memberCount, "PostgreSQL connection loss must roll back every uncommitted member")
	assertImportJobLease(t, ctx, job.ID, "processing", "worker-connection-loss")

	require.NoError(t, setImportJobLeaseTime(ctx, job.ID, time.Now().Add(-10*time.Minute)))
	replacementRepo := NewEnterpriseMemberImportRepository(integrationDB)
	replacement, err := replacementRepo.ClaimNextCommitJob(ctx, "worker-after-connection-loss", 3*time.Minute)
	require.NoError(t, err)
	require.Equal(t, job.ID, replacement.ID)
	require.NotNil(t, replacement.LockOwner)
	require.Equal(t, "worker-after-connection-loss", *replacement.LockOwner)
	require.NoError(t, replacementRepo.MarkCommitFailed(ctx, job.ID, "worker-after-connection-loss", "INTERRUPTED", "connection interrupted during test"))
}

func createCommittedImportQueueFixture(
	t *testing.T,
	ctx context.Context,
	status string,
	lockOwner *string,
	lockedAt *time.Time,
	attemptCount int,
) *service.EnterpriseMemberImportJob {
	t.Helper()
	return createCommittedImportQueueFixtureWithRows(t, ctx, status, lockOwner, lockedAt, attemptCount, 1)
}

func createCommittedImportQueueFixtureWithRows(
	t *testing.T,
	ctx context.Context,
	status string,
	lockOwner *string,
	lockedAt *time.Time,
	attemptCount int,
	rowCount int,
) *service.EnterpriseMemberImportJob {
	t.Helper()

	suffix := fmt.Sprintf("%s-%d", sanitizeRedisNamespace(t.Name()), time.Now().UnixNano())
	owner, err := integrationEntClient.User.Create().
		SetEmail(suffix + "@example.com").
		SetPasswordHash("integration-test-password-hash").
		SetStatus(service.StatusActive).
		SetRole(service.RoleUser).
		Save(ctx)
	require.NoError(t, err)

	idempotencyHash := integrationHash("idempotency:" + suffix)
	previewRows := make([]service.EnterpriseMemberImportRow, 0, rowCount)
	selectedRows := make([]int, 0, rowCount)
	for row := 1; row <= rowCount; row++ {
		previewRows = append(previewRows, service.EnterpriseMemberImportRow{
			RowNumber: row, MemberCode: fmt.Sprintf("member-%04d", row), MemberName: fmt.Sprintf("Member %04d", row), MonthlyLimitUSD: 100, Valid: true,
		})
		selectedRows = append(selectedRows, row)
	}
	job := &service.EnterpriseMemberImportJob{
		EnterpriseUserID:   owner.ID,
		TokenHash:          integrationHash("token:" + suffix),
		FileHash:           integrationHash("file:" + suffix),
		Format:             "csv",
		Preview:            service.EnterpriseMemberImportPreview{Rows: previewRows},
		VersionFingerprint: map[string]int64{},
		ExpiresAt:          time.Now().Add(time.Hour),
	}
	repo := NewEnterpriseMemberImportRepository(integrationDB)
	require.NoError(t, repo.CreatePreviewJob(ctx, job))
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, cleanupErr := integrationDB.ExecContext(cleanupCtx, `
			TRUNCATE TABLE enterprise_member_audit_logs, enterprise_member_import_jobs, enterprise_members CASCADE`)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM api_keys WHERE user_id = $1`, owner.ID)
		require.NoError(t, cleanupErr)
		_, cleanupErr = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id = $1`, owner.ID)
		require.NoError(t, cleanupErr)
	})

	var startedAt *time.Time
	if status == "processing" {
		value := time.Now().Add(-15 * time.Minute)
		startedAt = &value
	}
	selectedRowsJSON, err := json.Marshal(selectedRows)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		UPDATE enterprise_member_import_jobs
		SET status = $1, selected_rows = $2,
		    idempotency_key_hash = $3, queued_at = NOW() - INTERVAL '15 minutes',
		    started_at = $4, locked_at = $5, lock_owner = $6,
		    attempt_count = $7, updated_at = NOW()
		WHERE id = $8`, status, selectedRowsJSON, idempotencyHash, startedAt, lockedAt, lockOwner, attemptCount, job.ID)
	require.NoError(t, err)

	loaded, err := repo.GetJob(ctx, owner.ID, job.ID)
	require.NoError(t, err)
	return loaded
}

func integrationHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func setImportJobLeaseTime(ctx context.Context, jobID int64, lockedAt time.Time) error {
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE enterprise_member_import_jobs SET locked_at = $1 WHERE id = $2`, lockedAt, jobID)
	return err
}

func assertImportJobLease(t *testing.T, ctx context.Context, jobID int64, expectedStatus, expectedOwner string) {
	t.Helper()
	var status string
	var lockOwner *string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, lock_owner FROM enterprise_member_import_jobs WHERE id = $1`, jobID).
		Scan(&status, &lockOwner))
	require.Equal(t, expectedStatus, status)
	if expectedOwner == "" {
		require.Nil(t, lockOwner)
		return
	}
	require.NotNil(t, lockOwner)
	require.Equal(t, expectedOwner, *lockOwner)
}
