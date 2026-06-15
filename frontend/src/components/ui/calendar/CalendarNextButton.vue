<script lang="ts" setup>
import type { CalendarNextProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { CalendarNext, useForwardProps } from 'reka-ui'
import Icon from '@/components/icons/Icon.vue'
import { cn } from '@/lib/utils'
import { buttonVariants } from '@/components/ui/button'

const props = defineProps<CalendarNextProps & { class?: HTMLAttributes['class'] }>()
const delegatedProps = reactiveOmit(props, 'class')
const forwardedProps = useForwardProps(delegatedProps)
</script>

<template>
  <CalendarNext
    :class="cn(
      buttonVariants({ variant: 'outline', size: 'icon-sm' }),
      'h-8 w-8 rounded-xl bg-transparent p-0 text-stone-500 hover:text-stone-950 dark:text-stone-400 dark:hover:text-white',
      props.class,
    )"
    v-bind="forwardedProps"
  >
    <slot>
      <Icon name="chevronRight" size="sm" />
    </slot>
  </CalendarNext>
</template>
