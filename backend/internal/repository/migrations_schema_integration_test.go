//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate(t *testing.T) {
	tx := testTx(t)

	// Re-apply migrations to verify idempotency (no errors, no duplicate rows).
	require.NoError(t, ApplyMigrations(context.Background(), integrationDB))

	// schema_migrations should have at least the current migration set.
	var applied int
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&applied))
	require.GreaterOrEqual(t, applied, 7, "expected schema_migrations to contain applied migrations")

	// users: columns required by repository queries
	requireColumn(t, tx, "users", "username", "character varying", 100, false)
	requireColumn(t, tx, "users", "notes", "text", 0, false)

	// accounts: schedulable and rate-limit fields
	requireColumn(t, tx, "accounts", "notes", "text", 0, true)
	requireColumn(t, tx, "accounts", "schedulable", "boolean", 0, false)
	requireColumn(t, tx, "accounts", "rate_limited_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "rate_limit_reset_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "overload_until", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "session_window_status", "character varying", 20, true)
	requireIndex(t, tx, "accounts", "idx_accounts_autopause_expiry_due")

	// api_keys: key length should be 128
	requireColumn(t, tx, "api_keys", "key", "character varying", 128, false)

	// redeem_codes: subscription fields
	requireColumn(t, tx, "redeem_codes", "group_id", "bigint", 0, true)
	requireColumn(t, tx, "redeem_codes", "validity_days", "integer", 0, false)

	// usage_logs: billing_type used by filters/stats
	requireColumn(t, tx, "usage_logs", "billing_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "request_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_mode", "boolean", 0, false)
	requireColumn(t, tx, "usage_logs", "image_input_size", "character varying", 32, true)
	requireColumn(t, tx, "usage_logs", "image_output_size", "character varying", 32, true)
	requireColumn(t, tx, "usage_logs", "image_size_source", "character varying", 16, true)
	requireColumn(t, tx, "usage_logs", "image_size_breakdown", "jsonb", 0, true)
	requireColumn(t, tx, "usage_logs", "video_count", "integer", 0, false)
	requireColumn(t, tx, "usage_logs", "video_resolution", "character varying", 10, true)
	requireColumn(t, tx, "usage_logs", "video_duration_seconds", "integer", 0, true)
	requireConstraintDefinitionContains(
		t,
		tx,
		"usage_logs",
		"usage_logs_image_size_source_check",
		"image_size_source",
		"'output'",
		"'input'",
		"'default'",
		"'legacy'",
	)
	requireConstraintDefinitionContains(
		t,
		tx,
		"usage_logs",
		"usage_logs_image_billing_size_check",
		"image_count",
		"billing_mode",
		"'video'",
		"video_count",
		"image_size IS NOT NULL",
		"'1K'",
		"'2K'",
		"'4K'",
		"'mixed'",
	)
	requireForeignKeyOnDelete(t, tx, "usage_logs", "user_id", "users", "RESTRICT")
	requireForeignKeyOnDelete(t, tx, "usage_logs", "api_key_id", "api_keys", "RESTRICT")
	requireForeignKeyOnDelete(t, tx, "usage_logs", "account_id", "accounts", "RESTRICT")

	// usage_billing_dedup: billing idempotency narrow table
	var usageBillingDedupRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup')").Scan(&usageBillingDedupRegclass))
	require.True(t, usageBillingDedupRegclass.Valid, "expected usage_billing_dedup table to exist")
	requireColumn(t, tx, "usage_billing_dedup", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_request_api_key")
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_created_at_brin")

	var usageBillingDedupArchiveRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup_archive')").Scan(&usageBillingDedupArchiveRegclass))
	require.True(t, usageBillingDedupArchiveRegclass.Valid, "expected usage_billing_dedup_archive table to exist")
	requireColumn(t, tx, "usage_billing_dedup_archive", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup_archive", "usage_billing_dedup_archive_pkey")

	// enterprise member settlement outbox: successful upstream usage survives a
	// failed local billing transaction and can be replayed idempotently.
	requireColumn(t, tx, "enterprise_member_usage_settlement_outbox", "command_payload", "jsonb", 0, false)
	requireColumn(t, tx, "enterprise_member_usage_settlement_outbox", "enterprise_user_id", "bigint", 0, false)
	requireColumn(t, tx, "enterprise_member_usage_settlement_outbox", "request_fingerprint", "character varying", 64, false)
	requireColumn(t, tx, "enterprise_member_usage_settlement_outbox", "next_attempt_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "enterprise_member_usage_settlement_outbox", "idx_enterprise_member_usage_settlement_outbox_due")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_usage_settlement_outbox", "enterprise_member_usage_settlement_outbox_key_member_owner_fk", "api_key_id", "member_id", "enterprise_user_id", "api_keys", "user_id", "ON DELETE RESTRICT")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_usage_settlement_outbox", "enterprise_member_usage_settlement_outbox_member_owner_fk", "member_id", "enterprise_user_id", "enterprise_members", "ON DELETE RESTRICT")

	// settings table should exist
	var settingsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.settings')").Scan(&settingsRegclass))
	require.True(t, settingsRegclass.Valid, "expected settings table to exist")

	// security_secrets table should exist
	var securitySecretsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.security_secrets')").Scan(&securitySecretsRegclass))
	require.True(t, securitySecretsRegclass.Valid, "expected security_secrets table to exist")

	// scheduler_outbox pending dedup support
	requireColumn(t, tx, "scheduler_outbox", "dedup_key", "text", 0, true)
	requireIndex(t, tx, "scheduler_outbox", "idx_scheduler_outbox_pending_dedup_key")

	// ops_system_logs: API key id index for operational log triage
	requireColumn(t, tx, "ops_system_logs", "api_key_id", "bigint", 0, true)
	requireIndex(t, tx, "ops_system_logs", "idx_ops_system_logs_api_key_id_created_at")

	// user_allowed_groups table should exist
	var uagRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.user_allowed_groups')").Scan(&uagRegclass))
	require.True(t, uagRegclass.Valid, "expected user_allowed_groups table to exist")

	// user_subscriptions: deleted_at for soft delete support (migration 012)
	requireColumn(t, tx, "user_subscriptions", "deleted_at", "timestamp with time zone", 0, true)

	// orphan_allowed_groups_audit table should exist (migration 013)
	var orphanAuditRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.orphan_allowed_groups_audit')").Scan(&orphanAuditRegclass))
	require.True(t, orphanAuditRegclass.Valid, "expected orphan_allowed_groups_audit table to exist")

	// account_groups: created_at should be timestamptz
	requireColumn(t, tx, "account_groups", "created_at", "timestamp with time zone", 0, false)

	// user_allowed_groups: created_at should be timestamptz
	requireColumn(t, tx, "user_allowed_groups", "created_at", "timestamp with time zone", 0, false)
}

func TestMigrationsRunner_AuthIdentityAndPaymentSchemaStayAligned(t *testing.T) {
	tx := testTx(t)

	requireColumn(t, tx, "auth_identity_migration_reports", "report_type", "character varying", 80, false)
	requireColumn(t, tx, "users", "signup_source", "character varying", 20, false)
	requireColumnDefaultContains(t, tx, "users", "signup_source", "email")
	requireConstraintDefinitionContains(
		t,
		tx,
		"users",
		"users_signup_source_check",
		"signup_source",
		"'email'",
		"'linuxdo'",
		"'wechat'",
		"'oidc'",
	)

	requireForeignKeyOnDelete(t, tx, "auth_identities", "user_id", "users", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "auth_identity_channels", "identity_id", "auth_identities", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "pending_auth_sessions", "target_user_id", "users", "SET NULL")
	requireForeignKeyOnDelete(t, tx, "identity_adoption_decisions", "pending_auth_session_id", "pending_auth_sessions", "CASCADE")
	requireForeignKeyOnDelete(t, tx, "identity_adoption_decisions", "identity_id", "auth_identities", "SET NULL")

	requireIndex(t, tx, "payment_orders", "paymentorder_out_trade_no")
	requirePartialUniqueIndexDefinition(t, tx, "payment_orders", "paymentorder_out_trade_no", "out_trade_no", "WHERE")
	requireIndexAbsent(t, tx, "payment_orders", "paymentorder_out_trade_no_unique")
}

func TestMigrationsRunner_EnterpriseMemberSchemaStaysAligned(t *testing.T) {
	tx := testTx(t)

	requireColumn(t, tx, "users", "account_type", "character varying", 20, false)
	requireColumn(t, tx, "users", "enterprise_disabled_at", "timestamp with time zone", 0, true)
	requireColumnDefaultContains(t, tx, "users", "account_type", "individual")
	requireConstraintDefinitionContains(t, tx, "users", "users_account_type_check", "individual", "enterprise")
	requireIndex(t, tx, "users", "idx_users_account_type")

	requireTable(t, tx, "enterprise_members")
	requireColumn(t, tx, "enterprise_members", "enterprise_user_id", "bigint", 0, false)
	requireColumn(t, tx, "enterprise_members", "member_code", "character varying", 100, false)
	requireColumn(t, tx, "enterprise_members", "monthly_limit_usd", "numeric", 0, false)
	requireColumn(t, tx, "enterprise_members", "version", "bigint", 0, false)
	requireColumn(t, tx, "enterprise_members", "deleted_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "enterprise_members", "removed_at", "timestamp with time zone", 0, true)
	requireForeignKeyOnDelete(t, tx, "enterprise_members", "enterprise_user_id", "users", "RESTRICT")
	requireConstraintDefinitionContains(t, tx, "enterprise_members", "enterprise_members_owner_code_unique", "enterprise_user_id", "member_code")
	requireConstraintDefinitionContains(t, tx, "enterprise_members", "enterprise_members_id_owner_unique", "id", "enterprise_user_id")
	requireConstraintDefinitionContains(t, tx, "enterprise_members", "enterprise_members_status_check", "active", "disabled")
	requireConstraintDefinitionContains(t, tx, "enterprise_members", "enterprise_members_removed_requires_archive_check", "removed_at", "deleted_at")
	requireIndex(t, tx, "enterprise_members", "idx_enterprise_members_owner_status")
	requireIndex(t, tx, "enterprise_members", "enterprise_members_owner_code_ci_unique")
	requireIndex(t, tx, "enterprise_members", "idx_enterprise_members_owner_visible")

	requireColumn(t, tx, "api_keys", "member_id", "bigint", 0, true)
	requireConstraintDefinitionContains(t, tx, "api_keys", "api_keys_member_owner_fk", "member_id", "user_id", "enterprise_members", "enterprise_user_id", "ON DELETE RESTRICT")
	requireConstraintDefinitionContains(t, tx, "api_keys", "api_keys_member_group_exclusive_check", "member_id IS NULL", "group_id IS NULL")
	requireIndex(t, tx, "api_keys", "idx_api_keys_member_id")

	requireTable(t, tx, "enterprise_member_group_bindings")
	requireColumn(t, tx, "enterprise_member_group_bindings", "sort_order", "integer", 0, false)
	requireForeignKeyOnDelete(t, tx, "enterprise_member_group_bindings", "member_id", "enterprise_members", "RESTRICT")
	requireForeignKeyOnDelete(t, tx, "enterprise_member_group_bindings", "group_id", "groups", "RESTRICT")
	requireIndex(t, tx, "enterprise_member_group_bindings", "idx_enterprise_member_group_bindings_order")

	requireColumn(t, tx, "usage_logs", "member_id", "bigint", 0, true)
	requireColumn(t, tx, "usage_logs", "member_code_snapshot", "character varying", 100, true)
	requireColumn(t, tx, "usage_logs", "member_name_snapshot", "character varying", 100, true)
	requireForeignKeyOnDelete(t, tx, "usage_logs", "member_id", "enterprise_members", "RESTRICT")
	requireIndex(t, tx, "usage_logs", "idx_usage_logs_member_created_at")

	requireColumn(t, tx, "batch_image_jobs", "member_id", "bigint", 0, true)
	requireColumn(t, tx, "batch_image_jobs", "member_budget_request_id", "character varying", 128, true)
	requireIndex(t, tx, "batch_image_jobs", "idx_batch_image_jobs_member_budget_request")

	requireTable(t, tx, "enterprise_member_budget_periods")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_budget_periods", "enterprise_member_budget_periods_member_period_unique", "member_id", "period_start")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_budget_periods", "enterprise_member_budget_periods_amounts_check", "used_usd", "reserved_usd")
	requireTable(t, tx, "enterprise_member_budget_reservations")
	requireColumn(t, tx, "enterprise_member_budget_reservations", "group_id", "bigint", 0, true)
	requireColumn(t, tx, "enterprise_member_budget_reservations", "request_payload_hash", "character varying", 64, false)
	requireColumn(t, tx, "enterprise_member_budget_reservations", "outcome_reason", "character varying", 64, false)
	requireColumn(t, tx, "enterprise_member_budget_reservations", "reconcile_attempts", "integer", 0, false)
	requireColumn(t, tx, "enterprise_member_budget_reservations", "last_reconcile_at", "timestamp with time zone", 0, true)
	requireConstraintDefinitionContains(t, tx, "enterprise_member_budget_reservations", "enterprise_member_budget_reservations_status_check", "reserved", "settled", "released", "expired", "ambiguous")
	requireIndex(t, tx, "enterprise_member_budget_reservations", "idx_enterprise_member_budget_reservations_expiry")
	requireIndex(t, tx, "enterprise_member_budget_reservations", "idx_enterprise_member_budget_reservations_ambiguous")
	requireTable(t, tx, "enterprise_member_budget_entries")
	requireTable(t, tx, "enterprise_member_rate_limit_periods")
	requireConstraintDefinitionContains(t, tx, "enterprise_members", "enterprise_members_rate_limits_check", "rate_limit_5h", "rate_limit_1d", "rate_limit_7d")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_rate_limit_periods", "enterprise_member_rate_limit_usage_check", "usage_5h", "usage_1d", "usage_7d")
	requireIndex(t, tx, "enterprise_member_audit_logs", "enterprise_member_usage_adjustment_idempotency_unique")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_budget_entries", "enterprise_member_budget_entries_kind_check", "usage", "migration_opening", "manual_adjustment", "reconciliation")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_budget_entries", "enterprise_member_budget_entries_usage_shape_check", "request_id", "usage_log_id")

	requireTable(t, tx, "grok_media_tasks")
	requireConstraintDefinitionContains(t, tx, "grok_media_tasks", "grok_media_tasks_member_owner_fk", "member_id", "user_id", "enterprise_members", "enterprise_user_id", "ON DELETE RESTRICT")
	requireIndex(t, tx, "grok_media_tasks", "idx_grok_media_tasks_member_created")

	requireTable(t, tx, "enterprise_member_audit_logs")
	requireColumn(t, tx, "enterprise_member_audit_logs", "before_data", "jsonb", 0, false)
	requireColumn(t, tx, "enterprise_member_audit_logs", "after_data", "jsonb", 0, false)
	requireConstraintDefinitionContains(t, tx, "enterprise_member_audit_logs", "enterprise_member_audit_logs_payload_shape_check", "before_data", "after_data", "metadata")
	requireIndex(t, tx, "enterprise_member_audit_logs", "idx_enterprise_member_audit_owner_created")
	requireTrigger(t, tx, "enterprise_member_audit_logs", "enterprise_member_audit_immutable")
	requireTrigger(t, tx, "enterprise_members", "enterprise_member_audit_member")
	requireTrigger(t, tx, "enterprise_member_group_bindings", "enterprise_member_audit_group_binding")
	requireTrigger(t, tx, "api_keys", "enterprise_member_audit_key")
	requireTrigger(t, tx, "enterprise_member_budget_entries", "enterprise_member_audit_budget")
	requireTrigger(t, tx, "enterprise_member_budget_entries", "enterprise_member_budget_entry_immutable")
	requireTrigger(t, tx, "enterprise_member_import_jobs", "enterprise_member_audit_import_job")
	requireTrigger(t, tx, "users", "enterprise_member_audit_account")

	requireTable(t, tx, "enterprise_member_import_jobs")
	requireColumn(t, tx, "enterprise_member_import_jobs", "selected_rows", "jsonb", 0, false)
	requireColumn(t, tx, "enterprise_member_import_jobs", "default_group_ids", "jsonb", 0, false)
	requireColumn(t, tx, "enterprise_member_import_jobs", "activate_members", "boolean", 0, false)
	requireColumn(t, tx, "enterprise_member_import_jobs", "import_policy_version", "smallint", 0, false)
	requireColumn(t, tx, "enterprise_member_import_jobs", "commit_protocol_version", "smallint", 0, false)
	requireColumn(t, tx, "enterprise_member_import_jobs", "locked_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "enterprise_member_import_jobs", "lock_owner", "character varying", 128, true)
	requireColumn(t, tx, "enterprise_member_import_jobs", "attempt_count", "integer", 0, false)
	requireColumn(t, tx, "enterprise_member_import_jobs", "result_secrets_ciphertext", "text", 0, true)
	requireColumn(t, tx, "enterprise_member_import_jobs", "result_secrets_consumed_at", "timestamp with time zone", 0, true)
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_jobs_status_check", "previewed", "queued", "queued_v2", "processing", "processing_v2", "completed", "failed")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_jobs_selected_rows_shape_check", "jsonb_typeof", "array")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_jobs_default_group_ids_shape_check", "jsonb_typeof", "array")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_jobs_policy_version_check", "import_policy_version", "1", "2")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_jobs_commit_protocol_version_check", "commit_protocol_version", "1", "2")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_jobs_policy_v2_activation_groups_check", "import_policy_version", "activate_members", "default_group_ids")
	requireIndex(t, tx, "enterprise_member_import_jobs", "idx_enterprise_member_import_jobs_queue")
	requireIndex(t, tx, "enterprise_member_import_jobs", "idx_enterprise_member_import_jobs_queue_v2")
	requireTrigger(t, tx, "enterprise_member_import_jobs", "enterprise_member_import_queue_protocol_guard")

	requireTable(t, tx, "enterprise_member_import_usage_baselines")
	requireColumn(t, tx, "enterprise_member_import_usage_baselines", "billed_usd", "numeric", 0, false)
	for _, column := range []string{"total_tokens", "input_tokens", "output_tokens", "cache_tokens", "cache_creation_tokens", "cache_read_tokens"} {
		requireNumericColumn(t, tx, "enterprise_member_import_usage_baselines", column, 21, 2, false)
	}
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_usage_baselines", "enterprise_member_import_usage_baselines_source_unique", "import_job_id", "source_row_number")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_usage_baselines", "enterprise_member_import_usage_baselines_values_check", "billed_usd", "total_tokens", "cache_read_tokens")
	requireConstraintDefinitionContains(t, tx, "enterprise_member_import_usage_baselines", "enterprise_member_import_usage_baselines_key_member_owner_fk", "api_key_id", "member_id", "enterprise_user_id", "api_keys", "user_id", "ON DELETE RESTRICT")
	requireIndex(t, tx, "api_keys", "idx_api_keys_id_member_owner")
	requireIndex(t, tx, "enterprise_member_import_usage_baselines", "idx_enterprise_member_import_usage_baselines_member_period")
	requireIndex(t, tx, "enterprise_member_import_usage_baselines", "idx_enterprise_member_import_usage_baselines_owner_period")
	requireTrigger(t, tx, "enterprise_member_import_usage_baselines", "enterprise_member_import_usage_baseline_immutable")
}

