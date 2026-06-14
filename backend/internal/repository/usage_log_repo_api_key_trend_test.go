package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRepositoryGetAPIKeyUsageTrendForUserUsesTimezoneBucket(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 14, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	end := start.AddDate(0, 0, 1)
	rows := sqlmock.NewRows([]string{
		"date",
		"requests",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"total_tokens",
		"cost",
		"actual_cost",
	}).AddRow("2026-06-14 23:00", int64(2), int64(11), int64(13), int64(3), int64(5), int64(32), 0.21, 0.18)

	mock.ExpectQuery("(?s)TO_CHAR\\(created_at AT TIME ZONE \\$3, 'YYYY-MM-DD HH24:00'\\).*AND user_id = \\$4.*AND api_key_id = \\$5").
		WithArgs(start, end, "Asia/Shanghai", int64(42), int64(7)).
		WillReturnRows(rows)

	trend, err := repo.GetAPIKeyUsageTrendForUser(context.Background(), 42, 7, start, end, "hour", "Asia/Shanghai")
	require.NoError(t, err)
	require.Len(t, trend, 1)
	require.Equal(t, "2026-06-14 23:00", trend[0].Date)
	require.Equal(t, int64(2), trend[0].Requests)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetAPIKeyUsageTrendForUserRejectsInvalidGranularity(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	_, err := repo.GetAPIKeyUsageTrendForUser(context.Background(), 42, 7, start, start.AddDate(0, 0, 1), "minute", "UTC")

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
