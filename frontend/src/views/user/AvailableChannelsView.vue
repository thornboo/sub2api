<template>
  <AppLayout>
    <TablePageLayout :table-mode="viewMode === 'cards' ? 'auto' : 'scroll'">
      <template #filters>
        <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400 dark:text-stone-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('availableChannels.searchPlaceholder')"
                class="input pl-10"
              />
            </div>

            <Select v-model="platformFilter" :options="platformFilterOptions" class="w-full sm:w-52" />

            <Select
              v-model="billingModeFilter"
              :options="billingModeFilterOptions"
              class="w-full sm:w-44"
            />

            <Select
              v-model="groupScopeFilter"
              :options="groupScopeFilterOptions"
              class="w-full sm:w-44"
            />

            <Select
              v-model="priceStatusFilter"
              :options="priceStatusFilterOptions"
              class="w-full sm:w-40"
            />
          </div>

          <div class="flex w-full flex-shrink-0 flex-wrap items-center justify-end gap-3 lg:w-auto">
            <div class="segmented-control">
              <button
                type="button"
                class="segmented-option inline-flex items-center gap-1.5"
                :class="viewMode === 'cards' ? 'segmented-option-active' : 'segmented-option-muted'"
                @click="viewMode = 'cards'"
              >
                <Icon name="grid" size="sm" />
                {{ t('availableChannels.viewMode.marketplace') }}
              </button>
              <button
                type="button"
                class="segmented-option inline-flex items-center gap-1.5"
                :class="viewMode === 'table' ? 'segmented-option-active' : 'segmented-option-muted'"
                @click="viewMode = 'table'"
              >
                <Icon name="menu" size="sm" />
                {{ t('availableChannels.viewMode.table') }}
              </button>
            </div>

            <button
              type="button"
              @click="openExportDialog"
              :disabled="exportButtonDisabled"
              class="btn btn-secondary"
              :title="t('availableChannels.exportExcel')"
            >
              <Icon name="download" size="md" />
              <span class="hidden sm:inline">
                {{ exporting ? t('availableChannels.exporting') : t('availableChannels.exportExcel') }}
              </span>
            </button>

            <button
              @click="loadChannels"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh', 'Refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <AvailableModelMarketplace
          v-if="viewMode === 'cards'"
          :cards="marketplaceCards"
          :loading="loading"
          :pricing-labels="pricingLabels"
          :user-group-rates="userGroupRates"
          :empty-label="t('availableChannels.empty')"
        />
        <AvailableChannelModelsTable
          v-else
          :columns="modelColumnLabels"
          :rows="modelRows"
          :loading="loading"
          :pricing-labels="pricingLabels"
          :tooltips="modelColumnTooltips"
          :user-group-rates="userGroupRates"
          :sort-by="sortBy"
          :sort-order="sortOrder"
          :empty-label="t('availableChannels.empty')"
          @sort="handleModelSort"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="showExportDialog"
      :title="t('availableChannels.export.dialogTitle')"
      width="normal"
      @close="closeExportDialog"
    >
      <div class="space-y-5">
        <div v-if="authStore.isAdmin" class="space-y-2">
          <label class="input-label">{{ t('availableChannels.exportSource.label') }}</label>
          <Select v-model="exportSource" :options="exportSourceOptions" />
          <p v-if="adminCatalogBlocked" class="text-xs leading-relaxed text-amber-600 dark:text-amber-400">
            {{ t('availableChannels.export.fullCatalogUnavailableHint') }}
          </p>
        </div>

        <div class="space-y-2">
          <label class="input-label">{{ t('availableChannels.exportScope.label') }}</label>
          <Select v-model="exportGroupScope" :options="exportGroupScopeOptions" />
        </div>

        <div v-if="authStore.isAdmin && exportSource === 'admin_catalog'" class="space-y-2">
          <label class="input-label">{{ t('availableChannels.exportStatus.label') }}</label>
          <Select v-model="exportStatusScope" :options="exportStatusScopeOptions" />
        </div>

        <div class="rounded-lg border border-stone-200 bg-stone-50 px-4 py-3 dark:border-stone-800 dark:bg-stone-900/50">
          <div class="flex items-center justify-between gap-4 text-sm">
            <span class="text-stone-500 dark:text-stone-400">{{ t('availableChannels.export.rowCount') }}</span>
            <span class="font-mono font-semibold text-stone-950 dark:text-stone-100">{{ exportRows.length }}</span>
          </div>
        </div>
      </div>

      <template #footer>
        <button type="button" class="btn btn-secondary" :disabled="exporting" @click="closeExportDialog">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="exporting || exportRows.length === 0" @click="exportModelCatalog">
          <Icon name="download" size="md" />
          {{ exporting ? t('availableChannels.exporting') : t('availableChannels.exportExcel') }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import AvailableModelMarketplace from '@/components/channels/AvailableModelMarketplace.vue'
import AvailableChannelModelsTable from '@/components/channels/AvailableChannelModelsTable.vue'
import adminChannelsAPI, { type AdminAvailableChannel } from '@/api/admin/channels'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import userGroupsAPI from '@/api/groups'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
  type BillingMode,
} from '@/constants/channel'
import { extractApiErrorMessage } from '@/utils/apiError'
import {
  buildAvailableChannelCatalogRows,
  exportAvailableChannelsCatalog,
  type AvailableChannelGroupScope,
  type AvailableChannelExportLabels,
  type AvailableChannelPricingLabels,
  type AvailableChannelPriceStatus,
  type AvailableChannelSortKey,
  type AvailableChannelSortOrder,
  type AvailableChannelStatusScope,
} from '@/utils/availableChannelsCatalog'
import { buildAvailableModelMarketplaceCards } from '@/utils/availableModelMarketplace'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const channels = ref<UserAvailableChannel[]>([])
const adminChannels = ref<AdminAvailableChannel[]>([])
const userGroupRates = ref<Record<number, number>>({})
const loading = ref(false)
const exporting = ref(false)
const showExportDialog = ref(false)
const adminCatalogLoaded = ref(false)
const adminCatalogError = ref(false)
const searchQuery = ref('')
const platformFilter = ref('')
const billingModeFilter = ref<BillingMode | ''>('')
const groupScopeFilter = ref<AvailableChannelGroupScope>('all')
const priceStatusFilter = ref<AvailableChannelPriceStatus>('all')
type ExportSource = 'admin_catalog' | 'visible_channels'
const exportSource = ref<ExportSource>('admin_catalog')
const exportGroupScope = ref<AvailableChannelGroupScope>('public_exclusive')
const exportStatusScope = ref<AvailableChannelStatusScope>('all')
const sortBy = ref<AvailableChannelSortKey>('model')
const sortOrder = ref<AvailableChannelSortOrder>('asc')
const viewMode = ref<'cards' | 'table'>('cards')

