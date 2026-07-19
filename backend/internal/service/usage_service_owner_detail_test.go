package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type usageOwnerDetailRepoStub struct {
	UsageLogRepository
	log       *UsageLog
	err       error
	gotID     int64
	gotUserID int64
}

func (s *usageOwnerDetailRepoStub) GetByIDForOwner(_ context.Context, id, userID int64) (*UsageLog, error) {
	s.gotID = id
	s.gotUserID = userID
	return s.log, s.err
}

func TestUsageServiceGetByIDForOwnerUsesOwnerVisibleRepositoryQuery(t *testing.T) {
	want := &UsageLog{ID: 77, UserID: 42}
	repo := &usageOwnerDetailRepoStub{log: want}
	svc := &UsageService{usageRepo: repo}

	got, err := svc.GetByIDForOwner(context.Background(), 77, 42)
	require.NoError(t, err)
	require.Same(t, want, got)
	require.Equal(t, int64(77), repo.gotID)
	require.Equal(t, int64(42), repo.gotUserID)
}
