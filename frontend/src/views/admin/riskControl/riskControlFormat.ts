import type { ContentModerationAPIKeyStatus, ContentModerationLog, ModerationEngine } from '@/api/admin/riskControl'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'

type Translate = (key: string, params?: Record<string, unknown>) => string
type KeyStatus = ContentModerationAPIKeyStatus['status']

export const engineLabel = (engine: ModerationEngine | undefined) => (engine === 'typesafe' ? 'TypeSafe AI' : 'OpenAI')

export function percent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${(value * 100).toFixed(1)}%`
}

export function percentWidth(value: number): string {
  if (!Number.isFinite(value)) return '0%'
  return `${Math.min(100, Math.max(0, value * 100)).toFixed(1)}%`
}

export function latencyText(value: number | null): string {
  if (value === null || value === undefined) return '-'
  return `${value} ms`
}

export function formatDateTime(value: string): string {
  return formatDateTimeValue(value) || '-'
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value)
}

/** Literal `.ui-dot` classes so Tailwind keeps them. */
export function keyStatusDotClass(status: KeyStatus): string {
  if (status === 'ok') return 'ui-dot is-ok'
  if (status === 'error') return 'ui-dot is-warn'
  if (status === 'frozen') return 'ui-dot is-bad'
  return 'ui-dot'
}

export function resultLabel(row: ContentModerationLog, t: Translate): string {
  if (row.action === 'cyber_policy') return t('admin.riskControl.action.cyberPolicy')
  if (row.action === 'keyword_block') return t('admin.riskControl.action.keywordBlock')
  if (row.action === 'block') return t('admin.riskControl.action.block')
  if (row.action === 'error' || row.error) return t('admin.riskControl.action.error')
  if (row.flagged) return t('admin.riskControl.result.hit')
  return t('admin.riskControl.result.pass')
}

export function resultBadgeClass(row: ContentModerationLog): string {
  if (row.action === 'block' || row.action === 'keyword_block' || row.action === 'cyber_policy') return 'badge badge-danger'
  if (row.action === 'error' || row.error) return 'badge badge-gray'
  if (row.flagged) return 'badge badge-warning'
  return 'badge badge-success'
}

export function violationCountText(row: ContentModerationLog, t: Translate): string {
  if (!row.flagged) return '-'
  if (row.violation_count === 0) return t('admin.riskControl.violationNotCounted')
  return t('admin.riskControl.violationCount', { count: row.violation_count || 1 })
}

export function canUnbanRow(row: ContentModerationLog): boolean {
  return Boolean(row.auto_banned && row.user_id && row.user_status === 'disabled')
}

export function inputSummaryText(row: ContentModerationLog): string {
  return row.input_excerpt || row.error || '-'
}
