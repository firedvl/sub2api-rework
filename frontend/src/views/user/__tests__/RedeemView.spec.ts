import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import RedeemView from '../RedeemView.vue'

const { redeem, getHistory, refreshUser, fetchActiveSubscriptions, showError, showWarning, showSuccess } = vi.hoisted(() => ({
  redeem: vi.fn(), getHistory: vi.fn(), refreshUser: vi.fn(), fetchActiveSubscriptions: vi.fn(),
  showError: vi.fn(), showWarning: vi.fn(), showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  redeemAPI: { redeem, getHistory },
  authAPI: { getPublicSettings: vi.fn().mockResolvedValue({}) },
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { balance: 10, concurrency: 2 }, refreshUser }) }))
vi.mock('@/stores/subscriptions', () => ({ useSubscriptionStore: () => ({ fetchActiveSubscriptions }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showWarning, showSuccess }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('RedeemView refresh after redemption', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    redeem.mockResolvedValue({ type: 'balance', value: 20, message: 'Code applied' })
    getHistory.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 0 })
    refreshUser.mockRejectedValue(new Error('Service unavailable'))
    fetchActiveSubscriptions.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => vi.restoreAllMocks())

  it('retains the last page after a failed size change and supports retry', async () => {
    getHistory.mockResolvedValueOnce({ items: [{ id: 1, type: 'balance', code: 'HISTORY-OLD', value: 20, used_at: '2026-09-01T00:00:00Z' }], total: 105 })
    const wrapper = mount(RedeemView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } } })
    await flushPromises()
    getHistory.mockRejectedValueOnce(new Error('Current network error'))
    await wrapper.get('select').setValue('50')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 50)
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('20')
    expect(wrapper.text()).toContain('HISTORY-')
    expect(showError).toHaveBeenCalledWith('redeem.historyLoadFailed')
    getHistory.mockResolvedValueOnce({ items: [], total: 105 })
    await wrapper.get('select').setValue('50')
    await flushPromises()
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('50')
    expect(showError).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it.each(['success', 'failure'])('ignores superseded history %s responses', async (outcome) => {
    let resolveOld!: (value: unknown) => void
    let rejectOld!: (reason: Error) => void
    getHistory.mockReturnValueOnce(new Promise((resolve, reject) => { resolveOld = resolve; rejectOld = reject }))
    const wrapper = mount(RedeemView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } } })
    await flushPromises()
    getHistory.mockResolvedValueOnce({ items: [{ id: 2, type: 'concurrency', code: 'NEW-HISTORY', value: 30, used_at: '2026-09-02T00:00:00Z' }], total: 105 })
    await wrapper.get('select').setValue('50')
    await flushPromises()
    expect(getHistory).toHaveBeenLastCalledWith(1, 50)
    if (outcome === 'success') resolveOld({ items: [], total: 0 })
    else rejectOld(new Error('Stale network error'))
    await flushPromises()
    expect(wrapper.text()).toContain('NEW-HIST')
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('50')
    expect(showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it.each(['balance', 'concurrency', 'subscription'])('keeps successful %s redemption after account refresh fails', async (type) => {
    redeem.mockResolvedValue({ type, value: 20, message: 'Code applied' })
    const wrapper = mount(RedeemView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' }, Icon: true } },
    })
    await flushPromises()
    await wrapper.get('input#code').setValue(' REDEEM-CODE ')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(redeem).toHaveBeenCalledWith('REDEEM-CODE')
    expect(showWarning).toHaveBeenCalledWith('redeem.userRefreshFailed')
    expect(showError).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalledWith('redeem.codeRedeemSuccess')
    expect(wrapper.text()).toContain('Code applied')
    expect((wrapper.get('input#code').element as HTMLInputElement).value).toBe('')
    expect(fetchActiveSubscriptions).toHaveBeenCalledTimes(type === 'subscription' ? 1 : 0)
    wrapper.unmount()
  })
})
