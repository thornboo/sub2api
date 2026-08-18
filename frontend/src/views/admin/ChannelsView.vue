<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col justify-between gap-4 lg:flex-row lg:items-start">
          <!-- Left: Search + Filters -->
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-64">
              <Icon
                name="search"
                size="md"
                class="absolute left-3 top-1/2 -translate-y-1/2 text-stone-400 dark:text-stone-500"
              />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('admin.channels.searchChannels', 'Search channels...')"
                class="input pl-10"
                @input="handleSearch"
              />
            </div>

            <Select
              v-model="filters.status"
              :options="statusFilterOptions"
              :placeholder="t('admin.channels.allStatus', 'All Status')"
              class="w-40"
              @change="loadChannels"
            />
          </div>

          <!-- Right: Actions -->
          <div class="flex w-full flex-shrink-0 flex-wrap items-center justify-end gap-3 lg:w-auto">
            <button
              @click="loadChannels"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh', 'Refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button @click="openCreateDialog" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('admin.channels.createChannel', 'Create Channel') }}
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="channels"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-name="{ value }">
            <span class="font-medium text-stone-950 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-description="{ value }">
            <span class="text-sm text-stone-600 dark:text-stone-400">{{ value || '-' }}</span>
          </template>

          <template #cell-status="{ row }">
            <Toggle
              :modelValue="row.status === 'active'"
              @update:modelValue="toggleChannelStatus(row)"
            />
          </template>

          <template #cell-group_count="{ row }">
            <span
              class="inline-flex items-center rounded bg-stone-100 px-2 py-0.5 text-xs font-medium text-stone-800 dark:bg-white/10 dark:text-stone-300"
            >
              {{ (row.group_ids || []).length }}
              {{ t('admin.channels.groupsUnit', 'groups') }}
            </span>
          </template>

          <template #cell-pricing_count="{ row }">
            <span
              class="inline-flex items-center rounded bg-stone-100 px-2 py-0.5 text-xs font-medium text-stone-800 dark:bg-white/10 dark:text-stone-300"
            >
              {{ (row.model_pricing || []).length }}
              {{ t('admin.channels.pricingUnit', 'pricing rules') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-stone-600 dark:text-stone-400">
              {{ formatDate(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-1">
              <button
                @click="openEditDialog(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-stone-500 transition-colors hover:bg-stone-100 hover:text-emerald-600 dark:hover:bg-white/[0.06] dark:hover:text-emerald-300"
              >
                <Icon name="edit" size="sm" />
                <span class="text-xs">{{ t('common.edit', 'Edit') }}</span>
              </button>
              <button
                @click="handleDelete(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-stone-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-500/10 dark:hover:text-red-400"
              >
                <Icon name="trash" size="sm" />
                <span class="text-xs">{{ t('common.delete', 'Delete') }}</span>
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.channels.noChannelsYet', 'No Channels Yet')"
              :description="t('admin.channels.createFirstChannel', 'Create your first channel to manage model pricing')"
              :action-text="t('admin.channels.createChannel', 'Create Channel')"
              @action="openCreateDialog"
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

    <!-- Create/Edit Dialog -->
    <BaseDialog
      :show="showDialog"
      :title="editingChannel ? t('admin.channels.editChannel', 'Edit Channel') : t('admin.channels.createChannel', 'Create Channel')"
      width="extra-wide"
      @close="closeDialog"
    >
      <div class="channel-dialog-body">
        <!-- Tab Bar -->
        <div class="flex items-center border-b border-stone-200/70 dark:border-white/10 flex-shrink-0 -mx-4 sm:-mx-6 px-4 sm:px-6 -mt-3 sm:-mt-4">
          <!-- Basic Settings Tab -->
          <button
            type="button"
            @click="activeTab = 'basic'"
            class="channel-tab"
            :class="activeTab === 'basic' ? 'channel-tab-active' : 'channel-tab-inactive'"
          >
            {{ t('admin.channels.form.basicSettings') }}
          </button>
          <!-- Platform Tabs (only enabled) -->
          <button
            v-for="section in form.platforms.filter(s => s.enabled)"
            :key="section.platform"
            type="button"
            @click="activeTab = section.platform"
            class="channel-tab group"
            :class="activeTab === section.platform ? 'channel-tab-active' : 'channel-tab-inactive'"
          >
            <PlatformIcon :platform="section.platform" size="xs" :class="platformTextClass(section.platform)" />
            <span :class="platformTextClass(section.platform)">{{ t('admin.groups.platforms.' + section.platform, section.platform) }}</span>
          </button>
        </div>

        <!-- Tab Content -->
        <form id="channel-form" @submit.prevent="handleSubmit" class="flex-1 overflow-y-auto pt-4">
          <!-- Basic Settings Tab -->
          <div v-show="activeTab === 'basic'" class="space-y-5">
            <!-- Name -->
            <div>
              <label class="input-label">{{ t('admin.channels.form.name', 'Name') }} <span class="text-red-500">*</span></label>
              <input
                v-model="form.name"
                type="text"
                required
                class="input"
                :placeholder="t('admin.channels.form.namePlaceholder', 'Enter channel name')"
              />
            </div>

            <!-- Description -->
            <div>
              <label class="input-label">{{ t('admin.channels.form.description', 'Description') }}</label>
              <textarea
                v-model="form.description"
                rows="2"
                class="input"
                :placeholder="t('admin.channels.form.descriptionPlaceholder', 'Optional description')"
              ></textarea>
            </div>

            <!-- Status (edit only) -->
            <div v-if="editingChannel">
              <label class="input-label">{{ t('admin.channels.form.status', 'Status') }}</label>
              <Select v-model="form.status" :options="statusEditOptions" />
            </div>

            <!-- Model Restriction -->
            <div>
              <label class="flex items-center gap-2 cursor-pointer">
                <BaseCheckbox
                  v-model="form.restrict_models"
                  :aria-label="t('admin.channels.form.restrictModels', 'Restrict Models')"
                />
                <span class="input-label mb-0">{{ t('admin.channels.form.restrictModels', 'Restrict Models') }}</span>
              </label>
              <p class="mt-1 ml-6 text-xs text-stone-400">
                {{ t('admin.channels.form.restrictModelsHint', 'When enabled, only models in the pricing list are allowed. Others will be rejected.') }}
              </p>
            </div>

            <!-- Billing Basis -->
            <div>
              <label class="input-label">{{ t('admin.channels.form.billingModelSource', 'Billing Basis') }}</label>
              <Select v-model="form.billing_model_source" :options="billingModelSourceOptions" />
              <p class="mt-1 text-xs text-stone-400">
                {{ t('admin.channels.form.billingModelSourceHint', 'Controls which model name is used for pricing lookup') }}
              </p>
            </div>

            <!-- Platform Management -->
            <div class="space-y-3">
              <label class="input-label mb-0">{{ t('admin.channels.form.platformConfig') }}</label>
              <div class="flex flex-wrap gap-2">
                <label
                  v-for="p in platformOrder"
                  :key="p"
                  class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border px-3 py-1.5 text-sm transition-colors"
                  :class="activePlatforms.includes(p)
                    ? 'bg-emerald-50 border-emerald-300 dark:bg-emerald-900/20 dark:border-emerald-700'
                    : 'border-stone-200/70 hover:bg-stone-50/80 dark:border-white/10 dark:hover:bg-white/[0.06]'"
                >
                  <BaseCheckbox
                    :model-value="activePlatforms.includes(p)"
                    :aria-label="t('admin.groups.platforms.' + p, p)"
                    @update:modelValue="togglePlatform(p)"
                  />
                  <PlatformIcon :platform="p" size="xs" :class="platformTextClass(p)" />
                  <span :class="platformTextClass(p)">{{ t('admin.groups.platforms.' + p, p) }}</span>
                </label>
              </div>
            </div>

            <!-- Apply Pricing to Account Stats (toggle only in basic settings) -->
            <div class="border-t border-stone-200/70 pt-4 dark:border-white/10">
              <div class="flex items-center justify-between">
                <div>
                  <label class="text-sm font-medium text-stone-700 dark:text-stone-300">
                    {{ t('admin.channels.form.applyPricingToAccountStats') }}
                  </label>
                  <p class="mt-0.5 text-xs text-stone-500 dark:text-stone-500">
                    {{ t('admin.channels.form.applyPricingToAccountStatsDesc') }}
                  </p>
                </div>
                <Toggle
                  :modelValue="form.apply_pricing_to_account_stats"
                  @update:modelValue="form.apply_pricing_to_account_stats = $event"
                />
              </div>
            </div>
          </div>

          <!-- Platform Tab Content -->
          <div
            v-for="(section, sIdx) in form.platforms"
            :key="'tab-' + section.platform"
            v-show="section.enabled && activeTab === section.platform"
            class="space-y-4"
          >
            <!-- Groups -->
            <div>
              <label class="input-label text-xs">
                {{ t('admin.channels.form.groups', 'Associated Groups') }} <span class="text-red-500">*</span>
                <span v-if="section.group_ids.length > 0" class="ml-1 font-normal text-stone-400">
                  ({{ t('admin.channels.form.selectedCount', { count: section.group_ids.length }) }})
                </span>
              </label>
              <div class="max-h-40 overflow-auto rounded-lg border border-stone-200/70 bg-stone-50/80 p-2 dark:border-white/10 dark:bg-black/30">
                <div v-if="groupsLoading" class="py-2 text-center text-xs text-stone-500">
                  {{ t('common.loading', 'Loading...') }}
                </div>
                <div v-else-if="getGroupsForPlatform(section.platform).length === 0" class="py-2 text-center text-xs text-stone-500">
                  {{ t('admin.channels.form.noGroupsAvailable', 'No groups available') }}
                </div>
                <div v-else class="flex flex-wrap gap-1">
                  <label
                    v-for="group in getGroupsForPlatform(section.platform)"
                    :key="group.id"
                    class="inline-flex cursor-pointer items-center gap-1.5 rounded-md border border-stone-200/70 px-2 py-1 text-xs transition-colors hover:bg-stone-50/80 dark:border-white/10 dark:hover:bg-white/[0.06]"
                    :class="[
                      section.group_ids.includes(group.id) ? 'bg-emerald-50 border-emerald-300 dark:bg-emerald-900/20 dark:border-emerald-700' : '',
                      isGroupInOtherChannel(group.id, section.platform) ? 'opacity-40' : ''
                    ]"
                  >
                    <BaseCheckbox
                      size="sm"
                      :model-value="section.group_ids.includes(group.id)"
                      :disabled="isGroupInOtherChannel(group.id, section.platform)"
                      :aria-label="group.name"
                      @update:modelValue="toggleGroupInSection(sIdx, group.id)"
                    />
                    <span :class="['font-medium', platformTextClass(group.platform)]">{{ group.name }}</span>
                    <span
                      :class="['rounded-full px-1 py-0 text-[10px]', platformBadgeLightClass(group.platform)]"
                    >{{ group.rate_multiplier }}x</span>
                    <span class="text-[10px] text-stone-400">{{ group.account_count || 0 }}</span>
                    <span
                      v-if="isGroupInOtherChannel(group.id, section.platform)"
                      class="text-[10px] text-stone-400"
                    >{{ getGroupInOtherChannelLabel(group.id) }}</span>
                  </label>
                </div>
              </div>
            </div>

            <!-- Web Search Emulation (Anthropic only, hidden when global disabled) -->
            <div v-if="section.platform === 'anthropic' && webSearchGlobalEnabled" class="border-t border-stone-200/70 pt-3 dark:border-white/10">
              <div class="flex items-center justify-between">
                <div>
                  <label class="text-xs font-medium text-stone-700 dark:text-stone-300">
                    {{ t('admin.channels.form.webSearchEmulation') }}
                  </label>
                  <p class="mt-0.5 text-[11px] text-red-500 dark:text-red-400">
                    {{ t('admin.channels.form.webSearchEmulationHint') }}
                  </p>
                </div>
                <Toggle v-model="section.web_search_emulation" />
              </div>
            </div>

            <!-- Codex Image Generation Bridge (OpenAI only) -->
            <div v-if="section.platform === 'openai'" class="border-t border-gray-200 pt-3 dark:border-dark-600">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <label class="text-xs font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.channels.form.codexImageGenerationBridge') }}
                  </label>
                  <p class="mt-0.5 text-[11px] text-amber-600 dark:text-amber-400">
                    {{ t('admin.channels.form.codexImageGenerationBridgeHint') }}
                  </p>
                </div>
                <Toggle v-model="section.codex_image_generation_bridge" />
              </div>
            </div>

            <!-- Bedrock CC Compatibility (Anthropic only) -->
            <div v-if="section.platform === 'anthropic'" class="border-t border-gray-200 pt-3 dark:border-dark-600">
              <div class="flex items-center justify-between gap-4">
                <div>
                  <label class="text-xs font-medium text-gray-700 dark:text-gray-300">
                    {{ t('admin.channels.form.bedrockCCCompat') }}
                  </label>
                  <p class="mt-0.5 text-[11px] text-amber-600 dark:text-amber-400">
                    {{ t('admin.channels.form.bedrockCCCompatHint') }}
                  </p>
                </div>
                <Toggle v-model="section.bedrock_cc_compat" />
              </div>
            </div>

            <!-- Model Mapping -->
            <div>
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                <label class="input-label text-xs mb-0">{{ t('admin.channels.form.modelMapping', 'Model Mapping') }}</label>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <button
                    v-if="section.model_mapping_order.length > 1"
                    type="button"
                    @click="sortMappingNaturally(section)"
                    class="text-xs text-stone-500 hover:text-emerald-600"
                  >
                    {{ t('admin.channels.form.sortByName') }}
                  </button>
                  <button type="button" @click="addMappingEntry(sIdx)" class="text-xs text-emerald-600 hover:text-emerald-700">
                    + {{ t('common.add', 'Add') }}
                  </button>
                </div>
              </div>
              <div
                v-if="Object.keys(section.model_mapping).length === 0"
                class="rounded border border-dashed border-stone-300 p-2 text-center text-xs text-stone-400 dark:border-white/10"
              >
                {{ t('admin.channels.form.noMappingRules', 'No mapping rules. Click "Add" to create one.') }}
              </div>
              <VueDraggable
                v-else
                v-model="section.model_mapping_order"
                :animation="180"
                handle=".mapping-drag-handle"
                class="space-y-1"
                @end="normalizeSectionMappingOrder(section)"
              >
                <div
                  v-for="srcModel in section.model_mapping_order"
                  :key="srcModel"
                  class="flex items-center gap-2 rounded-md border border-transparent px-1 py-0.5 hover:border-stone-200/80 hover:bg-stone-50/60 dark:hover:border-white/10 dark:hover:bg-white/[0.03]"
                >
                  <button
                    type="button"
                    class="mapping-drag-handle cursor-grab p-1 text-stone-300 hover:text-stone-500 active:cursor-grabbing dark:text-stone-600 dark:hover:text-stone-400"
                    :aria-label="t('admin.channels.form.dragToSort')"
                    :title="t('admin.channels.form.dragToSort')"
                  >
                    <span aria-hidden="true" class="grid grid-cols-2 gap-0.5">
                      <span v-for="dot in 6" :key="dot" class="h-0.5 w-0.5 rounded-full bg-current"></span>
                    </span>
                  </button>
                  <input
                    :value="srcModel"
                    type="text"
                    class="input flex-1 text-xs"
                    :class="platformTextClass(section.platform)"
                    :placeholder="t('admin.channels.form.mappingSource', 'Source model')"
                    @change="renameMappingKey(sIdx, srcModel, ($event.target as HTMLInputElement).value)"
                  />
                  <span class="text-stone-400 text-xs">→</span>
                  <input
                    :value="section.model_mapping[srcModel]"
                    type="text"
                    class="input flex-1 text-xs"
                    :class="platformTextClass(section.platform)"
                    :placeholder="t('admin.channels.form.mappingTarget', 'Target model')"
                    @change="updateMappingTarget(sIdx, srcModel, ($event.target as HTMLInputElement).value)"
                  />
                  <span
                    class="hidden min-w-[4.5rem] justify-center rounded-full px-2 py-1 text-[10px] font-medium sm:inline-flex"
                    :class="mappingPricingStatusClass(section, srcModel)"
                  >
                    {{ mappingPricingStatusLabel(section, srcModel) }}
                  </span>
                  <button
                    v-if="mappingPricingStatus(section, srcModel) === 'missing'"
                    type="button"
                    class="rounded px-1.5 py-1 text-[11px] font-medium text-amber-700 hover:bg-amber-50 dark:text-amber-300 dark:hover:bg-amber-500/10"
                    :disabled="pricingRepairingPlatform !== null"
                    @click="repairMappingPricing(sIdx, srcModel)"
                  >
                    {{ t('admin.channels.form.addPricing') }}
                  </button>
                  <button
                    type="button"
                    @click="removeMappingEntry(sIdx, srcModel)"
                    class="rounded p-0.5 text-stone-400 hover:text-red-500"
                  >
                    <Icon name="trash" size="sm" />
                  </button>
                </div>
              </VueDraggable>
            </div>

            <!-- Model Pricing -->
            <div>
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                <label class="input-label text-xs mb-0">{{ t('admin.channels.form.modelPricing', 'Model Pricing') }}</label>
                <div class="flex flex-wrap items-center justify-end gap-2">
                  <button
                    v-if="section.model_pricing.length > 1"
                    type="button"
                    @click="sortPricingByMapping(section)"
                    class="text-xs text-stone-500 hover:text-emerald-600"
                  >
                    {{ t('admin.channels.form.sortByMapping') }}
                  </button>
                  <button
                    v-if="section.model_pricing.length > 1"
                    type="button"
                    @click="sortPricingNaturally(section)"
                    class="text-xs text-stone-500 hover:text-emerald-600"
                  >
                    {{ t('admin.channels.form.sortByName') }}
                  </button>
                  <button
                    type="button"
                    @click="syncLatestModels(sIdx)"
                    :disabled="syncingPlatform === section.platform"
                    class="text-xs text-stone-500 hover:text-emerald-600 disabled:opacity-50"
                  >
                    {{ syncingPlatform === section.platform ? t('admin.channels.form.syncingModels') : t('admin.channels.form.syncLatestModels') }}
                  </button>
                  <button type="button" @click="addPricingEntry(sIdx)" class="text-xs text-emerald-600 hover:text-emerald-700">
                    + {{ t('common.add', 'Add') }}
                  </button>
                </div>
              </div>
              <div
                v-if="Object.keys(section.model_mapping).length > 0"
                class="mb-2 rounded-lg border border-stone-200/80 bg-stone-50/70 px-3 py-2.5 dark:border-white/10 dark:bg-white/[0.03]"
              >
                <div class="flex flex-wrap items-center justify-between gap-2">
                  <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px]">
                    <span class="font-medium text-stone-600 dark:text-stone-300">
                      {{ t('admin.channels.form.mappingModelsCount', { count: pricingCoverage(section).expectedModels.length }) }}
                    </span>
                    <span class="text-emerald-600 dark:text-emerald-400">
                      {{ t('admin.channels.form.pricingCoveredCount', { count: pricingCoverage(section).coveredModels.length }) }}
                    </span>
                    <span
                      :class="pricingCoverage(section).missingModels.length > 0
                        ? 'font-medium text-amber-700 dark:text-amber-300'
                        : 'text-stone-400'"
                    >
                      {{ t('admin.channels.form.pricingMissingCount', { count: pricingCoverage(section).missingModels.length }) }}
                    </span>
                    <span class="text-stone-400">
                      {{ t('admin.channels.form.pricingExtraCount', { count: pricingCoverage(section).extraPricingModels.length }) }}
                    </span>
                  </div>
                  <div class="flex items-center gap-2">
                    <button
                      v-if="pricingCoverage(section).missingModels.length > 0"
                      type="button"
                      class="text-[11px] font-medium text-stone-500 hover:text-amber-700 dark:text-stone-400 dark:hover:text-amber-300"
                      @click="section.show_missing_models = !section.show_missing_models"
                    >
                      {{ section.show_missing_models ? t('admin.channels.form.hideMissingModels') : t('admin.channels.form.showMissingModels') }}
                    </button>
                    <button
                      v-if="pricingCoverage(section).missingModels.length > 0"
                      type="button"
                      class="rounded-md bg-emerald-600 px-2.5 py-1 text-[11px] font-medium text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-50"
                      :disabled="pricingRepairingPlatform !== null"
                      :title="t('admin.channels.form.quickPricingHint')"
                      @click="quickPriceMissingModels(sIdx)"
                    >
                      {{ pricingRepairingPlatform === section.platform
                        ? t('admin.channels.form.quickPricingRunning')
                        : t('admin.channels.form.quickPricing') }}
                    </button>
                  </div>
                </div>
                <p
                  v-if="pricingCoverage(section).indeterminate"
                  class="mt-1.5 text-[11px] text-amber-700 dark:text-amber-300"
                >
                  {{ t('admin.channels.form.upstreamPricingCoverageUnknown') }}
                </p>
                <div
                  v-if="section.show_missing_models && pricingCoverage(section).missingModels.length > 0"
                  class="mt-2 flex flex-wrap gap-1.5 border-t border-stone-200/80 pt-2 dark:border-white/10"
                >
                  <span
                    v-for="model in pricingCoverage(section).missingModels"
                    :key="model"
                    class="rounded bg-amber-100 px-1.5 py-0.5 font-mono text-[11px] text-amber-800 dark:bg-amber-500/10 dark:text-amber-300"
                  >
                    {{ model }}
                  </span>
                </div>
              </div>
              <div
                v-if="deliveryError || deliveryWarnings.length"
                class="mb-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300"
              >
                {{ deliveryError || t('admin.channels.form.deliveryEvidenceWarning') }}
              </div>
              <div
                v-if="section.model_pricing.length === 0"
                class="rounded border border-dashed border-stone-300 p-2 text-center text-xs text-stone-400 dark:border-white/10"
              >
                {{ t('admin.channels.form.noPricingRules', 'No pricing rules yet. Click "Add" to create one.') }}
              </div>
              <VueDraggable
                v-else
                v-model="section.model_pricing"
                :animation="180"
                handle=".pricing-drag-handle"
                class="space-y-2"
                @end="renumberPricing(section)"
              >
                <PricingEntryCard
                  v-for="(entry, idx) in section.model_pricing"
                  :key="entry._ui_id || `pricing-${idx}`"
                  :entry="entry"
                  :platform="section.platform"
                  :model-delivery="deliveryLoaded ? modelDeliveryForPlatform(section.platform) : undefined"
                  :delivery-loading="deliveryLoading"
                  @update="updatePricingEntry(sIdx, idx, $event)"
                  @remove="removePricingEntry(sIdx, idx)"
                  @inspect-delivery="openDeliveryDialog"
                />
              </VueDraggable>
            </div>

            <!-- Account Stats Pricing Rules (per-platform, always visible) -->
            <div class="mt-4 border-t border-stone-200/70 pt-4 dark:border-white/10 space-y-3">
              <div class="flex items-center justify-between">
                <h4 class="text-sm font-medium text-stone-700 dark:text-stone-300">
                  {{ t('admin.channels.form.accountStatsPricingRules') }}
                </h4>
                <button
                  type="button"
                  @click="addAccountStatsRule(sIdx)"
                  class="rounded-lg border border-emerald-300 px-3 py-1 text-xs font-medium text-emerald-600 hover:bg-emerald-50 dark:border-emerald-600 dark:text-emerald-400 dark:hover:bg-emerald-900/20"
                >
                  + {{ t('admin.channels.form.addRule') }}
                </button>
              </div>

              <!-- Filter rules for this platform's groups -->
              <p
                v-if="section.account_stats_pricing_rules.length === 0"
                class="text-xs italic text-stone-400 dark:text-stone-500"
              >
                {{ t('admin.channels.form.noRulesConfigured') }}
              </p>

              <div
                v-for="(rule, ruleIndex) in section.account_stats_pricing_rules"
                :key="ruleIndex"
                class="space-y-3 rounded-lg border border-stone-200/70 p-4 dark:border-white/10"
              >
                <div class="flex items-center justify-between">
                  <input
                    v-model="rule.name"
                    :placeholder="t('admin.channels.form.ruleName')"
                    class="bg-transparent text-sm font-medium text-stone-700 placeholder-stone-400 outline-none dark:text-stone-300"
                  />
                  <button type="button" @click="removeAccountStatsRule(sIdx, ruleIndex)" class="text-xs text-red-500 hover:text-red-700">
                    {{ t('common.delete') }}
                  </button>
                </div>

                <div>
                  <label class="text-xs text-stone-500 dark:text-stone-500">{{ t('admin.channels.form.ruleGroups') }}</label>
                  <div class="mt-1 flex flex-wrap gap-1">
                    <label
                      v-for="gid in section.group_ids"
                      :key="gid"
                      class="inline-flex cursor-pointer items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors"
                      :class="rule.group_ids.includes(gid)
                        ? 'border-emerald-300 bg-emerald-50 dark:border-emerald-700 dark:bg-emerald-900/20'
                        : 'border-stone-200/70 hover:bg-stone-50/80 dark:border-white/10 dark:hover:bg-white/[0.06]'"
                    >
                      <BaseCheckbox
                        size="sm"
                        :model-value="rule.group_ids.includes(gid)"
                        :aria-label="getGroupNameById(gid)"
                        @update:modelValue="toggleRuleGroup(rule, gid)"
                      />
                      <span :class="['font-medium', platformTextClass(section.platform)]">{{ getGroupNameById(gid) }}</span>
                    </label>
                  </div>
                  <p v-if="section.group_ids.length === 0" class="mt-1 text-xs text-stone-400">
                    {{ t('admin.channels.form.noGroupsInChannel') }}
                  </p>
                </div>

                <div>
                  <label class="text-xs text-stone-500 dark:text-stone-500">{{ t('admin.channels.form.ruleAccounts') }}</label>
                  <!-- Selected account chips -->
                  <div class="mt-1 flex flex-wrap gap-1">
                    <span
                      v-for="accountId in rule.account_ids"
                      :key="accountId"
                      class="inline-flex items-center gap-1 rounded-md border border-emerald-300 bg-emerald-50 px-2 py-0.5 text-xs dark:border-emerald-700 dark:bg-emerald-900/20"
                    >
                      <span :class="['font-medium', platformTextClass(section.platform)]">{{ getRuleAccountLabel(accountId) }}</span>
                      <button type="button" @click="removeRuleAccount(rule, accountId)" class="text-stone-400 hover:text-red-500">
                        <Icon name="x" size="xs" />
                      </button>
                    </span>
                  </div>
                  <!-- Account search input -->
                  <div class="relative mt-1 rule-account-search-container">
                    <input
                      v-model="ruleAccountSearchKeyword[`${section.platform}-${ruleIndex}`]"
                      type="text"
                      class="input text-sm"
                      :placeholder="t('admin.channels.form.searchAccountPlaceholder')"
                      @input="onRuleAccountSearchInput(section.platform, ruleIndex)"
                      @focus="onRuleAccountSearchFocus(section.platform, ruleIndex)"
                    />
                    <!-- Search results dropdown -->
                    <div
                      v-if="showRuleAccountDropdown[`${section.platform}-${ruleIndex}`] && (ruleAccountSearchResults[`${section.platform}-${ruleIndex}`]?.length ?? 0) > 0"
                      class="absolute z-50 mt-1 max-h-48 w-full overflow-auto rounded-lg border bg-white shadow-lg dark:border-white/10 dark:bg-neutral-950"
                    >
                      <button
                        v-for="account in ruleAccountSearchResults[`${section.platform}-${ruleIndex}`]"
                        :key="account.id"
                        type="button"
                        @click="selectRuleAccount(rule, account, section.platform, ruleIndex)"
                        class="w-full px-3 py-2 text-left text-sm hover:bg-stone-100 dark:hover:bg-white/[0.06]"
                        :class="{ 'opacity-50': rule.account_ids.includes(account.id) }"
                        :disabled="rule.account_ids.includes(account.id)"
                      >
                        <span :class="platformTextClass(account.platform)">{{ account.name }}</span>
                        <span class="ml-2 text-xs text-stone-400">#{{ account.id }}</span>
                      </button>
                    </div>
                  </div>
                  <p class="mt-1 text-xs text-stone-400">
                    {{ t('admin.channels.form.ruleAccountsHint') }}
                  </p>
                </div>

                <div>
                  <div class="mb-1 flex items-center justify-between">
                    <label class="text-xs text-stone-500 dark:text-stone-500">{{ t('admin.channels.form.ruleModelPricing') }}</label>
                    <button type="button" @click="addRulePricingEntry(sIdx, ruleIndex)" class="text-xs text-emerald-600 hover:text-emerald-700">
                      + {{ t('common.add') }}
                    </button>
                  </div>
                  <div v-if="rule.pricing.length === 0" class="rounded border border-dashed border-stone-300 p-2 text-center text-xs text-stone-400 dark:border-white/10">
                    {{ t('admin.channels.form.noPricingRules') }}
                  </div>
                  <div v-else class="space-y-2">
                    <PricingEntryCard
                      v-for="(entry, pIdx) in rule.pricing"
                      :key="pIdx"
                      :entry="entry"
                      :platform="section.platform"
                      hide-time-pricing
                      @update="rule.pricing.splice(pIdx, 1, $event)"
                      @remove="removeRulePricingEntry(sIdx, ruleIndex, pIdx)"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </form>
      </div>

      <template #footer>
        <div class="flex justify-end gap-3">
          <button @click="closeDialog" type="button" class="btn btn-secondary">
            {{ t('common.cancel', 'Cancel') }}
          </button>
          <button
            type="submit"
            form="channel-form"
            :disabled="submitting"
            class="btn btn-primary"
          >
            {{ submitting
              ? t('common.submitting', 'Submitting...')
              : editingChannel
                ? t('common.update', 'Update')
                : t('common.create', 'Create')
            }}
          </button>
        </div>
      </template>
    </BaseDialog>

    <ChannelModelDeliveryDialog
      :show="showDeliveryDialog"
      :model="selectedDeliveryModel"
      @close="closeDeliveryDialog"
    />

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.channels.deleteChannel', 'Delete Channel')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete', 'Delete')"
      :cancel-text="t('common.cancel', 'Cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type { Channel, ChannelModelDelivery, ChannelModelPricing, CreateChannelRequest, UpdateChannelRequest, AccountStatsPricingRule, ModelDefaultPricing } from '@/api/admin/channels'
import type { PricingFormEntry } from '@/components/admin/channel/types'
import {
  mTokToPerToken,
  perTokenToMTok,
  apiIntervalsToForm,
  formIntervalsToAPI,
  apiTimePricingToForm,
  formTimePricingToAPI,
  findModelConflict,
  validateIntervals,
  validateTimePricing,
} from '@/components/admin/channel/types'
import { derivePricingCoverage, naturalModelCompare, normalizeMappingOrder, patternCovers, pricingCoverageSeverity, pricingModelForMapping } from '@/components/admin/channel/pricingCoverage'
import type { PricingCoverage } from '@/components/admin/channel/pricingCoverage'
import type { AdminGroup, GroupPlatform } from '@/types'
import type { Column } from '@/components/common/types'
import { platformTextClass, platformBadgeLightClass } from '@/utils/platformColors'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import BaseCheckbox from '@/components/common/BaseCheckbox.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Toggle from '@/components/common/Toggle.vue'
import PricingEntryCard from '@/components/admin/channel/PricingEntryCard.vue'
import ChannelModelDeliveryDialog from '@/components/admin/channel/ChannelModelDeliveryDialog.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useKeyedDebouncedSearch } from '@/composables/useKeyedDebouncedSearch'
import { VueDraggable } from 'vue-draggable-plus'

const { t } = useI18n()
const appStore = useAppStore()

// Web Search global enabled state (loaded once on mount)
const webSearchGlobalEnabled = ref(false)
async function loadWebSearchGlobalState() {
  try {
    const cfg = await adminAPI.settings.getWebSearchEmulationConfig()
    webSearchGlobalEnabled.value = cfg?.enabled === true && (cfg?.providers?.length ?? 0) > 0
  } catch (err: unknown) {
    console.warn('Failed to load web search global state:', err)
    webSearchGlobalEnabled.value = false
  }
}

// ── Form-level pricing rule type (per-platform) ──
interface FormPricingRule {
  name: string
  group_ids: number[]
  account_ids: number[]
  pricing: PricingFormEntry[]
}

// ── Platform Section type ──
interface PlatformSection {
  platform: GroupPlatform
  enabled: boolean
  collapsed: boolean
  group_ids: number[]
  model_mapping: Record<string, string>
  model_mapping_order: string[]
  model_pricing: PricingFormEntry[]
  show_missing_models: boolean
  web_search_emulation: boolean
  codex_image_generation_bridge: boolean
  bedrock_cc_compat: boolean
  account_stats_pricing_rules: FormPricingRule[]
}

// ── Table columns ──
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channels.columns.name', 'Name'), sortable: true },
  { key: 'description', label: t('admin.channels.columns.description', 'Description'), sortable: false },
  { key: 'status', label: t('admin.channels.columns.status', 'Status'), sortable: true },
  { key: 'group_count', label: t('admin.channels.columns.groups', 'Groups'), sortable: false },
  { key: 'pricing_count', label: t('admin.channels.columns.pricing', 'Pricing'), sortable: false },
  { key: 'created_at', label: t('admin.channels.columns.createdAt', 'Created'), sortable: true },
  { key: 'actions', label: t('admin.channels.columns.actions', 'Actions'), sortable: false }
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.channels.allStatus', 'All Status') },
  { value: 'active', label: t('admin.channels.statusActive', 'Active') },
  { value: 'disabled', label: t('admin.channels.statusDisabled', 'Disabled') }
])

