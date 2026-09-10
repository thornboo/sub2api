<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-stone-200/80 bg-white shadow-sm shadow-stone-950/5 dark:border-white/10 dark:bg-stone-950/70 dark:shadow-black/20">
    <div v-if="error" class="m-4 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="border-b border-stone-200/80 px-4 py-3 dark:border-white/10">
      <div class="grid gap-3 lg:grid-cols-3">
        <section
          v-for="card in overviewCards"
          :key="card.key"
          class="rounded-lg border border-stone-200 bg-stone-50/70 p-4 shadow-sm shadow-stone-950/[0.03] dark:border-white/10 dark:bg-white/[0.035]"
          :data-test="`supplier-overview-card-${card.key}`"
          :aria-label="card.title"
        >
          <div class="flex items-center justify-between gap-3">
            <p class="text-sm font-medium text-stone-500 dark:text-stone-400" :title="card.details || undefined">
              {{ card.title }}
            </p>
            <span
              v-if="card.incomplete"
              role="img"
              tabindex="0"
              class="shrink-0 cursor-help rounded text-amber-600 outline-none focus-visible:ring-2 focus-visible:ring-amber-500 dark:text-amber-400"
              :title="card.details"
              :aria-label="card.details"
              :data-test="`supplier-overview-notice-${card.key}`"
            >
              <Icon name="exclamationCircle" size="sm" aria-hidden="true" />
            </span>
          </div>
          <div class="mt-3 flex flex-col gap-1">
            <template v-if="card.totals.length > 0">
              <div
                v-for="total in card.totals"
                :key="total.key"
                class="flex items-baseline justify-between gap-3"
              >
                <span class="font-mono text-2xl font-semibold leading-8 text-stone-950 dark:text-white">
                  {{ total.amount }}
                </span>
                <span class="shrink-0 text-xs font-medium text-stone-500 dark:text-stone-400">
                  {{ total.meta }}
                </span>
              </div>
            </template>
            <p v-else class="text-sm font-medium text-stone-500 dark:text-stone-400">
              {{ card.empty }}
            </p>
          </div>
        </section>
      </div>
      <p v-if="overviewError" class="mt-2 text-xs text-red-600 dark:text-red-300" data-test="supplier-overview-error">
        {{ overviewError }}
      </p>
    </div>

    <div class="min-h-0 flex-1 overflow-auto">
      <table class="min-w-[1260px] divide-y divide-stone-200 text-sm dark:divide-white/10">
        <thead class="sticky top-0 z-10 bg-stone-50 dark:bg-stone-950">
          <tr>
            <th
              v-for="column in sortableColumns"
              :key="column.key"
              class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400"
              :aria-sort="sortAria(column.key)"
            >
              <button
                type="button"
                class="sortable-header"
                :data-test="`sort-${column.key}`"
                @click="toggleSort(column.key)"
              >
                <span>{{ column.label }}</span>
                <Icon :name="sortIcon(column.key)" size="xs" />
              </button>
            </th>
            <th class="px-4 py-3 text-right text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-stone-100 dark:divide-white/[0.06]">
          <tr v-if="loading">
            <td colspan="10" class="px-4 py-10 text-center text-stone-500 dark:text-stone-400">
              {{ t('common.loading') }}...
            </td>
          </tr>
          <tr v-else-if="rows.length === 0">
            <td colspan="10" class="px-4 py-10 text-center text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.noSuppliers') }}
            </td>
          </tr>
          <template v-else>
            <tr
              v-for="row in sortedRows"
              :key="row.supplierID"
              class="hover:bg-stone-50/70 dark:hover:bg-white/[0.035]"
            >
              <td class="px-4 py-4">
                <div class="flex items-start gap-3">
                  <span class="mt-1.5 h-2.5 w-2.5 flex-shrink-0 rounded-full" :class="rowDotClass(row)" />
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="font-semibold text-stone-950 dark:text-white">{{ row.supplierName }}</span>
                      <span v-if="row.pool && row.showPoolName" class="rounded-md bg-stone-100 px-2 py-0.5 text-xs font-medium text-stone-500 dark:bg-white/[0.07] dark:text-stone-400">
                        {{ row.pool.name }}
                      </span>
                    </div>
                    <div v-if="row.supplierNote" class="mt-1 max-w-xs truncate text-xs text-stone-500 dark:text-stone-400" :title="row.supplierNote">
                      {{ row.supplierNote }}
                    </div>
                  </div>
                </div>
              </td>
              <td class="px-4 py-4">
                <span class="font-mono text-stone-700 dark:text-stone-300">{{ row.bindingCount }}</span>
              </td>
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                {{ formatCost(currentEffectiveCost(row)) }}
              </td>
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                <span v-if="hasCurrentCost(row)">
                  {{ formatRatio(row.pool?.current_effective_cny_per_usd) }}
                  <span class="text-stone-400">/</span>
                  {{ formatRatio(row.pool?.reference_fx_rate) }}
                </span>
                <span v-else class="text-stone-400 dark:text-stone-500">-</span>
              </td>
              <td class="px-4 py-4">
                <span :class="discountBadgeClass(row)">
                  {{ discountLabel(row) }}
                </span>
              </td>
              <td class="px-4 py-4">
                <div v-if="row.paidTotals.length > 0" class="flex flex-col gap-1">
                  <span
                    v-for="total in row.paidTotals"
                    :key="total.currency"
                    class="font-mono text-stone-700 dark:text-stone-300"
                  >
                    {{ formatMoney(total.amount, total.currency) }}
                  </span>
                </div>
                <span v-else class="text-stone-400 dark:text-stone-500">-</span>
              </td>
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                {{ row.recordCount }}
              </td>
              <td class="px-4 py-4">
                <div class="min-w-[180px]">
                  <div class="min-w-0">
                    <div :class="balanceValueClass(row)">
                      {{ balanceLabel(row) }}
                    </div>
                    <div
                      v-if="balanceTimeHint(row)"
                      class="mt-1 truncate text-xs text-stone-500 dark:text-stone-400"
                      :title="balanceTimeHint(row)"
                    >
                      {{ balanceTimeHint(row) }}
                    </div>
                    <div
                      v-if="balanceErrorHint(row)"
                      class="mt-1 line-clamp-2 max-w-[220px] break-words text-xs leading-4 text-red-700 dark:text-red-300"
                      :title="balanceErrorHint(row)"
                    >
                      {{ balanceErrorHint(row) }}
                    </div>
                  </div>
                </div>
              </td>
              <td class="px-4 py-4">
                <span :class="statusBadgeClass(row)">
                  {{ statusText(row) }}
                </span>
              </td>
              <td class="px-4 py-4">
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <button
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-stone-700 transition-colors hover:border-emerald-300 hover:text-emerald-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-emerald-500/40 dark:hover:text-emerald-300"
                    :disabled="!row.pool"
                    @click="row.pool && $emit('recharge-records', row.pool)"
                  >
                    <Icon name="creditCard" size="xs" />
                    {{ t('admin.accounts.upstreamCost.rechargeRecords.action') }}
                  </button>
                  <button
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-stone-700 transition-colors hover:border-emerald-300 hover:text-emerald-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-emerald-500/40 dark:hover:text-emerald-300"
                    @click="emit('recharge-trend', row.supplierID, row.supplierName)"
                  >
                    <Icon name="trendingUp" size="xs" />
                    {{ t('admin.accounts.upstreamCost.rechargeTrend.action') }}
                  </button>
                  <button
                    v-if="!isReserved(row)"
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-stone-700 transition-colors hover:border-sky-300 hover:text-sky-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-sky-500/40 dark:hover:text-sky-300"
                    :disabled="supplierMutating"
                    @click="emit('edit-supplier', row.supplierID)"
                  >
                    <Icon name="edit" size="xs" />
                    {{ t('common.edit') }}
                  </button>
                  <button
                    v-if="!isReserved(row)"
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-stone-700 transition-colors hover:border-amber-300 hover:text-amber-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-amber-500/40 dark:hover:text-amber-300"
                    :disabled="supplierMutating"
                    @click="toggleArchive(row)"
                  >
                    <Icon :name="row.supplierStatus === 'archived' ? 'refresh' : 'inbox'" size="xs" />
                    {{ row.supplierStatus === 'archived' ? t('admin.accounts.upstreamCost.unarchive') : t('admin.accounts.upstreamCost.archive') }}
                  </button>
                  <button
                    v-if="!isReserved(row)"
                    type="button"
                    class="inline-flex h-8 items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 text-xs font-medium text-stone-700 transition-colors hover:border-red-300 hover:text-red-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-red-500/40 dark:hover:text-red-300"
                    :disabled="supplierMutating"
                    @click="openSupplierDelete(row)"
                  >
                    <Icon name="trash" size="xs" />
                    {{ t('common.delete') }}
                  </button>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
    <ConfirmDialog
      :show="archiveTarget !== null"
      :title="t('admin.accounts.upstreamCost.archiveSupplierTitle')"
      :message="archiveMessage"
      :confirm-text="t('admin.accounts.upstreamCost.archive')"
      @confirm="confirmSupplierArchive"
      @cancel="archiveTarget = null"
    />
    <ConfirmDialog
      :show="deleteTarget !== null"
      danger
      :title="t('admin.accounts.upstreamCost.deleteSupplierTitle')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      @confirm="confirmSupplierDelete"
      @cancel="deleteTarget = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  UpstreamCostPool,
  UpstreamRechargeCurrencyTotal,
  UpstreamSupplier,
  UpstreamSupplierConsumptionIssue,
  UpstreamSupplierConsumptionPeriod,
  UpstreamSupplierConsumptionTotal,
  UpstreamSupplierRechargeOverview
} from '@/api/admin/accounts'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { ConfirmDialog } from '@/components/common'
import { extractApiErrorCode } from '@/utils/apiError'
import { formatUpstreamDiscountLabel, formatUpstreamRatio } from '@/utils/upstreamCost'

