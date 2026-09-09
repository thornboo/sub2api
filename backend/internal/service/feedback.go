package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	FeedbackSourceUser = "user"
	FeedbackSourceKey  = "key"

	FeedbackStatusOpen   = "open"
	FeedbackStatusClosed = "closed"

	FeedbackReplyStatusPending = "pending"
	FeedbackReplyStatusReplied = "replied"

	FeedbackActorUser  = "user"
	FeedbackActorAdmin = "admin"

	FeedbackContentMaxRunes = 2000
	FeedbackTitleMaxRunes   = 120
	FeedbackCooldown        = time.Minute
)

var (
	ErrFeedbackInvalidContent     = infraerrors.BadRequest("FEEDBACK_INVALID_CONTENT", "feedback content must be 1..2000 characters")
	ErrFeedbackInvalidStatus      = infraerrors.BadRequest("FEEDBACK_INVALID_STATUS", "feedback status must be open or closed")
	ErrFeedbackInvalidTitle       = infraerrors.BadRequest("FEEDBACK_INVALID_TITLE", "feedback title must be at most 120 characters")
	ErrFeedbackInvalidReplyStatus = infraerrors.BadRequest("FEEDBACK_INVALID_REPLY_STATUS", "feedback reply_status must be pending or replied")
	ErrFeedbackClosed             = infraerrors.New(409, "FEEDBACK_CLOSED", "feedback is closed")
	ErrFeedbackRateLimited        = infraerrors.TooManyRequests("FEEDBACK_RATE_LIMITED", "feedback can be submitted once per minute")
	ErrFeedbackUnavailable        = infraerrors.ServiceUnavailable("FEEDBACK_UNAVAILABLE", "feedback is temporarily unavailable")
	ErrFeedbackInvalidID          = infraerrors.BadRequest("FEEDBACK_INVALID_ID", "invalid feedback id")
	ErrFeedbackInvalidReadID      = infraerrors.BadRequest("FEEDBACK_INVALID_READ_ID", "invalid feedback read cursor")
)

type Feedback struct {
	ID          int64
	Title       string
	Content     string
	Source      string
	Status      string
	ReplyStatus string
	UserID      int64
	UserEmail   string
	UserName    string
	APIKeyID    *int64
	KeyName     string
	KeyPrefix   string
	MemberID    *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClosedAt    *time.Time
	ClosedBy    *string
	UnreadCount int64
}

type FeedbackCreateInput struct {
	Title    string
	Content  string
	Source   string
	UserID   int64
	APIKeyID *int64
	MemberID *int64
}

type FeedbackListFilters struct {
	Status      string
	ReplyStatus string
}

type FeedbackScope struct {
	Kind     string
	UserID   int64
	APIKeyID int64
	MemberID *int64
}

type FeedbackReader struct {
	Kind string
	ID   int64
}

type FeedbackReply struct {
	ID         int64
	FeedbackID int64
	AuthorRole string
	Content    string
	CreatedAt  time.Time
}

type FeedbackReplyInput struct {
	FeedbackID  int64
	Scope       FeedbackScope
	Reader      FeedbackReader
	AuthorRole  string
	Content     string
	UseCooldown bool
}

type FeedbackReadState struct {
	UnreadCount     int64
	LastReadReplyID int64
}

type FeedbackRepository interface {
	Create(ctx context.Context, input FeedbackCreateInput) (*Feedback, error)
	ListByScope(ctx context.Context, params pagination.PaginationParams, filters FeedbackListFilters, scope FeedbackScope, reader FeedbackReader) ([]Feedback, *pagination.PaginationResult, error)
	Get(ctx context.Context, id int64, scope FeedbackScope, reader FeedbackReader) (*Feedback, error)
	ListReplies(ctx context.Context, feedbackID int64, params pagination.PaginationParams, scope FeedbackScope) ([]FeedbackReply, *pagination.PaginationResult, error)
	CreateReply(ctx context.Context, input FeedbackReplyInput) (*FeedbackReply, error)
	Close(ctx context.Context, id int64, closedBy string, scope FeedbackScope, reader FeedbackReader) (*Feedback, error)
	MarkRead(ctx context.Context, feedbackID int64, scope FeedbackScope, reader FeedbackReader, lastReadReplyID int64) (*FeedbackReadState, error)
}

