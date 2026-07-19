import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import KeyUsageView from '../KeyUsageView.vue'

const homeViewSource = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../HomeView.vue'), 'utf8')

const {
  createSession,
  deleteSession,
  getSession,
  getSummary,
  getRecordDetail,
  listRecords,
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
  listRecords: vi.fn(),
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
    reserved_usd: 0,
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
    expect(wrapper.text()).toContain('张三')
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('gpt-5.6-sol')
    expect(wrapper.text()).toContain('gpt-5.5')
    expect(wrapper.text()).toContain('keyUsage.exit')
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
})
