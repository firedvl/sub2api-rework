import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import AccountActionMenu from '../AccountActionMenu.vue'
import type { Account } from '@/types'

enableAutoUnmount(afterEach)

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'test-account',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 3,
    priority: 50,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

const position = { top: 100, left: 100 }

describe('account action menu viewport bounds', () => {
  afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

  it('includes borders and reclamps when menu content grows without a prop change', async () => {
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(400)
    const menuHeight = vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(100)
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(102)
    vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(100)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 208, 102))
    let notifyResize!: () => void
    const disconnect = vi.fn()
    vi.stubGlobal('ResizeObserver', vi.fn().mockImplementation((callback: () => void) => {
      notifyResize = callback
      return { observe: vi.fn(), disconnect }
    }))
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account: makeAccount({}), position: { top: 390, left: 100 } }, attachTo: document.body
    })
    await flushPromises()
    const menu = document.querySelector<HTMLElement>('.action-menu-content')!
    expect(menu.style.top).toBe('290px')
    menuHeight.mockReturnValue(200)
    notifyResize()
    await flushPromises()
    expect(menu.style.top).toBe('190px')
    wrapper.unmount()
    expect(disconnect).toHaveBeenCalled()
  })

  it.each([390, 1280])('clamps long menus at every viewport corner and follows resize at %ipx', async width => {
    vi.spyOn(window, 'innerWidth', 'get').mockReturnValue(width)
    const viewportHeight = vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(400)
    const menuHeight = vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(600)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 208, 600))
    const onClose = vi.fn()
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account: makeAccount({}), position: { top: 390, left: width - 1 }, onClose },
      attachTo: document.body
    })
    await flushPromises()
    const menu = document.querySelector<HTMLElement>('.action-menu-content')!
    expect(menu.style.top).toBe('8px')
    expect(menu.style.left).toBe(`${width - 216}px`)
    expect(menu.style.maxHeight).toBe('384px')
    await wrapper.setProps({ position: { top: -50, left: -50 } })
    await flushPromises()
    expect(menu.style.top).toBe('8px')
    expect(menu.style.left).toBe('8px')
    viewportHeight.mockReturnValue(250)
    menuHeight.mockReturnValue(100)
    await wrapper.setProps({ position: { top: 240, left: width - 1 }, account: makeAccount({ type: 'apikey' }) })
    window.dispatchEvent(new Event('resize'))
    await flushPromises()
    expect(menu.style.top).toBe('142px')
    expect(menu.style.maxHeight).toBe('234px')
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    wrapper.unmount()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('keeps the menu above the persistent operator status bar', async () => {
    vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(900)
    vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(600)
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
      return this.classList.contains('operator-status-bar') ? new DOMRect(0, 300, 1280, 24) : new DOMRect(0, 0, 208, 600)
    })
    const statusBar = document.createElement('div')
    statusBar.className = 'operator-status-bar'
    document.body.appendChild(statusBar)
    const wrapper = mount(AccountActionMenu, { props: { show: true, account: makeAccount({}), position }, attachTo: document.body })
    await flushPromises()
    const menu = document.querySelector<HTMLElement>('.action-menu-content')!
    expect(menu.style.top).toBe('8px')
    expect(menu.style.maxHeight).toBe('284px')
    wrapper.unmount()
    statusBar.remove()
  })
})

// AccountActionMenu uses <Teleport to="body">; content is rendered in document.body, not in wrapper.
const getBodyText = () => document.body.textContent ?? ''
const getBodyButtons = () => Array.from(document.body.querySelectorAll('button'))

describe('AccountActionMenu — spark shadow 按钮可见性', () => {
  it('普通账号显示「复制账号」按钮', () => {
    const account = makeAccount({ platform: 'anthropic', type: 'apikey', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).toContain('admin.accounts.duplicateAccount')
    wrapper.unmount()
  })

  it('影子账号隐藏「复制账号」按钮', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: 42 })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.duplicateAccount')
    wrapper.unmount()
  })

  it.each(['oauth', 'setup-token'] as const)('%s 账号隐藏「复制账号」按钮，避免共享可轮换令牌', (type) => {
    const account = makeAccount({ platform: 'openai', type, parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.duplicateAccount')
    wrapper.unmount()
  })

  it('点击「复制账号」触发 duplicate 事件并携带 account', async () => {
    const account = makeAccount({ platform: 'anthropic', type: 'apikey', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })

    const duplicateBtn = getBodyButtons().find(b => b.textContent?.includes('admin.accounts.duplicateAccount'))
    expect(duplicateBtn).toBeDefined()

    duplicateBtn!.click()
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('duplicate')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toMatchObject({ id: account.id, name: account.name })
    wrapper.unmount()
  })

  it('OpenAI OAuth 母账号（无 parent_account_id）显示「创建 spark 影子」按钮', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).toContain('admin.accounts.createSparkShadow')
    wrapper.unmount()
  })

  it('影子账号（parent_account_id 非 null）隐藏「创建 spark 影子」按钮', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: 42 })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.createSparkShadow')
    wrapper.unmount()
  })

  it('非 OpenAI 账号隐藏「创建 spark 影子」按钮', () => {
    const account = makeAccount({ platform: 'antigravity', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    expect(getBodyText()).not.toContain('admin.accounts.createSparkShadow')
    wrapper.unmount()
  })

  it('影子账号隐藏凭据/隐私类操作(重授权/刷新token/隐私)— 外审 G4', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: 42 })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    const body = getBodyText()
    expect(body).not.toContain('admin.accounts.reAuthorize')
    expect(body).not.toContain('admin.accounts.refreshToken')
    expect(body).not.toContain('admin.accounts.setPrivacy')
    wrapper.unmount()
  })

  it('普通 OpenAI OAuth 母账号仍显示凭据/隐私类操作', () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })
    const body = getBodyText()
    expect(body).toContain('admin.accounts.reAuthorize')
    expect(body).toContain('admin.accounts.setPrivacy')
    wrapper.unmount()
  })

  it('点击按钮触发 create-spark-shadow 事件并携带 account', async () => {
    const account = makeAccount({ platform: 'openai', type: 'oauth', parent_account_id: null })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })

    // Content is teleported to body — find button by text there
    const sparkBtn = getBodyButtons().find(b => b.textContent?.includes('admin.accounts.createSparkShadow'))
    expect(sparkBtn).toBeDefined()

    sparkBtn!.click()
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('create-spark-shadow')
    expect(emitted).toBeTruthy()
    expect(emitted![0][0]).toMatchObject({ id: account.id, platform: 'openai' })

    wrapper.unmount()
  })

  it('删除账号是破坏性菜单项，并触发 delete 与 close 事件', async () => {
    const account = makeAccount({ name: 'delete-target' })
    const wrapper = mount(AccountActionMenu, {
      props: { show: true, account, position },
      attachTo: document.body,
    })

    const deleteBtn = getBodyButtons().find(b => b.textContent?.includes('admin.accounts.deleteAccount'))
    expect(deleteBtn).toBeDefined()
    expect(deleteBtn?.classList.contains('operator-menu-item-destructive')).toBe(true)

    deleteBtn!.click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('delete')?.[0]?.[0]).toMatchObject({ id: account.id, name: account.name })
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })
})
