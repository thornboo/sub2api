import { afterEach, describe, expect, it } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BaseDialog from '../BaseDialog.vue'

const IconStub = { template: '<span />' }

async function settleDialogWatchers() {
  await nextTick()
  await flushPromises()
  await new Promise((resolve) => window.setTimeout(resolve, 0))
  await nextTick()
}

describe('BaseDialog stacked modal behavior', () => {
  let wrapper: VueWrapper | null = null

  afterEach(async () => {
    wrapper?.unmount()
    await settleDialogWatchers()
    expect(document.body.classList.contains('modal-open')).toBe(false)
    wrapper = null
    document.body.innerHTML = ''
    document.body.className = ''
  })

  it('closes only the top dialog on Escape and keeps body locked while a parent dialog remains open', async () => {
    const Harness = defineComponent({
      components: { BaseDialog },
      setup() {
        const parentOpen = ref(true)
        const childOpen = ref(true)
        return { parentOpen, childOpen }
      },
      template: `
        <BaseDialog :show="parentOpen" title="Parent" @close="parentOpen = false">
          <button type="button">Parent action</button>
        </BaseDialog>
        <BaseDialog :show="childOpen" title="Child" @close="childOpen = false">
          <button type="button">Child action</button>
        </BaseDialog>
      `
    })

    wrapper = mount(Harness, {
      attachTo: document.body,
      global: {
        stubs: { Icon: IconStub }
      }
    })
    await settleDialogWatchers()

    const overlays = document.body.querySelectorAll<HTMLElement>('.modal-overlay')
    expect(overlays).toHaveLength(2)
    expect(overlays[0].style.zIndex).toBe('50')
    expect(overlays[1].style.zIndex).toBe('60')
    expect(document.body.classList.contains('modal-open')).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await settleDialogWatchers()

    expect((wrapper.vm as any).childOpen).toBe(false)
    expect((wrapper.vm as any).parentOpen).toBe(true)
    expect(document.body.classList.contains('modal-open')).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await settleDialogWatchers()

    expect((wrapper.vm as any).parentOpen).toBe(false)
    expect(document.body.classList.contains('modal-open')).toBe(false)
  })

  it('preserves an explicit z-index when the caller provides one', async () => {
    wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: {
        show: true,
        title: 'Pinned layer',
        zIndex: 120
      },
      global: {
        stubs: { Icon: IconStub }
      }
    })
    await settleDialogWatchers()

    const overlay = document.body.querySelector<HTMLElement>('.modal-overlay')
    expect(overlay?.style.zIndex).toBe('120')
  })

  it('treats the highest effective z-index dialog as the top dialog', async () => {
    const Harness = defineComponent({
      components: { BaseDialog },
      setup() {
        const pinnedOpen = ref(true)
        const laterOpen = ref(true)
        return { pinnedOpen, laterOpen }
      },
      template: `
        <BaseDialog :show="pinnedOpen" title="Pinned" :z-index="120" @close="pinnedOpen = false">
          <button type="button">Close pinned</button>
        </BaseDialog>
        <BaseDialog :show="laterOpen" title="Later" @close="laterOpen = false">
          <button type="button">Close later</button>
        </BaseDialog>
      `
    })

    wrapper = mount(Harness, {
      attachTo: document.body,
      global: {
        stubs: { Icon: IconStub }
      }
    })
    await settleDialogWatchers()

    const overlays = document.body.querySelectorAll<HTMLElement>('.modal-overlay')
    expect(overlays).toHaveLength(2)
    expect(overlays[0].style.zIndex).toBe('120')
    expect(overlays[1].style.zIndex).toBe('60')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await settleDialogWatchers()

    expect((wrapper.vm as any).pinnedOpen).toBe(false)
    expect((wrapper.vm as any).laterOpen).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await settleDialogWatchers()

    expect((wrapper.vm as any).laterOpen).toBe(false)
  })

  it('closes only the top dialog on outside overlay click', async () => {
    const Harness = defineComponent({
      components: { BaseDialog },
      setup() {
        const parentOpen = ref(true)
        const childOpen = ref(true)
        return { parentOpen, childOpen }
      },
      template: `
        <BaseDialog :show="parentOpen" title="Parent" close-on-click-outside @close="parentOpen = false">
          <button type="button">Parent action</button>
        </BaseDialog>
        <BaseDialog :show="childOpen" title="Child" close-on-click-outside @close="childOpen = false">
          <button type="button">Child action</button>
        </BaseDialog>
      `
    })

    wrapper = mount(Harness, {
      attachTo: document.body,
      global: {
        stubs: { Icon: IconStub }
      }
    })
    await settleDialogWatchers()

    const overlays = document.body.querySelectorAll<HTMLElement>('.modal-overlay')
    expect(overlays).toHaveLength(2)

    overlays[0].dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await settleDialogWatchers()

    expect((wrapper.vm as any).parentOpen).toBe(true)
    expect((wrapper.vm as any).childOpen).toBe(true)

    overlays[1].dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await settleDialogWatchers()

    expect((wrapper.vm as any).childOpen).toBe(false)
    expect((wrapper.vm as any).parentOpen).toBe(true)
  })
})
