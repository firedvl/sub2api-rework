import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PlatformTypeBadge from '../PlatformTypeBadge.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('platform-scoped plan labels', () => {
  it.each([
    ['pro', 'Pro 20x', 'bg-violet-100'],
    ['CHATGPT_PRO', 'Pro 20x', 'bg-violet-100'],
    [' pro-lite ', 'Pro 5x', 'bg-violet-100'],
    ['self_serve_business_prolite', 'Business Premium', 'bg-indigo-100'],
    ['Self Serve Business ProLite', 'Business Premium', 'bg-indigo-100'],
    ['team', 'Business Standard', 'bg-indigo-100'],
    ['plus', 'Plus', 'bg-sky-100'],
    ['free', 'Free', 'bg-gray-100'],
  ])('renders OpenAI %s as %s', (planType, label, color) => {
    const wrapper = mount(PlatformTypeBadge, { props: { platform: 'openai', type: 'oauth', planType } })
    expect(wrapper.text()).toContain(label)
    expect(wrapper.html()).toContain(color)
  })

  it.each(['antigravity', 'grok'] as const)('does not rename %s Pro', (platform) => {
    const wrapper = mount(PlatformTypeBadge, { props: { platform, type: 'oauth', planType: 'pro' } })
    expect(wrapper.text()).toContain('Pro')
    expect(wrapper.text()).not.toContain('20x')
    expect(wrapper.text()).not.toContain('5x')
  })

  it('keeps unknown OpenAI tiers unchanged and updates when the platform changes', async () => {
    const wrapper = mount(PlatformTypeBadge, { props: { platform: 'openai', type: 'oauth', planType: 'future_custom' } })
    expect(wrapper.text()).toContain('future_custom')
    await wrapper.setProps({ planType: 'prolite' })
    expect(wrapper.text()).toContain('Pro 5x')
    await wrapper.setProps({ platform: 'antigravity' })
    expect(wrapper.text()).toContain('prolite')
    expect(wrapper.html()).not.toContain('bg-violet-100')
  })
})
