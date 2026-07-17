<template>
  <BaseDialog :show="show" :title="title" width="full" :close-on-click-outside="true" @close="close">
    <div v-if="loading" class="flex items-center justify-center py-16">
      <div class="flex flex-col items-center gap-3">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-emerald-500"></div>
        <div class="text-sm font-medium text-stone-500 dark:text-stone-400">{{ t('admin.ops.errorDetail.loading') }}</div>
      </div>
    </div>

    <div v-else-if="!detail" class="py-10 text-center text-sm text-stone-500 dark:text-stone-400">
      {{ emptyText }}
    </div>

    <div v-else class="space-y-6 p-6">
      <div v-if="(detail.classification_version ?? 0) >= 2" class="rounded-xl border border-stone-200/80 bg-stone-50/70 p-4 dark:border-white/10 dark:bg-white/[0.04]">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-xs font-black uppercase tracking-wider text-stone-700 dark:text-stone-200">{{ t('admin.ops.errorDetail.classification.title') }}</h3>
          <span class="inline-flex rounded-full px-2 py-1 text-[10px] font-bold ring-1 ring-inset" :class="slaImpactClass">
            {{ slaImpactLabel }}
          </span>
        </div>
        <div class="grid grid-cols-1 gap-3 text-xs sm:grid-cols-2 lg:grid-cols-6">
          <div>
            <div class="text-stone-400">{{ t('admin.ops.errorDetail.classification.customerVisible') }}</div>
            <div class="mt-1 font-bold text-stone-900 dark:text-white">{{ detail.customer_visible ? t('common.yes') : t('common.no') }}</div>
          </div>
          <div>
            <div class="text-stone-400">{{ t('admin.ops.errorDetail.classification.domain') }}</div>
            <div class="mt-1 font-bold text-stone-900 dark:text-white">{{ classificationLabel('domain', detail.failure_domain) }}</div>
          </div>
          <div>
            <div class="text-stone-400">{{ t('admin.ops.errorDetail.classification.category') }}</div>
            <div class="mt-1 font-bold text-stone-900 dark:text-white">{{ classificationLabel('category', detail.failure_category) }}</div>
          </div>
          <div>
            <div class="text-stone-400">{{ t('admin.ops.errorDetail.classification.reason') }}</div>
            <div class="mt-1 break-all font-mono font-bold text-stone-900 dark:text-white">{{ detail.failure_reason || '—' }}</div>
          </div>
          <div>
            <div class="text-stone-400">{{ t('admin.ops.errorDetail.classification.resolutionOwner') }}</div>
            <div class="mt-1 font-bold text-stone-900 dark:text-white">{{ classificationLabel('resolutionOwner', detail.resolution_owner) }}</div>
          </div>
          <div>
            <div class="text-stone-400">{{ t('admin.ops.errorDetail.classification.poolOwnership') }}</div>
            <div class="mt-1 font-bold text-stone-900 dark:text-white">{{ classificationLabel('poolOwnership', detail.pool_ownership) }}</div>
          </div>
        </div>
      </div>

      <!-- Summary -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.requestId') }}</div>
          <div class="mt-1 break-all font-mono text-sm font-medium text-stone-950 dark:text-white">
            {{ requestId || '—' }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.time') }}</div>
          <div class="mt-1 text-sm font-medium text-stone-950 dark:text-white">
            {{ formatDateTime(detail.created_at) }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">
            {{ isUpstreamError(detail) ? t('admin.ops.errorDetail.account') : t('admin.ops.errorDetail.user') }}
          </div>
          <div class="mt-1 text-sm font-medium text-stone-950 dark:text-white">
            <template v-if="isUpstreamError(detail)">
              {{ detail.account_name || (detail.account_id != null ? String(detail.account_id) : '—') }}
            </template>
            <template v-else>
              {{ detail.user_email || (detail.user_id != null ? String(detail.user_id) : '—') }}
            </template>
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.platform') }}</div>
          <div class="mt-1 text-sm font-medium text-stone-950 dark:text-white">
            {{ detail.platform || '—' }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.group') }}</div>
          <div class="mt-1 text-sm font-medium text-stone-950 dark:text-white">
            {{ detail.group_name || (detail.group_id != null ? String(detail.group_id) : '—') }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.model') }}</div>
          <div class="mt-1 text-sm font-medium text-stone-950 dark:text-white">
            <template v-if="hasModelMapping(detail)">
              <span class="font-mono">{{ detail.requested_model }}</span>
              <span class="mx-1 text-stone-400">→</span>
              <span class="font-mono text-emerald-600 dark:text-emerald-300">{{ detail.upstream_model }}</span>
            </template>
            <template v-else>
              {{ displayModel(detail) || '—' }}
            </template>
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.inboundEndpoint') }}</div>
          <div class="mt-1 break-all font-mono text-sm font-medium text-stone-950 dark:text-white">
            {{ detail.inbound_endpoint || '—' }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.upstreamEndpoint') }}</div>
          <div class="mt-1 break-all font-mono text-sm font-medium text-stone-950 dark:text-white">
            {{ detail.upstream_endpoint || '—' }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.status') }}</div>
          <div class="mt-1">
            <span :class="['inline-flex items-center rounded-lg px-2 py-1 text-xs font-black ring-1 ring-inset shadow-sm', statusClass]">
              {{ detail.status_code }}
            </span>
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.requestType') }}</div>
          <div class="mt-1 text-sm font-medium text-stone-950 dark:text-white">
            {{ formatRequestTypeLabel(detail.request_type) }}
          </div>
        </div>

        <div class="rounded-xl bg-stone-50/80 p-4 dark:bg-white/[0.04]">
          <div class="text-xs font-bold uppercase tracking-wider text-stone-400">{{ t('admin.ops.errorDetail.message') }}</div>
          <div class="mt-1 truncate text-sm font-medium text-stone-950 dark:text-white" :title="detail.message">
            {{ detail.message || '—' }}
          </div>
        </div>

        <div v-if="detail.api_key_prefix" class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.errorDetail.apiKeyPrefix') }}</div>
          <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white">
            {{ detail.api_key_prefix }}
          </div>
        </div>

        <div v-if="detail.attempted_key_prefix" class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.errorDetail.attemptedKeyPrefix') }}</div>
          <div class="mt-1 font-mono text-sm font-medium text-gray-900 dark:text-white">
            {{ detail.attempted_key_prefix }}
          </div>
        </div>

        <div v-if="detail.deleted_key_owner_email" class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.errorDetail.deletedKeyOwner') }}</div>
          <div class="mt-1 text-sm font-medium text-gray-900 dark:text-white">
            {{ detail.deleted_key_owner_email }}
            <span v-if="detail.deleted_key_name" class="ml-1 text-xs text-gray-500 dark:text-gray-400">({{ detail.deleted_key_name }})</span>
            <span class="ml-2 inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold ring-1 ring-inset bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-400 dark:ring-red-500/30">
              {{ t('admin.ops.errorDetail.keyDeletedBadge') }}
            </span>
          </div>
        </div>
      </div>

      <!-- Response content (client request -> error_body; upstream -> upstream_error_detail/message) -->
      <div class="rounded-xl bg-stone-50/80 p-6 dark:bg-white/[0.04]">
        <h3 class="text-sm font-black uppercase tracking-wider text-stone-950 dark:text-white">{{ t('admin.ops.errorDetail.responseBody') }}</h3>
        <pre class="ops-response-block mt-4 max-h-[520px] overflow-y-auto rounded-xl border border-stone-200/80 bg-white/80 p-4 text-xs text-stone-800 dark:border-white/10 dark:bg-neutral-950/60 dark:text-stone-100"><code>{{ prettyJSON(primaryResponseBody || '') }}</code></pre>
      </div>

      <!-- Upstream errors list (only for request errors) -->
      <div v-if="showUpstreamList" class="rounded-xl bg-stone-50/80 p-6 dark:bg-white/[0.04]">
        <div class="flex flex-wrap items-center justify-between gap-2">
          <h3 class="text-sm font-black uppercase tracking-wider text-stone-950 dark:text-white">{{ t('admin.ops.errorDetails.upstreamErrors') }}</h3>
          <div class="text-xs text-stone-500 dark:text-stone-400" v-if="correlatedUpstreamLoading">{{ t('common.loading') }}</div>
        </div>

        <div v-if="!correlatedUpstreamLoading && !correlatedUpstreamErrors.length" class="mt-3 text-sm text-stone-500 dark:text-stone-400">
          {{ t('common.noData') }}
        </div>

        <div v-else class="mt-4 space-y-3">
          <div
            v-for="(ev, idx) in correlatedUpstreamErrors"
            :key="ev.id"
            class="rounded-xl border border-stone-200/80 bg-white/80 p-4 dark:border-white/10 dark:bg-neutral-950/60"
          >
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="text-xs font-black text-stone-950 dark:text-white">
                #{{ idx + 1 }}
                <span v-if="ev.type" class="ml-2 rounded-md bg-stone-100 px-2 py-0.5 font-mono text-[10px] font-bold text-stone-700 dark:bg-white/[0.08] dark:text-stone-200">{{ ev.type }}</span>
              </div>
              <div class="flex items-center gap-2">
                <div class="font-mono text-xs text-stone-500 dark:text-stone-400">
                  {{ ev.status_code ?? '—' }}
                </div>
                <button
                  type="button"
                  class="inline-flex items-center gap-1.5 rounded-md px-1.5 py-1 text-[10px] font-bold text-emerald-700 hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:text-emerald-200 dark:hover:bg-emerald-500/15"
                  :disabled="!getUpstreamResponsePreview(ev)"
                  :title="getUpstreamResponsePreview(ev) ? '' : t('common.noData')"
                  @click="toggleUpstreamDetail(ev.id)"
                >
                  <Icon
                    :name="expandedUpstreamDetailIds.has(ev.id) ? 'chevronDown' : 'chevronRight'"
                    size="xs"
                    :stroke-width="2"
                  />
                  <span>
                    {{
                      expandedUpstreamDetailIds.has(ev.id)
                        ? t('admin.ops.errorDetail.responsePreview.collapse')
                        : t('admin.ops.errorDetail.responsePreview.expand')
                    }}
                  </span>
                </button>
              </div>
            </div>

            <div class="mt-3 grid grid-cols-1 gap-2 text-xs text-stone-600 dark:text-stone-300 sm:grid-cols-2">
              <div>
                <span class="text-stone-400">{{ t('admin.ops.errorDetail.upstreamEvent.status') }}:</span>
                <span class="ml-1 font-mono">{{ ev.status_code ?? '—' }}</span>
              </div>
              <div>
                <span class="text-stone-400">{{ t('admin.ops.errorDetail.upstreamEvent.requestId') }}:</span>
                <span class="ml-1 font-mono">{{ ev.request_id || ev.client_request_id || '—' }}</span>
              </div>
            </div>

            <div v-if="ev.message" class="mt-3 break-words text-sm font-medium text-stone-950 dark:text-white">{{ ev.message }}</div>

            <pre
              v-if="expandedUpstreamDetailIds.has(ev.id)"
              class="ops-response-block mt-3 max-h-[240px] overflow-y-auto rounded-xl border border-stone-200/80 bg-stone-50/80 p-3 text-xs text-stone-800 dark:border-white/10 dark:bg-black/35 dark:text-stone-100"
            ><code>{{ prettyJSON(getUpstreamResponsePreview(ev)) }}</code></pre>
          </div>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { opsAPI, type OpsErrorDetail } from '@/api/admin/ops'
import { formatDateTime } from '@/utils/format'
import { resolvePrimaryResponseBody, resolveUpstreamPayload } from '../utils/errorDetailResponse'

interface Props {
  show: boolean
  errorId: number | null
  errorType?: 'request' | 'upstream'
}

interface Emits {
  (e: 'update:show', value: boolean): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const detail = ref<OpsErrorDetail | null>(null)

const showUpstreamList = computed(() => props.errorType === 'request')

const requestId = computed(() => detail.value?.request_id || detail.value?.client_request_id || '')

const primaryResponseBody = computed(() => {
  return resolvePrimaryResponseBody(detail.value, props.errorType)
})

function classificationLabel(group: 'domain' | 'category' | 'resolutionOwner' | 'poolOwnership', value?: string): string {
  const normalized = String(value || 'unknown').trim() || 'unknown'
  return t(`admin.ops.errorDetails.${group}.${normalized}`)
}

const slaImpactLabel = computed(() => {
  if (detail.value?.sla_impact === true) return t('admin.ops.errorDetails.slaImpact.included')
  if (detail.value?.sla_impact === false) return t('admin.ops.errorDetails.slaImpact.excluded')
  return t('admin.ops.errorDetails.slaImpact.unknown')
})

const slaImpactClass = computed(() => {
  if (detail.value?.sla_impact === true) return 'bg-red-500/10 text-red-700 ring-red-500/20 dark:text-red-300'
  if (detail.value?.sla_impact === false) return 'bg-stone-500/10 text-stone-700 ring-stone-500/20 dark:text-stone-300'
  return 'bg-amber-500/10 text-amber-700 ring-amber-500/20 dark:text-amber-300'
})




const title = computed(() => {
  if (!props.errorId) return t('admin.ops.errorDetail.title')
  return t('admin.ops.errorDetail.titleWithId', { id: String(props.errorId) })
})

const emptyText = computed(() => t('admin.ops.errorDetail.noErrorSelected'))

function isUpstreamError(d: OpsErrorDetail | null): boolean {
  if (!d) return false
  const phase = String(d.phase || '').toLowerCase()
  const owner = String(d.error_owner || '').toLowerCase()
  return phase === 'upstream' && owner === 'provider'
}

function formatRequestTypeLabel(type: number | null | undefined): string {
  switch (type) {
    case 1: return t('admin.ops.errorDetail.requestTypeSync')
    case 2: return t('admin.ops.errorDetail.requestTypeStream')
    case 3: return t('admin.ops.errorDetail.requestTypeWs')
    default: return t('admin.ops.errorDetail.requestTypeUnknown')
  }
}

function hasModelMapping(d: OpsErrorDetail | null): boolean {
  if (!d) return false
  const requested = String(d.requested_model || '').trim()
  const upstream = String(d.upstream_model || '').trim()
  return !!requested && !!upstream && requested !== upstream
}

function displayModel(d: OpsErrorDetail | null): string {
  if (!d) return ''
  const upstream = String(d.upstream_model || '').trim()
  if (upstream) return upstream
  const requested = String(d.requested_model || '').trim()
  if (requested) return requested
  return String(d.model || '').trim()
}

const correlatedUpstream = ref<OpsErrorDetail[]>([])
const correlatedUpstreamLoading = ref(false)

const correlatedUpstreamErrors = computed<OpsErrorDetail[]>(() => correlatedUpstream.value)

const expandedUpstreamDetailIds = ref(new Set<number>())

function getUpstreamResponsePreview(ev: OpsErrorDetail): string {
  const upstreamPayload = resolveUpstreamPayload(ev)
  if (upstreamPayload) return upstreamPayload
  return String(ev.error_body || '').trim()
}

function toggleUpstreamDetail(id: number) {
  const next = new Set(expandedUpstreamDetailIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  expandedUpstreamDetailIds.value = next
}

async function fetchCorrelatedUpstreamErrors(requestErrorId: number) {
  correlatedUpstreamLoading.value = true
  try {
    const res = await opsAPI.listRequestErrorUpstreamErrors(
      requestErrorId,
      { page: 1, page_size: 100, view: 'all' },
      { include_detail: true }
    )
    correlatedUpstream.value = res.items || []
  } catch (err) {
    console.error('[OpsErrorDetailModal] Failed to load correlated upstream errors', err)
    correlatedUpstream.value = []
  } finally {
    correlatedUpstreamLoading.value = false
  }
}

function close() {
  emit('update:show', false)
}

function prettyJSON(raw?: string): string {
  if (!raw) return 'N/A'
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

async function fetchDetail(id: number) {
  loading.value = true
  try {
    const kind = props.errorType || (detail.value?.phase === 'upstream' ? 'upstream' : 'request')
    const d = kind === 'upstream' ? await opsAPI.getUpstreamErrorDetail(id) : await opsAPI.getRequestErrorDetail(id)
    detail.value = d
  } catch (err: any) {
    detail.value = null
    appStore.showError(err?.message || t('admin.ops.failedToLoadErrorDetail'))
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.show, props.errorId] as const,
  ([show, id]) => {
    if (!show) {
      detail.value = null
      return
    }
    if (typeof id === 'number' && id > 0) {
      expandedUpstreamDetailIds.value = new Set()
      fetchDetail(id)
      if (props.errorType === 'request') {
        fetchCorrelatedUpstreamErrors(id)
      } else {
        correlatedUpstream.value = []
      }
    }
  },
  { immediate: true }
)

const statusClass = computed(() => {
  const code = detail.value?.status_code ?? 0
  if (code >= 500) return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-400 dark:ring-red-500/30'
  if (code === 429) return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-300 dark:ring-amber-500/30'
  if (code >= 400) return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-400 dark:ring-amber-500/30'
  return 'bg-stone-50 text-stone-700 ring-stone-600/20 dark:bg-white/[0.08] dark:text-stone-300 dark:ring-white/10'
})

</script>

<style scoped>
.ops-response-block {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
</style>
