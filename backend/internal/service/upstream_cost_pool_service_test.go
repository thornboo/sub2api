package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestApplyUpstreamSupplierUpdateRenamesAndArchives(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	name := "  Supplier A  "
	note := "shared wallet"
	status := "archived"
	current := &UpstreamSupplier{
		ID:     42,
		Name:   "Old Supplier",
		Status: "active",
	}

	mock.ExpectExec("UPDATE upstream_suppliers").
		WithArgs(int64(42), "Supplier A", sqlmock.AnyArg(), "archived", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = applyUpstreamSupplierUpdate(context.Background(), db, current, UpdateUpstreamSupplierInput{
		SupplierID: current.ID,
		Name:       &name,
		Note:       &note,
		Status:     &status,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyUpstreamSupplierUpdateRejectsBlankName(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	name := "   "
	current := &UpstreamSupplier{
		ID:     42,
		Name:   "Old Supplier",
		Status: "active",
	}

	err = applyUpstreamSupplierUpdate(context.Background(), db, current, UpdateUpstreamSupplierInput{
		SupplierID: current.ID,
		Name:       &name,
	})
	require.ErrorContains(t, err, "upstream supplier name is required")
}

func TestApplyUpstreamSupplierUpdateMapsNameConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	name := "Supplier A"
	current := &UpstreamSupplier{
		ID:     42,
		Name:   "Old Supplier",
		Status: "active",
	}

	mock.ExpectExec("UPDATE upstream_suppliers").
		WithArgs(int64(42), "Supplier A", sqlmock.AnyArg(), "active", sqlmock.AnyArg()).
		WillReturnError(&pq.Error{Code: "23505"})

	err = applyUpstreamSupplierUpdate(context.Background(), db, current, UpdateUpstreamSupplierInput{
		SupplierID: current.ID,
		Name:       &name,
	})
	require.ErrorIs(t, err, ErrUpstreamSupplierNameConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestReservedUpstreamSupplierUsesSystemFlag(t *testing.T) {
	require.True(t, isReservedUpstreamSupplier(&UpstreamSupplier{
		Name:     "Uncategorized",
		IsSystem: true,
	}))
	require.False(t, isReservedUpstreamSupplier(&UpstreamSupplier{
		Name:     "任意供应商",
		IsSystem: false,
	}))
}

func TestFindActiveUpstreamCostPoolIDForAccountIgnoresSystemSupplier(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_ string, actualSQL string) error {
		if !strings.Contains(actualSQL, "JOIN upstream_suppliers supplier") {
			return fmt.Errorf("expected supplier join in active binding lookup: %s", actualSQL)
		}
		if !strings.Contains(actualSQL, "supplier.is_system = FALSE") {
			return fmt.Errorf("active binding lookup must ignore system suppliers: %s", actualSQL)
		}
		return nil
	})))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"cost_pool_id"}))

	value, err := findActiveUpstreamCostPoolIDForAccount(context.Background(), db, 7)
	require.NoError(t, err)
	require.Nil(t, value)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureNamedUpstreamSupplierRejectsSystemNameConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("INSERT INTO upstream_suppliers").
		WithArgs("未归类供应商", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_system"}))
	mock.ExpectQuery("SELECT id, is_system").
		WithArgs("未归类供应商").
		WillReturnRows(sqlmock.NewRows([]string{"id", "is_system"}).AddRow(1, true))

	id, err := ensureNamedUpstreamSupplier(context.Background(), db, "未归类供应商", nil, nil)
	require.Zero(t, id)
	require.ErrorIs(t, err, ErrUpstreamSupplierReserved)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureUpstreamSupplierDeletableBlocksActiveBinding(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_ string, actualSQL string) error {
		if !strings.Contains(actualSQL, "FROM upstream_account_cost_bindings") {
			return fmt.Errorf("expected account binding check in query: %s", actualSQL)
		}
		if !strings.Contains(actualSQL, "b.status = 'active'") {
			return fmt.Errorf("supplier delete must follow active-only binding rule: %s", actualSQL)
		}
		if strings.Contains(actualSQL, "r.deleted_at") {
			return fmt.Errorf("supplier delete must check all recharge history, not only non-deleted rows: %s", actualSQL)
		}
		return nil
	})))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("supplier-deletable-counts").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"active_bindings",
			"binding_history",
			"records",
			"snapshots",
			"non_default_pools",
		}).AddRow(1, 1, 0, 0, 0))

	err = ensureUpstreamSupplierDeletable(context.Background(), db, 42)
	require.ErrorIs(t, err, ErrUpstreamSupplierHasBoundAccounts)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureUpstreamSupplierDeletableAllowsEmptyDefaultPool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"active_bindings",
			"binding_history",
			"records",
			"snapshots",
			"non_default_pools",
		}).AddRow(0, 0, 0, 0, 0))

	err = ensureUpstreamSupplierDeletable(context.Background(), db, 42)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUpdateDefaultUpstreamCostPoolConfigKeepsDefaultsSeparateFromSnapshots(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(_ string, actualSQL string) error {
		for _, fragment := range []string{
			"default_effective_cny_per_usd = COALESCE",
			"default_reference_fx_rate = COALESCE",
		} {
			if !strings.Contains(actualSQL, fragment) {
				return fmt.Errorf("expected %q in default cost-pool update: %s", fragment, actualSQL)
			}
		}
		for _, forbidden := range []string{"current_effective_cny_per_usd =", "reference_fx_rate = CASE"} {
			if strings.Contains(actualSQL, forbidden) {
				return fmt.Errorf("default config must not update real current cost field %q: %s", forbidden, actualSQL)
			}
		}
		return nil
	})))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	defaultEffective := 1.25
	defaultReferenceFX := 7.2
	mock.ExpectExec("UPDATE upstream_cost_pools").
		WithArgs(int64(9), defaultEffective, defaultReferenceFX).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = updateDefaultUpstreamCostPoolConfig(
		context.Background(),
		db,
		9,
		&defaultEffective,
		&defaultReferenceFX,
	)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureUpstreamSupplierDeletableBlocksBindingHistory(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("SELECT").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"active_bindings",
			"binding_history",
			"records",
			"snapshots",
			"non_default_pools",
		}).AddRow(0, 2, 0, 0, 0))

	err = ensureUpstreamSupplierDeletable(context.Background(), db, 42)
	require.ErrorIs(t, err, ErrUpstreamSupplierHasBindingHistory)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateUpstreamSupplierMapsDuplicateNameToConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectQuery("INSERT INTO upstream_suppliers").
		WithArgs("Supplier A", nil, nil).
		WillReturnError(&pq.Error{Code: "23505"})

	id, err := createUpstreamSupplier(context.Background(), db, "Supplier A", nil, nil)
	require.Zero(t, id)
	require.ErrorIs(t, err, ErrUpstreamSupplierNameConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNormalizeUpstreamCostPoolDefaultRejectsNonPositiveValue(t *testing.T) {
	_, err := normalizeUpstreamCostPoolDefault(
		0,
		"INVALID_UPSTREAM_DEFAULT_EFFECTIVE_COST",
		"default effective CNY per USD must be greater than 0",
	)
	require.Error(t, err)
}
