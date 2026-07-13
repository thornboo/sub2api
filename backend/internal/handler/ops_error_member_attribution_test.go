package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestApplyOpsMemberAttributionCapturesRequestTimeIdentity(t *testing.T) {
	memberID := int64(42)
	apiKey := &service.APIKey{
		MemberID: &memberID,
		Member: &service.EnterpriseMember{
			ID:         memberID,
			MemberCode: " finance-01 ",
			Name:       " Finance Team ",
		},
	}
	entry := &service.OpsInsertErrorLogInput{}

	applyOpsMemberAttribution(entry, apiKey)

	require.NotNil(t, entry.MemberID)
	require.Equal(t, memberID, *entry.MemberID)
	require.Equal(t, "finance-01", entry.MemberCodeSnapshot)
	require.Equal(t, "Finance Team", entry.MemberNameSnapshot)
}

func TestApplyOpsMemberAttributionKeepsIDWhenMemberRelationIsUnavailable(t *testing.T) {
	memberID := int64(42)
	entry := &service.OpsInsertErrorLogInput{}

	applyOpsMemberAttribution(entry, &service.APIKey{MemberID: &memberID})

	require.NotNil(t, entry.MemberID)
	require.Equal(t, memberID, *entry.MemberID)
	require.Empty(t, entry.MemberCodeSnapshot)
	require.Empty(t, entry.MemberNameSnapshot)
}
