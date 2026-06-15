package service

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
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
	created         []APIKey
	keysByID        map[int64]APIKey
	updated         []APIKey
	deleted         []int64
	createErrAt     int
	updateErrAt     int
	deleteErrAt     int
	txCalls         int
	listCalls       int
	lastListFilters APIKeyListFilters
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

func (s *batchCreateAPIKeyRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	key, ok := s.keysByID[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return &key, nil
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

func apiKeyMatchesListFilters(key APIKey, filters APIKeyListFilters) bool {
	if filters.Search != "" &&
		!strings.Contains(strings.ToLower(key.Name), strings.ToLower(filters.Search)) &&
		!strings.Contains(strings.ToLower(key.Key), strings.ToLower(filters.Search)) {
		return false
	}
	if filters.Status != "" && key.Status != filters.Status {
		return false
	}
	if filters.GroupID != nil {
		if *filters.GroupID == 0 {
			if key.GroupID != nil {
				return false
			}
		} else if key.GroupID == nil || *key.GroupID != *filters.GroupID {
			return false
		}
	}
	for _, tag := range filters.Tags {
		found := false
		for _, existing := range key.Tags {
			if existing == tag {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (s *batchCreateAPIKeyRepoStub) ListByUserID(_ context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	s.listCalls++
	s.lastListFilters = filters

	ids := make([]int64, 0, len(s.keysByID))
	for id := range s.keysByID {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	keys := make([]APIKey, 0, len(ids))
	for _, id := range ids {
		key := s.keysByID[id]
		if key.UserID == userID && apiKeyMatchesListFilters(key, filters) {
			keys = append(keys, key)
		}
	}

	limit := params.Limit()
	if len(keys) > limit {
		keys = keys[:limit]
	}
	return keys, &pagination.PaginationResult{
		Total:    int64(len(idsMatchingUserAndFilters(s.keysByID, userID, filters))),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func idsMatchingUserAndFilters(keysByID map[int64]APIKey, userID int64, filters APIKeyListFilters) []int64 {
	ids := make([]int64, 0, len(keysByID))
	for id, key := range keysByID {
		if key.UserID == userID && apiKeyMatchesListFilters(key, filters) {
			ids = append(ids, id)
		}
	}
	return ids
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

func TestAPIKeyServiceUpdate_NormalizesLegacyInactiveStatus(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Name: "legacy", Status: StatusActive},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	legacyStatus := "inactive"

	_, err := svc.Update(context.Background(), 1, 42, UpdateAPIKeyRequest{Status: &legacyStatus})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got := repo.keysByID[1].Status; got != StatusAPIKeyDisabled {
		t.Fatalf("status = %q, want %q", got, StatusAPIKeyDisabled)
	}
}

func TestAPIKeyServiceUpdate_RejectsInvalidStatus(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Name: "invalid", Status: StatusActive},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	invalidStatus := "foobar"

	_, err := svc.Update(context.Background(), 1, 42, UpdateAPIKeyRequest{Status: &invalidStatus})
	if !errors.Is(err, ErrAPIKeyStatusInvalid) {
		t.Fatalf("Update error = %v, want ErrAPIKeyStatusInvalid", err)
	}
	if got := repo.keysByID[1].Status; got != StatusActive {
		t.Fatalf("status after rejected update = %q, want %q", got, StatusActive)
	}
}

func TestAPIKeyServiceUpdate_PreservesSystemStatusWhenStatusOmitted(t *testing.T) {
	for _, systemStatus := range []string{StatusAPIKeyQuotaExhausted, StatusAPIKeyExpired} {
		t.Run(systemStatus, func(t *testing.T) {
			repo := &batchCreateAPIKeyRepoStub{
				keysByID: map[int64]APIKey{
					1: {ID: 1, UserID: 42, Key: "sk-1", Name: "system", Status: systemStatus},
				},
			}
			svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
			name := "renamed"

			_, err := svc.Update(context.Background(), 1, 42, UpdateAPIKeyRequest{Name: &name})
			if err != nil {
				t.Fatalf("Update returned error: %v", err)
			}
			if got := repo.keysByID[1].Status; got != systemStatus {
				t.Fatalf("status = %q, want %q", got, systemStatus)
			}
			if got := repo.keysByID[1].Name; got != name {
				t.Fatalf("name = %q, want %q", got, name)
			}
		})
	}
}

func TestAPIKeyServiceUpdate_AllowsExplicitDisabledForSystemStatus(t *testing.T) {
	for _, systemStatus := range []string{StatusAPIKeyQuotaExhausted, StatusAPIKeyExpired} {
		t.Run(systemStatus, func(t *testing.T) {
			repo := &batchCreateAPIKeyRepoStub{
				keysByID: map[int64]APIKey{
					1: {ID: 1, UserID: 42, Key: "sk-1", Name: "system", Status: systemStatus},
				},
			}
			svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
			disabledStatus := StatusAPIKeyDisabled

			_, err := svc.Update(context.Background(), 1, 42, UpdateAPIKeyRequest{Status: &disabledStatus})
			if err != nil {
				t.Fatalf("Update returned error: %v", err)
			}
			if got := repo.keysByID[1].Status; got != StatusAPIKeyDisabled {
				t.Fatalf("status = %q, want %q", got, StatusAPIKeyDisabled)
			}
		})
	}
}

func TestAPIKeyServiceUpdate_SkipsUnchangedGroupRebinding(t *testing.T) {
	groupID := int64(3)
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Name: "legacy", Status: StatusActive, GroupID: &groupID},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
	tags := []string{"team-a"}

	_, err := svc.Update(context.Background(), 1, 42, UpdateAPIKeyRequest{
		GroupID: &groupID,
		Tags:    &tags,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if got, want := repo.keysByID[1].Tags, []string{"team-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %#v, want %#v", got, want)
	}
	if repo.keysByID[1].GroupID == nil || *repo.keysByID[1].GroupID != groupID {
		t.Fatalf("group id = %#v, want %d", repo.keysByID[1].GroupID, groupID)
	}
}

func TestAPIKeyServiceUpdate_RejectsChangedForbiddenGroup(t *testing.T) {
	currentGroupID := int64(3)
	nextGroupID := int64(9)
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Name: "legacy", Status: StatusActive, GroupID: &currentGroupID},
		},
	}
	svc := NewAPIKeyService(
		repo,
		batchCreateUserRepoStub{user: &User{ID: 42, Status: StatusActive}},
		batchCreateGroupRepoStub{group: &Group{ID: nextGroupID, IsExclusive: true, Status: StatusActive}},
		nil,
		nil,
		nil,
		nil,
	)

	_, err := svc.Update(context.Background(), 1, 42, UpdateAPIKeyRequest{GroupID: &nextGroupID})
	if !errors.Is(err, ErrGroupNotAllowed) {
		t.Fatalf("Update error = %v, want ErrGroupNotAllowed", err)
	}
	if repo.keysByID[1].GroupID == nil || *repo.keysByID[1].GroupID != currentGroupID {
		t.Fatalf("group id = %#v, want %d", repo.keysByID[1].GroupID, currentGroupID)
	}
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

func TestAPIKeyServiceBatchUpdate_FilteredScopeUpdatesMatchingOwnedKeys(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Name: "alpha", Status: StatusActive, Tags: []string{"team-a"}},
			2: {ID: 2, UserID: 42, Key: "sk-2", Name: "beta", Status: StatusActive, Tags: []string{"team-b"}},
			3: {ID: 3, UserID: 42, Key: "sk-3", Name: "gamma", Status: StatusAPIKeyDisabled, Tags: []string{"team-a"}},
			4: {ID: 4, UserID: 99, Key: "sk-4", Name: "alpha", Status: StatusActive, Tags: []string{"team-a"}},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	result, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		ApplyTo:      APIKeyBatchApplyToFiltered,
		Filters:      APIKeyBatchFilters{Status: StatusActive, Tags: []string{"Team-A"}},
		UpdateStatus: true,
		Status:       "inactive",
	})
	if err != nil {
		t.Fatalf("BatchUpdate returned error: %v", err)
	}
	if result.Updated != 1 {
		t.Fatalf("Updated = %d, want 1", result.Updated)
	}
	if repo.txCalls != 1 {
		t.Fatalf("txCalls = %d, want 1", repo.txCalls)
	}
	if repo.listCalls != 1 {
		t.Fatalf("listCalls = %d, want 1", repo.listCalls)
	}
	if got, want := repo.lastListFilters.Tags, []string{"team-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("list filter tags = %#v, want %#v", got, want)
	}
	if got := repo.keysByID[1].Status; got != StatusAPIKeyDisabled {
		t.Fatalf("key 1 status = %q, want disabled", got)
	}
	if got := repo.keysByID[2].Status; got != StatusActive {
		t.Fatalf("key 2 status = %q, want active", got)
	}
	if got := repo.keysByID[4].Status; got != StatusActive {
		t.Fatalf("key 4 status = %q, want active", got)
	}
}

func TestAPIKeyServiceBatchUpdate_FilteredScopeNormalizesLegacyInactiveFilter(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Name: "active", Status: StatusActive},
			2: {ID: 2, UserID: 42, Key: "sk-2", Name: "disabled", Status: StatusAPIKeyDisabled},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		ApplyTo:    APIKeyBatchApplyToFiltered,
		Filters:    APIKeyBatchFilters{Status: "inactive"},
		UpdateTags: true,
		TagsMode:   APIKeyBatchTagsModeSet,
		Tags:       []string{"audited"},
	})
	if err != nil {
		t.Fatalf("BatchUpdate returned error: %v", err)
	}
	if got := repo.lastListFilters.Status; got != StatusAPIKeyDisabled {
		t.Fatalf("list filter status = %q, want disabled", got)
	}
	if got, want := repo.keysByID[1].Tags, []string(nil); !reflect.DeepEqual(got, want) {
		t.Fatalf("active key tags = %#v, want %#v", got, want)
	}
	if got, want := repo.keysByID[2].Tags, []string{"audited"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("disabled key tags = %#v, want %#v", got, want)
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

func TestAPIKeyServiceBatchUpdate_FilteredScopeRejectsTooLargeBeforeTransaction(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{keysByID: map[int64]APIKey{}}
	for i := int64(1); i <= HardAPIKeyBatchCreateMaxCount+1; i++ {
		repo.keysByID[i] = APIKey{ID: i, UserID: 42, Key: "sk-many", Status: StatusActive}
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchUpdate(context.Background(), 42, BatchUpdateAPIKeysRequest{
		ApplyTo:      APIKeyBatchApplyToFiltered,
		Filters:      APIKeyBatchFilters{Status: StatusActive},
		UpdateStatus: true,
		Status:       "inactive",
	})
	if !errors.Is(err, ErrAPIKeyBatchTooLarge) {
		t.Fatalf("BatchUpdate error = %v, want ErrAPIKeyBatchTooLarge", err)
	}
	if repo.txCalls != 0 {
		t.Fatalf("txCalls = %d, want 0", repo.txCalls)
	}
	if repo.listCalls != 1 {
		t.Fatalf("listCalls = %d, want 1", repo.listCalls)
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

func TestAPIKeyServiceBatchDelete_FilteredScopeRejectsEmptyFiltersBeforeTransaction(t *testing.T) {
	repo := &batchCreateAPIKeyRepoStub{
		keysByID: map[int64]APIKey{
			1: {ID: 1, UserID: 42, Key: "sk-1", Status: StatusActive},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.BatchDelete(context.Background(), 42, BatchDeleteAPIKeysRequest{ApplyTo: APIKeyBatchApplyToFiltered})
	if !errors.Is(err, ErrAPIKeyBatchInvalid) {
		t.Fatalf("BatchDelete error = %v, want ErrAPIKeyBatchInvalid", err)
	}
	if repo.txCalls != 0 {
		t.Fatalf("txCalls = %d, want 0", repo.txCalls)
	}
	if repo.listCalls != 0 {
		t.Fatalf("listCalls = %d, want 0", repo.listCalls)
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
