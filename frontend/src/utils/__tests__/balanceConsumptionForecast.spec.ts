import { describe, expect, it } from 'vitest'

import { calculateBalanceDepletionForecast } from '../balanceConsumptionForecast'

describe('calculateBalanceDepletionForecast', () => {
  it('estimates the rounded-up runway and depletion date', () => {
    expect(calculateBalanceDepletionForecast(53641.08, 1195.1518, '2026-09-05')).toEqual({
      status: 'estimated',
      remainingDays: 45,
      depletionDate: '2026-10-20'
    })
  })

  it('reports a runway shorter than one day', () => {
    expect(calculateBalanceDepletionForecast(5, 10, '2026-09-05')).toEqual({
      status: 'less-than-day',
      remainingDays: 1,
      depletionDate: '2026-09-06'
    })
  })

  it('reports an exhausted balance without a projected date', () => {
    expect(calculateBalanceDepletionForecast(0, 10, '2026-09-05')).toEqual({
      status: 'depleted',
      remainingDays: 0,
      depletionDate: null
    })
  })

  it('does not invent a depletion date without spending', () => {
    expect(calculateBalanceDepletionForecast(100, 0, '2026-09-05')).toEqual({
      status: 'no-consumption',
      remainingDays: null,
      depletionDate: null
    })
  })
})
