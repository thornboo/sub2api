package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

	"github.com/lib/pq"
)

const (
	upstreamDefaultCostPoolName              = "主余额池"
	upstreamCostPoolAccountAdvisoryLockBase  = int64(1660000000000)
	upstreamCostPoolSnapshotAdvisoryLockBase = int64(1661000000000)
	upstreamCostPoolSupplierAdvisoryLockBase = int64(1662000000000)
)

var (
	ErrUpstreamCostPoolNotFound          = infraerrors.NotFound("UPSTREAM_COST_POOL_NOT_FOUND", "upstream cost pool not found")
	ErrUpstreamSupplierNotFound          = infraerrors.NotFound("UPSTREAM_SUPPLIER_NOT_FOUND", "upstream supplier not found")
	ErrUpstreamCostBindingNotFound       = infraerrors.NotFound("UPSTREAM_COST_BINDING_NOT_FOUND", "upstream cost binding not found")
	ErrUpstreamSupplierNameConflict      = infraerrors.Conflict("SUPPLIER_NAME_CONFLICT", "another active upstream supplier already uses this name")
	ErrUpstreamSupplierReserved          = infraerrors.Forbidden("SUPPLIER_RESERVED", "system suppliers cannot be modified or deleted")
	ErrUpstreamSupplierHasBoundAccounts  = infraerrors.Conflict("SUPPLIER_HAS_BOUND_ACCOUNTS", "the supplier still has bound accounts; unbind them or archive the supplier instead")
	ErrUpstreamSupplierHasBindingHistory = infraerrors.Conflict("SUPPLIER_HAS_BINDING_HISTORY", "the supplier has account binding history; archive the supplier instead")
	ErrUpstreamSupplierHasCostData       = infraerrors.Conflict("SUPPLIER_HAS_COST_DATA", "the supplier still has cost pools, recharge records or snapshots; archive the supplier instead")
)

type upstreamCostPoolSQLExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type schedulerAccountSnapshotRefresher interface {
	SyncSchedulerAccountSnapshot(context.Context, int64)
}

func refreshSchedulerAccountSnapshot(ctx context.Context, repo AccountRepository, accountID int64) {
	if refresher, ok := repo.(schedulerAccountSnapshotRefresher); ok {
		refresher.SyncSchedulerAccountSnapshot(ctx, accountID)
	}
}

