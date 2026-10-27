import { expect, test, type Locator, type Page, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import {
  OPERATOR_FIXTURE_NOW,
  operatorFixtureAccounts,
  operatorLargeCapacityFixture,
} from './fixtures/operatorData.ts'

const primaryLinks = [
  { href: '/admin/dashboard', target: '/admin/dashboard' },
  { href: '/admin/stats', target: '/admin/stats' },
  { href: '/admin/accounts', target: '/admin/accounts' },
  { href: '/admin/groups', target: '/admin/groups' },
  { href: '/admin/usage', target: '/admin/usage' },
  { href: '/admin/settings', target: '/admin/settings' },
]

const primaryNavLink = (page: Page, href: string) =>
  page.locator(`nav[aria-label="Main navigation"] a[href="${href}"]`)

const fidelityAdminRoutes = ['/admin/dashboard', '/admin/stats', '/admin/accounts', '/admin/groups', '/admin/usage', '/admin/settings']
const fidelityViewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
]

const fulfillApiData = (route: Route, data: unknown, status = 200, headers?: Record<string, string>) => (
  route.fulfill({
    status,
    headers,
    contentType: 'application/json',
    body: JSON.stringify({ code: status >= 400 ? status : 0, message: status >= 400 ? 'fixture failure' : 'ok', data }),
  })
)

async function installManyAccountMock(page: Page) {
  const fixture = operatorLargeCapacityFixture()
  await installAccountListMock(page, fixture.accounts, fixture.usage)
  return fixture
}

async function installAccountListMock(
  page: Page,
  accounts: typeof operatorFixtureAccounts,
  usage?: Record<string, unknown>,
) {
  await page.route('**/api/v1/admin/accounts**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const pathname = url.pathname
    if (request.method() === 'GET' && pathname === '/api/v1/admin/accounts') {
      const pageNumber = Math.max(1, Number(url.searchParams.get('page') || 1))
      const pageSize = Math.max(1, Number(url.searchParams.get('page_size') || accounts.length))
      const start = (pageNumber - 1) * pageSize
      await fulfillApiData(route, {
        items: accounts.slice(start, start + pageSize),
        total: accounts.length,
        page: pageNumber,
        page_size: pageSize,
        pages: Math.ceil(accounts.length / pageSize),
      })
      return
    }
    if (pathname === '/api/v1/admin/accounts/usage/batch' && usage) {
      await fulfillApiData(route, { usage, errors: {} })
      return
    }
    await route.fallback()
  })
}

interface MutableAccountRefreshState {
  account: (typeof operatorFixtureAccounts)[number]
  utilization: number
  etag: string
  tableStatuses: number[]
  batchForces: boolean[]
}

async function installMutableAccountRefreshMock(page: Page): Promise<MutableAccountRefreshState> {
  const state: MutableAccountRefreshState = {
    account: {
      ...operatorFixtureAccounts[0],
      name: 'Refresh fixture initial',
      extra: { ...operatorFixtureAccounts[0].extra },
      group_ids: [...operatorFixtureAccounts[0].group_ids],
      groups: operatorFixtureAccounts[0].groups.map((group) => ({ ...group })),
    },
    utilization: 20,
    etag: '"refresh-a"',
    tableStatuses: [],
    batchForces: [],
  }

  await page.route('**/api/v1/admin/accounts**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const pathname = url.pathname
    if (request.method() === 'GET' && pathname === '/api/v1/admin/accounts') {
      const isTableRequest = url.searchParams.has('include_scheduler_score')
      if (isTableRequest && request.headers()['if-none-match'] === state.etag) {
        state.tableStatuses.push(304)
        await route.fulfill({ status: 304, headers: { etag: state.etag } })
        return
      }

      if (isTableRequest) state.tableStatuses.push(200)
      const pageNumber = Math.max(1, Number(url.searchParams.get('page') || 1))
      const pageSize = Math.max(1, Number(url.searchParams.get('page_size') || 20))
      await fulfillApiData(route, {
        items: pageNumber === 1 ? [{ ...state.account }] : [],
        total: 1,
        page: pageNumber,
        page_size: pageSize,
        pages: 1,
      }, 200, { etag: state.etag })
      return
    }
    if (request.method() === 'POST' && pathname === '/api/v1/admin/accounts/usage/batch') {
      const body = request.postDataJSON() as { force?: boolean }
      state.batchForces.push(body.force === true)
      await fulfillApiData(route, {
        usage: {
          [String(state.account.id)]: {
            source: 'passive',
            updated_at: '2026-09-02T00:00:00Z',
            five_hour: {
              utilization: state.utilization,
              resets_at: '2026-09-02T05:00:00Z',
              remaining_seconds: 3600,
            },
            seven_day: null,
            seven_day_sonnet: null,
          },
        },
        errors: {},
      })
      return
    }
    await route.fallback()
  })

  return state
}

interface WritableSettingsState {
  failMain: boolean
  failWebSearch: boolean
  delay: number
  mainSaves: number
  webSearchSaves: number
}

async function installWritableSettingsMock(page: Page): Promise<WritableSettingsState> {
  const state: WritableSettingsState = {
    failMain: false,
    failWebSearch: false,
    delay: 0,
    mainSaves: 0,
    webSearchSaves: 0,
  }

  await page.route('**/api/v1/admin/settings**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    if (request.method() !== 'PUT') {
      await route.fallback()
      return
    }
    if (state.delay) await new Promise((resolve) => setTimeout(resolve, state.delay))

    if (pathname === '/api/v1/admin/settings') {
      state.mainSaves += 1
      await fulfillApiData(route, state.failMain ? {} : request.postDataJSON(), state.failMain ? 500 : 200)
      return
    }
    if (pathname === '/api/v1/admin/settings/web-search-emulation') {
      state.webSearchSaves += 1
      await fulfillApiData(route, state.failWebSearch ? {} : request.postDataJSON(), state.failWebSearch ? 500 : 200)
      return
    }
    await route.fallback()
  })

  return state
}

async function expectStatsBandGeometry(chart: Locator, label: string) {
  const metrics = await chart.getByRole('group').evaluate((element) => {
    const bars = [...element.querySelectorAll<HTMLElement>('.stats-trend-bar > span')].map((bar) => {
      const bounds = bar.getBoundingClientRect()
      return { left: bounds.left, right: bounds.right, width: bounds.width }
    })
    const plot = element.querySelector<HTMLElement>('.stats-bar-grid-line')!.getBoundingClientRect()
    const labelRight = Math.max(...[...element.querySelectorAll<HTMLElement>('.stats-bar-y-label')]
      .map((item) => item.getBoundingClientRect().right))
    return {
      bars,
      plot: { left: plot.left, right: plot.right },
      labelRight,
    }
  })

  expect(metrics.bars.length, `${label} bars`).toBeGreaterThan(0)
  for (const [index, bar] of metrics.bars.entries()) {
    expect(bar.left, `${label} bar ${index} left bound`).toBeGreaterThanOrEqual(metrics.plot.left - 1)
    expect(bar.right, `${label} bar ${index} right bound`).toBeLessThanOrEqual(metrics.plot.right + 1)
    if (index > 0) {
      expect(bar.left - metrics.bars[index - 1].right, `${label} gap ${index - 1}-${index}`).toBeGreaterThanOrEqual(0.5)
    }
  }
  expect(metrics.bars[0].left, `${label} Y-axis clearance`).toBeGreaterThanOrEqual(metrics.labelRight + 2)
}

async function operatorTokens(page: Page) {
  return page.evaluate(() => {
    const style = getComputedStyle(document.documentElement)
    const probe = document.createElement('span')
    probe.hidden = true
    document.body.append(probe)
    // Resolve every token through a probe so var() chains and formats compare as browser colors.
    const color = (property: string) => {
      probe.style.color = ''
      probe.style.color = style.getPropertyValue(property).trim()
      return getComputedStyle(probe).color
    }
    const tokens = {
      background: color('--operator-background'),
      card: color('--operator-card'),
      border: color('--operator-border'),
      borderStrong: color('--operator-border-strong'),
      borderSubtle: color('--operator-border-subtle'),
      muted: color('--operator-muted'),
      mutedForeground: color('--operator-muted-foreground'),
      foreground: color('--operator-foreground'),
      focus: color('--operator-focus'),
      accent: color('--operator-accent'),
      accentText: color('--operator-accent-text'),
      accentSoft: color('--operator-accent-soft'),
      infoSoft: color('--operator-info-soft'),
      track: color('--operator-track'),
      switchOn: color('--operator-switch-on'),
      popupSurface: color('--operator-popup-surface'),
      popupSearch: color('--operator-popup-search'),
      popupHover: color('--operator-popup-hover'),
      popupSelected: color('--operator-popup-selected'),
      popupBorder: color('--operator-popup-border'),
      popupForeground: color('--operator-popup-foreground'),
      popupMuted: color('--operator-popup-muted'),
    }
    probe.remove()
    return tokens
  })
}

async function expectPopupMenu(
  page: Page,
  menu: Locator,
  emphasizedItem: Locator,
  state: 'hover' | 'selected' | 'checked' = 'hover',
) {
  await expect(menu).toBeVisible()
  await expect(emphasizedItem).toBeVisible()
  if (state === 'hover') await emphasizedItem.hover()
  await page.waitForTimeout(200)
  const tokens = await operatorTokens(page)

  const menuColors = await menu.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, border: style.borderColor }
  })
  const itemColors = await emphasizedItem.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, color: style.color }
  })

  expect(menuColors).toEqual({ background: tokens.popupSurface, border: tokens.popupBorder })
  // Multi-select toggles (column pickers) mark checked rows with a checkmark, not a fill.
  const background = {
    hover: tokens.popupHover,
    selected: tokens.popupSelected,
    checked: 'rgba(0, 0, 0, 0)',
  }[state]
  expect(itemColors).toEqual({ background, color: tokens.popupForeground })
  if (state === 'checked') await expect(emphasizedItem.locator('svg')).toBeVisible()
}

async function openPopupSelect(page: Page, trigger: Locator) {
  await trigger.scrollIntoViewIfNeeded()
  await trigger.click()

  const popup = page.locator('body > .select-dropdown-portal.operator-select-menu.operator-menu')
  await expect(popup).toBeVisible()
  const tokens = await operatorTokens(page)
  const layers = await popup.evaluate((element) => {
    const background = (selector: string) => {
      const layer = element.querySelector(selector)
      return layer ? getComputedStyle(layer).backgroundColor : null
    }
    return {
      root: getComputedStyle(element).backgroundColor,
      border: getComputedStyle(element).borderColor,
      search: background('.select-search'),
      list: background('.select-options'),
      selected: background('.select-option-selected'),
    }
  })

  expect(layers.root).toBe(tokens.popupSurface)
  expect(layers.border).toBe(tokens.popupBorder)
  if (layers.search !== null) expect(layers.search).toBe(tokens.popupSearch)
  expect(layers.list).toBe(tokens.popupSurface)
  expect(layers.selected).toBe(tokens.popupSelected)

  const normalOption = popup.locator('.select-option:not(.select-option-selected)').first()
  const selectedOption = popup.locator('.select-option-selected').first()
  await normalOption.hover()
  await expect.poll(() => normalOption.evaluate((element) => getComputedStyle(element).backgroundColor))
    .toBe(tokens.popupHover)
  expect(await selectedOption.evaluate((element) => getComputedStyle(element).backgroundColor))
    .toBe(tokens.popupSelected)

  await trigger.click()
  await expect(popup).toHaveCount(0)
}

