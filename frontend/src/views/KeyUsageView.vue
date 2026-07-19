<template>
  <div class="min-h-screen bg-stone-50 text-stone-950 dark:bg-[#050505] dark:text-white">
    <header class="sticky top-0 z-40 border-b border-stone-200/80 bg-white/90 backdrop-blur dark:border-[#1e1e1e] dark:bg-[#050505]/90">
      <nav class="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
        <router-link to="/home" class="truncate text-base font-bold tracking-tight text-emerald-600 dark:text-emerald-400">
          {{ siteName }}
        </router-link>
        <div class="flex items-center gap-2">
          <LocaleSwitcher />
          <button
            type="button"
            class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-stone-200 text-stone-500 transition hover:border-emerald-500/40 hover:text-emerald-600 dark:border-[#262626] dark:text-stone-400"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <button
            v-if="hasSession"
            type="button"
            class="inline-flex h-9 items-center rounded-lg border border-stone-200 px-3 text-sm font-medium text-stone-700 transition hover:border-rose-400 hover:text-rose-600 dark:border-[#262626] dark:text-stone-300"
            :disabled="sessionDeleting"
            @click="exitQuery"
          >
            {{ t('keyUsage.exit') }}
          </button>
        </div>
      </nav>
    </header>

    <main class="mx-auto w-full max-w-6xl px-4 py-8 md:py-12">
      <div v-if="sessionChecking || sessionDeleting" class="flex min-h-[55vh] items-center justify-center">
        <div class="flex items-center gap-3 text-sm text-stone-500 dark:text-stone-400">
          <span class="h-4 w-4 animate-spin rounded-full border-2 border-stone-300 border-t-emerald-500"></span>
          {{ t(sessionDeleting ? 'keyUsage.exiting' : 'keyUsage.restoring') }}
        </div>
      </div>

      <section v-else-if="!hasSession" class="mx-auto max-w-xl pt-10 md:pt-20">
        <div class="mb-8">
          <h1 class="text-3xl font-bold tracking-tight md:text-4xl">{{ t('keyUsage.title') }}</h1>
          <p class="mt-3 leading-7 text-stone-500 dark:text-stone-400">{{ t('keyUsage.subtitle') }}</p>
        </div>
        <form class="rounded-2xl border border-stone-200 bg-white p-5 shadow-sm dark:border-[#242424] dark:bg-[#0d0d0d] md:p-7" @submit.prevent="createSession">
          <label for="key-usage-input" class="mb-2 block text-sm font-semibold">{{ t('keyUsage.keyLabel') }}</label>
          <div class="relative">
            <input
              id="key-usage-input"
              v-model="apiKey"
              :type="keyVisible ? 'text' : 'password'"
              :placeholder="t('keyUsage.placeholder')"
              autocomplete="off"
              autocapitalize="off"
              spellcheck="false"
              class="h-12 w-full rounded-xl border border-stone-300 bg-white px-4 pr-20 font-mono text-sm outline-none transition placeholder:text-stone-400 focus:border-emerald-500 focus:ring-4 focus:ring-emerald-500/10 dark:border-[#303030] dark:bg-black dark:text-white"
            />
            <button type="button" class="absolute right-2 top-2 h-8 rounded-lg px-3 text-xs font-medium text-stone-500 hover:bg-stone-100 dark:hover:bg-white/10" @click="keyVisible = !keyVisible">
              {{ keyVisible ? t('keyUsage.hide') : t('keyUsage.show') }}
            </button>
          </div>
          <button
            type="submit"
            class="mt-4 inline-flex h-11 w-full items-center justify-center rounded-xl bg-emerald-500 px-5 text-sm font-bold text-black transition hover:bg-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
            :disabled="sessionCreating || sessionDeleting || !apiKey.trim()"
          >
            <span v-if="sessionCreating" class="mr-2 h-4 w-4 animate-spin rounded-full border-2 border-black/20 border-t-black"></span>
            {{ sessionCreating ? t('keyUsage.querying') : t('keyUsage.query') }}
          </button>
          <p class="mt-4 text-xs leading-5 text-stone-400 dark:text-stone-500">{{ t('keyUsage.sessionHint') }}</p>
        </form>
      </section>

      <div v-else class="space-y-5">
        <div class="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div>
            <h1 class="text-2xl font-bold tracking-tight md:text-3xl">{{ t('keyUsage.dashboardTitle') }}</h1>
            <p v-if="summary" class="mt-2 text-sm text-stone-500 dark:text-stone-400">
              {{ summary.start_date }} — {{ summary.end_date }} · {{ summary.timezone }}
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button
              v-for="range in dateRanges"
              :key="range.value"
              type="button"
              class="h-9 rounded-lg border px-3 text-sm font-medium transition"
              :class="selectedRange === range.value
                ? 'border-emerald-500 bg-emerald-500 text-black'
                : 'border-stone-200 bg-white text-stone-600 hover:border-emerald-500/50 dark:border-[#262626] dark:bg-[#0d0d0d] dark:text-stone-300'"
              @click="applyPresetRange(range.value)"
            >
              {{ range.label }}
            </button>
            <button type="button" class="inline-flex h-9 w-9 items-center justify-center rounded-lg border border-stone-200 bg-white text-stone-500 hover:border-emerald-500/50 dark:border-[#262626] dark:bg-[#0d0d0d]" :disabled="summaryLoading" @click="refreshAll">
              <Icon name="refresh" size="sm" :class="summaryLoading && 'animate-spin'" />
            </button>
          </div>
        </div>

        <div v-if="selectedRange === 'custom'" class="flex flex-wrap items-end gap-3 rounded-xl border border-stone-200 bg-white p-4 dark:border-[#242424] dark:bg-[#0d0d0d]">
          <label class="text-xs font-medium text-stone-500">
            {{ t('keyUsage.startDate') }}
            <input v-model="startDate" type="date" class="mt-1 block h-10 rounded-lg border border-stone-300 bg-white px-3 text-sm text-stone-900 outline-none focus:border-emerald-500 dark:border-[#303030] dark:bg-black dark:text-white" />
          </label>
          <label class="text-xs font-medium text-stone-500">
            {{ t('keyUsage.endDate') }}
            <input v-model="endDate" type="date" class="mt-1 block h-10 rounded-lg border border-stone-300 bg-white px-3 text-sm text-stone-900 outline-none focus:border-emerald-500 dark:border-[#303030] dark:bg-black dark:text-white" />
          </label>
          <button type="button" class="h-10 rounded-lg bg-emerald-500 px-4 text-sm font-bold text-black hover:bg-emerald-400" @click="refreshAll">{{ t('keyUsage.apply') }}</button>
        </div>

        <div v-if="summaryLoading && !summary" class="grid gap-4 md:grid-cols-3">
          <div v-for="index in 6" :key="index" class="h-32 animate-pulse rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]"></div>
        </div>

        <template v-else-if="summary">
          <section class="overflow-hidden rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]">
            <div class="flex flex-col gap-4 p-5 md:flex-row md:items-start md:justify-between md:p-6">
              <div>
                <div class="flex flex-wrap items-center gap-3">
                  <h2 class="text-xl font-bold">{{ summary.identity.member?.name || summary.identity.name }}</h2>
                  <span class="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-semibold" :class="summary.identity.active ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'">
                    <span class="h-1.5 w-1.5 rounded-full" :class="summary.identity.active ? 'bg-emerald-500' : 'bg-rose-500'"></span>
                    {{ statusLabel(summary.identity.status) }}
                  </span>
                </div>
                <div class="mt-2 flex flex-wrap gap-x-5 gap-y-1 font-mono text-xs text-stone-500 dark:text-stone-400">
                  <span>{{ summary.identity.key_prefix }}</span>
                  <span v-if="summary.identity.member">{{ summary.identity.member.code }}</span>
                </div>
              </div>
              <div class="grid grid-cols-2 gap-x-8 gap-y-3 text-sm sm:grid-cols-4">
                <InfoValue :label="t('keyUsage.createdAt')" :value="formatDate(summary.identity.created_at)" />
                <InfoValue :label="t('keyUsage.lastUsedAt')" :value="formatDate(summary.identity.last_used_at)" />
                <InfoValue :label="t('keyUsage.expiresAt')" :value="formatDate(summary.identity.expires_at)" />
                <InfoValue :label="t('keyUsage.ipAccess')" :value="ipAccessLabel" />
              </div>
            </div>
          </section>

          <section class="rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]">
            <div class="border-b border-stone-200 px-5 py-4 dark:border-[#242424]"><h2 class="font-bold">{{ t('keyUsage.accessTitle') }}</h2></div>
            <div v-if="summary.access_groups.length" class="divide-y divide-stone-200 dark:divide-[#242424]">
              <div v-for="group in summary.access_groups" :key="`${group.platform}-${group.name}`" class="p-5">
                <div class="flex flex-wrap items-center justify-between gap-3">
                  <div class="flex flex-wrap items-center gap-2">
                    <h3 class="font-semibold">{{ group.name }}</h3>
                    <span class="rounded-md bg-stone-100 px-2 py-1 text-xs text-stone-500 dark:bg-white/5 dark:text-stone-400">{{ group.platform }}</span>
                    <span class="text-xs" :class="group.status === 'active' ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-500'">{{ statusLabel(group.status) }}</span>
                  </div>
                  <span class="text-xs text-stone-400">RPM {{ group.rpm_limit > 0 ? group.rpm_limit : '∞' }}</span>
                </div>
                <div class="mt-4 flex flex-wrap gap-2">
                  <span v-for="model in group.models" :key="model" class="rounded-lg border border-stone-200 bg-stone-50 px-2.5 py-1.5 font-mono text-xs text-stone-700 dark:border-[#292929] dark:bg-black dark:text-stone-300">{{ model }}</span>
                  <span v-if="!group.models.length" class="text-sm text-stone-400">{{ t('keyUsage.noModels') }}</span>
                </div>
              </div>
            </div>
            <div v-else class="p-5 text-sm text-stone-400">{{ t('keyUsage.noGroups') }}</div>
          </section>

          <section class="rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]">
            <div class="border-b border-stone-200 px-5 py-4 dark:border-[#242424]"><h2 class="font-bold">{{ t('keyUsage.keyBudgetTitle') }}</h2></div>
            <div class="grid gap-px bg-stone-200 dark:bg-[#242424] sm:grid-cols-2 lg:grid-cols-4">
              <BudgetCell :label="t('keyUsage.totalQuota')" :limit="summary.key_budget.quota" />
              <BudgetCell :label="t('keyUsage.limit5h')" :limit="summary.key_budget.limit_5h" />
              <BudgetCell :label="t('keyUsage.limitDaily')" :limit="summary.key_budget.limit_1d" />
              <BudgetCell :label="t('keyUsage.limit7d')" :limit="summary.key_budget.limit_7d" />
            </div>
          </section>

          <section v-if="summary.member_budget" class="rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]">
            <div class="flex flex-wrap items-center justify-between gap-3 border-b border-stone-200 px-5 py-4 dark:border-[#242424]">
              <h2 class="font-bold">{{ t('keyUsage.memberBudgetTitle') }}</h2>
              <span class="text-xs text-stone-400">{{ formatDate(summary.member_budget.period_start) }} — {{ formatDate(summary.member_budget.period_end) }}</span>
            </div>
            <div class="grid gap-px bg-stone-200 dark:bg-[#242424] md:grid-cols-4">
              <div class="bg-emerald-50 p-5 dark:bg-emerald-500/[0.07]">
                <p class="text-xs font-semibold text-emerald-700 dark:text-emerald-400">{{ t('keyUsage.monthlyBudget') }}</p>
                <p class="mt-3 text-2xl font-bold tabular-nums">{{ formatMoney(summary.member_budget.monthly.limit) }}</p>
                <p class="mt-2 text-xs text-stone-500">{{ t('keyUsage.usedQuota') }} {{ formatMoney(summary.member_budget.monthly.used) }} · {{ t('keyUsage.remainingQuota') }} {{ formatMoney(summary.member_budget.monthly.remaining) }}</p>
                <div class="mt-4 h-1.5 overflow-hidden rounded-full bg-black/10 dark:bg-white/10"><span class="block h-full rounded-full bg-emerald-500" :style="{ width: `${limitPercent(summary.member_budget.monthly)}%` }"></span></div>
              </div>
              <BudgetCell :label="t('keyUsage.limit5h')" :limit="summary.member_budget.limit_5h" />
              <BudgetCell :label="t('keyUsage.limitDaily')" :limit="summary.member_budget.limit_1d" />
              <BudgetCell :label="t('keyUsage.limit7d')" :limit="summary.member_budget.limit_7d" />
            </div>
          </section>

          <section class="rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]">
            <div class="border-b border-stone-200 px-5 py-4 dark:border-[#242424]"><h2 class="font-bold">{{ t('keyUsage.statsTitle') }}</h2></div>
            <div class="grid gap-px bg-stone-200 dark:bg-[#242424] sm:grid-cols-2 lg:grid-cols-5">
              <StatCell :label="t('keyUsage.totalRequests')" :value="formatNumber(summary.stats.total_requests)" />
              <StatCell :label="t('keyUsage.totalCost')" :value="formatMoney(summary.stats.total_actual_cost)" />
              <StatCell :label="t('keyUsage.totalTokensLabel')" :value="formatNumber(summary.stats.total_tokens)" />
              <StatCell :label="t('keyUsage.inputTokens')" :value="formatNumber(summary.stats.total_input_tokens)" />
              <StatCell :label="t('keyUsage.outputTokens')" :value="formatNumber(summary.stats.total_output_tokens)" />
            </div>
            <div class="grid gap-5 border-t border-stone-200 p-5 dark:border-[#242424] lg:grid-cols-[1.3fr_0.7fr]">
              <div>
                <h3 class="text-sm font-semibold">{{ t('keyUsage.trendTitle') }}</h3>
                <div v-if="summary.trend.length" class="mt-5 flex h-48 items-end gap-1 overflow-hidden border-b border-stone-200 px-1 dark:border-[#292929]">
                  <div v-for="point in summary.trend" :key="point.date" class="group relative flex h-full min-w-1 flex-1 items-end">
                    <span class="w-full rounded-t bg-emerald-500/80 transition group-hover:bg-emerald-400" :style="{ height: `${trendHeight(point.actual_cost)}%` }"></span>
                    <div class="pointer-events-none absolute bottom-full left-1/2 z-10 mb-2 hidden -translate-x-1/2 whitespace-nowrap rounded-lg bg-stone-900 px-2 py-1 text-[10px] text-white shadow group-hover:block">{{ point.date }} · {{ formatMoney(point.actual_cost) }}</div>
                  </div>
                </div>
                <div v-else class="mt-5 flex h-48 items-center justify-center text-sm text-stone-400">{{ t('keyUsage.noData') }}</div>
              </div>
              <div>
                <h3 class="text-sm font-semibold">{{ t('keyUsage.modelStats') }}</h3>
                <div v-if="summary.models.length" class="mt-4 space-y-4">
                  <div v-for="model in summary.models" :key="model.model">
                    <div class="flex items-start justify-between gap-3 text-xs"><span class="truncate font-mono">{{ model.model }}</span><strong>{{ formatMoney(model.actual_cost) }}</strong></div>
                    <div class="mt-2 h-1.5 overflow-hidden rounded-full bg-stone-100 dark:bg-white/10"><span class="block h-full rounded-full bg-emerald-500" :style="{ width: `${modelWidth(model.actual_cost)}%` }"></span></div>
                    <p class="mt-1 text-[10px] text-stone-400">{{ formatNumber(model.requests) }} {{ t('keyUsage.requests') }} · {{ formatNumber(model.total_tokens) }} Token</p>
                  </div>
                </div>
                <div v-else class="mt-4 text-sm text-stone-400">{{ t('keyUsage.noData') }}</div>
              </div>
            </div>
          </section>

          <section class="overflow-hidden rounded-2xl border border-stone-200 bg-white dark:border-[#242424] dark:bg-[#0d0d0d]">
            <div class="border-b border-stone-200 p-5 dark:border-[#242424]">
              <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
                <div><h2 class="font-bold">{{ t('keyUsage.recordsTitle') }}</h2></div>
                <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                  <label class="text-xs font-medium text-stone-500">
                    {{ t('keyUsage.recordType') }}
                    <Select v-model="recordKind" class="mt-1 min-w-36" :options="recordKindOptions" @change="onRecordFilterChange" />
                  </label>
                  <label class="text-xs font-medium text-stone-500">
                    {{ t('keyUsage.model') }}
                    <Select v-model="modelFilter" class="mt-1 min-w-44" :options="modelOptions" clearable @change="onRecordFilterChange" />
                  </label>
                  <label v-if="recordKind === 'error'" class="text-xs font-medium text-stone-500">
                    {{ t('keyUsage.statusCode') }}
                    <Select v-model="statusFilter" class="mt-1 min-w-32" :options="statusOptions" clearable @change="onRecordFilterChange" />
                  </label>
                  <button type="button" class="mt-5 h-10 rounded-lg border border-stone-200 px-4 text-sm font-semibold text-stone-700 hover:border-emerald-500/50 dark:border-[#303030] dark:text-stone-300" :disabled="exporting" @click="exportRecords">
                    {{ exporting ? t('keyUsage.exporting') : t('keyUsage.export') }}
                  </button>
                </div>
              </div>
            </div>
            <div class="overflow-x-auto">
              <table class="w-full min-w-[920px] border-collapse text-left text-sm">
                <thead class="bg-stone-50 text-xs text-stone-500 dark:bg-black dark:text-stone-400">
                  <tr>
                    <th class="px-4 py-3 font-medium">{{ t('keyUsage.time') }}</th>
                    <th class="px-4 py-3 font-medium">{{ t('keyUsage.model') }}</th>
                    <th class="px-4 py-3 font-medium">{{ t('keyUsage.endpoint') }}</th>
                    <th class="px-4 py-3 font-medium">{{ t('keyUsage.totalTokens') }}</th>
                    <th class="px-4 py-3 font-medium">{{ t('keyUsage.cost') }}</th>
                    <th class="px-4 py-3 font-medium">{{ t('keyUsage.statusCode') }}</th>
                    <th class="px-4 py-3 font-medium"></th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-stone-200 dark:divide-[#242424]">
                  <tr v-for="record in records" :key="`${record.kind}-${record.id}`" class="hover:bg-stone-50 dark:hover:bg-white/[0.025]">
                    <td class="px-4 py-3 text-xs text-stone-500">{{ formatDateTime(record.created_at) }}</td>
                    <td class="max-w-56 truncate px-4 py-3 font-mono text-xs">{{ record.model || '—' }}</td>
                    <td class="max-w-52 truncate px-4 py-3 font-mono text-xs text-stone-500">{{ record.inbound_endpoint || '—' }}</td>
                    <td class="px-4 py-3 tabular-nums">{{ formatNumber(record.total_tokens || 0) }}</td>
                    <td class="px-4 py-3 tabular-nums">{{ record.kind === 'success' ? formatMoney(record.actual_cost || 0) : '—' }}</td>
                    <td class="px-4 py-3"><span class="rounded-md px-2 py-1 text-xs font-bold" :class="record.status_code < 400 ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400' : 'bg-rose-500/10 text-rose-600 dark:text-rose-400'">{{ record.status_code }}</span></td>
                    <td class="px-4 py-3 text-right"><button type="button" class="text-xs font-semibold text-emerald-600 hover:underline dark:text-emerald-400" @click="openRecord(record)">{{ t('keyUsage.detail') }}</button></td>
                  </tr>
                  <tr v-if="!recordsLoading && !records.length"><td colspan="7" class="px-4 py-12 text-center text-sm text-stone-400">{{ t('keyUsage.noRecords') }}</td></tr>
                  <tr v-if="recordsLoading"><td colspan="7" class="px-4 py-12 text-center text-sm text-stone-400">{{ t('keyUsage.loadingRecords') }}</td></tr>
                </tbody>
              </table>
            </div>
            <Pagination v-if="recordTotal > recordPageSize" :total="recordTotal" :page="recordPage" :page-size="recordPageSize" :show-page-size-selector="false" @update:page="changeRecordPage" />
          </section>
        </template>
      </div>
    </main>

    <div v-if="hasSession && selectedRecord" class="fixed inset-0 z-50 flex items-center justify-center bg-black/55 p-4 backdrop-blur-sm" @click.self="selectedRecord = null">
      <div class="max-h-[85vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-stone-200 bg-white shadow-2xl dark:border-[#303030] dark:bg-[#0d0d0d]">
        <div class="sticky top-0 flex items-center justify-between border-b border-stone-200 bg-white px-5 py-4 dark:border-[#242424] dark:bg-[#0d0d0d]">
          <h2 class="font-bold">{{ t('keyUsage.recordDetail') }}</h2>
          <button type="button" class="text-stone-400 hover:text-stone-900 dark:hover:text-white" @click="selectedRecord = null"><Icon name="x" size="sm" /></button>
        </div>
        <div class="grid gap-px bg-stone-200 dark:bg-[#242424] sm:grid-cols-2">
          <DetailCell :label="t('keyUsage.time')" :value="formatDateTime(selectedRecord.created_at)" />
          <DetailCell :label="t('keyUsage.statusCode')" :value="String(selectedRecord.status_code)" />
          <DetailCell :label="t('keyUsage.model')" :value="selectedRecord.model || '—'" mono />
          <DetailCell :label="t('keyUsage.endpoint')" :value="selectedRecord.inbound_endpoint || '—'" mono />
          <DetailCell :label="t('keyUsage.requestId')" :value="selectedRecord.request_id || '—'" mono />
          <DetailCell :label="t('keyUsage.ipAddress')" :value="selectedRecord.ip_address || '—'" mono />
          <DetailCell :label="t('keyUsage.group')" :value="selectedRecord.group_name || '—'" />
          <DetailCell :label="t('keyUsage.platform')" :value="selectedRecord.platform || '—'" />
          <DetailCell :label="t('keyUsage.requestType')" :value="selectedRecord.request_type || '—'" />
          <DetailCell :label="t('keyUsage.stream')" :value="selectedRecord.stream ? t('keyUsage.yes') : t('keyUsage.no')" />
          <DetailCell :label="t('keyUsage.inputTokens')" :value="formatNumber(selectedRecord.input_tokens || 0)" />
          <DetailCell :label="t('keyUsage.outputTokens')" :value="formatNumber(selectedRecord.output_tokens || 0)" />
          <DetailCell :label="t('keyUsage.cacheTokens')" :value="formatNumber((selectedRecord.cache_creation_tokens || 0) + (selectedRecord.cache_read_tokens || 0))" />
          <DetailCell :label="t('keyUsage.totalTokens')" :value="formatNumber(selectedRecord.total_tokens || 0)" />
          <DetailCell :label="t('keyUsage.duration')" :value="formatMilliseconds(selectedRecord.duration_ms)" />
          <DetailCell :label="t('keyUsage.firstToken')" :value="formatMilliseconds(selectedRecord.first_token_ms)" />
          <DetailCell :label="t('keyUsage.cost')" :value="selectedRecord.kind === 'success' ? formatMoney(selectedRecord.actual_cost || 0) : '—'" />
          <DetailCell :label="t('keyUsage.category')" :value="selectedRecord.category || '—'" />
          <DetailCell :label="t('keyUsage.upstreamStatus')" :value="selectedRecord.upstream_status_code ? String(selectedRecord.upstream_status_code) : '—'" />
          <DetailCell :label="t('keyUsage.userAgent')" :value="selectedRecord.user_agent || '—'" mono />
        </div>
        <div v-if="selectedRecord.message" class="border-t border-stone-200 p-5 dark:border-[#242424]">
          <p class="text-xs font-medium text-stone-400">{{ t('keyUsage.message') }}</p>
          <p class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-stone-700 dark:text-stone-300">{{ selectedRecord.message }}</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  publicKeyUsageAPI,
  type PublicKeyUsageLimit,
  type PublicKeyUsageRecord,
  type PublicKeyUsageRecordKind,
  type PublicKeyUsageSummary,
} from '@/api/publicKeyUsage'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const isDark = ref(false)
const sessionChecking = ref(true)
const sessionCreating = ref(false)
const sessionDeleting = ref(false)
const hasSession = ref(false)
const apiKey = ref('')
const keyVisible = ref(false)
const summary = ref<PublicKeyUsageSummary | null>(null)
const summaryLoading = ref(false)
const selectedRange = ref('30')
const startDate = ref('')
const endDate = ref('')
const recordKind = ref<PublicKeyUsageRecordKind>('success')
const modelFilter = ref<string | number | boolean | null>('')
const statusFilter = ref<string | number | boolean | null>(null)
const records = ref<PublicKeyUsageRecord[]>([])
const recordsLoading = ref(false)
const recordPage = ref(1)
const recordPageSize = 20
const recordTotal = ref(0)
const selectedRecord = ref<PublicKeyUsageRecord | null>(null)
const exporting = ref(false)

let sessionEpoch = 0
let summaryController: AbortController | null = null
let recordsController: AbortController | null = null
let detailController: AbortController | null = null
let exportController: AbortController | null = null

const timezoneName = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

const InfoValue = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true } },
  setup: (props) => () => h('div', [h('p', { class: 'text-[10px] font-medium text-stone-400' }, props.label), h('p', { class: 'mt-1 text-xs font-semibold text-stone-700 dark:text-stone-200' }, props.value)]),
})

