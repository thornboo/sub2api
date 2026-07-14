import { nextTick } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import EnterpriseMemberBatchPolicyDialog from '../EnterpriseMemberBatchPolicyDialog.vue'
import EnterpriseMemberBatchUsageDialog from '../EnterpriseMemberBatchUsageDialog.vue'
import BaseCheckbox from '@/components/common/BaseCheckbox.vue'

const i18n = createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })

const global = {
  plugins: [i18n],
  stubs: {
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
    ConfirmDialog: true,
    BaseCheckbox: true,
    Select: true
  }
}

describe('EnterpriseMemberBatchPolicyDialog', () => {
  it('lets users enable a limit by clicking its visible checkbox', async () => {
    const wrapper = shallowMount(EnterpriseMemberBatchPolicyDialog, {
      props: { show: true, memberCount: 2, availableGroups: [], saving: false },
      global: {
        ...global,
        stubs: { ...global.stubs, BaseCheckbox }
      }
    })

    await wrapper.findComponent(BaseCheckbox).trigger('click')

    const vm = wrapper.vm as unknown as { enabled: Record<string, boolean> }
    expect(vm.enabled.monthly_limit_usd).toBe(true)
  })

  it('emits only explicitly enabled policy fields', async () => {
    const wrapper = shallowMount(EnterpriseMemberBatchPolicyDialog, {
      props: { show: true, memberCount: 2, availableGroups: [], saving: false },
      global
    })
    const vm = wrapper.vm as unknown as {
      enabled: Record<string, boolean>
      values: Record<string, number>
      status: 'active' | 'disabled'
      submit: () => void
    }
    vm.enabled.monthly_limit_usd = true
    vm.enabled.status = true
    vm.values.monthly_limit_usd = 125
    vm.status = 'disabled'
    await nextTick()

    vm.submit()

    expect(wrapper.emitted('submit')).toEqual([[{
      group_mode: 'keep',
      monthly_limit_usd: 125,
      status: 'disabled'
    }]])
  })

  it('rejects a selected limit outside the database numeric range', async () => {
    const wrapper = shallowMount(EnterpriseMemberBatchPolicyDialog, {
      props: { show: true, memberCount: 2, availableGroups: [], saving: false },
      global
    })
    const vm = wrapper.vm as unknown as {
      enabled: Record<string, boolean>
      values: Record<string, number>
      submit: () => void
    }
    vm.enabled.monthly_limit_usd = true
    vm.values.monthly_limit_usd = 1_000_000_000_000
    await nextTick()

    vm.submit()

    expect(wrapper.emitted('submit')).toBeUndefined()
  })
})

describe('EnterpriseMemberBatchUsageDialog', () => {
  it('emits signed deltas when every resulting usage value remains non-negative', async () => {
    const wrapper = shallowMount(EnterpriseMemberBatchUsageDialog, {
      props: {
        show: true,
        saving: false,
        targets: [{ id: 11, name: 'A', monthlyUsed: 20, usage5h: 5, usage1d: 10, usage7d: 15 }]
      },
      global
    })
    const vm = wrapper.vm as unknown as {
      delta: Record<string, number>
      submit: () => void
    }
    vm.delta.monthly_used_delta = -5
    vm.delta.usage_5h_delta = 2
    await nextTick()

    vm.submit()

    expect(wrapper.emitted('submit')).toEqual([[{
      monthly_used_delta: -5,
      usage_5h_delta: 2,
      usage_1d_delta: 0,
      usage_7d_delta: 0
    }]])
  })

  it('does not emit a delta that would make a selected member negative', async () => {
    const wrapper = shallowMount(EnterpriseMemberBatchUsageDialog, {
      props: {
        show: true,
        saving: false,
        targets: [{ id: 11, name: 'A', monthlyUsed: 3, usage5h: 0, usage1d: 0, usage7d: 0 }]
      },
      global
    })
    const vm = wrapper.vm as unknown as {
      delta: Record<string, number>
      submit: () => void
    }
    vm.delta.monthly_used_delta = -4
    await nextTick()

    vm.submit()

    expect(wrapper.emitted('submit')).toBeUndefined()
  })

  it('does not emit a delta outside the database numeric range', async () => {
    const wrapper = shallowMount(EnterpriseMemberBatchUsageDialog, {
      props: {
        show: true,
        saving: false,
        targets: [{ id: 11, name: 'A', monthlyUsed: 3, usage5h: 0, usage1d: 0, usage7d: 0 }]
      },
      global
    })
    const vm = wrapper.vm as unknown as {
      delta: Record<string, number>
      submit: () => void
    }
    vm.delta.monthly_used_delta = 1_000_000_000_000
    await nextTick()

    vm.submit()

    expect(wrapper.emitted('submit')).toBeUndefined()
  })
})
