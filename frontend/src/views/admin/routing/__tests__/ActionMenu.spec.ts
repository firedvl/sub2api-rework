import { afterEach, describe, expect, it } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ActionMenu from '../ActionMenu.vue'

const items = [
  { key: 'a', label: 'First', pressed: true, action: () => {} },
  { key: 'b', label: 'Second', action: () => {} },
]

const mountMenu = () => mount(ActionMenu, {
  props: { label: 'View options', items },
  global: { stubs: { Icon: true } },
  attachTo: document.body,
})

const panel = () => document.body.querySelector<HTMLElement>('.action-menu-panel')

describe('ActionMenu', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('stays open while its own list scrolls and closes when the page scrolls', async () => {
    const wrapper = mountMenu()
    const trigger = wrapper.get('button[aria-haspopup="menu"]')
    await trigger.trigger('click')
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('true')

    panel()!.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('true')

    document.dispatchEvent(new Event('scroll'))
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('false')
    wrapper.unmount()
  })
})