const StatCell = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true } },
  setup: (props) => () => h('div', { class: 'bg-white p-5 dark:bg-[#0d0d0d]' }, [h('p', { class: 'text-xs text-stone-400' }, props.label), h('p', { class: 'mt-2 text-xl font-bold tabular-nums' }, props.value)]),
})

const DetailCell = defineComponent({
  props: { label: { type: String, required: true }, value: { type: String, required: true }, mono: Boolean },
  setup: (props) => () => h('div', { class: 'bg-white p-4 dark:bg-[#0d0d0d]' }, [h('p', { class: 'text-[10px] font-medium text-stone-400' }, props.label), h('p', { class: ['mt-1 break-words text-sm text-stone-700 dark:text-stone-200', props.mono && 'font-mono text-xs'] }, props.value)]),
})

const BudgetCell = defineComponent({
  props: { label: { type: String, required: true }, limit: { type: Object as () => PublicKeyUsageLimit, required: true } },
  setup: (props) => () => {
    const unlimited = props.limit.limit <= 0
    const percent = unlimited ? 0 : Math.min(100, Math.max(0, (props.limit.used / props.limit.limit) * 100))
    return h('div', { class: 'bg-white p-5 dark:bg-[#0d0d0d]' }, [
      h('p', { class: 'text-xs font-semibold text-stone-500 dark:text-stone-400' }, props.label),
      h('p', { class: 'mt-3 text-xl font-bold tabular-nums' }, unlimited ? t('keyUsage.unlimited') : formatMoney(props.limit.limit)),
      h('p', { class: 'mt-2 text-xs text-stone-400' }, `${t('keyUsage.usedQuota')} ${formatMoney(props.limit.used)}${unlimited ? '' : ` · ${t('keyUsage.remainingQuota')} ${formatMoney(props.limit.remaining)}`}`),
      h('div', { class: 'mt-4 h-1.5 overflow-hidden rounded-full bg-stone-100 dark:bg-white/10' }, [h('span', { class: 'block h-full rounded-full bg-emerald-500', style: { width: `${percent}%` } })]),
    ])
  },
})

