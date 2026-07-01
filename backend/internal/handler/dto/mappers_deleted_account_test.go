package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountSummaryFromService_MapsDeletedAt(t *testing.T) {
	deletedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	dto := AccountSummaryFromService(&service.Account{
		ID:        12,
		Name:      "archived upstream account",
		DeletedAt: &deletedAt,
	})

	require.NotNil(t, dto)
	require.Equal(t, int64(12), dto.ID)
	require.Equal(t, "archived upstream account", dto.Name)
	require.True(t, dto.Deleted)
	require.Equal(t, &deletedAt, dto.DeletedAt)
}

func TestAccountFromServiceShallow_MapsDeletedAt(t *testing.T) {
	deletedAt := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)

	dto := AccountFromServiceShallow(&service.Account{
		ID:        12,
		Name:      "archived upstream account",
		DeletedAt: &deletedAt,
	})

	require.NotNil(t, dto)
	require.True(t, dto.Deleted)
	require.Equal(t, &deletedAt, dto.DeletedAt)
}
