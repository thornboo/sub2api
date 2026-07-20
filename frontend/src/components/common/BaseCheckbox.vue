<template>
  <span
    class="inline-flex shrink-0 items-center justify-center"
    :class="disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer'"
    @click="activateInput"
  >
    <input
      ref="inputRef"
      type="checkbox"
      class="peer sr-only"
      :checked="modelValue"
      :disabled="disabled"
      :indeterminate="indeterminate"
      :aria-label="ariaLabel"
      :aria-checked="indeterminate ? 'mixed' : modelValue"
      :data-test="dataTest"
      @change="handleChange"
    />
    <span :class="boxClasses">
      <Icon
        v-if="modelValue && !indeterminate"
        name="check"
        :size="size === 'sm' ? 'xs' : 'sm'"
        :stroke-width="2.75"
      />
      <span
        v-else-if="indeterminate"
        class="rounded-full bg-current"
        :class="size === 'sm' ? 'h-0.5 w-2.5' : 'h-0.5 w-3'"
      />
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  disabled?: boolean
  indeterminate?: boolean
  ariaLabel?: string
  dataTest?: string
  size?: 'sm' | 'md'
}>(), {
  disabled: false,
  indeterminate: false,
  ariaLabel: undefined,
  dataTest: undefined,
  size: 'md',
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  change: [value: boolean]
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const active = computed(() => props.modelValue || props.indeterminate)

const boxClasses = computed(() => [
  'inline-flex items-center justify-center rounded-md border shadow-sm transition-all duration-150',
  'peer-focus-visible:outline-none peer-focus-visible:ring-2 peer-focus-visible:ring-emerald-500/35 peer-focus-visible:ring-offset-1 peer-focus-visible:ring-offset-white dark:peer-focus-visible:ring-offset-black',
  props.size === 'sm' ? 'h-4 w-4' : 'h-5 w-5',
  active.value
    ? 'border-emerald-500 bg-emerald-500 text-neutral-950 shadow-emerald-500/20 hover:border-emerald-400 hover:bg-emerald-400 dark:border-emerald-400 dark:bg-emerald-400 dark:text-black dark:hover:bg-emerald-300'
    : 'border-stone-300/80 bg-white/80 text-transparent hover:border-emerald-500/40 hover:bg-emerald-50/60 dark:border-white/10 dark:bg-neutral-950/70 dark:shadow-black/20 dark:hover:border-emerald-400/45 dark:hover:bg-emerald-500/5',
])

function handleChange(event: Event) {
  const checked = (event.target as HTMLInputElement).checked
  emitChecked(checked)
}

function emitChecked(checked: boolean) {
  emit('update:modelValue', checked)
  emit('change', checked)
}

function activateInput(event: MouseEvent) {
  if (props.disabled || event.target === inputRef.value) return
  event.preventDefault()
  inputRef.value?.focus()
  emitChecked(!props.modelValue)
}
</script>
