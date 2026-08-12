package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type enterpriseMemberAdmissionEvidenceRepository struct {
	db *sql.DB
}

func NewEnterpriseMemberAdmissionEvidenceRepository(db *sql.DB) service.EnterpriseMemberAdmissionEvidenceRepository {
	return &enterpriseMemberAdmissionEvidenceRepository{db: db}
}

func (r *enterpriseMemberAdmissionEvidenceRepository) GetEnterpriseMemberAdmissionEvidence(ctx context.Context, input service.EnterpriseMemberAdmissionEvidenceInput) (*service.EnterpriseMemberAdmissionEvidenceSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member admission evidence repository db is nil")
	}
	input = service.NormalizeEnterpriseMemberAdmissionEvidenceInput(input)
	fullEnd := time.Date(input.Now.Year(), input.Now.Month(), input.Now.Day(), 0, 0, 0, 0, time.UTC)
	fullStart := fullEnd.AddDate(0, 0, -input.ShadowFullDays)
	lookbackStart := input.Now.AddDate(0, 0, -input.LookbackDays)

	summary := &service.EnterpriseMemberAdmissionEvidenceSummary{
		GeneratedAt:    input.Now,
		ShadowFullDays: input.ShadowFullDays,
		FullDaysStart:  fullStart,
		FullDaysEnd:    fullEnd,
		LookbackStart:  lookbackStart,
		LookbackEnd:    input.Now,
	}
	var err error
	if summary.DailyShadowEvidence, err = r.getAdmissionDailyShadowEvidence(ctx, fullStart, fullEnd); err != nil {
		return nil, err
	}
	if err = r.db.QueryRowContext(ctx, enterpriseMemberAdmissionAliasAuditSQL, lookbackStart, input.Now).Scan(
		&summary.AliasAudit.ActiveLegacySuccessNewPruned30d,
		&summary.AliasAudit.UnreviewedActive30d,
	); err != nil {
		return nil, fmt.Errorf("get enterprise member admission alias audit: %w", err)
	}
	if err = r.db.QueryRowContext(ctx, enterpriseMemberAdmissionShadowKeptBaselineSQL, lookbackStart, input.Now).Scan(
		&summary.ShadowKeptBaseline.Successes,
		&summary.ShadowKeptBaseline.DistinctEndpoints,
		&summary.ShadowKeptBaseline.DistinctEnterpriseOwners,
	); err != nil {
		return nil, fmt.Errorf("get enterprise member admission shadow kept baseline: %w", err)
	}
	if err = r.db.QueryRowContext(ctx, enterpriseMemberAdmissionCanarySQL, lookbackStart, input.Now).Scan(
		&summary.CanaryComparison.CanarySuccesses,
		&summary.CanaryComparison.CanaryErrors,
		&summary.CanaryComparison.ControlSuccesses,
		&summary.CanaryComparison.ControlErrors,
	); err != nil {
		return nil, fmt.Errorf("get enterprise member admission canary comparison: %w", err)
	}
	if err = r.db.QueryRowContext(ctx, enterpriseMemberAdmissionUnpublishedGuardSQL, lookbackStart, input.Now).Scan(
		&summary.UnpublishedGuard.ModelUnpublishedNoCandidateErrors,
		&summary.UnpublishedGuard.ActualAttemptViolations,
	); err != nil {
		return nil, fmt.Errorf("get enterprise member admission unpublished guard: %w", err)
	}
	if err = r.getAdmissionPlannerHealth(ctx, lookbackStart, input.Now, &summary.PlannerHealth); err != nil {
		return nil, err
	}
	if err = r.db.QueryRowContext(ctx, enterpriseMemberAdmissionLKGSQL, lookbackStart, input.Now).Scan(
		&summary.LKG.LiveSuccesses,
		&summary.LKG.LKGSuccesses,
		&summary.LKG.LKGErrors,
		&summary.LKG.LKGMisses,
		&summary.LKG.LKGStaleOrExpired,
		&summary.LKG.LKGGenerationMismatch,
	); err != nil {
		return nil, fmt.Errorf("get enterprise member admission lkg evidence: %w", err)
	}
	return service.EvaluateEnterpriseMemberAdmissionEvidenceSummary(summary, input), nil
}