const dateRanges = computed(() => [
  { value: 'today', label: t('keyUsage.dateRangeToday') },
  { value: '7', label: t('keyUsage.dateRange7d') },
  { value: '30', label: t('keyUsage.dateRange30d') },
  { value: '90', label: t('keyUsage.dateRange90d') },
  { value: 'custom', label: t('keyUsage.dateRangeCustom') },
])

const recordKindOptions = computed(() => {
  const items = [{ value: 'success', label: t('keyUsage.successRecords') }]
  if (summary.value?.error_records_available) items.push({ value: 'error', label: t('keyUsage.errorRecords') })
  return items
})

const modelOptions = computed(() => {
  const models = new Set<string>()
  summary.value?.models.forEach((item) => item.model && models.add(item.model))
  summary.value?.access_groups.forEach((group) => group.models.forEach((model) => models.add(model)))
  return [...models].sort().map((model) => ({ value: model, label: model }))
})

const statusOptions = computed(() => [400, 401, 403, 404, 408, 429, 500, 502, 503, 504].map((value) => ({ value, label: String(value) })))
const ipAccessLabel = computed(() => {
  if (!summary.value) return '—'
  if (summary.value.identity.ip_access_mode === 'whitelist') return `${t('keyUsage.whitelist')} · ${summary.value.identity.whitelist_size}`
  if (summary.value.identity.ip_access_mode === 'blacklist') return `${t('keyUsage.blacklist')} · ${summary.value.identity.blacklist_size}`
  return t('keyUsage.unrestricted')
})
const trendMax = computed(() => Math.max(0, ...(summary.value?.trend.map((item) => item.actual_cost) || [])))
const modelMax = computed(() => Math.max(0, ...(summary.value?.models.map((item) => item.actual_cost) || [])))

