import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h, nextTick } from 'vue'
import { enableAutoUnmount, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ChannelStatusV1View from '../ChannelStatusV1View.vue'

enableAutoUnmount(afterEach)

const {
  authStoreHolder,
  listModelStatus,
  fetchModelStatusDetail,
  fetchSelfCheckTokenUsage,
  fetchModelSelfCheckChain,
  autoRefreshOnRefresh,
  showError,
  translate,
} = vi.hoisted(() => ({
  authStoreHolder: { store: null as null | { isAdmin: boolean } },
  listModelStatus: vi.fn(),
  fetchModelStatusDetail: vi.fn(),
  fetchSelfCheckTokenUsage: vi.fn(),
  fetchModelSelfCheckChain: vi.fn(),
  autoRefreshOnRefresh: { current: null as null | (() => unknown) },
  showError: vi.fn(),
  translate: (key: string, params?: Record<string, unknown>) => {
    const messages: Record<string, string> = {
      'channelStatus.title': '模型服务状态',
      'channelStatus.description': '查看站点对外模型的当前健康状态、可用率和近期延迟。',
      'channelStatus.windowTab.today': '今日',
      'channelStatus.windowTab.24h': '24 小时',
      'channelStatus.windowTab.7d': '7 天',
      'channelStatus.windowTab.30d': '30 天',
      'channelStatus.summary.overall': '总体状态',
      'channelStatus.summary.models': '监控模型',
      'channelStatus.summary.affected': '受影响模型',
      'channelStatus.summary.updated': '最后更新',
      'channelStatus.overall.operational': '全部正常',
      'channelStatus.groupPrefix': '分组：',
      'channelStatus.message.normal': '服务正常',
      'channelStatus.metrics.latency': '平均延迟',
      'channelStatus.metrics.avgLatency7d': '7 天平均延迟',
      'channelStatus.metrics.selfCheckTokens': '全局自检 Token',
      'channelStatus.metrics.selfCheckTokensScope': '按模型全局聚合，不按分组拆分',
      'channelStatus.metrics.inputTokens': '输入',
      'channelStatus.metrics.outputTokens': '输出',
      'channelStatus.metrics.lastChecked': '最后检测',
      'channelStatus.closeDetail': '关闭',
      'channelStatus.selfCheckChain.title': '探测链路',
      'channelStatus.selfCheckChain.description': '按账号管理优先级展示当前候选探针顺序，并单独展示最近一次真实探测轮次；该链路用于健康探测，可能不同于实时请求路由。',
      'channelStatus.selfCheckChain.currentCandidates': '当前候选',
      'channelStatus.selfCheckChain.roundStatus': '最近轮次',
      'channelStatus.selfCheckChain.roundTimeout': '轮次超时',
      'channelStatus.selfCheckChain.attemptTimeout': '单账号超时',
      'channelStatus.selfCheckChain.currentOrder': '当前候选顺序',
      'channelStatus.selfCheckChain.updatedAt': `配置视图更新：${String(params?.time ?? '')}`,
      'channelStatus.selfCheckChain.latestRound': '最近执行链路',
      'channelStatus.selfCheckChain.eligible': '可参与本轮探测',
      'channelStatus.selfCheckChain.priority': `优先级 ${String(params?.priority ?? '')}`,
      'channelStatus.selfCheckChain.neverChecked': '未检测',
      'channelStatus.selfCheckChain.noLatency': '无延迟',
      'channelStatus.selfCheckChain.startedAt': `开始：${String(params?.time ?? '')}`,
      'channelStatus.selfCheckChain.finishedAt': `结束：${String(params?.time ?? '')}`,
      'channelStatus.selfCheckChain.duration': `耗时：${String(params?.ms ?? '')}ms`,
      'channelStatus.selfCheckChain.winner': `命中账号 #${String(params?.id ?? '')}`,
      'channelStatus.selfCheckChain.roundStatusLabel.checking': '检测中',
      'channelStatus.selfCheckChain.roundStatusLabel.operational': '可用',
      'channelStatus.selfCheckChain.roundStatusLabel.degraded': '降级可用',
      'channelStatus.selfCheckChain.stepOutcome.failed': '失败',
      'channelStatus.selfCheckChain.stepOutcome.pending': '等待中',
      'channelStatus.selfCheckChain.stepOutcome.succeeded': '成功',
      'channelStatus.selfCheckChain.stepOutcome.not_attempted': '未探测',
      'channelStatus.selfCheckChain.reason.model_not_supported': '不支持该模型',
      'channelStatus.selfCheckChain.reason.probe_failed': '探测失败',
      'channelStatus.selfCheckChain.reason.prior_success': '前序账号已成功',
      'monitorCommon.status.operational': '正常',
      'monitorCommon.status.failed': '失败',
      'monitorCommon.status.unknown': '未知',
      'monitorCommon.availabilityPrefix': '可用率',
      'monitorCommon.relativeSecondsAgo': '刚刚',
      'monitorCommon.nextUpdateIn': '下次刷新',
      'monitorCommon.latencyEmpty': '-',
      'common.refresh': '刷新',
      'common.loading': '加载中',
    }
    return messages[key] ?? key
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: translate }),
  }
})

