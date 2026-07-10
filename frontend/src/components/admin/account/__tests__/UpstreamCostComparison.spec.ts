import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UpstreamCostComparison from '../UpstreamCostComparison.vue'

const {
  updateUpstreamSupplier,
  deleteUpstreamSupplier,
  showWarning,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  updateUpstreamSupplier: vi.fn(),
  deleteUpstreamSupplier: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      updateUpstreamSupplier,
      deleteUpstreamSupplier
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showWarning,
    showSuccess,
    showError
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const messages: Record<string, string> = {
    'admin.accounts.upstreamCost.addSupplier': 'Add',
    'admin.accounts.upstreamCost.newSupplierName': 'New supplier',
    'admin.accounts.upstreamCost.newSupplierNamePlaceholder': 'Supplier A',
    'admin.accounts.upstreamCost.saveSupplier': 'Save supplier',
    'admin.accounts.upstreamCost.supplierCreateHint': 'Create hint',
    'admin.accounts.upstreamCost.editSupplierTitle': 'Edit supplier',
    'admin.accounts.upstreamCost.supplierNotePlaceholder': 'Supplier note',
    'admin.accounts.upstreamCost.supplierNameRequired': 'Enter a supplier name',
    'admin.accounts.upstreamCost.supplierCreated': 'Supplier created',
    'admin.accounts.upstreamCost.supplierUpdated': 'Supplier updated',
    'admin.accounts.upstreamCost.supplierArchived': 'Supplier archived',
    'admin.accounts.upstreamCost.supplierUnarchived': 'Supplier restored',
    'admin.accounts.upstreamCost.supplierDeleted': 'Supplier deleted',
    'admin.accounts.upstreamCost.supplierCreateFailed': 'Create failed',
    'admin.accounts.upstreamCost.supplierUpdateFailed': 'Update failed',
    'admin.accounts.upstreamCost.supplierDeleteFailed': 'Delete failed',
    'admin.accounts.upstreamCost.archive': 'Archive',
    'admin.accounts.upstreamCost.unarchive': 'Restore',
    'admin.accounts.upstreamCost.archiveSupplierTitle': 'Archive supplier',
    'admin.accounts.upstreamCost.archiveSupplierConfirm': 'Archive supplier {name}? {count} bound.',
    'admin.accounts.upstreamCost.deleteSupplierTitle': 'Delete supplier',
    'admin.accounts.upstreamCost.deleteSupplierConfirm':
      'Delete supplier {name}? Only never-used suppliers can be deleted.',
    'admin.accounts.upstreamCost.supplier': 'Supplier',
    'admin.accounts.upstreamCost.boundAccounts': 'Bound',
    'admin.accounts.upstreamCost.currentCost': 'Cost',
    'admin.accounts.upstreamCost.rechargeRatio': 'Ratio',
    'admin.accounts.upstreamCost.rechargeDiscount': 'Discount',
    'admin.accounts.upstreamCost.status': 'Status',
    'admin.accounts.upstreamCost.noSuppliers': 'No suppliers',
    'admin.accounts.upstreamCost.supplierNoPool': 'No pool',
    'admin.accounts.upstreamCost.completeStatus': 'Complete',
    'admin.accounts.upstreamCost.archivedStatus': 'Archived',
    'admin.accounts.upstreamCost.needsConfig': 'Needs setup',
    'admin.accounts.upstreamCost.discountSuffix': '/10',
    'admin.accounts.upstreamCost.notConfigured': 'Not configured',
    'admin.accounts.upstreamCost.errors.hasBoundAccounts': 'Supplier has bound accounts',
    'admin.accounts.upstreamCost.errors.hasBindingHistory': 'Supplier has binding history',
    'admin.accounts.upstreamCost.rechargeRecords.action': 'Recharge records',
    'admin.accounts.upstreamCost.rechargeRecords.records': 'Records',
    'admin.accounts.columns.actions': 'Actions',
    'common.refresh': 'Refresh',
    'common.save': 'Save',
    'common.cancel': 'Cancel',
    'common.edit': 'Edit',
    'common.delete': 'Delete',
    'common.loading': 'Loading'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        let message = messages[key] || key
        for (const [paramKey, value] of Object.entries(params || {})) {
          message = message.replace(`{${paramKey}}`, String(value))
        }
        return message
      }
    })
  }
})

