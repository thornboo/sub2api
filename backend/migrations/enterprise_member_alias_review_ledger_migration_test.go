package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAliasReviewLedgerMigrationIsReviewOnly(t *testing.T) {
	sqlBytes, err := os.ReadFile("200_enterprise_member_alias_review_ledger.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS enterprise_member_model_alias_reviews")
	require.Contains(t, sql, "status IN ('pending', 'registered', 'rejected_invalid', 'obsolete', 'needs_owner_action')")
	require.Contains(t, sql, "UNIQUE (public_model_norm, endpoint)")
	require.Contains(t, sql, "public_model !~ '[[:cntrl:]]'")
	require.Contains(t, sql, "char_length(review_note) <= 1000")
	require.Contains(t, sql, "review_note !~ '[[:cntrl:]]'")
	require.Contains(t, sql, "validation_evidence JSONB NOT NULL DEFAULT '{}'::jsonb")
	require.Contains(t, sql, "review state is not a routing authority")
	require.NotContains(t, sql, "usage_logs ADD COLUMN")
	require.NotContains(t, sql, "ops_error_logs")
	require.NotContains(t, sql, "routing_eligibility")
}
