<template>
  <div class="card p-4">
    <div class="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
      <div class="min-w-0">
        <div class="flex flex-wrap items-center gap-2">
          <span class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300">
            <Icon :name="iconName" size="sm" :stroke-width="2" />
          </span>
          <div class="min-w-0">
            <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ title }}</p>
          </div>
        </div>
        <div v-if="hasMissingEntity" class="mt-3 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-200">
          <Icon name="exclamationTriangle" size="sm" class="mt-0.5 shrink-0" :stroke-width="2" />
          <span>{{ missingMessage }}</span>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2 lg:justify-end">
        <button
          v-if="user?.id && !user.notFound"
          type="button"
          class="btn btn-secondary text-sm"
          @click="$emit('openBalance')"
        >
          <Icon name="dollar" size="sm" class="mr-1.5" :stroke-width="2" />
          {{ t('admin.usage.profile.balanceHistory') }}
        </button>
        <button
          v-if="user?.id && !user.notFound"
          type="button"
          class="btn btn-secondary text-sm"
          @click="$emit('openApiKeys')"
        >
          <Icon name="key" size="sm" class="mr-1.5" :stroke-width="2" />
          {{ t('admin.usage.profile.userKeys') }}
        </button>
        <button
          v-if="apiKey?.id"
          type="button"
          class="btn btn-secondary text-sm"
          @click="$emit('clearApiKey')"
        >
          <Icon name="x" size="sm" class="mr-1.5" :stroke-width="2" />
          {{ t('admin.usage.profile.clearApiKey') }}
        </button>
        <button
          v-if="user?.id"
          type="button"
          class="btn btn-secondary text-sm"
          @click="$emit('clearUser')"
        >
          <Icon name="x" size="sm" class="mr-1.5" :stroke-width="2" />
          {{ t('admin.usage.profile.clearUser') }}
        </button>
      </div>
    </div>

    <div class="mt-4 rounded-xl border border-stone-200/70 bg-stone-50/70 p-3 dark:border-white/10 dark:bg-white/[0.025]">
      <div class="grid gap-3 lg:grid-cols-[minmax(320px,1.18fr)_minmax(220px,0.58fr)_minmax(132px,0.34fr)] lg:items-end">
        <div class="min-w-0">
          <UsageObjectFilterPicker
            :user="user"
            :api-key="apiKey"
            @clear-user="$emit('clearUser')"
            @clear-api-key="$emit('clearApiKey')"
            @select-user="(selectedUser) => $emit('selectUser', selectedUser)"
            @select-api-key="(selectedApiKey) => $emit('selectApiKey', selectedApiKey)"
          />
        </div>
        <slot name="controls" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import UsageObjectFilterPicker from './UsageObjectFilterPicker.vue'
import type { SimpleApiKey, SimpleUser } from '@/api/admin/usage'

export interface UsageProfileEntity {
  id: number
  label?: string | null
  loading?: boolean
  notFound?: boolean
}

const props = defineProps<{
  user?: UsageProfileEntity | null
  apiKey?: UsageProfileEntity | null
  startDate: string
  endDate: string
}>()

defineEmits<{
  clearUser: []
  clearApiKey: []
  openBalance: []
  openApiKeys: []
  selectUser: [user: SimpleUser]
  selectApiKey: [apiKey: SimpleApiKey]
}>()

const { t } = useI18n()

const hasMissingEntity = computed(() => !!props.user?.notFound || !!props.apiKey?.notFound)

const iconName = computed<'key' | 'user' | 'chartBar'>(() => {
  if (props.apiKey?.id) return 'key'
  if (props.user?.id) return 'user'
  return 'chartBar'
})

const userLabel = computed(() => {
  if (!props.user?.id) return ''
  if (props.user.notFound) return t('admin.usage.profile.userMissing', { id: props.user.id })
  if (props.user.loading) return t('admin.usage.profile.loadingUser', { id: props.user.id })
  return props.user.label || `#${props.user.id}`
})

const apiKeyLabel = computed(() => {
  if (!props.apiKey?.id) return ''
  if (props.apiKey.notFound) return t('admin.usage.profile.apiKeyMissing', { id: props.apiKey.id })
  if (props.apiKey.loading) return t('admin.usage.profile.loadingApiKey', { id: props.apiKey.id })
  return props.apiKey.label || `#${props.apiKey.id}`
})

const title = computed(() => {
  if (props.user?.id && props.apiKey?.id) {
    return t('admin.usage.profile.userApiKeyTitle', {
      user: userLabel.value,
      apiKey: apiKeyLabel.value,
    })
  }
  if (props.apiKey?.id) {
    return t('admin.usage.profile.apiKeyTitle', { apiKey: apiKeyLabel.value })
  }
  if (props.user?.id) {
    return t('admin.usage.profile.userTitle', { user: userLabel.value })
  }
  return t('admin.usage.profile.globalTitle')
})

const missingMessage = computed(() => {
  if (props.apiKey?.notFound) {
    return t('admin.usage.profile.apiKeyMissingHelp', { id: props.apiKey.id })
  }
  if (props.user?.notFound) {
    return t('admin.usage.profile.userMissingHelp', { id: props.user.id })
  }
  return ''
})
</script>