type FeedbackCooldownStore interface {
	ClaimFeedbackCooldown(ctx context.Context, identity string, ttl time.Duration) (bool, time.Duration, error)
}

type FeedbackService struct {
	repo     FeedbackRepository
	cooldown FeedbackCooldownStore
}

func NewFeedbackService(repo FeedbackRepository, cooldown FeedbackCooldownStore) *FeedbackService {
	return &FeedbackService{repo: repo, cooldown: cooldown}
}

type FeedbackCreateResult struct {
	Feedback   *Feedback
	RetryAfter time.Duration
}

type FeedbackReplyResult struct {
	Message    *FeedbackReply
	RetryAfter time.Duration
}

func (s *FeedbackService) CreateForUser(ctx context.Context, userID int64, title string, content string) (*FeedbackCreateResult, error) {
	if userID <= 0 {
		return nil, ErrFeedbackUnavailable
	}
	return s.create(ctx, fmt.Sprintf("user:%d", userID), FeedbackCreateInput{
		Title:   title,
		Content: content,
		Source:  FeedbackSourceUser,
		UserID:  userID,
	})
}

func (s *FeedbackService) CreateForKey(ctx context.Context, session *PublicKeyUsageSession, title string, content string) (*FeedbackCreateResult, error) {
	if session == nil || session.APIKeyID <= 0 || session.UserID <= 0 {
		return nil, ErrPublicKeyUsageSessionInvalid
	}
	apiKeyID := session.APIKeyID
	return s.create(ctx, fmt.Sprintf("key:%d", session.APIKeyID), FeedbackCreateInput{
		Title:    title,
		Content:  content,
		Source:   FeedbackSourceKey,
		UserID:   session.UserID,
		APIKeyID: &apiKeyID,
		MemberID: session.MemberID,
	})
}

func (s *FeedbackService) create(ctx context.Context, identity string, input FeedbackCreateInput) (*FeedbackCreateResult, error) {
	if s == nil || s.repo == nil || s.cooldown == nil {
		return nil, ErrFeedbackUnavailable
	}
	content, err := NormalizeFeedbackContent(input.Content)
	if err != nil {
		return nil, err
	}
	input.Content = content
	title, err := NormalizeFeedbackTitle(input.Title, content)
	if err != nil {
		return nil, err
	}
	input.Title = title

	ok, retryAfter, err := s.cooldown.ClaimFeedbackCooldown(ctx, identity, FeedbackCooldown)
	if err != nil {
		return nil, ErrFeedbackUnavailable.WithCause(err)
	}
	if !ok {
		if retryAfter <= 0 {
			retryAfter = FeedbackCooldown
		}
		return nil, ErrFeedbackRateLimited.WithMetadata(map[string]string{"retry_after": fmt.Sprintf("%d", int(retryAfter.Seconds()+0.999))})
	}

	created, err := s.repo.Create(ctx, input)
	if err != nil {
		return nil, ErrFeedbackUnavailable.WithCause(err)
	}
	return &FeedbackCreateResult{Feedback: created, RetryAfter: FeedbackCooldown}, nil
}

func (s *FeedbackService) List(ctx context.Context, params pagination.PaginationParams, filters FeedbackListFilters, adminUserID int64) ([]Feedback, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrFeedbackUnavailable
	}
	reader, err := feedbackAdminReader(adminUserID)
	if err != nil {
		return nil, nil, err
	}
	params, err = normalizeFeedbackListParams(params, filters)
	if err != nil {
		return nil, nil, err
	}
	return s.repo.ListByScope(ctx, params, filters, FeedbackScope{Kind: FeedbackActorAdmin}, reader)
}

