<template>
  <AppLayout>
    <div class="space-y-6">
      <UsageProfileHeader
        :user="profileHeaderUser"
        :api-key="profileHeaderApiKey"
        :start-date="startDate"
        :end-date="endDate"
        @clear-user="clearProfileUser"
        @clear-api-key="clearProfileApiKey"
        @open-balance="openProfileBalanceHistory"
        @open-api-keys="openProfileApiKeys"
        @select-user="handleProfileUserSelect"
        @select-api-key="handleProfileApiKeySelect"
      >
        <template #controls>
          <div class="min-w-0 usage-header-control">
            <span class="input-label">{{ t('admin.dashboard.timeRange') }}</span>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              v-model:start-time="startTime"
              v-model:end-time="endTime"
              @change="onDateRangeChange"
            />
          </div>
          <div class="min-w-0 usage-header-control">
            <span class="input-label">{{ t('admin.dashboard.granularity') }}</span>
            <div class="w-full">
              <Select v-model="granularity" :options="granularityOptions" @change="loadChartData" />
            </div>
          </div>
        </template>
      </UsageProfileHeader>
      <!-- Charts Section -->
      <div class="space-y-4">
        <UsageStatsCards :stats="usageStats" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <ModelDistributionChart
            v-model:source="modelDistributionSource"
            v-model:metric="modelDistributionMetric"
            :model-stats="requestedModelStats"
            :upstream-model-stats="upstreamModelStats"
            :mapping-model-stats="mappingModelStats"
            :loading="modelStatsLoading"
            :show-source-toggle="true"
            :show-metric-toggle="true"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
            :show-expand-button="true"
            @expand="openExpandedUsageChart('model')"
          />
          <GroupDistributionChart
            v-model:metric="groupDistributionMetric"
            :group-stats="groupStats"
            :loading="chartsLoading"
            :show-metric-toggle="true"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
            :show-expand-button="true"
            @expand="openExpandedUsageChart('group')"
          />
        </div>
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <EndpointDistributionChart
            v-model:source="endpointDistributionSource"
            v-model:metric="endpointDistributionMetric"
            :endpoint-stats="inboundEndpointStats"
            :upstream-endpoint-stats="upstreamEndpointStats"
            :endpoint-path-stats="endpointPathStats"
            :loading="endpointStatsLoading"
            :show-source-toggle="true"
            :show-metric-toggle="true"
            :title="t('usage.endpointDistribution')"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
            :show-expand-button="true"
            @expand="openExpandedUsageChart('endpoint')"
          />
          <TokenUsageTrend
            :trend-data="trendData"
            :loading="chartsLoading"
            :show-expand-button="true"
            @expand="openExpandedUsageChart('token')"
          />
        </div>
      </div>
      <!-- 明细区：tab 栏 + 筛选 + 内容收进同一张卡片，消除割裂感 -->
      <div class="card">
        <div class="flex flex-wrap items-center border-b border-gray-200 px-2 dark:border-dark-700 sm:px-4">
          <button
            v-for="tab in detailTabs"
            :key="tab.key"
            type="button"
            data-testid="usage-detail-tab"
            class="-mb-px inline-flex items-center gap-1.5 border-b-2 px-3 py-3 text-sm font-medium transition-colors sm:px-4"
            :class="activeTab === tab.key
              ? 'border-primary-500 text-primary-600 dark:text-primary-400'
              : 'border-transparent text-gray-500 hover:border-gray-300 hover:text-gray-700 dark:text-gray-400 dark:hover:border-dark-500 dark:hover:text-gray-200'"
            @click="switchTab(tab.key)"
          >
            <Icon :name="tab.icon" size="sm" />
            {{ tab.label }}
          </button>
        </div>

        <UsageFilters v-model="filters" ref="usageFiltersRef" flat :mode="activeTab" :show-object-filters="false" class="border-b border-gray-100 dark:border-dark-700/50" :start-date="startDate" :end-date="endDate" :exporting="exporting" :model-options="modelNameOptions" @change="applyFilters" @refresh="refreshData" @reset="resetFilters" @cleanup="openCleanupDialog" @export="exportToExcel">
          <template #after-reset>
            <div v-if="activeTab !== 'ranking'" class="relative" ref="columnDropdownRef">
              <button
                @click="toggleColumnDropdown"
                class="btn btn-secondary px-2 md:px-3"
                :title="t('admin.users.columnSettings')"
              >
                <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
                </svg>
                <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
              </button>
            </div>
          </template>
        </UsageFilters>

        <div v-show="activeTab === 'usage'" class="overflow-hidden rounded-b-2xl">
          <UsageTable
            flat
            :data="usageLogs"
            :loading="loading"
            :columns="visibleColumns"
            :server-side-sort="true"
            :default-sort-key="'created_at'"
            :default-sort-order="'desc'"
            @sort="handleSort"
            @userClick="handleUserClick"
            @userUsageClick="handleUserUsageClick"
            @apiKeyUsageClick="handleApiKeyUsageClick"
            @ipGeoBatchFailed="handleIpGeoBatchFailed"
          />
          <Pagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            :page-size-options="USAGE_PAGE_SIZE_OPTIONS"
            :persist-page-size="false"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
        <div v-show="activeTab === 'errors'" class="overflow-hidden rounded-b-2xl">
          <OpsErrorLogTable
            flat
            :rows="errRows" :total="errTotal" :loading="errLoading"
            :page="errPage" :page-size="errPageSize"
            :visible-column-keys="errVisibleColumnKeys"
            :virtual-scroll="false"
            :sticky-header="false"
            :sticky-first-column="false"
            :sticky-actions-column="false"
            user-clickable
            @userClick="handleUserClick"
            @openErrorDetail="openError"
            @sort="onErrSort"
            @update:page="onErrPage"
            @update:pageSize="onErrPageSize"
            @ipGeoBatchFailed="handleIpGeoBatchFailed" />
        </div>
        <!-- 懒挂载：首次切到该 tab 才请求排行数据，之后随筛选自动刷新 -->
        <div v-if="rankingMounted" v-show="activeTab === 'ranking'" class="overflow-hidden rounded-b-2xl">
          <UserTokenRanking
            ref="rankingRef"
            :start-date="startDate"
            :end-date="endDate"
            :filters="breakdownFilters"
            :model="filters.model"
            @select-user="handleRankingSelectUser"
          />
        </div>
      </div>
      <OpsErrorDetailModal v-model:show="showErrorModal" :error-id="selectedErrorId" :error-type="'request'" />
    </div>
  </AppLayout>
  <UsageExportProgress :show="exportProgress.show" :progress="exportProgress.progress" :current="exportProgress.current" :total="exportProgress.total" :estimated-time="exportProgress.estimatedTime" @cancel="cancelExport" />
  <UsageCleanupDialog
    :show="cleanupDialogVisible"
    :filters="filters"
    :start-date="startDate"
    :end-date="endDate"
    @close="cleanupDialogVisible = false"
  />
  <!-- Balance history modal triggered from usage table user click -->
  <UserBalanceHistoryModal
    :show="showBalanceHistoryModal"
    :user="balanceHistoryUser"
    :hide-actions="true"
    @close="showBalanceHistoryModal = false; balanceHistoryUser = null"
  />
  <UserApiKeysModal
    :show="showProfileApiKeysModal"
    :user="profileApiKeysUser"
    @close="showProfileApiKeysModal = false; profileApiKeysUser = null"
  />
  <BaseDialog
    :show="expandedUsageChart !== null"
    :title="expandedUsageChartTitle"
    width="full"
    @close="closeExpandedUsageChart"
  >
    <div class="usage-chart-modal-body">
      <ModelDistributionChart
        v-if="expandedUsageChart === 'model'"
        v-model:source="modelDistributionSource"
        v-model:metric="modelDistributionMetric"
        class="usage-expanded-chart"
        :model-stats="requestedModelStats"
        :upstream-model-stats="upstreamModelStats"
        :mapping-model-stats="mappingModelStats"
        :loading="modelStatsLoading"
        :show-source-toggle="true"
        :show-metric-toggle="true"
        :start-date="startDate"
        :end-date="endDate"
        :filters="breakdownFilters"
      />
      <GroupDistributionChart
        v-else-if="expandedUsageChart === 'group'"
        v-model:metric="groupDistributionMetric"
        class="usage-expanded-chart"
        :group-stats="groupStats"
        :loading="chartsLoading"
        :show-metric-toggle="true"
        :start-date="startDate"
        :end-date="endDate"
        :filters="breakdownFilters"
      />
      <EndpointDistributionChart
        v-else-if="expandedUsageChart === 'endpoint'"
        v-model:source="endpointDistributionSource"
        v-model:metric="endpointDistributionMetric"
        class="usage-expanded-chart"
        :endpoint-stats="inboundEndpointStats"
        :upstream-endpoint-stats="upstreamEndpointStats"
        :endpoint-path-stats="endpointPathStats"
        :loading="endpointStatsLoading"
        :show-source-toggle="true"
        :show-metric-toggle="true"
        :title="t('usage.endpointDistribution')"
        :start-date="startDate"
        :end-date="endDate"
        :filters="breakdownFilters"
      />
      <TokenUsageTrend
        v-else-if="expandedUsageChart === 'token'"
        class="usage-expanded-chart"
        :trend-data="trendData"
        :loading="chartsLoading"
      />
    </div>
  </BaseDialog>
  <Teleport to="body">
    <div
      v-if="showColumnDropdown"
      ref="columnDropdownMenuRef"
      class="popover-surface fixed z-[100000030] max-h-80 w-48 overflow-y-auto p-1"
      :style="{ top: `${columnDropdownPosition.top}px`, left: `${columnDropdownPosition.left}px` }"
    >
      <button
        v-for="col in currentToggleableColumns"
        :key="col.key"
        @click="toggleCurrentColumn(col.key)"
        class="popover-item"
      >
        <span>{{ col.label }}</span>
        <Icon
          v-if="isCurrentColumnVisible(col.key)"
          name="check"
          size="sm"
          class="text-emerald-500"
          :stroke-width="2"
        />
      </button>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, reactive, computed, nextTick, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { saveAs } from 'file-saver'
