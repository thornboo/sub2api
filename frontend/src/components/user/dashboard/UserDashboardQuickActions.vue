<template>
  <div class="card overflow-hidden">
    <div class="border-b border-stone-200/70 px-6 py-4 dark:border-white/10">
      <h2 class="text-lg font-semibold text-stone-950 dark:text-white">{{ t('dashboard.quickActions') }}</h2>
    </div>
    <div class="space-y-3 p-5">
      <button
        @click="router.push('/keys')"
        class="group flex w-full items-center gap-4 rounded-xl border border-stone-200/70 bg-gradient-to-r from-white to-primary-50/60 p-4 text-left shadow-sm shadow-stone-950/5 transition-all duration-200 hover:-translate-y-0.5 hover:border-primary-300/70 hover:shadow-md hover:shadow-primary-950/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-300/70 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:border-white/10 dark:from-white/[0.055] dark:to-cyan-400/[0.045] dark:shadow-black/20 dark:hover:border-cyan-300/25 dark:hover:from-white/[0.075] dark:hover:to-cyan-400/[0.075] dark:focus-visible:ring-cyan-300/70 dark:focus-visible:ring-offset-black"
      >
        <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-primary-50 text-primary-600 ring-1 ring-primary-200/70 transition-all duration-200 group-hover:scale-105 group-hover:bg-primary-100 dark:bg-cyan-400/10 dark:text-cyan-300 dark:ring-cyan-300/20 dark:group-hover:bg-cyan-400/15">
          <Icon name="key" size="lg" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-stone-900 dark:text-stone-100">{{ t('dashboard.createApiKey') }}</p>
          <p class="text-xs text-stone-500 dark:text-stone-500">{{ t('dashboard.generateNewKey') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-stone-400 transition-colors group-hover:text-primary-500 dark:text-stone-600 dark:group-hover:text-cyan-300"
        />
      </button>

      <button
        @click="router.push('/usage')"
        class="group flex w-full items-center gap-4 rounded-xl border border-stone-200/70 bg-gradient-to-r from-white to-emerald-50/60 p-4 text-left shadow-sm shadow-stone-950/5 transition-all duration-200 hover:-translate-y-0.5 hover:border-emerald-300/70 hover:shadow-md hover:shadow-emerald-950/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-300/70 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:border-white/10 dark:from-white/[0.055] dark:to-emerald-400/[0.045] dark:shadow-black/20 dark:hover:border-emerald-300/25 dark:hover:from-white/[0.075] dark:hover:to-emerald-400/[0.075] dark:focus-visible:ring-emerald-300/70 dark:focus-visible:ring-offset-black"
      >
        <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600 ring-1 ring-emerald-200/70 transition-all duration-200 group-hover:scale-105 group-hover:bg-emerald-100 dark:bg-emerald-400/10 dark:text-emerald-300 dark:ring-emerald-300/20 dark:group-hover:bg-emerald-400/15">
          <Icon name="chart" size="lg" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-stone-900 dark:text-stone-100">{{ t('dashboard.viewUsage') }}</p>
          <p class="text-xs text-stone-500 dark:text-stone-500">{{ t('dashboard.checkDetailedLogs') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-stone-400 transition-colors group-hover:text-emerald-500 dark:text-stone-600 dark:group-hover:text-emerald-300"
        />
      </button>

      <button
        v-if="canUseBatchImage"
        @click="router.push('/batch-image')"
        class="group flex w-full items-center gap-4 rounded-xl border border-stone-200/70 bg-gradient-to-r from-white to-sky-50/70 p-4 text-left shadow-sm shadow-stone-950/5 transition-all duration-200 hover:-translate-y-0.5 hover:border-sky-300/70 hover:shadow-md hover:shadow-sky-950/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sky-300/70 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:border-white/10 dark:from-white/[0.055] dark:to-sky-400/[0.05] dark:shadow-black/20 dark:hover:border-sky-300/25 dark:hover:from-white/[0.075] dark:hover:to-sky-400/[0.08] dark:focus-visible:ring-sky-300/70 dark:focus-visible:ring-offset-black"
      >
        <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-sky-50 text-sky-600 ring-1 ring-sky-200/80 transition-all duration-200 group-hover:scale-105 group-hover:bg-sky-100 dark:bg-sky-400/10 dark:text-sky-300 dark:ring-sky-300/20 dark:group-hover:bg-sky-400/15">
          <Icon name="sparkles" size="lg" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-stone-900 dark:text-stone-100">{{ t('dashboard.batchImageAgent') }}</p>
          <p class="text-xs text-stone-500 dark:text-stone-500">{{ t('dashboard.batchImageAgentDesc') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-stone-400 transition-colors group-hover:text-sky-500 dark:text-stone-600 dark:group-hover:text-sky-300"
        />
      </button>

      <button
        @click="router.push('/redeem')"
        class="group flex w-full items-center gap-4 rounded-xl border border-stone-200/70 bg-gradient-to-r from-white to-amber-50/70 p-4 text-left shadow-sm shadow-stone-950/5 transition-all duration-200 hover:-translate-y-0.5 hover:border-amber-300/70 hover:shadow-md hover:shadow-amber-950/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-300/70 focus-visible:ring-offset-2 focus-visible:ring-offset-white dark:border-white/10 dark:from-white/[0.055] dark:to-amber-400/[0.05] dark:shadow-black/20 dark:hover:border-amber-300/25 dark:hover:from-white/[0.075] dark:hover:to-amber-400/[0.08] dark:focus-visible:ring-amber-300/70 dark:focus-visible:ring-offset-black"
      >
        <div class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-amber-50 text-amber-600 ring-1 ring-amber-200/80 transition-all duration-200 group-hover:scale-105 group-hover:bg-amber-100 dark:bg-amber-400/10 dark:text-amber-300 dark:ring-amber-300/20 dark:group-hover:bg-amber-400/15">
          <Icon name="gift" size="lg" />
        </div>
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-stone-900 dark:text-stone-100">{{ t('dashboard.redeemCode') }}</p>
          <p class="text-xs text-stone-500 dark:text-stone-500">{{ t('dashboard.addBalanceWithCode') }}</p>
        </div>
        <Icon
          name="chevronRight"
          size="md"
          class="text-stone-400 transition-colors group-hover:text-amber-500 dark:text-stone-600 dark:group-hover:text-amber-300"
        />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
  void refreshBatchImageAccess()
})
</script>
