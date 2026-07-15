<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import OpsErrorLogTable from './OpsErrorLogTable.vue'
import { opsAPI, type OpsErrorLog } from '@/api/admin/ops'
import type { OpsErrorDetailsPreset, OpsErrorDetailsStatusCode, OpsErrorDetailType, OpsErrorDetailsView } from '../composables/useOpsModalStack'

interface Props {
  show: boolean
  timeRange: string
  customStartTime?: string | null
  customEndTime?: string | null
  platform?: string
  groupId?: number | null
  errorType: 'request' | 'upstream'
  preset?: OpsErrorDetailsPreset | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'openErrorDetail', errorId: number, errorType: OpsErrorDetailType): void
}>()

const { t } = useI18n()


const loading = ref(false)
const rows = ref<OpsErrorLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)

const q = ref('')
const statusCode = ref<OpsErrorDetailsStatusCode>(null)
const phase = ref<string>('')
const errorOwner = ref<string>('')
const viewMode = ref<OpsErrorDetailsView>('errors')
let searchTimeout: number | null = null
let resettingFilters = false
let fetchErrorLogsRequestId = 0
const rateLimitStatusCodes = '429,529'

function clearSearchTimeout() {
  if (!searchTimeout) return
  window.clearTimeout(searchTimeout)
  searchTimeout = null
}

function invalidateFetchErrorLogs() {
  fetchErrorLogsRequestId += 1
  loading.value = false
}


const modalTitle = computed(() => {
  const title = String(props.preset?.title || '').trim()
  if (title) return title
  return props.errorType === 'upstream' ? t('admin.ops.errorDetails.upstreamErrors') : t('admin.ops.errorDetails.requestErrors')
})

const statusCodeSelectOptions = computed(() => {
  const codes = [400, 401, 403, 404, 409, 422, 429, 500, 502, 503, 504, 529]
  const options: Array<{ value: OpsErrorDetailsStatusCode; label: string }> = [
    { value: null, label: t('common.all') },
    ...(props.errorType === 'upstream'
      ? [
          { value: 'rate_overload' as const, label: t('admin.ops.errorDetails.statusRateOverload') },
          { value: 'non_rate_overload' as const, label: t('admin.ops.errorDetails.statusNonRateOverload') }
        ]
      : []),
    ...codes.map((c) => ({ value: c, label: String(c) })),
    { value: 'other', label: t('admin.ops.errorDetails.statusCodeOther') || 'Other' }
  ]
  return options
})

const ownerSelectOptions = computed(() => {
  return [
    { value: '', label: t('common.all') },
    { value: 'provider', label: t('admin.ops.errorDetails.owner.provider') || 'provider' },
    { value: 'client', label: t('admin.ops.errorDetails.owner.client') || 'client' },
    { value: 'platform', label: t('admin.ops.errorDetails.owner.platform') || 'platform' }
  ]
})


const viewModeSelectOptions = computed(() => {
  return [
    { value: 'errors', label: t('admin.ops.errorDetails.viewErrors') || 'errors' },
    { value: 'excluded', label: t('admin.ops.errorDetails.viewExcluded') || 'excluded' },
    { value: 'all', label: t('admin.ops.errorDetails.viewAllFailures') || t('common.all') }
  ]
})

const phaseSelectOptions = computed(() => {
  const options = [
    { value: '', label: t('common.all') },
    { value: 'request', label: t('admin.ops.errorDetails.phase.request') || 'request' },
    { value: 'auth', label: t('admin.ops.errorDetails.phase.auth') || 'auth' },
    { value: 'account_auth', label: t('admin.ops.errorDetails.phase.account_auth') || 'account_auth' },
    { value: 'routing', label: t('admin.ops.errorDetails.phase.routing') || 'routing' },
    { value: 'upstream', label: t('admin.ops.errorDetails.phase.upstream') || 'upstream' },
    { value: 'network', label: t('admin.ops.errorDetails.phase.network') || 'network' },
    { value: 'internal', label: t('admin.ops.errorDetails.phase.internal') || 'internal' }
  ]
  return options
})

