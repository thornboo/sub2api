package handler

import (
	"context"
	"log/slog"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Enrichment runs after catalog authorization and publication filtering. The
// service cache may contain other groups; only keys in this response are copied.
func attachModelRuntimeMetrics(ctx context.Context, runtime *service.ModelRuntimeService, channels []userAvailableChannel) {
	if runtime == nil {
		return
	}
	requests := make(map[service.ModelRuntimeKey]service.ModelRuntimeRequest)
	for _, channel := range channels {
		for _, section := range channel.Platforms {
			for _, model := range section.SupportedModels {
				for _, groupID := range modelRuntimeVisibleGroupIDs(section, model) {
					key := service.ModelRuntimeKey{GroupID: groupID, Model: model.Name}
					pricing := model.Pricing
					for _, groupPricing := range model.GroupPricing {
						if groupPricing.GroupID == groupID && groupPricing.Pricing != nil {
							pricing = groupPricing.Pricing
							break
						}
					}
					request := service.ModelRuntimeRequest{ModelRuntimeKey: key}
					if pricing != nil {
						request.BillingMode = service.BillingMode(pricing.BillingMode)
					}
					requests[key] = request
				}
			}
		}
	}
	batch := make([]service.ModelRuntimeRequest, 0, len(requests))
	for _, request := range requests {
		batch = append(batch, request)
	}
	sort.Slice(batch, func(i, j int) bool {
		if batch[i].GroupID != batch[j].GroupID {
			return batch[i].GroupID < batch[j].GroupID
		}
		return batch[i].Model < batch[j].Model
	})
	metrics, err := runtime.GetForModels(ctx, batch)
	if err != nil {
		slog.WarnContext(ctx, "catalog_runtime_metrics_unavailable", "error", err)
		return
	}
	for i := range channels {
		for j := range channels[i].Platforms {
			section := &channels[i].Platforms[j]
			for k := range section.SupportedModels {
				model := &section.SupportedModels[k]
				for _, groupID := range modelRuntimeVisibleGroupIDs(*section, *model) {
					if metric, ok := metrics[service.ModelRuntimeKey{GroupID: groupID, Model: model.Name}]; ok {
						model.RuntimeMetrics = append(model.RuntimeMetrics, userGroupModelRuntime{GroupID: groupID, Metrics: metric})
					}
				}
			}
		}
	}
}

func modelRuntimeVisibleGroupIDs(section userChannelPlatformSection, model userSupportedModel) []int64 {
	visible := make(map[int64]struct{}, len(section.Groups))
	for _, group := range section.Groups {
		visible[group.ID] = struct{}{}
	}
	ids := model.CatalogGroupIDs
	if len(ids) == 0 {
		ids = model.RouteGroupIDs
	}
	return intersectVisibleGroupIDs(ids, visible)
}
