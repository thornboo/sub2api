<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.upstreamCost.rechargeTrend.title')"
    width="extra-wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0">
          <p class="text-lg font-semibold text-stone-950 dark:text-white">{{ supplierName || '-' }}</p>
          <p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
            {{ t('admin.accounts.upstreamCost.rechargeTrend.description') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary h-10 px-3 text-sm"
          :disabled="loading || supplierId === null"
          @click="loadTrend"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </div>

      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div
          v-if="totals.length === 0"
          class="rounded-xl border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]"
        >
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.rechargeTrend.totalPaid') }}
            </span>
            <Icon name="creditCard" size="sm" class="text-emerald-500" />
          </div>
          <p class="mt-2 font-mono text-2xl font-semibold text-stone-400 dark:text-stone-500">-</p>
        </div>
        <template v-else>
          <div
            v-for="total in totals"
            :key="total.currency"
            class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-4 dark:border-emerald-500/25 dark:bg-emerald-500/10"
          >
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs font-medium text-emerald-700 dark:text-emerald-300">
                {{ t('admin.accounts.upstreamCost.rechargeTrend.totalPaid') }}
              </span>
              <Icon name="creditCard" size="sm" class="text-emerald-500" />
            </div>
            <p class="mt-2 font-mono text-2xl font-semibold text-stone-950 dark:text-white">
              {{ formatMoney(total.amount, total.currency) }}
            </p>
            <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.recordCountBadge', { count: total.record_count }) }}
            </p>
          </div>
        </template>
      </div>

      <div class="rounded-xl border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
        <div class="mb-3 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p class="text-sm font-medium text-stone-800 dark:text-stone-200">
              {{ t('admin.accounts.upstreamCost.rechargeTrend.chartTitle') }}
            </p>
            <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.rechargeTrend.chartHint', { granularity: selectedGranularityLabel }) }}
            </p>
          </div>
          <div class="flex items-center gap-3 self-start sm:self-auto">
            <div
              role="group"
              :aria-label="t('admin.accounts.upstreamCost.rechargeTrend.granularityLabel')"
              class="inline-flex rounded-lg border border-stone-200 bg-stone-100/80 p-1 dark:border-white/10 dark:bg-black/20"
            >
              <button
                v-for="option in granularityOptions"
                :key="option.value"
                type="button"
                :data-test="`trend-granularity-${option.value}`"
                :aria-pressed="selectedGranularity === option.value"
                :disabled="loading"
                class="h-8 min-w-11 rounded-md px-3 text-xs font-medium transition-colors disabled:cursor-wait disabled:opacity-60"
                :class="selectedGranularity === option.value
                  ? 'bg-emerald-500 text-stone-950 shadow-sm shadow-emerald-950/10'
                  : 'text-stone-500 hover:bg-white hover:text-stone-900 dark:text-stone-400 dark:hover:bg-white/[0.07] dark:hover:text-white'"
                @click="selectGranularity(option.value)"
              >
                {{ option.label }}
              </button>
            </div>
            <Icon name="trendingUp" size="md" class="text-emerald-500" />
          </div>
        </div>

        <div
          v-if="loading"
          class="flex h-72 items-center justify-center text-sm text-stone-500 dark:text-stone-400"
        >
          {{ t('common.loading') }}...
        </div>
        <div
          v-else-if="!chartData"
          class="flex h-72 flex-col items-center justify-center gap-2 text-sm text-stone-500 dark:text-stone-400"
        >
          <Icon name="chartBar" size="xl" class="text-stone-300 dark:text-stone-600" />
          <span>{{ t('admin.accounts.upstreamCost.rechargeTrend.empty') }}</span>
        </div>
        <div v-else class="h-72">
          <Line :data="chartData" :options="chartOptions" />
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
  CategoryScale,
  Chart as ChartJS,
  Filler,
  Legend,
  LinearScale,
  LineElement,
  PointElement,
  Tooltip
} from 'chart.js'
import type { ChartData, ChartOptions, TooltipItem } from 'chart.js'
import { Line } from 'vue-chartjs'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  UpstreamRechargeCurrencyTotal,
  UpstreamRechargeTrendGranularity,
  UpstreamSupplierRechargeTrend
} from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const props = defineProps<{
  show: boolean
  supplierId: number | null
  supplierName?: string | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const loading = ref(false)
const error = ref<string | null>(null)
const trend = ref<UpstreamSupplierRechargeTrend | null>(null)
const selectedGranularity = ref<UpstreamRechargeTrendGranularity>('month')
let requestSeq = 0

const palette = [
  { border: '#10b981', background: 'rgba(16, 185, 129, 0.14)' },
  { border: '#38bdf8', background: 'rgba(56, 189, 248, 0.12)' },
  { border: '#f59e0b', background: 'rgba(245, 158, 11, 0.12)' },
  { border: '#a78bfa', background: 'rgba(167, 139, 250, 0.12)' },
  { border: '#f43f5e', background: 'rgba(244, 63, 94, 0.12)' }
]

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const chartColors = computed(() => ({
  text: isDarkMode.value ? '#d6d3d1' : '#57534e',
  grid: isDarkMode.value ? 'rgba(255,255,255,0.08)' : 'rgba(120,113,108,0.18)'
}))

const totals = computed(() => sortCurrencyTotals(trend.value?.totals || []))
const granularityOptions = computed<Array<{ value: UpstreamRechargeTrendGranularity, label: string }>>(() => [
  { value: 'day', label: t('admin.accounts.upstreamCost.rechargeTrend.granularity.day') },
  { value: 'week', label: t('admin.accounts.upstreamCost.rechargeTrend.granularity.week') },
  { value: 'month', label: t('admin.accounts.upstreamCost.rechargeTrend.granularity.month') },
  { value: 'year', label: t('admin.accounts.upstreamCost.rechargeTrend.granularity.year') }
])
const selectedGranularityLabel = computed(() => (
  granularityOptions.value.find((option) => option.value === selectedGranularity.value)?.label || '-'
))

const chartData = computed<ChartData<'line'> | null>(() => {
  const points = trend.value?.points || []
  if (points.length === 0) return null

  const periods = [...new Set(points.map((point) => point.period))].sort()
  const currencies = [...new Set(points.map((point) => point.currency))].sort()
  if (periods.length === 0 || currencies.length === 0) return null

  const amountByKey = new Map(points.map((point) => [
    `${point.period}:${point.currency}`,
    Number(point.amount) || 0
  ]))

  return {
    labels: periods,
    datasets: currencies.map((currency, index) => {
      const color = palette[index % palette.length]
      return {
        label: currency,
        data: periods.map((period) => amountByKey.get(`${period}:${currency}`) || 0),
        borderColor: color.border,
        backgroundColor: color.background,
        borderWidth: 2,
        pointRadius: 3,
        pointHoverRadius: 5,
        tension: 0.3,
        fill: index === 0
      }
    })
  }
})

const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index'
  },
  plugins: {
    legend: {
      position: 'top',
      labels: {
        color: chartColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        boxWidth: 8,
        font: { size: 11 }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: TooltipItem<'line'>) => {
          const currency = context.dataset.label || ''
          return `${currency}: ${formatAmount(Number(context.parsed.y || 0))}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: { color: chartColors.value.grid },
      ticks: { color: chartColors.value.text, font: { size: 11 } }
    },
    y: {
      beginAtZero: true,
      grid: { color: chartColors.value.grid },
      ticks: {
        color: chartColors.value.text,
        font: { size: 11 },
        callback: (value) => formatAmount(Number(value))
      }
    }
  }
}))

watch(
  () => [props.show, props.supplierId] as const,
  ([show]) => {
    if (show) {
      selectedGranularity.value = 'month'
      loadTrend()
    } else {
      requestSeq += 1
      loading.value = false
      error.value = null
      trend.value = null
    }
  },
  { immediate: true }
)

async function loadTrend() {
  if (!props.show || props.supplierId === null) return
  const seq = ++requestSeq
  loading.value = true
  error.value = null
  try {
    const data = await adminAPI.accounts.getUpstreamSupplierRechargeTrend(
      props.supplierId,
      selectedGranularity.value
    )
    if (seq !== requestSeq) return
    trend.value = data
  } catch (err: any) {
    if (seq !== requestSeq) return
    error.value = err?.message || t('admin.accounts.upstreamCost.rechargeTrend.loadFailed')
    trend.value = null
  } finally {
    if (seq === requestSeq) {
      loading.value = false
    }
  }
}

function selectGranularity(granularity: UpstreamRechargeTrendGranularity) {
  if (granularity === selectedGranularity.value || loading.value) return
  selectedGranularity.value = granularity
  loadTrend()
}

function handleClose() {
  emit('close')
}

function sortCurrencyTotals(totalsToSort: UpstreamRechargeCurrencyTotal[]) {
  return [...totalsToSort]
    .filter((total) => Number(total.amount) !== 0 || Number(total.record_count) > 0)
    .sort((a, b) => a.currency.localeCompare(b.currency))
}

function formatAmount(value: number) {
  const amount = Number(value)
  if (!Number.isFinite(amount)) return '0'
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2
  }).format(amount)
}

function formatMoney(amount: number, currency: string) {
  return `${formatAmount(amount)} ${currency || '-'}`
}
</script>
