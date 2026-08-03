package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestWithPostgresDeadlockRetryRetriesWholeOperation(t *testing.T) {
	attempts := 0
	result, err := withPostgresDeadlockRetryConfig(context.Background(), "test", 2, func(int) time.Duration { return 0 }, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, &pq.Error{Code: "40P01"}
		}
		return 42, nil
	})

	require.NoError(t, err)
	require.Equal(t, 42, result)
	require.Equal(t, 3, attempts)
}

func TestWithPostgresDeadlockRetryStopsAfterRetryLimit(t *testing.T) {
	deadlockErr := &pq.Error{Code: "40P01"}
	attempts := 0
	_, err := withPostgresDeadlockRetryConfig(context.Background(), "test", 2, func(int) time.Duration { return 0 }, func() (int, error) {
		attempts++
		return 0, deadlockErr
	})

	require.Same(t, deadlockErr, err)
	require.Equal(t, 3, attempts)
}

func TestWithPostgresDeadlockRetryDoesNotRetryOtherErrors(t *testing.T) {
	wantErr := errors.New("not retryable")
	attempts := 0
	_, err := withPostgresDeadlockRetryConfig(context.Background(), "test", 2, func(int) time.Duration { return 0 }, func() (int, error) {
		attempts++
		return 0, wantErr
	})

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, attempts)
}

func TestWithPostgresDeadlockRetryHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts := 0
	_, err := withPostgresDeadlockRetryConfig(ctx, "test", 2, func(int) time.Duration { return time.Second }, func() (int, error) {
		attempts++
		return 0, &pq.Error{Code: "40P01"}
	})

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts)
}

func TestWithPostgresDeadlockRetryCancelsDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	delayStarted := make(chan struct{})
	allowDelayReturn := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		_, err := withPostgresDeadlockRetryConfig(ctx, "test", 2, func(int) time.Duration {
			close(delayStarted)
			<-allowDelayReturn
			return time.Hour
		}, func() (int, error) {
			return 0, &pq.Error{Code: "40P01"}
		})
		done <- err
	}()

	<-delayStarted
	cancel()
	close(allowDelayReturn)

	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("deadlock retry did not return after context cancellation")
	}
}

func TestIsPostgresDeadlockRecognizesWrappedPQError(t *testing.T) {
	require.True(t, isPostgresDeadlock(errors.Join(errors.New("wrapped"), &pq.Error{Code: "40P01"})))
	require.False(t, isPostgresDeadlock(&pq.Error{Code: "23505"}))
	require.False(t, isPostgresDeadlock(nil))
}

func TestEnterpriseMemberBudgetReserveRetriesWithNewTransaction(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	requestID := "17:req-deadlock-retry"
	memberID := int64(42)
	payloadHash := "payload-hash"
	expiresAt := time.Date(2026, time.August, 3, 18, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnError(&pq.Error{Code: "40P01"})
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id, request_id, member_id, group_id, request_payload_hash`).
		WithArgs(requestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "request_id", "member_id", "group_id", "request_payload_hash", "period_start",
			"reserved_usd", "actual_usd", "status", "receipt_kind", "async_task_id", "async_task_phase", "usage_log_id", "expires_at",
		}).AddRow(int64(91), requestID, memberID, nil, payloadHash, expiresAt, 0.0, 0.0, "reserved", service.EnterpriseMemberReceiptKindSync, "", "", nil, expiresAt))
	mock.ExpectRollback()

	repo := &enterpriseMemberBudgetRepository{db: db}
	receipt, err := repo.ReserveWithKind(context.Background(), requestID, memberID, nil, payloadHash, 0, service.EnterpriseMemberReceiptKindSync, expiresAt)

	require.NoError(t, err)
	require.Equal(t, int64(91), receipt.ID)
	require.NoError(t, mock.ExpectationsWereMet())
}