const modelColumnLabels = computed(() => ({
  model: t('availableChannels.modelTable.columns.model'),
  platform: t('availableChannels.modelTable.columns.platform'),
  channel: t('availableChannels.modelTable.columns.channel'),
  billingMode: t('availableChannels.modelTable.columns.billingMode'),
  interval: t('availableChannels.modelTable.columns.interval'),
  inputPrice: t('availableChannels.modelTable.columns.inputPrice'),
  outputPrice: t('availableChannels.modelTable.columns.outputPrice'),
  cacheWritePrice: t('availableChannels.modelTable.columns.cacheWritePrice'),
  cacheReadPrice: t('availableChannels.modelTable.columns.cacheReadPrice'),
  imageOutputPrice: t('availableChannels.modelTable.columns.imageOutputPrice'),
  perRequestPrice: t('availableChannels.modelTable.columns.perRequestPrice'),
  groups: t('availableChannels.modelTable.columns.groups'),
}))

const modelColumnTooltips = computed(() => ({
  interval: t('availableChannels.modelTable.tooltips.interval'),
  inputPrice: t('availableChannels.modelTable.tooltips.inputPrice'),
  outputPrice: t('availableChannels.modelTable.tooltips.outputPrice'),
  cacheWritePrice: t('availableChannels.modelTable.tooltips.cacheWritePrice'),
  cacheReadPrice: t('availableChannels.modelTable.tooltips.cacheReadPrice'),
  imageOutputPrice: t('availableChannels.modelTable.tooltips.imageOutputPrice'),
  perRequestPrice: t('availableChannels.modelTable.tooltips.perRequestPrice'),
}))

const pricingLabels = computed<AvailableChannelPricingLabels>(() => ({
  billingModeToken: t('availableChannels.pricing.billingModeToken'),
  billingModePerRequest: t('availableChannels.pricing.billingModePerRequest'),
  billingModeImage: t('availableChannels.pricing.billingModeImage'),
  noPricing: t('availableChannels.noPricing'),
  unitPerMillion: t('availableChannels.pricing.unitPerMillion'),
  unitPerRequest: t('availableChannels.pricing.unitPerRequest'),
}))

