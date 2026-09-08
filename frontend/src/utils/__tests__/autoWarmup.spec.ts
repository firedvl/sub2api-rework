import { describe, expect, it } from 'vitest'
import type { Account } from '@/types'
import { autoWarmupReason } from '../autoWarmup'

describe('warm-up operator reason', () => {
  const now = Date.parse('2026-09-08T00:00:00Z')
  const base = { platform: 'openai', type: 'oauth', status: 'active', schedulable: true, extra: { auto_warmup_enabled: true } } as Account
  it('prioritizes switches and account eligibility', () => {
    expect(autoWarmupReason(base, false, now)).toBe('disabled_global')
    expect(autoWarmupReason({ ...base, extra: {} }, true, now)).toBe('disabled_account')
    expect(autoWarmupReason({ ...base, schedulable: false }, true, now)).toBe('account_not_schedulable')
    expect(autoWarmupReason({ ...base, status: 'error' }, true, now)).toBe('credential_attention')
  })
  it('does not label a previous success as current-window completion', () => {
    const account = { ...base, extra: { ...base.extra, codex_5h_reset_at: '2026-09-08T04:00:00Z', codex_auto_warmup_state: { status: 'succeeded', reset_at: '2026-09-07T23:00:00Z' } } }
    expect(autoWarmupReason(account, true, now)).toBe('waiting_evaluation')
    account.extra.codex_auto_warmup_state.reset_at = account.extra.codex_5h_reset_at
    expect(autoWarmupReason(account, true, now)).toBe('current_window_complete')
  })
  it('uses bounded reasons instead of raw upstream errors', () => {
    const account = { ...base, extra: { ...base.extra, codex_5h_reset_at: '2026-09-08T04:00:00Z', codex_auto_warmup_state: { status: 'failed', reset_at: '2026-09-08T04:00:00Z', error_code: 'OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED' } } }
    expect(autoWarmupReason(account, true, now)).toBe('model_unavailable')
    expect(autoWarmupReason(account, undefined, now)).not.toBe('disabled_global')
    account.extra.codex_auto_warmup_state.reset_at = '2026-09-07T23:00:00Z'
    expect(autoWarmupReason(account, true, now)).toBe('waiting_evaluation')
    expect(autoWarmupReason({ ...base, extra: { ...base.extra, codex_auto_warmup_evaluation: { reason: 'waiting_second_observation', checked_at: '2026-09-08T00:00:00Z' } } }, true, now)).toBe('waiting_second_observation')
  })
  it('shows the newer anchored window instead of a historical model failure', () => {
    const account = { ...base, extra: { ...base.extra, codex_5h_window_minutes: 300, codex_5h_used_percent: 25,
      codex_usage_updated_at: '2026-09-08T00:00:00Z', codex_5h_reset_at: '2026-09-08T03:00:00Z',
      codex_auto_warmup_state: { status: 'failed', reset_at: '2026-09-08T02:30:00Z', error_code: 'OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED' } } }
    expect(autoWarmupReason(account, true, now)).toBe('waiting_new_window')
    account.extra.codex_5h_window_minutes = 0
    expect(autoWarmupReason(account, true, now)).toBe('quota_unavailable')
  })
})
