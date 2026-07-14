<template>
  <section class="rounded-xl border border-stone-200/80 bg-white/80 p-4 shadow-sm backdrop-blur dark:border-white/10 dark:bg-stone-950/70">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <div class="flex items-center gap-2">
          <Icon name="trendingUp" size="md" class="text-primary-500" />
          <h2 class="text-base font-semibold text-stone-950 dark:text-white">
            {{ t('usage.analytics.title') }}
          </h2>
        </div>
        <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
          {{ scopeText }}
        </p>
      </div>
      <div class="flex flex-wrap items-end gap-2">
        <div v-if="enterprise" class="inline-flex h-10 rounded-lg border border-stone-200 bg-stone-50 p-1 dark:border-white/10 dark:bg-white/[0.04]">
          <button
            type="button"
            class="rounded-md px-3 text-xs font-medium transition-colors"
            :class="analyticsDimension === 'member' ? 'bg-white text-emerald-700 shadow-sm dark:bg-white/10 dark:text-emerald-300' : 'text-stone-500 dark:text-stone-400'"
            @click="setAnalyticsDimension('member')"
          >
            {{ t('usage.analytics.dimensionMember') }}
          </button>
          <button
            type="button"
            class="rounded-md px-3 text-xs font-medium transition-colors"
            :class="analyticsDimension === 'key' ? 'bg-white text-emerald-700 shadow-sm dark:bg-white/10 dark:text-emerald-300' : 'text-stone-500 dark:text-stone-400'"
            @click="setAnalyticsDimension('key')"
          >
            {{ t('usage.analytics.dimensionKey') }}
          </button>
        </div>
        <Select
          :model-value="granularity"
          :options="granularityOptions"
          class="w-32"
          @update:model-value="setGranularity"
        />
        <button
          type="button"
          class="btn btn-secondary h-10"
          :disabled="loading"
          @click="loadAnalytics"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          <span>{{ t('common.refresh') }}</span>
        </button>
      </div>
    </div>

    <div v-if="error" class="mt-3 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="mt-4 rounded-lg border border-stone-200/80 bg-stone-50/70 p-3 dark:border-white/10 dark:bg-white/[0.035]">
      <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
        <label v-if="enterprise">
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('usage.memberFilter') }}
          </span>
          <Select
            v-model="selectedAnalyticsMemberFilter"
            :options="memberFilterOptions"
            searchable="auto"
          />
        </label>
        <label v-if="analyticsDimension === 'key'" :class="enterprise ? '' : 'xl:col-span-2'">
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('usage.analytics.filters.search') }}
          </span>
          <input
            v-model="analyticsSearch"
            type="search"
            class="input h-10 w-full"
            :placeholder="t('usage.analytics.filters.searchPlaceholder')"
          />
        </label>
        <label>
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('usage.analytics.apiKey') }}
          </span>
          <Select
            v-model="selectedAnalyticsAPIKeyID"
            :options="apiKeyFilterOptions"
            searchable="auto"
          />
        </label>
        <label>
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('keys.group') }}
          </span>
          <Select
            v-model="selectedAnalyticsGroupID"
            :options="groupFilterOptions"
            searchable="auto"
          />
        </label>
        <label v-if="analyticsDimension === 'key'">
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('keys.tags') }}
          </span>
          <Select
            v-model="selectedAnalyticsTag"
            :options="tagFilterOptions"
            searchable="auto"
          />
        </label>
        <label v-if="analyticsDimension === 'key'">
          <span class="mb-1.5 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('common.status') }}
          </span>
          <Select
            v-model="selectedAnalyticsStatus"
            :options="statusFilterOptions"
          />
        </label>
      </div>
      <div class="mt-3 flex flex-wrap items-center justify-between gap-2">
        <div class="flex flex-wrap items-center gap-2">
          <label class="flex items-center gap-2 text-xs font-medium text-stone-500 dark:text-stone-400">
            <span>{{ t('usage.analytics.filters.limit') }}</span>
            <Select
              v-model="analyticsLimit"
              :options="limitOptions"
              class="w-24"
            />
          </label>
          <button
            v-if="hasAnalyticsFilters"
            type="button"
            class="btn btn-secondary h-9"
            @click="resetAnalyticsFilters"
          >
            {{ t('common.reset') }}
          </button>
        </div>
        <button
          type="button"
          class="btn btn-secondary h-9"
          :disabled="loading"
          @click="exportAnalyticsCSV"
        >
          <Icon name="download" size="sm" />
          <span>{{ t('usage.exportCsv') }}</span>
        </button>
      </div>
    </div>

    <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <div v-for="metric in summaryMetrics" :key="metric.key" class="rounded-lg border border-stone-200/80 bg-stone-50/70 px-3 py-2 dark:border-white/10 dark:bg-white/[0.035]">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ metric.label }}</div>
        <div class="mt-1 text-lg font-semibold tabular-nums text-stone-950 dark:text-white" :title="metric.title">
          {{ metric.value }}
        </div>
      </div>
    </div>

    <div v-if="analyticsDimension === 'key'" class="mt-3 grid gap-3 md:grid-cols-3">
      <div class="rounded-lg border border-stone-200/80 bg-white/60 px-3 py-2 dark:border-white/10 dark:bg-black/20">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.analytics.activeKeysNow') }}</div>
        <div class="mt-1 font-semibold tabular-nums text-stone-950 dark:text-white">{{ formatInteger(snapshot.active_key_count) }}</div>
      </div>
      <div class="rounded-lg border border-stone-200/80 bg-white/60 px-3 py-2 dark:border-white/10 dark:bg-black/20">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.analytics.nearQuotaKeys') }}</div>
        <div class="mt-1 font-semibold tabular-nums text-amber-600 dark:text-amber-300">{{ formatInteger(snapshot.near_quota_key_count) }}</div>
      </div>
      <div class="rounded-lg border border-stone-200/80 bg-white/60 px-3 py-2 dark:border-white/10 dark:bg-black/20">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.analytics.nearRateLimitKeys') }}</div>
        <div class="mt-1 flex flex-wrap items-baseline gap-2">
          <span class="font-semibold tabular-nums text-amber-600 dark:text-amber-300">{{ formatInteger(snapshot.near_rate_limit_key_count) }}</span>
          <span v-if="snapshot.snapshot_at" class="text-[11px] text-stone-400 dark:text-stone-500">
            {{ formatSnapshotTime(snapshot.snapshot_at) }}
          </span>
        </div>
      </div>
    </div>
    <div v-else class="mt-3 grid gap-3 md:grid-cols-3">
      <div class="rounded-lg border border-stone-200/80 bg-white/60 px-3 py-2 dark:border-white/10 dark:bg-black/20">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.analytics.memberCount') }}</div>
        <div class="mt-1 font-semibold tabular-nums text-stone-950 dark:text-white">{{ formatInteger(memberCount) }}</div>
      </div>
      <div class="rounded-lg border border-stone-200/80 bg-white/60 px-3 py-2 dark:border-white/10 dark:bg-black/20">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.analytics.budgetRiskMembers') }}</div>
        <div class="mt-1 font-semibold tabular-nums text-amber-600 dark:text-amber-300">{{ formatInteger(memberBudgetRiskCount) }}</div>
      </div>
      <div class="rounded-lg border border-stone-200/80 bg-white/60 px-3 py-2 dark:border-white/10 dark:bg-black/20">
        <div class="text-xs text-stone-500 dark:text-stone-400">{{ t('usage.analytics.reservedBudget') }}</div>
        <div class="mt-1 font-semibold tabular-nums text-stone-950 dark:text-white">{{ formatMoney(memberReservedTotal) }}</div>
      </div>
    </div>

    <div class="mt-4 border-b border-stone-200/80 dark:border-white/10">
      <div class="flex gap-1 overflow-x-auto">
        <button
          v-for="tab in tabs"
          :key="tab.value"
          type="button"
          :class="tabButtonClass(activeTab === tab.value)"
          @click="activeTab = tab.value"
        >
          <Icon :name="tab.icon" size="sm" />
          <span>{{ tab.label }}</span>
        </button>
      </div>
    </div>

    <div class="mt-4 min-h-[220px]">
      <div v-if="loading" class="flex h-52 items-center justify-center text-sm text-stone-500 dark:text-stone-400">
        {{ t('common.loading') }}
      </div>
      <template v-else>
        <div v-if="activeTab === 'trend'" class="space-y-3">
          <div v-if="trendChartData" class="h-72 rounded-lg border border-stone-200/80 p-3 dark:border-white/10">
            <Bar :data="trendChartData" :options="trendChartOptions" />
          </div>
          <OwnerTrendTable :items="trendItems" />
        </div>

        <div v-else-if="activeTab === 'leaderboard'" class="space-y-3">
          <div v-if="effectiveLeaderboardChartData" class="rounded-lg border border-stone-200/80 p-3 dark:border-white/10" :style="{ height: rankedChartHeight(effectiveLeaderboardCount) }">
            <Bar :data="effectiveLeaderboardChartData" :options="effectiveLeaderboardChartOptions" />
          </div>
          <OwnerLeaderboardTable
            v-if="analyticsDimension === 'key'"
            :items="leaderboardItems"
            :max-cost="leaderboardMaxCost"
            :on-select="selectAnalyticsAPIKey"
          />
          <OwnerMemberLeaderboardTable
            v-else
            :items="memberLeaderboardItems"
            :max-cost="memberLeaderboardMaxCost"
            :on-select="selectAnalyticsMember"
          />
        </div>

        <div v-else-if="activeTab === 'models'" class="space-y-3">
          <div v-if="modelChartData" class="rounded-lg border border-stone-200/80 p-3 dark:border-white/10" :style="{ height: rankedChartHeight(modelItems.length) }">
            <Bar :data="modelChartData" :options="modelChartOptions" />
          </div>
          <OwnerModelTable
            :items="modelItems"
            :max-cost="modelMaxCost"
          />
        </div>

        <div v-else-if="activeTab === 'groups'" class="space-y-3">
          <div v-if="groupChartData" class="rounded-lg border border-stone-200/80 p-3 dark:border-white/10" :style="{ height: rankedChartHeight(groupItems.length) }">
            <Bar :data="groupChartData" :options="groupChartOptions" />
          </div>
          <OwnerGroupTable
            :items="groupItems"
            :max-cost="groupMaxCost"
            :on-select="selectAnalyticsGroup"
          />
        </div>

        <div v-else class="space-y-3">
          <div v-if="tagChartData" class="rounded-lg border border-stone-200/80 p-3 dark:border-white/10" :style="{ height: rankedChartHeight(tagItems.length) }">
            <Bar :data="tagChartData" :options="tagChartOptions" />
          </div>
          <OwnerTagTable
            :items="tagItems"
            :max-cost="tagMaxCost"
            :on-select="selectAnalyticsTag"
          />
        </div>
      </template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, onUnmounted, ref, watch, type PropType } from 'vue'
