package service

import (
	"context"
	"sort"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type apiKeyRateChangeDisabler interface {
	DisableKeysForGroupRateChange(ctx context.Context, groupID int64) (int64, error)
	DisableKeysForGroupUsersRateChange(ctx context.Context, groupID int64, userIDs []int64) (int64, error)
}

type accountArchiveRepository interface {
	ListArchivedWithFilters(ctx context.Context, params pagination.PaginationParams, platform, accountType, status, search string, groupID int64, privacyMode string) ([]Account, *pagination.PaginationResult, error)
	Archive(ctx context.Context, id int64) error
	Restore(ctx context.Context, id int64) error
}

func (s *adminServiceImpl) disableKeysOnRateChangeEnabled(ctx context.Context) bool {
	return s.settingService != nil && s.settingService.IsDisableKeysOnRateChangeEnabled(ctx)
}

func (s *adminServiceImpl) rateChangeKeyDisabler() (apiKeyRateChangeDisabler, error) {
	disabler, ok := s.apiKeyRepo.(apiKeyRateChangeDisabler)
	if !ok || disabler == nil {
		return nil, infraerrors.InternalServer("RATE_CHANGE_KEY_DISABLER_UNAVAILABLE", "api key repository does not support rate-change key disabling")
	}
	return disabler, nil
}

func (s *adminServiceImpl) runRateChangeTx(ctx context.Context, fn func(context.Context) error) error {
	if fn == nil {
		return nil
	}
	if txRepo, ok := s.apiKeyRepo.(apiKeyTransactionalRepository); ok {
		return txRepo.RunInTx(ctx, fn)
	}
	if s.entClient != nil {
		tx, err := s.entClient.Tx(ctx)
		if err != nil {
			return err
		}
		txCtx := dbent.NewTxContext(ctx, tx)
		defer func() { _ = tx.Rollback() }()
		if err := fn(txCtx); err != nil {
			return err
		}
		return tx.Commit()
	}
	return fn(ctx)
}

func changedRateMultiplierUsers(before map[int64]float64, after map[int64]float64) []int64 {
	if len(before) == 0 && len(after) == 0 {
		return nil
	}
	changed := make([]int64, 0, len(before)+len(after))
	seen := make(map[int64]struct{}, len(before)+len(after))
	for userID, oldRate := range before {
		newRate, ok := after[userID]
		if !ok || newRate != oldRate {
			changed = append(changed, userID)
			seen[userID] = struct{}{}
		}
	}
	for userID, newRate := range after {
		if _, ok := seen[userID]; ok {
			continue
		}
		oldRate, ok := before[userID]
		if !ok || oldRate != newRate {
			changed = append(changed, userID)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i] < changed[j] })
	return changed
}

func groupRateEntriesToMap(entries []UserGroupRateEntry) map[int64]float64 {
	out := make(map[int64]float64, len(entries))
	for _, entry := range entries {
		if entry.RateMultiplier != nil {
			out[entry.UserID] = *entry.RateMultiplier
		}
	}
	return out
}

func groupRateInputsToMap(entries []GroupRateMultiplierInput) map[int64]float64 {
	out := make(map[int64]float64, len(entries))
	for _, entry := range entries {
		out[entry.UserID] = entry.RateMultiplier
	}
	return out
}

func changedUserGroupRateGroups(before map[int64]float64, updates map[int64]*float64) []int64 {
	if len(updates) == 0 {
		return nil
	}
	changed := make([]int64, 0, len(updates))
	for groupID, newRate := range updates {
		oldRate, hadOld := before[groupID]
		if newRate == nil {
			if hadOld {
				changed = append(changed, groupID)
			}
			continue
		}
		if !hadOld || oldRate != *newRate {
			changed = append(changed, groupID)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i] < changed[j] })
	return changed
}

func (s *adminServiceImpl) accountArchiveRepository() (accountArchiveRepository, error) {
	repo, ok := s.accountRepo.(accountArchiveRepository)
	if !ok {
		return nil, infraerrors.InternalServer("ACCOUNT_ARCHIVE_UNSUPPORTED", "account archive is unavailable")
	}
	return repo, nil
}

func (s *adminServiceImpl) ListArchivedAccounts(ctx context.Context, page, pageSize int, platform, accountType, status, search string, groupID int64, privacyMode string, sortBy, sortOrder string) ([]Account, int64, error) {
	repo, err := s.accountArchiveRepository()
	if err != nil {
		return nil, 0, err
	}
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}
	accounts, result, err := repo.ListArchivedWithFilters(ctx, params, platform, accountType, status, search, groupID, privacyMode)
	if err != nil {
		return nil, 0, err
	}
	return accounts, result.Total, nil
}

func (s *adminServiceImpl) ArchiveAccount(ctx context.Context, id int64) error {
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if account.Status != StatusDisabled {
		return infraerrors.BadRequest("ACCOUNT_ARCHIVE_REQUIRES_DISABLED", "account must be disabled before archive")
	}
	repo, err := s.accountArchiveRepository()
	if err != nil {
		return err
	}
	return repo.Archive(ctx, id)
}

func (s *adminServiceImpl) RestoreAccount(ctx context.Context, id int64) (*Account, error) {
	repo, err := s.accountArchiveRepository()
	if err != nil {
		return nil, err
	}
	if err := repo.Restore(ctx, id); err != nil {
		return nil, err
	}
	return s.accountRepo.GetByID(ctx, id)
}
