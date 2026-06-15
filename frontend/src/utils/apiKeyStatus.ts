import type { ApiKey } from '@/types'

export type ApiKeyStatus = ApiKey['status']
export type ApiKeyMutableStatus = Extract<ApiKeyStatus, 'active' | 'disabled'>
export type ApiKeySystemStatus = Extract<ApiKeyStatus, 'quota_exhausted' | 'expired'>

export function isApiKeySystemStatus(status: string | null | undefined): status is ApiKeySystemStatus {
  return status === 'quota_exhausted' || status === 'expired'
}

export function canToggleApiKeyStatus(status: ApiKeyStatus): status is ApiKeyMutableStatus {
  return status === 'active' || status === 'disabled'
}

export function initialApiKeyEditStatus(status: ApiKeyStatus): ApiKeyMutableStatus {
  return status === 'active' ? 'active' : 'disabled'
}

export function shouldPreserveApiKeySystemStatus(
  currentStatus: ApiKeyStatus,
  nextStatus: ApiKeyMutableStatus,
  manuallyDisableSystemStatus: boolean
): boolean {
  return isApiKeySystemStatus(currentStatus) &&
    nextStatus === 'disabled' &&
    !manuallyDisableSystemStatus
}
