import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { FeedbackReply, FeedbackTicket } from '@/api/feedback'
import FeedbackThreadPanel from '../FeedbackThreadPanel.vue'

enableAutoUnmount(afterEach)

const { showError } = vi.hoisted(() => ({ showError: vi.fn() }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess: vi.fn(), showWarning: vi.fn() }),
}))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const ticket: FeedbackTicket = {
  id: 9, content: 'ticket opening', source: 'user', status: 'open',
  created_at: '2026-09-08T00:00:00Z', updated_at: '2026-09-08T00:01:00Z',
  closed_at: null, closed_by: null, unread_count: 1,
}
const reply: FeedbackReply = {
  id: 5, feedback_id: 9, author_role: 'admin', content: 'admin answer',
  created_at: '2026-09-08T00:01:00Z',
}
function page<T>(items: T[]) {
  return { items, total: items.length, page: 1, page_size: 20, pages: items.length ? 1 : 0 }
}
function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}
function makeAPI() {
  return {
    list: vi.fn().mockResolvedValue(page([ticket])),
    get: vi.fn().mockResolvedValue(ticket),
    listMessages: vi.fn().mockResolvedValue(page([reply])),
    reply: vi.fn(), close: vi.fn(),
    markRead: vi.fn().mockResolvedValue({ unread_count: 0, last_read_reply_id: 5 }),
  }
}
function mountPanel(api: ReturnType<typeof makeAPI>) {
  return mount(FeedbackThreadPanel, {
    props: { identityKey: 'user:1', title: 'Tickets', api },
    global: { stubs: { BaseDialog: true, Pagination: true, Icon: true } },
  })
}

describe('Feedback unread lifecycle boundaries', () => {
  let visibility: DocumentVisibilityState
  beforeEach(() => {
    vi.clearAllMocks()
    visibility = 'visible'
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(() => visibility)
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('keeps unread when a message request finishes after the page becomes hidden', async () => {
    const api = makeAPI()
    const pending = deferred<ReturnType<typeof page<FeedbackReply>>>()
    api.listMessages.mockReturnValueOnce(pending.promise)
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    visibility = 'hidden'
    document.dispatchEvent(new Event('visibilitychange'))
    pending.resolve(page([reply]))
    await flushPromises()
    expect(api.markRead).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="feedback-unread-dot"]').exists()).toBe(true)
  })

  it('preserves a newer unread reply when an older read receipt returns zero', async () => {
    const api = makeAPI()
    const pending = deferred<{ unread_count: number; last_read_reply_id: number }>()
    api.markRead.mockReturnValueOnce(pending.promise)
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain(reply.content)
    expect(api.markRead).toHaveBeenCalledWith(9, 5, expect.any(AbortSignal))

    // Reply 6 arrived after the displayed snapshot and remains unread in the list.
    api.list.mockResolvedValue(page([{ ...ticket, unread_count: 1 }]))
    pending.resolve({ unread_count: 0, last_read_reply_id: 5 })
    await flushPromises()
    expect(api.list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-testid="feedback-unread-dot"]').exists()).toBe(true)
    expect(api.markRead).toHaveBeenCalledTimes(1)
  })

  it('does not start detail requests after a pending poll is unmounted', async () => {
    const api = makeAPI()
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    const pending = deferred<ReturnType<typeof page<FeedbackTicket>>>()
    api.list.mockReturnValueOnce(pending.promise)
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(api.list).toHaveBeenCalledTimes(3)
    wrapper.unmount()
    pending.resolve(page([ticket]))
    await flushPromises()
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(api.listMessages).toHaveBeenCalledTimes(1)
    expect(showError).not.toHaveBeenCalled()
  })

  it('keeps background message failures quiet and preserves the reply draft', async () => {
    const api = makeAPI()
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    await wrapper.get('#feedback-reply-content').setValue('unfinished answer')
    api.listMessages.mockRejectedValueOnce(new Error('temporary connection failure'))
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(api.listMessages).toHaveBeenCalledTimes(2)
    expect(showError).not.toHaveBeenCalled()
    expect((wrapper.get('#feedback-reply-content').element as HTMLTextAreaElement).value).toBe('unfinished answer')
  })
})