interface SupplierCostRow {
  supplierID: number
  supplierName: string
  supplierStatus: string
  supplierNote?: string | null
  isSystem: boolean
  supplier: UpstreamSupplier
  pools: UpstreamCostPool[]
  pool: UpstreamCostPool | null
  showPoolName: boolean
  bindingCount: number
  recordCount: number
  paidReferenceCNY: number
  paidTotals: UpstreamRechargeCurrencyTotal[]
}

const props = defineProps<{
  suppliers: UpstreamSupplier[]
  costPools: UpstreamCostPool[]
  rechargeOverview?: UpstreamSupplierRechargeOverview | null
  loading?: boolean
  error?: string | null
  overviewError?: string | null
}>()

const emit = defineEmits<{
  refresh: [options?: { forcePools?: boolean }]
  'recharge-records': [pool: UpstreamCostPool]
  'recharge-trend': [supplierID: number, supplierName: string]
  'edit-supplier': [supplierID: number]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const supplierMutating = ref(false)
const archiveTarget = ref<SupplierCostRow | null>(null)
const deleteTarget = ref<SupplierCostRow | null>(null)

type SupplierSortKey = 'supplier' | 'boundAccounts' | 'currentCost' | 'rechargeRatio' | 'poolDiscount' | 'totalPaid' | 'records' | 'walletBalance' | 'status'
type SupplierSortOrder = 'asc' | 'desc'

const sortKey = ref<SupplierSortKey | null>(null)
const sortOrder = ref<SupplierSortOrder>('asc')
const sortableColumns = computed<Array<{ key: SupplierSortKey; label: string }>>(() => [
  { key: 'supplier', label: t('admin.accounts.upstreamCost.supplier') },
  { key: 'boundAccounts', label: t('admin.accounts.upstreamCost.boundAccounts') },
  { key: 'currentCost', label: t('admin.accounts.upstreamCost.currentCost') },
  { key: 'rechargeRatio', label: t('admin.accounts.upstreamCost.rechargeRatio') },
  { key: 'poolDiscount', label: t('admin.accounts.upstreamCost.poolDiscountUSD') },
  { key: 'totalPaid', label: t('admin.accounts.upstreamCost.totalPaid') },
  { key: 'records', label: t('admin.accounts.upstreamCost.rechargeRecords.records') },
  { key: 'walletBalance', label: t('admin.accounts.upstreamCost.supplierBalance.balance') },
  { key: 'status', label: t('admin.accounts.upstreamCost.status') }
])

const isReserved = (row: SupplierCostRow) => row.isSystem
const isActivePool = (pool: UpstreamCostPool) => pool.status === 'active' && !pool.archived_at
const overviewTotals = computed(() => sortCurrencyTotals(props.rechargeOverview?.totals || []))
const consumptionOverview = computed(() => props.rechargeOverview?.consumption ?? null)
const overviewCards = computed(() => [
  buildRechargeOverviewCard(),
  buildConsumptionOverviewCard('today', consumptionOverview.value?.today ?? null),
  buildConsumptionOverviewCard('last7', consumptionOverview.value?.last_7_days ?? null)
])
const supplierPaidTotalsByID = computed(() => {
  const result = new Map<number, UpstreamRechargeCurrencyTotal[]>()
  for (const supplier of props.rechargeOverview?.suppliers || []) {
    result.set(supplier.supplier_id, sortCurrencyTotals(supplier.totals || []))
  }
  return result
})
const supplierPaidReferenceCNYByID = computed(() => {
  const result = new Map<number, number>()
  for (const supplier of props.rechargeOverview?.suppliers || []) {
    result.set(supplier.supplier_id, Number(supplier.reference_cny_amount) || 0)
  }
  return result
})

const toggleArchive = async (row: SupplierCostRow) => {
  const nextStatus = row.supplierStatus === 'archived' ? 'active' : 'archived'
  if (nextStatus === 'archived' && row.bindingCount > 0) {
    archiveTarget.value = row
    return
  }
  await archiveSupplier(row, nextStatus)
}

const archiveMessage = computed(() => {
  const row = archiveTarget.value
  if (!row) return ''
  return t('admin.accounts.upstreamCost.archiveSupplierConfirm', {
    name: row.supplierName,
    count: row.bindingCount
  })
})

const confirmSupplierArchive = async () => {
  const row = archiveTarget.value
  if (!row) return
  archiveTarget.value = null
  await archiveSupplier(row, 'archived')
}

const archiveSupplier = async (row: SupplierCostRow, nextStatus: 'active' | 'archived') => {
  supplierMutating.value = true
  try {
    await adminAPI.accounts.updateUpstreamSupplier(row.supplierID, { status: nextStatus })
    appStore.showSuccess(
      nextStatus === 'archived'
        ? t('admin.accounts.upstreamCost.supplierArchived')
        : t('admin.accounts.upstreamCost.supplierUnarchived')
    )
    emit('refresh', { forcePools: true })
  } catch (error: any) {
    appStore.showError(mapSupplierError(error, 'supplierUpdateFailed'))
  } finally {
    supplierMutating.value = false
  }
}

const openSupplierDelete = (row: SupplierCostRow) => {
  if (row.bindingCount > 0) {
    appStore.showWarning(t('admin.accounts.upstreamCost.errors.hasBoundAccounts'))
    return
  }
  deleteTarget.value = row
}

const deleteMessage = computed(() => {
  const row = deleteTarget.value
  if (!row) return ''
  return t('admin.accounts.upstreamCost.deleteSupplierConfirm', { name: row.supplierName })
})

const confirmSupplierDelete = async () => {
  const row = deleteTarget.value
  if (!row) return
  supplierMutating.value = true
  try {
    await adminAPI.accounts.deleteUpstreamSupplier(row.supplierID)
    appStore.showSuccess(t('admin.accounts.upstreamCost.supplierDeleted'))
    deleteTarget.value = null
    emit('refresh', { forcePools: true })
  } catch (error: any) {
    appStore.showError(mapSupplierError(error, 'supplierDeleteFailed'))
  } finally {
    supplierMutating.value = false
  }
}

const mapSupplierError = (error: any, fallbackKey: string): string => {
  const code = extractApiErrorCode(error)
  switch (code) {
    case 'SUPPLIER_NAME_CONFLICT':
      return t('admin.accounts.upstreamCost.errors.nameConflict')
    case 'SUPPLIER_RESERVED':
      return t('admin.accounts.upstreamCost.errors.reserved')
    case 'SUPPLIER_HAS_BOUND_ACCOUNTS':
      return t('admin.accounts.upstreamCost.errors.hasBoundAccounts')
    case 'SUPPLIER_HAS_BINDING_HISTORY':
      return t('admin.accounts.upstreamCost.errors.hasBindingHistory')
    case 'SUPPLIER_HAS_COST_DATA':
      return t('admin.accounts.upstreamCost.errors.hasCostData')
    default:
      return error?.message || t(`admin.accounts.upstreamCost.${fallbackKey}`)
  }
}

const rows = computed<SupplierCostRow[]>(() => {
  const bySupplier = new Map<number, SupplierCostRow>()
  const systemSupplierIDs = new Set(
    props.suppliers
      .filter((supplier) => supplier.is_system === true)
      .map((supplier) => supplier.id)
  )

  for (const supplier of props.suppliers) {
    if (systemSupplierIDs.has(supplier.id)) {
      continue
    }
    bySupplier.set(supplier.id, {
      supplierID: supplier.id,
      supplierName: supplier.name,
      supplierStatus: supplier.status,
      supplierNote: supplier.note,
      isSystem: supplier.is_system === true,
      supplier,
      pools: [],
      pool: null,
      showPoolName: false,
      bindingCount: 0,
      recordCount: 0,
      paidReferenceCNY: 0,
      paidTotals: []
    })
  }

  for (const pool of props.costPools) {
    if (systemSupplierIDs.has(pool.supplier_id)) {
      continue
    }
    if (!bySupplier.has(pool.supplier_id)) {
      bySupplier.set(pool.supplier_id, {
        supplierID: pool.supplier_id,
        supplierName: pool.supplier_name,
        supplierStatus: pool.archived_at ? 'archived' : pool.status,
        isSystem: false,
        supplier: {
          id: pool.supplier_id,
          name: pool.supplier_name,
          status: pool.archived_at ? 'archived' : pool.status,
          created_at: pool.created_at,
          updated_at: pool.updated_at
        },
        pools: [],
        pool: null,
        showPoolName: false,
        bindingCount: 0,
        recordCount: 0,
        paidReferenceCNY: 0,
        paidTotals: []
      })
    }
    bySupplier.get(pool.supplier_id)!.pools.push(pool)
  }

  return [...bySupplier.values()]
    .map((row) => {
      const pools = [...row.pools].sort((a, b) => {
        const activeDelta = Number(isActivePool(b)) - Number(isActivePool(a))
        if (activeDelta !== 0) return activeDelta
        const defaultDelta = Number(Boolean(b.is_default)) - Number(Boolean(a.is_default))
        if (defaultDelta !== 0) return defaultDelta
        const costDelta = Number(Boolean(b.current_snapshot_id && b.current_effective_cny_per_usd)) -
          Number(Boolean(a.current_snapshot_id && a.current_effective_cny_per_usd))
        if (costDelta !== 0) return costDelta
        if (b.binding_count !== a.binding_count) return b.binding_count - a.binding_count
        return a.id - b.id
      })
      return {
        ...row,
        pools,
        pool: pools[0] || null,
        showPoolName: pools.filter(isActivePool).length > 1,
        bindingCount: pools.reduce((sum, pool) => sum + (pool.binding_count || 0), 0),
        recordCount: pools.reduce((sum, pool) => sum + (pool.record_count || 0), 0),
        paidReferenceCNY: supplierPaidReferenceCNYByID.value.get(row.supplierID) || 0,
        paidTotals: supplierPaidTotalsByID.value.get(row.supplierID) || []
      }
    })
    .sort((a, b) => {
      if (a.supplierStatus !== b.supplierStatus) return a.supplierStatus.localeCompare(b.supplierStatus)
      return a.supplierName.localeCompare(b.supplierName)
    })
})

const sortedRows = computed(() => {
  if (!sortKey.value) return rows.value
  const direction = sortOrder.value === 'asc' ? 1 : -1
  return [...rows.value].sort((left, right) => {
    const leftMissing = isSortValueMissing(left, sortKey.value!)
    const rightMissing = isSortValueMissing(right, sortKey.value!)
    if (leftMissing !== rightMissing) return leftMissing ? 1 : -1
    const result = compareSupplierRows(left, right, sortKey.value!)
    if (result !== 0) return result * direction
    return left.supplierName.localeCompare(right.supplierName) || left.supplierID - right.supplierID
  })
})

const toggleSort = (key: SupplierSortKey) => {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
    return
  }
  sortKey.value = key
  sortOrder.value = 'asc'
}

