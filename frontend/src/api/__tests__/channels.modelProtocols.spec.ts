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
            catalog_group_ids: [10],
            route_group_ids: [],
            schedulable_group_ids: [10],
            runtime_metrics: [{
              group_id: 10,
              metrics: {
                window_hours: 24,
                updated_at: '2026-09-09T00:00:00Z',
                success_rate: 0.995,
                average_latency_ms: 1360,
                latency_kind: 'firstToken' as const,
                sample_state: 'ready' as const,
                throughput_tokens_per_second: 51,
                hours: [{
                  started_at: '2026-09-08T23:00:00Z',
                  success_rate: null,
                  average_latency_ms: null,
                  request_volume: undefined as unknown as number,
                }],
              },
            }],
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
    expect(result[0].platforms[0].supported_models[0].catalog_group_ids).toEqual([10])
    expect(result[0].platforms[0].supported_models[0].route_group_ids).toEqual([])
    expect(result[0].platforms[0].supported_models[0].schedulable_group_ids).toEqual([10])
    expect(result[0].platforms[0].supported_models[0].runtime_metrics).toEqual([{
      group_id: 10,
      metrics: {
        window_hours: 24,
        updated_at: '2026-09-09T00:00:00Z',
        success_rate: 0.995,
        average_latency_ms: 1360,
        latency_kind: 'firstToken',
        sample_state: 'ready',
        throughput_tokens_per_second: 51,
        hours: [{
          started_at: '2026-09-08T23:00:00Z',
          success_rate: null,
          average_latency_ms: null,
          request_volume: 0,
        }],
      },
    }])
    expect(result[0].platforms[0].supported_models[1].supported_endpoints).toBeUndefined()
    expect(result[0].platforms[0].supported_models[1].catalog_group_ids).toBeUndefined()
    expect(result[0].platforms[0].supported_models[1].schedulable_group_ids).toBeUndefined()
    expect(result[0].platforms[0].supported_models[1].runtime_metrics).toBeUndefined()
  })
})
