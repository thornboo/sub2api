//go:build integration

package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	svc "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const servicePostgresImageTag = "postgres:18.1-alpine3.23"

var (
	serviceIntegrationDB        *sql.DB
	serviceIntegrationEntClient *dbent.Client
)

type upstreamRechargeAdmin interface {
	CreateUpstreamRechargeRecord(context.Context, svc.UpstreamRechargeRecordInput) (*svc.UpstreamRechargeRecord, error)
	UpdateUpstreamRechargeRecord(context.Context, int64, svc.UpstreamRechargeRecordInput) (*svc.UpstreamRechargeRecord, error)
	DeleteUpstreamRechargeRecord(context.Context, int64, int64) error
	UpdateAccountUpstreamSupplierBinding(context.Context, svc.UpstreamSupplierBindingInput) (*svc.UpstreamAccountCostBinding, error)
}

type upstreamSupplierBindingAdmin interface {
	UpdateAccountUpstreamSupplierBinding(context.Context, svc.UpstreamSupplierBindingInput) (*svc.UpstreamAccountCostBinding, error)
}

type upstreamSupplierAdmin interface {
	CreateUpstreamSupplier(context.Context, svc.CreateUpstreamSupplierInput) (*svc.UpstreamSupplier, error)
	UpdateUpstreamSupplier(context.Context, svc.UpdateUpstreamSupplierInput) (*svc.UpstreamSupplier, error)
	DeleteUpstreamSupplier(context.Context, int64) error
	ListUpstreamCostPools(context.Context) ([]svc.UpstreamCostPool, error)
	UpdateAccountUpstreamSupplierBinding(context.Context, svc.UpstreamSupplierBindingInput) (*svc.UpstreamAccountCostBinding, error)
	UpdateAccountUpstreamCostBinding(context.Context, svc.UpstreamCostBindingInput) (*svc.UpstreamAccountCostBinding, error)
}

type recordingSchedulerCache struct {
	svc.SchedulerCache
	mu       sync.Mutex
	accounts []*svc.Account
}

func (c *recordingSchedulerCache) SetAccount(_ context.Context, account *svc.Account) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	copyValue := *account
	c.accounts = append(c.accounts, &copyValue)
	return nil
}

func (c *recordingSchedulerCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.accounts = nil
}

func (c *recordingSchedulerCache) lastAccount() *svc.Account {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.accounts) == 0 {
		return nil
	}
	copyValue := *c.accounts[len(c.accounts)-1]
	return &copyValue
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	if err := timezone.Init("UTC"); err != nil {
		log.Printf("failed to init timezone: %v", err)
		os.Exit(1)
	}
	serviceConfigureDockerHostFromContext(ctx)
	if !serviceDockerIsAvailable(ctx) {
		if os.Getenv("CI") != "" {
			log.Printf("docker is not available (CI=true); failing integration tests")
			os.Exit(1)
		}
		log.Printf("docker is not available; skipping integration tests")
		os.Exit(0)
	}

	pgContainer, providerErr, err := serviceRunPostgresContainer(ctx)
	if providerErr != nil {
		if os.Getenv("CI") != "" {
			log.Printf("testcontainers provider is not available (CI=true): %v", providerErr)
			os.Exit(1)
		}
		log.Printf("testcontainers provider is not available; skipping integration tests: %v", providerErr)
		os.Exit(0)
	}
	if err != nil {
		log.Printf("failed to start postgres container: %v", err)
		os.Exit(1)
	}
	defer func() { _ = pgContainer.Terminate(ctx) }()

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	if err != nil {
		log.Printf("failed to get postgres dsn: %v", err)
		os.Exit(1)
	}
	serviceIntegrationDB, err = serviceOpenSQLWithRetry(ctx, dsn, 30*time.Second)
	if err != nil {
		log.Printf("failed to open postgres: %v", err)
		os.Exit(1)
	}
	if err := repository.ApplyMigrations(ctx, serviceIntegrationDB); err != nil {
		log.Printf("failed to apply migrations: %v", err)
		os.Exit(1)
	}

	drv := entsql.OpenDB(dialect.Postgres, serviceIntegrationDB)
	serviceIntegrationEntClient = dbent.NewClient(dbent.Driver(drv))

	code := m.Run()

	_ = serviceIntegrationEntClient.Close()
	_ = serviceIntegrationDB.Close()
	os.Exit(code)
}

