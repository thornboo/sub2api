import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageView from '../UsageView.vue'

const { list, getStats, searchApiKeys, getSnapshotV2, getById, getUserApiKeys, getModelStats, listErrorLogs, routerReplace, routeQuery } = vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  })

  return {
    list: vi.fn(),
    getStats: vi.fn(),
    searchApiKeys: vi.fn(),
    getSnapshotV2: vi.fn(),
    getById: vi.fn(),
    getUserApiKeys: vi.fn(),
    getModelStats: vi.fn(),
    listErrorLogs: vi.fn(),
    routerReplace: vi.fn(),
    routeQuery: {} as Record<string, string | Array<string | null> | null | undefined>,
  }
})

const messages: Record<string, string> = {
  'admin.dashboard.timeRange': 'Time Range',
  'admin.dashboard.day': 'Day',
  'admin.dashboard.hour': 'Hour',
  'admin.dashboard.month': 'Month',
  'admin.dashboard.modelDistribution': 'Model Distribution',
  'admin.usage.failedToLoadUser': 'Failed to load user',
}

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: {
      list,
      getStats,
      searchApiKeys,
    },
    dashboard: {
      getSnapshotV2,
      getModelStats,
    },
    users: {
      getById,
      getUserApiKeys,
    },
  },
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/admin/ops', () => ({
  listErrorLogs,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/utils/format', () => ({
  formatReasoningEffort: (value: string | null | undefined) => value ?? '-',
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => ({
    path: '/admin/usage',
    query: routeQuery,
  }),
  useRouter: () => ({
    replace: routerReplace,
  }),
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const UsageFiltersStub = { template: '<div><slot name="after-reset" /></div>' }
const UsageProfileHeaderSelectStub = {
  emits: ['selectUser', 'selectApiKey'],
  template: `
    <div data-test="usage-profile-header">
      <button data-test="select-profile-user" @click="$emit('selectUser', { id: 7, email: 'ops@example.com', deleted: false })">select user</button>
      <button data-test="select-profile-key" @click="$emit('selectApiKey', { id: 11, user_id: 7, name: 'ops-key' })">select key</button>
    </div>
  `,
}
const UsageProfileHeaderControlsStub = {
  template: '<div data-test="usage-profile-header"><slot name="controls" /></div>',
}
const UsageTableStub = {
  emits: ['userClick'],
  template: '<div data-test="usage-table"><button class="user-click" @click="$emit(\'userClick\', 2)">user</button></div>',
}
const DateRangePickerLastMonthStub = {
  emits: ['update:startDate', 'update:endDate', 'change'],
  template: `
    <button
      data-test="select-last-month"
      @click="
        $emit('update:startDate', '2026-05-01');
        $emit('update:endDate', '2026-05-31');
        $emit('change', { startDate: '2026-05-01', endDate: '2026-05-31', preset: 'lastMonth' })
      "
    >
      last month
    </button>
  `,
}

const UserTokenRankingStub = {
  emits: ['select-user'],
  template: '<div data-test="ranking"><button class="pick-user" @click="$emit(\'select-user\', 5, \'rank@test.com\')">pick</button></div>',
}
const OpsErrorLogTableContractStub = {
  name: 'OpsErrorLogTable',
  props: ['virtualScroll', 'stickyHeader', 'stickyFirstColumn', 'stickyActionsColumn'],
  template: '<div data-test="ops-error-log-table-contract" />',
}
const ModelDistributionChartStub = {
  props: ['metric', 'showExpandButton'],
  emits: ['update:metric', 'expand'],
  template: `
    <div data-test="model-chart">
      <span class="metric">{{ metric }}</span>
      <button class="switch-metric" @click="$emit('update:metric', 'actual_cost')">switch</button>
      <button v-if="showExpandButton" class="expand-chart" @click="$emit('expand')">expand</button>
    </div>
  `,
}
const GroupDistributionChartStub = {
  props: ['metric', 'showExpandButton'],
  emits: ['update:metric', 'expand'],
  template: `
    <div data-test="group-chart">
      <span class="metric">{{ metric }}</span>
      <button class="switch-metric" @click="$emit('update:metric', 'actual_cost')">switch</button>
      <button v-if="showExpandButton" class="expand-chart" @click="$emit('expand')">expand</button>
    </div>
  `,
}
const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="expanded-chart-modal"><h2>{{ title }}</h2><slot /></div>',
}

