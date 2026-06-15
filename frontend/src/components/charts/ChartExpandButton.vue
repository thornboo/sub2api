<template>
  <button
    type="button"
    class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-stone-200/80 text-stone-500 transition hover:border-emerald-400 hover:bg-emerald-50 hover:text-emerald-600 focus:outline-none focus:ring-2 focus:ring-emerald-500/40 dark:border-white/10 dark:text-stone-400 dark:hover:border-emerald-500/50 dark:hover:bg-emerald-500/10 dark:hover:text-emerald-300"
    :title="buttonLabel"
    :aria-label="buttonLabel"
  >
    <Icon name="arrowsPointingOut" size="sm" :stroke-width="1.8" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  label?: string
}>(), {
  label: ''
})

const { t } = useI18n()

const buttonLabel = computed(() => props.label || t('common.expand'))
</script>
