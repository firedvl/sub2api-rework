import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { afterEach, describe, expect, it, vi } from 'vitest'
import AuthLayout from '@/components/layout/AuthLayout.vue'
import { useAppStore } from '@/stores/app'

vi.mock('@/api/auth', () => ({ getPublicSettings: vi.fn() }))

describe('AuthLayout identity', () => {
  afterEach(() => {
    delete window.__APP_CONFIG__
  })

  it('renders the Gateway identity for legacy public settings', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    window.__APP_CONFIG__ = {
      site_name: 'Sub2API',
      site_logo: '',
      site_subtitle: 'Subscription to API Conversion Platform',
    } as typeof window.__APP_CONFIG__
    useAppStore().initFromInjectedConfig()
    const i18n = createI18n({
      legacy: false,
      locale: 'en',
      messages: {
        en: {
          auth: {
            poweredBy: ({ named }: { named: (key: string) => unknown }) =>
              `Powered by ${named('product')}`,
          },
        },
      },
    })

    const wrapper = mount(AuthLayout, {
      global: { plugins: [pinia, i18n] },
      slots: { default: '<p>Auth form</p>' },
    })

    expect(wrapper.get('.auth-codex-brand h1').text()).toBe('Gateway')
    expect(wrapper.get('.auth-codex-brand p').text()).toBe('AI Gateway')
    expect(wrapper.get('.auth-codex-copyright').text()).toBe('Powered by Sub2API Rework')
    expect(wrapper.get('.auth-codex-logo img').attributes()).toMatchObject({
      alt: '',
      'aria-hidden': 'true',
      src: '/logo.svg',
    })
  })
})
