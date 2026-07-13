package service

import (
	"context"
	"fmt"
)

type memberUsageAnalyticsRepository interface {
	ListOwnerUsageMembers(ctx context.Context, ownerID int64) ([]OwnerUsageMember, error)
	ValidateOwnerUsageMember(ctx context.Context, ownerID, memberID int64) error
	GetOwnerMemberAnalyticsLeaderboard(ctx context.Context, filters OwnerAPIKeyAnalyticsFilters) (*OwnerMemberLeaderboardResponse, error)
}

// requireEnterpriseUsageOwner intentionally allows a disabled enterprise account.
// Disabling prevents new member-key traffic, but it must not erase or hide historical evidence.
func (s *UsageService) requireEnterpriseUsageOwner(ctx context.Context, ownerID int64) error {
	if ownerID <= 0 {
		return ErrEnterpriseAccountRequired
	}
	user, err := s.userRepo.GetByID(ctx, ownerID)
	if err != nil {
		return err
	}
	if user.Role != RoleUser || user.AccountType != UserAccountTypeEnterprise {
		return ErrEnterpriseAccountRequired
	}
	return nil
}

func (s *UsageService) ValidateEnterpriseUsageOwner(ctx context.Context, ownerID int64) error {
	return s.requireEnterpriseUsageOwner(ctx, ownerID)
}

func (s *UsageService) ListOwnerUsageMembers(ctx context.Context, ownerID int64) ([]OwnerUsageMember, error) {
	if err := s.requireEnterpriseUsageOwner(ctx, ownerID); err != nil {
		return nil, err
	}
	repo, ok := s.usageRepo.(memberUsageAnalyticsRepository)
	if !ok {
		return nil, fmt.Errorf("member usage analytics repository is not available")
	}
	members, err := repo.ListOwnerUsageMembers(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("list owner usage members: %w", err)
	}
	return members, nil
}

func (s *UsageService) ValidateOwnerUsageMember(ctx context.Context, ownerID, memberID int64) error {
	if memberID <= 0 {
		return ErrEnterpriseMemberNotFound
	}
	if err := s.requireEnterpriseUsageOwner(ctx, ownerID); err != nil {
		return err
	}
	repo, ok := s.usageRepo.(memberUsageAnalyticsRepository)
	if !ok {
		return fmt.Errorf("member usage analytics repository is not available")
	}
	if err := repo.ValidateOwnerUsageMember(ctx, ownerID, memberID); err != nil {
		return fmt.Errorf("validate owner usage member: %w", err)
	}
	return nil
}

func (s *UsageService) GetOwnerMemberAnalyticsLeaderboard(ctx context.Context, filters OwnerAPIKeyAnalyticsFilters) (*OwnerMemberLeaderboardResponse, error) {
	if err := s.requireEnterpriseUsageOwner(ctx, filters.UserID); err != nil {
		return nil, err
	}
	repo, ok := s.usageRepo.(memberUsageAnalyticsRepository)
	if !ok {
		return nil, fmt.Errorf("member usage analytics repository is not available")
	}
	result, err := repo.GetOwnerMemberAnalyticsLeaderboard(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("get owner member analytics leaderboard: %w", err)
	}
	return result, nil
}