async function expectTokenDialog(page: Page, dialog: Locator) {
  const tokens = await operatorTokens(page)
  const shell = dialog.locator('.operator-dialog')
  const colors = await shell.evaluate((element) => {
    const header = element.querySelector('.operator-dialog-header') as HTMLElement
    const footer = element.querySelector('.operator-dialog-footer') as HTMLElement | null
    return {
      shell: getComputedStyle(element).backgroundColor,
      border: getComputedStyle(element).borderColor,
      headerBorder: getComputedStyle(header).borderBottomColor,
      footerBorder: footer ? getComputedStyle(footer).borderTopColor : null,
    }
  })

  expect(colors).toMatchObject({
    shell: tokens.popupSurface,
    border: tokens.popupBorder,
    headerBorder: tokens.border,
  })
  if (colors.footerBorder !== null) expect(colors.footerBorder).toBe(tokens.border)
}

test.describe('operator console navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.clock.setFixedTime(OPERATOR_FIXTURE_NOW)
    await seedSession(page)
    await installOperatorApiMock(page)
  })

  test('uses Gateway identity on login and operator browser surfaces', async ({ page, browser }) => {
    const anonymous = await browser.newContext({ locale: 'en-US' })
    const loginPage = await anonymous.newPage()
    await installOperatorApiMock(loginPage)
    await loginPage.goto('/login')

    await expect(loginPage.getByRole('heading', { level: 1, name: 'Gateway' })).toBeVisible({ timeout: 15_000 })
    await expect(loginPage.locator('.auth-codex-brand p')).toHaveText('AI Gateway')
    await expect(loginPage.locator('.auth-codex-copyright')).toHaveText('Powered by Sub2API Rework')
    await expect(loginPage.locator('.auth-codex-logo img')).toHaveAttribute('src', '/logo.svg')
    await expect(loginPage.locator('.auth-codex-logo img')).toHaveAttribute('aria-hidden', 'true')
    await expect(loginPage.locator('html')).toHaveAttribute('lang', 'en')
    await expect(loginPage).toHaveTitle('Login · Gateway')
    await anonymous.close()

    await page.goto('/admin/dashboard')
    await expect(page.locator('.sidebar-brand-title')).toContainText('Gateway')
    await expect(page.locator('.sidebar-logo img')).toHaveAttribute('src', '/logo.svg')
    await expect(page).toHaveTitle('Overview · Gateway')
  })

  test('logs a fresh admin into the dashboard without the legacy tour', async ({ browser }) => {
    const freshSession = await browser.newContext({ locale: 'en-US' })
    const loginPage = await freshSession.newPage()
    await installOperatorApiMock(loginPage)
    await loginPage.goto('/login')

    expect(await loginPage.evaluate(() => localStorage.getItem('auth_token'))).toBeNull()
    expect(await loginPage.evaluate(() => Object.keys(localStorage).some((key) => /guide|onboarding|tour/i.test(key)))).toBe(false)
    await loginPage.getByLabel('Email').fill('admin@example.test')
    await loginPage.getByLabel('Password', { exact: true }).fill('fixture-password')
    await loginPage.getByRole('button', { name: 'Sign In' }).click()

    await expect(loginPage).toHaveURL(/\/admin\/dashboard$/)
    await expect(loginPage.locator('.driver-overlay, .driver-popover, .driver-active-element')).toHaveCount(0)
    await expect(loginPage.getByText(/Restart Onboarding Tour|Start setup|onboarding guide/i)).toHaveCount(0)
    expect(await loginPage.evaluate(() => Object.keys(localStorage).some((key) => /guide|onboarding|tour/i.test(key)))).toBe(false)
    await expect(loginPage.getByRole('button', { name: 'Ask Gateway' })).toBeVisible()
    await freshSession.close()
  })

  test('reaches all six areas and keeps history and deep-link state', async ({ page }) => {
    test.setTimeout(60_000)
    await page.goto('/admin/dashboard')
    await expect(primaryNavLink(page, '/admin/dashboard')).toHaveAttribute('aria-current', 'page')

    for (const link of primaryLinks.slice(1)) {
      const navLink = primaryNavLink(page, link.href)
      await expect(navLink).toBeVisible()
      await navLink.click()
      await expect(page).toHaveURL(new RegExp(`${link.target.replaceAll('/', '\\/')}(?:\\?.*)?$`), { timeout: 15_000 })
      await expect(navLink).toHaveAttribute('aria-current', 'page')
      await expect(page.getByRole('progressbar', { name: 'Loading' })).toBeHidden()
    }

    await page.goto('/admin/channels/pricing')
    await expect(primaryNavLink(page, '/admin/groups')).toHaveAttribute('aria-current', 'page')
    await expect(page.locator('.operator-context-nav a[href="/admin/channels/pricing"]')).toHaveClass(/operator-context-link-active/)

    await page.goto('/admin/audit-logs')
    await expect(primaryNavLink(page, '/admin/usage')).toHaveAttribute('aria-current', 'page')
    await expect(page.locator('.operator-context-nav a[href="/admin/audit-logs"]')).toHaveClass(/operator-context-link-active/)

    await page.goto('/admin/accounts')
    await page.goto('/admin/groups')
    await page.goBack()
    await expect(page).toHaveURL(/\/admin\/accounts$/)
    await expect(primaryNavLink(page, '/admin/accounts')).toHaveAttribute('aria-current', 'page')
    await page.goForward()
    await expect(page).toHaveURL(/\/admin\/groups$/)
  })

  test('uses the drawer at a narrow viewport', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/admin/dashboard')

    const menuButton = page.getByRole('button', { name: /toggle menu/i })
    await expect(menuButton).toBeVisible()
    await menuButton.click()

    const sidebar = page.locator('aside.sidebar')
    await expect(sidebar).toBeVisible()
    await sidebar.locator('a[href="/admin/accounts"]').click()
    await expect(page).toHaveURL(/\/admin\/accounts$/)
    await expect(sidebar).toHaveClass(/-translate-x-full/)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
  })

  test('has no legacy startup tour and keeps Ask Gateway usable across operator widths', async ({ page }) => {
    test.setTimeout(60_000)
    const cases = [
      { width: 1440, height: 900, route: '/admin/dashboard' },
      { width: 1024, height: 768, route: '/admin/accounts' },
      { width: 390, height: 844, route: '/admin/usage' },
    ]

    for (const reviewCase of cases) {
      await page.setViewportSize({ width: reviewCase.width, height: reviewCase.height })
      await page.goto(reviewCase.route)
      await expect(page.locator('.driver-overlay, .driver-popover, .driver-active-element')).toHaveCount(0)
      await expect(page.getByText(/Restart Onboarding Tour|Start setup|onboarding guide/i)).toHaveCount(0)

      const trigger = page.getByRole('button', { name: 'Ask Gateway' })
      await expect(trigger).toBeVisible()

      const triggerBox = await trigger.boundingBox()
      expect(triggerBox).not.toBeNull()
      expect(triggerBox!.x + triggerBox!.width).toBeLessThanOrEqual(reviewCase.width)
      await expect(page.locator('.app-header').getByRole('button', { name: 'Ask Gateway' })).toBeVisible()

      await trigger.click()

      const dialog = page.getByRole('dialog', { name: 'Ask Gateway' })
      await expect(dialog).toBeVisible()
      await expect(dialog.getByRole('textbox')).toBeFocused()
      await expect.poll(async () => {
        const box = await dialog.locator('.operator-assistant-panel').boundingBox()
        return box ? Math.round(box.x + box.width) : Number.POSITIVE_INFINITY
      }).toBeLessThanOrEqual(reviewCase.width + 1)
      const panelBox = await dialog.locator('.operator-assistant-panel').boundingBox()
      expect(panelBox).not.toBeNull()
      expect(panelBox!.width).toBeLessThanOrEqual(reviewCase.width)
      expect(await dialog.locator('.operator-assistant-panel').evaluate((element) => element.scrollWidth <= element.clientWidth)).toBe(true)

      await page.keyboard.press('Tab')
      expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)
      await page.keyboard.press('Escape')
      await expect(dialog).toBeHidden()
      await expect(trigger).toBeFocused()
    }

    expect(await page.evaluate(() => Object.keys(localStorage).some((key) => /guide|onboarding|tour/i.test(key)))).toBe(false)
  })

  test('streams, stops, retries, clears, sanitizes, and keeps assistant history ephemeral', async ({ page, context }) => {
    test.setTimeout(60_000)
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/dashboard')
    await page.getByRole('button', { name: 'Ask Gateway' }).click()
    const dialog = page.getByRole('dialog', { name: 'Ask Gateway' })
    const composer = dialog.getByRole('textbox')

    await expect(dialog.getByRole('button', { name: 'What needs attention?' })).toBeVisible()
    await expect(dialog.locator('.select-value')).toHaveText('Auto')
    await dialog.getByRole('button', { name: 'What needs attention?' }).click()
    let answer = dialog.locator('.operator-assistant-message-assistant').last()
    await expect(answer).toContainText('One OpenAI account needs attention')
    await expect(answer.locator('.operator-assistant-metadata')).toContainText(/Auto -> gpt-5\.4 \/ openai \/ [\d.]+s/)

    await answer.getByRole('button', { name: 'Copy' }).click()
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toContain('One OpenAI account')

    await dialog.getByRole('button', { name: 'Model' }).click()
    await page.getByRole('option', { name: /claude-sonnet-4-6/ }).click()
    expect(await page.evaluate(() => localStorage.getItem('operator_assistant_model'))).toBe('12:claude-sonnet-4-6')
    await expect(dialog.locator('.operator-assistant-message-assistant').first().locator('.operator-assistant-metadata'))
      .toContainText(/Auto -> gpt-5\.4 \/ openai \/ [\d.]+s/)
    await composer.fill('What version is running?')
    await dialog.getByRole('button', { name: 'Send message' }).click()
    answer = dialog.locator('.operator-assistant-message-assistant').last()
    await expect(answer).toContainText('0.1.183-rework.12')
    await expect(answer.locator('.operator-assistant-metadata')).toContainText(/claude-sonnet-4-6 \/ anthropic/)

    await composer.fill('Show markdown fixture')
    await dialog.getByRole('button', { name: 'Send message' }).click()
    answer = dialog.locator('.operator-assistant-message-assistant').last()
    await expect(answer.locator('strong')).toHaveText('operator answer')
    await expect(answer.locator('img')).toHaveCount(0)
    await expect(answer.locator('a[href^="javascript:"]')).toHaveCount(0)
    const runbook = answer.getByRole('link', { name: 'Runbook' })
    await expect(runbook).toHaveAttribute('target', '_blank')
    await expect(runbook).toHaveAttribute('rel', 'noopener noreferrer')
    expect(await page.evaluate(() => (window as Window & { fixtureXss?: boolean }).fixtureXss)).toBeUndefined()

    await composer.fill('Retry fixture')
    await dialog.getByRole('button', { name: 'Send message' }).click()
    answer = dialog.locator('.operator-assistant-message-assistant').last()
    await expect(answer).toContainText('No eligible model capacity is currently available.')
    await answer.getByRole('button', { name: 'Retry' }).click()
    await expect(dialog.locator('.operator-assistant-message-assistant').last()).toContainText('One OpenAI account needs attention')

    await composer.fill('Slow fixture')
    await dialog.getByRole('button', { name: 'Send message' }).click()
    await dialog.getByRole('button', { name: 'Stop response' }).click()
    await expect(dialog.locator('.operator-assistant-message-assistant').last()).toContainText('Stopped')

    await dialog.getByRole('button', { name: 'Clear conversation' }).click()
    await expect(dialog.locator('.operator-assistant-message')).toHaveCount(0)
    await expect(dialog.getByRole('button', { name: 'What needs attention?' })).toBeVisible()
    const storage = await page.evaluate(() => Object.fromEntries(Object.entries(localStorage)))
    expect(Object.keys(storage).some((key) => /conversation|messages|prompt/i.test(key))).toBe(false)
    expect(JSON.stringify(storage)).not.toContain('Retry fixture')
    expect(JSON.stringify(storage)).not.toContain('One OpenAI account needs attention')
  })

  test('uses the same shell for operator and personal routes', async ({ page }) => {
    await page.goto('/keys')

    await expect(page.locator('aside.sidebar')).toBeVisible()
    await expect(page.locator('.app-header')).toBeVisible()
    await expect(page.getByRole('heading', { level: 1, name: 'API Keys' })).toBeVisible()
    await expect(primaryNavLink(page, '/keys')).toHaveAttribute('aria-current', 'page')
    await expect(primaryNavLink(page, '/admin/dashboard')).toBeVisible()
  })

  test('keeps teleported popup layers on the active theme tokens', async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem('theme', 'light'))
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/accounts')
    await expect(page.locator('html')).not.toHaveClass(/dark/)

    await page.locator('.operator-accounts-toolbar .select-trigger').first().click()
    const popup = page.locator('body > .select-dropdown-portal.operator-select-menu.operator-menu')
    await expect(popup).toBeVisible()
    const tokens = await operatorTokens(page)
    expect(tokens.popupSurface).toBe('rgb(255, 255, 255)')

    const normalOption = popup.locator('.select-option:not(.select-option-selected)').first()
    const selectedOption = popup.locator('.select-option-selected')
    const initialColors = await popup.evaluate((element) => {
      const color = (selector: string) => getComputedStyle(element.querySelector(selector) as HTMLElement).backgroundColor
      return {
        root: getComputedStyle(element).backgroundColor,
        rootBorder: getComputedStyle(element).borderColor,
        search: color('.select-search'),
        searchBorder: getComputedStyle(element.querySelector('.select-search') as HTMLElement).borderBottomColor,
        list: color('.select-options'),
        selected: color('.select-option-selected'),
      }
    })
    expect(initialColors).toEqual({
      root: tokens.popupSurface,
      rootBorder: tokens.popupBorder,
      search: tokens.popupSearch,
      searchBorder: tokens.popupBorder,
      list: tokens.popupSurface,
      selected: tokens.popupSelected,
    })

    await normalOption.hover()
    await expect.poll(() => normalOption.evaluate((element) => getComputedStyle(element).backgroundColor))
      .toBe(tokens.popupHover)
    expect(await selectedOption.evaluate((element) => getComputedStyle(element).backgroundColor))
      .toBe(tokens.popupSelected)

    await popup.locator('.select-search-input').fill('__no_platform_match__')
    const empty = popup.locator('.select-empty')
    await expect(empty).toBeVisible()
    expect(await empty.evaluate((element) => getComputedStyle(element).color)).toBe(tokens.popupMuted)
  })

  test('keeps operator popup colors stable through same-context route lifecycles', async ({ page }) => {
    test.setTimeout(90_000)
    await page.addInitScript(() => localStorage.setItem('theme', 'light'))
    await page.setViewportSize({ width: 1440, height: 900 })

    const navigate = async (path: string) => {
      await primaryNavLink(page, path).click()
      await expect(page).toHaveURL(new RegExp(`${path.replaceAll('/', '\\/')}$`))
      await expect(page.locator('aside.sidebar')).toBeVisible()
      await expect(page.locator('body > .select-dropdown-portal')).toHaveCount(0)
    }
    const openLocaleMenu = async () => {
      const trigger = page.locator('.operator-locale-trigger')
      await trigger.click()
      const menu = page.locator('.app-header .operator-menu')
      await expectPopupMenu(page, menu, menu.locator('[aria-current="true"]'), 'selected')
      await trigger.click()
      await expect(menu).toHaveCount(0)
    }

    await page.goto('/admin/accounts')
    await expect(page.locator('html')).not.toHaveClass(/dark/)
    await expect(page.locator('aside.sidebar')).toBeVisible()
    await openPopupSelect(page, page.locator('.select-trigger').first())

    await navigate('/admin/usage')
    await openPopupSelect(page, page.locator('.select-trigger').filter({ hasText: 'Hour' }))

    await navigate('/admin/accounts')
    await openPopupSelect(page, page.locator('.select-trigger').first())
    const moreActionsButton = page.getByRole('button', { name: 'More Actions' })
    await moreActionsButton.click()
    const moreActionsMenu = page.locator('.account-tools-menu')
    await expectPopupMenu(page, moreActionsMenu, moreActionsMenu.locator('.operator-menu-item').first())
    await moreActionsButton.click()
    await expect(moreActionsMenu).toHaveCount(0)
    await openPopupSelect(page, page.locator('.select-trigger').nth(2))

    await navigate('/admin/stats')
    await openLocaleMenu()
    await navigate('/admin/groups')
    await openPopupSelect(page, page.locator('.select-trigger').first())
    await navigate('/admin/usage')
    await openPopupSelect(page, page.locator('.select-trigger').filter({ hasText: 'Hour' }))
    await navigate('/admin/settings')
    await page.getByRole('tab', { name: 'Gateway' }).click()
    await openPopupSelect(page, page.locator('.select-trigger:visible').first())
    await navigate('/admin/accounts')
    await openPopupSelect(page, page.locator('.select-trigger').first())

    for (let iteration = 0; iteration < 3; iteration += 1) {
      await navigate('/admin/groups')
      await openPopupSelect(page, page.locator('.select-trigger').first())
      await navigate('/admin/accounts')
      await openPopupSelect(page, page.locator('.select-trigger').first())
    }
  })

  test('switches Accounts views without stacking or refetching', async ({ page }) => {
    await page.goto('/admin/accounts')

    const capacityTab = page.getByRole('tab', { name: 'Capacity' })
    const technicalTab = page.getByRole('tab', { name: 'Technical' })
    const capacityPanel = page.getByRole('tabpanel', { name: 'Capacity' })
    const technicalPanel = page.getByRole('tabpanel', { name: 'Technical' })
    await expect(capacityTab).toHaveAttribute('aria-selected', 'true')
    await expect(capacityPanel).toBeVisible()
    await expect(technicalPanel).toBeHidden()
    const capacityDetails = page.getByTestId('account-technical-details')
    await expect(capacityDetails).toHaveCount(6)
    await expect(capacityDetails.first()).toBeHidden()

    await capacityTab.focus()
    await page.keyboard.press('ArrowRight')
    await expect(technicalTab).toBeFocused()
    await expect(technicalTab).toHaveAttribute('aria-selected', 'true')
    await expect(capacityPanel).toBeHidden()
    await expect(technicalPanel).toBeVisible()

    const technicalStatus = page.locator('.operator-accounts-toolbar .select-trigger').nth(2)
    await technicalStatus.click()
    await page.getByRole('option', { name: 'Active', exact: true }).click()
    await expect(technicalStatus).toContainText('Active')

    await technicalTab.focus()
    await page.keyboard.press('ArrowLeft')
    await expect(capacityTab).toBeFocused()
    await expect(capacityPanel).toBeVisible()
    await expect(technicalPanel).toBeHidden()
    await expect(capacityDetails).toHaveCount(6)
  })

  test('keeps Google One quota help interactive across pointer, keyboard, and mobile input', async ({ page }) => {
    await page.context().route('https://developers.google.com/**', (route) => route.fulfill({
      contentType: 'text/html',
      body: '<title>Quota docs fixture</title>',
    }))
    await page.goto('/admin/accounts')
    await page.getByRole('tab', { name: 'Technical' }).click()

    const trigger = page.getByRole('button', { name: 'Gemini Quota & Limit Policy (Reference)' }).last()
    const dialog = page.getByRole('dialog', { name: 'Gemini Quota & Limit Policy (Reference)' })
    await trigger.hover()
    await expect(dialog).toBeVisible()
    await dialog.hover()
    await expect(dialog).toBeVisible()

    await page.mouse.click(4, 4)
    await expect(dialog).toBeHidden()
    await trigger.focus()
    await expect(dialog).toBeVisible()
    await page.keyboard.press('Tab')
    const docs = dialog.getByRole('link', { name: 'Official Docs' })
    await expect(docs).toBeFocused()

    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    await expect(trigger).toBeFocused()

    await trigger.click()
    const popupPromise = page.waitForEvent('popup')
    await docs.click()
    const popup = await popupPromise
    await expect(popup).toHaveURL('https://developers.google.com/gemini-code-assist/resources/quotas')
    await popup.close()

    await page.setViewportSize({ width: 390, height: 844 })
    await page.mouse.click(4, 4)
    await trigger.click()
    await expect(dialog).toBeVisible()
  })

  test('updates Accounts after manual and auto refresh without reloading the route', async ({ page }) => {
    test.setTimeout(60_000)
    const state = await installMutableAccountRefreshMock(page)
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/accounts')

    const capacityRows = page.getByTestId('account-capacity-row')
    const capacityRow = capacityRows.first()
    await expect(capacityRows).toHaveCount(1)
    await expect(capacityRow).toContainText('Refresh fixture initial')
    await expect(capacityRow).toContainText('80%')
    await page.evaluate(() => { (window as any).__accountsRefreshRouteMarker = 'mounted' })

    state.account = {
      ...state.account,
      name: 'Refresh fixture manual',
      current_concurrency: 2,
      updated_at: '2026-09-02T00:01:00Z',
    }
    state.utilization = 46
    const createButton = page.getByRole('button', { name: 'Create Account' })
    const manualRefresh = createButton.locator('..').locator('button').first()
    await manualRefresh.click()
    await expect(capacityRow).toContainText('Refresh fixture manual')
    await expect(capacityRow).toContainText('54%')
    expect(state.batchForces.at(-1)).toBe(true)
    expect(await page.evaluate(() => (window as any).__accountsRefreshRouteMarker)).toBe('mounted')

    state.tableStatuses.length = 0
    state.batchForces.length = 0
    const autoRefresh = page.getByTitle('Auto Refresh')
    await autoRefresh.click()
    await page.getByRole('button', { name: '5 seconds', exact: true }).click()
    await page.getByRole('button', { name: 'Enable auto refresh' }).click()
    await autoRefresh.click()
    await expect(autoRefresh).toContainText('Auto refresh: 5s')

    await page.clock.runFor(5_000)
    await expect.poll(() => state.tableStatuses).toEqual([200])
    await expect(autoRefresh).toContainText('Auto refresh: 5s')

    state.utilization = 53
    await page.clock.runFor(5_000)
    await expect.poll(() => state.tableStatuses).toEqual([200, 304])
    await expect(capacityRow).toContainText('47%')
    expect(state.batchForces.at(-1)).toBe(false)

    state.etag = '"refresh-b"'
    state.account = {
      ...state.account,
      name: 'Refresh fixture auto',
      updated_at: '2026-09-02T00:02:00Z',
    }
    state.utilization = 35
    await page.clock.runFor(5_000)
    await expect.poll(() => state.tableStatuses).toEqual([200, 304, 200])
    await expect(capacityRows).toHaveCount(1)
    await expect(capacityRow).toContainText('Refresh fixture auto')
    await expect(capacityRow).toContainText('65%')
    expect(await page.evaluate(() => (window as any).__accountsRefreshRouteMarker)).toBe('mounted')
  })

  test('keeps technical sticky columns aligned at every review width', async ({ page }) => {
    test.setTimeout(60_000)

    for (const viewport of fidelityViewports) {
      await page.setViewportSize(viewport)
      await page.goto('/admin/accounts')
      await page.getByRole('tab', { name: 'Technical' }).click()

      const table = page.locator('.operator-account-table .table-wrapper')
      await expect(table).toBeVisible()
      const leftmost = await table.evaluate(async (element) => {
        element.scrollLeft = 0
        element.dispatchEvent(new Event('scroll'))
        await new Promise((resolve) => requestAnimationFrame(resolve))
        const wrapper = element.getBoundingClientRect()
        const headers = element.querySelectorAll('thead th')
        const cells = element.querySelectorAll('tbody tr[data-index]:first-of-type td')
        const checkbox = headers[0].getBoundingClientRect()
        const name = headers[1].getBoundingClientRect()
        const id = headers[2].getBoundingClientRect()
        const rowCheckbox = cells[0].getBoundingClientRect()
        const rowName = cells[1].getBoundingClientRect()
        return {
          scrollLeft: element.scrollLeft,
          wrapper: { left: wrapper.left, right: wrapper.right },
          checkbox: { left: checkbox.left, right: checkbox.right },
          name: { left: name.left, right: name.right, text: headers[1].textContent?.trim() },
          id: { left: id.left, text: headers[2].textContent?.trim() },
          rowCheckbox: { left: rowCheckbox.left, right: rowCheckbox.right },
          rowName: { left: rowName.left, text: cells[1].textContent?.trim() },
        }
      })

      expect(leftmost.scrollLeft, `${viewport.width}px scroll origin`).toBe(0)
      expect(leftmost.checkbox.left).toBeGreaterThanOrEqual(leftmost.wrapper.left - 1)
      expect(Math.abs(leftmost.checkbox.right - leftmost.name.left)).toBeLessThanOrEqual(1)
      expect(Math.abs(leftmost.name.right - leftmost.id.left)).toBeLessThanOrEqual(1)
      expect(leftmost.name.text).toBe('Name')
      expect(leftmost.id.text).toBe('Account ID')
      expect(leftmost.rowName.text).toContain('Codex Team West')
      expect(Math.abs(leftmost.rowCheckbox.right - leftmost.rowName.left)).toBeLessThanOrEqual(1)
      expect(Math.abs(leftmost.checkbox.left - leftmost.rowCheckbox.left)).toBeLessThanOrEqual(1)

      const scrolled = await table.evaluate(async (element) => {
        element.scrollLeft = Math.min(600, element.scrollWidth - element.clientWidth)
        element.dispatchEvent(new Event('scroll'))
        await new Promise((resolve) => requestAnimationFrame(resolve))
        const wrapper = element.getBoundingClientRect()
        const headers = element.querySelectorAll('thead th')
        const cells = element.querySelectorAll('tbody tr[data-index]:first-of-type td')
        const checkbox = headers[0].getBoundingClientRect()
        const name = headers[1].getBoundingClientRect()
        const rowName = cells[1].getBoundingClientRect()
        const actions = headers[headers.length - 1].getBoundingClientRect()
        return {
          scrollLeft: element.scrollLeft,
          hasScrolledClass: element.classList.contains('is-scrolled-left'),
          wrapper: { left: wrapper.left, right: wrapper.right },
          checkbox: { left: checkbox.left, right: checkbox.right },
          name: { left: name.left },
          rowName: { left: rowName.left },
          actions: { right: actions.right },
        }
      })

      expect(scrolled.scrollLeft, `${viewport.width}px horizontal scroll`).toBeGreaterThan(0)
      expect(scrolled.hasScrolledClass).toBe(true)
      expect(Math.abs(scrolled.checkbox.left - scrolled.wrapper.left)).toBeLessThanOrEqual(1)
      expect(Math.abs(scrolled.checkbox.right - scrolled.name.left)).toBeLessThanOrEqual(1)
      expect(Math.abs(scrolled.name.left - scrolled.rowName.left)).toBeLessThanOrEqual(1)
      expect(scrolled.actions.right).toBeLessThanOrEqual(scrolled.wrapper.right + 1)
    }
  })

  test('keeps one Accounts scroll path and reaches the final account in both views', async ({ page }) => {
    test.setTimeout(60_000)
    const { accounts: manyAccounts } = await installManyAccountMock(page)
    const finalAccount = manyAccounts.find((account) => account.platform === 'deepseek')!

    for (const viewport of [
      { width: 1440, height: 900 },
      { width: 1024, height: 768 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport)
      await page.goto('/admin/accounts')

      const main = page.locator('main.app-main')
      const capacityRows = page.getByTestId('account-capacity-row')
      await expect(capacityRows).toHaveCount(manyAccounts.length)
      const providerHeader = page.locator('.operator-capacity-provider-header').first()
      const providerStyle = await providerHeader.evaluate((element) => {
        const style = getComputedStyle(element)
        return {
          radius: Number.parseFloat(style.borderRadius),
          boxShadow: style.boxShadow,
          borderLeftWidth: style.borderLeftWidth,
          borderRightWidth: style.borderRightWidth,
          borderStyle: style.borderStyle,
        }
      })
      expect(providerStyle.radius).toBeGreaterThan(0)
      expect(providerStyle.radius).toBeLessThanOrEqual(8)
      expect(providerStyle.boxShadow).toBe('none')
      expect(providerStyle.borderLeftWidth).toBe(providerStyle.borderRightWidth)
      expect(providerStyle.borderStyle).toContain('solid')

      const antigravityRow = capacityRows.filter({ hasText: 'Antigravity Pro' })
      await expect(antigravityRow.locator('.operator-capacity-row-window')).toHaveCount(3)
      await expect(antigravityRow).toContainText('0% minimum across 20 models')
      if (viewport.width === 1440) {
        await antigravityRow.locator('.operator-capacity-details-toggle').click()
        const details = antigravityRow.getByTestId('account-technical-details')
        await expect(details.locator('.operator-capacity-window')).toHaveCount(3)
        await expect(details).toContainText('More model limits (17)')
        await details.getByRole('button', { name: 'Inspect a model limit' }).click()
        await page.getByRole('textbox', { name: 'Search model limits' }).fill('hidden-model-search-target')
        await page.getByRole('option', { name: 'hidden-model-search-target' }).click()
        await expect(details.locator('.operator-capacity-window')).toHaveCount(3)
        await expect(details).toContainText('hidden-model-search-target')
      }

      const initialMetrics = await page.evaluate(() => ({
        documentClientHeight: document.documentElement.clientHeight,
        documentScrollHeight: document.documentElement.scrollHeight,
        windowScrollY: window.scrollY,
      }))
      expect(initialMetrics.documentScrollHeight, `${viewport.width}px document tail`).toBeLessThanOrEqual(
        initialMetrics.documentClientHeight + 1,
      )
      expect(initialMetrics.windowScrollY).toBe(0)

      await main.evaluate((element) => { element.scrollTop = element.scrollHeight })
      const finalCapacityRow = capacityRows.filter({ hasText: finalAccount.name })
      await expect(finalCapacityRow).toBeVisible()
      const capacityScroll = await main.evaluate((element) => ({
        clientHeight: element.clientHeight,
        scrollHeight: element.scrollHeight,
        scrollTop: element.scrollTop,
      }))
      expect(capacityScroll.scrollHeight).toBeGreaterThan(capacityScroll.clientHeight)
      expect(capacityScroll.scrollTop).toBeGreaterThan(0)

      await page.getByRole('tab', { name: 'Technical' }).click()
      const pagination = page.locator('.operator-account-pagination')
      const nextPage = pagination.getByRole('button', { name: 'Next' })
      while (await nextPage.isEnabled()) await nextPage.click()
      const technicalFinal = page.locator('.operator-account-table').getByText(finalAccount.name, { exact: true })
      await main.evaluate((element) => { element.scrollTop = element.scrollHeight })
      await expect(technicalFinal).toBeVisible()
      await expect(pagination).toBeVisible()

      const finalMetrics = await page.evaluate(() => ({
        documentClientHeight: document.documentElement.clientHeight,
        documentScrollHeight: document.documentElement.scrollHeight,
        windowScrollY: window.scrollY,
      }))
      expect(finalMetrics.documentScrollHeight).toBeLessThanOrEqual(finalMetrics.documentClientHeight + 1)
      expect(finalMetrics.windowScrollY).toBe(0)
    }
  })

  for (const gridCase of [
    { name: '2 cards', accounts: [operatorFixtureAccounts[0]], cardCount: 2 },
    { name: '3 cards', accounts: [operatorFixtureAccounts[0], operatorFixtureAccounts[2]], cardCount: 3 },
    { name: '4 cards', accounts: [operatorFixtureAccounts[0], operatorFixtureAccounts[2], operatorFixtureAccounts[5]], cardCount: 4 },
    { name: 'more than 4 cards', accounts: [operatorFixtureAccounts[0], operatorFixtureAccounts[1], operatorFixtureAccounts[2], operatorFixtureAccounts[5]], cardCount: 5 },
  ]) {
    test(`fills the Capacity row with ${gridCase.name}`, async ({ page }) => {
      await installAccountListMock(page, gridCase.accounts)
      await page.setViewportSize({ width: 1440, height: 900 })
      await page.goto('/admin/stats')

      const overview = page.getByTestId('stats-capacity-donut-overview')
      const cards = overview.locator('.stats-capacity-donut')
      await expect(cards).toHaveCount(gridCase.cardCount)
      const metrics = await overview.evaluate((element) => {
        const gridBounds = element.getBoundingClientRect()
        const cardBounds = Array.from(element.children).map((child) => child.getBoundingClientRect())
        const firstRow = cardBounds.filter((bounds) => Math.abs(bounds.top - cardBounds[0].top) < 1)
        return {
          gridWidth: gridBounds.width,
          firstRowWidth: firstRow.at(-1)!.right - firstRow[0].left,
          cardWidths: cardBounds.map((bounds) => bounds.width),
        }
      })
      expect(metrics.firstRowWidth).toBeGreaterThanOrEqual(metrics.gridWidth - 1)
      expect(Math.max(...metrics.cardWidths) - Math.min(...metrics.cardWidths)).toBeLessThanOrEqual(1)

      if (gridCase.cardCount === 3) {
        for (const viewport of [{ width: 1024, height: 768 }, { width: 390, height: 844 }]) {
          await page.setViewportSize(viewport)
          const layout = await overview.evaluate((element) => ({
            gridWidth: element.getBoundingClientRect().width,
            cardWidths: Array.from(element.children).map((child) => child.getBoundingClientRect().width),
            cardTops: Array.from(element.children).map((child) => child.getBoundingClientRect().top),
            clientWidth: document.documentElement.clientWidth,
            scrollWidth: document.documentElement.scrollWidth,
          }))
          expect(layout.scrollWidth, `${viewport.width}px page overflow`).toBeLessThanOrEqual(layout.clientWidth + 1)
          if (viewport.width === 390) {
            expect(layout.cardWidths.every((width) => Math.abs(width - layout.gridWidth) <= 1)).toBe(true)
            expect(new Set(layout.cardTops.map(Math.round)).size).toBe(gridCase.cardCount)
          }
        }
      }
    })
  }

  test('promotes Gateway usage and separates short and long capacity on Stats', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 })
    const unknownAccount = {
      ...operatorFixtureAccounts[0],
      id: 999,
      name: 'OpenAI quota unknown',
      type: 'apikey',
      credentials: {},
      credentials_status: { has_api_key: true },
      extra: {},
      group_ids: [...operatorFixtureAccounts[0].group_ids],
      groups: operatorFixtureAccounts[0].groups.map((group) => ({ ...group })),
    }
    await installAccountListMock(page, [...operatorFixtureAccounts, unknownAccount])
    await page.goto('/admin/dashboard')

    const summary = page.getByTestId('account-pool-capacity')
    await expect(summary).toBeVisible()
    await expect(summary).toContainText('14%')
    await expect(summary.locator('[data-testid="capacity-account-segment"]')).toHaveCount(0)
    await expect(summary.getByRole('link', { name: 'View Stats' })).toHaveAttribute('href', '/admin/stats')

    await Promise.all([
      page.waitForURL(/\/admin\/stats$/),
      summary.getByRole('link', { name: 'View Stats' }).click(),
    ])

    const usage = page.locator('.stats-usage-section')
    await expect(usage).toContainText('Gateway usage')
    await expect(usage).toContainText(/4,286.*RPM/s)
    await expect(usage).toContainText(/5\.5M.*TPM/s)
    await expect(usage).toContainText('Actual cost')
    await expect(usage).toContainText('Account cost')
    await expect(usage).toContainText('Average response')
    const requestTrend = page.getByTestId('stats-request-trend')
    const tokenTrend = page.getByTestId('stats-token-trend')
    await expect(requestTrend).toBeVisible()
    await expect(tokenTrend).toBeVisible()
    await expect(requestTrend.getByTestId('stats-trend-bar')).toHaveCount(6)
    await expect(tokenTrend.getByTestId('stats-trend-bar')).toHaveCount(6)
    await expect(requestTrend.getByTestId('stats-trend-total')).toHaveText('4.3K')
    await expect(requestTrend.getByTestId('stats-trend-average')).toHaveText('714.3')
    await expect(requestTrend.getByTestId('stats-trend-peak')).toHaveText('946')
    await expect(tokenTrend.getByTestId('stats-trend-total')).toHaveText('6.4M')
    await expect(tokenTrend.getByTestId('stats-trend-average')).toHaveText('1.1M')
    await expect(tokenTrend.getByTestId('stats-trend-peak')).toHaveText('1.4M')
    await expect(requestTrend.getByRole('group')).toHaveAccessibleName(/Latest 6 hourly periods.*4,286 Requests/)
    await expect(tokenTrend.getByRole('group')).toHaveAccessibleName(/Latest 6 hourly periods.*6,396,500 Tokens/)
    await expectStatsBandGeometry(requestTrend, '1440px request chart')
    await expectStatsBandGeometry(tokenTrend, '1440px token chart')

    const requestBars = requestTrend.getByTestId('stats-trend-bar')
    expect(await requestBars.first().locator('span').evaluate((element) => element.getBoundingClientRect().width)).toBeGreaterThan(20)
    await requestBars.nth(2).focus()
    await expect(requestBars.nth(2)).toHaveClass(/is-active/)
    await expect(requestTrend.getByTestId('stats-trend-tooltip')).toContainText('684 Requests')
    await page.keyboard.press('ArrowRight')
    await expect(requestTrend.getByTestId('stats-trend-tooltip')).toContainText('792 Requests')
    await page.keyboard.press('Escape')
    await expect(requestTrend.getByTestId('stats-trend-tooltip')).toHaveCount(0)

    const tokenBars = tokenTrend.getByTestId('stats-trend-bar')
    await tokenBars.nth(2).hover()
    await expect(tokenBars.nth(2)).toHaveClass(/is-active/)
    expect(await tokenBars.nth(2).locator('span').evaluate((element) => getComputedStyle(element).boxShadow)).not.toBe('none')
    await expect(tokenTrend.getByTestId('stats-trend-tooltip')).toContainText('988.0K Tokens')
    await page.mouse.move(0, 0)
    await expect(tokenTrend.getByTestId('stats-trend-tooltip')).toHaveCount(0)

    const peakPoint = requestBars.nth(4)
    await peakPoint.focus()
    const [chartBox, peakTooltipBox] = await Promise.all([
      requestTrend.getByRole('group').boundingBox(),
      requestTrend.getByTestId('stats-trend-tooltip').boundingBox(),
    ])
    expect(chartBox).not.toBeNull()
    expect(peakTooltipBox).not.toBeNull()
    expect(peakTooltipBox!.y).toBeGreaterThanOrEqual(chartBox!.y)
    expect(peakTooltipBox!.y + peakTooltipBox!.height).toBeLessThanOrEqual(chartBox!.y + chartBox!.height)
    await page.keyboard.press('Escape')
    const [requestTrendBox, tokenTrendBox] = await Promise.all([
      requestTrend.boundingBox(),
      tokenTrend.boundingBox(),
    ])
    expect(requestTrendBox).not.toBeNull()
    expect(tokenTrendBox).not.toBeNull()
    expect(tokenTrendBox!.x).toBeGreaterThanOrEqual(requestTrendBox!.x + requestTrendBox!.width - 1)
    expect(await usage.evaluate((element, capacitySelector) => (
      Boolean(element.compareDocumentPosition(document.querySelector(capacitySelector)!) & Node.DOCUMENT_POSITION_FOLLOWING)
    ), '.stats-capacity-section')).toBe(true)

    const shortTerm = page.getByTestId('stats-short-term-capacity')
    const longTerm = page.getByTestId('stats-long-term-capacity')
    await expect(shortTerm).toContainText('5h / short-term capacity')
    await expect(longTerm).toContainText('Weekly / total capacity')
    await expect(shortTerm.getByTestId('short-provider-openai')).toContainText('52%')
    await expect(longTerm.getByTestId('long-provider-openai')).toContainText('52%')
    await expect(longTerm.getByTestId('long-provider-anthropic')).toContainText('4%')
    await expect(shortTerm.getByTestId('short-provider-gemini')).toContainText('0%')
    await expect(shortTerm.getByTestId('short-provider-openai')).toContainText('1 known, 1 without this window')
    await expect(shortTerm.getByTestId('short-provider-gemini')).toContainText('1 known, 0 without this window')
    await expect(longTerm.getByTestId('long-provider-gemini')).toContainText('No supported window reported')
    await expect(shortTerm.getByTestId('short-provider-openai').locator('svg')).toBeVisible()
    const antigravityCapacity = page.getByTestId('provider-capacity-antigravity')
    await expect(antigravityCapacity).toContainText('Antigravity')
    await expect(antigravityCapacity).not.toContainText('Google')
    const antigravityIcon = antigravityCapacity.locator('[data-provider-brand="antigravity"]')
    await expect(antigravityIcon).toBeVisible()
    await expect(antigravityIcon).toHaveAttribute('fill', 'currentColor')
    await expect(antigravityIcon).toHaveAttribute('viewBox', '0 0 112 112')
    await expect(antigravityIcon.locator('path')).toHaveAttribute('d', /^M89\.6992 93\.695/)
    expect(await antigravityIcon.locator('path').evaluateAll((paths) => paths.every((path) => !path.getAttribute('fill')))).toBe(true)
    await expect(antigravityCapacity.locator('[data-provider-brand="google"]')).toHaveCount(0)
    await expect(antigravityCapacity.locator('path[d^="M19.35 10.04"]')).toHaveCount(0)
    const capacityCards = page.locator('.stats-capacity-donut')
    await expect(capacityCards).toHaveCount(5)
    const capacityCardMetrics = await capacityCards.evaluateAll((cards) => cards.map((card) => ({
      height: card.getBoundingClientRect().height,
      metadataRows: card.querySelectorAll('dl > div').length,
      clientWidth: card.clientWidth,
      scrollWidth: card.scrollWidth,
      padding: Number.parseFloat(getComputedStyle(card).paddingTop),
      rowGap: Number.parseFloat(getComputedStyle(card).rowGap),
      columnGap: Number.parseFloat(getComputedStyle(card).columnGap),
    })))
    expect(new Set(capacityCardMetrics.map((card) => Math.round(card.height))).size).toBe(1)
    expect(capacityCardMetrics.every((card) => card.metadataRows === 4 && card.scrollWidth <= card.clientWidth)).toBe(true)
    expect(capacityCardMetrics.every((card) => card.padding >= 18 && card.rowGap >= 14 && card.columnGap >= 16)).toBe(true)
    const donutValueFit = await capacityCards.first().locator('.stats-capacity-donut-value').evaluate((element) => {
      const value = element.querySelector<HTMLElement>('strong')!
      const chart = element.closest<HTMLElement>('.stats-capacity-donut-chart')!
      const original = value.textContent
      value.textContent = '100.00%'
      const chartBounds = chart.getBoundingClientRect()
      const valueBounds = value.getBoundingClientRect()
      const metrics = {
        fontSize: Number.parseFloat(getComputedStyle(value).fontSize),
        clientWidth: element.clientWidth,
        scrollWidth: element.scrollWidth,
        left: valueBounds.left - chartBounds.left,
        right: chartBounds.right - valueBounds.right,
      }
      value.textContent = original
      return metrics
    })
    expect(donutValueFit.fontSize).toBeLessThanOrEqual(16)
    expect(donutValueFit.scrollWidth).toBeLessThanOrEqual(donutValueFit.clientWidth)
    expect(donutValueFit.left).toBeGreaterThanOrEqual(4)
    expect(donutValueFit.right).toBeGreaterThanOrEqual(4)
    await expect(page.getByTestId('stats-capacity-donut-overall'))
      .toContainText('Mixed-provider average · limiting quota · not pooled')
    await expect(page.getByTestId('stats-capacity-donut-overall').locator('.stats-capacity-donut-chart svg'))
      .toHaveAccessibleName(/Mixed-provider average · limiting quota · not pooled:/)
    await expect(page.getByTestId('provider-capacity-openai'))
      .toContainText('Average limiting quota · not pooled')
    await expect(page.getByTestId('provider-capacity-openai').locator('.stats-capacity-donut-chart svg'))
      .toHaveAccessibleName(/OpenAI, Average limiting quota · not pooled:/)
    await expect(page.getByTestId('provider-capacity-openai').getByTestId('stats-capacity-donut-coverage'))
      .toContainText('Coverage')
    await expect(page.getByTestId('provider-capacity-openai').getByTestId('stats-capacity-donut-limit'))
      .toContainText('Limiting account')
    await expect(page.getByTestId('provider-capacity-openai').locator('.stats-capacity-donut-value span')).toHaveCount(0)

    const inspector = page.getByTestId('stats-capacity-inspector')
    const detail = inspector.getByTestId('stats-selected-account-detail')
    const accountTrigger = inspector.getByRole('button', { name: 'Inspect account' })
    await expect(page.locator('.stats-view select')).toHaveCount(0)
    await expect(detail).toHaveCount(1)
    await expect(inspector.getByTestId('stats-inspector-account-row')).toHaveCount(0)
    await expect(inspector.getByText('Inspect capacity window')).toHaveCount(0)
    await openPopupSelect(page, accountTrigger)

    await accountTrigger.press('ArrowDown')
    const accountSearch = page.getByRole('textbox', { name: 'Search accounts' })
    await expect(accountSearch).toBeFocused()
    await accountSearch.fill('Codex Team West')
    await page.getByRole('option', { name: /Codex Team West/ }).click()
    await expect(detail).toContainText('Codex Team West')
    await expect(detail.getByTestId('stats-account-capacity-window')).toHaveCount(2)
    await expect(detail).toContainText('5h')
    await expect(detail).toContainText('68%')
    await expect(detail).toContainText('7d')
    await expect(detail).toContainText('52%')
    await expect(detail.getByTestId('stats-account-limiting-window')).toContainText('7d · 52% remaining')
    await expect(detail).toContainText('Schedulable')
    await expect(detail).toContainText('Passive quota state')
    await expect(detail).toContainText('OpenAI Production, Balanced Coding Route')
    await expect(detail).not.toContainText('codex-west@example.test')

    await accountTrigger.press('ArrowDown')
    await expect(accountSearch).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(accountTrigger).toHaveAttribute('aria-expanded', 'false')
    await expect(accountTrigger).toBeFocused()
    await expect(page.locator('body > .select-dropdown-portal')).toHaveCount(0)

    await accountTrigger.press('ArrowDown')
    await expect(page.locator('.select-dropdown-portal .select-option-focused'))
      .toContainText('Codex Team West')
    await page.keyboard.press('ArrowDown')
    await expect(page.locator('.select-dropdown-portal .select-option-focused'))
      .toContainText('Antigravity Pro')
    await page.keyboard.press('ArrowDown')
    await expect(page.locator('.select-dropdown-portal .select-option-focused'))
      .toContainText('Gemini Quota Limited')
    await page.keyboard.press('Enter')
    await expect(accountTrigger).toContainText('Gemini Quota Limited')
    await expect(detail).toContainText('Gemini Quota Limited')

    await accountTrigger.click()
    await page.getByRole('textbox', { name: 'Search accounts' }).fill('Antigravity Pro')
    await page.getByRole('option', { name: /Antigravity Pro/ }).click()
    await expect(detail).toContainText('Antigravity Pro')
    await expect(detail.getByTestId('stats-model-limit-row')).toHaveCount(3)
    await expect(inspector.getByRole('button', { name: 'Inspect a model limit' })).toBeVisible()

    await accountTrigger.click()
    await page.getByRole('textbox', { name: 'Search accounts' }).fill('OpenAI quota unknown')
    await page.getByRole('option', { name: /OpenAI quota unknown/ }).click()
    await expect(detail).toContainText('OpenAI quota unknown')
    await expect(detail.getByTestId('stats-account-capacity-unknown')).toBeVisible()
    await expect(detail).not.toContainText('0%')

    await accountTrigger.click()
    await page.getByRole('textbox', { name: 'Search accounts' }).fill('Gemini Quota Limited')
    await page.getByRole('option', { name: /Gemini Quota Limited/ }).click()
    await expect(detail.getByTestId('stats-account-capacity-window')).toHaveCount(1)
    await expect(detail).toContainText('Daily')
    await expect(detail).toContainText('0%')

    const [shortBox, longBox] = await Promise.all([shortTerm.boundingBox(), longTerm.boundingBox()])
    expect(shortBox).not.toBeNull()
    expect(longBox).not.toBeNull()
    expect(longBox!.x).toBeGreaterThanOrEqual(shortBox!.x + shortBox!.width - 1)

    for (const [viewportIndex, viewport] of [{ width: 1024, height: 768 }, { width: 390, height: 844 }].entries()) {
      await page.setViewportSize(viewport)
      // Charts re-render after the resize, so measure both boxes in one frame once layout settles.
      const stacked = (upper: Locator, lower: Locator) => async () => {
        const [upperBox, lowerBox] = await Promise.all([upper.boundingBox(), lower.boundingBox()])
        return upperBox !== null && lowerBox !== null && lowerBox.y >= upperBox.y + upperBox.height - 1
      }
      await expect.poll(stacked(shortTerm, longTerm)).toBe(true)
      await expect.poll(stacked(requestTrend, tokenTrend)).toBe(true)
      await requestBars.nth(viewportIndex === 0 ? 0 : 5).focus()
      await expect(requestTrend.getByTestId('stats-trend-tooltip')).toBeVisible()
      await page.keyboard.press('Escape')
      await tokenBars.nth(viewportIndex === 0 ? 5 : 0).focus()
      await expect(tokenTrend.getByTestId('stats-trend-tooltip')).toBeVisible()
      await page.keyboard.press('Escape')
      await expectStatsBandGeometry(requestTrend, `${viewport.width}px request chart`)
      await expectStatsBandGeometry(tokenTrend, `${viewport.width}px token chart`)
      const overflow = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }))
      expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1)
    }
  })

  test('bounds large-pool account and Antigravity model diagnostics', async ({ page }) => {
    test.setTimeout(60_000)
    const fixture = await installManyAccountMock(page)
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/stats')

    await expect(page.getByTestId('stats-capacity-donut-overview')).toBeVisible()
    await expect(page.getByTestId('stats-capacity-donut-overall')).toBeVisible()
    await expect(page.getByTestId('stats-short-term-capacity')).toBeVisible()
    await expect(page.getByTestId('stats-long-term-capacity')).toBeVisible()
    const googleCapacity = page.getByTestId('provider-capacity-antigravity')
    await expect(googleCapacity.locator('.stats-capacity-donut-icon svg')).toBeVisible()
    await expect(googleCapacity.getByTestId('stats-capacity-donut-quota')).toHaveText('claude-opus-4-1 · +19')
    await expect(googleCapacity).not.toContainText('hidden-model-search-target')
    await expect(googleCapacity.getByTestId('stats-capacity-donut-quota')).toHaveAttribute('title', /hidden-model-search-target/)

    const inspector = page.getByTestId('stats-capacity-inspector')
    const detail = inspector.getByTestId('stats-selected-account-detail')
    const accountTrigger = inspector.getByRole('button', { name: 'Inspect account' })
    await expect(detail).toHaveCount(1)
    await expect(inspector.getByTestId('stats-inspector-account-row')).toHaveCount(0)
    await expect(page.locator('.stats-view select')).toHaveCount(0)
    await expect(detail).toContainText('Antigravity Pro')
    await expect(detail.getByTestId('stats-model-limit-row')).toHaveCount(3)
    await expect(detail.getByTestId('stats-model-limit-summary')).toContainText('0% minimum across 20 models')
    await expect(detail).toContainText('More model limits (17)')

    const closedDocumentHeight = await page.evaluate(() => document.documentElement.scrollHeight)
    await accountTrigger.click()
    await expect(page.getByRole('option')).toHaveCount(fixture.accounts.length)
    expect(await page.evaluate(() => document.documentElement.scrollHeight)).toBe(closedDocumentHeight)

    await page.getByRole('textbox', { name: 'Search accounts' }).fill('Exhausted pool 01')
    await page.getByRole('option', { name: /Exhausted pool 01/ }).click()
    await expect(detail).toContainText('Exhausted pool 01')
    await expect(detail.getByTestId('stats-account-capacity-window')).toHaveCount(2)
    await expect(detail.getByTestId('stats-account-limiting-window')).toContainText('5h · 0% remaining')

    await accountTrigger.click()
    await page.getByRole('textbox', { name: 'Search accounts' }).fill(fixture.hiddenAccount.name)
    await page.getByRole('option', { name: new RegExp(fixture.hiddenAccount.name) }).click()
    await expect(detail).toContainText(fixture.hiddenAccount.name)
    await expect(detail.getByTestId('stats-account-capacity-unknown')).toBeVisible()
    await expect(detail).not.toContainText('0%')

    await accountTrigger.click()
    await page.getByRole('textbox', { name: 'Search accounts' }).fill('Antigravity Pro')
    await page.getByRole('option', { name: /Antigravity Pro/ }).click()
    await expect(detail.getByTestId('stats-model-limit-row')).toHaveCount(3)
    await expect(detail.getByTestId('stats-model-limit-row').first()).toContainText('claude-opus-4-1')

    await inspector.getByRole('button', { name: 'Inspect a model limit' }).click()
    await page.getByRole('textbox', { name: 'Search model limits' }).fill(fixture.hiddenModel)
    await page.getByRole('option', { name: new RegExp(fixture.hiddenModel) }).click()
    await expect(detail.getByTestId('stats-model-limit-row')).toHaveCount(3)
    await expect(detail).toContainText(fixture.hiddenModel)

    for (const viewport of [
      { width: 1440, height: 900 },
      { width: 1024, height: 768 },
      { width: 390, height: 844 },
    ]) {
      await page.setViewportSize(viewport)
      const bounds = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
        selectedDetails: document.querySelectorAll('[data-testid="stats-selected-account-detail"]').length,
        inspectorRows: document.querySelectorAll('[data-testid="stats-inspector-account-row"]').length,
        modelRows: document.querySelectorAll('[data-testid="stats-model-limit-row"]').length,
        nativeSelects: document.querySelectorAll('.stats-view select').length,
      }))
      expect(bounds.scrollWidth, `${viewport.width}px page overflow`).toBeLessThanOrEqual(bounds.clientWidth + 1)
      expect(bounds.selectedDetails).toBe(1)
      expect(bounds.inspectorRows).toBe(0)
      expect(bounds.modelRows).toBeLessThanOrEqual(3)
      expect(bounds.nativeSelects).toBe(0)
    }
  })

  test('drills from Overview into filtered operator states and expands account details', async ({ page }) => {
    await page.goto('/admin/dashboard')

    const fleet = page.locator('.operator-fleet-summary')
    await expect(fleet.locator('.operator-fleet-metric').nth(0)).toContainText('6')
    await expect(fleet.locator('.operator-fleet-metric.is-active')).toContainText('2')
    await expect(fleet.locator('.operator-fleet-metric.is-limited')).toContainText('2')
    await expect(fleet.locator('.operator-fleet-metric.is-error')).toContainText('1')
    await expect(fleet.locator('.operator-fleet-disabled')).toContainText('1 disabled')

    await fleet.locator('.operator-fleet-metric.is-limited').click()
    await expect(page).toHaveURL(/\/admin\/accounts\?operator_status=limited$/)
    const limitedRows = page.locator('[data-testid="account-capacity-row"][data-status="limited"]')
    await expect(limitedRows).toHaveCount(2)
    await expect(limitedRows.filter({ hasText: 'Antigravity Pro' })).toHaveCount(1)
    await expect(limitedRows.filter({ hasText: 'Gemini Quota Limited' })).toHaveCount(1)

    const cooldownRow = limitedRows.filter({ hasText: 'Antigravity Pro' })
    const detailsToggle = cooldownRow.locator('.operator-capacity-details-toggle')
    await expect(detailsToggle).toHaveAccessibleName('Show details for Antigravity Pro')
    await detailsToggle.focus()
    await page.keyboard.press('Enter')
    await expect(detailsToggle).toHaveAttribute('aria-expanded', 'true')
    await expect(cooldownRow.getByTestId('account-technical-details')).toBeVisible()
    const cooldownStatus = cooldownRow.locator('.operator-capacity-account-status small')
    await expect(cooldownStatus).toContainText('Resets')
    await expect(cooldownStatus).toHaveAttribute('title', /Provider cooldown until/)

    await page.goto('/admin/dashboard')
    await page.locator('.operator-fleet-metric.is-error').click()
    await expect(page).toHaveURL(/\/admin\/accounts\?operator_status=error$/)
    const errorRows = page.locator('[data-testid="account-capacity-row"][data-status="error"]')
    await expect(errorRows).toHaveCount(1)
    await expect(errorRows).toContainText('Gemini Recovery')
    await expect(errorRows).toContainText('Fixture: reauthorization required')

    await page.goto('/admin/accounts?operator_status=disabled')
    const disabledRows = page.locator('[data-testid="account-capacity-row"][data-status="disabled"]')
    await expect(disabledRows).toHaveCount(1)
    await expect(disabledRows).toContainText('Codex Standby')
  })

  test('keeps proxy rows inside the visible table and uses compact named actions', async ({ page }) => {
    for (const width of [1440, 1024, 768]) {
      await page.setViewportSize({ width, height: 900 })
      await page.goto('/admin/proxies')
      const table = page.locator('.operator-proxy-table .table-wrapper')
      const row = table.locator('tbody tr[data-index]').first()
      await expect(row).toBeVisible()

      const bounds = await table.evaluate((element) => {
        const row = element.querySelector('tbody tr[data-index]') as HTMLElement
        return {
          clientWidth: element.clientWidth,
          scrollWidth: element.scrollWidth,
          rowWidth: row.getBoundingClientRect().width,
        }
      })
      expect(bounds.scrollWidth, `${width}px table overflow`).toBeLessThanOrEqual(bounds.clientWidth + 1)
      expect(bounds.rowWidth, `${width}px row hover surface`).toBeLessThanOrEqual(bounds.clientWidth + 1)

      for (const name of ['Test Connection', 'Quality Check', 'Edit', 'Delete']) {
        const action = row.getByRole('button', { name: new RegExp(`^${name} US West relay$`) })
        await expect(action).toBeVisible()
        await expect(action).toHaveText('')
      }
    }

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/admin/proxies')
    await expect(page.locator('.operator-proxy-table .table-wrapper')).toHaveCount(0)
    await expect(page.locator('.operator-proxy-row-actions')).toHaveCount(2)
    const overflow = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }))
    expect(overflow.scrollWidth).toBeLessThanOrEqual(overflow.clientWidth + 1)
  })

  test('collapses provider groups from the keyboard and remembers the session state', async ({ page }) => {
    await page.goto('/admin/accounts')

    const toggle = page.getByTestId('provider-toggle-openai')
    const panel = page.locator('#operator-provider-openai-accounts')
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    await expect(toggle).toContainText('52% normalized remaining')
    await expect(toggle).toContainText('Lowest 52% - Codex Team West')
    await expect(toggle).toContainText('Next limiting reset')
    await toggle.focus()
    await page.keyboard.press('Enter')
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await expect(panel).toBeHidden()

    await page.reload()
    await expect(page.getByTestId('provider-toggle-openai')).toHaveAttribute('aria-expanded', 'false')
    await page.getByTestId('provider-toggle-openai').focus()
    await page.keyboard.press('Space')
    await expect(page.getByTestId('provider-toggle-openai')).toHaveAttribute('aria-expanded', 'true')
    await expect(panel).toBeVisible()
  })

  test('uses accent switches and token-driven open-menu states across operator areas', async ({ page }) => {
    test.setTimeout(60_000)
    await page.addInitScript(() => localStorage.setItem('theme', 'dark'))
    await page.setViewportSize({ width: 1024, height: 900 })
    await page.goto('/admin/accounts')
    const tokens = await operatorTokens(page)

    const indicatorOpacity = (link: Locator) =>
      link.evaluate((element) => getComputedStyle(element, '::after').opacity)
    await expect.poll(() => indicatorOpacity(page.locator('.operator-context-link-active'))).toBe('1')
    expect(await indicatorOpacity(page.locator('.operator-context-link:not(.operator-context-link-active)').first()))
      .toBe('0')

    const localeTrigger = page.locator('.operator-locale-trigger')
    await localeTrigger.hover()
    await expect.poll(() => localeTrigger.evaluate((element) => getComputedStyle(element).backgroundColor))
      .toBe(tokens.muted)
    await localeTrigger.focus()
    await page.keyboard.press('Enter')
    const localeMenu = page.locator('.app-header .operator-menu')
    const selectedLocale = localeMenu.getByRole('menuitemradio', { name: /English/ })
    await expectPopupMenu(page, localeMenu, selectedLocale, 'selected')
    await expect(selectedLocale).toHaveAttribute('aria-checked', 'true')
    const otherLocale = localeMenu.getByRole('menuitemradio', { name: /中文/ })
    await otherLocale.hover()
    await expect.poll(() => otherLocale.evaluate((element) => getComputedStyle(element).backgroundColor))
      .toBe(tokens.popupHover)
    await selectedLocale.click()
    await expect(localeMenu).toBeHidden()
    await localeTrigger.focus()
    await page.keyboard.press('Enter')
    await page.keyboard.press('Tab')
    await expect(selectedLocale).toBeFocused()
    await page.keyboard.press('Enter')
    await expect(localeMenu).toBeHidden()

    await page.getByRole('tab', { name: 'Technical' }).click()

    const enabledSwitch = page.locator('.operator-scheduling-switch[aria-checked="true"]').first()
    const disabledSwitch = page.locator('.operator-scheduling-switch[aria-checked="false"]').first()
    await expect(enabledSwitch).toBeVisible()
    await expect(disabledSwitch).toBeVisible()
    await expect(enabledSwitch).toHaveAttribute('aria-label', 'Scheduling enabled: Codex Team West')
    await expect(disabledSwitch).toHaveAttribute('aria-label', /Scheduling disabled: /)
    await expect(enabledSwitch).toHaveText('')
    await expect(disabledSwitch).toHaveText('')
    expect(await enabledSwitch.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(tokens.switchOn)
    expect(await disabledSwitch.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(tokens.track)

    const secondTechnicalRow = page.locator('.operator-account-table .table-body tr[data-index="1"]')
    expect(await secondTechnicalRow.locator('td').first().evaluate((element) => getComputedStyle(element).borderTopColor))
      .toBe(tokens.borderSubtle)

    const routingGroup = page.locator('.operator-account-routing-groups .group-badge').first()
    expect(await routingGroup.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, color: style.color }
    })).toEqual({ background: tokens.muted, color: tokens.mutedForeground })

    const platformSelect = page.locator('.select-trigger').first()
    const triggerColors = () => platformSelect.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, border: style.borderColor, color: style.color }
    })
    expect(await triggerColors()).toEqual({
      background: tokens.card,
      border: tokens.borderStrong,
      color: tokens.foreground,
    })
    await platformSelect.click()
    await expect(page.locator('body > .select-dropdown-portal.operator-select-menu.operator-menu')).toBeVisible()
    await expect.poll(triggerColors).toEqual({
      background: tokens.card,
      border: tokens.accent,
      color: tokens.foreground,
    })
    const selectedOption = page.locator('.select-dropdown-portal .select-option-selected')
    const popup = page.locator('.select-dropdown-portal')
    await expectPopupMenu(page, popup, selectedOption, 'selected')
    const popupColors = await popup.evaluate((element) => {
      const search = element.querySelector('.select-search') as HTMLElement
      const options = element.querySelector('.select-options') as HTMLElement
      return {
        search: getComputedStyle(search).backgroundColor,
        searchBorder: getComputedStyle(search).borderBottomColor,
        options: getComputedStyle(options).backgroundColor,
        scrollbarThumb: getComputedStyle(options).scrollbarColor.split(' ').slice(0, 3).join(' '),
      }
    })
    expect(popupColors).toEqual({
      search: tokens.popupSearch,
      searchBorder: tokens.popupBorder,
      options: tokens.popupSurface,
      scrollbarThumb: tokens.borderStrong,
    })
    expect(await selectedOption.locator('svg').evaluate((element) => getComputedStyle(element).color))
      .toBe(tokens.accentText)
    await page.keyboard.press('Escape')

    const viewportBottom = 900
    const moreActionsButton = page.getByRole('button', { name: 'More Actions' })
    await moreActionsButton.click()
    const moreActionsMenu = page.locator('.account-tools-menu')
    await expectPopupMenu(page, moreActionsMenu, moreActionsMenu.locator('.operator-menu-item').first())
    expect(await moreActionsMenu.locator('.operator-menu-divider').first().evaluate(
      (element) => getComputedStyle(element).borderTopColor,
    )).toBe(tokens.popupBorder)
    expect(await moreActionsMenu.locator('.text-xs.font-semibold.uppercase').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(tokens.popupMuted)
    const moreActionsBox = await moreActionsMenu.boundingBox()
    expect(moreActionsBox).not.toBeNull()
    expect(moreActionsBox!.y + moreActionsBox!.height).toBeLessThanOrEqual(viewportBottom)
    await moreActionsButton.click()

    const moreButton = page.locator('.operator-account-table .operator-table-row-action').filter({ hasText: 'More' }).first()
    await moreButton.click()
    const menu = page.locator('.action-menu-content')
    await expect(menu).toBeVisible()
    const menuBox = await menu.boundingBox()
    expect(menuBox).not.toBeNull()
    expect(menuBox!.y + menuBox!.height).toBeLessThanOrEqual(viewportBottom)
    await expectPopupMenu(page, menu, menu.locator('.operator-menu-item').first())
    await page.keyboard.press('Escape')

    await page.goto('/admin/usage')
    const dateRange = page.locator('.date-picker-trigger')
    await expect(dateRange).toBeVisible()
    const dateColors = () => dateRange.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, border: style.borderColor, color: style.color }
    })
    expect(await dateColors()).toEqual({
      background: tokens.card,
      border: tokens.borderStrong,
      color: tokens.foreground,
    })
    await dateRange.click()
    await expect.poll(dateColors).toEqual({
      background: tokens.card,
      border: tokens.accent,
      color: tokens.foreground,
    })
    await expectPopupMenu(
      page,
      page.locator('.date-picker-dropdown'),
      page.locator('.date-picker-preset-active'),
      'selected',
    )
    expect(await page.locator('.date-picker-preset:not(.date-picker-preset-active)').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(tokens.popupForeground)
    expect(await page.locator('.date-picker-label').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(tokens.popupMuted)
    await page.keyboard.press('Escape')

    await page.goto('/admin/ops')
    const activitySelect = page.locator('.select-trigger').first()
    await activitySelect.click()
    await page.keyboard.press('ArrowDown')
    await expectPopupMenu(
      page,
      page.locator('.select-dropdown-portal'),
      page.locator('.select-dropdown-portal .select-option-focused'),
    )
    await page.keyboard.press('Escape')

    await page.goto('/admin/groups')
    const neutralBadge = page.locator('.badge-gray').first()
    expect(await neutralBadge.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, color: style.color }
    })).toEqual({ background: tokens.muted, color: tokens.mutedForeground })
    expect(await page.locator('.table-body > tr + tr > td').first().evaluate(
      (element) => getComputedStyle(element).borderTopColor,
    )).toBe(tokens.borderSubtle)
    await page.getByRole('button', { name: 'Column Settings' }).click()
    const groupsMenu = page.locator('.operator-menu:visible')
    await expectPopupMenu(page, groupsMenu, groupsMenu.locator('[aria-pressed="true"]').first(), 'checked')

    await page.goto('/admin/settings')
    const settingsTheme = await page.locator('.settings-tabs-shell').evaluate((element) => {
      const active = element.querySelector('.settings-tab-active') as HTMLElement
      return {
        background: getComputedStyle(element).backgroundColor,
        border: getComputedStyle(element).borderColor,
        activeBackground: getComputedStyle(active).backgroundColor,
        activeColor: getComputedStyle(active).color,
      }
    })
    expect(settingsTheme).toEqual({
      background: tokens.muted,
      border: tokens.border,
      activeBackground: tokens.card,
      activeColor: tokens.foreground,
    })
    await page.getByRole('tab', { name: 'Gateway' }).click()
    await page.locator('.select-trigger:visible').first().click()
    await page.keyboard.press('ArrowDown')
    await expectPopupMenu(
      page,
      page.locator('.select-dropdown-portal'),
      page.locator('.select-dropdown-portal .select-option-focused'),
    )
    await page.keyboard.press('Escape')

    const webSearchHeading = page.getByRole('heading', { name: 'Web Search Emulation' })
    await webSearchHeading.scrollIntoViewIfNeeded()
    const webSearchCard = webSearchHeading.locator('xpath=../..')
    const providerCard = webSearchCard.locator('.rounded-lg.border.border-gray-200').first()
    await providerCard.locator(':scope > div').first().click({ position: { x: 12, y: 12 } })
    const proxyTrigger = providerCard.locator('.select-trigger').last()
    await expect(proxyTrigger).toContainText('US West relay')
    await proxyTrigger.click()
    const proxyMenu = providerCard.locator('.select-dropdown.operator-menu')
    const proxyOption = proxyMenu.locator('.select-option').filter({ hasText: 'EU standby relay' })
    await expectPopupMenu(page, proxyMenu, proxyOption)
    expect(await proxyMenu.locator('.select-search-input').evaluate((element) => getComputedStyle(element).color))
      .toBe(tokens.popupForeground)
    await page.keyboard.press('Escape')
    await expect(proxyMenu).toBeHidden()
  })

  test('keeps the row menu open for a queued scroll and closes it when its trigger moves', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 900 })
    await page.goto('/admin/accounts?view=technical')

    const trigger = page.locator('.operator-account-table .operator-table-row-action').filter({ hasText: 'More' }).first()
    await trigger.click()
    const menu = page.locator('.action-menu-content')
    await expect(menu).toBeVisible()

    const table = page.locator('.operator-account-table .table-wrapper')
    const triggerBefore = await trigger.boundingBox()
    await table.evaluate((element) => element.dispatchEvent(new Event('scroll')))
    expect(await trigger.boundingBox()).toEqual(triggerBefore)
    await expect(menu).toBeVisible()
    await expect(menu.locator('.operator-menu-item').first()).toBeEnabled()

    await page.locator('main.app-main').evaluate((element) => { element.scrollTop += 100 })
    await expect.poll(async () => (await trigger.boundingBox())?.y).not.toBe(triggerBefore!.y)
    await expect(menu).toBeHidden()
  })

  test('uses token-driven Create, Edit, and Bulk Edit dialog surfaces and callouts', async ({ page }) => {
    test.setTimeout(60_000)
    await page.addInitScript(() => localStorage.setItem('theme', 'dark'))
    await page.goto('/admin/accounts')

    const createTrigger = page.getByRole('button', { name: 'Create Account', exact: true }).first()
    await createTrigger.click()
    const createDialog = page.getByRole('dialog', { name: 'Create Account' })
    await expectTokenDialog(page, createDialog)
    const tokens = await operatorTokens(page)
    expect(await createDialog.locator('.input-hint').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(tokens.mutedForeground)
    await page.keyboard.press('Escape')

    await page.getByRole('tab', { name: 'Technical' }).click()
    await page.locator('.operator-account-table .operator-table-row-action').filter({ hasText: 'Edit' }).first().click()
    const editDialog = page.getByRole('dialog', { name: 'Edit Account' })
    await expectTokenDialog(page, editDialog)
    await page.keyboard.press('Escape')

    await page.locator('.operator-account-table tbody input[type="checkbox"]').first().check()
    await page.getByRole('button', { name: 'Bulk Edit', exact: true }).click()
    const bulkDialog = page.getByRole('dialog', { name: 'Bulk Edit Accounts' })
    await expectTokenDialog(page, bulkDialog)

    const callout = bulkDialog.locator('.operator-callout-info').first()
    const calloutColors = await callout.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, borderLeft: style.borderLeftColor }
    })
    expect(calloutColors).toEqual({
      background: tokens.infoSoft,
      borderLeft: tokens.accent,
    })

    const toggle = bulkDialog.locator('button[role="switch"]').first()
    await expect(toggle).toBeVisible()
    expect(await toggle.evaluate((element) => getComputedStyle(element).backgroundColor)).toBe(tokens.track)
  })

  test('keeps Auto Warm-up controls accessible and responsive', async ({ page }) => {
    test.setTimeout(60_000)
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/settings')
    await page.getByRole('tab', { name: 'Gateway' }).click()

    const globalSwitch = page.getByRole('switch', { name: 'OpenAI automatic warm-up' })
    await globalSwitch.scrollIntoViewIfNeeded()
    await expect(globalSwitch).toBeVisible()
    await expect(globalSwitch).toHaveAttribute('aria-checked', 'false')
    await globalSwitch.focus()
    await page.keyboard.press('Space')
    await expect(globalSwitch).toHaveAttribute('aria-checked', 'true')

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(globalSwitch).toBeVisible()
    const settingsOverflow = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }))
    expect(settingsOverflow.scrollWidth).toBeLessThanOrEqual(settingsOverflow.clientWidth + 1)

    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/accounts')
    await page.getByRole('tab', { name: 'Technical' }).click()
    const accountRow = page.locator('.operator-account-table tr').filter({ hasText: 'Codex Team West' })
    await accountRow.locator('.operator-table-row-action').filter({ hasText: 'Edit' }).click()
    const dialog = page.getByRole('dialog', { name: 'Edit Account' })
    const accountSwitch = dialog.getByRole('switch', { name: 'Automatic warm-up' })
    await accountSwitch.scrollIntoViewIfNeeded()
    await expect(accountSwitch).toHaveAttribute('aria-checked', 'true')
    await expect(dialog.getByTestId('auto-warmup-last-attempt')).toContainText('Last attempt: Succeeded')
    await accountSwitch.focus()
    await page.keyboard.press('Space')
    await expect(accountSwitch).toHaveAttribute('aria-checked', 'false')

    await page.setViewportSize({ width: 390, height: 844 })
    await expect(accountSwitch).toBeVisible()
    const dialogOverflow = await dialog.evaluate((element) => ({
      clientWidth: element.clientWidth,
      scrollWidth: element.scrollWidth,
    }))
    expect(dialogOverflow.scrollWidth).toBeLessThanOrEqual(dialogOverflow.clientWidth + 1)
  })

  test('keeps fidelity pages inside the comparison viewports', async ({ page, browser }) => {
    test.setTimeout(60_000)
    const anonymous = await browser.newContext()
    const loginPage = await anonymous.newPage()
    await installOperatorApiMock(loginPage)

    for (const viewport of fidelityViewports) {
      await loginPage.setViewportSize(viewport)
      await loginPage.goto('/login')
      await expect(loginPage).toHaveURL(/\/login$/)
      const loginOverflow = await loginPage.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }))
      expect(loginOverflow.scrollWidth, `login at ${viewport.width}x${viewport.height}`).toBeLessThanOrEqual(
        loginOverflow.clientWidth + 1,
      )

      await page.setViewportSize(viewport)
      for (const route of fidelityAdminRoutes) {
        await page.goto(route)
        const overflow = await page.evaluate(() => ({
          clientWidth: document.documentElement.clientWidth,
          scrollWidth: document.documentElement.scrollWidth,
        }))
        expect(overflow.scrollWidth, `${route} at ${viewport.width}x${viewport.height}`).toBeLessThanOrEqual(
          overflow.clientWidth + 1,
        )
      }
    }

    await anonymous.close()
  })

  test('keeps the header and task tabs visible while scrolling long Settings content', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 720 })
    await page.goto('/admin/settings')
    const main = page.locator('main')
    const tabs = page.locator('.settings-tabs-shell')
    await expect(tabs).toBeVisible()
    const scroll = await main.evaluate((element) => {
      element.scrollTop = element.scrollHeight
      return {
        paddingTop: Number.parseFloat(getComputedStyle(element).paddingTop),
        scrollTop: element.scrollTop,
        windowScrollY: window.scrollY,
      }
    })

    const header = await page.locator('.app-header').boundingBox()
    const mainRegion = await main.boundingBox()
    const tabBar = await tabs.boundingBox()

    expect(scroll.scrollTop).toBeGreaterThan(0)
    expect(scroll.windowScrollY).toBe(0)
    expect(header).not.toBeNull()
    expect(mainRegion).not.toBeNull()
    expect(tabBar).not.toBeNull()
    expect(header!.y).toBeGreaterThanOrEqual(-1)
    expect(header!.y).toBeLessThanOrEqual(1)
    expect(tabBar!.y).toBeGreaterThanOrEqual(mainRegion!.y + scroll.paddingTop - 1)
    expect(tabBar!.y).toBeLessThanOrEqual(mainRegion!.y + scroll.paddingTop + 1)
  })

  test('keeps Settings save, failure, discard, mobile, and footer states reachable', async ({ page }) => {
    test.setTimeout(60_000)
    const writable = await installWritableSettingsMock(page)
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/settings')

    const siteName = page.getByTestId('settings-site-name')
    const dirtyBar = page.getByTestId('settings-dirty-bar')
    const discard = page.getByTestId('settings-discard-button')
    const floatingSave = page.getByTestId('settings-floating-save-button')
    const main = page.locator('main.app-main')
    await expect(siteName).toHaveValue('Sub2API')
    await expect(dirtyBar).toHaveCount(0)

    await siteName.fill('Gateway draft')
    await expect(dirtyBar).toBeVisible()
    await expect(dirtyBar).toContainText('Unsaved changes')
    await main.evaluate((element) => { element.scrollTop = element.scrollHeight / 2 })
    await expect(dirtyBar).toBeVisible()
    expect(await dirtyBar.evaluate((element) => getComputedStyle(element).position)).toBe('fixed')

    await page.setViewportSize({ width: 390, height: 844 })
    const mobileBar = await dirtyBar.boundingBox()
    expect(mobileBar).not.toBeNull()
    expect(mobileBar!.x).toBeGreaterThanOrEqual(0)
    expect(mobileBar!.x + mobileBar!.width).toBeLessThanOrEqual(390)
    expect(mobileBar!.y + mobileBar!.height).toBeLessThanOrEqual(844)
    await expect(discard).toBeVisible()
    await expect(floatingSave).toBeVisible()

    await discard.click()
    await expect(siteName).toHaveValue('Sub2API')
    await expect(dirtyBar).toHaveCount(0)

    await page.setViewportSize({ width: 1440, height: 900 })
    writable.failMain = true
    await siteName.fill('Gateway failed save')
    await floatingSave.click()
    await expect.poll(() => writable.mainSaves).toBe(1)
    await expect(dirtyBar).toBeVisible()
    await expect(floatingSave).toBeEnabled()
    await expect(siteName).toHaveValue('Gateway failed save')

    await discard.click()
    await expect(siteName).toHaveValue('Sub2API')
    writable.failMain = false
    writable.delay = 250
    await siteName.fill('Gateway saved')
    await floatingSave.click()
    await expect(floatingSave).toBeDisabled()
    await expect(floatingSave).toHaveAttribute('aria-busy', 'true')
    await expect.poll(() => writable.mainSaves).toBe(2)
    await expect.poll(() => writable.webSearchSaves).toBe(1)
    await expect(dirtyBar).toHaveCount(0)
    await expect(siteName).toHaveValue('Gateway saved')

    await page.emulateMedia({ reducedMotion: 'reduce' })
    await siteName.fill('Gateway reduced motion')
    await expect(dirtyBar).toBeVisible()
    expect(await dirtyBar.evaluate((element) => Number.parseFloat(getComputedStyle(element).transitionDuration)))
      .toBeLessThanOrEqual(0.001)
    await discard.click()
    await expect(siteName).toHaveValue('Gateway saved')

    const bottomActions = page.locator('.settings-bottom-actions')
    await bottomActions.scrollIntoViewIfNeeded()
    const spacing = await bottomActions.evaluate((element) => {
      const style = getComputedStyle(element)
      return {
        marginTop: Number.parseFloat(style.marginTop),
        paddingBottom: Number.parseFloat(style.paddingBottom),
      }
    })
    expect(spacing.marginTop).toBeGreaterThanOrEqual(32)
    expect(spacing.paddingBottom).toBeGreaterThanOrEqual(32)
    const bottomBox = await bottomActions.boundingBox()
    expect(bottomBox).not.toBeNull()
    expect(bottomBox!.y + bottomBox!.height).toBeLessThanOrEqual(900 + 1)
  })

  test('stacks Activity health and metrics at compact desktop width', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 900 })
    await page.goto('/admin/ops')

    const health = await page.getByTestId('ops-health-summary').boundingBox()
    const metrics = await page.getByTestId('ops-metric-summary').boundingBox()

    expect(health).not.toBeNull()
    expect(metrics).not.toBeNull()
    expect(metrics!.y).toBeGreaterThanOrEqual(health!.y + health!.height - 1)
  })

  test('traps and restores focus for an account dialog', async ({ page }) => {
    await page.goto('/admin/accounts')
    const trigger = page.getByRole('button', { name: 'Create Account', exact: true }).first()
    await trigger.click()

    const dialog = page.getByRole('dialog', { name: 'Create Account' })
    await expect(dialog).toBeVisible()
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)

    await page.keyboard.press('Tab')
    expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)

    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    await expect(trigger).toBeFocused()
  })
})