const sortIcon = (key: SupplierSortKey): 'arrowUp' | 'arrowDown' | 'arrowsUpDown' => {
  if (sortKey.value !== key) return 'arrowsUpDown'
  return sortOrder.value === 'asc' ? 'arrowUp' : 'arrowDown'
}

const sortAria = (key: SupplierSortKey): 'ascending' | 'descending' | 'none' => {
  if (sortKey.value !== key) return 'none'
  return sortOrder.value === 'asc' ? 'ascending' : 'descending'
}

const compareSupplierRows = (left: SupplierCostRow, right: SupplierCostRow, key: SupplierSortKey): number => {
  switch (key) {
    case 'supplier':
      return left.supplierName.localeCompare(right.supplierName)
    case 'boundAccounts':
      return left.bindingCount - right.bindingCount
    case 'currentCost':
      return compareOptionalNumbers(currentEffectiveCost(left), currentEffectiveCost(right))
    case 'rechargeRatio':
    case 'poolDiscount':
      return compareOptionalNumbers(discountFactor(left), discountFactor(right))
    case 'totalPaid':
      return left.paidReferenceCNY - right.paidReferenceCNY
    case 'records':
      return left.recordCount - right.recordCount
    case 'walletBalance':
      return compareOptionalNumbers(walletBalanceValue(left), walletBalanceValue(right))
    case 'status':
      return statusSortRank(left) - statusSortRank(right)
  }
}

