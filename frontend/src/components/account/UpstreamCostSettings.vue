<template>
  <section class="space-y-4 border-t border-stone-200/80 pt-4 dark:border-white/10">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h3 class="input-label mb-0 text-base font-semibold">{{ t('admin.accounts.upstreamCost.settingsTitle') }}</h3>
        <p class="mt-1 text-xs leading-5 text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.upstreamCost.description') }}
        </p>
      </div>
      <span
        class="rounded-full px-2.5 py-1 text-xs font-medium"
        :class="defaultCalculation.complete
          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
          : 'bg-gray-100 text-gray-600 dark:bg-white/[0.08] dark:text-gray-300'"
      >
        {{ defaultCalculation.label }}
      </span>
    </div>

    <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
      <label class="block">
        <span class="input-label">{{ t('admin.accounts.upstreamCost.rechargeCnyPerUsd') }}</span>
        <input
          :value="profile.recharge_cny_per_usd ?? ''"
          type="number"
          min="0"
          step="0.0001"
          class="input"
          placeholder="1"
          @input="updateNumber('recharge_cny_per_usd', $event)"
        />
        <span class="input-hint">{{ t('admin.accounts.upstreamCost.rechargeCnyPerUsdHint') }}</span>
      </label>
      <label class="block">
        <span class="input-label">{{ t('admin.accounts.upstreamCost.referenceFxRate') }}</span>
        <input
          :value="profile.reference_fx_rate ?? ''"
          type="number"
          min="0"
          step="0.0001"
          class="input"
          :placeholder="String(DEFAULT_UPSTREAM_REFERENCE_FX_RATE)"
          @input="updateNumber('reference_fx_rate', $event)"
        />
        <span class="input-hint">{{ t('admin.accounts.upstreamCost.referenceFxRateHint') }}</span>
      </label>
      <label class="block">
        <span class="input-label">{{ t('admin.accounts.upstreamCost.groupMultiplier') }}</span>
        <input
          :value="profile.group_multiplier ?? ''"
          type="number"
          min="0"
          step="0.0001"
          class="input"
          placeholder="1"
          @input="updateNumber('group_multiplier', $event)"
        />
        <span class="input-hint">{{ t('admin.accounts.upstreamCost.groupMultiplierHint') }}</span>
      </label>
    </div>

    <label class="block">
      <span class="input-label">{{ t('admin.accounts.upstreamCost.note') }}</span>
      <input
        :value="profile.note ?? ''"
        type="text"
        class="input"
        :placeholder="t('admin.accounts.upstreamCost.notePlaceholder')"
        @input="updateText('note', $event)"
      />
    </label>

    <div class="rounded-lg border border-sky-200/70 bg-sky-50/45 p-3 dark:border-sky-500/20 dark:bg-sky-500/[0.06]">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.accounts.upstreamCost.balanceQuery.title') }}
          </h4>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ balanceProviderLabel }}
          </p>
        </div>
        <button
          type="button"
          class="inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-sm font-medium transition-colors"
          :class="balanceQueryEnabled
            ? 'border-emerald-300 bg-emerald-100 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/15 dark:text-emerald-300'
            : 'border-gray-200 bg-white text-gray-600 hover:border-sky-300 dark:border-white/10 dark:bg-white/[0.05] dark:text-gray-300'"
          @click="updateBalanceEnabled(!balanceQueryEnabled)"
        >
          <span class="h-2 w-2 rounded-full" :class="balanceQueryEnabled ? 'bg-emerald-500' : 'bg-gray-400'" />
          {{ balanceQueryEnabled ? t('common.enabled') : t('common.disabled') }}
        </button>
      </div>

      <div v-if="balanceQueryEnabled" class="mt-3 grid grid-cols-1 gap-3 lg:grid-cols-[1fr_1fr]">
        <div>
          <span class="input-label">{{ t('admin.accounts.upstreamCost.balanceQuery.provider') }}</span>
          <div class="grid grid-cols-2 gap-2">
            <button
              v-for="option in balanceProviderOptions"
              :key="option.value"
              type="button"
              class="rounded-lg border px-3 py-2 text-left text-sm font-medium transition-colors"
              :class="balanceProvider === option.value
                ? 'border-sky-300 bg-sky-50 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/15 dark:text-sky-300'
                : 'border-gray-200 bg-white text-gray-600 hover:border-sky-300 dark:border-white/10 dark:bg-white/[0.05] dark:text-gray-300'"
              @click="updateBalanceProvider(option.value)"
            >
              {{ option.label }}
            </button>
          </div>
        </div>
        <label class="block">
          <span class="input-label">{{ t('admin.accounts.upstreamCost.balanceQuery.endpoint') }}</span>
          <input
            :value="balanceEndpoint"
            type="text"
            class="input"
            :placeholder="balanceDefaultEndpoint"
            @input="updateBalanceEndpoint"
          />
        </label>
      </div>

      <div v-if="balanceQueryEnabled" class="mt-3 space-y-3">
        <div>
          <span class="input-label">{{ t('admin.accounts.upstreamCost.balanceQuery.authMode') }}</span>
          <div class="grid grid-cols-1 gap-2 sm:grid-cols-3">
            <button
              v-for="option in balanceAuthModeOptions"
              :key="option.value"
              type="button"
              class="rounded-lg border px-3 py-2 text-left text-sm font-medium transition-colors"
              :class="balanceAuthMode === option.value
                ? 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/15 dark:text-emerald-300'
                : 'border-gray-200 bg-white text-gray-600 hover:border-sky-300 dark:border-white/10 dark:bg-white/[0.05] dark:text-gray-300'"
              @click="updateBalanceAuthMode(option.value)"
            >
              {{ option.label }}
            </button>
          </div>
        </div>

        <div v-if="balanceAuthMode !== UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY" class="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_1fr]">
          <label v-if="balanceAuthMode === UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER" class="block">
            <span class="input-label">{{ t('admin.accounts.upstreamCost.balanceQuery.authHeader') }}</span>
            <input
              :value="profile.balance_auth_header || 'Authorization'"
              type="text"
              class="input"
              placeholder="Authorization"
              @input="updateBalanceAuthHeader"
            />
          </label>
          <label class="block" :class="balanceAuthMode === UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN ? 'sm:col-span-2' : ''">
            <span class="input-label">{{ t('admin.accounts.upstreamCost.balanceQuery.authToken') }}</span>
            <input
              :value="balanceAuthTokenValue || ''"
              type="password"
              class="input"
              autocomplete="off"
              :placeholder="balanceAuthTokenConfigured
                ? t('admin.accounts.upstreamCost.balanceQuery.authTokenConfigured')
                : t('admin.accounts.upstreamCost.balanceQuery.authTokenPlaceholder')"
              @input="updateBalanceAuthToken"
            />
            <span class="input-hint">{{ t('admin.accounts.upstreamCost.balanceQuery.authTokenHint') }}</span>
          </label>
        </div>
      </div>
    </div>

    <div class="rounded-lg border border-gray-200 bg-gray-50/70 p-3 dark:border-dark-600 dark:bg-dark-800/70">
      <div class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-4">
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamCost.rechargeDiscount') }}</p>
          <p class="mt-1 font-mono font-semibold text-gray-900 dark:text-white">
            {{ formatRatio(defaultCalculation.recharge_cost_factor) }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamCost.defaultMultiplier') }}</p>
          <p class="mt-1 font-mono font-semibold text-gray-900 dark:text-white">
            {{ formatRatio(defaultCalculation.group_multiplier) }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamCost.effectiveDiscount') }}</p>
          <p class="mt-1 font-mono font-semibold text-emerald-600 dark:text-emerald-300">
            {{ defaultCalculation.complete ? formatRatio(defaultCalculation.effective_discount) : '-' }}
          </p>
        </div>
        <div>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accounts.upstreamCost.displayDiscount') }}</p>
          <p class="mt-1 font-semibold text-gray-900 dark:text-white">{{ defaultCalculation.label }}</p>
        </div>
      </div>
      <p v-if="!defaultCalculation.complete" class="mt-3 text-xs text-amber-600 dark:text-amber-300">
        {{ t('admin.accounts.upstreamCost.missingFields', { fields: missingFieldLabels(defaultCalculation.missing_fields).join('、') }) }}
      </p>
    </div>

    <div class="space-y-3">
      <div class="flex items-center justify-between gap-3">
        <div>
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ t('admin.accounts.upstreamCost.familyOverrides') }}
          </h4>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.upstreamCost.familyOverridesHint') }}
          </p>
        </div>
        <button type="button" class="btn btn-secondary px-3 py-1.5 text-sm" @click="addFamily">
          <Icon name="plus" size="sm" />
          {{ t('admin.accounts.upstreamCost.addFamily') }}
        </button>
      </div>

      <div v-if="families.length === 0" class="rounded-lg border border-dashed border-gray-200 px-3 py-4 text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t('admin.accounts.upstreamCost.noFamilyOverrides') }}
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="(family, index) in families"
          :key="`${family.family || 'new'}-${index}`"
          class="grid grid-cols-1 gap-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600 lg:grid-cols-[1fr_10rem_1fr_auto]"
        >
          <input
            :value="family.family"
            type="text"
            class="input"
            :placeholder="t('admin.accounts.upstreamCost.familyPlaceholder')"
            @input="updateFamilyText(index, 'family', $event)"
          />
          <input
            :value="family.group_multiplier ?? ''"
            type="number"
            min="0"
            step="0.0001"
            class="input"
            :placeholder="t('admin.accounts.upstreamCost.groupMultiplier')"
            @input="updateFamilyNumber(index, $event)"
          />
          <input
            :value="family.note ?? ''"
            type="text"
            class="input"
            :placeholder="t('admin.accounts.upstreamCost.familyNotePlaceholder')"
            @input="updateFamilyText(index, 'note', $event)"
          />
          <div class="flex items-center justify-between gap-2 lg:justify-end">
            <span class="whitespace-nowrap rounded-full bg-blue-50 px-2.5 py-1 text-xs font-medium text-blue-700 dark:bg-blue-900/30 dark:text-blue-300">
              {{ previewFamily(family.family).label }}
            </span>
            <button type="button" class="rounded-lg p-2 text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20" @click="removeFamily(index)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import {
  DEFAULT_UPSTREAM_BALANCE_PROVIDER,
  DEFAULT_UPSTREAM_REFERENCE_FX_RATE,
  UPSTREAM_BALANCE_PROVIDER_SUB2API,
  UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY,
  UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN,
  UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER,
  UPSTREAM_BALANCE_PROVIDER_NEW_API,
  calculateUpstreamCost,
  defaultUpstreamBalanceAuthMode,
  defaultUpstreamBalanceEndpoint,
  formatUpstreamRatio,
  normalizeUpstreamBalanceAuthMode,
  normalizeUpstreamBalanceEndpoint,
  type UpstreamBalanceAuthMode,
  type UpstreamBalanceProvider,
  type UpstreamCostFamilyOverride,
  type UpstreamCostMissingField,
  type UpstreamCostProfile
} from '@/utils/upstreamCost'

