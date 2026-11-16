import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import PromoCodesView from '../PromoCodesView.vue'

const { list, getUsages, showError, showSuccess } = vi.hoisted(() => ({
  list: vi.fn(),
  getUsages: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    promo: { list, getUsages, create: vi.fn(), update: vi.fn(), delete: vi.fn() },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard: vi.fn().mockResolvedValue(true) }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const code = {
  id: 9,
  code: 'WELCOME10',
  bonus_amount: 10,
  max_uses: 4,
  used_count: 1,
  status: 'active' as const,
  expires_at: null,
  notes: 'launch',
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-02T00:00:00Z',
}

const DataTableStub = {
  props: ['data'],
  emits: ['rowClick'],
  template: `<div>
    <div v-for="row in data" :key="row.id" class="stub-row" @click="$emit('rowClick', row)">
      <slot name="cell-code" :value="row.code" :row="row" />
      <slot name="cell-usage" :row="row" />
    </div>
  </div>`,
}

function mountView() {
  return mount(PromoCodesView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: true,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
      },
    },
  })
}

describe('admin PromoCodesView', () => {
  beforeEach(() => {
    list.mockReset()
    getUsages.mockReset()
    list.mockResolvedValue({ items: [code], total: 1 })
    getUsages.mockResolvedValue({
      items: [{ id: 1, promo_code_id: 9, user_id: 5, bonus_amount: 10, used_at: '2026-11-03T00:00:00Z', user: { email: 'used@example.com' } }],
      total: 1,
    })
  })

  it('filters by status from the segmented control', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('1 / 4')

    await wrapper.get('[data-test="promo-status-disabled"]').trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(1, expect.any(Number), expect.objectContaining({ status: 'disabled' }), expect.anything())
    expect(wrapper.get('[data-test="promo-status-disabled"]').attributes('aria-pressed')).toBe('true')
    wrapper.unmount()
  })

  it('opens a row in the detail drawer with its usage records', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('.stub-row').trigger('click')
    await flushPromises()

    expect(getUsages).toHaveBeenCalledWith(9, 1, 20)
    const records = document.body.querySelector('[data-test="promo-usage-records"]')
    expect(records?.textContent).toContain('used@example.com')
    expect(document.body.querySelector('.ui-drawer')?.textContent).toContain('launch')
    wrapper.unmount()
  })
})
