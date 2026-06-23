import { describe, expect, it } from 'vitest'
import {
  UPSTREAM_COST_MODEL_FAMILIES_KEY,
  UPSTREAM_GROUP_MULTIPLIER_KEY,
  UPSTREAM_RECHARGE_CNY_PER_USD_KEY,
  UPSTREAM_REFERENCE_FX_RATE_KEY,
  calculateUpstreamCost,
  mergeUpstreamCostProfileExtra,
  readUpstreamCostProfile
} from '@/utils/upstreamCost'

describe('upstreamCost utils', () => {
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
      [UPSTREAM_GROUP_MULTIPLIER_KEY]: 0.5
    }, {})

    expect(extra).toEqual({ preserved: true })
  })
})
