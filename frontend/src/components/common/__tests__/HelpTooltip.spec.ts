import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

function getTooltipElement(): HTMLDivElement {
  const tooltip = document.body.querySelector('[role="tooltip"]')
  if (!(tooltip instanceof HTMLDivElement)) {
    throw new Error('tooltip element not found')
  }
  return tooltip
}

function getDialogElement(): HTMLDivElement {
  const dialog = document.body.querySelector('[role="dialog"]')
  if (!(dialog instanceof HTMLDivElement)) {
    throw new Error('dialog element not found')
  }
  return dialog
}

describe('HelpTooltip', () => {
  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('keeps the existing hover interaction by default', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'hover details',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('mouseenter')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    await trigger.trigger('mouseleave')
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports click-to-toggle details and closes on outside click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: {
        content: 'click details',
        trigger: 'click',
      },
    })

    const trigger = wrapper.get('.group')
    const tooltip = getTooltipElement()

    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')
    expect(tooltip.textContent).toContain('click details')

    const closeButton = tooltip.querySelector('button[aria-label="Close"]')
    if (!(closeButton instanceof HTMLButtonElement)) {
      throw new Error('close button not found')
    }
    closeButton.click()
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    await trigger.trigger('click')
    await nextTick()
    expect(tooltip.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(tooltip.style.display).toBe('none')

    wrapper.unmount()
  })

  it('keeps interactive content open across the trigger-to-panel hover path', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: { interactive: true, ariaLabel: 'Quota help' },
      slots: {
        trigger: '?',
        default: '<a href="#docs">Official Docs</a>',
      },
    })

    const trigger = wrapper.get('button[aria-label="Quota help"]')
    const dialog = getDialogElement()

    await trigger.trigger('mouseenter')
    expect(dialog.style.display).not.toBe('none')

    trigger.element.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: dialog }))
    await nextTick()
    expect(dialog.style.display).not.toBe('none')

    dialog.dispatchEvent(new MouseEvent('mouseenter'))
    dialog.dispatchEvent(new MouseEvent('mouseleave', { relatedTarget: document.body }))
    await nextTick()
    expect(dialog.style.display).toBe('none')

    wrapper.unmount()
  })

  it('supports focus, links, Escape, and outside-click dismissal', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: { interactive: true, ariaLabel: 'Quota help' },
      slots: {
        trigger: '?',
        default: '<a href="#docs">Official Docs</a>',
      },
    })

    const trigger = wrapper.get<HTMLButtonElement>('button[aria-label="Quota help"]')
    const dialog = getDialogElement()
    const link = dialog.querySelector('a')
    if (!(link instanceof HTMLAnchorElement)) throw new Error('docs link not found')

    trigger.element.focus()
    await nextTick()
    expect(dialog.style.display).not.toBe('none')
    expect(trigger.attributes('aria-expanded')).toBe('true')

    await trigger.trigger('keydown', { key: 'Tab' })
    await nextTick()
    expect(document.activeElement).toBe(link)
    expect(dialog.style.display).not.toBe('none')

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await nextTick()
    expect(dialog.style.display).toBe('none')
    expect(document.activeElement).toBe(trigger.element)

    await trigger.trigger('click')
    const clicked = vi.fn((event: Event) => event.preventDefault())
    link.addEventListener('click', clicked)
    link.click()
    expect(clicked).toHaveBeenCalledOnce()
    expect(dialog.style.display).not.toBe('none')

    document.body.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(dialog.style.display).toBe('none')

    wrapper.unmount()
  })

  it('keeps the first click open after focus and toggles closed on the second click', async () => {
    const wrapper = mount(HelpTooltip, {
      attachTo: document.body,
      props: { interactive: true, ariaLabel: 'Quota help' },
    })

    const trigger = wrapper.get<HTMLButtonElement>('button[aria-label="Quota help"]')
    const dialog = getDialogElement()

    trigger.element.focus()
    await trigger.trigger('click')
    expect(dialog.style.display).not.toBe('none')

    await trigger.trigger('click')
    expect(dialog.style.display).toBe('none')

    wrapper.unmount()
  })
})