import { useI18n } from 'vue-i18n'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js'
import type { ActiveElement, ChartData, ChartEvent, ChartOptions, TooltipItem } from 'chart.js'
import { Bar } from 'vue-chartjs'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { usageAPI } from '@/api'
import { serializeCSV } from '@/utils/csv'
import { formatCompactNumber } from '@/utils/format'
import type { ApiKey, Group } from '@/types'
import type {
  ApiKeyUsageTrendGranularity,
  OwnerApiKeyAnalyticsParams,
  OwnerApiKeyAnalyticsSnapshot,
  OwnerApiKeyAnalyticsSummary,
  OwnerApiKeyLeaderboardItem,
  OwnerMemberLeaderboardItem,
  OwnerMemberLeaderboardResponse,
  OwnerUsageMember,
  OwnerGroupAnalyticsItem,
  OwnerModelAnalyticsItem,
  OwnerTagAnalyticsItem,
  OwnerTrendAnalyticsPoint
} from '@/api/usage'

ChartJS.register(BarElement, CategoryScale, LinearScale, Tooltip, Legend)

type AnalyticsTab = 'leaderboard' | 'trend' | 'models' | 'groups' | 'tags'
type AnalyticsDimension = 'member' | 'key'
type IconName = InstanceType<typeof Icon>['$props']['name']
type AnalyticsStatus = NonNullable<OwnerApiKeyAnalyticsParams['status']>

const props = withDefaults(defineProps<{
  apiKeyId?: number | null
  startDate: string
  endDate: string
  startTime?: string
  endTime?: string
  apiKeys?: ApiKey[]
  groups?: Group[]
  enterprise?: boolean
  members?: OwnerUsageMember[]
  memberFilter?: string
}>(), {
  startTime: '',
  endTime: '',
  apiKeys: () => [],
  groups: () => [],
  enterprise: false,
  members: () => [],
  memberFilter: 'all'
})

const emit = defineEmits<{
  (event: 'update:member-filter', value: string): void
}>()

const { t } = useI18n()

const activeTab = ref<AnalyticsTab>('leaderboard')
const analyticsDimension = ref<AnalyticsDimension>(props.enterprise ? 'member' : 'key')
const granularity = ref<ApiKeyUsageTrendGranularity>('day')
const loading = ref(false)
const error = ref('')
const summary = ref<OwnerApiKeyAnalyticsSummary | null>(null)
const leaderboardItems = ref<OwnerApiKeyLeaderboardItem[]>([])
const memberLeaderboardItems = ref<OwnerMemberLeaderboardItem[]>([])
const memberCount = ref(0)
const memberBudgetRiskCount = ref(0)
const memberReservedTotal = ref(0)
const modelItems = ref<OwnerModelAnalyticsItem[]>([])
const groupItems = ref<OwnerGroupAnalyticsItem[]>([])
const tagItems = ref<OwnerTagAnalyticsItem[]>([])
const trendItems = ref<OwnerTrendAnalyticsPoint[]>([])
const selectedAnalyticsAPIKeyID = ref<number | null>(props.apiKeyId || null)
const selectedAnalyticsMemberFilter = ref(props.memberFilter)
const selectedAnalyticsGroupID = ref<number | null>(null)
const selectedAnalyticsTag = ref<string | null>(null)
const selectedAnalyticsStatus = ref<AnalyticsStatus | null>(null)
const analyticsSearch = ref('')
const analyticsLimit = ref(20)
// --- Precise time (to the second) bounds, supplied by the parent via DateRangePicker ---
// Empty string = that bound has no time → backend falls back to date-only behavior.
const preciseTimeParams = computed<{ start_time?: string; end_time?: string }>(() => {
  const p: { start_time?: string; end_time?: string } = {}
  if (props.startTime) p.start_time = props.startTime
  if (props.endTime) p.end_time = props.endTime
  return p
})
let abortController: AbortController | null = null
let latestRequestID = 0
let reloadTimer: ReturnType<typeof setTimeout> | null = null

const emptySnapshot: OwnerApiKeyAnalyticsSnapshot = {
  active_key_count: 0,
  near_quota_key_count: 0,
  near_rate_limit_key_count: 0,
  snapshot_at: ''
}

const tabs = computed<Array<{ value: AnalyticsTab; label: string; icon: IconName }>>(() => {
  const items: Array<{ value: AnalyticsTab; label: string; icon: IconName }> = [
    {
      value: 'leaderboard',
      label: analyticsDimension.value === 'member'
        ? t('usage.analytics.tabs.memberLeaderboard')
        : t('usage.analytics.tabs.leaderboard'),
      icon: 'users'
    },
    { value: 'trend', label: t('usage.analytics.tabs.trend'), icon: 'trendingUp' },
    { value: 'models', label: t('usage.analytics.tabs.models'), icon: 'database' },
    { value: 'groups', label: t('usage.analytics.tabs.groups'), icon: 'grid' }
  ]
  if (analyticsDimension.value === 'key') {
    items.push({ value: 'tags', label: t('usage.analytics.tabs.tags'), icon: 'filter' })
  }
  return items
})