beforeEach(() => {
  Object.keys(routeQuery).forEach((key) => {
    delete routeQuery[key]
  })
})

describe('admin UsageView pagination contract', () => {
  afterEach(() => {
    vi.useRealTimers()
    delete window.__APP_CONFIG__
  })

  it('caps a configured 1000-row global page size at 100 for the detailed usage table', async () => {
    vi.useFakeTimers()
    window.__APP_CONFIG__ = {
      table_default_page_size: 1000,
      table_page_size_options: [20, 50, 1000],
    } as any
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getModelStats.mockReset()
    list.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStats.mockResolvedValue({
      total_requests: 0, total_input_tokens: 0, total_output_tokens: 0,
      total_cache_tokens: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0,
    })
    getSnapshotV2.mockResolvedValue({ trend: [], models: [], groups: [] })
    getModelStats.mockResolvedValue({ models: [] })

    const wrapper = mount(UsageView, {
      global: { stubs: {
        AppLayout: AppLayoutStub, UsageStatsCards: true, UsageFilters: UsageFiltersStub,
        UsageTable: true, UsageExportProgress: true, UsageCleanupDialog: true,
        UserBalanceHistoryModal: true, AuditLogModal: true, Pagination: true, Select: true,
        DateRangePicker: true, Icon: true, TokenUsageTrend: true,
        ModelDistributionChart: true, GroupDistributionChart: true, EndpointDistributionChart: true,
        UserTokenRanking: true, OpsErrorLogTable: true, OpsErrorDetailModal: true,
      } },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(list).toHaveBeenCalledWith(
      expect.objectContaining({ page_size: 100 }),
      expect.anything(),
    )
    wrapper.unmount()
  })
})

describe('admin UsageView route query sanitization', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getById.mockReset()
    searchApiKeys.mockReset()
    getUserApiKeys.mockReset()
    getModelStats.mockReset()
    routerReplace.mockReset()

    list.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    getSnapshotV2.mockResolvedValue({ trend: [], models: [], groups: [] })
    getModelStats.mockResolvedValue({ models: [] })
    searchApiKeys.mockResolvedValue([])
    getUserApiKeys.mockResolvedValue({ items: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('silently normalizes dirty route query values on initial load', async () => {
    Object.assign(routeQuery, {
      user_id: '12',
      api_key_id: '0',
      model: ' claude-sonnet ',
      request_type: 'chat',
      billing_mode: 'token',
      start_date: '2026-06-01',
      end_date: '2026-02-31',
    })

    mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({
      path: '/admin/usage',
      query: {
        user_id: '12',
        model: 'claude-sonnet',
        billing_mode: 'token',
        start_date: '2026-06-01',
      },
    })
  })

  it('auto-selects monthly chart granularity for wide route date ranges', async () => {
    Object.assign(routeQuery, {
      start_date: '2026-01-01',
      end_date: '2026-06-15',
    })

    mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageProfileHeader: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-01-01',
      end_date: '2026-06-15',
      granularity: 'month',
    }))
  })

  it('syncs date range picker changes back to the route query', async () => {
    Object.assign(routeQuery, {
      start_date: '2026-06-01',
      end_date: '2026-06-15',
    })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageProfileHeader: UsageProfileHeaderControlsStub,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: DateRangePickerLastMonthStub,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()
    routerReplace.mockClear()
    list.mockClear()
    getStats.mockClear()
    getSnapshotV2.mockClear()

    await wrapper.find('[data-test="select-last-month"]').trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({
      path: '/admin/usage',
      query: {
        start_date: '2026-05-01',
        end_date: '2026-05-31',
      },
    })
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-05-01',
      end_date: '2026-05-31',
    }), expect.any(Object))
    expect(getStats).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-05-01',
      end_date: '2026-05-31',
    }))
    expect(getSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-05-01',
      end_date: '2026-05-31',
      granularity: 'day',
    }))
  })

  it('resolves route API key labels by exact id including deleted keys', async () => {
    Object.assign(routeQuery, {
      user_id: '7',
      api_key_id: '11',
    })
    getById.mockResolvedValue({ id: 7, email: 'ops@example.com' })
    searchApiKeys.mockResolvedValue([{ id: 11, user_id: 7, name: 'deleted-key', deleted: true }])

    mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageProfileHeader: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(searchApiKeys).toHaveBeenCalledWith(7, '', {
      includeDeleted: true,
      apiKeyId: 11,
    })
  })

  it('resolves standalone route API key labels by exact id', async () => {
    Object.assign(routeQuery, {
      api_key_id: '11',
    })
    searchApiKeys.mockResolvedValue([{ id: 11, user_id: 7, name: 'deleted-key', deleted: true }])

    mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageProfileHeader: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(searchApiKeys).toHaveBeenCalledWith(undefined, '', {
      includeDeleted: true,
      apiKeyId: 11,
    })
  })

  it('keeps the current monthly granularity when switching profile objects', async () => {
    Object.assign(routeQuery, {
      start_date: '2026-01-01',
      end_date: '2026-06-15',
    })
    getById.mockResolvedValue({ id: 7, email: 'ops@example.com' })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageProfileHeader: UsageProfileHeaderSelectStub,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()
    getSnapshotV2.mockClear()

    await wrapper.find('[data-test="select-profile-user"]').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      user_id: 7,
      granularity: 'month',
    }))
  })

  it('applies top profile header user and API key selections to route and usage requests', async () => {
    getById.mockResolvedValue({ id: 7, email: 'ops@example.com' })
    searchApiKeys.mockResolvedValue([{ id: 11, user_id: 7, name: 'ops-key' }])

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageProfileHeader: UsageProfileHeaderSelectStub,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          UserApiKeysModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    await flushPromises()

    await wrapper.find('[data-test="select-profile-user"]').trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      path: '/admin/usage',
      query: expect.objectContaining({
        user_id: '7',
      }),
    })
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({
      user_id: 7,
    }), expect.any(Object))
    expect(list.mock.calls.at(-1)?.[0].api_key_id).toBeUndefined()
    expect(getStats).toHaveBeenLastCalledWith(expect.objectContaining({
      user_id: 7,
    }))
    expect(getStats.mock.calls.at(-1)?.[0].api_key_id).toBeUndefined()

    await wrapper.find('[data-test="select-profile-key"]').trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      path: '/admin/usage',
      query: expect.objectContaining({
        user_id: '7',
        api_key_id: '11',
      }),
    })
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({
      user_id: 7,
      api_key_id: 11,
    }), expect.any(Object))
    expect(getStats).toHaveBeenLastCalledWith(expect.objectContaining({
      user_id: 7,
      api_key_id: 11,
    }))
  })
})