import { useRoute, useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'; import { adminAPI } from '@/api/admin'; import { adminUsageAPI } from '@/api/admin/usage'
import type { DashboardTrendGranularity } from '@/api/admin/dashboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatReasoningEffort } from '@/utils/format'
import { resolveUsageRequestType, requestTypeToLegacyStream } from '@/utils/usageRequestType'
import AppLayout from '@/components/layout/AppLayout.vue'; import Pagination from '@/components/common/Pagination.vue'; import Select from '@/components/common/Select.vue'; import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageStatsCards from '@/components/admin/usage/UsageStatsCards.vue'; import UsageFilters from '@/components/admin/usage/UsageFilters.vue'
import UsageProfileHeader from '@/components/admin/usage/UsageProfileHeader.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'; import UsageExportProgress from '@/components/admin/usage/UsageExportProgress.vue'
import UserTokenRanking from '@/components/admin/usage/UserTokenRanking.vue'
import UsageCleanupDialog from '@/components/admin/usage/UsageCleanupDialog.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import UserApiKeysModal from '@/components/admin/user/UserApiKeysModal.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import OpsErrorLogTable from '@/views/admin/ops/components/OpsErrorLogTable.vue'
import OpsErrorDetailModal from '@/views/admin/ops/components/OpsErrorDetailModal.vue'
import { listErrorLogs } from '@/api/admin/ops'
import type { OpsErrorLog } from '@/api/admin/ops'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'; import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'; import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import Icon from '@/components/icons/Icon.vue'
import type { AdminUsageLog, TrendDataPoint, ModelStat, GroupStat, EndpointStat, AdminUser } from '@/types'; import type { AdminUsageStatsResponse, AdminUsageQueryParams, SimpleApiKey, SimpleUser } from '@/api/admin/usage'
import {
  buildApiKeyProfileFilters,
  buildUserProfileFilters,
  clearApiKeyProfileFilters,
  clearUserProfileFilters,
  sanitizeAdminUsageRouteQuery,
} from '@/utils/adminUsageProfile'

const { t } = useI18n()
const appStore = useAppStore()
type DistributionMetric = 'tokens' | 'actual_cost'
type EndpointSource = 'inbound' | 'upstream' | 'path'
type ModelDistributionSource = 'requested' | 'upstream' | 'mapping'
type ExpandedUsageChart = 'model' | 'group' | 'endpoint' | 'token'
interface UsageProfileEntity {
  id: number
  label?: string | null
  loading?: boolean
  notFound?: boolean
}
const route = useRoute()
const router = useRouter()
const usageStats = ref<AdminUsageStatsResponse | null>(null); const usageLogs = ref<AdminUsageLog[]>([]); const loading = ref(false); const exporting = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const requestedModelStats = ref<ModelStat[]>([]); const upstreamModelStats = ref<ModelStat[]>([]); const mappingModelStats = ref<ModelStat[]>([]); const groupStats = ref<GroupStat[]>([]); const chartsLoading = ref(false); const modelStatsLoading = ref(false); const granularity = ref<DashboardTrendGranularity>('hour')
const modelDistributionMetric = ref<DistributionMetric>('tokens')
const modelDistributionSource = ref<ModelDistributionSource>('requested')
const loadedModelSources = reactive<Record<ModelDistributionSource, boolean>>({
  requested: false,
  upstream: false,
  mapping: false,
})
const groupDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionMetric = ref<DistributionMetric>('tokens')
const endpointDistributionSource = ref<EndpointSource>('inbound')
const inboundEndpointStats = ref<EndpointStat[]>([])
const upstreamEndpointStats = ref<EndpointStat[]>([])
const endpointPathStats = ref<EndpointStat[]>([])
const endpointStatsLoading = ref(false)
const expandedUsageChart = ref<ExpandedUsageChart | null>(null)
let abortController: AbortController | null = null; let exportAbortController: AbortController | null = null
let chartReqSeq = 0
let statsReqSeq = 0
let modelStatsReqSeq = 0
const exportProgress = reactive({ show: false, progress: 0, current: 0, total: 0, estimatedTime: '' })
const cleanupDialogVisible = ref(false)
// Balance history modal state
const showBalanceHistoryModal = ref(false)
const balanceHistoryUser = ref<AdminUser | null>(null)
const showProfileApiKeysModal = ref(false)
const profileApiKeysUser = ref<AdminUser | null>(null)
const profileHeaderUser = ref<UsageProfileEntity | null>(null)
const profileHeaderApiKey = ref<UsageProfileEntity | null>(null)
const profileUserRecord = ref<AdminUser | null>(null)
let profileLookupSeq = 0

const breakdownFilters = computed(() => {
  const f: Record<string, any> = {}
  if (filters.value.user_id) f.user_id = filters.value.user_id
  if (filters.value.api_key_id) f.api_key_id = filters.value.api_key_id
  if (filters.value.account_id) f.account_id = filters.value.account_id
  if (filters.value.group_id) f.group_id = filters.value.group_id
  if (filters.value.request_type != null) f.request_type = filters.value.request_type
  if (filters.value.billing_type != null) f.billing_type = filters.value.billing_type
  return f
})

const modelNameOptions = computed(() =>
  Array.from(new Set(requestedModelStats.value.map((m) => m.model).filter(Boolean))).sort()
)

const expandedUsageChartTitle = computed(() => {
  switch (expandedUsageChart.value) {
    case 'model':
      return t('admin.dashboard.modelDistribution')
    case 'group':
      return t('admin.dashboard.groupDistribution')
    case 'endpoint':
      return t('usage.endpointDistribution')
    case 'token':
      return t('admin.dashboard.tokenUsageTrend')
    default:
      return ''
  }
})

const openExpandedUsageChart = (chart: ExpandedUsageChart) => {
  expandedUsageChart.value = chart
}

const closeExpandedUsageChart = () => {
  expandedUsageChart.value = null
}

const handleUserClick = async (userId: number) => {
  try {
    const user = await adminAPI.users.getById(userId, true)
    balanceHistoryUser.value = user
    showBalanceHistoryModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}

const toRouteQuery = (params: AdminUsageQueryParams): Record<string, string> => {
  const query: Record<string, string> = {}
  if (params.user_id) query.user_id = String(params.user_id)
  if (params.api_key_id) query.api_key_id = String(params.api_key_id)
  if (params.account_id) query.account_id = String(params.account_id)
  if (params.group_id) query.group_id = String(params.group_id)
  if (params.model) query.model = params.model
  if (params.request_type) query.request_type = params.request_type
  if (params.billing_type != null) query.billing_type = String(params.billing_type)
  if (params.billing_mode) query.billing_mode = params.billing_mode
  if (params.start_date) query.start_date = params.start_date
  if (params.end_date) query.end_date = params.end_date
  return query
}

const getCurrentRouteQuerySnapshot = (): Record<string, string> => {
  const query: Record<string, string> = {}
  Object.entries(route.query).forEach(([key, value]) => {
    const raw = Array.isArray(value)
      ? value.find((item): item is string => typeof item === 'string' && item.trim().length > 0)
      : value
    if (typeof raw === 'string' && raw.trim().length > 0) {
      query[key] = raw.trim()
    }
  })
  return query
}

const areRouteQueriesEqual = (a: Record<string, string>, b: Record<string, string>): boolean => {
  const aKeys = Object.keys(a).sort()
  const bKeys = Object.keys(b).sort()
  if (aKeys.length !== bKeys.length) return false
  return aKeys.every((key, index) => key === bKeys[index] && a[key] === b[key])
}

const replaceRouteQueryFromFilters = () => {
  const nextQuery = toRouteQuery(filters.value)
  if (!areRouteQueriesEqual(getCurrentRouteQuerySnapshot(), nextQuery)) {
    void router.replace({ path: route.path, query: nextQuery })
  }
}

const setProfileFilters = async (next: AdminUsageQueryParams) => {
  filters.value = {
    ...next,
    start_date: next.start_date || startDate.value,
    end_date: next.end_date || endDate.value,
  }
  startDate.value = filters.value.start_date || startDate.value
  endDate.value = filters.value.end_date || endDate.value
  await router.replace({ path: route.path, query: toRouteQuery(filters.value) })
  applyFilters()
  void resolveProfileContext()
}

const hydrateApiKeyFromUsageLogs = () => {
  const apiKeyId = filters.value.api_key_id
  if (!apiKeyId || profileHeaderApiKey.value?.label || profileHeaderApiKey.value?.notFound) return
  const log = usageLogs.value.find((item) => item.api_key_id === apiKeyId && item.api_key)
  if (!log?.api_key) return
  profileHeaderApiKey.value = {
    id: apiKeyId,
    label: log.api_key.name || `#${apiKeyId}`,
    loading: false,
  }
}

const resolveProfileContext = async () => {
  const seq = ++profileLookupSeq
  const userId = filters.value.user_id
  const apiKeyId = filters.value.api_key_id
  profileUserRecord.value = null
  profileHeaderUser.value = userId ? { id: userId, loading: true } : null
  profileHeaderApiKey.value = apiKeyId ? { id: apiKeyId, loading: true } : null

  let userMissing = false
  if (userId) {
    try {
      const user = await adminAPI.users.getById(userId, true)
      if (seq !== profileLookupSeq) return
      profileUserRecord.value = user
      profileHeaderUser.value = {
        id: userId,
        label: user.email || `#${userId}`,
        loading: false,
      }
    } catch {
      if (seq !== profileLookupSeq) return
      userMissing = true
      profileHeaderUser.value = {
        id: userId,
        loading: false,
        notFound: true,
      }
    }
  }

  if (!apiKeyId) return

  if (userId && !userMissing) {
    try {
      const keys = await adminAPI.usage.searchApiKeys(userId, '', {
        includeDeleted: true,
        apiKeyId,
      })
      if (seq !== profileLookupSeq) return
      const key = keys[0]
      profileHeaderApiKey.value = key
        ? { id: apiKeyId, label: key.name || `#${apiKeyId}`, loading: false }
        : { id: apiKeyId, loading: false, notFound: true }
    } catch {
      if (seq !== profileLookupSeq) return
      profileHeaderApiKey.value = { id: apiKeyId, loading: false, notFound: true }
    }
    return
  }

  try {
    const keys = await adminAPI.usage.searchApiKeys(undefined, '', {
      includeDeleted: true,
      apiKeyId,
    })
    if (seq !== profileLookupSeq) return
    const key = keys[0]
    profileHeaderApiKey.value = key
      ? { id: apiKeyId, label: key.name || `#${apiKeyId}`, loading: false }
      : { id: apiKeyId, loading: false, notFound: true }
  } catch {
    if (seq !== profileLookupSeq) return
    profileHeaderApiKey.value = { id: apiKeyId, loading: false }
    hydrateApiKeyFromUsageLogs()
  }
}

const handleUserUsageClick = (userId: number, email?: string) => {
  profileHeaderUser.value = { id: userId, label: email || `#${userId}`, loading: false }
  void setProfileFilters(buildUserProfileFilters(filters.value, userId))
}

const handleApiKeyUsageClick = (apiKeyId: number, userId?: number, keyName?: string) => {
  profileHeaderApiKey.value = { id: apiKeyId, label: keyName || `#${apiKeyId}`, loading: false }
  void setProfileFilters(buildApiKeyProfileFilters(filters.value, apiKeyId, { user_id: userId ?? filters.value.user_id }))
}

const handleProfileUserSelect = (user: SimpleUser) => {
  profileHeaderUser.value = { id: user.id, label: user.email, loading: false }
  profileHeaderApiKey.value = null
  void setProfileFilters(buildUserProfileFilters(filters.value, user.id))
}

const handleProfileApiKeySelect = (apiKey: SimpleApiKey) => {
  profileHeaderApiKey.value = { id: apiKey.id, label: apiKey.name || `#${apiKey.id}`, loading: false }
  void setProfileFilters(buildApiKeyProfileFilters(filters.value, apiKey.id, { user_id: apiKey.user_id || filters.value.user_id }))
}

const clearProfileUser = () => {
  void setProfileFilters(clearUserProfileFilters(filters.value))
}

const clearProfileApiKey = () => {
  void setProfileFilters(clearApiKeyProfileFilters(filters.value))
}

const openProfileBalanceHistory = async () => {
  const userId = filters.value.user_id
  if (!userId) return
  if (profileUserRecord.value) {
    balanceHistoryUser.value = profileUserRecord.value
    showBalanceHistoryModal.value = true
    return
  }
  await handleUserClick(userId)
}

const openProfileApiKeys = async () => {
  const userId = filters.value.user_id
  if (!userId) return
  if (profileUserRecord.value) {
    profileApiKeysUser.value = profileUserRecord.value
    showProfileApiKeysModal.value = true
    return
  }
  try {
    const user = await adminAPI.users.getById(userId, true)
    profileUserRecord.value = user
    profileApiKeysUser.value = user
    showProfileApiKeysModal.value = true
  } catch {
    appStore.showError(t('admin.usage.failedToLoadUser'))
  }
}

const granularityOptions = computed(() => [
  { value: 'day', label: t('admin.dashboard.day') },
  { value: 'hour', label: t('admin.dashboard.hour') },
  { value: 'month', label: t('admin.dashboard.month') },
])

// Drill down from the per-user token ranking: scope the whole usage view to
// that user and jump to the usage-detail tab so the drill-down is visible.
const handleRankingSelectUser = (userId: number, email: string) => {
  filters.value = { ...filters.value, user_id: userId }
  usageFiltersRef.value?.setUserKeyword?.(email || '')
  activeTab.value = 'usage'
  applyFilters()
}
// Use local timezone to avoid UTC timezone issues
const formatLD = (d: Date) => {
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLD(start),
    end: formatLD(end)
  }
}
const getGranularityForRange = (start: string, end: string): DashboardTrendGranularity => {
  const startTime = new Date(`${start}T00:00:00`).getTime()
  const endTime = new Date(`${end}T00:00:00`).getTime()
  const daysDiff = Math.ceil((endTime - startTime) / (1000 * 60 * 60 * 24))
  if (daysDiff <= 1) return 'hour'
  if (daysDiff > 60) return 'month'
  return 'day'
}
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start); const endDate = ref(defaultRange.end)
const filters = ref<AdminUsageQueryParams>({ user_id: undefined, model: undefined, group_id: undefined, request_type: undefined, billing_type: null, start_date: startDate.value, end_date: endDate.value })

