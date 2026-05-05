<template>
  <div
    class="min-h-screen bg-[linear-gradient(180deg,#fafaf9_0%,#f5f5f4_48%,#fafaf9_100%)] text-stone-950 dark:bg-[linear-gradient(180deg,#050505_0%,#070707_52%,#050505_100%)] dark:text-white"
  >
    <!-- Neutral depth layer; no grid or green glow so console pages stay quiet. -->
    <div
      class="pointer-events-none fixed inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(15,23,42,0.045),transparent_34rem)] dark:bg-[radial-gradient(circle_at_50%_0%,rgba(255,255,255,0.045),transparent_34rem)]"
      aria-hidden="true"
    ></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="p-4 md:p-6 lg:p-8">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>
