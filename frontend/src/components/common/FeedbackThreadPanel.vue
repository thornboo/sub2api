<template>
  <div
    class="grid min-h-[560px] gap-4 lg:grid-cols-[minmax(280px,0.8fr)_minmax(0,2fr)]"
    :class="fillHeight ? 'lg:h-full' : 'lg:h-[min(760px,calc(100dvh-200px))]'"
  >
    <section class="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-stone-200 bg-white dark:border-white/10 dark:bg-[#0d0d0d]">
      <div class="shrink-0 border-b border-stone-200 p-3 dark:border-white/10">
        <div class="flex items-center justify-between gap-2">
          <h2 class="text-sm font-semibold text-stone-900 dark:text-white">{{ title }}</h2>
          <div class="flex items-center gap-1">
            <button
              type="button"
              class="rounded-lg p-1.5 text-stone-500 hover:bg-stone-100 hover:text-stone-900 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-white/10 dark:hover:text-white"
              :disabled="ticketsLoading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadTickets"
            >
              <Icon name="refresh" size="sm" :class="ticketsLoading && 'animate-spin'" />
            </button>
            <button v-if="showCreate" type="button" class="btn btn-primary h-8 px-3 text-xs" @click="$emit('create')">
              <Icon name="chat" size="sm" class="mr-1" />
              {{ t('feedback.newTicket') }}
            </button>
          </div>
        </div>
        <div class="mt-3 grid grid-cols-4 gap-1 rounded-lg bg-stone-100 p-1 text-xs dark:bg-white/[0.06]" :aria-label="t('feedback.filterLabel')">
          <button
            v-for="option in statusOptions"
            :key="option.value || 'all'"
            type="button"
            :data-testid="`feedback-filter-${option.value || 'all'}`"
            :aria-pressed="statusFilter === option.value"
            class="rounded-md border px-2 py-1.5 font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500 focus-visible:ring-offset-2 focus-visible:ring-offset-stone-100 dark:focus-visible:ring-offset-[#1b1b1b]"
            :class="statusFilter === option.value ? 'border-emerald-500/50 bg-emerald-50 text-emerald-800 shadow-sm dark:border-emerald-400/50 dark:bg-emerald-500/20 dark:text-emerald-200' : 'border-transparent text-stone-500 hover:bg-white/70 hover:text-stone-900 dark:text-stone-400 dark:hover:bg-white/[0.05] dark:hover:text-white'"
            @click="setStatusFilter(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="min-h-0 max-h-[430px] flex-1 overflow-y-auto lg:max-h-none">
        <button
          v-for="ticket in tickets"
          :key="ticket.id"
          type="button"
          data-testid="feedback-ticket-row"
          class="block w-full border-b border-stone-100 px-3 py-3 text-left transition last:border-b-0 hover:bg-stone-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-emerald-500 dark:border-white/[0.06] dark:hover:bg-white/[0.04]"
          :class="selectedTicket?.id === ticket.id && 'bg-emerald-50/70 dark:bg-emerald-500/[0.08]'"
          @click="selectTicket(ticket)"
        >
          <div class="flex min-w-0 items-center gap-2">
            <span class="inline-flex shrink-0 items-center gap-1.5 font-mono text-[11px] text-stone-500">
              #{{ ticket.id }}
              <span
                v-if="ticket.unread_count > 0"
                data-testid="feedback-unread-dot"
                class="inline-block h-2 w-2 rounded-full bg-rose-500"
                :title="t('feedback.unreadCount', { count: ticket.unread_count })"
                :aria-label="t('feedback.unreadCount', { count: ticket.unread_count })"
              ></span>
              <span v-if="ticket.unread_count > 0" class="sr-only">{{ t('feedback.unreadCount', { count: ticket.unread_count }) }}</span>
            </span>
            <span class="min-w-0 flex-1 truncate text-sm font-semibold leading-5 text-stone-900 dark:text-stone-100" :title="ticketTitle(ticket)">{{ ticketTitle(ticket) }}</span>
            <span :class="['badge shrink-0', statusBadge(ticket)]">{{ statusLabel(ticket) }}</span>
          </div>
          <div class="mt-2 flex min-w-0 items-center justify-between gap-2 text-xs text-stone-500 dark:text-stone-400">
            <span v-if="admin" class="flex min-w-0 items-center gap-1.5">
              <span class="truncate" :title="ticketUserName(ticket)">{{ ticketUserName(ticket) }}</span>
              <span v-if="ticket.user_id" class="shrink-0 font-mono" :aria-label="t('admin.feedback.userId', { id: ticket.user_id })">#{{ ticket.user_id }}</span>
            </span>
            <span v-else>{{ t('feedback.lastActivity') }}</span>
            <time class="shrink-0 text-[11px] tabular-nums" :datetime="ticket.updated_at" :title="formatDateTimeToMinute(ticket.updated_at)">{{ formatQueueTime(ticket.updated_at) }}</time>
          </div>
        </button>
        <div v-if="ticketsLoading && tickets.length === 0" class="px-3 py-10 text-center text-sm text-stone-400">{{ t('feedback.loadingTickets') }}</div>
        <div v-else-if="tickets.length === 0" class="px-3 py-10 text-center text-sm text-stone-400">{{ t('feedback.emptyTickets') }}</div>
      </div>

      <Pagination
        v-if="ticketPagination.total > ticketPagination.page_size"
        :page="ticketPagination.page"
        :total="ticketPagination.total"
        :page-size="ticketPagination.page_size"
        :show-page-size-selector="false"
        @update:page="changeTicketPage"
      />
    </section>

    <section class="flex min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border border-stone-200 bg-white dark:border-white/10 dark:bg-[#0d0d0d]">
      <div v-if="!selectedTicket" class="flex min-h-[520px] flex-1 items-center justify-center px-4 text-sm text-stone-400 lg:min-h-0">
        {{ t('feedback.selectTicket') }}
      </div>

      <div v-else class="flex min-h-0 flex-1 flex-col">
        <div data-testid="feedback-ticket-header" class="shrink-0 border-b border-stone-200 px-4 py-3 dark:border-white/10 sm:px-5">
          <div class="flex items-start justify-between gap-3">
            <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1.5 pt-1">
              <span class="shrink-0 font-mono text-xs text-stone-500">#{{ selectedTicket.id }}</span>
              <h2 class="min-w-0 max-w-full break-words text-base font-semibold leading-6 text-stone-950 [overflow-wrap:anywhere] dark:text-white">{{ ticketTitle(selectedTicket) }}</h2>
              <span :class="['badge shrink-0', statusBadge(selectedTicket)]">{{ statusLabel(selectedTicket, true) }}</span>
              <span v-if="selectedTicket.source" :class="['badge shrink-0', selectedTicket.source === 'key' ? 'badge-warning' : 'badge-success']">{{ sourceLabel(selectedTicket.source) }}</span>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <button type="button" class="btn btn-secondary h-9 w-9 p-0" :disabled="detailLoading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="() => refreshSelected()">
                <Icon name="refresh" size="sm" :class="detailLoading && 'animate-spin'" />
              </button>
              <button
                v-if="selectedTicket.status === 'open'"
                type="button"
                data-testid="feedback-close-ticket"
                class="btn btn-secondary h-9 px-3 text-sm text-rose-600 hover:border-rose-300 hover:text-rose-700 dark:text-rose-300"
                :disabled="closing || replying"
                @click="pendingCloseTicket = selectedTicket"
              >
                {{ t('feedback.closeTicket') }}
              </button>
            </div>
          </div>
          <div class="mt-2 flex min-w-0 flex-wrap items-baseline gap-x-5 gap-y-1.5 text-xs">
            <dl v-if="admin" data-testid="feedback-ticket-identity" class="flex min-w-0 flex-wrap items-baseline gap-x-4 gap-y-1.5">
              <div class="flex min-w-0 items-baseline gap-2">
                <dt class="shrink-0 text-stone-500 dark:text-stone-400">{{ t(selectedTicket.source === 'key' ? 'admin.feedback.owner' : 'admin.feedback.submitter') }}</dt>
                <dd class="flex min-w-0 items-baseline gap-1.5">
                  <span class="min-w-0 font-medium text-stone-800 [overflow-wrap:anywhere] dark:text-stone-200">{{ ticketUserName(selectedTicket) }}</span>
                  <span v-if="selectedTicket.user_id" class="shrink-0 font-mono text-stone-500" :aria-label="t('admin.feedback.userId', { id: selectedTicket.user_id })">#{{ selectedTicket.user_id }}</span>
                </dd>
              </div>
              <div v-if="selectedTicket.source === 'key'" class="flex min-w-0 items-baseline gap-2">
                <dt class="shrink-0 text-stone-500 dark:text-stone-400">{{ t('admin.feedback.keyLabel') }}</dt>
                <dd class="min-w-0 text-stone-700 [overflow-wrap:anywhere] dark:text-stone-300">{{ ticketKeyName(selectedTicket) }}</dd>
              </div>
            </dl>
            <p v-if="selectedTicket.status === 'closed' && selectedTicket.closed_at" class="min-w-0 text-stone-500 dark:text-stone-400 sm:ml-auto">
              {{ t('feedback.closedBy', { actor: closedByLabel(selectedTicket.closed_by), time: formatDateTimeToMinute(selectedTicket.closed_at) }) }}
            </p>
            <p v-else class="min-w-0 text-stone-500 dark:text-stone-400 sm:ml-auto">
              {{ t('feedback.lastActivity') }}
              <time class="ml-1 tabular-nums" :datetime="selectedTicket.updated_at">{{ formatDateTimeToMinute(selectedTicket.updated_at) }}</time>
            </p>
          </div>
        </div>

        <div class="min-h-40 flex-1 overflow-y-auto px-4 py-5 sm:px-5">
          <div class="space-y-5">
            <article
              v-for="message in conversationEntries"
              :key="message.key"
              data-testid="feedback-message"
              :data-side="isOutgoingMessage(message.author_role) ? 'outgoing' : 'incoming'"
              :data-opening="message.opening"
              class="flex min-w-0 flex-col"
              :class="isOutgoingMessage(message.author_role) ? 'items-end' : 'items-start'"
            >
              <div class="mb-1.5 flex flex-wrap items-baseline gap-x-2 gap-y-1 px-1 text-[11px] text-stone-500 dark:text-stone-400" :class="isOutgoingMessage(message.author_role) && 'justify-end'">
                <span class="font-medium text-stone-600 dark:text-stone-300">{{ authorLabel(message.author_role) }}</span>
                <span v-if="message.opening">{{ t('feedback.openingMessage') }}</span>
                <time class="tabular-nums" :datetime="message.created_at" :title="formatDateTimeToMinute(message.created_at)">{{ formatDateTimeToMinute(message.created_at) }}</time>
              </div>
              <div
                class="w-fit max-w-[92%] rounded-2xl border px-4 py-2.5 sm:max-w-[min(85%,42rem)]"
                :class="isOutgoingMessage(message.author_role)
                  ? 'rounded-tr-md border-emerald-500/25 bg-emerald-50 text-emerald-950 dark:bg-emerald-500/[0.12] dark:text-stone-100'
                  : 'rounded-tl-md border-stone-200 bg-stone-100 text-stone-800 dark:border-white/10 dark:bg-white/[0.05] dark:text-stone-200'"
              >
                <p class="whitespace-pre-wrap text-sm leading-6 [overflow-wrap:anywhere]">{{ message.content }}</p>
              </div>
            </article>
            <div v-if="messagesLoading" class="py-4 text-center text-sm text-stone-400">{{ t('feedback.loadingMessages') }}</div>
            <div v-else-if="messages.length === 0" class="py-4 text-center text-sm text-stone-400">{{ t('feedback.noReplies') }}</div>
          </div>
        </div>

        <div data-testid="feedback-ticket-footer" class="shrink-0 border-t border-stone-200 p-4 dark:border-white/10">
          <div v-if="messagePagination.pages > 1" class="mb-3 flex items-center justify-between gap-3 text-xs text-stone-500">
            <button type="button" class="btn btn-secondary h-8 px-3 text-xs" :disabled="messagePagination.page >= messagePagination.pages || messagesLoading" @click="changeMessagePage(messagePagination.page + 1)">
              {{ t('feedback.olderMessages') }}
            </button>
            <span>{{ t('feedback.messagePage', { page: messagePagination.page, pages: messagePagination.pages }) }}</span>
            <button type="button" class="btn btn-secondary h-8 px-3 text-xs" :disabled="messagePagination.page <= 1 || messagesLoading" @click="changeMessagePage(messagePagination.page - 1)">
              {{ t('feedback.newerMessages') }}
            </button>
          </div>

          <form v-if="selectedTicket.status === 'open'" class="space-y-2" @submit.prevent="sendReply">
            <label for="feedback-reply-content" class="sr-only">{{ t('feedback.sendReply') }}</label>
            <textarea
              id="feedback-reply-content"
              v-model="replyDraft"
              rows="3"
              class="input min-h-24 resize-y"
              :class="displayReplyError && 'border-rose-400 focus:border-rose-500 focus:ring-rose-500/10'"
              :placeholder="t('feedback.replyPlaceholder')"
              :disabled="replying"
              maxlength="4000"
              @paste="handlePaste"
              @drop.prevent="attachmentError = t('feedback.textOnly')"
              @dragover.prevent
            ></textarea>
            <div class="flex items-center justify-between gap-3 text-xs">
              <p class="min-w-0 text-stone-500">
                <span v-if="displayReplyError" class="text-rose-600 dark:text-rose-300">{{ replyError }}</span>
                <span v-else-if="attachmentError" class="text-amber-600 dark:text-amber-300">{{ attachmentError }}</span>
                <span v-else-if="cooldownRemaining > 0">{{ t('feedback.cooldown', { seconds: cooldownRemaining }) }}</span>
                <span v-else>{{ t('feedback.textOnly') }}</span>
              </p>
              <span :class="replyLength > maxContentLength ? 'text-rose-600 dark:text-rose-300' : 'text-stone-400'">{{ replyLength }} / {{ maxContentLength }}</span>
            </div>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary" :disabled="replyDisabled">
                <Icon v-if="replying" name="refresh" size="sm" class="mr-1 animate-spin" />
                <Icon v-else name="chat" size="sm" class="mr-1" />
                {{ replying ? t('common.sending') : replyButtonLabel }}
              </button>
            </div>
          </form>
          <p v-else class="text-center text-sm text-stone-500 dark:text-stone-400">{{ t('feedback.closedReadOnly') }}</p>
        </div>
      </div>
    </section>

    <ConfirmDialog
      :show="Boolean(pendingCloseTicket)"
      :title="t('feedback.closeConfirmTitle')"
      :message="t('feedback.closeConfirmMessage')"
      :confirm-text="t('feedback.closeTicket')"
      danger
      @confirm="confirmClose"
      @cancel="pendingCloseTicket = null"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import type { BasePaginationResponse } from '@/types'