func TestMigration174BackfillsOnlyUnambiguousRealSupplierDefaults(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	insertSupplier := func(name string, isSystem bool) int64 {
		t.Helper()
		var supplierID int64
		require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO upstream_suppliers (name, is_system)
VALUES ($1, $2)
RETURNING id`, name, isSystem).Scan(&supplierID))
		return supplierID
	}
	insertPool := func(supplierID int64, name string) int64 {
		t.Helper()
		var poolID int64
		require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO upstream_cost_pools (supplier_id, name, is_default)
VALUES ($1, $2, FALSE)
RETURNING id`, supplierID, name).Scan(&poolID))
		return poolID
	}
	loadDefault := func(poolID int64) bool {
		t.Helper()
		var isDefault bool
		require.NoError(t, tx.QueryRowContext(ctx, `
SELECT is_default
FROM upstream_cost_pools
WHERE id = $1`, poolID).Scan(&isDefault))
		return isDefault
	}

	canonicalSupplierID := insertSupplier("migration-174-canonical", false)
	canonicalPoolID := insertPool(canonicalSupplierID, "主余额池")
	secondaryPoolID := insertPool(canonicalSupplierID, "活动备用池")

	renamedSupplierID := insertSupplier("migration-174-renamed", false)
	renamedOnlyPoolID := insertPool(renamedSupplierID, "历史改名资金池")

	ambiguousSupplierID := insertSupplier("migration-174-ambiguous", false)
	ambiguousPoolAID := insertPool(ambiguousSupplierID, "历史资金池 A")
	ambiguousPoolBID := insertPool(ambiguousSupplierID, "历史资金池 B")

	systemSupplierID := insertSupplier("migration-174-system", true)
	systemCanonicalPoolID := insertPool(systemSupplierID, "主余额池")
	systemAccountPoolID := insertPool(systemSupplierID, "账号默认资金池 #174: migration")

	migrationSQL, err := migrations.FS.ReadFile("174_upstream_cost_pool_defaults.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	require.True(t, loadDefault(canonicalPoolID), "canonical real-supplier pool should be the default")
	require.False(t, loadDefault(secondaryPoolID), "secondary pool must not replace the canonical default")
	require.True(t, loadDefault(renamedOnlyPoolID), "a real supplier's sole active pool should recover as default")
	require.False(t, loadDefault(ambiguousPoolAID), "multiple renamed pools are ambiguous")
	require.False(t, loadDefault(ambiguousPoolBID), "multiple renamed pools are ambiguous")
	require.False(t, loadDefault(systemCanonicalPoolID), "system suppliers must never receive a default pool")
	require.False(t, loadDefault(systemAccountPoolID), "phase-1 account pools must remain non-default")
}

