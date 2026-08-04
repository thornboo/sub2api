import { describe, expect, it } from 'vitest'

import type { UserAvailableChannel } from '@/api/channels'
import { BILLING_MODE_IMAGE, BILLING_MODE_TOKEN } from '@/constants/channel'
import { buildAvailableModelMarketplaceCards } from '../availableModelMarketplace'

const publicGroup = {
  id: 1,
  name: '公开组',
  platform: 'openai',
  subscription_type: 'standard',
  rate_multiplier: 0.8,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  is_exclusive: false,
}

const exclusiveGroup = {
  id: 2,
  name: '专属组',
  platform: 'openai',
  subscription_type: 'standard',
  rate_multiplier: 0.7,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  is_exclusive: true,
}

const anthropicGroup = {
  id: 3,
  name: 'Anthropic 公开组',
  platform: 'anthropic',
  subscription_type: 'standard',
  rate_multiplier: 1,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  is_exclusive: false,
}

const pricing = {
  billing_mode: BILLING_MODE_TOKEN,
  input_price: 0.000001,
  output_price: 0.000005,
  cache_write_price: null,
  cache_read_price: null,
  image_input_price: null,
  image_output_price: null,
  per_request_price: null,
  intervals: [],
}

const channels: UserAvailableChannel[] = [
  {
    name: 'channel-a',
    description: 'first route',
    platforms: [
      {
        platform: 'openai',
        groups: [publicGroup, exclusiveGroup],
        supported_models: [
          {
            name: 'MiniMax-M3',
            platform: 'openai',
            pricing,
            supported_endpoints: [
              { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [2] },
              { protocol: 'openai_chat_completions', path: '/v1/chat/completions', group_ids: [1, 2] },
            ],
          },
        ],
      },
    ],
  },
  {
    name: 'channel-b',
    description: 'second route',
    platforms: [
      {
        platform: 'anthropic',
        groups: [anthropicGroup],
        supported_models: [
          {
            name: 'MiniMax-M3',
            platform: 'anthropic',
            pricing,
            supported_endpoints: [
              { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [3] },
            ],
          },
          {
            name: 'claude-sonnet',
            platform: 'anthropic',
            pricing: null,
            supported_endpoints: [
              { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [3] },
            ],
          },
        ],
      },
    ],
  },
  {
    name: 'channel-c',
    description: 'another route in the public group',
    platforms: [
      {
        platform: 'openai',
        groups: [publicGroup],
        supported_models: [
          {
            name: 'MiniMax-M3',
            platform: 'openai',
            pricing,
            supported_endpoints: [
              { protocol: 'openai_chat_completions', path: '/v1/chat/completions', group_ids: [1] },
              { protocol: 'openai_responses', path: '/v1/responses', group_ids: [1] },
            ],
          },
        ],
      },
    ],
  },
]

