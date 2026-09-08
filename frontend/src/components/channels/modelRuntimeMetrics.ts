import type { AvailableModelMarketplaceCard } from '@/utils/availableModelMarketplace'
import { BILLING_MODE_IMAGE, BILLING_MODE_TOKEN } from '@/constants/channel'

export interface ModelRuntimeHour {
  startedAt: string
  successRate: number | null
  averageLatencyMs: number | null
  requestVolume?: number
}

export interface ModelRuntimeMetrics {
  // Account scheduling configuration, independent of health and 24-hour metrics.
  hasSchedulableAccount?: boolean
  throughputTokensPerSecond?: number | null
  windowHours: 24
  updatedAt: string
  successRate: number | null
  averageLatencyMs: number | null
  latencyKind: 'firstToken' | 'generation' | 'response'
  sampleState: 'ready' | 'low' | 'empty'
  hours: ModelRuntimeHour[]
  isPreview?: boolean
}

export function buildModelRuntimeMetricsMap(
  cards: AvailableModelMarketplaceCard[],
): Record<string, ModelRuntimeMetrics> | undefined {
  const entries = cards
    .filter((card): card is AvailableModelMarketplaceCard & { runtimeMetrics: ModelRuntimeMetrics } => Boolean(card.runtimeMetrics))
    .map((card) => [card.id, card.runtimeMetrics] as const)
  return entries.length > 0 ? Object.fromEntries(entries) : undefined
}

// Development fixtures only. The caller is guarded by import.meta.env.DEV;
// no API, polling, probe, or production-data fallback belongs here.
export function createModelRuntimeMetricsPreview(
  cards: AvailableModelMarketplaceCard[],
  now = new Date(),
): Record<string, ModelRuntimeMetrics> {
  const hourMs = 3_600_000
  const end = Math.floor(now.getTime() / hourMs) * hourMs
  return Object.fromEntries(cards.map((card) => {
    const seed = Array.from(card.id).reduce((value, character) => (value * 31 + character.charCodeAt(0)) >>> 0, 7)
    const scenario = seed % 8
    const empty = scenario === 0
    const low = scenario === 1
    const billingMode = card.pricingOptions[0]?.billing_mode
    const latencyKind = billingMode === BILLING_MODE_IMAGE ? 'generation' : billingMode === BILLING_MODE_TOKEN ? 'firstToken' : 'response'
    const baseLatency = latencyKind === 'generation' ? 18_600 : 980 + seed % 1900
    let requests = 0
    let succeeded = 0
    let latencySum = 0
    const hours = Array.from({ length: 24 }, (_, index) => {
      const count = empty || (low && index < 22) ? 0 : low ? 2 : 70 + (seed + index * 37) % 170
      const incident = scenario === 2 && index >= 15 && index <= 18
      const failed = incident ? Math.round(count * (0.15 + (index % 3) * 0.13)) : low ? 0 : (seed + index) % 5 === 0 ? 1 : 0
      const success = count - failed
      const latency = baseLatency + (index * 137 + seed) % 640 + (incident ? 4200 : 0)
      requests += count
      succeeded += success
      latencySum += latency * success
      return {
        startedAt: new Date(end - (24 - index) * hourMs).toISOString(),
        successRate: count ? success / count : null,
        averageLatencyMs: success ? latency : null,
        requestVolume: count,
      }
    })
    return [card.id, {
      hasSchedulableAccount: seed % 4 !== 0,
      windowHours: 24,
      throughputTokensPerSecond: empty ? null : 43 + seed % 28,
      updatedAt: new Date(end).toISOString(),
      successRate: requests ? succeeded / requests : null,
      averageLatencyMs: succeeded ? latencySum / succeeded : null,
      latencyKind,
      sampleState: empty ? 'empty' : low ? 'low' : 'ready',
      hours,
      isPreview: true,
    } satisfies ModelRuntimeMetrics]
  }))
}
