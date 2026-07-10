<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <div class="flex flex-wrap items-center gap-3">
            <SearchInput
              v-model="filterSearch"
              :placeholder="t('keys.searchPlaceholder')"
              class="w-full sm:w-64"
              @search="onFilterChange"
            />
            <TagFilterSelect
              v-model="filterTags"
              :options="tagFilterOptions"
              :placeholder="t('keys.tagFilterPlaceholder')"
              :empty-label="t('keys.noTagOptions')"
              :clear-label="t('common.reset')"
              :remove-label="t('common.delete')"
              class="w-full sm:w-64"
              @change="onTagFilterChange"
            />
            <Select
              :model-value="filterGroupId"
              class="w-40"
              :options="groupFilterOptions"
              @update:model-value="onGroupFilterChange"
            />
            <Select
              :model-value="filterStatus"
              class="w-40"
              :options="statusFilterOptions"
              @update:model-value="onStatusFilterChange"
            />
          </div>
          <EndpointPopover
            v-if="publicSettings?.api_base_url || (publicSettings?.custom_endpoints?.length ?? 0) > 0"
            :api-base-url="publicSettings?.api_base_url || ''"
            :custom-endpoints="publicSettings?.custom_endpoints || []"
          />
        </div>
      </template>

      <template #actions>
        <div class="flex justify-end gap-3">
          <button
            @click="loadApiKeys"
            :disabled="loading"
            class="btn btn-secondary"
            :title="t('common.refresh')"
          >
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
          <div class="relative" ref="columnDropdownRef">
            <button
              @click="showColumnDropdown = !showColumnDropdown"
              class="btn btn-secondary px-2 md:px-3"
              :title="t('keys.columnSettings')"
            >
              <svg class="h-4 w-4 md:mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.004 3 5.625v12.75c0 .621.504 1.125 1.125 1.125z" />
              </svg>
              <span class="hidden md:inline">{{ t('keys.columnSettings') }}</span>
            </button>
            <div
              v-if="showColumnDropdown"
              class="absolute right-0 top-full z-50 mt-1 max-h-80 w-48 overflow-y-auto rounded-lg border border-stone-200 bg-white py-1 shadow-lg dark:border-white/10 dark:bg-neutral-900"
            >
              <button
                v-for="col in toggleableColumns"
                :key="col.key"
                @click="toggleColumn(col.key)"
                class="flex w-full items-center justify-between px-4 py-2 text-left text-sm text-stone-700 hover:bg-stone-100 dark:text-neutral-300 dark:hover:bg-white/[0.06]"
              >
                <span>{{ col.label }}</span>
                <Icon
                  v-if="isColumnVisible(col.key)"
                  name="check"
                  size="sm"
                  class="text-primary-500"
                  :stroke-width="2"
                />
              </button>
            </div>
          </div>
          <button @click="showBatchCreateModal = true" class="btn btn-secondary">
            <Icon name="copy" size="md" class="mr-2" />
            {{ t('keys.batchCreate.title') }}
          </button>
          <button @click="showCreateModal = true" class="btn btn-primary" data-tour="keys-create-btn">
            <Icon name="plus" size="md" class="mr-2" />
            {{ t('keys.createKey') }}
          </button>
        </div>
      </template>

      <template #table>
        <div
          v-if="selectedKeyCount > 0"
          class="mb-3 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-primary-200 bg-primary-50 px-4 py-3 dark:border-primary-900/50 dark:bg-primary-900/20"
        >
          <div class="text-sm text-primary-800 dark:text-primary-200">
            {{ t('keys.batchActions.selected', { count: selectedKeyCount }) }}
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              type="button"
              class="btn btn-secondary h-10 min-w-[112px] whitespace-nowrap px-4 text-sm"
              @click="openBatchUpdateModal('selected')"
            >
              <Icon name="edit" size="sm" class="shrink-0" />
              <span>{{ t('keys.batchActions.update') }}</span>
            </button>
            <button
              type="button"
              class="btn btn-danger h-10 min-w-[112px] whitespace-nowrap px-4 text-sm"
              @click="openBatchDeleteDialog('selected')"
            >
              <Icon name="trash" size="sm" class="shrink-0" />
              <span>{{ t('keys.batchActions.delete') }}</span>
            </button>
            <button
              type="button"
              class="btn btn-secondary h-10 min-w-[112px] whitespace-nowrap px-4 text-sm"
              @click="clearSelectedKeys"
            >
              <Icon name="x" size="sm" class="shrink-0" />
              <span>{{ t('keys.batchActions.clear') }}</span>
            </button>
          </div>
        </div>
        <DataTable
          :columns="columns"
          :data="apiKeys"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #header-select>
            <button
              type="button"
              role="checkbox"
              :class="selectionCheckboxClasses(allPageKeysSelected || somePageKeysSelected)"
              :aria-checked="somePageKeysSelected && !allPageKeysSelected ? 'mixed' : allPageKeysSelected"
              :aria-label="t('keys.batchActions.selectPage')"
              @click.stop="togglePageSelection"
              @keydown.space.prevent="togglePageSelection"
            >
              <Icon v-if="allPageKeysSelected" name="check" size="xs" :stroke-width="2.5" />
              <span
                v-else-if="somePageKeysSelected"
                class="h-0.5 w-2.5 rounded-full bg-current"
              />
            </button>
          </template>

          <template #cell-select="{ row }">
            <button
              type="button"
              role="checkbox"
              :class="selectionCheckboxClasses(isKeySelected(row.id))"
              :aria-checked="isKeySelected(row.id)"
              :aria-label="t('keys.batchActions.selectOne', { name: selectionLabel(row.name, `#${row.id}`) })"
              @click.stop="toggleKeySelection(row.id)"
              @keydown.space.prevent="toggleKeySelection(row.id)"
            >
              <Icon v-if="isKeySelected(row.id)" name="check" size="xs" :stroke-width="2.5" />
            </button>
          </template>

          <template #cell-key="{ value, row }">
            <div class="flex items-center gap-2">
              <code class="code text-xs">
                {{ maskApiKey(value) }}
              </code>
              <button
                @click="copyToClipboard(value, row.id)"
                class="rounded-lg p-1 transition-colors hover:bg-gray-100 dark:hover:bg-dark-700"
                :class="
                  copiedKeyId === row.id
                    ? 'text-green-500'
                    : 'text-gray-400 hover:text-gray-600 dark:hover:text-gray-300'
                "
                :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
              >
                <Icon
                  v-if="copiedKeyId === row.id"
                  name="check"
                  size="sm"
                  :stroke-width="2"
                />
                <Icon v-else name="clipboard" size="sm" />
              </button>
            </div>
          </template>

          <template #cell-name="{ value, row }">
            <div class="flex items-center gap-1.5">
              <span class="font-medium text-gray-900 dark:text-white">{{ value }}</span>
              <Icon
                v-if="row.ip_whitelist?.length > 0 || row.ip_blacklist?.length > 0"
                name="shield"
                size="sm"
                class="text-blue-500"
                :title="t('keys.ipRestrictionEnabled')"
              />
            </div>
          </template>

          <template #cell-tags="{ row }">
            <TagPills :tags="row.tags" :limit="visibleTagLimit" empty-label="-" class="max-w-[220px]" />
          </template>

          <template #cell-group="{ row }">
            <div class="group/dropdown relative">
              <button
                :ref="(el) => setGroupButtonRef(row.id, el)"
                @click="openGroupSelector(row)"
                class="-mx-2 -my-1 flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1 transition-all duration-200 hover:bg-gray-100 dark:hover:bg-dark-700"
                :title="t('keys.clickToChangeGroup')"
              >
                <GroupBadge
                  v-if="row.group"
                  :name="row.group.name"
                  :platform="row.group.platform"
                  :subscription-type="row.group.subscription_type"
                  :rate-multiplier="row.group.rate_multiplier"
                  :user-rate-multiplier="userGroupRates[row.group.id]"
                  :peak-rate-enabled="row.group.peak_rate_enabled"
                  :peak-start="row.group.peak_start"
                  :peak-end="row.group.peak_end"
                  :peak-rate-multiplier="row.group.peak_rate_multiplier"
                />
                <span v-else class="text-sm text-gray-400 dark:text-dark-500">{{
                  t('keys.noGroup')
                }}</span>
                <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('keys.selectGroup') }}</span>
                <svg
                  class="h-3.5 w-3.5 text-gray-400 opacity-60 transition-opacity group-hover/dropdown:opacity-100"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                  stroke-width="2"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M8.25 15L12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9"
                  />
                </svg>
              </button>
            </div>
          </template>

          <template #cell-current_concurrency="{ value }">
            <span
              :class="[
                'inline-flex min-w-8 items-center justify-center rounded px-2 py-1 text-sm font-semibold tabular-nums',
                (value ?? 0) > 0
                  ? 'bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200 dark:bg-emerald-900/25 dark:text-emerald-300 dark:ring-emerald-800'
                  : 'bg-gray-100 text-gray-500 dark:bg-dark-700 dark:text-dark-400'
              ]"
            >
              {{ value ?? 0 }}
            </span>
          </template>

          <template #cell-usage="{ row }">
            <div class="min-w-[150px] text-sm">
              <div class="flex items-center justify-between gap-2">
                <div>
                  <div class="flex items-center gap-1.5">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('keys.today') }}:</span>
                    <span class="font-medium text-gray-900 dark:text-white">
                      ${{ (usageStats[row.id]?.today_actual_cost ?? 0).toFixed(4) }}
                    </span>
                  </div>
                  <div class="mt-0.5 flex items-center gap-1.5">
                    <span class="text-gray-500 dark:text-gray-400">{{ t('keys.total') }}:</span>
                    <span class="font-medium text-gray-900 dark:text-white">
                      ${{ (usageStats[row.id]?.total_actual_cost ?? 0).toFixed(4) }}
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded text-gray-400 transition-colors hover:bg-primary-50 hover:text-primary-600 dark:hover:bg-primary-900/20 dark:hover:text-primary-400"
                  :title="t('keys.usageDetails.open')"
                  @click.stop="openUsageModal(row)"
                >
                  <Icon name="chart" size="sm" />
                </button>
              </div>
              <!-- Quota progress (if quota is set) -->
              <div v-if="row.quota > 0" class="mt-1.5">
                <div class="flex items-center gap-1.5">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('keys.quota') }}:</span>
                  <span :class="[
                    'font-medium',
                    row.quota_used >= row.quota ? 'text-red-500' :
                    row.quota_used >= row.quota * 0.8 ? 'text-yellow-500' :
                    'text-gray-900 dark:text-white'
                  ]">
                    ${{ row.quota_used?.toFixed(2) || '0.00' }} / ${{ row.quota?.toFixed(2) }}
                  </span>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.quota_used >= row.quota ? 'bg-red-500' :
                      row.quota_used >= row.quota * 0.8 ? 'bg-yellow-500' :
                      'bg-primary-500'
                    ]"
                    :style="{ width: Math.min((row.quota_used / row.quota) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>
          </template>

          <template #cell-rate_limit="{ row }">
            <div v-if="row.rate_limit_5h > 0 || row.rate_limit_1d > 0 || row.rate_limit_7d > 0" class="space-y-1.5 min-w-[140px]">
              <!-- 5h window -->
              <div v-if="row.rate_limit_5h > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">5h</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_5h >= row.rate_limit_5h ? 'text-red-500' :
                    row.usage_5h >= row.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ${{ row.usage_5h?.toFixed(2) || '0.00' }}/${{ row.rate_limit_5h?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_5h >= row.rate_limit_5h ? 'bg-red-500' :
                      row.usage_5h >= row.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_5h / row.rate_limit_5h) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_5h_at && formatResetTime(row.reset_5h_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_5h_at) }}
                </div>
              </div>
              <!-- 1d window -->
              <div v-if="row.rate_limit_1d > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">1d</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_1d >= row.rate_limit_1d ? 'text-red-500' :
                    row.usage_1d >= row.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ${{ row.usage_1d?.toFixed(2) || '0.00' }}/${{ row.rate_limit_1d?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_1d >= row.rate_limit_1d ? 'bg-red-500' :
                      row.usage_1d >= row.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_1d / row.rate_limit_1d) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_1d_at && formatResetTime(row.reset_1d_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_1d_at) }}
                </div>
              </div>
              <!-- 7d window -->
              <div v-if="row.rate_limit_7d > 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-500 dark:text-gray-400">7d</span>
                  <span :class="[
                    'font-medium tabular-nums',
                    row.usage_7d >= row.rate_limit_7d ? 'text-red-500' :
                    row.usage_7d >= row.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                    'text-gray-700 dark:text-gray-300'
                  ]">
                    ${{ row.usage_7d?.toFixed(2) || '0.00' }}/${{ row.rate_limit_7d?.toFixed(2) }}
                  </span>
                </div>
                <div class="h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      row.usage_7d >= row.rate_limit_7d ? 'bg-red-500' :
                      row.usage_7d >= row.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                      'bg-emerald-500'
                    ]"
                    :style="{ width: Math.min((row.usage_7d / row.rate_limit_7d) * 100, 100) + '%' }"
                  />
                </div>
                <div v-if="row.reset_7d_at && formatResetTime(row.reset_7d_at)" class="text-[10px] text-gray-400 dark:text-gray-500 tabular-nums">
                  ⟳ {{ formatResetTime(row.reset_7d_at) }}
                </div>
              </div>
              <!-- Reset button -->
              <button
                v-if="row.usage_5h > 0 || row.usage_1d > 0 || row.usage_7d > 0"
                @click.stop="confirmResetRateLimitFromTable(row)"
                class="mt-0.5 inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-xs text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
                :title="t('keys.resetRateLimitUsage')"
              >
                <Icon name="refresh" size="xs" />
                {{ t('keys.resetUsage') }}
              </button>
            </div>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-expires_at="{ value }">
            <span v-if="value" :class="[
              'text-sm',
              new Date(value) < new Date() ? 'text-red-500 dark:text-red-400' : 'text-gray-500 dark:text-dark-400'
            ]">
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">{{ t('keys.noExpiration') }}</span>
          </template>

          <template #cell-status="{ value, row }">
            <div class="flex flex-col items-start gap-1">
              <span :class="[
                'badge',
                value === 'active' ? 'badge-success' :
                value === 'quota_exhausted' ? 'badge-warning' :
                value === 'expired' ? 'badge-danger' :
                'badge-gray'
              ]">
                {{ t('keys.status.' + value) }}
              </span>
              <span
                v-if="isRateChangedDisabled(row)"
                class="inline-flex rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-medium text-amber-800 dark:bg-amber-900/30 dark:text-amber-200"
              >
                {{ t('keys.systemStatus.rateChangedBadge') }}
              </span>
            </div>
          </template>

          <template #cell-last_used_at="{ value }">
            <span v-if="value" class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-last_used_ip="{ value }">
            <span v-if="value" class="text-sm text-gray-500 dark:text-dark-400">
              {{ value }}
            </span>
            <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <!-- Use Key Button -->
              <button
                @click="openUseKeyModal(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400"
              >
                <Icon name="terminal" size="sm" />
                <span class="text-xs">{{ t('keys.useKey') }}</span>
              </button>
              <!-- Import to CC Switch Button -->
              <button
                v-if="!publicSettings?.hide_ccs_import_button"
                @click="importToCcswitch(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-blue-50 hover:text-blue-600 dark:hover:bg-blue-900/20 dark:hover:text-blue-400"
              >
                <Icon name="upload" size="sm" />
                <span class="text-xs">{{ t('keys.importToCcSwitch') }}</span>
              </button>
              <!-- Toggle Status Button -->
              <button
                v-if="canToggleApiKeyStatus(row.status)"
                @click="toggleKeyStatus(row)"
                :class="[
                  'flex flex-col items-center gap-0.5 rounded-lg p-1.5 transition-colors',
                  row.status === 'active'
                    ? 'text-gray-500 hover:bg-yellow-50 hover:text-yellow-600 dark:hover:bg-yellow-900/20 dark:hover:text-yellow-400'
                    : 'text-gray-500 hover:bg-green-50 hover:text-green-600 dark:hover:bg-green-900/20 dark:hover:text-green-400'
                ]"
              >
                <Icon v-if="row.status === 'active'" name="ban" size="sm" />
                <Icon v-else name="checkCircle" size="sm" />
                <span class="text-xs">{{ getStatusToggleLabel(row) }}</span>
              </button>
              <!-- Edit Button -->
              <button
                @click="editKey(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit') }}</span>
              </button>
              <!-- Delete Button -->
              <button
                @click="confirmDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('keys.noKeysYet')"
              :description="t('keys.createFirstKey')"
              :action-text="t('keys.createKey')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Create/Edit Modal -->
    <BaseDialog
      :show="showCreateModal || showEditModal"
      :title="showEditModal ? t('keys.editKey') : t('keys.createKey')"
      width="normal"
      @close="closeModals"
    >
      <form id="key-form" @submit.prevent="handleSubmit" class="space-y-5">
        <div>
          <label class="input-label">{{ t('keys.nameLabel') }}</label>
          <input
            v-model="formData.name"
            type="text"
            required
            class="input"
            :placeholder="t('keys.namePlaceholder')"
            data-tour="key-form-name"
          />
        </div>

        <div>
          <label class="input-label">{{ t('keys.tagsLabel') }}</label>
          <TagEditor
            v-model="formData.tags"
            :placeholder="t('keys.tagsPlaceholder')"
            :add-placeholder="t('keys.tagsAddPlaceholder')"
            :add-label="t('common.add')"
            :remove-label="t('common.delete')"
            @invalid="handleTagEditorInvalid"
          />
          <p class="input-hint">{{ t('keys.tagsHint') }}</p>
        </div>

        <div>
          <label class="input-label">{{ t('keys.groupLabel') }}</label>
          <Select
            v-model="formData.group_id"
            :options="groupOptions"
            :placeholder="t('keys.selectGroup')"
            :searchable="true"
            :search-placeholder="t('keys.searchGroup')"
            data-tour="key-form-group"
          >
            <template #selected="{ option }">
              <GroupBadge
                v-if="option"
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :user-rate-multiplier="(option as unknown as GroupOption).userRate"
                :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
                :peak-start="(option as unknown as GroupOption).peakStart"
                :peak-end="(option as unknown as GroupOption).peakEnd"
                :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
              />
              <span v-else class="text-gray-400">{{ t('keys.selectGroup') }}</span>
            </template>
            <template #option="{ option, selected }">
              <GroupOptionItem
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :user-rate-multiplier="(option as unknown as GroupOption).userRate"
                :peak-rate-enabled="(option as unknown as GroupOption).peakRateEnabled"
                :peak-start="(option as unknown as GroupOption).peakStart"
                :peak-end="(option as unknown as GroupOption).peakEnd"
                :peak-rate-multiplier="(option as unknown as GroupOption).peakRateMultiplier"
                :description="(option as unknown as GroupOption).description"
                :selected="selected"
              />
            </template>
          </Select>
        </div>

        <!-- Custom Key Section (only for create) -->
        <div v-if="!showEditModal" class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.customKeyLabel') }}</label>
            <button
              type="button"
              @click="formData.use_custom_key = !formData.use_custom_key"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.use_custom_key ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.use_custom_key ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          <div v-if="formData.use_custom_key">
            <input
              v-model="formData.custom_key"
              type="text"
              class="input font-mono"
              :placeholder="t('keys.customKeyPlaceholder')"
              :class="{ 'border-red-500 dark:border-red-500': customKeyError }"
            />
            <p v-if="customKeyError" class="mt-1 text-sm text-red-500">{{ customKeyError }}</p>
            <p v-else class="input-hint">{{ t('keys.customKeyHint') }}</p>
          </div>
        </div>

        <div v-if="showEditModal">
          <label class="input-label">{{ t('keys.statusLabel') }}</label>
          <div v-if="selectedKeySystemStatus" class="space-y-3">
            <div
              :class="[
                'rounded-lg border p-3',
                selectedKeySystemStatus === 'quota_exhausted'
                  ? 'border-yellow-200 bg-yellow-50 text-yellow-900 dark:border-yellow-900/50 dark:bg-yellow-900/20 dark:text-yellow-100'
                  : 'border-red-200 bg-red-50 text-red-900 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-100'
              ]"
            >
              <div class="flex gap-3">
                <Icon
                  :name="selectedKeySystemStatus === 'quota_exhausted' ? 'exclamationTriangle' : 'clock'"
                  size="sm"
                  class="mt-0.5 shrink-0"
                />
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="text-sm font-medium">
                      {{ t(`keys.systemStatus.${selectedKeySystemStatus}.title`) }}
                    </span>
                    <span
                      :class="[
                        'badge text-xs',
                        selectedKeySystemStatus === 'quota_exhausted' ? 'badge-warning' : 'badge-danger'
                      ]"
                    >
                      {{ t(`keys.status.${selectedKeySystemStatus}`) }}
                    </span>
                  </div>
                  <p class="mt-1 text-sm opacity-90">
                    {{ t(`keys.systemStatus.${selectedKeySystemStatus}.description`) }}
                  </p>
                </div>
              </div>
            </div>
            <label class="flex items-start gap-3 rounded-lg border border-gray-200 p-3 dark:border-dark-700">
              <input
                v-model="formData.manually_disable_system_status"
                type="checkbox"
                class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800"
              />
              <span>
                <span class="block text-sm font-medium text-gray-900 dark:text-white">
                  {{ t('keys.systemStatus.manualDisableLabel') }}
                </span>
                <span class="mt-1 block text-sm text-gray-500 dark:text-dark-400">
                  {{ t('keys.systemStatus.manualDisableHint') }}
                </span>
              </span>
            </label>
          </div>
          <div v-else class="space-y-3">
            <div
              v-if="selectedKeyRateChangedDisabled"
              class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-amber-900 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-100"
            >
              <div class="flex gap-3">
                <Icon
                  name="exclamationTriangle"
                  size="sm"
                  class="mt-0.5 shrink-0"
                />
                <div class="min-w-0">
                  <div class="text-sm font-medium">
                    {{ t('keys.systemStatus.rateChangedTitle') }}
                  </div>
                  <p class="mt-1 text-sm opacity-90">
                    {{ t('keys.systemStatus.rateChangedDescription') }}
                  </p>
                </div>
              </div>
            </div>
            <Select
              v-model="formData.status"
              :options="statusOptions"
              :placeholder="t('keys.selectStatus')"
            />
          </div>
        </div>

        <!-- IP Restriction Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.ipRestriction') }}</label>
            <button
              type="button"
              @click="formData.enable_ip_restriction = !formData.enable_ip_restriction"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_ip_restriction ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_ip_restriction ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_ip_restriction" class="space-y-4 pt-2">
            <div>
              <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
              <textarea
                v-model="formData.ip_whitelist"
                rows="3"
                class="input font-mono text-sm"
                :placeholder="t('keys.ipWhitelistPlaceholder')"
              />
              <p class="input-hint">{{ t('keys.ipWhitelistHint') }}</p>
            </div>

            <div>
              <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
              <textarea
                v-model="formData.ip_blacklist"
                rows="3"
                class="input font-mono text-sm"
                :placeholder="t('keys.ipBlacklistPlaceholder')"
              />
              <p class="input-hint">{{ t('keys.ipBlacklistHint') }}</p>
            </div>
          </div>
        </div>

        <!-- Quota Limit Section -->
        <div class="space-y-3">
          <label class="input-label">{{ t('keys.quotaLimit') }}</label>
          <!-- Switch commented out - always show input, 0 = unlimited
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.quotaLimit') }}</label>
            <button
              type="button"
              @click="formData.enable_quota = !formData.enable_quota"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_quota ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_quota ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>
          -->

          <div class="space-y-4">
            <div>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.quota"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="t('keys.quotaAmountPlaceholder')"
                />
              </div>
              <p class="input-hint">{{ t('keys.quotaAmountHint') }}</p>
            </div>

            <!-- Quota used display (only in edit mode) -->
            <div v-if="showEditModal && selectedKey && selectedKey.quota > 0">
              <label class="input-label">{{ t('keys.quotaUsed') }}</label>
              <div class="flex items-center gap-2">
                <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700">
                  <span class="font-medium text-gray-900 dark:text-white">
                    ${{ selectedKey.quota_used?.toFixed(4) || '0.0000' }}
                  </span>
                  <span class="mx-2 text-gray-400">/</span>
                  <span class="text-gray-500 dark:text-gray-400">
                    ${{ selectedKey.quota?.toFixed(2) || '0.00' }}
                  </span>
                </div>
                <button
                  type="button"
                  @click="confirmResetQuota"
                  class="btn btn-secondary text-sm"
                  :title="t('keys.resetQuotaUsed')"
                >
                  {{ t('keys.reset') }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Rate Limit Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.rateLimitSection') }}</label>
            <button
              type="button"
              @click="formData.enable_rate_limit = !formData.enable_rate_limit"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_rate_limit ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_rate_limit ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_rate_limit" class="space-y-4 pt-2">
            <p class="input-hint -mt-2">{{ t('keys.rateLimitHint') }}</p>
            <!-- 5-Hour Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit5h') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.rate_limit_5h"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_5h > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'text-red-500' :
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ${{ selectedKey.usage_5h?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ${{ selectedKey.rate_limit_5h?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h ? 'bg-red-500' :
                      selectedKey.usage_5h >= selectedKey.rate_limit_5h * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_5h / selectedKey.rate_limit_5h) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- Daily Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit1d') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.rate_limit_1d"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_1d > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'text-red-500' :
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ${{ selectedKey.usage_1d?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ${{ selectedKey.rate_limit_1d?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d ? 'bg-red-500' :
                      selectedKey.usage_1d >= selectedKey.rate_limit_1d * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_1d / selectedKey.rate_limit_1d) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- 7-Day Limit -->
            <div>
              <label class="input-label">{{ t('keys.rateLimit7d') }}</label>
              <div class="relative">
                <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                <input
                  v-model.number="formData.rate_limit_7d"
                  type="number"
                  step="0.01"
                  min="0"
                  class="input pl-7"
                  :placeholder="'0'"
                />
              </div>
              <!-- Usage info (edit mode only) -->
              <div v-if="showEditModal && selectedKey && selectedKey.rate_limit_7d > 0" class="mt-2">
                <div class="flex items-center gap-2">
                  <div class="flex-1 rounded-lg bg-gray-100 px-3 py-2 dark:bg-dark-700 text-sm">
                    <span :class="[
                      'font-medium',
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'text-red-500' :
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'text-yellow-500' :
                      'text-gray-900 dark:text-white'
                    ]">
                      ${{ selectedKey.usage_7d?.toFixed(4) || '0.0000' }}
                    </span>
                    <span class="mx-2 text-gray-400">/</span>
                    <span class="text-gray-500 dark:text-gray-400">
                      ${{ selectedKey.rate_limit_7d?.toFixed(2) || '0.00' }}
                    </span>
                  </div>
                </div>
                <div class="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-600">
                  <div
                    :class="[
                      'h-full rounded-full transition-all',
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d ? 'bg-red-500' :
                      selectedKey.usage_7d >= selectedKey.rate_limit_7d * 0.8 ? 'bg-yellow-500' :
                      'bg-green-500'
                    ]"
                    :style="{ width: Math.min((selectedKey.usage_7d / selectedKey.rate_limit_7d) * 100, 100) + '%' }"
                  />
                </div>
              </div>
            </div>

            <!-- Reset Rate Limit button (edit mode only) -->
            <div v-if="showEditModal && selectedKey && (selectedKey.rate_limit_5h > 0 || selectedKey.rate_limit_1d > 0 || selectedKey.rate_limit_7d > 0)">
              <button
                type="button"
                @click="confirmResetRateLimit"
                class="btn btn-secondary text-sm"
              >
                {{ t('keys.resetRateLimitUsage') }}
              </button>
            </div>
          </div>
        </div>

        <!-- Expiration Section -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <label class="input-label mb-0">{{ t('keys.expiration') }}</label>
            <button
              type="button"
              @click="formData.enable_expiration = !formData.enable_expiration"
              :class="[
                'relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none',
                formData.enable_expiration ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600'
              ]"
            >
              <span
                :class="[
                  'pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out',
                  formData.enable_expiration ? 'translate-x-4' : 'translate-x-0'
                ]"
              />
            </button>
          </div>

          <div v-if="formData.enable_expiration" class="space-y-4 pt-2">
            <!-- Quick select buttons (for both create and edit mode) -->
            <div class="flex flex-wrap gap-2">
              <button
                v-for="days in ['7', '30', '90']"
                :key="days"
                type="button"
                @click="setExpirationDays(parseInt(days))"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm transition-colors',
                  formData.expiration_preset === days
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                ]"
              >
                {{ showEditModal ? t('keys.extendDays', { days }) : t('keys.expiresInDays', { days }) }}
              </button>
              <button
                type="button"
                @click="formData.expiration_preset = 'custom'"
                :class="[
                  'rounded-lg px-3 py-1.5 text-sm transition-colors',
                  formData.expiration_preset === 'custom'
                    ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-400'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
                ]"
              >
                {{ t('keys.customDate') }}
              </button>
            </div>

            <!-- Date picker (always show for precise adjustment) -->
            <div>
              <label class="input-label">{{ t('keys.expirationDate') }}</label>
              <input
                v-model="formData.expiration_date"
                type="datetime-local"
                class="input"
              />
              <p class="input-hint">{{ t('keys.expirationDateHint') }}</p>
            </div>

            <!-- Current expiration display (only in edit mode) -->
            <div v-if="showEditModal && selectedKey?.expires_at" class="text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ t('keys.currentExpiration') }}: </span>
              <span class="font-medium text-gray-900 dark:text-white">
                {{ formatDateTime(selectedKey.expires_at) }}
              </span>
            </div>
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeModals" type="button" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
          <button
            form="key-form"
            type="submit"
            :disabled="submitting"
            class="btn btn-primary"
            data-tour="key-form-submit"
          >
            <svg
              v-if="submitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                class="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                stroke-width="4"
              ></circle>
              <path
                class="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            {{
              submitting
                ? t('keys.saving')
                : showEditModal
                  ? t('common.update')
                  : t('common.create')
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showBatchCreateModal"
      :title="t('keys.batchCreate.title')"
      width="wide"
      @close="closeBatchCreateModal"
    >
      <form id="batch-key-form" class="space-y-5" @submit.prevent="handleBatchCreate">
        <div>
          <label class="input-label">{{ t('keys.groupLabel') }}</label>
          <Select
            v-model="batchForm.group_id"
            :options="groupOptions"
            :placeholder="t('keys.selectGroup')"
            :searchable="true"
            :search-placeholder="t('keys.searchGroup')"
          >
            <template #selected="{ option }">
              <GroupBadge
                v-if="option"
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :user-rate-multiplier="(option as unknown as GroupOption).userRate"
              />
              <span v-else class="text-gray-400">{{ t('keys.selectGroup') }}</span>
            </template>
            <template #option="{ option, selected }">
              <GroupOptionItem
                :name="(option as unknown as GroupOption).label"
                :platform="(option as unknown as GroupOption).platform"
                :subscription-type="(option as unknown as GroupOption).subscriptionType"
                :rate-multiplier="(option as unknown as GroupOption).rate"
                :user-rate-multiplier="(option as unknown as GroupOption).userRate"
                :description="(option as unknown as GroupOption).description"
                :selected="selected"
              />
            </template>
          </Select>
        </div>

        <div>
          <label class="input-label">{{ t('keys.tagsLabel') }}</label>
          <TagEditor
            v-model="batchForm.tags"
            :placeholder="t('keys.tagsPlaceholder')"
            :add-placeholder="t('keys.tagsAddPlaceholder')"
            :add-label="t('common.add')"
            :remove-label="t('common.delete')"
            @invalid="handleTagEditorInvalid"
          />
          <p class="input-hint">{{ t('keys.batchCreate.tagsHint') }}</p>
        </div>

        <div class="rounded-lg border border-gray-200 p-4 dark:border-dark-700">
          <div class="mb-4 flex gap-2">
            <button
              type="button"
              :class="[
                'rounded-lg px-3 py-1.5 text-sm transition-colors',
                batchForm.mode === 'template'
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
              ]"
              @click="batchForm.mode = 'template'"
            >
              {{ t('keys.batchCreate.templateMode') }}
            </button>
            <button
              type="button"
              :class="[
                'rounded-lg px-3 py-1.5 text-sm transition-colors',
                batchForm.mode === 'names'
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600'
              ]"
              @click="batchForm.mode = 'names'"
            >
              {{ t('keys.batchCreate.namesMode') }}
            </button>
          </div>

          <div v-if="batchForm.mode === 'template'" class="grid gap-4 md:grid-cols-[1fr_160px]">
            <div>
              <label class="input-label">{{ t('keys.batchCreate.nameTemplate') }}</label>
              <input
                v-model="batchForm.name_template"
                type="text"
                class="input"
                :placeholder="t('keys.batchCreate.nameTemplatePlaceholder')"
              />
              <p class="input-hint">{{ t('keys.batchCreate.nameTemplateHint') }}</p>
            </div>
            <div>
              <label class="input-label">{{ t('keys.batchCreate.count') }}</label>
              <input
                v-model.number="batchForm.count"
                type="number"
                min="1"
                max="500"
                class="input"
              />
            </div>
          </div>

          <div v-else>
            <label class="input-label">{{ t('keys.batchCreate.names') }}</label>
            <textarea
              v-model="batchForm.names"
              rows="8"
              class="input text-sm"
              :placeholder="t('keys.batchCreate.namesPlaceholder')"
            />
            <p class="input-hint">{{ t('keys.batchCreate.namesHint', { count: batchNamesCount }) }}</p>
            <p v-if="batchNameError" class="mt-1 text-xs text-red-500">{{ batchNameError }}</p>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('keys.quotaLimit') }}</label>
            <div class="relative">
              <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
              <input
                v-model.number="batchForm.quota"
                type="number"
                step="0.01"
                min="0"
                class="input pl-7"
                :placeholder="t('keys.quotaAmountPlaceholder')"
              />
            </div>
            <p class="input-hint">{{ t('keys.batchCreate.quotaHint') }}</p>
          </div>
          <div>
            <label class="input-label">{{ t('keys.expiration') }}</label>
            <input
              v-model.number="batchForm.expires_in_days"
              type="number"
              min="0"
              class="input"
              :placeholder="t('keys.batchCreate.expirationPlaceholder')"
            />
            <p class="input-hint">{{ t('keys.batchCreate.expirationHint') }}</p>
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-3">
          <div>
            <label class="input-label">{{ t('keys.rateLimit5h') }}</label>
            <input v-model.number="batchForm.rate_limit_5h" type="number" step="0.01" min="0" class="input" placeholder="0" />
          </div>
          <div>
            <label class="input-label">{{ t('keys.rateLimit1d') }}</label>
            <input v-model.number="batchForm.rate_limit_1d" type="number" step="0.01" min="0" class="input" placeholder="0" />
          </div>
          <div>
            <label class="input-label">{{ t('keys.rateLimit7d') }}</label>
            <input v-model.number="batchForm.rate_limit_7d" type="number" step="0.01" min="0" class="input" placeholder="0" />
          </div>
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('keys.ipWhitelist') }}</label>
            <textarea
              v-model="batchForm.ip_whitelist"
              rows="3"
              class="input font-mono text-sm"
              :placeholder="t('keys.ipWhitelistPlaceholder')"
            />
          </div>
          <div>
            <label class="input-label">{{ t('keys.ipBlacklist') }}</label>
            <textarea
              v-model="batchForm.ip_blacklist"
              rows="3"
              class="input font-mono text-sm"
              :placeholder="t('keys.ipBlacklistPlaceholder')"
            />
          </div>
        </div>
      </form>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeBatchCreateModal">
            {{ t('common.cancel') }}
          </button>
          <button form="batch-key-form" type="submit" :disabled="batchSubmitting" class="btn btn-primary">
            <svg
              v-if="batchSubmitting"
              class="-ml-1 mr-2 h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ batchSubmitting ? t('keys.batchCreate.creating') : t('keys.batchCreate.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showBatchResultModal"
      :title="t('keys.batchCreate.resultTitle')"
      width="extra-wide"
      @close="closeBatchResult"
    >
      <div class="space-y-4">
        <div
          v-if="batchResult && !batchResult.plaintext_available"
          class="rounded-lg border border-yellow-200 bg-yellow-50 p-3 text-sm text-yellow-800 dark:border-yellow-900/50 dark:bg-yellow-900/20 dark:text-yellow-200"
        >
          {{ t('keys.batchCreate.replayWarning') }}
        </div>
        <div
          v-else
          class="rounded-lg border border-primary-200 bg-primary-50 p-3 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-200"
        >
          {{ t('keys.batchCreate.resultHint') }}
        </div>
        <div class="max-h-[420px] overflow-y-auto rounded-lg border border-gray-200 dark:border-dark-700">
          <table class="w-full text-left text-sm">
            <thead class="sticky top-0 bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="px-3 py-2">{{ t('keys.nameLabel') }}</th>
                <th class="px-3 py-2">{{ t('keys.apiKey') }}</th>
                <th class="px-3 py-2">{{ t('keys.tags') }}</th>
                <th class="px-3 py-2">{{ t('keys.group') }}</th>
                <th class="px-3 py-2">{{ t('keys.quota') }}</th>
                <th class="px-3 py-2">{{ t('keys.expiresAt') }}</th>
                <th class="px-3 py-2">{{ t('keys.rateLimitColumn') }}</th>
                <th class="px-3 py-2">{{ t('keys.ipRestriction') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="key in batchResult?.keys || []"
                :key="key.id"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="px-3 py-2 font-medium text-gray-900 dark:text-white">{{ key.name }}</td>
                <td class="px-3 py-2">
                  <div class="flex items-center gap-2">
                    <code class="break-all rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ key.key }}</code>
                    <button
                      type="button"
                      class="rounded-lg p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                      :title="copiedKeyId === key.id ? t('keys.copied') : t('keys.copyToClipboard')"
                      @click="copyToClipboard(key.key, key.id)"
                    >
                      <Icon v-if="copiedKeyId === key.id" name="check" size="sm" class="text-green-500" />
                      <Icon v-else name="clipboard" size="sm" />
                    </button>
                  </div>
                </td>
                <td class="px-3 py-2 text-gray-500 dark:text-dark-400">
                  <TagPills :tags="key.tags" :limit="visibleTagLimit" :empty-label="t('keys.noTags')" class="max-w-[180px]" />
                </td>
                <td class="px-3 py-2 text-gray-500 dark:text-dark-400">
                  {{ key.group?.name || t('keys.noGroup') }}
                </td>
                <td class="px-3 py-2 text-gray-500 dark:text-dark-400">
                  {{ key.quota > 0 ? `$${key.quota.toFixed(2)}` : t('keys.batchCreate.unlimited') }}
                </td>
                <td class="px-3 py-2 text-gray-500 dark:text-dark-400">
                  {{ key.expires_at ? formatDateTime(key.expires_at) : t('keys.noExpiration') }}
                </td>
                <td class="px-3 py-2 text-gray-500 dark:text-dark-400">
                  <div
                    v-if="key.rate_limit_5h > 0 || key.rate_limit_1d > 0 || key.rate_limit_7d > 0"
                    class="space-y-1 whitespace-nowrap text-xs"
                  >
                    <div v-if="key.rate_limit_5h > 0">5h: ${{ key.rate_limit_5h.toFixed(2) }}</div>
                    <div v-if="key.rate_limit_1d > 0">1d: ${{ key.rate_limit_1d.toFixed(2) }}</div>
                    <div v-if="key.rate_limit_7d > 0">7d: ${{ key.rate_limit_7d.toFixed(2) }}</div>
                  </div>
                  <span v-else>{{ t('keys.batchCreate.unlimited') }}</span>
                </td>
                <td class="px-3 py-2 text-gray-500 dark:text-dark-400">
                  <div
                    v-if="key.ip_whitelist?.length || key.ip_blacklist?.length"
                    class="max-w-[260px] space-y-1 text-xs"
                  >
                    <div v-if="key.ip_whitelist?.length">
                      <span class="font-medium text-gray-700 dark:text-dark-200">{{ t('keys.ipWhitelist') }}:</span>
                      {{ key.ip_whitelist.join(', ') }}
                    </div>
                    <div v-if="key.ip_blacklist?.length">
                      <span class="font-medium text-gray-700 dark:text-dark-200">{{ t('keys.ipBlacklist') }}:</span>
                      {{ key.ip_blacklist.join(', ') }}
                    </div>
                  </div>
                  <span v-else>{{ t('keys.batchCreate.unlimited') }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <template #footer>
        <div class="flex flex-wrap justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="copyBatchResult">
            <Icon name="copy" size="sm" class="mr-2" />
            {{ t('keys.batchCreate.copyAll') }}
          </button>
          <button type="button" class="btn btn-secondary" @click="downloadBatchResultCsv">
            <Icon name="download" size="sm" class="mr-2" />
            {{ t('keys.batchCreate.exportCsv') }}
          </button>
          <button type="button" class="btn btn-primary" @click="closeBatchResult">
            {{ t('common.close') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog
      :show="showBatchUpdateModal"
      :title="t('keys.batchActions.updateTitle')"
      width="wide"
      @close="closeBatchUpdateModal"
    >
      <form id="batch-update-key-form" class="space-y-5" @submit.prevent="handleBatchUpdate">
        <div class="rounded-lg border border-primary-200 bg-primary-50 p-3 text-sm text-primary-800 dark:border-primary-900/50 dark:bg-primary-900/20 dark:text-primary-200">
          {{
            batchActionScope === 'filtered'
              ? t('keys.batchActions.updateFilteredHint', { count: batchActionCount })
              : t('keys.batchActions.updateHint', { count: batchActionCount })
          }}
        </div>

        <div class="grid gap-4 md:grid-cols-2">
          <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_group)">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_group"
              :aria-label="t('keys.batchActions.fields.group')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_group), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_group')"
            >
              <Icon v-if="batchUpdateForm.update_group" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.group') }}</div>
              <Select
                v-model="batchUpdateForm.group_id"
                class="mt-2"
                :options="groupOptions"
                :placeholder="t('keys.batchActions.clearGroup')"
                :searchable="true"
                :search-placeholder="t('keys.searchGroup')"
                :disabled="!batchUpdateForm.update_group"
              />
              <button
                v-if="batchUpdateForm.group_id !== null"
                type="button"
                class="mt-2 text-xs text-primary-600 transition hover:text-primary-700 disabled:cursor-not-allowed disabled:opacity-50 dark:text-primary-300"
                :disabled="!batchUpdateForm.update_group"
                @click.prevent="batchUpdateForm.group_id = null"
              >
                {{ t('keys.batchActions.clearGroup') }}
              </button>
            </div>
          </div>

          <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_status)">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_status"
              :aria-label="t('keys.batchActions.fields.status')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_status), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_status')"
            >
              <Icon v-if="batchUpdateForm.update_status" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.status') }}</div>
              <Select
                v-model="batchUpdateForm.status"
                class="mt-2"
                :options="batchUpdateStatusOptions"
                :searchable="false"
                :disabled="!batchUpdateForm.update_status"
              />
            </div>
          </div>
        </div>

        <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_tags)">
          <div class="flex w-full items-start gap-3">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_tags"
              :aria-label="t('keys.batchActions.fields.tags')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_tags), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_tags')"
            >
              <Icon v-if="batchUpdateForm.update_tags" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.tags') }}</div>
              <div class="mt-3 grid gap-3 md:grid-cols-[180px_1fr]">
                <Select
                  v-model="batchUpdateForm.tags_mode"
                  :options="batchUpdateTagModeOptions"
                  :searchable="false"
                  :disabled="!batchUpdateForm.update_tags"
                />
                <div>
                  <TagEditor
                    v-if="batchUpdateForm.tags_mode !== 'clear'"
                    v-model="batchUpdateForm.tags"
                    :placeholder="t('keys.tagsPlaceholder')"
                    :add-placeholder="t('keys.tagsAddPlaceholder')"
                    :add-label="t('common.add')"
                    :remove-label="t('common.delete')"
                    :disabled="!batchUpdateForm.update_tags"
                    @invalid="handleTagEditorInvalid"
                  />
                  <p class="input-hint">
                    {{
                      batchUpdateForm.tags_mode === 'clear'
                        ? t('keys.batchActions.clearTagsHint')
                        : t('keys.batchActions.tagsHint')
                    }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_quota)">
          <div class="flex w-full items-start gap-3">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_quota"
              :aria-label="t('keys.batchActions.fields.quota')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_quota), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_quota')"
            >
              <Icon v-if="batchUpdateForm.update_quota" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.quota') }}</div>
              <div class="mt-3 grid gap-3 md:grid-cols-[180px_1fr]">
                <Select
                  v-model="batchUpdateForm.quota_mode"
                  :options="batchUpdateQuotaModeOptions"
                  :searchable="false"
                  :disabled="!batchUpdateForm.update_quota"
                />
                <div v-if="batchUpdateForm.quota_mode !== 'unlimited'" class="relative">
                  <span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500">$</span>
                  <input v-model.number="batchUpdateForm.quota_value" type="number" min="0" step="0.01" class="input pl-7" :disabled="!batchUpdateForm.update_quota" />
                </div>
              </div>
            </div>
          </div>
        </div>

        <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_expiration)">
          <div class="flex w-full items-start gap-3">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_expiration"
              :aria-label="t('keys.batchActions.fields.expiration')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_expiration), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_expiration')"
            >
              <Icon v-if="batchUpdateForm.update_expiration" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.expiration') }}</div>
              <div class="mt-3 grid gap-3 md:grid-cols-[180px_1fr]">
                <Select
                  v-model="batchUpdateForm.expiration_mode"
                  :options="batchUpdateExpirationModeOptions"
                  :searchable="false"
                  :disabled="!batchUpdateForm.update_expiration"
                />
                <input v-if="batchUpdateForm.expiration_mode === 'set'" v-model="batchUpdateForm.expires_at" type="datetime-local" class="input" :disabled="!batchUpdateForm.update_expiration" />
              </div>
            </div>
          </div>
        </div>

        <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_rate_limit)">
          <div class="flex w-full items-start gap-3">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_rate_limit"
              :aria-label="t('keys.batchActions.fields.rateLimit')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_rate_limit), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_rate_limit')"
            >
              <Icon v-if="batchUpdateForm.update_rate_limit" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.rateLimit') }}</div>
              <div class="mt-3 grid gap-3 md:grid-cols-3">
                <input v-model.number="batchUpdateForm.rate_limit_5h" type="number" min="0" step="0.01" class="input" :placeholder="t('keys.rateLimit5h')" :disabled="!batchUpdateForm.update_rate_limit" />
                <input v-model.number="batchUpdateForm.rate_limit_1d" type="number" min="0" step="0.01" class="input" :placeholder="t('keys.rateLimit1d')" :disabled="!batchUpdateForm.update_rate_limit" />
                <input v-model.number="batchUpdateForm.rate_limit_7d" type="number" min="0" step="0.01" class="input" :placeholder="t('keys.rateLimit7d')" :disabled="!batchUpdateForm.update_rate_limit" />
              </div>
              <div class="mt-3 flex items-center gap-2 text-sm text-gray-600 dark:text-dark-300">
                <button
                  type="button"
                  role="checkbox"
                  :aria-checked="batchUpdateForm.reset_rate_limit_usage"
                  :aria-label="t('keys.batchActions.resetRateUsage')"
                  :class="[selectionCheckboxClasses(batchUpdateForm.reset_rate_limit_usage), 'shrink-0', !batchUpdateForm.update_rate_limit && 'cursor-not-allowed opacity-50']"
                  :disabled="!batchUpdateForm.update_rate_limit"
                  @click="toggleBatchUpdateBoolean('reset_rate_limit_usage')"
                >
                  <Icon v-if="batchUpdateForm.reset_rate_limit_usage" name="check" size="xs" :stroke-width="2.5" />
                </button>
                <span>{{ t('keys.batchActions.resetRateUsage') }}</span>
              </div>
            </div>
          </div>
        </div>

        <div :class="batchUpdateFieldCardClasses(batchUpdateForm.update_ip_access_control)">
          <div class="flex w-full items-start gap-3">
            <button
              type="button"
              role="checkbox"
              :aria-checked="batchUpdateForm.update_ip_access_control"
              :aria-label="t('keys.batchActions.fields.ipAccess')"
              :class="[selectionCheckboxClasses(batchUpdateForm.update_ip_access_control), 'mt-1 shrink-0']"
              @click="toggleBatchUpdateBoolean('update_ip_access_control')"
            >
              <Icon v-if="batchUpdateForm.update_ip_access_control" name="check" size="xs" :stroke-width="2.5" />
            </button>
            <div class="min-w-0 flex-1">
              <div class="text-sm font-medium text-gray-900 dark:text-white">{{ t('keys.batchActions.fields.ipAccess') }}</div>
              <div class="mt-3 grid gap-3 md:grid-cols-2">
                <textarea v-model="batchUpdateForm.ip_whitelist" rows="3" class="input font-mono text-sm" :placeholder="t('keys.ipWhitelistPlaceholder')" :disabled="!batchUpdateForm.update_ip_access_control" />
                <textarea v-model="batchUpdateForm.ip_blacklist" rows="3" class="input font-mono text-sm" :placeholder="t('keys.ipBlacklistPlaceholder')" :disabled="!batchUpdateForm.update_ip_access_control" />
              </div>
            </div>
          </div>
        </div>
      </form>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button type="button" class="btn btn-secondary" @click="closeBatchUpdateModal">
            {{ t('common.cancel') }}
          </button>
          <button form="batch-update-key-form" type="submit" class="btn btn-primary" :disabled="batchActionSubmitting">
            {{ batchActionSubmitting ? t('common.saving') : t('keys.batchActions.updateSubmit') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showBatchDeleteDialog"
      :title="t('keys.batchActions.deleteTitle')"
      :message="
        batchActionScope === 'filtered'
          ? t('keys.batchActions.deleteFilteredConfirm', { count: batchActionCount })
          : t('keys.batchActions.deleteConfirm', { count: batchActionCount })
      "
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleBatchDelete"
      @cancel="showBatchDeleteDialog = false"
    />

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('keys.deleteKey')"
      :message="t('keys.deleteConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Reset Quota Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetQuotaDialog"
      :title="t('keys.resetQuotaTitle')"
      :message="t('keys.resetQuotaConfirmMessage', { name: selectedKey?.name, used: selectedKey?.quota_used?.toFixed(4) })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetQuotaUsed"
      @cancel="showResetQuotaDialog = false"
    />

    <!-- Reset Rate Limit Confirmation Dialog -->
    <ConfirmDialog
      :show="showResetRateLimitDialog"
      :title="t('keys.resetRateLimitTitle')"
      :message="t('keys.resetRateLimitConfirmMessage', { name: selectedKey?.name })"
      :confirm-text="t('keys.reset')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="resetRateLimitUsage"
      @cancel="showResetRateLimitDialog = false"
    />

    <!-- Use Key Modal -->
    <UseKeyModal
      :show="showUseKeyModal"
      :api-key="selectedKey?.key || ''"
      :base-url="publicSettings?.api_base_url || ''"
      :platform="selectedKey?.group?.platform || null"
      :allow-messages-dispatch="selectedKey?.group?.allow_messages_dispatch || false"
      @close="closeUseKeyModal"
    />

    <ApiKeyUsageModal
      :show="showUsageModal"
      :api-key="selectedUsageKey"
      :usage-stats="selectedUsageKey ? usageStats[selectedUsageKey.id] : null"
      @close="closeUsageModal"
    />

    <!-- CCS Client Selection Dialog for Antigravity -->
    <BaseDialog
      :show="showCcsClientSelect"
      :title="t('keys.ccsClientSelect.title')"
      width="narrow"
      @close="closeCcsClientSelect"
    >
      <div class="space-y-4">
        <p class="text-sm text-gray-600 dark:text-gray-400">
          {{ t('keys.ccsClientSelect.description') }}
	        </p>
	        <div class="grid grid-cols-2 gap-3">
	          <button
	            @click="handleCcsClientSelect('claude')"
	            class="flex flex-col items-center gap-2 p-4 rounded-xl border-2 border-gray-200 dark:border-dark-600 hover:border-primary-500 dark:hover:border-primary-500 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-all"
	          >
	            <Icon name="terminal" size="xl" class="text-gray-600 dark:text-gray-400" />
	            <span class="font-medium text-gray-900 dark:text-white">{{
	              t('keys.ccsClientSelect.claudeCode')
	            }}</span>
	            <span class="text-xs text-gray-500 dark:text-gray-400">{{
	              t('keys.ccsClientSelect.claudeCodeDesc')
	            }}</span>
	          </button>
	          <button
	            @click="handleCcsClientSelect('gemini')"
	            class="flex flex-col items-center gap-2 p-4 rounded-xl border-2 border-gray-200 dark:border-dark-600 hover:border-primary-500 dark:hover:border-primary-500 hover:bg-primary-50 dark:hover:bg-primary-900/20 transition-all"
	          >
	            <Icon name="sparkles" size="xl" class="text-gray-600 dark:text-gray-400" />
	            <span class="font-medium text-gray-900 dark:text-white">{{
	              t('keys.ccsClientSelect.geminiCli')
	            }}</span>
	            <span class="text-xs text-gray-500 dark:text-gray-400">{{
	              t('keys.ccsClientSelect.geminiCliDesc')
	            }}</span>
	          </button>
	        </div>
	      </div>
      <template #footer>
        <div class="flex justify-end">
          <button @click="closeCcsClientSelect" class="btn btn-secondary">
            {{ t('common.cancel') }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <!-- Group Selector Dropdown (Teleported to body to avoid overflow clipping) -->
    <Teleport to="body">
      <div
        v-if="groupSelectorKeyId !== null && dropdownPosition"
        ref="dropdownRef"
        class="animate-in fade-in slide-in-from-top-2 fixed z-[100000020] w-max min-w-[380px] overflow-hidden rounded-xl bg-white shadow-lg ring-1 ring-black/5 duration-200 dark:bg-dark-800 dark:ring-white/10"
        style="pointer-events: auto !important;"
        :style="{
          top: dropdownPosition.top !== undefined ? dropdownPosition.top + 'px' : undefined,
          bottom: dropdownPosition.bottom !== undefined ? dropdownPosition.bottom + 'px' : undefined,
          left: dropdownPosition.left + 'px'
        }"
      >
        <!-- Search box -->
        <div class="border-b border-gray-100 p-2 dark:border-dark-700">
          <div class="relative">
            <svg class="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
            <input
              v-model="groupSearchQuery"
              type="text"
              class="w-full rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-8 pr-3 text-sm text-gray-900 placeholder-gray-400 outline-none focus:border-primary-300 focus:ring-1 focus:ring-primary-300 dark:border-dark-600 dark:bg-dark-700 dark:text-white dark:placeholder-gray-500 dark:focus:border-primary-600 dark:focus:ring-primary-600"
              :placeholder="t('keys.searchGroup')"
              @click.stop
            />
          </div>
        </div>
        <!-- Group list -->
        <div class="max-h-80 overflow-y-auto p-1.5">
          <button
            v-for="option in filteredGroupOptions"
            :key="option.value ?? 'null'"
            @click="changeGroup(selectedKeyForGroup!, option.value)"
            :class="[
              'flex w-full items-center justify-between rounded-lg px-3 py-2.5 text-sm transition-colors',
              'border-b border-gray-100 last:border-0 dark:border-dark-700',
              selectedKeyForGroup?.group_id === option.value ||
              (!selectedKeyForGroup?.group_id && option.value === null)
                ? 'bg-primary-50 dark:bg-primary-900/20'
                : 'hover:bg-gray-100 dark:hover:bg-dark-700'
            ]"
            :title="option.description || undefined"
          >
            <GroupOptionItem
              :name="option.label"
              :platform="option.platform"
              :subscription-type="option.subscriptionType"
              :rate-multiplier="option.rate"
              :user-rate-multiplier="option.userRate"
              :peak-rate-enabled="option.peakRateEnabled"
              :peak-start="option.peakStart"
              :peak-end="option.peakEnd"
              :peak-rate-multiplier="option.peakRateMultiplier"
              :description="option.description"
              :selected="
                selectedKeyForGroup?.group_id === option.value ||
                (!selectedKeyForGroup?.group_id && option.value === null)
              "
            />
          </button>
          <!-- Empty state when search has no results -->
          <div v-if="filteredGroupOptions.length === 0" class="py-4 text-center text-sm text-gray-400 dark:text-gray-500">
            {{ t('keys.noGroupFound') }}
          </div>
        </div>
      </div>
    </Teleport>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, type ComponentPublicInstance } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'
