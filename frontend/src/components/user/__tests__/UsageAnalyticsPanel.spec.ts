import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import UsageAnalyticsPanel from '../UsageAnalyticsPanel.vue'

const {
  getSummary,
  getKeyLeaderboard,
  getMemberLeaderboard,
  getModels,
  getGroups,
  getTags,
  getTrend,
} = vi.hoisted(() => ({
  getSummary: vi.fn(),
  getKeyLeaderboard: vi.fn(),
  getMemberLeaderboard: vi.fn(),
  getModels: vi.fn(),
  getGroups: vi.fn(),
  getTags: vi.fn(),
  getTrend: vi.fn(),
}))

vi.mock('@/api', () => ({
  usageAPI: {
    getOwnerApiKeyAnalyticsSummary: getSummary,
    getOwnerApiKeyAnalyticsLeaderboard: getKeyLeaderboard,
    getOwnerMemberAnalyticsLeaderboard: getMemberLeaderboard,
    getOwnerApiKeyModelAnalytics: getModels,
    getOwnerApiKeyGroupAnalytics: getGroups,
    getOwnerApiKeyTagAnalytics: getTags,
    getOwnerApiKeyUsageTrend: getTrend,
  },
}))

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  BarElement: {},
  CategoryScale: {},
  Legend: {},
  LinearScale: {},
  Tooltip: {},
}))