describe('admin UsageView distribution metric toggles', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getById.mockReset()
    searchApiKeys.mockReset()
    getUserApiKeys.mockReset()
    getModelStats.mockReset()
    routerReplace.mockReset()

    list.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    getSnapshotV2.mockResolvedValue({
      trend: [],
      models: [],
      groups: [],
    })
    getModelStats.mockResolvedValue({ models: [] })
    searchApiKeys.mockResolvedValue([])
    getUserApiKeys.mockResolvedValue({ items: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('keeps previous model stats visible during refresh until new data arrives', async () => {
    // 首次加载返回 A
    getModelStats.mockResolvedValueOnce({ models: [{ model: 'A', total_tokens: 10 }] })

    const wrapper = mount(UsageView, {
      global: { stubs: {
        AppLayout: AppLayoutStub, UsageStatsCards: true, UsageFilters: UsageFiltersStub,
        UsageTable: true, UsageExportProgress: true, UsageCleanupDialog: true,
        UserBalanceHistoryModal: true, AuditLogModal: true, Pagination: true, Select: true,
        DateRangePicker: true, Icon: true, TokenUsageTrend: true,
        ModelDistributionChart: ModelDistributionChartStub, GroupDistributionChart: GroupDistributionChartStub,
        EndpointDistributionChart: true,
        OpsErrorLogTable: true, OpsErrorDetailModal: true,
        UserTokenRanking: true,
      } },
    })
    vi.advanceTimersByTime(120)
    await flushPromises()
    expect((wrapper.vm as any).requestedModelStats).toEqual([{ model: 'A', total_tokens: 10 }])

    // 刷新:让第二次 getModelStats 处于 pending,断言旧数据 A 仍在(不被清空成 [])
    let resolveSecond: (v: any) => void = () => {}
    getModelStats.mockReturnValueOnce(new Promise((res) => { resolveSecond = res }))
    ;(wrapper.vm as any).refreshData()
    await flushPromises()
    expect((wrapper.vm as any).requestedModelStats).toEqual([{ model: 'A', total_tokens: 10 }])

    // 新数据到达后替换为 B
    resolveSecond({ models: [{ model: 'B', total_tokens: 20 }] })
    await flushPromises()
    expect((wrapper.vm as any).requestedModelStats).toEqual([{ model: 'B', total_tokens: 20 }])
  })

  it('keeps model and group metric toggles independent without refetching chart data', async () => {
    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
          UserTokenRanking: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))

    const modelChart = wrapper.find('[data-test="model-chart"]')
    const groupChart = wrapper.find('[data-test="group-chart"]')

    expect(modelChart.find('.metric').text()).toBe('tokens')
    expect(groupChart.find('.metric').text()).toBe('tokens')

    await modelChart.find('.switch-metric').trigger('click')
    await flushPromises()

    expect(modelChart.find('.metric').text()).toBe('actual_cost')
    expect(groupChart.find('.metric').text()).toBe('tokens')
    expect(getSnapshotV2).toHaveBeenCalledTimes(1)

    await groupChart.find('.switch-metric').trigger('click')
    await flushPromises()

    expect(modelChart.find('.metric').text()).toBe('actual_cost')
    expect(groupChart.find('.metric').text()).toBe('actual_cost')
    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
  })

  it('opens the expanded chart modal from a chart header action', async () => {
    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
          EndpointDistributionChart: true,
          BaseDialog: BaseDialogStub,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(wrapper.find('[data-test="expanded-chart-modal"]').exists()).toBe(false)

    await wrapper.find('[data-test="model-chart"] .expand-chart').trigger('click')
    await flushPromises()

    const modal = wrapper.find('[data-test="expanded-chart-modal"]')
    expect(modal.exists()).toBe(true)
    expect(modal.text()).toContain('Model Distribution')
  })
})