const confirmDialogStub = {
  name: 'ConfirmDialog',
  props: ['show', 'title', 'message', 'confirmText', 'danger'],
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="show" data-test="confirm-dialog">
      <p>{{ title }}</p>
      <p>{{ message }}</p>
      <button class="confirm" type="button" @click="$emit('confirm')">{{ confirmText }}</button>
      <button class="cancel" type="button" @click="$emit('cancel')">cancel</button>
    </div>
  `
}

function mountComparison(options: {
  bindingCount?: number
  supplierStatus?: string
  poolArchivedAt?: string | null
  isSystem?: boolean
  supplierName?: string
  hasSnapshot?: boolean
} = {}) {
  const bindingCount = options.bindingCount ?? 0
  const supplierStatus = options.supplierStatus ?? 'active'
  const poolArchivedAt = options.poolArchivedAt ?? null
  const supplierName = options.supplierName ?? 'Supplier A'
  const hasSnapshot = options.hasSnapshot ?? true

  return mount(UpstreamCostComparison, {
    props: {
      suppliers: [
        {
          id: 7,
          name: supplierName,
          status: supplierStatus,
          note: 'old note',
          is_system: options.isSystem === true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
          archived_at: null
        }
      ],
      costPools: [
        {
          id: 9,
          supplier_id: 7,
          supplier_name: supplierName,
          name: '主余额池',
          is_default: true,
          status: supplierStatus,
          archived_at: poolArchivedAt,
          reference_fx_rate: 7,
          current_effective_cny_per_usd: 6,
          current_snapshot_id: hasSnapshot ? 10 : null,
          binding_count: bindingCount,
          record_count: 0
        }
      ],
      loading: false,
      error: null
    } as any,
    global: {
      stubs: {
        Icon: true,
        ConfirmDialog: confirmDialogStub
      }
    }
  })
}

const findButton = (wrapper: ReturnType<typeof mountComparison>, label: string) => {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().trim() === label)
  expect(button, `button ${label}`).toBeTruthy()
  return button!
}

describe('UpstreamCostComparison', () => {
  afterEach(() => {
    vi.clearAllMocks()
  })

  it('starts directly with the supplier table without a summary header', () => {
    const wrapper = mountComparison()

    expect(wrapper.find('table').exists()).toBe(true)
    expect(wrapper.find('h3').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('Manage supplier costs')
    expect(wrapper.text()).not.toContain('Configured')
    expect(wrapper.text()).not.toContain('Best')
  })

  it('does not present configured defaults as current cost without a real snapshot', () => {
    const wrapper = mountComparison({ hasSnapshot: false })

    expect(wrapper.text()).toContain('Needs setup')
    expect(wrapper.text()).not.toContain('6 CNY/USD')
  })

  it('hands supplier editing to the page-level modal', async () => {
    const wrapper = mountComparison()

    await findButton(wrapper, 'Edit').trigger('click')

    expect(wrapper.emitted('edit-supplier')).toEqual([[7]])
    expect(updateUpstreamSupplier).not.toHaveBeenCalled()
  })

  it('archives a supplier and forces cost-pool refresh', async () => {
    updateUpstreamSupplier.mockResolvedValue({
      id: 7,
      name: 'Supplier A',
      status: 'archived',
      note: 'old note'
    })
    const wrapper = mountComparison()

    await findButton(wrapper, 'Archive').trigger('click')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, { status: 'archived' })
    expect(showSuccess).toHaveBeenCalledWith('Supplier archived')
    expect(wrapper.emitted('refresh')).toEqual([[{ forcePools: true }]])
  })

  it('requires confirmation before archiving a supplier with active bindings', async () => {
    updateUpstreamSupplier.mockResolvedValue({
      id: 7,
      name: 'Supplier A',
      status: 'archived',
      note: 'old note'
    })
    const wrapper = mountComparison({ bindingCount: 2 })

    await findButton(wrapper, 'Archive').trigger('click')

    expect(updateUpstreamSupplier).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="confirm-dialog"]').text()).toContain('2 bound')

    await wrapper.find('[data-test="confirm-dialog"] .confirm').trigger('click')
    await flushPromises()

    expect(updateUpstreamSupplier).toHaveBeenCalledWith(7, { status: 'archived' })
    expect(showSuccess).toHaveBeenCalledWith('Supplier archived')
    expect(wrapper.emitted('refresh')).toEqual([[{ forcePools: true }]])
  })

  it('hides mutable actions for system suppliers using the API flag', async () => {
    const wrapper = mountComparison({ isSystem: true, supplierName: 'Uncategorized' })

    expect(wrapper.text()).not.toContain('Uncategorized')
    expect(wrapper.findAll('button').map((button) => button.text().trim())).not.toContain('Edit')
    expect(wrapper.findAll('button').map((button) => button.text().trim())).not.toContain('Archive')
    expect(wrapper.findAll('button').map((button) => button.text().trim())).not.toContain('Delete')
  })

  it('deletes a clean supplier after confirmation', async () => {
    deleteUpstreamSupplier.mockResolvedValue(undefined)
    const wrapper = mountComparison()

    await findButton(wrapper, 'Delete').trigger('click')
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)
    await wrapper.find('[data-test="confirm-dialog"] .confirm').trigger('click')
    await flushPromises()

    expect(deleteUpstreamSupplier).toHaveBeenCalledWith(7)
    expect(showSuccess).toHaveBeenCalledWith('Supplier deleted')
    expect(wrapper.emitted('refresh')).toEqual([[{ forcePools: true }]])
  })

  it('blocks delete locally while accounts are still bound', async () => {
    const wrapper = mountComparison({ bindingCount: 2 })

    await findButton(wrapper, 'Delete').trigger('click')

    expect(showWarning).toHaveBeenCalledWith('Supplier has bound accounts')
    expect(deleteUpstreamSupplier).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(false)
  })
})
