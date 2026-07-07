<template>
  <div class="mt-4 border-t border-stone-200/70 pt-3 dark:border-white/10">
    <div
      class="flex justify-between text-[10px] font-semibold uppercase tracking-widest text-stone-400 dark:text-stone-500 mb-2"
    >
      <span>{{ t('monitorCommon.history60pts', { n: length }) }}</span>
      <span class="tabular-nums">{{ t('monitorCommon.nextUpdateIn', { n: countdownSeconds }) }}</span>
    </div>

    <div
      v-if="maintenance"
      class="flex h-5 w-full items-center justify-center rounded border border-dashed border-stone-300 text-[10px] uppercase tracking-widest text-stone-400 dark:border-white/10"
    >
      {{ t('monitorCommon.maintenancePaused') }}
    </div>
    <div v-else class="flex items-end gap-[2px] h-5 w-full">
      <div
        v-for="(bar, idx) in displayBars"
        :key="idx"
        class="timeline-bar relative flex h-full min-w-[3px] flex-1 items-end"
      >
        <div
          class="w-full rounded-sm"
          :class="bar.colorClass"
          :style="{ height: bar.heightPct + '%' }"
          :aria-label="bar.title || undefined"
        ></div>
        <div
          v-if="bar.title"
          role="tooltip"
          class="timeline-tooltip pointer-events-none absolute bottom-full z-30 mb-2 max-w-[min(72vw,280px)] whitespace-nowrap rounded-sm border border-stone-700 bg-stone-900 px-2 py-1 text-[11px] font-semibold leading-tight text-stone-50 opacity-0 shadow-lg transition-opacity duration-75 dark:border-stone-600 dark:bg-stone-800"
          :class="bar.tooltipClass"
        >
          {{ bar.title }}
        </div>
      </div>
    </div>

    <div
      class="mt-1 flex justify-between text-[9px] uppercase tracking-widest text-stone-400 dark:text-stone-500"
    >
      <span>{{ t('monitorCommon.past') }}</span>
      <span>{{ t('monitorCommon.now') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

type TimelineStatus = MonitorTimelinePoint['status'] | 'unknown'
interface TimelinePoint extends Omit<MonitorTimelinePoint, 'status'> {
  status: TimelineStatus
}

const props = withDefaults(defineProps<{
  buckets?: TimelinePoint[]
  countdownSeconds: number
  length?: number
  maintenance?: boolean
}>(), {
  buckets: () => [],
  length: 60,
  maintenance: false,
})

const { t } = useI18n()
const { statusLabel, formatLatency, formatRelativeTime } = useChannelMonitorFormat()

interface Bar {
  colorClass: string
  heightPct: number
  title: string
  tooltipClass: string
}

// 5 级高度 + 颜色双重编码：高=好+绿，短=坏+红，灰=未知/未测试。
// 长绿(正常) > 中黄(降级) > 短红(失败/系统错误) > 灰(未知) > 很短灰(未测试)。
const STATUS_HEIGHT: Record<string, number> = {
  operational: 100,
  degraded: 65,
  failed: 35,
  error: 35,
  unknown: 25,
  empty: 15,
}

const STATUS_COLOR: Record<string, string> = {
  operational: 'bg-emerald-500',
  degraded: 'bg-amber-500',
  failed: 'bg-red-500',
  error: 'bg-red-500',
  unknown: 'bg-stone-400 dark:bg-stone-500',
  empty: 'bg-stone-300 dark:bg-white/15',
}

const displayBars = computed<Bar[]>(() => {
  // Real points come newest-first; convert to oldest-first so the rightmost
  // bar represents "now". Pad the left with empty placeholders to keep the
  // bar count stable at `length`.
  const real = [...(props.buckets ?? [])]
    .slice(0, props.length)
    .reverse()

  const padCount = Math.max(0, props.length - real.length)
  const bars: Bar[] = []

  for (let i = 0; i < padCount; i += 1) {
    bars.push({
      colorClass: STATUS_COLOR.empty,
      heightPct: STATUS_HEIGHT.empty,
      title: '',
      tooltipClass: '',
    })
  }

  for (const [idx, point] of real.entries()) {
    const status = point.status as keyof typeof STATUS_HEIGHT
    const colorClass = STATUS_COLOR[status] ?? STATUS_COLOR.empty
    const heightPct = STATUS_HEIGHT[status] ?? STATUS_HEIGHT.empty
    const latency = formatLatency(point.latency_ms)
    const relative = formatRelativeTime(point.checked_at)
    const label = statusLabel(point.status)
    bars.push({
      colorClass,
      heightPct,
      title: `${relative} · ${label} · ${latency}ms`,
      tooltipClass: tooltipAlignClass(padCount + idx, props.length),
    })
  }

  return bars
})

function tooltipAlignClass(index: number, total: number): string {
  if (index <= 1) return 'left-0'
  if (index >= total - 2) return 'right-0'
  return 'left-1/2 -translate-x-1/2'
}
</script>

<style scoped>
.timeline-bar:hover > .timeline-tooltip {
  opacity: 1;
}
</style>
