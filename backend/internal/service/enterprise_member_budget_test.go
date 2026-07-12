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
	note string
}

func (s *enterpriseMemberBudgetUsageSpy) SetUsage(_ context.Context, _, _ int64, _ time.Time, _, _, _, _ float64, _ int64, _, note string) error {
	s.note = note
	return nil
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
