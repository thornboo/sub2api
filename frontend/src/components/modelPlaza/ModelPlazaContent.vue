<template>
  <div class="space-y-4">
    <div v-if="!embedded">
      <h1 class="text-2xl font-bold tracking-tight text-stone-950 dark:text-white sm:text-3xl">
        {{ t('modelPlaza.title') }}
      </h1>
      <p class="mt-1.5 text-sm text-stone-500 dark:text-stone-400">
        {{ t('modelPlaza.description') }}
      </p>
    </div>

    <div
      v-if="descriptionHtml"
      class="plaza-description rounded-xl border border-stone-200/80 bg-white/80 px-5 py-4 text-sm shadow-sm dark:border-white/10 dark:bg-white/[0.035]"
      v-html="descriptionHtml"
    ></div>

    <div
      v-if="error"
      class="rounded-2xl border border-red-200 bg-red-50 px-5 py-8 text-center text-sm text-red-600 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300"
    >
      <p>{{ t('modelPlaza.loadFailed') }}</p>
      <button
        type="button"
        class="mt-4 inline-flex items-center justify-center rounded-lg border border-red-300 bg-white px-4 py-2 font-medium text-red-700 transition-colors hover:bg-red-100 focus:outline-none focus:ring-2 focus:ring-red-400 focus:ring-offset-2 dark:border-red-400/40 dark:bg-red-500/10 dark:text-red-200 dark:hover:bg-red-500/20 dark:focus:ring-offset-dark-950"
        @click="emit('retry')"
      >
        {{ t('modelPlaza.retry') }}
      </button>
    </div>

    <template v-else>
      <PlazaFilterBar
        v-if="!loading"
        :platforms="platforms"
        :groups="groupOptions"
        :platform="selectedPlatform"
        :group-id="selectedGroupId"
        :search="searchQuery"
        @update:platform="selectedPlatform = $event"
        @update:group-id="selectedGroupId = $event"
        @update:search="searchQuery = $event"
      />

      <div
        class="overflow-hidden rounded-2xl border border-stone-200/80 bg-white/80 shadow-sm dark:border-white/10 dark:bg-black/25"
      >
        <AvailableModelMarketplace
          :cards="filteredCards"
          :loading="loading"
          :pricing-labels="pricingLabels"
          :user-group-rates="emptyGroupRates"
          :empty-label="searchActive ? t('modelPlaza.noSearchResult') : t('modelPlaza.empty')"
          apply-rate-multiplier
        />
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'

import type { UserAvailableGroup } from '@/api/channels'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import AvailableModelMarketplace from '@/components/channels/AvailableModelMarketplace.vue'
import type { AvailableChannelPricingLabels } from '@/utils/availableChannelsCatalog'
import {
  buildAvailableModelMarketplaceCards,
  type AvailableModelMarketplaceCard,
} from '@/utils/availableModelMarketplace'
import PlazaFilterBar from './PlazaFilterBar.vue'

const props = defineProps<{
  response: ModelPlazaResponse | null
  loading: boolean
  error?: boolean
  /** 后台内嵌形态(AppLayout 内):隐藏页头。 */
  embedded?: boolean
}>()

const emit = defineEmits<{
  retry: []
}>()

const { t } = useI18n()
const emptyGroupRates: Record<number, number> = {}

const selectedPlatform = ref<string>('all')
const selectedGroupId = ref<number | 'all'>('all')
const searchQuery = ref('')

const searchActive = computed(() => searchQuery.value.trim() !== '')

const descriptionHtml = computed(() => {
  const md = props.response?.description?.trim()
  if (!md) return ''
  return DOMPurify.sanitize(marked.parse(md) as string)
})

const allCards = computed(() =>
  buildAvailableModelMarketplaceCards(props.response?.channels ?? [], {
    groupScope: 'public',
  }),
)

const groups = computed<UserAvailableGroup[]>(() => {
  const byID = new Map<number, UserAvailableGroup>()
  allCards.value.forEach((card) => byID.set(card.group.id, card.group))
  return Array.from(byID.values()).sort(
    (a, b) => a.rate_multiplier - b.rate_multiplier || a.name.localeCompare(b.name),
  )
})

const platforms = computed(() =>
  [...new Set(allCards.value.flatMap((card) => card.platforms).filter(Boolean))].sort(),
)

const groupOptions = computed(() =>
  groups.value.map((group) => ({
    id: group.id,
    name: group.name,
    platform: group.platform,
  })),
)

watch(groups, (list) => {
  if (
    selectedGroupId.value !== 'all'
    && !list.some((group) => group.id === selectedGroupId.value)
  ) {
    selectedGroupId.value = 'all'
  }
})

const filteredCards = computed<AvailableModelMarketplaceCard[]>(() => {
  let cards = allCards.value
  if (selectedPlatform.value !== 'all') {
    cards = cards.filter((card) => card.platforms.includes(selectedPlatform.value))
  }
  if (selectedGroupId.value !== 'all') {
    cards = cards.filter((card) => card.group.id === selectedGroupId.value)
  }
  const query = searchQuery.value.trim().toLowerCase()
  if (query) {
    cards = cards.filter((card) => card.name.toLowerCase().includes(query))
  }
  return cards
})

const pricingLabels = computed<AvailableChannelPricingLabels>(() => ({
  billingModeToken: t('availableChannels.pricing.billingModeToken'),
  billingModePerRequest: t('availableChannels.pricing.billingModePerRequest'),
  billingModeImage: t('availableChannels.pricing.billingModeImage'),
  noPricing: t('availableChannels.noPricing'),
  unitPerMillion: t('availableChannels.pricing.unitPerMillion'),
  unitPerRequest: t('availableChannels.pricing.unitPerRequest'),
}))
</script>

<style scoped>
.plaza-description {
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.plaza-description :deep(h1),
.plaza-description :deep(h2),
.plaza-description :deep(h3) {
  @apply mb-2 mt-3 font-semibold text-stone-950 first:mt-0 dark:text-white;
}

.plaza-description :deep(p) {
  @apply mb-2 text-stone-700 last:mb-0 dark:text-stone-300;
}

.plaza-description :deep(a) {
  @apply text-primary-600 underline underline-offset-4 hover:text-primary-700 dark:text-primary-300;
}

.plaza-description :deep(ul) {
  @apply mb-2 list-disc pl-5;
}

.plaza-description :deep(ol) {
  @apply mb-2 list-decimal pl-5;
}

.plaza-description :deep(li) {
  @apply mb-0.5 text-stone-700 dark:text-stone-300;
}

.plaza-description :deep(code) {
  @apply rounded bg-stone-100 px-1.5 py-0.5 font-mono text-xs dark:bg-white/[0.07];
}

.plaza-description :deep(blockquote) {
  @apply my-2 border-l-4 border-stone-300 pl-3 text-stone-600 dark:border-stone-700 dark:text-stone-400;
}
</style>
