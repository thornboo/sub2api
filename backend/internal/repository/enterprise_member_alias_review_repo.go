package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type enterpriseMemberAliasReviewRepository struct {
	db *sql.DB
}

func NewEnterpriseMemberAliasReviewRepository(db *sql.DB) service.EnterpriseMemberAliasReviewRepository {
	return &enterpriseMemberAliasReviewRepository{db: db}
}

func (r *enterpriseMemberAliasReviewRepository) ListLegacySuccessNewPruned(ctx context.Context, input service.EnterpriseMemberAliasReviewListInput) ([]service.EnterpriseMemberAliasReviewItem, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member alias review repository db is nil")
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, enterpriseMemberAliasReviewListSQL, now.AddDate(0, 0, -30), now, now.AddDate(0, 0, -7), limit)
	if err != nil {
		return nil, fmt.Errorf("list enterprise member alias reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.EnterpriseMemberAliasReviewItem, 0, limit)
	for rows.Next() {
		var item service.EnterpriseMemberAliasReviewItem
		var finalGroupID, channelID, reviewedBy sql.NullInt64
		var reviewedAt sql.NullTime
		var reasonCodes pq.StringArray
		if err := rows.Scan(
			&item.PublicModel,
			&item.PublicModelNorm,
			&item.Endpoint,
			&item.RequestCount7d,
			&item.RequestCount30d,
			&item.SuccessCount7d,
			&item.SuccessCount30d,
			&item.AffectedEnterpriseCount7d,
			&item.AffectedEnterpriseCount30d,
			&item.LastSeenAt,
			&finalGroupID,
			&channelID,
			&reasonCodes,
			&item.ReviewStatus,
			&reviewedBy,
			&reviewedAt,
			&item.ReviewNote,
			&item.StableRouteEvidence,
		); err != nil {
			return nil, fmt.Errorf("scan enterprise member alias review item: %w", err)
		}
		item.LegacyOutcome = "success"
		item.PlannedOutcome = "pruned"
		item.ReasonCodes = append([]string(nil), reasonCodes...)
		if finalGroupID.Valid {
			item.FinalGroupID = &finalGroupID.Int64
		}
		if channelID.Valid {
			item.ChannelID = &channelID.Int64
		}
		if reviewedBy.Valid {
			item.ReviewedBy = &reviewedBy.Int64
		}
		if reviewedAt.Valid {
			item.ReviewedAt = &reviewedAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enterprise member alias review items: %w", err)
	}
	return items, nil
}

func (r *enterpriseMemberAliasReviewRepository) UpsertReview(ctx context.Context, input service.EnterpriseMemberAliasReviewUpsert) (*service.EnterpriseMemberAliasReviewRecord, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member alias review repository db is nil")
	}
	evidence := input.ValidationEvidence
	if len(evidence) == 0 {
		evidence = json.RawMessage(`{}`)
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO enterprise_member_model_alias_reviews (
			public_model, public_model_norm, endpoint, status, final_group_id, channel_id,
			review_note, validation_evidence, reviewed_by, reviewed_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, NULLIF($9, 0), NOW(), NOW())
		ON CONFLICT (public_model_norm, endpoint) DO UPDATE SET
			public_model = EXCLUDED.public_model,
			status = EXCLUDED.status,
			final_group_id = EXCLUDED.final_group_id,
			channel_id = EXCLUDED.channel_id,
			review_note = EXCLUDED.review_note,
			validation_evidence = EXCLUDED.validation_evidence,
			reviewed_by = EXCLUDED.reviewed_by,
			reviewed_at = EXCLUDED.reviewed_at,
			updated_at = NOW()
		RETURNING id, public_model, public_model_norm, endpoint, status, final_group_id, channel_id,
		          review_note, validation_evidence, reviewed_by, reviewed_at, created_at, updated_at`,
		input.PublicModel, input.PublicModelNorm, input.Endpoint, input.Status,
		input.FinalGroupID, input.ChannelID, input.ReviewNote, string(evidence), input.ReviewedBy)
	return scanEnterpriseMemberAliasReviewRecord(row)
}

func (r *enterpriseMemberAliasReviewRepository) GetReadinessSummary(ctx context.Context, now time.Time) (*service.EnterpriseMemberAliasReviewReadinessSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("enterprise member alias review repository db is nil")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	summary := &service.EnterpriseMemberAliasReviewReadinessSummary{GeneratedAt: now}
	if err := r.db.QueryRowContext(ctx, enterpriseMemberAliasReviewReadinessSQL, now.AddDate(0, 0, -30), now, now.AddDate(0, 0, -7)).Scan(
		&summary.LegacySuccessNewPrunedActive7d,
		&summary.LegacySuccessNewPrunedActive30d,
		&summary.BlockingUnreviewedActive7d,
		&summary.BlockingUnreviewedActive30d,
	); err != nil {
		return nil, fmt.Errorf("get enterprise member alias review readiness: %w", err)
	}
	return summary, nil
}

type aliasReviewRowScanner interface {
	Scan(dest ...any) error
}

func scanEnterpriseMemberAliasReviewRecord(row aliasReviewRowScanner) (*service.EnterpriseMemberAliasReviewRecord, error) {
	var record service.EnterpriseMemberAliasReviewRecord
	var finalGroupID, channelID, reviewedBy sql.NullInt64
	var reviewedAt sql.NullTime
	var evidence []byte
	if err := row.Scan(
		&record.ID,
		&record.PublicModel,
		&record.PublicModelNorm,
		&record.Endpoint,
		&record.Status,
		&finalGroupID,
		&channelID,
		&record.ReviewNote,
		&evidence,
		&reviewedBy,
		&reviewedAt,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if finalGroupID.Valid {
		record.FinalGroupID = &finalGroupID.Int64
	}
	if channelID.Valid {
		record.ChannelID = &channelID.Int64
	}
	if reviewedBy.Valid {
		record.ReviewedBy = &reviewedBy.Int64
	}
	if reviewedAt.Valid {
		record.ReviewedAt = &reviewedAt.Time
	}
	if len(evidence) > 0 {
		record.ValidationEvidence = append(json.RawMessage(nil), evidence...)
	}
	return &record, nil
}

const enterpriseMemberAliasReviewBaseCTE = `
WITH alias_usage AS (
	SELECT
		BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model)) AS public_model,
		LOWER(BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model))) AS public_model_norm,
		COALESCE(NULLIF(BTRIM(ul.inbound_endpoint), ''), '') AS endpoint,
		ul.group_id,
		ul.channel_id,
		ul.user_id,
		ul.created_at,
		ul.schedule_meta
	FROM usage_logs ul
	WHERE ul.created_at >= $1
	  AND ul.created_at < $2
	  AND ul.member_id IS NOT NULL
	  AND BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model)) <> ''
	  AND CHAR_LENGTH(BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model))) <= 255
	  AND BTRIM(COALESCE(NULLIF(ul.requested_model, ''), ul.model)) !~ '[[:cntrl:]]'
	  AND (
		COALESCE(ul.schedule_meta->>'shadow_diff_type', '') = 'legacy_success_new_pruned'
		OR LOWER(COALESCE(ul.schedule_meta->>'shadow_legacy_success_new_pruned', '')) IN ('true', '1', 'yes')
	  )
), alias_reasons AS (
	SELECT au.public_model_norm, au.endpoint, ARRAY_REMOVE(ARRAY_AGG(DISTINCT reason.value), NULL) AS reason_codes
	FROM alias_usage au
	LEFT JOIN LATERAL jsonb_array_elements_text(
		CASE WHEN jsonb_typeof(au.schedule_meta->'shadow_reason_codes') = 'array'
		     THEN au.schedule_meta->'shadow_reason_codes'
		     ELSE '[]'::jsonb
		END
	) AS reason(value) ON TRUE
	GROUP BY au.public_model_norm, au.endpoint
), alias_aggregates AS (
	SELECT
		MIN(au.public_model) AS public_model,
		au.public_model_norm,
		au.endpoint,
		COUNT(*) FILTER (WHERE au.created_at >= $3) AS request_count_7d,
		COUNT(*) AS request_count_30d,
		COUNT(*) FILTER (WHERE au.created_at >= $3) AS success_count_7d,
		COUNT(*) AS success_count_30d,
		COUNT(DISTINCT au.user_id) FILTER (WHERE au.created_at >= $3) AS affected_enterprise_count_7d,
		COUNT(DISTINCT au.user_id) AS affected_enterprise_count_30d,
		MAX(au.created_at) AS last_seen_at,
		CASE WHEN COUNT(DISTINCT au.group_id) = 1 THEN MIN(au.group_id) ELSE NULL END AS final_group_id,
		CASE WHEN COUNT(DISTINCT au.channel_id) = 1 THEN MIN(au.channel_id) ELSE NULL END AS channel_id
	FROM alias_usage au
	GROUP BY au.public_model_norm, au.endpoint
)`

const enterpriseMemberAliasReviewListSQL = enterpriseMemberAliasReviewBaseCTE + `
SELECT
	aa.public_model,
	aa.public_model_norm,
	aa.endpoint,
	aa.request_count_7d,
	aa.request_count_30d,
	aa.success_count_7d,
	aa.success_count_30d,
	aa.affected_enterprise_count_7d,
	aa.affected_enterprise_count_30d,
	aa.last_seen_at,
	aa.final_group_id,
	aa.channel_id,
	COALESCE(ar.reason_codes, ARRAY[]::text[]) AS reason_codes,
	COALESCE(rv.status, 'pending') AS review_status,
	rv.reviewed_by,
	rv.reviewed_at,
	COALESCE(rv.review_note, '') AS review_note,
	CASE WHEN rv.status = 'registered' THEN 'registered_exact_stable_route'
	     ELSE 'not_registered' END AS stable_route_evidence
