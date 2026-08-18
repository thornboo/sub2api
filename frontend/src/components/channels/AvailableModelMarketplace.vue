<template>
  <div class="p-2.5 sm:p-3">
    <div
      v-if="loading"
      class="grid grid-cols-1 gap-2.5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4"
      aria-busy="true"
      :aria-label="t('common.loading')"
    >
      <div
        v-for="index in 6"
        :key="index"
        class="h-48 animate-pulse rounded-xl border border-stone-200/80 bg-stone-50/70 dark:border-white/10 dark:bg-white/[0.025]"
      />
    </div>

    <div v-else-if="cards.length === 0" class="py-16 text-center">
      <div class="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-stone-200 bg-stone-50 dark:border-white/10 dark:bg-white/[0.035]">
        <Icon name="inbox" size="xl" class="text-stone-400 dark:text-stone-500" />
      </div>
      <p class="mt-4 text-sm text-stone-500 dark:text-stone-400">{{ emptyLabel }}</p>
    </div>

    <div v-else class="space-y-5">
      <section
        v-for="section in groupSections"
        :key="section.group.id"
        data-testid="available-model-group-section"
        :data-group-id="section.group.id"
        :aria-labelledby="`available-model-group-${section.group.id}`"
      >
        <div class="mb-2.5 flex min-w-0 items-center gap-2.5 px-0.5">
          <span class="h-5 w-1 shrink-0 rounded-full bg-emerald-500/80 dark:bg-emerald-400/70" />
          <h2 :id="`available-model-group-${section.group.id}`" class="min-w-0 shrink-0">
            <GroupBadge
              :name="section.group.name"
              :platform="section.group.platform as GroupPlatform"
              :subscription-type="(section.group.subscription_type || 'standard') as SubscriptionType"
              :show-rate="false"
            />
          </h2>
          <span class="shrink-0 text-[10px] font-medium text-stone-400 dark:text-stone-500">
            {{ t('availableChannels.modelMarketplace.groupModelCount', { count: section.cards.length }) }}
          </span>
          <span class="h-px min-w-4 flex-1 bg-stone-200/80 dark:bg-white/[0.08]" />
        </div>
        <p
          v-if="section.group.description"
          class="-mt-1 mb-2.5 line-clamp-2 px-0.5 text-xs leading-5 text-stone-500 dark:text-stone-400"
        >
          {{ section.group.description }}
        </p>

        <div class="grid grid-cols-1 gap-2.5 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          <article
            v-for="card in section.cards"
            :key="card.id"
            data-testid="available-model-card"
            class="group flex min-w-0 flex-col rounded-xl border border-stone-200/80 bg-white/90 p-3 shadow-sm shadow-stone-950/[0.025] transition duration-150 hover:border-stone-300 hover:shadow-md hover:shadow-stone-950/[0.05] dark:border-white/[0.09] dark:bg-white/[0.025] dark:shadow-black/20 dark:hover:border-white/15 dark:hover:bg-white/[0.04]"
            :aria-label="t('availableChannels.modelMarketplace.groupCardLabel', { name: card.name, group: card.group.name })"
          >
            <header class="flex min-w-0 items-start gap-2.5">
              <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-stone-200 bg-stone-50 dark:border-white/10 dark:bg-white/[0.06]">
                <ModelIcon :model="card.name" size="20px" />
              </div>

              <div class="min-w-0 flex-1">
                <div class="flex min-w-0 items-center gap-1">
                  <h3 class="truncate font-mono text-[13px] font-bold leading-5 text-stone-950 dark:text-white" :title="card.name">
                    {{ card.name }}
                  </h3>
                  <button
                    type="button"
                    class="shrink-0 rounded p-0.5 text-stone-400 opacity-60 transition hover:bg-stone-100 hover:text-stone-700 group-hover:opacity-100 focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-500/50 dark:hover:bg-white/[0.08] dark:hover:text-stone-200"
                    :aria-label="t('availableChannels.modelMarketplace.copyModel', { name: card.name })"
                    :title="t('availableChannels.modelMarketplace.copyModel', { name: card.name })"
                    @click="copyModel(card.name)"
                  >
                    <Icon name="copy" size="xs" />
                  </button>
                </div>

                <div class="mt-1 flex min-w-0 items-center gap-1.5 overflow-hidden">
                  <span
                    v-for="platform in card.platforms"
                    :key="platform"
                    :class="[
                      'inline-flex shrink-0 items-center gap-1 rounded border px-1.5 py-px text-[9px] font-semibold uppercase tracking-wide',
                      platformBadgeClass(platform),
                    ]"
                  >
                    <PlatformIcon :platform="platform as GroupPlatform" size="xs" />
                    {{ platformLabel(platform) }}
                  </span>
                  <span class="truncate text-[10px] text-stone-400 dark:text-stone-500">
                    {{ t('availableChannels.modelMarketplace.channelCount', { count: card.channelNames.length }) }}
                  </span>
                </div>
              </div>
            </header>

            <section class="mt-2.5 rounded-lg bg-stone-50 px-2.5 py-2.5 dark:bg-black/15">
              <template v-if="card.pricingOptions.length === 1">
                <div v-if="card.pricingOptions[0]">
                  <template v-if="card.pricingOptions[0]?.billing_mode === BILLING_MODE_TOKEN">
                    <div class="grid grid-cols-2 divide-x divide-stone-200/80 dark:divide-white/[0.08]">
                      <div class="min-w-0 pr-2.5">
                        <div class="text-[10px] font-medium text-stone-500 dark:text-stone-400">
                          {{ t('availableChannels.pricing.inputPrice') }}
                        </div>
                        <strong
                          data-testid="effective-input-price"
                          class="mt-0.5 block truncate font-mono text-lg font-bold leading-6 tracking-tight text-stone-950 dark:text-stone-100"
                        >
                          {{ formatCompactTokenPrice(displayPrice(card, card.pricingOptions[0]?.input_price ?? null, card.pricingOptions[0])) }}
                        </strong>
                        <div
                          v-if="showOriginalPrice(card, card.pricingOptions[0]?.input_price ?? null, card.pricingOptions[0])"
                          class="mt-0.5 flex min-w-0 items-baseline gap-1 text-[10px] text-stone-400 dark:text-stone-500"
                        >
                          <span>{{ t('availableChannels.modelMarketplace.originalPrice') }}</span>
                          <del
                            data-testid="original-input-price"
                            class="truncate font-mono decoration-stone-400/80 dark:decoration-stone-500"
                          >
                            {{ formatCompactTokenPrice(card.pricingOptions[0]?.input_price ?? null) }}
                          </del>
                        </div>
                      </div>

                      <div class="min-w-0 pl-2.5">
                        <div class="text-[10px] font-medium text-stone-500 dark:text-stone-400">
                          {{ t('availableChannels.pricing.outputPrice') }}
                        </div>
                        <strong
                          data-testid="effective-output-price"
                          class="mt-0.5 block truncate font-mono text-lg font-bold leading-6 tracking-tight text-stone-950 dark:text-stone-100"
                        >
                          {{ formatCompactTokenPrice(displayPrice(card, card.pricingOptions[0]?.output_price ?? null, card.pricingOptions[0])) }}
                        </strong>
                        <div
                          v-if="showOriginalPrice(card, card.pricingOptions[0]?.output_price ?? null, card.pricingOptions[0])"
                          class="mt-0.5 flex min-w-0 items-baseline gap-1 text-[10px] text-stone-400 dark:text-stone-500"
                        >
                          <span>{{ t('availableChannels.modelMarketplace.originalPrice') }}</span>
                          <del
                            data-testid="original-output-price"
                            class="truncate font-mono decoration-stone-400/80 dark:decoration-stone-500"
                          >
                            {{ formatCompactTokenPrice(card.pricingOptions[0]?.output_price ?? null) }}
                          </del>
                        </div>
                      </div>
                    </div>

                    <div class="mt-2.5 flex min-w-0 items-center justify-between gap-2 border-t border-stone-200/80 pt-2 dark:border-white/[0.08]">
                      <div class="min-w-0 truncate text-[9px] text-stone-400 dark:text-stone-500">
                        <span class="font-semibold uppercase tracking-[0.1em]">
                          {{ formatBillingMode(card.pricingOptions[0], pricingLabels) }}
                        </span>
                        <span class="ml-1">{{ pricingUnit(card.pricingOptions[0]) }}</span>
                      </div>

                      <span
                        v-if="isDiscountedPrice(card, card.pricingOptions[0])"
                        data-testid="price-discount"
                        class="shrink-0 rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300"
                      >
                        {{ t('availableChannels.modelMarketplace.savings', { percent: discountPercent(card, card.pricingOptions[0]) }) }}
                      </span>
                    </div>

                    <p
                      v-if="hasTieredPricing(card.pricingOptions[0])"
                      class="mt-1.5 truncate text-[9px] text-stone-500 dark:text-stone-400"
                      :title="tieredPricing(card, card.pricingOptions[0])"
                    >
                      {{ t('availableChannels.modelMarketplace.tieredPricing') }} · {{ tieredPricing(card, card.pricingOptions[0]) }}
                    </p>

                    <details
                      v-if="hasTimePricing(card.pricingOptions[0])"
                      class="mt-2.5 rounded-lg border border-stone-200/80 bg-white/70 dark:border-white/[0.08] dark:bg-white/[0.03]"
                    >
                      <summary class="flex cursor-pointer list-none items-center justify-between gap-2 px-2 py-1.5 text-[10px] font-semibold text-stone-600 marker:hidden dark:text-stone-300">
                        <span class="inline-flex min-w-0 items-center gap-1.5">
                          <Icon name="clock" size="xs" class="shrink-0 text-emerald-600 dark:text-emerald-300" />
                          <span class="truncate">{{ t('availableChannels.modelMarketplace.timePricing.title') }}</span>
                          <span class="truncate font-mono text-stone-400 dark:text-stone-500">
                            {{ card.pricingOptions[0].time_pricing?.timezone }}
                          </span>
                        </span>
                        <Icon name="chevronDown" size="xs" class="shrink-0 text-stone-400" />
                      </summary>

                      <div class="overflow-x-auto border-t border-stone-200/80 dark:border-white/[0.08]">
                        <table class="min-w-full text-left text-[10px]">
                          <thead class="bg-stone-50/80 text-[9px] uppercase tracking-wide text-stone-400 dark:bg-black/10 dark:text-stone-500">
                            <tr>
                              <th class="whitespace-nowrap px-2 py-1 font-semibold">{{ t('availableChannels.modelMarketplace.timePricing.window') }}</th>
                              <th class="whitespace-nowrap px-2 py-1 font-semibold">{{ t('availableChannels.modelMarketplace.timePricing.type') }}</th>
                              <th class="whitespace-nowrap px-2 py-1 text-right font-semibold">{{ t('availableChannels.pricing.inputPrice') }}</th>
                              <th class="whitespace-nowrap px-2 py-1 text-right font-semibold">{{ t('availableChannels.pricing.outputPrice') }}</th>
                              <th class="whitespace-nowrap px-2 py-1 text-right font-semibold">{{ t('availableChannels.modelMarketplace.timePricing.cacheWrite') }}</th>
                              <th class="whitespace-nowrap px-2 py-1 text-right font-semibold">{{ t('availableChannels.modelMarketplace.timePricing.cacheRead') }}</th>
                            </tr>
                          </thead>
                          <tbody class="divide-y divide-stone-200/70 dark:divide-white/[0.07]">
                            <tr
                              v-for="row in timePricingRows(card, card.pricingOptions[0])"
                              :key="row.id"
                              :class="[
                                row.active ? 'bg-emerald-50/80 dark:bg-emerald-500/10' : '',
                              ]"
                              data-testid="time-pricing-row"
                            >
                              <td class="whitespace-nowrap px-2 py-1.5 font-mono text-stone-700 dark:text-stone-200">
                                <span class="inline-flex items-center gap-1">
                                  <Icon
                                    v-if="row.active"
                                    name="checkCircle"
                                    size="xs"
                                    class="shrink-0 text-emerald-600 dark:text-emerald-300"
                                  />
                                  <span>{{ row.windowLabel }}</span>
                                  <span v-if="row.active" class="font-sans text-[9px] font-semibold text-emerald-700 dark:text-emerald-300">
                                    {{ t('availableChannels.modelMarketplace.timePricing.active') }}
                                  </span>
                                </span>
                              </td>
                              <td class="whitespace-nowrap px-2 py-1.5">
                                <span class="inline-flex items-center rounded-full bg-stone-100 px-1.5 py-0.5 text-[9px] font-semibold text-stone-600 dark:bg-white/[0.07] dark:text-stone-300">
                                  {{ row.label }} · {{ formatRateMultiplier(row.multiplier) }}
                                </span>
                              </td>
                              <td class="whitespace-nowrap px-2 py-1.5 text-right font-mono text-stone-800 dark:text-stone-100">{{ formatTimePricingTokenPrice(row.inputPrice) }}</td>
                              <td class="whitespace-nowrap px-2 py-1.5 text-right font-mono text-stone-800 dark:text-stone-100">{{ formatTimePricingTokenPrice(row.outputPrice) }}</td>
                              <td class="whitespace-nowrap px-2 py-1.5 text-right font-mono text-stone-800 dark:text-stone-100">{{ formatTimePricingTokenPrice(row.cacheWritePrice) }}</td>
                              <td class="whitespace-nowrap px-2 py-1.5 text-right font-mono text-stone-800 dark:text-stone-100">{{ formatTimePricingTokenPrice(row.cacheReadPrice) }}</td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                    </details>
                  </template>

                  <div v-else class="flex min-h-7 items-center justify-between gap-2">
                    <div class="min-w-0">
                      <span class="text-[9px] font-semibold uppercase tracking-[0.1em] text-stone-400 dark:text-stone-500">
                        {{ formatBillingMode(card.pricingOptions[0], pricingLabels) }}
                      </span>
                      <span class="ml-1 text-[9px] text-stone-400 dark:text-stone-500">{{ pricingUnit(card.pricingOptions[0]) }}</span>
                      <p v-if="hasTieredPricing(card.pricingOptions[0])" class="mt-0.5 truncate text-[9px] text-stone-500 dark:text-stone-400" :title="tieredPricing(card, card.pricingOptions[0])">
                        {{ t('availableChannels.modelMarketplace.tieredPricing') }} · {{ tieredPricing(card, card.pricingOptions[0]) }}
                      </p>
                    </div>
                    <div class="shrink-0 font-mono text-[13px] font-semibold text-stone-950 dark:text-stone-100">
                      {{ requestPrice(card, card.pricingOptions[0]) }}
                    </div>
                  </div>
                </div>
                <div v-else class="flex min-h-7 items-center text-xs text-stone-500 dark:text-stone-400">
                  {{ pricingLabels.noPricing }}
                </div>
              </template>

              <div v-else class="flex min-h-7 items-center justify-between gap-2">
                <div class="min-w-0">
                  <div class="text-xs font-semibold text-stone-800 dark:text-stone-200">
                    {{ t('availableChannels.modelMarketplace.priceVariants', { count: card.pricingOptions.length }) }}
                  </div>
                  <p class="truncate text-[9px] text-stone-500 dark:text-stone-400">
                    {{ t('availableChannels.modelMarketplace.priceVariantsHint') }}
                  </p>
                </div>
                <Icon name="arrowsUpDown" size="sm" class="shrink-0 text-stone-400" />
              </div>
            </section>

            <div class="mt-2.5 flex-1">
              <section class="flex min-w-0 items-center gap-2">
                <span class="w-10 shrink-0 whitespace-nowrap text-[10px] text-stone-400 dark:text-stone-500">{{ t('availableChannels.modelMarketplace.availableChannels') }}</span>
                <div class="flex min-w-0 flex-1 items-center gap-1 overflow-hidden" :title="card.channelNames.join(', ')">
                  <span
                    v-for="channel in visibleChannels(card)"
                    :key="channel"
                    class="inline-flex min-w-0 max-w-[8rem] items-center gap-1 rounded bg-stone-100 px-1.5 py-0.5 text-[10px] font-medium text-stone-600 dark:bg-white/[0.06] dark:text-stone-300"
                  >
                    <Icon name="server" size="xs" class="shrink-0" />
                    <span class="truncate">{{ channel }}</span>
                  </span>
                  <span v-if="hiddenChannelCount(card) > 0" class="shrink-0 rounded bg-stone-100 px-1.5 py-0.5 text-[10px] text-stone-500 dark:bg-white/[0.06] dark:text-stone-400">
                    +{{ hiddenChannelCount(card) }}
                  </span>
                </div>
              </section>
            </div>

            <footer class="mt-2.5 flex min-w-0 items-start gap-2 border-t border-stone-200/80 pt-2 dark:border-white/[0.08]">
              <span class="w-10 shrink-0 whitespace-nowrap pt-0.5 text-[10px] text-stone-400 dark:text-stone-500">{{ t('availableChannels.modelMarketplace.apiEndpoints') }}</span>
              <div v-if="card.endpoints.length" class="flex min-w-0 flex-1 flex-wrap items-center gap-1">
                <button
                  v-for="endpoint in card.endpoints"
                  :key="`${endpoint.protocol}:${endpoint.path}`"
                  type="button"
                  :class="[
                    'inline-flex shrink-0 items-center gap-1 rounded border px-1.5 py-0.5 font-mono text-[9px] font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-1 dark:focus-visible:ring-offset-stone-950',
                    endpointClass(endpoint.protocol),
                  ]"
                  :aria-label="t('availableChannels.endpoints.copyHint', { path: endpoint.path })"
                  :title="`${endpointLabel(endpoint.protocol)} · ${endpoint.path}`"
                  @click="copyEndpoint(endpoint.path)"
                >
                  <span class="h-1 w-1 rounded-full bg-current opacity-70" />
                  {{ endpoint.path }}
                </button>
              </div>
              <div v-else class="flex min-w-0 items-center gap-1.5 text-[10px] text-stone-400 dark:text-stone-500">
                <span class="h-1 w-1 shrink-0 rounded-full bg-stone-300 dark:bg-stone-600" />
                {{ t('availableChannels.modelMarketplace.endpointUnavailable') }}
              </div>
            </footer>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import type { UserAvailableGroup, UserSupportedEndpoint, UserSupportedModelPricing } from '@/api/channels'
