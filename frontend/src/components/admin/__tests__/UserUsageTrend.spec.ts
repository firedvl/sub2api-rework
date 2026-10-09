import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UserUsageTrend from '../UserUsageTrend.vue'

const { getUserUsageTrend } = vi.hoisted(() => ({ getUserUsageTrend: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { dashboard: { getUserUsageTrend } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<div />' } }))
const point = (user: number, tokens: number, cost: number) => ({
  date: '2026-10-08', user_id: user, username: 'User ' + user, tokens, actual_cost: cost
})
beforeEach(() => getUserUsageTrend.mockReset().mockResolvedValue({ trend: [] }))

describe('UserUsageTrend', () => {
  it('switches to spending and ignores an older token response', async () => {
    let resolveTokens!: (value: unknown) => void
    getUserUsageTrend.mockImplementationOnce(() => new Promise(resolve => { resolveTokens = resolve }))
    getUserUsageTrend.mockResolvedValueOnce({ trend: [point(2, 10, 5)] })
    const wrapper = mount(UserUsageTrend)
    await wrapper.findAll('button')[1]!.trigger('click')
    await flushPromises()
    expect(getUserUsageTrend).toHaveBeenLastCalledWith({ granularity: 'day', limit: 12, metric: 'actual_cost' })
    const chart = wrapper.getComponent({ name: 'Line' })
    expect(chart.props('data').datasets[0].data).toEqual([5])
    expect(chart.props('options').scales.y.ticks.callback(5)).toBe('$5.00')
    resolveTokens({ trend: [point(1, 1000, 1)] })
    await flushPromises()
    expect(chart.props('data').datasets[0].data).toEqual([5])
    wrapper.unmount()
  })

  it('shows empty and failed states and retries the selected metric', async () => {
    const wrapper = mount(UserUsageTrend)
    await flushPromises()
    expect(wrapper.text()).toContain('admin.dashboard.noDataAvailable')
    getUserUsageTrend.mockRejectedValueOnce(new Error('fixture failure'))
    await wrapper.findAll('button')[1]!.trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('admin.dashboard.failedToLoad')
    getUserUsageTrend.mockResolvedValueOnce({ trend: [point(2, 10, 5)] })
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(getUserUsageTrend).toHaveBeenLastCalledWith({ granularity: 'day', limit: 12, metric: 'actual_cost' })
    expect(wrapper.getComponent({ name: 'Line' }).props('data').datasets[0].data).toEqual([5])
    wrapper.unmount()
  })
})
