<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { opsAPI, type OpsRuntimeLogConfig, type OpsSystemLog, type OpsSystemLogCleanupRequest, type OpsSystemLogSinkHealth } from '@/api/admin/ops'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useMediaQuery } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const appStore = useAppStore()
const { t } = useI18n()

// 与 DataTable 一致：< 768px 切换为卡片视图，避免宽表在移动端被截断。
const isDesktopViewport = useMediaQuery('(min-width: 768px)')

const props = withDefaults(defineProps<{
  platformFilter?: string
  refreshToken?: number
}>(), {
  platformFilter: '',
  refreshToken: 0
})

const loading = ref(false)
const logs = ref<OpsSystemLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const cleanupConfirmVisible = ref(false)
const cleanupSubmitting = ref(false)
const cleanupPayload = ref<OpsSystemLogCleanupRequest | null>(null)
const cleanupSummary = ref<Array<{ label: string; value: string }>>([])

const health = ref<OpsSystemLogSinkHealth>({
  queue_depth: 0,
  queue_capacity: 0,
  dropped_count: 0,
  write_failed_count: 0,
  written_count: 0,
  avg_write_delay_ms: 0
})

const runtimeLoading = ref(false)
const runtimeSaving = ref(false)
const runtimeConfig = reactive<OpsRuntimeLogConfig>({
  level: 'info',
  enable_sampling: false,
  sampling_initial: 100,
  sampling_thereafter: 100,
  caller: true,
  stacktrace_level: 'error',
  retention_days: 30
})

const filters = reactive({
  time_range: '1h' as '5m' | '30m' | '1h' | '6h' | '24h' | '7d' | '30d',
  start_time: '',
  end_time: '',
  host: '',
  level: '',
  component: '',
  request_id: '',
  client_request_id: '',
  user_id: '',
  api_key_id: '',
  account_id: '',
  platform: '',
  model: '',
  q: ''
})

const runtimeLevelOptions = [
  { value: 'debug', label: 'debug' },
  { value: 'info', label: 'info' },
  { value: 'warn', label: 'warn' },
  { value: 'error', label: 'error' }
]

const stacktraceLevelOptions = [
  { value: 'none', label: 'none' },
  { value: 'error', label: 'error' },
  { value: 'fatal', label: 'fatal' }
]

const timeRangeOptions = [
  { value: '5m', label: '5m' },
  { value: '30m', label: '30m' },
  { value: '1h', label: '1h' },
  { value: '6h', label: '6h' },
  { value: '24h', label: '24h' },
  { value: '7d', label: '7d' },
  { value: '30d', label: '30d' }
]

const filterLevelOptions = computed(() => [
  { value: '', label: t('admin.ops.systemLogs.all') },
  { value: 'debug', label: 'debug' },
  { value: 'info', label: 'info' },
  { value: 'warn', label: 'warn' },
  { value: 'error', label: 'error' }
])