const isSortValueMissing = (row: SupplierCostRow, key: SupplierSortKey): boolean => {
  if (key === 'currentCost') return currentEffectiveCost(row) == null
  if (key === 'rechargeRatio' || key === 'poolDiscount') return !Number.isFinite(discountFactor(row))
  if (key === 'walletBalance') return walletBalanceValue(row) == null
  return false
}

const compareOptionalNumbers = (left: number | null | undefined, right: number | null | undefined): number => {
  if (left == null && right == null) return 0
  if (left == null) return 1
  if (right == null) return -1
  const leftNumber = Number(left)
  const rightNumber = Number(right)
  const leftValid = Number.isFinite(leftNumber)
  const rightValid = Number.isFinite(rightNumber)
  if (leftValid && rightValid) return leftNumber - rightNumber
  if (leftValid) return -1
  if (rightValid) return 1
  return 0
}

const statusSortRank = (row: SupplierCostRow) => {
  if (row.supplierStatus === 'archived' || row.pool?.archived_at) return 3
  if (!row.pool) return 2
  if (!hasCurrentCost(row)) return 1
  return 0
}

const discountFactor = (row: SupplierCostRow) => {
  if (!row.pool?.current_snapshot_id) return Number.POSITIVE_INFINITY
  const cost = row.pool?.current_effective_cny_per_usd
  const fx = row.pool?.reference_fx_rate
  if (!Number.isFinite(Number(cost)) || !Number.isFinite(Number(fx)) || Number(fx) <= 0) {
    return Number.POSITIVE_INFINITY
  }
  return Number(cost) / Number(fx)
}

