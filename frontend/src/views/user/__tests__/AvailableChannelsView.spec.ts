import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { UserAvailableChannel } from '@/api/channels'
import { BILLING_MODE_TOKEN } from '@/constants/channel'
import AvailableChannelsView from '../AvailableChannelsView.vue'

const mocks = vi.hoisted(() => ({
  auth: { isAdmin: false },
  getAvailable: vi.fn(),
  getUserGroupRates: vi.fn(),
  getAvailableCatalog: vi.fn(),
  exportCatalog: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/channels', () => ({ default: { getAvailable: mocks.getAvailable } }))
vi.mock('@/api/groups', () => ({ default: { getUserGroupRates: mocks.getUserGroupRates } }))
vi.mock('@/api/admin/channels', () => ({ default: { getAvailableCatalog: mocks.getAvailableCatalog } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => mocks.auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/utils/availableChannelsCatalog', async (importOriginal) => ({
  ...await importOriginal<typeof import('@/utils/availableChannelsCatalog')>(),
  exportAvailableChannelsCatalog: mocks.exportCatalog,
}))

const MarketplaceStub = defineComponent({
  props: ['cards', 'userGroupRates'],
  template: '<section><span v-for="card in cards" :key="card.id">{{ card.name }}</span></section>',
})

function catalog(name: string, models: string[]): UserAvailableChannel[] {
  return [{
    name,
    description: '',
    platforms: [{
      platform: 'openai',
      groups: [{
        id: 1, name: 'public', platform: 'openai', subscription_type: 'standard',
        rate_multiplier: 0.8, is_exclusive: false, peak_rate_enabled: false,
        peak_start: '', peak_end: '', peak_rate_multiplier: 1,
      }],
      supported_models: models.map(model => ({
        name: model,
        platform: 'openai',
        catalog_group_ids: [1],
        pricing: {
          billing_mode: BILLING_MODE_TOKEN,
          input_price: 0.000002, output_price: 0.000004,
          cache_write_price: null, cache_read_price: null,
          image_input_price: null, image_output_price: null,
          per_request_price: null, intervals: [],
        },
      })),
    }],
  }]
}

function mountPage() {
  return mount(AvailableChannelsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show" role="dialog"><slot /><slot name="footer" /></div>',
        },
        AvailableModelMarketplace: MarketplaceStub,
        Icon: true,
        Select: true,
      },
    },
  })
}

enableAutoUnmount(afterEach)

describe('AvailableChannelsView catalog and export', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.auth.isAdmin = false
    mocks.getAvailable.mockResolvedValue(catalog('Visible channel', ['model-alpha', 'model-beta']))
    mocks.getUserGroupRates.mockResolvedValue({ 1: 0.5 })
    mocks.getAvailableCatalog.mockResolvedValue(catalog('Admin channel', ['admin-only-model']))
    mocks.exportCatalog.mockResolvedValue(undefined)
  })

  it('keeps model search and user-priced export available from the card view', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(wrapper.getComponent(MarketplaceStub).props('cards')).toHaveLength(2)

    await wrapper.get('input').setValue('MODEL-BETA')
    expect(wrapper.getComponent(MarketplaceStub).props('cards')).toMatchObject([{ name: 'model-beta' }])

    await wrapper.get('button[title="availableChannels.exportExcel"]').trigger('click')
    await wrapper.get('[role="dialog"] .btn-primary').trigger('click')
    await flushPromises()

    const [rows, , rates] = mocks.exportCatalog.mock.calls[0]
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({ modelName: 'model-beta', effectiveRateMultiplier: 0.5 })
    expect(rows[0].pricing.input_price).toBeCloseTo(0.000001)
    expect(rates).toEqual({ 1: 0.5 })
    expect(mocks.getAvailableCatalog).not.toHaveBeenCalled()
    expect(mocks.showSuccess).toHaveBeenCalledWith('availableChannels.export.success')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })

  it('keeps the admin export catalog separate from visible cards and personal rates', async () => {
    mocks.auth.isAdmin = true
    const wrapper = mountPage()
    await flushPromises()
    const cards = wrapper.getComponent(MarketplaceStub).props('cards')
    expect(cards.map((card: { name: string }) => card.name)).toEqual(['model-alpha', 'model-beta'])

    await wrapper.get('button[title="availableChannels.exportExcel"]').trigger('click')
    await wrapper.get('[role="dialog"] .btn-primary').trigger('click')
    await flushPromises()

    const [rows, , rates] = mocks.exportCatalog.mock.calls[0]
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({ modelName: 'admin-only-model', effectiveRateMultiplier: 0.8 })
    expect(rows[0].pricing.input_price).toBeCloseTo(0.0000016)
    expect(rates).toEqual({})
    expect(mocks.showError).not.toHaveBeenCalled()
  })
})
