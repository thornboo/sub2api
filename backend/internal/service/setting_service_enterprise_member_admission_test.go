package service

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type enterpriseMemberAdmissionSettingRepoStub struct {
	value        string
	rolloutValue string
	getErr       error
	all          map[string]string
	updates      map[string]string
}

func (s *enterpriseMemberAdmissionSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *enterpriseMemberAdmissionSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	switch key {
	case SettingKeyEnterpriseMemberModelAdmissionMode:
		return s.value, s.getErr
	case SettingKeyEnterpriseMemberModelAdmissionRolloutPolicy:
		if s.rolloutValue == "" {
			return "", ErrSettingNotFound
		}
		return s.rolloutValue, s.getErr
	default:
		return "", ErrSettingNotFound
	}
}

func (s *enterpriseMemberAdmissionSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *enterpriseMemberAdmissionSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *enterpriseMemberAdmissionSettingRepoStub) SetMultiple(_ context.Context, updates map[string]string) error {
	s.updates = make(map[string]string, len(updates))
	for key, value := range updates {
		s.updates[key] = value
	}
	return nil
}

func (s *enterpriseMemberAdmissionSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	if s.all == nil {
		return map[string]string{}, nil
	}
	out := make(map[string]string, len(s.all))
	for key, value := range s.all {
		out[key] = value
	}
	return out, nil
}

func (s *enterpriseMemberAdmissionSettingRepoStub) Delete(context.Context, string) error {
	return nil
}

type enterpriseMemberAdmissionReadinessProviderStub struct {
	readiness EnterpriseMemberModelAdmissionEnforceReadiness
}

func (s enterpriseMemberAdmissionReadinessProviderStub) EvaluateEnterpriseMemberModelAdmissionReadiness(context.Context) EnterpriseMemberModelAdmissionEnforceReadiness {
	return s.readiness
}

type countingEnterpriseMemberAdmissionReadinessProvider struct {
	readiness EnterpriseMemberModelAdmissionEnforceReadiness
	calls     atomic.Int64
}

func (p *countingEnterpriseMemberAdmissionReadinessProvider) EvaluateEnterpriseMemberModelAdmissionReadiness(context.Context) EnterpriseMemberModelAdmissionEnforceReadiness {
	p.calls.Add(1)
	return p.readiness
}

func readyEnterpriseMemberAdmissionReadiness() EnterpriseMemberModelAdmissionEnforceReadiness {
	return EnterpriseMemberModelAdmissionEnforceReadiness{
		Ready:  true,
		Source: "test",
		Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{
			{Name: "routing_revision_healthy", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "evaluator_coverage_verified", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "alias_audit_clear", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "evidence_pipeline_healthy", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "expansion_evidence_verified", Ready: true, Layer: "expansion", Source: "test"},
			{Name: "stop_clear", Ready: true, Layer: "stop", Source: "test"},
		},
	}
}

func blockedEnterpriseMemberAdmissionReadiness(condition, reason string) EnterpriseMemberModelAdmissionEnforceReadiness {
	return EnterpriseMemberModelAdmissionEnforceReadiness{
		Ready:  true,
		Source: "test",
		Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{
			{Name: "routing_revision_healthy", Ready: condition != "routing_revision_healthy", Layer: "foundation", Reason: reason, Source: "test"},
			{Name: "evaluator_coverage_verified", Ready: condition != "evaluator_coverage_verified", Layer: "foundation", Reason: reason, Source: "test"},
			{Name: "alias_audit_clear", Ready: condition != "alias_audit_clear", Layer: "foundation", Reason: reason, Source: "test"},
			{Name: "evidence_pipeline_healthy", Ready: condition != "evidence_pipeline_healthy", Layer: "foundation", Reason: reason, Source: "test"},
			{Name: "expansion_evidence_verified", Ready: true, Layer: "expansion", Source: "test"},
			{Name: "stop_clear", Ready: true, Layer: "stop", Source: "test"},
		},
	}
}

func expansionBlockedEnterpriseMemberAdmissionReadiness(reason string) EnterpriseMemberModelAdmissionEnforceReadiness {
	return EnterpriseMemberModelAdmissionEnforceReadiness{
		Ready:  true,
		Source: "test",
		Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{
			{Name: "routing_revision_healthy", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "evaluator_coverage_verified", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "alias_audit_clear", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "evidence_pipeline_healthy", Ready: true, Layer: "foundation", Source: "test"},
			{Name: "expansion_evidence_verified", Ready: false, Layer: "expansion", Reason: reason, Source: "test"},
			{Name: "stop_clear", Ready: true, Layer: "stop", Source: "test"},
		},
	}
}

