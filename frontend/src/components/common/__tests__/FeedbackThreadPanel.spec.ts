import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import FeedbackThreadPanel from '../FeedbackThreadPanel.vue'

enableAutoUnmount(afterEach)

const { showError, showSuccess, showWarning } = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess, showWarning }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key }) }
})

const openTicket = {
  id: 9,
  content: 'opening',
  source: 'user',
  status: 'open',
  created_at: '2026-09-08T00:00:00Z',
  updated_at: '2026-09-08T00:01:00Z',
  closed_at: null,
  closed_by: null,
  unread_count: 0,
}

const closedTicket = {
  ...openTicket,
  status: 'closed',
  updated_at: '2026-09-08T00:02:00Z',
  closed_at: '2026-09-08T00:02:00Z',
  closed_by: 'admin',
}

function makeAPI() {
  return {
    list: vi.fn().mockResolvedValue({ items: [openTicket], total: 1, page: 1, page_size: 20, pages: 1 }),
    get: vi.fn().mockResolvedValue(openTicket),
    listMessages: vi.fn().mockResolvedValue({
      items: [
        { id: 2, feedback_id: 9, author_role: 'admin', content: 'newer admin', created_at: '2026-09-08T00:02:00Z' },
        { id: 1, feedback_id: 9, author_role: 'user', content: 'older user', created_at: '2026-09-08T00:01:00Z' },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    }),
    reply: vi.fn().mockResolvedValue({
      message: { id: 3, feedback_id: 9, author_role: 'admin', content: 'sent', created_at: '2026-09-08T00:03:00Z' },
      retry_after: 0,
    }),
    close: vi.fn().mockResolvedValue(closedTicket),
    markRead: vi.fn().mockResolvedValue({ unread_count: 0, last_read_reply_id: 2 }),
  }
}

function mountPanel(api = makeAPI(), identityKey = 'user:1') {
  const wrapper = mount(FeedbackThreadPanel, {
    props: {
      identityKey,
      title: 'Tickets',
      api,
      showCreate: true,
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show" class="dialog-stub"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
        },
        Pagination: {
          props: ['page', 'total', 'pageSize'],
          emits: ['update:page'],
          template: '<button class="pagination-next" @click="$emit(\'update:page\', page + 1)">next</button>',
        },
        Icon: true,
      },
    },
  })
  return { wrapper, api }
}

