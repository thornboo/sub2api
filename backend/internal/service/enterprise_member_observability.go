package service

import (
	"sync/atomic"
	"time"
)

// EnterpriseMemberMetricsSnapshot contains bounded, process-local counters.
// It deliberately avoids owner/member/key labels so metrics cannot leak tenant
// identity or create unbounded cardinality.
type EnterpriseMemberMetricsSnapshot struct {
	AuthSuccessTotal                  uint64            `json:"auth_success_total"`
	AuthRejectedTotal                 uint64            `json:"auth_rejected_total"`
	AuthRejectedByReason              map[string]uint64 `json:"auth_rejected_by_reason"`
	AuthCacheVersionMissTotal         uint64            `json:"auth_cache_version_miss_total"`
	RoutingPlanTotal                  uint64            `json:"routing_plan_total"`
	RoutingCandidateTotal             uint64            `json:"routing_candidate_total"`
	RoutingActivationTotal            uint64            `json:"routing_activation_total"`
	RoutingCrossGroupAttemptTotal     uint64            `json:"routing_cross_group_attempt_total"`
	RoutingNoCandidateTotal           uint64            `json:"routing_no_candidate_total"`
	BudgetReservationCreatedTotal     uint64            `json:"budget_reservation_created_total"`
	BudgetReservationDeniedTotal      uint64            `json:"budget_reservation_denied_total"`
	BudgetReservationErrorTotal       uint64            `json:"budget_reservation_error_total"`
	BudgetSettlementTotal             uint64            `json:"budget_settlement_total"`
	BudgetSettlementOverrunTotal      uint64            `json:"budget_settlement_overrun_total"`
	BudgetAmbiguousTotal              uint64            `json:"budget_ambiguous_total"`
	BudgetReleaseTotal                uint64            `json:"budget_release_total"`
	BudgetReleaseErrorTotal           uint64            `json:"budget_release_error_total"`
	BudgetExpiredRecoveredTotal       uint64            `json:"budget_expired_recovered_total"`
	BudgetRecoveryErrorTotal          uint64            `json:"budget_recovery_error_total"`
	BudgetReconcileRunTotal           uint64            `json:"budget_reconcile_run_total"`
	BudgetReconcileErrorTotal         uint64            `json:"budget_reconcile_error_total"`
	BudgetReconcilePeriodsChecked     uint64            `json:"budget_reconcile_periods_checked"`
	BudgetReconcileProjectionsRebuilt uint64            `json:"budget_reconcile_projections_rebuilt"`
	BudgetReconcileEntriesCreated     uint64            `json:"budget_reconcile_entries_created"`
	BudgetReconcileLinksRepaired      uint64            `json:"budget_reconcile_links_repaired"`
	ImportPreviewTotal                uint64            `json:"import_preview_total"`
	ImportPreviewErrorTotal           uint64            `json:"import_preview_error_total"`
	ImportPreviewRowsTotal            uint64            `json:"import_preview_rows_total"`
	ImportPreviewInvalidRowsTotal     uint64            `json:"import_preview_invalid_rows_total"`
	ImportParseDurationCount          uint64            `json:"import_parse_duration_count"`
	ImportParseDurationTotalMs        float64           `json:"import_parse_duration_total_ms"`
	ImportCommitTotal                 uint64            `json:"import_commit_total"`
	ImportCommitRowsTotal             uint64            `json:"import_commit_rows_total"`
	ImportRollbackTotal               uint64            `json:"import_rollback_total"`
	ImportLeaseRenewalTotal           uint64            `json:"import_lease_renewal_total"`
	ImportLeaseRenewalErrorTotal      uint64            `json:"import_lease_renewal_error_total"`
	ImportLeaseLostTotal              uint64            `json:"import_lease_lost_total"`
}

type enterpriseMemberMetrics struct {
	authSuccess               atomic.Uint64
	authRejected              atomic.Uint64
	authRejectAccount         atomic.Uint64
	authRejectMemberMissing   atomic.Uint64
	authRejectMemberDisabled  atomic.Uint64
	authRejectInvalidKeyShape atomic.Uint64
	authRejectOther           atomic.Uint64
	authCacheVersionMiss      atomic.Uint64
	routingPlans              atomic.Uint64
	routingCandidates         atomic.Uint64
	routingActivations        atomic.Uint64
	routingCrossGroupAttempts atomic.Uint64
	routingNoCandidate        atomic.Uint64
	budgetReservations        atomic.Uint64
	budgetReservationDenied   atomic.Uint64
	budgetReservationErrors   atomic.Uint64
	budgetSettlements         atomic.Uint64
	budgetSettlementOverruns  atomic.Uint64
	budgetAmbiguous           atomic.Uint64
	budgetReleases            atomic.Uint64
	budgetReleaseErrors       atomic.Uint64
	budgetExpiredRecovered    atomic.Uint64
	budgetRecoveryErrors      atomic.Uint64
	budgetReconcileRuns       atomic.Uint64
	budgetReconcileErrors     atomic.Uint64
	budgetPeriodsChecked      atomic.Uint64
	budgetProjectionsRebuilt  atomic.Uint64
	budgetEntriesCreated      atomic.Uint64
	budgetLinksRepaired       atomic.Uint64
	importPreviews            atomic.Uint64
	importPreviewErrors       atomic.Uint64
	importPreviewRows         atomic.Uint64
	importPreviewInvalidRows  atomic.Uint64
	importParseDurationCount  atomic.Uint64
	importParseDurationMicros atomic.Uint64
	importCommits             atomic.Uint64
	importCommitRows          atomic.Uint64
	importRollbacks           atomic.Uint64
	importLeaseRenewals       atomic.Uint64
	importLeaseRenewalErrors  atomic.Uint64
	importLeaseLost           atomic.Uint64
}

