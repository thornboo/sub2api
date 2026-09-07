import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelSelfCheckChainPanel from '../ModelSelfCheckChainPanel.vue'
import type { ModelSelfCheckChainView } from '@/api/modelStatus'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      const messages: Record<string, string> = {
        'channelStatus.selfCheckChain.title': '探测链路',
        'channelStatus.selfCheckChain.description': '按账号管理优先级展示当前候选探针顺序，并单独展示最近一次真实探测轮次；该链路用于健康探测，可能不同于实时请求路由。',
        'channelStatus.selfCheckChain.retry': '重试',
        'channelStatus.selfCheckChain.empty': '暂无探测链路数据',
        'channelStatus.selfCheckChain.currentCandidates': '当前候选',
        'channelStatus.selfCheckChain.roundStatus': '最近轮次',
        'channelStatus.selfCheckChain.roundTimeout': '轮次超时',
        'channelStatus.selfCheckChain.attemptTimeout': '单账号超时',
        'channelStatus.selfCheckChain.currentOrder': '当前候选顺序',
        'channelStatus.selfCheckChain.latestRound': '最近执行链路',
        'channelStatus.selfCheckChain.updatedAt': `配置视图更新：${String(params?.time ?? '')}`,
        'channelStatus.selfCheckChain.noCandidates': '当前没有可展示的候选账号。',
        'channelStatus.selfCheckChain.noRound': '暂无轮次',
        'channelStatus.selfCheckChain.noRoundDetail': '还没有持久化的真实探测轮次。',
        'channelStatus.selfCheckChain.noSteps': '该轮次没有步骤记录。',
        'channelStatus.selfCheckChain.noLatency': '无延迟',
        'channelStatus.selfCheckChain.neverChecked': '未检测',
        'channelStatus.selfCheckChain.eligible': '可参与本轮探测',
        'channelStatus.selfCheckChain.priority': `优先级 ${String(params?.priority ?? '')}`,
        'channelStatus.selfCheckChain.startedAt': `开始：${String(params?.time ?? '')}`,
        'channelStatus.selfCheckChain.finishedAt': `结束：${String(params?.time ?? '')}`,
        'channelStatus.selfCheckChain.duration': `耗时：${String(params?.ms ?? '')}ms`,
        'channelStatus.selfCheckChain.winner': `命中账号 #${String(params?.id ?? '')}`,
        'channelStatus.selfCheckChain.roundStatusLabel.checking': '检测中',
        'channelStatus.selfCheckChain.roundStatusLabel.operational': '可用',
        'channelStatus.selfCheckChain.roundStatusLabel.degraded': '降级可用',
        'channelStatus.selfCheckChain.roundStatusLabel.failed': '失败',
        'channelStatus.selfCheckChain.roundStatusLabel.unknown': '未知',
        'channelStatus.selfCheckChain.stepOutcome.pending': '等待中',
        'channelStatus.selfCheckChain.stepOutcome.skipped': '已跳过',
        'channelStatus.selfCheckChain.stepOutcome.succeeded': '成功',
        'channelStatus.selfCheckChain.stepOutcome.failed': '失败',
        'channelStatus.selfCheckChain.stepOutcome.not_attempted': '未探测',
        'channelStatus.selfCheckChain.reason.none': '无原因码',
        'channelStatus.selfCheckChain.reason.model_not_supported': '不支持该模型',
        'channelStatus.selfCheckChain.reason.no_eligible_account': '无符合条件账号',
        'channelStatus.selfCheckChain.reason.prior_success': '前序账号已成功',
        'channelStatus.selfCheckChain.reason.round_deadline': '轮次达到截止时间',
        'channelStatus.selfCheckChain.reason.fallback_succeeded': '备用账号探测成功',
        'channelStatus.selfCheckChain.reason.round_incomplete': '轮次未完成',
        'channelStatus.selfCheckChain.reason.temporarily_unschedulable': '账号临时不可调度',
      }
      return messages[key] ?? key
    },
  }),
}))

vi.mock('@/composables/useChannelMonitorFormat', () => ({
  useChannelMonitorFormat: () => ({
    statusLabel: (status: string) => `status:${status}`,
  }),
}))