FROM alias_aggregates aa
LEFT JOIN alias_reasons ar
  ON ar.public_model_norm = aa.public_model_norm AND ar.endpoint = aa.endpoint
LEFT JOIN enterprise_member_model_alias_reviews rv
  ON rv.public_model_norm = aa.public_model_norm AND rv.endpoint = aa.endpoint
ORDER BY aa.last_seen_at DESC, aa.request_count_30d DESC, aa.public_model_norm
LIMIT $4`

const enterpriseMemberAliasReviewReadinessSQL = enterpriseMemberAliasReviewBaseCTE + `
SELECT
	COUNT(*) FILTER (WHERE aa.request_count_7d > 0) AS active_7d,
	COUNT(*) FILTER (WHERE aa.request_count_30d > 0) AS active_30d,
	COUNT(*) FILTER (WHERE aa.request_count_7d > 0 AND COALESCE(rv.status, 'pending') = 'pending') AS blocking_7d,
	COUNT(*) FILTER (WHERE aa.request_count_30d > 0 AND COALESCE(rv.status, 'pending') = 'pending') AS blocking_30d
FROM alias_aggregates aa
LEFT JOIN enterprise_member_model_alias_reviews rv
  ON rv.public_model_norm = aa.public_model_norm AND rv.endpoint = aa.endpoint`
