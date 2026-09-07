package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type probeStepsArgument struct {
	want []service.ModelSelfCheckProbeStep
}

func (a probeStepsArgument) Match(v driver.Value) bool {
	value, ok := v.(string)
	if !ok {
		return false
	}
	var steps []service.ModelSelfCheckProbeStep
	if json.Unmarshal([]byte(value), &steps) != nil {
		return false
	}
	actual, _ := json.Marshal(steps)
	expected, _ := json.Marshal(a.want)
	return string(actual) == string(expected)
}

func TestModelSelfCheckProbeRoundWritesSnapshotAndFinalizesOnlyOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo, ok := NewModelSelfCheckRepository(db).(service.ModelSelfCheckRoundRepository)
	require.True(t, ok)
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	round := &service.ModelSelfCheckProbeRound{
		GroupID: 2, Model: "deepseek-pro", Status: "checking", StartedAt: now,
		Steps: []service.ModelSelfCheckProbeStep{{AccountID: 31, AccountName: "C", Priority: 2, Order: 1, Outcome: "pending"}},
	}
	mock.ExpectBegin()
	mock.ExpectExec("SELECT pg_advisory_xact_lock").WithArgs("2:deepseek-pro").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE model_self_check_probe_rounds[\\s\\S]*INTERVAL '120 seconds'").WithArgs(int64(2), "deepseek-pro").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("INSERT INTO model_self_check_probe_rounds").
		WithArgs(int64(2), "deepseek-pro", "checking", "", nil, now, nil, nil, probeStepsArgument{round.Steps}).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(4))
	mock.ExpectCommit()
	require.NoError(t, repo.CreateProbeRound(context.Background(), round))
	require.EqualValues(t, 4, round.ID)
	round.Status, round.ReasonCode = "operational", "ok"
	round.FinishedAt = &now
	winner, duration := int64(31), 120
	round.WinnerAccountID, round.DurationMs = &winner, &duration
	round.Steps[0].Outcome = "succeeded"
	for _, affected := range []int64{1, 0} {
		mock.ExpectExec("UPDATE model_self_check_probe_rounds[\\s\\S]*WHERE id=\\$1 AND finished_at IS NULL").
			WithArgs(round.ID, round.Status, round.ReasonCode, winner, now, duration, probeStepsArgument{round.Steps}).
			WillReturnResult(sqlmock.NewResult(0, affected))
		err := repo.UpdateProbeRound(context.Background(), round)
		if affected == 1 {
			require.NoError(t, err)
		} else {
			require.ErrorContains(t, err, "already finished")
		}
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelSelfCheckLatestProbeRoundsKeepExactTargetPairsAndNulls(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo, ok := NewModelSelfCheckRepository(db).(service.ModelSelfCheckRoundRepository)
	require.True(t, ok)
	now := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	columns := []string{"id", "group_id", "model", "status", "reason_code", "winner_account_id", "started_at", "finished_at", "duration_ms", "steps"}
	mock.ExpectQuery("FROM UNNEST[\\s\\S]*WHERE group_id=t.group_id AND model=t.model[\\s\\S]*ORDER BY started_at DESC, id DESC LIMIT 1").
		WithArgs("{2,3}", `{"deepseek-pro","deepseek-flash"}`).
		WillReturnRows(sqlmock.NewRows(columns).
			AddRow(11, 2, "deepseek-pro", "checking", "", nil, now, nil, nil, "[]").
			AddRow(12, 3, "deepseek-flash", "degraded", "partial_degraded", 30, now, now, 300,
				`[{"account_id":30,"account_name":"Historical name","priority":3,"order":2,"outcome":"succeeded"}]`))
	rows, err := repo.ListLatestProbeRounds(context.Background(), []service.ModelSelfCheckTarget{{GroupID: 2, Model: "deepseek-pro"}, {GroupID: 3, Model: "deepseek-flash"}})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Nil(t, rows[0].WinnerAccountID)
	require.Nil(t, rows[0].FinishedAt)
	require.Nil(t, rows[0].DurationMs)
	require.NotNil(t, rows[0].Steps)
	require.Equal(t, "Historical name", rows[1].Steps[0].AccountName)
	require.Equal(t, 3, rows[1].Steps[0].Priority)
	require.EqualValues(t, 30, *rows[1].WinnerAccountID)
	require.Equal(t, 300, *rows[1].DurationMs)
	empty, err := repo.ListLatestProbeRounds(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, empty)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelSelfCheckProbeRoundStorageErrorsAreNotEmptyEvidence(t *testing.T) {
	for _, tc := range []struct {
		name     string
		payload  string
		queryErr bool
	}{
		{"query failure", "", true}, {"corrupt steps", "{bad json", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			repo, ok := NewModelSelfCheckRepository(db).(service.ModelSelfCheckRoundRepository)
			require.True(t, ok)
			query := mock.ExpectQuery("FROM UNNEST")
			if tc.queryErr {
				query.WillReturnError(errors.New("database unavailable"))
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "model", "status", "reason_code", "winner_account_id", "started_at", "finished_at", "duration_ms", "steps"}).
					AddRow(1, 2, "pro", "checking", "", nil, time.Now(), nil, nil, tc.payload))
			}
			rows, err := repo.ListLatestProbeRounds(context.Background(), []service.ModelSelfCheckTarget{{GroupID: 2, Model: "pro"}})
			require.Error(t, err)
			require.Nil(t, rows)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestModelSelfCheckProbeRoundRetentionUsesExplicitCutoff(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo, ok := NewModelSelfCheckRepository(db).(service.ModelSelfCheckRoundRepository)
	require.True(t, ok)
	before := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec("DELETE FROM model_self_check_probe_rounds WHERE started_at < \\$1").WithArgs(before).WillReturnResult(sqlmock.NewResult(0, 5))
	n, err := repo.DeleteProbeRoundsBefore(context.Background(), before)
	require.NoError(t, err)
	require.EqualValues(t, 5, n)
	require.NoError(t, mock.ExpectationsWereMet())
}
