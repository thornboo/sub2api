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
	CostPoolID              *int64    `json:"cost_pool_id,omitempty"`
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
	Items      []UpstreamRechargeRecord `json:"items"`
	Summary    UpstreamRechargeSummary  `json:"summary"`
	CostPoolID *int64                   `json:"cost_pool_id,omitempty"`
	Deprecated bool                     `json:"deprecated,omitempty"`
}

type UpstreamRechargeRecordInput struct {
	AccountID              int64
	CostPoolID             int64
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
	costPoolID, err := s.findActiveUpstreamCostPoolIDForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}

	items, err := s.listUpstreamRechargeRecords(ctx, accountID, costPoolID)
	if err != nil {
		return nil, err
	}

	summary, err := s.loadUpstreamRechargeSummary(ctx, accountID, costPoolID)
	if err != nil {
		return nil, err
	}

	return &UpstreamRechargeRecordsResult{
		Items:      items,
		Summary:    summary,
		CostPoolID: costPoolID,
		Deprecated: costPoolID != nil,
	}, nil
}

func (s *adminServiceImpl) listUpstreamRechargeRecords(ctx context.Context, accountID int64, costPoolID *int64) ([]UpstreamRechargeRecord, error) {
	query := `
SELECT id,
       account_id,
       cost_pool_id,
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
  AND voided_at IS NULL
ORDER BY recorded_at DESC, id DESC
LIMIT $2`
	args := []any{accountID, upstreamRechargeRecordLimit}
	if costPoolID != nil {
		query = `
SELECT id,
       account_id,
       cost_pool_id,
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
WHERE (cost_pool_id = $1 OR (cost_pool_id IS NULL AND account_id = $2))
  AND deleted_at IS NULL
  AND voided_at IS NULL
ORDER BY recorded_at DESC, id DESC
LIMIT $3`
		args = []any{*costPoolID, accountID, upstreamRechargeRecordLimit}
	}

	rows, err := s.entClient.QueryContext(ctx, query, args...)
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

	return items, nil
}

