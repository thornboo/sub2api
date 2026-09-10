//go:build integration

package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	svc "github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSupplierConsumptionOverviewUsesWalletHistoryAndCredits(t *testing.T) {
	ctx := context.Background()
	admin := newUpstreamSupplierBalanceAdmin(t).(interface {
		GetUpstreamSupplierRechargeOverview(context.Context) (*svc.UpstreamSupplierRechargeOverview, error)
	})
	before, err := admin.GetUpstreamSupplierRechargeOverview(ctx)
	require.NoError(t, err)
	start := timezone.Today()
	closing := time.Now().Add(-time.Second)
	if !closing.After(start) {
		t.Skip("requires one second of today's interval")
	}
	opening := start.Add(-time.Minute)
	insertSupplier := func(unit string) (int64, int64) {
		var id, pool int64
		err := serviceIntegrationDB.QueryRowContext(ctx, `INSERT INTO upstream_suppliers (name,balance_config,balance_snapshot) VALUES ($1,'{"enabled":true}','{"status":"ok"}') RETURNING id`, fmt.Sprintf("consumption-%s-%d", unit, time.Now().UnixNano())).Scan(&id)
		require.NoError(t, err)
		err = serviceIntegrationDB.QueryRowContext(ctx, `INSERT INTO upstream_cost_pools (supplier_id,name) VALUES ($1,'consumption-test') RETURNING id`, id).Scan(&pool)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, err := serviceIntegrationDB.ExecContext(ctx, `DELETE FROM upstream_recharge_records WHERE cost_pool_id=$1`, pool)
			require.NoError(t, err)
			_, err = serviceIntegrationDB.ExecContext(ctx, `DELETE FROM upstream_cost_pools WHERE id=$1`, pool)
			require.NoError(t, err)
			_, err = serviceIntegrationDB.ExecContext(ctx, `DELETE FROM upstream_suppliers WHERE id=$1`, id)
			require.NoError(t, err)
		})
		return id, pool
	}
	sample := func(id, revision int64, at time.Time, balance float64, unit string) {
		_, err := serviceIntegrationDB.ExecContext(ctx, `INSERT INTO upstream_supplier_balance_samples (supplier_id,revision,sampled_at,balance,unit) VALUES ($1,$2,$3,$4,$5)`, id, revision, at, balance, unit)
		require.NoError(t, err)
	}
	credit := func(pool int64, kind string, at time.Time, amount float64, voided, deleted bool) {
		_, err := serviceIntegrationDB.ExecContext(ctx, `INSERT INTO upstream_recharge_records (cost_pool_id,type,recorded_at,paid_amount,received_credit_amount,voided_at,deleted_at) VALUES ($1,$2,$3,1,$4,CASE WHEN $5 THEN NOW() END,CASE WHEN $6 THEN NOW() END)`, pool, kind, at, amount, voided, deleted)
		require.NoError(t, err)
	}
	usd, pool := insertSupplier("USD")
	// Nearest observation before midnight wins over an earlier nearby one.
	sample(usd, 1, start.Add(-8*time.Minute), 999, "USD")
	sample(usd, 1, opening, 100, "USD")
	sample(usd, 1, closing, 90, "USD")
	sample(usd, 1, closing.Add(24*time.Hour), 5000, "USD")
	credit(pool, "recharge", opening, 999, false, false) // already included in opening balance
	credit(pool, "recharge", start, 20, false, false)
	credit(pool, "bonus", start, 5, false, false)
	credit(pool, "adjustment", start, 3, false, false)
	credit(pool, "recharge", start, 999, true, false)
	credit(pool, "recharge", start, 999, false, true)
	credit(pool, "recharge", closing.Add(time.Hour), 999, false, false)
	creditsID, creditsPool := insertSupplier("Credits")
	sample(creditsID, 1, opening, 80, "Credits")
	sample(creditsID, 1, closing, 70, "Credits")
	revisedID, _ := insertSupplier("revision")
	sample(revisedID, 1, opening, 500, "USD")
	sample(revisedID, 1, closing, 100, "USD")
	_, err = serviceIntegrationDB.ExecContext(ctx, `UPDATE upstream_suppliers SET balance_revision=2 WHERE id=$1`, revisedID)
	require.NoError(t, err)
	sample(revisedID, 2, closing, 1000, "USD")

	overview, err := admin.GetUpstreamSupplierRechargeOverview(ctx)
	require.NoError(t, err)
	total := func(period svc.UpstreamSupplierConsumptionPeriod, unit string) float64 {
		for _, entry := range period.Totals {
			if entry.Unit == unit {
				return entry.Amount
			}
		}
		return 0
	}
	require.InDelta(t, 48, total(overview.Consumption.Today, "USD")-total(before.Consumption.Today, "USD"), 1e-6)
	require.Len(t, overview.Consumption.Today.Totals, 1)
	require.Equal(t, "USD", overview.Consumption.Today.Totals[0].Unit)
	require.Contains(t, overview.Consumption.Today.Issues, svc.UpstreamSupplierConsumptionIssue{SupplierID: revisedID, Reason: "no_history"})
	require.Contains(t, overview.Consumption.Last7Days.Issues, svc.UpstreamSupplierConsumptionIssue{SupplierID: usd, Reason: "insufficient_history"})

	// The owner's 1:1 quota convention includes USD ledger credits for Credits wallets.
	credit(creditsPool, "recharge", start, 5, false, false)
	overview, err = admin.GetUpstreamSupplierRechargeOverview(ctx)
	require.NoError(t, err)
	require.InDelta(t, 53, total(overview.Consumption.Today, "USD")-total(before.Consumption.Today, "USD"), 1e-6)
	require.NotContains(t, overview.Consumption.Today.Issues, svc.UpstreamSupplierConsumptionIssue{SupplierID: creditsID, Reason: "unit_mismatch"})
	// Conversion belongs to the overview, not to persisted upstream observations.
	var originalUnit string
	require.NoError(t, serviceIntegrationDB.QueryRowContext(ctx, `SELECT unit FROM upstream_supplier_balance_samples WHERE supplier_id=$1 ORDER BY sampled_at DESC LIMIT 1`, creditsID).Scan(&originalUnit))
	require.Equal(t, "Credits", originalUnit)
}
