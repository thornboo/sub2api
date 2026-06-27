<template>
  <AppLayout>
    <section class="space-y-5">
      <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold tracking-normal text-stone-950 dark:text-stone-50">
            {{ t('channelStatus.title') }}
          </h1>
          <p class="mt-1 max-w-3xl text-sm text-stone-500 dark:text-stone-400">
            {{ t('channelStatus.description') }}
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <div
            role="tablist"
            class="inline-flex rounded-lg border border-stone-200/80 bg-white p-0.5 text-xs shadow-sm dark:border-white/10 dark:bg-neutral-950"
          >
            <button
              v-for="opt in windowOptions"
              :key="opt.value"
              type="button"
              role="tab"
              :aria-selected="currentWindow === opt.value"
              class="rounded-md px-3 py-1.5 transition-colors"
              :class="currentWindow === opt.value
                ? 'bg-emerald-500 text-black font-semibold'
                : 'text-stone-500 hover:text-stone-800 dark:text-stone-400 dark:hover:text-stone-100'"
              @click="currentWindow = opt.value"
            >
              {{ opt.label }}
            </button>
          </div>

          <button
            type="button"
            class="btn btn-secondary btn-sm flex h-8 w-8 items-center justify-center px-0 disabled:opacity-50"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="manualReload"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>

          <AutoRefreshButton
            v-if="autoRefresh"
            :enabled="autoRefresh.enabled.value"
            :interval-seconds="autoRefresh.intervalSeconds.value"
            :countdown="autoRefresh.countdown.value"
            :intervals="autoRefresh.intervals"
            @update:enabled="autoRefresh.setEnabled"
            @update:interval="autoRefresh.setInterval"
          />
        </div>
      </div>

      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div class="rounded-lg border border-stone-200/80 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-neutral-950">
          <div class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.summary.overall') }}</div>
          <div class="mt-2 flex items-center gap-2">
            <span class="h-2.5 w-2.5 rounded-full" :class="overallDotClass"></span>
            <span class="text-lg font-semibold text-stone-950 dark:text-stone-50">{{ overallLabel }}</span>
          </div>
        </div>
        <div class="rounded-lg border border-stone-200/80 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-neutral-950">
          <div class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.summary.models') }}</div>
          <div class="mt-2 text-lg font-semibold text-stone-950 dark:text-stone-50">{{ items.length }}</div>
        </div>
        <div class="rounded-lg border border-stone-200/80 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-neutral-950">
          <div class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.summary.affected') }}</div>
          <div class="mt-2 text-lg font-semibold text-stone-950 dark:text-stone-50">{{ affectedCount }}</div>
        </div>
        <div class="rounded-lg border border-stone-200/80 bg-white p-4 shadow-sm dark:border-white/10 dark:bg-neutral-950">
          <div class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.summary.updated') }}</div>
          <div class="mt-2 truncate text-lg font-semibold text-stone-950 dark:text-stone-50">
            {{ formatRelativeTime(updatedAt) }}
          </div>
        </div>
      </div>

      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <label class="relative block w-full sm:max-w-sm">
          <Icon
            name="search"
            size="sm"
            class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-stone-400"
          />
          <input
            v-model.trim="search"
            type="search"
            class="input h-10 w-full rounded-lg pl-9"
            :placeholder="t('channelStatus.searchPlaceholder')"
          />
        </label>

        <div class="text-xs text-stone-500 dark:text-stone-400">
          {{ t('monitorCommon.nextUpdateIn', { n: countdown }) }}
        </div>
      </div>

      <div
        v-if="loading && items.length === 0"
        class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3"
      >
        <div
          v-for="i in 6"
          :key="i"
          class="min-h-[220px] animate-pulse rounded-lg border border-stone-200/80 bg-white p-5 dark:border-white/10 dark:bg-neutral-950"
        >
          <div class="h-5 w-2/3 rounded bg-stone-200 dark:bg-white/10"></div>
          <div class="mt-3 h-4 w-1/3 rounded bg-stone-100 dark:bg-white/[0.08]"></div>
          <div class="mt-6 grid grid-cols-2 gap-3">
            <div class="h-16 rounded bg-stone-100 dark:bg-white/[0.08]"></div>
            <div class="h-16 rounded bg-stone-100 dark:bg-white/[0.08]"></div>
          </div>
        </div>
      </div>

      <EmptyState
        v-else-if="filteredGroups.length === 0"
        :title="t('channelStatus.empty.title')"
        :description="t('channelStatus.empty.description')"
      />

      <div v-else class="space-y-5">
        <section
          v-for="group in filteredGroups"
          :key="group.key"
          class="space-y-3"
        >
          <div class="flex items-center justify-between gap-3">
            <h2
              class="min-w-0 truncate text-sm font-semibold text-stone-800 dark:text-stone-100"
              :title="`${t('channelStatus.groupPrefix')}${group.group_name}`"
            >
              <span class="text-stone-500 dark:text-stone-400">{{ t('channelStatus.groupPrefix') }}</span>{{ group.group_name }}
            </h2>
            <span class="flex-shrink-0 text-xs text-stone-500 dark:text-stone-400">
              {{ group.items.length }} {{ t('channelStatus.summary.models') }}
            </span>
          </div>

          <div class="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
            <button
              v-for="item in group.items"
              :key="`${item.group_id}:${item.model}`"
              type="button"
              class="group flex min-h-[220px] w-full flex-col rounded-lg border border-stone-200/80 bg-white p-5 text-left shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-emerald-500/25 hover:shadow-card-hover dark:border-white/10 dark:bg-neutral-950 dark:hover:border-emerald-500/25"
              @click="openDetail(item)"
            >
              <div class="flex items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate font-mono text-base font-semibold text-stone-950 dark:text-stone-50" :title="item.model">
                    {{ item.display_name || item.model }}
                  </div>
                  <div class="mt-1 truncate text-xs text-stone-500 dark:text-stone-400">
                    {{ t(`channelStatus.message.${item.message_code}`) }}
                  </div>
                </div>
                <span
                  class="inline-flex flex-shrink-0 items-center rounded-full px-2.5 py-1 text-xs font-semibold"
                  :class="statusBadgeClass(item.status)"
                >
                  {{ statusLabel(item.status) }}
                </span>
              </div>

              <div class="mt-5 grid grid-cols-2 gap-3">
                <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.06]">
                  <div class="text-[11px] font-medium text-stone-500 dark:text-stone-400">
                    {{ currentAvailabilityLabel }}
                  </div>
                  <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-stone-50">
                    {{ formatPercent(resolveAvailability(item)) }}
                  </div>
                </div>
                <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.06]">
                  <div class="text-[11px] font-medium text-stone-500 dark:text-stone-400">
                    {{ t('channelStatus.metrics.latency') }}
                  </div>
                  <div class="mt-1 text-lg font-semibold text-stone-950 dark:text-stone-50">
                    {{ formatLatencyWithUnit(resolveLatency(item)) }}
                  </div>
                </div>
              </div>

              <div class="mt-auto pt-5 text-xs text-stone-500 dark:text-stone-400">
                {{ t('channelStatus.metrics.lastChecked') }}:
                <span class="font-medium text-stone-700 dark:text-stone-300">{{ formatRelativeTime(item.last_checked_at) }}</span>
              </div>
            </button>
          </div>
        </section>
      </div>
    </section>

    <BaseDialog
      :show="showDetail"
      :title="detailTitle"
      width="wide"
      @close="closeDetail"
    >
      <div v-if="detailLoading" class="py-8 text-center text-sm text-stone-500">
        {{ t('common.loading') }}
      </div>
      <div v-else-if="!detail" class="py-8 text-center text-sm text-stone-500">
        {{ t('channelStatus.detailLoadError') }}
      </div>
      <div v-else class="space-y-5">
        <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div class="min-w-0">
            <div class="truncate font-mono text-lg font-semibold text-stone-950 dark:text-stone-50">
              {{ detail.display_name || detail.model }}
            </div>
            <div class="mt-1 text-sm text-stone-500 dark:text-stone-400">
              {{ detail.group_name }} ·
              {{ t(`channelStatus.message.${detail.message_code}`) }}
            </div>
          </div>
          <span
            class="inline-flex w-fit items-center rounded-full px-2.5 py-1 text-xs font-semibold"
            :class="statusBadgeClass(detail.status)"
          >
            {{ statusLabel(detail.status) }}
          </span>
        </div>

        <div class="grid grid-cols-2 gap-3 md:grid-cols-4">
          <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.06]">
            <div class="text-[11px] font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.windowTab.24h') }}</div>
            <div class="mt-1 font-semibold text-stone-950 dark:text-stone-50">{{ formatPercent(detail.availability_24h) }}</div>
          </div>
          <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.06]">
            <div class="text-[11px] font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.windowTab.7d') }}</div>
            <div class="mt-1 font-semibold text-stone-950 dark:text-stone-50">{{ formatPercent(detail.availability_7d) }}</div>
          </div>
          <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.06]">
            <div class="text-[11px] font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.windowTab.30d') }}</div>
            <div class="mt-1 font-semibold text-stone-950 dark:text-stone-50">{{ formatPercent(detail.availability_30d) }}</div>
          </div>
          <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.06]">
            <div class="text-[11px] font-medium text-stone-500 dark:text-stone-400">{{ t('channelStatus.metrics.avgLatency7d') }}</div>
            <div class="mt-1 font-semibold text-stone-950 dark:text-stone-50">{{ formatLatencyWithUnit(detail.avg_latency_7d_ms) }}</div>
          </div>
        </div>

        <MonitorTimeline
          :buckets="detail.timeline ?? []"
          :countdown-seconds="countdown"
          :length="60"
        />
      </div>

      <template #footer>
        <div class="flex justify-end">
          <button type="button" class="btn btn-secondary" @click="closeDetail">
            {{ t('channelStatus.closeDetail') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  list as listModelStatus,
  detail as fetchModelStatusDetail,
  type UserModelStatus,
  type ModelStatus,
} from '@/api/modelStatus'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AutoRefreshButton from '@/components/common/AutoRefreshButton.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import MonitorTimeline from '@/components/user/monitor/MonitorTimeline.vue'
import { DEFAULT_INTERVAL_SECONDS, STATUS_OPERATIONAL, STATUS_DEGRADED, STATUS_FAILED } from '@/constants/channelMonitor'
import { useAutoRefresh } from '@/composables/useAutoRefresh'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

type StatusWindow = '24h' | '7d' | '30d'
type OverallStatus = 'operational' | 'degraded' | 'unavailable' | 'unknown'
interface UserModelStatusGroup {
  key: string
  group_id: number
  group_name: string
  items: UserModelStatus[]
}

const { t } = useI18n()
const appStore = useAppStore()
const { statusLabel, statusBadgeClass, formatLatency, formatPercent, formatRelativeTime } = useChannelMonitorFormat()

const items = ref<UserModelStatus[]>([])
const updatedAt = ref<string | null>(null)
const loading = ref(false)
const search = ref('')
const currentWindow = ref<StatusWindow>('24h')
const showDetail = ref(false)
const detailLoading = ref(false)
const detail = ref<UserModelStatus | null>(null)
const detailTarget = ref<UserModelStatus | null>(null)

let abortController: AbortController | null = null

const autoRefresh = useAutoRefresh({
  storageKey: 'model-status-auto-refresh',
  intervals: [30, 60, 120] as const,
  defaultInterval: DEFAULT_INTERVAL_SECONDS,
  onRefresh: () => reload(true),
  shouldPause: () => document.hidden || loading.value,
})
const countdown = autoRefresh.countdown

const windowOptions = computed<{ value: StatusWindow; label: string }[]>(() => [
  { value: '24h', label: t('channelStatus.windowTab.24h') },
  { value: '7d', label: t('channelStatus.windowTab.7d') },
  { value: '30d', label: t('channelStatus.windowTab.30d') },
])

const filteredItems = computed(() => {
  const q = search.value.toLowerCase()
  const rows = q
    ? items.value.filter(item =>
      item.model.toLowerCase().includes(q)
      || (item.display_name || '').toLowerCase().includes(q)
      || (item.group_name || '').toLowerCase().includes(q)
    )
    : [...items.value]
  return rows.sort((a, b) => {
    const priority = statusPriority(a.status) - statusPriority(b.status)
    if (priority !== 0) return priority
    const groupOrder = (a.group_name || '').localeCompare(b.group_name || '')
    if (groupOrder !== 0) return groupOrder
    return a.model.localeCompare(b.model)
  })
})

const filteredGroups = computed<UserModelStatusGroup[]>(() => {
  const groups = new Map<string, UserModelStatusGroup>()
  for (const item of filteredItems.value) {
    const key = String(item.group_id)
    let group = groups.get(key)
    if (!group) {
      group = {
        key,
        group_id: item.group_id,
        group_name: item.group_name || t('channelStatus.unknownGroup'),
        items: [],
      }
      groups.set(key, group)
    }
    group.items.push(item)
  }
  return [...groups.values()].sort((a, b) => {
    const nameOrder = a.group_name.localeCompare(b.group_name)
    if (nameOrder !== 0) return nameOrder
    return a.group_id - b.group_id
  })
})

const affectedCount = computed(() =>
  items.value.filter(item => item.status !== STATUS_OPERATIONAL).length
)

const overallStatus = computed<OverallStatus>(() => {
  if (items.value.length === 0) return 'unknown'
  if (items.value.some(item => item.status === STATUS_FAILED)) return 'unavailable'
  if (items.value.some(item => item.status !== STATUS_OPERATIONAL)) return 'degraded'
  return 'operational'
})

const overallLabel = computed(() => t(`channelStatus.overall.${overallStatus.value}`))

const overallDotClass = computed(() => {
  switch (overallStatus.value) {
    case 'operational':
      return 'bg-emerald-500'
    case 'unavailable':
      return 'bg-red-500'
    case 'degraded':
      return 'bg-amber-500'
    case 'unknown':
    default:
      return 'bg-stone-300 dark:bg-white/20'
  }
})

const currentAvailabilityLabel = computed(() =>
  `${t('monitorCommon.availabilityPrefix')} · ${t(`channelStatus.windowTab.${currentWindow.value}`)}`
)

const detailTitle = computed(() =>
  detailTarget.value
    ? `${detailTarget.value.group_name} · ${detailTarget.value.display_name || detailTarget.value.model}`
    : t('channelStatus.detailTitle')
)

function statusPriority(status: ModelStatus): number {
  switch (status) {
    case STATUS_FAILED:
      return 0
    case STATUS_DEGRADED:
      return 1
    case 'unknown':
      return 2
    case STATUS_OPERATIONAL:
    default:
      return 3
  }
}

function resolveAvailability(item: UserModelStatus): number | null {
  switch (currentWindow.value) {
    case '24h':
      return item.availability_24h
    case '30d':
      return item.availability_30d
    case '7d':
    default:
      return item.availability_7d
  }
}

function resolveLatency(item: UserModelStatus): number | null {
  if (currentWindow.value === '24h') return item.avg_latency_24h_ms ?? item.latest_latency_ms
  if (currentWindow.value === '7d') return item.avg_latency_7d_ms ?? item.latest_latency_ms
  return item.latest_latency_ms
}

function formatLatencyWithUnit(ms: number | null | undefined): string {
  if (ms == null) return formatLatency(ms)
  return `${formatLatency(ms)}ms`
}

async function reload(silent = false) {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  if (!silent) loading.value = true
  try {
    const res = await listModelStatus({ signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    items.value = res.items || []
    updatedAt.value = res.updated_at
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.loadError')))
  } finally {
    if (abortController === ctrl) {
      if (!silent) loading.value = false
      countdown.value = autoRefresh.intervalSeconds.value
      abortController = null
    }
  }
}

async function manualReload() {
  await reload(false)
  if (detailTarget.value && showDetail.value) {
    await loadDetail(detailTarget.value)
  }
}

async function loadDetail(row: UserModelStatus) {
  detailLoading.value = true
  try {
    detail.value = await fetchModelStatusDetail(row.model, row.group_id)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('channelStatus.detailLoadError')))
  } finally {
    detailLoading.value = false
  }
}

function openDetail(row: UserModelStatus) {
  detailTarget.value = row
  detail.value = row
  showDetail.value = true
  void loadDetail(row)
}

function closeDetail() {
  showDetail.value = false
  detail.value = null
  detailTarget.value = null
}

watch(
  () => appStore.cachedPublicSettings?.channel_monitor_enabled,
  (enabled) => {
    if (enabled === false) autoRefresh.stop()
    else if (autoRefresh.enabled.value) autoRefresh.start()
  },
)

onMounted(() => {
  void reload(false)
  if (appStore.cachedPublicSettings?.channel_monitor_enabled !== false) {
    autoRefresh.setEnabled(true)
  }
})

onBeforeUnmount(() => {
  if (abortController) abortController.abort()
})
</script>