func requireIndex(t *testing.T, tx *sql.Tx, table, index string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_indexes
	WHERE schemaname = 'public'
	  AND tablename = $1
	  AND indexname = $2
)
`, table, index).Scan(&exists)
	require.NoError(t, err, "query pg_indexes for %s.%s", table, index)
	require.True(t, exists, "expected index %s on %s", index, table)
}

func requireTable(t *testing.T, tx *sql.Tx, table string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT to_regclass('public.' || $1) IS NOT NULL
`, table).Scan(&exists)
	require.NoError(t, err, "query table %s", table)
	require.True(t, exists, "expected table %s to exist", table)
}

func requireTrigger(t *testing.T, tx *sql.Tx, table, trigger string) {
	t.Helper()

	var enabled string
	err := tx.QueryRowContext(context.Background(), `
SELECT t.tgenabled::text
FROM pg_trigger t
JOIN pg_class tbl ON tbl.oid = t.tgrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND tbl.relname = $1
  AND t.tgname = $2
  AND NOT t.tgisinternal
`, table, trigger).Scan(&enabled)
	require.NoError(t, err, "query trigger %s on %s", trigger, table)
	require.Contains(t, []string{"O", "A", "R"}, enabled, "expected trigger %s on %s to be enabled", trigger, table)
}

