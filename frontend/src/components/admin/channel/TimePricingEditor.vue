<template>
  <section class="mt-3 rounded-lg border border-stone-200/80 bg-stone-50/60 p-3 dark:border-white/10 dark:bg-white/[0.03]">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <label class="inline-flex items-center gap-2 text-xs font-medium text-stone-600 dark:text-stone-300">
        <BaseCheckbox
          :model-value="schedule.enabled"
          :aria-label="t('admin.channels.form.timePricingEnabled')"
          size="sm"
          @update:modelValue="setEnabled($event)"
        />
        <span>{{ t('admin.channels.form.timePricing') }}</span>
      </label>
      <div v-if="schedule.enabled" class="flex min-w-0 items-center gap-2">
        <label class="text-xs text-stone-400" :for="timezoneInputId">
          {{ t('admin.channels.form.timePricingTimezone') }}
        </label>
        <Select
          :id="timezoneInputId"
          :model-value="schedule.timezone"
          :options="timezoneOptions"
          :placeholder="t('admin.channels.form.timePricingTimezonePlaceholder')"
          :search-placeholder="t('admin.channels.form.timePricingTimezoneSearch')"
          :creatable-prefix="t('admin.channels.form.timePricingTimezoneUse')"
          :aria-label="t('admin.channels.form.timePricingTimezone')"
          class="w-56 max-w-[70vw] text-xs"
          searchable
          creatable
          @update:modelValue="updateTimezone"
        />
      </div>
    </div>

    <div v-if="schedule.enabled" class="mt-3 space-y-2">
      <div class="grid grid-cols-12 items-end gap-2 rounded-md border border-dashed border-stone-300/80 bg-white/65 px-2 py-2 text-xs dark:border-white/10 dark:bg-black/20">
        <div class="col-span-12 sm:col-span-4 sm:self-center">
          <span class="font-medium text-stone-600 dark:text-stone-300">{{ t('admin.channels.form.timePricingOtherTimes') }}</span>
        </div>
        <div class="col-span-7 sm:col-span-4">
          <label class="text-[11px] text-stone-400" :for="defaultLabelInputId">
            {{ t('admin.channels.form.timePricingTypeName') }}
          </label>
          <input
            :id="defaultLabelInputId"
            :value="schedule.default_label"
            type="text"
            maxlength="32"
            class="input mt-0.5 h-8 text-xs"
            :placeholder="t('admin.channels.form.timePricingDefaultLabelPlaceholder')"
            @input="updateSchedule({ default_label: ($event.target as HTMLInputElement).value })"
          />
        </div>
        <div class="col-span-5 sm:col-span-4">
          <label class="text-[11px] text-stone-400" :for="defaultMultiplierInputId">
            {{ t('admin.channels.form.timePricingDefaultMultiplier') }}
          </label>
          <div class="mt-0.5 flex items-center gap-2">
            <span class="text-xs text-stone-400">x</span>
            <input
              :id="defaultMultiplierInputId"
              :value="schedule.default_multiplier"
              type="number"
              min="0"
              max="100"
              step="0.01"
              class="input h-8 flex-1 text-right font-mono text-xs"
              @input="updateSchedule({ default_multiplier: ($event.target as HTMLInputElement).value })"
            />
          </div>
        </div>
        <p v-if="Number(schedule.default_multiplier) === 0" class="col-span-12 text-right text-[11px] text-amber-700 dark:text-amber-300">
          {{ t('admin.channels.form.timePricingDefaultFreeWarning') }}
        </p>
      </div>

      <div v-for="(rule, idx) in schedule.rules" :key="idx" class="grid grid-cols-12 items-start gap-2 rounded-md border border-stone-200/80 bg-white/80 p-2 dark:border-white/10 dark:bg-neutral-950/55">
        <div class="col-span-12 sm:col-span-3">
          <label class="text-[11px] text-stone-400" :for="`${ruleInputPrefix}-${idx}-label`">
            {{ t('admin.channels.form.timePricingTypeName') }}
          </label>
          <input
            :id="`${ruleInputPrefix}-${idx}-label`"
            :value="rule.label"
            type="text"
            maxlength="32"
            class="input mt-0.5 h-8 text-xs"
            :placeholder="t('admin.channels.form.timePricingRuleLabelPlaceholder')"
            @input="updateRule(idx, { label: ($event.target as HTMLInputElement).value })"
          />
        </div>
        <div class="col-span-6 sm:col-span-2">
          <label class="text-[11px] text-stone-400" :for="`${ruleInputPrefix}-${idx}-start`">
            {{ t('admin.channels.form.timePricingStart') }}
          </label>
          <input
            :id="`${ruleInputPrefix}-${idx}-start`"
            :value="rule.start_time"
            type="time"
            class="input mt-0.5 h-8 text-xs"
            @input="updateRule(idx, { start_time: ($event.target as HTMLInputElement).value })"
          />
        </div>
        <div class="col-span-6 sm:col-span-2">
          <label class="text-[11px] text-stone-400" :for="`${ruleInputPrefix}-${idx}-end`">
            {{ t('admin.channels.form.timePricingEnd') }}
          </label>
          <input
            :id="`${ruleInputPrefix}-${idx}-end`"
            :value="rule.end_time"
            type="time"
            class="input mt-0.5 h-8 text-xs"
            @input="updateRule(idx, { end_time: ($event.target as HTMLInputElement).value })"
          />
        </div>
        <div class="col-span-7 sm:col-span-3">
          <label class="text-[11px] text-stone-400" :for="`${ruleInputPrefix}-${idx}-multiplier`">
            {{ t('admin.channels.form.timePricingMultiplier') }}
          </label>
          <div class="mt-0.5 flex items-center gap-2">
            <input
              :id="`${ruleInputPrefix}-${idx}-multiplier`"
              :value="rule.multiplier"
              type="number"
              min="0"
              max="100"
              step="0.01"
              class="input h-8 text-xs"
              @input="updateRule(idx, { multiplier: ($event.target as HTMLInputElement).value })"
            />
          </div>
          <p v-if="isCrossMidnight(rule)" class="mt-1 text-[11px] text-stone-500 dark:text-stone-400">
            {{ t('admin.channels.form.timePricingCrossMidnight') }}
          </p>
          <p v-if="Number(rule.multiplier) === 0" class="mt-1 text-[11px] text-amber-700 dark:text-amber-300">
            {{ t('admin.channels.form.timePricingFreeWarning') }}
          </p>
        </div>
        <div class="col-span-5 flex justify-end gap-0.5 pt-5 sm:col-span-2">
          <button
            type="button"
            class="rounded p-1 text-stone-400 transition-colors hover:bg-stone-100 hover:text-stone-600 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-white/[0.06] dark:hover:text-stone-200"
            :disabled="idx === 0"
            :aria-label="t('admin.channels.form.timePricingMoveRuleUp')"
            :title="t('admin.channels.form.timePricingMoveRuleUp')"
            @click="moveRule(idx, -1)"
          >
            <Icon name="chevronUp" size="sm" />
          </button>
          <button
            type="button"
            class="rounded p-1 text-stone-400 transition-colors hover:bg-stone-100 hover:text-stone-600 disabled:cursor-not-allowed disabled:opacity-30 dark:hover:bg-white/[0.06] dark:hover:text-stone-200"
            :disabled="idx === schedule.rules.length - 1"
            :aria-label="t('admin.channels.form.timePricingMoveRuleDown')"
            :title="t('admin.channels.form.timePricingMoveRuleDown')"
            @click="moveRule(idx, 1)"
          >
            <Icon name="chevronDown" size="sm" />
          </button>
          <button
            type="button"
            class="rounded p-1 text-stone-400 transition-colors hover:bg-red-500/10 hover:text-red-500"
            :aria-label="t('admin.channels.form.timePricingRemoveRule')"
            :title="t('admin.channels.form.timePricingRemoveRule')"
            @click="removeRule(idx)"
          >
            <Icon name="trash" size="sm" />
          </button>
        </div>
      </div>

      <button
        type="button"
        class="inline-flex items-center gap-1 text-xs font-medium text-emerald-600 transition-colors hover:text-emerald-700 disabled:cursor-not-allowed disabled:text-stone-400 dark:text-emerald-400 dark:hover:text-emerald-300"
        :disabled="schedule.rules.length >= 16"
        @click="addRule"
      >
        <Icon name="plus" size="xs" />
        {{ t('admin.channels.form.timePricingAddRule') }}
      </button>
      <p class="text-[11px] text-stone-400">
        {{ t('admin.channels.form.timePricingHint') }}
      </p>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseCheckbox from '@/components/common/BaseCheckbox.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { TimePricingFormEntry, TimePricingRuleFormEntry } from './types'
