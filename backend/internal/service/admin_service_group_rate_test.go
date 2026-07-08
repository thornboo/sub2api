//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type rateChangeTxMarkerKey struct{}

// userGroupRateRepoStubForGroupRate implements UserGroupRateRepository for group rate tests.
type userGroupRateRepoStubForGroupRate struct {
	getByUserIDData            map[int64]map[int64]float64
	getByUserIDErr             error
	getByUserIDSawRateChangeTx []bool

	getByGroupIDData            map[int64][]UserGroupRateEntry
	getByGroupIDErr             error
	getByGroupIDSawRateChangeTx []bool

	deletedGroupIDs       []int64
	deleteByGroupErr      error
	deleteSawRateChangeTx []bool

	syncUserID              int64
	syncUserRates           map[int64]*float64
	syncUserErr             error
	syncUserSawRateChangeTx []bool

	syncedGroupID       int64
	syncedEntries       []GroupRateMultiplierInput
	syncGroupErr        error
	syncSawRateChangeTx []bool

	rpmSyncedGroupID int64
	rpmSyncedEntries []GroupRPMOverrideInput
	rpmSyncErr       error
}

func (s *userGroupRateRepoStubForGroupRate) GetByUserID(ctx context.Context, userID int64) (map[int64]float64, error) {
	s.getByUserIDSawRateChangeTx = append(s.getByUserIDSawRateChangeTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	if s.getByUserIDErr != nil {
		return nil, s.getByUserIDErr
	}
	if s.getByUserIDData == nil {
		return map[int64]float64{}, nil
	}
	return s.getByUserIDData[userID], nil
}

func (s *userGroupRateRepoStubForGroupRate) GetByUserAndGroup(_ context.Context, _, _ int64) (*float64, error) {
	panic("unexpected GetByUserAndGroup call")
}

func (s *userGroupRateRepoStubForGroupRate) GetRPMOverrideByUserAndGroup(_ context.Context, _, _ int64) (*int, error) {
	panic("unexpected GetRPMOverrideByUserAndGroup call")
}

func (s *userGroupRateRepoStubForGroupRate) GetByGroupID(ctx context.Context, groupID int64) ([]UserGroupRateEntry, error) {
	s.getByGroupIDSawRateChangeTx = append(s.getByGroupIDSawRateChangeTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	if s.getByGroupIDErr != nil {
		return nil, s.getByGroupIDErr
	}
	return s.getByGroupIDData[groupID], nil
}

func (s *userGroupRateRepoStubForGroupRate) SyncUserGroupRates(ctx context.Context, userID int64, rates map[int64]*float64) error {
	s.syncUserID = userID
	s.syncUserRates = rates
	s.syncUserSawRateChangeTx = append(s.syncUserSawRateChangeTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	return s.syncUserErr
}

func (s *userGroupRateRepoStubForGroupRate) SyncGroupRateMultipliers(ctx context.Context, groupID int64, entries []GroupRateMultiplierInput) error {
	s.syncedGroupID = groupID
	s.syncedEntries = entries
	s.syncSawRateChangeTx = append(s.syncSawRateChangeTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	return s.syncGroupErr
}

func (s *userGroupRateRepoStubForGroupRate) SyncGroupRPMOverrides(_ context.Context, groupID int64, entries []GroupRPMOverrideInput) error {
	s.rpmSyncedGroupID = groupID
	s.rpmSyncedEntries = entries
	return s.rpmSyncErr
}

func (s *userGroupRateRepoStubForGroupRate) ClearGroupRPMOverrides(_ context.Context, _ int64) error {
	panic("unexpected ClearGroupRPMOverrides call")
}

func (s *userGroupRateRepoStubForGroupRate) DeleteByGroupID(ctx context.Context, groupID int64) error {
	s.deletedGroupIDs = append(s.deletedGroupIDs, groupID)
	s.deleteSawRateChangeTx = append(s.deleteSawRateChangeTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	return s.deleteByGroupErr
}

func (s *userGroupRateRepoStubForGroupRate) DeleteByUserID(_ context.Context, _ int64) error {
	panic("unexpected DeleteByUserID call")
}

type rateChangeAPIKeyRepoStub struct {
	*apiKeyRepoStub

	txCalls int

	groupDisableIDs       []int64
	groupDisableSawTx     []bool
	groupUserDisableCalls []rateChangeGroupUserCall
	groupUserDisableSawTx []bool
	disableErr            error
}

type rateChangeGroupUserCall struct {
	groupID int64
	userIDs []int64
}

func newRateChangeAPIKeyRepoStub() *rateChangeAPIKeyRepoStub {
	return &rateChangeAPIKeyRepoStub{apiKeyRepoStub: &apiKeyRepoStub{}}
}

func (s *rateChangeAPIKeyRepoStub) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	s.txCalls++
	return fn(context.WithValue(ctx, rateChangeTxMarkerKey{}, true))
}

func (s *rateChangeAPIKeyRepoStub) DisableKeysForGroupRateChange(ctx context.Context, groupID int64) (int64, error) {
	s.groupDisableIDs = append(s.groupDisableIDs, groupID)
	s.groupDisableSawTx = append(s.groupDisableSawTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	return 0, s.disableErr
}

func (s *rateChangeAPIKeyRepoStub) DisableKeysForGroupUsersRateChange(ctx context.Context, groupID int64, userIDs []int64) (int64, error) {
	copiedUserIDs := append([]int64(nil), userIDs...)
	s.groupUserDisableCalls = append(s.groupUserDisableCalls, rateChangeGroupUserCall{
		groupID: groupID,
		userIDs: copiedUserIDs,
	})
	s.groupUserDisableSawTx = append(s.groupUserDisableSawTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	return 0, s.disableErr
}

func newRateChangeGuardSettingService(enabled bool) *SettingService {
	value := "false"
	if enabled {
		value = "true"
	}
	return NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyDisableKeysOnRateChange: value,
	}}, &config.Config{})
}

func groupRateFloat(value float64) *float64 {
	return &value
}

func intPtrForGroupRate(value int) *int {
	return &value
}

type rateChangeUserRepoStub struct {
	*userRepoStub
	updateSawRateChangeTx []bool
}

func (s *rateChangeUserRepoStub) Update(ctx context.Context, user *User) error {
	s.updateSawRateChangeTx = append(s.updateSawRateChangeTx, ctx.Value(rateChangeTxMarkerKey{}) == true)
	if s.userRepoStub != nil {
		return s.userRepoStub.Update(ctx, user)
	}
	return nil
}

func TestAdminService_GetGroupRateMultipliers(t *testing.T) {
	t.Run("returns entries for group", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{
			getByGroupIDData: map[int64][]UserGroupRateEntry{
				10: {
					{UserID: 1, UserName: "alice", UserEmail: "alice@test.com", RateMultiplier: ptrFloat(1.5)},
					{UserID: 2, UserName: "bob", UserEmail: "bob@test.com", RateMultiplier: ptrFloat(0.8)},
				},
			},
		}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		entries, err := svc.GetGroupRateMultipliers(context.Background(), 10)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		require.Equal(t, int64(1), entries[0].UserID)
		require.Equal(t, "alice", entries[0].UserName)
		require.NotNil(t, entries[0].RateMultiplier)
		require.Equal(t, 1.5, *entries[0].RateMultiplier)
		require.Equal(t, int64(2), entries[1].UserID)
		require.NotNil(t, entries[1].RateMultiplier)
		require.Equal(t, 0.8, *entries[1].RateMultiplier)
	})

	t.Run("returns nil when repo is nil", func(t *testing.T) {
		svc := &adminServiceImpl{userGroupRateRepo: nil}

		entries, err := svc.GetGroupRateMultipliers(context.Background(), 10)
		require.NoError(t, err)
		require.Nil(t, entries)
	})

	t.Run("returns empty slice for group with no entries", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{
			getByGroupIDData: map[int64][]UserGroupRateEntry{},
		}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		entries, err := svc.GetGroupRateMultipliers(context.Background(), 99)
		require.NoError(t, err)
		require.Nil(t, entries)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{
			getByGroupIDErr: errors.New("db error"),
		}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		_, err := svc.GetGroupRateMultipliers(context.Background(), 10)
		require.Error(t, err)
		require.Contains(t, err.Error(), "db error")
	})
}

func TestAdminService_ClearGroupRateMultipliers(t *testing.T) {
	t.Run("deletes by group ID", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		err := svc.ClearGroupRateMultipliers(context.Background(), 42)
		require.NoError(t, err)
		require.Equal(t, []int64{42}, repo.deletedGroupIDs)
	})

	t.Run("returns nil when repo is nil", func(t *testing.T) {
		svc := &adminServiceImpl{userGroupRateRepo: nil}

		err := svc.ClearGroupRateMultipliers(context.Background(), 42)
		require.NoError(t, err)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{
			deleteByGroupErr: errors.New("delete failed"),
		}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		err := svc.ClearGroupRateMultipliers(context.Background(), 42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "delete failed")
	})
}

