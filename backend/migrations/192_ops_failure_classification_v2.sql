-- Ops failure classification v2: separate customer visibility, attribution,
-- remediation ownership, and platform SLA impact.
--
-- The last 31 days are deterministically backfilled because the dashboard and
-- scheduled reports do not query beyond that operational retention window.
-- Ambiguous rows remain explicitly unknown instead of being guessed.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS event_scope VARCHAR(32),
    ADD COLUMN IF NOT EXISTS customer_visible BOOLEAN,
    ADD COLUMN IF NOT EXISTS failure_domain VARCHAR(32),
    ADD COLUMN IF NOT EXISTS failure_category VARCHAR(32),
    ADD COLUMN IF NOT EXISTS failure_reason VARCHAR(64),
    ADD COLUMN IF NOT EXISTS resolution_owner VARCHAR(32),
    ADD COLUMN IF NOT EXISTS pool_ownership VARCHAR(16),
    ADD COLUMN IF NOT EXISTS sla_impact BOOLEAN,
    ADD COLUMN IF NOT EXISTS classification_version SMALLINT;

COMMENT ON COLUMN ops_error_logs.event_scope IS 'request_terminal, stream_terminal, or upstream_attempt_recovered';
COMMENT ON COLUMN ops_error_logs.customer_visible IS 'Whether the logical request ultimately failed for the client';
COMMENT ON COLUMN ops_error_logs.failure_domain IS 'customer, enterprise, client, platform, upstream, or unknown';
COMMENT ON COLUMN ops_error_logs.failure_category IS 'Stable v2 failure category';
COMMENT ON COLUMN ops_error_logs.failure_reason IS 'Stable v2 failure reason code';
COMMENT ON COLUMN ops_error_logs.resolution_owner IS 'Primary actor able to remediate the failure';
COMMENT ON COLUMN ops_error_logs.pool_ownership IS 'Snapshot of platform or enterprise ownership for routing/account-pool failures';
COMMENT ON COLUMN ops_error_logs.sla_impact IS 'TRUE when the terminal customer-visible failure counts against platform SLA; NULL means unknown';
COMMENT ON COLUMN ops_error_logs.classification_version IS 'Writer/backfill classification contract version';

ALTER TABLE ops_metrics_hourly
    ADD COLUMN IF NOT EXISTS classification_version SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS customer_visible_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_sla_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sla_excluded_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS classification_unknown_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS customer_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS enterprise_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS client_request_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS client_transport_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_routing_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_internal_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_terminal_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_recovered_attempt_count BIGINT NOT NULL DEFAULT 0;

ALTER TABLE ops_metrics_daily
    ADD COLUMN IF NOT EXISTS classification_version SMALLINT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS customer_visible_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_sla_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS sla_excluded_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS classification_unknown_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS customer_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS enterprise_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS client_request_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS client_transport_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_routing_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS platform_internal_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_terminal_failure_count BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS upstream_recovered_attempt_count BIGINT NOT NULL DEFAULT 0;

-- Preserve dashboard continuity for buckets produced before v2. The detailed
-- breakdown is repopulated by the normal aggregation job; the three headline
-- counters can be mapped losslessly from the legacy columns immediately.
UPDATE ops_metrics_hourly
SET customer_visible_failure_count = error_count_total,
    platform_sla_failure_count = error_count_sla,
    sla_excluded_failure_count = business_limited_count
WHERE customer_visible_failure_count = 0
  AND platform_sla_failure_count = 0
  AND sla_excluded_failure_count = 0
  AND (error_count_total <> 0 OR error_count_sla <> 0 OR business_limited_count <> 0);

UPDATE ops_metrics_daily
SET customer_visible_failure_count = error_count_total,
    platform_sla_failure_count = error_count_sla,
    sla_excluded_failure_count = business_limited_count
WHERE customer_visible_failure_count = 0
  AND platform_sla_failure_count = 0
  AND sla_excluded_failure_count = 0
  AND (error_count_total <> 0 OR error_count_sla <> 0 OR business_limited_count <> 0);

