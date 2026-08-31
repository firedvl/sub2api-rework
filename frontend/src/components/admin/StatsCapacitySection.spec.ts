import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Account, AccountUsageInfo } from '@/types'
import StatsCapacitySection from './StatsCapacitySection.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params
        ? `${key} ${Object.values(params).join(' ')}`
        : key,
    }),
  }
})

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

describe('StatsCapacitySection', () => {
  it('separates actual short and long windows, preserves unknowns, and renders provider icons', () => {
    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [
          account(1),
          account(2),
          account(3, { platform: 'gemini', name: 'Unknown Gemini' }),
        ],
        usageByAccountId: {
          '1': usage({
            five_hour: { utilization: 20, resets_at: null, remaining_seconds: null },
            seven_day: { utilization: 60, resets_at: null, remaining_seconds: null },
          }),
          '2': usage({
            five_hour: { utilization: 50, resets_at: null, remaining_seconds: null },
          }),
        },
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const shortTerm = wrapper.get('[data-testid="stats-short-term-capacity"]')
    const longTerm = wrapper.get('[data-testid="stats-long-term-capacity"]')
    expect(shortTerm.get('[data-testid="short-provider-openai"] svg').exists()).toBe(true)
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('5h')
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('65%')
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('admin.stats.capacity.windowCoverage 2 0')
    expect(shortTerm.get('[data-testid="short-provider-gemini"]').text()).toContain('admin.stats.capacity.notReported')

    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('7d')
    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('40%')
    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('admin.stats.capacity.windowCoverage 1 1')
    expect(longTerm.get('[data-testid="long-provider-gemini"]').text()).toContain('admin.stats.capacity.notReported')
    expect(wrapper.get('[data-testid="provider-capacity-gemini"]').text()).toContain('admin.stats.capacity.coverage 0 1')
  })

  it('uses one compact native inspector for account contributions and missing windows', async () => {
    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [account(1), account(2)],
        usageByAccountId: {
          '1': usage({
            five_hour: { utilization: 20, resets_at: null, remaining_seconds: null },
            seven_day: { utilization: 60, resets_at: null, remaining_seconds: null },
          }),
          '2': usage({
            five_hour: { utilization: 50, resets_at: null, remaining_seconds: null },
          }),
        },
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const inspector = wrapper.get('[data-testid="stats-capacity-inspector"]')
    const select = inspector.get('select')
    expect(select.findAll('option').map((option) => option.text())).toEqual([
      'OpenAI · 5h',
      'OpenAI · 7d',
    ])
    expect(inspector.text()).toContain('Account 1')
    expect(inspector.text()).toContain('Account 2')

    await select.setValue('openai:seven_day')
    expect(inspector.text()).toContain('OpenAI · 7d')
    expect(inspector.text()).toContain('admin.stats.capacity.windowCoverage 1 1')
    expect(inspector.text()).toContain('Account 1')
    expect(inspector.text()).toContain('Account 2')
    expect(inspector.text()).toContain('admin.dashboard.capacity.quotaUnknown')
  })
})
