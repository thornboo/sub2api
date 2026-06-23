<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl border border-stone-200/80 bg-white shadow-sm shadow-stone-950/5 dark:border-white/10 dark:bg-stone-950/55 dark:shadow-black/20">
    <div class="border-b border-stone-200/80 bg-stone-50/80 px-4 py-3 dark:border-white/10 dark:bg-white/[0.035]">
      <div class="flex flex-col gap-3">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div class="flex min-w-0 items-center gap-3">
            <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl border border-emerald-200/80 bg-emerald-50 text-emerald-700 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-300">
              <Icon name="dollar" size="sm" />
            </div>
            <h3 class="truncate text-lg font-semibold text-stone-950 dark:text-white">
              {{ t('admin.accounts.upstreamCost.comparisonTitle') }}
            </h3>
            <span class="rounded-full border border-sky-200 bg-sky-50 px-2 py-0.5 text-xs font-medium text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-300">
              {{ t('admin.accounts.upstreamCost.readonlyBadge') }}
            </span>
          </div>

          <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
            <div class="w-full sm:w-56">
              <Select
                v-model="selectedFamily"
                :options="familySelectOptions"
                :searchable="familySelectOptions.length > 6"
              >
                <template #selected="{ option }">
                  <span class="flex min-w-0 items-center gap-2">
                    <Icon name="filter" size="xs" class="flex-shrink-0 text-sky-500" />
                    <span class="truncate">{{ option?.label || selectedFamilyLabel }}</span>
                  </span>
                </template>
                <template #option="{ option, selected }">
                  <span class="flex min-w-0 items-center gap-2">
                    <span class="h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="option.value === DEFAULT_UPSTREAM_COST_FAMILY ? 'bg-stone-400' : 'bg-sky-500'" />
                    <span class="select-option-label">{{ option.label }}</span>
                  </span>
                  <Icon v-if="selected" name="check" size="sm" class="flex-shrink-0 text-emerald-500" />
                </template>
              </Select>
            </div>
            <button
              type="button"
              class="btn btn-secondary h-11 justify-center px-3 sm:w-auto"
              :disabled="loading"
              @click="$emit('refresh')"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>

        <div class="grid overflow-hidden rounded-xl border border-stone-200 bg-white dark:border-white/10 dark:bg-stone-950/45 md:grid-cols-[minmax(0,1.15fr)_minmax(0,1fr)_minmax(0,1fr)]">
          <div class="min-w-0 border-b border-stone-200 px-4 py-3 dark:border-white/10 md:border-b-0 md:border-r">
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs font-medium text-sky-700 dark:text-sky-300">{{ t('admin.accounts.upstreamCost.bestDiscount') }}</span>
              <Icon name="trendingUp" size="sm" class="text-sky-500" />
            </div>
            <div class="mt-1 flex min-w-0 items-baseline gap-2">
              <span class="text-2xl font-semibold leading-none text-stone-950 dark:text-white">{{ bestRow?.calculation.label || '-' }}</span>
              <span class="truncate text-sm text-stone-500 dark:text-stone-400">{{ bestAccountLabel }}</span>
            </div>
          </div>

          <div class="min-w-0 border-b border-stone-200 px-4 py-3 dark:border-white/10 md:border-b-0 md:border-r">
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs font-medium text-emerald-700 dark:text-emerald-300">{{ t('admin.accounts.upstreamCost.configuredAccounts') }}</span>
              <span class="font-mono text-xs font-semibold text-stone-500 dark:text-stone-400">{{ coverageLabel }}</span>
            </div>
            <div class="mt-1 flex items-baseline gap-2">
              <span class="text-2xl font-semibold leading-none text-stone-950 dark:text-white">{{ completeCount }} / {{ accounts.length }}</span>
              <span class="text-sm text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.configCoverage') }}</span>
            </div>
            <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-stone-200 dark:bg-white/10">
              <div class="h-full rounded-full bg-emerald-500 transition-all duration-300" :style="coverageBarStyle" />
            </div>
          </div>

          <div class="min-w-0 px-4 py-3">
            <div class="flex items-center justify-between gap-3">
              <span class="text-xs font-medium text-amber-700 dark:text-amber-300">{{ t('admin.accounts.upstreamCost.selectedFamily') }}</span>
              <Icon name="chartBar" size="sm" class="text-amber-500" />
            </div>
            <div class="mt-1 flex min-w-0 items-baseline gap-2">
              <span class="truncate text-2xl font-semibold leading-none text-stone-950 dark:text-white">{{ selectedFamilyLabel }}</span>
              <span class="whitespace-nowrap text-sm text-stone-500 dark:text-stone-400">{{ configuredCount }} {{ t('admin.accounts.upstreamCost.configuredStatus') }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="error" class="m-4 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
      {{ error }}
    </div>

    <div class="min-h-0 flex-1 overflow-auto">
      <table class="min-w-[1080px] divide-y divide-stone-200 text-sm dark:divide-white/10">
        <thead class="sticky top-0 z-10 bg-white/95 backdrop-blur dark:bg-stone-950/95">
          <tr>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.name') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.platformType') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.groups') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.priority') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.rechargeRatio') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.multiplier') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.effectiveDiscount') }}</th>
            <th class="px-4 py-3 text-left text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.status') }}</th>
            <th class="px-4 py-3 text-right text-xs font-semibold uppercase text-stone-500 dark:text-stone-400">{{ t('admin.accounts.columns.actions') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-stone-100 dark:divide-white/[0.06]">
          <tr v-if="loading">
            <td colspan="9" class="px-4 py-10 text-center text-stone-500 dark:text-stone-400">
              {{ t('common.loading') }}...
            </td>
          </tr>
          <tr v-else-if="rows.length === 0">
            <td colspan="9" class="px-4 py-10 text-center text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.noAccounts') }}
            </td>
          </tr>
          <template v-else>
            <tr
              v-for="row in rows"
              :key="row.account.id"
              :class="rowClass(row)"
            >
              <td class="px-4 py-4">
                <div class="flex items-start gap-3">
                  <div class="mt-1 h-2.5 w-2.5 flex-shrink-0 rounded-full" :class="rowDotClass(row)" />
                  <div class="min-w-0">
                    <div class="flex flex-wrap items-center gap-2">
                      <span class="font-semibold text-stone-950 dark:text-white">{{ row.account.name }}</span>
                      <span v-if="row.account.id === bestAccountId" class="rounded-full bg-emerald-100 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">
                        {{ t('admin.accounts.upstreamCost.bestChoice') }}
                      </span>
                    </div>
                    <div v-if="row.calculation.note" class="mt-1 max-w-xs truncate text-xs text-stone-500 dark:text-stone-400" :title="row.calculation.note">
                      {{ row.calculation.note }}
                    </div>
                    <div v-else-if="!row.calculation.complete" class="mt-1 text-xs text-amber-700 dark:text-amber-300">
                      {{ t('admin.accounts.upstreamCost.missingPrefix') }} {{ missingSummary(row) }}
                    </div>
                  </div>
                </div>
              </td>
              <td class="px-4 py-4">
                <div class="flex flex-wrap gap-1.5">
                  <span class="rounded-md bg-stone-100 px-2 py-1 text-xs font-medium text-stone-700 dark:bg-white/[0.07] dark:text-stone-200">{{ row.account.platform }}</span>
                  <span class="rounded-md bg-sky-50 px-2 py-1 text-xs font-medium text-sky-700 dark:bg-sky-500/10 dark:text-sky-300">{{ row.account.type }}</span>
                </div>
              </td>
              <td class="px-4 py-4">
                <div v-if="row.groupNames.length" class="flex max-w-xs flex-wrap gap-1.5">
                  <span
                    v-for="group in row.groupNames"
                    :key="group"
                    class="rounded-md bg-stone-100 px-2 py-1 text-xs font-medium text-stone-600 dark:bg-white/[0.06] dark:text-stone-300"
                  >
                    {{ group }}
                  </span>
                </div>
                <span v-else class="text-xs text-stone-400 dark:text-stone-500">{{ t('admin.accounts.upstreamCost.noGroups') }}</span>
              </td>
              <td class="px-4 py-4">
                <span class="rounded-md border border-stone-200 bg-white px-2 py-1 font-mono text-xs font-semibold text-stone-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200">
                  #{{ row.account.priority }}
                </span>
              </td>
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                <span v-if="row.calculation.recharge_cny_per_usd && row.calculation.reference_fx_rate">
                  {{ formatRatio(row.calculation.recharge_cny_per_usd) }}
                  <span class="text-stone-400">/</span>
                  {{ formatRatio(row.calculation.reference_fx_rate) }}
                </span>
                <span v-else class="text-stone-400 dark:text-stone-500">-</span>
              </td>
              <td class="px-4 py-4">
                <div class="flex items-center gap-2">
                  <span class="font-mono text-stone-700 dark:text-stone-300">{{ formatRatio(row.calculation.group_multiplier) }}</span>
                  <span
                    v-if="row.calculation.source === 'family_override'"
                    class="rounded-md bg-sky-50 px-1.5 py-0.5 text-[11px] font-medium text-sky-700 dark:bg-sky-500/10 dark:text-sky-300"
                  >
                    {{ t('admin.accounts.upstreamCost.familyOverride') }}
                  </span>
                </div>
              </td>
              <td class="px-4 py-4 font-mono text-stone-700 dark:text-stone-300">
                {{ row.calculation.complete ? formatRatio(row.calculation.effective_discount) : '-' }}
              </td>
              <td class="px-4 py-4">
                <div class="flex flex-col items-start gap-1">
                  <span :class="statusBadgeClass(row)">
                    {{ statusText(row) }}
                  </span>
                  <span v-if="row.calculation.complete" :class="discountBadgeClass(row)">
                    {{ row.calculation.label }}
                  </span>
                  <span v-else class="text-xs text-stone-500 dark:text-stone-400">
                    {{ missingSummary(row) }}
                  </span>
                </div>
              </td>
              <td class="px-4 py-4 text-right">
                <div class="flex justify-end gap-2">
                  <button
                    type="button"
                    class="inline-flex items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:border-sky-300 hover:text-sky-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-sky-500/40 dark:hover:text-sky-300"
                    @click="$emit('recharge-records', row.account)"
                  >
                    <Icon name="creditCard" size="xs" />
                    {{ t('admin.accounts.upstreamCost.rechargeRecords.action') }}
                  </button>
                  <button
                    type="button"
                    class="inline-flex items-center gap-1.5 rounded-lg border border-stone-200 bg-white px-3 py-1.5 text-xs font-medium text-stone-700 transition-colors hover:border-emerald-300 hover:text-emerald-700 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-200 dark:hover:border-emerald-500/40 dark:hover:text-emerald-300"
                    @click="$emit('edit', row.account)"
                  >
                    {{ t('admin.accounts.upstreamCost.configureAction') }}
                    <Icon name="arrowRight" size="xs" />
                  </button>
                </div>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import {
  DEFAULT_UPSTREAM_COST_FAMILY,
  calculateUpstreamCost,
  formatUpstreamRatio,
  getUpstreamCostFamilies,
  readUpstreamCostProfile,
  type UpstreamCostCalculation,
  type UpstreamCostMissingField
} from '@/utils/upstreamCost'
import type { Account } from '@/types'

interface CostComparisonRow {
  account: Account
  calculation: UpstreamCostCalculation
  groupNames: string[]
}

const props = defineProps<{
  accounts: Account[]
  loading?: boolean
  error?: string | null
}>()

defineEmits<{
  refresh: []
  edit: [account: Account]
  'recharge-records': [account: Account]
}>()

const { t } = useI18n()
const selectedFamily = ref(DEFAULT_UPSTREAM_COST_FAMILY)

const familyOptions = computed(() => {
  const seen = new Map<string, string>()
  for (const account of props.accounts) {
    for (const family of getUpstreamCostFamilies(readUpstreamCostProfile(account.extra))) {
      const key = family.toLowerCase()
      if (seen.has(key)) continue
      seen.set(key, family)
    }
  }
  return [...seen.values()].sort((a, b) => a.localeCompare(b))
})

const familySelectOptions = computed(() => [
  { value: DEFAULT_UPSTREAM_COST_FAMILY, label: t('admin.accounts.upstreamCost.defaultFamily') },
  ...familyOptions.value.map(family => ({ value: family, label: family }))
])

watch(familyOptions, (options) => {
  if (selectedFamily.value !== DEFAULT_UPSTREAM_COST_FAMILY && !options.includes(selectedFamily.value)) {
    selectedFamily.value = DEFAULT_UPSTREAM_COST_FAMILY
  }
})

const selectedFamilyLabel = computed(() => (
  selectedFamily.value === DEFAULT_UPSTREAM_COST_FAMILY
    ? t('admin.accounts.upstreamCost.defaultFamily')
    : selectedFamily.value
))

const rows = computed<CostComparisonRow[]>(() => {
  return props.accounts
    .map((account) => {
      const profile = readUpstreamCostProfile(account.extra)
      const family = selectedFamily.value === DEFAULT_UPSTREAM_COST_FAMILY ? '' : selectedFamily.value
      const calculation = calculateUpstreamCost(profile, family)
      return {
        account,
        calculation,
        groupNames: (account.groups || []).map(group => group.name).filter(Boolean)
      }
    })
    .sort((a, b) => {
      if (a.calculation.complete !== b.calculation.complete) {
        return a.calculation.complete ? -1 : 1
      }
      const aDiscount = a.calculation.effective_discount ?? Number.POSITIVE_INFINITY
      const bDiscount = b.calculation.effective_discount ?? Number.POSITIVE_INFINITY
      if (aDiscount !== bDiscount) return aDiscount - bDiscount
      if (a.account.priority !== b.account.priority) return a.account.priority - b.account.priority
      return a.account.name.localeCompare(b.account.name)
    })
})

const configuredCount = computed(() => rows.value.filter(row => row.calculation.configured).length)
const completeCount = computed(() => rows.value.filter(row => row.calculation.complete).length)
const bestRow = computed(() => rows.value.find(row => row.calculation.complete))
const bestAccountId = computed(() => bestRow.value?.account.id ?? null)
const bestAccountLabel = computed(() => bestRow.value?.account.name || t('admin.accounts.upstreamCost.noBestAccount'))
const coverageLabel = computed(() => {
  if (props.accounts.length === 0) return '-'
  return `${Math.round((completeCount.value / props.accounts.length) * 100)}%`
})
const coverageBarStyle = computed(() => ({
  width: props.accounts.length === 0
    ? '0%'
    : `${Math.round((completeCount.value / props.accounts.length) * 100)}%`
}))

const missingFieldLabels = (fields: UpstreamCostMissingField[]) => fields.map((field) => {
  if (field === 'recharge_cny_per_usd') return t('admin.accounts.upstreamCost.rechargeCnyPerUsd')
  if (field === 'reference_fx_rate') return t('admin.accounts.upstreamCost.referenceFxRate')
  return t('admin.accounts.upstreamCost.groupMultiplier')
})

const missingSummary = (row: CostComparisonRow) => {
  const labels = missingFieldLabels(row.calculation.missing_fields)
  return labels.length > 0 ? labels.join('、') : '-'
}

const statusText = (row: CostComparisonRow) => {
  if (row.calculation.complete) return t('admin.accounts.upstreamCost.completeStatus')
  if (row.calculation.configured) return t('admin.accounts.upstreamCost.incompleteStatus')
  return t('admin.accounts.upstreamCost.needsConfig')
}

const rowClass = (row: CostComparisonRow) => {
  if (row.account.id === bestAccountId.value) {
    return 'bg-emerald-50/45 hover:bg-emerald-50/80 dark:bg-emerald-500/[0.045] dark:hover:bg-emerald-500/[0.08]'
  }
  if (!row.calculation.configured) {
    return 'bg-amber-50/20 hover:bg-amber-50/45 dark:bg-amber-500/[0.025] dark:hover:bg-amber-500/[0.05]'
  }
  if (!row.calculation.complete) {
    return 'bg-stone-50/55 hover:bg-stone-50 dark:bg-white/[0.02] dark:hover:bg-white/[0.045]'
  }
  return 'hover:bg-stone-50/70 dark:hover:bg-white/[0.035]'
}

const rowDotClass = (row: CostComparisonRow) => {
  if (row.account.id === bestAccountId.value) return 'bg-emerald-500'
  if (row.calculation.complete) return 'bg-sky-500'
  if (row.calculation.configured) return 'bg-amber-500'
  return 'bg-stone-300 dark:bg-stone-600'
}

const statusBadgeClass = (row: CostComparisonRow) => {
  const base = 'rounded-full px-2.5 py-1 text-xs font-semibold'
  if (row.calculation.complete) {
    return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300`
  }
  if (row.calculation.configured) {
    return `${base} bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300`
  }
  return `${base} bg-stone-100 text-stone-600 dark:bg-white/[0.08] dark:text-stone-300`
}

const discountBadgeClass = (row: CostComparisonRow) => {
  const base = 'rounded-full px-2.5 py-1 text-xs font-semibold'
  const discount = row.calculation.effective_discount ?? Number.POSITIVE_INFINITY
  if (discount <= 0.3) {
    return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300`
  }
  if (discount <= 0.7) {
    return `${base} bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300`
  }
  return `${base} bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300`
}

const formatRatio = (value?: number) => formatUpstreamRatio(value)
</script>
