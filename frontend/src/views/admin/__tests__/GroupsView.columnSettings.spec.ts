import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminGroup } from '@/types'
import GroupsView from '../GroupsView.vue'

const {
  listGroups,
  getAllGroups,
  getModelsListCandidates,
  getModelAllowlistCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
  listAccounts,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getAllGroups: vi.fn(),
  getModelsListCandidates: vi.fn(),
  getModelAllowlistCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
  listAccounts: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

const messages: Record<string, string> = {
  'admin.groups.columnSettings': 'Column Settings',
  'admin.groups.columns.name': 'Name',
  'admin.groups.columns.id': 'ID',
  'admin.groups.columns.platform': 'Platform',
  'admin.groups.columns.billingType': 'Billing Type',
  'admin.groups.columns.rateMultiplier': 'Rate Multiplier',
  'admin.groups.columns.type': 'Type',
  'admin.groups.columns.accounts': 'Accounts',
  'admin.groups.columns.capacity': 'Capacity',
  'admin.groups.columns.usage': 'Usage',
  'admin.groups.columns.status': 'Status',
  'admin.groups.columns.actions': 'Actions',
  'admin.groups.usageToday': 'Today',
  'admin.groups.usageYesterday': 'Yesterday',
  'admin.groups.usageTotal': 'Total',
}

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      getAll: getAllGroups,
      getAllIncludingInactive: getAllGroups,
      getModelsListCandidates,
      getModelAllowlistCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      updateSortOrder: vi.fn(),
    },
    accounts: {
      list: listAccounts,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const createGroup = (overrides: Partial<AdminGroup> = {}): AdminGroup => ({
  id: 1,
  name: 'Core Anthropic',
  description: null,
  platform: 'anthropic',
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: 'active',
  subscription_type: 'standard',
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  allow_image_generation: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  default_mapped_model: '',
  messages_dispatch_model_config: undefined,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-07-01T00:00:00Z',
  updated_at: '2026-07-01T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 3,
  active_account_count: 2,
  rate_limited_account_count: 1,
  models_list_config: undefined,
  sort_order: 10,
  ...overrides,
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const SelectStub = {
  props: ['modelValue', 'options', 'placeholder'],
  emits: ['update:modelValue', 'change'],
  template: `
    <select
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value); $emit('change')"
    >
      <option v-for="option in options" :key="String(option.value)" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `,
}

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const mountView = async () => {
  const wrapper = mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        PlatformIcon: true,
        Icon: IconStub,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        VueDraggable: { template: '<div><slot /></div>' },
      },
    },
  })
  await flushPromises()
  return wrapper
}

const columnKeys = (wrapper: ReturnType<typeof mount>) =>
  wrapper.findAll('thead th').map((th) => th.attributes('data-col'))

const openColumnSettings = async (wrapper: ReturnType<typeof mount>) => {
  await wrapper.get('[data-testid="group-view-menu"]').trigger('click')
  await flushPromises()
}

const clickColumnToggle = async (_wrapper: ReturnType<typeof mount>, key: string) => {
  const button = document.body.querySelector<HTMLButtonElement>(`[data-testid="group-column-${key}"]`)
  expect(button, `column toggle ${key}`).toBeTruthy()
  button!.click()
  await flushPromises()
}

