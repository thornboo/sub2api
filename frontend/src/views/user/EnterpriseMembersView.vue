<template>
  <AppLayout>
    <div class="space-y-6">
      <section v-if="!isEnterprise" class="card border-amber-200 bg-amber-50/80 p-6 dark:border-amber-900/60 dark:bg-amber-950/20 sm:p-8">
        <div class="flex items-start gap-4">
          <span class="flex h-11 w-11 flex-none items-center justify-center rounded-xl bg-amber-100 text-amber-700 dark:bg-amber-400/10 dark:text-amber-300">
            <Icon name="shield" size="lg" />
          </span>
          <div>
            <h1 class="text-xl font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.thisPageIsAvailableOnlyToEnabledEnterpriseAccounts') }}</h1>
            <p class="mt-2 max-w-2xl text-sm leading-6 text-stone-600 dark:text-stone-300">{{ t('enterpriseMembers.copy.membersAreNonLoginIdentitiesAnAdministratorMustEnableTheEnterpriseAccountTypeFirst') }}</p>
          </div>
        </div>
      </section>

      <template v-else>
        <section class="card overflow-hidden p-0">
          <div class="flex flex-col gap-5 p-5 sm:p-6 lg:flex-row lg:items-center lg:justify-between">
            <div class="flex items-start gap-4">
              <span class="flex h-11 w-11 flex-none items-center justify-center rounded-xl border border-emerald-200/80 bg-emerald-50 text-emerald-700 dark:border-emerald-400/20 dark:bg-emerald-400/10 dark:text-emerald-300">
                <Icon name="users" size="lg" />
              </span>
              <div>
                <h2 class="text-base font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.memberOverview') }}</h2>
              </div>
            </div>
            <div class="flex flex-wrap gap-3 sm:justify-end">
              <button class="btn btn-secondary" type="button" @click="openOwnerAudit"><Icon name="clock" size="sm" />{{ t('enterpriseMembers.copy.auditTrail') }}</button>
              <button class="btn btn-secondary" type="button" @click="openImport"><Icon name="upload" size="sm" />{{ t('enterpriseMembers.copy.import') }}</button>
              <button class="btn btn-primary" type="button" @click="openCreate">
                <Icon name="plus" size="sm" />{{ t('enterpriseMembers.copy.newMember') }}
              </button>
            </div>
          </div>
          <dl class="grid border-t border-stone-200/80 bg-stone-50/60 dark:border-white/10 dark:bg-white/[0.025] sm:grid-cols-2 xl:grid-cols-4">
            <div class="px-5 py-4 sm:px-6"><dt class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.totalMembers') }}</dt><dd class="mt-1 text-2xl font-semibold text-stone-950 dark:text-white">{{ members.length }}</dd></div>
            <div class="border-stone-200/80 px-5 py-4 dark:border-white/10 sm:border-l sm:px-6"><dt class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.currentlyActive') }}</dt><dd class="mt-1 text-2xl font-semibold text-emerald-600 dark:text-emerald-300">{{ activeCount }}</dd></div>
            <div class="border-stone-200/80 px-5 py-4 dark:border-white/10 xl:border-l xl:px-6"><dt class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.memberSpendThisMonth') }}</dt><dd class="mt-1 text-2xl font-semibold text-stone-950 dark:text-white">{{ formatMoney(ownerUsageSummary?.used_usd || 0) }}</dd></div>
            <div class="border-stone-200/80 px-5 py-4 dark:border-white/10 sm:border-l sm:px-6"><dt class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.memberKeys') }}</dt><dd class="mt-1 text-2xl font-semibold text-stone-950 dark:text-white">{{ totalKeyCount }}</dd></div>
          </dl>
        </section>

        <section class="card p-4 sm:p-5">
          <div class="grid gap-3 lg:grid-cols-[minmax(240px,1fr)_160px_170px_170px_180px_auto]">
          <label class="relative block">
            <span class="sr-only">{{ t('enterpriseMembers.copy.searchMembers') }}</span>
            <Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-stone-400" />
            <input v-model.trim="search" class="input pl-10" :placeholder="t('enterpriseMembers.copy.searchNameOrStableCode')" />
          </label>
          <Select v-model="statusFilter" :options="memberStatusFilterOptions" class="w-full" />
          <Select v-model="budgetFilter" :options="memberBudgetFilterOptions" class="w-full" />
          <Select v-model="sortBy" :options="memberSortOptions" class="w-full" />
          <Select v-model="archiveScope" :options="memberArchiveScopeOptions" class="w-full" @change="handleArchiveScopeChange" />
            <button class="btn btn-secondary" type="button" :disabled="loading" @click="loadMembers"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />{{ t('enterpriseMembers.copy.refresh') }}</button>
          </div>

        <div v-if="selectedIds.size" class="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/50 dark:bg-amber-950/20">
          <p class="text-sm font-medium text-amber-900 dark:text-amber-100">{{ t('enterpriseMembers.dynamic.selectedMembers', { count: selectedIds.size }) }}</p>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-primary btn-sm" type="button" :disabled="!batchEditableTargets.length || batchTargetLimitExceeded" @click="batchPolicyOpen = true"><Icon name="edit" size="sm" />{{ t('enterpriseMembers.copy.batchEdit') }}</button>
            <button class="btn btn-secondary btn-sm" type="button" :disabled="!batchEditableTargets.length || batchTargetLimitExceeded" @click="batchUsageOpen = true"><Icon name="chartBar" size="sm" />{{ t('enterpriseMembers.copy.adjustUsedUsage') }}</button>
            <button class="btn btn-secondary btn-sm" type="button" :disabled="!batchEditableTargets.length || batchTargetLimitExceeded" @click="openBatchGroups()"><Icon name="users" size="sm" />{{ t('enterpriseMembers.copy.setAccessibleGroups') }}</button>
            <button class="btn btn-secondary btn-sm" type="button" :disabled="memberStatusUpdating || !batchEditableTargets.length || batchTargetLimitExceeded" @click="bulkSetStatus('active')">{{ t('enterpriseMembers.copy.enable') }}</button>
            <button class="btn btn-secondary btn-sm" type="button" :disabled="memberStatusUpdating || !batchEditableTargets.length || batchTargetLimitExceeded" @click="bulkSetStatus('disabled')">{{ t('enterpriseMembers.copy.disable') }}</button>
          </div>
          <p v-if="batchTargetLimitExceeded" class="basis-full text-xs text-rose-700 dark:text-rose-300">{{ t('enterpriseMembers.copy.batchMaximum500Members') }}</p>
        </div>
        </section>

        <section v-if="loading" class="card overflow-hidden p-0" aria-live="polite">
          <div class="h-11 animate-pulse border-b border-stone-200/80 bg-stone-100/80 dark:border-white/10 dark:bg-white/[0.05]"></div>
          <div class="divide-y divide-stone-100 dark:divide-white/10">
            <div v-for="n in 6" :key="n" class="flex h-20 animate-pulse items-center gap-5 px-5">
              <span class="h-4 w-4 rounded bg-stone-200 dark:bg-white/10"></span>
              <span class="h-9 w-40 rounded-lg bg-stone-100 dark:bg-white/[0.06]"></span>
              <span class="h-7 w-16 rounded-full bg-stone-100 dark:bg-white/[0.06]"></span>
              <span class="hidden h-9 flex-1 rounded-lg bg-stone-100 dark:bg-white/[0.06] sm:block"></span>
            </div>
          </div>
        </section>
        <section v-else-if="filteredMembers.length === 0" class="card border-dashed py-4">
          <EmptyState
            :title="members.length === 0 ? t('enterpriseMembers.copy.noEnterpriseMembersYet') : t('enterpriseMembers.copy.noMatchingMembers')"
            :description="members.length === 0 ? t('enterpriseMembers.copy.createTheFirstNonLoginMemberIdentityOrImportMembersFromAServerTemplate') : t('enterpriseMembers.copy.adjustTheSearchOrFiltersAndTryAgain')"
          >
            <template #icon><Icon name="users" size="xl" class="text-stone-400" /></template>
            <template #action>
              <div class="flex flex-wrap justify-center gap-3">
                <button v-if="members.length === 0" class="btn btn-secondary" type="button" @click="openImport"><Icon name="upload" size="sm" />{{ t('enterpriseMembers.copy.importMembers') }}</button>
                <button v-if="members.length === 0" class="btn btn-primary" type="button" @click="openCreate"><Icon name="plus" size="sm" />{{ t('enterpriseMembers.copy.newMember') }}</button>
                <button v-else class="btn btn-secondary" type="button" @click="resetFilters">{{ t('enterpriseMembers.copy.clearFilters') }}</button>
              </div>
            </template>
          </EmptyState>
        </section>
        <section v-else class="card overflow-hidden p-0" data-testid="enterprise-member-list">
          <div class="hidden overflow-x-auto lg:block" data-testid="enterprise-member-table">
            <table class="w-full min-w-[1680px] table-fixed text-left">
              <thead class="border-b border-stone-200/80 bg-stone-50/90 text-xs font-medium text-stone-500 dark:border-white/10 dark:bg-neutral-900/85 dark:text-stone-400">
                <tr>
                  <th class="w-12 px-4 py-2.5">
                    <button
                      type="button"
                      role="checkbox"
                      :class="selectionCheckboxClasses(allFilteredMembersSelected || someFilteredMembersSelected)"
                      :aria-checked="someFilteredMembersSelected && !allFilteredMembersSelected ? 'mixed' : allFilteredMembersSelected"
                      :aria-label="t('enterpriseMembers.copy.selectVisibleMembers')"
                      @click.stop="toggleAllFilteredMembers"
                      @keydown.space.prevent="toggleAllFilteredMembers"
                    >
                      <Icon v-if="allFilteredMembersSelected" name="check" size="xs" :stroke-width="2.5" />
                      <span v-else-if="someFilteredMembersSelected" class="h-0.5 w-2.5 rounded-full bg-current" />
                    </button>
                  </th>
                  <th class="w-40 px-3 py-2.5" data-testid="enterprise-member-name-column">{{ t('enterpriseMembers.copy.member') }}</th>
                  <th class="w-24 px-3 py-2.5" data-testid="enterprise-member-code-column">{{ t('enterpriseMembers.copy.code') }}</th>
                  <th class="w-24 px-3 py-2.5">{{ t('enterpriseMembers.copy.status') }}</th>
                  <th class="w-56 px-3 py-2.5">{{ t('enterpriseMembers.copy.monthlyBudget') }} / {{ t('enterpriseMembers.copy.usedThisMonth') }}</th>
                  <th class="w-20 px-3 py-2.5" data-testid="enterprise-member-keys-column">{{ t('enterpriseMembers.copy.keys') }}</th>
                  <th class="w-20 px-3 py-2.5" data-testid="enterprise-member-groups-column">{{ t('enterpriseMembers.copy.groups') }}</th>
                  <th class="w-64 px-3 py-2.5">{{ t('enterpriseMembers.copy.routingOrder') }}</th>
                  <th class="w-36 px-3 py-2.5">{{ t('enterpriseMembers.copy.updated') }}</th>
                  <th class="w-[560px] px-4 py-2.5 text-right">{{ t('enterpriseMembers.copy.action') }}</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-stone-100 dark:divide-white/10">
                <tr v-for="member in filteredMembers" :key="member.id" class="align-middle transition-colors hover:bg-stone-50/70 dark:hover:bg-white/[0.025]" :class="member.deleted_at ? 'opacity-70' : ''">
                  <td class="px-4 py-2 align-middle">
                    <button
                      type="button"
                      role="checkbox"
                      :class="selectionCheckboxClasses(selectedIds.has(member.id))"
                      :aria-checked="selectedIds.has(member.id)"
                      :aria-label="t('enterpriseMembers.dynamic.selectMember', { name: member.name })"
                      @click.stop="toggleSelected(member.id)"
                      @keydown.space.prevent="toggleSelected(member.id)"
                    >
                      <Icon v-if="selectedIds.has(member.id)" name="check" size="xs" :stroke-width="2.5" />
                    </button>
                  </td>
                  <td class="px-3 py-2 align-middle">
                    <p class="truncate text-sm font-semibold leading-5 text-stone-950 dark:text-white" :title="member.name">{{ member.name }}</p>
                  </td>
                  <td class="overflow-hidden px-3 py-2 align-middle">
                    <code class="block w-full truncate font-mono text-xs text-stone-500" :title="member.member_code">{{ member.member_code }}</code>
                  </td>
                  <td class="px-3 py-2 align-middle">
                    <span class="inline-flex rounded-full px-2.5 py-0.5 text-xs font-semibold" :class="statusClass(member)">{{ statusLabel(member) }}</span>
                  </td>
                  <td class="px-3 py-2 align-middle">
                    <div class="flex items-baseline justify-between gap-3">
                      <p class="whitespace-nowrap text-sm font-semibold tabular-nums text-stone-900 dark:text-white">{{ formatMoney(memberUsage(member.id)?.used_usd || 0) }}</p>
                      <p class="whitespace-nowrap text-xs tabular-nums text-stone-500">/ {{ member.monthly_limit_usd > 0 ? formatMoney(member.monthly_limit_usd) : t('enterpriseMembers.copy.unlimited6381d248') }}</p>
                    </div>
                    <div v-if="member.monthly_limit_usd > 0" class="mt-1 h-1 overflow-hidden rounded-full bg-stone-100 dark:bg-white/[0.08]">
                      <div class="h-full rounded-full transition-[width]" :class="memberBudgetBarClass(member)" :style="{ width: `${memberBudgetPercent(member)}%` }"></div>
                    </div>
                  </td>
                  <td class="px-3 py-2 align-middle text-sm font-semibold tabular-nums text-stone-900 dark:text-white">
                    {{ member.key_count }}
                  </td>
                  <td class="px-3 py-2 align-middle text-sm font-semibold tabular-nums text-stone-900 dark:text-white">
                    {{ member.group_ids.length }}
                  </td>
                  <td class="px-3 py-2 align-middle">
                    <div class="flex flex-wrap gap-1.5">
                      <span v-for="(groupId, index) in member.group_ids.slice(0, 4)" :key="groupId" class="inline-flex items-center gap-1 rounded-lg border border-stone-200 px-2 py-0.5 text-xs text-stone-600 dark:border-white/10 dark:text-stone-300"><b class="text-emerald-600 dark:text-emerald-300">{{ index + 1 }}</b>{{ groupName(groupId) }}</span>
                      <span v-if="member.group_ids.length > 4" class="rounded-lg bg-stone-100 px-2 py-0.5 text-xs text-stone-500 dark:bg-white/5">+{{ member.group_ids.length - 4 }}</span>
                      <span v-if="!member.group_ids.length" class="text-xs leading-5 text-rose-600 dark:text-rose-300">{{ t('enterpriseMembers.copy.noGroupsBoundKeysCannotCall') }}</span>
                    </div>
                  </td>
                  <td class="whitespace-nowrap px-3 py-2 align-middle text-xs tabular-nums text-stone-500">{{ formatDateTime(member.updated_at) }}</td>
                  <td class="whitespace-nowrap px-4 py-2 align-middle">
                    <div class="flex flex-nowrap justify-end gap-1.5">
                      <button class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap" type="button" @click="openBudget(member)"><Icon name="chartBar" size="sm" />{{ t('enterpriseMembers.copy.budgetUsage') }}</button>
                      <button class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap" type="button" @click="openKeys(member)"><Icon name="key" size="sm" />{{ t('enterpriseMembers.copy.keys') }}</button>
                      <button v-if="!member.deleted_at" class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap" type="button" @click="openEdit(member)"><Icon name="edit" size="sm" />{{ t('enterpriseMembers.copy.edit') }}</button>
                      <button v-if="!member.deleted_at" class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap" type="button" :disabled="memberStatusUpdating" @click="toggleStatus(member)">{{ member.status === 'active' ? t('enterpriseMembers.copy.disable5dac4e9c') : t('enterpriseMembers.copy.enable14891bd4') }}</button>
                      <button v-if="member.deleted_at" class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap" type="button" :disabled="restoringMemberId === member.id" @click="restoreMember(member)"><Icon name="refresh" size="sm" :class="restoringMemberId === member.id ? 'animate-spin' : ''" />{{ t('enterpriseMembers.copy.restore') }}</button>
                      <button class="shrink-0 whitespace-nowrap rounded-xl px-3 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-50 dark:text-rose-300 dark:hover:bg-rose-950/30" type="button" @click="removeMember(member)">{{ member.deleted_at ? t('enterpriseMembers.copy.deleteMember') : t('enterpriseMembers.copy.archive') }}</button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="divide-y divide-stone-100 dark:divide-white/10 lg:hidden" data-testid="enterprise-member-mobile-list">
            <div class="flex items-center gap-3 bg-stone-50/90 px-4 py-3 dark:bg-neutral-900/85">
              <button
                type="button"
                role="checkbox"
                :class="selectionCheckboxClasses(allFilteredMembersSelected || someFilteredMembersSelected)"
                :aria-checked="someFilteredMembersSelected && !allFilteredMembersSelected ? 'mixed' : allFilteredMembersSelected"
                :aria-label="t('enterpriseMembers.copy.selectVisibleMembers')"
                @click.stop="toggleAllFilteredMembers"
                @keydown.space.prevent="toggleAllFilteredMembers"
              >
                <Icon v-if="allFilteredMembersSelected" name="check" size="xs" :stroke-width="2.5" />
                <span v-else-if="someFilteredMembersSelected" class="h-0.5 w-2.5 rounded-full bg-current" />
              </button>
              <span class="text-xs font-medium text-stone-500 dark:text-stone-400">{{ t('enterpriseMembers.copy.selectVisibleMembers') }}</span>
            </div>
            <article v-for="member in filteredMembers" :key="member.id" class="px-4 py-4" :class="member.deleted_at ? 'opacity-70' : ''">
              <div class="flex items-start gap-3">
                <button
                  type="button"
                  role="checkbox"
                  :class="[selectionCheckboxClasses(selectedIds.has(member.id)), 'mt-0.5 shrink-0']"
                  :aria-checked="selectedIds.has(member.id)"
                  :aria-label="t('enterpriseMembers.dynamic.selectMember', { name: member.name })"
                  @click.stop="toggleSelected(member.id)"
                  @keydown.space.prevent="toggleSelected(member.id)"
                >
                  <Icon v-if="selectedIds.has(member.id)" name="check" size="xs" :stroke-width="2.5" />
                </button>
                <div class="min-w-0 flex-1">
                  <div class="flex items-start justify-between gap-3">
                    <div class="min-w-0">
                      <h2 class="break-words text-sm font-semibold leading-5 text-stone-950 dark:text-white">{{ member.name }}</h2>
                      <p class="mt-0.5 break-all font-mono text-xs text-stone-500">{{ member.member_code }}</p>
                    </div>
                    <span class="flex-none rounded-full px-2.5 py-1 text-xs font-semibold" :class="statusClass(member)">{{ statusLabel(member) }}</span>
                  </div>

                  <dl class="mt-4 grid grid-cols-2 gap-x-5 gap-y-3 text-xs">
                    <div><dt class="text-stone-500">{{ t('enterpriseMembers.copy.monthlyBudget') }}</dt><dd class="mt-1 whitespace-nowrap font-semibold tabular-nums text-stone-900 dark:text-white">{{ member.monthly_limit_usd > 0 ? formatMoney(member.monthly_limit_usd) : t('enterpriseMembers.copy.unlimited6381d248') }}</dd></div>
                    <div><dt class="text-stone-500">{{ t('enterpriseMembers.copy.usedThisMonth') }}</dt><dd class="mt-1 whitespace-nowrap font-semibold tabular-nums text-stone-900 dark:text-white">{{ formatMoney(memberUsage(member.id)?.used_usd || 0) }}</dd></div>
                    <div><dt class="text-stone-500">{{ t('enterpriseMembers.copy.keys') }}</dt><dd class="mt-1 font-semibold tabular-nums text-stone-900 dark:text-white">{{ member.key_count }}</dd></div>
                    <div><dt class="text-stone-500">{{ t('enterpriseMembers.copy.groups') }}</dt><dd class="mt-1 font-semibold tabular-nums text-stone-900 dark:text-white">{{ member.group_ids.length }}</dd></div>
                  </dl>

                  <div class="mt-4">
                    <p class="mb-2 text-[11px] font-medium text-stone-500">{{ t('enterpriseMembers.copy.routingOrder') }}</p>
                    <div class="flex flex-wrap gap-1.5">
                      <span v-for="(groupId, index) in member.group_ids.slice(0, 4)" :key="groupId" class="inline-flex items-center gap-1 rounded-lg border border-stone-200 px-2 py-1 text-xs text-stone-600 dark:border-white/10 dark:text-stone-300"><b class="text-emerald-600 dark:text-emerald-300">{{ index + 1 }}</b>{{ groupName(groupId) }}</span>
                      <span v-if="member.group_ids.length > 4" class="rounded-lg bg-stone-100 px-2 py-1 text-xs text-stone-500 dark:bg-white/5">+{{ member.group_ids.length - 4 }}</span>
                      <span v-if="!member.group_ids.length" class="text-xs text-rose-600 dark:text-rose-300">{{ t('enterpriseMembers.copy.noGroupsBoundKeysCannotCall') }}</span>
                    </div>
                  </div>

                  <div class="mt-4 flex flex-wrap gap-2 border-t border-stone-100 pt-3 dark:border-white/10">
                    <button class="btn btn-secondary btn-sm" type="button" @click="openBudget(member)"><Icon name="chartBar" size="sm" />{{ t('enterpriseMembers.copy.budgetUsage') }}</button>
                    <button class="btn btn-secondary btn-sm" type="button" @click="openKeys(member)"><Icon name="key" size="sm" />{{ t('enterpriseMembers.copy.keys') }}</button>
                    <button v-if="!member.deleted_at" class="btn btn-secondary btn-sm" type="button" @click="openEdit(member)"><Icon name="edit" size="sm" />{{ t('enterpriseMembers.copy.edit') }}</button>
                    <button v-if="!member.deleted_at" class="btn btn-secondary btn-sm" type="button" :disabled="memberStatusUpdating" @click="toggleStatus(member)">{{ member.status === 'active' ? t('enterpriseMembers.copy.disable5dac4e9c') : t('enterpriseMembers.copy.enable14891bd4') }}</button>
                    <button v-if="member.deleted_at" class="btn btn-secondary btn-sm" type="button" :disabled="restoringMemberId === member.id" @click="restoreMember(member)"><Icon name="refresh" size="sm" :class="restoringMemberId === member.id ? 'animate-spin' : ''" />{{ t('enterpriseMembers.copy.restore') }}</button>
                    <button class="rounded-xl px-3 py-1.5 text-xs font-medium text-rose-600 hover:bg-rose-50 dark:text-rose-300 dark:hover:bg-rose-950/30" type="button" @click="removeMember(member)">{{ member.deleted_at ? t('enterpriseMembers.copy.deleteMember') : t('enterpriseMembers.copy.archive') }}</button>
                  </div>
                </div>
              </div>
            </article>
          </div>
        </section>
      </template>

    <BaseDialog :show="ownerAuditOpen" :title="t('enterpriseMembers.copy.enterpriseMemberAuditTrail')" width="extra-wide" @close="ownerAuditOpen = false">
      <div v-if="ownerAuditLoading" class="py-16 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.loadingImmutableAuditRecords') }}</div>
      <div v-else class="space-y-4">
        <div class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 px-4 py-3 dark:border-emerald-800/50 dark:bg-emerald-950/20">
          <p class="text-xs leading-5 text-emerald-900 dark:text-emerald-100">{{ t('enterpriseMembers.copy.recordsAreWrittenByTransactionalDatabaseTriggersAndCannotBeEditedOrDeletedPayloadsAreAllowListed') }}</p>
          <span class="whitespace-nowrap rounded-full bg-white/80 px-3 py-1 text-xs font-semibold text-emerald-800 dark:bg-white/10 dark:text-emerald-200">{{ ownerAuditTotal }} {{ t('enterpriseMembers.copy.events') }}</span>
        </div>
        <div v-if="ownerAuditEvents.length" class="max-h-[62vh] overflow-auto rounded-2xl border border-stone-200 dark:border-white/10">
          <table class="w-full min-w-[760px] text-left text-xs">
            <thead class="sticky top-0 z-10 bg-stone-50 text-stone-500 dark:bg-neutral-900"><tr><th class="px-4 py-3">{{ t('enterpriseMembers.copy.time') }}</th><th class="px-4 py-3">{{ t('enterpriseMembers.copy.member') }}</th><th class="px-4 py-3">{{ t('enterpriseMembers.copy.action') }}</th><th class="px-4 py-3">{{ t('enterpriseMembers.copy.changeSummary') }}</th></tr></thead>
            <tbody class="divide-y divide-stone-100 dark:divide-white/10">
              <tr v-for="event in ownerAuditEvents" :key="event.id" class="align-top hover:bg-stone-50/70 dark:hover:bg-white/[0.025]">
                <td class="whitespace-nowrap px-4 py-3 text-stone-500">{{ formatDateTime(event.created_at) }}</td>
                <td class="px-4 py-3"><span class="font-medium text-stone-800 dark:text-stone-100">{{ auditMemberLabel(event) }}</span><span v-if="event.member_id" class="ml-1 font-mono text-[10px] text-stone-400">#{{ event.member_id }}</span></td>
                <td class="px-4 py-3"><p class="font-medium text-stone-900 dark:text-white">{{ auditActionLabel(event.action) }}</p><p class="mt-1 font-mono text-[10px] uppercase tracking-wide text-stone-400">{{ auditEntityLabel(event.entity_type) }}<template v-if="event.entity_id"> #{{ event.entity_id }}</template></p></td>
                <td class="max-w-xl px-4 py-3 leading-5 text-stone-500 dark:text-stone-400">{{ auditEventSummary(event) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-else class="rounded-2xl bg-stone-50 py-16 text-center text-sm text-stone-500 dark:bg-white/[0.03]">{{ t('enterpriseMembers.copy.noAuditRecordsYet') }}</p>
        <p v-if="ownerAuditTotal > ownerAuditEvents.length" class="text-center text-xs text-stone-400">{{ t('enterpriseMembers.dynamic.showingLatestOwnerAudit', { count: ownerAuditEvents.length }) }}</p>
      </div>
    </BaseDialog>

    <BaseDialog :show="batchGroupsOpen" :title="t('enterpriseMembers.copy.batchSetAccessibleGroups')" width="wide" @close="batchGroupsOpen = false">
      <div class="space-y-5">
        <section class="rounded-2xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.04]">
          <p class="text-sm font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.dynamic.selectedMembers', { count: selectedIds.size }) }}</p>
          <p class="mt-1 text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.batchGroupPolicyHint') }}</p>
          <label class="mt-4 block"><span class="input-label">{{ t('enterpriseMembers.copy.updateMode') }}</span><Select v-model="batchGroupMode" :options="batchGroupModeOptions" class="w-full" /></label>
        </section>
        <fieldset>
          <legend class="input-label">{{ t('enterpriseMembers.copy.memberAccessibleGroups') }}</legend>
          <div class="mt-2 max-h-72 space-y-2 overflow-y-auto rounded-2xl border border-stone-200 p-2 dark:border-white/10">
            <div v-for="group in availableGroups" :key="group.id" class="flex items-center gap-3 rounded-xl px-3 py-2.5 hover:bg-stone-50 dark:hover:bg-white/5">
              <button type="button" role="checkbox" :aria-checked="batchGroupIds.includes(group.id)" :aria-label="group.name" :class="selectionCheckboxClasses(batchGroupIds.includes(group.id))" @click="toggleBatchGroup(group.id)"><Icon v-if="batchGroupIds.includes(group.id)" name="check" size="xs" :stroke-width="2.5" /></button>
              <span class="min-w-0 flex-1"><b class="block truncate text-sm text-stone-900 dark:text-white">{{ group.name }}</b><span class="text-xs text-stone-500">{{ group.platform }}</span></span>
              <template v-if="batchGroupIds.includes(group.id)">
                <span class="rounded-lg bg-amber-100 px-2 py-1 text-xs font-bold text-amber-800 dark:bg-amber-300/10 dark:text-amber-200">#{{ batchGroupIds.indexOf(group.id) + 1 }}</span>
                <button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveUp')" @click.prevent="moveBatchGroup(group.id, -1)">↑</button>
                <button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveDown')" @click.prevent="moveBatchGroup(group.id, 1)">↓</button>
              </template>
            </div>
          </div>
          <p v-if="batchGroupMode === 'replace' && !batchGroupIds.length" class="mt-2 text-xs text-amber-700 dark:text-amber-300">{{ t('enterpriseMembers.copy.emptyReplaceClearsGroupsAndMembersCannotBeEnabled') }}</p>
        </fieldset>
      </div>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" @click="batchGroupsOpen = false">{{ t('enterpriseMembers.copy.cancel') }}</button><button class="btn" :class="batchClearIsDestructive ? 'btn-danger' : 'btn-primary'" type="button" :disabled="batchGroupsSaving" @click="requestSaveBatchGroups">{{ batchGroupsSaving ? t('enterpriseMembers.copy.saving') : batchClearIsDestructive ? t('enterpriseMembers.dynamic.clearGroupsAndDisableMembersAction', { count: batchGroupTargetCount }) : t('enterpriseMembers.copy.applyToSelectedMembers') }}</button></div></template>
    </BaseDialog>

    <ConfirmDialog
      :show="batchClearConfirmOpen"
      :title="t('enterpriseMembers.copy.clearGroupsAndDisableMembers')"
      :message="t('enterpriseMembers.dynamic.clearGroupsAndDisableMembersConfirm', { count: batchGroupTargetCount })"
      :confirm-text="t('enterpriseMembers.dynamic.clearGroupsAndDisableMembersAction', { count: batchGroupTargetCount })"
      :danger="true"
      @confirm="confirmBatchGroupClear"
      @cancel="cancelBatchGroupClear"
    />

    <EnterpriseMemberBatchPolicyDialog
      :show="batchPolicyOpen"
      :member-count="batchEditableTargets.length"
      :available-groups="availableGroups"
      :saving="batchPolicySaving"
      @close="closeBatchPolicy"
      @submit="saveBatchPolicy"
    />

    <EnterpriseMemberBatchUsageDialog
      :show="batchUsageOpen"
      :targets="batchUsageTargets"
      :saving="batchUsageSaving"
      @close="closeBatchUsage"
      @submit="saveBatchUsage"
    />

    <ConfirmDialog
      :show="Boolean(memberStatusChangeRequest)"
      :title="memberStatusChangeTitle"
      :message="memberStatusChangeMessage"
      :confirm-text="memberStatusChangeConfirmText"
      :danger="memberStatusChangeRequest?.status === 'disabled'"
      @confirm="confirmMemberStatusChange"
      @cancel="cancelMemberStatusChange"
    />

    <ConfirmDialog
      :show="Boolean(memberRemovalTarget)"
      :title="memberRemovalTarget?.deleted_at ? t('enterpriseMembers.copy.deleteMember') : t('enterpriseMembers.copy.memberArchived')"
      :message="memberRemovalMessage(memberRemovalTarget)"
      :confirm-text="memberRemovalTarget?.deleted_at ? t('enterpriseMembers.copy.deleteMember') : t('enterpriseMembers.copy.archive')"
      :danger="true"
      @confirm="confirmRemoveMember"
      @cancel="cancelRemoveMember"
    />

    <BaseDialog :show="importOpen" :title="t('enterpriseMembers.copy.importEnterpriseMembers')" width="extra-wide" @close="importOpen = false">
      <div class="space-y-5">
        <section class="rounded-2xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.04]">
          <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div class="flex min-w-0 items-start gap-3">
              <span class="flex h-10 w-10 flex-none items-center justify-center rounded-xl border border-emerald-200/80 bg-emerald-50 text-emerald-700 dark:border-emerald-400/20 dark:bg-emerald-400/10 dark:text-emerald-300">
                <Icon name="download" size="md" />
              </span>
              <div>
                <h3 class="text-sm font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.downloadImportTemplate') }}</h3>
                <p class="mt-1 max-w-4xl text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.importTemplateExternalFactsHint') }}</p>
              </div>
            </div>
            <div class="grid gap-2 sm:grid-cols-2 lg:flex-none">
              <button
                class="btn btn-secondary btn-sm justify-center"
                type="button"
                data-testid="download-enterprise-member-csv-template"
                :disabled="Boolean(templateDownloading)"
                @click="downloadTemplate('csv')"
              >
                <Icon :name="templateDownloading === 'csv' ? 'refresh' : 'download'" size="sm" :class="templateDownloading === 'csv' ? 'animate-spin' : ''" />
                {{ templateDownloading === 'csv' ? t('enterpriseMembers.copy.downloadingTemplate') : t('enterpriseMembers.copy.downloadCsvTemplate') }}
              </button>
              <button
                class="btn btn-secondary btn-sm justify-center"
                type="button"
                data-testid="download-enterprise-member-xlsx-template"
                :disabled="Boolean(templateDownloading)"
                @click="downloadTemplate('xlsx')"
              >
                <Icon :name="templateDownloading === 'xlsx' ? 'refresh' : 'download'" size="sm" :class="templateDownloading === 'xlsx' ? 'animate-spin' : ''" />
                {{ templateDownloading === 'xlsx' ? t('enterpriseMembers.copy.downloadingTemplate') : t('enterpriseMembers.copy.downloadXlsxTemplate') }}
              </button>
            </div>
          </div>
          <label class="mt-4 block rounded-2xl border border-dashed border-stone-300 p-5 text-center hover:border-amber-400 dark:border-white/15"><span class="block text-sm font-medium text-stone-800 dark:text-stone-100">{{ importFile?.name || t('enterpriseMembers.copy.chooseACsvOrXlsxFile') }}</span><span class="mt-1 block text-xs text-stone-500">{{ t('enterpriseMembers.copy.maximum10MibAnd5000RowsXlsxFormulasMacrosExternalLinksAndEmbeddedObjectsAreRejected') }}</span><input class="sr-only" type="file" accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" @change="selectImportFile" /></label>
          <div class="mt-3 flex justify-end"><button class="btn btn-primary" type="button" :disabled="!importFile || importPreviewLoading" @click="previewImportFile">{{ importPreviewLoading ? t('enterpriseMembers.copy.validatingOnServer') : t('enterpriseMembers.copy.generateAuthoritativePreview') }}</button></div>
        </section>

        <template v-if="importPreview">
          <section class="flex flex-wrap items-center justify-between gap-3 rounded-2xl border px-4 py-3" :class="importPreview.invalid_rows ? 'border-amber-200 bg-amber-50 dark:border-amber-900/50 dark:bg-amber-950/20' : 'border-emerald-200 bg-emerald-50 dark:border-emerald-900/50 dark:bg-emerald-950/20'">
            <div><p class="text-sm font-semibold text-stone-900 dark:text-white">{{ t('enterpriseMembers.dynamic.importValidationSummary', { valid: importPreview.valid_rows, invalid: importPreview.invalid_rows }) }}</p><p class="mt-1 text-xs text-stone-500">SHA-256 {{ importPreview.file_hash.slice(0, 16) }}… · {{ t('enterpriseMembers.copy.expires') }} {{ formatDateTime(importPreview.expires_at) }}</p></div>
            <button class="btn btn-secondary btn-sm" type="button" @click="toggleAllImportRows">{{ allValidImportRowsSelected ? t('enterpriseMembers.copy.clearSelection') : t('enterpriseMembers.copy.selectAllValid') }}</button>
          </section>
          <div class="max-h-[420px] overflow-auto rounded-2xl border border-stone-200 dark:border-white/10">
            <table class="w-full min-w-[1120px] text-left text-xs">
              <thead class="sticky top-0 z-10 bg-white text-stone-500 dark:bg-neutral-950"><tr><th class="p-3"></th><th>{{ t('enterpriseMembers.copy.row') }}</th><th>{{ t('enterpriseMembers.copy.member') }}</th><th>{{ t('enterpriseMembers.copy.budgetOpening') }}</th><th>{{ t('enterpriseMembers.copy.memberSpendingLimits') }}</th><th>{{ t('enterpriseMembers.copy.key') }}</th><th>{{ t('enterpriseMembers.copy.migrationUsage') }}</th><th>{{ t('enterpriseMembers.copy.validation') }}</th></tr></thead>
              <tbody class="divide-y divide-stone-100 dark:divide-white/10">
                <tr v-for="row in importPreview.rows" :key="row.row_number" :class="row.valid ? '' : 'bg-rose-50/60 dark:bg-rose-950/10'">
                  <td class="p-3"><button type="button" role="checkbox" :aria-checked="importSelectedRows.has(row.row_number)" :disabled="!row.valid" :class="selectionCheckboxClasses(importSelectedRows.has(row.row_number))" @click="toggleImportRow(row.row_number)"><Icon v-if="importSelectedRows.has(row.row_number)" name="check" size="xs" :stroke-width="2.5" /></button></td>
                  <td>{{ row.row_number }}</td>
                  <td><b class="block text-stone-900 dark:text-white">{{ row.member_name }}</b><code class="text-[11px] text-stone-500">{{ row.member_code }}</code></td>
                  <td>{{ formatMoney(row.monthly_limit_usd) }}<span v-if="row.opening_used_usd" class="block text-amber-700 dark:text-amber-300">+{{ formatMoney(row.opening_used_usd) }}</span></td>
                  <td class="whitespace-nowrap">5h {{ formatMoney(row.rate_limit_5h) }} · 1d {{ formatMoney(row.rate_limit_1d) }} · 7d {{ formatMoney(row.rate_limit_7d) }}</td>
                  <td><span v-if="row.key_present">{{ row.key_name }} · {{ t('enterpriseMembers.copy.plaintextEncrypted') }}</span><span v-else class="text-stone-400">—</span></td>
                  <td class="whitespace-nowrap"><b class="block text-stone-800 dark:text-stone-100">{{ formatNumber(row.total_tokens) }} Token</b><span class="text-stone-500">↓ {{ formatNumber(row.input_tokens) }} · ↑ {{ formatNumber(row.output_tokens) }} · C {{ formatNumber(row.cache_tokens) }}</span></td>
                  <td><span v-if="row.valid" class="font-medium text-emerald-600">{{ t('enterpriseMembers.copy.ready') }}</span><div v-else class="space-y-1 text-rose-600"><p v-for="error in row.errors" :key="error">{{ importIssueLabel(error) }}</p></div><p v-for="warning in row.warnings" :key="warning" class="text-amber-700">{{ importIssueLabel(warning) }}</p></td>
                </tr>
              </tbody>
            </table>
          </div>
          <section class="rounded-2xl border border-stone-200 p-4 dark:border-white/10">
            <div class="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between"><div><h3 class="text-sm font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.importAccessPolicy') }}</h3><p class="mt-1 max-w-3xl text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.importAccessPolicyHint') }}</p></div><span class="rounded-full bg-stone-100 px-3 py-1 text-xs font-medium text-stone-600 dark:bg-white/10 dark:text-stone-300">{{ t('enterpriseMembers.dynamic.importPeriod', { date: importPreview.period_start.slice(0, 10), timezone: importPreview.timezone }) }}</span></div>
            <div class="mt-4 max-h-64 space-y-2 overflow-y-auto rounded-2xl bg-stone-50 p-2 dark:bg-white/[0.035]">
              <div v-for="group in availableGroups" :key="group.id" class="flex items-center gap-3 rounded-xl px-3 py-2.5 hover:bg-white dark:hover:bg-white/5">
                <button type="button" role="checkbox" :aria-checked="importDefaultGroupIds.includes(group.id)" :aria-label="group.name" :class="selectionCheckboxClasses(importDefaultGroupIds.includes(group.id))" @click="toggleImportDefaultGroup(group.id)"><Icon v-if="importDefaultGroupIds.includes(group.id)" name="check" size="xs" :stroke-width="2.5" /></button>
                <span class="min-w-0 flex-1"><b class="block truncate text-sm text-stone-900 dark:text-white">{{ group.name }}</b><span class="text-xs text-stone-500">{{ group.platform }}</span></span>
                <template v-if="importDefaultGroupIds.includes(group.id)"><span class="rounded-lg bg-amber-100 px-2 py-1 text-xs font-bold text-amber-800 dark:bg-amber-300/10 dark:text-amber-200">#{{ importDefaultGroupIds.indexOf(group.id) + 1 }}</span><button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveUp')" @click.prevent="moveImportDefaultGroup(group.id, -1)">↑</button><button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveDown')" @click.prevent="moveImportDefaultGroup(group.id, 1)">↓</button></template>
              </div>
            </div>
            <button type="button" role="checkbox" :aria-checked="importActivateMembers" :disabled="!importDefaultGroupIds.length" class="mt-4 flex w-full items-start gap-3 rounded-xl border border-stone-200 p-3 text-left disabled:cursor-not-allowed disabled:opacity-60 dark:border-white/10" @click="importActivateMembers = !importActivateMembers"><span :class="selectionCheckboxClasses(importActivateMembers)"><Icon v-if="importActivateMembers" name="check" size="xs" :stroke-width="2.5" /></span><span><b class="block text-sm text-stone-900 dark:text-white">{{ t('enterpriseMembers.copy.activateImportedMembers') }}</b><span class="mt-1 block text-xs leading-5 text-stone-500">{{ importDefaultGroupIds.length ? t('enterpriseMembers.copy.activateImportedMembersHint') : t('enterpriseMembers.copy.importWithoutGroupsCreatesPendingMembers') }}</span></span></button>
          </section>
          <div class="flex flex-wrap items-center justify-between gap-3"><p class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.importCommitAtomicHint') }}</p><button class="btn btn-primary" type="button" :disabled="!importSelectedRows.size || importCommitting" @click="commitImportRows">{{ importCommitting ? t('enterpriseMembers.copy.processingInBackground') : t('enterpriseMembers.dynamic.commitRows', { count: importSelectedRows.size }) }}</button></div>
        </template>

        <section v-if="importJob && importJob.status !== 'completed'" class="rounded-2xl border p-4" :class="importJob.status === 'failed' ? 'border-rose-200 bg-rose-50 dark:border-rose-900/50 dark:bg-rose-950/20' : 'border-sky-200 bg-sky-50 dark:border-sky-900/50 dark:bg-sky-950/20'">
          <div class="flex flex-wrap items-center justify-between gap-3"><div><h3 class="font-semibold" :class="importJob.status === 'failed' ? 'text-rose-900 dark:text-rose-100' : 'text-sky-900 dark:text-sky-100'">{{ importJob.status === 'failed' ? t('enterpriseMembers.copy.importTransactionRolledBack') : t('enterpriseMembers.copy.importJobIsDurablyQueued') }}</h3><p class="mt-1 text-xs opacity-70">#{{ importJob.id }} · {{ importJobStatusLabel(importJob.status) }} · {{ t('enterpriseMembers.copy.attempt') }} {{ importJob.attempt_count }}</p><p v-if="importJob.error_summary" class="mt-2 text-xs">{{ importJob.error_summary }}</p></div><button v-if="importJob.status === 'failed'" class="btn btn-secondary btn-sm" type="button" @click="downloadImportErrors(importJob.id)">{{ t('enterpriseMembers.copy.downloadErrorReport') }}</button></div>
        </section>

        <section v-if="importResult" class="rounded-2xl border border-emerald-200 bg-emerald-50 p-4 dark:border-emerald-900/50 dark:bg-emerald-950/20">
          <div class="flex flex-wrap items-start justify-between gap-3"><div><h3 class="font-semibold text-emerald-900 dark:text-emerald-100">{{ t('enterpriseMembers.dynamic.importCreatedSummary', { members: importResult.created_members, keys: importResult.created_keys }) }}</h3><p class="mt-1 text-xs text-emerald-800/70 dark:text-emerald-200/70">{{ t('enterpriseMembers.dynamic.importBaselineSummary', { amount: formatMoney(importResult.migration_billed_usd), tokens: formatNumber(importResult.migration_total_tokens) }) }}</p><p v-if="importResult.period_start && importResult.timezone" class="mt-1 text-xs text-emerald-800/70 dark:text-emerald-200/70">{{ t('enterpriseMembers.dynamic.importPeriod', { date: importResult.period_start.slice(0, 10), timezone: importResult.timezone }) }}</p><p v-if="importResult.pending_members" class="mt-1 text-xs font-medium text-amber-800 dark:text-amber-200">{{ t('enterpriseMembers.dynamic.importPendingSummary', { count: importResult.pending_members }) }}</p></div><button v-if="importResult.pending_members" class="btn btn-secondary btn-sm" type="button" @click="configureImportedMembers">{{ t('enterpriseMembers.copy.configureImportedMembers') }}</button></div>
          <p v-if="importResult.keys.length" class="mt-3 text-xs text-emerald-800/70 dark:text-emerald-200/70">{{ t('enterpriseMembers.copy.plaintextBelowIsShownOnlyInThisSuccessfulResponseSaveItNow') }}</p>
          <div v-if="importResult.keys.length" class="mt-3 max-h-56 space-y-2 overflow-auto"><div v-for="key in importResult.keys" :key="`${key.member_code}:${key.key_name}`" class="rounded-xl bg-stone-950 p-3 text-xs text-white"><span class="text-stone-400">{{ key.member_code }} · {{ key.key_name }}</span><code class="mt-1 block break-all text-amber-200">{{ key.key }}</code></div></div>
        </section>
      </div>
    </BaseDialog>

    <BaseDialog :show="editorOpen" :title="editingMember ? t('enterpriseMembers.copy.editEnterpriseMember') : t('enterpriseMembers.copy.createEnterpriseMember')" width="extra-wide" @close="editorOpen = false">
      <form id="enterprise-member-form" class="space-y-5" @submit.prevent="saveMember">
        <div class="grid gap-4 sm:grid-cols-2">
          <label><span class="input-label">{{ t('enterpriseMembers.copy.memberIdentifier') }}</span><input v-model.trim="draft.member_code" class="input font-mono disabled:cursor-not-allowed disabled:bg-stone-100 disabled:text-stone-500 dark:disabled:bg-white/5" :disabled="Boolean(editingMember)" required maxlength="100" pattern="[A-Za-z0-9._-]+" placeholder="finance.ops-01" /><span class="input-hint">{{ editingMember ? t('enterpriseMembers.copy.memberIdentifierImmutableHint') : t('enterpriseMembers.copy.memberIdentifierCreateHint') }}</span></label>
          <label><span class="input-label">{{ t('enterpriseMembers.copy.displayName') }}</span><input v-model.trim="draft.name" class="input" required maxlength="100" /></label>
        </div>
        <section class="rounded-2xl border border-stone-200 p-4 dark:border-white/10">
          <div><h3 class="text-sm font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.memberSpendingLimits') }}</h3><p class="mt-1 text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.memberSpendingLimitsHint') }}</p></div>
          <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <div class="limit-field"><label><span class="input-label">5h {{ t('enterpriseMembers.copy.limit') }}</span><input v-model.number="draft.rate_limit_5h" class="input" type="number" min="0" step="0.01" /></label><label class="mt-3 block"><span class="input-label">{{ editingMember ? t('enterpriseMembers.copy.used') : t('enterpriseMembers.copy.initialUsed') }}</span><input v-model.number="editorUsage5h" class="input" type="number" min="0" step="0.00000001" :disabled="editorBudgetLoading" /></label></div>
            <div class="limit-field"><label><span class="input-label">1d {{ t('enterpriseMembers.copy.limit') }}</span><input v-model.number="draft.rate_limit_1d" class="input" type="number" min="0" step="0.01" /></label><label class="mt-3 block"><span class="input-label">{{ editingMember ? t('enterpriseMembers.copy.used') : t('enterpriseMembers.copy.initialUsed') }}</span><input v-model.number="editorUsage1d" class="input" type="number" min="0" step="0.00000001" :disabled="editorBudgetLoading" /></label></div>
            <div class="limit-field"><label><span class="input-label">7d {{ t('enterpriseMembers.copy.limit') }}</span><input v-model.number="draft.rate_limit_7d" class="input" type="number" min="0" step="0.01" /></label><label class="mt-3 block"><span class="input-label">{{ editingMember ? t('enterpriseMembers.copy.used') : t('enterpriseMembers.copy.initialUsed') }}</span><input v-model.number="editorUsage7d" class="input" type="number" min="0" step="0.00000001" :disabled="editorBudgetLoading" /></label></div>
            <div class="limit-field"><label><span class="input-label">{{ t('enterpriseMembers.copy.calendarMonthBudgetUsd') }}</span><input v-model.number="draft.monthly_limit_usd" class="input" type="number" min="0" step="0.01" /></label><label class="mt-3 block"><span class="input-label">{{ editingMember ? t('enterpriseMembers.copy.used') : t('enterpriseMembers.copy.initialUsed') }}</span><input v-model.number="editorMonthlyUsed" class="input" type="number" min="0" step="0.00000001" :disabled="editorBudgetLoading" /></label></div>
          </div>
          <p class="mt-3 text-xs text-stone-500">{{ t('enterpriseMembers.copy.zeroMeansUnlimitedAllMemberKeysShareTheseLimits') }}</p>
        </section>
        <fieldset>
          <legend class="input-label">{{ t('enterpriseMembers.copy.memberAccessibleGroups') }}</legend>
          <p class="mb-3 text-xs text-stone-500">{{ t('enterpriseMembers.copy.memberAccessibleGroupsHint') }}</p>
          <div class="max-h-72 space-y-2 overflow-y-auto rounded-2xl border border-stone-200 p-2 dark:border-white/10">
            <div v-for="group in availableGroups" :key="group.id" class="flex items-center gap-3 rounded-xl px-3 py-2.5 hover:bg-stone-50 dark:hover:bg-white/5">
              <button type="button" role="checkbox" :aria-checked="draft.group_ids.includes(group.id)" :aria-label="group.name" :class="selectionCheckboxClasses(draft.group_ids.includes(group.id))" @click="toggleDraftGroup(group.id)"><Icon v-if="draft.group_ids.includes(group.id)" name="check" size="xs" :stroke-width="2.5" /></button>
              <span class="min-w-0 flex-1"><b class="block truncate text-sm text-stone-900 dark:text-white">{{ group.name }}</b><span class="text-xs text-stone-500">{{ group.platform }}</span></span>
              <template v-if="draft.group_ids.includes(group.id)">
                <span class="rounded-lg bg-amber-100 px-2 py-1 text-xs font-bold text-amber-800 dark:bg-amber-300/10 dark:text-amber-200">#{{ draft.group_ids.indexOf(group.id) + 1 }}</span>
                <button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveUp')" @click.prevent="moveDraftGroup(group.id, -1)">↑</button>
                <button type="button" class="rounded-lg p-1 hover:bg-stone-200 dark:hover:bg-white/10" :aria-label="t('enterpriseMembers.copy.moveDown')" @click.prevent="moveDraftGroup(group.id, 1)">↓</button>
              </template>
            </div>
          </div>
        </fieldset>
      </form>
      <template #footer><div class="flex justify-end gap-3"><button class="btn btn-secondary" type="button" @click="editorOpen = false">{{ t('enterpriseMembers.copy.cancel') }}</button><button class="btn btn-primary" form="enterprise-member-form" type="submit" :disabled="saving">{{ saving ? t('enterpriseMembers.copy.saving') : t('enterpriseMembers.copy.saveMember') }}</button></div></template>
    </BaseDialog>

    <BaseDialog :show="keysOpen" :title="t('enterpriseMembers.dynamic.memberKeysTitle', { name: keyMember?.name || '' })" width="extra-wide" @close="keysOpen = false">
      <div class="space-y-4">
        <section class="flex flex-col gap-3 rounded-2xl border border-stone-200 bg-stone-50/70 px-4 py-3 dark:border-white/10 dark:bg-white/[0.025] sm:flex-row sm:items-center sm:justify-between">
          <div class="flex min-w-0 items-start gap-3">
            <span class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-emerald-100 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300"><Icon name="key" size="sm" /></span>
            <p class="text-xs leading-5 text-stone-600 dark:text-stone-300">{{ t('enterpriseMembers.copy.normalKeysRemainIndependent') }}</p>
          </div>
          <button class="btn btn-secondary btn-sm shrink-0" type="button" @click="openRegularKeys">{{ t('enterpriseMembers.copy.manageRegularKeys') }}</button>
        </section>
        <section v-if="keyMember?.deleted_at" class="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-amber-950 dark:border-amber-800/50 dark:bg-amber-950/20 dark:text-amber-100">
          <Icon name="lock" size="sm" class="mt-0.5 shrink-0" />
          <p class="text-xs leading-5">{{ t('enterpriseMembers.copy.archivedMemberReadOnly') }}</p>
        </section>
        <form v-if="!keyMember?.deleted_at" class="grid gap-3 rounded-2xl bg-stone-50 p-4 dark:bg-white/[0.04] sm:grid-cols-[1fr_150px_auto]" @submit.prevent="createMemberKey">
          <label><span class="input-label">{{ t('enterpriseMembers.copy.keyName') }}</span><input v-model.trim="keyDraft.name" class="input" required maxlength="100" /></label>
          <label><span class="input-label">{{ t('enterpriseMembers.copy.keyQuota') }}</span><input v-model.number="keyDraft.quota" class="input" type="number" min="0" step="0.01" /></label>
          <button class="btn btn-primary self-end" type="submit" :disabled="keySaving"><Icon name="plus" size="sm" />{{ t('enterpriseMembers.copy.createKey') }}</button>
        </form>
        <div v-if="keysLoading" class="py-12 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.loadingKeys') }}</div>
        <div v-else class="overflow-hidden rounded-2xl border border-stone-200 dark:border-white/10">
          <div v-if="memberKeys.length" class="overflow-x-auto">
            <div class="min-w-[960px]">
              <div class="grid grid-cols-[minmax(210px,1.25fr)_160px_105px_155px_250px] items-center gap-3 border-b border-stone-200 bg-stone-50/80 px-4 py-2.5 text-xs font-medium text-stone-500 dark:border-white/10 dark:bg-white/[0.035]">
                <span>{{ t('enterpriseMembers.copy.key') }}</span>
                <span>{{ t('enterpriseMembers.copy.keyQuota') }}</span>
                <span>{{ t('enterpriseMembers.copy.status') }}</span>
                <span>{{ t('enterpriseMembers.copy.updated') }}</span>
                <span class="pr-1 text-right">{{ t('enterpriseMembers.copy.action') }}</span>
              </div>
              <div v-for="key in memberKeys" :key="key.id" class="border-b border-stone-100 last:border-b-0 dark:border-white/[0.07]">
                <div class="grid min-h-16 grid-cols-[minmax(210px,1.25fr)_160px_105px_155px_250px] items-center gap-3 px-4 py-3">
                  <div class="min-w-0">
                    <p class="truncate text-sm font-semibold text-stone-900 dark:text-white" :title="key.name">{{ key.name }}</p>
                    <code class="mt-0.5 block truncate text-[11px] text-stone-500" :title="key.key">{{ key.key }}</code>
                  </div>
                  <div class="min-w-0">
                    <p class="whitespace-nowrap text-xs font-semibold tabular-nums text-stone-800 dark:text-stone-100">{{ key.quota > 0 ? `${formatMoney(key.quota_used)} / ${formatMoney(key.quota)}` : t('enterpriseMembers.copy.unlimited6381d248') }}</p>
                    <div v-if="key.quota > 0" class="mt-1.5 h-1 w-full overflow-hidden rounded-full bg-stone-200 dark:bg-white/10"><div class="h-full rounded-full bg-emerald-500" :style="{ width: `${memberKeyQuotaPercent(key)}%` }"></div></div>
                  </div>
                  <span class="inline-flex w-fit items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold" :class="memberKeyStatusBadgeClasses(key.status)"><span class="h-1.5 w-1.5 rounded-full bg-current"></span>{{ memberKeyStatusLabel(key.status) }}</span>
                  <div class="text-xs text-stone-500">
                    <time class="block whitespace-nowrap">{{ formatDateTime(key.updated_at) }}</time>
                    <span class="mt-0.5 block whitespace-nowrap text-[11px] text-stone-400">{{ key.expires_at ? `${t('enterpriseMembers.copy.expiresAt')} ${formatDate(key.expires_at)}` : t('enterpriseMembers.copy.neverExpires') }}</span>
                  </div>
                  <div class="flex flex-nowrap justify-end gap-1.5 pr-1">
                    <button v-if="!keyMember?.deleted_at" class="btn btn-secondary btn-sm w-[76px] shrink-0 whitespace-nowrap disabled:cursor-wait disabled:opacity-100" type="button" :disabled="copyingMemberKeyId === key.id" :aria-busy="copyingMemberKeyId === key.id" :aria-label="t('enterpriseMembers.copy.copyMemberKey')" @click="copyMemberKey(key)"><Icon :name="copyingMemberKeyId === key.id ? 'refresh' : copiedMemberKeyId === key.id ? 'check' : 'clipboard'" size="sm" :class="copyingMemberKeyId === key.id ? 'animate-spin' : copiedMemberKeyId === key.id ? 'text-emerald-500' : ''" />{{ t('enterpriseMembers.copy.copyMemberKey') }}</button>
                    <button v-if="!keyMember?.deleted_at" class="btn btn-secondary btn-sm shrink-0 whitespace-nowrap" type="button" @click="openKeyEdit(key)"><Icon name="edit" size="sm" />{{ t('enterpriseMembers.copy.edit') }}</button>
                    <button v-if="!keyMember?.deleted_at" class="inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-lg px-2.5 py-1.5 text-xs font-medium text-rose-600 transition-colors hover:bg-rose-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-rose-500/40 dark:text-rose-300 dark:hover:bg-rose-400/10" type="button" @click="removeKey(key.id)"><Icon name="trash" size="sm" />{{ t('enterpriseMembers.copy.delete') }}</button>
                  </div>
                </div>
                <form v-if="editingKey?.id === key.id && !keyMember?.deleted_at" class="space-y-4 border-t border-stone-200 bg-stone-50/80 p-4 dark:border-white/10 dark:bg-white/[0.025]" @submit.prevent="saveMemberKey">
                  <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                    <label><span class="input-label">{{ t('enterpriseMembers.copy.keyName') }}</span><input v-model.trim="keyEditDraft.name" class="input" required maxlength="100" /></label>
                    <label><span class="input-label">{{ t('enterpriseMembers.copy.status') }}</span><Select v-model="keyEditDraft.status" :options="memberKeyEditableStatusOptions" class="w-full" /></label>
                    <label><span class="input-label">{{ t('enterpriseMembers.copy.keyQuota') }}</span><input v-model.number="keyEditDraft.quota" class="input" type="number" min="0" step="0.00000001" /></label>
                    <label><span class="input-label">{{ t('enterpriseMembers.copy.expiresAt') }}</span><input v-model="keyEditDraft.expires_at" class="input" type="datetime-local" /></label>
                  </div>
                  <div class="grid gap-3 sm:grid-cols-3">
                    <label><span class="input-label">5h {{ t('enterpriseMembers.copy.limit') }}</span><input v-model.number="keyEditDraft.rate_limit_5h" class="input" type="number" min="0" step="0.00000001" /></label>
                    <label><span class="input-label">1d {{ t('enterpriseMembers.copy.limit') }}</span><input v-model.number="keyEditDraft.rate_limit_1d" class="input" type="number" min="0" step="0.00000001" /></label>
                    <label><span class="input-label">7d {{ t('enterpriseMembers.copy.limit') }}</span><input v-model.number="keyEditDraft.rate_limit_7d" class="input" type="number" min="0" step="0.00000001" /></label>
                  </div>
                  <div class="grid gap-3 sm:grid-cols-2">
                    <label><span class="input-label">{{ t('enterpriseMembers.copy.ipAllowlist') }}</span><textarea v-model="keyEditDraft.ip_whitelist" class="input" rows="3" :placeholder="t('enterpriseMembers.copy.onePerLineOrCommaSeparatedEmptyMeansUnrestricted')"></textarea></label>
                    <label><span class="input-label">{{ t('enterpriseMembers.copy.ipBlocklist') }}</span><textarea v-model="keyEditDraft.ip_blacklist" class="input" rows="3" :placeholder="t('enterpriseMembers.copy.onePerLineOrCommaSeparated')"></textarea></label>
                  </div>
                  <div class="flex justify-end gap-3"><button class="btn btn-secondary btn-sm" type="button" @click="editingKey = null">{{ t('enterpriseMembers.copy.cancel') }}</button><button class="btn btn-primary btn-sm" type="submit" :disabled="keyEditing">{{ keyEditing ? t('enterpriseMembers.copy.saving') : t('enterpriseMembers.copy.saveKey') }}</button></div>
                </form>
              </div>
            </div>
          </div>
          <p v-if="!memberKeys.length" class="p-8 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.noMemberKeysYet') }}</p>
        </div>
      </div>
    </BaseDialog>

    <BaseDialog :show="budgetOpen" :title="t('enterpriseMembers.dynamic.budgetUsageTitle', { name: budgetMember?.name || '' })" width="extra-wide" @close="closeBudget">
      <div v-if="budgetLoading" class="py-16 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.loadingBudgetAndUsage') }}</div>
      <div v-else-if="budgetSummary && budgetAnalytics" class="space-y-6">
        <section v-if="budgetMember?.deleted_at" class="flex items-start gap-3 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-amber-950 dark:border-amber-800/50 dark:bg-amber-950/20 dark:text-amber-100">
          <Icon name="lock" size="sm" class="mt-0.5 shrink-0" />
          <p class="text-xs leading-5">{{ t('enterpriseMembers.copy.archivedMemberReadOnly') }}</p>
        </section>
        <section class="overflow-hidden rounded-3xl border border-stone-200 bg-white dark:border-white/10 dark:bg-white/[0.02]">
          <header class="flex flex-wrap items-start justify-between gap-4 border-b border-stone-200 bg-stone-50/80 px-5 py-4 dark:border-white/10 dark:bg-white/[0.03]">
            <div class="flex items-start gap-3">
              <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300">
                <Icon name="chartBar" size="sm" />
              </span>
              <div>
                <h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.monthlyBudgetOverview') }}</h3>
                <p class="mt-1 text-xs text-stone-500">{{ formatDate(budgetSummary.period_start) }} – {{ formatDate(budgetSummary.period_end) }} · {{ budgetSummary.timezone }}</p>
              </div>
            </div>
          </header>

          <div class="p-5">
            <dl class="grid gap-3 md:grid-cols-3">
              <div class="rounded-2xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.04]">
                <dt class="text-xs font-medium text-stone-500">{{ t('enterpriseMembers.copy.monthlyBudget') }}</dt>
                <dd class="mt-2 text-2xl font-semibold tabular-nums tracking-tight text-stone-950 dark:text-white">
                  {{ budgetSummary.limit_usd > 0 ? formatMoney(budgetSummary.limit_usd) : t('enterpriseMembers.copy.monthlyLimitNotSet') }}
                </dd>
              </div>
              <div class="rounded-2xl border border-emerald-200 bg-emerald-50/70 p-4 dark:border-emerald-800/50 dark:bg-emerald-400/[0.08]">
                <dt class="text-xs font-medium text-emerald-800 dark:text-emerald-200">{{ t('enterpriseMembers.copy.usedThisMonth') }}</dt>
                <dd class="mt-2 text-2xl font-semibold tabular-nums tracking-tight text-stone-950 dark:text-white">{{ formatMoney(budgetSummary.used_usd) }}</dd>
                <p class="mt-2 text-xs leading-5 text-emerald-800/75 dark:text-emerald-200/70">{{ t('enterpriseMembers.copy.usedAmountHint') }}</p>
              </div>
              <div class="rounded-2xl border p-4" :class="budgetHasOverage ? 'border-rose-200 bg-rose-50/70 dark:border-rose-800/50 dark:bg-rose-400/[0.08]' : 'border-stone-200 bg-white dark:border-white/10 dark:bg-white/[0.025]'">
                <dt class="text-xs font-medium" :class="budgetHasOverage ? 'text-rose-700 dark:text-rose-200' : 'text-stone-500'">{{ t(budgetHasOverage ? 'enterpriseMembers.copy.budgetOverage' : 'enterpriseMembers.copy.availableBudget') }}</dt>
                <dd class="mt-2 text-2xl font-semibold tabular-nums tracking-tight" :class="budgetHasOverage ? 'text-rose-700 dark:text-rose-300' : 'text-emerald-700 dark:text-emerald-300'">
                  {{ budgetHasOverage ? formatMoney(budgetOverageUsd) : (budgetSummary.limit_usd > 0 ? formatMoney(Math.max(0, budgetSummary.remaining_usd)) : t('enterpriseMembers.copy.unlimited')) }}
                </dd>
              </div>
            </dl>

            <div v-if="budgetSummary.limit_usd > 0" class="mt-5">
              <div class="mb-2 flex items-center justify-between gap-3 text-xs">
                <span class="font-medium text-stone-600 dark:text-stone-300">{{ t('enterpriseMembers.copy.budgetUsageProgress') }}</span>
                <span class="tabular-nums text-stone-500">{{ budgetUsagePercentLabel }}</span>
              </div>
              <div class="flex h-2 overflow-hidden rounded-full bg-stone-100 dark:bg-white/5" role="progressbar" :aria-valuenow="budgetUsagePercent" aria-valuemin="0" aria-valuemax="100">
                <div class="h-full bg-emerald-500 transition-all" :style="{ width: `${budgetSettledPercent}%` }"></div>
              </div>
            </div>

            <div v-if="budgetExhausted" class="mt-5 flex items-start gap-3 rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-rose-950 dark:border-rose-800/50 dark:bg-rose-950/20 dark:text-rose-100">
              <Icon name="lock" size="sm" class="mt-0.5 shrink-0" />
              <p class="text-xs leading-5"><b v-if="budgetHasOverage">{{ t('enterpriseMembers.copy.budgetOverage') }} {{ formatMoney(budgetOverageUsd) }}</b><span :class="budgetHasOverage ? 'ml-1' : ''">{{ t('enterpriseMembers.copy.budgetRequestsStopped') }}</span></p>
            </div>

            <details class="mt-5 overflow-hidden rounded-2xl border border-stone-200 bg-stone-50/70 dark:border-white/10 dark:bg-white/[0.025]">
              <summary class="cursor-pointer px-4 py-3 text-sm font-medium text-stone-800 dark:text-stone-100">
                {{ t('enterpriseMembers.copy.monthlyUsageDetails') }}
                <span class="ml-2 text-xs font-normal text-stone-500">{{ formatNumber(budgetSummary.request_count) }} {{ t('enterpriseMembers.copy.requests') }} · {{ formatNumber(budgetSummary.input_tokens + budgetSummary.output_tokens) }} Token</span>
              </summary>
              <div class="border-t border-stone-200 p-4 dark:border-white/10">
                <dl class="grid gap-3 sm:grid-cols-2">
                  <div><dt class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.monthlyRequestCount') }}</dt><dd class="mt-1 text-lg font-semibold tabular-nums text-stone-950 dark:text-white">{{ formatNumber(budgetSummary.request_count) }}</dd></div>
                  <div><dt class="text-xs text-stone-500">{{ t('enterpriseMembers.copy.monthlyTokenUsage') }}</dt><dd class="mt-1 text-lg font-semibold tabular-nums text-stone-950 dark:text-white">{{ formatNumber(budgetSummary.input_tokens + budgetSummary.output_tokens) }}</dd></div>
                </dl>
                <div v-if="hasMigrationBaseline(budgetSummary)" class="mt-4 rounded-xl border border-sky-200 bg-sky-50 p-3 text-sky-950 dark:border-sky-800/50 dark:bg-sky-950/20 dark:text-sky-100">
                  <p class="text-xs font-semibold">{{ t('enterpriseMembers.copy.importedUsageRecord') }}</p>
                  <p class="mt-1 text-xs leading-5 opacity-75">{{ t('enterpriseMembers.copy.importedUsageRecordHint') }}</p>
                  <div class="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-xs">
                    <span><b>{{ formatMoney(budgetSummary.migration_billed_usd) }}</b></span>
                    <span>{{ t('enterpriseMembers.copy.inputTokens') }} {{ formatNumber(budgetSummary.migration_input_tokens) }}</span>
                    <span>{{ t('enterpriseMembers.copy.outputTokens') }} {{ formatNumber(budgetSummary.migration_output_tokens) }}</span>
                    <span>{{ t('enterpriseMembers.copy.cacheTokens') }} {{ formatNumber(budgetSummary.migration_cache_tokens) }}</span>
                  </div>
                </div>
              </div>
            </details>
          </div>
        </section>

        <section v-if="hasMemberRateLimits">
          <div class="mb-3">
            <h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.shortWindowLimits') }}</h3>
            <p class="mt-1 text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.shortWindowLimitsHint') }}</p>
          </div>
          <div class="grid gap-3 sm:grid-cols-3">
            <article v-for="window in memberRateLimitWindows" :key="window.key" class="rounded-2xl border border-stone-200 bg-stone-50 p-4 dark:border-white/10 dark:bg-white/[0.04]">
              <div class="flex min-h-9 items-start justify-between gap-3">
                <h4 class="text-sm font-semibold text-stone-900 dark:text-white">{{ window.label }}</h4>
                <span v-if="window.resetAt" class="text-right text-[10px] leading-4 text-stone-400">{{ t('enterpriseMembers.copy.expires') }}<br>{{ formatDateTime(window.resetAt) }}</span>
              </div>
              <p class="mt-4 text-xs text-stone-500">{{ t('enterpriseMembers.copy.used') }}</p>
              <strong class="mt-1 block text-2xl font-semibold tabular-nums text-stone-950 dark:text-white">{{ formatMoney(window.used) }}</strong>
              <div class="mt-4 flex items-center justify-between gap-3 border-t border-stone-200 pt-3 text-xs dark:border-white/10">
                <span class="text-stone-500">{{ t('enterpriseMembers.copy.limit') }}</span>
                <b v-if="window.limit > 0" class="tabular-nums text-stone-800 dark:text-stone-100">{{ formatMoney(window.limit) }}</b>
                <span v-else class="rounded-full bg-stone-200/70 px-2 py-1 font-medium text-stone-500 dark:bg-white/5 dark:text-stone-400">{{ t('enterpriseMembers.copy.limitNotSet') }}</span>
              </div>
              <div v-if="window.limit > 0" class="mt-3 h-1.5 overflow-hidden rounded-full bg-stone-200 dark:bg-white/10"><div class="h-full rounded-full bg-emerald-500" :style="{ width: `${Math.min(100, (window.used / window.limit) * 100)}%` }"></div></div>
            </article>
          </div>
        </section>

        <section class="grid gap-5 xl:grid-cols-[1.4fr_1fr]">
          <div class="rounded-3xl border border-stone-200 p-5 dark:border-white/10">
            <div class="flex items-center justify-between gap-3"><div><h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.usageTrend') }}</h3><p class="mt-1 text-xs text-stone-500">{{ t('enterpriseMembers.copy.showsMemberFacingCostOnlyUpstreamAccountsAndChannelCostStayPrivate') }}</p></div><select v-model.number="analyticsDays" class="input w-28" @change="reloadAnalytics"><option :value="7">7d</option><option :value="30">30d</option><option :value="90">90d</option><option :value="365">365d</option></select></div>
            <div v-if="budgetAnalytics.trend.length" class="mt-5 overflow-x-auto pb-1">
              <div :style="{ width: `max(100%, ${normalizedBudgetTrend.length * 5}px)` }">
                <div class="grid h-44 w-full items-end gap-px border-b border-stone-200 dark:border-white/10" :style="{ gridTemplateColumns: `repeat(${normalizedBudgetTrend.length}, minmax(3px, 1fr))` }" role="img" :aria-label="t('enterpriseMembers.copy.dailyCostBarChart')">
                  <div v-for="point in normalizedBudgetTrend" :key="point.date" class="group relative flex h-full min-w-0 items-end justify-center pt-8" :title="`${point.date} · ${formatMoney(point.actual_cost)} · ${formatNumber(point.request_count)} ${t('enterpriseMembers.copy.requests')}`">
                    <div v-if="point.actual_cost > 0" class="relative w-full max-w-6 rounded-t-sm bg-amber-300/80 transition-colors hover:bg-amber-400 dark:bg-amber-400/70 dark:hover:bg-amber-300" :style="{ height: `${trendHeight(point.actual_cost)}%` }">
                      <span class="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 hidden -translate-x-1/2 whitespace-nowrap rounded-lg bg-stone-950 px-2 py-1 text-[10px] text-white shadow-lg group-hover:block">{{ point.date }} · {{ formatMoney(point.actual_cost) }} · {{ formatNumber(point.request_count) }} {{ t('enterpriseMembers.copy.requests') }}</span>
                    </div>
                  </div>
                </div>
                <div class="mt-2 flex justify-between text-[10px] tabular-nums text-stone-400"><span>{{ normalizedBudgetTrend[0]?.date }}</span><span>{{ normalizedBudgetTrend[normalizedBudgetTrend.length - 1]?.date }}</span></div>
              </div>
            </div>
            <p v-else class="mt-5 rounded-2xl bg-stone-50 py-16 text-center text-sm text-stone-500 dark:bg-white/[0.03]">{{ t('enterpriseMembers.copy.noUsageInThisRange') }}</p>
          </div>
          <div class="rounded-3xl border border-stone-200 p-5 dark:border-white/10">
            <h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.groupCostBreakdown') }}</h3>
            <div class="mt-4 space-y-3"><div v-for="item in budgetAnalytics.groups.slice(0, 8)" :key="item.key" class="flex items-center gap-3"><div class="min-w-0 flex-1"><p class="truncate text-sm font-medium text-stone-800 dark:text-stone-100">{{ item.name }}</p><p class="text-xs text-stone-500">{{ formatNumber(item.request_count) }} {{ t('enterpriseMembers.copy.requests') }} · {{ formatNumber(item.input_tokens + item.output_tokens) }} Token</p></div><b class="text-sm text-stone-950 dark:text-white">{{ formatMoney(item.actual_cost) }}</b></div><p v-if="!budgetAnalytics.groups.length" class="py-10 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.noGroupData') }}</p></div>
          </div>
        </section>

        <section class="rounded-3xl border border-stone-200 p-5 dark:border-white/10">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div><h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.requestRecords') }}</h3><p class="mt-1 text-xs text-stone-500">{{ t('enterpriseMembers.copy.showsMemberFacingKeyModelPublicGroupAndBilledCostOnlyUpstreamAccountChannelAndMarginDataRemainPr') }}</p></div>
            <div class="flex flex-wrap items-center justify-end gap-2">
              <button class="btn btn-secondary btn-sm" type="button" @click="openUnifiedUsage(budgetMember)">
                <Icon name="trendingUp" size="sm" />
                {{ t('enterpriseMembers.copy.viewFullUsageRecords') }}
              </button>
              <span class="text-xs text-stone-500">{{ usageRecordTotal }} {{ t('enterpriseMembers.copy.records') }}</span>
              <button class="btn btn-secondary btn-sm" type="button" :aria-label="t('enterpriseMembers.copy.previousRequestRecordsPage')" :disabled="usageRecordsLoading || usageRecordPage <= 1" @click="loadUsageRecords(usageRecordPage - 1)">←</button>
              <span class="min-w-14 text-center text-xs text-stone-500">{{ usageRecordPage }} / {{ usageRecordPages }}</span>
              <button class="btn btn-secondary btn-sm" type="button" :aria-label="t('enterpriseMembers.copy.nextRequestRecordsPage')" :disabled="usageRecordsLoading || usageRecordPage >= usageRecordPages" @click="loadUsageRecords(usageRecordPage + 1)">→</button>
            </div>
          </div>
          <div class="mt-4 overflow-hidden rounded-2xl border border-stone-200 dark:border-white/10">
            <UsageTable
              flat
              :data="usageRecords"
              :loading="usageRecordsLoading"
              :columns="budgetUsageRecordColumns"
              :server-side-sort="true"
              :show-account-billing="false"
              :show-upstream-endpoint="false"
              default-sort-key="created_at"
              default-sort-order="desc"
              @sort="handleUsageRecordSort"
              @ipGeoBatchFailed="handleUsageIpGeoBatchFailed"
            />
          </div>
        </section>

        <section class="grid gap-5 xl:grid-cols-[1fr_1.4fr]">
          <div class="rounded-3xl border border-stone-200 p-5 dark:border-white/10">
            <h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.modelCostRanking') }}</h3>
            <div class="mt-4 space-y-3"><div v-for="item in budgetAnalytics.models.slice(0, 10)" :key="item.key" class="flex items-center justify-between gap-3"><div class="min-w-0"><p class="truncate font-mono text-xs text-stone-800 dark:text-stone-100">{{ item.name }}</p><p class="text-[11px] text-stone-500">{{ formatNumber(item.request_count) }} {{ t('enterpriseMembers.copy.requests') }}</p></div><b class="text-sm">{{ formatMoney(item.actual_cost) }}</b></div><p v-if="!budgetAnalytics.models.length" class="py-10 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.noModelData') }}</p></div>
          </div>
          <div class="rounded-3xl border border-stone-200 p-5 dark:border-white/10">
            <div class="flex items-center justify-between"><div><h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.budgetLedger') }}</h3><p class="mt-1 text-xs text-stone-500">{{ t('enterpriseMembers.copy.usageAdjustmentsAndReconciliationAreImmutableEvidence') }}</p></div><span class="text-xs text-stone-500">{{ budgetEntryTotal }} {{ t('enterpriseMembers.copy.entries') }}</span></div>
            <div class="mt-4 max-h-72 overflow-auto"><table class="w-full min-w-[520px] text-left text-xs"><thead class="sticky top-0 bg-white text-stone-500 dark:bg-neutral-950"><tr><th class="py-2">{{ t('enterpriseMembers.copy.time') }}</th><th>{{ t('enterpriseMembers.copy.type') }}</th><th>{{ t('enterpriseMembers.copy.amount') }}</th><th>{{ t('enterpriseMembers.copy.note') }}</th></tr></thead><tbody class="divide-y divide-stone-100 dark:divide-white/10"><tr v-for="entry in budgetEntries" :key="entry.id"><td class="py-2.5 pr-3 whitespace-nowrap">{{ formatDateTime(entry.created_at) }}</td><td class="pr-3">{{ entryKindLabel(entry.kind) }}</td><td class="pr-3 font-semibold" :class="entry.amount_usd < 0 ? 'text-emerald-600' : ''">{{ formatMoney(entry.amount_usd) }}</td><td class="max-w-xs truncate text-stone-500">{{ entry.note }}</td></tr></tbody></table><p v-if="!budgetEntries.length" class="py-10 text-center text-sm text-stone-500">{{ t('enterpriseMembers.copy.noLedgerEntriesThisMonth') }}</p></div>
          </div>
        </section>

        <section class="rounded-3xl border border-stone-200 p-5 dark:border-white/10">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <h3 class="font-semibold text-stone-950 dark:text-white">{{ t('enterpriseMembers.copy.activityAudit') }}</h3>
              <p class="mt-1 text-xs text-stone-500">{{ t('enterpriseMembers.copy.memberGroupKeyAndAdjustmentChangesAreCommittedAtomicallySecretKeyMaterialNeverEntersTheAuditTrai') }}</p>
            </div>
            <span class="rounded-full bg-stone-100 px-3 py-1 text-xs font-medium text-stone-600 dark:bg-white/5 dark:text-stone-300">{{ auditEventTotal }} {{ t('enterpriseMembers.copy.events27ddb7d1') }}</span>
          </div>
          <div v-if="auditEvents.length" class="mt-5 max-h-80 space-y-1 overflow-auto pr-1">
            <article v-for="event in auditEvents" :key="event.id" class="group grid grid-cols-[18px_minmax(0,1fr)] gap-3 rounded-2xl px-2 py-3 transition-colors hover:bg-stone-50 dark:hover:bg-white/[0.03]">
              <div class="relative flex justify-center"><span class="mt-1.5 h-2.5 w-2.5 rounded-full border-2 border-white bg-amber-500 ring-1 ring-amber-300 dark:border-neutral-950 dark:ring-amber-700"></span><span class="absolute bottom-[-14px] top-5 w-px bg-stone-200 group-last:hidden dark:bg-white/10"></span></div>
              <div class="min-w-0">
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div class="flex min-w-0 items-center gap-2"><strong class="truncate text-sm text-stone-900 dark:text-white">{{ auditActionLabel(event.action) }}</strong><span class="rounded-md bg-stone-100 px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-wide text-stone-500 dark:bg-white/5">{{ auditEntityLabel(event.entity_type) }}<template v-if="event.entity_id"> #{{ event.entity_id }}</template></span></div>
                  <time class="whitespace-nowrap text-[11px] text-stone-400">{{ formatDateTime(event.created_at) }}</time>
                </div>
                <p class="mt-1.5 text-xs leading-5 text-stone-500 dark:text-stone-400">{{ auditEventSummary(event) }}</p>
              </div>
            </article>
          </div>
          <p v-else class="mt-5 rounded-2xl bg-stone-50 py-10 text-center text-sm text-stone-500 dark:bg-white/[0.03]">{{ t('enterpriseMembers.copy.noAdministrativeActivityYet') }}</p>
          <p v-if="auditEventTotal > auditEvents.length" class="mt-3 text-center text-[11px] text-stone-400">{{ t('enterpriseMembers.dynamic.showingLatestAudit', { count: auditEvents.length }) }}</p>
        </section>

        <details v-if="!budgetMember?.deleted_at" class="overflow-hidden rounded-3xl border border-dashed border-stone-300 dark:border-white/15">
          <summary class="cursor-pointer px-5 py-4">
            <span class="block text-sm font-semibold text-stone-900 dark:text-white">{{ t('enterpriseMembers.copy.adjustUsedAmount') }}</span>
            <span class="mt-1 block text-xs leading-5 text-stone-500">{{ t('enterpriseMembers.copy.adjustUsedAmountHint') }}</span>
          </summary>
          <form class="border-t border-stone-200 p-5 dark:border-white/10" @submit.prevent="requestBudgetAdjustment">
            <div class="flex flex-wrap items-end gap-3"><div class="min-w-[180px] flex-1"><label class="input-label">{{ t('enterpriseMembers.copy.adjustmentAmountUsd') }}</label><input v-model.number="adjustment.amount" class="input" type="number" required step="0.00000001" min="-1000000" max="1000000" placeholder="-1.25" /></div><div class="min-w-[260px] flex-[2]"><label class="input-label">{{ t('enterpriseMembers.copy.auditNote') }}</label><input v-model.trim="adjustment.note" class="input" required maxlength="1000" :placeholder="t('enterpriseMembers.copy.stateTheReasonAndEvidence')" /></div><button class="btn btn-secondary" type="submit" :disabled="adjusting">{{ adjusting ? t('enterpriseMembers.copy.writing') : t('enterpriseMembers.copy.confirmAdjustment') }}</button></div>
            <p class="mt-2 text-xs text-stone-500">{{ t('enterpriseMembers.copy.positiveValuesIncreaseUsedCostNegativeValuesCreditItEntriesCannotBeDeletedAndUsageCannotBeReduce') }}</p>
          </form>
        </details>
      </div>
    </BaseDialog>

    <ConfirmDialog
      :show="Boolean(pendingBudgetAdjustment)"
      :title="t('enterpriseMembers.copy.adjustUsedAmount')"
      :message="budgetAdjustmentConfirmMessage"
      :confirm-text="t('enterpriseMembers.copy.confirmAdjustment')"
      :danger="true"
      @confirm="confirmBudgetAdjustment"
      @cancel="cancelBudgetAdjustment"
    />

    <BaseDialog :show="Boolean(plaintextKey)" :title="t('enterpriseMembers.copy.saveTheNewKey')" width="normal" @close="plaintextKey = ''">
      <div class="rounded-2xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/50 dark:bg-amber-950/20"><p class="text-sm font-semibold text-amber-950 dark:text-amber-100">{{ t('enterpriseMembers.copy.plaintextIsShownOnlyOnce') }}</p><code class="mt-3 block break-all rounded-xl bg-stone-950 p-4 text-xs text-amber-200">{{ plaintextKey }}</code></div>
      <template #footer><button class="btn btn-primary" type="button" @click="copyPlaintext">{{ t('enterpriseMembers.copy.copyAndClose') }}</button></template>
    </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'
import EnterpriseMemberBatchPolicyDialog from '@/components/enterprise/EnterpriseMemberBatchPolicyDialog.vue'
import EnterpriseMemberBatchUsageDialog, { type EnterpriseMemberBatchUsageTarget } from '@/components/enterprise/EnterpriseMemberBatchUsageDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { fillEnterpriseMemberUsageTrend } from '@/utils/enterpriseMemberUsageTrend'
import { tableSelectionCheckboxClasses as selectionCheckboxClasses } from '@/utils/tableSelectionCheckbox'
import { userGroupsAPI } from '@/api/groups'
import { usageAPI } from '@/api/usage'
import { enterpriseMembersAPI, type EnterpriseMember, type EnterpriseMemberAuditEvent, type EnterpriseMemberBatchPolicyInput, type EnterpriseMemberBudgetEntry, type EnterpriseMemberBudgetSummary, type EnterpriseMemberDraft, type EnterpriseMemberImportJob, type EnterpriseMemberImportPreview, type EnterpriseMemberImportResult, type EnterpriseMemberKeyUpdate, type EnterpriseMemberOwnerUsageItem, type EnterpriseMemberOwnerUsageSummary, type EnterpriseMemberStatus, type EnterpriseMemberUsageAnalytics, type EnterpriseMemberUsageDeltaInput } from '@/api/enterpriseMembers'
import type { ApiKey, Group, UsageLog } from '@/types'
import type { Column } from '@/components/common/types'

const { t, locale } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard: clipboardCopy } = useClipboard()
const isEnterprise = computed(() => authStore.user?.role === 'user' && authStore.user?.account_type === 'enterprise' && !authStore.user?.enterprise_disabled_at)

const members = ref<EnterpriseMember[]>([])
const availableGroups = ref<Group[]>([])
const loading = ref(false)
const saving = ref(false)
const archiveScope = ref<'current' | 'with_archived'>('current')
const includeArchived = computed(() => archiveScope.value === 'with_archived')
const search = ref('')
const statusFilter = ref('all')
const budgetFilter = ref('all')
const sortBy = ref('updated')
const selectedIds = ref(new Set<number>())
const batchGroupsOpen = ref(false)
const batchClearConfirmOpen = ref(false)
const batchGroupsSaving = ref(false)
const batchGroupMode = ref<'replace' | 'append'>('replace')
const batchGroupIds = ref<number[]>([])
const batchPolicyOpen = ref(false)
const batchPolicySaving = ref(false)
const batchUsageOpen = ref(false)
const batchUsageSaving = ref(false)
type PendingBatchPolicyOperation = {
  memberIDsKey: string
  inputKey: string
  members: EnterpriseMember[]
  input: EnterpriseMemberBatchPolicyInput
  idempotencyKey: string
}
type PendingBatchUsageOperation = {
  memberIDsKey: string
  inputKey: string
  members: EnterpriseMember[]
  input: EnterpriseMemberUsageDeltaInput
  idempotencyKey: string
}
const pendingBatchPolicyOperation = ref<PendingBatchPolicyOperation | null>(null)
const pendingBatchUsageOperation = ref<PendingBatchUsageOperation | null>(null)
const memberStatusChangeRequest = ref<{ status: EnterpriseMemberStatus, members: EnterpriseMember[], bulk: boolean } | null>(null)
const memberStatusUpdating = ref(false)
const memberRemovalTarget = ref<EnterpriseMember | null>(null)
const restoringMemberId = ref<number | null>(null)
const editorOpen = ref(false)
const editingMember = ref<EnterpriseMember | null>(null)
const emptyMemberDraft = (): EnterpriseMemberDraft => ({ member_code: '', name: '', monthly_limit_usd: 0, rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, group_ids: [] })
const draft = reactive<EnterpriseMemberDraft>(emptyMemberDraft())
const editorBudgetSummary = ref<EnterpriseMemberBudgetSummary | null>(null)
const editorBudgetLoading = ref(false)
const editorMonthlyUsed = ref(0)
const editorUsage5h = ref(0)
const editorUsage1d = ref(0)
const editorUsage7d = ref(0)
const usageAdjustmentChanged = computed(() => Boolean(editingMember.value && editorBudgetSummary.value && [
  editorMonthlyUsed.value - editorBudgetSummary.value.used_usd,
  editorUsage5h.value - editorBudgetSummary.value.usage_5h,
  editorUsage1d.value - editorBudgetSummary.value.usage_1d,
  editorUsage7d.value - editorBudgetSummary.value.usage_7d
].some(delta => Math.abs(delta) > 0.00000001)))

const keysOpen = ref(false)
const keyMember = ref<EnterpriseMember | null>(null)
const memberKeys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const keySaving = ref(false)
const keyDraft = reactive({ name: '', quota: 0 })
const editingKey = ref<ApiKey | null>(null)
const keyEditing = ref(false)
const keyEditDraft = reactive({ name: '', status: 'active' as 'active' | 'disabled', quota: 0, expires_at: '', rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, ip_whitelist: '', ip_blacklist: '' })
const plaintextKey = ref('')
const copyingMemberKeyId = ref<number | null>(null)
const copiedMemberKeyId = ref<number | null>(null)

const budgetOpen = ref(false)
const budgetMember = ref<EnterpriseMember | null>(null)
const budgetLoading = ref(false)
const budgetSummary = ref<EnterpriseMemberBudgetSummary | null>(null)
const budgetAnalytics = ref<EnterpriseMemberUsageAnalytics | null>(null)
const budgetEntries = ref<EnterpriseMemberBudgetEntry[]>([])
const budgetEntryTotal = ref(0)
const USAGE_RECORD_PAGE_SIZE = 20
const usageRecords = ref<UsageLog[]>([])
const usageRecordTotal = ref(0)
const usageRecordPage = ref(1)
const usageRecordsLoading = ref(false)
const usageRecordSortBy = ref('created_at')
const usageRecordSortOrder = ref<'asc' | 'desc'>('desc')
const auditEvents = ref<EnterpriseMemberAuditEvent[]>([])
const auditEventTotal = ref(0)
const analyticsDays = ref(30)
const adjusting = ref(false)
const adjustment = reactive({ amount: 0, note: '' })
const pendingBudgetAdjustment = ref<{ memberId: number; memberName: string; amount: number; note: string } | null>(null)
let budgetSessionSeq = 0
let usageRecordsReqSeq = 0
let budgetAnalyticsReqSeq = 0

const importOpen = ref(false)
const importFile = ref<File | null>(null)
const templateDownloading = ref<'csv' | 'xlsx' | null>(null)
const importPreviewLoading = ref(false)
const importCommitting = ref(false)
const importPreview = ref<EnterpriseMemberImportPreview | null>(null)
const importSelectedRows = ref(new Set<number>())
const importDefaultGroupIds = ref<number[]>([])
const importActivateMembers = ref(false)
const importCommitIdempotencyKey = ref('')
const importResult = ref<EnterpriseMemberImportResult | null>(null)
const importJob = ref<EnterpriseMemberImportJob | null>(null)
let importPollTimer: ReturnType<typeof setTimeout> | null = null
const ownerUsageSummary = ref<EnterpriseMemberOwnerUsageSummary | null>(null)
const ownerAuditOpen = ref(false)
const ownerAuditLoading = ref(false)
const ownerAuditEvents = ref<EnterpriseMemberAuditEvent[]>([])
const ownerAuditTotal = ref(0)

const activeCount = computed(() => members.value.filter(item => !item.deleted_at && item.status === 'active').length)
const totalKeyCount = computed(() => members.value.reduce((sum, item) => sum + item.key_count, 0))
const batchGroupTargets = computed(() => members.value.filter(member => selectedIds.value.has(member.id) && !member.deleted_at))
const batchGroupTargetCount = computed(() => batchGroupTargets.value.length)
const batchEditableTargets = computed(() => members.value.filter(member => selectedIds.value.has(member.id) && !member.deleted_at))
const batchTargetLimitExceeded = computed(() => batchEditableTargets.value.length > 500)
const batchUsageTargets = computed<EnterpriseMemberBatchUsageTarget[]>(() => batchEditableTargets.value.map(member => ({
  id: member.id,
  name: member.name,
  monthlyUsed: ownerUsageSummary.value?.members.find(item => item.member_id === member.id)?.used_usd || 0,
  usage5h: member.usage_5h,
  usage1d: member.usage_1d,
  usage7d: member.usage_7d
})))
const batchClearIsDestructive = computed(() => batchGroupMode.value === 'replace' && batchGroupIds.value.length === 0)
const memberStatusChangeTitle = computed(() => {
  if (!memberStatusChangeRequest.value) return ''
  return t(memberStatusChangeRequest.value.status === 'disabled'
    ? 'enterpriseMembers.copy.disableMemberTitle'
    : 'enterpriseMembers.copy.enableMemberTitle')
})
const memberStatusChangeConfirmText = computed(() => {
  if (!memberStatusChangeRequest.value) return ''
  return t(memberStatusChangeRequest.value.status === 'disabled'
    ? 'enterpriseMembers.copy.confirmDisable'
    : 'enterpriseMembers.copy.confirmEnable')
})
const memberStatusChangeMessage = computed(() => {
  const request = memberStatusChangeRequest.value
  if (!request) return ''
  if (request.bulk) {
    return t(request.status === 'disabled'
      ? 'enterpriseMembers.dynamic.disableMembersConfirm'
      : 'enterpriseMembers.dynamic.enableMembersConfirm', { count: request.members.length })
  }
  return t(request.status === 'disabled'
    ? 'enterpriseMembers.dynamic.disableMemberConfirm'
    : 'enterpriseMembers.dynamic.enableMemberConfirm', { name: request.members[0]?.name || '' })
})
const memberArchiveScopeOptions = computed<SelectOption[]>(() => [
  { value: 'current', label: t('enterpriseMembers.copy.currentMembersOnly') },
  { value: 'with_archived', label: t('enterpriseMembers.copy.includeArchivedMembers') }
])
const memberStatusFilterOptions = computed<SelectOption[]>(() => {
  const options: SelectOption[] = [
    { value: 'all', label: t('enterpriseMembers.copy.allStatuses') },
    { value: 'active', label: t('enterpriseMembers.copy.active') },
    { value: 'disabled', label: t('enterpriseMembers.copy.disabled') }
  ]
  if (includeArchived.value) options.push({ value: 'archived', label: t('enterpriseMembers.copy.archived') })
  return options
})
const memberBudgetFilterOptions = computed<SelectOption[]>(() => [
  { value: 'all', label: t('enterpriseMembers.copy.allBudgetStates') },
  { value: 'near', label: t('enterpriseMembers.copy.nearLimit80') },
  { value: 'exhausted', label: t('enterpriseMembers.copy.budgetExhausted') },
  { value: 'unlimited', label: t('enterpriseMembers.copy.unlimited') }
])
const memberSortOptions = computed<SelectOption[]>(() => [
  { value: 'updated', label: t('enterpriseMembers.copy.recentlyUpdated') },
  { value: 'name', label: t('enterpriseMembers.copy.memberName') },
  { value: 'budget', label: t('enterpriseMembers.copy.budgetHighToLow') },
  { value: 'keys', label: t('enterpriseMembers.copy.keyCountHighToLow') }
])
const memberKeyEditableStatusOptions = computed<SelectOption[]>(() => [
  { value: 'active', label: t('enterpriseMembers.copy.active') },
  { value: 'disabled', label: t('enterpriseMembers.copy.disabled') }
])
const batchGroupModeOptions = computed<SelectOption[]>(() => [
  { value: 'replace', label: t('enterpriseMembers.copy.replaceAccessibleGroups') },
  { value: 'append', label: t('enterpriseMembers.copy.appendAccessibleGroups') }
])
const usageRecordPages = computed(() => Math.max(1, Math.ceil(usageRecordTotal.value / USAGE_RECORD_PAGE_SIZE)))
const budgetUsageRecordColumns = computed<Column[]>(() => [
  { key: 'api_key', label: t('usage.apiKeyFilter'), sortable: false },
  { key: 'model', label: t('usage.model'), sortable: true },
  { key: 'reasoning_effort', label: t('usage.reasoningEffort'), sortable: false },
  { key: 'endpoint', label: t('usage.endpoint'), sortable: false },
  { key: 'ip_address', label: 'IP', sortable: false },
  { key: 'group', label: t('admin.usage.group'), sortable: false },
  { key: 'stream', label: t('usage.type'), sortable: false },
  { key: 'billing_mode', label: t('admin.usage.billingMode'), sortable: false },
  { key: 'tokens', label: t('usage.tokens'), sortable: false },
  { key: 'cost', label: t('usage.cost'), sortable: false },
  { key: 'latency', label: t('usage.latency'), sortable: false },
  { key: 'created_at', label: t('usage.time'), sortable: true }
])
const filteredMembers = computed(() => {
  const term = search.value.toLocaleLowerCase()
  const list = members.value.filter(member => {
    const matchesTerm = !term || member.name.toLocaleLowerCase().includes(term) || member.member_code.toLocaleLowerCase().includes(term)
    const matchesStatus = statusFilter.value === 'all'
      || (statusFilter.value === 'archived' ? Boolean(member.deleted_at) : !member.deleted_at && member.status === statusFilter.value)
    const matchesArchiveScope = includeArchived.value || !member.deleted_at
    const usage = memberUsage(member.id)
    const consumed = usage?.used_usd || 0
    const ratio = member.monthly_limit_usd > 0 ? consumed / member.monthly_limit_usd : 0
    const matchesBudget = budgetFilter.value === 'all'
      || (budgetFilter.value === 'unlimited' && member.monthly_limit_usd <= 0)
      || (budgetFilter.value === 'near' && member.monthly_limit_usd > 0 && ratio >= 0.8 && ratio < 1)
      || (budgetFilter.value === 'exhausted' && member.monthly_limit_usd > 0 && ratio >= 1)
    return matchesTerm && matchesArchiveScope && matchesStatus && matchesBudget
  })
  return [...list].sort((a, b) => {
    if (sortBy.value === 'name') return a.name.localeCompare(b.name)
    if (sortBy.value === 'budget') return b.monthly_limit_usd - a.monthly_limit_usd
    if (sortBy.value === 'keys') return b.key_count - a.key_count
    return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
  })
})
const allFilteredMembersSelected = computed(() => filteredMembers.value.length > 0 && filteredMembers.value.every(member => selectedIds.value.has(member.id)))
const someFilteredMembersSelected = computed(() => filteredMembers.value.some(member => selectedIds.value.has(member.id)))
const budgetSettledPercent = computed(() => {
  if (!budgetSummary.value || budgetSummary.value.limit_usd <= 0) return 0
  return Math.min(100, Math.max(0, (budgetSummary.value.used_usd / budgetSummary.value.limit_usd) * 100))
})
const budgetUsagePercentRaw = computed(() => {
  if (!budgetSummary.value || budgetSummary.value.limit_usd <= 0) return 0
  return Math.max(0, (budgetSummary.value.used_usd / budgetSummary.value.limit_usd) * 100)
})
const budgetUsagePercent = computed(() => Math.min(100, budgetUsagePercentRaw.value))
const budgetUsagePercentLabel = computed(() => {
  const percent = budgetUsagePercentRaw.value
  if (percent <= 0) return '0%'
  if (percent < 0.01) return '<0.01%'
  if (percent < 1) return `${percent.toFixed(2)}%`
  if (percent < 10) return `${percent.toFixed(1)}%`
  return `${Math.round(percent)}%`
})
const budgetOverageUsd = computed(() => {
  if (!budgetSummary.value || budgetSummary.value.limit_usd <= 0) return 0
  return Math.max(0, budgetSummary.value.used_usd - budgetSummary.value.limit_usd)
})
const budgetHasOverage = computed(() => budgetOverageUsd.value > 0)
const budgetExhausted = computed(() => Boolean(
  budgetSummary.value
  && budgetSummary.value.limit_usd > 0
  && budgetSummary.value.used_usd >= budgetSummary.value.limit_usd,
))
const budgetAdjustmentConfirmMessage = computed(() => pendingBudgetAdjustment.value
  ? t('enterpriseMembers.dynamic.confirmBudgetAdjustment', {
      name: pendingBudgetAdjustment.value.memberName,
      amount: formatMoney(pendingBudgetAdjustment.value.amount)
    })
  : '')
const memberRateLimitWindows = computed(() => {
  const summary = budgetSummary.value
  if (!summary) return []
  return [
    { key: '5h', label: t('enterpriseMembers.copy.fiveHourWindow'), limit: summary.rate_limit_5h, used: summary.usage_5h, resetAt: summary.reset_5h_at || null },
    { key: '1d', label: t('enterpriseMembers.copy.oneDayWindow'), limit: summary.rate_limit_1d, used: summary.usage_1d, resetAt: summary.reset_1d_at || null },
    { key: '7d', label: t('enterpriseMembers.copy.sevenDayWindow'), limit: summary.rate_limit_7d, used: summary.usage_7d, resetAt: summary.reset_7d_at || null }
  ]
})
const hasMemberRateLimits = computed(() => memberRateLimitWindows.value.some(window => window.limit > 0))
const normalizedBudgetTrend = computed(() => {
  const analytics = budgetAnalytics.value
  if (!analytics) return []
  return fillEnterpriseMemberUsageTrend(analytics.trend, analytics.start, analyticsDays.value, budgetSummary.value?.timezone || 'Asia/Shanghai')
})
const maxTrendCost = computed(() => Math.max(0, ...normalizedBudgetTrend.value.map(point => point.actual_cost)))
const allValidImportRowsSelected = computed(() => Boolean(importPreview.value?.valid_rows) && importPreview.value?.rows.filter(row => row.valid).every(row => importSelectedRows.value.has(row.row_number)))

async function loadMembers() {
  if (!isEnterprise.value) return
  loading.value = true
  try {
    const [memberRows, groups, usageSummary] = await Promise.all([enterpriseMembersAPI.list(includeArchived.value), userGroupsAPI.getAvailable(), enterpriseMembersAPI.getOwnerUsageSummary()])
    members.value = memberRows
    availableGroups.value = groups
    ownerUsageSummary.value = usageSummary
    selectedIds.value = new Set([...selectedIds.value].filter(id => memberRows.some(row => row.id === id)))
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToLoadEnterpriseMembers'))
  } finally { loading.value = false }
}

async function handleArchiveScopeChange() {
  if (!includeArchived.value && statusFilter.value === 'archived') statusFilter.value = 'all'
  await loadMembers()
}

function openCreate() {
  editingMember.value = null
  Object.assign(draft, emptyMemberDraft())
  editorBudgetSummary.value = null
  editorMonthlyUsed.value = 0
  editorUsage5h.value = 0
  editorUsage1d.value = 0
  editorUsage7d.value = 0
  editorOpen.value = true
}
function openImport() {
  importFile.value = null
  importPreview.value = null
  importSelectedRows.value = new Set()
  importDefaultGroupIds.value = []
  importActivateMembers.value = false
  importCommitIdempotencyKey.value = ''
  importResult.value = null
  importJob.value = null
  importOpen.value = true
}
async function openOwnerAudit() {
  ownerAuditOpen.value = true
  ownerAuditLoading.value = true
  try {
    const audit = await enterpriseMembersAPI.listOwnerAuditEvents()
    ownerAuditEvents.value = audit.items
    ownerAuditTotal.value = audit.total
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToLoadAuditRecords'))
  } finally { ownerAuditLoading.value = false }
}
function selectImportFile(event: Event) {
  importFile.value = (event.target as HTMLInputElement).files?.[0] || null
  importPreview.value = null
  importSelectedRows.value = new Set()
  importDefaultGroupIds.value = []
  importActivateMembers.value = false
  importCommitIdempotencyKey.value = ''
  importResult.value = null
}
async function downloadTemplate(format: 'csv' | 'xlsx') {
  if (templateDownloading.value) return
  templateDownloading.value = format
  try { await enterpriseMembersAPI.downloadImportTemplate(format, t('enterpriseMembers.copy.importTemplateFilename')) }
  catch (error: any) { appStore.showError(error.message || t('enterpriseMembers.copy.failedToDownloadTemplate')) }
  finally { templateDownloading.value = null }
}
async function previewImportFile() {
  if (!importFile.value) return
  importPreviewLoading.value = true
  try {
    const preview = await enterpriseMembersAPI.previewImport(importFile.value)
    if (preview.import_policy_version !== 2) {
      importPreview.value = null
      importCommitIdempotencyKey.value = ''
      importSelectedRows.value = new Set()
      throw new Error(t('enterpriseMembers.copy.importPreviewProtocolUnavailable'))
    }
    importPreview.value = preview
    importCommitIdempotencyKey.value = enterpriseMembersAPI.createImportCommitIdempotencyKey(preview.job_id)
    importSelectedRows.value = new Set(preview.rows.filter(row => row.valid).map(row => row.row_number))
  } catch (error: any) { appStore.showError(error.response?.data?.message || error.message || t('enterpriseMembers.copy.importPreviewFailed')) }
  finally { importPreviewLoading.value = false }
}
function toggleImportRow(rowNumber: number) {
  const next = new Set(importSelectedRows.value)
  next.has(rowNumber) ? next.delete(rowNumber) : next.add(rowNumber)
  importSelectedRows.value = next
}
function toggleAllImportRows() {
  if (!importPreview.value) return
  importSelectedRows.value = allValidImportRowsSelected.value ? new Set() : new Set(importPreview.value.rows.filter(row => row.valid).map(row => row.row_number))
}
async function commitImportRows() {
  if (!importPreview.value || !importSelectedRows.value.size) return
  const preview = importPreview.value
  if (!importCommitIdempotencyKey.value) {
    importCommitIdempotencyKey.value = enterpriseMembersAPI.createImportCommitIdempotencyKey(preview.job_id)
  }
  importCommitting.value = true
  try {
    const queued = await enterpriseMembersAPI.commitImport(preview, [...importSelectedRows.value], {
      defaultGroupIds: [...importDefaultGroupIds.value],
      activateMembers: importActivateMembers.value,
      idempotencyKey: importCommitIdempotencyKey.value
    })
    await pollImportJob(queued.job_id, preview.token)
  } catch {
    await reconcileImportCommitAfterError(preview.job_id, preview.token)
  }
}

async function reconcileImportCommitAfterError(jobId: number, resultToken: string) {
  try {
    const job = await enterpriseMembersAPI.getImportJob(jobId)
    if (job.status !== 'previewed') {
      importJob.value = job
      await pollImportJob(jobId, resultToken)
      return
    }
    importJob.value = null
    importCommitting.value = false
    importCommitIdempotencyKey.value = enterpriseMembersAPI.createImportCommitIdempotencyKey(jobId)
    appStore.showError(t('enterpriseMembers.copy.importCommitNotQueued'))
  } catch {
    importCommitting.value = false
    appStore.showError(t('enterpriseMembers.copy.importCommitOutcomeUnknown'))
  }
}

function toggleOrderedGroup(target: number[], groupId: number): number[] {
  return target.includes(groupId) ? target.filter(id => id !== groupId) : [...target, groupId]
}

function moveOrderedGroup(target: number[], groupId: number, direction: -1 | 1): number[] {
  const from = target.indexOf(groupId)
  const to = from + direction
  if (from < 0 || to < 0 || to >= target.length) return target
  const next = [...target]
  const moved = next[from]
  next[from] = next[to]
  next[to] = moved
  return next
}

function toggleImportDefaultGroup(groupId: number) {
  importDefaultGroupIds.value = toggleOrderedGroup(importDefaultGroupIds.value, groupId)
  if (importDefaultGroupIds.value.length === 0) importActivateMembers.value = false
}

function moveImportDefaultGroup(groupId: number, direction: -1 | 1) {
  importDefaultGroupIds.value = moveOrderedGroup(importDefaultGroupIds.value, groupId, direction)
}

function batchMemberIDsKey(targets: EnterpriseMember[]): string {
  return targets.map(member => member.id).sort((left, right) => left - right).join(',')
}

function cloneBatchMembers(targets: EnterpriseMember[]): EnterpriseMember[] {
  return targets.map(member => ({ ...member, group_ids: [...member.group_ids] }))
}

// If a response is lost after commit, a later member-list refresh must not
// replace the original optimistic versions. Retrying the same logical action
// therefore reuses the frozen request and key until the user changes its scope.
function prepareBatchPolicyOperation(targets: EnterpriseMember[], input: EnterpriseMemberBatchPolicyInput): PendingBatchPolicyOperation {
  const memberIDsKey = batchMemberIDsKey(targets)
  const frozenInput: EnterpriseMemberBatchPolicyInput = {
    ...input,
    ...(input.group_ids ? { group_ids: [...input.group_ids] } : {})
  }
  const inputKey = JSON.stringify(frozenInput)
  const pending = pendingBatchPolicyOperation.value
  if (pending?.memberIDsKey === memberIDsKey && pending.inputKey === inputKey) return pending

  const operation: PendingBatchPolicyOperation = {
    memberIDsKey,
    inputKey,
    members: cloneBatchMembers(targets),
    input: frozenInput,
    idempotencyKey: enterpriseMembersAPI.createEnterpriseMemberOperationIdempotencyKey('member-batch')
  }
  pendingBatchPolicyOperation.value = operation
  return operation
}

function prepareBatchUsageOperation(targets: EnterpriseMember[], input: EnterpriseMemberUsageDeltaInput): PendingBatchUsageOperation {
  const memberIDsKey = batchMemberIDsKey(targets)
  const frozenInput = { ...input }
  const inputKey = JSON.stringify(frozenInput)
  const pending = pendingBatchUsageOperation.value
  if (pending?.memberIDsKey === memberIDsKey && pending.inputKey === inputKey) return pending

  const operation: PendingBatchUsageOperation = {
    memberIDsKey,
    inputKey,
    members: cloneBatchMembers(targets),
    input: frozenInput,
    idempotencyKey: enterpriseMembersAPI.createEnterpriseMemberOperationIdempotencyKey('member-usage-batch')
  }
  pendingBatchUsageOperation.value = operation
  return operation
}

function closeBatchPolicy() {
  batchPolicyOpen.value = false
  pendingBatchPolicyOperation.value = null
}

function closeBatchUsage() {
  batchUsageOpen.value = false
  pendingBatchUsageOperation.value = null
}

function openBatchGroups(allowImportedBatch = false) {
  if (!batchGroupTargets.value.length || (batchGroupTargets.value.length > 500 && !allowImportedBatch)) return
  batchGroupMode.value = 'replace'
  batchGroupIds.value = []
  batchClearConfirmOpen.value = false
  batchGroupsOpen.value = true
}

function toggleBatchGroup(groupId: number) {
  batchGroupIds.value = toggleOrderedGroup(batchGroupIds.value, groupId)
}

function moveBatchGroup(groupId: number, direction: -1 | 1) {
  batchGroupIds.value = moveOrderedGroup(batchGroupIds.value, groupId, direction)
}

function requestSaveBatchGroups() {
  if (batchClearIsDestructive.value) {
    batchGroupsOpen.value = false
    batchClearConfirmOpen.value = true
    return
  }
  void saveBatchGroups()
}

function cancelBatchGroupClear() {
  batchClearConfirmOpen.value = false
  batchGroupsOpen.value = true
}

async function confirmBatchGroupClear() {
  batchClearConfirmOpen.value = false
  await saveBatchGroups()
}

async function saveBatchGroups() {
  const targets = batchGroupTargets.value
  if (!targets.length) return
  batchGroupsSaving.value = true
  try {
    const updated = await enterpriseMembersAPI.batchReplaceGroups(targets, batchGroupIds.value, batchGroupMode.value)
    const updatedById = new Map(updated.map(member => [member.id, member]))
    members.value = members.value.map(member => {
      const update = updatedById.get(member.id)
      return update ? { ...member, version: update.version, group_ids: update.group_ids, status: update.status, updated_at: update.updated_at } : member
    })
    batchGroupsOpen.value = false
    batchClearConfirmOpen.value = false
    appStore.showSuccess(t('enterpriseMembers.dynamic.batchGroupsUpdated', { count: updated.length }))
  } catch (error: any) {
    if (batchClearIsDestructive.value) batchGroupsOpen.value = true
    appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.batchGroupUpdateFailed'))
  } finally {
    batchGroupsSaving.value = false
  }
}

async function saveBatchPolicy(input: EnterpriseMemberBatchPolicyInput) {
  const targets = batchEditableTargets.value
  if (!targets.length || batchPolicySaving.value) return
  const operation = prepareBatchPolicyOperation(targets, input)
  batchPolicySaving.value = true
  try {
    const updated = await enterpriseMembersAPI.batchUpdate(operation.members, operation.input, operation.idempotencyKey)
    pendingBatchPolicyOperation.value = null
    await loadMembers()
    selectedIds.value = new Set()
    batchPolicyOpen.value = false
    appStore.showSuccess(batchPolicySuccessMessage(input, updated.updated_count))
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.batchEditFailed')))
  } finally {
    batchPolicySaving.value = false
  }
}

function batchPolicySuccessMessage(input: EnterpriseMemberBatchPolicyInput, count: number): string {
  const changesLimits = input.monthly_limit_usd !== undefined || input.rate_limit_5h !== undefined || input.rate_limit_1d !== undefined || input.rate_limit_7d !== undefined
  const changesGroups = input.group_mode !== undefined && input.group_mode !== 'keep'
  if (input.status && !changesLimits && !changesGroups) {
    return t(input.status === 'active'
      ? 'enterpriseMembers.dynamic.batchMembersEnabled'
      : 'enterpriseMembers.dynamic.batchMembersDisabled', { count })
  }
  return t('enterpriseMembers.dynamic.batchMembersUpdated', { count })
}

async function saveBatchUsage(input: EnterpriseMemberUsageDeltaInput) {
  const targets = batchEditableTargets.value
  if (!targets.length || batchUsageSaving.value) return
  const operation = prepareBatchUsageOperation(targets, input)
  batchUsageSaving.value = true
  try {
    const updated = await enterpriseMembersAPI.batchAdjustUsage(operation.members, operation.input, operation.idempotencyKey)
    pendingBatchUsageOperation.value = null
    await loadMembers()
    selectedIds.value = new Set()
    batchUsageOpen.value = false
    appStore.showSuccess(t('enterpriseMembers.dynamic.batchUsageAdjusted', { count: updated.updated_count }))
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.batchUsageAdjustmentFailed')))
  } finally {
    batchUsageSaving.value = false
  }
}
async function pollImportJob(jobId: number, resultToken: string) {
  try {
    const job = await enterpriseMembersAPI.getImportJob(jobId)
    importJob.value = job
    if (job.status === 'completed' && job.result) {
      let keys = job.result.keys || []
      if (job.result.created_keys > 0 && !job.result_secrets_consumed_at) {
        keys = await enterpriseMembersAPI.consumeImportResultSecrets(jobId, resultToken)
      }
      importResult.value = { ...job.result, keys }
      importPreview.value = null
      importSelectedRows.value = new Set()
      importCommitting.value = false
      await loadMembers()
      return
    }
    if (job.status === 'failed') {
      importCommitting.value = false
      appStore.showError(job.error_summary || t('enterpriseMembers.copy.importTransactionFailedAndWasFullyRolledBack'))
      return
    }
    importPollTimer = setTimeout(() => { void pollImportJob(jobId, resultToken) }, 1000)
  } catch (error: any) {
    if (importCommitting.value) {
      importPollTimer = setTimeout(() => { void pollImportJob(jobId, resultToken) }, 2000)
    } else {
      appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToReadImportJobStatus'))
    }
  }
}

function configureImportedMembers() {
  if (!importResult.value) return
  selectedIds.value = new Set(importResult.value.member_ids)
  importOpen.value = false
  openBatchGroups(true)
}

function hasMigrationBaseline(summary: EnterpriseMemberBudgetSummary): boolean {
  return summary.migration_billed_usd > 0
    || exactTokenCountIsPositive(summary.migration_total_tokens)
    || exactTokenCountIsPositive(summary.migration_input_tokens)
    || exactTokenCountIsPositive(summary.migration_output_tokens)
    || exactTokenCountIsPositive(summary.migration_cache_tokens)
    || exactTokenCountIsPositive(summary.migration_cache_write_tokens)
    || exactTokenCountIsPositive(summary.migration_cache_read_tokens)
}

async function downloadImportErrors(jobId: number) {
  try { await enterpriseMembersAPI.downloadImportErrorReport(jobId) }
  catch (error: any) { appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToDownloadErrorReport')) }
}
async function openEdit(member: EnterpriseMember) {
  editingMember.value = member
  Object.assign(draft, {
    member_code: member.member_code,
    name: member.name,
    monthly_limit_usd: member.monthly_limit_usd,
    rate_limit_5h: member.rate_limit_5h || 0,
    rate_limit_1d: member.rate_limit_1d || 0,
    rate_limit_7d: member.rate_limit_7d || 0,
    group_ids: [...member.group_ids]
  })
  editorBudgetSummary.value = null
  editorMonthlyUsed.value = 0
  editorUsage5h.value = 0
  editorUsage1d.value = 0
  editorUsage7d.value = 0
  editorOpen.value = true
  editorBudgetLoading.value = true
  try {
    editorBudgetSummary.value = await enterpriseMembersAPI.getBudget(member.id)
    editorMonthlyUsed.value = editorBudgetSummary.value.used_usd
    editorUsage5h.value = editorBudgetSummary.value.usage_5h
    editorUsage1d.value = editorBudgetSummary.value.usage_1d
    editorUsage7d.value = editorBudgetSummary.value.usage_7d
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.failedToLoadBudgetAndUsage')))
  } finally { editorBudgetLoading.value = false }
}
function toggleDraftGroup(id: number) {
  const index = draft.group_ids.indexOf(id)
  if (index >= 0) draft.group_ids.splice(index, 1)
  else draft.group_ids.push(id)
}
function moveDraftGroup(id: number, direction: number) {
  const index = draft.group_ids.indexOf(id)
  const target = index + direction
  if (index < 0 || target < 0 || target >= draft.group_ids.length) return
  const [value] = draft.group_ids.splice(index, 1)
  draft.group_ids.splice(target, 0, value)
}
async function saveMember() {
  saving.value = true
  try {
    if (editingMember.value) {
      let updated = await enterpriseMembersAPI.update(editingMember.value, {
        name: draft.name,
        monthly_limit_usd: draft.monthly_limit_usd,
        rate_limit_5h: draft.rate_limit_5h,
        rate_limit_1d: draft.rate_limit_1d,
        rate_limit_7d: draft.rate_limit_7d
      })
      const groups = await enterpriseMembersAPI.replaceGroups(updated, [...draft.group_ids])
      updated = { ...updated, group_ids: groups.group_ids, version: groups.version }
      if (usageAdjustmentChanged.value && editorBudgetSummary.value) {
        await enterpriseMembersAPI.setUsage(updated.id, {
          monthly_used_usd: editorMonthlyUsed.value,
          usage_5h: editorUsage5h.value,
          usage_1d: editorUsage1d.value,
          usage_7d: editorUsage7d.value
        })
      }
      members.value = members.value.map(item => item.id === updated.id ? updated : item)
    } else {
      const created = await enterpriseMembersAPI.create({
        ...draft,
        group_ids: [...draft.group_ids],
        monthly_used_usd: editorMonthlyUsed.value,
        usage_5h: editorUsage5h.value,
        usage_1d: editorUsage1d.value,
        usage_7d: editorUsage7d.value
      })
      members.value.unshift(created)
    }
    editorOpen.value = false
    await loadMembers()
    appStore.showSuccess(t('enterpriseMembers.copy.memberSaved'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.message || error.message || t('enterpriseMembers.copy.saveFailedPleaseRefreshAndRetry'))
  } finally { saving.value = false }
}

function toggleStatus(member: EnterpriseMember) {
  if (member.deleted_at || memberStatusUpdating.value) return
  memberStatusChangeRequest.value = {
    status: member.status === 'active' ? 'disabled' : 'active',
    members: [member],
    bulk: false
  }
}
function bulkSetStatus(status: EnterpriseMemberStatus) {
  if (memberStatusUpdating.value || batchTargetLimitExceeded.value) return
  const targets = members.value.filter(item => selectedIds.value.has(item.id) && !item.deleted_at && item.status !== status)
  if (!targets.length) return
  memberStatusChangeRequest.value = { status, members: targets, bulk: true }
}
function cancelMemberStatusChange() {
  memberStatusChangeRequest.value = null
  pendingBatchPolicyOperation.value = null
}
async function confirmMemberStatusChange() {
  const request = memberStatusChangeRequest.value
  if (!request || memberStatusUpdating.value) return
  memberStatusChangeRequest.value = null
  memberStatusUpdating.value = true
  try {
    if (request.bulk) {
      const input: EnterpriseMemberBatchPolicyInput = { status: request.status, group_mode: 'keep' }
      const operation = prepareBatchPolicyOperation(request.members, input)
      const updated = await enterpriseMembersAPI.batchUpdate(operation.members, operation.input, operation.idempotencyKey)
      pendingBatchPolicyOperation.value = null
      await loadMembers()
      selectedIds.value = new Set()
      appStore.showSuccess(t(request.status === 'active'
        ? 'enterpriseMembers.dynamic.batchMembersEnabled'
        : 'enterpriseMembers.dynamic.batchMembersDisabled', { count: updated.updated_count }))
      return
    }

    const updated = await enterpriseMembersAPI.setStatus(request.members[0], request.status)
    members.value = members.value.map(item => item.id === updated.id ? updated : item)
    appStore.showSuccess(t(request.status === 'disabled'
      ? 'enterpriseMembers.copy.memberDisabledSuccess'
      : 'enterpriseMembers.copy.memberEnabledSuccess'))
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.statusUpdateFailed')))
  } finally {
    memberStatusUpdating.value = false
  }
}
function removeMember(member: EnterpriseMember) {
  memberRemovalTarget.value = member
}
function cancelRemoveMember() {
  memberRemovalTarget.value = null
}
function memberRemovalMessage(member: EnterpriseMember | null): string {
  if (!member) return ''
  if (!member.deleted_at) return t('enterpriseMembers.copy.archivingImmediatelyInvalidatesAllMemberKeysWhilePreservingAuditHistoryContinue')
  return member.delete_strategy === 'hard_delete'
    ? t('enterpriseMembers.copy.deleteCleanMemberConfirm')
    : t('enterpriseMembers.copy.deleteHistoricalMemberConfirm')
}
async function restoreMember(member: EnterpriseMember) {
  if (!member.deleted_at || restoringMemberId.value !== null) return
  restoringMemberId.value = member.id
  try {
    await enterpriseMembersAPI.restore(member)
    await loadMembers()
    appStore.showSuccess(t('enterpriseMembers.copy.memberRestoredDisabled'))
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.operationFailed')))
  } finally {
    restoringMemberId.value = null
  }
}
async function confirmRemoveMember() {
  const member = memberRemovalTarget.value
  if (!member) return
  memberRemovalTarget.value = null
  try {
    member.deleted_at ? await enterpriseMembersAPI.permanentlyDelete(member) : await enterpriseMembersAPI.archive(member)
    await loadMembers()
    appStore.showSuccess(t(member.deleted_at ? 'enterpriseMembers.copy.memberPermanentlyDeleted' : 'enterpriseMembers.copy.memberArchived'))
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.operationFailed')))
  }
}
function toggleSelected(id: number) {
  const next = new Set(selectedIds.value)
  next.has(id) ? next.delete(id) : next.add(id)
  selectedIds.value = next
}
function toggleAllFilteredMembers() {
  const next = new Set(selectedIds.value)
  if (allFilteredMembersSelected.value) filteredMembers.value.forEach(member => next.delete(member.id))
  else filteredMembers.value.forEach(member => next.add(member.id))
  selectedIds.value = next
}

function resetFilters() {
  search.value = ''
  statusFilter.value = 'all'
  budgetFilter.value = 'all'
  sortBy.value = 'updated'
  archiveScope.value = 'current'
}

async function openKeys(member: EnterpriseMember) {
  keyMember.value = member
  keysOpen.value = true
  keysLoading.value = true
  editingKey.value = null
  copiedMemberKeyId.value = null
  try {
    memberKeys.value = await enterpriseMembersAPI.listKeys(member.id)
  }
  catch (error: unknown) { appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.failedToLoadKeys'))) }
  finally { keysLoading.value = false }
}

function openRegularKeys() {
  keysOpen.value = false
  void router.push('/keys')
}

async function copyMemberKey(key: ApiKey) {
  if (!keyMember.value || copyingMemberKeyId.value !== null) return
  const requestedMemberId = keyMember.value.id
  const requestedKeyId = key.id
  copyingMemberKeyId.value = requestedKeyId
  try {
    const detail = await enterpriseMembersAPI.revealKey(requestedMemberId, requestedKeyId)
    if (keyMember.value?.id !== requestedMemberId) return
    if (detail.id !== requestedKeyId || detail.member_id !== requestedMemberId) {
      appStore.showError(t('enterpriseMembers.copy.failedToCopyKey'))
      return
    }
    const copied = await clipboardCopy(detail.key, t('enterpriseMembers.copy.keyCopied'))
    if (copied) {
      copiedMemberKeyId.value = requestedKeyId
      window.setTimeout(() => {
        if (copiedMemberKeyId.value === requestedKeyId) copiedMemberKeyId.value = null
      }, 1600)
    }
  } catch (error: unknown) {
    appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.failedToCopyKey')))
  } finally {
    copyingMemberKeyId.value = null
  }
}

function openKeyEdit(key: ApiKey) {
  editingKey.value = key
  Object.assign(keyEditDraft, {
    name: key.name,
    status: key.status === 'active' ? 'active' : 'disabled',
    quota: key.quota,
    expires_at: toDateTimeLocal(key.expires_at),
    rate_limit_5h: key.rate_limit_5h,
    rate_limit_1d: key.rate_limit_1d,
    rate_limit_7d: key.rate_limit_7d,
    ip_whitelist: key.ip_whitelist.join('\n'),
    ip_blacklist: key.ip_blacklist.join('\n')
  })
}

async function saveMemberKey() {
  if (!keyMember.value || !editingKey.value) return
  keyEditing.value = true
  try {
    const input: EnterpriseMemberKeyUpdate = {
      name: keyEditDraft.name,
      status: keyEditDraft.status,
      quota: keyEditDraft.quota || 0,
      expires_at: keyEditDraft.expires_at ? new Date(keyEditDraft.expires_at).toISOString() : '',
      rate_limit_5h: keyEditDraft.rate_limit_5h || 0,
      rate_limit_1d: keyEditDraft.rate_limit_1d || 0,
      rate_limit_7d: keyEditDraft.rate_limit_7d || 0,
      ip_whitelist: splitIPRules(keyEditDraft.ip_whitelist),
      ip_blacklist: splitIPRules(keyEditDraft.ip_blacklist)
    }
    const updated = await enterpriseMembersAPI.updateKey(keyMember.value.id, editingKey.value.id, input)
    memberKeys.value = memberKeys.value.map(item => item.id === updated.id ? updated : item)
    editingKey.value = null
    appStore.showSuccess(t('enterpriseMembers.copy.memberKeyUpdated'))
  } catch (error: any) { appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToUpdateMemberKey')) }
  finally { keyEditing.value = false }
}
async function openBudget(member: EnterpriseMember) {
  const session = ++budgetSessionSeq
  const recordsSeq = ++usageRecordsReqSeq
  const analyticsSeq = ++budgetAnalyticsReqSeq
  const initialAnalyticsDays = analyticsDays.value
  budgetMember.value = member
  budgetOpen.value = true
  budgetLoading.value = true
  usageRecordsLoading.value = false
  usageRecordSortBy.value = 'created_at'
  usageRecordSortOrder.value = 'desc'
  budgetSummary.value = null
  budgetAnalytics.value = null
  budgetEntries.value = []
  budgetEntryTotal.value = 0
  auditEvents.value = []
  auditEventTotal.value = 0
  usageRecords.value = []
  usageRecordTotal.value = 0
  usageRecordPage.value = 1
  try {
    const [summary, analytics, ledger, audit, records] = await Promise.all([
      enterpriseMembersAPI.getBudget(member.id),
      enterpriseMembersAPI.getUsageAnalytics(member.id, initialAnalyticsDays),
      enterpriseMembersAPI.listBudgetEntries(member.id),
      enterpriseMembersAPI.listAuditEvents(member.id),
      queryMemberUsageRecords(member.id, 1)
    ])
    if (session !== budgetSessionSeq || !budgetOpen.value || budgetMember.value?.id !== member.id) return
    budgetSummary.value = summary
    if (analyticsSeq === budgetAnalyticsReqSeq && analyticsDays.value === initialAnalyticsDays) {
      budgetAnalytics.value = analytics
    }
    budgetEntries.value = ledger.items
    budgetEntryTotal.value = ledger.total
    auditEvents.value = audit.items
    auditEventTotal.value = audit.total
    if (recordsSeq === usageRecordsReqSeq) {
      usageRecords.value = records.items
      usageRecordTotal.value = records.total
      usageRecordPage.value = records.page
    }
  } catch (error: unknown) {
    if (session === budgetSessionSeq) appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.failedToLoadBudgetAndUsage')))
  } finally {
    if (session === budgetSessionSeq) budgetLoading.value = false
  }
}
function closeBudget() {
  budgetOpen.value = false
  pendingBudgetAdjustment.value = null
  budgetSessionSeq += 1
  usageRecordsReqSeq += 1
  budgetAnalyticsReqSeq += 1
  budgetLoading.value = false
  usageRecordsLoading.value = false
}
function openUnifiedUsage(member: EnterpriseMember | null) {
  if (!member) return
  closeBudget()
  void router.push({ name: 'EnterpriseMemberUsage', query: { tab: 'usage', member_id: String(member.id) } })
}
function queryMemberUsageRecords(
  memberID: number,
  page: number,
  sortBy = usageRecordSortBy.value,
  sortOrder = usageRecordSortOrder.value
) {
  return usageAPI.query({
    member_id: memberID,
    page,
    page_size: USAGE_RECORD_PAGE_SIZE,
    sort_by: sortBy,
    sort_order: sortOrder
  })
}
async function loadUsageRecords(page: number) {
  if (!budgetMember.value || page < 1 || page > usageRecordPages.value) return
  const memberID = budgetMember.value.id
  const sortBy = usageRecordSortBy.value
  const sortOrder = usageRecordSortOrder.value
  const seq = ++usageRecordsReqSeq
  usageRecordsLoading.value = true
  try {
    const records = await queryMemberUsageRecords(memberID, page, sortBy, sortOrder)
    if (seq !== usageRecordsReqSeq || !budgetOpen.value || budgetMember.value?.id !== memberID) return
    usageRecords.value = records.items
    usageRecordTotal.value = records.total
    usageRecordPage.value = records.page
  } catch (error: unknown) {
    if (seq === usageRecordsReqSeq) appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.failedToLoadRequestRecords')))
  } finally {
    if (seq === usageRecordsReqSeq) usageRecordsLoading.value = false
  }
}
function handleUsageRecordSort(key: string, order: 'asc' | 'desc') {
  usageRecordSortBy.value = key
  usageRecordSortOrder.value = order
  void loadUsageRecords(1)
}
function handleUsageIpGeoBatchFailed() {
  appStore.showError(t('usage.ipGeo.batchFailed'))
}
async function reloadAnalytics() {
  if (!budgetMember.value) return
  const memberID = budgetMember.value.id
  const days = analyticsDays.value
  const seq = ++budgetAnalyticsReqSeq
  try {
    const analytics = await enterpriseMembersAPI.getUsageAnalytics(memberID, days)
    if (seq !== budgetAnalyticsReqSeq || !budgetOpen.value || budgetMember.value?.id !== memberID || analyticsDays.value !== days) return
    budgetAnalytics.value = analytics
  } catch (error: unknown) {
    if (seq === budgetAnalyticsReqSeq) appStore.showError(extractI18nErrorMessage(error, t, 'enterpriseMembers.errors', t('enterpriseMembers.copy.failedToLoadTrend')))
  }
}
function requestBudgetAdjustment() {
  if (!budgetMember.value || !adjustment.amount || !adjustment.note) return
  pendingBudgetAdjustment.value = {
    memberId: budgetMember.value.id,
    memberName: budgetMember.value.name,
    amount: adjustment.amount,
    note: adjustment.note
  }
}
function cancelBudgetAdjustment() {
  pendingBudgetAdjustment.value = null
}
async function confirmBudgetAdjustment() {
  const pending = pendingBudgetAdjustment.value
  if (!pending || adjusting.value || budgetMember.value?.id !== pending.memberId) return
  pendingBudgetAdjustment.value = null
  adjusting.value = true
  try {
    budgetSummary.value = await enterpriseMembersAPI.createBudgetAdjustment(pending.memberId, pending.amount, pending.note)
    const [ledger, audit] = await Promise.all([
      enterpriseMembersAPI.listBudgetEntries(pending.memberId),
      enterpriseMembersAPI.listAuditEvents(pending.memberId)
    ])
    budgetEntries.value = ledger.items
    budgetEntryTotal.value = ledger.total
    auditEvents.value = audit.items
    auditEventTotal.value = audit.total
    Object.assign(adjustment, { amount: 0, note: '' })
    appStore.showSuccess(t('enterpriseMembers.copy.adjustmentPostedToLedger'))
  } catch (error: any) { appStore.showError(error.response?.data?.message || error.message || t('enterpriseMembers.copy.adjustmentFailed')) }
  finally { adjusting.value = false }
}
async function createMemberKey() {
  if (!keyMember.value) return
  keySaving.value = true
  try {
    const key = await enterpriseMembersAPI.createKey(keyMember.value.id, { name: keyDraft.name, quota: keyDraft.quota || undefined })
    plaintextKey.value = key.key
    memberKeys.value.unshift({ ...key, key: maskKey(key.key) })
    members.value = members.value.map(item => item.id === keyMember.value?.id ? { ...item, key_count: item.key_count + 1 } : item)
    Object.assign(keyDraft, { name: '', quota: 0 })
  } catch (error: any) { appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToCreateKey')) }
  finally { keySaving.value = false }
}
async function removeKey(keyId: number) {
  if (!keyMember.value || !window.confirm(t('enterpriseMembers.copy.thisKeyCannotBeRestoredContinue'))) return
  try {
    await enterpriseMembersAPI.deleteKey(keyMember.value.id, keyId)
    memberKeys.value = memberKeys.value.filter(item => item.id !== keyId)
    members.value = members.value.map(item => item.id === keyMember.value?.id ? { ...item, key_count: Math.max(0, item.key_count - 1) } : item)
  } catch (error: any) { appStore.showError(error.response?.data?.message || t('enterpriseMembers.copy.failedToDeleteKey')) }
}
async function copyPlaintext() {
  const copied = await clipboardCopy(plaintextKey.value, t('enterpriseMembers.copy.keyCopied'))
  if (copied) plaintextKey.value = ''
}

const groupName = (id: number) => availableGroups.value.find(group => group.id === id)?.name || `#${id}`
const memberUsage = (id: number): EnterpriseMemberOwnerUsageItem | undefined => ownerUsageSummary.value?.members.find(item => item.member_id === id)
const memberBudgetUsed = (member: EnterpriseMember) => {
  const usage = memberUsage(member.id)
  return usage?.used_usd || 0
}
const memberBudgetPercent = (member: EnterpriseMember) => member.monthly_limit_usd > 0
  ? Math.min(100, Math.max(0, (memberBudgetUsed(member) / member.monthly_limit_usd) * 100))
  : 0
const memberBudgetBarClass = (member: EnterpriseMember) => {
  const percent = memberBudgetPercent(member)
  if (percent >= 100) return 'bg-rose-500'
  if (percent >= 80) return 'bg-amber-500'
  return 'bg-emerald-500'
}
const memberKeyQuotaPercent = (key: ApiKey) => key.quota > 0 ? Math.min(100, Math.max(0, (key.quota_used / key.quota) * 100)) : 0
const memberKeyStatusLabel = (status: ApiKey['status']) => ({
  active: t('enterpriseMembers.copy.active'),
  disabled: t('enterpriseMembers.copy.disabled'),
  quota_exhausted: t('enterpriseMembers.copy.quotaExhausted'),
  expired: t('enterpriseMembers.copy.expired')
}[status])
const memberKeyStatusBadgeClasses = (status: ApiKey['status']) => ({
  active: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300',
  disabled: 'bg-stone-100 text-stone-600 dark:bg-white/[0.07] dark:text-stone-300',
  quota_exhausted: 'bg-amber-100 text-amber-800 dark:bg-amber-400/10 dark:text-amber-300',
  expired: 'bg-rose-100 text-rose-700 dark:bg-rose-400/10 dark:text-rose-300'
}[status])
const formatMoney = (value: number) => new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', maximumFractionDigits: 2 }).format(value)
const exactTokenCountPattern = /^(\d+)(?:\.(\d{1,2}))?$/
const exactTokenCountIsPositive = (value: string) => !/^0+(?:\.0+)?$/.test(value)
const formatExactTokenCount = (value: string) => {
  const match = value.match(exactTokenCountPattern)
  if (!match) return value
  const integer = new Intl.NumberFormat(locale.value, { maximumFractionDigits: 0 }).format(BigInt(match[1]))
  const fraction = (match[2] || '').replace(/0+$/, '')
  if (!fraction) return integer
  const decimalSeparator = new Intl.NumberFormat(locale.value).formatToParts(1.1).find(part => part.type === 'decimal')?.value || '.'
  return `${integer}${decimalSeparator}${fraction}`
}
const formatNumber = (value: number | string) => typeof value === 'string'
  ? formatExactTokenCount(value)
  : new Intl.NumberFormat(locale.value, { notation: value >= 1_000_000 ? 'compact' : 'standard', maximumFractionDigits: 2 }).format(value)
const formatDate = (value: string) => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium' }).format(new Date(value))
const formatDateTime = (value: string) => new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
const trendHeight = (value: number) => value > 0 && maxTrendCost.value > 0 ? Math.max(4, (value / maxTrendCost.value) * 100) : 0
const entryKindLabel = (kind: string) => ({ usage: t('enterpriseMembers.copy.usage'), manual_adjustment: t('enterpriseMembers.copy.adjustment'), migration_opening: t('enterpriseMembers.copy.opening'), reconciliation: t('enterpriseMembers.copy.reconciliation') }[kind] || kind)
const importJobStatusLabel = (status: EnterpriseMemberImportJob['status']) => ({ previewed: t('enterpriseMembers.copy.ready'), queued: t('enterpriseMembers.copy.queued'), queued_v2: t('enterpriseMembers.copy.queued'), processing: t('enterpriseMembers.copy.processing'), processing_v2: t('enterpriseMembers.copy.processing'), completed: t('enterpriseMembers.copy.completed'), failed: t('enterpriseMembers.copy.failed') }[status])
const auditActionLabel = (action: string) => ({
  'member.created': t('enterpriseMembers.copy.memberCreated'),
  'member.updated': t('enterpriseMembers.copy.memberUpdated'),
  'member.usage_adjusted': t('enterpriseMembers.copy.memberUsageAdjusted'),
  'member.enabled': t('enterpriseMembers.copy.memberEnabled'),
  'member.disabled': t('enterpriseMembers.copy.memberDisabled'),
  'member.archived': t('enterpriseMembers.copy.memberArchived'),
  'member.restored': t('enterpriseMembers.copy.memberRestored'),
  'member.removed': t('enterpriseMembers.copy.memberRemoved'),
  'member.deleted': t('enterpriseMembers.copy.memberPermanentlyDeleted'),
  'member_group.bound': t('enterpriseMembers.copy.groupAssigned'),
  'member_group.reordered': t('enterpriseMembers.copy.groupOrderChanged'),
  'member_group.unbound': t('enterpriseMembers.copy.groupRemoved'),
  'member_key.created': t('enterpriseMembers.copy.memberKeyCreated'),
  'member_key.adopted': t('enterpriseMembers.copy.existingKeyAdopted'),
  'member_key.updated': t('enterpriseMembers.copy.memberKeyUpdated0f6eb20f'),
  'member_key.enabled': t('enterpriseMembers.copy.memberKeyEnabled'),
  'member_key.disabled': t('enterpriseMembers.copy.memberKeyDisabled'),
  'member_key.deleted': t('enterpriseMembers.copy.memberKeyDeleted'),
  'member_key.reveal_authorized': t('enterpriseMembers.copy.memberKeyRevealAuthorized'),
  'budget.manual_adjustment': t('enterpriseMembers.copy.manualAdjustment'),
  'budget.migration_opening': t('enterpriseMembers.copy.openingBalancePosted'),
  'budget.reconciliation': t('enterpriseMembers.copy.budgetReconciled')
}[action] || action.split('.').join(' · '))
const auditEntityLabel = (entityType: string) => ({ member: t('enterpriseMembers.copy.member'), group: t('enterpriseMembers.copy.group'), api_key: 'Key', budget_entry: t('enterpriseMembers.copy.ledger'), import_job: t('enterpriseMembers.copy.import') }[entityType] || entityType)
const auditMemberLabel = (event: EnterpriseMemberAuditEvent) => event.member_id ? members.value.find(member => member.id === event.member_id)?.name || t('enterpriseMembers.copy.historicalMember') : t('enterpriseMembers.copy.enterpriseAccount')
const auditFieldLabel = (field: string) => ({ member_code: t('enterpriseMembers.copy.code'), name: t('enterpriseMembers.copy.name'), status: t('enterpriseMembers.copy.status'), monthly_limit_usd: t('enterpriseMembers.copy.monthlyBudget'), rate_limit_5h: `5h ${t('enterpriseMembers.copy.limit')}`, rate_limit_1d: `1d ${t('enterpriseMembers.copy.limit')}`, rate_limit_7d: `7d ${t('enterpriseMembers.copy.limit')}`, group_id: t('enterpriseMembers.copy.group'), member_id: t('enterpriseMembers.copy.member'), sort_order: t('enterpriseMembers.copy.order'), quota: t('enterpriseMembers.copy.keyQuota6121f112'), expires_at: t('enterpriseMembers.copy.expiry'), amount_usd: t('enterpriseMembers.copy.amount'), note: t('enterpriseMembers.copy.note'), period_start: t('enterpriseMembers.copy.period'), disabled_reason: t('enterpriseMembers.copy.disabledReason'), deleted_at: t('enterpriseMembers.copy.archivedAt'), removed_at: t('enterpriseMembers.copy.removedAt') }[field] || field.split('_').join(' '))
function auditValue(value: unknown): string {
  if (value === null || value === undefined || value === '') return t('enterpriseMembers.copy.none')
  if (Array.isArray(value)) return value.length ? value.join(', ') : t('enterpriseMembers.copy.empty')
  if (typeof value === 'boolean') return value ? t('enterpriseMembers.copy.yes') : t('enterpriseMembers.copy.no')
  if (typeof value === 'object') return t('enterpriseMembers.copy.updated')
  return String(value)
}
function auditEventSummary(event: EnterpriseMemberAuditEvent): string {
  const before = event.before_data || {}
  const after = event.after_data || {}
  const keys = [...new Set([...Object.keys(before), ...Object.keys(after)])]
    .filter(key => key !== 'version' && JSON.stringify(before[key]) !== JSON.stringify(after[key]))
  const details = keys.slice(0, 4).map(key => {
    const label = auditFieldLabel(key)
    if (!(key in before)) return `${label}: ${auditValue(after[key])}`
    if (!(key in after)) return `${label}: ${auditValue(before[key])}`
    return `${label}: ${auditValue(before[key])} → ${auditValue(after[key])}`
  })
  if (keys.length > 4) details.push(t('enterpriseMembers.dynamic.moreChanges', { count: keys.length - 4 }))
  return details.join(' · ') || t('enterpriseMembers.copy.recordedWithoutSensitiveCredentials')
}
const importIssueLabel = (issue: string) => {
  const unauthorizedGroup = issue.match(/^group_(\d+)_not_authorized$/)
  if (unauthorizedGroup) return t('enterpriseMembers.dynamic.legacyGroupNotAuthorized', { id: unauthorizedGroup[1] })
  const tooShort = issue.match(/^api_key_too_short_(\d+)_(\d+)$/)
  if (tooShort) return t('enterpriseMembers.dynamic.apiKeyTooShort', { minimum: tooShort[1], actual: tooShort[2] })
  const tooLong = issue.match(/^api_key_too_long_(\d+)_(\d+)$/)
  if (tooLong) return t('enterpriseMembers.dynamic.apiKeyTooLong', { maximum: tooLong[1], actual: tooLong[2] })
  return ({ invalid_member_code: t('enterpriseMembers.copy.invalidMemberCode'), invalid_member_name: t('enterpriseMembers.copy.invalidMemberName'), invalid_monthly_limit: t('enterpriseMembers.copy.invalidMonthlyLimit'), invalid_rate_limit_5h: t('enterpriseMembers.copy.invalidRateLimit5h'), invalid_rate_limit_1d: t('enterpriseMembers.copy.invalidRateLimit1d'), invalid_rate_limit_7d: t('enterpriseMembers.copy.invalidRateLimit7d'), invalid_opening_used: t('enterpriseMembers.copy.invalidOpeningAmount'), invalid_key_quota: t('enterpriseMembers.copy.invalidKeyQuota'), api_key_invalid_characters: t('enterpriseMembers.copy.apiKeyInvalidCharacters'), invalid_api_key: t('enterpriseMembers.copy.invalidApiKey'), invalid_total_tokens: t('enterpriseMembers.copy.tokenCountMustBeNonnegativeDecimal'), invalid_input_tokens: t('enterpriseMembers.copy.tokenCountMustBeNonnegativeDecimal'), invalid_output_tokens: t('enterpriseMembers.copy.tokenCountMustBeNonnegativeDecimal'), invalid_cache_tokens: t('enterpriseMembers.copy.tokenCountMustBeNonnegativeDecimal'), invalid_cache_creation_tokens: t('enterpriseMembers.copy.tokenCountMustBeNonnegativeDecimal'), invalid_cache_read_tokens: t('enterpriseMembers.copy.tokenCountMustBeNonnegativeDecimal'), key_name_required: t('enterpriseMembers.copy.keyNameIsRequired'), groups_required: t('enterpriseMembers.copy.atLeastOneGroupIsRequired'), member_fields_conflict: t('enterpriseMembers.copy.memberFieldsConflict'), member_identity_ambiguous: t('enterpriseMembers.copy.memberIdentityAmbiguous'), duplicate_member: t('enterpriseMembers.copy.duplicateMemberInMembersSheet'), member_not_found_in_members_sheet: t('enterpriseMembers.copy.memberNotFoundInMembersSheet'), opening_used_only_first_row: t('enterpriseMembers.copy.openingAmountIsAllowedOnlyOnTheFirstMemberRow'), member_code_exists: t('enterpriseMembers.copy.memberCodeAlreadyExists'), api_key_exists: t('enterpriseMembers.copy.apiKeyAlreadyExistsIncludingDeletedRecords'), budget_exhausted_at_import: t('enterpriseMembers.copy.budgetWillBeExhaustedOnImport'), member_code_generated: t('enterpriseMembers.copy.memberCodeGenerated'), key_name_generated: t('enterpriseMembers.copy.keyNameGenerated'), token_total_mismatch: t('enterpriseMembers.copy.tokenTotalMismatch') }[issue] || issue.split('_').join(' '))
}
const maskKey = (value: string) => value.length > 12 ? `${value.slice(0, 6)}…${value.slice(-4)}` : '***'
const splitIPRules = (value: string) => [...new Set(value.split(/[\n,]/).map(item => item.trim()).filter(Boolean))]
const toDateTimeLocal = (value: string | null) => {
  if (!value) return ''
  const date = new Date(value)
  const offset = date.getTimezoneOffset() * 60_000
  return new Date(date.getTime() - offset).toISOString().slice(0, 16)
}
const memberNeedsAccessConfiguration = (member: EnterpriseMember) => !member.deleted_at && member.status === 'disabled' && member.group_ids.length === 0
const statusLabel = (member: EnterpriseMember) => member.deleted_at ? t('enterpriseMembers.copy.archived') : memberNeedsAccessConfiguration(member) ? t('enterpriseMembers.copy.pendingConfiguration') : member.status === 'active' ? t('enterpriseMembers.copy.active') : t('enterpriseMembers.copy.disabled')
const statusClass = (member: EnterpriseMember) => member.deleted_at ? 'bg-stone-100 text-stone-500 dark:bg-white/5' : memberNeedsAccessConfiguration(member) ? 'bg-amber-50 text-amber-700 dark:bg-amber-400/10 dark:text-amber-300' : member.status === 'active' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300' : 'bg-rose-50 text-rose-700 dark:bg-rose-400/10 dark:text-rose-300'

onMounted(loadMembers)
onBeforeUnmount(() => {
  if (importPollTimer) clearTimeout(importPollTimer)
  budgetSessionSeq += 1
  usageRecordsReqSeq += 1
  budgetAnalyticsReqSeq += 1
})
</script>

<style scoped>
.limit-field {
  @apply rounded-xl bg-stone-50 p-3 dark:bg-white/[0.04];
}
</style>
