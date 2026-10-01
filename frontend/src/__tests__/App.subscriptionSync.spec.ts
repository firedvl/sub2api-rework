import { afterEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import App from '../App.vue'

const mocks = vi.hoisted(() => ({
  fetchSubscriptions: vi.fn().mockResolvedValue(undefined), startPolling: vi.fn(), clear: vi.fn(),
  fetchSettings: vi.fn().mockResolvedValue(null),
}))
const app = reactive({
  publicSettingsLoaded: false, cachedPublicSettings: { subscription_enabled: false },
  siteName: 'Gateway', siteLogo: '', fetchPublicSettings: mocks.fetchSettings,
})
const auth = reactive({ isAuthenticated: true, isAdmin: false })
vi.mock('@/stores', () => ({
  useAppStore: () => app, useAuthStore: () => auth,
  useSubscriptionStore: () => ({ fetchActiveSubscriptions: mocks.fetchSubscriptions, startPolling: mocks.startPolling, clear: mocks.clear }),
  useAnnouncementStore: () => ({ fetchAnnouncements: vi.fn(), reset: vi.fn() }),
  useAdminComplianceStore: () => ({ reset: vi.fn(), requireAcknowledgement: vi.fn() }),
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => app }))
vi.mock('vue-router', () => ({ RouterView: { template: '<div />' }, useRouter: () => ({ afterEach: vi.fn(), replace: vi.fn() }), useRoute: () => ({ path: '/dashboard', meta: {} }) }))
vi.mock('@/router/title', () => ({ resolveRouteDocumentTitle: () => 'Gateway' }))
vi.mock('@/api/setup', () => ({ getSetupStatus: async () => ({ needs_setup: false }) }))
vi.mock('@/utils/branding', () => ({ updateFavicon: vi.fn() }))
vi.mock('@/components/common/Toast.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/NavigationProgress.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/AnnouncementPopup.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/admin/AdminComplianceDialog.vue', () => ({ default: { template: '<div />' } }))

afterEach(() => vi.clearAllMocks())

describe('subscription sync follows loaded settings', () => {
  it('waits for settings, starts on enable, stops on disable and restarts after login', async () => {
    app.publicSettingsLoaded = false
    app.cachedPublicSettings.subscription_enabled = false
    auth.isAuthenticated = true
    const wrapper = mount(App)
    await flushPromises()
    expect(mocks.fetchSubscriptions).not.toHaveBeenCalled()
    expect(mocks.startPolling).not.toHaveBeenCalled()
    app.publicSettingsLoaded = true
    await flushPromises()
    expect(mocks.fetchSubscriptions).not.toHaveBeenCalled()
    app.cachedPublicSettings.subscription_enabled = true
    await flushPromises()
    expect(mocks.fetchSubscriptions).toHaveBeenCalledTimes(1)
    expect(mocks.startPolling).toHaveBeenCalledTimes(1)
    app.cachedPublicSettings.subscription_enabled = false
    await flushPromises()
    expect(mocks.clear).toHaveBeenCalledTimes(1)
    auth.isAuthenticated = false
    await flushPromises()
    app.cachedPublicSettings.subscription_enabled = true
    await flushPromises()
    expect(mocks.fetchSubscriptions).toHaveBeenCalledTimes(1)
    auth.isAuthenticated = true
    await flushPromises()
    expect(mocks.fetchSubscriptions).toHaveBeenCalledTimes(2)
    expect(mocks.startPolling).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })
})
