package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestAnnouncementKeyReadRepositoryMarkReadIsIdempotentByAPIKey(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	readAt := time.Unix(1776790020, 0)
	mock.ExpectExec(`INSERT INTO key_announcement_reads .* ON CONFLICT \(announcement_id, api_key_id\) DO NOTHING`).
		WithArgs(int64(10), int64(101), readAt).
		WillReturnResult(sqlmock.NewResult(0, 0))

	repo := NewAnnouncementKeyReadRepository(db)
	if err := repo.MarkRead(context.Background(), 10, 101, readAt); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAnnouncementKeyReadRepositoryGetReadMapScopesByAPIKey(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	readAt := time.Unix(1776790020, 0)
	mock.ExpectQuery(`FROM key_announcement_reads\s+WHERE api_key_id = \$1\s+AND announcement_id = ANY\(\$2\)`).
		WithArgs(int64(101), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"announcement_id", "read_at"}).AddRow(int64(10), readAt))

	repo := NewAnnouncementKeyReadRepository(db)
	readMap, err := repo.GetReadMapByAPIKey(context.Background(), 101, []int64{10, 11})
	if err != nil {
		t.Fatalf("GetReadMapByAPIKey: %v", err)
	}
	if got, ok := readMap[10]; !ok || !got.Equal(readAt) {
		t.Fatalf("read map = %+v, want announcement 10 at %v", readMap, readAt)
	}
	if _, ok := readMap[11]; ok {
		t.Fatalf("read map should not include unread announcement: %+v", readMap)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
