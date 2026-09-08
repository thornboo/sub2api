import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n, type MessageFunction } from 'vue-i18n'
import { baseCompile } from '@intlify/message-compiler'
import { describe, expect, it } from 'vitest'

import AvailableModelMarketplace from '../AvailableModelMarketplace.vue'
import type { UserAvailableChannel } from '@/api/channels'
import { BILLING_MODE_IMAGE } from '@/constants/channel'
import en from '@/i18n/locales/en/devzz'
import zh from '@/i18n/locales/zh/devzz'
import { buildAvailableModelMarketplaceCards } from '@/utils/availableModelMarketplace'

type CompiledMessages = { [key: string]: MessageFunction<string> | CompiledMessages }

// The runtime-only i18n build needs precompiled functions for our trusted locale files.
function compileMessages(messages: Record<string, unknown>): CompiledMessages {
  return Object.fromEntries(Object.entries(messages).map(([key, value]) => [
    key,
    typeof value === 'string'
      ? new Function(`return ${baseCompile(value, { mode: 'arrow' }).code}`)() as MessageFunction<string>
      : compileMessages(value as Record<string, unknown>),
  ]))
}

const messages = { zh: compileMessages(zh), en: compileMessages(en) }

describe('AvailableModelMarketplace configured catalog copy', () => {
  it.each([
    { locale: 'zh', message: '暂未发布原生端点信息', rateLabel: '生图倍率' },
    { locale: 'en', message: 'No native endpoint information is published yet.', rateLabel: 'Image rate' },
  ])('keeps a configured model visible with truthful endpoint copy in $locale', ({ locale, message, rateLabel }) => {
    const channels: UserAvailableChannel[] = [{
      name: 'GPT生图',
      description: '',
      platforms: [{
        platform: 'openai',
        groups: [{
          id: 1,
          name: '生图（gpt）',
          platform: 'openai',
          subscription_type: 'standard',
          rate_multiplier: 0.1,
          image_rate_independent: true,
          image_rate_multiplier: 0.5,
          peak_rate_enabled: false,
          peak_start: '',
          peak_end: '',
          peak_rate_multiplier: 1,
          is_exclusive: false,
        }],
        supported_models: [{
          name: 'gpt-image-2',
          platform: 'openai',
          catalog_group_ids: [1],
          route_group_ids: [],
          supported_endpoints: [],
          pricing: {
            billing_mode: BILLING_MODE_IMAGE,
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            image_input_price: null,
            image_output_price: null,
            per_request_price: 3.5,
            intervals: [],
          },
        }],
      }],
    }]
    const wrapper = mount(AvailableModelMarketplace, {
      props: {
        cards: buildAvailableModelMarketplaceCards(channels),
        loading: false,
        emptyLabel: 'No models',
        applyRateMultiplier: true,
        showGroupRates: true,
        userGroupRates: { 1: 0.01 },
        pricingLabels: {
          billingModeToken: 'Token',
          billingModePerRequest: 'Request',
          billingModeImage: 'Image',
          noPricing: 'No pricing',
          unitPerMillion: '/ 1M tokens',
          unitPerRequest: '/ image',
        },
      },
      global: {
        plugins: [createPinia(), createI18n({ legacy: false, locale, messages })],
        stubs: {
          Icon: true,
          ModelIcon: true,
          PlatformIcon: true,
          GroupBadge: { props: ['name'], template: '<span>{{ name }}</span>' },
        },
      },
    })

    try {
      const card = wrapper.get('[data-testid="available-model-card"]')
      expect(card.text()).toContain('gpt-image-2')
      expect(card.text()).toContain('$1.75 / image')
      expect(wrapper.get('[data-testid="group-rate-badge"]').text()).toContain(`${rateLabel} 0.5x`)
      expect(card.get('footer').text()).toContain(message)
      expect(card.get('footer').text()).not.toMatch(/兼容转换路由|compatibility route/i)
      expect(card.get('footer').findAll('button')).toHaveLength(0)
    } finally {
      wrapper.unmount()
    }
  })
})
