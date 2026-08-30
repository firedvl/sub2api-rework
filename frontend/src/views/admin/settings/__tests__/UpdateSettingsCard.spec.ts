import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import UpdateSettingsCard from '../UpdateSettingsCard.vue'

const { checkUpdates, prepareUpdate, installUpdate, rollbackUpdate, run } = vi.hoisted(() => ({
  checkUpdates: vi.fn(), prepareUpdate: vi.fn(), installUpdate: vi.fn(), rollbackUpdate: vi.fn(), run: vi.fn()
}))

vi.mock('@/api/admin/system', () => ({
  checkUpdates, prepareUpdate, installUpdate, rollbackUpdate,
  default: { checkUpdates, prepareUpdate, installUpdate, rollbackUpdate }
}))
vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ visible: { value: false }, run }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => ''
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: (key: string) => key })
}))

const ConfirmDialogStub = { name: 'ConfirmDialog', props: ['show', 'title'], template: '<div v-if="show"><slot /><button type="button" @click="$emit(\'confirm\')">confirm</button></div>' }

function updateStatus(state: string, updaterState = 'idle', prepared = '') {
  return {
    current_version: '1.2.3-rework.1', upstream_baseline: 'v1.2.2', latest_upstream: 'v1.2.3', update_channel: 'stable', checked_at: '2026-08-28T00:00:00Z',
    latest_compatible_rework: '1.2.3-rework.4', state, installable: state === 'update_ready', release_notes: { upstream: '<b>safe text</b>', rework: '', compatibility: 'Review pending.', migrations: '', rollback: '' },
    updater: { healthy: updaterState !== 'unavailable', state: updaterState, busy: false, prepared_version: prepared, rollback_version: '1.2.3-rework.0' }
  }
}

function mountCard() {
  setActivePinia(createPinia())
  return mount(UpdateSettingsCard, { global: { stubs: { ConfirmDialog: ConfirmDialogStub, TotpStepUpDialog: true, Icon: true } } })
}

describe('UpdateSettingsCard', () => {
  beforeEach(() => {
    checkUpdates.mockReset(); prepareUpdate.mockReset(); installUpdate.mockReset(); rollbackUpdate.mockReset(); run.mockReset()
    run.mockImplementation((operation: () => unknown) => operation())
  })

  it('shows a compatibility-pending release without unsafe actions or HTML rendering', async () => {
    checkUpdates.mockResolvedValue(updateStatus('compatibility_pending'))
    const wrapper = mountCard()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.settings.updates.states.compatibility_pending')
    expect(wrapper.text()).toContain('admin.settings.updates.upstreamBaseline')
    expect(wrapper.text()).toContain('v1.2.2')
    expect(wrapper.text()).toContain('admin.settings.updates.latestUpstream')
    expect(wrapper.text()).toContain('v1.2.3')
    expect(wrapper.text()).toContain('admin.settings.updates.noteLabels.upstream')
    expect(wrapper.text()).toContain('admin.settings.updates.noteLabels.compatibility')
    expect(wrapper.text()).toContain('<b>safe text</b>')
    expect(wrapper.find('b').exists()).toBe(false)
    expect(wrapper.getComponent({ name: 'ConfirmDialog' }).props('title')).toBe('')
    const prepareButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.prepare'))
    expect(prepareButton!.attributes('disabled')).toBeDefined()
  })

  it('prepares an approved release through the step-up wrapper', async () => {
    checkUpdates.mockResolvedValue(updateStatus('update_ready', 'idle'))
    prepareUpdate.mockResolvedValue({ operation_id: 'prepare-1' })
    const wrapper = mountCard()
    await flushPromises()
    const prepareButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.prepare'))
    await prepareButton!.trigger('click')
    await flushPromises()
    expect(run).toHaveBeenCalled()
    expect(prepareUpdate).toHaveBeenCalledWith('1.2.3-rework.4')
  })

  it('refreshes an accepted operation until the updater is no longer busy', async () => {
    vi.useFakeTimers()
    const preparing = updateStatus('update_ready', 'preparing')
    preparing.updater.busy = true
    checkUpdates
      .mockResolvedValueOnce(updateStatus('update_ready', 'idle'))
      .mockResolvedValueOnce(preparing)
      .mockRejectedValueOnce(new Error('application restarting'))
      .mockResolvedValueOnce(updateStatus('update_ready', 'prepared', '1.2.3-rework.4'))
    prepareUpdate.mockResolvedValue({ operation_id: 'prepare-1' })
    const wrapper = mountCard()

    try {
      await flushPromises()
      const prepareButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.prepare'))
      await prepareButton!.trigger('click')
      await flushPromises()
      expect(checkUpdates).toHaveBeenCalledTimes(2)

      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(checkUpdates).toHaveBeenCalledTimes(3)

      await vi.advanceTimersByTimeAsync(1000)
      await flushPromises()
      expect(checkUpdates).toHaveBeenCalledTimes(4)
      const installButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.install'))
      expect(installButton!.attributes('disabled')).toBeUndefined()
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
    }
  })

  it('passes exact install confirmation through the step-up wrapper', async () => {
    checkUpdates.mockResolvedValue(updateStatus('update_ready', 'prepared', '1.2.3-rework.4'))
    installUpdate.mockResolvedValue({ operation_id: 'install-1' })
    const wrapper = mountCard()
    await flushPromises()
    const installButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.install'))
    await installButton!.trigger('click')
    await wrapper.get('#update-confirmation').setValue('INSTALL 1.2.3-rework.4')
    await wrapper.findAll('button').find(button => button.text() === 'confirm')!.trigger('click')
    await flushPromises()
    expect(run).toHaveBeenCalled()
    expect(installUpdate).toHaveBeenCalledWith('1.2.3-rework.4', 'INSTALL 1.2.3-rework.4')
  })

  it('keeps operations unavailable when the updater socket is unavailable', async () => {
    checkUpdates.mockResolvedValue(updateStatus('update_ready', 'unavailable', '1.2.3-rework.4'))
    const wrapper = mountCard()
    await flushPromises()
    const prepareButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.prepare'))
    const installButton = wrapper.findAll('button').find(button => button.text().includes('admin.settings.updates.install'))
    expect(prepareButton!.attributes('disabled')).toBeDefined()
    expect(installButton!.attributes('disabled')).toBeDefined()
  })
})