// --- Precise time (to the second) bounds, driven by DateRangePicker's optional time inputs ---
// Empty string = that bound has no time → falls back to date-only behavior on the backend.
const startTime = ref('')
const endTime = ref('')
const preciseTimeParams = computed<{ start_time?: string; end_time?: string }>(() => {
  const p: { start_time?: string; end_time?: string } = {}
  if (startTime.value) p.start_time = startTime.value
  if (endTime.value) p.end_time = endTime.value
  return p
})
const USAGE_PAGE_SIZE_OPTIONS = [10, 20, 50, 100]
const normalizeUsagePageSize = (value: number): number =>
  USAGE_PAGE_SIZE_OPTIONS.find((option) => option >= value) ?? USAGE_PAGE_SIZE_OPTIONS[USAGE_PAGE_SIZE_OPTIONS.length - 1]
const pagination = reactive({ page: 1, page_size: normalizeUsagePageSize(getPersistedPageSize()), total: 0 })
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

const applyRouteQueryFilters = () => {
  const sanitized = sanitizeAdminUsageRouteQuery(route.query, {
    startDate: defaultRange.start,
    endDate: defaultRange.end,
  })
  startDate.value = sanitized.startDate || defaultRange.start
  endDate.value = sanitized.endDate || defaultRange.end
  filters.value = sanitized.filters
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
  const currentQuery = getCurrentRouteQuerySnapshot()
  if (!areRouteQueriesEqual(currentQuery, sanitized.routeQuery)) {
    void router.replace({ path: route.path, query: sanitized.routeQuery })
  }
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  filters.value = {
    ...filters.value,
    start_date: range.startDate,
    end_date: range.endDate
  }
  granularity.value = getGranularityForRange(range.startDate, range.endDate)
  replaceRouteQueryFromFilters()
  applyFilters()
}

