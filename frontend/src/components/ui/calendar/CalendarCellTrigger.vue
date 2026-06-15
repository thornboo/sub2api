<script lang="ts" setup>
import type { CalendarCellTriggerProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { CalendarCellTrigger, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'
import { buttonVariants } from '@/components/ui/button'

const props = defineProps<CalendarCellTriggerProps & { class?: HTMLAttributes['class'] }>()
const delegatedProps = reactiveOmit(props, 'class')
const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <CalendarCellTrigger
    :class="cn(
      buttonVariants({ variant: 'ghost' }),
      'h-9 w-9 rounded-xl p-0 font-normal tabular-nums',
      '[&[data-today]:not([data-selected])]:bg-stone-100 [&[data-today]:not([data-selected])]:text-stone-950 dark:[&[data-today]:not([data-selected])]:bg-white/10 dark:[&[data-today]:not([data-selected])]:text-white',
      'data-[selected]:bg-primary-500 data-[selected]:text-white data-[selected]:opacity-100 data-[selected]:hover:bg-primary-500 data-[selected]:hover:text-white data-[selected]:focus:bg-primary-500 data-[selected]:focus:text-white',
      'data-[disabled]:text-stone-400 data-[disabled]:opacity-40 data-[disabled]:dark:text-stone-600',
      'data-[unavailable]:text-red-500 data-[unavailable]:line-through',
      'data-[outside-view]:text-stone-400 data-[outside-view]:opacity-60 dark:data-[outside-view]:text-stone-600 [&[data-outside-view][data-selected]]:bg-stone-100 [&[data-outside-view][data-selected]]:text-stone-400 dark:[&[data-outside-view][data-selected]]:bg-white/5 dark:[&[data-outside-view][data-selected]]:text-stone-600',
      props.class,
    )"
    v-bind="forwardedProps"
  >
    <slot />
  </CalendarCellTrigger>
</template>
