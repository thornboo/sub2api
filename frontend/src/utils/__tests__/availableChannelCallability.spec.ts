import { describe, expect, it } from 'vitest'

import type { UserAvailableGroup, UserSupportedModel } from '@/api/channels'
import { resolveAvailableModelGroupContexts } from '@/utils/availableChannelCallability'

const groups: UserAvailableGroup[] = [
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

function model(overrides: Partial<UserSupportedModel> = {}): UserSupportedModel {
  return {
    name: 'gpt-test',
    platform: 'openai',
    pricing: null,
    ...overrides,
  }
}

describe('resolveAvailableModelGroupContexts', () => {
  it('treats route group metadata as authoritative over endpoint metadata', () => {
    const contexts = resolveAvailableModelGroupContexts(model({
      route_group_ids: [2],
      supported_endpoints: [
        { protocol: 'openai_responses', path: '/v1/responses', group_ids: [1] },
      ],
    }), groups)

    expect(contexts.map(({ group }) => group.id)).toEqual([2])
    expect(contexts[0].endpoints).toEqual([])
  })

  it('falls back to endpoint groups when route metadata is absent', () => {
    const contexts = resolveAvailableModelGroupContexts(model({
      supported_endpoints: [
        { protocol: 'openai_responses', path: '/v1/responses', group_ids: [2] },
      ],
    }), groups)

    expect(contexts.map(({ group }) => group.id)).toEqual([2])
    expect(contexts[0].endpoints[0].group_ids).toEqual([2])
  })

  it('retains rollback-compatible groups when both metadata fields are omitted', () => {
    const contexts = resolveAvailableModelGroupContexts(model(), groups)

    expect(contexts.map(({ group }) => group.id)).toEqual([1, 2])
    expect(contexts.every(({ endpoints }) => endpoints.length === 0)).toBe(true)
  })

  it('treats an explicit empty route group array as no callable group', () => {
    const contexts = resolveAvailableModelGroupContexts(model({
      route_group_ids: [],
      supported_endpoints: [
        { protocol: 'openai_responses', path: '/v1/responses', group_ids: [] },
      ],
    }), groups)

    expect(contexts).toEqual([])
  })
})
