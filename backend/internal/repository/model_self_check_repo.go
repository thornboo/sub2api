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
		  AND a.status = 'active'
		  AND a.schedulable = TRUE
		  AND a.deleted_at IS NULL
		  AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())
		ORDER BY ag.group_id, ag.priority, ag.account_id`,
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

func (r *modelSelfCheckRepository) CreateHistory(ctx context.Context, history *service.ModelSelfCheckHistory) error {
	if history == nil {
		return fmt.Errorf("insert model self check history: nil history")
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO model_self_check_histories
		    (model, account_id, platform, status, latency_ms, http_status, error_code, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id`,
		history.Model,
		history.AccountID,
		history.Platform,
		history.Status,
		history.LatencyMs,
		history.HTTPStatus,
		nullableStringValue(history.ErrorCode),
		history.CheckedAt,
	).Scan(&history.ID)
	if err != nil {
		return fmt.Errorf("insert model self check history: %w", err)
	}
	return nil
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
