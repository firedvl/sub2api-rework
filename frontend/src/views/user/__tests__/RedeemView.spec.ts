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
    getHistory.mockResolvedValue([])
    refreshUser.mockRejectedValue(new Error('Service unavailable'))
    fetchActiveSubscriptions.mockResolvedValue([])
    vi.spyOn(console, 'error').mockImplementation(() => {})
  })
  afterEach(() => vi.restoreAllMocks())

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
