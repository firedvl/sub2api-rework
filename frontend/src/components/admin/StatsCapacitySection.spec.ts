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
  it('renders global and provider pool partitions with unknown accounts excluded', () => {
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
            seven_day: { utilization: 10, resets_at: null, remaining_seconds: null },
          }),
          '2': usage({
            five_hour: { utilization: 60, resets_at: null, remaining_seconds: null },
          }),
        },
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const global = wrapper.get('[data-testid="global-capacity-donut"]')
    expect(global.get('[data-testid="capacity-used-segment"]').attributes('style')).toContain('40 60')
    expect(global.findAll('[data-testid="capacity-account-segment"]')).toHaveLength(2)
    expect(global.text()).toContain('Unknown Gemini')
    expect(global.text()).toContain('admin.dashboard.capacity.poolContribution 40')
    expect(global.text()).toContain('admin.dashboard.capacity.poolContribution 20')

    const openAI = wrapper.get('[data-testid="provider-capacity-donut-openai"]')
    expect(openAI.get('[data-testid="capacity-used-segment"]').attributes('style')).toContain('40 60')
    expect(openAI.findAll('[data-testid="capacity-account-segment"]')).toHaveLength(2)

    const gemini = wrapper.get('[data-testid="provider-capacity-donut-gemini"]')
    expect(gemini.get('[data-testid="capacity-unknown-ring"]').attributes('tabindex')).toBe('0')
    expect(gemini.find('[data-testid="capacity-used-segment"]').exists()).toBe(false)
    expect(gemini.get('svg').attributes('aria-label')).toContain('admin.dashboard.capacity.quotaUnknown')
  })

  it('shows each provider window and its per-account contributions', () => {
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

    const provider = wrapper.get('[data-testid="provider-capacity-openai"]')
    expect(provider.text()).toContain('5h')
    expect(provider.text()).toContain('7d')
    expect(provider.text()).toContain('admin.stats.capacity.windowCoverage 2 0')
    expect(provider.text()).toContain('admin.stats.capacity.windowCoverage 1 1')
    expect(provider.text()).toContain('admin.dashboard.capacity.poolContribution 40')
    expect(provider.text()).toContain('admin.dashboard.capacity.poolContribution 25')
  })
})
