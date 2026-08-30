import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { routeLocationKey } from 'vue-router'

import OperatorCapacityOverview from '@/components/admin/OperatorCapacityOverview.vue'
import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchUsage,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchUsage: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchUsage,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
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

// Render the per-column header slots so we can assert the usage-window header hint.
const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div data-test="table-rows">{{ data.map(row => row.name).join(',') }}</div>
      <template v-for="column in columns" :key="column.key">
        <div v-if="column.key === 'usage'" data-test="usage-header">
          <slot :name="'header-' + column.key" :column="column" />
        </div>
        <div v-if="column.key === 'upstream_billing_rate'" data-test="upstream-billing-header">
          <slot :name="'header-' + column.key" :column="column" />
        </div>
      </template>
      <div v-for="row in data" :key="row.id" data-test="account-rate">
        <slot name="cell-rate_multiplier" :row="row" />
      </div>
    </div>
  `
}

// Expose the content passed to HelpTooltip without dealing with its <Teleport>.
const HelpTooltipStub = {
  props: ['content', 'widthClass'],
  template: '<span data-test="usage-windows-hint">{{ content }}</span>'
}

function mountView(attachTo?: Element, query: Record<string, string> = {}) {
  return mount(AccountsView, {
    attachTo,
    global: {
      provide: {
        [routeLocationKey as symbol]: { query },
      },
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        HelpTooltip: HelpTooltipStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: {
          props: ['groups'],
          template: '<div data-test="account-filters" :data-group-count="groups.length"></div>'
        },
        AccountBulkActionsBar: { template: '<div data-test="bulk-actions"></div>' },
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
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true,
        RouterLink: true,
      }
    }
  })
}

describe('admin AccountsView usage windows hint', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchUsage.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchUsage.mockResolvedValue({ usage: {}, errors: {} })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('keeps groups available when loading proxies fails', async () => {
    getAllProxies.mockRejectedValue(new Error('proxy service unavailable'))
    getAllGroups.mockResolvedValue([{ id: 7, name: 'production' }])

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="account-filters"]').attributes('data-group-count')).toBe('1')
  })

  it('summarizes every filtered account while the technical table remains paginated', async () => {
    const makeAccount = (id: number, name: string, platform: 'openai' | 'gemini') => ({
      id,
      name,
      platform,
      type: platform === 'openai' ? 'oauth' : 'apikey',
      status: 'active',
      schedulable: true,
      created_at: '2026-08-25T00:00:00Z',
      updated_at: '2026-08-26T00:00:00Z',
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      error_message: null,
      last_used_at: null,
      expires_at: null,
      auto_pause_on_expired: false,
      rate_limited_at: null,
      rate_limit_reset_at: null,
      overload_until: null,
      temp_unschedulable_until: null,
      temp_unschedulable_reason: null,
      session_window_start: null,
      session_window_end: null,
      session_window_status: null,
    })
    const tableAccount = makeAccount(1, 'Current table page', 'openai')
    const reserveAccount = makeAccount(2, 'OpenAI reserve fleet row', 'openai')
    const geminiAccount = makeAccount(3, 'Gemini fleet row', 'gemini')

    listAccounts.mockImplementation(async (page: number, pageSize: number) => {
      if (pageSize !== 1000) {
        return { items: [tableAccount], total: 3, page: 1, page_size: pageSize, pages: 3 }
      }
      return page === 1
        ? { items: [tableAccount, reserveAccount], total: 3, page: 1, page_size: pageSize, pages: 2 }
        : { items: [geminiAccount], total: 3, page: 2, page_size: pageSize, pages: 2 }
    })
    getBatchUsage.mockResolvedValue({
      usage: {
        '1': { updated_at: null, five_hour: null, seven_day: null, seven_day_sonnet: null },
        '2': { updated_at: null, five_hour: null, seven_day: null, seven_day_sonnet: null },
        '3': { updated_at: null, five_hour: null, seven_day: null, seven_day_sonnet: null },
      },
      errors: {},
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="table-rows"]').text()).toBe('Current table page')
    expect(wrapper.text()).toContain('OpenAI reserve fleet row')
    expect(wrapper.text()).toContain('Gemini fleet row')
    expect(wrapper.findAll('.operator-capacity-provider-name').map((node) => node.text())).toEqual([
      'OpenAI',
      'Gemini',
    ])
    expect(listAccounts).toHaveBeenCalledWith(1, 1000, {
      platform: '',
      type: '',
      status: '',
      group: '',
      privacy_mode: '',
      search: '',
    })
    expect(listAccounts).toHaveBeenCalledWith(2, 1000, expect.any(Object))
    expect(getBatchUsage).toHaveBeenCalledWith([1, 2, 3], false)

    const capacityPanel = wrapper.get('#account-capacity-panel')
    const technicalPanel = wrapper.get('#account-technical-panel')
    expect(capacityPanel.attributes('style')).toBeUndefined()
    expect(technicalPanel.attributes('style')).toContain('display: none')
    expect(wrapper.find('details.operator-account-details').exists()).toBe(false)

    const listCalls = listAccounts.mock.calls.length
    const usageCalls = getBatchUsage.mock.calls.length
    await wrapper.get('#account-technical-tab').trigger('click')
    expect(capacityPanel.attributes('style')).toContain('display: none')
    expect(technicalPanel.attributes('style')).not.toContain('display: none')
    expect(technicalPanel.find('[data-test="bulk-actions"]').exists()).toBe(true)
    expect(technicalPanel.find('[data-test="data-table"]').exists()).toBe(true)
    expect(listAccounts).toHaveBeenCalledTimes(listCalls)
    expect(getBatchUsage).toHaveBeenCalledTimes(usageCalls)
  })

  it('hydrates the operator status filter from a dashboard query link', async () => {
    const wrapper = mountView(undefined, { operator_status: 'limited' })
    await flushPromises()

    expect(wrapper.getComponent(OperatorCapacityOverview).props('status')).toBe('limited')
  })

  it('renders an explanatory tooltip next to the usage windows column header', async () => {
    const wrapper = mountView()
    await flushPromises()

    const header = wrapper.find('[data-test="usage-header"]')
    expect(header.exists()).toBe(true)
    // Column label is still shown alongside the help icon.
    expect(header.text()).toContain('admin.accounts.columns.usageWindows')

    const hint = wrapper.find('[data-test="usage-windows-hint"]')
    expect(hint.exists()).toBe(true)
    expect(hint.text()).toBe('admin.accounts.usageWindowsHint')
  })

  it('closes the account tools dropdown on Escape and restores trigger focus', async () => {
    const host = document.createElement('div')
    document.body.append(host)
    const wrapper = mountView(host)
    await flushPromises()

    const trigger = wrapper.get('button[title="admin.accounts.moreActions"]')
    ;(trigger.element as HTMLButtonElement).focus()
    await trigger.trigger('click')
    await flushPromises()

    const menu = document.body.querySelector<HTMLElement>('.account-tools-menu')
    expect(menu).not.toBeNull()
    menu?.querySelector<HTMLElement>('button')?.focus()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()

    expect(document.body.querySelector('.account-tools-menu')).toBeNull()
    expect(document.activeElement).toBe(trigger.element)
    wrapper.unmount()
    host.remove()
  })

  it('keeps Ollama Cloud in the single usage column and ignores legacy column preferences', async () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['ollama_cloud_usage']))
    const wrapper = mountView()
    await flushPromises()

    const columns = wrapper.getComponent(DataTableStub).props('columns') as Array<{ key: string }>
    expect(columns.filter(column => column.key === 'usage')).toHaveLength(1)
    expect(columns.some(column => column.key === 'ollama_cloud_usage')).toBe(false)
  })

  it('renders the upstream billing trust warning next to the declared-rate column', async () => {
    const wrapper = mountView()
    await flushPromises()

    const header = wrapper.find('[data-test="upstream-billing-header"]')
    expect(header.exists()).toBe(true)
    expect(header.text()).toContain('admin.accounts.columns.upstreamBillingRate')
    expect(wrapper.findAll('[data-test="usage-windows-hint"]').some(node =>
      node.text() === 'admin.accounts.upstreamBilling.trustWarning'
    )).toBe(true)
    const columns = wrapper.getComponent(DataTableStub).props('columns') as Array<{ key: string; sortable: boolean }>
    expect(columns.find(column => column.key === 'upstream_billing_rate')?.sortable).toBe(true)
  })

  it('shows account multipliers with enough precision to match declared rates', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [{
        id: 7,
        name: 'precision-account',
        platform: 'gemini',
        type: 'apikey',
        status: 'active',
        schedulable: true,
        rate_multiplier: 0.065,
        extra: {
          upstream_billing_probe_enabled: true,
          upstream_billing_rate_sync_enabled: true
        },
        created_at: '2026-07-13T00:00:00Z',
        updated_at: '2026-07-13T00:00:00Z'
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="account-rate"]').text()).toBe('0.065x')
    const indicator = wrapper.get('[data-testid="account-rate-sync-indicator"]')
    expect(indicator.attributes('title')).toBe('admin.accounts.upstreamBilling.syncedRateTooltip')
  })
})
