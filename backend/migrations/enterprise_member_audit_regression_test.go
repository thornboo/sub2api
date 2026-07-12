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
