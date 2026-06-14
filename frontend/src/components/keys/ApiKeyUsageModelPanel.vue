<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-end gap-3">
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
      <button type="button" class="btn btn-secondary h-10" :disabled="loading" @click="loadModels">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        <span>{{ t('common.refresh') }}</span>
      </button>
    </div>

    <div v-if="error" class="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('keys.usageDetails.modelCount') }}</div>
        <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">{{ formatInteger(models.length) }}</div>
      </div>
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
    </div>

    <div class="grid gap-4 xl:grid-cols-[minmax(280px,360px)_1fr]">
      <div class="rounded-xl border border-stone-200/80 bg-white p-4 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="mb-3 flex items-center justify-between gap-3">
          <div class="text-sm font-medium text-stone-950 dark:text-white">
            {{ t('keys.usageDetails.modelsTab') }}
          </div>
          <div class="truncate text-xs text-stone-500 dark:text-stone-400" :title="responseTimezone">
            {{ responseTimezone }}
          </div>
        </div>
        <div v-if="loading" class="flex h-64 items-center justify-center text-sm text-stone-500 dark:text-stone-400">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="chartData" class="h-64">
          <Doughnut :data="chartData" :options="chartOptions" />
        </div>
        <div v-else class="flex h-64 items-center justify-center text-sm text-stone-500 dark:text-stone-400">
          {{ t('common.noData') }}
        </div>
      </div>

      <div class="overflow-hidden rounded-xl border border-stone-200/80 dark:border-white/10">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-stone-200/80 text-sm dark:divide-white/10">
            <thead class="bg-stone-50 dark:bg-white/[0.04]">
              <tr>
                <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('usage.model') }}</th>
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
              <tr v-else-if="models.length === 0">
                <td colspan="8" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">
                  {{ t('common.noData') }}
                </td>
              </tr>
              <template v-else>
                <tr v-for="model in models" :key="model.model" class="hover:bg-stone-50 dark:hover:bg-white/[0.04]">
                  <td class="max-w-[220px] truncate px-3 py-2 font-medium text-stone-950 dark:text-white" :title="model.model">
                    {{ model.model || '-' }}
                  </td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300">{{ formatInteger(model.requests) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(model.input_tokens)">{{ formatTokenNumber(model.input_tokens) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(model.output_tokens)">{{ formatTokenNumber(model.output_tokens) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(model.cache_creation_tokens)">{{ formatTokenNumber(model.cache_creation_tokens) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(model.cache_read_tokens)">{{ formatTokenNumber(model.cache_read_tokens) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(model.total_tokens)">{{ formatTokenNumber(model.total_tokens) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums font-medium text-stone-950 dark:text-white">{{ formatMoney(model.actual_cost) }}</td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, ArcElement, Tooltip, Legend } from 'chart.js'
import { Doughnut } from 'vue-chartjs'
import { usageAPI } from '@/api'
import type { UserModelStat } from '@/api/usage'
import Icon from '@/components/icons/Icon.vue'
import { formatCompactNumber } from '@/utils/format'

ChartJS.register(ArcElement, Tooltip, Legend)

const props = withDefaults(defineProps<{
  apiKeyId: number
  active?: boolean
}>(), {
  active: true
})

const { t } = useI18n()

const models = ref<UserModelStat[]>([])
const loading = ref(false)
const error = ref('')
const startDate = ref('')
const endDate = ref('')
const responseTimezone = ref(Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC')
let latestModelRequestID = 0

const palette = ['#10b981', '#38bdf8', '#f59e0b', '#a78bfa', '#f97316', '#14b8a6', '#e879f9', '#84cc16']

const totals = computed(() => models.value.reduce(
  (acc, model) => ({
    requests: acc.requests + model.requests,
    total_tokens: acc.total_tokens + model.total_tokens,
    actual_cost: acc.actual_cost + model.actual_cost
  }),
  { requests: 0, total_tokens: 0, actual_cost: 0 }
))

const chartData = computed(() => {
  if (!models.value.length || totals.value.total_tokens <= 0) return null
  return {
    labels: models.value.map((model) => model.model || '-'),
    datasets: [
      {
        data: models.value.map((model) => model.total_tokens),
        backgroundColor: models.value.map((_, index) => palette[index % palette.length]),
        borderColor: 'transparent',
        borderWidth: 0,
        hoverOffset: 4
      }
    ]
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  cutout: '64%',
  plugins: {
    legend: {
      position: 'bottom' as const,
      labels: {
        boxWidth: 10,
        boxHeight: 10,
        usePointStyle: true
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => `${context.label}: ${formatTokenNumber(Number(context.parsed ?? 0))}`
      }
    }
  }
}

const formatInteger = (value: number) => new Intl.NumberFormat().format(value)
const formatTokenNumber = (value: number) => formatCompactNumber(value)
const formatMoney = (value: number) => `$${value.toFixed(4)}`
const timezoneName = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

const formatDateInput = (date: Date) => {
  const pad = (value: number) => value.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

const applyDefaultRange = () => {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - 29)
  startDate.value = formatDateInput(start)
  endDate.value = formatDateInput(end)
}

const extractErrorMessage = (err: unknown) => {
  const maybeResponse = err as { response?: { data?: { message?: string; error?: string } } }
  return maybeResponse.response?.data?.message || maybeResponse.response?.data?.error || t('keys.usageDetails.modelsLoadFailed')
}

const loadModels = async () => {
  const requestID = ++latestModelRequestID
  loading.value = true
  error.value = ''
  try {
    const response = await usageAPI.getMyApiKeyModelStats(props.apiKeyId, {
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: timezoneName()
    })
    if (requestID !== latestModelRequestID) return
    models.value = response.models || []
    responseTimezone.value = response.timezone || timezoneName()
  } catch (err) {
    if (requestID !== latestModelRequestID) return
    error.value = extractErrorMessage(err)
    models.value = []
  } finally {
    if (requestID === latestModelRequestID) {
      loading.value = false
    }
  }
}

watch(() => props.apiKeyId, () => {
  applyDefaultRange()
  if (props.active) {
    void loadModels()
  }
})

watch(() => props.active, (active) => {
  if (active && models.value.length === 0 && !loading.value) {
    void loadModels()
  }
})

onMounted(() => {
  applyDefaultRange()
  if (props.active) {
    void loadModels()
  }
})
</script>
