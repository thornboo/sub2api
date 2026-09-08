package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestModelRuntimeRepositoryAggregateModelRuntimeScansBuckets(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &modelRuntimeRepository{db: db}
	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"group_id", "model", "hour", "successes", "failures",
		"first_token_sum_ms", "first_token_count", "duration_sum_ms", "duration_count",
		"output_tokens", "generation_time_ms",
	}).AddRow(int64(7), "claude-fable-5", 3, int64(9), int64(1), 1200.5, int64(8), 4100.25, int64(9), 320.0, 6400.0)

	mock.ExpectQuery(regexp.QuoteMeta(modelRuntimeAggregationSQL)).
		WithArgs(start, end).
		WillReturnRows(rows)

	buckets, err := repo.AggregateModelRuntime(context.Background(), start, end)
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	require.Equal(t, int64(7), buckets[0].GroupID)
	require.Equal(t, "claude-fable-5", buckets[0].Model)
	require.Equal(t, 3, buckets[0].Hour)
	require.Equal(t, int64(9), buckets[0].Successes)
	require.Equal(t, int64(1), buckets[0].Failures)
	require.Equal(t, 1200.5, buckets[0].FirstTokenSumMS)
	require.Equal(t, int64(8), buckets[0].FirstTokenCount)
	require.Equal(t, 4100.25, buckets[0].DurationSumMS)
	require.Equal(t, int64(9), buckets[0].DurationCount)
	require.Equal(t, 320.0, buckets[0].OutputTokens)
	require.Equal(t, 6400.0, buckets[0].GenerationTimeMS)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelRuntimeRepositoryAggregateModelRuntimePropagatesQueryError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &modelRuntimeRepository{db: db}
	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	queryErr := errors.New("query failed")

	mock.ExpectQuery(regexp.QuoteMeta(modelRuntimeAggregationSQL)).
		WithArgs(start, end).
		WillReturnError(queryErr)

	buckets, err := repo.AggregateModelRuntime(context.Background(), start, end)
	require.ErrorIs(t, err, queryErr)
	require.Nil(t, buckets)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelRuntimeRepositoryAggregateModelRuntimePropagatesScanError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &modelRuntimeRepository{db: db}
	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"group_id", "model", "hour", "successes", "failures",
		"first_token_sum_ms", "first_token_count", "duration_sum_ms", "duration_count",
		"output_tokens", "generation_time_ms",
	}).AddRow("not-a-group-id", "claude-fable-5", 3, int64(9), int64(1), 1200.5, int64(8), 4100.25, int64(9), 320.0, 6400.0)

	mock.ExpectQuery(regexp.QuoteMeta(modelRuntimeAggregationSQL)).
		WithArgs(start, end).
		WillReturnRows(rows)

	buckets, err := repo.AggregateModelRuntime(context.Background(), start, end)
	require.Error(t, err)
	require.Nil(t, buckets)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelRuntimeRepositoryAggregateModelRuntimePropagatesRowsError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &modelRuntimeRepository{db: db}
	start := time.Date(2026, 9, 8, 0, 15, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	rowsErr := errors.New("rows failed")

	rows := sqlmock.NewRows([]string{
		"group_id", "model", "hour", "successes", "failures",
		"first_token_sum_ms", "first_token_count", "duration_sum_ms", "duration_count",
		"output_tokens", "generation_time_ms",
	}).AddRow(int64(7), "claude-fable-5", 3, int64(9), int64(1), 1200.5, int64(8), 4100.25, int64(9), 320.0, 6400.0).
		RowError(0, rowsErr)

	mock.ExpectQuery(regexp.QuoteMeta(modelRuntimeAggregationSQL)).
		WithArgs(start, end).
		WillReturnRows(rows)

	buckets, err := repo.AggregateModelRuntime(context.Background(), start, end)
	require.ErrorIs(t, err, rowsErr)
	require.Nil(t, buckets)
	require.NoError(t, mock.ExpectationsWereMet())
}
