/**
 * Usage tracking API endpoints
 * Handles usage logs and statistics retrieval
 */

import { apiClient } from './client'
import type {
  UsageLog,
  UsageQueryParams,
  UsageStatsResponse,
  PaginatedResponse,
  TrendDataPoint,
  ModelStat,
  GroupStat,
  UsageRequestType,
  UserErrorRequest,
  UserErrorRequestDetail,
  UserErrorListParams
} from '@/types'

// ==================== Dashboard Types ====================

export interface PlatformDashboardStats {
  platform: string
  total_requests: number
  total_tokens: number
  total_actual_cost: number
  today_requests: number
  today_tokens: number
  today_actual_cost: number
}

export interface UserDashboardStats {
  total_api_keys: number
  active_api_keys: number
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_cost: number // 标准计费
  total_actual_cost: number // 实际扣除
  today_requests: number
  today_input_tokens: number
  today_output_tokens: number
  today_cache_creation_tokens: number
  today_cache_read_tokens: number
  today_tokens: number
  today_cost: number // 今日标准计费
  today_actual_cost: number // 今日实际扣除
  average_duration_ms: number
  rpm: number // 近5分钟平均每分钟请求数
  tpm: number // 近5分钟平均每分钟Token数
  by_platform?: PlatformDashboardStats[]
}

export interface TrendParams {
  start_date?: string
  end_date?: string
  start_time?: string
  end_time?: string
  granularity?: 'day' | 'hour'
  api_key_id?: number
  member_id?: number
  member_scope?: 'all' | 'assigned' | 'unassigned'
  model?: string
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
  billing_mode?: string | null
  timezone?: string
}

export interface TrendResponse {
  trend: TrendDataPoint[]
  start_date: string
  end_date: string
  granularity: string
}

export interface UserModelStat {
  model: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
  actual_cost: number
}

export interface ModelStatsResponse {
  models: UserModelStat[]
  start_date: string
  end_date: string
}

export interface ApiKeyDailyUsagePoint {
  date: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_tokens: number
  cost: number
  actual_cost: number
}

export interface ApiKeyDailyUsageResponse {
  items: ApiKeyDailyUsagePoint[]
  days: number
  start_date: string
  end_date: string
}

export type ApiKeyUsageTrendGranularity = 'hour' | 'day' | 'week' | 'month'

export interface ApiKeyUsageTrendParams {
  start_date?: string
  end_date?: string
  granularity?: ApiKeyUsageTrendGranularity
  timezone?: string
}

export interface ApiKeyUsageTrendResponse {
  items: TrendDataPoint[]
  granularity: ApiKeyUsageTrendGranularity
  start_date: string
  end_date: string
  timezone: string
}

export interface ApiKeyModelStatsParams {
  start_date?: string
  end_date?: string
  timezone?: string
}

export interface ApiKeyModelStatsResponse {
  models: UserModelStat[]
  start_date: string
  end_date: string
  timezone: string
}

export interface OwnerApiKeyAnalyticsParams {
  start_date?: string
  end_date?: string
  start_time?: string
  end_time?: string
  api_key_id?: number
  member_id?: number
  member_scope?: 'all' | 'assigned' | 'unassigned'
  granularity?: ApiKeyUsageTrendGranularity
  timezone?: string
  group_id?: number
  tags?: string
  status?: 'active' | 'disabled' | 'quota_exhausted' | 'expired'
  search?: string
  limit?: number
}

export interface OwnerUsageMember {
  id: number
  member_code: string
  name: string
  status: string
  archived: boolean
  key_count: number
  monthly_limit_usd: number
  deleted_at?: string
}

export interface OwnerUsageMembersResponse {
  members: OwnerUsageMember[]
}

export interface OwnerApiKeyUsageTotals {
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
  actual_cost: number
}

export interface OwnerApiKeyAnalyticsSnapshot {
  active_key_count: number
  near_quota_key_count: number
  near_rate_limit_key_count: number
  snapshot_at: string
}

export interface OwnerApiKeyAnalyticsSummary extends OwnerApiKeyUsageTotals {
  used_key_count: number
  current_key_snapshot: OwnerApiKeyAnalyticsSnapshot
}

export interface OwnerApiKeyAnalyticsMeta {
  start_date: string
  end_date: string
  timezone: string
  granularity: ApiKeyUsageTrendGranularity
}

export interface OwnerApiKeyAnalyticsSummaryResponse extends OwnerApiKeyAnalyticsMeta {
  summary: OwnerApiKeyAnalyticsSummary
}

