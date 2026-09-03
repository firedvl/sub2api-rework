import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PlatformIcon from '../PlatformIcon.vue'

describe('PlatformIcon', () => {
  it('renders the official one-color Antigravity mark', () => {
    const wrapper = mount(PlatformIcon, { props: { platform: 'antigravity' } })
    const icon = wrapper.get('[data-provider-brand="antigravity"]')

    expect(icon.attributes('viewBox')).toBe('0 0 112 112')
    expect(icon.attributes('fill')).toBe('currentColor')
    expect(icon.get('path').attributes('d')).toMatch(/^M89\.6992 93\.695/)
    expect(wrapper.html()).not.toMatch(/M19\.35 10\.04|M23 12\.245/)
  })
})