func TestEnterpriseMemberModelAdmissionRuntimeFallsBackToConfigWhenSettingMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(EnterpriseMemberModelAdmissionLegacyOrderOnly)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{getErr: ErrSettingNotFound}, cfg)

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, "config", runtime.Source)
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, svc.GetEnterpriseMemberModelAdmissionMode(context.Background()))
}

func TestEnterpriseMemberModelAdmissionRuntimeAllowsEnforceWhenReadinessIsComplete(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: readyEnterpriseMemberAdmissionReadiness()})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, runtime.Mode)
	require.Equal(t, "settings", runtime.Source)
	require.True(t, runtime.Readiness.Ready)
	require.Equal(t, []int64{42}, runtime.Rollout.Policy.MemberIDs)
}

func TestEnterpriseMemberModelAdmissionRuntimeCacheMissEvaluatesReadinessOnce(t *testing.T) {
	provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
	defer restore()

	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, int64(1), provider.calls.Load())
	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, runtime.Mode)
	require.Equal(t, "settings", runtime.Source)
}

func TestEnterpriseMemberModelAdmissionRuntimeCacheHitDoesNotEvaluateReadiness(t *testing.T) {
	provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
	defer restore()

	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})
	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background()).Mode)
	provider.calls.Store(0)

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, int64(0), provider.calls.Load())
	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, runtime.Mode)
	require.Equal(t, "settings", runtime.Source)
}

func TestEnterpriseMemberModelAdmissionRuntimeConcurrentCacheMissesEvaluateReadinessOnce(t *testing.T) {
	provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
	defer restore()

	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	const goroutines = 32
	start := make(chan struct{})
	modes := make(chan EnterpriseMemberModelAdmissionMode, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			<-start
			modes <- svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background()).Mode
		}()
	}
	close(start)
	wg.Wait()
	close(modes)

	for mode := range modes {
		require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, mode)
	}
	require.Equal(t, int64(1), provider.calls.Load())
}

func TestEnterpriseMemberModelAdmissionRuntimeConcurrentCacheHitsDoNotEvaluateReadiness(t *testing.T) {
	provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
	defer restore()

	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})
	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background()).Mode)
	provider.calls.Store(0)

	const goroutines = 32
	start := make(chan struct{})
	modes := make(chan EnterpriseMemberModelAdmissionMode, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			<-start
			modes <- svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background()).Mode
		}()
	}
	close(start)
	wg.Wait()
	close(modes)

	for mode := range modes {
		require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, mode)
	}
	require.Equal(t, int64(0), provider.calls.Load())
}

func TestEnterpriseMemberModelAdmissionResolveUsesExplicitAllowlists(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: readyEnterpriseMemberAdmissionReadiness()})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{
		EnterpriseUserIDs: []int64{7},
		MemberIDs:         []int64{42},
	})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	enterpriseMatch := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{EnterpriseUserID: 7, MemberID: 1, APIKeyID: 1})
	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, enterpriseMatch.Mode)
	require.Equal(t, "enterprise_user_allowlist", enterpriseMatch.Rollout.MatchedBy)

	memberMatch := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{EnterpriseUserID: 8, MemberID: 42, APIKeyID: 1})
	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, memberMatch.Mode)
	require.Equal(t, "member_allowlist", memberMatch.Rollout.MatchedBy)

	miss := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{EnterpriseUserID: 8, MemberID: 43, APIKeyID: 1})
	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, miss.Mode)
	require.Equal(t, "rollout_shadow", miss.Source)
	require.Equal(t, EnterpriseMemberModelAdmissionRolloutMissReason, miss.Rollout.Reason)
}

func TestEnterpriseMemberModelAdmissionCanaryAllowlistDoesNotRequireExpansionEvidence(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: expansionBlockedEnterpriseMemberAdmissionReadiness(EnterpriseMemberAdmissionEvidenceReasonCanaryEvidenceMissing)})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42, APIKeyID: 1})

	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, runtime.Mode)
	require.Equal(t, "member_allowlist", runtime.Rollout.MatchedBy)
	require.True(t, runtime.Readiness.CanaryReady)
	require.False(t, runtime.Readiness.ExpansionReady)
}

