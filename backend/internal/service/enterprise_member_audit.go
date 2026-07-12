package service

import (
	"context"
	"encoding/json"
	"time"
)

// EnterpriseMemberAuditEvent is a credential-safe, append-only administration event.
type EnterpriseMemberAuditEvent struct {
	ID               int64           `json:"id"`
	EnterpriseUserID int64           `json:"enterprise_user_id"`
	MemberID         *int64          `json:"member_id,omitempty"`
	ActorUserID      *int64          `json:"actor_user_id,omitempty"`
	Action           string          `json:"action"`
	EntityType       string          `json:"entity_type"`
	EntityID         *int64          `json:"entity_id,omitempty"`
	BeforeData       json.RawMessage `json:"before_data"`
	AfterData        json.RawMessage `json:"after_data"`
	Metadata         json.RawMessage `json:"metadata"`
	CreatedAt        time.Time       `json:"created_at"`
}

type EnterpriseMemberAuditRepository interface {
	ListByOwner(ctx context.Context, ownerID int64, page, pageSize int) ([]EnterpriseMemberAuditEvent, int64, error)
	ListByMember(ctx context.Context, ownerID, memberID int64, page, pageSize int) ([]EnterpriseMemberAuditEvent, int64, error)
}
