import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { Account, DashboardStats } from '@/types'
import StatsView from '../StatsView.vue'

const { getSnapshotV2, listAccounts, getBatchUsage } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  listAccounts: vi.fn(),
  getBatchUsage: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: { getSnapshotV2 },
    accounts: { list: listAccounts, getBatchUsage },
  },
}))

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

const stats = (overrides: Partial<DashboardStats> = {}): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '2026-08-25T00:00:00Z',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 12,
  today_input_tokens: 100,
  today_output_tokens: 50,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 150,
  today_cost: 2,
  today_actual_cost: 1.5,
  today_account_cost: 0.75,
  average_duration_ms: 420,
  uptime: 100,
  rpm: 2,
  tpm: 25,
  ...overrides,
})

const account = (id: number): Account => ({
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
})

const mountView = () => mount(StatsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      LoadingSpinner: true,
    },
  },
})

describe('admin StatsView', () => {
  beforeEach(() => {
    getSnapshotV2.mockReset()
    listAccounts.mockReset()
    getBatchUsage.mockReset()
    vi.spyOn(console, 'error').mockImplementation(() => undefined)

    getSnapshotV2.mockResolvedValue({
      generated_at: '2026-08-25T12:00:00Z',
      stats: stats(),
      trend: [{
        date: '2026-08-25T00:00:00Z',
        requests: 12,
        input_tokens: 100,
        output_tokens: 50,
        cache_creation_tokens: 0,
        cache_read_tokens: 0,
        total_tokens: 150,
        cost: 2,
        actual_cost: 1.5,
      }],
    })
    listAccounts.mockResolvedValue({ items: [account(1)], total: 1, page: 1, page_size: 1000, pages: 1 })
    getBatchUsage.mockResolvedValue({
      usage: {
        '1': {
          updated_at: '2026-08-25T12:00:00Z',
          five_hour: { utilization: 25, resets_at: null, remaining_seconds: null },
          seven_day: null,
          seven_day_sonnet: null,
        },
      },
      errors: {},
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads supported metrics, trend, all account pages, and non-force usage', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledWith({
      include_stats: true,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false,
    })
    expect(listAccounts).toHaveBeenCalledWith(1, 1000, { include_scheduler_score: '0' })
    expect(getBatchUsage).toHaveBeenCalledWith([1], false)
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('$1.50')
    expect(wrapper.text()).toContain('$0.75')
    expect(wrapper.text()).toContain('420ms')
    expect(wrapper.get('[data-testid="stats-request-trend"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="provider-capacity-openai"]').text()).toContain('Account 1')
    expect(wrapper.get('.stats-view').element.children[0].classList).toContain('stats-usage-section')
    expect(wrapper.get('.stats-view').element.children[1].classList).toContain('stats-capacity-section')
  })

  it('keeps capacity visible when the stats snapshot fails', async () => {
    getSnapshotV2.mockRejectedValueOnce(new Error('snapshot unavailable'))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="stats-usage-error"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="provider-capacity-openai"]').text()).toContain('Account 1')
    expect(wrapper.find('[data-testid="stats-capacity-error"]').exists()).toBe(false)
  })

  it('keeps usage metrics visible when the account fleet fails', async () => {
    listAccounts.mockRejectedValueOnce(new Error('fleet unavailable'))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('$1.50')
    expect(wrapper.get('[data-testid="stats-capacity-error"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="stats-usage-error"]').exists()).toBe(false)
  })
})
