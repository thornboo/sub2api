package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type feedbackRepository struct {
	db *sql.DB
}

func NewFeedbackRepository(db *sql.DB) service.FeedbackRepository {
	return &feedbackRepository{db: db}
}

const feedbackBaseSelectColumns = `
	f.id,
	f.content,
	f.source,
	f.status,
	f.user_id,
	COALESCE(u.email, '') AS user_email,
	f.api_key_id,
	COALESCE(k.name, '') AS key_name,
	CASE
		WHEN k.key IS NULL THEN ''
		WHEN char_length(k.key) <= 8 THEN '***'
		ELSE left(k.key, 8) || '...'
	END AS key_prefix,
	f.member_id,
	f.created_at,
	f.updated_at,
	f.closed_at,
	f.closed_by`

func feedbackSelectProjection(reader service.FeedbackReader, firstArg int) (string, string, []any) {
	incomingRole := service.FeedbackActorAdmin
	if reader.Kind == service.FeedbackActorAdmin {
		incomingRole = service.FeedbackActorUser
	}
	readerKindArg := firstArg
	readerIDArg := firstArg + 1
	incomingRoleArg := firstArg + 2
	columns := feedbackBaseSelectColumns + `,
	(
		CASE
			WHEN $` + itoa(readerKindArg) + ` = '` + service.FeedbackActorAdmin + `' AND rr.feedback_id IS NULL THEN 1
			ELSE 0
		END
		+
		COALESCE((
			SELECT COUNT(*)
			FROM feedback_replies fr_unread
			WHERE fr_unread.feedback_id = f.id
			  AND fr_unread.author_role = $` + itoa(incomingRoleArg) + `
			  AND fr_unread.id > COALESCE(rr.last_read_reply_id, 0)
		), 0)
	) AS unread_count`
	joinArgs := []any{reader.Kind, reader.ID, incomingRole}
	join := `
		LEFT JOIN feedback_read_receipts rr
			ON rr.feedback_id = f.id
			AND rr.reader_kind = $` + itoa(readerKindArg) + `
			AND rr.reader_id = $` + itoa(readerIDArg)
	return columns, join, joinArgs
}