func TestEnterpriseMemberModelAdmissionPercentageRequiresExpansionEvidence(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: expansionBlockedEnterpriseMemberAdmissionReadiness(EnterpriseMemberAdmissionEvidenceReasonCanaryEvidenceMissing)})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 100})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42, APIKeyID: 1})

	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
	require.Equal(t, "expansion_blocked", runtime.Source)
	require.Equal(t, "stable_hash", runtime.Rollout.MatchedBy)
}

func TestEnterpriseMemberModelAdmissionPercentageAllowsWhenExpansionReady(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: readyEnterpriseMemberAdmissionReadiness()})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 100})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42, APIKeyID: 1})

	require.Equal(t, EnterpriseMemberModelAdmissionEnforcePublished, runtime.Mode)
	require.Equal(t, "stable_hash", runtime.Rollout.MatchedBy)
	require.True(t, runtime.Readiness.ExpansionReady)
}

func TestEnterpriseMemberModelAdmissionManualStopTakesPriorityOverMetricStop(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{
		readiness: EnterpriseMemberModelAdmissionEnforceReadiness{
			Ready:    false,
			Source:   "test",
			AutoStop: EnterpriseMemberModelAdmissionAutoStopState{Stopped: true, Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics, Reason: EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay, Reasons: []string{EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay}},
			Conditions: []EnterpriseMemberModelAdmissionReadinessCondition{
				{Name: "routing_revision_healthy", Ready: true, Layer: "foundation", Source: "test"},
				{Name: "evaluator_coverage_verified", Ready: true, Layer: "foundation", Source: "test"},
				{Name: "alias_audit_clear", Ready: true, Layer: "foundation", Source: "test"},
				{Name: "evidence_pipeline_healthy", Ready: true, Layer: "foundation", Source: "test"},
				{Name: "expansion_evidence_verified", Ready: true, Layer: "expansion", Source: "test"},
				{Name: "stop_clear", Ready: false, Layer: "stop", Source: EnterpriseMemberModelAdmissionAutoStopSourceMetrics, Reason: EnterpriseMemberModelAdmissionAutoStopReasonUnsafeReplay},
			},
		},
	})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}, AutoStop: true})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42})

	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
	require.Equal(t, "auto_stop", runtime.Source)
	require.Equal(t, EnterpriseMemberModelAdmissionAutoStopSourceManual, runtime.Rollout.AutoStop.Source)
	require.Equal(t, EnterpriseMemberModelAdmissionAutoStopReasonManual, runtime.Rollout.Reason)
}

func TestEnterpriseMemberModelAdmissionResolverUsesCachedGateSnapshot(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: readyEnterpriseMemberAdmissionReadiness()})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())
	resolved := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42})

	require.Equal(t, runtime.Snapshot.Version, resolved.Snapshot.Version)
	require.Equal(t, runtime.Snapshot.GeneratedAt, resolved.Snapshot.GeneratedAt)
	require.Equal(t, runtime.Snapshot.PolicyHash, resolved.Snapshot.PolicyHash)
	require.Equal(t, resolved.Snapshot, resolved.Readiness.Snapshot)
}

func TestEnterpriseMemberModelAdmissionRolloutStableHashIsDeterministic(t *testing.T) {
	policy := EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: 50, Salt: "deterministic"}
	input := EnterpriseMemberModelAdmissionRolloutInput{EnterpriseUserID: 7, MemberID: 42, APIKeyID: 99}

	first := EvaluateEnterpriseMemberModelAdmissionRollout(policy, input)
	second := EvaluateEnterpriseMemberModelAdmissionRollout(policy, input)

	require.True(t, first.Valid)
	require.Equal(t, first.HashBucket, second.HashBucket)
	require.Equal(t, first.Matched, second.Matched)
	require.Equal(t, first.HashBucket < 50, first.Matched)
}

