import type { Account, AccountPlatform, AccountType, AccountUsageInfo, WindowStats } from '@/types'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import {
  classifyOperatorAccount,
  normalizeAccountCapacity,
  type OperatorAccountCapacity,
  type OperatorAccountStatus,
  type OperatorCapacityWindow,
} from '@/utils/operatorCapacity'

/** One account as the fleet table shows it: capacity summary plus the two headline windows. */
export interface FleetRow {
  account: Account
  summary: OperatorAccountCapacity
  status: OperatorAccountStatus
  reason: string | null
  until: string | null
  short: OperatorCapacityWindow | null
  long: OperatorCapacityWindow | null
  today: WindowStats | null
}

export type FleetSortKey = 'capacity' | 'name' | 'status' | 'short' | 'long' | 'today' | 'last_used'
export interface FleetSort {
  key: FleetSortKey
  dir: 1 | -1
}

export const OPERATOR_STATUSES: OperatorAccountStatus[] = ['active', 'limited', 'error', 'disabled']

/** Literal class names so Tailwind keeps them. */
export const STATUS_BADGE: Record<OperatorAccountStatus, string> = {
  active: 'badge-success',
  limited: 'badge-warning',
  error: 'badge-danger',
  disabled: 'badge-gray',
}

export const STATUS_DOT: Record<OperatorAccountStatus, string> = {
  active: 'is-ok',
  limited: 'is-warn',
  error: 'is-bad',
  disabled: '',
}

const STATUS_ORDER: Record<OperatorAccountStatus, number> = { error: 0, limited: 1, disabled: 2, active: 3 }

const SHORT_KEYS = ['five_hour', 'gemini_shared_daily', 'gemini_pro_daily', 'gemini_flash_daily', 'grok:requests', 'spend:daily']
const LONG_KEYS = ['seven_day', 'thirty_day', 'seven_day_sonnet', 'seven_day_fable', 'grok:monthly-spend', 'spend:weekly', 'spend:total']

/** Pick the short (session/daily) and long (weekly/monthly) headline windows. */
export function pickHeadlineWindows(windows: OperatorCapacityWindow[]): {
  short: OperatorCapacityWindow | null
  long: OperatorCapacityWindow | null
} {
  const current = windows.filter((window) => window.current !== false)
  const pool = current.length ? current : windows
  const find = (keys: string[]) => keys
    .map((key) => pool.find((window) => window.key === key))
    .find((window): window is OperatorCapacityWindow => Boolean(window)) ?? null
  const long = find(LONG_KEYS)
  let short = find(SHORT_KEYS)
  if (!short) {
    const models = pool.filter((window) => window.key.startsWith('antigravity:'))
    short = models.length
      ? models.reduce((lowest, window) => (window.remainingPercent < lowest.remainingPercent ? window : lowest))
      : pool.find((window) => window !== long) ?? null
  }
  return { short, long }
}

export function buildFleetRow(
  account: Account,
  usage: AccountUsageInfo | null | undefined,
  error: string | null | undefined,
  today: WindowStats | null | undefined,
  now: Date,
): FleetRow {
  const summary = normalizeAccountCapacity(account, usage, error, now)
  const info = classifyOperatorAccount(summary)
  const { short, long } = pickHeadlineWindows(summary.windows)
  return {
    account,
    summary,
    status: info.status,
    reason: info.reason,
    until: info.until,
    short,
    long,
    today: today ?? null,
  }
}

const timeOf = (value: string | null | undefined) => {
  const time = value ? new Date(value).getTime() : Number.NaN
  return Number.isFinite(time) ? time : null
}

/** Compare with nulls always last, whatever the direction. */
function compareNullable(left: number | null, right: number | null, dir: 1 | -1): number {
  if (left === null) return right === null ? 0 : 1
  if (right === null) return -1
  return (left - right) * dir
}

export function sortFleetRows(rows: FleetRow[], sort: FleetSort): FleetRow[] {
  const byName = (left: FleetRow, right: FleetRow) => left.account.name.localeCompare(right.account.name)
  const value = (row: FleetRow): number | null => {
    switch (sort.key) {
      case 'capacity': return row.summary.lowestRemaining
      case 'short': return row.short?.remainingPercent ?? null
      case 'long': return row.long?.remainingPercent ?? null
      case 'today': return row.today?.requests ?? null
      case 'last_used': return timeOf(row.account.last_used_at)
      default: return null
    }
  }
  return rows.slice().sort((left, right) => {
    if (sort.key === 'name') return byName(left, right) * sort.dir
    if (sort.key === 'status') {
      return (STATUS_ORDER[left.status] - STATUS_ORDER[right.status]) * sort.dir || byName(left, right)
    }
    return compareNullable(value(left), value(right), sort.dir) || byName(left, right)
  })
}

const PLATFORM_LABELS = new Map<string, string>(CONCRETE_PLATFORM_OPTIONS.map((option) => [option.value, option.label]))
export const PLATFORM_ORDER: string[] = CONCRETE_PLATFORM_OPTIONS.map((option) => option.value)

export const providerLabel = (platform: AccountPlatform | string) => PLATFORM_LABELS.get(platform) ?? platform

const TYPE_LABELS: Record<AccountType, string> = {
  oauth: 'OAuth',
  'setup-token': 'Setup token',
  apikey: 'API key',
  upstream: 'Upstream',
  bedrock: 'Bedrock',
  service_account: 'Service account',
}
export const accountTypeLabel = (type: AccountType) => TYPE_LABELS[type] ?? type

/** Signature of what the table shows for a row; a change triggers the row flash. */
export const fleetRowSignature = (row: FleetRow) => [
  row.status,
  row.account.name,
  row.short ? Math.round(row.short.remainingPercent) : '',
  row.long ? Math.round(row.long.remainingPercent) : '',
  row.today?.requests ?? '',
  row.account.last_used_at ?? '',
].join('|')

export const formatPercent = (value: number) => value.toFixed(1).replace(/\.0$/, '')
