package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memberUsageUserRepoStub struct {
	UserRepository
	user *User
	err  error
}

func (s *memberUsageUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, s.err
}

type memberUsageRepoStub struct {
	UsageLogRepository
	members     []OwnerUsageMember
	leaderboard *OwnerMemberLeaderboardResponse
	validated   [2]int64
}

func (s *memberUsageRepoStub) ListOwnerUsageMembers(context.Context, int64) ([]OwnerUsageMember, error) {
	return s.members, nil
}

func (s *memberUsageRepoStub) ValidateOwnerUsageMember(_ context.Context, ownerID, memberID int64) error {
	s.validated = [2]int64{ownerID, memberID}
	return nil
}

func (s *memberUsageRepoStub) GetOwnerMemberAnalyticsLeaderboard(context.Context, OwnerAPIKeyAnalyticsFilters) (*OwnerMemberLeaderboardResponse, error) {
	return s.leaderboard, nil
}

func TestEnterpriseUsageHistoryRemainsAvailableAfterCapabilityIsDisabled(t *testing.T) {
	disabledAt := time.Now()
	repo := &memberUsageRepoStub{members: []OwnerUsageMember{{ID: 42, MemberCode: "finance-01", Name: "Finance"}}}
	svc := NewUsageService(repo, &memberUsageUserRepoStub{user: &User{
		ID:                   7,
		Role:                 RoleUser,
		AccountType:          UserAccountTypeEnterprise,
		EnterpriseDisabledAt: &disabledAt,
	}}, nil, nil)

	members, err := svc.ListOwnerUsageMembers(context.Background(), 7)

	require.NoError(t, err)
	require.Equal(t, repo.members, members)
}

func TestValidateOwnerUsageMemberPreservesOwnerBoundary(t *testing.T) {
	repo := &memberUsageRepoStub{}
	svc := NewUsageService(repo, &memberUsageUserRepoStub{user: &User{ID: 7, Role: RoleUser, AccountType: UserAccountTypeEnterprise}}, nil, nil)

	err := svc.ValidateOwnerUsageMember(context.Background(), 7, 42)

	require.NoError(t, err)
	require.Equal(t, [2]int64{7, 42}, repo.validated)
}

func TestEnterpriseUsageMemberEndpointsRejectNonEnterpriseOwners(t *testing.T) {
	svc := NewUsageService(&memberUsageRepoStub{}, &memberUsageUserRepoStub{user: &User{ID: 7, Role: RoleUser, AccountType: UserAccountTypeIndividual}}, nil, nil)

	_, err := svc.ListOwnerUsageMembers(context.Background(), 7)

	require.ErrorIs(t, err, ErrEnterpriseAccountRequired)
}