test('simple mode hides navigation for a restricted operator area', async ({ browser }) => {
  const context = await browser.newContext()
  const page = await context.newPage()
  await seedSession(page, 'admin', 'simple')
  await installOperatorApiMock(page, 'admin', 'simple')

  await page.goto('/admin/channels/pricing')
  await expect(page).toHaveURL(/\/admin\/channels\/pricing$/)
  await expect(primaryNavLink(page, '/admin/groups')).toHaveCount(0)
  await expect(page.locator('.operator-context-nav')).toHaveCount(0)

  await page.goto('/admin/dashboard')
  await expect(page.getByRole('button', { name: /Models & Routing/i })).toHaveCount(0)
  await context.close()
})

test('production guards remain authoritative for hidden or unauthorized links', async ({ browser }) => {
  const anonymous = await browser.newContext()
  const anonymousPage = await anonymous.newPage()
  await installOperatorApiMock(anonymousPage)
  await anonymousPage.goto('/admin/accounts')
  await expect(anonymousPage).toHaveURL(/\/login\?redirect=/)
  expect(new URL(anonymousPage.url()).searchParams.get('redirect')).toBe('/admin/accounts')
  await anonymous.close()

  const personal = await browser.newContext()
  const personalPage = await personal.newPage()
  await seedSession(personalPage, 'user')
  await installOperatorApiMock(personalPage, 'user')
  await personalPage.goto('/admin/accounts')
  await expect(personalPage).toHaveURL(/\/dashboard$/)
  await expect(personalPage.getByRole('button', { name: 'Ask Gateway' })).toHaveCount(0)
  await personal.close()
})
