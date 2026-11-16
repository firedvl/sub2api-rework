import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'

import AdminPaymentDashboardView from '../AdminPaymentDashboardView.vue'

const { getDashboard } = vi.hoisted(() => ({ getDashboard: vi.fn() }))

vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getDashboard }, default: { getDashboard } }))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('AdminPaymentDashboardView', () => {
  it('renders KPI tiles, methods, and per-currency top users, and reloads on range change', async () => {
    getDashboard.mockResolvedValue({
      data: {
        today_amount: { CNY: 100 },
        total_amount: { CNY: 900, USD: 50 },
        today_count: 3,
        total_count: 40,
        avg_amount: { CNY: 22.5 },
        daily_series: [{ date: '2026-11-01', amount: { CNY: 100 }, count: 3 }],
        payment_methods: [{ type: 'alipay', amount: { CNY: 900 }, count: 30 }],
        top_users: {
          CNY: [{ user_id: 1, email: 'cny@example.com', amount: 500 }],
          USD: [{ user_id: 2, email: 'usd@example.com', amount: 50 }],
        },
      },
    })
    const wrapper = mount(AdminPaymentDashboardView, {
      global: {
        plugins: [createPinia()],
        stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true, TrendChart: true },
      },
    })
    await flushPromises()

    expect(getDashboard).toHaveBeenCalledWith(30)
    expect(wrapper.findAll('[data-testid="payment-kpis"] .viz-kpi')).toHaveLength(4)
    expect(wrapper.get('[data-testid="payment-methods"]').text()).toContain('payment.methods.alipay')

    const topUsers = wrapper.get('[data-testid="payment-top-users"]')
    expect(topUsers.text()).toContain('cny@example.com')
    await topUsers.findAll('.viz-seg button').find((button) => button.text() === 'USD')!.trigger('click')
    expect(topUsers.text()).toContain('usd@example.com')

    await wrapper.findAll('.ui-toolbar .viz-seg button')[0].trigger('click')
    await flushPromises()
    expect(getDashboard).toHaveBeenLastCalledWith(7)
  })
})
