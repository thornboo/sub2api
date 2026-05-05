<template>
  <div ref="rootRef" class="relative">
    <label
      v-if="label"
      class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300"
    >
      {{ label }}
    </label>
    <div class="flex gap-2">
      <input
        :value="modelValue"
        type="text"
        class="input flex-1"
        :placeholder="placeholder || t('admin.accounts.enterCustomModelName')"
        @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
        @keydown.enter.prevent="handleEnter"
        @compositionstart="isComposing = true"
        @compositionend="isComposing = false"
      />
      <button
        type="button"
        @click="queryCatalog"
        :disabled="loading"
        class="rounded-lg border border-emerald-200 px-4 py-2 text-sm font-medium text-emerald-600 hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-800 dark:text-emerald-400 dark:hover:bg-emerald-900/30"
        :title="t('admin.accounts.modelCatalogDisclaimer')"
      >
        {{ loading ? t('admin.accounts.queryingModelCatalog') : t('admin.accounts.queryModelCatalog') }}
      </button>
      <button
        type="button"
        @click="emit('add')"
        class="rounded-lg bg-primary-50 px-4 py-2 text-sm font-medium text-primary-600 hover:bg-primary-100 dark:bg-primary-900/30 dark:text-primary-400 dark:hover:bg-primary-900/50"
      >
        {{ addLabel || t('admin.accounts.addModel') }}
      </button>
    </div>

    <div
      v-if="showResults"
      class="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-lg border border-gray-200 bg-white shadow-lg dark:border-dark-600 dark:bg-dark-700"
    >
      <div class="border-b border-gray-200 px-3 py-2 text-xs text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ t('admin.accounts.modelCatalogResultHint') }}
      </div>
      <div class="max-h-72 overflow-auto">
        <button
          v-for="entry in results"
          :key="`${entry.providerId}:${entry.id}`"
          type="button"
          @click="selectEntry(entry.id)"
          class="flex w-full flex-col gap-0.5 px-3 py-2 text-left hover:bg-gray-100 dark:hover:bg-dark-600"
        >
          <span class="truncate text-sm font-medium text-gray-900 dark:text-white">{{ entry.id }}</span>
          <span class="truncate text-xs text-gray-500 dark:text-gray-400">
            {{ formatEntryMeta(entry) }}
          </span>
        </button>
        <div v-if="results.length === 0" class="px-3 py-4 text-center text-sm text-gray-500">
          {{ t('admin.accounts.modelCatalogNoResults') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import {
  loadModelsDevCatalog,
  searchModelCatalogEntries,
  type ModelCatalogEntry
} from '@/components/account/modelCatalog'

const props = defineProps<{
  modelValue: string
  label?: string
  placeholder?: string
  addLabel?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  add: [value?: string]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const showResults = ref(false)
const results = ref<ModelCatalogEntry[]>([])
const isComposing = ref(false)
const rootRef = ref<HTMLElement | null>(null)

onClickOutside(rootRef, () => {
  showResults.value = false
})

const queryCatalog = async () => {
  const query = props.modelValue.trim()
  if (!query) {
    appStore.showError(t('admin.accounts.modelCatalogQueryRequired'))
    return
  }

  loading.value = true
  try {
    const catalog = await loadModelsDevCatalog()
    results.value = searchModelCatalogEntries(catalog, query)
    showResults.value = true
  } catch (error) {
    console.warn('Failed to query models.dev catalog:', error)
    showResults.value = false
    appStore.showError(t('admin.accounts.modelCatalogLoadFailed'))
  } finally {
    loading.value = false
  }
}

const selectEntry = (modelId: string) => {
  emit('update:modelValue', modelId)
  showResults.value = false
  emit('add', modelId)
}

const handleEnter = () => {
  if (!isComposing.value) emit('add')
}

const formatEntryMeta = (entry: ModelCatalogEntry) => {
  const parts = [entry.providerName]
  if (entry.name && entry.name !== entry.id) parts.push(entry.name)
  if (entry.context) parts.push(`${entry.context.toLocaleString()} ctx`)
  if (entry.modalities.length > 0) parts.push(entry.modalities.join('/'))
  return parts.join(' · ')
}
</script>
