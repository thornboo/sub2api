package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type enterpriseMemberAuditRepository struct{ db *sql.DB }

func NewEnterpriseMemberAuditRepository(db *sql.DB) service.EnterpriseMemberAuditRepository {
	return &enterpriseMemberAuditRepository{db: db}
}

func (r *enterpriseMemberAuditRepository) ListByOwner(ctx context.Context, ownerID int64, page, pageSize int) ([]service.EnterpriseMemberAuditEvent, int64, error) {
	page, pageSize, err := normalizeEnterpriseMemberAuditPage(r, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enterprise_member_audit_logs
		WHERE enterprise_user_id = $1`, ownerID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, enterprise_user_id, member_id, actor_user_id, action, entity_type,
		       entity_id, before_data, after_data, metadata, created_at
		FROM enterprise_member_audit_logs
		WHERE enterprise_user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, ownerID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return scanEnterpriseMemberAuditEvents(rows, pageSize, total)
}

func (r *enterpriseMemberAuditRepository) ListByMember(ctx context.Context, ownerID, memberID int64, page, pageSize int) ([]service.EnterpriseMemberAuditEvent, int64, error) {
	page, pageSize, err := normalizeEnterpriseMemberAuditPage(r, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM enterprise_member_audit_logs
		WHERE enterprise_user_id = $1 AND member_id = $2`, ownerID, memberID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, enterprise_user_id, member_id, actor_user_id, action, entity_type,
		       entity_id, before_data, after_data, metadata, created_at
		FROM enterprise_member_audit_logs
		WHERE enterprise_user_id = $1 AND member_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4`, ownerID, memberID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	return scanEnterpriseMemberAuditEvents(rows, pageSize, total)
}

func normalizeEnterpriseMemberAuditPage(r *enterpriseMemberAuditRepository, page, pageSize int) (int, int, error) {
	if r == nil || r.db == nil {
		return 0, 0, errors.New("enterprise member audit repository db is nil")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize, nil
}

func scanEnterpriseMemberAuditEvents(rows *sql.Rows, capacity int, total int64) ([]service.EnterpriseMemberAuditEvent, int64, error) {
	defer func() { _ = rows.Close() }()

	items := make([]service.EnterpriseMemberAuditEvent, 0, capacity)
	for rows.Next() {
		var item service.EnterpriseMemberAuditEvent
		if err := rows.Scan(
			&item.ID,
			&item.EnterpriseUserID,
			&item.MemberID,
			&item.ActorUserID,
			&item.Action,
			&item.EntityType,
			&item.EntityID,
			&item.BeforeData,
			&item.AfterData,
			&item.Metadata,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
