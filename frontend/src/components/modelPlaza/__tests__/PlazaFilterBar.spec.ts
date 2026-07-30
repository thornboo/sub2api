import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import Select from '@/components/common/Select.vue'
import PlazaFilterBar from '../PlazaFilterBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const groups = [
  { id: 1, name: 'anthropic', platform: 'anthropic' },
  { id: 2, name: 'openai', platform: 'openai' },
]

function mountFilterBar() {
  return mount(PlazaFilterBar, {
    props: {
      platforms: ['anthropic', 'openai'],
      groups,
      platform: 'all',
      groupId: 2,
      search: '',
    },
    global: {
      stubs: {
        Icon: true,
        PlatformIcon: true,
      },
    },
  })
}

describe('PlazaFilterBar', () => {
  it('uses the shared dropdown control for platform and group filters', () => {
    const wrapper = mountFilterBar()
    const selects = wrapper.findAllComponents(Select)

    expect(selects).toHaveLength(2)
    expect(selects[0].props('id')).toBe('model-plaza-platform-filter')
    expect(selects[1].props('id')).toBe('model-plaza-group-filter')
    expect(wrapper.find('input[type="text"]').exists()).toBe(true)
    expect(wrapper.findAll('.chip-tinted')).toHaveLength(0)

    wrapper.unmount()
  })

  it('keeps platform and group options mutually constrained', async () => {
    const wrapper = mountFilterBar()
    const selects = wrapper.findAllComponents(Select)
    const platformOptions = selects[0].props('options') as Array<{
      value: string
      disabled?: boolean
    }>

    expect(platformOptions.find((option) => option.value === 'anthropic')?.disabled).toBe(true)
    expect(platformOptions.find((option) => option.value === 'openai')?.disabled).toBe(false)

    await wrapper.setProps({ platform: 'anthropic', groupId: 'all' })

    const groupOptions = wrapper.findAllComponents(Select)[1].props('options') as Array<{
      value: string | number
      disabled?: boolean
    }>
    expect(groupOptions.find((option) => option.value === 1)?.disabled).toBe(false)
    expect(groupOptions.find((option) => option.value === 2)?.disabled).toBe(true)

    wrapper.unmount()
  })

  it('forwards dropdown selections through the filter contract', () => {
    const wrapper = mountFilterBar()
    const selects = wrapper.findAllComponents(Select)

    selects[0].vm.$emit('update:modelValue', 'openai')
    selects[1].vm.$emit('update:modelValue', 1)

    expect(wrapper.emitted('update:platform')).toEqual([['openai']])
    expect(wrapper.emitted('update:groupId')).toEqual([[1]])

    wrapper.unmount()
  })
})