const statusEditOptions = computed(() => [
  { value: 'active', label: t('admin.channels.statusActive', 'Active') },
  { value: 'disabled', label: t('admin.channels.statusDisabled', 'Disabled') }
])

const billingModelSourceOptions = computed(() => [
  { value: 'channel_mapped', label: t('admin.channels.form.billingModelSourceChannelMapped', 'Bill by channel-mapped model') },
  { value: 'requested', label: t('admin.channels.form.billingModelSourceRequested', 'Bill by requested model') },
  { value: 'upstream', label: t('admin.channels.form.billingModelSourceUpstream', 'Bill by final upstream model') },
  { value: 'response_model', label: t('admin.channels.form.billingModelSourceResponse', 'Bill by upstream response model') }
])

// ── State ──
const channels = ref<Channel[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({ status: '' })
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// Dialog state
const showDialog = ref(false)
const editingChannel = ref<Channel | null>(null)
const submitting = ref(false)
const showDeleteDialog = ref(false)
const deletingChannel = ref<Channel | null>(null)
const activeTab = ref<string>('basic')
const deliveryLoading = ref(false)
const deliveryLoaded = ref(false)
const deliveryError = ref('')
const deliveryWarnings = ref<string[]>([])
const channelModelDelivery = ref<ChannelModelDelivery[]>([])
const showDeliveryDialog = ref(false)
const selectedDeliveryModel = ref<ChannelModelDelivery | null>(null)

// Groups
const allGroups = ref<AdminGroup[]>([])
const groupsLoading = ref(false)

// All channels for group-conflict detection (independent of current page)
const allChannelsForConflict = ref<Channel[]>([])

// Form data
const form = reactive({
  name: '',
  description: '',
  status: 'active',
  restrict_models: false,
  billing_model_source: 'channel_mapped' as string,
  platforms: [] as PlatformSection[],
  apply_pricing_to_account_stats: false,
})

let abortController: AbortController | null = null

// ── Platform config ──
const platformOrder: GroupPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek']
// composite 分组仅覆盖主平台（与后端 isConcreteRequestPlatform / composite-routes target_platform 一致），
// 不含国产供应商平台。
const compositePlatforms: GroupPlatform[] = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok']

// ── Helpers ──
function formatDate(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleDateString()
}

// ── Platform section helpers ──
const activePlatforms = computed(() => form.platforms.filter(s => s.enabled).map(s => s.platform))

const deliveryByPlatform = computed(() => {
  const result: Record<string, Record<string, ChannelModelDelivery>> = {}
  for (const delivery of channelModelDelivery.value) {
    const platform = delivery.platform || ''
    if (!result[platform]) result[platform] = {}
    result[platform][delivery.name.trim().toLowerCase()] = delivery
  }
  return result
})

function modelDeliveryForPlatform(platform: string): Record<string, ChannelModelDelivery> {
  return deliveryByPlatform.value[platform] || {}
}

function openDeliveryDialog(delivery: ChannelModelDelivery) {
  selectedDeliveryModel.value = delivery
  showDeliveryDialog.value = true
}

function closeDeliveryDialog() {
  showDeliveryDialog.value = false
  selectedDeliveryModel.value = null
}

function addPlatformSection(platform: GroupPlatform) {
  form.platforms.push({
    platform,
    enabled: true,
    collapsed: false,
    group_ids: [],
    model_mapping: {},
    model_mapping_order: [],
    model_pricing: [],
    show_missing_models: false,
    web_search_emulation: false,
    codex_image_generation_bridge: false,
    bedrock_cc_compat: false,
    account_stats_pricing_rules: [],
  })
}

function togglePlatform(platform: GroupPlatform) {
  const section = form.platforms.find(s => s.platform === platform)
  if (section) {
    section.enabled = !section.enabled
    if (!section.enabled && activeTab.value === platform) {
      activeTab.value = 'basic'
    }
  } else {
    addPlatformSection(platform)
  }
}

function getGroupsForPlatform(platform: GroupPlatform): AdminGroup[] {
  return allGroups.value.filter(
    g => g.platform === platform || (g.platform === 'composite' && compositePlatforms.includes(platform))
  )
}

// ── Group helpers ──
const groupToChannelMap = computed(() => {
  const map = new Map<number, Channel>()
  for (const ch of allChannelsForConflict.value) {
    if (editingChannel.value && ch.id === editingChannel.value.id) continue
    for (const gid of ch.group_ids || []) {
      map.set(gid, ch)
    }
  }
  return map
})

function isGroupInOtherChannel(groupId: number, _platform: string): boolean {
  return groupToChannelMap.value.has(groupId)
}

function getGroupChannelName(groupId: number): string {
  return groupToChannelMap.value.get(groupId)?.name || ''
}

function getGroupInOtherChannelLabel(groupId: number): string {
  const name = getGroupChannelName(groupId)
  return t('admin.channels.form.inOtherChannel', { name }, `In "${name}"`)
}

const deleteConfirmMessage = computed(() => {
  const name = deletingChannel.value?.name || ''
  return t(
    'admin.channels.deleteConfirm',
    { name },
    `Are you sure you want to delete channel "${name}"? This action cannot be undone.`
  )
})

function toggleGroupInSection(sectionIdx: number, groupId: number) {
  const section = form.platforms[sectionIdx]
  const idx = section.group_ids.indexOf(groupId)
  if (idx >= 0) {
    section.group_ids.splice(idx, 1)
  } else {
    section.group_ids.push(groupId)
  }
}

function toggleRuleGroup(rule: FormPricingRule, groupId: number) {
  const idx = rule.group_ids.indexOf(groupId)
  if (idx >= 0) {
    rule.group_ids.splice(idx, 1)
  } else {
    rule.group_ids.push(groupId)
  }
}

// ── Pricing helpers ──
let pricingUISequence = 0

function nextPricingUIId(): string {
  pricingUISequence += 1
  return `pricing-${pricingUISequence}`
}

function createPricingEntry(models: string[] = [], defaults?: ModelDefaultPricing): PricingFormEntry {
  return {
    _ui_id: nextPricingUIId(),
    sort_order: 0,
    models,
    billing_mode: 'token',
    input_price: defaults?.found ? perTokenToMTok(defaults.input_price ?? null) : null,
    output_price: defaults?.found ? perTokenToMTok(defaults.output_price ?? null) : null,
    cache_write_price: defaults?.found ? perTokenToMTok(defaults.cache_write_price ?? null) : null,
    cache_read_price: defaults?.found ? perTokenToMTok(defaults.cache_read_price ?? null) : null,
    image_input_price: defaults?.found ? perTokenToMTok(defaults.image_input_price ?? null) : null,
    image_output_price: defaults?.found ? perTokenToMTok(defaults.image_output_price ?? null) : null,
    per_request_price: null,
    intervals: [],
    time_pricing: undefined,
    self_check_enabled_models: [],
  }
}

function renumberPricing(section: PlatformSection) {
  section.model_pricing.forEach((entry, index) => {
    entry.sort_order = index
    if (!entry._ui_id) entry._ui_id = nextPricingUIId()
  })
}

function addPricingEntry(sectionIdx: number) {
  const section = form.platforms[sectionIdx]
  section.model_pricing.push(createPricingEntry())
  renumberPricing(section)
}

const syncingPlatform = ref<string | null>(null)
const pricingRepairingPlatform = ref<string | null>(null)

const pricingCoverageByPlatform = computed(() => {
  const coverage = new Map<GroupPlatform, PricingCoverage>()
  for (const section of form.platforms) {
    coverage.set(section.platform, derivePricingCoverage(
      section.model_mapping,
      section.model_mapping_order,
      section.model_pricing,
      form.billing_model_source,
    ))
  }
  return coverage
})

function pricingCoverage(section: PlatformSection): PricingCoverage {
  return pricingCoverageByPlatform.value.get(section.platform) ?? {
    expectedModels: [],
    coveredModels: [],
    missingModels: [],
    extraPricingModels: [],
    indeterminate: false,
  }
}

function defaultPricingSignature(model: string, pricing: ModelDefaultPricing): string {
  if (!pricing.found) return `missing:${model.toLocaleLowerCase()}`
  return JSON.stringify({
    input_price: pricing.input_price ?? null,
    output_price: pricing.output_price ?? null,
    cache_write_price: pricing.cache_write_price ?? null,
    cache_read_price: pricing.cache_read_price ?? null,
    image_input_price: pricing.image_input_price ?? null,
    image_output_price: pricing.image_output_price ?? null,
  })
}

async function loadDefaultPricing(models: string[]) {
  const resolved: Array<{ model: string, pricing: ModelDefaultPricing }> = []
  const concurrency = 6
  for (let start = 0; start < models.length; start += concurrency) {
    const batch = models.slice(start, start + concurrency)
    const batchResult = await Promise.all(batch.map(async model => {
      try {
        const pricing = await adminAPI.channels.getModelDefaultPricing(model)
        return { model, pricing }
      } catch {
        return { model, pricing: { found: false } as ModelDefaultPricing }
      }
    }))
    resolved.push(...batchResult)
  }
  return resolved
}

async function repairPricingModels(sectionIdx: number, requestedModels: string[]) {
  const section = form.platforms[sectionIdx]
  if (!section || pricingRepairingPlatform.value) return
  if (form.billing_model_source === 'upstream') {
    appStore.showWarning(t('admin.channels.form.upstreamPricingCoverageUnknown'))
    return
  }

  const missingKeys = new Set(pricingCoverage(section).missingModels.map(model => model.toLocaleLowerCase()))
  const models = requestedModels.filter(model => missingKeys.has(model.toLocaleLowerCase()))
  if (models.length === 0) return

  pricingRepairingPlatform.value = section.platform
  try {
    const resolved = await loadDefaultPricing(models)
    if (form.platforms[sectionIdx] !== section) return

    const stillMissing = new Set(pricingCoverage(section).missingModels.map(model => model.toLocaleLowerCase()))
    const groups = new Map<string, { models: string[], pricing: ModelDefaultPricing }>()
    for (const item of resolved) {
      if (!stillMissing.has(item.model.toLocaleLowerCase())) continue
      const signature = defaultPricingSignature(item.model, item.pricing)
      const group = groups.get(signature)
      if (group) {
        group.models.push(item.model)
      } else {
        groups.set(signature, { models: [item.model], pricing: item.pricing })
      }
    }

    let addedCount = 0
    for (const group of groups.values()) {
      section.model_pricing.push(createPricingEntry(group.models, group.pricing))
      addedCount += group.models.length
    }
    renumberPricing(section)
    if (addedCount > 0) {
      appStore.showSuccess(t('admin.channels.form.quickPricingSuccess', { count: addedCount }))
    }
  } finally {
    if (pricingRepairingPlatform.value === section.platform) {
      pricingRepairingPlatform.value = null
    }
  }
}

async function quickPriceMissingModels(sectionIdx: number) {
  const section = form.platforms[sectionIdx]
  await repairPricingModels(sectionIdx, pricingCoverage(section).missingModels)
}

async function repairMappingPricing(sectionIdx: number, sourceModel: string) {
  const section = form.platforms[sectionIdx]
  const expected = pricingModelForMapping(
    sourceModel,
    section.model_mapping[sourceModel] || '',
    form.billing_model_source,
  )
  if (expected) await repairPricingModels(sectionIdx, [expected])
}

function sortPricingNaturally(section: PlatformSection) {
  section.model_pricing.sort((left, right) =>
    naturalModelCompare(left.models[0] || '', right.models[0] || ''),
  )
  renumberPricing(section)
}

function sortPricingByMapping(section: PlatformSection) {
  const expected = pricingCoverage(section).expectedModels
  const orderIndex = (entry: PricingFormEntry): number => {
    const indexes = expected
      .map((model, index) => entry.models.some(pricingModel => patternCovers(pricingModel, model)) ? index : -1)
      .filter(index => index >= 0)
    return indexes.length > 0 ? Math.min(...indexes) : Number.MAX_SAFE_INTEGER
  }
  section.model_pricing.sort((left, right) => {
    const byMapping = orderIndex(left) - orderIndex(right)
    if (byMapping !== 0) return byMapping
    return naturalModelCompare(left.models[0] || '', right.models[0] || '')
  })
  renumberPricing(section)
}

async function syncLatestModels(sectionIdx: number) {
  const platform = form.platforms[sectionIdx].platform
  if (syncingPlatform.value) return
  syncingPlatform.value = platform
  try {
    const result = await adminAPI.channels.syncPricingModels(platform)
    // Collect all model names already present in this platform's pricing entries
    const existingModels = new Set<string>()
    for (const entry of form.platforms[sectionIdx].model_pricing) {
      for (const m of entry.models) existingModels.add(m)
    }
    const newModels = result.models.filter(m => !existingModels.has(m))
    if (newModels.length === 0) {
      appStore.showSuccess(t('admin.channels.form.syncModelsAlreadyUpToDate'))
      return
    }
    // Add new models as a single new pricing entry (user fills in prices)
    form.platforms[sectionIdx].model_pricing.push(createPricingEntry(newModels))
    renumberPricing(form.platforms[sectionIdx])
    appStore.showSuccess(t('admin.channels.form.syncModelsSuccess', { count: newModels.length }))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channels.form.syncModelsError')))
  } finally {
    syncingPlatform.value = null
  }
}

function updatePricingEntry(sectionIdx: number, idx: number, updated: PricingFormEntry) {
  form.platforms[sectionIdx].model_pricing.splice(idx, 1, updated)
  renumberPricing(form.platforms[sectionIdx])
}

function removePricingEntry(sectionIdx: number, idx: number) {
  form.platforms[sectionIdx].model_pricing.splice(idx, 1)
  renumberPricing(form.platforms[sectionIdx])
}

// ── Model Mapping helpers ──
function addMappingEntry(sectionIdx: number) {
  const section = form.platforms[sectionIdx]
  const mapping = section.model_mapping
  let key = ''
  let i = 1
  while (key === '' || key in mapping) {
    key = `model-${i}`
    i++
  }
  mapping[key] = ''
  section.model_mapping_order.push(key)
}

function removeMappingEntry(sectionIdx: number, key: string) {
  const section = form.platforms[sectionIdx]
  delete section.model_mapping[key]
  section.model_mapping_order = section.model_mapping_order.filter(model => model !== key)
}

function renameMappingKey(sectionIdx: number, oldKey: string, newKey: string) {
  newKey = newKey.trim()
  if (!newKey || newKey === oldKey) return
  const section = form.platforms[sectionIdx]
  const mapping = section.model_mapping
  if (newKey in mapping) return
  const value = mapping[oldKey]
  delete mapping[oldKey]
  mapping[newKey] = value
  const orderIndex = section.model_mapping_order.indexOf(oldKey)
  if (orderIndex >= 0) section.model_mapping_order.splice(orderIndex, 1, newKey)
  normalizeSectionMappingOrder(section)
}

function updateMappingTarget(sectionIdx: number, sourceModel: string, targetModel: string) {
  const section = form.platforms[sectionIdx]
  section.model_mapping[sourceModel] = targetModel.trim()
}

function normalizeSectionMappingOrder(section: PlatformSection) {
  section.model_mapping_order = normalizeMappingOrder(section.model_mapping, section.model_mapping_order)
}

function sortMappingNaturally(section: PlatformSection) {
  section.model_mapping_order = Object.keys(section.model_mapping).sort(naturalModelCompare)
}

function mappingPricingStatus(section: PlatformSection, sourceModel: string): 'covered' | 'missing' | 'unknown' | 'pending' {
  if (form.billing_model_source === 'upstream') return 'unknown'
  const expected = pricingModelForMapping(
    sourceModel,
    section.model_mapping[sourceModel] || '',
    form.billing_model_source,
  )
  if (!expected) return 'pending'
  const pricingModels = section.model_pricing.flatMap(entry => entry.models)
  return pricingModels.some(model => patternCovers(model, expected)) ? 'covered' : 'missing'
}

function mappingPricingStatusLabel(section: PlatformSection, sourceModel: string): string {
  return t(`admin.channels.form.mappingPricingStatus.${mappingPricingStatus(section, sourceModel)}`)
}

function mappingPricingStatusClass(section: PlatformSection, sourceModel: string): string {
  const status = mappingPricingStatus(section, sourceModel)
  if (status === 'covered') {
    return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300'
  }
  if (status === 'missing') {
    return 'bg-amber-50 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300'
  }
  return 'bg-stone-100 text-stone-500 dark:bg-white/[0.06] dark:text-stone-400'
}

// ── Account Stats Pricing helpers ──
function addAccountStatsRule(sectionIdx: number) {
  form.platforms[sectionIdx].account_stats_pricing_rules.push({
    name: '',
    group_ids: [],
    account_ids: [],
    pricing: []
  })
}

function addRulePricingEntry(sectionIdx: number, ruleIndex: number) {
  form.platforms[sectionIdx].account_stats_pricing_rules[ruleIndex].pricing.push({
    models: [],
    billing_mode: 'token',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    self_check_enabled_models: []
  })
}

function removeAccountStatsRule(sectionIdx: number, ruleIndex: number) {
  form.platforms[sectionIdx].account_stats_pricing_rules.splice(ruleIndex, 1)
  // Clear all search state since indices shift after removal
  ruleAccountSearchRunner.clearAll()
  clearAllRuleAccountSearchState()
}

function removeRulePricingEntry(sectionIdx: number, ruleIndex: number, pricingIndex: number) {
  form.platforms[sectionIdx].account_stats_pricing_rules[ruleIndex].pricing.splice(pricingIndex, 1)
}

function getGroupNameById(groupId: number): string {
  const group = allGroups.value.find(g => g.id === groupId)
  return group ? group.name : `#${groupId}`
}

// ── Account search for pricing rules ──
interface SimpleAccount { id: number; name: string; platform: string }

const ruleAccountSearchKeyword = ref<Record<string, string>>({})
const ruleAccountSearchResults = ref<Record<string, SimpleAccount[]>>({})
const showRuleAccountDropdown = ref<Record<string, boolean>>({})
// Cache: account ID → name, populated when search results are selected
const ruleAccountNameCache = ref<Record<number, string>>({})

const ruleAccountSearchRunner = useKeyedDebouncedSearch<SimpleAccount[]>({
  delay: 300,
  search: async (keyword, { key, signal }) => {
    const platform = key.split('-')[0]
    const res = await adminAPI.accounts.list(1, 20, { platform, search: keyword }, { signal })
    return res.items.map(a => ({ id: a.id, name: a.name, platform: a.platform }))
  },
  onSuccess: (key, result) => { ruleAccountSearchResults.value[key] = result },
  onError: (key) => { ruleAccountSearchResults.value[key] = [] },
})

function onRuleAccountSearchInput(platform: string, ruleIndex: number) {
  const key = `${platform}-${ruleIndex}`
  showRuleAccountDropdown.value[key] = true
  ruleAccountSearchRunner.trigger(key, ruleAccountSearchKeyword.value[key] || '')
}

function onRuleAccountSearchFocus(platform: string, ruleIndex: number) {
  const key = `${platform}-${ruleIndex}`
  showRuleAccountDropdown.value[key] = true
  if (!ruleAccountSearchResults.value[key]?.length) {
    ruleAccountSearchRunner.trigger(key, ruleAccountSearchKeyword.value[key] || '')
  }
}

function selectRuleAccount(
  rule: { account_ids: number[] },
  account: SimpleAccount,
  platform: string,
  ruleIndex: number,
) {
  if (!rule.account_ids.includes(account.id)) {
    rule.account_ids.push(account.id)
    ruleAccountNameCache.value[account.id] = account.name
  }
  const key = `${platform}-${ruleIndex}`
  ruleAccountSearchKeyword.value[key] = ''
  showRuleAccountDropdown.value[key] = false
}

function removeRuleAccount(rule: { account_ids: number[] }, accountId: number) {
  const idx = rule.account_ids.indexOf(accountId)
  if (idx !== -1) rule.account_ids.splice(idx, 1)
}

function getRuleAccountLabel(accountId: number): string {
  const name = ruleAccountNameCache.value[accountId]
  return name ? `${name} #${accountId}` : `#${accountId}`
}

function handleRuleAccountClickOutside(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.rule-account-search-container')) {
    Object.keys(showRuleAccountDropdown.value).forEach(key => {
      showRuleAccountDropdown.value[key] = false
    })
  }
}

