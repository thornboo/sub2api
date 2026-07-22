<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.modelProtocol.title')"
    width="extra-wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div class="flex flex-col gap-3 rounded-xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.03] sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p class="font-semibold text-stone-900 dark:text-stone-100">{{ account?.name }}</p>
          <p class="mt-1 max-w-3xl text-sm leading-6 text-stone-500 dark:text-stone-400">
            {{ t('admin.accounts.modelProtocol.description') }}
          </p>
        </div>
        <button
          class="inline-flex shrink-0 items-center justify-center gap-2 rounded-lg border border-emerald-200 bg-white px-3 py-2 text-sm font-medium text-emerald-700 transition hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-300 dark:hover:bg-emerald-500/15"
          :disabled="loading || syncing || saving || !account"
          @click="syncCapabilities"
        >
          <Icon name="sync" size="sm" :class="{ 'animate-spin': syncing }" />
          {{ syncing ? t('admin.accounts.modelProtocol.syncing') : t('admin.accounts.modelProtocol.sync') }}
        </button>
      </div>

      <div
        class="flex flex-col gap-3 rounded-xl border px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
        :class="
          nativeRoutingEnabled === true
            ? 'border-emerald-200 bg-emerald-50/70 dark:border-emerald-500/25 dark:bg-emerald-500/[0.08]'
            : nativeRoutingEnabled === false
              ? 'border-amber-200 bg-amber-50/80 dark:border-amber-500/25 dark:bg-amber-500/[0.08]'
              : 'border-stone-200 bg-stone-50 dark:border-white/10 dark:bg-white/[0.03]'
        "
      >
        <div class="flex min-w-0 items-start gap-3">
          <span
            class="mt-0.5 inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-full"
            :class="
              nativeRoutingEnabled === true
                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
                : nativeRoutingEnabled === false
                  ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
                  : 'bg-stone-200 text-stone-600 dark:bg-white/10 dark:text-stone-300'
            "
          >
            <Icon :name="nativeRoutingEnabled === true ? 'checkCircle' : 'infoCircle'" size="sm" />
          </span>
          <div>
            <p class="text-sm font-semibold text-stone-900 dark:text-stone-100">
              {{
                nativeRoutingEnabled === true
                  ? t('admin.accounts.modelProtocol.globalRoutingEnabled')
                  : nativeRoutingEnabled === false
                    ? t('admin.accounts.modelProtocol.globalRoutingDisabled')
                    : t('admin.accounts.modelProtocol.globalRoutingUnknown')
              }}
            </p>
            <p class="mt-0.5 text-xs leading-5 text-stone-600 dark:text-stone-400">
              {{
                nativeRoutingEnabled === true
                  ? t('admin.accounts.modelProtocol.globalRoutingEnabledHint')
                  : nativeRoutingEnabled === false
                    ? t('admin.accounts.modelProtocol.globalRoutingDisabledHint')
                    : t('admin.accounts.modelProtocol.globalRoutingUnknownHint')
              }}
            </p>
          </div>
        </div>
        <router-link
          to="/admin/settings?tab=gateway"
          class="inline-flex shrink-0 items-center gap-1 rounded-lg border border-current/15 bg-white/70 px-3 py-2 text-xs font-semibold text-stone-700 transition hover:bg-white dark:bg-black/20 dark:text-stone-200 dark:hover:bg-black/30"
        >
          {{ t('admin.accounts.modelProtocol.manageGlobalRouting') }}
          <span aria-hidden="true">→</span>
        </router-link>
      </div>

      <div class="flex flex-col gap-2 rounded-xl border border-stone-200 bg-white p-3 dark:border-white/10 dark:bg-white/[0.025] sm:flex-row sm:items-center">
        <div class="min-w-0 flex-1">
          <label for="model-protocol-manual-model" class="text-xs font-semibold uppercase tracking-wide text-stone-500 dark:text-stone-400">
            {{ t('admin.accounts.modelProtocol.addExactModel') }}
          </label>
          <input
            id="model-protocol-manual-model"
            v-model="manualModelInput"
            type="text"
            :placeholder="t('admin.accounts.modelProtocol.modelPlaceholder')"
            class="mt-1.5 w-full rounded-lg border border-stone-200 bg-white px-3 py-2 text-sm text-stone-800 outline-none transition placeholder:text-stone-400 focus:border-emerald-500 focus:ring-2 focus:ring-emerald-500/15 dark:border-white/10 dark:bg-black/30 dark:text-stone-100"
            @keyup.enter="addManualModel"
          />
        </div>
        <button
          class="mt-1 inline-flex shrink-0 items-center justify-center rounded-lg border border-stone-200 px-3 py-2 text-sm font-medium text-stone-700 transition hover:bg-stone-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-white/10 dark:text-stone-200 dark:hover:bg-white/[0.05] sm:mt-5"
          :disabled="!manualModelInput.trim()"
          @click="addManualModel"
        >
          {{ t('admin.accounts.modelProtocol.addModel') }}
        </button>
      </div>

      <div v-if="warnings.length" class="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/20">
        <p class="text-sm font-medium text-amber-800 dark:text-amber-300">{{ t('admin.accounts.modelProtocol.syncWarnings') }}</p>
        <ul class="mt-2 list-disc space-y-1 pl-5 text-sm text-amber-700 dark:text-amber-400">
          <li v-for="warning in warnings" :key="warning">{{ warning }}</li>
        </ul>
      </div>

      <div v-if="error" class="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/60 dark:bg-red-950/20 dark:text-red-300">
        {{ error }}
      </div>

      <div v-if="loading" class="flex min-h-56 items-center justify-center">
        <LoadingSpinner />
      </div>

      <div v-else class="overflow-hidden rounded-xl border border-stone-200 dark:border-white/10">
        <div class="overflow-x-auto">
          <table class="min-w-[1120px] w-full border-collapse text-left text-sm">
            <thead class="bg-stone-50 text-xs uppercase tracking-wide text-stone-500 dark:bg-white/[0.035] dark:text-stone-400">
              <tr>
                <th class="w-60 px-4 py-3 font-semibold">{{ t('admin.accounts.modelProtocol.upstreamModel') }}</th>
                <th v-for="protocol in protocols" :key="protocol.value" class="px-4 py-3 font-semibold">
                  {{ t(protocol.labelKey) }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-stone-200 bg-white dark:divide-white/10 dark:bg-white/[0.015]">
              <tr v-for="model in models" :key="model" class="align-top transition-colors hover:bg-stone-50/60 dark:hover:bg-white/[0.025]">
                <td class="px-4 py-4">
                  <div class="font-mono text-sm font-semibold text-stone-900 dark:text-stone-100">{{ model }}</div>
                  <div class="mt-1 text-xs text-stone-500 dark:text-stone-400">
                    {{ model === '*' ? t('admin.accounts.modelProtocol.defaultCapability') : t('admin.accounts.modelProtocol.exactModel') }}
                  </div>
                  <div v-if="impactsForModel(model).length" class="mt-3 space-y-1.5">
                    <div class="text-[10px] font-semibold uppercase tracking-[0.12em] text-stone-400 dark:text-stone-500">
                      {{ t('admin.accounts.modelProtocol.publicModelImpacts') }}
                    </div>
                    <div
                      v-for="impact in impactsForModel(model).slice(0, 3)"
                      :key="`${impact.channel_id}-${impact.group_id}-${impact.public_model}`"
                      class="rounded-md border border-stone-200/80 bg-stone-50 px-2 py-1.5 text-[11px] leading-4 text-stone-600 dark:border-white/10 dark:bg-black/20 dark:text-stone-300"
                    >
                      <span class="font-mono font-semibold">{{ impact.public_model }}</span>
                      <span class="block text-stone-400 dark:text-stone-500">{{ impact.channel_name }} · {{ impact.group_name }}</span>
                    </div>
                    <div v-if="impactsForModel(model).length > 3" class="text-[11px] text-stone-400">
                      {{ t('admin.accounts.modelProtocol.moreImpacts', { count: impactsForModel(model).length - 3 }) }}
                    </div>
                  </div>
                  <div
                    v-else-if="isOrphanModel(model)"
                    class="mt-3 rounded-md border border-amber-200 bg-amber-50 px-2 py-1.5 text-[11px] leading-4 text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300"
                  >
                    {{ t('admin.accounts.modelProtocol.orphanCapability') }}
                  </div>
                </td>
                <td v-for="protocol in protocols" :key="protocol.value" class="px-4 py-4">
                  <div class="space-y-2.5">
                    <div class="flex items-center justify-between gap-2">
                      <span class="text-[10px] font-semibold uppercase tracking-[0.12em] text-stone-400 dark:text-stone-500">
                        {{ t('admin.accounts.modelProtocol.effectiveState') }}
                      </span>
                      <span :class="stateBadgeClass(displayCapability(model, protocol.value).state)">
                        {{ stateLabel(displayCapability(model, protocol.value).state) }}
                      </span>
                    </div>

                    <div>
                      <div class="mb-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-stone-400 dark:text-stone-500">
                        {{ t('admin.accounts.modelProtocol.overridePolicy') }}
                      </div>
                      <div
                        class="grid grid-cols-3 gap-1 rounded-xl border border-stone-200/80 bg-stone-100/80 p-1 shadow-inner shadow-stone-950/[0.03] dark:border-white/10 dark:bg-black/30 dark:shadow-black/20"
                        role="radiogroup"
                        :aria-label="t('admin.accounts.modelProtocol.overrideLabel', { model, protocol: t(protocol.labelKey) })"
                      >
                        <button
                          v-for="option in overrideOptions"
                          :key="option.value"
                          type="button"
                          role="radio"
                          :data-override-state="option.value"
                          :aria-checked="draft[capabilityKey(model, protocol.value)] === option.value"
                          :title="t(option.labelKey)"
                          :disabled="saving || syncing"
                          :class="overrideOptionClass(option.value, draft[capabilityKey(model, protocol.value)] === option.value)"
                          @click="setOverride(model, protocol.value, option.value)"
                        >
                          <Icon :name="option.icon" size="xs" :stroke-width="2" class="shrink-0" />
                          <span class="truncate">{{ t(option.shortLabelKey) }}</span>
                        </button>
                      </div>
                    </div>

                    <div class="flex min-h-10 items-start gap-2 rounded-lg bg-stone-50/80 px-2.5 py-2 text-[11px] leading-4 text-stone-500 dark:bg-black/25 dark:text-stone-400">
                      <span :class="stateDotClass(displayCapability(model, protocol.value).state)" />
                      <span>
                        <template v-if="displayCapability(model, protocol.value).source">
                          {{ sourceLabel(displayCapability(model, protocol.value).source) }}
                          <span v-if="displayCapability(model, protocol.value).observedAt" class="block text-stone-400 dark:text-stone-500">
                            {{ formatObservedAt(displayCapability(model, protocol.value).observedAt) }}
                          </span>
                        </template>
                        <template v-else>{{ t('admin.accounts.modelProtocol.noEvidence') }}</template>
                      </span>
                    </div>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex w-full items-center justify-between gap-3">
        <p class="text-xs text-stone-500 dark:text-stone-400">{{ t('admin.accounts.modelProtocol.saveHint') }}</p>
        <div class="flex gap-2">
          <button class="rounded-lg px-4 py-2 text-sm font-medium text-stone-600 hover:bg-stone-100 dark:text-stone-300 dark:hover:bg-white/[0.06]" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button
            class="rounded-lg bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
            :disabled="loading || saving || syncing || !account"
            @click="saveOverrides"
          >
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Account } from '@/types'
import type {
  AccountPublicModelImpact,
  AccountModelProtocolCapability,
  ModelProtocol,
  ModelProtocolOverrideInput
} from '@/api/admin/accounts'

