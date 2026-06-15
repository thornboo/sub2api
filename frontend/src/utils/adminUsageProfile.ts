import type { AdminUsageQueryParams } from '@/api/admin/usage'
import type { UsageRequestType } from '@/types'

type QueryValue = string | Array<string | null> | null | undefined
export type AdminUsageRouteQuery = Record<string, QueryValue>

export const ADMIN_USAGE_REQUEST_TYPES = ['ws_v2', 'stream', 'sync'] as const
export const ADMIN_USAGE_BILLING_MODES = ['token', 'per_request', 'image'] as const
export const ADMIN_USAGE_BILLING_TYPES = [0, 1] as const

export interface SanitizedAdminUsageQuery {
  filters: AdminUsageQueryParams
  startDate?: string
  endDate?: string
  routeQuery: Record<string, string>
}

const getSingleQueryValue = (value: QueryValue): string | undefined => {
  if (Array.isArray(value)) {
    return value.find((item): item is string => typeof item === 'string' && item.trim().length > 0)
  }
  return typeof value === 'string' && value.trim().length > 0 ? value : undefined
}

const parsePositiveIntQuery = (value: QueryValue): number | undefined => {
  const raw = getSingleQueryValue(value)
  if (!raw || !/^\d+$/.test(raw)) return undefined
  const parsed = Number(raw)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}

const parseDateQuery = (value: QueryValue): string | undefined => {
  const raw = getSingleQueryValue(value)?.trim()
  if (!raw || !/^\d{4}-\d{2}-\d{2}$/.test(raw)) return undefined
  const date = new Date(`${raw}T00:00:00`)
  if (Number.isNaN(date.getTime())) return undefined
  if (date.getFullYear() !== Number(raw.slice(0, 4))) return undefined
  if (date.getMonth() + 1 !== Number(raw.slice(5, 7))) return undefined
  if (date.getDate() !== Number(raw.slice(8, 10))) return undefined
  return raw
}

const parseStringQuery = (value: QueryValue): string | undefined => {
  const raw = getSingleQueryValue(value)?.trim()
  return raw ? raw : undefined
}

const parseRequestTypeQuery = (value: QueryValue): UsageRequestType | undefined => {
  const raw = getSingleQueryValue(value)
  return raw && (ADMIN_USAGE_REQUEST_TYPES as readonly string[]).includes(raw) ? raw as UsageRequestType : undefined
}

const parseBillingTypeQuery = (value: QueryValue): number | null | undefined => {
  const raw = getSingleQueryValue(value)
  if (raw == null) return undefined
  const parsed = Number(raw)
  return ADMIN_USAGE_BILLING_TYPES.includes(parsed as 0 | 1) ? parsed : undefined
}

const parseBillingModeQuery = (value: QueryValue): string | undefined => {
  const raw = getSingleQueryValue(value)
  return raw && (ADMIN_USAGE_BILLING_MODES as readonly string[]).includes(raw) ? raw : undefined
}

export const sanitizeAdminUsageRouteQuery = (
  query: AdminUsageRouteQuery,
  defaults: { startDate: string; endDate: string }
): SanitizedAdminUsageQuery => {
  const routeQuery: Record<string, string> = {}
  const queryStartDate = parseDateQuery(query.start_date)
  const queryEndDate = parseDateQuery(query.end_date)
  const startDate = queryStartDate ?? defaults.startDate
  const endDate = queryEndDate ?? defaults.endDate
  const filters: AdminUsageQueryParams = {
    start_date: startDate,
    end_date: endDate,
  }
  if (queryStartDate !== undefined) routeQuery.start_date = queryStartDate
  if (queryEndDate !== undefined) routeQuery.end_date = queryEndDate

  const userId = parsePositiveIntQuery(query.user_id)
  if (userId !== undefined) {
    filters.user_id = userId
    routeQuery.user_id = String(userId)
  }

  const apiKeyId = parsePositiveIntQuery(query.api_key_id)
  if (apiKeyId !== undefined) {
    filters.api_key_id = apiKeyId
    routeQuery.api_key_id = String(apiKeyId)
  }

  const accountId = parsePositiveIntQuery(query.account_id)
  if (accountId !== undefined) {
    filters.account_id = accountId
    routeQuery.account_id = String(accountId)
  }

  const groupId = parsePositiveIntQuery(query.group_id)
  if (groupId !== undefined) {
    filters.group_id = groupId
    routeQuery.group_id = String(groupId)
  }

  const model = parseStringQuery(query.model)
  if (model !== undefined) {
    filters.model = model
    routeQuery.model = model
  }

  const requestType = parseRequestTypeQuery(query.request_type)
  if (requestType !== undefined) {
    filters.request_type = requestType
    routeQuery.request_type = requestType
  }

  const billingType = parseBillingTypeQuery(query.billing_type)
  if (billingType !== undefined) {
    filters.billing_type = billingType
    routeQuery.billing_type = String(billingType)
  }

  const billingMode = parseBillingModeQuery(query.billing_mode)
  if (billingMode !== undefined) {
    filters.billing_mode = billingMode
    routeQuery.billing_mode = billingMode
  }

  return { filters, startDate, endDate, routeQuery }
}

export const buildUserProfileFilters = (
  current: AdminUsageQueryParams,
  userId: number,
  explicit: Partial<AdminUsageQueryParams> = {}
): AdminUsageQueryParams => ({
  start_date: current.start_date,
  end_date: current.end_date,
  request_type: current.request_type,
  billing_type: current.billing_type,
  billing_mode: current.billing_mode,
  group_id: current.group_id,
  user_id: userId,
  ...explicit,
})

export const buildApiKeyProfileFilters = (
  current: AdminUsageQueryParams,
  apiKeyId: number,
  explicit: Partial<AdminUsageQueryParams> = {}
): AdminUsageQueryParams => ({
  start_date: current.start_date,
  end_date: current.end_date,
  request_type: current.request_type,
  billing_type: current.billing_type,
  billing_mode: current.billing_mode,
  group_id: current.group_id,
  user_id: current.user_id,
  api_key_id: apiKeyId,
  ...explicit,
})

export const clearUserProfileFilters = (current: AdminUsageQueryParams): AdminUsageQueryParams => ({
  ...current,
  user_id: undefined,
  api_key_id: undefined,
})

export const clearApiKeyProfileFilters = (current: AdminUsageQueryParams): AdminUsageQueryParams => ({
  ...current,
  api_key_id: undefined,
})