vi.mock('@/api/modelStatus', () => ({
  list: listModelStatus,
  detail: fetchModelStatusDetail,
  fetchSelfCheckTokenUsage,
  fetchModelSelfCheckChain,
}))

vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  authStoreHolder.store = reactive({ isAdmin: true })
  return {
    useAuthStore: () => authStoreHolder.store,
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: { channel_monitor_enabled: true },
    showError,
  }),
}))

vi.mock('@/composables/useAutoRefresh', async () => {
  const { ref } = await import('vue')
  return {
    useAutoRefresh: (options: { onRefresh: () => unknown }) => {
      autoRefreshOnRefresh.current = options.onRefresh
      return {
        enabled: ref(false),
        intervalSeconds: ref(60),
        countdown: ref(60),
        intervals: [30, 60, 120],
        setEnabled: vi.fn(),
        setInterval: vi.fn(),
        start: vi.fn(),
        stop: vi.fn(),
      }
    },
  }
})

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: defineComponent({ name: 'AppLayout', setup: (_, { slots }) => () => h('div', slots.default?.()) }),
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: defineComponent({
    name: 'BaseDialog',
    props: { show: Boolean, title: String },
    setup: (props, { slots }) => () => props.show
      ? h('div', { 'data-testid': 'dialog' }, [slots.default?.(), slots.footer?.()])
      : null,
  }),
}))

vi.mock('@/components/common/AutoRefreshButton.vue', () => ({
  default: defineComponent({ name: 'AutoRefreshButton', setup: () => () => h('button', { type: 'button' }, 'auto') }),
}))

vi.mock('@/components/common/EmptyState.vue', () => ({
  default: defineComponent({
    name: 'EmptyState',
    props: { title: String, description: String },
    setup: props => () => h('div', [props.title, props.description]),
  }),
}))

vi.mock('@/components/user/monitor/MonitorTimeline.vue', () => ({
  default: defineComponent({ name: 'MonitorTimeline', setup: () => () => h('div', { 'data-testid': 'timeline' }) }),
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: defineComponent({ name: 'Icon', setup: () => () => h('span') }),
}))

function statusRow(model: string, groupId = 10) {
  return {
    group_id: groupId,
    group_name: groupId === 10 ? 'DeepSeek' : 'Moonshot',
    model,
    display_name: model,
    status: 'operational',
    message_code: 'normal',
    latest_latency_ms: 300,
    avg_latency_24h_ms: 310,
    avg_latency_7d_ms: 320,
    availability_24h: 99.9,
    availability_7d: 99.7,
    availability_30d: 99.5,
    degraded_ratio_24h: 0,
    last_checked_at: '2026-09-07T10:00:00Z',
    timeline: [],
  }
}

