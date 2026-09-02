package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"golang.org/x/sync/singleflight"
)

// cachedVersionBounds 缓存 Claude Code 版本号上下限（进程内缓存，60s TTL）
type cachedVersionBounds struct {
	min       string // 空字符串 = 不检查
	max       string // 空字符串 = 不检查
	expiresAt int64  // unix nano
}

// versionBoundsCache 版本号上下限进程内缓存
var versionBoundsCache atomic.Value // *cachedVersionBounds

// versionBoundsSF 防止缓存过期时 thundering herd
var versionBoundsSF singleflight.Group

// versionBoundsCacheTTL 缓存有效期
const versionBoundsCacheTTL = 60 * time.Second

// versionBoundsErrorTTL DB 错误时的短缓存，快速重试
const versionBoundsErrorTTL = 5 * time.Second

// versionBoundsDBTimeout singleflight 内 DB 查询超时，独立于请求 context
const versionBoundsDBTimeout = 5 * time.Second

// cachedBackendMode Backend Mode cache (in-process, 60s TTL)
type cachedBackendMode struct {
	value     bool
	expiresAt int64 // unix nano
}

// NativeModelProtocolRoutingRuntime is the effective global gate consumed by
// the catalog projection and request-routing paths. Source is "settings" when
// explicitly overridden in the database, otherwise "config".
type NativeModelProtocolRoutingRuntime struct {
	Enabled bool
	Source  string
}

type cachedNativeModelProtocolRouting struct {
	enabled   bool
	source    string
	expiresAt int64
}

type EnterpriseMemberModelAdmissionMode string

const (
	EnterpriseMemberModelAdmissionLegacyOrderOnly  EnterpriseMemberModelAdmissionMode = "legacy_order_only"
	EnterpriseMemberModelAdmissionShadowPublished  EnterpriseMemberModelAdmissionMode = "shadow_published"
	EnterpriseMemberModelAdmissionEnforcePublished EnterpriseMemberModelAdmissionMode = "enforce_published"
)

const (
	EnterpriseMemberModelAdmissionLegacyRetirementStatusNotScheduled = "not_scheduled"
	EnterpriseMemberModelAdmissionLegacyRetirementStatusScheduled    = "scheduled"
	EnterpriseMemberModelAdmissionLegacyRetirementStatusInvalid      = "invalid"
	EnterpriseMemberModelAdmissionLegacyRetirementKindDate           = "date"
	EnterpriseMemberModelAdmissionLegacyRetirementKindVersion        = "version"
	EnterpriseMemberModelAdmissionPhase5GatePendingReason            = "phase5_production_gate_pending"
	EnterpriseMemberModelAdmissionDefaultCutoverGate                 = EnterpriseMemberModelAdmissionPhase5GatePendingReason
	enterpriseMemberModelAdmissionDefaultCutoverReadyGate            = "phase5_production_evidence_verified_v1"
	enterpriseMemberLegacyAdmissionWarningInterval                   = time.Minute
)

type EnterpriseMemberModelAdmissionLegacyRetirementStatus struct {
	Deprecated              bool   `json:"deprecated"`
	EmergencyRollbackOnly   bool   `json:"emergency_rollback_only"`
	Warning                 bool   `json:"warning"`
	UsageTotal              uint64 `json:"usage_total"`
	RetirementTarget        string `json:"retirement_target"`
	RetirementTargetKind    string `json:"retirement_target_kind"`
	RetirementStatus        string `json:"retirement_status"`
	RetirementReason        string `json:"retirement_reason"`
	Phase5Ready             bool   `json:"phase5_ready"`
	Phase5Reason            string `json:"phase5_reason"`
	RiskReductionOnlyNotice string `json:"risk_reduction_only_notice"`
}

func DefaultEnterpriseMemberModelAdmissionModeForNewInstall() EnterpriseMemberModelAdmissionMode {
	if EnterpriseMemberModelAdmissionDefaultCutoverGate == enterpriseMemberModelAdmissionDefaultCutoverReadyGate {
		return EnterpriseMemberModelAdmissionEnforcePublished
	}
	return EnterpriseMemberModelAdmissionLegacyOrderOnly
}

func ValidateEnterpriseMemberModelAdmissionLegacyRetirementTarget(raw string) (string, string, error) {
	target := strings.TrimSpace(raw)
	if target == "" {
		return "", "", nil
	}
	if _, err := time.Parse("2006-01-02", target); err == nil {
		return target, EnterpriseMemberModelAdmissionLegacyRetirementKindDate, nil
	}
	version := strings.TrimPrefix(target, "v")
	parts := strings.Split(version, ".")
	if len(parts) == 3 {
		valid := true
		for _, part := range parts {
			if part == "" {
				valid = false
				break
			}
			for _, ch := range part {
				if ch < '0' || ch > '9' {
					valid = false
					break
				}
			}
		}
		if valid {
			return target, EnterpriseMemberModelAdmissionLegacyRetirementKindVersion, nil
		}
	}
	return "", "", fmt.Errorf("retirement target must be an ISO date YYYY-MM-DD or semantic version vX.Y.Z")
}

func EnterpriseMemberModelAdmissionLegacyRetirementStatusForTarget(raw string) EnterpriseMemberModelAdmissionLegacyRetirementStatus {
	target, kind, err := ValidateEnterpriseMemberModelAdmissionLegacyRetirementTarget(raw)
	status := EnterpriseMemberModelAdmissionLegacyRetirementStatus{
		Deprecated:              true,
		EmergencyRollbackOnly:   true,
		UsageTotal:              GetEnterpriseMemberMetricsSnapshot().RoutePlanningLegacyEffectiveTotal,
		RetirementTarget:        target,
		RetirementTargetKind:    kind,
		Phase5Ready:             false,
		Phase5Reason:            EnterpriseMemberModelAdmissionPhase5GatePendingReason,
		RiskReductionOnlyNotice: "legacy_order_only is deprecated and must only shrink production risk as an emergency rollback; it must not be used to expand routing behavior.",
	}
	switch {
	case err != nil:
		status.RetirementTarget = strings.TrimSpace(raw)
		status.RetirementStatus = EnterpriseMemberModelAdmissionLegacyRetirementStatusInvalid
		status.RetirementReason = "invalid_retirement_target"
	case target == "":
		status.RetirementStatus = EnterpriseMemberModelAdmissionLegacyRetirementStatusNotScheduled
		status.RetirementReason = "retirement_target_missing"
	default:
		status.RetirementStatus = EnterpriseMemberModelAdmissionLegacyRetirementStatusScheduled
	}
	return status
}

// EnterpriseMemberModelAdmissionSettingReader keeps the enterprise member
// router independent from the concrete settings service.
type EnterpriseMemberModelAdmissionSettingReader interface {
	GetEnterpriseMemberModelAdmissionMode(ctx context.Context) EnterpriseMemberModelAdmissionMode
}

type EnterpriseMemberModelAdmissionRequestResolver interface {
	ResolveEnterpriseMemberModelAdmissionMode(ctx context.Context, input EnterpriseMemberModelAdmissionRolloutInput) EnterpriseMemberModelAdmissionRuntime
}

// EnterpriseMemberModelAdmissionRuntime is the effective mode plus source used
// by admin surfaces and diagnostics.
type EnterpriseMemberModelAdmissionRuntime struct {
	Mode      EnterpriseMemberModelAdmissionMode
	Source    string
	Readiness EnterpriseMemberModelAdmissionEnforceReadiness
	Rollout   EnterpriseMemberModelAdmissionRolloutState
	Snapshot  EnterpriseMemberModelAdmissionGateSnapshot
}

func enforceSafeEnterpriseMemberModelAdmission(ctx context.Context, mode EnterpriseMemberModelAdmissionMode, source string, rollout EnterpriseMemberModelAdmissionRolloutPolicy, generatedAt, expiresAt time.Time) EnterpriseMemberModelAdmissionRuntime {
	readiness := EvaluateEnterpriseMemberModelAdmissionEnforceReadiness(ctx)
	rolloutState := EvaluateEnterpriseMemberModelAdmissionRollout(rollout, EnterpriseMemberModelAdmissionRolloutInput{})
	snapshot := BuildEnterpriseMemberModelAdmissionGateSnapshot(readiness, rollout, source, generatedAt, expiresAt)
	readiness.Snapshot = snapshot
	runtime := EnterpriseMemberModelAdmissionRuntime{Mode: mode, Source: source, Readiness: readiness, Rollout: rolloutState, Snapshot: snapshot}
	if mode == EnterpriseMemberModelAdmissionEnforcePublished && (!rolloutState.Valid || rolloutState.AutoStopped) {
		if !rolloutState.Valid {
			source = "rollout_invalid"
		} else {
			source = "auto_stop"
		}
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = source
		return runtime
	}
	if mode == EnterpriseMemberModelAdmissionEnforcePublished && !hasEnterpriseMemberModelAdmissionRolloutTarget(rollout) {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "rollout_shadow"
		runtime.Rollout.Reason = EnterpriseMemberModelAdmissionRolloutNoTargetReason
		return runtime
	}
	if mode == EnterpriseMemberModelAdmissionEnforcePublished && !readiness.CanaryReady {
		blockSource := "enforce_blocked"
		if readiness.AutoStop.Stopped {
			blockSource = readinessAutoStopSource(readiness)
		}
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = blockSource
		return runtime
	}
	if mode == EnterpriseMemberModelAdmissionEnforcePublished && rollout.Percentage > 0 && !readiness.ExpansionReady {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "expansion_blocked"
		return runtime
	}
	return runtime
}

