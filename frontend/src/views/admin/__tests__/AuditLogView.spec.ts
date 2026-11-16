import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { list, get, clear, getStatus, showError, showSuccess } = vi.hoisted(() => ({
  list: vi.fn(),
  get: vi.fn(),
  clear: vi.fn(),
  getStatus: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({ adminAPI: { audit: { list, get, clear } } }))
vi.mock('@/api', () => ({ totpAPI: { getStatus } }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/utils/format', () => ({ formatRelativeTime: () => '2m ago' }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import AuditLogView from '../AuditLogView.vue'

const entry = {
  id: 5,
  created_at: '2026-11-15T10:00:00Z',
  actor_email: 'admin@example.com',
  actor_role: 'admin',
  auth_method: 'jwt',
  credential_masked: 'ey…9Q',
  action: 'accounts.update',
  method: 'PUT',
  path: '/api/v1/admin/accounts/3',
  request_id: 'req-audit-1',
  client_ip: '10.0.0.1',
  user_agent: 'Mozilla',
  status_code: 200,
  latency_ms: 42,
}

const SideDrawerStub = {
  props: ['open', 'title'],
  template: '<div v-if="open" data-test="drawer"><h2>{{ title }}</h2><slot name="badge" /><slot /></div>',
}

const mountView = () => mount(AuditLogView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      SideDrawer: SideDrawerStub,
      Pagination: true,
      Select: true,
      BaseDialog: true,
      ConfirmDialog: true,
      Icon: true,
      Teleport: true,
    },
  },
})

describe('AuditLogView', () => {
  beforeEach(() => {
    list.mockReset().mockResolvedValue({ items: [entry], total: 1 })
    get.mockReset().mockResolvedValue({ ...entry, request_body: '{"name":"x"}', extra: { reason: 'test' } })
  })

  it('opens the entry drawer from a row with pretty-printed JSON', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('tbody tr').trigger('click')
    await flushPromises()

    expect(get).toHaveBeenCalledWith(5)
    const drawer = wrapper.get('[data-test="drawer"]')
    expect(drawer.find('h2').text()).toBe('accounts.update')
    expect(drawer.text()).toContain('req-audit-1')
    expect(drawer.findAll('pre')[0].text()).toBe('{\n  "name": "x"\n}')
    expect(drawer.findAll('pre')[1].text()).toContain('"reason": "test"')
  })

  it('filters by result from the segmented control and clears the chip', async () => {
    const wrapper = mountView()
    await flushPromises()

    const failure = wrapper.findAll('.viz-seg button').find((b) => b.text() === 'admin.audit.filters.resultFailure')!
    await failure.trigger('click')
    await flushPromises()
    expect(failure.attributes('aria-pressed')).toBe('true')
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ success: 'false', page: 1 }))

    const chip = wrapper.get('.ui-chip')
    expect(chip.text()).toContain('admin.audit.filters.resultFailure')
    await chip.trigger('click')
    await flushPromises()
    expect(list).toHaveBeenLastCalledWith(expect.objectContaining({ success: undefined }))
    expect(wrapper.find('.ui-chip').exists()).toBe(false)
  })

  it('keeps reset and clear-all in the overflow menu', async () => {
    getStatus.mockResolvedValue({ enabled: false })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="audit-more-actions"]').trigger('click')
    const items = wrapper.findAll('[role="menuitem"]')
    expect(items.map((i) => i.text())).toEqual(['common.reset', 'admin.audit.clearAll'])
    await items[1].trigger('click')
    await flushPromises()
    expect(getStatus).toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('stepUp.notEnabled')
  })
})