const props = defineProps<{
  modelValue?: UpstreamCostProfile
  balanceAuthTokenValue?: string
  balanceAuthTokenConfigured?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: UpstreamCostProfile]
  'update:balanceAuthTokenValue': [value: string]
}>()

const { t } = useI18n()

const profile = computed<UpstreamCostProfile>(() => {
  const value = props.modelValue || {}
  return {
    ...value,
    reference_fx_rate: value.reference_fx_rate ?? DEFAULT_UPSTREAM_REFERENCE_FX_RATE
  }
})
const families = computed(() => profile.value.model_families || [])
const defaultCalculation = computed(() => calculateUpstreamCost(profile.value))
const balanceQueryEnabled = computed(() => profile.value.balance_query_enabled === true)
const balanceProvider = computed<UpstreamBalanceProvider>(() => profile.value.balance_provider || DEFAULT_UPSTREAM_BALANCE_PROVIDER)
const balanceDefaultEndpoint = computed(() => defaultUpstreamBalanceEndpoint(balanceProvider.value))
const balanceAuthMode = computed<UpstreamBalanceAuthMode>(() => normalizeUpstreamBalanceAuthMode(balanceProvider.value, profile.value.balance_auth_mode))
const balanceEndpoint = computed(() => normalizeUpstreamBalanceEndpoint(balanceProvider.value, profile.value.balance_endpoint, balanceAuthMode.value))
const balanceProviderOptions = computed<Array<{ value: UpstreamBalanceProvider; label: string }>>(() => [
  { value: UPSTREAM_BALANCE_PROVIDER_SUB2API, label: t('admin.accounts.upstreamCost.balanceQuery.providerSub2Api') },
  { value: UPSTREAM_BALANCE_PROVIDER_NEW_API, label: t('admin.accounts.upstreamCost.balanceQuery.providerNewApi') }
])
const balanceProviderLabel = computed(() => {
  const option = balanceProviderOptions.value.find(item => item.value === balanceProvider.value)
  return option?.label || t('admin.accounts.upstreamCost.balanceQuery.providerSub2Api')
})
const balanceAuthModeOptions = computed<Array<{ value: UpstreamBalanceAuthMode; label: string }>>(() => {
  const options: Array<{ value: UpstreamBalanceAuthMode; label: string }> = [
    { value: UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY, label: t('admin.accounts.upstreamCost.balanceQuery.authModeAccountKey') },
    { value: UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN, label: t('admin.accounts.upstreamCost.balanceQuery.authModeBearer') },
    { value: UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER, label: t('admin.accounts.upstreamCost.balanceQuery.authModeCustomHeader') }
  ]
  return options
})