func (s *FeedbackService) ListForUser(ctx context.Context, params pagination.PaginationParams, filters FeedbackListFilters, userID int64) ([]Feedback, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil || userID <= 0 {
		return nil, nil, ErrFeedbackUnavailable
	}
	params, err := normalizeFeedbackListParams(params, filters)
	if err != nil {
		return nil, nil, err
	}
	return s.repo.ListByScope(ctx, params, filters, FeedbackScope{Kind: FeedbackActorUser, UserID: userID}, FeedbackReader{Kind: FeedbackActorUser, ID: userID})
}

func (s *FeedbackService) ListForKey(ctx context.Context, params pagination.PaginationParams, filters FeedbackListFilters, session *PublicKeyUsageSession) ([]Feedback, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrFeedbackUnavailable
	}
	scope, err := feedbackKeyScope(session)
	if err != nil {
		return nil, nil, err
	}
	params, err = normalizeFeedbackListParams(params, filters)
	if err != nil {
		return nil, nil, err
	}
	return s.repo.ListByScope(ctx, params, filters, scope, FeedbackReader{Kind: FeedbackSourceKey, ID: session.APIKeyID})
}

func (s *FeedbackService) Get(ctx context.Context, id int64, adminUserID int64) (*Feedback, error) {
	reader, err := feedbackAdminReader(adminUserID)
	if err != nil {
		return nil, err
	}
	return s.get(ctx, id, FeedbackScope{Kind: FeedbackActorAdmin}, reader)
}

func (s *FeedbackService) GetForUser(ctx context.Context, id int64, userID int64) (*Feedback, error) {
	return s.get(ctx, id, FeedbackScope{Kind: FeedbackActorUser, UserID: userID}, FeedbackReader{Kind: FeedbackActorUser, ID: userID})
}

func (s *FeedbackService) GetForKey(ctx context.Context, id int64, session *PublicKeyUsageSession) (*Feedback, error) {
	scope, err := feedbackKeyScope(session)
	if err != nil {
		return nil, err
	}
	return s.get(ctx, id, scope, FeedbackReader{Kind: FeedbackSourceKey, ID: session.APIKeyID})
}

func (s *FeedbackService) get(ctx context.Context, id int64, scope FeedbackScope, reader FeedbackReader) (*Feedback, error) {
	if s == nil || s.repo == nil {
		return nil, ErrFeedbackUnavailable
	}
	if id <= 0 {
		return nil, ErrFeedbackInvalidID
	}
	if err := validateFeedbackReader(reader); err != nil {
		return nil, err
	}
	item, err := s.repo.Get(ctx, id, scope, reader)
	return handleFeedbackRepoItem(item, err)
}

func (s *FeedbackService) ListReplies(ctx context.Context, feedbackID int64, params pagination.PaginationParams) ([]FeedbackReply, *pagination.PaginationResult, error) {
	return s.listReplies(ctx, feedbackID, params, FeedbackScope{Kind: FeedbackActorAdmin})
}

func (s *FeedbackService) ListRepliesForUser(ctx context.Context, feedbackID int64, params pagination.PaginationParams, userID int64) ([]FeedbackReply, *pagination.PaginationResult, error) {
	return s.listReplies(ctx, feedbackID, params, FeedbackScope{Kind: FeedbackActorUser, UserID: userID})
}

func (s *FeedbackService) ListRepliesForKey(ctx context.Context, feedbackID int64, params pagination.PaginationParams, session *PublicKeyUsageSession) ([]FeedbackReply, *pagination.PaginationResult, error) {
	scope, err := feedbackKeyScope(session)
	if err != nil {
		return nil, nil, err
	}
	return s.listReplies(ctx, feedbackID, params, scope)
}