import { useClipboard } from '@/composables/useClipboard'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { keysAPI, authAPI, usageAPI, userGroupsAPI } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import TagEditor from '@/components/common/TagEditor.vue'
import TagFilterSelect from '@/components/common/TagFilterSelect.vue'
import TagPills from '@/components/common/TagPills.vue'
import Icon from '@/components/icons/Icon.vue'
import UseKeyModal from '@/components/keys/UseKeyModal.vue'
import ApiKeyUsageModal from '@/components/keys/ApiKeyUsageModal.vue'
import EndpointPopover from '@/components/keys/EndpointPopover.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import type {
  ApiKey,
  BatchApiKeyQuotaMode,
  BatchApiKeyTagsMode,
  BatchCreateApiKeysResponse,
  BatchUpdateApiKeysRequest,
  ApiKeyBatchFilters,
  UpdateApiKeyRequest,
  Group,
  PublicSettings,
  SubscriptionType,
  GroupPlatform
} from '@/types'
import type { Column } from '@/components/common/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import { formatDateTime } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { tableSelectionCheckboxClasses as selectionCheckboxClasses, tableSelectionLabel as selectionLabel } from '@/utils/tableSelectionCheckbox'
import {
  canToggleApiKeyStatus,
  initialApiKeyEditStatus,
  isApiKeySystemStatus,
  shouldPreserveApiKeySystemStatus,
} from '@/utils/apiKeyStatus'
import {
  buildCcSwitchImportDeeplink,
  type CcSwitchClientType
} from '@/utils/ccswitchImport'