const emitProfile = (patch: Partial<UpstreamCostProfile>) => {
  emit('update:modelValue', {
    ...profile.value,
    ...patch
  })
}

const parseInputNumber = (event: Event): number | undefined => {
  const value = (event.target as HTMLInputElement).value
  if (!value.trim()) return undefined
  const num = Number(value)
  return Number.isFinite(num) && num > 0 ? num : undefined
}

const updateNumber = (key: 'recharge_cny_per_usd' | 'reference_fx_rate' | 'group_multiplier', event: Event) => {
  emitProfile({ [key]: parseInputNumber(event) })
}

const updateText = (key: 'note', event: Event) => {
  const value = (event.target as HTMLInputElement).value.trim()
  emitProfile({ [key]: value || undefined })
}

const updateBalanceEnabled = (enabled: boolean) => {
  const provider = balanceProvider.value
  emitProfile({
    balance_query_enabled: enabled,
    balance_provider: enabled ? provider : undefined,
    balance_endpoint: enabled ? balanceEndpoint.value : undefined,
    balance_auth_mode: enabled ? normalizeUpstreamBalanceAuthMode(provider, profile.value.balance_auth_mode) : undefined,
    balance_auth_header: enabled ? profile.value.balance_auth_header : undefined
  })
}

const updateBalanceProvider = (provider: UpstreamBalanceProvider) => {
  emitProfile({
    balance_query_enabled: true,
    balance_provider: provider,
    balance_endpoint: defaultUpstreamBalanceEndpoint(provider),
    balance_auth_mode: defaultUpstreamBalanceAuthMode(provider),
    balance_auth_header: undefined
  })
}