func TestAdminService_UpdateGroup_DisablesKeysWhenDefaultRateChanges(t *testing.T) {
	t.Run("disables keys still using group default rate", func(t *testing.T) {
		groupRepo := &groupRepoStubForAdmin{
			getByID: &Group{
				ID:               10,
				Name:             "default-rate",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1.0,
			},
		}
		apiKeyRepo := newRateChangeAPIKeyRepoStub()
		svc := &adminServiceImpl{
			groupRepo:      groupRepo,
			apiKeyRepo:     apiKeyRepo,
			settingService: newRateChangeGuardSettingService(true),
		}
		newRate := 1.2

		updated, err := svc.UpdateGroup(context.Background(), 10, &UpdateGroupInput{RateMultiplier: &newRate})

		require.NoError(t, err)
		require.NotNil(t, updated)
		require.Equal(t, 1.2, groupRepo.updated.RateMultiplier)
		require.Equal(t, 1, apiKeyRepo.txCalls)
		require.Equal(t, []bool{true}, groupRepo.updateSawRateChangeTx)
		require.Equal(t, []int64{10}, apiKeyRepo.groupDisableIDs)
		require.Equal(t, []bool{true}, apiKeyRepo.groupDisableSawTx)
	})

	t.Run("skips disabler when rate is unchanged", func(t *testing.T) {
		groupRepo := &groupRepoStubForAdmin{
			getByID: &Group{
				ID:               11,
				Name:             "unchanged-rate",
				Platform:         PlatformOpenAI,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeStandard,
				RateMultiplier:   1.2,
			},
		}
		svc := &adminServiceImpl{
			groupRepo:      groupRepo,
			settingService: newRateChangeGuardSettingService(true),
		}
		sameRate := 1.2

		updated, err := svc.UpdateGroup(context.Background(), 11, &UpdateGroupInput{RateMultiplier: &sameRate})

		require.NoError(t, err)
		require.NotNil(t, updated)
		require.NotNil(t, groupRepo.updated)
		require.Equal(t, []bool{false}, groupRepo.updateSawRateChangeTx)
	})
}