const hasCurrentCost = (row: SupplierCostRow) => Boolean(
  row.pool?.current_snapshot_id && Number.isFinite(Number(row.pool.current_effective_cny_per_usd))
)

const currentEffectiveCost = (row: SupplierCostRow) => (
  hasCurrentCost(row) ? row.pool?.current_effective_cny_per_usd : null
)

const discountLabel = (row: SupplierCostRow) => {
  const factor = discountFactor(row)
  if (!Number.isFinite(factor)) return '-'
  return formatUpstreamDiscountLabel(factor * 10, {
    suffix: t('admin.accounts.upstreamCost.discountSuffix'),
    notConfiguredLabel: t('admin.accounts.upstreamCost.notConfigured'),
    fractionDigits: 2
  })
}

const formatRatio = (value?: number | null) => formatUpstreamRatio(Number(value))

const formatCost = (value?: number | null) => {
  if (!Number.isFinite(Number(value))) return '-'
  return `${formatRatio(value)} CNY/USD`
}

function sortCurrencyTotals(totals: UpstreamRechargeCurrencyTotal[]) {
  return [...totals]
    .filter((total) => Number(total.amount) !== 0 || Number(total.record_count) > 0)
    .sort((a, b) => a.currency.localeCompare(b.currency))
}

function sortConsumptionTotals(totals: UpstreamSupplierConsumptionTotal[]) {
  return [...totals]
    .filter((total) => Number.isFinite(Number(total.amount)) && Number(total.supplier_count) > 0)
    .sort((a, b) => a.unit.localeCompare(b.unit))
}

