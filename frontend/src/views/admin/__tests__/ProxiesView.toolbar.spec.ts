import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import ProxiesView from '../ProxiesView.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(),
  getAllWithCount: vi.fn(),
  batchDelete: vi.fn(),
  exportData: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: { proxies: api } }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const ConfirmDialogStub = {
  props: ['show', 'title'],
  template: '<div v-if="show" data-test="confirmation">{{ title }}</div>'
}

const mountView = () => mount(ProxiesView, {
  attachTo: document.body,
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
      DataTable: {
        props: ['data'],
        template: '<div><div v-for="row in data" :key="row.id" data-test="row-selection"><slot name="cell-select" :row="row" /></div></div>'
      },
      ConfirmDialog: ConfirmDialogStub,
      ImportDataModal: { props: ['show'], template: '<div v-if="show" data-test="import-dialog" />' },
      BaseDialog: true,
      Select: true,
      Pagination: true,
      ProxyAdBanner: true,
      Icon: true
    }
  }
})

let wrapper: ReturnType<typeof mountView>
const body = new DOMWrapper(document.body)
const menuButton = (key: string) => body.get('[data-testid="proxy-tools-menu"]')
  .findAll('button').find(button => button.text() === key)!
const openMenu = async () => {
  await wrapper.get('[aria-label="common.more"]').trigger('click')
  await flushPromises()
}

beforeEach(() => {
  vi.clearAllMocks()
  localStorage.clear()
  api.list.mockResolvedValue({ items: [{ id: 7, name: 'Fixture proxy' }], total: 1, pages: 1 })
  api.getAllWithCount.mockResolvedValue([])
})
afterEach(() => {
  wrapper?.unmount()
  document.body.innerHTML = ''
})

describe('proxy toolbar secondary tools', () => {
  it('keeps deletion disabled without selection and opens confirmation without deleting selected rows', async () => {
    wrapper = mountView()
    await flushPromises()
    await openMenu()
    expect(menuButton('admin.proxies.batchDeleteAction').attributes('disabled')).toBeDefined()
    await wrapper.get('[aria-label="common.more"]').trigger('click')
    await wrapper.get('[data-test="row-selection"] button').trigger('click')
    await openMenu()
    expect(menuButton('admin.proxies.batchDeleteAction').attributes('disabled')).toBeUndefined()
    expect(menuButton('admin.proxies.dataExportSelected').exists()).toBe(true)
    await menuButton('admin.proxies.batchDeleteAction').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="confirmation"]').text()).toBe('admin.proxies.batchDelete')
    expect(body.find('[data-testid="proxy-tools-menu"]').exists()).toBe(false)
    expect(api.batchDelete).not.toHaveBeenCalled()
  })

  it('closes the tools popover when opening import and retains export confirmation', async () => {
    wrapper = mountView()
    await flushPromises()
    await openMenu()
    await menuButton('admin.proxies.dataImport').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="import-dialog"]').exists()).toBe(true)
    expect(body.find('[data-testid="proxy-tools-menu"]').exists()).toBe(false)
    wrapper.getComponent('[data-test="import-dialog"]').vm.$emit('close')
    await flushPromises()
    await openMenu()
    await menuButton('admin.proxies.dataExport').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="confirmation"]').text()).toBe('admin.proxies.dataExport')
    expect(api.exportData).not.toHaveBeenCalled()
  })
})
