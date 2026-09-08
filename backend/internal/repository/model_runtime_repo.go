package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/opssql"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type modelRuntimeRepository struct{ db *sql.DB }

func NewModelRuntimeRepository(db *sql.DB) service.ModelRuntimeRepository {
	return &modelRuntimeRepository{db: db}
}

func (r *modelRuntimeRepository) AggregateModelRuntime(ctx context.Context, start, end time.Time) ([]service.ModelRuntimeBucket, error) {
	rows, err := r.db.QueryContext(ctx, modelRuntimeAggregationSQL, start, end)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []service.ModelRuntimeBucket
	for rows.Next() {
		var row service.ModelRuntimeBucket
		if err := rows.Scan(&row.GroupID, &row.Model, &row.Hour, &row.Successes, &row.Failures,
			&row.FirstTokenSumMS, &row.FirstTokenCount, &row.DurationSumMS, &row.DurationCount,
			&row.OutputTokens, &row.GenerationTimeMS); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// Billing IDs are scoped to the API key. For ordinary HTTP calls the billing
// writer uses client:<client_request_id> or local:<site request_id>. An upstream
// request ID (notably WS turns and durable media jobs) is a different namespace:
// never guess a match by comparing it to the Ops site ID or by timestamp.
// Customer-visible terminal errors override partial billed usage, including
// client cancellations that are excluded from model availability statistics.
// Separate web_search money events are not additional model requests.
// Ordinary HTTP client_request_id is generated per request by the gateway;
// request_id can come from a reused X-Request-ID header. Prefer the generated ID
// for terminal deduplication, with separate namespaces for legacy fallback IDs.
var modelRuntimeAggregationSQL = `
WITH terminal_errors AS MATERIALIZED (
  SELECT DISTINCT ON (e.api_key_id,
    COALESCE('client:' || NULLIF(e.client_request_id, ''), 'local:' || NULLIF(e.request_id, ''), 'error:' || e.id::text))
    e.api_key_id, e.group_id, e.created_at,
    COALESCE(NULLIF(BTRIM(e.requested_model), ''), NULLIF(BTRIM(e.model), '')) AS model,
    NULLIF(e.request_id, '') AS local_id, NULLIF(e.client_request_id, '') AS client_id,
    (` + opssql.SLAImpact("e") + `) IS TRUE AS failed
  FROM ops_error_logs e
  WHERE e.created_at >= $1 AND e.created_at < $2
    AND e.api_key_id IS NOT NULL AND e.group_id > 0
    AND NOT COALESCE(e.is_count_tokens, FALSE)
    AND COALESCE(e.event_scope, '') <> 'upstream_attempt_recovered'
    AND ` + opssql.CustomerVisible("e") + `
  ORDER BY e.api_key_id,
    COALESCE('client:' || NULLIF(e.client_request_id, ''), 'local:' || NULLIF(e.request_id, ''), 'error:' || e.id::text),
    e.created_at DESC, e.id DESC
), terminal_ids AS (
  SELECT api_key_id, 'local:' || local_id AS billing_id FROM terminal_errors WHERE local_id IS NOT NULL
  UNION
  SELECT api_key_id, 'client:' || client_id FROM terminal_errors WHERE client_id IS NOT NULL
), usage_requests AS (
  SELECT DISTINCT ON (ul.api_key_id, COALESCE(NULLIF(ul.request_id, ''), 'usage:' || ul.id::text))
    ul.api_key_id, ul.request_id, ul.group_id, ul.created_at,
    COALESCE(NULLIF(BTRIM(ul.requested_model), ''), NULLIF(BTRIM(ul.model), '')) AS model,
    ul.first_token_ms, ul.duration_ms, ul.output_tokens,
    (ul.request_type IN (2, 3) OR (COALESCE(ul.request_type, 0) = 0 AND ul.stream)) AS streaming
  FROM usage_logs ul
  WHERE ul.created_at >= $1 AND ul.created_at < $2 AND ul.group_id > 0
    AND COALESCE(ul.request_type, 0) <> 4
    AND COALESCE(ul.request_id, '') NOT LIKE 'web_search:%'
    AND (` + usageLogSuccessFilterUL + ` OR ul.input_tokens > 0 OR ul.output_tokens > 0
      OR ul.cache_creation_tokens > 0 OR ul.cache_read_tokens > 0
      OR ul.image_input_tokens > 0 OR ul.image_output_tokens > 0 OR ul.image_count > 0 OR ul.video_count > 0)
  ORDER BY ul.api_key_id, COALESCE(NULLIF(ul.request_id, ''), 'usage:' || ul.id::text), ul.created_at DESC, ul.id DESC
), observations AS (
  SELECT u.group_id, u.model, u.created_at, TRUE AS succeeded,
    CASE WHEN u.streaming AND u.first_token_ms > 0 THEN u.first_token_ms END AS ttft,
    CASE WHEN u.duration_ms > 0 THEN u.duration_ms END AS duration,
    CASE WHEN u.streaming AND u.output_tokens > 0 AND u.first_token_ms > 0 AND u.duration_ms > u.first_token_ms
      THEN u.output_tokens END AS output_tokens,
    CASE WHEN u.streaming AND u.output_tokens > 0 AND u.first_token_ms > 0 AND u.duration_ms > u.first_token_ms
      THEN u.duration_ms - u.first_token_ms END AS generation_ms
  FROM usage_requests u
  WHERE NOT EXISTS (
    SELECT 1 FROM terminal_ids e WHERE e.api_key_id = u.api_key_id AND e.billing_id = u.request_id
  )
  UNION ALL
  SELECT group_id, model, created_at, FALSE, NULL, NULL, NULL, NULL
  FROM terminal_errors WHERE failed
)
SELECT group_id, model,
  FLOOR(EXTRACT(EPOCH FROM (created_at - $1::timestamptz)) / 3600)::int AS hour,
  COUNT(*) FILTER (WHERE succeeded), COUNT(*) FILTER (WHERE NOT succeeded),
  COALESCE(SUM(ttft), 0), COUNT(ttft), COALESCE(SUM(duration), 0), COUNT(duration),
  COALESCE(SUM(output_tokens), 0), COALESCE(SUM(generation_ms), 0)
FROM observations WHERE model IS NOT NULL
GROUP BY 1, 2, 3`
