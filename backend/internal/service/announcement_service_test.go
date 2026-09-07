package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type announcementRepoStub struct {
	item        *Announcement
	activeItems []Announcement
}

func (s *announcementRepoStub) Create(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (s *announcementRepoStub) GetByID(_ context.Context, _ int64) (*Announcement, error) {
	if s.item == nil {
		return nil, ErrAnnouncementNotFound
	}
	return s.item, nil
}

func (s *announcementRepoStub) Update(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (*announcementRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (*announcementRepoStub) List(context.Context, pagination.PaginationParams, AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (s *announcementRepoStub) ListActive(context.Context, time.Time) ([]Announcement, error) {
	return s.activeItems, nil
}

type announcementReadRepoStub struct {
	AnnouncementReadRepository
	reads map[int64]time.Time
	marks []announcementReadMark
}

type announcementKeyReadRepoStub struct {
	AnnouncementKeyReadRepository
	readsByKey map[int64]map[int64]time.Time
	marks      []announcementKeyReadMark
}

type announcementReadMark struct {
	AnnouncementID int64
	UserID         int64
}

type announcementKeyReadMark struct {
	AnnouncementID int64
	APIKeyID       int64
}

func (s *announcementReadRepoStub) MarkRead(_ context.Context, announcementID, userID int64, _ time.Time) error {
	s.marks = append(s.marks, announcementReadMark{AnnouncementID: announcementID, UserID: userID})
	return nil
}

func (s *announcementReadRepoStub) GetReadMapByUser(_ context.Context, _ int64, _ []int64) (map[int64]time.Time, error) {
	return s.reads, nil
}

func (s *announcementKeyReadRepoStub) MarkRead(_ context.Context, announcementID, apiKeyID int64, _ time.Time) error {
	s.marks = append(s.marks, announcementKeyReadMark{AnnouncementID: announcementID, APIKeyID: apiKeyID})
	return nil
}

func (s *announcementKeyReadRepoStub) GetReadMapByAPIKey(_ context.Context, apiKeyID int64, _ []int64) (map[int64]time.Time, error) {
	if s.readsByKey == nil {
		return map[int64]time.Time{}, nil
	}
	if reads, ok := s.readsByKey[apiKeyID]; ok {
		return reads, nil
	}
	return map[int64]time.Time{}, nil
}

type announcementUserRepoStub struct {
	UserRepository
	users map[int64]*User
}

func (s *announcementUserRepoStub) GetByID(_ context.Context, userID int64) (*User, error) {
	if user, ok := s.users[userID]; ok {
		return user, nil
	}
	return nil, ErrUserNotFound
}

type announcementUserSubRepoStub struct {
	UserSubscriptionRepository
	subsByUser map[int64][]UserSubscription
}

func (s *announcementUserSubRepoStub) ListActiveByUserID(_ context.Context, userID int64) ([]UserSubscription, error) {
	return s.subsByUser[userID], nil
}

func TestAnnouncementServiceCreateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil, nil)
	now := time.Unix(1776790020, 0)

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "公告",
		Content:    "内容",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		StartsAt:   &now,
		EndsAt:     &now,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceUpdateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{
		item: &Announcement{
			ID:         1,
			Title:      "公告",
			Content:    "内容",
			Status:     AnnouncementStatusActive,
			NotifyMode: AnnouncementNotifyModePopup,
		},
	}
	svc := NewAnnouncementService(repo, nil, nil, nil, nil)
	now := time.Unix(1776790020, 0)
	startsAt := &now
	endsAt := &now

	_, err := svc.Update(context.Background(), 1, &UpdateAnnouncementInput{
		StartsAt: &startsAt,
		EndsAt:   &endsAt,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceKeyReadsAreIsolatedByAPIKey(t *testing.T) {
	readAt := time.Unix(1776790020, 0)
	svc := newAnnouncementServiceReadFixture(
		[]Announcement{{ID: 10, Title: "A", Content: "content", Status: AnnouncementStatusActive, NotifyMode: AnnouncementNotifyModePopup}},
		map[int64]map[int64]time.Time{
			101: {10: readAt},
		},
		map[int64]time.Time{},
	)

	keyOneItems, err := svc.ListForAPIKey(context.Background(), &APIKey{ID: 101, UserID: 1, Status: StatusAPIKeyDisabled}, false)
	require.NoError(t, err)
	require.Len(t, keyOneItems, 1)
	require.NotNil(t, keyOneItems[0].ReadAt)

	keyTwoItems, err := svc.ListForAPIKey(context.Background(), &APIKey{ID: 102, UserID: 1, Status: StatusAPIKeyExpired}, false)
	require.NoError(t, err)
	require.Len(t, keyTwoItems, 1)
	require.Nil(t, keyTwoItems[0].ReadAt)
}

func TestAnnouncementServiceUserAndKeyReadsAreIsolated(t *testing.T) {
	readAt := time.Unix(1776790020, 0)
	svc := newAnnouncementServiceReadFixture(
		[]Announcement{{ID: 10, Title: "A", Content: "content", Status: AnnouncementStatusActive, NotifyMode: AnnouncementNotifyModePopup}},
		map[int64]map[int64]time.Time{},
		map[int64]time.Time{10: readAt},
	)

	userItems, err := svc.ListForUser(context.Background(), 1, false)
	require.NoError(t, err)
	require.Len(t, userItems, 1)
	require.NotNil(t, userItems[0].ReadAt)

	keyItems, err := svc.ListForAPIKey(context.Background(), &APIKey{ID: 101, UserID: 1}, false)
	require.NoError(t, err)
	require.Len(t, keyItems, 1)
	require.Nil(t, keyItems[0].ReadAt)
}

func TestAnnouncementServiceKeyVisibilityUsesOwnerBalanceAndSubscriptions(t *testing.T) {
	svc := newAnnouncementServiceReadFixture(
		[]Announcement{
			{
				ID:         10,
				Title:      "visible",
				Content:    "content",
				Status:     AnnouncementStatusActive,
				NotifyMode: AnnouncementNotifyModePopup,
				Targeting: AnnouncementTargeting{AnyOf: []AnnouncementConditionGroup{{
					AllOf: []AnnouncementCondition{
						{Type: AnnouncementConditionTypeBalance, Operator: AnnouncementOperatorGTE, Value: 100},
						{Type: AnnouncementConditionTypeSubscription, Operator: AnnouncementOperatorIn, GroupIDs: []int64{7}},
					},
				}}},
			},
			{
				ID:         11,
				Title:      "hidden",
				Content:    "content",
				Status:     AnnouncementStatusActive,
				NotifyMode: AnnouncementNotifyModePopup,
				Targeting: AnnouncementTargeting{AnyOf: []AnnouncementConditionGroup{{
					AllOf: []AnnouncementCondition{{Type: AnnouncementConditionTypeSubscription, Operator: AnnouncementOperatorIn, GroupIDs: []int64{8}}},
				}}},
			},
		},
		nil,
		nil,
	)

	items, err := svc.ListForAPIKey(context.Background(), &APIKey{ID: 101, UserID: 1, GroupID: announcementInt64Ptr(8)}, false)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(10), items[0].Announcement.ID)
}

func TestAnnouncementServiceMarkReadForAPIKeyGuardsInactiveExpiredAndHidden(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	expiredEnd := now.Add(-time.Minute)
	futureStart := now.Add(time.Hour)
	tests := []struct {
		name string
		ann  Announcement
	}{
		{
			name: "draft",
			ann:  Announcement{ID: 10, Title: "draft", Content: "content", Status: AnnouncementStatusDraft, NotifyMode: AnnouncementNotifyModePopup},
		},
		{
			name: "expired",
			ann:  Announcement{ID: 10, Title: "expired", Content: "content", Status: AnnouncementStatusActive, NotifyMode: AnnouncementNotifyModePopup, EndsAt: &expiredEnd},
		},
		{
			name: "not started",
			ann:  Announcement{ID: 10, Title: "future", Content: "content", Status: AnnouncementStatusActive, NotifyMode: AnnouncementNotifyModePopup, StartsAt: &futureStart},
		},
		{
			name: "target hidden",
			ann: Announcement{
				ID:         10,
				Title:      "hidden",
				Content:    "content",
				Status:     AnnouncementStatusActive,
				NotifyMode: AnnouncementNotifyModePopup,
				StartsAt:   &past,
				Targeting: AnnouncementTargeting{AnyOf: []AnnouncementConditionGroup{{
					AllOf: []AnnouncementCondition{{Type: AnnouncementConditionTypeBalance, Operator: AnnouncementOperatorGT, Value: 200}},
				}}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyReadRepo := &announcementKeyReadRepoStub{}
			svc := newAnnouncementServiceReadFixtureWithKeyRepo([]Announcement{tt.ann}, nil, nil, keyReadRepo)
			err := svc.MarkReadForAPIKey(context.Background(), &APIKey{ID: 101, UserID: 1}, tt.ann.ID)
			require.ErrorIs(t, err, ErrAnnouncementNotFound)
			require.Empty(t, keyReadRepo.marks)
		})
	}
}

func TestAnnouncementServiceMarkReadForAPIKeyWritesKeyOnly(t *testing.T) {
	keyReadRepo := &announcementKeyReadRepoStub{}
	userReadRepo := &announcementReadRepoStub{}
	svc := newAnnouncementServiceReadFixtureWithRepos(
		[]Announcement{{ID: 10, Title: "visible", Content: "content", Status: AnnouncementStatusActive, NotifyMode: AnnouncementNotifyModePopup}},
		map[int64]map[int64]time.Time{},
		map[int64]time.Time{},
		userReadRepo,
		keyReadRepo,
	)

	err := svc.MarkReadForAPIKey(context.Background(), &APIKey{ID: 101, UserID: 1}, 10)
	require.NoError(t, err)
	require.Empty(t, userReadRepo.marks)
	require.Equal(t, []announcementKeyReadMark{{AnnouncementID: 10, APIKeyID: 101}}, keyReadRepo.marks)
}

func newAnnouncementServiceReadFixture(active []Announcement, keyReads map[int64]map[int64]time.Time, userReads map[int64]time.Time) *AnnouncementService {
	return newAnnouncementServiceReadFixtureWithKeyRepo(active, keyReads, userReads, &announcementKeyReadRepoStub{})
}

func newAnnouncementServiceReadFixtureWithKeyRepo(active []Announcement, keyReads map[int64]map[int64]time.Time, userReads map[int64]time.Time, keyReadRepo *announcementKeyReadRepoStub) *AnnouncementService {
	return newAnnouncementServiceReadFixtureWithRepos(active, keyReads, userReads, &announcementReadRepoStub{}, keyReadRepo)
}

func newAnnouncementServiceReadFixtureWithRepos(active []Announcement, keyReads map[int64]map[int64]time.Time, userReads map[int64]time.Time, userReadRepo *announcementReadRepoStub, keyReadRepo *announcementKeyReadRepoStub) *AnnouncementService {
	repo := &announcementRepoStub{activeItems: active}
	if len(active) > 0 {
		repo.item = &active[0]
	}
	userReadRepo.reads = userReads
	keyReadRepo.readsByKey = keyReads
	return NewAnnouncementService(
		repo,
		userReadRepo,
		keyReadRepo,
		&announcementUserRepoStub{users: map[int64]*User{1: {ID: 1, Balance: 150}}},
		&announcementUserSubRepoStub{subsByUser: map[int64][]UserSubscription{1: {{UserID: 1, GroupID: 7, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)}}}},
	)
}

func announcementInt64Ptr(v int64) *int64 {
	return &v
}
