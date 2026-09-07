package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type modelSelfCheckRepository struct {
	db *sql.DB
}

func NewModelSelfCheckRepository(db *sql.DB) service.ModelSelfCheckRepository {
	return &modelSelfCheckRepository{db: db}
}

func (r *modelSelfCheckRepository) ListStatusTargets(ctx context.Context) ([]service.ModelSelfCheckTarget, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT g.id, g.name, g.platform, cfg.model
		FROM model_self_check_config cfg
		JOIN channels c ON c.id = cfg.channel_id
		JOIN channel_groups cg ON cg.channel_id = c.id
		JOIN groups g ON g.id = cg.group_id
		WHERE cfg.enabled = TRUE
		  AND c.status = 'active'
		  AND g.status = 'active'
		  AND g.deleted_at IS NULL
		ORDER BY g.name, g.id, cfg.model`)
	if err != nil {
		return nil, fmt.Errorf("list model self check targets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	targets := []service.ModelSelfCheckTarget{}
	for rows.Next() {
		var target service.ModelSelfCheckTarget
		if err := rows.Scan(&target.GroupID, &target.GroupName, &target.GroupPlatform, &target.Model); err != nil {
			return nil, fmt.Errorf("scan model self check target: %w", err)
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check targets: %w", err)
	}
	return targets, nil
}

func (r *modelSelfCheckRepository) ListTargetAccounts(ctx context.Context, groupIDs []int64) ([]service.ModelSelfCheckTargetAccount, error) {
	if len(groupIDs) == 0 {
		return []service.ModelSelfCheckTargetAccount{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT ag.group_id, ag.account_id, a.platform
		FROM account_groups ag
		JOIN accounts a ON a.id = ag.account_id
		WHERE ag.group_id = ANY($1)
		  AND a.deleted_at IS NULL
		ORDER BY ag.group_id, a.priority, ag.account_id`,
		pq.Array(groupIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("list model self check target accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	accounts := []service.ModelSelfCheckTargetAccount{}
	for rows.Next() {
		var account service.ModelSelfCheckTargetAccount
		if err := rows.Scan(&account.GroupID, &account.AccountID, &account.Platform); err != nil {
			return nil, fmt.Errorf("scan model self check target account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check target accounts: %w", err)
	}
	return accounts, nil
}

func (r *modelSelfCheckRepository) ListLatestByModels(ctx context.Context, models []string) ([]service.ModelSelfCheckHistory, error) {
	if len(models) == 0 {
		return []service.ModelSelfCheckHistory{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (model, account_id)
		       id, model, account_id, platform, status, latency_ms, checked_at
		FROM model_self_check_histories
		WHERE model = ANY($1)
		ORDER BY model, account_id, checked_at DESC, id DESC`,
		pq.Array(models),
	)
	if err != nil {
		return nil, fmt.Errorf("list latest model self check histories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckHistoryRows(rows)
}

func (r *modelSelfCheckRepository) ListHistoriesSince(ctx context.Context, models []string, since time.Time) ([]service.ModelSelfCheckHistory, error) {
	if len(models) == 0 {
		return []service.ModelSelfCheckHistory{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, model, account_id, platform, status, latency_ms, checked_at
		FROM model_self_check_histories
		WHERE model = ANY($1)
		  AND checked_at >= $2
		ORDER BY checked_at DESC, id DESC`,
		pq.Array(models), since,
	)
	if err != nil {
		return nil, fmt.Errorf("list model self check histories since: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckHistoryRows(rows)
}

func (r *modelSelfCheckRepository) ListRecentHistories(ctx context.Context, model string, accountIDs []int64, limit int) ([]service.ModelSelfCheckHistory, error) {
	if len(accountIDs) == 0 {
		return []service.ModelSelfCheckHistory{}, nil
	}
	if limit <= 0 {
		limit = service.MonitorHistoryDefaultLimit
	}
	if limit > service.MonitorHistoryMaxLimit {
		limit = service.MonitorHistoryMaxLimit
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, model, account_id, platform, status, latency_ms, checked_at
		FROM model_self_check_histories
		WHERE model = $1
		  AND account_id = ANY($2)
		ORDER BY checked_at DESC, id DESC
		LIMIT $3`,
		model, pq.Array(accountIDs), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent model self check histories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckHistoryRows(rows)
}

func (r *modelSelfCheckRepository) ListRecentHistoriesBefore(
	ctx context.Context,
	model string,
	accountIDs []int64,
	before time.Time,
	limit int,
) ([]service.ModelSelfCheckHistory, error) {
	if len(accountIDs) == 0 {
		return []service.ModelSelfCheckHistory{}, nil
	}
	if limit <= 0 {
		limit = service.MonitorHistoryDefaultLimit
	}
	if limit > service.MonitorHistoryMaxLimit {
		limit = service.MonitorHistoryMaxLimit
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, model, account_id, platform, status, latency_ms, checked_at
		FROM model_self_check_histories
		WHERE model = $1
		  AND account_id = ANY($2)
		  AND checked_at < $3
		ORDER BY checked_at DESC, id DESC
		LIMIT $4`,
		model, pq.Array(accountIDs), before, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent model self check histories before: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckHistoryRows(rows)
}

func (r *modelSelfCheckRepository) ListRecentStatusSnapshots(
	ctx context.Context,
	groupID int64,
	model string,
	limit int,
) ([]service.ModelSelfCheckStatusSnapshot, error) {
	if groupID <= 0 || model == "" {
		return []service.ModelSelfCheckStatusSnapshot{}, nil
	}
	if limit <= 0 {
		limit = service.MonitorHistoryDefaultLimit
	}
	if limit > service.MonitorHistoryMaxLimit {
		limit = service.MonitorHistoryMaxLimit
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, group_id, model, status, reason_code,
		       eligible_account_count, checked_account_count,
		       operational_account_count, degraded_account_count, failed_account_count,
		       latency_ms, checked_at, created_at
		FROM model_self_check_status_snapshots
		WHERE group_id = $1
		  AND model = $2
		ORDER BY checked_at DESC, id DESC
		LIMIT $3`,
		groupID, model, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent model self check status snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckStatusSnapshotRows(rows)
}

func (r *modelSelfCheckRepository) ListStatusSnapshotsSince(
	ctx context.Context,
	groupID int64,
	model string,
	since time.Time,
) ([]service.ModelSelfCheckStatusSnapshot, error) {
	if groupID <= 0 || model == "" {
		return []service.ModelSelfCheckStatusSnapshot{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, group_id, model, status, reason_code,
		       eligible_account_count, checked_account_count,
		       operational_account_count, degraded_account_count, failed_account_count,
		       latency_ms, checked_at, created_at
		FROM model_self_check_status_snapshots
		WHERE group_id = $1
		  AND model = $2
		  AND checked_at >= $3
		ORDER BY checked_at DESC, id DESC`,
		groupID, model, since,
	)
	if err != nil {
		return nil, fmt.Errorf("list model self check status snapshots since: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckStatusSnapshotRows(rows)
}

func (r *modelSelfCheckRepository) ListStatusSnapshotMetrics(
	ctx context.Context,
	targets []service.ModelSelfCheckTarget,
	now time.Time,
) ([]service.ModelSelfCheckStatusMetrics, error) {
	if len(targets) == 0 {
		return []service.ModelSelfCheckStatusMetrics{}, nil
	}
	groupIDs := make([]int64, 0, len(targets))
	models := make([]string, 0, len(targets))
	for _, target := range targets {
		groupIDs = append(groupIDs, target.GroupID)
		models = append(models, target.Model)
	}
	now = now.UTC()
	since30d := now.AddDate(0, 0, -30)
	since24h := now.AddDate(0, 0, -1)
	since7d := now.AddDate(0, 0, -7)
	rows, err := r.db.QueryContext(ctx, `
		WITH target_input AS (
			SELECT ord, group_id, model
			FROM unnest($1::bigint[], $2::text[]) WITH ORDINALITY AS target(group_id, model, ord)
		)
		SELECT target_input.group_id,
		       target_input.model,
		       EXISTS (
		           SELECT 1
		           FROM model_self_check_status_snapshots snapshot
		           WHERE snapshot.group_id = target_input.group_id
		             AND snapshot.model = target_input.model
		             AND snapshot.checked_at <= $4
		       ) AS has_snapshots,
		       CASE WHEN metrics.total_24h > 0
		            THEN metrics.usable_24h::double precision * 100 / metrics.total_24h
		       END AS availability_24h,
		       CASE WHEN metrics.total_7d > 0
		            THEN metrics.usable_7d::double precision * 100 / metrics.total_7d
		       END AS availability_7d,
		       CASE WHEN metrics.total_30d > 0
		            THEN metrics.usable_30d::double precision * 100 / metrics.total_30d
		       END AS availability_30d,
		       CASE WHEN metrics.latency_count_24h > 0
		            THEN metrics.latency_sum_24h / metrics.latency_count_24h
		       END AS avg_latency_24h_ms,
		       CASE WHEN metrics.latency_count_7d > 0
		            THEN metrics.latency_sum_7d / metrics.latency_count_7d
		       END AS avg_latency_7d_ms,
		       CASE WHEN metrics.total_24h > 0
		            THEN metrics.degraded_24h::double precision * 100 / metrics.total_24h
		       END AS degraded_ratio_24h
		FROM target_input
		LEFT JOIN LATERAL (
			SELECT COUNT(*) FILTER (WHERE snapshot.checked_at >= $5) AS total_24h,
			       COUNT(*) FILTER (WHERE snapshot.checked_at >= $5 AND snapshot.status IN ($7, $8)) AS usable_24h,
			       COUNT(*) FILTER (WHERE snapshot.checked_at >= $5 AND snapshot.status = $8) AS degraded_24h,
			       COALESCE(SUM(snapshot.latency_ms) FILTER (
			           WHERE snapshot.checked_at >= $5
			             AND snapshot.status IN ($7, $8)
			             AND snapshot.latency_ms IS NOT NULL
			       ), 0)::bigint AS latency_sum_24h,
			       COUNT(snapshot.latency_ms) FILTER (
			           WHERE snapshot.checked_at >= $5
			             AND snapshot.status IN ($7, $8)
			       ) AS latency_count_24h,
			       COUNT(*) FILTER (WHERE snapshot.checked_at >= $6) AS total_7d,
			       COUNT(*) FILTER (WHERE snapshot.checked_at >= $6 AND snapshot.status IN ($7, $8)) AS usable_7d,
			       COALESCE(SUM(snapshot.latency_ms) FILTER (
			           WHERE snapshot.checked_at >= $6
			             AND snapshot.status IN ($7, $8)
			             AND snapshot.latency_ms IS NOT NULL
			       ), 0)::bigint AS latency_sum_7d,
			       COUNT(snapshot.latency_ms) FILTER (
			           WHERE snapshot.checked_at >= $6
			             AND snapshot.status IN ($7, $8)
			       ) AS latency_count_7d,
			       COUNT(*) AS total_30d,
			       COUNT(*) FILTER (WHERE snapshot.status IN ($7, $8)) AS usable_30d
			FROM model_self_check_status_snapshots snapshot
			WHERE snapshot.group_id = target_input.group_id
			  AND snapshot.model = target_input.model
			  AND snapshot.checked_at >= $3
			  AND snapshot.checked_at <= $4
		) metrics ON TRUE
		ORDER BY target_input.ord`,
		pq.Array(groupIDs),
		pq.Array(models),
		since30d,
		now,
		since24h,
		since7d,
		service.MonitorStatusOperational,
		service.MonitorStatusDegraded,
	)
	if err != nil {
		return nil, fmt.Errorf("list model self check status snapshot metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanModelSelfCheckStatusMetricsRows(rows)
}

func (r *modelSelfCheckRepository) CreateHistory(ctx context.Context, history *service.ModelSelfCheckHistory) error {
	if history == nil {
		return fmt.Errorf("insert model self check history: nil history")
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO model_self_check_histories
		    (model, account_id, platform, status, latency_ms, http_status, error_code, input_tokens, output_tokens, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id`,
		history.Model,
		history.AccountID,
		history.Platform,
		history.Status,
		history.LatencyMs,
		history.HTTPStatus,
		nullableStringValue(history.ErrorCode),
		history.InputTokens,
		history.OutputTokens,
		history.CheckedAt,
	).Scan(&history.ID)
	if err != nil {
		return fmt.Errorf("insert model self check history: %w", err)
	}
	return nil
}

func (r *modelSelfCheckRepository) ListTokenUsageSince(ctx context.Context, since time.Time) ([]service.ModelSelfCheckTokenUsage, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT model,
		       COALESCE(SUM(input_tokens), 0) AS input_tokens,
		       COALESCE(SUM(output_tokens), 0) AS output_tokens
		FROM model_self_check_histories
		WHERE checked_at >= $1
		GROUP BY model
		ORDER BY model`,
		since,
	)
	if err != nil {
		return nil, fmt.Errorf("list model self check token usage: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := []service.ModelSelfCheckTokenUsage{}
	for rows.Next() {
		var row service.ModelSelfCheckTokenUsage
		if err := rows.Scan(&row.Model, &row.InputTokens, &row.OutputTokens); err != nil {
			return nil, fmt.Errorf("scan model self check token usage: %w", err)
		}
		row.TotalTokens = row.InputTokens + row.OutputTokens
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check token usage: %w", err)
	}
	return out, nil
}

func (r *modelSelfCheckRepository) CreateStatusSnapshot(ctx context.Context, snapshot *service.ModelSelfCheckStatusSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("insert model self check status snapshot: nil snapshot")
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO model_self_check_status_snapshots
		    (group_id, model, status, reason_code,
		     eligible_account_count, checked_account_count,
		     operational_account_count, degraded_account_count, failed_account_count,
		     latency_ms, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`,
		snapshot.GroupID,
		snapshot.Model,
		snapshot.Status,
		snapshot.ReasonCode,
		snapshot.EligibleAccountCount,
		snapshot.CheckedAccountCount,
		snapshot.OperationalAccountCount,
		snapshot.DegradedAccountCount,
		snapshot.FailedAccountCount,
		snapshot.LatencyMs,
		snapshot.CheckedAt,
	).Scan(&snapshot.ID)
	if err != nil {
		return fmt.Errorf("insert model self check status snapshot: %w", err)
	}
	return nil
}

func (r *modelSelfCheckRepository) DeleteStatusSnapshotsBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM model_self_check_status_snapshots
		WHERE checked_at < $1`,
		before,
	)
	if err != nil {
		return 0, fmt.Errorf("delete old model self check status snapshots: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted model self check status snapshot count: %w", err)
	}
	return deleted, nil
}

func nullableStringValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func scanModelSelfCheckHistoryRows(rows *sql.Rows) ([]service.ModelSelfCheckHistory, error) {
	out := []service.ModelSelfCheckHistory{}
	for rows.Next() {
		var row service.ModelSelfCheckHistory
		var latency sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.Model,
			&row.AccountID,
			&row.Platform,
			&row.Status,
			&latency,
			&row.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model self check history: %w", err)
		}
		if latency.Valid {
			v := int(latency.Int64)
			row.LatencyMs = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check histories: %w", err)
	}
	return out, nil
}

func scanModelSelfCheckStatusSnapshotRows(rows *sql.Rows) ([]service.ModelSelfCheckStatusSnapshot, error) {
	out := []service.ModelSelfCheckStatusSnapshot{}
	for rows.Next() {
		var row service.ModelSelfCheckStatusSnapshot
		var latency sql.NullInt64
		if err := rows.Scan(
			&row.ID,
			&row.GroupID,
			&row.Model,
			&row.Status,
			&row.ReasonCode,
			&row.EligibleAccountCount,
			&row.CheckedAccountCount,
			&row.OperationalAccountCount,
			&row.DegradedAccountCount,
			&row.FailedAccountCount,
			&latency,
			&row.CheckedAt,
			&row.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan model self check status snapshot: %w", err)
		}
		if latency.Valid {
			v := int(latency.Int64)
			row.LatencyMs = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check status snapshots: %w", err)
	}
	return out, nil
}

func scanModelSelfCheckStatusMetricsRows(rows *sql.Rows) ([]service.ModelSelfCheckStatusMetrics, error) {
	out := []service.ModelSelfCheckStatusMetrics{}
	for rows.Next() {
		var row service.ModelSelfCheckStatusMetrics
		var availability24h sql.NullFloat64
		var availability7d sql.NullFloat64
		var availability30d sql.NullFloat64
		var avgLatency24h sql.NullInt64
		var avgLatency7d sql.NullInt64
		var degradedRatio24h sql.NullFloat64
		if err := rows.Scan(
			&row.GroupID,
			&row.Model,
			&row.HasSnapshots,
			&availability24h,
			&availability7d,
			&availability30d,
			&avgLatency24h,
			&avgLatency7d,
			&degradedRatio24h,
		); err != nil {
			return nil, fmt.Errorf("scan model self check status snapshot metrics: %w", err)
		}
		if availability24h.Valid {
			v := availability24h.Float64
			row.Availability24h = &v
		}
		if availability7d.Valid {
			v := availability7d.Float64
			row.Availability7d = &v
		}
		if availability30d.Valid {
			v := availability30d.Float64
			row.Availability30d = &v
		}
		if avgLatency24h.Valid {
			v := int(avgLatency24h.Int64)
			row.AvgLatency24hMs = &v
		}
		if avgLatency7d.Valid {
			v := int(avgLatency7d.Int64)
			row.AvgLatency7dMs = &v
		}
		if degradedRatio24h.Valid {
			v := degradedRatio24h.Float64
			row.DegradedRatio24h = &v
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check status snapshot metrics: %w", err)
	}
	return out, nil
}
