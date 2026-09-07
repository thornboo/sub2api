package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestModelSelfCheckRepositoryListStatusSnapshotMetricsEmptyInput(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewModelSelfCheckRepository(db)
	rows, err := repo.ListStatusSnapshotMetrics(context.Background(), nil, time.Now())
	require.NoError(t, err)
	require.Empty(t, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelSelfCheckRepositoryListStatusSnapshotMetricsMapsNullableWindows(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 9, 7, 10, 30, 0, 0, time.FixedZone("CST", 8*60*60))
	nowUTC := now.UTC()
	repo := NewModelSelfCheckRepository(db)

	mock.ExpectQuery("WITH target_input AS").
		WithArgs(
			pgArrayArg{want: []int64{10, 20, 10, 20}},
			pgArrayArg{want: []string{"gpt-4o", "gpt-4o", "claude-sonnet", "gemini-pro"}},
			nowUTC.AddDate(0, 0, -30),
			nowUTC,
			nowUTC.AddDate(0, 0, -1),
			nowUTC.AddDate(0, 0, -7),
			service.MonitorStatusOperational,
			service.MonitorStatusDegraded,
		).
		WillReturnRows(sqlmock.NewRows([]string{
			"group_id",
			"model",
			"has_snapshots",
			"availability_24h",
			"availability_7d",
			"availability_30d",
			"avg_latency_24h_ms",
			"avg_latency_7d_ms",
			"degraded_ratio_24h",
		}).
			AddRow(int64(10), "gpt-4o", true, 66.6666667, 80.0, 90.0, int64(123), int64(456), 33.3333333).
			AddRow(int64(20), "gpt-4o", true, nil, nil, nil, nil, nil, nil).
			AddRow(int64(10), "claude-sonnet", false, nil, nil, nil, nil, nil, nil).
			AddRow(int64(20), "gemini-pro", true, nil, nil, nil, nil, nil, nil))

	rows, err := repo.ListStatusSnapshotMetrics(context.Background(), []service.ModelSelfCheckTarget{
		{GroupID: 10, Model: "gpt-4o"},
		{GroupID: 20, Model: "gpt-4o"},
		{GroupID: 10, Model: "claude-sonnet"},
		{GroupID: 20, Model: "gemini-pro"},
	}, now)
	require.NoError(t, err)
	require.Len(t, rows, 4)

	require.Equal(t, int64(10), rows[0].GroupID)
	require.Equal(t, "gpt-4o", rows[0].Model)
	require.True(t, rows[0].HasSnapshots)
	require.InDelta(t, 66.6666667, *rows[0].Availability24h, 0.000001)
	require.InDelta(t, 80.0, *rows[0].Availability7d, 0.000001)
	require.InDelta(t, 90.0, *rows[0].Availability30d, 0.000001)
	require.Equal(t, 123, *rows[0].AvgLatency24hMs)
	require.Equal(t, 456, *rows[0].AvgLatency7dMs)
	require.InDelta(t, 33.3333333, *rows[0].DegradedRatio24h, 0.000001)

	require.Equal(t, int64(20), rows[1].GroupID)
	require.Equal(t, "gpt-4o", rows[1].Model)
	require.True(t, rows[1].HasSnapshots)
	require.Nil(t, rows[1].Availability24h)
	require.Nil(t, rows[1].Availability7d)
	require.Nil(t, rows[1].Availability30d)
	require.Nil(t, rows[1].AvgLatency24hMs)
	require.Nil(t, rows[1].AvgLatency7dMs)
	require.Nil(t, rows[1].DegradedRatio24h)

	require.Equal(t, int64(10), rows[2].GroupID)
	require.Equal(t, "claude-sonnet", rows[2].Model)
	require.False(t, rows[2].HasSnapshots)
	require.Nil(t, rows[2].Availability24h)

	require.Equal(t, int64(20), rows[3].GroupID)
	require.Equal(t, "gemini-pro", rows[3].Model)
	require.True(t, rows[3].HasSnapshots)
	require.Nil(t, rows[3].Availability24h)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelSelfCheckRepositoryListStatusSnapshotMetricsPropagatesQueryError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	now := time.Date(2026, 9, 7, 2, 30, 0, 0, time.UTC)
	queryErr := errors.New("database unavailable")
	repo := NewModelSelfCheckRepository(db)

	mock.ExpectQuery("WITH target_input AS").
		WithArgs(
			pgArrayArg{want: []int64{10}},
			pgArrayArg{want: []string{"gpt-4o"}},
			now.AddDate(0, 0, -30),
			now,
			now.AddDate(0, 0, -1),
			now.AddDate(0, 0, -7),
			service.MonitorStatusOperational,
			service.MonitorStatusDegraded,
		).
		WillReturnError(queryErr)

	rows, err := repo.ListStatusSnapshotMetrics(context.Background(), []service.ModelSelfCheckTarget{
		{GroupID: 10, Model: "gpt-4o"},
	}, now)
	require.Nil(t, rows)
	require.ErrorIs(t, err, queryErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

type pgArrayArg struct {
	want any
}

func (a pgArrayArg) Match(value driver.Value) bool {
	want, err := pq.Array(a.want).Value()
	if err != nil {
		return false
	}
	return driverValueString(value) == driverValueString(want)
}

func driverValueString(value any) string {
	switch v := value.(type) {
	case []byte:
		return string(v)
	default:
		return fmt.Sprint(v)
	}
}
