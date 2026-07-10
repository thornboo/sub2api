import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UpstreamSupplierModal from '../UpstreamSupplierModal.vue'

const {
  createUpstreamSupplier,
  updateUpstreamSupplier,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  createUpstreamSupplier: vi.fn(),
  updateUpstreamSupplier: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      createUpstreamSupplier,
      updateUpstreamSupplier
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError })
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

const mountModal = (props: Record<string, unknown> = {}) => mount(UpstreamSupplierModal, {
  props: {
    show: true,
    ...props
  } as any,
  global: {
    stubs: {
      BaseDialog: baseDialogStub,
      Icon: true
    }
  }
})

describe('UpstreamSupplierModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    createUpstreamSupplier.mockResolvedValue({ id: 7 })
    updateUpstreamSupplier.mockResolvedValue({ id: 7 })
  })

  it('creates a supplier with stable settlement defaults', async () => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('Supplier A')
    await wrapper.get('#upstream-supplier-note').setValue('shared wallet')
    await wrapper.get('[data-testid="supplier-default-credit-per-cny"]').setValue('2')
    await wrapper.get('[data-testid="supplier-default-reference-fx"]').setValue('7.2')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamSupplier).toHaveBeenCalledWith({
      name: 'Supplier A',
      note: 'shared wallet',
      default_effective_cny_per_usd: 0.5,
      default_reference_fx_rate: 7.2
    })
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('hydrates and updates the supplier default pool configuration', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'Supplier A',
        status: 'active',
        note: 'old note'
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    expect((wrapper.get('#upstream-supplier-name').element as HTMLInputElement).value).toBe('Supplier A')
    expect((wrapper.get('[data-testid="supplier-default-credit-per-cny"]').element as HTMLInputElement).value).toBe('0.5')

    await wrapper.get('#upstream-supplier-note').setValue('new note')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, {
      name: 'Supplier A',
      note: 'new note',
      default_effective_cny_per_usd: 2,
      default_reference_fx_rate: 6.8
    })
  })

  it('clears an existing supplier note with an explicit empty string', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'Supplier A',
        status: 'active',
        note: 'old note'
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    await wrapper.get('#upstream-supplier-note').setValue('')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, {
      name: 'Supplier A',
      note: '',
      default_effective_cny_per_usd: 2,
      default_reference_fx_rate: 6.8
    })
  })
})
