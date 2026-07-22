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
              :rate-multiplier="section.group.rate_multiplier"
              :user-rate-multiplier="userGroupRates[section.group.id] ?? null"
              :peak-rate-enabled="section.group.peak_rate_enabled"
              :peak-start="section.group.peak_start"
              :peak-end="section.group.peak_end"
              :peak-rate-multiplier="section.group.peak_rate_multiplier"
              always-show-rate
            />
          </h2>
          <span class="shrink-0 text-[10px] font-medium text-stone-400 dark:text-stone-500">
            {{ t('availableChannels.modelMarketplace.groupModelCount', { count: section.cards.length }) }}
          </span>
          <span class="h-px min-w-4 flex-1 bg-stone-200/80 dark:bg-white/[0.08]" />
        </div>

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

            <section class="mt-2.5 rounded-lg bg-stone-50 px-2.5 py-2 dark:bg-black/15">
              <template v-if="card.pricingOptions.length === 1">
                <div v-if="card.pricingOptions[0]" class="flex min-h-7 items-center justify-between gap-2">
                  <div class="min-w-0">
                    <span class="text-[9px] font-semibold uppercase tracking-[0.1em] text-stone-400 dark:text-stone-500">
                      {{ formatBillingMode(card.pricingOptions[0], pricingLabels) }}
                    </span>
                    <span class="ml-1 text-[9px] text-stone-400 dark:text-stone-500">{{ pricingUnit(card.pricingOptions[0]) }}</span>
                    <p v-if="hasTieredPricing(card.pricingOptions[0])" class="mt-0.5 truncate text-[9px] text-stone-500 dark:text-stone-400" :title="tieredPricing(card.pricingOptions[0])">
                      {{ t('availableChannels.modelMarketplace.tieredPricing') }} · {{ tieredPricing(card.pricingOptions[0]) }}
                    </p>
                  </div>

                  <div v-if="card.pricingOptions[0]?.billing_mode === BILLING_MODE_TOKEN" class="flex shrink-0 items-baseline gap-2 font-mono">
                    <div class="whitespace-nowrap text-[10px] text-stone-500 dark:text-stone-400">
                      {{ t('availableChannels.pricing.inputPrice') }}
                      <strong class="ml-0.5 text-[13px] text-stone-950 dark:text-stone-100">{{ formatCompactTokenPrice(card.pricingOptions[0]?.input_price ?? null) }}</strong>
                    </div>
                    <span class="h-3 w-px bg-stone-200 dark:bg-white/10" />
                    <div class="whitespace-nowrap text-[10px] text-stone-500 dark:text-stone-400">
                      {{ t('availableChannels.pricing.outputPrice') }}
                      <strong class="ml-0.5 text-[13px] text-stone-950 dark:text-stone-100">{{ formatCompactTokenPrice(card.pricingOptions[0]?.output_price ?? null) }}</strong>
                    </div>
                  </div>

                  <div v-else class="shrink-0 font-mono text-[13px] font-semibold text-stone-950 dark:text-stone-100">
                    {{ requestPrice(card.pricingOptions[0]) }}
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
  BILLING_MODE_IMAGE,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { useClipboard } from '@/composables/useClipboard'
import {
  formatAvailableChannelIntervals,
  formatBillingMode,
  formatCompactRequestPrice,
  formatCompactTokenPrice,
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
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const MAX_VISIBLE_CHANNELS = 2

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

function requestPrice(pricing: UserSupportedModelPricing): string {
  const value = pricing.billing_mode === BILLING_MODE_IMAGE
    ? pricing.image_output_price
    : pricing.per_request_price
  return `${formatCompactRequestPrice(value)} ${props.pricingLabels.unitPerRequest}`
}

function hasTieredPricing(pricing: UserSupportedModelPricing): boolean {
  return pricing.intervals.length > 0
}

function tieredPricing(pricing: UserSupportedModelPricing): string {
  return formatAvailableChannelIntervals(pricing, props.pricingLabels, { compact: true })
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