func TestUpstreamCostPoolRechargeSnapshotIgnoresAdjustment(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamRechargeAdmin(t)
	account := createUpstreamCostPoolAccount(t, map[string]any{
		"upstream_reference_fx_rate":    7.0,
		"upstream_recharge_cny_per_usd": 6.5,
	})
	bindUpstreamRechargeAccount(t, admin, account)

	record, err := admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           7,
		ReceivedCreditAmount: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, record.CostPoolID)
	poolID := *record.CostPoolID

	currentCost, activeSnapshotID := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 7.0, currentCost, 0.000001)
	totalSnapshots, activeSnapshots := requireUpstreamSnapshotCounts(t, poolID)
	require.Equal(t, 1, totalSnapshots)
	require.Equal(t, 1, activeSnapshots)

	_, err = admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "adjustment",
		PaidAmount:           0.01,
		ReceivedCreditAmount: 100,
	})
	require.NoError(t, err)

	currentCost, snapshotAfterAdjustment := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 7.0, currentCost, 0.000001)
	require.Equal(t, activeSnapshotID, snapshotAfterAdjustment)
	totalAfterAdjustment, activeAfterAdjustment := requireUpstreamSnapshotCounts(t, poolID)
	require.Equal(t, totalSnapshots, totalAfterAdjustment)
	require.Equal(t, 1, activeAfterAdjustment)

	_, err = admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "bonus",
		PaidAmount:           7,
		ReceivedCreditAmount: 2,
	})
	require.NoError(t, err)

	currentCost, snapshotAfterBonus := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 7.0, currentCost, 0.000001)
	require.Equal(t, activeSnapshotID, snapshotAfterBonus)
	totalAfterBonus, activeAfterBonus := requireUpstreamSnapshotCounts(t, poolID)
	require.Equal(t, totalSnapshots, totalAfterBonus)
	require.Equal(t, 1, activeAfterBonus)

	_, err = admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           8,
		ReceivedCreditAmount: 1,
	})
	require.NoError(t, err)

	currentCost, latestSnapshotID := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 8.0, currentCost, 0.000001)
	require.NotEqual(t, activeSnapshotID, latestSnapshotID)
	requireUpstreamSnapshotClosed(t, activeSnapshotID)
	totalAfterRecharge, activeAfterRecharge := requireUpstreamSnapshotCounts(t, poolID)
	require.Equal(t, totalSnapshots+1, totalAfterRecharge)
	require.Equal(t, 1, activeAfterRecharge)
}

