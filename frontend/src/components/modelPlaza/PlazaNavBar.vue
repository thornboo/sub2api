<template>
  <header
    class="sticky top-0 z-30 border-b border-stone-200/80 bg-white/90 backdrop-blur-xl dark:border-[#1e1e1e] dark:bg-[#050505]/90"
  >
    <div class="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-3.5 sm:px-6">
      <!-- 左:站点 logo + 名称 -->
      <div class="flex min-w-0 items-center gap-3">
        <template v-if="settings">
          <span
            class="flex h-9 w-9 flex-shrink-0 items-center justify-center overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-stone-200 dark:bg-black dark:ring-white/10"
          >
            <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
          </span>
          <span class="truncate text-base font-semibold text-stone-950 dark:text-white">
            {{ siteName }}
          </span>
        </template>
        <template v-else>
          <span class="h-9 w-9 flex-shrink-0 animate-pulse rounded-lg bg-stone-200 dark:bg-white/10" aria-hidden="true"></span>
          <span class="h-5 w-28 animate-pulse rounded bg-stone-200 dark:bg-white/10" aria-hidden="true"></span>
        </template>
      </div>

      <!-- 右:根据登录态返回控制台或首页 -->
      <RouterLink
        v-if="isAuthenticated"
        :to="backTarget"
        class="inline-flex flex-shrink-0 items-center justify-center gap-1.5 rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-stone-950 shadow-sm shadow-emerald-500/20 transition hover:bg-emerald-400 active:scale-[0.98]"
      >
        {{ t('modelPlaza.nav.backToConsole') }}
      </RouterLink>
      <RouterLink
        v-else
        to="/"
        class="inline-flex flex-shrink-0 items-center justify-center rounded-lg bg-emerald-500 px-4 py-2 text-sm font-semibold text-stone-950 shadow-sm shadow-emerald-500/20 transition hover:bg-emerald-400 active:scale-[0.98]"
      >
        {{ t('modelPlaza.nav.backToHome') }}
      </RouterLink>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { sanitizeUrl } from '@/utils/url'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => settings.value?.site_name || 'Sub2API')
const siteLogo = computed(() =>
  sanitizeUrl(settings.value?.site_logo || '', { allowRelative: true, allowDataUrl: true })
)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const backTarget = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
</script>