func (s *FeedbackService) listReplies(ctx context.Context, feedbackID int64, params pagination.PaginationParams, scope FeedbackScope) ([]FeedbackReply, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrFeedbackUnavailable
	}
	if feedbackID <= 0 {
		return nil, nil, ErrFeedbackInvalidID
	}
	params = normalizeFeedbackPagination(params)
	return s.repo.ListReplies(ctx, feedbackID, params, scope)
}

func (s *FeedbackService) ReplyForUser(ctx context.Context, feedbackID int64, userID int64, content string) (*FeedbackReplyResult, error) {
	if userID <= 0 {
		return nil, ErrFeedbackUnavailable
	}
	return s.reply(ctx, feedbackID, fmt.Sprintf("user:%d", userID), FeedbackReplyInput{
		FeedbackID:  feedbackID,
		Scope:       FeedbackScope{Kind: FeedbackActorUser, UserID: userID},
		Reader:      FeedbackReader{Kind: FeedbackActorUser, ID: userID},
		AuthorRole:  FeedbackActorUser,
		Content:     content,
		UseCooldown: true,
	})
}

func (s *FeedbackService) ReplyForKey(ctx context.Context, feedbackID int64, session *PublicKeyUsageSession, content string) (*FeedbackReplyResult, error) {
	scope, err := feedbackKeyScope(session)
	if err != nil {
		return nil, err
	}
	return s.reply(ctx, feedbackID, fmt.Sprintf("key:%d", session.APIKeyID), FeedbackReplyInput{
		FeedbackID:  feedbackID,
		Scope:       scope,
		Reader:      FeedbackReader{Kind: FeedbackSourceKey, ID: session.APIKeyID},
		AuthorRole:  FeedbackActorUser,
		Content:     content,
		UseCooldown: true,
	})
}

func (s *FeedbackService) ReplyAsAdmin(ctx context.Context, feedbackID int64, adminUserID int64, content string) (*FeedbackReplyResult, error) {
	reader, err := feedbackAdminReader(adminUserID)
	if err != nil {
		return nil, err
	}
	return s.reply(ctx, feedbackID, "", FeedbackReplyInput{
		FeedbackID: feedbackID,
		Scope:      FeedbackScope{Kind: FeedbackActorAdmin},
		Reader:     reader,
		AuthorRole: FeedbackActorAdmin,
		Content:    content,
	})
}

func (s *FeedbackService) reply(ctx context.Context, feedbackID int64, identity string, input FeedbackReplyInput) (*FeedbackReplyResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrFeedbackUnavailable
	}
	if input.UseCooldown && s.cooldown == nil {
		return nil, ErrFeedbackUnavailable
	}
	if feedbackID <= 0 {
		return nil, ErrFeedbackInvalidID
	}
	content, err := NormalizeFeedbackContent(input.Content)
	if err != nil {
		return nil, err
	}
	input.Content = content

	if err := validateFeedbackReader(input.Reader); err != nil {
		return nil, err
	}
	item, err := s.repo.Get(ctx, feedbackID, input.Scope, input.Reader)
	if _, err = handleFeedbackRepoItem(item, err); err != nil {
		return nil, err
	}
	if item.Status == FeedbackStatusClosed {
		return nil, ErrFeedbackClosed
	}

	retryAfter := time.Duration(0)
	if input.UseCooldown {
		ok, wait, err := s.cooldown.ClaimFeedbackCooldown(ctx, identity, FeedbackCooldown)
		if err != nil {
			return nil, ErrFeedbackUnavailable.WithCause(err)
		}
		if !ok {
			if wait <= 0 {
				wait = FeedbackCooldown
			}
			return nil, ErrFeedbackRateLimited.WithMetadata(map[string]string{"retry_after": fmt.Sprintf("%d", int(wait.Seconds()+0.999))})
		}
		retryAfter = FeedbackCooldown
	}

	reply, err := s.repo.CreateReply(ctx, input)
	if err != nil {
		if appErr := infraerrors.FromError(err); appErr != nil && appErr.Reason != infraerrors.UnknownReason {
			return nil, err
		}
		return nil, ErrFeedbackUnavailable.WithCause(err)
	}
	return &FeedbackReplyResult{Message: reply, RetryAfter: retryAfter}, nil
}