func TestAdminService_BatchSetGroupRateMultipliers(t *testing.T) {
	t.Run("syncs entries to repo", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		entries := []GroupRateMultiplierInput{
			{UserID: 1, RateMultiplier: 1.5},
			{UserID: 2, RateMultiplier: 0.8},
		}
		err := svc.BatchSetGroupRateMultipliers(context.Background(), 10, entries)
		require.NoError(t, err)
		require.Equal(t, int64(10), repo.syncedGroupID)
		require.Equal(t, entries, repo.syncedEntries)
	})

	t.Run("returns nil when repo is nil", func(t *testing.T) {
		svc := &adminServiceImpl{userGroupRateRepo: nil}

		err := svc.BatchSetGroupRateMultipliers(context.Background(), 10, nil)
		require.NoError(t, err)
	})

	t.Run("propagates repo error", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{
			syncGroupErr: errors.New("sync failed"),
		}
		svc := &adminServiceImpl{userGroupRateRepo: repo}

		err := svc.BatchSetGroupRateMultipliers(context.Background(), 10, []GroupRateMultiplierInput{
			{UserID: 1, RateMultiplier: 1.0},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "sync failed")
	})

	t.Run("disables keys for changed user-specific rates when enabled", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{
			getByGroupIDData: map[int64][]UserGroupRateEntry{
				10: {
					{UserID: 1, RateMultiplier: groupRateFloat(1.5)},
					{UserID: 2, RateMultiplier: groupRateFloat(0.8)},
					{UserID: 4, RPMOverride: intPtrForGroupRate(60)},
				},
			},
		}
		apiKeyRepo := newRateChangeAPIKeyRepoStub()
		svc := &adminServiceImpl{
			userGroupRateRepo: repo,
			apiKeyRepo:        apiKeyRepo,
			settingService:    newRateChangeGuardSettingService(true),
		}
		entries := []GroupRateMultiplierInput{
			{UserID: 1, RateMultiplier: 1.5},
			{UserID: 2, RateMultiplier: 0.9},
			{UserID: 3, RateMultiplier: 1.2},
		}

		err := svc.BatchSetGroupRateMultipliers(context.Background(), 10, entries)

		require.NoError(t, err)
		require.Equal(t, int64(10), repo.syncedGroupID)
		require.Equal(t, entries, repo.syncedEntries)
		require.Equal(t, 1, apiKeyRepo.txCalls)
		require.Equal(t, []bool{true}, repo.getByGroupIDSawRateChangeTx)
		require.Equal(t, []bool{true}, repo.syncSawRateChangeTx)
		require.Equal(t, []rateChangeGroupUserCall{{groupID: 10, userIDs: []int64{2, 3}}}, apiKeyRepo.groupUserDisableCalls)
		require.Equal(t, []bool{true}, apiKeyRepo.groupUserDisableSawTx)
	})
}

