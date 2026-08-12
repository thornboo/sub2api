/**
 * Model service status API.
 *
 * Public endpoints intentionally expose only model health fields. Admin-only
 * helpers below are guarded by backend admin routes and keep the same
 * account/upstream redaction boundary.
 */

import { apiClient } from './client'
import type { MonitorStatus } from './admin/channelMonitor'

export type ModelStatus = MonitorStatus | 'unknown'
export type ModelStatusMessageCode = 'normal' | 'partial' | 'unavailable' | 'no_data'

export interface ModelStatusTimelinePoint {
  status: ModelStatus
  latency_ms: number | null
  ping_latency_ms: number | null
  checked_at: string
}

export interface UserModelStatus {
  group_id: number
  group_name: string
  model: string
  display_name: string
  status: ModelStatus
  message_code: ModelStatusMessageCode
  latest_latency_ms: number | null
  avg_latency_24h_ms: number | null
  avg_latency_7d_ms: number | null
  availability_24h: number | null
  availability_7d: number | null
  availability_30d: number | null
  degraded_ratio_24h: number | null
  last_checked_at: string | null
  timeline?: ModelStatusTimelinePoint[]
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function normalizeStatus(value: unknown): ModelStatus {
  switch (value) {
    case 'operational':
    case 'degraded':
    case 'failed':
    case 'error':
    case 'unknown':
      return value
    default:
      return 'unknown'
  }
}

function normalizeMessageCode(value: unknown): ModelStatusMessageCode {
  switch (value) {
    case 'normal':
    case 'partial':
    case 'unavailable':
    case 'no_data':
      return value
    default:
      return 'no_data'
  }
}

function nullableFiniteNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function normalizeTimelinePoint(value: unknown): ModelStatusTimelinePoint | null {
  if (!isRecord(value) || typeof value.checked_at !== 'string' || value.checked_at.trim() === '') {
    return null
  }
  return {
    status: normalizeStatus(value.status),
    latency_ms: nullableFiniteNumber(value.latency_ms),
    ping_latency_ms: nullableFiniteNumber(value.ping_latency_ms),
    checked_at: value.checked_at,
  }
}

function normalizeModelStatus(item: unknown): UserModelStatus | null {
  if (!isRecord(item)) return null
  const groupId = item.group_id
  const model = typeof item.model === 'string' ? item.model.trim() : ''
  if (!Number.isSafeInteger(groupId) || Number(groupId) <= 0 || model === '') return null

  const timeline = Array.isArray(item.timeline)
    ? item.timeline.map(normalizeTimelinePoint).filter((point): point is ModelStatusTimelinePoint => point !== null)
    : []
  return {
    group_id: Number(groupId),
    group_name: typeof item.group_name === 'string' ? item.group_name : '',
    model,
    display_name: typeof item.display_name === 'string' ? item.display_name : model,
    status: normalizeStatus(item.status),
    message_code: normalizeMessageCode(item.message_code),
    latest_latency_ms: nullableFiniteNumber(item.latest_latency_ms),
    avg_latency_24h_ms: nullableFiniteNumber(item.avg_latency_24h_ms),
    avg_latency_7d_ms: nullableFiniteNumber(item.avg_latency_7d_ms),
    availability_24h: nullableFiniteNumber(item.availability_24h),
    availability_7d: nullableFiniteNumber(item.availability_7d),
    availability_30d: nullableFiniteNumber(item.availability_30d),
    degraded_ratio_24h: nullableFiniteNumber(item.degraded_ratio_24h),
    last_checked_at: typeof item.last_checked_at === 'string' ? item.last_checked_at : null,
    timeline,
  }
}

export interface ModelStatusListResponse {
  items: UserModelStatus[]
  updated_at: string | null
}

export type SelfCheckTokenUsageWindow = 'today' | '7d' | '30d'

export interface SelfCheckTokenUsageItem {
  model: string
  input_tokens: number
  output_tokens: number
  total_tokens: number
}

export interface SelfCheckTokenUsageResponse {
  window: SelfCheckTokenUsageWindow
  items: SelfCheckTokenUsageItem[]
}

export async function list(options?: { signal?: AbortSignal }): Promise<ModelStatusListResponse> {
  const { data } = await apiClient.get<unknown>('/model-status', {
    signal: options?.signal,
  })
  const payload = isRecord(data) ? data : {}
  return {
    items: Array.isArray(payload.items)
      ? payload.items.map(normalizeModelStatus).filter((item): item is UserModelStatus => item !== null)
      : [],
    updated_at: typeof payload.updated_at === 'string' ? payload.updated_at : null,
  }
}

export async function detail(
  model: string,
  groupId?: number,
  options?: { signal?: AbortSignal }
): Promise<UserModelStatus> {
  const { data } = await apiClient.get<unknown>('/model-status/detail', {
    params: groupId ? { model, group_id: groupId } : { model },
    signal: options?.signal,
  })
  const normalized = normalizeModelStatus(data)
  if (normalized === null) {
    throw new Error('Invalid model status detail response')
  }
  return normalized
}

export async function fetchSelfCheckTokenUsage(
  window: SelfCheckTokenUsageWindow,
  options?: { signal?: AbortSignal }
): Promise<SelfCheckTokenUsageResponse> {
  const timezone = getBrowserTimeZone()
  const { data } = await apiClient.get<SelfCheckTokenUsageResponse>('/admin/model-self-check/token-usage', {
    params: timezone ? { window, timezone } : { window },
    signal: options?.signal,
  })
  return {
    window: data.window || 'today',
    items: Array.isArray(data.items) ? data.items : [],
  }
}

function getBrowserTimeZone(): string | undefined {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || undefined
  } catch {
    return undefined
  }
}

export const modelStatusAPI = {
  list,
  detail,
  fetchSelfCheckTokenUsage,
}

export default modelStatusAPI
