-- Concurrent indexes for the v2 dashboard/detail query paths.
-- This file must run outside a transaction (the migration runner recognizes _notx).
--
-- Compatibility expressions intentionally mirror internal/opssql exactly. A
-- partial index on only `customer_visible IS TRUE` cannot serve rolling-upgrade
-- queries because those queries must also surface legacy HTTP status failures,
-- cyber-policy outcomes, and non-recovered HTTP 200 stream-terminal failures.

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_customer_visible_time_v2
    ON ops_error_logs (created_at DESC)
    WHERE COALESCE(customer_visible, (
        COALESCE(status_code, 0) >= 400
        OR LOWER(COALESCE(error_type, '')) IN ('cyber_policy', 'cyber_policy_session_blocked')
        OR (
            COALESCE(status_code, 0) < 400
            AND COALESCE(stream, FALSE)
            AND NOT (
                COALESCE(status_code, 0) < 400
                AND LOWER(COALESCE(error_phase, '')) IN ('upstream', 'account_auth')
                AND LOWER(COALESCE(error_message, '')) LIKE 'recovered %'
            )
        )
    ));

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_sla_impact_time_v2
    ON ops_error_logs ((
        CASE
            WHEN COALESCE(classification_version, 0) >= 2 THEN sla_impact
            WHEN (
                COALESCE(status_code, 0) < 400
                AND LOWER(COALESCE(error_phase, '')) IN ('upstream', 'account_auth')
                AND LOWER(COALESCE(error_message, '')) LIKE 'recovered %'
            ) THEN FALSE
            WHEN LOWER(COALESCE(error_type, '')) IN ('cyber_policy', 'cyber_policy_session_blocked') THEN FALSE
            WHEN (
                COALESCE(status_code, 0) < 400
                AND COALESCE(stream, FALSE)
                AND NOT (
                    COALESCE(status_code, 0) < 400
                    AND LOWER(COALESCE(error_phase, '')) IN ('upstream', 'account_auth')
                    AND LOWER(COALESCE(error_message, '')) LIKE 'recovered %'
                )
            ) THEN NULL
            ELSE COALESCE(status_code, 0) >= 400
                AND NOT COALESCE(is_business_limited, FALSE)
        END
    ), created_at DESC);

-- These dimensions are nullable for rolling-upgrade rows, so raw-column
-- partial predicates would exclude compatibility results. Keep them complete.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_failure_domain_time_v2
    ON ops_error_logs (failure_domain, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_failure_category_time_v2
    ON ops_error_logs (failure_category, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_failure_reason_time_v2
    ON ops_error_logs (failure_reason, created_at DESC);

-- The pre-aggregation fallback probe uses this exact predicate to detect any
-- stale replica that still writes v1 rows into an otherwise-v2 time bucket.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_error_logs_legacy_classification_time_v2
    ON ops_error_logs (created_at DESC)
    WHERE COALESCE(classification_version, 0) < 2;
