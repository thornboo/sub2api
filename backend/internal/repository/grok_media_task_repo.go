package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type grokMediaTaskRepository struct{ db *sql.DB }

func NewGrokMediaTaskRepository(db *sql.DB) service.GrokMediaTaskRepository {
	return &grokMediaTaskRepository{db: db}
}

func (r *grokMediaTaskRepository) Create(ctx context.Context, task *service.GrokMediaTask) error {
	if r == nil || r.db == nil {
		return errors.New("grok media task repository db is nil")
	}
	if task == nil || strings.TrimSpace(task.UpstreamRequestID) == "" || task.UserID <= 0 || task.APIKeyID <= 0 || task.GroupID <= 0 || task.AccountID <= 0 {
		return errors.New("grok media task is invalid")
	}
	return r.db.QueryRowContext(ctx, `
		INSERT INTO grok_media_tasks
			(upstream_request_id, user_id, api_key_id, member_id, group_id, account_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`, strings.TrimSpace(task.UpstreamRequestID), task.UserID, task.APIKeyID, task.MemberID, task.GroupID, task.AccountID).
		Scan(&task.ID)
}

func (r *grokMediaTaskRepository) GetByRequestID(ctx context.Context, userID int64, memberID *int64, upstreamRequestID string) (*service.GrokMediaTask, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("grok media task repository db is nil")
	}
	task := &service.GrokMediaTask{}
	err := r.db.QueryRowContext(ctx, `
		SELECT id, upstream_request_id, user_id, api_key_id, member_id, group_id, account_id
		FROM grok_media_tasks
		WHERE upstream_request_id = $1
		  AND user_id = $2
		  AND (($3::bigint IS NULL AND member_id IS NULL) OR member_id = $3)`, strings.TrimSpace(upstreamRequestID), userID, memberID).
		Scan(&task.ID, &task.UpstreamRequestID, &task.UserID, &task.APIKeyID, &task.MemberID, &task.GroupID, &task.AccountID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrGrokMediaTaskNotFound
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}