func enterpriseMemberModelAdmissionRuntimeForGateway(ctx context.Context, mode EnterpriseMemberModelAdmissionMode, source string, rollout EnterpriseMemberModelAdmissionRolloutPolicy, generatedAt, expiresAt time.Time) EnterpriseMemberModelAdmissionRuntime {
	if mode != EnterpriseMemberModelAdmissionLegacyOrderOnly {
		return enforceSafeEnterpriseMemberModelAdmission(ctx, mode, source, rollout, generatedAt, expiresAt)
	}
	readiness := defaultEnterpriseMemberModelAdmissionReadinessProvider{}.EvaluateEnterpriseMemberModelAdmissionReadiness(ctx)
	rolloutState := EvaluateEnterpriseMemberModelAdmissionRollout(rollout, EnterpriseMemberModelAdmissionRolloutInput{})
	snapshot := BuildEnterpriseMemberModelAdmissionGateSnapshot(readiness, rollout, source, generatedAt, expiresAt)
	readiness.Snapshot = snapshot
	return EnterpriseMemberModelAdmissionRuntime{
		Mode:      EnterpriseMemberModelAdmissionLegacyOrderOnly,
		Source:    source,
		Readiness: readiness,
		Rollout:   rolloutState,
		Snapshot:  snapshot,
	}
}

type cachedEnterpriseMemberModelAdmission struct {
	runtime       EnterpriseMemberModelAdmissionRuntime
	rolloutPolicy EnterpriseMemberModelAdmissionRolloutPolicy
	expiresAt     int64
}

var enterpriseMemberLegacyAdmissionLastWarning atomic.Int64

func warnIfEnterpriseMemberLegacyAdmissionEffective(runtime EnterpriseMemberModelAdmissionRuntime) EnterpriseMemberModelAdmissionRuntime {
	if runtime.Mode != EnterpriseMemberModelAdmissionLegacyOrderOnly {
		return runtime
	}
	RecordEnterpriseMemberLegacyAdmissionEffective()
	if !claimEnterpriseMemberLegacyAdmissionWarning(time.Now()) {
		return runtime
	}
	slog.Warn(
		"deprecated enterprise member legacy admission mode is effective",
		"mode", string(runtime.Mode),
		"source", runtime.Source,
		"phase5_gate", EnterpriseMemberModelAdmissionDefaultCutoverGate,
	)
	return runtime
}

func claimEnterpriseMemberLegacyAdmissionWarning(now time.Time) bool {
	nowUnix := now.UnixNano()
	for {
		previous := enterpriseMemberLegacyAdmissionLastWarning.Load()
		if previous != 0 && nowUnix-previous < enterpriseMemberLegacyAdmissionWarningInterval.Nanoseconds() {
			return false
		}
		if enterpriseMemberLegacyAdmissionLastWarning.CompareAndSwap(previous, nowUnix) {
			return true
		}
	}
}

func hasEnterpriseMemberModelAdmissionRolloutTarget(policy EnterpriseMemberModelAdmissionRolloutPolicy) bool {
	normalized, err := NormalizeEnterpriseMemberModelAdmissionRolloutPolicy(policy)
	if err != nil {
		return false
	}
	return normalized.Percentage > 0 || len(normalized.EnterpriseUserIDs) > 0 || len(normalized.MemberIDs) > 0 || normalized.AutoStop
}

func readinessAutoStopSource(readiness EnterpriseMemberModelAdmissionEnforceReadiness) string {
	if readiness.AutoStop.Source == EnterpriseMemberModelAdmissionAutoStopSourceManual {
		return "manual_auto_stop"
	}
	return "metric_auto_stop"
}

const (
	nativeModelProtocolRoutingCacheTTL  = 5 * time.Second
	nativeModelProtocolRoutingErrorTTL  = 5 * time.Second
	nativeModelProtocolRoutingDBTimeout = 5 * time.Second
)

const (
	enterpriseMemberModelAdmissionCacheTTL  = 5 * time.Second
	enterpriseMemberModelAdmissionErrorTTL  = 5 * time.Second
	enterpriseMemberModelAdmissionDBTimeout = 5 * time.Second
)

var backendModeCache atomic.Value // *cachedBackendMode
var backendModeSF singleflight.Group

const backendModeCacheTTL = 60 * time.Second
const backendModeErrorTTL = 5 * time.Second
const backendModeDBTimeout = 5 * time.Second

// cachedGatewayForwardingSettings 缓存网关转发行为设置（进程内缓存，60s TTL）
type cachedGatewayForwardingSettings struct {
	openAITTFTMode                   string
	fingerprintUnification           bool
	metadataPassthrough              bool
	cchSigning                       bool
	claudeOAuthSystemPromptInjection bool
	claudeOAuthSystemPrompt          string
	claudeOAuthSystemPromptBlocks    string
	anthropicCacheTTL1hInjection     bool
	rewriteMessageCacheControl       bool
	clientDatelineNormalization      bool
	expiresAt                        int64 // unix nano
}

var gatewayForwardingCache atomic.Value // *cachedGatewayForwardingSettings
var gatewayForwardingSF singleflight.Group

const gatewayForwardingCacheTTL = 60 * time.Second
const gatewayForwardingErrorTTL = 5 * time.Second
const gatewayForwardingDBTimeout = 5 * time.Second

// cachedAccountSchedulingThresholds 缓存平台自动停调阈值（进程内缓存，60s TTL）
type cachedAccountSchedulingThresholds struct {
	thresholds map[string]int
	expiresAt  int64 // unix nano
}

var accountSchedulingThresholdsCache atomic.Value // *cachedAccountSchedulingThresholds
var accountSchedulingThresholdsSF singleflight.Group

const accountSchedulingThresholdsCacheTTL = 60 * time.Second
const accountSchedulingThresholdsErrorTTL = 5 * time.Second
const accountSchedulingThresholdsDBTimeout = 5 * time.Second

// cachedAntigravityUserAgentVersion 缓存 Antigravity UA 版本号（进程内缓存，60s TTL）
type cachedAntigravityUserAgentVersion struct {
	version   string
	expiresAt int64 // unix nano
}

const antigravityUserAgentVersionCacheTTL = 60 * time.Second
const antigravityUserAgentVersionErrorTTL = 5 * time.Second
const antigravityUserAgentVersionDBTimeout = 5 * time.Second

// DefaultOpenAICodexUserAgent 是 OpenAI Codex 默认 User-Agent，用于规避浏览器 UA 的质询。
// 默认采用 codex-tui 身份，版本段随 codexCLIVersion 一起更新。
const DefaultOpenAICodexUserAgent = codexCLIUserAgent

// cachedOpenAICodexUserAgent 缓存 OpenAI Codex UA（进程内缓存，60s TTL）
type cachedOpenAICodexUserAgent struct {
	value     string
	expiresAt int64 // unix nano
}

// cachedOpenAICodexClientVersion 缓存出站 Codex 客户端版本号（进程内缓存，60s TTL）
type cachedOpenAICodexClientVersion struct {
	version   string
	expiresAt int64 // unix nano
}

const openAICodexClientVersionCacheTTL = 60 * time.Second
const openAICodexClientVersionErrorTTL = 5 * time.Second
const openAICodexClientVersionDBTimeout = 5 * time.Second

// openAICodexClientVersionSFKey singleflight 键。
const openAICodexClientVersionSFKey = "openai_codex_client_version"

type cachedOpenAIQuotaAutoPauseSettings struct {
	settings  OpsOpenAIAccountQuotaAutoPauseSettings
	expiresAt int64
}

const openAICodexUserAgentCacheTTL = 60 * time.Second
const openAICodexUserAgentErrorTTL = 5 * time.Second
const openAICodexUserAgentDBTimeout = 5 * time.Second

const codexRestrictionPolicyCacheTTL = 60 * time.Second
const codexRestrictionPolicyDBTimeout = 5 * time.Second