const buildUsageListParams = (
  page: number,
  pageSize: number,
  exactTotal: boolean
): AdminUsageQueryParams => {
  const requestType = filters.value.request_type
  const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
  return {
    page,
    page_size: pageSize,
    exact_total: exactTotal,
    ...filters.value,
    ...preciseTimeParams.value,
    stream: legacyStream === null ? undefined : legacyStream,
    sort_by: sortState.sort_by,
    sort_order: sortState.sort_order
  }
}

const loadLogs = async () => {
  abortController?.abort(); const c = new AbortController(); abortController = c; loading.value = true
  try {
    const res = await adminAPI.usage.list(
      buildUsageListParams(pagination.page, pagination.page_size, false),
      { signal: c.signal }
    )
    if(!c.signal.aborted) {
      usageLogs.value = res.items
      pagination.total = res.total
      hydrateApiKeyFromUsageLogs()
    }
  } catch (error: any) { if(error?.name !== 'AbortError') console.error('Failed to load usage logs:', error) } finally { if(abortController === c) loading.value = false }
}
const loadStats = async (force = false) => {
  const seq = ++statsReqSeq
  endpointStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const s = await adminAPI.usage.getStats({
      ...filters.value,
      ...preciseTimeParams.value,
      stream: legacyStream === null ? undefined : legacyStream,
      ...(force ? { nocache: 1 } : {}),
    })
    if (seq !== statsReqSeq) return
    usageStats.value = s
    inboundEndpointStats.value = s.endpoints || []
    upstreamEndpointStats.value = s.upstream_endpoints || []
    endpointPathStats.value = s.endpoint_paths || []
  } catch (error) {
    if (seq !== statsReqSeq) return
    console.error('Failed to load usage stats:', error)
    inboundEndpointStats.value = []
    upstreamEndpointStats.value = []
    endpointPathStats.value = []
  } finally {
    if (seq === statsReqSeq) endpointStatsLoading.value = false
  }
}