function localDateString(date: Date) {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function setRangeDays(days: number) {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - (days - 1))
  startDate.value = localDateString(start)
  endDate.value = localDateString(end)
}

function applyPresetRange(value: string) {
  selectedRange.value = value
  if (value === 'custom') return
  setRangeDays(value === 'today' ? 1 : Number(value))
  refreshAll()
}

async function createSession() {
  let submittedKey = apiKey.value.trim()
  if (!submittedKey || sessionCreating.value || sessionDeleting.value) return
  const epoch = invalidateSessionRequests()
  clearQueryData()
  sessionCreating.value = true
  try {
    await publicKeyUsageAPI.createSession(submittedKey)
    submittedKey = ''
    if (epoch !== sessionEpoch) return
    hasSession.value = true
    appStore.showSuccess(t('keyUsage.querySuccess'))
    await refreshAll()
  } catch (error) {
    if (epoch !== sessionEpoch) return
    appStore.showError(localizedAPIErrorMessage(error, t('keyUsage.queryFailedRetry'), true))
  } finally {
    submittedKey = ''
    apiKey.value = ''
    keyVisible.value = false
    sessionCreating.value = false
  }
}

async function restoreSession() {
  const epoch = invalidateSessionRequests()
  clearQueryData()
  try {
    const session = await publicKeyUsageAPI.getSession()
    if (epoch !== sessionEpoch) return
    hasSession.value = session.valid
    if (session.valid) await refreshAll()
  } catch {
    if (epoch === sessionEpoch) hasSession.value = false
  } finally {
    if (epoch === sessionEpoch) sessionChecking.value = false
  }
}

