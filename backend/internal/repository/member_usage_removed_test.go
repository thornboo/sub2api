package repository

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOwnerUsageMemberDirectoryExcludesRemovedTombstones(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`WHERE em\.enterprise_user_id = \$1\s+AND em\.removed_at IS NULL`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "member_code", "name", "status", "archived", "key_count", "monthly_limit_usd", "deleted_at",
		}).AddRow(12, "member-12", "Member 12", "active", false, 1, 100, nil))

	repo := &usageLogRepository{sql: db}
	members, err := repo.ListOwnerUsageMembers(t.Context(), 7)
	require.NoError(t, err)
	require.Len(t, members, 1)
	require.Equal(t, int64(12), members[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestValidateOwnerUsageMemberRejectsRemovedTombstone(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)WHERE id = \$1\s+AND enterprise_user_id = \$2\s+AND removed_at IS NULL`).
		WithArgs(int64(12), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	repo := &usageLogRepository{sql: db}
	err = repo.ValidateOwnerUsageMember(t.Context(), 7, 12)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