const props = defineProps<{ show: boolean; account: Account | null }>()
const emit = defineEmits<{ (event: 'close'): void }>()
const { t } = useI18n()
const appStore = useAppStore()
const nativeRoutingEnabled = ref<boolean | null>(null)

const protocols: Array<{ value: ModelProtocol; labelKey: string }> = [
  { value: 'anthropic_messages', labelKey: 'admin.accounts.modelProtocol.anthropicMessages' },
  { value: 'openai_chat_completions', labelKey: 'admin.accounts.modelProtocol.openaiChat' },
  { value: 'openai_responses', labelKey: 'admin.accounts.modelProtocol.openaiResponses' }
]

const overrideOptions: Array<{
  value: ModelProtocolOverrideInput['state']
  labelKey: string
  shortLabelKey: string
  icon: 'sparkles' | 'checkCircle' | 'xCircle'
}> = [
  {
    value: 'auto',
    labelKey: 'admin.accounts.modelProtocol.auto',
    shortLabelKey: 'admin.accounts.modelProtocol.auto',
    icon: 'sparkles'
  },
  {
    value: 'supported',
    labelKey: 'admin.accounts.modelProtocol.forceSupported',
    shortLabelKey: 'admin.accounts.modelProtocol.supportedShort',
    icon: 'checkCircle'
  },
  {
    value: 'unsupported',
    labelKey: 'admin.accounts.modelProtocol.forceUnsupported',
    shortLabelKey: 'admin.accounts.modelProtocol.unsupportedShort',
    icon: 'xCircle'
  }
]