vi.mock('vue-chartjs', async () => {
  const { defineComponent } = await import('vue')
  return {
    Bar: defineComponent({
      name: 'Bar',
      props: {
        data: { type: Object, required: true },
        options: { type: Object, default: () => ({}) },
      },
      template: '<div class="bar-stub" />',
    }),
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const analyticsMeta = {
  start_time: '2026-07-01T00:00:00Z',
  end_time: '2026-07-08T00:00:00Z',
  timezone: 'UTC',
  granularity: 'day',
}

function memberResponse(memberID?: number) {
  return {
    ...analyticsMeta,
    items: [],
    total: memberID ? 1 : 3,
    member_count: memberID ? 1 : 3,
    budget_risk_member_count: memberID ? 0 : 2,
    total_reserved_usd: memberID ? 2 : 12,
    total_actual_cost: 0,
    displayed_actual_cost: 0,
  }
}

function memberLeaderboardItem(memberID: number | null, name: string) {
  return {
    member_id: memberID,
    member_code: memberID ? 'finance-01' : '',
    member_name: name,
    status: memberID ? 'active' : 'unassigned',
    archived: false,
    key_count: memberID ? 1 : 3,
    monthly_limit_usd: memberID ? 100 : 0,
    current_used_usd: 0,
    current_reserved_usd: 0,
    requests: memberID ? 2 : 5,
    input_tokens: 10,
    output_tokens: 5,
    cache_creation_tokens: 0,
    cache_read_tokens: 0,
    total_tokens: 15,
    actual_cost: memberID ? 1 : 2,
    share_percent: memberID ? 100 : 0,
    previous_actual_cost: 0,
    change_percent: 0,
  }
}

function metricCardText(wrapper: ReturnType<typeof mount>, label: string): string {
  const labelNode = wrapper.findAll('div').find((node) => node.text() === label)
  return labelNode?.element.parentElement?.textContent || ''
}

describe('UsageAnalyticsPanel member metrics', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getSummary.mockReset().mockResolvedValue({
      ...analyticsMeta,
      summary: {
        requests: 0,
        input_tokens: 0,
        output_tokens: 0,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 0,
        actual_cost: 0,
        used_key_count: 0,
        current_key_snapshot: {
          active_key_count: 0,
          near_quota_key_count: 0,
          near_rate_limit_key_count: 0,
          snapshot_at: '2026-07-13T00:00:00Z',
        },
      },
    })
    getMemberLeaderboard.mockReset().mockImplementation((params: { member_id?: number }) =>
      Promise.resolve(memberResponse(params.member_id)),
    )
    getTrend.mockReset().mockResolvedValue({ ...analyticsMeta, items: [] })
    getKeyLeaderboard.mockReset().mockResolvedValue({ ...analyticsMeta, items: [], total: 0, total_actual_cost: 0, displayed_actual_cost: 0 })
    getModels.mockReset().mockResolvedValue({ ...analyticsMeta, models: [] })
    getGroups.mockReset().mockResolvedValue({ ...analyticsMeta, groups: [] })
    getTags.mockReset().mockResolvedValue({ ...analyticsMeta, tags: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('refreshes member KPI metadata while the trend tab is active', async () => {
    const wrapper = mount(UsageAnalyticsPanel, {
      props: {
        enterprise: true,
        startDate: '2026-07-01',
        endDate: '2026-07-08',
        members: [{ id: 42, member_code: 'finance-01', name: 'Finance', status: 'active', archived: false, key_count: 1, monthly_limit_usd: 100 }],
      },
      global: {
        stubs: {
          Icon: true,
          Select: true,
        },
      },
    })
    await flushPromises()
    expect(metricCardText(wrapper, 'usage.analytics.memberCount')).toContain('3')
    expect(metricCardText(wrapper, 'usage.analytics.budgetRiskMembers')).toContain('2')
    expect(metricCardText(wrapper, 'usage.analytics.reservedBudget')).toContain('$12.0000')

    const trendTab = wrapper.findAll('button').find((button) => button.text() === 'usage.analytics.tabs.trend')
    expect(trendTab).toBeDefined()
    await trendTab!.trigger('click')
    vi.advanceTimersByTime(300)
    await flushPromises()

    await wrapper.setProps({ memberFilter: 'member:42' })
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(getTrend).toHaveBeenLastCalledWith(
      expect.objectContaining({ member_id: 42 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(getMemberLeaderboard).toHaveBeenLastCalledWith(
      expect.objectContaining({ member_id: 42, limit: 1 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(metricCardText(wrapper, 'usage.analytics.memberCount')).toContain('1')
    expect(metricCardText(wrapper, 'usage.analytics.budgetRiskMembers')).toContain('0')
    expect(metricCardText(wrapper, 'usage.analytics.reservedBudget')).toContain('$2.0000')
  })

  it('keeps the regular-key compatibility bucket out of the member ranking', async () => {
    getMemberLeaderboard.mockResolvedValueOnce({
      ...memberResponse(),
      items: [
        memberLeaderboardItem(null, ''),
        memberLeaderboardItem(42, 'Finance'),
      ],
    })

    const wrapper = mount(UsageAnalyticsPanel, {
      props: {
        enterprise: true,
        startDate: '2026-07-01',
        endDate: '2026-07-08',
        members: [{ id: 42, member_code: 'finance-01', name: 'Finance', status: 'active', archived: false, key_count: 1, monthly_limit_usd: 100 }],
      },
      global: {
        stubs: {
          Icon: true,
          Select: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Finance')
    expect(wrapper.text()).not.toContain('usage.members.unassignedShort')
  })

  it('keeps the dedicated member module scoped to assigned usage until a member is selected', async () => {
    const wrapper = mount(UsageAnalyticsPanel, {
      props: {
        enterprise: true,
        memberCentric: true,
        memberScope: 'assigned',
        startDate: '2026-07-01',
        endDate: '2026-07-08',
        members: [{ id: 42, member_code: 'finance-01', name: 'Finance', status: 'active', archived: false, key_count: 1, monthly_limit_usd: 100 }],
      },
      global: {
        stubs: {
          Icon: true,
          Select: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.findAll('button').some((button) => button.text() === 'usage.analytics.dimensionKey')).toBe(false)
    expect(wrapper.findAll('label').some((label) => label.text().includes('usage.analytics.apiKey'))).toBe(false)
    expect(wrapper.findAll('label').some((label) => label.text().includes('usage.memberFilter'))).toBe(false)
    expect(wrapper.text()).toContain('usage.analytics.memberTitle')
    expect(getKeyLeaderboard).not.toHaveBeenCalled()

    expect(getSummary).toHaveBeenLastCalledWith(
      expect.objectContaining({ member_scope: 'assigned' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    expect(getMemberLeaderboard).toHaveBeenLastCalledWith(
      expect.objectContaining({ member_scope: 'assigned' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )

    await wrapper.setProps({ memberFilter: 'member:42' })
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(getSummary).toHaveBeenLastCalledWith(
      expect.objectContaining({ member_id: 42 }),
      expect.objectContaining({ signal: expect.any(AbortSignal) }),
    )
    const params = getSummary.mock.calls.at(-1)?.[0] as Record<string, unknown>
    expect(params).not.toHaveProperty('member_scope')
  })
})
