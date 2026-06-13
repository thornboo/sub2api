package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildBatchAPIKeyNames_TemplatePadsFromOne(t *testing.T) {
	tmpl := "member-{seq}"
	names, err := buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{
		Count:        12,
		NameTemplate: &tmpl,
	})
	if err != nil {
		t.Fatalf("buildBatchAPIKeyNames returned error: %v", err)
	}
	if len(names) != 12 {
		t.Fatalf("len(names) = %d, want 12", len(names))
	}
	if names[0] != "member-001" {
		t.Fatalf("first name = %q, want member-001", names[0])
	}
	if names[11] != "member-012" {
		t.Fatalf("last name = %q, want member-012", names[11])
	}
}

func TestBuildBatchAPIKeyNames_RequiresExactlyOneNameMode(t *testing.T) {
	tmpl := "member-{seq}"
	_, err := buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{
		Count:        2,
		NameTemplate: &tmpl,
		Names:        []string{"alice", "bob"},
	})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("error = %v, want ErrAPIKeyBatchInvalid", err)
	}

	_, err = buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("error = %v, want ErrAPIKeyBatchInvalid", err)
	}
}

func TestBuildBatchAPIKeyNames_RejectsTemplateWithoutSeq(t *testing.T) {
	tmpl := "member"
	_, err := buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{
		Count:        2,
		NameTemplate: &tmpl,
	})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("error = %v, want ErrAPIKeyBatchInvalid", err)
	}
}

func TestBuildBatchAPIKeyNames_RejectsNamesCountMismatch(t *testing.T) {
	_, err := buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{
		Count: 3,
		Names: []string{"alice", "bob"},
	})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("error = %v, want ErrAPIKeyBatchInvalid", err)
	}
}

func TestBuildBatchAPIKeyNames_RejectsDuplicateNames(t *testing.T) {
	_, err := buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{
		Count: 2,
		Names: []string{"alice", " alice "},
	})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("error = %v, want ErrAPIKeyBatchInvalid", err)
	}
}

func TestBuildBatchAPIKeyNames_RejectsOverlongGeneratedName(t *testing.T) {
	tmpl := "member-" + strings.Repeat("x", apiKeyNameMaxLength) + "-{seq}"
	_, err := buildBatchAPIKeyNames(BatchCreateAPIKeysRequest{
		Count:        1,
		NameTemplate: &tmpl,
	})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("error = %v, want ErrAPIKeyBatchInvalid", err)
	}
}

func TestAPIKeyServiceBatchCreate_RejectsDefaultMaxCount(t *testing.T) {
	tmpl := "member-{seq}"
	svc := &APIKeyService{}

	_, err := svc.BatchCreate(context.Background(), 42, BatchCreateAPIKeysRequest{
		Count:        DefaultAPIKeyBatchCreateMaxCount + 1,
		NameTemplate: &tmpl,
	})
	if !errors.Is(err, ErrAPIKeyBatchTooLarge) {
		t.Fatalf("BatchCreate error = %v, want ErrAPIKeyBatchTooLarge", err)
	}
}

type batchCreateUserRepoStub struct {
	UserRepository
	user *User
	err  error
}

func (s batchCreateUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, s.err
}

type batchCreateGroupRepoStub struct {
	GroupRepository
	group *Group
	err   error
}

func (s batchCreateGroupRepoStub) GetByID(context.Context, int64) (*Group, error) {
	return s.group, s.err
}

type batchCreateAPIKeyRepoStub struct {
	APIKeyRepository
	created     []APIKey
	keysByID    map[int64]APIKey
	updated     []APIKey
	deleted     []int64
	createErrAt int
	updateErrAt int
	deleteErrAt int
	txCalls     int
}

func cloneAPIKeyMap(in map[int64]APIKey) map[int64]APIKey {
	if in == nil {
		return nil
	}
	out := make(map[int64]APIKey, len(in))
	for id, key := range in {
		out[id] = key
	}
	return out
}

func (s *batchCreateAPIKeyRepoStub) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	s.txCalls++
	before := len(s.created)
	updatedBefore := len(s.updated)
	deletedBefore := len(s.deleted)
	keysBefore := cloneAPIKeyMap(s.keysByID)
	err := fn(ctx)
	if err != nil {
		s.created = s.created[:before]
		s.updated = s.updated[:updatedBefore]
		s.deleted = s.deleted[:deletedBefore]
		s.keysByID = keysBefore
		return err
	}
	return nil
}

func (s *batchCreateAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	if s.createErrAt > 0 && len(s.created)+1 == s.createErrAt {
		return errors.New("insert failed")
	}
	created := *key
	created.ID = int64(len(s.created) + 1)
	s.created = append(s.created, created)
	key.ID = created.ID
	return nil
}

