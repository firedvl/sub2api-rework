import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Account, AccountUsageInfo } from '@/types'
import Select from '@/components/common/Select.vue'
import StatsCapacitySection from './StatsCapacitySection.vue'

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

const account = (id: number, overrides: Partial<Account> = {}): Account => ({
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
  ...overrides,
})

const usage = (overrides: Partial<AccountUsageInfo>): AccountUsageInfo => ({
  source: 'passive',
  updated_at: '2026-08-25T00:00:00Z',
  five_hour: null,
  seven_day: null,
  seven_day_sonnet: null,
  ...overrides,
})

afterEach(() => {
  document.body.querySelectorAll('.select-dropdown-portal').forEach((element) => element.remove())
})

describe('StatsCapacitySection', () => {
  it('separates actual windows and labels normalized donut semantics without turning unknown into zero', () => {
    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [
          account(1),
          account(2),
          account(3, { platform: 'gemini', name: 'Unknown Gemini' }),
        ],
        usageByAccountId: {
          '1': usage({
            five_hour: { utilization: 20, resets_at: null, remaining_seconds: null },
            seven_day: { utilization: 60, resets_at: null, remaining_seconds: null },
          }),
          '2': usage({
            five_hour: { utilization: 50, resets_at: null, remaining_seconds: null },
          }),
        },
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const shortTerm = wrapper.get('[data-testid="stats-short-term-capacity"]')
    const longTerm = wrapper.get('[data-testid="stats-long-term-capacity"]')
    const overall = wrapper.get('[data-testid="stats-capacity-donut-overall"]')
    const openAI = wrapper.get('[data-testid="provider-capacity-openai"]')
    const unknown = wrapper.get('[data-testid="provider-capacity-gemini"]')

    expect(overall.get('[data-testid="stats-capacity-donut-basis"]').text()).toBe('admin.stats.capacity.mixedAverageLimitingQuota')
    expect(overall.get('svg').attributes('aria-label')).toContain('admin.stats.capacity.mixedAverageLimitingQuota')
    expect(openAI.get('[data-testid="stats-capacity-donut-basis"]').text()).toBe('admin.stats.capacity.averageLimitingQuota')
    expect(openAI.get('.stats-capacity-donut-chart svg').attributes('aria-label')).toContain('admin.stats.capacity.averageLimitingQuota')
    expect(openAI.text()).toContain('Account 1 · 7d')
    expect(unknown.get('.stats-capacity-donut-chart svg').attributes('aria-label')).toContain('quotaUnknown')
    expect(unknown.text()).not.toContain('0%')
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('5h')
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('65%')
    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('7d')
    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('40%')
  })

  it('qualifies mixed-window averages as non-pooled capacity', () => {
    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [
          account(1),
          account(2, {
            name: 'Daily spend account',
            platform: 'grok',
            type: 'api_key',
            quota_daily_limit: 100,
            quota_daily_used: 10,
          }),
        ],
        usageByAccountId: {
          '1': usage({
            seven_day: { utilization: 90, resets_at: null, remaining_seconds: null },
          }),
        },
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const overall = wrapper.get('[data-testid="stats-capacity-donut-overall"]')
    expect(overall.text()).toContain('50%')
    expect(overall.get('[data-testid="stats-capacity-donut-basis"]').text()).toBe('admin.stats.capacity.mixedAverageLimitingQuota')
    expect(overall.get('svg').attributes('aria-label')).toContain('admin.stats.capacity.mixedAverageLimitingQuota')
  })

  it('defaults to the most constrained account and shows one selected account with every reported window', async () => {
    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [
          account(1, {
            name: 'Healthy account',
            credentials: { api_key: 'never-render-this-secret', email: 'safe@example.test' },
          }),
          account(2, { name: 'Exhausted account' }),
          account(3, { name: 'Unknown account', platform: 'gemini' }),
        ],
        usageByAccountId: {
          '1': usage({
            five_hour: { utilization: 20, resets_at: '2026-08-25T20:00:00Z', remaining_seconds: 7200 },
            seven_day: { utilization: 60, resets_at: '2026-08-29T00:00:00Z', remaining_seconds: 345600 },
          }),
          '2': usage({
            five_hour: { utilization: 100, resets_at: '2026-08-25T21:00:00Z', remaining_seconds: 10800 },
          }),
        },
      },
      global: { stubs: { LoadingSpinner: true, Transition: false } },
    })

    const inspector = wrapper.get('[data-testid="stats-capacity-inspector"]')
    const accountSelect = wrapper.findAllComponents(Select)
      .find((component) => component.props('ariaLabel') === 'admin.stats.capacity.inspectAccount')!

    expect(inspector.find('select').exists()).toBe(false)
    expect(inspector.find('.stats-inspector-accounts').exists()).toBe(false)
    expect(inspector.find('[data-testid="stats-inspector-account-row"]').exists()).toBe(false)
    expect(accountSelect.props('searchable')).toBe(true)
    expect(accountSelect.props('options')).toHaveLength(3)
    expect(inspector.get('[data-testid="stats-selected-account-detail"]').text()).toContain('Exhausted account')
    expect(inspector.findAll('[data-testid="stats-account-capacity-window"]')).toHaveLength(1)
    expect(inspector.get('[data-testid="stats-account-limiting-window"]').text()).toContain('5h 0%')
    expect(inspector.text()).toContain('admin.stats.capacity.status.limited')

    accountSelect.vm.$emit('update:modelValue', '1')
    await wrapper.vm.$nextTick()

    const detail = inspector.get('[data-testid="stats-selected-account-detail"]')
    expect(detail.text()).toContain('Healthy account')
    expect(detail.findAll('[data-testid="stats-account-capacity-window"]')).toHaveLength(2)
    expect(detail.text()).toContain('5h')
    expect(detail.text()).toContain('80%')
    expect(detail.text()).toContain('7d')
    expect(detail.text()).toContain('40%')
    expect(detail.get('[data-testid="stats-account-limiting-window"]').text()).toContain('7d 40%')
    expect(detail.text()).toContain('admin.stats.capacity.passiveSnapshot')
    expect(detail.text()).not.toContain('never-render-this-secret')
    expect(detail.text()).not.toContain('safe@example.test')
  })

  it('uses the shared searchable combobox keyboard contract on Stats', async () => {
    const wrapper = mount(StatsCapacitySection, {
      attachTo: document.body,
      props: {
        accounts: [account(1), account(2, { platform: 'gemini', name: 'Gemini reserve' })],
        usageByAccountId: {},
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const trigger = wrapper.get('button[aria-label="admin.stats.capacity.inspectAccount"]')
    await trigger.trigger('keydown', { key: 'ArrowDown' })
    const listbox = document.body.querySelector<HTMLElement>('[role="listbox"]')!
    const search = document.body.querySelector<HTMLInputElement>('[aria-label="admin.stats.capacity.searchAccounts"]')!

    expect(listbox).not.toBeNull()
    expect(search).not.toBeNull()
    search.value = 'gemini'
    search.dispatchEvent(new Event('input'))
    await wrapper.vm.$nextTick()
    expect(listbox.querySelectorAll('[role="option"]')).toHaveLength(1)
    expect(listbox.textContent).toContain('Gemini reserve')

    listbox.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await wrapper.vm.$nextTick()
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger.element)
    await vi.waitFor(() => expect(document.body.querySelector('[role="listbox"]')).toBeNull())
    wrapper.unmount()
  })

  it('keeps a 50+ account pool compact and limits Antigravity to three searchable model rows', async () => {
    const accounts = Array.from({ length: 52 }, (_, index) => account(index + 1, {
      name: index === 51 ? 'Hidden searchable reserve' : `Pool account ${String(index + 1).padStart(2, '0')}`,
    }))
    const usageByAccountId = Object.fromEntries(accounts.map((item, index) => [String(item.id), usage({
      five_hour: index < 20
        ? { utilization: 100, resets_at: null, remaining_seconds: null }
        : index < 47
          ? { utilization: index < 30 ? 90 : 20, resets_at: null, remaining_seconds: null }
          : null,
    })]))
    const antigravityQuota = Object.fromEntries(Array.from({ length: 20 }, (_, index) => [
      index === 19 ? 'hidden-model-search-target' : `model-${String(index + 1).padStart(2, '0')}`,
      {
        utilization: index === 0 ? 100 : Math.max(4, 95 - index * 4),
        reset_time: new Date(Date.UTC(2026, 7, 30, 3 + index)).toISOString(),
      },
    ]))
    const antigravity = account(100, { platform: 'antigravity', name: 'Antigravity pool' })

    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [...accounts, antigravity],
        usageByAccountId: {
          ...usageByAccountId,
          '100': usage({ antigravity_quota: antigravityQuota }),
        },
      },
      global: { stubs: { LoadingSpinner: true } },
    })

    const accountSelect = wrapper.findAllComponents(Select)
      .find((component) => component.props('ariaLabel') === 'admin.stats.capacity.inspectAccount')!
    expect(accountSelect.props('options')).toHaveLength(53)
    expect(wrapper.findAll('[data-testid="stats-selected-account-detail"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="stats-inspector-account-row"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="stats-selected-account-detail"]').text()).toContain('Antigravity pool')

    accountSelect.vm.$emit('update:modelValue', '52')
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-testid="stats-selected-account-detail"]').text()).toContain('Hidden searchable reserve')
    expect(wrapper.get('[data-testid="stats-account-capacity-unknown"]').exists()).toBe(true)

    accountSelect.vm.$emit('update:modelValue', '100')
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('[data-testid="stats-model-limit-row"]')).toHaveLength(3)
    expect(wrapper.get('[data-testid="stats-model-limit-summary"]').text()).toContain('0% 20 model-01')

    const modelSelect = wrapper.findAllComponents(Select)
      .find((component) => component.props('ariaLabel') === 'admin.stats.capacity.inspectModel')!
    expect(modelSelect.props('options')).toHaveLength(20)
    modelSelect.vm.$emit('update:modelValue', 'antigravity:hidden-model-search-target')
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('[data-testid="stats-model-limit-row"]')).toHaveLength(3)
    expect(wrapper.get('[data-testid="stats-selected-account-detail"]').text()).toContain('hidden-model-search-target')
  })
})
