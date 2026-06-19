import { describe, expect, it } from 'vitest'

import type { UserAvailableChannel } from '@/api/channels'
import { BILLING_MODE_PER_REQUEST, BILLING_MODE_TOKEN } from '@/constants/channel'
import {
  buildAvailableChannelCatalogRows,
  formatChannelStatus,
  formatAvailableChannelGroups,
  formatAvailableChannelIntervals,
  formatBillingMode,
  formatCompactRequestPrice,
  formatCompactTokenPrice,
  formatRateMultiplier,
  formatTokenPrice,
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
    expect(tokenRows.map((row) => row.modelName)).toEqual(['priced-model'])

    const unpricedRows = buildAvailableChannelCatalogRows(channels, { priceStatus: 'unpriced' })
    expect(unpricedRows.map((row) => row.modelName)).toEqual(['unpriced-model'])
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
