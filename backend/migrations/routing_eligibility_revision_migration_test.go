package migrations

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoutingEligibilityRevisionMigrationOwnsDurableAuthorityAndWriterCoverage(t *testing.T) {
	sqlBytes, err := os.ReadFile("199_routing_eligibility_revision.sql")
	require.NoError(t, err)
	sql := string(sqlBytes)

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS routing_eligibility_revisions")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS routing_eligibility_outbox")
	require.Contains(t, sql, "bump_routing_eligibility_revision")
	require.Contains(t, sql, "INSERT INTO routing_eligibility_outbox")

	for _, trigger := range []string{
		"routing_eligibility_channels_update",
		"routing_eligibility_channel_groups",
		"routing_eligibility_channel_model_pricing_update",
		"routing_eligibility_groups_update",
		"routing_eligibility_accounts_update",
		"routing_eligibility_account_groups",
		"routing_eligibility_protocol_update",
		"routing_eligibility_composite_update",
	} {
		require.Contains(t, sql, trigger)
	}

	// Transient account churn must not invalidate stable qualification. The
	// trigger watches selected JSON keys rather than comparing all credentials
	// or extra payloads, and does not subscribe to cooldown/last-used columns.
	require.Contains(t, sql, "OLD.credentials -> 'model_mapping'")
	require.Contains(t, sql, "OLD.credentials -> 'openai_capabilities'")
	require.Contains(t, sql, "OLD.extra -> 'privacy_mode'")
	require.NotContains(t, sql, "AFTER UPDATE OF last_used_at")
	require.NotContains(t, sql, "AFTER UPDATE OF rate_limited_at")
}