func requireIndexAbsent(t *testing.T, tx *sql.Tx, table, index string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_indexes
	WHERE schemaname = 'public'
	  AND tablename = $1
	  AND indexname = $2
)
`, table, index).Scan(&exists)
	require.NoError(t, err, "query pg_indexes for %s.%s", table, index)
	require.False(t, exists, "expected index %s on %s to be absent", index, table)
}

func requirePartialUniqueIndexDefinition(t *testing.T, tx *sql.Tx, table, index string, fragments ...string) {
	t.Helper()

	var (
		unique bool
		def    string
	)

	err := tx.QueryRowContext(context.Background(), `
SELECT
	i.indisunique,
	pg_get_indexdef(i.indexrelid)
FROM pg_class idx
JOIN pg_index i ON i.indexrelid = idx.oid
JOIN pg_class tbl ON tbl.oid = i.indrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND tbl.relname = $1
  AND idx.relname = $2
`, table, index).Scan(&unique, &def)
	require.NoError(t, err, "query index definition for %s.%s", table, index)
	require.True(t, unique, "expected index %s on %s to be unique", index, table)

	for _, fragment := range fragments {
		require.Contains(t, def, fragment, "expected index definition for %s.%s to contain %q", table, index, fragment)
	}
}

func requireForeignKeyOnDelete(t *testing.T, tx *sql.Tx, table, column, refTable, expected string) {
	t.Helper()

	var actual string
	err := tx.QueryRowContext(context.Background(), `
