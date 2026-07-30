import { mount, RouterLinkStub } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import PlazaNavBar from '../PlazaNavBar.vue'

const authStore = vi.hoisted(() => ({
  isAuthenticated: false,
  isAdmin: false,
}))

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: {
    site_name: 'ZedRouter',
    site_logo: '',
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function mountNavBar() {
  return mount(PlazaNavBar, {
    global: {
      stubs: {
        RouterLink: RouterLinkStub,
      },
    },
  })
}

describe('PlazaNavBar', () => {
  beforeEach(() => {
    authStore.isAuthenticated = false
    authStore.isAdmin = false
  })

  it('returns anonymous visitors to the home page', () => {
    const wrapper = mountNavBar()
    const link = wrapper.getComponent(RouterLinkStub)

    expect(link.props('to')).toBe('/')
    expect(wrapper.text()).toContain('modelPlaza.nav.backToHome')
    expect(wrapper.text()).not.toContain('modelPlaza.nav.backToConsole')
  })

  it('returns authenticated users to their console', () => {
    authStore.isAuthenticated = true
    const wrapper = mountNavBar()
    const link = wrapper.getComponent(RouterLinkStub)

    expect(link.props('to')).toBe('/dashboard')
    expect(wrapper.text()).toContain('modelPlaza.nav.backToConsole')
  })

  it('returns administrators to the admin console', () => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true
    const wrapper = mountNavBar()

    expect(wrapper.getComponent(RouterLinkStub).props('to')).toBe('/admin/dashboard')
  })
})
