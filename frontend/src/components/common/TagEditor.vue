<template>
  <div
    :class="[
      'flex w-full flex-wrap items-center gap-1.5 rounded-lg border bg-white transition-colors dark:bg-neutral-950/70',
      compact ? 'min-h-10 px-2 py-1.5' : 'min-h-11 px-2.5 py-2',
      disabled
        ? 'cursor-not-allowed border-gray-200 opacity-70 dark:border-white/10'
        : 'cursor-text border-gray-300 focus-within:border-emerald-500 focus-within:ring-2 focus-within:ring-emerald-500/20 dark:border-white/10 dark:focus-within:border-emerald-400 dark:focus-within:ring-emerald-400/15'
    ]"
    @click="focusInput"
  >
    <span
      v-for="tag in tags"
      :key="tag"
      class="inline-flex max-w-[150px] items-center gap-1 rounded-md border border-emerald-200/70 bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-800 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-200"
      :title="tag"
    >
      <span class="truncate">{{ tag }}</span>
      <button
        type="button"
        class="rounded text-emerald-700/70 transition hover:bg-emerald-100 hover:text-emerald-900 focus:outline-none focus:ring-2 focus:ring-emerald-500/30 dark:text-emerald-200/70 dark:hover:bg-emerald-400/10 dark:hover:text-emerald-100"
        :aria-label="`${removeLabel} ${tag}`"
        :disabled="disabled"
        @click.stop="removeTag(tag)"
      >
        <Icon name="x" size="xs" :stroke-width="2" />
      </button>
    </span>

    <input
      ref="inputRef"
      v-model="draft"
      type="text"
      class="min-w-[8rem] flex-1 border-0 bg-transparent p-0 text-sm text-gray-900 placeholder:text-gray-400 focus:outline-none focus:ring-0 dark:text-white dark:placeholder:text-dark-400"
      :placeholder="tags.length === 0 ? placeholder : addPlaceholder"
      :disabled="disabled"
      @keydown="handleKeydown"
      @paste="handlePaste"
      @blur="commitDraft"
    />

    <button
      v-if="showAddButton"
      type="button"
      :disabled="disabled || !canAddDraft"
      :class="[
        'inline-flex shrink-0 items-center justify-center rounded-md border text-xs font-medium transition focus:outline-none focus:ring-2 focus:ring-emerald-500/30',
        compact ? 'h-7 w-7 px-0' : 'h-8 gap-1.5 px-2.5',
        canAddDraft && !disabled
          ? 'border-emerald-300 bg-emerald-50 text-emerald-700 hover:border-emerald-400 hover:bg-emerald-100 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-200 dark:hover:bg-emerald-500/20'
          : 'cursor-not-allowed border-gray-200 bg-gray-50 text-gray-400 dark:border-white/10 dark:bg-white/5 dark:text-dark-400'
      ]"
      @click.stop="commitDraft"
    >
      <Icon name="plus" size="xs" :stroke-width="2" />
      <span v-if="!compact">{{ addLabel }}</span>
      <span v-else class="sr-only">{{ addLabel }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  addPlaceholder?: string
  addLabel?: string
  removeLabel?: string
  maxTags?: number
  maxTagLength?: number
  disabled?: boolean
  compact?: boolean
  showAddButton?: boolean
}>(), {
  placeholder: '',
  addPlaceholder: '',
  addLabel: 'Add',
  removeLabel: 'Remove',
  maxTags: 20,
  maxTagLength: 40,
  disabled: false,
  compact: false,
  showAddButton: true
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
  (event: 'commit', value: string[]): void
  (event: 'invalid', reason: 'too_many' | 'too_long', value?: string): void
}>()

const draft = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

const normalizeTag = (tag: string) => tag.trim().toLowerCase()

const parseTagString = (value: string): string[] => {
  const seen = new Set<string>()
  const out: string[] = []
  value
    .split(/[,\n\r，；;]+/)
    .map(normalizeTag)
    .filter(Boolean)
    .forEach((tag) => {
      if (!seen.has(tag)) {
        seen.add(tag)
        out.push(tag)
      }
    })
  return out
}

const tags = computed(() => parseTagString(props.modelValue || ''))
const canAddDraft = computed(() => normalizeTag(draft.value).length > 0 && tags.value.length < props.maxTags)

const emitTags = (nextTags: string[]) => {
  emit('update:modelValue', nextTags.join(', '))
  emit('commit', nextTags)
}

const addTags = (rawTags: string[]) => {
  if (props.disabled) return

  const next = [...tags.value]
  const seen = new Set(next)
  let changed = false

  for (const rawTag of rawTags) {
    const tag = normalizeTag(rawTag)
    if (!tag || seen.has(tag)) continue

    if (Array.from(tag).length > props.maxTagLength) {
      emit('invalid', 'too_long', tag)
      return
    }
    if (next.length >= props.maxTags) {
      emit('invalid', 'too_many', tag)
      return
    }

    next.push(tag)
    seen.add(tag)
    changed = true
  }

  draft.value = ''
  if (changed) {
    emitTags(next)
  }
}

const commitDraft = () => {
  addTags(parseTagString(draft.value))
}

const removeTag = (tag: string) => {
  if (props.disabled) return
  emitTags(tags.value.filter((item) => item !== tag))
}

const handleKeydown = (event: KeyboardEvent) => {
  if (event.isComposing) return

  if (event.key === 'Enter' || event.key === ',') {
    event.preventDefault()
    commitDraft()
    return
  }

  if (event.key === 'Backspace' && draft.value === '' && tags.value.length > 0) {
    event.preventDefault()
    removeTag(tags.value[tags.value.length - 1])
  }
}

const handlePaste = (event: ClipboardEvent) => {
  const text = event.clipboardData?.getData('text') || ''
  if (!/[,\n\r，；;]/.test(text)) return

  event.preventDefault()
  addTags(parseTagString(text))
}

const focusInput = () => {
  if (!props.disabled) {
    inputRef.value?.focus()
  }
}
</script>
