package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRebindPostgresPlaceholdersTreatsEachPlaceholderAsOneToken(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		offset int
		want   string
	}{
		{name: "single digit becomes two digits", value: "ul.api_key_id = $5", offset: 5, want: "ul.api_key_id = $10"},
		{name: "mixed widths are each rebound once", value: "$1, $5, $10", offset: 5, want: "$6, $10, $15"},
		{name: "zero offset is unchanged", value: "ul.member_id = $4", offset: 0, want: "ul.member_id = $4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, rebindPostgresPlaceholders(tt.value, tt.offset))
		})
	}
}

func TestRebindOwnerMemberPreviousConditionsSupportsCombinedFilters(t *testing.T) {
	memberID := int64(42)
	apiKeyID := int64(11)
	groupID := int64(9)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filters := service.OwnerAPIKeyAnalyticsFilters{
		UserID:      7,
		MemberID:    &memberID,
		APIKeyID:    &apiKeyID,
		GroupID:     &groupID,
		StartTime:   start,
		EndTime:     end,
		MemberScope: usagestats.MemberScopeAll,
	}

	currentConditions, currentArgs, err := ownerMemberUsageConditions(filters, start, end)
	require.NoError(t, err)
	previousConditions, _, err := ownerMemberUsageConditions(filters, start.Add(-24*time.Hour), start)
	require.NoError(t, err)
	rebound := rebindSQLConditions(previousConditions, len(currentArgs))
	joined := strings.Join(rebound, " ")

	require.Len(t, currentConditions, 6)
	require.Contains(t, joined, "ul.user_id = $7")
	require.Contains(t, joined, "ul.member_id = $10")
	require.Contains(t, joined, "ul.api_key_id = $11")
	require.Contains(t, joined, "ul.group_id = $12")
	require.NotContains(t, joined, "$60")
}

func TestGetOwnerMemberAnalyticsLeaderboardReturnsOnlyRealMemberScope(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"member_id", "member_code", "member_name", "status", "archived", "key_count",
		"monthly_limit_usd", "current_used_usd", "current_reserved_usd", "requests",
		"input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens",
		"total_tokens", "actual_cost", "previous_actual_cost", "last_used_at", "total_items",
		"member_count", "budget_risk_member_count", "total_reserved_usd", "total_actual_cost",
	}).AddRow(
		int64(42), "finance-01", "Finance", "active", false, int64(2),
		100.0, 80.0, 5.0, int64(12),
		int64(100), int64(50), int64(10), int64(20),
		int64(180), 30.0, 20.0, now, int64(36),
		int64(36), int64(4), 18.5, 120.0,
	)
	mock.ExpectQuery(`(?s)member_scope AS \(\s*SELECT em\.id AS member_id\s*FROM enterprise_members em\s*WHERE em\.enterprise_user_id = \$\d+ AND em\.removed_at IS NULL\s*\),\s*ranked AS`).WillReturnRows(rows)

	result, err := repo.GetOwnerMemberAnalyticsLeaderboard(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:      7,
		MemberScope: usagestats.MemberScopeAll,
		StartTime:   now.Add(-24 * time.Hour),
		EndTime:     now,
		Limit:       20,
	})

	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, int64(36), result.Total)
	require.Equal(t, int64(36), result.MemberCount)
	require.Equal(t, int64(4), result.BudgetRiskMemberCount)
	require.Equal(t, 18.5, result.TotalReservedUSD)
	require.Equal(t, 120.0, result.TotalActualCost)
	require.Equal(t, 30.0, result.DisplayedActualCost)
	require.Equal(t, 25.0, result.Items[0].SharePercent)
	require.Equal(t, 50.0, result.Items[0].ChangePercent)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetOwnerMemberAnalyticsLeaderboardReturnsNoVirtualMemberForRegularKeyScope(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	now := time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)

	result, err := repo.GetOwnerMemberAnalyticsLeaderboard(context.Background(), service.OwnerAPIKeyAnalyticsFilters{
		UserID:      7,
		MemberScope: usagestats.MemberScopeUnassigned,
		StartTime:   now.Add(-24 * time.Hour),
		EndTime:     now,
		Limit:       20,
	})

	require.NoError(t, err)
	require.Empty(t, result.Items)
	require.Zero(t, result.Total)
	require.Zero(t, result.MemberCount)
	require.NoError(t, mock.ExpectationsWereMet())
}
