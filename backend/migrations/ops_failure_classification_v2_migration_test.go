package migrations

import (
	"strings"
	"testing"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/opssql"
	"github.com/stretchr/testify/require"
)

func TestOpsFailureClassificationV2MigrationContract(t *testing.T) {
	content, err := FS.ReadFile("192_ops_failure_classification_v2.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	for _, column := range []string{
		"event_scope",
		"customer_visible",
		"failure_domain",
		"failure_category",
		"failure_reason",
		"resolution_owner",
		"pool_ownership",
		"sla_impact",
		"classification_version",
	} {
		require.Contains(t, sql, "add column if not exists "+column)
	}
	require.Contains(t, sql, "no_available_accounts")
	require.Contains(t, sql, "enterprise_member_budget_exhausted")
	require.Contains(t, sql, "client_cancelled")
	require.Contains(t, sql, "provider_4xx")
	require.Contains(t, sql, "provider_error_unknown")
	require.Contains(t, sql, "classification_version = 2")
	require.Contains(t, sql, "alter table ops_metrics_hourly")
	require.Contains(t, sql, "add column if not exists classification_version smallint not null default 1")

	cyber := "when err_type in ('cyber_policy', 'cyber_policy_session_blocked')"
	require.GreaterOrEqual(t, strings.Count(sql, cyber), 6, "all classification dimensions must preserve cyber-policy semantics")
	require.Contains(t, sql, cyber+" then 'customer'")
	require.Contains(t, sql, cyber+" then 'permission'")
	require.Contains(t, sql, cyber+" then 'endpoint_not_allowed'")
	require.Contains(t, sql, cyber+" then 'unknown'")
	require.Contains(t, sql, cyber+" then false")

	// Provider billing evidence is more specific than a generic 401/403, and
	// only 5xx statuses may receive the provider_5xx reason.
	requireSQLOrder(t, sql,
		"and (msg like '%balance%' or msg like '%quota%') then 'provider_balance_exhausted'",
		"then 'provider_auth_failed'",
		"then 'provider_5xx'",
		"then 'provider_4xx'",
		"then 'provider_error_unknown'",
	)
}

func TestOpsFailureClassificationV2IndexesMatchCompatibilityQueries(t *testing.T) {
	content, err := FS.ReadFile("193_ops_failure_classification_v2_indexes_notx.sql")
	require.NoError(t, err)
	sql := normalizeSQL(string(content))
	require.Contains(t, sql, "create index concurrently")
	compact := compactSQL(string(content))
	require.Contains(t, compact, compactSQL(opssql.CustomerVisible("")))
	require.Contains(t, compact, compactSQL(opssql.SLAImpact("")))
	require.Contains(t, compact, "where"+compactSQL(opssql.LegacyClassification("")))

	for _, name := range []string{
		"idx_ops_error_logs_failure_domain_time_v2",
		"idx_ops_error_logs_failure_category_time_v2",
		"idx_ops_error_logs_failure_reason_time_v2",
	} {
		statement := migrationStatementForIndex(t, sql, name)
		require.NotContains(t, statement, "where customer_visible is true")
		require.NotContains(t, statement, " where ", "dimension index %s must include rolling-upgrade rows", name)
	}
}

func requireSQLOrder(t *testing.T, sql string, fragments ...string) {
	t.Helper()
	previous := -1
	for _, fragment := range fragments {
		index := strings.Index(sql, fragment)
		require.Greater(t, index, previous, "SQL fragment missing or out of order: %s", fragment)
		previous = index
	}
}

func normalizeSQL(sql string) string {
	return strings.ToLower(strings.Join(strings.Fields(sql), " "))
}

func compactSQL(sql string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, sql))
}

func migrationStatementForIndex(t *testing.T, normalizedSQL, indexName string) string {
	t.Helper()
	start := strings.Index(normalizedSQL, "create index concurrently if not exists "+indexName)
	require.NotEqual(t, -1, start, "index %s not found", indexName)
	remaining := normalizedSQL[start:]
	end := strings.Index(remaining, ";")
	require.NotEqual(t, -1, end, "index %s statement is not terminated", indexName)
	return remaining[:end]
}
