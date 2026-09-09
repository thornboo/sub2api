import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, type ComponentPublicInstance } from 'vue'

import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

const { listByAccount, createPlan, updatePlan, listResults, resolveTestModel, showSuccess, showError } = vi.hoisted(() => ({
  listByAccount: vi.fn(),
  createPlan: vi.fn(),
  updatePlan: vi.fn(),
  listResults: vi.fn(),
  resolveTestModel: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      resolveTestModel
    },
    scheduledTests: {
      listByAccount,
      create: createPlan,
      update: updatePlan,
      delete: vi.fn(),
      listResults
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const SelectStub = {
  props: ['modelValue', 'options', 'valueKey', 'labelKey'],
  emits: ['update:modelValue'],
  methods: {
    optionValue(option: Record<string, unknown>) {
      return option[String(this.valueKey || 'value')]
    },
    optionLabel(option: Record<string, unknown>) {
      return option[String(this.labelKey || 'label')]
    }
  },
  template: `
    <select
      data-testid="scheduled-model-select"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option
        v-if="modelValue && !options.some((option) => optionValue(option) === modelValue)"
        :value="modelValue"
      >
        {{ modelValue }}
      </option>
      <option v-for="option in options" :key="optionValue(option)" :value="optionValue(option)" :disabled="option.disabled">
        {{ optionLabel(option) }}
      </option>
    </select>
  `
}

const InputStub = {
  props: ['modelValue', 'type'],
  emits: ['update:modelValue'],
  template: '<input :type="type || \'text\'" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
}

const ToggleStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<input type="checkbox" :checked="modelValue" @change="$emit(\'update:modelValue\', $event.target.checked)" />'
}

const mountPanel = async () => {
  const wrapper = mount(ScheduledTestsPanel, {
    props: {
      show: false,
      accountId: 42,
      modelOptions: [
        { value: 'alpha-alias', label: 'Alpha Alias', upstream_model_id: 'beta-target' },
        { value: 'beta-alias', label: 'Beta Alias', upstream_model_id: 'beta-target' },
        { value: 'claude-sonnet-4-20250514', label: 'Claude Sonnet', upstream_model_id: 'claude-sonnet-4-20250514' },
        { value: 'claude-*', label: 'Claude wildcard', upstream_model_id: 'claude-*', is_pattern: true },
        { value: 'invalid-bedrock-alias', label: 'Invalid Bedrock Alias', upstream_model_id: '', disabled: true }
      ]
    },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        HelpTooltip: { template: '<span><slot name="trigger" /><slot /></span>' },
        Select: SelectStub,
        Input: InputStub,
        Toggle: ToggleStub,
        Icon: true
      }
    }
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

const openModelOptions = async (wrapper: VueWrapper<ComponentPublicInstance>) => {
  await wrapper.get('.select-trigger').trigger('click')
  await nextTick()
  return Array.from(document.body.querySelectorAll<HTMLElement>('[role="option"]'))
}

const chooseModelOption = async (wrapper: VueWrapper<ComponentPublicInstance>, text: string) => {
  const stubSelect = wrapper.find('[data-testid="scheduled-model-select"]')
  if (stubSelect.exists()) {
    const options = stubSelect.findAll('option')
    const option = options.find((node) => node.text().includes(text))
    expect(
      option,
      `expected model option containing "${text}", got: ${options.map((node) => node.text()).join(', ')}`
    ).toBeDefined()
    await stubSelect.setValue(option!.attributes('value'))
    return
  }

  const options = await openModelOptions(wrapper)
  const option = options.find((node) => node.textContent?.includes(text))
  expect(
    option,
    `expected model option containing "${text}", got: ${options.map((node) => node.textContent?.trim()).join(', ')}`
  ).toBeDefined()
  option!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  await nextTick()
}

const setCronExpression = async (wrapper: VueWrapper<ComponentPublicInstance>, value: string) => {
  const cronInput = wrapper.findAll('input').find((input) =>
    input.attributes('type') !== 'checkbox' &&
    input.attributes('type') !== 'number' &&
    input.attributes('autocomplete') !== 'off'
  )
  expect(cronInput, 'expected cron expression input').toBeDefined()
  await cronInput!.setValue(value)
}

afterEach(() => {
  document.body.innerHTML = ''
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('ScheduledTestsPanel model selection', () => {
  beforeEach(() => {
    listByAccount.mockResolvedValue([])
    createPlan.mockResolvedValue({
      id: 1,
      account_id: 42,
      model_id: 'alpha-alias',
      cron_expression: '*/15 * * * *',
      enabled: true,
      max_results: 100,
      auto_recover: false,
      last_run_at: null,
      next_run_at: null,
      created_at: '2026-09-09T00:00:00Z',
      updated_at: '2026-09-09T00:00:00Z'
    })
    updatePlan.mockResolvedValue({
      id: 2,
      account_id: 42,
      model_id: 'claude-saved-custom-20260909',
      cron_expression: '0 * * * *',
      enabled: true,
      max_results: 20,
      auto_recover: false,
      last_run_at: null,
      next_run_at: null,
      created_at: '2026-09-09T00:00:00Z',
      updated_at: '2026-09-09T00:00:00Z'
    })
    listResults.mockResolvedValue([])
    resolveTestModel.mockResolvedValue({
      model_id: 'claude-opus-4-1-20260805',
      upstream_model_id: 'claude-opus-4-1-20260805'
    })
  })

  it('creates a scheduled test with the selected left-side alias id', async () => {
    const wrapper = await mountPanel()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.scheduledTests.addPlan'))!.trigger('click')
    await chooseModelOption(wrapper, 'alpha-alias → beta-target')
    await setCronExpression(wrapper, '*/15 * * * *')
    await wrapper.findAll('button').find((button) => button.text().includes('common.save'))!.trigger('click')
    await flushPromises()

    expect(createPlan).toHaveBeenCalledWith(expect.objectContaining({
      account_id: 42,
      model_id: 'alpha-alias',
      cron_expression: '*/15 * * * *'
    }))
  })

  it('creates a scheduled test with the resolved concrete wildcard model id', async () => {
    vi.useFakeTimers()
    const wrapper = await mountPanel()

    await wrapper.findAll('button').find((button) => button.text().includes('admin.scheduledTests.addPlan'))!.trigger('click')
    await chooseModelOption(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-opus-4-1-20260805')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    await setCronExpression(wrapper, '*/20 * * * *')
    await wrapper.findAll('button').find((button) => button.text().includes('common.save'))!.trigger('click')
    await flushPromises()

    expect(resolveTestModel).toHaveBeenCalledWith(
      42,
      'claude-opus-4-1-20260805',
      expect.any(AbortSignal)
    )
    expect(createPlan).toHaveBeenCalledWith(expect.objectContaining({
      account_id: 42,
      model_id: 'claude-opus-4-1-20260805',
      cron_expression: '*/20 * * * *'
    }))
  })

  it('updates a scheduled test without dropping a saved concrete id missing from options', async () => {
    listByAccount.mockResolvedValueOnce([
      {
        id: 2,
        account_id: 42,
        model_id: 'claude-saved-custom-20260909',
        cron_expression: '0 * * * *',
        enabled: true,
        max_results: 20,
        auto_recover: false,
        last_run_at: null,
        next_run_at: null,
        created_at: '2026-09-09T00:00:00Z',
        updated_at: '2026-09-09T00:00:00Z'
      }
    ])
    const wrapper = await mountPanel()

    await wrapper.get('button[title="admin.scheduledTests.editPlan"]').trigger('click')
    await wrapper.findAll('button').find((button) => button.text().includes('common.save'))!.trigger('click')
    await flushPromises()

    expect(updatePlan).toHaveBeenCalledWith(2, expect.objectContaining({
      model_id: 'claude-saved-custom-20260909',
      cron_expression: '0 * * * *'
    }))
  })

  it('does not update a scheduled test whose saved configured option is disabled', async () => {
    listByAccount.mockResolvedValueOnce([
      {
        id: 3,
        account_id: 42,
        model_id: 'invalid-bedrock-alias',
        cron_expression: '0 * * * *',
        enabled: true,
        max_results: 20,
        auto_recover: false,
        last_run_at: null,
        next_run_at: null,
        created_at: '2026-09-09T00:00:00Z',
        updated_at: '2026-09-09T00:00:00Z'
      }
    ])
    const wrapper = await mountPanel()

    await wrapper.get('button[title="admin.scheduledTests.editPlan"]').trigger('click')
    await flushPromises()
    const saveButton = wrapper.findAll('button').find((button) => button.text().includes('common.save'))!

    expect(saveButton.attributes('disabled')).toBeDefined()
    await saveButton.trigger('click')

    expect(updatePlan).not.toHaveBeenCalled()
  })
})
