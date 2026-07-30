import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import type { ModelPlazaResponse } from '@/api/modelPlaza'
import ModelPlazaContent from '../ModelPlazaContent.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const publicGroup = {
  id: 1,
  name: 'public',
  platform: 'openai',
  subscription_type: 'standard',
  rate_multiplier: 0.8,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  is_exclusive: false,
}

const response: ModelPlazaResponse = {
  description: 'Public **pricing**',
  channels: [{
    name: 'deepseek',
    description: '',
    platforms: [{
      platform: 'openai',
      groups: [
        publicGroup,
        { ...publicGroup, id: 2, name: 'exclusive', is_exclusive: true },
        {
          ...publicGroup,
          id: 3,
          name: 'subscription',
          subscription_type: 'subscription',
        },
      ],
      supported_models: [
        {
          name: 'deepseek-v4-flash',
          platform: 'openai',
          pricing: null,
          route_group_ids: [1, 2, 3],
          supported_endpoints: [{
            protocol: 'openai_chat_completions',
            path: '/v1/chat/completions',
            group_ids: [1, 2],
          }],
        },
        {
          name: 'deepseek-v4-pro',
          platform: 'openai',
          pricing: null,
          route_group_ids: [1],
          supported_endpoints: [{
            protocol: 'openai_responses',
            path: '/v1/responses',
            group_ids: [1],
          }],
        },
      ],
    }],
  }],
}

const MarketplaceStub = defineComponent({
  name: 'AvailableModelMarketplace',
  props: {
    cards: { type: Array, required: true },
    loading: Boolean,
    pricingLabels: { type: Object, required: true },
    userGroupRates: { type: Object, required: true },
    emptyLabel: { type: String, required: true },
    applyRateMultiplier: Boolean,
  },
  template: '<div data-testid="marketplace-stub"></div>',
})

function mountContent() {
  return mount(ModelPlazaContent, {
    props: {
      response,
      loading: false,
    },
    global: {
      stubs: {
        AvailableModelMarketplace: MarketplaceStub,
        Icon: true,
        PlatformIcon: true,
      },
    },
  })
}

describe('ModelPlazaContent', () => {
  it('renders only public standard group cards through the shared marketplace contract', () => {
    const wrapper = mountContent()
    const marketplace = wrapper.findComponent(MarketplaceStub)
    const cards = marketplace.props('cards') as Array<{
      name: string
      group: { id: number }
      endpoints: Array<{ path: string }>
    }>

    expect(cards).toHaveLength(2)
    expect(cards.every((card) => card.group.id === publicGroup.id)).toBe(true)
    expect(cards.map((card) => card.name)).toEqual([
      'deepseek-v4-flash',
      'deepseek-v4-pro',
    ])
    expect(cards[0].endpoints.map((endpoint) => endpoint.path)).toEqual([
      '/v1/chat/completions',
    ])
    expect(marketplace.props('applyRateMultiplier')).toBe(true)
    expect(marketplace.props('userGroupRates')).toEqual({})
    expect(wrapper.text()).not.toContain('modelPlaza.publicHint')
    expect(wrapper.text()).not.toContain('modelPlaza.filters.rateLabel')
    expect(wrapper.html()).not.toContain('exclusive')
    expect(wrapper.html()).not.toContain('subscription')
  })

  it('filters the public cards by model name without changing the source response', async () => {
    const wrapper = mountContent()
    const input = wrapper.get('input[type="text"]')

    await input.setValue('flash')
    await flushPromises()

    const cards = wrapper.findComponent(MarketplaceStub).props('cards') as Array<{ name: string }>
    expect(cards.map((card) => card.name)).toEqual(['deepseek-v4-flash'])
    expect(response.channels[0].platforms[0].supported_models).toHaveLength(2)
  })

  it('offers a retry action after the catalog request fails', async () => {
    const wrapper = mount(ModelPlazaContent, {
      props: {
        response: null,
        loading: false,
        error: true,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await wrapper.get('button').trigger('click')

    expect(wrapper.text()).toContain('modelPlaza.loadFailed')
    expect(wrapper.text()).toContain('modelPlaza.retry')
    expect(wrapper.emitted('retry')).toHaveLength(1)
  })
})
