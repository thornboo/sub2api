/**
 * User-facing model service status API.
 *
 * These endpoints intentionally expose only public model health fields. They
 * do not return monitor IDs, providers, accounts, upstream endpoints, or raw
 * upstream errors.
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

function normalizeModelStatus(item: UserModelStatus): UserModelStatus {
  return {
    ...item,
    timeline: Array.isArray(item.timeline)
      ? item.timeline.map(point => ({
        ...point,
        ping_latency_ms: point.ping_latency_ms ?? null,
      }))
      : [],
  }
}

export interface ModelStatusListResponse {
  items: UserModelStatus[]
  updated_at: string
}

export async function list(options?: { signal?: AbortSignal }): Promise<ModelStatusListResponse> {
  const { data } = await apiClient.get<ModelStatusListResponse>('/model-status', {
    signal: options?.signal,
  })
  return {
    items: Array.isArray(data.items) ? data.items.map(normalizeModelStatus) : [],
    updated_at: data.updated_at,
  }
}

export async function detail(model: string, groupId?: number): Promise<UserModelStatus> {
  const { data } = await apiClient.get<UserModelStatus>('/model-status/detail', {
    params: groupId ? { model, group_id: groupId } : { model },
  })
  return normalizeModelStatus(data)
}

export const modelStatusAPI = {
  list,
  detail,
}

export default modelStatusAPI
