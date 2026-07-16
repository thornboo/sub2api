package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberFractionalTokenBaselinesMigration(t *testing.T) {
	content, err := FS.ReadFile("191_enterprise_member_fractional_token_baselines.sql")
	require.NoError(t, err)
	sql := string(content)

	for _, column := range []string{
		"total_tokens",
		"input_tokens",
		"output_tokens",
		"cache_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
	} {
		require.Contains(t, sql, "ALTER COLUMN "+column+" TYPE NUMERIC(21,2)")
		require.Contains(t, sql, column+" <= 9223372036854775807.99")
	}
	require.Contains(t, sql, "DROP CONSTRAINT IF EXISTS enterprise_member_import_usage_baselines_values_check")
	require.Contains(t, sql, "ADD CONSTRAINT enterprise_member_import_usage_baselines_values_check")
}
