import { describe, expect, it } from 'vitest'

import { normalizeAvailableChannels } from '@/api/channels'

describe('available channels model protocol metadata', () => {
  it('normalizes declared endpoint arrays while preserving omitted rollback metadata', () => {
    const result = normalizeAvailableChannels([
      {
        name: 'new-api',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [],
            supported_models: [
              {
                name: 'MiniMax-M3',
                platform: 'openai',
                pricing: null,
                supported_endpoints: [
                  {
                    protocol: 'anthropic_messages' as const,
                    path: '/v1/messages',
                    group_ids: [10],
                  },
                ],
              },
              {
                name: 'Kimi-K2',
                platform: 'openai',
                pricing: null,
              },
            ],
          },
        ],
      },
    ])

    expect(result[0].platforms[0].supported_models[0].supported_endpoints).toEqual([
      { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [10] },
    ])
    expect(result[0].platforms[0].supported_models[1].supported_endpoints).toBeUndefined()
  })
})