function chainPayload(model = 'deepseek-pro') {
  return {
    group_id: 10,
    group_name: 'DeepSeek',
    model,
    updated_at: '2026-09-07T10:01:00Z',
    attempt_timeout_seconds: 30,
    round_timeout_seconds: 90,
    candidates: [
      {
        account_id: 3,
        account_name: 'account-c',
        priority: 2,
        platform: 'deepseek',
        order: 1,
        eligible: true,
        reason_code: 'eligible',
        last_checked_at: '2026-09-07T10:00:00Z',
        last_status: 'failed',
      },
      {
        account_id: 2,
        account_name: 'account-b',
        priority: 3,
        platform: 'deepseek',
        order: 2,
        eligible: true,
        reason_code: 'eligible',
        last_checked_at: '2026-09-07T10:00:10Z',
        last_status: 'operational',
      },
      {
        account_id: 1,
        account_name: 'flash-only',
        priority: 1,
        platform: 'deepseek',
        order: 0,
        eligible: false,
        reason_code: 'model_not_supported',
        last_checked_at: null,
        last_status: 'unknown',
      },
    ],
    latest_round: {
      id: 101,
      group_id: 10,
      model,
      status: 'degraded',
      reason_code: 'failover_success',
      winner_account_id: 2,
      started_at: '2026-09-07T10:00:00Z',
      finished_at: '2026-09-07T10:00:10Z',
      duration_ms: 10000,
      steps: [
        {
          account_id: 3,
          account_name: 'account-c',
          priority: 2,
          platform: 'deepseek',
          order: 1,
          outcome: 'failed',
          reason_code: 'probe_failed',
          started_at: '2026-09-07T10:00:00Z',
          finished_at: '2026-09-07T10:00:03Z',
          latency_ms: 3000,
          http_status: 500,
          error_code: 'upstream_error',
        },
        {
          account_id: 2,
          account_name: 'account-b',
          priority: 3,
          platform: 'deepseek',
          order: 2,
          outcome: 'succeeded',
          reason_code: '',
          started_at: '2026-09-07T10:00:03Z',
          finished_at: '2026-09-07T10:00:10Z',
          latency_ms: 7000,
          http_status: 200,
          error_code: '',
        },
        {
          account_id: 1,
          account_name: 'flash-only',
          priority: 1,
          platform: 'deepseek',
          order: 3,
          outcome: 'not_attempted',
          reason_code: 'prior_success',
          started_at: null,
          finished_at: null,
          latency_ms: null,
          http_status: null,
          error_code: '',
        },
      ],
    },
  }
}

function pendingChainPayload(model = 'deepseek-pro') {
  const payload = chainPayload(model)
  return {
    ...payload,
    updated_at: '2026-09-07T10:00:30Z',
    latest_round: payload.latest_round
      ? {
          ...payload.latest_round,
          status: 'checking',
          winner_account_id: null,
          finished_at: null,
          duration_ms: null,
          steps: [
            {
              ...payload.latest_round.steps[0],
              outcome: 'pending',
              reason_code: '',
              finished_at: null,
              latency_ms: null,
              http_status: null,
              error_code: '',
            },
          ],
        }
      : null,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, resolve, reject }
}

function mountView() {
  return mount(ChannelStatusV1View)
}

async function clickModelCard(wrapper: VueWrapper, model: string) {
  const card = wrapper.findAll('button').find(button => button.text().includes(model))
  expect(card, `card for ${model}`).toBeTruthy()
  await card!.trigger('click')
  await flushPromises()
  await nextTick()
}

