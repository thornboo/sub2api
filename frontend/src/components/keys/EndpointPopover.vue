<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import type { CustomEndpoint } from '@/types'

const props = defineProps<{
  apiBaseUrl: string
  customEndpoints: CustomEndpoint[]
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const copiedEndpoint = ref<string | null>(null)

let copiedResetTimer: number | undefined

const allEndpoints = computed(() => {
  const items: Array<{ name: string; endpoint: string; description: string; isDefault: boolean }> = []
  if (props.apiBaseUrl) {
    items.push({
      name: t('keys.endpoints.title'),
      endpoint: props.apiBaseUrl,
      description: '',
      isDefault: true,
    })
  }
  for (const ep of props.customEndpoints) {
    items.push({ ...ep, isDefault: false })
  }
  return items
})

async function copy(url: string) {
  const success = await copyToClipboard(url, t('keys.endpoints.copied'))
  if (!success) return

  copiedEndpoint.value = url
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
  copiedResetTimer = window.setTimeout(() => {
    if (copiedEndpoint.value === url) {
      copiedEndpoint.value = null
    }
  }, 1800)
}

function tooltipHint(endpoint: string): string {
  return copiedEndpoint.value === endpoint
    ? t('keys.endpoints.copiedHint')
    : t('keys.endpoints.clickToCopy')
}

function speedTestUrl(endpoint: string): string {
  return `https://www.tcptest.cn/http/${encodeURIComponent(endpoint)}`
}

onBeforeUnmount(() => {
  if (copiedResetTimer !== undefined) {
    window.clearTimeout(copiedResetTimer)
  }
})
</script>

<template>
  <div v-if="allEndpoints.length > 0" class="flex flex-wrap gap-2">
    <div
      v-for="(item, index) in allEndpoints"
      :key="index"
      class="flex items-center gap-2 rounded-xl border border-stone-200/80 bg-white/80 px-3 py-1.5 text-xs shadow-sm shadow-stone-950/5 backdrop-blur-xl transition-all duration-200 hover:border-emerald-500/30 hover:bg-white dark:border-white/10 dark:bg-neutral-950/70 dark:shadow-black/20 dark:hover:border-emerald-500/25 dark:hover:bg-white/[0.06]"
    >
      <span class="font-medium text-stone-700 dark:text-stone-300">{{ item.name }}</span>
      <span
        v-if="item.isDefault"
        class="rounded-md bg-emerald-100/80 px-1.5 py-0.5 text-[10px] font-medium leading-tight text-emerald-700 ring-1 ring-emerald-600/10 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-400/15"
      >{{ t('keys.endpoints.default') }}</span>

      <span class="h-4 w-px bg-stone-200 dark:bg-white/10" aria-hidden="true"></span>

      <div class="group/endpoint relative flex items-center gap-1.5">
        <div
          class="pointer-events-none absolute bottom-full left-1/2 z-20 mb-2 w-max max-w-[24rem] -translate-x-1/2 translate-y-1 rounded-xl border border-stone-200/80 bg-white/95 px-3 py-2.5 text-left opacity-0 shadow-xl shadow-stone-950/10 ring-1 ring-stone-200/60 backdrop-blur-xl transition-all duration-150 group-hover/endpoint:translate-y-0 group-hover/endpoint:opacity-100 group-focus-within/endpoint:translate-y-0 group-focus-within/endpoint:opacity-100 dark:border-white/10 dark:bg-neutral-950/95 dark:shadow-black/30 dark:ring-white/10"
        >
          <p
            v-if="item.description"
            class="max-w-[24rem] break-words text-xs leading-5 text-stone-600 dark:text-stone-200"
          >
            {{ item.description }}
          </p>
          <p
            class="flex items-center gap-1.5 text-[11px] leading-4 text-emerald-600 dark:text-emerald-300"
            :class="item.description ? 'mt-1.5' : ''"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-emerald-500 dark:bg-emerald-300"></span>
            {{ tooltipHint(item.endpoint) }}
          </p>
          <div class="absolute left-1/2 top-full h-3 w-3 -translate-x-1/2 -translate-y-1/2 rotate-45 border-b border-r border-stone-200/80 bg-white dark:border-white/10 dark:bg-neutral-950"></div>
        </div>

        <code
          class="cursor-pointer font-mono text-stone-500 decoration-stone-400 decoration-dashed underline-offset-2 transition-colors hover:text-emerald-600 hover:underline focus:text-emerald-600 focus:underline focus:outline-none dark:text-stone-400 dark:decoration-stone-500 dark:hover:text-emerald-300 dark:focus:text-emerald-300"
          role="button"
          tabindex="0"
          @click="copy(item.endpoint)"
          @keydown.enter.prevent="copy(item.endpoint)"
          @keydown.space.prevent="copy(item.endpoint)"
        >{{ item.endpoint }}</code>

        <button
          type="button"
          class="rounded-md p-0.5 transition-colors hover:bg-stone-100/80 dark:hover:bg-white/[0.06]"
          :class="copiedEndpoint === item.endpoint
            ? 'text-emerald-500 dark:text-emerald-400'
            : 'text-stone-400 hover:text-emerald-600 dark:text-stone-500 dark:hover:text-emerald-300'"
          :aria-label="tooltipHint(item.endpoint)"
          @click="copy(item.endpoint)"
        >
          <svg v-if="copiedEndpoint === item.endpoint" class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
          </svg>
          <svg v-else class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>

        <a
          :href="speedTestUrl(item.endpoint)"
          target="_blank"
          rel="noopener noreferrer"
          class="rounded-md p-0.5 text-stone-400 transition-colors hover:bg-stone-100/80 hover:text-emerald-600 dark:text-stone-500 dark:hover:bg-white/[0.06] dark:hover:text-emerald-300"
          :title="t('keys.endpoints.speedTest')"
        >
          <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
        </a>
      </div>
    </div>
  </div>
</template>
