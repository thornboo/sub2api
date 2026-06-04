<template>
  <div class="table-wrapper">
    <table class="min-w-[1680px] table-fixed border-collapse text-sm">
      <thead>
        <tr>
          <th class="w-[260px] whitespace-nowrap" :aria-sort="sortAria('model')">
            <button type="button" class="sortable-header" @click="emit('sort', 'model')">
              {{ columns.model }}
              <Icon :name="sortIcon('model')" size="xs" />
            </button>
          </th>
          <th class="w-[130px] whitespace-nowrap" :aria-sort="sortAria('platform')">
            <button type="button" class="sortable-header" @click="emit('sort', 'platform')">
              {{ columns.platform }}
              <Icon :name="sortIcon('platform')" size="xs" />
            </button>
          </th>
          <th class="w-[220px] whitespace-nowrap" :aria-sort="sortAria('channel')">
            <button type="button" class="sortable-header" @click="emit('sort', 'channel')">
              {{ columns.channel }}
              <Icon :name="sortIcon('channel')" size="xs" />
            </button>
          </th>
          <th class="w-[120px] whitespace-nowrap" :aria-sort="sortAria('billingMode')">
            <button type="button" class="sortable-header" @click="emit('sort', 'billingMode')">
              {{ columns.billingMode }}
              <Icon :name="sortIcon('billingMode')" size="xs" />
            </button>
          </th>
          <th class="w-[150px] whitespace-nowrap" :aria-sort="sortAria('interval')">
            <button type="button" class="sortable-header" @click="emit('sort', 'interval')">
              <span class="price-header">
                {{ columns.interval }}
                <span class="price-header-tooltip">{{ tooltips.interval }}</span>
              </span>
              <Icon :name="sortIcon('interval')" size="xs" />
            </button>
          </th>
          <th class="w-[90px] whitespace-nowrap" :aria-sort="sortAria('inputPrice')">
            <button type="button" class="sortable-header" @click="emit('sort', 'inputPrice')">
              <span class="price-header">
                {{ columns.inputPrice }}
                <span class="price-header-tooltip">{{ tooltips.inputPrice }}</span>
              </span>
              <Icon :name="sortIcon('inputPrice')" size="xs" />
            </button>
          </th>
          <th class="w-[90px] whitespace-nowrap" :aria-sort="sortAria('outputPrice')">
            <button type="button" class="sortable-header" @click="emit('sort', 'outputPrice')">
              <span class="price-header">
                {{ columns.outputPrice }}
                <span class="price-header-tooltip">{{ tooltips.outputPrice }}</span>
              </span>
              <Icon :name="sortIcon('outputPrice')" size="xs" />
            </button>
          </th>
          <th class="w-[110px] whitespace-nowrap" :aria-sort="sortAria('cacheWritePrice')">
            <button type="button" class="sortable-header" @click="emit('sort', 'cacheWritePrice')">
              <span class="price-header">
                {{ columns.cacheWritePrice }}
                <span class="price-header-tooltip">{{ tooltips.cacheWritePrice }}</span>
              </span>
              <Icon :name="sortIcon('cacheWritePrice')" size="xs" />
            </button>
          </th>
          <th class="w-[110px] whitespace-nowrap" :aria-sort="sortAria('cacheReadPrice')">
            <button type="button" class="sortable-header" @click="emit('sort', 'cacheReadPrice')">
              <span class="price-header">
                {{ columns.cacheReadPrice }}
                <span class="price-header-tooltip">{{ tooltips.cacheReadPrice }}</span>
              </span>
              <Icon :name="sortIcon('cacheReadPrice')" size="xs" />
            </button>
          </th>
          <th class="w-[110px] whitespace-nowrap" :aria-sort="sortAria('imageOutputPrice')">
            <button type="button" class="sortable-header" @click="emit('sort', 'imageOutputPrice')">
              <span class="price-header">
                {{ columns.imageOutputPrice }}
                <span class="price-header-tooltip">{{ tooltips.imageOutputPrice }}</span>
              </span>
              <Icon :name="sortIcon('imageOutputPrice')" size="xs" />
            </button>
          </th>
          <th class="w-[90px] whitespace-nowrap" :aria-sort="sortAria('perRequestPrice')">
            <button type="button" class="sortable-header" @click="emit('sort', 'perRequestPrice')">
              <span class="price-header">
                {{ columns.perRequestPrice }}
                <span class="price-header-tooltip">{{ tooltips.perRequestPrice }}</span>
              </span>
              <Icon :name="sortIcon('perRequestPrice')" size="xs" />
            </button>
          </th>
          <th class="w-[300px] whitespace-nowrap">{{ columns.groups }}</th>
        </tr>
      </thead>

      <tbody v-if="loading">
        <tr>
          <td colspan="12" class="py-10 text-center">
            <Icon name="refresh" size="lg" class="inline-block animate-spin text-stone-400 dark:text-stone-500" />
          </td>
        </tr>
      </tbody>

      <tbody v-else-if="rows.length === 0">
        <tr>
          <td colspan="12" class="py-12 text-center">
            <Icon name="inbox" size="xl" class="mx-auto mb-3 h-12 w-12 text-stone-400 dark:text-stone-500" />
            <p class="text-sm text-stone-500 dark:text-stone-400">{{ emptyLabel }}</p>
          </td>
        </tr>
      </tbody>

      <tbody v-else>
        <tr
          v-for="row in rows"
          :key="row.id"
          class="transition-colors hover:bg-stone-50/70 dark:hover:bg-white/[0.04]"
        >
          <td class="whitespace-nowrap">
            <div class="flex min-w-0 items-center gap-2">
              <PlatformIcon :platform="row.platform as GroupPlatform" size="sm" class="shrink-0" />
              <span class="truncate font-semibold text-stone-950 dark:text-white" :title="row.modelName">
                {{ row.modelName }}
              </span>
            </div>
          </td>

          <td class="whitespace-nowrap">
            <span
              :class="[
                'inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-medium uppercase',
                platformBadgeClass(row.platform),
              ]"
            >
              <PlatformIcon :platform="row.platform as GroupPlatform" size="xs" />
              {{ row.platform }}
            </span>
          </td>

          <td class="whitespace-nowrap">
            <div class="min-w-0">
              <div class="truncate font-medium text-stone-900 dark:text-stone-100" :title="row.channelName">
                {{ row.channelName }}
              </div>
              <div
                v-if="row.channelDescription"
                class="mt-0.5 truncate text-xs text-stone-500 dark:text-stone-400"
                :title="row.channelDescription"
              >
                {{ row.channelDescription }}
              </div>
            </div>
          </td>

          <td class="whitespace-nowrap">
            <span class="font-medium text-stone-800 dark:text-stone-200">
              {{ formatBillingMode(row.pricing, pricingLabels) }}
            </span>
          </td>
          <td class="whitespace-nowrap">
            <span class="block truncate text-xs text-stone-600 dark:text-stone-300" :title="row.intervalLabel">
              {{ row.intervalLabel }}
            </span>
          </td>
          <td class="whitespace-nowrap font-mono text-xs">{{ formatCompactTokenPrice(getRowInputPrice(row)) }}</td>
          <td class="whitespace-nowrap font-mono text-xs">{{ formatCompactTokenPrice(getRowOutputPrice(row)) }}</td>
          <td class="whitespace-nowrap font-mono text-xs">{{ formatCompactTokenPrice(getRowCacheWritePrice(row)) }}</td>
          <td class="whitespace-nowrap font-mono text-xs">{{ formatCompactTokenPrice(getRowCacheReadPrice(row)) }}</td>
          <td class="whitespace-nowrap font-mono text-xs">{{ formatCompactRequestPrice(getRowImageOutputPrice(row)) }}</td>
          <td class="whitespace-nowrap font-mono text-xs">{{ formatCompactRequestPrice(getRowPerRequestPrice(row)) }}</td>

          <td class="whitespace-nowrap">
            <div v-if="row.groups.length > 0" class="flex flex-nowrap gap-1 overflow-hidden">
              <GroupBadge
                v-for="group in row.groups"
                :key="group.id"
                :name="group.name"
                :platform="group.platform as GroupPlatform"
                :subscription-type="(group.subscription_type || 'standard') as SubscriptionType"
                :rate-multiplier="group.rate_multiplier"
                :user-rate-multiplier="userGroupRates[group.id] ?? null"
                always-show-rate
              />
            </div>
            <span v-else class="text-xs text-stone-400">-</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { platformBadgeClass } from '@/utils/platformColors'
