import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, describe, expect, it, vi } from 'vitest'
import OperatorLabView from '../OperatorLabView.vue'

afterEach(() => vi.restoreAllMocks())

describe('OperatorLabView', () => {
  it('switches all four fixture-only prototypes and preserves the selected page in the URL', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/ui-lab', component: OperatorLabView }],
    })
    await router.push('/ui-lab?variant=a&page=overview')
    await router.isReady()

    const wrapper = mount(OperatorLabView, { global: { plugins: [router] } })
    expect(wrapper.get('[data-testid="prototype-a"]').text()).toContain('Codex Team West')

    for (const variant of ['b', 'c', 'd']) {
      await wrapper.get(`.prototype-options button:nth-of-type(${variant.charCodeAt(0) - 96})`).trigger('click')
      await flushPromises()
      expect(router.currentRoute.value.query.variant).toBe(variant)
      expect(wrapper.get(`[data-testid="prototype-${variant}"]`).text()).toContain('Codex Team West')
    }

    const accountsButton = wrapper.findAll('[data-testid="prototype-d"] nav button').find((button) => button.text().includes('Accounts'))
    expect(accountsButton).toBeDefined()
    await accountsButton!.trigger('click')
    await flushPromises()

    expect(router.currentRoute.value.query.page).toBe('accounts')
    expect(wrapper.get('[data-testid="prototype-d"]').text()).toContain('Account capacity')

    const geminiFilter = wrapper.findAll('.d-account-filter button').find((button) => button.text().includes('Gemini'))
    expect(geminiFilter).toBeDefined()
    await geminiFilter!.trigger('click')
    expect(wrapper.get('.d-account-detail').text()).toContain('Gemini Recovery')
    expect(wrapper.get('.d-account-detail').text()).not.toContain('Codex Team West')

    await wrapper.get('.prototype-options button:nth-of-type(2)').trigger('click')
    await flushPromises()
    const accountRows = wrapper.findAll('.b-account-table .b-table-row')
    expect(accountRows.map((row) => row.find('.b-account').text())).toEqual([
      'Antigravity Burstantigravity-burst@example.test',
      'Gemini Recoverygemini-recovery@example.test',
      'Codex Batch Eastcodex-east@example.test',
      'Claude Primaryclaude-primary@example.test',
      'Gemini Pro Northgemini-north@example.test',
      'Codex Team Westcodex-west@example.test',
      'Antigravity Proantigravity@example.test',
      'Claude Overflowclaude-overflow@example.test',
    ])
    expect(fetchSpy).not.toHaveBeenCalled()
  })
})
