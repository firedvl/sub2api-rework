import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import ProxiesView from '../ProxiesView.vue'

const { listProxies, getAllWithCount } = vi.hoisted(() => ({
  listProxies: vi.fn(),
  getAllWithCount: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: { proxies: { list: listProxies, getAllWithCount } }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

const mountView = () => shallowMount(ProxiesView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      ToolbarPopover: { template: '<div><slot :close="() => {}" /></div>' },
      DataTable: {
        props: ['data'],
        template: '<div data-test="rows">{{ data.map(row => row.name).join(",") }}</div>'
      },
      Pagination: {
        props: ['page'],
        emits: ['update:page'],
        template: '<button data-test="page" @click="$emit(\'update:page\', 2)">{{ page }}</button>'
      },
      Select: {
        props: ['modelValue', 'options', 'placeholder'],
        emits: ['update:modelValue', 'change'],
        template: `<select :data-filter="placeholder" :value="modelValue"
          @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value)">
          <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
        </select>`
      }
    }
  }
})

let wrapper: ReturnType<typeof mountView>

beforeEach(() => {
  vi.clearAllMocks()
  getAllWithCount.mockResolvedValue([])
  listProxies.mockResolvedValue({ items: [], total: 100, pages: 5 })
})

afterEach(() => wrapper?.unmount())

describe('proxy list filter pagination', () => {
  const listCalls = () => listProxies.mock.calls.filter((call) => call[3]?.signal)
  const countCalls = () => listProxies.mock.calls.filter((call) => !call[3])

  const goToPageTwo = async () => {
    await wrapper.get('[data-test="page"]').trigger('click')
    await flushPromises()
    expect(listProxies.mock.lastCall?.[0]).toBe(2)
  }

  it.each([
    ['socks5', ''],
    ['', 'http']
  ])('starts at page one when changing protocol to "%s"', async (value, initial) => {
    wrapper = mountView()
    await flushPromises()
    const filter = wrapper.get('select[data-filter="admin.proxies.allProtocols"]')
    if (initial) {
      await filter.setValue(initial)
      await flushPromises()
    }
    await goToPageTwo()
    const [, pageSize, previousFilters] = listProxies.mock.lastCall!
    listProxies.mockImplementation(async (page) => ({
      items: page === 1 ? [{ id: 1, name: 'matching-proxy' }] : [],
      total: 1,
      pages: 1
    }))
    const countsBefore = countCalls().length

    await filter.setValue(value)
    await flushPromises()

    expect(listProxies).toHaveBeenLastCalledWith(
      1, pageSize, { ...previousFilters, protocol: value || undefined },
      { signal: expect.any(AbortSignal) }
    )
    // Status counts follow the protocol scope.
    expect(countCalls().slice(countsBefore).map((call) => call[2])).toEqual(
      ['active', 'inactive', 'expired'].map((status) => ({
        protocol: value || undefined, status, search: undefined
      }))
    )
    expect(wrapper.get('[data-test="page"]').text()).toBe('1')
    expect(wrapper.get('[data-test="rows"]').text()).toBe('matching-proxy')
  })

  it.each([
    ['expired', ''],
    ['', 'active']
  ])('starts at page one when switching the status segment to "%s"', async (value, initial) => {
    wrapper = mountView()
    await flushPromises()
    if (initial) {
      await wrapper.get(`[data-test="proxy-status-${initial}"]`).trigger('click')
      await flushPromises()
    }
    await goToPageTwo()
    const [, pageSize, previousFilters] = listProxies.mock.lastCall!
    const countsBefore = countCalls().length

    await wrapper.get(`[data-test="proxy-status-${value || 'all'}"]`).trigger('click')
    await flushPromises()

    expect(listProxies).toHaveBeenLastCalledWith(
      1, pageSize, { ...previousFilters, status: value || undefined },
      { signal: expect.any(AbortSignal) }
    )
    expect(countCalls().length).toBe(countsBefore)
    expect(wrapper.get(`[data-test="proxy-status-${value || 'all'}"]`).attributes('aria-pressed')).toBe('true')
    expect(wrapper.get('[data-test="page"]').text()).toBe('1')
  })

  it('shows per-status counts on the segment', async () => {
    listProxies.mockImplementation(async (_page, _size, query, options) => {
      if (options?.signal) return { items: [], total: 9, pages: 1 }
      return { items: [], total: { active: 5, inactive: 3, expired: 1 }[query.status as string], pages: 1 }
    })
    wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-test="proxy-status-all"]').text()).toContain('9')
    expect(wrapper.get('[data-test="proxy-status-active"]').text()).toContain('5')
    expect(wrapper.get('[data-test="proxy-status-inactive"]').text()).toContain('3')
    expect(wrapper.get('[data-test="proxy-status-expired"]').text()).toContain('1')
  })

  it('keeps the current page when refreshing the same filter', async () => {
    wrapper = mountView()
    await flushPromises()
    await goToPageTwo()

    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()

    expect(listProxies.mock.lastCall?.[0]).toBe(2)
    expect(listCalls().at(-1)?.[0]).toBe(2)
    expect(wrapper.get('[data-test="page"]').text()).toBe('2')
  })
})
