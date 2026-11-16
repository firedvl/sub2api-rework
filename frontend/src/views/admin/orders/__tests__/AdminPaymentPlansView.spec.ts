import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminPaymentPlansView from '../AdminPaymentPlansView.vue'

const { getPlans, getConfig, getGroups } = vi.hoisted(() => ({
  getPlans: vi.fn(),
  getConfig: vi.fn(),
  getGroups: vi.fn(),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getPlans,
    getConfig,
  },
}))

vi.mock('@/api/admin', () => ({
  default: {
    groups: {
      getAll: getGroups,
    },
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('AdminPaymentPlansView', () => {
  beforeEach(() => {
    getGroups.mockResolvedValue([])
    getConfig.mockResolvedValue({ data: {} })
    getPlans.mockResolvedValue({
      data: [
        {
          id: 1,
          name: 'CNY plan',
          group_id: 1,
          price: 499,
          original_price: 599,
          currency: 'CNY',
          validity_days: 30,
          validity_unit: 'day',
          sort_order: 0,
          for_sale: true,
          features: [],
        },
        {
          id: 2,
          name: 'Legacy plan',
          group_id: 1,
          price: 10,
          original_price: 0,
          currency: '',
          validity_days: 30,
          validity_unit: 'day',
          sort_order: 0,
          for_sale: true,
          features: [],
        },
      ],
    })
  })

  it('uses the configured currency symbol and keeps legacy prices in USD', async () => {
    const wrapper = mount(AdminPaymentPlansView, {
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ConfirmDialog: true,
          GroupBadge: true,
          Icon: true,
          PlanEditDialog: true,
        },
      },
    })

    await flushPromises()

    const cny = wrapper.get('[data-testid="plan-card-1"]').text()
    expect(cny).toContain('¥499.00')
    expect(cny).toContain('CNY')
    expect(cny).toContain('¥599.00')
    expect(wrapper.get('[data-testid="plan-card-2"]').text()).toContain('$10.00')
  })

  it('filters plans by sale state with counts', async () => {
    getPlans.mockResolvedValueOnce({
      data: [
        { id: 1, name: 'On', group_id: 1, price: 1, currency: 'USD', validity_days: 30, validity_unit: 'day', sort_order: 0, for_sale: true, features: [] },
        { id: 2, name: 'Off', group_id: 1, price: 1, currency: 'USD', validity_days: 30, validity_unit: 'day', sort_order: 0, for_sale: false, features: [] },
      ],
    })
    const wrapper = mount(AdminPaymentPlansView, {
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ConfirmDialog: true,
          GroupBadge: true,
          Icon: true,
          PlanEditDialog: true,
          TransitionGroup: { template: '<div><slot /></div>' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-testid="plans-sale-off"]').text()).toContain('1')
    await wrapper.get('[data-testid="plans-sale-off"]').trigger('click')
    expect(wrapper.find('[data-testid="plan-card-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="plan-card-2"]').exists()).toBe(true)
  })
})
