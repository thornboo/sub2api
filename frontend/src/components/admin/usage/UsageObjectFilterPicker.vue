<template>
  <div ref="rootRef" class="relative">
    <label class="input-label">{{ t('admin.usage.profile.objectFilter') }}</label>
    <div
      ref="triggerRef"
      role="button"
      tabindex="0"
      class="input flex h-11 w-full items-center gap-3 px-3 text-left"
      :class="{ 'border-emerald-500/60 ring-2 ring-emerald-500/20': open }"
      @click="toggleOpen"
      @keydown.enter.prevent="toggleOpen"
      @keydown.space.prevent="toggleOpen"
    >
      <Icon :name="triggerIcon" size="sm" class="shrink-0 text-stone-400 dark:text-stone-500" :stroke-width="2" />
      <span class="min-w-0 flex-1 truncate text-gray-900 dark:text-white">
        {{ triggerLabel }}
      </span>
      <button
        v-if="user?.id || apiKey?.id"
        type="button"
        class="shrink-0 rounded-md p-1 text-stone-400 transition hover:bg-stone-100 hover:text-stone-700 dark:text-stone-500 dark:hover:bg-dark-700 dark:hover:text-stone-200"
        :aria-label="t('admin.usage.profile.clearObjectFilter')"
        @click.stop="clearObject"
      >
        <Icon name="x" size="sm" :stroke-width="2" />
      </button>
      <Icon name="chevronDown" size="sm" class="shrink-0 text-stone-400 dark:text-stone-500" :stroke-width="2" />
    </div>

    <Teleport to="body">
      <div
        v-if="open"
        ref="panelRef"
        class="fixed z-[100000060] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-2xl shadow-black/20 dark:border-white/10 dark:bg-neutral-950 dark:shadow-black/60"
        :style="panelStyle"
      >
        <div class="grid max-h-[min(62vh,460px)] grid-cols-1 md:grid-cols-[minmax(280px,0.92fr)_minmax(320px,1.08fr)]">
          <section class="min-w-0 border-b border-gray-200 p-3 dark:border-white/10 md:border-b-0 md:border-r">
            <div class="mb-2.5 flex items-center justify-between gap-2 px-1">
              <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.usage.profile.usersColumn') }}</p>
              <span v-if="user?.id && !user.notFound" class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300">#{{ user.id }}</span>
            </div>
            <div class="relative">
              <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-stone-400 dark:text-stone-500" :stroke-width="2" />
              <input
                ref="userInputRef"
                v-model="userKeyword"
                type="text"
                class="input h-9 rounded-lg pl-9 pr-3 text-sm"
                :placeholder="t('admin.usage.searchUserPlaceholder')"
                @input="debounceUserSearch"
                @keydown.escape="close"
              />
            </div>

            <div
              ref="userListRef"
              data-test="usage-object-user-list"
              class="mt-2.5 max-h-60 space-y-0.5 overflow-auto pr-1"
              @scroll="handleUserScroll"
            >
              <button
                v-for="result in userResults"
                :key="result.id"
                type="button"
                class="flex w-full items-center gap-2.5 rounded-lg border border-transparent px-2.5 py-1 text-left transition hover:bg-gray-100 dark:hover:bg-white/[0.04]"
                :class="{ 'border-emerald-500/25 bg-emerald-50 text-emerald-900 dark:bg-emerald-500/[0.08] dark:text-emerald-100': result.id === user?.id }"
                @click="selectUser(result)"
              >
                <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-100 text-xs font-semibold text-gray-600 dark:bg-white/[0.07] dark:text-dark-200">
                  {{ result.email.slice(0, 1).toUpperCase() }}
                </span>
                <span class="flex min-w-0 flex-1 items-center gap-2">
                  <span class="min-w-0 truncate text-sm font-medium leading-5">{{ result.email }}</span>
                  <span class="shrink-0 text-xs text-stone-500 dark:text-stone-400">#{{ result.id }}</span>
                  <span v-if="result.deleted" class="shrink-0 rounded-full bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-600 dark:text-amber-300">
                    {{ t('admin.usage.userDeletedBadge') }}
                  </span>
                </span>
                <span v-if="result.id === user?.id" class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500">
                  <Icon name="check" size="xs" :stroke-width="2" />
                </span>
              </button>

              <div v-if="userResults.length === 0" class="rounded-lg px-3 py-6 text-center text-sm text-stone-500 dark:text-stone-400">
                {{ usersLoading ? t('common.loading') : t('admin.usage.profile.noUsersFound') }}
              </div>

              <div v-else-if="usersLoading" class="rounded-lg px-3 py-2 text-center text-xs text-stone-500 dark:text-stone-400">
                {{ t('common.loading') }}
              </div>
            </div>
          </section>

          <section class="min-w-0 p-3">
            <div data-test="usage-object-key-header" class="mb-2.5 flex items-center gap-2 px-1">
              <p class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.usage.profile.keysColumn') }}</p>
            </div>

            <template v-if="user?.id && !user.notFound">
              <div class="relative">
                <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-stone-400 dark:text-stone-500" :stroke-width="2" />
                <input
                  v-model="apiKeyKeyword"
                  type="text"
                  class="input h-9 rounded-lg pl-9 pr-3 text-sm"
                  :placeholder="t('admin.usage.searchApiKeyPlaceholder')"
                  @input="debounceApiKeySearch"
                  @keydown.escape="close"
                />
              </div>
              <label class="mt-2 flex items-center gap-2 px-1 text-xs text-stone-600 dark:text-stone-400">
                <input
                  v-model="includeDeletedApiKeys"
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-emerald-600 focus:ring-emerald-500 dark:border-white/20 dark:bg-neutral-900"
                  @change="loadApiKeys()"
                />
                <span>{{ t('admin.usage.profile.includeDeletedApiKeys') }}</span>
              </label>

              <div class="mt-2.5 max-h-60 space-y-0.5 overflow-auto pr-1">
                <button
                  type="button"
                  class="flex w-full items-center gap-2.5 rounded-lg border border-transparent px-2.5 py-1 text-left transition hover:bg-gray-100 dark:hover:bg-white/[0.04]"
                  :class="{ 'border-emerald-500/25 bg-emerald-50 text-emerald-900 dark:bg-emerald-500/[0.08] dark:text-emerald-100': !apiKey?.id }"
                  @click="selectAllKeys"
                >
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300">
                    <Icon name="chartBar" size="sm" :stroke-width="2" />
                  </span>
                  <span class="min-w-0 flex-1">
                    <span class="block truncate text-sm font-medium leading-5">{{ t('admin.usage.profile.allUserKeys') }}</span>
                    <span class="text-xs text-stone-500 dark:text-stone-400">{{ t('admin.usage.profile.allUserKeysHelp') }}</span>
                  </span>
                  <span v-if="!apiKey?.id" class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500">
                    <Icon name="check" size="xs" :stroke-width="2" />
                  </span>
                </button>

                <button
                  v-for="key in apiKeyResults"
                  :key="key.id"
                  type="button"
                  class="flex w-full items-center gap-2.5 rounded-lg border border-transparent px-2.5 py-1 text-left transition hover:bg-gray-100 dark:hover:bg-white/[0.04]"
                  :class="{ 'border-emerald-500/25 bg-emerald-50 text-emerald-900 dark:bg-emerald-500/[0.08] dark:text-emerald-100': key.id === apiKey?.id }"
                  @click="selectApiKey(key)"
                >
                  <span class="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-gray-100 text-stone-500 dark:bg-white/[0.07] dark:text-dark-200">
                    <Icon name="key" size="sm" :stroke-width="2" />
                  </span>
                  <span class="flex min-w-0 flex-1 items-center gap-2">
                    <span class="min-w-0 truncate text-sm font-medium leading-5">{{ key.name || `#${key.id}` }}</span>
                    <span class="shrink-0 text-xs text-stone-500 dark:text-stone-400">#{{ key.id }}</span>
                    <span v-if="key.deleted" class="shrink-0 rounded-full bg-amber-500/10 px-1.5 py-0.5 text-[10px] font-medium text-amber-600 dark:text-amber-300">
                      {{ t('admin.usage.apiKeyDeletedBadge') }}
                    </span>
                  </span>
                  <span v-if="key.id === apiKey?.id" class="flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-500">
                    <Icon name="check" size="xs" :stroke-width="2" />
                  </span>
                </button>

                <div v-if="!apiKeysLoading && apiKeyResults.length === 0" class="rounded-lg px-3 py-6 text-center text-sm text-stone-500 dark:text-stone-400">
                  {{ t('admin.usage.profile.noKeysFound') }}
                </div>
              </div>
            </template>

            <div v-else class="flex min-h-[180px] items-center justify-center rounded-xl border border-dashed border-gray-200 px-4 text-center text-sm text-stone-500 dark:border-white/10 dark:text-stone-400">
              {{ t('admin.usage.profile.selectUserFirst') }}
            </div>
          </section>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onUnmounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import Icon from '@/components/icons/Icon.vue'
