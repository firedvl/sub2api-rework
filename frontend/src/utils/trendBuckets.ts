import type { TrendDataPoint } from '@/types'

/**
 * Helpers that turn the sparse dashboard trend (only periods with traffic are
 * returned) into a complete, evenly spaced series for charts.
 */

export type TrendTotals = Pick<
  TrendDataPoint,
  | 'requests'
  | 'input_tokens'
  | 'output_tokens'
  | 'cache_creation_tokens'
  | 'cache_read_tokens'
  | 'total_tokens'
  | 'cost'
  | 'actual_cost'
>

export interface TrendBucket extends TrendTotals {
  start: Date
}

const FIELDS: Array<keyof TrendTotals> = [
  'requests',
  'input_tokens',
  'output_tokens',
  'cache_creation_tokens',
  'cache_read_tokens',
  'total_tokens',
  'cost',
  'actual_cost',
]

export const HOUR_MS = 3_600_000

export const emptyTotals = (): TrendTotals => ({
  requests: 0,
  input_tokens: 0,
  output_tokens: 0,
  cache_creation_tokens: 0,
  cache_read_tokens: 0,
  total_tokens: 0,
  cost: 0,
  actual_cost: 0,
})

/** The API sends `YYYY-MM-DD` or `YYYY-MM-DD HH:00` in the viewer's time zone. */
export function parseTrendDate(value: string): Date {
  const match = /^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2}))?$/.exec(value.trim())
  if (match) {
    return new Date(Number(match[1]), Number(match[2]) - 1, Number(match[3]), Number(match[4] ?? 0), Number(match[5] ?? 0))
  }
  return new Date(value)
}

export function localDateString(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

export function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate())
}

export function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days, date.getHours())
}

export function addHours(date: Date, hours: number): Date {
  return new Date(date.getTime() + hours * HOUR_MS)
}

/** `count` slot starts, `stepHours` apart, beginning at `first`. A 24-hour step follows the calendar. */
export function slotStarts(first: Date, count: number, stepHours: number): Date[] {
  return Array.from({ length: count }, (_, index) => (
    stepHours % 24 === 0 ? addDays(first, (index * stepHours) / 24) : addHours(first, index * stepHours)
  ))
}

/** Sums trend points into the slot that contains them. Points outside every slot are dropped. */
export function bucketTrend(points: TrendDataPoint[], starts: Date[], end: Date): TrendBucket[] {
  const buckets = starts.map((start): TrendBucket => ({ start, ...emptyTotals() }))
  const endTime = end.getTime()
  for (const point of points) {
    const time = parseTrendDate(point.date).getTime()
    if (Number.isNaN(time) || time >= endTime) continue
    let index = -1
    for (let slot = starts.length - 1; slot >= 0; slot -= 1) {
      if (starts[slot].getTime() <= time) {
        index = slot
        break
      }
    }
    if (index === -1) continue
    for (const field of FIELDS) buckets[index][field] += Number(point[field]) || 0
  }
  return buckets
}

export function sumBuckets(buckets: TrendTotals[]): TrendTotals {
  const totals = emptyTotals()
  for (const bucket of buckets) {
    for (const field of FIELDS) totals[field] += bucket[field]
  }
  return totals
}
