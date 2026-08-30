import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import type { Account, AccountUsageInfo } from '@/types'
import OperatorCapacityOverview from './OperatorCapacityOverview.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (params?.count != null) return `${key}:${params.count}`
        if (params?.time != null) return `${key}:${params.time}`
        return key
      }
    })
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
  created_at: '2026-08-25T10:00:00Z',
  updated_at: '2026-08-26T10:00:00Z',
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

const usage = (utilization: number): AccountUsageInfo => ({
  updated_at: '2026-08-26T10:00:00Z',
  five_hour: { utilization, resets_at: '2026-08-26T12:00:00Z', remaining_seconds: 7200 },
  seven_day: null,
  seven_day_sonnet: null,
})

describe('OperatorCapacityOverview', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('groups accounts by provider and persists collapsed sections for the session', async () => {
    const wrapper = mount(OperatorCapacityOverview, {
      props: {
        accounts: [
          account(1, {
            name: 'OpenAI primary',
            groups: [{ id: 7, name: 'production' } as Account['groups'][number]],
            credentials: {
              project_id: 'project-visible',
              model_mapping: { 'gpt-5': 'gpt-5.1' },
              compact_model_mapping: { 'gpt-5-mini': 'gpt-5.1-mini' },
            },
          }),
          account(2, { name: 'OpenAI reserve' }),
          account(3, { name: 'Gemini primary', platform: 'gemini', type: 'apikey' }),
        ],
        usageByAccountId: {
          '1': usage(18),
          '2': usage(42),
          '3': usage(70),
        },
      },
      global: {
        stubs: { LoadingSpinner: true, RouterLink: true }
      }
    })

    expect(wrapper.findAll('.operator-capacity-provider-name').map((node) => node.text())).toEqual([
      'OpenAI',
      'Gemini',
    ])
    expect(wrapper.findAll('.operator-capacity-provider-mark')).toHaveLength(2)
    expect(wrapper.get('[data-testid="provider-toggle-openai"] .operator-capacity-provider-mark svg').exists()).toBe(true)
    expect(wrapper.text()).toContain('OpenAI primary')
    expect(wrapper.text()).toContain('OpenAI reserve')
    expect(wrapper.text()).toContain('Gemini primary')
    const details = wrapper.findAll('[data-testid="account-technical-details"]')
    expect(details).toHaveLength(3)
    expect(details.every((detail) => detail.attributes('style')?.includes('display: none'))).toBe(true)

    const primaryAccount = wrapper.findAll('.operator-capacity-account')
      .find((row) => row.text().includes('OpenAI primary'))!
    const accountToggle = primaryAccount.get('.operator-capacity-details-toggle')
    const accountDetails = primaryAccount.get('[data-testid="account-technical-details"]')
    expect(accountToggle.attributes('aria-expanded')).toBe('false')
    await accountToggle.trigger('click')
    expect(accountToggle.attributes('aria-expanded')).toBe('true')
    expect(accountDetails.attributes('style')).not.toContain('display: none')
    expect(accountDetails.text()).toContain('admin.dashboard.capacity.modelCount:2')
    expect(accountDetails.text()).toContain('production')
    expect(wrapper.get('[data-testid="provider-toggle-openai"]').text()).toContain(
      'admin.dashboard.capacity.normalizedRemaining',
    )

    const openAIToggle = wrapper.get('[data-testid="provider-toggle-openai"]')
    expect(openAIToggle.attributes('aria-expanded')).toBe('true')
    await openAIToggle.trigger('click')
    expect(openAIToggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get('#operator-provider-openai-accounts').attributes('style')).toContain('display: none')
    expect(sessionStorage.getItem('operator-capacity-collapsed-providers')).toBe('["openai"]')
  })

  it('renders every known quota window as a default-row progress bar and keeps unknown quota text-only', () => {
    const wrapper = mount(OperatorCapacityOverview, {
      props: {
        accounts: [account(1, { name: 'Known account' }), account(2, { name: 'Unknown account' })],
        usageByAccountId: {
          '1': {
            updated_at: '2026-08-26T10:00:00Z',
            five_hour: { utilization: 58, resets_at: null, remaining_seconds: null },
            seven_day: { utilization: 32, resets_at: null, remaining_seconds: null },
            seven_day_sonnet: null,
          },
        },
      },
      global: {
        stubs: { LoadingSpinner: true, RouterLink: true }
      }
    })

    const rows = wrapper.findAll('[data-testid="account-capacity-row"]')
    const known = rows.find((row) => row.text().includes('Known account'))!
    const unknown = rows.find((row) => row.text().includes('Unknown account'))!
    const bars = known.findAll('.operator-capacity-row-track')

    expect(bars).toHaveLength(2)
    expect(bars.map((bar) => bar.attributes('aria-valuenow'))).toEqual(['42', '68'])
    expect(known.text()).toContain('5h')
    expect(known.text()).toContain('42%')
    expect(known.text()).toContain('7d')
    expect(known.text()).toContain('68%')
    expect(unknown.text()).toContain('admin.dashboard.capacity.quotaUnknown')
    expect(unknown.find('[role="progressbar"]').exists()).toBe(false)
    expect(unknown.find('[aria-valuenow="0"]').exists()).toBe(false)
  })

  it('shows a compact reset line while preserving the full limited reason in the title', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 7, 30, 10, 0))
    const resetAt = new Date(2026, 7, 30, 15, 0).toISOString()
    const wrapper = mount(OperatorCapacityOverview, {
      props: {
        accounts: [account(1, {
          name: 'Rate-limited account',
          rate_limited_at: new Date(2026, 7, 30, 9, 0).toISOString(),
          rate_limit_reset_at: resetAt,
        })],
      },
      global: {
        stubs: { LoadingSpinner: true, RouterLink: true }
      }
    })

    const status = wrapper.get('.operator-capacity-account-status')
    expect(status.text()).toContain('Limited')
    expect(status.text()).toContain('admin.dashboard.capacity.resets:3:00 PM')
    expect(status.text()).not.toContain('Provider rate limit')
    expect(status.get('small').attributes('title')).toContain('Provider rate limit until')
    expect(status.get('small').attributes('title')).toContain('15:00')
  })

  it('renders an accessible normalized pool and excludes unknown quota', () => {
    const wrapper = mount(OperatorCapacityOverview, {
      props: {
        compact: true,
        accounts: [account(1), account(2), account(3, { platform: 'gemini' })],
        usageByAccountId: {
          '1': usage(20),
          '2': usage(60),
        },
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          RouterLink: {
            props: ['to'],
            template: '<a :href="to"><slot /></a>',
          },
        }
      }
    })

    const pool = wrapper.get('[data-testid="account-pool-capacity"]')
    expect(pool.get('[role="progressbar"]').attributes('aria-label')).toContain('admin.dashboard.capacity.poolTitle')
    expect(pool.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('60')
    expect(pool.text()).toContain('60%')
    expect(pool.text()).toContain('admin.dashboard.capacity.knownCount:2')
    expect(pool.text()).toContain('admin.dashboard.capacity.unknownCount:1')
    expect(pool.text()).toContain('Account 2')
    expect(pool.get('a').attributes('href')).toBe('/admin/stats')
    expect(wrapper.find('.operator-capacity-providers').exists()).toBe(false)
  })

  it('filters by operator status and keeps quota exhaustion separate from errors', () => {
    const wrapper = mount(OperatorCapacityOverview, {
      props: {
        accounts: [
          account(1, { name: 'Ready account' }),
          account(2, { name: 'Quota limited' }),
          account(3, { name: 'Broken account', status: 'error', error_message: 'Authentication failed' }),
          account(4, { name: 'Disabled account', status: 'inactive', schedulable: false }),
        ],
        usageByAccountId: {
          '1': usage(18),
          '2': usage(100),
        },
        status: 'limited',
      },
      global: {
        stubs: { LoadingSpinner: true, RouterLink: true }
      }
    })

    const rows = wrapper.findAll('[data-testid="account-capacity-row"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].attributes('data-status')).toBe('limited')
    expect(rows[0].text()).toContain('Quota limited')
    expect(rows[0].get('.operator-capacity-account-status small').attributes('title')).toContain('5h quota exhausted')
    expect(rows[0].text()).toContain('5h')
    expect(rows[0].text()).toContain('0%')
    expect(wrapper.text()).not.toContain('Broken account')
    expect(wrapper.text()).not.toContain('Disabled account')
  })
})
