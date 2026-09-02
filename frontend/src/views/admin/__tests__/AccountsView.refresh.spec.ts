import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'
import type { Account, AccountUsageInfo } from '@/types'

const {
  listAccounts,
  listWithEtag,
  getBatchUsage,
  getBatchTodayStats,
  getUpstreamBillingProbeSettings,
  getAllProxies,
  getAllGroups,
  showError,
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchUsage: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getUpstreamBillingProbeSettings: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchUsage,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
    },
    proxies: { getAll: getAllProxies },
    groups: { getAll: getAllGroups },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess: vi.fn(), showInfo: vi.fn() }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (
        key === 'admin.accounts.autoRefreshCountdown'
          ? `Auto refresh: ${params?.seconds}s`
          : key
      ),
    }),
  }
})

const makeAccount = (overrides: Partial<Account> = {}): Account => ({
  id: 1,
  name: 'Primary account',
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
  current_concurrency: 0,
  current_window_cost: 0,
  active_sessions: 0,
  credentials: {},
  extra: {},
  group_ids: [],
  groups: [],
  ...overrides,
})

const makeUsage = (utilization: number): AccountUsageInfo => ({
  updated_at: '2026-08-25T00:00:00Z',
  five_hour: {
    utilization,
    resets_at: '2026-08-25T05:00:00Z',
    remaining_seconds: 3600,
  },
  seven_day: null,
  seven_day_sonnet: null,
})

const AccountTableActionsStub = {
  props: ['loading'],
  emits: ['refresh'],
  template: `
    <div>
      <button data-test="manual-refresh" :disabled="loading" @click="$emit('refresh')">refresh</button>
      <slot name="after" />
    </div>
  `,
}