const buildRechargeOverviewCard = () => ({
  key: 'recharge',
  title: t('admin.accounts.upstreamCost.overview.rechargeTitle'),
  incomplete: false,
  details: '',
  totals: overviewTotals.value.map((total) => ({
    key: total.currency,
    amount: formatMoney(total.amount, total.currency),
    meta: t('admin.accounts.upstreamCost.recordCountBadge', { count: total.record_count })
  })),
  empty: t('admin.accounts.upstreamCost.overview.noRechargeData')
})

const buildConsumptionOverviewCard = (key: 'today' | 'last7', period: UpstreamSupplierConsumptionPeriod | null) => {
  const timezone = consumptionOverview.value?.timezone || ''
  const totals = period ? sortConsumptionTotals(period.totals || []) : []
  const coverage = period ? formatCoverage(period) : t('admin.accounts.upstreamCost.overview.consumptionUnknown')
  const issueSummary = period ? formatIssueSummary(period.issues || []) : ''
  const title = key === 'today'
    ? t('admin.accounts.upstreamCost.overview.todayConsumptionTitle')
    : t('admin.accounts.upstreamCost.overview.last7ConsumptionTitle')
  return {
    key,
    title,
    incomplete: !!period && (period.complete_supplier_count < period.supplier_count || !!issueSummary),
    details: period
      ? [t('admin.accounts.upstreamCost.overview.estimatedBadge'), `${coverage}${formatPeriodRange(period)} ${timezone}`.trim(), t('admin.accounts.upstreamCost.overview.estimateFormula'), issueSummary].filter(Boolean).join('\n')
      : t('admin.accounts.upstreamCost.overview.consumptionUnknownHint'),
    totals: totals.map((total) => ({
      key: total.unit,
      amount: formatMoney(total.amount, total.unit),
      meta: t('admin.accounts.upstreamCost.overview.supplierCountBadge', { count: total.supplier_count })
    })),
    empty: period ? emptyConsumptionLabel(period) : t('admin.accounts.upstreamCost.overview.consumptionUnknown')
  }
}