// 失效模型统计缓存:仅标记需要重取,保留旧数据直到新数据到达(避免刷新时图表闪空)。
const invalidateModelStatsCache = () => {
  loadedModelSources.requested = false
  loadedModelSources.upstream = false
  loadedModelSources.mapping = false
}

const loadModelStats = async (source: ModelDistributionSource, force = false) => {
  if (!force && loadedModelSources[source]) {
    return
  }

  const seq = ++modelStatsReqSeq
  modelStatsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const baseParams = {
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      ...preciseTimeParams.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      group_id: filters.value.group_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      billing_type: filters.value.billing_type,
    }

    const response = await adminAPI.dashboard.getModelStats({ ...baseParams, model_source: source })

    if (seq !== modelStatsReqSeq) return

    const models = response.models || []
    if (source === 'requested') {
      requestedModelStats.value = models
    } else if (source === 'upstream') {
      upstreamModelStats.value = models
    } else {
      mappingModelStats.value = models
    }
    loadedModelSources[source] = true
  } catch (error) {
    if (seq !== modelStatsReqSeq) return
    console.error('Failed to load model stats:', error)
    if (source === 'requested') {
      requestedModelStats.value = []
    } else if (source === 'upstream') {
      upstreamModelStats.value = []
    } else {
      mappingModelStats.value = []
    }
    loadedModelSources[source] = false
  } finally {
    if (seq === modelStatsReqSeq) modelStatsLoading.value = false
  }
}