function clearAllRuleAccountSearchState() {
  ruleAccountSearchKeyword.value = {}
  ruleAccountSearchResults.value = {}
  showRuleAccountDropdown.value = {}
}

function accountStatsRulesToAPI(): AccountStatsPricingRule[] {
  const rules: AccountStatsPricingRule[] = []
  for (const section of form.platforms) {
    if (!section.enabled) continue
    for (const rule of section.account_stats_pricing_rules) {
      rules.push({
        name: rule.name,
        group_ids: rule.group_ids,
        account_ids: rule.account_ids,
        pricing: rule.pricing
          .filter(p => p.models.length > 0)
          .map(p => ({
            platform: section.platform,
            models: p.models,
            billing_mode: p.billing_mode,
            input_price: mTokToPerToken(p.input_price),
            output_price: mTokToPerToken(p.output_price),
            cache_write_price: mTokToPerToken(p.cache_write_price),
            cache_read_price: mTokToPerToken(p.cache_read_price),
            image_input_price: mTokToPerToken(p.image_input_price),
            image_output_price: mTokToPerToken(p.image_output_price),
            per_request_price: p.per_request_price != null && p.per_request_price !== '' ? Number(p.per_request_price) : null,
            intervals: formIntervalsToAPI(p.intervals || []),
            time_pricing: undefined,
            self_check_enabled_models: []
          }))
      })
    }
  }
  return rules
}

