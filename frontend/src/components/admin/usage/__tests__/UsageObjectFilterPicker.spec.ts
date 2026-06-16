import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageObjectFilterPicker from '../UsageObjectFilterPicker.vue'

const { listUsers, searchUsers, searchApiKeys } = vi.hoisted(() => ({
  listUsers: vi.fn(),
  searchUsers: vi.fn(),
  searchApiKeys: vi.fn(),
}))

const messages: Record<string, string> = {
  'admin.usage.profile.objectFilter': 'Object',
  'admin.usage.profile.objectFilterPlaceholder': 'All usage',
  'admin.usage.profile.clearObjectFilter': 'Clear object',
  'admin.usage.profile.usersColumn': 'Users',
  'admin.usage.profile.keysColumn': 'API Keys',
  'admin.usage.profile.allUserKeys': 'All Keys',
  'admin.usage.profile.allUserKeysHelp': 'All keys for this user',
  'admin.usage.profile.noUsersFound': 'No users',
  'admin.usage.profile.noKeysFound': 'No keys',
  'admin.usage.profile.includeDeletedApiKeys': 'Include deleted keys',
  'admin.usage.profile.selectUserFirst': 'Select a user',
  'admin.usage.searchUserPlaceholder': 'Search users',
  'admin.usage.searchApiKeyPlaceholder': 'Search keys',
  'admin.usage.userDeletedBadge': 'deleted',
  'admin.usage.apiKeyDeletedBadge': 'deleted',
  'common.loading': 'Loading',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: (...args: unknown[]) => listUsers(...args),
    },
    usage: {
      searchUsers: (...args: unknown[]) => searchUsers(...args),
      searchApiKeys: (...args: unknown[]) => searchApiKeys(...args),
    },
  },
}))

const IconStub = {
  template: '<span />',
}

function mountPicker(props = {}) {
  return mount(UsageObjectFilterPicker, {
    props: {
      user: null,
      apiKey: null,
      ...props,
    },
    global: {
      stubs: {
        Icon: IconStub,
        Teleport: true,
      },
    },
  })
}

async function openPicker(wrapper: ReturnType<typeof mountPicker>) {
  await wrapper.find('[role="button"]').trigger('click')
  await flushPromises()
}

describe('UsageObjectFilterPicker', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    listUsers.mockReset()
    searchUsers.mockReset()
    searchApiKeys.mockReset()
    searchApiKeys.mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('loads the default user list page and appends the next page on scroll', async () => {
    listUsers
      .mockResolvedValueOnce({
        items: [{ id: 1, email: 'a@example.com', deleted_at: null }],
        pages: 2,
      })
      .mockResolvedValueOnce({
        items: [{ id: 2, email: 'b@example.com', deleted_at: null }],
        pages: 2,
      })

    const wrapper = mountPicker()
    await openPicker(wrapper)

    expect(listUsers).toHaveBeenCalledWith(1, 30, {
      include_subscriptions: false,
      sort_by: 'email',
      sort_order: 'asc',
    })

    const userList = wrapper.find('[data-test="usage-object-user-list"]')
    Object.defineProperty(userList.element, 'scrollTop', { value: 132, configurable: true })
    Object.defineProperty(userList.element, 'clientHeight', { value: 80, configurable: true })
    Object.defineProperty(userList.element, 'scrollHeight', { value: 200, configurable: true })
    await userList.trigger('scroll')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(2, 30, {
      include_subscriptions: false,
      sort_by: 'email',
      sort_order: 'asc',
    })
    expect(wrapper.text()).toContain('a@example.com')
    expect(wrapper.text()).toContain('b@example.com')
  })

  it('keeps keyword search on the usage search endpoint so deleted users remain discoverable', async () => {
    listUsers.mockResolvedValue({
      items: [{ id: 1, email: 'a@example.com', deleted_at: null }],
      pages: 1,
    })
    searchUsers.mockResolvedValue([
      { id: 9, email: 'deleted@example.com', deleted: true },
    ])

    const wrapper = mountPicker()
    await openPicker(wrapper)
    listUsers.mockClear()

    await wrapper.find('input').setValue('deleted')
    vi.advanceTimersByTime(300)
    await flushPromises()

    expect(searchUsers).toHaveBeenCalledWith('deleted')
    expect(listUsers).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('deleted@example.com')
    expect(wrapper.text()).toContain('deleted')
  })

  it('does not repeat the selected user label in the API key column header', async () => {
    listUsers.mockResolvedValue({
      items: [{ id: 4, email: 'htzh@htwisdom.cn', deleted_at: null }],
      pages: 1,
    })

    const wrapper = mountPicker({
      user: { id: 4, label: 'htzh@htwisdom.cn' },
    })
    await openPicker(wrapper)

    expect(wrapper.find('[data-test="usage-object-key-header"]').text()).toBe('API Keys')
  })

  it('can include deleted API keys only when the admin opts in', async () => {
    listUsers.mockResolvedValue({
      items: [{ id: 4, email: 'htzh@htwisdom.cn', deleted_at: null }],
      pages: 1,
    })
    searchApiKeys.mockResolvedValueOnce([]).mockResolvedValueOnce([
      { id: 8, name: 'deleted-key', user_id: 4, deleted: true, deleted_at: '2026-06-10T12:00:00Z' },
    ])

    const wrapper = mountPicker({
      user: { id: 4, label: 'htzh@htwisdom.cn' },
    })
    await openPicker(wrapper)

    expect(searchApiKeys).toHaveBeenCalledWith(4, '', { includeDeleted: false })

    const includeDeleted = wrapper.find('input[type="checkbox"]')
    await includeDeleted.setValue(true)
    await flushPromises()

    expect(searchApiKeys).toHaveBeenLastCalledWith(4, '', { includeDeleted: true })
    expect(wrapper.text()).toContain('deleted-key')
    expect(wrapper.text()).toContain('deleted')
  })

  it('attaches global positioning listeners only while the dropdown is open', async () => {
    listUsers.mockResolvedValue({
      items: [],
      pages: 0,
    })
    const addWindowListener = vi.spyOn(window, 'addEventListener')
    const removeWindowListener = vi.spyOn(window, 'removeEventListener')
    const addDocumentListener = vi.spyOn(document, 'addEventListener')
    const removeDocumentListener = vi.spyOn(document, 'removeEventListener')

    const wrapper = mountPicker()

    expect(addWindowListener).not.toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(addWindowListener).not.toHaveBeenCalledWith('resize', expect.any(Function))
    expect(addDocumentListener).not.toHaveBeenCalledWith('click', expect.any(Function))

    await openPicker(wrapper)

    expect(addWindowListener).toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(addWindowListener).toHaveBeenCalledWith('resize', expect.any(Function))
    expect(addDocumentListener).toHaveBeenCalledWith('click', expect.any(Function))

    await wrapper.find('[role="button"]').trigger('click')
    await flushPromises()

    expect(removeWindowListener).toHaveBeenCalledWith('scroll', expect.any(Function), true)
    expect(removeWindowListener).toHaveBeenCalledWith('resize', expect.any(Function))
    expect(removeDocumentListener).toHaveBeenCalledWith('click', expect.any(Function))
  })
})