func TestUpstreamRechargeUpdateDeleteRefreshesCurrentSnapshot(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamRechargeAdmin(t)
	account := createUpstreamCostPoolAccount(t, map[string]any{
		"upstream_reference_fx_rate":    7.0,
		"upstream_recharge_cny_per_usd": 7.0,
	})
	bindUpstreamRechargeAccount(t, admin, account)
	recordedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	first, err := admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           7,
		ReceivedCreditAmount: 1,
		RecordedAt:           &recordedAt,
	})
	require.NoError(t, err)
	require.NotNil(t, first.CostPoolID)
	poolID := *first.CostPoolID

	secondRecordedAt := recordedAt.Add(time.Minute)
	second, err := admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           8,
		ReceivedCreditAmount: 1,
		RecordedAt:           &secondRecordedAt,
	})
	require.NoError(t, err)

	currentCost, secondSnapshotID := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 8.0, currentCost, 0.000001)
	require.Equal(t, sql.NullInt64{Int64: second.ID, Valid: true}, requireActiveUpstreamSnapshotSource(t, poolID))

	updatedRecordedAt := recordedAt.Add(2 * time.Minute)
	updated, err := admin.UpdateUpstreamRechargeRecord(ctx, second.ID, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           6,
		ReceivedCreditAmount: 1,
		RecordedAt:           &updatedRecordedAt,
	})
	require.NoError(t, err)
	require.NotNil(t, updated.CostPoolID)

	currentCost, updatedSnapshotID := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 6.0, currentCost, 0.000001)
	require.NotEqual(t, secondSnapshotID, updatedSnapshotID)
	requireUpstreamSnapshotClosed(t, secondSnapshotID)
	require.Equal(t, sql.NullInt64{Int64: second.ID, Valid: true}, requireActiveUpstreamSnapshotSource(t, poolID))

	require.NoError(t, admin.DeleteUpstreamRechargeRecord(ctx, account.ID, second.ID))

	currentCost, fallbackSnapshotID := requireUpstreamPoolCurrentCost(t, poolID)
	require.InDelta(t, 7.0, currentCost, 0.000001)
	require.NotEqual(t, updatedSnapshotID, fallbackSnapshotID)
	requireUpstreamSnapshotClosed(t, updatedSnapshotID)
	require.Equal(t, sql.NullInt64{Int64: first.ID, Valid: true}, requireActiveUpstreamSnapshotSource(t, poolID))

	require.NoError(t, admin.DeleteUpstreamRechargeRecord(ctx, account.ID, first.ID))

	clearedCost, clearedSnapshotID := requireUpstreamPoolCurrentCostNullable(t, poolID)
	require.False(t, clearedCost.Valid)
	require.False(t, clearedSnapshotID.Valid)
	requireUpstreamSnapshotClosed(t, fallbackSnapshotID)
	_, activeSnapshots := requireUpstreamSnapshotCounts(t, poolID)
	require.Equal(t, 0, activeSnapshots)
}

func TestUpstreamRechargeMutationRefreshesBoundSchedulerAccount(t *testing.T) {
	ctx := context.Background()
	cache := &recordingSchedulerCache{}
	admin := newUpstreamRechargeAdminWithCache(t, cache)
	account := createUpstreamCostPoolAccount(t, nil)
	binding := bindUpstreamRechargeAccount(t, admin, account)
	cache.reset()

	record, err := admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           7,
		ReceivedCreditAmount: 1,
	})
	require.NoError(t, err)
	refreshed := cache.lastAccount()
	require.NotNil(t, refreshed)
	require.Equal(t, account.ID, refreshed.ID)
	require.NotNil(t, refreshed.UpstreamEffectiveDiscount)
	require.InDelta(t, 1, *refreshed.UpstreamEffectiveDiscount, 0.000001)

	cache.reset()
	_, err = admin.UpdateUpstreamRechargeRecord(ctx, record.ID, svc.UpstreamRechargeRecordInput{
		AccountID:            account.ID,
		Type:                 "recharge",
		PaidAmount:           3.5,
		ReceivedCreditAmount: 1,
	})
	require.NoError(t, err)
	refreshed = cache.lastAccount()
	require.NotNil(t, refreshed)
	require.NotNil(t, refreshed.UpstreamEffectiveDiscount)
	require.InDelta(t, 0.5, *refreshed.UpstreamEffectiveDiscount, 0.000001)

	cache.reset()
	require.NoError(t, admin.DeleteUpstreamRechargeRecord(ctx, account.ID, record.ID))
	refreshed = cache.lastAccount()
	require.NotNil(t, refreshed)
	require.Nil(t, refreshed.UpstreamEffectiveDiscount)

	var activeBindings int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM upstream_account_cost_bindings
WHERE cost_pool_id = $1
  AND status = 'active'`, binding.CostPoolID).Scan(&activeBindings))
	require.Equal(t, 1, activeBindings)
}

func TestUpstreamCostPoolConcurrentRechargeUsesExistingSupplierBinding(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamRechargeAdmin(t)
	account := createUpstreamCostPoolAccount(t, map[string]any{
		"upstream_reference_fx_rate":    7.0,
		"upstream_recharge_cny_per_usd": 7.0,
	})
	binding := bindUpstreamRechargeAccount(t, admin, account)

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)
	poolIDs := make([]int64, 2)
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			record, err := admin.CreateUpstreamRechargeRecord(ctx, svc.UpstreamRechargeRecordInput{
				AccountID:            account.ID,
				Type:                 "recharge",
				PaidAmount:           7 + float64(index),
				ReceivedCreditAmount: 1,
			})
			errs[index] = err
			if err == nil && record != nil && record.CostPoolID != nil {
				poolIDs[index] = *record.CostPoolID
			}
		}(i)
	}
	close(start)
	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.NotZero(t, poolIDs[0])
	require.Equal(t, poolIDs[0], poolIDs[1])
	require.Equal(t, binding.CostPoolID, poolIDs[0])

	var activeBindings int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
SELECT COUNT(*)::int
FROM upstream_account_cost_bindings
WHERE account_id = $1
  AND status = 'active'`, account.ID).Scan(&activeBindings))
	require.Equal(t, 1, activeBindings)

	var recordCount, distinctPools int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
