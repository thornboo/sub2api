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

export type ModelSelfCheckRoundStatus = 'checking' | 'operational' | 'degraded' | 'failed' | 'unknown'
export type ModelSelfCheckStepOutcome = 'pending' | 'skipped' | 'succeeded' | 'failed' | 'not_attempted' | 'incomplete'

export interface ModelSelfCheckProbeStep {
  account_id: number
  account_name: string
  priority: number
  platform: string
  order: number
  outcome: ModelSelfCheckStepOutcome
  reason_code: string
  started_at: string | null
  finished_at: string | null
  latency_ms: number | null
  http_status: number | null
  error_code: string
}

export interface ModelSelfCheckProbeRound {
  id: number
  group_id: number
  model: string
  status: ModelSelfCheckRoundStatus
  reason_code: string
  winner_account_id: number | null
  started_at: string
  finished_at: string | null
  duration_ms: number | null
  steps: ModelSelfCheckProbeStep[]
}

export interface ModelSelfCheckChainCandidate {
  account_id: number
  account_name: string
  priority: number
  platform: string
  order: number
  eligible: boolean
  reason_code: string
  last_checked_at: string | null
  last_status: ModelStatus
}

export interface ModelSelfCheckChainView {
  group_id: number
  group_name: string
  model: string
  updated_at: string
  attempt_timeout_seconds: number
  round_timeout_seconds: number
  candidates: ModelSelfCheckChainCandidate[]
  latest_round: ModelSelfCheckProbeRound | null
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

function normalizeRoundStatus(value: unknown): ModelSelfCheckRoundStatus {
  switch (value) {
    case 'checking':
    case 'operational':
    case 'degraded':
    case 'failed':
    case 'unknown':
      return value
    default:
      return 'unknown'
  }
}

function normalizeStepOutcome(value: unknown): ModelSelfCheckStepOutcome {
  switch (value) {
    case 'incomplete':
    case 'pending':
    case 'skipped':
    case 'succeeded':
    case 'failed':
    case 'not_attempted':
      return value
    default:
      return 'skipped'
  }
}

function nullableString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() !== '' ? value : null
}

function normalizeProbeStep(item: unknown): ModelSelfCheckProbeStep | null {
  if (!isRecord(item) || !Number.isSafeInteger(item.account_id)) return null
  return {
    account_id: Number(item.account_id),
    account_name: typeof item.account_name === 'string' ? item.account_name : '',
    priority: Number.isSafeInteger(item.priority) ? Number(item.priority) : 0,
    platform: typeof item.platform === 'string' ? item.platform : '',
    order: Number.isSafeInteger(item.order) ? Number(item.order) : 0,
    outcome: normalizeStepOutcome(item.outcome),
    reason_code: typeof item.reason_code === 'string' ? item.reason_code : '',
    started_at: nullableString(item.started_at),
    finished_at: nullableString(item.finished_at),
    latency_ms: nullableFiniteNumber(item.latency_ms),
    http_status: Number.isSafeInteger(item.http_status) ? Number(item.http_status) : null,
    error_code: typeof item.error_code === 'string' ? item.error_code : '',
  }
}

function normalizeProbeRound(item: unknown): ModelSelfCheckProbeRound | null {
  if (!isRecord(item) || !Number.isSafeInteger(item.id) || !Number.isSafeInteger(item.group_id)) return null
  const model = typeof item.model === 'string' ? item.model.trim() : ''
  const startedAt = nullableString(item.started_at)
  if (model === '' || startedAt === null) return null
  return {
    id: Number(item.id),
    group_id: Number(item.group_id),
    model,
    status: normalizeRoundStatus(item.status),
    reason_code: typeof item.reason_code === 'string' ? item.reason_code : '',
    winner_account_id: Number.isSafeInteger(item.winner_account_id) ? Number(item.winner_account_id) : null,
    started_at: startedAt,
    finished_at: nullableString(item.finished_at),
    duration_ms: nullableFiniteNumber(item.duration_ms),
    steps: Array.isArray(item.steps)
      ? item.steps.map(normalizeProbeStep).filter((step): step is ModelSelfCheckProbeStep => step !== null)
      : [],
  }
}

function normalizeChainCandidate(item: unknown): ModelSelfCheckChainCandidate | null {
  if (!isRecord(item) || !Number.isSafeInteger(item.account_id)) return null
  return {
    account_id: Number(item.account_id),
    account_name: typeof item.account_name === 'string' ? item.account_name : '',
    priority: Number.isSafeInteger(item.priority) ? Number(item.priority) : 0,
    platform: typeof item.platform === 'string' ? item.platform : '',
    order: Number.isSafeInteger(item.order) ? Number(item.order) : 0,
    eligible: item.eligible === true,
    reason_code: typeof item.reason_code === 'string' ? item.reason_code : '',
    last_checked_at: nullableString(item.last_checked_at),
    last_status: normalizeStatus(item.last_status),
  }
}

function normalizeSelfCheckChain(data: unknown): ModelSelfCheckChainView | null {
  if (!isRecord(data) || !Number.isSafeInteger(data.group_id)) return null
  const model = typeof data.model === 'string' ? data.model.trim() : ''
  const updatedAt = nullableString(data.updated_at)
  if (model === '' || updatedAt === null) return null
  return {
    group_id: Number(data.group_id),
    group_name: typeof data.group_name === 'string' ? data.group_name : '',
    model,
    updated_at: updatedAt,
    attempt_timeout_seconds: Number.isSafeInteger(data.attempt_timeout_seconds) ? Number(data.attempt_timeout_seconds) : 0,
    round_timeout_seconds: Number.isSafeInteger(data.round_timeout_seconds) ? Number(data.round_timeout_seconds) : 0,
    candidates: Array.isArray(data.candidates)
      ? data.candidates.map(normalizeChainCandidate).filter((item): item is ModelSelfCheckChainCandidate => item !== null)
      : [],
    latest_round: normalizeProbeRound(data.latest_round),
  }
}

export async function fetchModelSelfCheckChain(
  groupId: number,
  model: string,
  options?: { signal?: AbortSignal }
): Promise<ModelSelfCheckChainView> {
  const { data } = await apiClient.get<unknown>('/admin/model-self-check/chain', {
    params: { group_id: groupId, model },
    signal: options?.signal,
  })
  const normalized = normalizeSelfCheckChain(data)
  if (normalized === null) {
    throw new Error('Invalid model self-check chain response')
  }
  return normalized
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
  fetchModelSelfCheckChain,
}

export default modelStatusAPI