const exportLabels = computed<AvailableChannelExportLabels>(() => ({
  ...pricingLabels.value,
  sheetName: t('availableChannels.export.sheetName'),
  channel: t('availableChannels.export.columns.channel'),
  status: t('availableChannels.export.columns.status'),
  description: t('availableChannels.export.columns.description'),
  platform: t('availableChannels.export.columns.platform'),
  model: t('availableChannels.export.columns.model'),
  groups: t('availableChannels.export.columns.groups'),
  billingMode: t('availableChannels.export.columns.billingMode'),
  interval: t('availableChannels.export.columns.interval'),
  inputPrice: t('availableChannels.export.columns.inputPrice'),
  outputPrice: t('availableChannels.export.columns.outputPrice'),
  cacheWritePrice: t('availableChannels.export.columns.cacheWritePrice'),
  cacheReadPrice: t('availableChannels.export.columns.cacheReadPrice'),
  imageOutputPrice: t('availableChannels.export.columns.imageOutputPrice'),
  perRequestPrice: t('availableChannels.export.columns.perRequestPrice'),
  statusActive: t('common.active'),
  statusDisabled: t('common.disabled'),
  statusUnknown: t('common.none'),
}))

const displayCatalogChannels = computed<UserAvailableChannel[]>(() => {
  return channels.value
})

const exportCatalogChannels = computed<UserAvailableChannel[]>(() => {
  if (authStore.isAdmin && exportSource.value === 'admin_catalog') return adminChannels.value
  return channels.value
})

const platformOptions = computed(() => {
  const platforms = new Set<string>()
  displayCatalogChannels.value.forEach((channel) => {
    channel.platforms.forEach((section) => {
      platforms.add(section.platform)
    })
  })
  return Array.from(platforms).sort((a, b) => a.localeCompare(b))
})

const platformFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('availableChannels.platformFilter.all') },
  ...platformOptions.value.map((platform) => ({ value: platform, label: platform })),
])

const billingModeFilterOptions = computed<SelectOption[]>(() => [
  { value: '', label: t('availableChannels.billingModeFilter.all') },
  { value: BILLING_MODE_TOKEN, label: t('availableChannels.pricing.billingModeToken') },
  { value: BILLING_MODE_PER_REQUEST, label: t('availableChannels.pricing.billingModePerRequest') },
  { value: BILLING_MODE_IMAGE, label: t('availableChannels.pricing.billingModeImage') },
])

const groupScopeFilterOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('availableChannels.groupScopeFilter.all') },
  { value: 'public_exclusive', label: t('availableChannels.groupScopeFilter.publicExclusive') },
  { value: 'public', label: t('availableChannels.groupScopeFilter.public') },
  { value: 'exclusive', label: t('availableChannels.groupScopeFilter.exclusive') },
])

const priceStatusFilterOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('availableChannels.priceStatusFilter.all') },
  { value: 'priced', label: t('availableChannels.priceStatusFilter.priced') },
  { value: 'unpriced', label: t('availableChannels.priceStatusFilter.unpriced') },
])

const exportGroupScopeOptions = computed<SelectOption[]>(() => [
  { value: 'public_exclusive', label: t('availableChannels.exportScope.publicExclusive') },
  { value: 'public', label: t('availableChannels.exportScope.public') },
])

const exportStatusScopeOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('availableChannels.exportStatus.all') },
  { value: 'active', label: t('availableChannels.exportStatus.active') },
  { value: 'disabled', label: t('availableChannels.exportStatus.disabled') },
])

const adminCatalogBlocked = computed(() => authStore.isAdmin && (!adminCatalogLoaded.value || adminCatalogError.value))

const exportSourceOptions = computed<SelectOption[]>(() => [
  {
    value: 'admin_catalog',
    label: t('availableChannels.exportSource.adminCatalog'),
    disabled: adminCatalogBlocked.value,
  },
  { value: 'visible_channels', label: t('availableChannels.exportSource.visibleChannels') },
])

const exportButtonDisabled = computed(() => loading.value || exporting.value || exportRows.value.length === 0)

/**
 * 搜索过滤：
 * - 命中渠道名/描述 → 整个渠道（所有 platforms）都保留
 * - 否则按 platform/group/model 维度在 sections 里过滤，保留有匹配的 section
 * - 所有 sections 都不匹配时，渠道本身被过滤掉
 */
const filteredChannels = computed(() => filterChannelsForSearch(displayCatalogChannels.value))
const filteredExportChannels = computed(() => filterChannelsForSearch(exportCatalogChannels.value))

const marketplaceCards = computed(() =>
  buildAvailableModelMarketplaceCards(filteredChannels.value, {
    billingMode: billingModeFilter.value,
    groupScope: groupScopeFilter.value,
    priceStatus: priceStatusFilter.value,
  }),
)

const modelRows = computed(() =>
  buildAvailableChannelCatalogRows(filteredChannels.value, {
    billingMode: billingModeFilter.value,
    groupScope: groupScopeFilter.value,
    priceStatus: priceStatusFilter.value,
    expandIntervals: true,
    sortBy: sortBy.value,
    sortOrder: sortOrder.value,
    activeOnly: true,
  }),
)