import type { FeedbackListFilters, FeedbackReadState, FeedbackReply, FeedbackReplyResult, FeedbackReplyStatus, FeedbackTicket } from '@/api/feedback'
import { useAppStore } from '@/stores/app'
import { formatDateTime, formatDateTimeToMinute } from '@/utils/format'

import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'

interface ThreadAPI<T extends FeedbackTicket> {
  list: (page: number, pageSize: number, filters: FeedbackListFilters, options?: { signal?: AbortSignal }) => Promise<BasePaginationResponse<T>>
  get: (id: number, signal?: AbortSignal) => Promise<T>
  listMessages: (id: number, page: number, pageSize: number, signal?: AbortSignal) => Promise<BasePaginationResponse<FeedbackReply>>
  reply: (id: number, content: string, signal?: AbortSignal) => Promise<FeedbackReplyResult>
  close: (id: number, signal?: AbortSignal) => Promise<T>
  markRead: (id: number, lastReadReplyID: number, signal?: AbortSignal) => Promise<FeedbackReadState>
}

type AnyTicket = FeedbackTicket & {
  user_id?: number
  user_name?: string
  user_email?: string
  api_key_id?: number | null
  key_name?: string
  key_prefix?: string
  member_id?: number | null
}

const props = withDefaults(defineProps<{
  identityKey: string
  title: string
  api: ThreadAPI<AnyTicket>
  showCreate?: boolean
  admin?: boolean
  fillHeight?: boolean
}>(), {
  showCreate: false,
  admin: false,
  fillHeight: false,
})

