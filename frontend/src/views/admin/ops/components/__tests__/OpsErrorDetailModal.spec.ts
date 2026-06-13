import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { opsAPI, type OpsErrorDetail } from '@/api/admin/ops'
import OpsErrorDetailModal from '../OpsErrorDetailModal.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRequestErrorDetail: vi.fn(),
    getUpstreamErrorDetail: vi.fn(),
    listRequestErrorUpstreamErrors: vi.fn()
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

const BaseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show"><slot /></div>'
}

const IconStub = { template: '<span />' }

function makeErrorDetail(overrides: Partial<OpsErrorDetail> = {}): OpsErrorDetail {
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
    client_request_id: 'client-req-1',
    request_id: 'req-1',
    message: 'Upstream request failed',
    user_email: 'user@example.com',
    account_name: 'provider-account',
    group_name: 'default',
    error_body: '{"error":"' + 'x'.repeat(180) + '"}',
    user_agent: 'test-agent',
    is_business_limited: false,
    ...overrides
  }
}

describe('OpsErrorDetailModal', () => {
  afterEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  it('wraps long response detail text instead of requiring horizontal scrolling', async () => {
    vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValue(makeErrorDetail())
    vi.mocked(opsAPI.listRequestErrorUpstreamErrors).mockResolvedValue({
      items: [
        makeErrorDetail({
          id: 2,
          phase: 'upstream',
          error_source: 'upstream_http',
          request_id: 'upstream-req-1',
          upstream_error_detail: '{"upstream":"' + 'y'.repeat(180) + '"}'
        })
      ],
      total: 1
    })

    const wrapper = mount(OpsErrorDetailModal, {
      props: {
        show: true,
        errorId: 1,
        errorType: 'request'
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: IconStub
        }
      }
    })

    await flushPromises()

    const primaryResponse = wrapper.get('pre')
    expect(primaryResponse.classes()).toContain('ops-response-block')
    expect(primaryResponse.classes()).toContain('overflow-y-auto')
    expect(primaryResponse.classes()).not.toContain('overflow-auto')

    await wrapper.get('button').trigger('click')
    await nextTick()

    const responseBlocks = wrapper.findAll('pre')
    expect(responseBlocks).toHaveLength(2)
    expect(responseBlocks[1].classes()).toContain('ops-response-block')
    expect(responseBlocks[1].classes()).toContain('overflow-y-auto')
    expect(responseBlocks[1].classes()).not.toContain('overflow-auto')
  })
})
