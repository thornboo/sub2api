import { flushPromises, shallowMount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { archive, batchReplaceGroups, batchUpdate, clipboardCopy, createBudgetAdjustment, createEnterpriseMemberOperationIdempotencyKey, getBudget, getOwnerUsageSummary, getUsageAnalytics, getAvailableGroups, listAuditEvents, listBudgetEntries, listKeys, listMembers, permanentlyDelete, restore, revealKey, setStatus, showError, showSuccess, usageQuery } = vi.hoisted(() => ({
  archive: vi.fn(),
  batchReplaceGroups: vi.fn(),
  batchUpdate: vi.fn(),
  clipboardCopy: vi.fn(),
  createBudgetAdjustment: vi.fn(),
  createEnterpriseMemberOperationIdempotencyKey: vi.fn(() => 'stable-operation-key'),
  getBudget: vi.fn(),
  getOwnerUsageSummary: vi.fn(),
  getUsageAnalytics: vi.fn(),
  getAvailableGroups: vi.fn(),
  listAuditEvents: vi.fn(),
  listBudgetEntries: vi.fn(),
  listKeys: vi.fn(),
  listMembers: vi.fn(),
  permanentlyDelete: vi.fn(),
  restore: vi.fn(),
  revealKey: vi.fn(),
  setStatus: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  usageQuery: vi.fn(),
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

vi.mock('@/api/usage', () => ({
  usageAPI: { query: usageQuery },
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: clipboardCopy }),
}))

vi.mock('@/api/enterpriseMembers', () => ({
  ENTERPRISE_MEMBER_MAX_MONETARY_VALUE: 999_999_999_999.99,
  enterpriseMembersAPI: {
    archive,
    list: listMembers,
    getOwnerUsageSummary,
    batchReplaceGroups,
    batchUpdate,
    createBudgetAdjustment,
    createEnterpriseMemberOperationIdempotencyKey,
    getBudget,
    getUsageAnalytics,
    permanentlyDelete,
    listAuditEvents,
    listBudgetEntries,
    listKeys,
    revealKey,
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

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => { resolve = res })
  return { promise, resolve }
}

const budgetSummaryFixture = (memberID: number, marker: string) => ({
  marker,
  member_id: memberID,
  period_start: '2026-07-01',
  period_end: '2026-07-31',
  timezone: 'Asia/Shanghai',
  limit_usd: 100,
  used_usd: 1,
  reserved_usd: 0,
  remaining_usd: 99,
  request_count: 1,
  input_tokens: 10,
  output_tokens: 5,
  migration_billed_usd: 0,
  migration_total_tokens: '0.00',
  migration_input_tokens: '0.00',
  migration_output_tokens: '0.00',
  migration_cache_tokens: '0.00',
  migration_cache_write_tokens: '0.00',
  migration_cache_read_tokens: '0.00',
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
})

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
    getUsageAnalytics.mockResolvedValue({ trend: [], groups: [], models: [] })
    listBudgetEntries.mockResolvedValue({ items: [], total: 0 })
    listAuditEvents.mockResolvedValue({ items: [], total: 0 })
    listKeys.mockResolvedValue([])
    revealKey.mockResolvedValue({ id: 28, member_id: member.id, key: 'sk-plaintext-secret' })
    clipboardCopy.mockResolvedValue(true)
    usageQuery.mockResolvedValue({ items: [], total: 0, page: 1, pages: 0 })
    batchReplaceGroups.mockResolvedValue([{ id: member.id, version: 4, group_ids: [], status: 'disabled', updated_at: '2026-07-02T00:00:00Z' }])
    batchUpdate.mockResolvedValue({ updated_count: 1 })
    createBudgetAdjustment.mockResolvedValue(budgetSummaryFixture(member.id, 'adjusted'))
    restore.mockResolvedValue({ ...member, status: 'disabled', version: 4 })
    setStatus.mockResolvedValue({ ...member, status: 'disabled', version: 4 })
  })

  it('renders a normalized pending member with no groups instead of crashing the page', async () => {
    listMembers.mockResolvedValue([{ ...member, status: 'disabled', group_ids: [] }])

    const wrapper = mountView()
    await flushPromises()

    expect(showError).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('enterpriseMembers.copy.pendingConfiguration')
    expect(wrapper.text()).toContain('enterpriseMembers.copy.noGroupsBoundKeysCannotCall')
  })

  it('copies a member key through the enterprise-scoped reveal endpoint', async () => {
    const key = {
      id: 28,
      user_id: member.enterprise_user_id,
      member_id: member.id,
      name: '张三',
      key: 'sk-77c...0d6e',
      tags: [],
      group_id: null,
      status: 'active',
      ip_whitelist: [],
      ip_blacklist: [],
      last_used_at: null,
      last_used_ip: null,
      quota: 0,
      quota_used: 0,
      expires_at: null,
      created_at: '2026-07-01T00:00:00Z',
      updated_at: '2026-07-20T00:00:00Z',
      current_concurrency: 0,
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
    } as const
    listKeys.mockResolvedValue([key])
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openKeys: (target: typeof member) => Promise<void>
      copyMemberKey: (target: typeof key) => Promise<void>
    }

    await vm.openKeys(member)
    await vm.copyMemberKey(key)

    expect(revealKey).toHaveBeenCalledWith(member.id, key.id)
    expect(clipboardCopy).toHaveBeenCalledWith('sk-plaintext-secret', 'enterpriseMembers.copy.keyCopied')
    expect(showError).not.toHaveBeenCalled()
  })

  it('rejects a reveal response that does not match the requested key', async () => {
    const key = {
      id: 28,
      member_id: member.id,
      name: '张三',
      key: 'sk-77c...0d6e',
      status: 'active',
    }
    revealKey.mockResolvedValueOnce({ id: 29, member_id: member.id, key: 'sk-wrong-key' })
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openKeys: (target: typeof member) => Promise<void>
      copyMemberKey: (target: typeof key) => Promise<void>
    }

    await vm.openKeys(member)
    await vm.copyMemberKey(key)

    expect(clipboardCopy).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('enterpriseMembers.copy.failedToCopyKey')
  })

  it('discards a late reveal response after switching to another member', async () => {
    const pendingReveal = deferred<{ id: number, member_id: number, key: string }>()
    const key = { id: 28, member_id: member.id, name: '张三', key: 'sk-77c...0d6e', status: 'active' }
    const anotherMember = { ...member, id: 42, member_code: 'member-42', name: '成员 42' }
    revealKey.mockReturnValueOnce(pendingReveal.promise)
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      keyMember: typeof member | null
      openKeys: (target: typeof member) => Promise<void>
      copyMemberKey: (target: typeof key) => Promise<void>
    }

    await vm.openKeys(member)
    const copyPromise = vm.copyMemberKey(key)
    vm.keyMember = anotherMember
    pendingReveal.resolve({ id: key.id, member_id: member.id, key: 'sk-old-member-secret' })
    await copyPromise

    expect(clipboardCopy).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
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

  it('reuses the same batch operation key and frozen target after an unknown response failure', async () => {
    batchUpdate.mockRejectedValueOnce(new Error('network response lost')).mockResolvedValueOnce({ updated_count: 1 })
    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      selectedIds: Set<number>
      bulkSetStatus: (status: 'active' | 'disabled') => void
      confirmMemberStatusChange: () => Promise<void>
    }

    vm.selectedIds.add(member.id)
    vm.bulkSetStatus('disabled')
    await vm.confirmMemberStatusChange()
    vm.bulkSetStatus('disabled')
    await vm.confirmMemberStatusChange()

    expect(createEnterpriseMemberOperationIdempotencyKey).toHaveBeenCalledTimes(1)
    expect(batchUpdate).toHaveBeenCalledTimes(2)
    expect(batchUpdate.mock.calls[0]).toEqual(batchUpdate.mock.calls[1])
    expect(batchUpdate).toHaveBeenCalledWith(
      [expect.objectContaining({ id: member.id, version: member.version })],
      { status: 'disabled', group_mode: 'keep' },
      'stable-operation-key'
    )
  })

  it('does not let an older member budget request overwrite a newer dialog session', async () => {
    const first = deferred<Record<string, unknown>>()
    const second = deferred<Record<string, unknown>>()
    const secondMember = { ...member, id: 42, member_code: 'member-42', name: '成员 42' }
    getBudget.mockImplementation((memberID: number) => memberID === member.id ? first.promise : second.promise)

    const wrapper = mountView()
    await nextTick()
    await nextTick()
    const vm = wrapper.vm as unknown as {
      budgetMember: typeof member | null
      budgetSummary: Record<string, unknown> | null
      budgetLoading: boolean
      openBudget: (target: typeof member) => Promise<void>
    }

    const firstOpen = vm.openBudget(member)
    const secondOpen = vm.openBudget(secondMember)
    second.resolve(budgetSummaryFixture(secondMember.id, 'second'))
    await secondOpen

    expect(vm.budgetMember?.id).toBe(secondMember.id)
    expect(vm.budgetSummary).toEqual(budgetSummaryFixture(secondMember.id, 'second'))
    expect(vm.budgetLoading).toBe(false)

    first.resolve(budgetSummaryFixture(member.id, 'first'))
    await firstOpen

    expect(vm.budgetMember?.id).toBe(secondMember.id)
    expect(vm.budgetSummary).toEqual(budgetSummaryFixture(secondMember.id, 'second'))
    expect(vm.budgetLoading).toBe(false)
  })

  it('shows small monthly budget usage without rounding it down to zero', async () => {
    getBudget.mockResolvedValue({
      ...budgetSummaryFixture(member.id, 'small-usage'),
      used_usd: 0.09,
      reserved_usd: 253.38,
      remaining_usd: 99.91,
      request_count: 3,
      input_tokens: 24_000,
      output_tokens: 528,
    })

    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openBudget: (target: typeof member) => Promise<void>
    }

    await vm.openBudget(member)
    await nextTick()

    expect(wrapper.text()).toContain('0.09%')
    expect(wrapper.text()).toContain('enterpriseMembers.copy.monthlyBudget')
    expect(wrapper.text()).toContain('enterpriseMembers.copy.usedThisMonth')
    expect(wrapper.text()).toContain('enterpriseMembers.copy.availableBudget')
    expect(wrapper.text()).not.toContain('enterpriseMembers.copy.periodActivity')
    expect(wrapper.text()).not.toContain('enterpriseMembers.copy.reservedAmount')
    expect(wrapper.text()).not.toContain('US$253.38')
  })

  it('shows actual overage and explains that subsequent requests are stopped', async () => {
    getBudget.mockResolvedValue({
      ...budgetSummaryFixture(member.id, 'over-budget'),
      limit_usd: 100,
      used_usd: 100.2,
      reserved_usd: 80,
      remaining_usd: 0,
    })

    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      openBudget: (target: typeof member) => Promise<void>
    }

    await vm.openBudget(member)
    await nextTick()

    expect(wrapper.text()).toContain('enterpriseMembers.copy.budgetOverage')
    expect(wrapper.text()).toContain('US$0.20')
    expect(wrapper.text()).toContain('enterpriseMembers.copy.budgetRequestsStopped')
    expect(wrapper.text()).not.toContain('US$80.00')
  })

  it('requires project confirmation and freezes the budget adjustment payload before writing', async () => {
    getBudget.mockResolvedValue(budgetSummaryFixture(member.id, 'before-adjustment'))
    const wrapper = mountView()
    await flushPromises()
    const vm = wrapper.vm as unknown as {
      adjustment: { amount: number; note: string }
      pendingBudgetAdjustment: { memberId: number; memberName: string; amount: number; note: string } | null
      openBudget: (target: typeof member) => Promise<void>
      requestBudgetAdjustment: () => void
      confirmBudgetAdjustment: () => Promise<void>
    }

    await vm.openBudget(member)
    vm.adjustment.amount = -1.25
    vm.adjustment.note = '对账修正'
    vm.requestBudgetAdjustment()

    expect(createBudgetAdjustment).not.toHaveBeenCalled()
    expect(vm.pendingBudgetAdjustment).toEqual({
      memberId: member.id,
      memberName: member.name,
      amount: -1.25,
      note: '对账修正',
    })

    vm.adjustment.amount = -9
    vm.adjustment.note = '不应被提交'
    await vm.confirmBudgetAdjustment()

    expect(createBudgetAdjustment).toHaveBeenCalledWith(member.id, -1.25, '对账修正')
    expect(vm.pendingBudgetAdjustment).toBeNull()
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
      formatNumber: (value: number | string) => string
    }
    const baseResult = {
      job_id: 9,
      status: 'completed',
      created_members: 1,
      created_keys: 0,
      member_ids: [member.id],
      pending_members: 1,
      migration_billed_usd: 30,
      migration_total_tokens: '100.00',
      rows: [2],
      keys: [],
      completed_at: '2026-07-14T00:00:00Z',
    }

    vm.importOpen = true
    vm.importResult = { ...baseResult, migration_total_tokens: '421.63' }
    await nextTick()
    expect(wrapper.text()).not.toContain('enterpriseMembers.dynamic.importPeriod')
    expect(vm.formatNumber('421.63')).toBe('421.63')
    expect(vm.formatNumber('1000000.63')).toBe('1,000,000.63')
    expect(vm.formatNumber('9223372036854775807.99')).toBe('9,223,372,036,854,775,807.99')

    vm.importResult = { ...baseResult, period_start: '2026-07-01T00:00:00+08:00', timezone: 'Asia/Shanghai' }
    await nextTick()
    expect(wrapper.text()).toContain('enterpriseMembers.dynamic.importPeriod')
  })
})
