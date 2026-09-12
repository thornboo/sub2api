import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'

import AccountsView from '../AccountsView.vue'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import UpstreamCostComparison from '@/components/admin/account/UpstreamCostComparison.vue'
import UpstreamSupplierModal from '@/components/admin/account/UpstreamSupplierModal.vue'
import AccountTableActions from '@/components/admin/account/AccountTableActions.vue'

const {
  listAccounts,
  listWithEtag,
  getById,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  listUpstreamCostPools,
  listUpstreamCostPoolAccounts,
  listUpstreamSuppliers,
  refreshUpstreamSupplierBalance,
  getUpstreamSupplierRechargeOverview,
  getAllProxies,
  getAllGroups,
  showError
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getById: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  listUpstreamCostPools: vi.fn(),
  listUpstreamCostPoolAccounts: vi.fn(),
  listUpstreamSuppliers: vi.fn(),
  refreshUpstreamSupplierBalance: vi.fn(),
  getUpstreamSupplierRechargeOverview: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      getById,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      listUpstreamCostPools,
      listUpstreamCostPoolAccounts,
      listUpstreamSuppliers,
      refreshUpstreamSupplierBalance,
      getUpstreamSupplierRechargeOverview,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token', isSimpleMode: false })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const DataTableStub = defineComponent({
  props: { data: { type: Array, default: () => [] } },
  template: `
    <div>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-groups" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
})

const AccountGroupsCellStub = defineComponent({
  props: { groups: { type: Array, default: () => [] } },
  template: '<span data-test="account-groups">{{ groups.map(group => group.name).join(",") }}</span>'
})

const EditAccountModalStub = defineComponent({
  props: { show: Boolean, account: { type: Object, default: null } },
  template: '<div data-test="edit-account">{{ show ? account?.name : "" }}</div>'
})

const AccountTestModalStub = defineComponent({
  props: { show: Boolean, account: { type: Object, default: null } },
  template: '<div data-test="test-account">{{ show ? account?.name : "" }}</div>'
})

const AccountStatsModalStub = defineComponent({
  props: { show: Boolean, account: { type: Object, default: null } },
  template: '<div data-test="stats-account">{{ show ? account?.name : "" }}</div>'
})

function mountView(stubActionMenu = true) {
  return mount(AccountsView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
        DataTable: DataTableStub,
        AccountTableActions: { props: ['loading'], template: '<div><slot name="after" /></div>' },
        AccountTableFilters: true,
        AccountBulkActionsBar: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountActionMenu: stubActionMenu,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: AccountTestModalStub,
        AccountStatsModal: AccountStatsModalStub,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: EditAccountModalStub,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: AccountGroupsCellStub,
        AccountUsageCell: true,
        UpstreamBillingRateCell: true,
        UpstreamCostComparison: true,
        UpstreamSupplierModal: true,
        HelpTooltip: true,
        Icon: true,
        Teleport: stubActionMenu,
        RouterLink: true,
        'router-link': true
      }
    }
  })
}

const listRow = {
  id: 42,
  name: 'compact row',
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  concurrency: 2,
  priority: 1,
  group_ids: [7],
  extra: {},
  credentials: {}
}

const fullAccount = {
  ...listRow,
  groups: [{ id: 7, name: 'codex', platform: 'openai' }],
  account_groups: [{ account_id: 42, group_id: 7 }],
  credentials: { api_key: 'redacted' },
  extra: { detail_only: true }
}

describe('admin AccountsView lite account list', () => {
  beforeEach(() => {
    localStorage.clear()
    listAccounts.mockReset().mockResolvedValue({ items: [listRow], total: 1, page: 1, page_size: 20, pages: 1 })
    listWithEtag.mockReset().mockResolvedValue({ notModified: true, etag: 'compact-etag', data: null })
    getById.mockReset().mockResolvedValue(fullAccount)
    getBatchTodayStats.mockReset().mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockReset().mockResolvedValue({ enabled: true })
    listUpstreamCostPools.mockReset().mockResolvedValue([])
    listUpstreamCostPoolAccounts.mockReset().mockResolvedValue([])
    listUpstreamSuppliers.mockReset().mockResolvedValue([])
    refreshUpstreamSupplierBalance.mockReset()
    getUpstreamSupplierRechargeOverview.mockReset().mockResolvedValue({ totals: [], suppliers: [] })
    getAllProxies.mockReset().mockResolvedValue([])
    getAllGroups.mockReset().mockResolvedValue([{ id: 7, name: 'codex', platform: 'openai' }])
    showError.mockReset()
  })

  afterEach(() => {
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      value: false
    })
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  const supplier = (id: number) => ({
    id, name: `supplier-${id}`, status: 'active', is_system: false,
    balance_config: { enabled: true, provider: 'newapi', has_access_token: true },
    balance_snapshot: { balance_usd: 13, updated_at: '2026-09-09T10:00:00Z', status: 'ok' }
  })

  const selectView = async (wrapper: ReturnType<typeof mountView>, key: string) => {
    await wrapper.findAll('button').find(button => button.text() === `admin.accounts.views.${key}`)!.trigger('click')
    await flushPromises()
  }

  const setPageHidden = (hidden: boolean) => {
    Object.defineProperty(document, 'hidden', {
      configurable: true,
      value: hidden
    })
  }

  it('shows the supplier list before wallet queries complete and queries only enabled active suppliers', async () => {
    const active = supplier(1)
    const disabled = { ...supplier(2), balance_config: { enabled: false } }
    const archived = { ...supplier(3), status: 'archived' }
    const system = { ...supplier(4), is_system: true }
    listUpstreamSuppliers.mockResolvedValue([active, disabled, archived, system])
    let finish!: (value: unknown) => void
    refreshUpstreamSupplierBalance.mockReturnValue(new Promise(resolve => { finish = resolve }))
    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')

    expect(refreshUpstreamSupplierBalance.mock.calls).toEqual([[1]])
    expect(wrapper.findComponent(UpstreamCostComparison).props('loading')).toBe(false)
    expect(wrapper.findComponent(UpstreamCostComparison).props('suppliers')).toEqual([active, disabled, archived, system])
    expect(wrapper.findComponent(AccountTableActions).props('loading')).toBe(true)

    const updated = { ...active, balance_snapshot: { ...active.balance_snapshot, balance_usd: 27.33 } }
    finish(updated)
    await flushPromises()
    expect(wrapper.findComponent(UpstreamCostComparison).props('suppliers')[0]).toEqual(updated)
    expect(wrapper.findComponent(AccountTableActions).props('loading')).toBe(false)
    wrapper.unmount()
  })

  it('preserves the last balance on a failed request while updating other suppliers', async () => {
    const first = supplier(1)
    const second = supplier(2)
    listUpstreamSuppliers.mockResolvedValue([first, second])
    refreshUpstreamSupplierBalance.mockImplementation((id: number) => id === 1
      ? Promise.reject({ message: 'HTTP 503: balance service unavailable' })
      : Promise.resolve({ ...second, balance_snapshot: { status: 'ok', balance_usd: 0 } }))
    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')

    const rows = wrapper.findComponent(UpstreamCostComparison).props('suppliers')
    expect(rows[0].balance_snapshot).toMatchObject({
      balance_usd: 13, updated_at: first.balance_snapshot.updated_at,
      status: 'error', error: 'HTTP 503: balance service unavailable'
    })
    expect(rows[1].balance_snapshot).toMatchObject({ status: 'ok', balance_usd: 0 })
    expect(wrapper.findComponent(UpstreamCostComparison).props('error')).toBeNull()
    wrapper.unmount()
  })

  it('keeps the supplier table when the overview request fails', async () => {
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)
    getUpstreamSupplierRechargeOverview.mockRejectedValue(new Error('overview unavailable'))

    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')

    const costView = wrapper.findComponent(UpstreamCostComparison)
    expect(costView.props('suppliers')).toEqual([active])
    expect(costView.props('error')).toBeNull()
    expect(costView.props('overviewError')).toBe('overview unavailable')
    wrapper.unmount()
  })

  it('refetches the overview after wallet refresh and ignores older overview responses', async () => {
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)
    let finishOldOverview!: (value: unknown) => void
    getUpstreamSupplierRechargeOverview
      .mockReturnValueOnce(new Promise(resolve => { finishOldOverview = resolve }))
      .mockResolvedValueOnce({ totals: [{ currency: 'USD', amount: 2, record_count: 1 }], suppliers: [] })
      .mockResolvedValueOnce({ totals: [{ currency: 'USD', amount: 3, record_count: 1 }], suppliers: [] })

    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')
    await selectView(wrapper, 'list')
    await selectView(wrapper, 'upstreamCost')

    finishOldOverview({ totals: [{ currency: 'USD', amount: 999, record_count: 1 }], suppliers: [] })
    await flushPromises()

    expect(getUpstreamSupplierRechargeOverview).toHaveBeenCalledTimes(3)
    expect(wrapper.findComponent(UpstreamCostComparison).props('rechargeOverview')).toEqual({
      totals: [{ currency: 'USD', amount: 3, record_count: 1 }],
      suppliers: []
    })
    wrapper.unmount()
  })

  it('surfaces background stored-data refresh failures without posting wallet refreshes', async () => {
    vi.useFakeTimers()
    setPageHidden(false)
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)

    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')
    const balanceRefreshesAfterLoad = refreshUpstreamSupplierBalance.mock.calls.length

    listUpstreamSuppliers.mockRejectedValueOnce(new Error('cached suppliers failed'))
    getUpstreamSupplierRechargeOverview.mockRejectedValueOnce(new Error('cached overview failed'))
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()

    const costView = wrapper.findComponent(UpstreamCostComparison)
    expect(costView.props('error')).toBe('cached suppliers failed')
    expect(costView.props('overviewError')).toBe('cached overview failed')
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(balanceRefreshesAfterLoad)
    wrapper.unmount()
  })

  it('stops background stored-data refresh after leaving the supplier tab', async () => {
    vi.useFakeTimers()
    setPageHidden(false)
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)

    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')
    const suppliersCallsAfterLoad = listUpstreamSuppliers.mock.calls.length
    const overviewCallsAfterLoad = getUpstreamSupplierRechargeOverview.mock.calls.length
    const balanceCallsAfterLoad = refreshUpstreamSupplierBalance.mock.calls.length

    await selectView(wrapper, 'list')
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()

    expect(listUpstreamSuppliers).toHaveBeenCalledTimes(suppliersCallsAfterLoad)
    expect(getUpstreamSupplierRechargeOverview).toHaveBeenCalledTimes(overviewCallsAfterLoad)
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(balanceCallsAfterLoad)
    wrapper.unmount()
  })

  it('does not let an older stored-data response clear a newer refresh generation', async () => {
    vi.useFakeTimers()
    setPageHidden(false)
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)

    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')

    let finishOldSuppliers!: (value: unknown) => void
    let finishOldOverview!: (value: unknown) => void
    listUpstreamSuppliers.mockReturnValueOnce(new Promise(resolve => { finishOldSuppliers = resolve }))
    getUpstreamSupplierRechargeOverview.mockReturnValueOnce(new Promise(resolve => { finishOldOverview = resolve }))
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()

    await selectView(wrapper, 'list')
    listUpstreamSuppliers.mockResolvedValueOnce([active])
    getUpstreamSupplierRechargeOverview
      .mockResolvedValueOnce({ totals: [], suppliers: [] })
      .mockResolvedValueOnce({ totals: [], suppliers: [] })
    await selectView(wrapper, 'upstreamCost')

    let finishNewSuppliers!: (value: unknown) => void
    let finishNewOverview!: (value: unknown) => void
    listUpstreamSuppliers.mockReturnValueOnce(new Promise(resolve => { finishNewSuppliers = resolve }))
    getUpstreamSupplierRechargeOverview.mockReturnValueOnce(new Promise(resolve => { finishNewOverview = resolve }))
    const supplierCallsBeforeNewRefresh = listUpstreamSuppliers.mock.calls.length
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(listUpstreamSuppliers).toHaveBeenCalledTimes(supplierCallsBeforeNewRefresh + 1)

    finishOldSuppliers([active])
    finishOldOverview({ totals: [{ currency: 'USD', amount: 999, record_count: 1 }], suppliers: [] })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(listUpstreamSuppliers).toHaveBeenCalledTimes(supplierCallsBeforeNewRefresh + 1)

    finishNewSuppliers([active])
    finishNewOverview({ totals: [], suppliers: [] })
    await flushPromises()
    wrapper.unmount()
  })

  it('queries balances again through toolbar refresh and the supplier saved event', async () => {
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)
    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(1)

    wrapper.findComponent(AccountTableActions).vm.$emit('refresh')
    await flushPromises()
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(2)

    wrapper.findComponent(UpstreamSupplierModal).vm.$emit('saved')
    await flushPromises()
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(3)
    wrapper.unmount()
  })

  it('resumes stored-data polling after a manual refresh supersedes a pending background read', async () => {
    vi.useFakeTimers()
    setPageHidden(false)
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    refreshUpstreamSupplierBalance.mockResolvedValue(active)
    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')

    let finishOld!: (value: unknown) => void
    listUpstreamSuppliers.mockReturnValueOnce(new Promise(resolve => { finishOld = resolve }))
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    wrapper.findComponent(AccountTableActions).vm.$emit('refresh')
    await flushPromises()
    finishOld([active])
    await flushPromises()

    const before = listUpstreamSuppliers.mock.calls.length
    await vi.advanceTimersByTimeAsync(60_000)
    await flushPromises()
    expect(listUpstreamSuppliers).toHaveBeenCalledTimes(before + 1)
    wrapper.unmount()
  })

  it('ignores old balance results after a newer supplier list refresh', async () => {
    const active = supplier(1)
    listUpstreamSuppliers.mockResolvedValue([active])
    let finishOld!: (value: unknown) => void
    refreshUpstreamSupplierBalance.mockReturnValueOnce(new Promise(resolve => { finishOld = resolve }))
    const latest = { ...active, balance_snapshot: { status: 'ok', balance_usd: 42 } }
    refreshUpstreamSupplierBalance.mockResolvedValue(latest)
    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')
    await selectView(wrapper, 'list')
    await selectView(wrapper, 'upstreamCost')

    finishOld({ ...active, balance_snapshot: { status: 'ok', balance_usd: 999 } })
    await flushPromises()
    expect(wrapper.findComponent(UpstreamCostComparison).props('suppliers')).toEqual([latest])
    wrapper.unmount()
  })

  it('limits concurrent wallet queries and stops queued queries after leaving the supplier list', async () => {
    listUpstreamSuppliers.mockResolvedValue(Array.from({ length: 9 }, (_, index) => supplier(index + 1)))
    let finish!: () => void
    const pending = new Promise<void>(resolve => { finish = resolve })
    refreshUpstreamSupplierBalance.mockImplementation((id: number) => pending.then(() => supplier(id)))
    const wrapper = mountView()
    await flushPromises()
    await selectView(wrapper, 'upstreamCost')
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(4)

    await selectView(wrapper, 'list')
    finish()
    await flushPromises()
    expect(refreshUpstreamSupplierBalance).toHaveBeenCalledTimes(4)
    wrapper.unmount()
  })

  it('keeps lite=1 on the initial list request', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({ lite: '1' }),
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
    wrapper.unmount()
  })

  it('maps group_ids through the group catalog for the table cell', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="account-groups"]').text()).toBe('codex')
    wrapper.unmount()
  })

  it('keeps the action menu open during internal scrolling but closes it on table scrolling', async () => {
    const wrapper = mountView(false)
    await flushPromises()

    const trigger = wrapper.findAll('button').find(button => button.text() === 'common.more')!
    await trigger.trigger('click')
    const menu = new DOMWrapper(document.body.querySelector('.action-menu-content')!)
    menu.element.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.findComponent(AccountActionMenu).props('show')).toBe(true)

    menu.get('button').element.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.findComponent(AccountActionMenu).props('show')).toBe(true)

    wrapper.getComponent(DataTableStub).element.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(wrapper.findComponent(AccountActionMenu).props('show')).toBe(false)
    wrapper.unmount()
  })

  it('keeps lite=1 on automatic ETag refreshes', async () => {
    vi.useFakeTimers()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()

    await vi.advanceTimersByTimeAsync(6000)
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({ lite: '1' }),
      expect.objectContaining({ etag: null })
    )
    wrapper.unmount()
  })

  it('loads the full account by id before opening edit, test, and stats actions', async () => {
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))
    expect(editButton).toBeTruthy()
    await editButton!.trigger('click')
    await flushPromises()
    expect(getById).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('compact row')

    const menu = wrapper.findComponent(AccountActionMenu)
    menu.vm.$emit('test', listRow)
    await flushPromises()
    expect(getById).toHaveBeenCalledTimes(2)
    expect(wrapper.get('[data-test="test-account"]').text()).toBe('compact row')

    menu.vm.$emit('stats', listRow)
    await flushPromises()
    expect(getById).toHaveBeenCalledTimes(3)
    expect(wrapper.get('[data-test="stats-account"]').text()).toBe('compact row')
    wrapper.unmount()
  })

  it('shows an error and keeps the modal closed when detail loading fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    getById.mockRejectedValueOnce(new Error('detail failed'))
    const wrapper = mountView()
    await flushPromises()

    const editButton = wrapper.findAll('button').find(button => button.text().includes('common.edit'))
    await editButton!.trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('detail failed')
    expect(wrapper.get('[data-test="edit-account"]').text()).toBe('')
    consoleError.mockRestore()
    wrapper.unmount()
  })
})