const mountedWrappers: VueWrapper[] = []
const mountView = () => {
  const wrapper = mount(AccountsView, {
    global: {
      stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      DataTable: {
        props: ['data'],
        template: '<div data-test="technical-rows"><span v-for="row in data" :key="row.id">{{ row.name }}</span></div>',
      },
      AccountTableActions: AccountTableActionsStub,
      AccountTableFilters: true,
      AccountBulkActionsBar: true,
      AccountActionMenu: true,
      Pagination: true,
      ConfirmDialog: true,
      HelpTooltip: true,
      EmptyState: true,
      CreateAccountModal: true,
      EditAccountModal: true,
      BulkEditAccountModal: true,
      SyncFromCrsModal: true,
      TempUnschedStatusModal: true,
      ImportDataModal: true,
      ReAuthAccountModal: true,
      AccountTestModal: true,
      AccountStatsModal: true,
      ScheduledTestsPanel: true,
      ErrorPassthroughRulesModal: true,
      TLSFingerprintProfilesModal: true,
      TotpStepUpDialog: true,
      AccountCapacityCell: true,
      AccountStatusIndicator: true,
      AccountTodayStatsCell: true,
      AccountGroupsCell: true,
      AccountUsageCell: true,
      UpstreamBillingRateCell: true,
      PlatformTypeBadge: true,
      LoadingSpinner: true,
      ProviderIcon: true,
      Select: true,
      Icon: true,
        RouterLink: true,
      },
    },
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

let serverAccount: Account
let usageUtilization: number
let documentHidden = false

describe('admin AccountsView refresh behavior', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-25T00:00:00Z'))
    localStorage.clear()
    serverAccount = makeAccount()
    usageUtilization = 20
    documentHidden = false

    Object.defineProperty(document, 'hidden', {
      configurable: true,
      get: () => documentHidden,
    })
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      value: vi.fn(() => ({
        matches: true,
        media: '(min-width: 768px)',
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    })

    for (const mock of [
      listAccounts,
      listWithEtag,
      getBatchUsage,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings,
      getAllProxies,
      getAllGroups,
      showError,
    ]) mock.mockReset()

    listAccounts.mockImplementation(async (_page: number, pageSize: number) => ({
      items: [{ ...serverAccount }],
      total: 1,
      page: 1,
      page_size: pageSize,
      pages: 1,
    }))
    listWithEtag.mockResolvedValue({ notModified: true, etag: 'abc', data: null })
    getBatchUsage.mockImplementation(async () => ({
      usage: { '1': makeUsage(usageUtilization) },
      errors: {},
    }))
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getUpstreamBillingProbeSettings.mockResolvedValue({ enabled: true, interval_minutes: 30 })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  afterEach(() => {
    mountedWrappers.splice(0).forEach((wrapper) => wrapper.unmount())
    vi.useRealTimers()
  })

  it('replaces manual live data without remounting and prevents duplicate clicks', async () => {
    const wrapper = mountView()
    await flushPromises()
    const root = wrapper.element
    expect(wrapper.text()).toContain('80%')
    expect(wrapper.text()).toContain('Primary account')

    serverAccount = makeAccount({ name: 'Primary account refreshed', current_concurrency: 2 })
    usageUtilization = 46
    const listCallsBefore = listAccounts.mock.calls.length

    const refresh = wrapper.get('[data-test="manual-refresh"]')
    await refresh.trigger('click')
    await refresh.trigger('click')
    await flushPromises()

    expect(wrapper.element).toBe(root)
    expect(wrapper.text()).toContain('54%')
    expect(wrapper.text()).toContain('Primary account refreshed')
    expect(listAccounts.mock.calls.length - listCallsBefore).toBe(2)
    expect(getBatchUsage).toHaveBeenLastCalledWith([1], true)
    expect(refresh.attributes('disabled')).toBeUndefined()
  })

  it('updates live capacity after a 304 and resets the countdown only after the tick runs', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('80%')

    usageUtilization = 53
    await vi.advanceTimersByTimeAsync(4_000)
    expect(listWithEtag).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Auto refresh: 1s')

    await vi.advanceTimersByTimeAsync(1_000)
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalledTimes(1)
    expect(getBatchUsage).toHaveBeenLastCalledWith([1], false)
    expect(wrapper.text()).toContain('47%')
    expect(wrapper.text()).toContain('Auto refresh: 5s')
  })

  it('applies every interval change without accumulating timers across repeated toggles', async () => {
    const wrapper = mountView()
    await flushPromises()
    const trigger = wrapper.get('button[title="admin.accounts.autoRefresh"]')

    const openMenu = async () => {
      if (trigger.attributes('aria-expanded') !== 'true') await trigger.trigger('click')
    }
    const closeMenu = async () => {
      if (trigger.attributes('aria-expanded') === 'true') await trigger.trigger('click')
    }
    const chooseInterval = async (seconds: number) => {
      await openMenu()
      const option = wrapper.findAll('button').find((button) => (
        button.text() === `admin.accounts.refreshInterval${seconds}s`
      ))
      expect(option).toBeDefined()
      await option!.trigger('click')
      await closeMenu()
    }
    const toggleEnabled = async () => {
      await openMenu()
      const toggle = wrapper.findAll('button').find((button) => (
        button.text() === 'admin.accounts.enableAutoRefresh'
      ))
      expect(toggle).toBeDefined()
      await toggle!.trigger('click')
      await closeMenu()
    }

    await chooseInterval(5)
    await toggleEnabled()
    for (const seconds of [5, 10, 15, 30]) {
      if (seconds !== 5) await chooseInterval(seconds)
      const callsBefore = listWithEtag.mock.calls.length
      await vi.advanceTimersByTimeAsync((seconds - 1) * 1_000)
      expect(listWithEtag).toHaveBeenCalledTimes(callsBefore)
      await vi.advanceTimersByTimeAsync(1_000)
      await flushPromises()
      expect(listWithEtag).toHaveBeenCalledTimes(callsBefore + 1)
      expect(wrapper.text()).toContain(`Auto refresh: ${seconds}s`)
    }

    await toggleEnabled()
    await vi.advanceTimersByTimeAsync(60_000)
    const callsWhileDisabled = listWithEtag.mock.calls.length

    await toggleEnabled()
    await toggleEnabled()
    await toggleEnabled()
    await vi.advanceTimersByTimeAsync(30_000)
    await flushPromises()
    expect(listWithEtag).toHaveBeenCalledTimes(callsWhileDisabled + 1)
  })

  it('applies a changed list ETag and live usage without duplicating rows', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()

    serverAccount = makeAccount({ name: 'Structurally updated account', updated_at: '2026-08-25T00:01:00Z' })
    usageUtilization = 35
    listWithEtag.mockResolvedValueOnce({
      notModified: false,
      etag: 'def',
      data: { items: [{ ...serverAccount }], total: 1, page: 1, page_size: 20, pages: 1 },
    })

    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()

    expect(wrapper.findAll('.operator-capacity-account')).toHaveLength(1)
    expect(wrapper.text()).toContain('Structurally updated account')
    expect(wrapper.text()).toContain('65%')
  })

  it('pauses while hidden and catches up immediately when stale data becomes visible', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()

    documentHidden = true
    usageUtilization = 40
    await vi.advanceTimersByTimeAsync(12_000)
    expect(listWithEtag).not.toHaveBeenCalled()

    documentHidden = false
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()

    expect(listWithEtag).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('60%')
  })

  it('suppresses overlapping ticks, preserves old data on failure, and recovers later', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()

    let rejectRefresh!: (reason: Error) => void
    listWithEtag.mockImplementationOnce(() => new Promise((_resolve, reject) => {
      rejectRefresh = reject
    }))

    await vi.advanceTimersByTimeAsync(5_000)
    expect(listWithEtag).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(listWithEtag).toHaveBeenCalledTimes(1)

    rejectRefresh(new Error('temporary refresh failure'))
    await flushPromises()
    expect(wrapper.text()).toContain('80%')

    usageUtilization = 53
    await vi.advanceTimersByTimeAsync(5_000)
    await flushPromises()
    expect(listWithEtag).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('47%')
  })

  it('stops polling when disabled and after unmount', async () => {
    localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 }))
    const wrapper = mountView()
    await flushPromises()

    const autoRefreshTrigger = wrapper.get('button[title="admin.accounts.autoRefresh"]')
    await autoRefreshTrigger.trigger('click')
    await wrapper.get('button[aria-pressed="true"]').trigger('click')
    await vi.advanceTimersByTimeAsync(10_000)
    expect(listWithEtag).not.toHaveBeenCalled()

    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(10_000)
    expect(listWithEtag).not.toHaveBeenCalled()
  })

  it('keeps previous data and exposes the existing toast when manual refresh fails', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('80%')

    listAccounts.mockRejectedValue(new Error('account service unavailable'))
    await wrapper.get('[data-test="manual-refresh"]').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('80%')
    expect(showError).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="manual-refresh"]').attributes('disabled')).toBeUndefined()
  })
})
