//go:build unit

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newUpstreamSupplierRechargeAnalyticsService(t *testing.T) (*adminServiceImpl, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(matchUpstreamSupplierRechargeAnalyticsSQL)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &adminServiceImpl{
		entClient:   client,
		accountRepo: &mockAccountRepoForGemini{},
	}, mock
}

func matchUpstreamSupplierRechargeAnalyticsSQL(expectedSQL, actualSQL string) error {
	commonRechargeFragments := []string{
		"FROM upstream_recharge_records r",
		"JOIN upstream_cost_pools p ON p.id = r.cost_pool_id",
		"JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id",
		"r.deleted_at IS NULL",
		"r.voided_at IS NULL",
		"supplier.is_system = FALSE",
	}

	switch expectedSQL {
	case "analytics-overview-suppliers":
		return requireSQLFragments(actualSQL, append(commonRechargeFragments,
			"WHEN r.paid_currency = 'CNY' THEN r.paid_amount",
			"WHEN r.paid_currency = 'USD' THEN r.paid_amount * r.reference_fx_rate",
			"OVER (PARTITION BY p.supplier_id)",
			"GROUP BY p.supplier_id, r.paid_currency",
			"ORDER BY p.supplier_id, r.paid_currency",
		)...)
	case "analytics-supplier-visible":
		return requireSQLFragments(actualSQL,
			"FROM upstream_suppliers",
			"id = $1",
			"is_system = FALSE",
		)
	case "analytics-trend-totals":
		return requireSQLFragments(actualSQL, append(commonRechargeFragments,
			"p.supplier_id = $1",
			"GROUP BY r.paid_currency",
			"ORDER BY r.paid_currency",
		)...)
	case "analytics-trend-points":
		return requireSQLFragments(actualSQL, append(commonRechargeFragments,
			"to_char(date_trunc($2, r.recorded_at AT TIME ZONE 'UTC'), $3)",
			"p.supplier_id = $1",
			"GROUP BY period, r.paid_currency",
			"ORDER BY period, r.paid_currency",
		)...)
	default:
		if strings.Contains(actualSQL, expectedSQL) {
			return nil
		}
		return fmt.Errorf("unexpected query marker %q for SQL: %s", expectedSQL, actualSQL)
	}
}

func requireSQLFragments(sql string, fragments ...string) error {
	for _, fragment := range fragments {
		if !strings.Contains(sql, fragment) {
			return fmt.Errorf("expected SQL to contain %q: %s", fragment, sql)
		}
	}
	return nil
}

func TestGetUpstreamSupplierRechargeOverviewGroupsByPaidCurrency(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)

	mock.ExpectQuery("analytics-overview-suppliers").
		WillReturnRows(sqlmock.NewRows([]string{"supplier_id", "paid_currency", "amount", "record_count", "reference_cny_amount"}).
			AddRow(int64(10), "CNY", 80.5, 1, 257.25).
			AddRow(int64(10), "USD", 25.25, 1, 257.25).
			AddRow(int64(11), "CNY", 20.0, 1, 20.0))

	overview, err := svc.GetUpstreamSupplierRechargeOverview(context.Background())
	require.NoError(t, err)
	require.Equal(t, []UpstreamRechargePaymentTotal{
		{Currency: "CNY", Amount: 100.5, RecordCount: 2},
		{Currency: "USD", Amount: 25.25, RecordCount: 1},
	}, overview.Totals)
	require.Equal(t, []UpstreamSupplierRechargeTotal{
		{
			SupplierID:         10,
			ReferenceCNYAmount: 257.25,
			Totals: []UpstreamRechargePaymentTotal{
				{Currency: "CNY", Amount: 80.5, RecordCount: 1},
				{Currency: "USD", Amount: 25.25, RecordCount: 1},
			},
		},
		{
			SupplierID:         11,
			ReferenceCNYAmount: 20,
			Totals: []UpstreamRechargePaymentTotal{
				{Currency: "CNY", Amount: 20, RecordCount: 1},
			},
		},
	}, overview.Suppliers)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUpstreamSupplierRechargeOverviewReturnsEmptySlices(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)

	mock.ExpectQuery("analytics-overview-suppliers").
		WillReturnRows(sqlmock.NewRows([]string{"supplier_id", "paid_currency", "amount", "record_count", "reference_cny_amount"}))

	overview, err := svc.GetUpstreamSupplierRechargeOverview(context.Background())
	require.NoError(t, err)
	require.Empty(t, overview.Totals)
	require.Empty(t, overview.Suppliers)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResolveUpstreamRechargeTrendGranularity(t *testing.T) {
	tests := []struct {
		name             string
		requested        UpstreamRechargeTrendGranularity
		wantGranularity  UpstreamRechargeTrendGranularity
		wantPeriodFormat string
	}{
		{name: "defaults to month", wantGranularity: UpstreamRechargeTrendGranularityMonth, wantPeriodFormat: "YYYY-MM"},
		{name: "day", requested: UpstreamRechargeTrendGranularityDay, wantGranularity: UpstreamRechargeTrendGranularityDay, wantPeriodFormat: "YYYY-MM-DD"},
		{name: "week", requested: UpstreamRechargeTrendGranularityWeek, wantGranularity: UpstreamRechargeTrendGranularityWeek, wantPeriodFormat: `IYYY-"W"IW`},
		{name: "month", requested: UpstreamRechargeTrendGranularityMonth, wantGranularity: UpstreamRechargeTrendGranularityMonth, wantPeriodFormat: "YYYY-MM"},
		{name: "year", requested: UpstreamRechargeTrendGranularityYear, wantGranularity: UpstreamRechargeTrendGranularityYear, wantPeriodFormat: "YYYY"},
		{name: "normalizes case and spaces", requested: "  WEEK  ", wantGranularity: UpstreamRechargeTrendGranularityWeek, wantPeriodFormat: `IYYY-"W"IW`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			granularity, periodFormat, err := resolveUpstreamRechargeTrendGranularity(test.requested)
			require.NoError(t, err)
			require.Equal(t, test.wantGranularity, granularity)
			require.Equal(t, test.wantPeriodFormat, periodFormat)
		})
	}
}

