import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { FeedbackReplyResult, FeedbackTicket } from '@/api/feedback'
import FeedbackThreadPanel from '../FeedbackThreadPanel.vue'

enableAutoUnmount(afterEach)

const { showError, showSuccess, showWarning } = vi.hoisted(() => ({
  showError: vi.fn(), showSuccess: vi.fn(), showWarning: vi.fn(),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess, showWarning }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const ticket: FeedbackTicket = {
  title: 'First issue', reply_status: 'pending',
  id: 9, content: 'first ticket opening', source: 'user', status: 'open',
  created_at: '2026-09-08T00:00:00Z', updated_at: '2026-09-08T00:01:00Z',
  closed_at: null, closed_by: null,
  unread_count: 0,
}
const nextTicket: FeedbackTicket = { ...ticket, id: 10, content: 'second ticket opening' }
const adminReply = { id: 1, feedback_id: 9, author_role: 'admin' as const, content: 'answer for first ticket only', created_at: '2026-09-08T00:01:00Z' }
const replied: FeedbackReplyResult = { message: { ...adminReply, author_role: 'user', content: 'sent text' }, retry_after: 60 }

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => { resolve = done })
  return { promise, resolve }
}

function makeAPI() {
  return {
    list: vi.fn().mockResolvedValue({ items: [ticket, nextTicket], total: 2, page: 1, page_size: 20, pages: 1 }),
    get: vi.fn().mockImplementation(async (id: number) => id === 9 ? ticket : nextTicket),
    listMessages: vi.fn().mockResolvedValue({ items: [adminReply], total: 1, page: 1, page_size: 20, pages: 1 }),
    reply: vi.fn().mockResolvedValue(replied),
    close: vi.fn().mockResolvedValue({ ...ticket, status: 'closed', closed_by: 'user', closed_at: '2026-09-08T00:03:00Z' }),
    markRead: vi.fn().mockResolvedValue({ unread_count: 0, last_read_reply_id: 1 }),
  }
}

function mountPanel(api = makeAPI()) {
  return mount(FeedbackThreadPanel, {
    props: { identityKey: 'user:1', title: 'Tickets', api },
    global: { stubs: {
      BaseDialog: { props: ['show'], template: '<section v-if="show" class="confirmation"><slot /><slot name="footer" /></section>' },
      Pagination: true,
      Icon: true,
    } },
  })
}

describe('Feedback conversation lifecycle', () => {
  beforeEach(() => { vi.clearAllMocks() })

  it('never shows the previous ticket replies while the next ticket loads', async () => {
    const api = makeAPI()
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain(adminReply.content)

    const pending = deferred<unknown>()
    api.listMessages.mockImplementationOnce(() => pending.promise)
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[1].trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain(adminReply.content)
    expect(wrapper.text()).toContain(nextTicket.content)
    pending.resolve({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    await flushPromises()
    expect(wrapper.text()).not.toContain(adminReply.content)
  })

  it('does not restore an old detail when a filter removes its ticket', async () => {
    const api = makeAPI()
    const pending = deferred<FeedbackTicket>()
    api.get.mockImplementationOnce(() => pending.promise)
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    api.list.mockResolvedValueOnce({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    await wrapper.get('[data-testid="feedback-filter-closed"]').trigger('click')
    await flushPromises()
    pending.resolve(ticket)
    await flushPromises()
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
    expect(wrapper.text()).not.toContain(ticket.content)
    expect(api.listMessages).not.toHaveBeenCalled()
  })

  it('ignores a reply completion after the conversation is unmounted', async () => {
    const api = makeAPI()
    const pending = deferred<FeedbackReplyResult>()
    api.reply.mockImplementationOnce(() => pending.promise)
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    await flushPromises()
    await wrapper.get('#feedback-reply-content').setValue('sent text')
    await wrapper.get('form').trigger('submit')
    wrapper.unmount()
    pending.resolve(replied)
    await flushPromises()
    expect(showSuccess).not.toHaveBeenCalled()
    expect(api.get).toHaveBeenCalledTimes(1)
    expect(api.list).toHaveBeenCalledTimes(2)
  })

  it('clears old drafts and ignores old replies after a Key identity changes', async () => {
    const api = makeAPI()
    const pending = deferred<FeedbackReplyResult>()
    api.reply.mockImplementationOnce(() => pending.promise)
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    await flushPromises()
    await wrapper.get('#feedback-reply-content').setValue('private old identity draft')
    await wrapper.get('form').trigger('submit')
    api.list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    await wrapper.setProps({ identityKey: 'key:new-query-identity' })
    pending.resolve(replied)
    await flushPromises()
    expect(wrapper.text()).not.toContain(adminReply.content)
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('keeps the composer closed after a closed-ticket rejection even when refreshing details fails', async () => {
    const api = makeAPI()
    api.get.mockResolvedValueOnce(ticket).mockRejectedValueOnce(new Error('temporary read failure'))
    api.reply.mockRejectedValue({ status: 409, reason: 'FEEDBACK_CLOSED' })
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    await flushPromises()
    await wrapper.get('#feedback-reply-content').setValue('message overtaken by closure')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('feedback.closedReadOnly')
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('keeps the just-closed conversation visible when it leaves the open-ticket filter', async () => {
    const api = makeAPI()
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.get('[data-testid="feedback-filter-pending"]').trigger('click')
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    await flushPromises()
    api.list.mockResolvedValueOnce({ items: [nextTicket], total: 1, page: 1, page_size: 20, pages: 1 })
    await wrapper.findAll('button').find((button) => button.text() === 'feedback.closeTicket')!.trigger('click')
    await wrapper.get('.confirmation').findAll('button').at(-1)!.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="feedback-ticket-row"]')).toHaveLength(1)
    expect(wrapper.text()).toContain(ticket.content)
    expect(wrapper.text()).toContain('feedback.closedReadOnly')
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
  })

  it.each(['messages', 'reply', 'close'])('propagates an expired session from %s to the parent', async (action) => {
    const api = makeAPI()
    if (action === 'messages') api.listMessages.mockRejectedValue({ response: { status: 401 } })
    if (action === 'reply') api.reply.mockRejectedValue({ response: { status: 401 } })
    if (action === 'close') api.close.mockRejectedValue({ response: { status: 401 } })
    const wrapper = mountPanel(api)
    await flushPromises()
    await wrapper.findAll('[data-testid="feedback-ticket-row"]')[0].trigger('click')
    await flushPromises()
    if (action === 'reply') {
      await wrapper.get('#feedback-reply-content').setValue('request needs a valid session')
      await wrapper.get('form').trigger('submit')
    }
    if (action === 'close') {
      await wrapper.findAll('button').find((button) => button.text() === 'feedback.closeTicket')!.trigger('click')
      await wrapper.get('.confirmation').findAll('button').at(-1)!.trigger('click')
    }
    await flushPromises()
    expect(wrapper.emitted('unauthorized')).toHaveLength(1)
    expect(showSuccess).not.toHaveBeenCalled()
  })
})
