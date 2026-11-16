import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

const mountBar = (props: Partial<InstanceType<typeof AccountBulkActionsBar>['$props']> = {}) => mount(AccountBulkActionsBar, {
  props: {
    selectedIds: [1],
    totalResults: 45,
    selectingAll: false,
    allResultsSelected: false,
    ...props
  },
  global: { stubs: { teleport: true } }
})

const findButton = (wrapper: ReturnType<typeof mountBar>, text: string) =>
  wrapper.findAll('button').find(item => item.text().includes(text))

describe('AccountBulkActionsBar', () => {
  it('stays hidden until a row is selected', () => {
    const wrapper = mountBar({ selectedIds: [] })
    expect(wrapper.find('[data-testid="account-bulk-bar"]').exists()).toBe(false)
  })

  it('offers selecting all results once a row is selected', async () => {
    const wrapper = mountBar()

    const button = findButton(wrapper, 'admin.accounts.bulkActions.selectAllResults')
    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('select-all-results')).toHaveLength(1)
  })

  it('hides the page link when the view has no pages', () => {
    const wrapper = mountBar({ pageSelectable: false })
    expect(findButton(wrapper, 'admin.accounts.bulkActions.selectCurrentPage')).toBeUndefined()
  })

  it('preserves the upstream billing probe action from v0.1.166', async () => {
    const wrapper = mountBar()

    await wrapper.get('button[aria-label="common.more"]').trigger('click')
    const button = findButton(wrapper, 'admin.accounts.bulkActions.probeUpstreamBilling')

    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('probe-upstream-billing')).toHaveLength(1)
    expect(wrapper.get('button[aria-label="common.more"]').attributes('aria-expanded')).toBe('false')
  })

  it.each([
    ['admin.accounts.bulkActions.refreshToken', 'refresh-token'],
    ['admin.accounts.bulkActions.resetStatus', 'reset-status'],
    ['admin.accounts.bulkActions.delete', 'delete']
  ])('emits %s from the More menu', async (label, event) => {
    const wrapper = mountBar()
    await wrapper.get('button[aria-label="common.more"]').trigger('click')
    await findButton(wrapper, label)!.trigger('click')
    expect(wrapper.emitted(event)).toHaveLength(1)
  })

  it('emits pause and resume with the target state', async () => {
    const wrapper = mountBar()
    await findButton(wrapper, 'admin.accountsUi.resume')!.trigger('click')
    await findButton(wrapper, 'admin.accountsUi.pause')!.trigger('click')
    expect(wrapper.emitted('toggle-schedulable')).toEqual([[true], [false]])
  })
})