const formatCoverage = (period: UpstreamSupplierConsumptionPeriod) => {
  const supplierCount = Number(period.supplier_count) || 0
  const coveredCount = Number(period.covered_supplier_count) || 0
  const completeCount = Number(period.complete_supplier_count) || 0
  const base = t('admin.accounts.upstreamCost.overview.coverage', {
    covered: coveredCount,
    total: supplierCount
  })
  if (supplierCount > 0 && completeCount < supplierCount) {
    return `${base} · ${t('admin.accounts.upstreamCost.overview.incomplete')}`
  }
  return base
}

const formatPeriodRange = (period: UpstreamSupplierConsumptionPeriod) => {
  const start = formatShortDate(period.start_at, consumptionOverview.value?.timezone)
  const end = formatShortDate(period.end_at, consumptionOverview.value?.timezone)
  if (!start || !end) return ''
  return ` · ${start}-${end}`
}

const emptyConsumptionLabel = (period: UpstreamSupplierConsumptionPeriod) => {
  if ((Number(period.supplier_count) || 0) <= 0) {
    return t('admin.accounts.upstreamCost.overview.noAvailableData')
  }
  return t('admin.accounts.upstreamCost.overview.accumulatingData')
}

const formatIssueSummary = (issues: UpstreamSupplierConsumptionIssue[]) => {
  if (issues.length === 0) return ''
  const counts = new Map<string, number>()
  for (const issue of issues) {
    const reason = issue.reason || 'no_history'
    counts.set(reason, (counts.get(reason) || 0) + 1)
  }
  return [...counts.entries()]
    .sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0]))
    .map(([reason, count]) => t('admin.accounts.upstreamCost.overview.issueCount', {
      reason: consumptionIssueReasonLabel(reason),
      count
    }))
    .join(' · ')
}

const consumptionIssueReasonLabel = (reason: string) => {
  const key = `admin.accounts.upstreamCost.overview.issueReasons.${reason}`
  const label = t(key)
  return label === key ? reason : label
}

const formatAmount = (value: number) => {
  const amount = Number(value)
  if (!Number.isFinite(amount)) return '0'
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2
  }).format(amount)
}

const formatMoney = (amount: number, currency: string) => `${formatAmount(amount)} ${currency || '-'}`

const formatShortDate = (value?: string | null, timeZone?: string | null) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  if (timeZone) {
    try {
      const parts = new Intl.DateTimeFormat(undefined, {
        timeZone,
        month: '2-digit',
        day: '2-digit'
      }).formatToParts(date)
      const month = parts.find(part => part.type === 'month')?.value
      const day = parts.find(part => part.type === 'day')?.value
      if (month && day) return `${month}/${day}`
    } catch {
      // Older API responses may omit or send an invalid timezone; fall back to the browser locale below.
    }
  }
  return `${String(date.getMonth() + 1).padStart(2, '0')}/${String(date.getDate()).padStart(2, '0')}`
}

const formatUSD = (amount: number) => {
  if (!Number.isFinite(amount)) return '-'
  return `$${new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(amount)}`
}

const formatBalanceAmount = (amount: number, unit?: string | null) => {
  if (unit === 'Credits') {
    return `${formatAmount(amount)} Credits`
  }
  return `${formatUSD(amount)} USD`
}

const walletBalanceValue = (row: SupplierCostRow) => {
  const value = row.supplier.balance_snapshot?.balance_usd
  if (value == null) return null
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : null
}

