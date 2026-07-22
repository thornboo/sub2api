package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type nativeModelProtocolRoutingSettingRepoStub struct {
	value   string
	getErr  error
	all     map[string]string
	updates map[string]string
}

type nativeModelProtocolRoutingSettingReaderStub bool

func (s nativeModelProtocolRoutingSettingReaderStub) IsNativeModelProtocolRoutingEnabled(context.Context) bool {
	return bool(s)
}

func (s *nativeModelProtocolRoutingSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (s *nativeModelProtocolRoutingSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key != SettingKeyNativeModelProtocolRoutingEnabled {
		return "", ErrSettingNotFound
	}
	return s.value, s.getErr
}

func (s *nativeModelProtocolRoutingSettingRepoStub) Set(_ context.Context, key, value string) error {
	if s.updates == nil {
		s.updates = make(map[string]string)
	}
	s.updates[key] = value
	return nil
}

func (s *nativeModelProtocolRoutingSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *nativeModelProtocolRoutingSettingRepoStub) SetMultiple(_ context.Context, updates map[string]string) error {
	if s.updates == nil {
		s.updates = make(map[string]string)
	}
	for key, value := range updates {
		s.updates[key] = value
	}
	return nil
}

func (s *nativeModelProtocolRoutingSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	if s.all == nil {
		return map[string]string{}, nil
	}
	return s.all, nil
}

func (s *nativeModelProtocolRoutingSettingRepoStub) Delete(context.Context, string) error {
	return nil
}

func TestNativeModelProtocolRoutingRuntimeFallsBackToConfigWhenSettingMissing(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	repo := &nativeModelProtocolRoutingSettingRepoStub{getErr: ErrSettingNotFound}
	svc := NewSettingService(repo, cfg)

	runtime := svc.GetNativeModelProtocolRoutingRuntime(context.Background())

	require.True(t, runtime.Enabled)
	require.Equal(t, "config", runtime.Source)
	require.True(t, svc.IsNativeModelProtocolRoutingEnabled(context.Background()))
}

func TestNativeModelProtocolRoutingRuntimeDatabaseOverrideWins(t *testing.T) {
	tests := []struct {
		name        string
		configValue bool
		storedValue string
		want        bool
	}{
		{name: "database enables a disabled deployment default", storedValue: "true", want: true},
		{name: "database disables an enabled deployment default", configValue: true, storedValue: "false", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Gateway.NativeModelProtocolRoutingEnabled = tt.configValue
			svc := NewSettingService(&nativeModelProtocolRoutingSettingRepoStub{value: tt.storedValue}, cfg)

			runtime := svc.GetNativeModelProtocolRoutingRuntime(context.Background())

			require.Equal(t, tt.want, runtime.Enabled)
			require.Equal(t, "settings", runtime.Source)
		})
	}
}

func TestNativeModelProtocolRoutingRuntimeDatabaseErrorFailsClosedWithoutKnownValue(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	svc := NewSettingService(&nativeModelProtocolRoutingSettingRepoStub{getErr: errors.New("database unavailable")}, cfg)

	runtime := svc.GetNativeModelProtocolRoutingRuntime(context.Background())

	require.False(t, runtime.Enabled)
	require.Equal(t, "error_fallback", runtime.Source)
}

func TestNativeModelProtocolRoutingRuntimeDatabaseErrorPreservesLastKnownOverride(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	svc := NewSettingService(&nativeModelProtocolRoutingSettingRepoStub{getErr: errors.New("database unavailable")}, cfg)
	svc.nativeModelProtocolRoutingCache.Store(&cachedNativeModelProtocolRouting{
		enabled:   false,
		source:    "settings",
		expiresAt: time.Now().Add(-time.Second).UnixNano(),
	})

	runtime := svc.GetNativeModelProtocolRoutingRuntime(context.Background())

	require.False(t, runtime.Enabled)
	require.Equal(t, "settings", runtime.Source)
}

func TestNativeModelProtocolRoutingConsumerUsesRuntimeReaderBeforeConfig(t *testing.T) {
	cfgEnabled := &config.Config{}
	cfgEnabled.Gateway.NativeModelProtocolRoutingEnabled = true

	require.False(t, nativeModelProtocolRoutingEnabled(
		context.Background(),
		nativeModelProtocolRoutingSettingReaderStub(false),
		cfgEnabled,
	))
	require.True(t, nativeModelProtocolRoutingEnabled(
		context.Background(),
		nativeModelProtocolRoutingSettingReaderStub(true),
		&config.Config{},
	))
	require.True(t, nativeModelProtocolRoutingEnabled(context.Background(), nil, cfgEnabled))
}

func TestParseSettingsReportsNativeModelProtocolRoutingSource(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	svc := NewSettingService(&nativeModelProtocolRoutingSettingRepoStub{}, cfg)

	fromConfig := svc.parseSettings(map[string]string{})
	require.True(t, fromConfig.NativeModelProtocolRoutingEnabled)
	require.Equal(t, "config", fromConfig.NativeModelProtocolRoutingSource)

	fromSettings := svc.parseSettings(map[string]string{
		SettingKeyNativeModelProtocolRoutingEnabled: "false",
	})
	require.False(t, fromSettings.NativeModelProtocolRoutingEnabled)
	require.Equal(t, "settings", fromSettings.NativeModelProtocolRoutingSource)
}

func TestBuildSystemSettingsUpdatesPersistsNativeModelProtocolRoutingOverride(t *testing.T) {
	svc := NewSettingService(&nativeModelProtocolRoutingSettingRepoStub{}, &config.Config{})

	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		NativeModelProtocolRoutingEnabled: true,
	})

	require.NoError(t, err)
	require.Equal(t, "true", updates[SettingKeyNativeModelProtocolRoutingEnabled])
}

func TestBuildSystemSettingsUpdatesDoesNotSolidifyConfigFallback(t *testing.T) {
	svc := NewSettingService(&nativeModelProtocolRoutingSettingRepoStub{}, &config.Config{})

	updates, err := svc.buildSystemSettingsUpdates(context.Background(), &SystemSettings{
		NativeModelProtocolRoutingEnabled: true,
		NativeModelProtocolRoutingSource:  "config",
	})

	require.NoError(t, err)
	_, exists := updates[SettingKeyNativeModelProtocolRoutingEnabled]
	require.False(t, exists)
}