import {
  formatBillingMode,
  formatCompactRequestPrice,
  formatCompactTokenPrice,
  getRowCacheReadPrice,
  getRowCacheWritePrice,
  getRowImageOutputPrice,
  getRowInputPrice,
  getRowOutputPrice,
  getRowPerRequestPrice,
  type AvailableChannelCatalogRow,
  type AvailableChannelPricingLabels,
  type AvailableChannelSortKey,
  type AvailableChannelSortOrder,
} from '@/utils/availableChannelsCatalog'

const props = defineProps<{
  columns: {
    model: string
    platform: string
    channel: string
    billingMode: string
    interval: string
    inputPrice: string
    outputPrice: string
    cacheWritePrice: string
    cacheReadPrice: string
    imageOutputPrice: string
    perRequestPrice: string
    groups: string
  }
  tooltips: {
    interval: string
    inputPrice: string
    outputPrice: string
    cacheWritePrice: string
    cacheReadPrice: string
    imageOutputPrice: string
    perRequestPrice: string
  }
  rows: AvailableChannelCatalogRow[]
  loading: boolean
  emptyLabel: string
  pricingLabels: AvailableChannelPricingLabels
  userGroupRates: Record<number, number>
  sortBy: AvailableChannelSortKey
  sortOrder: AvailableChannelSortOrder
}>()