func TestAdminService_ClearGroupRateMultipliers_DisablesAffectedKeysWhenEnabled(t *testing.T) {
	repo := &userGroupRateRepoStubForGroupRate{
		getByGroupIDData: map[int64][]UserGroupRateEntry{
			10: {
				{UserID: 1, RateMultiplier: groupRateFloat(1.5)},
				{UserID: 2, RPMOverride: intPtrForGroupRate(80)},
				{UserID: 3, RateMultiplier: groupRateFloat(0.9)},
			},
		},
	}
	apiKeyRepo := newRateChangeAPIKeyRepoStub()
	svc := &adminServiceImpl{
		userGroupRateRepo: repo,
		apiKeyRepo:        apiKeyRepo,
		settingService:    newRateChangeGuardSettingService(true),
	}

	err := svc.ClearGroupRateMultipliers(context.Background(), 10)

	require.NoError(t, err)
	require.Equal(t, []int64{10}, repo.deletedGroupIDs)
	require.Equal(t, 1, apiKeyRepo.txCalls)
	require.Equal(t, []bool{true}, repo.getByGroupIDSawRateChangeTx)
	require.Equal(t, []bool{true}, repo.deleteSawRateChangeTx)
	require.Equal(t, []rateChangeGroupUserCall{{groupID: 10, userIDs: []int64{1, 3}}}, apiKeyRepo.groupUserDisableCalls)
	require.Equal(t, []bool{true}, apiKeyRepo.groupUserDisableSawTx)
}

