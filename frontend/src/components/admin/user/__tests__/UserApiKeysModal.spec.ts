import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser, ApiKey } from '@/types'
import UserApiKeysModal from '../UserApiKeysModal.vue'

const { getUserApiKeys, getAllGroups, getBatchApiKeysUsage, updateApiKeyGroup } = vi.hoisted(() => ({
  getUserApiKeys: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchApiKeysUsage: vi.fn(),
  updateApiKeyGroup: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      getUserApiKeys,
    },
    groups: {
      getAll: getAllGroups,
    },
    dashboard: {
      getBatchApiKeysUsage,
    },
    apiKeys: {
      updateApiKeyGroup,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
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

const user: AdminUser = {
  id: 7,
  username: 'target',
  email: 'target@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
  notes: '',
}

const apiKey: ApiKey = {
  id: 11,
  user_id: 7,
  key: 'sk-test-abcdefghijklmnopqrstuvwxyz',
  name: 'prod-key',
  tags: [],
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-01T00:00:00Z',
  updated_at: '2026-06-01T00:00:00Z',
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
}

describe('UserApiKeysModal', () => {
  beforeEach(() => {
    getUserApiKeys.mockReset()
    getAllGroups.mockReset()
    getBatchApiKeysUsage.mockReset()
    updateApiKeyGroup.mockReset()

    getUserApiKeys.mockResolvedValue({ items: [apiKey] })
    getAllGroups.mockResolvedValue([])
    getBatchApiKeysUsage.mockResolvedValue({
      stats: {
        11: {
          api_key_id: 11,
          today_actual_cost: 1.23,
          total_actual_cost: 4.56,
        },
      },
    })
  })

  it('opens an admin key usage modal from the API key list instead of navigating away', async () => {
    const wrapper = mount(UserApiKeysModal, {
      props: {
        show: true,
        user,
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<section v-if="show"><slot /></section>',
          },
          Icon: {
            props: ['name'],
            template: '<span class="icon">{{ name }}</span>',
          },
          GroupBadge: true,
          GroupOptionItem: true,
          AdminApiKeyUsageModal: {
            props: ['show', 'user', 'apiKey'],
            template: '<div v-if="show" data-test="admin-key-usage-modal">{{ apiKey?.name || user?.email }}</div>',
          },
        },
      },
    })

    await flushPromises()

    expect(getUserApiKeys).toHaveBeenCalledWith(7)
    expect(getBatchApiKeysUsage).toHaveBeenCalledWith([11])
    expect(wrapper.text()).toContain('$1.2300')
    expect(wrapper.text()).toContain('$4.5600')

    await wrapper.find('button[title="admin.users.viewApiKeyUsageDetails"]').trigger('click')

    expect(wrapper.find('[data-test="admin-key-usage-modal"]').text()).toBe('prod-key')
  })

  it('opens user-wide usage details from the user API keys dialog header', async () => {
    const wrapper = mount(UserApiKeysModal, {
      props: {
        show: true,
        user,
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<section v-if="show"><slot /></section>',
          },
          Icon: {
            props: ['name'],
            template: '<span class="icon">{{ name }}</span>',
          },
          GroupBadge: true,
          GroupOptionItem: true,
          AdminApiKeyUsageModal: {
            props: ['show', 'user', 'apiKey'],
            template: '<div v-if="show" data-test="admin-key-usage-modal" :data-api-key-id="apiKey?.id || \'\'">{{ user?.email }}</div>',
          },
        },
      },
    })

    await flushPromises()

    await wrapper.find('button[title="admin.users.viewUserUsageDetails"]').trigger('click')

    const modal = wrapper.find('[data-test="admin-key-usage-modal"]')
    expect(modal.text()).toBe('target@example.com')
    expect(modal.attributes('data-api-key-id')).toBe('')
  })
})
