package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type feedbackRepoStub struct {
	created   []FeedbackCreateInput
	replies   []FeedbackReplyInput
	createErr error
	replyErr  error
	closeErr  error
	getErr    error
	getItem   *Feedback
	closed    bool
	marked    []struct {
		feedbackID int64
		scope      FeedbackScope
		reader     FeedbackReader
		cursor     int64
	}
}

func (r *feedbackRepoStub) Create(_ context.Context, input FeedbackCreateInput) (*Feedback, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	r.created = append(r.created, input)
	return &Feedback{ID: int64(len(r.created)), Title: input.Title, Content: input.Content, Source: input.Source, Status: FeedbackStatusOpen, ReplyStatus: FeedbackReplyStatusPending, UserID: input.UserID, APIKeyID: input.APIKeyID, MemberID: input.MemberID, CreatedAt: time.Unix(1776790020, 0)}, nil
}

func (r *feedbackRepoStub) List(context.Context, pagination.PaginationParams, FeedbackListFilters) ([]Feedback, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *feedbackRepoStub) ListByScope(context.Context, pagination.PaginationParams, FeedbackListFilters, FeedbackScope, FeedbackReader) ([]Feedback, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *feedbackRepoStub) Get(context.Context, int64, FeedbackScope, FeedbackReader) (*Feedback, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.getItem != nil {
		return r.getItem, nil
	}
	return &Feedback{ID: 1, Status: FeedbackStatusOpen}, nil
}

func (r *feedbackRepoStub) ListReplies(context.Context, int64, pagination.PaginationParams, FeedbackScope) ([]FeedbackReply, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (r *feedbackRepoStub) CreateReply(_ context.Context, input FeedbackReplyInput) (*FeedbackReply, error) {
	if r.replyErr != nil {
		return nil, r.replyErr
	}
	r.replies = append(r.replies, input)
	return &FeedbackReply{ID: int64(len(r.replies)), FeedbackID: input.FeedbackID, AuthorRole: input.AuthorRole, Content: input.Content, CreatedAt: time.Unix(1776790020, 0)}, nil
}

func (r *feedbackRepoStub) Close(context.Context, int64, string, FeedbackScope, FeedbackReader) (*Feedback, error) {
	if r.closeErr != nil {
		return nil, r.closeErr
	}
	r.closed = true
	closedAt := time.Unix(1776790020, 0)
	closedBy := FeedbackActorUser
	return &Feedback{ID: 1, Status: FeedbackStatusClosed, ClosedAt: &closedAt, ClosedBy: &closedBy}, nil
}

func (r *feedbackRepoStub) MarkRead(_ context.Context, feedbackID int64, scope FeedbackScope, reader FeedbackReader, lastReadReplyID int64) (*FeedbackReadState, error) {
	r.marked = append(r.marked, struct {
		feedbackID int64
		scope      FeedbackScope
		reader     FeedbackReader
		cursor     int64
	}{feedbackID: feedbackID, scope: scope, reader: reader, cursor: lastReadReplyID})
	if r.getErr != nil {
		return nil, r.getErr
	}
	return &FeedbackReadState{UnreadCount: 2, LastReadReplyID: lastReadReplyID}, nil
}

type feedbackCooldownStub struct {
	mu       sync.Mutex
	claimed  map[string]bool
	calls    []string
	err      error
	retryFor time.Duration
}

func (s *feedbackCooldownStub) ClaimFeedbackCooldown(_ context.Context, identity string, _ time.Duration) (bool, time.Duration, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return false, 0, s.err
	}
	s.calls = append(s.calls, identity)
	if s.claimed == nil {
		s.claimed = make(map[string]bool)
	}
	if s.claimed[identity] {
		retry := s.retryFor
		if retry == 0 {
			retry = 42 * time.Second
		}
		return false, retry, nil
	}
	s.claimed[identity] = true
	return true, 0, nil
}