const { t } = useI18n()

const API_KEY_DISABLED_REASON_RATE_CHANGED = 'rate_changed'

const isRateChangedDisabled = (key: Pick<ApiKey, 'status' | 'disabled_reason'> | null | undefined): boolean =>
  key?.status === 'disabled' && key.disabled_reason === API_KEY_DISABLED_REASON_RATE_CHANGED

const getStatusToggleLabel = (key: Pick<ApiKey, 'status' | 'disabled_reason'>): string => {
  if (key.status === 'active') {
    return t('keys.disable')
  }
  return isRateChangedDisabled(key) ? t('keys.systemStatus.rateChangedEnableAction') : t('keys.enable')
}

// Helper to format date for datetime-local input
const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

interface GroupOption {
  value: number
  label: string
  description: string | null
  rate: number
  userRate: number | null
  peakRateEnabled: boolean
  peakStart: string
  peakEnd: string
  peakRateMultiplier: number
  subscriptionType: SubscriptionType
  platform: GroupPlatform
}

const appStore = useAppStore()
const onboardingStore = useOnboardingStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

const allColumns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false, class: 'w-12' },
  { key: 'name', label: t('common.name'), sortable: true },
  { key: 'tags', label: t('keys.tags'), sortable: false },
  { key: 'key', label: t('keys.apiKey'), sortable: false },
  { key: 'group', label: t('keys.group'), sortable: false },
  { key: 'current_concurrency', label: t('keys.currentConcurrency'), sortable: true },
  { key: 'usage', label: t('keys.usage'), sortable: false },
  { key: 'rate_limit', label: t('keys.rateLimitColumn'), sortable: false },
  { key: 'expires_at', label: t('keys.expiresAt'), sortable: true },
  { key: 'status', label: t('common.status'), sortable: true },
  { key: 'last_used_at', label: t('keys.lastUsedAt'), sortable: true },
  { key: 'last_used_ip', label: t('keys.lastUsedIP'), sortable: false },
  { key: 'created_at', label: t('keys.created'), sortable: true },
  { key: 'actions', label: t('common.actions'), sortable: false }
])

