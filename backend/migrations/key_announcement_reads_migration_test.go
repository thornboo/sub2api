package migrations

import (
	"strings"
	"testing"
)

func TestKeyAnnouncementReadsMigrationDefinesKeyScopedReadState(t *testing.T) {
	data, err := FS.ReadFile("238_key_announcement_reads.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS key_announcement_reads",
		"announcement_id BIGINT NOT NULL REFERENCES announcements(id) ON DELETE CASCADE",
		"api_key_id BIGINT NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE",
		"idx_key_announcement_reads_announcement_api_key",
		"ON key_announcement_reads(announcement_id, api_key_id)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q:\n%s", want, sql)
		}
	}
}
