import type { Account, AccountPlatform, AccountUsageInfo, UsageProgress } from '@/types'
import { getActiveModelRateLimits } from '@/utils/modelRateLimits'

export interface OperatorCapacityWindow {
  key: string
  label: string
  remainingPercent: number
  resetsAt: string | null
  kind: 'provider' | 'spend'
}

export type OperatorAccountHealth =
  | 'healthy'
  | 'rate_limited'
  | 'overloaded'
  | 'paused'
  | 'unschedulable'
  | 'inactive'
  | 'error'

export type OperatorAccountStatus = 'active' | 'limited' | 'error' | 'disabled'

export interface OperatorAccountStatusInfo {
  status: OperatorAccountStatus
  reason: string | null
  until: string | null
}

export interface OperatorAccountCapacity {
  account: Account
  identity: string
  tier: string
  health: OperatorAccountHealth
  groups: string[]
  windows: OperatorCapacityWindow[]
  lowestRemaining: number | null
  error: string | null
}

export interface OperatorProviderCapacity {
  platform: AccountPlatform
  accounts: OperatorAccountCapacity[]
  remainingPercent: number | null
  knownCount: number
  unknownCount: number
  lowestRemaining: number | null
  lowestAccount: OperatorAccountCapacity | null
  schedulableCount: number
  nextReset: string | null
}

export interface OperatorNormalizedCapacityAggregate {
  remainingPercent: number | null
  knownCount: number
  unknownCount: number
  lowestRemaining: number | null
  lowestAccount: OperatorAccountCapacity | null
  schedulableCount: number
  nextReset: string | null
}

export interface OperatorPoolCapacitySegment {
  summary: OperatorAccountCapacity & { lowestRemaining: number }
  contributionPercent: number
}

export interface OperatorPoolCapacity extends OperatorNormalizedCapacityAggregate {
  usedPercent: number | null
  segments: OperatorPoolCapacitySegment[]
  unknownAccounts: OperatorAccountCapacity[]
}

export interface OperatorWindowCapacitySegment {
  summary: OperatorAccountCapacity
  remainingPercent: number
  contributionPercent: number
}

export interface OperatorWindowCapacity {
  key: string
  label: string
  kind: OperatorCapacityWindow['kind']
  remainingPercent: number | null
  usedPercent: number | null
  knownCount: number
  unknownCount: number
  nextReset: string | null
  segments: OperatorWindowCapacitySegment[]
  unknownAccounts: OperatorAccountCapacity[]
}

const PLATFORM_ORDER: AccountPlatform[] = [
  'openai',
  'anthropic',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
]

const PROGRESS_WINDOWS: Array<[keyof AccountUsageInfo, string]> = [
  ['five_hour', '5h'],
  ['seven_day', '7d'],
  ['seven_day_sonnet', 'Sonnet 7d'],
  ['seven_day_fable', 'Fable 7d'],
  ['thirty_day', '30d'],
  ['gemini_shared_daily', 'Daily'],
  ['gemini_pro_daily', 'Pro daily'],
  ['gemini_flash_daily', 'Flash daily'],
  ['gemini_shared_minute', 'Minute'],
  ['gemini_pro_minute', 'Pro minute'],
  ['gemini_flash_minute', 'Flash minute'],
]