const ALWAYS_VISIBLE_COLUMNS = new Set(['select', 'name', 'actions'])
const DEFAULT_HIDDEN_COLUMNS = ['rate_limit', 'last_used_at', 'last_used_ip']
const HIDDEN_COLUMNS_KEY = 'api-key-hidden-columns'
const COLUMN_SETTINGS_VERSION_KEY = 'api-key-column-settings-version'
const COLUMN_SETTINGS_VERSION = 2
const VERSION_NEW_HIDDEN_COLUMNS: Record<number, string[]> = {
  2: ['last_used_ip']
}

const toggleableColumns = computed(() =>
  allColumns.value.filter((col) => !ALWAYS_VISIBLE_COLUMNS.has(col.key))
)

const hiddenColumns = reactive<Set<string>>(new Set())

const saveColumnsToStorage = () => {
  try {
    localStorage.setItem(HIDDEN_COLUMNS_KEY, JSON.stringify([...hiddenColumns]))
    localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
  } catch (error) {
    console.error('Failed to save API key table columns:', error)
  }
}

const loadSavedColumns = () => {
  hiddenColumns.clear()
  try {
    const saved = localStorage.getItem(HIDDEN_COLUMNS_KEY)
    if (saved) {
      const parsed = JSON.parse(saved) as string[]
      const validColumnKeys = new Set(allColumns.value.map((col) => col.key))
      parsed
        .filter((key) =>
          typeof key === 'string' &&
          validColumnKeys.has(key) &&
          !ALWAYS_VISIBLE_COLUMNS.has(key)
        )
        .forEach((key) => hiddenColumns.add(key))
      const storedVersion = Number(localStorage.getItem(COLUMN_SETTINGS_VERSION_KEY) ?? '1')
      if (storedVersion < COLUMN_SETTINGS_VERSION) {
        for (let v = storedVersion + 1; v <= COLUMN_SETTINGS_VERSION; v++) {
          for (const key of VERSION_NEW_HIDDEN_COLUMNS[v] ?? []) {
            if (validColumnKeys.has(key) && !ALWAYS_VISIBLE_COLUMNS.has(key)) {
              hiddenColumns.add(key)
            }
          }
        }
        saveColumnsToStorage()
      } else {
        localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
      }
    } else {
      DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
      localStorage.setItem(COLUMN_SETTINGS_VERSION_KEY, String(COLUMN_SETTINGS_VERSION))
    }
  } catch (error) {
    console.error('Failed to load API key table columns:', error)
    DEFAULT_HIDDEN_COLUMNS.forEach((key) => hiddenColumns.add(key))
  }
}

