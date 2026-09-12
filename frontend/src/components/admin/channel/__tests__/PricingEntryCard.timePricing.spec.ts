import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PricingEntryCard from '../PricingEntryCard.vue'
import type { PricingFormEntry } from '../types'

vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function createEntry(billingMode: PricingFormEntry['billing_mode'] = 'token'): PricingFormEntry {
  return {
    models: [],
    billing_mode: billingMode,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    fast_multiplier: null,
    flex_multiplier: null,
    max_reasoning_effort_multiplier: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    time_pricing: {
      enabled: true,
      timezone: 'Asia/Shanghai',
      default_label: '平时',
      default_multiplier: 1,
      rules: [{ label: '高峰', start_time: '09:00', end_time: '12:00', multiplier: '2.00' }],
    },
    self_check_enabled_models: [],
  }
}

describe('PricingEntryCard time pricing visibility', () => {
  it('is shown for token pricing by default', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry() },
    })

    expect(wrapper.findComponent({ name: 'TimePricingEditor' }).exists()).toBe(true)
  })

  it('can be hidden for account-stat pricing reuse', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), hideTimePricing: true },
    })

    expect(wrapper.findComponent({ name: 'TimePricingEditor' }).exists()).toBe(false)
  })

  it('is hidden for non-token pricing', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry('per_request') },
    })

    expect(wrapper.findComponent({ name: 'TimePricingEditor' }).exists()).toBe(false)
  })
})

describe('PricingEntryCard request multipliers', () => {
  it('shows Fast, Flex, and Max effort controls only when explicitly enabled', () => {
    const hidden = shallowMount(PricingEntryCard, { props: { entry: createEntry() } })
    expect(hidden.text()).not.toContain('admin.channels.form.fastMultiplier')

    const shown = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), enableTierMultipliers: true },
    })
    expect(shown.text()).toContain('admin.channels.form.fastMultiplier')
    expect(shown.text()).toContain('admin.channels.form.flexMultiplier')
    expect(shown.text()).toContain('admin.channels.form.maxReasoningEffortMultiplier')
  })
})

describe('PricingEntryCard token interval guidance', () => {
  it('shows a non-blocking fallback warning for sparse token intervals', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: {
        entry: {
          ...createEntry(),
          intervals: [
            {
              min_tokens: 0,
              max_tokens: 272000,
              tier_label: '',
              input_price: 10,
              output_price: 50,
              cache_write_price: null,
              cache_read_price: 1,
              input_multiplier: null,
              output_multiplier: null,
              cache_write_multiplier: null,
              cache_read_multiplier: null,
              per_request_price: null,
              sort_order: 0,
            },
            {
              min_tokens: 272001,
              max_tokens: null,
              tier_label: '',
              input_price: 20,
              output_price: 75,
              cache_write_price: null,
              cache_read_price: 2,
              input_multiplier: null,
              output_multiplier: null,
              cache_write_multiplier: null,
              cache_read_multiplier: null,
              per_request_price: null,
              sort_order: 1,
            },
          ],
        },
      },
    })

    expect(wrapper.text()).toContain('admin.channels.form.intervalBoundsHint')
    expect(wrapper.findAll('[data-testid="token-interval-gap-warning"]')).toHaveLength(1)
  })
})
