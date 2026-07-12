import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../EnterpriseMembersView.vue')
const source = readFileSync(viewPath, 'utf8')

describe('EnterpriseMembersView layout contract', () => {
  it('stays inside the authenticated application shell', () => {
    expect(source).toContain('<AppLayout>')
    expect(source).toContain("import AppLayout from '@/components/layout/AppLayout.vue'")
  })

  it('uses the existing console visual language instead of a standalone product hero', () => {
    expect(source).not.toContain('enterprise-hero')
    expect(source).not.toContain('Member control plane')
    expect(source).not.toContain('max-w-[1500px]')
    expect(source).toContain('btn btn-primary')
    expect(source).toContain('<EmptyState')
  })

  it('renders members as a dense management list instead of a card grid', () => {
    expect(source).toContain('data-testid="enterprise-member-table"')
    expect(source).toContain('data-testid="enterprise-member-mobile-list"')
    expect(source).toContain('<table class="w-full min-w-[1680px] table-fixed text-left">')
    expect(source).toContain('data-testid="enterprise-member-name-column"')
    expect(source).toContain('data-testid="enterprise-member-code-column"')
    expect(source).toContain('data-testid="enterprise-member-keys-column"')
    expect(source).toContain('data-testid="enterprise-member-groups-column"')
    expect(source).toContain('class="w-[560px] px-4 py-2.5 text-right"')
    expect(source).toContain('class="flex flex-nowrap justify-end gap-1.5"')
    expect(source).toContain("t('enterpriseMembers.copy.selectVisibleMembers')")
    expect(source).toContain("import { tableSelectionCheckboxClasses as selectionCheckboxClasses } from '@/utils/tableSelectionCheckbox'")
    expect(source).toContain(':class="selectionCheckboxClasses(allFilteredMembersSelected || someFilteredMembersSelected)"')
    expect(source).toContain(':class="selectionCheckboxClasses(selectedIds.has(member.id))"')
    expect(source).toContain('role="checkbox"')
    expect(source).toContain('name="check" size="xs" :stroke-width="2.5"')
    expect(source).not.toContain('class="grid gap-4 md:grid-cols-2 xl:grid-cols-3"')
    expect(source).not.toContain('min-h-[285px]')
    expect(source).not.toContain('mt-1 truncate text-sm font-semibold')
    expect(source).not.toContain('class="flex flex-wrap justify-end gap-1.5"')
    expect(source).not.toContain("t('enterpriseMembers.copy.keys') }} / {{ t('enterpriseMembers.copy.groups')")
    expect(source).not.toContain('px-3 py-4 align-top')
    expect(source).not.toContain('px-3 py-3 align-top')
    expect(source).not.toContain('class="h-4 w-4 rounded border-stone-300 text-emerald-600 focus:ring-emerald-500"')
    expect(source).not.toContain('manageNonLoginMemberIdentitiesKeysOrderedGroupsAndCalendarMonthBudgetsInOnePlace')
    expect(source).toContain("import Select, { type SelectOption } from '@/components/common/Select.vue'")
    expect(source).toContain('<Select v-model="statusFilter" :options="memberStatusFilterOptions" class="w-full" />')
    expect(source).toContain('<Select v-model="budgetFilter" :options="memberBudgetFilterOptions" class="w-full" />')
    expect(source).toContain('<Select v-model="sortBy" :options="memberSortOptions" class="w-full" />')
    expect(source).toContain('<Select v-model="archiveScope" :options="memberArchiveScopeOptions" class="w-full" @change="handleArchiveScopeChange" />')
    expect(source).toContain("const includeArchived = computed(() => archiveScope.value === 'with_archived')")
    expect(source).toContain("if (!includeArchived.value && statusFilter.value === 'archived') statusFilter.value = 'all'")
    expect(source).toContain("if (includeArchived.value) options.push({ value: 'archived'")
    expect(source).not.toContain('<select v-model="statusFilter"')
    expect(source).not.toContain('<select v-model="budgetFilter"')
    expect(source).not.toContain('<select v-model="sortBy"')
    expect(source).not.toContain(':aria-pressed="includeArchived"')
    expect(source).not.toContain("includeArchived ? 'eyeOff' : 'eye'")
  })

  it('keeps the member audit trail inside the existing member detail workflow', () => {
    expect(source).toContain("enterpriseMembersAPI.listAuditEvents(member.id)")
    expect(source).toContain('enterpriseMembersAPI.listOwnerAuditEvents()')
    expect(source).toContain("t('enterpriseMembers.copy.auditTrail')")
    expect(source).toContain("t('enterpriseMembers.copy.budgetLedger')")
  })

  it('uses durable import jobs instead of holding the commit request open', () => {
    expect(source).toContain('enterpriseMembersAPI.getImportJob(jobId)')
    expect(source).toContain('enterpriseMembersAPI.consumeImportResultSecrets(jobId, resultToken)')
    expect(source).toContain('enterpriseMembersAPI.downloadImportErrorReport(jobId)')
    expect(source).toContain("t('enterpriseMembers.copy.processingInBackground')")
  })

  it('adopts existing keys without silently dropping their original group', () => {
    expect(source).toContain('enterpriseMembersAPI.listAdoptableKeys(member.id)')
    expect(source).toContain('enterpriseMembersAPI.adoptKey(member, key.id)')
    expect(source).toContain("'member_key.adopted': t('enterpriseMembers.copy.existingKeyAdopted')")
    expect(source).toContain("t('enterpriseMembers.copy.adoptExistingKeys')")
    expect(source).toContain("t('enterpriseMembers.copy.appendToRoute')")
  })

  it('shows member request records without exposing upstream routing internals', () => {
    expect(source).toContain('enterpriseMembersAPI.listUsageRecords(member.id, 1, 20)')
    expect(source).toContain("t('enterpriseMembers.copy.requestRecords')")
    expect(source).toContain("t('enterpriseMembers.copy.showsMemberFacingKeyModelPublicGroupAndBilledCostOnlyUpstreamAccountChannelAndMarginDataRemainPr')")
    expect(source).not.toContain('record.account_id')
    expect(source).not.toContain('record.channel_id')
    expect(source).not.toContain('record.account_cost')
  })

  it('uses formal locale keys instead of a page-local bilingual helper', () => {
    expect(source).not.toMatch(/\btext\(/)
    expect(source).not.toContain('const text =')
    expect(source).toContain("const { t, locale } = useI18n()")
  })

  it('treats member identity, aggregate limits, usage adjustments, and accessible groups as durable product concepts', () => {
    expect(source).toContain("t('enterpriseMembers.copy.memberIdentifier')")
    expect(source).toContain(':disabled="Boolean(editingMember)"')
    expect(source).not.toContain("t('enterpriseMembers.copy.stableMemberCode')")
    expect(source).toContain('v-model.number="draft.rate_limit_5h"')
    expect(source).toContain('v-model.number="draft.rate_limit_1d"')
    expect(source).toContain('v-model.number="draft.rate_limit_7d"')
    expect(source).toContain('v-model.number="editorMonthlyUsed"')
    expect(source).toContain('v-model.number="editorUsage5h"')
    expect(source).not.toContain('<label v-if="editingMember" class="mt-3 block"><span class="input-label">{{ t(\'enterpriseMembers.copy.used\') }}</span>')
    expect(source).toContain("editingMember ? t('enterpriseMembers.copy.used') : t('enterpriseMembers.copy.initialUsed')")
    expect(source).toContain('monthly_used_usd: editorMonthlyUsed.value')
    expect(source).not.toContain('usage_opening_note')
    expect(source).not.toContain('editorAdjustmentNote')
    expect(source).not.toContain("t('enterpriseMembers.copy.adjustmentReason')")
    expect(source).toContain('enterpriseMembersAPI.setUsage(updated.id')
    expect(source).toContain("error.response?.data?.message || error.message || t('enterpriseMembers.copy.saveFailedPleaseRefreshAndRetry')")
    expect(source).toContain("t('enterpriseMembers.copy.memberAccessibleGroups')")
    expect(source).not.toContain("t('enterpriseMembers.copy.orderedGroupCandidates')")
  })
})
