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
      <button type="button" class="btn btn-secondary h-10" :disabled="loading" @click="refreshLogs">
        <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        <span>{{ t('common.refresh') }}</span>
      </button>
    </div>

    <div v-if="error" class="rounded border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="overflow-hidden rounded-xl border border-stone-200/80 dark:border-white/10">
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
              <th class="px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400">{{ t('keys.usageDetails.requestId') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-stone-100 bg-white dark:divide-white/10 dark:bg-transparent">
            <tr v-if="loading">
              <td colspan="7" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">
                {{ t('common.loading') }}
              </td>
            </tr>
            <tr v-else-if="logs.length === 0">
              <td colspan="7" class="px-3 py-8 text-center text-stone-500 dark:text-stone-400">
                {{ t('usage.noRecords') }}
              </td>
            </tr>
            <template v-else>
              <tr v-for="log in logs" :key="log.id" class="hover:bg-stone-50 dark:hover:bg-white/[0.04]">
                <td class="whitespace-nowrap px-3 py-2 text-stone-700 dark:text-stone-300">{{ formatDateTime(log.created_at) }}</td>
                <td class="max-w-[180px] truncate px-3 py-2 font-medium text-stone-950 dark:text-white" :title="log.model">{{ log.model || '-' }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-stone-700 dark:text-stone-300">{{ formatRequestType(log) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300" :title="formatInteger(totalTokens(log))">{{ formatTokenNumber(totalTokens(log)) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums font-medium text-stone-950 dark:text-white">{{ formatMoney(log.actual_cost) }}</td>
                <td class="whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300">{{ formatDuration(log.duration_ms) }}</td>
                <td class="max-w-[180px] truncate px-3 py-2 font-mono text-xs text-stone-500 dark:text-stone-400" :title="log.request_id">{{ log.request_id }}</td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3 text-sm text-stone-500 dark:text-stone-400">
      <span>{{ t('keys.usageDetails.recordsTotal', { total: pagination.total }) }}</span>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="btn btn-secondary h-9 px-3"
          :disabled="loading || pagination.page <= 1"
          @click="goToPage(pagination.page - 1)"
        >
          <Icon name="chevronLeft" size="sm" />
        </button>
        <span class="min-w-[72px] text-center tabular-nums">
          {{ pagination.page }} / {{ Math.max(pagination.pages, 1) }}
        </span>
        <button
          type="button"
          class="btn btn-secondary h-9 px-3"
          :disabled="loading || pagination.page >= pagination.pages"
          @click="goToPage(pagination.page + 1)"
        >
          <Icon name="chevronRight" size="sm" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { usageAPI } from '@/api'
import Icon from '@/components/icons/Icon.vue'
import type { UsageLog } from '@/types'
import { formatCompactNumber, formatDateTime } from '@/utils/format'

const props = withDefaults(defineProps<{
  apiKeyId: number
  active?: boolean
}>(), {
  active: true
})

const { t } = useI18n()

const logs = ref<UsageLog[]>([])
const loading = ref(false)
const error = ref('')
const startDate = ref('')
const endDate = ref('')
const pageSize = 10
const pagination = reactive({
  page: 1,
  page_size: pageSize,
  total: 0,
  pages: 0
})
let latestLogsRequestID = 0

const formatInteger = (value: number) => new Intl.NumberFormat().format(value)
const formatTokenNumber = (value: number) => formatCompactNumber(value)
const formatMoney = (value: number) => `$${value.toFixed(4)}`

const formatDateInput = (date: Date) => {
  const pad = (value: number) => value.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

const applyDefaultRange = () => {
  const end = new Date()
  const start = new Date()
  start.setDate(start.getDate() - 6)
  startDate.value = formatDateInput(start)
  endDate.value = formatDateInput(end)
}

const extractErrorMessage = (err: unknown) => {
  const maybeResponse = err as { response?: { data?: { message?: string; error?: string } } }
  return maybeResponse.response?.data?.message || maybeResponse.response?.data?.error || t('usage.failedToLoad')
}

const totalTokens = (log: UsageLog) =>
  log.input_tokens + log.output_tokens + log.cache_creation_tokens + log.cache_read_tokens

const formatDuration = (value: number | null) => {
  if (value === null || value === undefined) return '-'
  return `${value}ms`
}

const formatRequestType = (log: UsageLog) => {
  if (log.openai_ws_mode || log.request_type === 'ws_v2') return t('usage.ws')
  if (log.stream || log.request_type === 'stream') return t('usage.stream')
  if (log.request_type === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const loadLogs = async () => {
  const requestID = ++latestLogsRequestID
  loading.value = true
  error.value = ''
  try {
    const response = await usageAPI.query({
      api_key_id: props.apiKeyId,
      start_date: startDate.value,
      end_date: endDate.value,
      page: pagination.page,
      page_size: pageSize,
      sort_by: 'created_at',
      sort_order: 'desc'
    })
    if (requestID !== latestLogsRequestID) return
    logs.value = response.items || []
    pagination.page = response.page
    pagination.page_size = response.page_size
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (err) {
    if (requestID !== latestLogsRequestID) return
    error.value = extractErrorMessage(err)
    logs.value = []
  } finally {
    if (requestID === latestLogsRequestID) {
      loading.value = false
    }
  }
}

const refreshLogs = () => {
  pagination.page = 1
  void loadLogs()
}

const goToPage = (page: number) => {
  pagination.page = page
  void loadLogs()
}

watch(() => props.apiKeyId, () => {
  applyDefaultRange()
  pagination.page = 1
  if (props.active) {
    void loadLogs()
  }
})

watch(() => props.active, (active) => {
  if (active && logs.value.length === 0 && !loading.value) {
    void loadLogs()
  }
})

onMounted(() => {
  applyDefaultRange()
  if (props.active) {
    void loadLogs()
  }
})
</script>
