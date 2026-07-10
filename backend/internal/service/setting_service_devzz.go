package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const (
	modelSelfCheckIntervalMin               = 60
	modelSelfCheckIntervalMax               = 86400
	modelSelfCheckIntervalFallback          = 300
	modelSelfCheckConcurrencyMin            = 1
	modelSelfCheckConcurrencyMax            = 64
	modelSelfCheckConcurrencyFallback       = 4
	modelSelfCheckMaxTasksMin               = 1
	modelSelfCheckMaxTasksMax               = 10000
	modelSelfCheckMaxTasksFallback          = 500
	modelSelfCheckSnapshotRetentionMin      = 30
	modelSelfCheckSnapshotRetentionMax      = 3650
	modelSelfCheckSnapshotRetentionFallback = 90

	scheduleStrategyCacheTTL  = 60 * time.Second
	scheduleStrategyErrorTTL  = 5 * time.Second
	scheduleStrategyDBTimeout = 5 * time.Second
)

type cachedScheduleStrategy struct {
	value     string
	expiresAt int64
}

type ModelSelfCheckRuntime struct {
	Enabled                bool
	DefaultIntervalSeconds int
	MaxConcurrency         int
	MaxTasksPerRound       int
	SnapshotRetentionDays  int
}

func parseModelSelfCheckInterval(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return modelSelfCheckIntervalFallback
	}
	return clampModelSelfCheckInterval(v)
}

func clampModelSelfCheckInterval(v int) int {
	if v <= 0 {
		return 0
	}
	if v < modelSelfCheckIntervalMin {
		return modelSelfCheckIntervalMin
	}
	if v > modelSelfCheckIntervalMax {
		return modelSelfCheckIntervalMax
	}
	return v
}

func parseModelSelfCheckMaxConcurrency(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return modelSelfCheckConcurrencyFallback
	}
	return clampModelSelfCheckMaxConcurrency(v)
}

func clampModelSelfCheckMaxConcurrency(v int) int {
	if v <= 0 {
		return modelSelfCheckConcurrencyFallback
	}
	if v < modelSelfCheckConcurrencyMin {
		return modelSelfCheckConcurrencyMin
	}
	if v > modelSelfCheckConcurrencyMax {
		return modelSelfCheckConcurrencyMax
	}
	return v
}

func parseModelSelfCheckMaxTasksPerRound(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return modelSelfCheckMaxTasksFallback
	}
	return clampModelSelfCheckMaxTasksPerRound(v)
}

func clampModelSelfCheckMaxTasksPerRound(v int) int {
	if v <= 0 {
		return modelSelfCheckMaxTasksFallback
	}
	if v < modelSelfCheckMaxTasksMin {
		return modelSelfCheckMaxTasksMin
	}
	if v > modelSelfCheckMaxTasksMax {
		return modelSelfCheckMaxTasksMax
	}
	return v
}

func parseModelSelfCheckSnapshotRetentionDays(raw string) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return modelSelfCheckSnapshotRetentionFallback
	}
	return clampModelSelfCheckSnapshotRetentionDays(v)
}

func clampModelSelfCheckSnapshotRetentionDays(v int) int {
	if v == 0 {
		return 0
	}
	if v < 0 {
		return modelSelfCheckSnapshotRetentionFallback
	}
	if v < modelSelfCheckSnapshotRetentionMin {
		return modelSelfCheckSnapshotRetentionMin
	}
	if v > modelSelfCheckSnapshotRetentionMax {
		return modelSelfCheckSnapshotRetentionMax
	}
	return v
}

func (s *SettingService) GetModelSelfCheckRuntime(ctx context.Context) ModelSelfCheckRuntime {
	vals, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyModelSelfCheckEnabled,
		SettingKeyModelSelfCheckDefaultIntervalSeconds,
		SettingKeyModelSelfCheckMaxConcurrency,
		SettingKeyModelSelfCheckMaxTasksPerRound,
		SettingKeyModelSelfCheckSnapshotRetentionDays,
	})
	if err != nil {
		return ModelSelfCheckRuntime{
			Enabled:                true,
			DefaultIntervalSeconds: modelSelfCheckIntervalFallback,
			MaxConcurrency:         modelSelfCheckConcurrencyFallback,
			MaxTasksPerRound:       modelSelfCheckMaxTasksFallback,
			SnapshotRetentionDays:  modelSelfCheckSnapshotRetentionFallback,
		}
	}
	return ModelSelfCheckRuntime{
		Enabled:                !isFalseSettingValue(vals[SettingKeyModelSelfCheckEnabled]),
		DefaultIntervalSeconds: parseModelSelfCheckInterval(vals[SettingKeyModelSelfCheckDefaultIntervalSeconds]),
		MaxConcurrency:         parseModelSelfCheckMaxConcurrency(vals[SettingKeyModelSelfCheckMaxConcurrency]),
		MaxTasksPerRound:       parseModelSelfCheckMaxTasksPerRound(vals[SettingKeyModelSelfCheckMaxTasksPerRound]),
		SnapshotRetentionDays:  parseModelSelfCheckSnapshotRetentionDays(vals[SettingKeyModelSelfCheckSnapshotRetentionDays]),
	}
}

