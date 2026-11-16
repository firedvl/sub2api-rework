import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CountUp from '../CountUp.vue'
import QuotaMeter from '../QuotaMeter.vue'
import SideDrawer from '../SideDrawer.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('CountUp', () => {
  it('renders the formatted value (reduced motion in tests) and a dash for missing values', async () => {
    const wrapper = mount(CountUp, { props: { value: 4286 } })
    expect(wrapper.text()).toBe('4,286')
    await wrapper.setProps({ value: 12.5, format: (value: number) => `$${value.toFixed(2)}` })
    expect(wrapper.text()).toBe('$12.50')
    await wrapper.setProps({ value: null })
    expect(wrapper.text()).toBe('–')
  })
})

describe('QuotaMeter', () => {
  it.each([
    [80, 'is-ok', '80%'],
    [30, 'is-warn', '30%'],
    [10, 'is-bad', '10%'],
    [null, 'is-none', '–'],
  ])('tones %s as %s', (value, tone, text) => {
    const wrapper = mount(QuotaMeter, { props: { value, label: 'Quota' } })
    expect(wrapper.classes()).toContain(tone)
    expect(wrapper.text()).toBe(text)
    expect(wrapper.attributes('aria-label')).toBe(`Quota: ${text}`)
  })
})

describe('SideDrawer', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('opens as a labelled dialog and emits close on Escape and the close button', async () => {
    const wrapper = mount(SideDrawer, {
      props: { open: false, title: 'Account' },
      slots: { default: '<p>Body</p>' },
      attachTo: document.body,
      global: { stubs: { transition: false } },
    })
    expect(document.querySelector('[role="dialog"]')).toBeNull()

    await wrapper.setProps({ open: true })
    await nextTick()
    const dialog = document.querySelector('[role="dialog"]') as HTMLElement
    expect(dialog).not.toBeNull()
    expect(document.getElementById(dialog.getAttribute('aria-labelledby') ?? '')?.textContent).toBe('Account')
    expect(dialog.textContent).toContain('Body')

    dialog.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(wrapper.emitted('close')).toHaveLength(1)

    ;(document.querySelector('button[aria-label="common.close"]') as HTMLButtonElement).click()
    expect(wrapper.emitted('close')).toHaveLength(2)
    wrapper.unmount()
  })
})
