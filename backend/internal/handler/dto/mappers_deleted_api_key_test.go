package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyFromService_MapsDeletedAtAndHidesDeletedKeyMaterial(t *testing.T) {
	deletedAt := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)

	dto := APIKeyFromService(&service.APIKey{
		ID:        9,
		UserID:    4,
		Key:       "__deleted__9__123",
		Name:      "deleted evidence key",
		Status:    service.StatusDisabled,
		DeletedAt: &deletedAt,
	})

	require.NotNil(t, dto)
	require.Equal(t, int64(9), dto.ID)
	require.Equal(t, "deleted evidence key", dto.Name)
	require.NotNil(t, dto.DeletedAt)
	require.Empty(t, dto.Key, "deleted key material/tombstones must not be exposed in usage DTOs")
}