function finiteNumber(value: unknown): number | null {
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

function clampPercent(value: number): number {
  return Math.min(100, Math.max(0, value))
}

function remainingFromUsed(value: unknown): number | null {
  const used = finiteNumber(value)
  return used === null ? null : clampPercent(100 - used)
}

function remainingFromLimit(limitValue: unknown, remainingValue: unknown): number | null {
  const limit = finiteNumber(limitValue)
  const remaining = finiteNumber(remainingValue)
  if (limit === null || remaining === null || limit <= 0) return null
  return clampPercent((remaining / limit) * 100)
}

function remainingFromSpend(limitValue: unknown, usedValue: unknown): number | null {
  const limit = finiteNumber(limitValue)
  const used = finiteNumber(usedValue)
  if (limit === null || used === null || limit <= 0) return null
  return clampPercent(((limit - used) / limit) * 100)
}

function futureReset(secondsValue: unknown, now: Date): string | null {
  const seconds = finiteNumber(secondsValue)
  return seconds === null || seconds < 0
    ? null
    : new Date(now.getTime() + seconds * 1000).toISOString()
}

function quotaReset(resetAt: unknown, resetUnix: unknown): string | null {
  if (typeof resetAt === 'string' && resetAt.trim()) return resetAt
  const unix = finiteNumber(resetUnix)
  return unix === null ? null : new Date(unix * 1000).toISOString()
}

function stringValue(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  return ''
}

function isFuture(value: string | null | undefined, now: Date): boolean {
  if (!value) return false
  const timestamp = new Date(value).getTime()
  return Number.isFinite(timestamp) && timestamp > now.getTime()
}

function accountHealth(account: Account, now: Date): OperatorAccountHealth {
  if (account.status === 'error') return 'error'
  if (account.status === 'inactive') return 'inactive'
  if (account.rate_limited_at && (!account.rate_limit_reset_at || isFuture(account.rate_limit_reset_at, now))) {
    return 'rate_limited'
  }
  if (isFuture(account.overload_until, now)) return 'overloaded'
  if (isFuture(account.temp_unschedulable_until, now)) return 'paused'
  if (account.session_window_status === 'rejected') return 'paused'
  if (getActiveModelRateLimits(account.extra, now).length) return 'rate_limited'
  if (!account.schedulable) return 'unschedulable'
  return 'healthy'
}

const AUTH_FAILURE_PATTERN = /\b(?:401|403|unauthenticated|unauthorized|invalid_grant|forbidden|reauthori[sz](?:e|ation)|authentication failed)\b/i
const RATE_LIMIT_PATTERN = /\b(?:429|rate[\s_-]?limit(?:ed)?|too many requests)\b/i

function usageHealth(
  usage: AccountUsageInfo | null | undefined,
  error: string | null,
): { health: 'rate_limited' | 'error'; reason: string } | null {
  const errorCode = usage?.error_code?.toLowerCase()
  if (usage?.needs_verify) {
    return { health: 'error', reason: usage?.forbidden_reason?.trim() || 'Account verification required' }
  }
  if (usage?.is_banned) {
    return { health: 'error', reason: usage?.forbidden_reason?.trim() || 'Provider account banned' }
  }
  if (usage?.needs_reauth || errorCode === 'unauthenticated') {
    return { health: 'error', reason: 'Authentication failed' }
  }
  if (usage?.is_forbidden || errorCode === 'forbidden') {
    return { health: 'error', reason: usage?.forbidden_reason?.trim() || 'Provider access forbidden' }
  }
  if (error && AUTH_FAILURE_PATTERN.test(error)) return { health: 'error', reason: error }
  if (errorCode === 'rate_limited' || (error && RATE_LIMIT_PATTERN.test(error))) {
    return { health: 'rate_limited', reason: 'Provider rate limit' }
  }
  return null
}

function addWindow(
  windows: OperatorCapacityWindow[],
  window: Omit<OperatorCapacityWindow, 'remainingPercent'> & { remainingPercent: number | null },
): void {
  if (window.remainingPercent === null || windows.some((item) => item.key === window.key)) return
  windows.push({ ...window, remainingPercent: window.remainingPercent })
}

function addProgressWindow(
  windows: OperatorCapacityWindow[],
  key: string,
  label: string,
  progress: UsageProgress | null | undefined,
): void {
  if (!progress) return
  addWindow(windows, {
    key,
    label,
    remainingPercent: remainingFromUsed(progress.utilization),
    resetsAt: progress.resets_at,
    kind: 'provider',
  })
}

export function supportsBatchAccountUsage(account: Account): boolean {
  if (account.platform === 'anthropic') {
    return account.type === 'oauth' || account.type === 'setup-token'
  }
  if (account.platform === 'gemini') return true
  if (account.platform === 'antigravity') return account.type === 'oauth'
  if (account.platform === 'openai') return account.type === 'oauth'
  if (account.platform === 'grok') return account.type === 'oauth'
  return false
}

export function normalizeAccountCapacity(
  account: Account,
  usage?: AccountUsageInfo | null,
  error?: string | null,
  now = new Date(),
): OperatorAccountCapacity {
  const windows: OperatorCapacityWindow[] = []

  if (usage) {
    for (const [key, label] of PROGRESS_WINDOWS) {
      addProgressWindow(windows, String(key), label, usage[key] as UsageProgress | null | undefined)
    }

    for (const [model, quota] of Object.entries(usage.antigravity_quota ?? {})) {
      addWindow(windows, {
        key: `antigravity:${model}`,
        label: model,
        remainingPercent: remainingFromUsed(quota.utilization),
        resetsAt: quota.reset_time,
        kind: 'provider',
      })
    }

    addWindow(windows, {
      key: 'grok:requests',
      label: 'Requests',
      remainingPercent: remainingFromLimit(
        usage.grok_request_quota?.limit,
        usage.grok_request_quota?.remaining,
      ),
      resetsAt: quotaReset(
        usage.grok_request_quota?.reset_at,
        usage.grok_request_quota?.reset_unix,
      ),
      kind: 'provider',
    })
    addWindow(windows, {
      key: 'grok:tokens',
      label: 'Tokens',
      remainingPercent: remainingFromLimit(
        usage.grok_token_quota?.limit,
        usage.grok_token_quota?.remaining,
      ),
      resetsAt: quotaReset(
        usage.grok_token_quota?.reset_at,
        usage.grok_token_quota?.reset_unix,
      ),
      kind: 'provider',
    })
    addWindow(windows, {
      key: 'grok:monthly-spend',
      label: '$ monthly',
      remainingPercent: remainingFromSpend(
        usage.grok_billing?.monthly_limit,
        usage.grok_billing?.monthly_used,
      ),
      resetsAt: usage.grok_billing?.period_end ?? null,
      kind: 'spend',
    })
  }

  const extra = account.extra ?? {}
  addWindow(windows, {
    key: 'five_hour',
    label: '5h',
    remainingPercent: remainingFromUsed(extra.codex_5h_used_percent),
    resetsAt: stringValue(extra.codex_5h_reset_at) || futureReset(extra.codex_5h_reset_after_seconds, now),
    kind: 'provider',
  })
  addWindow(windows, {
    key: 'seven_day',
    label: '7d',
    remainingPercent: remainingFromUsed(extra.codex_7d_used_percent),
    resetsAt: stringValue(extra.codex_7d_reset_at) || futureReset(extra.codex_7d_reset_after_seconds, now),
    kind: 'provider',
  })

  const spendLimits: Array<[string, string, unknown, unknown, string | null | undefined]> = [
    ['spend:daily', '$ daily', account.quota_daily_limit, account.quota_daily_used, account.quota_daily_reset_at],
    ['spend:weekly', '$ weekly', account.quota_weekly_limit, account.quota_weekly_used, account.quota_weekly_reset_at],
    ['spend:total', '$ total', account.quota_limit, account.quota_used, null],
  ]
  for (const [key, label, limit, used, resetsAt] of spendLimits) {
    addWindow(windows, {
      key,
      label,
      remainingPercent: remainingFromSpend(limit, used),
      resetsAt: resetsAt ?? null,
      kind: 'spend',
    })
  }

  const remainingValues = windows.map((window) => window.remainingPercent)
  const credentials = account.credentials ?? {}
  const usageError = error?.trim() || usage?.forbidden_reason?.trim() || usage?.error?.trim() || null
  const baseHealth = accountHealth(account, now)
  const derivedUsageHealth = usageHealth(usage, usageError)
  let health = baseHealth
  if (derivedUsageHealth?.health === 'error') {
    health = 'error'
  } else if (
    derivedUsageHealth?.health === 'rate_limited'
    && (baseHealth === 'healthy' || baseHealth === 'unschedulable')
  ) {
    health = 'rate_limited'
  }

  return {
    account,
    identity: stringValue(
      extra.email_address,
      extra.email,
      credentials.email,
      account.parent_email,
    ),
    tier: stringValue(
      usage?.subscription_tier,
      usage?.subscription_tier_raw,
      credentials.subscription_plan_type,
      credentials.plan_type,
      credentials.tier_id,
      extra.plan_type,
    ),
    health,
    groups: (account.groups ?? []).map((group) => group.name).filter(Boolean),
    windows,
    lowestRemaining: remainingValues.length ? Math.min(...remainingValues) : null,
    error: derivedUsageHealth?.reason ?? usageError,
  }
}

export function classifyOperatorAccount(
  summary: OperatorAccountCapacity,
): OperatorAccountStatusInfo {
  const { account, health, windows } = summary

  if (health === 'error') {
    return {
      status: 'error',
      reason: account.error_message?.trim() || summary.error?.trim() || 'Account error',
      until: null,
    }
  }
  if (health === 'inactive') {
    return { status: 'disabled', reason: 'Account disabled', until: null }
  }
  if (health === 'rate_limited') {
    return { status: 'limited', reason: 'Provider rate limit', until: account.rate_limit_reset_at }
  }
  if (health === 'overloaded') {
    return { status: 'limited', reason: 'Provider cooldown', until: account.overload_until }
  }
  if (health === 'paused') {
    return {
      status: 'limited',
      reason: account.temp_unschedulable_reason?.trim() || 'Temporarily unavailable',
      until: account.temp_unschedulable_until,
    }
  }

  const exhaustedWindow = windows.find((window) => window.remainingPercent === 0)
  if (exhaustedWindow) {
    return {
      status: 'limited',
      reason: `${exhaustedWindow.label} quota exhausted`,
      until: exhaustedWindow.resetsAt,
    }
  }
  if (health === 'unschedulable') {
    return { status: 'disabled', reason: 'Scheduling disabled', until: null }
  }

  return { status: 'active', reason: null, until: null }
}

function serviceableCapacityAccounts(accounts: OperatorAccountCapacity[]): OperatorAccountCapacity[] {
  return accounts.filter((summary) => {
    const status = classifyOperatorAccount(summary).status
    return status === 'active' || status === 'limited'
  })
}

function compactPercent(value: number): string {
  return value.toFixed(1).replace(/\.0$/, '')
}

export function summarizeOperatorQuota(summary: OperatorAccountCapacity): string {
  const { platform } = summary.account
  let windows = summary.windows

  if (platform === 'openai') {
    const primaryWindows = ['five_hour', 'seven_day']
      .flatMap((key) => windows.filter((window) => window.key === key))
    if (primaryWindows.length) windows = primaryWindows
  }

  const modelWindows = windows.filter((window) => window.key.startsWith('antigravity:'))
  if (platform === 'antigravity' && modelWindows.length > 1) {
    const minimum = Math.min(...modelWindows.map((window) => window.remainingPercent))
    return `${compactPercent(minimum)}% minimum across ${modelWindows.length} models`
  }

  if (!windows.length) return 'Quota unavailable'

  const visible = windows.slice(0, 2)
  const summaryText = visible
    .map((window) => `${window.label} ${compactPercent(window.remainingPercent)}%`)
    .join(' · ')
  return windows.length > visible.length
    ? `${summaryText} · +${windows.length - visible.length} more`
    : summaryText
}

export function buildProviderCapacity(
  accounts: Account[],
  usageByAccountId: Record<string, AccountUsageInfo | null | undefined>,
  errorsByAccountId: Record<string, string | null | undefined> = {},
  now = new Date(),
): OperatorProviderCapacity[] {
  const grouped = new Map<AccountPlatform, OperatorAccountCapacity[]>()

  for (const account of accounts) {
    const key = String(account.id)
    const summary = normalizeAccountCapacity(
      account,
      usageByAccountId[key],
      errorsByAccountId[key],
      now,
    )
    const providerAccounts = grouped.get(account.platform) ?? []
    providerAccounts.push(summary)
    grouped.set(account.platform, providerAccounts)
  }

  return Array.from(grouped.entries())
    .map(([platform, providerAccounts]) => {
      providerAccounts.sort((left, right) => {
        if (left.lowestRemaining === null) return right.lowestRemaining === null
          ? left.account.name.localeCompare(right.account.name)
          : 1
        if (right.lowestRemaining === null) return -1
        return left.lowestRemaining - right.lowestRemaining
          || left.account.name.localeCompare(right.account.name)
      })
      return {
        platform,
        accounts: providerAccounts,
        ...aggregateNormalizedCapacity(providerAccounts, now),
      }
    })
    .sort((left, right) => PLATFORM_ORDER.indexOf(left.platform) - PLATFORM_ORDER.indexOf(right.platform))
}

export function aggregateNormalizedCapacity(
  accounts: OperatorAccountCapacity[],
  now = new Date(),
): OperatorNormalizedCapacityAggregate {
  const capacityAccounts = serviceableCapacityAccounts(accounts)
  const knownAccounts = capacityAccounts.filter(
    (summary): summary is OperatorAccountCapacity & { lowestRemaining: number } => (
      summary.lowestRemaining !== null
    ),
  )
  const lowestAccount = knownAccounts.reduce<OperatorAccountCapacity | null>(
    (lowest, summary) => (
      lowest === null || summary.lowestRemaining < (lowest.lowestRemaining as number)
        ? summary
        : lowest
    ),
    null,
  )
  const nextReset = knownAccounts
    .flatMap((summary) => summary.windows.filter((window) => (
      window.remainingPercent === summary.lowestRemaining && isFuture(window.resetsAt, now)
    )))
    .sort((left, right) => (
      new Date(left.resetsAt as string).getTime() - new Date(right.resetsAt as string).getTime()
    ))[0]?.resetsAt ?? null

  return {
    remainingPercent: knownAccounts.length
      ? knownAccounts.reduce((total, summary) => total + summary.lowestRemaining, 0) / knownAccounts.length
      : null,
    knownCount: knownAccounts.length,
    unknownCount: capacityAccounts.length - knownAccounts.length,
    lowestRemaining: lowestAccount?.lowestRemaining ?? null,
    lowestAccount,
    schedulableCount: capacityAccounts.filter((summary) => summary.account.schedulable).length,
    nextReset,
  }
}

export function buildNormalizedPoolCapacity(
  accounts: OperatorAccountCapacity[],
  now = new Date(),
): OperatorPoolCapacity {
  const aggregate = aggregateNormalizedCapacity(accounts, now)
  const capacityAccounts = serviceableCapacityAccounts(accounts)
  const knownAccounts = capacityAccounts.filter(
    (summary): summary is OperatorAccountCapacity & { lowestRemaining: number } => (
      summary.lowestRemaining !== null
    ),
  )
  const unknownAccounts = capacityAccounts.filter((summary) => summary.lowestRemaining === null)

  if (!knownAccounts.length) {
    return {
      ...aggregate,
      usedPercent: null,
      segments: [],
      unknownAccounts,
    }
  }

  const segments = knownAccounts.map((summary) => ({
    summary,
    contributionPercent: summary.lowestRemaining / knownAccounts.length,
  }))

  return {
    ...aggregate,
    usedPercent: 100 - (aggregate.remainingPercent as number),
    segments,
    unknownAccounts,
  }
}

export function buildWindowCapacities(
  accounts: OperatorAccountCapacity[],
  now = new Date(),
): OperatorWindowCapacity[] {
  const capacityAccounts = serviceableCapacityAccounts(accounts)
  const windowsByKey = new Map<string, { label: string; kind: OperatorCapacityWindow['kind'] }>()

  for (const summary of capacityAccounts) {
    for (const window of summary.windows) {
      if (!windowsByKey.has(window.key)) {
        windowsByKey.set(window.key, { label: window.label, kind: window.kind })
      }
    }
  }

  return Array.from(windowsByKey.entries()).map(([key, definition]) => {
    const known = capacityAccounts.flatMap((summary) => {
      const window = summary.windows.find((candidate) => candidate.key === key)
      return window ? [{ summary, window }] : []
    })
    const knownAccountIDs = new Set(known.map(({ summary }) => summary.account.id))
    const unknownAccounts = capacityAccounts.filter((summary) => !knownAccountIDs.has(summary.account.id))

    if (!known.length) {
      return {
        key,
        ...definition,
        remainingPercent: null,
        usedPercent: null,
        knownCount: 0,
        unknownCount: capacityAccounts.length,
        nextReset: null,
        segments: [],
        unknownAccounts,
      }
    }

    const remainingPercent = known.reduce(
      (total, { window }) => total + window.remainingPercent,
      0,
    ) / known.length
    const nextReset = known
      .map(({ window }) => window.resetsAt)
      .filter((value): value is string => isFuture(value, now))
      .sort((left, right) => new Date(left).getTime() - new Date(right).getTime())[0] ?? null

    return {
      key,
      ...definition,
      remainingPercent,
      usedPercent: 100 - remainingPercent,
      knownCount: known.length,
      unknownCount: unknownAccounts.length,
      nextReset,
      segments: known.map(({ summary, window }) => ({
        summary,
        remainingPercent: window.remainingPercent,
        contributionPercent: window.remainingPercent / known.length,
      })),
      unknownAccounts,
    }
  })
}