const loadChartData = async () => {
  const seq = ++chartReqSeq
  chartsLoading.value = true
  try {
    const requestType = filters.value.request_type
    const legacyStream = requestType ? requestTypeToLegacyStream(requestType) : filters.value.stream
    const snapshot = await adminAPI.dashboard.getSnapshotV2({
      start_date: filters.value.start_date || startDate.value,
      end_date: filters.value.end_date || endDate.value,
      ...preciseTimeParams.value,
      granularity: granularity.value,
      user_id: filters.value.user_id,
      model: filters.value.model,
      api_key_id: filters.value.api_key_id,
      account_id: filters.value.account_id,
      group_id: filters.value.group_id,
      request_type: requestType,
      stream: legacyStream === null ? undefined : legacyStream,
      billing_type: filters.value.billing_type,
      include_stats: false,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
      include_users_trend: false
    })
    if (seq !== chartReqSeq) return
    trendData.value = snapshot.trend || []
    groupStats.value = snapshot.groups || []
  } catch (error) { console.error('Failed to load chart data:', error) } finally { if (seq === chartReqSeq) chartsLoading.value = false }
}
const applyFilters = () => {
  pagination.page = 1
  invalidateModelStatsCache()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
  errPage.value = 1
  if (activeTab.value === 'errors') {
    loadAdminErrors()
  } else {
    errRows.value = []
  }
}
const refreshData = () => {
  invalidateModelStatsCache()
  loadLogs()
  loadStats(true)
  loadModelStats(modelDistributionSource.value, true)
  loadChartData()
  if (activeTab.value === 'errors') loadAdminErrors()
  if (rankingMounted.value) rankingRef.value?.reload()
}
const resetFilters = () => {
  const range = getLast24HoursRangeDates()
  startDate.value = range.start
  endDate.value = range.end
  filters.value = { start_date: startDate.value, end_date: endDate.value, request_type: undefined, billing_type: null, billing_mode: undefined }
  granularity.value = getGranularityForRange(startDate.value, endDate.value)
  profileHeaderUser.value = null
  profileHeaderApiKey.value = null
  profileUserRecord.value = null
  void router.replace({ path: route.path, query: toRouteQuery(filters.value) })
  applyFilters()
}
const handlePageChange = (p: number) => { pagination.page = p; loadLogs() }
const handlePageSizeChange = (s: number) => { pagination.page_size = normalizeUsagePageSize(s); pagination.page = 1; loadLogs() }
const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadLogs()
}

const handleIpGeoBatchFailed = () => {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}
const cancelExport = () => exportAbortController?.abort()
const openCleanupDialog = () => { cleanupDialogVisible.value = true }
const getRequestTypeLabel = (log: AdminUsageLog): string => {
  const requestType = resolveUsageRequestType(log)
  if (requestType === 'cyber') return t('usage.cyber')
  if (requestType === 'ws_v2') return t('usage.ws')
  if (requestType === 'stream') return t('usage.stream')
  if (requestType === 'sync') return t('usage.sync')
  return t('usage.unknown')
}

