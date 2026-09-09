import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import KeyUsageView from '../KeyUsageView.vue'
import { clearKeyAnnouncementSession } from '@/utils/keyAnnouncementSession'

enableAutoUnmount(afterEach)
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

const homeViewSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../HomeView.vue'), 'utf8')

const {
  createSession,
  deleteSession,
  getSession,
  getSummary,
  getRecordDetail,
  listAnnouncements,
  listFeedback,
  getFeedback,
  listFeedbackMessages,
  replyFeedback,
  closeFeedback,
  markFeedbackRead,
  listRecords,
  markAnnouncementRead,
  submitFeedback,
  showError,
  showInfo,
  showSuccess,
  showWarning,
} = vi.hoisted(() => ({
  createSession: vi.fn(),
  deleteSession: vi.fn(),
  getSession: vi.fn(),
  getSummary: vi.fn(),
  getRecordDetail: vi.fn(),
  listAnnouncements: vi.fn(),
  listFeedback: vi.fn(),
  getFeedback: vi.fn(),
  listFeedbackMessages: vi.fn(),
  replyFeedback: vi.fn(),
  closeFeedback: vi.fn(),
  markFeedbackRead: vi.fn(),
  listRecords: vi.fn(),
  markAnnouncementRead: vi.fn(),
  submitFeedback: vi.fn(),
  showError: vi.fn(),
  showInfo: vi.fn(),
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
}))

vi.mock('@/api/publicKeyUsage', () => ({
  publicKeyUsageAPI: {
    createSession,
    getSession,
    getSummary,
    listRecords,
    deleteSession,
    getRecordDetail,
    listAnnouncements,
    markAnnouncementRead,
    submitFeedback,
    listFeedback,
    getFeedback,
    listFeedbackMessages,
    replyFeedback,
    closeFeedback,
    markFeedbackRead,
    exportRecords: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { site_name: 'Sub2API' },
    siteName: 'Sub2API',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
    showError,
    showInfo,
    showSuccess,
    showWarning,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { site_name: 'Sub2API' },
    siteName: 'Sub2API',
    publicSettingsLoaded: true,
    fetchPublicSettings: vi.fn(),
    showError,
    showInfo,
    showSuccess,
    showWarning,
  }),
}))

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  BarElement: {},
  CategoryScale: {},
  Legend: {},
  LinearScale: {},
  Tooltip: {},
}))