export interface OwnerApiKeyLeaderboardItem extends OwnerApiKeyUsageTotals {
  api_key_id: number
  key_name: string
  tags: string[]
  group_id?: number
  group_name: string
  status: string
  share_percent: number
  previous_actual_cost: number
  change_percent: number
  last_used_at?: string
}

export interface OwnerApiKeyLeaderboardResponse extends OwnerApiKeyAnalyticsMeta {
  items: OwnerApiKeyLeaderboardItem[]
  total: number
  total_actual_cost: number
  displayed_actual_cost: number
}

export interface OwnerMemberLeaderboardItem extends OwnerApiKeyUsageTotals {
  member_id: number | null
  member_code: string
  member_name: string
  status: string
  archived: boolean
  key_count: number
  monthly_limit_usd: number
  current_used_usd: number
  current_reserved_usd: number
  share_percent: number
  previous_actual_cost: number
  change_percent: number
  last_used_at?: string
}

export interface OwnerMemberLeaderboardResponse extends OwnerApiKeyAnalyticsMeta {
  items: OwnerMemberLeaderboardItem[]
  total: number
  member_count: number
  budget_risk_member_count: number
  total_reserved_usd: number
  total_actual_cost: number
  displayed_actual_cost: number
}

export interface OwnerModelAnalyticsItem extends OwnerApiKeyUsageTotals {
  model: string
}

export interface OwnerModelAnalyticsResponse extends OwnerApiKeyAnalyticsMeta {
  models: OwnerModelAnalyticsItem[]
}

export interface OwnerGroupAnalyticsItem extends OwnerApiKeyUsageTotals {
  group_id?: number
  group_name: string
  key_count: number
  share_percent: number
}

export interface OwnerGroupAnalyticsResponse extends OwnerApiKeyAnalyticsMeta {
  groups: OwnerGroupAnalyticsItem[]
}

export interface OwnerTagAnalyticsItem extends OwnerApiKeyUsageTotals {
  tag: string
  key_count: number
}

export interface OwnerTagAnalyticsResponse extends OwnerApiKeyAnalyticsMeta {
  tags: OwnerTagAnalyticsItem[]
}

export interface OwnerTrendAnalyticsPoint extends OwnerApiKeyUsageTotals {
  date: string
}

export interface OwnerTrendAnalyticsResponse extends OwnerApiKeyAnalyticsMeta {
  items: OwnerTrendAnalyticsPoint[]
}

export interface UsageDashboardSnapshotV2Params extends TrendParams {
  include_trend?: boolean
  include_model_stats?: boolean
  include_group_stats?: boolean
}

export interface UsageDashboardSnapshotV2Response {
  generated_at: string
  start_date: string
  end_date: string
  granularity: string
  trend?: TrendDataPoint[]
  models?: ModelStat[]
  groups?: GroupStat[]
}

/**
 * List usage logs with optional filters
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 20)
 * @param apiKeyId - Filter by API key ID
 * @returns Paginated list of usage logs
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  apiKeyId?: number
): Promise<PaginatedResponse<UsageLog>> {
  const params: UsageQueryParams = {
    page,
    page_size: pageSize
  }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/usage', {
    params
  })
  return data
}

/**
 * Get usage logs with advanced query parameters
 * @param params - Query parameters for filtering and pagination
 * @returns Paginated list of usage logs
 */
export async function query(
  params: UsageQueryParams & { sort_by?: string; sort_order?: 'asc' | 'desc' },
  config: { signal?: AbortSignal } = {}
): Promise<PaginatedResponse<UsageLog>> {
  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/usage', {
    ...config,
    params
  })
  return data
}

/**
 * Get usage statistics for a specific period
 * @param period - Time period ('today', 'week', 'month', 'year')
 * @param apiKeyId - Optional API key ID filter
 * @returns Usage statistics
 */
export async function getStats(
  paramsOrPeriod: (UsageQueryParams & { period?: string; timezone?: string }) | string = 'today',
  apiKeyId?: number
): Promise<UsageStatsResponse> {
  const params: Record<string, unknown> = typeof paramsOrPeriod === 'string'
    ? { period: paramsOrPeriod }
    : { ...paramsOrPeriod }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<UsageStatsResponse>('/usage/stats', {
    params
  })
  return data
}

/**
 * Get usage statistics for a date range
 * @param startDate - Start date (YYYY-MM-DD format)
 * @param endDate - End date (YYYY-MM-DD format)
 * @param apiKeyId - Optional API key ID filter
 * @returns Usage statistics
 */
