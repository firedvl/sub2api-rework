import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  updateAccountMock,
  getEligibilityMock,
  updateEligibilityMock,
  showErrorMock,
  authIsSimpleMode
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  getEligibilityMock: vi.fn(),
  updateEligibilityMock: vi.fn(),
  showErrorMock: vi.fn(),
  authIsSimpleMode: { value: true }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: showErrorMock, showSuccess: vi.fn(), showInfo: vi.fn() })
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ get isSimpleMode() { return authIsSimpleMode.value } }) }))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock,
      getGrokMediaEligibility: getEligibilityMock,
      updateGrokMediaEligibility: updateEligibilityMock,
      checkMixedChannelRisk: vi.fn().mockResolvedValue({ has_risk: false })
    },
    settings: {
      getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false, providers: [] }),
      getSettings: vi.fn().mockResolvedValue({})
    },
    tlsFingerprintProfiles: { list: vi.fn().mockResolvedValue([]) }
  }
}))
vi.mock('@/api/admin/accounts', () => ({ getAntigravityDefaultModelMapping: vi.fn() }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

import EditAccountModal from '../EditAccountModal.vue'

const BaseDialogStub = defineComponent({
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const account = (platform = 'grok', type = 'oauth', extra: Record<string, unknown> = {}) => ({
  id: 12, name: 'Grok', notes: '', platform, type,
  credentials: { expires_at: '2027-01-01T00:00:00Z', token_type: 'Bearer' },
  credentials_status: { has_access_token: true, has_refresh_token: true }, extra,
  proxy_id: null, concurrency: 1, priority: 1, rate_multiplier: 1, status: 'active',
  group_ids: [], expires_at: null, auto_pause_on_expired: false
})

function mountModal(value = account()) {
  return mount(EditAccountModal, {
    props: { show: true, account: value, proxies: [], groups: [] },
    global: { stubs: {
      BaseDialog: BaseDialogStub, Select: true, Icon: true, ProxySelector: true,
      GroupSelector: true, ModelWhitelistSelector: true
    } }
  })
}

describe('EditAccountModal Grok media eligibility', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getEligibilityMock.mockReset()
    updateAccountMock.mockReset()
    authIsSimpleMode.value = true
    getEligibilityMock.mockResolvedValue({ account_id: 12, mode: 'auto', eligible: true, reason: 'billing_inconclusive' })
    updateAccountMock.mockResolvedValue(account())
  })

  it('loads evaluated eligibility only for Grok OAuth', async () => {
    const wrapper = mountModal()
    await vi.waitFor(() => expect(getEligibilityMock).toHaveBeenCalledWith(12))
    expect(wrapper.get('[data-testid="grok-media-eligibility-status"]').text()).toContain('billing_inconclusive')
    expect(wrapper.get('[data-testid="grok-media-eligibility-card"]').exists()).toBe(true)
    wrapper.unmount()
    for (const value of [account('grok', 'apikey'), account('openai', 'oauth')]) {
      const unsupported = mountModal(value)
      expect(unsupported.find('[data-testid="grok-media-eligibility-card"]').exists()).toBe(false)
      unsupported.unmount()
    }
    expect(getEligibilityMock).toHaveBeenCalledTimes(1)
  })

  it.each(['auto', 'enabled', 'disabled'])('saves %s through the single canonical account update', async mode => {
    getEligibilityMock.mockResolvedValue({ account_id: 12, mode: mode === 'auto' ? 'enabled' : 'auto', eligible: true, reason: 'eligible' })
    const wrapper = mountModal(account('grok', 'oauth', { unrelated_setting: 'retain' }))
    await vi.waitFor(() => expect(wrapper.find('[data-testid="grok-media-eligibility-status"]').exists()).toBe(true))
    const select = wrapper.findAllComponents({ name: 'Select' }).find(component => component.attributes('data-testid') === 'grok-media-eligibility-mode')
    expect(select).toBeDefined()
    select!.vm.$emit('update:modelValue', mode)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await vi.waitFor(() => expect(updateAccountMock).toHaveBeenCalledTimes(1))
    expect(updateAccountMock.mock.calls[0]?.[1].extra).toMatchObject({ unrelated_setting: 'retain', grok_media_eligible: mode === 'auto' ? null : mode === 'enabled' })
    expect(updateEligibilityMock).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('surfaces a failed eligibility read and retries without a false empty success', async () => {
    getEligibilityMock.mockRejectedValueOnce(new Error('Eligibility unavailable'))
    const wrapper = mountModal()
    await vi.waitFor(() => expect(wrapper.text()).toContain('Eligibility unavailable'))
    expect(wrapper.find('[data-testid="grok-media-eligibility-status"]').exists()).toBe(false)
    await wrapper.get('[data-testid="grok-media-eligibility-retry"]').trigger('click')
    await vi.waitFor(() => expect(wrapper.find('[data-testid="grok-media-eligibility-status"]').exists()).toBe(true))
    expect(getEligibilityMock).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('ignores late eligibility from a previous or closed modal', async () => {
    let resolve!: (state: object) => void
    getEligibilityMock.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = mountModal()
    await vi.waitFor(() => expect(getEligibilityMock).toHaveBeenCalledTimes(1))
    await wrapper.setProps({ account: account('openai', 'oauth') })
    resolve({ account_id: 12, mode: 'enabled', eligible: true, reason: 'override_enabled' })
    await Promise.resolve()
    expect(wrapper.find('[data-testid="grok-media-eligibility-card"]').exists()).toBe(false)
    await wrapper.setProps({ show: false })
    expect(getEligibilityMock).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('does not expose a late read after closing and reloads on reopen', async () => {
    let resolve!: (state: object) => void
    getEligibilityMock.mockReturnValueOnce(new Promise(done => { resolve = done }))
    const wrapper = mountModal()
    await wrapper.setProps({ show: false })
    resolve({ account_id: 12, mode: 'enabled', eligible: true, reason: 'override_enabled' })
    await flushPromises()
    await wrapper.setProps({ show: true })
    await flushPromises()
    const select = wrapper.findAllComponents({ name: 'Select' }).find(component => component.attributes('data-testid') === 'grok-media-eligibility-mode')
    expect(select?.props('modelValue')).toBe('auto')
    expect(getEligibilityMock).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('keeps an atomic update failure open without emitting a false saved account', async () => {
    updateAccountMock.mockRejectedValueOnce(new Error('Atomic account update unavailable'))
    const wrapper = mountModal()
    await flushPromises()
    const select = wrapper.findAllComponents({ name: 'Select' }).find(component => component.attributes('data-testid') === 'grok-media-eligibility-mode')
    select!.vm.$emit('update:modelValue', 'disabled')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(showErrorMock).toHaveBeenCalled()
    expect(wrapper.emitted('updated')).toBeUndefined()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1].extra.grok_media_eligible).toBe(false)
    wrapper.unmount()
  })

  it('omits an unchanged override instead of writing a stale account-list flag', async () => {
    getEligibilityMock.mockResolvedValue({ account_id: 12, mode: 'enabled', eligible: true, reason: 'override_enabled' })
    const wrapper = mountModal(account('grok', 'oauth', { grok_media_eligible: false, unrelated_setting: 'retain' }))
    await flushPromises()
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await flushPromises()
    expect(updateAccountMock.mock.calls[0]?.[1].extra).toMatchObject({ unrelated_setting: 'retain' })
    expect(updateAccountMock.mock.calls[0]?.[1].extra).not.toHaveProperty('grok_media_eligible')
    wrapper.unmount()
  })
})