describe('ChannelStatusV1View admin self-check chain', () => {
  beforeEach(() => {
    if (authStoreHolder.store) authStoreHolder.store.isAdmin = true
    listModelStatus.mockReset()
    fetchModelStatusDetail.mockReset()
    fetchSelfCheckTokenUsage.mockReset()
    fetchModelSelfCheckChain.mockReset()
    autoRefreshOnRefresh.current = null
    showError.mockReset()
    listModelStatus.mockResolvedValue({
      items: [statusRow('deepseek-pro', 10), statusRow('kimi-k2', 20)],
      updated_at: '2026-09-07T10:02:00Z',
    })
    fetchModelStatusDetail.mockImplementation((model: string, groupId: number) => Promise.resolve(statusRow(model, groupId)))
    fetchSelfCheckTokenUsage.mockResolvedValue({ window: 'today', items: [] })
    fetchModelSelfCheckChain.mockResolvedValue(chainPayload())
  })

  it('loads and renders the admin probe chain only inside the detail dialog', async () => {
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')

    expect(fetchModelSelfCheckChain).toHaveBeenCalledWith(10, 'deepseek-pro', {
      signal: expect.any(AbortSignal),
    })
    expect(wrapper.text()).toContain('探测链路')
    expect(wrapper.text()).toContain('account-c')
    expect(wrapper.text()).toContain('account-b')
    expect(wrapper.text()).toContain('flash-only')
    expect(wrapper.text()).toContain('前序账号已成功')
    expect(wrapper.text()).toContain('可能不同于实时请求路由')
  })

  it('does not request admin chain data for ordinary users', async () => {
    if (authStoreHolder.store) authStoreHolder.store.isAdmin = false
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')

    expect(fetchModelSelfCheckChain).not.toHaveBeenCalled()
    expect(wrapper.text()).not.toContain('探测链路')
  })

  it('aborts a stale admin chain request when switching model cards', async () => {
    const signals: AbortSignal[] = []
    fetchModelSelfCheckChain.mockImplementation((_groupId: number, _model: string, options?: { signal?: AbortSignal }) => {
      if (options?.signal) signals.push(options.signal)
      return new Promise(() => undefined)
    })
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')
    await clickModelCard(wrapper, 'kimi-k2')

    expect(signals).toHaveLength(2)
    expect(signals[0].aborted).toBe(true)
    expect(signals[1].aborted).toBe(false)
  })

  it('aborts the admin chain request and removes admin evidence when closing the dialog', async () => {
    const signals: AbortSignal[] = []
    fetchModelSelfCheckChain.mockImplementation((_groupId: number, _model: string, options?: { signal?: AbortSignal }) => {
      if (options?.signal) signals.push(options.signal)
      return new Promise(() => undefined)
    })
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')
    expect(signals).toHaveLength(1)
    await wrapper.findAll('button').find(button => button.text().includes('关闭'))!.trigger('click')
    await nextTick()

    expect(signals[0].aborted).toBe(true)
    expect(wrapper.find('[data-testid="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('探测链路')
  })

  it('cancels and clears the admin chain when the session loses admin privileges', async () => {
    const signals: AbortSignal[] = []
    fetchModelSelfCheckChain.mockImplementation((_groupId: number, _model: string, options?: { signal?: AbortSignal }) => {
      if (options?.signal) signals.push(options.signal)
      return new Promise(() => undefined)
    })
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')
    expect(wrapper.text()).toContain('探测链路')
    if (authStoreHolder.store) authStoreHolder.store.isAdmin = false
    await nextTick()

    expect(signals[0].aborted).toBe(true)
    expect(wrapper.text()).not.toContain('探测链路')
  })

  it('keeps the admin probe chain visible when the public detail refresh fails', async () => {
    fetchModelStatusDetail.mockRejectedValue(new Error('detail failed'))
    fetchModelSelfCheckChain.mockResolvedValue(chainPayload())
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')

    expect(showError).toHaveBeenCalledWith('detail failed')
    expect(wrapper.text()).toContain('deepseek-pro')
    expect(wrapper.text()).toContain('探测链路')
    expect(wrapper.text()).toContain('account-c')
  })

  it('refreshes an open admin chain on auto refresh without clearing the current content', async () => {
    const pending = pendingChainPayload()
    const finished = chainPayload()
    const chainRefresh = deferred<ReturnType<typeof chainPayload>>()
    fetchModelSelfCheckChain
      .mockResolvedValueOnce(pending)
      .mockReturnValueOnce(chainRefresh.promise)
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')
    expect(wrapper.text()).toContain('检测中')
    expect(wrapper.text()).toContain('等待中')

    const refreshPromise = autoRefreshOnRefresh.current?.()
    await flushPromises()
    await nextTick()

    expect(fetchModelSelfCheckChain).toHaveBeenCalledTimes(2)
    expect(fetchModelStatusDetail).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('检测中')
    expect(wrapper.text()).toContain('等待中')
    expect(wrapper.text()).not.toContain('加载中')

    chainRefresh.resolve(finished)
    await refreshPromise
    await flushPromises()
    await nextTick()

    expect(wrapper.text()).toContain('降级可用')
    expect(wrapper.text()).toContain('成功')
    expect(wrapper.text()).toContain('命中账号 #2')
  })

  it('does not request an admin chain on auto refresh after the dialog is closed or for ordinary users', async () => {
    const wrapper = mountView()
    await flushPromises()

    await clickModelCard(wrapper, 'deepseek-pro')
    expect(fetchModelSelfCheckChain).toHaveBeenCalledTimes(1)
    await wrapper.findAll('button').find(button => button.text().includes('关闭'))!.trigger('click')
    await autoRefreshOnRefresh.current?.()
    await flushPromises()

    expect(fetchModelSelfCheckChain).toHaveBeenCalledTimes(1)

    if (authStoreHolder.store) authStoreHolder.store.isAdmin = false
    await clickModelCard(wrapper, 'deepseek-pro')
    await autoRefreshOnRefresh.current?.()
    await flushPromises()

    expect(fetchModelSelfCheckChain).toHaveBeenCalledTimes(1)
  })

  it('does not start detail requests when an auto refresh settles after unmount', async () => {
    const wrapper = mountView()
    await flushPromises()
    await clickModelCard(wrapper, 'deepseek-pro')
    const pendingList = deferred<{ items: ReturnType<typeof statusRow>[]; updated_at: string }>()
    listModelStatus.mockReturnValueOnce(pendingList.promise)
    const refreshPromise = autoRefreshOnRefresh.current?.()
    wrapper.unmount()
    pendingList.resolve({ items: [], updated_at: '2026-09-07T10:03:00Z' })
    await refreshPromise
    await flushPromises()
    expect(fetchModelStatusDetail).toHaveBeenCalledTimes(1)
    expect(fetchModelSelfCheckChain).toHaveBeenCalledTimes(1)
  })
})