async function exitQuery() {
  sessionDeleting.value = true
  resetQueryState()
  try {
    await publicKeyUsageAPI.deleteSession()
  } catch {
    // The local view is still cleared. A stale server-side session remains
    // bounded by its short idle and absolute expiry.
    appStore.showWarning(t('keyUsage.exitRevokeFailed'))
  } finally {
    sessionDeleting.value = false
  }
}

function resetQueryState() {
  invalidateSessionRequests()
  clearQueryData()
  sessionChecking.value = false
}

function clearQueryData() {
  hasSession.value = false
  summary.value = null
  records.value = []
  recordTotal.value = 0
  selectedRecord.value = null
  apiKey.value = ''
}

function invalidateSessionRequests() {
  sessionEpoch += 1
  summaryController?.abort()
  recordsController?.abort()
  detailController?.abort()
  exportController?.abort()
  summaryController = null
  recordsController = null
  detailController = null
  exportController = null
  summaryLoading.value = false
  recordsLoading.value = false
  exporting.value = false
  return sessionEpoch
}

async function refreshAll() {
  if (!hasSession.value || !startDate.value || !endDate.value) return
  summaryController?.abort()
  recordsController?.abort()
  recordsController = null
  recordsLoading.value = false
  const controller = new AbortController()
  summaryController = controller
  const epoch = sessionEpoch
  const start = startDate.value
  const end = endDate.value
  summaryLoading.value = true
  try {
    const nextSummary = await publicKeyUsageAPI.getSummary({ start_date: start, end_date: end, timezone: timezoneName }, controller.signal)
    if (controller.signal.aborted || epoch !== sessionEpoch || !hasSession.value) return
    summary.value = nextSummary
    records.value = []
    recordTotal.value = 0
    if (recordKind.value === 'error' && !summary.value.error_records_available) recordKind.value = 'success'
    recordPage.value = 1
    await loadRecords({ epoch, startDate: start, endDate: end })
  } catch (error) {
    if (controller.signal.aborted || epoch !== sessionEpoch) return
    if (isSessionExpired(error)) {
      resetQueryState()
      appStore.showInfo(t('keyUsage.sessionExpired'))
    } else {
      appStore.showError(localizedAPIErrorMessage(error, t('keyUsage.queryFailedRetry')))
    }
  } finally {
    if (summaryController === controller) {
      summaryController = null
      summaryLoading.value = false
    }
  }
}

