package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEnterpriseMemberAdmissionPhase5DefaultGateStaysShadowWithoutProductionEvidence(t *testing.T) {
	require.Equal(t, EnterpriseMemberModelAdmissionPhase5GatePendingReason, EnterpriseMemberModelAdmissionDefaultCutoverGate)
	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, DefaultEnterpriseMemberModelAdmissionModeForNewInstall())
}

func TestEnterpriseMemberAdmissionLegacyRetirementTargetValidation(t *testing.T) {
	target, kind, err := ValidateEnterpriseMemberModelAdmissionLegacyRetirementTarget("2026-09-30")
	require.NoError(t, err)
	require.Equal(t, "2026-09-30", target)
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyRetirementKindDate, kind)

	target, kind, err = ValidateEnterpriseMemberModelAdmissionLegacyRetirementTarget("v1.8.0")
	require.NoError(t, err)
	require.Equal(t, "v1.8.0", target)
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyRetirementKindVersion, kind)

	_, _, err = ValidateEnterpriseMemberModelAdmissionLegacyRetirementTarget("next release")
	require.Error(t, err)
}

func TestEnterpriseMemberAdmissionLegacyRuntimeIsCountedAndReported(t *testing.T) {
	resetEnterpriseMemberMetricsForTest()
	t.Cleanup(resetEnterpriseMemberMetricsForTest)

	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(EnterpriseMemberModelAdmissionLegacyOrderOnly)
	svc := NewSettingService(nil, cfg)

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)

	status := EnterpriseMemberModelAdmissionLegacyRetirementStatusForTarget("")
	require.True(t, status.Deprecated)
	require.True(t, status.EmergencyRollbackOnly)
	require.Equal(t, uint64(1), status.UsageTotal)
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyRetirementStatusNotScheduled, status.RetirementStatus)
	require.False(t, status.Phase5Ready)
	require.Equal(t, EnterpriseMemberModelAdmissionPhase5GatePendingReason, status.Phase5Reason)
}
