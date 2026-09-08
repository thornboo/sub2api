import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import AvailableModelMarketplace from '../AvailableModelMarketplace.vue'
import { createModelRuntimeMetricsPreview } from '../modelRuntimeMetrics'
import type { AvailableModelMarketplaceCard } from '@/utils/availableModelMarketplace'
import { BILLING_MODE_IMAGE, BILLING_MODE_TOKEN } from '@/constants/channel'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh' },
      t: (key: string, params?: Record<string, string | number>) => {
        if (!params) return key
        return `${key}:${Object.values(params).join(':')}`
      },
    }),
  }
})

const pricing = {
  billing_mode: BILLING_MODE_TOKEN,
  input_price: 0.0000008,
  output_price: 0.000004,
  cache_write_price: null,
  cache_read_price: null,
  image_input_price: null,
  image_output_price: null,
  per_request_price: null,
  intervals: [],
}

const publicGroup = {
  id: 1,
  name: '公开 8 折',
  platform: 'openai',
  subscription_type: 'standard',
  rate_multiplier: 0.8,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  is_exclusive: false,
}

const exclusiveGroup = {
  ...publicGroup,
  id: 2,
  name: '专属 7 折',
  rate_multiplier: 0.7,
  is_exclusive: true,
}

const cards: AvailableModelMarketplaceCard[] = [
  {
    id: '1::MiniMax-M3',
    name: 'MiniMax-M3',
    group: publicGroup,
    platforms: ['openai'],
    channelNames: ['ikuncode-cx', '智链'],
    endpoints: [
      { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [1] },
      { protocol: 'openai_chat_completions', path: '/v1/chat/completions', group_ids: [1] },
      { protocol: 'openai_responses', path: '/v1/responses', group_ids: [1] },
    ],
    pricingOptions: [pricing],
    routes: [
      {
        id: 'route-1',
        channelName: 'ikuncode-cx',
        channelDescription: '',
        platform: 'openai',
        group: publicGroup,
        pricing,
        endpoints: [],
      },
      {
        id: 'route-2',
        channelName: '智链',
        channelDescription: '',
        platform: 'openai',
        group: publicGroup,
        pricing,
        endpoints: [],
      },
    ],
  },
  {
    id: '2::MiniMax-M3',
    name: 'MiniMax-M3',
    group: exclusiveGroup,
    platforms: ['openai'],
    channelNames: ['专属线路'],
    endpoints: [
      { protocol: 'anthropic_messages', path: '/v1/messages', group_ids: [2] },
    ],
    pricingOptions: [pricing],
    routes: [
      {
        id: 'route-3',
        channelName: '专属线路',
        channelDescription: '',
        platform: 'openai',
        group: exclusiveGroup,
        pricing,
        endpoints: [],
      },
    ],
  },
]

const scheduledTierPricing = {
  ...pricing,
  time_pricing: {
    enabled: true,
    timezone: 'Asia/Shanghai',
    default_label: '平时',
    default_multiplier: 1,
    rules: [
      { label: '高峰', start_time: '10:00', end_time: '11:00', multiplier: 2 },
      { label: '夜间', start_time: '23:00', end_time: '07:00', multiplier: 0.5 },
    ],
  },
  intervals: [{
    min_tokens: 0,
    max_tokens: null,
    input_price: pricing.input_price,
    output_price: pricing.output_price,
    cache_write_price: null,
    cache_read_price: null,
    per_request_price: null,
  }],
}

const ModelIconStub = defineComponent({
  props: { model: String },
  template: '<span class="model-icon">{{ model }}</span>',
})

const PlatformIconStub = defineComponent({
  props: { platform: String },
  template: '<span class="platform-icon">{{ platform }}</span>',
})

const GroupBadgeStub = defineComponent({
  props: {
    name: String,
    rateMultiplier: Number,
    showRate: { type: Boolean, default: true },
  },
  template: '<span class="group-badge">{{ name }}<span v-if="showRate && rateMultiplier !== undefined" data-testid="group-rate">{{ rateMultiplier }}x</span></span>',
})

