<template>
  <div class="relative min-h-screen overflow-hidden bg-stone-50 text-stone-950 dark:bg-[#050505] dark:text-white">
    <div class="pointer-events-none absolute inset-0" aria-hidden="true">
      <div
        class="absolute inset-0 opacity-[0.22] dark:opacity-[0.16]"
      >
        <div
          class="h-full w-full bg-[linear-gradient(rgba(34,197,94,0.16)_1px,transparent_1px),linear-gradient(90deg,rgba(34,197,94,0.12)_1px,transparent_1px)] bg-[size:56px_56px]"
        ></div>
      </div>
      <div class="absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-emerald-500/40 to-transparent"></div>
      <div class="absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(16,185,129,0.10),transparent_34rem)] dark:bg-[radial-gradient(circle_at_50%_0%,rgba(16,185,129,0.12),transparent_34rem)]"></div>
    </div>

    <div class="absolute right-4 top-4 z-20 flex items-center gap-2 sm:right-6 sm:top-6">
      <LocaleSwitcher />
      <button
        type="button"
        class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-stone-200 bg-white/70 text-stone-500 backdrop-blur transition hover:border-emerald-500/40 hover:text-emerald-500 dark:border-[#1e1e1e] dark:bg-black/30 dark:text-stone-400"
        :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
        @click="toggleTheme"
      >
        <Icon v-if="isDark" name="sun" size="sm" />
        <Icon v-else name="moon" size="sm" />
      </button>
    </div>

    <main
      class="relative z-10 mx-auto grid min-h-screen w-full max-w-6xl items-center gap-10 px-4 py-20 sm:px-6 lg:grid-cols-[minmax(0,1fr)_minmax(360px,440px)] lg:px-8"
    >
      <section class="hidden max-w-2xl lg:block">
        <template v-if="settingsLoaded">
          <router-link to="/home" class="inline-flex items-center gap-3">
            <span
              class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-lg border border-emerald-500/30 bg-emerald-500/10"
            >
              <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
            </span>
            <span class="truncate text-lg font-bold tracking-tight text-emerald-500">
              {{ siteName }}
            </span>
          </router-link>
          <h1 class="mt-10 text-5xl font-black leading-tight tracking-tight text-stone-950 dark:text-white">
            {{ siteName }}
          </h1>
          <p class="mt-5 max-w-xl text-lg leading-8 text-stone-600 dark:text-stone-400">
            {{ siteSubtitle }}
          </p>
        </template>
      </section>

      <section class="w-full max-w-md justify-self-center lg:justify-self-end">
        <div class="mb-8 text-center lg:hidden">
          <template v-if="settingsLoaded">
            <router-link to="/home" class="inline-flex flex-col items-center">
              <span
                class="mb-4 inline-flex h-14 w-14 items-center justify-center overflow-hidden rounded-lg border border-emerald-500/30 bg-emerald-500/10 shadow-sm shadow-emerald-500/20"
              >
                <img :src="siteLogo || '/logo.png'" alt="Logo" class="h-full w-full object-contain" />
              </span>
              <span class="text-2xl font-black tracking-tight text-emerald-500">
                {{ siteName }}
              </span>
            </router-link>
            <p class="mt-2 text-sm leading-6 text-stone-500 dark:text-stone-400">
              {{ siteSubtitle }}
            </p>
          </template>
        </div>

        <div
          class="rounded-xl border border-stone-200/80 bg-white/80 p-6 shadow-xl shadow-stone-950/5 backdrop-blur-xl dark:border-[#1e1e1e] dark:bg-[#101010]/80 dark:shadow-black/30 sm:p-8"
        >
          <slot />
        </div>

        <div class="mt-6 text-center text-sm text-stone-500 dark:text-stone-400">
          <slot name="footer" />
        </div>

        <div class="mt-8 text-center text-xs text-stone-400 dark:text-stone-600">
          {{ t('auth.copyright', { year: currentYear, siteName }) }}
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()
const { t } = useI18n()
const isDark = ref(document.documentElement.classList.contains('dark'))

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
    return
  }
  isDark.value = false
  document.documentElement.classList.remove('dark')
}

onMounted(() => {
  initTheme()
  appStore.fetchPublicSettings()
})
</script>
