import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AdminRefundDialog from '../AdminRefundDialog.vue'
import type { PaymentOrder } from '@/types/payment'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const order = { id: 1, amount: 100, pay_amount: 100, status: 'PAID' } as PaymentOrder

describe('refund balance warning', () => {
  it('compares the requested partial refund against user balance', async () => {
    const wrapper = mount(AdminRefundDialog, {
      props: { show: false, order, userBalance: 30 },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } },
    })
    await wrapper.setProps({ show: true })
    expect(wrapper.text()).toContain('payment.admin.insufficientBalance')
    await wrapper.get('input[type="number"]').setValue('20')
    expect(wrapper.text()).not.toContain('payment.admin.insufficientBalance')
    await wrapper.get('input[type="number"]').setValue('40')
    expect(wrapper.text()).toContain('payment.admin.insufficientBalance')
    wrapper.unmount()
  })
})
