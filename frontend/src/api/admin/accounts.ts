/**
 * Admin Accounts API endpoints
 * Handles AI platform account management for administrators
 */

import { apiClient } from '../client'
import type {
  Account,
  CreateAccountRequest,
  UpdateAccountRequest,
  PaginatedResponse,
  AccountUsageInfo,
  WindowStats,
  ClaudeModel,
  AccountUsageStatsResponse,
  TempUnschedulableStatus,
  AdminDataPayload,
  AdminDataImportResult,
  CodexSessionImportRequest,
  CodexSessionImportResult,
  OpenAICodexPATCreateRequest,
  CheckMixedChannelRequest,
  CheckMixedChannelResponse,
  UpstreamBillingProbeResult,
  UpstreamBillingProbeSettings
} from '@/types'
import type {
  UpstreamBalanceSnapshot,
  UpstreamCostProfile,
  UpstreamPriceReferenceCurrency
} from '@/utils/upstreamCost'

/**
 * List all accounts with pagination
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param filters - Optional filters
 * @returns Paginated list of accounts
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    search?: string
    privacy_mode?: string
    lite?: string
    include_scheduler_score?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<Account>> {
  const { data } = await apiClient.get<PaginatedResponse<Account>>('/admin/accounts', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

export interface AccountListWithEtagResult {
  notModified: boolean
  etag: string | null
  data: PaginatedResponse<Account> | null
}

export async function listWithEtag(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    search?: string
    privacy_mode?: string
    lite?: string
    include_scheduler_score?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
    etag?: string | null
  }
): Promise<AccountListWithEtagResult> {
  const headers: Record<string, string> = {}
  if (options?.etag) {
    headers['If-None-Match'] = options.etag
  }

  const response = await apiClient.get<PaginatedResponse<Account>>('/admin/accounts', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    headers,
    signal: options?.signal,
    validateStatus: (status) => (status >= 200 && status < 300) || status === 304
  })

  const etagHeader = typeof response.headers?.etag === 'string' ? response.headers.etag : null
  if (response.status === 304) {
    return {
      notModified: true,
      etag: etagHeader,
      data: null
    }
  }

  return {
    notModified: false,
    etag: etagHeader,
    data: response.data
  }
}

export async function listArchived(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    search?: string
    privacy_mode?: string
    lite?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<Account>> {
  const { data } = await apiClient.get<PaginatedResponse<Account>>('/admin/accounts/archived', {
    params: {
      page,
      page_size: pageSize,
      ...filters
    },
    signal: options?.signal
  })
  return data
}

/**
 * Get account by ID
 * @param id - Account ID
 * @returns Account details
 */
export async function getById(id: number): Promise<Account> {
  const { data } = await apiClient.get<Account>(`/admin/accounts/${id}`)
  return data
}

/**
 * Create new account
 * @param accountData - Account data
 * @returns Created account
 */
export async function create(accountData: CreateAccountRequest): Promise<Account> {
  const { data } = await apiClient.post<Account>('/admin/accounts', accountData)
  return data
}

/**
 * Duplicate an account while keeping credentials on the server.
 * @param id - Source account ID
 * @returns Newly created account
 */
const duplicateOperationKeys = new Map<number, string>()

function duplicateOperationStorageKey(id: number): string {
  return `sub2api:admin:account-duplicate:${id}`
}

function getStoredDuplicateOperationKey(id: number): string | null {
  try {
    return globalThis.sessionStorage?.getItem(duplicateOperationStorageKey(id)) ?? null
  } catch {
    return null
  }
}

function storeDuplicateOperationKey(id: number, key: string | null): void {
  try {
    if (key) globalThis.sessionStorage?.setItem(duplicateOperationStorageKey(id), key)
    else globalThis.sessionStorage?.removeItem(duplicateOperationStorageKey(id))
  } catch {
    // In-memory retry protection still works when browser storage is unavailable.
  }
}

export async function duplicate(id: number): Promise<Account> {
  let idempotencyKey = duplicateOperationKeys.get(id) ?? getStoredDuplicateOperationKey(id)
  if (!idempotencyKey) {
    const requestID = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
    idempotencyKey = `account-duplicate-${id}-${requestID}`
  }
  duplicateOperationKeys.set(id, idempotencyKey)
  storeDuplicateOperationKey(id, idempotencyKey)
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/duplicate`, undefined, {
    headers: { 'Idempotency-Key': idempotencyKey }
  })
  duplicateOperationKeys.delete(id)
  storeDuplicateOperationKey(id, null)
  return data
}

/**
 * Update account
 * @param id - Account ID
 * @param updates - Fields to update
 * @returns Updated account
 */
export async function update(id: number, updates: UpdateAccountRequest): Promise<Account> {
  const { data } = await apiClient.put<Account>(`/admin/accounts/${id}`, updates)
  return data
}

export async function updateUpstreamCostProfile(id: number, profile: UpstreamCostProfile): Promise<Account> {
  const { data } = await apiClient.patch<Account>(`/admin/accounts/${id}/upstream-cost-profile`, profile)
  return data
}

export async function refreshUpstreamBalance(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/upstream-balance/refresh`)
  return data
}