const emit = defineEmits<{
  (event: 'create'): void
  (event: 'unauthorized'): void
}>()

defineExpose({ loadTickets, refreshAfterCreate, applyCooldown })

const { t } = useI18n()
const appStore = useAppStore()
const maxContentLength = 2000
const messagePageSize = 20
const ticketPageSize = 20

const tickets = ref<AnyTicket[]>([])
const selectedTicket = ref<AnyTicket | null>(null)
const messages = ref<FeedbackReply[]>([])
const ticketsLoading = ref(false)
const detailLoading = ref(false)
const messagesLoading = ref(false)
const replying = ref(false)
const closing = ref(false)
type TicketFilter = FeedbackReplyStatus | 'closed' | ''
const statusFilter = ref<TicketFilter>('')
const replyDraft = ref('')
const attachmentError = ref('')
const attemptedReply = ref(false)
const pendingCloseTicket = ref<AnyTicket | null>(null)
const closedTicketIds = new Set<number>()
const now = ref(Date.now())
const cooldownUntil = ref(0)

const ticketPagination = reactive({ page: 1, page_size: ticketPageSize, total: 0, pages: 0 })
const messagePagination = reactive({ page: 1, page_size: messagePageSize, total: 0, pages: 0 })

let ticketsController: AbortController | null = null
let detailController: AbortController | null = null
let messagesController: AbortController | null = null
let mutationController: AbortController | null = null
let readController: AbortController | null = null
let isUnmounted = false
let identityEpoch = 0
let selectionEpoch = 0
let tickId: number | null = null
let pollId: number | null = null
let polling = false
let lastAckByTicket = new Map<number, number>()