async function loadRecords(snapshot?: { epoch: number; startDate: string; endDate: string }) {
  if (!hasSession.value) return
  recordsController?.abort()
  const controller = new AbortController()
  recordsController = controller
  const epoch = snapshot?.epoch ?? sessionEpoch
  const queryStartDate = snapshot?.startDate ?? startDate.value
  const queryEndDate = snapshot?.endDate ?? endDate.value
  const queryKind = recordKind.value
  const queryModel = typeof modelFilter.value === 'string' ? modelFilter.value : undefined
  const queryStatus = typeof statusFilter.value === 'number' ? statusFilter.value : null
  const queryPage = recordPage.value
  recordsLoading.value = true
  try {
    const result = await publicKeyUsageAPI.listRecords({
      kind: queryKind,
      start_date: queryStartDate,
      end_date: queryEndDate,
      timezone: timezoneName,
      model: queryModel,
      status_code: queryStatus,
      page: queryPage,
      page_size: recordPageSize,
    }, controller.signal)
    if (controller.signal.aborted || epoch !== sessionEpoch || !hasSession.value) return
    records.value = result.items
    recordTotal.value = result.total
  } catch (error) {
    if (controller.signal.aborted || epoch !== sessionEpoch) return
    records.value = []
    recordTotal.value = 0
    if (isSessionExpired(error)) resetQueryState()
    else appStore.showError(localizedAPIErrorMessage(error, t('keyUsage.recordsFailed')))
  } finally {
    if (recordsController === controller) {
      recordsController = null
      recordsLoading.value = false
    }
  }
}

