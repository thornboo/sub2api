import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick, ref, type ComponentPublicInstance } from 'vue'

import Select from '../Select.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const IconStub = { template: '<span />' }
const originalInnerWidth = window.innerWidth

let wrapper: VueWrapper<ComponentPublicInstance> | null = null
let unmountWrapper: (() => void) | undefined

const setViewportWidth = (width: number) => {
  Object.defineProperty(window, 'innerWidth', {
    configurable: true,
    value: width
  })
}

const mockTriggerRect = (left: number, width: number) => {
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
    x: left,
    y: 20,
    top: 20,
    right: left + width,
    bottom: 60,
    left,
    width,
    height: 40,
    toJSON: () => ({})
  })
}

const openSelect = async () => {
  const selectWrapper = mount(Select, {
    props: {
      modelValue: null,
      options: [
        {
          value: 'example',
          label: 'very-long-unbroken-option-value-that-must-not-overflow'
        }
      ]
    }
  })
  unmountWrapper = () => selectWrapper.unmount()

  await selectWrapper.get('button').trigger('click')
  await nextTick()

  return document.body.querySelector<HTMLElement>('.select-dropdown-portal')
}

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  unmountWrapper?.()
  unmountWrapper = undefined
  document.body.innerHTML = ''
  setViewportWidth(originalInnerWidth)
  vi.restoreAllMocks()
})

describe('Select outside click behavior', () => {
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

describe('Select dropdown viewport constraints', () => {
  it('preserves the existing 200px minimum width when space is available', async () => {
    setViewportWidth(1024)
    mockTriggerRect(20, 80)

    const dropdown = await openSelect()

    expect(dropdown).not.toBeNull()
    expect(dropdown?.style.left).toBe('20px')
    expect(dropdown?.style.minWidth).toBe('200px')
    expect(dropdown?.style.maxWidth).toBe('996px')
  })

  it('shrinks the minimum width to fit near the right viewport edge', async () => {
    setViewportWidth(320)
    mockTriggerRect(220, 80)

    const dropdown = await openSelect()

    expect(dropdown).not.toBeNull()
    expect(dropdown?.style.left).toBe('220px')
    expect(dropdown?.style.minWidth).toBe('92px')
    expect(dropdown?.style.maxWidth).toBe('92px')
  })

  it('clamps a trigger left of the viewport to the safe padding', async () => {
    setViewportWidth(320)
    mockTriggerRect(-20, 80)

    const dropdown = await openSelect()

    expect(dropdown).not.toBeNull()
    expect(dropdown?.style.left).toBe('8px')
    expect(dropdown?.style.minWidth).toBe('200px')
    expect(dropdown?.style.maxWidth).toBe('304px')
  })

  it('clamps an offscreen-right trigger position to the viewport boundary', async () => {
    setViewportWidth(320)
    mockTriggerRect(400, 80)

    const dropdown = await openSelect()

    expect(dropdown).not.toBeNull()
    expect(dropdown?.style.left).toBe('312px')
    expect(dropdown?.style.minWidth).toBe('0px')
    expect(dropdown?.style.maxWidth).toBe('0px')
  })
})
