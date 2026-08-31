import { describe, expect, it, vi } from 'vitest'
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
  updated_at: '2026-08-25T00:00:00Z',
  five_hour: null,
  seven_day: null,
  seven_day_sonnet: null,
  ...overrides,
})

describe('StatsCapacitySection', () => {
  it('separates actual short and long windows, preserves unknowns, and renders provider icons', () => {
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
    const donuts = wrapper.get('[data-testid="stats-capacity-donut-overview"]')
    expect(donuts.get('[data-testid="stats-capacity-donut-overall"] svg').attributes('aria-label')).toContain('45%')
    expect(donuts.get('[data-testid="provider-capacity-openai"] .stats-capacity-donut-icon svg').exists()).toBe(true)
    expect(donuts.get('[data-testid="provider-capacity-gemini"] .stats-capacity-donut-chart svg').attributes('aria-label')).toContain('quotaUnknown')
    expect(donuts.get('[data-testid="provider-capacity-gemini"]').text()).not.toContain('0%')
    expect(shortTerm.get('[data-testid="short-provider-openai"] svg').exists()).toBe(true)
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('5h')
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('65%')
    expect(shortTerm.get('[data-testid="short-provider-openai"]').text()).toContain('admin.stats.capacity.windowCoverage 2 0')
    expect(shortTerm.get('[data-testid="short-provider-gemini"]').text()).toContain('admin.stats.capacity.notReported')

    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('7d')
    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('40%')
    expect(longTerm.get('[data-testid="long-provider-openai"]').text()).toContain('admin.stats.capacity.windowCoverage 1 1')
    expect(longTerm.get('[data-testid="long-provider-gemini"]').text()).toContain('admin.stats.capacity.notReported')
    expect(wrapper.get('[data-testid="provider-capacity-gemini"]').text()).toContain('admin.stats.capacity.coverage 0 1')
  })

  it('uses one compact native inspector for account contributions and missing windows', async () => {
    const wrapper = mount(StatsCapacitySection, {
      props: {
        accounts: [account(1), account(2)],
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

    const inspector = wrapper.get('[data-testid="stats-capacity-inspector"]')
    const select = inspector.get('select')
    expect(select.findAll('option').map((option) => option.text())).toEqual([
      'OpenAI · 5h',
      'OpenAI · 7d',
    ])
    expect(inspector.text()).toContain('Account 1')
    expect(inspector.text()).toContain('Account 2')

    await select.setValue('openai:seven_day')
    expect(inspector.text()).toContain('OpenAI · 7d')
    expect(inspector.text()).toContain('admin.stats.capacity.windowCoverage 1 1')
    expect(inspector.text()).toContain('Account 1')
    expect(inspector.text()).toContain('Account 2')
    expect(inspector.text()).toContain('admin.dashboard.capacity.quotaUnknown')
  })

  it('bounds a 52-account inspector and compacts 20 Antigravity model limits', async () => {
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

    expect(wrapper.findAll('[data-testid="stats-inspector-account-row"]')).toHaveLength(5)
    expect(wrapper.get('[data-testid="stats-inspector-account-summary"]').text()).toContain('52 20 5 47')
    expect(wrapper.get('[data-testid="stats-inspector-account-row"]').text()).toContain('admin.dashboard.capacity.remaining 0')
    expect(wrapper.text()).not.toContain('Hidden searchable reserve')

    const accountSelect = wrapper.findAllComponents(Select)
      .find((component) => component.props('ariaLabel') === 'admin.stats.capacity.inspectAccount')!
    expect(accountSelect.props('options')).toHaveLength(52)
    accountSelect.vm.$emit('update:modelValue', '52')
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('[data-testid="stats-inspector-account-row"]')).toHaveLength(5)
    const hiddenAccount = wrapper.findAll('[data-testid="stats-inspector-account-row"]')
      .find((row) => row.text().includes('Hidden searchable reserve'))!
    expect(hiddenAccount.text()).toContain('admin.dashboard.capacity.quotaUnknown')
    expect(hiddenAccount.text()).not.toContain('0%')

    const inspectorSelect = wrapper.get('#stats-capacity-inspector-select')
    expect(inspectorSelect.findAll('option').filter((option) => option.text().includes('Antigravity'))).toHaveLength(1)
    await inspectorSelect.setValue('antigravity:model-limits')
    expect(wrapper.findAll('[data-testid="stats-model-limit-row"]')).toHaveLength(3)
    expect(wrapper.get('[data-testid="stats-model-limit-summary"]').text()).toContain('0% 20 model-01')
    expect(wrapper.text()).toContain('admin.stats.capacity.moreModelLimits 17')

    const modelSelect = wrapper.findAllComponents(Select)
      .find((component) => component.props('ariaLabel') === 'admin.stats.capacity.inspectModel')!
    expect(modelSelect.props('options')).toHaveLength(20)
    modelSelect.vm.$emit('update:modelValue', 'antigravity:hidden-model-search-target')
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('[data-testid="stats-model-limit-row"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('hidden-model-search-target')
  })
})