func (s *adminServiceImpl) loadUpstreamRechargeSummary(ctx context.Context, accountID int64, costPoolID *int64) (UpstreamRechargeSummary, error) {
	query := `
WITH base AS (
    SELECT *
    FROM upstream_recharge_records
    WHERE account_id = $1
      AND deleted_at IS NULL
      AND voided_at IS NULL
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
      AND type IN ('recharge', 'bonus')
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
LEFT JOIN latest_cost ON true`
	args := []any{accountID, UpstreamRechargeDefaultReferenceFXRate}
	if costPoolID != nil {
		query = `
WITH base AS (
    SELECT *
    FROM upstream_recharge_records
    WHERE (cost_pool_id = $1 OR (cost_pool_id IS NULL AND account_id = $2))
      AND deleted_at IS NULL
      AND voided_at IS NULL
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
      AND type IN ('recharge', 'bonus')
    ORDER BY recorded_at DESC, id DESC
    LIMIT 1
)
SELECT totals.record_count,
       totals.total_paid_amount,
       totals.total_received_credit_amount,
       latest_record.recorded_at,
       latest_cost.latest_effective_cny_per_usd,
       latest_cost.latest_recharge_discount,
       COALESCE(latest_cost.reference_fx_rate, $3)::double precision
FROM totals
LEFT JOIN latest_record ON true
LEFT JOIN latest_cost ON true`
		args = []any{*costPoolID, accountID, UpstreamRechargeDefaultReferenceFXRate}
	}

	rows, err := s.entClient.QueryContext(ctx, query, args...)
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
	var account *Account
	if input.AccountID > 0 {
		var err error
		account, err = s.accountRepo.GetByID(ctx, input.AccountID)
		if err != nil {
			return nil, err
		}
	}
	values, err := normalizeUpstreamRechargeRecordInput(input)
	if err != nil {
		return nil, err
	}
	costPoolID := input.CostPoolID
	if costPoolID <= 0 {
		if account == nil {
			return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "account id is required when cost pool id is not provided")
		}
		costPoolID, err = s.requireActiveUpstreamCostPoolForAccount(ctx, account)
		if err != nil {
			return nil, err
		}
	} else if err := s.ensureUpstreamCostPoolExists(ctx, costPoolID); err != nil {
		return nil, err
	}

	var (
		accountID       *int64
		accountName     string
		accountPlatform string
		accountType     string
	)
	if account != nil {
		accountID = &account.ID
		accountName = account.Name
		accountPlatform = account.Platform
		accountType = account.Type
	}

	rows, err := s.entClient.QueryContext(ctx, `
WITH pool_lock AS (
    SELECT pg_advisory_xact_lock($17::bigint)
),
new_record AS (
    INSERT INTO upstream_recharge_records (
        account_id,
        cost_pool_id,
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
    )
    SELECT $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
    FROM pool_lock
    RETURNING id,
              account_id,
              cost_pool_id,
              account_name_snapshot,
              account_platform_snapshot,
              account_type_snapshot,
              type,
              paid_amount::double precision AS paid_amount,
              paid_currency,
              received_credit_amount::double precision AS received_credit_amount,
              received_credit_currency,
              reference_fx_rate::double precision AS reference_fx_rate,
              effective_cny_per_usd::double precision AS effective_cny_per_usd,
              recharge_discount::double precision AS recharge_discount,
              recorded_at,
              note,
              created_by,
              created_at,
              updated_at
),
closed_snapshot AS (
    UPDATE upstream_cost_snapshots
    SET valid_to = NOW()
    WHERE cost_pool_id = $2
      AND valid_to IS NULL
      AND EXISTS (
          SELECT 1
          FROM new_record
          WHERE effective_cny_per_usd IS NOT NULL
			AND type = 'recharge'
      )
    RETURNING id
),
new_snapshot AS (
    INSERT INTO upstream_cost_snapshots (
        cost_pool_id,
        effective_cny_per_usd,
        reference_fx_rate,
        calculation_method,
        source_record_id,
        included_record_ids,
        created_by,
        note
    )
    SELECT $2,
           effective_cny_per_usd,
           reference_fx_rate,
           'latest',
           id,
           jsonb_build_array(id),
           $16::bigint,
           '新增资金池账本后生成的最新成本快照。'
    FROM new_record
    WHERE effective_cny_per_usd IS NOT NULL
	  AND type = 'recharge'
      AND (SELECT COUNT(*) FROM closed_snapshot) >= 0
    RETURNING id, cost_pool_id, effective_cny_per_usd::double precision AS effective_cny_per_usd, reference_fx_rate::double precision AS reference_fx_rate
),
updated_pool AS (
    UPDATE upstream_cost_pools pool
    SET current_snapshot_id = snapshot.id,
        current_effective_cny_per_usd = snapshot.effective_cny_per_usd,
        reference_fx_rate = snapshot.reference_fx_rate,
        cost_method = 'latest',
        updated_at = NOW()
    FROM new_snapshot snapshot
    WHERE pool.id = snapshot.cost_pool_id
    RETURNING pool.id
)
SELECT id,
       account_id,
       cost_pool_id,
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
       created_by,
       created_at,
       updated_at
FROM new_record`,
		nullableInt64(accountID),
		costPoolID,
		accountName,
		accountPlatform,
		accountType,
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
		upstreamCostPoolSnapshotAdvisoryLockBase+costPoolID,
	)
	if err != nil {
		return nil, err
	}
	record, scanErr := scanSingleUpstreamRechargeRecord(rows)
	closeErr := rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	s.refreshSchedulerAccountSnapshotsForCostPool(ctx, costPoolID)
	return record, nil
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

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	rows, err := txClient.QueryContext(ctx, `
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
  AND voided_at IS NULL
RETURNING id,
          account_id,
          cost_pool_id,
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
	record, err := scanSingleUpstreamRechargeRecord(rows)
	closeErr := rows.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	if record.CostPoolID != nil {
		if err := refreshLatestUpstreamCostSnapshotForPool(ctx, txClient, *record.CostPoolID, input.CreatedBy); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if record.CostPoolID != nil {
		s.refreshSchedulerAccountSnapshotsForCostPool(ctx, *record.CostPoolID)
	}
	return record, nil
}

func (s *adminServiceImpl) DeleteUpstreamRechargeRecord(ctx context.Context, accountID, recordID int64) error {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return err
	}
	if accountID <= 0 || recordID <= 0 {
		return infraerrors.BadRequest("INVALID_RECORD_ID", "invalid record id")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	rows, err := txClient.QueryContext(ctx, `
UPDATE upstream_recharge_records
SET deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1
  AND account_id = $2
  AND deleted_at IS NULL
  AND voided_at IS NULL
RETURNING cost_pool_id`, recordID, accountID)
	if err != nil {
		return err
	}

	var (
		costPoolID sql.NullInt64
		found      bool
	)
	if rows.Next() {
		found = true
		if err := rows.Scan(&costPoolID); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	closeErr := rows.Close()
	if closeErr != nil {
		return closeErr
	}
	if !found {
		return ErrUpstreamRechargeRecordNotFound
	}
	if costPoolID.Valid {
		if err := refreshLatestUpstreamCostSnapshotForPool(ctx, txClient, costPoolID.Int64, nil); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if costPoolID.Valid {
		s.refreshSchedulerAccountSnapshotsForCostPool(ctx, costPoolID.Int64)
	}
	return nil
}

func refreshLatestUpstreamCostSnapshotForPool(ctx context.Context, exec upstreamCostPoolSQLExecutor, poolID int64, createdBy *int64) error {
	rows, err := exec.QueryContext(ctx, `
WITH pool_lock AS (
    SELECT pg_advisory_xact_lock($2::bigint)
),
latest_record AS (
    SELECT record.id,
           record.effective_cny_per_usd,
           record.reference_fx_rate
    FROM upstream_recharge_records record, pool_lock
    WHERE record.cost_pool_id = $1
      AND record.deleted_at IS NULL
      AND record.voided_at IS NULL
	  AND record.type = 'recharge'
      AND record.effective_cny_per_usd IS NOT NULL
    ORDER BY record.recorded_at DESC, record.id DESC
    LIMIT 1
),
current_snapshot AS (
    SELECT snapshot.id,
           snapshot.source_record_id,
           snapshot.effective_cny_per_usd,
           snapshot.reference_fx_rate
    FROM upstream_cost_snapshots snapshot, pool_lock
    WHERE snapshot.cost_pool_id = $1
      AND snapshot.valid_to IS NULL
    ORDER BY snapshot.id DESC
    LIMIT 1
),
stale_snapshot AS (
    SELECT current_snapshot.id
    FROM current_snapshot
    LEFT JOIN latest_record ON true
    WHERE latest_record.id IS NULL
       OR current_snapshot.source_record_id IS DISTINCT FROM latest_record.id
       OR current_snapshot.effective_cny_per_usd IS DISTINCT FROM latest_record.effective_cny_per_usd
       OR current_snapshot.reference_fx_rate IS DISTINCT FROM latest_record.reference_fx_rate
),
closed_snapshot AS (
    UPDATE upstream_cost_snapshots snapshot
    SET valid_to = NOW()
    FROM stale_snapshot
    WHERE snapshot.id = stale_snapshot.id
    RETURNING snapshot.id
),
new_snapshot AS (
    INSERT INTO upstream_cost_snapshots (
        cost_pool_id,
        effective_cny_per_usd,
        reference_fx_rate,
        calculation_method,
        source_record_id,
        included_record_ids,
        created_by,
        note
    )
    SELECT $1,
           latest_record.effective_cny_per_usd,
           latest_record.reference_fx_rate,
           'latest',
           latest_record.id,
           jsonb_build_array(latest_record.id),
           $3::bigint,
           '资金池账本更新后重建的最新成本快照。'
    FROM latest_record
    WHERE NOT EXISTS (
        SELECT 1
        FROM current_snapshot
        WHERE current_snapshot.source_record_id IS NOT DISTINCT FROM latest_record.id
          AND current_snapshot.effective_cny_per_usd IS NOT DISTINCT FROM latest_record.effective_cny_per_usd
          AND current_snapshot.reference_fx_rate IS NOT DISTINCT FROM latest_record.reference_fx_rate
    )
      AND (SELECT COUNT(*) FROM closed_snapshot) >= 0
    RETURNING id,
              effective_cny_per_usd,
              reference_fx_rate
),
active_snapshot AS (
    SELECT id,
           effective_cny_per_usd,
           reference_fx_rate
    FROM new_snapshot
    UNION ALL
    SELECT current_snapshot.id,
           current_snapshot.effective_cny_per_usd,
           current_snapshot.reference_fx_rate
    FROM current_snapshot
    JOIN latest_record ON true
    WHERE current_snapshot.source_record_id IS NOT DISTINCT FROM latest_record.id
      AND current_snapshot.effective_cny_per_usd IS NOT DISTINCT FROM latest_record.effective_cny_per_usd
      AND current_snapshot.reference_fx_rate IS NOT DISTINCT FROM latest_record.reference_fx_rate
    LIMIT 1
),
updated_pool_with_cost AS (
    UPDATE upstream_cost_pools pool
    SET current_snapshot_id = active_snapshot.id,
        current_effective_cny_per_usd = active_snapshot.effective_cny_per_usd,
        reference_fx_rate = active_snapshot.reference_fx_rate,
        cost_method = 'latest',
        updated_at = NOW()
    FROM active_snapshot
    WHERE pool.id = $1
    RETURNING pool.id
),
cleared_pool AS (
    UPDATE upstream_cost_pools pool
    SET current_snapshot_id = NULL,
        current_effective_cny_per_usd = NULL,
        updated_at = NOW()
    WHERE pool.id = $1
      AND NOT EXISTS (SELECT 1 FROM latest_record)
    RETURNING pool.id
)
SELECT COALESCE(
    (SELECT id FROM updated_pool_with_cost),
    (SELECT id FROM cleared_pool),
    $1
)`,
		poolID,
		upstreamCostPoolSnapshotAdvisoryLockBase+poolID,
		nullableInt64(createdBy),
	)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var refreshedPoolID int64
		if err := rows.Scan(&refreshedPoolID); err != nil {
			return err
		}
		return rows.Err()
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return ErrUpstreamCostPoolNotFound
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
	if recordType == "recharge" && paidAmount > 0 && receivedAmount > 0 {
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
		record     UpstreamRechargeRecord
		accountID  sql.NullInt64
		costPoolID sql.NullInt64
		effective  sql.NullFloat64
		discount   sql.NullFloat64
		note       sql.NullString
		createdBy  sql.NullInt64
	)
	if err := scanner.Scan(
		&record.ID,
		&accountID,
		&costPoolID,
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
	if costPoolID.Valid {
		record.CostPoolID = &costPoolID.Int64
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
