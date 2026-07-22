package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type channelModelDeliveryChannelRepoStub struct {
	service.ChannelRepository
	channel *service.Channel
}

func (r *channelModelDeliveryChannelRepoStub) GetByID(_ context.Context, id int64) (*service.Channel, error) {
	if r.channel == nil || r.channel.ID != id {
		return nil, service.ErrChannelNotFound
	}
	return r.channel.Clone(), nil
}

type channelModelDeliveryGroupRepoStub struct {
	service.GroupRepository
	groups     []service.Group
	accountIDs []int64
}

func (r *channelModelDeliveryGroupRepoStub) ListActive(_ context.Context) ([]service.Group, error) {
	return append([]service.Group(nil), r.groups...), nil
}

func (r *channelModelDeliveryGroupRepoStub) GetAccountIDsByGroupIDs(_ context.Context, _ []int64) ([]int64, error) {
	return append([]int64(nil), r.accountIDs...), nil
}

type channelModelDeliveryAccountRepoStub struct {
	service.AccountRepository
	accounts []*service.Account
}

func (r *channelModelDeliveryAccountRepoStub) GetByIDs(_ context.Context, _ []int64) ([]*service.Account, error) {
	return append([]*service.Account(nil), r.accounts...), nil
}

type channelModelDeliveryCapabilityRepoStub struct {
	itemsByAccount map[int64][]service.AccountModelProtocolCapability
}

func (r *channelModelDeliveryCapabilityRepoStub) ListByAccount(_ context.Context, accountID int64) ([]service.AccountModelProtocolCapability, error) {
	return append([]service.AccountModelProtocolCapability(nil), r.itemsByAccount[accountID]...), nil
}

func (r *channelModelDeliveryCapabilityRepoStub) ListByAccountIDs(_ context.Context, accountIDs []int64) (map[int64][]service.AccountModelProtocolCapability, error) {
	result := make(map[int64][]service.AccountModelProtocolCapability, len(accountIDs))
	for _, accountID := range accountIDs {
		result[accountID] = append([]service.AccountModelProtocolCapability(nil), r.itemsByAccount[accountID]...)
	}
	return result, nil
}

func (r *channelModelDeliveryCapabilityRepoStub) SyncObserved(_ context.Context, _ int64, _ []service.ModelProtocolObservation) error {
	return nil
}

func (r *channelModelDeliveryCapabilityRepoStub) UpdateOverrides(_ context.Context, _ int64, _ []service.ModelProtocolOverride) error {
	return nil
}

