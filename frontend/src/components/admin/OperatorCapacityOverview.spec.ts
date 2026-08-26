import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import type { Account, AccountUsageInfo } from '@/types'
import OperatorCapacityOverview from './OperatorCapacityOverview.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => params?.count == null
        ? key
        : `${key}:${params.count}`
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
        stubs: { LoadingSpinner: true }
      }
    })

    expect(wrapper.findAll('.operator-capacity-provider-name').map((node) => node.text())).toEqual([
      'OpenAI',
      'Gemini',
    ])
    expect(wrapper.text()).toContain('OpenAI primary')
    expect(wrapper.text()).toContain('OpenAI reserve')
    expect(wrapper.text()).toContain('Gemini primary')
    expect(wrapper.findAll('[data-testid="account-technical-details"]')).toHaveLength(0)
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
        stubs: { LoadingSpinner: true }
      }
    })

    const pool = wrapper.get('[data-testid="account-pool-capacity"]')
    expect(pool.get('svg[role="img"]').attributes('aria-label')).toContain('admin.dashboard.capacity.poolTitle')
    expect(pool.findAll('[data-testid="account-pool-segment"]')).toHaveLength(2)
    expect(pool.text()).toContain('Account 1')
    expect(pool.text()).toContain('Account 2')
    expect(pool.text()).toContain('Account 3')
    expect(pool.text()).toContain('admin.dashboard.capacity.poolUnknownExcluded:1')

    const openAIProvider = wrapper.get('[data-testid="provider-capacity-openai"]')
    expect(openAIProvider.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('60')
    expect(openAIProvider.text()).toContain('admin.dashboard.capacity.knownCount:2')
    expect(openAIProvider.text()).toContain('admin.dashboard.capacity.unknownCount:0')

    const geminiProvider = wrapper.get('[data-testid="provider-capacity-gemini"]')
    expect(geminiProvider.get('[role="progressbar"]').attributes('aria-valuenow')).toBeUndefined()
    expect(geminiProvider.text()).toContain('admin.dashboard.capacity.quotaUnknown')
  })
})
