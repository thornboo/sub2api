<template>
  <BaseDialog
    :show="show && !!user"
    :title="dialogTitle"
    width="extra-wide"
    close-on-click-outside
    @close="emit('close')"
  >
    <div v-if="user" class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-stone-200/80 bg-stone-50 p-3 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2 text-sm text-stone-500 dark:text-stone-400">
          <span v-if="apiKey" :class="statusBadgeClass(apiKey.status)">
            {{ t('keys.status.' + apiKey.status) }}
          </span>
          <span v-else class="badge badge-gray">{{ t('admin.users.allApiKeysUsageScope') }}</span>
          <span class="truncate">{{ user.email }}</span>
          <span v-if="apiKey" class="truncate">{{ t('admin.users.apiKeyUsageScope') }}: {{ apiKey.name }}</span>
          <span v-if="apiKey?.group?.name">{{ t('keys.group') }}: {{ apiKey.group.name }}</span>
          <span>{{ startDate }} - {{ endDate }}</span>
          <span>{{ t('usage.actualCost') }}: <strong class="font-medium text-stone-950 dark:text-white">{{ formatMoney(stats?.total_actual_cost ?? 0) }}</strong></span>
        </div>
      </div>

      <div class="flex flex-wrap items-end gap-3">
        <label class="w-44">
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('keys.usageDetails.startDate') }}
          </span>
          <AppDatePicker
            v-model="startDateModel"
            :max="endDate"
            :placeholder="t('keys.usageDetails.startDate')"
          />
        </label>
        <label class="w-44">
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('keys.usageDetails.endDate') }}
          </span>
          <AppDatePicker
            v-model="endDateModel"
            :min="startDate"
            :placeholder="t('keys.usageDetails.endDate')"
          />
        </label>
        <button
          type="button"
          class="btn btn-secondary h-10"
          :disabled="loading"
          @click="loadUsage"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          <span>{{ t('common.refresh') }}</span>
        </button>
      </div>

      <div v-if="error" class="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </div>

      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
          <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.totalRequests') }}</div>
          <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">{{ formatInteger(stats?.total_requests ?? 0) }}</div>
        </div>
        <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
          <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.totalTokens') }}</div>
          <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white" :title="formatInteger(stats?.total_tokens ?? 0)">{{ formatTokenNumber(stats?.total_tokens ?? 0) }}</div>
        </div>
        <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
          <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.actualCost') }}</div>
          <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">{{ formatMoney(stats?.total_actual_cost ?? 0) }}</div>
        </div>
        <div class="rounded-xl border border-stone-200/80 bg-white px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
          <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.avgDuration') }}</div>
          <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-white">{{ formatDuration(stats?.average_duration_ms ?? 0) }}</div>
        </div>
      </div>

      <div class="border-b border-stone-200/80 dark:border-white/10">
        <div class="flex gap-1">
          <button type="button" :class="tabButtonClass(activeTab === 'trend')" @click="activeTab = 'trend'">
            <Icon name="chart" size="sm" />
            <span>{{ t('keys.usageDetails.trendTab') }}</span>
          </button>
          <button type="button" :class="tabButtonClass(activeTab === 'models')" @click="activeTab = 'models'">
            <Icon name="database" size="sm" />
            <span>{{ t('keys.usageDetails.modelsTab') }}</span>
          </button>
          <button type="button" :class="tabButtonClass(activeTab === 'logs')" @click="activeTab = 'logs'">
            <Icon name="document" size="sm" />
            <span>{{ t('keys.usageDetails.logsTab') }}</span>
          </button>
        </div>
      </div>

      <TokenUsageTrend
        v-if="activeTab === 'trend'"
        :trend-data="trendData"
        :loading="loading"
      />

      <div v-else-if="activeTab === 'models'" class="overflow-hidden rounded-xl border border-stone-200/80 dark:border-white/10">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-stone-200/80 text-sm dark:divide-white/10">
            <thead class="bg-stone-50 dark:bg-white/[0.04]">
              <tr>
                <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('usage.model') }}</th>
                <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.totalRequests') }}</th>
                <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.totalTokens') }}</th>
                <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.actualCost') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-stone-100 bg-white dark:divide-white/10 dark:bg-transparent">
              <tr v-if="loading">
                <td colspan="4" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="models.length === 0">
                <td colspan="4" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">{{ t('common.noData') }}</td>
              </tr>
              <template v-else>
                <tr v-for="model in models" :key="model.model" class="hover:bg-stone-50 dark:hover:bg-white/[0.04]">
                  <td class="max-w-[260px] truncate px-3 py-2 font-medium text-stone-950 dark:text-white" :title="model.model">{{ model.model || '-' }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300">{{ formatInteger(model.requests) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(model.total_tokens)">{{ formatTokenNumber(model.total_tokens) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums font-medium text-stone-950 dark:text-white">{{ formatMoney(model.actual_cost) }}</td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else class="overflow-hidden rounded-xl border border-stone-200/80 dark:border-white/10">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-stone-200/80 text-sm dark:divide-white/10">
            <thead class="bg-stone-50 dark:bg-white/[0.04]">
              <tr>
                <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('usage.time') }}</th>
                <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('usage.model') }}</th>
                <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('usage.type') }}</th>
                <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.tokens') }}</th>
                <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.actualCost') }}</th>
                <th class="px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400">{{ t('usage.duration') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-stone-100 bg-white dark:divide-white/10 dark:bg-transparent">
              <tr v-if="loading">
                <td colspan="6" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="logs.length === 0">
                <td colspan="6" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">{{ t('usage.noRecords') }}</td>
              </tr>
              <template v-else>
                <tr v-for="log in logs" :key="log.id" class="hover:bg-stone-50 dark:hover:bg-white/[0.04]">
                  <td class="whitespace-nowrap px-3 py-2 text-stone-700 dark:text-stone-300">{{ formatDateTime(log.created_at) }}</td>
                  <td class="max-w-[220px] truncate px-3 py-2 font-medium text-stone-950 dark:text-white" :title="log.model">{{ log.model || '-' }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-stone-700 dark:text-stone-300">{{ formatRequestType(log) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(totalTokens(log))">{{ formatTokenNumber(totalTokens(log)) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums font-medium text-stone-950 dark:text-white">{{ formatMoney(log.actual_cost) }}</td>
                  <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300">{{ formatDuration(log.duration_ms) }}</td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { parseDate } from '@internationalized/date'
import { adminAPI } from '@/api/admin'
import AppDatePicker from '@/components/common/AppDatePicker.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { formatCompactNumber, formatDateTime } from '@/utils/format'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { AdminUsageLog, AdminUser, ApiKey, ModelStat, TrendDataPoint } from '@/types'

const props = defineProps<{
  show: boolean
  user: AdminUser | null
  apiKey: ApiKey | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()

const dialogTitle = computed(() => {
  if (props.apiKey) {
    return t('admin.users.apiKeyUsageDetailsTitle', { name: props.apiKey.name })
  }
  if (props.user) {
    return t('admin.users.userUsageDetailsTitle', { email: props.user.email })
  }
  return t('admin.users.usageDetails')
})

const activeTab = ref<'trend' | 'models' | 'logs'>('trend')
const startDate = ref('')
const endDate = ref('')
const loading = ref(false)
const error = ref('')
const stats = ref<AdminUsageStatsResponse | null>(null)
const trendData = ref<TrendDataPoint[]>([])
const models = ref<ModelStat[]>([])
const logs = ref<AdminUsageLog[]>([])
let requestSeq = 0

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

const compareDateInput = (left: string, right: string) => {
  try {
    return parseDate(left).compare(parseDate(right))
  } catch {
    return left.localeCompare(right)
  }
}

const normalizeDateRange = () => {
  if (startDate.value && endDate.value && compareDateInput(startDate.value, endDate.value) > 0) {
    endDate.value = startDate.value
  }
}

const startDateModel = computed({
  get: () => startDate.value,
  set: (value: string) => {
    startDate.value = value
    if (endDate.value && compareDateInput(value, endDate.value) > 0) {
      endDate.value = value
    }
  },
})

const endDateModel = computed({
  get: () => endDate.value,
  set: (value: string) => {
    endDate.value = value
    if (startDate.value && compareDateInput(value, startDate.value) < 0) {
      startDate.value = value
    }
  },
})

const formatInteger = (value: number) => new Intl.NumberFormat().format(value)
const formatTokenNumber = (value: number) => formatCompactNumber(value)
const formatMoney = (value: number) => `$${value.toFixed(4)}`
const formatDuration = (value: number | null | undefined) => {
  if (value == null) return '-'
  return `${Math.round(value)}ms`
}

const statusBadgeClass = (status: string) => [
  'badge',
  status === 'active' ? 'badge-success' :
    status === 'quota_exhausted' ? 'badge-warning' :
      status === 'expired' ? 'badge-danger' :
        'badge-gray'
]

const tabButtonClass = (active: boolean) => [
  'inline-flex h-10 items-center gap-2 border-b-2 px-3 text-sm font-medium transition-colors',
  active
    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
    : 'border-transparent text-stone-500 hover:text-stone-950 dark:text-stone-400 dark:hover:text-white'
]

const totalTokens = (log: AdminUsageLog) =>
  log.input_tokens + log.output_tokens + log.cache_creation_tokens + log.cache_read_tokens

const formatRequestType = (log: AdminUsageLog) => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const extractErrorMessage = (err: unknown) => {
  const maybeResponse = err as { response?: { data?: { detail?: string; message?: string; error?: string } } }
  return maybeResponse.response?.data?.detail ||
    maybeResponse.response?.data?.message ||
    maybeResponse.response?.data?.error ||
    t('keys.usageDetails.loadFailed')
}

const loadUsage = async () => {
  if (!props.user) return
  normalizeDateRange()
  const seq = ++requestSeq
  loading.value = true
  error.value = ''
  const baseParams = {
    user_id: props.user.id,
    ...(props.apiKey ? { api_key_id: props.apiKey.id } : {}),
    start_date: startDate.value,
    end_date: endDate.value,
  }

  try {
    const [statsRes, trendRes, modelRes, logsRes] = await Promise.all([
      adminAPI.usage.getStats(baseParams),
      adminAPI.dashboard.getUsageTrend({ ...baseParams, granularity: 'day' }),
      adminAPI.dashboard.getModelStats(baseParams),
      adminAPI.usage.list({
        ...baseParams,
        page: 1,
        page_size: 10,
        sort_by: 'created_at',
        sort_order: 'desc',
      }),
    ])
    if (seq !== requestSeq) return
    stats.value = statsRes
    trendData.value = trendRes.trend || []
    models.value = modelRes.models || []
    logs.value = logsRes.items || []
  } catch (err) {
    if (seq !== requestSeq) return
    error.value = extractErrorMessage(err)
    stats.value = null
    trendData.value = []
    models.value = []
    logs.value = []
  } finally {
    if (seq === requestSeq) {
      loading.value = false
    }
  }
}

watch(
  () => [props.show, props.user?.id, props.apiKey?.id],
  ([show]) => {
    if (!show || !props.user) return
    activeTab.value = 'trend'
    applyDefaultRange()
    void loadUsage()
  },
  { immediate: true }
)
</script>