export type { UpstreamBalanceSnapshot }

export type UpstreamRechargeRecordType = 'recharge' | 'bonus' | 'adjustment'

export interface UpstreamRechargeRecord {
  id: number
  account_id?: number | null
  cost_pool_id?: number | null
  account_name_snapshot: string
  account_platform_snapshot: string
  account_type_snapshot: string
  type: UpstreamRechargeRecordType
  paid_amount: number
  paid_currency: string
  received_credit_amount: number
  received_credit_currency: string
  reference_fx_rate: number
  effective_cny_per_usd?: number | null
  recharge_discount?: number | null
  recorded_at: string
  note?: string | null
  created_by?: number | null
  created_at: string
  updated_at: string
}

export interface UpstreamRechargeSummary {
  record_count: number
  total_paid_amount: number
  total_received_credit_amount: number
  weighted_effective_cny_per_usd?: number | null
  weighted_recharge_discount?: number | null
  latest_effective_cny_per_usd?: number | null
  latest_recharge_discount?: number | null
  latest_recorded_at?: string | null
  reference_fx_rate: number
}

export interface UpstreamRechargeRecordsResult {
  items: UpstreamRechargeRecord[]
  summary: UpstreamRechargeSummary
  cost_pool_id?: number | null
  deprecated?: boolean
}

export interface UpstreamSupplier {
  id: number
  name: string
  status: string
  note?: string | null
  is_system?: boolean
  created_at: string
  updated_at: string
  archived_at?: string | null
}

export interface UpstreamSupplierPayload {
  name: string
  note?: string | null
  default_effective_cny_per_usd?: number
  default_reference_fx_rate?: number
}

export interface UpstreamCostPool {
  id: number
  supplier_id: number
  supplier_name: string
  name: string
  is_default: boolean
  status: string
  base_currency: string
  credit_currency: string
  reference_fx_rate: number
  default_effective_cny_per_usd: number
  default_reference_fx_rate: number
  cost_method: string
  current_effective_cny_per_usd?: number | null
  current_snapshot_id?: number | null
  balance_query_enabled: boolean
  balance_provider?: string | null
  balance_endpoint?: string | null
  balance_auth_mode?: string | null
  balance_auth_header?: string | null
  balance_low_threshold?: number | null
  last_balance_snapshot?: Record<string, unknown> | null
  note?: string | null
  binding_count: number
  record_count: number
  created_at: string
  updated_at: string
  archived_at?: string | null
}

export interface UpstreamCostModelFamilyMultiplier {
  family: string
  group_multiplier: number
  note?: string | null
}

export interface UpstreamAccountCostBinding {
  id: number
  account_id: number
  account_name?: string
  account_platform?: string
  cost_pool_id: number
  cost_pool_name?: string
  supplier_id?: number
  supplier_name?: string
  status: string
  default_multiplier: number
  upstream_group_name?: string | null
  price_reference_currency: UpstreamPriceReferenceCurrency
  price_reference_confirmed: boolean
  upstream_group_multiplier?: number
  model_family_multipliers: UpstreamCostModelFamilyMultiplier[]
  note?: string | null
  valid_from: string
  valid_to?: string | null
  created_at: string
  updated_at: string
}

export interface UpstreamSupplierBindingPayload {
  supplier_id?: number | null
  supplier_name?: string | null
  cost_pool_id?: number | null
  upstream_group_name?: string | null
  price_reference_currency?: UpstreamPriceReferenceCurrency
  upstream_group_multiplier?: number
  default_multiplier?: number
  model_families?: UpstreamCostModelFamilyMultiplier[]
  note?: string | null
}

export interface UpstreamRechargeRecordPayload {
  account_id?: number | null
  type?: UpstreamRechargeRecordType
  paid_amount: number
  paid_currency?: string
  received_credit_amount: number
  received_credit_currency?: string
  reference_fx_rate?: number
  recorded_at?: string | null
  note?: string | null
}

export async function listUpstreamRechargeRecords(id: number): Promise<UpstreamRechargeRecordsResult> {
  const { data } = await apiClient.get<UpstreamRechargeRecordsResult>(`/admin/accounts/${id}/recharge-records`)
  return data
}

export async function createUpstreamRechargeRecord(
  id: number,
  payload: UpstreamRechargeRecordPayload
): Promise<UpstreamRechargeRecord> {
  const { data } = await apiClient.post<UpstreamRechargeRecord>(`/admin/accounts/${id}/recharge-records`, payload)
  return data
}