// ── Form ↔ API conversion ──
function sanitizeSelfCheckModels(entry: Pick<PricingFormEntry, 'models' | 'self_check_enabled_models'>): string[] {
  const allowed = new Map(entry.models.map(model => [model.trim().toLowerCase(), model.trim()]))
  const out: string[] = []
  const seen = new Set<string>()
  for (const model of entry.self_check_enabled_models || []) {
    const key = model.trim().toLowerCase()
    const canonical = allowed.get(key)
    if (!canonical || seen.has(key)) continue
    seen.add(key)
    out.push(canonical)
  }
  return out
}

function formToAPI(): {
  group_ids: number[]
  model_pricing: ChannelModelPricing[]
  model_mapping: Record<string, Record<string, string>>
  model_mapping_order: Record<string, string[]>
  features_config: Record<string, unknown>
} {
  const group_ids: number[] = []
  const model_pricing: ChannelModelPricing[] = []
  const model_mapping: Record<string, Record<string, string>> = {}
  const model_mapping_order: Record<string, string[]> = {}
  // Preserve existing features_config fields not managed by the form
  const featuresConfig: Record<string, unknown> = editingChannel.value?.features_config
    ? { ...editingChannel.value.features_config }
    : {}

  for (const section of form.platforms) {
    if (!section.enabled) continue
    group_ids.push(...section.group_ids)

    // Model mapping per platform
    if (Object.keys(section.model_mapping).length > 0) {
      model_mapping[section.platform] = { ...section.model_mapping }
      model_mapping_order[section.platform] = normalizeMappingOrder(
        section.model_mapping,
        section.model_mapping_order,
      )
    }

    // Model pricing with platform tag
    for (const [pricingIndex, entry] of section.model_pricing.entries()) {
      if (entry.models.length === 0) continue
      model_pricing.push({
        sort_order: pricingIndex,
        platform: section.platform,
        models: entry.models,
        billing_mode: entry.billing_mode,
        input_price: mTokToPerToken(entry.input_price),
        output_price: mTokToPerToken(entry.output_price),
        cache_write_price: mTokToPerToken(entry.cache_write_price),
        cache_read_price: mTokToPerToken(entry.cache_read_price),
        image_input_price: mTokToPerToken(entry.image_input_price),
        image_output_price: mTokToPerToken(entry.image_output_price),
        per_request_price: entry.per_request_price != null && entry.per_request_price !== '' ? Number(entry.per_request_price) : null,
        intervals: formIntervalsToAPI(entry.intervals || []),
        time_pricing: entry.billing_mode === 'token' ? formTimePricingToAPI(entry.time_pricing) : undefined,
        self_check_enabled_models: sanitizeSelfCheckModels(entry)
      })
    }
  }
  const uniqueGroupIds = Array.from(new Set(group_ids))

  // Collect web_search_emulation (only anthropic platform supports it)
  // Always write the key so that disabling in the UI correctly sets platform to false,
  // rather than leaving a stale true value from the cloned features_config.
  const wsEmulation: Record<string, boolean> = {}
  for (const section of form.platforms) {
    if (!section.enabled) continue
    if (section.platform === 'anthropic') {
      wsEmulation[section.platform] = !!section.web_search_emulation
    }
  }
  if (Object.keys(wsEmulation).length > 0) {
    featuresConfig.web_search_emulation = wsEmulation
  } else {
    delete featuresConfig.web_search_emulation
  }

  const codexImageGenerationBridge: Record<string, boolean> = {}
  for (const section of form.platforms) {
    if (!section.enabled) continue
    if (section.platform === 'openai') {
      codexImageGenerationBridge[section.platform] = !!section.codex_image_generation_bridge
    }
  }
  if (Object.keys(codexImageGenerationBridge).length > 0) {
    featuresConfig.codex_image_generation_bridge = codexImageGenerationBridge
  } else {
    delete featuresConfig.codex_image_generation_bridge
  }

  const bedrockCCCompat: Record<string, boolean> = {}
  for (const section of form.platforms) {
    if (!section.enabled) continue
    if (section.platform === 'anthropic') {
      bedrockCCCompat[section.platform] = !!section.bedrock_cc_compat
    }
  }
  if (Object.keys(bedrockCCCompat).length > 0) {
    featuresConfig.bedrock_cc_compat = bedrockCCCompat
  } else {
    delete featuresConfig.bedrock_cc_compat
  }

  return {
    group_ids: uniqueGroupIds,
    model_pricing,
    model_mapping,
    model_mapping_order,
    features_config: featuresConfig,
  }
}