SELECT COUNT(*)::int,
       COUNT(DISTINCT cost_pool_id)::int
FROM upstream_recharge_records
WHERE account_id = $1
  AND deleted_at IS NULL`, account.ID).Scan(&recordCount, &distinctPools))
	require.Equal(t, 2, recordCount)
	require.Equal(t, 1, distinctPools)

	_, activeSnapshots := requireUpstreamSnapshotCounts(t, poolIDs[0])
	require.Equal(t, 1, activeSnapshots)
}

func TestUpstreamSupplierBindingConcurrentCreateConverges(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamSupplierBindingAdmin(t)
	supplierName := fmt.Sprintf("supplier-a-%d", time.Now().UnixNano())
	accounts := []*svc.Account{
		createUpstreamCostPoolAccount(t, nil),
		createUpstreamCostPoolAccount(t, nil),
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, len(accounts))
	bindings := make([]*svc.UpstreamAccountCostBinding, len(accounts))
	for i, account := range accounts {
		wg.Add(1)
		go func(index int, accountID int64) {
			defer wg.Done()
			<-start
			binding, err := admin.UpdateAccountUpstreamSupplierBinding(ctx, svc.UpstreamSupplierBindingInput{
				AccountID:         accountID,
				SupplierName:      supplierName,
				DefaultMultiplier: 0.5 + float64(index)/10,
			})
			errs[index] = err
			bindings[index] = binding
		}(i, account.ID)
	}
	close(start)
	wg.Wait()

	for i := range errs {
		require.NoError(t, errs[i])
		require.NotNil(t, bindings[i])
	}
	require.Equal(t, bindings[0].SupplierID, bindings[1].SupplierID)
	require.Equal(t, bindings[0].CostPoolID, bindings[1].CostPoolID)
	require.Equal(t, supplierName, bindings[0].SupplierName)
	require.Equal(t, "主余额池", bindings[0].CostPoolName)
	require.InDelta(t, 0.5, bindings[0].DefaultMultiplier, 0.000001)
	require.InDelta(t, 0.6, bindings[1].DefaultMultiplier, 0.000001)

	var supplierCount int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
	SELECT COUNT(*)::int
	FROM upstream_suppliers
	WHERE name = $1
	  AND archived_at IS NULL`, supplierName).Scan(&supplierCount))
	require.Equal(t, 1, supplierCount)

	var defaultPoolCount int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
	SELECT COUNT(*)::int
	FROM upstream_cost_pools
	WHERE supplier_id = $1
	  AND name = $2
	  AND archived_at IS NULL`, bindings[0].SupplierID, "主余额池").Scan(&defaultPoolCount))
	require.Equal(t, 1, defaultPoolCount)

	initialSnapshotCount, _ := requireUpstreamSnapshotCounts(t, bindings[0].CostPoolID)
	require.Equal(t, 0, initialSnapshotCount)

	var activeBindingCount int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
	SELECT COUNT(*)::int
	FROM upstream_account_cost_bindings
	WHERE cost_pool_id = $1
	  AND status = 'active'`, bindings[0].CostPoolID).Scan(&activeBindingCount))
	require.Equal(t, len(accounts), activeBindingCount)

	clearedBinding, err := admin.UpdateAccountUpstreamSupplierBinding(ctx, svc.UpstreamSupplierBindingInput{
		AccountID: accounts[0].ID,
		Clear:     true,
	})
	require.NoError(t, err)
	require.Nil(t, clearedBinding)

	var clearedAccountActiveBindings int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
	SELECT COUNT(*)::int
	FROM upstream_account_cost_bindings
	WHERE account_id = $1
	  AND status = 'active'`, accounts[0].ID).Scan(&clearedAccountActiveBindings))
	require.Equal(t, 0, clearedAccountActiveBindings)

	var remainingActiveBindings int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
	SELECT COUNT(*)::int
	FROM upstream_account_cost_bindings
	WHERE cost_pool_id = $1
	  AND status = 'active'`, bindings[0].CostPoolID).Scan(&remainingActiveBindings))
	require.Equal(t, len(accounts)-1, remainingActiveBindings)
}