function mountMarketplace(overrides: Partial<InstanceType<typeof AvailableModelMarketplace>['$props']> = {}) {
  return mount(AvailableModelMarketplace, {
    props: {
      cards,
      loading: false,
      pricingLabels: {
        billingModeToken: '按 Token',
        billingModePerRequest: '按次',
        billingModeImage: '按图片',
        noPricing: '未配置定价',
        unitPerMillion: '/ 1M token',
        unitPerRequest: '/ 次',
      },
      emptyLabel: '暂无模型',
      userGroupRates: {},
      ...overrides,
    },
    global: {
      plugins: [createPinia()],
      stubs: {
        Icon: true,
        ModelIcon: ModelIconStub,
        PlatformIcon: PlatformIconStub,
        GroupBadge: GroupBadgeStub,
      },
    },
  })
}

describe('AvailableModelMarketplace', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('separates model cards by group and keeps each group route inside its own section', () => {
    const wrapper = mountMarketplace()
    const publicSection = wrapper.get('[data-group-id="1"]')
    const exclusiveSection = wrapper.get('[data-group-id="2"]')

    expect(wrapper.findAll('[data-testid="available-model-group-section"]')).toHaveLength(2)
    expect(wrapper.findAll('[data-testid="available-model-card"]')).toHaveLength(2)
    expect(publicSection.text()).toContain('公开 8 折')
    expect(publicSection.text()).toContain('availableChannels.modelMarketplace.channelCount:2')
    expect(publicSection.text()).toContain('ikuncode-cx')
    expect(publicSection.text()).toContain('智链')
    expect(publicSection.text()).not.toContain('专属线路')
    expect(publicSection.text()).toContain('$0.8')
    expect(publicSection.text()).toContain('$4')
    expect(publicSection.text()).toContain('/v1/messages')
    expect(publicSection.text()).toContain('/v1/chat/completions')
    expect(publicSection.text()).toContain('/v1/responses')
    expect(exclusiveSection.text()).toContain('专属 7 折')
    expect(exclusiveSection.text()).toContain('专属线路')
    expect(exclusiveSection.text()).not.toContain('ikuncode-cx')
  })

  it('has distinct loading and empty states', () => {
    const loading = mountMarketplace({ loading: true })
    expect(loading.get('[aria-busy="true"]').exists()).toBe(true)
    loading.unmount()

    const empty = mountMarketplace({ cards: [] })
    expect(empty.text()).toContain('暂无模型')
  })

  it.each([
    { userGroupRates: {}, kind: 'group', rate: '0.8x', price: '$0.64' },
    { userGroupRates: { 1: 0.5 }, kind: 'user', rate: '0.5x', price: '$0.4' },
    { userGroupRates: { 1: 0 }, kind: 'user', rate: '0x', price: '$0' },
    { userGroupRates: { 1: 0.8 }, kind: 'group', rate: '0.8x', price: '$0.64' },
  ])('shows a $kind heading rate of $rate without multiplying prices again', ({ userGroupRates, kind, rate, price }) => {
    const wrapper = mountMarketplace({
      cards: [cards[0]],
      showGroupRates: true,
      applyRateMultiplier: true,
      userGroupRates,
    })
    const badge = wrapper.get('[data-testid="group-rate-badge"]')
    expect(badge.attributes('data-rate-kind')).toBe(kind)
    expect(badge.text()).toContain(`availableChannels.modelMarketplace.groupRate.${kind}`)
    expect(badge.text()).toContain(rate)
    expect(wrapper.get('[data-testid="effective-input-price"]').text()).toBe(price)
    wrapper.unmount()
  })

  it('keeps rate badges opt-in and hides them when prices are not adjusted', () => {
    for (const props of [{ applyRateMultiplier: true }, { showGroupRates: true }]) {
      const wrapper = mountMarketplace(props)
      expect(wrapper.find('[data-testid="group-rate-badge"]').exists()).toBe(false)
      wrapper.unmount()
    }
  })

  it('opens rate details with the default rate and already-calculated price explanation', async () => {
    const wrapper = mountMarketplace({
      cards: [cards[0]],
      showGroupRates: true,
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.5 },
    })
    const badge = wrapper.get('[data-testid="group-rate-badge"]')
    expect(badge.element.tagName).toBe('BUTTON')
    expect(badge.attributes('aria-expanded')).toBe('false')
    await badge.trigger('click')
    await flushPromises()
    const dialog = document.body.querySelector('[role="dialog"]')
    expect(badge.attributes('aria-expanded')).toBe('true')
    expect(dialog?.textContent).toContain('availableChannels.modelMarketplace.groupRate.defaultRate:0.8x')
    expect(dialog?.textContent).toContain('availableChannels.modelMarketplace.groupRate.priceHint')
    await badge.trigger('click')
    await flushPromises()
    expect(badge.attributes('aria-expanded')).toBe('false')
    wrapper.unmount()
  })

  it('shows both ordinary and independent image rates in a mixed group', async () => {
    const group = { ...publicGroup, image_rate_independent: true, image_rate_multiplier: 0.6 }
    const imagePricing = { ...pricing, billing_mode: BILLING_MODE_IMAGE, per_request_price: 3.5, intervals: [] }
    const wrapper = mountMarketplace({
      cards: [
        { ...cards[0], group },
        { ...cards[0], id: '1::gpt-image-2', name: 'gpt-image-2', group, pricingOptions: [imagePricing] },
      ],
      showGroupRates: true,
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.5 },
    })
    expect(wrapper.findAll('[data-testid="group-rate-badge"]')).toHaveLength(2)
    expect(wrapper.get('[data-rate-kind="user"]').text()).toContain('0.5x')
    expect(wrapper.get('[data-rate-kind="image"]').text()).toContain('0.6x')
    await wrapper.setProps({ userGroupRates: { 1: 0.7 } })
    expect(wrapper.get('[data-rate-kind="user"]').text()).toContain('0.7x')
    expect(wrapper.get('[data-rate-kind="image"]').text()).toContain('0.6x')
    wrapper.unmount()
  })

  it('can display effective prices using the group or user multiplier', () => {
    const groupRate = mountMarketplace({ applyRateMultiplier: true })
    const publicSection = groupRate.get('[data-group-id="1"]')
    expect(publicSection.get('[data-testid="effective-input-price"]').text()).toBe('$0.64')
    expect(publicSection.get('[data-testid="effective-output-price"]').text()).toBe('$3.2')
    expect(publicSection.get('[data-testid="original-input-price"]').text()).toBe('$0.8')
    expect(publicSection.get('[data-testid="original-output-price"]').text()).toBe('$4')
    expect(publicSection.get('[data-testid="price-discount"]').text()).toBe(
      'availableChannels.modelMarketplace.savings:20',
    )
    expect(publicSection.find('[data-testid="price-effective-rate"]').exists()).toBe(false)
    groupRate.unmount()

    const userRate = mountMarketplace({
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.5 },
    })
    const userPublicSection = userRate.get('[data-group-id="1"]')
    expect(userPublicSection.get('[data-testid="effective-input-price"]').text()).toBe('$0.4')
    expect(userPublicSection.get('[data-testid="effective-output-price"]').text()).toBe('$2')
    expect(userPublicSection.get('[data-testid="original-input-price"]').text()).toBe('$0.8')
    expect(userPublicSection.get('[data-testid="original-output-price"]').text()).toBe('$4')
    expect(userPublicSection.get('[data-testid="price-discount"]').text()).toBe(
      'availableChannels.modelMarketplace.savings:50',
    )
  })

  it('shows final surcharge prices without exposing internal multipliers', () => {
    const surchargeCard: AvailableModelMarketplaceCard = {
      ...cards[0],
      group: {
        ...cards[0].group,
        rate_multiplier: 2.5,
      },
    }
    const wrapper = mountMarketplace({
      cards: [surchargeCard],
      applyRateMultiplier: true,
    })
    const card = wrapper.get('[data-testid="available-model-card"]')

    expect(card.get('[data-testid="effective-input-price"]').text()).toBe('$2')
    expect(card.get('[data-testid="effective-output-price"]').text()).toBe('$10')
    expect(card.get('[data-testid="original-input-price"]').text()).toBe('$0.8')
    expect(card.get('[data-testid="original-output-price"]').text()).toBe('$4')
    expect(card.find('[data-testid="price-effective-rate"]').exists()).toBe(false)
    expect(card.find('[data-testid="price-discount"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="group-rate"]').exists()).toBe(false)
  })

  it('does not repeat an identical original price for a 1x rate', () => {
    const standardCard: AvailableModelMarketplaceCard = {
      ...cards[0],
      group: {
        ...cards[0].group,
        rate_multiplier: 1,
      },
    }
    const wrapper = mountMarketplace({
      cards: [standardCard],
      applyRateMultiplier: true,
    })
    const card = wrapper.get('[data-testid="available-model-card"]')

    expect(card.get('[data-testid="effective-input-price"]').text()).toBe('$0.8')
    expect(card.get('[data-testid="effective-output-price"]').text()).toBe('$4')
    expect(card.find('[data-testid="original-input-price"]').exists()).toBe(false)
    expect(card.find('[data-testid="original-output-price"]').exists()).toBe(false)
    expect(card.find('[data-testid="price-discount"]').exists()).toBe(false)
    expect(card.find('[data-testid="price-group-rate"]').exists()).toBe(false)
  })


  it('shows token max reasoning multiplier without changing normal prices', () => {
    const maxReasoningPricing = {
      ...pricing,
      max_reasoning_effort_multiplier: 2,
    }
    const maxReasoningCard: AvailableModelMarketplaceCard = {
      ...cards[0],
      pricingOptions: [maxReasoningPricing],
      routes: cards[0].routes.map((route) => ({ ...route, pricing: maxReasoningPricing })),
    }

    const wrapper = mountMarketplace({ cards: [maxReasoningCard] })
    const card = wrapper.get('[data-testid="available-model-card"]')
    const badge = card.get('[data-testid="max-reasoning-multiplier"]')

    expect(badge.text()).toBe('modelPlaza.table.maxReasoningMultiplierBadge:2')
    expect(badge.attributes('title')).toBe('modelPlaza.table.maxReasoningMultiplierHint:2')
    expect(card.get('[data-testid="effective-input-price"]').text()).toBe('$0.8')
    expect(card.get('[data-testid="effective-output-price"]').text()).toBe('$4')
  })

  it('does not show max reasoning multiplier without a positive token multiplier', () => {
    const wrapper = mountMarketplace()

    expect(wrapper.find('[data-testid="max-reasoning-multiplier"]').exists()).toBe(false)
  })

  it.each([
    { scheduled: false, firstPrices: ['$9', '$45'], secondPrices: ['$18', '$67.5'] },
    { scheduled: true, firstPrices: ['$5', '$25', '$0', '$0.5'], secondPrices: ['$10', '$37.5', '-', '-'] },
  ])('shows token tiers as labeled rows using the applicable rate (scheduled: $scheduled)', async ({ scheduled, firstPrices, secondPrices }) => {
    const tierPricing = {
      ...pricing,
      input_price: 0.00001,
      output_price: 0.00005,
      time_pricing: scheduled ? {
        enabled: true,
        timezone: 'Asia/Shanghai',
        default_label: '平时',
        default_multiplier: 0.5,
        rules: [],
      } : null,
      intervals: [
        {
          min_tokens: 0,
          max_tokens: 272000,
          input_price: 0.00001,
          output_price: 0.00005,
          cache_write_price: scheduled ? 0 : null,
          cache_read_price: scheduled ? 0.000001 : null,
          per_request_price: null,
        },
        {
          min_tokens: 272001,
          max_tokens: null,
          input_price: 0.00002,
          output_price: 0.000075,
          cache_write_price: null,
          cache_read_price: null,
          per_request_price: null,
        },
      ],
    }
    const wrapper = mountMarketplace({
      cards: [{ ...cards[0], name: 'gpt-6-astra', pricingOptions: [tierPricing] }],
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.9 },
    })

    try {
      const card = wrapper.get('[data-testid="available-model-card"]')
      const trigger = card.get('[data-testid="tier-pricing-trigger"]')
      expect(trigger.text()).toContain('availableChannels.modelMarketplace.tierPricing.count:2')
      expect(trigger.attributes('aria-expanded')).toBe('false')
      expect(card.text()).not.toContain('272000')

      await trigger.trigger('click')
      await flushPromises()
      const popoverElement = document.body.querySelector<HTMLElement>('[data-testid="tier-pricing-popover"]')
      expect(popoverElement).not.toBeNull()
      const popover = new DOMWrapper(popoverElement!)
      expect(card.element.contains(popover.element)).toBe(false)
      expect(popover.attributes('aria-label')).toContain('gpt-6-astra')
      expect(popover.get('p').text()).toContain('/ 1M token')
      expect(popover.findAll('thead th')).toHaveLength(scheduled ? 5 : 3)
      const rows = popover.findAll('[data-testid="tier-pricing-row"]')
      expect(rows).toHaveLength(2)
      expect(rows[0].get('th').text()).toBe('(0, 272000]')
      expect(rows[1].get('th').text()).toBe('(272001, ∞]')
      expect(rows[0].findAll('td').map(cell => cell.text())).toEqual(firstPrices)
      expect(rows[1].findAll('td').map(cell => cell.text())).toEqual(secondPrices)
      expect(popover.text().includes('availableChannels.modelMarketplace.tierPricing.currentTimePrices')).toBe(scheduled)

      await popover.get('button[aria-label="common.close"]').trigger('click')
      await flushPromises()
      expect(trigger.attributes('aria-expanded')).toBe('false')
      expect(document.body.querySelector('[data-testid="tier-pricing-popover"]')).toBeNull()
    } finally {
      wrapper.unmount()
    }
  })

  it('omits a tier entry when none of its tiers publish a price', () => {
    const wrapper = mountMarketplace({
      cards: [{
        ...cards[0],
        pricingOptions: [{
          ...pricing,
          intervals: [{
            min_tokens: 0,
            max_tokens: null,
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null,
          }],
        }],
      }],
    })
    expect(wrapper.find('[data-testid="tier-pricing-trigger"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('uses image request tiers and the independent image multiplier', async () => {
    const imagePricing = {
      billing_mode: BILLING_MODE_IMAGE,
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: 0.00003,
      per_request_price: 0.2,
      intervals: [{
        min_tokens: 0,
        max_tokens: null,
        tier_label: '1K',
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        per_request_price: 0.02,
      }],
    }
    const imageGroup = {
      ...publicGroup,
      rate_multiplier: 0.1,
      image_rate_independent: true,
      image_rate_multiplier: 1,
    }
    const imageCard: AvailableModelMarketplaceCard = {
      id: '1::gpt-image-2',
      name: 'gpt-image-2',
      group: imageGroup,
      platforms: ['openai'],
      channelNames: ['images'],
      endpoints: [],
      pricingOptions: [imagePricing],
      routes: [{
        id: 'image-route',
        channelName: 'images',
        channelDescription: '',
        platform: 'openai',
        group: imageGroup,
        pricing: imagePricing,
        endpoints: [],
      }],
    }

    const wrapper = mountMarketplace({
      cards: [imageCard],
      showGroupRates: true,
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.01 },
    })
    try {
      const card = wrapper.get('[data-testid="available-model-card"]')
      expect(card.text()).toContain('$0.2 / 次')
      expect(card.text()).not.toContain('$0.02')
      expect(card.text()).not.toContain('$0.00003')
      expect(wrapper.findAll('[data-testid="group-rate-badge"]')).toHaveLength(1)
      expect(wrapper.get('[data-rate-kind="image"]').text()).toContain('1x')

      await card.get('[data-testid="tier-pricing-trigger"]').trigger('click')
      await flushPromises()
      const popoverElement = document.body.querySelector<HTMLElement>('[data-testid="tier-pricing-popover"]')
      expect(popoverElement).not.toBeNull()
      const popover = new DOMWrapper(popoverElement!)
      expect(popover.get('p').text()).toContain('/ 次')
      expect(popover.findAll('thead th')).toHaveLength(2)
      expect(popover.get('[data-testid="tier-pricing-row"] th').text()).toBe('1K')
      expect(popover.get('[data-testid="tier-pricing-row"] td').text()).toBe('$0.02')
    } finally {
      wrapper.unmount()
    }
  })

  it.each([
    { before: '2026-09-08T01:59:59Z', initial: '$0.8', expected: '$1.6', timePrice: '$1.6000', window: '10:00-11:00', detail: 'time' },
    { before: '2026-09-08T02:59:59Z', initial: '$1.6', expected: '$0.8', timePrice: '$0.8000', window: 'availableChannels.modelMarketplace.timePricing.otherTimes', detail: 'tier' },
    { before: '2026-09-08T14:59:59Z', initial: '$0.8', expected: '$0.4', timePrice: '$0.4000', window: '23:00-07:00', detail: 'time' },
    { before: '2026-09-08T22:59:59Z', initial: '$0.4', expected: '$0.8', timePrice: '$0.8000', window: 'availableChannels.modelMarketplace.timePricing.otherTimes', detail: 'tier' },
  ])('synchronizes the card and open pricing details across $before', async ({ before, initial, expected, timePrice, window, detail }) => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    vi.setSystemTime(new Date(before))
    const wrapper = mountMarketplace({
      cards: [{ ...cards[0], pricingOptions: [scheduledTierPricing] }],
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.7 },
    })
    const assertQuote = (kind: string) => {
      const element = document.body.querySelector<HTMLElement>(`[data-testid="${kind}-pricing-popover"]`)
      expect(element).not.toBeNull()
      const popover = new DOMWrapper(element!)
      if (kind === 'tier') {
        expect(popover.get('[data-testid="tier-pricing-row"] td').text()).toBe(expected)
      } else {
        const active = popover.findAll('[data-testid="time-pricing-row"]').find(row => row.text().includes('availableChannels.modelMarketplace.timePricing.active'))
        expect(active?.text()).toContain(window)
        expect(active?.findAll('td')[2].text()).toBe(timePrice)
      }
    }
    try {
      expect(wrapper.get('[data-testid="effective-input-price"]').text()).toBe(initial)
      await wrapper.get(`[data-testid="${detail}-pricing-trigger"]`).trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(wrapper.get('[data-testid="effective-input-price"]').text()).toBe(expected)
      assertQuote(detail)
      const otherDetail = detail === 'time' ? 'tier' : 'time'
      await wrapper.get(`[data-testid="${otherDetail}-pricing-trigger"]`).trigger('click')
      await flushPromises()
      assertQuote(otherDetail)
    } finally {
      wrapper.unmount()
    }
  })

  it('refreshes current tier prices immediately when a hidden page becomes visible again', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setInterval', 'clearInterval'] })
    vi.setSystemTime(new Date('2026-09-08T01:59:00Z'))
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    const wrapper = mountMarketplace({ cards: [{ ...cards[0], pricingOptions: [scheduledTierPricing] }] })
    try {
      await wrapper.get('[data-testid="tier-pricing-trigger"]').trigger('click')
      await flushPromises()
      visibility.mockReturnValue('hidden')
      document.dispatchEvent(new Event('visibilitychange'))
      await flushPromises()
      vi.setSystemTime(new Date('2026-09-08T02:01:00Z'))
      visibility.mockReturnValue('visible')
      document.dispatchEvent(new Event('visibilitychange'))
      await flushPromises()
      expect(wrapper.get('[data-testid="effective-input-price"]').text()).toBe('$1.6')
      expect(document.body.querySelector('[data-testid="tier-pricing-row"] td')?.textContent).toBe('$1.6')
    } finally {
      wrapper.unmount()
      visibility.mockRestore()
    }
  })

  it.each([
    { scheduled: false, price: 0.000005, expected: '$3.5' },
    { scheduled: true, price: 0.000005, expected: '$10' },
    { scheduled: true, price: 0, expected: '$0' },
  ])('shows a 1h-cache-only tier at its applicable rate (scheduled: $scheduled, price: $price)', async ({ scheduled, price, expected }) => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-09-08T02:01:00Z'))
    const oneHourPricing = {
      ...pricing,
      time_pricing: scheduled ? scheduledTierPricing.time_pricing : null,
      intervals: [{
        min_tokens: 0,
        max_tokens: null,
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_write_1h_price: price,
        cache_read_price: null,
        per_request_price: null,
      }],
    }
    const wrapper = mountMarketplace({
      cards: [{ ...cards[0], pricingOptions: [oneHourPricing] }],
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.7 },
    })
    try {
      await wrapper.get('[data-testid="tier-pricing-trigger"]').trigger('click')
      await flushPromises()
      const element = document.body.querySelector<HTMLElement>('[data-testid="tier-pricing-popover"]')
      expect(element).not.toBeNull()
      const popover = new DOMWrapper(element!)
      expect(popover.findAll('thead th').map(column => column.text())).toContain('availableChannels.modelMarketplace.timePricing.cacheWrite1h')
      expect(popover.findAll('[data-testid="tier-pricing-row"]')).toHaveLength(1)
      expect(popover.findAll('[data-testid="tier-pricing-row"] td').map(cell => cell.text())).toEqual(['-', '-', expected])
    } finally {
      wrapper.unmount()
    }
  })

  it('shows configured 1h cache prices in every time window', async () => {
    const wrapper = mountMarketplace({
      cards: [{ ...cards[0], pricingOptions: [{ ...scheduledTierPricing, cache_write_1h_price: 0.000005 }] }],
    })
    try {
      await wrapper.get('[data-testid="time-pricing-trigger"]').trigger('click')
      await flushPromises()
      const element = document.body.querySelector<HTMLElement>('[data-testid="time-pricing-popover"]')
      expect(element).not.toBeNull()
      const popover = new DOMWrapper(element!)
      const columnIndex = popover.findAll('thead th').findIndex(column => column.text() === 'availableChannels.modelMarketplace.timePricing.cacheWrite1h')
      expect(columnIndex).toBeGreaterThan(-1)
      expect(popover.findAll('[data-testid="time-pricing-row"]').map(row => row.findAll('td')[columnIndex].text())).toEqual(['$5.0000', '$10.0000', '$2.5000'])
    } finally {
      wrapper.unmount()
    }
  })

  it('shows scheduled prices outside the card in a dismissible popover', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date('2026-08-17T01:30:00.000Z'))

    const timePricing = {
      ...pricing,
      cache_write_price: 0,
      cache_read_price: 0.00000012,
      time_pricing: {
        enabled: true,
        timezone: 'Asia/Shanghai',
        default_label: '平时',
        default_multiplier: 1.1,
        rules: [
          { label: '上午繁忙', start_time: '09:00', end_time: '12:00', multiplier: 2 },
          { label: '夜间优惠', start_time: '23:00', end_time: '07:00', multiplier: 0.5 },
        ],
      },
    }
    const timeCard: AvailableModelMarketplaceCard = {
      ...cards[0],
      pricingOptions: [timePricing],
      routes: cards[0].routes.map((route) => ({ ...route, pricing: timePricing })),
    }

    const wrapper = mountMarketplace({
      cards: [timeCard],
      showGroupRates: true,
      applyRateMultiplier: true,
      userGroupRates: { 1: 0.7 },
    })
    try {
      const card = wrapper.get('[data-testid="available-model-card"]')
      const trigger = card.get('[data-testid="time-pricing-trigger"]')
      const popoverSelector = '[data-testid="time-pricing-popover"]'

      expect(card.get('[data-testid="effective-input-price"]').text()).toBe('$1.6')
      expect(wrapper.get('[data-rate-kind="user"]').text()).toContain('0.7x')
      expect(card.get('[data-testid="effective-output-price"]').text()).toBe('$8')
      expect(card.get('[data-testid="original-input-price"]').text()).toBe('$0.8')
      expect(card.find('[data-testid="price-effective-rate"]').exists()).toBe(false)
      expect(trigger.text()).toContain('availableChannels.modelMarketplace.timePricing.title')
      expect(trigger.text()).toContain('Asia/Shanghai')
      expect(trigger.attributes('aria-expanded')).toBe('false')
      expect(document.body.querySelector(popoverSelector)).toBeNull()

      await trigger.trigger('click')
      await flushPromises()

      const popoverElement = document.body.querySelector<HTMLElement>(popoverSelector)
      expect(popoverElement).not.toBeNull()
      const popover = new DOMWrapper(popoverElement!)
      const rows = popover.findAll('[data-testid="time-pricing-row"]')

      expect(trigger.attributes('aria-expanded')).toBe('true')
      expect(card.element.contains(popover.element)).toBe(false)
      expect(card.findAll('[data-testid="time-pricing-row"]')).toHaveLength(0)
      expect(popover.attributes('aria-label')).toContain(timeCard.name)
      expect(popover.text()).toContain('Asia/Shanghai')
      expect(popover.get('p').text()).toContain('/ 1M token')
      expect(rows).toHaveLength(3)
      expect(rows[0].text()).toContain('availableChannels.modelMarketplace.timePricing.otherTimes')
      expect(rows[0].text()).toContain('平时')
      expect(rows[0].text()).toContain('1.1x')
      expect(rows[0].text()).toContain('$0.8800')
      expect(rows[1].text()).toContain('09:00-12:00')
      expect(rows[1].text()).toContain('availableChannels.modelMarketplace.timePricing.active')
      expect(rows[1].text()).toContain('上午繁忙')
      expect(rows[1].text()).toContain('2x')
      expect(rows[1].text()).toContain('$1.6000')
      expect(rows[1].text()).toContain('$8.0000')
      expect(rows[1].text()).toContain('$0.0000')
      expect(rows[1].text()).toContain('$0.2400')
      expect(rows[2].text()).toContain('23:00-07:00')
      expect(rows[2].text()).toContain('夜间优惠')
      expect(popover.text()).not.toContain('availableChannels.modelMarketplace.timePricing.types.peak')

      await popover.get('button[aria-label="common.close"]').trigger('click')
      await flushPromises()
      expect(trigger.attributes('aria-expanded')).toBe('false')
      expect(document.body.querySelector(popoverSelector)).toBeNull()

      await trigger.trigger('click')
      await flushPromises()
      expect(trigger.attributes('aria-expanded')).toBe('true')
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      await flushPromises()
      expect(trigger.attributes('aria-expanded')).toBe('false')
      expect(document.body.querySelector(popoverSelector)).toBeNull()

      await trigger.trigger('click')
      await flushPromises()
      expect(trigger.attributes('aria-expanded')).toBe('true')
      await vi.waitFor(async () => {
        document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))
        await flushPromises()
        expect(trigger.attributes('aria-expanded')).toBe('false')
      })
      expect(document.body.querySelector(popoverSelector)).toBeNull()
    } finally {
      wrapper.unmount()
    }
  })
})

