package service

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

// Consumption is an estimate from wallet observations and recorded credits.
// It deliberately does not depend on request costs or upstream usage counters.
type UpstreamSupplierConsumptionOverview struct {
	Timezone  string                            `json:"timezone"`
	Today     UpstreamSupplierConsumptionPeriod `json:"today"`
	Last7Days UpstreamSupplierConsumptionPeriod `json:"last_7_days"`
}

type UpstreamSupplierConsumptionPeriod struct {
	StartAt               time.Time                          `json:"start_at"`
	EndAt                 time.Time                          `json:"end_at"`
	Totals                []UpstreamSupplierConsumptionTotal `json:"totals"`
	SupplierCount         int                                `json:"supplier_count"`
	CoveredSupplierCount  int                                `json:"covered_supplier_count"`
	CompleteSupplierCount int                                `json:"complete_supplier_count"`
	Issues                []UpstreamSupplierConsumptionIssue `json:"issues"`
}

type UpstreamSupplierConsumptionTotal struct {
	Unit          string  `json:"unit"`
	Amount        float64 `json:"amount"`
	SupplierCount int     `json:"supplier_count"`
}

type UpstreamSupplierConsumptionIssue struct {
	SupplierID int64  `json:"supplier_id"`
	Reason     string `json:"reason"`
}

type supplierConsumptionObservation struct {
	supplierID                     int64
	enabled, active                bool
	queryStatus                    string
	openingAt, closingAt           sql.NullTime
	openingBalance, closingBalance sql.NullFloat64
	openingUnit, closingUnit       sql.NullString
	credited                       float64
	incompatibleCredits            bool
}

const supplierConsumptionFreshness = 15 * time.Minute

func newSupplierConsumptionPeriod(start, end time.Time) UpstreamSupplierConsumptionPeriod {
	return UpstreamSupplierConsumptionPeriod{
		StartAt: start, EndAt: end,
		Totals: make([]UpstreamSupplierConsumptionTotal, 0),
		Issues: make([]UpstreamSupplierConsumptionIssue, 0),
	}
}

func supplierConsumptionPeriods(now time.Time, location *time.Location) *UpstreamSupplierConsumptionOverview {
	local := now.In(location)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return &UpstreamSupplierConsumptionOverview{
		Timezone:  location.String(),
		Today:     newSupplierConsumptionPeriod(today.UTC(), now.UTC()),
		Last7Days: newSupplierConsumptionPeriod(today.AddDate(0, 0, -6).UTC(), now.UTC()),
	}
}

// Pick the last observation within ten minutes before the boundary, or the
// first one after it. The latter is explicitly partial history. Credits use
// exactly the same (opening, closing] interval as the observed wallet change.
// The revision join prevents combining balances for different credentials.
// This installation treats Credits and USD wallet quota as equivalent at 1:1;
// only these units are eligible, independent of actual recharge payment currency.
const supplierConsumptionQuery = `
WITH periods (period, start_at, end_at) AS (
    VALUES ('today', $1::timestamptz, $3::timestamptz),
           ('last_7_days', $2::timestamptz, $3::timestamptz)
)
SELECT periods.period, s.id,
       COALESCE(s.balance_config->>'enabled', 'false') = 'true',
       s.status = 'active', COALESCE(s.balance_snapshot->>'status', ''),
       opening.sampled_at, opening.balance::double precision, opening.unit,
       closing.sampled_at, closing.balance::double precision, closing.unit,
       COALESCE(credits.amount, 0)::double precision,
       COALESCE(credits.incompatible, FALSE)
FROM upstream_suppliers s CROSS JOIN periods
LEFT JOIN LATERAL (
    SELECT h.sampled_at, h.balance, h.unit
    FROM upstream_supplier_balance_samples h
    WHERE h.supplier_id = s.id AND h.revision = s.balance_revision
      AND h.sampled_at >= periods.start_at - INTERVAL '10 minutes'
      AND h.sampled_at <= periods.end_at
    ORDER BY (h.sampled_at <= periods.start_at) DESC,
             CASE WHEN h.sampled_at <= periods.start_at THEN h.sampled_at END DESC,
             h.sampled_at ASC, h.id DESC
    LIMIT 1
) opening ON TRUE
LEFT JOIN LATERAL (
    SELECT h.sampled_at, h.balance, h.unit
    FROM upstream_supplier_balance_samples h
    WHERE h.supplier_id = s.id AND h.revision = s.balance_revision
      AND h.sampled_at >= opening.sampled_at AND h.sampled_at <= periods.end_at
    ORDER BY h.sampled_at DESC, h.id DESC LIMIT 1
) closing ON TRUE
LEFT JOIN LATERAL (
    SELECT SUM(r.received_credit_amount) FILTER (
               WHERE UPPER(TRIM(r.received_credit_currency)) IN ('USD', 'CREDITS')) AS amount,
           BOOL_OR(r.received_credit_amount <> 0 AND
                   UPPER(TRIM(r.received_credit_currency)) NOT IN ('USD', 'CREDITS')) AS incompatible
    FROM upstream_recharge_records r
    JOIN upstream_cost_pools p ON p.id = r.cost_pool_id
    WHERE p.supplier_id = s.id AND r.deleted_at IS NULL AND r.voided_at IS NULL
      AND r.recorded_at > opening.sampled_at AND r.recorded_at <= closing.sampled_at
) credits ON TRUE
WHERE s.is_system = FALSE
ORDER BY s.id, periods.period`