func TestUpstreamSupplierDefaultsStaySeparateAndDuplicateCreateConflicts(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamSupplierAdmin(t)
	name := fmt.Sprintf("strict-supplier-%d", time.Now().UnixNano())

	supplier, err := admin.CreateUpstreamSupplier(ctx, svc.CreateUpstreamSupplierInput{
		Name:                      name,
		DefaultEffectiveCNYPerUSD: 1.25,
		DefaultReferenceFXRate:    7.2,
	})
	require.NoError(t, err)
	require.NotNil(t, supplier)

	pools, err := admin.ListUpstreamCostPools(ctx)
	require.NoError(t, err)
	var defaultPool *svc.UpstreamCostPool
	for i := range pools {
		if pools[i].SupplierID == supplier.ID && pools[i].IsDefault {
			defaultPool = &pools[i]
			break
		}
	}
	require.NotNil(t, defaultPool)
	require.InDelta(t, 1.25, defaultPool.DefaultEffectiveCNYPerUSD, 0.000001)
	require.InDelta(t, 7.2, defaultPool.DefaultReferenceFXRate, 0.000001)
	require.Nil(t, defaultPool.CurrentEffectiveCNYPerUSD)
	require.Nil(t, defaultPool.CurrentSnapshotID)

	_, err = admin.CreateUpstreamSupplier(ctx, svc.CreateUpstreamSupplierInput{
		Name:                      name,
		DefaultEffectiveCNYPerUSD: 0.5,
		DefaultReferenceFXRate:    6.8,
	})
	require.ErrorIs(t, err, svc.ErrUpstreamSupplierNameConflict)

	var persistedEffective, persistedReference float64
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `
SELECT default_effective_cny_per_usd::double precision,
       default_reference_fx_rate::double precision
FROM upstream_cost_pools
WHERE supplier_id = $1
  AND is_default = TRUE`, supplier.ID).Scan(&persistedEffective, &persistedReference))
	require.InDelta(t, 1.25, persistedEffective, 0.000001)
	require.InDelta(t, 7.2, persistedReference, 0.000001)
}

func TestArchivedUpstreamSupplierRejectsNewBinding(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamSupplierAdmin(t)
	account := createUpstreamCostPoolAccount(t, nil)
	name := fmt.Sprintf("archived-supplier-%d", time.Now().UnixNano())
	supplier, err := admin.CreateUpstreamSupplier(ctx, svc.CreateUpstreamSupplierInput{Name: name})
	require.NoError(t, err)
	pools, err := admin.ListUpstreamCostPools(ctx)
	require.NoError(t, err)
	var defaultPoolID int64
	for _, pool := range pools {
		if pool.SupplierID == supplier.ID && pool.IsDefault {
			defaultPoolID = pool.ID
			break
		}
	}
	require.Positive(t, defaultPoolID)

	archived := "archived"
	_, err = admin.UpdateUpstreamSupplier(ctx, svc.UpdateUpstreamSupplierInput{
		SupplierID: supplier.ID,
		Status:     &archived,
	})
	require.NoError(t, err)

	binding, err := admin.UpdateAccountUpstreamSupplierBinding(ctx, svc.UpstreamSupplierBindingInput{
		AccountID:         account.ID,
		SupplierID:        supplier.ID,
		DefaultMultiplier: 1,
	})
	require.Nil(t, binding)
	require.ErrorIs(t, err, svc.ErrUpstreamSupplierNotFound)

	binding, err = admin.UpdateAccountUpstreamCostBinding(ctx, svc.UpstreamCostBindingInput{
		AccountID:         account.ID,
		CostPoolID:        defaultPoolID,
		DefaultMultiplier: 1,
	})
	require.Nil(t, binding)
	require.ErrorIs(t, err, svc.ErrUpstreamCostPoolNotFound)
}

