import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UpstreamSupplierModal from '../UpstreamSupplierModal.vue'
import Select from '@/components/common/Select.vue'

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

  it('creates a supplier with stable settlement defaults and disabled balance config', async () => {
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
      default_reference_fx_rate: 7.2,
      balance_config: {
        enabled: false,
        provider: 'newapi',
        base_url: '',
        user_id: undefined,
        access_token: undefined
      }
    })
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('creates an enabled New API balance config without a standalone post-save refresh', async () => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('Supplier A')
    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(true)
    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://newapi.example.com///')
    await wrapper.get('[data-testid="supplier-balance-user-id"]').setValue('15')
    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('access-token')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamSupplier).toHaveBeenCalledWith(expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'newapi',
        base_url: 'https://newapi.example.com',
        user_id: 15,
        access_token: 'access-token'
      }
    }))
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('keeps New API user ID optional when the field is blank', async () => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('Supplier A')
    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(true)
    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://newapi.example.com')
    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('access-token')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamSupplier).toHaveBeenCalledWith(expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'newapi',
        base_url: 'https://newapi.example.com',
        user_id: undefined,
        access_token: 'access-token'
      }
    }))
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it.each(['0', '-1', '1.5'])('blocks invalid New API user ID value %s', async (userID) => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('Supplier A')
    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(true)
    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://newapi.example.com')
    await wrapper.get('[data-testid="supplier-balance-user-id"]').setValue(userID)
    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('access-token')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamSupplier).not.toHaveBeenCalled()
    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('creates an enabled SoleAPI balance config with API Key fields and no user ID', async () => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('SoleAPI Supplier')
    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(true)
    await wrapper.findComponent(Select).vm.$emit('update:modelValue', 'soleapi')
    await flushPromises()

    expect(wrapper.find('[data-testid="supplier-balance-user-id"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.upstreamCost.supplierBalance.apiKey')
    expect(wrapper.get('[data-testid="supplier-balance-base-url"]').attributes('placeholder')).toBe('https://soleapi.com')
    expect(wrapper.get('[data-testid="supplier-balance-token"]').attributes('placeholder')).toBe('admin.accounts.upstreamCost.supplierBalance.apiKeyPlaceholder')

    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://soleapi.com///')
    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('sk-sole-test')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamSupplier).toHaveBeenCalledWith(expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'soleapi',
        base_url: 'https://soleapi.com',
        user_id: undefined,
        access_token: 'sk-sole-test'
      }
    }))
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('creates an enabled sub2api balance config with API Key fields and no user ID', async () => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('sub2api Supplier')
    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(true)
    await wrapper.findComponent(Select).vm.$emit('update:modelValue', 'sub2api')
    await flushPromises()

    expect(wrapper.find('[data-testid="supplier-balance-user-id"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.upstreamCost.supplierBalance.apiKey')
    expect(wrapper.text()).toContain('admin.accounts.upstreamCost.supplierBalance.sub2ApiKeyHint')
    expect(wrapper.get('[data-testid="supplier-balance-base-url"]').attributes('placeholder')).toBe('https://sub2api.example.com')
    expect(wrapper.get('[data-testid="supplier-balance-token"]').attributes('placeholder')).toBe('admin.accounts.upstreamCost.supplierBalance.sub2ApiKeyPlaceholder')

    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://sub2api.example.com///')
    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('sk-sub2api-test')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(createUpstreamSupplier).toHaveBeenCalledWith(expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'sub2api',
        base_url: 'https://sub2api.example.com',
        user_id: undefined,
        access_token: 'sk-sub2api-test'
      }
    }))
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
      default_reference_fx_rate: 6.8,
      balance_config: {
        enabled: false,
        provider: 'newapi',
        base_url: '',
        user_id: undefined,
        access_token: undefined
      }
    })
  })

  it('preserves an existing balance token when editing without re-entering it', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'Supplier A',
        status: 'active',
        note: 'old note',
        balance_config: {
          enabled: true,
          provider: 'newapi',
          base_url: 'https://newapi.example.com',
          user_id: 15,
          has_access_token: true
        }
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    await wrapper.get('#upstream-supplier-note').setValue('new note')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'newapi',
        base_url: 'https://newapi.example.com',
        user_id: 15,
        access_token: undefined
      }
    }))
  })

  it('preserves an existing SoleAPI API Key when editing the same provider and site', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'SoleAPI Supplier',
        status: 'active',
        note: 'old note',
        balance_config: {
          enabled: true,
          provider: 'soleapi',
          base_url: 'https://soleapi.com',
          has_access_token: true
        }
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    expect(wrapper.find('[data-testid="supplier-balance-user-id"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="supplier-balance-token"]').attributes('placeholder')).toBe('admin.accounts.upstreamCost.supplierBalance.accessTokenConfigured')

    await wrapper.get('#upstream-supplier-note').setValue('new note')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'soleapi',
        base_url: 'https://soleapi.com',
        user_id: undefined,
        access_token: undefined
      }
    }))
  })

  it('preserves an existing sub2api API Key when editing the same provider and site', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'sub2api Supplier',
        status: 'active',
        note: 'old note',
        balance_config: {
          enabled: true,
          provider: 'sub2api',
          base_url: 'https://sub2api.example.com',
          has_access_token: true
        }
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    expect(wrapper.find('[data-testid="supplier-balance-user-id"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="supplier-balance-token"]').attributes('placeholder')).toBe('admin.accounts.upstreamCost.supplierBalance.accessTokenConfigured')

    await wrapper.get('#upstream-supplier-note').setValue('new note')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, expect.objectContaining({
      balance_config: {
        enabled: true,
        provider: 'sub2api',
        base_url: 'https://sub2api.example.com',
        user_id: undefined,
        access_token: undefined
      }
    }))
  })

  it('keeps same-site balance credentials when disabled during edit', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'Supplier A',
        status: 'active',
        note: 'old note',
        balance_config: {
          enabled: true,
          provider: 'newapi',
          base_url: 'https://newapi.example.com',
          has_access_token: true
        }
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(false)
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, expect.objectContaining({
      balance_config: {
        enabled: false,
        provider: 'newapi',
        base_url: 'https://newapi.example.com',
        user_id: undefined,
        access_token: undefined
      }
    }))
  })

  it('requires a new token when changing the saved supplier website', async () => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'Supplier A',
        status: 'active',
        note: 'old note',
        balance_config: {
          enabled: true,
          provider: 'newapi',
          base_url: 'https://old.example.com',
          has_access_token: true
        }
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://new.example.com')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).not.toHaveBeenCalled()

    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('new-token')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, expect.objectContaining({
      balance_config: expect.objectContaining({
        base_url: 'https://new.example.com',
        access_token: 'new-token'
      })
    }))
  })

  it.each([
    { provider: 'soleapi', placeholder: 'apiKeyPlaceholder', hint: 'apiKeyHint' },
    { provider: 'sub2api', placeholder: 'sub2ApiKeyPlaceholder', hint: 'sub2ApiKeyHint' }
  ])('requires a new credential when changing only the saved balance provider to $provider', async ({ provider, placeholder, hint }) => {
    const wrapper = mountModal({
      supplier: {
        id: 7,
        name: 'Supplier A',
        status: 'active',
        note: 'old note',
        balance_config: {
          enabled: true,
          provider: 'newapi',
          base_url: 'https://newapi.example.com',
          user_id: 15,
          has_access_token: true
        }
      },
      costPool: {
        id: 9,
        supplier_id: 7,
        default_effective_cny_per_usd: 2,
        default_reference_fx_rate: 6.8
      }
    })

    await wrapper.findComponent(Select).vm.$emit('update:modelValue', provider)
    await flushPromises()
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="supplier-balance-token"]').attributes('placeholder')).toBe(`admin.accounts.upstreamCost.supplierBalance.${placeholder}`)
    expect(wrapper.text()).toContain(`admin.accounts.upstreamCost.supplierBalance.${hint}`)
    expect(wrapper.text()).toContain('admin.accounts.upstreamCost.supplierBalance.accessTokenProviderChangedHint')

    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('sk-replacement')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, expect.objectContaining({
      balance_config: {
        enabled: true,
        provider,
        base_url: 'https://newapi.example.com',
        user_id: undefined,
        access_token: 'sk-replacement'
      }
    }))
  })

  it('renders the shared site type selector and leaves balance refresh to the parent list reload after save', async () => {
    const wrapper = mountModal()

    await wrapper.get('#upstream-supplier-name').setValue('Supplier A')
    await wrapper.get('[data-testid="supplier-balance-enabled"]').setValue(true)
    expect(wrapper.find('select[data-testid="supplier-balance-provider"]').exists()).toBe(false)
    expect(wrapper.get('#upstream-supplier-balance-provider').attributes('aria-label')).toBe('admin.accounts.upstreamCost.supplierBalance.provider')
    await wrapper.get('[data-testid="supplier-balance-base-url"]').setValue('https://newapi.example.com')
    await wrapper.get('[data-testid="supplier-balance-token"]').setValue('access-token')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
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
      default_reference_fx_rate: 6.8,
      balance_config: {
        enabled: false,
        provider: 'newapi',
        base_url: '',
        user_id: undefined,
        access_token: undefined
      }
    })
  })
})
