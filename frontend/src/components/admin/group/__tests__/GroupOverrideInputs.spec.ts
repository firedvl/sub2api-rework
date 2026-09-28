import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminGroup } from '@/types'
import GroupRPMOverridesModal from '../GroupRPMOverridesModal.vue'
import GroupRateMultipliersModal from '../GroupRateMultipliersModal.vue'

const mocks = vi.hoisted(() => ({
  list: vi.fn(), getGroupRPMOverrides: vi.fn(), batchSetGroupRPMOverrides: vi.fn(),
  getGroupRateMultipliers: vi.fn(), batchSetGroupRateMultipliers: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: { users: { list: mocks.list }, groups: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
afterEach(() => vi.useRealTimers())
beforeEach(() => {
  vi.clearAllMocks()
  vi.useFakeTimers()
  mocks.getGroupRPMOverrides.mockResolvedValue([])
  mocks.getGroupRateMultipliers.mockResolvedValue([])
  mocks.list.mockResolvedValue({ items: [{ id: 7, email: 'user@example.com', status: 'active' }] })
  mocks.batchSetGroupRPMOverrides.mockResolvedValue(undefined)
  mocks.batchSetGroupRateMultipliers.mockResolvedValue(undefined)
})

async function selectUser(component: typeof GroupRPMOverridesModal | typeof GroupRateMultipliersModal) {
  const wrapper = mount(component, {
    props: { show: false, group: { id: 1, name: 'Group', platform: 'openai' } as AdminGroup },
    global: { stubs: {
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
      Icon: true, PlatformIcon: true, Pagination: true
    } }
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  await wrapper.get('input[type="text"]').setValue('user')
  await vi.advanceTimersByTimeAsync(300)
  await flushPromises()
  await wrapper.findAll('button').find(b => b.text().includes('user@example.com'))!.trigger('click')
  return wrapper
}

describe('group override input validation', () => {
  it.each(['', '1.5', '-1'])('rejects invalid RPM %j', async value => {
    const wrapper = await selectUser(GroupRPMOverridesModal)
    const input = wrapper.get('input[placeholder="100"]')
    await input.setValue('100')
    await input.setValue(value)
    const add = wrapper.findAll('button').find(b => b.text() === 'common.add')!
    expect(add.attributes('disabled')).toBeDefined()
    await add.trigger('click')
    expect(wrapper.find('tbody tr').exists()).toBe(false)
  })

  it('accepts unlimited zero RPM', async () => {
    const wrapper = await selectUser(GroupRPMOverridesModal)
    await wrapper.get('input[placeholder="100"]').setValue('0')
    await wrapper.findAll('button').find(b => b.text() === 'common.add')!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(mocks.batchSetGroupRPMOverrides).toHaveBeenCalledWith(1, [{ user_id: 7, rpm_override: 0 }])
  })

  it.each(['', '0', '-1'])('rejects invalid rate %j', async value => {
    const wrapper = await selectUser(GroupRateMultipliersModal)
    const input = wrapper.get('input[placeholder="1.0"]')
    await input.setValue('1')
    await input.setValue(value)
    const add = wrapper.findAll('button').find(b => b.text() === 'common.add')!
    expect(add.attributes('disabled')).toBeDefined()
    await add.trigger('click')
    expect(wrapper.find('tbody tr').exists()).toBe(false)
  })

  it('accepts positive rates', async () => {
    const wrapper = await selectUser(GroupRateMultipliersModal)
    await wrapper.get('input[placeholder="1.0"]').setValue('0.25')
    await wrapper.findAll('button').find(b => b.text() === 'common.add')!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(mocks.batchSetGroupRateMultipliers).toHaveBeenCalledWith(1, [{ user_id: 7, rate_multiplier: 0.25 }])
  })
})
