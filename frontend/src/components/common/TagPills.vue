<template>
  <span class="inline-flex flex-wrap gap-1">
    <template v-if="normalizedTags.length > 0">
      <span
        v-for="tag in visibleTags"
        :key="tag"
        class="inline-flex max-w-[96px] items-center truncate rounded-md border border-emerald-200/70 bg-emerald-50 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-200"
        :title="tag"
      >
        {{ tag }}
      </span>
      <span
        v-if="hiddenCount > 0"
        class="group/tag-more relative inline-flex"
      >
        <span
          class="inline-flex cursor-help items-center rounded-md border border-gray-200 bg-gray-50 px-1.5 py-0.5 text-xs font-medium text-gray-500 dark:border-white/10 dark:bg-white/5 dark:text-dark-300"
          :title="hiddenTitle"
        >
          +{{ hiddenCount }}
        </span>
        <span
          class="pointer-events-none absolute left-1/2 top-full z-50 mt-1 hidden min-w-max max-w-64 -translate-x-1/2 flex-wrap gap-1 rounded-lg border border-gray-200 bg-white p-2 shadow-lg group-hover/tag-more:flex dark:border-white/10 dark:bg-neutral-950"
        >
          <span
            v-for="tag in hiddenTags"
            :key="tag"
            class="inline-flex max-w-[120px] items-center truncate rounded-md border border-emerald-200/70 bg-emerald-50 px-1.5 py-0.5 text-xs font-medium text-emerald-800 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-200"
            :title="tag"
          >
            {{ tag }}
          </span>
        </span>
      </span>
    </template>
    <span v-else-if="emptyLabel" class="text-sm text-gray-400 dark:text-dark-500">
      {{ emptyLabel }}
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  tags?: string[] | null
  limit?: number
  emptyLabel?: string
}>(), {
  tags: () => [],
  limit: 2,
  emptyLabel: ''
})

const normalizedTags = computed(() => {
  const rawTags = Array.isArray(props.tags) ? props.tags : []
  return rawTags.filter((tag): tag is string => typeof tag === 'string' && tag.trim().length > 0)
})
const visibleTags = computed(() => normalizedTags.value.slice(0, props.limit))
const hiddenTags = computed(() => normalizedTags.value.slice(props.limit))
const hiddenCount = computed(() => hiddenTags.value.length)
const hiddenTitle = computed(() => hiddenTags.value.join(', '))
</script>