function onRecordFilterChange() {
  recordPage.value = 1
  loadRecords()
}

function changeRecordPage(page: number) {
  recordPage.value = page
  loadRecords()
}

async function openRecord(record: PublicKeyUsageRecord) {
  detailController?.abort()
  const controller = new AbortController()
  detailController = controller
  const epoch = sessionEpoch
  selectedRecord.value = record
  try {
    const detail = await publicKeyUsageAPI.getRecordDetail(record.kind, record.id, controller.signal)
    if (controller.signal.aborted || epoch !== sessionEpoch || !hasSession.value) return
    selectedRecord.value = detail
  } catch (error) {
    if (controller.signal.aborted || epoch !== sessionEpoch) return
    if (isSessionExpired(error)) {
      resetQueryState()
      appStore.showInfo(t('keyUsage.sessionExpired'))
    } else {
      appStore.showError(localizedAPIErrorMessage(error, t('keyUsage.detailFailed')))
    }
  } finally {
    if (detailController === controller) detailController = null
  }
}

async function exportRecords() {
  if (exporting.value) return
  exportController?.abort()
  const controller = new AbortController()
  exportController = controller
  const epoch = sessionEpoch
  exporting.value = true
  try {
    const blob = await publicKeyUsageAPI.exportRecords({
      kind: recordKind.value,
      start_date: startDate.value,
      end_date: endDate.value,
      timezone: timezoneName,
      model: typeof modelFilter.value === 'string' ? modelFilter.value : undefined,
      status_code: typeof statusFilter.value === 'number' ? statusFilter.value : null,
    }, controller.signal)
    if (controller.signal.aborted || epoch !== sessionEpoch || !hasSession.value) return
    const href = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = href
    link.download = `key-usage-${recordKind.value}-${startDate.value}-to-${endDate.value}.csv`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(href)
  } catch (error) {
    if (controller.signal.aborted || epoch !== sessionEpoch) return
    if (isSessionExpired(error)) {
      resetQueryState()
      appStore.showInfo(t('keyUsage.sessionExpired'))
    } else {
      appStore.showError(localizedAPIErrorMessage(error, t('keyUsage.exportFailed')))
    }
  } finally {
    if (exportController === controller) {
      exportController = null
      exporting.value = false
    }
  }
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  isDark.value = savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', isDark.value)
}

