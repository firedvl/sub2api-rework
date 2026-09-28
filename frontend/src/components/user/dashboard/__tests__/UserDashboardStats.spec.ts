import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import UserDashboardStats from '../UserDashboardStats.vue'
import type { UserDashboardStats as Stats } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string, params?: { count: number }) => params ? `${key}:${params.count}` : key }),
}))

const stats: Stats = {
  total_api_keys: 0, active_api_keys: 0, total_requests: 0, total_input_tokens: 0,
  total_output_tokens: 0, total_cache_creation_tokens: 0, total_cache_read_tokens: 0,
  total_tokens: 0, total_cost: 0, total_actual_cost: 5, today_requests: 0,
  today_input_tokens: 0, today_output_tokens: 0, today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0, today_tokens: 0, today_cost: 0, today_actual_cost: 0,
  average_duration_ms: 0, rpm: 0, tpm: 0,
  by_platform: [{ platform: 'openai', total_requests: 1, total_tokens: 10,
    total_actual_cost: 3, today_requests: 0, today_tokens: 0, today_actual_cost: 0 }],
}

describe('platform quota dashboard cards', () => {
  it('shows usage and configured limits, hides unlimited empty rows, and excludes Other from count', () => {
    const quota = (platform: PlatformQuotaItem['platform'], daily_limit_usd: number | null): PlatformQuotaItem => ({
      platform, daily_limit_usd, weekly_limit_usd: null, monthly_limit_usd: null,
      daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0,
    })
    const wrapper = mount(UserDashboardStats, { props: {
      stats, balance: 0, isSimple: false,
      platformQuotas: [quota('anthropic', 0), quota('gemini', null), quota('openai', null)],
    } })
    expect(wrapper.findAll('[data-testid="platform-card"]').map(card => card.attributes('data-platform')))
      .toEqual(['anthropic', 'openai', '__other__'])
    expect(wrapper.text()).toContain('dashboard.platformCount:2')
    wrapper.unmount()
  })
})