export async function getStatsByDateRange(
  startDate: string,
  endDate: string,
  apiKeyId?: number,
  opts?: { start_time?: string; end_time?: string }
): Promise<UsageStatsResponse> {
  const params: Record<string, unknown> = {
    start_date: startDate,
    end_date: endDate
  }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }
  if (opts?.start_time) {
    params.start_time = opts.start_time
  }
  if (opts?.end_time) {
    params.end_time = opts.end_time
  }

  const { data } = await apiClient.get<UsageStatsResponse>('/usage/stats', {
    params
  })
  return data
}

/**
 * Get usage by date range
 * @param startDate - Start date (YYYY-MM-DD format)
 * @param endDate - End date (YYYY-MM-DD format)
 * @param apiKeyId - Optional API key ID filter
 * @returns Usage logs within date range
 */
export async function getByDateRange(
  startDate: string,
  endDate: string,
  apiKeyId?: number
): Promise<PaginatedResponse<UsageLog>> {
  const params: UsageQueryParams = {
    start_date: startDate,
    end_date: endDate,
    page: 1,
    page_size: 100
  }

  if (apiKeyId !== undefined) {
    params.api_key_id = apiKeyId
  }

  const { data } = await apiClient.get<PaginatedResponse<UsageLog>>('/usage', {
    params
  })
  return data
}

/**
 * Get detailed usage log by ID
 * @param id - Usage log ID
 * @returns Usage log details
 */
export async function getById(id: number): Promise<UsageLog> {
  const { data } = await apiClient.get<UsageLog>(`/usage/${id}`)
  return data
}

// ==================== Dashboard API ====================

/**
 * Get user dashboard statistics
 * @returns Dashboard statistics for current user
 */
export async function getDashboardStats(): Promise<UserDashboardStats> {
  const { data } = await apiClient.get<UserDashboardStats>('/usage/dashboard/stats')
  return data
}

/**
 * Get user usage trend data
 * @param params - Query parameters for filtering
 * @returns Usage trend data for current user
 */
export async function getDashboardTrend(params?: TrendParams): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/usage/dashboard/trend', { params })
  return data
}

/**
 * Get user model usage statistics
 * @param params - Query parameters for filtering
 * @returns Model usage statistics for current user
 */
export async function getDashboardModels(params?: {
  start_date?: string
  end_date?: string
  start_time?: string
  end_time?: string
  api_key_id?: number
  model?: string
  model_source?: 'requested'
  group_id?: number
  request_type?: UsageRequestType
  stream?: boolean
  billing_type?: number | null
  billing_mode?: string | null
  timezone?: string
}): Promise<ModelStatsResponse> {
  const { data } = await apiClient.get<ModelStatsResponse>('/usage/dashboard/models', { params })
  return data
}

/**
 * Get daily usage details for one API key owned by the current user.
 * @param apiKeyId - API key ID
 * @param days - Number of days to include (1-90)
 * @returns Daily usage detail rows
 */
export async function getMyApiKeyDailyUsage(
  apiKeyId: number,
  days: number = 30
): Promise<ApiKeyDailyUsageResponse> {
  const { data } = await apiClient.get<ApiKeyDailyUsageResponse>(
    `/user/api-keys/${apiKeyId}/usage/daily`,
    { params: { days } }
  )
  return data
}

/**
 * Get usage trend details for one API key owned by the current user.
 * @param apiKeyId - API key ID
 * @param params - Date range, granularity, and timezone
 * @returns Usage trend detail rows
 */
export async function getMyApiKeyUsageTrend(
  apiKeyId: number,
  params: ApiKeyUsageTrendParams = {}
): Promise<ApiKeyUsageTrendResponse> {
  const { data } = await apiClient.get<ApiKeyUsageTrendResponse>(
    `/user/api-keys/${apiKeyId}/usage/trend`,
    { params }
  )
  return data
}

export async function getDashboardSnapshotV2(
  params?: UsageDashboardSnapshotV2Params
): Promise<UsageDashboardSnapshotV2Response> {
  const { data } = await apiClient.get<UsageDashboardSnapshotV2Response>(
    '/usage/dashboard/snapshot-v2',
    { params }
  )
  return data
}

/**
 * Get user-safe model distribution for one API key owned by the current user.
 * @param apiKeyId - API key ID
 * @param params - Date range and timezone
 * @returns Model distribution without admin-only cost fields
 */
export async function getMyApiKeyModelStats(
  apiKeyId: number,
  params: ApiKeyModelStatsParams = {}
): Promise<ApiKeyModelStatsResponse> {
  const { data } = await apiClient.get<ApiKeyModelStatsResponse>(
    `/user/api-keys/${apiKeyId}/usage/models`,
    { params }
  )
  return data
}