const emit = defineEmits<{
  sort: [key: AvailableChannelSortKey]
}>()

function sortIcon(key: AvailableChannelSortKey): 'arrowUp' | 'arrowDown' | 'arrowsUpDown' {
  if (props.sortBy !== key) return 'arrowsUpDown'
  return props.sortOrder === 'asc' ? 'arrowUp' : 'arrowDown'
}

function sortAria(key: AvailableChannelSortKey): 'ascending' | 'descending' | 'none' {
  if (props.sortBy !== key) return 'none'
  return props.sortOrder === 'asc' ? 'ascending' : 'descending'
}
</script>

<style scoped>
.sortable-header {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 0.375rem;
  white-space: nowrap;
  color: inherit;
  transition: color 120ms ease;
}

.sortable-header:hover {
  color: rgb(87 83 78);
}

:global(.dark) .sortable-header:hover {
  color: rgb(245 245 244);
}

.price-header {
  position: relative;
  display: inline-flex;
  cursor: help;
  align-items: center;
  border-bottom: 1px dotted currentColor;
  line-height: 1.2;
}

.price-header-tooltip {
  position: absolute;
  left: 50%;
  top: calc(100% + 0.5rem);
  z-index: 30;
  width: max-content;
  max-width: 18rem;
  transform: translateX(-50%);
  border-radius: 0.5rem;
  border: 1px solid rgba(68, 64, 60, 0.75);
  background: rgba(12, 10, 9, 0.96);
  padding: 0.5rem 0.625rem;
  color: white;
  font-size: 0.75rem;
  font-weight: 500;
  line-height: 1.35;
  opacity: 0;
  pointer-events: none;
  white-space: normal;
  box-shadow: 0 14px 32px rgba(0, 0, 0, 0.35);
  transition: opacity 120ms ease, transform 120ms ease;
}

.price-header:hover .price-header-tooltip,
.price-header:focus-within .price-header-tooltip {
  opacity: 1;
  transform: translateX(-50%) translateY(2px);
}
</style>