func TestFeedbackServiceValidationHappensBeforeCooldown(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)

	for _, content := range []string{"", "   ", "ok\x00bad"} {
		if _, err := svc.CreateForUser(context.Background(), 7, "", content); !errors.Is(err, ErrFeedbackInvalidContent) {
			t.Fatalf("CreateForUser(%q) err = %v, want invalid content", content, err)
		}
	}
	long := make([]rune, FeedbackContentMaxRunes+1)
	for i := range long {
		long[i] = '你'
	}
	if _, err := svc.CreateForUser(context.Background(), 7, "", string(long)); !errors.Is(err, ErrFeedbackInvalidContent) {
		t.Fatalf("long unicode content err = %v, want invalid content", err)
	}
	if len(cooldown.calls) != 0 || len(repo.created) != 0 {
		t.Fatalf("invalid content should not claim cooldown or write DB, calls=%v created=%v", cooldown.calls, repo.created)
	}
}

func TestFeedbackServiceNormalizesTitleAndKeepsLegacyContentOnlyCreate(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)

	if _, err := svc.CreateForUser(context.Background(), 7, "  标题\n\t含 空白  ", " first line\nsecond line "); err != nil {
		t.Fatalf("CreateForUser with title: %v", err)
	}
	if got := repo.created[0].Title; got != "标题 含 空白" {
		t.Fatalf("normalized title = %q", got)
	}

	if _, err := svc.CreateForUser(context.Background(), 8, "   ", "内容第一行\n第二行"); err != nil {
		t.Fatalf("legacy content-only CreateForUser: %v", err)
	}
	if got := repo.created[1].Title; got != "内容第一行 第二行" {
		t.Fatalf("fallback title = %q", got)
	}
}

func TestFeedbackServiceRejectsOverlongExplicitUnicodeTitle(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)
	long := make([]rune, FeedbackTitleMaxRunes+1)
	for i := range long {
		long[i] = '你'
	}

	if _, err := svc.CreateForUser(context.Background(), 7, string(long), "content"); !errors.Is(err, ErrFeedbackInvalidTitle) {
		t.Fatalf("overlong title err = %v, want invalid title", err)
	}
	if len(cooldown.calls) != 0 || len(repo.created) != 0 {
		t.Fatalf("invalid title should not claim cooldown or write DB, calls=%v created=%v", cooldown.calls, repo.created)
	}
}

func TestFeedbackServiceRejectsTitleContainingNULBeforeCooldown(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)

	if _, err := svc.CreateForUser(context.Background(), 7, "bad\x00title", "content"); !errors.Is(err, ErrFeedbackInvalidTitle) {
		t.Fatalf("NUL title err = %v, want invalid title", err)
	}
	if len(cooldown.calls) != 0 || len(repo.created) != 0 {
		t.Fatalf("invalid title should not claim cooldown or write DB, calls=%v created=%v", cooldown.calls, repo.created)
	}
}

func TestFeedbackServiceContentFallbackTitleTruncatesUnicode(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)
	long := make([]rune, FeedbackTitleMaxRunes+5)
	for i := range long {
		long[i] = '你'
	}

	if _, err := svc.CreateForUser(context.Background(), 7, "", string(long)); err != nil {
		t.Fatalf("content fallback should truncate title while accepting valid content: %v", err)
	}
	if got := len([]rune(repo.created[0].Title)); got != FeedbackTitleMaxRunes {
		t.Fatalf("fallback title runes = %d, want %d", got, FeedbackTitleMaxRunes)
	}
}