import type { SimpleApiKey, SimpleUser } from '@/api/admin/usage'
import type { AdminUser } from '@/types'

interface UsageProfileEntity {
  id: number
  label?: string | null
  loading?: boolean
  notFound?: boolean
}

const props = defineProps<{
  user?: UsageProfileEntity | null
  apiKey?: UsageProfileEntity | null
}>()

const emit = defineEmits<{
  clearUser: []
  clearApiKey: []
  selectUser: [user: SimpleUser]
  selectApiKey: [apiKey: SimpleApiKey]
}>()

const { t } = useI18n()

const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const userInputRef = ref<HTMLInputElement | null>(null)
const userListRef = ref<HTMLElement | null>(null)
const open = ref(false)
const panelRect = reactive({ top: 0, left: 0, width: 720 })

const USER_PAGE_SIZE = 30
const userKeyword = ref('')
const userResults = ref<SimpleUser[]>([])
const usersLoading = ref(false)
const userPage = ref(1)
const userHasMore = ref(true)
let userSearchTimeout: ReturnType<typeof setTimeout> | null = null
let userListRequestId = 0

const apiKeyKeyword = ref('')
const apiKeyResults = ref<SimpleApiKey[]>([])
const apiKeysLoading = ref(false)
const includeDeletedApiKeys = ref(false)
let apiKeySearchTimeout: ReturnType<typeof setTimeout> | null = null