func (s *adminServiceImpl) getUpstreamSupplierConsumptionOverview(ctx context.Context, now time.Time) (*UpstreamSupplierConsumptionOverview, error) {
	result := supplierConsumptionPeriods(now, timezone.Location())
	rows, err := s.entClient.QueryContext(ctx, supplierConsumptionQuery, result.Today.StartAt, result.Last7Days.StartAt, result.Today.EndAt)
	if err != nil {
		return nil, fmt.Errorf("query supplier wallet consumption: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var period string
		var sample supplierConsumptionObservation
		if err := rows.Scan(&period, &sample.supplierID, &sample.enabled, &sample.active, &sample.queryStatus,
			&sample.openingAt, &sample.openingBalance, &sample.openingUnit,
			&sample.closingAt, &sample.closingBalance, &sample.closingUnit,
			&sample.credited, &sample.incompatibleCredits); err != nil {
			return nil, fmt.Errorf("scan supplier wallet consumption: %w", err)
		}
		if period == "today" {
			addSupplierConsumption(&result.Today, sample)
		} else {
			addSupplierConsumption(&result.Last7Days, sample)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read supplier wallet consumption: %w", err)
	}
	for _, period := range []*UpstreamSupplierConsumptionPeriod{&result.Today, &result.Last7Days} {
		for i := range period.Totals {
			period.Totals[i].Amount = math.Round(period.Totals[i].Amount*1e8) / 1e8
		}
	}
	return result, nil
}

func addSupplierConsumption(period *UpstreamSupplierConsumptionPeriod, sample supplierConsumptionObservation) {
	period.SupplierCount++
	issuesBefore := len(period.Issues)
	issue := func(reason string) {
		period.Issues = append(period.Issues, UpstreamSupplierConsumptionIssue{SupplierID: sample.supplierID, Reason: reason})
	}
	if !sample.enabled || !sample.active {
		issue("not_configured")
	}
	if sample.queryStatus == "error" {
		issue("query_failed")
	}
	if !sample.openingAt.Valid || !sample.closingAt.Valid || !sample.openingBalance.Valid || !sample.closingBalance.Valid ||
		!sample.closingAt.Time.After(sample.openingAt.Time) || sample.closingAt.Time.Before(period.StartAt) {
		issue("no_history")
		return
	}
	if sample.openingAt.Time.After(period.StartAt) {
		issue("insufficient_history")
	}
	if period.EndAt.Sub(sample.closingAt.Time) > supplierConsumptionFreshness {
		issue("stale_balance")
	}
	unit := supplierConsumptionUnit(sample.openingUnit.String)
	if !sample.openingUnit.Valid || !sample.closingUnit.Valid || unit == "" ||
		unit != supplierConsumptionUnit(sample.closingUnit.String) || sample.incompatibleCredits {
		issue("unit_mismatch")
		return
	}
	amount := sample.openingBalance.Float64 + sample.credited - sample.closingBalance.Float64
	// A balance increase not explained by recorded credits is not negative usage.
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount < -1e-8 {
		issue("unrecorded_credit")
		return
	}
	amount = math.Max(amount, 0)
	period.CoveredSupplierCount++
	if len(period.Issues) == issuesBefore {
		period.CompleteSupplierCount++
	}
	for i := range period.Totals {
		if period.Totals[i].Unit == unit {
			period.Totals[i].Amount += amount
			period.Totals[i].SupplierCount++
			return
		}
	}
	period.Totals = append(period.Totals, UpstreamSupplierConsumptionTotal{Unit: unit, Amount: amount, SupplierCount: 1})
}

// Normalize only consumption totals. Keep original wallet units in stored samples
// and supplier balances so administrators can still check the upstream response.
func supplierConsumptionUnit(unit string) string {
	switch strings.ToUpper(strings.TrimSpace(unit)) {
	case "USD", "CREDITS":
		return "USD"
	default:
		return ""
	}
}
