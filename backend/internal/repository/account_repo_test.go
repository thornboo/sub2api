package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

const parameterLimitTestDriverName = "sub2api_param_limit_test"

var registerParameterLimitTestDriverOnce sync.Once

func TestAccountsToService_LargeActiveAccountSetDoesNotExceedPostgresParameterLimit(t *testing.T) {
	repo := newParameterLimitAccountRepo(t)

	accounts := make([]*dbent.Account, 0, 65536)
	for i := range 65536 {
		accounts = append(accounts, &dbent.Account{
			ID:          int64(i + 1),
			Name:        "large-active",
			Platform:    service.PlatformOpenAI,
			Type:        service.AccountTypeOAuth,
			Credentials: map[string]any{},
			Extra:       map[string]any{},
			Status:      service.StatusActive,
			Schedulable: true,
		})
	}

	got, err := repo.accountsToService(context.Background(), accounts)
	require.NoError(t, err)
	require.Len(t, got, len(accounts))
}

func TestAccountListOrder_UpstreamEffectiveDiscountSQL(t *testing.T) {
	query, args := accountListOrderQueryForTest(pagination.PaginationParams{
		SortBy:    "upstream_effective_discount",
		SortOrder: pagination.SortOrderDesc,
	})

	require.Equal(t, []any{service.StatusActive, service.StatusActive}, args)
	require.Contains(t, query, `LEFT JOIN "upstream_account_cost_bindings" AS "upstream_account_cost_binding_sort"`)
	require.Contains(t, query, `LEFT JOIN "upstream_cost_pools" AS "upstream_cost_pool_sort"`)
	require.Contains(t, query, `LEFT JOIN "upstream_suppliers" AS "upstream_supplier_sort"`)
	require.Contains(t, query, `"upstream_account_cost_binding_sort"."account_id" = "accounts"."id"`)
	require.Contains(t, query, `"upstream_cost_pool_sort"."id" = "upstream_account_cost_binding_sort"."cost_pool_id"`)
	require.Contains(t, query, `"upstream_cost_pool_sort"."archived_at" IS NULL`)
	require.Contains(t, query, `"upstream_supplier_sort"."id" IS NOT NULL AND "upstream_cost_pool_sort"."current_snapshot_id" IS NOT NULL`)
	require.Contains(t, query, `(("upstream_cost_pool_sort"."current_effective_cny_per_usd" / NULLIF("upstream_cost_pool_sort"."reference_fx_rate", 0)) * "upstream_account_cost_binding_sort"."default_multiplier") END DESC NULLS LAST`)
	require.True(t, strings.Contains(query, `ORDER BY`) && strings.Contains(query, `"accounts"."id" DESC`), query)
}

func TestAccountListOrder_UpstreamMultiplierSQL(t *testing.T) {
	query, args := accountListOrderQueryForTest(pagination.PaginationParams{
		SortBy:    "upstream_multiplier",
		SortOrder: pagination.SortOrderAsc,
	})

	require.Equal(t, []any{service.StatusActive, service.StatusActive}, args)
	require.Contains(t, query, `LEFT JOIN "upstream_account_cost_bindings" AS "upstream_account_cost_binding_sort"`)
	require.Contains(t, query, `LEFT JOIN "upstream_cost_pools" AS "upstream_cost_pool_sort"`)
	require.Contains(t, query, `"upstream_cost_pool_sort"."archived_at" IS NULL`)
	require.Contains(t, query, `CASE WHEN "upstream_supplier_sort"."id" IS NOT NULL THEN "upstream_account_cost_binding_sort"."default_multiplier" END ASC NULLS LAST`)
	require.True(t, strings.Contains(query, `ORDER BY`) && strings.Contains(query, `"accounts"."id" ASC`), query)
}