const userLabel = computed(() => {
  if (!props.user?.id) return ''
  if (props.user.loading) return t('admin.usage.profile.loadingUser', { id: props.user.id })
  if (props.user.notFound) return t('admin.usage.profile.userMissing', { id: props.user.id })
  return props.user.label || `#${props.user.id}`
})

const apiKeyLabel = computed(() => {
  if (!props.apiKey?.id) return ''
  if (props.apiKey.loading) return t('admin.usage.profile.loadingApiKey', { id: props.apiKey.id })
  if (props.apiKey.notFound) return t('admin.usage.profile.apiKeyMissing', { id: props.apiKey.id })
  return props.apiKey.label || `#${props.apiKey.id}`
})

const triggerIcon = computed<'chartBar' | 'user' | 'key'>(() => {
  if (props.apiKey?.id) return 'key'
  if (props.user?.id) return 'user'
  return 'chartBar'
})

const triggerLabel = computed(() => {
  if (props.user?.id && props.apiKey?.id) {
    return `${userLabel.value} / ${apiKeyLabel.value}`
  }
  if (props.user?.id) {
    return `${userLabel.value} / ${t('admin.usage.profile.allUserKeys')}`
  }
  if (props.apiKey?.id) {
    return apiKeyLabel.value
  }
  return t('admin.usage.profile.objectFilterPlaceholder')
})

const panelStyle = computed(() => ({
  top: `${panelRect.top}px`,
  left: `${panelRect.left}px`,
  width: `${panelRect.width}px`,
}))

const updatePanelPosition = () => {
  const trigger = triggerRef.value
  if (!trigger) return
  const rect = trigger.getBoundingClientRect()
  const margin = 12
  const desiredWidth = Math.min(Math.max(rect.width, 720), 980)
  const width = Math.min(desiredWidth, window.innerWidth - margin * 2)
  panelRect.width = width
  panelRect.left = Math.max(margin, Math.min(rect.left, window.innerWidth - width - margin))

  const panelHeight = Math.min(panelRef.value?.offsetHeight || 520, window.innerHeight - margin * 2)
  const belowTop = rect.bottom + 8
  const aboveTop = rect.top - panelHeight - 8
  panelRect.top = belowTop + panelHeight <= window.innerHeight - margin
    ? belowTop
    : Math.max(margin, aboveTop)
}

const toSimpleUser = (user: AdminUser): SimpleUser => ({
  id: user.id,
  email: user.email,
  deleted: Boolean(user.deleted_at),
})

const dedupeUsers = (users: SimpleUser[]) => {
  const seen = new Set<number>()
  return users.filter((user) => {
    if (seen.has(user.id)) return false
    seen.add(user.id)
    return true
  })
}

const selectedUserResult = (): SimpleUser | null => {
  if (!props.user?.id || props.user.notFound) return null
  return {
    id: props.user.id,
    email: userLabel.value,
    deleted: false,
  }
}

const withSelectedUser = (users: SimpleUser[]) => {
  const selected = selectedUserResult()
  if (!selected) return users
  return dedupeUsers([selected, ...users])
}

const resetUserPaging = () => {
  userPage.value = 1
  userHasMore.value = true
}