const statusOptions = computed(() => [
  { value: '' as const, label: t('feedback.allTickets') },
  { value: 'pending' as const, label: t('feedback.replyStatusLabels.pending') },
  { value: 'replied' as const, label: t('feedback.replyStatusLabels.replied') },
  { value: 'closed' as const, label: t('feedback.statusLabels.closed') },
])
const conversationEntries = computed(() => {
  const ticket = selectedTicket.value
  if (!ticket) return []
  return [
    { key: `opening:${ticket.id}`, author_role: 'user' as const, content: ticket.content, created_at: ticket.created_at, opening: true },
    ...[...messages.value].reverse().map(message => ({ ...message, key: `reply:${message.id}`, opening: false })),
  ]
})
const trimmedReply = computed(() => replyDraft.value.trim())
const replyLength = computed(() => Array.from(trimmedReply.value).length)
const cooldownRemaining = computed(() => Math.max(0, Math.ceil((cooldownUntil.value - now.value) / 1000)))
const replyError = computed(() => {
  if (!trimmedReply.value) return t('feedback.emptyError')
  if (replyLength.value > maxContentLength) return t('feedback.tooLongError', { max: maxContentLength })
  return ''
})
const displayReplyError = computed(() => attemptedReply.value && Boolean(replyError.value))
const replyDisabled = computed(() => replying.value || closing.value || cooldownRemaining.value > 0 || Boolean(replyError.value))
const replyButtonLabel = computed(() => cooldownRemaining.value > 0 ? t('feedback.waitSubmit', { seconds: cooldownRemaining.value }) : t('feedback.sendReply'))

