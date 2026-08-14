package service

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type UpstreamRechargeTrendGranularity string

const (
	UpstreamRechargeTrendGranularityDay   UpstreamRechargeTrendGranularity = "day"
	UpstreamRechargeTrendGranularityWeek  UpstreamRechargeTrendGranularity = "week"
	UpstreamRechargeTrendGranularityMonth UpstreamRechargeTrendGranularity = "month"
	UpstreamRechargeTrendGranularityYear  UpstreamRechargeTrendGranularity = "year"
)

var ErrInvalidUpstreamRechargeTrendGranularity = infraerrors.BadRequest(
	"INVALID_UPSTREAM_RECHARGE_TREND_GRANULARITY",
	"recharge trend granularity must be one of day, week, month, year",
)

type UpstreamRechargePaymentTotal struct {
	Currency    string  `json:"currency"`
	Amount      float64 `json:"amount"`
	RecordCount int     `json:"record_count"`
}

type UpstreamSupplierRechargeTotal struct {
	SupplierID int64                          `json:"supplier_id"`
	Totals     []UpstreamRechargePaymentTotal `json:"totals"`
}

type UpstreamSupplierRechargeOverview struct {
	Totals    []UpstreamRechargePaymentTotal  `json:"totals"`
	Suppliers []UpstreamSupplierRechargeTotal `json:"suppliers"`
}

type UpstreamSupplierRechargeTrendPoint struct {
	Period      string  `json:"period"`
	Currency    string  `json:"currency"`
	Amount      float64 `json:"amount"`
	RecordCount int     `json:"record_count"`
}

type UpstreamSupplierRechargeTrend struct {
	SupplierID  int64                                `json:"supplier_id"`
	Granularity UpstreamRechargeTrendGranularity     `json:"granularity"`
	Totals      []UpstreamRechargePaymentTotal       `json:"totals"`
	Points      []UpstreamSupplierRechargeTrendPoint `json:"points"`
}

func (s *adminServiceImpl) GetUpstreamSupplierRechargeOverview(ctx context.Context) (*UpstreamSupplierRechargeOverview, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}

	suppliers, err := s.queryUpstreamSupplierRechargeTotals(ctx)
	if err != nil {
		return nil, err
	}

	return &UpstreamSupplierRechargeOverview{
		Totals:    aggregateUpstreamSupplierRechargeTotals(suppliers),
		Suppliers: suppliers,
	}, nil
}

func (s *adminServiceImpl) GetUpstreamSupplierRechargeTrend(
	ctx context.Context,
	supplierID int64,
	requestedGranularity UpstreamRechargeTrendGranularity,
) (*UpstreamSupplierRechargeTrend, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	granularity, periodFormat, err := resolveUpstreamRechargeTrendGranularity(requestedGranularity)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUpstreamSupplierVisibleForRechargeAnalytics(ctx, supplierID); err != nil {
		return nil, err
	}

	totals, err := s.queryUpstreamRechargePaymentTotals(ctx, supplierID)
	if err != nil {
		return nil, err
	}
	points, err := s.queryUpstreamSupplierRechargeTrendPoints(ctx, supplierID, granularity, periodFormat)
	if err != nil {
		return nil, err
	}

	return &UpstreamSupplierRechargeTrend{
		SupplierID:  supplierID,
		Granularity: granularity,
		Totals:      totals,
		Points:      points,
	}, nil
}

func resolveUpstreamRechargeTrendGranularity(
	requested UpstreamRechargeTrendGranularity,
) (UpstreamRechargeTrendGranularity, string, error) {
	granularity := UpstreamRechargeTrendGranularity(strings.ToLower(strings.TrimSpace(string(requested))))
	if granularity == "" {
		granularity = UpstreamRechargeTrendGranularityMonth
	}

	switch granularity {
	case UpstreamRechargeTrendGranularityDay:
		return granularity, "YYYY-MM-DD", nil
	case UpstreamRechargeTrendGranularityWeek:
		return granularity, `IYYY-"W"IW`, nil
	case UpstreamRechargeTrendGranularityMonth:
		return granularity, "YYYY-MM", nil
	case UpstreamRechargeTrendGranularityYear:
		return granularity, "YYYY", nil
	default:
		return "", "", ErrInvalidUpstreamRechargeTrendGranularity
	}
}