// cachedCodexRestrictionPolicy codex_cli_only 全局加固策略缓存（进程内，60s TTL）。
// GetCodexRestrictionPolicy 在每个 codex_cli_only 账号的网关请求热路径上被调用，避免每次访问 DB。
type cachedCodexRestrictionPolicy struct {
	value     CodexRestrictionPolicy
	expiresAt int64 // unix nano
}

// cachedCyberSessionBlockRuntime cyber 会话屏蔽开关+TTL 进程内缓存（60s TTL）。
// GetCyberSessionBlockRuntime 在网关请求热路径上被调用，避免每次访问 DB。
type cachedCyberSessionBlockRuntime struct {
	enabled   bool
	ttl       time.Duration
	expiresAt int64 // unix nano
}

const cyberSessionBlockRuntimeCacheTTL = 60 * time.Second
const cyberSessionBlockRuntimeErrorTTL = 5 * time.Second
const cyberSessionBlockRuntimeDBTimeout = 5 * time.Second

const openAIQuotaAutoPauseSettingsCacheTTL = 60 * time.Second
const openAIQuotaAutoPauseSettingsErrorTTL = 5 * time.Second
const openAIQuotaAutoPauseSettingsDBTimeout = 5 * time.Second

const openAIQuotaAutoPauseSettingsRefreshKey = "openai_quota_auto_pause_settings"

// GetCyberSessionBlockRuntime 返回 (开关, TTL)，进程内缓存 ~60s，
// 供网关热路径读取时避免 DB 往返。
// 两个 setting key 在单次 singleflight 里一起读取，减少 DB 往返。
// 默认值：开关 false，TTL 1h（与粘性会话对齐）。
func (s *SettingService) GetCyberSessionBlockRuntime(ctx context.Context) (bool, time.Duration) {
	if cached, ok := s.cyberSessionBlockRuntimeCache.Load().(*cachedCyberSessionBlockRuntime); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.enabled, cached.ttl
		}
	}
	result, _, _ := s.cyberSessionBlockRuntimeSF.Do("cyber_session_block_runtime", func() (any, error) {
		if cached, ok := s.cyberSessionBlockRuntimeCache.Load().(*cachedCyberSessionBlockRuntime); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cyberSessionBlockRuntimeDBTimeout)
		defer cancel()

		enabledVal, enabledErr := s.settingRepo.GetValue(dbCtx, SettingKeyCyberSessionBlockEnabled)
		ttlVal, ttlErr := s.settingRepo.GetValue(dbCtx, SettingKeyCyberSessionBlockTTLSeconds)

		if enabledErr != nil && !errors.Is(enabledErr, ErrSettingNotFound) {
			slog.Warn("failed to get cyber_session_block_enabled setting", "error", enabledErr)
			entry := &cachedCyberSessionBlockRuntime{
				enabled:   false,
				ttl:       time.Hour,
				expiresAt: time.Now().Add(cyberSessionBlockRuntimeErrorTTL).UnixNano(),
			}
			s.cyberSessionBlockRuntimeCache.Store(entry)
			return entry, nil
		}

		enabled := enabledErr == nil && strings.TrimSpace(enabledVal) == "true"

		ttl := time.Hour
		if ttlErr == nil {
			if n, perr := strconv.Atoi(strings.TrimSpace(ttlVal)); perr == nil && n > 0 {
				ttl = time.Duration(n) * time.Second
			}
		}

		entry := &cachedCyberSessionBlockRuntime{
			enabled:   enabled,
			ttl:       ttl,
			expiresAt: time.Now().Add(cyberSessionBlockRuntimeCacheTTL).UnixNano(),
		}
		s.cyberSessionBlockRuntimeCache.Store(entry)
		return entry, nil
	})
	if entry, ok := result.(*cachedCyberSessionBlockRuntime); ok && entry != nil {
		return entry.enabled, entry.ttl
	}
	return false, time.Hour
}

// GetAntigravityUserAgentVersion 返回 Antigravity 上游请求使用的版本号。
// 后台设置优先；为空、缺失或非法时回退到 ANTIGRAVITY_USER_AGENT_VERSION / 内置默认值。
func (s *SettingService) GetAntigravityUserAgentVersion(ctx context.Context) string {
	fallback := antigravity.GetDefaultUserAgentVersion()
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if cached, ok := s.antigravityUAVersionCache.Load().(*cachedAntigravityUserAgentVersion); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.version
		}
	}

	result, _, _ := s.antigravityUAVersionSF.Do("antigravity_user_agent_version", func() (any, error) {
		if cached, ok := s.antigravityUAVersionCache.Load().(*cachedAntigravityUserAgentVersion); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.version, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), antigravityUserAgentVersionDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyAntigravityUserAgentVersion)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("failed to get antigravity user agent version setting", "error", err)
			s.antigravityUAVersionCache.Store(&cachedAntigravityUserAgentVersion{
				version:   fallback,
				expiresAt: time.Now().Add(antigravityUserAgentVersionErrorTTL).UnixNano(),
			})
			return fallback, nil
		}
		version := antigravity.NormalizeUserAgentVersion(value)
		if version == "" {
			version = fallback
		}
		s.antigravityUAVersionCache.Store(&cachedAntigravityUserAgentVersion{
			version:   version,
			expiresAt: time.Now().Add(antigravityUserAgentVersionCacheTTL).UnixNano(),
		})
		return version, nil
	})
	if version, ok := result.(string); ok && version != "" {
		return version
	}
	return fallback
}

// GetOpenAICodexUserAgent 返回 OpenAI Codex 上游请求使用的 User-Agent。
// 后台设置优先；为空时回退到内置默认值。
func (s *SettingService) GetOpenAICodexUserAgent(ctx context.Context) string {
	fallback := DefaultOpenAICodexUserAgent
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if cached, ok := s.openAICodexUACache.Load().(*cachedOpenAICodexUserAgent); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.value
		}
	}

	result, _, _ := s.openAICodexUASF.Do("openai_codex_user_agent", func() (any, error) {
		if cached, ok := s.openAICodexUACache.Load().(*cachedOpenAICodexUserAgent); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.value, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAICodexUserAgentDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpenAICodexUserAgent)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("failed to get openai codex user agent setting", "error", err)
			s.openAICodexUACache.Store(&cachedOpenAICodexUserAgent{
				value:     fallback,
				expiresAt: time.Now().Add(openAICodexUserAgentErrorTTL).UnixNano(),
			})
			return fallback, nil
		}
		ua := strings.TrimSpace(value)
		if ua == "" {
			ua = fallback
		}
		s.openAICodexUACache.Store(&cachedOpenAICodexUserAgent{
			value:     ua,
			expiresAt: time.Now().Add(openAICodexUserAgentCacheTTL).UnixNano(),
		})
		return ua, nil
	})
	if ua, ok := result.(string); ok && ua != "" {
		return ua
	}
	return fallback
}

// GetOpenAICodexClientVersion 返回出站声明的 Codex 客户端版本号。
// 优先级：管理员在面板覆写的版本 → 自动同步到的官方最新稳定版 → 内置常量。
// 上游在容量紧张时按客户端身份分优先级降载，陈旧版本会被优先丢弃，故该值需保持跟随官方发布；
// 自动同步让运维不必为了跟版本而发新版本。
func (s *SettingService) GetOpenAICodexClientVersion(ctx context.Context) string {
	fallback := codexCLIVersion
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if cached, ok := s.openAICodexVersionCache.Load().(*cachedOpenAICodexClientVersion); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.version
		}
	}

	result, _, _ := s.openAICodexVersionSF.Do(openAICodexClientVersionSFKey, func() (any, error) {
		if cached, ok := s.openAICodexVersionCache.Load().(*cachedOpenAICodexClientVersion); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.version, nil
			}
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAICodexClientVersionDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyOpenAICodexClientVersion,
			SettingKeyOpenAICodexClientVersionSynced,
		})
		if err != nil {
			slog.Warn("failed to get openai codex client version setting", "error", err)
			s.openAICodexVersionCache.Store(&cachedOpenAICodexClientVersion{
				version:   fallback,
				expiresAt: time.Now().Add(openAICodexClientVersionErrorTTL).UnixNano(),
			})
			return fallback, nil
		}
		version := NormalizeCodexClientVersion(values[SettingKeyOpenAICodexClientVersion])
		if version == "" {
			version = NormalizeCodexClientVersion(values[SettingKeyOpenAICodexClientVersionSynced])
		}
		if version == "" {
			version = fallback
		}
		s.openAICodexVersionCache.Store(&cachedOpenAICodexClientVersion{
			version:   version,
			expiresAt: time.Now().Add(openAICodexClientVersionCacheTTL).UnixNano(),
		})
		return version, nil
	})
	if version, ok := result.(string); ok && version != "" {
		return version
	}
	return fallback
}

