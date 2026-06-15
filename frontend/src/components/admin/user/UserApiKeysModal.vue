<template>
  <BaseDialog :show="show" :title="t('admin.users.userApiKeys')" width="wide" @close="handleClose">
    <div v-if="user" class="space-y-4">
      <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-stone-200/80 bg-stone-50/80 p-3 dark:border-white/10 dark:bg-white/[0.03]">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-100/80 dark:bg-primary-500/10">
            <span class="text-lg font-medium text-primary-700 dark:text-primary-300">{{ user.email.charAt(0).toUpperCase() }}</span>
          </div>
          <div class="min-w-0">
            <p class="truncate font-medium text-stone-950 dark:text-white">{{ user.email }}</p>
            <p class="truncate text-sm text-stone-500 dark:text-stone-400">{{ user.username }}</p>
          </div>
        </div>
        <button
          type="button"
          class="inline-flex h-8 shrink-0 items-center gap-2 rounded-lg px-2.5 text-sm font-medium text-stone-500 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:text-stone-400 dark:hover:bg-primary-500/10 dark:hover:text-primary-300"
          :title="t('admin.users.viewUserUsageDetails')"
          @click="openUsageModal(null)"
        >
          <Icon name="chart" size="sm" />
          <span>{{ t('admin.users.usageDetails') }}</span>
        </button>
      </div>
      <div v-if="loading" class="flex justify-center py-8"><svg class="h-8 w-8 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg></div>
      <div v-else-if="apiKeys.length === 0" class="py-8 text-center"><p class="text-sm text-stone-500 dark:text-stone-400">{{ t('admin.users.noApiKeys') }}</p></div>
      <div v-else ref="scrollContainerRef" class="max-h-96 space-y-3 overflow-y-auto" @scroll="closeGroupSelector">
        <div
          v-for="key in apiKeys"
          :key="key.id"
          class="rounded-xl border border-stone-200/80 bg-white/80 p-4 transition-colors hover:border-primary-200 hover:bg-primary-50/40 dark:border-white/10 dark:bg-white/[0.03] dark:hover:border-primary-500/40 dark:hover:bg-primary-500/[0.06]"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0 flex-1">
              <div class="mb-1 flex flex-wrap items-center gap-2">
                <span class="font-medium text-stone-950 dark:text-white">{{ key.name }}</span>
                <span :class="apiKeyStatusBadgeClass(key.status)">{{ t('keys.status.' + key.status) }}</span>
              </div>
              <p class="truncate font-mono text-sm text-stone-500 dark:text-stone-400">{{ key.key.substring(0, 20) }}...{{ key.key.substring(key.key.length - 8) }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <div class="hidden text-right text-xs text-stone-500 dark:text-stone-400 sm:block">
                <div>{{ t('keys.today') }}: <span class="font-medium text-stone-950 dark:text-white">{{ formatMoney(apiKeyUsageStats[key.id]?.today_actual_cost ?? 0) }}</span></div>
                <div>{{ t('keys.total') }}: <span class="font-medium text-stone-950 dark:text-white">{{ formatMoney(apiKeyUsageStats[key.id]?.total_actual_cost ?? 0) }}</span></div>
              </div>
              <button
                type="button"
                class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-stone-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:text-stone-500 dark:hover:bg-primary-500/10 dark:hover:text-primary-300"
                :title="t('admin.users.viewApiKeyUsageDetails')"
                @click.stop="openUsageModal(key)"
              >
                <Icon name="chart" size="sm" />
              </button>
            </div>
          </div>
          <div class="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-xs text-stone-500 dark:text-stone-400">
            <div class="flex items-center gap-1">
              <span>{{ t('admin.users.group') }}:</span>
              <button
                :ref="(el) => setGroupButtonRef(key.id, el)"
                @click="openGroupSelector(key)"
                class="-mx-1 -my-0.5 flex cursor-pointer items-center gap-1 rounded-md px-1 py-0.5 transition-colors hover:bg-stone-100 dark:hover:bg-white/[0.06]"
                :disabled="updatingKeyIds.has(key.id)"
              >
                <GroupBadge
                  v-if="key.group_id && key.group"
                  :name="key.group.name"
                  :platform="key.group.platform"
                  :subscription-type="key.group.subscription_type"
                  :rate-multiplier="key.group.rate_multiplier"
                />
                <span v-else class="text-stone-400 italic dark:text-stone-500">{{ t('admin.users.none') }}</span>
                <svg v-if="updatingKeyIds.has(key.id)" class="h-3 w-3 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                <svg v-else class="h-3 w-3 text-stone-400 dark:text-stone-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M8.25 15L12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9" /></svg>
              </button>
            </div>
            <div class="flex items-center gap-1"><span>{{ t('admin.users.columns.created') }}: {{ formatDateTime(key.created_at) }}</span></div>
          </div>
        </div>
      </div>
    </div>
  </BaseDialog>

  <!-- Group Selector Dropdown -->
  <Teleport to="body">
    <div
      v-if="groupSelectorKeyId !== null && dropdownPosition"
      ref="dropdownRef"
      class="animate-in fade-in slide-in-from-top-2 fixed z-[100000020] w-64 overflow-hidden rounded-xl border border-stone-200/80 bg-white/95 shadow-xl shadow-stone-950/10 backdrop-blur-xl duration-200 dark:border-white/10 dark:bg-neutral-950/95 dark:shadow-black/30"
      :style="{ top: dropdownPosition.top + 'px', left: dropdownPosition.left + 'px' }"
    >
      <div class="max-h-64 overflow-y-auto p-1.5">
        <!-- Unbind option -->
        <button
          @click="changeGroup(selectedKeyForGroup!, null)"
          :class="[
            'flex w-full items-center rounded-lg px-3 py-2 text-sm transition-colors',
            !selectedKeyForGroup?.group_id
              ? 'bg-primary-50 dark:bg-primary-500/10'
              : 'hover:bg-stone-100 dark:hover:bg-white/[0.06]'
          ]"
        >
          <span class="text-stone-500 italic dark:text-stone-400">{{ t('admin.users.none') }}</span>
          <svg
            v-if="!selectedKeyForGroup?.group_id"
            class="ml-auto h-4 w-4 shrink-0 text-primary-600 dark:text-primary-400"
            fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"
          ><path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" /></svg>
        </button>
        <!-- Group options -->
        <button
          v-for="group in allGroups"
          :key="group.id"
          @click="changeGroup(selectedKeyForGroup!, group.id)"
          :class="[
            'flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors',
            selectedKeyForGroup?.group_id === group.id
              ? 'bg-primary-50 dark:bg-primary-500/10'
              : 'hover:bg-stone-100 dark:hover:bg-white/[0.06]'
          ]"
        >
          <GroupOptionItem
            :name="group.name"
            :platform="group.platform"
            :subscription-type="group.subscription_type"
            :rate-multiplier="group.rate_multiplier"
            :description="group.description"
            :selected="selectedKeyForGroup?.group_id === group.id"
          />
        </button>
      </div>
    </div>
  </Teleport>

  <AdminApiKeyUsageModal
    :show="showUsageModal"
    :user="user"
    :api-key="selectedUsageKey"
    @close="closeUsageModal"
  />
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { BatchApiKeyUsageStats } from '@/api/admin/dashboard'
import { formatDateTime } from '@/utils/format'
import type { AdminUser, AdminGroup, ApiKey } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import AdminApiKeyUsageModal from './AdminApiKeyUsageModal.vue'

