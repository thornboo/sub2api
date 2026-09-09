package migrations

import (
	"strings"
	"testing"
)

func TestFeedbacksMigrationDefinesMinimalTextOnlyFeedback(t *testing.T) {
	data, err := FS.ReadFile("239_create_feedbacks.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS feedbacks",
		"content TEXT NOT NULL",
		"source TEXT NOT NULL",
		"status TEXT NOT NULL DEFAULT 'pending'",
		"user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE",
		"api_key_id BIGINT NULL REFERENCES api_keys(id) ON DELETE SET NULL",
		"member_id BIGINT NULL REFERENCES enterprise_members(id) ON DELETE SET NULL",
		"char_length(btrim(content)) BETWEEN 1 AND 2000",
		"source IN ('user', 'key')",
		"status IN ('pending', 'processed')",
		"source = 'key' OR",
		"(source = 'user' AND api_key_id IS NULL AND member_id IS NULL)",
		"idx_feedbacks_status_created",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "source = 'key' AND api_key_id IS NOT NULL") {
		t.Fatal("key-sourced feedback must survive API Key deletion where ON DELETE SET NULL clears api_key_id")
	}
}

func TestFeedbackTicketsMigrationConvertsFeedbackIntoThreadedTickets(t *testing.T) {
	data, err := FS.ReadFile("240_feedback_tickets.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, want := range []string{
		"ADD COLUMN IF NOT EXISTS closed_at TIMESTAMPTZ NULL",
		"ADD COLUMN IF NOT EXISTS closed_by TEXT NULL",
		"ADD COLUMN IF NOT EXISTS origin_member_id BIGINT NULL",
		"SET origin_member_id = member_id",
		"DROP CONSTRAINT IF EXISTS feedbacks_status_check",
		"WHEN 'processed' THEN 'closed'",
		"ALTER COLUMN status SET DEFAULT 'open'",
		"status IN ('open', 'closed')",
		"closed_by IS NULL OR closed_by IN ('user', 'admin')",
		"CREATE TABLE IF NOT EXISTS feedback_replies",
		"feedback_id BIGINT NOT NULL REFERENCES feedbacks(id) ON DELETE CASCADE",
		"author_role IN ('user', 'admin')",
		"char_length(btrim(content)) BETWEEN 1 AND 2000",
		"idx_feedback_replies_feedback_created",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q:\n%s", want, sql)
		}
	}
	if strings.Index(sql, "DROP CONSTRAINT IF EXISTS feedbacks_status_check") > strings.Index(sql, "WHEN 'processed' THEN 'closed'") {
		t.Fatalf("migration must drop the old pending/processed check before writing open/closed:\n%s", sql)
	}
}

func TestFeedbackReadReceiptsMigrationDefinesPersistentUnreadCursors(t *testing.T) {
	data, err := FS.ReadFile("241_feedback_read_receipts.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS feedback_read_receipts",
		"feedback_id BIGINT NOT NULL REFERENCES feedbacks(id) ON DELETE CASCADE",
		"reader_kind TEXT NOT NULL",
		"reader_id BIGINT NOT NULL",
		"last_read_reply_id BIGINT NOT NULL DEFAULT 0",
		"PRIMARY KEY (feedback_id, reader_kind, reader_id)",
		"reader_kind IN ('user', 'key', 'admin')",
		"last_read_reply_id >= 0",
		"idx_feedback_read_receipts_reader",
		"idx_feedback_replies_feedback_author_id",
		"ON feedback_replies(feedback_id, author_role, id)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q:\n%s", want, sql)
		}
	}
}

func TestFeedbackTicketTitleMigrationAddsTitle(t *testing.T) {
	data, err := FS.ReadFile("242_feedback_ticket_title.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, want := range []string{
		"ADD COLUMN IF NOT EXISTS title TEXT NOT NULL DEFAULT ''",
		"left(regexp_replace(btrim(content), '\\s+', ' ', 'g'), 120)",
		"feedbacks_title_length_check CHECK (char_length(title) <= 120)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q:\n%s", want, sql)
		}
	}
	for _, unexpected := range []string{"reply_status", "feedback_replies"} {
		if strings.Contains(sql, unexpected) {
			t.Fatalf("title migration must not add reply-status schema/index changes containing %q:\n%s", unexpected, sql)
		}
	}
}