func TestUsedUpstreamSupplierMustBeArchivedInsteadOfDeleted(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamSupplierAdmin(t)
	account := createUpstreamCostPoolAccount(t, nil)
	supplier, err := admin.CreateUpstreamSupplier(ctx, svc.CreateUpstreamSupplierInput{
		Name: fmt.Sprintf("used-supplier-%d", time.Now().UnixNano()),
	})
	require.NoError(t, err)

	binding, err := admin.UpdateAccountUpstreamSupplierBinding(ctx, svc.UpstreamSupplierBindingInput{
		AccountID:         account.ID,
		SupplierID:        supplier.ID,
		DefaultMultiplier: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, binding)

	cleared, err := admin.UpdateAccountUpstreamSupplierBinding(ctx, svc.UpstreamSupplierBindingInput{
		AccountID: account.ID,
		Clear:     true,
	})
	require.NoError(t, err)
	require.Nil(t, cleared)
	require.ErrorIs(t, admin.DeleteUpstreamSupplier(ctx, supplier.ID), svc.ErrUpstreamSupplierHasBindingHistory)
}

func newUpstreamRechargeAdmin(t *testing.T) upstreamRechargeAdmin {
	t.Helper()
	return newUpstreamRechargeAdminWithCache(t, nil)
}

func newUpstreamRechargeAdminWithCache(t *testing.T, cache svc.SchedulerCache) upstreamRechargeAdmin {
	t.Helper()
	accountRepo := repository.NewAccountRepository(serviceIntegrationEntClient, serviceIntegrationDB, cache)
	adminService := svc.NewAdminService(
		nil,
		nil,
		accountRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		serviceIntegrationEntClient,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	admin, ok := adminService.(upstreamRechargeAdmin)
	require.True(t, ok)
	return admin
}

func bindUpstreamRechargeAccount(t *testing.T, admin upstreamRechargeAdmin, account *svc.Account) *svc.UpstreamAccountCostBinding {
	t.Helper()
	require.NotNil(t, account)
	binding, err := admin.UpdateAccountUpstreamSupplierBinding(context.Background(), svc.UpstreamSupplierBindingInput{
		AccountID:         account.ID,
		SupplierName:      fmt.Sprintf("recharge-supplier-%d", account.ID),
		DefaultMultiplier: 1,
	})
	require.NoError(t, err)
	require.NotNil(t, binding)
	return binding
}

func newUpstreamSupplierBindingAdmin(t *testing.T) upstreamSupplierBindingAdmin {
	t.Helper()
	accountRepo := repository.NewAccountRepository(serviceIntegrationEntClient, serviceIntegrationDB, nil)
	adminService := svc.NewAdminService(
		nil,
		nil,
		accountRepo,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		serviceIntegrationEntClient,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	admin, ok := adminService.(upstreamSupplierBindingAdmin)
	require.True(t, ok)
	return admin
}

func newUpstreamSupplierAdmin(t *testing.T) upstreamSupplierAdmin {
	t.Helper()
	accountRepo := repository.NewAccountRepository(serviceIntegrationEntClient, serviceIntegrationDB, nil)
	adminService := svc.NewAdminService(
		nil, nil, accountRepo, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		serviceIntegrationEntClient,
		nil, nil, nil, nil, nil,
	)
	admin, ok := adminService.(upstreamSupplierAdmin)
	require.True(t, ok)
	return admin
}

func createUpstreamCostPoolAccount(t *testing.T, extra map[string]any) *svc.Account {
	t.Helper()
	accountRepo := repository.NewAccountRepository(serviceIntegrationEntClient, serviceIntegrationDB, nil)
	account := &svc.Account{
		Name:               fmt.Sprintf("upstream-cost-it-%d", time.Now().UnixNano()),
		Platform:           svc.PlatformOpenAI,
		Type:               svc.AccountTypeAPIKey,
		Credentials:        map[string]any{"api_key": "sk-test"},
		Extra:              extra,
		Concurrency:        1,
		Priority:           50,
		Status:             svc.StatusActive,
		Schedulable:        true,
		AutoPauseOnExpired: true,
	}
	require.NoError(t, accountRepo.Create(context.Background(), account))
	return account
}

func requireUpstreamPoolCurrentCost(t *testing.T, poolID int64) (float64, int64) {
	t.Helper()
	var cost float64
	var snapshotID int64
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT current_effective_cny_per_usd::double precision,
       current_snapshot_id
FROM upstream_cost_pools
WHERE id = $1`, poolID).Scan(&cost, &snapshotID))
	return cost, snapshotID
}

func requireUpstreamPoolCurrentCostNullable(t *testing.T, poolID int64) (sql.NullFloat64, sql.NullInt64) {
	t.Helper()
	var cost sql.NullFloat64
	var snapshotID sql.NullInt64
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT current_effective_cny_per_usd::double precision,
       current_snapshot_id
FROM upstream_cost_pools
WHERE id = $1`, poolID).Scan(&cost, &snapshotID))
	return cost, snapshotID
}

func requireActiveUpstreamSnapshotSource(t *testing.T, poolID int64) sql.NullInt64 {
	t.Helper()
	var sourceRecordID sql.NullInt64
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT source_record_id
FROM upstream_cost_snapshots
WHERE cost_pool_id = $1
  AND valid_to IS NULL
ORDER BY id DESC
LIMIT 1`, poolID).Scan(&sourceRecordID))
	return sourceRecordID
}

func requireUpstreamSnapshotCounts(t *testing.T, poolID int64) (int, int) {
	t.Helper()
	var total, active int
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT COUNT(*)::int,
       COUNT(*) FILTER (WHERE valid_to IS NULL)::int
FROM upstream_cost_snapshots
WHERE cost_pool_id = $1`, poolID).Scan(&total, &active))
	return total, active
}