func (r *enterpriseMemberAdmissionEvidenceRepository) getAdmissionDailyShadowEvidence(ctx context.Context, start, end time.Time) ([]service.EnterpriseMemberAdmissionDailyShadowEvidence, error) {
	rows, err := r.db.QueryContext(ctx, enterpriseMemberAdmissionDailyShadowSQL, start, end)
	if err != nil {
		return nil, fmt.Errorf("get enterprise member admission daily shadow evidence: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.EnterpriseMemberAdmissionDailyShadowEvidence, 0, int(end.Sub(start).Hours()/24))
	for rows.Next() {
		var item service.EnterpriseMemberAdmissionDailyShadowEvidence
		if err := rows.Scan(&item.DayStart, &item.ShadowSamples, &item.LegacySuccessNewPruned, &item.UnreviewedLegacySuccessNewPruned); err != nil {
			return nil, fmt.Errorf("scan enterprise member admission daily shadow evidence: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enterprise member admission daily shadow evidence: %w", err)
	}
	return out, nil
}

func (r *enterpriseMemberAdmissionEvidenceRepository) getAdmissionPlannerHealth(ctx context.Context, start, end time.Time, out *service.EnterpriseMemberAdmissionPlannerHealth) error {
	if out == nil {
		return nil
	}
	var p95, p99 sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, enterpriseMemberAdmissionPlannerHealthSQL, start, end).Scan(
		&out.Evaluations,
		&out.EvaluationFailed,
		&out.LatencySamples,
		&p95,
		&p99,
	); err != nil {
		return fmt.Errorf("get enterprise member admission planner health: %w", err)
	}
	if p95.Valid {
		out.P95Ms = int64(math.Ceil(p95.Float64))
	}
	if p99.Valid {
		out.P99Ms = int64(math.Ceil(p99.Float64))
	}
	return nil
}

const enterpriseMemberAdmissionDailyShadowSQL = `
WITH days AS (
	SELECT generate_series($1::timestamptz, $2::timestamptz - interval '1 day', interval '1 day') AS day_start
), shadow_usage AS (
	SELECT
		date_trunc('day', ul.created_at) AS day_start,
		BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model)) AS public_model,
		LOWER(BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model))) AS public_model_norm,
		COALESCE(NULLIF(BTRIM(ul.inbound_endpoint), ''), '') AS endpoint,
		ul.schedule_meta
	FROM usage_logs ul
	WHERE ul.created_at >= $1
	  AND ul.created_at < $2
	  AND ul.member_id IS NOT NULL
	  AND COALESCE(LOWER(ul.schedule_meta->>'shadow_plan_evaluated'), '') IN ('true', '1', 'yes')
), daily AS (
	SELECT
		su.day_start,
		COUNT(*) AS shadow_samples,
		COUNT(*) FILTER (WHERE COALESCE(su.schedule_meta->>'shadow_diff_type', '') = 'legacy_success_new_pruned') AS legacy_success_new_pruned,
		COUNT(*) FILTER (
			WHERE COALESCE(su.schedule_meta->>'shadow_diff_type', '') = 'legacy_success_new_pruned'
			  AND COALESCE(rv.status, 'pending') = 'pending'
		) AS unreviewed_legacy_success_new_pruned
	FROM shadow_usage su
	LEFT JOIN enterprise_member_model_alias_reviews rv
	  ON rv.public_model_norm = su.public_model_norm AND rv.endpoint = su.endpoint
	GROUP BY su.day_start
)
SELECT
	d.day_start,
	COALESCE(daily.shadow_samples, 0) AS shadow_samples,
	COALESCE(daily.legacy_success_new_pruned, 0) AS legacy_success_new_pruned,
	COALESCE(daily.unreviewed_legacy_success_new_pruned, 0) AS unreviewed_legacy_success_new_pruned
FROM days d
LEFT JOIN daily ON daily.day_start = d.day_start
ORDER BY d.day_start`

const enterpriseMemberAdmissionAliasAuditSQL = `
WITH alias_usage AS (
	SELECT
		LOWER(BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model))) AS public_model_norm,
		COALESCE(NULLIF(BTRIM(ul.inbound_endpoint), ''), '') AS endpoint
	FROM usage_logs ul
	WHERE ul.created_at >= $1
	  AND ul.created_at < $2
	  AND ul.member_id IS NOT NULL
	  AND BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model)) <> ''
	  AND COALESCE(ul.schedule_meta->>'shadow_diff_type', '') = 'legacy_success_new_pruned'
	GROUP BY 1, 2
)
SELECT
	COUNT(*) AS active_30d,
	COUNT(*) FILTER (WHERE COALESCE(rv.status, 'pending') = 'pending') AS unreviewed_30d
FROM alias_usage au
LEFT JOIN enterprise_member_model_alias_reviews rv
  ON rv.public_model_norm = au.public_model_norm AND rv.endpoint = au.endpoint`

const enterpriseMemberAdmissionShadowKeptBaselineSQL = `
SELECT
	COUNT(*) AS successes,
	COUNT(DISTINCT COALESCE(NULLIF(BTRIM(inbound_endpoint), ''), '')) AS distinct_endpoints,
	COUNT(DISTINCT user_id) AS distinct_enterprise_owners
FROM usage_logs
WHERE created_at >= $1
  AND created_at < $2
  AND member_id IS NOT NULL
  AND COALESCE(LOWER(schedule_meta->>'shadow_plan_evaluated'), '') IN ('true', '1', 'yes')
  AND COALESCE(LOWER(schedule_meta->>'shadow_group_kept'), '') IN ('true', '1', 'yes')`

const enterpriseMemberAdmissionCanarySQL = `
WITH canary_success AS (
	SELECT COUNT(*) AS count
	FROM usage_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND route_plan_source IN ('live', 'last_known_good')
), canary_error AS (
	SELECT COUNT(*) AS count
	FROM ops_error_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND routing_plan_source IN ('live', 'last_known_good')
), control_success AS (
	SELECT COUNT(*) AS count
	FROM usage_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND COALESCE(route_plan_source, '') = ''
	  AND COALESCE(LOWER(schedule_meta->>'shadow_plan_evaluated'), '') IN ('true', '1', 'yes')
	  AND COALESCE(LOWER(schedule_meta->>'shadow_group_kept'), '') IN ('true', '1', 'yes')
), control_error AS (
	SELECT COUNT(*) AS count
	FROM ops_error_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND COALESCE(routing_plan_source, '') = ''
	  AND failure_domain IN ('platform', 'upstream')
)
SELECT
	(SELECT count FROM canary_success),
	(SELECT count FROM canary_error),
	(SELECT count FROM control_success),
	(SELECT count FROM control_error)`

const enterpriseMemberAdmissionUnpublishedGuardSQL = `
WITH unpublished_errors AS (
	SELECT e.id, e.routing_attempts
	FROM ops_error_logs e
	WHERE e.created_at >= $1
	  AND e.created_at < $2
	  AND e.member_id IS NOT NULL
	  AND EXISTS (
		SELECT 1
		FROM jsonb_array_elements(e.routing_attempts) AS attempt
		WHERE attempt->>'reason' = 'model_unpublished'
	  )
)
SELECT
	COUNT(*) AS model_unpublished_no_candidate_errors,
	COUNT(*) FILTER (
		WHERE EXISTS (
			SELECT 1
			FROM jsonb_array_elements(unpublished_errors.routing_attempts) AS attempt
			WHERE attempt->>'stage' = 'actual_attempt'
		)
	) AS actual_attempt_violations
FROM unpublished_errors`

const enterpriseMemberAdmissionPlannerHealthSQL = `
WITH usage_eval AS (
	SELECT
		(schedule_meta->>'shadow_evaluation_error')::boolean AS evaluation_failed,
		NULLIF(schedule_meta->>'shadow_planner_latency_ms', '')::double precision AS latency_ms
	FROM usage_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND COALESCE(LOWER(schedule_meta->>'shadow_plan_evaluated'), '') IN ('true', '1', 'yes')
), ops_eval AS (
	SELECT EXISTS (
		SELECT 1
		FROM jsonb_array_elements(e.routing_attempts) AS attempt
		WHERE attempt->>'reason' = 'evaluation_failed'
	) AS evaluation_failed
	FROM ops_error_logs e
	WHERE e.created_at >= $1
	  AND e.created_at < $2
	  AND e.member_id IS NOT NULL
	  AND jsonb_array_length(e.routing_attempts) > 0
)
SELECT
	(SELECT COUNT(*) FROM usage_eval) + (SELECT COUNT(*) FROM ops_eval) AS evaluations,
	(SELECT COUNT(*) FROM usage_eval WHERE evaluation_failed) + (SELECT COUNT(*) FROM ops_eval WHERE evaluation_failed) AS evaluation_failed,
	(SELECT COUNT(*) FROM usage_eval WHERE latency_ms IS NOT NULL AND latency_ms >= 0) AS latency_samples,
	(SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY latency_ms) FROM usage_eval WHERE latency_ms IS NOT NULL AND latency_ms >= 0) AS p95_ms,
	(SELECT percentile_cont(0.99) WITHIN GROUP (ORDER BY latency_ms) FROM usage_eval WHERE latency_ms IS NOT NULL AND latency_ms >= 0) AS p99_ms`

const enterpriseMemberAdmissionLKGSQL = `
WITH usage_plan AS (
	SELECT route_plan_source, COUNT(*) AS count
	FROM usage_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND route_plan_source IN ('live', 'last_known_good')
	GROUP BY route_plan_source
), ops_plan AS (
	SELECT routing_plan_source, COUNT(*) AS count
	FROM ops_error_logs
	WHERE created_at >= $1
	  AND created_at < $2
	  AND member_id IS NOT NULL
	  AND COALESCE(routing_plan_source, '') <> ''
	GROUP BY routing_plan_source
), lkg_reason AS (
	SELECT attempt->>'reason' AS reason, COUNT(*) AS count
	FROM ops_error_logs e
	CROSS JOIN LATERAL jsonb_array_elements(e.routing_attempts) AS attempt
	WHERE e.created_at >= $1
	  AND e.created_at < $2
	  AND e.member_id IS NOT NULL
	  AND attempt->>'reason' IN ('evaluation_failed', 'lkg_stale', 'lkg_expired', 'lkg_generation_mismatch')
	GROUP BY attempt->>'reason'
)
SELECT
	COALESCE((SELECT count FROM usage_plan WHERE route_plan_source = 'live'), 0) AS live_successes,
	COALESCE((SELECT count FROM usage_plan WHERE route_plan_source = 'last_known_good'), 0) AS lkg_successes,
	COALESCE((SELECT count FROM ops_plan WHERE routing_plan_source = 'last_known_good'), 0) AS lkg_errors,
	COALESCE((SELECT count FROM lkg_reason WHERE reason = 'evaluation_failed'), 0) AS lkg_misses,
	COALESCE((SELECT SUM(count) FROM lkg_reason WHERE reason IN ('lkg_stale', 'lkg_expired')), 0) AS lkg_stale_or_expired,
	COALESCE((SELECT count FROM lkg_reason WHERE reason = 'lkg_generation_mismatch'), 0) AS lkg_generation_mismatch`
