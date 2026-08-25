import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import BaseDialog from '../BaseDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('BaseDialog', () => {
  afterEach(() => {
    document.body.innerHTML = ''
    document.body.classList.remove('modal-open')
  })

  it('resets body scroll position when reopened', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: false, title: 'Details' },
      slots: { default: '<div style="height: 2000px">content</div>' },
      global: { stubs: { Icon: true } }
    })

    await wrapper.setProps({ show: true })
    await nextTick()
    const body = document.body.querySelector<HTMLElement>('.modal-body')
    expect(body).not.toBeNull()
    body!.scrollTop = 480

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    expect(document.body.querySelector<HTMLElement>('.modal-body')?.scrollTop).toBe(0)
    wrapper.unmount()
  })

  it('focuses the panel when no focusable content is available', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Details', showCloseButton: false },
      slots: { default: '<p>Read only</p>' },
      global: { stubs: { Icon: true } }
    })

    await nextTick()

    expect(document.activeElement).toBe(document.body.querySelector('.modal-content'))
    wrapper.unmount()
  })

  it('traps Tab and Shift+Tab inside the topmost dialog', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Details', showCloseButton: false },
      slots: { default: '<button id="first">First</button><button id="last">Last</button>' },
      global: { stubs: { Icon: true } }
    })

    await nextTick()
    const first = document.getElementById('first')!
    const last = document.getElementById('last')!

    last.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(first)

    first.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true }))
    expect(document.activeElement).toBe(last)
    wrapper.unmount()
  })

  it('keeps scroll locked and routes Escape only to the topmost dialog', async () => {
    const onFirstClose = vi.fn()
    const onSecondClose = vi.fn()
    const first = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'First', onClose: onFirstClose },
      global: { stubs: { Icon: true } }
    })
    const second = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Second', onClose: onSecondClose },
      global: { stubs: { Icon: true } }
    })

    await nextTick()
    expect(document.body.classList.contains('modal-open')).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(onSecondClose).toHaveBeenCalledTimes(1)
    expect(onFirstClose).not.toHaveBeenCalled()

    await second.setProps({ show: false })
    expect(document.body.classList.contains('modal-open')).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(onFirstClose).toHaveBeenCalledTimes(1)

    await first.setProps({ show: false })
    expect(document.body.classList.contains('modal-open')).toBe(false)
    second.unmount()
    first.unmount()
  })

  it('does not move focus outside the top dialog when a lower dialog closes', async () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()
    const first = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'First' },
      global: { stubs: { Icon: true } }
    })
    await nextTick()
    const second = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Second' },
      global: { stubs: { Icon: true } }
    })
    await nextTick()

    const topDialogFocus = document.activeElement
    await first.setProps({ show: false })
    expect(document.activeElement).toBe(topDialogFocus)

    await second.setProps({ show: false })
    expect(document.activeElement).toBe(trigger)
    second.unmount()
    first.unmount()
  })

  it('restores focus when closed and cleans up when unmounted', async () => {
    const trigger = document.createElement('button')
    document.body.appendChild(trigger)
    trigger.focus()
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Details' },
      global: { stubs: { Icon: true } }
    })

    await nextTick()
    await wrapper.setProps({ show: false })
    expect(document.activeElement).toBe(trigger)

    await wrapper.setProps({ show: true })
    await nextTick()
    wrapper.unmount()
    expect(document.body.classList.contains('modal-open')).toBe(false)
    expect(document.activeElement).toBe(trigger)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(wrapper.emitted('close')).toBeUndefined()
  })
})