import ModelIcon from '@/components/common/ModelIcon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  BILLING_MODE_TOKEN,
} from '@/constants/channel'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useClipboard } from '@/composables/useClipboard'
import {
  buildTimePricingDisplayRows,
  formatAvailableChannelIntervals,
  formatBillingMode,
  formatCompactRequestPrice,
  formatCompactTokenPrice,
  formatRateMultiplier,
  formatTimePricingTokenPrice,
  getActiveTimePricingMultiplier,
  hasEnabledTimePricing,
  resolveAvailableGroupPriceMultiplier,
  type TimePricingDisplayRow,
  type AvailableChannelPricingLabels,
} from '@/utils/availableChannelsCatalog'
import type { AvailableModelMarketplaceCard } from '@/utils/availableModelMarketplace'
import { platformBadgeClass, platformLabel } from '@/utils/platformColors'

const props = defineProps<{
  cards: AvailableModelMarketplaceCard[]
  loading: boolean
  pricingLabels: AvailableChannelPricingLabels
  emptyLabel: string
  userGroupRates: Record<number, number>
  applyRateMultiplier?: boolean
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const MAX_VISIBLE_CHANNELS = 2
const RATE_COMPARISON_EPSILON = 1e-9

interface AvailableModelGroupSection {
  group: UserAvailableGroup
  cards: AvailableModelMarketplaceCard[]
}

const groupSections = computed<AvailableModelGroupSection[]>(() => {
  const sections = new Map<number, AvailableModelGroupSection>()
  props.cards.forEach((card) => {
    const section = sections.get(card.group.id) ?? { group: card.group, cards: [] }
    section.cards.push(card)
    sections.set(card.group.id, section)
  })
  return Array.from(sections.values())
})

function visibleChannels(card: AvailableModelMarketplaceCard): string[] {
  return card.channelNames.slice(0, MAX_VISIBLE_CHANNELS)
}

function hiddenChannelCount(card: AvailableModelMarketplaceCard): number {
  return Math.max(card.channelNames.length - MAX_VISIBLE_CHANNELS, 0)
}

function pricingUnit(pricing: UserSupportedModelPricing): string {
  return pricing.billing_mode === BILLING_MODE_TOKEN
    ? props.pricingLabels.unitPerMillion
    : props.pricingLabels.unitPerRequest
}

function requestPrice(card: AvailableModelMarketplaceCard, pricing: UserSupportedModelPricing): string {
  const value = pricing.per_request_price
  return `${formatCompactRequestPrice(displayPrice(card, value, pricing))} ${props.pricingLabels.unitPerRequest}`
}

function hasTieredPricing(pricing: UserSupportedModelPricing): boolean {
  return pricing.intervals.length > 0
}

function tieredPricing(
  card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing,
): string {
  return formatAvailableChannelIntervals(
    scalePricing(pricing, effectiveDisplayMultiplier(card, pricing)),
    props.pricingLabels,
    { compact: true },
  )
}

function displayPrice(
  card: AvailableModelMarketplaceCard,
  value: number | null,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): number | null {
  return value == null ? null : value * effectiveDisplayMultiplier(card, pricing)
}

function priceMultiplier(
  card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): number {
  if (!props.applyRateMultiplier) return 1
  return resolveAvailableGroupPriceMultiplier(
    card.group,
    props.userGroupRates,
    pricing?.billing_mode,
  )
}

function effectiveDisplayMultiplier(
  card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): number {
  if (hasTimePricing(pricing)) {
    return activeTimePricingMultiplier(pricing)
  }
  return priceMultiplier(card, pricing)
}

function hasAdjustedPrice(
  card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): boolean {
  return Math.abs(effectiveDisplayMultiplier(card, pricing) - 1) > RATE_COMPARISON_EPSILON
}

function showOriginalPrice(
  card: AvailableModelMarketplaceCard,
  value: number | null,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): boolean {
  return value != null && hasAdjustedPrice(card, pricing)
}

function isDiscountedPrice(
  card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): boolean {
  return hasAdjustedPrice(card, pricing) && effectiveDisplayMultiplier(card, pricing) < 1
}

function discountPercent(
  card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing | null = card.pricingOptions[0] ?? null,
): string {
  const percentage = Math.max(0, (1 - effectiveDisplayMultiplier(card, pricing)) * 100)
  return Number(percentage.toFixed(2)).toString()
}

function hasTimePricing(pricing: UserSupportedModelPricing | null): boolean {
  return hasEnabledTimePricing(pricing)
}

function activeTimePricingMultiplier(pricing: UserSupportedModelPricing | null): number {
  return getActiveTimePricingMultiplier(pricing)
}

function timePricingRows(
  _card: AvailableModelMarketplaceCard,
  pricing: UserSupportedModelPricing,
): TimePricingDisplayRow[] {
  return buildTimePricingDisplayRows(
    pricing,
    {
      otherTimes: t('availableChannels.modelMarketplace.timePricing.otherTimes'),
      unnamedType: t('availableChannels.modelMarketplace.timePricing.unnamedType'),
    },
  )
}

function scalePricing(
  pricing: UserSupportedModelPricing,
  multiplier: number,
): UserSupportedModelPricing {
  const scale = (value: number | null) => (value == null ? null : value * multiplier)
  return {
    ...pricing,
    input_price: scale(pricing.input_price),
    output_price: scale(pricing.output_price),
    cache_write_price: scale(pricing.cache_write_price),
    cache_read_price: scale(pricing.cache_read_price),
    image_input_price: scale(pricing.image_input_price),
    image_output_price: scale(pricing.image_output_price),
    per_request_price: scale(pricing.per_request_price),
    intervals: pricing.intervals.map((interval) => ({
      ...interval,
      input_price: scale(interval.input_price),
      output_price: scale(interval.output_price),
      cache_write_price: scale(interval.cache_write_price),
      cache_read_price: scale(interval.cache_read_price),
      per_request_price: scale(interval.per_request_price),
    })),
  }
}

function endpointLabel(protocol: UserSupportedEndpoint['protocol']): string {
  switch (protocol) {
    case 'anthropic_messages':
      return 'Messages'
    case 'openai_chat_completions':
      return 'Chat'
    case 'openai_responses':
      return 'Responses'
  }
}

function endpointClass(protocol: UserSupportedEndpoint['protocol']): string {
  switch (protocol) {
    case 'anthropic_messages':
      return 'border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100 focus-visible:ring-amber-500/40 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300 dark:hover:bg-amber-500/15'
    case 'openai_chat_completions':
      return 'border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100 focus-visible:ring-emerald-500/40 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300 dark:hover:bg-emerald-500/15'
    case 'openai_responses':
      return 'border-sky-200 bg-sky-50 text-sky-700 hover:bg-sky-100 focus-visible:ring-sky-500/40 dark:border-sky-500/20 dark:bg-sky-500/10 dark:text-sky-300 dark:hover:bg-sky-500/15'
  }
}

function copyModel(name: string) {
  void copyToClipboard(name, t('availableChannels.modelMarketplace.modelCopied'))
}

function copyEndpoint(path: string) {
  void copyToClipboard(path, t('availableChannels.endpoints.copied'))
}
</script>