export async function listUpstreamSuppliers(): Promise<UpstreamSupplier[]> {
  const { data } = await apiClient.get<{ items: UpstreamSupplier[] }>('/admin/upstream-suppliers')
  return data.items
}

export async function createUpstreamSupplier(payload: UpstreamSupplierPayload): Promise<UpstreamSupplier> {
  const { data } = await apiClient.post<UpstreamSupplier>('/admin/upstream-suppliers', payload)
  return data
}

export interface UpstreamSupplierUpdatePayload {
  name?: string
  note?: string | null
  status?: 'active' | 'archived'
  default_effective_cny_per_usd?: number
  default_reference_fx_rate?: number
}

export async function updateUpstreamSupplier(
  id: number,
  payload: UpstreamSupplierUpdatePayload
): Promise<UpstreamSupplier> {
  const { data } = await apiClient.patch<UpstreamSupplier>(`/admin/upstream-suppliers/${id}`, payload)
  return data
}

export async function deleteUpstreamSupplier(id: number): Promise<void> {
  await apiClient.delete(`/admin/upstream-suppliers/${id}`)
}

export async function listUpstreamCostPools(): Promise<UpstreamCostPool[]> {
  const { data } = await apiClient.get<{ items: UpstreamCostPool[] }>('/admin/upstream-cost-pools')
  return data.items
}

export async function listUpstreamCostPoolAccounts(poolId: number): Promise<UpstreamAccountCostBinding[]> {
  const { data } = await apiClient.get<{ items: UpstreamAccountCostBinding[] }>(
    `/admin/upstream-cost-pools/${poolId}/accounts`
  )
  return data.items
}

export async function listUpstreamCostPoolRechargeRecords(poolId: number): Promise<UpstreamRechargeRecordsResult> {
  const { data } = await apiClient.get<UpstreamRechargeRecordsResult>(
    `/admin/upstream-cost-pools/${poolId}/recharge-records`
  )
  return data
}

export async function createUpstreamCostPoolRechargeRecord(
  poolId: number,
  payload: UpstreamRechargeRecordPayload
): Promise<UpstreamRechargeRecord> {
  const { data } = await apiClient.post<UpstreamRechargeRecord>(
    `/admin/upstream-cost-pools/${poolId}/recharge-records`,
    payload
  )
  return data
}

export async function updateUpstreamCostPoolRechargeRecord(
  poolId: number,
  recordId: number,
  payload: UpstreamRechargeRecordPayload
): Promise<UpstreamRechargeRecord> {
  const { data } = await apiClient.put<UpstreamRechargeRecord>(
    `/admin/upstream-cost-pools/${poolId}/recharge-records/${recordId}`,
    payload
  )
  return data
}

export async function deleteUpstreamCostPoolRechargeRecord(poolId: number, recordId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/upstream-cost-pools/${poolId}/recharge-records/${recordId}`
  )
  return data
}

export async function getAccountUpstreamCostBinding(id: number): Promise<UpstreamAccountCostBinding> {
  const { data } = await apiClient.get<UpstreamAccountCostBinding>(`/admin/accounts/${id}/upstream-cost-binding`)
  return data
}

export async function updateAccountUpstreamSupplierBinding(
  id: number,
  payload: UpstreamSupplierBindingPayload
): Promise<UpstreamAccountCostBinding | null> {
  const { data } = await apiClient.put<UpstreamAccountCostBinding | null>(
    `/admin/accounts/${id}/upstream-supplier-binding`,
    payload
  )
  return data
}

export async function updateUpstreamRechargeRecord(
  id: number,
  recordId: number,
  payload: UpstreamRechargeRecordPayload
): Promise<UpstreamRechargeRecord> {
  const { data } = await apiClient.put<UpstreamRechargeRecord>(`/admin/accounts/${id}/recharge-records/${recordId}`, payload)
  return data
}

export async function deleteUpstreamRechargeRecord(id: number, recordId: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/accounts/${id}/recharge-records/${recordId}`)
  return data
}

/**
 * Check mixed-channel risk for account-group binding.
 */
export async function checkMixedChannelRisk(
  payload: CheckMixedChannelRequest
): Promise<CheckMixedChannelResponse> {
  const { data } = await apiClient.post<CheckMixedChannelResponse>('/admin/accounts/check-mixed-channel', payload)
  return data
}

/**
 * Delete account
 * @param id - Account ID
 * @returns Success confirmation
 */
export async function deleteAccount(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/accounts/${id}`)
  return data
}

export async function archiveAccount(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/accounts/${id}/archive`)
  return data
}

