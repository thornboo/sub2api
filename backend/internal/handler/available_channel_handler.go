package handler

import (
	"context"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AvailableChannelHandler 处理用户侧「可用渠道」查询。
//
// 用户侧接口委托 ChannelService.ListAvailable，并在返回前做四层过滤：
//  1. 行过滤：只保留状态为 Active 且与当前用户可访问分组有交集的渠道；
//  2. 分组过滤：渠道的 Groups 只保留用户可访问的那些；
//  3. 平台过滤：普通分组只保留自身平台模型；Composite 分组按渠道已配置的具体模型平台
//     展开。这样既防止普通分组跨平台泄漏，也让 Composite 正确展示其多平台能力；
//  4. 字段白名单：仅返回用户需要的字段（省略 BillingModelSource / RestrictModels
//     / 内部 ID / Status 等管理字段）。
type AvailableChannelHandler struct {
	channelService *service.ChannelService
	apiKeyService  *service.APIKeyService
	settingService *service.SettingService
	modelDelivery  *service.ModelDeliveryService
}

// NewAvailableChannelHandler 创建用户侧可用渠道 handler。
func NewAvailableChannelHandler(
	channelService *service.ChannelService,
	apiKeyService *service.APIKeyService,
	settingService *service.SettingService,
	modelDelivery *service.ModelDeliveryService,
) *AvailableChannelHandler {
	return &AvailableChannelHandler{
		channelService: channelService,
		apiKeyService:  apiKeyService,
		settingService: settingService,
		modelDelivery:  modelDelivery,
	}
}

// featureEnabled 返回 available-channels 开关是否启用。默认关闭（opt-in）。
func (h *AvailableChannelHandler) featureEnabled(c *gin.Context) bool {
	if h.settingService == nil {
		return false
	}
	return h.settingService.GetAvailableChannelsRuntime(c.Request.Context()).Enabled
}

// userAvailableGroup 用户可见的分组概要（白名单字段）。
//
// 前端据此区分专属 vs 公开分组（IsExclusive）、订阅 vs 标准分组（SubscriptionType，
// 订阅视觉加深），并展示默认倍率与高峰倍率规则；用户专属倍率前端走
// /groups/rates，和 API 密钥页面保持一致。
type userAvailableGroup struct {
	ID                   int64    `json:"id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Platform             string   `json:"platform"`
	SubscriptionType     string   `json:"subscription_type"`
	RateMultiplier       float64  `json:"rate_multiplier"`
	ImageRateIndependent bool     `json:"image_rate_independent"`
	ImageRateMultiplier  float64  `json:"image_rate_multiplier"`
	ImagePrice1K         *float64 `json:"image_price_1k"`
	ImagePrice2K         *float64 `json:"image_price_2k"`
	ImagePrice4K         *float64 `json:"image_price_4k"`
	PeakRateEnabled      bool     `json:"peak_rate_enabled"`
	PeakStart            string   `json:"peak_start"`
	PeakEnd              string   `json:"peak_end"`
	PeakRateMultiplier   float64  `json:"peak_rate_multiplier"`
	IsExclusive          bool     `json:"is_exclusive"`
	// AllowMessagesDispatch participates in the user-callable endpoint contract
	// for OpenAI groups, but is not exposed as channel-directory metadata.
	AllowMessagesDispatch bool `json:"-"`
}

// userSupportedModelPricing 用户可见的定价字段白名单。
type userSupportedModelPricing struct {
	BillingMode      string                   `json:"billing_mode"`
	InputPrice       *float64                 `json:"input_price"`
	OutputPrice      *float64                 `json:"output_price"`
	CacheWritePrice  *float64                 `json:"cache_write_price"`
	CacheReadPrice   *float64                 `json:"cache_read_price"`
	ImageInputPrice  *float64                 `json:"image_input_price"`
	ImageOutputPrice *float64                 `json:"image_output_price"`
	PerRequestPrice  *float64                 `json:"per_request_price"`
	Intervals        []userPricingIntervalDTO `json:"intervals"`
	TimePricing      *userTimePricing         `json:"time_pricing,omitempty"`
}

// userTimePricing 是客户目录可见的分时售价合同。类型名称是管理员明确
// 配置的客户文案，与倍率相互独立。
type userTimePricing struct {
	Enabled           bool                  `json:"enabled"`
	Timezone          string                `json:"timezone"`
	DefaultLabel      string                `json:"default_label"`
	DefaultMultiplier *float64              `json:"default_multiplier,omitempty"`
	Rules             []userTimePricingRule `json:"rules"`
}

type userTimePricingRule struct {
	Label      string  `json:"label"`
	StartTime  string  `json:"start_time"`
	EndTime    string  `json:"end_time"`
	Multiplier float64 `json:"multiplier"`
}

// userPricingIntervalDTO 定价区间白名单（去掉内部 ID、SortOrder 等前端不渲染的字段）。
type userPricingIntervalDTO struct {
	MinTokens       int      `json:"min_tokens"`
	MaxTokens       *int     `json:"max_tokens"`
	TierLabel       string   `json:"tier_label,omitempty"`
	InputPrice      *float64 `json:"input_price"`
	OutputPrice     *float64 `json:"output_price"`
	CacheWritePrice *float64 `json:"cache_write_price"`
	CacheReadPrice  *float64 `json:"cache_read_price"`
	PerRequestPrice *float64 `json:"per_request_price"`
}

// userSupportedModel 用户可见的支持模型条目。
type userSupportedModel struct {
	Name               string                     `json:"name"`
	Platform           string                     `json:"platform"`
	Pricing            *userSupportedModelPricing `json:"pricing"`
	GroupPricing       []userGroupModelPricing    `json:"group_pricing,omitempty"`
	RouteGroupIDs      []int64                    `json:"route_group_ids,omitempty"`
	SupportedEndpoints []userSupportedEndpoint    `json:"supported_endpoints,omitempty"`
}

type userGroupModelPricing struct {
	GroupID int64                      `json:"group_id"`
	Pricing *userSupportedModelPricing `json:"pricing"`
}

type userSupportedEndpoint struct {
	Protocol string  `json:"protocol"`
	Path     string  `json:"path"`
	GroupIDs []int64 `json:"group_ids"`
}

// userChannelPlatformSection 单渠道内某个平台的子视图：用户可见的分组 + 该平台
// 支持的模型。按 platform 聚合后让前端可以把渠道名作为 row-group 一次渲染，
// 后面的平台行按 sections 顺序铺开。
type userChannelPlatformSection struct {
	Platform        string               `json:"platform"`
	Groups          []userAvailableGroup `json:"groups"`
	SupportedModels []userSupportedModel `json:"supported_models"`
}

// userAvailableChannel 用户可见的渠道条目（白名单字段）。
//
// 每个渠道聚合为一条记录，内嵌 platforms 子数组：每个 section 对应一个平台，
// 包含该平台的 groups 和 supported_models。
type userAvailableChannel struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Platforms   []userChannelPlatformSection `json:"platforms"`
}

type availableGroupFilter func([]service.AvailableGroupRef) []userAvailableGroup

// List 列出当前用户可见的「可用渠道」。
// GET /api/v1/channels/available
func (h *AvailableChannelHandler) List(c *gin.Context) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	// Feature 未启用时返回空数组（不暴露渠道信息）。检查放在认证之后，
	// 保持与未开关前的 401 行为一致：未登录先 401，登录后再按开关决定。
	if !h.featureEnabled(c) {
		response.Success(c, []userAvailableChannel{})
		return
	}

	userGroups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	allowedGroupIDs := make(map[int64]struct{}, len(userGroups))
	for i := range userGroups {
		allowedGroupIDs[userGroups[i].ID] = struct{}{}
	}

	channels, err := h.channelService.ListAvailable(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out, err := buildAvailableChannelCatalog(
		c.Request.Context(),
		channels,
		h.modelDelivery,
		func(groups []service.AvailableGroupRef) []userAvailableGroup {
			return filterUserVisibleGroups(groups, allowedGroupIDs)
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, out)
}

// buildAvailableChannelCatalog projects a set of visible groups through the
// same customer-safe channel, model, pricing and endpoint contract. Callers
// own only group visibility; delivery eligibility must stay shared.
func buildAvailableChannelCatalog(
	ctx context.Context,
	channels []service.AvailableChannel,
	modelDelivery *service.ModelDeliveryService,
	filterGroups availableGroupFilter,
) ([]userAvailableChannel, error) {
	out := make([]userAvailableChannel, 0, len(channels))
	for _, ch := range channels {
		if ch.Status != service.StatusActive {
			continue
		}
		visibleGroups := filterGroups(ch.Groups)
		if len(visibleGroups) == 0 {
			continue
		}
		sections := buildPlatformSections(ch, visibleGroups)
		if len(sections) == 0 {
			continue
		}
		out = append(out, userAvailableChannel{
			Name:        ch.Name,
			Description: ch.Description,
			Platforms:   sections,
		})
	}
	if err := attachSupportedEndpoints(ctx, modelDelivery, out); err != nil {
		return nil, err
	}
	return pruneUndeliverableChannels(out), nil
}

func attachSupportedEndpoints(
	ctx context.Context,
	modelDelivery *service.ModelDeliveryService,
	channels []userAvailableChannel,
) error {
	if len(channels) == 0 {
		return nil
	}

	if modelDelivery == nil {
		return nil
	}
	groupSet := make(map[int64]struct{})
	modelSet := make(map[string]struct{})
	for i := range channels {
		for j := range channels[i].Platforms {
			section := &channels[i].Platforms[j]
			for _, group := range section.Groups {
				groupSet[group.ID] = struct{}{}
			}
			for _, model := range section.SupportedModels {
				modelSet[model.Name] = struct{}{}
			}
		}
	}
	groupIDs := make([]int64, 0, len(groupSet))
	for groupID := range groupSet {
		groupIDs = append(groupIDs, groupID)
	}
	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	delivery, err := modelDelivery.ResolveForGroups(ctx, groupIDs, models)
	if err != nil {
		return err
	}
	protocolOrder := []service.ModelProtocol{
		service.ModelProtocolAnthropicMessages,
		service.ModelProtocolOpenAIChat,
		service.ModelProtocolOpenAIResponses,
	}
	for i := range channels {
		for j := range channels[i].Platforms {
			section := &channels[i].Platforms[j]
			visible := make(map[int64]struct{}, len(section.Groups))
			for _, group := range section.Groups {
				visible[group.ID] = struct{}{}
			}
			for k := range section.SupportedModels {
				model := &section.SupportedModels[k]
				model.RouteGroupIDs = intersectVisibleGroupIDs(delivery.CallableGroupIDs(model.Name), visible)
				for _, protocol := range protocolOrder {
					ids := intersectVisibleGroupIDs(delivery.NativeEndpointGroupIDs(model.Name, protocol), visible)
					if len(ids) == 0 {
						continue
					}
					upsertUserSupportedEndpoint(model, protocol, ids)
				}
			}
		}
	}
	filterUndeliverableModels(channels, delivery)
	return nil
}

func filterUndeliverableModels(channels []userAvailableChannel, delivery *service.ModelDeliveryProjection) {
	for i := range channels {
		for j := range channels[i].Platforms {
			section := &channels[i].Platforms[j]
			visible := make(map[int64]struct{}, len(section.Groups))
			for _, group := range section.Groups {
				visible[group.ID] = struct{}{}
			}
			filtered := section.SupportedModels[:0]
			for _, model := range section.SupportedModels {
				eligibleGroupIDs := delivery.CallableGroupIDs(model.Name)
				if len(intersectVisibleGroupIDs(eligibleGroupIDs, visible)) == 0 {
					continue
				}
				filtered = append(filtered, model)
			}
			section.SupportedModels = filtered
		}
	}
}

func pruneUndeliverableChannels(channels []userAvailableChannel) []userAvailableChannel {
	filteredChannels := channels[:0]
	for i := range channels {
		channel := &channels[i]
		filteredSections := channel.Platforms[:0]
		for _, section := range channel.Platforms {
			if len(section.SupportedModels) > 0 {
				filteredSections = append(filteredSections, section)
			}
		}
		channel.Platforms = filteredSections
		if len(channel.Platforms) > 0 {
			filteredChannels = append(filteredChannels, *channel)
		}
	}
	return filteredChannels
}

func upsertUserSupportedEndpoint(model *userSupportedModel, protocol service.ModelProtocol, groupIDs []int64) {
	if model == nil || len(groupIDs) == 0 {
		return
	}
	path := modelProtocolPath(protocol)
	if path == "" {
		return
	}
	for i := range model.SupportedEndpoints {
		endpoint := &model.SupportedEndpoints[i]
		if endpoint.Protocol != string(protocol) || endpoint.Path != path {
			continue
		}
		endpoint.GroupIDs = mergeEndpointGroupIDs(endpoint.GroupIDs, groupIDs)
		return
	}
	model.SupportedEndpoints = append(model.SupportedEndpoints, userSupportedEndpoint{
		Protocol: string(protocol),
		Path:     path,
		GroupIDs: mergeEndpointGroupIDs(nil, groupIDs),
	})
}

func mergeEndpointGroupIDs(existing, additions []int64) []int64 {
	seen := make(map[int64]struct{}, len(existing)+len(additions))
	for _, ids := range [][]int64{existing, additions} {
		for _, id := range ids {
			if id > 0 {
				seen[id] = struct{}{}
			}
		}
	}
	merged := make([]int64, 0, len(seen))
	for id := range seen {
		merged = append(merged, id)
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i] < merged[j] })
	return merged
}

func modelProtocolPath(protocol service.ModelProtocol) string {
	switch protocol {
	case service.ModelProtocolAnthropicMessages:
		return "/v1/messages"
	case service.ModelProtocolOpenAIChat:
		return "/v1/chat/completions"
	case service.ModelProtocolOpenAIResponses:
		return "/v1/responses"
	default:
		return ""
	}
}

func intersectVisibleGroupIDs(ids []int64, visible map[int64]struct{}) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, ok := visible[id]; ok {
			result = append(result, id)
		}
	}
	return result
}

// buildPlatformSections 把一个渠道按 visibleGroups 的平台集合拆成有序的 section 列表：
// 每个 section 对应一个具体平台，只包含该平台的 groups 和 supported_models。
//
// Composite 分组可访问渠道中所有已配置的具体平台，因此会被展开到每个有支持模型的
// 平台 section。普通分组仍严格留在自身平台，避免跨平台模型信息泄漏。Composite 渠道
// 尚未配置任何模型时保留 composite section，以便前端继续展示该分组和“未配置模型”状态。
// 输出按 platform 字母序稳定排序，便于前端等效比较与回归测试。
func buildPlatformSections(
	ch service.AvailableChannel,
	visibleGroups []userAvailableGroup,
) []userChannelPlatformSection {
	groupsByPlatform := make(map[string][]userAvailableGroup, 4)
	compositeGroups := make([]userAvailableGroup, 0, 1)
	for _, g := range visibleGroups {
		if g.Platform == "" {
			continue
		}
		if g.Platform == service.PlatformComposite {
			compositeGroups = append(compositeGroups, g)
			continue
		}
		groupsByPlatform[g.Platform] = append(groupsByPlatform[g.Platform], g)
	}

	if len(compositeGroups) > 0 {
		modelPlatforms := make(map[string]struct{}, len(ch.SupportedModels))
		for i := range ch.SupportedModels {
			if platform := ch.SupportedModels[i].Platform; platform != "" {
				modelPlatforms[platform] = struct{}{}
			}
		}
		if len(modelPlatforms) == 0 {
			groupsByPlatform[service.PlatformComposite] = append(
				groupsByPlatform[service.PlatformComposite],
				compositeGroups...,
			)
		} else {
			for platform := range modelPlatforms {
				groupsByPlatform[platform] = append(groupsByPlatform[platform], compositeGroups...)
			}
		}
	}
	if len(groupsByPlatform) == 0 {
		return nil
	}

	platforms := make([]string, 0, len(groupsByPlatform))
	for p := range groupsByPlatform {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	sections := make([]userChannelPlatformSection, 0, len(platforms))
	for _, platform := range platforms {
		platformSet := map[string]struct{}{platform: {}}
		visibleGroupIDs := make(map[int64]struct{}, len(groupsByPlatform[platform]))
		for _, group := range groupsByPlatform[platform] {
			visibleGroupIDs[group.ID] = struct{}{}
		}
		sections = append(sections, userChannelPlatformSection{
			Platform:        platform,
			Groups:          groupsByPlatform[platform],
			SupportedModels: toUserSupportedModels(ch.SupportedModels, platformSet, visibleGroupIDs),
		})
	}
	return sections
}

// filterUserVisibleGroups 仅保留用户可访问的分组。
func filterUserVisibleGroups(
	groups []service.AvailableGroupRef,
	allowed map[int64]struct{},
) []userAvailableGroup {
	visible := make([]userAvailableGroup, 0, len(groups))
	for _, g := range groups {
		if _, ok := allowed[g.ID]; !ok {
			continue
		}
		visible = append(visible, toUserAvailableGroup(g))
	}
	return visible
}

// filterPublicStandardGroups is the public catalog visibility boundary.
// ListAvailable already sources active groups; this filter additionally keeps
// only standard groups that require no explicit grant or subscription.
func filterPublicStandardGroups(groups []service.AvailableGroupRef) []userAvailableGroup {
	visible := make([]userAvailableGroup, 0, len(groups))
	for _, g := range groups {
		if g.IsExclusive || g.SubscriptionType != service.SubscriptionTypeStandard {
			continue
		}
		visible = append(visible, toUserAvailableGroup(g))
	}
	return visible
}

func toUserAvailableGroup(g service.AvailableGroupRef) userAvailableGroup {
	return userAvailableGroup{
		ID:                    g.ID,
		Name:                  g.Name,
		Description:           g.Description,
		Platform:              g.Platform,
		SubscriptionType:      g.SubscriptionType,
		RateMultiplier:        g.RateMultiplier,
		ImageRateIndependent:  g.ImageRateIndependent,
		ImageRateMultiplier:   g.ImageRateMultiplier,
		ImagePrice1K:          g.ImagePrice1K,
		ImagePrice2K:          g.ImagePrice2K,
		ImagePrice4K:          g.ImagePrice4K,
		PeakRateEnabled:       g.PeakRateEnabled,
		PeakStart:             g.PeakStart,
		PeakEnd:               g.PeakEnd,
		PeakRateMultiplier:    g.PeakRateMultiplier,
		IsExclusive:           g.IsExclusive,
		AllowMessagesDispatch: g.AllowMessagesDispatch,
	}
}

// toUserSupportedModels 将 service 层支持模型转换为用户 DTO（字段白名单）。
// 仅保留平台在 allowedPlatforms 中的条目，防止跨平台模型信息泄漏。
// allowedPlatforms 为 nil 时不做平台过滤（保留全部，供测试或明确无过滤场景使用）。
func toUserSupportedModels(
	src []service.SupportedModel,
	allowedPlatforms map[string]struct{},
	visibleGroupIDs ...map[int64]struct{},
) []userSupportedModel {
	out := make([]userSupportedModel, 0, len(src))
	var allowedGroups map[int64]struct{}
	if len(visibleGroupIDs) > 0 {
		allowedGroups = visibleGroupIDs[0]
	}
	for i := range src {
		m := src[i]
		if allowedPlatforms != nil {
			if _, ok := allowedPlatforms[m.Platform]; !ok {
				continue
			}
		}
		groupPricing := make([]userGroupModelPricing, 0, len(m.GroupPricing))
		for groupID, pricing := range m.GroupPricing {
			if allowedGroups != nil {
				if _, ok := allowedGroups[groupID]; !ok {
					continue
				}
			}
			groupPricing = append(groupPricing, userGroupModelPricing{
				GroupID: groupID,
				Pricing: toUserPricing(pricing),
			})
		}
		sort.Slice(groupPricing, func(i, j int) bool { return groupPricing[i].GroupID < groupPricing[j].GroupID })
		out = append(out, userSupportedModel{
			Name:         m.Name,
			Platform:     m.Platform,
			Pricing:      toUserPricing(m.Pricing),
			GroupPricing: groupPricing,
		})
	}
	return out
}

// toUserPricing 将 service 层定价转换为用户 DTO；入参为 nil 时返回 nil。
func toUserPricing(p *service.ChannelModelPricing) *userSupportedModelPricing {
	if p == nil {
		return nil
	}
	intervals := make([]userPricingIntervalDTO, 0, len(p.Intervals))
	for _, iv := range p.Intervals {
		intervals = append(intervals, userPricingIntervalDTO{
			MinTokens:       iv.MinTokens,
			MaxTokens:       iv.MaxTokens,
			TierLabel:       iv.TierLabel,
			InputPrice:      iv.InputPrice,
			OutputPrice:     iv.OutputPrice,
			CacheWritePrice: iv.CacheWritePrice,
			CacheReadPrice:  iv.CacheReadPrice,
			PerRequestPrice: iv.PerRequestPrice,
		})
	}
	billingMode := string(p.BillingMode)
	if billingMode == "" {
		billingMode = string(service.BillingModeToken)
	}
	return &userSupportedModelPricing{
		BillingMode:      billingMode,
		InputPrice:       p.InputPrice,
		OutputPrice:      p.OutputPrice,
		CacheWritePrice:  p.CacheWritePrice,
		CacheReadPrice:   p.CacheReadPrice,
		ImageInputPrice:  p.ImageInputPrice,
		ImageOutputPrice: p.ImageOutputPrice,
		PerRequestPrice:  p.PerRequestPrice,
		Intervals:        intervals,
		TimePricing:      toUserTimePricing(p.TimePricing),
	}
}

func toUserTimePricing(p *service.TimePricing) *userTimePricing {
	if p == nil {
		return nil
	}
	rules := make([]userTimePricingRule, 0, len(p.Rules))
	for _, rule := range p.Rules {
		rules = append(rules, userTimePricingRule{
			Label:      rule.Label,
			StartTime:  rule.StartTime,
			EndTime:    rule.EndTime,
			Multiplier: rule.Multiplier,
		})
	}
	var defaultMultiplier *float64
	if p.DefaultMultiplier != nil {
		value := *p.DefaultMultiplier
		defaultMultiplier = &value
	}
	return &userTimePricing{
		Enabled:           p.Enabled,
		Timezone:          p.Timezone,
		DefaultLabel:      p.DefaultLabel,
		DefaultMultiplier: defaultMultiplier,
		Rules:             rules,
	}
}
