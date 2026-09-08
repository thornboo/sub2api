<template>
  <div class="grid min-h-[560px] gap-4 lg:grid-cols-[320px_minmax(0,1fr)]">
    <section class="min-w-0 rounded-lg border border-stone-200 bg-white dark:border-white/10 dark:bg-[#0d0d0d]">
      <div class="border-b border-stone-200 p-3 dark:border-white/10">
        <div class="flex items-center justify-between gap-2">
          <h2 class="text-sm font-semibold text-stone-900 dark:text-white">{{ title }}</h2>
          <div class="flex items-center gap-1">
            <button
              type="button"
              class="rounded-lg p-1.5 text-stone-500 hover:bg-stone-100 hover:text-stone-900 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:bg-white/10 dark:hover:text-white"
              :disabled="ticketsLoading"
              :title="t('common.refresh')"
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
        <div class="mt-3 grid grid-cols-3 gap-1 rounded-lg bg-stone-100 p-1 text-xs dark:bg-white/[0.06]">
          <button
            v-for="option in statusOptions"
            :key="option.value || 'all'"
            type="button"
            :data-testid="`feedback-filter-${option.value || 'all'}`"
            class="rounded-md px-2 py-1.5 font-medium transition"
            :class="statusFilter === option.value ? 'bg-white text-stone-950 shadow-sm dark:bg-[#1b1b1b] dark:text-white' : 'text-stone-500 hover:text-stone-900 dark:text-stone-400 dark:hover:text-white'"
            @click="setStatusFilter(option.value)"
          >
            {{ option.label }}
          </button>
        </div>
      </div>

      <div class="max-h-[430px] overflow-y-auto">
        <button
          v-for="ticket in tickets"
          :key="ticket.id"
          type="button"
          data-testid="feedback-ticket-row"
          class="block w-full border-b border-stone-100 px-3 py-3 text-left transition last:border-b-0 hover:bg-stone-50 dark:border-white/[0.06] dark:hover:bg-white/[0.04]"
          :class="selectedTicket?.id === ticket.id && 'bg-emerald-50/70 dark:bg-emerald-500/[0.08]'"
          @click="selectTicket(ticket)"
        >
          <div class="flex items-center justify-between gap-3">
            <span class="inline-flex items-center gap-1.5 font-mono text-xs text-stone-500">
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
            <span :class="['badge', ticket.status === 'open' ? 'badge-success' : 'badge-gray']">{{ statusLabel(ticket.status) }}</span>
          </div>
          <p class="mt-2 line-clamp-2 whitespace-pre-wrap break-words text-sm leading-5 text-stone-800 dark:text-stone-200">{{ preview(ticket.content) }}</p>
          <div class="mt-2 flex min-w-0 items-center justify-between gap-2 text-xs text-stone-400">
            <span class="truncate">{{ ticketMeta(ticket) }}</span>
            <span class="shrink-0">{{ formatDateTime(ticket.updated_at) }}</span>
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

    <section class="min-w-0 rounded-lg border border-stone-200 bg-white dark:border-white/10 dark:bg-[#0d0d0d]">
      <div v-if="!selectedTicket" class="flex min-h-[520px] items-center justify-center px-4 text-sm text-stone-400">
        {{ t('feedback.selectTicket') }}
      </div>

      <div v-else class="flex min-h-[560px] flex-col">
        <div class="border-b border-stone-200 p-4 dark:border-white/10">
          <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <h2 class="font-mono text-sm font-semibold text-stone-900 dark:text-white">#{{ selectedTicket.id }}</h2>
                <span :class="['badge', selectedTicket.status === 'open' ? 'badge-success' : 'badge-gray']">{{ statusLabel(selectedTicket.status) }}</span>
                <span v-if="selectedTicket.source" :class="['badge', selectedTicket.source === 'key' ? 'badge-warning' : 'badge-success']">{{ sourceLabel(selectedTicket.source) }}</span>
              </div>
              <p class="mt-1 text-xs text-stone-500 dark:text-stone-400">{{ formatDateTime(selectedTicket.created_at) }} · {{ t('feedback.lastActivity') }} {{ formatDateTime(selectedTicket.updated_at) }}</p>
              <p v-if="selectedTicket.status === 'closed' && selectedTicket.closed_at" class="mt-1 text-xs text-stone-500 dark:text-stone-400">
                {{ t('feedback.closedBy', { actor: closedByLabel(selectedTicket.closed_by), time: formatDateTime(selectedTicket.closed_at) }) }}
              </p>
            </div>
            <div class="flex items-center gap-2">
              <button type="button" class="btn btn-secondary h-9 px-3 text-sm" :disabled="detailLoading" @click="() => refreshSelected()">
                <Icon name="refresh" size="sm" :class="detailLoading && 'animate-spin'" class="mr-1" />
                {{ t('common.refresh') }}
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
        </div>

        <div class="flex-1 overflow-y-auto p-4">
          <article class="rounded-lg border border-stone-200 bg-stone-50 p-3 dark:border-white/10 dark:bg-white/[0.04]">
            <div class="mb-2 flex items-center justify-between gap-3 text-xs text-stone-500">
              <span class="font-semibold text-stone-700 dark:text-stone-300">{{ t('feedback.openingMessage') }}</span>
              <span>{{ formatDateTime(selectedTicket.created_at) }}</span>
            </div>
            <p class="whitespace-pre-wrap break-words text-sm leading-6 text-stone-800 dark:text-stone-200">{{ selectedTicket.content }}</p>
          </article>

          <div class="mt-4 space-y-3">
            <article
              v-for="message in chronologicalMessages"
              :key="message.id"
              class="rounded-lg border p-3"
              :class="message.author_role === 'admin'
                ? 'border-emerald-500/25 bg-emerald-50/70 dark:bg-emerald-500/[0.08]'
                : 'border-stone-200 bg-white dark:border-white/10 dark:bg-black/20'"
            >
              <div class="mb-2 flex items-center justify-between gap-3 text-xs text-stone-500">
                <span class="font-semibold text-stone-700 dark:text-stone-300">{{ authorLabel(message.author_role) }}</span>
                <span>{{ formatDateTime(message.created_at) }}</span>
              </div>
              <p class="whitespace-pre-wrap break-words text-sm leading-6 text-stone-800 dark:text-stone-200">{{ message.content }}</p>
            </article>
            <div v-if="messagesLoading" class="py-4 text-center text-sm text-stone-400">{{ t('feedback.loadingMessages') }}</div>
            <div v-else-if="messages.length === 0" class="py-4 text-center text-sm text-stone-400">{{ t('feedback.noReplies') }}</div>
          </div>
        </div>

        <div class="border-t border-stone-200 p-4 dark:border-white/10">
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
          <p v-else class="rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-sm text-stone-500 dark:border-white/10 dark:bg-white/[0.04] dark:text-stone-400">
            {{ t('feedback.closedReadOnly') }}
          </p>
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
import type { FeedbackReadState, FeedbackReply, FeedbackReplyResult, FeedbackStatus, FeedbackTicket } from '@/api/feedback'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'

