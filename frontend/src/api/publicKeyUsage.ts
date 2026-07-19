import axios, { type AxiosInstance } from 'axios'

import { getLocale } from '@/i18n'
import type { ApiResponse, PaginatedResponse } from '@/types'
import { getAPIBaseURL } from './url'

export type PublicKeyUsageRecordKind = 'success' | 'error'

export interface PublicKeyUsageSession {
  valid: boolean
  expires_at?: string
  absolute_expires_at?: string
}

export interface PublicKeyUsageLimit {
  limit: number
  used: number
  remaining: number
  reset_at?: string
}

export interface PublicKeyUsageIdentity {
  name: string
  key_prefix: string
  status: string
  active: boolean
  created_at: string
  last_used_at?: string
  expires_at?: string
  ip_access_mode: 'unrestricted' | 'whitelist' | 'blacklist'
  whitelist_size: number
  blacklist_size: number
  member?: {
    code: string
    name: string
    status: string
  }
}

export interface PublicKeyUsageAccessGroup {
  name: string
  platform: string
  status: string
  sort_order: number
  rpm_limit: number
  models: string[]
  model_count: number
  description?: string
}

export interface PublicKeyUsageMemberBudget {
  period_start: string
  period_end: string
  timezone: string
  monthly: PublicKeyUsageLimit
  settled_usd: number
  reserved_usd: number
  request_count: number
  input_tokens: number
  output_tokens: number
  limit_5h: PublicKeyUsageLimit
  limit_1d: PublicKeyUsageLimit
  limit_7d: PublicKeyUsageLimit
}

export interface PublicKeyUsageStats {
  total_requests: number
  total_input_tokens: number
  total_output_tokens: number
  total_cache_creation_tokens: number
  total_cache_read_tokens: number
  total_tokens: number
  total_actual_cost: number
  average_duration_ms: number
}

export interface PublicKeyUsageTrendPoint {
  date: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
  actual_cost: number
}

export interface PublicKeyUsageModelStat {
  model: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  total_tokens: number
  actual_cost: number
}

export interface PublicKeyUsageSummary {
  identity: PublicKeyUsageIdentity
  key_budget: {
    quota: PublicKeyUsageLimit
    limit_5h: PublicKeyUsageLimit
    limit_1d: PublicKeyUsageLimit
    limit_7d: PublicKeyUsageLimit
  }
  member_budget?: PublicKeyUsageMemberBudget
  access_groups: PublicKeyUsageAccessGroup[]
  stats: PublicKeyUsageStats
  trend: PublicKeyUsageTrendPoint[]
  models: PublicKeyUsageModelStat[]
  start_date: string
  end_date: string
  timezone: string
  error_records_available: boolean
}

export interface PublicKeyUsageRecord {
  id: number
  kind: PublicKeyUsageRecordKind
  created_at: string
  request_id?: string
  model: string
  inbound_endpoint?: string
  group_name?: string
  status_code: number
  request_type?: string
  stream: boolean
  input_tokens?: number
  output_tokens?: number
  cache_creation_tokens?: number
  cache_read_tokens?: number
  total_tokens?: number
  actual_cost?: number
  duration_ms?: number
  first_token_ms?: number
  ip_address?: string
  user_agent?: string
  category?: string
  platform?: string
  message?: string
  upstream_status_code?: number
}

export interface PublicKeyUsageQuery {
  start_date: string
  end_date: string
  timezone: string
  model?: string
}

export interface PublicKeyUsageRecordQuery extends PublicKeyUsageQuery {
  kind: PublicKeyUsageRecordKind
  page?: number
  page_size?: number
  status_code?: number | null
  category?: string
}

// This client is intentionally isolated from apiClient. The shared client
// injects the signed-in user's JWT and would overwrite the API Key used for the
// one-time session exchange.
const publicKeyUsageClient: AxiosInstance = axios.create({
  baseURL: getAPIBaseURL(),
  withCredentials: true,
  timeout: 30_000,
  headers: { 'Content-Type': 'application/json' },
})

publicKeyUsageClient.interceptors.request.use((config) => {
  if (config.headers) config.headers['Accept-Language'] = getLocale()
  return config
})

function unwrap<T>(payload: ApiResponse<T> | T): T {
  if (payload && typeof payload === 'object' && 'code' in payload) {
    const wrapped = payload as ApiResponse<T>
    if (wrapped.code !== 0) throw new Error(wrapped.message || 'Request failed')
    return wrapped.data as T
  }
  return payload as T
}

export const publicKeyUsageAPI = {
  async createSession(apiKey: string): Promise<PublicKeyUsageSession> {
    const response = await publicKeyUsageClient.post<ApiResponse<PublicKeyUsageSession>>(
      '/key/usage-session',
      undefined,
      { headers: { Authorization: `Bearer ${apiKey}` } },
    )
    return unwrap(response.data)
  },

  async getSession(): Promise<PublicKeyUsageSession> {
    const response = await publicKeyUsageClient.get<ApiResponse<PublicKeyUsageSession>>('/key/usage-session')
    return unwrap(response.data)
  },

  async deleteSession(): Promise<void> {
    await publicKeyUsageClient.delete('/key/usage-session')
  },

  async getSummary(query: PublicKeyUsageQuery, signal?: AbortSignal): Promise<PublicKeyUsageSummary> {
    const response = await publicKeyUsageClient.get<ApiResponse<PublicKeyUsageSummary>>('/key/usage/summary', { params: query, signal })
    return unwrap(response.data)
  },

  async listRecords(query: PublicKeyUsageRecordQuery, signal?: AbortSignal): Promise<PaginatedResponse<PublicKeyUsageRecord>> {
    const response = await publicKeyUsageClient.get<ApiResponse<PaginatedResponse<PublicKeyUsageRecord>>>('/key/usage/records', { params: query, signal })
    return unwrap(response.data)
  },

  async getRecordDetail(kind: PublicKeyUsageRecordKind, id: number, signal?: AbortSignal): Promise<PublicKeyUsageRecord> {
    const response = await publicKeyUsageClient.get<ApiResponse<PublicKeyUsageRecord>>(`/key/usage/records/${id}`, { params: { kind }, signal })
    return unwrap(response.data)
  },

  async exportRecords(query: PublicKeyUsageRecordQuery, signal?: AbortSignal): Promise<Blob> {
    const response = await publicKeyUsageClient.get<Blob>('/key/usage/export', {
      params: query,
      responseType: 'blob',
      timeout: 60_000,
      signal,
    })
    return response.data
  },
}
