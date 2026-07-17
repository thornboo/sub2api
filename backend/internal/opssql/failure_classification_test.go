package opssql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFailureClassificationExpressionsPreserveV2UnknownAndLegacyFallback(t *testing.T) {
	t.Parallel()

	require.Contains(t, CustomerVisible("e"), "COALESCE(e.status_code, 0) >= 400")
	require.Contains(t, CustomerVisible("e"), "cyber_policy_session_blocked")
	require.Contains(t, CustomerVisible("e"), "COALESCE(e.stream, FALSE)")
	require.Contains(t, CustomerVisible("e"), "NOT (COALESCE(e.status_code, 0) < 400")
	require.Contains(t, CustomerVisible("e"), "LIKE 'recovered %'")
	require.Contains(t, SLAImpact("e"), "COALESCE(e.classification_version, 0) >= 2 THEN e.sla_impact")
	require.Contains(t, SLAImpact("e"), "LIKE 'recovered %') THEN FALSE")
	require.Contains(t, SLAImpact("e"), "cyber_policy_session_blocked")
	require.Contains(t, SLAImpact("e"), "COALESCE(e.stream, FALSE)")
	require.Contains(t, SLAImpact("e"), "THEN NULL")
	require.Contains(t, SLAImpact("e"), "NOT COALESCE(e.is_business_limited, FALSE)")
	require.Contains(t, ClassificationUnknown("e"), "COALESCE(e.classification_version, 0) < 2")
	require.Contains(t, ClassificationUnknown("e"), "e.sla_impact IS NULL")
	require.Equal(t, "COALESCE(e.failure_domain = 'upstream', e.error_owner = 'provider')", UpstreamOwned("e"))
}

func TestLegacyClassificationExpressionsUseStrictRecoveredMarker(t *testing.T) {
	t.Parallel()

	require.Equal(t, "COALESCE(e.classification_version, 0) < 2", LegacyClassification("e"))
	require.Equal(t,
		"(COALESCE(e.status_code, 0) < 400 AND LOWER(COALESCE(e.error_phase, '')) IN ('upstream', 'account_auth') AND LOWER(COALESCE(e.error_message, '')) LIKE 'recovered %')",
		LegacyRecoveredAttempt("e"),
	)
}
