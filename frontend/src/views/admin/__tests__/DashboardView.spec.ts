import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import type { Account, DashboardStats } from '@/types'
import OperatorCapacityOverview from '@/components/admin/OperatorCapacityOverview.vue'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking, listAccounts, getBatchUsage, showError } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn(),
  listAccounts: vi.fn(),
  getBatchUsage: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking
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
      t: (key: string) => key
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

const mountDashboard = () => mount(DashboardView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      LoadingSpinner: true,
      Icon: true,
    }
  }
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()
    listAccounts.mockReset()
    getBatchUsage.mockReset()
    showError.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 1000, pages: 0 })
    getBatchUsage.mockResolvedValue({ usage: {}, errors: {} })
  })

  it('requests only the fleet-wide snapshot data owned by Overview', async () => {
    mountDashboard()

    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith({
      include_stats: true,
      include_trend: false,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false
    })
    expect(getUserUsageTrend).not.toHaveBeenCalled()
    expect(getUserSpendingRanking).not.toHaveBeenCalled()
  })

  it('reports a snapshot load failure', async () => {
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
    expect(wrapper.getComponent(OperatorCapacityOverview).props('accounts')).toEqual([
      expect.objectContaining({ name: 'Persisted capacity account' })
    ])

    await wrapper.get('[data-testid="dashboard-load-error"] button').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(2)
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
    expect(wrapper.getComponent(OperatorCapacityOverview).props('accounts')).toEqual([
      expect.objectContaining({ name: 'First persisted account' }),
      expect.objectContaining({ name: 'Second persisted account' })
    ])
    expect(getBatchUsage).toHaveBeenCalledWith([1, 2], false)
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

    expect(wrapper.get('[data-testid="capacity-load-error"]').text()).toContain('admin.dashboard.capacity.loadFailed')
    expect(wrapper.text()).not.toContain('admin.dashboard.capacity.empty')

    await wrapper.get('[data-testid="capacity-load-error"] button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="capacity-load-error"]').exists()).toBe(false)
    expect(wrapper.getComponent(OperatorCapacityOverview).props('accounts')).toEqual([
      expect.objectContaining({ name: 'Recovered capacity account' })
    ])
    expect(getBatchUsage).toHaveBeenCalledWith([3], false)
  })
})
