<template>
  <div class="space-y-2">
    <Select
      :model-value="selectedId"
      :options="displayOptions"
      :disabled="disabled"
      :placeholder="placeholder"
      :aria-label="t('admin.accounts.selectTestModel')"
      value-key="id"
      label-key="display_name"
      @update:model-value="selectModel(String($event ?? ''))"
    >
      <template #selected="{ option }">
        <span :title="String(option?.display_name || '')">{{ option?.display_name || placeholder }}</span>
      </template>
      <template #option="{ option, selected }">
        <span class="min-w-0 flex-1 whitespace-normal break-all leading-5">{{ option.display_name }}</span>
        <Icon v-if="selected" name="check" size="sm" class="shrink-0 text-emerald-500" />
      </template>
    </Select>
    <template v-if="selectedOption?.is_pattern">
      <label data-testid="account-test-concrete-model" class="block text-xs text-stone-500 dark:text-stone-400">
        {{ t('admin.accounts.concreteTestModel') }}
        <input
          v-model="concreteModel"
          class="input mt-1.5 w-full"
          :disabled="disabled"
          :placeholder="t('admin.accounts.concreteTestModelPlaceholder')"
          :aria-invalid="Boolean(errorMessage)"
          autocomplete="off"
          spellcheck="false"
        />
      </label>
      <p v-if="errorMessage" role="alert" class="text-xs text-red-500">{{ errorMessage }}</p>
      <p v-else-if="resolving" role="status" class="text-xs text-stone-500">{{ t('common.loading') }}...</p>
      <p v-else-if="resolvedTarget" class="break-all text-xs text-stone-500 dark:text-stone-400">
        {{ t('admin.accounts.resolvedTestModel', { model: resolvedTarget }) }}
      </p>
      <p v-else class="text-xs text-stone-500 dark:text-stone-400">{{ t('admin.accounts.concreteTestModelHint') }}</p>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AccountTestModel } from '@/api/admin/accounts'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  options: AccountTestModel[]
  accountId: number
  disabled?: boolean
  active?: boolean
  placeholder?: string
}>(), { disabled: false, active: true, placeholder: '' })
const emit = defineEmits<{
  (event: 'update:modelValue', value: string): void
  (event: 'update:resolvedTarget', value: string): void
}>()
const { t } = useI18n()
const selectedId = ref('')
const concreteModel = ref('')
const resolvedTarget = ref('')
const errorMessage = ref('')
const resolving = ref(false)
let publishedValue: string | undefined
let controller: AbortController | null = null
let timer: ReturnType<typeof setTimeout> | undefined

const selectedOption = computed(() => props.options.find((model) => model.id === selectedId.value))
const displayOptions = computed(() => {
  const options = props.options.map((model) => ({
    ...model,
    display_name: model.upstream_model_id && model.upstream_model_id !== model.id
      ? `${model.id} → ${model.upstream_model_id}`
      : model.id
  }))
  // A scheduled test may already store a concrete ID covered by a wildcard rule.
  if (selectedId.value && !selectedOption.value && !selectedId.value.includes('*')) {
    return [...options, { id: selectedId.value, display_name: selectedId.value }]
  }
  return options
})

function publish(value: string) {
  publishedValue = value
  emit('update:modelValue', value)
}

function publishResolvedTarget(value: string) {
  emit('update:resolvedTarget', value)
}

function cancelResolution() {
  clearTimeout(timer)
  controller?.abort()
  controller = null
  resolving.value = false
  resolvedTarget.value = ''
  errorMessage.value = ''
}

function selectModel(id: string) {
  cancelResolution()
  selectedId.value = id
  concreteModel.value = ''
  const option = selectedOption.value
  if (option?.disabled) {
    publish('')
    publishResolvedTarget('')
    return
  }
  if (option?.is_pattern || id.includes('*')) {
    publish('')
    publishResolvedTarget('')
    return
  }
  publish(id)
  publishResolvedTarget(option?.upstream_model_id || id)
}

watch(() => props.modelValue, (value) => {
  if (value !== publishedValue) selectModel(value)
}, { immediate: true })

watch([() => props.accountId, () => props.active], () => selectModel(''))

watch(concreteModel, (value) => {
  cancelResolution()
  if (!selectedOption.value?.is_pattern) return
  publish('')
  publishResolvedTarget('')
  const modelId = value.trim()
  if (!modelId || !props.active) return
  if (modelId.includes('*')) {
    errorMessage.value = t('admin.accounts.concreteTestModelRequired')
    return
  }
  resolving.value = true
  const request = new AbortController()
  controller = request
  const accountId = props.accountId
  timer = setTimeout(async () => {
    try {
      const result = await adminAPI.accounts.resolveTestModel(accountId, modelId, request.signal)
      if (request.signal.aborted || controller !== request) return
      resolvedTarget.value = result.upstream_model_id
      publish(result.model_id)
      publishResolvedTarget(result.upstream_model_id)
    } catch {
      if (request.signal.aborted || controller !== request) return
      errorMessage.value = t('admin.accounts.resolveTestModelFailed')
    } finally {
      if (controller === request) {
        resolving.value = false
        controller = null
      }
    }
  }, 300)
}, { flush: 'sync' })

onBeforeUnmount(cancelResolution)
</script>
