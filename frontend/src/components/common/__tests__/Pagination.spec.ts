import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import Pagination from '../Pagination.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

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
