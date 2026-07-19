package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type publicKeyUsageRepoStub struct {
	APIKeyRepository
	byKey map[string]*APIKey
	byID  map[int64]*APIKey
}

func (s *publicKeyUsageRepoStub) GetByKey(_ context.Context, key string) (*APIKey, error) {
	value := s.byKey[key]
	if value == nil {
		return nil, ErrAPIKeyNotFound
	}
	copy := *value
	return &copy, nil
}

func (s *publicKeyUsageRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	value := s.byID[id]
	if value == nil {
		return nil, ErrAPIKeyNotFound
	}
	copy := *value
	return &copy, nil
}

type publicKeyUsageCacheStub struct {
	APIKeyCache
	sessions map[string]PublicKeyUsageSession
	lastHash string
	lastTTL  time.Duration
	getErr   error
}

func (s *publicKeyUsageCacheStub) ClaimStatusLookupCooldown(context.Context, string, time.Duration) (bool, error) {
	return true, nil
}

func (s *publicKeyUsageCacheStub) CreatePublicKeyUsageSession(_ context.Context, tokenHash string, session *PublicKeyUsageSession, ttl time.Duration) error {
	s.lastHash = tokenHash
	s.lastTTL = ttl
	s.sessions[tokenHash] = *session
	return nil
}

func (s *publicKeyUsageCacheStub) GetPublicKeyUsageSession(_ context.Context, tokenHash string) (*PublicKeyUsageSession, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	value, ok := s.sessions[tokenHash]
	if !ok {
		return nil, ErrPublicKeyUsageSessionInvalid
	}
	return &value, nil
}

func (s *publicKeyUsageCacheStub) RefreshPublicKeyUsageSession(_ context.Context, tokenHash string, ttl time.Duration) error {
	if _, ok := s.sessions[tokenHash]; !ok {
		return ErrPublicKeyUsageSessionInvalid
	}
	s.lastTTL = ttl
	return nil
}

func (s *publicKeyUsageCacheStub) DeletePublicKeyUsageSession(_ context.Context, tokenHash string) error {
	delete(s.sessions, tokenHash)
	return nil
}

func newPublicKeyUsageSessionService(key *APIKey) (*APIKeyService, *publicKeyUsageCacheStub) {
	repo := &publicKeyUsageRepoStub{
		byKey: map[string]*APIKey{key.Key: key},
		byID:  map[int64]*APIKey{key.ID: key},
	}
	cache := &publicKeyUsageCacheStub{sessions: make(map[string]PublicKeyUsageSession)}
	return NewAPIKeyService(repo, nil, nil, nil, nil, cache, nil), cache
}

func TestPublicKeyUsageSessionDoesNotSerializeTokenOrAuthority(t *testing.T) {
	memberID := int64(9)
	key := &APIKey{ID: 7, UserID: 3, MemberID: &memberID, Key: "sk-public-session-test", Name: "test", Status: StatusActive}
	svc, cache := newPublicKeyUsageSessionService(key)

	created, err := svc.CreatePublicKeyUsageSession(context.Background(), key.Key)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if len(created.Token) != 64 {
		t.Fatalf("token hex length = %d, want 64", len(created.Token))
	}
	if cache.lastHash == "" || cache.lastHash == created.Token || cache.lastHash == key.Key {
		t.Fatalf("cache key must contain only a token hash")
	}
	if cache.lastTTL != PublicKeyUsageSessionIdleTTL {
		t.Fatalf("cache ttl = %v, want %v", cache.lastTTL, PublicKeyUsageSessionIdleTTL)
	}
	payload, err := json.Marshal(created)
	if err != nil {
		t.Fatalf("marshal created response: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{created.Token, key.Key, "api_key_id", "user_id", "member_id"} {
		if forbidden != "" && strings.Contains(text, forbidden) {
			t.Fatalf("serialized session contains forbidden value %q: %s", forbidden, text)
		}
	}
}

func TestPublicKeyUsageSessionInvalidatesWhenAuthorityChanges(t *testing.T) {
	memberID := int64(9)
	key := &APIKey{ID: 7, UserID: 3, MemberID: &memberID, Key: "sk-public-session-test", Name: "test", Status: StatusActive}
	svc, _ := newPublicKeyUsageSessionService(key)
	created, err := svc.CreatePublicKeyUsageSession(context.Background(), key.Key)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	changedMemberID := int64(10)
	key.MemberID = &changedMemberID
	_, _, _, err = svc.ResolvePublicKeyUsageSession(context.Background(), created.Token)
	if !errors.Is(err, ErrPublicKeyUsageSessionInvalid) {
		t.Fatalf("resolve after member change = %v, want invalid session", err)
	}
}

func TestPublicKeyUsageSessionSeparatesMissingFromCacheFailure(t *testing.T) {
	key := &APIKey{ID: 7, UserID: 3, Key: "sk-public-session-test", Name: "test", Status: StatusActive}
	svc, cache := newPublicKeyUsageSessionService(key)

	_, _, _, err := svc.ResolvePublicKeyUsageSession(context.Background(), "missing")
	if !errors.Is(err, ErrPublicKeyUsageSessionInvalid) {
		t.Fatalf("missing session = %v, want invalid", err)
	}

	cache.getErr = errors.New("redis unavailable")
	_, _, _, err = svc.ResolvePublicKeyUsageSession(context.Background(), "any-token")
	if !errors.Is(err, ErrPublicKeyUsageSessionUnavailable) {
		t.Fatalf("cache failure = %v, want unavailable", err)
	}
}

func TestPublicKeyUsageSessionRefreshNeverExtendsAbsoluteExpiry(t *testing.T) {
	key := &APIKey{ID: 7, UserID: 3, Key: "sk-public-session-test", Name: "test", Status: StatusActive}
	svc, cache := newPublicKeyUsageSessionService(key)
	created, err := svc.CreatePublicKeyUsageSession(context.Background(), key.Key)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	tokenHash := publicKeyUsageSessionHash(created.Token)
	session := cache.sessions[tokenHash]
	session.AbsoluteExpiresAt = time.Now().UTC().Add(2 * time.Minute)
	cache.sessions[tokenHash] = session

	_, _, expiresAt, err := svc.ResolvePublicKeyUsageSession(context.Background(), created.Token)
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}
	if cache.lastTTL <= 0 || cache.lastTTL > 2*time.Minute {
		t.Fatalf("refreshed ttl = %v, want at most remaining absolute lifetime", cache.lastTTL)
	}
	if expiresAt.After(session.AbsoluteExpiresAt) {
		t.Fatalf("reported expiry %v exceeds absolute expiry %v", expiresAt, session.AbsoluteExpiresAt)
	}
}

func TestPublicKeyUsageSessionDeleteRevokesToken(t *testing.T) {
	key := &APIKey{ID: 7, UserID: 3, Key: "sk-public-session-test", Name: "test", Status: StatusActive}
	svc, _ := newPublicKeyUsageSessionService(key)
	created, err := svc.CreatePublicKeyUsageSession(context.Background(), key.Key)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := svc.DeletePublicKeyUsageSession(context.Background(), created.Token); err != nil {
		t.Fatalf("delete session: %v", err)
	}
	_, _, _, err = svc.ResolvePublicKeyUsageSession(context.Background(), created.Token)
	if !errors.Is(err, ErrPublicKeyUsageSessionInvalid) {
		t.Fatalf("resolve deleted session = %v, want invalid", err)
	}
}
