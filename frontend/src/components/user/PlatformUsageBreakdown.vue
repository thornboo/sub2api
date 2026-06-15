<template>
  <div class="group/usage relative text-sm">
    <div class="flex items-center gap-1.5">
      <span class="text-gray-500 dark:text-gray-400">{{ t('admin.users.today') }}:</span>
      <span class="font-medium text-gray-900 dark:text-white">${{ today.toFixed(4) }}</span>
      <Icon
        v-if="hasBreakdown"
        name="infoCircle"
        size="xs"
        class="text-gray-400 dark:text-gray-500"
      />
    </div>
    <div class="mt-0.5 flex items-center gap-1.5">
      <span class="text-gray-500 dark:text-gray-400">{{ t('admin.users.total') }}:</span>
      <span class="font-medium text-gray-900 dark:text-white">${{ total.toFixed(4) }}</span>
    </div>

    <div
      v-if="hasBreakdown"
      class="pointer-events-none absolute left-full top-0 z-50 ml-2 min-w-[244px] translate-y-1 whitespace-nowrap rounded-xl border border-stone-200/80 bg-white/95 p-2.5 text-xs text-stone-700 opacity-0 shadow-xl shadow-stone-950/10 backdrop-blur-xl transition-all duration-150 group-hover/usage:translate-y-0 group-hover/usage:opacity-100 dark:border-white/10 dark:bg-neutral-950/95 dark:text-stone-200 dark:shadow-black/40"
    >
      <div class="mb-2 flex items-center justify-between gap-4 border-b border-stone-200/70 pb-1.5 text-[11px] font-medium text-stone-500 dark:border-white/10 dark:text-stone-400">
        <span class="text-stone-600 dark:text-stone-300">{{ t('admin.users.platformBreakdown') }}</span>
        <span class="font-mono">{{ t('admin.users.today') }} / {{ t('admin.users.total') }}</span>
      </div>
      <div
        v-for="item in sortedBreakdown"
        :key="item.platform"
        class="flex items-center justify-between gap-4 rounded-lg px-1.5 py-1"
        :class="item.isOther
          ? 'italic text-stone-400 dark:text-stone-500'
          : 'text-stone-700 dark:text-stone-200'"
      >
        <span class="font-medium capitalize">
          {{ item.isOther ? t('admin.users.platformOther') : platformLabel(item.platform) }}
        </span>
        <span class="font-mono text-stone-900 dark:text-stone-100">
          ${{ item.today_actual_cost.toFixed(4) }}
          <span class="px-1.5 text-stone-400 dark:text-stone-500">/</span>
          ${{ item.total_actual_cost.toFixed(4) }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PlatformUsage } from '@/api/admin/dashboard'

const props = defineProps<{
  today: number
  total: number
  byPlatform?: PlatformUsage[]
}>()

const { t } = useI18n()

// 与 UserDashboardStats 保持一致：把"总值 - 各平台之和"的差作为"其他"行展示，
// 避免 tooltip 内各平台费用加总与列首总值对不上。
const OTHER_THRESHOLD = 0.0001

interface BreakdownRow {
  platform: string
  today_actual_cost: number
  total_actual_cost: number
  isOther?: boolean
}

const sortedBreakdown = computed<BreakdownRow[]>(() => {
  const list = props.byPlatform ?? []
  const rows: BreakdownRow[] = [...list]
    .sort((a, b) => b.total_actual_cost - a.total_actual_cost)
    .map((p) => ({ ...p }))

  const sumTotal = rows.reduce((s, r) => s + r.total_actual_cost, 0)
  const sumToday = rows.reduce((s, r) => s + r.today_actual_cost, 0)
  const diffTotal = Math.max(0, props.total - sumTotal)
  const diffToday = Math.max(0, props.today - sumToday)
  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    rows.push({
      platform: '__other__',
      today_actual_cost: diffToday,
      total_actual_cost: diffTotal,
      isOther: true
    })
  }
  return rows
})

const hasBreakdown = computed(() => sortedBreakdown.value.length > 0)

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity'
}

function platformLabel(platform: string): string {
  return PLATFORM_LABELS[platform] ?? platform
}
</script>
