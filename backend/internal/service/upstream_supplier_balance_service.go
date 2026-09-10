package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Supplier balances belong to the upstream login account, not its API keys.
type UpstreamSupplierBalanceConfig struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	BaseURL        string `json:"base_url"`
	UserID         int64  `json:"user_id,omitempty"`
	HasAccessToken bool   `json:"has_access_token"`
}

type UpstreamSupplierBalanceInput struct {
	Enabled     bool   `json:"enabled"`
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url"`
	UserID      int64  `json:"user_id,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
}

type UpstreamSupplierBalanceSnapshot struct {
	// Retain the original numeric field for API compatibility; Unit controls its display.
	BalanceUSD    *float64   `json:"balance_usd,omitempty"`
	Unit          string     `json:"unit,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
	LastAttemptAt time.Time  `json:"last_attempt_at"`
	Status        string     `json:"status"`
	Error         string     `json:"error,omitempty"`
}

func normalizeSupplierBalanceInput(current *UpstreamSupplier, input UpstreamSupplierBalanceInput, encryptor SecretEncryptor) (UpstreamSupplierBalanceConfig, string, bool, error) {
	cfg := UpstreamSupplierBalanceConfig{
		Enabled: input.Enabled, Provider: strings.TrimSpace(input.Provider),
		BaseURL: strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"), UserID: input.UserID,
	}
	invalid := func(message string) (UpstreamSupplierBalanceConfig, string, bool, error) {
		return cfg, "", false, infraerrors.BadRequest("INVALID_SUPPLIER_BALANCE_CONFIG", message)
	}
	if cfg.Provider != "" && cfg.Provider != "newapi" && cfg.Provider != "soleapi" && cfg.Provider != "sub2api" {
		return invalid("unsupported supplier balance provider")
	}
	if cfg.UserID < 0 {
		return invalid("upstream user ID must be a positive integer")
	}
	if cfg.Provider == "soleapi" || cfg.Provider == "sub2api" {
		cfg.UserID = 0
	}
	if cfg.BaseURL != "" {
		u, err := url.Parse(cfg.BaseURL)
		if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return invalid("supplier website must be an HTTP(S) URL without credentials, query or fragment")
		}
	}
	token := strings.TrimSpace(input.AccessToken)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		token = strings.TrimSpace(token[7:])
	}
	if len(token) > 8192 || strings.ContainsAny(token, "\r\n") {
		return invalid("invalid supplier Access Token")
	}
	ciphertext := current.balanceAccessToken
	targetChanged := cfg.Provider != current.BalanceConfig.Provider || cfg.BaseURL != current.BalanceConfig.BaseURL || cfg.UserID != current.BalanceConfig.UserID
	// A different provider can require a different kind of credential, even on the same website.
	if (cfg.Provider != current.BalanceConfig.Provider || cfg.BaseURL != current.BalanceConfig.BaseURL) && token == "" {
		ciphertext = ""
	}
	if token != "" {
		if encryptor == nil {
			return cfg, "", false, errors.New("supplier credential encryption is unavailable")
		}
		var err error
		ciphertext, err = encryptor.Encrypt(token)
		if err != nil {
			return cfg, "", false, errors.New("failed to encrypt supplier Access Token")
		}
	}
	if cfg.Enabled && (cfg.Provider == "" || cfg.BaseURL == "" || ciphertext == "") {
		return invalid("balance queries require a site type, website and query credential; enter a new token when changing the site type or website")
	}
	cfg.HasAccessToken = ciphertext != ""
	return cfg, ciphertext, targetChanged || token != "", nil
}

func (s *adminServiceImpl) configureUpstreamSupplierBalance(ctx context.Context, exec upstreamCostPoolSQLExecutor, current *UpstreamSupplier, input *UpstreamSupplierBalanceInput) error {
	if input == nil {
		return nil
	}
	cfg, token, resetSnapshot, err := normalizeSupplierBalanceInput(current, *input, s.secretEncryptor)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode supplier balance configuration: %w", err)
	}
	_, err = exec.ExecContext(ctx, `UPDATE upstream_suppliers
SET balance_config = $2::jsonb, balance_access_token = $3,
    balance_snapshot = CASE WHEN $4 THEN '{}'::jsonb ELSE balance_snapshot END,
    balance_revision = CASE WHEN $4 THEN balance_revision + 1 ELSE balance_revision END,
    balance_next_poll_at = CASE WHEN $5 THEN NOW() ELSE balance_next_poll_at END,
    updated_at = NOW()
WHERE id = $1`, current.ID, string(encoded), token, resetSnapshot, cfg.Enabled || resetSnapshot)
	if err != nil {
		return fmt.Errorf("save supplier balance configuration: %w", err)
	}
	return nil
}

func supplierBalanceSnapshot(previous *UpstreamSupplierBalanceSnapshot, balance float64, queryErr error, now time.Time) *UpstreamSupplierBalanceSnapshot {
	snapshot := &UpstreamSupplierBalanceSnapshot{}
	if previous != nil {
		*snapshot = *previous
	}
	snapshot.LastAttemptAt = now
	if queryErr != nil {
		snapshot.Status = "error"
		snapshot.Error = queryErr.Error()
		return snapshot
	}
	snapshot.Status, snapshot.Error = "ok", ""
	snapshot.BalanceUSD, snapshot.UpdatedAt = &balance, &now
	return snapshot
}

