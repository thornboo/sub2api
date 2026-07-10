import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountTableActions from '../AccountTableActions.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key === 'admin.accounts.createAccount' ? 'Add account' : key
    })
  }
})

const iconStub = {
  name: 'Icon',
  props: ['name', 'size'],
  template: '<span data-test="icon" :data-name="name" />'
}

describe('AccountTableActions', () => {
  it('uses the account action by default', () => {
    const wrapper = mount(AccountTableActions, {
      global: { stubs: { Icon: iconStub } }
    })

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(2)
    expect(buttons[1].text()).toBe('Add account')
    expect(buttons[1].find('[data-name="plus"]').exists()).toBe(false)
  })

  it('uses one supplier-specific primary action when configured', async () => {
    const wrapper = mount(AccountTableActions, {
      props: {
        createLabel: 'Add supplier',
        showCreateIcon: true
      },
      global: { stubs: { Icon: iconStub } }
    })

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(2)
    expect(buttons[1].text()).toBe('Add supplier')
    expect(buttons[1].find('[data-name="plus"]').exists()).toBe(true)

    await buttons[0].trigger('click')
    await buttons[1].trigger('click')

    expect(wrapper.emitted('refresh')).toHaveLength(1)
    expect(wrapper.emitted('create')).toHaveLength(1)
  })
})