const loading = ref(false)
const syncing = ref(false)
const saving = ref(false)
const error = ref('')
const warnings = ref<string[]>([])
const items = ref<AccountModelProtocolCapability[]>([])
const publicModelImpacts = ref<Record<string, AccountPublicModelImpact[]>>({})
const orphanUpstreamModels = ref<string[]>([])
const manualModels = ref<string[]>([])
const manualModelInput = ref('')
const draft = reactive<Record<string, ModelProtocolOverrideInput['state']>>({})
let contextGeneration = 0

const models = computed(() => {
  const values = new Set(items.value.map(item => item.upstream_model))
  manualModels.value.forEach(model => values.add(model))
  values.add('*')
  return [...values].sort((a, b) => {
    if (a === '*') return -1
    if (b === '*') return 1
    return a.localeCompare(b)
  })
})

const allPublicModelImpacts = computed(() => {
  const seen = new Set<string>()
  const result: AccountPublicModelImpact[] = []
  for (const impacts of Object.values(publicModelImpacts.value)) {
    for (const impact of impacts) {
      const key = `${impact.channel_id}\u0000${impact.group_id}\u0000${impact.public_model}`
      if (seen.has(key)) continue
      seen.add(key)
      result.push(impact)
    }
  }
  return result
})

function impactsForModel(model: string): AccountPublicModelImpact[] {
  if (model === '*') return allPublicModelImpacts.value
  return publicModelImpacts.value[model] || []
}

