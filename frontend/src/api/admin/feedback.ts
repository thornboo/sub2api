import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type {
  FeedbackClosedBy,
  FeedbackReadState,
  FeedbackReply,
  FeedbackReplyResult,
  FeedbackSource,
  FeedbackStatus,
  FeedbackTicket,
} from '@/api/feedback'

export type AdminFeedbackSource = FeedbackSource
export type AdminFeedbackStatus = FeedbackStatus

export interface AdminFeedbackRecord extends FeedbackTicket {
  user_id: number
  user_email?: string
  api_key_id: number | null
  key_name: string
  key_prefix: string
  member_id: number | null
  closed_by: FeedbackClosedBy | null
}

export interface AdminFeedbackListFilters {
  status?: AdminFeedbackStatus
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters: AdminFeedbackListFilters = {},
  options?: { signal?: AbortSignal },
): Promise<BasePaginationResponse<AdminFeedbackRecord>> {
  const { data } = await apiClient.get<BasePaginationResponse<AdminFeedbackRecord>>('/admin/feedback', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal,
  })
  return data
}

export async function get(id: number, signal?: AbortSignal): Promise<AdminFeedbackRecord> {
  const { data } = await apiClient.get<AdminFeedbackRecord>(`/admin/feedback/${id}`, { signal })
  return data
}

export async function listMessages(
  id: number,
  page: number = 1,
  pageSize: number = 20,
  signal?: AbortSignal,
): Promise<BasePaginationResponse<FeedbackReply>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackReply>>(`/admin/feedback/${id}/messages`, {
    params: { page, page_size: pageSize },
    signal,
  })
  return data
}

export async function reply(id: number, content: string, signal?: AbortSignal): Promise<FeedbackReplyResult> {
  const { data } = await apiClient.post<FeedbackReplyResult>(`/admin/feedback/${id}/messages`, { content }, { signal })
  return data
}

export async function close(id: number, signal?: AbortSignal): Promise<AdminFeedbackRecord> {
  const { data } = await apiClient.post<AdminFeedbackRecord>(`/admin/feedback/${id}/close`, {}, { signal })
  return data
}

export async function markRead(id: number, lastReadReplyID: number, signal?: AbortSignal): Promise<FeedbackReadState> {
  const { data } = await apiClient.post<FeedbackReadState>(`/admin/feedback/${id}/read`, { last_read_reply_id: lastReadReplyID }, { signal })
  return data
}

const feedbackAPI = {
  list,
  get,
  listMessages,
  reply,
  close,
  markRead,
}

export default feedbackAPI