func TestAccountRepository_LoadUpstreamEffectiveDiscounts(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := newAccountRepositoryWithSQL(nil, db, nil)
	mock.ExpectQuery(`(?s)SELECT binding\.account_id,.*FROM upstream_account_cost_bindings binding.*JOIN upstream_cost_pools pool ON pool\.id = binding\.cost_pool_id.*JOIN upstream_suppliers supplier ON supplier\.id = pool\.supplier_id.*binding\.status = \$2.*binding\.valid_to IS NULL.*pool\.status = \$2.*pool\.archived_at IS NULL.*supplier\.is_system = FALSE.*pool\.current_snapshot_id IS NOT NULL.*pool\.reference_fx_rate > 0.*binding\.default_multiplier > 0`).
		WithArgs(sqlmock.AnyArg(), service.StatusActive).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "effective_discount"}).
			AddRow(int64(1), 0.4).
			AddRow(int64(2), 1.2).
			AddRow(int64(3), nil))

	got, err := repo.loadUpstreamEffectiveDiscounts(context.Background(), []int64{1, 2, 3})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
	require.InDelta(t, 0.4, *got[1], 0.000001)
	require.InDelta(t, 1.2, *got[2], 0.000001)
	require.Nil(t, got[3])
}

func accountListOrderQueryForTest(params pagination.PaginationParams) (string, []any) {
	selector := entsql.Dialect(dialect.Postgres).
		Select(dbaccount.FieldID).
		From(entsql.Dialect(dialect.Postgres).Table(dbaccount.Table))

	for _, order := range accountListOrder(params) {
		order(selector)
	}

	return selector.Query()
}

func newParameterLimitAccountRepo(t *testing.T) *accountRepository {
	t.Helper()

	registerParameterLimitTestDriverOnce.Do(func() {
		sql.Register(parameterLimitTestDriverName, parameterLimitDriver{})
	})

	db, err := sql.Open(parameterLimitTestDriverName, "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	return newAccountRepositoryWithSQL(client, nil, nil)
}

type parameterLimitDriver struct{}

func (parameterLimitDriver) Open(string) (driver.Conn, error) {
	return parameterLimitConn{}, nil
}

type parameterLimitConn struct{}

func (parameterLimitConn) Prepare(query string) (driver.Stmt, error) {
	return parameterLimitStmt{query: query}, nil
}

func (parameterLimitConn) Close() error {
	return nil
}

func (parameterLimitConn) Begin() (driver.Tx, error) {
	return parameterLimitTx{}, nil
}

func (parameterLimitConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return queryWithParameterLimit(query, args)
}

type parameterLimitStmt struct {
	query string
}

func (s parameterLimitStmt) Close() error {
	return nil
}

func (s parameterLimitStmt) NumInput() int {
	return -1
}

func (s parameterLimitStmt) Exec(args []driver.Value) (driver.Result, error) {
	return driver.RowsAffected(0), parameterLimitError(len(args))
}

func (s parameterLimitStmt) Query(args []driver.Value) (driver.Rows, error) {
	namedArgs := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return queryWithParameterLimit(s.query, namedArgs)
}

type parameterLimitTx struct{}

func (parameterLimitTx) Commit() error {
	return nil
}

func (parameterLimitTx) Rollback() error {
	return nil
}

func queryWithParameterLimit(query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := parameterLimitError(len(args)); err != nil {
		return nil, err
	}
	return parameterLimitRows{columns: columnsForParameterLimitQuery(query)}, nil
}

func parameterLimitError(paramCount int) error {
	if paramCount <= 65535 {
		return nil
	}
	return fmt.Errorf("pq: got %d parameters but PostgreSQL only supports 65535 parameters", paramCount)
}

func columnsForParameterLimitQuery(query string) []string {
	if query == "" {
		return nil
	}
	return []string{"account_id", "group_id", "priority", "created_at"}
}

type parameterLimitRows struct {
	columns []string
}

func (r parameterLimitRows) Columns() []string {
	return r.columns
}

func (parameterLimitRows) Close() error {
	return nil
}

func (parameterLimitRows) Next([]driver.Value) error {
	return io.EOF
}