describe('admin GroupsView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listGroups.mockReset()
    getAllGroups.mockReset()
    getModelsListCandidates.mockReset()
    getModelAllowlistCandidates.mockReset()
    getUsageSummary.mockReset()
    getCapacitySummary.mockReset()
    listAccounts.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    listGroups.mockResolvedValue({
      items: [createGroup()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getAllGroups.mockResolvedValue([])
    getModelsListCandidates.mockResolvedValue([])
    getModelAllowlistCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getLiveCapability.mockResolvedValue({ supported: false })
    listAccounts.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
  })

  afterEach(() => {
    localStorage.clear()
    document.body.innerHTML = ''
  })

  it('shows the route view by default and hides detail columns', async () => {
    const wrapper = await mountView()

    expect(columnKeys(wrapper)).toEqual([
      'name',
      'account_count',
      'capacity',
      'billing_type',
      'rate_multiplier',
      'actions',
    ])
    expect(localStorage.getItem('group-hidden-columns')).toBe(
      JSON.stringify(['id', 'platform', 'is_exclusive', 'usage', 'status']),
    )
    expect(localStorage.getItem('group-column-settings-version')).toBe('4')
  })

  it('applies saved hidden columns on mount and ignores unknown keys', async () => {
    localStorage.setItem(
      'group-hidden-columns',
      JSON.stringify(['usage', 'capacity', 'removed_column', 'name', 'actions']),
    )
    localStorage.setItem('group-column-settings-version', '4')

    const wrapper = await mountView()

    expect(columnKeys(wrapper)).toEqual([
      'name',
      'id',
      'platform',
      'account_count',
      'billing_type',
      'rate_multiplier',
      'is_exclusive',
      'status',
      'actions',
    ])
  })

  it('auto-hides newly default-hidden columns for existing saved prefs after version bumps', async () => {
    localStorage.setItem('group-hidden-columns', JSON.stringify([]))
    // No version key means version 1, so migrations 2 through 4 all apply.

    const wrapper = await mountView()

    expect(columnKeys(wrapper)).toEqual([
      'name',
      'account_count',
      'capacity',
      'billing_type',
      'rate_multiplier',
      'actions',
    ])
    expect(JSON.parse(localStorage.getItem('group-hidden-columns')!)).toEqual(
      expect.arrayContaining(['id', 'usage', 'platform', 'is_exclusive', 'status']),
    )
    expect(localStorage.getItem('group-column-settings-version')).toBe('4')
  })

  it('toggles a column from the view menu and persists hidden column keys', async () => {
    const wrapper = await mountView()

    await openColumnSettings(wrapper)
    await clickColumnToggle(wrapper, 'usage')

    expect(columnKeys(wrapper)).toEqual([
      'name',
      'account_count',
      'capacity',
      'billing_type',
      'rate_multiplier',
      'usage',
      'actions',
    ])
    expect(localStorage.getItem('group-hidden-columns')).toBe(
      JSON.stringify(['id', 'platform', 'is_exclusive', 'status']),
    )
  })

  it('can show the id column from the view menu', async () => {
    const wrapper = await mountView()

    await openColumnSettings(wrapper)
    await clickColumnToggle(wrapper, 'id')

    expect(columnKeys(wrapper)).toEqual([
      'name',
      'id',
      'account_count',
      'capacity',
      'billing_type',
      'rate_multiplier',
      'actions',
    ])
    expect(localStorage.getItem('group-hidden-columns')).toBe(
      JSON.stringify(['platform', 'is_exclusive', 'usage', 'status']),
    )
  })

  it('skips usage and capacity fetches until consuming columns are shown', async () => {
    localStorage.setItem(
      'group-hidden-columns',
      JSON.stringify(['billing_type', 'usage', 'capacity']),
    )
    localStorage.setItem('group-column-settings-version', '4')

    const wrapper = await mountView()

    expect(getUsageSummary).not.toHaveBeenCalled()
    expect(getCapacitySummary).not.toHaveBeenCalled()

    await openColumnSettings(wrapper)
    await clickColumnToggle(wrapper, 'usage')
    expect(getUsageSummary).toHaveBeenCalledTimes(1)
    expect(getUsageSummary).toHaveBeenCalledWith()
    expect(getCapacitySummary).not.toHaveBeenCalled()

    await clickColumnToggle(wrapper, 'capacity')
    expect(getUsageSummary).toHaveBeenCalledTimes(1)
    expect(getCapacitySummary).toHaveBeenCalledTimes(1)
  })

  it('renders yesterday usage between today and total', async () => {
    getUsageSummary.mockResolvedValue([
      { group_id: 1, today_cost: 1.25, yesterday_cost: 2.5, total_cost: 9.75 },
    ])

    localStorage.setItem('group-hidden-columns', JSON.stringify([]))
    localStorage.setItem('group-column-settings-version', '4')
    const wrapper = await mountView()
    const text = wrapper.get('td[data-col="usage"]').text()

    expect(text).toContain('Today$1.25')
    expect(text).toContain('Yesterday$2.50')
    expect(text).toContain('Total$9.75')
    expect(text.indexOf('Today')).toBeLessThan(text.indexOf('Yesterday'))
    expect(text.indexOf('Yesterday')).toBeLessThan(text.indexOf('Total'))
  })

  it('filters by provider chip and opens the route drawer with member accounts', async () => {
    getAllGroups.mockResolvedValue([
      createGroup(),
      createGroup({ id: 2, name: 'Codex', platform: 'openai' }),
    ])
    listAccounts.mockResolvedValue({
      items: [
        { id: 7, name: 'Claude Primary', platform: 'anthropic', status: 'active', schedulable: true, group_ids: [1], rate_limit_reset_at: null, overload_until: null, temp_unschedulable_until: null },
        { id: 8, name: 'Other Group', platform: 'anthropic', status: 'active', schedulable: true, group_ids: [9], rate_limit_reset_at: null, overload_until: null, temp_unschedulable_until: null },
      ],
      total: 2,
      page: 1,
      page_size: 100,
      pages: 1,
    })

    const wrapper = await mountView()

    const chips = wrapper.findAll('[data-testid^="group-platform-"]')
    expect(chips.map((chip) => chip.attributes('data-testid'))).toEqual([
      'group-platform-anthropic',
      'group-platform-openai',
    ])
    await chips[1].trigger('click')
    await flushPromises()
    expect(listGroups).toHaveBeenLastCalledWith(
      1,
      expect.any(Number),
      expect.objectContaining({ platform: 'openai' }),
      expect.anything(),
    )
    expect(chips[1].attributes('aria-pressed')).toBe('true')

    await wrapper.get('[data-testid="group-row-1"]').trigger('click')
    await flushPromises()

    expect(listAccounts).toHaveBeenCalledWith(1, 100, { group: '1', lite: '1' })
    const map = document.body.querySelector('[data-testid="group-route-map"]')
    expect(map?.textContent).toContain('Claude Primary')
    expect(map?.textContent).not.toContain('Other Group')
    expect(getUsageSummary).toHaveBeenCalled()
  })
})