func (s *SettingService) IsDisableKeysOnRateChangeEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	vals, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyDisableKeysOnRateChange})
	if err != nil {
		slog.Warn("failed to get disable_keys_on_rate_change setting, defaulting to false", "error", err)
		return false
	}
	return vals[SettingKeyDisableKeysOnRateChange] == "true"
}

func (s *SettingService) GetAPIKeyBatchCreateMaxCount(ctx context.Context) int {
	if s == nil || s.settingRepo == nil {
		return DefaultAPIKeyBatchCreateMaxCount
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAPIKeyBatchCreateMaxCount)
	if err != nil || value == "" {
		return DefaultAPIKeyBatchCreateMaxCount
	}
	v, err := strconv.Atoi(value)
	if err != nil || v <= 0 {
		return DefaultAPIKeyBatchCreateMaxCount
	}
	if v > HardAPIKeyBatchCreateMaxCount {
		return HardAPIKeyBatchCreateMaxCount
	}
	return v
}

func (s *SettingService) GetModelRateLimitSettings(ctx context.Context) (*ModelRateLimitSettings, error) {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyModelRateLimitSettings)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return DefaultModelRateLimitSettings(), nil
		}
		return nil, fmt.Errorf("get model rate limit settings: %w", err)
	}
	if value == "" {
		return DefaultModelRateLimitSettings(), nil
	}

	var settings ModelRateLimitSettings
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultModelRateLimitSettings(), nil
	}

	clampModelRateLimitSettings(&settings)
	return &settings, nil
}

func (s *SettingService) SetModelRateLimitSettings(ctx context.Context, settings *ModelRateLimitSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}

	if settings.Enabled {
		if settings.FailureThreshold < 1 || settings.FailureThreshold > 100 {
			return fmt.Errorf("failure_threshold must be between 1-100")
		}
		if settings.WindowMinutes < 1 || settings.WindowMinutes > 1440 {
			return fmt.Errorf("window_minutes must be between 1-1440")
		}
		if settings.CooldownSeconds < 1 || settings.CooldownSeconds > 7200 {
			return fmt.Errorf("cooldown_seconds must be between 1-7200")
		}
	} else {
		// 关闭时归一化为默认值，避免存入越界数据。
		clampModelRateLimitSettings(settings)
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal model rate limit settings: %w", err)
	}

	return s.settingRepo.Set(ctx, SettingKeyModelRateLimitSettings, string(data))
}

func clampModelRateLimitSettings(settings *ModelRateLimitSettings) {
	def := DefaultModelRateLimitSettings()
	if settings.FailureThreshold < 1 {
		settings.FailureThreshold = def.FailureThreshold
	} else if settings.FailureThreshold > 100 {
		settings.FailureThreshold = 100
	}
	if settings.WindowMinutes < 1 {
		settings.WindowMinutes = def.WindowMinutes
	} else if settings.WindowMinutes > 1440 {
		settings.WindowMinutes = 1440
	}
	if settings.CooldownSeconds < 1 {
		settings.CooldownSeconds = def.CooldownSeconds
	} else if settings.CooldownSeconds > 7200 {
		settings.CooldownSeconds = 7200
	}
}

func (s *SettingService) GetScheduleStrategy(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return ScheduleStrategyStrictPriority
	}
	if cached, ok := s.scheduleStrategyCache.Load().(*cachedScheduleStrategy); ok {
		if time.Now().UnixNano() < cached.expiresAt {
			return NormalizeScheduleStrategy(cached.value)
		}
	}

	result, _, _ := s.scheduleStrategySF.Do("schedule_strategy", func() (any, error) {
		if cached, ok := s.scheduleStrategyCache.Load().(*cachedScheduleStrategy); ok {
			if time.Now().UnixNano() < cached.expiresAt {
				return NormalizeScheduleStrategy(cached.value), nil
			}
		}

		dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), scheduleStrategyDBTimeout)
		defer cancel()
		value, err := s.settingRepo.GetValue(dbCtx, SettingKeyScheduleStrategy)
		if err != nil && !errors.Is(err, ErrSettingNotFound) {
			slog.Warn("failed to get schedule strategy setting, using strict priority", "error", err)
			s.scheduleStrategyCache.Store(&cachedScheduleStrategy{
				value:     ScheduleStrategyStrictPriority,
				expiresAt: time.Now().Add(scheduleStrategyErrorTTL).UnixNano(),
			})
			return ScheduleStrategyStrictPriority, nil
		}

		strategy := NormalizeScheduleStrategy(value)
		s.scheduleStrategyCache.Store(&cachedScheduleStrategy{
			value:     strategy,
			expiresAt: time.Now().Add(scheduleStrategyCacheTTL).UnixNano(),
		})
		return strategy, nil
	})

	if strategy, ok := result.(string); ok {
		return NormalizeScheduleStrategy(strategy)
	}
	return ScheduleStrategyStrictPriority
}
