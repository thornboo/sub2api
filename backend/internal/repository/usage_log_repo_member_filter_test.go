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
		{name: "owner all excludes removed tombstones", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeAll, OwnerVisibleMembers: true}, conditions: []string{"(member_id IS NULL OR (member_id IS NOT NULL AND EXISTS (SELECT 1 FROM enterprise_members visible_member WHERE visible_member.id = member_id AND visible_member.enterprise_user_id = user_id AND visible_member.removed_at IS NULL)))"}},
		{name: "owner empty scope excludes removed tombstones", filters: usagestats.UsageLogFilters{OwnerVisibleMembers: true}, alias: "ul", conditions: []string{"(ul.member_id IS NULL OR (ul.member_id IS NOT NULL AND EXISTS (SELECT 1 FROM enterprise_members visible_member WHERE visible_member.id = ul.member_id AND visible_member.enterprise_user_id = ul.user_id AND visible_member.removed_at IS NULL)))"}},
		{name: "audit assigned retains tombstones", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeAssigned}, conditions: []string{"member_id IS NOT NULL"}},
		{name: "owner assigned with alias excludes tombstones", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeAssigned, OwnerVisibleMembers: true}, alias: "ul", conditions: []string{"ul.member_id IS NOT NULL AND EXISTS (SELECT 1 FROM enterprise_members visible_member WHERE visible_member.id = ul.member_id AND visible_member.enterprise_user_id = ul.user_id AND visible_member.removed_at IS NULL)"}},
		{name: "unassigned with alias", filters: usagestats.UsageLogFilters{MemberScope: usagestats.MemberScopeUnassigned}, alias: "ul", conditions: []string{"ul.member_id IS NULL"}},
		{name: "specific member wins", filters: usagestats.UsageLogFilters{MemberID: &memberID, MemberScope: usagestats.MemberScopeAssigned}, alias: "ul", conditions: []string{"ul.member_id = $2"}, args: []any{int64(7), memberID}},
		{name: "owner specific member also requires current visibility", filters: usagestats.UsageLogFilters{MemberID: &memberID, OwnerVisibleMembers: true}, alias: "ul", conditions: []string{"ul.member_id = $1", "ul.member_id IS NOT NULL AND EXISTS (SELECT 1 FROM enterprise_members visible_member WHERE visible_member.id = ul.member_id AND visible_member.enterprise_user_id = ul.user_id AND visible_member.removed_at IS NULL)"}, args: []any{memberID}},
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

func TestAppendUsageLogMemberQueryFilterPreservesAllOwnerConditions(t *testing.T) {
	memberID := int64(42)
	query, args := appendUsageLogMemberQueryFilter(
		"SELECT 1 FROM usage_logs WHERE user_id = $1",
		[]any{int64(7)},
		usagestats.UsageLogFilters{MemberID: &memberID, OwnerVisibleMembers: true},
		"",
	)

	require.Equal(t, []any{int64(7), memberID}, args)
	require.Contains(t, query, "member_id = $2 AND member_id IS NOT NULL AND EXISTS")
	require.Contains(t, query, "visible_member.id = member_id")
	require.Contains(t, query, "visible_member.enterprise_user_id = user_id")
	require.Contains(t, query, "visible_member.removed_at IS NULL")
}

func TestOwnerAnalyticsAssignedMembersExcludeRemovedTombstones(t *testing.T) {
	conditions, args, err := ownerAnalyticsUsageConditions(service.OwnerAPIKeyAnalyticsFilters{
		UserID:          7,
		MemberScope:     usagestats.MemberScopeAssigned,
		MemberFilterSet: true,
		StartTime:       time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	}, true)

	require.NoError(t, err)
	require.Len(t, args, 3)
	joined := strings.Join(conditions, " ")
	require.Contains(t, joined, "ul.member_id IS NOT NULL")
	require.Contains(t, joined, "visible_member.id = ul.member_id")
	require.Contains(t, joined, "visible_member.enterprise_user_id = ul.user_id")
	require.Contains(t, joined, "visible_member.removed_at IS NULL")
	require.NotContains(t, joined, "visible_member.deleted_at", "archived members remain part of owner-visible history")
}

func TestOwnerAnalyticsAllMembersExcludeRemovedTombstonesWithoutForcingAssignment(t *testing.T) {
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
	require.Contains(t, joined, "ul.member_id IS NULL OR")
	require.Contains(t, joined, "visible_member.id = ul.member_id")
	require.Contains(t, joined, "visible_member.enterprise_user_id = ul.user_id")
	require.Contains(t, joined, "visible_member.removed_at IS NULL")
	require.NotContains(t, joined, "visible_member.deleted_at")
}

func TestOwnerAnalyticsEmptyMemberScopeStillExcludesRemovedTombstones(t *testing.T) {
	conditions, _, err := ownerAnalyticsUsageConditions(service.OwnerAPIKeyAnalyticsFilters{
		UserID:    7,
		StartTime: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	}, true)

	require.NoError(t, err)
	joined := strings.Join(conditions, " ")
	require.Contains(t, joined, "ak.deleted_at IS NULL")
	require.Contains(t, joined, "ul.member_id IS NULL OR")
	require.Contains(t, joined, "visible_member.id = ul.member_id")
	require.Contains(t, joined, "visible_member.removed_at IS NULL")
}
