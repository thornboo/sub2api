package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// AvailableGroupRef 渠道视图中关联分组的简要信息。
//
// 用户侧「可用渠道」页面据此展示：专属分组 vs 公开分组（IsExclusive）、
// 订阅 vs 标准（SubscriptionType）、默认倍率（RateMultiplier）与高峰倍率规则。
// 用户专属倍率不在这里暴露，前端自己通过 /groups/rates 拉取，和 API 密钥页面保持一致。
type AvailableGroupRef struct {
	ID                          int64
	Name                        string
	Description                 string
	Platform                    string
	SubscriptionType            string
	RateMultiplier              float64
	ImageRateIndependent        bool
	ImageRateMultiplier         float64
	ImagePrice1K                *float64
	ImagePrice2K                *float64
	ImagePrice4K                *float64
	PeakRateEnabled             bool
	PeakStart                   string
	PeakEnd                     string
	PeakRateMultiplier          float64
	IsExclusive                 bool
	AllowMessagesDispatch       bool
	MessagesDispatchModelConfig OpenAIMessagesDispatchModelConfig
}

// AvailableChannel 可用渠道视图：用于「可用渠道」页面展示渠道基础信息 +
// 关联的分组 + 推导出的支持模型列表（无通配符）。
type AvailableChannel struct {
	ID                 int64
	Name               string
	Description        string
	Status             string
	BillingModelSource string
	RestrictModels     bool
	Groups             []AvailableGroupRef
	SupportedModels    []SupportedModel
}

