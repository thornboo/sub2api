<template>
  <section
    class="min-w-0 max-w-full border-t border-stone-200 pt-5 [overflow-wrap:anywhere] dark:border-white/10"
    aria-live="polite"
  >
    <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
      <div class="min-w-0">
        <h3 class="text-sm font-semibold text-stone-900 dark:text-stone-50">
          {{ t('channelStatus.selfCheckChain.title') }}
        </h3>
        <p class="mt-1 max-w-2xl text-xs leading-relaxed text-stone-500 dark:text-stone-400">
          {{ t('channelStatus.selfCheckChain.description') }}
        </p>
      </div>
      <button
        v-if="showRetry"
        type="button"
        class="btn btn-secondary btn-sm"
        @click="$emit('retry')"
      >
        {{ t('channelStatus.selfCheckChain.retry') }}
      </button>
    </div>

    <div v-if="loading" class="mt-4 space-y-2">
      <div v-for="i in 3" :key="i" class="h-10 animate-pulse rounded bg-stone-100 dark:bg-white/[0.04]"></div>
    </div>

    <div v-else-if="error" class="py-4 text-sm text-red-600 dark:text-red-300">
      {{ error }}
    </div>

    <div v-else-if="!chain" class="py-4 text-sm text-stone-500 dark:text-stone-400">
      {{ t('channelStatus.selfCheckChain.empty') }}
    </div>

    <div v-else class="mt-4 space-y-4">
      <div class="flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-stone-500 dark:text-stone-400">
        <span>
          {{ t('channelStatus.selfCheckChain.currentCandidates') }}
          <span class="ml-1 font-medium tabular-nums text-stone-900 dark:text-stone-100">{{ eligibleCandidates.length }} / {{ chain.candidates.length }}</span>
        </span>
        <span>{{ t('channelStatus.selfCheckChain.attemptTimeout') }} <span class="ml-1 tabular-nums">{{ chain.attempt_timeout_seconds }}s</span></span>
        <span>{{ t('channelStatus.selfCheckChain.roundTimeout') }} <span class="ml-1 tabular-nums">{{ chain.round_timeout_seconds }}s</span></span>
      </div>

      <div>
        <div class="mb-2 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <h4 class="text-xs font-semibold text-stone-700 dark:text-stone-200">
            {{ t('channelStatus.selfCheckChain.currentOrder') }}
          </h4>
          <span class="text-xs text-stone-500 dark:text-stone-400">
            {{ t('channelStatus.selfCheckChain.updatedAt', { time: formatTime(chain.updated_at) }) }}
          </span>
        </div>

        <div v-if="chain.candidates.length === 0" class="py-3 text-sm text-stone-500 dark:text-stone-400">
          {{ t('channelStatus.selfCheckChain.noCandidates') }}
        </div>
        <div v-else class="overflow-hidden rounded-lg border border-stone-200 dark:border-white/10">
          <div
            v-for="candidate in orderedCandidates"
            :key="candidate.account_id"
            class="grid grid-cols-[1.5rem_minmax(0,1fr)] items-start gap-x-3 gap-y-1 border-b border-stone-100 px-3 py-2.5 last:border-b-0 dark:border-white/[0.06] sm:grid-cols-[1.5rem_minmax(0,1fr)_minmax(0,0.7fr)]"
          >
            <div class="flex h-6 w-6 items-center justify-center text-xs font-medium tabular-nums text-stone-400 dark:text-stone-500">
              {{ candidate.eligible ? candidate.order : '-' }}
            </div>
            <div class="min-w-0">
              <div class="flex min-w-0 flex-wrap items-center gap-2">
                <span class="min-w-0 break-words text-sm font-medium text-stone-900 dark:text-stone-100">
                  {{ candidate.account_name || `#${candidate.account_id}` }}
                </span>
                <span class="text-xs text-stone-500 dark:text-stone-400">
                  {{ t('channelStatus.selfCheckChain.priority', { priority: candidate.priority }) }}
                </span>
                <span class="text-xs text-stone-500 dark:text-stone-400">
                  {{ candidate.platform || '-' }}
                </span>
              </div>
              <div class="mt-1 text-xs text-stone-500 dark:text-stone-400">
                {{ candidate.eligible ? t('channelStatus.selfCheckChain.eligible') : reasonLabel(candidate.reason_code) }}
              </div>
            </div>
            <div class="col-start-2 min-w-0 text-xs text-stone-500 dark:text-stone-400 sm:col-start-auto sm:text-right">
              <span class="mr-2">{{ t('channelStatus.selfCheckChain.lastProbe') }}</span>
              <span class="font-medium" :class="statusTextClass(candidate.last_status)">
                {{ statusLabel(candidate.last_status) }}
              </span>
              <div class="mt-1 tabular-nums" :title="candidate.last_checked_at ? formatTime(candidate.last_checked_at) : undefined">
                {{ candidate.last_checked_at ? formatTime(candidate.last_checked_at) : t('channelStatus.selfCheckChain.neverChecked') }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div>
        <div class="mb-2 flex flex-wrap items-center gap-2">
          <h4 class="text-xs font-semibold text-stone-700 dark:text-stone-200">
            {{ t('channelStatus.selfCheckChain.latestRound') }}
          </h4>
          <span v-if="latestRound" class="inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium" :class="roundStatusClass">
            {{ roundStatusLabel(latestRound.status) }}
          </span>
          <span v-else class="text-[11px] text-stone-500 dark:text-stone-400">
            {{ t('channelStatus.selfCheckChain.noRound') }}
          </span>
        </div>

        <div v-if="!latestRound" class="py-3 text-sm text-stone-500 dark:text-stone-400">
          {{ t('channelStatus.selfCheckChain.noRoundDetail') }}
        </div>
        <div v-else class="space-y-3">
          <div class="flex flex-wrap items-center gap-2 text-xs text-stone-500 dark:text-stone-400">
            <span>{{ t('channelStatus.selfCheckChain.startedAt', { time: formatTime(latestRound.started_at) }) }}</span>
            <span v-if="latestRound.finished_at">{{ t('channelStatus.selfCheckChain.finishedAt', { time: formatTime(latestRound.finished_at) }) }}</span>
            <span v-if="latestRound.duration_ms != null">{{ t('channelStatus.selfCheckChain.duration', { ms: latestRound.duration_ms }) }}</span>
            <span v-if="latestRound.winner_account_id != null">{{ t('channelStatus.selfCheckChain.winner', { id: latestRound.winner_account_id }) }}</span>
          </div>

          <div class="overflow-hidden rounded-lg border border-stone-200 dark:border-white/10">
            <div
              v-for="step in latestRound.steps"
              :key="`${latestRound.id}:${step.order}:${step.account_id}`"
              class="grid grid-cols-[1.5rem_minmax(0,1fr)] items-start gap-x-3 gap-y-2 border-b border-stone-100 px-3 py-2.5 last:border-b-0 dark:border-white/[0.06] md:grid-cols-[1.5rem_minmax(0,1fr)_minmax(0,0.8fr)_minmax(0,0.8fr)]"
            >
              <div class="flex h-6 w-6 items-center justify-center text-xs font-medium tabular-nums text-stone-400 dark:text-stone-500">
                {{ step.order || '-' }}
              </div>
              <div class="min-w-0">
                <div class="min-w-0 break-words text-sm font-medium text-stone-900 dark:text-stone-100">
                  {{ step.account_name || `#${step.account_id}` }}
                </div>
                <div class="mt-1 text-xs text-stone-500 dark:text-stone-400">
                  {{ t('channelStatus.selfCheckChain.priority', { priority: step.priority }) }} · {{ step.platform || '-' }}
                </div>
              </div>
              <div class="col-start-2 min-w-0 md:col-start-auto">
                <span class="inline-flex rounded px-1.5 py-0.5 text-[11px] font-medium" :class="stepOutcomeClass(step.outcome)">
                  {{ stepOutcomeLabel(step.outcome) }}
                </span>
                <div v-if="step.reason_code" class="mt-1 text-xs text-stone-500 dark:text-stone-400">
                  {{ reasonLabel(step.reason_code) }}
                </div>
              </div>
              <div class="col-start-2 min-w-0 text-xs text-stone-500 dark:text-stone-400 md:col-start-auto md:text-right">
                <div>{{ step.latency_ms != null ? `${step.latency_ms}ms` : t('channelStatus.selfCheckChain.noLatency') }}</div>
                <div v-if="step.http_status || step.error_code" class="mt-1">
                  {{ step.http_status ? `HTTP ${step.http_status}` : '' }}
                  {{ step.error_code ? step.error_code : '' }}
                </div>
                <div v-if="step.started_at" class="mt-1">
                  {{ formatTime(step.started_at) }}
                </div>
              </div>
            </div>
            <div v-if="latestRound.steps.length === 0" class="p-3 text-sm text-stone-500 dark:text-stone-400">
              {{ t('channelStatus.selfCheckChain.noSteps') }}
            </div>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  ModelSelfCheckChainCandidate,
  ModelSelfCheckChainView,
  ModelSelfCheckRoundStatus,
  ModelSelfCheckStepOutcome,
  ModelStatus,
} from '@/api/modelStatus'
import { STATUS_DEGRADED, STATUS_FAILED, STATUS_OPERATIONAL } from '@/constants/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

