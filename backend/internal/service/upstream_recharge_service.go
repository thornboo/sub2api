package service

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	UpstreamRechargeDefaultReferenceFXRate = 7
	upstreamRechargeRecordLimit            = 100
	upstreamRechargePaidCurrency           = "CNY"
	upstreamRechargeCreditCurrency         = "USD"
)

var ErrUpstreamRechargeRecordNotFound = infraerrors.NotFound("UPSTREAM_RECHARGE_RECORD_NOT_FOUND", "upstream recharge record not found")

type UpstreamRechargeRecord struct {
	ID                      int64     `json:"id"`
	AccountID               *int64    `json:"account_id,omitempty"`
	AccountNameSnapshot     string    `json:"account_name_snapshot"`
	AccountPlatformSnapshot string    `json:"account_platform_snapshot"`
	AccountTypeSnapshot     string    `json:"account_type_snapshot"`
	Type                    string    `json:"type"`
	PaidAmount              float64   `json:"paid_amount"`
	PaidCurrency            string    `json:"paid_currency"`
	ReceivedCreditAmount    float64   `json:"received_credit_amount"`
	ReceivedCreditCurrency  string    `json:"received_credit_currency"`
	ReferenceFXRate         float64   `json:"reference_fx_rate"`
	EffectiveCNYPerUSD      *float64  `json:"effective_cny_per_usd,omitempty"`
	RechargeDiscount        *float64  `json:"recharge_discount,omitempty"`
	RecordedAt              time.Time `json:"recorded_at"`
	Note                    *string   `json:"note,omitempty"`
	CreatedBy               *int64    `json:"created_by,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type UpstreamRechargeSummary struct {
	RecordCount                int        `json:"record_count"`
	TotalPaidAmount            float64    `json:"total_paid_amount"`
	TotalReceivedCreditAmount  float64    `json:"total_received_credit_amount"`
	WeightedEffectiveCNYPerUSD *float64   `json:"weighted_effective_cny_per_usd,omitempty"`
	WeightedRechargeDiscount   *float64   `json:"weighted_recharge_discount,omitempty"`
	LatestEffectiveCNYPerUSD   *float64   `json:"latest_effective_cny_per_usd,omitempty"`
	LatestRechargeDiscount     *float64   `json:"latest_recharge_discount,omitempty"`
	LatestRecordedAt           *time.Time `json:"latest_recorded_at,omitempty"`
	ReferenceFXRate            float64    `json:"reference_fx_rate"`
}

type UpstreamRechargeRecordsResult struct {
	Items   []UpstreamRechargeRecord `json:"items"`
	Summary UpstreamRechargeSummary  `json:"summary"`
}

type UpstreamRechargeRecordInput struct {
	AccountID              int64
	Type                   string
	PaidAmount             float64
	PaidCurrency           string
	ReceivedCreditAmount   float64
	ReceivedCreditCurrency string
	ReferenceFXRate        float64
	RecordedAt             *time.Time
	Note                   *string
	CreatedBy              *int64
}

type upstreamRechargeRecordValues struct {
	Type                   string
	PaidAmount             float64
	PaidCurrency           string
	ReceivedCreditAmount   float64
	ReceivedCreditCurrency string
	ReferenceFXRate        float64
	EffectiveCNYPerUSD     *float64
	RechargeDiscount       *float64
	RecordedAt             time.Time
	Note                   *string
}

func (s *adminServiceImpl) ListUpstreamRechargeRecords(ctx context.Context, accountID int64) (*UpstreamRechargeRecordsResult, error) {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return nil, err
	}
	if accountID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	if _, err := s.accountRepo.GetByID(ctx, accountID); err != nil {
		return nil, err
	}

	rows, err := s.entClient.QueryContext(ctx, `
SELECT id,
       account_id,
       account_name_snapshot,
       account_platform_snapshot,
       account_type_snapshot,
       type,
       paid_amount::double precision,
       paid_currency,
       received_credit_amount::double precision,
       received_credit_currency,
       reference_fx_rate::double precision,
       effective_cny_per_usd::double precision,
       recharge_discount::double precision,
       recorded_at,
       note,
       created_by,
       created_at,
       updated_at
FROM upstream_recharge_records
WHERE account_id = $1
  AND deleted_at IS NULL
ORDER BY recorded_at DESC, id DESC
LIMIT $2`, accountID, upstreamRechargeRecordLimit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]UpstreamRechargeRecord, 0)
	for rows.Next() {
		record, scanErr := scanUpstreamRechargeRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary, err := s.loadUpstreamRechargeSummary(ctx, accountID)
	if err != nil {
		return nil, err
	}

	return &UpstreamRechargeRecordsResult{
		Items:   items,
		Summary: summary,
	}, nil
}

func (s *adminServiceImpl) loadUpstreamRechargeSummary(ctx context.Context, accountID int64) (UpstreamRechargeSummary, error) {
	rows, err := s.entClient.QueryContext(ctx, `
WITH base AS (
    SELECT *
    FROM upstream_recharge_records
    WHERE account_id = $1
      AND deleted_at IS NULL
),
totals AS (
    SELECT COUNT(*)::int AS record_count,
           COALESCE(SUM(paid_amount), 0)::double precision AS total_paid_amount,
           COALESCE(SUM(received_credit_amount), 0)::double precision AS total_received_credit_amount
    FROM base
),
latest_record AS (
    SELECT recorded_at
    FROM base
    ORDER BY recorded_at DESC, id DESC
    LIMIT 1
),
latest_cost AS (
    SELECT effective_cny_per_usd::double precision AS latest_effective_cny_per_usd,
           recharge_discount::double precision AS latest_recharge_discount,
           reference_fx_rate::double precision AS reference_fx_rate
    FROM base
    WHERE effective_cny_per_usd IS NOT NULL
    ORDER BY recorded_at DESC, id DESC
    LIMIT 1
)
SELECT totals.record_count,
       totals.total_paid_amount,
       totals.total_received_credit_amount,
       latest_record.recorded_at,
       latest_cost.latest_effective_cny_per_usd,
       latest_cost.latest_recharge_discount,
       COALESCE(latest_cost.reference_fx_rate, $2)::double precision
FROM totals
LEFT JOIN latest_record ON true
LEFT JOIN latest_cost ON true`, accountID, UpstreamRechargeDefaultReferenceFXRate)
	if err != nil {
		return UpstreamRechargeSummary{}, err
	}
	defer func() { _ = rows.Close() }()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return UpstreamRechargeSummary{}, err
		}
		return UpstreamRechargeSummary{ReferenceFXRate: UpstreamRechargeDefaultReferenceFXRate}, nil
	}

	var (
		summary        UpstreamRechargeSummary
		latestRecorded sql.NullTime
		latestCost     sql.NullFloat64
		latestDiscount sql.NullFloat64
		referenceFX    sql.NullFloat64
	)
	if err := rows.Scan(
		&summary.RecordCount,
		&summary.TotalPaidAmount,
		&summary.TotalReceivedCreditAmount,
		&latestRecorded,
		&latestCost,
		&latestDiscount,
		&referenceFX,
	); err != nil {
		return UpstreamRechargeSummary{}, err
	}
	if err := rows.Err(); err != nil {
		return UpstreamRechargeSummary{}, err
	}

	summary.ReferenceFXRate = UpstreamRechargeDefaultReferenceFXRate
	if referenceFX.Valid && referenceFX.Float64 > 0 {
		summary.ReferenceFXRate = referenceFX.Float64
	}
	if latestRecorded.Valid {
		value := latestRecorded.Time
		summary.LatestRecordedAt = &value
	}
	if latestCost.Valid {
		value := latestCost.Float64
		summary.LatestEffectiveCNYPerUSD = &value
	}
	if latestDiscount.Valid {
		value := latestDiscount.Float64
		summary.LatestRechargeDiscount = &value
	}
	if summary.TotalPaidAmount > 0 && summary.TotalReceivedCreditAmount > 0 {
		weighted := summary.TotalPaidAmount / summary.TotalReceivedCreditAmount
		summary.WeightedEffectiveCNYPerUSD = &weighted
		discount := weighted / summary.ReferenceFXRate
		summary.WeightedRechargeDiscount = &discount
	}

	return summary, nil
}

func (s *adminServiceImpl) CreateUpstreamRechargeRecord(ctx context.Context, input UpstreamRechargeRecordInput) (*UpstreamRechargeRecord, error) {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return nil, err
	}
	account, err := s.accountRepo.GetByID(ctx, input.AccountID)
	if err != nil {
		return nil, err
	}
	values, err := normalizeUpstreamRechargeRecordInput(input)
	if err != nil {
		return nil, err
	}

	rows, err := s.entClient.QueryContext(ctx, `
INSERT INTO upstream_recharge_records (
    account_id,
    account_name_snapshot,
    account_platform_snapshot,
    account_type_snapshot,
    type,
    paid_amount,
    paid_currency,
    received_credit_amount,
    received_credit_currency,
    reference_fx_rate,
    effective_cny_per_usd,
    recharge_discount,
    recorded_at,
    note,
    created_by
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id,
          account_id,
          account_name_snapshot,
          account_platform_snapshot,
          account_type_snapshot,
          type,
          paid_amount::double precision,
          paid_currency,
          received_credit_amount::double precision,
          received_credit_currency,
          reference_fx_rate::double precision,
          effective_cny_per_usd::double precision,
          recharge_discount::double precision,
          recorded_at,
          note,
          created_by,
          created_at,
          updated_at`,
		account.ID,
		account.Name,
		account.Platform,
		account.Type,
		values.Type,
		values.PaidAmount,
		values.PaidCurrency,
		values.ReceivedCreditAmount,
		values.ReceivedCreditCurrency,
		values.ReferenceFXRate,
		nullableFloat(values.EffectiveCNYPerUSD),
		nullableFloat(values.RechargeDiscount),
		values.RecordedAt,
		nullableString(values.Note),
		nullableInt64(input.CreatedBy),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanSingleUpstreamRechargeRecord(rows)
}

func (s *adminServiceImpl) UpdateUpstreamRechargeRecord(ctx context.Context, recordID int64, input UpstreamRechargeRecordInput) (*UpstreamRechargeRecord, error) {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return nil, err
	}
	if recordID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_RECORD_ID", "invalid record id")
	}
	if _, err := s.accountRepo.GetByID(ctx, input.AccountID); err != nil {
		return nil, err
	}
	values, err := normalizeUpstreamRechargeRecordInput(input)
	if err != nil {
		return nil, err
	}

	rows, err := s.entClient.QueryContext(ctx, `
UPDATE upstream_recharge_records
SET type = $3,
    paid_amount = $4,
    paid_currency = $5,
    received_credit_amount = $6,
    received_credit_currency = $7,
    reference_fx_rate = $8,
    effective_cny_per_usd = $9,
    recharge_discount = $10,
    recorded_at = $11,
    note = $12,
    updated_at = NOW()
WHERE id = $1
  AND account_id = $2
  AND deleted_at IS NULL
RETURNING id,
          account_id,
          account_name_snapshot,
          account_platform_snapshot,
          account_type_snapshot,
          type,
          paid_amount::double precision,
          paid_currency,
          received_credit_amount::double precision,
          received_credit_currency,
          reference_fx_rate::double precision,
          effective_cny_per_usd::double precision,
          recharge_discount::double precision,
          recorded_at,
          note,
          created_by,
          created_at,
          updated_at`,
		recordID,
		input.AccountID,
		values.Type,
		values.PaidAmount,
		values.PaidCurrency,
		values.ReceivedCreditAmount,
		values.ReceivedCreditCurrency,
		values.ReferenceFXRate,
		nullableFloat(values.EffectiveCNYPerUSD),
		nullableFloat(values.RechargeDiscount),
		values.RecordedAt,
		nullableString(values.Note),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	return scanSingleUpstreamRechargeRecord(rows)
}

func (s *adminServiceImpl) DeleteUpstreamRechargeRecord(ctx context.Context, accountID, recordID int64) error {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return err
	}
	if accountID <= 0 || recordID <= 0 {
		return infraerrors.BadRequest("INVALID_RECORD_ID", "invalid record id")
	}
	result, err := s.entClient.ExecContext(ctx, `
UPDATE upstream_recharge_records
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND account_id = $2
  AND deleted_at IS NULL`, recordID, accountID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrUpstreamRechargeRecordNotFound
	}
	return nil
}

func (s *adminServiceImpl) ensureUpstreamRechargeServiceAvailable() error {
	if s == nil || s.entClient == nil || s.accountRepo == nil {
		return infraerrors.InternalServer("UPSTREAM_RECHARGE_UNAVAILABLE", "upstream recharge service is unavailable")
	}
	return nil
}

func normalizeUpstreamRechargeRecordInput(input UpstreamRechargeRecordInput) (upstreamRechargeRecordValues, error) {
	recordType := strings.ToLower(strings.TrimSpace(input.Type))
	if recordType == "" {
		recordType = "recharge"
	}
	switch recordType {
	case "recharge", "bonus", "adjustment":
	default:
		return upstreamRechargeRecordValues{}, infraerrors.BadRequest("INVALID_UPSTREAM_RECHARGE_TYPE", "invalid recharge record type")
	}

	paidAmount := normalizeMoney(input.PaidAmount)
	receivedAmount := normalizeMoney(input.ReceivedCreditAmount)
	if paidAmount < 0 || receivedAmount < 0 {
		return upstreamRechargeRecordValues{}, infraerrors.BadRequest("INVALID_UPSTREAM_RECHARGE_AMOUNT", "amount must be greater than or equal to 0")
	}
	if paidAmount == 0 && receivedAmount == 0 {
		return upstreamRechargeRecordValues{}, infraerrors.BadRequest("INVALID_UPSTREAM_RECHARGE_AMOUNT", "paid amount or received credit amount is required")
	}

	referenceFXRate := input.ReferenceFXRate
	if !isPositiveFinite(referenceFXRate) {
		referenceFXRate = UpstreamRechargeDefaultReferenceFXRate
	}

	recordedAt := time.Now().UTC()
	if input.RecordedAt != nil && !input.RecordedAt.IsZero() {
		recordedAt = input.RecordedAt.UTC()
	}

	paidCurrency := normalizeCurrency(input.PaidCurrency, upstreamRechargePaidCurrency)
	receivedCurrency := normalizeCurrency(input.ReceivedCreditCurrency, upstreamRechargeCreditCurrency)
	if paidCurrency != upstreamRechargePaidCurrency || receivedCurrency != upstreamRechargeCreditCurrency {
		return upstreamRechargeRecordValues{}, infraerrors.BadRequest("INVALID_UPSTREAM_RECHARGE_CURRENCY", "upstream recharge records currently only support CNY paid amount and USD received credit")
	}
	note := normalizeOptionalString(input.Note)

	var effective *float64
	var discount *float64
	if paidAmount > 0 && receivedAmount > 0 {
		effectiveValue := paidAmount / receivedAmount
		discountValue := effectiveValue / referenceFXRate
		effective = &effectiveValue
		discount = &discountValue
	}

	return upstreamRechargeRecordValues{
		Type:                   recordType,
		PaidAmount:             paidAmount,
		PaidCurrency:           paidCurrency,
		ReceivedCreditAmount:   receivedAmount,
		ReceivedCreditCurrency: receivedCurrency,
		ReferenceFXRate:        referenceFXRate,
		EffectiveCNYPerUSD:     effective,
		RechargeDiscount:       discount,
		RecordedAt:             recordedAt,
		Note:                   note,
	}, nil
}

type upstreamRechargeScanner interface {
	Scan(dest ...any) error
}

func scanSingleUpstreamRechargeRecord(rows *sql.Rows) (*UpstreamRechargeRecord, error) {
	if rows.Next() {
		return scanUpstreamRechargeRecord(rows)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUpstreamRechargeRecordNotFound
}

func scanUpstreamRechargeRecord(scanner upstreamRechargeScanner) (*UpstreamRechargeRecord, error) {
	var (
		record    UpstreamRechargeRecord
		accountID sql.NullInt64
		effective sql.NullFloat64
		discount  sql.NullFloat64
		note      sql.NullString
		createdBy sql.NullInt64
	)
	if err := scanner.Scan(
		&record.ID,
		&accountID,
		&record.AccountNameSnapshot,
		&record.AccountPlatformSnapshot,
		&record.AccountTypeSnapshot,
		&record.Type,
		&record.PaidAmount,
		&record.PaidCurrency,
		&record.ReceivedCreditAmount,
		&record.ReceivedCreditCurrency,
		&record.ReferenceFXRate,
		&effective,
		&discount,
		&record.RecordedAt,
		&note,
		&createdBy,
		&record.CreatedAt,
		&record.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUpstreamRechargeRecordNotFound
		}
		return nil, err
	}
	if accountID.Valid {
		record.AccountID = &accountID.Int64
	}
	if effective.Valid {
		record.EffectiveCNYPerUSD = &effective.Float64
	}
	if discount.Valid {
		record.RechargeDiscount = &discount.Float64
	}
	if note.Valid {
		record.Note = &note.String
	}
	if createdBy.Valid {
		record.CreatedBy = &createdBy.Int64
	}
	return &record, nil
}

func normalizeMoney(value float64) float64 {
	if !isFinite(value) {
		return 0
	}
	return value
}

func normalizeCurrency(value, fallback string) string {
	currency := strings.ToUpper(strings.TrimSpace(value))
	if currency == "" {
		return fallback
	}
	if len(currency) > 8 {
		return currency[:8]
	}
	return currency
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullableFloat(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func nullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func isPositiveFinite(value float64) bool {
	return value > 0 && isFinite(value)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