function formatMoney(value: number) {
  if (value < 0) return t('keyUsage.unlimited')
  return new Intl.NumberFormat(undefined, { style: 'currency', currency: 'USD', minimumFractionDigits: 2, maximumFractionDigits: 4 }).format(value || 0)
}

function formatNumber(value: number) {
  return new Intl.NumberFormat().format(value || 0)
}

function formatDate(value?: string) {
  if (!value) return t('keyUsage.never')
  return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit' }).format(new Date(value))
}

function formatDateTime(value?: string) {
  if (!value) return '—'
  return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).format(new Date(value))
}

function formatMilliseconds(value?: number) {
  return typeof value === 'number' ? `${formatNumber(value)} ms` : '—'
}

function statusLabel(status: string) {
  const key = `keyUsage.status.${status}`
  const translated = t(key)
  return translated === key ? status : translated
}

function limitPercent(limit: PublicKeyUsageLimit) {
  if (limit.limit <= 0) return 0
  return Math.min(100, Math.max(0, (limit.used / limit.limit) * 100))
}

function trendHeight(value: number) {
  if (trendMax.value <= 0) return 0
  return Math.max(3, (value / trendMax.value) * 100)
}

function modelWidth(value: number) {
  if (modelMax.value <= 0) return 0
  return Math.max(2, (value / modelMax.value) * 100)
}

function apiErrorStatus(error: unknown) {
  if (!error || typeof error !== 'object') return undefined
  return (error as { response?: { status?: number } }).response?.status
}

function localizedAPIErrorMessage(error: unknown, fallback: string, invalidKeyOnUnauthorized = false) {
  const status = apiErrorStatus(error)
  if (status === 401) {
    return t(invalidKeyOnUnauthorized ? 'keyUsage.invalidKey' : 'keyUsage.sessionExpired')
  }
  if (status === 400) return t('keyUsage.invalidRequest')
  if (status === 403) return t('keyUsage.accessDenied')
  if (status === 404) return t('keyUsage.dataNotFound')
  if (status === 429) return t('keyUsage.tooManyRequests')
  if (status !== undefined && status >= 500) return t('keyUsage.serviceUnavailable')
  return status === undefined ? t('keyUsage.networkError') : fallback
}

function isSessionExpired(error: unknown) {
  return apiErrorStatus(error) === 401
}

onMounted(() => {
  initTheme()
  setRangeDays(30)
  if (!appStore.publicSettingsLoaded) appStore.fetchPublicSettings()
  restoreSession()
})

onBeforeUnmount(() => {
  invalidateSessionRequests()
})
</script>
