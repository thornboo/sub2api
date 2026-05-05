import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

describe('HelpTooltip', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('uses viewport coordinates for fixed positioning after page scroll', async () => {
    Object.defineProperty(window, 'scrollY', { value: 480, configurable: true })
    Object.defineProperty(window, 'scrollX', { value: 32, configurable: true })
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true })

    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'position details',
      },
    })

    const trigger = wrapper.get('.group')
    vi.spyOn(trigger.element, 'getBoundingClientRect').mockReturnValue({
      top: 200,
      right: 130,
      bottom: 220,
      left: 110,
      width: 20,
      height: 20,
      x: 110,
      y: 200,
      toJSON: () => ({}),
    } as DOMRect)

    const tooltip = getTooltipElement()
    Object.defineProperty(tooltip, 'offsetWidth', { value: 120, configurable: true })
    Object.defineProperty(tooltip, 'offsetHeight', { value: 40, configurable: true })

    await trigger.trigger('mouseenter')
    await nextTick()

    expect(tooltip.style.top).toBe('192px')
    expect(tooltip.style.left).toBe('120px')

    wrapper.unmount()
  })
})
