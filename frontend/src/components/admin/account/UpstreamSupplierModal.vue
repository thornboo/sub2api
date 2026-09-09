<template>
  <BaseDialog
    :show="show"
    :title="isEditing ? t('admin.accounts.upstreamCost.editSupplierTitle') : t('admin.accounts.upstreamCost.createSupplierTitle')"
    width="normal"
    @close="handleClose"
  >
    <form id="upstream-supplier-form" class="space-y-5" @submit.prevent="handleSubmit">
      <div>
        <label class="input-label" for="upstream-supplier-name">
          {{ t('admin.accounts.upstreamCost.newSupplierName') }}
        </label>
        <input
          id="upstream-supplier-name"
          v-model="form.name"
          type="text"
          maxlength="120"
          required
          class="input"
          :placeholder="t('admin.accounts.upstreamCost.newSupplierNamePlaceholder')"
        />
      </div>

      <div>
        <label class="input-label" for="upstream-supplier-note">
          {{ t('admin.accounts.upstreamCost.supplierNote') }}
        </label>
        <textarea
          id="upstream-supplier-note"
          v-model="form.note"
          rows="3"
          class="input resize-none"
          :placeholder="t('admin.accounts.upstreamCost.supplierNotePlaceholder')"
        />
      </div>

      <section class="overflow-hidden rounded-xl border border-stone-200 bg-stone-50/70 dark:border-white/10 dark:bg-white/[0.035]">
        <div class="border-b border-stone-200 px-4 py-3 dark:border-white/10">
          <h4 class="font-semibold text-stone-950 dark:text-white">
            {{ t('admin.accounts.upstreamCost.defaultSettlementTitle') }}
          </h4>
          <p class="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
            {{ t('admin.accounts.upstreamCost.defaultSettlementDescription') }}
          </p>
        </div>

        <div class="space-y-4 p-4">
          <div>
            <label class="input-label" for="upstream-supplier-credit-per-cny">
              {{ t('admin.accounts.upstreamCost.defaultRechargeConversion') }}
            </label>
            <div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2">
              <span class="text-sm font-medium text-stone-600 dark:text-stone-300">1 CNY =</span>
              <input
                id="upstream-supplier-credit-per-cny"
                v-model="form.creditPerCNY"
                type="number"
                min="0.000001"
                step="0.000001"
                required
                class="input font-mono"
                data-testid="supplier-default-credit-per-cny"
                @input="defaultEffectiveInputDirty = true"
              />
              <span class="text-sm font-medium text-stone-600 dark:text-stone-300">USD</span>
            </div>
            <p class="input-hint">{{ t('admin.accounts.upstreamCost.defaultRechargeConversionHint') }}</p>
          </div>

          <div>
            <label class="input-label" for="upstream-supplier-reference-fx">
              {{ t('admin.accounts.upstreamCost.defaultReferenceFxRate') }}
            </label>
            <div class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2">
              <span class="text-sm font-medium text-stone-600 dark:text-stone-300">1 USD ≈</span>
              <input
                id="upstream-supplier-reference-fx"
                v-model="form.referenceFXRate"
                type="number"
                min="0.000001"
                step="0.000001"
                required
                class="input font-mono"
                data-testid="supplier-default-reference-fx"
              />
              <span class="text-sm font-medium text-stone-600 dark:text-stone-300">CNY</span>
            </div>
          </div>

          <div class="flex items-center justify-between gap-3 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2.5 dark:border-emerald-500/20 dark:bg-emerald-500/10">
            <span class="text-sm font-medium text-emerald-800 dark:text-emerald-200">
              {{ t('admin.accounts.upstreamCost.estimatedRechargeDiscount') }}
            </span>
            <span class="font-mono text-base font-semibold text-emerald-800 dark:text-emerald-200">
              {{ estimatedDiscountLabel }}
            </span>
          </div>
        </div>
      </section>

      <section class="overflow-hidden rounded-xl border border-stone-200 bg-stone-50/70 dark:border-white/10 dark:bg-white/[0.035]">
        <div class="flex items-start justify-between gap-3 border-b border-stone-200 px-4 py-3 dark:border-white/10">
          <div>
            <h4 class="font-semibold text-stone-950 dark:text-white">
              {{ t('admin.accounts.upstreamCost.supplierBalance.title') }}
            </h4>
            <p class="mt-1 text-xs leading-5 text-stone-500 dark:text-stone-400">
              {{ t('admin.accounts.upstreamCost.supplierBalance.description') }}
            </p>
          </div>
          <label class="inline-flex cursor-pointer items-center gap-2 text-sm font-medium text-stone-700 dark:text-stone-200">
            <input
              v-model="form.balanceEnabled"
              type="checkbox"
              class="h-4 w-4 rounded border-stone-300 text-emerald-600 focus:ring-emerald-500 dark:border-white/20 dark:bg-white/[0.05]"
              data-testid="supplier-balance-enabled"
            />
            {{ t('admin.accounts.upstreamCost.supplierBalance.enable') }}
          </label>
        </div>

        <div v-if="form.balanceEnabled" class="grid gap-4 p-4 sm:grid-cols-2">
          <div>
            <label class="input-label" for="upstream-supplier-balance-provider">
              {{ t('admin.accounts.upstreamCost.supplierBalance.provider') }}
            </label>
            <Select
              id="upstream-supplier-balance-provider"
              v-model="form.balanceProvider"
              class="w-full"
              :options="balanceProviderOptions"
              :aria-label="t('admin.accounts.upstreamCost.supplierBalance.provider')"
              match-trigger-width
              data-testid="supplier-balance-provider"
            />
          </div>

          <div v-if="balanceProviderRequiresUserID">
            <label class="input-label" for="upstream-supplier-balance-user-id">
              {{ t('admin.accounts.upstreamCost.supplierBalance.userId') }}
            </label>
            <input
              id="upstream-supplier-balance-user-id"
              v-model="form.balanceUserID"
              type="number"
              min="1"
              step="1"
              class="input font-mono"
              data-testid="supplier-balance-user-id"
              :placeholder="t('admin.accounts.upstreamCost.supplierBalance.userIdPlaceholder')"
            />
          </div>

          <div :class="{ 'sm:col-span-2': balanceProviderRequiresUserID }">
            <label class="input-label" for="upstream-supplier-balance-base-url">
              {{ t('admin.accounts.upstreamCost.supplierBalance.baseUrl') }}
            </label>
            <input
              id="upstream-supplier-balance-base-url"
              v-model="form.balanceBaseURL"
              type="url"
              required
              class="input font-mono"
              data-testid="supplier-balance-base-url"
              :placeholder="balanceBaseURLPlaceholder"
            />
          </div>

          <div class="sm:col-span-2">
            <label class="input-label" for="upstream-supplier-balance-token">
              {{ balanceTokenLabel }}
            </label>
            <input
              id="upstream-supplier-balance-token"
              v-model="form.balanceAccessToken"
              type="password"
              autocomplete="new-password"
              class="input font-mono"
              data-testid="supplier-balance-token"
              :placeholder="balanceTokenPlaceholder"
            />
            <p class="input-hint">
              {{ balanceTokenHint }}
            </p>
            <p v-if="balanceCredentialChangeHint" class="input-hint">
              {{ balanceCredentialChangeHint }}
            </p>
          </div>
        </div>
      </section>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="saving" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="submit"
          form="upstream-supplier-form"
          class="btn btn-primary"
          :disabled="saving || !canSubmit"
        >
          <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="check" size="sm" />
          {{ t('common.save') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { UpstreamCostPool, UpstreamSupplier } from '@/api/admin/accounts'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode } from '@/utils/apiError'
import { formatUpstreamDiscountLabel } from '@/utils/upstreamCost'

const props = defineProps<{
  show: boolean
  supplier?: UpstreamSupplier | null
  costPool?: UpstreamCostPool | null
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const { t } = useI18n()
const appStore = useAppStore()
const saving = ref(false)
const defaultEffectiveInputDirty = ref(false)
const balanceProviderOptions = computed<SelectOption[]>(() => [
  { value: 'newapi', label: t('admin.accounts.upstreamCost.supplierBalance.providerNewApi') },
  { value: 'soleapi', label: t('admin.accounts.upstreamCost.supplierBalance.providerSoleApi') },
  { value: 'sub2api', label: t('admin.accounts.upstreamCost.supplierBalance.providerSub2Api') }
])
const balanceProviders = ['newapi', 'soleapi', 'sub2api']
const form = reactive({
  name: '',
  note: '',
  creditPerCNY: '1',
  referenceFXRate: '7',
  balanceEnabled: false,
  balanceProvider: 'newapi',
  balanceBaseURL: '',
  balanceUserID: '',
  balanceAccessToken: ''
})

const isEditing = computed(() => Boolean(props.supplier?.id))
const isSoleAPIProvider = computed(() => form.balanceProvider === 'soleapi')
const isSub2APIProvider = computed(() => form.balanceProvider === 'sub2api')
const isAPIKeyProvider = computed(() => isSoleAPIProvider.value || isSub2APIProvider.value)
const balanceProviderRequiresUserID = computed(() => form.balanceProvider === 'newapi')
const positiveNumber = (value: string): number | null => {
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}
const defaultEffectiveCNYPerUSD = computed(() => {
  const existing = Number(props.costPool?.default_effective_cny_per_usd)
  if (!defaultEffectiveInputDirty.value && Number.isFinite(existing) && existing > 0) {
    return existing
  }
  const creditPerCNY = positiveNumber(form.creditPerCNY)
  return creditPerCNY ? 1 / creditPerCNY : null
})
const referenceFXRate = computed(() => positiveNumber(form.referenceFXRate))
const existingBalanceBaseURL = computed(() => normalizeBaseURL(props.supplier?.balance_config?.base_url || ''))
const existingBalanceProvider = computed(() => props.supplier?.balance_config?.provider || 'newapi')
const normalizedBalanceBaseURL = computed(() => normalizeBaseURL(form.balanceBaseURL))
const balanceSiteChanged = computed(() => (
  isEditing.value &&
  existingBalanceBaseURL.value !== '' &&
  normalizedBalanceBaseURL.value !== existingBalanceBaseURL.value
))
const balanceProviderChanged = computed(() => (
  isEditing.value &&
  form.balanceProvider !== existingBalanceProvider.value
))
const existingBalanceCredentialReusable = computed(() => (
  props.supplier?.balance_config?.has_access_token === true &&
  !balanceSiteChanged.value &&
  !balanceProviderChanged.value
))
const balanceTokenRequired = computed(() => (
  form.balanceEnabled &&
  !form.balanceAccessToken.trim() &&
  !existingBalanceCredentialReusable.value
))
const canSubmit = computed(() => (
  form.name.trim().length > 0 &&
  defaultEffectiveCNYPerUSD.value !== null &&
  referenceFXRate.value !== null &&
  (!form.balanceEnabled || (
    balanceProviders.includes(form.balanceProvider) &&
    normalizedBalanceBaseURL.value !== '' &&
    (!balanceProviderRequiresUserID.value || optionalPositiveInteger(form.balanceUserID) !== null) &&
    !balanceTokenRequired.value
  ))
))
const estimatedDiscountLabel = computed(() => {
  if (defaultEffectiveCNYPerUSD.value === null || referenceFXRate.value === null) {
    return t('admin.accounts.upstreamCost.notConfigured')
  }
  return formatUpstreamDiscountLabel(
    (defaultEffectiveCNYPerUSD.value / referenceFXRate.value) * 10,
    { suffix: t('admin.accounts.upstreamCost.discountSuffix') }
  )
})

const resetForm = () => {
  const effective = Number(props.costPool?.default_effective_cny_per_usd)
  const creditPerCNY = Number.isFinite(effective) && effective > 0 ? 1 / effective : 1
  const reference = Number(props.costPool?.default_reference_fx_rate)
  defaultEffectiveInputDirty.value = false
  form.name = props.supplier?.name || ''
  form.note = props.supplier?.note || ''
  form.creditPerCNY = formatInputNumber(creditPerCNY)
  form.referenceFXRate = formatInputNumber(Number.isFinite(reference) && reference > 0 ? reference : 7)
  form.balanceEnabled = props.supplier?.balance_config?.enabled === true
  form.balanceProvider = props.supplier?.balance_config?.provider || 'newapi'
  form.balanceBaseURL = props.supplier?.balance_config?.base_url || ''
  form.balanceUserID = props.supplier?.balance_config?.user_id ? String(props.supplier.balance_config.user_id) : ''
  form.balanceAccessToken = ''
}

const formatInputNumber = (value: number): string => (
  Number(value).toFixed(6).replace(/\.?0+$/, '')
)

const normalizeBaseURL = (value: string): string => value.trim().replace(/\/+$/, '')

const optionalPositiveInteger = (value: string | number | null | undefined): number | null | undefined => {
  const trimmed = String(value ?? '').trim()
  if (trimmed === '') return undefined
  const parsed = Number(trimmed)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null
}

const balanceTokenPlaceholder = computed(() => (
  existingBalanceCredentialReusable.value
    ? t('admin.accounts.upstreamCost.supplierBalance.accessTokenConfigured')
    : t(`admin.accounts.upstreamCost.supplierBalance.${isSub2APIProvider.value ? 'sub2ApiKeyPlaceholder' : isAPIKeyProvider.value ? 'apiKeyPlaceholder' : 'accessTokenPlaceholder'}`)
))

const balanceTokenLabel = computed(() => (
  t(`admin.accounts.upstreamCost.supplierBalance.${isAPIKeyProvider.value ? 'apiKey' : 'accessToken'}`)
))

const balanceTokenHint = computed(() => (
  t(`admin.accounts.upstreamCost.supplierBalance.${isSub2APIProvider.value ? 'sub2ApiKeyHint' : isAPIKeyProvider.value ? 'apiKeyHint' : 'accessTokenHint'}`)
))

const balanceCredentialChangeHint = computed(() => (
  balanceProviderChanged.value
    ? t('admin.accounts.upstreamCost.supplierBalance.accessTokenProviderChangedHint')
    : balanceSiteChanged.value
    ? t('admin.accounts.upstreamCost.supplierBalance.accessTokenSiteChangedHint')
    : ''
))

const balanceBaseURLPlaceholder = computed(() => (
  isSub2APIProvider.value ? 'https://sub2api.example.com' : isSoleAPIProvider.value ? 'https://soleapi.com' : 'https://newapi.example.com'
))

watch(
  () => [props.show, props.supplier?.id, props.costPool?.id] as const,
  ([show]) => {
    if (show) resetForm()
  },
  { immediate: true }
)

const handleClose = () => {
  if (!saving.value) emit('close')
}

const supplierErrorMessage = (error: any): string => {
  switch (extractApiErrorCode(error)) {
    case 'SUPPLIER_NAME_CONFLICT':
      return t('admin.accounts.upstreamCost.errors.nameConflict')
    case 'SUPPLIER_RESERVED':
      return t('admin.accounts.upstreamCost.errors.reserved')
    default:
      return error?.message || t(
        props.supplier?.id
          ? 'admin.accounts.upstreamCost.supplierUpdateFailed'
          : 'admin.accounts.upstreamCost.supplierCreateFailed'
      )
  }
}

const handleSubmit = async () => {
  if (!canSubmit.value || defaultEffectiveCNYPerUSD.value === null || referenceFXRate.value === null) return
  saving.value = true
  const balanceUserID = optionalPositiveInteger(form.balanceUserID)
  const balanceConfig = {
    enabled: form.balanceEnabled,
    provider: form.balanceProvider || 'newapi',
    base_url: normalizedBalanceBaseURL.value,
    user_id: balanceProviderRequiresUserID.value ? balanceUserID ?? undefined : undefined,
    access_token: form.balanceAccessToken.trim() || undefined
  }
  const payload = {
    name: form.name.trim(),
    note: props.supplier?.id ? form.note.trim() : form.note.trim() || null,
    default_effective_cny_per_usd: defaultEffectiveCNYPerUSD.value,
    default_reference_fx_rate: referenceFXRate.value,
    balance_config: balanceConfig
  }
  try {
    if (props.supplier?.id) {
      await adminAPI.accounts.updateUpstreamSupplier(props.supplier.id, payload)
      appStore.showSuccess(t('admin.accounts.upstreamCost.supplierUpdated'))
    } else {
      await adminAPI.accounts.createUpstreamSupplier(payload)
      appStore.showSuccess(t('admin.accounts.upstreamCost.supplierCreated'))
    }
    emit('saved')
    emit('close')
  } catch (error: any) {
    appStore.showError(supplierErrorMessage(error))
  } finally {
    saving.value = false
  }
}
</script>
