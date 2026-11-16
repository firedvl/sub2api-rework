import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { routeLocationKey, routerKey } from 'vue-router'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchUsage,
  getBatchTodayStats,
  getById,
  getStats
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchUsage: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getById: vi.fn(),
  getStats: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchUsage,
      getBatchTodayStats,
      getById,
      getStats,
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      toggleSchedulable: vi.fn()
    },
    proxies: { getAll: vi.fn().mockResolvedValue([]) },
    groups: { getAll: vi.fn().mockResolvedValue([]) }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn(), showInfo: vi.fn() })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ token: 'test-token' })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const makeAccount = (id: number, name: string) => ({
  id,
  name,
  platform: 'openai',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z'
})

const DrawerStub = {
  name: 'AccountDetailDrawer',
  props: ['open', 'row'],
  emits: ['close'],
  template: `
    <div data-test="drawer" :data-open="String(open)" :data-account="row ? row.account.name : ''">
      <button data-test="drawer-close" @click="$emit('close')">close</button>
    </div>
  `
}

const FiltersStub = {
  props: ['searchQuery'],
  template: '<div data-test="filters" :data-search="searchQuery"><slot /></div>'
}

function mountView(query: Record<string, string>) {
  const route = reactive({ query: { ...query } as Record<string, string | undefined> })
  const router = {
    replace: vi.fn(async ({ query: next }: { query: Record<string, string | undefined> }) => {
      route.query = next
    })
  }
  const wrapper = mount(AccountsView, {
    global: {
      provide: {
        [routeLocationKey as symbol]: route,
        [routerKey as symbol]: router
      },
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DataTable: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: true,
        AccountTableFilters: FiltersStub,
        AccountBulkActionsBar: true,
        AccountDetailDrawer: DrawerStub,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        Icon: true
      }
    }
  })
  return { wrapper, route, router }
}

describe('admin AccountsView command palette deep links', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    for (const mock of [listAccounts, listWithEtag, getBatchUsage, getBatchTodayStats, getById, getStats]) mock.mockReset()

    const accounts = [makeAccount(1, 'alpha'), makeAccount(2, 'beta')]
    listAccounts.mockImplementation(async (page: number, pageSize: number) => ({
      items: accounts,
      total: accounts.length,
      page,
      page_size: pageSize,
      pages: 1
    }))
    listWithEtag.mockResolvedValue({ notModified: true, etag: null, data: null })
    getBatchUsage.mockResolvedValue({ usage: {}, errors: {} })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getStats.mockResolvedValue({ history: [] })
  })

  it('pre-fills and applies ?search', async () => {
    const { wrapper } = mountView({ search: 'alpha' })
    await flushPromises()

    expect(wrapper.get('[data-test="filters"]').attributes('data-search')).toBe('alpha')
    expect(listAccounts).toHaveBeenCalledWith(1, 1000, expect.objectContaining({ search: 'alpha' }))
  })

  it('opens the drawer for ?account once the list loads and drops the param on close', async () => {
    const { wrapper, route, router } = mountView({ account: '2', view: 'capacity' })
    await flushPromises()

    const drawer = wrapper.get('[data-test="drawer"]')
    expect(drawer.attributes('data-open')).toBe('true')
    expect(drawer.attributes('data-account')).toBe('beta')
    expect(getById).not.toHaveBeenCalled()

    await wrapper.get('[data-test="drawer-close"]').trigger('click')
    await flushPromises()

    expect(router.replace).toHaveBeenCalledWith({ query: { account: undefined, view: 'capacity' } })
    expect(route.query.view).toBe('capacity')
    expect(wrapper.get('[data-test="drawer"]').attributes('data-open')).toBe('false')
  })

  it('loads an account outside the current results directly', async () => {
    getById.mockResolvedValue(makeAccount(9, 'filtered-out'))
    const { wrapper } = mountView({ account: '9' })
    await flushPromises()

    expect(getById).toHaveBeenCalledWith(9)
    expect(wrapper.get('[data-test="drawer"]').attributes('data-account')).toBe('filtered-out')
  })

  it('ignores an invalid ?account', async () => {
    const { wrapper } = mountView({ account: 'nope' })
    await flushPromises()

    expect(getById).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="drawer"]').attributes('data-open')).toBe('false')
  })
})