func (s *FeedbackService) Close(ctx context.Context, id int64, adminUserID int64) (*Feedback, error) {
	reader, err := feedbackAdminReader(adminUserID)
	if err != nil {
		return nil, err
	}
	return s.close(ctx, id, FeedbackActorAdmin, FeedbackScope{Kind: FeedbackActorAdmin}, reader)
}

func (s *FeedbackService) CloseForUser(ctx context.Context, id int64, userID int64) (*Feedback, error) {
	if userID <= 0 {
		return nil, ErrFeedbackUnavailable
	}
	return s.close(ctx, id, FeedbackActorUser, FeedbackScope{Kind: FeedbackActorUser, UserID: userID}, FeedbackReader{Kind: FeedbackActorUser, ID: userID})
}

func (s *FeedbackService) CloseForKey(ctx context.Context, id int64, session *PublicKeyUsageSession) (*Feedback, error) {
	scope, err := feedbackKeyScope(session)
	if err != nil {
		return nil, err
	}
	return s.close(ctx, id, FeedbackActorUser, scope, FeedbackReader{Kind: FeedbackSourceKey, ID: session.APIKeyID})
}

func (s *FeedbackService) close(ctx context.Context, id int64, closedBy string, scope FeedbackScope, reader FeedbackReader) (*Feedback, error) {
	if s == nil || s.repo == nil {
		return nil, ErrFeedbackUnavailable
	}
	if id <= 0 {
		return nil, ErrFeedbackInvalidID
	}
	if err := validateFeedbackReader(reader); err != nil {
		return nil, err
	}
	item, err := s.repo.Close(ctx, id, closedBy, scope, reader)
	return handleFeedbackRepoItem(item, err)
}

func (s *FeedbackService) MarkRead(ctx context.Context, id int64, adminUserID int64, lastReadReplyID int64) (*FeedbackReadState, error) {
	reader, err := feedbackAdminReader(adminUserID)
	if err != nil {
		return nil, err
	}
	return s.markRead(ctx, id, FeedbackScope{Kind: FeedbackActorAdmin}, reader, lastReadReplyID)
}

func (s *FeedbackService) MarkReadForUser(ctx context.Context, id int64, userID int64, lastReadReplyID int64) (*FeedbackReadState, error) {
	if userID <= 0 {
		return nil, ErrFeedbackUnavailable
	}
	return s.markRead(ctx, id, FeedbackScope{Kind: FeedbackActorUser, UserID: userID}, FeedbackReader{Kind: FeedbackActorUser, ID: userID}, lastReadReplyID)
}

func (s *FeedbackService) MarkReadForKey(ctx context.Context, id int64, session *PublicKeyUsageSession, lastReadReplyID int64) (*FeedbackReadState, error) {
	scope, err := feedbackKeyScope(session)
	if err != nil {
		return nil, err
	}
	return s.markRead(ctx, id, scope, FeedbackReader{Kind: FeedbackSourceKey, ID: session.APIKeyID}, lastReadReplyID)
}

func (s *FeedbackService) markRead(ctx context.Context, id int64, scope FeedbackScope, reader FeedbackReader, lastReadReplyID int64) (*FeedbackReadState, error) {
	if s == nil || s.repo == nil {
		return nil, ErrFeedbackUnavailable
	}
	if id <= 0 {
		return nil, ErrFeedbackInvalidID
	}
	if lastReadReplyID < 0 {
		return nil, ErrFeedbackInvalidReadID
	}
	if err := validateFeedbackReader(reader); err != nil {
		return nil, err
	}
	state, err := s.repo.MarkRead(ctx, id, scope, reader, lastReadReplyID)
	if err != nil {
		var appErr *infraerrors.ApplicationError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, ErrFeedbackUnavailable.WithCause(err)
	}
	return state, nil
}