const exportRows = computed(() =>
  buildAvailableChannelCatalogRows(filteredExportChannels.value, {
    includeSubscriptionGroups: false,
    groupScope: exportGroupScope.value,
    billingMode: billingModeFilter.value,
    priceStatus: priceStatusFilter.value,
    statusScope: authStore.isAdmin && exportSource.value === 'admin_catalog' ? exportStatusScope.value : 'active',
    expandIntervals: true,
    sortBy: sortBy.value,
    sortOrder: sortOrder.value,
  }),
)

function filterChannelsForSearch(source: UserAvailableChannel[]): UserAvailableChannel[] {
  const q = searchQuery.value.trim().toLowerCase()
  const selectedPlatform = platformFilter.value
  if (!q && !selectedPlatform) return source
  return source
    .map((ch) => {
      const nameHit = ch.name.toLowerCase().includes(q)
      const descHit = (ch.description || '').toLowerCase().includes(q)
      const sections = Array.isArray(ch.platforms) ? ch.platforms : []
      const matchingSections = sections.map((p) => {
        const groups = Array.isArray(p.groups) ? p.groups : []
        const supportedModels = Array.isArray(p.supported_models) ? p.supported_models : []
        if (selectedPlatform && p.platform !== selectedPlatform) return null
        if (!q || nameHit || descHit) return p

        if (p.platform.toLowerCase().includes(q)) return p

        const matchingGroups = groups.filter((group) => group.name.toLowerCase().includes(q))
        if (matchingGroups.length > 0) return { ...p, groups: matchingGroups }

        const matchingModels = supportedModels.filter((model) =>
          model.name.toLowerCase().includes(q) ||
          (model.supported_endpoints ?? []).some((endpoint) =>
            endpoint.protocol.toLowerCase().includes(q) || endpoint.path.toLowerCase().includes(q),
          ),
        )
        if (matchingModels.length === 0) return null
        return { ...p, supported_models: matchingModels }
      }).filter((section): section is UserAvailableChannel['platforms'][number] => section !== null)
      if (matchingSections.length === 0) return null
      return { ...ch, platforms: matchingSections }
    })
    .filter((ch): ch is UserAvailableChannel => ch !== null)
}

async function loadChannels() {
  loading.value = true
  if (authStore.isAdmin) {
    adminCatalogLoaded.value = false
    adminCatalogError.value = false
  }
  try {
    // 渠道列表和用户专属倍率并发拉取。专属倍率失败不阻塞渠道展示——
    // 失败时只是无法渲染专属倍率角标，降级为仅显示默认倍率。
    const [list, rates, adminCatalogResult] = await Promise.all([
      userChannelsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates().catch((err: unknown) => {
        console.error('Failed to load user group rates:', err)
        return {} as Record<number, number>
      }),
      loadAdminCatalog(),
    ])
    channels.value = list
    userGroupRates.value = rates
    if (adminCatalogResult.ok) {
      adminChannels.value = adminCatalogResult.data
      adminCatalogLoaded.value = true
      adminCatalogError.value = false
      exportSource.value = 'admin_catalog'
    } else {
      adminChannels.value = []
      adminCatalogLoaded.value = false
      adminCatalogError.value = true
      exportSource.value = 'visible_channels'
    }
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  } finally {
    loading.value = false
  }
}

type AdminCatalogLoadResult =
  | { ok: true; data: AdminAvailableChannel[] }
  | { ok: false; error: unknown }

async function loadAdminCatalog(): Promise<AdminCatalogLoadResult> {
  if (!authStore.isAdmin) return { ok: true, data: [] }
  try {
    return { ok: true, data: await adminChannelsAPI.getAvailableCatalog() }
  } catch (error: unknown) {
    console.error('Failed to load admin available-channel catalog:', error)
    return { ok: false, error }
  }
}

function openExportDialog() {
  if (adminCatalogBlocked.value && exportSource.value === 'admin_catalog') exportSource.value = 'visible_channels'
  if (exportRows.value.length === 0) {
    appStore.showError(t('availableChannels.export.noData'))
    return
  }
  showExportDialog.value = true
}

function closeExportDialog() {
  if (exporting.value) return
  showExportDialog.value = false
}

async function exportModelCatalog() {
  if (exportRows.value.length === 0) {
    appStore.showError(t('availableChannels.export.noData'))
    return
  }

  exporting.value = true
  try {
    await exportAvailableChannelsCatalog(exportRows.value, exportLabels.value, userGroupRates.value)
    appStore.showSuccess(t('availableChannels.export.success'))
    showExportDialog.value = false
  } catch (err: unknown) {
    console.error('Failed to export available channels:', err)
    appStore.showError(t('availableChannels.export.failed'))
  } finally {
    exporting.value = false
  }
}

function handleModelSort(key: AvailableChannelSortKey) {
  if (sortBy.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortBy.value = key
  sortOrder.value = 'asc'
}

onMounted(loadChannels)
</script>