const toggleColumn = (key: string) => {
  if (ALWAYS_VISIBLE_COLUMNS.has(key)) return
  if (hiddenColumns.has(key)) {
    hiddenColumns.delete(key)
  } else {
    hiddenColumns.add(key)
  }
  saveColumnsToStorage()
}

const isColumnVisible = (key: string) => !hiddenColumns.has(key)

const columns = computed<Column[]>(() =>
  allColumns.value.filter((col) => ALWAYS_VISIBLE_COLUMNS.has(col.key) || !hiddenColumns.has(col.key))
)

const apiKeys = ref<ApiKey[]>([])
const groups = ref<Group[]>([])
const loading = ref(false)
const submitting = ref(false)
const now = ref(new Date())
let resetTimer: ReturnType<typeof setInterval> | null = null
const usageStats = ref<Record<string, BatchApiKeyUsageStats>>({})
const userGroupRates = ref<Record<number, number>>({})

const pagination = ref({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = ref({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// Filter state
const filterSearch = ref('')
const filterTags = ref('')
const filterStatus = ref('')
const filterGroupId = ref<string | number>('')

const showCreateModal = ref(false)
const showBatchCreateModal = ref(false)
const showBatchResultModal = ref(false)
const showBatchUpdateModal = ref(false)
const showBatchDeleteDialog = ref(false)
const showEditModal = ref(false)
const showDeleteDialog = ref(false)
const showResetQuotaDialog = ref(false)
const showResetRateLimitDialog = ref(false)
const showUseKeyModal = ref(false)
const showUsageModal = ref(false)
const showCcsClientSelect = ref(false)
const showColumnDropdown = ref(false)
const pendingCcsRow = ref<ApiKey | null>(null)
const selectedKey = ref<ApiKey | null>(null)
const selectedUsageKey = ref<ApiKey | null>(null)
const batchResult = ref<BatchCreateApiKeysResponse | null>(null)
const batchSubmitting = ref(false)
const batchActionSubmitting = ref(false)
const selectedKeyIds = ref<Set<number>>(new Set())
type BatchActionScope = 'selected' | 'filtered'
const batchActionScope = ref<BatchActionScope>('selected')
const knownFilterTags = ref<string[]>([])
const copiedKeyId = ref<number | null>(null)
const groupSelectorKeyId = ref<number | null>(null)
const publicSettings = ref<PublicSettings | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const columnDropdownRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<{ top?: number; bottom?: number; left: number } | null>(null)
const groupButtonRefs = ref<Map<number, HTMLElement>>(new Map())
let abortController: AbortController | null = null

// Get the currently selected key for group change
const selectedKeyForGroup = computed(() => {
  if (groupSelectorKeyId.value === null) return null
  return apiKeys.value.find((k) => k.id === groupSelectorKeyId.value) || null
})

const setGroupButtonRef = (keyId: number, el: Element | ComponentPublicInstance | null) => {
  if (el instanceof HTMLElement) {
    groupButtonRefs.value.set(keyId, el)
  } else {
    groupButtonRefs.value.delete(keyId)
  }
}

const formData = ref({
  name: '',
  tags: '',
  group_id: null as number | null,
  status: 'active' as 'active' | 'disabled',
  manually_disable_system_status: false,
  use_custom_key: false,
  custom_key: '',
  enable_ip_restriction: false,
  ip_whitelist: '',
  ip_blacklist: '',
  // Quota settings (empty = unlimited)
  enable_quota: false,
  quota: null as number | null,
  // Rate limit settings
  enable_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  enable_expiration: false,
  expiration_preset: '30' as '7' | '30' | '90' | 'custom',
  expiration_date: ''
})

const batchForm = ref({
  mode: 'template' as 'template' | 'names',
  count: 10,
  name_template: '成员-{seq}',
  names: '',
  tags: '',
  group_id: null as number | null,
  ip_whitelist: '',
  ip_blacklist: '',
  quota: null as number | null,
  expires_in_days: null as number | null,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null
})

type BatchExpirationMode = 'clear' | 'set'

const defaultBatchUpdateForm = () => ({
  update_group: false,
  group_id: null as number | null,
  update_status: false,
  status: 'active' as 'active' | 'disabled',
  update_quota: false,
  quota_mode: 'set' as BatchApiKeyQuotaMode,
  quota_value: null as number | null,
  update_expiration: false,
  expiration_mode: 'clear' as BatchExpirationMode,
  expires_at: '',
  update_rate_limit: false,
  rate_limit_5h: null as number | null,
  rate_limit_1d: null as number | null,
  rate_limit_7d: null as number | null,
  reset_rate_limit_usage: false,
  update_ip_access_control: false,
  ip_whitelist: '',
  ip_blacklist: '',
  update_tags: false,
  tags_mode: 'add' as BatchApiKeyTagsMode,
  tags: ''
})

const batchUpdateForm = ref(defaultBatchUpdateForm())

const batchNames = computed(() =>
  batchForm.value.names
    .split('\n')
    .map((name) => name.trim())
    .filter((name) => name.length > 0)
)

const batchNamesCount = computed(() => batchNames.value.length)
const apiKeyNameMaxLength = 100

const batchNameError = computed(() => {
  const seen = new Set<string>()
  for (const name of batchNames.value) {
    if (Array.from(name).length > apiKeyNameMaxLength) {
      return t('keys.batchCreate.nameTooLong')
    }
    if (seen.has(name)) {
      return t('keys.batchCreate.namesDuplicate')
    }
    seen.add(name)
  }
  return ''
})

const visibleTagLimit = 2
const apiKeyTagMaxLength = 40
const apiKeyTagsMaxCount = 20

const parseTagList = (text: string): string[] => {
  const seen = new Set<string>()
  const tags: string[] = []
  text
    .split(/[,\n\r，；;]+/)
    .map((tag) => tag.trim().toLowerCase())
    .filter((tag) => tag.length > 0)
    .forEach((tag) => {
      if (!seen.has(tag)) {
        seen.add(tag)
        tags.push(tag)
      }
    })
  return tags
}

const validateTagList = (tags: string[]): boolean => {
  if (tags.length > apiKeyTagsMaxCount) {
    appStore.showError(t('keys.tagsTooMany', { count: apiKeyTagsMaxCount }))
    return false
  }
  if (tags.some((tag) => Array.from(tag).length > apiKeyTagMaxLength)) {
    appStore.showError(t('keys.tagTooLong', { count: apiKeyTagMaxLength }))
    return false
  }
  return true
}

const handleTagEditorInvalid = (reason: 'too_many' | 'too_long') => {
  if (reason === 'too_many') {
    appStore.showError(t('keys.tagsTooMany', { count: apiKeyTagsMaxCount }))
    return
  }
  appStore.showError(t('keys.tagTooLong', { count: apiKeyTagMaxLength }))
}

const normalizeTagOptions = (tags: string[]) => {
  const seen = new Set<string>()
  for (const rawTag of tags) {
    const tag = rawTag.trim().toLowerCase()
    if (tag) seen.add(tag)
  }
  return Array.from(seen).sort((a, b) => a.localeCompare(b))
}

const rememberFilterTags = (keys: ApiKey[]) => {
  const tags = [...knownFilterTags.value]
  for (const key of keys) {
    for (const tag of key.tags || []) {
      tags.push(tag)
    }
  }
  knownFilterTags.value = normalizeTagOptions(tags)
}

const tagFilterOptions = computed(() => {
  const selected = parseTagList(filterTags.value)
  const seen = new Set([...knownFilterTags.value, ...selected])
  return Array.from(seen).sort((a, b) => a.localeCompare(b))
})

const apiKeyFilterBatchMaxCount = 500

const filteredBatchTooLarge = computed(() => pagination.value.total > apiKeyFilterBatchMaxCount)

const buildCurrentBatchFilters = (): ApiKeyBatchFilters | null => {
  const tags = parseTagList(filterTags.value)
  if (!validateTagList(tags)) return null

  const filters: ApiKeyBatchFilters = {}
  const search = filterSearch.value.trim()
  if (search) filters.search = search
  if (tags.length > 0) filters.tags = tags
  if (filterStatus.value) filters.status = filterStatus.value as ApiKey['status']
  if (filterGroupId.value !== '') filters.group_id = Number(filterGroupId.value)

  return Object.keys(filters).length > 0 ? filters : null
}

const selectedKeyIdList = computed(() => Array.from(selectedKeyIds.value))
const selectedKeyCount = computed(() => selectedKeyIds.value.size)
const batchActionCount = computed(() =>
  batchActionScope.value === 'filtered' ? pagination.value.total : selectedKeyCount.value
)
const pageKeyIds = computed(() => apiKeys.value.map((key) => key.id))
const allPageKeysSelected = computed(() =>
  pageKeyIds.value.length > 0 && pageKeyIds.value.every((id) => selectedKeyIds.value.has(id))
)
const somePageKeysSelected = computed(() => pageKeyIds.value.some((id) => selectedKeyIds.value.has(id)))

const isKeySelected = (id: number) => selectedKeyIds.value.has(id)

const batchUpdateFieldCardClasses = (active: boolean) => [
  'flex items-start gap-3 rounded-lg border p-3 transition-colors duration-150',
  active
    ? 'border-primary-300 bg-primary-50/45 dark:border-primary-500/55 dark:bg-primary-900/10'
    : 'border-gray-200 bg-white/40 dark:border-dark-700 dark:bg-black/10'
]

type BatchUpdateBooleanField =
  | 'update_group'
  | 'update_status'
  | 'update_quota'
  | 'update_expiration'
  | 'update_rate_limit'
  | 'reset_rate_limit_usage'
  | 'update_ip_access_control'
  | 'update_tags'

const toggleBatchUpdateBoolean = (field: BatchUpdateBooleanField) => {
  const next = !batchUpdateForm.value[field]
  batchUpdateForm.value[field] = next

  if (field === 'update_rate_limit' && !next) {
    batchUpdateForm.value.reset_rate_limit_usage = false
  }
}

const clearSelectedKeys = () => {
  selectedKeyIds.value = new Set()
}

const toggleKeySelection = (id: number) => {
  const next = new Set(selectedKeyIds.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  selectedKeyIds.value = next
}

const togglePageSelection = () => {
  const next = new Set(selectedKeyIds.value)
  if (allPageKeysSelected.value) {
    for (const id of pageKeyIds.value) next.delete(id)
  } else {
    for (const id of pageKeyIds.value) next.add(id)
  }
  selectedKeyIds.value = next
}

const resetBatchUpdateForm = () => {
  batchUpdateForm.value = defaultBatchUpdateForm()
}

const closeBatchUpdateModal = () => {
  showBatchUpdateModal.value = false
  resetBatchUpdateForm()
}

const ensureFilteredBatchTarget = (): ApiKeyBatchFilters | null => {
  const filters = buildCurrentBatchFilters()
  if (!filters) {
    appStore.showError(t('keys.batchActions.filtersRequired'))
    return null
  }
  if (pagination.value.total <= 0) {
    appStore.showError(t('keys.batchActions.emptyFilteredResult'))
    return null
  }
  if (filteredBatchTooLarge.value) {
    appStore.showError(t('keys.batchActions.filterLimitExceeded', { max: apiKeyFilterBatchMaxCount }))
    return null
  }
  return filters
}

const openBatchUpdateModal = (scope: BatchActionScope = 'selected') => {
  if (scope === 'selected' && selectedKeyCount.value === 0) {
    appStore.showError(t('keys.batchActions.selectRequired'))
    return
  }
  if (scope === 'filtered' && !ensureFilteredBatchTarget()) {
    return
  }
  batchActionScope.value = scope
  resetBatchUpdateForm()
  showBatchUpdateModal.value = true
}

const openBatchDeleteDialog = (scope: BatchActionScope) => {
  if (scope === 'selected' && selectedKeyCount.value === 0) {
    appStore.showError(t('keys.batchActions.selectRequired'))
    return
  }
  if (scope === 'filtered' && !ensureFilteredBatchTarget()) {
    return
  }
  batchActionScope.value = scope
  showBatchDeleteDialog.value = true
}

// 自定义Key验证
const customKeyError = computed(() => {
  if (!formData.value.use_custom_key || !formData.value.custom_key) {
    return ''
  }
  const key = formData.value.custom_key
  if (key.length < 16) {
    return t('keys.customKeyTooShort')
  }
  // 检查字符：只允许字母、数字、下划线、连字符
  if (!/^[a-zA-Z0-9_-]+$/.test(key)) {
    return t('keys.customKeyInvalidChars')
  }
  return ''
})

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'disabled', label: t('keys.status.disabled') }
])

const selectedKeySystemStatus = computed(() => {
  const status = selectedKey.value?.status
  return isApiKeySystemStatus(status) ? status : null
})

const selectedKeyRateChangedDisabled = computed(() => isRateChangedDisabled(selectedKey.value))

const batchUpdateStatusOptions = computed(() => [
  { value: 'active', label: t('keys.status.active') },
  { value: 'disabled', label: t('keys.status.disabled') }
])

const batchUpdateTagModeOptions = computed(() => [
  { value: 'add', label: t('keys.batchActions.tagModes.add') },
  { value: 'set', label: t('keys.batchActions.tagModes.set') },
  { value: 'remove', label: t('keys.batchActions.tagModes.remove') },
  { value: 'clear', label: t('keys.batchActions.tagModes.clear') }
])

const batchUpdateQuotaModeOptions = computed(() => [
  { value: 'set', label: t('keys.batchActions.quotaModes.set') },
  { value: 'add', label: t('keys.batchActions.quotaModes.add') },
  { value: 'unlimited', label: t('keys.batchActions.quotaModes.unlimited') }
])

const batchUpdateExpirationModeOptions = computed(() => [
  { value: 'clear', label: t('keys.batchActions.expirationModes.clear') },
  { value: 'set', label: t('keys.batchActions.expirationModes.set') }
])

// Filter dropdown options
const groupFilterOptions = computed(() => [
  { value: '', label: t('keys.allGroups') },
  { value: 0, label: t('keys.noGroup') },
  ...groups.value.map((g) => ({ value: g.id, label: g.name }))
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('keys.allStatus') },
  { value: 'active', label: t('keys.status.active') },
  { value: 'disabled', label: t('keys.status.disabled') },
  { value: 'quota_exhausted', label: t('keys.status.quota_exhausted') },
  { value: 'expired', label: t('keys.status.expired') }
])

const onFilterChange = () => {
  pagination.value.page = 1
  loadApiKeys()
}

const onTagFilterChange = (tags: string[]) => {
  filterTags.value = tags.join(', ')
  onFilterChange()
}

const onGroupFilterChange = (value: string | number | boolean | null) => {
  filterGroupId.value = value as string | number
  onFilterChange()
}

const onStatusFilterChange = (value: string | number | boolean | null) => {
  filterStatus.value = value as string
  onFilterChange()
}

// Convert groups to Select options format with rate multiplier and subscription type
const groupOptions = computed(() =>
  groups.value.map((group) => ({
    value: group.id,
    label: group.name,
    description: group.description,
    rate: group.rate_multiplier,
    userRate: userGroupRates.value[group.id] ?? null,
    peakRateEnabled: group.peak_rate_enabled,
    peakStart: group.peak_start,
    peakEnd: group.peak_end,
    peakRateMultiplier: group.peak_rate_multiplier,
    subscriptionType: group.subscription_type,
    platform: group.platform
  }))
)

// Group dropdown search
const groupSearchQuery = ref('')
const filteredGroupOptions = computed(() => {
  const query = groupSearchQuery.value.trim().toLowerCase()
  if (!query) return groupOptions.value
  return groupOptions.value.filter((opt) => {
    return opt.label.toLowerCase().includes(query) ||
      (opt.description && opt.description.toLowerCase().includes(query))
  })
})

const copyToClipboard = async (text: string, keyId: number) => {
  const success = await clipboardCopy(text, t('keys.copied'))
  if (success) {
    copiedKeyId.value = keyId
    setTimeout(() => {
      copiedKeyId.value = null
    }, 800)
  }
}

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const { name, code } = error as { name?: string; code?: string }
  return name === 'AbortError' || name === 'CanceledError' || code === 'ERR_CANCELED'
}

