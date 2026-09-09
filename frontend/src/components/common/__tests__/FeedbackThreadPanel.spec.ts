import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { formatDateTimeToMinute } from '@/utils/format'
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

function mountPanel(
  api = makeAPI(),
  identityKey = 'user:1',
  props: Partial<InstanceType<typeof FeedbackThreadPanel>['$props']> = {},
) {
  const wrapper = mount(FeedbackThreadPanel, {
    props: {
      identityKey,
      title: 'Tickets',
      api,
      showCreate: true,
      ...props,
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

  it('shows reply progress independently of unread messages and gives closure precedence', async () => {
    const api = makeAPI()
    api.list.mockResolvedValue({ items: [
      { ...openTicket, title: 'Waiting for help', reply_status: 'pending', unread_count: 0 },
      { ...openTicket, id: 10, title: 'Answer received', reply_status: 'replied', unread_count: 2 },
      { ...closedTicket, id: 11, title: 'Finished thread', reply_status: 'pending' },
    ], total: 3, page: 1, page_size: 20, pages: 1 })
    const { wrapper } = mountPanel(api)
    await flushPromises()
    const rows = wrapper.findAll('[data-testid="feedback-ticket-row"]')
    expect(rows[0].text()).toContain('Waiting for help')
    expect(rows[0].text()).toContain('feedback.replyStatusLabels.pending')
    expect(rows[1].text()).toContain('feedback.replyStatusLabels.replied')
    expect(rows[2].text()).toContain('feedback.statusLabels.closed')
    expect(rows[2].text()).not.toContain('feedback.replyStatusLabels.pending')
  })

  it('requests reply-status filtering from the server and resets pagination', async () => {
    const { wrapper, api } = mountPanel()
    await flushPromises()
    await wrapper.get('[data-testid="feedback-filter-pending"]').trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: 'open', reply_status: 'pending' }, { signal: expect.any(AbortSignal) })
    await wrapper.get('[data-testid="feedback-filter-replied"]').trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: 'open', reply_status: 'replied' }, { signal: expect.any(AbortSignal) })
  })

  it.each([
    { admin: true, filter: 'pending', nextStatus: 'replied' },
    { admin: false, filter: 'replied', nextStatus: 'pending' },
  ])('returns to the last valid $filter page after replying and keeps the conversation open', async ({ admin, filter, nextStatus }) => {
    const api = makeAPI()
    let replied = false
    const selected = { ...openTicket, title: 'Last ticket on page two', reply_status: filter }
    const remaining = Array.from({ length: 20 }, (_, index) => ({
      ...openTicket, id: 100 + index, title: `Remaining ticket ${index}`, reply_status: filter,
    }))
    api.list.mockImplementation(async (page: number) => ({
      items: page === 1 ? remaining : replied ? [] : [selected],
      total: replied ? 20 : 21, page, page_size: 20, pages: replied ? 1 : 2,
    }))
    api.get.mockImplementation(async () => ({ ...selected, reply_status: replied ? nextStatus : filter }))
    api.reply.mockImplementation(async () => {
      replied = true
      return { message: { id: 3, feedback_id: 9, author_role: admin ? 'admin' : 'user', content: 'sent', created_at: openTicket.updated_at }, retry_after: 0 }
    })
    const { wrapper } = mountPanel(api, admin ? 'admin:1' : 'user:1', { admin })
    await flushPromises()
    await wrapper.get(`[data-testid="feedback-filter-${filter}"]`).trigger('click')
    await flushPromises()
    await wrapper.get('.pagination-next').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    await wrapper.get('#feedback-reply-content').setValue('sent')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: 'open', reply_status: filter }, { signal: expect.any(AbortSignal) })
    expect(wrapper.findAll('[data-testid="feedback-ticket-row"]')).toHaveLength(20)
    expect(wrapper.find('.pagination-next').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('feedback.emptyTickets')
    expect(wrapper.get('[data-testid="feedback-ticket-header"]').text()).toContain(selected.title)
    expect(wrapper.get('[data-testid="feedback-ticket-header"]').text()).toContain(admin ? `feedback.replyStatusLabels.${nextStatus}` : `feedback.userReplyStatusLabels.${nextStatus}`)
    expect(wrapper.get('#feedback-reply-content').element).toHaveProperty('value', '')
  })

  it('normalizes an emptied queue to page one so subsequent refreshes can show new tickets', async () => {
    const api = makeAPI()
    api.list.mockImplementation(async (page: number) => ({ items: [openTicket], total: 21, page, page_size: 20, pages: 2 }))
    const { wrapper } = mountPanel(api)
    await flushPromises()
    await wrapper.get('.pagination-next').trigger('click')
    await flushPromises()

    api.list.mockImplementation(async (page: number) => ({ items: [], total: 0, page, page_size: 20, pages: 0 }))
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(api.list.mock.calls.at(-1)?.[0]).toBe(1)
    expect(wrapper.text()).toContain('feedback.emptyTickets')
    expect(wrapper.find('.pagination-next').exists()).toBe(false)

    api.list.mockImplementation(async (page: number) => ({ items: page === 1 ? [openTicket] : [], total: 1, page, page_size: 20, pages: 1 }))
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(api.list.mock.calls.at(-1)?.[0]).toBe(1)
    expect(wrapper.findAll('[data-testid="feedback-ticket-row"]')).toHaveLength(1)
  })

  it('ignores a late page fallback after switching the status filter', async () => {
    const api = makeAPI()
    api.list.mockImplementation(async (page: number) => ({ items: [openTicket], total: 21, page, page_size: 20, pages: 2 }))
    const { wrapper } = mountPanel(api)
    await flushPromises()
    await wrapper.get('.pagination-next').trigger('click')
    await flushPromises()

    let resolveFallback!: (value: unknown) => void
    const fallback = new Promise((resolve) => { resolveFallback = resolve })
    api.list
      .mockResolvedValueOnce({ items: [], total: 20, page: 2, page_size: 20, pages: 1 })
      .mockImplementationOnce(() => fallback)
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(api.list.mock.calls.at(-1)?.[0]).toBe(1)
    const fallbackSignal = api.list.mock.calls.at(-1)?.[3].signal as AbortSignal

    api.list.mockResolvedValue({ items: [closedTicket], total: 1, page: 1, page_size: 20, pages: 1 })
    await wrapper.get('[data-testid="feedback-filter-closed"]').trigger('click')
    await flushPromises()
    expect(fallbackSignal.aborted).toBe(true)
    resolveFallback({ items: [{ ...openTicket, title: 'Stale fallback ticket' }], total: 20, page: 1, page_size: 20, pages: 1 })
    await flushPromises()

    expect(wrapper.findAll('[data-testid="feedback-ticket-row"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="feedback-ticket-row"]').text()).toContain('feedback.statusLabels.closed')
    expect(wrapper.text()).not.toContain('Stale fallback ticket')
    expect(showError).not.toHaveBeenCalled()
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

  it('renders user-viewer conversation bubbles chronologically with user messages outgoing', async () => {
    const api = makeAPI()
    api.listMessages.mockResolvedValue({
      items: [
        { id: 2, feedback_id: 9, author_role: 'admin', content: 'newer admin', created_at: '2026-09-08T00:02:00Z' },
        { id: 1, feedback_id: 9, author_role: 'user', content: '<b>older user</b>', created_at: '2026-09-08T00:01:00Z' },
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const { wrapper } = mountPanel(api)
    await flushPromises()

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    const bubbles = wrapper.findAll('[data-testid="feedback-message"]')
    expect(bubbles).toHaveLength(3)
    expect(bubbles[0].text()).toContain('feedback.openingMessage')
    expect(bubbles[0].text()).toContain('opening')
    expect(bubbles[1].text()).toContain('<b>older user</b>')
    expect(bubbles[1].html()).not.toContain('<b>older user</b>')
    expect(bubbles[2].text()).toContain('newer admin')
    expect(bubbles.map((bubble) => bubble.attributes('data-opening'))).toEqual(['true', 'false', 'false'])
    expect(bubbles.map((bubble) => bubble.attributes('data-side'))).toEqual(['outgoing', 'outgoing', 'incoming'])
    expect(bubbles[0].classes()).toContain('items-end')
    expect(bubbles[2].classes()).toContain('items-start')
  })

  it('renders admin-viewer conversation bubbles with administrator replies outgoing', async () => {
    const { wrapper } = mountPanel(makeAPI(), 'admin:1', { admin: true })
    await flushPromises()

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    const bubbles = wrapper.findAll('[data-testid="feedback-message"]')
    expect(bubbles).toHaveLength(3)
    expect(bubbles.map((bubble) => bubble.attributes('data-side'))).toEqual(['incoming', 'incoming', 'outgoing'])
    expect(bubbles[0].classes()).toContain('items-start')
    expect(bubbles[2].classes()).toContain('items-end')
  })

  it('shows ticket timestamps and tooltips to minute precision while preserving machine-readable dates', async () => {
    const { wrapper } = mountPanel()
    await flushPromises()

    const time = wrapper.get('[data-testid="feedback-ticket-row"] time')
    expect(time.attributes('datetime')).toBe('2026-09-08T00:01:00Z')
    expect(time.attributes('title')).toBe(formatDateTimeToMinute(openTicket.updated_at))

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()
    for (const timestamp of wrapper.findAll('time')) {
      expect(timestamp.text()).not.toMatch(/\d{1,2}:\d{2}:\d{2}/)
      expect(timestamp.attributes('title') || '').not.toMatch(/\d{1,2}:\d{2}:\d{2}/)
    }
    const header = wrapper.get('[data-testid="feedback-ticket-header"]')
    expect(header.text()).toContain('feedback.lastActivity')
    expect(header.text()).toContain(formatDateTimeToMinute(openTicket.updated_at))
    expect(header.find(`time[datetime="${openTicket.created_at}"]`).exists()).toBe(false)
    expect(wrapper.get('[data-opening="true"] time').attributes('datetime')).toBe(openTicket.created_at)
  })

  it('places closure actor and minute-precision time in the header and keeps the footer read-only', async () => {
    const api = makeAPI()
    api.list.mockResolvedValue({ items: [closedTicket], total: 1, page: 1, page_size: 20, pages: 1 })
    api.get.mockResolvedValue(closedTicket)
    const { wrapper } = mountPanel(api)
    await flushPromises()

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    const closedByParams = {
      actor: 'feedback.authorLabels.admin',
      time: formatDateTimeToMinute(closedTicket.closed_at),
    }
    const header = wrapper.get('[data-testid="feedback-ticket-header"]')
    const footer = wrapper.get('[data-testid="feedback-ticket-footer"]')
    expect(header.text()).toContain(`feedback.closedBy:${JSON.stringify(closedByParams)}`)
    expect(header.text()).not.toContain('feedback.lastActivity')
    expect(footer.text()).toContain('feedback.closedReadOnly')
    expect(footer.text()).not.toContain('feedback.closedBy')
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
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

  it('filters pending tickets and paginates their messages', async () => {
    const { wrapper, api } = mountPanel()
    await flushPromises()

    await wrapper.get('[data-testid="feedback-filter-pending"]').trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith(1, 20, { status: 'open', reply_status: 'pending' }, { signal: expect.any(AbortSignal) })

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