// InvalidateOpenAICodexClientVersionCache 丢弃版本号缓存，下次读取回源。
// 面板保存与自动同步写入后调用。
func (s *SettingService) InvalidateOpenAICodexClientVersionCache() {
	if s == nil {
		return
	}
	s.openAICodexVersionSF.Forget(openAICodexClientVersionSFKey)
	s.openAICodexVersionCache.Store((*cachedOpenAICodexClientVersion)(nil))
}

// GetOpenAICodexCanonicalUserAgent 返回出站规范 Codex User-Agent。
// 未填面板 UA 时按当前生效的客户端版本号拼出标准 Codex TUI UA。
//
// 面板 UA 只贡献客户端名与 OS / 架构 / 终端指纹，版本段一律用生效版本重建：该输入框是
// 唯一能改 UA 后缀的地方，但它填写于某个历史版本，逐字沿用会把出站身份永久钉死在陈旧
// 版本上并绕过自动同步——而陈旧身份正是上游优先降载的那一侧。
// 需要固定版本请填「Codex 客户端版本号」并关闭自动同步。
func (s *SettingService) GetOpenAICodexCanonicalUserAgent(ctx context.Context) string {
	if s == nil {
		return codexCLIUserAgent
	}
	version := s.GetOpenAICodexClientVersion(ctx)
	ua := strings.TrimSpace(s.GetOpenAICodexUserAgent(ctx))
	if ua == "" {
		return buildCodexCLIUserAgent(version)
	}
	if rebuilt := openai.SetCodexUserAgentVersion(ua, version); rebuilt != "" {
		return rebuilt
	}
	// 非 `{client}/{version}` 形态：交给 PairCodexClientIdentity 判定，
	// 推导不出官方身份时由收口整体回退规范身份。
	return ua
}

var legacyClaudeCodeCodexWhitelistEntry = openai.AllowedClientEntry{
	Originator: "Claude Code",
	UAContains: []string{"Claude Code/"},
}

// MigrateOpenAIAllowClaudeCodeCodexPluginSetting folds the deprecated global Claude Code
// plugin allow switch into codex_cli_only_whitelist. The app-server identity model is the
// same originator + UA marker pair, so runtime checks no longer need a separate flag.
func (s *SettingService) MigrateOpenAIAllowClaudeCodeCodexPluginSetting(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), codexRestrictionPolicyDBTimeout)
	defer cancel()

	legacyValue, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpenAIAllowClaudeCodeCodexPlugin)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil
		}
		return fmt.Errorf("get deprecated %s setting: %w", SettingKeyOpenAIAllowClaudeCodeCodexPlugin, err)
	}
	if strings.TrimSpace(legacyValue) != "true" {
		return nil
	}

	rawWhitelist, err := s.settingRepo.GetValue(dbCtx, SettingKeyCodexCLIOnlyWhitelist)
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("get %s setting: %w", SettingKeyCodexCLIOnlyWhitelist, err)
	}

	var entries []openai.AllowedClientEntry
	if strings.TrimSpace(rawWhitelist) != "" {
		if err := json.Unmarshal([]byte(rawWhitelist), &entries); err != nil {
			return fmt.Errorf("parse %s setting: %w", SettingKeyCodexCLIOnlyWhitelist, err)
		}
	}
	if codexClientEntriesContain(entries, legacyClaudeCodeCodexWhitelistEntry) {
		return nil
	}

	entries = append(entries, legacyClaudeCodeCodexWhitelistEntry)
	encoded, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal %s setting: %w", SettingKeyCodexCLIOnlyWhitelist, err)
	}
	if err := s.settingRepo.Set(dbCtx, SettingKeyCodexCLIOnlyWhitelist, string(encoded)); err != nil {
		return fmt.Errorf("set %s setting: %w", SettingKeyCodexCLIOnlyWhitelist, err)
	}
	s.codexRestrictionPolicySF.Forget("codex_restriction_policy")
	s.codexRestrictionPolicyCache.Store(&cachedCodexRestrictionPolicy{expiresAt: 0})
	return nil
}

// MigrateGrokDefaultTextModel upgrades the pre-4.6 built-in default for
// existing installations. Explicit operator choices are left untouched.
func (s *SettingService) MigrateGrokDefaultTextModel(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), codexRestrictionPolicyDBTimeout)
	defer cancel()

	value, err := s.settingRepo.GetValue(dbCtx, SettingKeyGrokDefaultTextModel)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return nil
		}
		return fmt.Errorf("get %s setting: %w", SettingKeyGrokDefaultTextModel, err)
	}
	// Only migrate the value that was previously shipped as the built-in
	// default. Any other value is an explicit operator choice or a future
	// default and must remain unchanged.
	if strings.TrimSpace(value) != "grok-4.5" {
		return nil
	}
	if err := s.settingRepo.Set(dbCtx, SettingKeyGrokDefaultTextModel, "grok-4.6"); err != nil {
		return fmt.Errorf("set %s setting: %w", SettingKeyGrokDefaultTextModel, err)
	}
	return nil
}

// MigrateCodexBodyFingerprintToSignals 把已废弃的 codex_cli_only_allow_body_engine_fingerprint
// 开关并入引擎指纹信号列表。幂等:信号键已存在(非空)则不动;缺失时写默认种子,
// 并把 body 路径行的 Required 设为旧 body 开关的值(旧 true ⇒ 勾上 body 行)。
func (s *SettingService) MigrateCodexBodyFingerprintToSignals(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), codexRestrictionPolicyDBTimeout)
	defer cancel()

	if v, err := s.settingRepo.GetValue(dbCtx, SettingKeyCodexCLIOnlyEngineFingerprintSignals); err == nil && strings.TrimSpace(v) != "" {
		return nil // 已配置/已迁移
	} else if err != nil && !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("get %s setting: %w", SettingKeyCodexCLIOnlyEngineFingerprintSignals, err)
	}

	bodyOn := false
	if v, err := s.settingRepo.GetValue(dbCtx, SettingKeyCodexCLIOnlyAllowBodyEngineFingerprint); err == nil {
		bodyOn = strings.TrimSpace(v) == "true"
	} else if !errors.Is(err, ErrSettingNotFound) {
		return fmt.Errorf("get deprecated %s setting: %w", SettingKeyCodexCLIOnlyAllowBodyEngineFingerprint, err)
	}

	seed := make([]openai.EngineFingerprintSignal, len(openai.DefaultEngineFingerprintSignals))
	copy(seed, openai.DefaultEngineFingerprintSignals)
	if bodyOn {
		for i := range seed {
			if seed[i].Type == openai.FingerprintSignalBodyPath {
				seed[i].Required = true
			}
		}
	}
	encoded, err := json.Marshal(seed)
	if err != nil {
		return fmt.Errorf("marshal %s setting: %w", SettingKeyCodexCLIOnlyEngineFingerprintSignals, err)
	}
	if err := s.settingRepo.Set(dbCtx, SettingKeyCodexCLIOnlyEngineFingerprintSignals, string(encoded)); err != nil {
		return fmt.Errorf("set %s setting: %w", SettingKeyCodexCLIOnlyEngineFingerprintSignals, err)
	}
	s.codexRestrictionPolicySF.Forget("codex_restriction_policy")
	s.codexRestrictionPolicyCache.Store(&cachedCodexRestrictionPolicy{expiresAt: 0})
	return nil
}

