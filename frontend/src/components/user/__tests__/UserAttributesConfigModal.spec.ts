import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import UserAttributesConfigModal from '../UserAttributesConfigModal.vue'

const mocks = vi.hoisted(() => ({ listDefinitions: vi.fn(), updateDefinition: vi.fn(), createDefinition: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { userAttributes: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

beforeEach(() => {
  vi.clearAllMocks()
  mocks.listDefinitions.mockResolvedValue([{ id: 1, key: 'note', name: 'Note', type: 'text', description: 'Old', placeholder: 'Old hint', required: false, enabled: true }])
  mocks.updateDefinition.mockResolvedValue(undefined)
})

describe('UserAttributesConfigModal optional text', () => {
  it.each([['', 'Old hint'], ['Old', ''], ['', '']])('sends cleared fields explicitly', async (description, placeholder) => {
    const wrapper = mount(UserAttributesConfigModal, {
      props: { show: false }, global: { stubs: {
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true, Select: true, Icon: true
      } }
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    await wrapper.get('button[title="common.edit"]').trigger('click')
    await wrapper.get('input[placeholder="admin.users.attributes.fieldDescriptionHint"]').setValue(description)
    await wrapper.get('input[placeholder="admin.users.attributes.placeholderHint"]').setValue(placeholder)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.updateDefinition).toHaveBeenCalledWith(1, expect.objectContaining({ description, placeholder }))
  })
})