function preview(content: string) {
  const normalized = content.trim()
  const chars = Array.from(normalized)
  return chars.length > 96 ? `${chars.slice(0, 96).join('')}...` : normalized
}

function ticketTitle(ticket: AnyTicket) {
  return ticket.title?.trim() || preview(ticket.content)
}

function formatQueueTime(value: string) {
  return formatDateTime(value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false })
}

function isOutgoingMessage(role: FeedbackReply['author_role']) {
  return role === (props.admin ? 'admin' : 'user')
}

function statusLabel(ticket: AnyTicket, detail = false) {
  if (ticket.status === 'closed') return t('feedback.statusLabels.closed')
  // Older servers do not expose reply progress; do not invent a pending state.
  if (!ticket.reply_status) return t('feedback.statusLabels.open')
  return t(`feedback.${detail && !props.admin ? 'userReplyStatusLabels' : 'replyStatusLabels'}.${ticket.reply_status}`)
}

function statusBadge(ticket: AnyTicket) {
  if (ticket.status === 'closed') return 'badge-gray'
  return ticket.reply_status === 'pending' ? 'badge-warning' : 'badge-success'
}

function listFilters(): FeedbackListFilters {
  if (statusFilter.value === 'pending' || statusFilter.value === 'replied') {
    return { status: 'open', reply_status: statusFilter.value }
  }
  return { status: statusFilter.value || undefined }
}

function sourceLabel(source: AnyTicket['source']) {
  return source === 'key' ? t('feedback.sourceLabels.key') : t('feedback.sourceLabels.user')
}

function closedByLabel(value: AnyTicket['closed_by']) {
  if (value === 'admin') return t('feedback.authorLabels.admin')
  if (value === 'user') return t('feedback.authorLabels.user')
  return t('common.unknown')
}

function authorLabel(role: FeedbackReply['author_role']) {
  return role === 'admin' ? t('feedback.authorLabels.admin') : t('feedback.authorLabels.user')
}

// Closure is final. A late list/detail response must not reopen a ticket
// after a successful close or an authoritative FEEDBACK_CLOSED rejection.
function preserveClosedState(ticket: AnyTicket): AnyTicket {
  if (ticket.status === 'closed') closedTicketIds.add(ticket.id)
  return closedTicketIds.has(ticket.id) && ticket.status !== 'closed'
    ? { ...ticket, status: 'closed' }
    : ticket
}

function markClosed(id: number) {
  closedTicketIds.add(id)
  tickets.value = tickets.value.map(preserveClosedState)
  if (selectedTicket.value?.id === id) selectedTicket.value = preserveClosedState(selectedTicket.value)
  if (pendingCloseTicket.value?.id === id) pendingCloseTicket.value = null
}

function ticketUserName(ticket: AnyTicket) {
  return ticket.user_name?.trim() || ticket.user_email?.trim() || t('admin.feedback.unnamedUser')
}

