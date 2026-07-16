<template>
  <div class="rounded-lg border border-stone-200/80 bg-white/80 p-3 shadow-sm dark:border-white/10 dark:bg-neutral-950/70">
    <!-- Collapsed summary header (clickable) -->
    <div
      class="flex cursor-pointer select-none items-center gap-2"
      @click="collapsed = !collapsed"
    >
      <Icon
        :name="collapsed ? 'chevronRight' : 'chevronDown'"
        size="sm"
        :stroke-width="2"
        class="flex-shrink-0 text-stone-400 transition-transform duration-200"
      />

      <!-- Summary: model tags + billing badge -->
      <div v-if="collapsed" class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden">
        <!-- Compact model tags (show first 3) -->
        <div class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
          <span
            v-for="(m, i) in entry.models.slice(0, 3)"
            :key="i"
            class="inline-flex shrink-0 rounded px-1.5 py-0.5 text-xs"
            :class="getPlatformTagClass(props.platform || '')"
          >
            {{ m }}
          </span>
          <span
            v-if="entry.models.length > 3"
            class="whitespace-nowrap text-xs text-stone-400"
          >
            +{{ entry.models.length - 3 }}
          </span>
          <span
            v-if="entry.models.length === 0"
            class="text-xs italic text-stone-400"
          >
            {{ t('admin.channels.form.noModels') }}
          </span>
        </div>

        <!-- Billing mode badge -->
        <span
          class="flex-shrink-0 rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 ring-1 ring-inset ring-emerald-200/80 dark:bg-emerald-500/10 dark:text-emerald-300 dark:ring-emerald-500/20"
        >
          {{ billingModeLabel }}
        </span>
      </div>

      <!-- Expanded: show the label "Pricing Entry" or similar -->
      <div v-else class="flex-1 text-xs font-medium text-stone-500 dark:text-stone-400">
        {{ t('admin.channels.form.pricingEntry') }}
      </div>

      <!-- Remove button (always visible, stop propagation) -->
      <button
        type="button"
        @click.stop="emit('remove')"
        class="flex-shrink-0 rounded p-1 text-stone-400 transition-colors hover:bg-red-500/10 hover:text-red-500"
      >
        <Icon name="trash" size="sm" />
      </button>
    </div>

    <!-- Expandable content with transition -->
    <div
      class="collapsible-content"
      :class="{ 'collapsible-content--collapsed': collapsed }"
    >
      <div class="collapsible-inner">
        <!-- Header: Models + Billing Mode -->
        <div class="mt-3 flex items-start gap-2">
          <div class="flex-1">
            <label class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.channels.form.models') }} <span class="text-red-500">*</span>
            </label>
            <ModelTagInput
              :models="entry.models"
              :platform="props.platform"
              @update:models="onModelsUpdate($event)"
              :placeholder="t('admin.channels.form.modelsPlaceholder')"
              class="mt-1"
            />
            <div v-if="entry.models.length > 0" class="mt-2 space-y-2">
              <div class="flex items-center justify-between gap-2 text-xs">
                <span class="font-medium text-stone-500 dark:text-stone-400">
                  {{ t('admin.channels.form.modelSelfCheck', '模型健康自检') }}
                </span>
                <span class="text-stone-400">
                  {{ enabledSelfCheckCount }}/{{ entry.models.length }}
                </span>
              </div>
              <div class="grid grid-cols-1 gap-1.5 sm:grid-cols-2">
                <label
                  v-for="model in entry.models"
                  :key="model"
                  class="flex min-w-0 items-center gap-2 text-xs text-stone-600 dark:text-stone-300"
                >
                  <BaseCheckbox
                    :model-value="isSelfCheckEnabled(model)"
                    :aria-label="`${t('admin.channels.form.modelSelfCheck', '模型健康自检')} ${model}`"
                    @update:modelValue="toggleSelfCheck(model, $event)"
                  />
                  <span class="min-w-0 truncate font-mono" :title="model">{{ model }}</span>
                </label>
              </div>
            </div>
          </div>
          <div class="w-40">
            <label class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.channels.form.billingMode') }}
            </label>
            <Select
              :modelValue="entry.billing_mode"
              @update:modelValue="emit('update', { ...entry, billing_mode: $event as BillingMode, intervals: [] })"
              :options="billingModeOptions"
              class="mt-1"
            />
          </div>
        </div>

        <!-- Token mode -->
        <div v-if="entry.billing_mode === 'token'">
          <!-- Default prices (fallback when no interval matches) -->
          <label class="mt-3 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('admin.channels.form.defaultPrices') }}
            <span class="ml-1 font-normal text-stone-400">$/MTok</span>
          </label>
          <div class="mt-1 grid grid-cols-2 gap-2 sm:grid-cols-6">
            <div>
              <label class="text-xs text-stone-400">{{ t('admin.channels.form.inputPrice') }}</label>
              <input :value="entry.input_price" @input="emitField('input_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
            </div>
            <div>
              <label class="text-xs text-stone-400">{{ t('admin.channels.form.outputPrice') }}</label>
              <input :value="entry.output_price" @input="emitField('output_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
            </div>
            <div>
              <label class="text-xs text-stone-400">{{ t('admin.channels.form.cacheWritePrice') }}</label>
              <input :value="entry.cache_write_price" @input="emitField('cache_write_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
            </div>
            <div>
              <label class="text-xs text-stone-400">{{ t('admin.channels.form.cacheReadPrice') }}</label>
              <input :value="entry.cache_read_price" @input="emitField('cache_read_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
            </div>
            <div>
              <label class="text-xs text-stone-400">{{ t('admin.channels.form.imageInputPrice') }}</label>
              <input :value="entry.image_input_price" @input="emitField('image_input_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
            </div>
            <div>
              <label class="text-xs text-stone-400">{{ t('admin.channels.form.imageTokenPrice') }}</label>
              <input :value="entry.image_output_price" @input="emitField('image_output_price', ($event.target as HTMLInputElement).value)"
                type="number" step="any" min="0" class="input mt-0.5 text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
            </div>
          </div>

          <!-- Token intervals -->
          <div class="mt-3">
            <div class="flex items-center justify-between">
              <label class="text-xs font-medium text-stone-500 dark:text-stone-400">
                {{ t('admin.channels.form.intervals') }}
                <span class="ml-1 font-normal text-stone-400">(min, max]</span>
              </label>
              <button type="button" @click="addInterval" class="text-xs text-emerald-600 transition-colors hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300">
                + {{ t('admin.channels.form.addInterval') }}
              </button>
            </div>
            <div v-if="entry.intervals && entry.intervals.length > 0" class="mt-2 space-y-2">
              <IntervalRow
                v-for="(iv, idx) in entry.intervals"
                :key="idx"
                :interval="iv"
                :mode="entry.billing_mode"
                @update="updateInterval(idx, $event)"
                @remove="removeInterval(idx)"
              />
            </div>
          </div>
        </div>

        <!-- Per-request mode -->
        <div v-else-if="entry.billing_mode === 'per_request'">
          <!-- Default per-request price -->
          <label class="mt-3 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('admin.channels.form.defaultPerRequestPrice') }}
            <span class="ml-1 font-normal text-stone-400">$</span>
          </label>
          <div class="mt-1 w-48">
            <input :value="entry.per_request_price" @input="emitField('per_request_price', ($event.target as HTMLInputElement).value)"
              type="number" step="any" min="0" class="input text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
          </div>

          <!-- Tiers -->
          <div class="mt-3 flex items-center justify-between">
            <label class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.channels.form.requestTiers') }}
            </label>
            <button type="button" @click="addInterval" class="text-xs text-emerald-600 transition-colors hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300">
              + {{ t('admin.channels.form.addTier') }}
            </button>
          </div>
          <div v-if="entry.intervals && entry.intervals.length > 0" class="mt-2 space-y-2">
            <IntervalRow
              v-for="(iv, idx) in entry.intervals"
              :key="idx"
              :interval="iv"
              :mode="entry.billing_mode"
              @update="updateInterval(idx, $event)"
              @remove="removeInterval(idx)"
            />
          </div>
          <div v-else class="mt-2 rounded border border-dashed border-stone-300 p-3 text-center text-xs text-stone-400 dark:border-white/10">
            {{ t('admin.channels.form.noTiersYet') }}
          </div>
        </div>

        <!-- Image mode -->
        <div v-else-if="entry.billing_mode === 'image'">
          <!-- Default image price (per-request, same as per_request mode) -->
          <label class="mt-3 block text-xs font-medium text-stone-500 dark:text-stone-400">
            {{ t('admin.channels.form.defaultImagePrice') }}
            <span class="ml-1 font-normal text-stone-400">$</span>
          </label>
          <div class="mt-1 w-48">
            <input :value="entry.per_request_price" @input="emitField('per_request_price', ($event.target as HTMLInputElement).value)"
              type="number" step="any" min="0" class="input text-sm" :placeholder="t('admin.channels.form.pricePlaceholder')" />
          </div>

          <!-- Image tiers -->
          <div class="mt-3 flex items-center justify-between">
            <label class="text-xs font-medium text-stone-500 dark:text-stone-400">
              {{ t('admin.channels.form.imageTiers') }}
            </label>
            <button type="button" @click="addImageTier" class="text-xs text-emerald-600 transition-colors hover:text-emerald-700 dark:text-emerald-400 dark:hover:text-emerald-300">
              + {{ t('admin.channels.form.addTier') }}
            </button>
          </div>
          <div v-if="entry.intervals && entry.intervals.length > 0" class="mt-2 space-y-2">
            <IntervalRow
              v-for="(iv, idx) in entry.intervals"
              :key="idx"
              :interval="iv"
              :mode="entry.billing_mode"
              @update="updateInterval(idx, $event)"
              @remove="removeInterval(idx)"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import BaseCheckbox from '@/components/common/BaseCheckbox.vue'
