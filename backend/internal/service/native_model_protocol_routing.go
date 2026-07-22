package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// NativeModelProtocolRoutingSettingReader keeps the global gate independent
// from the concrete settings service and makes all routing consumers share the
// same effective-value contract.
type NativeModelProtocolRoutingSettingReader interface {
	IsNativeModelProtocolRoutingEnabled(ctx context.Context) bool
}

func nativeModelProtocolRoutingEnabled(
	ctx context.Context,
	settings NativeModelProtocolRoutingSettingReader,
	cfg *config.Config,
) bool {
	if settings != nil {
		return settings.IsNativeModelProtocolRoutingEnabled(ctx)
	}
	return cfg != nil && cfg.Gateway.NativeModelProtocolRoutingEnabled
}
