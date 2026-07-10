<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.upstreamCost.rechargeRecords.title')"
    width="extra-wide"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <span class="text-lg font-semibold text-stone-950 dark:text-white">{{ targetName }}</span>
            <span class="rounded-md bg-stone-100 px-2 py-1 text-xs font-medium text-stone-700 dark:bg-white/[0.07] dark:text-stone-200">
              {{ targetPrimaryMeta }}
            </span>
            <span
              v-if="targetSecondaryMeta"
              class="rounded-md bg-sky-50 px-2 py-1 text-xs font-medium text-sky-700 dark:bg-sky-500/10 dark:text-sky-300"
            >
              {{ targetSecondaryMeta }}
            </span>
          </div>
        </div>
        <button
          type="button"
          class="btn btn-secondary h-10 px-3 text-sm"
          :disabled="loading"
          @click="loadRecords"
        >
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-900/20 dark:text-red-300">
        {{ error }}
      </div>

      <div class="grid gap-3 md:grid-cols-4">
        <div class="rounded-xl border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.rechargeRecords.totalPaid') }}</span>
            <Icon name="creditCard" size="sm" class="text-emerald-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatMoney(summary.total_paid_amount, primaryPaidCurrency) }}
          </p>
        </div>
        <div class="rounded-xl border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('admin.accounts.upstreamCost.rechargeRecords.totalCredit') }}</span>
            <Icon name="database" size="sm" class="text-sky-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatMoney(summary.total_received_credit_amount, primaryCreditCurrency) }}
          </p>
        </div>
        <div class="rounded-xl border border-sky-200 bg-sky-50/60 p-4 dark:border-sky-500/25 dark:bg-sky-500/10">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-sky-700 dark:text-sky-300">{{ t('admin.accounts.upstreamCost.rechargeRecords.weightedCost') }}</span>
            <Icon name="calculator" size="sm" class="text-sky-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatCost(summary.weighted_effective_cny_per_usd) }}
          </p>
          <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
            {{ formatDiscount(summary.weighted_recharge_discount) }}
          </p>
        </div>
        <div class="rounded-xl border border-emerald-200 bg-emerald-50/60 p-4 dark:border-emerald-500/25 dark:bg-emerald-500/10">
          <div class="flex items-center justify-between gap-3">
            <span class="text-xs font-medium text-emerald-700 dark:text-emerald-300">{{ t('admin.accounts.upstreamCost.rechargeRecords.latestCost') }}</span>
            <Icon name="clock" size="sm" class="text-emerald-500" />
          </div>
          <p class="mt-2 text-2xl font-semibold text-stone-950 dark:text-white">
            {{ formatCost(summary.latest_effective_cny_per_usd) }}
          </p>
          <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">
            {{ formatDiscount(summary.latest_recharge_discount) }}
          </p>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <form class="rounded-xl border border-stone-200 bg-white p-4 dark:border-white/10 dark:bg-white/[0.035]" @submit.prevent="submitRecord">
          <div class="flex items-center justify-between gap-3">
            <h4 class="font-semibold text-stone-950 dark:text-white">
              {{ editingRecordId ? t('admin.accounts.upstreamCost.rechargeRecords.editRecord') : t('admin.accounts.upstreamCost.rechargeRecords.addRecord') }}
            </h4>
            <button
              v-if="editingRecordId"
              type="button"
              class="rounded-lg px-2 py-1 text-xs font-medium text-stone-500 hover:bg-stone-100 hover:text-stone-700 dark:text-stone-400 dark:hover:bg-white/[0.06] dark:hover:text-stone-200"
              @click="resetForm"
            >
              {{ t('common.cancel') }}
            </button>
          </div>

          <div class="mt-4 space-y-3">
            <label class="block">
              <span class="input-label">{{ t('admin.accounts.upstreamCost.rechargeRecords.type') }}</span>
              <Select v-model="form.type" :options="typeOptions">
                <template #selected="{ option }">
                  <span class="flex items-center gap-2">
                    <Icon name="gift" size="xs" class="text-emerald-500" />
                    <span>{{ option?.label || typeLabel(form.type) }}</span>
                  </span>
                </template>
              </Select>
            </label>

            <div class="rounded-lg border border-sky-200 bg-sky-50/70 px-3 py-2 text-xs leading-5 text-sky-800 dark:border-sky-500/20 dark:bg-sky-500/10 dark:text-sky-200">
              {{ t('admin.accounts.upstreamCost.rechargeRecords.defaultConfigApplied', {
                conversion: defaultConversionLabel,
                fx: formatUpstreamRatio(defaultReferenceFXRate)
              }) }}
            </div>

            <div v-if="form.type !== 'bonus'" class="grid grid-cols-[minmax(0,1fr)_72px] gap-2">
              <label class="block">
                <span class="input-label">{{ t('admin.accounts.upstreamCost.rechargeRecords.paidAmount') }}</span>
                <input v-model="form.paidAmount" type="number" min="0" step="0.000001" class="input" placeholder="0" />
              </label>
              <div>
                <span class="input-label">{{ t('admin.accounts.upstreamCost.rechargeRecords.currency') }}</span>
                <div class="flex h-[42px] items-center justify-center rounded-xl border border-stone-200/80 bg-stone-50 px-3 font-mono text-sm font-semibold text-stone-700 dark:border-white/10 dark:bg-white/[0.05] dark:text-stone-200">
                  {{ form.paidCurrency }}
                </div>
              </div>
            </div>

            <div class="grid grid-cols-[minmax(0,1fr)_72px] gap-2">
              <label class="block">
                <span class="input-label flex items-center justify-between gap-2">
                  <span>{{ t('admin.accounts.upstreamCost.rechargeRecords.receivedCredit') }}</span>
                  <button
                    v-if="form.type === 'recharge' && autoReceivedCreditAvailable"
                    type="button"
                    class="rounded-md px-1.5 py-0.5 text-[11px] font-medium text-sky-600 transition-colors hover:bg-sky-50 hover:text-sky-700 dark:text-sky-300 dark:hover:bg-sky-500/10"
                    @click="toggleReceivedCreditOverride"
                  >
                    {{ receivedCreditManual ? t('admin.accounts.upstreamCost.rechargeRecords.useDefaultCalculation') : t('admin.accounts.upstreamCost.rechargeRecords.overrideThisRecord') }}
                  </button>
                </span>
                <input
                  v-if="showReceivedCreditInput"
                  v-model="form.receivedCreditAmount"
                  type="number"
                  min="0"
                  step="0.000001"
                  class="input"
                  placeholder="0"
                  @input="markReceivedCreditManual"
                />
                <div
                  v-else
                  class="flex h-[42px] items-center rounded-xl border border-emerald-200 bg-emerald-50 px-3 font-mono text-sm font-semibold text-emerald-800 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-200"
                  data-testid="auto-received-credit"
                >
                  {{ form.receivedCreditAmount || '0' }}
                </div>
              </label>
              <div>
                <span class="input-label">{{ t('admin.accounts.upstreamCost.rechargeRecords.currency') }}</span>
                <div class="flex h-[42px] items-center justify-center rounded-xl border border-stone-200/80 bg-stone-50 px-3 font-mono text-sm font-semibold text-stone-700 dark:border-white/10 dark:bg-white/[0.05] dark:text-stone-200">
                  {{ form.receivedCreditCurrency }}
                </div>
              </div>
            </div>

            <div :class="showReferenceFXInput ? 'grid grid-cols-2 gap-2' : ''">
              <label v-if="showReferenceFXInput" class="block">
                <span class="input-label">{{ t('admin.accounts.upstreamCost.referenceFxRate') }}</span>
                <input v-model="form.referenceFXRate" type="number" min="0.000001" step="0.000001" class="input" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.accounts.upstreamCost.rechargeRecords.recordedAt') }}</span>
                <input v-model="form.recordedAt" type="datetime-local" class="input" />
              </label>
            </div>

            <label class="block">
              <span class="input-label">{{ t('admin.accounts.upstreamCost.note') }}</span>
              <textarea v-model="form.note" rows="3" class="input resize-none" :placeholder="t('admin.accounts.upstreamCost.rechargeRecords.notePlaceholder')" />
            </label>

            <button type="submit" class="btn btn-primary w-full justify-center" :disabled="saving || !canSubmit">
              <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="check" size="sm" />
              {{ editingRecordId ? t('common.save') : t('admin.accounts.upstreamCost.rechargeRecords.addRecord') }}
            </button>
          </div>
        </form>

        <div class="min-w-0 overflow-hidden rounded-xl border border-stone-200 bg-white dark:border-white/10 dark:bg-white/[0.035]">
          <div class="flex flex-wrap items-center justify-between gap-3 border-b border-stone-200 px-4 py-3 dark:border-white/10">
            <div>
              <h4 class="font-semibold text-stone-950 dark:text-white">{{ t('admin.accounts.upstreamCost.rechargeRecords.history') }}</h4>
              <p class="text-xs text-stone-500 dark:text-stone-400">
                {{ summary.record_count }} {{ t('admin.accounts.upstreamCost.rechargeRecords.records') }}
              </p>
            </div>
            <div v-if="isPoolMode" class="text-xs text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.rechargeRecords.poolAutoApplyHint') }}
            </div>
            <div v-else class="flex flex-wrap gap-2">
              <button
                type="button"
                class="btn btn-secondary h-9 px-3 text-xs"
                :disabled="applying || !summary.latest_effective_cny_per_usd"
                @click="applyCost('latest')"
              >
                {{ t('admin.accounts.upstreamCost.rechargeRecords.applyLatest') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary h-9 px-3 text-xs"
                :disabled="applying || !summary.weighted_effective_cny_per_usd"
                @click="applyCost('weighted')"
              >
                {{ t('admin.accounts.upstreamCost.rechargeRecords.applyWeighted') }}
              </button>
            </div>
          </div>

          <div v-if="loading" class="flex items-center justify-center py-12 text-stone-500 dark:text-stone-400">
            <Icon name="refresh" size="md" class="mr-2 animate-spin" />
            {{ t('common.loading') }}...
          </div>
          <div v-else-if="records.length === 0" class="flex flex-col items-center justify-center py-12 text-center text-stone-500 dark:text-stone-400">
            <Icon name="creditCard" size="lg" class="mb-2 text-stone-400" />
            <p class="text-sm">{{ t('admin.accounts.upstreamCost.rechargeRecords.empty') }}</p>
          </div>
          <div v-else class="max-h-[420px] overflow-auto">
            <table class="min-w-[760px] divide-y divide-stone-200 text-sm dark:divide-white/10">
              <thead class="sticky top-0 bg-stone-50 text-xs font-semibold uppercase text-stone-500 dark:bg-stone-950 dark:text-stone-400">
                <tr>
                  <th class="px-4 py-3 text-left">{{ t('admin.accounts.upstreamCost.rechargeRecords.recordedAt') }}</th>
                  <th class="px-4 py-3 text-left">{{ t('admin.accounts.upstreamCost.rechargeRecords.type') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.accounts.upstreamCost.rechargeRecords.paidAmount') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.accounts.upstreamCost.rechargeRecords.receivedCredit') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.accounts.upstreamCost.rechargeRecords.effectiveCost') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.accounts.upstreamCost.effectiveDiscount') }}</th>
                  <th class="px-4 py-3 text-right">{{ t('admin.accounts.columns.actions') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-stone-100 dark:divide-white/[0.06]">
                <tr v-for="record in records" :key="record.id" class="hover:bg-stone-50/70 dark:hover:bg-white/[0.035]">
                  <td class="px-4 py-3">
                    <div class="font-medium text-stone-800 dark:text-stone-100">{{ formatDate(record.recorded_at) }}</div>
                    <div v-if="record.note" class="mt-1 max-w-[180px] truncate text-xs text-stone-500 dark:text-stone-400" :title="record.note">
                      {{ record.note }}
                    </div>
                  </td>
                  <td class="px-4 py-3">
                    <span :class="typeBadgeClass(record.type)">{{ typeLabel(record.type) }}</span>
                  </td>
                  <td class="px-4 py-3 text-right font-mono text-stone-700 dark:text-stone-200">
                    {{ formatMoney(record.paid_amount, record.paid_currency) }}
                  </td>
                  <td class="px-4 py-3 text-right font-mono text-stone-700 dark:text-stone-200">
                    {{ formatMoney(record.received_credit_amount, record.received_credit_currency) }}
                  </td>
                  <td class="px-4 py-3 text-right font-mono text-stone-700 dark:text-stone-200">
                    {{ formatCost(record.effective_cny_per_usd) }}
                  </td>
                  <td class="px-4 py-3 text-right">
                    <span class="rounded-full bg-stone-100 px-2 py-1 font-mono text-xs font-semibold text-stone-700 dark:bg-white/[0.08] dark:text-stone-200">
                      {{ formatDiscount(record.recharge_discount) }}
                    </span>
                  </td>
                  <td class="px-4 py-3">
                    <div class="flex justify-end gap-1">
                      <button
                        type="button"
                        class="rounded-lg p-2 text-stone-500 hover:bg-stone-100 hover:text-stone-800 dark:text-stone-400 dark:hover:bg-white/[0.07] dark:hover:text-stone-100"
                        :aria-label="t('common.edit')"
                        @click="startEdit(record)"
                      >
                        <Icon name="edit" size="sm" />
                      </button>
                      <button
                        type="button"
                        class="rounded-lg p-2 text-stone-500 hover:bg-red-50 hover:text-red-700 dark:text-stone-400 dark:hover:bg-red-500/10 dark:hover:text-red-300"
                        :aria-label="t('common.delete')"
                        @click="askDelete(record)"
                      >
                        <Icon name="trash" size="sm" />
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

  </BaseDialog>

  <ConfirmDialog
    :show="!!deleteTarget"
    :title="t('admin.accounts.upstreamCost.rechargeRecords.deleteTitle')"
    :message="t('admin.accounts.upstreamCost.rechargeRecords.deleteMessage')"
    :confirm-text="t('common.delete')"
    :cancel-text="t('common.cancel')"
    :danger="true"
    @confirm="confirmDelete"
    @cancel="deleteTarget = null"
  />
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  UpstreamCostPool,
  UpstreamRechargeRecord,
  UpstreamRechargeRecordPayload,
  UpstreamRechargeRecordType,
  UpstreamRechargeSummary
} from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import type { Account } from '@/types'
import { extractI18nErrorMessage } from '@/utils/apiError'
import {
  DEFAULT_UPSTREAM_REFERENCE_FX_RATE,
  formatUpstreamDiscountLabel,
  formatUpstreamRatio,
  readUpstreamCostProfile
} from '@/utils/upstreamCost'

const props = defineProps<{
  show: boolean
  account?: Account | null
  costPool?: UpstreamCostPool | null
}>()

const emit = defineEmits<{
  close: []
  updated: [account: Account]
  'pool-updated': []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const paidCurrency = 'CNY'
const receivedCreditCurrency = 'USD'
const defaultReferenceFXRate = computed(() => (
  props.costPool?.default_reference_fx_rate ||
  props.costPool?.reference_fx_rate ||
  DEFAULT_UPSTREAM_REFERENCE_FX_RATE
))
const formatRechargeError = (err: unknown, fallback: string) =>
  extractI18nErrorMessage(err, t, 'admin.accounts.upstreamCost.errors', fallback)

const emptySummary = (): UpstreamRechargeSummary => ({
  record_count: 0,
  total_paid_amount: 0,
  total_received_credit_amount: 0,
  reference_fx_rate: defaultReferenceFXRate.value
})

const records = ref<UpstreamRechargeRecord[]>([])
const summary = ref<UpstreamRechargeSummary>(emptySummary())
const loading = ref(false)
const saving = ref(false)
const applying = ref(false)
const error = ref<string | null>(null)
const editingRecordId = ref<number | null>(null)
const deleteTarget = ref<UpstreamRechargeRecord | null>(null)
const receivedCreditManual = ref(false)
const isPoolMode = computed(() => Boolean(props.costPool?.id))
const targetName = computed(() => props.costPool?.supplier_name || props.account?.name || '-')
const targetPrimaryMeta = computed(() => {
  if (props.costPool) return props.costPool.name || t('admin.accounts.upstreamCost.supplierPool')
  return props.account?.platform || '-'
})
const targetSecondaryMeta = computed(() => {
  if (props.costPool) return t('admin.accounts.upstreamCost.supplierRechargeScope')
  return props.account?.type || ''
})

const form = reactive({
  type: 'recharge' as UpstreamRechargeRecordType,
  paidAmount: '',
  paidCurrency,
  receivedCreditAmount: '',
  receivedCreditCurrency,
  referenceFXRate: String(DEFAULT_UPSTREAM_REFERENCE_FX_RATE),
  recordedAt: '',
  note: ''
})

const typeOptions = computed(() => [
  { value: 'recharge', label: t('admin.accounts.upstreamCost.rechargeRecords.typeRecharge') },
  { value: 'bonus', label: t('admin.accounts.upstreamCost.rechargeRecords.typeBonus') },
  { value: 'adjustment', label: t('admin.accounts.upstreamCost.rechargeRecords.typeAdjustment') }
])

const canSubmit = computed(() => {
  const paid = numberFromInput(form.paidAmount)
  const received = numberFromInput(form.receivedCreditAmount)
  const fx = numberFromInput(form.referenceFXRate)
  if (paid < 0 || received < 0 || fx <= 0) return false
  if (form.type === 'recharge') return paid > 0 && received > 0
  if (form.type === 'bonus') return received > 0
  return paid + received > 0
})

const primaryPaidCurrency = computed(() => records.value[0]?.paid_currency || form.paidCurrency || 'CNY')
const primaryCreditCurrency = computed(() => records.value[0]?.received_credit_currency || form.receivedCreditCurrency || 'USD')
const accountCostProfile = computed(() => readUpstreamCostProfile(props.account?.extra as Record<string, unknown> | undefined))
const configuredRechargeCNYPerUSD = computed(() => (
  props.costPool?.default_effective_cny_per_usd ||
  props.costPool?.current_effective_cny_per_usd ||
  accountCostProfile.value.recharge_cny_per_usd
))
const defaultConversionLabel = computed(() => {
  const rate = Number(configuredRechargeCNYPerUSD.value)
  if (!Number.isFinite(rate) || rate <= 0) return '-'
  return `1 CNY = ${formatUpstreamRatio(1 / rate)} USD`
})
const showReceivedCreditInput = computed(() => form.type !== 'recharge' || receivedCreditManual.value)
const showReferenceFXInput = computed(() => (
  editingRecordId.value !== null ||
  (form.type === 'recharge' && receivedCreditManual.value)
))
const autoReceivedCreditAvailable = computed(() => {
  const rate = configuredRechargeCNYPerUSD.value
  return Boolean(
    rate && rate > 0 &&
    normalizeCurrency(form.paidCurrency, paidCurrency) === paidCurrency &&
    normalizeCurrency(form.receivedCreditCurrency, receivedCreditCurrency) === receivedCreditCurrency
  )
})

watch(
  () => [props.show, props.account?.id, props.costPool?.id] as const,
  ([show]) => {
    if (show) {
      resetForm()
      loadRecords()
    } else {
      records.value = []
      summary.value = emptySummary()
      error.value = null
      deleteTarget.value = null
    }
  }
)

watch(
  () => form.type,
  (type) => {
    if (editingRecordId.value !== null) return
    if (type === 'bonus') {
      form.paidAmount = '0'
      receivedCreditManual.value = true
      return
    }
    if (type === 'adjustment') {
      if (form.paidAmount === '0') form.paidAmount = ''
      receivedCreditManual.value = true
      return
    }
    if (form.paidAmount === '0') form.paidAmount = ''
    receivedCreditManual.value = false
    applyAutoReceivedCredit()
  }
)

watch(
  () => [
    form.paidAmount,
    form.paidCurrency,
    form.receivedCreditCurrency,
    configuredRechargeCNYPerUSD.value,
    receivedCreditManual.value
  ] as const,
  () => {
    if (!receivedCreditManual.value) {
      applyAutoReceivedCredit()
    }
  }
)

const loadRecords = async () => {
  if (!props.account && !props.costPool) return
  loading.value = true
  error.value = null
  try {
    const result = props.costPool
      ? await adminAPI.accounts.listUpstreamCostPoolRechargeRecords(props.costPool.id)
      : await adminAPI.accounts.listUpstreamRechargeRecords(props.account!.id)
    records.value = result.items || []
    summary.value = result.summary || emptySummary()
  } catch (err: unknown) {
    error.value = formatRechargeError(err, t('admin.accounts.upstreamCost.rechargeRecords.loadFailed'))
  } finally {
    loading.value = false
  }
}

const submitRecord = async () => {
  if ((!props.account && !props.costPool) || saving.value || !canSubmit.value) return
  saving.value = true
  error.value = null
  try {
    const payload = buildPayload()
    if (editingRecordId.value) {
      if (props.costPool) {
        await adminAPI.accounts.updateUpstreamCostPoolRechargeRecord(props.costPool.id, editingRecordId.value, payload)
      } else {
        await adminAPI.accounts.updateUpstreamRechargeRecord(props.account!.id, editingRecordId.value, payload)
      }
      appStore.showSuccess(t('admin.accounts.upstreamCost.rechargeRecords.saved'))
    } else {
      if (props.costPool) {
        await adminAPI.accounts.createUpstreamCostPoolRechargeRecord(props.costPool.id, payload)
      } else {
        await adminAPI.accounts.createUpstreamRechargeRecord(props.account!.id, payload)
      }
      appStore.showSuccess(t('admin.accounts.upstreamCost.rechargeRecords.created'))
    }
    resetForm()
    await loadRecords()
    if (props.costPool) {
      emit('pool-updated')
    }
  } catch (err: unknown) {
    error.value = formatRechargeError(err, t('admin.accounts.upstreamCost.rechargeRecords.saveFailed'))
  } finally {
    saving.value = false
  }
}

const buildPayload = (): UpstreamRechargeRecordPayload => ({
  type: form.type,
  paid_amount: numberFromInput(form.paidAmount),
  paid_currency: paidCurrency,
  received_credit_amount: numberFromInput(form.receivedCreditAmount),
  received_credit_currency: receivedCreditCurrency,
  reference_fx_rate: numberFromInput(form.referenceFXRate) || defaultReferenceFXRate.value,
  recorded_at: fromDateTimeLocal(form.recordedAt),
  note: form.note.trim() || null
})

const resetForm = () => {
  editingRecordId.value = null
  receivedCreditManual.value = false
  form.type = 'recharge'
  form.paidAmount = ''
  form.paidCurrency = paidCurrency
  form.receivedCreditAmount = ''
  form.receivedCreditCurrency = receivedCreditCurrency
  form.referenceFXRate = String(defaultReferenceFXRate.value)
  form.recordedAt = toDateTimeLocal(new Date())
  form.note = ''
}

const startEdit = (record: UpstreamRechargeRecord) => {
  editingRecordId.value = record.id
  receivedCreditManual.value = true
  form.type = record.type
  form.paidAmount = String(record.paid_amount)
  form.paidCurrency = paidCurrency
  form.receivedCreditAmount = String(record.received_credit_amount)
  form.receivedCreditCurrency = receivedCreditCurrency
  form.referenceFXRate = String(record.reference_fx_rate || defaultReferenceFXRate.value)
  form.recordedAt = toDateTimeLocal(record.recorded_at)
  form.note = record.note || ''
}

const askDelete = (record: UpstreamRechargeRecord) => {
  deleteTarget.value = record
}

const confirmDelete = async () => {
  if ((!props.account && !props.costPool) || !deleteTarget.value) return
  const target = deleteTarget.value
  deleteTarget.value = null
  error.value = null
  try {
    if (props.costPool) {
      await adminAPI.accounts.deleteUpstreamCostPoolRechargeRecord(props.costPool.id, target.id)
    } else {
      await adminAPI.accounts.deleteUpstreamRechargeRecord(props.account!.id, target.id)
    }
    if (editingRecordId.value === target.id) {
      resetForm()
    }
    appStore.showSuccess(t('admin.accounts.upstreamCost.rechargeRecords.deleted'))
    await loadRecords()
    if (props.costPool) {
      emit('pool-updated')
    }
  } catch (err: unknown) {
    error.value = formatRechargeError(err, t('admin.accounts.upstreamCost.rechargeRecords.deleteFailed'))
  }
}

const applyCost = async (mode: 'latest' | 'weighted') => {
  if (!props.account || props.costPool || applying.value) return
  const value = mode === 'latest'
    ? summary.value.latest_effective_cny_per_usd
    : summary.value.weighted_effective_cny_per_usd
  if (!value || value <= 0) return

  applying.value = true
  error.value = null
  try {
    const profile = readUpstreamCostProfile(props.account.extra)
    const updated = await adminAPI.accounts.updateUpstreamCostProfile(props.account.id, {
      ...profile,
      recharge_cny_per_usd: value,
      reference_fx_rate: summary.value.reference_fx_rate || DEFAULT_UPSTREAM_REFERENCE_FX_RATE
    })
    emit('updated', updated)
    appStore.showSuccess(t('admin.accounts.upstreamCost.rechargeRecords.applied'))
  } catch (err: unknown) {
    error.value = formatRechargeError(err, t('admin.accounts.upstreamCost.rechargeRecords.applyFailed'))
  } finally {
    applying.value = false
  }
}

const handleClose = () => {
  emit('close')
}

const markReceivedCreditManual = () => {
  receivedCreditManual.value = true
}

const toggleReceivedCreditOverride = () => {
  if (receivedCreditManual.value) {
    receivedCreditManual.value = false
    form.referenceFXRate = String(defaultReferenceFXRate.value)
    applyAutoReceivedCredit()
    return
  }
  receivedCreditManual.value = true
}

const applyAutoReceivedCredit = () => {
  if (!autoReceivedCreditAvailable.value || !configuredRechargeCNYPerUSD.value) return
  const paid = numberFromInput(form.paidAmount)
  if (paid <= 0) {
    form.receivedCreditAmount = ''
    receivedCreditManual.value = false
    return
  }
  form.receivedCreditAmount = formatInputNumber(paid / configuredRechargeCNYPerUSD.value)
  receivedCreditManual.value = false
}

const numberFromInput = (value: string): number => {
  const parsed = Number(String(value).trim())
  return Number.isFinite(parsed) ? parsed : 0
}

const formatInputNumber = (value: number): string => {
  if (!Number.isFinite(value)) return ''
  return value.toFixed(6).replace(/\.?0+$/, '')
}

const normalizeCurrency = (value: string, fallback: string) => {
  const currency = value.trim().toUpperCase()
  return currency || fallback
}

const toDateTimeLocal = (value: string | Date) => {
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}

const fromDateTimeLocal = (value: string) => {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date.toISOString()
}

const formatDate = (value?: string | null) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '-'
  return new Intl.DateTimeFormat(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  }).format(date)
}