function close() {
  emit('update:show', false)
}

const sortBy = ref('created_at')
const sortOrder = ref<'asc' | 'desc'>('desc')

function onSort(nextSortBy: string, nextSortOrder: 'asc' | 'desc') {
  sortBy.value = nextSortBy
  sortOrder.value = nextSortOrder
  page.value = 1
  void fetchErrorLogs()
}

async function fetchErrorLogs() {
  if (!props.show) return

  const requestId = ++fetchErrorLogsRequestId
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: page.value,
      page_size: pageSize.value,
      view: viewMode.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value
    }

    if (props.timeRange === 'custom') {
      if (props.customStartTime && props.customEndTime) {
        params.start_time = props.customStartTime
        params.end_time = props.customEndTime
      } else {
        params.time_range = '1h'
      }
    } else {
      params.time_range = props.timeRange
    }

    const platform = String(props.platform || '').trim()
    if (platform) params.platform = platform
    if (typeof props.groupId === 'number' && props.groupId > 0) params.group_id = props.groupId

    if (q.value.trim()) params.q = q.value.trim()
    if (statusCode.value === 'other') params.status_codes_other = '1'
    else if (statusCode.value === 'rate_overload') params.status_codes = rateLimitStatusCodes
    else if (statusCode.value === 'non_rate_overload') params.status_codes_exclude = rateLimitStatusCodes
    else if (typeof statusCode.value === 'number') params.status_codes = String(statusCode.value)

    const phaseVal = String(phase.value || '').trim()
    if (phaseVal) params.phase = phaseVal

    const ownerVal = String(errorOwner.value || '').trim()
    if (ownerVal) params.error_owner = ownerVal


    const res = props.errorType === 'upstream'
      ? await opsAPI.listUpstreamErrors(params)
      : await opsAPI.listRequestErrors(params)
    if (requestId !== fetchErrorLogsRequestId) return
    rows.value = res.items || []
    total.value = res.total || 0
  } catch (err) {
    if (requestId !== fetchErrorLogsRequestId) return
    console.error('[OpsErrorDetailsModal] Failed to fetch error logs', err)
    rows.value = []
    total.value = 0
  } finally {
    if (requestId === fetchErrorLogsRequestId) {
      loading.value = false
    }
  }
}

async function resetFilters(options: { resetPageSize?: boolean } = {}) {
  // Keep filter/page watchers quiet for this Vue flush cycle; resetFilters owns
  // the single fetch after it has put all filter refs into a consistent state.
  // This relies on default watcher flush timing: related watchers run before
  // the awaited nextTick resolves. Revisit this guard before using flush: 'post'.
  resettingFilters = true
  clearSearchTimeout()
  const preset = props.preset ?? null
  q.value = ''
  statusCode.value = preset?.statusCode ?? null
  phase.value = preset?.phase ?? ''
  errorOwner.value = preset?.owner ?? ''
  viewMode.value = preset?.view ?? 'errors'
  page.value = 1
  if (options.resetPageSize) pageSize.value = 10
  fetchErrorLogs()
  try {
    await nextTick()
  } finally {
    resettingFilters = false
  }
}

function fetchFirstPage() {
  if (page.value === 1) {
    fetchErrorLogs()
    return
  }
  page.value = 1
}


watch(
  () => [props.show, props.errorType, props.preset] as const,
  ([open]) => {
    if (!open) {
      clearSearchTimeout()
      invalidateFetchErrorLogs()
      return
    }
    resetFilters({ resetPageSize: true })
  },
  { immediate: true }
)

watch(
  () => [props.timeRange, props.customStartTime, props.customEndTime, props.platform, props.groupId] as const,
  () => {
    if (!props.show) return
    fetchFirstPage()
  }
)

watch(
  () => [page.value, pageSize.value] as const,
  () => {
    if (!props.show || resettingFilters) return
    fetchErrorLogs()
  }
)

watch(
  () => q.value,
  () => {
    if (!props.show || resettingFilters) return
    clearSearchTimeout()
    searchTimeout = window.setTimeout(() => {
      searchTimeout = null
      fetchFirstPage()
    }, 350)
  }
)

