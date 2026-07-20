import { describe, expect, it } from 'vitest'
import {
  DEFAULT_UPSTREAM_BALANCE_ENDPOINT,
  NEW_API_UPSTREAM_BALANCE_ENDPOINT,
  SUB2API_PROFILE_UPSTREAM_BALANCE_ENDPOINT,
  UPSTREAM_BALANCE_AUTH_HEADER_KEY,
  UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY,
  UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN,
  UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER,
  UPSTREAM_BALANCE_AUTH_MODE_KEY,
  UPSTREAM_BALANCE_ENDPOINT_KEY,
  UPSTREAM_BALANCE_PROVIDER_KEY,
  UPSTREAM_BALANCE_PROVIDER_NEW_API,
  UPSTREAM_BALANCE_PROVIDER_SUB2API,
  UPSTREAM_BALANCE_QUERY_ENABLED_KEY,
  UPSTREAM_BALANCE_SNAPSHOT_KEY,
  UPSTREAM_COST_MODEL_FAMILIES_KEY,
  UPSTREAM_GROUP_MULTIPLIER_KEY,
  UPSTREAM_RECHARGE_CNY_PER_USD_KEY,
  UPSTREAM_REFERENCE_FX_RATE_KEY,
  calculateUpstreamBindingEffectiveFactor,
  calculateUpstreamCost,
  isUpstreamBalanceQueryEnabled,
  mergeUpstreamCostProfileExtra,
  normalizeUpstreamBalanceAuthMode,
  normalizeUpstreamBalanceEndpoint,
  readUpstreamBalanceSnapshot,
  readUpstreamCostProfile,
  formatUpstreamDiscountLabel,
  requiresUpstreamBalanceAuthToken
} from '@/utils/upstreamCost'

