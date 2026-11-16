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

// Charts re-render after a resize, so both boxes are measured together once layout settles.
const stackedBelow = (upper: Locator, lower: Locator) => async () => {
  const [upperBox, lowerBox] = await Promise.all([upper.boundingBox(), lower.boundingBox()])
  return upperBox !== null && lowerBox !== null && lowerBox.y >= upperBox.y + upperBox.height - 1
}

// Crossing the sidebar breakpoint animates the shell, so a card keeps moving after it has stacked.
const settledBox = (locator: Locator) => () => locator.evaluate((element) => new Promise<boolean>((resolve) => {
  const before = element.getBoundingClientRect()
  requestAnimationFrame(() => requestAnimationFrame(() => {
    const after = element.getBoundingClientRect()
    resolve(before.left === after.left && before.width === after.width)
  }))
}))

/** The SVG chart, its axis labels, and an open tooltip stay inside the chart's card. */
async function expectTrendChartInside(card: Locator, label: string) {
  const metrics = await card.evaluate((element) => {
    const bounds = (node: Element) => {
      const box = node.getBoundingClientRect()
      return { left: box.left, right: box.right, top: box.top, bottom: box.bottom }
    }
    const svg = element.querySelector('.trend-chart svg[role="img"]')!
    const tip = element.querySelector('.trend-tip')
    return {
      card: bounds(element),
      svg: bounds(svg),
      labels: [...svg.querySelectorAll('text')].map(bounds),
      tip: tip ? bounds(tip) : null,
    }
  })

  expect(metrics.svg.left, `${label} chart left`).toBeGreaterThanOrEqual(metrics.card.left - 1)
  expect(metrics.svg.right, `${label} chart right`).toBeLessThanOrEqual(metrics.card.right + 1)
  expect(metrics.labels.length, `${label} axis labels`).toBeGreaterThan(0)
  for (const [index, text] of metrics.labels.entries()) {
    expect(text.left, `${label} label ${index} left`).toBeGreaterThanOrEqual(metrics.card.left - 1)
    expect(text.right, `${label} label ${index} right`).toBeLessThanOrEqual(metrics.card.right + 1)
  }
  if (metrics.tip) {
    expect(metrics.tip.left, `${label} tooltip left`).toBeGreaterThanOrEqual(metrics.card.left - 1)
    expect(metrics.tip.right, `${label} tooltip right`).toBeLessThanOrEqual(metrics.card.right + 1)
    expect(metrics.tip.bottom, `${label} tooltip bottom`).toBeLessThanOrEqual(metrics.svg.bottom + 1)
  }
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
      subtleForeground: color('--operator-subtle-foreground'),
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

/** Account filter selects live in the toolbar Filters popover; opens it and returns the named trigger. */
async function openAccountFilters(page: Page) {
  await page.locator('.acct-toolbar .acct-filter-trigger').click()
  const filters = page.getByRole('dialog', { name: 'Filters' })
  await expect(filters).toBeVisible()
  return filters
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
    // Without a custom site logo the sidebar draws the built-in brand mark inline.
    const sidebarLogo = page.locator('.sidebar-logo')
    await expect(sidebarLogo).toHaveClass(/sidebar-logo-brand/)
    await expect(sidebarLogo.locator('svg')).toBeVisible()
    await expect(sidebarLogo.locator('svg')).toHaveAttribute('aria-hidden', 'true')
    await expect(sidebarLogo.locator('img')).toHaveCount(0)
    await expect(page.locator('.app-header').getByRole('heading', { level: 1, name: 'Overview' })).toBeVisible()
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
    await sidebar.locator('nav[aria-label="Main navigation"] a[href="/admin/accounts"]').click()
    await expect(page).toHaveURL(/\/admin\/accounts$/)
    await expect(sidebar).toHaveClass(/-translate-x-full/)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
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
    // The searchable Status filter is the toolbar's teleported select with a search field.
    await page.goto('/admin/accounts?view=technical')
    await expect(page.locator('html')).not.toHaveClass(/dark/)

    await (await openAccountFilters(page)).getByRole('button', { name: 'Status', exact: true }).click()
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

    await popup.locator('.select-search-input').fill('__no_status_match__')
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

    // Accounts selects sit in the Filters popover; Activity keeps its Model quick select in the toolbar.
    const openAccountSelect = async (name: string) => {
      const filters = await openAccountFilters(page)
      await openPopupSelect(page, filters.getByRole('button', { name, exact: true }))
      await filters.getByRole('button', { name: 'Close', exact: true }).click()
      await expect(filters).toHaveCount(0)
    }
    const openUsageModelSelect = () => openPopupSelect(
      page,
      page.locator('.usage-records-toolbar').getByRole('button', { name: 'Model', exact: true }),
    )
    // Models & Routing has no toolbar select; its View options menu is the toolbar popup.
    const openGroupsViewMenu = async () => {
      await expect(page.locator('[data-testid^="group-row-"]').first()).toBeVisible()
      const trigger = page.getByTestId('group-view-menu')
      await trigger.click()
      const menu = page.getByRole('menu', { name: 'View options' })
      await expectPopupMenu(page, menu, menu.getByRole('menuitemcheckbox').first())
      await trigger.click()
      await expect(menu).toHaveCount(0)
    }

    await page.goto('/admin/accounts')
    await expect(page.locator('html')).not.toHaveClass(/dark/)
    await expect(page.locator('aside.sidebar')).toBeVisible()
    await openAccountSelect('Type')

    await navigate('/admin/usage')
    await openUsageModelSelect()

    await navigate('/admin/accounts')
    await openAccountSelect('Type')
    const moreActionsButton = page.getByRole('button', { name: 'More Actions' })
    await moreActionsButton.click()
    const moreActionsMenu = page.locator('.account-tools-menu')
    await expectPopupMenu(page, moreActionsMenu, moreActionsMenu.locator('.operator-menu-item').first())
    await moreActionsButton.click()
    await expect(moreActionsMenu).toHaveCount(0)
    await openAccountSelect('Warm-up')

    await navigate('/admin/stats')
    await openLocaleMenu()
    await navigate('/admin/groups')
    await openGroupsViewMenu()
    await navigate('/admin/usage')
    await openUsageModelSelect()
    await navigate('/admin/settings')
    await page.getByRole('tab', { name: 'Gateway' }).click()
    await openPopupSelect(page, page.locator('.select-trigger:visible').first())
    await navigate('/admin/accounts')
    await openAccountSelect('Type')

    for (let iteration = 0; iteration < 3; iteration += 1) {
      await navigate('/admin/groups')
      await openGroupsViewMenu()
      await navigate('/admin/accounts')
      await openAccountSelect('Type')
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
    // Capacity rows open their details in a drawer on demand, so none is open after the switch.
    const capacityRows = page.getByTestId('account-capacity-row')
    const accountDrawer = page.locator('.ui-drawer')
    await expect(capacityRows).toHaveCount(6)
    await expect(accountDrawer).toHaveCount(0)

    await capacityTab.focus()
    await page.keyboard.press('ArrowRight')
    await expect(technicalTab).toBeFocused()
    await expect(technicalTab).toHaveAttribute('aria-selected', 'true')
    await expect(capacityPanel).toBeHidden()
    await expect(technicalPanel).toBeVisible()

    const technicalStatus = (await openAccountFilters(page)).getByRole('button', { name: 'Status', exact: true })
    await technicalStatus.click()
    await page.getByRole('option', { name: 'Active', exact: true }).click()
    await expect(technicalStatus).toContainText('Active')

    await technicalTab.focus()
    await page.keyboard.press('ArrowLeft')
    await expect(capacityTab).toBeFocused()
    await expect(capacityPanel).toBeVisible()
    await expect(technicalPanel).toBeHidden()
    await expect(capacityRows).toHaveCount(6)
    await expect(accountDrawer).toHaveCount(0)
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
    const manualRefresh = page.getByRole('toolbar', { name: 'Account controls' }).getByRole('button', { name: 'Refresh', exact: true })
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
    // The joined auto-refresh button shows the bare countdown and names the interval for assistive tech.
    await expect(autoRefresh).toHaveAccessibleName('Auto refresh: 5s')
    await expect(autoRefresh).toContainText('5s')

    await page.clock.runFor(5_000)
    await expect.poll(() => state.tableStatuses).toEqual([200])
    await expect(autoRefresh).toHaveAccessibleName('Auto refresh: 5s')

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
      // Provider headers appear once the table is grouped; the choice persists across reloads.
      const groupByProvider = page.getByRole('button', { name: 'Group by provider' })
      if (viewport.width === 1440) await groupByProvider.click()
      await expect(groupByProvider).toHaveAttribute('aria-pressed', 'true')
      const providerHeader = page.locator('.fleet-group-toggle').first()
      await expect(providerHeader).toBeVisible()
      const providerStyle = await providerHeader.evaluate((element) => {
        const style = getComputedStyle(element)
        return {
          radius: Number.parseFloat(style.borderRadius),
          boxShadow: style.boxShadow,
          borderLeftWidth: style.borderLeftWidth,
          borderRightWidth: style.borderRightWidth,
        }
      })
      expect(providerStyle.radius).toBeGreaterThan(0)
      expect(providerStyle.radius).toBeLessThanOrEqual(8)
      expect(providerStyle.boxShadow).toBe('none')
      expect(providerStyle.borderLeftWidth).toBe(providerStyle.borderRightWidth)

      // The row shows the limiting model window; every model window is listed in the account drawer.
      const antigravityRow = capacityRows.filter({ hasText: 'Antigravity Pro' })
      await expect(antigravityRow.getByRole('img', { name: 'claude-opus-4-1: 0%' })).toBeVisible()
      if (viewport.width === 1440) {
        await antigravityRow.click()
        const drawer = page.getByRole('dialog', { name: 'Antigravity Pro' })
        const windows = drawer.getByTestId('account-drawer-windows')
        await expect(windows).toContainText('hidden-model-search-target')
        await expect(windows.locator('li')).toHaveCount(19)
        expect(await drawer.evaluate((element) => element.scrollWidth <= element.clientWidth + 1)).toBe(true)
        await page.keyboard.press('Escape')
        await expect(drawer).toHaveCount(0)
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
      await page.goto('/admin/stats?view=capacity')
      await expect(page.getByTestId('stats-tab-capacity')).toHaveAttribute('aria-pressed', 'true')

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

  test('opens Stats capacity from Overview and separates short and long capacity', async ({ page }) => {
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

    // Overview keeps one ring per provider and links to the Stats capacity tab.
    const capacityCard = page.locator('section[aria-labelledby="overview-capacity"]')
    await expect(capacityCard.getByRole('heading', { name: 'Capacity left' })).toBeVisible()
    await expect(capacityCard.locator('.overview-gauges > li')).toHaveCount(4)
    for (const provider of ['OpenAI', 'Anthropic', 'Gemini', 'Antigravity']) {
      await expect(capacityCard.getByRole('img', { name: new RegExp(`^${provider}: (?:\\d+%|–)$`) })).toBeVisible()
    }
    await expect(page.getByTestId('account-pool-capacity')).toHaveCount(0)
    const capacityLink = capacityCard.getByRole('link', { name: 'Stats', exact: true })
    await expect(capacityLink).toHaveAttribute('href', '/admin/stats?view=capacity')

    await Promise.all([
      page.waitForURL(/\/admin\/stats\?view=capacity$/),
      capacityLink.click(),
    ])

    await expect(page.getByTestId('stats-tab-capacity')).toHaveAttribute('aria-pressed', 'true')
    await expect(page.getByTestId('stats-tab-usage')).toHaveAttribute('aria-pressed', 'false')
    await expect(page.locator('.stats-capacity-section')).toBeVisible()
    // Usage controls and tiles belong to the Usage tab only.
    await expect(page.getByTestId('stats-kpis')).toHaveCount(0)
    await expect(page.getByTestId('stats-range-d7')).toHaveCount(0)
    await expect(page.getByTestId('stats-traffic')).toHaveCount(0)

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
    await expect(page.locator('main select')).toHaveCount(0)
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

    for (const viewport of [{ width: 1024, height: 768 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport)
      // Charts re-render after the resize, so measure both boxes in one frame once layout settles.
      await expect.poll(stackedBelow(shortTerm, longTerm)).toBe(true)
      const overflow = await page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }))
      expect(overflow.scrollWidth, `${viewport.width}px page overflow`).toBeLessThanOrEqual(overflow.clientWidth + 1)
    }

    // The tabs swap the whole view in place and keep the choice in the URL.
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.getByTestId('stats-tab-usage').click()
    await expect(page).toHaveURL(/\/admin\/stats$/)
    await expect(page.getByTestId('stats-tab-usage')).toHaveAttribute('aria-pressed', 'true')
    await expect(page.locator('.stats-capacity-section')).toHaveCount(0)
    await expect(page.getByTestId('stats-kpis')).toBeVisible()
    await page.getByTestId('stats-tab-capacity').click()
    await expect(page).toHaveURL(/\/admin\/stats\?view=capacity$/)
    await expect(page.getByTestId('stats-kpis')).toHaveCount(0)
    await expect(page.getByTestId('provider-capacity-openai')).toBeVisible()
    await expect(inspector.getByTestId('stats-selected-account-detail')).toHaveCount(1)
  })

  test('bounds large-pool account and Antigravity model diagnostics', async ({ page }) => {
    test.setTimeout(60_000)
    const fixture = await installManyAccountMock(page)
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/stats?view=capacity')

    await expect(page.getByTestId('stats-tab-capacity')).toHaveAttribute('aria-pressed', 'true')
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
    await expect(page.locator('main select')).toHaveCount(0)
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
        nativeSelects: document.querySelectorAll('main select').length,
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

    const attention = page.locator('section[aria-labelledby="overview-attention"]')
    await expect(attention.getByRole('heading', { name: /^Needs attention/ })).toBeVisible()
    const states = {
      active: page.getByTestId('overview-state-active'),
      limited: page.getByTestId('overview-state-limited'),
      error: page.getByTestId('overview-state-error'),
    }
    await expect(states.active).toHaveText('2 Active')
    await expect(states.limited).toHaveText('2 Limited')
    await expect(states.error).toHaveText('1 Error')
    for (const [status, link] of Object.entries(states)) {
      await expect(link).toHaveAttribute('href', `/admin/accounts?operator_status=${status}`)
    }
    // A disabled account is an operator choice, so Overview neither counts nor lists it.
    await expect(page.getByTestId('overview-state-disabled')).toHaveCount(0)
    await expect(attention).not.toContainText('Codex Standby')

    // Errors come first, then limits; each problem opens its own account in the Accounts drawer.
    const accountId = (name: string) => operatorFixtureAccounts.find((account) => account.name === name)!.id
    const problems = attention.locator('.overview-attention-item')
    await expect(problems).toHaveCount(3)
    await expect(problems.first()).toHaveAccessibleName('Open Gemini Recovery in Accounts')
    await expect(problems.first()).toHaveAttribute('href', `/admin/accounts?account=${accountId('Gemini Recovery')}`)
    await expect(problems.first()).toContainText('Fixture: reauthorization required')
    for (const name of ['Antigravity Pro', 'Gemini Quota Limited']) {
      const problem = problems.filter({ hasText: name })
      await expect(problem).toHaveCount(1)
      await expect(problem).toHaveAttribute('href', `/admin/accounts?account=${accountId(name)}`)
      await expect(problem).toContainText('Limited')
    }

    await states.limited.click()
    await expect(page).toHaveURL(/\/admin\/accounts\?operator_status=limited$/)
    const limitedRows = page.locator('[data-testid="account-capacity-row"][data-status="limited"]')
    await expect(limitedRows).toHaveCount(2)
    await expect(limitedRows.filter({ hasText: 'Antigravity Pro' })).toHaveCount(1)
    await expect(limitedRows.filter({ hasText: 'Gemini Quota Limited' })).toHaveCount(1)

    // Each row opens its details drawer from the keyboard.
    const cooldownRow = limitedRows.filter({ hasText: 'Antigravity Pro' })
    await expect(cooldownRow).toHaveAccessibleName('Open Antigravity Pro')
    await expect(cooldownRow.locator('.badge')).toHaveAttribute('title', /^Provider cooldown · until /)
    await cooldownRow.focus()
    await page.keyboard.press('Enter')
    const cooldownDrawer = page.getByRole('dialog', { name: 'Antigravity Pro' })
    await expect(cooldownDrawer).toBeVisible()
    await expect(cooldownDrawer.getByTestId('account-drawer-status')).toHaveText('Limited')
    const cooldownStatus = cooldownDrawer.locator('.acct-notice')
    await expect(cooldownStatus).toContainText('Provider cooldown')
    await expect(cooldownStatus.locator('small')).toContainText('until')
    await page.keyboard.press('Escape')
    await expect(cooldownDrawer).toHaveCount(0)
    await expect(cooldownRow).toBeFocused()

    await page.goto('/admin/dashboard')
    const errorProblem = page.getByRole('link', { name: 'Open Gemini Recovery in Accounts' })
    await errorProblem.focus()
    await page.keyboard.press('Enter')
    await expect(page).toHaveURL(new RegExp(`/admin/accounts\\?account=${accountId('Gemini Recovery')}$`))
    const errorDrawer = page.getByRole('dialog', { name: 'Gemini Recovery' })
    await expect(errorDrawer).toBeVisible()
    await expect(errorDrawer.getByTestId('account-drawer-status')).toHaveText('Error')
    await expect(errorDrawer).toContainText('Fixture: reauthorization required')

    await page.goto('/admin/accounts?operator_status=disabled')
    const disabledRows = page.locator('[data-testid="account-capacity-row"][data-status="disabled"]')
    await expect(disabledRows).toHaveCount(1)
    await expect(disabledRows).toContainText('Codex Standby')
  })

  // Overview and Stats bucket the fixture trend by local hours and days, so these
  // checks pin the zone. The fixture traffic runs 20:00 to 01:00 UTC across 29-30 August.
  test.describe('charts on a UTC calendar', () => {
    test.use({ timezoneId: 'UTC' })

    test('summarizes today, hourly traffic, capacity, and recent requests on Overview', async ({ page }) => {
      test.setTimeout(60_000)
      await page.setViewportSize({ width: 1440, height: 900 })
      await page.goto('/admin/dashboard')

      await expect(page.locator('.operator-fleet-summary, [data-testid="account-pool-capacity"]')).toHaveCount(0)
      await expect(page.getByText('Recent Usage (Top 12)')).toHaveCount(0)

      const status = page.locator('section[aria-labelledby="overview-status"]')
      await expect(status.getByRole('heading', { name: 'Serving traffic' })).toBeVisible()
      const today = (label: string) => status.locator('.overview-today > div').filter({ has: page.locator('dt', { hasText: label }) }).locator('dd')
      await expect(today('Requests today')).toHaveText('4,286')
      await expect(today('Cost')).toHaveText('$19.43')
      await expect(today('Avg response')).toHaveText('1.28s')
      await expect(status.locator('.overview-live b')).toHaveText('42')
      await expect(status.getByRole('link', { name: 'Stats', exact: true })).toHaveAttribute('href', '/admin/stats')

      // The hourly chart is read from the keyboard: the last slot first, then earlier hours.
      const hourly = status.getByRole('img', { name: 'Requests per hour over the last 24 hours. Use the arrow keys to read values' })
      const hourlyTip = status.locator('.trend-tip')
      await hourly.focus()
      await page.keyboard.press('ArrowLeft')
      await expect(hourlyTip.locator('.trend-tip-title')).toHaveText(/^Sun,? 1\sAM$/)
      await expect(hourlyTip.locator('.trend-tip-row')).toHaveText(['Requests946'])
      await page.keyboard.press('ArrowLeft')
      await page.keyboard.press('ArrowLeft')
      await expect(hourlyTip.locator('.trend-tip-title')).toHaveText(/^Sat,? 11\sPM$/)
      await expect(hourlyTip.locator('.trend-tip-row b')).toHaveText('792')
      await page.keyboard.press('ArrowRight')
      await expect(hourlyTip.locator('.trend-tip-row b')).toHaveText('946')
      await page.keyboard.press('Escape')
      await expect(hourlyTip).toHaveCount(0)

      const attention = page.locator('section[aria-labelledby="overview-attention"]')
      const capacity = page.locator('section[aria-labelledby="overview-capacity"]')
      await expect(capacity.locator('.overview-gauges > li')).toHaveCount(4)
      await expect(capacity.getByRole('img', { name: 'OpenAI: 52%', exact: true })).toBeVisible()
      await expect(capacity.locator('.overview-gauges > li').first()).toContainText('2 accounts')

      const recent = page.locator('section[aria-labelledby="overview-recent"]')
      const recentRows = recent.locator('.overview-table tbody tr')
      await expect(recent.locator('.overview-table thead th')).toHaveText(['Model', 'Account', 'Tokens', 'Duration', 'Cost', 'When'])
      await expect(recentRows).toHaveCount(4)
      await expect(recentRows.first().locator('code')).toHaveText('gpt-5.2-codex')
      await expect(recentRows.first().locator('td')).toHaveText(['gpt-5.2-codex', '–', '27,330', '1.48s', '$0.0440', '0s'])
      await expect(recentRows.nth(3).locator('td').nth(3)).toHaveText('742ms')
      await expect(recent.getByRole('link', { name: 'All activity' })).toHaveAttribute('href', '/admin/usage')

      const [attentionBox, capacityBox] = await Promise.all([attention.boundingBox(), capacity.boundingBox()])
      expect(attentionBox).not.toBeNull()
      expect(capacityBox).not.toBeNull()
      expect(capacityBox!.x).toBeGreaterThanOrEqual(attentionBox!.x + attentionBox!.width - 1)
      await expectTrendChartInside(status, '1440px hourly chart')

      for (const viewport of [{ width: 1024, height: 768 }, { width: 390, height: 844 }]) {
        await page.setViewportSize(viewport)
        await expect.poll(stackedBelow(attention, capacity)).toBe(true)
        await expect.poll(settledBox(status)).toBe(true)
        await hourly.focus()
        await page.keyboard.press('ArrowLeft')
        await expect(hourlyTip.locator('.trend-tip-row b')).toHaveText('946')
        await expectTrendChartInside(status, `${viewport.width}px hourly chart`)
        await page.keyboard.press('Escape')
        // The narrow table drops the account and cost columns instead of overflowing.
        const narrow = viewport.width < 640
        for (const column of [1, 4]) {
          if (narrow) await expect(recent.locator('thead th').nth(column)).toBeHidden()
          else await expect(recent.locator('thead th').nth(column)).toBeVisible()
        }
        await expect(recentRows).toHaveCount(4)
        const overflow = await page.evaluate(() => ({
          clientWidth: document.documentElement.clientWidth,
          scrollWidth: document.documentElement.scrollWidth,
        }))
        expect(overflow.scrollWidth, `${viewport.width}px page overflow`).toBeLessThanOrEqual(overflow.clientWidth + 1)
      }
    })

    test('reads Stats usage from the fixture totals by keyboard, pointer, and table', async ({ page }) => {
      test.setTimeout(90_000)
      await page.setViewportSize({ width: 1440, height: 900 })
      const keyTrendRequests: string[] = []
      page.on('request', (request) => {
        if (new URL(request.url()).pathname === '/api/v1/admin/dashboard/api-keys-trend') keyTrendRequests.push(request.url())
      })
      await page.goto('/admin/stats')

      await expect(page.getByTestId('stats-tab-usage')).toHaveAttribute('aria-pressed', 'true')
      await expect(page.getByTestId('stats-tab-capacity')).toHaveAttribute('aria-pressed', 'false')
      await expect(page.getByTestId('stats-range-d7')).toHaveAttribute('aria-pressed', 'true')
      await expect(page.locator('.stats-capacity-section')).toHaveCount(0)
      await expect(page.locator('[data-testid="stats-request-trend"], [data-testid="stats-trend-grid"]')).toHaveCount(0)

      // Seven days of six-hour slots: 25 slots up to the fixture's 01:45 UTC snapshot.
      const kpi = (label: string) => page.getByTestId('stats-kpis').locator('.viz-kpi')
        .filter({ has: page.locator('dt', { hasText: label }) })
      await expect(kpi('Requests').locator('.viz-kpi-value')).toHaveText('4,286')
      await expect(kpi('Tokens').locator('.viz-kpi-value')).toHaveText('6.4M')
      await expect(kpi('Cost').locator('.viz-kpi-value')).toHaveText('$19.43')
      await expect(kpi('Avg response').locator('.viz-kpi-value')).toHaveText('1.28s')
      // The previous week has no traffic, so there is no baseline to compare against.
      await expect(kpi('Requests').locator('.viz-delta')).toHaveText('')

      const traffic = page.getByTestId('stats-traffic')
      const trafficFoot = traffic.locator('.viz-card-foot')
      const trafficTip = traffic.locator('.trend-tip')
      const trafficChart = traffic.getByRole('img', { name: 'Requests · 7 days. Use the arrow keys to read values' })
      await expect(trafficChart).toBeVisible()
      await expect(page.getByRole('switch', { name: 'Compare' })).toHaveAttribute('aria-checked', 'true')
      await expect(traffic.locator('.viz-key')).toHaveText('Previous')
      await expect(trafficFoot).toContainText('Total 4,286')
      await expect(trafficFoot).toContainText(/Peak 2,394 · Sat, Aug 29, 6\sPM/)

      await trafficChart.focus()
      await page.keyboard.press('ArrowLeft')
      await expect(trafficTip.locator('.trend-tip-title')).toHaveText(/^Sun, Aug 30, 12\sAM$/)
      await expect(trafficTip.locator('.trend-tip-row')).toHaveText(['This period1,892', 'Previous0'])
      await page.keyboard.press('ArrowLeft')
      await expect(trafficTip.locator('.trend-tip-row').first()).toHaveText('This period2,394')
      await page.keyboard.press('ArrowRight')
      await expect(trafficTip.locator('.trend-tip-row').first()).toHaveText('This period1,892')
      await page.keyboard.press('Escape')
      await expect(trafficTip).toHaveCount(0)

      const chartBox = await trafficChart.boundingBox()
      expect(chartBox).not.toBeNull()
      await trafficChart.hover({ position: { x: chartBox!.width - 14, y: chartBox!.height / 2 } })
      await expect(trafficTip.locator('.trend-tip-row').first()).toHaveText('This period1,892')
      await expectTrendChartInside(traffic, '1440px traffic chart')
      await page.mouse.move(0, 0)
      await expect(trafficTip).toHaveCount(0)

      await page.getByTestId('stats-metric-tokens').click()
      await expect(page.getByTestId('stats-metric-tokens')).toHaveAttribute('aria-pressed', 'true')
      await expect(trafficFoot).toContainText('Total 6.4M')
      await page.getByTestId('stats-metric-cost').click()
      await expect(traffic.getByRole('img', { name: 'Cost · 7 days. Use the arrow keys to read values' })).toBeVisible()
      await expect(trafficFoot).toContainText('Total $19.43')
      await expect(trafficFoot).toContainText(/Peak \$11\.01 · Sat, Aug 29, 6\sPM/)

      await traffic.getByRole('button', { name: 'Show as table' }).click()
      const trafficTable = traffic.locator('.stats-table')
      await expect(traffic.locator('.trend-chart')).toHaveCount(0)
      await expect(trafficTable.locator('thead th')).toHaveText(['Period', 'Cost', 'Previous'])
      await expect(trafficTable.locator('tbody tr')).toHaveCount(25)
      await expect(trafficTable.locator('tbody tr').filter({ hasText: /Sat, Aug 29, 6\sPM/ }).locator('td'))
        .toHaveText([/Sat, Aug 29, 6\sPM/, '$11.01', '$0.00'])
      await expect(trafficTable.locator('tbody tr').last().locator('td').nth(1)).toHaveText('$8.42')
      await traffic.getByRole('button', { name: 'Show chart' }).click()
      await expect(trafficTable).toHaveCount(0)

      await page.getByTestId('stats-metric-requests').click()
      await page.getByRole('switch', { name: 'Compare' }).click()
      await expect(traffic.locator('.viz-key')).toHaveCount(0)
      await trafficChart.focus()
      await page.keyboard.press('ArrowLeft')
      await expect(trafficTip.locator('.trend-tip-row')).toHaveText(['This period1,892'])
      await page.keyboard.press('Escape')
      await page.getByRole('switch', { name: 'Compare' }).click()
      await expect(traffic.locator('.viz-key')).toHaveText('Previous')

      const share = page.getByTestId('stats-share')
      await expect(share.getByRole('heading', { name: 'Providers' })).toBeVisible()
      await expect(share.locator('.viz-row')).toHaveText(['OpenAI50%1,426', 'Anthropic33%924', 'Antigravity17%488'])

      const top = page.getByTestId('stats-top')
      await expect(top.locator('.viz-hbar')).toHaveText(['gpt-5.2-codex1,84248%', 'claude-sonnet-4-51,31634%', 'gemini-2.5-pro71418%'])
      // API-key totals load only when their tab opens.
      expect(keyTrendRequests).toEqual([])
      await Promise.all([
        page.waitForRequest((request) => new URL(request.url()).pathname === '/api/v1/admin/dashboard/api-keys-trend'),
        page.getByTestId('stats-top-keys').click(),
      ])
      await expect(page.getByTestId('stats-top-keys')).toHaveAttribute('aria-pressed', 'true')
      await expect(top.locator('.viz-hbar')).toHaveText(['review-console2,10649%', 'coding-agents1,38432%', 'ci-pipeline79619%'])

      const performanceCard = page.getByTestId('stats-performance')
      const percentile = (label: string) => performanceCard.locator('.viz-percentiles > div').filter({ hasText: label }).locator('b')
      await expect(page.getByTestId('stats-performance-latency')).toHaveAttribute('aria-pressed', 'true')
      await expect(percentile('p50')).toHaveText('842ms')
      await expect(percentile('p90')).toHaveText('1.92s')
      await expect(percentile('p95')).toHaveText('2.49s')
      await expect(percentile('p99')).toHaveText('4.81s')
      await expect(performanceCard.locator('.viz-card-foot')).toHaveText('First token p50 248ms')
      const latencyChart = performanceCard.getByRole('img', { name: 'Response time distribution. Use the arrow keys to read values' })
      await latencyChart.focus()
      await page.keyboard.press('ArrowRight')
      await expect(performanceCard.locator('.trend-tip')).toHaveText('<500msRequests614')
      await page.keyboard.press('ArrowRight')
      await expect(performanceCard.locator('.trend-tip')).toHaveText('500ms-1sRequests1,182')
      await page.keyboard.press('Escape')
      await expect(performanceCard.locator('.trend-tip')).toHaveCount(0)

      await page.getByTestId('stats-performance-errors').click()
      await expect(performanceCard.locator('.viz-big b')).toHaveText('0.85%')
      await expect(performanceCard.locator('.viz-big span')).toHaveText('24 of 2,838 failed')
      await expect(performanceCard.locator('.viz-hbar')).toHaveText(['HTTP 42911', 'HTTP 5028', 'HTTP 4005'])
      await page.getByTestId('stats-performance-tokens').click()
      await expect(performanceCard.locator('.viz-big')).toContainText('11%')
      await expect(performanceCard.locator('.viz-row')).toHaveText(['Cache read576K9%', 'Input4.61M72%', 'Output1.12M18%', 'Cache write84K1%'])

      const busy = page.getByTestId('stats-busy')
      await expect(busy.getByRole('img', { name: /Busiest: Sun,? 12\sAM\.$/ })).toBeVisible()
      await expect(busy.locator('.heat-cell')).toHaveCount(7 * 24)
      // Hours after the snapshot are empty, not zero.
      await expect(busy.locator('.heat-cell.is-empty')).toHaveCount(22)
      await expect(busy.locator('.heat-cell[title$=" · 946"]')).toHaveCount(2)
      await expect(busy.locator('.heat-cell[title$=" · 792"]')).toHaveAttribute('title', /^Sat, Aug 29 · 11\sPM · 792$/)

      // Today: two hourly slots so far.
      await page.getByTestId('stats-range-today').click()
      await expect(page.getByTestId('stats-range-today')).toHaveAttribute('aria-pressed', 'true')
      await expect(kpi('Requests').locator('.viz-kpi-value')).toHaveText('1,892')
      await expect(trafficFoot).toContainText('Total 1,892')
      const todayChart = traffic.getByRole('img', { name: 'Requests · Today. Use the arrow keys to read values' })
      await todayChart.focus()
      await page.keyboard.press('ArrowLeft')
      await expect(trafficTip.locator('.trend-tip-title')).toHaveText(/^Sun, Aug 30, 1\sAM$/)
      await expect(trafficTip.locator('.trend-tip-row').first()).toHaveText('This period946')
      await page.keyboard.press('Escape')

      const [trafficBox, shareBox] = await Promise.all([traffic.boundingBox(), share.boundingBox()])
      expect(trafficBox).not.toBeNull()
      expect(shareBox).not.toBeNull()
      expect(shareBox!.x).toBeGreaterThanOrEqual(trafficBox!.x + trafficBox!.width - 1)

      for (const viewport of [{ width: 1024, height: 768 }, { width: 390, height: 844 }]) {
        await page.setViewportSize(viewport)
        await expect.poll(stackedBelow(traffic, share)).toBe(true)
        await expect.poll(stackedBelow(page.getByTestId('stats-performance'), busy)).toBe(true)
        await todayChart.focus()
        await page.keyboard.press('ArrowLeft')
        await expect(trafficTip.locator('.trend-tip-row').first()).toHaveText('This period946')
        await expectTrendChartInside(traffic, `${viewport.width}px traffic chart`)
        await page.keyboard.press('Escape')
        const overflow = await page.evaluate(() => ({
          clientWidth: document.documentElement.clientWidth,
          scrollWidth: document.documentElement.scrollWidth,
        }))
        expect(overflow.scrollWidth, `${viewport.width}px page overflow`).toBeLessThanOrEqual(overflow.clientWidth + 1)
      }
    })
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

      for (const name of ['Test Connection', 'Edit', 'More actions']) {
        const action = row.getByRole('button', { name: new RegExp(`^${name} US West relay$`) })
        await expect(action).toBeVisible()
        await expect(action).toHaveText('')
      }
      // Less frequent row actions moved into the row's overflow menu.
      const more = row.getByRole('button', { name: 'More actions US West relay' })
      await more.click()
      const rowMenu = page.getByRole('menu', { name: 'More actions US West relay' })
      for (const name of ['Quality Check', 'Delete']) {
        await expect(rowMenu.getByRole('menuitem', { name })).toBeVisible()
      }
      await page.keyboard.press('Escape')
      await expect(rowMenu).toHaveCount(0)
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
    // The table is flat until the operator groups it by provider.
    await expect(page.getByTestId('provider-toggle-openai')).toHaveCount(0)
    await page.getByRole('button', { name: 'Group by provider' }).click()

    const toggle = page.getByTestId('provider-toggle-openai')
    const openAIRows = page.getByTestId('account-capacity-row').filter({ hasText: /OpenAI ·/ })
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    await expect(openAIRows).toHaveCount(2)
    await expect(toggle.getByRole('img', { name: 'OpenAI: 52%', exact: true })).toBeVisible()
    await expect(toggle).toContainText(/OpenAI\s*2\s*·\s*1 with quota/)
    await toggle.focus()
    await page.keyboard.press('Enter')
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await expect(openAIRows).toHaveCount(0)

    await page.reload()
    await expect(page.getByTestId('provider-toggle-openai')).toHaveAttribute('aria-expanded', 'false')
    await expect(openAIRows).toHaveCount(0)
    await page.getByTestId('provider-toggle-openai').focus()
    await page.keyboard.press('Space')
    await expect(page.getByTestId('provider-toggle-openai')).toHaveAttribute('aria-expanded', 'true')
    await expect(openAIRows).toHaveCount(2)
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

    // The searchable Status filter in the toolbar Filters popover.
    const platformSelect = (await openAccountFilters(page)).getByRole('button', { name: 'Status', exact: true })
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
    // Menu section labels share the redesigned overflow menus' subtle label token.
    expect(await moreActionsMenu.locator('.account-tools-menu-label').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(tokens.subtleForeground)
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
    await expect(page.getByTestId('group-row-11')).toBeVisible()
    expect(await page.locator('.groups-table tbody > tr:first-child > td').first().evaluate(
      (element) => getComputedStyle(element).borderBottomColor,
    )).toBe(tokens.borderSubtle)
    // Column settings live in View options as checkable menu items.
    await page.getByTestId('group-view-menu').click()
    const groupsMenu = page.getByRole('menu', { name: 'View options' })
    await expectPopupMenu(page, groupsMenu, groupsMenu.locator('[aria-checked="true"]').first(), 'checked')
    const typeColumn = groupsMenu.getByTestId('group-column-is_exclusive')
    await expect(typeColumn).toHaveAttribute('aria-checked', 'false')
    await typeColumn.click()
    await expect(typeColumn).toHaveAttribute('aria-checked', 'true')
    await page.keyboard.press('Escape')
    await expect(groupsMenu).toHaveCount(0)
    const neutralBadge = page.locator('.groups-table td[data-col="is_exclusive"] .badge-gray').first()
    expect(await neutralBadge.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, color: style.color }
    })).toEqual({ background: tokens.muted, color: tokens.mutedForeground })

    await page.goto('/admin/settings')
    // The section nav is no muted box: below 1100px it is a frosted page-colored sticky row,
    // and a sliding card-colored pill sits under the active tab.
    const settingsIndicator = page.locator('.settings-tabs .settings-tab-indicator')
    await expect(settingsIndicator).toHaveClass(/settings-tab-indicator-ready/)
    const settingsTheme = await page.locator('.settings-tabs-shell').evaluate((element) => {
      const active = element.querySelector('.settings-tab-active') as HTMLElement
      const inactive = element.querySelector('.settings-tab:not(.settings-tab-active)') as HTMLElement
      const pill = element.querySelector('.settings-tab-indicator') as HTMLElement
      const activeBox = active.getBoundingClientRect()
      const pillBox = pill.getBoundingClientRect()
      const probe = document.createElement('span')
      probe.style.backgroundColor = 'color-mix(in srgb, var(--operator-background) 88%, transparent)'
      element.append(probe)
      const frosted = getComputedStyle(probe).backgroundColor
      probe.remove()
      return {
        frostedBackground: getComputedStyle(element).backgroundColor === frosted,
        pillBackground: getComputedStyle(pill).backgroundColor,
        activeColor: getComputedStyle(active).color,
        inactiveColor: getComputedStyle(inactive).color,
        pillCoversActive: Math.abs(pillBox.left - activeBox.left) <= 1
          && Math.abs(pillBox.top - activeBox.top) <= 1
          && Math.abs(pillBox.width - activeBox.width) <= 1
          && Math.abs(pillBox.height - activeBox.height) <= 1,
      }
    })
    expect(settingsTheme).toEqual({
      frostedBackground: true,
      pillBackground: tokens.card,
      activeColor: tokens.foreground,
      inactiveColor: tokens.mutedForeground,
      pillCoversActive: true,
    })
    await page.getByRole('tab', { name: 'Gateway' }).click()
    // Open from the keyboard: ArrowDown opens on the selected option, the next ArrowDown moves past it.
    await page.locator('.select-trigger:visible').first().focus()
    await page.keyboard.press('ArrowDown')
    await expect(page.locator('.select-dropdown-portal .select-option-selected.select-option-focused')).toBeVisible()
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

    const createTrigger = page.getByRole('toolbar', { name: 'Account controls' }).getByRole('button', { name: 'Add', exact: true })
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

    // The bulk bar appears once a row is selected; its Edit opens Bulk Edit.
    await page.locator('.operator-account-table tbody input[type="checkbox"]').first().check()
    await page.getByTestId('account-bulk-bar').getByRole('button', { name: 'Edit', exact: true }).click()
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
    // Pages sit in an inset app frame, so the header pins to the frame's inner top edge.
    const frameTop = await page.locator('.app-frame').evaluate((element) => (
      element.getBoundingClientRect().top + Number.parseFloat(getComputedStyle(element).borderTopWidth)
    ))

    expect(scroll.scrollTop).toBeGreaterThan(0)
    expect(scroll.windowScrollY).toBe(0)
    expect(header).not.toBeNull()
    expect(mainRegion).not.toBeNull()
    expect(tabBar).not.toBeNull()
    expect(frameTop).toBeLessThanOrEqual(10)
    expect(header!.y).toBeGreaterThanOrEqual(frameTop - 1)
    expect(header!.y).toBeLessThanOrEqual(frameTop + 1)
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
    const trigger = page.getByRole('toolbar', { name: 'Account controls' }).getByRole('button', { name: 'Add', exact: true })
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
  await personal.close()
})
