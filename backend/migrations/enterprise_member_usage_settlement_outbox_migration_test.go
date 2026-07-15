package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberUsageSettlementOutboxMigration(t *testing.T) {
	content, err := FS.ReadFile("190_enterprise_member_usage_settlement_outbox.sql")
	require.NoError(t, err)
	sql := string(content)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS enterprise_member_usage_settlement_outbox")
	require.Contains(t, sql, "UNIQUE (api_key_id, request_id)")
	require.Contains(t, sql, "command_payload JSONB NOT NULL")
	require.Contains(t, sql, "FOREIGN KEY (api_key_id, member_id, enterprise_user_id)")
	require.Contains(t, sql, "REFERENCES api_keys(id, member_id, user_id)")
	require.Contains(t, sql, "FOREIGN KEY (member_id, enterprise_user_id)")
	require.Contains(t, sql, "REFERENCES enterprise_members(id, enterprise_user_id)")
	require.Contains(t, sql, "enterprise_member_usage_settlement_outbox_identity_check")
	require.Contains(t, strings.ToLower(sql), "create index if not exists idx_enterprise_member_usage_settlement_outbox_due")
}