const formatMoney = (value?: number | null, currency = '') => {
  if (!Number.isFinite(Number(value))) return '-'
  const amount = Number(value)
  const formatted = amount.toLocaleString(undefined, {
    minimumFractionDigits: amount >= 100 ? 2 : 4,
    maximumFractionDigits: 6
  })
  return currency ? `${formatted} ${currency}` : formatted
}

const formatCost = (value?: number | null) => {
  if (!Number.isFinite(Number(value))) return '-'
  return `${formatUpstreamRatio(Number(value))} CNY/USD`
}

const formatDiscount = (value?: number | null) => {
  if (!Number.isFinite(Number(value))) return '-'
  return formatUpstreamDiscountLabel(Number(value) * 10, {
    suffix: t('admin.accounts.upstreamCost.discountSuffix'),
    notConfiguredLabel: t('admin.accounts.upstreamCost.notConfigured')
  })
}

const typeLabel = (value: string) => {
  if (value === 'bonus') return t('admin.accounts.upstreamCost.rechargeRecords.typeBonus')
  if (value === 'adjustment') return t('admin.accounts.upstreamCost.rechargeRecords.typeAdjustment')
  return t('admin.accounts.upstreamCost.rechargeRecords.typeRecharge')
}

const typeBadgeClass = (value: string) => {
  const base = 'rounded-full px-2.5 py-1 text-xs font-semibold'
  if (value === 'bonus') return `${base} bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300`
  if (value === 'adjustment') return `${base} bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300`
  return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300`
}
</script>
