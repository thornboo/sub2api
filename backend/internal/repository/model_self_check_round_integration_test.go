//go:build integration

package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestModelSelfCheckProbeRoundPostgresEvidence(t *testing.T) {
	ctx := context.Background()
	db := integrationDB // The harness provisions disposable PostgreSQL, never the development database.
	var groupA, groupB int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO groups (name, platform) VALUES ('probe-round-a','deepseek') RETURNING id`).Scan(&groupA))
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO groups (name, platform) VALUES ('probe-round-b','deepseek') RETURNING id`).Scan(&groupB))
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM groups WHERE id IN ($1,$2)`, groupA, groupB) })
	// Reapplying the append-only migration must not erase existing evidence.
	schema, err := migrations.FS.ReadFile("237_model_self_check_probe_rounds.sql")
	require.NoError(t, err)
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(schema))
	if err != nil {
		_ = tx.Rollback()
	}
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	repo := NewModelSelfCheckRepository(db).(service.ModelSelfCheckRoundRepository)
	now := time.Now().UTC().Truncate(time.Microsecond)
	old := now.Add(-time.Hour)
	first := &service.ModelSelfCheckProbeRound{GroupID: groupA, Model: "pro", Status: "failed", StartedAt: old, FinishedAt: &old, Steps: []service.ModelSelfCheckProbeStep{}}
	require.NoError(t, repo.CreateProbeRound(ctx, first))
	latest := &service.ModelSelfCheckProbeRound{GroupID: groupA, Model: "pro", Status: "checking", StartedAt: now, Steps: []service.ModelSelfCheckProbeStep{{AccountID: 1234567, AccountName: "C at probe time", Priority: 2, Order: 1, Outcome: "pending"}}}
	require.NoError(t, repo.CreateProbeRound(ctx, latest))
	duplicate := &service.ModelSelfCheckProbeRound{GroupID: groupA, Model: "pro", Status: "checking", StartedAt: now}
	require.ErrorIs(t, repo.CreateProbeRound(ctx, duplicate), service.ErrModelSelfCheckRoundInProgress)
	require.Zero(t, duplicate.ID)
	// Other group/model pairs must not leak into the requested pair set.
	other := &service.ModelSelfCheckProbeRound{GroupID: groupB, Model: "pro", Status: "operational", StartedAt: now, FinishedAt: &now, Steps: []service.ModelSelfCheckProbeStep{}}
	require.NoError(t, repo.CreateProbeRound(ctx, other))
	targets := []service.ModelSelfCheckTarget{{GroupID: groupA, Model: "pro"}, {GroupID: groupB, Model: "flash"}}
	rows, err := repo.ListLatestProbeRounds(ctx, targets)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, latest.ID, rows[0].ID)
	require.Nil(t, rows[0].FinishedAt)
	latest.Status, latest.ReasonCode = "operational", "ok"
	latest.FinishedAt = &now
	winner, elapsed := int64(1234567), 21
	latest.WinnerAccountID, latest.DurationMs = &winner, &elapsed
	latest.Steps[0].Outcome = "succeeded"
	require.NoError(t, repo.UpdateProbeRound(ctx, latest))
	latest.Steps[0].AccountName = "changed after completion"
	require.Error(t, repo.UpdateProbeRound(ctx, latest))
	rows, err = repo.ListLatestProbeRounds(ctx, targets)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "C at probe time", rows[0].Steps[0].AccountName)
	require.Equal(t, 2, rows[0].Steps[0].Priority)
	require.Equal(t, winner, *rows[0].WinnerAccountID)
	require.Equal(t, elapsed, *rows[0].DurationMs)
	n, err := repo.DeleteProbeRoundsBefore(ctx, now.Add(-time.Minute))
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	rows, err = repo.ListLatestProbeRounds(ctx, targets)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	// Array and non-negative duration invariants are enforced by PostgreSQL.
	_, err = db.ExecContext(ctx, `UPDATE model_self_check_probe_rounds SET steps='{}'::jsonb WHERE id=$1`, latest.ID)
	require.Error(t, err)
	_, err = db.ExecContext(ctx, `UPDATE model_self_check_probe_rounds SET duration_ms=-1 WHERE id=$1`, latest.ID)
	require.Error(t, err)

	// A crashed worker's claim expires; the successor cannot rewrite its steps.
	staleStart := now.Add(-3 * time.Minute)
	stale := &service.ModelSelfCheckProbeRound{GroupID: groupB, Model: "flash", Status: "checking", StartedAt: staleStart,
		Steps: []service.ModelSelfCheckProbeStep{{AccountID: 77, AccountName: "stale worker", Priority: 2, Order: 1, Outcome: "pending", StartedAt: &staleStart}}}
	require.NoError(t, repo.CreateProbeRound(ctx, stale))
	successor := &service.ModelSelfCheckProbeRound{GroupID: groupB, Model: "flash", Status: "checking", StartedAt: now}
	require.NoError(t, repo.CreateProbeRound(ctx, successor))
	var status, reason, outcome, name string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status,reason_code,steps->0->>'outcome',steps->0->>'account_name' FROM model_self_check_probe_rounds WHERE id=$1 AND finished_at IS NOT NULL`, stale.ID).Scan(&status, &reason, &outcome, &name))
	require.Equal(t, "unknown", status)
	require.Equal(t, "round_incomplete", reason)
	require.Equal(t, "incomplete", outcome)
	require.Equal(t, "stale worker", name)
	require.Error(t, repo.UpdateProbeRound(ctx, stale))

	// Competing app instances can claim only one unfinished round.
	results := make(chan error, 8)
	for i := 0; i < cap(results); i++ {
		go func() {
			results <- repo.CreateProbeRound(ctx, &service.ModelSelfCheckProbeRound{GroupID: groupB, Model: "exp", Status: "checking", StartedAt: now})
		}()
	}
	claimed := 0
	for i := 0; i < cap(results); i++ {
		err := <-results
		if err == nil {
			claimed++
		} else {
			require.True(t, errors.Is(err, service.ErrModelSelfCheckRoundInProgress), "%v", err)
		}
	}
	require.Equal(t, 1, claimed)
}
