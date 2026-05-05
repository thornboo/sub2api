import { describe, expect, it } from 'vitest'
import { buildChannelModelRecommendations } from '../channelModelRecommendations'
import type { Channel } from '@/api/admin/channels'

function channel(overrides: Partial<Channel>): Channel {
  return {
    id: 1,
    name: 'Main channel',
    description: '',
    status: 'active',
    billing_model_source: 'custom',
    restrict_models: false,
    group_ids: [10],
    model_pricing: [],
    model_mapping: {},
    apply_pricing_to_account_stats: false,
    account_stats_pricing_rules: [],
    created_at: '',
    updated_at: '',
    ...overrides
  } as Channel
}

describe('buildChannelModelRecommendations', () => {
  it('uses channel mapping targets for account-layer mapping recommendations', () => {
    const recommendations = buildChannelModelRecommendations(
      [
        channel({
          model_mapping: {
            openai: {
              'gpt-5.5': 'gemini-2.5-pro',
              'gpt-5.5-mini': 'gemini-2.5-flash'
            }
          },
          model_pricing: [
            {
              platform: 'openai',
              models: ['gpt-5.5'],
              billing_mode: 'token',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              image_output_price: null,
              per_request_price: null,
              intervals: []
            }
          ]
        }),
        channel({
          id: 2,
          name: 'Other group',
          group_ids: [20],
          model_mapping: {
            openai: {
              ignored: 'ignored-target'
            }
          }
        })
      ],
      [10],
      'openai'
    )

    expect(recommendations.map(item => item.model)).toEqual(['gemini-2.5-pro', 'gemini-2.5-flash'])
    expect(recommendations.every(item => item.source === 'mapping')).toBe(true)
  })

  it('falls back to pricing models only when matching channels have no platform mapping', () => {
    const recommendations = buildChannelModelRecommendations(
      [
        channel({
          model_pricing: [
            {
              platform: 'openai',
              models: ['gpt-5.5', 'gpt-5.5-mini'],
              billing_mode: 'token',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              image_output_price: null,
              per_request_price: null,
              intervals: []
            },
            {
              platform: 'anthropic',
              models: ['claude-opus-4-7'],
              billing_mode: 'token',
              input_price: null,
              output_price: null,
              cache_write_price: null,
              cache_read_price: null,
              image_output_price: null,
              per_request_price: null,
              intervals: []
            }
          ]
        })
      ],
      [10],
      'openai'
    )

    expect(recommendations).toEqual([
      {
        model: 'gpt-5.5',
        title: 'Main channel: gpt-5.5',
        source: 'pricing'
      },
      {
        model: 'gpt-5.5-mini',
        title: 'Main channel: gpt-5.5-mini',
        source: 'pricing'
      }
    ])
  })
})