interface ThreadAPI<T extends FeedbackTicket> {
  list: (page: number, pageSize: number, filters: { status?: FeedbackStatus }, options?: { signal?: AbortSignal }) => Promise<BasePaginationResponse<T>>
  get: (id: number, signal?: AbortSignal) => Promise<T>
  listMessages: (id: number, page: number, pageSize: number, signal?: AbortSignal) => Promise<BasePaginationResponse<FeedbackReply>>
  reply: (id: number, content: string, signal?: AbortSignal) => Promise<FeedbackReplyResult>
  close: (id: number, signal?: AbortSignal) => Promise<T>
  markRead: (id: number, lastReadReplyID: number, signal?: AbortSignal) => Promise<FeedbackReadState>
}

type AnyTicket = FeedbackTicket & {
  user_id?: number
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
}>(), {
  showCreate: false,
  admin: false,
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
const statusFilter = ref<FeedbackStatus | ''>('')
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
  { value: 'open' as const, label: t('feedback.statusLabels.open') },
  { value: 'closed' as const, label: t('feedback.statusLabels.closed') },
])
const chronologicalMessages = computed(() => [...messages.value].reverse())
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

function statusLabel(status: FeedbackStatus) {
  return t(`feedback.statusLabels.${status}`)
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

function ticketMeta(ticket: AnyTicket) {
  if (!props.admin) return t('feedback.lastActivity')
  if (ticket.source === 'key') {
    const title = ticket.key_name || ticket.key_prefix || (ticket.api_key_id ? t('admin.feedback.keyId', { id: ticket.api_key_id }) : t('admin.feedback.deletedKey'))
    return [title, ticket.user_id ? t('admin.feedback.userId', { id: ticket.user_id }) : ''].filter(Boolean).join(' · ')
  }
  return ticket.user_email || (ticket.user_id ? t('admin.feedback.userId', { id: ticket.user_id }) : sourceLabel(ticket.source))
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

  try {
    const res = await props.api.list(ticketPagination.page, ticketPagination.page_size, { status: statusFilter.value || undefined }, { signal: controller.signal })
    if (controller.signal.aborted || requestEpoch !== identityEpoch || ticketsController !== controller) return
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

function setStatusFilter(value: FeedbackStatus | '') {
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
