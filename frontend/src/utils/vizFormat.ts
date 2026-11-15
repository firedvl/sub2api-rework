/** Number formatting shared by the Overview and Stats visualizations. */

const trim = (value: number): string =>
  value >= 100 ? value.toFixed(0) : value >= 10 ? value.toFixed(1).replace(/\.0$/, '') : value.toFixed(2).replace(/\.?0+$/, '')

const finite = (value: unknown): number => {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

export const formatInt = (value: unknown): string => Math.round(finite(value)).toLocaleString('en-US')

export function formatCompact(value: unknown): string {
  const number = finite(value)
  const abs = Math.abs(number)
  if (abs >= 1e9) return `${trim(number / 1e9)}B`
  if (abs >= 1e6) return `${trim(number / 1e6)}M`
  if (abs >= 1e4) return `${trim(number / 1e3)}K`
  if (abs >= 1000) return formatInt(number)
  return String(Math.round(number * 10) / 10)
}

export function formatMoney(value: unknown): string {
  const number = finite(value)
  return `$${number >= 1000 ? formatInt(number) : number.toFixed(2)}`
}

export function formatDuration(milliseconds: unknown): string {
  const number = finite(milliseconds)
  return number >= 1000 ? `${(number / 1000).toFixed(2)}s` : `${Math.round(number)}ms`
}

export const formatPercent = (fraction: unknown, digits = 0): string => `${(finite(fraction) * 100).toFixed(digits)}%`

/** Axis ticks keep one style per axis: full numbers below 10,000, compact above. */
export function formatAxisNumber(value: number, axisMax: number): string {
  if (axisMax < 1e4) return formatInt(value)
  if (value === 0) return '0'
  if (value >= 1e9) return `${trim(value / 1e9)}B`
  if (value >= 1e6) return `${trim(value / 1e6)}M`
  return `${trim(value / 1e3)}K`
}

export function formatAxisMoney(value: number): string {
  if (value >= 1000) return `$${formatCompact(value)}`
  return `$${Number.isInteger(value) ? value : value.toFixed(2)}`
}

/** A rounded axis maximum and its ticks for values from zero to `maxValue`. */
export function niceScale(maxValue: number): { max: number; ticks: number[] } {
  const value = Math.max(maxValue, 1e-9)
  const raw = value / 4
  const power = Math.pow(10, Math.floor(Math.log10(raw)))
  const step = ([1, 2, 2.5, 5, 10].find((multiple) => multiple * power >= raw) ?? 10) * power
  const max = Math.ceil(value / step) * step
  const ticks: number[] = []
  for (let tick = 0; tick <= max + step / 2; tick += step) ticks.push(tick)
  return { max, ticks }
}

/** Signed relative change, or null when there is no baseline. */
export function relativeChange(current: number, previous: number): number | null {
  if (!Number.isFinite(previous) || previous === 0) return null
  return (current - previous) / previous
}