WITH candidates AS (
    SELECT
        id,
        LOWER(COALESCE(error_message, '')) AS msg,
        UPPER(COALESCE(error_type, '')) AS err_type,
        LOWER(COALESCE(error_phase, '')) AS phase,
        LOWER(COALESCE(error_owner, '')) AS owner,
        COALESCE(status_code, 0) AS client_status,
        COALESCE(upstream_status_code, status_code, 0) AS effective_status,
        COALESCE(stream, FALSE) AS is_stream,
        member_id,
        COALESCE(is_business_limited, FALSE) AS legacy_excluded
    FROM ops_error_logs
    WHERE classification_version IS NULL
      AND created_at >= NOW() - INTERVAL '31 days'
), classified AS (
    SELECT
        id,
        CASE
            WHEN client_status < 400 AND phase IN ('upstream', 'account_auth') AND msg LIKE 'recovered %'
                THEN 'upstream_attempt_recovered'
            WHEN is_stream AND client_status < 400 THEN 'stream_terminal'
            ELSE 'request_terminal'
        END AS event_scope,
        NOT (client_status < 400 AND phase IN ('upstream', 'account_auth') AND msg LIKE 'recovered %') AS customer_visible,
        CASE
            WHEN err_type IN ('CYBER_POLICY', 'CYBER_POLICY_SESSION_BLOCKED') THEN 'customer'
            WHEN phase = 'routing' OR msg LIKE '%no available account%' THEN 'platform'
            WHEN msg LIKE '%enterprise_member_budget_exceeded%'
              OR msg LIKE '%enterprise member monthly budget is exhausted%'
              OR msg LIKE '%enterprise_member_rate_%'
              OR msg LIKE '%enterprise member 5-hour spending limit is exhausted%'
              OR msg LIKE '%enterprise member daily spending limit is exhausted%'
              OR msg LIKE '%enterprise member 7-day spending limit is exhausted%' THEN 'enterprise'
            WHEN client_status = 499 OR msg LIKE '%context canceled%' OR msg LIKE '%client disconnected%' THEN 'client'
            WHEN phase IN ('upstream', 'account_auth') OR owner = 'provider' THEN 'upstream'
            WHEN phase = 'internal' OR owner = 'platform' OR client_status >= 500 THEN 'platform'
            WHEN phase IN ('request', 'auth') OR legacy_excluded THEN CASE WHEN member_id IS NOT NULL THEN 'enterprise' ELSE 'customer' END
            WHEN client_status >= 400 THEN 'client'
            ELSE 'unknown'
        END AS failure_domain,
        CASE
            WHEN err_type IN ('CYBER_POLICY', 'CYBER_POLICY_SESSION_BLOCKED') THEN 'permission'
            WHEN phase = 'routing' OR msg LIKE '%no available account%' THEN 'routing_capacity'
            WHEN msg LIKE '%enterprise_member_budget_exceeded%' OR msg LIKE '%monthly budget is exhausted%' THEN 'budget'
            WHEN msg LIKE '%enterprise_member_rate_%' OR msg LIKE '%spending limit is exhausted%' THEN 'rate_limit'
            WHEN client_status = 499 OR msg LIKE '%context canceled%' THEN 'cancellation'
            WHEN msg LIKE '%client disconnected%' THEN 'network'
            WHEN (phase IN ('upstream', 'account_auth') OR owner = 'provider')
              AND (msg LIKE '%balance%' OR msg LIKE '%quota%') THEN 'balance'
            WHEN phase = 'account_auth' OR effective_status IN (401, 403) AND owner = 'provider' THEN 'credential'
            WHEN effective_status = 429 AND (phase = 'upstream' OR owner = 'provider') THEN 'rate_limit'
            WHEN effective_status = 529 AND (phase = 'upstream' OR owner = 'provider') THEN 'overload'
            WHEN (msg LIKE '%timeout%' OR msg LIKE '%deadline exceeded%')
              AND (phase = 'upstream' OR owner = 'provider') THEN 'timeout'
            WHEN effective_status IN (408, 504) AND (phase = 'upstream' OR owner = 'provider') THEN 'timeout'
            WHEN (msg LIKE '%network%' OR msg LIKE '%connection reset%' OR msg LIKE '%connection refused%')
              AND (phase = 'upstream' OR owner = 'provider') THEN 'network'
            WHEN effective_status >= 500 AND (phase = 'upstream' OR owner = 'provider') THEN 'internal'
            WHEN effective_status BETWEEN 400 AND 499 AND (phase = 'upstream' OR owner = 'provider') THEN 'protocol'
            WHEN phase = 'upstream' OR owner = 'provider' THEN 'unknown'
            WHEN msg LIKE '%balance%' THEN 'balance'
            WHEN msg LIKE '%quota%' OR msg LIKE '%额度已用完%' THEN 'quota'
            WHEN msg LIKE '%concurrency%' OR msg LIKE '%pending requests%' THEN 'concurrency'
            WHEN phase = 'auth' THEN 'authentication'
            WHEN phase = 'request' OR client_status BETWEEN 400 AND 499 THEN 'protocol'
            WHEN phase = 'internal' OR client_status >= 500 THEN 'internal'
            ELSE 'unknown'
        END AS failure_category,
        CASE
            WHEN err_type IN ('CYBER_POLICY', 'CYBER_POLICY_SESSION_BLOCKED') THEN 'endpoint_not_allowed'
            WHEN phase = 'routing' OR msg LIKE '%no available account%' THEN 'no_available_accounts'
            WHEN msg LIKE '%enterprise_member_budget_exceeded%' OR msg LIKE '%monthly budget is exhausted%' THEN 'enterprise_member_budget_exhausted'
            WHEN msg LIKE '%enterprise_member_rate_%' OR msg LIKE '%spending limit is exhausted%' THEN 'enterprise_member_rate_limit_exceeded'
            WHEN client_status = 499 OR msg LIKE '%context canceled%' THEN 'client_cancelled'
            WHEN msg LIKE '%client disconnected%' THEN 'client_disconnected'
            WHEN (phase IN ('upstream', 'account_auth') OR owner = 'provider')
              AND (msg LIKE '%balance%' OR msg LIKE '%quota%') THEN 'provider_balance_exhausted'
            WHEN phase = 'account_auth' OR effective_status IN (401, 403) AND owner = 'provider' THEN 'provider_auth_failed'
            WHEN effective_status = 429 AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_rate_limited'
            WHEN effective_status = 529 AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_overloaded'
            WHEN (msg LIKE '%timeout%' OR msg LIKE '%deadline exceeded%')
              AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_timeout'
            WHEN effective_status IN (408, 504) AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_timeout'
            WHEN (msg LIKE '%network%' OR msg LIKE '%connection reset%' OR msg LIKE '%connection refused%')
              AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_network_error'
            WHEN effective_status >= 500 AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_5xx'
            WHEN effective_status BETWEEN 400 AND 499 AND (phase = 'upstream' OR owner = 'provider') THEN 'provider_4xx'
            WHEN phase = 'upstream' OR owner = 'provider' THEN 'provider_error_unknown'
            WHEN msg LIKE '%insufficient%balance%' THEN 'user_balance_exhausted'
            WHEN msg LIKE '%api key%额度已用完%' THEN 'api_key_quota_exhausted'
            WHEN msg LIKE '%quota%' THEN 'user_quota_exhausted'
            WHEN msg LIKE '%concurrency%' OR msg LIKE '%pending requests%' THEN 'concurrency_exceeded'
            WHEN phase = 'auth' THEN 'api_key_invalid'
            WHEN phase = 'request' OR client_status BETWEEN 400 AND 499 THEN 'invalid_request'
            WHEN phase = 'internal' OR client_status >= 500 THEN 'internal_error'
            ELSE 'legacy_unknown'
        END AS failure_reason,
        CASE
            WHEN err_type IN ('CYBER_POLICY', 'CYBER_POLICY_SESSION_BLOCKED') THEN 'customer'
            WHEN phase = 'routing' OR msg LIKE '%no available account%' THEN 'platform_ops'
            WHEN msg LIKE '%enterprise_member_%' OR msg LIKE '%enterprise member %' OR member_id IS NOT NULL AND phase IN ('request', 'auth') THEN 'enterprise_admin'
            WHEN client_status = 499 OR msg LIKE '%context canceled%' OR msg LIKE '%client disconnected%' THEN 'client'
            WHEN phase IN ('upstream', 'account_auth') OR owner = 'provider' THEN 'platform_ops'
            WHEN phase = 'internal' OR owner = 'platform' OR client_status >= 500 THEN 'platform_ops'
            WHEN phase IN ('request', 'auth') OR legacy_excluded THEN 'customer'
            WHEN client_status >= 400 THEN 'client'
            ELSE 'unknown'
        END AS resolution_owner,
        CASE
            WHEN err_type IN ('CYBER_POLICY', 'CYBER_POLICY_SESSION_BLOCKED') THEN 'unknown'
            WHEN phase IN ('routing', 'upstream', 'account_auth') OR owner = 'provider' OR msg LIKE '%no available account%' THEN 'platform'
            ELSE 'unknown'
        END AS pool_ownership,
        CASE
            WHEN client_status < 400 AND phase IN ('upstream', 'account_auth') AND msg LIKE 'recovered %' THEN FALSE
            WHEN err_type IN ('CYBER_POLICY', 'CYBER_POLICY_SESSION_BLOCKED') THEN FALSE
            WHEN phase = 'routing' OR msg LIKE '%no available account%' THEN TRUE
            WHEN msg LIKE '%enterprise_member_%' OR msg LIKE '%enterprise member %' THEN FALSE
            WHEN client_status = 499 OR msg LIKE '%context canceled%' OR msg LIKE '%client disconnected%' THEN FALSE
            WHEN phase IN ('upstream', 'account_auth') OR owner = 'provider' THEN TRUE
            WHEN phase = 'internal' OR owner = 'platform' OR client_status >= 500 THEN TRUE
            WHEN phase IN ('request', 'auth') OR legacy_excluded OR client_status BETWEEN 400 AND 499 THEN FALSE
            ELSE NULL
        END AS sla_impact
    FROM candidates
)
UPDATE ops_error_logs target
SET event_scope = classified.event_scope,
    customer_visible = classified.customer_visible,
    failure_domain = classified.failure_domain,
    failure_category = classified.failure_category,
    failure_reason = classified.failure_reason,
    resolution_owner = classified.resolution_owner,
    pool_ownership = classified.pool_ownership,
    sla_impact = classified.sla_impact,
    classification_version = 2,
    is_business_limited = CASE
        WHEN classified.sla_impact IS FALSE AND classified.customer_visible THEN TRUE
        WHEN classified.sla_impact IS TRUE THEN FALSE
        ELSE target.is_business_limited
    END
FROM classified
WHERE target.id = classified.id;
