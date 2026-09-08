import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export type FeedbackSource = 'user' | 'key'
export type FeedbackStatus = 'open' | 'closed'
export type FeedbackClosedBy = 'user' | 'admin'
export type FeedbackReplyAuthorRole = 'user' | 'admin'

export interface FeedbackTicket {
  id: number
  content: string
  source: FeedbackSource
  status: FeedbackStatus
  created_at: string
  updated_at: string
  closed_at: string | null
  closed_by: FeedbackClosedBy | null
  unread_count: number
}

export interface FeedbackReply {
  id: number
  feedback_id: number
  author_role: FeedbackReplyAuthorRole
  content: string
  created_at: string
}

export interface FeedbackSubmitResult {
  id: number
  created_at: string
  retry_after: number
}

export interface FeedbackSubmitRequest {
  content: string
}

export interface FeedbackListFilters {
  status?: FeedbackStatus
}

export interface FeedbackReplyResult {
  message: FeedbackReply
  retry_after: number
}

export interface FeedbackReadState {
  unread_count: number
  last_read_reply_id: number
}

export async function submit(request: FeedbackSubmitRequest, signal?: AbortSignal): Promise<FeedbackSubmitResult> {
  const { data } = await apiClient.post<FeedbackSubmitResult>('/feedback', request, { signal })
  return data
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters: FeedbackListFilters = {},
  options?: { signal?: AbortSignal },
): Promise<BasePaginationResponse<FeedbackTicket>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackTicket>>('/feedback', {
    params: { page, page_size: pageSize, ...filters },
    signal: options?.signal,
  })
  return data
}

export async function get(id: number, signal?: AbortSignal): Promise<FeedbackTicket> {
  const { data } = await apiClient.get<FeedbackTicket>(`/feedback/${id}`, { signal })
  return data
}

export async function listMessages(
  id: number,
  page: number = 1,
  pageSize: number = 20,
  signal?: AbortSignal,
): Promise<BasePaginationResponse<FeedbackReply>> {
  const { data } = await apiClient.get<BasePaginationResponse<FeedbackReply>>(`/feedback/${id}/messages`, {
    params: { page, page_size: pageSize },
    signal,
  })
  return data
}

export async function reply(id: number, content: string, signal?: AbortSignal): Promise<FeedbackReplyResult> {
  const { data } = await apiClient.post<FeedbackReplyResult>(`/feedback/${id}/messages`, { content }, { signal })
  return data
}

export async function close(id: number, signal?: AbortSignal): Promise<FeedbackTicket> {
  const { data } = await apiClient.post<FeedbackTicket>(`/feedback/${id}/close`, {}, { signal })
  return data
}

export async function markRead(id: number, lastReadReplyID: number, signal?: AbortSignal): Promise<FeedbackReadState> {
  const { data } = await apiClient.post<FeedbackReadState>(`/feedback/${id}/read`, { last_read_reply_id: lastReadReplyID }, { signal })
  return data
}

const feedbackAPI = {
  submit,
  list,
  get,
  listMessages,
  reply,
  close,
  markRead,
}

export default feedbackAPI
