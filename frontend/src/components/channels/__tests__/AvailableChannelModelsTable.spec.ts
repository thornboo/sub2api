import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it } from 'vitest'

import AvailableChannelModelsTable from '../AvailableChannelModelsTable.vue'
import type { UserAvailableChannel } from '@/api/channels'
import { BILLING_MODE_TOKEN } from '@/constants/channel'
import { buildAvailableChannelCatalogRows } from '@/utils/availableChannelsCatalog'

const columns = {
  model: 'Model',
  platform: 'Platform',
  channel: 'Channel',
  billingMode: 'Billing Mode',
  interval: 'Range',
  inputPrice: 'Input',
  outputPrice: 'Output',
  cacheWritePrice: 'Cache Write',
  cacheReadPrice: 'Cache Read',
  imageOutputPrice: 'Image Output',
  perRequestPrice: 'Per Call',
  groups: 'Pricing Group',
}

const tooltips = {
  interval: 'Range',
  inputPrice: 'Effective input price',
  outputPrice: 'Effective output price',
  cacheWritePrice: 'Effective cache write price',
  cacheReadPrice: 'Effective cache read price',
  imageOutputPrice: 'Effective image output price',
  perRequestPrice: 'Effective per-call price',
}

const pricingLabels = {
  billingModeToken: 'Per Token',
  billingModePerRequest: 'Per Request',
  billingModeImage: 'Per Image',
  noPricing: 'No Pricing',
  unitPerMillion: '/ 1M tokens',
  unitPerRequest: '/ request',
}

const IconStub = defineComponent({ template: '<span />' })
const PlatformIconStub = defineComponent({ template: '<span />' })
const GroupBadgeStub = defineComponent({
  props: {
    name: String,
    rateMultiplier: Number,
    showRate: { type: Boolean, default: true },
  },
  template: '<span class="group-badge">{{ name }}<span v-if="showRate && rateMultiplier !== undefined" data-testid="group-rate">{{ rateMultiplier }}x</span></span>',
})

describe('AvailableChannelModelsTable', () => {
  it('renders the same effective group price as the model marketplace', () => {
    const channels: UserAvailableChannel[] = [
      {
        name: 'OpenAI Channel',
        description: '',
        platforms: [
          {
            platform: 'openai',
            groups: [
              {
                id: 1,
                name: 'Public 8 Off',
                platform: 'openai',
                subscription_type: 'standard',
                rate_multiplier: 0.8,
                is_exclusive: false,
              },
            ],
            supported_models: [
              {
                name: 'MiniMax-M3',
                platform: 'openai',
                route_group_ids: [1],
                pricing: {
                  billing_mode: BILLING_MODE_TOKEN,
                  input_price: 0.0000008,
                  output_price: 0.000004,
                  cache_write_price: null,
                  cache_read_price: null,
                  image_input_price: null,
                  image_output_price: null,
                  per_request_price: null,
                  intervals: [],
                },
              },
            ],
          },
        ],
      },
    ]
    const rows = buildAvailableChannelCatalogRows(channels, {
      userGroupRates: { 1: 0.5 },
    })

    const wrapper = mount(AvailableChannelModelsTable, {
      props: {
        columns,
        tooltips,
        pricingLabels,
        rows,
        loading: false,
        emptyLabel: 'No models',
        sortBy: 'model',
        sortOrder: 'asc',
      },
      global: {
        stubs: {
          Icon: IconStub,
          PlatformIcon: PlatformIconStub,
          GroupBadge: GroupBadgeStub,
        },
      },
    })

    expect(wrapper.text()).toContain('$0.4')
    expect(wrapper.text()).toContain('$2')
    expect(wrapper.text()).not.toContain('$0.8')
    expect(wrapper.text()).not.toContain('$4')
    expect(wrapper.text()).toContain('Public 8 Off')
    expect(wrapper.find('[data-testid="group-rate"]').exists()).toBe(false)
  })
})
