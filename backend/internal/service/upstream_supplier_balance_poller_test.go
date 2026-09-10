package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUpstreamSupplierBalancePollerClaimsDueSuppliersBounded(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	poller := NewUpstreamSupplierBalancePollerWithOptions(db, nil, nil, time.Minute, 5*time.Minute, 4, 4, 35*24*time.Hour)
	mock.ExpectQuery("WITH due AS").
		WithArgs(4, "300 seconds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)).AddRow(int64(11)))

	ids, err := poller.claimDueSuppliers(context.Background())

	require.NoError(t, err)
	require.Equal(t, []int64{10, 11}, ids)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamSupplierBalancePollerRunOnceDrainsDueBatches(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	poller := NewUpstreamSupplierBalancePollerWithOptions(db, nil, nil, time.Minute, 5*time.Minute, 4, 4, 35*24*time.Hour)
	var mu sync.Mutex
	refreshed := make([]int64, 0)
	poller.refresh = func(_ context.Context, id int64) error {
		mu.Lock()
		defer mu.Unlock()
		refreshed = append(refreshed, id)
		return nil
	}
	mock.ExpectQuery("WITH due AS").
		WithArgs(4, "300 seconds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)).AddRow(int64(2)).AddRow(int64(3)).AddRow(int64(4)))
	mock.ExpectQuery("WITH due AS").
		WithArgs(4, "300 seconds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(5)).AddRow(int64(6)))

	poller.runOnce(context.Background())

	require.ElementsMatch(t, []int64{1, 2, 3, 4, 5, 6}, refreshed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpstreamSupplierBalancePollerStopCancelsLoop(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	poller := NewUpstreamSupplierBalancePollerWithOptions(db, nil, nil, time.Hour, 5*time.Minute, 4, 4, 35*24*time.Hour)
	mock.ExpectQuery("WITH due AS").
		WithArgs(4, "300 seconds").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	poller.Start()
	require.Eventually(t, func() bool {
		return mock.ExpectationsWereMet() == nil
	}, time.Second, 10*time.Millisecond)
	poller.Stop()
	poller.Stop()
}

func TestUpstreamSupplierBalancePollerPrunesOldSamplesBounded(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	poller := NewUpstreamSupplierBalancePollerWithOptions(db, nil, nil, time.Hour, 5*time.Minute, 4, 4, 35*24*time.Hour)
	mock.ExpectExec("DELETE FROM upstream_supplier_balance_samples").
		WithArgs("3024000 seconds", 1000).
		WillReturnResult(sqlmock.NewResult(0, 8))

	poller.pruneOldSamples(context.Background(), 1000)

	require.NoError(t, mock.ExpectationsWereMet())
}
