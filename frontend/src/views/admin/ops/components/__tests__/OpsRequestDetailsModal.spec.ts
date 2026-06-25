import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'
import { opsAPI } from '@/api/admin/ops'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'admin.ops.requestDetails.rangeLabel' && params?.range) {
          return `Window: ${params.range}`
        }
        return key
      }
    })
  }
})

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    listRequestDetails: vi.fn()
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /></div>'
}

const PaginationStub = {
  template: '<div />'
}

describe('OpsRequestDetailsModal', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('opens a request error detail without closing the request details modal', async () => {
    vi.mocked(opsAPI.listRequestDetails).mockResolvedValue({
      items: [
        {
          kind: 'error',
          created_at: '2026-06-13T10:00:00Z',
          request_id: 'req-1',
          platform: 'openai',
          model: 'gpt-4o',
          duration_ms: 1200,
          status_code: 500,
          error_id: 42
        }
      ],
      total: 1
    })

    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: true,
        timeRange: '1h',
        preset: {
          title: 'Requests',
          kind: 'all',
          sort: 'created_at_desc'
        },
        platform: '',
        groupId: null
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub
        }
      }
    })

    await flushPromises()

    const viewErrorButton = wrapper.findAll('button').find((button) => {
      return button.text() === 'admin.ops.requestDetails.viewError'
    })
    expect(viewErrorButton).toBeTruthy()

    await viewErrorButton!.trigger('click')

    expect(wrapper.emitted('openErrorDetail')).toEqual([[42, 'request']])
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('uses explicit custom start and end times for request detail queries', async () => {
    vi.mocked(opsAPI.listRequestDetails).mockResolvedValue({ items: [], total: 0 })

    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: true,
        timeRange: 'custom',
        customStartTime: '2026-06-22T00:00:00',
        customEndTime: '2026-06-23T00:00:00',
        preset: {
          title: 'Requests',
          kind: 'all',
          sort: 'created_at_desc'
        },
        platform: '',
        groupId: null
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub
        }
      }
    })

    await flushPromises()

    expect(opsAPI.listRequestDetails).toHaveBeenCalledWith(
      expect.objectContaining({
        start_time: '2026-06-22T00:00:00',
        end_time: '2026-06-23T00:00:00',
        page: 1,
        page_size: 10,
        kind: 'all',
        sort: 'created_at_desc'
      })
    )
    expect(wrapper.text()).toContain('Window: 06-22 00:00 ~ 06-23 00:00')
  })
})
