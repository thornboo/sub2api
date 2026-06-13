/**
 * API Keys management endpoints
 * Handles CRUD operations for user API keys
 */

import { apiClient } from './client'
import type {
  ApiKey,
  BatchCreateApiKeysRequest,
  BatchCreateApiKeysResponse,
  BatchDeleteApiKeysRequest,
  BatchDeleteApiKeysResponse,
  BatchUpdateApiKeysRequest,
  BatchUpdateApiKeysResponse,
  CreateApiKeyRequest,
  PublicApiKeyStatus,
  UpdateApiKeyRequest,
  PaginatedResponse
} from '@/types'

function createIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `key-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

/**
 * List all API keys for current user
 * @param page - Page number (default: 1)
 * @param pageSize - Items per page (default: 10)
 * @param filters - Optional filter parameters
 * @param options - Optional request options
 * @returns Paginated list of API keys
 */
export async function list(
  page: number = 1,
  pageSize: number = 10,
  filters?: {
    search?: string
    status?: string
    group_id?: number | string
    tags?: string
    sort_by?: string
    sort_order?: 'asc' | 'desc'
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<ApiKey>> {
  const { data } = await apiClient.get<PaginatedResponse<ApiKey>>('/keys', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal
  })
  return data
}

/**
 * Get API key by ID
 * @param id - API key ID
 * @returns API key details
 */
export async function getById(id: number): Promise<ApiKey> {
  const { data } = await apiClient.get<ApiKey>(`/keys/${id}`)
  return data
}

/**
 * Create new API key
 * @param name - Key name
 * @param groupId - Optional group ID
 * @param customKey - Optional custom key value
 * @param ipWhitelist - Optional IP whitelist
 * @param ipBlacklist - Optional IP blacklist
 * @param quota - Optional quota limit in USD (0 = unlimited)
 * @param expiresInDays - Optional days until expiry (undefined = never expires)
 * @param rateLimitData - Optional rate limit fields
 * @returns Created API key
 */
export async function create(
  name: string,
  groupId?: number | null,
  customKey?: string,
  ipWhitelist?: string[],
  ipBlacklist?: string[],
  quota?: number,
  expiresInDays?: number,
  rateLimitData?: { rate_limit_5h?: number; rate_limit_1d?: number; rate_limit_7d?: number },
  tags?: string[]
): Promise<ApiKey> {
  const payload: CreateApiKeyRequest = { name }
  if (tags && tags.length > 0) {
    payload.tags = tags
  }
  if (groupId !== undefined) {
    payload.group_id = groupId
  }
  if (customKey) {
    payload.custom_key = customKey
  }
  if (ipWhitelist && ipWhitelist.length > 0) {
    payload.ip_whitelist = ipWhitelist
  }
  if (ipBlacklist && ipBlacklist.length > 0) {
    payload.ip_blacklist = ipBlacklist
  }
  if (quota !== undefined && quota > 0) {
    payload.quota = quota
  }
  if (expiresInDays !== undefined && expiresInDays > 0) {
    payload.expires_in_days = expiresInDays
  }
  if (rateLimitData?.rate_limit_5h && rateLimitData.rate_limit_5h > 0) {
    payload.rate_limit_5h = rateLimitData.rate_limit_5h
  }
  if (rateLimitData?.rate_limit_1d && rateLimitData.rate_limit_1d > 0) {
    payload.rate_limit_1d = rateLimitData.rate_limit_1d
  }
  if (rateLimitData?.rate_limit_7d && rateLimitData.rate_limit_7d > 0) {
    payload.rate_limit_7d = rateLimitData.rate_limit_7d
  }

  const { data } = await apiClient.post<ApiKey>('/keys', payload)
  return data
}

export async function batchCreate(
  payload: BatchCreateApiKeysRequest
): Promise<BatchCreateApiKeysResponse> {
  const { data } = await apiClient.post<BatchCreateApiKeysResponse>('/keys/batch', payload, {
    headers: {
      'Idempotency-Key': createIdempotencyKey()
    }
  })
  return data
}

export async function batchUpdate(
  payload: BatchUpdateApiKeysRequest
): Promise<BatchUpdateApiKeysResponse> {
  const { data } = await apiClient.post<BatchUpdateApiKeysResponse>('/keys/batch-update', payload, {
    headers: {
      'Idempotency-Key': createIdempotencyKey()
    }
  })
  return data
}

export async function batchDelete(
  payload: BatchDeleteApiKeysRequest
): Promise<BatchDeleteApiKeysResponse> {
  const { data } = await apiClient.post<BatchDeleteApiKeysResponse>('/keys/batch-delete', payload, {
    headers: {
      'Idempotency-Key': createIdempotencyKey()
    }
  })
  return data
}

export async function getPublicStatus(key: string): Promise<PublicApiKeyStatus> {
  const { data } = await apiClient.post<PublicApiKeyStatus>('/key/status', { key })
  return data
}

/**
 * Update API key
 * @param id - API key ID
 * @param updates - Fields to update
 * @returns Updated API key
 */
export async function update(id: number, updates: UpdateApiKeyRequest): Promise<ApiKey> {
  const { data } = await apiClient.put<ApiKey>(`/keys/${id}`, updates)
  return data
}

/**
 * Delete API key
 * @param id - API key ID
 * @returns Success confirmation
 */
export async function deleteKey(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/keys/${id}`)
  return data
}

/**
 * Toggle API key status (active/inactive)
 * @param id - API key ID
 * @param status - New status
 * @returns Updated API key
 */
export async function toggleStatus(id: number, status: 'active' | 'inactive'): Promise<ApiKey> {
  return update(id, { status })
}

export const keysAPI = {
  list,
  getById,
  create,
  batchCreate,
  batchUpdate,
  batchDelete,
  getPublicStatus,
  update,
  delete: deleteKey,
  toggleStatus
}

export default keysAPI
