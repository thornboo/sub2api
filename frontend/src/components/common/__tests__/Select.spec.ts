import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick, ref, type ComponentPublicInstance } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import Select from '../Select.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const IconStub = { template: '<span />' }

describe('Select outside click behavior', () => {
  let wrapper: VueWrapper<ComponentPublicInstance> | null = null

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    document.body.innerHTML = ''
  })

  it('closes when clicking outside even if an ancestor stops bubbling', async () => {
    const Harness = defineComponent({
      components: { AppSelect: Select },
      setup() {
        const selected = ref('401')
        const options = [
          { value: '', label: '全部' },
          { value: '400', label: '400' },
          { value: '401', label: '401' }
        ]

        return { selected, options }
      },
      template: `
        <div class="dialog-panel" @click.stop>
          <AppSelect v-model="selected" :options="options" />
          <button type="button" class="dialog-blank-area">Blank area</button>
        </div>
      `
    })

    wrapper = mount(Harness, {
      attachTo: document.body,
      global: {
        stubs: { Icon: IconStub }
      }
    })

    await wrapper.get('.select-trigger').trigger('click')
    await nextTick()

    expect(wrapper.get('.select-trigger').classes()).toContain('select-trigger-open')
    expect(document.body.querySelector('.select-dropdown-portal')).not.toBeNull()

    await wrapper.get('.dialog-blank-area').trigger('click')
    await nextTick()

    expect(wrapper.get('.select-trigger').classes()).not.toContain('select-trigger-open')
  })
})
