import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountFleetTable from '../AccountFleetTable.vue'
import { buildFleetRow, pickHeadlineWindows, sortFleetRows, type FleetRow } from '../accountFleet'
import type { Account } from '@/types'
import type { OperatorCapacityWindow } from '@/utils/operatorCapacity'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const NOW = new Date('2026-11-15T12:00:00Z')

const makeAccount = (id: number, name: string, extra: Partial<Account> = {}) => ({
  id,
  name,
  platform: 'anthropic',
  type: 'oauth',
  status: 'active',
  schedulable: true,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
  ...extra
}) as Account

const window = (key: string, remainingPercent: number, extra: Partial<OperatorCapacityWindow> = {}): OperatorCapacityWindow => ({
  key,
  label: key,
  remainingPercent,
  resetsAt: null,
  kind: 'provider',
  ...extra
})

const usage = (fiveHour: number, sevenDay: number) => ({
  updated_at: NOW.toISOString(),
  five_hour: { utilization: fiveHour, resets_at: '2026-11-15T14:00:00Z' },
  seven_day: { utilization: sevenDay, resets_at: '2026-11-20T00:00:00Z' }
}) as never

describe('accountFleet helpers', () => {
  it('picks session and weekly windows as headlines and prefers current windows', () => {
    const picked = pickHeadlineWindows([
      window('seven_day', 40),
      window('five_hour', 90, { current: false }),
      window('five_hour', 70)
    ])
    expect(picked.short?.remainingPercent).toBe(70)
    expect(picked.long?.key).toBe('seven_day')
  })

  it('falls back to the tightest model window when no session window exists', () => {
    const picked = pickHeadlineWindows([window('antigravity:a', 60), window('antigravity:b', 20)])
    expect(picked.short?.key).toBe('antigravity:b')
    expect(picked.long).toBeNull()
  })

  it('sorts by status severity and keeps missing values last in both directions', () => {
    const rows: FleetRow[] = [
      buildFleetRow(makeAccount(1, 'b-active'), usage(10, 10), null, null, NOW),
      buildFleetRow(makeAccount(2, 'a-disabled', { schedulable: false }), null, null, null, NOW),
      buildFleetRow(makeAccount(3, 'c-error', { status: 'error', error_message: 'token revoked' }), null, null, null, NOW)
    ]
    expect(sortFleetRows(rows, { key: 'status', dir: 1 }).map((row) => row.status)).toEqual(['error', 'disabled', 'active'])
    expect(sortFleetRows(rows, { key: 'name', dir: -1 }).map((row) => row.account.id)).toEqual([3, 1, 2])
    expect(sortFleetRows(rows, { key: 'short', dir: 1 })[0].account.id).toBe(1)
    expect(sortFleetRows(rows, { key: 'short', dir: -1 })[0].account.id).toBe(1)
  })
})

describe('AccountFleetTable', () => {
  const rows = [
    buildFleetRow(makeAccount(1, 'Claude main'), usage(20, 50), null, { requests: 12, tokens: 0, cost: 0 } as never, NOW),
    buildFleetRow(makeAccount(2, 'Claude broken', { status: 'error', error_message: 'token revoked' }), null, null, null, NOW)
  ]

  const mountTable = (props: Record<string, unknown> = {}) => mount(AccountFleetTable, {
    props: {
      rows,
      sort: { key: 'capacity', dir: 1 },
      selected: new Set<number>(),
      groupByProvider: false,
      loading: false,
      error: null,
      filtered: false,
      todayLoading: false,
      ...props
    },
    global: { stubs: { Icon: true, ProviderMark: true } }
  })

  it('renders one row per account with status and headline meters', () => {
    const wrapper = mountTable()
    const tableRows = wrapper.findAll('[data-testid="account-capacity-row"]')
    expect(tableRows).toHaveLength(2)
    expect(tableRows[0].attributes('data-status')).toBe('active')
    expect(tableRows[0].text()).toContain('80%')
    expect(tableRows[1].attributes('data-status')).toBe('error')
  })

  it('opens a row by click and by Enter, and toggles selection without opening', async () => {
    const wrapper = mountTable()
    const row = wrapper.findAll('[data-testid="account-capacity-row"]')[0]
    await row.trigger('click')
    await row.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('open')).toHaveLength(2)

    await row.get('td.fleet-check').trigger('click')
    await row.get('input[type="checkbox"]').trigger('change')
    expect(wrapper.emitted('toggle')?.[0]).toEqual([1])
    expect(wrapper.emitted('open')).toHaveLength(2)
  })

  it('emits sort keys from the column headers', async () => {
    const wrapper = mountTable()
    const nameHeader = wrapper.findAll('th button').find((button) => button.text().includes('admin.accountsUi.account'))
    await nameHeader!.trigger('click')
    expect(wrapper.emitted('sort')?.[0]).toEqual(['name'])
  })

  it('shows provider group headers only when grouped', () => {
    expect(mountTable().find('[data-testid="provider-toggle-anthropic"]').exists()).toBe(false)
    expect(mountTable({ groupByProvider: true }).get('[data-testid="provider-toggle-anthropic"]').attributes('aria-expanded')).toBe('true')
  })

  it('offers retry on load errors and reset when filters hide everything', async () => {
    const failed = mountTable({ rows: [], error: true })
    await failed.get('[data-testid="capacity-load-error"] button').trigger('click')
    expect(failed.emitted('retry')).toHaveLength(1)

    const empty = mountTable({ rows: [], filtered: true })
    await empty.get('[data-testid="capacity-filter-empty"] button').trigger('click')
    expect(empty.emitted('reset-filters')).toHaveLength(1)
  })
})