function isOrphanModel(model: string): boolean {
  return model !== '*' && (
    orphanUpstreamModels.value.includes(model) || manualModels.value.includes(model)
  )
}

function addManualModel() {
  const model = manualModelInput.value.trim()
  if (!model) return
  const hasControlCharacter = Array.from(model).some(character => {
    const codePoint = character.codePointAt(0) || 0
    return codePoint <= 31 || codePoint === 127
  })
  const invalid = model === '*' || model.includes('*') || Array.from(model).length > 255 || hasControlCharacter
  if (invalid) {
    error.value = t('admin.accounts.modelProtocol.invalidModel')
    return
  }
  if (models.value.includes(model)) {
    error.value = t('admin.accounts.modelProtocol.duplicateModel')
    return
  }
  manualModels.value.push(model)
  manualModelInput.value = ''
  error.value = ''
  for (const protocol of protocols) {
    draft[capabilityKey(model, protocol.value)] = 'auto'
  }
}

function capabilityKey(model: string, protocol: ModelProtocol) {
  return `${model}\u0000${protocol}`
}

function setOverride(
  model: string,
  protocol: ModelProtocol,
  state: ModelProtocolOverrideInput['state']
) {
  draft[capabilityKey(model, protocol)] = state
}

function entryFor(model: string, protocol: ModelProtocol) {
  return items.value.find(item => item.upstream_model === model && item.protocol === protocol)
}

type DisplayCapability = {
  state: AccountModelProtocolCapability['effective_state']
  source?: string
  observedAt?: string
}

function originalOverrideState(model: string, protocol: ModelProtocol) {
  return entryFor(model, protocol)?.override_state || 'auto'
}

function backendCapability(model: string, protocol: ModelProtocol): DisplayCapability {
  const exact = entryFor(model, protocol)
  const fallback = model === '*' ? undefined : entryFor('*', protocol)
  const effective = exact || fallback
  if (!effective) return { state: 'unknown' }

  let observedAt: string | undefined
  if (
    exact &&
    exact.effective_source === exact.observed_source &&
    exact.effective_state === exact.observed_state
  ) {
    observedAt = exact.observed_at
  } else if (
    fallback &&
    effective.effective_source === fallback.observed_source &&
    effective.effective_state === fallback.observed_state
  ) {
    observedAt = fallback.observed_at
  }

  return {
    state: effective.effective_state,
    source: effective.effective_source,
    observedAt
  }
}