func codexClientEntriesContain(entries []openai.AllowedClientEntry, want openai.AllowedClientEntry) bool {
	wantOriginator := strings.TrimSpace(want.Originator)
	if wantOriginator == "" {
		return false
	}
	wantMarkers := normalizedCodexClientMarkers(want.UAContains)
	if len(wantMarkers) == 0 {
		return false
	}
	for _, entry := range entries {
		if !strings.EqualFold(strings.TrimSpace(entry.Originator), wantOriginator) {
			continue
		}
		gotMarkers := normalizedCodexClientMarkers(entry.UAContains)
		if len(gotMarkers) != len(wantMarkers) {
			continue
		}
		matched := true
		for marker := range wantMarkers {
			if _, ok := gotMarkers[marker]; !ok {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func normalizedCodexClientMarkers(markers []string) map[string]struct{} {
	normalized := make(map[string]struct{}, len(markers))
	for _, marker := range markers {
		marker = strings.TrimSpace(marker)
		if marker == "" {
			continue
		}
		normalized[strings.ToLower(marker)] = struct{}{}
	}
	return normalized
}

// GetCodexRestrictionPolicy 读取 codex_cli_only 全局加固策略（黑/白名单、最低版本、引擎指纹门）。
// 仅在调用方已确认账号 codex_cli_only 开启时读取；进程内 atomic.Value 缓存（60s TTL）避免热路径访问 DB。
// 任意键缺失/解析失败 → 安全默认：空名单、空版本、默认种子指纹信号。
func (s *SettingService) GetCodexRestrictionPolicy(ctx context.Context) CodexRestrictionPolicy {
	if cached, ok := s.codexRestrictionPolicyCache.Load().(*cachedCodexRestrictionPolicy); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.value
		}
	}
	result, _, _ := s.codexRestrictionPolicySF.Do("codex_restriction_policy", func() (any, error) {
		if cached, ok := s.codexRestrictionPolicyCache.Load().(*cachedCodexRestrictionPolicy); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.value, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), codexRestrictionPolicyDBTimeout)
		defer cancel()

		pol := CodexRestrictionPolicy{EngineFingerprintSignals: openai.DefaultEngineFingerprintSignals} // 安全默认：默认种子指纹信号
		if v, err := s.settingRepo.GetValue(dbCtx, SettingKeyMinCodexVersion); err == nil {
			pol.MinCodexVersion = strings.TrimSpace(v)
		}
		if v, err := s.settingRepo.GetValue(dbCtx, SettingKeyMaxCodexVersion); err == nil {
			pol.MaxCodexVersion = strings.TrimSpace(v)
		}
		if v, err := s.settingRepo.GetValue(dbCtx, SettingKeyCodexCLIOnlyAllowAppServerClients); err == nil {
			pol.AllowAppServerClients = strings.TrimSpace(v) == "true" // 仅显式 "true" 开启
		}
		pol.EngineFingerprintSignals = s.loadEngineFingerprintSignals(dbCtx)
		pol.Whitelist = s.loadCodexClientEntries(dbCtx, SettingKeyCodexCLIOnlyWhitelist)
		pol.Blacklist = s.loadCodexClientEntries(dbCtx, SettingKeyCodexCLIOnlyBlacklist)

		s.codexRestrictionPolicyCache.Store(&cachedCodexRestrictionPolicy{
			value:     pol,
			expiresAt: time.Now().Add(codexRestrictionPolicyCacheTTL).UnixNano(),
		})
		return pol, nil
	})
	if pol, ok := result.(CodexRestrictionPolicy); ok {
		return pol
	}
	return CodexRestrictionPolicy{EngineFingerprintSignals: openai.DefaultEngineFingerprintSignals}
}

// loadCodexClientEntries 读取并解析 []openai.AllowedClientEntry JSON 设置；缺失/空/非法 → nil（安全忽略）。
func (s *SettingService) loadCodexClientEntries(ctx context.Context, key string) []openai.AllowedClientEntry {
	v, err := s.settingRepo.GetValue(ctx, key)
	if err != nil || strings.TrimSpace(v) == "" {
		return nil
	}
	var entries []openai.AllowedClientEntry
	if json.Unmarshal([]byte(v), &entries) != nil {
		return nil
	}
	return entries
}

// loadEngineFingerprintSignals 读取引擎指纹信号列表;缺失/空/非法 → 默认种子。
func (s *SettingService) loadEngineFingerprintSignals(ctx context.Context) []openai.EngineFingerprintSignal {
	v, err := s.settingRepo.GetValue(ctx, SettingKeyCodexCLIOnlyEngineFingerprintSignals)
	if err != nil || strings.TrimSpace(v) == "" {
		return openai.DefaultEngineFingerprintSignals
	}
	sigs, ok := openai.ParseEngineFingerprintSignals(v)
	if !ok {
		return openai.DefaultEngineFingerprintSignals
	}
	return sigs
}

// ValidateCodexClientEntriesJSON 校验 codex_cli_only 名单 JSON 配置（黑名单语义）：
// 空=合法（禁用）；非空须为 []AllowedClientEntry 的 JSON 数组。黑名单是 OR 宽 deny，
// 允许 originator-only 条目，故不校验 ua_contains。白名单请用 ValidateCodexWhitelistEntriesJSON。
func ValidateCodexClientEntriesJSON(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var entries []openai.AllowedClientEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return fmt.Errorf("must be empty or a valid JSON array of {originator, ua_contains}")
	}
	return nil
}

// ValidateCodexWhitelistEntriesJSON 在 ValidateCodexClientEntriesJSON 的数组结构校验之上，额外要求
// 每条白名单条目「有可能命中」（openai.AllowedClientEntry.IsWhitelistable）。白名单是双因子 AND：
// originator-only、空或含空白 ua_contains 的条目会在运行时静默失效——这里让管理员在写入时即收到反馈，
// 而非存入永不命中的死规则。黑名单（OR 宽 deny）仍用 ValidateCodexClientEntriesJSON。
func ValidateCodexWhitelistEntriesJSON(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var entries []openai.AllowedClientEntry
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return fmt.Errorf("must be empty or a valid JSON array of {originator, ua_contains}")
	}
	for i, e := range entries {
		if !e.IsWhitelistable() {
			return fmt.Errorf("entry %d: whitelist requires a non-empty originator and at least one non-empty ua_contains (double-factor AND; otherwise the rule never matches)", i)
		}
	}
	return nil
}

// ValidateEngineFingerprintSignalsJSON 服务层包装,复用 openai 校验逻辑。
func ValidateEngineFingerprintSignalsJSON(raw string) error {
	return openai.ValidateEngineFingerprintSignalsJSON(raw)
}

// IsBackendModeEnabled checks if backend mode is enabled
// Uses in-process atomic.Value cache with 60s TTL, zero-lock hot path
func (s *SettingService) IsBackendModeEnabled(ctx context.Context) bool {
	if cached, ok := backendModeCache.Load().(*cachedBackendMode); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.value
		}
	}
	result, _, _ := backendModeSF.Do("backend_mode", func() (any, error) {
		if cached, ok := backendModeCache.Load().(*cachedBackendMode); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cached.value, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), backendModeDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyBackendModeEnabled)
		if err != nil {
			if errors.Is(err, ErrSettingNotFound) {
				// Setting not yet created (fresh install) - default to disabled with full TTL
				backendModeCache.Store(&cachedBackendMode{
					value:     false,
					expiresAt: time.Now().Add(backendModeCacheTTL).UnixNano(),
				})
				return false, nil
			}
			slog.Warn("failed to get backend_mode_enabled setting", "error", err)
			backendModeCache.Store(&cachedBackendMode{
				value:     false,
				expiresAt: time.Now().Add(backendModeErrorTTL).UnixNano(),
			})
			return false, nil
		}
		enabled := value == "true"
		backendModeCache.Store(&cachedBackendMode{
			value:     enabled,
			expiresAt: time.Now().Add(backendModeCacheTTL).UnixNano(),
		})
		return enabled, nil
	})
	if val, ok := result.(bool); ok {
		return val
	}
	return false
}

func (s *SettingService) nativeModelProtocolRoutingConfigDefault() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.NativeModelProtocolRoutingEnabled
}

// GetNativeModelProtocolRoutingRuntime returns the single effective switch used
// by every native model-protocol consumer. A missing DB key deliberately
// inherits the deployment config, preserving existing YAML/environment setups.
// Successful admin writes refresh the local cache immediately; other instances
// converge within the short cache TTL through the shared settings table.
func (s *SettingService) GetNativeModelProtocolRoutingRuntime(ctx context.Context) NativeModelProtocolRoutingRuntime {
	fallback := NativeModelProtocolRoutingRuntime{
		Enabled: s.nativeModelProtocolRoutingConfigDefault(),
		Source:  "config",
	}
	if s == nil || s.settingRepo == nil {
		return fallback
	}
	if cached, ok := s.nativeModelProtocolRoutingCache.Load().(*cachedNativeModelProtocolRouting); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return NativeModelProtocolRoutingRuntime{Enabled: cached.enabled, Source: cached.source}
		}
	}

	value, _, _ := s.nativeModelProtocolRoutingSF.Do("native_model_protocol_routing", func() (any, error) {
		var stale *cachedNativeModelProtocolRouting
		if cached, ok := s.nativeModelProtocolRoutingCache.Load().(*cachedNativeModelProtocolRouting); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return NativeModelProtocolRoutingRuntime{Enabled: cached.enabled, Source: cached.source}, nil
			}
			stale = cached
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nativeModelProtocolRoutingDBTimeout)
		defer cancel()

		raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyNativeModelProtocolRoutingEnabled)
		runtime := fallback
		ttl := nativeModelProtocolRoutingCacheTTL
		switch {
		case err == nil:
			runtime.Enabled = strings.TrimSpace(raw) == "true"
			runtime.Source = "settings"
		case errors.Is(err, ErrSettingNotFound):
			// Deliberately inherit the deployment default until an administrator
			// explicitly saves an override.
		default:
			slog.Warn("failed to get native model protocol routing setting", "error", err)
			// A database-backed false value is an operational kill switch. Never
			// re-enable routing from a true config fallback merely because that
			// override is temporarily unreadable. Preserve the last known state;
			// without one, fail closed until settings can be read again.
			if stale != nil {
				runtime.Enabled = stale.enabled
				runtime.Source = stale.source
			} else {
				runtime.Enabled = false
				runtime.Source = "error_fallback"
			}
			ttl = nativeModelProtocolRoutingErrorTTL
		}
		s.nativeModelProtocolRoutingCache.Store(&cachedNativeModelProtocolRouting{
			enabled:   runtime.Enabled,
			source:    runtime.Source,
			expiresAt: time.Now().Add(ttl).UnixNano(),
		})
		return runtime, nil
	})
	if runtime, ok := value.(NativeModelProtocolRoutingRuntime); ok {
		return runtime
	}
	return fallback
}

