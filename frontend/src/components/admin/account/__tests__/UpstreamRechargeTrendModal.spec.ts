import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UpstreamRechargeTrendModal from '../UpstreamRechargeTrendModal.vue'

const { getUpstreamSupplierRechargeTrend } = vi.hoisted(() => ({
  getUpstreamSupplierRechargeTrend: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getUpstreamSupplierRechargeTrend
    }
  }
}))

vi.mock('chart.js', () => ({
  CategoryScale: {},
  Chart: { register: vi.fn() },
  Filler: {},
  Legend: {},
  LinearScale: {},
  LineElement: {},
  PointElement: {},
  Tooltip: {}
}))

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template: `
      <div data-test="line-chart">
        <span data-test="chart-labels">{{ data.labels.join(',') }}</span>
        <span data-test="chart-datasets">{{ data.datasets.map((dataset) => dataset.label + ':' + dataset.data.join('/')).join(';') }}</span>
      </div>
    `
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.upstreamCost.rechargeTrend.title': 'Supplier Recharge Trend',
    'admin.accounts.upstreamCost.rechargeTrend.description': 'Trend description',
    'admin.accounts.upstreamCost.rechargeTrend.totalPaid': 'Total paid',
    'admin.accounts.upstreamCost.rechargeTrend.chartTitle': 'Paid amount trend',
    'admin.accounts.upstreamCost.rechargeTrend.chartHint': 'Grouped by {granularity}',
    'admin.accounts.upstreamCost.rechargeTrend.granularityLabel': 'Trend granularity',
    'admin.accounts.upstreamCost.rechargeTrend.granularity.day': 'Day',
    'admin.accounts.upstreamCost.rechargeTrend.granularity.week': 'Week',
    'admin.accounts.upstreamCost.rechargeTrend.granularity.month': 'Month',
    'admin.accounts.upstreamCost.rechargeTrend.granularity.year': 'Year',
    'admin.accounts.upstreamCost.rechargeTrend.empty': 'No recharge trend data yet',
    'admin.accounts.upstreamCost.rechargeTrend.loadFailed': 'Failed to load recharge trend',
    'admin.accounts.upstreamCost.recordCountBadge': '{count} records',
    'common.refresh': 'Refresh',
    'common.loading': 'Loading',
    'common.close': 'Close'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        let message = messages[key] || key
        for (const [paramKey, value] of Object.entries(params || {})) {
          message = message.replace(`{${paramKey}}`, String(value))
        }
        return message
      }
    })
  }
})

const baseDialogStub = {
  name: 'BaseDialog',
  props: ['show', 'title'],
  emits: ['close'],
  template: `
    <section v-if="show" data-test="base-dialog">
      <h2>{{ title }}</h2>
      <slot />
      <slot name="footer" />
    </section>
  `
}

function mountModal() {
  return mount(UpstreamRechargeTrendModal, {
    props: {
      show: true,
      supplierId: 7,
      supplierName: 'Supplier A'
    },
    global: {
      stubs: {
        BaseDialog: baseDialogStub,
        Icon: true
      }
    }
  })
}

describe('UpstreamRechargeTrendModal', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('loads and renders multi-currency recharge trend data', async () => {
    getUpstreamSupplierRechargeTrend.mockResolvedValue({
      supplier_id: 7,
      granularity: 'day',
      totals: [
        { currency: 'USD', amount: 30, record_count: 2 },
        { currency: 'CNY', amount: 700, record_count: 1 }
      ],
      points: [
        { period: '2026-01-01', currency: 'USD', amount: 10, record_count: 1 },
        { period: '2026-01-02', currency: 'USD', amount: 20, record_count: 1 },
        { period: '2026-01-02', currency: 'CNY', amount: 700, record_count: 1 }
      ]
    })

    const wrapper = mountModal()
    await flushPromises()

    expect(getUpstreamSupplierRechargeTrend).toHaveBeenCalledWith(7, 'day')
    expect(wrapper.text()).toContain('Supplier A')
    expect(wrapper.text()).toContain('700 CNY')
    expect(wrapper.text()).toContain('30 USD')
    expect(wrapper.get('[data-test="chart-labels"]').text()).toBe('2026-01-01,2026-01-02')
    expect(wrapper.get('[data-test="chart-datasets"]').text()).toContain('CNY:0/700')
    expect(wrapper.get('[data-test="chart-datasets"]').text()).toContain('USD:10/20')
  })

  it('reloads the trend when the administrator changes the time granularity', async () => {
    getUpstreamSupplierRechargeTrend
      .mockResolvedValueOnce({
        supplier_id: 7,
        granularity: 'day',
        totals: [{ currency: 'CNY', amount: 700, record_count: 2 }],
        points: [{ period: '2026-08-14', currency: 'CNY', amount: 700, record_count: 2 }]
      })
      .mockResolvedValueOnce({
        supplier_id: 7,
        granularity: 'month',
        totals: [{ currency: 'CNY', amount: 700, record_count: 2 }],
        points: [
          { period: '2026-07', currency: 'CNY', amount: 200, record_count: 1 },
          { period: '2026-08', currency: 'CNY', amount: 500, record_count: 1 }
        ]
      })

    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.get('[data-test="trend-granularity-day"]').attributes('aria-pressed')).toBe('true')
    await wrapper.get('[data-test="trend-granularity-month"]').trigger('click')
    await flushPromises()

    expect(getUpstreamSupplierRechargeTrend).toHaveBeenNthCalledWith(1, 7, 'day')
    expect(getUpstreamSupplierRechargeTrend).toHaveBeenNthCalledWith(2, 7, 'month')
    expect(wrapper.get('[data-test="trend-granularity-month"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="chart-labels"]').text()).toBe('2026-07,2026-08')
    expect(wrapper.text()).toContain('Grouped by Month')
  })

  it('restores the daily default when the modal is closed and reopened', async () => {
    getUpstreamSupplierRechargeTrend.mockImplementation(async (_supplierId: number, granularity: string) => ({
      supplier_id: 7,
      granularity,
      totals: [{ currency: 'CNY', amount: 700, record_count: 1 }],
      points: [{ period: granularity === 'day' ? '2026-08-14' : '2026-08', currency: 'CNY', amount: 700, record_count: 1 }]
    }))

    const wrapper = mountModal()
    await flushPromises()
    await wrapper.get('[data-test="trend-granularity-month"]').trigger('click')
    await flushPromises()
    await wrapper.setProps({ show: false })
    await flushPromises()
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getUpstreamSupplierRechargeTrend).toHaveBeenNthCalledWith(1, 7, 'day')
    expect(getUpstreamSupplierRechargeTrend).toHaveBeenNthCalledWith(2, 7, 'month')
    expect(getUpstreamSupplierRechargeTrend).toHaveBeenNthCalledWith(3, 7, 'day')
    expect(wrapper.get('[data-test="trend-granularity-day"]').attributes('aria-pressed')).toBe('true')
  })

  it('shows an empty state when the supplier has no trend points', async () => {
    getUpstreamSupplierRechargeTrend.mockResolvedValue({
      supplier_id: 7,
      granularity: 'day',
      totals: [],
      points: []
    })

    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.text()).toContain('No recharge trend data yet')
    expect(wrapper.find('[data-test="line-chart"]').exists()).toBe(false)
  })

  it('shows a load error from the trend endpoint', async () => {
    getUpstreamSupplierRechargeTrend.mockRejectedValue(new Error('trend unavailable'))

    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.text()).toContain('trend unavailable')
  })
})