function apiToForm(channel: Channel): PlatformSection[] {
  // Build a map: groupID → platform
  const groupPlatformMap = new Map<number, GroupPlatform>()
  for (const g of allGroups.value) {
    groupPlatformMap.set(g.id, g.platform)
  }

  // Determine which platforms are active (from groups + pricing + mapping)
  const activePlatforms = new Set<GroupPlatform>()
  for (const gid of channel.group_ids || []) {
    const p = groupPlatformMap.get(gid)
    if (p === 'composite') {
      compositePlatforms.forEach(platform => activePlatforms.add(platform))
    } else if (p) {
      activePlatforms.add(p)
    }
  }
  for (const p of channel.model_pricing || []) {
    if (p.platform) activePlatforms.add(p.platform as GroupPlatform)
  }
  for (const p of Object.keys(channel.model_mapping || {})) {
    if (platformOrder.includes(p as GroupPlatform)) activePlatforms.add(p as GroupPlatform)
  }

  // Build sections in platform order
  const sections: PlatformSection[] = []
  for (const platform of platformOrder) {
    if (!activePlatforms.has(platform)) continue

    const groupIds = (channel.group_ids || []).filter(gid => {
      const groupPlatform = groupPlatformMap.get(gid)
      return groupPlatform === platform ||
        (groupPlatform === 'composite' && compositePlatforms.includes(platform))
    })
    const mapping = (channel.model_mapping || {})[platform] || {}
    const mappingOrder = normalizeMappingOrder(
      mapping,
      channel.model_mapping_order?.[platform],
    )
    const pricing = (channel.model_pricing || [])
      .filter(p => (p.platform || 'anthropic') === platform)
      .map(p => ({
        _ui_id: p.id ? `pricing-${p.id}` : nextPricingUIId(),
        sort_order: p.sort_order ?? 0,
        models: p.models || [],
        billing_mode: p.billing_mode,
        input_price: perTokenToMTok(p.input_price),
        output_price: perTokenToMTok(p.output_price),
        cache_write_price: perTokenToMTok(p.cache_write_price),
        cache_read_price: perTokenToMTok(p.cache_read_price),
        image_input_price: perTokenToMTok(p.image_input_price),
        image_output_price: perTokenToMTok(p.image_output_price),
        per_request_price: p.per_request_price,
        intervals: apiIntervalsToForm(p.intervals || []),
        time_pricing: apiTimePricingToForm(p.time_pricing),
        self_check_enabled_models: sanitizeSelfCheckModels({
          models: p.models || [],
          self_check_enabled_models: p.self_check_enabled_models || []
        })
      } as PricingFormEntry))

    // Read web_search_emulation from features_config
    const fc = channel.features_config
    const wsEmulation = fc?.web_search_emulation as Record<string, boolean> | undefined
    const webSearchEnabled = wsEmulation?.[platform] === true
    const codexImageGenerationBridge = fc?.codex_image_generation_bridge as Record<string, boolean> | undefined
    const codexImageGenerationBridgeEnabled = codexImageGenerationBridge?.[platform] === true
    const bedrockCCCompatEnabled = fc?.bedrock_cc_compat === true

    sections.push({
      platform,
      enabled: true,
      collapsed: false,
      group_ids: groupIds,
      model_mapping: { ...mapping },
      model_mapping_order: mappingOrder,
      model_pricing: pricing,
      show_missing_models: false,
      web_search_emulation: webSearchEnabled,
      codex_image_generation_bridge: codexImageGenerationBridgeEnabled,
      bedrock_cc_compat: bedrockCCCompatEnabled,
      account_stats_pricing_rules: [],
    })
  }

  return sections
}

