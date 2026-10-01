import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import UsersView from '../UsersView.vue'

const { list, toggleStatus, showError, getAll, getBatchUsersUsage, listEnabledDefinitions, getBatchUserAttributes } = vi.hoisted(() => ({ list: vi.fn(), toggleStatus: vi.fn(), showError: vi.fn(), getAll: vi.fn(), getBatchUsersUsage: vi.fn(), listEnabledDefinitions: vi.fn(), getBatchUserAttributes: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: {
  users: { list, toggleStatus }, groups: { getAll },
  dashboard: { getBatchUsersUsage },
  userAttributes: { listEnabledDefinitions, getBatchUserAttributes }
} }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const rows = [{ id: 42, email: 'fixture@example.test', username: '', role: 'user', status: 'active', current_concurrency: 3,
  subscriptions: [{ id: 9 }], allowed_groups: [], balance: 1, concurrency: 5, updated_at: '2026-09-01T00:00:00Z' }]
const response = (items = rows) => ({ items, total: items.length, page: 1, page_size: 20, pages: 1 })
const mountView = () => shallowMount(UsersView, { global: { stubs: {
  AppLayout: { template: '<div><slot /></div>' },
  TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
  DataTable: { name: 'DataTable', props: ['data'], emits: ['sort'], template: '<div><button data-test="sort" @click="$emit(\'sort\', \'last_used_at\', \'desc\')">Sort</button><div v-for="row in data" :key="row.id" data-test="row">{{ row.status }}:{{ row.current_concurrency }}:{{ row.subscriptions?.length }}<slot name="cell-actions" :row="row" /></div></div>' }
} } })
let wrapper: ReturnType<typeof mountView>
const toggle = () => wrapper.get('[data-test="row"]').findAll('button').find(button => /admin.users.(enable|disable)$/.test(button.text()))!

beforeEach(() => {
  vi.clearAllMocks()
  list.mockReset()
  toggleStatus.mockReset()
  localStorage.clear()
  list.mockResolvedValue(response(rows.map(row => ({ ...row }))))
  getAll.mockResolvedValue([])
  getBatchUsersUsage.mockResolvedValue({ stats: {} })
  listEnabledDefinitions.mockResolvedValue([])
  getBatchUserAttributes.mockResolvedValue({ values: {} })
})
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks() })

describe('user status row updates', () => {
  it('updates status in place without replacing list-only fields or loading the list', async () => {
    toggleStatus.mockResolvedValue({ id: 42, status: 'disabled', updated_at: '2026-10-01T00:00:00Z' })
    wrapper = mountView()
    await flushPromises()
    await toggle().trigger('click')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)
    expect(toggleStatus).toHaveBeenCalledWith(42, 'disabled')
    expect(wrapper.get('[data-test="row"]').text()).toContain('disabled:3:1')
    expect(toggle().text()).toBe('admin.users.enable')
  })

  it('does not change the row or reload the list after failure', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    toggleStatus.mockRejectedValue(new Error('status update failed'))
    wrapper = mountView()
    await flushPromises()
    await toggle().trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="row"]').text()).toContain('active:3:1')
    expect(list).toHaveBeenCalledTimes(1)
    expect(showError).toHaveBeenCalled()
  })

  it('supersedes a concurrent list request when the status update resolves', async () => {
    let resolveToggle!: (value: object) => void
    toggleStatus.mockReturnValue(new Promise(resolve => { resolveToggle = resolve }))
    wrapper = mountView()
    await flushPromises()
    await toggle().trigger('click')
    let staleList!: (value: object) => void
    list.mockReturnValueOnce(new Promise(resolve => { staleList = resolve }))
    await wrapper.get('[data-test="sort"]').trigger('click')
    await flushPromises()
    list.mockResolvedValue(response(rows.map(row => ({ ...row, status: 'disabled' }))))
    resolveToggle({ id: 42, status: 'disabled', updated_at: '2026-10-01T00:00:00Z' })
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(3)
    staleList(response())
    await flushPromises()
    expect(wrapper.get('[data-test="row"]').text()).toContain('disabled:3:1')
  })
})
