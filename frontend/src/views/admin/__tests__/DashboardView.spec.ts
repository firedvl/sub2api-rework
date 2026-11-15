import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import type { Account, DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const RouterLinkStub = {
  props: ['to'],
  template: '<a><slot /></a>'
}

const { getSnapshotV2, listUsage, listAccounts, getBatchUsage, showError } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  listUsage: vi.fn(),
  listAccounts: vi.fn(),
  getBatchUsage: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2
    },
    usage: {
      list: listUsage
    },
    accounts: {
      list: listAccounts,
      getBatchUsage
    },
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${JSON.stringify(params)}` : key),
      locale: { value: 'en-US' }
    })
  }
})

const createDashboardStats = (): DashboardStats => ({
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
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
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

const createAccount = (id: number, name: string, overrides: Partial<Account> = {}): Account => ({
  id,
  name,
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
  ...overrides
})

const TrendChartStub = {
  props: ['labels', 'tips', 'series', 'summary'],
  template: '<div data-testid="hourly-chart" />'
}

const mountDashboard = () => mount(DashboardView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      LoadingSpinner: true,
      Icon: true,
      ProviderMark: true,
      RingGauge: { props: ['value', 'label'], template: '<div class="ring-stub">{{ label }}:{{ value }}</div>' },
      TrendChart: TrendChartStub,
      RouterLink: RouterLinkStub,
    }
  }
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    listUsage.mockReset()
    listAccounts.mockReset()
    getBatchUsage.mockReset()
    showError.mockReset()

    getSnapshotV2.mockResolvedValue({
      generated_at: '2026-08-30T01:45:00Z',
      stats: createDashboardStats(),
      trend: []
    })
    listUsage.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 6, pages: 0 })
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 1000, pages: 0 })
    getBatchUsage.mockResolvedValue({ usage: {}, errors: {} })
  })

  it('requests one hourly snapshot for the last two days and the latest requests', async () => {
    mountDashboard()
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith({
      start_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      end_date: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/),
      granularity: 'hour',
      include_stats: true,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false
    })
    expect(listUsage).toHaveBeenCalledWith({ page: 1, page_size: 6 })
  })

  it('fills the last 24 hours from a sparse trend, anchored on the server time', async () => {
    getSnapshotV2.mockResolvedValue({
      generated_at: '2026-08-30T01:45:00Z',
      stats: { ...createDashboardStats(), rpm: 42, today_requests: 4286 },
      trend: [
        { date: '2026-08-30T01:00:00Z', requests: 946, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, cost: 0, actual_cost: 0 },
        { date: '2026-08-29T23:00:00Z', requests: 792, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, cost: 0, actual_cost: 0 },
        { date: '2026-08-28T23:00:00Z', requests: 5, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, cost: 0, actual_cost: 0 },
      ]
    })

    const wrapper = mountDashboard()
    await flushPromises()

    const chart = wrapper.getComponent(TrendChartStub)
    const values = chart.props('series')[0].values as number[]
    expect(values).toHaveLength(24)
    expect(chart.props('labels')).toHaveLength(24)
    // The newest hour is last; the hour with no traffic between them is zero; older data is dropped.
    expect(values.slice(-3)).toEqual([792, 0, 946])
    expect(values.reduce((sum, value) => sum + value, 0)).toBe(792 + 946)
    expect(wrapper.text()).toContain('4,286')
    expect(wrapper.text()).toContain('admin.viz.status.serving')
  })

  it('keeps accounts visible when the snapshot fails, and retries only the snapshot', async () => {
    getSnapshotV2.mockRejectedValueOnce(new Error('snapshot unavailable'))
    listAccounts.mockResolvedValue({
      items: [createAccount(1, 'Persisted capacity account')],
      total: 1,
      page: 1,
      page_size: 1000,
      pages: 1
    })

    const wrapper = mountDashboard()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('admin.dashboard.failedToLoad')
    expect(wrapper.get('[data-testid="dashboard-load-error"]').text()).toContain('admin.dashboard.failedToLoad')
    expect(wrapper.get('[data-testid="overview-state-active"]').text()).toContain('1')

    await wrapper.get('[data-testid="dashboard-load-error"] button').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(2)
    expect(listAccounts).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-testid="dashboard-load-error"]').exists()).toBe(false)
  })

  it('loads every persisted account and its passive capacity snapshot', async () => {
    listAccounts
      .mockResolvedValueOnce({
        items: [createAccount(1, 'First persisted account')],
        total: 2,
        page: 1,
        page_size: 1000,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: [createAccount(2, 'Second persisted account', { platform: 'gemini' })],
        total: 2,
        page: 2,
        page_size: 1000,
        pages: 2
      })

    const wrapper = mountDashboard()
    await flushPromises()

    expect(listAccounts).toHaveBeenNthCalledWith(1, 1, 1000, { include_scheduler_score: '0' })
    expect(listAccounts).toHaveBeenNthCalledWith(2, 2, 1000, { include_scheduler_score: '0' })
    expect(getBatchUsage).toHaveBeenCalledWith([1, 2], false)
    // One capacity ring per provider that has accounts.
    expect(wrapper.findAll('.ring-stub').map((ring) => ring.text().split(':')[0])).toEqual(['OpenAI', 'Gemini'])
  })

  it('counts account states, links them to Accounts, and lists problems with errors first', async () => {
    listAccounts.mockResolvedValue({
      items: [
        createAccount(1, 'Ready account'),
        createAccount(2, 'Rate limited', {
          rate_limited_at: '2099-01-01T00:00:00Z',
          rate_limit_reset_at: '2099-01-01T01:00:00Z',
        }),
        createAccount(3, 'Quota limited'),
        createAccount(4, 'Broken account', {
          status: 'error',
          schedulable: false,
          error_message: 'Authentication failed',
        }),
        createAccount(5, 'Disabled account', { status: 'inactive', schedulable: false }),
      ],
      total: 5,
      page: 1,
      page_size: 1000,
      pages: 1,
    })
    getBatchUsage.mockResolvedValue({
      usage: {
        '3': {
          updated_at: '2026-08-29T00:00:00Z',
          five_hour: { utilization: 100, resets_at: '2099-01-01T01:00:00Z', remaining_seconds: 3600 },
          seven_day: null,
          seven_day_sonnet: null,
        },
      },
      errors: {},
    })

    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.get('[data-testid="overview-state-active"]').text()).toBe('1 admin.viz.states.active')
    expect(wrapper.get('[data-testid="overview-state-limited"]').text()).toBe('2 admin.viz.states.limited')
    expect(wrapper.get('[data-testid="overview-state-error"]').text()).toBe('1 admin.viz.states.error')

    const links = wrapper.findAllComponents(RouterLinkStub)
    expect(links.map((link) => link.props('to'))).toEqual(expect.arrayContaining([
      { path: '/admin/accounts', query: { operator_status: 'active' } },
      { path: '/admin/accounts', query: { operator_status: 'limited' } },
      { path: '/admin/accounts', query: { operator_status: 'error' } },
    ]))

    // The disabled account is a choice, not a problem, so it is not listed.
    const problems = wrapper.findAll('.overview-attention-item').map((item) => item.text())
    expect(problems).toHaveLength(3)
    expect(problems[0]).toContain('Broken account')
    expect(problems[0]).toContain('Authentication failed')
    expect(problems.join(' ')).not.toContain('Disabled account')
    expect(problems.join(' ')).not.toContain('Ready account')
  })

  it('shows an account-list failure and recovers on retry', async () => {
    listAccounts
      .mockRejectedValueOnce(new Error('accounts unavailable'))
      .mockResolvedValueOnce({
        items: [createAccount(3, 'Recovered capacity account')],
        total: 1,
        page: 1,
        page_size: 1000,
        pages: 1
      })

    const wrapper = mountDashboard()
    await flushPromises()

    expect(wrapper.get('[data-testid="overview-fleet-error"]').text()).toContain('admin.viz.loadFailed')

    await wrapper.get('[data-testid="overview-fleet-error"] button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="overview-fleet-error"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="overview-state-active"]').text()).toContain('1')
    expect(getBatchUsage).toHaveBeenCalledWith([3], false)
  })

  it('lists the latest requests with their account and totals', async () => {
    listUsage.mockResolvedValue({
      items: [{
        id: 9,
        model: 'claude-sonnet-4-5',
        account: { id: 4, name: 'Claude Max Primary' },
        input_tokens: 1000,
        output_tokens: 200,
        cache_creation_tokens: 30,
        cache_read_tokens: 4,
        actual_cost: 0.0123,
        duration_ms: 1280,
        created_at: '2026-08-30T01:44:00Z',
      }],
      total: 1,
      page: 1,
      page_size: 6,
      pages: 1,
    })

    const wrapper = mountDashboard()
    await flushPromises()

    const row = wrapper.get('.overview-table tbody tr').text()
    expect(row).toContain('claude-sonnet-4-5')
    expect(row).toContain('Claude Max Primary')
    expect(row).toContain('1,234')
    expect(row).toContain('1.28s')
    expect(row).toContain('$0.0123')
    expect(row).toContain('1m')
  })
})
