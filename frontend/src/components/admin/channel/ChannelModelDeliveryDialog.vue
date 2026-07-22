<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channels.deliveryDialog.title')"
    width="wide"
    @close="emit('close')"
  >
    <div v-if="model" class="space-y-4">
      <div class="flex flex-col gap-3 rounded-xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.03] sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <div class="flex items-center gap-2">
            <PlatformIcon :platform="model.platform" size="sm" />
            <h3 class="truncate font-mono text-base font-semibold text-stone-900 dark:text-stone-100">{{ model.name }}</h3>
          </div>
          <p class="mt-1 text-sm text-stone-500 dark:text-stone-400">
            {{ t('admin.channels.deliveryDialog.summary', {
              delivered: model.deliverable_group_count,
              total: model.total_group_count,
              routes: model.route_count
            }) }}
          </p>
        </div>
        <span class="inline-flex shrink-0 rounded-full border px-3 py-1 text-xs font-semibold" :class="statusClass(model.status)">
          {{ statusLabel(model.status) }}
        </span>
      </div>

      <div>
        <div class="mb-2 text-xs font-semibold uppercase tracking-wide text-stone-500 dark:text-stone-400">
          {{ t('admin.channels.deliveryDialog.publicEndpoints') }}
        </div>
        <div v-if="model.protocols.length" class="overflow-hidden rounded-xl border border-stone-200 bg-white dark:border-white/10 dark:bg-black/20">
          <div
            v-for="protocol in model.protocols"
            :key="protocol.protocol"
            class="grid gap-2 border-b border-stone-200/80 px-3 py-2.5 last:border-b-0 dark:border-white/10 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)] sm:items-center"
          >
            <div class="flex min-w-0 items-center gap-2">
              <code class="truncate text-xs font-semibold text-stone-800 dark:text-stone-100">{{ protocol.path }}</code>
              <span class="shrink-0 rounded-full border px-2 py-0.5 text-[11px] font-semibold" :class="protocolStatusClass(protocol.status)">
                {{ protocolStatusLabel(protocol.status) }}
              </span>
            </div>
            <div v-if="protocol.status === 'available'" class="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-stone-500 dark:text-stone-400">
              <span v-if="protocol.mode" :class="modeClass(protocol.mode)">{{ modeLabel(protocol.mode) }}</span>
              <span>{{ (protocol.group_ids || []).length }} {{ t('admin.channels.deliveryDialog.groupsUnit') }}</span>
              <span v-if="protocol.upstream_protocol">
                {{ t('admin.channels.deliveryDialog.actualUpstream') }}
                <code class="text-stone-700 dark:text-stone-300">{{ protocolPath(protocol.upstream_protocol) }}</code>
              </span>
            </div>
            <div v-else class="text-[11px] leading-5 text-amber-700 dark:text-amber-300">
              {{ protocolReasonSummary(protocol.reason_codes) }}
            </div>
          </div>
        </div>
        <div v-else class="rounded-lg border border-dashed border-rose-200 bg-rose-50/60 px-3 py-3 text-sm text-rose-700 dark:border-rose-500/20 dark:bg-rose-500/[0.06] dark:text-rose-300">
          {{ t('admin.channels.deliveryDialog.noEndpoints') }}
        </div>
      </div>

      <div class="space-y-2">
        <article
          v-for="group in model.groups"
          :key="group.id"
          class="overflow-hidden rounded-xl border border-stone-200 bg-white dark:border-white/10 dark:bg-white/[0.02]"
        >
          <header class="flex items-center justify-between gap-3 border-b border-stone-200/80 px-4 py-3 dark:border-white/10">
            <div class="min-w-0">
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-semibold text-stone-900 dark:text-stone-100">{{ group.name }}</span>
                <span class="text-xs text-stone-400">#{{ group.id }}</span>
              </div>
              <p class="mt-0.5 text-xs text-stone-500 dark:text-stone-400">
                {{ t('admin.channels.deliveryDialog.groupRouteCount', { count: group.route_count }) }}
              </p>
            </div>
            <span class="rounded-full border px-2 py-0.5 text-[11px] font-semibold" :class="statusClass(group.status)">
              {{ statusLabel(group.status) }}
            </span>
          </header>

          <div v-if="group.routes.length" class="divide-y divide-stone-200/80 dark:divide-white/10">
            <div v-for="route in group.routes" :key="route.account_id" class="grid gap-3 px-4 py-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.2fr)]">
              <div class="min-w-0">
                <div class="truncate text-sm font-medium text-stone-800 dark:text-stone-200">{{ route.account_name }}</div>
                <div class="mt-0.5 text-xs text-stone-400">{{ t('admin.channels.deliveryDialog.accountId', { id: route.account_id }) }}</div>
                <div class="mt-2 grid gap-1 text-xs">
                  <div class="flex min-w-0 gap-2">
                    <span class="shrink-0 text-stone-400">{{ t('admin.channels.deliveryDialog.channelMappedModel') }}</span>
                    <code class="truncate text-stone-700 dark:text-stone-300" :title="route.channel_mapped_model">{{ route.channel_mapped_model || '—' }}</code>
                  </div>
                  <div class="flex min-w-0 gap-2">
                    <span class="shrink-0 text-stone-400">{{ t('admin.channels.deliveryDialog.upstreamModel') }}</span>
                    <code class="truncate font-semibold text-stone-800 dark:text-stone-200" :title="route.upstream_model">{{ route.upstream_model || '—' }}</code>
                  </div>
                </div>
              </div>
              <div class="grid content-start gap-1.5">
                <span
                  v-for="protocol in route.protocols"
                  :key="protocol.protocol"
                  class="flex min-w-0 items-center gap-1.5 rounded-md border border-stone-200 px-2 py-1 text-[11px] dark:border-white/10"
                >
                  <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="protocol.status === 'available' ? 'bg-emerald-500' : 'bg-amber-500'" />
                  <code class="shrink-0">{{ protocol.path }}</code>
                  <span v-if="protocol.status === 'available' && protocol.mode" :class="modeClass(protocol.mode)">{{ modeLabel(protocol.mode) }}</span>
                  <code
                    v-if="protocol.status === 'available' && protocolModelChain(protocol)"
                    class="min-w-0 truncate text-stone-500 dark:text-stone-400"
                    :title="protocolModelChain(protocol)"
                  >{{ protocolModelChain(protocol) }}</code>
                  <span v-if="protocol.status === 'blocked'" class="min-w-0 truncate text-amber-700 dark:text-amber-300" :title="protocolReasonSummary(protocol.reason_codes)">
                    {{ protocolReasonSummary(protocol.reason_codes) }}
                  </span>
                </span>
                <span v-if="!route.protocols.length" class="text-xs text-stone-400">
                  {{ t('admin.channels.deliveryDialog.noPublicEndpointOnRoute') }}
                </span>
              </div>
            </div>
          </div>
          <div v-else class="px-4 py-4 text-sm text-rose-600 dark:text-rose-300">
            {{ t('admin.channels.deliveryDialog.noStableRoute') }}
          </div>
        </article>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full justify-end">
        <button type="button" class="btn btn-secondary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type {
  ChannelModelDelivery,
  ChannelModelDeliveryProtocolDecision,
  ModelDeliveryMode,
  ModelDeliveryProtocolStatus,
  ModelDeliveryStatus
} from '@/api/admin/channels'