const updateBalanceEndpoint = (event: Event) => {
  const value = (event.target as HTMLInputElement).value.trim()
  emitProfile({
    balance_query_enabled: true,
    balance_provider: balanceProvider.value,
    balance_endpoint: value || balanceDefaultEndpoint.value,
    balance_auth_mode: balanceAuthMode.value
  })
}

const updateBalanceAuthMode = (mode: UpstreamBalanceAuthMode) => {
  emitProfile({
    balance_query_enabled: true,
    balance_provider: balanceProvider.value,
    balance_endpoint: balanceEndpoint.value,
    balance_auth_mode: mode,
    balance_auth_header: mode === UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER
      ? (profile.value.balance_auth_header || 'Authorization')
      : undefined
  })
}

const updateBalanceAuthHeader = (event: Event) => {
  const value = (event.target as HTMLInputElement).value.trim()
  emitProfile({
    balance_query_enabled: true,
    balance_provider: balanceProvider.value,
    balance_endpoint: balanceEndpoint.value,
    balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER,
    balance_auth_header: value || 'Authorization'
  })
}

const updateBalanceAuthToken = (event: Event) => {
  emit('update:balanceAuthTokenValue', (event.target as HTMLInputElement).value)
}

const emitFamilies = (nextFamilies: UpstreamCostFamilyOverride[]) => {
  emitProfile({ model_families: nextFamilies })
}

const addFamily = () => {
  emitFamilies([...families.value, { family: '' }])
}

const removeFamily = (index: number) => {
  emitFamilies(families.value.filter((_, itemIndex) => itemIndex !== index))
}

const updateFamilyText = (index: number, key: 'family' | 'note', event: Event) => {
  const value = (event.target as HTMLInputElement).value.trim()
  emitFamilies(families.value.map((item, itemIndex) =>
    itemIndex === index
      ? key === 'family'
        ? { ...item, family: value }
        : { ...item, note: value || undefined }
      : item
  ))
}

const updateFamilyNumber = (index: number, event: Event) => {
  emitFamilies(families.value.map((item, itemIndex) =>
    itemIndex === index ? { ...item, group_multiplier: parseInputNumber(event) } : item
  ))
}

const previewFamily = (family: string) => calculateUpstreamCost(profile.value, family)

const missingFieldLabels = (fields: UpstreamCostMissingField[]) => fields.map((field) => {
  if (field === 'recharge_cny_per_usd') return t('admin.accounts.upstreamCost.rechargeCnyPerUsd')
  if (field === 'reference_fx_rate') return t('admin.accounts.upstreamCost.referenceFxRate')
  return t('admin.accounts.upstreamCost.groupMultiplier')
})

const formatRatio = (value?: number) => formatUpstreamRatio(value)
</script>