const levelBadgeClass = (level: string) => {
  const v = String(level || '').toLowerCase()
  if (v === 'error' || v === 'fatal') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (v === 'warn' || v === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (v === 'debug') return 'bg-stone-100 text-stone-700 dark:bg-white/[0.08] dark:text-neutral-300'
  return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300'
}

const formatTime = (value: string) => {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

const getExtraString = (extra: Record<string, any> | undefined, key: string) => {
  if (!extra) return ''
  const v = extra[key]
  if (v == null) return ''
  if (typeof v === 'string') return v.trim()
  if (typeof v === 'number' || typeof v === 'boolean') return String(v)
  return ''
}

const formatSystemLogDetail = (row: OpsSystemLog) => {
  const parts: string[] = []
  const msg = String(row.message || '').trim()
  if (msg) parts.push(msg)

  const extra = row.extra || {}
  const statusCode = getExtraString(extra, 'status_code')
  const latencyMs = getExtraString(extra, 'latency_ms')
  const method = getExtraString(extra, 'method')
  const path = getExtraString(extra, 'path')
  const clientIP = getExtraString(extra, 'client_ip')
  const protocol = getExtraString(extra, 'protocol')

  const accessParts: string[] = []
  if (statusCode) accessParts.push(`status=${statusCode}`)
  if (latencyMs) accessParts.push(`latency_ms=${latencyMs}`)
  if (method) accessParts.push(`method=${method}`)
  if (path) accessParts.push(`path=${path}`)
  if (clientIP) accessParts.push(`ip=${clientIP}`)
  if (protocol) accessParts.push(`proto=${protocol}`)
  if (accessParts.length > 0) parts.push(accessParts.join(' '))

  const corrParts: string[] = []
  if (row.request_id) corrParts.push(`req=${row.request_id}`)
  if (row.client_request_id) corrParts.push(`client_req=${row.client_request_id}`)
  if (row.user_id != null) corrParts.push(`user=${row.user_id}`)
  if (row.api_key_id != null) corrParts.push(`key=${row.api_key_id}`)
  if (row.account_id != null) corrParts.push(`acc=${row.account_id}`)
  if (row.platform) corrParts.push(`platform=${row.platform}`)
  if (row.model) corrParts.push(`model=${row.model}`)
  if (corrParts.length > 0) parts.push(corrParts.join(' '))

  const errors = getExtraString(extra, 'errors')
  if (errors) parts.push(`errors=${errors}`)
  const err = getExtraString(extra, 'err') || getExtraString(extra, 'error')
  if (err) parts.push(`error=${err}`)

  // 用空格拼接，交给 CSS 自动换行，尽量“填满再换行”。
  return parts.join('  ')
}

const toRFC3339 = (value: string) => {
  if (!value) return undefined
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return undefined
  return d.toISOString()
}

const formatDateTime = (value?: string) => {
  if (!value) return '-'
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}

const timeRangeMs: Record<typeof filters.time_range, number> = {
  '5m': 5 * 60 * 1000,
  '30m': 30 * 60 * 1000,
  '1h': 60 * 60 * 1000,
  '6h': 6 * 60 * 60 * 1000,
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000
}

const resolveCleanupTimeRange = () => {
  const explicitStart = toRFC3339(filters.start_time)
  const explicitEnd = toRFC3339(filters.end_time)
  if (explicitStart || explicitEnd) {
    return { start_time: explicitStart, end_time: explicitEnd, label: `${formatDateTime(explicitStart)} - ${formatDateTime(explicitEnd)}` }
  }

  const end = new Date()
  const start = new Date(end.getTime() - timeRangeMs[filters.time_range])
  return {
    start_time: start.toISOString(),
    end_time: end.toISOString(),
    label: `最近 ${filters.time_range} (${formatDateTime(start.toISOString())} - ${formatDateTime(end.toISOString())})`
  }
}

const buildQuery = () => {
  const query: Record<string, any> = {
    page: page.value,
    page_size: pageSize.value,
    time_range: filters.time_range
  }

  if (filters.time_range === '30d') {
    query.time_range = '30d'
  }
  if (filters.start_time) query.start_time = toRFC3339(filters.start_time)
  if (filters.end_time) query.end_time = toRFC3339(filters.end_time)
  if (filters.host.trim()) query.host = filters.host.trim()
  if (filters.level.trim()) query.level = filters.level.trim()
  if (filters.component.trim()) query.component = filters.component.trim()
  if (filters.request_id.trim()) query.request_id = filters.request_id.trim()
  if (filters.client_request_id.trim()) query.client_request_id = filters.client_request_id.trim()
  if (filters.user_id.trim()) {
    const v = Number.parseInt(filters.user_id.trim(), 10)
    if (Number.isFinite(v) && v > 0) query.user_id = v
  }
  if (filters.api_key_id.trim()) {
    const v = Number.parseInt(filters.api_key_id.trim(), 10)
    if (Number.isFinite(v) && v > 0) query.api_key_id = v
  }
  if (filters.account_id.trim()) {
    const v = Number.parseInt(filters.account_id.trim(), 10)
    if (Number.isFinite(v) && v > 0) query.account_id = v
  }
  if (filters.platform.trim()) query.platform = filters.platform.trim()
  if (filters.model.trim()) query.model = filters.model.trim()
  if (filters.q.trim()) query.q = filters.q.trim()
  return query
}

const buildCleanupPayload = () => {
  const range = resolveCleanupTimeRange()
  const payload: OpsSystemLogCleanupRequest = {
    start_time: range.start_time,
    end_time: range.end_time,
    host: filters.host.trim() || undefined,
    level: filters.level.trim() || undefined,
    component: filters.component.trim() || undefined,
    request_id: filters.request_id.trim() || undefined,
    client_request_id: filters.client_request_id.trim() || undefined,
    user_id: filters.user_id.trim() ? Number.parseInt(filters.user_id.trim(), 10) : undefined,
    api_key_id: filters.api_key_id.trim() ? Number.parseInt(filters.api_key_id.trim(), 10) : undefined,
    account_id: filters.account_id.trim() ? Number.parseInt(filters.account_id.trim(), 10) : undefined,
    platform: filters.platform.trim() || undefined,
    model: filters.model.trim() || undefined,
    q: filters.q.trim() || undefined
  }

  const summary: Array<{ label: string; value: string }> = [
    { label: '时间范围', value: range.label }
  ]
  if (payload.level) summary.push({ label: '级别', value: payload.level })
  if (payload.host) summary.push({ label: t('admin.ops.systemLogs.host'), value: payload.host })
  if (payload.component) summary.push({ label: '组件', value: payload.component })
  if (payload.request_id) summary.push({ label: 'request_id', value: payload.request_id })
  if (payload.client_request_id) summary.push({ label: 'client_request_id', value: payload.client_request_id })
  if (payload.user_id) summary.push({ label: 'user_id', value: String(payload.user_id) })
  if (payload.api_key_id) summary.push({ label: 'api_key_id', value: String(payload.api_key_id) })
  if (payload.account_id) summary.push({ label: 'account_id', value: String(payload.account_id) })
  if (payload.platform) summary.push({ label: '平台', value: payload.platform })
  if (payload.model) summary.push({ label: '模型', value: payload.model })
  if (payload.q) summary.push({ label: '关键词', value: payload.q })

  return { payload, summary }
}

const fetchLogs = async () => {
  loading.value = true
  try {
    const res = await opsAPI.listSystemLogs(buildQuery())
    logs.value = res.items || []
    total.value = res.total || 0
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to fetch logs', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.systemLogs.loadFailed'))
  } finally {
    loading.value = false
  }
}

