package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberBudgetReceiptTaskLinkMigrationPersistsLifecycleIdentity(t *testing.T) {
	content, err := FS.ReadFile("195_enterprise_member_budget_receipt_task_link.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS receipt_kind VARCHAR(32) NOT NULL DEFAULT 'legacy'")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS async_task_id VARCHAR(64)")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS async_task_phase VARCHAR(16)")
	require.Contains(t, sql, "'sync', 'async_image', 'async_video', 'batch_image'")
	require.Contains(t, sql, "async_task_phase IN ('queued', 'executing')")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS idx_enterprise_member_budget_reservations_async_task")
	require.Contains(t, sql, "WHERE async_task_id IS NOT NULL")
}
