import { describe, expect, it } from 'vitest'
import type { Account, AccountUsageInfo } from '@/types'
import {
  buildNormalizedPoolCapacity,
  buildProviderCapacity,
  normalizeAccountCapacity,
} from '../operatorCapacity'

const account = (id: number, overrides: Partial<Account> = {}): Account => ({
  id,
  name: `Account ${id}`,
  platform: 'openai',
  type: 'oauth',
  proxy_id: null,
  concurrency: 1,
  priority: 1,
  status: 'active',
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-08-25T00:00:00Z',
  updated_at: '2026-08-25T00:00:00Z',
  schedulable: true,
  rate_limited_at: null,
  rate_limit_reset_at: null,
  overload_until: null,
  temp_unschedulable_until: null,
  temp_unschedulable_reason: null,
  session_window_start: null,
  session_window_end: null,
  session_window_status: null,
  ...overrides,
})

const usage = (overrides: Partial<AccountUsageInfo>): AccountUsageInfo => ({
  updated_at: '2026-08-25T00:00:00Z',
  five_hour: null,
  seven_day: null,
  seven_day_sonnet: null,
  ...overrides,
})

describe('operator capacity normalization', () => {
  it('keeps provider windows and spend limits separate while surfacing the lowest remaining quota', () => {
    const openAIUsage = usage({
      five_hour: { utilization: 32, resets_at: '2026-08-25T21:00:00Z', remaining_seconds: 7200 },
      seven_day: { utilization: 48, resets_at: '2026-08-29T00:00:00Z', remaining_seconds: 345600 },
    })
    const antigravityUsage = usage({
      antigravity_quota: {
        'gemini-2.5-pro': { utilization: 72, reset_time: '2026-08-26T00:00:00Z' },
        'claude-sonnet-4-5': { utilization: 44, reset_time: '2026-08-26T00:00:00Z' },
      },
    })
    const apiKey = account(2, {
      name: 'API key account',
      type: 'apikey',
      quota_limit: 100,
      quota_used: 18.4,
    })

    const openAI = normalizeAccountCapacity(account(1), openAIUsage)
    const antigravity = normalizeAccountCapacity(
      account(3, { platform: 'antigravity' }),
      antigravityUsage,
    )
    const spend = normalizeAccountCapacity(apiKey)
    const unknown = normalizeAccountCapacity(account(4, { platform: 'gemini' }))

    expect(openAI.windows.map((window) => [window.label, window.remainingPercent])).toEqual([
      ['5h', 68],
      ['7d', 52],
    ])
    expect(openAI.lowestRemaining).toBe(52)
    expect(antigravity.lowestRemaining).toBe(28)
    expect(spend.windows).toEqual([
      expect.objectContaining({ label: '$ total', remainingPercent: 81.6, kind: 'spend' }),
    ])
    expect(unknown.lowestRemaining).toBeNull()

    const providers = buildProviderCapacity(
      [account(1), apiKey, account(3, { platform: 'antigravity' }), account(4, { platform: 'gemini' })],
      { '1': openAIUsage, '3': antigravityUsage },
    )
    expect(providers.find((provider) => provider.platform === 'openai')).toMatchObject({
      knownCount: 2,
      unknownCount: 0,
      remainingPercent: 66.8,
      lowestRemaining: 52,
      lowestAccount: expect.objectContaining({ account: expect.objectContaining({ id: 1 }) }),
      schedulableCount: 2,
    })
    expect(providers.find((provider) => provider.platform === 'gemini')).toMatchObject({
      knownCount: 0,
      unknownCount: 1,
      lowestRemaining: null,
    })
  })

  it('weights known accounts equally and excludes unknown quota from the pool', () => {
    const highCapacity = normalizeAccountCapacity(account(1), usage({
      five_hour: { utilization: 20, resets_at: '2026-08-25T21:00:00Z', remaining_seconds: 7200 },
      seven_day: { utilization: 10, resets_at: '2026-08-29T00:00:00Z', remaining_seconds: 345600 },
    }))
    const limitedCapacity = normalizeAccountCapacity(account(2), usage({
      five_hour: { utilization: 60, resets_at: '2026-08-25T21:00:00Z', remaining_seconds: 7200 },
    }))
    const unknownCapacity = normalizeAccountCapacity(account(3, { platform: 'gemini' }))

    const pool = buildNormalizedPoolCapacity([
      highCapacity,
      limitedCapacity,
      unknownCapacity,
    ])

    expect(highCapacity.lowestRemaining).toBe(80)
    expect(pool.remainingPercent).toBe(60)
    expect(pool.usedPercent).toBe(40)
    expect(pool.segments.map((segment) => segment.contributionPercent)).toEqual([40, 20])
    expect(pool.knownCount).toBe(2)
    expect(pool.unknownAccounts.map((summary) => summary.account.id)).toEqual([3])
  })

  it('reports mixed and all-unknown provider quota without treating unknown as zero', () => {
    const providers = buildProviderCapacity(
      [
        account(1),
        account(2),
        account(3, { platform: 'gemini' }),
      ],
      {
        '1': usage({
          five_hour: { utilization: 25, resets_at: '2026-08-26T02:00:00Z', remaining_seconds: 7200 },
        }),
      },
      {},
      new Date('2026-08-25T18:00:00Z'),
    )

    expect(providers.find((provider) => provider.platform === 'openai')).toMatchObject({
      remainingPercent: 75,
      knownCount: 1,
      unknownCount: 1,
      lowestRemaining: 75,
      nextReset: '2026-08-26T02:00:00Z',
    })
    expect(providers.find((provider) => provider.platform === 'gemini')).toMatchObject({
      remainingPercent: null,
      knownCount: 0,
      unknownCount: 1,
      lowestRemaining: null,
      lowestAccount: null,
      nextReset: null,
    })
  })

  it('uses only limiting windows for the next provider reset', () => {
    const provider = buildProviderCapacity(
      [account(1), account(2)],
      {
        '1': usage({
          five_hour: { utilization: 20, resets_at: '2026-08-25T20:00:00Z', remaining_seconds: 7200 },
          seven_day: { utilization: 60, resets_at: '2026-08-28T00:00:00Z', remaining_seconds: 172800 },
        }),
        '2': usage({
          five_hour: { utilization: 70, resets_at: '2026-08-26T01:00:00Z', remaining_seconds: 25200 },
        }),
      },
      {},
      new Date('2026-08-25T18:00:00Z'),
    )[0]

    expect(provider.lowestAccount?.account.id).toBe(2)
    expect(provider.nextReset).toBe('2026-08-26T01:00:00Z')
  })

  it('keeps the global average consistent with known-account-weighted provider averages', () => {
    const providers = buildProviderCapacity(
      [
        account(1),
        account(2),
        account(3, { platform: 'anthropic' }),
        account(4, { platform: 'gemini' }),
      ],
      {
        '1': usage({ five_hour: { utilization: 20, resets_at: null, remaining_seconds: null } }),
        '2': usage({ five_hour: { utilization: 40, resets_at: null, remaining_seconds: null } }),
        '3': usage({ five_hour: { utilization: 90, resets_at: null, remaining_seconds: null } }),
      },
    )
    const pool = buildNormalizedPoolCapacity(providers.flatMap((provider) => provider.accounts))
    const providerWeightedAverage = providers.reduce(
      (total, provider) => total + (provider.remainingPercent ?? 0) * provider.knownCount,
      0,
    ) / providers.reduce((total, provider) => total + provider.knownCount, 0)

    expect(pool.remainingPercent).toBeCloseTo(providerWeightedAverage)
    expect(pool.remainingPercent).toBe(50)
    expect(pool.unknownCount).toBe(1)
  })
})
