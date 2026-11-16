/**
 * Shared presentation helpers for admin usage records (table row + drawer).
 */
import type { AdminUsageLog } from '@/types'
import { reasoningEffortValuesEqual } from '@/utils/format'
import { resolveUsageRequestType } from '@/utils/usageRequestType'
import { durationSeverity, firstTokenSeverity, type LatencySeverity } from '@/utils/latencyHealth'
import { providerOfModel } from '@/utils/providerSeries'

type Translate = (key: string, values?: Record<string, unknown>) => string

/** Literal class names so Tailwind keeps them. */
export const DOT_CLASS: Record<LatencySeverity, string> = {
  good: 'is-ok',
  warn: 'is-warn',
  slow: 'is-warn',
  critical: 'is-bad',
}

export const LATENCY_TONE_CLASS: Record<LatencySeverity, string> = {
  good: 'text-green-600 dark:text-green-400',
  warn: 'text-amber-600 dark:text-amber-400',
  slow: 'text-orange-600 dark:text-orange-400',
  critical: 'text-red-600 dark:text-red-400',
}

const SEVERITY_RANK: Record<LatencySeverity, number> = { good: 0, warn: 1, slow: 2, critical: 3 }

/** Worst of first-token and total-duration health. */
export function latencySeverity(row: Pick<AdminUsageLog, 'first_token_ms' | 'duration_ms'>): LatencySeverity {
  const total = durationSeverity(row.duration_ms ?? 0)
  if (row.first_token_ms == null) return total
  const first = firstTokenSeverity(row.first_token_ms)
  return SEVERITY_RANK[first] > SEVERITY_RANK[total] ? first : total
}

/** Platform for the logo: the group's platform when known, else inferred from the model name. */
export function platformOf(row: AdminUsageLog): string {
  const groupPlatform = row.group?.platform
  if (groupPlatform && groupPlatform !== 'composite') return groupPlatform
  return providerOfModel(row.upstream_model || row.model || '')
}

export function sentUpstreamModel(row: AdminUsageLog): string {
  return row.upstream_model?.trim() || row.model?.trim() || ''
}

/** The final routed model when it differs from the requested one. */
export function routedModel(row: AdminUsageLog): string {
  const chain = row.model_mapping_chain
  if (chain && chain.includes('→')) {
    const steps = chain.split('→').map((step) => step.trim()).filter(Boolean)
    return steps[steps.length - 1] || ''
  }
  if (row.upstream_model && row.upstream_model !== row.model) return row.upstream_model
  return ''
}

export function mappingSteps(row: AdminUsageLog): string[] {
  const chain = row.model_mapping_chain
  if (!chain || !chain.includes('→')) return []
  return chain.split('→').map((step) => step.trim()).filter(Boolean)
}

const normalizeModelVariant = (model: string): string => model
  .trim()
  .toLowerCase()
  .replace(/-latest$/, '')
  .replace(/-\d{4}-\d{2}-\d{2}$/, '')
  .replace(/-\d{8}$/, '')

export function isLikelyModelVariant(row: AdminUsageLog): boolean {
  const sent = sentUpstreamModel(row)
  const response = row.upstream_response_model?.trim() || ''
  return sent !== '' && response !== '' && normalizeModelVariant(sent) === normalizeModelVariant(response)
}

export function hasReasoningEffortMapping(row: AdminUsageLog): boolean {
  const requested = row.reasoning_effort?.trim() || ''
  const forwarded = row.upstream_reasoning_effort?.trim() || ''
  return requested !== '' && forwarded !== '' && !reasoningEffortValuesEqual(requested, forwarded)
}

export function requestTypeBadge(row: AdminUsageLog): string {
  const type = resolveUsageRequestType(row)
  if (type === 'cyber') return 'badge-danger'
  if (type === 'live') return 'badge-success'
  if (type === 'ws_v2' || type === 'stream') return 'badge-primary'
  if (type === 'sync') return 'badge-gray'
  return 'badge-warning'
}

export function totalTokens(row: AdminUsageLog): number {
  return (row.input_tokens || 0) + (row.output_tokens || 0) + (row.cache_creation_tokens || 0) + (row.cache_read_tokens || 0)
}

/** (account_stats_cost ?? total_cost) * account_rate_multiplier */
export function accountBilled(row: Pick<AdminUsageLog, 'total_cost' | 'account_stats_cost' | 'account_rate_multiplier'>): number {
  const base = row.account_stats_cost != null ? row.account_stats_cost : (row.total_cost ?? 0)
  const result = base * (row.account_rate_multiplier ?? 1)
  return Number.isNaN(result) ? 0 : result
}

export function recordId(row: AdminUsageLog, key: string): string {
  if (key === 'request_id') return row.request_id || ''
  if (key === 'upstream_request_id') return row.upstream_request_id || ''
  return ''
}

export function formatCompact(value: number | null | undefined): string {
  const n = value || 0
  if (n >= 1e9) return (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return (n / 1e6).toFixed(2) + 'M'
  if (n >= 1e4) return (n / 1e3).toFixed(1) + 'K'
  return n.toLocaleString()
}

export function formatCost(value: number | null | undefined): string {
  const n = value || 0
  if (n === 0) return '$0'
  return '$' + (n >= 1 ? n.toFixed(4) : n.toFixed(6))
}

/** Over a minute becomes "Xm Ys"; over an hour "Xh Ym". */
export function formatDuration(ms: number | null | undefined): string {
  if (ms == null) return '-'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)}s`
  const totalSec = Math.round(ms / 1000)
  if (totalSec < 3600) return `${Math.floor(totalSec / 60)}m ${totalSec % 60}s`
  return `${Math.floor(totalSec / 3600)}h ${Math.floor((totalSec % 3600) / 60)}m`
}

export function useRecordLabels(t: Translate) {
  const requestTypeLabel = (row: AdminUsageLog): string => {
    const type = resolveUsageRequestType(row)
    if (type === 'cyber') return t('usage.cyber')
    if (type === 'live') return t('usage.live')
    if (type === 'ws_v2') return t('usage.ws')
    if (type === 'stream') return t('usage.stream')
    if (type === 'sync') return t('usage.sync')
    return t('usage.unknown')
  }

  const modelAuditTitle = (row: AdminUsageLog): string => [
    `${t('usage.requestedModel')}: ${row.model || '-'}`,
    `${t('usage.sentUpstreamModel')}: ${sentUpstreamModel(row) || '-'}`,
    `${t('usage.upstreamResponseModel')}: ${row.upstream_response_model || '-'}`,
  ].join('\n')

  return { requestTypeLabel, modelAuditTitle }
}
