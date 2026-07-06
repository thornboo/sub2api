<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl border border-stone-200/80 bg-white shadow-sm shadow-stone-950/5 dark:border-white/10 dark:bg-stone-950/70 dark:shadow-black/20">
    <div class="border-b border-stone-200/80 bg-white px-4 py-3 dark:border-white/10 dark:bg-stone-950">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="min-w-0">
          <h3 class="truncate text-base font-semibold text-stone-950 dark:text-white">
            {{ t('admin.accounts.upstreamCost.supplierListTitle') }}
          </h3>
          <p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
            {{ t('admin.accounts.upstreamCost.supplierListDescription') }}
          </p>
        </div>
        <button
          type="button"
          class="btn btn-secondary h-10 justify-center px-3"
          :disabled="loading"
          @click="$emit('refresh')"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div class="grid border-b border-stone-200 bg-stone-50/70 dark:border-white/10 dark:bg-white/[0.025] md:grid-cols-3">
      <div class="min-w-0 border-b border-stone-200 px-4 py-3 dark:border-white/10 md:border-b-0 md:border-r">
        <div class="text-xs font-medium text-stone-500 dark:text-stone-400">
          {{ t('admin.accounts.upstreamCost.supplierCount') }}
        </div>
        <div class="mt-1 font-mono text-2xl font-semibold leading-none text-stone-950 dark:text-white">
          {{ rows.length }}
        </div>
      </div>
      <div class="min-w-0 border-b border-stone-200 px-4 py-3 dark:border-white/10 md:border-b-0 md:border-r">
        <div class="text-xs font-medium text-stone-500 dark:text-stone-400">
          {{ t('admin.accounts.upstreamCost.configuredSuppliers') }}
        </div>
        <div class="mt-1 flex items-baseline gap-2">
          <span class="font-mono text-2xl font-semibold leading-none text-stone-950 dark:text-white">{{ configuredRows.length }}</span>
          <span class="text-sm text-stone-500 dark:text-stone-400">/ {{ rows.length }}</span>
        </div>
      </div>
      <div class="min-w-0 px-4 py-3">
        <div class="text-xs font-medium text-stone-500 dark:text-stone-400">
          {{ t('admin.accounts.upstreamCost.bestSupplier') }}
        </div>
        <div class="mt-1 flex min-w-0 items-baseline gap-2">
          <span class="font-mono text-2xl font-semibold leading-none text-stone-950 dark:text-white">{{ bestRow ? discountLabel(bestRow) : '-' }}</span>
          <span class="truncate text-sm text-stone-500 dark:text-stone-400">{{ bestRow?.supplierName || '-' }}</span>
        </div>
      </div>
    </div>

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
            <th class="px-4 py-3 text-left text-[13px] font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.rechargeDiscount') }}</th>
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
                      <span v-if="row.pool" class="rounded-md bg-stone-100 px-2 py-0.5 text-xs font-medium text-stone-500 dark:bg-white/[0.07] dark:text-stone-400">
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
                {{ formatCost(row.pool?.current_effective_cny_per_usd) }}
              </td>
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                <span v-if="row.pool?.current_effective_cny_per_usd">
                  {{ formatRatio(row.pool.current_effective_cny_per_usd) }}
                  <span class="text-stone-400">/</span>
                  {{ formatRatio(row.pool.reference_fx_rate) }}
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
              <td class="px-4 py-4 text-right">
                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:border-emerald-300 hover:text-emerald-700 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-emerald-500/40 dark:hover:text-emerald-300"
                  :disabled="!row.pool"
                  @click="row.pool && $emit('recharge-records', row.pool)"
                >
                  <Icon name="creditCard" size="xs" />
                  {{ t('admin.accounts.upstreamCost.rechargeRecords.action') }}
                </button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UpstreamCostPool, UpstreamSupplier } from '@/api/admin/accounts'