const fetchHealth = async () => {
  try {
    health.value = await opsAPI.getSystemLogSinkHealth()
  } catch {
    // 忽略健康数据读取失败，不影响主流程。
  }
}

const loadRuntimeConfig = async () => {
  runtimeLoading.value = true
  try {
    const cfg = await opsAPI.getRuntimeLogConfig()
    runtimeConfig.level = cfg.level
    runtimeConfig.enable_sampling = cfg.enable_sampling
    runtimeConfig.sampling_initial = cfg.sampling_initial
    runtimeConfig.sampling_thereafter = cfg.sampling_thereafter
    runtimeConfig.caller = cfg.caller
    runtimeConfig.stacktrace_level = cfg.stacktrace_level
    runtimeConfig.retention_days = cfg.retention_days
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to load runtime log config', err)
  } finally {
    runtimeLoading.value = false
  }
}

const saveRuntimeConfig = async () => {
  runtimeSaving.value = true
  try {
    const saved = await opsAPI.updateRuntimeLogConfig({ ...runtimeConfig })
    runtimeConfig.level = saved.level
    runtimeConfig.enable_sampling = saved.enable_sampling
    runtimeConfig.sampling_initial = saved.sampling_initial
    runtimeConfig.sampling_thereafter = saved.sampling_thereafter
    runtimeConfig.caller = saved.caller
    runtimeConfig.stacktrace_level = saved.stacktrace_level
    runtimeConfig.retention_days = saved.retention_days
    appStore.showSuccess(t('admin.ops.systemLogs.runtimeConfigActive'))
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to save runtime log config', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.systemLogs.runtimeConfigSaveFailed'))
  } finally {
    runtimeSaving.value = false
  }
}

const resetRuntimeConfig = async () => {
  const ok = window.confirm(t('admin.ops.systemLogs.resetRuntimeConfigConfirm'))
  if (!ok) return

  runtimeSaving.value = true
  try {
    const saved = await opsAPI.resetRuntimeLogConfig()
    runtimeConfig.level = saved.level
    runtimeConfig.enable_sampling = saved.enable_sampling
    runtimeConfig.sampling_initial = saved.sampling_initial
    runtimeConfig.sampling_thereafter = saved.sampling_thereafter
    runtimeConfig.caller = saved.caller
    runtimeConfig.stacktrace_level = saved.stacktrace_level
    runtimeConfig.retention_days = saved.retention_days
    appStore.showSuccess(t('admin.ops.systemLogs.runtimeConfigReset'))
    await fetchHealth()
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to reset runtime log config', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.systemLogs.runtimeConfigResetFailed'))
  } finally {
    runtimeSaving.value = false
  }
}

