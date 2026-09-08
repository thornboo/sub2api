<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { PopoverClose } from 'reka-ui'
import type { UserPricingInterval, UserSupportedModelPricing } from '@/api/channels'
import Icon from '@/components/icons/Icon.vue'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { BILLING_MODE_TOKEN } from '@/constants/channel'
import {
  formatAvailableChannelIntervalLabel,
  formatCompactRequestPrice,
  formatCompactTokenPrice,
  hasEnabledTimePricing,
  rowHasPricing,
} from '@/utils/availableChannelsCatalog'

// Prices are already adjusted by the marketplace's applicable rate policy.
const props = defineProps<{
  modelName: string
  pricing: UserSupportedModelPricing
  unit: string
}>()

const { t } = useI18n()
const isToken = computed(() => props.pricing.billing_mode === BILLING_MODE_TOKEN)
const intervals = computed(() => props.pricing.intervals.filter(interval => rowHasPricing({ pricing: props.pricing, interval })))
type TokenPriceKey = keyof Pick<UserPricingInterval, 'input_price' | 'output_price' | 'cache_write_price' | 'cache_write_1h_price' | 'cache_read_price'>
const tokenColumns = computed(() => {
  const columns: { key: TokenPriceKey; label: string }[] = [
    { key: 'input_price', label: t('availableChannels.modelTable.columns.inputPrice') },
    { key: 'output_price', label: t('availableChannels.modelTable.columns.outputPrice') },
    { key: 'cache_write_price', label: t('availableChannels.modelTable.columns.cacheWritePrice') },
    { key: 'cache_write_1h_price', label: t('availableChannels.modelMarketplace.timePricing.cacheWrite1h') },
    { key: 'cache_read_price', label: t('availableChannels.modelTable.columns.cacheReadPrice') },
  ]
  return columns.filter((column, index) => index < 2 || intervals.value.some(interval => interval[column.key] != null))
})
</script>

<template>
  <Popover v-if="intervals.length">
    <PopoverTrigger as-child>
      <button
        type="button"
        data-testid="tier-pricing-trigger"
        class="group/tier-pricing mt-2.5 flex w-full items-center justify-between gap-2 rounded-lg border border-stone-200/80 bg-white/70 px-2 py-1.5 text-[10px] font-semibold text-stone-600 transition hover:border-stone-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/50 dark:border-white/[0.08] dark:bg-white/[0.03] dark:text-stone-300 dark:hover:border-white/20"
      >
        <span class="inline-flex min-w-0 items-center gap-1.5">
          <Icon name="chart" size="xs" class="shrink-0 text-emerald-600 dark:text-emerald-300" />
          <span class="truncate">{{ t('availableChannels.modelMarketplace.tieredPricing') }}</span>
          <span class="truncate font-normal text-stone-400 dark:text-stone-500">{{ t('availableChannels.modelMarketplace.tierPricing.count', { count: intervals.length }) }}</span>
        </span>
        <Icon name="chevronDown" size="xs" class="shrink-0 text-stone-400 transition-transform group-data-[state=open]/tier-pricing:rotate-180" />
      </button>
    </PopoverTrigger>

    <PopoverContent
      data-testid="tier-pricing-popover"
      side="bottom"
      align="start"
      :collision-padding="16"
      :aria-label="`${modelName} · ${t('availableChannels.modelMarketplace.tieredPricing')}`"
      :class="[
        'flex max-h-[min(80dvh,var(--reka-popover-content-available-height))] max-w-[calc(100vw-2rem)] flex-col overflow-hidden rounded-xl',
        isToken && tokenColumns.length > 2 ? 'w-[42rem]' : 'w-[32rem]',
      ]"
    >
      <div class="flex shrink-0 items-start justify-between gap-3 border-b border-stone-200/80 px-3 py-2.5 dark:border-white/[0.08]">
        <div class="min-w-0">
          <p class="text-xs font-semibold">
            {{ t('availableChannels.modelMarketplace.tieredPricing') }}
            <span class="ml-1 font-normal text-stone-500 dark:text-stone-400">{{ unit }}</span>
          </p>
          <p class="mt-0.5 break-words font-mono text-[10px] text-stone-500 dark:text-stone-400">{{ modelName }}</p>
          <p v-if="hasEnabledTimePricing(pricing)" class="mt-1 text-[10px] text-stone-500 dark:text-stone-400">
            {{ t('availableChannels.modelMarketplace.tierPricing.currentTimePrices') }} · {{ pricing.time_pricing?.timezone }}
          </p>
        </div>
        <PopoverClose
          :aria-label="t('common.close')"
          class="shrink-0 rounded p-1 text-stone-400 transition hover:bg-stone-100 hover:text-stone-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/50 dark:hover:bg-white/10 dark:hover:text-stone-200"
        >
          <Icon name="x" size="sm" aria-hidden="true" />
        </PopoverClose>
      </div>
      <div class="min-h-0 overflow-auto overscroll-contain">
        <table :class="['w-full text-left text-xs', isToken && tokenColumns.length > 2 ? 'min-w-[32rem]' : 'min-w-[18rem]']">
          <thead class="bg-stone-50/80 text-[10px] text-stone-500 dark:bg-black/10 dark:text-stone-400">
            <tr>
              <th scope="col" class="whitespace-nowrap px-3 py-2 font-medium">{{ t(`availableChannels.modelMarketplace.tierPricing.${isToken ? 'range' : 'tier'}`) }}</th>
              <template v-if="isToken">
                <th v-for="column in tokenColumns" :key="column.key" scope="col" class="whitespace-nowrap px-3 py-2 text-right font-medium">{{ column.label }}</th>
              </template>
              <th v-else scope="col" class="whitespace-nowrap px-3 py-2 text-right font-medium">{{ t('availableChannels.modelMarketplace.tierPricing.price') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-stone-200/70 dark:divide-white/[0.07]">
            <tr v-for="(interval, index) in intervals" :key="index" data-testid="tier-pricing-row">
              <th scope="row" class="px-3 py-2.5 text-left font-mono font-medium tabular-nums text-stone-700 dark:text-stone-200">
                {{ formatAvailableChannelIntervalLabel(interval) }}
              </th>
              <template v-if="isToken">
                <td v-for="column in tokenColumns" :key="column.key" class="whitespace-nowrap px-3 py-2.5 text-right font-mono tabular-nums text-stone-800 dark:text-stone-100">{{ formatCompactTokenPrice(interval[column.key] ?? null) }}</td>
              </template>
              <td v-else class="whitespace-nowrap px-3 py-2.5 text-right font-mono tabular-nums text-stone-800 dark:text-stone-100">{{ formatCompactRequestPrice(interval.per_request_price) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </PopoverContent>
  </Popover>
</template>
