/**
 * Model Plaza API（公开端点，可匿名访问）
 * 公开目录与登录后的「可用渠道」复用同一份客户安全渠道、模型、价格和端点 DTO。
 * 请求是否携带 token 不改变目录内容。
 */

import { apiClient } from './client'
import {
  normalizeAvailableChannels,
  type UserAvailableChannel,
} from './channels'

export interface ModelPlazaResponse {
  /** 管理员配置的全局价格说明（Markdown）。 */
  description: string
  channels: UserAvailableChannel[]
}

/** 获取模型广场数据。开关未启用时后端返回 404。 */
export async function getModelPlaza(options?: { signal?: AbortSignal }): Promise<ModelPlazaResponse> {
  const { data } = await apiClient.get<ModelPlazaResponse>('/model-plaza', {
    signal: options?.signal
  })
  return {
    ...data,
    description: data.description ?? '',
    channels: normalizeAvailableChannels(data.channels),
  }
}

export const modelPlazaAPI = { getModelPlaza }

export default modelPlazaAPI
