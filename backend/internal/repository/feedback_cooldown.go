package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const feedbackCooldownPrefix = "feedback:cooldown:"

type feedbackCooldownStore struct {
	rdb *redis.Client
}

func NewFeedbackCooldownStore(rdb *redis.Client) service.FeedbackCooldownStore {
	return &feedbackCooldownStore{rdb: rdb}
}

func (s *feedbackCooldownStore) ClaimFeedbackCooldown(ctx context.Context, identity string, ttl time.Duration) (bool, time.Duration, error) {
	if s == nil || s.rdb == nil {
		return false, 0, errors.New("nil feedback cooldown store")
	}
	if identity == "" || ttl <= 0 {
		return false, 0, errors.New("invalid feedback cooldown")
	}
	key := feedbackCooldownPrefix + identity
	ok, err := s.rdb.SetNX(ctx, key, "1", ttl).Result()
	if err != nil {
		return false, 0, fmt.Errorf("claim feedback cooldown: %w", err)
	}
	if ok {
		return true, 0, nil
	}
	remaining, err := s.rdb.PTTL(ctx, key).Result()
	if err != nil {
		return false, 0, fmt.Errorf("read feedback cooldown ttl: %w", err)
	}
	if remaining <= 0 {
		remaining = ttl
	}
	return false, remaining, nil
}
