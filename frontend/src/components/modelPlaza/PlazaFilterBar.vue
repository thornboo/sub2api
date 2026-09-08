<template>
  <div
    class="grid min-w-0 grid-cols-2 gap-2 md:grid-cols-[minmax(0,1.5fr)_repeat(2,minmax(0,1fr))] [&_.select-trigger]:!px-2"
  >
    <div class="relative col-span-2 min-w-0 md:col-span-1">
      <Icon
        name="search"
        size="md"
        class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-stone-400 dark:text-stone-500"
      />
      <input
        :value="search"
        type="text"
        :aria-label="t('modelPlaza.filters.modelLabel')"
        :placeholder="t('modelPlaza.filters.searchPlaceholder')"
        class="input pl-10 pr-9"
        @input="$emit('update:search', ($event.target as HTMLInputElement).value)"
      />
      <button
        v-if="search"
        type="button"
        class="absolute right-2.5 top-1/2 -translate-y-1/2 rounded p-0.5 text-stone-400 transition-colors hover:bg-stone-100 hover:text-stone-700 dark:text-stone-500 dark:hover:bg-white/[0.06] dark:hover:text-stone-200"
        :aria-label="t('modelPlaza.filters.clearSearch')"
        @click="$emit('update:search', '')"
      >
        <Icon name="x" size="xs" class="h-3.5 w-3.5" />
      </button>
    </div>

    <Select
      id="model-plaza-platform-filter"
      class="min-w-0"
      :model-value="platform"
      :options="platformOptions"
      :searchable="false"
      :aria-label="t('modelPlaza.filters.platformLabel')"
      match-trigger-width
      @update:model-value="updatePlatform"
    >
      <template #selected="{ option }">
        <span class="flex min-w-0 items-center gap-2">
          <PlatformIcon
            v-if="option && option.value !== 'all'"
            :platform="option.value as GroupPlatform"
            size="xs"
          />
          <span class="truncate">{{ option?.label ?? t('modelPlaza.filters.allPlatforms') }}</span>
        </span>
      </template>
      <template #option="{ option, selected }">
        <span class="flex w-full min-w-0 items-center gap-2">
          <PlatformIcon
            v-if="option.value !== 'all'"
            :platform="option.value as GroupPlatform"
            size="xs"
          />
          <span class="min-w-0 flex-1 truncate" :title="option.label">{{ option.label }}</span>
          <Icon v-if="selected" name="check" size="sm" class="shrink-0 text-emerald-500" />
        </span>
      </template>
    </Select>
    <Select
      id="model-plaza-group-filter"
      class="min-w-0"
      :model-value="groupId"
      :options="groupOptions"
      :searchable="false"
      :aria-label="t('modelPlaza.filters.groupLabel')"
      match-trigger-width
      @update:model-value="updateGroup"
    >
      <template #selected="{ option }">
        <span class="flex min-w-0 items-center gap-2">
          <PlatformIcon
            v-if="option?.platform"
            :platform="option.platform as GroupPlatform"
            size="xs"
          />
          <span class="truncate">{{ option?.label ?? t('modelPlaza.filters.allGroups') }}</span>
        </span>
      </template>
      <template #option="{ option, selected }">
        <span class="flex w-full min-w-0 items-center gap-2">
          <PlatformIcon
            v-if="option.platform"
            :platform="option.platform as GroupPlatform"
            size="xs"
          />
          <span class="min-w-0 flex-1 truncate" :title="option.label">{{ option.label }}</span>
          <Icon v-if="selected" name="check" size="sm" class="shrink-0 text-emerald-500" />
        </span>
      </template>
    </Select>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import type { GroupPlatform } from '@/types'

const props = defineProps<{
  /** 数据中出现的平台(去重排序后)。 */
  platforms: string[]
  /** 全量公开分组；平台和分组两个维度互相约束。 */
  groups: Array<{ id: number; name: string; platform: string }>
  platform: string
  groupId: number | 'all'
  /** 模型名搜索词(纯前端过滤)。 */
  search: string
}>()

const emit = defineEmits<{
  'update:platform': [value: string]
  'update:groupId': [value: number | 'all']
  'update:search': [value: string]
}>()

const { t } = useI18n()

/**
 * 平台和分组互为约束；「全部」始终保留为清除当前维度的出口。
 */
function platformEnabled(p: string): boolean {
  return props.groups.some(
    (g) =>
      g.platform === p &&
      (props.groupId === 'all' || g.id === props.groupId)
  )
}

function groupEnabled(g: { platform: string }): boolean {
  return props.platform === 'all' || g.platform === props.platform
}

const platformOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('modelPlaza.filters.allPlatforms') },
  ...props.platforms.map((platform) => ({
    value: platform,
    label: platform,
    disabled: !platformEnabled(platform),
  })),
])

const groupOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('modelPlaza.filters.allGroups'), platform: '' },
  ...props.groups.map((group) => ({
    value: group.id,
    label: group.name,
    platform: group.platform,
    disabled: !groupEnabled(group),
  })),
])

function updatePlatform(value: string | number | boolean | null): void {
  if (typeof value === 'string') {
    emit('update:platform', value)
  }
}

function updateGroup(value: string | number | boolean | null): void {
  if (value === 'all' || typeof value === 'number') {
    emit('update:groupId', value)
  }
}
</script>
