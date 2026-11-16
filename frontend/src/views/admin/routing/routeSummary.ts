import type { AccountListItem, AdminGroup } from '@/types'

/** Account pool split for a group: disjoint available / limited / unavailable counts. */
export interface PoolSplit {
  total: number
  available: number
  limited: number
  unavailable: number
}

const count = (value: number | null | undefined) =>
  Number.isFinite(value) && (value as number) > 0 ? Math.floor(value as number) : 0

/**
 * The backend counts "available" and "temporarily limited" with exclusive
 * filters, so the remainder of the pool is unavailable (paused, errored,
 * expired, or unschedulable).
 */
export function poolSplit(group: Pick<AdminGroup, 'account_count' | 'active_account_count' | 'rate_limited_account_count'>): PoolSplit {
  const total = count(group.account_count)
  const available = Math.min(count(group.active_account_count), total)
  const limited = Math.min(count(group.rate_limited_account_count), total - available)
  return { total, available, limited, unavailable: total - available - limited }
}

export type Tone = 'ok' | 'warn' | 'bad' | 'idle'

/** Overall health of a pool, used for the status dot beside a route. */
export function poolTone(split: PoolSplit): Tone {
  if (split.total === 0) return 'idle'
  if (split.available === 0) return 'bad'
  if (split.limited > 0 || split.unavailable > 0) return 'warn'
  return 'ok'
}

const future = (value: string | null | undefined, now: number) => {
  if (!value) return false
  const time = Date.parse(value)
  return Number.isFinite(time) && time > now
}

/** Scheduling state of one member account, mirroring the backend pool filters. */
export function memberTone(account: Pick<AccountListItem, 'status' | 'schedulable' | 'rate_limit_reset_at' | 'overload_until' | 'temp_unschedulable_until'>, now = Date.now()): Tone {
  if (account.status === 'error') return 'bad'
  if (account.status !== 'active' || !account.schedulable) return 'idle'
  if (future(account.rate_limit_reset_at, now) || future(account.overload_until, now) || future(account.temp_unschedulable_until, now)) return 'warn'
  return 'ok'
}

/** Models a group explicitly serves; an empty list means the platform default catalog. */
export function servedModels(group: Pick<AdminGroup, 'model_routing' | 'model_routing_enabled' | 'models_list_config' | 'model_allowlist' | 'default_mapped_model'>): string[] {
  const models = new Set<string>()
  if (group.model_allowlist?.enabled) group.model_allowlist.models.forEach((m) => models.add(m))
  if (group.models_list_config?.enabled) group.models_list_config.models.forEach((m) => models.add(m))
  if (group.model_routing_enabled && group.model_routing) Object.keys(group.model_routing).forEach((m) => models.add(m))
  if (group.default_mapped_model) models.add(group.default_mapped_model)
  return [...models].filter(Boolean).sort((a, b) => a.localeCompare(b))
}

/** Share of a spend limit already used today, as a 0-100 percentage of what is left. */
export function remainingPercent(used: number | null | undefined, limit: number | null | undefined): number | null {
  if (!limit || limit <= 0) return null
  const ratio = Math.max(0, Math.min(1, (used ?? 0) / limit))
  return Math.round((1 - ratio) * 100)
}
