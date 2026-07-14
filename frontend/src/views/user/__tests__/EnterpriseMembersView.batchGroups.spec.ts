import { shallowMount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { batchReplaceGroups, getOwnerUsageSummary, getAvailableGroups, listMembers, showError, showSuccess } = vi.hoisted(() => ({
  batchReplaceGroups: vi.fn(),
  getOwnerUsageSummary: vi.fn(),
  getAvailableGroups: vi.fn(),
  listMembers: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({
    t: (key: string) => key,
    locale: { value: 'zh-CN' },
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: vi.fn() }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
  useAuthStore: () => ({
    user: { id: 7, role: 'user', account_type: 'enterprise', enterprise_disabled_at: null },
  }),
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: { getAvailable: getAvailableGroups },
}))

vi.mock('@/api/keys', () => ({
  keysAPI: {},
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

vi.mock('@/api/enterpriseMembers', () => ({
  enterpriseMembersAPI: {
    list: listMembers,
    getOwnerUsageSummary,
    batchReplaceGroups,
  },
}))

import EnterpriseMembersView from '../EnterpriseMembersView.vue'

const member = {
  id: 41,
  enterprise_user_id: 7,
  member_code: 'member-41',
  name: '成员 41',
  status: 'active' as const,
  monthly_limit_usd: 100,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  version: 3,
  group_ids: [9],
  key_count: 1,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
  deleted_at: null,
}

function mountView() {
  return shallowMount(EnterpriseMembersView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        BaseDialog: { props: ['show'], template: '<section v-if="show"><slot /><slot name="footer" /></section>' },
        ConfirmDialog: true,
        EmptyState: true,
        Icon: true,
        Select: true,
      },
    },
  })
}

describe('EnterpriseMembersView destructive batch group confirmation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listMembers.mockResolvedValue([member])
    getAvailableGroups.mockResolvedValue([])
    getOwnerUsageSummary.mockResolvedValue(null)
    batchReplaceGroups.mockResolvedValue([{ id: member.id, version: 4, group_ids: [], status: 'disabled', updated_at: '2026-07-02T00:00:00Z' }])
  })

  it('does not clear access until the explicit danger confirmation is accepted', async () => {
    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      selectedIds: Set<number>
      batchGroupIds: number[]
      batchGroupMode: 'replace' | 'append'
      batchGroupsOpen: boolean
      batchClearConfirmOpen: boolean
      openBatchGroups: () => void
      requestSaveBatchGroups: () => void
      cancelBatchGroupClear: () => void
      confirmBatchGroupClear: () => Promise<void>
    }

    vm.selectedIds.add(member.id)
    vm.openBatchGroups()
    vm.batchGroupMode = 'replace'
    vm.batchGroupIds = []
    vm.requestSaveBatchGroups()
    await nextTick()

    expect(batchReplaceGroups).not.toHaveBeenCalled()
    expect(vm.batchGroupsOpen).toBe(false)
    expect(vm.batchClearConfirmOpen).toBe(true)

    vm.cancelBatchGroupClear()
    await nextTick()
    expect(batchReplaceGroups).not.toHaveBeenCalled()
    expect(vm.batchGroupsOpen).toBe(true)
    expect(vm.batchClearConfirmOpen).toBe(false)

    vm.requestSaveBatchGroups()
    await vm.confirmBatchGroupClear()

    expect(batchReplaceGroups).toHaveBeenCalledTimes(1)
    expect(batchReplaceGroups).toHaveBeenCalledWith([expect.objectContaining({ id: member.id, version: member.version })], [], 'replace')
  })

  it('renders the frozen period only when a completed result actually carries period metadata', async () => {
    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      importOpen: boolean
      importResult: Record<string, unknown> | null
    }
    const baseResult = {
      job_id: 9,
      status: 'completed',
      created_members: 1,
      created_keys: 0,
      member_ids: [member.id],
      pending_members: 1,
      migration_billed_usd: 30,
      migration_total_tokens: 100,
      rows: [2],
      keys: [],
      completed_at: '2026-07-14T00:00:00Z',
    }

    vm.importOpen = true
    vm.importResult = baseResult
    await nextTick()
    expect(wrapper.text()).not.toContain('enterpriseMembers.dynamic.importPeriod')

    vm.importResult = { ...baseResult, period_start: '2026-07-01T00:00:00+08:00', timezone: 'Asia/Shanghai' }
    await nextTick()
    expect(wrapper.text()).toContain('enterpriseMembers.dynamic.importPeriod')
  })
})
