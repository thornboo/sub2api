//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type catalogRuntimeRepositoryStub struct {
	rows []service.ModelRuntimeBucket
	err  error
}

func (r catalogRuntimeRepositoryStub) AggregateModelRuntime(context.Context, time.Time, time.Time) ([]service.ModelRuntimeBucket, error) {
	return r.rows, r.err
}

func TestCatalogRuntimeEnrichmentUsesOnlyPublishedVisibleGroups(t *testing.T) {
	runtime := service.NewModelRuntimeService(catalogRuntimeRepositoryStub{rows: []service.ModelRuntimeBucket{
		{ModelRuntimeKey: service.ModelRuntimeKey{GroupID: 7, Model: "model"}, Hour: 23, Successes: 9, Failures: 1},
		{ModelRuntimeKey: service.ModelRuntimeKey{GroupID: 9, Model: "model"}, Hour: 23, Successes: 1, Failures: 9},
		{ModelRuntimeKey: service.ModelRuntimeKey{GroupID: 11, Model: "model"}, Hour: 23, Successes: 12345},
	}})
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "callable catalog", true: "configured catalog"}[configured], func(t *testing.T) {
			model := userSupportedModel{Name: "model", RouteGroupIDs: []int64{7, 11},
				Pricing:      &userSupportedModelPricing{BillingMode: "token"},
				GroupPricing: []userGroupModelPricing{{GroupID: 9, Pricing: &userSupportedModelPricing{BillingMode: "image"}}},
			}
			if configured {
				model.CatalogGroupIDs = []int64{7, 9, 11}
			}
			channels := []userAvailableChannel{{Platforms: []userChannelPlatformSection{{
				Groups: []userAvailableGroup{{ID: 7}, {ID: 9}}, SupportedModels: []userSupportedModel{model},
			}}}}
			attachModelRuntimeMetrics(context.Background(), runtime, channels)
			metrics := channels[0].Platforms[0].SupportedModels[0].RuntimeMetrics
			require.Equal(t, int64(7), metrics[0].GroupID)
			require.Equal(t, 0.9, *metrics[0].Metrics.SuccessRate)
			require.Equal(t, "firstToken", metrics[0].Metrics.LatencyKind)
			require.Len(t, metrics[0].Metrics.Hours, 24)
			if configured {
				require.Len(t, metrics, 2)
				require.Equal(t, int64(9), metrics[1].GroupID)
				require.Equal(t, 0.1, *metrics[1].Metrics.SuccessRate)
				require.Equal(t, "generation", metrics[1].Metrics.LatencyKind)
			} else {
				require.Len(t, metrics, 1)
			}
			raw, err := json.Marshal(channels)
			require.NoError(t, err)
			require.NotContains(t, string(raw), `"group_id":11`)
			require.NotContains(t, string(raw), "12345")
		})
	}
}

func TestCatalogRuntimeFailurePreservesCatalogAndSchedulingState(t *testing.T) {
	none := []int64{}
	channels := []userAvailableChannel{{Name: "configured", Platforms: []userChannelPlatformSection{{
		Groups:          []userAvailableGroup{{ID: 7}},
		SupportedModels: []userSupportedModel{{Name: "model", CatalogGroupIDs: []int64{7}, SchedulableGroupIDs: &none}},
	}}}}
	runtime := service.NewModelRuntimeService(catalogRuntimeRepositoryStub{err: errors.New("statistics unavailable")})
	attachModelRuntimeMetrics(context.Background(), runtime, channels)
	raw, err := json.Marshal(channels)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"name":"configured"`)
	require.Contains(t, string(raw), `"schedulable_group_ids":[]`)
	require.NotContains(t, string(raw), "runtime_metrics")
}

func TestCatalogSchedulingMetadataSeparatesNoAccountFromUnknownEvidence(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		groups := &availableDeliveryGroupRepoStub{groups: []service.Group{{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive}}, accountIDs: []int64{70}}
		reset := time.Now().Add(time.Hour)
		accounts := &availableDeliveryAccountRepoStub{accounts: []*service.Account{{
			ID: 70, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: enabled, GroupIDs: []int64{7}, RateLimitResetAt: &reset,
		}}}
		cfg := &config.Config{}
		cfg.Gateway.NativeModelProtocolRoutingEnabled = true
		capability := service.NewModelProtocolCapabilityService(&availableDeliveryCapabilityRepoStub{}, accounts, groups, nil, cfg)
		delivery := service.NewModelDeliveryService(accounts, groups, nil, capability, cfg)
		out, err := buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), delivery, configuredCatalogVisibleGroupsForTest, catalogModelsConfigured)
		require.NoError(t, err)
		model := out[0].Platforms[0].SupportedModels[0]
		require.NotNil(t, model.SchedulableGroupIDs)
		require.Empty(t, model.RouteGroupIDs, "unknown protocol evidence is different from absent scheduling accounts")
		if enabled {
			require.Equal(t, []int64{7}, *model.SchedulableGroupIDs, "temporary rate limit does not erase scheduling configuration")
		} else {
			require.Empty(t, *model.SchedulableGroupIDs)
		}
	}
	out, err := buildAvailableChannelCatalog(context.Background(), configuredCatalogChannelsForTest(), nil, configuredCatalogVisibleGroupsForTest, catalogModelsConfigured)
	require.NoError(t, err)
	require.Nil(t, out[0].Platforms[0].SupportedModels[0].SchedulableGroupIDs, "missing metadata is not zero accounts")
}