const props = defineProps<{
  chain: ModelSelfCheckChainView | null
  loading: boolean
  error: string
}>()

defineEmits<{
  retry: []
}>()

const { t } = useI18n()
const { statusLabel } = useChannelMonitorFormat()

const latestRound = computed(() => props.chain?.latest_round ?? null)
const eligibleCandidates = computed(() => props.chain?.candidates.filter(candidate => candidate.eligible) ?? [])
const showRetry = computed(() => !props.loading && props.error !== '')

const orderedCandidates = computed<ModelSelfCheckChainCandidate[]>(() => {
  const candidates = props.chain?.candidates ?? []
  return [...candidates].sort((a, b) => {
    if (a.eligible !== b.eligible) return a.eligible ? -1 : 1
    const orderA = a.order > 0 ? a.order : Number.MAX_SAFE_INTEGER
    const orderB = b.order > 0 ? b.order : Number.MAX_SAFE_INTEGER
    if (orderA !== orderB) return orderA - orderB
    if (a.priority !== b.priority) return a.priority - b.priority
    return a.account_id - b.account_id
  })
})

const roundStatusClass = computed(() => latestRound.value ? statusPillClass(latestRound.value.status) : 'bg-stone-100 text-stone-600 dark:bg-white/10 dark:text-stone-300')