const granularityOptions = computed(() => [
  { value: 'hour', label: t('keys.usageDetails.granularity.hour') },
  { value: 'day', label: t('keys.usageDetails.granularity.day') },
  { value: 'week', label: t('keys.usageDetails.granularity.week') },
  { value: 'month', label: t('keys.usageDetails.granularity.month') }
])
const memberFilterOptions = computed(() => [
  { value: 'all', label: t('usage.members.all') },
  { value: 'assigned', label: t('usage.members.assigned') },
  { value: 'unassigned', label: t('usage.members.unassigned') },
  ...props.members.map((member) => ({
    value: `member:${member.id}`,
    label: member.archived
      ? t('usage.members.optionArchived', { name: member.name, code: member.member_code })
      : t('usage.members.option', { name: member.name, code: member.member_code })
  }))
])
const analyticsAPIKeys = computed(() => {
  const selected = selectedAnalyticsMemberFilter.value
  if (!props.enterprise || selected === 'all') return props.apiKeys
  if (selected === 'assigned') return props.apiKeys.filter((key) => key.member_id != null)
  if (selected === 'unassigned') return props.apiKeys.filter((key) => key.member_id == null)
  if (selected.startsWith('member:')) {
    const memberID = Number(selected.slice('member:'.length))
    return props.apiKeys.filter((key) => key.member_id === memberID)
  }
  return props.apiKeys
})
const apiKeyFilterOptions = computed(() => [
  { value: null, label: t('usage.allApiKeys') },
  ...analyticsAPIKeys.value.map((key) => ({
    value: key.id,
    label: key.name || `#${key.id}`
  }))
])
const groupFilterOptions = computed(() => {
  const groups = new Map<number, string>()
  for (const group of props.groups) {
    groups.set(group.id, group.name || `#${group.id}`)
  }
  for (const key of props.apiKeys) {
    if (typeof key.group_id === 'number' && key.group_id > 0) {
      groups.set(key.group_id, key.group?.name || `#${key.group_id}`)
    }
  }
  return [
    { value: null, label: t('usage.analytics.filters.allGroups') },
    { value: 0, label: t('keys.noGroup') },
    ...Array.from(groups.entries())
      .sort(([, a], [, b]) => a.localeCompare(b))
      .map(([value, label]) => ({ value, label }))
  ]
})
const tagFilterOptions = computed(() => {
  const tags = new Set<string>()
  for (const key of props.apiKeys) {
    for (const tag of key.tags || []) {
      const normalized = tag.trim()
      if (normalized) tags.add(normalized)
    }
  }
  return [
    { value: null, label: t('usage.analytics.filters.allTags') },
    ...Array.from(tags)
      .sort((a, b) => a.localeCompare(b))
      .map((tag) => ({ value: tag, label: tag }))
  ]
})
const statusFilterOptions = computed(() => [
  { value: null, label: t('usage.analytics.filters.allStatuses') },
  { value: 'active', label: t('keys.status.active') },
  { value: 'disabled', label: t('keys.status.disabled') },
  { value: 'quota_exhausted', label: t('keys.status.quota_exhausted') },
  { value: 'expired', label: t('keys.status.expired') }
])
const limitOptions = computed(() => [
  { value: 10, label: '10' },
  { value: 20, label: '20' },
  { value: 50, label: '50' },
  { value: 100, label: '100' }
])

const snapshot = computed(() => summary.value?.current_key_snapshot || emptySnapshot)
const scopeText = computed(() =>
  analyticsDimension.value === 'member'
    ? t('usage.analytics.memberScope')
    : props.apiKeyId
    ? t('usage.analytics.singleKeyScope')
    : t('usage.analytics.scope')
)
const summaryMetrics = computed(() => [
  {
    key: 'actual_cost',
    label: t('usage.actualCost'),
    value: formatMoney(summary.value?.actual_cost || 0),
    title: formatMoney(summary.value?.actual_cost || 0)
  },
  {
    key: 'requests',
    label: t('usage.totalRequests'),
    value: formatCompactNumber(summary.value?.requests || 0),
    title: formatInteger(summary.value?.requests || 0)
  },
  {
    key: 'tokens',
    label: t('usage.totalTokens'),
    value: formatCompactNumber(summary.value?.total_tokens || 0),
    title: formatInteger(summary.value?.total_tokens || 0)
  },
  {
    key: 'used_keys',
    label: t('usage.analytics.usedKeys'),
    value: formatInteger(summary.value?.used_key_count || 0),
    title: formatInteger(summary.value?.used_key_count || 0)
  }
])
const hasAnalyticsFilters = computed(() =>
  selectedAnalyticsMemberFilter.value !== props.memberFilter ||
  selectedAnalyticsAPIKeyID.value !== (props.apiKeyId || null) ||
  selectedAnalyticsGroupID.value !== null ||
  selectedAnalyticsTag.value !== null ||
  selectedAnalyticsStatus.value !== null ||
  analyticsSearch.value.trim() !== '' ||
  analyticsLimit.value !== 20
)

