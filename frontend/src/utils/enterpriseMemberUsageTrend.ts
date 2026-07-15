export interface EnterpriseMemberDailyUsagePoint {
  date: string
  request_count: number
  input_tokens: number
  output_tokens: number
  actual_cost: number
}

const DAY_MS = 24 * 60 * 60 * 1000

function dateKeyAtTimezone(value: string, timezone: string): string | null {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return null

  try {
    const parts = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit'
    }).formatToParts(date)
    const values = Object.fromEntries(parts.map(part => [part.type, part.value]))
    if (!values.year || !values.month || !values.day) return null
    return `${values.year}-${values.month}-${values.day}`
  } catch {
    return null
  }
}

function addCalendarDays(dateKey: string, offset: number): string {
  const [year, month, day] = dateKey.split('-').map(Number)
  const date = new Date(Date.UTC(year, month - 1, day) + offset * DAY_MS)
  return date.toISOString().slice(0, 10)
}

export function fillEnterpriseMemberUsageTrend(
  points: EnterpriseMemberDailyUsagePoint[],
  start: string,
  days: number,
  timezone: string
): EnterpriseMemberDailyUsagePoint[] {
  const startDateKey = dateKeyAtTimezone(start, timezone)
  if (!startDateKey || !Number.isInteger(days) || days < 1 || days > 365) return [...points]

  const byDate = new Map(points.map(point => [point.date, point]))
  return Array.from({ length: days }, (_, index) => {
    const date = addCalendarDays(startDateKey, index)
    return byDate.get(date) || {
      date,
      request_count: 0,
      input_tokens: 0,
      output_tokens: 0,
      actual_cost: 0
    }
  })
}
