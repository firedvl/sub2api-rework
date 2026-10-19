import { describe, expect, it } from 'vitest'
import { buildGrokUsageRefreshKey, buildOpenAIUsageRefreshKey, mergeClaudeUsageObservations } from '../accountUsageRefresh'

describe('buildOpenAIUsageRefreshKey', () => {
  it('会在 codex 快照变化时生成不同 key', () => {
    const base = {
      id: 1,
      platform: 'openai',
      type: 'oauth',
      updated_at: '2026-03-07T10:00:00Z',
      last_used_at: '2026-03-07T09:59:00Z',
      extra: {
        codex_usage_updated_at: '2026-03-07T10:00:00Z',
        codex_5h_used_percent: 0,
        codex_7d_used_percent: 0
      }
    } as any

    const next = {
      ...base,
      extra: {
        ...base.extra,
        codex_usage_updated_at: '2026-03-07T10:01:00Z',
        codex_5h_used_percent: 100
      }
    }

    expect(buildOpenAIUsageRefreshKey(base)).not.toBe(buildOpenAIUsageRefreshKey(next))
  })

  it('会在 last_used_at 变化时生成不同 key', () => {
    const base = {
      id: 3,
      platform: 'openai',
      type: 'oauth',
      updated_at: '2026-03-07T10:00:00Z',
      last_used_at: '2026-03-07T10:00:00Z',
      extra: {
        codex_usage_updated_at: '2026-03-07T10:00:00Z',
        codex_5h_used_percent: 12,
        codex_7d_used_percent: 24
      }
    } as any

    const next = {
      ...base,
      last_used_at: '2026-03-07T10:02:00Z'
    }

    expect(buildOpenAIUsageRefreshKey(base)).not.toBe(buildOpenAIUsageRefreshKey(next))
  })

  it('非 OpenAI OAuth 账号返回空 key', () => {
    expect(buildOpenAIUsageRefreshKey({
      id: 2,
      platform: 'anthropic',
      type: 'oauth',
      updated_at: '2026-03-07T10:00:00Z',
      last_used_at: '2026-03-07T10:00:00Z',
      extra: {}
    } as any)).toBe('')
  })
})

describe('buildGrokUsageRefreshKey', () => {
  it('changes when a canonical Grok billing or usage snapshot changes', () => {
    const base = {
      platform: 'grok',
      extra: {
        grok_billing_snapshot: { plan: 'Free', usage_percent: 0 },
        grok_usage_snapshot: { subscription_tier: 'Free', status_code: 200 }
      }
    } as any

    expect(buildGrokUsageRefreshKey(base)).not.toBe(buildGrokUsageRefreshKey({
      ...base,
      extra: {
        ...base.extra,
        grok_billing_snapshot: { plan: 'SuperGrok', usage_percent: 0 }
      }
    }))
    expect(buildGrokUsageRefreshKey(base)).not.toBe(buildGrokUsageRefreshKey({
      ...base,
      extra: {
        ...base.extra,
        grok_usage_snapshot: { subscription_tier: 'SuperGrok', status_code: 200 }
      }
    }))
  })

  it('ignores object key order and a legacy alias shadowed by canonical usage', () => {
    const first = {
      platform: 'grok',
      extra: {
        grok_billing_snapshot: {
          plan: 'SuperGrok',
          limits: { monthly: 100, weekly: 25 }
        },
        grok_usage_snapshot: { status_code: 200, subscription_tier: 'SuperGrok' },
        grok_quota_snapshot: { subscription_tier: 'Free' }
      }
    } as any
    const reordered = {
      platform: 'grok',
      extra: {
        grok_quota_snapshot: { subscription_tier: 'SuperGrok Heavy' },
        grok_usage_snapshot: { subscription_tier: 'SuperGrok', status_code: 200 },
        grok_billing_snapshot: {
          limits: { weekly: 25, monthly: 100 },
          plan: 'SuperGrok'
        }
      }
    } as any

    expect(buildGrokUsageRefreshKey(first)).toBe(buildGrokUsageRefreshKey(reordered))
  })

  it('uses the legacy quota alias only when the canonical snapshot is absent', () => {
    const base = {
      platform: 'grok',
      extra: { grok_quota_snapshot: { subscription_tier: 'Free' } }
    } as any
    const next = {
      platform: 'grok',
      extra: { grok_quota_snapshot: { subscription_tier: 'SuperGrok' } }
    } as any

    expect(buildGrokUsageRefreshKey(base)).not.toBe(buildGrokUsageRefreshKey(next))
  })

  it('tracks the legacy tier when the canonical snapshot has no usable tier', () => {
    for (const canonicalSnapshot of [
      { status_code: 200 },
      { status_code: 200, subscription_tier: '   ' },
    ]) {
      const base = {
        platform: 'grok',
        extra: {
          grok_usage_snapshot: canonicalSnapshot,
          grok_quota_snapshot: { subscription_tier: 'Free' },
        },
      } as any
      const next = {
        ...base,
        extra: {
          ...base.extra,
          grok_quota_snapshot: { subscription_tier: 'SuperGrok' },
        },
      }

      expect(buildGrokUsageRefreshKey(base)).not.toBe(buildGrokUsageRefreshKey(next))
    }
  })

  it('returns an empty key for non-Grok accounts', () => {
    expect(buildGrokUsageRefreshKey({
      platform: 'openai',
      extra: { grok_usage_snapshot: { subscription_tier: 'SuperGrok' } }
    } as any)).toBe('')
  })
})


