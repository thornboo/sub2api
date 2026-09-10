//go:build unit

package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func supplierConsumptionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"period", "id", "enabled", "active", "query_status", "opening_at", "opening_balance", "opening_unit", "closing_at", "closing_balance", "closing_unit", "credited", "incompatible"})
}

func expectEmptySupplierConsumption(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("FROM upstream_supplier_balance_samples h").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnRows(supplierConsumptionRows())
}

func TestSupplierConsumptionPeriodsUseCalendarDays(t *testing.T) {
	for _, tc := range []struct {
		zone, now, today, week string
	}{
		{"Asia/Shanghai", "2026-09-10T04:00:00Z", "2026-09-09T16:00:00Z", "2026-09-03T16:00:00Z"},
		// A seven-calendar-day range spanning DST is not a fixed 168 hours.
		{"America/New_York", "2026-03-10T04:30:00Z", "2026-03-10T04:00:00Z", "2026-03-04T05:00:00Z"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			require.NoError(t, err)
			now, err := time.Parse(time.RFC3339, tc.now)
			require.NoError(t, err)
			periods := supplierConsumptionPeriods(now, loc)
			require.Equal(t, tc.today, periods.Today.StartAt.Format(time.RFC3339))
			require.Equal(t, tc.week, periods.Last7Days.StartAt.Format(time.RFC3339))
			require.Equal(t, now.UTC(), periods.Today.EndAt)
			require.Equal(t, tc.zone, periods.Timezone)
		})
	}
}

func TestSupplierConsumptionCalculationAndCoverage(t *testing.T) {
	start := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	end := start.Add(12 * time.Hour)
	base := supplierConsumptionObservation{
		supplierID: 1, enabled: true, active: true, queryStatus: "ok",
		openingAt: sql.NullTime{Time: start, Valid: true}, closingAt: sql.NullTime{Time: end.Add(-time.Minute), Valid: true},
		openingBalance: sql.NullFloat64{Float64: 100, Valid: true}, closingBalance: sql.NullFloat64{Float64: 90, Valid: true},
		openingUnit: sql.NullString{String: "USD", Valid: true}, closingUnit: sql.NullString{String: "USD", Valid: true},
		credited: 20,
	}
	for _, tc := range []struct {
		name              string
		change            func(*supplierConsumptionObservation)
		amount            float64
		covered, complete int
		issues            []string
	}{
		{name: "include credited quota rather than cash paid", amount: 30, covered: 1, complete: 1},
		{name: "zero is valid consumption", change: func(s *supplierConsumptionObservation) { s.closingBalance.Float64 = 120 }, covered: 1, complete: 1},
		{name: "single sample is unknown rather than zero", change: func(s *supplierConsumptionObservation) { s.closingAt = s.openingAt }, issues: []string{"no_history"}},
		{name: "no successful balance yet", change: func(s *supplierConsumptionObservation) { s.openingAt.Valid = false }, issues: []string{"no_history"}},
		{name: "partial first day", change: func(s *supplierConsumptionObservation) { s.openingAt.Time = start.Add(time.Hour) }, amount: 30, covered: 1, issues: []string{"insufficient_history"}},
		{name: "outage keeps successful samples but reports incompleteness", change: func(s *supplierConsumptionObservation) {
			s.queryStatus = "error"
			s.closingAt.Time = end.Add(-time.Hour)
		}, amount: 30, covered: 1, issues: []string{"query_failed", "stale_balance"}},
		{name: "unit mismatch is not convertible", change: func(s *supplierConsumptionObservation) { s.incompatibleCredits = true }, issues: []string{"unit_mismatch"}},
		{name: "Credits quota equals USD quota", change: func(s *supplierConsumptionObservation) { s.closingUnit.String = "Credits" }, amount: 30, covered: 1, complete: 1},
		{name: "unknown unit remains incompatible", change: func(s *supplierConsumptionObservation) { s.closingUnit.String = "EUR" }, issues: []string{"unit_mismatch"}},
		{name: "unrecorded credit never becomes negative usage", change: func(s *supplierConsumptionObservation) { s.closingBalance.Float64 = 150 }, issues: []string{"unrecorded_credit"}},
		{name: "disabled collection is visible with historical figures", change: func(s *supplierConsumptionObservation) { s.enabled = false }, amount: 30, covered: 1, issues: []string{"not_configured"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sample := base
			if tc.change != nil {
				tc.change(&sample)
			}
			period := newSupplierConsumptionPeriod(start, end)
			addSupplierConsumption(&period, sample)
			require.Equal(t, 1, period.SupplierCount)
			require.Equal(t, tc.covered, period.CoveredSupplierCount)
			require.Equal(t, tc.complete, period.CompleteSupplierCount)
			var reasons []string
			for _, issue := range period.Issues {
				reasons = append(reasons, issue.Reason)
			}
			require.Equal(t, tc.issues, reasons)
			if tc.covered == 0 {
				require.Empty(t, period.Totals)
			} else {
				require.Equal(t, tc.amount, period.Totals[0].Amount)
				require.Equal(t, "USD", period.Totals[0].Unit)
			}
		})
	}
}

func TestSupplierConsumptionQueryCombinesEquivalentWalletUnitsAndKeepsPeriodsSeparate(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)
	now := time.Now()
	start := now.Add(-2 * time.Hour)
	mock.ExpectQuery("FROM upstream_supplier_balance_samples h").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(supplierConsumptionRows().
			AddRow("today", 1, true, true, "ok", start, 100, "USD", now, 70, "USD", 20, false).
			AddRow("today", 2, true, true, "ok", start, 80, "Credits", now, 60, "Credits", 5, false).
			AddRow("last_7_days", 1, true, true, "ok", start, 100, "USD", now, 70, "USD", 20, false))
	result, err := svc.getUpstreamSupplierConsumptionOverview(context.Background(), now)
	require.NoError(t, err)
	require.Equal(t, []UpstreamSupplierConsumptionTotal{{Unit: "USD", Amount: 75, SupplierCount: 2}}, result.Today.Totals)
	require.Equal(t, 2, result.Today.CoveredSupplierCount)
	require.Equal(t, []UpstreamSupplierConsumptionTotal{{Unit: "USD", Amount: 50, SupplierCount: 1}}, result.Last7Days.Totals)
	require.Equal(t, 1, result.Last7Days.CoveredSupplierCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSupplierConsumptionQueryFailureIsNotEmptySuccess(t *testing.T) {
	svc, mock := newUpstreamSupplierRechargeAnalyticsService(t)
	mock.ExpectQuery("FROM upstream_supplier_balance_samples h").WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnError(errors.New("offline"))
	result, err := svc.getUpstreamSupplierConsumptionOverview(context.Background(), time.Now())
	require.Nil(t, result)
	require.ErrorContains(t, err, "query supplier wallet consumption: offline")
	require.NoError(t, mock.ExpectationsWereMet())
}
