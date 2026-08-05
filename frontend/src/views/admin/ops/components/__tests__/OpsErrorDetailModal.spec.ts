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

function mountModal() {
  return mount(OpsErrorDetailModal, {
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
}

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

    const wrapper = mountModal()

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

  it('renders enterprise member route trace only when present', async () => {
    vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValueOnce(
      makeErrorDetail({
        enterprise_member_route: {
          planned_group_ids: [11, 12],
          pruned_groups: [{ group_id: 11, group_name: 'mimo', reason: 'model_unpublished' }],
          attempts: [
            {
              group_id: 12,
              group_name: 'glm',
              attempt_number: 1,
              outcome: 'terminal_failure',
              reason: 'capability_mismatch',
              safe_to_replay: false,
              source: 'last_known_good',
              lkg_age_seconds: 42
            }
          ],
          final_responsibility: 'client',
          source: 'last_known_good',
          lkg_age_seconds: 42
        }
      })
    )
    vi.mocked(opsAPI.listRequestErrorUpstreamErrors).mockResolvedValue({ items: [], total: 0 })

    const wrapper = mountModal()

    await flushPromises()

    const routeTrace = wrapper.get('[data-testid="enterprise-member-route-trace"]')
    expect(routeTrace.text()).toContain('#11')
    expect(routeTrace.text()).toContain('#12')
    expect(routeTrace.text()).toContain('model_unpublished')
    expect(routeTrace.text()).toContain('glm')
    expect(routeTrace.text()).toContain('client')
    expect(routeTrace.text()).toContain('admin.ops.errorDetail.enterpriseRoute.lkgSourceWithAge')

    vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValueOnce(makeErrorDetail({ id: 3 }))
    await wrapper.setProps({ errorId: 3 })
    await flushPromises()

    expect(wrapper.find('[data-testid="enterprise-member-route-trace"]').exists()).toBe(false)
  })

  const persistedRoutingFixtures: Array<{
    name: string
    model: string
    source: string
    ageMs?: number
    attempts: NonNullable<OpsErrorDetail['routing_attempts']>
    expected: string[]
  }> = [
    {
      name: 'gpt-image-1.5 unpublished image model',
      model: 'gpt-image-1.5',
      source: 'live',
      attempts: [
        {
          stage: 'pruned_candidate',
          outcome: 'pruned',
          group_id: 31,
          model_owner_group_id: 91,
          requested_model: 'gpt-image-1.5',
          reason: 'model_unpublished'
        }
      ],
      expected: ['pruned:#31', '#91', 'gpt-image-1.5', 'model_unpublished']
    },
    {
      name: 'glm-5.2 stays on GLM owner group',
      model: 'glm-5.2',
      source: 'live',
      attempts: [
        {
          stage: 'planned_candidate',
          outcome: 'planned',
          group_id: 42,
          model_owner_group_id: 42,
          requested_model: 'glm-5.2'
        },
        {
          stage: 'actual_attempt',
          outcome: 'selected',
          group_id: 42,
          model_owner_group_id: 42,
          attempt_number: 1,
          requested_model: 'glm-5.2',
          mapped_model: 'glm-5.2',
          safe_to_replay: true
        }
      ],
      expected: ['planned:#42', 'actual:#42', '#42', 'glm-5.2']
    },
    {
      name: 'minimax-m3 terminal evidence keeps owner separate',
      model: 'minimax-m3',
      source: 'live',
      attempts: [
        {
          stage: 'planned_candidate',
          outcome: 'planned',
          group_id: 52,
          model_owner_group_id: 52,
          requested_model: 'minimax-m3'
        },
        {
          stage: 'actual_attempt',
          outcome: 'terminal_failure',
          group_id: 53,
          model_owner_group_id: 52,
          attempt_number: 1,
          requested_model: 'minimax-m3',
          reason: 'capacity_exhausted',
          safe_to_replay: false,
          response_committed: false
        }
      ],
      expected: ['planned:#52', 'terminal:#53', '#52', 'terminal_failure', 'capacity_exhausted']
    },
    {
      name: 'gpt-5.6-terra last known good snapshot age',
      model: 'gpt-5.6-terra',
      source: 'last_known_good',
      ageMs: 42000,
      attempts: [
        {
          stage: 'planned_candidate',
          outcome: 'planned',
          group_id: 62,
          model_owner_group_id: 62,
          requested_model: 'gpt-5.6-terra'
        },
        {
          stage: 'pruned_candidate',
          outcome: 'pruned',
          group_id: 63,
          model_owner_group_id: 99,
          requested_model: 'gpt-5.6-terra',
          reason: 'endpoint_capability'
        }
      ],
      expected: ['planned:#62', 'pruned:#63', '#99', 'gpt-5.6-terra', 'admin.ops.errorDetail.enterpriseRoute.lkgSourceWithAge']
    }
  ]

  for (const fixture of persistedRoutingFixtures) {
    it(`renders persisted routing_attempts for ${fixture.name}`, async () => {
      vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValueOnce(
        makeErrorDetail({
          model: fixture.model,
          requested_model: fixture.model,
          routing_plan_source: fixture.source,
          routing_snapshot_age_ms: fixture.ageMs,
          routing_attempts: fixture.attempts
        })
      )
      vi.mocked(opsAPI.listRequestErrorUpstreamErrors).mockResolvedValue({ items: [], total: 0 })

      const wrapper = mountModal()

      await flushPromises()

      const routeTrace = wrapper.get('[data-testid="enterprise-member-route-trace"]')
      const text = routeTrace.text()
      for (const expected of fixture.expected) {
        expect(text).toContain(expected)
      }
    })
  }

  it('prefers persisted routing fields over legacy enterprise member route fields', async () => {
    vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValueOnce(
      makeErrorDetail({
        routing_plan_source: 'live',
        routing_attempts: [
          {
            stage: 'actual_attempt',
            outcome: 'selected',
            group_id: 77,
            model_owner_group_id: 88,
            requested_model: 'glm-5.2'
          }
        ],
        enterprise_member_route: {
          attempts: [{ group_id: 11, group_name: 'legacy-mimo', outcome: 'terminal_failure' }],
          final_responsibility: 'legacy-final',
          source: 'last_known_good',
          lkg_age_seconds: 9
        }
      })
    )
    vi.mocked(opsAPI.listRequestErrorUpstreamErrors).mockResolvedValue({ items: [], total: 0 })

    const wrapper = mountModal()

    await flushPromises()

    const text = wrapper.get('[data-testid="enterprise-member-route-trace"]').text()
    expect(text).toContain('actual:#77')
    expect(text).toContain('#88')
    expect(text).not.toContain('legacy-mimo')
  })

  it('does not render empty routing arrays or sensitive routing payload fields', async () => {
    vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValueOnce(
      makeErrorDetail({
        id: 4,
        routing_attempts: []
      })
    )
    vi.mocked(opsAPI.listRequestErrorUpstreamErrors).mockResolvedValue({ items: [], total: 0 })

    const emptyWrapper = mountModal()
    await flushPromises()
    expect(emptyWrapper.find('[data-testid="enterprise-member-route-trace"]').exists()).toBe(false)

    vi.mocked(opsAPI.getRequestErrorDetail).mockResolvedValueOnce(
      makeErrorDetail({
        id: 5,
        routing_plan_source: 'live',
        routing_attempts: [
          {
            stage: 'actual_attempt',
            outcome: 'terminal_failure',
            group_id: 81,
            model_owner_group_id: 82,
            requested_model: 'gpt-5.6-terra',
            reason: 'capability_mismatch',
            api_key: 'sk-secret-route-key',
            body: '{"token":"secret-body"}',
            credentials: { bearer: 'secret-credentials' },
            member: { code: 'secret-member' }
          } as NonNullable<OpsErrorDetail['routing_attempts']>[number] & Record<string, unknown>
        ]
      })
    )

    const privacyWrapper = mountModal()
    await flushPromises()

    const text = privacyWrapper.get('[data-testid="enterprise-member-route-trace"]').text()
    expect(text).toContain('terminal:#81')
    expect(text).toContain('#82')
    expect(text).not.toContain('sk-secret-route-key')
    expect(text).not.toContain('secret-body')
    expect(text).not.toContain('secret-credentials')
    expect(text).not.toContain('secret-member')
  })
})
