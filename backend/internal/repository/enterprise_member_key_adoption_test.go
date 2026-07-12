package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberRepositoryAdoptKeyPreservesOriginalGroupAtomically(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	const ownerID, memberID, keyID, groupID, version int64 = 11, 22, 33, 44, 5
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, deleted_at, version\s+FROM enterprise_members`).
		WithArgs(memberID, ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"status", "deleted_at", "version"}).AddRow("active", nil, version))
	mock.ExpectQuery(`SELECT group_id, member_id, status, deleted_at\s+FROM api_keys`).
		WithArgs(keyID, ownerID).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "member_id", "status", "deleted_at"}).AddRow(groupID, nil, "active", nil))
	mock.ExpectQuery(`SELECT EXISTS \(`).
		WithArgs(ownerID, groupID).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`INSERT INTO enterprise_member_group_bindings`).
		WithArgs(memberID, groupID).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(groupID))
	mock.ExpectExec(`UPDATE api_keys\s+SET member_id = \$1, group_id = NULL`).
		WithArgs(memberID, keyID, ownerID, groupID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE enterprise_members\s+SET version = version \+ 1`).
		WithArgs(memberID, ownerID, version).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT group_id\s+FROM enterprise_member_group_bindings`).
		WithArgs(memberID).
		WillReturnRows(sqlmock.NewRows([]string{"group_id"}).AddRow(int64(9)).AddRow(groupID))
	mock.ExpectCommit()

	repo := &enterpriseMemberRepository{db: db}
	result, err := repo.AdoptKey(context.Background(), ownerID, memberID, keyID, version)
	require.NoError(t, err)
	require.Equal(t, &service.EnterpriseMemberKeyAdoptionResult{
		KeyID: keyID, OriginalGroupID: groupID, GroupAdded: true,
		GroupIDs: []int64{9, groupID}, MemberVersion: version + 1,
	}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberRepositoryAdoptKeyRejectsStaleVersionBeforeKeyMutation(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, deleted_at, version\s+FROM enterprise_members`).
		WithArgs(int64(22), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "deleted_at", "version"}).AddRow("active", nil, int64(8)))
	mock.ExpectRollback()

	repo := &enterpriseMemberRepository{db: db}
	_, err = repo.AdoptKey(context.Background(), 11, 22, 33, 7)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberVersion)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberRepositoryAdoptKeyRejectsLostGroupAuthorization(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT status, deleted_at, version\s+FROM enterprise_members`).
		WithArgs(int64(22), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "deleted_at", "version"}).AddRow("active", nil, int64(7)))
	mock.ExpectQuery(`SELECT group_id, member_id, status, deleted_at\s+FROM api_keys`).
		WithArgs(int64(33), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "member_id", "status", "deleted_at"}).AddRow(int64(44), nil, "active", nil))
	mock.ExpectQuery(`SELECT EXISTS \(`).
		WithArgs(int64(11), int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectRollback()

	repo := &enterpriseMemberRepository{db: db}
	_, err = repo.AdoptKey(context.Background(), 11, 22, 33, 7)
	require.ErrorIs(t, err, service.ErrGroupNotAllowed)
	require.NoError(t, mock.ExpectationsWereMet())
}
