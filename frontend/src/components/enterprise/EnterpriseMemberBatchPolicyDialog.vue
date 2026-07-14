<template>
  <BaseDialog :show="show" :title="t('enterpriseMembers.copy.batchEditMembers')" width="extra-wide" @close="handleClose">
    <div class="space-y-5">
      <section class="rounded-2xl border border-emerald-200 bg-emerald-50/70 px-4 py-3 dark:border-emerald-800/50 dark:bg-emerald-950/20">
        <p class="text-sm font-semibold text-emerald-950 dark:text-emerald-100">{{ t('enterpriseMembers.dynamic.selectedMembers', { count: memberCount }) }}</p>
        <p class="mt-1 text-xs leading-5 text-emerald-800/80 dark:text-emerald-200/80">{{ t('enterpriseMembers.copy.batchEditExplicitFieldsHint') }}</p>
      </section>

      <section class="rounded-2xl border border-stone-200 p-4 dark:border-white/10">
        <div>
          <h4 class="text-sm font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.memberSpendingLimits') }}</h4>
          <p class="mt-1 text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.batchLimitHint') }}</p>
        </div>
        <div class="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-4">
          <div v-for="field in limitFields" :key="field.key" class="rounded-2xl bg-stone-50 p-3 dark:bg-white/[0.04]">
            <label class="flex w-fit cursor-pointer items-center gap-2 text-sm font-medium text-stone-800 dark:text-stone-100">
              <BaseCheckbox v-model="enabled[field.key]" size="sm" :aria-label="field.label" />
              {{ field.label }}
            </label>
            <input v-model.number="values[field.key]" class="input mt-3" type="number" min="0" :max="ENTERPRISE_MEMBER_MAX_MONETARY_VALUE" step="0.01" :aria-label="field.label" :disabled="!enabled[field.key]" />
          </div>
        </div>
      </section>

      <section class="grid gap-4 lg:grid-cols-[minmax(0,0.72fr)_minmax(0,1.28fr)]">
        <div class="rounded-2xl border border-stone-200 p-4 dark:border-white/10">
          <label class="flex items-start gap-3">
            <BaseCheckbox v-model="enabled.status" class="mt-0.5" :aria-label="t('enterpriseMembers.copy.batchChangeStatus')" />
            <span>
              <b class="block text-sm text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.batchChangeStatus') }}</b>
              <span class="mt-1 block text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.batchStatusHint') }}</span>
            </span>
          </label>
          <Select v-model="status" :options="statusOptions" class="mt-4 w-full" :disabled="!enabled.status" />
        </div>

        <div class="rounded-2xl border border-stone-200 p-4 dark:border-white/10">
          <label class="flex items-start gap-3">
            <BaseCheckbox v-model="enabled.groups" class="mt-0.5" :aria-label="t('enterpriseMembers.copy.batchChangeGroups')" />
            <span>
              <b class="block text-sm text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.batchChangeGroups') }}</b>
              <span class="mt-1 block text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.batchGroupPolicyHint') }}</span>
            </span>
          </label>
          <Select v-model="groupMode" :options="groupModeOptions" class="mt-4 w-full" :disabled="!enabled.groups" />
          <div v-if="enabled.groups" class="mt-3 max-h-64 space-y-2 overflow-y-auto rounded-2xl border border-stone-200 p-2 dark:border-white/10">
            <p v-if="!availableGroups.length" class="px-3 py-8 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.noGroupData') }}</p>
            <div v-for="group in availableGroups" :key="group.id" class="flex items-center gap-3 rounded-xl px-3 py-2.5 hover:bg-stone-50 dark:hover:bg-white/5">
              <BaseCheckbox :model-value="groupIds.includes(group.id)" size="sm" :aria-label="group.name" @update:model-value="toggleGroup(group.id)" />
              <span class="min-w-0 flex-1"><b class="block truncate text-sm text-stone-900 dark:text-white">{{ group.name }}</b><span class="text-xs text-stone-500">{{ group.platform }}</span></span>
              <template v-if="groupIds.includes(group.id)">
                <span class="rounded-lg bg-amber-100 px-2 py-1 text-xs font-bold text-amber-800 dark:bg-amber-300/10 dark:text-amber-200">#{{ groupIds.indexOf(group.id) + 1 }}</span>
                <button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveUp')" @click.prevent="moveGroup(group.id, -1)">↑</button>
                <button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveDown')" @click.prevent="moveGroup(group.id, 1)">↓</button>
              </template>
            </div>
          </div>
          <p v-if="enabled.groups && groupMode === 'replace' && groupIds.length === 0" class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t('enterpriseMembers.copy.emptyReplaceClearsGroupsAndMembersCannotBeEnabled') }}</p>
        </div>
      </section>

      <p v-if="hasInvalidLimit" class="text-sm text-rose-600 dark:text-rose-300">{{ t('enterpriseMembers.copy.batchLimitMustBeNonnegative') }}</p>
      <p v-if="invalidActivation" class="text-sm text-rose-600 dark:text-rose-300">{{ t('enterpriseMembers.copy.batchEnableRequiresGroups') }}</p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="saving" @click="handleClose">{{ t('enterpriseMembers.copy.cancel') }}</button>
        <button class="btn" :class="destructive ? 'btn-danger' : 'btn-primary'" type="button" :disabled="!canSubmit || saving" @click="confirmOpen = true">
          {{ saving ? t('enterpriseMembers.copy.saving') : t('enterpriseMembers.copy.reviewBatchChanges') }}
        </button>
      </div>
    </template>
  </BaseDialog>

  <ConfirmDialog
    :show="confirmOpen"
    :title="t('enterpriseMembers.copy.confirmBatchEdit')"
    :message="t('enterpriseMembers.dynamic.confirmBatchEditMessage', { count: memberCount, changes: enabledCount })"
    :confirm-text="t('enterpriseMembers.copy.applyBatchChanges')"
    :danger="destructive"
    @confirm="submit"
    @cancel="confirmOpen = false"
  />
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseCheckbox from '@/components/common/BaseCheckbox.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import { ENTERPRISE_MEMBER_MAX_MONETARY_VALUE, type EnterpriseMemberBatchPolicyInput, type EnterpriseMemberStatus } from '@/api/enterpriseMembers'
import type { Group } from '@/types'

