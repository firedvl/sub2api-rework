import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import type { Account, DashboardStats, TrendDataPoint } from '@/types'
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

const trendPoint = (date: string, requests: number, totalTokens = 0): TrendDataPoint => ({
  date,
  requests,
  input_tokens: 0,
  output_tokens: 0,
  cache_creation_tokens: 0,
  cache_read_tokens: 0,
  total_tokens: totalTokens,
  cost: 0,
  actual_cost: 0,
})

const mockTrend = (trend: TrendDataPoint[], granularity: 'day' | 'hour' = 'day') => {
  getSnapshotV2.mockResolvedValueOnce({
    generated_at: '2026-08-25T12:00:00Z',
    stats: stats(),
    granularity,
    trend,
  })
}

const mountView = (attachTo?: Element) => mount(StatsView, {
  ...(attachTo ? { attachTo } : {}),
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
      granularity: 'day',
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
    document.body.innerHTML = ''
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
    expect(wrapper.get('[data-testid="stats-token-trend"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="provider-capacity-openai"]').text()).toContain('Account 1')
    expect(wrapper.get('.stats-view').element.children[0].classList).toContain('stats-usage-section')
    expect(wrapper.get('.stats-view').element.children[1].classList).toContain('stats-capacity-section')
  })

  it('renders coordinated hourly bar charts with exact summaries, tooltips, and keyboard navigation', async () => {
    mockTrend([
      trendPoint('2026-08-25T00:00:00Z', 0, 0),
      trendPoint('2026-08-25T01:00:00Z', 10, 100),
      trendPoint('2026-08-25T02:00:00Z', 20, 300),
      trendPoint('2026-08-25T03:00:00Z', 50, 600),
    ], 'hour')

    const wrapper = mountView(document.body)
    await flushPromises()

    const requestChart = wrapper.get('[data-testid="stats-request-trend"]')
    const tokenChart = wrapper.get('[data-testid="stats-token-trend"]')
    const requestBars = requestChart.findAll('[data-testid="stats-trend-bar"]')
    const tokenBars = tokenChart.findAll('[data-testid="stats-trend-bar"]')

    expect(requestBars).toHaveLength(4)
    expect(tokenBars).toHaveLength(4)
    expect(requestBars[0].classes()).toContain('is-first')
    expect(requestBars[requestBars.length - 1].classes()).toContain('is-last')
    expect(tokenBars[0].classes()).toContain('is-first')
    expect(tokenBars[tokenBars.length - 1].classes()).toContain('is-last')
    expect(requestChart.get('[data-testid="stats-trend-total"]').text()).toBe('80')
    expect(requestChart.get('[data-testid="stats-trend-average"]').text()).toBe('20')
    expect(requestChart.get('[data-testid="stats-trend-peak"]').text()).toBe('50')
    expect(tokenChart.get('[data-testid="stats-trend-total"]').text()).toBe('1.0K')
    expect(tokenChart.get('[data-testid="stats-trend-average"]').text()).toBe('250')
    expect(tokenChart.get('[data-testid="stats-trend-peak"]').text()).toBe('600')
    expect(requestChart.findAll('.stats-bar-grid-line')).toHaveLength(4)
    expect(requestChart.get('[role="group"]').attributes('aria-label')).toContain('admin.stats.usage.recentHourlyPeriods 4')
    expect(tokenChart.get('[role="group"]').attributes('aria-label')).toContain('1,000 admin.stats.usage.tokens')
    expect(requestChart.findAll('.stats-bar-x-label')).toHaveLength(4)
    expect(requestBars.map((bar) => bar.attributes('aria-label'))).toEqual([
      expect.stringContaining(': 0 admin.stats.usage.requests'),
      expect.stringContaining(': 10 admin.stats.usage.requests'),
      expect.stringContaining(': 20 admin.stats.usage.requests'),
      expect.stringContaining(': 50 admin.stats.usage.requests'),
    ])
    expect(Number.parseFloat(requestBars[0].attributes('style')!.match(/width: ([\d.]+)%/)![1])).toBeGreaterThan(6)
    expect(wrapper.find('.stats-view select').exists()).toBe(false)

    await requestBars[1].trigger('focus')
    expect(requestBars[1].classes()).toContain('is-active')
    expect(requestChart.get('[data-testid="stats-trend-tooltip"]').text()).toContain('10 admin.stats.usage.requests')
    await requestBars[1].trigger('keydown', { key: 'ArrowRight' })
    expect(document.activeElement).toBe(requestBars[2].element)
    expect(requestChart.get('[data-testid="stats-trend-tooltip"]').text()).toContain('20 admin.stats.usage.requests')
    expect(requestBars[2].attributes('aria-describedby')).toBe('stats-request-trend-tooltip')

    await requestBars[2].trigger('keydown', { key: 'Escape' })
    expect(requestChart.find('[data-testid="stats-trend-tooltip"]').exists()).toBe(false)

    await tokenBars[2].trigger('mouseenter')
    expect(tokenChart.get('[data-testid="stats-trend-tooltip"]').text()).toContain('300 admin.stats.usage.tokens')
    await tokenBars[2].trigger('mouseleave')
    await requestBars[3].trigger('focus')
    expect(requestChart.get('[data-testid="stats-trend-tooltip"]').classes()).toContain('is-below')
    wrapper.unmount()
  })

  it('compacts large tooltip headlines while preserving exact accessible values and timestamps', async () => {
    const date = '2026-08-25T02:00:00Z'
    mockTrend([trendPoint(date, 20_800, 2_800_000_000)], 'hour')

    const wrapper = mountView(document.body)
    await flushPromises()

    const requestBar = wrapper.get('[data-testid="stats-request-trend"] [data-testid="stats-trend-bar"]')
    const tokenBar = wrapper.get('[data-testid="stats-token-trend"] [data-testid="stats-trend-bar"]')

    await requestBar.trigger('focus')
    const requestTooltip = wrapper.get('[data-testid="stats-request-trend"] [data-testid="stats-trend-tooltip"]')
    expect(requestTooltip.get('strong').text()).toBe('20.8K admin.stats.usage.requests')
    expect(requestTooltip.get('span').text()).toBe(new Date(date).toLocaleString())
    expect(requestBar.attributes('aria-label')).toContain('20,800 admin.stats.usage.requests')

    await tokenBar.trigger('focus')
    const tokenTooltip = wrapper.get('[data-testid="stats-token-trend"] [data-testid="stats-trend-tooltip"]')
    expect(tokenTooltip.get('strong').text()).toBe('2.8B admin.stats.usage.tokens')
    expect(tokenTooltip.get('span').text()).toBe(new Date(date).toLocaleString())
    expect(tokenBar.attributes('aria-label')).toContain('2,800,000,000 admin.stats.usage.tokens')
    wrapper.unmount()
  })

  it('keeps a single zero-request period on the baseline', async () => {
    mockTrend([trendPoint('2026-08-25T00:00:00Z', 0)])

    const wrapper = mountView()
    await flushPromises()

    const chart = wrapper.get('[data-testid="stats-request-trend"]')
    const point = chart.get('[data-testid="stats-trend-bar"]')
    expect(chart.get('[data-testid="stats-trend-total"]').text()).toBe('0')
    expect(chart.get('[data-testid="stats-trend-average"]').text()).toBe('0')
    expect(chart.get('[data-testid="stats-trend-peak"]').text()).toBe('0')
    expect(point.classes()).toContain('is-zero')
    expect(point.attributes('style')).toContain('left: 50%')
    expect(point.attributes('style')).toContain('bottom: 22%')
    expect(point.attributes('style')).toContain('height: 1.25%')
  })

  it('plots equal request values without collapsing the chart', async () => {
    mockTrend([
      trendPoint('2026-08-23T00:00:00Z', 7),
      trendPoint('2026-08-24T00:00:00Z', 7),
      trendPoint('2026-08-25T00:00:00Z', 7),
    ])

    const wrapper = mountView()
    await flushPromises()

    const chart = wrapper.get('[data-testid="stats-request-trend"]')
    const points = chart.findAll('[data-testid="stats-trend-bar"]')
    expect(points).toHaveLength(3)
    expect(new Set(points.map((point) => point.attributes('style')?.match(/height: ([^;]+)/)?.[1]))).toHaveLength(1)
    expect(chart.get('[data-testid="stats-trend-total"]').text()).toBe('21')
    expect(chart.get('[data-testid="stats-trend-average"]').text()).toBe('7')
    expect(chart.get('[data-testid="stats-trend-peak"]').text()).toBe('7')
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
