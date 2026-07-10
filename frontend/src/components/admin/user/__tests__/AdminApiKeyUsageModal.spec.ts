import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser, ApiKey } from '@/types'
import AdminApiKeyUsageModal from '../AdminApiKeyUsageModal.vue'

const { getStats, getUsageTrend, getModelStats, listUsage } = vi.hoisted(() => ({
  getStats: vi.fn(),
  getUsageTrend: vi.fn(),
  getModelStats: vi.fn(),
  listUsage: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: {
      getStats,
      list: listUsage,
    },
    dashboard: {
      getUsageTrend,
      getModelStats,
    },
  },
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const user: AdminUser = {
  id: 7,
  username: 'target',
  email: 'target@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
  notes: '',
}

const apiKey: ApiKey = {
  id: 11,
  user_id: 7,
  key: 'sk-test-abcdefghijklmnopqrstuvwxyz',
  name: 'prod-key',
  tags: [],
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
}

const mountModal = (props: { apiKey: ApiKey | null }) => mount(AdminApiKeyUsageModal, {
  props: {
    show: true,
    user,
    apiKey: props.apiKey,
  },
  global: {
    stubs: {
      AppDatePicker: {
        props: ['modelValue', 'placeholder'],
        emits: ['update:modelValue'],
        template: '<input class="date-picker-stub" :aria-label="placeholder" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
      },
      BaseDialog: {
        props: ['show', 'title'],
        template: '<section v-if="show" :data-title="title"><slot /></section>',
      },
      Icon: {
        props: ['name'],
        template: '<span class="icon">{{ name }}</span>',
      },
      TokenUsageTrend: true,
    },
  },
})

describe('AdminApiKeyUsageModal', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-10T12:00:00Z'))
    getStats.mockReset()
    getUsageTrend.mockReset()
    getModelStats.mockReset()
    listUsage.mockReset()

    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      total_account_cost: 0,
      average_duration_ms: 0,
    })
    getUsageTrend.mockResolvedValue({ trend: [] })
    getModelStats.mockResolvedValue({ models: [] })
    listUsage.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads user-wide usage without an api_key_id filter', async () => {
    mountModal({ apiKey: null })

    await flushPromises()

    const statsParams = getStats.mock.calls[0][0]
    const logsParams = listUsage.mock.calls[0][0]
    expect(statsParams).toEqual(expect.objectContaining({ user_id: 7 }))
    expect(statsParams).not.toHaveProperty('api_key_id')
    expect(logsParams).toEqual(expect.objectContaining({ user_id: 7 }))
    expect(logsParams).not.toHaveProperty('api_key_id')
  })

  it('loads key-scoped usage with the selected api_key_id filter', async () => {
    mountModal({ apiKey })

    await flushPromises()

    expect(getStats.mock.calls[0][0]).toEqual(expect.objectContaining({
      user_id: 7,
      api_key_id: 11,
    }))
    expect(listUsage.mock.calls[0][0]).toEqual(expect.objectContaining({
      user_id: 7,
      api_key_id: 11,
    }))
  })

  it('uses the selected date range when refreshing usage', async () => {
    const wrapper = mountModal({ apiKey: null })

    await flushPromises()
    getStats.mockClear()
    getUsageTrend.mockClear()
    getModelStats.mockClear()
    listUsage.mockClear()

    const dateInputs = wrapper.findAll<HTMLInputElement>('.date-picker-stub')
    await dateInputs[0].setValue('2026-06-01')
    await dateInputs[1].setValue('2026-06-10')

    const refreshButton = wrapper.findAll('button').find((button) => button.text().includes('common.refresh'))
    expect(refreshButton).toBeDefined()
    await refreshButton!.trigger('click')
    await flushPromises()

    expect(getStats.mock.calls[0][0]).toEqual(expect.objectContaining({
      user_id: 7,
      start_date: '2026-06-01',
      end_date: '2026-06-10',
    }))
    expect(listUsage.mock.calls[0][0]).toEqual(expect.objectContaining({
      user_id: 7,
      start_date: '2026-06-01',
      end_date: '2026-06-10',
    }))
  })

  it('keeps the selected date range valid when either boundary crosses the other', async () => {
    const wrapper = mountModal({ apiKey: null })

    await flushPromises()
    getStats.mockClear()
    getUsageTrend.mockClear()
    getModelStats.mockClear()
    listUsage.mockClear()

    const dateInputs = wrapper.findAll<HTMLInputElement>('.date-picker-stub')

    await dateInputs[0].setValue('2026-06-20')
    expect(dateInputs[0].element.value).toBe('2026-06-20')
    expect(dateInputs[1].element.value).toBe('2026-06-20')

    await dateInputs[1].setValue('2026-06-01')
    expect(dateInputs[0].element.value).toBe('2026-06-01')
    expect(dateInputs[1].element.value).toBe('2026-06-01')

    const refreshButton = wrapper.findAll('button').find((button) => button.text().includes('common.refresh'))
    expect(refreshButton).toBeDefined()
    await refreshButton!.trigger('click')
    await flushPromises()

    expect(getStats.mock.calls[0][0]).toEqual(expect.objectContaining({
      user_id: 7,
      start_date: '2026-06-01',
      end_date: '2026-06-01',
    }))
  })
})
