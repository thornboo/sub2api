import { mount } from '@vue/test-utils'
import { createI18n, type MessageFunction } from 'vue-i18n'
import { baseCompile } from '@intlify/message-compiler'
import { afterEach, describe, expect, it } from 'vitest'
import AvailableModelRuntimeMetrics from '../AvailableModelRuntimeMetrics.vue'
import { buildModelRuntimeMetricsMap, createModelRuntimeMetricsPreview, type ModelRuntimeMetrics } from '../modelRuntimeMetrics'
import type { AvailableModelMarketplaceCard } from '@/utils/availableModelMarketplace'

const metrics: ModelRuntimeMetrics = {
  windowHours: 24,
  updatedAt: '2026-09-09T04:00:00Z',
  successRate: 0.966,
  averageLatencyMs: 1590,
  latencyKind: 'firstToken',
  sampleState: 'ready',
  isPreview: true,
  hours: Array.from({ length: 24 }, (_, i) => ({
    startedAt: new Date(Date.parse('2026-09-08T04:00:00Z') + i * 3_600_000).toISOString(),
    successRate: i === 0 ? null : i === 1 ? 0.6 : 1,
    averageLatencyMs: i === 0 ? null : 1590,
  })),
}

type Messages = { [key: string]: MessageFunction<string> | Messages }
function compileMessages(messages: Record<string, unknown>): Messages {
  return Object.fromEntries(Object.entries(messages).map(([key, value]) => [key,
    typeof value === 'string'
      ? new Function(`return ${baseCompile(value, { mode: 'arrow' }).code}`)() as MessageFunction<string>
      : compileMessages(value as Record<string, unknown>),
  ]))
}

function render(value = metrics) {
  const i18n = createI18n({
    legacy: false,
    locale: 'en',
    missingWarn: false,
    fallbackWarn: false,
    messages: { en: compileMessages({ availableChannels: { modelMarketplace: { runtime: {
      seconds: '{value}s', window: '24h', noRequests: 'No requests', lowSamples: 'Few samples',
      successRate: 'Success rate', firstToken: 'Avg. first token', generation: 'Avg. generation',
      preview: 'Sample data',
    } } }, common: { close: 'Close' } }) },
  })
  return mount(AvailableModelRuntimeMetrics, { props: { metrics: value }, attachTo: document.body, global: { plugins: [i18n] } })
}

afterEach(() => { document.body.innerHTML = '' })

describe('AvailableModelRuntimeMetrics', () => {
  it('shows a 24-hour summary and keeps hourly missing data visibly absent', () => {
    const wrapper = render()
    expect(wrapper.get('[data-testid="runtime-success-rate"]').text()).toBe('96.6%')
    expect(wrapper.get('[data-testid="runtime-average-latency"]').text()).toBe('1.59s')
    expect(wrapper.findAll('[data-testid="runtime-hour-bar"]')).toHaveLength(24)
    expect(wrapper.findAll('[data-testid="runtime-hour-bar"]')[0].classes()).toContain('bg-stone-300/70')
    expect(wrapper.text()).toContain('Sample data')
    wrapper.unmount()
  })

  it('keeps absent values without adding empty or low-sample notices', async () => {
    const wrapper = render({ ...metrics, isPreview: false, successRate: null, averageLatencyMs: null, sampleState: 'empty' })
    expect(wrapper.get('[data-testid="runtime-success-rate"]').text()).toBe('—')
    expect(wrapper.get('[data-testid="runtime-average-latency"]').text()).toBe('—')
    expect(wrapper.text()).not.toContain('No requests')
    expect(wrapper.find('[data-testid="runtime-throughput"]').exists()).toBe(false)
    await wrapper.setProps({ metrics: { ...metrics, isPreview: false, sampleState: 'low' } })
    expect(wrapper.get('[data-testid="runtime-success-rate"]').text()).toBe('96.6%')
    expect(wrapper.text()).not.toContain('Few samples')
    wrapper.unmount()
  })

  it.each([
    [72.10513797197416, '72.11 t/s'],
    [65.91333519236782, '65.91 t/s'],
    [51, '51 t/s'],
    [12.5, '12.5 t/s'],
    [0, '0 t/s'],
  ])('formats throughput %s with at most two decimal places', async (throughput, expected) => {
    const wrapper = render()
    await wrapper.setProps({ throughput })
    expect(wrapper.get('[data-testid="runtime-throughput"]').text()).toBe(expected)
    wrapper.unmount()
  })

  it('keeps runtime history display-only when clicked', async () => {
    const wrapper = render()
    const history = wrapper.get('[data-testid="runtime-history"]')
    expect(history.attributes('role')).toBe('img')
    expect(history.attributes('aria-label')).toContain('96.6%')
    expect(wrapper.find('button').exists()).toBe(false)
    await history.trigger('click')
    expect(document.querySelector('[data-testid="runtime-history-popover"]')).toBeNull()
    expect(wrapper.findAll('[data-testid="runtime-hour-bar"]')).toHaveLength(24)
    wrapper.unmount()
  })

  it('labels image generation duration separately from first-token latency', () => {
    const wrapper = render({ ...metrics, latencyKind: 'generation', averageLatencyMs: 18600 })
    expect(wrapper.text()).toContain('Avg. generation')
    expect(wrapper.get('[data-testid="runtime-average-latency"]').text()).toBe('18.60s')
    wrapper.unmount()
  })

  it('keeps fixtures stable across card filtering and scopes them by card identity', () => {
    const cards = ['1::claude-fable-5', '2::claude-fable-5'].map(id => ({ id, pricingOptions: [{ billing_mode: 'token' }] })) as AvailableModelMarketplaceCard[]
    const now = new Date('2026-09-09T04:32:00Z')
    const all = createModelRuntimeMetricsPreview(cards, now)
    const filtered = createModelRuntimeMetricsPreview([cards[1]], now)
    expect(filtered[cards[1].id]).toEqual(all[cards[1].id])
    expect(all[cards[0].id]).not.toEqual(all[cards[1].id])
    for (const item of Object.values(all)) {
      expect(item.windowHours).toBe(24)
      expect(item.hours).toHaveLength(24)
      expect(Date.parse(item.updatedAt) - Date.parse(item.hours[0].startedAt)).toBe(24 * 3_600_000)
    }
  })

  it('builds a runtime map only from cards with real metrics', () => {
    const cards = [
      { id: '1::claude-fable-5', runtimeMetrics: metrics },
      { id: '2::claude-fable-5' },
    ] as AvailableModelMarketplaceCard[]

    expect(buildModelRuntimeMetricsMap(cards)).toEqual({
      '1::claude-fable-5': metrics,
    })
    expect(buildModelRuntimeMetricsMap([{ id: '2::claude-fable-5' } as AvailableModelMarketplaceCard])).toBeUndefined()
  })
})
