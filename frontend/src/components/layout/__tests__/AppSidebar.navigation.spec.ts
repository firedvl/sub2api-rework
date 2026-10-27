import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import AppSidebar from '../AppSidebar.vue'
import { useAdminSettingsStore, useAuthStore } from '@/stores'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/composables/useBatchImageAccess', async () => {
  const { computed } = await import('vue')
  return {
    useBatchImageAccess: () => ({
      canUseBatchImage: computed(() => false),
      refreshBatchImageAccess: vi.fn(),
    }),
  }
})

async function mountSidebar(path: string, role: 'admin' | 'user') {
  const pinia = createPinia()
  setActivePinia(pinia)
  useAuthStore().user = { id: 1, email: 'operator@example.test', username: 'operator', role } as never
  vi.spyOn(useAdminSettingsStore(), 'fetch').mockResolvedValue()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }],
  })
  await router.push(path)
  await router.isReady()

  const wrapper = mount(AppSidebar, {
    global: { plugins: [pinia, router], stubs: { VersionBadge: true } },
  })
  await flushPromises()
  return wrapper
}

const hrefs = (wrapper: Awaited<ReturnType<typeof mountSidebar>>) =>
  wrapper.findAll('a').map((link) => link.attributes('href'))

describe('AppSidebar navigation reach', () => {
  it('gives operators every area, secondary admin page, and personal page', async () => {
    const wrapper = await mountSidebar('/admin/dashboard', 'admin')
    const links = hrefs(wrapper)

    for (const path of [
      '/admin/dashboard',
      '/admin/stats',
      '/admin/accounts',
      '/admin/groups',
      '/admin/usage',
      '/admin/settings',
      '/admin/users',
      '/admin/announcements',
      '/admin/redeem',
      '/admin/promo-codes',
      '/keys',
      '/profile',
    ]) {
      expect(links).toContain(path)
    }
  })

  it('marks the owning area current for a section route', async () => {
    const wrapper = await mountSidebar('/admin/channels/pricing', 'admin')

    const current = wrapper.findAll('a[aria-current="page"]')
    expect(current).toHaveLength(1)
    expect(current[0].attributes('href')).toBe('/admin/groups')
  })

  it('keeps collapsed More links out of the tab order until the group opens', async () => {
    const wrapper = await mountSidebar('/admin/dashboard', 'admin')
    const toggle = wrapper.get('button[aria-controls]')
    const panel = wrapper.get(`#${toggle.attributes('aria-controls')}`)

    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(panel.attributes('inert')).toBeDefined()

    await toggle.trigger('click')

    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(panel.attributes('inert')).toBeUndefined()
  })

  it('opens More when the current route belongs to it', async () => {
    const wrapper = await mountSidebar('/admin/users', 'admin')

    expect(wrapper.get('button[aria-controls]').attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('a[aria-current="page"]').attributes('href')).toBe('/admin/users')
  })

  it('shows regular users only their personal pages', async () => {
    const wrapper = await mountSidebar('/keys', 'user')
    const links = hrefs(wrapper)

    expect(links).toContain('/dashboard')
    expect(links).toContain('/keys')
    expect(links.some((href) => href?.startsWith('/admin'))).toBe(false)
  })
})
