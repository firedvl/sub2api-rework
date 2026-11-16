import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountDetailDrawer from '../AccountDetailDrawer.vue'
import { buildFleetRow } from '../accountFleet'
import type { Account, AccountUsageInfo } from '@/types'

const { getStats } = vi.hoisted(() => ({ getStats: vi.fn() }))

vi.mock('@/api/admin', () => ({
  adminAPI: { accounts: { getStats } }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const NOW = new Date('2026-11-15T12:00:00Z')

const makeAccount = (extra: Partial<Account> = {}) => ({
  id: 7,
  name: 'Claude main',
  platform: 'anthropic',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  priority: 3,
  concurrency: 4,
  group_ids: [1],
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
  ...extra
}) as Account

const mountDrawer = (account: Account, usage: AccountUsageInfo | null = null, usageError: string | null = null) => {
  getStats.mockResolvedValue({ history: [] })
  return mount(AccountDetailDrawer, {
    props: {
      open: true,
      row: buildFleetRow(account, usage, usageError, null, NOW),
      groups: [{ id: 1, name: 'production' }] as never
    },
    global: { stubs: { teleport: true, Icon: true, ProviderMark: true, AutoWarmupStatus: true } }
  })
}

const button = (wrapper: ReturnType<typeof mountDrawer>, text: string) =>
  wrapper.findAll('button').find((item) => item.text().includes(text))

describe('AccountDetailDrawer', () => {
  it('shows status, resolved groups and the recent-requests request for a healthy account', async () => {
    const wrapper = mountDrawer(makeAccount())
    await flushPromises()

    expect(wrapper.get('[data-testid="account-drawer-status"]').text()).toBe('admin.viz.states.active')
    expect(wrapper.get('[data-testid="account-drawer-details"]').text()).toContain('production')
    expect(wrapper.find('.acct-notice').exists()).toBe(false)
    expect(getStats).toHaveBeenCalledWith(7, 14)

    await button(wrapper, 'admin.accountsUi.test')!.trigger('click')
    await button(wrapper, 'admin.accountsUi.pause')!.trigger('click')
    await wrapper.get('button[aria-label="common.edit"]').trigger('click')
    expect(wrapper.emitted('test')).toHaveLength(1)
    expect(wrapper.emitted('toggle-schedulable')).toHaveLength(1)
    expect(wrapper.emitted('edit')).toHaveLength(1)
  })

  it('leads with Reauthorize and explains the error for a broken OAuth account', async () => {
    const wrapper = mountDrawer(makeAccount({ status: 'error', error_message: 'token revoked' }))
    await flushPromises()

    expect(wrapper.get('.acct-notice').text()).toContain('token revoked')
    expect(button(wrapper, 'admin.accountsUi.test')).toBeUndefined()
    await button(wrapper, 'admin.accounts.reAuthorize')!.trigger('click')
    expect(wrapper.emitted('reauth')).toHaveLength(1)
  })

  it('keeps a usage refresh error visible next to cached quota windows', async () => {
    const observedAt = NOW.toISOString()
    const usage = {
      source: 'active', cached: true, stale: false, updated_at: observedAt,
      five_hour: { utilization: 60, source: 'active', observed_at: observedAt, stale: false, resets_at: '2026-11-15T17:00:00Z' },
    } as AccountUsageInfo
    const wrapper = mountDrawer(makeAccount(), usage, 'Claude refresh temporarily unavailable')
    await flushPromises()

    expect(wrapper.findAll('.acct-gauge').length).toBeGreaterThan(0)
    expect(wrapper.get('[data-testid="account-drawer-usage-error"]').text()).toBe('Claude refresh temporarily unavailable')
  })
})
