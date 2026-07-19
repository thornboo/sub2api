package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type enterpriseMemberBatchRepositorySpy struct {
	EnterpriseMemberRepository
	targets []EnterpriseMemberBatchTarget
	patch   BatchEnterpriseMemberPolicyPatch
}

func (r *enterpriseMemberBatchRepositorySpy) BatchUpdate(_ context.Context, _ int64, targets []EnterpriseMemberBatchTarget, patch BatchEnterpriseMemberPolicyPatch) ([]BatchEnterpriseMemberUpdate, error) {
	r.targets = append([]EnterpriseMemberBatchTarget(nil), targets...)
	r.patch = patch
	return []BatchEnterpriseMemberUpdate{{ID: targets[0].ID, Version: targets[0].ExpectedVersion + 1}}, nil
}

func TestEnterpriseMemberBatchUpdateOnlyChangesExplicitFields(t *testing.T) {
	repo := &enterpriseMemberBatchRepositorySpy{}
	memberService := NewEnterpriseMemberService(repo, &archivedMemberOwnerRepo{}, nil, nil)
	monthlyLimit := 125.0
	status := EnterpriseMemberStatusDisabled

	updated, err := memberService.BatchUpdate(context.Background(), 7, BatchUpdateEnterpriseMembersInput{
		Members:         []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		MonthlyLimitUSD: &monthlyLimit,
		Status:          &status,
		GroupMode:       "keep",
	})

	require.NoError(t, err)
	require.Equal(t, []BatchEnterpriseMemberUpdate{{ID: 11, Version: 4}}, updated)
	require.Equal(t, []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}}, repo.targets)
	require.Same(t, &monthlyLimit, repo.patch.MonthlyLimitUSD)
	require.Nil(t, repo.patch.RateLimit5h)
	require.Nil(t, repo.patch.RateLimit1d)
	require.Nil(t, repo.patch.RateLimit7d)
	require.Same(t, &status, repo.patch.Status)
	require.Equal(t, "keep", repo.patch.GroupMode)
}

func TestEnterpriseMemberBatchUpdateRejectsNoopAppend(t *testing.T) {
	memberService := NewEnterpriseMemberService(&enterpriseMemberBatchRepositorySpy{}, &archivedMemberOwnerRepo{}, nil, nil)

	_, err := memberService.BatchUpdate(context.Background(), 7, BatchUpdateEnterpriseMembersInput{
		Members:   []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		GroupMode: "append",
	})

	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)
}

func TestEnterpriseMemberBatchUpdateRejectsIgnoredGroupsInKeepMode(t *testing.T) {
	memberService := NewEnterpriseMemberService(&enterpriseMemberBatchRepositorySpy{}, &archivedMemberOwnerRepo{}, nil, nil)
	limit := 25.0

	_, err := memberService.BatchUpdate(context.Background(), 7, BatchUpdateEnterpriseMembersInput{
		Members:         []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		MonthlyLimitUSD: &limit,
		GroupMode:       "keep",
		GroupIDs:        []int64{9},
	})

	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)
}

func TestEnterpriseMemberBatchTargetsRejectOversizedRequest(t *testing.T) {
	targets := make([]EnterpriseMemberBatchTarget, EnterpriseMemberBatchMaxSize+1)
	for index := range targets {
		targets[index] = EnterpriseMemberBatchTarget{ID: int64(index + 1), ExpectedVersion: 1}
	}

	require.ErrorIs(t, validateEnterpriseMemberBatchTargets(targets), ErrEnterpriseMemberInvalid)
}

func TestEnterpriseMemberBatchUpdateRejectsLimitOutsideDatabaseNumericRange(t *testing.T) {
	memberService := NewEnterpriseMemberService(&enterpriseMemberBatchRepositorySpy{}, &archivedMemberOwnerRepo{}, nil, nil)
	limit := EnterpriseMemberMaxMonetaryValue + 1

	_, err := memberService.BatchUpdate(context.Background(), 7, BatchUpdateEnterpriseMembersInput{
		Members:         []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		MonthlyLimitUSD: &limit,
		GroupMode:       "keep",
	})

	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)
}
