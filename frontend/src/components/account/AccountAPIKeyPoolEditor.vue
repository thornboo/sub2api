<template>
  <div class="border-t border-stone-200/80 pt-4 dark:border-white/10">
    <div class="mb-3 flex items-center justify-between gap-3">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.keyPool.title') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.keyPool.hint') }}
        </p>
      </div>
      <button
        type="button"
        class="rounded-lg border border-emerald-200 px-3 py-1.5 text-sm text-emerald-600 hover:bg-emerald-50 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-900/30"
        @click="addKey"
      >
        {{ t('admin.accounts.keyPool.add') }}
      </button>
    </div>

    <div v-if="keys.length === 0" class="rounded-lg bg-stone-50 p-3 text-xs text-gray-500 dark:bg-white/[0.04] dark:text-gray-400">
      {{ t('admin.accounts.keyPool.empty') }}
    </div>

    <div v-else class="space-y-3">
      <div
        v-for="(key, index) in keys"
        :key="key.id ?? index"
        class="rounded-lg border border-stone-200 p-3 dark:border-white/10"
      >
        <div class="grid grid-cols-1 gap-4 xl:grid-cols-[minmax(300px,0.38fr)_minmax(0,0.62fr)]">
          <div class="space-y-3">
            <div>
              <label class="input-label text-xs">{{ t('admin.accounts.keyPool.noteLabel') }}</label>
              <input
                :value="key.name"
                type="text"
                required
                class="input"
                :placeholder="t('admin.accounts.keyPool.namePlaceholder')"
                @input="updateKey(index, { name: ($event.target as HTMLInputElement).value })"
              />
            </div>
            <div>
              <label class="input-label text-xs">{{ t('admin.accounts.keyPool.apiKeyLabel') }}</label>
              <input
                :value="key.api_key || ''"
                type="password"
                class="input font-mono"
                autocomplete="new-password"
                data-1p-ignore
                data-lpignore="true"
                data-bwignore="true"
                :placeholder="isExistingKey(key) ? t('admin.accounts.keyPool.apiKeyPlaceholderEdit') : t('admin.accounts.keyPool.apiKeyPlaceholder')"
                @input="updateKey(index, { api_key: ($event.target as HTMLInputElement).value })"
              />
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-[1fr_1.2fr]">
              <div>
                <label class="input-label text-xs">{{ t('admin.accounts.keyPool.priorityLabel') }}</label>
                <input
                  :value="key.priority"
                  type="number"
                  min="0"
                  step="1"
                  class="input"
                  :title="t('admin.accounts.keyPool.priorityHint')"
                  @input="updateKey(index, { priority: Number(($event.target as HTMLInputElement).value) })"
                />
              </div>
              <div>
                <label class="input-label text-xs">{{ t('admin.accounts.keyPool.statusLabel') }}</label>
                <Select
                  class="w-full"
                  :model-value="key.status"
                  :options="statusOptions"
                  :searchable="false"
                  @update:model-value="updateStatus(index, $event)"
                />
              </div>
            </div>
            <p class="text-[11px] leading-4 text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.keyPool.priorityHint') }}
            </p>
            <div v-if="key.id" class="flex flex-wrap gap-3 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.accounts.keyPool.requests', { count: key.recent_request_count || 0 }) }}</span>
              <span>{{ t('admin.accounts.keyPool.errors', { count: key.recent_error_count || 0 }) }}</span>
              <span v-if="key.last_used_at">{{ t('admin.accounts.keyPool.lastUsedAt', { time: key.last_used_at }) }}</span>
            </div>
            <button
              type="button"
              class="inline-flex items-center gap-1 rounded-lg px-2 py-1.5 text-sm text-red-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
              @click="removeKey(index)"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
              {{ t('common.delete') }}
            </button>
          </div>

          <div class="rounded-lg bg-stone-50 p-3 dark:bg-white/[0.04]">
            <div class="mb-3">
              <label class="input-label text-xs">{{ t('admin.accounts.keyPool.modelRestrictionTitle') }}</label>
              <div class="grid grid-cols-2 gap-2">
                <button
                  type="button"
                  @click="updateModelRestrictionMode(index, 'whitelist')"
                  :class="[
                    'rounded-lg px-3 py-2 text-sm font-medium transition-all',
                    modelModeFor(key) === 'whitelist'
                      ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300'
                      : 'bg-white text-gray-600 hover:bg-gray-100 dark:bg-white/[0.06] dark:text-gray-400 dark:hover:bg-white/10'
                  ]"
                >
                  {{ t('admin.accounts.modelWhitelist') }}
                </button>
                <button
                  type="button"
                  @click="updateModelRestrictionMode(index, 'mapping')"
                  :class="[
                    'rounded-lg px-3 py-2 text-sm font-medium transition-all',
                    modelModeFor(key) === 'mapping'
                      ? 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300'
                      : 'bg-white text-gray-600 hover:bg-gray-100 dark:bg-white/[0.06] dark:text-gray-400 dark:hover:bg-white/10'
                  ]"
                >
                  {{ t('admin.accounts.modelMapping') }}
                </button>
              </div>
              <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.accounts.keyPool.modelRestrictionHint') }}
              </p>
            </div>

            <div v-if="modelModeFor(key) === 'whitelist'" class="space-y-3">
              <ModelWhitelistSelector
                :model-value="whitelistModels(key)"
                :platform="platform"
                :can-probe-models="true"
                :probe-models-loading="isProbing(index)"
                :probe-new-models="newWhitelistModelsFor(index)"
                :probe-missing-models="missingWhitelistModelsFor(index)"
                @update:model-value="updateWhitelistModels(index, $event)"
                @probe-models="probeKeyModels(index)"
              />
            </div>

            <div v-else class="space-y-3">
              <div
                v-for="(row, mappingIndex) in modelMappingRows(index, key)"
                :key="mappingIndex"
                class="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto] items-center gap-2"
              >
                <input
                  :value="row.from"
                  type="text"
                  class="input font-mono text-sm"
                  :placeholder="t('admin.accounts.requestModel')"
                  @input="updateMappingEntry(index, mappingIndex, 'from', ($event.target as HTMLInputElement).value)"
                />
                <span class="text-gray-400">→</span>
                <div class="flex min-w-0 items-center gap-2">
                  <input
                    :value="row.to"
                    type="text"
                    class="input min-w-0 flex-1 font-mono text-sm"
                    :placeholder="t('admin.accounts.actualModel')"
                    @input="updateMappingEntry(index, mappingIndex, 'to', ($event.target as HTMLInputElement).value)"
                  />
                  <span
                    v-if="newMappingTargetsFor(index).includes(row.to.trim())"
                    class="shrink-0 rounded bg-emerald-100 px-2 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300"
                  >
                    {{ t('admin.accounts.probeModelNew') }}
                  </span>
                  <span
                    v-else-if="missingMappingTargetsFor(index).includes(row.to.trim())"
                    class="shrink-0 rounded bg-amber-100 px-2 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
                  >
                    {{ t('admin.accounts.probeModelMissing') }}
                  </span>
                </div>
                <button
                  type="button"
                  class="rounded-lg p-2 text-red-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20"
                  :title="t('common.delete')"
                  @click="removeMappingEntry(index, mappingIndex)"
                >
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      stroke-linecap="round"
                      stroke-linejoin="round"
                      stroke-width="2"
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </div>
              <div class="flex flex-wrap gap-2">
                <button
                  type="button"
                  class="rounded-lg border border-emerald-200 px-3 py-1.5 text-sm text-emerald-600 hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-900/30"
                  :disabled="isProbing(index)"
                  @click="probeKeyModels(index)"
                >
                  {{ isProbing(index) ? t('admin.accounts.probingSupportedModels') : t('admin.accounts.probeSupportedModels') }}
                </button>
                <button
                  type="button"
                  class="rounded-lg border border-purple-200 px-3 py-1.5 text-sm text-purple-600 hover:bg-purple-50 dark:border-purple-800 dark:text-purple-300 dark:hover:bg-purple-900/30"
                  @click="addMappingEntry(index)"
                >
                  {{ t('admin.accounts.addMapping') }}
                </button>
                <button
                  type="button"
                  class="rounded-lg border border-red-200 px-3 py-1.5 text-sm text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/30"
                  @click="clearKeyModels(index)"
                >
                  {{ t('admin.accounts.clearAllModels') }}
                </button>
              </div>
              <ModelCatalogSearch
                :model-value="catalogModelFor(index)"
                :label="t('admin.accounts.customModelName')"
                :placeholder="t('admin.accounts.enterCustomModelName')"
                :add-label="t('admin.accounts.addModel')"
                @update:model-value="setCatalogModel(index, $event)"
                @add="addCatalogMapping(index, $event)"
              />
            </div>
          </div>
        </div>
        <div v-if="key.model_cooldowns && Object.keys(key.model_cooldowns).length > 0" class="mt-2 flex flex-wrap gap-2">
          <span
            v-for="cooldown in Object.values(key.model_cooldowns)"
            :key="cooldown.upstream_model"
            class="rounded bg-amber-100 px-2 py-1 text-xs text-amber-700 dark:bg-amber-900/40 dark:text-amber-300"
          >
            {{ cooldown.upstream_model }} {{ t('admin.accounts.keyPool.cooling') }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import ModelCatalogSearch from '@/components/account/ModelCatalogSearch.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AccountPlatform, AccountUpstreamAPIKey } from '@/types'

interface MappingRow {
  from: string
  to: string
}

const props = defineProps<{
  modelValue: AccountUpstreamAPIKey[]
  platform: AccountPlatform
  baseUrl: string
  accountId?: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: AccountUpstreamAPIKey[]]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const rowTokens = ref<string[]>([])
const mappingDrafts = ref<Record<string, MappingRow[]>>({})
const catalogDrafts = ref<Record<string, string>>({})
const probingTokens = ref<Set<string>>(new Set())
const newWhitelistModelsByToken = ref<Record<string, string[]>>({})
const missingWhitelistModelsByToken = ref<Record<string, string[]>>({})
const newMappingTargetsByToken = ref<Record<string, string[]>>({})
const missingMappingTargetsByToken = ref<Record<string, string[]>>({})
let nextRowToken = 1

const keys = computed({
  get: () => props.modelValue,
  set: (value: AccountUpstreamAPIKey[]) => emit('update:modelValue', value)
})

const rowsFromMapping = (mapping?: Record<string, string>): MappingRow[] =>
  Object.entries(mapping || {}).map(([from, to]) => ({ from, to }))

const keyTokenFor = (index: number) => rowTokens.value[index] || `fallback-${index}`

const isExistingKey = (key: AccountUpstreamAPIKey) => Number(key.id || 0) > 0

const isProbing = (index: number) => probingTokens.value.has(keyTokenFor(index))

const retainByActiveTokens = (source: Record<string, string[]>, active: Set<string>) => {
  const retained: Record<string, string[]> = {}
  for (const [token, values] of Object.entries(source)) {
    if (active.has(token)) retained[token] = values
  }
  return retained
}

const uniqueModels = (models: string[]) => Array.from(new Set(models.map(model => model.trim()).filter(Boolean)))

const getProbeModelsErrorMessage = (error: unknown) => {
  const normalized = error as { status?: unknown; message?: unknown } | undefined
  const status = typeof normalized?.status === 'number' ? normalized.status : undefined
  const message = typeof normalized?.message === 'string' ? normalized.message.trim() : ''
  if (status === 404) {
    return t('admin.accounts.probeModelsEndpointMissing')
  }
  return message || t('admin.accounts.probeModelsFailed')
}

const newWhitelistModelsFor = (index: number) => newWhitelistModelsByToken.value[keyTokenFor(index)] || []

const missingWhitelistModelsFor = (index: number) => missingWhitelistModelsByToken.value[keyTokenFor(index)] || []

const newMappingTargetsFor = (index: number) => newMappingTargetsByToken.value[keyTokenFor(index)] || []

const missingMappingTargetsFor = (index: number) => missingMappingTargetsByToken.value[keyTokenFor(index)] || []

const setTokenList = (target: typeof newWhitelistModelsByToken, token: string, values: string[]) => {
  const normalized = uniqueModels(values)
  target.value = {
    ...target.value,
    [token]: normalized
  }
}

const appendTokenList = (target: typeof newWhitelistModelsByToken, token: string, values: string[]) => {
  setTokenList(target, token, [...(target.value[token] || []), ...values])
}

const clearTokenModelMarkers = (token: string) => {
  const { [token]: _newWhitelist, ...restNewWhitelist } = newWhitelistModelsByToken.value
  const { [token]: _missingWhitelist, ...restMissingWhitelist } = missingWhitelistModelsByToken.value
  const { [token]: _newMapping, ...restNewMapping } = newMappingTargetsByToken.value
  const { [token]: _missingMapping, ...restMissingMapping } = missingMappingTargetsByToken.value
  newWhitelistModelsByToken.value = restNewWhitelist
  missingWhitelistModelsByToken.value = restMissingWhitelist
  newMappingTargetsByToken.value = restNewMapping
  missingMappingTargetsByToken.value = restMissingMapping
}

const setProbing = (index: number, probing: boolean) => {
  const token = keyTokenFor(index)
  const next = new Set(probingTokens.value)
  if (probing) next.add(token)
  else next.delete(token)
  probingTokens.value = next
}

const syncRowTokens = () => {
  const previous = rowTokens.value
  const next = props.modelValue.map((key, index) => {
    if (key.id) return `id-${key.id}`
    return previous[index] || `new-${nextRowToken++}`
  })
  rowTokens.value = next
  const active = new Set(next)
  const retainedDrafts: Record<string, MappingRow[]> = {}
  for (const [token, rows] of Object.entries(mappingDrafts.value)) {
    if (active.has(token)) retainedDrafts[token] = rows
  }
  mappingDrafts.value = retainedDrafts
  const retainedCatalogDrafts: Record<string, string> = {}
  for (const [token, value] of Object.entries(catalogDrafts.value)) {
    if (active.has(token)) retainedCatalogDrafts[token] = value
  }
  catalogDrafts.value = retainedCatalogDrafts
  newWhitelistModelsByToken.value = retainByActiveTokens(newWhitelistModelsByToken.value, active)
  missingWhitelistModelsByToken.value = retainByActiveTokens(missingWhitelistModelsByToken.value, active)
  newMappingTargetsByToken.value = retainByActiveTokens(newMappingTargetsByToken.value, active)
  missingMappingTargetsByToken.value = retainByActiveTokens(missingMappingTargetsByToken.value, active)
}

watch(
  () => props.modelValue.map(key => `${key.id || 'new'}:${key.model_restriction_mode || ''}`).join('|'),
  syncRowTokens,
  { immediate: true }
)

const addKey = () => {
  rowTokens.value = [...rowTokens.value, `new-${nextRowToken++}`]
  keys.value = [
    ...keys.value,
    {
      name: '',
      api_key: '',
      priority: 1,
      status: 'active',
      model_restriction_mode: 'whitelist',
      model_mapping: {}
    }
  ]
}

const updateKey = (index: number, patch: Partial<AccountUpstreamAPIKey>) => {
  keys.value = keys.value.map((key, i) => i === index ? { ...key, ...patch } : key)
}

const statusOptions = computed(() => [
  { value: 'active', label: t('admin.accounts.keyPool.statusActive') },
  { value: 'inactive', label: t('admin.accounts.keyPool.statusInactive') }
])

const updateStatus = (index: number, status: string | number | boolean | null) => {
  const normalized = status === 'inactive' ? status : 'active'
  updateKey(index, { status: normalized })
}

const modelModeFor = (key: AccountUpstreamAPIKey): 'whitelist' | 'mapping' =>
  key.model_restriction_mode === 'mapping' ? 'mapping' : 'whitelist'

const updateModelRestrictionMode = (index: number, mode: 'whitelist' | 'mapping') => {
  if (mode !== 'mapping') {
    const token = keyTokenFor(index)
    const { [token]: _removed, ...rest } = mappingDrafts.value
    mappingDrafts.value = rest
  }
  updateKey(index, {
    model_restriction_mode: mode,
    model_mapping: keys.value[index]?.model_mapping || {}
  })
}

const whitelistModels = (key: AccountUpstreamAPIKey) => Object.keys(key.model_mapping || {})

const updateWhitelistModels = (index: number, models: string[]) => {
  const before = new Set(whitelistModels(keys.value[index] || {}))
  const nextModels = uniqueModels(models)
  const mapping: Record<string, string> = {}
  for (const model of nextModels) {
    mapping[model] = model
  }
  updateKey(index, { model_restriction_mode: 'whitelist', model_mapping: mapping })
  const added = nextModels.filter(model => !before.has(model))
  if (added.length > 0) {
    const token = keyTokenFor(index)
    appendTokenList(newWhitelistModelsByToken, token, added)
  }
}

const mappingFromRows = (rows: MappingRow[]) => {
  const mapping: Record<string, string> = {}
  for (const row of rows) {
    const cleanFrom = row.from.trim()
    const cleanTo = row.to.trim()
    if (cleanFrom && cleanTo) {
      mapping[cleanFrom] = cleanTo
    }
  }
  return mapping
}

const modelMappingRows = (index: number, key: AccountUpstreamAPIKey): MappingRow[] => {
  const token = keyTokenFor(index)
  if (!mappingDrafts.value[token]) {
    mappingDrafts.value = {
      ...mappingDrafts.value,
      [token]: rowsFromMapping(key.model_mapping)
    }
  }
  return mappingDrafts.value[token]
}

const setMappingRows = (index: number, rows: MappingRow[]) => {
  const token = keyTokenFor(index)
  mappingDrafts.value = {
    ...mappingDrafts.value,
    [token]: rows
  }
  updateKey(index, { model_mapping: mappingFromRows(rows) })
}

const updateMappingEntry = (index: number, mappingIndex: number, side: 'from' | 'to', value: string) => {
  const rows = [...modelMappingRows(index, keys.value[index] || {})]
  if (!rows[mappingIndex]) return
  const normalizedTarget = side === 'to' ? value.trim() : ''
  const existingTargets = new Set(
    rows
      .filter((_, i) => i !== mappingIndex)
      .map(row => row.to.trim())
      .filter(Boolean)
  )
  rows[mappingIndex] = side === 'from' ? { ...rows[mappingIndex], from: value } : { ...rows[mappingIndex], to: value }
  setMappingRows(index, rows)
  if (normalizedTarget && !existingTargets.has(normalizedTarget)) {
    appendTokenList(newMappingTargetsByToken, keyTokenFor(index), [normalizedTarget])
  }
}

const addMappingEntry = (index: number) => {
  const rows = [...modelMappingRows(index, keys.value[index] || {}), { from: '', to: '' }]
  const token = keyTokenFor(index)
  mappingDrafts.value = {
    ...mappingDrafts.value,
    [token]: rows
  }
}

const catalogModelFor = (index: number) => catalogDrafts.value[keyTokenFor(index)] || ''

const setCatalogModel = (index: number, value: string) => {
  const token = keyTokenFor(index)
  catalogDrafts.value = {
    ...catalogDrafts.value,
    [token]: value
  }
}

const addWhitelistModels = (index: number, models: string[]) => {
  const key = keys.value[index]
  const mapping = { ...(key?.model_mapping || {}) }
  const fetched = uniqueModels(models)
  const addedModels: string[] = []
  let added = 0
  let existing = 0
  for (const model of fetched) {
    if (mapping[model]) existing += 1
    else added += 1
    if (!mapping[model]) addedModels.push(model)
    mapping[model] = model
  }
  updateKey(index, { model_restriction_mode: 'whitelist', model_mapping: mapping })
  const token = keyTokenFor(index)
  setTokenList(newWhitelistModelsByToken, token, addedModels)
  const missingModels = Object.keys(mapping).filter(model => !fetched.includes(model))
  setTokenList(missingWhitelistModelsByToken, token, missingModels)
  return { added, existing, missing: missingModels.length }
}

const addCatalogMapping = (index: number, selectedModel?: string) => {
  const model = (selectedModel || catalogModelFor(index)).trim()
  if (!model) return
  addCatalogMappings(index, [model])
  setCatalogModel(index, '')
}

const addCatalogMappings = (index: number, models: string[]) => {
  const rows = [...modelMappingRows(index, keys.value[index] || {})]
  const existingSources = new Set(rows.map(row => row.from.trim()).filter(Boolean))
  const existingTargets = new Set(rows.map(row => row.to.trim()).filter(Boolean))
  const nextRows = [...rows]
  const addedTargets: string[] = []
  let added = 0
  let existing = 0
  for (const model of uniqueModels(models)) {
    if (existingTargets.has(model) || existingSources.has(model)) {
      existing += 1
      continue
    }
    existingSources.add(model)
    existingTargets.add(model)
    nextRows.push({ from: model, to: model })
    addedTargets.push(model)
    added += 1
  }
  setMappingRows(index, nextRows)
  const token = keyTokenFor(index)
  appendTokenList(newMappingTargetsByToken, token, addedTargets)
  return { added, existing, missing: 0 }
}

const probeKeyModels = async (index: number) => {
  const key = keys.value[index]
  const baseUrl = props.baseUrl.trim()
  const apiKey = (key?.api_key || '').trim()
  const keyID = Number(key?.id || 0)
  const accountID = Number(props.accountId || 0)
  if (!baseUrl) {
    appStore.showError(t('admin.accounts.probeModelsMissingBaseUrl'))
    return
  }
  if (!apiKey && (!accountID || !keyID)) {
    appStore.showError(t('admin.accounts.probeModelsMissingApiKey'))
    return
  }

  setProbing(index, true)
  try {
    const result = await adminAPI.accounts.probeModels({
      base_url: baseUrl,
      api_key: apiKey || undefined,
      account_id: !apiKey && accountID ? accountID : undefined,
      account_api_key_id: !apiKey && keyID ? keyID : undefined
    })
    const models = result.models ?? []
    const mode = modelModeFor(key)
    const summary = mode === 'mapping'
      ? addCatalogMappings(index, models)
      : addWhitelistModels(index, models)
    if (mode === 'mapping') {
      const fetched = new Set(uniqueModels(models))
      const missing = modelMappingRows(index, keys.value[index] || {})
        .map(row => row.to.trim())
        .filter(target => target && !fetched.has(target))
      setTokenList(missingMappingTargetsByToken, keyTokenFor(index), missing)
      summary.missing = uniqueModels(missing).length
    }
    appStore.showSuccess(t('admin.accounts.probeModelsSummary', {
      added: summary.added,
      existing: summary.existing,
      missing: summary.missing
    }))
  } catch (error) {
    appStore.showError(getProbeModelsErrorMessage(error))
  } finally {
    setProbing(index, false)
  }
}

const clearKeyModels = (index: number) => {
  const token = keyTokenFor(index)
  const { [token]: _removedRows, ...restRows } = mappingDrafts.value
  const { [token]: _removedCatalog, ...restCatalog } = catalogDrafts.value
  mappingDrafts.value = restRows
  catalogDrafts.value = restCatalog
  clearTokenModelMarkers(token)
  updateKey(index, { model_mapping: {} })
}

const removeMappingEntry = (index: number, mappingIndex: number) => {
  const rows = modelMappingRows(index, keys.value[index] || {}).filter((_, i) => i !== mappingIndex)
  setMappingRows(index, rows)
}

const removeKey = (index: number) => {
  const token = keyTokenFor(index)
  const { [token]: _removed, ...rest } = mappingDrafts.value
  mappingDrafts.value = rest
  clearTokenModelMarkers(token)
  rowTokens.value = rowTokens.value.filter((_, i) => i !== index)
  keys.value = keys.value.filter((_, i) => i !== index)
}
</script>