defineProps<{ show: boolean; model: ChannelModelDelivery | null }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

function statusLabel(status: ModelDeliveryStatus) {
  return t(`admin.channels.form.deliveryStatus.${status}`)
}

function statusClass(status: ModelDeliveryStatus) {
  if (status === 'deliverable') return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-300'
  if (status === 'partial') return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/25 dark:bg-amber-500/10 dark:text-amber-300'
  if (status === 'no_endpoint') return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/25 dark:bg-amber-500/10 dark:text-amber-300'
  return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500/25 dark:bg-rose-500/10 dark:text-rose-300'
}

function modeLabel(mode: ModelDeliveryMode) {
  return t(`admin.channels.deliveryDialog.mode.${mode}`)
}

function modeClass(mode: ModelDeliveryMode) {
  if (mode === 'native') return 'font-semibold text-emerald-600 dark:text-emerald-300'
  if (mode === 'mixed') return 'font-semibold text-amber-600 dark:text-amber-300'
  return 'font-semibold text-sky-600 dark:text-sky-300'
}

function protocolStatusLabel(status: ModelDeliveryProtocolStatus) {
  return t(`admin.channels.deliveryDialog.protocolStatus.${status}`)
}

function protocolStatusClass(status: ModelDeliveryProtocolStatus) {
  if (status === 'available') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-300'
  }
  return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/25 dark:bg-amber-500/10 dark:text-amber-300'
}

function protocolPath(protocol: string) {
  if (protocol === 'anthropic_messages') return '/v1/messages'
  if (protocol === 'openai_chat_completions') return '/v1/chat/completions'
  if (protocol === 'openai_responses') return '/v1/responses'
  return protocol
}

function protocolReasonSummary(reasonCodes: string[]) {
  if (!reasonCodes.length) return t('admin.channels.deliveryDialog.reason.unknown')
  return reasonCodes.map((reason) => t(`admin.channels.deliveryDialog.reason.${reason}`, reason)).join(' · ')
}

function protocolModelChain(protocol: ChannelModelDeliveryProtocolDecision) {
  const mapped = protocol.channel_mapped_model?.trim() || ''
  const upstream = protocol.upstream_model?.trim() || ''
  if (mapped && upstream && mapped !== upstream) return `${mapped} → ${upstream}`
  return upstream || mapped
}
</script>