const exportToExcel = async () => {
  if (exporting.value) return; exporting.value = true; exportProgress.show = true
  const c = new AbortController(); exportAbortController = c
  try {
    let p = 1; let total = pagination.total; let exportedCount = 0
    const XLSX = await import('xlsx')
    const headers = [
      t('usage.time'), t('admin.usage.user'), t('admin.usage.apiKeyId'), t('usage.apiKeyFilter'),
      t('admin.usage.apiKeyStatus'), t('admin.usage.apiKeyDeletedAt'),
      t('admin.usage.account'), t('usage.model'), t('usage.upstreamModel'), t('usage.reasoningEffort'), t('admin.usage.group'),
      t('usage.inboundEndpoint'), t('usage.upstreamEndpoint'),
      t('usage.type'),
      t('admin.usage.inputTokens'), t('admin.usage.outputTokens'),
      t('admin.usage.cacheReadTokens'), t('admin.usage.cacheCreationTokens'),
      t('admin.usage.inputCost'), t('admin.usage.outputCost'),
      t('admin.usage.cacheReadCost'), t('admin.usage.cacheCreationCost'),
      t('usage.rate'), t('usage.accountMultiplier'), t('usage.original'), t('usage.userBilled'), t('usage.accountBilled'),
      t('usage.firstToken'), t('usage.duration'),
      t('admin.usage.requestId'), t('usage.userAgent'), t('admin.usage.ipAddress')
    ]
    const ws = XLSX.utils.aoa_to_sheet([headers])
    while (true) {
      const res = await adminUsageAPI.list(
        buildUsageListParams(p, 100, true),
        { signal: c.signal }
      )
      if (c.signal.aborted) break; if (p === 1) { total = res.total; exportProgress.total = total }
      const rows = (res.items || []).map((log: AdminUsageLog) => [
        log.created_at, log.user?.email || '', log.api_key_id || '', log.api_key?.name || '',
        log.api_key?.deleted_at ? t('admin.usage.apiKeyDeletedBadge') : (log.api_key ? t('admin.usage.apiKeyActiveBadge') : ''),
        log.api_key?.deleted_at || '', log.account?.name || '', log.model,
        log.upstream_model || '', formatReasoningEffort(log.reasoning_effort), log.group?.name || '',
        log.inbound_endpoint || '', log.upstream_endpoint || '', getRequestTypeLabel(log),
        log.input_tokens, log.output_tokens, log.cache_read_tokens, log.cache_creation_tokens,
        log.input_cost?.toFixed(6) || '0.000000', log.output_cost?.toFixed(6) || '0.000000',
        log.cache_read_cost?.toFixed(6) || '0.000000', log.cache_creation_cost?.toFixed(6) || '0.000000',
        log.rate_multiplier?.toPrecision(4) || '1.00', (log.account_rate_multiplier ?? 1).toPrecision(4),
        log.total_cost?.toFixed(6) || '0.000000', log.actual_cost?.toFixed(6) || '0.000000',
        ((log.account_stats_cost ?? log.total_cost) * (log.account_rate_multiplier ?? 1)).toFixed(6), log.first_token_ms ?? '', log.duration_ms,
        log.request_id || '', log.user_agent || '', log.ip_address || ''
      ])
      if (rows.length) {
        XLSX.utils.sheet_add_aoa(ws, rows, { origin: -1 })
      }
      exportedCount += rows.length
      exportProgress.current = exportedCount
      exportProgress.progress = total > 0 ? Math.min(100, Math.round(exportedCount / total * 100)) : 0
      if (exportedCount >= total || res.items.length < 100) break; p++
    }
    if(!c.signal.aborted) {
      const wb = XLSX.utils.book_new()
      XLSX.utils.book_append_sheet(wb, ws, 'Usage')
      saveAs(new Blob([XLSX.write(wb, { bookType: 'xlsx', type: 'array' })], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' }), `usage_${filters.value.start_date}_to_${filters.value.end_date}.xlsx`)
      appStore.showSuccess(t('usage.exportSuccess'))
    }
  } catch (error) { console.error('Failed to export:', error); appStore.showError('Export Failed') }
  finally { if(exportAbortController === c) { exportAbortController = null; exporting.value = false; exportProgress.show = false } }
}

// Column visibility
const ALWAYS_VISIBLE = ['user', 'created_at']
const DEFAULT_HIDDEN_COLUMNS = ['reasoning_effort', 'user_agent']
const HIDDEN_COLUMNS_KEY = 'usage-hidden-columns'

const allColumns = computed(() => [
  { key: 'user', label: t('admin.usage.user'), sortable: false },
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'account', label: t('admin.usage.account'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'group', label: t('admin.usage.group'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
  { key: 'ip_address', label: t('admin.usage.ipAddress'), sortable: false }
])

const hiddenColumns = reactive<Set<string>>(new Set())

const toggleableColumns = computed(() =>
  allColumns.value.filter(col => !ALWAYS_VISIBLE.includes(col.key))
)

const visibleColumns = computed(() =>
  allColumns.value.filter(col =>
    ALWAYS_VISIBLE.includes(col.key) || !hiddenColumns.has(col.key)
  )
)

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const toggleColumn = (key: string) => {
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
  } catch (e) {
    console.error('Failed to save columns:', e)
  }
}

// ---- 错误请求 tab 列设置(与用量明细同机制,独立存储) ----
const ERR_ALWAYS_VISIBLE = ['user', 'status', 'created_at', 'actions']
const ERR_DEFAULT_HIDDEN_COLUMNS = ['user_agent']
const ERR_HIDDEN_COLUMNS_KEY = 'usage-error-hidden-columns'

// key 集合须与 OpsErrorLogTable 内部 allColumns 一致
const errAllColumns = computed(() => [
  { key: 'user', label: t('admin.ops.errorLog.user') },
  { key: 'api_key', label: t('admin.ops.errorLog.apiKey') },
  { key: 'account', label: t('admin.ops.errorLog.account') },
  { key: 'platform', label: t('admin.ops.errorLog.platform') },
  { key: 'model', label: t('admin.ops.errorLog.model') },
  { key: 'endpoint', label: t('admin.ops.errorLog.endpoint') },
  { key: 'group', label: t('admin.ops.errorLog.group') },
  { key: 'type', label: t('admin.ops.errorLog.type') },
  { key: 'category', label: t('usage.errors.category') },
  { key: 'status', label: t('admin.ops.errorLog.status') },
  { key: 'message', label: t('admin.ops.errorLog.message') },
  { key: 'created_at', label: t('admin.ops.errorLog.time') },
  { key: 'user_agent', label: t('usage.userAgent') },
  { key: 'client_ip', label: t('admin.ops.errorLog.ip') },
  { key: 'actions', label: t('admin.ops.errorLog.action') },
])

const errHiddenColumns = reactive<Set<string>>(new Set())

const errToggleableColumns = computed(() =>
  errAllColumns.value.filter(col => !ERR_ALWAYS_VISIBLE.includes(col.key))
)

const errVisibleColumnKeys = computed(() =>
  errAllColumns.value
    .filter(col => ERR_ALWAYS_VISIBLE.includes(col.key) || !errHiddenColumns.has(col.key))
    .map(col => col.key)
)

const toggleErrColumn = (key: string) => {
  if (errHiddenColumns.has(key)) {
    errHiddenColumns.delete(key)
  } else {
    errHiddenColumns.add(key)
  }
  try {
    localStorage.setItem(ERR_HIDDEN_COLUMNS_KEY, JSON.stringify([...errHiddenColumns]))
  } catch (e) {
    console.error('Failed to save error columns:', e)
  }
}

const loadSavedErrColumns = () => {
  try {
    const saved = localStorage.getItem(ERR_HIDDEN_COLUMNS_KEY)
    const keys = saved ? (JSON.parse(saved) as string[]) : ERR_DEFAULT_HIDDEN_COLUMNS
    keys.forEach((key) => errHiddenColumns.add(key))
  } catch {
    ERR_DEFAULT_HIDDEN_COLUMNS.forEach((key) => errHiddenColumns.add(key))
  }
}

// 列设置下拉按当前 tab 分发
const currentToggleableColumns = computed(() =>
  activeTab.value === 'errors' ? errToggleableColumns.value : toggleableColumns.value
)
const isCurrentColumnVisible = (key: string) =>
  activeTab.value === 'errors' ? !errHiddenColumns.has(key) : isColumnVisible(key)
const toggleCurrentColumn = (key: string) =>
  activeTab.value === 'errors' ? toggleErrColumn(key) : toggleColumn(key)

const loadSavedColumns = () => {
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      (JSON.parse(saved) as string[]).forEach((key) => {
        hiddenColumns.add(key)
      })
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
        hiddenColumns.add(key)
      })
    }
  } catch {
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => {
      hiddenColumns.add(key)
    })
  }
}

// Detail tabs
type DetailTab = 'usage' | 'errors' | 'ranking'
const activeTab = ref<DetailTab>('usage')
const detailTabs = computed(() => [
  { key: 'usage' as const, label: t('usage.tabs.usage'), icon: 'document' as const },
  { key: 'errors' as const, label: t('usage.tabs.errors'), icon: 'exclamationTriangle' as const },
  { key: 'ranking' as const, label: t('usage.tabs.ranking'), icon: 'chart' as const },
])
const usageFiltersRef = ref<InstanceType<typeof UsageFilters> | null>(null)
const rankingMounted = ref(false)
const rankingRef = ref<InstanceType<typeof UserTokenRanking> | null>(null)

const switchTab = (tab: DetailTab) => {
  activeTab.value = tab
  if (tab === 'errors' && errRows.value.length === 0) loadAdminErrors()
  if (tab === 'ranking') rankingMounted.value = true
}

// Error tab state
const errRows = ref<OpsErrorLog[]>([])
const errLoading = ref(false)
const errPage = ref(1)
const errPageSize = ref(20)
const errTotal = ref(0)
const errSortBy = ref('created_at')
const errSortOrder = ref<'asc' | 'desc'>('desc')
const showErrorModal = ref(false)
const selectedErrorId = ref<number | null>(null)

// 注意：'YYYY-MM-DDT00:00:00' 无时区后缀，按本地时区解析后再转 UTC——与页面其它日期处理语义一致，刻意如此，勿改成 'T00:00:00Z'
const toRFC3339 = (d: string | undefined, endOfDay = false): string | undefined =>
  d ? new Date(d + (endOfDay ? 'T23:59:59.999' : 'T00:00:00')).toISOString() : undefined