import { createDefaultTimePricing } from './types'

const props = defineProps<{
  modelValue?: TimePricingFormEntry
}>()

const emit = defineEmits<{
  'update:modelValue': [value: TimePricingFormEntry]
}>()

const { t } = useI18n()

const schedule = computed(() => props.modelValue || createDefaultTimePricing())
const inputSeed = Math.random().toString(36).slice(2)
const timezoneInputId = `time-pricing-timezone-${inputSeed}`
const defaultLabelInputId = `time-pricing-default-label-${inputSeed}`
const defaultMultiplierInputId = `time-pricing-default-multiplier-${inputSeed}`
const ruleInputPrefix = `time-pricing-rule-${inputSeed}`
const fallbackTimezones = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Tokyo',
  'Asia/Singapore',
  'Asia/Seoul',
  'Asia/Kolkata',
  'Europe/London',
  'Europe/Paris',
  'Europe/Berlin',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'Australia/Sydney',
]

const timezoneOptions = computed<SelectOption[]>(() => {
  const detectedTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone
  const intl = Intl as typeof Intl & { supportedValuesOf?: (key: 'timeZone') => string[] }
  const supportedTimezones = intl.supportedValuesOf?.('timeZone') ?? fallbackTimezones
  const prioritized = Array.from(new Set(['Asia/Shanghai', detectedTimezone, 'UTC'].filter(Boolean)))
  const timezones = Array.from(new Set([...prioritized, ...supportedTimezones]))
  const priority = new Map(prioritized.map((timezone, index) => [timezone, index]))
  timezones.sort((left, right) => {
    const leftPriority = priority.get(left)
    const rightPriority = priority.get(right)
    if (leftPriority != null || rightPriority != null) {
      return (leftPriority ?? Number.MAX_SAFE_INTEGER) - (rightPriority ?? Number.MAX_SAFE_INTEGER)
    }
    return left.localeCompare(right)
  })
  return timezones.map(timezone => ({ value: timezone, label: timezone }))
})