describe('mergeClaudeUsageObservations', () => {
  const observation = (utilization: number, observed_at: string, source: 'active' | 'passive') => ({
    utilization, observed_at, source, resets_at: null, remaining_seconds: 0,
  })

  it('keeps the latest observation independently for each Claude window', () => {
    const earlier = '2026-10-18T10:00:00Z'
    const later = '2026-10-18T10:01:00Z'
    const previous = {
      source: 'active' as const, updated_at: later,
      five_hour: observation(30, later, 'active'),
      seven_day: observation(60, earlier, 'active'), seven_day_sonnet: null,
      seven_day_fable: { ...observation(12, earlier, 'passive'), stale: true },
    }
    const incoming = {
      source: 'passive' as const, cached: true, updated_at: later,
      five_hour: observation(20, earlier, 'passive'),
      seven_day: observation(40, later, 'passive'), seven_day_sonnet: null,
    }
    const merged = mergeClaudeUsageObservations(previous, incoming)
    expect(merged.five_hour).toBe(previous.five_hour)
    expect(merged.seven_day).toBe(incoming.seven_day)
    expect(merged.seven_day_fable).toBe(previous.seven_day_fable)
    expect(merged.source).toBe('mixed')
    expect(merged.cached).toBe(true)
    expect(merged.stale).toBe(true)
    expect(merged.updated_at).toBe(later)
  })

  it('does not turn cached active data into a newer observation or lose refresh failures', () => {
    const previous = {
      source: 'active' as const, updated_at: '2026-10-18T10:01:00Z',
      five_hour: observation(30, '2026-10-18T10:01:00Z', 'active'),
      seven_day: null, seven_day_sonnet: null,
    }
    const merged = mergeClaudeUsageObservations(previous, {
      source: 'passive', cached: true, updated_at: '2026-10-18T10:00:00Z',
      five_hour: observation(90, '2026-10-18T10:00:00Z', 'passive'),
      seven_day: null, seven_day_sonnet: null, error: 'Refresh failed',
    })
    expect(merged.five_hour).toBe(previous.five_hour)
    expect(merged.updated_at).toBe(previous.updated_at)
    expect(merged.source).toBe('active')
    expect(merged.error).toBe('Refresh failed')
  })

  it('keeps active data over an equal-time passive replay, while allowing a later reset to reduce usage', () => {
    const previous = {
      source: 'active' as const, updated_at: '2026-10-18T10:01:00Z',
      five_hour: observation(90, '2026-10-18T10:01:00Z', 'active'),
      seven_day: null, seven_day_sonnet: null,
    }
    const replay = { ...previous, source: 'passive' as const, five_hour: observation(20, previous.updated_at, 'passive') }
    expect(mergeClaudeUsageObservations(previous, replay).five_hour).toBe(previous.five_hour)
    const reset = { ...replay, five_hour: observation(0, '2026-10-18T15:01:00Z', 'passive') }
    expect(mergeClaudeUsageObservations(previous, reset).five_hour).toBe(reset.five_hour)
  })
})
