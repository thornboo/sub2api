//go:build integration

package repository

import (
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *AccountRepoSuite) TestList_DefaultSortByNameAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "z-account"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "a-account"})

	accounts, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("a-account", accounts[0].Name)
	s.Require().Equal("z-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByPriorityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-priority", Priority: 10})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-priority", Priority: 90})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "priority",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-priority", accounts[0].Name)
	s.Require().Equal("low-priority", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByUpstreamEffectiveDiscount() {
	lowDiscount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "discount-low"})
	highDiscount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "discount-high"})
	unconfigured := mustCreateAccount(s.T(), s.client, &service.Account{Name: "discount-none"})

	s.mustBindAccountCostForSort(lowDiscount.ID, 5.6, 7, 0.5)
	s.mustBindAccountCostForSort(highDiscount.ID, 3.5, 7, 2)

	asc, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "upstream_effective_discount",
		SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(asc, 3)
	s.Require().Equal(lowDiscount.ID, asc[0].ID)
	s.Require().Equal(highDiscount.ID, asc[1].ID)
	s.Require().Equal(unconfigured.ID, asc[2].ID)

	desc, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "upstream_effective_discount",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(desc, 3)
	s.Require().Equal(highDiscount.ID, desc[0].ID)
	s.Require().Equal(lowDiscount.ID, desc[1].ID)
	s.Require().Equal(unconfigured.ID, desc[2].ID)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByUpstreamMultiplier() {
	lowMultiplier := mustCreateAccount(s.T(), s.client, &service.Account{Name: "multiplier-low"})
	highMultiplier := mustCreateAccount(s.T(), s.client, &service.Account{Name: "multiplier-high"})
	unconfigured := mustCreateAccount(s.T(), s.client, &service.Account{Name: "multiplier-none"})

	s.mustBindAccountCostForSort(lowMultiplier.ID, 7, 7, 0.5)
	s.mustBindAccountCostForSort(highMultiplier.ID, 7, 7, 2)

	asc, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "upstream_multiplier",
		SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(asc, 3)
	s.Require().Equal(lowMultiplier.ID, asc[0].ID)
	s.Require().Equal(highMultiplier.ID, asc[1].ID)
	s.Require().Equal(unconfigured.ID, asc[2].ID)

	desc, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "upstream_multiplier",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(desc, 3)
	s.Require().Equal(highMultiplier.ID, desc[0].ID)
	s.Require().Equal(lowMultiplier.ID, desc[1].ID)
	s.Require().Equal(unconfigured.ID, desc[2].ID)
}

func (s *AccountRepoSuite) TestListWithFilters_UpstreamDiscountRequiresRealNonSystemSnapshot() {
	systemAccount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "discount-system-supplier"})
	configuredOnlyAccount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "discount-config-only"})
	archivedSupplierAccount := mustCreateAccount(s.T(), s.client, &service.Account{Name: "discount-archived-supplier"})

	s.mustBindAccountCostForEligibility(systemAccount.ID, true, "active", true)
	s.mustBindAccountCostForEligibility(configuredOnlyAccount.ID, false, "active", false)
	s.mustBindAccountCostForEligibility(archivedSupplierAccount.ID, false, "archived", true)

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:     1,
		PageSize: 10,
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)

	byID := make(map[int64]service.Account, len(accounts))
	for _, account := range accounts {
		byID[account.ID] = account
	}
	s.Require().Nil(byID[systemAccount.ID].UpstreamEffectiveDiscount)
	s.Require().Nil(byID[configuredOnlyAccount.ID].UpstreamEffectiveDiscount)
	s.Require().NotNil(byID[archivedSupplierAccount.ID].UpstreamEffectiveDiscount)
	s.Require().InDelta(1, *byID[archivedSupplierAccount.ID].UpstreamEffectiveDiscount, 0.000001)
}

