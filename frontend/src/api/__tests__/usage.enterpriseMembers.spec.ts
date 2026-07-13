import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get },
}))

import {
  getOwnerMemberAnalyticsLeaderboard,
  listOwnerUsageMembers,
  type OwnerMemberLeaderboardResponse,
  type OwnerUsageMembersResponse,
} from '@/api/usage'

describe('enterprise member usage API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('loads the current owner member directory including archived identities', async () => {
    const response: OwnerUsageMembersResponse = {
      members: [{
        id: 42,
        member_code: 'finance-01',
        name: 'Finance',
        status: 'active',
        archived: true,
        key_count: 0,
        monthly_limit_usd: 100,
      }],
    }
    get.mockResolvedValue({ data: response })

    await expect(listOwnerUsageMembers()).resolves.toEqual(response)
    expect(get).toHaveBeenCalledWith('/usage/members')
  })

  it('passes the immutable member selector and secondary key filter to member analytics', async () => {
    const response: OwnerMemberLeaderboardResponse = {
      items: [],
      total: 0,
      member_count: 0,
      budget_risk_member_count: 0,
      total_reserved_usd: 0,
      total_actual_cost: 0,
      displayed_actual_cost: 0,
      start_date: '2026-07-01',
      end_date: '2026-07-14',
      timezone: 'Asia/Shanghai',
      granularity: 'day',
    }
    const signal = new AbortController().signal
    get.mockResolvedValue({ data: response })

    await expect(getOwnerMemberAnalyticsLeaderboard({
      member_id: 42,
      api_key_id: 7,
      start_date: '2026-07-01',
      end_date: '2026-07-14',
    }, { signal })).resolves.toEqual(response)

    expect(get).toHaveBeenCalledWith('/usage/analytics/members', {
      signal,
      params: {
        member_id: 42,
        api_key_id: 7,
        start_date: '2026-07-01',
        end_date: '2026-07-14',
      },
    })
  })
})