export async function restoreAccount(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/restore`)
  return data
}

/**
 * Toggle account status
 * @param id - Account ID
 * @param status - New status
 * @returns Updated account
 */
export async function toggleStatus(id: number, status: 'active' | 'inactive' | 'disabled'): Promise<Account> {
  return update(id, { status })
}

/**
 * Test account connectivity
 * @param id - Account ID
 * @returns Test result
 */
export async function testAccount(id: number): Promise<{
  success: boolean
  message: string
  latency_ms?: number
}> {
  const { data } = await apiClient.post<{
    success: boolean
    message: string
    latency_ms?: number
  }>(`/admin/accounts/${id}/test`)
  return data
}

/**
 * Refresh account credentials
 * @param id - Account ID
 * @returns Updated account
 */
export async function refreshCredentials(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/refresh`)
  return data
}

/**
 * Apply OAuth credentials after re-authorization.
 *
 * Unlike `update()`, this endpoint:
 * - never overwrites the whole `extra` JSONB (merges incrementally instead),
 *   so persistent settings like `base_rpm`, `window_cost_limit`, `max_sessions`,
 *   `quota_*` and `privacy_mode` are preserved
 * - clears the account error and invalidates the token cache server-side
 */
export async function applyOAuthCredentials(
  id: number,
  payload: {
    type: 'oauth' | 'setup-token'
    credentials: Record<string, unknown>
    extra?: Record<string, unknown>
  }
): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/apply-oauth-credentials`,
    payload
  )
  return data
}

/**
 * Get account usage statistics
 * @param id - Account ID
 * @param days - Number of days (default: 30)
 * @returns Account usage statistics with history, summary, and models
 */
export async function getStats(id: number, days: number = 30): Promise<AccountUsageStatsResponse> {
  const { data } = await apiClient.get<AccountUsageStatsResponse>(`/admin/accounts/${id}/stats`, {
    params: { days }
  })
  return data
}

/**
 * Clear account error
 * @param id - Account ID
 * @returns Updated account
 */
export async function clearError(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/clear-error`)
  return data
}

/**
 * Get account usage information (5h/7d window)
 * @param id - Account ID
 * @returns Account usage info
 */
export async function getUsage(id: number, source?: 'passive' | 'active', force?: boolean): Promise<AccountUsageInfo> {
  const params: Record<string, string> = {}
  if (source) params.source = source
  if (force) params.force = 'true'
  const { data } = await apiClient.get<AccountUsageInfo>(`/admin/accounts/${id}/usage`, {
    params: Object.keys(params).length > 0 ? params : undefined
  })
  return data
}

/**
 * Clear account rate limit status
 * @param id - Account ID
 * @returns Updated account
 */
export async function clearRateLimit(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/clear-rate-limit`
  )
  return data
}

/**
 * Clear the rate limit of a single model (scope) on an account
 * @param id - Account ID
 * @param scope - Model rate-limit scope key (the key under extra.model_rate_limits)
 * @returns Updated account
 */
export async function clearModelRateLimit(id: number, scope: string): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/clear-model-rate-limit`,
    { scope }
  )
  return data
}

/**
 * Recover account runtime state in one call
 * @param id - Account ID
 * @returns Updated account
 */
