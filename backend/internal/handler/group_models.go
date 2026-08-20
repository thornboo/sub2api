package handler

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"go.uber.org/zap"
)

// configuredModelsForGroup resolves the models a discovery surface may
// advertise for one group. An empty persistently enabled account pool is not
// equivalent to an enabled account without model_mapping: only the latter may
// use platform defaults.
func (h *GatewayHandler) configuredModelsForGroup(ctx context.Context, group *service.Group) []string {
	return h.configuredModelsForGroupWithCacheMode(ctx, group, true)
}

func (h *GatewayHandler) configuredModelsForGroupFresh(ctx context.Context, group *service.Group) []string {
	return h.configuredModelsForGroupWithCacheMode(ctx, group, false)
}

func (h *GatewayHandler) configuredModelsForGroupWithCacheMode(
	ctx context.Context,
	group *service.Group,
	useCache bool,
) []string {
	if h == nil || h.gatewayService == nil || group == nil {
		return []string{}
	}

	if group.Platform == service.PlatformComposite {
		available := h.configuredCompositeModels(ctx, &group.ID, useCache)
		if len(available) == 0 {
			return []string{}
		}
		if group.CustomModelsListEnabled() {
			available = filterModelsByCustomList(
				available,
				defaultModelIDsForPlatform(service.PlatformComposite),
				group.ModelsListConfig.Models,
			)
		}
		return normalizeConfiguredModels(available)
	}

	var available []string
	var hasAccounts bool
	var err error
	if useCache {
		available, hasAccounts, err = h.gatewayService.GetConfiguredGroupModels(ctx, &group.ID, group.Platform)
	} else {
		available, hasAccounts, err = h.gatewayService.GetConfiguredGroupModelsFresh(ctx, &group.ID, group.Platform)
	}
	if err != nil {
		logger.FromContext(ctx).Warn("gateway.configured_group_models_lookup_failed",
			zap.Int64("group_id", group.ID),
			zap.String("platform", group.Platform),
			zap.Error(err),
		)
		return []string{}
	}
	if !hasAccounts {
		return []string{}
	}

	fallback := defaultModelIDsForPlatform(group.Platform)
	if group.CustomModelsListEnabled() {
		available = filterModelsByCustomList(
			customModelsListSource(group.Platform, available, fallback),
			fallback,
			group.ModelsListConfig.Models,
		)
	} else if len(available) == 0 {
		available = fallback
	}
	return normalizeConfiguredModels(available)
}

func (h *GatewayHandler) configuredCompositeModels(ctx context.Context, groupID *int64, useCache bool) []string {
	if h == nil || h.gatewayService == nil {
		return []string{}
	}

	seen := make(map[string]struct{})
	models := make([]string, 0)
	configuredPlatforms := make(map[string]struct{})
	for _, platform := range []string{
		service.PlatformAnthropic,
		service.PlatformGemini,
		service.PlatformOpenAI,
		service.PlatformAntigravity,
		service.PlatformGrok,
		service.PlatformKimi,
		service.PlatformZhipu,
		service.PlatformDeepseek,
	} {
		var platformModels []string
		var hasAccounts bool
		var err error
		if useCache {
			platformModels, hasAccounts, err = h.gatewayService.GetConfiguredGroupModels(ctx, groupID, platform)
		} else {
			platformModels, hasAccounts, err = h.gatewayService.GetConfiguredGroupModelsFresh(ctx, groupID, platform)
		}
		if err != nil {
			logger.FromContext(ctx).Warn("gateway.configured_composite_models_lookup_failed",
				zap.Int64("group_id", dereferenceGroupID(groupID)),
				zap.String("platform", platform),
				zap.Error(err),
			)
			continue
		}
		if !hasAccounts {
			continue
		}
		configuredPlatforms[platform] = struct{}{}
		// CN providers have no trustworthy static default catalog; expose only
		// model-mapping keys discovered from configured accounts.
		if len(platformModels) == 0 && !service.IsCNProvider(platform) {
			platformModels = defaultModelIDsForPlatform(platform)
		}
		models = appendNormalizedModels(models, seen, platformModels)
	}

	if groupID == nil {
		return models
	}
	routes, err := h.gatewayService.CompositeCatalogRoutes(ctx, *groupID)
	if err != nil {
		logger.FromContext(ctx).Warn("gateway.composite_catalog_routes_failed", zap.Int64("group_id", *groupID), zap.Error(err))
		return models
	}
	for _, route := range routes {
		if _, ok := configuredPlatforms[strings.TrimSpace(route.TargetPlatform)]; !ok {
			continue
		}
		models = appendNormalizedModels(models, seen, []string{route.PublicModel})
	}
	return models
}

func appendNormalizedModels(models []string, seen map[string]struct{}, candidates []string) []string {
	for _, model := range candidates {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		models = append(models, model)
	}
	return models
}

func dereferenceGroupID(groupID *int64) int64 {
	if groupID == nil {
		return 0
	}
	return *groupID
}

func normalizeConfiguredModels(available []string) []string {
	return appendNormalizedModels(make([]string, 0, len(available)), make(map[string]struct{}, len(available)), available)
}
