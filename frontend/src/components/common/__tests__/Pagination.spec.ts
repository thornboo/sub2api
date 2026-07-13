import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import Pagination from '../Pagination.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: '<button data-test="page-size-select" @click="$emit(\'update:modelValue\', 100)"></button>'
}

describe('Pagination', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('uses caller-provided page-size options instead of replacing them with global options', () => {
    const wrapper = mount(Pagination, {
      props: {
        total: 10,
        page: 1,
        pageSize: 5,
        pageSizeOptions: [5]
      },
      global: {
        stubs: {
          Select: SelectStub,
          Icon: true
        }
      }
    })

    expect(wrapper.findComponent(SelectStub).props('options')).toEqual([
      { value: 5, label: '5' }
    ])
  })

  it('can emit a page-size change without mutating the global persisted preference', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem')
    const wrapper = mount(Pagination, {
      props: {
        total: 200,
        page: 1,
        pageSize: 20,
        pageSizeOptions: [10, 20, 50, 100],
        persistPageSize: false
      },
      global: {
        stubs: {
          Select: SelectStub,
          Icon: true
        }
      }
    })

    await wrapper.find('[data-test="page-size-select"]').trigger('click')

    expect(wrapper.emitted('update:pageSize')).toEqual([[100]])
    expect(setItem).not.toHaveBeenCalled()
  })
})
