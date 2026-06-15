import { describe, expect, it } from 'vitest'
import {
  canToggleApiKeyStatus,
  initialApiKeyEditStatus,
  isApiKeySystemStatus,
  shouldPreserveApiKeySystemStatus,
} from '../apiKeyStatus'

describe('apiKeyStatus helpers', () => {
  it('classifies system statuses separately from user-toggleable statuses', () => {
    expect(isApiKeySystemStatus('quota_exhausted')).toBe(true)
    expect(isApiKeySystemStatus('expired')).toBe(true)
    expect(isApiKeySystemStatus('active')).toBe(false)
    expect(isApiKeySystemStatus('disabled')).toBe(false)

    expect(canToggleApiKeyStatus('active')).toBe(true)
    expect(canToggleApiKeyStatus('disabled')).toBe(true)
    expect(canToggleApiKeyStatus('quota_exhausted')).toBe(false)
    expect(canToggleApiKeyStatus('expired')).toBe(false)
  })

  it('uses disabled as the editable default for non-active statuses', () => {
    expect(initialApiKeyEditStatus('active')).toBe('active')
    expect(initialApiKeyEditStatus('disabled')).toBe('disabled')
    expect(initialApiKeyEditStatus('quota_exhausted')).toBe('disabled')
    expect(initialApiKeyEditStatus('expired')).toBe('disabled')
  })

  it('preserves system status unless the user explicitly disables it', () => {
    expect(shouldPreserveApiKeySystemStatus('quota_exhausted', 'disabled', false)).toBe(true)
    expect(shouldPreserveApiKeySystemStatus('expired', 'disabled', false)).toBe(true)
    expect(shouldPreserveApiKeySystemStatus('quota_exhausted', 'disabled', true)).toBe(false)
    expect(shouldPreserveApiKeySystemStatus('expired', 'disabled', true)).toBe(false)
    expect(shouldPreserveApiKeySystemStatus('disabled', 'disabled', false)).toBe(false)
    expect(shouldPreserveApiKeySystemStatus('active', 'active', false)).toBe(false)
  })
})