// ── Load data ──
async function loadChannels() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true

  try {
    const response = await adminAPI.channels.list(pagination.page, pagination.page_size, {
      status: filters.status || undefined,
      search: searchQuery.value || undefined,
      sort_by: sortState.sort_by,
      sort_order: sortState.sort_order
    }, { signal: ctrl.signal })

    if (ctrl.signal.aborted || abortController !== ctrl) return
    channels.value = response.items || []
    pagination.total = response.total
  } catch (error: unknown) {
    const e = error as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(error, t('admin.channels.loadError', 'Failed to load channels')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

async function loadGroups() {
  groupsLoading.value = true
  try {
    allGroups.value = await adminAPI.groups.getAll()
  } catch (error) {
    console.error('Error loading groups:', error)
  } finally {
    groupsLoading.value = false
  }
}

async function loadAllChannelsForConflict() {
  try {
    const response = await adminAPI.channels.list(1, 1000)
    allChannelsForConflict.value = response.items || []
  } catch (error) {
    // Fallback to current page data
    allChannelsForConflict.value = channels.value
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
function handleSearch() {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadChannels()
  }, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  loadChannels()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadChannels()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadChannels()
}

// ── Dialog ──
function resetForm() {
  form.name = ''
  form.description = ''
  form.status = 'active'
  form.restrict_models = false
  form.billing_model_source = 'channel_mapped'
  form.platforms = []
  form.apply_pricing_to_account_stats = false
  activeTab.value = 'basic'
  ruleAccountSearchRunner.clearAll()
  clearAllRuleAccountSearchState()
  ruleAccountNameCache.value = {}
  deliveryLoading.value = false
  deliveryLoaded.value = false
  deliveryError.value = ''
  deliveryWarnings.value = []
  channelModelDelivery.value = []
  closeDeliveryDialog()
}

async function openCreateDialog() {
  editingChannel.value = null
  resetForm()
  await Promise.all([loadGroups(), loadAllChannelsForConflict()])
  showDialog.value = true
}

async function openEditDialog(channel: Channel) {
  editingChannel.value = channel
  form.name = channel.name
  form.description = channel.description || ''
  form.status = channel.status
  form.restrict_models = channel.restrict_models || false
  form.billing_model_source = channel.billing_model_source || 'channel_mapped'
  form.apply_pricing_to_account_stats = channel.apply_pricing_to_account_stats || false
  // Must load groups first so apiToForm can map groupID → platform
  await Promise.all([loadGroups(), loadAllChannelsForConflict(), loadChannelModelDelivery(channel.id)])
  form.platforms = apiToForm(channel)

  // Distribute channel-level rules into per-platform sections
  distributeRulesToPlatforms(channel.account_stats_pricing_rules || [])

  // Populate ruleAccountNameCache for existing rule accounts
  await populateRuleAccountNameCache()

  showDialog.value = true
}

async function loadChannelModelDelivery(channelId: number) {
  deliveryLoading.value = true
  deliveryLoaded.value = false
  deliveryError.value = ''
  deliveryWarnings.value = []
  try {
    const result = await adminAPI.channels.getModelDelivery(channelId)
    if (editingChannel.value?.id !== channelId) return
    channelModelDelivery.value = result.models || []
    deliveryWarnings.value = result.warnings || []
    deliveryLoaded.value = true
  } catch (error) {
    if (editingChannel.value?.id !== channelId) return
    channelModelDelivery.value = []
    deliveryError.value = extractApiErrorMessage(error, t('admin.channels.form.deliveryLoadFailed'))
  } finally {
    if (editingChannel.value?.id === channelId) deliveryLoading.value = false
  }
}

/** Distribute flat channel-level rules into the matching platform section based on group_ids */
function distributeRulesToPlatforms(apiRules: AccountStatsPricingRule[]) {
  // Build groupID → platform lookup
  const groupPlatformMap = new Map<number, GroupPlatform>()
  for (const g of allGroups.value) {
    groupPlatformMap.set(g.id, g.platform)
  }

  for (const apiRule of apiRules) {
    // Infer platform from group_ids
    const platforms = new Set<GroupPlatform>()
    for (const gid of apiRule.group_ids || []) {
      const p = groupPlatformMap.get(gid)
      if (p && p !== 'composite') platforms.add(p)
    }
    // If pricing has a platform field, use that as fallback
    if (platforms.size === 0 && apiRule.pricing?.length > 0) {
      const p = apiRule.pricing[0].platform as GroupPlatform | undefined
      if (p) platforms.add(p)
    }
    const targetPlatform = platforms.size >= 1 ? [...platforms][0] : null
    if (!targetPlatform) continue

    const section = form.platforms.find(s => s.platform === targetPlatform)
    if (!section) continue

    const formRule: FormPricingRule = {
      name: apiRule.name || '',
      group_ids: [...(apiRule.group_ids || [])],
      account_ids: [...(apiRule.account_ids || [])],
      pricing: (apiRule.pricing || []).map(p => ({
        models: [...(p.models || [])],
        billing_mode: p.billing_mode,
        input_price: perTokenToMTok(p.input_price),
        output_price: perTokenToMTok(p.output_price),
        cache_write_price: perTokenToMTok(p.cache_write_price),
        cache_read_price: perTokenToMTok(p.cache_read_price),
        image_input_price: perTokenToMTok(p.image_input_price),
        image_output_price: perTokenToMTok(p.image_output_price),
        per_request_price: p.per_request_price,
        intervals: apiIntervalsToForm(p.intervals || []),
        time_pricing: undefined,
        self_check_enabled_models: []
      } as PricingFormEntry))
    }
    section.account_stats_pricing_rules.push(formRule)
  }
}

/** Populate ruleAccountNameCache by fetching account details for all account_ids in rules */
async function populateRuleAccountNameCache() {
  const allAccountIds = new Set<number>()
  for (const section of form.platforms) {
    for (const rule of section.account_stats_pricing_rules) {
      for (const id of rule.account_ids) {
        allAccountIds.add(id)
      }
    }
  }
  if (allAccountIds.size === 0) return

  // Fetch account details in parallel (batch of individual getById calls)
  const ids = [...allAccountIds]
  const results = await Promise.allSettled(
    ids.map(id => adminAPI.accounts.getById(id))
  )
  for (let i = 0; i < ids.length; i++) {
    const result = results[i]
    if (result.status === 'fulfilled') {
      ruleAccountNameCache.value[ids[i]] = result.value.name
    }
    // If rejected, the cache won't have the name, so it'll show "#ID" which is acceptable
  }
}

function closeDialog() {
  showDialog.value = false
  editingChannel.value = null
  resetForm()
}

async function handleSubmit() {
  if (submitting.value) return
  if (!form.name.trim()) {
    appStore.showError(t('admin.channels.nameRequired', 'Please enter a channel name'))
    return
  }

  // Check for pricing entries with empty models (would be silently skipped)
  for (const section of form.platforms.filter(s => s.enabled)) {
    if (section.group_ids.length === 0) {
      const platformLabel = t('admin.groups.platforms.' + section.platform, section.platform)
      appStore.showError(t('admin.channels.noGroupsSelected', { platform: platformLabel }))
      activeTab.value = section.platform
      return
    }
    for (const entry of section.model_pricing) {
      if (entry.models.length === 0) {
        const platformLabel = t('admin.groups.platforms.' + section.platform, section.platform)
        appStore.showError(t('admin.channels.emptyModelsInPricing', { platform: platformLabel }))
        activeTab.value = section.platform
        return
      }
    }
  }

  const pricingCoverageWarnings: string[] = []
  for (const section of form.platforms.filter(s => s.enabled)) {
    const coverage = pricingCoverage(section)
    const severity = pricingCoverageSeverity(coverage, form.restrict_models)
    if (severity === 'unknown' || severity === 'none') continue
    const platformLabel = t('admin.groups.platforms.' + section.platform, section.platform)
    const message = t('admin.channels.form.missingPricingCoverage', {
      platform: platformLabel,
      models: coverage.missingModels.join(', '),
    })
    if (severity === 'error') {
      appStore.showError(message)
      section.show_missing_models = true
      activeTab.value = section.platform
      return
    }
    pricingCoverageWarnings.push(message)
  }
  if (pricingCoverageWarnings.length > 0) {
    appStore.showWarning(pricingCoverageWarnings.join('；'), 7000)
  }

  // Check model pattern conflicts per platform (duplicate / wildcard overlap)
  for (const section of form.platforms.filter(s => s.enabled)) {
    // Collect all pricing models for this platform
    const allModels: string[] = []
    for (const entry of section.model_pricing) {
      allModels.push(...entry.models)
    }
    const pricingConflict = findModelConflict(allModels)
    if (pricingConflict) {
      appStore.showError(
        t('admin.channels.modelConflict',
          { model1: pricingConflict[0], model2: pricingConflict[1] })
      )
      activeTab.value = section.platform
      return
    }
    // Check model mapping source pattern conflicts
    const mappingKeys = Object.keys(section.model_mapping)
    if (mappingKeys.length > 0) {
      const mappingConflict = findModelConflict(mappingKeys)
      if (mappingConflict) {
        appStore.showError(
          t('admin.channels.mappingConflict',
            { model1: mappingConflict[0], model2: mappingConflict[1] })
        )
        activeTab.value = section.platform
        return
      }
    }
  }

  // 校验 per_request/image 模式必须有价格 (只校验启用的平台)
  for (const section of form.platforms.filter(s => s.enabled)) {
    for (const entry of section.model_pricing) {
      if (entry.models.length === 0) continue
      if ((entry.billing_mode === 'per_request' || entry.billing_mode === 'image') &&
          (entry.per_request_price == null || entry.per_request_price === '') &&
          (!entry.intervals || entry.intervals.length === 0)) {
        appStore.showError(t('admin.channels.form.perRequestPriceRequired'))
        return
      }
    }
  }

  // 校验区间合法性（范围、重叠等）
  for (const section of form.platforms.filter(s => s.enabled)) {
    for (const entry of section.model_pricing) {
      const intervalErr = validateIntervals(entry.intervals || [], entry.billing_mode, t)
      if (intervalErr) {
        const platformLabel = t('admin.groups.platforms.' + section.platform, section.platform)
        const modelLabel = entry.models.join(', ') || t('admin.channels.form.unnamed')
        appStore.showError(`${platformLabel} - ${modelLabel}: ${intervalErr}`)
        activeTab.value = section.platform
        return
      }
      const timePricingErr = validateTimePricing(entry, t)
      if (timePricingErr) {
        const platformLabel = t('admin.groups.platforms.' + section.platform, section.platform)
        const modelLabel = entry.models.join(', ') || t('admin.channels.form.unnamed')
        appStore.showError(`${platformLabel} - ${modelLabel}: ${timePricingErr}`)
        activeTab.value = section.platform
        return
      }
    }
  }

  const { group_ids, model_pricing, model_mapping, model_mapping_order, features_config } = formToAPI()

  submitting.value = true
  try {
    if (editingChannel.value) {
      const req: UpdateChannelRequest = {
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        status: form.status,
        group_ids,
        model_pricing,
        model_mapping: Object.keys(model_mapping).length > 0 ? model_mapping : {},
        model_mapping_order,
        billing_model_source: form.billing_model_source,
        restrict_models: form.restrict_models,
        features_config,
        apply_pricing_to_account_stats: form.apply_pricing_to_account_stats,
        account_stats_pricing_rules: accountStatsRulesToAPI()
      }
      await adminAPI.channels.update(editingChannel.value.id, req)
      appStore.showSuccess(t('admin.channels.updateSuccess', 'Channel updated'))
    } else {
      const req: CreateChannelRequest = {
        name: form.name.trim(),
        description: form.description.trim() || undefined,
        group_ids,
        model_pricing,
        model_mapping: Object.keys(model_mapping).length > 0 ? model_mapping : {},
        model_mapping_order,
        billing_model_source: form.billing_model_source,
        restrict_models: form.restrict_models,
        features_config,
        apply_pricing_to_account_stats: form.apply_pricing_to_account_stats,
        account_stats_pricing_rules: accountStatsRulesToAPI()
      }
      await adminAPI.channels.create(req)
      appStore.showSuccess(t('admin.channels.createSuccess', 'Channel created'))
    }
    closeDialog()
    loadChannels()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, editingChannel.value
      ? t('admin.channels.updateError', 'Failed to update channel')
      : t('admin.channels.createError', 'Failed to create channel')))
  } finally {
    submitting.value = false
  }
}

// ── Toggle status ──
async function toggleChannelStatus(channel: Channel) {
  const newStatus = channel.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.channels.update(channel.id, { status: newStatus })
    if (filters.status && filters.status !== newStatus) {
      // Item no longer matches the active filter — reload list
      await loadChannels()
    } else {
      channel.status = newStatus
    }
  } catch (error) {
    appStore.showError(t('admin.channels.updateError', 'Failed to update channel'))
    console.error('Error toggling channel status:', error)
  }
}

