import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { detail, list } from '@/api/modelStatus'

describe('model status API normalization', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('keeps valid rows while dropping malformed rows and optional history points', async () => {
    const signal = new AbortController().signal
    get.mockResolvedValue({
      data: {
        items: [
          {
            group_id: 10,
            group_name: null,
            model: ' gpt-4o ',
            display_name: null,
            status: 'future-status',
            message_code: 'future-message',
            latest_latency_ms: Number.POSITIVE_INFINITY,
            avg_latency_24h_ms: 480,
            availability_24h: 99.5,
            last_checked_at: null,
            timeline: [
              { status: 'operational', latency_ms: 420, checked_at: '2026-08-12T10:00:00Z' },
              { status: 'failed', checked_at: null },
              null,
            ],
          },
          { group_id: 20, model: null },
          { group_id: 0, model: 'invalid-group' },
          null,
        ],
        updated_at: null,
      },
    })

    const result = await list({ signal })

    expect(get).toHaveBeenCalledWith('/model-status', { signal })
    expect(result.updated_at).toBeNull()
    expect(result.items).toHaveLength(1)
    expect(result.items[0]).toMatchObject({
      group_id: 10,
      group_name: '',
      model: 'gpt-4o',
      display_name: 'gpt-4o',
      status: 'unknown',
      message_code: 'no_data',
      latest_latency_ms: null,
      avg_latency_24h_ms: 480,
      availability_24h: 99.5,
      last_checked_at: null,
    })
    expect(result.items[0].timeline).toEqual([
      {
        status: 'operational',
        latency_ms: 420,
        ping_latency_ms: null,
        checked_at: '2026-08-12T10:00:00Z',
      },
    ])
  })

  it('returns an empty recoverable payload for a malformed list envelope', async () => {
    get.mockResolvedValue({ data: null })

    await expect(list()).resolves.toEqual({ items: [], updated_at: null })
  })

  it('rejects a malformed detail payload so the view can retain its list-row fallback', async () => {
    const signal = new AbortController().signal
    get.mockResolvedValue({ data: { group_id: 10, model: null } })

    await expect(detail('gpt-4o', 10, { signal })).rejects.toThrow('Invalid model status detail response')
    expect(get).toHaveBeenCalledWith('/model-status/detail', {
      params: { model: 'gpt-4o', group_id: 10 },
      signal,
    })
  })
})