export async function recoverState(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/recover-state`)
  return data
}

/**
 * Reset account quota usage
 * @param id - Account ID
 * @returns Updated account
 */
export async function resetAccountQuota(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(
    `/admin/accounts/${id}/reset-quota`
  )
  return data
}

/**
 * Get temporary unschedulable status
 * @param id - Account ID
 * @returns Status with detail state if active
 */
export async function getTempUnschedulableStatus(id: number): Promise<TempUnschedulableStatus> {
  const { data } = await apiClient.get<TempUnschedulableStatus>(
    `/admin/accounts/${id}/temp-unschedulable`
  )
  return data
}

/**
 * Reset temporary unschedulable status
 * @param id - Account ID
 * @returns Success confirmation
 */
export async function resetTempUnschedulable(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    `/admin/accounts/${id}/temp-unschedulable`
  )
  return data
}

/**
 * Generate OAuth authorization URL
 * @param endpoint - API endpoint path
 * @param config - Proxy configuration
 * @returns Auth URL and session ID
 */
export async function generateAuthUrl(
  endpoint: string,
  config: { proxy_id?: number }
): Promise<{ auth_url: string; session_id: string }> {
  const { data } = await apiClient.post<{ auth_url: string; session_id: string }>(endpoint, config)
  return data
}

/**
 * Exchange authorization code for tokens
 * @param endpoint - API endpoint path
 * @param exchangeData - Session ID, code, and optional proxy config
 * @returns Token information
 */
export async function exchangeCode(
  endpoint: string,
  exchangeData: { session_id: string; code: string; state?: string; proxy_id?: number }
): Promise<Record<string, unknown>> {
  const { data } = await apiClient.post<Record<string, unknown>>(endpoint, exchangeData)
  return data
}

/**
 * Batch create accounts
 * @param accounts - Array of account data
 * @returns Results of batch creation
 */
export async function batchCreate(accounts: CreateAccountRequest[]): Promise<{
  success: number
  failed: number
  results: Array<{ success: boolean; account?: Account; error?: string }>
}> {
  const { data } = await apiClient.post<{
    success: number
    failed: number
    results: Array<{ success: boolean; account?: Account; error?: string }>
  }>('/admin/accounts/batch', { accounts })
  return data
}

/**
 * Batch update credentials fields for multiple accounts
 * @param request - Batch update request containing account IDs, field name, and value
 * @returns Results of batch update
 */
export async function batchUpdateCredentials(request: {
  account_ids: number[]
  field: string
  value: any
}): Promise<{
  success: number
  failed: number
  results: Array<{ account_id: number; success: boolean; error?: string }>
}> {
  const { data } = await apiClient.post<{
    success: number
    failed: number
    results: Array<{ account_id: number; success: boolean; error?: string }>
  }>('/admin/accounts/batch-update-credentials', request)
  return data
}

/**
 * Bulk update multiple accounts
 * @param accountIds - Array of account IDs
 * @param updates - Fields to update
 * @returns Success confirmation
 */
export async function bulkUpdate(
  accountIdsOrPayload: number[] | Record<string, unknown>,
  updates?: Record<string, unknown>
): Promise<{
  success: number
  failed: number
  success_ids?: number[]
  failed_ids?: number[]
  results: Array<{ account_id: number; success: boolean; error?: string }>
  }> {
  const payload = Array.isArray(accountIdsOrPayload)
    ? {
        account_ids: accountIdsOrPayload,
        ...(updates ?? {})
      }
    : accountIdsOrPayload
  const { data } = await apiClient.post<{
    success: number
    failed: number
    success_ids?: number[]
    failed_ids?: number[]
    results: Array<{ account_id: number; success: boolean; error?: string }>
  }>('/admin/accounts/bulk-update', payload)
  return data
}

/**
 * Get account today statistics
 * @param id - Account ID
 * @returns Today's stats (requests, tokens, cost)
 */
export async function getTodayStats(id: number): Promise<WindowStats> {
  const { data } = await apiClient.get<WindowStats>(`/admin/accounts/${id}/today-stats`)
  return data
}

export interface BatchTodayStatsResponse {
  stats: Record<string, WindowStats>
}

/**
 * 批量获取多个账号的今日统计
 * @param accountIds - 账号 ID 列表
 * @returns 以账号 ID（字符串）为键的统计映射
 */
export async function getBatchTodayStats(accountIds: number[]): Promise<BatchTodayStatsResponse> {
  const { data } = await apiClient.post<BatchTodayStatsResponse>('/admin/accounts/today-stats/batch', {
    account_ids: accountIds
  })
  return data
}

/**
 * Set account schedulable status
 * @param id - Account ID
 * @param schedulable - Whether the account should participate in scheduling
 * @returns Updated account
 */
export async function setSchedulable(id: number, schedulable: boolean): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/schedulable`, {
    schedulable
  })
  return data
}

/**
 * Get available models for an account
 * @param id - Account ID
 * @returns List of available models for this account
 */
export async function getAvailableModels(id: number): Promise<ClaudeModel[]> {
  const { data } = await apiClient.get<ClaudeModel[]>(`/admin/accounts/${id}/models`)
  return data
}

export interface ProbeModelsRequest {
  base_url: string
  api_key: string
}

export interface ProbeModelsResponse {
  models: string[]
}

export async function probeModels(request: ProbeModelsRequest): Promise<ProbeModelsResponse> {
  const { data } = await apiClient.post<ProbeModelsResponse>('/admin/accounts/probe-models', request)
  return data
}

export interface SyncUpstreamModelsResult {
  models: string[]
}

/**
 * Sync live supported models from the account's upstream model-list endpoint
 * @param id - Account ID
 * @returns List of model IDs returned by the upstream
 */
export async function syncUpstreamModels(id: number): Promise<SyncUpstreamModelsResult> {
  const { data } = await apiClient.post<SyncUpstreamModelsResult>(`/admin/accounts/${id}/models/sync-upstream`)
  return data
}

export interface SyncUpstreamPreviewParams {
  platform: string
  type: string
  base_url?: string
  api_key: string
}

/**
 * Preview upstream models without a saved account (create-flow)
 * @param params - Connection credentials
 * @returns List of model IDs returned by the upstream
 */
export async function syncUpstreamModelsPreview(params: SyncUpstreamPreviewParams): Promise<SyncUpstreamModelsResult> {
  const { data } = await apiClient.post<SyncUpstreamModelsResult>('/admin/accounts/models/sync-upstream-preview', params)
  return data
}