const props = defineProps<{ show: boolean; memberCount: number; availableGroups: Group[]; saving: boolean }>()
const emit = defineEmits<{ close: []; submit: [input: EnterpriseMemberBatchPolicyInput] }>()
const { t } = useI18n()

type LimitKey = 'monthly_limit_usd' | 'rate_limit_5h' | 'rate_limit_1d' | 'rate_limit_7d'
const enabled = reactive<Record<LimitKey | 'status' | 'groups', boolean>>({
  monthly_limit_usd: false,
  rate_limit_5h: false,
  rate_limit_1d: false,
  rate_limit_7d: false,
  status: false,
  groups: false
})
const values = reactive<Record<LimitKey, number>>({ monthly_limit_usd: 0, rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 })
const status = ref<EnterpriseMemberStatus>('active')
const groupMode = ref<'replace' | 'append'>('replace')
const groupIds = ref<number[]>([])
const confirmOpen = ref(false)

const limitFields = computed<Array<{ key: LimitKey; label: string }>>(() => [
  { key: 'monthly_limit_usd', label: t('enterpriseMembers.copy.calendarMonthBudgetUsd') },
  { key: 'rate_limit_5h', label: t('enterpriseMembers.copy.rateLimit5h') },
  { key: 'rate_limit_1d', label: t('enterpriseMembers.copy.rateLimit1d') },
  { key: 'rate_limit_7d', label: t('enterpriseMembers.copy.rateLimit7d') }
])
const statusOptions = computed<SelectOption[]>(() => [
  { value: 'active', label: t('enterpriseMembers.copy.active') },
  { value: 'disabled', label: t('enterpriseMembers.copy.disabled') }
])
const groupModeOptions = computed<SelectOption[]>(() => [
  { value: 'replace', label: t('enterpriseMembers.copy.replaceAccessibleGroups') },
  { value: 'append', label: t('enterpriseMembers.copy.appendAccessibleGroups') }
])
const enabledCount = computed(() => Object.values(enabled).filter(Boolean).length)
const hasInvalidLimit = computed(() => limitFields.value.some(field => enabled[field.key] && (
  !Number.isFinite(values[field.key]) || values[field.key] < 0 || values[field.key] > ENTERPRISE_MEMBER_MAX_MONETARY_VALUE
)))
const invalidActivation = computed(() => enabled.status && status.value === 'active' && enabled.groups && groupMode.value === 'replace' && groupIds.value.length === 0)
const canSubmit = computed(() => props.memberCount > 0 && props.memberCount <= 500 && enabledCount.value > 0 && !hasInvalidLimit.value && !invalidActivation.value && !(enabled.groups && groupMode.value === 'append' && groupIds.value.length === 0))
const destructive = computed(() => (enabled.status && status.value === 'disabled') || (enabled.groups && groupMode.value === 'replace' && groupIds.value.length === 0))

watch(() => props.show, (show) => {
  if (!show) return
  Object.keys(enabled).forEach(key => { enabled[key as keyof typeof enabled] = false })
  Object.assign(values, { monthly_limit_usd: 0, rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 })
  status.value = 'active'
  groupMode.value = 'replace'
  groupIds.value = []
  confirmOpen.value = false
})

function toggleGroup(groupId: number) {
  groupIds.value = groupIds.value.includes(groupId) ? groupIds.value.filter(id => id !== groupId) : [...groupIds.value, groupId]
}

function moveGroup(groupId: number, direction: -1 | 1) {
  const current = groupIds.value.indexOf(groupId)
  const target = current + direction
  if (current < 0 || target < 0 || target >= groupIds.value.length) return
  const next = [...groupIds.value]
  ;[next[current], next[target]] = [next[target], next[current]]
  groupIds.value = next
}

function handleClose() {
  if (!props.saving) emit('close')
}

function submit() {
  if (!canSubmit.value) return
  confirmOpen.value = false
  const input: EnterpriseMemberBatchPolicyInput = { group_mode: 'keep' }
  limitFields.value.forEach(field => {
    if (enabled[field.key]) input[field.key] = values[field.key]
  })
  if (enabled.status) input.status = status.value
  if (enabled.groups) {
    input.group_mode = groupMode.value
    input.group_ids = [...groupIds.value]
  }
  emit('submit', input)
}
</script>