function baseChain(overrides: Partial<ModelSelfCheckChainView> = {}): ModelSelfCheckChainView {
  return {
    group_id: 10,
    group_name: 'DeepSeek',
    model: 'deepseek-pro',
    updated_at: '2026-09-07T10:10:00Z',
    attempt_timeout_seconds: 30,
    round_timeout_seconds: 90,
    candidates: [
      {
        account_id: 12,
        account_name: 'current-c',
        priority: 2,
        platform: 'deepseek',
        order: 1,
        eligible: true,
        reason_code: 'eligible',
        last_checked_at: '2026-09-07T10:08:00Z',
        last_status: 'failed',
      },
      {
        account_id: 11,
        account_name: 'current-b',
        priority: 3,
        platform: 'deepseek',
        order: 2,
        eligible: true,
        reason_code: 'eligible',
        last_checked_at: '2026-09-07T10:08:05Z',
        last_status: 'operational',
      },
      {
        account_id: 10,
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
      id: 7,
      group_id: 10,
      model: 'deepseek-pro',
      status: 'degraded',
      reason_code: 'fallback_succeeded',
      winner_account_id: 11,
      started_at: '2026-09-07T10:00:00Z',
      finished_at: '2026-09-07T10:00:04Z',
      duration_ms: 4000,
      steps: [
        {
          account_id: 30,
          account_name: 'historical-old-priority',
          priority: 9,
          platform: 'deepseek',
          order: 1,
          outcome: 'failed',
          reason_code: 'round_deadline',
          started_at: '2026-09-07T10:00:00Z',
          finished_at: '2026-09-07T10:00:02Z',
          latency_ms: 2000,
          http_status: 504,
          error_code: 'deadline',
        },
        {
          account_id: 11,
          account_name: 'current-b',
          priority: 3,
          platform: 'deepseek',
          order: 2,
          outcome: 'succeeded',
          reason_code: 'fallback_succeeded',
          started_at: '2026-09-07T10:00:02Z',
          finished_at: '2026-09-07T10:00:04Z',
          latency_ms: 2000,
          http_status: 200,
          error_code: '',
        },
        {
          account_id: 12,
          account_name: 'current-c',
          priority: 2,
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
    ...overrides,
  }
}

describe('ModelSelfCheckChainPanel', () => {
  it('separates current priority order from the immutable latest round order', () => {
    const wrapper = mount(ModelSelfCheckChainPanel, {
      props: { chain: baseChain(), loading: false, error: '' },
    })
    const text = wrapper.text()

    expect(text).toMatch(/当前候选\s*2 \/ 3/)
    expect(text).toContain('current-c')
    expect(text).toContain('current-b')
    expect(text).toContain('historical-old-priority')
    expect(text.indexOf('current-c')).toBeLessThan(text.indexOf('current-b'))
    expect(text.indexOf('historical-old-priority')).toBeLessThan(text.indexOf('备用账号探测成功'))
    expect(text).toContain('前序账号已成功')
    expect(text).toContain('不支持该模型')
  })

  it('shows backups as not attempted while preserving their last known check state', () => {
    const wrapper = mount(ModelSelfCheckChainPanel, {
      props: { chain: baseChain(), loading: false, error: '' },
    })
    const text = wrapper.text()

    expect(text).toContain('未探测')
    expect(text).toContain('无延迟')
    expect(text).toContain('status:failed')
    expect(text).toContain('status:operational')
  })

  it('renders empty candidate and no-round states independently', () => {
    const wrapper = mount(ModelSelfCheckChainPanel, {
      props: {
        chain: baseChain({
          candidates: [],
          latest_round: null,
        }),
        loading: false,
        error: '',
      },
    })
    const text = wrapper.text()

    expect(text).toMatch(/当前候选\s*0 \/ 0/)
    expect(text).toContain('当前没有可展示的候选账号。')
    expect(text).toContain('暂无轮次')
    expect(text).toContain('还没有持久化的真实探测轮次。')
  })

  it('shows error retry and emits retry without rendering stale chain data', async () => {
    const wrapper = mount(ModelSelfCheckChainPanel, {
      props: { chain: baseChain(), loading: false, error: '加载失败' },
    })

    expect(wrapper.text()).toContain('加载失败')
    expect(wrapper.text()).not.toContain('current-c')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('renders running rounds with pending and incomplete evidence', () => {
    const wrapper = mount(ModelSelfCheckChainPanel, {
      props: {
        chain: baseChain({
          latest_round: {
            id: 8,
            group_id: 10,
            model: 'deepseek-pro',
            status: 'checking',
            reason_code: 'round_incomplete',
            winner_account_id: null,
            started_at: '2026-09-07T10:09:00Z',
            finished_at: null,
            duration_ms: null,
            steps: [
              {
                account_id: 12,
                account_name: 'current-c',
                priority: 2,
                platform: 'deepseek',
                order: 1,
                outcome: 'pending',
                reason_code: 'round_incomplete',
                started_at: '2026-09-07T10:09:00Z',
                finished_at: null,
                latency_ms: null,
                http_status: null,
                error_code: '',
              },
              {
                account_id: 11,
                account_name: 'current-b',
                priority: 3,
                platform: 'deepseek',
                order: 2,
                outcome: 'skipped',
                reason_code: 'temporarily_unschedulable',
                started_at: null,
                finished_at: null,
                latency_ms: null,
                http_status: null,
                error_code: '',
              },
            ],
          },
        }),
        loading: false,
        error: '',
      },
    })
    const text = wrapper.text()

    expect(text).toContain('检测中')
    expect(text).toContain('等待中')
    expect(text).toContain('轮次未完成')
    expect(text).toContain('已跳过')
    expect(text).toContain('账号临时不可调度')
  })
})
