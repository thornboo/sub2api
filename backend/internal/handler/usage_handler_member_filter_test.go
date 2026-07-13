package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func usageMemberFilterContext(t *testing.T, query string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/v1/usage?"+query, nil)
	return c
}

func TestParseUsageMemberFilters(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		memberID  *int64
		scope     string
		set       bool
		errString string
	}{
		{name: "absent", query: "", set: false},
		{name: "specific member", query: "member_id=42", memberID: func() *int64 { v := int64(42); return &v }(), scope: usagestats.MemberScopeAll, set: true},
		{name: "assigned", query: "member_scope=assigned", scope: usagestats.MemberScopeAssigned, set: true},
		{name: "unassigned", query: "member_scope=unassigned", scope: usagestats.MemberScopeUnassigned, set: true},
		{name: "invalid id", query: "member_id=0", set: true, errString: "Invalid member_id"},
		{name: "invalid scope", query: "member_scope=archived", set: true, errString: "Invalid member_scope, allowed values are all, assigned, unassigned"},
		{name: "conflicting selectors", query: "member_id=42&member_scope=assigned", set: true, errString: "member_id cannot be combined with assigned or unassigned member_scope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memberID, scope, set, errString := parseUsageMemberFilters(usageMemberFilterContext(t, tt.query))
			require.Equal(t, tt.memberID, memberID)
			require.Equal(t, tt.scope, scope)
			require.Equal(t, tt.set, set)
			require.Equal(t, tt.errString, errString)
		})
	}
}

func TestAPIKeyMatchesMemberFilter(t *testing.T) {
	memberID := int64(42)
	otherMemberID := int64(99)
	assignedKey := &service.APIKey{MemberID: &memberID}
	unassignedKey := &service.APIKey{}

	require.True(t, apiKeyMatchesMemberFilter(nil, &memberID, usagestats.MemberScopeAll))
	require.True(t, apiKeyMatchesMemberFilter(assignedKey, &memberID, usagestats.MemberScopeAll))
	require.False(t, apiKeyMatchesMemberFilter(assignedKey, &otherMemberID, usagestats.MemberScopeAll))
	require.True(t, apiKeyMatchesMemberFilter(assignedKey, nil, usagestats.MemberScopeAssigned))
	require.False(t, apiKeyMatchesMemberFilter(unassignedKey, nil, usagestats.MemberScopeAssigned))
	require.True(t, apiKeyMatchesMemberFilter(unassignedKey, nil, usagestats.MemberScopeUnassigned))
}