func (r *feedbackRepository) Create(ctx context.Context, input service.FeedbackCreateInput) (*service.Feedback, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil feedback repository")
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO feedbacks (content, source, status, user_id, api_key_id, member_id, origin_member_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, content, source, status, user_id, '' AS user_email, api_key_id, '' AS key_name, '' AS key_prefix, member_id, created_at, updated_at, closed_at, closed_by, 0 AS unread_count`,
		input.Content,
		input.Source,
		service.FeedbackStatusOpen,
		input.UserID,
		nullInt64Ptr(input.APIKeyID),
		nullInt64Ptr(input.MemberID),
		nullInt64Ptr(input.MemberID),
	)
	return scanFeedback(row)
}

func (r *feedbackRepository) ListByScope(ctx context.Context, params pagination.PaginationParams, filters service.FeedbackListFilters, scope service.FeedbackScope, reader service.FeedbackReader) ([]service.Feedback, *pagination.PaginationResult, error) {
	return r.list(ctx, params, filters, scope, reader)
}

func (r *feedbackRepository) list(ctx context.Context, params pagination.PaginationParams, filters service.FeedbackListFilters, scope service.FeedbackScope, reader service.FeedbackReader) ([]service.Feedback, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, fmt.Errorf("nil feedback repository")
	}
	where, args := feedbackWhere(filters, scope, 0)

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM feedbacks f WHERE "+where, args...).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count feedbacks: %w", err)
	}

	selectColumns, readJoin, readArgs := feedbackSelectProjection(reader, len(args)+1)
	args = append(args, readArgs...)
	args = append(args, params.Limit(), params.Offset())
	query := `SELECT` + selectColumns + `
		FROM feedbacks f
		LEFT JOIN users u ON u.id = f.user_id
		LEFT JOIN api_keys k ON k.id = f.api_key_id
		` + readJoin + `
		WHERE ` + where + `
		ORDER BY f.updated_at DESC, f.id DESC
		LIMIT $` + itoa(len(args)-1) + ` OFFSET $` + itoa(len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list feedbacks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.Feedback, 0, params.Limit())
	for rows.Next() {
		item, err := scanFeedback(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate feedbacks: %w", err)
	}

	pages := int(math.Ceil(float64(total) / float64(params.Limit())))
	if pages < 1 {
		pages = 1
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit(), Pages: pages}, nil
}

func (r *feedbackRepository) Get(ctx context.Context, id int64, scope service.FeedbackScope, reader service.FeedbackReader) (*service.Feedback, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil feedback repository")
	}
	where, args := feedbackWhere(service.FeedbackListFilters{}, scope, id)
	selectColumns, readJoin, readArgs := feedbackSelectProjection(reader, len(args)+1)
	args = append(args, readArgs...)
	row := r.db.QueryRowContext(ctx, `SELECT`+selectColumns+`
		FROM feedbacks f
		LEFT JOIN users u ON u.id = f.user_id
		LEFT JOIN api_keys k ON k.id = f.api_key_id
		`+readJoin+`
		WHERE `+where,
		args...,
	)
	return scanFeedback(row)
}

func (r *feedbackRepository) ListReplies(ctx context.Context, feedbackID int64, params pagination.PaginationParams, scope service.FeedbackScope) ([]service.FeedbackReply, *pagination.PaginationResult, error) {
	if r == nil || r.db == nil {
		return nil, nil, fmt.Errorf("nil feedback repository")
	}
	where, args := feedbackWhere(service.FeedbackListFilters{}, scope, feedbackID)

	var exists bool
	if err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM feedbacks f WHERE "+where+")", args...).Scan(&exists); err != nil {
		return nil, nil, fmt.Errorf("check feedback replies scope: %w", err)
	}
	if !exists {
		return nil, nil, infraerrors.NotFound("FEEDBACK_NOT_FOUND", "feedback not found")
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM feedback_replies WHERE feedback_id = $1", feedbackID).Scan(&total); err != nil {
		return nil, nil, fmt.Errorf("count feedback replies: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, feedback_id, author_role, content, created_at
		FROM feedback_replies
		WHERE feedback_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`,
		feedbackID,
		params.Limit(),
		params.Offset(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("list feedback replies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.FeedbackReply, 0, params.Limit())
	for rows.Next() {
		item, err := scanFeedbackReply(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate feedback replies: %w", err)
	}

	pages := int(math.Ceil(float64(total) / float64(params.Limit())))
	if pages < 1 {
		pages = 1
	}
	return items, &pagination.PaginationResult{Total: total, Page: params.Page, PageSize: params.Limit(), Pages: pages}, nil
}

func (r *feedbackRepository) CreateReply(ctx context.Context, input service.FeedbackReplyInput) (*service.FeedbackReply, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil feedback repository")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin feedback reply tx: %w", err)
	}
	defer rollbackUnlessCommitted(tx)

	feedback, err := lockFeedback(ctx, tx, input.FeedbackID, input.Scope)
	if err != nil {
		return nil, err
	}
	if feedback.Status == service.FeedbackStatusClosed {
		return nil, service.ErrFeedbackClosed
	}

	row := tx.QueryRowContext(ctx, `
		INSERT INTO feedback_replies (feedback_id, author_role, content)
		VALUES ($1, $2, $3)
		RETURNING id, feedback_id, author_role, content, created_at`,
		input.FeedbackID,
		input.AuthorRole,
		input.Content,
	)
	reply, err := scanFeedbackReply(row)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE feedbacks SET updated_at = NOW() WHERE id = $1", input.FeedbackID); err != nil {
		return nil, fmt.Errorf("touch feedback after reply: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit feedback reply tx: %w", err)
	}
	return reply, nil
}

func (r *feedbackRepository) Close(ctx context.Context, id int64, closedBy string, scope service.FeedbackScope, reader service.FeedbackReader) (*service.Feedback, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil feedback repository")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin feedback close tx: %w", err)
	}
	defer rollbackUnlessCommitted(tx)

	feedback, err := lockFeedback(ctx, tx, id, scope)
	if err != nil {
		return nil, err
	}
	if feedback.Status != service.FeedbackStatusClosed {
		selectColumns, readJoin, readArgs := feedbackSelectProjection(reader, 4)
		row := tx.QueryRowContext(ctx, `WITH updated AS (
				UPDATE feedbacks
				SET status = $2, closed_at = NOW(), closed_by = $3, updated_at = NOW()
				WHERE id = $1
				RETURNING *
			)
			SELECT`+selectColumns+`
			FROM updated f
			LEFT JOIN users u ON u.id = f.user_id
			LEFT JOIN api_keys k ON k.id = f.api_key_id
			`+readJoin,
			id,
			service.FeedbackStatusClosed,
			closedBy,
			readArgs[0],
			readArgs[1],
			readArgs[2],
		)
		feedback, err = scanFeedback(row)
		if err != nil {
			return nil, err
		}
	} else {
		where, args := feedbackWhere(service.FeedbackListFilters{}, scope, id)
		selectColumns, readJoin, readArgs := feedbackSelectProjection(reader, len(args)+1)
		args = append(args, readArgs...)
		row := tx.QueryRowContext(ctx, `SELECT`+selectColumns+`
			FROM feedbacks f
			LEFT JOIN users u ON u.id = f.user_id
			LEFT JOIN api_keys k ON k.id = f.api_key_id
			`+readJoin+`
			WHERE `+where,
			args...,
		)
		feedback, err = scanFeedback(row)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit feedback close tx: %w", err)
	}
	return feedback, nil
}

