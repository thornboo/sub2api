<template>
  <BaseDialog
    :show="show"
    :title="t('feedback.title')"
    width="normal"
    prevent-horizontal-scroll
    @close="handleClose"
  >
    <form id="feedback-form" class="space-y-4" @submit.prevent="handleSubmit">
      <div>
        <label for="feedback-content" class="input-label">{{ t('feedback.contentLabel') }}</label>
        <textarea
          id="feedback-content"
          v-model="draft"
          rows="7"
          class="input min-h-40 resize-y"
          :class="displayContentError && 'border-rose-400 focus:border-rose-500 focus:ring-rose-500/10'"
          :placeholder="t('feedback.placeholder')"
          :disabled="submitting"
          maxlength="4000"
          @paste="handlePaste"
          @drop.prevent="attachmentError = t('feedback.textOnly')"
          @dragover.prevent
        ></textarea>
        <div class="mt-2 flex items-center justify-between gap-3 text-xs">
          <p class="min-w-0 text-stone-500 dark:text-stone-400">
            <span v-if="displayContentError" class="text-rose-600 dark:text-rose-300">{{ contentError }}</span>
            <span v-else-if="attachmentError" class="text-amber-600 dark:text-amber-300">{{ attachmentError }}</span>
            <span v-else>{{ t('feedback.textOnly') }}</span>
          </p>
          <span
            class="shrink-0 tabular-nums"
            :class="contentLength > maxContentLength ? 'text-rose-600 dark:text-rose-300' : 'text-stone-400'"
          >
            {{ contentLength }} / {{ maxContentLength }}
          </span>
        </div>
      </div>

      <div
        v-if="message"
        class="rounded-lg border px-3 py-2 text-sm"
        :class="messageType === 'success'
          ? 'border-emerald-500/25 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
          : 'border-rose-500/25 bg-rose-500/10 text-rose-700 dark:text-rose-300'"
      >
        {{ message }}
      </div>

      <p v-if="cooldownRemaining > 0" class="rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-sm text-stone-600 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-300">
        {{ t('feedback.cooldown', { seconds: cooldownRemaining }) }}
      </p>
    </form>

    <template #footer>
      <div class="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" form="feedback-form" class="btn btn-primary" :disabled="submitDisabled">
          <Icon v-if="submitting" name="refresh" size="sm" class="mr-1 animate-spin" />
          <Icon v-else name="chat" size="sm" class="mr-1" />
          {{ submitting ? t('common.submitting') : submitLabel }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

export interface FeedbackSubmitResult {
  id: number
  created_at: string
  retry_after: number
}

type MessageType = 'success' | 'error'

const props = withDefaults(defineProps<{
  show: boolean
  identityKey: string
  submitter: (content: string, signal?: AbortSignal) => Promise<FeedbackSubmitResult>
}>(), {
  identityKey: '',
})

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'submitted', result: FeedbackSubmitResult): void
  (event: 'unauthorized'): void
}>()

const { t } = useI18n()
const appStore = useAppStore()

const maxContentLength = 2000
const draft = ref('')
const submitting = ref(false)
const now = ref(Date.now())
const cooldownUntil = ref(0)
const message = ref('')
const messageType = ref<MessageType>('success')
const attachmentError = ref('')
const attemptedSubmit = ref(false)
let tickId: number | null = null
let activeController: AbortController | null = null

const trimmedContent = computed(() => draft.value.trim())
const contentLength = computed(() => Array.from(trimmedContent.value).length)
const cooldownRemaining = computed(() => Math.max(0, Math.ceil((cooldownUntil.value - now.value) / 1000)))
const contentError = computed(() => {
  if (!trimmedContent.value) return t('feedback.emptyError')
  if (contentLength.value > maxContentLength) return t('feedback.tooLongError', { max: maxContentLength })
  return ''
})
const submitLabel = computed(() => (
  cooldownRemaining.value > 0
    ? t('feedback.waitSubmit', { seconds: cooldownRemaining.value })
    : t('feedback.submit')
))
const submitDisabled = computed(() => submitting.value || cooldownRemaining.value > 0 || Boolean(contentError.value))
const displayContentError = computed(() => attemptedSubmit.value && Boolean(contentError.value))