const formatBalanceTime = (value?: string | null) => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return `${String(date.getMonth() + 1).padStart(2, '0')}/${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

const isBalanceExplicitlyDisabled = (row: SupplierCostRow) => row.supplier.balance_config?.enabled === false

const balanceLabel = (row: SupplierCostRow) => {
  if (!row.supplier.balance_config) {
    return t('admin.accounts.upstreamCost.supplierBalance.unknown')
  }
  if (isBalanceExplicitlyDisabled(row)) {
    return t('admin.accounts.upstreamCost.supplierBalance.disabled')
  }
  const value = walletBalanceValue(row)
  if (row.supplier.balance_snapshot?.status === 'error') {
    return t('admin.accounts.upstreamCost.supplierBalance.failed')
  }
  if (value == null) {
    return t('admin.accounts.upstreamCost.supplierBalance.pending')
  }
  return formatBalanceAmount(value, row.supplier.balance_snapshot?.unit)
}

const balanceTimeHint = (row: SupplierCostRow) => {
  if (!row.supplier.balance_config) {
    return t('admin.accounts.upstreamCost.supplierBalance.detailsNotLoaded')
  }
  if (isBalanceExplicitlyDisabled(row)) {
    return t('admin.accounts.upstreamCost.supplierBalance.disabledHint')
  }
  const snapshot = row.supplier.balance_snapshot
  if (!snapshot?.last_attempt_at) {
    return t('admin.accounts.upstreamCost.supplierBalance.notFetched')
  }
  if (snapshot.status === 'error') {
    const lastSuccessTime = formatBalanceTime(snapshot.updated_at)
    if (walletBalanceValue(row) != null && lastSuccessTime) {
      return t('admin.accounts.upstreamCost.supplierBalance.lastSuccess', {
        amount: formatBalanceAmount(walletBalanceValue(row)!, snapshot.unit),
        time: lastSuccessTime
      })
    }
    return ''
  }
  return t('admin.accounts.upstreamCost.supplierBalance.updatedAt', {
    time: formatBalanceTime(snapshot.updated_at || snapshot.last_attempt_at)
  })
}

const balanceErrorHint = (row: SupplierCostRow) => {
  if (row.supplier.balance_config?.enabled !== true || row.supplier.balance_snapshot?.status !== 'error') {
    return ''
  }
  return balanceErrorReason(row.supplier.balance_snapshot.error)
}

const balanceErrorReason = (reason?: string | null) => {
  const trimmed = String(reason || '').trim()
  return trimmed || t('admin.accounts.upstreamCost.supplierBalance.refreshFailed')
}

const balanceValueClass = (row: SupplierCostRow) => {
  const base = 'font-mono text-sm font-semibold'
  if (row.supplier.balance_snapshot?.status === 'error') {
    return `${base} text-red-700 dark:text-red-300`
  }
  if (walletBalanceValue(row) != null) {
    return `${base} text-stone-800 dark:text-stone-100`
  }
  return `${base} text-stone-400 dark:text-stone-500`
}

const statusText = (row: SupplierCostRow) => {
  if (!row.pool) return t('admin.accounts.upstreamCost.supplierNoPool')
  if (row.supplierStatus === 'archived' || row.pool.archived_at) return t('admin.accounts.upstreamCost.archivedStatus')
  if (hasCurrentCost(row)) return t('admin.accounts.upstreamCost.completeStatus')
  return t('admin.accounts.upstreamCost.needsConfig')
}

const statusBadgeClass = (row: SupplierCostRow) => {
  const base = 'rounded-md px-2 py-1 text-xs font-medium'
  if (hasCurrentCost(row)) {
    return `${base} bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300`
  }
  if (!row.pool) {
    return `${base} bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300`
  }
  return `${base} bg-stone-100 text-stone-500 dark:bg-white/[0.07] dark:text-stone-400`
}

const discountBadgeClass = (row: SupplierCostRow) => {
  const base = 'rounded-md px-2 py-1 font-mono text-xs font-semibold'
  const factor = discountFactor(row)
  if (!Number.isFinite(factor)) {
    return `${base} bg-stone-100 text-stone-500 dark:bg-white/[0.07] dark:text-stone-400`
  }
  if (factor <= 0.3) {
    return `${base} bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300`
  }
  if (factor <= 0.7) {
    return `${base} bg-sky-50 text-sky-700 dark:bg-sky-500/10 dark:text-sky-300`
  }
  return `${base} bg-stone-100 text-stone-700 dark:bg-white/[0.07] dark:text-stone-300`
}

const rowDotClass = (row: SupplierCostRow) => {
  if (hasCurrentCost(row)) return 'bg-emerald-500'
  if (row.pool) return 'bg-amber-500'
  return 'bg-stone-300 dark:bg-stone-600'
}
</script>

<style scoped>
.sortable-header {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 0.375rem;
  white-space: nowrap;
  color: inherit;
  transition: color 120ms ease;
}

.sortable-header:hover {
  color: rgb(87 83 78);
}

:global(.dark) .sortable-header:hover {
  color: rgb(245 245 244);
}
</style>
