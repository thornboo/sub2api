import { shallowMount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { archive, batchReplaceGroups, getOwnerUsageSummary, getAvailableGroups, listMembers, permanentlyDelete, restore, setStatus, showError, showSuccess } = vi.hoisted(() => ({
  archive: vi.fn(),
  batchReplaceGroups: vi.fn(),
  getOwnerUsageSummary: vi.fn(),
  getAvailableGroups: vi.fn(),
  listMembers: vi.fn(),
  permanentlyDelete: vi.fn(),
  restore: vi.fn(),
  setStatus: vi.fn(),
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
    archive,
    list: listMembers,
    getOwnerUsageSummary,
    batchReplaceGroups,
    permanentlyDelete,
    restore,
    setStatus,
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
    restore.mockResolvedValue({ ...member, status: 'disabled', version: 4 })
    setStatus.mockResolvedValue({ ...member, status: 'disabled', version: 4 })
  })

  it('does not change a member status until the explicit confirmation is accepted', async () => {
    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      memberStatusChangeRequest: { status: 'active' | 'disabled', members: typeof member[], bulk: boolean } | null
      memberStatusUpdating: boolean
      toggleStatus: (target: typeof member) => void
      cancelMemberStatusChange: () => void
      confirmMemberStatusChange: () => Promise<void>
    }

    vm.toggleStatus(member)
    await nextTick()

    expect(vm.memberStatusChangeRequest).toEqual({ status: 'disabled', members: [member], bulk: false })
    expect(setStatus).not.toHaveBeenCalled()

    vm.cancelMemberStatusChange()
    expect(setStatus).not.toHaveBeenCalled()

    vm.toggleStatus(member)
    await vm.confirmMemberStatusChange()

    expect(setStatus).toHaveBeenCalledTimes(1)
    expect(setStatus).toHaveBeenCalledWith(member, 'disabled')
    expect(showSuccess).toHaveBeenCalledWith('enterpriseMembers.copy.memberDisabledSuccess')
    expect(vm.memberStatusChangeRequest).toBeNull()
    expect(vm.memberStatusUpdating).toBe(false)

    const disabledMember = { ...member, status: 'disabled' as const, version: 4 }
    setStatus.mockResolvedValueOnce({ ...disabledMember, status: 'active', version: 5 })
    vm.toggleStatus(disabledMember)
    await nextTick()

    expect(vm.memberStatusChangeRequest).toEqual({ status: 'active', members: [disabledMember], bulk: false })
    expect(setStatus).toHaveBeenCalledTimes(1)

    await vm.confirmMemberStatusChange()

    expect(setStatus).toHaveBeenNthCalledWith(2, disabledMember, 'active')
    expect(showSuccess).toHaveBeenCalledWith('enterpriseMembers.copy.memberEnabledSuccess')
  })

  it('uses the project confirmation dialog and permanently removes historical members through the server strategy', async () => {
    const archivedMember = { ...member, status: 'disabled' as const, deleted_at: '2026-07-03T00:00:00Z', delete_strategy: 'tombstone' as const }
    listMembers.mockResolvedValue([archivedMember])
    permanentlyDelete.mockResolvedValue({ archived: false, permanently_deleted: true, deletion_mode: 'tombstone' })
    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      memberRemovalTarget: typeof archivedMember | null
      removeMember: (target: typeof archivedMember) => void
      confirmRemoveMember: () => Promise<void>
    }

    vm.removeMember(archivedMember)
    await nextTick()
    expect(vm.memberRemovalTarget).toEqual(archivedMember)
    expect(permanentlyDelete).not.toHaveBeenCalled()

    await vm.confirmRemoveMember()

    expect(permanentlyDelete).toHaveBeenCalledWith(archivedMember)
    expect(showError).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('enterpriseMembers.copy.memberPermanentlyDeleted')
    expect(vm.memberRemovalTarget).toBeNull()
  })

  it('restores an archived member as disabled before the owner explicitly enables it', async () => {
    const archivedMember = { ...member, status: 'disabled' as const, deleted_at: '2026-07-03T00:00:00Z', delete_strategy: 'hard_delete' as const }
    listMembers.mockResolvedValue([archivedMember])
    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      restoreMember: (target: typeof archivedMember) => Promise<void>
      restoringMemberId: number | null
    }

    await vm.restoreMember(archivedMember)

    expect(restore).toHaveBeenCalledWith(archivedMember)
    expect(showSuccess).toHaveBeenCalledWith('enterpriseMembers.copy.memberRestoredDisabled')
    expect(vm.restoringMemberId).toBeNull()
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
