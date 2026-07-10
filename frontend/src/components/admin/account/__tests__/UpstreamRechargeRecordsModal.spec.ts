import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UpstreamRechargeRecordsModal from '../UpstreamRechargeRecordsModal.vue'

const {
  listUpstreamCostPoolRechargeRecords,
  createUpstreamCostPoolRechargeRecord,
  showSuccess
} = vi.hoisted(() => ({
  listUpstreamCostPoolRechargeRecords: vi.fn(),
  createUpstreamCostPoolRechargeRecord: vi.fn(),
  showSuccess: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      listUpstreamCostPoolRechargeRecords,
      createUpstreamCostPoolRechargeRecord
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const baseDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show"><h2>{{ title }}</h2><slot /><slot name="footer" /></div>'
}

const selectStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<div data-test="select"><slot name="selected" :option="{ label: modelValue }" /></div>'
}

const costPool = {
  id: 9,
  supplier_id: 7,
  supplier_name: 'Supplier A',
  name: '主余额池',
  status: 'active',
  base_currency: 'CNY',
  credit_currency: 'USD',
  reference_fx_rate: 6.8,
  default_effective_cny_per_usd: 2,
  default_reference_fx_rate: 7,
  cost_method: 'latest',
  current_effective_cny_per_usd: 1.5,
  balance_query_enabled: false,
  binding_count: 1,
  record_count: 0,
  created_at: '2026-07-10T00:00:00Z',
  updated_at: '2026-07-10T00:00:00Z'
}

const emptyResult = {
  items: [],
  summary: {
    record_count: 0,
    total_paid_amount: 0,
    total_received_credit_amount: 0,
    reference_fx_rate: 7
  }
}

const mountModal = () => mount(UpstreamRechargeRecordsModal, {
  props: {
    show: true,
    costPool
  } as any,
  global: {
    stubs: {
      BaseDialog: baseDialogStub,
      ConfirmDialog: true,
      Select: selectStub,
      Icon: true
    }
  }
})

describe('UpstreamRechargeRecordsModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listUpstreamCostPoolRechargeRecords.mockResolvedValue(emptyResult)
    createUpstreamCostPoolRechargeRecord.mockResolvedValue({ id: 1 })
  })

  it('uses supplier defaults for the common recharge path', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const paidInput = wrapper.findAll('input[type="number"]')[0]
    await paidInput.setValue('20')
    await flushPromises()

    expect(wrapper.get('[data-testid="auto-received-credit"]').text()).toBe('10')
    expect(wrapper.text()).toContain('admin.accounts.upstreamCost.rechargeRecords.overrideThisRecord')
    expect(wrapper.find('input[step="0.000001"][min="0.000001"]').exists()).toBe(false)

    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamCostPoolRechargeRecord).toHaveBeenCalledWith(9, expect.objectContaining({
      type: 'recharge',
      paid_amount: 20,
      paid_currency: 'CNY',
      received_credit_amount: 10,
      received_credit_currency: 'USD',
      reference_fx_rate: 7
    }))
  })

  it('reveals per-record overrides only when requested', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const overrideButton = wrapper.findAll('button').find((button) => (
      button.text().includes('admin.accounts.upstreamCost.rechargeRecords.overrideThisRecord')
    ))
    expect(overrideButton).toBeTruthy()
    await overrideButton!.trigger('click')

    expect(wrapper.find('[data-testid="auto-received-credit"]').exists()).toBe(false)
    expect(wrapper.findAll('input[type="number"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('admin.accounts.upstreamCost.rechargeRecords.useDefaultCalculation')
  })
})