func (r *feedbackRepository) MarkRead(ctx context.Context, feedbackID int64, scope service.FeedbackScope, reader service.FeedbackReader, lastReadReplyID int64) (*service.FeedbackReadState, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil feedback repository")
	}
	if _, err := r.Get(ctx, feedbackID, scope, reader); err != nil {
		return nil, err
	}
	if lastReadReplyID > 0 {
		var belongs bool
		if err := r.db.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1
				FROM feedback_replies
				WHERE feedback_id = $1 AND id = $2
			)`,
			feedbackID,
			lastReadReplyID,
		).Scan(&belongs); err != nil {
			return nil, fmt.Errorf("check feedback read cursor: %w", err)
		}
		if !belongs {
			return nil, service.ErrFeedbackInvalidReadID
		}
	}
	var state service.FeedbackReadState
	err := r.db.QueryRowContext(ctx, `
		WITH upserted AS (
			INSERT INTO feedback_read_receipts (feedback_id, reader_kind, reader_id, last_read_reply_id, updated_at)
			VALUES ($1, $2, $3, $4, NOW())
			ON CONFLICT (feedback_id, reader_kind, reader_id)
			DO UPDATE SET
				last_read_reply_id = GREATEST(feedback_read_receipts.last_read_reply_id, EXCLUDED.last_read_reply_id),
				updated_at = NOW()
			RETURNING last_read_reply_id
		)
		SELECT
			(
				COALESCE((
					SELECT COUNT(*)
					FROM feedback_replies fr
					WHERE fr.feedback_id = $1
					  AND fr.author_role = $5
					  AND fr.id > upserted.last_read_reply_id
				), 0)
			) AS unread_count,
			upserted.last_read_reply_id
		FROM upserted`,
		feedbackID,
		reader.Kind,
		reader.ID,
		lastReadReplyID,
		feedbackIncomingRole(reader),
	).Scan(&state.UnreadCount, &state.LastReadReplyID)
	if err != nil {
		return nil, fmt.Errorf("mark feedback read: %w", err)
	}
	return &state, nil
}

func feedbackWhere(filters service.FeedbackListFilters, scope service.FeedbackScope, id int64) (string, []any) {
	where := "1=1"
	args := make([]any, 0, 5)
	if id > 0 {
		args = append(args, id)
		where += " AND f.id = $" + itoa(len(args))
	}
	if filters.Status != "" {
		args = append(args, filters.Status)
		where += " AND f.status = $" + itoa(len(args))
	}
	switch scope.Kind {
	case service.FeedbackActorUser:
		args = append(args, scope.UserID)
		where += " AND f.source = 'user' AND f.user_id = $" + itoa(len(args))
	case service.FeedbackSourceKey:
		args = append(args, scope.UserID)
		where += " AND f.source = 'key' AND f.user_id = $" + itoa(len(args))
		args = append(args, scope.APIKeyID)
		where += " AND f.api_key_id = $" + itoa(len(args))
		if scope.MemberID == nil {
			where += " AND f.origin_member_id IS NULL"
		} else {
			args = append(args, *scope.MemberID)
			where += " AND f.origin_member_id = $" + itoa(len(args))
		}
	}
	return where, args
}

func lockFeedback(ctx context.Context, tx *sql.Tx, id int64, scope service.FeedbackScope) (*service.Feedback, error) {
	where, args := feedbackWhere(service.FeedbackListFilters{}, scope, id)
	row := tx.QueryRowContext(ctx, `SELECT`+feedbackBaseSelectColumns+`, 0 AS unread_count
		FROM feedbacks f
		LEFT JOIN users u ON u.id = f.user_id
		LEFT JOIN api_keys k ON k.id = f.api_key_id
		WHERE `+where+`
		FOR UPDATE OF f`,
		args...,
	)
	return scanFeedback(row)
}

func feedbackIncomingRole(reader service.FeedbackReader) string {
	if reader.Kind == service.FeedbackActorAdmin {
		return service.FeedbackActorUser
	}
	return service.FeedbackActorAdmin
}

func rollbackUnlessCommitted(tx *sql.Tx) {
	_ = tx.Rollback()
}

type feedbackScanner interface {
	Scan(dest ...any) error
}

func scanFeedback(scanner feedbackScanner) (*service.Feedback, error) {
	var item service.Feedback
	var apiKeyID sql.NullInt64
	var memberID sql.NullInt64
	var closedAt sql.NullTime
	var closedBy sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.Content,
		&item.Source,
		&item.Status,
		&item.UserID,
		&item.UserEmail,
		&apiKeyID,
		&item.KeyName,
		&item.KeyPrefix,
		&memberID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&closedAt,
		&closedBy,
		&item.UnreadCount,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.NotFound("FEEDBACK_NOT_FOUND", "feedback not found")
		}
		return nil, fmt.Errorf("scan feedback: %w", err)
	}
	if apiKeyID.Valid {
		item.APIKeyID = &apiKeyID.Int64
	}
	if memberID.Valid {
		item.MemberID = &memberID.Int64
	}
	if closedAt.Valid {
		item.ClosedAt = &closedAt.Time
	}
	if closedBy.Valid {
		item.ClosedBy = &closedBy.String
	}
	return &item, nil
}

func scanFeedbackReply(scanner feedbackScanner) (*service.FeedbackReply, error) {
	var item service.FeedbackReply
	if err := scanner.Scan(
		&item.ID,
		&item.FeedbackID,
		&item.AuthorRole,
		&item.Content,
		&item.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infraerrors.NotFound("FEEDBACK_REPLY_NOT_FOUND", "feedback reply not found")
		}
		return nil, fmt.Errorf("scan feedback reply: %w", err)
	}
	return &item, nil
}