function updateSchedule(patch: Partial<TimePricingFormEntry>) {
  emit('update:modelValue', {
    ...schedule.value,
    ...patch,
    rules: patch.rules || schedule.value.rules || [],
  })
}

function setEnabled(enabled: boolean) {
  updateSchedule({ enabled })
}

function updateTimezone(value: string | number | boolean | null) {
  updateSchedule({ timezone: value == null ? '' : String(value) })
}

function addRule() {
  if (schedule.value.rules.length >= 16) return
  updateSchedule({
    rules: [
      ...schedule.value.rules,
      { label: '', start_time: '09:00', end_time: '12:00', multiplier: 2 },
    ],
  })
}

function updateRule(index: number, patch: Partial<TimePricingRuleFormEntry>) {
  const rules = [...schedule.value.rules]
  rules[index] = { ...rules[index], ...patch }
  updateSchedule({ rules })
}

function removeRule(index: number) {
  const rules = [...schedule.value.rules]
  rules.splice(index, 1)
  updateSchedule({ rules })
}

function moveRule(index: number, offset: -1 | 1) {
  const target = index + offset
  if (target < 0 || target >= schedule.value.rules.length) return
  const rules = [...schedule.value.rules]
  const [rule] = rules.splice(index, 1)
  rules.splice(target, 0, rule)
  updateSchedule({ rules })
}

function isCrossMidnight(rule: TimePricingRuleFormEntry): boolean {
  return !!rule.start_time && !!rule.end_time && rule.start_time > rule.end_time
}
</script>