func (s *adminServiceImpl) RefreshUpstreamSupplierBalance(ctx context.Context, supplierID int64) (*UpstreamSupplier, error) {
	if err := s.ensureUpstreamCostPoolServiceAvailable(); err != nil {
		return nil, err
	}
	return refreshUpstreamSupplierBalance(ctx, s.entClient, s.cfg, s.secretEncryptor, supplierID)
}

func refreshUpstreamSupplierBalance(ctx context.Context, exec upstreamCostPoolSQLExecutor, cfg *config.Config, encryptor SecretEncryptor, supplierID int64) (*UpstreamSupplier, error) {
	current, err := fetchUpstreamSupplierByID(ctx, exec, supplierID)
	if err != nil {
		return nil, fmt.Errorf("load supplier for balance refresh: %w", err)
	}
	if isReservedUpstreamSupplier(current) {
		return nil, ErrUpstreamSupplierReserved
	}
	if current.Status != "active" || !current.BalanceConfig.Enabled {
		return nil, infraerrors.BadRequest("SUPPLIER_BALANCE_DISABLED", "balance query is disabled or supplier is archived")
	}
	var balance float64
	unit := "USD"
	var queryErr error
	if encryptor == nil {
		queryErr = errors.New("supplier credential encryption is unavailable")
	} else {
		token, decryptErr := encryptor.Decrypt(current.balanceAccessToken)
		if decryptErr != nil {
			queryErr = errors.New("unable to read saved query credential; please enter it again")
		} else {
			switch current.BalanceConfig.Provider {
			case "newapi":
				balance, queryErr = fetchNewAPISupplierBalance(ctx, cfg, current.BalanceConfig, token)
			case "soleapi":
				balance, queryErr = fetchSoleAPISupplierBalance(ctx, cfg, current.BalanceConfig, token)
				unit = "Credits"
			case "sub2api":
				balance, queryErr = fetchSub2APISupplierBalance(ctx, cfg, current.BalanceConfig, token)
			default:
				queryErr = errors.New("unsupported supplier balance provider")
			}
		}
	}
	snapshot := supplierBalanceSnapshot(current.BalanceSnapshot, balance, queryErr, time.Now().UTC())
	if queryErr == nil {
		snapshot.Unit = unit
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encode supplier balance snapshot: %w", err)
	}
	accepted, err := saveUpstreamSupplierBalanceSnapshot(ctx, exec, current, string(encoded), queryErr == nil, balance, unit, snapshot.LastAttemptAt)
	if err != nil {
		return nil, err
	}
	if !accepted {
		slog.DebugContext(ctx, "Supplier balance refresh result discarded because stored state changed", "supplier_id", supplierID)
	}
	latest, err := fetchUpstreamSupplierByID(ctx, exec, supplierID)
	if err != nil {
		return nil, fmt.Errorf("reload supplier after balance refresh: %w", err)
	}
	return latest, nil
}

func saveUpstreamSupplierBalanceSnapshot(
	ctx context.Context,
	exec upstreamCostPoolSQLExecutor,
	current *UpstreamSupplier,
	encodedSnapshot string,
	success bool,
	balance float64,
	unit string,
	sampledAt time.Time,
) (bool, error) {
	if success {
		rows, err := exec.QueryContext(ctx, `
WITH updated AS (
    UPDATE upstream_suppliers
    SET balance_snapshot = $2::jsonb,
        balance_next_poll_at = NOW() + INTERVAL '5 minutes'
    WHERE id = $1 AND balance_config = $3::jsonb AND balance_access_token = $4
      AND balance_snapshot = $5::jsonb AND balance_revision = $9 AND status = 'active'
    RETURNING id, balance_revision
), inserted AS (
    INSERT INTO upstream_supplier_balance_samples (supplier_id, revision, balance, unit, sampled_at)
    SELECT id, balance_revision, $6, $7, $8
    FROM updated
    RETURNING id
)
SELECT COUNT(*) FROM inserted`,
			current.ID, encodedSnapshot, current.balanceConfigJSON, current.balanceAccessToken, current.balanceSnapshotJSON, balance, unit, sampledAt, current.balanceRevision)
		if err != nil {
			return false, fmt.Errorf("save supplier balance snapshot: %w", err)
		}
		defer func() { _ = rows.Close() }()
		var inserted int
		if rows.Next() {
			if err := rows.Scan(&inserted); err != nil {
				return false, fmt.Errorf("check supplier balance snapshot update: %w", err)
			}
		}
		if err := rows.Err(); err != nil {
			return false, fmt.Errorf("check supplier balance snapshot update: %w", err)
		}
		return inserted > 0, nil
	}

	result, err := exec.ExecContext(ctx, `UPDATE upstream_suppliers
SET balance_snapshot = $2::jsonb
WHERE id = $1 AND balance_config = $3::jsonb AND balance_access_token = $4
  AND balance_snapshot = $5::jsonb AND balance_revision = $6 AND status = 'active'`,
		current.ID, encodedSnapshot, current.balanceConfigJSON, current.balanceAccessToken, current.balanceSnapshotJSON, current.balanceRevision)
	if err != nil {
		return false, fmt.Errorf("save supplier balance snapshot: %w", err)
	}
	affected, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return false, fmt.Errorf("check supplier balance snapshot update: %w", rowsErr)
	}
	return affected > 0, nil
}
