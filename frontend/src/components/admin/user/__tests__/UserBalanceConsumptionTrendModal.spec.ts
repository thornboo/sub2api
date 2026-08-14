import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UserBalanceConsumptionTrendModal from '../UserBalanceConsumptionTrendModal.vue'

const { getUsageTrend } = vi.hoisted(() => ({
  getUsageTrend: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getUsageTrend
    }
  }
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

vi.mock('vue-chartjs', () => ({
  Bar: {
    props: ['data', 'options'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>'
  }
}))

const user: AdminUser = {
  id: 7,
  username: 'target',
  email: 'target@example.com',
  role: 'user',
  account_type: 'individual',
  balance: 42.5,
  frozen_balance: 1.5,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
  notes: ''
}

const mountModal = () => mount(UserBalanceConsumptionTrendModal, {
  props: {
    show: true,
    user
  },
  global: {
    stubs: {
      BaseDialog: {
        props: ['show', 'title'],
        template: '<section v-if="show" :data-title="title"><slot /><slot name="footer" /></section>'
      },
      Icon: true
    }
  }
})

describe('UserBalanceConsumptionTrendModal', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-10T12:00:00Z'))
    getUsageTrend.mockReset()
    getUsageTrend.mockResolvedValue({
      trend: [
        {
          date: '2026-06-08',
          requests: 5,
          input_tokens: 100,
          output_tokens: 50,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 150,
          cost: 3,
          actual_cost: 2.5
        },
        {
          date: '2026-06-10',
          requests: 3,
          input_tokens: 80,
          output_tokens: 20,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          total_tokens: 100,
          cost: 1.5,
          actual_cost: 1.25
        }
      ]
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads only wallet-billed daily spending and renders the 30-day summary', async () => {
    const wrapper = mountModal()

    await flushPromises()

    expect(getUsageTrend).toHaveBeenCalledWith({
      user_id: 7,
      billing_type: 0,
      granularity: 'day',
      start_date: '2026-05-12',
      end_date: '2026-06-10'
    })
    expect(wrapper.get('[data-test="current-balance"]').text()).toBe('$42.50')
    expect(wrapper.get('[data-test="today-consumption"]').text()).toBe('$1.2500')
    expect(wrapper.get('[data-test="period-consumption"]').text()).toBe('$3.7500')
    expect(wrapper.get('[data-test="daily-average"]').text()).toBe('$0.1250')

    const chartData = JSON.parse(wrapper.get('.chart-data').text())
    expect(chartData.labels).toHaveLength(30)
    expect(chartData.labels.at(-1)).toBe('06-10')
    expect(chartData.datasets[0].data.at(-1)).toBe(1.25)
    expect(chartData.datasets[0].data.at(-2)).toBe(0)
  })

  it('reloads the trend with the selected seven-day range', async () => {
    const wrapper = mountModal()
    await flushPromises()
    getUsageTrend.mockClear()

    await wrapper.get('[data-test="balance-range-7"]').trigger('click')
    await flushPromises()

    expect(getUsageTrend).toHaveBeenCalledWith(expect.objectContaining({
      user_id: 7,
      billing_type: 0,
      granularity: 'day',
      start_date: '2026-06-04',
      end_date: '2026-06-10'
    }))
  })
})
