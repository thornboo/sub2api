//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type accountArchiveRepoStub struct {
	accountRepoStub
	account     *Account
	getErr      error
	archiveIDs  []int64
	archiveErr  error
	restoreIDs  []int64
	restoreErr  error
	archived    []Account
	archivedRes *pagination.PaginationResult
}

func (s *accountArchiveRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.account == nil || s.account.ID != id {
		return nil, ErrAccountNotFound
	}
	return s.account, nil
}

func (s *accountArchiveRepoStub) Archive(ctx context.Context, id int64) error {
	s.archiveIDs = append(s.archiveIDs, id)
	return s.archiveErr
}

func (s *accountArchiveRepoStub) Restore(ctx context.Context, id int64) error {
	s.restoreIDs = append(s.restoreIDs, id)
	if s.restoreErr != nil {
		return s.restoreErr
	}
	if s.account != nil && s.account.ID == id {
		s.account.Status = StatusDisabled
		s.account.Schedulable = false
	}
	return nil
}

func (s *accountArchiveRepoStub) ListArchivedWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error) {
	if s.archivedRes == nil {
		s.archivedRes = &pagination.PaginationResult{Total: int64(len(s.archived)), Page: params.Page, PageSize: params.PageSize, Pages: 1}
	}
	return s.archived, s.archivedRes, nil
}

func TestAdminServiceArchiveAccountRejectsActiveAccount(t *testing.T) {
	repo := &accountArchiveRepoStub{
		account: &Account{ID: 42, Status: StatusActive, Schedulable: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ArchiveAccount(context.Background(), 42)

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Empty(t, repo.archiveIDs)
}

func TestAdminServiceArchiveAccountRejectsRecoverableNonDisabledStatuses(t *testing.T) {
	for _, status := range []string{"inactive", StatusError} {
		t.Run(status, func(t *testing.T) {
			repo := &accountArchiveRepoStub{
				account: &Account{ID: 42, Status: status, Schedulable: false},
			}
			svc := &adminServiceImpl{accountRepo: repo}

			err := svc.ArchiveAccount(context.Background(), 42)

			require.Error(t, err)
			require.True(t, infraerrors.IsBadRequest(err))
			require.Empty(t, repo.archiveIDs)
		})
	}
}

func TestAdminServiceArchiveAccountAllowsDisabledAccount(t *testing.T) {
	repo := &accountArchiveRepoStub{
		account: &Account{ID: 42, Status: StatusDisabled, Schedulable: false},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	err := svc.ArchiveAccount(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, []int64{42}, repo.archiveIDs)
}

func TestAdminServiceRestoreAccountReturnsDisabledNonSchedulableAccount(t *testing.T) {
	repo := &accountArchiveRepoStub{
		account: &Account{ID: 42, Status: StatusActive, Schedulable: true},
	}
	svc := &adminServiceImpl{accountRepo: repo}

	account, err := svc.RestoreAccount(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, []int64{42}, repo.restoreIDs)
	require.Equal(t, StatusDisabled, account.Status)
	require.False(t, account.Schedulable)
}
