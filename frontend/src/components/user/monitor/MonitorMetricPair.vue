<template>
  <div class="mt-5 grid grid-cols-2 gap-2">
    <div
      class="rounded-xl border border-stone-200/70 bg-white/60 p-3 dark:border-white/10 dark:bg-black/25"
    >
      <div
        class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-stone-400 dark:text-stone-500"
      >
        <Icon :name="primaryIcon" size="xs" />
        <span>{{ primaryLabel }}</span>
      </div>
      <div class="mt-1.5 text-lg font-bold font-mono tabular-nums text-stone-950 dark:text-stone-100">
        {{ primaryValue }}<span class="text-xs font-normal text-stone-400 ml-0.5">{{ primaryUnit }}</span>
      </div>
    </div>
    <div
      class="rounded-xl border border-stone-200/70 bg-white/60 p-3 dark:border-white/10 dark:bg-black/25"
    >
      <div
        class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-stone-400 dark:text-stone-500"
      >
        <Icon :name="secondaryIcon" size="xs" />
        <span>{{ secondaryLabel }}</span>
      </div>
      <div class="mt-1.5 text-lg font-bold font-mono tabular-nums text-stone-950 dark:text-stone-100">
        {{ secondaryValue }}<span class="text-xs font-normal text-stone-400 ml-0.5">{{ secondaryUnit }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  primaryLabel: string
  primaryValue: string
  primaryUnit: string
  primaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
  secondaryLabel: string
  secondaryValue: string
  secondaryUnit: string
  secondaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
}>()
</script>
