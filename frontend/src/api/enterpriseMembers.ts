import { apiClient } from './client'
import type { ApiKey } from '@/types'

export type EnterpriseMemberStatus = 'active' | 'disabled'

export interface EnterpriseMember {
  id: number
  enterprise_user_id: number
  member_code: string
  name: string
  status: EnterpriseMemberStatus
  monthly_limit_usd: number
  rate_limit_5h: number
  rate_limit_1d: number
  rate_limit_7d: number
  usage_5h: number
  usage_1d: number
  usage_7d: number
  window_5h_start?: string | null
  window_1d_start?: string | null
  window_7d_start?: string | null
  version: number
  group_ids: number[]
  key_count: number
  created_at: string
  updated_at: string
  deleted_at?: string | null
}

export interface EnterpriseMemberDraft {
  member_code: string
  name: string
  monthly_limit_usd: number
  rate_limit_5h: number
  rate_limit_1d: number
  rate_limit_7d: number
  group_ids: number[]
}

export interface CreateEnterpriseMemberInput extends EnterpriseMemberDraft {
  monthly_used_usd: number
  usage_5h: number
  usage_1d: number
  usage_7d: number
}

export interface EnterpriseMemberBudgetSummary {
  member_id: number
  period_start: string
  period_end: string
  timezone: string
  limit_usd: number
  used_usd: number
  reserved_usd: number
  remaining_usd: number
  request_count: number
  input_tokens: number
  output_tokens: number
  rate_limit_5h: number
  rate_limit_1d: number
  rate_limit_7d: number
  usage_5h: number
  usage_1d: number
  usage_7d: number
  reset_5h_at?: string | null
  reset_1d_at?: string | null
  reset_7d_at?: string | null
}

export interface EnterpriseMemberOwnerUsageItem {
  member_id: number
  member_code: string
  member_name: string
  status: EnterpriseMemberStatus
  limit_usd: number
  used_usd: number
  reserved_usd: number
  remaining_usd: number
  request_count: number
  input_tokens: number
  output_tokens: number
}

export interface EnterpriseMemberOwnerUsageSummary {
  period_start: string
  period_end: string
  timezone: string
  used_usd: number
  reserved_usd: number
  request_count: number
  input_tokens: number
  output_tokens: number
  members: EnterpriseMemberOwnerUsageItem[]
}

export interface EnterpriseMemberBudgetEntry {
  id: number
  kind: 'usage' | 'manual_adjustment' | 'migration_opening' | 'reconciliation'
  request_id?: string
  amount_usd: number
  usage_log_id?: number
  actor_user_id?: number
  note: string
  created_at: string
}

export interface EnterpriseMemberAuditEvent {
  id: number
  enterprise_user_id: number
  member_id?: number
  actor_user_id?: number
  action: string
  entity_type: 'member' | 'group' | 'api_key' | 'budget_entry' | 'import_job' | 'enterprise_account' | string
  entity_id?: number
  before_data: Record<string, unknown>
  after_data: Record<string, unknown>
  metadata: Record<string, unknown>
  created_at: string
}

export interface EnterpriseMemberUsagePoint {
  date: string
  request_count: number
  input_tokens: number
  output_tokens: number
  actual_cost: number
}

export interface EnterpriseMemberUsageBreakdown {
  key: string
  name: string
  request_count: number
  input_tokens: number
  output_tokens: number
  actual_cost: number
}

export interface EnterpriseMemberUsageAnalytics {
  start: string
  end: string
  trend: EnterpriseMemberUsagePoint[]
  models: EnterpriseMemberUsageBreakdown[]
  groups: EnterpriseMemberUsageBreakdown[]
}

export interface EnterpriseMemberUsageRecord {
  id: number
  request_id: string
  api_key_id: number
  api_key_name: string
  model: string
  group_id?: number | null
  group_name: string
  request_type: 'unknown' | 'sync' | 'stream' | 'ws_v2' | 'cyber' | string
  input_tokens: number
  output_tokens: number
  cache_creation_tokens: number
  cache_read_tokens: number
  actual_cost: number
  duration_ms?: number | null
  first_token_ms?: number | null
  billing_mode: string
  inbound_endpoint: string
  image_count: number
  video_count: number
  created_at: string
}

export interface EnterpriseMemberImportRow {
  row_number: number
  member_code: string
  member_name: string
  monthly_limit_usd: number
  rate_limit_5h: number
  rate_limit_1d: number
  rate_limit_7d: number
  opening_used_usd: number
  key_name?: string
  key_present: boolean
  key_quota_usd: number
  group_ids: number[]
  valid: boolean
  errors: string[]
  warnings: string[]
}

export interface EnterpriseMemberImportPreview {
  job_id: number
  token: string
  file_hash: string
  format: 'csv' | 'xlsx'
  expires_at: string
  rows: EnterpriseMemberImportRow[]
  valid_rows: number
  invalid_rows: number
}

export interface EnterpriseMemberImportResult {
  job_id: number
  status: 'completed'
  created_members: number
  created_keys: number
  rows: number[]
  keys: Array<{ member_code: string; key_name: string; key?: string; key_masked: string }>
  completed_at: string
}

