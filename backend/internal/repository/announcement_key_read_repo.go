package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type announcementKeyReadRepository struct {
	db *sql.DB
}

func NewAnnouncementKeyReadRepository(db *sql.DB) service.AnnouncementKeyReadRepository {
	return &announcementKeyReadRepository{db: db}
}

func (r *announcementKeyReadRepository) MarkRead(ctx context.Context, announcementID, apiKeyID int64, readAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO key_announcement_reads (announcement_id, api_key_id, read_at, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (announcement_id, api_key_id) DO NOTHING`,
		announcementID, apiKeyID, readAt,
	)
	if err != nil {
		return fmt.Errorf("mark key announcement read: %w", err)
	}
	return nil
}

func (r *announcementKeyReadRepository) GetReadMapByAPIKey(ctx context.Context, apiKeyID int64, announcementIDs []int64) (map[int64]time.Time, error) {
	if len(announcementIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT announcement_id, read_at
		FROM key_announcement_reads
		WHERE api_key_id = $1
		  AND announcement_id = ANY($2)`,
		apiKeyID, pq.Array(announcementIDs),
	)
	if err != nil {
		return nil, fmt.Errorf("list key announcement reads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[int64]time.Time)
	for rows.Next() {
		var announcementID int64
		var readAt time.Time
		if err := rows.Scan(&announcementID, &readAt); err != nil {
			return nil, fmt.Errorf("scan key announcement read: %w", err)
		}
		out[announcementID] = readAt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate key announcement reads: %w", err)
	}
	return out, nil
}
