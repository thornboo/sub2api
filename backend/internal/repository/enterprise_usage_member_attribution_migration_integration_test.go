//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseUsageMemberAttributionMigrationRequiresLedgerEvidence(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)

	var userID, memberID, keyID, accountID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, role, status, balance, concurrency, account_type)
		VALUES ($1, 'hash', 'user', 'active', 0, 5, 'enterprise')
		RETURNING id`, uniqueTestValue(t, "member-attribution-user")+"@example.test").Scan(&userID))
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO enterprise_members (enterprise_user_id, member_code, name, status)
		VALUES ($1, $2, 'Original name', 'active')
		RETURNING id`, userID, uniqueTestValue(t, "member-attribution-code")).Scan(&memberID))
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO api_keys (user_id, key, name, member_id, status)
		VALUES ($1, $2, 'Member key', $3, 'active')
		RETURNING id`, userID, "sk-"+integrationHash(t.Name() + ":member-attribution-key")[:32], memberID).Scan(&keyID))
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO accounts (name, platform, type)
		VALUES ($1, 'openai', 'apikey')
		RETURNING id`, uniqueTestValue(t, "member-attribution-account")).Scan(&accountID))

	eligibleRequestID := integrationHash(t.Name() + ":eligible")[:32]
	ineligibleRequestID := integrationHash(t.Name() + ":no-ledger")[:32]
	var eligibleUsageID, ineligibleUsageID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO usage_logs (user_id, api_key_id, account_id, request_id, model, actual_cost)
		VALUES ($1, $2, $3, $4, 'test-model', 1)
		RETURNING id`, userID, keyID, accountID, eligibleRequestID).Scan(&eligibleUsageID))
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO usage_logs (user_id, api_key_id, account_id, request_id, model, actual_cost)
		VALUES ($1, $2, $3, $4, 'test-model', 1)
		RETURNING id`, userID, keyID, accountID, ineligibleRequestID).Scan(&ineligibleUsageID))

	budgetRequestID := fmt.Sprintf("%d:%s", keyID, eligibleRequestID)
	var budgetEntryID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
		INSERT INTO enterprise_member_budget_entries
			(member_id, period_start, kind, request_id, amount_usd, idempotency_key)
		VALUES ($1, CURRENT_DATE, 'usage', $2, 1, $3)
		RETURNING id`, memberID, budgetRequestID, "usage:"+budgetRequestID).Scan(&budgetEntryID))

	_, err := tx.ExecContext(ctx, `UPDATE enterprise_members SET name = 'Renamed later' WHERE id = $1`, memberID)
	require.NoError(t, err)

	migrationSQL, err := migrations.FS.ReadFile("187_backfill_enterprise_member_usage_attribution.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	var attributedMemberID *int64
	var memberCodeSnapshot, memberNameSnapshot *string
	require.NoError(t, tx.QueryRowContext(ctx, `
		SELECT member_id, member_code_snapshot, member_name_snapshot
		FROM usage_logs WHERE id = $1`, eligibleUsageID).
		Scan(&attributedMemberID, &memberCodeSnapshot, &memberNameSnapshot))
	require.NotNil(t, attributedMemberID)
	require.Equal(t, memberID, *attributedMemberID)
	require.Nil(t, memberCodeSnapshot, "current member code must not be fabricated as a request-time snapshot")
	require.Nil(t, memberNameSnapshot, "current member name must not be fabricated as a request-time snapshot")

	var ineligibleMemberID *int64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT member_id FROM usage_logs WHERE id = $1`, ineligibleUsageID).Scan(&ineligibleMemberID))
	require.Nil(t, ineligibleMemberID, "current key membership without ledger evidence is not enough to rewrite history")

	var linkedUsageID *int64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT usage_log_id FROM enterprise_member_budget_entries WHERE id = $1`, budgetEntryID).Scan(&linkedUsageID))
	require.NotNil(t, linkedUsageID)
	require.Equal(t, eligibleUsageID, *linkedUsageID)

	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err, "member attribution backfill must be idempotent")
}