function ticketKeyName(ticket: AnyTicket) {
  return ticket.key_name || ticket.key_prefix || (ticket.api_key_id ? t('admin.feedback.keyId', { id: ticket.api_key_id }) : t('admin.feedback.deletedKey'))
}

function isCanceled(error: unknown) {
  return Boolean(error && typeof error === 'object' && (error as { code?: string }).code === 'ERR_CANCELED')
}

function extractStatus(error: unknown) {
  if (!error || typeof error !== 'object') return undefined
  const value = error as { status?: number; response?: { status?: number; data?: { code?: string } }; code?: string }
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

function ensureTicking() {
  if (tickId !== null) return
  tickId = window.setInterval(() => {
    now.value = Date.now()
    if (cooldownRemaining.value <= 0) stopTicking()
  }, 1000)
}

function stopTicking() {
  if (tickId === null) return
  window.clearInterval(tickId)
  tickId = null
}

function startCooldown(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    cooldownUntil.value = 0
    now.value = Date.now()
    stopTicking()
    return
  }
  const normalized = Number.isFinite(seconds) ? Math.max(1, Math.ceil(seconds)) : 60
  cooldownUntil.value = Date.now() + normalized * 1000
  now.value = Date.now()
  ensureTicking()
}

function applyCooldown(seconds?: number) {
  if (seconds === undefined) return
  startCooldown(seconds)
}

function resetForIdentity() {
  identityEpoch += 1
  selectionEpoch += 1
  ticketsController?.abort()
  detailController?.abort()
  messagesController?.abort()
  mutationController?.abort()
  readController?.abort()
  tickets.value = []
  selectedTicket.value = null
  messages.value = []
  replyDraft.value = ''
  attachmentError.value = ''
  attemptedReply.value = false
  pendingCloseTicket.value = null
  closedTicketIds.clear()
  lastAckByTicket = new Map()
  replying.value = false
  closing.value = false
  ticketsLoading.value = false
  detailLoading.value = false
  messagesLoading.value = false
  cooldownUntil.value = 0
  ticketPagination.page = 1
  ticketPagination.total = 0
  ticketPagination.pages = 0
  messagePagination.page = 1
  messagePagination.total = 0
  messagePagination.pages = 0
  stopTicking()
  stopPolling()
}

async function loadTickets() {
  await fetchTickets(false)
}

async function fetchTickets(preserveSelected: boolean, quiet = false) {
  if (isUnmounted || !props.identityKey) return
  ticketsController?.abort()
  const requestEpoch = identityEpoch
  const controller = new AbortController()
  ticketsController = controller
  ticketsLoading.value = true
  let page = ticketPagination.page
  const pageSize = ticketPagination.page_size
  const filters = listFilters()

  try {
    while (true) {
      const res = await props.api.list(page, pageSize, filters, { signal: controller.signal })
      if (controller.signal.aborted || requestEpoch !== identityEpoch || ticketsController !== controller) return
      const lastPage = Math.max(1, res.pages)
      // Replies or closures can shrink a filtered queue. Each retry moves to a lower page.
      if (page > lastPage) {
        page = lastPage
        continue
      }
      tickets.value = res.items.map(preserveClosedState)
      ticketPagination.total = res.total
      ticketPagination.pages = res.pages
      ticketPagination.page = res.page
      ticketPagination.page_size = res.page_size
      if (!preserveSelected && selectedTicket.value && !res.items.some((ticket) => ticket.id === selectedTicket.value?.id)) {
        selectionEpoch += 1
        detailController?.abort()
        messagesController?.abort()
        selectedTicket.value = null
        messages.value = []
      }
      break
    }
  } catch (error: unknown) {
    if (controller.signal.aborted || requestEpoch !== identityEpoch || isCanceled(error)) return
    if (extractStatus(error) === 401) emit('unauthorized')
    else if (!quiet) appStore.showError(t('feedback.loadFailed'))
  } finally {
    if (ticketsController === controller) {
      ticketsController = null
      ticketsLoading.value = false
    }
  }
}

async function refreshAfterCreate(createdId?: number) {
  statusFilter.value = ''
  ticketPagination.page = 1
  selectionEpoch += 1
  detailController?.abort()
  messagesController?.abort()
  selectedTicket.value = null
  messages.value = []
  await loadTickets()
  if (!createdId) return
  const createdTicket = tickets.value.find((ticket) => ticket.id === createdId)
  if (createdTicket) await selectTicket(createdTicket)
}

function setStatusFilter(value: TicketFilter) {
  statusFilter.value = value
  ticketPagination.page = 1
  void loadTickets()
}

function changeTicketPage(page: number) {
  ticketPagination.page = page
  void loadTickets()
}

