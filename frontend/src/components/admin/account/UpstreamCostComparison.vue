<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-stone-200/80 bg-white shadow-sm shadow-stone-950/5 dark:border-white/10 dark:bg-stone-950/70 dark:shadow-black/20">
    <div v-if="error" class="m-4 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="min-h-0 flex-1 overflow-auto">
      <table class="min-w-[980px] divide-y divide-stone-200 text-sm dark:divide-white/10">
        <thead class="sticky top-0 z-10 bg-stone-50 dark:bg-stone-950">
          <tr>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.supplier') }}</th>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.boundAccounts') }}</th>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.currentCost') }}</th>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.rechargeRatio') }}</th>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.poolDiscountUSD') }}</th>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.rechargeRecords.records') }}</th>
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.status') }}</th>
            <th class="px-4 py-3 text-right text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-stone-100 dark:divide-white/[0.06]">
          <tr v-if="loading">
            <td colspan="8" class="px-4 py-10 text-center text-stone-500 dark:text-stone-400">
              {{ t('common.loading') }}...
            </td>
          </tr>
          <tr v-else-if="rows.length === 0">
            <td colspan="8" class="px-4 py-10 text-center text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.noSuppliers') }}
            </td>
          </tr>
          <template v-else>
            <tr
              v-for="row in rows"
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
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                {{ row.recordCount }}
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
import type { UpstreamCostPool, UpstreamSupplier } from '@/api/admin/accounts'
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
  pools: UpstreamCostPool[]
  pool: UpstreamCostPool | null
  showPoolName: boolean
  bindingCount: number
  recordCount: number
}

const props = defineProps<{
  suppliers: UpstreamSupplier[]
  costPools: UpstreamCostPool[]
  loading?: boolean
  error?: string | null
}>()

const emit = defineEmits<{
  refresh: [options?: { forcePools?: boolean }]
  'recharge-records': [pool: UpstreamCostPool]
  'edit-supplier': [supplierID: number]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const supplierMutating = ref(false)
const archiveTarget = ref<SupplierCostRow | null>(null)
const deleteTarget = ref<SupplierCostRow | null>(null)

const isReserved = (row: SupplierCostRow) => row.isSystem
const isActivePool = (pool: UpstreamCostPool) => pool.status === 'active' && !pool.archived_at

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
      pools: [],
      pool: null,
      showPoolName: false,
      bindingCount: 0,
      recordCount: 0
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
        pools: [],
        pool: null,
        showPoolName: false,
        bindingCount: 0,
        recordCount: 0
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
        recordCount: pools.reduce((sum, pool) => sum + (pool.record_count || 0), 0)
      }
    })
    .sort((a, b) => {
      if (a.supplierStatus !== b.supplierStatus) return a.supplierStatus.localeCompare(b.supplierStatus)
      return a.supplierName.localeCompare(b.supplierName)
    })
})

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
    notConfiguredLabel: t('admin.accounts.upstreamCost.notConfigured')
  })
}

const formatRatio = (value?: number | null) => formatUpstreamRatio(Number(value))

const formatCost = (value?: number | null) => {
  if (!Number.isFinite(Number(value))) return '-'
  return `${formatRatio(value)} CNY/USD`
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