export async function getOwnerApiKeyAnalyticsSummary(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerApiKeyAnalyticsSummaryResponse> {
  const { data } = await apiClient.get<OwnerApiKeyAnalyticsSummaryResponse>(
    '/usage/analytics/summary',
    { ...config, params }
  )
  return data
}

export async function getOwnerApiKeyAnalyticsLeaderboard(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerApiKeyLeaderboardResponse> {
  const { data } = await apiClient.get<OwnerApiKeyLeaderboardResponse>(
    '/usage/analytics/leaderboard',
    { ...config, params }
  )
  return data
}

export async function listOwnerUsageMembers(): Promise<OwnerUsageMembersResponse> {
  const { data } = await apiClient.get<OwnerUsageMembersResponse>('/usage/members')
  return data
}

export async function getOwnerMemberAnalyticsLeaderboard(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerMemberLeaderboardResponse> {
  const { data } = await apiClient.get<OwnerMemberLeaderboardResponse>(
    '/usage/analytics/members',
    { ...config, params }
  )
  return data
}

export async function getOwnerApiKeyModelAnalytics(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerModelAnalyticsResponse> {
  const { data } = await apiClient.get<OwnerModelAnalyticsResponse>(
    '/usage/analytics/models',
    { ...config, params }
  )
  return data
}

export async function getOwnerApiKeyGroupAnalytics(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerGroupAnalyticsResponse> {
  const { data } = await apiClient.get<OwnerGroupAnalyticsResponse>(
    '/usage/analytics/groups',
    { ...config, params }
  )
  return data
}

export async function getOwnerApiKeyTagAnalytics(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerTagAnalyticsResponse> {
  const { data } = await apiClient.get<OwnerTagAnalyticsResponse>(
    '/usage/analytics/tags',
    { ...config, params }
  )
  return data
}

export async function getOwnerApiKeyUsageTrend(
  params: OwnerApiKeyAnalyticsParams = {},
  config: { signal?: AbortSignal } = {}
): Promise<OwnerTrendAnalyticsResponse> {
  const { data } = await apiClient.get<OwnerTrendAnalyticsResponse>(
    '/usage/analytics/trend',
    { ...config, params }
  )
  return data
}

export interface BatchApiKeyUsageStats {
  api_key_id: number
  today_actual_cost: number
  total_actual_cost: number
}

export interface BatchApiKeysUsageResponse {
  stats: Record<string, BatchApiKeyUsageStats>
}

/**
 * Get batch usage stats for user's own API keys
 * @param apiKeyIds - Array of API key IDs
 * @param options - Optional request options
 * @returns Usage stats map keyed by API key ID
 */
export async function getDashboardApiKeysUsage(
  apiKeyIds: number[],
  options?: {
    signal?: AbortSignal
  }
): Promise<BatchApiKeysUsageResponse> {
  const { data } = await apiClient.post<BatchApiKeysUsageResponse>(
    '/usage/dashboard/api-keys-usage',
    {
      api_key_ids: apiKeyIds
    },
    {
      signal: options?.signal
    }
  )
  return data
}

export async function listMyErrorRequests(
  params: UserErrorListParams
): Promise<PaginatedResponse<UserErrorRequest>> {
  const { data } = await apiClient.get<PaginatedResponse<UserErrorRequest>>('/usage/errors', {
    params
  })
  return data
}

export async function getMyErrorDetail(id: number): Promise<UserErrorRequestDetail> {
  const { data } = await apiClient.get<UserErrorRequestDetail>(`/usage/errors/${id}`)
  return data
}

export const usageAPI = {
  list,
  query,
  getStats,
  getStatsByDateRange,
  getByDateRange,
  getById,
  // Dashboard
  getDashboardStats,
  getDashboardTrend,
  getDashboardModels,
  getMyApiKeyDailyUsage,
  getMyApiKeyUsageTrend,
  getMyApiKeyModelStats,
  getDashboardSnapshotV2,
  getOwnerApiKeyAnalyticsSummary,
  getOwnerApiKeyAnalyticsLeaderboard,
  listOwnerUsageMembers,
  getOwnerMemberAnalyticsLeaderboard,
  getOwnerApiKeyModelAnalytics,
  getOwnerApiKeyGroupAnalytics,
  getOwnerApiKeyTagAnalytics,
  getOwnerApiKeyUsageTrend,
  getDashboardApiKeysUsage,
  // Error requests
  listMyErrorRequests,
  getMyErrorDetail
}

export default usageAPI