function changeMessagePage(page: number) {
  messagePagination.page = page
  if (selectedTicket.value) void loadMessages(selectedTicket.value.id)
}

async function selectTicket(ticket: AnyTicket) {
  selectionEpoch += 1
  detailController?.abort()
  messagesController?.abort()
  selectedTicket.value = preserveClosedState(ticket)
  messages.value = []
  messagesLoading.value = false
  detailLoading.value = false
  messagePagination.page = 1
  replyDraft.value = ''
  attachmentError.value = ''
  attemptedReply.value = false
  await refreshSelected()
}

async function refreshSelected(quiet = false) {
  const ticket = selectedTicket.value
  if (!ticket || !props.identityKey) return
  detailController?.abort()
  const requestEpoch = identityEpoch
  const requestSelectionEpoch = selectionEpoch
  const ticketId = ticket.id
  const controller = new AbortController()
  detailController = controller
  detailLoading.value = true

  try {
    const fresh = await props.api.get(ticket.id, controller.signal)
    if (controller.signal.aborted || requestEpoch !== identityEpoch || requestSelectionEpoch !== selectionEpoch || detailController !== controller || selectedTicket.value?.id !== ticketId) return
    selectedTicket.value = preserveClosedState(fresh)
    await loadMessages(fresh.id, quiet)
  } catch (error: unknown) {
    if (controller.signal.aborted || requestEpoch !== identityEpoch || isCanceled(error)) return
    if (extractStatus(error) === 401) emit('unauthorized')
    else if (!quiet) appStore.showError(t('feedback.detailLoadFailed'))
  } finally {
    if (detailController === controller) {
      detailController = null
      detailLoading.value = false
    }
  }
}

async function loadMessages(id: number, quiet = false) {
  messagesController?.abort()
  const requestEpoch = identityEpoch
  const requestSelectionEpoch = selectionEpoch
  const controller = new AbortController()
  messagesController = controller
  messagesLoading.value = true

  try {
    const res = await props.api.listMessages(id, messagePagination.page, messagePagination.page_size, controller.signal)
    if (controller.signal.aborted || requestEpoch !== identityEpoch || requestSelectionEpoch !== selectionEpoch || messagesController !== controller || selectedTicket.value?.id !== id) return
    messages.value = res.items
    messagePagination.total = res.total
    messagePagination.pages = res.pages
    messagePagination.page = res.page
    messagePagination.page_size = res.page_size
    await acknowledgeDisplayedMessages(id, res.items, requestEpoch, requestSelectionEpoch)
  } catch (error: unknown) {
    if (controller.signal.aborted || requestEpoch !== identityEpoch || isCanceled(error)) return
    if (extractStatus(error) === 401) emit('unauthorized')
    else if (!quiet) appStore.showError(t('feedback.messagesLoadFailed'))
  } finally {
    if (messagesController === controller) {
      messagesController = null
      messagesLoading.value = false
    }
  }
}

