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

var _ service.ModelSelfCheckRoundRepository = (*modelSelfCheckRepository)(nil)

func marshalProbeSteps(round *service.ModelSelfCheckProbeRound) ([]byte, error) {
	if round == nil {
		return nil, fmt.Errorf("model self check round is required")
	}
	if round.Steps == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(round.Steps)
}

func (r *modelSelfCheckRepository) CreateProbeRound(ctx context.Context, round *service.ModelSelfCheckProbeRound) error {
	steps, err := marshalProbeSteps(round)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin model self check probe round claim: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	// Serialize the short claim transaction across instances. The unique index
	// additionally protects unfinished rounds from any writer that bypasses it.
	claimKey := fmt.Sprintf("%d:%s", round.GroupID, round.Model)
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 237))`, claimKey); err != nil {
		return fmt.Errorf("lock model self check probe round claim: %w", err)
	}
	// A round has 90 seconds of work and 30 seconds of persistence grace. Reclaim
	// crashed workers without leaving a target permanently blocked. Actual step
	// outcomes and original account snapshots remain intact.
	if _, err := tx.ExecContext(ctx, `
		UPDATE model_self_check_probe_rounds
		SET status='unknown', reason_code='round_incomplete', finished_at=NOW(),
		    duration_ms=GREATEST(0, LEAST(2147483647, EXTRACT(EPOCH FROM (NOW()-started_at))*1000))::int,
		    steps=COALESCE((SELECT jsonb_agg(CASE WHEN step->>'outcome'='pending'
		        THEN step || '{"outcome":"incomplete","reason_code":"round_incomplete"}'::jsonb
		        WHEN step->>'outcome'='not_attempted' THEN step || '{"reason_code":"round_incomplete"}'::jsonb
		        ELSE step END ORDER BY ord)
		        FROM jsonb_array_elements(steps) WITH ORDINALITY AS s(step,ord)), '[]'::jsonb)
		WHERE group_id=$1 AND model=$2 AND finished_at IS NULL
		  AND started_at < NOW()-INTERVAL '120 seconds'`, round.GroupID, round.Model); err != nil {
		return fmt.Errorf("expire model self check probe round: %w", err)
	}
	var id int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO model_self_check_probe_rounds
		(group_id, model, status, reason_code, winner_account_id, started_at, finished_at, duration_ms, steps)
		SELECT $1::bigint, $2::varchar(255), $3::varchar(20), $4::varchar(80),
		       $5::bigint, $6::timestamptz, $7::timestamptz, $8::int, $9::jsonb
		WHERE NOT EXISTS (SELECT 1 FROM model_self_check_probe_rounds
		                  WHERE group_id=$1 AND model=$2 AND finished_at IS NULL)
		ON CONFLICT (group_id, model) WHERE finished_at IS NULL DO NOTHING RETURNING id`,
		round.GroupID, round.Model, round.Status, round.ReasonCode, round.WinnerAccountID,
		round.StartedAt, round.FinishedAt, round.DurationMs, string(steps)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrModelSelfCheckRoundInProgress
	}
	if err != nil {
		return fmt.Errorf("create model self check probe round: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit model self check probe round claim: %w", err)
	}
	round.ID = id
	return nil
}

func (r *modelSelfCheckRepository) UpdateProbeRound(ctx context.Context, round *service.ModelSelfCheckProbeRound) error {
	steps, err := marshalProbeSteps(round)
	if err != nil {
		return err
	}
	// A finished round is immutable. A late worker cannot rewrite old evidence.
	result, err := r.db.ExecContext(ctx, `
		UPDATE model_self_check_probe_rounds
		SET status=$2, reason_code=$3, winner_account_id=$4, finished_at=$5, duration_ms=$6, steps=$7::jsonb
		WHERE id=$1 AND finished_at IS NULL`, round.ID, round.Status, round.ReasonCode,
		round.WinnerAccountID, round.FinishedAt, round.DurationMs, string(steps))
	if err != nil {
		return fmt.Errorf("update model self check probe round: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update model self check probe round rows: %w", err)
	}
	if n != 1 {
		return fmt.Errorf("update model self check probe round: missing or already finished round %d", round.ID)
	}
	return nil
}

func (r *modelSelfCheckRepository) ListLatestProbeRounds(ctx context.Context, targets []service.ModelSelfCheckTarget) ([]service.ModelSelfCheckProbeRound, error) {
	out := make([]service.ModelSelfCheckProbeRound, 0, len(targets))
	if len(targets) == 0 {
		return out, nil
	}
	groupIDs := make([]int64, 0, len(targets))
	models := make([]string, 0, len(targets))
	for _, target := range targets {
		groupIDs = append(groupIDs, target.GroupID)
		models = append(models, target.Model)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.group_id, r.model, r.status, r.reason_code, r.winner_account_id,
		       r.started_at, r.finished_at, r.duration_ms, r.steps
		FROM UNNEST($1::bigint[], $2::text[]) WITH ORDINALITY AS t(group_id, model, ord)
		JOIN LATERAL (
		    SELECT id, group_id, model, status, reason_code, winner_account_id,
		           started_at, finished_at, duration_ms, steps
		    FROM model_self_check_probe_rounds
		    WHERE group_id=t.group_id AND model=t.model
		    ORDER BY started_at DESC, id DESC LIMIT 1
		) r ON TRUE
		ORDER BY t.ord`, pq.Array(groupIDs), pq.Array(models))
	if err != nil {
		return nil, fmt.Errorf("list latest model self check probe rounds: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var round service.ModelSelfCheckProbeRound
		var winner sql.NullInt64
		var finished sql.NullTime
		var duration sql.NullInt64
		var steps []byte
		if err := rows.Scan(&round.ID, &round.GroupID, &round.Model, &round.Status, &round.ReasonCode,
			&winner, &round.StartedAt, &finished, &duration, &steps); err != nil {
			return nil, fmt.Errorf("scan model self check probe round: %w", err)
		}
		if winner.Valid {
			round.WinnerAccountID = &winner.Int64
		}
		if finished.Valid {
			round.FinishedAt = &finished.Time
		}
		if duration.Valid {
			value := int(duration.Int64)
			round.DurationMs = &value
		}
		if err := json.Unmarshal(steps, &round.Steps); err != nil {
			return nil, fmt.Errorf("decode model self check probe steps: %w", err)
		}
		if round.Steps == nil {
			round.Steps = []service.ModelSelfCheckProbeStep{}
		}
		out = append(out, round)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate model self check probe rounds: %w", err)
	}
	return out, nil
}

func (r *modelSelfCheckRepository) DeleteProbeRoundsBefore(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM model_self_check_probe_rounds WHERE started_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete model self check probe rounds: %w", err)
	}
	return result.RowsAffected()
}
