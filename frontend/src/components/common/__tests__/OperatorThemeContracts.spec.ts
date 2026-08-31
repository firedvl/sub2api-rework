import { beforeEach, describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { useAppStore } from '@/stores/app'
import Toast from '../Toast.vue'
import Toggle from '../Toggle.vue'

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
    expect(toast.get('.operator-toast-close').exists()).toBe(true)
    expect(toast.get('.operator-toast-track').exists()).toBe(true)
  })

  it('keeps bootstrap and full-page loading roots neutral in dark mode', () => {
    expect(source('src/style.css')).toContain('dark:bg-black dark:text-gray-100')
    expect(source('src/style.css')).toContain('--operator-background: oklch(0.07 0 0)')
    expect(source('src/views/setup/SetupWizardView.vue')).toContain('class="auth-codex-shell"')
    expect(source('src/views/auth/OAuthCallbackView.vue')).toContain('<AuthLayout>')
    expect(source('src/views/auth/WechatPaymentCallbackView.vue')).toContain('<AuthLayout>')
  })

  it('keeps operator scrollbars, text controls, locale states, and proxy controls neutral', () => {
    const styles = source('src/style.css')
    const locale = source('src/components/common/LocaleSwitcher.vue')
    const proxySelector = source('src/components/common/ProxySelector.vue')
    const proxies = source('src/views/admin/ProxiesView.vue')

    expect(styles).toContain('scrollbar-color: var(--operator-border) transparent')
    expect(styles).toContain('caret-color: var(--operator-foreground)')
    expect(styles).toContain('background: color-mix(in oklch, var(--operator-border) 62%, var(--operator-foreground))')
    expect(locale).toContain('operator-locale-selected')
    expect(locale).toContain('bg-gray-100 text-gray-900 dark:bg-dark-700 dark:text-gray-100')
    expect(locale).not.toContain('bg-primary-50')
    expect(proxySelector).toContain('focus:border-gray-500')
    expect(proxySelector).toContain('select-option-selected')
    expect(proxySelector).not.toContain('focus:border-primary-500')
    expect(proxies).toContain('class="badge badge-gray"')
    expect(proxies).toContain('operator-checkbox')
  })

  it('keeps main as the sole operator scroller without changing non-operator sizing', () => {
    const layout = source('src/components/layout/AppLayout.vue')
    const styles = source('src/style.css')

    expect(layout.match(/overflow-y-auto/g)).toHaveLength(1)
    expect(layout).toContain("isOperatorConsole ? 'operator-main'")
    expect(layout).toContain("!isOperatorConsole")
    expect(layout).toContain("['h-screen', sidebarCollapsed")
    expect(styles).toContain('.operator-console,\n.operator-shell {\n  height: 100dvh;')
    expect(styles).toContain('.operator-main {\n  min-height: 0;\n  overflow-y: auto;')
  })

  it('uses the operator radius for provider group headers', () => {
    const capacity = source('src/components/admin/OperatorCapacityOverview.vue')

    expect(capacity).toMatch(
      /\.operator-capacity-provider-header \{[\s\S]*?border: 1px solid var\(--operator-border-subtle\);[\s\S]*?border-radius: var\(--operator-radius\);/,
    )
  })
})
