package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseMemberImportRepository struct{ db *sql.DB }

func NewEnterpriseMemberImportRepository(db *sql.DB) service.EnterpriseMemberImportRepository {
	return &enterpriseMemberImportRepository{db: db}
}

func (r *enterpriseMemberImportRepository) ValidateReferences(ctx context.Context, ownerID int64, memberCodes, keys []string, groupIDs []int64) (*service.EnterpriseMemberImportReferenceState, error) {
	state := &service.EnterpriseMemberImportReferenceState{
		ExistingMemberCodes: map[string]bool{}, ExistingKeys: map[string]bool{}, AuthorizedGroupIDs: map[int64]bool{}, VersionFingerprint: map[string]int64{},
	}
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member import repository db is nil")
	}
	if len(memberCodes) > 0 {
		rows, err := r.db.QueryContext(ctx, `SELECT LOWER(member_code), version FROM enterprise_members WHERE enterprise_user_id = $1 AND LOWER(member_code) = ANY($2)`, ownerID, pq.Array(lowerStrings(memberCodes)))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var code string
			var version int64
			if err := rows.Scan(&code, &version); err != nil {
				_ = rows.Close()
				return nil, err
			}
			state.ExistingMemberCodes[code] = true
			state.VersionFingerprint["member:"+code] = version
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	if len(keys) > 0 {
		// api_keys uses in-table soft deletion, so this raw query already includes
		// both active and deleted rows. Keeping deleted keys in the conflict set
		// prevents historical credentials from being silently reused.
		rows, err := r.db.QueryContext(ctx, `SELECT key FROM api_keys WHERE key = ANY($1)`, pq.Array(keys))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var key string
			if err := rows.Scan(&key); err != nil {
				_ = rows.Close()
				return nil, err
			}
			state.ExistingKeys[key] = true
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	if len(groupIDs) > 0 {
		rows, err := r.db.QueryContext(ctx, `
			SELECT g.id, EXTRACT(EPOCH FROM g.updated_at)::BIGINT
			FROM groups g
			WHERE g.id = ANY($2) AND g.deleted_at IS NULL AND g.status = 'active' AND (
				(g.subscription_type = 'subscription' AND EXISTS (
					SELECT 1 FROM user_subscriptions us
					WHERE us.user_id = $1 AND us.group_id = g.id AND us.deleted_at IS NULL
					  AND us.status = 'active' AND us.starts_at <= NOW() AND us.expires_at > NOW()
				)) OR
				(g.subscription_type <> 'subscription' AND (
					NOT g.is_exclusive OR EXISTS (SELECT 1 FROM user_allowed_groups uag WHERE uag.user_id = $1 AND uag.group_id = g.id)
				))
			)`, ownerID, pq.Array(groupIDs))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id, version int64
			if err := rows.Scan(&id, &version); err != nil {
				_ = rows.Close()
				return nil, err
			}
			state.AuthorizedGroupIDs[id] = true
			state.VersionFingerprint[fmt.Sprintf("group:%d", id)] = version
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func (r *enterpriseMemberImportRepository) CreatePreviewJob(ctx context.Context, job *service.EnterpriseMemberImportJob) error {
	if job.ImportPolicyVersion <= 0 {
		job.ImportPolicyVersion = service.EnterpriseMemberImportPolicyExplicitActivation
	}
	previewJSON, err := json.Marshal(job.Preview)
	if err != nil {
		return err
	}
	versionJSON, err := json.Marshal(job.VersionFingerprint)
	if err != nil {
		return err
	}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO enterprise_member_import_jobs
		(enterprise_user_id, token_hash, file_hash, format, status, preview, version_fingerprint, expires_at, import_policy_version)
		VALUES ($1, $2, $3, $4, 'previewed', $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`, job.EnterpriseUserID, job.TokenHash, job.FileHash, job.Format, previewJSON, versionJSON, job.ExpiresAt, job.ImportPolicyVersion).
		Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)
	return err
}

func (r *enterpriseMemberImportRepository) GetPreviewJob(ctx context.Context, ownerID, jobID int64, tokenHash string) (*service.EnterpriseMemberImportJob, error) {
	job, err := r.getJob(ctx, ownerID, jobID, tokenHash)
	if err != nil {
		return nil, err
	}
	if job.Status != "previewed" {
		if job.Status == "completed" {
			return job, nil
		}
		return nil, service.ErrEnterpriseMemberImportConflict
	}
	if time.Now().After(job.ExpiresAt) {
		return nil, service.ErrEnterpriseMemberImportExpired
	}
	return job, nil
}

func (r *enterpriseMemberImportRepository) GetJob(ctx context.Context, ownerID, jobID int64) (*service.EnterpriseMemberImportJob, error) {
	return r.getJob(ctx, ownerID, jobID, "")
}

func (r *enterpriseMemberImportRepository) GetJobByToken(ctx context.Context, ownerID, jobID int64, tokenHash string) (*service.EnterpriseMemberImportJob, error) {
	return r.getJob(ctx, ownerID, jobID, tokenHash)
}

func (r *enterpriseMemberImportRepository) getJob(ctx context.Context, ownerID, jobID int64, tokenHash string) (*service.EnterpriseMemberImportJob, error) {
	query := `SELECT id, enterprise_user_id, token_hash, file_hash, format, status, preview, result, version_fingerprint, idempotency_key_hash, expires_at, created_at, updated_at, completed_at,
		selected_rows, default_group_ids, activate_members, import_policy_version, queued_at, started_at, locked_at, lock_owner, attempt_count, error_code, error_summary, result_secrets_consumed_at
		FROM enterprise_member_import_jobs WHERE id = $1 AND enterprise_user_id = $2`
	args := []any{jobID, ownerID}
	if tokenHash != "" {
		query += ` AND token_hash = $3`
		args = append(args, tokenHash)
	}
	var job service.EnterpriseMemberImportJob
	var previewJSON, versionJSON, selectedRowsJSON, defaultGroupIDsJSON []byte
	var resultJSON []byte
	var idempotency, lockOwner, errorCode, errorSummary sql.NullString
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&job.ID, &job.EnterpriseUserID, &job.TokenHash, &job.FileHash, &job.Format, &job.Status, &previewJSON, &resultJSON, &versionJSON, &idempotency, &job.ExpiresAt, &job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
		&selectedRowsJSON, &defaultGroupIDsJSON, &job.ActivateMembers, &job.ImportPolicyVersion, &job.QueuedAt, &job.StartedAt, &job.LockedAt, &lockOwner, &job.AttemptCount, &errorCode, &errorSummary, &job.ResultSecretsConsumedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberImportExpired
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(previewJSON, &job.Preview); err != nil {
		return nil, err
	}
	job.Preview.JobID = job.ID
	if len(resultJSON) > 0 {
		var result service.EnterpriseMemberImportResult
		if err := json.Unmarshal(resultJSON, &result); err != nil {
			return nil, err
		}
		job.Result = &result
	}
	if len(versionJSON) > 0 {
		if err := json.Unmarshal(versionJSON, &job.VersionFingerprint); err != nil {
			return nil, err
		}
	}
	if idempotency.Valid {
		job.IdempotencyKeyHash = &idempotency.String
	}
	if len(selectedRowsJSON) > 0 {
		if err := json.Unmarshal(selectedRowsJSON, &job.SelectedRows); err != nil {
			return nil, err
		}
	}
	if len(defaultGroupIDsJSON) > 0 {
		if err := json.Unmarshal(defaultGroupIDsJSON, &job.DefaultGroupIDs); err != nil {
			return nil, err
		}
	}
	if lockOwner.Valid {
		job.LockOwner = &lockOwner.String
	}
	if errorCode.Valid {
		job.ErrorCode = &errorCode.String
	}
	if errorSummary.Valid {
		job.ErrorSummary = &errorSummary.String
	}
	return &job, nil
}

func (r *enterpriseMemberImportRepository) QueueCommit(ctx context.Context, ownerID, jobID int64, tokenHash string, selectedRows []int, defaultGroupIDs []int64, activateMembers bool, idempotencyKeyHash string) (*service.EnterpriseMemberImportJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member import repository db is nil")
	}
	selectedJSON, err := json.Marshal(selectedRows)
	if err != nil {
		return nil, err
	}
	defaultGroupsJSON, err := json.Marshal(defaultGroupIDs)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var expiresAt time.Time
	var existingHash sql.NullString
	var existingSelectedRows, existingDefaultGroups []byte
	var existingActivateMembers bool
	var importPolicyVersion int
	if err := tx.QueryRowContext(ctx, `SELECT status, expires_at, idempotency_key_hash, selected_rows, default_group_ids, activate_members, import_policy_version FROM enterprise_member_import_jobs WHERE id = $1 AND enterprise_user_id = $2 AND token_hash = $3 FOR UPDATE`, jobID, ownerID, tokenHash).
		Scan(&status, &expiresAt, &existingHash, &existingSelectedRows, &existingDefaultGroups, &existingActivateMembers, &importPolicyVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrEnterpriseMemberImportExpired
		}
		return nil, err
	}
	queueStatus := "queued"
	commitProtocolVersion := service.EnterpriseMemberImportCommitProtocolLegacy
	if importPolicyVersion >= service.EnterpriseMemberImportPolicyExplicitActivation {
		queueStatus = service.EnterpriseMemberImportStatusQueuedV2
		commitProtocolVersion = service.EnterpriseMemberImportCommitProtocolPolicyV2
	}
	if status == "previewed" {
		if time.Now().After(expiresAt) {
			return nil, service.ErrEnterpriseMemberImportExpired
		}
		if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_import_jobs SET status = $1, commit_protocol_version = $2, selected_rows = $3, default_group_ids = $4, activate_members = $5, idempotency_key_hash = $6, queued_at = NOW(), updated_at = NOW(), error_code = NULL, error_summary = NULL WHERE id = $7`, queueStatus, commitProtocolVersion, selectedJSON, defaultGroupsJSON, activateMembers, idempotencyKeyHash, jobID); err != nil {
			return nil, err
		}
	} else {
		if status != "queued" && status != service.EnterpriseMemberImportStatusQueuedV2 && status != "processing" && status != service.EnterpriseMemberImportStatusProcessingV2 && status != "completed" {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
		if !existingHash.Valid || existingHash.String != idempotencyKeyHash {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
		var existingRows []int
		if json.Unmarshal(existingSelectedRows, &existingRows) != nil || !equalEnterpriseMemberImportRows(existingRows, selectedRows) {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
		var existingGroups []int64
		if json.Unmarshal(existingDefaultGroups, &existingGroups) != nil || !equalEnterpriseMemberImportGroups(existingGroups, defaultGroupIDs) || existingActivateMembers != activateMembers {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if status == "previewed" {
		status = queueStatus
	}
	return &service.EnterpriseMemberImportJob{ID: jobID, EnterpriseUserID: ownerID, Status: status}, nil
}

func equalEnterpriseMemberImportGroups(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalEnterpriseMemberImportRows(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (r *enterpriseMemberImportRepository) ClaimNextCommitJob(ctx context.Context, workerID string, staleAfter time.Duration) (*service.EnterpriseMemberImportJob, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member import repository db is nil")
	}
	if staleAfter <= 0 {
		staleAfter = 2 * time.Minute
	}
	var jobID, ownerID int64
	err := r.db.QueryRowContext(ctx, `
		WITH candidate AS (
			SELECT id FROM enterprise_member_import_jobs
			WHERE status IN ('queued', 'queued_v2')
			   OR (status IN ('processing', 'processing_v2') AND (locked_at IS NULL OR locked_at < NOW() - ($2 * INTERVAL '1 second')))
			ORDER BY COALESCE(queued_at, created_at), id
			FOR UPDATE SKIP LOCKED LIMIT 1
		)
		UPDATE enterprise_member_import_jobs job
		SET status = CASE
		        WHEN job.import_policy_version >= 2 THEN 'processing_v2'
		        ELSE 'processing'
		    END,
		    started_at = COALESCE(started_at, NOW()), locked_at = NOW(),
		    lock_owner = $1, attempt_count = attempt_count + 1, updated_at = NOW()
		FROM candidate WHERE job.id = candidate.id
		RETURNING job.id, job.enterprise_user_id`, workerID, int(staleAfter.Seconds())).Scan(&jobID, &ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberImportQueueEmpty
	}
	if err != nil {
		return nil, err
	}
	return r.GetJob(ctx, ownerID, jobID)
}

func (r *enterpriseMemberImportRepository) RenewCommitLease(ctx context.Context, jobID int64, workerID string) (bool, error) {
	if r == nil || r.db == nil {
		return false, errors.New("enterprise member import repository db is nil")
	}
	if jobID <= 0 || strings.TrimSpace(workerID) == "" {
		return false, nil
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_import_jobs
		SET locked_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ('processing', 'processing_v2') AND lock_owner = $2`, jobID, workerID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *enterpriseMemberImportRepository) MarkCommitFailed(ctx context.Context, jobID int64, workerID, errorCode, summary string) error {
	if r == nil || r.db == nil {
		return errors.New("enterprise member import repository db is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_import_jobs
		SET status = 'failed', error_code = $1, error_summary = $2,
		    preview = jsonb_set(preview, '{rows}', COALESCE((
		        SELECT jsonb_agg(row_value - 'api_key_ciphertext')
		        FROM jsonb_array_elements(preview->'rows') AS row_value
		    ), '[]'::jsonb)),
		    result_secrets_ciphertext = NULL, lock_owner = NULL, locked_at = NULL,
		    completed_at = NOW(), updated_at = NOW()
		WHERE id = $3 AND status IN ('processing', 'processing_v2') AND lock_owner = $4`, errorCode, summary, jobID, workerID)
	return err
}

func (r *enterpriseMemberImportRepository) ConsumeResultSecrets(ctx context.Context, ownerID, jobID int64, tokenHash string) (string, error) {
	if r == nil || r.db == nil {
		return "", errors.New("enterprise member import repository db is nil")
	}
	var ciphertext sql.NullString
	err := r.db.QueryRowContext(ctx, `
		WITH claimed AS (
			SELECT id, result_secrets_ciphertext
			FROM enterprise_member_import_jobs
			WHERE id = $1 AND enterprise_user_id = $2 AND token_hash = $3 AND status = 'completed'
			  AND result_secrets_ciphertext IS NOT NULL AND result_secrets_consumed_at IS NULL
			FOR UPDATE
		), consumed AS (
			UPDATE enterprise_member_import_jobs job
			SET result_secrets_ciphertext = NULL, result_secrets_consumed_at = NOW(), updated_at = NOW()
			FROM claimed WHERE job.id = claimed.id
			RETURNING claimed.result_secrets_ciphertext
		)
		SELECT result_secrets_ciphertext FROM consumed`, jobID, ownerID, tokenHash).Scan(&ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		var status string
		var consumedAt sql.NullTime
		lookupErr := r.db.QueryRowContext(ctx, `SELECT status, result_secrets_consumed_at FROM enterprise_member_import_jobs WHERE id = $1 AND enterprise_user_id = $2 AND token_hash = $3`, jobID, ownerID, tokenHash).Scan(&status, &consumedAt)
		if lookupErr != nil {
			return "", service.ErrEnterpriseMemberImportExpired
		}
		if status != "completed" {
			return "", service.ErrEnterpriseMemberImportPending
		}
		return "", service.ErrEnterpriseMemberImportConsumed
	}
	if err != nil {
		return "", err
	}
	return ciphertext.String, nil
}

func (r *enterpriseMemberImportRepository) Commit(ctx context.Context, job *service.EnterpriseMemberImportJob, rows []service.EnterpriseMemberImportRow, plaintextKeys map[int]string, idempotencyKeyHash, resultSecretsCiphertext string) (_ *service.EnterpriseMemberImportResult, err error) {
	if r == nil || r.db == nil || job == nil {
		return nil, service.ErrEnterpriseMemberImportConflict
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	var expiresAt time.Time
	var existingHash, currentLockOwner sql.NullString
	var storedResult []byte
	err = tx.QueryRowContext(ctx, `SELECT status, expires_at, idempotency_key_hash, result, lock_owner FROM enterprise_member_import_jobs WHERE id = $1 AND enterprise_user_id = $2 AND token_hash = $3 FOR UPDATE`, job.ID, job.EnterpriseUserID, job.TokenHash).
		Scan(&status, &expiresAt, &existingHash, &storedResult, &currentLockOwner)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrEnterpriseMemberImportExpired
	}
	if err != nil {
		return nil, err
	}
	if status == "completed" {
		if !existingHash.Valid || existingHash.String != idempotencyKeyHash {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
		var result service.EnterpriseMemberImportResult
		if err := json.Unmarshal(storedResult, &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
	if status != "previewed" && status != "processing" && status != service.EnterpriseMemberImportStatusProcessingV2 {
		return nil, service.ErrEnterpriseMemberImportConflict
	}
	if status == "previewed" && time.Now().After(expiresAt) {
		return nil, service.ErrEnterpriseMemberImportExpired
	}
	if status == "processing" || status == service.EnterpriseMemberImportStatusProcessingV2 {
		if !existingHash.Valid || existingHash.String != idempotencyKeyHash {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
		// A worker whose lease has been taken over must never commit against the
		// replacement worker's job. Requiring the persisted lock owner here is
		// the fencing check that protects all writes below this point.
		if currentLockOwner.Valid && (job.LockOwner == nil || currentLockOwner.String != *job.LockOwner) {
			return nil, service.ErrEnterpriseMemberImportConflict
		}
	} else {
		processingStatus := "processing"
		if job.ImportPolicyVersion >= service.EnterpriseMemberImportPolicyExplicitActivation {
			processingStatus = service.EnterpriseMemberImportStatusProcessingV2
		}
		commitProtocolVersion := service.EnterpriseMemberImportCommitProtocolLegacy
		if job.ImportPolicyVersion >= service.EnterpriseMemberImportPolicyExplicitActivation {
			commitProtocolVersion = service.EnterpriseMemberImportCommitProtocolPolicyV2
		}
		if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_import_jobs SET status = $1, commit_protocol_version = $2, idempotency_key_hash = $3, updated_at = NOW() WHERE id = $4`, processingStatus, commitProtocolVersion, idempotencyKeyHash, job.ID); err != nil {
			return nil, err
		}
	}

	selectedCodes := map[string]bool{}
	selectedRowNumbers := make([]int, 0, len(rows))
	keys := make([]string, 0, len(plaintextKeys))
	groupSet := map[int64]bool{}
	memberRows := make(map[string]service.EnterpriseMemberImportRow)
	memberGroups := make(map[string][]int64)
	openingByMember := make(map[string]float64)
	migrationBilledUSD := 0.0
	var migrationTotalTokens service.EnterpriseMemberTokenCount
	for _, row := range rows {
		code := strings.ToLower(row.MemberCode)
		selectedCodes[code] = true
		if _, exists := memberRows[code]; !exists {
			memberRows[code] = row
			effectiveGroups := enterpriseMemberImportEffectiveGroups(job, row)
			memberGroups[code] = append([]int64(nil), effectiveGroups...)
		}
		openingByMember[code] += row.OpeningUsedUSD
		migrationBilledUSD += row.OpeningUsedUSD
		migrationTotalTokens = migrationTotalTokens.Add(enterpriseMemberImportSummaryTokens(row))
		selectedRowNumbers = append(selectedRowNumbers, row.RowNumber)
		if key := plaintextKeys[row.RowNumber]; key != "" {
			keys = append(keys, key)
		}
		for _, id := range memberGroups[code] {
			groupSet[id] = true
		}
	}
	memberCodes := make([]string, 0, len(selectedCodes))
	for code := range selectedCodes {
		memberCodes = append(memberCodes, code)
	}
	groupIDs := make([]int64, 0, len(groupSet))
	for id := range groupSet {
		groupIDs = append(groupIDs, id)
	}
	if err := validateEnterpriseMemberImportCommitReferences(ctx, tx, job.EnterpriseUserID, memberCodes, keys, groupIDs); err != nil {
		return nil, err
	}

	memberIDs := map[string]int64{}
	sortedCodes := append([]string(nil), memberCodes...)
	sort.Strings(sortedCodes)
	periodStart := job.Preview.PeriodStart
	if periodStart.IsZero() {
		periodStart, _ = service.EnterpriseMemberCurrentBudgetPeriod(time.Now())
	}
	periodStartDate := periodStart.Format("2006-01-02")
	periodTimezone := strings.TrimSpace(job.Preview.Timezone)
	if periodTimezone == "" {
		periodTimezone = service.EnterpriseMemberBudgetTimezone
	}
	pendingMembers := 0
	for _, code := range sortedCodes {
		row := memberRows[code]
		groups := memberGroups[code]
		status := service.EnterpriseMemberStatusDisabled
		if enterpriseMemberImportShouldActivate(job, groups) {
			status = service.EnterpriseMemberStatusActive
		}
		if len(groups) == 0 {
			pendingMembers++
		}
		var memberID int64
		err := tx.QueryRowContext(ctx, `INSERT INTO enterprise_members (enterprise_user_id, member_code, name, status, monthly_limit_usd, rate_limit_5h, rate_limit_1d, rate_limit_7d, version) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1) RETURNING id`, job.EnterpriseUserID, row.MemberCode, row.MemberName, status, row.MonthlyLimitUSD, row.RateLimit5h, row.RateLimit1d, row.RateLimit7d).Scan(&memberID)
		if err != nil {
			return nil, classifyEnterpriseMemberImportWriteError("insert enterprise member", err)
		}
		memberIDs[code] = memberID
		for order, groupID := range groups {
			if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_member_group_bindings (member_id, group_id, sort_order) VALUES ($1, $2, $3)`, memberID, groupID, order); err != nil {
				return nil, classifyEnterpriseMemberImportWriteError("insert enterprise member group binding", err)
			}
		}
		opening := openingByMember[code]
		if opening > 0 {
			if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_member_budget_periods (member_id, period_start, timezone, used_usd) VALUES ($1, $2, $3, $4)`, memberID, periodStartDate, periodTimezone, opening); err != nil {
				return nil, err
			}
			ledgerKey := fmt.Sprintf("import:%d:member:%s", job.ID, strings.ToLower(row.MemberCode))
			if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_member_budget_entries (member_id, period_start, kind, amount_usd, idempotency_key, actor_user_id, note) VALUES ($1, $2, 'migration_opening', $3, $4, $5, $6)`, memberID, periodStartDate, opening, ledgerKey, job.EnterpriseUserID, "enterprise member import opening balance"); err != nil {
				return nil, err
			}
		}
	}
	createdKeys := make([]service.EnterpriseMemberImportCreatedKey, 0, len(keys))
	for _, row := range rows {
		key := plaintextKeys[row.RowNumber]
		memberID := memberIDs[strings.ToLower(row.MemberCode)]
		var apiKeyID sql.NullInt64
		if key != "" {
			var createdKeyID int64
			if err := tx.QueryRowContext(ctx, `INSERT INTO api_keys (user_id, key, name, member_id, status, quota) VALUES ($1, $2, $3, $4, 'active', $5) RETURNING id`, job.EnterpriseUserID, key, row.KeyName, memberID, row.KeyQuotaUSD).Scan(&createdKeyID); err != nil {
				return nil, classifyEnterpriseMemberImportWriteError("insert enterprise member key", err)
			}
			apiKeyID = sql.NullInt64{Int64: createdKeyID, Valid: true}
			createdKeys = append(createdKeys, service.EnterpriseMemberImportCreatedKey{MemberCode: row.MemberCode, KeyName: row.KeyName, Key: key, KeyMasked: maskEnterpriseImportKey(key)})
		}
		if enterpriseMemberImportRowHasBaseline(row) {
			if _, err := tx.ExecContext(ctx, `INSERT INTO enterprise_member_import_usage_baselines (enterprise_user_id, member_id, api_key_id, import_job_id, source_row_number, period_start, billed_usd, total_tokens, input_tokens, output_tokens, cache_tokens, cache_creation_tokens, cache_read_tokens) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`, job.EnterpriseUserID, memberID, apiKeyID, job.ID, row.RowNumber, periodStartDate, row.OpeningUsedUSD, enterpriseMemberImportSummaryTokens(row), row.InputTokens, row.OutputTokens, row.CacheTokens, row.CacheCreationTokens, row.CacheReadTokens); err != nil {
				return nil, err
			}
		}
	}
	now := time.Now()
	createdMemberIDs := make([]int64, 0, len(sortedCodes))
	for _, code := range sortedCodes {
		createdMemberIDs = append(createdMemberIDs, memberIDs[code])
	}
	result := &service.EnterpriseMemberImportResult{
		JobID: job.ID, Status: "completed", CreatedMembers: len(memberIDs), CreatedKeys: len(createdKeys),
		MemberIDs: createdMemberIDs, PendingMembers: pendingMembers, MigrationBilledUSD: migrationBilledUSD,
		MigrationTotalTokens: migrationTotalTokens, PeriodStart: periodStart, Timezone: periodTimezone,
		Rows: selectedRowNumbers, Keys: createdKeys, CompletedAt: now,
	}
	stored := *result
	stored.Keys = append([]service.EnterpriseMemberImportCreatedKey(nil), result.Keys...)
	for i := range stored.Keys {
		stored.Keys[i].Key = ""
	}
	resultJSON, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	sanitizedPreview := job.Preview
	for i := range sanitizedPreview.Rows {
		sanitizedPreview.Rows[i].APIKeyCiphertext = ""
	}
	previewJSON, err := json.Marshal(sanitizedPreview)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE enterprise_member_import_jobs SET status = 'completed', preview = $1, result = $2, result_secrets_ciphertext = NULLIF($3, ''), lock_owner = NULL, locked_at = NULL, updated_at = $4, completed_at = $4 WHERE id = $5`, previewJSON, resultJSON, resultSecretsCiphertext, now, job.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func enterpriseMemberImportRowHasBaseline(row service.EnterpriseMemberImportRow) bool {
	return row.OpeningUsedUSD > 0 || row.TotalTokens.IsPositive() || row.InputTokens.IsPositive() || row.OutputTokens.IsPositive() || row.CacheTokens.IsPositive() || row.CacheCreationTokens.IsPositive() || row.CacheReadTokens.IsPositive()
}

func enterpriseMemberImportSummaryTokens(row service.EnterpriseMemberImportRow) service.EnterpriseMemberTokenCount {
	if row.TotalTokensProvided || row.TotalTokens.IsPositive() {
		return row.TotalTokens
	}
	return row.InputTokens.Add(row.OutputTokens)
}

func enterpriseMemberImportShouldActivate(job *service.EnterpriseMemberImportJob, groupIDs []int64) bool {
	if job == nil || len(groupIDs) == 0 {
		return false
	}
	if job.ImportPolicyVersion <= service.EnterpriseMemberImportPolicyLegacyAutoActivate {
		return true
	}
	return job.ActivateMembers
}

func enterpriseMemberImportEffectiveGroups(job *service.EnterpriseMemberImportJob, row service.EnterpriseMemberImportRow) []int64 {
	if job != nil && job.ImportPolicyVersion >= service.EnterpriseMemberImportPolicyExplicitActivation {
		return job.DefaultGroupIDs
	}
	return row.GroupIDs
}

func classifyEnterpriseMemberImportWriteError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var postgresError *pq.Error
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			switch postgresError.Constraint {
			case "enterprise_members_owner_code_unique", "enterprise_members_owner_code_ci_unique", "api_keys_key_key":
				return service.ErrEnterpriseMemberImportConflict
			}
		case "23503":
			if postgresError.Constraint == "enterprise_member_group_bindings_group_id_fkey" {
				return service.ErrEnterpriseMemberImportConflict
			}
		}
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func (r *enterpriseMemberImportRepository) DeleteExpiredPreviews(ctx context.Context, limit int) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("enterprise member import repository db is nil")
	}
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	if _, err := r.db.ExecContext(ctx, `
		UPDATE enterprise_member_import_jobs
		SET result_secrets_ciphertext = NULL, updated_at = NOW()
		WHERE status = 'completed' AND result_secrets_ciphertext IS NOT NULL
		  AND completed_at <= NOW() - INTERVAL '24 hours'`); err != nil {
		return 0, err
	}
	result, err := r.db.ExecContext(ctx, `
		WITH expired AS (
			SELECT id FROM enterprise_member_import_jobs
			WHERE status = 'previewed' AND expires_at <= NOW()
			ORDER BY expires_at, id LIMIT $1 FOR UPDATE SKIP LOCKED
		)
		DELETE FROM enterprise_member_import_jobs j USING expired e WHERE j.id = e.id`, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func validateEnterpriseMemberImportCommitReferences(ctx context.Context, tx *sql.Tx, ownerID int64, memberCodes, keys []string, groupIDs []int64) error {
	var conflict bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM enterprise_members WHERE enterprise_user_id = $1 AND LOWER(member_code) = ANY($2)) OR EXISTS (SELECT 1 FROM api_keys WHERE key = ANY($3))`, ownerID, pq.Array(memberCodes), pq.Array(keys)).Scan(&conflict); err != nil {
		return err
	}
	if conflict {
		return service.ErrEnterpriseMemberImportConflict
	}
	var authorizedCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups g WHERE g.id = ANY($2) AND g.deleted_at IS NULL AND g.status = 'active' AND ((g.subscription_type = 'subscription' AND EXISTS (SELECT 1 FROM user_subscriptions us WHERE us.user_id = $1 AND us.group_id = g.id AND us.deleted_at IS NULL AND us.status = 'active' AND us.starts_at <= NOW() AND us.expires_at > NOW())) OR (g.subscription_type <> 'subscription' AND (NOT g.is_exclusive OR EXISTS (SELECT 1 FROM user_allowed_groups uag WHERE uag.user_id = $1 AND uag.group_id = g.id))))`, ownerID, pq.Array(groupIDs)).Scan(&authorizedCount); err != nil {
		return err
	}
	if authorizedCount != len(groupIDs) {
		return service.ErrEnterpriseMemberImportConflict
	}
	return nil
}

func lowerStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strings.ToLower(strings.TrimSpace(value)))
	}
	return out
}
func maskEnterpriseImportKey(key string) string {
	if len(key) <= 12 {
		return "***"
	}
	return key[:6] + "…" + key[len(key)-4:]
}
