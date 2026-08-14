<template>
  <BaseDialog
    :show="show && !!user"
    :title="t('admin.users.balanceConsumption.title')"
    width="extra-wide"
    close-on-click-outside
    @close="handleClose"
  >
    <div v-if="user" class="space-y-4">
      <div
        class="flex flex-col gap-3 rounded-xl border border-stone-200/80 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.03] sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="flex min-w-0 items-center gap-3">
          <div
            class="flex h-10 w-10 flex-none items-center justify-center rounded-full bg-emerald-100 text-sm font-semibold text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300"
          >
            {{ user.email.charAt(0).toUpperCase() }}
          </div>
          <div class="min-w-0">
            <p class="truncate font-medium text-stone-950 dark:text-white">{{ user.email }}</p>
            <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-stone-500 dark:text-stone-400">
              <span v-if="user.username">{{ user.username }}</span>
              <span
                class="inline-flex items-center gap-1 rounded-full bg-emerald-100/80 px-2 py-0.5 font-medium text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300"
              >
                <Icon name="dollar" size="xs" />
                {{ t('admin.users.balanceConsumption.walletScope') }}
              </span>
            </div>
          </div>
        </div>
        <p class="text-sm text-stone-500 dark:text-stone-400">
          {{ t('admin.users.balanceConsumption.dateRange', { start: startDate, end: endDate }) }}
        </p>
      </div>

      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div class="rounded-xl border border-stone-200/80 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.users.balanceConsumption.currentBalance') }}
            </span>
            <Icon name="dollar" size="sm" class="text-emerald-500" />
          </div>
          <p data-test="current-balance" class="mt-2 font-mono text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatCurrentBalance(user.balance) }}
          </p>
          <p v-if="(user.frozen_balance || 0) > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-300">
            {{ t('admin.users.balanceConsumption.frozenBalance', { amount: formatMoney(user.frozen_balance || 0) }) }}
          </p>
        </div>

        <div class="rounded-xl border border-stone-200/80 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.users.balanceConsumption.todayConsumption') }}
            </span>
            <Icon name="bolt" size="sm" class="text-amber-500" />
          </div>
          <p data-test="today-consumption" class="mt-2 font-mono text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatMoney(todayConsumption) }}
          </p>
          <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
            {{ t('admin.users.balanceConsumption.actualDeduction') }}
          </p>
        </div>

        <div class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-4 dark:border-emerald-500/25 dark:bg-emerald-500/10">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
              {{ t('admin.users.balanceConsumption.periodConsumption', { days: selectedRangeDays }) }}
            </span>
            <Icon name="chartBar" size="sm" class="text-emerald-500" />
          </div>
          <p data-test="period-consumption" class="mt-2 font-mono text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatMoney(periodConsumption) }}
          </p>
          <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
            {{ t('admin.users.balanceConsumption.requestCount', { count: periodRequests }) }}
          </p>
        </div>

        <div class="rounded-xl border border-stone-200/80 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.users.balanceConsumption.dailyAverage') }}
            </span>
            <Icon name="trendingUp" size="sm" class="text-sky-500" />
          </div>
          <p data-test="daily-average" class="mt-2 font-mono text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatMoney(dailyAverage) }}
          </p>
          <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
            {{ t('admin.users.balanceConsumption.calendarDayAverage') }}
          </p>
        </div>
      </div>

      <div class="rounded-xl border border-stone-200/80 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
        <div class="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p class="text-sm font-medium text-stone-800 dark:text-stone-200">
              {{ t('admin.users.balanceConsumption.chartTitle') }}
            </p>
            <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
              {{ t('admin.users.balanceConsumption.chartHint') }}
            </p>
          </div>
          <div class="flex items-center gap-2 self-start sm:self-auto">
            <div
              role="group"
              :aria-label="t('admin.users.balanceConsumption.rangeLabel')"
              class="inline-flex rounded-lg border border-stone-200 bg-stone-100/80 p-1 dark:border-white/10 dark:bg-black/20"
            >
              <button
                v-for="option in rangeOptions"
                :key="option"
                type="button"
                :data-test="`balance-range-${option}`"
                :aria-pressed="selectedRangeDays === option"
                :disabled="loading"
                class="h-8 min-w-12 rounded-md px-3 text-xs font-medium transition-colors disabled:cursor-wait disabled:opacity-60"
                :class="selectedRangeDays === option
                  ? 'bg-emerald-500 text-stone-950 shadow-sm shadow-emerald-950/10'
                  : 'text-stone-500 hover:bg-white hover:text-stone-900 dark:text-stone-400 dark:hover:bg-white/[0.07] dark:hover:text-white'"
                @click="selectRange(option)"
              >
                {{ t('admin.users.balanceConsumption.rangeDays', { days: option }) }}
              </button>
            </div>
            <button
              type="button"
              class="btn btn-secondary h-10 px-3"
              :aria-label="t('common.refresh')"
              :disabled="loading"
              @click="loadTrend"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            </button>
          </div>
        </div>

        <div v-if="error" class="mb-4 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
          {{ error }}
        </div>

        <div v-if="loading" class="flex h-72 items-center justify-center text-sm text-stone-500 dark:text-stone-400">
          {{ t('common.loading') }}...
        </div>
        <div
          v-else-if="!chartData"
          class="flex h-72 flex-col items-center justify-center gap-2 text-sm text-stone-500 dark:text-stone-400"
        >
          <Icon name="chartBar" size="xl" class="text-stone-300 dark:text-stone-600" />
          <span>{{ t('admin.users.balanceConsumption.noData') }}</span>
        </div>
        <div v-else class="h-72">
          <Bar :data="chartData" :options="chartOptions" />
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  BarElement,
  CategoryScale,
  Chart as ChartJS,
  LinearScale,
  Tooltip
} from 'chart.js'
import type { ChartData, ChartOptions, TooltipItem } from 'chart.js'
import { Bar } from 'vue-chartjs'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminUser, TrendDataPoint } from '@/types'

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip)

