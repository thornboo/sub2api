<template>
  <BaseDialog :show="show" :title="t('enterpriseMembers.copy.batchAdjustUsedUsage')" width="wide" @close="handleClose">
    <div class="space-y-5">
      <section class="rounded-2xl border border-amber-200 bg-amber-50/80 px-4 py-3 dark:border-amber-900/50 dark:bg-amber-950/20">
        <p class="text-sm font-semibold text-amber-950 dark:text-amber-100">{{ t('enterpriseMembers.dynamic.selectedMembers', { count: targets.length }) }}</p>
        <p class="mt-1 text-xs leading-5 text-amber-800/80 dark:text-amber-200/80">{{ t('enterpriseMembers.copy.batchUsageDeltaHint') }}</p>
      </section>

      <div class="grid gap-3 sm:grid-cols-2">
        <label v-for="field in fields" :key="field.key" class="rounded-2xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.04]">
          <span class="input-label">{{ field.label }}</span>
          <input v-model.number="delta[field.key]" class="input mt-2" type="number" :min="-ENTERPRISE_MEMBER_MAX_MONETARY_VALUE" :max="ENTERPRISE_MEMBER_MAX_MONETARY_VALUE" step="0.01" />
          <span class="mt-2 block text-xs text-stone-500">{{ t('enterpriseMembers.copy.signedDeltaHint') }}</span>
        </label>
      </div>

      <section v-if="hasChange" class="rounded-2xl border border-stone-200 p-4 dark:border-white/10">
        <h4 class="text-sm font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.batchAggregateImpact') }}</h4>
        <div class="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-4">
          <div v-for="field in fields" :key="field.key" class="rounded-xl bg-stone-50 px-3 py-2 dark:bg-white/[0.04]">
            <span class="block text-[11px] text-stone-500">{{ field.label }}</span>
            <strong class="mt-1 block text-sm tabular-nums text-stone-900 dark:text-white">{{ formatSigned(delta[field.key] * targets.length) }}</strong>
          </div>
        </div>
      </section>

      <section v-if="negativeTargets.length" class="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 dark:border-rose-900/50 dark:bg-rose-950/20">
        <p class="text-sm font-semibold text-rose-800 dark:text-rose-200">{{ t('enterpriseMembers.copy.batchUsageCannotBecomeNegative') }}</p>
        <p class="mt-1 text-xs text-rose-700 dark:text-rose-300">{{ negativeTargets.slice(0, 4).map(item => item.name).join(locale.startsWith('zh') ? '、' : ', ') }}</p>
      </section>
      <p v-else class="text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.batchUsageAuditHint') }}</p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="saving" @click="handleClose">{{ t('enterpriseMembers.copy.cancel') }}</button>
        <button class="btn btn-danger" type="button" :disabled="!canSubmit || saving" @click="confirmOpen = true">{{ saving ? t('enterpriseMembers.copy.saving') : t('enterpriseMembers.copy.reviewUsageAdjustment') }}</button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="confirmOpen"
    :title="t('enterpriseMembers.copy.confirmBatchUsageAdjustment')"
    :message="t('enterpriseMembers.dynamic.confirmBatchUsageMessage', { count: targets.length })"
    :confirm-text="t('enterpriseMembers.copy.applyUsageAdjustment')"
    :danger="true"
    @confirm="submit"
    @cancel="confirmOpen = false"
  />
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { ENTERPRISE_MEMBER_MAX_MONETARY_VALUE, type EnterpriseMemberUsageDeltaInput } from '@/api/enterpriseMembers'

export interface EnterpriseMemberBatchUsageTarget {
  id: number
  name: string
  monthlyUsed: number
  usage5h: number
  usage1d: number
  usage7d: number
}

const props = defineProps<{ show: boolean; targets: EnterpriseMemberBatchUsageTarget[]; saving: boolean }>()
const emit = defineEmits<{ close: []; submit: [input: EnterpriseMemberUsageDeltaInput] }>()
const { t, locale } = useI18n()
type DeltaKey = keyof EnterpriseMemberUsageDeltaInput
const delta = reactive<EnterpriseMemberUsageDeltaInput>({ monthly_used_delta: 0, usage_5h_delta: 0, usage_1d_delta: 0, usage_7d_delta: 0 })
const confirmOpen = ref(false)
const fields = computed<Array<{ key: DeltaKey; label: string }>>(() => [
  { key: 'monthly_used_delta', label: t('enterpriseMembers.copy.currentMonthUsedDeltaUsd') },
  { key: 'usage_5h_delta', label: t('enterpriseMembers.copy.usage5hDelta') },
  { key: 'usage_1d_delta', label: t('enterpriseMembers.copy.usage1dDelta') },
  { key: 'usage_7d_delta', label: t('enterpriseMembers.copy.usage7dDelta') }
])
const hasInvalidValue = computed(() => fields.value.some(field => !Number.isFinite(delta[field.key]) || Math.abs(delta[field.key]) > ENTERPRISE_MEMBER_MAX_MONETARY_VALUE))
const hasChange = computed(() => fields.value.some(field => Math.abs(delta[field.key]) > 0.00000001))
const negativeTargets = computed(() => props.targets.filter(target =>
  target.monthlyUsed + delta.monthly_used_delta < -0.00000001 ||
  target.usage5h + delta.usage_5h_delta < -0.00000001 ||
  target.usage1d + delta.usage_1d_delta < -0.00000001 ||
  target.usage7d + delta.usage_7d_delta < -0.00000001
))
const canSubmit = computed(() => props.targets.length > 0 && props.targets.length <= 500 && hasChange.value && !hasInvalidValue.value && negativeTargets.value.length === 0)

watch(() => props.show, (show) => {
  if (!show) return
  Object.assign(delta, { monthly_used_delta: 0, usage_5h_delta: 0, usage_1d_delta: 0, usage_7d_delta: 0 })
  confirmOpen.value = false
})

function handleClose() {
  if (!props.saving) emit('close')
}

function submit() {
  if (!canSubmit.value) return
  confirmOpen.value = false
  emit('submit', { ...delta })
}

function formatSigned(value: number) {
  const formatted = new Intl.NumberFormat(locale.value, { maximumFractionDigits: 4 }).format(Math.abs(value))
  return `${value > 0 ? '+' : value < 0 ? '−' : ''}${formatted}`
}
</script>
