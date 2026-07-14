import { mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { describe, expect, it } from 'vitest'
import BaseCheckbox from '../BaseCheckbox.vue'

describe('BaseCheckbox', () => {
  it('activates the native checkbox when the visible control is clicked', async () => {
    const wrapper = mount(BaseCheckbox, {
      props: { modelValue: false, ariaLabel: 'Enable limit' }
    })

    await wrapper.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[true]])
    expect(wrapper.emitted('change')).toEqual([[true]])
  })

  it('does not activate when disabled', async () => {
    const wrapper = mount(BaseCheckbox, {
      props: { modelValue: false, disabled: true, ariaLabel: 'Enable limit' }
    })

    await wrapper.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.emitted('change')).toBeUndefined()
  })

  it('toggles exactly once when nested inside a label and its label text is clicked', async () => {
    const Host = defineComponent({
      components: { BaseCheckbox },
      setup() {
        const checked = ref(false)
        return { checked }
      },
      template: `
        <label>
          <BaseCheckbox v-model="checked" aria-label="Enable limit" />
          <span data-testid="label-text">Enable limit</span>
        </label>
        <output>{{ checked }}</output>
      `
    })
    const wrapper = mount(Host, { attachTo: document.body })

    ;(wrapper.get('[data-testid="label-text"]').element as HTMLElement).click()
    await nextTick()

    expect(wrapper.get('output').text()).toBe('true')
    wrapper.unmount()
  })

  it('keeps the native checkbox focusable and emits once through its change path', async () => {
    const wrapper = mount(BaseCheckbox, {
      attachTo: document.body,
      props: { modelValue: false, ariaLabel: 'Enable limit' }
    })
    const input = wrapper.get('input')

    ;(input.element as HTMLInputElement).focus()
    await input.setValue(true)

    expect(document.activeElement).toBe(input.element)
    expect(wrapper.emitted('update:modelValue')).toEqual([[true]])
    expect(wrapper.emitted('change')).toEqual([[true]])
    wrapper.unmount()
  })
})
