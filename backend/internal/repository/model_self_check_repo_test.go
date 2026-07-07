package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestModelSelfCheckRepositoryCreateHistoryWritesTokenUsage(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	latency := 123
	httpStatus := 200
	checkedAt := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	repo := NewModelSelfCheckRepository(db)

	mock.ExpectQuery("INSERT INTO model_self_check_histories").
		WithArgs(
			"gpt-4o",
			int64(7),
			service.PlatformOpenAI,
			service.MonitorStatusOperational,
			&latency,
			&httpStatus,
			nil,
			12,
			1,
			checkedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))

	history := &service.ModelSelfCheckHistory{
		Model:        "gpt-4o",
		AccountID:    7,
		Platform:     service.PlatformOpenAI,
		Status:       service.MonitorStatusOperational,
		LatencyMs:    &latency,
		HTTPStatus:   &httpStatus,
		InputTokens:  12,
		OutputTokens: 1,
		CheckedAt:    checkedAt,
	}
	require.NoError(t, repo.CreateHistory(context.Background(), history))
	require.Equal(t, int64(99), history.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestModelSelfCheckRepositoryListTokenUsageSince(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	since := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	repo := NewModelSelfCheckRepository(db)

	mock.ExpectQuery("SELECT model,\\s+COALESCE\\(SUM\\(input_tokens\\), 0\\) AS input_tokens,\\s+COALESCE\\(SUM\\(output_tokens\\), 0\\) AS output_tokens\\s+FROM model_self_check_histories\\s+WHERE checked_at >= \\$1\\s+GROUP BY model\\s+ORDER BY model").
		WithArgs(since).
		WillReturnRows(sqlmock.NewRows([]string{"model", "input_tokens", "output_tokens"}).
			AddRow("claude-sonnet", int64(30), int64(1)).
			AddRow("gpt-4o", int64(12), int64(2)))

	rows, err := repo.ListTokenUsageSince(context.Background(), since)
	require.NoError(t, err)
	require.Equal(t, []service.ModelSelfCheckTokenUsage{
		{Model: "claude-sonnet", InputTokens: 30, OutputTokens: 1, TotalTokens: 31},
		{Model: "gpt-4o", InputTokens: 12, OutputTokens: 2, TotalTokens: 14},
	}, rows)
	require.NoError(t, mock.ExpectationsWereMet())
}
