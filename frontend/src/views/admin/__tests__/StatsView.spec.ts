import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import type { TrendDataPoint } from '@/types'
import StatsView from '../StatsView.vue'

const api = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getApiKeyUsageTrend: vi.fn(),
  getUsageStats: vi.fn(),
  getDashboardOverview: vi.fn(),
  getThroughputTrend: vi.fn(),
  getLatencyHistogram: vi.fn(),
  getErrorDistribution: vi.fn(),
  listAccounts: vi.fn(),
  getBatchUsage: vi.fn(),
  replace: vi.fn(),
  settings: { opsMonitoringEnabled: true },
}))

const route = reactive<{ query: Record<string, string | undefined> }>({ query: {} })

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: { getSnapshotV2: api.getSnapshotV2, getApiKeyUsageTrend: api.getApiKeyUsageTrend },
    usage: { getStats: api.getUsageStats },
    ops: {
      getDashboardOverview: api.getDashboardOverview,
      getThroughputTrend: api.getThroughputTrend,
      getLatencyHistogram: api.getLatencyHistogram,
      getErrorDistribution: api.getErrorDistribution,
    },
    accounts: { list: api.listAccounts, getBatchUsage: api.getBatchUsage },
  },
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => api.settings,
}))

vi.mock('vue-router', () => ({
  useRoute: () => route,
  useRouter: () => ({ replace: api.replace }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => (params ? `${key} ${Object.values(params).join(' ')}` : key),
      locale: { value: 'en-US' },
    }),
  }
})

// Noon on Sunday 30 August 2026, in the test machine's own time zone.
const NOW = new Date(2026, 7, 30, 12, 0, 0)

const point = (date: string, requests: number, overrides: Partial<TrendDataPoint> = {}): TrendDataPoint => ({
  date,
  requests,
  input_tokens: requests * 10,
  output_tokens: requests * 2,
  cache_creation_tokens: requests,
  cache_read_tokens: requests * 30,
  total_tokens: requests * 43,
  cost: requests * 0.02,
  actual_cost: requests * 0.01,
  ...overrides,
})

const TrendChartStub = {
  props: ['labels', 'tips', 'series', 'summary', 'type'],
  template: '<div class="trend-stub" />',
}
const HeatGridStub = { props: ['rows', 'summary'], template: '<div class="heat-stub" />' }
const CapacityStub = { props: ['accounts', 'loading', 'error'], template: '<div data-testid="capacity-section" />' }
const ToggleStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<button data-testid="compare-toggle" @click="$emit(\'update:modelValue\', !modelValue)" />',
}

const mountView = () => mount(StatsView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      LoadingSpinner: true,
      Icon: true,
      ProviderMark: true,
      SparkLine: true,
      Toggle: ToggleStub,
      TrendChart: TrendChartStub,
      HeatGrid: HeatGridStub,
      StatsCapacitySection: CapacityStub,
    },
  },
})

const snapshot = (trend: TrendDataPoint[], extra: Record<string, unknown> = {}) => ({
  generated_at: NOW.toISOString(),
  trend,
  ...extra,
})

/** Answers each snapshot request by its date range and granularity. */
function mockSnapshots(byStart: Record<string, ReturnType<typeof snapshot>>) {
  api.getSnapshotV2.mockImplementation(async (params: { start_date: string; include_model_stats?: boolean }) => (
    byStart[`${params.start_date}${params.include_model_stats ? '+models' : ''}`] ?? byStart[params.start_date] ?? snapshot([])
  ))
}

const trafficChart = (wrapper: ReturnType<typeof mountView>) => wrapper.get('[data-testid="stats-traffic"]').getComponent(TrendChartStub)
const kpiText = (wrapper: ReturnType<typeof mountView>) => wrapper.get('[data-testid="stats-kpis"]').findAll('.viz-kpi').map((tile) => tile.text())