import Icon from '@/components/icons/Icon.vue'
import IntervalRow from './IntervalRow.vue'
import ModelTagInput from './ModelTagInput.vue'
import type { PricingFormEntry, IntervalFormEntry } from './types'
import { perTokenToMTok, getPlatformTagClass } from './types'
import type { BillingMode } from '@/api/admin/channels'
import channelsAPI from '@/api/admin/channels'

const { t } = useI18n()

const props = defineProps<{
  entry: PricingFormEntry
  platform?: string
}>()

const emit = defineEmits<{
  update: [entry: PricingFormEntry]
  remove: []
}>()

// Collapse state: entries with existing models default to collapsed
const collapsed = ref(props.entry.models.length > 0)

const billingModeOptions = computed(() => [
  { value: 'token', label: t('admin.channels.billingMode.token') },
  { value: 'per_request', label: t('admin.channels.billingMode.perRequest') },
  { value: 'image', label: t('admin.channels.billingMode.image') }
])

const billingModeLabel = computed(() => {
  const opt = billingModeOptions.value.find(o => o.value === props.entry.billing_mode)
  return opt ? opt.label : props.entry.billing_mode
})

const enabledSelfCheckCount = computed(() =>
  pruneSelfCheckModels(props.entry.models, props.entry.self_check_enabled_models || []).length
)