const parseIPList = (text: string): string[] =>
  text.split('\n').map(ip => ip.trim()).filter(ip => ip.length > 0)

const loadApiKeys = async () => {
  abortController?.abort()
  const controller = new AbortController()
  abortController = controller
  const { signal } = controller
  loading.value = true
  try {
    // Build filters
    const filters: {
      search?: string
      status?: string
      group_id?: number | string
      tags?: string
      sort_by?: string
      sort_order?: 'asc' | 'desc'
    } = {}
    if (filterSearch.value) filters.search = filterSearch.value
    const tagFilters = parseTagList(filterTags.value)
    if (!validateTagList(tagFilters)) return
    if (tagFilters.length > 0) filters.tags = tagFilters.join(',')
    if (filterStatus.value) filters.status = filterStatus.value
    if (filterGroupId.value !== '') filters.group_id = filterGroupId.value
    filters.sort_by = sortState.value.sort_by
    filters.sort_order = sortState.value.sort_order

    const response = await keysAPI.list(pagination.value.page, pagination.value.page_size, filters, {
      signal
    })
    if (signal.aborted) return
    apiKeys.value = response.items
    rememberFilterTags(response.items)
    clearSelectedKeys()
    pagination.value.total = response.total
    pagination.value.pages = response.pages

    // Load usage stats for all API keys in the list
    if (response.items.length > 0) {
      const keyIds = response.items.map((k) => k.id)
      try {
        const usageResponse = await usageAPI.getDashboardApiKeysUsage(keyIds, { signal })
        if (signal.aborted) return
        usageStats.value = usageResponse.stats
      } catch (e) {
        if (!isAbortError(e)) {
          console.error('Failed to load usage stats:', e)
        }
      }
    } else {
      usageStats.value = {}
    }
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('keys.failedToLoad'))
  } finally {
    if (abortController === controller) {
      loading.value = false
    }
  }
}