var defaultEnterpriseMemberMetrics enterpriseMemberMetrics

func GetEnterpriseMemberMetricsSnapshot() EnterpriseMemberMetricsSnapshot {
	m := &defaultEnterpriseMemberMetrics
	return EnterpriseMemberMetricsSnapshot{
		AuthSuccessTotal:                  m.authSuccess.Load(),
		AuthRejectedTotal:                 m.authRejected.Load(),
		AuthCacheVersionMissTotal:         m.authCacheVersionMiss.Load(),
		RoutingPlanTotal:                  m.routingPlans.Load(),
		RoutingCandidateTotal:             m.routingCandidates.Load(),
		RoutingActivationTotal:            m.routingActivations.Load(),
		RoutingCrossGroupAttemptTotal:     m.routingCrossGroupAttempts.Load(),
		RoutingNoCandidateTotal:           m.routingNoCandidate.Load(),
		BudgetReservationCreatedTotal:     m.budgetReservations.Load(),
		BudgetReservationDeniedTotal:      m.budgetReservationDenied.Load(),
		BudgetReservationErrorTotal:       m.budgetReservationErrors.Load(),
		BudgetSettlementTotal:             m.budgetSettlements.Load(),
		BudgetSettlementOverrunTotal:      m.budgetSettlementOverruns.Load(),
		BudgetAmbiguousTotal:              m.budgetAmbiguous.Load(),
		BudgetReleaseTotal:                m.budgetReleases.Load(),
		BudgetReleaseErrorTotal:           m.budgetReleaseErrors.Load(),
		BudgetExpiredRecoveredTotal:       m.budgetExpiredRecovered.Load(),
		BudgetRecoveryErrorTotal:          m.budgetRecoveryErrors.Load(),
		BudgetReconcileRunTotal:           m.budgetReconcileRuns.Load(),
		BudgetReconcileErrorTotal:         m.budgetReconcileErrors.Load(),
		BudgetReconcilePeriodsChecked:     m.budgetPeriodsChecked.Load(),
		BudgetReconcileProjectionsRebuilt: m.budgetProjectionsRebuilt.Load(),
		BudgetReconcileEntriesCreated:     m.budgetEntriesCreated.Load(),
		BudgetReconcileLinksRepaired:      m.budgetLinksRepaired.Load(),
		ImportPreviewTotal:                m.importPreviews.Load(),
		ImportPreviewErrorTotal:           m.importPreviewErrors.Load(),
		ImportPreviewRowsTotal:            m.importPreviewRows.Load(),
		ImportPreviewInvalidRowsTotal:     m.importPreviewInvalidRows.Load(),
		ImportParseDurationCount:          m.importParseDurationCount.Load(),
		ImportParseDurationTotalMs:        float64(m.importParseDurationMicros.Load()) / 1000,
		ImportCommitTotal:                 m.importCommits.Load(),
		ImportCommitRowsTotal:             m.importCommitRows.Load(),
		ImportRollbackTotal:               m.importRollbacks.Load(),
		ImportLeaseRenewalTotal:           m.importLeaseRenewals.Load(),
		ImportLeaseRenewalErrorTotal:      m.importLeaseRenewalErrors.Load(),
		ImportLeaseLostTotal:              m.importLeaseLost.Load(),
		AuthRejectedByReason: map[string]uint64{
			"enterprise_account_disabled": m.authRejectAccount.Load(),
			"member_not_found":            m.authRejectMemberMissing.Load(),
			"member_disabled":             m.authRejectMemberDisabled.Load(),
			"invalid_member_key_shape":    m.authRejectInvalidKeyShape.Load(),
			"other":                       m.authRejectOther.Load(),
		},
	}
}

func RecordEnterpriseMemberAuthResult(success bool, reason string) {
	m := &defaultEnterpriseMemberMetrics
	if success {
		m.authSuccess.Add(1)
		return
	}
	m.authRejected.Add(1)
	switch reason {
	case "ENTERPRISE_ACCOUNT_DISABLED":
		m.authRejectAccount.Add(1)
	case "ENTERPRISE_MEMBER_NOT_FOUND":
		m.authRejectMemberMissing.Add(1)
	case "ENTERPRISE_MEMBER_DISABLED":
		m.authRejectMemberDisabled.Add(1)
	case "ENTERPRISE_MEMBER_KEY_INVALID":
		m.authRejectInvalidKeyShape.Add(1)
	default:
		m.authRejectOther.Add(1)
	}
}