func (s *adminServiceImpl) ensureUpstreamSupplierVisibleForRechargeAnalytics(ctx context.Context, supplierID int64) error {
	if supplierID <= 0 {
		return ErrUpstreamSupplierNotFound
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id
FROM upstream_suppliers
WHERE id = $1
  AND is_system = FALSE`, supplierID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return rows.Err()
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return ErrUpstreamSupplierNotFound
}

func (s *adminServiceImpl) queryUpstreamRechargePaymentTotals(ctx context.Context, supplierID int64) ([]UpstreamRechargePaymentTotal, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT r.paid_currency,
       COALESCE(SUM(r.paid_amount), 0)::double precision AS amount,
       COUNT(*)::int AS record_count
FROM upstream_recharge_records r
JOIN upstream_cost_pools p ON p.id = r.cost_pool_id
JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
WHERE p.supplier_id = $1
  AND r.deleted_at IS NULL
  AND r.voided_at IS NULL
  AND supplier.is_system = FALSE
GROUP BY r.paid_currency
ORDER BY r.paid_currency`, supplierID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanUpstreamRechargePaymentTotals(rows)
}

func (s *adminServiceImpl) queryUpstreamSupplierRechargeTotals(ctx context.Context) ([]UpstreamSupplierRechargeTotal, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT p.supplier_id,
       r.paid_currency,
       COALESCE(SUM(r.paid_amount), 0)::double precision AS amount,
       COUNT(*)::int AS record_count
FROM upstream_recharge_records r
JOIN upstream_cost_pools p ON p.id = r.cost_pool_id
JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
WHERE r.deleted_at IS NULL
  AND r.voided_at IS NULL
  AND supplier.is_system = FALSE
GROUP BY p.supplier_id, r.paid_currency
ORDER BY p.supplier_id, r.paid_currency`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	bySupplier := make(map[int64][]UpstreamRechargePaymentTotal)
	order := make([]int64, 0)
	for rows.Next() {
		var (
			supplierID int64
			total      UpstreamRechargePaymentTotal
		)
		if err := rows.Scan(&supplierID, &total.Currency, &total.Amount, &total.RecordCount); err != nil {
			return nil, err
		}
		if _, ok := bySupplier[supplierID]; !ok {
			order = append(order, supplierID)
		}
		bySupplier[supplierID] = append(bySupplier[supplierID], total)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	items := make([]UpstreamSupplierRechargeTotal, 0, len(order))
	for _, supplierID := range order {
		items = append(items, UpstreamSupplierRechargeTotal{
			SupplierID: supplierID,
			Totals:     bySupplier[supplierID],
		})
	}
	return items, nil
}

func aggregateUpstreamSupplierRechargeTotals(suppliers []UpstreamSupplierRechargeTotal) []UpstreamRechargePaymentTotal {
	byCurrency := make(map[string]UpstreamRechargePaymentTotal)
	for _, supplier := range suppliers {
		for _, total := range supplier.Totals {
			aggregate := byCurrency[total.Currency]
			aggregate.Currency = total.Currency
			aggregate.Amount += total.Amount
			aggregate.RecordCount += total.RecordCount
			byCurrency[total.Currency] = aggregate
		}
	}

	currencies := make([]string, 0, len(byCurrency))
	for currency := range byCurrency {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)

	totals := make([]UpstreamRechargePaymentTotal, 0, len(currencies))
	for _, currency := range currencies {
		totals = append(totals, byCurrency[currency])
	}
	return totals
}

func (s *adminServiceImpl) queryUpstreamSupplierRechargeTrendPoints(
	ctx context.Context,
	supplierID int64,
	granularity UpstreamRechargeTrendGranularity,
	periodFormat string,
) ([]UpstreamSupplierRechargeTrendPoint, error) {
	rows, err := s.entClient.QueryContext(ctx, `
SELECT to_char(date_trunc($2, r.recorded_at AT TIME ZONE 'UTC'), $3) AS period,
       r.paid_currency,
       COALESCE(SUM(r.paid_amount), 0)::double precision AS amount,
       COUNT(*)::int AS record_count
FROM upstream_recharge_records r
JOIN upstream_cost_pools p ON p.id = r.cost_pool_id
JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
WHERE p.supplier_id = $1
  AND r.deleted_at IS NULL
  AND r.voided_at IS NULL
  AND supplier.is_system = FALSE
GROUP BY period, r.paid_currency
ORDER BY period, r.paid_currency`, supplierID, string(granularity), periodFormat)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	points := make([]UpstreamSupplierRechargeTrendPoint, 0)
	for rows.Next() {
		var point UpstreamSupplierRechargeTrendPoint
		if err := rows.Scan(&point.Period, &point.Currency, &point.Amount, &point.RecordCount); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

func scanUpstreamRechargePaymentTotals(rows *sql.Rows) ([]UpstreamRechargePaymentTotal, error) {
	totals := make([]UpstreamRechargePaymentTotal, 0)
	for rows.Next() {
		var total UpstreamRechargePaymentTotal
		if err := rows.Scan(&total.Currency, &total.Amount, &total.RecordCount); err != nil {
			return nil, err
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return totals, nil
}
