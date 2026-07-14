package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type archivedMemberAccessRepo struct {
	EnterpriseMemberRepository
	includeArchivedCalls []bool
	keys                 []APIKey
	restoredVersion      int64
	deletedMemberID      int64
}

func (r *archivedMemberAccessRepo) GetByOwnerAndID(_ context.Context, ownerID, memberID int64, includeArchived bool) (*EnterpriseMember, error) {
	r.includeArchivedCalls = append(r.includeArchivedCalls, includeArchived)
	if !includeArchived {
		return nil, ErrEnterpriseMemberNotFound
	}
	deletedAt := time.Now()
	return &EnterpriseMember{ID: memberID, EnterpriseUserID: ownerID, Status: EnterpriseMemberStatusDisabled, DeletedAt: &deletedAt}, nil
}

func (r *archivedMemberAccessRepo) ListKeys(_ context.Context, _, _ int64) ([]APIKey, error) {
	return r.keys, nil
}

func (r *archivedMemberAccessRepo) Restore(_ context.Context, ownerID, memberID, expectedVersion int64) (*EnterpriseMember, error) {
	r.restoredVersion = expectedVersion
	return &EnterpriseMember{ID: memberID, EnterpriseUserID: ownerID, Status: EnterpriseMemberStatusDisabled, Version: expectedVersion + 1}, nil
}

func (r *archivedMemberAccessRepo) DeletePermanently(_ context.Context, _, memberID int64) (*EnterpriseMemberDeletionResult, error) {
	r.deletedMemberID = memberID
	return &EnterpriseMemberDeletionResult{Mode: EnterpriseMemberDeletionModeTombstone}, nil
}

type archivedMemberOwnerRepo struct {
	UserRepository
}

func TestEnterpriseMemberArchivedLifecycleSupportsRestoreAndPermanentRemoval(t *testing.T) {
	t.Parallel()

	repo := &archivedMemberAccessRepo{}
	memberService := NewEnterpriseMemberService(repo, &archivedMemberOwnerRepo{}, nil)

	restored, err := memberService.Restore(context.Background(), 7, 11, 4)
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberStatusDisabled, restored.Status)
	require.Equal(t, int64(4), repo.restoredVersion)

	deleted, err := memberService.DeletePermanently(context.Background(), 7, 11)
	require.NoError(t, err)
	require.Equal(t, EnterpriseMemberDeletionModeTombstone, deleted.Mode)
	require.Equal(t, int64(11), repo.deletedMemberID)
}

func (r *archivedMemberOwnerRepo) GetByID(_ context.Context, id int64) (*User, error) {
	return &User{ID: id, Role: RoleUser, AccountType: UserAccountTypeEnterprise, Status: StatusActive}, nil
}

func TestEnterpriseMemberArchivedAccessIsReadOnly(t *testing.T) {
	t.Parallel()

	repo := &archivedMemberAccessRepo{keys: []APIKey{{ID: 31, Name: "historical-key"}}}
	memberService := NewEnterpriseMemberService(repo, &archivedMemberOwnerRepo{}, nil)

	keys, err := memberService.ListKeys(context.Background(), 7, 11)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, []bool{true}, repo.includeArchivedCalls)

	changedName := "changed"
	_, err = memberService.UpdateKey(context.Background(), 7, 11, 31, UpdateAPIKeyRequest{Name: &changedName})
	require.ErrorIs(t, err, ErrEnterpriseMemberNotFound)

	err = memberService.DeleteKey(context.Background(), 7, 11, 31)
	require.ErrorIs(t, err, ErrEnterpriseMemberNotFound)
	require.Equal(t, []bool{true, false, false}, repo.includeArchivedCalls)
}