func TestFeedbackServiceCooldownIsPerStableIdentity(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)
	memberID := int64(9)

	if _, err := svc.CreateForUser(context.Background(), 7, "", " first "); err != nil {
		t.Fatalf("first user feedback: %v", err)
	}
	if _, err := svc.CreateForUser(context.Background(), 8, "", "second user"); err != nil {
		t.Fatalf("separate user feedback: %v", err)
	}
	if _, err := svc.CreateForKey(context.Background(), &PublicKeyUsageSession{APIKeyID: 101, UserID: 7, MemberID: &memberID}, "", "key feedback"); err != nil {
		t.Fatalf("key feedback: %v", err)
	}
	_, err := svc.CreateForUser(context.Background(), 7, "", "again")
	if !errors.Is(err, ErrFeedbackRateLimited) {
		t.Fatalf("repeat user feedback err = %v, want rate limited", err)
	}
	app := infraerrors.FromError(err)
	if got := app.Metadata["retry_after"]; got != "42" {
		t.Fatalf("retry_after = %q, want 42", got)
	}
	if len(repo.created) != 3 {
		t.Fatalf("created = %d, want 3", len(repo.created))
	}
	if repo.created[0].Content != "first" || repo.created[2].Source != FeedbackSourceKey || repo.created[2].APIKeyID == nil || *repo.created[2].APIKeyID != 101 {
		t.Fatalf("unexpected creates: %+v", repo.created)
	}
}

func TestFeedbackServiceConcurrentCooldownAllowsOne(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateForUser(context.Background(), 7, "", "content")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	successes := 0
	limited := 0
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrFeedbackRateLimited):
			limited++
		default:
			t.Fatalf("unexpected err: %v", err)
		}
	}
	if successes != 1 || limited != 15 {
		t.Fatalf("successes=%d limited=%d, want 1/15", successes, limited)
	}
}

func TestFeedbackServiceFailsClosedOnCooldownOrDBErrors(t *testing.T) {
	svc := NewFeedbackService(&feedbackRepoStub{}, &feedbackCooldownStub{err: errors.New("redis down")})
	if _, err := svc.CreateForUser(context.Background(), 7, "", "content"); !errors.Is(err, ErrFeedbackUnavailable) {
		t.Fatalf("redis err = %v, want unavailable", err)
	}

	cooldown := &feedbackCooldownStub{}
	repo := &feedbackRepoStub{createErr: errors.New("db down")}
	svc = NewFeedbackService(repo, cooldown)
	if _, err := svc.CreateForUser(context.Background(), 7, "", "content"); !errors.Is(err, ErrFeedbackUnavailable) {
		t.Fatalf("db err = %v, want unavailable", err)
	}
	if !cooldown.claimed["user:7"] {
		t.Fatal("DB failures keep the conservative cooldown claim")
	}
}

func TestFeedbackServiceReplyChecksTicketBeforeCooldownAndRejectsClosed(t *testing.T) {
	repo := &feedbackRepoStub{getItem: &Feedback{ID: 42, Status: FeedbackStatusClosed}}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)
	_, err := svc.ReplyForUser(context.Background(), 42, 7, "answer")
	if !errors.Is(err, ErrFeedbackClosed) {
		t.Fatalf("closed reply err = %v, want ErrFeedbackClosed", err)
	}
	if len(cooldown.calls) != 0 || len(repo.replies) != 0 {
		t.Fatalf("closed ticket should not claim cooldown or write reply, calls=%v replies=%v", cooldown.calls, repo.replies)
	}
}

func TestFeedbackServiceReplyWrongOwnerDoesNotClaimCooldown(t *testing.T) {
	repo := &feedbackRepoStub{getErr: infraerrors.NotFound("FEEDBACK_NOT_FOUND", "feedback not found")}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)
	_, err := svc.ReplyForUser(context.Background(), 42, 7, "answer")
	if infraerrors.Code(err) != 404 || infraerrors.Reason(err) != "FEEDBACK_NOT_FOUND" {
		t.Fatalf("wrong owner err = %v, want not found", err)
	}
	if len(cooldown.calls) != 0 || len(repo.replies) != 0 {
		t.Fatalf("wrong owner should not claim cooldown or write reply, calls=%v replies=%v", cooldown.calls, repo.replies)
	}
}

