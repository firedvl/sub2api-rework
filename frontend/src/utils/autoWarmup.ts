import type { Account } from '@/types'

export function isOpenAIAutoWarmupConfigurable(account: Pick<Account, 'platform' | 'type' | 'parent_account_id'>): boolean {
  return account.platform === 'openai' && account.type === 'oauth' && account.parent_account_id == null
}

export function autoWarmupReason(account: Account, globalEnabled: boolean | undefined, now = Date.now()): string {
  if (globalEnabled === false) return 'disabled_global'
  if (!account.extra?.auto_warmup_enabled) return 'disabled_account'
  if (account.status === 'error') return 'credential_attention'
  if (account.status !== 'active' || !account.schedulable) return 'account_not_schedulable'
  if ([account.temp_unschedulable_until, account.overload_until, account.rate_limit_reset_at].some(time => time && Date.parse(time) > now)) return 'account_not_schedulable'
  const attempt = account.extra.codex_auto_warmup_state
  const evaluation = account.extra.codex_auto_warmup_evaluation
  const resetAt = Date.parse(account.extra.codex_5h_reset_at || '')
  const observedAt = Date.parse(account.extra.codex_usage_updated_at || '')
  const dormant = Number(account.extra.codex_5h_used_percent) <= 0.1 && Math.abs(resetAt - observedAt - 5 * 60 * 60 * 1000) <= 2 * 60 * 1000
  if (evaluation && ['quota_unavailable', 'credential_attention'].includes(evaluation.reason) && (!attempt?.attempted_at || Date.parse(evaluation.checked_at) > Date.parse(attempt.attempted_at))) return evaluation.reason
  if (attempt) {
    const sameWindow = Math.abs(Date.parse(attempt.observed_5h_reset_at || attempt.reset_at || '') - Date.parse(account.extra.codex_5h_reset_at || '')) <= 60000
    if (sameWindow && Date.parse(account.extra.codex_5h_reset_at || '') > now && (attempt.status === 'succeeded' || attempt.window_started)) return 'current_window_complete'
    if (attempt.status === 'failed') {
      if ((sameWindow || dormant) && Date.parse(attempt.reset_at || '') > now && ['OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED', 'OPENAI_AUTO_WARMUP_MODEL_UNAVAILABLE'].includes(attempt.error_code || '')) return 'model_unavailable'
      if (attempt.error_code === 'OPENAI_AUTO_WARMUP_AUTH_FAILED') return 'credential_attention'
      if (sameWindow && Date.parse(attempt.reset_at || '') > now) return 'failed'
    }
    if (attempt.status === 'pending') return now - Date.parse(attempt.attempted_at || '') < 60000 ? 'pending' : 'already_attempted'
  }
  const known = ['waiting_new_window', 'waiting_second_observation', 'retry_floor', 'already_attempted', 'quota_unavailable', 'claim_unavailable', 'account_not_schedulable']
  if (evaluation && known.includes(evaluation.reason)) return evaluation.reason
  if (Number.isFinite(observedAt)) {
    if (account.extra.codex_5h_window_minutes !== 300) return 'quota_unavailable'
    if (resetAt > now) return dormant ? 'waiting_second_observation' : 'waiting_new_window'
  }
  return 'waiting_evaluation'
}
