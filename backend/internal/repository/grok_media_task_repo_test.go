package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGrokMediaTaskRepositoryPersistsAndScopesTaskIdentity(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(8)
	task := &service.GrokMediaTask{
		UpstreamRequestID: "video-123",
		UserID:            3,
		APIKeyID:          17,
		MemberID:          &memberID,
		GroupID:           12,
		AccountID:         63,
	}
	mock.ExpectQuery(`INSERT INTO grok_media_tasks`).
		WithArgs("video-123", int64(3), int64(17), memberID, int64(12), int64(63)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(99)))
	mock.ExpectQuery(`SELECT id, upstream_request_id, user_id, api_key_id, member_id, group_id, account_id`).
		WithArgs("video-123", int64(3), memberID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "upstream_request_id", "user_id", "api_key_id", "member_id", "group_id", "account_id"}).
			AddRow(int64(99), "video-123", int64(3), int64(17), memberID, int64(12), int64(63)))

	repo := &grokMediaTaskRepository{db: db}
	require.NoError(t, repo.Create(context.Background(), task))
	require.Equal(t, int64(99), task.ID)

	loaded, err := repo.GetByRequestID(context.Background(), 3, &memberID, "video-123")
	require.NoError(t, err)
	require.Equal(t, task, loaded)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGrokMediaTaskRepositoryHidesCrossIdentityTask(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	memberID := int64(9)
	mock.ExpectQuery(`SELECT id, upstream_request_id, user_id, api_key_id, member_id, group_id, account_id`).
		WithArgs("video-123", int64(3), memberID).
		WillReturnError(sql.ErrNoRows)

	repo := &grokMediaTaskRepository{db: db}
	_, err = repo.GetByRequestID(context.Background(), 3, &memberID, "video-123")
	require.ErrorIs(t, err, service.ErrGrokMediaTaskNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