export interface EnterpriseMemberImportJob {
  id: number
  status: 'queued' | 'processing' | 'completed' | 'failed'
  result?: EnterpriseMemberImportResult | null
  selected_rows: number[]
  attempt_count: number
  error_code?: string | null
  error_summary?: string | null
  queued_at?: string | null
  started_at?: string | null
  updated_at: string
  completed_at?: string | null
  result_secrets_consumed_at?: string | null
}

export interface EnterpriseMemberImportQueueResult {
  job_id: number
  status: 'queued' | 'processing' | 'completed'
}

export interface EnterpriseMemberKeyUpdate {
  name: string
  status: 'active' | 'disabled'
  quota: number
  expires_at: string
  rate_limit_5h: number
  rate_limit_1d: number
  rate_limit_7d: number
  ip_whitelist: string[]
  ip_blacklist: string[]
}

export interface EnterpriseMemberKeyAdoptionResult {
  key_id: number
  original_group_id: number
  group_added: boolean
  group_ids: number[]
  member_version: number
}

function idempotencyKey(prefix: string): string {
  return typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID()
    : `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export async function list(includeArchived = false): Promise<EnterpriseMember[]> {
  const { data } = await apiClient.get<EnterpriseMember[]>('/enterprise/members', { params: { include_archived: includeArchived } })
  return data
}

export async function create(input: CreateEnterpriseMemberInput): Promise<EnterpriseMember> {
  const { data } = await apiClient.post<EnterpriseMember>('/enterprise/members', input, { headers: { 'Idempotency-Key': idempotencyKey('member') } })
  return data
}

export async function update(member: EnterpriseMember, input: Partial<EnterpriseMemberDraft>): Promise<EnterpriseMember> {
  const { data } = await apiClient.patch<EnterpriseMember>(`/enterprise/members/${member.id}`, { expected_version: member.version, ...input })
  return data
}

export async function replaceGroups(member: EnterpriseMember, groupIds: number[]): Promise<{ group_ids: number[]; version: number }> {
  const { data } = await apiClient.put<{ group_ids: number[]; version: number }>(`/enterprise/members/${member.id}/groups`, {
    expected_version: member.version,
    group_ids: groupIds
  })
  return data
}

export async function setStatus(member: EnterpriseMember, status: EnterpriseMemberStatus): Promise<EnterpriseMember> {
  const { data } = await apiClient.post<EnterpriseMember>(`/enterprise/members/${member.id}/${status === 'active' ? 'enable' : 'disable'}`, {
    expected_version: member.version
  })
  return data
}

export async function archive(member: EnterpriseMember): Promise<void> {
  await apiClient.delete(`/enterprise/members/${member.id}`, { params: { expected_version: member.version } })
}

export async function permanentlyDelete(member: EnterpriseMember): Promise<void> {
  await apiClient.delete(`/enterprise/members/${member.id}`, { params: { permanent: true } })
}

export async function listKeys(memberId: number): Promise<ApiKey[]> {
  const { data } = await apiClient.get<ApiKey[]>(`/enterprise/members/${memberId}/keys`)
  return data
}

export async function listAdoptableKeys(memberId: number): Promise<ApiKey[]> {
  const { data } = await apiClient.get<ApiKey[]>(`/enterprise/members/${memberId}/adoptable-keys`)
  return data
}

export async function adoptKey(member: EnterpriseMember, keyId: number): Promise<EnterpriseMemberKeyAdoptionResult> {
  const { data } = await apiClient.post<EnterpriseMemberKeyAdoptionResult>(`/enterprise/members/${member.id}/keys/${keyId}/adopt`, {
    expected_version: member.version
  }, { headers: { 'Idempotency-Key': idempotencyKey('member-key-adopt') } })
  return data
}

export async function createKey(memberId: number, input: { name: string; quota?: number; expires_in_days?: number }): Promise<ApiKey> {
  const { data } = await apiClient.post<ApiKey>(`/enterprise/members/${memberId}/keys`, input, { headers: { 'Idempotency-Key': idempotencyKey('member-key') } })
  return data
}

export async function updateKey(memberId: number, keyId: number, input: EnterpriseMemberKeyUpdate): Promise<ApiKey> {
  const { data } = await apiClient.patch<ApiKey>(`/enterprise/members/${memberId}/keys/${keyId}`, input)
  return data
}

export async function deleteKey(memberId: number, keyId: number): Promise<void> {
  await apiClient.delete(`/enterprise/members/${memberId}/keys/${keyId}`)
}

export async function getBudget(memberId: number): Promise<EnterpriseMemberBudgetSummary> {
  const { data } = await apiClient.get<EnterpriseMemberBudgetSummary>(`/enterprise/members/${memberId}/budget`)
  return data
}

export async function getOwnerUsageSummary(): Promise<EnterpriseMemberOwnerUsageSummary> {
  const { data } = await apiClient.get<EnterpriseMemberOwnerUsageSummary>('/enterprise/members/usage/summary')
  return data
}

export async function listBudgetEntries(memberId: number, page = 1, pageSize = 50): Promise<{ items: EnterpriseMemberBudgetEntry[]; total: number; page: number; page_size: number }> {
  const { data } = await apiClient.get(`/enterprise/members/${memberId}/budget/entries`, { params: { page, page_size: pageSize } })
  return data
}

export async function listAuditEvents(memberId: number, page = 1, pageSize = 50): Promise<{ items: EnterpriseMemberAuditEvent[]; total: number; page: number; page_size: number }> {
  const { data } = await apiClient.get(`/enterprise/members/${memberId}/audit`, { params: { page, page_size: pageSize } })
  return data
}

export async function listOwnerAuditEvents(page = 1, pageSize = 100): Promise<{ items: EnterpriseMemberAuditEvent[]; total: number; page: number; page_size: number }> {
  const { data } = await apiClient.get('/enterprise/members/audit', { params: { page, page_size: pageSize } })
  return data
}

export async function createBudgetAdjustment(memberId: number, amountUSD: number, note: string): Promise<EnterpriseMemberBudgetSummary> {
  const { data } = await apiClient.post<EnterpriseMemberBudgetSummary>(`/enterprise/members/${memberId}/budget/adjustments`, {
    amount_usd: amountUSD,
    note
  }, { headers: { 'Idempotency-Key': idempotencyKey('member-budget-adjustment') } })
  return data
}

export async function setUsage(memberId: number, input: { monthly_used_usd: number; usage_5h: number; usage_1d: number; usage_7d: number }): Promise<EnterpriseMemberBudgetSummary> {
  const { data } = await apiClient.put<EnterpriseMemberBudgetSummary>(`/enterprise/members/${memberId}/usage`, input, {
    headers: { 'Idempotency-Key': idempotencyKey('member-usage-adjustment') }
  })
  return data
}

export async function getUsageAnalytics(memberId: number, days = 30): Promise<EnterpriseMemberUsageAnalytics> {
  const { data } = await apiClient.get<EnterpriseMemberUsageAnalytics>(`/enterprise/members/${memberId}/usage/analytics`, { params: { days } })
  return data
}

export async function listUsageRecords(memberId: number, page = 1, pageSize = 20): Promise<{ items: EnterpriseMemberUsageRecord[]; total: number; page: number; page_size: number }> {
  const { data } = await apiClient.get(`/enterprise/members/${memberId}/usage/records`, { params: { page, page_size: pageSize } })
  return data
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  window.setTimeout(() => URL.revokeObjectURL(url), 0)
}

export async function downloadImportTemplate(format: 'csv' | 'xlsx'): Promise<void> {
  const response = await apiClient.get('/enterprise/members/import/template', { params: { format }, responseType: 'blob' })
  downloadBlob(response.data, `企业成员导入模板.${format}`)
}

export async function previewImport(file: File): Promise<EnterpriseMemberImportPreview> {
  const form = new FormData()
  form.append('file', file)
  const format = file.name.toLocaleLowerCase().endsWith('.xlsx') ? 'xlsx' : 'csv'
  form.append('format', format)
  const { data } = await apiClient.post<EnterpriseMemberImportPreview>('/enterprise/members/import/preview', form, { headers: { 'Content-Type': undefined } })
  return data
}

export async function commitImport(preview: EnterpriseMemberImportPreview, selectedRows: number[]): Promise<EnterpriseMemberImportQueueResult> {
  const { data } = await apiClient.post<EnterpriseMemberImportQueueResult>('/enterprise/members/import/commit', {
    job_id: preview.job_id,
    preview_token: preview.token,
    selected_rows: selectedRows
  }, { headers: { 'Idempotency-Key': idempotencyKey('member-import') } })
  return data
}

export async function getImportJob(jobId: number): Promise<EnterpriseMemberImportJob> {
  const { data } = await apiClient.get<EnterpriseMemberImportJob>(`/enterprise/members/import/jobs/${jobId}`)
  return data
}

export async function consumeImportResultSecrets(jobId: number, resultToken: string): Promise<EnterpriseMemberImportResult['keys']> {
  const { data } = await apiClient.post<{ keys: EnterpriseMemberImportResult['keys'] }>(`/enterprise/members/import/jobs/${jobId}/result-secrets`, { result_token: resultToken })
  return data.keys
}

export async function downloadImportErrorReport(jobId: number): Promise<void> {
  const response = await apiClient.get(`/enterprise/members/import/jobs/${jobId}/error-report`, { responseType: 'blob' })
  downloadBlob(response.data, 'enterprise-member-import-errors.csv')
}

export const enterpriseMembersAPI = {
  list,
  create,
  update,
  replaceGroups,
  setStatus,
  archive,
  permanentlyDelete,
  listKeys,
  listAdoptableKeys,
  adoptKey,
  createKey,
  updateKey,
  deleteKey,
  getBudget,
  getOwnerUsageSummary,
  listBudgetEntries,
  listAuditEvents,
  listOwnerAuditEvents,
  createBudgetAdjustment,
  setUsage,
  getUsageAnalytics,
  listUsageRecords,
  downloadImportTemplate,
  previewImport,
  commitImport,
  getImportJob,
  consumeImportResultSecrets,
  downloadImportErrorReport
}
