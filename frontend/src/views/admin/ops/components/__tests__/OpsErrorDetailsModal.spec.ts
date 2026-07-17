import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { opsAPI, type OpsErrorLog } from '@/api/admin/ops'
import OpsErrorDetailsModal from '../OpsErrorDetailsModal.vue'
import zhLocale from '@/i18n/locales/zh'
import enLocale from '@/i18n/locales/en'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    listRequestErrors: vi.fn(),
    listUpstreamErrors: vi.fn()
  }
}))

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /></div>'
}

const SelectStub = {
  template: '<div />'
}

const OpsErrorLogTableStub = {
  template: '<div />'
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })

  return { promise, resolve, reject }
}

function makeErrorLog(requestId: string): OpsErrorLog {
  return {
    id: 1,
    created_at: '2026-06-13T10:00:00Z',
    phase: 'request',
    type: 'upstream_error',
    error_owner: 'provider',
    error_source: 'client_request',
    severity: 'error',
    status_code: 500,
    platform: 'openai',
    model: 'gpt-4o',
    resolved: false,
    client_request_id: `client-${requestId}`,
    request_id: requestId,
    message: 'Upstream request failed',
    user_email: 'user@example.com',
    account_name: 'provider-account',
    group_name: 'default'
  }
}

describe('OpsErrorDetailsModal', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('refetches from the matching endpoint when errorType changes while open', async () => {
    vi.mocked(opsAPI.listRequestErrors).mockResolvedValue({ items: [], total: 0 })
    vi.mocked(opsAPI.listUpstreamErrors).mockResolvedValue({ items: [], total: 0 })

    const wrapper = mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '1h',
        platform: '',
        groupId: null,
        errorType: 'request'
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          OpsErrorLogTable: OpsErrorLogTableStub
        }
      }
    })

    await flushPromises()

    expect(opsAPI.listRequestErrors).toHaveBeenCalledTimes(1)
    expect(opsAPI.listRequestErrors).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        time_range: '1h',
        view: 'errors'
      })
    )
    expect(opsAPI.listUpstreamErrors).not.toHaveBeenCalled()

    await wrapper.setProps({ errorType: 'upstream' })
    await flushPromises()

    expect(opsAPI.listUpstreamErrors).toHaveBeenCalledTimes(1)
    expect(opsAPI.listUpstreamErrors).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        time_range: '1h',
        view: 'errors'
      })
    )
    expect(vi.mocked(opsAPI.listUpstreamErrors).mock.calls[0]?.[0]).not.toHaveProperty('phase')
  })

  it('ignores stale fetch responses when errorType changes quickly', async () => {
    const staleRequest = createDeferred<{ items: OpsErrorLog[]; total: number }>()
    const freshUpstream = createDeferred<{ items: OpsErrorLog[]; total: number }>()
    vi.mocked(opsAPI.listRequestErrors).mockReturnValueOnce(staleRequest.promise)
    vi.mocked(opsAPI.listUpstreamErrors).mockReturnValueOnce(freshUpstream.promise)

    const ErrorLogRowsStub = {
      props: ['rows'],
      template: '<div data-testid="rows">{{ rows.map((row) => row.request_id).join(",") }}</div>'
    }

    const wrapper = mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '1h',
        platform: '',
        groupId: null,
        errorType: 'request'
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          OpsErrorLogTable: ErrorLogRowsStub
        }
      }
    })

    await wrapper.setProps({ errorType: 'upstream' })

    freshUpstream.resolve({ items: [makeErrorLog('fresh-upstream')], total: 1 })
    await flushPromises()

    expect(wrapper.get('[data-testid="rows"]').text()).toBe('fresh-upstream')

    staleRequest.resolve({ items: [makeErrorLog('stale-request')], total: 1 })
    await flushPromises()

    expect(wrapper.get('[data-testid="rows"]').text()).toBe('fresh-upstream')
  })

  it('renders explicit filter labels for the details toolbar', async () => {
    vi.mocked(opsAPI.listRequestErrors).mockResolvedValue({ items: [], total: 0 })

    const wrapper = mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '1h',
        platform: '',
        groupId: null,
        errorType: 'request'
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          OpsErrorLogTable: OpsErrorLogTableStub
        }
      }
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('admin.ops.errorDetails.filters.search')
    expect(text).toContain('admin.ops.errorDetails.filters.statusCode')
    expect(text).toContain('admin.ops.errorDetails.filters.phase')
    expect(text).toContain('admin.ops.errorDetails.filters.owner')
    expect(text).toContain('admin.ops.errorDetails.filters.scope')
    expect(text).toContain('admin.ops.errorDetails.filters.domain')
    expect(text).toContain('admin.ops.errorDetails.filters.category')
    expect(text).toContain('admin.ops.errorDetails.filters.resolutionOwner')
    expect(text).toContain('admin.ops.errorDetails.filters.slaImpact')
    expect(wrapper.find('input').attributes('placeholder')).toBe('admin.ops.errorDetails.searchPlaceholder')
  })

  it('applies preset filters when opening upstream non-rate errors', async () => {
    vi.mocked(opsAPI.listUpstreamErrors).mockResolvedValue({ items: [], total: 0 })

    mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '1h',
        platform: '',
        groupId: null,
        errorType: 'upstream',
        preset: {
          title: 'Non-rate upstream',
          view: 'errors',
          owner: 'provider',
          statusCode: 'non_rate_overload'
        }
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          OpsErrorLogTable: OpsErrorLogTableStub
        }
      }
    })

    await flushPromises()

    expect(opsAPI.listUpstreamErrors).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        time_range: '1h',
        view: 'errors',
        error_owner: 'provider',
        status_codes_exclude: '429,529'
      })
    )
    expect(vi.mocked(opsAPI.listUpstreamErrors).mock.calls[0]?.[0]).not.toHaveProperty('phase')
  })

  it('uses explicit custom start and end times for detail queries', async () => {
    vi.mocked(opsAPI.listRequestErrors).mockResolvedValue({ items: [], total: 0 })

    mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: 'custom',
        customStartTime: '2026-06-22T00:00:00.000Z',
        customEndTime: '2026-06-23T00:00:00.000Z',
        platform: '',
        groupId: null,
        errorType: 'request'
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          OpsErrorLogTable: OpsErrorLogTableStub
        }
      }
    })

    await flushPromises()

    const params = vi.mocked(opsAPI.listRequestErrors).mock.calls[0]?.[0]
    expect(params).toEqual(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        start_time: '2026-06-22T00:00:00.000Z',
        end_time: '2026-06-23T00:00:00.000Z',
        view: 'errors'
      })
    )
    expect(params).not.toHaveProperty('time_range')
  })

  it('locks dashboard drill-downs to the snapshot and applies v2 classification filters', async () => {
    vi.mocked(opsAPI.listRequestErrors).mockResolvedValue({ items: [], total: 0 })

    mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '6h',
        platform: 'openai',
        groupId: 7,
        errorType: 'request',
        preset: {
          title: 'Platform routing',
          view: 'all',
          startTime: '2026-07-18T00:00:00.000Z',
          endTime: '2026-07-18T06:00:00.000Z',
          customerVisible: true,
          failureDomain: 'platform',
          failureCategory: 'routing_capacity',
          resolutionOwner: 'platform_ops',
          slaImpact: true
        }
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          OpsErrorLogTable: OpsErrorLogTableStub
        }
      }
    })

    await flushPromises()

    const params = vi.mocked(opsAPI.listRequestErrors).mock.calls[0]?.[0]
    expect(params).toEqual(expect.objectContaining({
      start_time: '2026-07-18T00:00:00.000Z',
      end_time: '2026-07-18T06:00:00.000Z',
      platform: 'openai',
      group_id: 7,
      view: 'all',
      customer_visible: true,
      failure_domain: 'platform',
      failure_category: 'routing_capacity',
      resolution_owner: 'platform_ops',
      sla_impact: 'true'
    }))
    expect(params).not.toHaveProperty('time_range')
  })
})

describe('OpsErrorDetailsModal locale copy', () => {
  it('uses user-facing search placeholder copy instead of raw field names', () => {
    expect(zhLocale.admin.ops.errorDetails.searchPlaceholder).toBe('搜索请求 ID、客户端请求 ID、错误信息')
    expect(zhLocale.admin.ops.errorDetails.searchPlaceholder).not.toContain('request_id')
    expect(enLocale.admin.ops.errorDetails.searchPlaceholder).toBe('Search request ID, client request ID, or error message')
    expect(enLocale.admin.ops.errorDetails.searchPlaceholder).not.toContain('request_id')
  })
})