function ensureTicking() {
  if (tickId !== null) return
  tickId = window.setInterval(() => {
    now.value = Date.now()
    if (!props.show && cooldownRemaining.value <= 0) stopTicking()
  }, 1000)
}

function stopTicking() {
  if (tickId === null) return
  window.clearInterval(tickId)
  tickId = null
}

function startCooldown(seconds: number) {
  const normalized = Number.isFinite(seconds) ? Math.max(1, Math.ceil(seconds)) : 60
  cooldownUntil.value = Date.now() + normalized * 1000
  now.value = Date.now()
  ensureTicking()
}

function resetState() {
  activeController?.abort()
  activeController = null
  draft.value = ''
  submitting.value = false
  cooldownUntil.value = 0
  message.value = ''
  messageType.value = 'success'
  attachmentError.value = ''
  attemptedSubmit.value = false
  stopTicking()
}

function handleClose() {
  emit('close')
}

function extractStatus(error: unknown) {
  if (!error || typeof error !== 'object') return undefined
  const value = error as { status?: number; response?: { status?: number } }
  return value.status ?? value.response?.status
}

function extractRetryAfter(error: unknown) {
  if (!error || typeof error !== 'object') return undefined
  const value = error as {
    metadata?: { retry_after?: string | number }
    response?: {
      data?: { metadata?: { retry_after?: string | number } }
      headers?: Record<string, string | number | undefined>
    }
  }
  const retryAfter = value.metadata?.retry_after
    ?? value.response?.data?.metadata?.retry_after
    ?? value.response?.headers?.['retry-after']
  const seconds = Number(retryAfter)
  return Number.isFinite(seconds) && seconds > 0 ? seconds : undefined
}

function handlePaste(event: ClipboardEvent) {
  const items = Array.from(event.clipboardData?.items || [])
  const hasFileOrImage = items.some((item) => item.kind === 'file' || item.type.startsWith('image/'))
  if (!hasFileOrImage && (event.clipboardData?.files?.length || 0) === 0) return
  event.preventDefault()
  attachmentError.value = t('feedback.textOnly')
}

async function handleSubmit() {
  attemptedSubmit.value = true
  if (submitDisabled.value) return
  const content = trimmedContent.value
  const identityAtSubmit = props.identityKey
  const controller = new AbortController()
  activeController = controller
  submitting.value = true
  message.value = ''
  attachmentError.value = ''

  try {
    const result = await props.submitter(content, controller.signal)
    if (controller.signal.aborted || identityAtSubmit !== props.identityKey) return
    draft.value = ''
    startCooldown(result.retry_after || 60)
    appStore.showSuccess(t('feedback.success'))
    emit('submitted', result)
    emit('close')
  } catch (error) {
    if (controller.signal.aborted || identityAtSubmit !== props.identityKey) return
    const status = extractStatus(error)
    if (status === 401) {
      emit('unauthorized')
      return
    }
    if (status === 429) {
      const retryAfter = extractRetryAfter(error) || 60
      startCooldown(retryAfter)
      messageType.value = 'error'
      message.value = t('feedback.rateLimited', { seconds: cooldownRemaining.value || Math.ceil(retryAfter) })
      return
    }
    messageType.value = 'error'
    message.value = t('feedback.failed')
  } finally {
    if (activeController === controller) {
      activeController = null
      submitting.value = false
    }
  }
}

watch(
  () => props.show,
  (visible) => {
    if (visible) {
      now.value = Date.now()
      if (cooldownRemaining.value > 0) ensureTicking()
    }
  },
)

watch(
  () => props.identityKey,
  () => resetState(),
)

onBeforeUnmount(() => {
  resetState()
})
</script>