const leaderboardMaxCost = computed(() => maxCost(leaderboardItems.value))
const memberLeaderboardMaxCost = computed(() => maxCost(memberLeaderboardItems.value))
const modelMaxCost = computed(() => maxCost(modelItems.value))
const groupMaxCost = computed(() => maxCost(groupItems.value))
const tagMaxCost = computed(() => maxCost(tagItems.value))
const isDarkMode = computed(() => typeof document !== 'undefined' && document.documentElement.classList.contains('dark'))
const chartTheme = computed(() => ({
  grid: isDarkMode.value ? 'rgba(255,255,255,0.10)' : 'rgba(120,113,108,0.18)',
  text: isDarkMode.value ? '#d6d3d1' : '#57534e',
  mutedText: isDarkMode.value ? '#a8a29e' : '#78716c',
  tooltipBg: isDarkMode.value ? 'rgba(12,12,12,0.96)' : 'rgba(255,255,255,0.96)',
  tooltipBorder: isDarkMode.value ? 'rgba(255,255,255,0.14)' : 'rgba(120,113,108,0.22)',
  trendBar: isDarkMode.value ? 'rgba(94, 234, 212, 0.72)' : 'rgba(20, 184, 166, 0.72)',
  trendBorder: isDarkMode.value ? '#5eead4' : '#0f766e'
}))
const trendChartData = computed<ChartData<'bar'> | null>(() => {
  if (trendItems.value.length === 0) return null
  const colors = chartTheme.value

  return {
    labels: trendItems.value.map((item) => formatBucket(item.date)),
    datasets: [
      {
        label: t('usage.actualCost'),
        data: trendItems.value.map((item) => item.actual_cost || 0),
        backgroundColor: colors.trendBar,
        borderColor: colors.trendBorder,
        borderRadius: 4,
        borderSkipped: false,
        borderWidth: 1,
        maxBarThickness: 46,
        categoryPercentage: trendItems.value.length <= 2 ? 0.72 : 0.82,
        barPercentage: trendItems.value.length <= 2 ? 0.38 : 0.62
      }
    ]
  }
})
const trendChartOptions = computed<ChartOptions<'bar'>>(() => {
  const colors = chartTheme.value

  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      mode: 'index',
      intersect: false
    },
    layout: {
      padding: { top: 6, right: 8, bottom: 0, left: 0 }
    },
    plugins: {
      legend: { display: false },
      tooltip: {
        displayColors: false,
        backgroundColor: colors.tooltipBg,
        borderColor: colors.tooltipBorder,
        borderWidth: 1,
        titleColor: colors.text,
        bodyColor: colors.text,
        padding: 10,
        callbacks: {
          title: (items: TooltipItem<'bar'>[]) => items[0]?.label || '',
          label: (context: TooltipItem<'bar'>) => `${t('usage.actualCost')}: ${formatMoney(Number(context.parsed.y || 0))}`,
          afterLabel: (context: TooltipItem<'bar'>) => {
            const item = trendItems.value[context.dataIndex]
            if (!item) return []
            return [
              `${t('usage.totalRequests')}: ${formatInteger(item.requests)}`,
              `${t('usage.totalTokens')}: ${formatCompactNumber(item.total_tokens)}`
            ]
          }
        }
      }
    },
    scales: {
      x: {
        grid: { display: false },
        border: { color: colors.grid },
        ticks: {
          color: colors.mutedText,
          autoSkip: trendItems.value.length > 12,
          maxRotation: 0,
          minRotation: 0,
          font: { size: 11 }
        }
      },
      y: {
        beginAtZero: true,
        border: { display: false },
        grid: {
          color: colors.grid,
          tickLength: 0
        },
        ticks: {
          color: colors.mutedText,
          padding: 8,
          font: { size: 11 },
          callback: (value: string | number) => formatMoney(Number(value))
        }
      }
    }
  }
})
const leaderboardChartData = computed<ChartData<'bar'> | null>(() => makeRankedChartData(
  leaderboardItems.value,
  (item) => item.key_name || `#${item.api_key_id}`,
  'teal'
))
const leaderboardChartOptions = computed<ChartOptions<'bar'>>(() => makeRankedChartOptions(
  leaderboardItems.value.map((item) => item.key_name || `#${item.api_key_id}`),
  (index) => {
    const item = leaderboardItems.value[index]
    if (!item) return []
    return [
      `${t('usage.actualCost')}: ${formatMoney(item.actual_cost)}`,
      `${t('usage.analytics.share')}: ${formatPercent(item.share_percent)}`,
      `${t('usage.totalRequests')}: ${formatInteger(item.requests)}`,
      `${t('usage.totalTokens')}: ${formatCompactNumber(item.total_tokens)}`
    ]
  },
  (index) => {
    const item = leaderboardItems.value[index]
    if (item) selectAnalyticsAPIKey(item)
  }
))
const memberLeaderboardChartData = computed<ChartData<'bar'> | null>(() => makeRankedChartData(
  memberLeaderboardItems.value,
  memberLeaderboardLabel,
  'teal'
))
const memberLeaderboardChartOptions = computed<ChartOptions<'bar'>>(() => makeRankedChartOptions(
  memberLeaderboardItems.value.map(memberLeaderboardLabel),
  (index) => {
    const item = memberLeaderboardItems.value[index]
    if (!item) return []
    return [
      `${t('usage.actualCost')}: ${formatMoney(item.actual_cost)}`,
      `${t('usage.analytics.share')}: ${formatPercent(item.share_percent)}`,
      `${t('usage.analytics.budgetUsed')}: ${formatMoney(item.current_used_usd)}`,
      `${t('usage.analytics.reservedBudget')}: ${formatMoney(item.current_reserved_usd)}`,
      `${t('usage.totalRequests')}: ${formatInteger(item.requests)}`
    ]
  },
  (index) => {
    const item = memberLeaderboardItems.value[index]
    if (item) selectAnalyticsMember(item)
  }
))
const effectiveLeaderboardChartData = computed(() =>
  analyticsDimension.value === 'member' ? memberLeaderboardChartData.value : leaderboardChartData.value
)
const effectiveLeaderboardChartOptions = computed(() =>
  analyticsDimension.value === 'member' ? memberLeaderboardChartOptions.value : leaderboardChartOptions.value
)
const effectiveLeaderboardCount = computed(() =>
  analyticsDimension.value === 'member' ? memberLeaderboardItems.value.length : leaderboardItems.value.length
)
const modelChartData = computed<ChartData<'bar'> | null>(() => makeRankedChartData(
  modelItems.value,
  (item) => item.model || 'unknown',
  'blue'
))
const modelChartOptions = computed<ChartOptions<'bar'>>(() => makeRankedChartOptions(
  modelItems.value.map((item) => item.model || 'unknown'),
  (index) => {
    const item = modelItems.value[index]
    if (!item) return []
    return [
      `${t('usage.actualCost')}: ${formatMoney(item.actual_cost)}`,
      `${t('usage.totalRequests')}: ${formatInteger(item.requests)}`,
      `${t('usage.totalTokens')}: ${formatCompactNumber(item.total_tokens)}`
    ]
  }
))
const groupChartData = computed<ChartData<'bar'> | null>(() => makeRankedChartData(
  groupItems.value,
  (item) => item.group_name || t('keys.noGroup'),
  'amber'
))
const groupChartOptions = computed<ChartOptions<'bar'>>(() => makeRankedChartOptions(
  groupItems.value.map((item) => item.group_name || t('keys.noGroup')),
  (index) => {
    const item = groupItems.value[index]
    if (!item) return []
    return [
      `${t('usage.actualCost')}: ${formatMoney(item.actual_cost)}`,
      `${t('usage.analytics.keyCount')}: ${formatInteger(item.key_count)}`,
      `${t('usage.analytics.share')}: ${formatPercent(item.share_percent)}`,
      `${t('usage.totalTokens')}: ${formatCompactNumber(item.total_tokens)}`
    ]
  },
  (index) => {
    const item = groupItems.value[index]
    if (item) selectAnalyticsGroup(item)
  }
))
const tagChartData = computed<ChartData<'bar'> | null>(() => makeRankedChartData(
  tagItems.value,
  (item) => item.tag,
  'rose'
))
const tagChartOptions = computed<ChartOptions<'bar'>>(() => makeRankedChartOptions(
  tagItems.value.map((item) => item.tag),
  (index) => {
    const item = tagItems.value[index]
    if (!item) return []
    return [
      `${t('usage.actualCost')}: ${formatMoney(item.actual_cost)}`,
      `${t('usage.analytics.keyCount')}: ${formatInteger(item.key_count)}`,
      `${t('usage.totalRequests')}: ${formatInteger(item.requests)}`,
      `${t('usage.totalTokens')}: ${formatCompactNumber(item.total_tokens)}`
    ]
  },
  (index) => {
    const item = tagItems.value[index]
    if (item) selectAnalyticsTag(item)
  }
))

const requestSignature = computed(() => JSON.stringify({
  tab: activeTab.value,
  dimension: analyticsDimension.value,
  memberFilter: selectedAnalyticsMemberFilter.value,
  granularity: granularity.value,
  apiKeyId: props.apiKeyId || null,
  analyticsApiKeyId: selectedAnalyticsAPIKeyID.value,
  groupId: selectedAnalyticsGroupID.value,
  tag: selectedAnalyticsTag.value,
  status: selectedAnalyticsStatus.value,
  search: analyticsSearch.value.trim(),
  limit: analyticsLimit.value,
  start: props.startDate,
  end: props.endDate,
  preciseStart: props.startTime || '',
  preciseEnd: props.endTime || ''
}))

function timezoneName() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

function buildParams(): OwnerApiKeyAnalyticsParams {
  const params: OwnerApiKeyAnalyticsParams = {
    granularity: granularity.value,
    start_date: props.startDate,
    end_date: props.endDate,
    ...preciseTimeParams.value,
    timezone: timezoneName(),
    limit: analyticsLimit.value
  }
  if (selectedAnalyticsAPIKeyID.value) {
    params.api_key_id = selectedAnalyticsAPIKeyID.value
  }
  if (props.enterprise) {
    const selectedMember = selectedAnalyticsMemberFilter.value
    if (selectedMember.startsWith('member:')) {
      const memberID = Number(selectedMember.slice('member:'.length))
      if (Number.isFinite(memberID) && memberID > 0) params.member_id = memberID
    } else if (selectedMember === 'assigned' || selectedMember === 'unassigned') {
      params.member_scope = selectedMember
    } else if (analyticsDimension.value === 'member') {
      params.member_scope = 'all'
    }
  }
  if (selectedAnalyticsGroupID.value !== null) {
    params.group_id = selectedAnalyticsGroupID.value
  }
  if (analyticsDimension.value === 'key' && selectedAnalyticsTag.value) {
    params.tags = selectedAnalyticsTag.value
  }
  if (analyticsDimension.value === 'key' && selectedAnalyticsStatus.value) {
    params.status = selectedAnalyticsStatus.value
  }
  const search = analyticsSearch.value.trim()
  if (analyticsDimension.value === 'key' && search) {
    params.search = search
  }
  return params
}

async function loadAnalytics() {
  if (abortController) abortController.abort()
  const controller = new AbortController()
  abortController = controller
  const requestID = ++latestRequestID
  const params = buildParams()
  loading.value = true
  error.value = ''

  try {
    const memberMetricsRequest = analyticsDimension.value === 'member' && activeTab.value !== 'leaderboard'
      ? usageAPI.getOwnerMemberAnalyticsLeaderboard({ ...params, limit: 1 }, { signal: controller.signal })
      : Promise.resolve(null)
    const [summaryResult, tabResult, memberMetricsResult] = await Promise.all([
      usageAPI.getOwnerApiKeyAnalyticsSummary(params, { signal: controller.signal }),
      loadActiveTab(params, controller.signal),
      memberMetricsRequest
    ])
    if (requestID !== latestRequestID || controller.signal.aborted) return
    summary.value = summaryResult.summary
    applyTabResult(tabResult)
    if (memberMetricsResult) applyMemberMetrics(memberMetricsResult)
  } catch (err) {
    if (requestID !== latestRequestID || controller.signal.aborted || isAbortError(err)) return
    console.error('[UsageAnalyticsPanel] Failed to load usage analytics:', err)
    error.value = formatAnalyticsLoadError(err)
  } finally {
    if (requestID === latestRequestID) {
      loading.value = false
    }
  }
}