const props = defineProps<{ show: boolean; user: AdminUser | null }>()
const emit = defineEmits(['close'])
const { t } = useI18n()
const appStore = useAppStore()

const apiKeys = ref<ApiKey[]>([])
const apiKeyUsageStats = ref<Record<string, BatchApiKeyUsageStats>>({})
const allGroups = ref<AdminGroup[]>([])
const loading = ref(false)
const updatingKeyIds = ref(new Set<number>())
const groupSelectorKeyId = ref<number | null>(null)
const showUsageModal = ref(false)
const selectedUsageKey = ref<ApiKey | null>(null)
const dropdownPosition = ref<{ top: number; left: number } | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const scrollContainerRef = ref<HTMLElement | null>(null)
const groupButtonRefs = ref<Map<number, HTMLElement>>(new Map())

const selectedKeyForGroup = computed(() => {
  if (groupSelectorKeyId.value === null) return null
  return apiKeys.value.find((k) => k.id === groupSelectorKeyId.value) || null
})

const setGroupButtonRef = (keyId: number, el: Element | ComponentPublicInstance | null) => {
  if (el instanceof HTMLElement) {
    groupButtonRefs.value.set(keyId, el)
  } else {
    groupButtonRefs.value.delete(keyId)
  }
}

const load = async () => {
  if (!props.user) return
  loading.value = true
  groupButtonRefs.value.clear()
  apiKeyUsageStats.value = {}
  try {
    const res = await adminAPI.users.getUserApiKeys(props.user.id)
    apiKeys.value = res.items || []
    void loadUsageStats()
  } catch (error) {
    console.error('Failed to load API keys:', error)
  } finally {
    loading.value = false
  }
}

