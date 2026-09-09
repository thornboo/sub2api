<template>
  <AppLayout>
    <!-- Desktop height accounts for the 4rem header and AppLayout's 4rem vertical padding. -->
    <div class="space-y-4 lg:h-[calc(100dvh-8rem)] lg:min-h-[560px] lg:space-y-0">
      <h1 class="text-xl font-semibold text-stone-950 dark:text-white lg:hidden">{{ t('feedback.myTickets') }}</h1>

      <FeedbackThreadPanel
        ref="threadPanelRef"
        :identity-key="identityKey"
        :title="t('feedback.myTickets')"
        :api="threadAPI"
        show-create
        fill-height
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
import type { FeedbackSubmitRequest, FeedbackSubmitResult } from '@/api/feedback'
import AppLayout from '@/components/layout/AppLayout.vue'
import FeedbackDialog from '@/components/common/FeedbackDialog.vue'
import FeedbackThreadPanel from '@/components/common/FeedbackThreadPanel.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()
const createOpen = ref(false)
const threadPanelRef = ref<InstanceType<typeof FeedbackThreadPanel> | null>(null)
const identityKey = computed(() => authStore.user ? `user:${authStore.user.id}:${authStore.user.email || ''}` : '')
const threadAPI = feedbackAPI

function submitFeedback(request: FeedbackSubmitRequest, signal?: AbortSignal) {
  return feedbackAPI.submit(request, signal)
}

function handleSubmitted(result: FeedbackSubmitResult) {
  threadPanelRef.value?.applyCooldown(result.retry_after)
  void threadPanelRef.value?.refreshAfterCreate(result.id)
}
</script>
