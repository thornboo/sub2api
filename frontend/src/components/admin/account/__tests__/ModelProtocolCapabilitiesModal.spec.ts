import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ModelProtocolCapabilitiesModal from '../ModelProtocolCapabilitiesModal.vue'
import type {
  AccountModelProtocolCapabilitiesResponse,
  AccountModelProtocolCapability
} from '@/api/admin/accounts'

const {
  getModelProtocolCapabilities,
  getSettings,
  syncModelProtocolCapabilities,
  updateModelProtocolCapabilityOverrides,
  showSuccess
} = vi.hoisted(() => ({
  getModelProtocolCapabilities: vi.fn(),
  getSettings: vi.fn(),
  syncModelProtocolCapabilities: vi.fn(),
  updateModelProtocolCapabilityOverrides: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getModelProtocolCapabilities,
      syncModelProtocolCapabilities,
      updateModelProtocolCapabilityOverrides
    },
    settings: {
      getSettings
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (!params) return key
        return `${key}:${Object.values(params).join(':')}`
      }
    })
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

function capability(
  upstreamModel: string,
  protocol: AccountModelProtocolCapability['protocol'],
  overrides: Partial<AccountModelProtocolCapability> = {}
): AccountModelProtocolCapability {
  return {
    id: 1,
    account_id: 7,
    upstream_model: upstreamModel,
    protocol,
    override_state: 'auto',
    observed_state: 'unknown',
    effective_state: 'unknown',
    created_at: '2026-07-21T00:00:00Z',
    updated_at: '2026-07-21T00:00:00Z',
    ...overrides
  }
}

function mountModal(
  items: AccountModelProtocolCapability[],
  response: Partial<AccountModelProtocolCapabilitiesResponse> = {}
) {
  getModelProtocolCapabilities.mockResolvedValueOnce({
    account_id: 7,
    items,
    warnings: [],
    public_model_impacts: {},
    orphan_upstream_models: [],
    ...response
  })
  return mount(ModelProtocolCapabilitiesModal, {
    props: {
      show: true,
      account: { id: 7, name: 'new-api upstream' } as any
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        LoadingSpinner: true,
        Icon: true,
        RouterLink: true
      }
    }
  })
}

function rowFor(wrapper: ReturnType<typeof mountModal>, model: string) {
  const row = wrapper.findAll('tbody tr').find(candidate => candidate.find('td > div').text() === model)
  expect(row, `row for ${model}`).toBeTruthy()
  return row!
}

async function setOverride(
  row: ReturnType<typeof rowFor>,
  state: 'auto' | 'supported' | 'unsupported'
) {
  await row.get(`button[data-override-state="${state}"]`).trigger('click')
}

describe('ModelProtocolCapabilitiesModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSettings.mockResolvedValue({
      native_model_protocol_routing_enabled: false,
      native_model_protocol_routing_source: 'config'
    })
  })

  it('warns when saved capabilities cannot participate in routing yet', async () => {
    const wrapper = mountModal([])
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.modelProtocol.globalRoutingDisabled')
    expect(wrapper.text()).toContain('admin.accounts.modelProtocol.globalRoutingDisabledHint')
    expect(getSettings).toHaveBeenCalledOnce()

    wrapper.unmount()
  })

  it('does not misreport a failed global status request as disabled', async () => {
    getSettings.mockRejectedValueOnce(new Error('settings unavailable'))

    const wrapper = mountModal([])
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.modelProtocol.globalRoutingUnknown')
    expect(wrapper.text()).not.toContain('admin.accounts.modelProtocol.globalRoutingDisabledHint')

    wrapper.unmount()
  })

  it('uses the backend effective state until a relevant draft changes', async () => {
    const wrapper = mountModal([
      capability('MiniMax-M3', 'anthropic_messages', {
        observed_state: 'supported',
        observed_source: 'upstream_model_list',
        effective_state: 'unsupported',
        effective_source: 'future_backend_policy'
      })
    ])
    await flushPromises()

    const exactRow = rowFor(wrapper, 'MiniMax-M3')
    expect(exactRow.text()).toContain('admin.accounts.modelProtocol.states.unsupported')
    expect(exactRow.text()).toContain('future_backend_policy')

    wrapper.unmount()
  })

  it('previews a wildcard draft change across exact model rows', async () => {
    const wrapper = mountModal([
      capability('*', 'anthropic_messages', {
        override_state: 'unsupported',
        observed_state: 'unknown',
        effective_state: 'unsupported',
        effective_source: 'admin_override'
      }),
      capability('MiniMax-M3', 'anthropic_messages', {
        observed_state: 'supported',
        observed_source: 'upstream_model_list',
        observed_at: '2026-07-21T01:00:00Z',
        effective_state: 'unsupported',
        effective_source: 'admin_override'
      })
    ])
    await flushPromises()

    const wildcardRow = rowFor(wrapper, '*')
    const exactRow = rowFor(wrapper, 'MiniMax-M3')
    expect(exactRow.text()).toContain('admin.accounts.modelProtocol.states.unsupported')

    await setOverride(wildcardRow, 'auto')

    expect(exactRow.text()).toContain('admin.accounts.modelProtocol.states.supported')
    expect(exactRow.text()).toContain('admin.accounts.modelProtocol.sources.upstreamModelList')

    wrapper.unmount()
  })

  it('does not present an observation timestamp as draft override evidence', async () => {
    const wrapper = mountModal([
      capability('MiniMax-M3', 'anthropic_messages', {
        observed_state: 'supported',
        observed_source: 'upstream_model_list',
        observed_at: '2026-07-21T01:00:00Z',
        effective_state: 'supported',
        effective_source: 'upstream_model_list'
      })
    ])
    await flushPromises()

    const exactRow = rowFor(wrapper, 'MiniMax-M3')
    await setOverride(exactRow, 'unsupported')

    expect(exactRow.text()).toContain('admin.accounts.modelProtocol.sources.adminOverride')
    expect(exactRow.text()).not.toContain('2026')

    wrapper.unmount()
  })

  it('associates labels and exposes each override as an accessible three-state control', async () => {
    const wrapper = mountModal([
      capability('MiniMax-M3', 'anthropic_messages', {
        effective_state: 'supported',
        effective_source: 'upstream_model_list'
      })
    ])
    await flushPromises()

    expect(wrapper.get('label[for="model-protocol-manual-model"]').exists()).toBe(true)
    expect(wrapper.get('#model-protocol-manual-model').exists()).toBe(true)
    expect(wrapper.find('select').exists()).toBe(false)
    const groups = wrapper.findAll('[role="radiogroup"]')
    expect(groups).toHaveLength(6)
    for (const group of groups) {
      expect(group.attributes('aria-label')).toContain('admin.accounts.modelProtocol.overrideLabel')
      const radios = group.findAll('[role="radio"]')
      expect(radios).toHaveLength(3)
      expect(radios.filter(radio => radio.attributes('aria-checked') === 'true')).toHaveLength(1)
    }

    wrapper.unmount()
  })

  it('shows which public channel models use an upstream model', async () => {
    const wrapper = mountModal(
      [capability('MiniMax-M3-upstream', 'anthropic_messages')],
      {
        public_model_impacts: {
          'MiniMax-M3-upstream': [{
            upstream_model: 'MiniMax-M3-upstream',
            public_model: 'MiniMax-M3',
            channel_id: 9,
            channel_name: '国产模型',
            group_id: 10,
            group_name: 'OpenAI 主线路',
            platform: 'openai'
          }]
        }
      }
    )
    await flushPromises()

    const exactRow = rowFor(wrapper, 'MiniMax-M3-upstream')
    expect(exactRow.text()).toContain('MiniMax-M3')
    expect(exactRow.text()).toContain('国产模型')
    expect(exactRow.text()).toContain('OpenAI 主线路')
    expect(exactRow.text()).not.toContain('admin.accounts.modelProtocol.orphanCapability')

    wrapper.unmount()
  })

  it('shows orphan state only when the backend confirms the model has no public impact', async () => {
    const item = capability('unused-upstream-model', 'anthropic_messages')
    const confirmed = mountModal([item], { orphan_upstream_models: ['unused-upstream-model'] })
    await flushPromises()
    expect(rowFor(confirmed, 'unused-upstream-model').text()).toContain('admin.accounts.modelProtocol.orphanCapability')
    confirmed.unmount()

    const unresolved = mountModal([item], {
      warnings: ['Public model impact could not be resolved; capability facts are still available'],
      orphan_upstream_models: []
    })
    await flushPromises()
    expect(rowFor(unresolved, 'unused-upstream-model').text()).not.toContain('admin.accounts.modelProtocol.orphanCapability')
    unresolved.unmount()
  })

  it('saves administrator intent only and never sends observed capability fields', async () => {
    const item = capability('MiniMax-M3', 'anthropic_messages', {
      observed_state: 'supported',
      observed_source: 'upstream_model_list',
      effective_state: 'supported',
      effective_source: 'upstream_model_list'
    })
    updateModelProtocolCapabilityOverrides.mockResolvedValueOnce({
      account_id: 7,
      items: [item],
      warnings: [],
      public_model_impacts: {},
      orphan_upstream_models: ['MiniMax-M3']
    })
    const wrapper = mountModal([item])
    await flushPromises()

    await setOverride(rowFor(wrapper, 'MiniMax-M3'), 'unsupported')
    const saveButton = wrapper.findAll('button').find(button => button.text() === 'common.save')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(updateModelProtocolCapabilityOverrides).toHaveBeenCalledOnce()
    const payload = updateModelProtocolCapabilityOverrides.mock.calls[0][1]
    expect(payload).toContainEqual({
      upstream_model: 'MiniMax-M3',
      protocol: 'anthropic_messages',
      state: 'unsupported'
    })
    for (const override of payload) {
      expect(Object.keys(override).sort()).toEqual(['protocol', 'state', 'upstream_model'])
    }

    wrapper.unmount()
  })
})