watch(
  () => [statusCode.value, phase.value, errorOwner.value, viewMode.value] as const,
  () => {
    if (!props.show || resettingFilters) return
    fetchFirstPage()
  }
)
</script>

<template>
  <BaseDialog :show="show" :title="modalTitle" width="full" @close="close">
    <div class="flex h-full min-h-0 flex-col">
      <!-- Filters -->
      <div class="mb-4 flex-shrink-0 border-b border-stone-200/80 pb-4 dark:border-white/10">
        <div class="space-y-3">
          <div>
            <div class="mb-1.5 text-[11px] font-bold uppercase tracking-wider text-stone-500 dark:text-stone-500">
              {{ t('admin.ops.errorDetails.filters.search') }}
            </div>
            <div class="relative group">
              <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                <svg
                  class="h-3.5 w-3.5 text-stone-400 transition-colors group-focus-within:text-emerald-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </div>
              <input
                v-model="q"
                type="text"
                class="w-full rounded-lg border-stone-200/80 bg-stone-50/80 py-1.5 pl-9 pr-3 text-xs font-medium text-stone-700 transition-all focus:border-emerald-500/60 focus:bg-white focus:ring-2 focus:ring-emerald-500/25 dark:border-white/10 dark:bg-neutral-950/70 dark:text-stone-300 dark:focus:bg-white/[0.06]"
                :placeholder="t('admin.ops.errorDetails.searchPlaceholder')"
              />
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-5">
            <div class="ops-filter-field compact-select">
              <div class="ops-filter-label">{{ t('admin.ops.errorDetails.filters.statusCode') }}</div>
              <Select :model-value="statusCode" :options="statusCodeSelectOptions" @update:model-value="statusCode = $event as any" />
            </div>

            <div class="ops-filter-field compact-select">
              <div class="ops-filter-label">{{ t('admin.ops.errorDetails.filters.phase') }}</div>
              <Select :model-value="phase" :options="phaseSelectOptions" @update:model-value="phase = String($event ?? '')" />
            </div>

            <div class="ops-filter-field compact-select">
              <div class="ops-filter-label">{{ t('admin.ops.errorDetails.filters.owner') }}</div>
              <Select :model-value="errorOwner" :options="ownerSelectOptions" @update:model-value="errorOwner = String($event ?? '')" />
            </div>

            <div class="ops-filter-field compact-select">
              <div class="ops-filter-label">{{ t('admin.ops.errorDetails.filters.scope') }}</div>
              <Select :model-value="viewMode" :options="viewModeSelectOptions" @update:model-value="viewMode = $event as any" />
            </div>

            <div class="flex items-end">
              <button type="button" class="h-9 w-full rounded-lg bg-stone-100 px-3 text-xs font-semibold text-stone-700 transition-colors hover:bg-stone-200 dark:bg-white/[0.08] dark:text-stone-300 dark:hover:bg-white/[0.12]" @click="resetFilters()">
                {{ t('common.reset') }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Body -->
      <div class="flex min-h-0 flex-1 flex-col">
        <div class="mb-2 flex-shrink-0 text-xs text-stone-500 dark:text-stone-400">
          {{ t('admin.ops.errorDetails.total') }} {{ total }}
        </div>

          <OpsErrorLogTable
            class="min-h-0 flex-1"
            :rows="rows"
            :total="total"
            :loading="loading"
            :page="page"
            :page-size="pageSize"
            @openErrorDetail="emit('openErrorDetail', $event, props.errorType)"
            @sort="onSort"

            @update:page="page = $event"
            @update:pageSize="pageSize = $event"
          />

      </div>
    </div>
  </BaseDialog>
</template>

<style>
.compact-select .select-trigger {
  @apply w-full py-1.5 px-3 text-xs rounded-lg;
}

.ops-filter-field {
  @apply min-w-0;
}

.ops-filter-label {
  @apply mb-1.5 text-[11px] font-bold uppercase tracking-wider text-stone-500 dark:text-stone-500;
}
</style>
