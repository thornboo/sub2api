package repository

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/lib/pq"
)

const postgresDeadlockMaxRetries = 2

func isPostgresDeadlock(err error) bool {
	if err == nil {
		return false
	}
	var postgresErr *pq.Error
	return errors.As(err, &postgresErr) && postgresErr.Code == "40P01"
}

// withPostgresDeadlockRetry retries an operation that owns its complete
// transaction lifecycle. Callers must begin a new transaction on every
// invocation; a PostgreSQL transaction cannot be reused after a deadlock.
func withPostgresDeadlockRetry[T any](ctx context.Context, operationName string, operation func() (T, error)) (T, error) {
	return withPostgresDeadlockRetryConfig(ctx, operationName, postgresDeadlockMaxRetries, postgresDeadlockRetryDelay, operation)
}

func withPostgresDeadlockRetryConfig[T any](
	ctx context.Context,
	operationName string,
	maxRetries int,
	retryDelay func(int) time.Duration,
	operation func() (T, error),
) (T, error) {
	for attempt := 0; ; attempt++ {
		result, err := operation()
		if err == nil || !isPostgresDeadlock(err) {
			return result, err
		}
		if attempt >= maxRetries {
			logger.LegacyPrintf("repository.postgres", "postgres deadlock retry exhausted: operation=%s attempts=%d", operationName, attempt+1)
			var zero T
			return zero, err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			var zero T
			return zero, ctxErr
		}

		retry := attempt + 1
		logger.LegacyPrintf("repository.postgres", "retrying postgres transaction after deadlock: operation=%s retry=%d/%d", operationName, retry, maxRetries)
		delay := retryDelay(retry)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			// Timer channels are synchronous on the repository's Go 1.23+
			// toolchain, so Stop must not be followed by the pre-1.23 drain
			// pattern: that receive can block after Stop returns.
			timer.Stop()
			var zero T
			return zero, ctx.Err()
		case <-timer.C:
		}
	}
}

func postgresDeadlockRetryDelay(retry int) time.Duration {
	base := 20 * time.Millisecond * time.Duration(1<<max(retry-1, 0))
	return base + time.Duration(rand.Int64N(int64(base)))
}