const loadApiKeyTagOptions = async () => {
  try {
    const tags = await keysAPI.listTags()
    knownFilterTags.value = normalizeTagOptions(tags)
  } catch (error) {
    console.error('Failed to load API key tag options:', error)
  }
}

const refreshApiKeysAndTagOptions = async () => {
  await Promise.all([
    loadApiKeys(),
    loadApiKeyTagOptions()
  ])
}

const loadGroups = async () => {
  try {
    groups.value = await userGroupsAPI.getAvailable()
  } catch (error) {
    console.error('Failed to load groups:', error)
  }
}

const loadUserGroupRates = async () => {
  try {
    userGroupRates.value = await userGroupsAPI.getUserGroupRates()
  } catch (error) {
    console.error('Failed to load user group rates:', error)
  }
}

const loadPublicSettings = async () => {
  try {
    publicSettings.value = await authAPI.getPublicSettings()
  } catch (error) {
    console.error('Failed to load public settings:', error)
  }
}

const openUseKeyModal = (key: ApiKey) => {
  selectedKey.value = key
  showUseKeyModal.value = true
}

const closeUseKeyModal = () => {
  showUseKeyModal.value = false
  selectedKey.value = null
}

const openUsageModal = (key: ApiKey) => {
  selectedUsageKey.value = key
  showUsageModal.value = true
}

const closeUsageModal = () => {
  showUsageModal.value = false
  selectedUsageKey.value = null
}

const handlePageChange = (page: number) => {
  pagination.value.page = page
  loadApiKeys()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.value.page_size = pageSize
  pagination.value.page = 1
  loadApiKeys()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.value.sort_by = key
  sortState.value.sort_order = order
  pagination.value.page = 1
  loadApiKeys()
}

const editKey = (key: ApiKey) => {
  selectedKey.value = key
  const hasIPRestriction = (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)
  const hasExpiration = !!key.expires_at
  const status = initialApiKeyEditStatus(key.status)
  formData.value = {
    name: key.name,
    tags: (key.tags || []).join(', '),
    group_id: key.group_id,
    status,
    manually_disable_system_status: false,
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: hasIPRestriction,
    ip_whitelist: (key.ip_whitelist || []).join('\n'),
    ip_blacklist: (key.ip_blacklist || []).join('\n'),
    enable_quota: key.quota > 0,
    quota: key.quota > 0 ? key.quota : null,
    enable_rate_limit: (key.rate_limit_5h > 0) || (key.rate_limit_1d > 0) || (key.rate_limit_7d > 0),
    rate_limit_5h: key.rate_limit_5h || null,
    rate_limit_1d: key.rate_limit_1d || null,
    rate_limit_7d: key.rate_limit_7d || null,
    enable_expiration: hasExpiration,
    expiration_preset: 'custom',
    expiration_date: key.expires_at ? formatDateTimeLocal(key.expires_at) : ''
  }
  showEditModal.value = true
}

const toggleKeyStatus = async (key: ApiKey) => {
  if (!canToggleApiKeyStatus(key.status)) {
    return
  }
  const newStatus = key.status === 'active' ? 'disabled' : 'active'
  try {
    await keysAPI.toggleStatus(key.id, newStatus)
    appStore.showSuccess(
      newStatus === 'active' ? t('keys.keyEnabledSuccess') : t('keys.keyDisabledSuccess')
    )
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToUpdateStatus'))
  }
}

const openGroupSelector = (key: ApiKey) => {
  if (groupSelectorKeyId.value === key.id) {
    groupSelectorKeyId.value = null
    dropdownPosition.value = null
  } else {
    const buttonEl = groupButtonRefs.value.get(key.id)
    if (buttonEl) {
      const rect = buttonEl.getBoundingClientRect()
      const dropdownEstHeight = 400 // estimated max dropdown height
      const spaceBelow = window.innerHeight - rect.bottom
      const spaceAbove = rect.top

      if (spaceBelow < dropdownEstHeight && spaceAbove > spaceBelow) {
        // Not enough space below, pop upward
        dropdownPosition.value = {
          bottom: window.innerHeight - rect.top + 4,
          left: rect.left
        }
      } else {
        // Default: pop downward
        dropdownPosition.value = {
          top: rect.bottom + 4,
          left: rect.left
        }
      }
    }
    groupSelectorKeyId.value = key.id
    groupSearchQuery.value = ''
  }
}

const changeGroup = async (key: ApiKey, newGroupId: number | null) => {
  groupSelectorKeyId.value = null
  dropdownPosition.value = null
  if (key.group_id === newGroupId) return

  try {
    await keysAPI.update(key.id, { group_id: newGroupId })
    appStore.showSuccess(t('keys.groupChangedSuccess'))
    loadApiKeys()
  } catch (error) {
    appStore.showError(t('keys.failedToChangeGroup'))
  }
}

const closeGroupSelector = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  // Check if click is inside the dropdown or the trigger button
  if (!target.closest('.group\\/dropdown') && !dropdownRef.value?.contains(target)) {
    groupSelectorKeyId.value = null
    dropdownPosition.value = null
  }
  if (columnDropdownRef.value && !columnDropdownRef.value.contains(target)) {
    showColumnDropdown.value = false
  }
}

const confirmDelete = (key: ApiKey) => {
  selectedKey.value = key
  showDeleteDialog.value = true
}

const closeBatchCreateModal = () => {
  showBatchCreateModal.value = false
}

const closeBatchResult = () => {
  showBatchResultModal.value = false
  batchResult.value = null
}

const resetBatchForm = () => {
  batchForm.value = {
    mode: 'template',
    count: 10,
    name_template: '成员-{seq}',
    names: '',
    tags: '',
    group_id: null,
    ip_whitelist: '',
    ip_blacklist: '',
    quota: null,
    expires_in_days: null,
    rate_limit_5h: null,
    rate_limit_1d: null,
    rate_limit_7d: null
  }
}

const positiveNumberOrUndefined = (value: number | null): number | undefined =>
  value && value > 0 ? value : undefined

const handleBatchCreate = async () => {
  if (batchForm.value.group_id === null) {
    appStore.showError(t('keys.groupRequired'))
    return
  }

  const payload = {
    group_id: batchForm.value.group_id,
    tags: parseTagList(batchForm.value.tags),
    ip_whitelist: parseIPList(batchForm.value.ip_whitelist),
    ip_blacklist: parseIPList(batchForm.value.ip_blacklist),
    quota: positiveNumberOrUndefined(batchForm.value.quota),
    expires_in_days: positiveNumberOrUndefined(batchForm.value.expires_in_days),
    rate_limit_5h: positiveNumberOrUndefined(batchForm.value.rate_limit_5h),
    rate_limit_1d: positiveNumberOrUndefined(batchForm.value.rate_limit_1d),
    rate_limit_7d: positiveNumberOrUndefined(batchForm.value.rate_limit_7d)
  }
  if (!validateTagList(payload.tags)) {
    return
  }

  if (batchForm.value.mode === 'template') {
    const template = batchForm.value.name_template.trim()
    if (!template || !template.includes('{seq}')) {
      appStore.showError(t('keys.batchCreate.templateRequired'))
      return
    }
    if (!batchForm.value.count || batchForm.value.count <= 0) {
      appStore.showError(t('keys.batchCreate.countRequired'))
      return
    }
    const width = Math.max(3, String(batchForm.value.count).length)
    const longestGeneratedName = template.split('{seq}').join(String(batchForm.value.count).padStart(width, '0'))
    if (Array.from(longestGeneratedName).length > apiKeyNameMaxLength) {
      appStore.showError(t('keys.batchCreate.nameTooLong'))
      return
    }
    Object.assign(payload, {
      name_template: template,
      count: batchForm.value.count
    })
  } else {
    if (batchNames.value.length === 0) {
      appStore.showError(t('keys.batchCreate.namesRequired'))
      return
    }
    if (batchNameError.value) {
      appStore.showError(batchNameError.value)
      return
    }
    Object.assign(payload, {
      count: batchNames.value.length,
      names: batchNames.value
    })
  }

  batchSubmitting.value = true
  try {
    batchResult.value = await keysAPI.batchCreate(payload)
    showBatchCreateModal.value = false
    showBatchResultModal.value = true
    resetBatchForm()
    appStore.showSuccess(t('keys.batchCreate.success', { count: batchResult.value.created }))
    await refreshApiKeysAndTagOptions()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || error?.message || t('keys.batchCreate.failed')
    appStore.showError(errorMsg)
  } finally {
    batchSubmitting.value = false
  }
}

const batchResultCsv = computed(() => {
  const rows = batchResult.value?.keys || []
  const headers = [
    'name',
    'key',
    'tags',
    'group',
    'quota',
    'expires_at',
    'rate_limit_5h',
    'rate_limit_1d',
    'rate_limit_7d',
    'ip_whitelist',
    'ip_blacklist'
  ]
  const escapeCsv = (value: unknown) => `"${String(value ?? '').replace(/"/g, '""')}"`
  return [
    headers.map(escapeCsv).join(','),
    ...rows.map((key) => [
      key.name,
      key.key,
      (key.tags || []).join(';'),
      key.group?.name || '',
      key.quota > 0 ? key.quota : '',
      key.expires_at || '',
      key.rate_limit_5h > 0 ? key.rate_limit_5h : '',
      key.rate_limit_1d > 0 ? key.rate_limit_1d : '',
      key.rate_limit_7d > 0 ? key.rate_limit_7d : '',
      (key.ip_whitelist || []).join(';'),
      (key.ip_blacklist || []).join(';')
    ].map(escapeCsv).join(','))
  ].join('\n')
})

const copyBatchResult = async () => {
  if (!batchResult.value) return
  await clipboardCopy(batchResultCsv.value, t('keys.batchCreate.copied'))
}