// ListAvailable 返回所有渠道的可用视图：每个渠道附带关联分组信息与支持模型列表。
//
// 支持模型通过 (*Channel).SupportedModels() 计算（mapping ∪ pricing 并联）。
// 对于渠道未配置定价的模型，进一步复用 BillingService 的 LiteLLM → fallback
// 基础价格链合成展示用定价，让用户看到的默认价格与真实结算一致。
//
// 关联分组信息通过 groupRepo.ListActive 查询后按 ID 映射；渠道 GroupIDs 中未在活跃列表中
// 的分组（已停用或删除）会被忽略。
//
// 前置条件：s.groupRepo 必须非 nil（由 wire DI 保证）。直接 nil-deref 用于 fail-fast，
// 避免静默掩盖注入缺失。
func (s *ChannelService) ListAvailable(ctx context.Context) ([]AvailableChannel, error) {
	channels, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}

	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}
	groupByID := make(map[int64]AvailableGroupRef, len(groups))
	groupEntityByID := make(map[int64]*Group, len(groups))
	for i := range groups {
		g := &groups[i]
		groupByID[g.ID] = AvailableGroupRef{
			ID:                          g.ID,
			Name:                        g.Name,
			Description:                 g.Description,
			Platform:                    g.Platform,
			SubscriptionType:            g.SubscriptionType,
			RateMultiplier:              g.RateMultiplier,
			ImageRateIndependent:        g.ImageRateIndependent,
			ImageRateMultiplier:         g.ImageRateMultiplier,
			ImagePrice1K:                g.ImagePrice1K,
			ImagePrice2K:                g.ImagePrice2K,
			ImagePrice4K:                g.ImagePrice4K,
			PeakRateEnabled:             g.PeakRateEnabled,
			PeakStart:                   g.PeakStart,
			PeakEnd:                     g.PeakEnd,
			PeakRateMultiplier:          g.PeakRateMultiplier,
			IsExclusive:                 g.IsExclusive,
			AllowMessagesDispatch:       g.AllowMessagesDispatch,
			MessagesDispatchModelConfig: cloneGroupMessagesDispatchModelConfig(g.MessagesDispatchModelConfig),
		}
		groupEntityByID[g.ID] = g
	}

	out := make([]AvailableChannel, 0, len(channels))
	for i := range channels {
		ch := &channels[i]
		groups := make([]AvailableGroupRef, 0, len(ch.GroupIDs))
		for _, gid := range ch.GroupIDs {
			if ref, ok := groupByID[gid]; ok {
				groups = append(groups, ref)
			}
		}
		sort.SliceStable(groups, func(i, j int) bool { return groups[i].Name < groups[j].Name })

		ch.normalizeBillingModelSource()

		supported := ch.SupportedModels()
		s.fillGlobalPricingFallback(supported)
		for modelIndex := range supported {
			for _, groupRef := range groups {
				groupPricing := s.availableGroupModelPricing(groupEntityByID[groupRef.ID], supported[modelIndex].Name)
				if groupPricing == nil {
					continue
				}
				if supported[modelIndex].GroupPricing == nil {
					supported[modelIndex].GroupPricing = make(map[int64]*ChannelModelPricing)
				}
				supported[modelIndex].GroupPricing[groupRef.ID] = groupPricing
			}
		}

		out = append(out, AvailableChannel{
			ID:                 ch.ID,
			Name:               ch.Name,
			Description:        ch.Description,
			Status:             ch.Status,
			BillingModelSource: ch.BillingModelSource,
			RestrictModels:     ch.RestrictModels,
			Groups:             groups,
			SupportedModels:    supported,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// availableGroupModelPricing mirrors the Group-first pricing precedence for
// the customer catalog. A matching Group entry replaces Channel pricing as a
// unit; token fields not explicitly overridden continue to fall back to the
// global model price, exactly as ModelPricingResolver does at settlement.
func (s *ChannelService) availableGroupModelPricing(group *Group, model string) *ChannelModelPricing {
	configured := matchGroupModelPricing(group, model)
	if configured == nil {
		return nil
	}
	mode := configured.BillingMode
	if mode == "" {
		mode = BillingModeToken
	}
	if mode != BillingModeToken {
		result := configured.Clone()
		result.BillingMode = mode
		return &result
	}

	result := s.availableTokenBasePricing(model, nil)
	if result == nil {
		result = &ChannelModelPricing{BillingMode: BillingModeToken}
	}
	result.BillingMode = BillingModeToken
	result.Models = append([]string(nil), configured.Models...)
	result.Platform = configured.Platform
	result.Intervals = nil // Group token cards never replace official long-context ladders.
	if configured.InputPrice != nil {
		result.InputPrice = configured.InputPrice
	}
	if configured.OutputPrice != nil {
		result.OutputPrice = configured.OutputPrice
	}
	if configured.CacheWritePrice != nil {
		result.CacheWritePrice = configured.CacheWritePrice
	}
	if configured.CacheReadPrice != nil {
		result.CacheReadPrice = configured.CacheReadPrice
	}
	// These two fields intentionally follow the settlement override semantics:
	// an omitted Group image-token override does not inherit the Channel value.
	result.ImageInputPrice = configured.ImageInputPrice
	result.ImageOutputPrice = configured.ImageOutputPrice
	result.PerRequestPrice = configured.PerRequestPrice
	if configured.TimePricing != nil {
		timePricing := configured.TimePricing.Clone()
		result.TimePricing = &timePricing
	}
	return result
}

// fillGlobalPricingFallback 对未命中渠道定价的支持模型合成一份展示用定价。
// 图片/按次模式保留 LiteLLM 元数据；token 模式复用 BillingService 的
// LiteLLM → fallback 解析链，确保报价与真实结算不分叉。
//
// 触发条件：
//  1. Pricing == nil（渠道完全没声明该模型的定价条目）
//  2. Pricing 非 nil 但所有价格字段为空（admin UI 建了条目但没填价格）
func (s *ChannelService) fillGlobalPricingFallback(models []SupportedModel) {
	for i := range models {
		if !pricingNeedsFallback(models[i].Pricing) {
			continue
		}
		existing := models[i].Pricing
		if s.pricingService != nil {
			existing = synthesizePricingFromLiteLLM(s.pricingService.GetModelPricing(models[i].Name), existing)
		}
		if !pricingNeedsFallback(existing) || (existing != nil && existing.BillingMode != "" && existing.BillingMode != BillingModeToken) {
			models[i].Pricing = existing
			continue
		}
		models[i].Pricing = s.availableTokenBasePricing(models[i].Name, existing)
	}
}

// fillGlobalPricingFallback is retained for services that only have the
// catalog pricing dependency. ChannelService uses the method above so it can
// prefer the BillingService settlement basis when available.
func fillGlobalPricingFallback(pricingService *PricingService, models []SupportedModel) {
	if pricingService == nil {
		return
	}
	for i := range models {
		if !pricingNeedsFallback(models[i].Pricing) {
			continue
		}
		existing := synthesizePricingFromLiteLLM(pricingService.GetModelPricing(models[i].Name), models[i].Pricing)
		if pricingNeedsFallback(existing) {
			continue
		}
		models[i].Pricing = existing
	}
}

// availableTokenBasePricing returns the same token basis used by settlement.
// The existing shape is retained so channel/group metadata can be overlaid by callers.
func (s *ChannelService) availableTokenBasePricing(model string, existing *ChannelModelPricing) *ChannelModelPricing {
	if s.billingService == nil {
		if s.pricingService == nil {
			return existing
		}
		return synthesizePricingFromLiteLLM(s.pricingService.GetModelPricing(model), existing)
	}
	pricing, err := s.billingService.GetModelPricing(model)
	if err != nil || pricing == nil {
		return existing
	}
	return synthesizePricingFromModelPricing(pricing, existing)
}

// pricingNeedsFallback 判定一个 ChannelModelPricing 是否需要走全局回落。
// 价格全部缺失（无 flat 字段且无任何带价 interval）即视为未配置。
func pricingNeedsFallback(p *ChannelModelPricing) bool {
	if p == nil {
		return true
	}
	if p.InputPrice != nil || p.OutputPrice != nil ||
		p.CacheWritePrice != nil || p.CacheReadPrice != nil ||
		p.ImageInputPrice != nil || p.ImageOutputPrice != nil || p.PerRequestPrice != nil {
		return false
	}
	for _, iv := range p.Intervals {
		if iv.InputPrice != nil || iv.OutputPrice != nil ||
			iv.CacheWritePrice != nil || iv.CacheReadPrice != nil ||
			iv.PerRequestPrice != nil {
			return false
		}
	}
	return true
}

// synthesizePricingFromLiteLLM 把 LiteLLM 的定价数据转成 ChannelModelPricing 形态，
// 仅用于展示。
//
// 计费模式优先级：
//  1. 渠道已选 BillingMode（admin 在 UI 里选了 image / per_request 但没填价的场景，
//     按选定模式合成对应字段）
//  2. LiteLLM mode="image_generation" → image
//  3. 默认 token
//
// LiteLLM 中字段 0 视为未配置，不带入展示。
func synthesizePricingFromLiteLLM(lp *LiteLLMModelPricing, existing *ChannelModelPricing) *ChannelModelPricing {
	if lp == nil {
		return existing
	}

	mode := BillingModeToken
	switch {
	case existing != nil && existing.BillingMode != "":
		mode = existing.BillingMode
	case lp.Mode == "image_generation":
		mode = BillingModeImage
	}

	if mode == BillingModeImage || mode == BillingModePerRequest {
		return &ChannelModelPricing{
			BillingMode:      mode,
			PerRequestPrice:  nonZeroPtr(lp.OutputCostPerImage),
			ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
			InputPrice:       nonZeroPtr(lp.InputCostPerToken),
			OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		}
	}
	return &ChannelModelPricing{
		BillingMode:      mode,
		InputPrice:       nonZeroPtr(lp.InputCostPerToken),
		OutputPrice:      nonZeroPtr(lp.OutputCostPerToken),
		CacheWritePrice:  nonZeroPtr(lp.CacheCreationInputTokenCost),
		CacheReadPrice:   nonZeroPtr(lp.CacheReadInputTokenCost),
		ImageOutputPrice: nonZeroPtr(lp.OutputCostPerImageToken),
	}
}

// synthesizePricingFromModelPricing converts BillingService's resolved token basis
// into the customer catalog shape. Zero is kept as an explicit pointer because an
// administrator/provider may intentionally configure a free token dimension.
func synthesizePricingFromModelPricing(pricing *ModelPricing, existing *ChannelModelPricing) *ChannelModelPricing {
	if pricing == nil {
		return existing
	}
	result := &ChannelModelPricing{BillingMode: BillingModeToken}
	if existing != nil {
		*result = existing.Clone()
		result.BillingMode = BillingModeToken
	}
	result.InputPrice = float64ValuePtr(pricing.InputPricePerToken)
	result.OutputPrice = float64ValuePtr(pricing.OutputPricePerToken)
	result.CacheWritePrice = float64ValuePtr(pricing.CacheCreationPricePerToken)
	result.CacheReadPrice = float64ValuePtr(pricing.CacheReadPricePerToken)
	result.ImageInputPrice = float64ValuePtr(pricing.ImageInputPricePerToken)
	result.ImageOutputPrice = float64ValuePtr(pricing.ImageOutputPricePerToken)
	return result
}

func nonZeroPtr(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

func float64ValuePtr(v float64) *float64 {
	return &v
}
