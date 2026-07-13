package repository

import (
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAppendUsageLogMemberWhereCondition(t *testing.T) {
	memberID := int64(42)
	tests := []struct {
		name       string
		filters    usagestats.UsageLogFilters
		alias      string
		conditions []string
		args       []any
	}{
		{name: "all is additive no-op", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeAll}},
		{name: "assigned", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeAssigned}, conditions: []string{"member_id IS NOT NULL"}},
		{name: "unassigned with alias", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeUnassigned}, alias: "ul", conditions: []string{"ul.member_id IS NULL"}},
		{name: "specific member wins", filters: usagestats.UsageLogFilters{MemberID: &memberID, MemberScope: usagestats.MemberScopeAssigned}, alias: "ul", conditions: []string{"ul.member_id = $2"}, args: []any{int64(7), memberID}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialArgs := []any(nil)
			if tt.name == "specific member wins" {
				initialArgs = []any{int64(7)}
			}
			conditions, args := appendUsageLogMemberWhereCondition(nil, initialArgs, tt.filters, tt.alias)
			require.Equal(t, tt.conditions, conditions)
			require.Equal(t, tt.args, args)
		})
	}
}

func TestOwnerAnalyticsAllMembersUsesRequestFactsWithoutForcingAssignment(t *testing.T) {
	conditions, args, err := ownerAnalyticsUsageConditions(service.OwnerAPIKeyAnalyticsFilters{
		UserID:          7,
		MemberScope:     usagestats.MemberScopeAll,
		MemberFilterSet: true,
		StartTime:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
		GroupID:         func() *int64 { value := int64(9); return &value }(),
	}, true)

	require.NoError(t, err)
	require.NotEmpty(t, args)
	joined := strings.Join(conditions, " ")
	require.Contains(t, joined, "ul.user_id")
	require.Contains(t, joined, "ul.group_id")
	require.NotContains(t, joined, "ak.group_id")
	require.NotContains(t, joined, "ul.member_id IS NOT NULL")
	require.NotContains(t, joined, "ul.member_id IS NULL")
}
