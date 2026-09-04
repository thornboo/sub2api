export type BalanceDepletionForecastStatus =
  | 'estimated'
  | 'less-than-day'
  | 'depleted'
  | 'no-consumption'

export interface BalanceDepletionForecast {
  status: BalanceDepletionForecastStatus
  remainingDays: number | null
  depletionDate: string | null
}

export function calculateBalanceDepletionForecast(
  balance: number,
  dailyAverage: number,
  baseDate: string
): BalanceDepletionForecast {
  const availableBalance = Number.isFinite(balance) ? balance : 0
  if (availableBalance <= 0) {
    return { status: 'depleted', remainingDays: 0, depletionDate: null }
  }

  const dailySpend = Number.isFinite(dailyAverage) ? dailyAverage : 0
  if (dailySpend <= 0) {
    return { status: 'no-consumption', remainingDays: null, depletionDate: null }
  }

  const rawRemainingDays = availableBalance / dailySpend
  const remainingDays = Math.ceil(rawRemainingDays)
  const depletionDate = addCalendarDays(baseDate, remainingDays)

  return {
    status: rawRemainingDays < 1 ? 'less-than-day' : 'estimated',
    remainingDays,
    depletionDate
  }
}

function addCalendarDays(baseDate: string, days: number): string | null {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(baseDate)) return null
  const date = new Date(`${baseDate}T00:00:00Z`)
  if (Number.isNaN(date.getTime())) return null
  date.setUTCDate(date.getUTCDate() + days)
  if (Number.isNaN(date.getTime())) return null
  return date.toISOString().slice(0, 10)
}