func normalizeFeedbackListParams(params pagination.PaginationParams, filters FeedbackListFilters) (pagination.PaginationParams, error) {
	if filters.Status != "" && !IsFeedbackStatus(filters.Status) {
		return params, ErrFeedbackInvalidStatus
	}
	if filters.ReplyStatus != "" && !IsFeedbackReplyStatus(filters.ReplyStatus) {
		return params, ErrFeedbackInvalidReplyStatus
	}
	params = normalizeFeedbackPagination(params)
	params.SortBy = "updated_at"
	params.SortOrder = pagination.SortOrderDesc
	return params, nil
}

func normalizeFeedbackPagination(params pagination.PaginationParams) pagination.PaginationParams {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 {
		params.PageSize = 20
	}
	if params.PageSize > 100 {
		params.PageSize = 100
	}
	return params
}

func handleFeedbackRepoItem(item *Feedback, err error) (*Feedback, error) {
	if err != nil {
		var appErr *infraerrors.ApplicationError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, ErrFeedbackUnavailable.WithCause(err)
	}
	return item, nil
}

func feedbackKeyScope(session *PublicKeyUsageSession) (FeedbackScope, error) {
	if session == nil || session.APIKeyID <= 0 || session.UserID <= 0 {
		return FeedbackScope{}, ErrPublicKeyUsageSessionInvalid
	}
	return FeedbackScope{Kind: FeedbackSourceKey, UserID: session.UserID, APIKeyID: session.APIKeyID, MemberID: session.MemberID}, nil
}

func feedbackAdminReader(adminUserID int64) (FeedbackReader, error) {
	if adminUserID <= 0 {
		return FeedbackReader{}, ErrFeedbackUnavailable
	}
	return FeedbackReader{Kind: FeedbackActorAdmin, ID: adminUserID}, nil
}

func validateFeedbackReader(reader FeedbackReader) error {
	if reader.ID <= 0 {
		return ErrFeedbackUnavailable
	}
	switch reader.Kind {
	case FeedbackActorAdmin, FeedbackActorUser, FeedbackSourceKey:
		return nil
	default:
		return ErrFeedbackUnavailable
	}
}

func NormalizeFeedbackContent(content string) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || strings.ContainsRune(trimmed, '\x00') || utf8.RuneCountInString(trimmed) > FeedbackContentMaxRunes {
		return "", ErrFeedbackInvalidContent
	}
	return trimmed, nil
}

func NormalizeFeedbackTitle(title string, content string) (string, error) {
	if strings.ContainsRune(title, '\x00') {
		return "", ErrFeedbackInvalidTitle
	}
	normalized := singleLineWhitespace(title)
	if normalized == "" {
		normalized = feedbackTitleFromContent(content)
	}
	if utf8.RuneCountInString(normalized) > FeedbackTitleMaxRunes {
		return "", ErrFeedbackInvalidTitle
	}
	return normalized, nil
}

func FeedbackTitleOrFallback(title string, content string) string {
	normalized := singleLineWhitespace(title)
	if normalized != "" {
		if utf8.RuneCountInString(normalized) <= FeedbackTitleMaxRunes {
			return normalized
		}
		return truncateRunes(normalized, FeedbackTitleMaxRunes)
	}
	return feedbackTitleFromContent(content)
}

func feedbackTitleFromContent(content string) string {
	normalized := singleLineWhitespace(content)
	if normalized == "" {
		return "Ticket"
	}
	return truncateRunes(normalized, FeedbackTitleMaxRunes)
}

func singleLineWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func truncateRunes(value string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(value) <= maxRunes {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxRunes])
}

func IsFeedbackStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case FeedbackStatusOpen, FeedbackStatusClosed:
		return true
	default:
		return false
	}
}

func IsFeedbackReplyStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case FeedbackReplyStatusPending, FeedbackReplyStatusReplied:
		return true
	default:
		return false
	}
}
