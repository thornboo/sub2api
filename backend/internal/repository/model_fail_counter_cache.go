package repository

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const modelFailCounterPrefix = "model_fail_count:account:"

var modelFailCounterIncrScript = redis.NewScript(`
	local key = KEYS[1]
	local ttl = tonumber(ARGV[1])

	local count = redis.call('INCR', key)
	if count == 1 then
		redis.call('EXPIRE', key, ttl)
	end

	return count
`)

type modelFailCounterCache struct {
	rdb *redis.Client
}

func NewModelFailCounterCache(rdb *redis.Client) service.ModelFailCounterCache {
	return &modelFailCounterCache{rdb: rdb}
}

func (c *modelFailCounterCache) modelFailCounterKey(accountID int64, scope string) string {
	// scope 原样拼入 key（仅构造、从不解析），与 accounts.extra.model_rate_limits 的 key 保持一致。
	return fmt.Sprintf("%s%d:%s", modelFailCounterPrefix, accountID, scope)
}

func (c *modelFailCounterCache) IncrementModelFailCount(ctx context.Context, accountID int64, scope string, windowMinutes int) (int64, error) {
	key := c.modelFailCounterKey(accountID, scope)

	ttlSeconds := windowMinutes * 60
	if ttlSeconds < 60 {
		ttlSeconds = 60
	}

	result, err := modelFailCounterIncrScript.Run(ctx, c.rdb, []string{key}, ttlSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("increment model fail count: %w", err)
	}
	return result, nil
}

func (c *modelFailCounterCache) ResetModelFailCount(ctx context.Context, accountID int64, scope string) error {
	key := c.modelFailCounterKey(accountID, scope)
	return c.rdb.Del(ctx, key).Err()
}