func TestGetUpstreamSupplierRechargeTrendGroupsUTCMonthsByPaidCurrency(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)

	mock.ExpectQuery("analytics-supplier-visible").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))
	mock.ExpectQuery("analytics-trend-totals").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"paid_currency", "amount", "record_count"}).
			AddRow("CNY", 120.0, 3).
			AddRow("USD", 10.0, 1))
	mock.ExpectQuery("analytics-trend-points").
		WithArgs(int64(10), "month", "YYYY-MM").
		WillReturnRows(sqlmock.NewRows([]string{"period", "paid_currency", "amount", "record_count"}).
			AddRow("2026-07", "CNY", 20.0, 1).
			AddRow("2026-08", "CNY", 100.0, 2).
			AddRow("2026-08", "USD", 10.0, 1))

	trend, err := svc.GetUpstreamSupplierRechargeTrend(
		context.Background(),
		10,
		UpstreamRechargeTrendGranularityMonth,
	)
	require.NoError(t, err)
	require.Equal(t, int64(10), trend.SupplierID)
	require.Equal(t, UpstreamRechargeTrendGranularityMonth, trend.Granularity)
	require.Equal(t, []UpstreamRechargePaymentTotal{
		{Currency: "CNY", Amount: 120, RecordCount: 3},
		{Currency: "USD", Amount: 10, RecordCount: 1},
	}, trend.Totals)
	require.Equal(t, []UpstreamSupplierRechargeTrendPoint{
		{Period: "2026-07", Currency: "CNY", Amount: 20, RecordCount: 1},
		{Period: "2026-07", Currency: "USD", Amount: 0, RecordCount: 0},
		{Period: "2026-08", Currency: "CNY", Amount: 100, RecordCount: 2},
		{Period: "2026-08", Currency: "USD", Amount: 10, RecordCount: 1},
	}, trend.Points)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUpstreamSupplierRechargeTrendFillsMissingDaysByPaidCurrency(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)

	mock.ExpectQuery("analytics-supplier-visible").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))
	mock.ExpectQuery("analytics-trend-totals").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"paid_currency", "amount", "record_count"}).
			AddRow("CNY", 500.0, 2).
			AddRow("USD", 20.0, 1))
	mock.ExpectQuery("analytics-trend-points").
		WithArgs(int64(10), "day", "YYYY-MM-DD").
		WillReturnRows(sqlmock.NewRows([]string{"period", "paid_currency", "amount", "record_count"}).
			AddRow("2026-08-09", "CNY", 200.0, 1).
			AddRow("2026-08-11", "CNY", 300.0, 1).
			AddRow("2026-08-11", "USD", 20.0, 1))

	trend, err := svc.GetUpstreamSupplierRechargeTrend(
		context.Background(),
		10,
		UpstreamRechargeTrendGranularityDay,
	)
	require.NoError(t, err)
	require.Equal(t, []UpstreamSupplierRechargeTrendPoint{
		{Period: "2026-08-09", Currency: "CNY", Amount: 200, RecordCount: 1},
		{Period: "2026-08-09", Currency: "USD", Amount: 0, RecordCount: 0},
		{Period: "2026-08-10", Currency: "CNY", Amount: 0, RecordCount: 0},
		{Period: "2026-08-10", Currency: "USD", Amount: 0, RecordCount: 0},
		{Period: "2026-08-11", Currency: "CNY", Amount: 300, RecordCount: 1},
		{Period: "2026-08-11", Currency: "USD", Amount: 20, RecordCount: 1},
	}, trend.Points)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFillUpstreamRechargeTrendPeriodsSupportsEveryGranularity(t *testing.T) {
	tests := []struct {
		name        string
		granularity UpstreamRechargeTrendGranularity
		points      []UpstreamSupplierRechargeTrendPoint
		wantPeriods []string
	}{
		{
			name:        "day",
			granularity: UpstreamRechargeTrendGranularityDay,
			points: []UpstreamSupplierRechargeTrendPoint{
				{Period: "2026-08-09", Currency: "CNY", Amount: 1, RecordCount: 1},
				{Period: "2026-08-11", Currency: "CNY", Amount: 1, RecordCount: 1},
			},
			wantPeriods: []string{"2026-08-09", "2026-08-10", "2026-08-11"},
		},
		{
			name:        "ISO week across year boundary",
			granularity: UpstreamRechargeTrendGranularityWeek,
			points: []UpstreamSupplierRechargeTrendPoint{
				{Period: "2025-W52", Currency: "CNY", Amount: 1, RecordCount: 1},
				{Period: "2026-W02", Currency: "CNY", Amount: 1, RecordCount: 1},
			},
			wantPeriods: []string{"2025-W52", "2026-W01", "2026-W02"},
		},
		{
			name:        "month",
			granularity: UpstreamRechargeTrendGranularityMonth,
			points: []UpstreamSupplierRechargeTrendPoint{
				{Period: "2025-12", Currency: "CNY", Amount: 1, RecordCount: 1},
				{Period: "2026-02", Currency: "CNY", Amount: 1, RecordCount: 1},
			},
			wantPeriods: []string{"2025-12", "2026-01", "2026-02"},
		},
		{
			name:        "year",
			granularity: UpstreamRechargeTrendGranularityYear,
			points: []UpstreamSupplierRechargeTrendPoint{
				{Period: "2024", Currency: "CNY", Amount: 1, RecordCount: 1},
				{Period: "2026", Currency: "CNY", Amount: 1, RecordCount: 1},
			},
			wantPeriods: []string{"2024", "2025", "2026"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filled := fillUpstreamRechargeTrendPeriods(test.points, []UpstreamRechargePaymentTotal{{Currency: "CNY"}}, test.granularity)
			periods := make([]string, 0, len(filled))
			for _, point := range filled {
				periods = append(periods, point.Period)
			}
			require.Equal(t, test.wantPeriods, periods)
		})
	}
}

func TestGetUpstreamSupplierRechargeTrendReturnsNotFoundForInvalidOrSystemSupplier(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)

	_, err := svc.GetUpstreamSupplierRechargeTrend(context.Background(), 0, UpstreamRechargeTrendGranularityMonth)
	require.ErrorIs(t, err, ErrUpstreamSupplierNotFound)

	mock.ExpectQuery("analytics-supplier-visible").
		WithArgs(int64(99)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err = svc.GetUpstreamSupplierRechargeTrend(context.Background(), 99, UpstreamRechargeTrendGranularityMonth)
	require.ErrorIs(t, err, ErrUpstreamSupplierNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetUpstreamSupplierRechargeTrendRejectsInvalidGranularity(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)

	_, err := svc.GetUpstreamSupplierRechargeTrend(context.Background(), 10, "hour")
	require.ErrorIs(t, err, ErrInvalidUpstreamRechargeTrendGranularity)
	require.NoError(t, mock.ExpectationsWereMet())
}
