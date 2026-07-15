package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberReceiptReconciliationMetadataMigrationPersistsDiagnostics(t *testing.T) {
	content, err := FS.ReadFile("189_enterprise_member_receipt_reconciliation_metadata.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ALTER COLUMN request_payload_hash TYPE VARCHAR(64)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS outcome_reason VARCHAR(64) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS reconcile_attempts INTEGER NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS last_reconcile_at TIMESTAMPTZ")
	require.Contains(t, sql, "CHECK (reconcile_attempts >= 0)")
}
