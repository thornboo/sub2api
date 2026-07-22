import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { defineComponent } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import AvailableModelMarketplace from '../AvailableModelMarketplace.vue'
import type { AvailableModelMarketplaceCard } from '@/utils/availableModelMarketplace'
import { BILLING_MODE_TOKEN } from '@/constants/channel'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
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

const ModelIconStub = defineComponent({
  props: { model: String },
  template: '<span class="model-icon">{{ model }}</span>',
})

const PlatformIconStub = defineComponent({
  props: { platform: String },
  template: '<span class="platform-icon">{{ platform }}</span>',
})

const GroupBadgeStub = defineComponent({
  props: { name: String },
  template: '<span class="group-badge">{{ name }}</span>',
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
})
