package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
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

func TestUsageLogRepositoryGetOwnerAPIKeyUsageTrendUsesTimezoneAndDoubleUserFilter(t *testing.T) {
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
		"actual_cost",
	}).AddRow("2026-06-14 23:00", int64(2), int64(11), int64(13), int64(3), int64(5), int64(32), 0.18)

	mock.ExpectQuery("(?s)TO_CHAR\\(ul\\.created_at AT TIME ZONE \\$5, 'YYYY-MM-DD HH24:00'\\).*ul\\.user_id = \\$1.*ul\\.created_at >= \\$2.*ul\\.created_at < \\$3.*ak\\.user_id = \\$4.*ak\\.deleted_at IS NULL").
		WithArgs(int64(42), start, end, int64(42), "Asia/Shanghai").
		WillReturnRows(rows)

	trend, err := repo.GetOwnerAPIKeyUsageTrend(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:       42,
		StartTime:    start,
		EndTime:      end,
		TimezoneName: "Asia/Shanghai",
		Granularity:  "hour",
	})
	require.NoError(t, err)
	require.Len(t, trend, 1)
	require.Equal(t, "2026-06-14 23:00", trend[0].Date)
	require.Equal(t, int64(2), trend[0].Requests)
	require.Equal(t, 0.18, trend[0].ActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetOwnerAPIKeyUsageTrendFiltersAPIKeyWithinOwner(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	apiKeyID := int64(77)
	rows := sqlmock.NewRows([]string{
		"date",
		"requests",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"total_tokens",
		"actual_cost",
	}).AddRow("2026-06-14", int64(1), int64(10), int64(20), int64(0), int64(5), int64(35), 0.08)

	mock.ExpectQuery("(?s)TO_CHAR\\(ul\\.created_at AT TIME ZONE \\$6, 'YYYY-MM-DD'\\).*ul\\.user_id = \\$1.*ul\\.created_at >= \\$2.*ul\\.created_at < \\$3.*ak\\.user_id = \\$4.*ak\\.deleted_at IS NULL.*ak\\.id = \\$5").
		WithArgs(int64(42), start, end, int64(42), apiKeyID, "UTC").
		WillReturnRows(rows)

	trend, err := repo.GetOwnerAPIKeyUsageTrend(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:       42,
		APIKeyID:     &apiKeyID,
		StartTime:    start,
		EndTime:      end,
		TimezoneName: "UTC",
		Granularity:  "day",
	})
	require.NoError(t, err)
	require.Len(t, trend, 1)
	require.Equal(t, "2026-06-14", trend[0].Date)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetOwnerAPIKeyUsageTrendRejectsInvalidGranularity(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	_, err := repo.GetOwnerAPIKeyUsageTrend(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:       42,
		StartTime:    start,
		EndTime:      start.AddDate(0, 0, 1),
		TimezoneName: "UTC",
		Granularity:  "minute",
	})

	require.Error(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOwnerAnalyticsLimitUsesSharedDefaults(t *testing.T) {
	require.Equal(t, service.DefaultOwnerAPIKeyAnalyticsLimit, ownerAnalyticsLimit(0))
	require.Equal(t, service.DefaultOwnerAPIKeyAnalyticsLimit, ownerAnalyticsLimit(-1))
	require.Equal(t, 7, ownerAnalyticsLimit(7))
	require.Equal(t, service.MaxOwnerAPIKeyAnalyticsLimit, ownerAnalyticsLimit(service.MaxOwnerAPIKeyAnalyticsLimit+1))
}

func TestUsageLogRepositoryGetOwnerAPIKeyLeaderboardReturnsDisplayedCostAndGlobalShare(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	previousStart := start.Add(-end.Sub(start))

	mock.ExpectQuery("(?s)COUNT\\(DISTINCT ul\\.api_key_id\\).*FROM usage_logs ul\\s+JOIN api_keys ak ON ul\\.api_key_id = ak\\.id.*ul\\.user_id = \\$1.*ak\\.user_id = \\$4").
		WithArgs(int64(42), start, end, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"total_keys", "total_actual_cost"}).AddRow(int64(3), 100.0))

	rows := sqlmock.NewRows([]string{
		"id",
		"name",
		"tags",
		"group_id",
		"name",
		"status",
		"last_used_at",
		"requests",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"total_tokens",
		"actual_cost",
	}).
		AddRow(int64(7), "Alice", `["team-a"]`, nil, "", service.StatusAPIKeyActive, nil, int64(10), int64(100), int64(20), int64(3), int64(5), int64(128), 60.0).
		AddRow(int64(8), "Bob", `[]`, nil, "", service.StatusAPIKeyActive, nil, int64(2), int64(40), int64(10), int64(0), int64(0), int64(50), 15.0)
	mock.ExpectQuery("(?s)SELECT\\s+ak\\.id.*FROM usage_logs ul\\s+JOIN api_keys ak ON ul\\.api_key_id = ak\\.id.*LEFT JOIN groups g ON g\\.id = ak\\.group_id.*LIMIT \\$5").
		WithArgs(int64(42), start, end, int64(42), 2).
		WillReturnRows(rows)

	mock.ExpectQuery("(?s)SELECT ul\\.api_key_id, COALESCE\\(SUM\\(ul\\.actual_cost\\), 0\\).*FROM usage_logs ul\\s+JOIN api_keys ak ON ul\\.api_key_id = ak\\.id.*ul\\.user_id = \\$1.*ak\\.user_id = \\$4.*ul\\.api_key_id = ANY\\(\\$5\\)").
		WithArgs(int64(42), previousStart, start, int64(42), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"api_key_id", "actual_cost"}).AddRow(int64(7), 30.0))

	got, err := repo.GetOwnerAPIKeyAnalyticsLeaderboard(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:      42,
		StartTime:   start,
		EndTime:     end,
		Granularity: "day",
		Limit:       2,
	})
	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	require.Equal(t, int64(3), got.Total)
	require.Equal(t, 100.0, got.TotalActualCost)
	require.Equal(t, 75.0, got.DisplayedActualCost)
	require.Equal(t, 60.0, got.Items[0].SharePercent)
	require.Equal(t, 15.0, got.Items[1].SharePercent)
	require.Equal(t, 30.0, got.Items[0].PreviousActualCost)
	require.Equal(t, 100.0, got.Items[0].ChangePercent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOwnerAPIKeyPreviousActualCostPreservesHistoricalMemberAndGroupScope(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	previousStart := start.Add(-end.Sub(start))
	memberID := int64(42)
	groupID := int64(9)
	filters := service.OwnerAPIKeyAnalyticsFilters{
		UserID:          7,
		MemberID:        &memberID,
		MemberScope:     usagestats.MemberScopeAll,
		MemberFilterSet: true,
		GroupID:         &groupID,
		StartTime:       start,
		EndTime:         end,
	}

	mock.ExpectQuery("(?s)SELECT ul\\.api_key_id.*FROM usage_logs ul\\s+JOIN api_keys ak ON ul\\.api_key_id = ak\\.id.*ul\\.user_id = \\$1.*ul\\.created_at >= \\$2.*ul\\.created_at < \\$3.*ul\\.member_id = \\$4.*ul\\.group_id = \\$5.*ul\\.api_key_id = ANY\\(\\$6\\)").
		WithArgs(int64(7), previousStart, start, memberID, groupID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"api_key_id", "actual_cost"}).AddRow(int64(11), 12.5))

	got, err := repo.ownerAPIKeyPreviousActualCost(context.Background(), filters, []int64{11})
	require.NoError(t, err)
	require.Equal(t, map[int64]float64{11: 12.5}, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetOwnerAPIKeyLeaderboardFiltersAPIKeyWithinOwner(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	apiKeyID := int64(99)

	mock.ExpectQuery("(?s)COUNT\\(DISTINCT ul\\.api_key_id\\).*FROM usage_logs ul\\s+JOIN api_keys ak ON ul\\.api_key_id = ak\\.id.*ul\\.user_id = \\$1.*ak\\.user_id = \\$4.*ak\\.id = \\$5").
		WithArgs(int64(42), start, end, int64(42), apiKeyID).
		WillReturnRows(sqlmock.NewRows([]string{"total_keys", "total_actual_cost"}).AddRow(int64(0), 0.0))
	mock.ExpectQuery("(?s)SELECT\\s+ak\\.id.*FROM usage_logs ul\\s+JOIN api_keys ak ON ul\\.api_key_id = ak\\.id.*ul\\.user_id = \\$1.*ak\\.user_id = \\$4.*ak\\.id = \\$5.*LIMIT \\$6").
		WithArgs(int64(42), start, end, int64(42), apiKeyID, service.DefaultOwnerAPIKeyAnalyticsLimit).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"name",
			"tags",
			"group_id",
			"name",
			"status",
			"last_used_at",
			"requests",
			"input_tokens",
			"output_tokens",
			"cache_creation_tokens",
			"cache_read_tokens",
			"total_tokens",
			"actual_cost",
		}))

	got, err := repo.GetOwnerAPIKeyAnalyticsLeaderboard(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:      42,
		APIKeyID:    &apiKeyID,
		StartTime:   start,
		EndTime:     end,
		Granularity: "day",
	})
	require.NoError(t, err)
	require.Empty(t, got.Items)
	require.Equal(t, int64(0), got.Total)
	require.Equal(t, 0.0, got.TotalActualCost)
	require.Equal(t, 0.0, got.DisplayedActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetOwnerAPIKeyTagAnalyticsDeduplicatesKeyTagsInSQL(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 7)
	rows := sqlmock.NewRows([]string{
		"tag",
		"key_count",
		"requests",
		"input_tokens",
		"output_tokens",
		"cache_creation_tokens",
		"cache_read_tokens",
		"total_tokens",
		"actual_cost",
	}).AddRow("frontend", int64(1), int64(2), int64(100), int64(20), int64(0), int64(5), int64(125), 1.25)

	mock.ExpectQuery("(?s)CROSS JOIN LATERAL \\(\\s*SELECT DISTINCT btrim\\(raw_tag\\.value\\) AS value.*jsonb_array_elements_text\\(COALESCE\\(ak\\.tags, '\\[\\]'::jsonb\\)\\).*GROUP BY tag\\.value").
		WithArgs(int64(42), start, end, int64(42), service.DefaultOwnerAPIKeyAnalyticsLimit).
		WillReturnRows(rows)

	got, err := repo.GetOwnerAPIKeyTagAnalytics(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:      42,
		StartTime:   start,
		EndTime:     end,
		Granularity: "day",
	})
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, "frontend", got[0].Tag)
	require.Equal(t, int64(1), got[0].KeyCount)
	require.Equal(t, int64(2), got[0].Requests)
	require.NoError(t, mock.ExpectationsWereMet())
}