export interface CRSPreviewAccount {
  crs_account_id: string
  kind: string
  name: string
  platform: string
  type: string
}

export interface PreviewFromCRSResult {
  new_accounts: CRSPreviewAccount[]
  existing_accounts: CRSPreviewAccount[]
}

export async function previewFromCrs(params: {
  base_url: string
  username: string
  password: string
}): Promise<PreviewFromCRSResult> {
  const { data } = await apiClient.post<PreviewFromCRSResult>('/admin/accounts/sync/crs/preview', params)
  return data
}

export async function syncFromCrs(params: {
  base_url: string
  username: string
  password: string
  sync_proxies?: boolean
  selected_account_ids?: string[]
}): Promise<{
  created: number
  updated: number
  skipped: number
  failed: number
  items: Array<{
    crs_account_id: string
    kind: string
    name: string
    action: string
    error?: string
  }>
}> {
  const { data } = await apiClient.post<{
    created: number
    updated: number
    skipped: number
    failed: number
    items: Array<{
      crs_account_id: string
      kind: string
      name: string
      action: string
      error?: string
    }>
  }>('/admin/accounts/sync/crs', params, {
    timeout: 180000 // 180s timeout: sync refreshes each existing account's OAuth token serially
  })
  return data
}

export async function exportData(options?: {
  ids?: number[]
  filters?: {
    platform?: string
    type?: string
    status?: string
    group?: string
    privacy_mode?: string
    search?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  }
  includeProxies?: boolean
}): Promise<AdminDataPayload> {
  const params: Record<string, string> = {}
  if (options?.ids && options.ids.length > 0) {
    params.ids = options.ids.join(',')
  } else if (options?.filters) {
    const { platform, type, status, group, privacy_mode, search, sort_by, sort_order } = options.filters
    if (platform) params.platform = platform
    if (type) params.type = type
    if (status) params.status = status
    if (group) params.group = group
    if (privacy_mode) params.privacy_mode = privacy_mode
    if (search) params.search = search
    if (sort_by) params.sort_by = sort_by
    if (sort_order) params.sort_order = sort_order
  }
  if (options?.includeProxies === false) {
    params.include_proxies = 'false'
  }
  const { data } = await apiClient.get<AdminDataPayload>('/admin/accounts/data', { params })
  return data
}

export async function importData(payload: {
  data: AdminDataPayload
  skip_default_group_bind?: boolean
}): Promise<AdminDataImportResult> {
  const { data } = await apiClient.post<AdminDataImportResult>('/admin/accounts/data', {
    data: payload.data,
    skip_default_group_bind: payload.skip_default_group_bind
  })
  return data
}

export async function importCodexSession(payload: CodexSessionImportRequest): Promise<CodexSessionImportResult> {
  const { data } = await apiClient.post<CodexSessionImportResult>('/admin/accounts/import/codex-session', payload, {
    timeout: 120000 // 120s timeout for large session imports
  })
  return data
}

export async function createOpenAICodexPAT(payload: OpenAICodexPATCreateRequest): Promise<Account> {
  const { data } = await apiClient.post<Account>('/admin/openai/create-from-codex-pat', payload)
  return data
}

/**
 * Get Antigravity default model mapping from backend
 * @returns Default model mapping (from -> to)
 */
export async function getAntigravityDefaultModelMapping(): Promise<Record<string, string>> {
  const { data } = await apiClient.get<Record<string, string>>(
    '/admin/accounts/antigravity/default-model-mapping'
  )
  return data
}

/**
 * Refresh OpenAI token using refresh token
 * @param refreshToken - The refresh token
 * @param proxyId - Optional proxy ID
 * @returns Token information including access_token, email, etc.
 */
export async function refreshOpenAIToken(
  refreshToken: string,
  proxyId?: number | null,
  endpoint: string = '/admin/openai/refresh-token',
  clientId?: string
): Promise<Record<string, unknown>> {
  const payload: { refresh_token: string; proxy_id?: number; client_id?: string } = {
    refresh_token: refreshToken
  }
  if (proxyId) {
    payload.proxy_id = proxyId
  }
  if (clientId) {
    payload.client_id = clientId
  }
  const { data } = await apiClient.post<Record<string, unknown>>(endpoint, payload)
  return data
}

/**
 * Batch operation result type
 */
export interface BatchOperationResult {
  total: number
  success: number
  failed: number
  errors?: Array<{ account_id: number; error: string }>
  warnings?: Array<{ account_id: number; warning: string }>
}

/**
 * Revert account proxy to original before fallback
 * @param id - Account ID
 * @returns Success confirmation
 */
export async function revertProxyFallback(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(`/admin/accounts/${id}/revert-proxy-fallback`)
  return data
}

/**
 * Batch clear account errors
 * @param accountIds - Array of account IDs
 * @returns Batch operation result
 */