const openCleanupConfirm = () => {
  const next = buildCleanupPayload()
  cleanupPayload.value = next.payload
  cleanupSummary.value = next.summary
  cleanupConfirmVisible.value = true
}

const cleanupCurrentFilter = async () => {
  if (!cleanupPayload.value) return
  cleanupSubmitting.value = true
  try {
    const res = await opsAPI.cleanupSystemLogs(cleanupPayload.value)
    appStore.showSuccess(t('admin.ops.systemLogs.cleanupSuccess', { count: res.deleted || 0 }))
    cleanupConfirmVisible.value = false
    cleanupPayload.value = null
    page.value = 1
    await Promise.all([fetchLogs(), fetchHealth()])
  } catch (err: any) {
    console.error('[OpsSystemLogTable] Failed to cleanup logs', err)
    appStore.showError(
      extractApiErrorMessage(err, t('admin.ops.systemLogs.cleanupFailed'), {
        OPS_SYSTEM_LOG_CLEANUP_FILTER_REQUIRED: t('admin.ops.systemLogs.cleanupFilterRequired')
      })
    )
  } finally {
    cleanupSubmitting.value = false
  }
}

const resetFilters = () => {
  filters.time_range = '1h'
  filters.start_time = ''
  filters.end_time = ''
  filters.host = ''
  filters.level = ''
  filters.component = ''
  filters.request_id = ''
  filters.client_request_id = ''
  filters.user_id = ''
  filters.api_key_id = ''
  filters.account_id = ''
  filters.platform = props.platformFilter || ''
  filters.model = ''
  filters.q = ''
  page.value = 1
  fetchLogs()
}

watch(() => props.platformFilter, (v) => {
  if (v && !filters.platform) {
    filters.platform = v
    page.value = 1
    fetchLogs()
  }
})

watch(() => props.refreshToken, () => {
  fetchLogs()
  fetchHealth()
})

const onPageChange = (next: number) => {
  page.value = next
  fetchLogs()
}

const onPageSizeChange = (next: number) => {
  pageSize.value = next
  page.value = 1
  fetchLogs()
}

const applyFilters = () => {
  page.value = 1
  fetchLogs()
}

const hasData = computed(() => logs.value.length > 0)

onMounted(async () => {
  if (props.platformFilter) {
    filters.platform = props.platformFilter
  }
  await Promise.all([fetchLogs(), fetchHealth(), loadRuntimeConfig()])
})
</script>