SELECT CASE c.confdeltype
	WHEN 'a' THEN 'NO ACTION'
	WHEN 'r' THEN 'RESTRICT'
	WHEN 'c' THEN 'CASCADE'
	WHEN 'n' THEN 'SET NULL'
	WHEN 'd' THEN 'SET DEFAULT'
END
FROM pg_constraint c
JOIN pg_class tbl ON tbl.oid = c.conrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
JOIN pg_class ref_tbl ON ref_tbl.oid = c.confrelid
JOIN pg_attribute attr ON attr.attrelid = tbl.oid AND attr.attnum = ANY(c.conkey)
WHERE ns.nspname = 'public'
  AND c.contype = 'f'
  AND tbl.relname = $1
  AND attr.attname = $2
  AND ref_tbl.relname = $3
LIMIT 1
`, table, column, refTable).Scan(&actual)
	require.NoError(t, err, "query foreign key action for %s.%s -> %s", table, column, refTable)
	require.Equal(t, expected, actual, "unexpected ON DELETE action for %s.%s -> %s", table, column, refTable)
}

func requireConstraintDefinitionContains(t *testing.T, tx *sql.Tx, table, constraint string, fragments ...string) {
	t.Helper()

	var def string
	err := tx.QueryRowContext(context.Background(), `
