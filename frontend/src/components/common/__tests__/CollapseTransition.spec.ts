import { afterEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, nextTick, ref, vShow, withDirectives } from 'vue'
import { mount } from '@vue/test-utils'
import CollapseTransition from '../CollapseTransition.vue'

const open = ref(false)
const Host = defineComponent({
  setup: () => () =>
    h(CollapseTransition, null, {
      default: () => withDirectives(h('div', { class: 'panel' }, 'Details'), [[vShow, open.value]])
    })
})

function stubMotion(reduce: boolean) {
  vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: reduce })))
}

describe('CollapseTransition', () => {
  afterEach(() => {
    open.value = false
    vi.unstubAllGlobals()
    delete (HTMLElement.prototype as Partial<HTMLElement>).animate
  })

  it('animates height from zero when content opens', async () => {
    stubMotion(false)
    const animation = { onfinish: null as null | (() => void), oncancel: null }
    const animate = vi.fn(() => animation)
    HTMLElement.prototype.animate = animate as unknown as HTMLElement['animate']

    const wrapper = mount(Host)
    open.value = true
    await nextTick()

    expect(animate).toHaveBeenCalledTimes(1)
    const [frames] = animate.mock.calls[0] as unknown as [Array<Record<string, string | number>>]
    expect(frames[0]).toMatchObject({ height: '0px', opacity: 0 })
    expect(frames[1]).toMatchObject({ opacity: 1 })
    expect(wrapper.find('.panel').element.style.overflow).toBe('hidden')

    animation.onfinish?.()
    expect(wrapper.find('.panel').element.style.overflow).toBe('')
  })

  it('shows content without animating under reduced motion', async () => {
    stubMotion(true)
    const animate = vi.fn()
    HTMLElement.prototype.animate = animate as unknown as HTMLElement['animate']

    const wrapper = mount(Host)
    open.value = true
    await nextTick()

    expect(animate).not.toHaveBeenCalled()
    expect(wrapper.find('.panel').isVisible()).toBe(true)
  })
})