describe('FeedbackThreadPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('loads personal tickets, opens detail, and renders replies chronologically within the current page', async () => {
    const { wrapper, api } = mountPanel()
    await flushPromises()

    expect(api.list).toHaveBeenCalledWith(1, 20, { status: undefined }, { signal: expect.any(AbortSignal) })
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    expect(api.get).toHaveBeenCalledWith(9, expect.any(AbortSignal))
    expect(api.listMessages).toHaveBeenCalledWith(9, 1, 20, expect.any(AbortSignal))
    const text = wrapper.text()
    expect(text.indexOf('older user')).toBeLessThan(text.indexOf('newer admin'))
  })

  it('shows an unread dot and clears it by acknowledging the displayed latest reply page', async () => {
    const api = makeAPI()
    api.list
      .mockResolvedValueOnce({ items: [{ ...openTicket, unread_count: 2 }], total: 1, page: 1, page_size: 20, pages: 1 })
      .mockResolvedValueOnce({ items: [{ ...openTicket, unread_count: 0 }], total: 1, page: 1, page_size: 20, pages: 1 })
    const { wrapper } = mountPanel(api)
    await flushPromises()

    const dot = wrapper.get('[data-testid="feedback-unread-dot"]')
    expect(dot.attributes('title')).toContain('feedback.unreadCount')

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    expect(api.markRead).toHaveBeenCalledWith(9, 2, expect.any(AbortSignal))
    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: undefined }, { signal: expect.any(AbortSignal) })
  })

  it('does not acknowledge messages when the visible page is an older message page', async () => {
    const { wrapper, api } = mountPanel()
    await flushPromises()
    api.listMessages.mockResolvedValue({ items: [], total: 21, page: 1, page_size: 20, pages: 2 })
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    api.markRead.mockClear()

    const olderButton = wrapper.findAll('button').find((button) => button.text().includes('feedback.olderMessages'))
    expect(olderButton).toBeTruthy()
    api.listMessages.mockResolvedValue({ items: [{ id: 1, feedback_id: 9, author_role: 'admin', content: 'old admin', created_at: '2026-09-08T00:01:00Z' }], total: 21, page: 2, page_size: 20, pages: 2 })
    await olderButton!.trigger('click')
    await flushPromises()

    expect(api.markRead).not.toHaveBeenCalled()
  })

  it('does not acknowledge displayed messages while the tab is hidden', async () => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' })
    const { wrapper, api } = mountPanel()
    await flushPromises()

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    expect(api.markRead).not.toHaveBeenCalled()
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
  })

  it('polls quietly every 15 seconds while mounted and visible', async () => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
    const { api } = mountPanel()
    await flushPromises()
    expect(api.list).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(15_000)
    await flushPromises()

    expect(api.list).toHaveBeenCalledTimes(2)
    vi.useRealTimers()
  })

  it('filters and paginates using the open/closed ticket contract', async () => {
    const { wrapper, api } = mountPanel()
    await flushPromises()

    await wrapper.get('[data-testid="feedback-filter-open"]').trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: 'open' }, { signal: expect.any(AbortSignal) })

    api.listMessages.mockResolvedValue({ items: [], total: 21, page: 1, page_size: 20, pages: 2 })
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    const olderButton = wrapper.findAll('button').find((button) => button.text().includes('feedback.olderMessages'))
    expect(olderButton).toBeTruthy()
    await olderButton!.trigger('click')
    await flushPromises()
    expect(api.listMessages).toHaveBeenLastCalledWith(9, 2, 20, expect.any(AbortSignal))
  })

  it('resets filters and first page after creating a new open ticket', async () => {
    const api = makeAPI()
    const createdTicket = { ...openTicket, id: 10, content: 'newly created' }
    const { wrapper } = mountPanel(api)
    await flushPromises()

    await wrapper.get('[data-testid="feedback-filter-closed"]').trigger('click')
    await flushPromises()
    api.list.mockResolvedValueOnce({ items: [createdTicket], total: 1, page: 1, page_size: 20, pages: 1 })
    api.get.mockResolvedValueOnce(createdTicket)
    api.listMessages.mockResolvedValueOnce({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })

    await (wrapper.vm as unknown as { refreshAfterCreate: (id?: number) => Promise<void> }).refreshAfterCreate(10)
    await flushPromises()

    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: undefined }, { signal: expect.any(AbortSignal) })
    expect(api.get).toHaveBeenLastCalledWith(10, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('newly created')
  })

  it('honors admin retry_after zero without starting a customer cooldown', async () => {
    const { wrapper, api } = mountPanel()
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    await wrapper.get('#feedback-reply-content').setValue('admin answer')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.reply).toHaveBeenCalledWith(9, 'admin answer', expect.any(AbortSignal))
    expect(showSuccess).toHaveBeenCalledWith('feedback.replySuccess')
    expect(wrapper.text()).not.toContain('feedback.cooldown')
  })

  it('refreshes a stale open ticket when reply is rejected as closed', async () => {
    const api = makeAPI()
    api.get.mockResolvedValueOnce(openTicket).mockResolvedValueOnce(closedTicket)
    api.reply.mockRejectedValue({ status: 409 })
    const { wrapper } = mountPanel(api)
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    await wrapper.get('#feedback-reply-content').setValue('late reply')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(showWarning).toHaveBeenCalledWith('feedback.closedStale')
    expect(wrapper.text()).toContain('feedback.closedReadOnly')
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
  })

  it('clears stale ticket state when the identity changes before a list response returns', async () => {
    let resolveFirst!: (value: unknown) => void
    const api = makeAPI()
    api.list.mockImplementationOnce(() => new Promise((resolve) => { resolveFirst = resolve }))
    const { wrapper } = mountPanel(api, 'key:a')

    await wrapper.setProps({ identityKey: 'key:b' })
    resolveFirst({ items: [{ ...openTicket, id: 99, content: 'stale key ticket' }], total: 1, page: 1, page_size: 20, pages: 1 })
    await flushPromises()

    expect(api.list).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('stale key ticket')
  })
})