function roundStatusLabel(status: ModelSelfCheckRoundStatus): string {
  return t(`channelStatus.selfCheckChain.roundStatusLabel.${status}`)
}

function stepOutcomeLabel(outcome: ModelSelfCheckStepOutcome): string {
  return t(`channelStatus.selfCheckChain.stepOutcome.${outcome}`)
}

function reasonLabel(code: string): string {
  if (!code) return t('channelStatus.selfCheckChain.reason.none')
  const key = `channelStatus.selfCheckChain.reason.${code}`
  const value = t(key)
  return value === key ? code : value
}

function statusPillClass(status: ModelSelfCheckRoundStatus | ModelStatus): string {
  switch (status) {
    case STATUS_OPERATIONAL:
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-200'
    case STATUS_DEGRADED:
      return 'bg-amber-100 text-amber-700 dark:bg-amber-400/15 dark:text-amber-200'
    case STATUS_FAILED:
    case 'error':
      return 'bg-red-100 text-red-700 dark:bg-red-400/15 dark:text-red-200'
    case 'checking':
      return 'bg-sky-100 text-sky-700 dark:bg-sky-400/15 dark:text-sky-200'
    case 'unknown':
    default:
      return 'bg-stone-100 text-stone-600 dark:bg-white/10 dark:text-stone-300'
  }
}

function stepOutcomeClass(outcome: ModelSelfCheckStepOutcome): string {
  switch (outcome) {
    case 'succeeded':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-400/15 dark:text-emerald-200'
    case 'failed':
      return 'bg-red-100 text-red-700 dark:bg-red-400/15 dark:text-red-200'
    case 'pending':
      return 'bg-sky-100 text-sky-700 dark:bg-sky-400/15 dark:text-sky-200'
    case 'not_attempted':
      return 'bg-stone-100 text-stone-600 dark:bg-white/10 dark:text-stone-300'
    case 'skipped':
    default:
      return 'bg-amber-100 text-amber-700 dark:bg-amber-400/15 dark:text-amber-200'
  }
}

function statusTextClass(status: ModelStatus): string {
  switch (status) {
    case STATUS_OPERATIONAL:
      return 'text-emerald-600 dark:text-emerald-300'
    case STATUS_DEGRADED:
      return 'text-amber-600 dark:text-amber-300'
    case STATUS_FAILED:
    case 'error':
      return 'text-red-600 dark:text-red-300'
    default:
      return 'text-stone-600 dark:text-stone-300'
  }
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}
</script>