func TestChannelHandlerGetModelDeliveryMapsAdminProjection(t *testing.T) {
	channelRepo := &channelModelDeliveryChannelRepoStub{channel: &service.Channel{
		ID:       77,
		Name:     "primary-channel",
		Status:   service.StatusActive,
		GroupIDs: []int64{20, 10},
		ModelPricing: []service.ChannelModelPricing{{
			Platform: service.PlatformOpenAI,
			Models:   []string{"MiniMax-M3"},
		}},
	}}
	groupRepo := &channelModelDeliveryGroupRepoStub{
		groups: []service.Group{
			{ID: 10, Name: "A compatibility", Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: true},
			{ID: 20, Name: "B native", Platform: service.PlatformOpenAI, Status: service.StatusActive, AllowMessagesDispatch: true},
		},
		accountIDs: []int64{82, 83},
	}
	accountRepo := &channelModelDeliveryAccountRepoStub{accounts: []*service.Account{
		{
			ID: 82, Name: "compat-account", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{10},
			Credentials: map[string]any{"model_mapping": map[string]any{"MiniMax-M3": "compat-upstream"}},
		},
		{
			ID: 83, Name: "native-account", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
			Status: service.StatusActive, Schedulable: true, GroupIDs: []int64{20},
			Credentials: map[string]any{"model_mapping": map[string]any{"MiniMax-M3": "native-upstream"}},
		},
	}}
	capabilityRepo := &channelModelDeliveryCapabilityRepoStub{itemsByAccount: map[int64][]service.AccountModelProtocolCapability{
		83: {{
			UpstreamModel: "native-upstream",
			Protocol:      service.ModelProtocolAnthropicMessages,
			OverrideState: service.ModelProtocolStateSupported,
		}},
	}}
	cfg := &config.Config{}
	cfg.Gateway.NativeModelProtocolRoutingEnabled = true
	channelService := service.NewChannelService(channelRepo, groupRepo, nil, nil)
	capabilityService := service.NewModelProtocolCapabilityService(capabilityRepo, accountRepo, groupRepo, nil, cfg)
	deliveryService := service.NewModelDeliveryService(accountRepo, groupRepo, nil, capabilityService, cfg)
	handler := NewChannelHandler(channelService, nil, nil, deliveryService)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "id", Value: "77"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/channels/77/model-delivery", nil)

	handler.GetModelDelivery(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var body struct {
		Code int `json:"code"`
		Data struct {
			ChannelID   int64                          `json:"channel_id"`
			ChannelName string                         `json:"channel_name"`
			Models      []channelModelDeliveryResponse `json:"models"`
			Warnings    []string                       `json:"warnings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	require.Zero(t, body.Code)
	require.Equal(t, int64(77), body.Data.ChannelID)
	require.Equal(t, "primary-channel", body.Data.ChannelName)
	require.NotNil(t, body.Data.Warnings)
	require.Empty(t, body.Data.Warnings)
	require.Len(t, body.Data.Models, 1)

	model := body.Data.Models[0]
	require.Equal(t, "MiniMax-M3", model.Name)
	require.Equal(t, service.PlatformOpenAI, model.Platform)
	require.Equal(t, "deliverable", model.Status)
	require.Equal(t, 2, model.DeliverableGroupCount)
	require.Equal(t, 2, model.TotalGroupCount)
	require.Equal(t, 2, model.RouteCount)
	require.Equal(t, []channelModelDeliveryEndpointResponse{{
		Protocol: string(service.ModelProtocolAnthropicMessages),
		Path:     "/v1/messages",
		Mode:     string(service.ModelDeliveryModeMixed),
		GroupIDs: []int64{10, 20},
	}}, model.Endpoints)
	require.Len(t, model.Protocols, len(service.AllModelProtocols))
	require.Equal(t, "available", model.Protocols[0].Status)
	require.Equal(t, string(service.ModelDeliveryModeMixed), model.Protocols[0].Mode)
	require.Empty(t, model.Protocols[0].UpstreamProtocol)
	require.Equal(t, []int64{10, 20}, model.Protocols[0].GroupIDs)
	require.Equal(t, "blocked", model.Protocols[1].Status)
	require.Equal(t, []string{string(service.ModelDeliveryReasonCapabilityUnknown)}, model.Protocols[1].ReasonCodes)
	require.Equal(t, "blocked", model.Protocols[2].Status)
	require.Equal(t, []string{string(service.ModelDeliveryReasonCapabilityUnknown)}, model.Protocols[2].ReasonCodes)
	require.Len(t, model.Groups, 2)

	compatGroup := model.Groups[0]
	require.Equal(t, int64(10), compatGroup.ID)
	require.Equal(t, "deliverable", compatGroup.Status)
	require.Equal(t, 1, compatGroup.RouteCount)
	require.Len(t, compatGroup.Routes, 1)
	require.Equal(t, int64(82), compatGroup.Routes[0].AccountID)
	require.Equal(t, "compat-account", compatGroup.Routes[0].AccountName)
	require.Equal(t, "MiniMax-M3", compatGroup.Routes[0].ChannelMappedModel)
	require.Equal(t, "compat-upstream", compatGroup.Routes[0].UpstreamModel)
	require.Equal(t, []channelModelDeliveryRouteEndpointResponse{{
		Protocol: string(service.ModelProtocolAnthropicMessages),
		Path:     "/v1/messages",
		Mode:     string(service.ModelDeliveryModeCompatibility),
		Source:   "existing_gateway_contract",
	}}, compatGroup.Routes[0].Endpoints)
	require.Len(t, compatGroup.Routes[0].Protocols, len(service.AllModelProtocols))
	require.Equal(t, "available", compatGroup.Routes[0].Protocols[0].Status)
	require.Equal(t, "MiniMax-M3", compatGroup.Routes[0].Protocols[0].ChannelMappedModel)
	require.Equal(t, "compat-upstream", compatGroup.Routes[0].Protocols[0].UpstreamModel)
	require.Equal(t, string(service.ModelProtocolOpenAIResponses), compatGroup.Routes[0].Protocols[0].UpstreamProtocol)

	nativeGroup := model.Groups[1]
	require.Equal(t, int64(20), nativeGroup.ID)
	require.Equal(t, "deliverable", nativeGroup.Status)
	require.Equal(t, 1, nativeGroup.RouteCount)
	require.Len(t, nativeGroup.Routes, 1)
	require.Equal(t, int64(83), nativeGroup.Routes[0].AccountID)
	require.Equal(t, "native-account", nativeGroup.Routes[0].AccountName)
	require.Equal(t, "native-upstream", nativeGroup.Routes[0].UpstreamModel)
	require.Equal(t, []channelModelDeliveryRouteEndpointResponse{{
		Protocol: string(service.ModelProtocolAnthropicMessages),
		Path:     "/v1/messages",
		Mode:     string(service.ModelDeliveryModeNative),
		Source:   "admin_override",
	}}, nativeGroup.Routes[0].Endpoints)
	require.Len(t, nativeGroup.Routes[0].Protocols, len(service.AllModelProtocols))
	require.Equal(t, "available", nativeGroup.Routes[0].Protocols[0].Status)
	require.Equal(t, "MiniMax-M3", nativeGroup.Routes[0].Protocols[0].ChannelMappedModel)
	require.Equal(t, "native-upstream", nativeGroup.Routes[0].Protocols[0].UpstreamModel)
	require.Equal(t, string(service.ModelProtocolAnthropicMessages), nativeGroup.Routes[0].Protocols[0].UpstreamProtocol)
}
