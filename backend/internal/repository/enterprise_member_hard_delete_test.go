package repository

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberHardDeleteRequiresArchivedMember(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM enterprise_members\s+WHERE id = \$1 AND enterprise_user_id = \$2\s+AND deleted_at IS NOT NULL AND removed_at IS NULL\s+FOR UPDATE`).
		WithArgs(int64(11), int64(7)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	repo := &enterpriseMemberRepository{db: db}
	_, err = repo.DeletePermanently(t.Context(), 7, 11)
	require.ErrorIs(t, err, service.ErrEnterpriseMemberNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberDeleteTombstonesArchivedMemberWithHistoricalFacts(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM enterprise_members\s+WHERE id = \$1 AND enterprise_user_id = \$2\s+AND deleted_at IS NOT NULL AND removed_at IS NULL\s+FOR UPDATE`).
		WithArgs(int64(11), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(`(?s)FROM unnest\(\$1::bigint\[\]\).*enterprise_member_rate_limit_periods.*ops_error_logs.*batch_image_jobs.*grok_media_tasks`).
		WithArgs(pq.Array([]int64{11})).
		WillReturnRows(sqlmock.NewRows([]string{"member_id"}).AddRow(11))
	mock.ExpectExec(`DELETE FROM enterprise_member_group_bindings WHERE member_id = \$1`).
		WithArgs(int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`(?s)UPDATE api_keys.*disabled_reason = \$3.*WHERE member_id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(11), service.StatusAPIKeyDisabled, service.APIKeyDisabledReasonMemberRemoved).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`(?s)UPDATE enterprise_members.*member_code = '~deleted~'.*removed_at = NOW\(\).*WHERE id = \$1 AND enterprise_user_id = \$2 AND removed_at IS NULL`).
		WithArgs(int64(11), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberRepository{db: db}
	result, err := repo.DeletePermanently(t.Context(), 7, 11)
	require.NoError(t, err)
	require.Equal(t, service.EnterpriseMemberDeletionModeTombstone, result.Mode)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseMemberHardDeleteRemovesCleanArchivedMember(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`FROM enterprise_members\s+WHERE id = \$1 AND enterprise_user_id = \$2\s+AND deleted_at IS NOT NULL AND removed_at IS NULL\s+FOR UPDATE`).
		WithArgs(int64(11), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery(`FROM unnest\(\$1::bigint\[\]\)`).
		WithArgs(pq.Array([]int64{11})).
		WillReturnRows(sqlmock.NewRows([]string{"member_id"}))
	mock.ExpectExec(`DELETE FROM enterprise_member_group_bindings WHERE member_id = \$1`).
		WithArgs(int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM enterprise_members WHERE id = \$1 AND enterprise_user_id = \$2`).
		WithArgs(int64(11), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := &enterpriseMemberRepository{db: db}
	result, err := repo.DeletePermanently(t.Context(), 7, 11)
	require.NoError(t, err)
	require.Equal(t, service.EnterpriseMemberDeletionModeHardDelete, result.Mode)
	require.NoError(t, mock.ExpectationsWereMet())
}