function draftCapability(model: string, protocol: ModelProtocol): DisplayCapability {
  const exact = entryFor(model, protocol)
  const fallback = model === '*' ? undefined : entryFor('*', protocol)
  const exactDraft = draft[capabilityKey(model, protocol)] || 'auto'
  const fallbackDraft = fallback ? draft[capabilityKey('*', protocol)] || fallback.override_state : 'auto'
  if (exactDraft === 'supported' || exactDraft === 'unsupported') {
    return { state: exactDraft, source: 'admin_override' }
  }
  if (fallbackDraft === 'supported' || fallbackDraft === 'unsupported') {
    return { state: fallbackDraft, source: 'admin_override' }
  }
  if (exact?.observed_state === 'supported' || exact?.observed_state === 'unsupported') {
    return { state: exact.observed_state, source: exact.observed_source, observedAt: exact.observed_at }
  }
  if (fallback?.observed_state === 'supported' || fallback?.observed_state === 'unsupported') {
    return { state: fallback.observed_state, source: fallback.observed_source, observedAt: fallback.observed_at }
  }
  return {
    state: 'unknown',
    source: exact?.observed_source || fallback?.observed_source,
    observedAt: exact?.observed_at || fallback?.observed_at
  }
}

function displayCapability(model: string, protocol: ModelProtocol): DisplayCapability {
  const exactDraftChanged = (draft[capabilityKey(model, protocol)] || 'auto') !== originalOverrideState(model, protocol)
  const wildcardDraftChanged = model !== '*' && (
    (draft[capabilityKey('*', protocol)] || 'auto') !== originalOverrideState('*', protocol)
  )

  // The server owns the persisted resolution contract. Local resolution is
  // limited to cells affected by an unsaved exact or wildcard override.
  if (!exactDraftChanged && !wildcardDraftChanged) {
    return backendCapability(model, protocol)
  }
  return draftCapability(model, protocol)
}

function resetDraft() {
  Object.keys(draft).forEach(key => delete draft[key])
  for (const model of models.value) {
    for (const protocol of protocols) {
      draft[capabilityKey(model, protocol.value)] = entryFor(model, protocol.value)?.override_state || 'auto'
    }
  }
}

function isCurrentAccount(accountId: number, generation: number) {
  return generation === contextGeneration && props.show && props.account?.id === accountId
}

async function loadCapabilities() {
  if (!props.account) return
  const accountId = props.account.id
  const generation = contextGeneration
  loading.value = true
  error.value = ''
  items.value = []
  publicModelImpacts.value = {}
  orphanUpstreamModels.value = []
  warnings.value = []
  manualModels.value = []
  manualModelInput.value = ''
  try {
    const result = await adminAPI.accounts.getModelProtocolCapabilities(accountId)
    if (!isCurrentAccount(accountId, generation)) return
    items.value = result.items || []
    publicModelImpacts.value = result.public_model_impacts || {}
    orphanUpstreamModels.value = result.orphan_upstream_models || []
    manualModels.value = []
    manualModelInput.value = ''
    warnings.value = result.warnings || []
    resetDraft()
  } catch (requestError) {
    if (isCurrentAccount(accountId, generation)) error.value = extractApiErrorMessage(requestError)
  } finally {
    if (isCurrentAccount(accountId, generation)) loading.value = false
  }
}

async function loadNativeRoutingStatus() {
  nativeRoutingEnabled.value = null
  const settingsAPI = adminAPI.settings
  if (!settingsAPI?.getSettings) return
  try {
    const settings = await settingsAPI.getSettings()
    if (props.show) {
      nativeRoutingEnabled.value = settings.native_model_protocol_routing_enabled === true
    }
  } catch {
    // Keep status unknown: a transient settings request failure must not be
    // presented to the administrator as a confirmed disabled state.
  }
}

async function syncCapabilities() {
  if (!props.account) return
  const accountId = props.account.id
  const generation = contextGeneration
  syncing.value = true
  error.value = ''
  try {
    const result = await adminAPI.accounts.syncModelProtocolCapabilities(accountId)
    if (!isCurrentAccount(accountId, generation)) return
    items.value = result.items || []
    publicModelImpacts.value = result.public_model_impacts || {}
    orphanUpstreamModels.value = result.orphan_upstream_models || []
    warnings.value = result.warnings || []
    resetDraft()
    appStore.showSuccess(t('admin.accounts.modelProtocol.syncSuccess'))
  } catch (requestError) {
    if (isCurrentAccount(accountId, generation)) error.value = extractApiErrorMessage(requestError)
  } finally {
    if (isCurrentAccount(accountId, generation)) syncing.value = false
  }
}

