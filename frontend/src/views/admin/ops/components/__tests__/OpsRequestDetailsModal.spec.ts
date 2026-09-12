import { flushPromises, mount, shallowMount } from '@vue/test-utils'
import { ref } from 'vue'
import type { OpsDashboardOverview } from '@/api/admin/ops'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import OpsDashboardHeader from '../OpsDashboardHeader.vue'
import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'
import { opsAPI } from '@/api/admin/ops'

const { listRequestDetails, viewport } = vi.hoisted(() => ({
  listRequestDetails: vi.fn(),
  viewport: { desktop: true },
}))

vi.mock('@vueuse/core', () => ({ useMediaQuery: () => ref(viewport.desktop) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listRequestDetails } }))
vi.mock('@/api', () => ({ adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([]) } } }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn(),
  }),
  useAdminSettingsStore: () => ({ opsRealtimeMonitoringEnabled: false }),
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn().mockResolvedValue(true) }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'admin.ops.requestDetails.rangeLabel' && params?.range) return `Window: ${params.range}`
      return key
    },
  }),
}))

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /></div>',
}

async function openDetails(sort: 'created_at_desc' | 'duration_desc' | 'ttft_desc' = 'created_at_desc') {
  const wrapper = mount(OpsRequestDetailsModal, {
    props: { modelValue: false, timeRange: '1h', preset: { title: 'Details', kind: 'all', sort } },
    global: { stubs: { BaseDialog: BaseDialogStub, Pagination: true } },
  })
  await wrapper.setProps({ modelValue: true })
  await flushPromises()
  return wrapper
}

describe('OpsRequestDetailsModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    viewport.desktop = true
    listRequestDetails.mockResolvedValue({
      items: [
        {
          kind: 'error',
          created_at: '2026-06-13T10:00:00Z',
          request_id: 'req-1',
          platform: 'openai',
          model: 'gpt-4o',
          duration_ms: 1200,
          first_token_ms: 800,
          status_code: 500,
          error_id: 42,
        },
        { kind: 'success', created_at: '2026-09-10T00:00:01Z', duration_ms: 9000, first_token_ms: 0 },
        { kind: 'success', created_at: '2026-09-10T00:00:02Z', duration_ms: 5000, first_token_ms: null },
      ],
      total: 3,
    })
  })

  it('opens a request error detail without closing the request details modal', async () => {
    const wrapper = await openDetails()

    const viewErrorButton = wrapper.findAll('button').find((button) => {
      return button.text() === 'admin.ops.requestDetails.viewError'
    })
    expect(viewErrorButton).toBeTruthy()

    await viewErrorButton!.trigger('click')

    expect(wrapper.emitted('openErrorDetail')).toEqual([[42, 'request']])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('uses explicit custom start and end times for request detail queries', async () => {
    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: true,
        timeRange: 'custom',
        customStartTime: '2026-06-22T00:00:00',
        customEndTime: '2026-06-23T00:00:00',
        preset: {
          title: 'Requests',
          kind: 'all',
          sort: 'created_at_desc',
        },
        platform: '',
        groupId: null,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Pagination: true,
        },
      },
    })

    await flushPromises()

    expect(opsAPI.listRequestDetails).toHaveBeenCalledWith(
      expect.objectContaining({
        start_time: '2026-06-22T00:00:00',
        end_time: '2026-06-23T00:00:00',
        page: 1,
        page_size: 10,
        kind: 'all',
        sort: 'created_at_desc',
      }),
    )
    expect(wrapper.text()).toContain('Window: 06-22 00:00 ~ 06-23 00:00')
  })

  it('opens the TTFT card with first-token sorting and successful requests', async () => {
    const wrapper = shallowMount(OpsDashboardHeader, {
      props: { overview: {} as OpsDashboardOverview, platform: '', groupId: null, timeRange: '1h', queryMode: 'auto', loading: false, lastUpdated: null },
    })
    await flushPromises()
    const button = wrapper.findAll('button').find((item) =>
      item.text() === 'admin.ops.requestDetails.details' &&
      item.element.parentElement?.textContent?.includes('TTFT'),
    )
    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('openRequestDetails')).toEqual([[
      { title: 'admin.ops.ttftLabel', kind: 'success', sort: 'ttft_desc' },
    ]])
  })

  it.each([true, false])('shows TTFT rather than total duration (desktop: %s)', async (desktop) => {
    viewport.desktop = desktop
    const wrapper = await openDetails('ttft_desc')
    expect(listRequestDetails).toHaveBeenCalledWith(expect.objectContaining({ sort: 'ttft_desc' }))
    expect(wrapper.text()).toContain('admin.ops.ttftLabel')
    expect(wrapper.text()).toContain('800 ms')
    expect(wrapper.text()).toContain('0 ms')
    expect(wrapper.text()).not.toContain('1200 ms')
    expect(wrapper.text()).not.toContain('9000 ms')
    expect(wrapper.text()).not.toContain('5000 ms')
    if (desktop) expect(wrapper.findAll('tbody tr')[2].findAll('td')[4].text()).toBe('-')
    else expect(wrapper.text()).toContain('admin.ops.ttftLabel: -')
  })

  it('keeps total duration for duration details', async () => {
    const wrapper = await openDetails('duration_desc')
    expect(listRequestDetails).toHaveBeenCalledWith(expect.objectContaining({ sort: 'duration_desc' }))
    expect(wrapper.text()).toContain('admin.ops.requestDetails.table.duration')
    expect(wrapper.text()).toContain('1200 ms')
    expect(wrapper.text()).not.toContain('800 ms')
  })
})
