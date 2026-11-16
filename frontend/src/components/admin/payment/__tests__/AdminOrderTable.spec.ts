import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { PaymentOrder } from '@/types/payment'
import AdminOrderTable from '../AdminOrderTable.vue'
import AdminOrderDetail from '../AdminOrderDetail.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

function order(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 7,
    user_id: 3,
    amount: 10,
    pay_amount: 10,
    currency: 'USD',
    fee_rate: 0,
    payment_type: 'stripe',
    out_trade_no: 'sub2_7',
    status: 'COMPLETED',
    order_type: 'balance',
    created_at: '2026-06-25T10:00:00Z',
    expires_at: '2026-06-25T10:30:00Z',
    refund_amount: 0,
    ...overrides,
  }
}

function mountTable(orders: PaymentOrder[]) {
  return mount(AdminOrderTable, {
    props: { orders, loading: false, page: 1, pageSize: 20, total: orders.length },
    global: { stubs: { Icon: true, Pagination: true } },
  })
}

describe('AdminOrderTable', () => {
  it('opens the detail on row click and Enter', async () => {
    const wrapper = mountTable([order()])
    const row = wrapper.get('tbody tr')
    await row.trigger('click')
    await row.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('detail')).toHaveLength(2)
  })

  it.each([
    ['PENDING', 'cancel'],
    ['FAILED', 'retry'],
    ['REFUND_REQUESTED', 'refund'],
    ['REFUND_FAILED', 'refund'],
    ['REFUND_PENDING', 'queryRefund'],
    ['COMPLETED', 'refund'],
    ['PARTIALLY_REFUNDED', 'refund'],
  ] as const)('routes the %s row action to %s without opening the detail', async (status, event) => {
    const wrapper = mountTable([order({ status })])
    await wrapper.get('[data-testid="order-action-7"]').trigger('click')
    expect(wrapper.emitted(event)).toHaveLength(1)
    expect(wrapper.emitted('detail')).toBeUndefined()
  })

  it('offers no row action for closed orders', () => {
    const wrapper = mountTable([order({ status: 'EXPIRED' })])
    expect(wrapper.find('[data-testid="order-action-7"]').exists()).toBe(false)
  })

  it('shows the clear-filters empty state when filtered', async () => {
    const wrapper = mount(AdminOrderTable, {
      props: { orders: [], loading: false, page: 1, pageSize: 20, total: 0, filtered: true },
      global: { stubs: { Icon: true, Pagination: true } },
    })
    expect(wrapper.text()).toContain('admin.adminUi.noMatches')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('clearFilters')).toHaveLength(1)
  })
})

describe('AdminOrderDetail drawer', () => {
  const SideDrawerStub = {
    props: ['open'],
    emits: ['close'],
    template: '<div v-if="open"><slot /><slot name="footer" /></div>',
  }

  it('shows audit logs and the refund request, and runs the footer action', async () => {
    const wrapper = mount(AdminOrderDetail, {
      props: {
        show: true,
        order: order({
          status: 'REFUND_REQUESTED',
          refund_requested_at: '2026-06-26T10:00:00Z',
          refund_requested_by: 3,
          refund_request_reason: 'not needed',
        }),
        auditLogs: [{ id: 1, action: 'ORDER_PAID', detail: 'ok', operator: 'system', created_at: '2026-06-25T10:01:00Z' }],
      },
      global: { stubs: { SideDrawer: SideDrawerStub, Icon: true } },
    })
    expect(wrapper.text()).toContain('ORDER_PAID')
    expect(wrapper.text()).toContain('not needed')
    const action = wrapper.get('[data-testid="order-drawer-action"]')
    expect(action.text()).toContain('payment.admin.approveRefund')
    await action.trigger('click')
    expect(wrapper.emitted('refund')).toHaveLength(1)
  })
})