const loadUsageStats = async () => {
  const ids = apiKeys.value.map((key) => key.id)
  if (ids.length === 0) {
    apiKeyUsageStats.value = {}
    return
  }
  try {
    const res = await adminAPI.dashboard.getBatchApiKeysUsage(ids)
    apiKeyUsageStats.value = res.stats || {}
  } catch (error) {
    console.error('Failed to load API key usage stats:', error)
    apiKeyUsageStats.value = {}
  }
}

const loadGroups = async () => {
  try {
    const groups = await adminAPI.groups.getAll()
    allGroups.value = groups
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

const DROPDOWN_HEIGHT = 272 // max-h-64 = 16rem = 256px + padding
const DROPDOWN_GAP = 4

const openGroupSelector = (key: ApiKey) => {
  if (groupSelectorKeyId.value === key.id) {
    closeGroupSelector()
  } else {
    const buttonEl = groupButtonRefs.value.get(key.id)
    if (buttonEl) {
      const rect = buttonEl.getBoundingClientRect()
      const spaceBelow = window.innerHeight - rect.bottom
      const openUpward = spaceBelow < DROPDOWN_HEIGHT && rect.top > spaceBelow
      dropdownPosition.value = {
        top: openUpward ? rect.top - DROPDOWN_HEIGHT - DROPDOWN_GAP : rect.bottom + DROPDOWN_GAP,
        left: rect.left
      }
    }
    groupSelectorKeyId.value = key.id
  }
}

const closeGroupSelector = () => {
  groupSelectorKeyId.value = null
  dropdownPosition.value = null
}

const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  closeGroupSelector()
  if (key.group_id === newGroupId || (!key.group_id && newGroupId === null)) return

  updatingKeyIds.value.add(key.id)
  try {
    const result = await adminAPI.apiKeys.updateApiKeyGroup(key.id, newGroupId)
    // Update local data
    const idx = apiKeys.value.findIndex((k) => k.id === key.id)
    if (idx !== -1) {
      apiKeys.value[idx] = result.api_key
    }
    if (result.auto_granted_group_access && result.granted_group_name) {
      appStore.showSuccess(t('admin.users.groupChangedWithGrant', { group: result.granted_group_name }))
    } else {
      appStore.showSuccess(t('admin.users.groupChangedSuccess'))
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.users.groupChangeFailed'))
  } finally {
    updatingKeyIds.value.delete(key.id)
  }
}

const handleKeyDown = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && groupSelectorKeyId.value !== null) {
    event.stopPropagation()
    closeGroupSelector()
  }
}

const handleClickOutside = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  if (dropdownRef.value && !dropdownRef.value.contains(target)) {
    // Check if the click is on one of the group trigger buttons
    for (const el of groupButtonRefs.value.values()) {
      if (el.contains(target)) return
    }
    closeGroupSelector()
  }
}

const handleClose = () => {
  closeGroupSelector()
  closeUsageModal()
  emit('close')
}

const openUsageModal = (key: ApiKey | null) => {
  closeGroupSelector()
  selectedUsageKey.value = key
  showUsageModal.value = true
}

const closeUsageModal = () => {
  showUsageModal.value = false
  selectedUsageKey.value = null
}

const apiKeyStatusBadgeClass = (status: ApiKey['status']) => [
  'badge text-xs',
  status === 'active' ? 'badge-success'
    : status === 'quota_exhausted' ? 'badge-warning'
      : status === 'expired' ? 'badge-danger'
        : 'badge-gray'
]

const formatMoney = (value: number) => `$${value.toFixed(4)}`

watch(
  () => props.show,
  (v) => {
    if (v && props.user) {
      load()
      loadGroups()
    } else {
      closeGroupSelector()
      closeUsageModal()
    }
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleKeyDown, true)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleKeyDown, true)
})
</script>
