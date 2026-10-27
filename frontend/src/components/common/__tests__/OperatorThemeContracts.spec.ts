import { beforeEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { useAppStore } from '@/stores/app'
import Toast from '../Toast.vue'
import Toggle from '../Toggle.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

const frontendRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../../..')
const source = (path: string) => readFileSync(resolve(frontendRoot, path), 'utf8')

describe('operator shared theme contracts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('marks toggle state and thumb roles', () => {
    const wrapper = mount(Toggle, { props: { modelValue: true } })

    expect(wrapper.get('button').classes()).toEqual(expect.arrayContaining(['operator-toggle', 'operator-toggle-on']))
    expect(wrapper.get('span').classes()).toContain('operator-toggle-thumb')
  })

  it('marks toast tone, icon, close action, and track roles', () => {
    const store = useAppStore()
    store.showInfo('Snapshot ready', 5000)
    const wrapper = mount(Toast, {
      global: { stubs: { Icon: true, Teleport: true } }
    })

    const toast = wrapper.get('.operator-toast')
    expect(toast.attributes('data-tone')).toBe('info')
    expect(toast.classes()).not.toContain('border-l-4')
    expect(toast.classes()).not.toContain('border-blue-500')
    expect(toast.get('.operator-toast-icon').exists()).toBe(true)
    expect(toast.get('.operator-toast-close').attributes('aria-label')).toBe('common.closeNotification')
    expect(toast.get('.operator-toast-track').exists()).toBe(true)
  })

  it('drives every page background from the shared neutral tokens', () => {
    const styles = source('src/style.css')
    expect(styles).toMatch(/body \{[\s\S]*?background: var\(--operator-background\);/)
    expect(styles).toMatch(/\.dark \{[\s\S]*?--operator-background: #0b0b0d;/)
    expect(styles).not.toContain('body[data-operator-console]')
    expect(source('src/views/setup/SetupWizardView.vue')).toContain('class="auth-codex-shell"')
    expect(source('src/views/auth/OAuthCallbackView.vue')).toContain('<AuthLayout>')
    expect(source('src/views/auth/WechatPaymentCallbackView.vue')).toContain('<AuthLayout>')
  })

  it('keeps scrollbars, text controls, locale states, and proxy controls on shared tokens', () => {
    const styles = source('src/style.css')
    const locale = source('src/components/common/LocaleSwitcher.vue')
    const proxySelector = source('src/components/common/ProxySelector.vue')
    const proxies = source('src/views/admin/ProxiesView.vue')

    expect(styles).toContain('scrollbar-color: var(--operator-border-strong) transparent')
    expect(styles).toContain('caret-color: var(--operator-foreground)')
    expect(styles).toContain('outline: 2px solid var(--operator-focus)')
    expect(locale).toContain('operator-locale-selected')
    expect(locale).not.toContain('bg-primary-50')
    expect(proxySelector).toContain('border-color: var(--operator-accent)')
    expect(proxySelector).toContain('select-option-selected')
    expect(proxySelector).not.toContain('data-operator-console')
    expect(proxies).toContain('class="badge badge-gray"')
    expect(proxies).toContain('operator-checkbox')
  })

  it('keeps main as the sole page scroller for every route', () => {
    const layout = source('src/components/layout/AppLayout.vue')
    const styles = source('src/style.css')

    expect(layout).not.toContain('overflow-y-auto')
    expect(layout).toContain('class="app-main"')
    expect(layout).not.toContain('isOperatorConsole')
    expect(styles).toMatch(/\.app-shell \{[\s\S]*?height: 100dvh;[\s\S]*?overflow: hidden;/)
    expect(styles).toMatch(/\.app-main \{[\s\S]*?overflow-y: auto;/)
  })

  it('uses the operator radius for provider group headers', () => {
    const capacity = source('src/components/admin/OperatorCapacityOverview.vue')

    expect(capacity).toMatch(
      /\.operator-capacity-provider-header \{[\s\S]*?border: 1px solid var\(--operator-border-subtle\);[\s\S]*?border-radius: var\(--operator-radius\);/,
    )
  })
})
