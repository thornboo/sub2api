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

    await wrapper.get('textarea').setValue('<img src=x onerror=alert(1)>hello')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(submitter).toHaveBeenCalledWith('<img src=x onerror=alert(1)>hello', expect.any(AbortSignal))
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

    await wrapper.get('textarea').setValue('first')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('feedback.cooldown')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await wrapper.get('textarea').setValue('second')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()

    await wrapper.setProps({ identityKey: 'user:2' })
    await wrapper.get('textarea').setValue('second')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })

  it('uses backend retry_after on 429 and preserves the draft', async () => {
    vi.useFakeTimers()
    const submitter = vi.fn().mockRejectedValue({ status: 429, metadata: { retry_after: '12' } })
    const { wrapper } = mountDialog(submitter)

    await wrapper.get('textarea').setValue('please help')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(wrapper.get('textarea').element.value).toBe('please help')
    expect(wrapper.text()).toContain('feedback.rateLimited')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
  })
})
