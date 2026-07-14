package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAppliedMigrationsRemainImmutable(t *testing.T) {
	expected := map[string]string{
		"175_enterprise_members.sql":            "79c4c3f17274e284ff883118281b373ff87f63a051789173ac614782d85db01a",
		"177_enterprise_member_audit_logs.sql":  "2843384d74ac6c1b10ffdaa344090cb5f44c591021e74c8b62251ebb6b81a98d",
		"178_enterprise_member_import_jobs.sql": "6cdbabbc8706f5a57cd89379509f9a9085b221c98db78be76ff5ffdf6b896e98",
		"179_enterprise_member_rate_limits.sql": "44f55ec424c3d0b53793e582ed5211ebb11c89808fd4f670f104cedd3ff03aa1",
	}

	for filename, expectedChecksum := range expected {
		t.Run(filename, func(t *testing.T) {
			content, err := FS.ReadFile(filename)
			require.NoError(t, err)

			sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
			require.Equal(t, expectedChecksum, hex.EncodeToString(sum[:]),
				"applied migrations are immutable; add a new numbered migration for schema changes")
		})
	}
}

func TestEnterpriseMemberAuditMigrationIsAtomicAppendOnlyAndCredentialSafe(t *testing.T) {
	content, err := FS.ReadFile("177_enterprise_member_audit_logs.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS enterprise_member_audit_logs")
	require.Contains(t, sql, "enterprise_member_audit_reject_mutation")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON enterprise_member_audit_logs")
	require.Contains(t, sql, "AFTER INSERT OR UPDATE OR DELETE ON enterprise_members")
	require.Contains(t, sql, "AFTER INSERT OR DELETE OR UPDATE OF name, status, quota")
	require.Contains(t, sql, "WHEN OLD.member_id IS NULL AND NEW.member_id IS NOT NULL THEN 'member_key.adopted'")
	require.Contains(t, sql, "member_id, group_id")
	require.Contains(t, sql, "WHERE member_id IS NOT NULL")
	require.Contains(t, sql, "WHEN (NEW.kind <> 'usage')")
	require.NotContains(t, sql, "to_jsonb(NEW)")
	require.NotContains(t, sql, "to_jsonb(OLD)")
	require.NotContains(t, sql, "row_to_json")
	require.NotContains(t, sql, "NEW.key")
	require.NotContains(t, sql, "OLD.key")
	require.NotContains(t, sql, "NEW.preview")
	require.NotContains(t, sql, "NEW.result")
}

func TestEnterpriseMemberImportJobMigrationUsesDurableLeasesAndEncryptedOneTimeResults(t *testing.T) {
	content, err := FS.ReadFile("178_enterprise_member_import_jobs.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "'queued', 'processing', 'completed', 'failed'")
	require.Contains(t, sql, "selected_rows JSONB")
	require.Contains(t, sql, "locked_at TIMESTAMPTZ")
	require.Contains(t, sql, "lock_owner VARCHAR(128)")
	require.Contains(t, sql, "attempt_count INTEGER")
	require.Contains(t, sql, "result_secrets_ciphertext TEXT")
	require.Contains(t, sql, "result_secrets_consumed_at TIMESTAMPTZ")
	require.NotContains(t, sql, "result_secrets_plaintext")
}

func TestEnterpriseMemberImportBaselineMigrationSeparatesExternalFactsFromRequestLogs(t *testing.T) {
	content, err := FS.ReadFile("182_enterprise_member_import_baselines.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "default_group_ids JSONB")
	require.Contains(t, sql, "activate_members BOOLEAN")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS enterprise_member_import_usage_baselines")
	require.Contains(t, sql, "enterprise_member_import_usage_baseline_reject_mutation")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON enterprise_member_import_usage_baselines")
	require.Contains(t, sql, "UNIQUE (import_job_id, source_row_number)")
	require.NotContains(t, sql, "INSERT INTO usage_logs")
}

func TestEnterpriseMemberImportPolicyMigrationPreservesLegacyJobsAndVersionsNewJobs(t *testing.T) {
	content, err := FS.ReadFile("183_enterprise_member_import_policy_versions.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS import_policy_version SMALLINT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS commit_protocol_version SMALLINT")
	require.Contains(t, sql, "SET import_policy_version = 1")
	require.Contains(t, sql, "ALTER COLUMN import_policy_version SET DEFAULT 1")
	require.Contains(t, sql, "New code writes policy 2")
	require.Contains(t, sql, "CHECK (import_policy_version IN (1, 2))")
	require.Contains(t, sql, "'queued_v2'")
	require.Contains(t, sql, "'processing_v2'")
	require.Contains(t, sql, "enterprise_member_import_enforce_queue_protocol")
	require.Contains(t, sql, "policy-2 enterprise member import requires commit protocol 2")
	require.Contains(t, sql, "BEFORE INSERT OR UPDATE OF status, import_policy_version, commit_protocol_version")
}

func TestEnterpriseMemberLedgerIntegrityMigrationProtectsAccountingFacts(t *testing.T) {
	indexContent, err := FS.ReadFile("184_enterprise_member_baseline_identity_index_notx.sql")
	require.NoError(t, err)
	require.Contains(t, string(indexContent), "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_api_keys_id_member_owner")

	content, err := FS.ReadFile("185_enterprise_member_ledger_integrity.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "enterprise_member_import_usage_baselines_key_member_owner_fk")
	require.Contains(t, sql, "FOREIGN KEY (api_key_id, member_id, enterprise_user_id)")
	require.Contains(t, sql, "enterprise_member_import_jobs_policy_v2_activation_groups_check")
	require.Contains(t, sql, "jsonb_array_length(default_group_ids) > 0")
	require.Contains(t, sql, "enterprise_member_budget_entry_reject_mutation")
	require.Contains(t, sql, "BEFORE UPDATE OR DELETE ON enterprise_member_budget_entries")
	require.Contains(t, sql, "OLD.usage_log_id IS NULL")
	require.Contains(t, sql, "NEW.usage_log_id IS NOT NULL")
	require.Contains(t, sql, "NEW IS NOT DISTINCT FROM OLD")
}

func TestEnterpriseMemberRemovalLifecycleSeparatesArchiveFromDeletion(t *testing.T) {
	content, err := FS.ReadFile("186_enterprise_member_removal_lifecycle.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS removed_at TIMESTAMPTZ")
	require.Contains(t, sql, "CHECK (removed_at IS NULL OR deleted_at IS NOT NULL)")
	require.Contains(t, sql, "WHEN OLD.removed_at IS NULL AND NEW.removed_at IS NOT NULL THEN 'member.removed'")
	require.Contains(t, sql, "WHEN OLD.deleted_at IS NOT NULL AND NEW.deleted_at IS NULL THEN 'member.restored'")
	require.Contains(t, sql, "'removed_at', NEW.removed_at")
	require.NotContains(t, sql, "ON DELETE CASCADE")
}