func TestEnterpriseMemberModelAdmissionReadinessBlocksUnhealthyDependencies(t *testing.T) {
	for _, tc := range []struct {
		name      string
		condition string
		reason    string
	}{
		{name: "revision", condition: "routing_revision_healthy", reason: "routing_revision_unhealthy"},
		{name: "evaluator", condition: "evaluator_coverage_verified", reason: "evaluator_unverified"},
		{name: "alias", condition: "alias_audit_clear", reason: "alias_audit_pending"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: blockedEnterpriseMemberAdmissionReadiness(tc.condition, tc.reason)})
			defer restore()
			policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
			require.NoError(t, err)
			svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
				value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
				rolloutValue: policy,
			}, &config.Config{})

			runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

			require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
			require.Equal(t, "enforce_blocked", runtime.Source)
			require.False(t, runtime.Readiness.Ready)
			require.Equal(t, tc.reason, runtime.Readiness.Reason)
		})
	}
}

func TestEnterpriseMemberModelAdmissionAutoStopForcesShadow(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: readyEnterpriseMemberAdmissionReadiness()})
	defer restore()
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{
		MemberIDs: []int64{42},
		AutoStop:  true,
	})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, &config.Config{})

	runtime := svc.ResolveEnterpriseMemberModelAdmissionMode(context.Background(), EnterpriseMemberModelAdmissionRolloutInput{MemberID: 42})

	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
	require.Equal(t, "auto_stop", runtime.Source)
	require.True(t, runtime.Rollout.AutoStopped)
}

func TestEnterpriseMemberModelAdmissionInvalidRolloutConfigFallsBackToShadow(t *testing.T) {
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(enterpriseMemberAdmissionReadinessProviderStub{readiness: readyEnterpriseMemberAdmissionReadiness()})
	defer restore()
	invalidPolicy, err := json.Marshal(map[string]any{"percentage": 101})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: string(invalidPolicy),
	}, &config.Config{})

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
	require.Equal(t, "rollout_invalid", runtime.Source)
	require.False(t, runtime.Rollout.Valid)
	require.Equal(t, EnterpriseMemberModelAdmissionRolloutInvalidReason, runtime.Rollout.Reason)
}

func TestEnterpriseMemberModelAdmissionRuntimeDefaultsToLegacyOrderAfterFailedShadowRelease(t *testing.T) {
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{getErr: ErrSettingNotFound}, &config.Config{})

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, "config", runtime.Source)
}

func TestEnterpriseMemberModelAdmissionRuntimeDefaultLegacySkipsReadinessEvaluation(t *testing.T) {
	provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
	defer restore()

	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{getErr: ErrSettingNotFound}, &config.Config{})
	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, int64(0), provider.calls.Load())
}

func TestRefreshCachedSettingsDefaultLegacySkipsReadinessEvaluation(t *testing.T) {
	provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
	restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
	defer restore()

	svc := NewSettingService(nil, &config.Config{})
	svc.refreshCachedSettings(&SystemSettings{
		EnterpriseMemberModelAdmissionMode: string(EnterpriseMemberModelAdmissionLegacyOrderOnly),
	})

	require.Equal(t, int64(0), provider.calls.Load())
	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, int64(0), provider.calls.Load())
}

func TestEnterpriseMemberModelAdmissionRuntimeBlocksPrematureDatabaseEnforceOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(EnterpriseMemberModelAdmissionLegacyOrderOnly)
	policy, err := MarshalEnterpriseMemberModelAdmissionRolloutPolicy(EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}})
	require.NoError(t, err)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{
		value:        string(EnterpriseMemberModelAdmissionEnforcePublished),
		rolloutValue: policy,
	}, cfg)

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionShadowPublished, runtime.Mode)
	require.Equal(t, "enforce_blocked", runtime.Source)
}

func TestEnterpriseMemberModelAdmissionRuntimeInvalidValuesNormalizeToLegacyOrder(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = "not-a-mode"
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{getErr: ErrSettingNotFound}, cfg)

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, "config_invalid", runtime.Source)

	svc = NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{value: "also-bad"}, &config.Config{})
	runtime = svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())
	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, "settings_invalid", runtime.Source)
}

func TestEnterpriseMemberModelAdmissionRuntimeDatabaseErrorFallsBackSafelyWithoutKnownValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(EnterpriseMemberModelAdmissionEnforcePublished)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{getErr: errors.New("database unavailable")}, cfg)

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, "error_fallback", runtime.Source)
}