func (s *batchCreateAPIKeyRepoStub) ListByIDsForUser(_ context.Context, userID int64, ids []int64) ([]APIKey, error) {
	keys := make([]APIKey, 0, len(ids))
	for _, id := range ids {
		key, ok := s.keysByID[id]
		if ok && key.UserID == userID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func (s *batchCreateAPIKeyRepoStub) Update(_ context.Context, key *APIKey) error {
	if s.updateErrAt > 0 && len(s.updated)+1 == s.updateErrAt {
		return errors.New("update failed")
	}
	updated := *key
	s.updated = append(s.updated, updated)
	if s.keysByID != nil {
		s.keysByID[key.ID] = updated
	}
	return nil
}

func (s *batchCreateAPIKeyRepoStub) DeleteWithAudit(_ context.Context, id int64) error {
	if s.deleteErrAt > 0 && len(s.deleted)+1 == s.deleteErrAt {
		return errors.New("delete failed")
	}
	s.deleted = append(s.deleted, id)
	return nil
}

func TestAPIKeyServiceBatchCreate_RollsBackWholeBatchOnCreateFailure(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{createErrAt: 2}
	svc := NewAPIKeyService(
		repo,
		batchCreateUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		nil,
		nil,
		nil,
		nil,
		nil,
	)

	_, err := svc.BatchCreate(context.Background(), 42, BatchCreateAPIKeysRequest{
		Count: 3,
		Names: []string{"alice", "bob", "carol"},
	})
	if err == nil {
		t.Fatal("BatchCreate returned nil error, want create failure")
	}
	if repo.txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", repo.txCalls)
	}
	if len(repo.created) != 0 {
		t.Fatalf("created keys after rollback = %d, want 0", len(repo.created))
	}
}

func TestAPIKeyServiceBatchCreate_RejectsForbiddenGroupBeforeTransaction(t *testing.T) {
	groupID := int64(9)
	repo := &batchCreateAPIKeyRepoStub{}
	svc := NewAPIKeyService(
		repo,
		batchCreateUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		batchCreateGroupRepoStub{group: &Group{ID: groupID, IsExclusive: true, Status: StatusActive}},
		nil,
		nil,
		nil,
		nil,
	)

	_, err := svc.BatchCreate(context.Background(), 42, BatchCreateAPIKeysRequest{
		Count:   1,
		Names:   []string{"alice"},
		GroupID: &groupID,
	})
	if !errors.Is(err, ErrGroupNotAllowed) {
		t.Fatalf("BatchCreate error = %v, want ErrGroupNotAllowed", err)
	}
	if repo.txCalls != 0 {
		t.Fatalf("txCalls = %d, want 0", repo.txCalls)
	}
}

func TestAPIKeyServiceBatchUpdate_RejectsForbiddenKeyBeforeTransaction(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:         []int64{1, 99},
		UpdateQuota: true,
		QuotaMode:   APIKeyBatchQuotaModeSet,
		QuotaValue:  10,
	})
	if !errors.Is(err, ErrInsufficientPerms) {
		t.Fatalf("BatchUpdate error = %v, want ErrInsufficientPerms", err)
	}
	if repo.txCalls != 0 {
		t.Fatalf("txCalls = %d, want 0", repo.txCalls)
	}
}

func TestAPIKeyServiceBatchUpdate_AddQuotaDedupesIDs(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive, Quota: 10},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	result, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:         []int64{1, 1},
		UpdateQuota: true,
		QuotaMode:   APIKeyBatchQuotaModeAdd,
		QuotaValue:  5,
	})
	if err != nil {
		t.Fatalf("BatchUpdate returned error: %v", err)
	}
	if result.Updated != 1 {
		t.Fatalf("Updated = %d, want 1", result.Updated)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("len(updated) = %d, want 1", len(repo.updated))
	}
	if repo.updated[0].Quota != 15 {
		t.Fatalf("updated quota = %v, want 15", repo.updated[0].Quota)
	}
}

func TestNormalizeAPIKeyTags_LowercasesDedupesAndRejectsLimits(t *testing.T) {
	got, err := normalizeAPIKeyTags([]string{" Team-A ", "team-a", "Project-X"})
	if err != nil {
		t.Fatalf("normalizeAPIKeyTags returned error: %v", err)
	}
	want := []string{"team-a", "project-x"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}

	_, err = normalizeAPIKeyTags([]string{strings.Repeat("x", APIKeyTagMaxLength+1)})
	if !errors.Is(err, ErrAPIKeyTagsInvalid) {
		t.Fatalf("overlong tag error = %v, want ErrAPIKeyTagsInvalid", err)
	}

	tooMany := make([]string, DefaultAPIKeyTagsMaxCount+1)
	for i := range tooMany {
		tooMany[i] = "tag-" + string(rune('a'+i))
	}
	_, err = normalizeAPIKeyTags(tooMany)
	if !errors.Is(err, ErrAPIKeyTagsInvalid) {
		t.Fatalf("too many tags error = %v, want ErrAPIKeyTagsInvalid", err)
	}
}

