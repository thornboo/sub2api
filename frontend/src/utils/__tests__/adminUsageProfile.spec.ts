import { describe, expect, it } from 'vitest'
import {
  buildApiKeyProfileFilters,
  buildUserProfileFilters,
  clearApiKeyProfileFilters,
  clearUserProfileFilters,
  sanitizeAdminUsageRouteQuery,
} from '../adminUsageProfile'

describe('sanitizeAdminUsageRouteQuery', () => {
  const defaults = { startDate: '2026-06-14', endDate: '2026-06-15' }

  it('keeps the full supported admin usage filter set when query values are valid', () => {
    const result = sanitizeAdminUsageRouteQuery({
      user_id: '12',
      api_key_id: '34',
      account_id: '56',
      group_id: '78',
      model: ' claude-sonnet ',
      request_type: 'stream',
      billing_type: '1',
      billing_mode: 'per_request',
      start_date: '2026-06-01',
      end_date: '2026-06-10',
    }, defaults)

    expect(result.filters).toEqual({
      user_id: 12,
      api_key_id: 34,
      account_id: 56,
      group_id: 78,
      model: 'claude-sonnet',
      request_type: 'stream',
      billing_type: 1,
      billing_mode: 'per_request',
      start_date: '2026-06-01',
      end_date: '2026-06-10',
    })
    expect(result.routeQuery).toEqual({
      user_id: '12',
      api_key_id: '34',
      account_id: '56',
      group_id: '78',
      model: 'claude-sonnet',
      request_type: 'stream',
      billing_type: '1',
      billing_mode: 'per_request',
      start_date: '2026-06-01',
      end_date: '2026-06-10',
    })
  })

  it('drops dirty or unsupported query values before they reach API filters', () => {
    const result = sanitizeAdminUsageRouteQuery({
      user_id: '-1',
      api_key_id: '0',
      account_id: 'abc',
      group_id: '9.5',
      model: '   ',
      request_type: 'chat',
      billing_type: '2',
      billing_mode: 'flat',
      start_date: '2026/06/01',
      end_date: '2026-02-31',
    }, defaults)

    expect(result.filters).toEqual({
      start_date: defaults.startDate,
      end_date: defaults.endDate,
    })
    expect(result.routeQuery).toEqual({})
  })

  it('uses the first non-empty query value when duplicate query params are present', () => {
    const result = sanitizeAdminUsageRouteQuery({
      user_id: ['', '42'],
      request_type: ['unknown', 'sync'],
    }, defaults)

    expect(result.filters.user_id).toBe(42)
    expect(result.filters.request_type).toBeUndefined()
  })
})

describe('admin usage profile filter transitions', () => {
  const current = {
    user_id: 1,
    api_key_id: 2,
    account_id: 3,
    group_id: 4,
    model: 'old-model',
    request_type: 'sync' as const,
    billing_type: 0,
    billing_mode: 'token',
    start_date: '2026-06-01',
    end_date: '2026-06-15',
  }

  it('switches to a different user while preserving date and diagnostic filters', () => {
    expect(buildUserProfileFilters(current, 99)).toEqual({
      user_id: 99,
      group_id: 4,
      request_type: 'sync',
      billing_type: 0,
      billing_mode: 'token',
      start_date: '2026-06-01',
      end_date: '2026-06-15',
    })
  })

  it('switches to a key profile without carrying stale account or model filters', () => {
    expect(buildApiKeyProfileFilters(current, 88, { user_id: 99 })).toEqual({
      user_id: 99,
      api_key_id: 88,
      group_id: 4,
      request_type: 'sync',
      billing_type: 0,
      billing_mode: 'token',
      start_date: '2026-06-01',
      end_date: '2026-06-15',
    })
  })

  it('clears user scope together with the nested key scope', () => {
    expect(clearUserProfileFilters(current)).toMatchObject({
      user_id: undefined,
      api_key_id: undefined,
      account_id: 3,
      model: 'old-model',
    })
  })

  it('clears only the key scope when returning from a key profile to the user profile', () => {
    expect(clearApiKeyProfileFilters(current)).toMatchObject({
      user_id: 1,
      api_key_id: undefined,
      account_id: 3,
      model: 'old-model',
    })
  })
})
