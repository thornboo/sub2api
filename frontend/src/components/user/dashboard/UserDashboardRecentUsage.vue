<template>
  <div class="card overflow-hidden">
    <div class="flex items-center justify-between border-b border-stone-200/70 px-6 py-4 dark:border-white/10">
      <h2 class="text-lg font-semibold text-stone-950 dark:text-white">{{ t('dashboard.recentUsage') }}</h2>
      <span class="rounded-full border border-stone-200/80 bg-stone-100/80 px-3 py-1 text-xs font-medium text-stone-600 dark:border-emerald-400/15 dark:bg-emerald-400/10 dark:text-emerald-200">
        {{ t('dashboard.last7Days') }}
      </span>
    </div>
    <div class="p-6">
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner size="lg" />
      </div>
      <div v-else-if="data.length === 0" class="py-8">
        <EmptyState :title="t('dashboard.noUsageRecords')" :description="t('dashboard.startUsingApi')" />
      </div>
      <div v-else class="space-y-3">
        <div
          v-for="log in data"
          :key="log.id"
          class="group relative flex items-center justify-between gap-4 overflow-hidden rounded-xl border border-stone-200/70 bg-gradient-to-r from-white to-stone-50/90 p-4 shadow-sm shadow-stone-950/5 transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/60 hover:shadow-md hover:shadow-primary-950/5 dark:border-white/10 dark:from-white/[0.055] dark:via-white/[0.035] dark:to-emerald-400/[0.035] dark:shadow-black/20 dark:hover:border-emerald-300/25 dark:hover:from-white/[0.075] dark:hover:to-emerald-400/[0.06]"
        >
          <div class="flex min-w-0 items-center gap-4">
            <div class="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-primary-200/70 transition-colors group-hover:bg-primary-100 dark:bg-emerald-400/10 dark:text-emerald-300 dark:ring-emerald-300/20 dark:group-hover:bg-emerald-400/15">
              <Icon name="beaker" size="md" />
            </div>
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-stone-900 dark:text-stone-100">{{ log.model }}</p>
              <p class="text-xs text-stone-500 dark:text-stone-500">{{ formatDateTime(log.created_at) }}</p>
            </div>
          </div>
          <div class="flex-shrink-0 text-right">
            <p class="font-mono text-sm font-semibold">
              <span class="text-emerald-600 dark:text-emerald-300" :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</span>
              <span class="font-normal text-stone-400 dark:text-stone-600" :title="t('dashboard.standard')"> / ${{ formatCost(log.total_cost) }}</span>
            </p>
            <p class="font-mono text-xs text-stone-500 dark:text-stone-500">{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</p>
          </div>
        </div>

        <router-link to="/usage" class="flex items-center justify-center gap-2 rounded-xl py-3 text-sm font-medium text-primary-600 transition-colors hover:bg-primary-50/70 hover:text-primary-700 dark:text-emerald-300 dark:hover:bg-emerald-400/10 dark:hover:text-emerald-200">
          {{ t('dashboard.viewAllUsage') }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'

defineProps<{
  data: UsageLog[]
  loading: boolean
}>()
const { t } = useI18n()
const formatCost = (c: number) => c.toFixed(4)
</script>
