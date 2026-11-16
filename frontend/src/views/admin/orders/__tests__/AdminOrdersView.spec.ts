import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AdminOrdersView from '../AdminOrdersView.vue'

const { getOrders, getOrder, cancelOrder } = vi.hoisted(() => ({
  getOrders: vi.fn(),
  getOrder: vi.fn(),
  cancelOrder: vi.fn(),
}))

vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getOrders, getOrder, cancelOrder }, default: { getOrders, getOrder, cancelOrder } }))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const row = {
  id: 9,
  user_id: 4,
  user_email: 'buyer@example.com',
  amount: 10,
  pay_amount: 10,
  currency: 'USD',
  fee_rate: 0,
  payment_type: 'stripe',
  out_trade_no: 'sub2_9',
  status: 'PENDING',
  order_type: 'balance',
  created_at: '2026-11-01T10:00:00Z',
  expires_at: '2026-11-01T10:30:00Z',
  refund_amount: 0,
}

function mountView() {
  return mount(AdminOrdersView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: true,
        Select: true,
        AdminRefundDialog: true,
        teleport: true,
      },
    },
  })
}

describe('AdminOrdersView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getOrders.mockResolvedValue({ data: { items: [row], total: 1 } })
    getOrder.mockResolvedValue({ data: { order: { ...row, user_email: undefined }, auditLogs: [{ id: 1, action: 'ORDER_CREATED', detail: null, operator: null, created_at: row.created_at }] } })
    cancelOrder.mockResolvedValue({})
  })

  it('filters by the status segment from the first page', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="orders-status-FAILED"]').trigger('click')
    await flushPromises()
    expect(getOrders).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1, status: 'FAILED' }))
  })

  it('opens the order drawer with audit logs and keeps list user fields', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('tbody tr.is-clickable').trigger('click')
    await flushPromises()
    expect(getOrder).toHaveBeenCalledWith(9)
    const drawer = wrapper.get('.ui-drawer')
    expect(drawer.text()).toContain('ORDER_CREATED')
    expect(drawer.text()).toContain('buyer@example.com')

    await drawer.get('[data-testid="order-drawer-action"]').trigger('click')
    await flushPromises()
    expect(cancelOrder).toHaveBeenCalledWith(9)
  })
})