SELECT pg_get_constraintdef(c.oid)
FROM pg_constraint c
JOIN pg_class tbl ON tbl.oid = c.conrelid
JOIN pg_namespace ns ON ns.oid = tbl.relnamespace
WHERE ns.nspname = 'public'
  AND tbl.relname = $1
  AND c.conname = $2
`, table, constraint).Scan(&def)
	require.NoError(t, err, "query constraint definition for %s.%s", table, constraint)

	for _, fragment := range fragments {
		require.Contains(t, def, fragment, "expected constraint definition for %s.%s to contain %q", table, constraint, fragment)
	}
}

func requireColumnDefaultContains(t *testing.T, tx *sql.Tx, table, column string, fragments ...string) {
	t.Helper()

	var columnDefault sql.NullString
	err := tx.QueryRowContext(context.Background(), `
SELECT column_default
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&columnDefault)
	require.NoError(t, err, "query column_default for %s.%s", table, column)
	require.True(t, columnDefault.Valid, "expected column_default for %s.%s", table, column)

	for _, fragment := range fragments {
		require.Contains(t, columnDefault.String, fragment, "expected default for %s.%s to contain %q", table, column, fragment)
	}
}

func requireColumn(t *testing.T, tx *sql.Tx, table, column, dataType string, maxLen int, nullable bool) {
	t.Helper()

	var row struct {
		DataType string
		MaxLen   sql.NullInt64
		Nullable string
	}

	err := tx.QueryRowContext(context.Background(), `
SELECT
  data_type,
  character_maximum_length,
  is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&row.DataType, &row.MaxLen, &row.Nullable)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.Equal(t, dataType, row.DataType, "data_type mismatch for %s.%s", table, column)

	if maxLen > 0 {
		require.True(t, row.MaxLen.Valid, "expected maxLen for %s.%s", table, column)
		require.Equal(t, int64(maxLen), row.MaxLen.Int64, "maxLen mismatch for %s.%s", table, column)
	}

	if nullable {
		require.Equal(t, "YES", row.Nullable, "nullable mismatch for %s.%s", table, column)
	} else {
		require.Equal(t, "NO", row.Nullable, "nullable mismatch for %s.%s", table, column)
	}
}

func requireNumericColumn(t *testing.T, tx *sql.Tx, table, column string, precision, scale int, nullable bool) {
	t.Helper()

	var row struct {
		DataType  string
		Precision int
		Scale     int
		Nullable  string
	}
	err := tx.QueryRowContext(context.Background(), `
SELECT data_type, numeric_precision, numeric_scale, is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&row.DataType, &row.Precision, &row.Scale, &row.Nullable)
	require.NoError(t, err, "query numeric column for %s.%s", table, column)
	require.Equal(t, "numeric", row.DataType, "data_type mismatch for %s.%s", table, column)
	require.Equal(t, precision, row.Precision, "numeric_precision mismatch for %s.%s", table, column)
	require.Equal(t, scale, row.Scale, "numeric_scale mismatch for %s.%s", table, column)
	if nullable {
		require.Equal(t, "YES", row.Nullable, "nullable mismatch for %s.%s", table, column)
	} else {
		require.Equal(t, "NO", row.Nullable, "nullable mismatch for %s.%s", table, column)
	}
}