function emitField(field: keyof PricingFormEntry, value: string) {
  emit('update', { ...props.entry, [field]: value === '' ? null : value })
}

function addInterval() {
  const intervals = [...(props.entry.intervals || [])]
  intervals.push({
    min_tokens: 0, max_tokens: null, tier_label: '',
    input_price: null, output_price: null, cache_write_price: null,
    cache_read_price: null, per_request_price: null,
    sort_order: intervals.length
  })
  emit('update', { ...props.entry, intervals })
}

function addImageTier() {
  const intervals = [...(props.entry.intervals || [])]
  const labels = ['1K', '2K', '4K', 'HD']
  intervals.push({
    min_tokens: 0, max_tokens: null, tier_label: labels[intervals.length] || '',
    input_price: null, output_price: null, cache_write_price: null,
    cache_read_price: null, per_request_price: null,
    sort_order: intervals.length
  })
  emit('update', { ...props.entry, intervals })
}

function updateInterval(idx: number, updated: IntervalFormEntry) {
  const intervals = [...(props.entry.intervals || [])]
  intervals[idx] = updated
  emit('update', { ...props.entry, intervals })
}

function removeInterval(idx: number) {
  const intervals = [...(props.entry.intervals || [])]
  intervals.splice(idx, 1)
  emit('update', { ...props.entry, intervals })
}

