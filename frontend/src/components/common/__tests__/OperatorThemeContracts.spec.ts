import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { useAppStore } from '@/stores/app'
import Toast from '../Toast.vue'
import Toggle from '../Toggle.vue'

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
})
