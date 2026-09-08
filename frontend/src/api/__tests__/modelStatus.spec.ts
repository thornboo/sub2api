import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import { detail, fetchModelSelfCheckChain, list } from '@/api/modelStatus'

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
            reason_code: 'no_available_account',
            message_code: 'future-message',
            latest_latency_ms: Number.POSITIVE_INFINITY,
            avg_latency_24h_ms: 480,
            availability_24h: 99.5,
            last_checked_at: null,
            timeline: [
              { status: 'operational', reason_code: 'ok', latency_ms: 420, checked_at: '2026-08-12T10:00:00Z' },
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
      reason_code: 'no_available_account',
      message_code: 'no_data',
      latest_latency_ms: null,
      avg_latency_24h_ms: 480,
      availability_24h: 99.5,
      last_checked_at: null,
    })
    expect(result.items[0].timeline).toEqual([
      {
        status: 'operational',
        reason_code: 'ok',
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

  it('normalizes admin self-check chain payloads without exposing them through public calls', async () => {
    const signal = new AbortController().signal
    get.mockResolvedValue({
      data: {
        group_id: 10,
        group_name: 'DeepSeek',
        model: ' deepseek-pro ',
        updated_at: '2026-09-07T10:00:00Z',
        attempt_timeout_seconds: 30,
        round_timeout_seconds: 90,
        candidates: [
          {
            account_id: 3,
            account_name: 'account-c',
            priority: 2,
            platform: 'deepseek',
            order: 1,
            eligible: true,
            reason_code: 'eligible',
            last_checked_at: '2026-09-07T09:59:00Z',
            last_status: 'future',
          },
          {
            account_id: 2,
            account_name: 'account-b',
            priority: 3,
            platform: 'deepseek',
            order: 2,
            eligible: true,
            reason_code: '',
            last_checked_at: null,
            last_status: 'operational',
          },
          { account_id: 'bad' },
        ],
        latest_round: {
          id: 99,
          group_id: 10,
          model: 'deepseek-pro',
          status: 'degraded',
          reason_code: 'failover_success',
          winner_account_id: 2,
          started_at: '2026-09-07T09:58:00Z',
          finished_at: '2026-09-07T09:58:03Z',
          duration_ms: 3000,
          steps: [
            {
              account_id: 3,
              account_name: 'account-c',
              priority: 2,
              platform: 'deepseek',
              order: 1,
              outcome: 'failed',
              reason_code: 'probe_failed',
              started_at: '2026-09-07T09:58:00Z',
              finished_at: '2026-09-07T09:58:01Z',
              latency_ms: 1000,
              http_status: 500,
              error_code: 'upstream_error',
            },
            {
              account_id: 2,
              account_name: 'account-b',
              priority: 3,
              platform: 'deepseek',
              order: 2,
              outcome: 'succeeded',
              reason_code: '',
              started_at: '2026-09-07T09:58:01Z',
              finished_at: '2026-09-07T09:58:03Z',
              latency_ms: 2000,
              http_status: 200,
              error_code: '',
            },
            { account_id: 1, outcome: 'future', started_at: '' },
          ],
        },
      },
    })

    const result = await fetchModelSelfCheckChain(10, 'deepseek-pro', { signal })

    expect(get).toHaveBeenCalledWith('/admin/model-self-check/chain', {
      params: { group_id: 10, model: 'deepseek-pro' },
      signal,
    })
    expect(result).toMatchObject({
      group_id: 10,
      group_name: 'DeepSeek',
      model: 'deepseek-pro',
      attempt_timeout_seconds: 30,
      round_timeout_seconds: 90,
    })
    expect(result.candidates).toHaveLength(2)
    expect(result.candidates[0]).toMatchObject({
      account_id: 3,
      last_status: 'unknown',
    })
    expect(result.latest_round?.steps).toHaveLength(3)
    expect(result.latest_round?.steps[0]).toMatchObject({
      account_id: 3,
      outcome: 'failed',
      http_status: 500,
    })
    expect(result.latest_round?.steps[2]).toMatchObject({
      account_id: 1,
      outcome: 'skipped',
    })
  })

  it('rejects malformed admin self-check chain envelopes', async () => {
    get.mockResolvedValue({ data: { group_id: 10, model: '' } })

    await expect(fetchModelSelfCheckChain(10, 'deepseek-pro')).rejects.toThrow('Invalid model self-check chain response')
  })

  it('keeps interrupted attempts distinct from unattempted backups', async () => {
    get.mockResolvedValue({ data: {
      group_id: 10, model: 'deepseek-pro', candidates: [],
      updated_at: '2026-09-07T00:02:00Z',
      latest_round: {
        id: 5, group_id: 10, model: 'deepseek-pro', status: 'unknown',
        started_at: '2026-09-07T00:00:00Z', finished_at: '2026-09-07T00:02:00Z',
        steps: [
          { account_id: 3, outcome: 'incomplete', reason_code: 'round_incomplete', started_at: '2026-09-07T00:00:00Z' },
          { account_id: 2, outcome: 'not_attempted', reason_code: 'round_incomplete', started_at: null },
        ],
      },
    } })
    const chain = await fetchModelSelfCheckChain(10, 'deepseek-pro')
    expect(chain.latest_round?.steps.map(step => step.outcome)).toEqual(['incomplete', 'not_attempted'])
  })
})
