import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick, ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import AnnouncementPopup from '../AnnouncementPopup.vue'
import AnnouncementBell from '../AnnouncementBell.vue'
import BaseDialog from '../BaseDialog.vue'
import { useAnnouncementStore } from '@/stores/announcements'

const announcementMarkdownStyles = readFileSync(
  resolve(process.cwd(), 'src/styles/announcement-markdown.css'),
  'utf8',
)

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const announcement = {
  id: 1,
  title: 'Preview announcement',
  content: '## Preview heading\n\n<div>HTML content</div><script>window.__xss = true</script>',
  status: 'draft' as const,
  notify_mode: 'popup' as const,
  targeting: { any_of: [] },
  created_at: '2026-07-24T07:30:00Z',
  updated_at: '2026-07-24T07:30:00Z',
}

describe('AnnouncementPopup', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    document.body.innerHTML = ''
    document.body.className = ''
    document.body.style.overflow = ''
  })

  it('renders mixed Markdown and HTML inside the shared styled container', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = {
      id: 1,
      title: 'Mixed content announcement',
      content: [
        '## Markdown heading',
        '',
        '<div><h3>HTML heading</h3><ul><li>HTML list item</li></ul></div>',
        '',
        '<table><thead><tr><th>Status</th></tr></thead><tbody><tr><td>OK</td></tr></tbody></table>',
        '<script>window.__announcementXss = true</script>',
      ].join('\n'),
      notify_mode: 'popup',
      created_at: '2026-07-24T07:30:00Z',
      updated_at: '2026-07-24T07:30:00Z',
    }

    const wrapper = mount(AnnouncementPopup)
    await wrapper.vm.$nextTick()

    const content = document.body.querySelector('.markdown-body')
    expect(content?.querySelector('h2')?.textContent).toBe('Markdown heading')
    expect(content?.querySelector('h3')?.textContent).toBe('HTML heading')
    expect(content?.querySelector('li')?.textContent).toBe('HTML list item')
    expect(content?.querySelector('table td')?.textContent).toBe('OK')
    expect(content?.querySelector('script')).toBeNull()

    wrapper.unmount()
  })

  it.each(['h2', 'h3', 'ul', 'li', 'blockquote', 'table', 'th', 'td', 'code'])(
    'loads a shared style rule for mixed-content <%s> elements',
    (element) => {
      expect(announcementMarkdownStyles).toContain(`.markdown-body ${element}`)
    },
  )

  it('previews an admin announcement without marking it as read', async () => {
    const store = useAnnouncementStore()
    const dismissPopup = vi.spyOn(store, 'dismissPopup')
    const wrapper = mount(AnnouncementPopup, {
      props: {
        announcement,
        preview: true,
      },
    })

    expect(document.body.textContent).toContain('Preview announcement')
    expect(document.body.querySelector('.markdown-body h2')?.textContent).toBe('Preview heading')
    expect(document.body.querySelector('.markdown-body script')).toBeNull()
    expect(document.body.textContent).toContain('common.close')

    const dismissButton = document.body.querySelector<HTMLButtonElement>(
      '[data-testid="announcement-popup-dismiss"]',
    )
    dismissButton?.click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(dismissPopup).not.toHaveBeenCalled()

    await wrapper.setProps({ announcement: null })
    expect(document.body.style.overflow).toBe('')
    wrapper.unmount()
  })

  it('keeps the existing user popup dismissal behavior', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = announcement
    const dismissPopup = vi.spyOn(store, 'dismissPopup').mockResolvedValue()
    const wrapper = mount(AnnouncementPopup)

    const dismissButton = document.body.querySelector<HTMLButtonElement>(
      '[data-testid="announcement-popup-dismiss"]',
    )
    dismissButton?.click()
    await wrapper.vm.$nextTick()

    expect(dismissPopup).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('close')).toBeUndefined()
    wrapper.unmount()
  })

  it('releases its body scroll lock when a route-level v-if unmounts the user popup', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = announcement

    const Host = defineComponent({
      components: { AnnouncementPopup },
      setup() {
        const showPopup = ref(true)
        return { showPopup }
      },
      template: '<AnnouncementPopup v-if="showPopup" />',
    })

    const wrapper = mount(Host)
    await nextTick()

    expect(document.body.style.overflow).toBe('hidden')

    ;(wrapper.vm as unknown as { showPopup: boolean }).showPopup = false
    await nextTick()

    expect(document.body.style.overflow).toBe('')
    wrapper.unmount()
  })

  it('keeps body locked until every announcement overlay owner closes', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = announcement
    store.announcements = [{ ...announcement, read_at: null }]

    const Host = defineComponent({
      components: { AnnouncementPopup, AnnouncementBell },
      template: `
        <AnnouncementPopup />
        <AnnouncementBell />
      `,
    })

    const wrapper = mount(Host, {
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })
    await nextTick()

    await wrapper.findComponent(AnnouncementBell).find('button').trigger('click')
    await nextTick()

    expect(document.body.style.overflow).toBe('hidden')

    store.currentPopup = null
    await nextTick()

    expect(document.body.style.overflow).toBe('hidden')

    document.body.querySelector<HTMLButtonElement>('button[aria-label="common.close"]')?.click()
    await nextTick()
    await flushPromises()

    expect(document.body.style.overflow).toBe('')
    wrapper.unmount()
  })

  it('does not remove the BaseDialog modal-open body lock when an announcement owner closes', async () => {
    const store = useAnnouncementStore()
    store.currentPopup = announcement

    const Host = defineComponent({
      components: { AnnouncementPopup, BaseDialog },
      setup() {
        const showPopup = ref(true)
        const showDialog = ref(true)
        return { showPopup, showDialog }
      },
      template: `
        <AnnouncementPopup v-if="showPopup" />
        <BaseDialog :show="showDialog" title="Dialog">
          <button type="button">Dialog action</button>
        </BaseDialog>
      `,
    })

    const wrapper = mount(Host, {
      attachTo: document.body,
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })
    await nextTick()
    await flushPromises()

    expect(document.body.style.overflow).toBe('hidden')
    expect(document.body.classList.contains('modal-open')).toBe(true)

    ;(wrapper.vm as unknown as { showPopup: boolean }).showPopup = false
    await nextTick()

    expect(document.body.style.overflow).toBe('')
    expect(document.body.classList.contains('modal-open')).toBe(true)

    wrapper.unmount()
  })
})