// ── Delete ──
function handleDelete(channel: Channel) {
  deletingChannel.value = channel
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deletingChannel.value) return

  try {
    await adminAPI.channels.remove(deletingChannel.value.id)
    appStore.showSuccess(t('admin.channels.deleteSuccess', 'Channel deleted'))
    showDeleteDialog.value = false
    deletingChannel.value = null
    loadChannels()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channels.deleteError', 'Failed to delete channel')))
  }
}

// ── Lifecycle ──
onMounted(() => {
  loadChannels()
  loadGroups()
  loadWebSearchGlobalState()
  document.addEventListener('click', handleRuleAccountClickOutside)
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
  document.removeEventListener('click', handleRuleAccountClickOutside)
  ruleAccountSearchRunner.clearAll()
  clearAllRuleAccountSearchState()
})
</script>

<style scoped>
.channel-dialog-body {
  display: flex;
  flex-direction: column;
  height: 70vh;
  min-height: 400px;
}

.channel-tab {
  @apply flex items-center gap-1.5 px-3 py-2.5 text-sm font-medium border-b-2 transition-colors whitespace-nowrap;
}

.channel-tab-active {
  @apply border-emerald-500 text-emerald-700 dark:border-emerald-400 dark:text-emerald-300;
}

.channel-tab-inactive {
  @apply border-transparent text-stone-500 hover:text-stone-700 hover:border-stone-300 dark:text-stone-400 dark:hover:text-stone-300;
}
</style>
