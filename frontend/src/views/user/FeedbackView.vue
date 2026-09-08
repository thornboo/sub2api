<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-xl font-semibold text-stone-950 dark:text-white">{{ t('feedback.myTickets') }}</h1>
          <p class="mt-1 text-sm text-stone-500 dark:text-stone-400">{{ t('feedback.myTicketsDescription') }}</p>
        </div>
        <button type="button" class="btn btn-primary" @click="createOpen = true">
          <Icon name="chat" size="sm" class="mr-1" />
          {{ t('feedback.newTicket') }}
        </button>
      </div>

      <FeedbackThreadPanel
        ref="threadPanelRef"
        :identity-key="identityKey"
        :title="t('feedback.myTickets')"
        :api="threadAPI"
        @create="createOpen = true"
      />

      <FeedbackDialog
        :show="createOpen"
        :identity-key="identityKey"
        :submitter="submitFeedback"
        @close="createOpen = false"
        @submitted="handleSubmitted"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { feedbackAPI } from '@/api'
import type { FeedbackSubmitResult } from '@/api/feedback'
import AppLayout from '@/components/layout/AppLayout.vue'
import FeedbackDialog from '@/components/common/FeedbackDialog.vue'
import FeedbackThreadPanel from '@/components/common/FeedbackThreadPanel.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()
const createOpen = ref(false)
const threadPanelRef = ref<InstanceType<typeof FeedbackThreadPanel> | null>(null)
const identityKey = computed(() => authStore.user ? `user:${authStore.user.id}:${authStore.user.email || ''}` : '')
const threadAPI = feedbackAPI

function submitFeedback(content: string, signal?: AbortSignal) {
  return feedbackAPI.submit({ content }, signal)
}

function handleSubmitted(result: FeedbackSubmitResult) {
  threadPanelRef.value?.applyCooldown(result.retry_after)
  void threadPanelRef.value?.refreshAfterCreate(result.id)
}
</script>