import Icon from '@/components/icons/Icon.vue'
import { formatUpstreamDiscountLabel, formatUpstreamRatio } from '@/utils/upstreamCost'

interface SupplierCostRow {
  supplierID: number
  supplierName: string
  supplierStatus: string
  supplierNote?: string | null
  pools: UpstreamCostPool[]
  pool: UpstreamCostPool | null
  bindingCount: number
  recordCount: number
}

const props = defineProps<{
  suppliers: UpstreamSupplier[]
  costPools: UpstreamCostPool[]
  loading?: boolean
  error?: string | null
}>()

defineEmits<{
  refresh: []
  'recharge-records': [pool: UpstreamCostPool]
}>()

const { t } = useI18n()

const rows = computed<SupplierCostRow[]>(() => {
  const bySupplier = new Map<number, SupplierCostRow>()

  for (const supplier of props.suppliers) {
    bySupplier.set(supplier.id, {
      supplierID: supplier.id,
      supplierName: supplier.name,
      supplierStatus: supplier.status,
      supplierNote: supplier.note,
      pools: [],
      pool: null,
      bindingCount: 0,
      recordCount: 0
    })
  }

  for (const pool of props.costPools) {
    if (!bySupplier.has(pool.supplier_id)) {
      bySupplier.set(pool.supplier_id, {
        supplierID: pool.supplier_id,
        supplierName: pool.supplier_name,
        supplierStatus: pool.archived_at ? 'archived' : pool.status,
        pools: [],
        pool: null,
        bindingCount: 0,
        recordCount: 0
      })
    }
    bySupplier.get(pool.supplier_id)!.pools.push(pool)
  }

  return [...bySupplier.values()]
    .map((row) => {
      const pools = [...row.pools].sort((a, b) => {
        const activeDelta = Number(b.status === 'active') - Number(a.status === 'active')
        if (activeDelta !== 0) return activeDelta
        const costDelta = Number(Boolean(b.current_effective_cny_per_usd)) - Number(Boolean(a.current_effective_cny_per_usd))
        if (costDelta !== 0) return costDelta
        if (b.binding_count !== a.binding_count) return b.binding_count - a.binding_count
        return a.id - b.id
      })
      return {
        ...row,
        pools,
        pool: pools[0] || null,
        bindingCount: pools.reduce((sum, pool) => sum + (pool.binding_count || 0), 0),
        recordCount: pools.reduce((sum, pool) => sum + (pool.record_count || 0), 0)
      }
    })
    .sort((a, b) => {
      if (a.supplierStatus !== b.supplierStatus) return a.supplierStatus.localeCompare(b.supplierStatus)
      return a.supplierName.localeCompare(b.supplierName)
    })
})

const configuredRows = computed(() => rows.value.filter(row => Boolean(row.pool?.current_effective_cny_per_usd)))
const bestRow = computed(() => {
  return [...configuredRows.value].sort((a, b) => discountFactor(a) - discountFactor(b))[0] || null
})

const discountFactor = (row: SupplierCostRow) => {
  const cost = row.pool?.current_effective_cny_per_usd
  const fx = row.pool?.reference_fx_rate
  if (!Number.isFinite(Number(cost)) || !Number.isFinite(Number(fx)) || Number(fx) <= 0) {
    return Number.POSITIVE_INFINITY
  }
  return Number(cost) / Number(fx)
}

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
  if (row.pool.current_effective_cny_per_usd) return t('admin.accounts.upstreamCost.completeStatus')
  return t('admin.accounts.upstreamCost.needsConfig')
}

const statusBadgeClass = (row: SupplierCostRow) => {
  const base = 'rounded-md px-2 py-1 text-xs font-medium'
  if (row.pool?.current_effective_cny_per_usd) {
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
  if (row.pool?.current_effective_cny_per_usd) return 'bg-emerald-500'
  if (row.pool) return 'bg-amber-500'
  return 'bg-stone-300 dark:bg-stone-600'
}
</script>