describe('marketplace layout preview', () => {
  it('retains configured prices and uses hourly graphics only when preview data is supplied', () => {
    const source = [cards[0]]
    const original = mountMarketplace({ cards: source })
    const inputPrice = original.get('[data-testid="effective-input-price"]').text()
    const outputPrice = original.get('[data-testid="effective-output-price"]').text()
    expect(original.find('[data-testid="model-runtime-metrics"]').exists()).toBe(false)
    original.unmount()

    const preview = mountMarketplace({ cards: source, runtimeMetrics: createModelRuntimeMetricsPreview(source) })
    expect(preview.get('[data-testid="effective-input-price"]').text()).toBe(inputPrice)
    expect(preview.get('[data-testid="effective-output-price"]').text()).toBe(outputPrice)
    expect(preview.findAll('[data-testid="runtime-hour-bar"]')).toHaveLength(24)
    expect(preview.text()).not.toContain('Credits')
    preview.unmount()
  })

  it('shows the single no-account notice from card schedulable state without runtime metrics', () => {
    const unavailableCard: AvailableModelMarketplaceCard = {
      ...cards[0],
      hasSchedulableAccount: false,
    }

    const wrapper = mountMarketplace({ cards: [unavailableCard] })

    expect(wrapper.find('[data-testid="model-runtime-metrics"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="model-availability-notice"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="model-availability-notice"]').text()).toContain(
      'availableChannels.modelMarketplace.availability.noAccounts',
    )
    wrapper.unmount()
  })
})
