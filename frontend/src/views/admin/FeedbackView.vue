<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-stone-950 dark:text-white">{{ t('admin.feedback.title') }}</h1>
          <p class="mt-1 text-sm text-stone-500 dark:text-stone-400">{{ t('admin.feedback.description') }}</p>
        </div>
      </div>

      <FeedbackThreadPanel
        :identity-key="identityKey"
        :title="t('admin.feedback.queueTitle')"
        :api="threadAPI"
        admin
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