type RangeDays = 7 | 30 | 90

const props = defineProps<{
  show: boolean
  user: AdminUser | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const rangeOptions: RangeDays[] = [7, 30, 90]
const selectedRangeDays = ref<RangeDays>(30)
const startDate = ref('')
const endDate = ref('')
const loading = ref(false)
const error = ref('')
const trend = ref<TrendDataPoint[]>([])
let requestSeq = 0

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#d6d3d1' : '#57534e',
  grid: isDarkMode.value ? 'rgba(255,255,255,0.08)' : 'rgba(120,113,108,0.18)',
  bar: isDarkMode.value ? 'rgba(52, 211, 153, 0.72)' : 'rgba(16, 185, 129, 0.78)',
  barHover: isDarkMode.value ? '#6ee7b7' : '#059669'
}))

const dailyTrend = computed(() => fillDailyTrend(trend.value, startDate.value, endDate.value))
const periodConsumption = computed(() => dailyTrend.value.reduce((sum, point) => sum + point.actual_cost, 0))
const periodRequests = computed(() => dailyTrend.value.reduce((sum, point) => sum + point.requests, 0))
const todayConsumption = computed(() => dailyTrend.value.find((point) => point.date === endDate.value)?.actual_cost || 0)
const dailyAverage = computed(() => dailyTrend.value.length > 0 ? periodConsumption.value / dailyTrend.value.length : 0)
const hasConsumption = computed(() => dailyTrend.value.some((point) => point.actual_cost > 0))

const chartData = computed<ChartData<'bar'> | null>(() => {
  if (!hasConsumption.value) return null
  return {
    labels: dailyTrend.value.map((point) => point.date.slice(5)),
    datasets: [{
      label: t('admin.users.balanceConsumption.dailySpend'),
      data: dailyTrend.value.map((point) => point.actual_cost),
      backgroundColor: chartColors.value.bar,
      hoverBackgroundColor: chartColors.value.barHover,
      borderRadius: 6,
      borderSkipped: false,
      maxBarThickness: 32
    }]
  }
})