func TestEnterpriseMemberModelAdmissionRuntimeDatabaseErrorDoesNotRestoreBlockedEnforce(t *testing.T) {
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{getErr: errors.New("database unavailable")}, &config.Config{})
	svc.enterpriseMemberAdmissionCache.Store(&cachedEnterpriseMemberModelAdmission{
		runtime: EnterpriseMemberModelAdmissionRuntime{
			Mode:   EnterpriseMemberModelAdmissionEnforcePublished,
			Source: "settings",
		},
		expiresAt: time.Now().Add(-time.Second).UnixNano(),
	})

	runtime := svc.GetEnterpriseMemberModelAdmissionRuntime(context.Background())

	require.Equal(t, EnterpriseMemberModelAdmissionLegacyOrderOnly, runtime.Mode)
	require.Equal(t, "error_fallback", runtime.Source)
}

func TestParseSettingsReportsEnterpriseMemberModelAdmissionSource(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(EnterpriseMemberModelAdmissionLegacyOrderOnly)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{}, cfg)

	fromConfig := svc.parseSettings(map[string]string{})
	require.Equal(t, string(EnterpriseMemberModelAdmissionLegacyOrderOnly), fromConfig.EnterpriseMemberModelAdmissionMode)
	require.Equal(t, "config", fromConfig.EnterpriseMemberModelAdmissionSource)

	fromSettings := svc.parseSettings(map[string]string{
		SettingKeyEnterpriseMemberModelAdmissionMode: string(EnterpriseMemberModelAdmissionEnforcePublished),
	})
	require.Equal(t, string(EnterpriseMemberModelAdmissionShadowPublished), fromSettings.EnterpriseMemberModelAdmissionMode)
	require.Equal(t, "rollout_shadow", fromSettings.EnterpriseMemberModelAdmissionSource)
	require.False(t, fromSettings.EnterpriseMemberModelAdmissionEnforceReady)
	require.Equal(t, EnterpriseMemberModelAdmissionEnforceBlockedReason, fromSettings.EnterpriseMemberModelAdmissionEnforceReason)

	fromInvalid := svc.parseSettings(map[string]string{
		SettingKeyEnterpriseMemberModelAdmissionMode: "bad-mode",
	})
	require.Equal(t, string(EnterpriseMemberModelAdmissionLegacyOrderOnly), fromInvalid.EnterpriseMemberModelAdmissionMode)
	require.Equal(t, "settings_invalid", fromInvalid.EnterpriseMemberModelAdmissionSource)
}

func TestBuildSystemSettingsUpdatesNonEnforceSkipsReadinessEvaluation(t *testing.T) {
	for _, mode := range []EnterpriseMemberModelAdmissionMode{
		EnterpriseMemberModelAdmissionLegacyOrderOnly,
		EnterpriseMemberModelAdmissionShadowPublished,
	} {
		t.Run(string(mode), func(t *testing.T) {
			provider := &countingEnterpriseMemberAdmissionReadinessProvider{readiness: readyEnterpriseMemberAdmissionReadiness()}
			restore := SetEnterpriseMemberModelAdmissionReadinessProviderForTest(provider)
			defer restore()

			svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{}, &config.Config{})
			updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
				EnterpriseMemberModelAdmissionMode:   string(mode),
				EnterpriseMemberModelAdmissionSource: "settings",
			})

			require.NoError(t, err)
			require.NotNil(t, updates)
			require.Equal(t, int64(0), provider.calls.Load())
		})
	}
}

func TestBuildSystemSettingsUpdatesRejectsPrematureEnterpriseMemberEnforceOverride(t *testing.T) {
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{}, &config.Config{})

	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		EnterpriseMemberModelAdmissionMode:   string(EnterpriseMemberModelAdmissionEnforcePublished),
		EnterpriseMemberModelAdmissionSource: "settings",
		EnterpriseMemberModelAdmissionRollout: EnterpriseMemberModelAdmissionRolloutState{Policy: EnterpriseMemberModelAdmissionRolloutPolicy{
			MemberIDs: []int64{42},
		}},
	})

	require.Error(t, err)
	require.Nil(t, updates)
	require.Contains(t, err.Error(), "ENFORCE_NOT_READY")
}

