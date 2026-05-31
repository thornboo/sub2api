package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type accountAPIKeyPoolAuditEvent struct {
	message string
	args    []any
}

func newAccountAPIKeyPoolAuditEvent(message string, args ...any) accountAPIKeyPoolAuditEvent {
	return accountAPIKeyPoolAuditEvent{message: message, args: args}
}

func (r *accountRepository) ReplaceAccountAPIKeys(ctx context.Context, accountID int64, keys []service.AccountAPIKeyInput) error {
	if accountID <= 0 {
		return service.ErrAccountNotFound
	}
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return errors.New("account api key replacement requires sql.DB executor")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	existingKeys, err := loadAccountAPIKeySecretsForUpdate(ctx, tx, accountID)
	if err != nil {
		return err
	}
	keptIDs := make([]int64, 0, len(keys))
	keptIDSet := make(map[int64]struct{}, len(keys))
	auditEvents := make([]accountAPIKeyPoolAuditEvent, 0, len(keys)+1)
	for _, key := range keys {
		keyID := int64(0)
		if key.ID != nil && *key.ID > 0 {
			if _, ok := existingKeys[*key.ID]; ok {
				keyID = *key.ID
			}
		}
		apiKey := strings.TrimSpace(key.APIKey)
		preservedExistingSecret := false
		if apiKey == "" && keyID > 0 {
			apiKey = existingKeys[keyID]
			preservedExistingSecret = true
		}
		if apiKey == "" {
			auditEvents = append(auditEvents, newAccountAPIKeyPoolAuditEvent("[AccountAPIKeyPool] skipped blank key: account=%d submitted_key_id=%d", accountID, keyID))
			continue
		}
		name := strings.TrimSpace(key.Name)
		if name == "" {
			name = "API Key"
		}
		status := normalizeAccountAPIKeyStatus(key.Status)
		mode := normalizeAccountAPIKeyModelRestrictionMode(key.ModelRestrictionMode)
		modelMapping := normalizeAccountAPIKeyModelMapping(key.ModelMapping)
		modelMappingBytes, err := json.Marshal(modelMapping)
		if err != nil {
			return err
		}
		priority := key.Priority
		if priority == 0 {
			priority = service.DefaultAccountAPIKeyPriority
		}
		if keyID > 0 {
			if _, err := tx.ExecContext(ctx, `
				UPDATE account_api_keys
				SET name = $1, api_key = $2, priority = $3, status = $4,
					model_restriction_mode = $5, model_mapping = $6::jsonb, updated_at = NOW()
				WHERE id = $7 AND account_id = $8
			`, name, apiKey, priority, status, mode, string(modelMappingBytes), keyID, accountID); err != nil {
				return err
			}
			keptIDs = append(keptIDs, keyID)
			keptIDSet[keyID] = struct{}{}
			if preservedExistingSecret {
				auditEvents = append(auditEvents, newAccountAPIKeyPoolAuditEvent("[AccountAPIKeyPool] preserved existing key secret: account=%d key=%d name=%q", accountID, keyID, name))
			} else {
				auditEvents = append(auditEvents, newAccountAPIKeyPoolAuditEvent("[AccountAPIKeyPool] updated key secret: account=%d key=%d name=%q", accountID, keyID, name))
			}
		} else {
			var insertedID int64
			if err := tx.QueryRowContext(ctx, `
				INSERT INTO account_api_keys (account_id, name, api_key, priority, status, model_restriction_mode, model_mapping)
				VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
				RETURNING id
			`, accountID, name, apiKey, priority, status, mode, string(modelMappingBytes)).Scan(&insertedID); err != nil {
				return err
			}
			keptIDs = append(keptIDs, insertedID)
			keptIDSet[insertedID] = struct{}{}
			auditEvents = append(auditEvents, newAccountAPIKeyPoolAuditEvent("[AccountAPIKeyPool] created key: account=%d key=%d name=%q", accountID, insertedID, name))
		}
	}
	deletedCount := 0
	for keyID := range existingKeys {
		if _, kept := keptIDSet[keyID]; !kept {
			deletedCount++
		}
	}
	if len(keptIDs) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM account_api_keys WHERE account_id = $1`, accountID); err != nil {
			return err
		}
	} else if _, err := tx.ExecContext(ctx, `
		DELETE FROM account_api_keys
		WHERE account_id = $1 AND NOT (id = ANY($2))
	`, accountID, pq.Array(keptIDs)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if deletedCount > 0 {
		auditEvents = append(auditEvents, newAccountAPIKeyPoolAuditEvent("[AccountAPIKeyPool] deleted keys: account=%d count=%d", accountID, deletedCount))
	}
	for _, event := range auditEvents {
		logger.LegacyPrintf("repository.account_api_key_pool", event.message, event.args...)
	}
	r.syncSchedulerAccountSnapshot(ctx, accountID)
	return nil
}

func loadAccountAPIKeySecretsForUpdate(ctx context.Context, tx *sql.Tx, accountID int64) (map[int64]string, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, api_key
		FROM account_api_keys
		WHERE account_id = $1
		FOR UPDATE
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int64]string)
	for rows.Next() {
		var id int64
		var apiKey string
		if err := rows.Scan(&id, &apiKey); err != nil {
			return nil, err
		}
		out[id] = apiKey
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *accountRepository) SetAccountAPIKeyModelCooldown(ctx context.Context, keyID int64, upstreamModel string, resetAt time.Time, reason string, statusCode int, message string) error {
	upstreamModel = strings.TrimSpace(upstreamModel)
	if keyID <= 0 || upstreamModel == "" {
		return nil
	}
	var statusCodeArg any
	if statusCode > 0 {
		statusCodeArg = statusCode
	}
	_, err := r.sql.ExecContext(ctx, `
		INSERT INTO account_api_key_model_cooldowns (
			account_api_key_id, upstream_model, reason, status_code, cooldown_until,
			last_error_at, last_error_message_sanitized, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, NOW(), $6, NOW())
		ON CONFLICT (account_api_key_id, upstream_model) DO UPDATE SET
			reason = EXCLUDED.reason,
			status_code = EXCLUDED.status_code,
			cooldown_until = EXCLUDED.cooldown_until,
			last_error_at = NOW(),
			last_error_message_sanitized = EXCLUDED.last_error_message_sanitized,
			updated_at = NOW()
	`, keyID, upstreamModel, strings.TrimSpace(reason), statusCodeArg, resetAt.UTC(), truncateStringForDB(message, 1000))
	if err != nil {
		return err
	}
	accountID, err := r.accountIDByAPIKeyID(ctx, keyID)
	if err == nil && accountID > 0 {
		r.syncSchedulerAccountSnapshot(ctx, accountID)
	}
	return err
}

func (r *accountRepository) MarkAccountAPIKeyUsed(ctx context.Context, keyID int64, when time.Time, failed bool) error {
	if keyID <= 0 {
		return nil
	}
	if failed {
		_, err := r.sql.ExecContext(ctx, `
			UPDATE account_api_keys
			SET last_used_at = $1, recent_request_count = recent_request_count + 1,
				recent_error_count = recent_error_count + 1, updated_at = NOW()
			WHERE id = $2
		`, when.UTC(), keyID)
		return err
	}
	_, err := r.sql.ExecContext(ctx, `
		UPDATE account_api_keys
		SET last_used_at = $1, recent_request_count = recent_request_count + 1, updated_at = NOW()
		WHERE id = $2
	`, when.UTC(), keyID)
	return err
}

func (r *accountRepository) ClearAccountAPIKeyModelCooldown(ctx context.Context, keyID int64, upstreamModel string) error {
	if keyID <= 0 {
		return nil
	}
	upstreamModel = strings.TrimSpace(upstreamModel)
	var err error
	if upstreamModel == "" {
		_, err = r.sql.ExecContext(ctx, `DELETE FROM account_api_key_model_cooldowns WHERE account_api_key_id = $1`, keyID)
	} else {
		_, err = r.sql.ExecContext(ctx, `DELETE FROM account_api_key_model_cooldowns WHERE account_api_key_id = $1 AND upstream_model = $2`, keyID, upstreamModel)
	}
	return err
}

func (r *accountRepository) loadAccountAPIKeys(ctx context.Context, accountIDs []int64) (map[int64][]service.AccountAPIKey, error) {
	out := make(map[int64][]service.AccountAPIKey)
	if len(accountIDs) == 0 {
		return out, nil
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT id, account_id, name, api_key, priority, status,
		       model_restriction_mode, model_mapping, global_cooldown_until,
		       last_used_at, recent_request_count, recent_error_count, created_at, updated_at
		FROM account_api_keys
		WHERE account_id = ANY($1)
		ORDER BY account_id, priority, last_used_at NULLS FIRST, id
	`, pq.Array(accountIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keyIDs := make([]int64, 0)
	byKeyID := make(map[int64]*service.AccountAPIKey)
	for rows.Next() {
		var key service.AccountAPIKey
		var modelMappingBytes []byte
		if err := rows.Scan(
			&key.ID,
			&key.AccountID,
			&key.Name,
			&key.APIKey,
			&key.Priority,
			&key.Status,
			&key.ModelRestrictionMode,
			&modelMappingBytes,
			&key.GlobalCooldownUntil,
			&key.LastUsedAt,
			&key.RecentRequestCount,
			&key.RecentErrorCount,
			&key.CreatedAt,
			&key.UpdatedAt,
		); err != nil {
			return nil, err
		}
		key.ModelRestrictionMode = normalizeAccountAPIKeyModelRestrictionMode(key.ModelRestrictionMode)
		key.ModelMapping = parseAccountAPIKeyModelMapping(modelMappingBytes)
		key.ModelCooldowns = map[string]service.AccountAPIKeyModelCooldown{}
		out[key.AccountID] = append(out[key.AccountID], key)
		keyIDs = append(keyIDs, key.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for accountID := range out {
		for i := range out[accountID] {
			byKeyID[out[accountID][i].ID] = &out[accountID][i]
		}
	}
	if err := r.loadAccountAPIKeyModelCooldowns(ctx, keyIDs, byKeyID); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *accountRepository) loadAccountAPIKeyModelCooldowns(ctx context.Context, keyIDs []int64, byKeyID map[int64]*service.AccountAPIKey) error {
	if len(keyIDs) == 0 {
		return nil
	}
	rows, err := r.sql.QueryContext(ctx, `
		SELECT account_api_key_id, upstream_model, reason, status_code, cooldown_until,
		       last_error_at, last_error_message_sanitized
		FROM account_api_key_model_cooldowns
		WHERE account_api_key_id = ANY($1) AND cooldown_until > NOW()
	`, pq.Array(keyIDs))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var keyID int64
		var cooldown service.AccountAPIKeyModelCooldown
		if err := rows.Scan(
			&keyID,
			&cooldown.UpstreamModel,
			&cooldown.Reason,
			&cooldown.StatusCode,
			&cooldown.CooldownUntil,
			&cooldown.LastErrorAt,
			&cooldown.LastErrorMessageSanitized,
		); err != nil {
			return err
		}
		if key, ok := byKeyID[keyID]; ok {
			key.ModelCooldowns[cooldown.UpstreamModel] = cooldown
		}
	}
	return rows.Err()
}

func (r *accountRepository) accountIDByAPIKeyID(ctx context.Context, keyID int64) (int64, error) {
	rows, err := r.sql.QueryContext(ctx, `SELECT account_id FROM account_api_keys WHERE id = $1`, keyID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, err
		}
		return 0, sql.ErrNoRows
	}
	var accountID int64
	if err := rows.Scan(&accountID); err != nil {
		return 0, err
	}
	if rows.Next() {
		return 0, fmt.Errorf("multiple accounts found for api key %d", keyID)
	}
	return accountID, rows.Err()
}

func normalizeAccountAPIKeyStatus(status string) string {
	switch strings.TrimSpace(status) {
	case service.AccountAPIKeyStatusInactive:
		return service.AccountAPIKeyStatusInactive
	case service.AccountAPIKeyStatusError:
		return service.AccountAPIKeyStatusInactive
	default:
		return service.AccountAPIKeyStatusActive
	}
}

func normalizeAccountAPIKeyModelRestrictionMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case "mapping":
		return "mapping"
	default:
		return "whitelist"
	}
}

func normalizeAccountAPIKeyModelMapping(mapping map[string]string) map[string]string {
	out := make(map[string]string, len(mapping))
	for from, to := range mapping {
		from = strings.TrimSpace(from)
		to = strings.TrimSpace(to)
		if from == "" || to == "" {
			continue
		}
		out[from] = to
	}
	return out
}

func parseAccountAPIKeyModelMapping(raw []byte) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var mapping map[string]string
	if err := json.Unmarshal(raw, &mapping); err != nil {
		return nil
	}
	return normalizeAccountAPIKeyModelMapping(mapping)
}

func truncateStringForDB(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}
