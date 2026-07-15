package service

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberMetricsSnapshotUsesBoundedReasonsAndLifecycleCounters(t *testing.T) {
	resetEnterpriseMemberMetricsForTest()
	t.Cleanup(resetEnterpriseMemberMetricsForTest)

	RecordEnterpriseMemberAuthResult(true, "")
	RecordEnterpriseMemberAuthResult(false, "ENTERPRISE_MEMBER_DISABLED")
	RecordEnterpriseMemberAuthResult(false, "arbitrary-high-cardinality-value")
	RecordEnterpriseMemberAuthCacheVersionMiss()
	RecordEnterpriseMemberRoutingPlan(3)
	RecordEnterpriseMemberRoutingActivation(false)
	RecordEnterpriseMemberRoutingActivation(true)
	RecordEnterpriseMemberBudgetReservation(nil)
	RecordEnterpriseMemberBudgetReservation(ErrEnterpriseMemberBudgetExceeded)
	RecordEnterpriseMemberBudgetSettlement()
	RecordEnterpriseMemberBudgetSettlementOverrun()
	RecordEnterpriseMemberBudgetAmbiguous()
	RecordEnterpriseMemberBudgetRelease(nil)
	RecordEnterpriseMemberBudgetRecovery(2, nil)
	RecordEnterpriseMemberBudgetReconciliation(EnterpriseMemberBudgetReconciliationResult{
		PeriodsChecked: 4, ProjectionsRebuilt: 1, MissingEntriesCreated: 2, EvidenceLinksRepaired: 3,
	}, nil)
	RecordEnterpriseMemberImportPreview(1500*time.Microsecond, 10, 2, nil)
	RecordEnterpriseMemberImportCommit(8, nil)
	RecordEnterpriseMemberImportCommit(0, errors.New("rollback"))
	RecordEnterpriseMemberImportLeaseRenewal(true, false)
	RecordEnterpriseMemberImportLeaseRenewal(false, false)
	RecordEnterpriseMemberImportLeaseRenewal(false, true)

	snapshot := GetEnterpriseMemberMetricsSnapshot()
	require.Equal(t, uint64(1), snapshot.AuthSuccessTotal)
	require.Equal(t, uint64(2), snapshot.AuthRejectedTotal)
	require.Equal(t, uint64(1), snapshot.AuthRejectedByReason["member_disabled"])
	require.Equal(t, uint64(1), snapshot.AuthRejectedByReason["other"])
	require.Len(t, snapshot.AuthRejectedByReason, 5)
	require.Equal(t, uint64(3), snapshot.RoutingCandidateTotal)
	require.Equal(t, uint64(1), snapshot.RoutingCrossGroupAttemptTotal)
	require.Equal(t, uint64(1), snapshot.BudgetReservationCreatedTotal)
	require.Equal(t, uint64(1), snapshot.BudgetReservationDeniedTotal)
	require.Equal(t, uint64(1), snapshot.BudgetSettlementTotal)
	require.Equal(t, uint64(1), snapshot.BudgetSettlementOverrunTotal)
	require.Equal(t, uint64(1), snapshot.BudgetAmbiguousTotal)
	require.Equal(t, uint64(2), snapshot.BudgetExpiredRecoveredTotal)
	require.Equal(t, uint64(4), snapshot.BudgetReconcilePeriodsChecked)
	require.Equal(t, uint64(2), snapshot.ImportPreviewInvalidRowsTotal)
	require.Equal(t, 1.5, snapshot.ImportParseDurationTotalMs)
	require.Equal(t, uint64(8), snapshot.ImportCommitRowsTotal)
	require.Equal(t, uint64(1), snapshot.ImportRollbackTotal)
	require.Equal(t, uint64(1), snapshot.ImportLeaseRenewalTotal)
	require.Equal(t, uint64(1), snapshot.ImportLeaseRenewalErrorTotal)
	require.Equal(t, uint64(1), snapshot.ImportLeaseLostTotal)
}
