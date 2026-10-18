import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import en from '@/i18n/locales/en'
import type { Account } from '@/types'
import AutoWarmupStatus from '../AutoWarmupStatus.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => {
  const messages = en.admin.accounts.autoWarmup
  if (key.endsWith('.claudeUnavailableReason')) return messages.claudeUnavailableReason
  if (key.endsWith('.claudeAPIBilling')) return messages.claudeAPIBilling
  return messages.claudeUnavailable
} }) }))

describe('provider warm-up availability', () => {
  it('keeps Claude unavailable even with forged OpenAI warm-up state', () => {
    const account = {
      platform: 'anthropic', type: 'oauth', status: 'active', schedulable: true,
      extra: { auto_warmup_enabled: true, codex_auto_warmup_state: { status: 'succeeded', completed_at: '2026-10-17T12:00:00Z' } }
    } as Account
    const wrapper = mount(AutoWarmupStatus, {
      props: { account, globalEnabled: true, details: true }
    })
    expect(wrapper.get('[data-testid="claude-auto-warmup-unavailable"]').text()).toContain('Warm-up: Unavailable')
    expect(wrapper.text()).toContain('separate local official Claude Code companion')
    expect(wrapper.text()).toContain('this gateway cannot enable or inspect them')
    expect(wrapper.text()).not.toContain('Succeeded')
    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.find('dl').exists()).toBe(false)
  })

  it('labels API usage separately from subscription windows', () => {
    const account = { platform: 'anthropic', type: 'apikey' } as Account
    const wrapper = mount(AutoWarmupStatus, { props: { account, details: true } })
    expect(wrapper.text()).toContain('API-key requests incur API charges')
    expect(wrapper.text()).toContain('do not start or replenish a Claude subscription window')
    expect(wrapper.find('button').exists()).toBe(false)
  })
})
