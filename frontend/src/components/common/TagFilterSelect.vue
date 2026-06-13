<template>
  <div ref="rootRef" class="relative">
    <div
      role="combobox"
      tabindex="0"
      :aria-expanded="isOpen"
      class="flex min-h-11 w-full cursor-pointer items-center justify-between gap-2 rounded-lg border border-gray-300 bg-white px-3 py-2 transition-colors focus:outline-none focus:ring-2 focus:ring-emerald-500/25 dark:border-white/10 dark:bg-neutral-950/70 dark:focus:ring-emerald-400/15"
      @click="toggleOpen"
      @keydown.enter.prevent="toggleOpen"
      @keydown.space.prevent="toggleOpen"
      @keydown.esc.prevent="close"
    >
      <div class="min-w-0 flex-1">
        <div v-if="selectedTags.length > 0" class="flex flex-wrap gap-1">
          <span
            v-for="tag in visibleSelectedTags"
            :key="tag"
            class="inline-flex max-w-[96px] items-center gap-1 rounded-md border border-emerald-200/70 bg-emerald-50 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-200"
            :title="tag"
          >
            <span class="truncate">{{ tag }}</span>
            <button
              type="button"
              class="rounded text-emerald-700/70 transition hover:bg-emerald-100 hover:text-emerald-900 focus:outline-none focus:ring-2 focus:ring-emerald-500/30 dark:text-emerald-200/70 dark:hover:bg-emerald-400/10 dark:hover:text-emerald-100"
              :aria-label="`${removeLabel} ${tag}`"
              @click.stop="removeTag(tag)"
            >
              <Icon name="x" size="xs" :stroke-width="2" />
            </button>
          </span>
          <span
            v-if="selectedTags.length > maxVisible"
            class="inline-flex items-center rounded-md border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-xs font-medium text-gray-500 dark:border-white/10 dark:bg-white/5 dark:text-dark-300"
            :title="selectedTags.slice(maxVisible).join(', ')"
          >
            +{{ selectedTags.length - maxVisible }}
          </span>
        </div>
        <span v-else class="block truncate text-sm text-gray-400 dark:text-dark-400">
          {{ placeholder }}
        </span>
      </div>
      <Icon
        name="chevronDown"
        size="sm"
        class="shrink-0 text-gray-500 transition-transform dark:text-dark-400"
        :class="{ 'rotate-180': isOpen }"
      />
    </div>

    <div
      v-if="isOpen"
      class="absolute left-0 right-0 z-50 mt-1 max-h-64 overflow-auto rounded-lg border border-gray-200 bg-white py-1 shadow-lg dark:border-white/10 dark:bg-neutral-950"
    >
      <button
        v-if="selectedTags.length > 0"
        type="button"
        class="flex w-full items-center justify-between px-3 py-2 text-left text-sm text-gray-500 transition hover:bg-gray-50 dark:text-dark-300 dark:hover:bg-white/5"
        @click="clear"
      >
        <span>{{ clearLabel }}</span>
        <Icon name="x" size="xs" :stroke-width="2" />
      </button>

      <button
        v-for="tag in normalizedOptions"
        :key="tag"
        type="button"
        class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-800 transition hover:bg-gray-50 dark:text-dark-100 dark:hover:bg-white/5"
        @click="toggleTag(tag)"
      >
        <span
          :class="[
            'flex h-4 w-4 shrink-0 items-center justify-center rounded border',
            isSelected(tag)
              ? 'border-emerald-500 bg-emerald-500 text-black dark:border-emerald-400 dark:bg-emerald-400'
              : 'border-gray-300 dark:border-white/15'
          ]"
        >
          <Icon v-if="isSelected(tag)" name="check" size="xs" :stroke-width="2.5" />
        </span>
        <span class="truncate">{{ tag }}</span>
      </button>

      <div v-if="normalizedOptions.length === 0" class="px-3 py-3 text-sm text-gray-500 dark:text-dark-300">
        {{ emptyLabel }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  options?: string[]
  placeholder?: string
  emptyLabel?: string
  clearLabel?: string
  removeLabel?: string
  maxVisible?: number
}>(), {
  options: () => [],
  placeholder: '',
  emptyLabel: '',
  clearLabel: 'Clear',
  removeLabel: 'Remove',
  maxVisible: 2
})

const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
  (event: 'change', value: string[]): void
}>()

const rootRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)

const normalizeTag = (tag: string) => tag.trim().toLowerCase()

const uniqueTags = (tags: string[]) => {
  const seen = new Set<string>()
  const out: string[] = []
  for (const rawTag of tags) {
    const tag = normalizeTag(rawTag)
    if (!tag || seen.has(tag)) continue
    seen.add(tag)
    out.push(tag)
  }
  return out
}

const parseTagString = (value: string) => uniqueTags(value.split(/[,\n\r，；;]+/))

const selectedTags = computed(() => parseTagString(props.modelValue || ''))
const normalizedOptions = computed(() =>
  uniqueTags([...(props.options || []), ...selectedTags.value]).sort((a, b) => a.localeCompare(b))
)
const visibleSelectedTags = computed(() => selectedTags.value.slice(0, props.maxVisible))

const emitTags = (tags: string[]) => {
  emit('update:modelValue', tags.join(', '))
  emit('change', tags)
}

const isSelected = (tag: string) => selectedTags.value.includes(tag)

const toggleTag = (tag: string) => {
  if (isSelected(tag)) {
    emitTags(selectedTags.value.filter((item) => item !== tag))
    return
  }
  emitTags([...selectedTags.value, tag])
}

const removeTag = (tag: string) => {
  emitTags(selectedTags.value.filter((item) => item !== tag))
}

const clear = () => {
  emitTags([])
}

const toggleOpen = () => {
  isOpen.value = !isOpen.value
}

const close = () => {
  isOpen.value = false
}

const handleDocumentMouseDown = (event: MouseEvent) => {
  if (!rootRef.value?.contains(event.target as Node)) {
    close()
  }
}

onMounted(() => {
  document.addEventListener('mousedown', handleDocumentMouseDown)
})

onUnmounted(() => {
  document.removeEventListener('mousedown', handleDocumentMouseDown)
})
</script>