func RecordEnterpriseMemberAuthCacheVersionMiss() {
	defaultEnterpriseMemberMetrics.authCacheVersionMiss.Add(1)
}

func RecordEnterpriseMemberRoutingPlan(candidateCount int) {
	m := &defaultEnterpriseMemberMetrics
	m.routingPlans.Add(1)
	if candidateCount <= 0 {
		m.routingNoCandidate.Add(1)
		return
	}
	m.routingCandidates.Add(uint64(candidateCount))
}

func RecordEnterpriseMemberRoutingActivation(crossGroup bool) {
	m := &defaultEnterpriseMemberMetrics
	m.routingActivations.Add(1)
	if crossGroup {
		m.routingCrossGroupAttempts.Add(1)
	}
}

func RecordEnterpriseMemberBudgetReservation(err error) {
	m := &defaultEnterpriseMemberMetrics
	if err == nil {
		m.budgetReservations.Add(1)
	} else if IsEnterpriseMemberBudgetExceeded(err) {
		m.budgetReservationDenied.Add(1)
	} else {
		m.budgetReservationErrors.Add(1)
	}
}

func RecordEnterpriseMemberBudgetSettlement() {
	defaultEnterpriseMemberMetrics.budgetSettlements.Add(1)
}

// RecordEnterpriseMemberBudgetSettlementOverrun records a breach of the
// reservation invariant. The already-completed request is still persisted at
// actual cost so accounting facts are not lost; this metric makes estimator
// regressions visible and actionable.
func RecordEnterpriseMemberBudgetSettlementOverrun() {
	defaultEnterpriseMemberMetrics.budgetSettlementOverruns.Add(1)
}

func RecordEnterpriseMemberBudgetAmbiguous() {
	defaultEnterpriseMemberMetrics.budgetAmbiguous.Add(1)
}

func RecordEnterpriseMemberBudgetRelease(err error) {
	if err == nil {
		defaultEnterpriseMemberMetrics.budgetReleases.Add(1)
	} else {
		defaultEnterpriseMemberMetrics.budgetReleaseErrors.Add(1)
	}
}

func RecordEnterpriseMemberBudgetRecovery(recovered int, err error) {
	if recovered > 0 {
		defaultEnterpriseMemberMetrics.budgetExpiredRecovered.Add(uint64(recovered))
	}
	if err != nil {
		defaultEnterpriseMemberMetrics.budgetRecoveryErrors.Add(1)
	}
}

func RecordEnterpriseMemberBudgetReconciliation(result EnterpriseMemberBudgetReconciliationResult, err error) {
	m := &defaultEnterpriseMemberMetrics
	m.budgetReconcileRuns.Add(1)
	if err != nil {
		m.budgetReconcileErrors.Add(1)
		return
	}
	m.budgetPeriodsChecked.Add(uint64(result.PeriodsChecked))
	m.budgetProjectionsRebuilt.Add(uint64(result.ProjectionsRebuilt))
	m.budgetEntriesCreated.Add(uint64(result.MissingEntriesCreated))
	m.budgetLinksRepaired.Add(uint64(result.EvidenceLinksRepaired))
}

func RecordEnterpriseMemberImportPreview(duration time.Duration, rows, invalidRows int, err error) {
	m := &defaultEnterpriseMemberMetrics
	m.importPreviews.Add(1)
	m.importParseDurationCount.Add(1)
	if duration > 0 {
		m.importParseDurationMicros.Add(uint64(duration.Microseconds()))
	}
	if rows > 0 {
		m.importPreviewRows.Add(uint64(rows))
	}
	if invalidRows > 0 {
		m.importPreviewInvalidRows.Add(uint64(invalidRows))
	}
	if err != nil {
		m.importPreviewErrors.Add(1)
	}
}

func RecordEnterpriseMemberImportCommit(rows int, err error) {
	m := &defaultEnterpriseMemberMetrics
	if err != nil {
		m.importRollbacks.Add(1)
		return
	}
	m.importCommits.Add(1)
	if rows > 0 {
		m.importCommitRows.Add(uint64(rows))
	}
}

func RecordEnterpriseMemberImportLeaseRenewal(success, lost bool) {
	m := &defaultEnterpriseMemberMetrics
	if success {
		m.importLeaseRenewals.Add(1)
	} else if !lost {
		m.importLeaseRenewalErrors.Add(1)
	}
	if lost {
		m.importLeaseLost.Add(1)
	}
}

func resetEnterpriseMemberMetricsForTest() {
	defaultEnterpriseMemberMetrics = enterpriseMemberMetrics{}
}