func TestAPIKeyServiceBatchUpdate_TagModes(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive, Tags: []string{"existing", "legacy"}},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:        []int64{1},
		UpdateTags: true,
		TagsMode:   APIKeyBatchTagsModeAdd,
		Tags:       []string{"Project-X", "existing"},
	})
	if err != nil {
		t.Fatalf("BatchUpdate add tags returned error: %v", err)
	}
	if got, want := repo.keysByID[1].Tags, []string{"existing", "legacy", "project-x"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("add tags = %#v, want %#v", got, want)
	}

	_, err = svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:        []int64{1},
		UpdateTags: true,
		TagsMode:   APIKeyBatchTagsModeRemove,
		Tags:       []string{"legacy"},
	})
	if err != nil {
		t.Fatalf("BatchUpdate remove tags returned error: %v", err)
	}
	if got, want := repo.keysByID[1].Tags, []string{"existing", "project-x"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("remove tags = %#v, want %#v", got, want)
	}

	_, err = svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:        []int64{1},
		UpdateTags: true,
		TagsMode:   APIKeyBatchTagsModeClear,
	})
	if err != nil {
		t.Fatalf("BatchUpdate clear tags returned error: %v", err)
	}
	if got, want := repo.keysByID[1].Tags, []string{}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clear tags = %#v, want %#v", got, want)
	}

	_, err = svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:        []int64{1},
		UpdateTags: true,
		TagsMode:   APIKeyBatchTagsModeSet,
		Tags:       []string{"Beta", "Alpha"},
	})
	if err != nil {
		t.Fatalf("BatchUpdate set tags returned error: %v", err)
	}
	if got, want := repo.keysByID[1].Tags, []string{"beta", "alpha"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("set tags = %#v, want %#v", got, want)
	}
}

func TestAPIKeyServiceBatchUpdate_RollsBackWholeBatchOnUpdateFailure(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive},
			2: {ID: 2, UserID: 42, Key: "sk-2", Status: StatusActive},
		},
		updateErrAt: 2,
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		IDs:          []int64{1, 2},
		UpdateStatus: true,
		Status:       "inactive",
	})
	if err == nil {
		t.Fatal("BatchUpdate returned nil error, want update failure")
	}
	if repo.txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", repo.txCalls)
	}
	if len(repo.updated) != 0 {
		t.Fatalf("updated keys after rollback = %d, want 0", len(repo.updated))
	}
}

func TestAPIKeyServiceBatchDelete_RejectsForbiddenKeyBeforeTransaction(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchDelete(context.Background(), 42, BatchDeleteAPIKeysRequest{IDs: []int64{1, 99}})
	if !errors.Is(err, ErrInsufficientPerms) {
		t.Fatalf("BatchDelete error = %v, want ErrInsufficientPerms", err)
	}
	if repo.txCalls != 0 {
		t.Fatalf("txCalls = %d, want 0", repo.txCalls)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("deleted keys = %d, want 0", len(repo.deleted))
	}
}

func TestAPIKeyServiceBatchDelete_RollsBackWholeBatchOnDeleteFailure(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive},
			2: {ID: 2, UserID: 42, Key: "sk-2", Status: StatusActive},
		},
		deleteErrAt: 2,
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchDelete(context.Background(), 42, BatchDeleteAPIKeysRequest{IDs: []int64{1, 2}})
	if err == nil {
		t.Fatal("BatchDelete returned nil error, want delete failure")
	}
	if repo.txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", repo.txCalls)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("deleted keys after rollback = %d, want 0", len(repo.deleted))
	}
}

func TestAPIKeyServiceClaimPublicStatusLookup_RateLimitsSameKey(t *testing.T) {
	svc := &APIKeyService{}
	ctx := context.Background()

	if err := svc.claimPublicStatusLookup(ctx, "sk-test"); err != nil {
		t.Fatalf("first claim returned error: %v", err)
	}
	err := svc.claimPublicStatusLookup(ctx, "sk-test")
	if !errors.Is(err, ErrAPIKeyStatusLookupRateLimited) {
		t.Fatalf("second claim error = %v, want ErrAPIKeyStatusLookupRateLimited", err)
	}
}

type statusLookupCooldownCacheStub struct {
	APIKeyCache
	allowed bool
	err     error
}

func (s statusLookupCooldownCacheStub) ClaimStatusLookupCooldown(context.Context, string, time.Duration) (bool, error) {
	return s.allowed, s.err
}

func TestAPIKeyServiceClaimPublicStatusLookup_FailClosedWhenCacheErrors(t *testing.T) {
	svc := &APIKeyService{
		cache: statusLookupCooldownCacheStub{
			err: errors.New("redis unavailable"),
		},
	}

	err := svc.claimPublicStatusLookup(context.Background(), "sk-test")
	if !errors.Is(err, ErrAPIKeyStatusLookupUnavailable) {
		t.Fatalf("claim error = %v, want ErrAPIKeyStatusLookupUnavailable", err)
	}
}