const downloadBatchResultCsv = () => {
  if (!batchResult.value) return
  const blob = new Blob([batchResultCsv.value], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `api-keys-${new Date().toISOString().slice(0, 10)}.csv`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

const nonNegativeNumber = (value: number | null): number => {
  const n = Number(value ?? 0)
  return Number.isFinite(n) && n > 0 ? n : 0
}

const batchUpdateHasFields = () =>
  batchUpdateForm.value.update_group ||
  batchUpdateForm.value.update_status ||
  batchUpdateForm.value.update_quota ||
  batchUpdateForm.value.update_expiration ||
  batchUpdateForm.value.update_rate_limit ||
  batchUpdateForm.value.update_ip_access_control ||
  batchUpdateForm.value.update_tags

const localDateTimeToISOString = (value: string): string | null => {
  if (!value) return null
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null
  return date.toISOString()
}

const buildBatchTargetPayload = (): Pick<BatchUpdateApiKeysRequest, 'ids' | 'apply_to' | 'filters'> | null => {
  if (batchActionScope.value === 'selected') {
    if (selectedKeyCount.value === 0) {
      appStore.showError(t('keys.batchActions.selectRequired'))
      return null
    }
    return { ids: selectedKeyIdList.value }
  }

  const filters = ensureFilteredBatchTarget()
  if (!filters) return null
  return {
    apply_to: 'filtered',
    filters
  }
}

const handleBatchUpdate = async () => {
  if (batchActionSubmitting.value) return
  const target = buildBatchTargetPayload()
  if (!target) return
  if (!batchUpdateHasFields()) {
    appStore.showError(t('keys.batchActions.noFields'))
    return
  }

  const payload: BatchUpdateApiKeysRequest = {
    ...target
  }

  if (batchUpdateForm.value.update_group) {
    payload.update_group = true
    payload.group_id = batchUpdateForm.value.group_id
  }
  if (batchUpdateForm.value.update_status) {
    payload.update_status = true
    payload.status = batchUpdateForm.value.status
  }
  if (batchUpdateForm.value.update_quota) {
    payload.update_quota = true
    payload.quota_mode = batchUpdateForm.value.quota_mode
    payload.quota_value = batchUpdateForm.value.quota_mode === 'unlimited'
      ? 0
      : nonNegativeNumber(batchUpdateForm.value.quota_value)
    if (batchUpdateForm.value.quota_mode === 'add' && payload.quota_value <= 0) {
      appStore.showError(t('keys.batchActions.addQuotaRequired'))
      return
    }
  }
  if (batchUpdateForm.value.update_expiration) {
    payload.update_expiration = true
    if (batchUpdateForm.value.expiration_mode === 'set') {
      const expiresAt = localDateTimeToISOString(batchUpdateForm.value.expires_at)
      if (!expiresAt) {
        appStore.showError(t('keys.batchActions.expirationRequired'))
        return
      }
      payload.expires_at = expiresAt
    } else {
      payload.expires_at = null
    }
  }
  if (batchUpdateForm.value.update_rate_limit) {
    payload.update_rate_limit = true
    payload.rate_limit_5h = nonNegativeNumber(batchUpdateForm.value.rate_limit_5h)
    payload.rate_limit_1d = nonNegativeNumber(batchUpdateForm.value.rate_limit_1d)
    payload.rate_limit_7d = nonNegativeNumber(batchUpdateForm.value.rate_limit_7d)
  }
  if (batchUpdateForm.value.update_rate_limit && batchUpdateForm.value.reset_rate_limit_usage) {
    payload.reset_rate_limit_usage = true
  }
  if (batchUpdateForm.value.update_ip_access_control) {
    payload.update_ip_access_control = true
    payload.ip_whitelist = parseIPList(batchUpdateForm.value.ip_whitelist)
    payload.ip_blacklist = parseIPList(batchUpdateForm.value.ip_blacklist)
  }
  if (batchUpdateForm.value.update_tags) {
    const tags = parseTagList(batchUpdateForm.value.tags)
    if (!validateTagList(tags)) {
      return
    }
    if (
      (batchUpdateForm.value.tags_mode === 'add' || batchUpdateForm.value.tags_mode === 'remove') &&
      tags.length === 0
    ) {
      appStore.showError(t('keys.batchActions.tagsRequired'))
      return
    }
    payload.update_tags = true
    payload.tags_mode = batchUpdateForm.value.tags_mode
    payload.tags = batchUpdateForm.value.tags_mode === 'clear' ? [] : tags
  }

  batchActionSubmitting.value = true
  try {
    const result = await keysAPI.batchUpdate(payload)
    appStore.showSuccess(t('keys.batchActions.updateSuccess', { count: result.updated }))
    closeBatchUpdateModal()
    await refreshApiKeysAndTagOptions()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || error?.message || t('keys.batchActions.updateFailed')
    appStore.showError(errorMsg)
  } finally {
    batchActionSubmitting.value = false
  }
}

const handleBatchDelete = async () => {
  if (batchActionSubmitting.value) return
  const target = buildBatchTargetPayload()
  if (!target) {
    showBatchDeleteDialog.value = false
    return
  }

  batchActionSubmitting.value = true
  showBatchDeleteDialog.value = false
  try {
    const result = await keysAPI.batchDelete(target)
    appStore.showSuccess(t('keys.batchActions.deleteSuccess', { count: result.deleted }))
    await refreshApiKeysAndTagOptions()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || error?.message || t('keys.batchActions.deleteFailed')
    appStore.showError(errorMsg)
  } finally {
    batchActionSubmitting.value = false
  }
}

const handleSubmit = async () => {
  // Validate group_id is required
  if (formData.value.group_id === null) {
    appStore.showError(t('keys.groupRequired'))
    return
  }

  // Validate custom key if enabled
  if (!showEditModal.value && formData.value.use_custom_key) {
    if (!formData.value.custom_key) {
      appStore.showError(t('keys.customKeyRequired'))
      return
    }
    if (customKeyError.value) {
      appStore.showError(customKeyError.value)
      return
    }
  }

  const ipWhitelist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_whitelist) : []
  const ipBlacklist = formData.value.enable_ip_restriction ? parseIPList(formData.value.ip_blacklist) : []
  const tags = parseTagList(formData.value.tags)
  if (!validateTagList(tags)) {
    return
  }

  // Calculate quota value (null/empty/0 = unlimited, stored as 0)
  const quota = formData.value.quota && formData.value.quota > 0 ? formData.value.quota : 0

  // Calculate expiration
  let expiresInDays: number | undefined
  let expiresAt: string | null | undefined
  if (formData.value.enable_expiration && formData.value.expiration_date) {
    if (!showEditModal.value) {
      // Create mode: calculate days from date
      const expDate = new Date(formData.value.expiration_date)
      const now = new Date()
      const diffDays = Math.ceil((expDate.getTime() - now.getTime()) / (1000 * 60 * 60 * 24))
      expiresInDays = diffDays > 0 ? diffDays : 1
    } else {
      // Edit mode: use custom date directly
      expiresAt = new Date(formData.value.expiration_date).toISOString()
    }
  } else if (showEditModal.value) {
    // Edit mode: if expiration disabled or date cleared, send empty string to clear
    expiresAt = ''
  }

  // Calculate rate limit values (send 0 when toggle is off)
  const rateLimitData = formData.value.enable_rate_limit ? {
    rate_limit_5h: formData.value.rate_limit_5h && formData.value.rate_limit_5h > 0 ? formData.value.rate_limit_5h : 0,
    rate_limit_1d: formData.value.rate_limit_1d && formData.value.rate_limit_1d > 0 ? formData.value.rate_limit_1d : 0,
    rate_limit_7d: formData.value.rate_limit_7d && formData.value.rate_limit_7d > 0 ? formData.value.rate_limit_7d : 0,
  } : { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 }

  submitting.value = true
  try {
    if (showEditModal.value && selectedKey.value) {
      const currentStatus = selectedKey.value.status
      const shouldPreserveSystemStatus = shouldPreserveApiKeySystemStatus(
        currentStatus,
        formData.value.status,
        formData.value.manually_disable_system_status
      )
      const payload: UpdateApiKeyRequest = {
        name: formData.value.name,
        tags,
        ip_whitelist: ipWhitelist,
        ip_blacklist: ipBlacklist,
        quota: quota,
        expires_at: expiresAt,
        rate_limit_5h: rateLimitData.rate_limit_5h,
        rate_limit_1d: rateLimitData.rate_limit_1d,
        rate_limit_7d: rateLimitData.rate_limit_7d,
      }
      if (formData.value.group_id !== selectedKey.value.group_id) {
        payload.group_id = formData.value.group_id
      }
      if (!shouldPreserveSystemStatus) {
        payload.status = formData.value.status
      }
      await keysAPI.update(selectedKey.value.id, payload)
      appStore.showSuccess(t('keys.keyUpdatedSuccess'))
    } else {
      const customKey = formData.value.use_custom_key ? formData.value.custom_key : undefined
      await keysAPI.create(
        formData.value.name,
        formData.value.group_id,
        customKey,
        ipWhitelist,
        ipBlacklist,
        quota,
        expiresInDays,
        rateLimitData,
        tags
      )
      appStore.showSuccess(t('keys.keyCreatedSuccess'))
      // Only advance tour if active, on submit step, and creation succeeded
      if (onboardingStore.isCurrentStep('[data-tour="key-form-submit"]')) {
        onboardingStore.nextStep(500)
      }
    }
    closeModals()
    refreshApiKeysAndTagOptions()
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || error.response?.data?.message || t('keys.failedToSave')
    appStore.showError(errorMsg)
    // Don't advance tour on error
  } finally {
    submitting.value = false
  }
}

/**
 * 处理删除 API Key 的操作
 * 优化：错误处理改进，优先显示后端返回的具体错误消息（如权限不足等），
 * 若后端未返回消息则显示默认的国际化文本
 */
const handleDelete = async () => {
  if (!selectedKey.value) return

  try {
    await keysAPI.delete(selectedKey.value.id)
    appStore.showSuccess(t('keys.keyDeletedSuccess'))
    showDeleteDialog.value = false
    refreshApiKeysAndTagOptions()
  } catch (error: any) {
    // 优先使用后端返回的错误消息，提供更具体的错误信息给用户
    const errorMsg = error?.message || t('keys.failedToDelete')
    appStore.showError(errorMsg)
  }
}

const closeModals = () => {
  showCreateModal.value = false
  showEditModal.value = false
  selectedKey.value = null
  formData.value = {
    name: '',
    tags: '',
    group_id: null,
    status: 'active',
    manually_disable_system_status: false,
    use_custom_key: false,
    custom_key: '',
    enable_ip_restriction: false,
    ip_whitelist: '',
    ip_blacklist: '',
    enable_quota: false,
    quota: null,
    enable_rate_limit: false,
    rate_limit_5h: null,
    rate_limit_1d: null,
    rate_limit_7d: null,
    enable_expiration: false,
    expiration_preset: '30',
    expiration_date: ''
  }
}

// Show reset quota confirmation dialog
const confirmResetQuota = () => {
  showResetQuotaDialog.value = true
}

// Set expiration date based on quick select days
const setExpirationDays = (days: number) => {
  formData.value.expiration_preset = days.toString() as '7' | '30' | '90'
  const expDate = new Date()
  expDate.setDate(expDate.getDate() + days)
  formData.value.expiration_date = formatDateTimeLocal(expDate.toISOString())
}

// Reset quota used for an API key
const resetQuotaUsed = async () => {
  if (!selectedKey.value) return
  showResetQuotaDialog.value = false
  try {
    await keysAPI.update(selectedKey.value.id, { reset_quota: true })
    appStore.showSuccess(t('keys.quotaResetSuccess'))
    // Update local state
    if (selectedKey.value) {
      selectedKey.value.quota_used = 0
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetQuota')
    appStore.showError(errorMsg)
  }
}

// Show reset rate limit confirmation dialog (from edit modal)
const confirmResetRateLimit = () => {
  showResetRateLimitDialog.value = true
}

// Show reset rate limit confirmation dialog (from table row)
const confirmResetRateLimitFromTable = (row: ApiKey) => {
  selectedKey.value = row
  showResetRateLimitDialog.value = true
}

// Reset rate limit usage for an API key
const resetRateLimitUsage = async () => {
  if (!selectedKey.value) return
  showResetRateLimitDialog.value = false
  try {
    await keysAPI.update(selectedKey.value.id, { reset_rate_limit_usage: true })
    appStore.showSuccess(t('keys.rateLimitResetSuccess'))
    // Refresh key data
    await loadApiKeys()
    // Update the editing key with fresh data
    const refreshedKey = apiKeys.value.find(k => k.id === selectedKey.value!.id)
    if (refreshedKey) {
      selectedKey.value = refreshedKey
    }
  } catch (error: any) {
    const errorMsg = error.response?.data?.detail || t('keys.failedToResetRateLimit')
    appStore.showError(errorMsg)
  }
}

const importToCcswitch = (row: ApiKey) => {
  const platform = row.group?.platform || 'anthropic'

  // For antigravity platform, show client selection dialog
  if (platform === 'antigravity') {
    pendingCcsRow.value = row
    showCcsClientSelect.value = true
    return
  }

  // For other platforms, execute directly
  executeCcsImport(row, platform === 'gemini' ? 'gemini' : 'claude')
}

const executeCcsImport = (row: ApiKey, clientType: CcSwitchClientType) => {
  const baseUrl = publicSettings.value?.api_base_url || window.location.origin
  const platform = row.group?.platform || 'anthropic'

  const usageScript = `({
    request: {
      url: "{{baseUrl}}/v1/usage",
      method: "GET",
      headers: { "Authorization": "Bearer {{apiKey}}" }
    },
    extractor: function(response) {
      const remaining = response?.remaining ?? response?.quota?.remaining ?? response?.balance;
      const unit = response?.unit ?? response?.quota?.unit ?? "USD";
      return {
        isValid: response?.is_active ?? response?.isValid ?? true,
        remaining,
        unit
      };
    }
  })`
  const providerName = (publicSettings.value?.site_name || 'sub2api').trim() || 'sub2api'
  const deeplink = buildCcSwitchImportDeeplink({
    baseUrl,
    platform,
    clientType,
    providerName,
    apiKey: row.key,
    usageScript
  })

  try {
    window.open(deeplink, '_self')

    // Check if the protocol handler worked by detecting if we're still focused
    setTimeout(() => {
      if (document.hasFocus()) {
        // Still focused means the protocol handler likely failed
        appStore.showError(t('keys.ccSwitchNotInstalled'))
      }
    }, 100)
  } catch (error) {
    appStore.showError(t('keys.ccSwitchNotInstalled'))
  }
}

const handleCcsClientSelect = (clientType: CcSwitchClientType) => {
  if (pendingCcsRow.value) {
    executeCcsImport(pendingCcsRow.value, clientType)
  }
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

const closeCcsClientSelect = () => {
  showCcsClientSelect.value = false
  pendingCcsRow.value = null
}

function formatResetTime(resetAt: string | null): string {
  if (!resetAt) return ''
  const diff = new Date(resetAt).getTime() - now.value.getTime()
  if (diff <= 0) return t('keys.resetNow')
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.floor((diff % 3600000) / 60000)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

onMounted(() => {
  loadApiKeyTagOptions()
  loadSavedColumns()
  loadApiKeys()
  loadGroups()
  loadUserGroupRates()
  loadPublicSettings()
  document.addEventListener('click', closeGroupSelector)
  resetTimer = setInterval(() => { now.value = new Date() }, 60000)
})

onUnmounted(() => {
  document.removeEventListener('click', closeGroupSelector)
  if (resetTimer) clearInterval(resetTimer)
})
</script>
