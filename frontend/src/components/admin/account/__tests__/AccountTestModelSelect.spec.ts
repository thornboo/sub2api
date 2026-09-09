import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, type ComponentPublicInstance } from 'vue'

import AccountTestModelSelect from '../AccountTestModelSelect.vue'

const { resolveTestModel } = vi.hoisted(() => ({
  resolveTestModel: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      resolveTestModel
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

type AccountTestModel = {
  id: string
  display_name?: string
  upstream_model_id?: string
  is_pattern?: boolean
  disabled?: boolean
}

const IconStub = { template: '<span />' }

let wrapper: VueWrapper<ComponentPublicInstance> | null = null

const model = (overrides: AccountTestModel): AccountTestModel => ({
  id: overrides.id,
  display_name: overrides.display_name ?? overrides.id,
  upstream_model_id: overrides.upstream_model_id,
  is_pattern: overrides.is_pattern,
  disabled: overrides.disabled
})

const mountSelect = (props: {
  modelValue?: string
  options: AccountTestModel[]
  accountId?: number
  active?: boolean
}) => {
  wrapper = mount(AccountTestModelSelect, {
    attachTo: document.body,
    props: {
      modelValue: props.modelValue ?? '',
      options: props.options,
      accountId: props.accountId ?? 42,
      active: props.active ?? true
    },
    global: {
      stubs: {
        Icon: IconStub
      }
    }
  })
  return wrapper
}

const openOptions = async (wrapper: VueWrapper<ComponentPublicInstance>) => {
  await wrapper.get('.select-trigger').trigger('click')
  await nextTick()
  return Array.from(document.body.querySelectorAll<HTMLElement>('[role="option"]'))
}

const chooseOptionContaining = async (wrapper: VueWrapper<ComponentPublicInstance>, text: string) => {
  const options = await openOptions(wrapper)
  const option = options.find((node) => node.textContent?.includes(text))
  expect(
    option,
    `expected an option containing "${text}", got: ${options.map((node) => node.textContent?.trim()).join(', ')}`
  ).toBeDefined()
  option!.dispatchEvent(new MouseEvent('click', { bubbles: true }))
  await nextTick()
}

const emittedValues = (wrapper: VueWrapper<ComponentPublicInstance>) =>
  (wrapper.emitted('update:modelValue') ?? []).map(([value]) => value)

const emittedTargets = (wrapper: VueWrapper<ComponentPublicInstance>) =>
  (wrapper.emitted('update:resolvedTarget') ?? []).map(([value]) => value)

type ResolveModelResult = { model_id: string; upstream_model_id: string }

const deferredResolve = () => {
  let resolve!: (value: ResolveModelResult) => void
  const promise = new Promise<ResolveModelResult>((innerResolve) => {
    resolve = innerResolve
  })
  return { promise, resolve }
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('AccountTestModelSelect', () => {
  beforeEach(() => {
    resolveTestModel.mockResolvedValue({
      model_id: 'claude-opus-4-1-20260805',
      upstream_model_id: 'claude-opus-4-1-20260805'
    })
  })

  it('emits the left-side alias id when an alias targets a different upstream model', async () => {
    const wrapper = mountSelect({
      options: [
        model({ id: 'alpha-alias', upstream_model_id: 'beta-target' }),
        model({ id: 'beta-target', upstream_model_id: 'gamma-target' })
      ]
    })

    await chooseOptionContaining(wrapper, 'alpha-alias → beta-target')

    expect(emittedValues(wrapper)).toContain('alpha-alias')
  })

  it('keeps two aliases visible when both target the same upstream model', async () => {
    const wrapper = mountSelect({
      options: [
        model({ id: 'fast-sonnet', upstream_model_id: 'claude-sonnet-4-20250514' }),
        model({ id: 'cheap-sonnet', upstream_model_id: 'claude-sonnet-4-20250514' })
      ]
    })

    const labels = (await openOptions(wrapper)).map((node) => node.textContent ?? '')

    expect(
      labels.filter((label) => label.includes('claude-sonnet-4-20250514')),
      `expected both aliases to show their shared target, got: ${labels.join(', ')}`
    ).toHaveLength(2)
  })

  it('shows the saved raw model id when the value is not in the current options', () => {
    const wrapper = mountSelect({
      modelValue: 'claude-saved-custom-20260909',
      options: [
        model({ id: 'claude-sonnet-4-20250514', upstream_model_id: 'claude-sonnet-4-20250514' })
      ]
    })

    expect(wrapper.get('.select-trigger').text()).toContain('claude-saved-custom-20260909')
  })

  it('shows the raw id for a model without configured metadata', async () => {
    const wrapper = mountSelect({
      options: [
        model({ id: 'gpt-4.1-mini', display_name: 'friendly display name' })
      ]
    })

    const labels = (await openOptions(wrapper)).map((node) => node.textContent ?? '')

    expect(labels).toContain('gpt-4.1-mini')
    expect(labels).not.toContain('friendly display name')
  })

  it('clears the submitted value when an external value selects a disabled model', () => {
    const wrapper = mountSelect({
      modelValue: 'invalid-bedrock-alias',
      options: [
        model({ id: 'invalid-bedrock-alias', upstream_model_id: '', disabled: true })
      ]
    })

    expect(emittedValues(wrapper).at(-1)).toBe('')
    expect(emittedTargets(wrapper).at(-1)).toBe('')
  })

  it('emits the concrete id after a wildcard option resolves successfully', async () => {
    vi.useFakeTimers()
    const wrapper = mountSelect({
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    expect(emittedValues(wrapper).at(-1)).toBe('')

    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-opus-4-1-20260805')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    expect(resolveTestModel).toHaveBeenCalledWith(
      42,
      'claude-opus-4-1-20260805',
      expect.any(AbortSignal)
    )
    expect(emittedValues(wrapper).at(-1)).toBe('claude-opus-4-1-20260805')
    expect(emittedTargets(wrapper).at(-1)).toBe('claude-opus-4-1-20260805')
  })

  it('rejects wildcard characters in the concrete id without resolving', async () => {
    vi.useFakeTimers()
    const wrapper = mountSelect({
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-*')
    await vi.advanceTimersByTimeAsync(300)

    expect(resolveTestModel).not.toHaveBeenCalled()
    expect(emittedValues(wrapper).at(-1)).toBe('')
  })

  it('clears the submitted value when concrete id resolution fails', async () => {
    vi.useFakeTimers()
    resolveTestModel.mockRejectedValueOnce(new Error('not found'))
    const wrapper = mountSelect({
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-missing')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()

    expect(emittedValues(wrapper).at(-1)).toBe('')
  })

  it('ignores a stale wildcard resolution after a newer input has resolved', async () => {
    vi.useFakeTimers()
    let resolveFirst!: (value: { model_id: string; upstream_model_id: string }) => void
    let resolveSecond!: (value: { model_id: string; upstream_model_id: string }) => void
    resolveTestModel
      .mockReturnValueOnce(new Promise((resolve) => { resolveFirst = resolve }))
      .mockReturnValueOnce(new Promise((resolve) => { resolveSecond = resolve }))

    const wrapper = mountSelect({
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-slow')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-fast')
    await vi.advanceTimersByTimeAsync(300)

    resolveSecond({ model_id: 'claude-fast', upstream_model_id: 'claude-fast' })
    await flushPromises()
    expect(emittedValues(wrapper).at(-1)).toBe('claude-fast')

    resolveFirst({ model_id: 'claude-slow', upstream_model_id: 'claude-slow' })
    await flushPromises()
    expect(emittedValues(wrapper).at(-1)).toBe('claude-fast')
  })

  it('ignores a wildcard resolution from the previous account', async () => {
    vi.useFakeTimers()
    const first = deferredResolve()
    resolveTestModel.mockReturnValueOnce(first.promise)
    const wrapper = mountSelect({
      accountId: 42,
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-old-account')
    await vi.advanceTimersByTimeAsync(300)
    await wrapper.setProps({ accountId: 43 })

    first.resolve({ model_id: 'claude-old-account', upstream_model_id: 'claude-old-account' })
    await flushPromises()

    expect(emittedValues(wrapper).at(-1)).toBe('')
  })

  it.each([
    {
      name: 'when the selector closes',
      invalidate: async (wrapper: VueWrapper<ComponentPublicInstance>) => {
        await wrapper.setProps({ active: false })
      },
      expectedValue: ''
    },
    {
      name: 'when a normal option is selected',
      invalidate: async (wrapper: VueWrapper<ComponentPublicInstance>) => {
        await chooseOptionContaining(wrapper, 'claude-sonnet-4-20250514')
      },
      expectedValue: 'claude-sonnet-4-20250514'
    }
  ])('ignores a stale wildcard resolution $name', async ({ invalidate, expectedValue }) => {
    vi.useFakeTimers()
    const first = deferredResolve()
    resolveTestModel.mockReturnValueOnce(first.promise)
    const wrapper = mountSelect({
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true }),
        model({ id: 'claude-sonnet-4-20250514', upstream_model_id: 'claude-sonnet-4-20250514' })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-old')
    await vi.advanceTimersByTimeAsync(300)
    await invalidate(wrapper)

    first.resolve({ model_id: 'claude-old', upstream_model_id: 'claude-old' })
    await flushPromises()

    expect(emittedValues(wrapper).at(-1)).toBe(expectedValue)
  })

  it('clears the submitted value immediately when editing after a successful wildcard resolution', async () => {
    vi.useFakeTimers()
    const wrapper = mountSelect({
      options: [
        model({ id: 'claude-*', upstream_model_id: 'claude-*', is_pattern: true })
      ]
    })

    await chooseOptionContaining(wrapper, 'claude-*')
    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-opus-4-1-20260805')
    await vi.advanceTimersByTimeAsync(300)
    await flushPromises()
    expect(emittedValues(wrapper).at(-1)).toBe('claude-opus-4-1-20260805')

    await wrapper.get('[data-testid="account-test-concrete-model"] input').setValue('claude-opus-4-2-20260909')

    expect(emittedValues(wrapper).at(-1)).toBe('')
  })
})