// refreshSchedulerAccountSnapshotsForCostPool is intentionally best-effort,
// matching the existing single-account cache refresh contract. The database is
// already authoritative; this closes the normal stale-cache window after a
// pool's real recharge snapshot changes.
func (s *adminServiceImpl) refreshSchedulerAccountSnapshotsForCostPool(ctx context.Context, poolID int64) {
	if s == nil || s.entClient == nil || poolID <= 0 {
		return
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT DISTINCT account_id
FROM upstream_account_cost_bindings
WHERE cost_pool_id = $1
  AND status = 'active'
  AND valid_to IS NULL`, poolID)
	if err != nil {
		return
	}
	defer func() { _ = rows.Close() }()

	accountIDs := make([]int64, 0)
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return
		}
		accountIDs = append(accountIDs, accountID)
	}
	if rows.Err() != nil {
		return
	}
	for _, accountID := range accountIDs {
		refreshSchedulerAccountSnapshot(ctx, s.accountRepo, accountID)
	}
}

type UpstreamSupplier struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
	Note       *string    `json:"note,omitempty"`
	IsSystem   bool       `json:"is_system"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

type UpstreamCostPool struct {
	ID                        int64          `json:"id"`
	SupplierID                int64          `json:"supplier_id"`
	SupplierName              string         `json:"supplier_name"`
	Name                      string         `json:"name"`
	IsDefault                 bool           `json:"is_default"`
	Status                    string         `json:"status"`
	BaseCurrency              string         `json:"base_currency"`
	CreditCurrency            string         `json:"credit_currency"`
	ReferenceFXRate           float64        `json:"reference_fx_rate"`
	DefaultEffectiveCNYPerUSD float64        `json:"default_effective_cny_per_usd"`
	DefaultReferenceFXRate    float64        `json:"default_reference_fx_rate"`
	CostMethod                string         `json:"cost_method"`
	CurrentEffectiveCNYPerUSD *float64       `json:"current_effective_cny_per_usd,omitempty"`
	CurrentSnapshotID         *int64         `json:"current_snapshot_id,omitempty"`
	BalanceQueryEnabled       bool           `json:"balance_query_enabled"`
	BalanceProvider           *string        `json:"balance_provider,omitempty"`
	BalanceEndpoint           *string        `json:"balance_endpoint,omitempty"`
	BalanceAuthMode           *string        `json:"balance_auth_mode,omitempty"`
	BalanceAuthHeader         *string        `json:"balance_auth_header,omitempty"`
	BalanceLowThreshold       *float64       `json:"balance_low_threshold,omitempty"`
	LastBalanceSnapshot       map[string]any `json:"last_balance_snapshot,omitempty"`
	Note                      *string        `json:"note,omitempty"`
	BindingCount              int            `json:"binding_count"`
	RecordCount               int            `json:"record_count"`
	CreatedAt                 time.Time      `json:"created_at"`
	UpdatedAt                 time.Time      `json:"updated_at"`
	ArchivedAt                *time.Time     `json:"archived_at,omitempty"`
}

type UpstreamCostModelFamilyMultiplier struct {
	Family          string  `json:"family"`
	GroupMultiplier float64 `json:"group_multiplier"`
	Note            *string `json:"note,omitempty"`
}

type UpstreamAccountCostBinding struct {
	ID                      int64                               `json:"id"`
	AccountID               int64                               `json:"account_id"`
	AccountName             string                              `json:"account_name,omitempty"`
	AccountPlatform         string                              `json:"account_platform,omitempty"`
	CostPoolID              int64                               `json:"cost_pool_id"`
	CostPoolName            string                              `json:"cost_pool_name,omitempty"`
	SupplierID              int64                               `json:"supplier_id,omitempty"`
	SupplierName            string                              `json:"supplier_name,omitempty"`
	Status                  string                              `json:"status"`
	DefaultMultiplier       float64                             `json:"default_multiplier"`
	UpstreamGroupName       *string                             `json:"upstream_group_name,omitempty"`
	UpstreamGroupMultiplier float64                             `json:"upstream_group_multiplier"`
	ModelFamilyMultipliers  []UpstreamCostModelFamilyMultiplier `json:"model_family_multipliers"`
	Note                    *string                             `json:"note,omitempty"`
	ValidFrom               time.Time                           `json:"valid_from"`
	ValidTo                 *time.Time                          `json:"valid_to,omitempty"`
	CreatedAt               time.Time                           `json:"created_at"`
	UpdatedAt               time.Time                           `json:"updated_at"`
}

type UpstreamCostBindingInput struct {
	AccountID              int64
	CostPoolID             int64
	UpstreamGroupName      *string
	DefaultMultiplier      float64
	ModelFamilyMultipliers []UpstreamCostModelFamilyMultiplier
	Note                   *string
	CreatedBy              *int64
}

type UpstreamSupplierBindingInput struct {
	AccountID              int64
	SupplierID             int64
	SupplierName           string
	CostPoolID             int64
	Clear                  bool
	UpstreamGroupName      *string
	DefaultMultiplier      float64
	ModelFamilyMultipliers []UpstreamCostModelFamilyMultiplier
	Note                   *string
	CreatedBy              *int64
}

type CreateUpstreamSupplierInput struct {
	Name                      string
	Note                      *string
	DefaultEffectiveCNYPerUSD float64
	DefaultReferenceFXRate    float64
	CreatedBy                 *int64
}

func (s *adminServiceImpl) ListUpstreamSuppliers(ctx context.Context) ([]UpstreamSupplier, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT id, name, status, note, is_system, created_at, updated_at, archived_at
FROM upstream_suppliers
WHERE is_system = FALSE
ORDER BY status ASC, name ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]UpstreamSupplier, 0)
	for rows.Next() {
		item, scanErr := scanUpstreamSupplier(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *adminServiceImpl) CreateUpstreamSupplier(ctx context.Context, input CreateUpstreamSupplierInput) (*UpstreamSupplier, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	note := normalizeOptionalString(input.Note)
	if note == nil {
		note = upstreamCostPoolStringPtr("通过管理端新增到供应商列表。")
	}
	supplierID, err := createUpstreamSupplier(ctx, txClient, input.Name, note, input.CreatedBy)
	if err != nil {
		return nil, err
	}
	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolSupplierAdvisoryLockBase+supplierID); err != nil {
		return nil, err
	}
	poolID, err := ensureDefaultUpstreamCostPoolForSupplier(ctx, txClient, supplierID, input.CreatedBy)
	if err != nil {
		return nil, err
	}
	if input.DefaultEffectiveCNYPerUSD == 0 {
		input.DefaultEffectiveCNYPerUSD = UpstreamRechargeDefaultReferenceFXRate
	}
	if input.DefaultReferenceFXRate == 0 {
		input.DefaultReferenceFXRate = UpstreamRechargeDefaultReferenceFXRate
	}
	defaultEffective, err := normalizeUpstreamCostPoolDefault(
		input.DefaultEffectiveCNYPerUSD,
		"INVALID_UPSTREAM_DEFAULT_EFFECTIVE_COST",
		"default effective CNY per USD must be greater than 0",
	)
	if err != nil {
		return nil, err
	}
	defaultReferenceFX, err := normalizeUpstreamCostPoolDefault(
		input.DefaultReferenceFXRate,
		"INVALID_UPSTREAM_DEFAULT_REFERENCE_FX_RATE",
		"default reference FX rate must be greater than 0",
	)
	if err != nil {
		return nil, err
	}
	if err := updateDefaultUpstreamCostPoolConfig(ctx, txClient, poolID, &defaultEffective, &defaultReferenceFX); err != nil {
		return nil, err
	}

	rows, err := txClient.QueryContext(ctx, `
SELECT id, name, status, note, is_system, created_at, updated_at, archived_at
FROM upstream_suppliers
WHERE id = $1`, supplierID)
	if err != nil {
		return nil, err
	}

	var supplier *UpstreamSupplier
	if rows.Next() {
		supplier, err = scanUpstreamSupplier(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	_ = rows.Close()
	if supplier == nil {
		return nil, ErrUpstreamSupplierNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return supplier, nil
}

// UpdateUpstreamSupplierInput carries optional edits for an upstream supplier.
type UpdateUpstreamSupplierInput struct {
	SupplierID                int64
	Name                      *string
	Note                      *string
	Status                    *string
	DefaultEffectiveCNYPerUSD *float64
	DefaultReferenceFXRate    *float64
}

// UpdateUpstreamSupplier renames, re-notes or archives an upstream supplier.
func (s *adminServiceImpl) UpdateUpstreamSupplier(ctx context.Context, input UpdateUpstreamSupplierInput) (*UpstreamSupplier, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	if input.SupplierID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_ID", "invalid upstream supplier id")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolSupplierAdvisoryLockBase+input.SupplierID); err != nil {
		return nil, err
	}

	current, err := loadUpstreamSupplierForUpdate(ctx, txClient, input.SupplierID)
	if err != nil {
		return nil, err
	}
	if isReservedUpstreamSupplier(current) {
		return nil, ErrUpstreamSupplierReserved
	}

	if err := applyUpstreamSupplierUpdate(ctx, txClient, current, input); err != nil {
		return nil, err
	}
	if input.DefaultEffectiveCNYPerUSD != nil || input.DefaultReferenceFXRate != nil {
		poolID, poolErr := ensureDefaultUpstreamCostPoolForSupplier(ctx, txClient, input.SupplierID, nil)
		if poolErr != nil {
			return nil, poolErr
		}
		var defaultEffective *float64
		if input.DefaultEffectiveCNYPerUSD != nil {
			value, normalizeErr := normalizeUpstreamCostPoolDefault(
				*input.DefaultEffectiveCNYPerUSD,
				"INVALID_UPSTREAM_DEFAULT_EFFECTIVE_COST",
				"default effective CNY per USD must be greater than 0",
			)
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			defaultEffective = &value
		}
		var defaultReferenceFX *float64
		if input.DefaultReferenceFXRate != nil {
			value, normalizeErr := normalizeUpstreamCostPoolDefault(
				*input.DefaultReferenceFXRate,
				"INVALID_UPSTREAM_DEFAULT_REFERENCE_FX_RATE",
				"default reference FX rate must be greater than 0",
			)
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			defaultReferenceFX = &value
		}
		if err := updateDefaultUpstreamCostPoolConfig(ctx, txClient, poolID, defaultEffective, defaultReferenceFX); err != nil {
			return nil, err
		}
	}

	supplier, err := fetchUpstreamSupplierByID(ctx, txClient, input.SupplierID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return supplier, nil
}

// DeleteUpstreamSupplier hard-deletes a supplier only when it has never been
// used: no account binding history, recharge records, snapshots, or explicit
// non-default pools. Used suppliers must be archived so audit history remains
// explainable.
func (s *adminServiceImpl) DeleteUpstreamSupplier(ctx context.Context, supplierID int64) error {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return err
	}
	if supplierID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_ID", "invalid upstream supplier id")
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolSupplierAdvisoryLockBase+supplierID); err != nil {
		return err
	}

	current, err := loadUpstreamSupplierForUpdate(ctx, txClient, supplierID)
	if err != nil {
		return err
	}
	if isReservedUpstreamSupplier(current) {
		return ErrUpstreamSupplierReserved
	}
	if err := ensureUpstreamSupplierDeletable(ctx, txClient, supplierID); err != nil {
		return err
	}

	if _, err := txClient.ExecContext(ctx,
		`DELETE FROM upstream_cost_pools WHERE supplier_id = $1`, supplierID); err != nil {
		return err
	}
	if _, err := txClient.ExecContext(ctx,
		`DELETE FROM upstream_suppliers WHERE id = $1`, supplierID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *adminServiceImpl) ListUpstreamCostPools(ctx context.Context) ([]UpstreamCostPool, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	rows, err := s.entClient.QueryContext(ctx, upstreamCostPoolSelectSQL()+`
WHERE supplier.is_system = FALSE
GROUP BY p.id, supplier.id, supplier.name
ORDER BY p.status ASC, supplier.name ASC, p.name ASC, p.id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]UpstreamCostPool, 0)
	for rows.Next() {
		item, scanErr := scanUpstreamCostPool(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *adminServiceImpl) GetUpstreamCostPool(ctx context.Context, poolID int64) (*UpstreamCostPool, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	if poolID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	rows, err := s.entClient.QueryContext(ctx, upstreamCostPoolSelectSQL()+`
WHERE p.id = $1
  AND supplier.is_system = FALSE
GROUP BY p.id, supplier.id, supplier.name`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return scanUpstreamCostPool(rows)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUpstreamCostPoolNotFound
}

func (s *adminServiceImpl) ListUpstreamCostPoolRechargeRecords(ctx context.Context, poolID int64) (*UpstreamRechargeRecordsResult, error) {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return nil, err
	}
	if err := s.ensureUpstreamCostPoolExists(ctx, poolID); err != nil {
		return nil, err
	}
	items, err := s.listUpstreamRechargeRecords(ctx, 0, &poolID)
	if err != nil {
		return nil, err
	}
	summary, err := s.loadUpstreamRechargeSummary(ctx, 0, &poolID)
	if err != nil {
		return nil, err
	}
	return &UpstreamRechargeRecordsResult{
		Items:      items,
		Summary:    summary,
		CostPoolID: &poolID,
	}, nil
}

func (s *adminServiceImpl) CreateUpstreamCostPoolRechargeRecord(ctx context.Context, poolID int64, input UpstreamRechargeRecordInput) (*UpstreamRechargeRecord, error) {
	if poolID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	input.CostPoolID = poolID
	return s.CreateUpstreamRechargeRecord(ctx, input)
}

func (s *adminServiceImpl) UpdateUpstreamCostPoolRechargeRecord(ctx context.Context, poolID, recordID int64, input UpstreamRechargeRecordInput) (*UpstreamRechargeRecord, error) {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return nil, err
	}
	if poolID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	if recordID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_RECORD_ID", "invalid record id")
	}
	if err := s.ensureUpstreamCostPoolExists(ctx, poolID); err != nil {
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
  AND cost_pool_id = $2
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
		poolID,
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

	if err := refreshLatestUpstreamCostSnapshotForPool(ctx, txClient, poolID, input.CreatedBy); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	s.refreshSchedulerAccountSnapshotsForCostPool(ctx, poolID)
	return record, nil
}

func (s *adminServiceImpl) DeleteUpstreamCostPoolRechargeRecord(ctx context.Context, poolID, recordID int64) error {
	if err := s.ensureUpstreamRechargeServiceAvailable(); err != nil {
		return err
	}
	if poolID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	if recordID <= 0 {
		return infraerrors.BadRequest("INVALID_RECORD_ID", "invalid record id")
	}
	if err := s.ensureUpstreamCostPoolExists(ctx, poolID); err != nil {
		return err
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
  AND cost_pool_id = $2
  AND deleted_at IS NULL
  AND voided_at IS NULL
RETURNING id`, recordID, poolID)
	if err != nil {
		return err
	}
	found := false
	if rows.Next() {
		found = true
		var deletedID int64
		if err := rows.Scan(&deletedID); err != nil {
			_ = rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !found {
		return ErrUpstreamRechargeRecordNotFound
	}

	if err := refreshLatestUpstreamCostSnapshotForPool(ctx, txClient, poolID, nil); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.refreshSchedulerAccountSnapshotsForCostPool(ctx, poolID)
	return nil
}

func (s *adminServiceImpl) GetAccountUpstreamCostBinding(ctx context.Context, accountID int64) (*UpstreamAccountCostBinding, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	if _, err := s.accountRepo.GetByID(ctx, accountID); err != nil {
		return nil, err
	}
	return s.loadActiveUpstreamCostBinding(ctx, accountID)
}

func (s *adminServiceImpl) UpdateAccountUpstreamCostBinding(ctx context.Context, input UpstreamCostBindingInput) (*UpstreamAccountCostBinding, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	if input.AccountID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	if _, err := s.accountRepo.GetByID(ctx, input.AccountID); err != nil {
		return nil, err
	}
	normalized, err := normalizeUpstreamCostBindingInput(input)
	if err != nil {
		return nil, err
	}
	modelFamiliesJSON, err := json.Marshal(normalized.ModelFamilyMultipliers)
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolAccountAdvisoryLockBase+normalized.AccountID); err != nil {
		return nil, err
	}

	supplierID, err := requireBindableUpstreamCostPool(ctx, txClient, normalized.CostPoolID)
	if err != nil {
		return nil, err
	}
	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolSupplierAdvisoryLockBase+supplierID); err != nil {
		return nil, err
	}
	if _, err := requireBindableUpstreamCostPool(ctx, txClient, normalized.CostPoolID); err != nil {
		return nil, err
	}

	rows, err := txClient.QueryContext(ctx, `
WITH archived AS (
    UPDATE upstream_account_cost_bindings
    SET status = 'archived',
        valid_to = NOW(),
        updated_at = NOW()
    WHERE account_id = $1
      AND status = 'active'
    RETURNING id
),
inserted AS (
    INSERT INTO upstream_account_cost_bindings (
        account_id,
        cost_pool_id,
        upstream_group_name,
        default_multiplier,
        model_family_multipliers,
        note,
        created_by
    )
	    SELECT $1, $2, $3, $4, $5::jsonb, $6, $7
	    WHERE (SELECT COUNT(*) FROM archived) >= 0
	    RETURNING id,
	              account_id,
	              cost_pool_id,
	              status,
	              upstream_group_name,
	              default_multiplier,
	              model_family_multipliers,
	              note,
	              valid_from,
	              valid_to,
	              created_at,
	              updated_at
	)
	`+upstreamCostBindingSelectSQLFrom("inserted"),
		normalized.AccountID,
		normalized.CostPoolID,
		nullableString(normalized.UpstreamGroupName),
		normalized.DefaultMultiplier,
		string(modelFamiliesJSON),
		nullableString(normalized.Note),
		nullableInt64(normalized.CreatedBy),
	)
	if err != nil {
		return nil, err
	}
	var binding *UpstreamAccountCostBinding
	if rows.Next() {
		binding, err = scanUpstreamAccountCostBinding(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	closeErr := rows.Close()
	if closeErr != nil {
		return nil, closeErr
	}
	if binding == nil {
		return nil, ErrUpstreamCostBindingNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	refreshSchedulerAccountSnapshot(ctx, s.accountRepo, normalized.AccountID)
	return binding, nil
}

func (s *adminServiceImpl) UpdateAccountUpstreamSupplierBinding(ctx context.Context, input UpstreamSupplierBindingInput) (*UpstreamAccountCostBinding, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	if input.AccountID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	if _, err := s.accountRepo.GetByID(ctx, input.AccountID); err != nil {
		return nil, err
	}
	normalized, err := normalizeUpstreamSupplierBindingInput(input)
	if err != nil {
		return nil, err
	}
	modelFamiliesJSON, err := json.Marshal(normalized.ModelFamilyMultipliers)
	if err != nil {
		return nil, err
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txClient := tx.Client()

	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolAccountAdvisoryLockBase+normalized.AccountID); err != nil {
		return nil, err
	}

	if normalized.Clear {
		if _, err := txClient.ExecContext(ctx, `
UPDATE upstream_account_cost_bindings
SET status = 'archived',
    valid_to = NOW(),
    updated_at = NOW()
WHERE account_id = $1
  AND status = 'active'`, normalized.AccountID); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		refreshSchedulerAccountSnapshot(ctx, s.accountRepo, normalized.AccountID)
		return nil, nil
	}

	supplierID := normalized.SupplierID
	if supplierID <= 0 {
		supplierID, err = ensureNamedUpstreamSupplier(ctx, txClient, normalized.SupplierName, upstreamCostPoolStringPtr("通过账号编辑创建的上游供应商。"), normalized.CreatedBy)
		if err != nil {
			return nil, err
		}
	}

	if err := acquireUpstreamCostPoolAdvisoryLock(ctx, txClient, upstreamCostPoolSupplierAdvisoryLockBase+supplierID); err != nil {
		return nil, err
	}
	if err := ensureUpstreamSupplierExists(ctx, txClient, supplierID); err != nil {
		return nil, err
	}

	costPoolID := normalized.CostPoolID
	if costPoolID > 0 {
		if err := ensureUpstreamCostPoolBelongsToSupplier(ctx, txClient, costPoolID, supplierID); err != nil {
			return nil, err
		}
	} else {
		costPoolID, err = ensureDefaultUpstreamCostPoolForSupplier(ctx, txClient, supplierID, normalized.CreatedBy)
		if err != nil {
			return nil, err
		}
	}

	rows, err := txClient.QueryContext(ctx, `
WITH archived AS (
    UPDATE upstream_account_cost_bindings
    SET status = 'archived',
        valid_to = NOW(),
        updated_at = NOW()
    WHERE account_id = $1
      AND status = 'active'
    RETURNING id
),
inserted AS (
    INSERT INTO upstream_account_cost_bindings (
        account_id,
        cost_pool_id,
        upstream_group_name,
        default_multiplier,
        model_family_multipliers,
        note,
        created_by
    )
	    SELECT $1, $2, $3, $4, $5::jsonb, $6, $7
	    WHERE (SELECT COUNT(*) FROM archived) >= 0
	    RETURNING id,
	              account_id,
	              cost_pool_id,
	              status,
	              upstream_group_name,
	              default_multiplier,
	              model_family_multipliers,
	              note,
	              valid_from,
	              valid_to,
	              created_at,
	              updated_at
	)
	`+upstreamCostBindingSelectSQLFrom("inserted"),
		normalized.AccountID,
		costPoolID,
		nullableString(normalized.UpstreamGroupName),
		normalized.DefaultMultiplier,
		string(modelFamiliesJSON),
		nullableString(normalized.Note),
		nullableInt64(normalized.CreatedBy),
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var binding *UpstreamAccountCostBinding
	if rows.Next() {
		binding, err = scanUpstreamAccountCostBinding(rows)
		if err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if binding == nil {
		return nil, ErrUpstreamCostBindingNotFound
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	refreshSchedulerAccountSnapshot(ctx, s.accountRepo, normalized.AccountID)
	return binding, nil
}

func (s *adminServiceImpl) ListUpstreamCostPoolAccounts(ctx context.Context, poolID int64) ([]UpstreamAccountCostBinding, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	if err := s.ensureUpstreamCostPoolExists(ctx, poolID); err != nil {
		return nil, err
	}
	rows, err := s.entClient.QueryContext(ctx, upstreamCostBindingSelectSQL()+`
WHERE binding.cost_pool_id = $1
  AND binding.status = 'active'
  AND supplier.is_system = FALSE
ORDER BY account.name ASC, binding.id ASC`, poolID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]UpstreamAccountCostBinding, 0)
	for rows.Next() {
		item, scanErr := scanUpstreamAccountCostBinding(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *adminServiceImpl) findActiveUpstreamCostPoolIDForAccount(ctx context.Context, accountID int64) (*int64, error) {
	if s == nil || s.entClient == nil || accountID <= 0 {
		return nil, nil
	}
	return findActiveUpstreamCostPoolIDForAccount(ctx, s.entClient, accountID)
}

func findActiveUpstreamCostPoolIDForAccount(ctx context.Context, exec upstreamCostPoolSQLExecutor, accountID int64) (*int64, error) {
	rows, err := exec.QueryContext(ctx, `
SELECT binding.cost_pool_id
FROM upstream_account_cost_bindings binding
JOIN upstream_cost_pools pool ON pool.id = binding.cost_pool_id
JOIN upstream_suppliers supplier ON supplier.id = pool.supplier_id
WHERE binding.account_id = $1
  AND binding.status = 'active'
  AND supplier.is_system = FALSE
ORDER BY binding.id DESC
LIMIT 1`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, nil
	}
	var value int64
	if err := rows.Scan(&value); err != nil {
		return nil, err
	}
	return &value, nil
}

func (s *adminServiceImpl) requireActiveUpstreamCostPoolForAccount(ctx context.Context, account *Account) (int64, error) {
	if account == nil || account.ID <= 0 {
		return 0, infraerrors.BadRequest("INVALID_ACCOUNT_ID", "invalid account id")
	}
	existing, err := s.findActiveUpstreamCostPoolIDForAccount(ctx, account.ID)
	if err != nil {
		return 0, err
	}
	if existing != nil {
		return *existing, nil
	}
	return 0, infraerrors.BadRequest("UPSTREAM_SUPPLIER_BINDING_REQUIRED", "bind the account to an upstream supplier before recording recharge")
}

func ensureNamedUpstreamSupplier(ctx context.Context, exec upstreamCostPoolSQLExecutor, name string, note *string, createdBy *int64) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is required")
	}
	if len([]rune(name)) > 120 {
		return 0, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is too long")
	}
	rows, err := exec.QueryContext(ctx, `
	INSERT INTO upstream_suppliers (name, note, created_by)
	VALUES ($1, $2, $3)
	ON CONFLICT (name) WHERE archived_at IS NULL
	DO NOTHING
	RETURNING id, is_system`,
		name,
		nullableString(note),
		nullableInt64(createdBy),
	)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var id int64
		var isSystem bool
		if err := rows.Scan(&id, &isSystem); err != nil {
			return 0, err
		}
		if isSystem {
			return 0, ErrUpstreamSupplierReserved
		}
		return id, nil
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}

	existingRows, err := exec.QueryContext(ctx, `
	SELECT id, is_system
	FROM upstream_suppliers
	WHERE name = $1
	  AND archived_at IS NULL`, name)
	if err != nil {
		return 0, err
	}
	defer func() { _ = existingRows.Close() }()
	if existingRows.Next() {
		var id int64
		var isSystem bool
		if err := existingRows.Scan(&id, &isSystem); err != nil {
			return 0, err
		}
		if isSystem {
			return 0, ErrUpstreamSupplierReserved
		}
		return id, nil
	}
	if err := existingRows.Err(); err != nil {
		return 0, err
	}
	return 0, infraerrors.InternalServer("UPSTREAM_SUPPLIER_UNAVAILABLE", "upstream supplier is unavailable")
}

// createUpstreamSupplier is the strict user-facing create path. Unlike the
// internal ensure helper, it never reuses or mutates an existing supplier.
func createUpstreamSupplier(ctx context.Context, exec upstreamCostPoolSQLExecutor, name string, note *string, createdBy *int64) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is required")
	}
	if len([]rune(name)) > 120 {
		return 0, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is too long")
	}

	rows, err := exec.QueryContext(ctx, `
INSERT INTO upstream_suppliers (name, note, created_by)
VALUES ($1, $2, $3)
RETURNING id`, name, nullableString(note), nullableInt64(createdBy))
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrUpstreamSupplierNameConflict
		}
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var supplierID int64
		if err := rows.Scan(&supplierID); err != nil {
			return 0, err
		}
		return supplierID, rows.Err()
	}
	if err := rows.Err(); err != nil {
		if isUniqueViolation(err) {
			return 0, ErrUpstreamSupplierNameConflict
		}
		return 0, err
	}
	return 0, infraerrors.InternalServer("UPSTREAM_SUPPLIER_UNAVAILABLE", "upstream supplier is unavailable")
}

// loadUpstreamSupplierForUpdate fetches a supplier row (any status) or returns
// ErrUpstreamSupplierNotFound.
func loadUpstreamSupplierForUpdate(ctx context.Context, exec upstreamCostPoolSQLExecutor, supplierID int64) (*UpstreamSupplier, error) {
	rows, err := exec.QueryContext(ctx, `
SELECT id, name, status, note, is_system, created_at, updated_at, archived_at
FROM upstream_suppliers
WHERE id = $1`, supplierID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		supplier, scanErr := scanUpstreamSupplier(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		return supplier, rows.Err()
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUpstreamSupplierNotFound
}

func fetchUpstreamSupplierByID(ctx context.Context, exec upstreamCostPoolSQLExecutor, supplierID int64) (*UpstreamSupplier, error) {
	return loadUpstreamSupplierForUpdate(ctx, exec, supplierID)
}

func isReservedUpstreamSupplier(supplier *UpstreamSupplier) bool {
	return supplier != nil && supplier.IsSystem
}

// applyUpstreamSupplierUpdate mutates only the provided fields. Name changes are
// validated against the active-name unique index; status toggles archived_at.
func applyUpstreamSupplierUpdate(ctx context.Context, exec upstreamCostPoolSQLExecutor, current *UpstreamSupplier, input UpdateUpstreamSupplierInput) error {
	name := current.Name
	if input.Name != nil {
		trimmed := strings.TrimSpace(*input.Name)
		if trimmed == "" {
			return infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is required")
		}
		if len([]rune(trimmed)) > 120 {
			return infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is too long")
		}
		name = trimmed
	}

	status := current.Status
	var archivedAt any = current.ArchivedAt
	if input.Status != nil {
		switch *input.Status {
		case "active":
			status, archivedAt = "active", nil
		case "archived":
			status = "archived"
			archivedAt = time.Now()
		default:
			return infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_STATUS", "invalid upstream supplier status")
		}
	}

	note := current.Note
	if input.Note != nil {
		note = normalizeOptionalString(input.Note)
	}

	_, err := exec.ExecContext(ctx, `
UPDATE upstream_suppliers
SET name = $2, note = $3, status = $4, archived_at = $5, updated_at = NOW()
WHERE id = $1`, current.ID, name, nullableString(note), status, archivedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUpstreamSupplierNameConflict
		}
		return err
	}
	return nil
}

// isUniqueViolation reports whether err is a Postgres unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}

// ensureUpstreamSupplierDeletable returns a typed conflict error when the
// supplier still has active account bindings or any cost data beyond an empty
// default pool.
func ensureUpstreamSupplierDeletable(ctx context.Context, exec upstreamCostPoolSQLExecutor, supplierID int64) error {
	var activeBindings, bindingHistory, records, snapshots, nonDefaultPools int
	row := `
SELECT
  (SELECT COUNT(*) FROM upstream_account_cost_bindings b
     JOIN upstream_cost_pools p ON p.id = b.cost_pool_id
    WHERE p.supplier_id = $1 AND b.status = 'active'),
  (SELECT COUNT(*) FROM upstream_account_cost_bindings b
     JOIN upstream_cost_pools p ON p.id = b.cost_pool_id
    WHERE p.supplier_id = $1),
  (SELECT COUNT(*) FROM upstream_recharge_records r
     JOIN upstream_cost_pools p ON p.id = r.cost_pool_id
    WHERE p.supplier_id = $1),
  (SELECT COUNT(*) FROM upstream_cost_snapshots s
     JOIN upstream_cost_pools p ON p.id = s.cost_pool_id
    WHERE p.supplier_id = $1),
  (SELECT COUNT(*) FROM upstream_cost_pools p
    WHERE p.supplier_id = $1 AND p.is_default = FALSE)`
	rows, err := exec.QueryContext(ctx, row, supplierID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		if err := rows.Scan(&activeBindings, &bindingHistory, &records, &snapshots, &nonDefaultPools); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if activeBindings > 0 {
		return ErrUpstreamSupplierHasBoundAccounts
	}
	if bindingHistory > 0 {
		return ErrUpstreamSupplierHasBindingHistory
	}
	if records > 0 || snapshots > 0 || nonDefaultPools > 0 {
		return ErrUpstreamSupplierHasCostData
	}
	return nil
}

func ensureUpstreamSupplierExists(ctx context.Context, exec upstreamCostPoolSQLExecutor, supplierID int64) error {
	if supplierID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_ID", "invalid upstream supplier id")
	}
	rows, err := exec.QueryContext(ctx, `
	SELECT id
	FROM upstream_suppliers
	WHERE id = $1
	  AND status = 'active'
	  AND archived_at IS NULL
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

func ensureUpstreamCostPoolBelongsToSupplier(ctx context.Context, exec upstreamCostPoolSQLExecutor, poolID int64, supplierID int64) error {
	if poolID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	if supplierID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_ID", "invalid upstream supplier id")
	}
	rows, err := exec.QueryContext(ctx, `
	SELECT p.id
	FROM upstream_cost_pools p
	JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
	WHERE p.id = $1
	  AND p.supplier_id = $2
	  AND p.status = 'active'
	  AND p.archived_at IS NULL
	  AND supplier.status = 'active'
	  AND supplier.archived_at IS NULL
	  AND supplier.is_system = FALSE`, poolID, supplierID)
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
	return ErrUpstreamCostPoolNotFound
}

func ensureDefaultUpstreamCostPoolForSupplier(ctx context.Context, exec upstreamCostPoolSQLExecutor, supplierID int64, createdBy *int64) (int64, error) {
	if supplierID <= 0 {
		return 0, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_ID", "invalid upstream supplier id")
	}
	rows, err := exec.QueryContext(ctx, `
	SELECT id
	FROM upstream_cost_pools
	WHERE supplier_id = $1
	  AND is_default = TRUE
	  AND status = 'active'
	  AND archived_at IS NULL
	ORDER BY id ASC
	LIMIT 1`, supplierID)
	if err != nil {
		return 0, err
	}
	if rows.Next() {
		var poolID int64
		if err := rows.Scan(&poolID); err != nil {
			_ = rows.Close()
			return 0, err
		}
		_ = rows.Close()
		return poolID, nil
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()

	referenceFX := UpstreamRechargeDefaultReferenceFXRate
	rows, err = exec.QueryContext(ctx, `
	INSERT INTO upstream_cost_pools (
	    supplier_id,
	    name,
	    is_default,
	    reference_fx_rate,
	    default_effective_cny_per_usd,
	    default_reference_fx_rate,
	    cost_method,
	    note,
	    created_by
	) VALUES ($1, $2, TRUE, $3, $4, $5, 'latest', $6, $7)
	RETURNING id`,
		supplierID,
		upstreamDefaultCostPoolName,
		referenceFX,
		referenceFX,
		referenceFX,
		"供应商默认资金池。可在资金池管理里继续拆分余额池。",
		nullableInt64(createdBy),
	)
	if err != nil {
		return 0, err
	}
	var poolID int64
	if rows.Next() {
		if err := rows.Scan(&poolID); err != nil {
			_ = rows.Close()
			return 0, err
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	_ = rows.Close()
	if poolID <= 0 {
		return 0, ErrUpstreamCostPoolNotFound
	}
	return poolID, nil
}

func normalizeUpstreamCostPoolDefault(value float64, code, message string) (float64, error) {
	value = normalizeMoney(value)
	if !isPositiveFinite(value) {
		return 0, infraerrors.BadRequest(code, message)
	}
	return value, nil
}

func updateDefaultUpstreamCostPoolConfig(
	ctx context.Context,
	exec upstreamCostPoolSQLExecutor,
	poolID int64,
	defaultEffectiveCNYPerUSD *float64,
	defaultReferenceFXRate *float64,
) error {
	if poolID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	_, err := exec.ExecContext(ctx, `
UPDATE upstream_cost_pools
SET default_effective_cny_per_usd = COALESCE($2, default_effective_cny_per_usd),
    default_reference_fx_rate = COALESCE($3, default_reference_fx_rate),
    updated_at = NOW()
WHERE id = $1`, poolID, nullableFloat(defaultEffectiveCNYPerUSD), nullableFloat(defaultReferenceFXRate))
	return err
}

func requireBindableUpstreamCostPool(ctx context.Context, exec upstreamCostPoolSQLExecutor, poolID int64) (int64, error) {
	if poolID <= 0 {
		return 0, infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	rows, err := exec.QueryContext(ctx, `
SELECT supplier.id
FROM upstream_cost_pools p
JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
WHERE p.id = $1
  AND p.status = 'active'
  AND p.archived_at IS NULL
  AND supplier.status = 'active'
  AND supplier.archived_at IS NULL
  AND supplier.is_system = FALSE`, poolID)
	if err != nil {
		return 0, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		var supplierID int64
		if err := rows.Scan(&supplierID); err != nil {
			return 0, err
		}
		return supplierID, rows.Err()
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return 0, ErrUpstreamCostPoolNotFound
}

func (s *adminServiceImpl) ensureUpstreamCostPoolExists(ctx context.Context, poolID int64) error {
	if poolID <= 0 {
		return infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}
	rows, err := s.entClient.QueryContext(ctx, `
SELECT p.id
FROM upstream_cost_pools p
JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
WHERE p.id = $1
  AND p.status = 'active'
  AND p.archived_at IS NULL
  AND supplier.is_system = FALSE`, poolID)
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
	return ErrUpstreamCostPoolNotFound
}

func (s *adminServiceImpl) loadActiveUpstreamCostBinding(ctx context.Context, accountID int64) (*UpstreamAccountCostBinding, error) {
	rows, err := s.entClient.QueryContext(ctx, upstreamCostBindingSelectSQL()+`
WHERE binding.account_id = $1
  AND binding.status = 'active'
  AND supplier.is_system = FALSE
ORDER BY binding.id DESC
LIMIT 1`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return scanUpstreamAccountCostBinding(rows)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUpstreamCostBindingNotFound
}

func (s *adminServiceImpl) ensureUpstreamCostPoolServiceAvailable() error {
	if s == nil || s.entClient == nil || s.accountRepo == nil {
		return infraerrors.InternalServer("UPSTREAM_COST_POOL_UNAVAILABLE", "upstream cost pool service is unavailable")
	}
	return nil
}

func normalizeUpstreamCostBindingInput(input UpstreamCostBindingInput) (UpstreamCostBindingInput, error) {
	input.UpstreamGroupName = normalizeOptionalString(input.UpstreamGroupName)
	if input.UpstreamGroupName != nil && len([]rune(*input.UpstreamGroupName)) > 120 {
		return UpstreamCostBindingInput{}, infraerrors.BadRequest("INVALID_UPSTREAM_GROUP_NAME", "upstream group name is too long")
	}
	input.DefaultMultiplier = normalizeMoney(input.DefaultMultiplier)
	if input.DefaultMultiplier <= 0 {
		input.DefaultMultiplier = 1
	}
	if input.CostPoolID <= 0 {
		return UpstreamCostBindingInput{}, infraerrors.BadRequest("INVALID_UPSTREAM_COST_POOL_ID", "invalid upstream cost pool id")
	}

	normalizedFamilies := make([]UpstreamCostModelFamilyMultiplier, 0, len(input.ModelFamilyMultipliers))
	seen := make(map[string]struct{}, len(input.ModelFamilyMultipliers))
	for _, item := range input.ModelFamilyMultipliers {
		family := strings.ToLower(strings.TrimSpace(item.Family))
		if family == "" {
			continue
		}
		if _, exists := seen[family]; exists {
			continue
		}
		multiplier := normalizeMoney(item.GroupMultiplier)
		if multiplier <= 0 {
			return UpstreamCostBindingInput{}, infraerrors.BadRequest("INVALID_UPSTREAM_COST_MODEL_FAMILY_MULTIPLIER", "model family multiplier must be greater than 0")
		}
		seen[family] = struct{}{}
		normalizedFamilies = append(normalizedFamilies, UpstreamCostModelFamilyMultiplier{
			Family:          family,
			GroupMultiplier: multiplier,
			Note:            normalizeOptionalString(item.Note),
		})
	}
	input.ModelFamilyMultipliers = normalizedFamilies
	input.Note = normalizeOptionalString(input.Note)
	return input, nil
}

func normalizeUpstreamSupplierBindingInput(input UpstreamSupplierBindingInput) (UpstreamSupplierBindingInput, error) {
	input.SupplierName = strings.TrimSpace(input.SupplierName)
	if input.Clear {
		input.SupplierID = 0
		input.SupplierName = ""
		input.CostPoolID = 0
		input.UpstreamGroupName = nil
		input.ModelFamilyMultipliers = nil
		input.Note = nil
		return input, nil
	}
	if input.CostPoolID <= 0 && input.SupplierID <= 0 && input.SupplierName == "" {
		return UpstreamSupplierBindingInput{}, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER", "upstream supplier is required")
	}
	if input.SupplierName != "" && len([]rune(input.SupplierName)) > 120 {
		return UpstreamSupplierBindingInput{}, infraerrors.BadRequest("INVALID_UPSTREAM_SUPPLIER_NAME", "upstream supplier name is too long")
	}
	costBinding, err := normalizeUpstreamCostBindingInput(UpstreamCostBindingInput{
		AccountID:              input.AccountID,
		CostPoolID:             1,
		UpstreamGroupName:      input.UpstreamGroupName,
		DefaultMultiplier:      input.DefaultMultiplier,
		ModelFamilyMultipliers: input.ModelFamilyMultipliers,
		Note:                   input.Note,
		CreatedBy:              input.CreatedBy,
	})
	if err != nil {
		return UpstreamSupplierBindingInput{}, err
	}
	input.UpstreamGroupName = costBinding.UpstreamGroupName
	input.DefaultMultiplier = costBinding.DefaultMultiplier
	input.ModelFamilyMultipliers = costBinding.ModelFamilyMultipliers
	input.Note = costBinding.Note
	return input, nil
}

func acquireUpstreamCostPoolAdvisoryLock(ctx context.Context, exec upstreamCostPoolSQLExecutor, lockID int64) error {
	rows, err := exec.QueryContext(ctx, `SELECT pg_advisory_xact_lock($1)`, lockID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return sql.ErrNoRows
	}
	return rows.Err()
}

func upstreamCostPoolStringPtr(value string) *string {
	return &value
}

func upstreamCostPoolSelectSQL() string {
	return `
SELECT p.id,
       p.supplier_id,
       supplier.name AS supplier_name,
       p.name,
	   p.is_default,
       p.status,
       p.base_currency,
       p.credit_currency,
       p.reference_fx_rate::double precision,
	   p.default_effective_cny_per_usd::double precision,
	   p.default_reference_fx_rate::double precision,
       p.cost_method,
       p.current_effective_cny_per_usd::double precision,
       p.current_snapshot_id,
       p.balance_query_enabled,
       p.balance_provider,
       p.balance_endpoint,
       p.balance_auth_mode,
       p.balance_auth_header,
       p.balance_low_threshold::double precision,
       p.last_balance_snapshot::text,
       p.note,
       COUNT(DISTINCT binding.id)::int AS binding_count,
       COUNT(DISTINCT record.id)::int AS record_count,
       p.created_at,
       p.updated_at,
       p.archived_at
FROM upstream_cost_pools p
JOIN upstream_suppliers supplier ON supplier.id = p.supplier_id
LEFT JOIN upstream_account_cost_bindings binding
  ON binding.cost_pool_id = p.id
 AND binding.status = 'active'
LEFT JOIN upstream_recharge_records record
  ON record.cost_pool_id = p.id
 AND record.deleted_at IS NULL
 AND record.voided_at IS NULL
`
}

func upstreamCostBindingSelectSQL() string {
	return upstreamCostBindingSelectSQLFrom("upstream_account_cost_bindings")
}

func upstreamCostBindingSelectSQLFrom(source string) string {
	return `
SELECT binding.id,
	       binding.account_id,
       account.name AS account_name,
       account.platform AS account_platform,
       binding.cost_pool_id,
       pool.name AS cost_pool_name,
       pool.supplier_id,
       supplier.name AS supplier_name,
       binding.status,
       binding.upstream_group_name,
       binding.default_multiplier::double precision,
       binding.model_family_multipliers::text,
       binding.note,
       binding.valid_from,
       binding.valid_to,
	       binding.created_at,
	       binding.updated_at
	FROM ` + source + ` binding
	JOIN accounts account ON account.id = binding.account_id
	JOIN upstream_cost_pools pool ON pool.id = binding.cost_pool_id
	JOIN upstream_suppliers supplier ON supplier.id = pool.supplier_id
	`
}

func scanUpstreamSupplier(scanner upstreamRechargeScanner) (*UpstreamSupplier, error) {
	var (
		item       UpstreamSupplier
		note       sql.NullString
		archivedAt sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.Status,
		&note,
		&item.IsSystem,
		&item.CreatedAt,
		&item.UpdatedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}
	if note.Valid {
		item.Note = &note.String
	}
	if archivedAt.Valid {
		item.ArchivedAt = &archivedAt.Time
	}
	return &item, nil
}

func scanUpstreamCostPool(scanner upstreamRechargeScanner) (*UpstreamCostPool, error) {
	var (
		item                UpstreamCostPool
		currentCost         sql.NullFloat64
		currentSnapID       sql.NullInt64
		balanceProvider     sql.NullString
		balanceEndpoint     sql.NullString
		balanceAuthMode     sql.NullString
		balanceAuthHeader   sql.NullString
		balanceLowThreshold sql.NullFloat64
		lastBalanceSnapshot sql.NullString
		note                sql.NullString
		archivedAt          sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.SupplierID,
		&item.SupplierName,
		&item.Name,
		&item.IsDefault,
		&item.Status,
		&item.BaseCurrency,
		&item.CreditCurrency,
		&item.ReferenceFXRate,
		&item.DefaultEffectiveCNYPerUSD,
		&item.DefaultReferenceFXRate,
		&item.CostMethod,
		&currentCost,
		&currentSnapID,
		&item.BalanceQueryEnabled,
		&balanceProvider,
		&balanceEndpoint,
		&balanceAuthMode,
		&balanceAuthHeader,
		&balanceLowThreshold,
		&lastBalanceSnapshot,
		&note,
		&item.BindingCount,
		&item.RecordCount,
		&item.CreatedAt,
		&item.UpdatedAt,
		&archivedAt,
	); err != nil {
		return nil, err
	}
	if currentCost.Valid {
		item.CurrentEffectiveCNYPerUSD = &currentCost.Float64
	}
	if currentSnapID.Valid {
		item.CurrentSnapshotID = &currentSnapID.Int64
	}
	if balanceProvider.Valid {
		item.BalanceProvider = &balanceProvider.String
	}
	if balanceEndpoint.Valid {
		item.BalanceEndpoint = &balanceEndpoint.String
	}
	if balanceAuthMode.Valid {
		item.BalanceAuthMode = &balanceAuthMode.String
	}
	if balanceAuthHeader.Valid {
		item.BalanceAuthHeader = &balanceAuthHeader.String
	}
	if balanceLowThreshold.Valid {
		item.BalanceLowThreshold = &balanceLowThreshold.Float64
	}
	if lastBalanceSnapshot.Valid && lastBalanceSnapshot.String != "" && lastBalanceSnapshot.String != "{}" {
		var snapshot map[string]any
		if err := json.Unmarshal([]byte(lastBalanceSnapshot.String), &snapshot); err == nil && len(snapshot) > 0 {
			item.LastBalanceSnapshot = snapshot
		}
	}
	if note.Valid {
		item.Note = &note.String
	}
	if archivedAt.Valid {
		item.ArchivedAt = &archivedAt.Time
	}
	return &item, nil
}

func scanUpstreamAccountCostBinding(scanner upstreamRechargeScanner) (*UpstreamAccountCostBinding, error) {
	var (
		item          UpstreamAccountCostBinding
		upstreamGroup sql.NullString
		modelFamilies sql.NullString
		note          sql.NullString
		validTo       sql.NullTime
	)
	if err := scanner.Scan(
		&item.ID,
		&item.AccountID,
		&item.AccountName,
		&item.AccountPlatform,
		&item.CostPoolID,
		&item.CostPoolName,
		&item.SupplierID,
		&item.SupplierName,
		&item.Status,
		&upstreamGroup,
		&item.DefaultMultiplier,
		&modelFamilies,
		&note,
		&item.ValidFrom,
		&validTo,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if modelFamilies.Valid && modelFamilies.String != "" {
		_ = json.Unmarshal([]byte(modelFamilies.String), &item.ModelFamilyMultipliers)
	}
	if upstreamGroup.Valid {
		item.UpstreamGroupName = &upstreamGroup.String
	}
	item.UpstreamGroupMultiplier = item.DefaultMultiplier
	if item.ModelFamilyMultipliers == nil {
		item.ModelFamilyMultipliers = []UpstreamCostModelFamilyMultiplier{}
	}
	if note.Valid {
		item.Note = &note.String
	}
	if validTo.Valid {
		item.ValidTo = &validTo.Time
	}
	return &item, nil
}

func positiveFloatFromExtra(extra map[string]any, key string, fallback float64) float64 {
	if value := optionalPositiveFloatFromExtra(extra, key); value != nil {
		return *value
	}
	return fallback
}

func optionalPositiveFloatFromExtra(extra map[string]any, key string) *float64 {
	if extra == nil {
		return nil
	}
	value, ok := extra[key]
	if !ok {
		return nil
	}
	var parsed float64
	switch typed := value.(type) {
	case float64:
		parsed = typed
	case float32:
		parsed = float64(typed)
	case int:
		parsed = float64(typed)
	case int64:
		parsed = float64(typed)
	case json.Number:
		number, err := typed.Float64()
		if err != nil {
			return nil
		}
		parsed = number
	default:
		return nil
	}
	if !isPositiveFinite(parsed) {
		return nil
	}
	return &parsed
}

func modelFamilyMultipliersFromExtra(extra map[string]any) []UpstreamCostModelFamilyMultiplier {
	raw, ok := extra["upstream_cost_model_families"]
	if !ok || raw == nil {
		return []UpstreamCostModelFamilyMultiplier{}
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return []UpstreamCostModelFamilyMultiplier{}
	}
	var families []UpstreamCostModelFamilyMultiplier
	if err := json.Unmarshal(encoded, &families); err != nil {
		return []UpstreamCostModelFamilyMultiplier{}
	}
	normalized, err := normalizeUpstreamCostBindingInput(UpstreamCostBindingInput{
		CostPoolID:             1,
		DefaultMultiplier:      1,
		ModelFamilyMultipliers: families,
	})
	if err != nil {
		return []UpstreamCostModelFamilyMultiplier{}
	}
	return normalized.ModelFamilyMultipliers
}