func TestFeedbackServiceReplyUsesCustomerCooldownAndAdminBypassesIt(t *testing.T) {
	repo := &feedbackRepoStub{}
	cooldown := &feedbackCooldownStub{}
	svc := NewFeedbackService(repo, cooldown)
	if _, err := svc.ReplyForUser(context.Background(), 42, 7, " first reply "); err != nil {
		t.Fatalf("user reply: %v", err)
	}
	if _, err := svc.ReplyAsAdmin(context.Background(), 42, 900, "admin reply"); err != nil {
		t.Fatalf("admin reply: %v", err)
	}
	_, err := svc.ReplyForUser(context.Background(), 42, 7, "second")
	if !errors.Is(err, ErrFeedbackRateLimited) {
		t.Fatalf("second user reply err = %v, want rate limited", err)
	}
	if len(repo.replies) != 2 || repo.replies[0].Content != "first reply" || repo.replies[1].AuthorRole != FeedbackActorAdmin {
		t.Fatalf("unexpected replies: %+v", repo.replies)
	}
	if len(cooldown.calls) != 2 {
		t.Fatalf("admin should not claim customer cooldown, calls=%v", cooldown.calls)
	}
}

func TestFeedbackServiceClosePreservesRepositoryApplicationErrors(t *testing.T) {
	repo := &feedbackRepoStub{closeErr: infraerrors.NotFound("FEEDBACK_NOT_FOUND", "feedback not found")}
	svc := NewFeedbackService(repo, &feedbackCooldownStub{})
	_, err := svc.Close(context.Background(), 42, 900)
	if infraerrors.Code(err) != 404 || infraerrors.Reason(err) != "FEEDBACK_NOT_FOUND" {
		t.Fatalf("Close err = %v, want FEEDBACK_NOT_FOUND 404", err)
	}
}

func TestFeedbackServiceMarkReadValidatesCursorAndUsesViewerIdentity(t *testing.T) {
	repo := &feedbackRepoStub{}
	svc := NewFeedbackService(repo, &feedbackCooldownStub{})

	if _, err := svc.MarkReadForUser(context.Background(), 42, 7, -1); !errors.Is(err, ErrFeedbackInvalidReadID) {
		t.Fatalf("negative cursor err = %v, want invalid read id", err)
	}
	state, err := svc.MarkReadForUser(context.Background(), 42, 7, 0)
	if err != nil {
		t.Fatalf("MarkReadForUser: %v", err)
	}
	if state.UnreadCount != 2 || state.LastReadReplyID != 0 {
		t.Fatalf("unexpected state: %+v", state)
	}
	if len(repo.marked) != 1 || repo.marked[0].scope.UserID != 7 || repo.marked[0].reader.Kind != FeedbackActorUser || repo.marked[0].reader.ID != 7 {
		t.Fatalf("wrong user read identity: %+v", repo.marked)
	}

	memberID := int64(9)
	session := &PublicKeyUsageSession{APIKeyID: 101, UserID: 7, MemberID: &memberID}
	if _, err := svc.MarkReadForKey(context.Background(), 42, session, 5); err != nil {
		t.Fatalf("MarkReadForKey: %v", err)
	}
	if repo.marked[1].scope.APIKeyID != 101 || repo.marked[1].reader.Kind != FeedbackSourceKey || repo.marked[1].reader.ID != 101 {
		t.Fatalf("wrong key read identity: %+v", repo.marked[1])
	}
}

func TestFeedbackServiceMarkReadPreservesRepositoryApplicationErrors(t *testing.T) {
	repo := &feedbackRepoStub{getErr: ErrFeedbackInvalidReadID}
	svc := NewFeedbackService(repo, &feedbackCooldownStub{})
	_, err := svc.MarkRead(context.Background(), 42, 900, 99)
	if !errors.Is(err, ErrFeedbackInvalidReadID) {
		t.Fatalf("MarkRead err = %v, want invalid read cursor", err)
	}
}
