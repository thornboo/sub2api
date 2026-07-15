import { describe, expect, it } from 'vitest'
import { fillEnterpriseMemberUsageTrend } from '../enterpriseMemberUsageTrend'

describe('fillEnterpriseMemberUsageTrend', () => {
  it('fills missing local calendar days so one active day does not occupy the whole chart', () => {
    const result = fillEnterpriseMemberUsageTrend([
      { date: '2026-07-15', request_count: 2, input_tokens: 100, output_tokens: 20, actual_cost: 0.5 }
    ], '2026-07-13T16:00:00Z', 3, 'Asia/Shanghai')

    expect(result).toEqual([
      { date: '2026-07-14', request_count: 0, input_tokens: 0, output_tokens: 0, actual_cost: 0 },
      { date: '2026-07-15', request_count: 2, input_tokens: 100, output_tokens: 20, actual_cost: 0.5 },
      { date: '2026-07-16', request_count: 0, input_tokens: 0, output_tokens: 0, actual_cost: 0 }
    ])
  })

  it('returns the original series when range metadata is invalid', () => {
    const points = [{ date: '2026-07-15', request_count: 1, input_tokens: 10, output_tokens: 5, actual_cost: 0.1 }]
    expect(fillEnterpriseMemberUsageTrend(points, 'invalid', 30, 'Asia/Shanghai')).toEqual(points)
    expect(fillEnterpriseMemberUsageTrend(points, '2026-07-15T00:00:00Z', 0, 'Asia/Shanghai')).toEqual(points)
  })
})