describe('buildAvailableModelMarketplaceCards', () => {
	it('aggregates group-specific image tiers using settlement fallback precedence', () => {
		const imageChannels: UserAvailableChannel[] = [{
			name: 'images',
			description: '',
			platforms: [{
				platform: 'openai',
				groups: [{
					...publicGroup,
					image_rate_independent: true,
					image_rate_multiplier: 0.5,
					image_price_1k: 0.02,
					image_price_2k: null,
					image_price_4k: null,
				}],
				supported_models: [{
					name: 'gpt-image-2',
					platform: 'openai',
					pricing: {
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
					},
				}],
			}],
		}]

		const [card] = buildAvailableModelMarketplaceCards(imageChannels)

		expect(card.pricingOptions).toHaveLength(1)
		expect(card.pricingOptions[0]?.intervals.map((interval) => [
			interval.tier_label,
			interval.per_request_price,
		])).toEqual([
			['1K', 0.02],
			['2K', 0.2],
			['4K', 0.3],
		])
		expect(card.routes[0].pricing).toEqual(card.pricingOptions[0])
	})

  it('separates the same model by group while aggregating routes inside each group', () => {
    const cards = buildAvailableModelMarketplaceCards(channels)
    const minimaxCards = cards.filter(card => card.name === 'MiniMax-M3')
    const publicMinimax = minimaxCards.find(card => card.group.id === 1)
    const exclusiveMinimax = minimaxCards.find(card => card.group.id === 2)
    const anthropicMinimax = minimaxCards.find(card => card.group.id === 3)

    expect(cards).toHaveLength(4)
    expect(minimaxCards).toHaveLength(3)
    expect(publicMinimax?.channelNames).toEqual(['channel-a', 'channel-c'])
    expect(publicMinimax?.platforms).toEqual(['openai'])
    expect(publicMinimax?.routes).toHaveLength(2)
    expect(publicMinimax?.routes.every(route => route.group.id === 1)).toBe(true)
    expect(publicMinimax?.endpoints.map(endpoint => endpoint.protocol)).toEqual([
      'openai_chat_completions',
      'openai_responses',
    ])
    expect(exclusiveMinimax?.channelNames).toEqual(['channel-a'])
    expect(exclusiveMinimax?.endpoints.map(endpoint => endpoint.protocol)).toEqual([
      'anthropic_messages',
      'openai_chat_completions',
    ])
    expect(anthropicMinimax?.channelNames).toEqual(['channel-b'])
    expect(anthropicMinimax?.endpoints.map(endpoint => endpoint.protocol)).toEqual([
      'anthropic_messages',
    ])
    expect(minimaxCards.every(card => card.pricingOptions.length === 1)).toBe(true)
  })

  it('applies group and price filters before aggregating protocol capabilities', () => {
    const exclusiveCards = buildAvailableModelMarketplaceCards(channels, {
      groupScope: 'exclusive',
      priceStatus: 'priced',
    })

    expect(exclusiveCards).toHaveLength(1)
    expect(exclusiveCards[0].name).toBe('MiniMax-M3')
    expect(exclusiveCards[0].channelNames).toEqual(['channel-a'])
    expect(exclusiveCards[0].group.name).toBe('专属组')
    expect(exclusiveCards[0].endpoints.map(endpoint => endpoint.protocol)).toEqual([
      'anthropic_messages',
      'openai_chat_completions',
    ])

    const unpricedCards = buildAvailableModelMarketplaceCards(channels, { priceStatus: 'unpriced' })
    expect(unpricedCards.map(card => card.name)).toEqual(['claude-sonnet'])
  })

  it('does not create a group card when that exact group has no callable endpoint', () => {
    const noEndpointForPublicGroup: UserAvailableChannel[] = [
      {
        name: 'channel-a',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [publicGroup, exclusiveGroup],
            supported_models: [
              {
                name: 'MiniMax-M3',
                platform: 'openai',
                pricing,
                supported_endpoints: [
                  { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [2] },
                ],
              },
            ],
          },
        ],
      },
    ]

    const cards = buildAvailableModelMarketplaceCards(noEndpointForPublicGroup)

    expect(cards).toHaveLength(1)
    expect(cards[0].group.id).toBe(2)
  })

  it('keeps stable legacy cards when endpoint metadata is omitted by the global rollback switch', () => {
    const legacyChannels: UserAvailableChannel[] = [{
      name: 'legacy-channel',
      description: '',
      platforms: [{
        platform: 'openai',
        groups: [publicGroup],
        supported_models: [{
          name: 'glm-5.2',
          platform: 'openai',
          pricing,
        }],
      }],
    }]

    const cards = buildAvailableModelMarketplaceCards(legacyChannels)

    expect(cards).toHaveLength(1)
    expect(cards[0].name).toBe('glm-5.2')
    expect(cards[0].group.id).toBe(publicGroup.id)
    expect(cards[0].endpoints).toEqual([])
  })

  it('uses route group metadata to retain only the callable group when no endpoint is published', () => {
    const unknownCapabilityChannels: UserAvailableChannel[] = [{
      name: 'unknown-capability',
      description: '',
      platforms: [{
        platform: 'openai',
        groups: [publicGroup, exclusiveGroup],
        supported_models: [{
          name: 'glm-5.2',
          platform: 'openai',
          pricing,
          route_group_ids: [exclusiveGroup.id],
          supported_endpoints: [],
        }],
      }],
    }]

    const cards = buildAvailableModelMarketplaceCards(unknownCapabilityChannels)

    expect(cards).toHaveLength(1)
    expect(cards[0].group.id).toBe(exclusiveGroup.id)
    expect(cards[0].endpoints).toEqual([])
  })
})
