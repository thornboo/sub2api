package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOwnerVisibleEnterpriseMemberFactOrUnassignedCondition(t *testing.T) {
	condition := ownerVisibleEnterpriseMemberFactOrUnassignedCondition("usage_record.member_id", "usage_record.user_id")

	for _, want := range []string{
		"usage_record.member_id IS NULL",
		"visible_member.id = usage_record.member_id",
		"visible_member.enterprise_user_id = usage_record.user_id",
		"visible_member.removed_at IS NULL",
	} {
		require.Contains(t, condition, want)
	}
	require.NotContains(t, condition, "visible_member.deleted_at", "archived members must remain owner-visible")
}

func TestUsageLogRepositoryGetByIDForOwnerRequiresVisibleOrUnassignedMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectQuery(`(?s)FROM usage_logs usage_record\s+WHERE usage_record\.id = \$1\s+AND usage_record\.user_id = \$2\s+AND \(usage_record\.member_id IS NULL OR \(usage_record\.member_id IS NOT NULL AND EXISTS \(SELECT 1 FROM enterprise_members visible_member WHERE visible_member\.id = usage_record\.member_id AND visible_member\.enterprise_user_id = usage_record\.user_id AND visible_member\.removed_at IS NULL\)\)\)`).
		WithArgs(int64(77), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.GetByIDForOwner(context.Background(), 77, 42)
	require.Nil(t, got)
	require.ErrorIs(t, err, service.ErrUsageLogNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryGetByIDRetainsUnfilteredAuditLookup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectQuery(`FROM usage_logs WHERE id = \$1`).
		WithArgs(int64(77)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.GetByID(context.Background(), 77)
	require.Nil(t, got)
	require.ErrorIs(t, err, service.ErrUsageLogNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepositoryGetErrorLogByIDForOwnerRequiresVisibleOrUnassignedMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &opsRepository{db: db}
	mock.ExpectQuery(`(?s)WHERE e\.id = \$1\s+AND \(e\.user_id = \$2 OR e\.deleted_key_owner_user_id = \$2\)\s+AND \(e\.member_id IS NULL OR \(e\.member_id IS NOT NULL AND EXISTS \(SELECT 1 FROM enterprise_members visible_member WHERE visible_member\.id = e\.member_id AND visible_member\.enterprise_user_id = \$2 AND visible_member\.removed_at IS NULL\)\)\)\s+LIMIT 1`).
		WithArgs(int64(88), int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.GetErrorLogByIDForOwner(context.Background(), 88, 42)
	require.Nil(t, got)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpsRepositoryGetErrorLogByIDRetainsUnfilteredAuditLookup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := &opsRepository{db: db}
	mock.ExpectQuery(`(?s)WHERE e\.id = \$1\s+LIMIT 1`).
		WithArgs(int64(88)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.GetErrorLogByID(context.Background(), 88)
	require.Nil(t, got)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.NoError(t, mock.ExpectationsWereMet())
}