function loadActiveTab(params: OwnerApiKeyAnalyticsParams, signal: AbortSignal) {
  switch (activeTab.value) {
    case 'trend':
      return usageAPI.getOwnerApiKeyUsageTrend(params, { signal })
    case 'models':
      return usageAPI.getOwnerApiKeyModelAnalytics(params, { signal })
    case 'groups':
      return usageAPI.getOwnerApiKeyGroupAnalytics(params, { signal })
    case 'tags':
      return usageAPI.getOwnerApiKeyTagAnalytics(params, { signal })
    default:
      return analyticsDimension.value === 'member'
        ? usageAPI.getOwnerMemberAnalyticsLeaderboard(params, { signal })
        : usageAPI.getOwnerApiKeyAnalyticsLeaderboard(params, { signal })
  }
}

function applyTabResult(result: Awaited<ReturnType<typeof loadActiveTab>>) {
  if ('items' in result) {
    if (activeTab.value === 'trend') {
      trendItems.value = result.items as OwnerTrendAnalyticsPoint[]
    } else if (analyticsDimension.value === 'member') {
      memberLeaderboardItems.value = (result.items as OwnerMemberLeaderboardItem[]).filter((item) => item.member_id != null)
      applyMemberMetrics(result as OwnerMemberLeaderboardResponse)
    } else {
      leaderboardItems.value = result.items as OwnerApiKeyLeaderboardItem[]
    }
  } else if ('models' in result) {
    modelItems.value = result.models
  } else if ('groups' in result) {
    groupItems.value = result.groups
  } else if ('tags' in result) {
    tagItems.value = result.tags
  }
}

function applyMemberMetrics(result: OwnerMemberLeaderboardResponse) {
  memberCount.value = result.member_count
  memberBudgetRiskCount.value = result.budget_risk_member_count
  memberReservedTotal.value = result.total_reserved_usd
}

function scheduleLoad() {
  if (reloadTimer) clearTimeout(reloadTimer)
  reloadTimer = setTimeout(() => {
    reloadTimer = null
    loadAnalytics()
  }, 250)
}

function setGranularity(value: string | number | boolean | null) {
  if (value === 'hour' || value === 'day' || value === 'week' || value === 'month') {
    granularity.value = value
  }
}

function isAbortError(err: unknown) {
  if (!err || typeof err !== 'object') return false
  const { name, code } = err as { name?: string; code?: string }
  return name === 'AbortError' || name === 'CanceledError' || code === 'ERR_CANCELED'
}

function formatAnalyticsLoadError(err: unknown) {
  const base = t('usage.analytics.loadFailed')
  if (!err || typeof err !== 'object') return base

  const apiErr = err as { status?: number | string; message?: string }
  const status = Number(apiErr.status)
  if (status === 0) {
    return `${base}: ${t('usage.analytics.errors.network')}`
  }
  if (status === 404) {
    return `${base}: ${t('usage.analytics.errors.endpointMissing')}`
  }
  if (Number.isFinite(status) && status >= 500) {
    return `${base}: ${t('usage.analytics.errors.server', { status })}`
  }

  const message = typeof apiErr.message === 'string' ? apiErr.message.trim() : ''
  if (message) {
    return `${base}: ${message}`
  }
  return base
}

function formatInteger(value: number) {
  return new Intl.NumberFormat().format(value || 0)
}

function formatMoney(value: number) {
  return `$${(value || 0).toFixed(4)}`
}

function formatPercent(value: number) {
  return `${(value || 0).toFixed(1)}%`
}

function formatChange(value: number) {
  if (!Number.isFinite(value) || value === 0) return '0.0%'
  return `${value > 0 ? '+' : ''}${value.toFixed(1)}%`
}

function formatSnapshotTime(value: string) {
  if (!value) return ''
  return new Date(value).toLocaleString()
}

function formatBucket(value: string) {
  return value || '-'
}

function maxCost<T extends { actual_cost: number }>(items: T[]) {
  return Math.max(...items.map((item) => item.actual_cost || 0), 0)
}

type RankedChartAccent = 'teal' | 'blue' | 'amber' | 'rose'

const rankedChartPalette: Record<RankedChartAccent, { fill: string; border: string }> = {
  teal: { fill: 'rgba(45, 212, 191, 0.70)', border: '#2dd4bf' },
  blue: { fill: 'rgba(96, 165, 250, 0.70)', border: '#60a5fa' },
  amber: { fill: 'rgba(251, 191, 36, 0.68)', border: '#fbbf24' },
  rose: { fill: 'rgba(251, 113, 133, 0.66)', border: '#fb7185' }
}

function makeRankedChartData<T extends { actual_cost: number }>(
  items: T[],
  labelOf: (item: T) => string,
  accent: RankedChartAccent
): ChartData<'bar'> | null {
  if (items.length === 0) return null
  const palette = rankedChartPalette[accent]

  return {
    labels: items.map(labelOf),
    datasets: [
      {
        label: t('usage.actualCost'),
        data: items.map((item) => item.actual_cost || 0),
        backgroundColor: palette.fill,
        borderColor: palette.border,
        borderRadius: 4,
        borderSkipped: false,
        borderWidth: 1,
        maxBarThickness: 26,
        categoryPercentage: 0.74,
        barPercentage: 0.78
      }
    ]
  }
}

function makeRankedChartOptions(
  labels: string[],
  tooltipLines: (index: number) => string[],
  onSelect?: (index: number) => void
): ChartOptions<'bar'> {
  const colors = chartTheme.value

  return {
    indexAxis: 'y',
    responsive: true,
    maintainAspectRatio: false,
    interaction: {
      mode: 'index',
      intersect: false
    },
    onClick: (_event: ChartEvent, elements: ActiveElement[]) => {
      const index = elements[0]?.index
      if (typeof index === 'number') {
        onSelect?.(index)
      }
    },
    onHover: (event: ChartEvent, elements: ActiveElement[]) => {
      const target = event.native?.target
      if (target instanceof HTMLElement) {
        target.style.cursor = onSelect && elements.length > 0 ? 'pointer' : 'default'
      }
    },
    layout: {
      padding: { top: 4, right: 8, bottom: 0, left: 0 }
    },
    plugins: {
      legend: { display: false },
      tooltip: {
        displayColors: false,
        backgroundColor: colors.tooltipBg,
        borderColor: colors.tooltipBorder,
        borderWidth: 1,
        titleColor: colors.text,
        bodyColor: colors.text,
        padding: 10,
        callbacks: {
          title: (items: TooltipItem<'bar'>[]) => items[0]?.label || '',
          label: (context: TooltipItem<'bar'>) => tooltipLines(context.dataIndex)[0] || '',
          afterLabel: (context: TooltipItem<'bar'>) => tooltipLines(context.dataIndex).slice(1)
        }
      }
    },
    scales: {
      x: {
        beginAtZero: true,
        border: { display: false },
        grid: {
          color: colors.grid,
          tickLength: 0
        },
        ticks: {
          color: colors.mutedText,
          padding: 8,
          font: { size: 11 },
          callback: (value: string | number) => formatMoney(Number(value))
        }
      },
      y: {
        border: { color: colors.grid },
        grid: { display: false },
        ticks: {
          color: colors.mutedText,
          font: { size: 11 },
          callback: (_value: string | number, index: number) => truncateChartLabel(labels[index] || '')
        }
      }
    }
  }
}

function rankedChartHeight(count: number) {
  return `${Math.max(220, Math.min(420, 96 + count * 34))}px`
}

function truncateChartLabel(value: string) {
  return value.length > 18 ? `${value.slice(0, 17)}...` : value
}

function memberLeaderboardLabel(item: OwnerMemberLeaderboardItem) {
  return item.member_name || item.member_code || `#${item.member_id}`
}

