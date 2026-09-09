<template>
  <AppLayout>
    <!-- Desktop height accounts for the 4rem header and AppLayout's 4rem vertical padding. -->
    <div class="space-y-4 lg:h-[calc(100dvh-8rem)] lg:min-h-[560px] lg:space-y-0">
      <h1 class="text-xl font-semibold text-stone-950 dark:text-white lg:hidden">{{ t('admin.feedback.title') }}</h1>

      <FeedbackThreadPanel
        :identity-key="identityKey"
        :title="t('admin.feedback.queueTitle')"
        :api="threadAPI"
        admin
        fill-height
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import { adminAPI } from '@/api/admin'
import AppLayout from '@/components/layout/AppLayout.vue'
import FeedbackThreadPanel from '@/components/common/FeedbackThreadPanel.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()
const threadAPI = adminAPI.feedback
const identityKey = computed(() => authStore.user ? `admin:${authStore.user.id}:${authStore.user.email || ''}` : '')
</script>