func TestUpdateSettingsRejectsPrematureEnterpriseMemberEnforceWithManualAutoStopWithoutPersistence(t *testing.T) {
	repo := &enterpriseMemberAdmissionSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PromoCodeEnabled:                            true,
		EnterpriseMemberModelAdmissionMode:          string(EnterpriseMemberModelAdmissionEnforcePublished),
		EnterpriseMemberModelAdmissionSource:        "settings",
		EnterpriseMemberModelAdmissionRollout:       EnterpriseMemberModelAdmissionRolloutState{Source: "settings", Policy: EnterpriseMemberModelAdmissionRolloutPolicy{MemberIDs: []int64{42}, AutoStop: true}},
		NativeModelProtocolRoutingSource:            "config",
		EnterpriseMemberModelAdmissionLegacy:        EnterpriseMemberModelAdmissionLegacyRetirementStatusForTarget(""),
		EnterpriseMemberModelAdmissionEnforceReason: EnterpriseMemberModelAdmissionEnforceBlockedReason,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "ENFORCE_NOT_READY")
	require.Nil(t, repo.updates)
}

func TestBuildSystemSettingsUpdatesRejectsPrematureEnterpriseMemberEnforceOverrideWithoutRolloutTarget(t *testing.T) {
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{}, &config.Config{})

	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		EnterpriseMemberModelAdmissionMode:   string(EnterpriseMemberModelAdmissionEnforcePublished),
		EnterpriseMemberModelAdmissionSource: "settings",
	})

	require.Error(t, err)
	require.Nil(t, updates)
	require.Contains(t, err.Error(), "ENFORCE_NOT_READY")
}

func TestUpdateSettingsRejectsOversizedEnterpriseMemberAdmissionRolloutPolicyWithoutPersistence(t *testing.T) {
	repo := &enterpriseMemberAdmissionSettingRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		PromoCodeEnabled:                     true,
		EnterpriseMemberModelAdmissionMode:   string(EnterpriseMemberModelAdmissionShadowPublished),
		EnterpriseMemberModelAdmissionSource: "settings",
		EnterpriseMemberModelAdmissionRollout: EnterpriseMemberModelAdmissionRolloutState{
			Source: "settings",
			Policy: EnterpriseMemberModelAdmissionRolloutPolicy{
				EnterpriseUserIDs: enterpriseMemberAdmissionSequentialIDs(1, enterpriseMemberModelAdmissionMaxRolloutIDsPerList+1),
			},
		},
		NativeModelProtocolRoutingSource: "config",
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "INVALID_ENTERPRISE_MEMBER_ADMISSION_ROLLOUT")
	require.Nil(t, repo.updates)
}

func TestBuildSystemSettingsUpdatesDoesNotSolidifyEnterpriseMemberModelAdmissionFallbacks(t *testing.T) {
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{}, &config.Config{})

	for _, source := range []string{"config", "config_invalid", "enforce_blocked"} {
		t.Run(source, func(t *testing.T) {
			mode := EnterpriseMemberModelAdmissionEnforcePublished
			if source == "enforce_blocked" {
				mode = EnterpriseMemberModelAdmissionShadowPublished
			}
			updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
				EnterpriseMemberModelAdmissionMode:   string(mode),
				EnterpriseMemberModelAdmissionSource: source,
			})

			require.NoError(t, err)
			_, exists := updates[SettingKeyEnterpriseMemberModelAdmissionMode]
			require.False(t, exists)
		})
	}
}

func TestBuildSystemSettingsUpdatesDoesNotPersistBlockedConfigEnforceOnUnrelatedSave(t *testing.T) {
	cfg := &config.Config{Default: config.DefaultConfig{UserConcurrency: 5}}
	cfg.Gateway.EnterpriseMemberModelAdmissionMode = string(EnterpriseMemberModelAdmissionEnforcePublished)
	svc := NewSettingService(&enterpriseMemberAdmissionSettingRepoStub{}, cfg)

	settings := svc.parseSettings(map[string]string{})
	require.Equal(t, string(EnterpriseMemberModelAdmissionShadowPublished), settings.EnterpriseMemberModelAdmissionMode)
	require.Equal(t, "rollout_shadow", settings.EnterpriseMemberModelAdmissionSource)

	settings.PromoCodeEnabled = true
	updates, err := svc.buildSystemSettingsUpdates(context.Background(), settings)

	require.NoError(t, err)
	_, persistsMode := updates[SettingKeyEnterpriseMemberModelAdmissionMode]
	require.False(t, persistsMode)
	require.Equal(t, "true", updates[SettingKeyPromoCodeEnabled])
}
