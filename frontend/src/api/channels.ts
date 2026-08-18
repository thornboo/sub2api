/**
 * User Channels API endpoints (non-admin)
 * 用户侧「可用渠道」聚合查询：渠道 + 用户可访问的分组 + 支持模型（含定价）。
 */

import { apiClient } from './client'
import type { BillingMode } from '@/constants/channel'

export interface UserAvailableGroup {
  id: number
  name: string
  description?: string
  platform: string
  /** 'standard' | 'subscription' — 订阅分组视觉加深，和 API 密钥页保持一致。 */
  subscription_type: string
  /** 分组默认倍率。用户专属倍率（若有）通过 /groups/rates 获取后在前端 join。 */
  rate_multiplier: number
  /** 图片计费使用独立倍率时，不再叠加分组或用户专属倍率。 */
  image_rate_independent?: boolean
  image_rate_multiplier?: number
  /** 分组图片档位价；存在任一档时按实际计费回落规则生成 1K / 2K / 4K 展示价。 */
  image_price_1k?: number | null
  image_price_2k?: number | null
  image_price_4k?: number | null
  peak_rate_enabled: boolean
  peak_start: string
  peak_end: string
  peak_rate_multiplier: number
  /** true = 专属分组（小范围授权）；false = 公开分组。 */
  is_exclusive: boolean
}

export interface UserPricingInterval {
  min_tokens: number
  max_tokens: number | null
  tier_label?: string
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  per_request_price: number | null
}

export interface UserTimePricingRule {
  label: string
  start_time: string
  end_time: string
  multiplier: number
}

export interface UserTimePricing {
  enabled: boolean
  timezone: string
  default_label: string
  /** Missing on legacy records; billing treats omission as 1x. */
  default_multiplier?: number
  rules: UserTimePricingRule[]
}

export interface UserSupportedModelPricing {
  billing_mode: BillingMode
  input_price: number | null
  output_price: number | null
  cache_write_price: number | null
  cache_read_price: number | null
  image_input_price: number | null
  image_output_price: number | null
  per_request_price: number | null
  intervals: UserPricingInterval[]
  time_pricing?: UserTimePricing | null
}

export interface UserSupportedModel {
  name: string
  platform: string
  pricing: UserSupportedModelPricing | null
  group_pricing?: Array<{
    group_id: number
    pricing: UserSupportedModelPricing | null
  }>
  route_group_ids?: number[]
  supported_endpoints?: UserSupportedEndpoint[]
}

export interface UserSupportedEndpoint {
  protocol: 'anthropic_messages' | 'openai_chat_completions' | 'openai_responses'
  path: string
  group_ids: number[]
}

/**
 * 渠道下单个平台的子视图：用户可访问的分组 + 该平台支持的模型。
 * 后端把一个渠道按平台聚合成 sections，前端可以把渠道名作为 row-group
 * 一次渲染，后面按 sections 顺序用 rowspan 铺开。
 */
export interface UserChannelPlatformSection {
  platform: string
  groups: UserAvailableGroup[]
  supported_models: UserSupportedModel[]
}

export interface UserAvailableChannel {
  name: string
  description: string
  platforms: UserChannelPlatformSection[]
}

function arrayOrEmpty<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : []
}

export function normalizeAvailableChannels<T extends UserAvailableChannel>(
  channels: T[] | null | undefined,
): T[] {
  return arrayOrEmpty(channels).map((channel) => ({
    ...channel,
    description: channel.description ?? '',
    platforms: arrayOrEmpty(channel.platforms).map((section) => ({
      ...section,
      groups: arrayOrEmpty(section.groups).map((group) => ({
        ...group,
        description: group.description ?? '',
      })),
      supported_models: arrayOrEmpty(section.supported_models).map((model) => {
        const normalized = {
          ...model,
          pricing: model.pricing
            ? {
                ...model.pricing,
                intervals: arrayOrEmpty(model.pricing.intervals),
                time_pricing: model.pricing.time_pricing
                  ? {
                      ...model.pricing.time_pricing,
                      rules: arrayOrEmpty(model.pricing.time_pricing.rules),
                    }
                  : model.pricing.time_pricing,
              }
            : null,
        }
        if (Array.isArray(model.route_group_ids)) {
          normalized.route_group_ids = arrayOrEmpty(model.route_group_ids)
        }
        if (Array.isArray(model.group_pricing)) {
          normalized.group_pricing = model.group_pricing.map((entry) => ({
            ...entry,
            pricing: entry.pricing
              ? {
                  ...entry.pricing,
                  intervals: arrayOrEmpty(entry.pricing.intervals),
                  time_pricing: entry.pricing.time_pricing
                    ? {
                        ...entry.pricing.time_pricing,
                        rules: arrayOrEmpty(entry.pricing.time_pricing.rules),
                      }
                    : entry.pricing.time_pricing,
                }
              : null,
          }))
        }
        if (Array.isArray(model.supported_endpoints)) {
          normalized.supported_endpoints = model.supported_endpoints.map((endpoint) => ({
            ...endpoint,
            group_ids: arrayOrEmpty(endpoint.group_ids),
          }))
        }
        return normalized
      }),
    })),
  }))
}

/** 列出当前用户可见的「可用渠道」（与 /groups/available 保持一致，返回平数组）。 */
export async function getAvailable(options?: { signal?: AbortSignal }): Promise<UserAvailableChannel[]> {
  const { data } = await apiClient.get<UserAvailableChannel[]>('/channels/available', {
    signal: options?.signal
  })
  return normalizeAvailableChannels(data)
}

export const userChannelsAPI = { getAvailable }

export default userChannelsAPI