const chartOptions = computed<ChartOptions<'bar'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index'
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        title: (items) => {
          const index = items[0]?.dataIndex
          return index === undefined ? '' : dailyTrend.value[index]?.date || ''
        },
        label: (context: TooltipItem<'bar'>) => (
          `${t('admin.users.balanceConsumption.dailySpend')}: ${formatMoney(Number(context.parsed.y || 0))}`
        ),
        afterLabel: (context: TooltipItem<'bar'>) => {
          const requests = dailyTrend.value[context.dataIndex]?.requests || 0
          return t('admin.users.balanceConsumption.requestCount', { count: requests })
        }
      }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: {
        color: chartColors.value.text,
        font: { size: 11 },
        maxRotation: 0,
        autoSkip: true,
        maxTicksLimit: selectedRangeDays.value === 90 ? 12 : 15
      }
    },
    y: {
      beginAtZero: true,
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 11 },
        callback: (value) => formatAxisMoney(Number(value))
      }
    }
  }
}))

watch(
  () => [props.show, props.user?.id] as const,
  ([show]) => {
    if (show && props.user) {
      selectedRangeDays.value = 30
      applyRange(30)
      void loadTrend()
    } else {
      requestSeq += 1
      loading.value = false
      error.value = ''
      trend.value = []
    }
  },
  { immediate: true }
)

function selectRange(days: RangeDays) {
  if (days === selectedRangeDays.value || loading.value) return
  selectedRangeDays.value = days
  applyRange(days)
  void loadTrend()
}

function applyRange(days: RangeDays) {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - (days - 1))
  startDate.value = formatDateInput(start)
  endDate.value = formatDateInput(end)
}

async function loadTrend() {
  if (!props.show || !props.user) return
  const seq = ++requestSeq
  loading.value = true
  error.value = ''
  try {
    const response = await adminAPI.dashboard.getUsageTrend({
      user_id: props.user.id,
      billing_type: 0,
      granularity: 'day',
      start_date: startDate.value,
      end_date: endDate.value
    })
    if (seq !== requestSeq) return
    trend.value = response.trend || []
  } catch (err: any) {
    if (seq !== requestSeq) return
    error.value = err?.response?.data?.detail || err?.message || t('admin.users.balanceConsumption.loadFailed')
    trend.value = []
  } finally {
    if (seq === requestSeq) {
      loading.value = false
    }
  }
}

function handleClose() {
  emit('close')
}

function formatDateInput(date: Date) {
  const pad = (value: number) => value.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function fillDailyTrend(points: TrendDataPoint[], start: string, end: string): TrendDataPoint[] {
  if (!start || !end) return []
  const byDate = new Map(points.map((point) => [point.date.slice(0, 10), point]))
  const results: TrendDataPoint[] = []
  const cursor = new Date(`${start}T00:00:00Z`)
  const last = new Date(`${end}T00:00:00Z`)

  while (cursor <= last) {
    const date = cursor.toISOString().slice(0, 10)
    const point = byDate.get(date)
    results.push(point || {
      date,
      requests: 0,
      input_tokens: 0,
      output_tokens: 0,
      cache_creation_tokens: 0,
      cache_read_tokens: 0,
      total_tokens: 0,
      cost: 0,
      actual_cost: 0
    })
    cursor.setUTCDate(cursor.getUTCDate() + 1)
  }

  return results
}

function formatCurrentBalance(value: number) {
  return `$${Number(value || 0).toFixed(2)}`
}

function formatMoney(value: number) {
  return `$${Number(value || 0).toFixed(4)}`
}

function formatAxisMoney(value: number) {
  if (Math.abs(value) < 1) return `$${value.toFixed(4)}`
  return `$${value.toFixed(2)}`
}
</script>
