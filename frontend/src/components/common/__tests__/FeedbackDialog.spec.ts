import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import FeedbackDialog from '../FeedbackDialog.vue'

enableAutoUnmount(afterEach)

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key,
    }),
  }
})

const { showSuccess } = vi.hoisted(() => ({ showSuccess: vi.fn() }))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess }),
}))

function mountDialog(submitter = vi.fn().mockResolvedValue({ id: 1, created_at: '2026-09-08T00:00:00Z', retry_after: 60 })) {
  return {
    submitter,
    wrapper: mount(FeedbackDialog, {
      props: {
        show: true,
        identityKey: 'user:1',
        submitter,
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<section v-if="show" class="dialog-stub"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
          },
          Icon: true,
        },
      },
    }),
  }
}

describe('FeedbackDialog', () => {
  it('requires a title and validates its Unicode length independently of the description', async () => {
    const { wrapper, submitter } = mountDialog()
    await wrapper.get('textarea').setValue('Please check my request')
    await wrapper.get('form').trigger('submit')
    expect(submitter).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('feedback.titleRequired')

    await wrapper.get('#feedback-title').setValue('🙂'.repeat(121))
    await wrapper.get('form').trigger('submit')
    expect(submitter).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('feedback.titleTooLong')

    await wrapper.get('#feedback-title').setValue('🙂'.repeat(120))
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(submitter).toHaveBeenCalledWith({ title: '🙂'.repeat(120), content: 'Please check my request' }, expect.any(AbortSignal))
  })

  it('requires trimmed text and counts Unicode code points', async () => {
    const { wrapper, submitter } = mountDialog()

    await wrapper.get('textarea').setValue('   ')
    await wrapper.get('form').trigger('submit')
    expect(submitter).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('feedback.emptyError')

    await wrapper.get('textarea').setValue('🙂'.repeat(2001))
    await wrapper.get('form').trigger('submit')
    expect(submitter).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('feedback.tooLongError')
    expect(wrapper.text()).toContain('2001 / 2000')
  })

  it('submits plain text, shows the shared toast, and closes after success', async () => {
    const { wrapper, submitter } = mountDialog()

    await wrapper.get('#feedback-title').setValue('  Help   with request  ')
    await wrapper.get('textarea').setValue('<img src=x onerror=alert(1)>hello')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(submitter).toHaveBeenCalledWith({ title: 'Help with request', content: '<img src=x onerror=alert(1)>hello' }, expect.any(AbortSignal))
    expect(wrapper.get<HTMLInputElement>('#feedback-title').element.value).toBe('')
    expect(wrapper.html()).not.toContain('<img src=x')
    expect(showSuccess).toHaveBeenCalledWith('feedback.success')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('blocks image/file paste and keeps submission text-only', async () => {
    const { wrapper, submitter } = mountDialog()
    const textarea = wrapper.get('textarea').element
    const event = new Event('paste', { bubbles: true, cancelable: true })
    Object.defineProperty(event, 'clipboardData', {
      value: {
        items: [{ kind: 'file', type: 'image/png' }],
        files: [{ name: 'shot.png' }],
      },
    })

    textarea.dispatchEvent(event)
    await wrapper.vm.$nextTick()

    expect(event.defaultPrevented).toBe(true)
    expect(wrapper.text()).toContain('feedback.textOnly')
    expect(submitter).not.toHaveBeenCalled()
  })

  it('keeps cooldown across close/open and clears it when identity changes', async () => {
    vi.useFakeTimers()
    const { wrapper } = mountDialog()

    await wrapper.get('#feedback-title').setValue('First issue')
    await wrapper.get('textarea').setValue('first')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('feedback.cooldown')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await wrapper.get('#feedback-title').setValue('Second issue')
    await wrapper.get('textarea').setValue('second')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()

    await wrapper.setProps({ identityKey: 'user:2' })
    expect(wrapper.get<HTMLInputElement>('#feedback-title').element.value).toBe('')
    expect(wrapper.get('textarea').element.value).toBe('')
    await wrapper.get('#feedback-title').setValue('Other identity issue')
    await wrapper.get('textarea').setValue('second')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('uses backend retry_after on 429 and preserves the draft', async () => {
    vi.useFakeTimers()
    const submitter = vi.fn().mockRejectedValue({ status: 429, metadata: { retry_after: '12' } })
    const { wrapper } = mountDialog(submitter)

    await wrapper.get('#feedback-title').setValue('Missing balance')
    await wrapper.get('textarea').setValue('please help')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('textarea').element.value).toBe('please help')
    expect(wrapper.get<HTMLInputElement>('#feedback-title').element.value).toBe('Missing balance')
    expect(wrapper.text()).toContain('feedback.rateLimited')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
  })

  it('ignores a previous identity submission without clearing the next identity title and description', async () => {
    let resolve!: (result: { id: number; created_at: string; retry_after: number }) => void
    const pending = new Promise<{ id: number; created_at: string; retry_after: number }>((done) => { resolve = done })
    const submitter = vi.fn().mockReturnValue(pending)
    const { wrapper } = mountDialog(submitter)
    showSuccess.mockClear()
    await wrapper.get('#feedback-title').setValue('First identity issue')
    await wrapper.get('textarea').setValue('First identity description')
    await wrapper.get('form').trigger('submit')

    await wrapper.setProps({ identityKey: 'user:2' })
    expect(submitter.mock.calls[0][1].aborted).toBe(true)
    await wrapper.get('#feedback-title').setValue('Second identity issue')
    await wrapper.get('textarea').setValue('Second identity description')
    resolve({ id: 1, created_at: '2026-09-08T00:00:00Z', retry_after: 60 })
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('#feedback-title').element.value).toBe('Second identity issue')
    expect(wrapper.get('textarea').element.value).toBe('Second identity description')
    expect(wrapper.emitted('submitted')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(showSuccess).not.toHaveBeenCalled()
  })
})
