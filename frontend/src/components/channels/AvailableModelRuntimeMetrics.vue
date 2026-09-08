<template>
  <section class="mt-3 min-w-0" data-testid="model-runtime-metrics" :aria-label="label('title')">
    <div
      role="img"
      data-testid="runtime-history"
      class="runtime-strip grid w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2.5 rounded-lg bg-stone-100/80 px-2.5 py-2 dark:bg-white/[0.045]"
      :aria-label="`${label('window')} · ${label('successRate')} ${formatRate(metrics.successRate)} · ${label(metrics.hours.some(hour => hour.requestVolume != null) ? 'volumeHint' : 'hourHint')}`"
    >
      <span class="inline-flex items-center gap-1 whitespace-nowrap text-[10px] text-stone-500 dark:text-stone-400">
        <Icon name="clock" size="xs" aria-hidden="true" />
        {{ label('window') }}
      </span>
      <span class="grid h-5 min-w-0 grid-cols-[repeat(24,minmax(0,1fr))] items-end gap-[2px]" aria-hidden="true">
        <span
          v-for="hour in metrics.hours"
          :key="hour.startedAt"
          data-testid="runtime-hour-bar"
          class="min-w-0 rounded-[1px]"
          :class="barColor(hour.successRate, hour.requestVolume)"
          :style="{ height: hour.requestVolume != null ? `${Math.max(18, hour.requestVolume / maxVolume * 100)}%` : barHeight(hour.successRate) }"
        />
      </span>
      <span class="text-right">
        <span class="block whitespace-nowrap font-mono text-[13px] font-semibold leading-4 tracking-tight" :class="rateColor(metrics.successRate)" data-testid="runtime-success-rate">
          {{ formatRate(metrics.successRate) }}
        </span>
        <span class="mt-0.5 block text-[9px] leading-3 text-stone-400 dark:text-stone-500">{{ label('successRate') }}</span>
      </span>
    </div>

    <div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-stone-200/80 pt-3 text-[11px] text-stone-500 dark:border-white/[0.08] dark:text-stone-400">
      <span v-if="throughput != null" class="inline-flex items-center gap-1"><Icon name="trendingUp" size="sm" class="text-emerald-600 dark:text-emerald-400" />{{ label('throughput') }} <strong class="font-mono font-medium text-stone-700 dark:text-stone-200" data-testid="runtime-throughput">{{ Number(throughput.toFixed(2)) }} t/s</strong></span>
      <span class="inline-flex items-center gap-1" :title="label('window')"><Icon name="clock" size="sm" class="text-emerald-600 dark:text-emerald-400" />{{ latencyLabel }} <strong class="font-mono font-medium text-stone-700 dark:text-stone-200" data-testid="runtime-average-latency">{{ formatLatency(metrics.averageLatencyMs) }}</strong></span>
      <span v-if="savings != null" class="ml-auto rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">{{ t('availableChannels.modelMarketplace.savings', { percent: savings }) }}</span>
      <span v-if="metrics.isPreview && throughput == null" class="text-[9px] text-stone-400 dark:text-stone-500">{{ label('preview') }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { ModelRuntimeMetrics } from './modelRuntimeMetrics'

const props = defineProps<{ metrics: ModelRuntimeMetrics; throughput?: number; savings?: number }>()
const { t } = useI18n()
const label = (key: string) => t(`availableChannels.modelMarketplace.runtime.${key}`)
const latencyLabel = computed(() => label(props.metrics.latencyKind))
const maxVolume = computed(() => Math.max(1, ...props.metrics.hours.map(hour => hour.requestVolume ?? 1)))
const formatRate = (value: number | null) => value === null ? '—' : `${(value * 100).toFixed(1)}%`
const formatLatency = (value: number | null) => value === null ? '—' : t('availableChannels.modelMarketplace.runtime.seconds', { value: (value / 1000).toFixed(2) })
const barHeight = (rate: number | null) => rate === null ? '18%' : `${Math.max(12, Math.min(100, rate * 100))}%`
const barColor = (rate: number | null, requestVolume = 0) => rate === null || requestVolume < 5
  ? 'bg-stone-300/70 dark:bg-stone-600/50'
  : rate >= 0.9 ? 'bg-emerald-500/75 dark:bg-emerald-400/75'
    : rate >= 0.7 ? 'bg-amber-400/90 dark:bg-amber-400/80' : 'bg-rose-400 dark:bg-rose-400/85'
const rateColor = (rate: number | null) => rate === null
  ? 'text-stone-400 dark:text-stone-500'
  // The API marks the 24-hour sample ready at 20 requests.
  : props.metrics.sampleState !== 'ready' ? 'text-stone-500 dark:text-stone-400'
    : rate >= 0.9 ? 'text-emerald-700 dark:text-emerald-300'
      : rate >= 0.7 ? 'text-amber-700 dark:text-amber-300' : 'text-rose-600 dark:text-rose-300'
</script>
