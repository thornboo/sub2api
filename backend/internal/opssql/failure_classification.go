package opssql

// This package owns the SQL-side compatibility contract for Ops failure
// classification. Writers persist v2 fields, while readers must continue to
// interpret pre-v2 rows during the migration window. Keeping these expressions
// centralized prevents raw dashboards, trends, collectors, and pre-aggregation
// from silently drifting to different SLA semantics.

func column(alias, name string) string {
	if alias == "" {
		return name
	}
	return alias + "." + name
}

// LegacyClassification identifies rows written before the structured v2
// contract. Keep this predicate text stable: the repository's fallback probe
// and migration 193's partial index deliberately use the same expression.
func LegacyClassification(alias string) string {
	return "COALESCE(" + column(alias, "classification_version") + ", 0) < 2"
}

// LegacyRecoveredAttempt recognizes the only pre-v2 HTTP 200 stream record
// that is known not to be a terminal customer failure. Older replicas wrote
// recovered upstream attempts with a stable phase/message marker.
func LegacyRecoveredAttempt(alias string) string {
	return "(COALESCE(" + column(alias, "status_code") + ", 0) < 400" +
		" AND LOWER(COALESCE(" + column(alias, "error_phase") + ", '')) IN ('upstream', 'account_auth')" +
		" AND LOWER(COALESCE(" + column(alias, "error_message") + ", '')) LIKE 'recovered %')"
}

func legacyCyberPolicy(alias string) string {
	return "LOWER(COALESCE(" + column(alias, "error_type") + ", '')) IN ('cyber_policy', 'cyber_policy_session_blocked')"
}

func legacyStreamTerminal(alias string) string {
	return "(COALESCE(" + column(alias, "status_code") + ", 0) < 400" +
		" AND COALESCE(" + column(alias, "stream") + ", FALSE)" +
		" AND NOT " + LegacyRecoveredAttempt(alias) + ")"
}

func CustomerVisible(alias string) string {
	legacyVisible := "(COALESCE(" + column(alias, "status_code") + ", 0) >= 400" +
		" OR " + legacyCyberPolicy(alias) +
		" OR " + legacyStreamTerminal(alias) + ")"
	return "COALESCE(" + column(alias, "customer_visible") + ", " + legacyVisible + ")"
}

// SLAImpact returns a nullable boolean. V2 NULL is intentionally preserved as
// unknown. Legacy recovered attempts and cyber-policy outcomes are
// deterministic; other HTTP 200 stream-terminal rows remain NULL because an
// old replica did not persist enough evidence to assign SLA responsibility.
func SLAImpact(alias string) string {
	return "CASE WHEN COALESCE(" + column(alias, "classification_version") + ", 0) >= 2 THEN " + column(alias, "sla_impact") +
		" WHEN " + LegacyRecoveredAttempt(alias) + " THEN FALSE" +
		" WHEN " + legacyCyberPolicy(alias) + " THEN FALSE" +
		" WHEN " + legacyStreamTerminal(alias) + " THEN NULL" +
		" ELSE COALESCE(" + column(alias, "status_code") + ", 0) >= 400 AND NOT COALESCE(" + column(alias, "is_business_limited") + ", FALSE) END"
}

// ClassificationUnknown covers both an explicit v2 unknown SLA decision and
// a legacy row that has no structured attribution. Legacy SLA compatibility
// can still produce a headline value, but it must not hide the data-quality gap.
func ClassificationUnknown(alias string) string {
	return "(" + CustomerVisible(alias) + " AND (" + LegacyClassification(alias) + " OR " + column(alias, "sla_impact") + " IS NULL))"
}

func UpstreamOwned(alias string) string {
	return "COALESCE(" + column(alias, "failure_domain") + " = 'upstream', " + column(alias, "error_owner") + " = 'provider')"
}

func EffectiveStatus(alias string) string {
	return "COALESCE(" + column(alias, "upstream_status_code") + ", " + column(alias, "status_code") + ", 0)"
}