const loadAdminErrors = async () => {
  errLoading.value = true
  try {
    const resp = await listErrorLogs({
      page: errPage.value,
      page_size: errPageSize.value,
      view: 'all',
      start_time: toRFC3339(filters.value.start_date),
      end_time: toRFC3339(filters.value.end_date, true),
      user_id: filters.value.user_id ?? undefined,
      api_key_id: filters.value.api_key_id ?? undefined,
      account_id: filters.value.account_id ?? undefined,
      group_id: filters.value.group_id ?? undefined,
      model: filters.value.model || undefined,
      phase: filters.value.error_phase || undefined,
      category: filters.value.error_category || undefined,
      status_codes: filters.value.status_code != null ? String(filters.value.status_code) : undefined,
      sort_by: errSortBy.value,
      sort_order: errSortOrder.value,
    })
    errRows.value = resp.items
    errTotal.value = resp.total
  } catch (error) {
    console.error('Failed to load admin errors:', error)
    appStore.showError(t('usage.errors.failedToLoad'))
  } finally {
    errLoading.value = false
  }
}

const onErrSort = (sortBy: string, sortOrder: 'asc' | 'desc') => {
  errSortBy.value = sortBy
  errSortOrder.value = sortOrder
  errPage.value = 1
  loadAdminErrors()
}
const onErrPage = (p: number) => { errPage.value = p; loadAdminErrors() }
const onErrPageSize = (s: number) => { errPageSize.value = s; errPage.value = 1; loadAdminErrors() }
const openError = (id: number) => { selectedErrorId.value = id; showErrorModal.value = true }

const showColumnDropdown = ref(false)
const columnDropdownRef = ref<HTMLElement | null>(null)
const columnDropdownMenuRef = ref<HTMLElement | null>(null)
const columnDropdownPosition = reactive({ top: 0, left: 0 })

const updateColumnDropdownPosition = () => {
  if (!showColumnDropdown.value) return
  const trigger = columnDropdownRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const width = 192
  const margin = 8
  const menuHeight = Math.min(columnDropdownMenuRef.value?.offsetHeight || 320, window.innerHeight - margin * 2)
  let top = rect.bottom + margin
  if (top + menuHeight > window.innerHeight - margin) {
    top = Math.max(margin, rect.top - margin - menuHeight)
  }
  columnDropdownPosition.top = top
  columnDropdownPosition.left = Math.max(margin, Math.min(rect.right - width, window.innerWidth - width - margin))
}

let columnDropdownListenersActive = false

const addColumnDropdownListeners = () => {
  if (columnDropdownListenersActive) return
  columnDropdownListenersActive = true
  document.addEventListener('click', handleColumnClickOutside)
  window.addEventListener('resize', updateColumnDropdownPosition)
  window.addEventListener('scroll', updateColumnDropdownPosition, true)
}

const removeColumnDropdownListeners = () => {
  if (!columnDropdownListenersActive) return
  columnDropdownListenersActive = false
  document.removeEventListener('click', handleColumnClickOutside)
  window.removeEventListener('resize', updateColumnDropdownPosition)
  window.removeEventListener('scroll', updateColumnDropdownPosition, true)
}

const toggleColumnDropdown = async () => {
  showColumnDropdown.value = !showColumnDropdown.value
  if (showColumnDropdown.value) {
    updateColumnDropdownPosition()
    await nextTick()
    updateColumnDropdownPosition()
  }
}

const handleColumnClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (
    columnDropdownRef.value &&
    !columnDropdownRef.value.contains(target) &&
    !columnDropdownMenuRef.value?.contains(target)
  ) {
    showColumnDropdown.value = false
  }
}

onMounted(() => {
  applyRouteQueryFilters()
  void resolveProfileContext()
  loadLogs()
  loadStats()
  loadModelStats(modelDistributionSource.value, true)
  window.setTimeout(() => {
    void loadChartData()
  }, 120)
  loadSavedColumns()
  loadSavedErrColumns()
})
onUnmounted(() => {
  abortController?.abort()
  exportAbortController?.abort()
  removeColumnDropdownListeners()
})

watch(showColumnDropdown, async (open) => {
  if (open) {
    addColumnDropdownListeners()
    await nextTick()
    updateColumnDropdownPosition()
    return
  }
  removeColumnDropdownListeners()
})

watch(modelDistributionSource, (source) => {
  void loadModelStats(source)
})

defineExpose({ requestedModelStats, refreshData })
</script>

<style scoped>
.usage-header-control :deep(.date-picker-trigger),
.usage-header-control :deep(.select-trigger) {
  @apply h-11 w-full rounded-xl px-3;
}

.usage-header-control :deep(.date-picker-trigger) {
  @apply justify-between;
}

.usage-header-control :deep(.date-picker-value) {
  @apply min-w-0 flex-1 truncate text-left;
}

.usage-chart-modal-body {
  max-height: min(72vh, 44rem);
  overflow: auto;
}

.usage-chart-modal-body :deep(.usage-expanded-chart.card) {
  border: 0;
  background: transparent;
  box-shadow: none;
  padding: 0;
}

.usage-chart-modal-body :deep(.usage-expanded-chart .chart-table-scroll) {
  max-height: min(30rem, 58vh);
}

.usage-chart-modal-body :deep(.usage-expanded-chart .chart-doughnut-canvas) {
  height: clamp(14rem, 22vw, 18rem);
  width: clamp(14rem, 22vw, 18rem);
}

.usage-chart-modal-body :deep(.usage-expanded-chart .chart-line-canvas) {
  height: min(30rem, 58vh);
}

.usage-chart-modal-body :deep(.usage-expanded-chart table) {
  font-size: 0.875rem;
  line-height: 1.25rem;
}

.usage-chart-modal-body :deep(.usage-expanded-chart th) {
  padding-bottom: 0.625rem;
}

.usage-chart-modal-body :deep(.usage-expanded-chart td) {
  padding-bottom: 0.55rem;
  padding-top: 0.55rem;
}

.usage-chart-modal-body :deep(.usage-expanded-chart td[title]) {
  max-width: 18rem;
}
</style>
