import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import FeedbackView from '../FeedbackView.vue'

enableAutoUnmount(afterEach)

const {
  listFeedback,
  getFeedback,
  listMessages,
  replyFeedback,
  closeFeedback,
  markRead,
  showError,
  showSuccess,
  showWarning,
} = vi.hoisted(() => ({
  listFeedback: vi.fn(),
  getFeedback: vi.fn(),
  listMessages: vi.fn(),
  replyFeedback: vi.fn(),
  closeFeedback: vi.fn(),
  markRead: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    feedback: {
      list: listFeedback,
      get: getFeedback,
      listMessages,
      reply: replyFeedback,
      close: closeFeedback,
      markRead,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning,
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 1, email: 'admin@example.com' },
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params ? `${key}:${JSON.stringify(params)}` : key }) }
})

const openTicket = {
  id: 9,
  content: '<img src=x onerror=alert(1)>plain\nnext',
  source: 'key',
  status: 'open',
  user_id: 3,
  user_email: '',
  api_key_id: 7,
  key_name: 'Prod Key',
  key_prefix: 'sk-prod-1234',
  member_id: 4,
  created_at: '2026-09-08T00:00:00Z',
  updated_at: '2026-09-08T00:00:00Z',
  closed_at: null,
  closed_by: null,
  unread_count: 0,
}

const adminReply = {
  id: 11,
  feedback_id: 9,
  author_role: 'admin',
  content: 'Admin reply',
  created_at: '2026-09-08T00:01:00Z',
}

function mountView() {
  return mount(FeedbackView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show" class="dialog-stub"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
        },
        Pagination: true,
        Icon: true,
      },
    },
  })
}

describe('FeedbackView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listFeedback.mockResolvedValue({ items: [openTicket], total: 1, page: 1, page_size: 20, pages: 1 })
    getFeedback.mockResolvedValue(openTicket)
    listMessages.mockResolvedValue({ items: [adminReply], total: 1, page: 1, page_size: 20, pages: 1 })
    replyFeedback.mockResolvedValue({
      message: { id: 12, feedback_id: 9, author_role: 'admin', content: 'follow up', created_at: '2026-09-08T00:02:00Z' },
      retry_after: 0,
    })
    closeFeedback.mockResolvedValue({
      ...openTicket,
      status: 'closed',
      updated_at: '2026-09-08T00:03:00Z',
      closed_at: '2026-09-08T00:03:00Z',
      closed_by: 'admin',
    })
    markRead.mockResolvedValue({ unread_count: 0, last_read_reply_id: 11 })
  })

  it('loads tickets and renders submitted content as escaped text', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listFeedback).toHaveBeenCalledWith(1, 20, { status: undefined }, { signal: expect.any(AbortSignal) })
    expect(wrapper.text()).toContain('Prod Key')
    expect(wrapper.text()).toContain('<img src=x onerror=alert(1)>plain')
    expect(wrapper.html()).not.toContain('<img src=x')
  })

  it('opens a ticket detail and shows administrator replies', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    expect(getFeedback).toHaveBeenCalledWith(9, expect.any(AbortSignal))
    expect(listMessages).toHaveBeenCalledWith(9, 1, 20, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('feedback.openingMessage')
    expect(wrapper.text()).toContain('Admin reply')
    expect(wrapper.text()).toContain('feedback.authorLabels.admin')
  })

  it('allows the administrator to reply and refreshes the thread', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    await wrapper.get('#feedback-reply-content').setValue('follow up')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(replyFeedback).toHaveBeenCalledWith(9, 'follow up', expect.any(AbortSignal))
    expect(showSuccess).toHaveBeenCalledWith('feedback.replySuccess')
    expect(getFeedback).toHaveBeenCalledTimes(2)
  })

  it('closes an open ticket and removes the composer', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="feedback-ticket-row"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="feedback-close-ticket"]').trigger('click')
    await wrapper.get('.dialog-stub button.btn-danger').trigger('click')
    await flushPromises()

    expect(closeFeedback).toHaveBeenCalledWith(9, expect.any(AbortSignal))
    expect(showSuccess).toHaveBeenCalledWith('feedback.closeSuccess')
    expect(wrapper.text()).toContain('feedback.closedReadOnly')
    expect(wrapper.find('#feedback-reply-content').exists()).toBe(false)
  })
})