vi.mock('vue-chartjs', async () => {
  const { defineComponent } = await import('vue')
  return {
    Bar: defineComponent({
      name: 'Bar',
      props: {
        data: { type: Object, required: true },
        options: { type: Object, required: true },
      },
      template: '<div class="bar-stub" />',
    }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const summary = {
  identity: {
    name: 'Zhang San Key',
    key_prefix: 'sk-test-••••',
    status: 'active',
    active: true,
    created_at: '2026-07-01T00:00:00Z',
    ip_access_mode: 'unrestricted',
    whitelist_size: 0,
    blacklist_size: 0,
    member: { code: 'zhang-san', name: '张三', status: 'active' },
  },
  key_budget: {
    quota: { limit: 100, used: 8, remaining: 92 },
    limit_5h: { limit: 0, used: 0, remaining: -1 },
    limit_1d: { limit: 0, used: 0, remaining: -1 },
    limit_7d: { limit: 0, used: 0, remaining: -1 },
  },
  member_budget: {
    period_start: '2026-07-01T00:00:00+08:00',
    period_end: '2026-08-01T00:00:00+08:00',
    timezone: 'Asia/Shanghai',
    monthly: { limit: 100, used: 8, remaining: 92 },
    settled_usd: 8,
    reserved_usd: 53.38,
    request_count: 3,
    input_tokens: 100,
    output_tokens: 50,
    limit_5h: { limit: 0, used: 0, remaining: -1 },
    limit_1d: { limit: 0, used: 0, remaining: -1 },
    limit_7d: { limit: 0, used: 0, remaining: -1 },
  },
  access_groups: [{
    name: 'OpenAI', platform: 'openai', status: 'active', sort_order: 1,
    rpm_limit: 30, models: ['gpt-5.6-sol', 'gpt-5.5'], model_count: 2,
  }],
  stats: {
    total_requests: 3,
    total_input_tokens: 100,
    total_output_tokens: 50,
    total_cache_creation_tokens: 0,
    total_cache_read_tokens: 0,
    total_tokens: 150,
    total_actual_cost: 0.08,
    average_duration_ms: 300,
  },
  trend: [{ date: '2026-07-19', requests: 3, input_tokens: 100, output_tokens: 50, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 150, actual_cost: 0.08 }],
  models: [{ model: 'gpt-5.6-sol', requests: 3, input_tokens: 100, output_tokens: 50, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 150, actual_cost: 0.08 }],
  start_date: '2026-06-20',
  end_date: '2026-07-19',
  timezone: 'Asia/Shanghai',
  error_records_available: true,
}

function mountView() {
  return mount(KeyUsageView, {
    global: {
      stubs: {
        RouterLink: { template: '<a><slot /></a>' },
        LocaleSwitcher: true,
        Icon: true,
        Select: { template: '<div class="select-stub"></div>' },
        Pagination: true,
        BaseDialog: {
          props: ['show', 'title'],
          template: '<section v-if="show" class="base-dialog-stub"><h2>{{ title }}</h2><slot /><footer><slot name="footer" /></footer></section>',
        },
      },
    },
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('KeyUsageView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    sessionStorage.clear()
    clearKeyAnnouncementSession()
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn().mockReturnValue({ matches: false }),
    })
    getSession.mockResolvedValue({ valid: false })
    createSession.mockResolvedValue({ valid: true })
    deleteSession.mockResolvedValue(undefined)
    getSummary.mockResolvedValue(summary)
    listRecords.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
    getRecordDetail.mockResolvedValue({
      id: 1,
      kind: 'success',
      created_at: '2026-07-19T00:00:00Z',
      model: 'gpt-5.6-sol',
      status_code: 200,
      stream: false,
    })
    listAnnouncements.mockResolvedValue([])
    listFeedback.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    getFeedback.mockResolvedValue({
      id: 9,
      content: 'key issue',
      source: 'key',
      status: 'open',
      created_at: '2026-09-08T00:00:00Z',
      updated_at: '2026-09-08T00:00:00Z',
      closed_at: null,
      closed_by: null,
      unread_count: 0,
    })
    listFeedbackMessages.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    replyFeedback.mockResolvedValue({ message: { id: 1, feedback_id: 9, author_role: 'user', content: 'reply', created_at: '2026-09-08T00:01:00Z' }, retry_after: 60 })
    closeFeedback.mockResolvedValue({
      id: 9,
      content: 'key issue',
      source: 'key',
      status: 'closed',
      created_at: '2026-09-08T00:00:00Z',
      updated_at: '2026-09-08T00:02:00Z',
      closed_at: '2026-09-08T00:02:00Z',
      closed_by: 'user',
      unread_count: 0,
    })
    markAnnouncementRead.mockResolvedValue({ message: 'ok' })
    markFeedbackRead.mockResolvedValue({ unread_count: 0, last_read_reply_id: 0 })
    submitFeedback.mockResolvedValue({ id: 17, created_at: '2026-09-08T00:00:00Z', retry_after: 60 })
    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: 'visible',
    })
  })

  it('uses the raw Key only for session creation and clears the input afterwards', async () => {
    const wrapper = mountView()
    await flushPromises()

    const input = wrapper.get('#key-usage-input')
    await input.setValue('sk-one-time-secret')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(createSession).toHaveBeenCalledTimes(1)
    expect(createSession).toHaveBeenCalledWith('sk-one-time-secret')
    expect(getSummary).toHaveBeenCalledTimes(1)
    expect(JSON.stringify(getSummary.mock.calls)).not.toContain('sk-one-time-secret')
    expect((wrapper.vm as unknown as { apiKey: string }).apiKey).toBe('')
    expect(wrapper.find('#key-usage-input').exists()).toBe(false)
    expect(showSuccess).toHaveBeenCalled()
  })

  it('maps an invalid Key response to localized copy instead of the backend English message', async () => {
    createSession.mockRejectedValue({
      response: { status: 401, data: { message: 'A valid API Key is required' } },
    })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#key-usage-input').setValue('sk-invalid')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('keyUsage.invalidKey')
    expect(JSON.stringify(showError.mock.calls)).not.toContain('A valid API Key is required')
  })

  it('restores an established session and shows every accessible model', async () => {
    getSession.mockResolvedValue({ valid: true })
    const wrapper = mountView()
    await flushPromises()

    expect(getSummary).toHaveBeenCalledTimes(1)
    expect(listAnnouncements).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('张三')
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('gpt-5.6-sol')
    expect(wrapper.text()).toContain('gpt-5.5')
    expect(wrapper.text()).toContain('keyUsage.exit')
    expect(wrapper.text()).not.toContain('$53.38')
  })

  it('submits key feedback through the isolated key usage API after a session exists', async () => {
    getSession.mockResolvedValue({ valid: true, session_id: 'session-a' })
    const createdTicket = {
      id: 17,
      content: 'key scoped issue',
      source: 'key',
      status: 'open',
      created_at: '2026-09-08T00:00:00Z',
      updated_at: '2026-09-08T00:00:00Z',
      closed_at: null,
      closed_by: null,
    }
    listFeedback
      .mockResolvedValueOnce({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
      .mockResolvedValueOnce({ items: [createdTicket], total: 1, page: 1, page_size: 20, pages: 1 })
      .mockResolvedValueOnce({ items: [createdTicket], total: 1, page: 1, page_size: 20, pages: 1 })
    getFeedback.mockResolvedValueOnce(createdTicket)
    const wrapper = mountView()
    await flushPromises()

    const feedbackButton = wrapper.findAll('button').find((button) => button.text().includes('feedback.entry'))
    expect(feedbackButton).toBeTruthy()
    await feedbackButton!.trigger('click')
    await flushPromises()
    expect(listFeedback).toHaveBeenCalledWith(1, 20, { status: undefined }, { signal: expect.any(AbortSignal) })
    const newButton = wrapper.findAll('button').find((button) => button.text().includes('feedback.newTicket'))
    expect(newButton).toBeTruthy()
    await newButton!.trigger('click')
    await flushPromises()

    await wrapper.get('#feedback-title').setValue('Key problem')
    await wrapper.get('#feedback-content').setValue('key scoped issue')
    await wrapper.get('#feedback-form').trigger('submit')
    await flushPromises()

    expect(submitFeedback).toHaveBeenCalledWith({ title: 'Key problem', content: 'key scoped issue' }, expect.any(AbortSignal))
    expect(JSON.stringify(getSummary.mock.calls)).not.toContain('key scoped issue')
    expect(showSuccess).toHaveBeenCalledWith('feedback.success')
    expect(wrapper.find('#feedback-content').exists()).toBe(false)
    expect(wrapper.find('.base-dialog-stub').exists()).toBe(false)

    await feedbackButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('key scoped issue')
    expect(listFeedback).toHaveBeenLastCalledWith(1, 20, { status: undefined }, { signal: expect.any(AbortSignal) })
    expect(getFeedback).toHaveBeenCalledWith(17, expect.any(AbortSignal))
  })

  it('clears key feedback state when exiting the query session', async () => {
    getSession.mockResolvedValue({ valid: true, session_id: 'session-a' })
    const wrapper = mountView()
    await flushPromises()

    const feedbackButton = wrapper.findAll('button').find((button) => button.text().includes('feedback.entry'))
    await feedbackButton!.trigger('click')
    await flushPromises()
    const newButton = wrapper.findAll('button').find((button) => button.text().includes('feedback.newTicket'))
    await newButton!.trigger('click')
    await flushPromises()
    await wrapper.get('#feedback-content').setValue('draft')

    const exitButton = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    await exitButton!.trigger('click')
    await flushPromises()

    expect(wrapper.find('#feedback-content').exists()).toBe(false)
  })

  it('renders an interactive spending trend with daily cost, request, and token details', async () => {
    getSession.mockResolvedValue({ valid: true })
    const wrapper = mountView()
    await flushPromises()

    const chart = wrapper.findComponent({ name: 'Bar' })
    expect(chart.exists()).toBe(true)
    expect(wrapper.text()).toContain('keyUsage.trendHint')
    expect(chart.element.parentElement?.getAttribute('role')).toBe('img')
    expect(chart.element.parentElement?.getAttribute('aria-label')).toBe('keyUsage.trendChartLabel')

    const data = chart.props('data') as {
      labels: string[]
      datasets: Array<{ data: number[], hoverBackgroundColor: string }>
    }
    expect(data.labels).toEqual(['2026-07-19'])
    expect(data.datasets[0]?.data).toEqual([0.08])
    expect(data.datasets[0]?.hoverBackgroundColor).toBeTruthy()

    const options = chart.props('options') as {
      interaction: { mode: string, intersect: boolean }
      plugins: {
        tooltip: {
          enabled: boolean
          callbacks: {
            title: (items: Array<{ label: string }>) => string
            label: (context: { parsed: { y: number } }) => string
            afterLabel: (context: { dataIndex: number }) => string[]
          }
        }
      }
    }
    expect(options.interaction).toEqual({ mode: 'index', intersect: false })
    expect(options.plugins.tooltip.enabled).toBe(true)
    expect(options.plugins.tooltip.callbacks.title([{ label: '2026-07-19' }])).toBe('2026-07-19')
    expect(options.plugins.tooltip.callbacks.label({ parsed: { y: 0.08 } })).toContain('keyUsage.cost:')
    expect(options.plugins.tooltip.callbacks.afterLabel({ dataIndex: 0 })).toEqual([
      'keyUsage.requests: 3',
      'keyUsage.totalTokens: 150',
    ])
  })

  it('keeps an authorized group visible but shows no models when its account pool is disabled', async () => {
    getSession.mockResolvedValue({ valid: true })
    getSummary.mockResolvedValueOnce({
      ...summary,
      access_groups: [{
        name: 'minimax', platform: 'openai', status: 'active', sort_order: 1,
        rpm_limit: 0, models: [], model_count: 0,
      }],
    })

    const wrapper = mountView()
    await flushPromises()

    const groupHeading = wrapper.findAll('h3').find((heading) => heading.text() === 'minimax')
    const groupCardText = groupHeading?.element.closest('.p-5')?.textContent ?? ''
    expect(groupHeading).toBeDefined()
    expect(groupCardText).toContain('keyUsage.noModels')
    expect(groupCardText).not.toContain('gpt-5.6-sol')
  })

  it('shows the actual member-budget overage without exposing internal reservations', async () => {
    getSession.mockResolvedValue({ valid: true })
    getSummary.mockResolvedValueOnce({
      ...summary,
      member_budget: {
        ...summary.member_budget,
        monthly: { limit: 100, used: 100.2, remaining: 0 },
        settled_usd: 100.2,
        reserved_usd: 80,
      },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('keyUsage.budgetOverage')
    expect(wrapper.text()).toContain('$0.20')
    expect(wrapper.text()).toContain('keyUsage.budgetRequestsStopped')
    expect(wrapper.text()).not.toContain('$80.00')
  })

  it('is linked from the public home navigation', () => {
    expect(homeViewSource).toContain('to="/key-usage"')
    expect(homeViewSource).toContain("t('home.keyQuery')")
  })

  it('revokes the short session from the top navigation and clears the dashboard', async () => {
    getSession.mockResolvedValue({ valid: true })
    const wrapper = mountView()
    await flushPromises()

    const exit = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    expect(exit).toBeDefined()
    await exit!.trigger('click')
    await flushPromises()

    expect(deleteSession).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('张三')
    expect(wrapper.find('#key-usage-input').exists()).toBe(true)
  })

  it('warns when the local page exits but server-side revocation fails', async () => {
    getSession.mockResolvedValue({ valid: true })
    deleteSession.mockRejectedValueOnce(new Error('redis unavailable'))
    const wrapper = mountView()
    await flushPromises()

    const exit = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    await exit!.trigger('click')
    await flushPromises()

    expect(wrapper.find('#key-usage-input').exists()).toBe(true)
    expect(showWarning).toHaveBeenCalledWith('keyUsage.exitRevokeFailed')
  })

  it('does not allow a new Key query until the previous session revocation finishes', async () => {
    const pendingDelete = deferred<void>()
    getSession.mockResolvedValue({ valid: true })
    deleteSession.mockReturnValueOnce(pendingDelete.promise)
    const wrapper = mountView()
    await flushPromises()

    const exit = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    await exit!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('keyUsage.exiting')
    expect(wrapper.find('#key-usage-input').exists()).toBe(false)

    pendingDelete.resolve()
    await flushPromises()

    expect(wrapper.find('#key-usage-input').exists()).toBe(true)
  })

  it('clears the raw Key before a slow summary request completes', async () => {
    const pendingSummary = deferred<typeof summary>()
    getSummary.mockReturnValueOnce(pendingSummary.promise)
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('#key-usage-input').setValue('sk-one-time-secret')
    await wrapper.get('form').trigger('submit')
    await flushPromises()

    expect(createSession).toHaveBeenCalledWith('sk-one-time-secret')
    expect((wrapper.vm as unknown as { apiKey: string }).apiKey).toBe('')
    expect(JSON.stringify(getSummary.mock.calls)).not.toContain('sk-one-time-secret')

    pendingSummary.resolve(summary)
    await flushPromises()
  })

  it('does not restore an old summary after the user exits while it is loading', async () => {
    const pendingSummary = deferred<typeof summary>()
    getSession.mockResolvedValue({ valid: true })
    getSummary.mockReturnValueOnce(pendingSummary.promise)
    const wrapper = mountView()
    await flushPromises()

    const exit = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    expect(exit).toBeDefined()
    await exit!.trigger('click')
    await flushPromises()

    pendingSummary.resolve(summary)
    await flushPromises()

    expect((wrapper.vm as unknown as { summary: unknown }).summary).toBeNull()
    expect(wrapper.text()).not.toContain('张三')
    expect(wrapper.find('#key-usage-input').exists()).toBe(true)
  })

  it('does not reopen an old record detail after the user exits', async () => {
    const record = {
      id: 7,
      kind: 'success' as const,
      created_at: '2026-07-19T00:00:00Z',
      model: 'gpt-5.6-sol',
      status_code: 200,
      stream: false,
    }
    const pendingDetail = deferred<typeof record>()
    getSession.mockResolvedValue({ valid: true })
    listRecords.mockResolvedValue({ items: [record], total: 1, page: 1, page_size: 20, pages: 1 })
    getRecordDetail.mockReturnValueOnce(pendingDetail.promise)
    const wrapper = mountView()
    await flushPromises()

    const detail = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.detail')
    expect(detail).toBeDefined()
    await detail!.trigger('click')
    const exit = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    await exit!.trigger('click')
    await flushPromises()

    pendingDetail.resolve(record)
    await flushPromises()

    expect((wrapper.vm as unknown as { selectedRecord: unknown }).selectedRecord).toBeNull()
    expect(wrapper.text()).not.toContain('keyUsage.recordDetail')
  })

  it('does not fetch key announcements before a validated or restored Key session exists', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSession).toHaveBeenCalledTimes(1)
    expect(getSummary).not.toHaveBeenCalled()
    expect(listAnnouncements).not.toHaveBeenCalled()
    expect(wrapper.find('#key-usage-input').exists()).toBe(true)
  })

  it('loads key announcements after session restoration even when usage summary fails', async () => {
    getSession.mockResolvedValue({ valid: true })
    getSummary.mockRejectedValueOnce(new Error('summary unavailable'))
    listAnnouncements.mockResolvedValueOnce([{
      id: 10,
      title: 'Session notice',
      content: 'Notice body',
      notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z',
      updated_at: '2026-07-24T07:30:00Z',
    }])

    const wrapper = mountView()
    await flushPromises()

    expect(listAnnouncements).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Session notice')
  })

  it('keeps local read acknowledgements when a slower list response returns unread data', async () => {
    getSession.mockResolvedValue({ valid: true })
    listAnnouncements.mockResolvedValue([{
      id: 18,
      title: 'Race notice',
      content: 'Race body',
      notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z',
      updated_at: '2026-07-24T07:30:00Z',
    }])

    const wrapper = mountView()
    await flushPromises()

    const markButton = wrapper.findAll('button').find((button) => button.text() === 'announcements.markRead')
    await markButton!.trigger('click')
    await flushPromises()

    await (wrapper.vm as unknown as { refreshKeyAnnouncements: (options: { force: boolean; silent: boolean }) => Promise<void> }).refreshKeyAnnouncements({ force: true, silent: true })

    const vm = wrapper.vm as unknown as { announcements: Array<{ id: number; read_at?: string }>; unreadAnnouncementCount: number }
    expect(vm.announcements.find((announcement) => announcement.id === 18)?.read_at).toBeTruthy()
    expect(vm.unreadAnnouncementCount).toBe(0)
  })

  it('auto-opens unread popup-mode key announcements and keeps silent announcements in the list', async () => {
    getSession.mockResolvedValue({ valid: true })
    listAnnouncements.mockResolvedValue([
      {
        id: 11,
        title: 'Popup notice',
        content: '## Popup body\n\n<script>window.__keyAnnouncementXss = true</script>',
        notify_mode: 'popup',
        created_at: '2026-07-24T07:30:00Z',
        updated_at: '2026-07-24T07:30:00Z',
      },
      {
        id: 12,
        title: 'Silent notice',
        content: 'Silent body',
        notify_mode: 'silent',
        created_at: '2026-07-24T07:31:00Z',
        updated_at: '2026-07-24T07:31:00Z',
      },
    ])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Popup notice')
    expect(wrapper.text()).toContain('Popup body')
    expect(wrapper.find('.markdown-body script').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Silent notice')

    await wrapper.find('button[aria-label="keyUsage.announcements"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('Silent notice')
    expect(wrapper.text()).toContain('keyUsage.unreadAnnouncements')
  })

  it('marks key announcements as read through the key-scoped API and updates unread state', async () => {
    getSession.mockResolvedValue({ valid: true })
    listAnnouncements.mockResolvedValue([{
      id: 21,
      title: 'Read me',
      content: 'Read body',
      notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z',
      updated_at: '2026-07-24T07:30:00Z',
    }])

    const wrapper = mountView()
    await flushPromises()

    const markButton = wrapper.findAll('button').find((button) => button.text() === 'announcements.markRead')
    expect(markButton).toBeDefined()
    await markButton!.trigger('click')
    await flushPromises()

    expect(markAnnouncementRead).toHaveBeenCalledWith(21, expect.any(AbortSignal))
    const vm = wrapper.vm as unknown as { announcements: Array<{ id: number; read_at?: string }>; unreadAnnouncementCount: number }
    expect(vm.announcements.find((announcement) => announcement.id === 21)?.read_at).toBeTruthy()
    expect(vm.unreadAnnouncementCount).toBe(0)
  })

  it('drops queued popup announcements when refresh shows they are no longer popup candidates', async () => {
    getSession.mockResolvedValue({ valid: true })
    listAnnouncements
      .mockResolvedValueOnce([
        {
          id: 31,
          title: 'First popup',
          content: 'First body',
          notify_mode: 'popup',
          created_at: '2026-07-24T07:30:00Z',
          updated_at: '2026-07-24T07:30:00Z',
        },
        {
          id: 32,
          title: 'Second popup',
          content: 'Second body',
          notify_mode: 'popup',
          created_at: '2026-07-24T07:31:00Z',
          updated_at: '2026-07-24T07:31:00Z',
        },
      ])
      .mockResolvedValueOnce([
        {
          id: 31,
          title: 'First popup',
          content: 'First body',
          notify_mode: 'popup',
          created_at: '2026-07-24T07:30:00Z',
          updated_at: '2026-07-24T07:30:00Z',
        },
        {
          id: 32,
          title: 'Second popup',
          content: 'Second body',
          notify_mode: 'silent',
          created_at: '2026-07-24T07:31:00Z',
          updated_at: '2026-07-24T07:31:00Z',
        },
      ])

    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('First popup')

    await wrapper.find('button[aria-label="keyUsage.announcements"]').trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find((button) => button.text() === 'keyUsage.refreshAnnouncements')!.trigger('click')
    await flushPromises()

    ;(wrapper.vm as unknown as { announcementListOpen: boolean }).announcementListOpen = false
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).not.toContain('Second body')
  })

  it('ignores late announcement read responses after the key identity changes', async () => {
    const pendingRead = deferred<{ message: string }>()
    getSession.mockResolvedValue({ valid: true })
    listAnnouncements.mockResolvedValue([{
      id: 41,
      title: 'Late read',
      content: 'Late body',
      notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z',
      updated_at: '2026-07-24T07:30:00Z',
    }])
    markAnnouncementRead.mockReturnValueOnce(pendingRead.promise)

    const wrapper = mountView()
    await flushPromises()

    const markButton = wrapper.findAll('button').find((button) => button.text() === 'announcements.markRead')
    await markButton!.trigger('click')
    const exit = wrapper.findAll('button').find((button) => button.text() === 'keyUsage.exit')
    await exit!.trigger('click')
    await flushPromises()

    pendingRead.resolve({ message: 'ok' })
    await flushPromises()

    expect((wrapper.vm as unknown as { announcements: unknown[] }).announcements).toEqual([])
    expect(wrapper.text()).not.toContain('Late read')
  })

  it('pauses hidden-tab automatic announcement refreshes and mutes background errors', async () => {
    vi.useFakeTimers()
    getSession.mockResolvedValue({ valid: true })
    listAnnouncements.mockResolvedValue([])
    const wrapper = mountView()
    await flushPromises()
    expect(listAnnouncements).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: 'hidden',
    })
    vi.advanceTimersByTime(20 * 60 * 1000)
    await flushPromises()
    expect(listAnnouncements).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'visibilityState', {
      configurable: true,
      value: 'visible',
    })
    listAnnouncements.mockRejectedValueOnce(new Error('temporary network issue'))
    vi.advanceTimersByTime(20 * 60 * 1000)
    await flushPromises()

    expect(listAnnouncements).toHaveBeenCalledTimes(2)
    expect(showError).not.toHaveBeenCalled()
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(listAnnouncements).toHaveBeenCalledTimes(3)
    wrapper.unmount()
    vi.useRealTimers()
  })

  it('keeps the global logged-in announcement popup isolated from the key-usage route', () => {
    const appSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../../App.vue'), 'utf8')

    expect(appSource).toContain("route.name === 'KeyUsage'")
    expect(appSource).toContain("matched.name === 'KeyUsage'")
    expect(appSource).toContain('<AnnouncementPopup v-if="!suppressUserAnnouncements" />')
    expect(appSource).toContain('authStore.isAuthenticated && !suppressUserAnnouncements.value')
  })

  it.each([false, true])('does not replay a dismissed unread popup after returning (storage denied: %s)', async (storageDenied) => {
    if (storageDenied) {
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Storage denied') })
    }
    getSession.mockResolvedValue({ valid: true, session_id: 'session-one' })
    listAnnouncements.mockResolvedValue([{
      id: 71, title: 'Dismiss once', content: 'Notice body', notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z', updated_at: '2026-07-24T07:30:00Z',
    }])
    const first = mountView()
    await flushPromises()
    expect(first.text()).toContain('Dismiss once')
    await (first.vm as unknown as { dismissAnnouncementPopup: (read: boolean) => Promise<void> }).dismissAnnouncementPopup(false)
    first.unmount()

    const returned = mountView()
    await flushPromises()
    expect(returned.text()).not.toContain('Notice body')
    expect(markAnnouncementRead).not.toHaveBeenCalled()
    await returned.get('button[aria-label="keyUsage.announcements"]').trigger('click')
    await flushPromises()
    expect(returned.text()).toContain('Dismiss once')
    expect((returned.vm as unknown as { unreadAnnouncementCount: number }).unreadAnnouncementCount).toBe(1)
  })

  it('does not reuse popup suppression when the server restores a different query session', async () => {
    getSession.mockResolvedValue({ valid: true, session_id: 'session-one' })
    listAnnouncements.mockResolvedValue([{
      id: 72, title: 'Session notice', content: 'New session body', notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z', updated_at: '2026-07-24T07:30:00Z',
    }])
    const first = mountView()
    await flushPromises()
    await (first.vm as unknown as { dismissAnnouncementPopup: (read: boolean) => Promise<void> }).dismissAnnouncementPopup(false)
    first.unmount()

    getSession.mockResolvedValue({ valid: true, session_id: 'session-two' })
    const returned = mountView()
    await flushPromises()
    expect(returned.text()).toContain('New session body')
  })

  it('retains popup suppression when a newly created query session is later restored', async () => {
    createSession.mockResolvedValue({ valid: true, session_id: 'created-session' })
    listAnnouncements.mockResolvedValue([{
      id: 74, title: 'Created session notice', content: 'Created body', notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z', updated_at: '2026-07-24T07:30:00Z',
    }])
    const first = mountView()
    await flushPromises()
    await first.get('#key-usage-input').setValue('sk-query-proof')
    await first.get('form').trigger('submit')
    await flushPromises()
    expect(first.text()).toContain('Created body')
    await (first.vm as unknown as { dismissAnnouncementPopup: (read: boolean) => Promise<void> }).dismissAnnouncementPopup(false)
    first.unmount()
    getSession.mockResolvedValue({ valid: true, session_id: 'created-session' })
    const returned = mountView()
    await flushPromises()
    expect(returned.text()).not.toContain('Created body')
    expect(sessionStorage.getItem('key-usage-announcement-session')).not.toContain('sk-query-proof')
  })

  it('retains the latest popup markers when storage becomes full after an earlier successful write', async () => {
    vi.useFakeTimers()
    getSession.mockResolvedValue({ valid: true, session_id: 'quota-session' })
    listAnnouncements.mockResolvedValue([75, 76].map((id) => ({
      id, title: `Notice ${id}`, content: `Body ${id}`, notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z', updated_at: '2026-07-24T07:30:00Z',
    })))
    const first = mountView()
    await flushPromises()
    expect(first.text()).toContain('Body 75')
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new Error('Quota exceeded') })
    await (first.vm as unknown as { dismissAnnouncementPopup: (read: boolean) => Promise<void> }).dismissAnnouncementPopup(false)
    vi.advanceTimersByTime(250)
    await flushPromises()
    expect(first.text()).toContain('Body 76')
    first.unmount()
    const returned = mountView()
    await flushPromises()
    expect(returned.text()).not.toContain('Body 75')
    expect(returned.text()).not.toContain('Body 76')
  })

  it.each(['exit', 'expired'])('forgets displayed announcements when the query session is %s', async (reason) => {
    getSession.mockResolvedValue({ valid: true, session_id: 'session-one' })
    listAnnouncements.mockResolvedValue([{
      id: 73, title: 'Clear on expiry', content: 'Body', notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z', updated_at: '2026-07-24T07:30:00Z',
    }])
    const first = mountView()
    await flushPromises()
    expect(sessionStorage.length).toBe(1)
    if (reason === 'exit') {
      await first.findAll('button').find((button) => button.text() === 'keyUsage.exit')!.trigger('click')
      await flushPromises()
      expect(sessionStorage.length).toBe(0)
      first.unmount()
    } else {
      first.unmount()
      getSession.mockResolvedValue({ valid: false })
      const expired = mountView()
      await flushPromises()
      expect(sessionStorage.length).toBe(0)
      expired.unmount()
    }
    getSession.mockResolvedValue({ valid: true, session_id: 'session-one' })
    const returned = mountView()
    await flushPromises()
    expect(returned.text()).toContain('Clear on expiry')
  })
})