const loadUsers = async (reset = false) => {
  if (usersLoading.value) return
  if (reset) {
    resetUserPaging()
    userResults.value = withSelectedUser([])
    userListRef.value?.scrollTo?.({ top: 0 })
  }
  if (!userHasMore.value) return

  const requestId = ++userListRequestId
  const page = userPage.value
  const keyword = userKeyword.value.trim()
  usersLoading.value = true

  try {
    if (keyword) {
      const results = await adminAPI.usage.searchUsers(keyword)
      if (requestId !== userListRequestId) return

      userResults.value = withSelectedUser(
        results.sort((a, b) => Number(a.deleted) - Number(b.deleted))
      )
      userHasMore.value = false
      return
    }

    const response = await adminAPI.users.list(page, USER_PAGE_SIZE, {
      include_subscriptions: false,
      sort_by: 'email',
      sort_order: 'asc',
    })
    if (requestId !== userListRequestId) return

    const users = response.items.map(toSimpleUser)
    userResults.value = withSelectedUser(reset ? users : [...userResults.value, ...users])
    userPage.value = page + 1
    userHasMore.value = users.length > 0 && page < response.pages
  } catch {
    if (requestId !== userListRequestId) return
    if (reset) userResults.value = withSelectedUser([])
    userHasMore.value = false
  } finally {
    if (requestId === userListRequestId) usersLoading.value = false
  }
}

const loadApiKeys = async (userId = props.user?.id, keyword = apiKeyKeyword.value.trim()) => {
  if (!userId || props.user?.notFound) {
    apiKeyResults.value = []
    return
  }
  apiKeysLoading.value = true
  try {
    apiKeyResults.value = await adminAPI.usage.searchApiKeys(userId, keyword, {
      includeDeleted: includeDeletedApiKeys.value,
    })
  } catch {
    apiKeyResults.value = []
  } finally {
    apiKeysLoading.value = false
  }
}

const debounceUserSearch = () => {
  if (userSearchTimeout) clearTimeout(userSearchTimeout)
  userSearchTimeout = setTimeout(() => {
    void loadUsers(true)
  }, 300)
}

const debounceApiKeySearch = () => {
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
  apiKeySearchTimeout = setTimeout(() => {
    void loadApiKeys()
  }, 300)
}

const openPicker = async () => {
  open.value = true
  userKeyword.value = ''
  apiKeyKeyword.value = ''
  includeDeletedApiKeys.value = Boolean(props.apiKey?.id && props.apiKey.notFound)
  void loadUsers(true)
  void loadApiKeys()
  await nextTick()
  updatePanelPosition()
  userInputRef.value?.focus()
}

const close = () => {
  open.value = false
}

const toggleOpen = () => {
  if (open.value) {
    close()
    return
  }
  void openPicker()
}

const selectUser = (user: SimpleUser) => {
  userKeyword.value = ''
  userResults.value = withSelectedUser([user])
  resetUserPaging()
  apiKeyKeyword.value = ''
  emit('selectUser', user)
  void loadApiKeys(user.id, '')
}

const selectAllKeys = () => {
  emit('clearApiKey')
  close()
}

const selectApiKey = (apiKey: SimpleApiKey) => {
  emit('selectApiKey', apiKey)
  close()
}

const clearObject = () => {
  emit('clearUser')
  close()
}

const handleUserScroll = (event: Event) => {
  const el = event.currentTarget as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 48) {
    void loadUsers(false)
  }
}

const onDocumentClick = (event: MouseEvent) => {
  if (!open.value) return
  const target = event.target as Node | null
  if (!target) return
  const insideTrigger = rootRef.value?.contains(target) ?? false
  const insidePanel = panelRef.value?.contains(target) ?? false
  if (!insideTrigger && !insidePanel) {
    close()
  }
}

let panelListenersActive = false

const addPanelListeners = () => {
  if (panelListenersActive) return
  panelListenersActive = true
  document.addEventListener('click', onDocumentClick)
  window.addEventListener('resize', updatePanelPosition)
  window.addEventListener('scroll', updatePanelPosition, true)
}

const removePanelListeners = () => {
  if (!panelListenersActive) return
  panelListenersActive = false
  document.removeEventListener('click', onDocumentClick)
  window.removeEventListener('resize', updatePanelPosition)
  window.removeEventListener('scroll', updatePanelPosition, true)
}

onUnmounted(() => {
  removePanelListeners()
  if (userSearchTimeout) clearTimeout(userSearchTimeout)
  if (apiKeySearchTimeout) clearTimeout(apiKeySearchTimeout)
})

watch(open, async (isOpen) => {
  if (isOpen) {
    addPanelListeners()
    await nextTick()
    updatePanelPosition()
    return
  }
  removePanelListeners()
})

watch(
  () => props.user?.id,
  () => {
    if (!open.value) return
    apiKeyKeyword.value = ''
    void loadUsers(true)
    void loadApiKeys()
  }
)
</script>
