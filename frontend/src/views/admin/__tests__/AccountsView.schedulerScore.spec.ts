import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  listUpstreamSuppliers,
  listUpstreamCostPools,
  listUpstreamCostPoolAccounts,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  listUpstreamSuppliers: vi.fn(),
  listUpstreamCostPools: vi.fn(),
  listUpstreamCostPoolAccounts: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      listUpstreamSuppliers,
      listUpstreamCostPools,
      listUpstreamCostPoolAccounts,
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

// Render the scheduler-score cell slot for every row so the fallback logic is observable.
const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id">
        <div :data-test="'scheduler-score-' + row.id">
          <slot name="cell-scheduler_score" :row="row" />
        </div>
        <div :data-test="'upstream-discount-' + row.id">
          <slot name="cell-upstream_effective_discount" :row="row" />
        </div>
      </div>
    </div>
  `
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        HelpTooltip: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        UpstreamSupplierModal: true,
        UpstreamCostComparison: {
          props: ['costPools'],
          emits: ['refresh', 'recharge-records'],
          template: `
            <div data-test="upstream-cost-comparison">
              <span data-test="upstream-current-cost">{{ costPools[0]?.current_effective_cny_per_usd ?? '-' }}</span>
              <button data-test="upstream-cost-refresh" @click="$emit('refresh')">refresh costs</button>
              <button v-if="costPools[0]" data-test="open-recharge-records" @click="$emit('recharge-records', costPools[0])">records</button>
            </div>
          `
        },
        UpstreamRechargeRecordsModal: {
          props: ['show'],
          emits: ['pool-updated'],
          template: '<button v-if="show" data-test="emit-pool-updated" @click="$emit(\'pool-updated\')">updated</button>'
        },
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

const baseAccount = {
  platform: 'openai',
  type: 'apikey',
  status: 'active',
  schedulable: true,
  concurrency: 1,
  priority: 0,
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
}

describe('admin AccountsView scheduler score column', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    listUpstreamSuppliers.mockReset()
    listUpstreamCostPools.mockReset()
    listUpstreamCostPoolAccounts.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [
        {
          ...baseAccount,
          id: 1,
          name: 'ungrouped-openai',
          // 未分组账号：后端只返回基础分（scheduler_score），无分组维度分数
          scheduler_score: {
            base_score: 1.234567,
            sticky_score: 0,
            sticky_weighted_enabled: false
          }
        },
        {
          ...baseAccount,
          id: 2,
          name: 'grouped-openai',
          scheduler_score: {
            base_score: 2,
            sticky_score: 3,
            sticky_weighted_enabled: true
          },
          scheduler_scores: [
            {
              group_id: 5,
              group_name: 'group-five',
              base_score: 2,
              sticky_score: 3,
              sticky_weighted_enabled: true
            }
          ]
        },
        {
          ...baseAccount,
          id: 3,
          name: 'no-score',
          platform: 'anthropic'
        }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    listUpstreamSuppliers.mockResolvedValue([])
    listUpstreamCostPools.mockResolvedValue([])
    listUpstreamCostPoolAccounts.mockResolvedValue([])
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('falls back to the base score for ungrouped accounts instead of showing a dash', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_scheduler_score: '0'
    }))

    const ungroupedCell = wrapper.find('[data-test="scheduler-score-1"]')
    expect(ungroupedCell.exists()).toBe(true)
    expect(ungroupedCell.text()).toContain('1.234567')
    expect(ungroupedCell.text()).toContain('admin.accounts.schedulerScore.ungrouped')
    expect(ungroupedCell.text()).not.toBe('-')
  })

  it('renders per-group scores for grouped accounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    const groupedCell = wrapper.find('[data-test="scheduler-score-2"]')
    expect(groupedCell.exists()).toBe(true)
    expect(groupedCell.text()).toContain('group-five')
    expect(groupedCell.text()).toContain('2')
  })

  it('keeps scheduler score hidden for old saved column settings until the admin opts in again', async () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['today_stats']))

    mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_scheduler_score: '0'
    }))
    expect(JSON.parse(localStorage.getItem('account-hidden-columns') || '[]')).toContain('scheduler_score')
  })

  it('requests scheduler scores when the migrated column settings explicitly show the column', async () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['today_stats']))
    localStorage.setItem('account-hidden-columns-version', 'scheduler-score-hidden-by-default')

    mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_scheduler_score: '1'
    }))
  })

  it('still shows a dash when no scheduler score is available', async () => {
    const wrapper = mountView()
    await flushPromises()

    const emptyCell = wrapper.find('[data-test="scheduler-score-3"]')
    expect(emptyCell.exists()).toBe(true)
    expect(emptyCell.text()).toBe('-')
    expect(wrapper.get('[data-test="upstream-discount-3"]').text()).toBe('-')
  })

  it('renders confirmed CNY discount, marks legacy bindings pending, and requires a real snapshot', async () => {
    listAccounts.mockResolvedValue({
      items: [
        { ...baseAccount, id: 11, name: 'kimi-confirmed' },
        { ...baseAccount, id: 12, name: 'legacy-unconfirmed' },
        { ...baseAccount, id: 13, name: 'no-snapshot' }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listUpstreamCostPools.mockResolvedValue([
      {
        id: 21,
        supplier_id: 31,
        supplier_name: 'Supplier A',
        name: '主余额池',
        is_default: true,
        status: 'active',
        reference_fx_rate: 7,
        current_effective_cny_per_usd: 1,
        current_snapshot_id: 41
      },
      {
        id: 22,
        supplier_id: 31,
        supplier_name: 'Supplier A',
        name: '备用池',
        is_default: false,
        status: 'active',
        reference_fx_rate: 7,
        current_effective_cny_per_usd: 1,
        current_snapshot_id: null
      }
    ])
    listUpstreamCostPoolAccounts.mockImplementation(async (poolID: number) => (
      poolID === 21
        ? [
            {
              account_id: 11,
              cost_pool_id: 21,
              status: 'active',
              default_multiplier: 0.8,
              price_reference_currency: 'CNY',
              price_reference_confirmed: true,
              model_family_multipliers: []
            },
            {
              account_id: 12,
              cost_pool_id: 21,
              status: 'active',
              default_multiplier: 0.8,
              price_reference_currency: 'USD',
              price_reference_confirmed: false,
              model_family_multipliers: []
            }
          ]
        : [
            {
              account_id: 13,
              cost_pool_id: 22,
              status: 'active',
              default_multiplier: 0.8,
              price_reference_currency: 'CNY',
              price_reference_confirmed: true,
              model_family_multipliers: []
            }
          ]
    ))

    const wrapper = mountView()
    await flushPromises()

    const confirmedCell = wrapper.get('[data-test="upstream-discount-11"]')
    expect(confirmedCell.text()).toContain('8.0admin.accounts.upstreamCost.discountSuffix')
    expect(confirmedCell.text()).toContain('admin.accounts.upstreamCost.priceReferenceShortCNY')

    const legacyCell = wrapper.get('[data-test="upstream-discount-12"]')
    expect(legacyCell.text()).toContain('admin.accounts.upstreamCost.priceReferencePending')
    expect(legacyCell.text()).toContain('admin.accounts.upstreamCost.priceReferencePendingLegacy')
    expect(legacyCell.text()).not.toContain('1.1')

    const noSnapshotCell = wrapper.get('[data-test="upstream-discount-13"]')
    expect(noSnapshotCell.text()).toContain('-')
  })

  it('forces a fresh pool request after recharge updates and ignores the older in-flight response', async () => {
    const initialPool = {
      id: 9,
      supplier_id: 7,
      supplier_name: 'Supplier A',
      name: '主余额池',
      is_default: true,
      status: 'active',
      reference_fx_rate: 7,
      default_effective_cny_per_usd: 7,
      default_reference_fx_rate: 7,
      cost_method: 'latest',
      current_effective_cny_per_usd: 7,
      current_snapshot_id: 10,
      binding_count: 1,
      record_count: 1
    }
    const refreshedPool = {
      ...initialPool,
      current_effective_cny_per_usd: 5,
      current_snapshot_id: 11,
      record_count: 2
    }
    listUpstreamSuppliers.mockResolvedValue([
      { id: 7, name: 'Supplier A', status: 'active', is_system: false }
    ])
    listUpstreamCostPools.mockResolvedValue([initialPool])

    const wrapper = mountView()
    await flushPromises()
    const costTab = wrapper.findAll('button').find((button) => button.text() === 'admin.accounts.views.upstreamCost')
    expect(costTab).toBeTruthy()
    await costTab!.trigger('click')
    await flushPromises()

    let resolveStaleRequest!: (value: typeof initialPool[]) => void
    const staleRequest = new Promise<typeof initialPool[]>((resolve) => {
      resolveStaleRequest = resolve
    })
    listUpstreamCostPools.mockImplementationOnce(() => staleRequest)
    const callsBeforeStaleRefresh = listUpstreamCostPools.mock.calls.length
    await wrapper.get('[data-test="upstream-cost-refresh"]').trigger('click')
    await flushPromises()
    expect(listUpstreamCostPools).toHaveBeenCalledTimes(callsBeforeStaleRefresh + 1)

    listUpstreamCostPools.mockResolvedValueOnce([refreshedPool])
    await wrapper.get('[data-test="open-recharge-records"]').trigger('click')
    await wrapper.get('[data-test="emit-pool-updated"]').trigger('click')
    await flushPromises()
    expect(listUpstreamCostPools).toHaveBeenCalledTimes(callsBeforeStaleRefresh + 2)
    expect(wrapper.get('[data-test="upstream-current-cost"]').text()).toBe('5')

    resolveStaleRequest([initialPool])
    await flushPromises()
    expect(wrapper.get('[data-test="upstream-current-cost"]').text()).toBe('5')
  })
})