async function onModelsUpdate(newModels: string[]) {
  const oldModels = props.entry.models
  const nextSelfCheckModels = pruneSelfCheckModels(newModels, props.entry.self_check_enabled_models || [])
  emit('update', { ...props.entry, models: newModels, self_check_enabled_models: nextSelfCheckModels })

  // 只在新增模型且当前无价格时自动填充
  const addedModels = newModels.filter(m => !oldModels.includes(m))
  if (addedModels.length === 0) return

  // 检查是否所有价格字段都为空
  const e = props.entry
  const hasPrice = e.input_price != null || e.output_price != null ||
                   e.cache_write_price != null || e.cache_read_price != null
  if (hasPrice) return

  // 查询第一个新增模型的默认价格
  try {
    const result = await channelsAPI.getModelDefaultPricing(addedModels[0])
    if (result.found) {
      emit('update', {
        ...props.entry,
        models: newModels,
        self_check_enabled_models: nextSelfCheckModels,
        input_price: perTokenToMTok(result.input_price ?? null),
        output_price: perTokenToMTok(result.output_price ?? null),
        cache_write_price: perTokenToMTok(result.cache_write_price ?? null),
        cache_read_price: perTokenToMTok(result.cache_read_price ?? null),
        image_input_price: perTokenToMTok(result.image_input_price ?? null),
        image_output_price: perTokenToMTok(result.image_output_price ?? null),
      })
    }
  } catch {
    // 查询失败不影响用户操作
  }
}

function isSelfCheckEnabled(model: string): boolean {
  const key = normalizeModelKey(model)
  return (props.entry.self_check_enabled_models || []).some(item => normalizeModelKey(item) === key)
}

function toggleSelfCheck(model: string, checked: boolean) {
  const key = normalizeModelKey(model)
  let next = pruneSelfCheckModels(props.entry.models, props.entry.self_check_enabled_models || [])
  if (checked) {
    if (!next.some(item => normalizeModelKey(item) === key)) {
      next = [...next, model.trim()]
    }
  } else {
    next = next.filter(item => normalizeModelKey(item) !== key)
  }
  emit('update', { ...props.entry, self_check_enabled_models: next })
}

function pruneSelfCheckModels(models: string[], enabledModels: string[]): string[] {
  const allowed = new Map(models.map(model => [normalizeModelKey(model), model.trim()]))
  const next: string[] = []
  const seen = new Set<string>()
  for (const model of enabledModels) {
    const key = normalizeModelKey(model)
    const canonical = allowed.get(key)
    if (!canonical || seen.has(key)) continue
    seen.add(key)
    next.push(canonical)
  }
  return next
}

function normalizeModelKey(model: string): string {
  return model.trim().toLowerCase()
}
</script>

<style scoped>
.collapsible-content {
  display: grid;
  grid-template-rows: 1fr;
  transition: grid-template-rows 0.25s ease;
}

.collapsible-content--collapsed {
  grid-template-rows: 0fr;
}

.collapsible-inner {
  overflow: hidden;
}
</style>
