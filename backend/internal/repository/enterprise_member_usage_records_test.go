package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberRepositoryListUsageRecordsReturnsOwnerSafeProjection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	createdAt := time.Date(2026, time.July, 12, 8, 9, 10, 0, time.UTC)
	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM usage_logs`).
		WithArgs(int64(11), int64(22)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT ul.id, ul.request_id, ul.api_key_id`).
		WithArgs(int64(11), int64(22), 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "api_key_id", "api_key_name", "model", "group_id", "group_name", "request_type",
			"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "actual_cost", "duration_ms", "first_token_ms",
			"billing_mode", "inbound_endpoint", "image_count", "video_count", "created_at",
		}).AddRow(int64(1), "req-safe", int64(33), "member-key", "gpt-5", int64(44), "public-group", int16(2),
			120, 30, 5, 7, 0.125, 900, 80, "token", "/v1/responses", 0, 0, createdAt))

	repo := &enterpriseMemberRepository{db: db}
	items, total, err := repo.ListUsageRecords(context.Background(), 11, 22, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "req-safe", items[0].RequestID)
	require.Equal(t, "stream", items[0].RequestType)
	require.Equal(t, "member-key", items[0].APIKeyName)
	require.Equal(t, "public-group", items[0].GroupName)
	require.Equal(t, 900, *items[0].DurationMs)
	payload, err := json.Marshal(items[0])
	require.NoError(t, err)
	require.NotContains(t, string(payload), "account_id")
	require.NotContains(t, string(payload), "channel_id")
	require.NotContains(t, string(payload), "upstream_")
	require.NotContains(t, string(payload), "schedule_meta")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberRepositoryListUsageRecordsSkipsDataQueryWhenEmpty(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`SELECT COUNT\(\*\)\s+FROM usage_logs`).
		WithArgs(int64(11), int64(22)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))

	repo := &enterpriseMemberRepository{db: db}
	items, total, err := repo.ListUsageRecords(context.Background(), 11, 22, 1, 20)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	require.NoError(t, mock.ExpectationsWereMet())
}
