package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberRequestReceiptMigrationPreservesAmbiguousOutcomes(t *testing.T) {
	content, err := FS.ReadFile("188_enterprise_member_request_receipts.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS group_id BIGINT")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS request_payload_hash CHAR(64) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "'ambiguous'")
	require.Contains(t, sql, "enterprise_member_budget_reservations(expires_at, id)")
	require.Contains(t, sql, "WHERE status = 'ambiguous'")
	require.NotContains(t, sql, "UPDATE enterprise_member_budget_periods")
}