describe('upstreamCost utils', () => {
  it('uses the selected upstream group price basis for effective discount', () => {
    expect(calculateUpstreamBindingEffectiveFactor(1, 7, 0.8, 'CNY')).toBeCloseTo(0.8, 6)
    expect(calculateUpstreamBindingEffectiveFactor(1, 7, 0.8, 'USD')).toBeCloseTo(0.114286, 6)
    expect(calculateUpstreamBindingEffectiveFactor(1, 7, 0.8, undefined)).toBeCloseTo(0.114286, 6)
  })

  it('calculates effective discount from recharge ratio and group multiplier', () => {
    const result = calculateUpstreamCost({
      recharge_cny_per_usd: 1,
      reference_fx_rate: 6.9,
      group_multiplier: 0.5
    })

    expect(result.complete).toBe(true)
    expect(result.recharge_cost_factor).toBeCloseTo(0.1449, 4)
    expect(result.effective_discount).toBeCloseTo(0.07246, 4)
    expect(result.display_discount).toBeCloseTo(0.7246, 4)
    expect(result.label).toBe('0.7折')
  })

  it('uses model-family multiplier override without changing recharge settings', () => {
    const result = calculateUpstreamCost({
      recharge_cny_per_usd: 1,
      reference_fx_rate: 7,
      group_multiplier: 1,
      model_families: [
        { family: 'haiku', group_multiplier: 0.25 },
        { family: 'opus', group_multiplier: 0.9 }
      ]
    }, 'HAIKU')

    expect(result.complete).toBe(true)
    expect(result.source).toBe('family_override')
    expect(result.group_multiplier).toBe(0.25)
    expect(result.effective_discount).toBeCloseTo(0.03571, 4)
  })

  it('uses 7 as the default reference FX rate', () => {
    const result = calculateUpstreamCost({
      recharge_cny_per_usd: 1,
      group_multiplier: 0.5
    })

    expect(result.configured).toBe(true)
    expect(result.complete).toBe(true)
    expect(result.reference_fx_rate).toBe(7)
    expect(result.effective_discount).toBeCloseTo(0.07143, 4)
    expect(result.label).toBe('0.7折')
    expect(result.missing_fields).toEqual([])
  })

  it('still reports missing recharge or multiplier fields', () => {
    const result = calculateUpstreamCost({
      reference_fx_rate: 7
    })

    expect(result.configured).toBe(true)
    expect(result.complete).toBe(false)
    expect(result.label).toBe('未配置')
    expect(result.missing_fields).toEqual(['recharge_cny_per_usd', 'group_multiplier'])
  })

  it('formats discount labels with localized suffixes', () => {
    expect(formatUpstreamDiscountLabel(1.43, {
      suffix: '/10',
      notConfiguredLabel: 'Not configured'
    })).toBe('1.4/10')
    expect(formatUpstreamDiscountLabel(undefined, {
      suffix: '/10',
      notConfiguredLabel: 'Not configured'
    })).toBe('Not configured')
  })

  it('reads and writes cost profile fields in account extra', () => {
    const extra = mergeUpstreamCostProfileExtra(
      { preserved: true, [UPSTREAM_GROUP_MULTIPLIER_KEY]: 99 },
      {
        recharge_cny_per_usd: 1,
        reference_fx_rate: 6.9,
        group_multiplier: 0.5,
        model_families: [
          { family: 'sonnet', group_multiplier: 0.7, note: 'fast lane' },
          { family: ' ', group_multiplier: 1 }
        ]
      }
    )

    expect(extra.preserved).toBe(true)
    expect(extra[UPSTREAM_RECHARGE_CNY_PER_USD_KEY]).toBe(1)
    expect(extra[UPSTREAM_REFERENCE_FX_RATE_KEY]).toBe(6.9)
    expect(extra[UPSTREAM_GROUP_MULTIPLIER_KEY]).toBe(0.5)
    expect(extra[UPSTREAM_COST_MODEL_FAMILIES_KEY]).toEqual([
      { family: 'sonnet', group_multiplier: 0.7, note: 'fast lane' }
    ])
    expect(readUpstreamCostProfile(extra)).toEqual({
      recharge_cny_per_usd: 1,
      reference_fx_rate: 6.9,
      group_multiplier: 0.5,
      model_families: [
        { family: 'sonnet', group_multiplier: 0.7, note: 'fast lane' }
      ]
    })
  })

  it('clears existing cost keys while keeping unrelated extra fields', () => {
    const extra = mergeUpstreamCostProfileExtra({
      preserved: true,
      [UPSTREAM_RECHARGE_CNY_PER_USD_KEY]: 1,
      [UPSTREAM_REFERENCE_FX_RATE_KEY]: 7,
      [UPSTREAM_GROUP_MULTIPLIER_KEY]: 0.5,
      upstream_account_balance_query_enabled: true,
      upstream_account_balance_endpoint: '/api/v1/user/profile'
    }, {})

    expect(extra).toEqual({ preserved: true })
  })

  it('reads and writes upstream balance query config separately from cost completeness', () => {
    const extra = mergeUpstreamCostProfileExtra(
      { preserved: true },
      { balance_query_enabled: true, balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN }
    )

    expect(extra.preserved).toBe(true)
    expect(extra[UPSTREAM_BALANCE_QUERY_ENABLED_KEY]).toBe(true)
    expect(extra[UPSTREAM_BALANCE_PROVIDER_KEY]).toBe(UPSTREAM_BALANCE_PROVIDER_SUB2API)
    expect(extra[UPSTREAM_BALANCE_ENDPOINT_KEY]).toBe(DEFAULT_UPSTREAM_BALANCE_ENDPOINT)
    expect(extra[UPSTREAM_BALANCE_AUTH_MODE_KEY]).toBe(UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN)
    expect(isUpstreamBalanceQueryEnabled(extra)).toBe(true)
    expect(readUpstreamCostProfile(extra)).toEqual({
      balance_query_enabled: true,
      balance_provider: UPSTREAM_BALANCE_PROVIDER_SUB2API,
      balance_endpoint: DEFAULT_UPSTREAM_BALANCE_ENDPOINT,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN
    })
    expect(calculateUpstreamCost(readUpstreamCostProfile(extra)).configured).toBe(false)
  })

  it('uses New API defaults only when that provider is selected', () => {
    const extra = mergeUpstreamCostProfileExtra({}, {
      balance_query_enabled: true,
      balance_provider: UPSTREAM_BALANCE_PROVIDER_NEW_API
    })

    expect(extra[UPSTREAM_BALANCE_PROVIDER_KEY]).toBe(UPSTREAM_BALANCE_PROVIDER_NEW_API)
    expect(extra[UPSTREAM_BALANCE_ENDPOINT_KEY]).toBe(NEW_API_UPSTREAM_BALANCE_ENDPOINT)
    expect(extra[UPSTREAM_BALANCE_AUTH_MODE_KEY]).toBe(UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY)
    expect(requiresUpstreamBalanceAuthToken(readUpstreamCostProfile(extra))).toBe(false)
  })

  it('keeps Sub2API account API key auth on the usage endpoint', () => {
    const extra = mergeUpstreamCostProfileExtra({}, {
      balance_query_enabled: true,
      balance_provider: UPSTREAM_BALANCE_PROVIDER_SUB2API,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
    })

    expect(normalizeUpstreamBalanceAuthMode(
      UPSTREAM_BALANCE_PROVIDER_SUB2API,
      UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
    )).toBe(UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY)
    expect(extra[UPSTREAM_BALANCE_ENDPOINT_KEY]).toBe(DEFAULT_UPSTREAM_BALANCE_ENDPOINT)
    expect(extra[UPSTREAM_BALANCE_AUTH_MODE_KEY]).toBe(UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY)
    expect(readUpstreamCostProfile(extra).balance_auth_mode).toBe(UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY)
    expect(requiresUpstreamBalanceAuthToken(readUpstreamCostProfile(extra))).toBe(false)
  })

  it('migrates legacy Sub2API profile endpoint when using model API key auth', () => {
    const extra = mergeUpstreamCostProfileExtra({}, {
      balance_query_enabled: true,
      balance_provider: UPSTREAM_BALANCE_PROVIDER_SUB2API,
      balance_endpoint: SUB2API_PROFILE_UPSTREAM_BALANCE_ENDPOINT,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
    })

    expect(normalizeUpstreamBalanceEndpoint(
      UPSTREAM_BALANCE_PROVIDER_SUB2API,
      SUB2API_PROFILE_UPSTREAM_BALANCE_ENDPOINT,
      UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
    )).toBe(DEFAULT_UPSTREAM_BALANCE_ENDPOINT)
    expect(extra[UPSTREAM_BALANCE_ENDPOINT_KEY]).toBe(DEFAULT_UPSTREAM_BALANCE_ENDPOINT)
    expect(extra[UPSTREAM_BALANCE_AUTH_MODE_KEY]).toBe(UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY)
  })

  it('writes custom header auth config only when custom header mode is enabled', () => {
    const extra = mergeUpstreamCostProfileExtra({}, {
      balance_query_enabled: true,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER,
      balance_auth_header: 'X-Panel-Token'
    })

    expect(extra[UPSTREAM_BALANCE_AUTH_MODE_KEY]).toBe(UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER)
    expect(extra[UPSTREAM_BALANCE_AUTH_HEADER_KEY]).toBe('X-Panel-Token')
    expect(readUpstreamCostProfile(extra).balance_auth_header).toBe('X-Panel-Token')
  })

  it('requires a dedicated balance token only for non-account-key auth modes', () => {
    expect(requiresUpstreamBalanceAuthToken({})).toBe(false)
    expect(requiresUpstreamBalanceAuthToken({
      balance_query_enabled: true,
      balance_provider: UPSTREAM_BALANCE_PROVIDER_SUB2API
    })).toBe(false)
    expect(requiresUpstreamBalanceAuthToken({
      balance_query_enabled: true,
      balance_provider: UPSTREAM_BALANCE_PROVIDER_NEW_API,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_ACCOUNT_API_KEY
    })).toBe(false)
    expect(requiresUpstreamBalanceAuthToken({
      balance_query_enabled: true,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_BEARER_TOKEN
    })).toBe(true)
    expect(requiresUpstreamBalanceAuthToken({
      balance_query_enabled: true,
      balance_auth_mode: UPSTREAM_BALANCE_AUTH_MODE_CUSTOM_HEADER
    })).toBe(true)
  })

  it('reads upstream balance snapshot from account extra', () => {
    const snapshot = {
      status: 'ok',
      available_usd: 8.64,
      raw_available: 4320000,
      fetched_at: '2026-06-23T00:00:00Z'
    }
    const extra = {
      [UPSTREAM_BALANCE_SNAPSHOT_KEY]: snapshot
    }

    expect(readUpstreamBalanceSnapshot(extra)).toEqual(snapshot)
    expect(readUpstreamBalanceSnapshot({})).toBeNull()
  })

})
