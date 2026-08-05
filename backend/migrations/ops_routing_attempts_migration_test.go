package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpsRoutingAttemptsMigration(t *testing.T) {
	content, err := FS.ReadFile("201_ops_routing_attempts.sql")
	require.NoError(t, err)
	sql := string(content)
	lower := strings.ToLower(sql)

	require.Contains(t, lower, "alter table ops_error_logs")
	require.Contains(t, lower, "add column if not exists routing_attempts jsonb not null default '[]'::jsonb")
	require.Contains(t, lower, "add column if not exists routing_plan_source text")
	require.Contains(t, lower, "add column if not exists routing_snapshot_age_ms bigint")
	require.Contains(t, lower, "jsonb_typeof(routing_attempts) = 'array') not valid")
	require.Contains(t, lower, "routing_snapshot_age_ms is null or routing_snapshot_age_ms >= 0) not valid")

	require.Contains(t, lower, "alter table usage_logs")
	require.Contains(t, lower, "add column if not exists route_plan_source text")
	require.Contains(t, lower, "add column if not exists route_plan_snapshot_age_ms bigint")
	require.Contains(t, lower, "route_plan_snapshot_age_ms is null or route_plan_snapshot_age_ms >= 0) not valid")

	require.Contains(t, lower, "c.conrelid = 'public.ops_error_logs'::regclass")
	require.Contains(t, lower, "c.conrelid = 'public.usage_logs'::regclass")
	require.Contains(t, lower, "c.connamespace = 'public'::regnamespace")
	require.NotContains(t, lower, "validate constraint")
	require.NotContains(t, lower, "create index")

	require.Contains(t, lower, "no keys, body, credentials, member identity, or upstream secrets")
	require.NotContains(t, lower, "api_key")
	require.NotContains(t, lower, "request_body")
}

func TestOpsRoutingAttemptsIndexesMigrationIsOnline(t *testing.T) {
	content, err := FS.ReadFile("201a_ops_routing_attempts_indexes_notx.sql")
	require.NoError(t, err)
	sql := string(content)
	lower := strings.ToLower(sql)

	require.Contains(t, lower, "create index concurrently if not exists idx_ops_error_logs_routing_plan_source_created")
	require.Contains(t, lower, "on ops_error_logs (routing_plan_source, created_at)")
	require.Contains(t, lower, "where routing_plan_source is not null")
	require.Contains(t, lower, "create index concurrently if not exists idx_usage_logs_route_plan_source_created")
	require.Contains(t, lower, "on usage_logs (route_plan_source, created_at)")
	require.Contains(t, lower, "where route_plan_source is not null")
	require.NotContains(t, lower, "begin")
	require.NotContains(t, lower, "commit")
	require.NotContains(t, lower, "rollback")
}
