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

function chatControls(wrapper: ReturnType<typeof mountModal>, model: string) {
  return rowFor(wrapper, model).findAll('[role="radiogroup"]')[1]
}

function protocolCell(wrapper: ReturnType<typeof mountModal>, model: string, protocol: 'anthropic_messages' | 'openai_chat_completions' | 'openai_responses') {
  const protocolIndex = {
    anthropic_messages: 1,
    openai_chat_completions: 2,
    openai_responses: 3
  }[protocol]
  return rowFor(wrapper, model).findAll('td')[protocolIndex]
}

function saveButton(wrapper: ReturnType<typeof mountModal>) {
  const button = wrapper.findAll('button').find(button => button.text() === 'common.save')
  expect(button, 'save button').toBeTruthy()
  return button!
}

async function syncCapabilities(wrapper: ReturnType<typeof mountModal>) {
  const button = wrapper.findAll('button').find(button => button.text() === 'admin.accounts.modelProtocol.sync')
  await button!.trigger('click')
  await flushPromises()
}

function syncResult(
  items: AccountModelProtocolCapability[],
  observations: Array<{ upstream_model: string; protocol: string; state: string }> = []
) {
  return {
    account_id: 7,
    items,
    warnings: [],
    public_model_impacts: {},
    orphan_upstream_models: [],
    synced_observations: observations.map(observation => ({
      ...observation,
      source: 'upstream_model_list',
      observed_at: '2026-09-08T01:00:00Z'
    }))
  }
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
    const observedEvidence = protocolCell(wrapper, 'MiniMax-M3', 'anthropic_messages').find('[data-observed-evidence]')
    expect(observedEvidence.text()).toContain('admin.accounts.modelProtocol.observedEvidence')
    expect(observedEvidence.text()).toContain('admin.accounts.modelProtocol.states.supported')
    expect(observedEvidence.text()).toContain('admin.accounts.modelProtocol.sources.upstreamModelList')
    expect(observedEvidence.text()).toContain('2026')

    wrapper.unmount()
  })

  it('does not fabricate observed evidence when upstream has not provided an observation', async () => {
    const wrapper = mountModal([
      capability('MiniMax-M3', 'anthropic_messages', {
        override_state: 'unsupported',
        observed_state: 'unknown',
        observed_source: undefined,
        observed_at: undefined,
        effective_state: 'unsupported',
        effective_source: 'admin_override'
      })
    ])
    await flushPromises()

    expect(protocolCell(wrapper, 'MiniMax-M3', 'anthropic_messages').find('[data-observed-evidence]').exists()).toBe(false)

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

  it('shows only the upstream models scoped by the account mapping', async () => {
    const wrapper = mountModal(
      [
        capability('glm-5', 'anthropic_messages'),
        capability('kimi-k2.5', 'anthropic_messages'),
        capability('minimax-m2.5', 'anthropic_messages'),
        capability('MiniMax-M2.7', 'anthropic_messages')
      ],
      {
        models: ['minimax-m2.5', 'MiniMax-M2.7'],
        mapping_restricted: true
      }
    )
    await flushPromises()

    expect(rowFor(wrapper, 'minimax-m2.5').exists()).toBe(true)
    expect(rowFor(wrapper, 'MiniMax-M2.7').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('glm-5')
    expect(wrapper.text()).not.toContain('kimi-k2.5')
    expect(wrapper.find('#model-protocol-manual-model').exists()).toBe(false)

    wrapper.unmount()
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
    expect(wrapper.emitted('close')).toHaveLength(1)

    wrapper.unmount()
  })

  it.each(['supported', 'unsupported'] as const)('selects a freshly synced %s result over a saved manual choice', async state => {
    const previousState = state === 'supported' ? 'unsupported' : 'supported'
    const item = capability('gpt-test', 'openai_chat_completions', {
      override_state: previousState,
      observed_state: state,
      observed_source: 'upstream_model_list',
      observed_at: '2026-09-08T01:00:00Z',
      effective_state: previousState,
      effective_source: 'admin_override'
    })
    const wrapper = mountModal([item])
    await flushPromises()
    syncModelProtocolCapabilities.mockResolvedValueOnce(syncResult([item], [
      { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state }
    ]))

    await syncCapabilities(wrapper)

    expect(chatControls(wrapper, 'gpt-test').get(`[data-override-state="${state}"]`).attributes('aria-checked')).toBe('true')
    const evidence = protocolCell(wrapper, 'gpt-test', 'openai_chat_completions').get('[data-observed-evidence]')
    expect(evidence.text()).toContain(`admin.accounts.modelProtocol.states.${state}`)
    expect(evidence.text()).toContain('admin.accounts.modelProtocol.sources.upstreamModelList')
    expect(evidence.text()).toContain('2026')
    expect(updateModelProtocolCapabilityOverrides).not.toHaveBeenCalled()
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it.each(['unknown', 'missing', 'legacy'] as const)('preserves unsaved choices when fresh capability evidence is %s', async evidence => {
    const item = capability('gpt-test', 'openai_chat_completions', {
      observed_state: 'supported',
      effective_state: 'supported',
      observed_source: 'upstream_model_list'
    })
    const wrapper = mountModal([item])
    await flushPromises()
    await chatControls(wrapper, 'gpt-test').get('[data-override-state="unsupported"]').trigger('click')
    await chatControls(wrapper, '*').get('[data-override-state="unsupported"]').trigger('click')
    const result = syncResult([item], evidence === 'unknown' ? [
      { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state: 'unknown' }
    ] : [])
    syncModelProtocolCapabilities.mockResolvedValueOnce(evidence === 'legacy'
      ? { ...result, synced_observations: undefined }
      : result)

    await syncCapabilities(wrapper)

    expect(chatControls(wrapper, 'gpt-test').get('[data-override-state="unsupported"]').attributes('aria-checked')).toBe('true')
    expect(chatControls(wrapper, '*').get('[data-override-state="unsupported"]').attributes('aria-checked')).toBe('true')
    wrapper.unmount()
  })

  it('fills new model choices without changing the account default or unsupported protocol controls', async () => {
    const wrapper = mountModal([])
    await flushPromises()
    const item = capability('new-model', 'openai_chat_completions', { observed_state: 'supported' })
    syncModelProtocolCapabilities.mockResolvedValueOnce(syncResult([item], [
      { upstream_model: 'new-model', protocol: 'openai_chat_completions', state: 'supported' },
      { upstream_model: '*', protocol: 'openai_chat_completions', state: 'supported' },
      { upstream_model: 'new-model', protocol: 'openai_images', state: 'supported' }
    ]))

    await syncCapabilities(wrapper)

    expect(chatControls(wrapper, 'new-model').get('[data-override-state="supported"]').attributes('aria-checked')).toBe('true')
    expect(chatControls(wrapper, '*').get('[data-override-state="auto"]').attributes('aria-checked')).toBe('true')
    expect(rowFor(wrapper, 'new-model').findAll('[role="radiogroup"]')).toHaveLength(3)
    wrapper.unmount()
  })

  it('replaces unsaved manual choices on every sync and saves subsequent adjustments only when requested', async () => {
    const item = capability('gpt-test', 'openai_chat_completions')
    const wrapper = mountModal([item])
    await flushPromises()
    syncModelProtocolCapabilities
      .mockResolvedValueOnce(syncResult([item], [
        { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state: 'supported' }
      ]))
      .mockResolvedValueOnce(syncResult([item], [
        { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state: 'supported' }
      ]))

    await syncCapabilities(wrapper)
    await chatControls(wrapper, 'gpt-test').get('[data-override-state="unsupported"]').trigger('click')
    await syncCapabilities(wrapper)

    expect(chatControls(wrapper, 'gpt-test').get('[data-override-state="supported"]').attributes('aria-checked')).toBe('true')
    expect(syncModelProtocolCapabilities).toHaveBeenCalledTimes(2)
    expect(updateModelProtocolCapabilityOverrides).not.toHaveBeenCalled()
    // The administrator can still adjust another protocol before saving all choices.
    await setOverride(rowFor(wrapper, 'gpt-test'), 'unsupported')
    updateModelProtocolCapabilityOverrides.mockResolvedValueOnce(syncResult([item]))
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(updateModelProtocolCapabilityOverrides).toHaveBeenCalledWith(7, expect.arrayContaining([
      { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state: 'supported' },
      { upstream_model: 'gpt-test', protocol: 'anthropic_messages', state: 'unsupported' }
    ]))
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('retains unsaved choices and keeps the dialog open when sync fails', async () => {
    const wrapper = mountModal([capability('gpt-test', 'openai_chat_completions')])
    await flushPromises()
    await chatControls(wrapper, 'gpt-test').get('[data-override-state="supported"]').trigger('click')
    syncModelProtocolCapabilities.mockRejectedValueOnce(new Error('upstream unavailable'))

    await syncCapabilities(wrapper)

    expect(chatControls(wrapper, 'gpt-test').get('[data-override-state="supported"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.text()).toContain('upstream unavailable')
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(updateModelProtocolCapabilityOverrides).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('does not submit a previous account draft after the next account first load fails', async () => {
    const wrapper = mountModal([
      capability('*', 'openai_chat_completions', { override_state: 'supported', effective_state: 'supported', effective_source: 'admin_override' }),
      capability('shared-model', 'openai_chat_completions', { override_state: 'unsupported', effective_state: 'unsupported', effective_source: 'admin_override' }),
      capability('a-only-model', 'openai_chat_completions', { override_state: 'supported', effective_state: 'supported', effective_source: 'admin_override' })
    ])
    await flushPromises()
    await chatControls(wrapper, '*').get('[data-override-state="unsupported"]').trigger('click')
    await chatControls(wrapper, 'shared-model').get('[data-override-state="supported"]').trigger('click')

    getModelProtocolCapabilities.mockRejectedValueOnce(new Error('initial load failed'))
    await wrapper.setProps({ account: { id: 8, name: 'second account' } as any })
    await flushPromises()

    expect(saveButton(wrapper).attributes('disabled')).toBeDefined()

    const bItem = capability('shared-model', 'openai_chat_completions', {
      account_id: 8,
      observed_state: 'unknown',
      effective_state: 'unknown'
    })
    syncModelProtocolCapabilities.mockResolvedValueOnce(syncResult([bItem], [
      { upstream_model: 'shared-model', protocol: 'openai_chat_completions', state: 'unknown' }
    ]))
    await syncCapabilities(wrapper)

    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined()
    updateModelProtocolCapabilityOverrides.mockResolvedValueOnce(syncResult([bItem]))
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(updateModelProtocolCapabilityOverrides).toHaveBeenCalledOnce()
    expect(updateModelProtocolCapabilityOverrides.mock.calls[0][0]).toBe(8)
    const payload = updateModelProtocolCapabilityOverrides.mock.calls[0][1]
    expect(payload).not.toContainEqual(expect.objectContaining({ upstream_model: 'a-only-model' }))
    expect(payload).toContainEqual({ upstream_model: '*', protocol: 'openai_chat_completions', state: 'auto' })
    expect(payload).toContainEqual({ upstream_model: 'shared-model', protocol: 'openai_chat_completions', state: 'auto' })
    wrapper.unmount()
  })

  it('does not retain cancelled drafts when the same account is reopened and reloaded by sync', async () => {
    const item = capability('gpt-test', 'openai_chat_completions', {
      observed_state: 'unknown',
      effective_state: 'unknown'
    })
    const wrapper = mountModal([item])
    await flushPromises()
    await chatControls(wrapper, 'gpt-test').get('[data-override-state="unsupported"]').trigger('click')

    await wrapper.setProps({ show: false })
    await flushPromises()
    getModelProtocolCapabilities.mockRejectedValueOnce(new Error('initial load failed'))
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(saveButton(wrapper).attributes('disabled')).toBeDefined()

    syncModelProtocolCapabilities.mockResolvedValueOnce(syncResult([item], [
      { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state: 'unknown' }
    ]))
    await syncCapabilities(wrapper)

    expect(saveButton(wrapper).attributes('disabled')).toBeUndefined()
    updateModelProtocolCapabilityOverrides.mockResolvedValueOnce(syncResult([item]))
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(updateModelProtocolCapabilityOverrides.mock.calls[0][1]).toContainEqual({
      upstream_model: 'gpt-test',
      protocol: 'openai_chat_completions',
      state: 'auto'
    })
    wrapper.unmount()
  })

  it('ignores a sync response from a previously selected account', async () => {
    const wrapper = mountModal([capability('first-model', 'openai_chat_completions')])
    await flushPromises()
    let resolveSync!: (value: ReturnType<typeof syncResult>) => void
    syncModelProtocolCapabilities.mockReturnValueOnce(new Promise(resolve => { resolveSync = resolve }))
    await syncCapabilities(wrapper)
    getModelProtocolCapabilities.mockResolvedValueOnce(syncResult([capability('second-model', 'openai_chat_completions')]))
    await wrapper.setProps({ account: { id: 8, name: 'second account' } as any })
    await flushPromises()

    resolveSync(syncResult([capability('first-model', 'openai_chat_completions')], [
      { upstream_model: 'first-model', protocol: 'openai_chat_completions', state: 'supported' }
    ]))
    await flushPromises()

    expect(wrapper.text()).not.toContain('first-model')
    expect(chatControls(wrapper, 'second-model').get('[data-override-state="auto"]').attributes('aria-checked')).toBe('true')
    expect(showSuccess).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('marks a synced choice as pending until it is saved successfully', async () => {
    const savedAuto = capability('gpt-test', 'openai_chat_completions', {
      override_state: 'auto',
      observed_state: 'unknown',
      effective_state: 'unknown'
    })
    const savedSupported = capability('gpt-test', 'openai_chat_completions', {
      override_state: 'supported',
      observed_state: 'supported',
      observed_source: 'upstream_model_list',
      observed_at: '2026-09-08T01:00:00Z',
      effective_state: 'supported',
      effective_source: 'admin_override'
    })
    const wrapper = mountModal([savedAuto])
    await flushPromises()
    syncModelProtocolCapabilities.mockResolvedValueOnce(syncResult([savedAuto], [
      { upstream_model: 'gpt-test', protocol: 'openai_chat_completions', state: 'supported' }
    ]))

    await syncCapabilities(wrapper)

    expect(protocolCell(wrapper, 'gpt-test', 'openai_chat_completions').text()).toContain('admin.accounts.modelProtocol.afterSaveState')
    expect(protocolCell(wrapper, 'gpt-test', 'openai_chat_completions').text()).not.toContain('admin.accounts.modelProtocol.effectiveState')

    updateModelProtocolCapabilityOverrides.mockRejectedValueOnce(new Error('save failed'))
    await saveButton(wrapper).trigger('click')
    await flushPromises()

    expect(protocolCell(wrapper, 'gpt-test', 'openai_chat_completions').text()).toContain('admin.accounts.modelProtocol.afterSaveState')

    updateModelProtocolCapabilityOverrides.mockResolvedValueOnce(syncResult([savedSupported]))
    await saveButton(wrapper).trigger('click')
    await flushPromises()
    getModelProtocolCapabilities.mockResolvedValueOnce(syncResult([savedSupported]))
    await wrapper.setProps({ show: false })
    await flushPromises()
    await wrapper.setProps({ show: true, account: { id: 7, name: 'new-api upstream' } as any })
    await flushPromises()

    expect(protocolCell(wrapper, 'gpt-test', 'openai_chat_completions').text()).toContain('admin.accounts.modelProtocol.effectiveState')
    expect(protocolCell(wrapper, 'gpt-test', 'openai_chat_completions').text()).not.toContain('admin.accounts.modelProtocol.afterSaveState')
    wrapper.unmount()
  })

  it('marks inherited wildcard drafts as pending without marking exact saved overrides', async () => {
    const wrapper = mountModal([
      capability('*', 'openai_chat_completions', {
        override_state: 'unsupported',
        effective_state: 'unsupported',
        effective_source: 'admin_override'
      }),
      capability('inherits-default', 'openai_chat_completions', {
        override_state: 'auto',
        effective_state: 'unsupported',
        effective_source: 'admin_override'
      }),
      capability('has-exact-override', 'openai_chat_completions', {
        override_state: 'supported',
        effective_state: 'supported',
        effective_source: 'admin_override'
      })
    ])
    await flushPromises()

    await chatControls(wrapper, '*').get('[data-override-state="supported"]').trigger('click')

    expect(protocolCell(wrapper, 'inherits-default', 'openai_chat_completions').text()).toContain('admin.accounts.modelProtocol.afterSaveState')
    expect(protocolCell(wrapper, 'inherits-default', 'openai_chat_completions').text()).toContain('admin.accounts.modelProtocol.states.supported')
    expect(protocolCell(wrapper, 'has-exact-override', 'openai_chat_completions').text()).toContain('admin.accounts.modelProtocol.effectiveState')
    expect(protocolCell(wrapper, 'has-exact-override', 'openai_chat_completions').text()).not.toContain('admin.accounts.modelProtocol.afterSaveState')
    wrapper.unmount()
  })

  it('previews a new account default before any wildcard record has been saved', async () => {
    const wrapper = mountModal([
      capability('inherits-new-default', 'openai_chat_completions', {
        observed_state: 'supported',
        observed_source: 'upstream_model_list',
        effective_state: 'supported',
        effective_source: 'upstream_model_list'
      })
    ])
    await flushPromises()

    await chatControls(wrapper, '*').get('[data-override-state="unsupported"]').trigger('click')

    const cell = protocolCell(wrapper, 'inherits-new-default', 'openai_chat_completions')
    expect(cell.text()).toContain('admin.accounts.modelProtocol.afterSaveState')
    expect(cell.text()).toContain('admin.accounts.modelProtocol.states.unsupported')
    expect(cell.get('[data-observed-evidence]').text()).toContain('admin.accounts.modelProtocol.states.supported')
    wrapper.unmount()
  })

  it('keeps the modal open when saving overrides fails', async () => {
    const item = capability('MiniMax-M3', 'anthropic_messages')
    updateModelProtocolCapabilityOverrides.mockRejectedValueOnce(new Error('save failed'))
    const wrapper = mountModal([item])
    await flushPromises()

    const saveButton = wrapper.findAll('button').find(button => button.text() === 'common.save')
    expect(saveButton).toBeTruthy()
    await saveButton!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.text()).toContain('save failed')

    wrapper.unmount()
  })
})
