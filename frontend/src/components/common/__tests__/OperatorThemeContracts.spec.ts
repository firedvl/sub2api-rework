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
    expect(source('src/views/setup/SetupWizardView.vue')).toContain('dark:bg-black dark:bg-none')
    expect(source('src/views/auth/OAuthCallbackView.vue')).toContain('min-h-screen bg-gray-50 px-4 py-10 dark:bg-black')
    expect(source('src/views/auth/WechatPaymentCallbackView.vue')).toContain('min-h-screen bg-gray-50 px-4 py-10 dark:bg-black')
  })
})
