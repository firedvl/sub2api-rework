import { describe, expect, it } from 'vitest'
import { memberTone, poolSplit, poolTone, remainingPercent, servedModels } from '../routeSummary'

describe('routeSummary', () => {
  it('splits a pool into disjoint, clamped segments', () => {
    expect(poolSplit({ account_count: 5, active_account_count: 3, rate_limited_account_count: 1 })).toEqual({
      total: 5, available: 3, limited: 1, unavailable: 1,
    })
    expect(poolSplit({ account_count: 2, active_account_count: 4, rate_limited_account_count: 3 })).toEqual({
      total: 2, available: 2, limited: 0, unavailable: 0,
    })
    expect(poolSplit({})).toEqual({ total: 0, available: 0, limited: 0, unavailable: 0 })
  })

  it('derives a pool tone', () => {
    expect(poolTone(poolSplit({}))).toBe('idle')
    expect(poolTone(poolSplit({ account_count: 2, active_account_count: 2 }))).toBe('ok')
    expect(poolTone(poolSplit({ account_count: 2, active_account_count: 1, rate_limited_account_count: 1 }))).toBe('warn')
    expect(poolTone(poolSplit({ account_count: 2, active_account_count: 0 }))).toBe('bad')
  })

  it('classifies member accounts like the scheduler pool filters', () => {
    const now = Date.parse('2026-08-30T02:00:00Z')
    const base = { status: 'active' as const, schedulable: true, rate_limit_reset_at: null, overload_until: null, temp_unschedulable_until: null }
    expect(memberTone(base, now)).toBe('ok')
    expect(memberTone({ ...base, rate_limit_reset_at: '2026-08-30T03:00:00Z' }, now)).toBe('warn')
    expect(memberTone({ ...base, rate_limit_reset_at: '2026-08-30T01:00:00Z' }, now)).toBe('ok')
    expect(memberTone({ ...base, schedulable: false }, now)).toBe('idle')
    expect(memberTone({ ...base, status: 'error' }, now)).toBe('bad')
  })

  it('collects served models from enabled sources only', () => {
    expect(servedModels({
      model_routing_enabled: true,
      model_routing: { 'gpt-5.2': [1], 'gpt-5.2-codex': [2] },
      models_list_config: { enabled: false, models: ['hidden'] },
      model_allowlist: { enabled: true, models: ['gpt-5.2'] },
      default_mapped_model: '',
    })).toEqual(['gpt-5.2', 'gpt-5.2-codex'])
    expect(servedModels({ model_routing_enabled: false, model_routing: { x: [1] }, models_list_config: undefined, model_allowlist: undefined })).toEqual([])
  })

  it('reports remaining spend as a percentage', () => {
    expect(remainingPercent(10, 40)).toBe(75)
    expect(remainingPercent(50, 40)).toBe(0)
    expect(remainingPercent(1, null)).toBeNull()
  })
})