func TestAdminService_UpdateUser_DisablesKeysWhenUserGroupRatesChange(t *testing.T) {
	baseUserRepo := &userRepoStub{user: &User{
		ID:          42,
		Email:       "user-rate@example.com",
		Status:      StatusActive,
		Role:        RoleUser,
		Concurrency: 1,
	}}
	userRepo := &rateChangeUserRepoStub{userRepoStub: baseUserRepo}
	groupRateRepo := &userGroupRateRepoStubForGroupRate{
		getByUserIDData: map[int64]map[int64]float64{
			42: {
				10: 1.5,
				20: 0.8,
				30: 2.0,
			},
		},
	}
	apiKeyRepo := newRateChangeAPIKeyRepoStub()
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{
		userRepo:             userRepo,
		userGroupRateRepo:    groupRateRepo,
		apiKeyRepo:           apiKeyRepo,
		redeemCodeRepo:       &redeemRepoStub{},
		authCacheInvalidator: invalidator,
		settingService:       newRateChangeGuardSettingService(true),
	}
	sameRate := 1.5
	changedRate := 0.9
	addedRate := 1.2
	inputRates := map[int64]*float64{
		10: &sameRate,
		20: &changedRate,
		30: nil,
		40: &addedRate,
	}

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		GroupRates: inputRates,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, apiKeyRepo.txCalls)
	require.Equal(t, []bool{true}, groupRateRepo.getByUserIDSawRateChangeTx)
	require.Equal(t, []bool{true}, userRepo.updateSawRateChangeTx)
	require.Equal(t, []bool{true}, groupRateRepo.syncUserSawRateChangeTx)
	require.Equal(t, int64(42), groupRateRepo.syncUserID)
	require.Equal(t, inputRates, groupRateRepo.syncUserRates)
	require.Equal(t, []rateChangeGroupUserCall{
		{groupID: 20, userIDs: []int64{42}},
		{groupID: 30, userIDs: []int64{42}},
		{groupID: 40, userIDs: []int64{42}},
	}, apiKeyRepo.groupUserDisableCalls)
	require.Equal(t, []bool{true, true, true}, apiKeyRepo.groupUserDisableSawTx)
	require.Equal(t, []int64{42}, invalidator.userIDs)
}

func TestAdminService_UpdateUser_DoesNotDisableKeysWhenUserGroupRatesUnchanged(t *testing.T) {
	baseUserRepo := &userRepoStub{user: &User{
		ID:          42,
		Email:       "user-rate@example.com",
		Status:      StatusActive,
		Role:        RoleUser,
		Concurrency: 1,
	}}
	userRepo := &rateChangeUserRepoStub{userRepoStub: baseUserRepo}
	groupRateRepo := &userGroupRateRepoStubForGroupRate{
		getByUserIDData: map[int64]map[int64]float64{
			42: {
				10: 1.5,
				20: 0.8,
			},
		},
	}
	apiKeyRepo := newRateChangeAPIKeyRepoStub()
	svc := &adminServiceImpl{
		userRepo:          userRepo,
		userGroupRateRepo: groupRateRepo,
		apiKeyRepo:        apiKeyRepo,
		redeemCodeRepo:    &redeemRepoStub{},
		settingService:    newRateChangeGuardSettingService(true),
	}
	sameRate := 1.5
	alsoSameRate := 0.8

	updated, err := svc.UpdateUser(context.Background(), 42, &UpdateUserInput{
		GroupRates: map[int64]*float64{
			10: &sameRate,
			20: &alsoSameRate,
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 1, apiKeyRepo.txCalls)
	require.Equal(t, []bool{true}, groupRateRepo.getByUserIDSawRateChangeTx)
	require.Equal(t, []bool{true}, userRepo.updateSawRateChangeTx)
	require.Equal(t, []bool{true}, groupRateRepo.syncUserSawRateChangeTx)
	require.Empty(t, apiKeyRepo.groupUserDisableCalls)
}

func TestAdminService_BatchSetGroupRPMOverrides(t *testing.T) {
	t.Run("syncs entries to repo", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{}
		svc := &adminServiceImpl{userGroupRateRepo: repo}
		override := 20
		entries := []GroupRPMOverrideInput{{UserID: 2, RPMOverride: &override}}

		err := svc.BatchSetGroupRPMOverrides(context.Background(), 10, entries)
		require.NoError(t, err)
		require.Equal(t, int64(10), repo.rpmSyncedGroupID)
		require.Equal(t, entries, repo.rpmSyncedEntries)
	})

	t.Run("rejects negative override as bad request", func(t *testing.T) {
		repo := &userGroupRateRepoStubForGroupRate{}
		svc := &adminServiceImpl{userGroupRateRepo: repo}
		negative := -1

		err := svc.BatchSetGroupRPMOverrides(context.Background(), 10, []GroupRPMOverrideInput{
			{UserID: 2, RPMOverride: &negative},
		})
		require.Error(t, err)
		require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
		require.Zero(t, repo.rpmSyncedGroupID)
	})
}
