package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type modelProtocolCapabilityRepository struct {
	db *sql.DB
}

func NewModelProtocolCapabilityRepository(db *sql.DB) service.ModelProtocolCapabilityRepository {
	return &modelProtocolCapabilityRepository{db: db}
}

func (r *modelProtocolCapabilityRepository) ListByAccount(ctx context.Context, accountID int64) ([]service.AccountModelProtocolCapability, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, upstream_model, protocol, override_state, observed_state,
		       COALESCE(observed_source, ''), observed_at, created_at, updated_at
		FROM account_model_protocol_capabilities
		WHERE account_id = $1
		ORDER BY CASE WHEN upstream_model = '*' THEN 0 ELSE 1 END, upstream_model, protocol`, accountID)
	if err != nil {
		return nil, fmt.Errorf("list model protocol capabilities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.AccountModelProtocolCapability, 0)
	for rows.Next() {
		var item service.AccountModelProtocolCapability
		if err := rows.Scan(&item.ID, &item.AccountID, &item.UpstreamModel, &item.Protocol, &item.OverrideState, &item.ObservedState, &item.ObservedSource, &item.ObservedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan model protocol capability: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model protocol capabilities: %w", err)
	}
	return items, nil
}

func (r *modelProtocolCapabilityRepository) ListByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64][]service.AccountModelProtocolCapability, error) {
	result := make(map[int64][]service.AccountModelProtocolCapability, len(accountIDs))
	if len(accountIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, upstream_model, protocol, override_state, observed_state,
		       COALESCE(observed_source, ''), observed_at, created_at, updated_at
		FROM account_model_protocol_capabilities
		WHERE account_id = ANY($1)
		ORDER BY account_id, CASE WHEN upstream_model = '*' THEN 0 ELSE 1 END, upstream_model, protocol`, pq.Array(accountIDs))
	if err != nil {
		return nil, fmt.Errorf("list model protocol capabilities by accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var item service.AccountModelProtocolCapability
		if err := rows.Scan(&item.ID, &item.AccountID, &item.UpstreamModel, &item.Protocol, &item.OverrideState, &item.ObservedState, &item.ObservedSource, &item.ObservedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan model protocol capability by accounts: %w", err)
		}
		result[item.AccountID] = append(result[item.AccountID], item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model protocol capabilities by accounts: %w", err)
	}
	return result, nil
}

func lockModelProtocolAccount(ctx context.Context, tx *sql.Tx, accountID int64) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('account_model_protocol_capabilities:' || $1::text, 0))`, accountID)
	return err
}

func (r *modelProtocolCapabilityRepository) SyncObserved(ctx context.Context, accountID int64, observations []service.ModelProtocolObservation) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin model protocol observation sync: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockModelProtocolAccount(ctx, tx, accountID); err != nil {
		return fmt.Errorf("lock model protocol observation sync: %w", err)
	}
	for _, item := range observations {
		query := `
			INSERT INTO account_model_protocol_capabilities (
				account_id, upstream_model, protocol, observed_state, observed_source, observed_at
			) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
			ON CONFLICT (account_id, upstream_model, protocol) DO UPDATE SET
				observed_state = EXCLUDED.observed_state,
				observed_source = EXCLUDED.observed_source,
				observed_at = EXCLUDED.observed_at,
				updated_at = NOW()`
		if item.State == service.ModelProtocolStateUnknown {
			// Missing/unknown upstream metadata creates a visible unknown row but
			// never erases an earlier supported/unsupported observation.
			query = `
				INSERT INTO account_model_protocol_capabilities (
					account_id, upstream_model, protocol, observed_state, observed_source, observed_at
				) VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6)
				ON CONFLICT (account_id, upstream_model, protocol) DO NOTHING`
		}
		if _, err := tx.ExecContext(ctx, query, accountID, strings.TrimSpace(item.UpstreamModel), item.Protocol, item.State, strings.TrimSpace(item.Source), item.ObservedAt); err != nil {
			return fmt.Errorf("upsert model protocol observation: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit model protocol observation sync: %w", err)
	}
	return nil
}

func (r *modelProtocolCapabilityRepository) UpdateOverrides(ctx context.Context, accountID int64, overrides []service.ModelProtocolOverride) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin model protocol override update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockModelProtocolAccount(ctx, tx, accountID); err != nil {
		return fmt.Errorf("lock model protocol override update: %w", err)
	}
	for _, item := range overrides {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO account_model_protocol_capabilities (
				account_id, upstream_model, protocol, override_state
			) VALUES ($1, $2, $3, $4)
			ON CONFLICT (account_id, upstream_model, protocol) DO UPDATE SET
				override_state = EXCLUDED.override_state,
				updated_at = NOW()`, accountID, strings.TrimSpace(item.UpstreamModel), item.Protocol, item.State); err != nil {
			return fmt.Errorf("upsert model protocol override: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM account_model_protocol_capabilities
		WHERE account_id = $1
		  AND override_state = 'auto'
		  AND observed_state = 'unknown'
		  AND observed_source IS NULL
		  AND observed_at IS NULL`, accountID); err != nil {
		return fmt.Errorf("delete empty model protocol overrides: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit model protocol override update: %w", err)
	}
	return nil
}

var _ service.ModelProtocolCapabilityRepository = (*modelProtocolCapabilityRepository)(nil)