function setAnalyticsDimension(value: AnalyticsDimension) {
  analyticsDimension.value = value
  if (value === 'member' && activeTab.value === 'tags') {
    activeTab.value = 'leaderboard'
  }
  if (value === 'member') {
    selectedAnalyticsTag.value = null
    selectedAnalyticsStatus.value = null
    analyticsSearch.value = ''
  }
}

function selectAnalyticsAPIKey(item: OwnerApiKeyLeaderboardItem) {
  selectedAnalyticsAPIKeyID.value = item.api_key_id
  activeTab.value = 'trend'
}

function selectAnalyticsMember(item: OwnerMemberLeaderboardItem) {
  if (!item.member_id) return
  selectedAnalyticsMemberFilter.value = `member:${item.member_id}`
  selectedAnalyticsAPIKeyID.value = null
  emit('update:member-filter', selectedAnalyticsMemberFilter.value)
  activeTab.value = 'trend'
}

function selectAnalyticsGroup(item: OwnerGroupAnalyticsItem) {
  selectedAnalyticsAPIKeyID.value = null
  selectedAnalyticsGroupID.value = typeof item.group_id === 'number' && item.group_id > 0 ? item.group_id : 0
  activeTab.value = 'leaderboard'
}

function selectAnalyticsTag(item: OwnerTagAnalyticsItem) {
  selectedAnalyticsAPIKeyID.value = null
  selectedAnalyticsTag.value = item.tag
  activeTab.value = 'leaderboard'
}

function resetAnalyticsFilters() {
  selectedAnalyticsMemberFilter.value = props.memberFilter
  selectedAnalyticsAPIKeyID.value = props.apiKeyId || null
  selectedAnalyticsGroupID.value = null
  selectedAnalyticsTag.value = null
  selectedAnalyticsStatus.value = null
  analyticsSearch.value = ''
  analyticsLimit.value = 20
  emit('update:member-filter', selectedAnalyticsMemberFilter.value)
}