describe('admin UsageView handleUserClick', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getById.mockReset()

    list.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStats.mockResolvedValue({
      total_requests: 0, total_input_tokens: 0, total_output_tokens: 0,
      total_cache_tokens: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0,
    })
    getSnapshotV2.mockResolvedValue({ trend: [], models: [], groups: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('opens user via include_deleted when clicking a usage row user', async () => {
    getById.mockResolvedValue({ id: 2, email: 'd@test.com', deleted_at: '2026-05-28T00:00:00Z' })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: UsageTableStub,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          AuditLogModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: true,
          GroupDistributionChart: true,
          EndpointDistributionChart: true,
          OpsErrorLogTable: true,
          OpsErrorDetailModal: true,
          UserTokenRanking: true,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    await wrapper.find('[data-test="usage-table"] .user-click').trigger('click')
    await flushPromises()

    expect(getById).toHaveBeenCalledWith(2, true)
  })
})

describe('admin UsageView errors tab filter forwarding', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getModelStats.mockReset()
    listErrorLogs.mockReset()

    list.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStats.mockResolvedValue({
      total_requests: 0, total_input_tokens: 0, total_output_tokens: 0,
      total_cache_tokens: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0,
    })
    getSnapshotV2.mockResolvedValue({ trend: [], models: [], groups: [] })
    getModelStats.mockResolvedValue({ models: [] })
    listErrorLogs.mockResolvedValue({ items: [], total: 0, pages: 0 })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('uses the same stable native table flow as the usage detail tab', async () => {
    const wrapper = mount(UsageView, {
      global: { stubs: {
        AppLayout: AppLayoutStub, UsageStatsCards: true, UsageFilters: UsageFiltersStub,
        UsageTable: true, UsageExportProgress: true, UsageCleanupDialog: true,
        UserBalanceHistoryModal: true, AuditLogModal: true, Pagination: true, Select: true,
        DateRangePicker: true, Icon: true, TokenUsageTrend: true,
        ModelDistributionChart: true, GroupDistributionChart: true, EndpointDistributionChart: true,
        UserTokenRanking: true, OpsErrorLogTable: OpsErrorLogTableContractStub, OpsErrorDetailModal: true,
      } },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    const errorTable = wrapper.findComponent(OpsErrorLogTableContractStub)
    expect(errorTable.props('virtualScroll')).toBe(false)
    expect(errorTable.props('stickyHeader')).toBe(false)
    expect(errorTable.props('stickyFirstColumn')).toBe(false)
    expect(errorTable.props('stickyActionsColumn')).toBe(false)
  })

  it('forwards model/account_id/group_id to listErrorLogs on the errors tab', async () => {
    const wrapper = mount(UsageView, {
      global: { stubs: {
        AppLayout: AppLayoutStub, UsageStatsCards: true, UsageFilters: UsageFiltersStub,
        UsageTable: true, UsageExportProgress: true, UsageCleanupDialog: true,
        UserBalanceHistoryModal: true, AuditLogModal: true, Pagination: true, Select: true,
        DateRangePicker: true, Icon: true, TokenUsageTrend: true,
        ModelDistributionChart: true, GroupDistributionChart: true, EndpointDistributionChart: true,
        UserTokenRanking: true, OpsErrorLogTable: true, OpsErrorDetailModal: true,
      } },
    })
    vi.advanceTimersByTime(120)
    await flushPromises()

    // 模拟用户在过滤器里选择了模型/账户/分组
    const vm = wrapper.vm as any
    vm.filters.model = 'gpt-5.3-codex'
    vm.filters.account_id = 7
    vm.filters.group_id = 3
    await flushPromises()

    // 切换到「错误请求」标签（第二个 tab 按钮）触发 loadAdminErrors
    const tabs = wrapper.findAll('[data-testid="usage-detail-tab"]')
    await tabs[1].trigger('click')
    await flushPromises()

    expect(listErrorLogs).toHaveBeenCalledWith(expect.objectContaining({
      view: 'all',
      model: 'gpt-5.3-codex',
      account_id: 7,
      group_id: 3,
    }))
  })
})

describe('admin UsageView ranking tab', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getModelStats.mockReset()

    list.mockResolvedValue({ items: [], total: 0, pages: 0 })
    getStats.mockResolvedValue({
      total_requests: 0, total_input_tokens: 0, total_output_tokens: 0,
      total_cache_tokens: 0, total_tokens: 0, total_cost: 0, total_actual_cost: 0, average_duration_ms: 0,
    })
    getSnapshotV2.mockResolvedValue({ trend: [], models: [], groups: [] })
    getModelStats.mockResolvedValue({ models: [] })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('mounts ranking lazily and drill-down sets user filter then jumps back to usage tab', async () => {
    const wrapper = mount(UsageView, {
      global: { stubs: {
        AppLayout: AppLayoutStub, UsageStatsCards: true, UsageFilters: UsageFiltersStub,
        UsageTable: true, UsageExportProgress: true, UsageCleanupDialog: true,
        UserBalanceHistoryModal: true, Pagination: true, Select: true,
        DateRangePicker: true, Icon: true, TokenUsageTrend: true,
        ModelDistributionChart: true, GroupDistributionChart: true, EndpointDistributionChart: true,
        UserTokenRanking: UserTokenRankingStub, OpsErrorLogTable: true, OpsErrorDetailModal: true,
      } },
    })
    vi.advanceTimersByTime(120)
    await flushPromises()

    // 懒挂载:切到排行 tab 前不渲染
    expect(wrapper.find('[data-test="ranking"]').exists()).toBe(false)

    const tabs = wrapper.findAll('[data-testid="usage-detail-tab"]')
    expect(tabs).toHaveLength(3)
    await tabs[2].trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="ranking"]').exists()).toBe(true)

    // 下钻:设置 user_id、切回用量明细 tab 并按新筛选重新拉取列表
    list.mockClear()
    await wrapper.find('[data-test="ranking"] .pick-user').trigger('click')
    await flushPromises()

    expect((wrapper.vm as any).activeTab).toBe('usage')
    expect((wrapper.vm as any).filters.user_id).toBe(5)
    expect(list).toHaveBeenCalledWith(expect.objectContaining({ user_id: 5 }), expect.anything())
  })
})
