package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsErrorMemberAttributionMigrationKeepsHistoricalEvidenceIndependent(t *testing.T) {
	content, err := FS.ReadFile("180_ops_error_logs_enterprise_member_attribution.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS member_id BIGINT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS member_code_snapshot VARCHAR(100)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS member_name_snapshot VARCHAR(100)")
	require.NotContains(t, strings.ToUpper(sql), "REFERENCES ENTERPRISE_MEMBERS")
	require.NotContains(t, strings.ToUpper(sql), "ADD CONSTRAINT")
}

func TestOpsErrorMemberTimeIndexUsesNonTransactionalConcurrentMigration(t *testing.T) {
	content, err := FS.ReadFile("181_ops_error_logs_member_time_index_notx.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_user_member_time")
	require.Contains(t, sql, "ON ops_error_logs (user_id, member_id, created_at DESC)")
	require.Contains(t, sql, "WHERE user_id IS NOT NULL AND member_id IS NOT NULL")
}

func TestUsageMemberAttributionBackfillRequiresImmutableBudgetEvidence(t *testing.T) {
	content, err := FS.ReadFile("187_backfill_enterprise_member_usage_attribution.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "entry.kind = 'usage'")
	require.Contains(t, sql, "entry.usage_log_id IS NULL")
	require.Contains(t, sql, "entry.member_id = key.member_id")
	require.Contains(t, sql, "usage.user_id = key.user_id")
	require.Contains(t, sql, "entry.request_id = usage.api_key_id::text || ':' || usage.request_id")
	require.NotContains(t, sql, "member_name_snapshot =")
	require.NotContains(t, sql, "member_code_snapshot =")
	require.NotContains(t, sql, "enterprise_member_audit_logs")
}