func (s *SettingService) IsNativeModelProtocolRoutingEnabled(ctx context.Context) bool {
	return s.GetNativeModelProtocolRoutingRuntime(ctx).Enabled
}

func normalizeEnterpriseMemberModelAdmissionMode(raw string) (EnterpriseMemberModelAdmissionMode, bool) {
	switch EnterpriseMemberModelAdmissionMode(strings.ToLower(strings.TrimSpace(raw))) {
	case EnterpriseMemberModelAdmissionLegacyOrderOnly, "":
		return EnterpriseMemberModelAdmissionLegacyOrderOnly, true
	case EnterpriseMemberModelAdmissionShadowPublished:
		return EnterpriseMemberModelAdmissionShadowPublished, true
	case EnterpriseMemberModelAdmissionEnforcePublished:
		return EnterpriseMemberModelAdmissionEnforcePublished, true
	default:
		return EnterpriseMemberModelAdmissionLegacyOrderOnly, false
	}
}

func NormalizeEnterpriseMemberModelAdmissionModeForSettings(raw string) (EnterpriseMemberModelAdmissionMode, bool) {
	return normalizeEnterpriseMemberModelAdmissionMode(raw)
}

func (s *SettingService) enterpriseMemberModelAdmissionConfigDefault() (EnterpriseMemberModelAdmissionMode, string) {
	if s == nil || s.cfg == nil {
		return EnterpriseMemberModelAdmissionLegacyOrderOnly, "config"
	}
	mode, ok := normalizeEnterpriseMemberModelAdmissionMode(s.cfg.Gateway.EnterpriseMemberModelAdmissionMode)
	if !ok {
		return EnterpriseMemberModelAdmissionLegacyOrderOnly, "config_invalid"
	}
	return mode, "config"
}

func enterpriseMemberModelAdmissionRolloutFromSettings(settings map[string]string) (EnterpriseMemberModelAdmissionRolloutPolicy, string) {
	raw, ok := settings[SettingKeyEnterpriseMemberModelAdmissionRolloutPolicy]
	if !ok || strings.TrimSpace(raw) == "" {
		return DefaultEnterpriseMemberModelAdmissionRolloutPolicy(), "default"
	}
	policy, err := ParseEnterpriseMemberModelAdmissionRolloutPolicy(raw)
	if err != nil {
		return EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: -1}, "settings_invalid"
	}
	return policy, "settings"
}

// GetEnterpriseMemberModelAdmissionRuntime returns the effective runtime mode.
// Missing DB values inherit the deployment config, while unreadable settings use
// the load-safe legacy default so shadow planning remains an explicit opt-in.
func (s *SettingService) GetEnterpriseMemberModelAdmissionRuntime(ctx context.Context) EnterpriseMemberModelAdmissionRuntime {
	mode, source := s.enterpriseMemberModelAdmissionConfigDefault()
	defaultRollout := DefaultEnterpriseMemberModelAdmissionRolloutPolicy()
	if s == nil || s.settingRepo == nil {
		now := time.Now()
		fallback := enterpriseMemberModelAdmissionRuntimeForGateway(ctx, mode, source, defaultRollout, now, now.Add(enterpriseMemberModelAdmissionCacheTTL))
		return fallbackWithAdmissionSnapshotAge(fallback)
	}
	if cached, ok := s.enterpriseMemberAdmissionCache.Load().(*cachedEnterpriseMemberModelAdmission); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return cachedEnterpriseMemberAdmissionRuntimeWithAge(cached)
		}
	}

	value, _, _ := s.enterpriseMemberAdmissionSF.Do("enterprise_member_model_admission", func() (any, error) {
		var stale *cachedEnterpriseMemberModelAdmission
		if cached, ok := s.enterpriseMemberAdmissionCache.Load().(*cachedEnterpriseMemberModelAdmission); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return cachedEnterpriseMemberAdmissionRuntimeWithAge(cached), nil
			}
			stale = cached
		}
		if ctx == nil {
			ctx = context.Background()
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), enterpriseMemberModelAdmissionDBTimeout)
		defer cancel()

		raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyEnterpriseMemberModelAdmissionMode)
		runtime := EnterpriseMemberModelAdmissionRuntime{Mode: mode, Source: source}
		rolloutPolicy := defaultRollout
		rolloutSource := "default"
		if rawRollout, rolloutErr := s.settingRepo.GetValue(dbCtx, SettingKeyEnterpriseMemberModelAdmissionRolloutPolicy); rolloutErr == nil {
			if parsed, parseErr := ParseEnterpriseMemberModelAdmissionRolloutPolicy(rawRollout); parseErr == nil {
				rolloutPolicy = parsed
				rolloutSource = "settings"
			} else {
				rolloutPolicy = EnterpriseMemberModelAdmissionRolloutPolicy{Percentage: -1}
				rolloutSource = "settings_invalid"
			}
		} else if !errors.Is(rolloutErr, ErrSettingNotFound) {
			slog.Warn("failed to get enterprise member model admission rollout policy setting", "error", rolloutErr)
			if stale != nil {
				rolloutPolicy = stale.rolloutPolicy
				rolloutSource = "stale"
			}
		}
		ttl := enterpriseMemberModelAdmissionCacheTTL
		switch {
		case err == nil:
			mode, ok := normalizeEnterpriseMemberModelAdmissionMode(raw)
			runtime.Mode = mode
			if ok {
				runtime.Source = "settings"
			} else {
				runtime.Source = "settings_invalid"
			}
		case errors.Is(err, ErrSettingNotFound):
			// Inherit config until an administrator explicitly persists a mode.
		default:
			slog.Warn("failed to get enterprise member model admission mode setting", "error", err)
			runtime.Mode = EnterpriseMemberModelAdmissionLegacyOrderOnly
			runtime.Source = "error_fallback"
			if stale != nil {
				rolloutPolicy = stale.rolloutPolicy
			}
			ttl = enterpriseMemberModelAdmissionErrorTTL
		}
		generatedAt := time.Now()
		expiresAt := generatedAt.Add(ttl)
		runtime = enterpriseMemberModelAdmissionRuntimeForGateway(dbCtx, runtime.Mode, runtime.Source, rolloutPolicy, generatedAt, expiresAt)
		runtime.Rollout.Source = rolloutSource
		runtime.Readiness.Snapshot = runtime.Snapshot
		s.enterpriseMemberAdmissionCache.Store(&cachedEnterpriseMemberModelAdmission{
			runtime:       runtime,
			rolloutPolicy: rolloutPolicy,
			expiresAt:     expiresAt.UnixNano(),
		})
		return runtime, nil
	})
	if runtime, ok := value.(EnterpriseMemberModelAdmissionRuntime); ok {
		return fallbackWithAdmissionSnapshotAge(runtime)
	}
	now := time.Now()
	fallback := enterpriseMemberModelAdmissionRuntimeForGateway(ctx, mode, source, defaultRollout, now, now.Add(enterpriseMemberModelAdmissionCacheTTL))
	return fallbackWithAdmissionSnapshotAge(fallback)
}

func cachedEnterpriseMemberAdmissionRuntimeWithAge(cached *cachedEnterpriseMemberModelAdmission) EnterpriseMemberModelAdmissionRuntime {
	if cached == nil {
		return EnterpriseMemberModelAdmissionRuntime{Mode: EnterpriseMemberModelAdmissionShadowPublished, Source: "error_fallback"}
	}
	return fallbackWithAdmissionSnapshotAge(cached.runtime)
}

func fallbackWithAdmissionSnapshotAge(runtime EnterpriseMemberModelAdmissionRuntime) EnterpriseMemberModelAdmissionRuntime {
	runtime.Snapshot = runtime.Snapshot.WithAge(time.Now())
	runtime.Readiness.Snapshot = runtime.Snapshot
	return warnIfEnterpriseMemberLegacyAdmissionEffective(runtime)
}

func (s *SettingService) GetEnterpriseMemberModelAdmissionMode(ctx context.Context) EnterpriseMemberModelAdmissionMode {
	return s.GetEnterpriseMemberModelAdmissionRuntime(ctx).Mode
}