<template>
  <section class="card p-4">
    <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-sm font-bold text-stone-950 dark:text-white">{{ t('admin.ops.systemLogs.title') }}</h3>
        <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">{{ t('admin.ops.systemLogs.description') }}</p>
      </div>
      <div class="flex flex-wrap items-center gap-2 text-xs">
        <span class="rounded-md bg-stone-100 px-2 py-1 text-stone-700 dark:bg-white/10 dark:text-stone-200">{{ t('admin.ops.systemLogs.queue') }} {{ health.queue_depth }}/{{ health.queue_capacity }}</span>
        <span class="rounded-md bg-stone-100 px-2 py-1 text-stone-700 dark:bg-white/10 dark:text-stone-200">{{ t('admin.ops.systemLogs.written') }} {{ health.written_count }}</span>
        <span class="rounded-md bg-amber-100 px-2 py-1 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300">{{ t('admin.ops.systemLogs.dropped') }} {{ health.dropped_count }}</span>
        <span class="rounded-md bg-red-100 px-2 py-1 text-red-700 dark:bg-red-900/30 dark:text-red-300">{{ t('admin.ops.systemLogs.failed') }} {{ health.write_failed_count }}</span>
      </div>
    </div>

    <div class="mb-4 rounded-xl border border-stone-200/80 bg-white/65 p-3 dark:border-white/10 dark:bg-black/25">
      <div class="mb-2 flex items-center justify-between">
        <div class="text-xs font-semibold text-stone-700 dark:text-neutral-200">{{ t('admin.ops.systemLogs.runtimeConfig') }}</div>
        <span v-if="runtimeLoading" class="text-xs text-stone-500">{{ t('common.loading') }}</span>
      </div>
      <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-6">
        <label class="text-xs text-stone-600 dark:text-neutral-300">
          {{ t('admin.ops.systemLogs.level') }}
          <Select v-model="runtimeConfig.level" class="mt-1" :options="runtimeLevelOptions" />
        </label>
        <label class="text-xs text-stone-600 dark:text-neutral-300">
          {{ t('admin.ops.systemLogs.stacktraceThreshold') }}
          <Select v-model="runtimeConfig.stacktrace_level" class="mt-1" :options="stacktraceLevelOptions" />
        </label>
        <label class="text-xs text-stone-600 dark:text-neutral-300">
          {{ t('admin.ops.systemLogs.samplingInitial') }}
          <input v-model.number="runtimeConfig.sampling_initial" type="number" min="1" class="input mt-1" />
        </label>
        <label class="text-xs text-stone-600 dark:text-neutral-300">
          {{ t('admin.ops.systemLogs.samplingThereafter') }}
          <input v-model.number="runtimeConfig.sampling_thereafter" type="number" min="1" class="input mt-1" />
        </label>
        <label class="text-xs text-stone-600 dark:text-neutral-300">
          {{ t('admin.ops.systemLogs.retentionDays') }}
          <input v-model.number="runtimeConfig.retention_days" type="number" min="1" max="3650" class="input mt-1" />
        </label>
        <div class="md:col-span-2 xl:col-span-6">
          <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-end">
            <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
              <label class="inline-flex items-center gap-2 text-xs text-stone-600 dark:text-neutral-300">
                <input v-model="runtimeConfig.caller" type="checkbox" />
                {{ t('admin.ops.systemLogs.caller') }}
              </label>
              <label class="inline-flex items-center gap-2 text-xs text-stone-600 dark:text-neutral-300">
                <input v-model="runtimeConfig.enable_sampling" type="checkbox" />
                {{ t('admin.ops.systemLogs.sampling') }}
              </label>
            </div>
            <div class="flex flex-wrap items-center gap-2 lg:justify-end">
              <button type="button" class="btn btn-primary btn-sm" :disabled="runtimeSaving" @click="saveRuntimeConfig">
                {{ runtimeSaving ? t('common.saving') : t('admin.ops.systemLogs.saveAndApply') }}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="runtimeSaving" @click="resetRuntimeConfig">
                {{ t('admin.ops.systemLogs.resetDefaults') }}
              </button>
            </div>
          </div>
        </div>
      </div>
      <p v-if="health.last_error" class="mt-2 text-xs text-red-600 dark:text-red-400">{{ t('admin.ops.systemLogs.latestWriteError') }} {{ health.last_error }}</p>
    </div>

    <div class="mb-4 grid grid-cols-1 gap-3 md:grid-cols-5">
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.timeRange') }}
        <Select v-model="filters.time_range" class="mt-1" :options="timeRangeOptions" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.startTime') }}
        <input v-model="filters.start_time" type="datetime-local" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.endTime') }}
        <input v-model="filters.end_time" type="datetime-local" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.level') }}
        <Select v-model="filters.level" class="mt-1" :options="filterLevelOptions" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.component') }}
        <input v-model="filters.component" type="text" class="input mt-1" :placeholder="t('admin.ops.systemLogs.componentPlaceholder')" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.host') }}
        <input v-model="filters.host" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        request_id
        <input v-model="filters.request_id" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        client_request_id
        <input v-model="filters.client_request_id" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        user_id
        <input v-model="filters.user_id" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.keyId') }}
        <input v-model="filters.api_key_id" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        account_id
        <input v-model="filters.account_id" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.platform') }}
        <input v-model="filters.platform" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.model') }}
        <input v-model="filters.model" type="text" class="input mt-1" />
      </label>
      <label class="text-xs text-stone-600 dark:text-neutral-300">
        {{ t('admin.ops.systemLogs.keyword') }}
        <input v-model="filters.q" type="text" class="input mt-1" :placeholder="t('admin.ops.systemLogs.keywordPlaceholder')" />
      </label>
    </div>

    <div class="mb-3 flex flex-wrap gap-2">
      <button type="button" class="btn btn-primary btn-sm" @click="applyFilters">{{ t('admin.ops.systemLogs.search') }}</button>
      <button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">{{ t('common.reset') }}</button>
      <button type="button" class="btn btn-danger btn-sm" @click="openCleanupConfirm">{{ t('admin.ops.systemLogs.cleanCurrentFilters') }}</button>
      <button type="button" class="btn btn-secondary btn-sm" @click="fetchHealth">{{ t('admin.ops.systemLogs.refreshHealth') }}</button>
    </div>

    <div class="overflow-hidden rounded-xl border border-stone-200/80 dark:border-white/10">
      <div v-if="loading" class="px-4 py-8 text-center text-sm text-stone-500">{{ t('common.loading') }}</div>
      <div v-else-if="!hasData" class="px-4 py-8 text-center text-sm text-stone-500">{{ t('admin.ops.systemLogs.empty') }}</div>
      <div v-else-if="!isDesktopViewport" class="divide-y divide-stone-200/70 dark:divide-white/10">
        <div v-for="row in logs" :key="row.id" class="space-y-1.5 p-3">
          <div class="flex items-center justify-between gap-2">
            <span class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold" :class="levelBadgeClass(row.level)">
              {{ row.level }}
            </span>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatTime(row.created_at) }}</span>
          </div>
          <div v-if="row.host" class="truncate text-xs text-gray-500 dark:text-gray-400" :title="row.host">
            {{ row.host }}
          </div>
          <div class="whitespace-normal break-all text-xs text-gray-700 dark:text-gray-300">
            {{ formatSystemLogDetail(row) }}
          </div>
        </div>
      </div>
      <div v-else class="overflow-auto">
        <table class="min-w-full table-fixed divide-y divide-stone-200/80 dark:divide-white/10">
          <thead class="bg-stone-50/90 dark:bg-neutral-950">
            <tr>
              <th class="w-[170px] px-3 py-2 text-left text-[11px] font-semibold text-stone-500">{{ t('admin.ops.systemLogs.time') }}</th>
              <th class="w-[160px] px-3 py-2 text-left text-[11px] font-semibold text-stone-500">{{ t('admin.ops.systemLogs.host') }}</th>
              <th class="w-[80px] px-3 py-2 text-left text-[11px] font-semibold text-stone-500">{{ t('admin.ops.systemLogs.level') }}</th>
              <th class="px-3 py-2 text-left text-[11px] font-semibold text-stone-500">{{ t('admin.ops.systemLogs.logDetails') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-stone-200/70 dark:divide-white/10">
            <tr v-for="row in logs" :key="row.id" class="align-top">
              <td class="px-3 py-2 text-xs text-stone-700 dark:text-neutral-300">{{ formatTime(row.created_at) }}</td>
              <td class="px-3 py-2 text-xs text-stone-700 dark:text-neutral-300">
                <span class="block truncate" :title="row.host || '-'">{{ row.host || '-' }}</span>
              </td>
              <td class="px-3 py-2 text-xs">
                <span class="inline-flex rounded-full px-2 py-0.5 font-semibold" :class="levelBadgeClass(row.level)">
                  {{ row.level }}
                </span>
              </td>
              <td class="px-3 py-2 text-xs text-stone-700 dark:text-neutral-300 whitespace-normal break-all">
                {{ formatSystemLogDetail(row) }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <Pagination
        :total="total"
        :page="page"
        :page-size="pageSize"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
      />
    </div>
  </section>

  <ConfirmDialog
    :show="cleanupConfirmVisible"
    title="确认清理系统日志"
    message="将按当前筛选条件删除系统日志，该操作不可恢复。"
    :confirm-text="cleanupSubmitting ? '清理中...' : '确认清理'"
    cancel-text="取消"
    danger
    @confirm="cleanupCurrentFilter"
    @cancel="cleanupConfirmVisible = false"
  >
    <div class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
      删除只影响系统日志索引，不会删除用户使用记录。请确认筛选条件符合预期。
    </div>
    <div class="rounded-xl border border-stone-200/80 bg-stone-50/80 p-3 dark:border-white/10 dark:bg-white/[0.04]">
      <div class="mb-2 text-xs font-semibold text-stone-600 dark:text-stone-300">当前清理条件</div>
      <dl class="grid gap-2 text-xs sm:grid-cols-2">
        <div v-for="item in cleanupSummary" :key="item.label" class="min-w-0">
          <dt class="text-stone-500 dark:text-stone-400">{{ item.label }}</dt>
          <dd class="mt-0.5 break-all font-medium text-stone-800 dark:text-stone-100">{{ item.value }}</dd>
        </div>
      </dl>
    </div>
  </ConfirmDialog>
</template>
