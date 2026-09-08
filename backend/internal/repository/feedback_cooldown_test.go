package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestFeedbackCooldownStoreSetNXAllowsOneConcurrentClaim(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewFeedbackCooldownStore(rdb)

	var wg sync.WaitGroup
	results := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, _, err := store.ClaimFeedbackCooldown(context.Background(), "user:7", time.Minute)
			if err != nil {
				t.Errorf("ClaimFeedbackCooldown: %v", err)
			}
			results <- ok
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for ok := range results {
		if ok {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d, want 1", successes)
	}
	ok, retryAfter, err := store.ClaimFeedbackCooldown(context.Background(), "user:7", time.Minute)
	if err != nil {
		t.Fatalf("repeat claim: %v", err)
	}
	if ok || retryAfter <= 0 || retryAfter > time.Minute {
		t.Fatalf("repeat claim ok=%v retryAfter=%v, want blocked with remaining TTL", ok, retryAfter)
	}
}
