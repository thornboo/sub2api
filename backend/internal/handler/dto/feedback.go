package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Feedback struct {
	ID          int64      `json:"id"`
	Content     string     `json:"content"`
	Source      string     `json:"source"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at"`
	ClosedBy    *string    `json:"closed_by"`
	UnreadCount int64      `json:"unread_count"`
}

type AdminFeedback struct {
	Feedback
	UserID    int64  `json:"user_id"`
	UserEmail string `json:"user_email"`
	APIKeyID  *int64 `json:"api_key_id"`
	KeyName   string `json:"key_name"`
	KeyPrefix string `json:"key_prefix"`
	MemberID  *int64 `json:"member_id"`
}

type FeedbackReply struct {
	ID         int64     `json:"id"`
	FeedbackID int64     `json:"feedback_id"`
	AuthorRole string    `json:"author_role"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

func FeedbackFromService(item *service.Feedback) *Feedback {
	if item == nil {
		return nil
	}
	return &Feedback{
		ID:          item.ID,
		Content:     item.Content,
		Source:      item.Source,
		Status:      item.Status,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		ClosedAt:    item.ClosedAt,
		ClosedBy:    item.ClosedBy,
		UnreadCount: item.UnreadCount,
	}
}

func AdminFeedbackFromService(item *service.Feedback) *AdminFeedback {
	if item == nil {
		return nil
	}
	return &AdminFeedback{
		Feedback:  *FeedbackFromService(item),
		UserID:    item.UserID,
		UserEmail: item.UserEmail,
		APIKeyID:  item.APIKeyID,
		KeyName:   item.KeyName,
		KeyPrefix: item.KeyPrefix,
		MemberID:  item.MemberID,
	}
}

func FeedbackReplyFromService(item *service.FeedbackReply) *FeedbackReply {
	if item == nil {
		return nil
	}
	return &FeedbackReply{
		ID:         item.ID,
		FeedbackID: item.FeedbackID,
		AuthorRole: item.AuthorRole,
		Content:    item.Content,
		CreatedAt:  item.CreatedAt,
	}
}
