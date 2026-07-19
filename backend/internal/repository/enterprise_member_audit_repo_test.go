package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAuditRepositoryRecordsCredentialSafeKeyReveal(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`(?s)INSERT INTO enterprise_member_audit_logs.*member_key.reveal_authorized.*enterprise_member_key_reveal`).
		WithArgs(int64(7), int64(41), int64(7), int64(28)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := &enterpriseMemberAuditRepository{db: db}
	require.NoError(t, repo.RecordKeyReveal(context.Background(), 7, 41, 7, 28))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberAuditRepositoryScopesByOwnerAndMember(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ownerID := int64(41)
	memberID := int64(73)
	createdAt := time.Date(2026, time.July, 12, 10, 30, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT COUNT\(\*\).*FROM enterprise_member_audit_logs.*WHERE enterprise_user_id = \$1 AND member_id = \$2`).
		WithArgs(ownerID, memberID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT id, enterprise_user_id, member_id, actor_user_id, action, entity_type,.*WHERE enterprise_user_id = \$1 AND member_id = \$2.*ORDER BY created_at DESC, id DESC.*LIMIT \$3 OFFSET \$4`).
		WithArgs(ownerID, memberID, 25, 25).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "enterprise_user_id", "member_id", "actor_user_id", "action", "entity_type",
			"entity_id", "before_data", "after_data", "metadata", "created_at",
		}).AddRow(
			int64(9), ownerID, memberID, ownerID, "member.updated", "member",
			memberID, []byte(`{"name":"before"}`), []byte(`{"name":"after"}`), []byte(`{}`), createdAt,
		))

	repo := &enterpriseMemberAuditRepository{db: db}
	items, total, err := repo.ListByMember(context.Background(), ownerID, memberID, 2, 25)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, ownerID, items[0].EnterpriseUserID)
	require.Equal(t, memberID, *items[0].MemberID)
	require.Equal(t, "member.updated", items[0].Action)
	require.JSONEq(t, `{"name":"after"}`, string(items[0].AfterData))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberAuditRepositoryBoundsPagination(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`SELECT COUNT\(\*\)`).WithArgs(int64(2), int64(3)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`SELECT id, enterprise_user_id`).WithArgs(int64(2), int64(3), 200, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "enterprise_user_id", "member_id", "actor_user_id", "action", "entity_type",
			"entity_id", "before_data", "after_data", "metadata", "created_at",
		}))

	repo := &enterpriseMemberAuditRepository{db: db}
	items, total, err := repo.ListByMember(context.Background(), 2, 3, 0, 1000)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberAuditRepositoryListsOwnerWideEventsWithoutCrossTenantRows(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ownerID := int64(11)
	createdAt := time.Date(2026, time.July, 12, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`SELECT COUNT\(\*\).*WHERE enterprise_user_id = \$1`).
		WithArgs(ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT id, enterprise_user_id, member_id, actor_user_id, action, entity_type,.*WHERE enterprise_user_id = \$1.*LIMIT \$2 OFFSET \$3`).
		WithArgs(ownerID, 100, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "enterprise_user_id", "member_id", "actor_user_id", "action", "entity_type",
			"entity_id", "before_data", "after_data", "metadata", "created_at",
		}).AddRow(
			int64(10), ownerID, nil, ownerID, "import.completed", "import_job",
			int64(51), []byte(`{"status":"processing"}`), []byte(`{"status":"completed"}`), []byte(`{}`), createdAt,
		))

	repo := &enterpriseMemberAuditRepository{db: db}
	items, total, err := repo.ListByOwner(context.Background(), ownerID, 1, 100)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Nil(t, items[0].MemberID)
	require.Equal(t, "import.completed", items[0].Action)
	require.NoError(t, mock.ExpectationsWereMet())
}
