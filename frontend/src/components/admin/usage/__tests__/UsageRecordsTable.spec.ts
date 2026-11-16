const ipGeoMocks = vi.hoisted(() => ({
  getEntry: vi.fn(() => ({ status: 'idle' as const })),
  fetchOne: vi.fn(),
  fetchBatch: vi.fn(),
}))
const appStoreMocks = vi.hoisted(() => ({ showSuccess: vi.fn(), showError: vi.fn() }))

vi.mock('@/utils/ipGeoLookup', () => ipGeoMocks)
vi.mock('@/stores/app', () => ({ useAppStore: () => appStoreMocks }))
vi.mock('@/utils/format', async () => {
  const actual = await vi.importActual<typeof import('@/utils/format')>('@/utils/format')
  return {
    ...actual,
    formatRelativeTime: () => '5m ago',
    formatDateTime: () => '2026-11-15 10:00:00',
  }
})
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageRecordsTable from '../UsageRecordsTable.vue'
import UsageRecordDrawer from '../UsageRecordDrawer.vue'
import { formatCost, latencySeverity, platformOf, routedModel } from '../usageRecord'

const row = (overrides: Record<string, unknown> = {}) => ({
  id: 7,
  user_id: 3,
  api_key_id: 9,
  account_id: 11,
  request_id: 'req-123',
  upstream_request_id: 'up-456',
  model: 'claude-sonnet-4',
  upstream_model: 'claude-sonnet-4-20250514',
  input_tokens: 1200,
  output_tokens: 300,
  cache_creation_tokens: 0,
  cache_read_tokens: 500,
  cache_creation_5m_tokens: 0,
  cache_creation_1h_tokens: 0,
  input_cost: 0.0036,
  output_cost: 0.0045,
  cache_creation_cost: 0,
  cache_read_cost: 0.00015,
  total_cost: 0.00825,
  actual_cost: 0.00825,
  rate_multiplier: 1,
  account_rate_multiplier: 1,
  billing_type: 0,
  stream: true,
  native_compaction_v2: false,
  long_context_billing_applied: false,
  duration_ms: 2400,
  first_token_ms: 800,
  image_count: 0,
  image_input_tokens: 0,
  image_output_tokens: 0,
  image_input_cost: 0,
  image_output_cost: 0,
  cache_ttl_overridden: false,
  user_agent: 'curl/8',
  ip_address: null,
  usage_observation_complete: true,
  created_at: '2026-11-15T10:00:00Z',
  user: { email: 'ops@example.com' },
  account: { id: 11, name: 'acct-main' },
  ...overrides,
})

const columns = [
  { key: 'created_at', label: 'Time', sortable: true },
  { key: 'user', label: 'User' },
  { key: 'model', label: 'Model', sortable: true },
  { key: 'tokens', label: 'Tokens' },
  { key: 'cost', label: 'Cost' },
  { key: 'latency', label: 'Latency' },
  { key: 'request_id', label: 'Request ID' },
]

const mountTable = (data = [row()]) => mount(UsageRecordsTable, {
  props: { data: data as any, columns },
  global: { stubs: { ProviderMark: true, Icon: true, IpGeoCell: true } },
})

describe('UsageRecordsTable', () => {
  beforeEach(() => {
    appStoreMocks.showSuccess.mockReset()
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
  })

  it('right-aligns numeric columns and shows relative time with the exact time on hover', () => {
    const wrapper = mountTable()
    const headers = wrapper.findAll('thead th')
    expect(headers.map((th) => th.classes().includes('is-num'))).toEqual([false, false, false, true, true, true, false])
    const time = wrapper.get('tbody time')
    expect(time.text()).toBe('5m ago')
    expect(time.attributes('title')).toBe('2026-11-15 10:00:00')
    expect(wrapper.get('.usage-col-model').text()).toContain('claude-sonnet-4-20250514')
  })

  it('emits rowClick on click and Enter, but not from the user link or copy button', async () => {
    const wrapper = mountTable()
    const tr = wrapper.get('tbody tr')
    await tr.trigger('click')
    await tr.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('rowClick')).toHaveLength(2)

    await wrapper.get('.usage-user-link').trigger('click')
    expect(wrapper.emitted('userClick')?.[0]).toEqual([3, 'ops@example.com'])
    await wrapper.get('.usage-copy').trigger('click')
    await flushPromises()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('req-123')
    expect(wrapper.emitted('rowClick')).toHaveLength(2)
  })

  it('toggles server-side sort from the header buttons', async () => {
    const wrapper = mountTable()
    const sortButtons = wrapper.findAll('button.ui-sort')
    expect(sortButtons).toHaveLength(2)
    await sortButtons[0].trigger('click')
    expect(wrapper.emitted('sort')?.[0]).toEqual(['created_at', 'asc'])
    await sortButtons[1].trigger('click')
    expect(wrapper.emitted('sort')?.[1]).toEqual(['model', 'desc'])
  })

  it('keeps the observation label for incomplete records', () => {
    const wrapper = mountTable([row({ usage_observation_complete: false })])
    expect(wrapper.get('[data-testid="usage-observation-label"]').text()).toBe('usage.observationPartial')
  })
})

describe('UsageRecordDrawer', () => {
  it('shows the cost breakdown and copies IDs', async () => {
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
    const wrapper = mount(UsageRecordDrawer, {
      props: { open: true, record: row() as any },
      global: { stubs: { ProviderMark: true, Icon: true, IpGeoCell: true, Teleport: true, Transition: false } },
    })
    const text = wrapper.text()
    expect(text).toContain('usage.costDetails')
    expect(text).toContain('$0.00825000')
    expect(text).toContain('usage.accountBilled')
    expect(text).toContain('req-123')
    expect(text).toContain('up-456')

    const copyButtons = wrapper.findAll('.usage-drawer-copy')
    expect(copyButtons).toHaveLength(3)
    await copyButtons[1].trigger('click')
    await flushPromises()
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('req-123')
  })
})

describe('usageRecord helpers', () => {
  it('derives provider, routed model, cost and latency tone', () => {
    expect(platformOf(row() as any)).toBe('anthropic')
    expect(platformOf(row({ group: { platform: 'openai' } }) as any)).toBe('openai')
    expect(routedModel(row({ model_mapping_chain: 'a→b→c' }) as any)).toBe('c')
    expect(formatCost(0)).toBe('$0')
    expect(formatCost(1.5)).toBe('$1.5000')
    expect(latencySeverity({ first_token_ms: 70_000, duration_ms: 1000 })).toBe('critical')
  })
})