describe('admin StatsView', () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(NOW)
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
    route.query = {}
    api.settings.opsMonitoringEnabled = true
    for (const mock of Object.values(api)) {
      if (typeof mock === 'function') mock.mockReset()
    }

    mockSnapshots({
      '2026-08-24+models': snapshot(
        [point('2026-08-29 09:00', 100), point('2026-08-30 08:00', 200)],
        {
          models: [
            { model: 'gpt-5.2-codex', requests: 180, total_tokens: 9000, actual_cost: 2.5 },
            { model: 'claude-sonnet-4-5', requests: 120, total_tokens: 6000, actual_cost: 3.5 },
          ],
          groups: [
            { group_id: 11, group_name: 'OpenAI Production', requests: 180, total_tokens: 9000 },
            { group_id: 12, group_name: 'Claude Production', requests: 120, total_tokens: 6000 },
          ],
        },
      ),
      '2026-08-17': snapshot([point('2026-08-22 09:00', 200)]),
      '2026-08-24': snapshot([point('2026-08-30 08:00', 200), point('2026-08-30 20:00', 999)]),
    })
    api.getUsageStats.mockImplementation(async (params: { start_date: string }) => (
      params.start_date === '2026-08-24'
        ? { total_requests: 300, average_duration_ms: 1200 }
        : { total_requests: 200, average_duration_ms: 1500 }
    ))
    api.getDashboardOverview.mockResolvedValue({
      request_count_total: 300,
      error_count_total: 6,
      error_rate: 0.02,
      duration: { p50_ms: 800, p90_ms: 1900, p95_ms: 2500, p99_ms: 4800 },
      ttft: { p50_ms: 250 },
    })
    api.getThroughputTrend.mockResolvedValue({
      bucket: 'hour',
      points: [],
      by_platform: [
        { platform: 'anthropic', request_count: 100, token_consumed: 5000 },
        { platform: 'openai', request_count: 300, token_consumed: 9000 },
      ],
    })
    api.getLatencyHistogram.mockResolvedValue({ buckets: [{ range: '<500ms', count: 40 }, { range: '500ms-1s', count: 60 }] })
    api.getErrorDistribution.mockResolvedValue({ total: 6, items: [{ status_code: 502, total: 2 }, { status_code: 429, total: 4 }] })
    api.getApiKeyUsageTrend.mockResolvedValue({
      trend: [
        { date: '2026-08-29', api_key_id: 1, key_name: 'ci-pipeline', requests: 40, tokens: 400 },
        { date: '2026-08-30', api_key_id: 1, key_name: 'ci-pipeline', requests: 60, tokens: 600 },
        { date: '2026-08-30', api_key_id: 2, key_name: 'notebooks', requests: 20, tokens: 100 },
      ],
    })
    api.listAccounts.mockResolvedValue({ items: [{ id: 1, name: 'Codex', platform: 'openai', type: 'oauth' }], total: 1, page: 1, page_size: 1000, pages: 1 })
    api.getBatchUsage.mockResolvedValue({ usage: {}, errors: {} })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('loads the 7-day period, its previous period, the heatmap, and ops detail, but not the account fleet', async () => {
    mountView()
    await flushPromises()

    const trendOnly = {
      include_stats: false,
      include_trend: true,
      include_model_stats: false,
      include_group_stats: false,
      include_users_trend: false,
    }
    expect(api.getSnapshotV2).toHaveBeenCalledTimes(3)
    expect(api.getSnapshotV2).toHaveBeenCalledWith({
      ...trendOnly,
      start_date: '2026-08-24',
      end_date: '2026-08-30',
      granularity: 'hour',
      include_model_stats: true,
      include_group_stats: true,
    })
    expect(api.getSnapshotV2).toHaveBeenCalledWith({ ...trendOnly, start_date: '2026-08-17', end_date: '2026-08-23', granularity: 'hour' })
    expect(api.getUsageStats).toHaveBeenCalledWith({ start_date: '2026-08-24', end_date: '2026-08-30' })
    expect(api.getUsageStats).toHaveBeenCalledWith({ start_date: '2026-08-17', end_date: '2026-08-23' })
    expect(api.getDashboardOverview).toHaveBeenCalledWith({
      start_time: new Date(2026, 7, 24).toISOString(),
      end_time: new Date(2026, 7, 31).toISOString(),
    })
    expect(api.listAccounts).not.toHaveBeenCalled()
    expect(api.getApiKeyUsageTrend).not.toHaveBeenCalled()
  })

  it('totals each tile and compares it with the same span of the previous period', async () => {
    const wrapper = mountView()
    await flushPromises()

    const [requests, tokens, cost, latency] = kpiText(wrapper)
    expect(requests).toContain('300')
    expect(requests).toContain('+50.0%')
    expect(tokens).toContain('12.9K')
    expect(cost).toContain('$3.00')
    expect(latency).toContain('1.20s')
    expect(latency).toContain('−20.0%')

    // More requests is good; a higher bill and slower responses are not.
    const tones = wrapper.findAll('.viz-delta').map((delta) => delta.classes().find((name) => name.startsWith('is-')))
    expect(tones).toEqual(['is-good', 'is-good', 'is-poor', 'is-good'])

    await wrapper.get('[data-testid="compare-toggle"]').trigger('click')
    expect(kpiText(wrapper)[0]).not.toContain('%')
  })

  it('draws complete six-hour slots up to now, with the previous period behind', async () => {
    const wrapper = mountView()
    await flushPromises()

    const chart = trafficChart(wrapper)
    const [current, previous] = chart.props('series')
    // Six full days of four slots, then midnight, 6 AM, and the running noon slot.
    expect(current.values).toHaveLength(27)
    expect(chart.props('labels')).toHaveLength(27)
    expect(current.values.reduce((sum: number, value: number) => sum + value, 0)).toBe(300)
    expect(current.values[25]).toBe(200)
    expect(previous.muted).toBe(true)
    expect(previous.values).toHaveLength(27)
    expect(wrapper.get('[data-testid="stats-traffic"] .viz-card-foot').text()).toContain('300')

    await wrapper.get('[data-testid="stats-metric-cost"]').trigger('click')
    expect(trafficChart(wrapper).props('series')[0].values[25]).toBe(2)
    expect(wrapper.get('[data-testid="stats-traffic"] .viz-card-foot').text()).toContain('$3.00')
  })

  it('reloads hourly data for today and stops the line at the current hour', async () => {
    const wrapper = mountView()
    await flushPromises()
    mockSnapshots({ '2026-08-30+models': snapshot([point('2026-08-30 08:00', 50)]) })

    await wrapper.get('[data-testid="stats-range-today"]').trigger('click')
    await flushPromises()

    expect(api.getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-08-30',
      end_date: '2026-08-30',
      granularity: 'hour',
      include_model_stats: true,
    }))
    expect(api.getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({ start_date: '2026-08-29', end_date: '2026-08-29' }))
    const values = trafficChart(wrapper).props('series')[0].values as number[]
    expect(values).toHaveLength(13)
    expect(values[8]).toBe(50)
  })

  it('ranks providers from ops data and falls back to groups when ops monitoring is off', async () => {
    const wrapper = mountView()
    await flushPromises()

    const share = wrapper.get('[data-testid="stats-share"]')
    expect(share.get('h2').text()).toBe('admin.viz.providers')
    expect(share.findAll('.viz-row').map((row) => row.text())).toEqual(['OpenAI75%300', 'Anthropic25%100'])
    expect(wrapper.findAll('[data-testid^="stats-performance-"]')).toHaveLength(3)

    api.settings.opsMonitoringEnabled = false
    const offline = mountView()
    await flushPromises()

    expect(api.getDashboardOverview).toHaveBeenCalledTimes(1)
    const groups = offline.get('[data-testid="stats-share"]')
    expect(groups.get('h2').text()).toBe('admin.viz.groups')
    expect(groups.findAll('.viz-row').map((row) => row.text())).toEqual(['OpenAI Production60%180', 'Claude Production40%120'])
    // Without ops data only the token breakdown is available.
    expect(offline.findAll('[data-testid^="stats-performance-"]')).toHaveLength(1)
    expect(offline.get('[data-testid="stats-performance"]').text()).toContain('75%')
  })

  it('shows latency percentiles first and error codes by frequency', async () => {
    const wrapper = mountView()
    await flushPromises()

    const card = wrapper.get('[data-testid="stats-performance"]')
    expect(card.findAll('.viz-percentiles b').map((value) => value.text())).toEqual(['800ms', '1.90s', '2.50s', '4.80s'])
    expect(card.getComponent(TrendChartStub).props('type')).toBe('bar')
    expect(card.text()).toContain('250ms')

    await wrapper.get('[data-testid="stats-performance-errors"]').trigger('click')
    expect(card.text()).toContain('2.00%')
    expect(card.findAll('.viz-hbar').map((row) => row.text())).toEqual(['admin.viz.httpStatus 4294', 'admin.viz.httpStatus 5022'])
  })

  it('loads API key totals only when their tab opens, once per range', async () => {
    const wrapper = mountView()
    await flushPromises()

    const top = wrapper.get('[data-testid="stats-top"]')
    expect(top.findAll('.viz-hbar').map((row) => row.text())).toEqual(['gpt-5.2-codex18060%', 'claude-sonnet-4-512040%'])

    await wrapper.get('[data-testid="stats-top-keys"]').trigger('click')
    await flushPromises()

    expect(api.getApiKeyUsageTrend).toHaveBeenCalledTimes(1)
    expect(api.getApiKeyUsageTrend).toHaveBeenCalledWith({ start_date: '2026-08-24', end_date: '2026-08-30', granularity: 'day', limit: 6 })
    expect(top.findAll('.viz-hbar').map((row) => row.text())).toEqual(['ci-pipeline10083%', 'notebooks2017%'])
  })

  it('marks the busiest hour and leaves later hours of today empty', async () => {
    const wrapper = mountView()
    await flushPromises()

    const rows = wrapper.getComponent(HeatGridStub).props('rows') as Array<{ values: Array<number | null> }>
    expect(rows).toHaveLength(7)
    const today = rows[6].values
    expect(today[8]).toBe(200)
    expect(today[12]).toBe(0)
    // 8 PM today has not happened yet, even though the fixture carries a value for it.
    expect(today[20]).toBeNull()
    expect(wrapper.get('[data-testid="stats-busy"] .viz-card-foot').text()).toContain('8 AM')
  })

  it('reports a failed usage load and recovers on retry', async () => {
    api.getUsageStats.mockRejectedValueOnce(new Error('stats unavailable'))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="stats-usage-error"]').text()).toContain('admin.stats.usage.loadFailed')
    expect(wrapper.find('[data-testid="stats-kpis"]').exists()).toBe(false)

    await wrapper.get('[data-testid="stats-usage-error"] button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="stats-usage-error"]').exists()).toBe(false)
    expect(kpiText(wrapper)[0]).toContain('300')
  })

  it('keeps usage when optional ops detail fails', async () => {
    api.getDashboardOverview.mockRejectedValue(new Error('ops unavailable'))

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.find('[data-testid="stats-usage-error"]').exists()).toBe(false)
    expect(kpiText(wrapper)[0]).toContain('300')
    expect(wrapper.findAll('[data-testid^="stats-performance-"]')).toHaveLength(1)
  })

  it('opens Capacity from the URL and loads the account fleet for it', async () => {
    route.query = { view: 'capacity' }

    const wrapper = mountView()
    await flushPromises()

    expect(api.listAccounts).toHaveBeenCalledWith(1, 1000, { include_scheduler_score: '0' })
    expect(wrapper.getComponent(CapacityStub).props('accounts')).toEqual([expect.objectContaining({ name: 'Codex' })])
    expect(wrapper.find('[data-testid="stats-kpis"]').exists()).toBe(false)

    await wrapper.get('[data-testid="stats-tab-usage"]').trigger('click')
    expect(api.replace).toHaveBeenCalledWith({ query: { view: undefined } })
  })
})
