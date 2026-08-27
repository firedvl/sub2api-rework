import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

const stores = vi.hoisted(() => ({
  auth: {
    isAuthenticated: false,
    isAdmin: false,
    isSimpleMode: false,
    hasPendingAuthSession: false,
    checkAuth: vi.fn(),
  },
  app: {
    backendModeEnabled: false,
    publicSettingsLoaded: false,
    cachedPublicSettings: null as Record<string, unknown> | null,
    siteName: 'Sub2API',
    fetchPublicSettings: vi.fn(),
  },
  adminSettings: {
    customMenuItems: [],
  },
  compliance: {
    initialized: true,
    fetchStatus: vi.fn(),
    requireAcknowledgement: vi.fn(),
  },
}))

const api = vi.hoisted(() => ({
  getSetupStatus: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => stores.auth }))
vi.mock('@/stores/app', () => ({ useAppStore: () => stores.app }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => stores.adminSettings }))
vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => stores.compliance,
}))
vi.mock('@/api/setup', () => ({
  getSetupStatus: api.getSetupStatus,
}))
vi.mock('@/router/title', () => ({ resolveRouteDocumentTitle: () => 'Sub2API' }))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
  }),
}))
vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({ triggerPrefetch: vi.fn() }),
}))

const view = { template: '<main />' }
vi.mock('@/views/HomeView.vue', () => ({ default: view }))
vi.mock('@/views/auth/LoginView.vue', () => ({ default: view }))
vi.mock('@/views/user/DashboardView.vue', () => ({ default: view }))
vi.mock('@/views/user/PaymentView.vue', () => ({ default: view }))
vi.mock('@/views/admin/DashboardView.vue', () => ({ default: view }))
vi.mock('@/views/admin/StatsView.vue', () => ({ default: view }))
vi.mock('@/views/admin/AccountsView.vue', () => ({ default: view }))
vi.mock('@/views/admin/GroupsView.vue', () => ({ default: view }))
vi.mock('@/views/admin/SettingsView.vue', () => ({ default: view }))

let router: typeof import('@/router').default

async function navigate(path: string) {
  await router.push(path)
  await router.isReady()
  return router.currentRoute.value
}

describe('production router navigation', () => {
  beforeAll(async () => {
    window.scrollTo = vi.fn()
    router = (await import('@/router')).default
  })

  beforeEach(async () => {
    stores.auth.isAuthenticated = false
    stores.auth.isAdmin = false
    stores.auth.isSimpleMode = false
    stores.auth.hasPendingAuthSession = false
    stores.app.backendModeEnabled = false
    stores.app.publicSettingsLoaded = false
    stores.app.cachedPublicSettings = null
    stores.app.fetchPublicSettings.mockReset().mockResolvedValue(null)
    stores.compliance.initialized = true
    stores.compliance.fetchStatus.mockReset().mockResolvedValue({ required: false })
    stores.compliance.requireAcknowledgement.mockReset()
    api.getSetupStatus.mockReset().mockResolvedValue({ needs_setup: true })
    await navigate('/home')
  })

  it('redirects completed setup through the production guard', async () => {
    api.getSetupStatus.mockResolvedValue({ needs_setup: false })

    expect((await navigate('/setup')).path).toBe('/login')

    stores.auth.isAuthenticated = true
    stores.auth.isAdmin = true
    expect((await navigate('/setup')).path).toBe('/admin/dashboard')
  })

  it('preserves a protected deep link for unauthenticated users', async () => {
    const route = await navigate('/admin/accounts?page=3&status=error')

    expect(route.path).toBe('/login')
    expect(route.query.redirect).toBe('/admin/accounts?page=3&status=error')
  })

  it('keeps an authenticated personal user out of admin routes', async () => {
    stores.auth.isAuthenticated = true

    expect((await navigate('/admin/accounts')).path).toBe('/dashboard')
  })

  it('allows an admin route and runs the production compliance check', async () => {
    stores.auth.isAuthenticated = true
    stores.auth.isAdmin = true
    stores.compliance.initialized = false

    expect((await navigate('/admin/accounts')).path).toBe('/admin/accounts')
    expect(stores.compliance.fetchStatus).toHaveBeenCalledOnce()
  })

  it('allows an authenticated admin to open Stats', async () => {
    stores.auth.isAuthenticated = true
    stores.auth.isAdmin = true

    const route = await navigate('/admin/stats')

    expect(route.path).toBe('/admin/stats')
    expect(route.meta).toMatchObject({
      requiresAuth: true,
      requiresAdmin: true,
      titleKey: 'admin.stats.title',
      descriptionKey: 'admin.stats.description',
    })
  })

  it('redirects an authenticated admin away from login', async () => {
    stores.auth.isAuthenticated = true
    stores.auth.isAdmin = true

    expect((await navigate('/login')).path).toBe('/admin/dashboard')
  })

  it('applies simple-mode restrictions through the production route table', async () => {
    stores.auth.isAuthenticated = true
    stores.auth.isAdmin = true
    stores.auth.isSimpleMode = true

    expect((await navigate('/admin/groups')).path).toBe('/admin/dashboard')
  })

  it('applies simple-mode restrictions to personal subscription routes', async () => {
    stores.auth.isAuthenticated = true
    stores.auth.isSimpleMode = true

    expect((await navigate('/subscriptions')).path).toBe('/dashboard')
    expect((await navigate('/redeem')).path).toBe('/dashboard')
  })

  it('blocks an authenticated non-admin in backend mode', async () => {
    stores.auth.isAuthenticated = true
    stores.app.backendModeEnabled = true

    expect((await navigate('/dashboard')).path).toBe('/login')
  })

  it('uses the production backend-mode callback and pending-auth allowlists', async () => {
    stores.app.backendModeEnabled = true

    expect((await navigate('/auth/wechat/callback')).path).toBe('/auth/wechat/callback')

    stores.auth.hasPendingAuthSession = true
    expect((await navigate('/register')).path).toBe('/register')

    stores.auth.hasPendingAuthSession = false
    expect((await navigate('/email-verify')).path).toBe('/login')
  })

  it('opens compliance acknowledgement when the production check returns 423', async () => {
    const metadata = { version: 'e2e' }
    stores.auth.isAuthenticated = true
    stores.auth.isAdmin = true
    stores.compliance.initialized = false
    stores.compliance.fetchStatus.mockRejectedValue({
      status: 423,
      code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
      metadata,
    })

    expect((await navigate('/admin/accounts')).path).toBe('/admin/accounts')
    expect(stores.compliance.requireAcknowledgement).toHaveBeenCalledWith(metadata)
  })

  it('redirects when loaded settings explicitly disable a feature route', async () => {
    stores.auth.isAuthenticated = true
    stores.app.publicSettingsLoaded = true
    stores.app.cachedPublicSettings = { payment_enabled: false }

    expect((await navigate('/purchase')).path).toBe('/dashboard')
    expect(stores.app.fetchPublicSettings).not.toHaveBeenCalled()
  })

  it('allows a feature route while settings remain unknown', async () => {
    stores.auth.isAuthenticated = true
    stores.app.fetchPublicSettings.mockResolvedValue(null)

    expect((await navigate('/purchase')).path).toBe('/purchase')
    expect(stores.app.fetchPublicSettings).toHaveBeenCalledOnce()
  })
})
