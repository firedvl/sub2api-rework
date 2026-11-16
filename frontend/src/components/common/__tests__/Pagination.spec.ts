import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import Pagination from '../Pagination.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key}:${Object.values(params).join(',')}` : key,
  }),
}))

describe('numeric pagination jump', () => {
  it('accepts numeric v-model input and clamps it to the final page', async () => {
    const wrapper = mount(Pagination, {
      props: { total: 80, page: 1, pageSize: 20, showJump: true, showPageSizeSelector: false },
      global: { stubs: { Icon: true, Select: true } },
    })
    const input = wrapper.get('input[type="number"]')
    await input.setValue('3')
    await input.trigger('keyup.enter')
    expect(wrapper.emitted('update:page')).toEqual([[3]])
    expect((input.element as HTMLInputElement).value).toBe('')
    await input.setValue('999')
    await input.trigger('keyup.enter')
    expect(wrapper.emitted('update:page')).toEqual([[3], [4]])
    await input.setValue('')
    await input.trigger('keyup.enter')
    expect(wrapper.emitted('update:page')).toHaveLength(2)
    wrapper.unmount()
  })
})

describe('compact pagination', () => {
  it('shows the visible range and steps between pages', async () => {
    const wrapper = mount(Pagination, {
      props: { total: 312, page: 1, pageSize: 20, showPageSizeSelector: false },
      global: { stubs: { Icon: true, Select: true } },
    })
    expect(wrapper.get('.pagination-info').text()).toBe('admin.adminUi.table.range:1,20,312')
    expect(wrapper.get('.pagination-compact').text()).toBe('1 / 16')
    const prev = wrapper.get('[aria-label="pagination.previous"]')
    const next = wrapper.get('[aria-label="pagination.next"]')
    expect(prev.attributes()).toHaveProperty('disabled')
    await next.trigger('click')
    expect(wrapper.emitted('update:page')).toEqual([[2]])
    wrapper.unmount()
  })

  it('disables both steps when there is nothing to page through', () => {
    const wrapper = mount(Pagination, {
      props: { total: 0, page: 1, pageSize: 20, showPageSizeSelector: false },
      global: { stubs: { Icon: true, Select: true } },
    })
    expect(wrapper.get('.pagination-info').text()).toBe('admin.adminUi.table.range:0,0,0')
    expect(wrapper.get('[aria-label="pagination.next"]').attributes()).toHaveProperty('disabled')
    wrapper.unmount()
  })
})
