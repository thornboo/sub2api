import { describe, expect, it } from 'vitest'

import type { UserAvailableChannel } from '@/api/channels'
import { BILLING_MODE_IMAGE, BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import {
  buildAvailableChannelCatalogRows,
  formatChannelStatus,
  formatAvailableChannelGroups,
  formatAvailableChannelIntervals,
  formatBillingMode,
  formatCompactRequestPrice,
  formatCompactTokenPrice,
  formatTimePricingTokenPrice,
  formatRateMultiplier,
  formatTokenPrice,
  buildTimePricingDisplayRows,
  getActiveTimePricingMultiplier,
  hasEnabledTimePricing,
  getRowCacheReadPrice,
  getRowCacheWritePrice,
  getRowImageOutputPrice,
  getRowInputPrice,
  getRowOutputPrice,
  getRowPerRequestPrice,
  type AvailableChannelPricingLabels,
} from '../availableChannelsCatalog'

const labels: AvailableChannelPricingLabels = {
  billingModeToken: 'Per Token',
  billingModePerRequest: 'Per Request',
  billingModeImage: 'Per Image',
  noPricing: 'No Pricing',
  unitPerMillion: '/ 1M tokens',
  unitPerRequest: '/ request',
}

describe('availableChannelsCatalog', () => {
  it('derives token time-pricing rows that replace the effective group rate', () => {
    const pricing = {
      billing_mode: BILLING_MODE_TOKEN,
      input_price: 0.0000036,
      output_price: 0.0000108,
      cache_write_price: 0,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: [],
      time_pricing: {
        enabled: true,
        timezone: 'Asia/Shanghai',
        default_label: '平时',
        default_multiplier: 0.8,
        rules: [
          { label: 'morning', start_time: '09:00', end_time: '12:00', multiplier: 2 },
          { label: 'night', start_time: '23:00', end_time: '07:00', multiplier: 0.5 },
          { label: 'free', start_time: '12:00', end_time: '13:00', multiplier: 0 },
        ],
      },
    }

    expect(hasEnabledTimePricing(pricing)).toBe(true)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T00:59:00.000Z'))).toBe(0.8)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T01:00:00.000Z'))).toBe(2)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T04:00:00.000Z'))).toBe(0)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T04:59:59.000Z'))).toBe(0)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T05:00:00.000Z'))).toBe(0.8)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T15:30:00.000Z'))).toBe(0.5)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T22:30:00.000Z'))).toBe(0.5)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T23:00:00.000Z'))).toBe(0.8)

    const rows = buildTimePricingDisplayRows(
      pricing,
      { otherTimes: 'Other times', unnamedType: 'Unnamed period' },
      new Date('2026-08-17T01:00:00.000Z'),
    )

    expect(rows.map((row) => [row.windowLabel, row.label, row.active])).toEqual([
      ['Other times', '平时', false],
      ['09:00-12:00', 'morning', true],
      ['23:00-07:00', 'night', false],
      ['12:00-13:00', 'free', false],
    ])
    expect(rows[1].inputPrice).toBeCloseTo(0.0000072)
    expect(rows[1].outputPrice).toBeCloseTo(0.0000216)
    expect(rows[1].cacheWritePrice).toBe(0)
    expect(rows[1].cacheReadPrice).toBeNull()
    expect(rows[0].multiplier).toBe(0.8)
    expect(rows[0].inputPrice).toBeCloseTo(0.00000288)
    expect(formatTimePricingTokenPrice(rows[1].inputPrice)).toBe('$7.2000')
    expect(formatTimePricingTokenPrice(rows[1].cacheWritePrice)).toBe('$0.0000')
    expect(formatTimePricingTokenPrice(rows[1].cacheReadPrice)).toBe('—')
  })

  it('applies a default multiplier across the whole day when no explicit rules exist', () => {
    const pricing = {
      billing_mode: BILLING_MODE_TOKEN,
      input_price: 0.000001,
      output_price: 0.000002,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: [],
      time_pricing: {
        enabled: true,
        timezone: 'Asia/Shanghai',
        default_label: '平时',
        default_multiplier: 1.1,
        rules: [],
      },
    }

    expect(hasEnabledTimePricing(pricing)).toBe(true)
    expect(getActiveTimePricingMultiplier(pricing, new Date('2026-08-17T01:00:00.000Z'))).toBe(1.1)
    expect(buildTimePricingDisplayRows(
      pricing,
      { otherTimes: 'Other times', unnamedType: 'Unnamed period' },
      new Date('2026-08-17T01:00:00.000Z'),
    )).toMatchObject([{
      source: 'implicit',
      label: '平时',
      multiplier: 1.1,
      active: true,
      inputPrice: 0.0000011,
    }])
  })

  it('uses the winning group model price and schedule instead of the channel schedule', () => {
    const channels: UserAvailableChannel[] = [{
      name: 'DeepSeek',
      description: '',
      platforms: [{
        platform: 'deepseek',
        groups: [{
          id: 7,
          name: 'standard',
          platform: 'deepseek',
          subscription_type: 'standard',
          rate_multiplier: 0.8,
          peak_rate_enabled: false,
          peak_start: '',
          peak_end: '',
          peak_rate_multiplier: 1,
          is_exclusive: false,
        }],
        supported_models: [{
          name: 'deepseek-chat',
          platform: 'deepseek',
          pricing: {
            billing_mode: BILLING_MODE_TOKEN,
            input_price: 0.0000036,
            output_price: 0.0000108,
            cache_write_price: null,
            cache_read_price: null,
            image_input_price: null,
            image_output_price: null,
            per_request_price: null,
            intervals: [],
            time_pricing: {
              enabled: true,
              timezone: 'UTC',
              default_label: 'standard',
              rules: [{ label: 'busy', start_time: '09:00', end_time: '12:00', multiplier: 3 }],
            },
          },
          group_pricing: [{
            group_id: 7,
            pricing: {
              billing_mode: BILLING_MODE_TOKEN,
              input_price: 0.000002,
              output_price: 0.000006,
              cache_write_price: null,
              cache_read_price: null,
              image_input_price: null,
              image_output_price: null,
              per_request_price: null,
              intervals: [],
              time_pricing: {
                enabled: true,
                timezone: 'Asia/Shanghai',
                default_label: 'regular',
                default_multiplier: 2,
                rules: [{ label: 'peak', start_time: '09:00', end_time: '12:00', multiplier: 2 }],
              },
            },
          }],
        }],
      }],
    }]

    const rows = buildAvailableChannelCatalogRows(channels, { userGroupRates: { 7: 0.5 } })
    expect(rows).toHaveLength(1)
    expect(rows[0].pricing?.input_price).toBe(0.000002)
    expect(rows[0].pricing?.time_pricing?.timezone).toBe('Asia/Shanghai')
    expect(rows[0].effectiveRateMultiplier).toBe(0.5)
    expect(getRowInputPrice(rows[0])).toBeCloseTo(0.000004)
  })

  it('ignores disabled, invalid-timezone, and non-token time pricing for customer display helpers', () => {
    const base = {
      billing_mode: BILLING_MODE_TOKEN,
      input_price: 0.000001,
      output_price: 0.000002,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: [],
    }

    expect(hasEnabledTimePricing({
      ...base,
      time_pricing: {
        enabled: false,
        timezone: 'Asia/Shanghai',
        default_label: 'regular',
        rules: [{ label: 'peak', start_time: '09:00', end_time: '12:00', multiplier: 2 }],
      },
    })).toBe(false)
    expect(hasEnabledTimePricing({
      ...base,
      time_pricing: {
        enabled: true,
        timezone: 'Not/AZone',
        default_label: 'regular',
        rules: [{ label: 'peak', start_time: '09:00', end_time: '12:00', multiplier: 2 }],
      },
    })).toBe(false)
    expect(getActiveTimePricingMultiplier({
      ...base,
      billing_mode: BILLING_MODE_PER_REQUEST,
      time_pricing: {
        enabled: true,
        timezone: 'Asia/Shanghai',
        default_label: 'regular',
        rules: [{ label: 'peak', start_time: '09:00', end_time: '12:00', multiplier: 2 }],
      },
    })).toBe(1)

    expect(getActiveTimePricingMultiplier({
      ...base,
      time_pricing: {
        enabled: true,
        timezone: 'Asia/Shanghai',
        default_label: 'regular',
        rules: [{ label: 'peak', start_time: '09:00', end_time: '12:00', multiplier: -1 }],
      },
    }, new Date('2026-08-17T01:30:00.000Z'))).toBe(1)
  })

	// Group-specific image pricing is resolved here so the table, export and
	// marketplace share the settlement precedence instead of drifting apart.
	it('uses group image tiers and the independent image multiplier without mutating channel pricing', () => {
		const channelPricing = {
			billing_mode: BILLING_MODE_IMAGE,
			input_price: null,
			output_price: null,
			cache_write_price: null,
			cache_read_price: null,
			image_input_price: null,
			image_output_price: 0.00003,
			per_request_price: 0.2,
			intervals: [{
				min_tokens: 0,
				max_tokens: null,
				tier_label: '4K',
				input_price: null,
				output_price: null,
				cache_write_price: null,
				cache_read_price: null,
				per_request_price: 0.3,
			}],
		}
		const channels: UserAvailableChannel[] = [{
			name: 'Image Channel',
			description: '',
			platforms: [{
				platform: 'openai',
				groups: [{
					id: 1,
					name: 'images',
					platform: 'openai',
					subscription_type: 'standard',
					rate_multiplier: 0.1,
					image_rate_independent: true,
					image_rate_multiplier: 0.5,
					image_price_1k: 0.02,
					image_price_2k: null,
					image_price_4k: null,
					peak_rate_enabled: false,
					peak_start: '',
					peak_end: '',
					peak_rate_multiplier: 1,
					is_exclusive: false,
				}],
				supported_models: [{
					name: 'gpt-image-2',
					platform: 'openai',
					pricing: channelPricing,
				}],
			}],
		}]

		const rows = buildAvailableChannelCatalogRows(channels, {
			expandIntervals: true,
			userGroupRates: { 1: 0.01 },
		})

		expect(rows.map((row) => row.intervalLabel)).toEqual(['1K', '2K', '4K'])
		expect(rows.map((row) => row.effectiveRateMultiplier)).toEqual([0.5, 0.5, 0.5])
		expect(rows.map(getRowPerRequestPrice)).toEqual([0.01, 0.1, 0.15])
		expect(channelPricing.intervals).toHaveLength(1)
		expect(channelPricing.intervals[0].tier_label).toBe('4K')
	})

  it('flattens channel sections into model rows sorted by model then platform', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'OpenAI Channel',
        description: 'primary',
        platforms: [
          {
            platform: 'openai',
            groups: [],
            supported_models: [
              {
                name: 'gpt-5.4',
                platform: 'openai',
                pricing: null,
              },
              {
                name: 'gpt-5.2',
                platform: 'openai',
                pricing: null,
              },
            ],
          },
        ],
      },
      {
        name: 'Gemini Channel',
        description: '',
        platforms: [
          {
            platform: 'gemini',
            groups: [],
            supported_models: [
              {
                name: 'gemini-3.5-flash',
                platform: 'gemini',
                pricing: null,
              },
              {
                name: 'gpt-5.2',
                platform: 'gemini',
                pricing: null,
              },
            ],
          },
        ],
      },
    ]

    expect(buildAvailableChannelCatalogRows(channels).map((row) => row.modelName)).toEqual([
      'gemini-3.5-flash',
      'gpt-5.2',
      'gpt-5.2',
      'gpt-5.4',
    ])
  })

  it('formats token pricing and interval pricing for comparison/export rows', () => {
    const pricing = {
      billing_mode: BILLING_MODE_TOKEN,
      input_price: 0.000003,
      output_price: 0.000015,
      cache_write_price: 0.00000375,
      cache_read_price: 0.0000003,
      image_output_price: null,
      per_request_price: null,
      intervals: [
        {
          min_tokens: 0,
          max_tokens: 128000,
          input_price: 0.000003,
          output_price: 0.000015,
          cache_write_price: null,
          cache_read_price: null,
          per_request_price: null,
        },
      ],
    }

    expect(formatBillingMode(pricing, labels)).toBe('Per Token')
    expect(formatTokenPrice(pricing.input_price, labels)).toBe('$3 / 1M tokens')
    expect(formatCompactTokenPrice(pricing.input_price)).toBe('$3')
    expect(formatCompactRequestPrice(0.5)).toBe('$0.5')
    expect(formatAvailableChannelIntervals(pricing, labels)).toBe('(0, 128000]: $3 / $15 / 1M tokens')
    expect(formatAvailableChannelIntervals(pricing, labels, { compact: true })).toBe('(0, 128000]: $3 / $15')
  })

  it('can omit subscription groups from export rows without dropping empty-group sections', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Mixed Channel',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'public',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 1,
                is_exclusive: false,
              },
              {
                id: 2,
                name: 'monthly',
                platform: 'openai',
                subscription_type: 'subscription',
                rate_multiplier: 1,
                is_exclusive: false,
              },
            ],
            supported_models: [
              {
                name: 'gpt-5.4',
                platform: 'openai',
                pricing: null,
              },
            ],
          },
          {
            platform: 'gemini',
            groups: [
              {
                id: 3,
                name: 'sub-only',
                platform: 'gemini',
                subscription_type: 'subscription',
                rate_multiplier: 1,
                is_exclusive: false,
              },
            ],
            supported_models: [
              {
                name: 'gemini-3.5-flash',
                platform: 'gemini',
                pricing: null,
              },
            ],
          },
          {
            platform: 'anthropic',
            groups: [],
            supported_models: [
              {
                name: 'claude-sonnet-4.6',
                platform: 'anthropic',
                pricing: null,
              },
            ],
          },
        ],
      },
    ]

    const rows = buildAvailableChannelCatalogRows(channels, { includeSubscriptionGroups: false })

    expect(rows.map((row) => row.modelName)).toEqual(['claude-sonnet-4.6', 'gpt-5.4'])
    expect(rows.find((row) => row.modelName === 'gpt-5.4')?.groups.map((group) => group.name)).toEqual(['public'])
  })

  it('treats null catalog arrays from backend responses as empty arrays', () => {
    const channels = [
      {
        name: 'Admin Catalog Channel',
        description: '',
        platforms: [
          {
            platform: 'anthropic',
            groups: null,
            supported_models: [
              {
                name: 'claude-sonnet-4.6',
                platform: 'anthropic',
                pricing: {
                  billing_mode: BILLING_MODE_TOKEN,
                  input_price: null,
                  output_price: null,
                  cache_write_price: null,
                  cache_read_price: null,
                  image_output_price: null,
                  per_request_price: null,
                  intervals: null,
                },
              },
            ],
          },
        ],
      },
      {
        name: 'Empty Catalog Channel',
        description: '',
        platforms: null,
      },
    ] as unknown as UserAvailableChannel[]

    const rows = buildAvailableChannelCatalogRows(channels, {
      includeSubscriptionGroups: false,
      expandIntervals: true,
    })

    expect(rows).toHaveLength(1)
    expect(rows[0].modelName).toBe('claude-sonnet-4.6')
    expect(rows[0].groups).toEqual([])
  })

  it('filters group scope, billing mode and price status before building rows', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Mixed Channel',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'public',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 1,
                is_exclusive: false,
              },
              {
                id: 2,
                name: 'exclusive',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 1,
                is_exclusive: true,
              },
            ],
            supported_models: [
              {
                name: 'priced-model',
                platform: 'openai',
                pricing: {
                  billing_mode: BILLING_MODE_TOKEN,
                  input_price: 0.000001,
                  output_price: 0.000002,
                  cache_write_price: null,
                  cache_read_price: null,
                  image_output_price: null,
                  per_request_price: null,
                  intervals: [],
                },
              },
              {
                name: 'per-request-model',
                platform: 'openai',
                pricing: {
                  billing_mode: BILLING_MODE_PER_REQUEST,
                  input_price: null,
                  output_price: null,
                  cache_write_price: null,
                  cache_read_price: null,
                  image_output_price: null,
                  per_request_price: 0.02,
                  intervals: [],
                },
              },
              {
                name: 'unpriced-model',
                platform: 'openai',
                pricing: null,
              },
            ],
          },
        ],
      },
    ]

    const publicRows = buildAvailableChannelCatalogRows(channels, { groupScope: 'public' })
    expect(publicRows.every((row) => row.groups.every((group) => !group.is_exclusive))).toBe(true)

    const tokenRows = buildAvailableChannelCatalogRows(channels, {
      billingMode: BILLING_MODE_TOKEN,
      priceStatus: 'priced',
    })
    expect(tokenRows).toHaveLength(2)
    expect(tokenRows.every((row) => row.modelName === 'priced-model')).toBe(true)
    expect(tokenRows.map((row) => row.groups[0]?.name).sort()).toEqual(['exclusive', 'public'])

    const unpricedRows = buildAvailableChannelCatalogRows(channels, { priceStatus: 'unpriced' })
    expect(unpricedRows).toHaveLength(2)
    expect(unpricedRows.every((row) => row.modelName === 'unpriced-model')).toBe(true)
  })

  it('splits model quotes by callable group and applies the effective user rate to every price field', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Shared Channel',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'public',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 0.8,
                is_exclusive: false,
              },
              {
                id: 2,
                name: 'exclusive',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 0.7,
                is_exclusive: true,
              },
              {
                id: 3,
                name: 'not-callable',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 0.1,
                is_exclusive: false,
              },
            ],
            supported_models: [
              {
                name: 'priced-model',
                platform: 'openai',
                route_group_ids: [1, 2],
                pricing: {
                  billing_mode: BILLING_MODE_TOKEN,
                  input_price: 0.000001,
                  output_price: 0.000002,
                  cache_write_price: 0.000003,
                  cache_read_price: 0.0000004,
                  image_input_price: null,
                  image_output_price: 0.04,
                  per_request_price: 0.02,
                  intervals: [],
                },
              },
            ],
          },
        ],
      },
    ]

    const rows = buildAvailableChannelCatalogRows(channels, {
      sortBy: 'inputPrice',
      userGroupRates: { 1: 0.5 },
    })

    expect(rows).toHaveLength(2)
    expect(rows.map((row) => row.groups[0]?.name)).toEqual(['public', 'exclusive'])
    expect(rows.map((row) => row.effectiveRateMultiplier)).toEqual([0.5, 0.7])
    expect(getRowInputPrice(rows[0])).toBeCloseTo(0.0000005)
    expect(getRowOutputPrice(rows[0])).toBeCloseTo(0.000001)
    expect(getRowCacheWritePrice(rows[0])).toBeCloseTo(0.0000015)
    expect(getRowCacheReadPrice(rows[0])).toBeCloseTo(0.0000002)
    expect(getRowImageOutputPrice(rows[0])).toBeCloseTo(0.02)
    expect(getRowPerRequestPrice(rows[0])).toBeCloseTo(0.01)
    expect(getRowInputPrice(rows[1])).toBeCloseTo(0.0000007)
  })

  it('uses endpoint group metadata only as a legacy fallback for quote rows', () => {
    const sectionGroups = [
      {
        id: 1,
        name: 'public',
        platform: 'openai',
        subscription_type: 'standard',
        rate_multiplier: 0.8,
        peak_rate_enabled: false,
        peak_start: '',
        peak_end: '',
        peak_rate_multiplier: 1,
        is_exclusive: false,
      },
      {
        id: 2,
        name: 'exclusive',
        platform: 'openai',
        subscription_type: 'standard',
        rate_multiplier: 0.7,
        peak_rate_enabled: false,
        peak_start: '',
        peak_end: '',
        peak_rate_multiplier: 1,
        is_exclusive: true,
      },
    ]
    const channel = (model: UserAvailableChannel['platforms'][number]['supported_models'][number]): UserAvailableChannel => ({
      name: 'Legacy Channel',
      description: '',
      platforms: [{ platform: 'openai', groups: sectionGroups, supported_models: [model] }],
    })

    const legacyRows = buildAvailableChannelCatalogRows([channel({
      name: 'legacy-model',
      platform: 'openai',
      pricing: null,
      supported_endpoints: [
        { protocol: 'openai_chat_completions', path: '/v1/chat/completions', group_ids: [2] },
      ],
    })])
    expect(legacyRows.map((row) => row.groups[0]?.id)).toEqual([2])

    const authoritativeEmptyRows = buildAvailableChannelCatalogRows([channel({
      name: 'no-route-model',
      platform: 'openai',
      pricing: null,
      route_group_ids: [],
      supported_endpoints: [
        { protocol: 'openai_chat_completions', path: '/v1/chat/completions', group_ids: [] },
      ],
    })])
    expect(authoritativeEmptyRows).toEqual([])

    const noGroupAuthoritativeRows = buildAvailableChannelCatalogRows([{
      name: 'Admin Diagnostic Channel',
      description: '',
      platforms: [{
        platform: 'openai',
        groups: [],
        supported_models: [{
          name: 'no-group-route-model',
          platform: 'openai',
          pricing: null,
          route_group_ids: [],
        }],
      }],
    }])
    expect(noGroupAuthoritativeRows).toEqual([])

    const noGroupLegacyRows = buildAvailableChannelCatalogRows([{
      name: 'Rollback-Compatible Channel',
      description: '',
      platforms: [{
        platform: 'openai',
        groups: [],
        supported_models: [{
          name: 'legacy-no-group-model',
          platform: 'openai',
          pricing: null,
        }],
      }],
    }])
    expect(noGroupLegacyRows).toHaveLength(1)
  })

  it('expands tiered pricing into interval rows and sorts by effective row price', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Tiered Channel',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [],
            supported_models: [
              {
                name: 'tiered-model',
                platform: 'openai',
                pricing: {
                  billing_mode: BILLING_MODE_TOKEN,
                  input_price: 0.00001,
                  output_price: 0.00002,
                  cache_write_price: null,
                  cache_read_price: null,
                  image_output_price: null,
                  per_request_price: null,
                  intervals: [
                    {
                      min_tokens: 0,
                      max_tokens: 128000,
                      tier_label: 'short',
                      input_price: 0.000001,
                      output_price: 0.000002,
                      cache_write_price: null,
                      cache_read_price: null,
                      per_request_price: null,
                    },
                    {
                      min_tokens: 128000,
                      max_tokens: null,
                      tier_label: 'long',
                      input_price: 0.000003,
                      output_price: 0.000004,
                      cache_write_price: null,
                      cache_read_price: null,
                      per_request_price: null,
                    },
                  ],
                },
              },
            ],
          },
        ],
      },
    ]

    const rows = buildAvailableChannelCatalogRows(channels, {
      expandIntervals: true,
      sortBy: 'inputPrice',
      sortOrder: 'desc',
    })

    expect(rows.map((row) => row.intervalLabel)).toEqual(['long', 'short'])
    expect(getRowInputPrice(rows[0])).toBe(0.000003)
    expect(getRowOutputPrice(rows[1])).toBe(0.000002)
  })

  it('uses interval request price for per-request tier rows', () => {
    const rows = buildAvailableChannelCatalogRows(
      [
        {
          name: 'Request Tier Channel',
          description: '',
          platforms: [
            {
              platform: 'openai',
              groups: [],
              supported_models: [
                {
                  name: 'request-model',
                  platform: 'openai',
                  pricing: {
                    billing_mode: BILLING_MODE_PER_REQUEST,
                    input_price: null,
                    output_price: null,
                    cache_write_price: null,
                    cache_read_price: null,
                    image_output_price: null,
                    per_request_price: 0.01,
                    intervals: [
                      {
                        min_tokens: 0,
                        max_tokens: 1000,
                        tier_label: 'small',
                        input_price: null,
                        output_price: null,
                        cache_write_price: null,
                        cache_read_price: null,
                        per_request_price: 0.02,
                      },
                    ],
                  },
                },
              ],
            },
          ],
        },
      ],
      { expandIntervals: true },
    )

    expect(rows).toHaveLength(1)
    expect(getRowPerRequestPrice(rows[0])).toBe(0.02)
  })

  it('uses user-specific group rates when available', () => {
    expect(
      formatAvailableChannelGroups(
        [
          {
            id: 7,
            name: 'public',
            platform: 'openai',
            subscription_type: 'standard',
            rate_multiplier: 1,
            is_exclusive: false,
          },
        ],
        { 7: 0.75 },
      ),
    ).toBe('public 0.75x')
  })

  it('formats large group rate multipliers without losing integer digits', () => {
    expect(formatRateMultiplier(1000)).toBe('1000x')
    expect(formatRateMultiplier(1230)).toBe('1230x')
    expect(formatRateMultiplier(2000)).toBe('2000x')
    expect(formatRateMultiplier(0.333333)).toBe('0.3333x')
  })

  it('can include or isolate disabled admin channels for export rows', () => {
    const channels: Array<UserAvailableChannel & { status: string }> = [
      {
        name: 'Active Channel',
        description: '',
        status: 'active',
        platforms: [
          {
            platform: 'openai',
            groups: [],
            supported_models: [{ name: 'gpt-5.4', platform: 'openai', pricing: null }],
          },
        ],
      },
      {
        name: 'Disabled Channel',
        description: '',
        status: 'disabled',
        platforms: [
          {
            platform: 'openai',
            groups: [],
            supported_models: [{ name: 'gpt-5.5', platform: 'openai', pricing: null }],
          },
        ],
      },
    ]

    expect(buildAvailableChannelCatalogRows(channels, { statusScope: 'all' }).map((row) => row.channelName)).toEqual([
      'Active Channel',
      'Disabled Channel',
    ])
    expect(buildAvailableChannelCatalogRows(channels, { statusScope: 'disabled' }).map((row) => row.channelName)).toEqual([
      'Disabled Channel',
    ])
    expect(buildAvailableChannelCatalogRows(channels, { activeOnly: true }).map((row) => row.channelName)).toEqual([
      'Active Channel',
    ])
  })

  it('keeps current available-channel rows when active status filtering falls back to user data', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'Visible Channel',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [],
            supported_models: [{ name: 'gpt-5.4', platform: 'openai', pricing: null }],
          },
        ],
      },
    ]

    const rows = buildAvailableChannelCatalogRows(channels, { statusScope: 'active' })

    expect(rows.map((row) => row.channelName)).toEqual(['Visible Channel'])
  })

  it('formats channel status labels for export', () => {
    const statusLabels = {
      statusActive: 'Enabled',
      statusDisabled: 'Disabled',
      statusUnknown: '-',
    }

    expect(formatChannelStatus('active', statusLabels)).toBe('Enabled')
    expect(formatChannelStatus('disabled', statusLabels)).toBe('Disabled')
    expect(formatChannelStatus(undefined, statusLabels)).toBe('-')
  })
})
