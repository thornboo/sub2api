package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestResolvedPricingUpperBoundUsesWorstTokenAndTierPrices(t *testing.T) {
	input := 0.02
	output := 0.05
	pricing := &ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: &ModelPricing{InputPricePerToken: 0.01, OutputPricePerToken: 0.03},
		Intervals:   []PricingInterval{{InputPrice: &input, OutputPrice: &output}},
	}
	require.InDelta(t, 0.2, resolvedPricingUpperBound(pricing, 5, 2, 1), 1e-12)
}

func TestEnterpriseMemberBudgetRequestIDScopesClientIDByKey(t *testing.T) {
	require.Equal(t, "42:client-request", EnterpriseMemberBudgetRequestID(42, " client-request "))
}

func TestNormalizeEnterpriseMemberBudgetRequestIDMatchesUnifiedBilling(t *testing.T) {
	requestID, err := normalizeEnterpriseMemberBudgetRequestID(" request-uuid ")
	require.NoError(t, err)
	require.Equal(t, "client:request-uuid", requestID)

	requestID, err = normalizeEnterpriseMemberBudgetRequestID("client:request-uuid")
	require.NoError(t, err)
	require.Equal(t, "client:request-uuid", requestID)
}

func TestEnterpriseMemberEndpointIsBillableDelegatesAsyncBatchLifecycle(t *testing.T) {
	require.False(t, enterpriseMemberEndpointIsBillable("/v1/images/batches"))
	require.False(t, enterpriseMemberEndpointIsBillable("/v1/images/batches/job-1"))
	require.True(t, enterpriseMemberEndpointIsBillable("/v1/images/generations"))
}

func TestEnterpriseMemberCurrentBudgetPeriodUsesShanghaiCalendarBoundary(t *testing.T) {
	start, end := enterpriseMemberCurrentBudgetPeriod(time.Date(2026, time.January, 31, 16, 30, 0, 0, time.UTC))
	require.Equal(t, "2026-02-01T00:00:00+08:00", start.Format(time.RFC3339))
	require.Equal(t, "2026-03-01T00:00:00+08:00", end.Format(time.RFC3339))
}

func TestEnterpriseMemberSystemUsageNoteIsAutomaticAndOnlyWrittenForUsage(t *testing.T) {
	require.Equal(t, "usage values updated by member creation", enterpriseMemberSystemUsageNote(true, "member creation"))
	require.Equal(t, "usage values updated by member editor", enterpriseMemberSystemUsageNote(true, "member editor"))
	require.Empty(t, enterpriseMemberSystemUsageNote(false, "member creation"))
}

type enterpriseMemberBudgetUsageSpy struct {
	EnterpriseMemberBudgetRepository
	note        string
	batchKey    string
	batchDelta  EnterpriseMemberUsageDelta
	batchTarget []EnterpriseMemberBatchTarget
}

func (s *enterpriseMemberBudgetUsageSpy) SetUsage(_ context.Context, _, _ int64, _ time.Time, _, _, _, _ float64, _ int64, _, note string) error {
	s.note = note
	return nil
}

func (s *enterpriseMemberBudgetUsageSpy) BatchAdjustUsage(_ context.Context, _ int64, _ time.Time, targets []EnterpriseMemberBatchTarget, delta EnterpriseMemberUsageDelta, _ int64, key, note string) ([]BatchEnterpriseMemberUsageUpdate, error) {
	s.note = note
	s.batchKey = key
	s.batchDelta = delta
	s.batchTarget = append([]EnterpriseMemberBatchTarget(nil), targets...)
	return []BatchEnterpriseMemberUsageUpdate{{ID: targets[0].ID, MonthlyUsedUSD: 12}}, nil
}

func TestEnterpriseMemberSetUsageSuppliesSystemAuditNoteWhenClientOmitsIt(t *testing.T) {
	repo := &enterpriseMemberBudgetUsageSpy{}
	service := NewEnterpriseMemberBudgetService(repo, nil, nil)

	err := service.SetUsage(context.Background(), 7, 11, EnterpriseMemberUsageAdjustmentInput{
		MonthlyUsedUSD: 30,
		Usage5h:        5,
		Usage1d:        10,
		Usage7d:        20,
	}, "member-editor-test")

	require.NoError(t, err)
	require.Equal(t, "usage values updated by member editor", repo.note)
}

func TestEnterpriseMemberBatchAdjustUsageUsesSignedDeltaAndStableLedgerScope(t *testing.T) {
	repo := &enterpriseMemberBudgetUsageSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)

	updated, err := budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		Members: []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{
			MonthlyUsedUSD: 4.5,
			Usage5h:        -1,
		},
	}, "batch-usage-request")

	require.NoError(t, err)
	require.Equal(t, []BatchEnterpriseMemberUsageUpdate{{ID: 11, MonthlyUsedUSD: 12}}, updated)
	require.Equal(t, []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}}, repo.batchTarget)
	require.Equal(t, EnterpriseMemberUsageDelta{MonthlyUsedUSD: 4.5, Usage5h: -1}, repo.batchDelta)
	require.Contains(t, repo.batchKey, "usage-batch:7:")
	require.Equal(t, "usage values updated by batch member editor", repo.note)
}

func TestEnterpriseMemberBatchAdjustUsageRejectsEmptyAndDuplicateTargets(t *testing.T) {
	budgetService := NewEnterpriseMemberBudgetService(&enterpriseMemberBudgetUsageSpy{}, nil, nil)

	_, err := budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
	}, "batch-empty")
	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)

	_, err = budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		Members:                    []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}, {ID: 11, ExpectedVersion: 3}},
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
	}, "batch-duplicate")
	require.ErrorIs(t, err, ErrEnterpriseMemberInvalid)
}

func TestEnterpriseMemberBatchAdjustUsageRequiresIdempotencyKey(t *testing.T) {
	repo := &enterpriseMemberBudgetUsageSpy{}
	budgetService := NewEnterpriseMemberBudgetService(repo, nil, nil)

	_, err := budgetService.BatchAdjustUsage(context.Background(), 7, BatchAdjustEnterpriseMemberUsageInput{
		Members:                    []EnterpriseMemberBatchTarget{{ID: 11, ExpectedVersion: 3}},
		EnterpriseMemberUsageDelta: EnterpriseMemberUsageDelta{MonthlyUsedUSD: 1},
	}, "  ")

	require.ErrorIs(t, err, ErrIdempotencyKeyRequired)
	require.Empty(t, repo.batchTarget)
}

func TestEnterpriseMemberUsageValidationMatchesDatabaseNumericRange(t *testing.T) {
	require.NoError(t, validateEnterpriseUsageValues(EnterpriseMemberMaxMonetaryValue))
	require.ErrorIs(t, validateEnterpriseUsageValues(EnterpriseMemberMaxMonetaryValue+1), ErrEnterpriseMemberInvalid)
	require.NoError(t, validateEnterpriseUsageDeltas(-EnterpriseMemberMaxMonetaryValue))
	require.ErrorIs(t, validateEnterpriseUsageDeltas(EnterpriseMemberMaxMonetaryValue+1), ErrEnterpriseMemberInvalid)
}