func requireUpstreamSnapshotClosed(t *testing.T, snapshotID int64) {
	t.Helper()
	var closed bool
	require.NoError(t, serviceIntegrationDB.QueryRowContext(context.Background(), `
SELECT valid_to IS NOT NULL
FROM upstream_cost_snapshots
WHERE id = $1`, snapshotID).Scan(&closed))
	require.True(t, closed)
}

func serviceDockerIsAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "info")
	cmd.Env = os.Environ()
	return cmd.Run() == nil
}

func serviceConfigureDockerHostFromContext(ctx context.Context) {
	if os.Getenv("DOCKER_HOST") == "" {
		cmd := exec.CommandContext(ctx, "docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
		cmd.Env = os.Environ()
		out, err := cmd.Output()
		dockerHost := strings.TrimSpace(string(out))
		if err == nil && dockerHost != "" && dockerHost != "<no value>" {
			_ = os.Setenv("DOCKER_HOST", dockerHost)
		}
	}

	if os.Getenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE") == "" && strings.Contains(os.Getenv("DOCKER_HOST"), ".colima/") {
		_ = os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")
	}
}

func serviceRunPostgresContainer(ctx context.Context) (container *tcpostgres.PostgresContainer, providerErr error, runErr error) {
	defer func() {
		if r := recover(); r != nil {
			providerErr = fmt.Errorf("%v", r)
		}
	}()

	container, runErr = tcpostgres.Run(
		ctx,
		servicePostgresImageTag,
		tcpostgres.WithDatabase("sub2api_service_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	return container, nil, runErr
}

func serviceOpenSQLWithRetry(ctx context.Context, dsn string, timeout time.Duration) (*sql.DB, error) {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		db, err := sql.Open("postgres", dsn)
		if err != nil {
			lastErr = err
			time.Sleep(250 * time.Millisecond)
			continue
		}
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}
		lastErr = err
		_ = db.Close()
		time.Sleep(250 * time.Millisecond)
	}
	return nil, fmt.Errorf("db not ready after %s: %w", timeout, lastErr)
}
