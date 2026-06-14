<template>
  <BaseDialog
    :show="show && !!apiKey"
    :title="apiKey?.name || t('keys.usageDetails.open')"
    width="extra-wide"
    close-on-click-outside
    @close="emit('close')"
  >
    <div v-if="apiKey" class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-stone-200/80 bg-stone-50 p-3 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-2 text-sm text-stone-500 dark:text-stone-400">
          <span :class="[
            'badge',
            apiKey.status === 'active' ? 'badge-success' :
            apiKey.status === 'quota_exhausted' ? 'badge-warning' :
            apiKey.status === 'expired' ? 'badge-danger' :
            'badge-gray'
          ]">
            {{ t('keys.status.' + apiKey.status) }}
          </span>
          <span>{{ t('keys.today') }}: <strong class="font-medium text-stone-950 dark:text-white">{{ formatMoney(usageStats?.today_actual_cost ?? 0) }}</strong></span>
          <span>{{ t('keys.total') }}: <strong class="font-medium text-stone-950 dark:text-white">{{ formatMoney(usageStats?.total_actual_cost ?? 0) }}</strong></span>
          <span v-if="apiKey.group?.name">{{ t('keys.group') }}: {{ apiKey.group.name }}</span>
        </div>
      </div>

      <div class="border-b border-stone-200/80 dark:border-white/10">
        <div class="flex gap-1">
          <button
            type="button"
            :class="tabButtonClass(activeTab === 'trend')"
            @click="activeTab = 'trend'"
          >
            <Icon name="chart" size="sm" />
            <span>{{ t('keys.usageDetails.trendTab') }}</span>
          </button>
          <button
            type="button"
            :class="tabButtonClass(activeTab === 'models')"
            @click="activeTab = 'models'"
          >
            <Icon name="database" size="sm" />
            <span>{{ t('keys.usageDetails.modelsTab') }}</span>
          </button>
          <button
            type="button"
            :class="tabButtonClass(activeTab === 'logs')"
            @click="activeTab = 'logs'"
          >
            <Icon name="document" size="sm" />
            <span>{{ t('keys.usageDetails.logsTab') }}</span>
          </button>
        </div>
      </div>

      <ApiKeyUsageTrendPanel
        v-if="activeTab === 'trend'"
        :api-key-id="apiKey.id"
        :active="show && activeTab === 'trend'"
      />
      <ApiKeyUsageModelPanel
        v-else-if="activeTab === 'models'"
        :api-key-id="apiKey.id"
        :active="show && activeTab === 'models'"
      />
      <ApiKeyUsageLogsPanel
        v-else
        :api-key-id="apiKey.id"
        :active="show && activeTab === 'logs'"
      />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import ApiKeyUsageTrendPanel from '@/components/keys/ApiKeyUsageTrendPanel.vue'
import ApiKeyUsageModelPanel from '@/components/keys/ApiKeyUsageModelPanel.vue'
import ApiKeyUsageLogsPanel from '@/components/keys/ApiKeyUsageLogsPanel.vue'
import type { ApiKey } from '@/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'

const props = defineProps<{
  show: boolean
  apiKey: ApiKey | null
  usageStats?: BatchApiKeyUsageStats | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
const activeTab = ref<'trend' | 'models' | 'logs'>('trend')

const formatMoney = (value: number) => `$${value.toFixed(4)}`

const tabButtonClass = (active: boolean) => [
  'inline-flex h-10 items-center gap-2 border-b-2 px-3 text-sm font-medium transition-colors',
  active
    ? 'border-primary-500 text-primary-600 dark:text-primary-400'
    : 'border-transparent text-stone-500 hover:text-stone-950 dark:text-stone-400 dark:hover:text-white'
]

watch(() => props.apiKey?.id, () => {
  activeTab.value = 'trend'
})
</script>