async function saveOverrides() {
  if (!props.account) return
  const accountId = props.account.id
  const generation = contextGeneration
  const payload: ModelProtocolOverrideInput[] = []
  for (const model of models.value) {
    for (const protocol of protocols) {
      payload.push({
        upstream_model: model,
        protocol: protocol.value,
        state: draft[capabilityKey(model, protocol.value)] || 'auto'
      })
    }
  }
  saving.value = true
  error.value = ''
  try {
    const result = await adminAPI.accounts.updateModelProtocolCapabilityOverrides(accountId, payload)
    if (!isCurrentAccount(accountId, generation)) return
    items.value = result.items || []
    publicModelImpacts.value = result.public_model_impacts || {}
    orphanUpstreamModels.value = result.orphan_upstream_models || []
    manualModels.value = []
    warnings.value = result.warnings || []
    resetDraft()
    appStore.showSuccess(t('admin.accounts.modelProtocol.saveSuccess'))
  } catch (requestError) {
    if (isCurrentAccount(accountId, generation)) error.value = extractApiErrorMessage(requestError)
  } finally {
    if (isCurrentAccount(accountId, generation)) saving.value = false
  }
}

function stateLabel(state: string) {
  return t(`admin.accounts.modelProtocol.states.${state}`)
}

function stateBadgeClass(state: string) {
  const base = 'inline-flex rounded-full border px-2 py-0.5 text-[11px] font-semibold'
  if (state === 'supported') return `${base} border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/25 dark:bg-emerald-500/10 dark:text-emerald-300`
  if (state === 'unsupported') return `${base} border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500/25 dark:bg-rose-500/10 dark:text-rose-300`
  return `${base} border-stone-200 bg-stone-100 text-stone-600 dark:border-white/10 dark:bg-white/[0.05] dark:text-stone-400`
}

function overrideOptionClass(state: ModelProtocolOverrideInput['state'], active: boolean) {
  const base = 'inline-flex min-w-0 items-center justify-center gap-1 rounded-lg border px-2 py-1.5 text-[11px] font-semibold transition-all duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/40 disabled:cursor-not-allowed disabled:opacity-50'
  if (!active) {
    return `${base} border-transparent text-stone-500 hover:border-stone-200 hover:bg-white/80 hover:text-stone-800 dark:text-stone-500 dark:hover:border-white/10 dark:hover:bg-white/[0.06] dark:hover:text-stone-200`
  }
  if (state === 'supported') {
    return `${base} border-emerald-300 bg-emerald-50 text-emerald-700 shadow-sm dark:border-emerald-500/30 dark:bg-emerald-500/15 dark:text-emerald-300`
  }
  if (state === 'unsupported') {
    return `${base} border-rose-300 bg-rose-50 text-rose-700 shadow-sm dark:border-rose-500/30 dark:bg-rose-500/15 dark:text-rose-300`
  }
  return `${base} border-stone-200 bg-white text-stone-800 shadow-sm dark:border-white/10 dark:bg-white/[0.09] dark:text-stone-100`
}

function stateDotClass(state: string) {
  const base = 'mt-1 h-1.5 w-1.5 shrink-0 rounded-full'
  if (state === 'supported') return `${base} bg-emerald-500`
  if (state === 'unsupported') return `${base} bg-rose-500`
  return `${base} bg-stone-400 dark:bg-stone-600`
}

function sourceLabel(source?: string) {
  if (!source) return t('admin.accounts.modelProtocol.noEvidence')
  const known: Record<string, string> = {
    upstream_model_list: 'upstreamModelList',
    upstream_model_list_missing: 'upstreamDidNotDeclare',
    upstream_model_list_empty: 'upstreamEmptyDeclaration',
    upstream_unknown_values: 'upstreamUnknownDeclaration',
    legacy_migration: 'legacyMigration',
    admin_override: 'adminOverride',
    intrinsic: 'intrinsic'
  }
  const key = known[source]
  return key ? t(`admin.accounts.modelProtocol.sources.${key}`) : source
}

function formatObservedAt(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

watch(
  () => [props.show, props.account?.id] as const,
  ([visible]) => {
    contextGeneration += 1
    loading.value = false
    syncing.value = false
    saving.value = false
    if (visible) {
      void loadNativeRoutingStatus()
      void loadCapabilities()
    }
  },
  { immediate: true }
)
</script>
