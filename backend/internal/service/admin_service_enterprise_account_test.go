//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type enterpriseLifecycleUserRepoStub struct {
	*rpmUserRepoStub
	hasFacts bool
	factsErr error
	checked  []int64
}

func (s *enterpriseLifecycleUserRepoStub) HasEnterpriseMemberFacts(_ context.Context, userID int64) (bool, error) {
	s.checked = append(s.checked, userID)
	return s.hasFacts, s.factsErr
}

func newEnterpriseLifecycleService(user *User, hasFacts bool) (*adminServiceImpl, *enterpriseLifecycleUserRepoStub, *authCacheInvalidatorStub) {
	base := &userRepoStub{user: user}
	repo := &enterpriseLifecycleUserRepoStub{
		rpmUserRepoStub: &rpmUserRepoStub{userRepoStub: base},
		hasFacts:        hasFacts,
	}
	invalidator := &authCacheInvalidatorStub{}
	return &adminServiceImpl{
		userRepo:             repo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
	}, repo, invalidator
}

func TestAdminServiceUpdateUserRejectsEnterpriseDowngradeWithHistory(t *testing.T) {
	svc, repo, invalidator := newEnterpriseLifecycleService(&User{
		ID:          42,
		Email:       "enterprise@example.com",
		Role:        RoleUser,
		AccountType: UserAccountTypeEnterprise,
	}, true)

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{AccountType: UserAccountTypeIndividual})

	require.ErrorIs(t, err, ErrEnterpriseAccountHasFacts)
	require.Equal(t, []int64{42}, repo.checked)
	require.Nil(t, repo.lastUpdated, "destructive downgrade must not be persisted")
	require.Empty(t, invalidator.userIDs)
}

func TestAdminServiceUpdateUserFailsClosedWhenEnterpriseHistoryCheckFails(t *testing.T) {
	svc, repo, invalidator := newEnterpriseLifecycleService(&User{
		ID:          42,
		Email:       "enterprise@example.com",
		Role:        RoleUser,
		AccountType: UserAccountTypeEnterprise,
	}, false)
	repo.factsErr = errors.New("database unavailable")

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{AccountType: UserAccountTypeIndividual})

	require.ErrorContains(t, err, "check enterprise account history")
	require.Nil(t, repo.lastUpdated)
	require.Empty(t, invalidator.userIDs)
}

func TestAdminServiceUpdateUserAllowsEnterpriseDowngradeWithoutHistory(t *testing.T) {
	disabledAt := time.Now().Add(-time.Hour)
	svc, repo, invalidator := newEnterpriseLifecycleService(&User{
		ID:                   42,
		Email:                "enterprise@example.com",
		Role:                 RoleUser,
		AccountType:          UserAccountTypeEnterprise,
		EnterpriseDisabledAt: &disabledAt,
	}, false)

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{AccountType: UserAccountTypeIndividual})

	require.NoError(t, err)
	require.Equal(t, UserAccountTypeIndividual, updated.AccountType)
	require.Nil(t, updated.EnterpriseDisabledAt)
	require.Equal(t, []int64{42}, repo.checked)
	require.Equal(t, []int64{42}, invalidator.userIDs)
}

func TestAdminServiceUpdateUserDisablesAndRestoresEnterpriseCapability(t *testing.T) {
	svc, repo, invalidator := newEnterpriseLifecycleService(&User{
		ID:          42,
		Email:       "enterprise@example.com",
		Role:        RoleUser,
		AccountType: UserAccountTypeEnterprise,
	}, true)
	disabled := false

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{EnterpriseEnabled: &disabled})
	require.NoError(t, err)
	require.NotNil(t, updated.EnterpriseDisabledAt)
	require.Equal(t, []int64{42}, invalidator.userIDs)

	enabled := true
	updated, err = svc.UpdateUser(context.Background(), 42, &UpdateUserInput{EnterpriseEnabled: &enabled})
	require.NoError(t, err)
	require.Nil(t, updated.EnterpriseDisabledAt)
	require.Equal(t, []int64{42, 42}, invalidator.userIDs)
	require.Empty(t, repo.checked, "capability toggles preserve history and must not enter the destructive downgrade path")
}

func TestAdminServiceUpdateUserRejectsEnterpriseCapabilityForIndividualAccount(t *testing.T) {
	svc, repo, invalidator := newEnterpriseLifecycleService(&User{
		ID:          42,
		Email:       "individual@example.com",
		Role:        RoleUser,
		AccountType: UserAccountTypeIndividual,
	}, false)
	enabled := false

	_, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{EnterpriseEnabled: &enabled})

	require.ErrorContains(t, err, "enterprise_enabled requires account_type=enterprise")
	require.Nil(t, repo.lastUpdated)
	require.Empty(t, invalidator.userIDs)
}

func TestAdminServiceUpdateUserInvalidatesCacheWhenAccountBecomesEnterprise(t *testing.T) {
	svc, _, invalidator := newEnterpriseLifecycleService(&User{
		ID:          42,
		Email:       "individual@example.com",
		Role:        RoleUser,
		AccountType: UserAccountTypeIndividual,
	}, false)

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{AccountType: UserAccountTypeEnterprise})

	require.NoError(t, err)
	require.Equal(t, UserAccountTypeEnterprise, updated.AccountType)
	require.Equal(t, []int64{42}, invalidator.userIDs)
}
