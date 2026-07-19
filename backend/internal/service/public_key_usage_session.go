package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	PublicKeyUsageSessionIdleTTL     = 15 * time.Minute
	PublicKeyUsageSessionAbsoluteTTL = time.Hour
)

var (
	ErrPublicKeyUsageSessionInvalid = infraerrors.Unauthorized(
		"PUBLIC_KEY_USAGE_SESSION_INVALID",
		"key usage session is invalid or expired",
	)
	ErrPublicKeyUsageSessionUnavailable = infraerrors.ServiceUnavailable(
		"PUBLIC_KEY_USAGE_SESSION_UNAVAILABLE",
		"key usage query is temporarily unavailable",
	)
)

// PublicKeyUsageSession is the minimal server-side authority granted to a
// caller that proved possession of one API key. It deliberately contains no
// plaintext key and cannot be widened to sibling keys owned by the same user.
type PublicKeyUsageSession struct {
	APIKeyID          int64     `json:"api_key_id"`
	UserID            int64     `json:"user_id"`
	MemberID          *int64    `json:"member_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	AbsoluteExpiresAt time.Time `json:"absolute_expires_at"`
}

type CreatedPublicKeyUsageSession struct {
	Token     string                `json:"-"`
	Session   PublicKeyUsageSession `json:"-"`
	ExpiresAt time.Time             `json:"expires_at"`
}

type publicKeyUsageSessionCache interface {
	CreatePublicKeyUsageSession(ctx context.Context, tokenHash string, session *PublicKeyUsageSession, ttl time.Duration) error
	GetPublicKeyUsageSession(ctx context.Context, tokenHash string) (*PublicKeyUsageSession, error)
	RefreshPublicKeyUsageSession(ctx context.Context, tokenHash string, ttl time.Duration) error
	DeletePublicKeyUsageSession(ctx context.Context, tokenHash string) error
}

func publicKeyUsageSessionHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newPublicKeyUsageSessionToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func (s *APIKeyService) publicKeyUsageSessionCache() (publicKeyUsageSessionCache, error) {
	if s == nil || s.cache == nil {
		return nil, ErrPublicKeyUsageSessionUnavailable
	}
	cache, ok := s.cache.(publicKeyUsageSessionCache)
	if !ok {
		return nil, ErrPublicKeyUsageSessionUnavailable
	}
	return cache, nil
}

// CreatePublicKeyUsageSession exchanges one plaintext API key for an opaque,
// short-lived server-side session. The plaintext key is never written to the
// session cache or returned to the browser.
func (s *APIKeyService) CreatePublicKeyUsageSession(ctx context.Context, rawKey string) (*CreatedPublicKeyUsageSession, error) {
	rawKey = strings.TrimSpace(rawKey)
	if rawKey == "" {
		return nil, ErrAPIKeyNotFound
	}
	cache, err := s.publicKeyUsageSessionCache()
	if err != nil {
		return nil, err
	}
	status, err := s.GetPublicStatusByKey(ctx, rawKey)
	if err != nil {
		return nil, err
	}
	token, err := newPublicKeyUsageSessionToken()
	if err != nil {
		return nil, ErrPublicKeyUsageSessionUnavailable.WithCause(err)
	}
	now := time.Now().UTC()
	session := PublicKeyUsageSession{
		APIKeyID:          status.ID,
		UserID:            status.UserID,
		MemberID:          status.MemberID,
		CreatedAt:         now,
		AbsoluteExpiresAt: now.Add(PublicKeyUsageSessionAbsoluteTTL),
	}
	if err := cache.CreatePublicKeyUsageSession(ctx, publicKeyUsageSessionHash(token), &session, PublicKeyUsageSessionIdleTTL); err != nil {
		return nil, ErrPublicKeyUsageSessionUnavailable.WithCause(err)
	}
	return &CreatedPublicKeyUsageSession{
		Token:     token,
		Session:   session,
		ExpiresAt: now.Add(PublicKeyUsageSessionIdleTTL),
	}, nil
}

// ResolvePublicKeyUsageSession validates the opaque token, reloads the current
// API-key row, and refreshes only the idle TTL. The absolute expiry can never be
// extended by reads.
func (s *APIKeyService) ResolvePublicKeyUsageSession(ctx context.Context, token string) (*PublicKeyUsageSession, *APIKey, time.Time, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	}
	cache, err := s.publicKeyUsageSessionCache()
	if err != nil {
		return nil, nil, time.Time{}, err
	}
	tokenHash := publicKeyUsageSessionHash(token)
	session, err := cache.GetPublicKeyUsageSession(ctx, tokenHash)
	if errors.Is(err, ErrPublicKeyUsageSessionInvalid) {
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	}
	if err != nil {
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionUnavailable.WithCause(err)
	}
	if session == nil {
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	}
	now := time.Now().UTC()
	if !session.AbsoluteExpiresAt.After(now) {
		_ = cache.DeletePublicKeyUsageSession(ctx, tokenHash)
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	}
	apiKey, err := s.GetByID(ctx, session.APIKeyID)
	if err != nil || apiKey == nil || apiKey.UserID != session.UserID || !sameOptionalInt64(apiKey.MemberID, session.MemberID) {
		_ = cache.DeletePublicKeyUsageSession(ctx, tokenHash)
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	}
	ttl := PublicKeyUsageSessionIdleTTL
	if remaining := session.AbsoluteExpiresAt.Sub(now); remaining < ttl {
		ttl = remaining
	}
	if ttl <= 0 {
		_ = cache.DeletePublicKeyUsageSession(ctx, tokenHash)
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	}
	if err := cache.RefreshPublicKeyUsageSession(ctx, tokenHash, ttl); errors.Is(err, ErrPublicKeyUsageSessionInvalid) {
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionInvalid
	} else if err != nil {
		return nil, nil, time.Time{}, ErrPublicKeyUsageSessionUnavailable.WithCause(err)
	}
	return session, apiKey, now.Add(ttl), nil
}

func sameOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func (s *APIKeyService) DeletePublicKeyUsageSession(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	cache, err := s.publicKeyUsageSessionCache()
	if err != nil {
		return err
	}
	if err := cache.DeletePublicKeyUsageSession(ctx, publicKeyUsageSessionHash(token)); err != nil && !errors.Is(err, ErrPublicKeyUsageSessionInvalid) {
		return ErrPublicKeyUsageSessionUnavailable.WithCause(err)
	}
	return nil
}
