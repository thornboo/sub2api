package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestFeedbackRepositoryListReturnsAdminTicketWithoutFullKey(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)
	apiKeyID := int64(101)
	memberID := int64(9)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM feedbacks f WHERE 1=1 AND f.status = \$1`).
		WithArgs(service.FeedbackStatusOpen).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`CASE\s+WHEN k.key IS NULL THEN ''\s+WHEN char_length\(k.key\) <= 8 THEN '\*\*\*'\s+ELSE left\(k.key, 8\) \|\| '\.\.\.'\s+END AS key_prefix.*feedback_read_receipts rr.*WHERE 1=1 AND f.status = \$1.*ORDER BY f.updated_at DESC, f.id DESC.*LIMIT \$5 OFFSET \$6`).
		WithArgs(service.FeedbackStatusOpen, service.FeedbackActorAdmin, int64(1), service.FeedbackActorUser, 20, 0).
		WillReturnRows(feedbackRows().
			AddRow(int64(1), "title", "content", service.FeedbackSourceKey, service.FeedbackStatusOpen, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", apiKeyID, "test key", "sk-12345...", memberID, now, now, nil, nil, int64(3)))

	repo := NewFeedbackRepository(db)
	items, result, err := repo.ListByScope(
		context.Background(),
		pagination.PaginationParams{Page: 1, PageSize: 20},
		service.FeedbackListFilters{Status: service.FeedbackStatusOpen},
		service.FeedbackScope{Kind: service.FeedbackActorAdmin},
		service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 1},
	)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if result.Total != 1 || len(items) != 1 {
		t.Fatalf("result=%+v items=%+v", result, items)
	}
	if items[0].APIKeyID == nil || *items[0].APIKeyID != apiKeyID || items[0].MemberID == nil || *items[0].MemberID != memberID {
		t.Fatalf("nullable ids not mapped: %+v", items[0])
	}
	if items[0].KeyPrefix != "sk-12345..." {
		t.Fatalf("key prefix = %q, want masked prefix", items[0].KeyPrefix)
	}
	if items[0].UnreadCount != 3 {
		t.Fatalf("unread count = %d, want 3", items[0].UnreadCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryKeyScopeUsesOriginMemberSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	memberID := int64(9)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM feedbacks f WHERE 1=1 AND f.source = 'key' AND f.user_id = \$1 AND f.api_key_id = \$2 AND f.origin_member_id = \$3`).
		WithArgs(int64(7), int64(101), memberID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`WHERE 1=1 AND f.source = 'key' AND f.user_id = \$1 AND f.api_key_id = \$2 AND f.origin_member_id = \$3.*ORDER BY f.updated_at DESC`).
		WithArgs(int64(7), int64(101), memberID, service.FeedbackSourceKey, int64(101), service.FeedbackActorAdmin, 20, 0).
		WillReturnRows(feedbackRows())

	repo := NewFeedbackRepository(db)
	_, _, err = repo.ListByScope(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, service.FeedbackListFilters{}, service.FeedbackScope{
		Kind:     service.FeedbackSourceKey,
		UserID:   7,
		APIKeyID: 101,
		MemberID: &memberID,
	}, service.FeedbackReader{Kind: service.FeedbackSourceKey, ID: 101})
	if err != nil {
		t.Fatalf("ListByScope: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryReplyStatusFilterAppliesBeforeCountAndPagination(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)

	replyStatusPredicate := `CASE WHEN \(\s+SELECT fr_latest\.author_role\s+FROM feedback_replies fr_latest\s+WHERE fr_latest\.feedback_id = f\.id\s+ORDER BY fr_latest\.created_at DESC, fr_latest\.id DESC\s+LIMIT 1\s+\) = 'admin' THEN 'replied'\s+ELSE 'pending' END\) = \$1`
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM feedbacks f WHERE 1=1 AND \(` + replyStatusPredicate).
		WithArgs(service.FeedbackReplyStatusReplied).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`SELECT.*AS reply_status.*feedback_read_receipts rr.*WHERE 1=1 AND \(`+replyStatusPredicate+`.*ORDER BY f.updated_at DESC, f.id DESC.*LIMIT \$5 OFFSET \$6`).
		WithArgs(service.FeedbackReplyStatusReplied, service.FeedbackActorAdmin, int64(900), service.FeedbackActorUser, 1, 1).
		WillReturnRows(feedbackRows().
			AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusOpen, service.FeedbackReplyStatusReplied, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, now, nil, nil, int64(0)))

	repo := NewFeedbackRepository(db)
	items, result, err := repo.ListByScope(
		context.Background(),
		pagination.PaginationParams{Page: 2, PageSize: 1},
		service.FeedbackListFilters{ReplyStatus: service.FeedbackReplyStatusReplied},
		service.FeedbackScope{Kind: service.FeedbackActorAdmin},
		service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 900},
	)
	if err != nil {
		t.Fatalf("ListByScope: %v", err)
	}
	if result.Total != 1 || result.PageSize != 1 || len(items) != 1 || items[0].ReplyStatus != service.FeedbackReplyStatusReplied {
		t.Fatalf("unexpected result=%+v items=%+v", result, items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryReplyStatusFilterCombinesWithKeyScope(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	memberID := int64(9)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM feedbacks f WHERE 1=1 AND \(CASE WHEN .* THEN 'replied'\s+ELSE 'pending' END\) = \$1 AND f.source = 'key' AND f.user_id = \$2 AND f.api_key_id = \$3 AND f.origin_member_id = \$4`).
		WithArgs(service.FeedbackReplyStatusPending, int64(7), int64(101), memberID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`WHERE 1=1 AND \(CASE WHEN .* THEN 'replied'\s+ELSE 'pending' END\) = \$1 AND f.source = 'key' AND f.user_id = \$2 AND f.api_key_id = \$3 AND f.origin_member_id = \$4.*ORDER BY f.updated_at DESC`).
		WithArgs(service.FeedbackReplyStatusPending, int64(7), int64(101), memberID, service.FeedbackSourceKey, int64(101), service.FeedbackActorAdmin, 20, 0).
		WillReturnRows(feedbackRows())

	repo := NewFeedbackRepository(db)
	_, _, err = repo.ListByScope(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, service.FeedbackListFilters{ReplyStatus: service.FeedbackReplyStatusPending}, service.FeedbackScope{
		Kind:     service.FeedbackSourceKey,
		UserID:   7,
		APIKeyID: 101,
		MemberID: &memberID,
	}, service.FeedbackReader{Kind: service.FeedbackSourceKey, ID: 101})
	if err != nil {
		t.Fatalf("ListByScope: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryCloseLocksFeedbackAndIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)
	closedAt := time.Unix(1776790080, 0)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT.*FROM feedbacks f.*WHERE 1=1 AND f.id = \$1\s+FOR UPDATE OF f`).
		WithArgs(int64(42)).
		WillReturnRows(feedbackRows().AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusOpen, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, now, nil, nil, int64(0)))
	mock.ExpectQuery(`WITH updated AS \(\s+UPDATE feedbacks\s+SET status = \$2, closed_at = NOW\(\), closed_by = \$3, updated_at = NOW\(\)\s+WHERE id = \$1\s+RETURNING \*\s+\)\s+SELECT.*FROM updated f.*feedback_read_receipts rr`).
		WithArgs(int64(42), service.FeedbackStatusClosed, service.FeedbackActorAdmin, service.FeedbackActorAdmin, int64(900), service.FeedbackActorUser).
		WillReturnRows(feedbackRows().AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusClosed, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, closedAt, closedAt, service.FeedbackActorAdmin, int64(1)))
	mock.ExpectCommit()

	repo := NewFeedbackRepository(db)
	item, err := repo.Close(context.Background(), 42, service.FeedbackActorAdmin, service.FeedbackScope{Kind: service.FeedbackActorAdmin}, service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 900})
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if item.Status != service.FeedbackStatusClosed || item.ClosedAt == nil || item.ClosedBy == nil || *item.ClosedBy != service.FeedbackActorAdmin {
		t.Fatalf("closed fields not mapped: %+v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryCreateReplyRejectsClosedWithinLock(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)
	closedBy := service.FeedbackActorUser

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT.*FROM feedbacks f.*WHERE 1=1 AND f.id = \$1 AND f.source = 'user' AND f.user_id = \$2\s+FOR UPDATE OF f`).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(feedbackRows().AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusClosed, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, now, now, closedBy, int64(0)))
	mock.ExpectRollback()

	repo := NewFeedbackRepository(db)
	_, err = repo.CreateReply(context.Background(), service.FeedbackReplyInput{
		FeedbackID: 42,
		Scope:      service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: 7},
		AuthorRole: service.FeedbackActorUser,
		Content:    "reply",
	})
	if !errors.Is(err, service.ErrFeedbackClosed) {
		t.Fatalf("CreateReply err = %v, want ErrFeedbackClosed", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryCreateReplyLocksInsertsTouchesAndCommits(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT.*FROM feedbacks f.*WHERE 1=1 AND f.id = \$1 AND f.source = 'user' AND f.user_id = \$2\s+FOR UPDATE OF f`).
		WithArgs(int64(42), int64(7)).
		WillReturnRows(feedbackRows().AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusOpen, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, now, nil, nil, int64(0)))
	mock.ExpectQuery(`INSERT INTO feedback_replies \(feedback_id, author_role, content\)\s+VALUES \(\$1, \$2, \$3\)\s+RETURNING id, feedback_id, author_role, content, created_at`).
		WithArgs(int64(42), service.FeedbackActorUser, "reply").
		WillReturnRows(sqlmock.NewRows([]string{"id", "feedback_id", "author_role", "content", "created_at"}).
			AddRow(int64(5), int64(42), service.FeedbackActorUser, "reply", now))
	mock.ExpectExec(`UPDATE feedbacks SET updated_at = NOW\(\) WHERE id = \$1`).
		WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	repo := NewFeedbackRepository(db)
	reply, err := repo.CreateReply(context.Background(), service.FeedbackReplyInput{
		FeedbackID: 42,
		Scope:      service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: 7},
		AuthorRole: service.FeedbackActorUser,
		Content:    "reply",
	})
	if err != nil {
		t.Fatalf("CreateReply: %v", err)
	}
	if reply.ID != 5 || reply.FeedbackID != 42 || reply.AuthorRole != service.FeedbackActorUser {
		t.Fatalf("unexpected reply: %+v", reply)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryGetMapsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT.*FROM feedbacks f.*feedback_read_receipts rr.*WHERE 1=1 AND f.id = \$1`).
		WithArgs(int64(42), service.FeedbackActorAdmin, int64(900), service.FeedbackActorUser).
		WillReturnError(sql.ErrNoRows)

	repo := NewFeedbackRepository(db)
	_, err = repo.Get(context.Background(), 42, service.FeedbackScope{Kind: service.FeedbackActorAdmin}, service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 900})
	if infraerrors.Code(err) != 404 || infraerrors.Reason(err) != "FEEDBACK_NOT_FOUND" {
		t.Fatalf("Get err = %v, want not found application error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryMarkReadUsesMonotonicCursorAndReturnsIncomingUnread(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)

	mock.ExpectQuery(`SELECT.*FROM feedbacks f.*feedback_read_receipts rr.*WHERE 1=1 AND f.id = \$1 AND f.source = 'user' AND f.user_id = \$2`).
		WithArgs(int64(42), int64(7), service.FeedbackActorUser, int64(7), service.FeedbackActorAdmin).
		WillReturnRows(feedbackRows().AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusOpen, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, now, nil, nil, int64(2)))
	mock.ExpectQuery(`SELECT EXISTS\(.*FROM feedback_replies.*WHERE feedback_id = \$1 AND id = \$2`).
		WithArgs(int64(42), int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery(`INSERT INTO feedback_read_receipts .*ON CONFLICT \(feedback_id, reader_kind, reader_id\).*GREATEST\(feedback_read_receipts.last_read_reply_id, EXCLUDED.last_read_reply_id\).*fr.author_role = \$5.*fr.id > upserted.last_read_reply_id`).
		WithArgs(int64(42), service.FeedbackActorUser, int64(7), int64(5), service.FeedbackActorAdmin).
		WillReturnRows(sqlmock.NewRows([]string{"unread_count", "last_read_reply_id"}).AddRow(int64(1), int64(5)))

	repo := NewFeedbackRepository(db)
	state, err := repo.MarkRead(
		context.Background(),
		42,
		service.FeedbackScope{Kind: service.FeedbackActorUser, UserID: 7},
		service.FeedbackReader{Kind: service.FeedbackActorUser, ID: 7},
		5,
	)
	if err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if state.UnreadCount != 1 || state.LastReadReplyID != 5 {
		t.Fatalf("unexpected read state: %+v", state)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestFeedbackRepositoryMarkReadRejectsForeignCursorAfterScopePasses(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()
	now := time.Unix(1776790020, 0)

	mock.ExpectQuery(`SELECT.*FROM feedbacks f.*feedback_read_receipts rr.*WHERE 1=1 AND f.id = \$1`).
		WithArgs(int64(42), service.FeedbackActorAdmin, int64(900), service.FeedbackActorUser).
		WillReturnRows(feedbackRows().AddRow(int64(42), "title", "content", service.FeedbackSourceUser, service.FeedbackStatusOpen, service.FeedbackReplyStatusPending, int64(7), "u@example.com", "Test User", nil, "", "", nil, now, now, nil, nil, int64(1)))
	mock.ExpectQuery(`SELECT EXISTS\(.*FROM feedback_replies.*WHERE feedback_id = \$1 AND id = \$2`).
		WithArgs(int64(42), int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

	repo := NewFeedbackRepository(db)
	_, err = repo.MarkRead(
		context.Background(),
		42,
		service.FeedbackScope{Kind: service.FeedbackActorAdmin},
		service.FeedbackReader{Kind: service.FeedbackActorAdmin, ID: 900},
		99,
	)
	if !errors.Is(err, service.ErrFeedbackInvalidReadID) {
		t.Fatalf("MarkRead err = %v, want invalid read cursor", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func feedbackRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "title", "content", "source", "status", "reply_status", "user_id", "user_email", "user_name", "api_key_id", "key_name", "key_prefix", "member_id", "created_at", "updated_at", "closed_at", "closed_by", "unread_count"})
}