function exportAnalyticsCSV() {
  const exportData = currentAnalyticsExport()
  if (exportData.rows.length === 0) return
  const csv = serializeCSV(exportData.headers, exportData.rows)
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = exportData.filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function currentAnalyticsExport() {
  const suffix = `${props.startDate || 'start'}_to_${props.endDate || 'end'}`
  const dimensionPrefix = analyticsDimension.value === 'member' ? 'member' : 'api_key'
  if (activeTab.value === 'trend') {
    return {
      filename: `${dimensionPrefix}_usage_trend_${suffix}.csv`,
      headers: ['bucket', 'requests', 'input_tokens', 'output_tokens', 'cache_creation_tokens', 'cache_read_tokens', 'total_tokens', 'actual_cost'],
      rows: trendItems.value.map((item) => [
        item.date,
        item.requests,
        item.input_tokens,
        item.output_tokens,
        item.cache_creation_tokens,
        item.cache_read_tokens,
        item.total_tokens,
        item.actual_cost
      ])
    }
  }
  if (activeTab.value === 'models') {
    return {
      filename: `${dimensionPrefix}_usage_models_${suffix}.csv`,
      headers: ['model', 'requests', 'total_tokens', 'actual_cost'],
      rows: modelItems.value.map((item) => [item.model, item.requests, item.total_tokens, item.actual_cost])
    }
  }
  if (activeTab.value === 'groups') {
    return {
      filename: `${dimensionPrefix}_usage_groups_${suffix}.csv`,
      headers: ['group_id', 'group_name', 'key_count', 'share_percent', 'requests', 'total_tokens', 'actual_cost'],
      rows: groupItems.value.map((item) => [item.group_id ?? '', item.group_name, item.key_count, item.share_percent, item.requests, item.total_tokens, item.actual_cost])
    }
  }
  if (activeTab.value === 'tags') {
    return {
      filename: `api_key_usage_tags_${suffix}.csv`,
      headers: ['tag', 'key_count', 'requests', 'total_tokens', 'actual_cost'],
      rows: tagItems.value.map((item) => [item.tag, item.key_count, item.requests, item.total_tokens, item.actual_cost])
    }
  }
  if (analyticsDimension.value === 'member') {
    return {
      filename: `member_usage_leaderboard_${suffix}.csv`,
      headers: ['member_id', 'member_code', 'member_name', 'status', 'archived', 'key_count', 'monthly_limit_usd', 'current_used_usd', 'current_reserved_usd', 'requests', 'total_tokens', 'actual_cost', 'share_percent', 'change_percent', 'last_used_at'],
      rows: memberLeaderboardItems.value.map((item) => [
        item.member_id ?? '',
        item.member_code,
        item.member_name,
        item.status,
        item.archived ? 'true' : 'false',
        item.key_count,
        item.monthly_limit_usd,
        item.current_used_usd,
        item.current_reserved_usd,
        item.requests,
        item.total_tokens,
        item.actual_cost,
        item.share_percent,
        item.change_percent,
        item.last_used_at || ''
      ])
    }
  }
  return {
    filename: `api_key_usage_leaderboard_${suffix}.csv`,
    headers: ['api_key_id', 'key_name', 'status', 'group_id', 'group_name', 'tags', 'requests', 'total_tokens', 'actual_cost', 'share_percent', 'change_percent', 'last_used_at'],
    rows: leaderboardItems.value.map((item) => [
      item.api_key_id,
      item.key_name,
      item.status,
      item.group_id ?? '',
      item.group_name,
      (item.tags || []).join('|'),
      item.requests,
      item.total_tokens,
      item.actual_cost,
      item.share_percent,
      item.change_percent,
      item.last_used_at || ''
    ])
  }
}

function barWidth(value: number, max: number) {
  if (max <= 0) return '0%'
  return `${Math.max((value / max) * 100, value > 0 ? 3 : 0)}%`
}

function tabButtonClass(active: boolean) {
  return [
    'inline-flex h-10 items-center gap-2 whitespace-nowrap border-b-2 px-3 text-sm font-medium transition-colors',
    active
      ? 'border-primary-500 text-primary-600 dark:text-primary-400'
      : 'border-transparent text-stone-500 hover:text-stone-950 dark:text-stone-400 dark:hover:text-white'
  ]
}

const EmptyRows = defineComponent({
  name: 'OwnerAnalyticsEmptyRows',
  props: {
    colspan: { type: Number, required: true }
  },
  setup(props) {
    const { t } = useI18n()
    return () => h('tr', [
      h('td', {
        colspan: props.colspan,
        class: 'px-3 py-8 text-center text-sm text-stone-500 dark:text-stone-400'
      }, t('common.noData'))
    ])
  }
})

const ProgressCell = defineComponent({
  name: 'OwnerAnalyticsProgressCell',
  props: {
    value: { type: Number, required: true },
    max: { type: Number, required: true }
  },
  setup(props) {
    return () => h('div', { class: 'flex items-center justify-end gap-2' }, [
      h('div', { class: 'h-1.5 w-16 overflow-hidden rounded-full bg-stone-200 dark:bg-white/10' }, [
        h('div', {
          class: 'h-full rounded-full bg-primary-500 dark:bg-primary-400',
          style: { width: barWidth(props.value, props.max) }
        })
      ]),
      h('span', { class: 'min-w-[74px] tabular-nums font-medium text-stone-950 dark:text-white' }, formatMoney(props.value))
    ])
  }
})

const OwnerLeaderboardTable = defineComponent({
  name: 'OwnerLeaderboardTable',
  props: {
    items: { type: Array as PropType<OwnerApiKeyLeaderboardItem[]>, required: true },
    maxCost: { type: Number, required: true },
    onSelect: { type: Function as PropType<(item: OwnerApiKeyLeaderboardItem) => void>, default: null }
  },
  setup(props) {
    const { t } = useI18n()
    return () => h('div', { class: 'overflow-hidden rounded-lg border border-stone-200/80 dark:border-white/10' }, [
      h('div', { class: 'overflow-x-auto' }, [
        h('table', { class: 'min-w-full text-sm' }, [
          h('thead', { class: 'bg-stone-50 dark:bg-white/[0.04]' }, [
            h('tr', [
              h('th', { class: headerClass }, t('usage.analytics.apiKey')),
              h('th', { class: headerClass }, t('keys.tags')),
              h('th', { class: headerClass }, t('keys.group')),
              h('th', { class: numericHeaderClass }, t('usage.totalRequests')),
              h('th', { class: numericHeaderClass }, t('usage.totalTokens')),
              h('th', { class: numericHeaderClass }, t('usage.actualCost')),
              h('th', { class: numericHeaderClass }, t('usage.analytics.share')),
              h('th', { class: numericHeaderClass }, t('usage.analytics.change'))
            ])
          ]),
          h('tbody', { class: bodyClass }, props.items.length === 0 ? [
            h(EmptyRows, { colspan: 8 })
          ] : props.items.map((item, index) => h('tr', {
            key: item.api_key_id,
            class: tableRowClass(!!props.onSelect),
            onClick: () => props.onSelect?.(item)
          }, [
            h('td', { class: cellClass }, [
              h('div', { class: 'flex items-center gap-2' }, [
                h('span', { class: 'flex h-6 w-6 items-center justify-center rounded bg-stone-100 text-xs font-semibold text-stone-500 dark:bg-white/10 dark:text-stone-300' }, index + 1),
                h('span', { class: 'font-medium text-stone-950 dark:text-white' }, item.key_name || `#${item.api_key_id}`)
              ])
            ]),
            h('td', { class: cellClass }, renderTags(item.tags)),
            h('td', { class: cellClass }, item.group_name || t('keys.noGroup')),
            h('td', { class: numericCellClass }, formatCompactNumber(item.requests)),
            h('td', { class: numericCellClass, title: formatInteger(item.total_tokens) }, formatCompactNumber(item.total_tokens)),
            h('td', { class: numericCellClass }, h(ProgressCell, { value: item.actual_cost, max: props.maxCost })),
            h('td', { class: numericCellClass }, formatPercent(item.share_percent)),
            h('td', { class: changeClass(item.change_percent) }, formatChange(item.change_percent))
          ])))
        ])
      ])
    ])
  }
})

const OwnerMemberLeaderboardTable = defineComponent({
  name: 'OwnerMemberLeaderboardTable',
  props: {
    items: { type: Array as PropType<OwnerMemberLeaderboardItem[]>, required: true },
    maxCost: { type: Number, required: true },
    onSelect: { type: Function as PropType<(item: OwnerMemberLeaderboardItem) => void>, default: null }
  },
  setup(props) {
    const { t } = useI18n()
    const statusLabel = (item: OwnerMemberLeaderboardItem) => {
      if (item.archived) return t('usage.members.archived')
      if (item.status === 'active') return t('keys.status.active')
      if (item.status === 'disabled') return t('keys.status.disabled')
      return item.status
    }
    const budgetText = (item: OwnerMemberLeaderboardItem) => {
      const consumed = item.current_used_usd + item.current_reserved_usd
      if (item.monthly_limit_usd <= 0) {
        return `${formatMoney(consumed)} / ∞`
      }
      return `${formatMoney(consumed)} / ${formatMoney(item.monthly_limit_usd)}`
    }
    return () => h('div', { class: 'overflow-hidden rounded-lg border border-stone-200/80 dark:border-white/10' }, [
      h('div', { class: 'overflow-x-auto' }, [
        h('table', { class: 'min-w-full text-sm' }, [
          h('thead', { class: 'bg-stone-50 dark:bg-white/[0.04]' }, [
            h('tr', [
              h('th', { class: headerClass }, t('usage.member')),
              h('th', { class: headerClass }, t('common.status')),
              h('th', { class: numericHeaderClass }, t('usage.analytics.keyCount')),
              h('th', { class: numericHeaderClass }, t('usage.analytics.memberBudget')),
              h('th', { class: numericHeaderClass }, t('usage.totalRequests')),
              h('th', { class: numericHeaderClass }, t('usage.totalTokens')),
              h('th', { class: numericHeaderClass }, t('usage.actualCost')),
              h('th', { class: numericHeaderClass }, t('usage.analytics.share')),
              h('th', { class: numericHeaderClass }, t('usage.analytics.change'))
            ])
          ]),
          h('tbody', { class: bodyClass }, props.items.length === 0 ? [
            h(EmptyRows, { colspan: 9 })
          ] : props.items.map((item, index) => h('tr', {
            key: item.member_id!,
            class: tableRowClass(!!props.onSelect),
            onClick: () => props.onSelect?.(item)
          }, [
            h('td', { class: cellClass }, [
              h('div', { class: 'flex items-center gap-2' }, [
                h('span', { class: 'flex h-6 w-6 items-center justify-center rounded bg-stone-100 text-xs font-semibold text-stone-500 dark:bg-white/10 dark:text-stone-300' }, index + 1),
                h('div', { class: 'min-w-0' }, [
                  h('div', { class: 'max-w-52 truncate font-medium text-stone-950 dark:text-white' }, memberLeaderboardLabel(item)),
                  item.member_id
                    ? h('div', { class: 'mt-0.5 max-w-52 truncate font-mono text-[11px] text-stone-400' }, item.member_code || `#${item.member_id}`)
                    : null
                ])
              ])
            ]),
            h('td', { class: cellClass }, [
              h('span', {
                class: item.archived
                  ? 'rounded bg-stone-200 px-2 py-1 text-xs text-stone-600 dark:bg-white/10 dark:text-stone-300'
                  : 'rounded bg-emerald-50 px-2 py-1 text-xs text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300'
              }, statusLabel(item))
            ]),
            h('td', { class: numericCellClass }, formatInteger(item.key_count)),
            h('td', {
              class: numericCellClass,
              title: `${t('usage.analytics.budgetUsed')}: ${formatMoney(item.current_used_usd)}; ${t('usage.analytics.reservedBudget')}: ${formatMoney(item.current_reserved_usd)}`
            }, budgetText(item)),
            h('td', { class: numericCellClass }, formatCompactNumber(item.requests)),
            h('td', { class: numericCellClass, title: formatInteger(item.total_tokens) }, formatCompactNumber(item.total_tokens)),
            h('td', { class: numericCellClass }, h(ProgressCell, { value: item.actual_cost, max: props.maxCost })),
            h('td', { class: numericCellClass }, formatPercent(item.share_percent)),
            h('td', { class: changeClass(item.change_percent) }, formatChange(item.change_percent))
          ])))
        ])
      ])
    ])
  }
})

const OwnerModelTable = defineComponent({
  name: 'OwnerModelTable',
  props: {
    items: { type: Array as PropType<OwnerModelAnalyticsItem[]>, required: true },
    maxCost: { type: Number, required: true }
  },
  setup(props) {
    const { t } = useI18n()
    return () => h(SimpleMetricTable, {
      items: props.items,
      maxCost: props.maxCost,
      nameHeader: t('usage.analytics.model'),
      nameOf: (item: OwnerModelAnalyticsItem) => item.model || 'unknown'
    })
  }
})

const OwnerGroupTable = defineComponent({
  name: 'OwnerGroupTable',
  props: {
    items: { type: Array as PropType<OwnerGroupAnalyticsItem[]>, required: true },
    maxCost: { type: Number, required: true },
    onSelect: { type: Function as PropType<(item: OwnerGroupAnalyticsItem) => void>, default: null }
  },
  setup(props) {
    const { t } = useI18n()
    return () => h(SimpleMetricTable, {
      items: props.items,
      maxCost: props.maxCost,
      nameHeader: t('keys.group'),
      nameOf: (item: OwnerGroupAnalyticsItem) => item.group_name || t('keys.noGroup'),
      extraHeader: t('usage.analytics.keyCount'),
      extraOf: (item: OwnerGroupAnalyticsItem) => formatInteger(item.key_count),
      shareOf: (item: OwnerGroupAnalyticsItem) => formatPercent(item.share_percent),
      onSelect: props.onSelect
    })
  }
})

const OwnerTagTable = defineComponent({
  name: 'OwnerTagTable',
  props: {
    items: { type: Array as PropType<OwnerTagAnalyticsItem[]>, required: true },
    maxCost: { type: Number, required: true },
    onSelect: { type: Function as PropType<(item: OwnerTagAnalyticsItem) => void>, default: null }
  },
  setup(props) {
    const { t } = useI18n()
    return () => h(SimpleMetricTable, {
      items: props.items,
      maxCost: props.maxCost,
      nameHeader: t('keys.tags'),
      nameOf: (item: OwnerTagAnalyticsItem) => item.tag,
      extraHeader: t('usage.analytics.keyCount'),
      extraOf: (item: OwnerTagAnalyticsItem) => formatInteger(item.key_count),
      onSelect: props.onSelect
    })
  }
})

const OwnerTrendTable = defineComponent({
  name: 'OwnerTrendTable',
  props: {
    items: { type: Array as PropType<OwnerTrendAnalyticsPoint[]>, required: true }
  },
  setup(props) {
    const { t } = useI18n()
    return () => h('div', { class: 'overflow-hidden rounded-lg border border-stone-200/80 dark:border-white/10' }, [
      h('div', { class: 'overflow-x-auto' }, [
        h('table', { class: 'min-w-full text-sm' }, [
          h('thead', { class: 'bg-stone-50 dark:bg-white/[0.04]' }, [
            h('tr', [
              h('th', { class: headerClass }, t('keys.usageDetails.bucket')),
              h('th', { class: numericHeaderClass }, t('usage.totalRequests')),
              h('th', { class: numericHeaderClass }, t('usage.in')),
              h('th', { class: numericHeaderClass }, t('usage.out')),
              h('th', { class: numericHeaderClass }, t('usage.cacheWrite')),
              h('th', { class: numericHeaderClass }, t('usage.cacheRead')),
              h('th', { class: numericHeaderClass }, t('usage.totalTokens')),
              h('th', { class: numericHeaderClass }, t('usage.actualCost'))
            ])
          ]),
          h('tbody', { class: bodyClass }, props.items.length === 0 ? [
            h(EmptyRows, { colspan: 8 })
          ] : props.items.map((item) => h('tr', { key: item.date, class: rowClass }, [
            h('td', { class: `${cellClass} font-medium text-stone-950 dark:text-white` }, formatBucket(item.date)),
            h('td', { class: numericCellClass }, formatCompactNumber(item.requests)),
            h('td', { class: numericCellClass, title: formatInteger(item.input_tokens) }, formatCompactNumber(item.input_tokens)),
            h('td', { class: numericCellClass, title: formatInteger(item.output_tokens) }, formatCompactNumber(item.output_tokens)),
            h('td', { class: numericCellClass, title: formatInteger(item.cache_creation_tokens) }, formatCompactNumber(item.cache_creation_tokens)),
            h('td', { class: numericCellClass, title: formatInteger(item.cache_read_tokens) }, formatCompactNumber(item.cache_read_tokens)),
            h('td', { class: numericCellClass, title: formatInteger(item.total_tokens) }, formatCompactNumber(item.total_tokens)),
            h('td', { class: `${numericCellClass} font-medium text-stone-950 dark:text-white` }, formatMoney(item.actual_cost))
          ])))
        ])
      ])
    ])
  }
})

const SimpleMetricTable = defineComponent({
  name: 'OwnerSimpleMetricTable',
  props: {
    items: { type: Array as PropType<Array<{ requests: number; total_tokens: number; actual_cost: number }>>, required: true },
    maxCost: { type: Number, required: true },
    nameHeader: { type: String, required: true },
    nameOf: { type: Function as PropType<(item: any) => string>, required: true },
    extraHeader: { type: String, default: '' },
    extraOf: { type: Function as PropType<(item: any) => string>, default: null },
    shareOf: { type: Function as PropType<(item: any) => string>, default: null },
    onSelect: { type: Function as PropType<(item: any) => void>, default: null }
  },
  setup(props) {
    const { t } = useI18n()
    return () => {
      const extraOf = props.extraOf as ((item: any) => string) | null | undefined
      const shareOf = props.shareOf as ((item: any) => string) | null | undefined
      const hasExtra = props.extraHeader !== ''
      const hasShare = typeof shareOf === 'function'

      return h('div', { class: 'overflow-hidden rounded-lg border border-stone-200/80 dark:border-white/10' }, [
      h('div', { class: 'overflow-x-auto' }, [
        h('table', { class: 'min-w-full text-sm' }, [
          h('thead', { class: 'bg-stone-50 dark:bg-white/[0.04]' }, [
            h('tr', [
              h('th', { class: headerClass }, props.nameHeader),
              hasExtra ? h('th', { class: numericHeaderClass }, props.extraHeader) : null,
              h('th', { class: numericHeaderClass }, t('usage.totalRequests')),
              h('th', { class: numericHeaderClass }, t('usage.totalTokens')),
              h('th', { class: numericHeaderClass }, t('usage.actualCost')),
              hasShare ? h('th', { class: numericHeaderClass }, t('usage.analytics.share')) : null
            ].filter(Boolean))
          ]),
          h('tbody', { class: bodyClass }, props.items.length === 0 ? [
            h(EmptyRows, { colspan: hasShare ? 6 : hasExtra ? 5 : 4 })
          ] : props.items.map((item, index) => h('tr', {
            key: `${props.nameOf(item)}-${index}`,
            class: tableRowClass(!!props.onSelect),
            onClick: () => props.onSelect?.(item)
          }, [
            h('td', { class: `${cellClass} font-medium text-stone-950 dark:text-white` }, props.nameOf(item)),
            hasExtra ? h('td', { class: numericCellClass }, extraOf ? extraOf(item) : '') : null,
            h('td', { class: numericCellClass }, formatCompactNumber(item.requests)),
            h('td', { class: numericCellClass, title: formatInteger(item.total_tokens) }, formatCompactNumber(item.total_tokens)),
            h('td', { class: numericCellClass }, h(ProgressCell, { value: item.actual_cost, max: props.maxCost })),
            hasShare ? h('td', { class: numericCellClass }, shareOf(item)) : null
          ].filter(Boolean))))
        ])
      ])
    ])
    }
  }
})

const headerClass = 'px-3 py-2 text-left font-medium text-stone-500 dark:text-stone-400'
const numericHeaderClass = 'px-3 py-2 text-right font-medium text-stone-500 dark:text-stone-400'
const cellClass = 'whitespace-nowrap px-3 py-2 text-stone-700 dark:text-stone-300'
const numericCellClass = 'whitespace-nowrap px-3 py-2 text-right tabular-nums text-stone-700 dark:text-stone-300'
const bodyClass = 'divide-y divide-stone-100 bg-white/70 dark:divide-white/10 dark:bg-transparent'
const rowClass = 'hover:bg-stone-50 dark:hover:bg-white/[0.04]'

function tableRowClass(clickable: boolean) {
  return [rowClass, clickable ? 'cursor-pointer' : '']
}

function changeClass(value: number) {
  return [
    numericCellClass,
    value > 0 ? 'text-red-600 dark:text-red-300' : value < 0 ? 'text-emerald-600 dark:text-emerald-300' : ''
  ]
}

function renderTags(tags: string[] | undefined) {
  if (!tags || tags.length === 0) {
    return h('span', { class: 'text-stone-400 dark:text-stone-500' }, '-')
  }
  return h('div', { class: 'flex max-w-[220px] flex-wrap gap-1' }, tags.slice(0, 3).map((tag) =>
    h('span', { class: 'rounded bg-primary-50 px-1.5 py-0.5 text-xs text-primary-700 dark:bg-primary-500/10 dark:text-primary-300' }, tag)
  ))
}

watch(() => props.apiKeyId, (apiKeyID) => {
  selectedAnalyticsAPIKeyID.value = apiKeyID || null
})

watch(() => props.memberFilter, (memberFilter) => {
  if (memberFilter !== selectedAnalyticsMemberFilter.value) {
    selectedAnalyticsMemberFilter.value = memberFilter
  }
})

watch(() => props.enterprise, (enterprise) => {
  analyticsDimension.value = enterprise ? 'member' : 'key'
  if (!enterprise) selectedAnalyticsMemberFilter.value = 'all'
})

watch(selectedAnalyticsMemberFilter, (memberFilter) => {
  if (selectedAnalyticsAPIKeyID.value && !analyticsAPIKeys.value.some((key) => key.id === selectedAnalyticsAPIKeyID.value)) {
    selectedAnalyticsAPIKeyID.value = null
  }
  if (memberFilter !== props.memberFilter) {
    emit('update:member-filter', memberFilter)
  }
})

watch(requestSignature, scheduleLoad)

onMounted(loadAnalytics)

onUnmounted(() => {
  if (abortController) abortController.abort()
  if (reloadTimer) clearTimeout(reloadTimer)
})
</script>