func (s *AccountRepoSuite) mustBindAccountCostForSort(accountID int64, effectiveCNYPerUSD, referenceFXRate, defaultMultiplier float64) {
	s.T().Helper()

	supplierID := s.mustInsertIDForAccountSort(
		`INSERT INTO upstream_suppliers (name) VALUES ($1) RETURNING id`,
		fmt.Sprintf("account-sort-supplier-%d", accountID),
	)
	poolID := s.mustInsertIDForAccountSort(
		`INSERT INTO upstream_cost_pools (supplier_id, name, reference_fx_rate, current_effective_cny_per_usd)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		supplierID,
		fmt.Sprintf("account-sort-pool-%d", accountID),
		referenceFXRate,
		effectiveCNYPerUSD,
	)
	snapshotID := s.mustInsertIDForAccountSort(
		`INSERT INTO upstream_cost_snapshots (cost_pool_id, effective_cny_per_usd, reference_fx_rate, calculation_method)
		 VALUES ($1, $2, $3, 'latest')
		 RETURNING id`,
		poolID,
		effectiveCNYPerUSD,
		referenceFXRate,
	)
	_, err := s.repo.sql.ExecContext(
		s.ctx,
		`UPDATE upstream_cost_pools SET current_snapshot_id = $2 WHERE id = $1`,
		poolID,
		snapshotID,
	)
	s.Require().NoError(err)
	_, err = s.repo.sql.ExecContext(
		s.ctx,
		`INSERT INTO upstream_account_cost_bindings (account_id, cost_pool_id, status, default_multiplier)
		 VALUES ($1, $2, 'active', $3)`,
		accountID,
		poolID,
		defaultMultiplier,
	)
	s.Require().NoError(err)
}

func (s *AccountRepoSuite) mustBindAccountCostForEligibility(accountID int64, isSystem bool, supplierStatus string, withSnapshot bool) {
	s.T().Helper()

	supplierID := s.mustInsertIDForAccountSort(
		`INSERT INTO upstream_suppliers (name, status, is_system, archived_at)
		 VALUES ($1, $2, $3, CASE WHEN $4 THEN NOW() ELSE NULL END)
		 RETURNING id`,
		fmt.Sprintf("account-eligibility-supplier-%d", accountID),
		supplierStatus,
		isSystem,
		supplierStatus == "archived",
	)
	poolID := s.mustInsertIDForAccountSort(
		`INSERT INTO upstream_cost_pools (supplier_id, name, reference_fx_rate, current_effective_cny_per_usd)
		 VALUES ($1, $2, 7, 7)
		 RETURNING id`,
		supplierID,
		fmt.Sprintf("account-eligibility-pool-%d", accountID),
	)
	if withSnapshot {
		snapshotID := s.mustInsertIDForAccountSort(
			`INSERT INTO upstream_cost_snapshots (cost_pool_id, effective_cny_per_usd, reference_fx_rate, calculation_method)
			 VALUES ($1, 7, 7, 'latest')
			 RETURNING id`,
			poolID,
		)
		_, err := s.repo.sql.ExecContext(s.ctx, `UPDATE upstream_cost_pools SET current_snapshot_id = $2 WHERE id = $1`, poolID, snapshotID)
		s.Require().NoError(err)
	}
	_, err := s.repo.sql.ExecContext(
		s.ctx,
		`INSERT INTO upstream_account_cost_bindings (account_id, cost_pool_id, status, default_multiplier)
		 VALUES ($1, $2, 'active', 1)`,
		accountID,
		poolID,
	)
	s.Require().NoError(err)
}

func (s *AccountRepoSuite) mustInsertIDForAccountSort(query string, args ...any) int64 {
	s.T().Helper()

	rows, err := s.repo.sql.QueryContext(s.ctx, query, args...)
	s.Require().NoError(err)
	defer func() { _ = rows.Close() }()

	s.Require().True(rows.Next(), "expected INSERT ... RETURNING id to return a row")
	var id int64
	s.Require().NoError(rows.Scan(&id))
	s.Require().NoError(rows.Err())
	return id
}