func (s *SettingService) ResolveEnterpriseMemberModelAdmissionMode(ctx context.Context, input EnterpriseMemberModelAdmissionRolloutInput) EnterpriseMemberModelAdmissionRuntime {
	runtime := s.GetEnterpriseMemberModelAdmissionRuntime(ctx)
	if runtime.Mode != EnterpriseMemberModelAdmissionEnforcePublished {
		return runtime
	}
	rolloutState := EvaluateEnterpriseMemberModelAdmissionRollout(runtime.Rollout.Policy, input)
	rolloutState.Source = runtime.Rollout.Source
	runtime.Rollout = rolloutState
	if !rolloutState.Valid {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "rollout_invalid"
		return runtime
	}
	if rolloutState.AutoStopped {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "auto_stop"
		return runtime
	}
	readiness := runtime.Readiness
	runtime.Readiness = readiness
	if !readiness.CanaryReady || readiness.AutoStop.Stopped {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		if readiness.AutoStop.Stopped {
			runtime.Source = readinessAutoStopSource(readiness)
		} else {
			runtime.Source = "enforce_blocked"
		}
		return runtime
	}
	if !rolloutState.Matched {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "rollout_shadow"
		return runtime
	}
	if rolloutState.MatchedBy == "stable_hash" && !readiness.ExpansionReady {
		runtime.Mode = EnterpriseMemberModelAdmissionShadowPublished
		runtime.Source = "expansion_blocked"
		return runtime
	}
	runtime.Source = runtime.Source + ":rollout_" + rolloutState.MatchedBy
	return runtime
}

type gatewayForwardingSettingsResult struct {
	openAITTFTMode                                                                        string
	fp, mp, cch, claudeOAuthSystemPromptInjection, cacheTTL1h, rewriteMessageCacheControl bool
	clientDatelineNormalization                                                           bool
	claudeOAuthSystemPrompt, claudeOAuthSystemPromptBlocks                                string
}

func (s *SettingService) getGatewayForwardingSettingsCached(ctx context.Context) gatewayForwardingSettingsResult {
	if cached, ok := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings); ok && cached != nil {
		if time.Now().UnixNano() < cached.expiresAt {
			return gatewayForwardingSettingsResult{
				openAITTFTMode:                   cached.openAITTFTMode,
				fp:                               cached.fingerprintUnification,
				mp:                               cached.metadataPassthrough,
				cch:                              cached.cchSigning,
				claudeOAuthSystemPromptInjection: cached.claudeOAuthSystemPromptInjection,
				claudeOAuthSystemPrompt:          cached.claudeOAuthSystemPrompt,
				claudeOAuthSystemPromptBlocks:    cached.claudeOAuthSystemPromptBlocks,
				cacheTTL1h:                       cached.anthropicCacheTTL1hInjection,
				rewriteMessageCacheControl:       cached.rewriteMessageCacheControl,
				clientDatelineNormalization:      cached.clientDatelineNormalization,
			}
		}
	}
	val, _, _ := gatewayForwardingSF.Do("gateway_forwarding", func() (any, error) {
		if cached, ok := gatewayForwardingCache.Load().(*cachedGatewayForwardingSettings); ok && cached != nil {
			if time.Now().UnixNano() < cached.expiresAt {
				return gatewayForwardingSettingsResult{
					openAITTFTMode:                   cached.openAITTFTMode,
					fp:                               cached.fingerprintUnification,
					mp:                               cached.metadataPassthrough,
					cch:                              cached.cchSigning,
					claudeOAuthSystemPromptInjection: cached.claudeOAuthSystemPromptInjection,
					claudeOAuthSystemPrompt:          cached.claudeOAuthSystemPrompt,
					claudeOAuthSystemPromptBlocks:    cached.claudeOAuthSystemPromptBlocks,
					cacheTTL1h:                       cached.anthropicCacheTTL1hInjection,
					rewriteMessageCacheControl:       cached.rewriteMessageCacheControl,
					clientDatelineNormalization:      cached.clientDatelineNormalization,
				}, nil
			}
		}
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayForwardingDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyOpenAITTFTMode,
			SettingKeyEnableFingerprintUnification,
			SettingKeyEnableMetadataPassthrough,
			SettingKeyEnableCCHSigning,
			SettingKeyEnableClaudeOAuthSystemPromptInjection,
			SettingKeyClaudeOAuthSystemPrompt,
			SettingKeyClaudeOAuthSystemPromptBlocks,
			SettingKeyEnableAnthropicCacheTTL1hInjection,
			SettingKeyRewriteMessageCacheControl,
			SettingKeyEnableClientDatelineNormalization,
		})
		if err != nil {
			slog.Warn("failed to get gateway forwarding settings", "error", err)
			gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
				openAITTFTMode:                   OpenAITTFTModeSemantic,
				fingerprintUnification:           true,
				metadataPassthrough:              false,
				cchSigning:                       false,
				claudeOAuthSystemPromptInjection: true,
				anthropicCacheTTL1hInjection:     false,
				rewriteMessageCacheControl:       s.defaultRewriteMessageCacheControl(),
				clientDatelineNormalization:      true,
				expiresAt:                        time.Now().Add(gatewayForwardingErrorTTL).UnixNano(),
			})
			return gatewayForwardingSettingsResult{openAITTFTMode: OpenAITTFTModeSemantic, fp: true, claudeOAuthSystemPromptInjection: true, rewriteMessageCacheControl: s.defaultRewriteMessageCacheControl(), clientDatelineNormalization: true}, nil
		}
		ttftMode := normalizeOpenAITTFTMode(values[SettingKeyOpenAITTFTMode])
		fp := true
		if v, ok := values[SettingKeyEnableFingerprintUnification]; ok && v != "" {
			fp = v == "true"
		}
		mp := values[SettingKeyEnableMetadataPassthrough] == "true"
		cch := values[SettingKeyEnableCCHSigning] == "true"
		systemPromptInjection := true
		if v, ok := values[SettingKeyEnableClaudeOAuthSystemPromptInjection]; ok && v != "" {
			systemPromptInjection = v == "true"
		}
		systemPrompt := values[SettingKeyClaudeOAuthSystemPrompt]
		systemPromptBlocks := values[SettingKeyClaudeOAuthSystemPromptBlocks]
		cacheTTL1h := values[SettingKeyEnableAnthropicCacheTTL1hInjection] == "true"
		rewriteMessageCacheControl := s.defaultRewriteMessageCacheControl()
		if v, ok := values[SettingKeyRewriteMessageCacheControl]; ok && v != "" {
			rewriteMessageCacheControl = v == "true"
		}
		clientDatelineNormalization := true
		if v, ok := values[SettingKeyEnableClientDatelineNormalization]; ok && v != "" {
			clientDatelineNormalization = v == "true"
		}
		gatewayForwardingCache.Store(&cachedGatewayForwardingSettings{
			openAITTFTMode:                   ttftMode,
			fingerprintUnification:           fp,
			metadataPassthrough:              mp,
			cchSigning:                       cch,
			claudeOAuthSystemPromptInjection: systemPromptInjection,
			claudeOAuthSystemPrompt:          systemPrompt,
			claudeOAuthSystemPromptBlocks:    systemPromptBlocks,
			anthropicCacheTTL1hInjection:     cacheTTL1h,
			rewriteMessageCacheControl:       rewriteMessageCacheControl,
			clientDatelineNormalization:      clientDatelineNormalization,
			expiresAt:                        time.Now().Add(gatewayForwardingCacheTTL).UnixNano(),
		})
		return gatewayForwardingSettingsResult{
			openAITTFTMode:                   ttftMode,
			fp:                               fp,
			mp:                               mp,
			cch:                              cch,
			claudeOAuthSystemPromptInjection: systemPromptInjection,
			claudeOAuthSystemPrompt:          systemPrompt,
			claudeOAuthSystemPromptBlocks:    systemPromptBlocks,
			cacheTTL1h:                       cacheTTL1h,
			rewriteMessageCacheControl:       rewriteMessageCacheControl,
			clientDatelineNormalization:      clientDatelineNormalization,
		}, nil
	})
	if r, ok := val.(gatewayForwardingSettingsResult); ok {
		return r
	}
	return gatewayForwardingSettingsResult{fp: true, claudeOAuthSystemPromptInjection: true, clientDatelineNormalization: true}
}

// GetOpenAITTFTMode 返回 Responses first_token_ms 的统计口径。
func (s *SettingService) GetOpenAITTFTMode(ctx context.Context) string {
	return s.getGatewayForwardingSettingsCached(ctx).openAITTFTMode
}

// GetGatewayForwardingSettings returns cached gateway forwarding settings.
// Uses in-process atomic.Value cache with 60s TTL, zero-lock hot path.
// Returns (fingerprintUnification, metadataPassthrough, cchSigning).
func (s *SettingService) GetGatewayForwardingSettings(ctx context.Context) (fingerprintUnification, metadataPassthrough, cchSigning bool) {
	result := s.getGatewayForwardingSettingsCached(ctx)
	return result.fp, result.mp, result.cch
}

