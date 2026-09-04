package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration234CreatesAuditableUpstreamExpectedCostEvidence(t *testing.T) {
	content, err := FS.ReadFile("234_usage_log_upstream_expected_cost.sql")
	require.NoError(t, err)

	sql := string(content)
	for _, column := range []string{
		"upstream_cost_binding_id",
		"upstream_group_multiplier",
		"upstream_price_reference_currency",
		"upstream_reference_fx_rate",
		"upstream_expected_cost",
	} {
		require.Contains(t, sql, column)
	}
	require.Contains(t, sql, "expected_cost_was_missing")
	require.Contains(t, sql, "UPDATE usage_dashboard_hourly SET account_cost = 0")
	require.Contains(t, sql, "UPDATE usage_dashboard_daily SET account_cost = 0")
	require.Contains(t, sql, "usage_logs_upstream_expected_cost_evidence_check")
}

func TestMigration235AddsDashboardUpstreamEvidenceCoverage(t *testing.T) {
	content, err := FS.ReadFile("235_dashboard_upstream_cost_evidence_coverage.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "upstream_expected_cost_count")
	require.Contains(t, sql, "missing_upstream_cost_count")
	require.Contains(t, sql, "missing_upstream_cost_count = total_requests")
	require.Contains(t, sql, "account_cost = 0")
	require.Contains(t, sql, "hourly_coverage_was_missing")
	require.Contains(t, sql, "daily_coverage_was_missing")
}

func TestMigration236BindsExplicitOfficialPricingAndNormalizesAggregates(t *testing.T) {
	content, err := FS.ReadFile("236_upstream_official_pricing_channel.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "official_pricing_channel_id")
	require.Contains(t, sql, "REFERENCES channels(id)")
	require.Contains(t, sql, "upstream_expected_cost_usd")
	require.Contains(t, sql, "upstream_expected_cost / upstream_reference_fx_rate")
}