export async function batchClearError(accountIds: number[]): Promise<BatchOperationResult> {
  const { data } = await apiClient.post<BatchOperationResult>('/admin/accounts/batch-clear-error', {
    account_ids: accountIds
  })
  return data
}

/**
 * Batch refresh account credentials
 * @param accountIds - Array of account IDs
 * @returns Batch operation result
 */
export async function batchRefresh(accountIds: number[]): Promise<BatchOperationResult> {
  const { data } = await apiClient.post<BatchOperationResult>('/admin/accounts/batch-refresh', {
    account_ids: accountIds,
  }, {
    timeout: 120000  // 120s timeout for large batch refreshes
  })
  return data
}

/**
 * Set privacy for an Antigravity OAuth account
 * @param id - Account ID
 * @returns Updated account
 */
export async function setPrivacy(id: number): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${id}/set-privacy`)
  return data
}

/**
 * OpenAI / Codex rate-limit reset feature: query and reset upstream usage.
 */
export interface OpenAIRateLimitWindow {
  used_percent: number
  limit_window_seconds: number
  reset_after_seconds: number
  reset_at: number
}

export interface OpenAIRateLimit {
  allowed: boolean
  limit_reached: boolean
  primary_window?: OpenAIRateLimitWindow | null
  secondary_window?: OpenAIRateLimitWindow | null
}

export interface OpenAIAdditionalRateLimit {
  limit_name: string
  metered_feature: string
  rate_limit?: OpenAIRateLimit | null
}

export interface OpenAIRateLimitResetCreditDetail {
  expires_at?: string
}

export interface OpenAIRateLimitResetCredits {
  available_count: number
  credits?: OpenAIRateLimitResetCreditDetail[]
}

export interface OpenAIQuotaUsage {
  user_id?: string
  account_id?: string
  email?: string
  plan_type?: string
  rate_limit?: OpenAIRateLimit | null
  additional_rate_limits?: OpenAIAdditionalRateLimit[]
  rate_limit_reset_credits?: OpenAIRateLimitResetCredits | null
  fetched_at: number
}

export interface OpenAIQuotaResetCredit {
  id?: string
  reset_type?: string
  status?: string
  granted_at?: string
  expires_at?: string
  redeem_started_at?: string
  redeemed_at?: string
}

export interface OpenAIQuotaResetResult {
  code: string
  credit?: OpenAIQuotaResetCredit | null
  windows_reset: number
}

/**
 * Query OpenAI/Codex rate-limit usage for an OAuth account.
 */
export async function queryOpenAIQuota(id: number): Promise<OpenAIQuotaUsage> {
  const { data } = await apiClient.get<OpenAIQuotaUsage>(`/admin/openai/accounts/${id}/quota`)
  return data
}

/**
 * Consume one rate-limit-reset credit for an OpenAI/Codex OAuth account.
 */
export async function resetOpenAIQuota(id: number): Promise<OpenAIQuotaResetResult> {
  const { data } = await apiClient.post<OpenAIQuotaResetResult>(`/admin/openai/accounts/${id}/reset-quota`)
  return data
}

export interface SparkShadowCreatePayload {
  name?: string
  priority?: number
  concurrency?: number
  group_ids?: number[]
}

export async function createSparkShadow(parentId: number, payload: SparkShadowCreatePayload): Promise<Account> {
  const { data } = await apiClient.post<Account>(`/admin/accounts/${parentId}/shadow`, payload)
  return data
}

export async function getUpstreamBillingProbeSettings(): Promise<UpstreamBillingProbeSettings> {
  const { data } = await apiClient.get<UpstreamBillingProbeSettings>('/admin/accounts/upstream-billing-probe/settings')
  return data
}

export async function updateUpstreamBillingProbeSettings(
  settings: UpstreamBillingProbeSettings
): Promise<UpstreamBillingProbeSettings> {
  const { data } = await apiClient.put<UpstreamBillingProbeSettings>(
    '/admin/accounts/upstream-billing-probe/settings',
    settings
  )
  return data
}

export async function setUpstreamBillingProbeEnabled(id: number, enabled: boolean): Promise<void> {
  await apiClient.put(`/admin/accounts/${id}/upstream-billing-probe`, { enabled })
}

export async function probeUpstreamBilling(id: number): Promise<UpstreamBillingProbeResult> {
  const { data } = await apiClient.post<UpstreamBillingProbeResult>(`/admin/accounts/${id}/upstream-billing-probe`)
  return data
}

export async function probeUpstreamBillingBatch(accountIds: number[]): Promise<UpstreamBillingProbeResult[]> {
  const { data } = await apiClient.post<{ results: UpstreamBillingProbeResult[] }>(
    '/admin/accounts/upstream-billing-probe/batch',
    { account_ids: accountIds }
  )
  return data.results
}

export type ModelProtocol = 'anthropic_messages' | 'openai_chat_completions' | 'openai_responses'
export type ModelProtocolState = 'auto' | 'unknown' | 'supported' | 'unsupported'

export interface AccountModelProtocolCapability {
  id: number
  account_id: number
  upstream_model: string
  protocol: ModelProtocol
  override_state: Exclude<ModelProtocolState, 'unknown'>
  observed_state: Exclude<ModelProtocolState, 'auto'>
  effective_state: Exclude<ModelProtocolState, 'auto'>
  effective_source?: string
  observed_source?: string
  observed_at?: string
  created_at: string
  updated_at: string
}

export interface AccountModelProtocolCapabilitiesResponse {
  account_id: number
  items: AccountModelProtocolCapability[]
  warnings: string[]
  models?: string[]
  public_model_impacts: Record<string, AccountPublicModelImpact[]>
  orphan_upstream_models: string[]
}

export interface AccountPublicModelImpact {
  upstream_model: string
  public_model: string
  channel_id: number
  channel_name: string
  group_id: number
  group_name: string
  platform: string
}

export interface ModelProtocolOverrideInput {
  upstream_model: string
  protocol: ModelProtocol
  state: 'auto' | 'supported' | 'unsupported'
}

export async function getModelProtocolCapabilities(id: number): Promise<AccountModelProtocolCapabilitiesResponse> {
  const { data } = await apiClient.get<AccountModelProtocolCapabilitiesResponse>(
    `/admin/accounts/${id}/model-protocol-capabilities`
  )
  return data
}

export async function updateModelProtocolCapabilityOverrides(
  id: number,
  items: ModelProtocolOverrideInput[]
): Promise<AccountModelProtocolCapabilitiesResponse> {
  const overrides = items.map(({ upstream_model, protocol, state }) => ({
    upstream_model,
    protocol,
    state
  }))
  const { data } = await apiClient.put<AccountModelProtocolCapabilitiesResponse>(
    `/admin/accounts/${id}/model-protocol-capabilities/overrides`,
    { items: overrides }
  )
  return data
}

export async function syncModelProtocolCapabilities(id: number): Promise<AccountModelProtocolCapabilitiesResponse> {
  const { data } = await apiClient.post<AccountModelProtocolCapabilitiesResponse>(
    `/admin/accounts/${id}/model-protocol-capabilities/sync`
  )
  return data
}

export const accountsAPI = {
  list,
  listWithEtag,
  listArchived,
  getById,
  create,
  duplicate,
  update,
  updateUpstreamCostProfile,
  refreshUpstreamBalance,
  listUpstreamRechargeRecords,
  createUpstreamRechargeRecord,
  listUpstreamSuppliers,
  createUpstreamSupplier,
  updateUpstreamSupplier,
  deleteUpstreamSupplier,
  listUpstreamCostPools,
  listUpstreamCostPoolAccounts,
  listUpstreamCostPoolRechargeRecords,
  createUpstreamCostPoolRechargeRecord,
  updateUpstreamCostPoolRechargeRecord,
  deleteUpstreamCostPoolRechargeRecord,
  getAccountUpstreamCostBinding,
  updateAccountUpstreamSupplierBinding,
  updateUpstreamRechargeRecord,
  deleteUpstreamRechargeRecord,
  checkMixedChannelRisk,
  delete: deleteAccount,
  archive: archiveAccount,
  restore: restoreAccount,
  toggleStatus,
  testAccount,
  refreshCredentials,
  applyOAuthCredentials,
  getStats,
  clearError,
  getUsage,
  getTodayStats,
  getBatchTodayStats,
  clearRateLimit,
  clearModelRateLimit,
  recoverState,
  resetAccountQuota,
  getTempUnschedulableStatus,
  resetTempUnschedulable,
  setSchedulable,
  getAvailableModels,
  probeModels,
  syncUpstreamModels,
  syncUpstreamModelsPreview,
  generateAuthUrl,
  exchangeCode,
  refreshOpenAIToken,
  batchCreate,
  batchUpdateCredentials,
  bulkUpdate,
  previewFromCrs,
  syncFromCrs,
  exportData,
  importData,
  importCodexSession,
  createOpenAICodexPAT,
  getAntigravityDefaultModelMapping,
  batchClearError,
  batchRefresh,
  setPrivacy,
  revertProxyFallback,
  queryOpenAIQuota,
  resetOpenAIQuota,
  createSparkShadow,
  getUpstreamBillingProbeSettings,
  updateUpstreamBillingProbeSettings,
  setUpstreamBillingProbeEnabled,
  probeUpstreamBilling,
  probeUpstreamBillingBatch,
  getModelProtocolCapabilities,
  updateModelProtocolCapabilityOverrides,
  syncModelProtocolCapabilities
}

export default accountsAPI