// IsAnthropicCacheTTL1hInjectionEnabled 检查是否对 Anthropic OAuth/SetupToken 请求体注入 1h cache_control ttl。
func (s *SettingService) IsAnthropicCacheTTL1hInjectionEnabled(ctx context.Context) bool {
	return s.getGatewayForwardingSettingsCached(ctx).cacheTTL1h
}

// IsRewriteMessageCacheControlEnabled 检查是否启用 messages cache_control 改写。
func (s *SettingService) IsRewriteMessageCacheControlEnabled(ctx context.Context) bool {
	return s.getGatewayForwardingSettingsCached(ctx).rewriteMessageCacheControl
}

// IsClientDatelineNormalizationEnabled 检查是否启用 Anthropic OAuth/SetupToken 请求体
// 的客户端 dateline 归一化。默认开启。
func (s *SettingService) IsClientDatelineNormalizationEnabled(ctx context.Context) bool {
	return s.getGatewayForwardingSettingsCached(ctx).clientDatelineNormalization
}

// GetClaudeOAuthSystemPromptInjectionSettings returns the Claude OAuth mimic
// system block switch, legacy custom expansion prompt, and configurable blocks JSON.
// Empty values mean use the built-in Claude Code default blocks.
func (s *SettingService) GetClaudeOAuthSystemPromptInjectionSettings(ctx context.Context) (enabled bool, prompt string, blocks string) {
	result := s.getGatewayForwardingSettingsCached(ctx)
	return result.claudeOAuthSystemPromptInjection, result.claudeOAuthSystemPrompt, result.claudeOAuthSystemPromptBlocks
}

// GetClaudeCodeVersionBounds 获取 Claude Code 版本号上下限要求
// 使用进程内 atomic.Value 缓存，60 秒 TTL，热路径零锁开销
// singleflight 防止缓存过期时 thundering herd
// 返回空字符串表示不做对应方向的版本检查
func (s *SettingService) GetClaudeCodeVersionBounds(ctx context.Context) (min, max string) {
	if cached, ok := versionBoundsCache.Load().(*cachedVersionBounds); ok {
		if time.Now().UnixNano() < cached.expiresAt {
			return cached.min, cached.max
		}
	}
	// singleflight: 同一时刻只有一个 goroutine 查询 DB，其余复用结果
	type bounds struct{ min, max string }
	result, err, _ := versionBoundsSF.Do("version_bounds", func() (any, error) {
		// 二次检查，避免排队的 goroutine 重复查询
		if cached, ok := versionBoundsCache.Load().(*cachedVersionBounds); ok {
			if time.Now().UnixNano() < cached.expiresAt {
				return bounds{cached.min, cached.max}, nil
			}
		}
		// 使用独立 context：断开请求取消链，避免客户端断连导致空值被长期缓存
		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), versionBoundsDBTimeout)
		defer cancel()
		values, err := s.settingRepo.GetMultiple(dbCtx, []string{
			SettingKeyMinClaudeCodeVersion,
			SettingKeyMaxClaudeCodeVersion,
		})
		if err != nil {
			// fail-open: DB 错误时不阻塞请求，但记录日志并使用短 TTL 快速重试
			slog.Warn("failed to get claude code version bounds setting, skipping version check", "error", err)
			versionBoundsCache.Store(&cachedVersionBounds{
				min:       "",
				max:       "",
				expiresAt: time.Now().Add(versionBoundsErrorTTL).UnixNano(),
			})
			return bounds{"", ""}, nil
		}
		b := bounds{
			min: values[SettingKeyMinClaudeCodeVersion],
			max: values[SettingKeyMaxClaudeCodeVersion],
		}
		versionBoundsCache.Store(&cachedVersionBounds{
			min:       b.min,
			max:       b.max,
			expiresAt: time.Now().Add(versionBoundsCacheTTL).UnixNano(),
		})
		return b, nil
	})
	if err != nil {
		return "", ""
	}
	b, ok := result.(bounds)
	if !ok {
		return "", ""
	}
	return b.min, b.max
}

// GetOpenAIQuotaAutoPauseSettings returns the current global default quota auto-pause
// settings. It is invoked on the OpenAI scheduling hot path (once per request) and is
// therefore designed to never block on the DB:
//
//   - Fresh cached value → returned immediately.
//   - Stale or empty cache → the last known value is returned, and a background
//     goroutine refreshes the cache via singleflight (stale-while-revalidate).
//   - First call with no cache yet → zero defaults are returned and the same async
//     refresh is kicked off; the next call gets the freshly populated value.
//
// Callers that need the freshly persisted value synchronously (tests, post-update
// confirmation, optional startup warm-up) should call WarmOpenAIQuotaAutoPauseSettings.
func (s *SettingService) GetOpenAIQuotaAutoPauseSettings(ctx context.Context) OpsOpenAIAccountQuotaAutoPauseSettings {
	if s == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	cached, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings)
	now := time.Now().UnixNano()
	if cached != nil && now < cached.expiresAt {
		return cached.settings
	}
	// Stale or unset: trigger background refresh without blocking this request.
	// singleflight.DoChan dedupes concurrent refreshes; we deliberately ignore the
	// returned channel — the result is observable via the atomic cache.
	s.openAIQuotaAutoPauseSettingsSF.DoChan(openAIQuotaAutoPauseSettingsRefreshKey, func() (any, error) {
		s.refreshOpenAIQuotaAutoPauseSettings(context.Background())
		return nil, nil
	})
	if cached != nil {
		return cached.settings // serve stale value while revalidating
	}
	return OpsOpenAIAccountQuotaAutoPauseSettings{}
}

// WarmOpenAIQuotaAutoPauseSettings synchronously loads the quota auto-pause settings
// into the in-memory cache. Useful for application startup (so the first request hits
// a warm cache) and for tests that need deterministic reads immediately after
// constructing the service.
func (s *SettingService) WarmOpenAIQuotaAutoPauseSettings(ctx context.Context) OpsOpenAIAccountQuotaAutoPauseSettings {
	if s == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	s.refreshOpenAIQuotaAutoPauseSettings(ctx)
	cached, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings)
	if cached == nil {
		return OpsOpenAIAccountQuotaAutoPauseSettings{}
	}
	return cached.settings
}

// refreshOpenAIQuotaAutoPauseSettings reads the latest settings from the DB and stores
// them into the in-memory cache. On error it stores the prior value (or zero defaults
// if nothing is cached yet) with the shorter error TTL so the next refresh comes
// sooner. Always uses its own timeout-bounded context to keep refresh latency
// predictable regardless of the caller.
func (s *SettingService) refreshOpenAIQuotaAutoPauseSettings(ctx context.Context) {
	if s == nil || s.settingRepo == nil {
		return
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), openAIQuotaAutoPauseSettingsDBTimeout)
	defer cancel()

	settings := OpsOpenAIAccountQuotaAutoPauseSettings{}
	ttl := openAIQuotaAutoPauseSettingsCacheTTL
	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyOpsAdvancedSettings)
	if err == nil {
		cfg := defaultOpsAdvancedSettings()
		if strings.TrimSpace(raw) != "" {
			if jsonErr := json.Unmarshal([]byte(raw), cfg); jsonErr == nil {
				normalizeOpsAdvancedSettings(cfg)
			}
		}
		settings = cfg.OpenAIAccountQuotaAutoPause
	} else if !errors.Is(err, ErrSettingNotFound) {
		// Real error: keep serving prior value but refresh sooner.
		if prior, _ := s.openAIQuotaAutoPauseSettingsCache.Load().(*cachedOpenAIQuotaAutoPauseSettings); prior != nil {
			settings = prior.settings
		}
		ttl = openAIQuotaAutoPauseSettingsErrorTTL
	}

	s.openAIQuotaAutoPauseSettingsCache.Store(&cachedOpenAIQuotaAutoPauseSettings{
		settings:  settings,
		expiresAt: time.Now().Add(ttl).UnixNano(),
	})
}

// SetOpenAIQuotaAutoPauseSettings writes the given settings directly into the in-memory
// cache. Called from settings-write code paths so that the next read reflects the new
// value immediately, without waiting for the background refresh.
func (s *SettingService) SetOpenAIQuotaAutoPauseSettings(settings OpsOpenAIAccountQuotaAutoPauseSettings) {
	if s == nil {
		return
	}
	s.openAIQuotaAutoPauseSettingsCache.Store(&cachedOpenAIQuotaAutoPauseSettings{
		settings:  settings,
		expiresAt: time.Now().Add(openAIQuotaAutoPauseSettingsCacheTTL).UnixNano(),
	})
}
