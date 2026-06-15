<template>
  <Popover v-model:open="open">
    <PopoverTrigger as-child>
      <Button
        type="button"
        variant="outline"
        :disabled="disabled"
        :class="cn(
          'h-10 w-full justify-start px-3 text-left font-normal tabular-nums',
          !modelValue && 'text-stone-400 dark:text-stone-500',
        )"
      >
        <Icon name="calendar" size="sm" class="text-stone-400 dark:text-stone-500" />
        <span>{{ displayValue }}</span>
        <Icon name="chevronDown" size="sm" class="ml-auto text-stone-400 dark:text-stone-500" />
      </Button>
    </PopoverTrigger>
    <PopoverContent class="w-auto overflow-hidden" align="start">
      <Calendar
        :model-value="selectedDate"
        :default-placeholder="defaultPlaceholder"
        :min-value="minDate"
        :max-value="maxDate"
        :locale="calendarLocale"
        :week-starts-on="0"
        weekday-format="short"
        fixed-weeks
        initial-focus
        @update:model-value="handleCalendarUpdate"
      />
    </PopoverContent>
  </Popover>
</template>

<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DateValue } from '@internationalized/date'
import { getLocalTimeZone, parseDate, today } from '@internationalized/date'
import Icon from '@/components/icons/Icon.vue'
import { Button } from '@/components/ui/button'
import { Calendar } from '@/components/ui/calendar'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { cn } from '@/lib/utils'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  disabled?: boolean
  min?: string
  max?: string
}>(), {
  placeholder: 'Pick a date',
  disabled: false,
  min: undefined,
  max: undefined,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  change: [value: string]
}>()

const { locale } = useI18n()
const open = ref(false)

const parseDateValue = (value?: string): DateValue | undefined => {
  if (!value) return undefined
  try {
    return parseDate(value)
  } catch {
    return undefined
  }
}

const selectedDate = shallowRef<DateValue | undefined>(parseDateValue(props.modelValue))
const minDate = computed(() => parseDateValue(props.min))
const maxDate = computed(() => parseDateValue(props.max))
const defaultPlaceholder = computed(() => selectedDate.value || today(getLocalTimeZone()))
const calendarLocale = computed(() => locale.value === 'zh' ? 'zh-CN' : 'en-US')
const displayValue = computed(() => props.modelValue ? props.modelValue.replace(/-/g, '/') : props.placeholder)

const handleCalendarUpdate = (date: DateValue | DateValue[] | undefined) => {
  if (!date || Array.isArray(date)) return
  const nextValue = date.toString()
  selectedDate.value = date
  emit('update:modelValue', nextValue)
  emit('change', nextValue)
  open.value = false
}

watch(
  () => props.modelValue,
  (value) => {
    selectedDate.value = parseDateValue(value)
  }
)
</script>
