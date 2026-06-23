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
  DEFAULT_UPSTREAM_REFERENCE_FX_RATE,
  calculateUpstreamCost,
  formatUpstreamRatio,
  type UpstreamCostFamilyOverride,
  type UpstreamCostMissingField,
  type UpstreamCostProfile
} from '@/utils/upstreamCost'

const props = defineProps<{
  modelValue?: UpstreamCostProfile
}>()

const emit = defineEmits<{
  'update:modelValue': [value: UpstreamCostProfile]
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