function isVisible() {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

function displayedReadCursor(items: FeedbackReply[]) {
  if (messagePagination.page !== 1) return null
  return items.reduce((max, message) => Math.max(max, message.id), 0)
}

async function acknowledgeDisplayedMessages(id: number, items: FeedbackReply[], requestEpoch: number, requestSelectionEpoch: number) {
  const cursor = displayedReadCursor(items)
  if (cursor === null || !isVisible() || isUnmounted || !props.identityKey || selectedTicket.value?.id !== id) return
  if ((lastAckByTicket.get(id) ?? -1) >= cursor) return
  await nextTick()
  if (messagePagination.page !== 1 || !isVisible() || isUnmounted || requestEpoch !== identityEpoch || requestSelectionEpoch !== selectionEpoch || selectedTicket.value?.id !== id) return

  readController?.abort()
  const controller = new AbortController()
  readController = controller
  try {
    await props.api.markRead(id, cursor, controller.signal)
    if (controller.signal.aborted || requestEpoch !== identityEpoch || requestSelectionEpoch !== selectionEpoch || selectedTicket.value?.id !== id) return
    lastAckByTicket.set(id, cursor)
    await fetchTickets(true, true)
  } catch (error: unknown) {
    if (controller.signal.aborted || requestEpoch !== identityEpoch || isCanceled(error)) return
    if (extractStatus(error) === 401) emit('unauthorized')
  } finally {
    if (readController === controller) readController = null
  }
}

async function pollFeedback() {
  if (polling || ticketsLoading.value || isUnmounted || !props.identityKey || !isVisible()) return
  const requestEpoch = identityEpoch
  polling = true
  try {
    await fetchTickets(true, true)
    if (isUnmounted || requestEpoch !== identityEpoch || !props.identityKey || !isVisible()) return
    if (selectedTicket.value && messagePagination.page === 1 && !detailLoading.value && !messagesLoading.value) {
      await refreshSelected(true)
    }
  } finally {
    polling = false
  }
}

function startPolling() {
  if (pollId !== null || !props.identityKey) return
  pollId = window.setInterval(() => {
    void pollFeedback()
  }, 15_000)
}

function stopPolling() {
  if (pollId === null) return
  window.clearInterval(pollId)
  pollId = null
}

function handleVisibilityChange() {
  if (isVisible()) {
    startPolling()
    void pollFeedback()
  } else {
    readController?.abort()
    stopPolling()
  }
}

function handlePaste(event: ClipboardEvent) {
  const items = Array.from(event.clipboardData?.items || [])
  const hasFileOrImage = items.some((item) => item.kind === 'file' || item.type.startsWith('image/'))
  if (!hasFileOrImage && (event.clipboardData?.files?.length || 0) === 0) return
  event.preventDefault()
  attachmentError.value = t('feedback.textOnly')
}

async function sendReply() {
  attemptedReply.value = true
  if (replyDisabled.value || !selectedTicket.value || closing.value) return
  const ticketId = selectedTicket.value.id
  const requestEpoch = identityEpoch
  const requestSelectionEpoch = selectionEpoch
  const controller = new AbortController()
  mutationController = controller
  replying.value = true
  attachmentError.value = ''

  try {
    const result = await props.api.reply(ticketId, trimmedReply.value, controller.signal)
    if (controller.signal.aborted || requestEpoch !== identityEpoch || requestSelectionEpoch !== selectionEpoch || selectedTicket.value?.id !== ticketId) return
    replyDraft.value = ''
    attemptedReply.value = false
    startCooldown(result.retry_after)
    appStore.showSuccess(t('feedback.replySuccess'))
    messagePagination.page = 1
    await refreshSelected()
    await fetchTickets(true)
  } catch (error: unknown) {
    if (controller.signal.aborted || requestEpoch !== identityEpoch || isCanceled(error)) return
    const status = extractStatus(error)
    if (status === 409) {
      markClosed(ticketId)
      appStore.showWarning(t('feedback.closedStale'))
      if (selectedTicket.value?.id === ticketId) await refreshSelected()
    } else if (status === 429) {
      const retryAfter = extractRetryAfter(error) || 60
      startCooldown(retryAfter)
      appStore.showError(t('feedback.rateLimited', { seconds: Math.ceil(retryAfter) }))
    } else if (status === 401) {
      emit('unauthorized')
    } else {
      appStore.showError(t('feedback.replyFailed'))
    }
  } finally {
    if (mutationController === controller) {
      mutationController = null
      replying.value = false
    }
  }
}

async function confirmClose() {
  const ticket = pendingCloseTicket.value
  if (!ticket || replying.value) return
  pendingCloseTicket.value = null
  const requestEpoch = identityEpoch
  const requestSelectionEpoch = selectionEpoch
  const controller = new AbortController()
  mutationController = controller
  closing.value = true

  try {
    const closed = await props.api.close(ticket.id, controller.signal)
    if (controller.signal.aborted || requestEpoch !== identityEpoch || requestSelectionEpoch !== selectionEpoch) return
    selectionEpoch += 1
    markClosed(ticket.id)
    if (selectedTicket.value?.id === ticket.id) selectedTicket.value = preserveClosedState(closed)
    appStore.showSuccess(t('feedback.closeSuccess'))
    await fetchTickets(true)
  } catch (error: unknown) {
    if (controller.signal.aborted || requestEpoch !== identityEpoch || isCanceled(error)) return
    if (extractStatus(error) === 409) {
      markClosed(ticket.id)
      appStore.showWarning(t('feedback.closedStale'))
      if (selectedTicket.value?.id === ticket.id) await refreshSelected()
    } else if (extractStatus(error) === 401) {
      emit('unauthorized')
    } else {
      appStore.showError(t('feedback.closeFailed'))
    }
  } finally {
    if (mutationController === controller) {
      mutationController = null
      closing.value = false
    }
  }
}

watch(
  () => props.identityKey,
  () => {
    resetForIdentity()
    if (props.identityKey) {
      if (isVisible()) startPolling()
      void loadTickets()
    }
  },
  { immediate: true },
)

onMounted(() => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  if (props.identityKey && isVisible()) startPolling()
})

onBeforeUnmount(() => {
  isUnmounted = true
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  resetForIdentity()
})
</script>
