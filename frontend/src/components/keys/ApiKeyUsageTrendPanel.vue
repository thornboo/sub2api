<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-end gap-3">
      <Select
        :model-value="granularity"
        :options="granularityOptions"
        class="w-36"
        @update:model-value="handleGranularityChange"
      />
      <label class="w-40">
        <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
          {{ t('keys.usageDetails.startDate') }}
        </span>
        <input v-model="startDate" type="date" class="input h-10 w-full" />
      </label>
      <label class="w-40">
        <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
          {{ t('keys.usageDetails.endDate') }}
        </span>
        <input v-model="endDate" type="date" class="input h-10 w-full" />
      </label>
      <button
        type="button"
        class="btn btn-secondary h-10"
        :disabled="loading"
        @click="loadTrend"
      >
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        <span>{{ t('common.refresh') }}</span>
      </button>
    </div>

    <div v-if="error" class="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="rounded-xl border border-stone-200/80 bg-white p-3 dark:border-white/10 dark:bg-white/[0.03]">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <div class="text-sm font-medium text-stone-950 dark:text-white">
          {{ t('keys.usageDetails.trendChart') }}
        </div>
        <div class="inline-flex rounded-lg border border-stone-200/80 bg-stone-50 p-1 dark:border-white/10 dark:bg-black/20">
          <button
            v-for="option in chartMetricOptions"
            :key="option.value"
            type="button"
            :class="metricButtonClass(chartMetric === option.value)"
            @click="chartMetric = option.value"
          >
            {{ option.label }}
          </button>
        </div>
      </div>
      <div v-if="loading" class="flex h-56 items-center justify-center text-sm text-stone-500 dark:text-stone-400">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="chartData" class="h-56">
        <Line :data="chartData" :options="chartOptions" />
      </div>
      <div v-else class="flex h-56 items-center justify-center text-sm text-stone-500 dark:text-stone-400">
        {{ t('common.noData') }}
      </div>
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.totalRequests') }}</div>
        <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">{{ formatInteger(totals.requests) }}</div>
      </div>
      <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.totalTokens') }}</div>
        <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white" :title="formatInteger(totals.total_tokens)">{{ formatTokenNumber(totals.total_tokens) }}</div>
      </div>
      <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.actualCost') }}</div>
        <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">{{ formatMoney(totals.actual_cost) }}</div>
      </div>
      <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('keys.usageDetails.timezone') }}</div>
        <div class="mt-1 truncate text-sm font-medium text-stone-950 dark:text-white">{{ responseTimezone }}</div>
      </div>
    </div>

    <div class="overflow-hidden rounded-xl border border-stone-200/80 dark:border-white/10">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-stone-200/80 text-sm dark:divide-white/10">
          <thead class="bg-stone-50 dark:bg-white/[0.04]">
            <tr>
              <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('keys.usageDetails.bucket') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.totalRequests') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.in') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.out') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.cacheWrite') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.cacheRead') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.totalTokens') }}</th>
              <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.actualCost') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-stone-100 bg-white dark:divide-white/10 dark:bg-transparent">
            <tr v-if="loading">
              <td colspan="8" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">
                {{ t('common.loading') }}
              </td>
            </tr>
            <tr v-else-if="items.length === 0">
              <td colspan="8" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">
                {{ t('common.noData') }}
              </td>
            </tr>
            <template v-else>
              <tr v-for="item in items" :key="item.date" class="hover:bg-stone-50 dark:hover:bg-white/[0.04]">
                <td class="whitespace-nowrap px-3 py-2 font-medium text-stone-950 dark:text-white">
                  {{ formatBucketLabel(item.date) }}
                </td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300">{{ formatInteger(item.requests) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(item.input_tokens)">{{ formatTokenNumber(item.input_tokens) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(item.output_tokens)">{{ formatTokenNumber(item.output_tokens) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(item.cache_creation_tokens)">{{ formatTokenNumber(item.cache_creation_tokens) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(item.cache_read_tokens)">{{ formatTokenNumber(item.cache_read_tokens) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(item.total_tokens)">{{ formatTokenNumber(item.total_tokens) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums font-medium text-stone-950 dark:text-white">{{ formatMoney(item.actual_cost) }}</td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { usageAPI } from '@/api'
import type { ApiKeyUsageTrendGranularity } from '@/api/usage'
import type { TrendDataPoint } from '@/types'
import { formatCompactNumber } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

type TrendMetric = 'actual_cost' | 'total_tokens' | 'requests'

const props = withDefaults(defineProps<{
  apiKeyId: number
  active?: boolean
}>(), {
  active: true
})

const { t } = useI18n()

const granularity = ref<ApiKeyUsageTrendGranularity>('day')
const startDate = ref('')
const endDate = ref('')
const items = ref<TrendDataPoint[]>([])
const loading = ref(false)
const error = ref('')
const responseTimezone = ref(Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC')
const chartMetric = ref<TrendMetric>('actual_cost')
let latestTrendRequestID = 0

const granularityOptions = computed(() => [
  { value: 'hour', label: t('keys.usageDetails.granularity.hour') },
  { value: 'day', label: t('keys.usageDetails.granularity.day') },
  { value: 'week', label: t('keys.usageDetails.granularity.week') },
  { value: 'month', label: t('keys.usageDetails.granularity.month') }
])

const totals = computed(() => items.value.reduce(
  (acc, item) => ({
    requests: acc.requests + item.requests,
    total_tokens: acc.total_tokens + item.total_tokens,
    actual_cost: acc.actual_cost + item.actual_cost
  }),
  { requests: 0, total_tokens: 0, actual_cost: 0 }
))

const chartMetricOptions = computed<Array<{ value: TrendMetric; label: string }>>(() => [
  { value: 'actual_cost', label: t('keys.usageDetails.metricCost') },
  { value: 'total_tokens', label: t('keys.usageDetails.metricTokens') },
  { value: 'requests', label: t('keys.usageDetails.metricRequests') }
])

const formatInteger = (value: number) => new Intl.NumberFormat().format(value)
const formatTokenNumber = (value: number) => formatCompactNumber(value)
const formatMoney = (value: number) => `$${value.toFixed(4)}`
const timezoneName = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
const formatChartValue = (value: number) => {
  if (chartMetric.value === 'actual_cost') return formatMoney(value)
  if (chartMetric.value === 'total_tokens') return formatTokenNumber(value)
  return formatInteger(value)
}

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const chartTextColor = computed(() => isDarkMode.value ? '#d6d3d1' : '#57534e')
const chartGridColor = computed(() => isDarkMode.value ? 'rgba(255,255,255,0.10)' : 'rgba(120,113,108,0.18)')
const chartMetricColors: Record<TrendMetric, string> = {
  actual_cost: '#10b981',
  total_tokens: '#38bdf8',
  requests: '#a78bfa'
}
const chartMetricLabel = computed(() => chartMetricOptions.value.find((option) => option.value === chartMetric.value)?.label || '')
const metricButtonClass = (active: boolean) => [
  'h-8 rounded-md px-3 text-xs font-medium transition-colors',
  active
    ? 'bg-white text-stone-950 shadow-sm dark:bg-white/10 dark:text-white'
    : 'text-stone-500 hover:text-stone-950 dark:text-stone-400 dark:hover:text-white'
]
const chartValueForItem = (item: TrendDataPoint) => {
  if (chartMetric.value === 'actual_cost') return item.actual_cost
  if (chartMetric.value === 'total_tokens') return item.total_tokens
  return item.requests
}
const chartData = computed(() => {
  if (!items.value.length) return null
  const color = chartMetricColors[chartMetric.value]
  return {
    labels: items.value.map((item) => formatBucketLabel(item.date)),
    datasets: [
      {
        label: chartMetricLabel.value,
        data: items.value.map(chartValueForItem),
        borderColor: color,
        backgroundColor: `${color}26`,
        pointBackgroundColor: color,
        pointBorderColor: color,
        pointRadius: 2,
        pointHoverRadius: 4,
        borderWidth: 2,
        tension: 0.28,
        fill: true
      }
    ]
  }
})
const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: any) => `${chartMetricLabel.value}: ${formatChartValue(Number(context.parsed?.y ?? context.parsed ?? 0))}`
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: {
        color: chartTextColor.value,
        maxRotation: 0,
        autoSkip: true,
        maxTicksLimit: 8
      }
    },
    y: {
      beginAtZero: true,
      grid: { color: chartGridColor.value },
      ticks: {
        color: chartTextColor.value,
        callback: (value: string | number) => formatChartValue(Number(value))
      }
    }
  }
}))

const formatDateInput = (date: Date) => {
  const pad = (value: number) => value.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

const startOfWeek = (date: Date) => {
  const result = new Date(date)
  const weekday = result.getDay() || 7
  result.setDate(result.getDate() - weekday + 1)
  result.setHours(0, 0, 0, 0)
  return result
}

const applyDefaultRange = () => {
  const now = new Date()
  const start = new Date(now)
  const end = new Date(now)

  if (granularity.value === 'hour') {
    startDate.value = formatDateInput(now)
    endDate.value = formatDateInput(now)
    return
  }
  if (granularity.value === 'week') {
    const weekStart = startOfWeek(now)
    weekStart.setDate(weekStart.getDate() - 7 * 11)
    const weekEnd = startOfWeek(now)
    weekEnd.setDate(weekEnd.getDate() + 6)
    startDate.value = formatDateInput(weekStart)
    endDate.value = formatDateInput(weekEnd)
    return
  }
  if (granularity.value === 'month') {
    start.setDate(1)
    start.setMonth(start.getMonth() - 11)
    end.setMonth(end.getMonth() + 1, 0)
    startDate.value = formatDateInput(start)
    endDate.value = formatDateInput(end)
    return
  }

  start.setDate(start.getDate() - 29)
  startDate.value = formatDateInput(start)
  endDate.value = formatDateInput(end)
}

const extractErrorMessage = (err: unknown) => {
  const maybeResponse = err as { response?: { data?: { message?: string; error?: string } } }
  return maybeResponse.response?.data?.message || maybeResponse.response?.data?.error || t('keys.usageDetails.loadFailed')
}

const loadTrend = async () => {
  const requestID = ++latestTrendRequestID
  loading.value = true
  error.value = ''
  try {
    const response = await usageAPI.getMyApiKeyUsageTrend(props.apiKeyId, {
      granularity: granularity.value,
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: timezoneName()
    })
    if (requestID !== latestTrendRequestID) return
    items.value = response.items || []
    responseTimezone.value = response.timezone || timezoneName()
  } catch (err) {
    if (requestID !== latestTrendRequestID) return
    error.value = extractErrorMessage(err)
    items.value = []
  } finally {
    if (requestID === latestTrendRequestID) {
      loading.value = false
    }
  }
}

const handleGranularityChange = (value: string | number | boolean | null) => {
  if (value === 'hour' || value === 'day' || value === 'week' || value === 'month') {
    granularity.value = value
    applyDefaultRange()
    if (props.active) {
      void loadTrend()
    }
  }
}

type CalendarDate = { year: number; month: number; day: number }

const utcDateToCalendarDate = (date: Date): CalendarDate => ({
  year: date.getUTCFullYear(),
  month: date.getUTCMonth() + 1,
  day: date.getUTCDate()
})

const addCalendarDays = (date: CalendarDate, days: number): CalendarDate => {
  const next = new Date(Date.UTC(date.year, date.month - 1, date.day + days))
  return utcDateToCalendarDate(next)
}

const formatShortDate = (date: CalendarDate) => {
  const pad = (value: number) => value.toString().padStart(2, '0')
  return `${pad(date.month)}-${pad(date.day)}`
}

const formatISOWeekRange = (bucket: string) => {
  const match = /^(\d{4})-(\d{2})$/.exec(bucket)
  if (!match) return bucket

  const year = Number(match[1])
  const week = Number(match[2])
  const jan4 = new Date(Date.UTC(year, 0, 4))
  const jan4Weekday = jan4.getUTCDay() || 7
  const weekOneMonday = new Date(jan4)
  weekOneMonday.setUTCDate(jan4.getUTCDate() - jan4Weekday + 1)

  const monday = new Date(weekOneMonday)
  monday.setUTCDate(weekOneMonday.getUTCDate() + (week - 1) * 7)
  const mondayDate = utcDateToCalendarDate(monday)
  const sundayDate = addCalendarDays(mondayDate, 6)

  return `${bucket} (${formatShortDate(mondayDate)} ~ ${formatShortDate(sundayDate)})`
}

const formatBucketLabel = (bucket: string) => {
  if (granularity.value === 'week') {
    return formatISOWeekRange(bucket)
  }
  return bucket
}

watch(() => props.apiKeyId, () => {
  applyDefaultRange()
  if (props.active) {
    void loadTrend()
  }
})

watch(() => props.active, (active) => {
  if (active && items.value.length === 0 && !loading.value) {
    void loadTrend()
  }
})

onMounted(() => {
  applyDefaultRange()
  if (props.active) {
    void loadTrend()
  }
})
</script>
