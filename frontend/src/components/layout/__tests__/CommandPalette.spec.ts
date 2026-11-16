import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, nextTick, reactive, ref } from 'vue'

const { push, list } = vi.hoisted(() => ({ push: vi.fn(), list: vi.fn() }))

vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})
vi.mock('@/stores', () => ({
  useAppStore: () => reactive({ toggleSidebar: vi.fn() }),
  useAuthStore: () => reactive({ isSimpleMode: false, logout: vi.fn() }),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { list } } }))
vi.mock('@/router/operatorNavigation', () => ({ operatorAreas: [] }))
vi.mock('@/composables/useAppNavigation', () => ({
  useAppNavigation: () => ({
    isAdmin: ref(true),
    adminSettingsStore: reactive({ opsMonitoringEnabled: true }),
    sections: computed(() => [
      {
        items: [
          { path: '/admin/dashboard', label: 'Overview', icon: 'grid' },
          { path: '/admin/accounts', label: 'Accounts', icon: 'server' },
        ],
      },
    ]),
  }),
}))

import CommandPalette from '../CommandPalette.vue'
import { useCommandPalette } from '@/composables/useCommandPalette'

const palette = useCommandPalette()

function mountPalette() {
  return mount(CommandPalette, {
    attachTo: document.body,
    global: { stubs: { transition: false, ProviderMark: true, LoadingSpinner: true } },
  })
}

const input = () => document.querySelector('[data-testid="command-palette"] input') as HTMLInputElement
const options = () => [...document.querySelectorAll('[role="option"]')].map((node) => node.textContent?.trim())

function press(key: string, init: KeyboardEventInit = {}) {
  const target = document.activeElement ?? document.body
  target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, ...init }))
}

describe('CommandPalette', () => {
  beforeEach(() => {
    push.mockReset()
    list.mockReset()
    palette.hide()
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('opens with Cmd+K, filters pages, and navigates on Enter', async () => {
    const wrapper = mountPalette()
    press('k', { metaKey: true })
    await nextTick()
    await nextTick()
    expect(input()).not.toBeNull()
    expect(document.activeElement).toBe(input())
    expect(options()).toEqual(expect.arrayContaining(['Overview', 'Accounts']))

    input().value = 'acc'
    input().dispatchEvent(new Event('input'))
    await nextTick()
    expect(options()[0]).toBe('Accounts')

    press('Enter')
    await nextTick()
    expect(push).toHaveBeenCalledWith('/admin/accounts')
    expect(palette.open.value).toBe(false)
    wrapper.unmount()
  })

  it('closes on Escape and ignores "/" inside text fields', async () => {
    const wrapper = mountPalette()
    const field = document.createElement('input')
    document.body.appendChild(field)
    field.focus()
    press('/')
    await nextTick()
    expect(palette.open.value).toBe(false)

    field.blur()
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '/', bubbles: true }))
    await nextTick()
    expect(palette.open.value).toBe(true)

    await nextTick()
    press('Escape')
    await nextTick()
    expect(palette.open.value).toBe(false)
    wrapper.unmount()
  })
})
