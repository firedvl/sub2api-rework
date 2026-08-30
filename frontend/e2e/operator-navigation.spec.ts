import { expect, test, type Locator, type Page } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { OPERATOR_FIXTURE_NOW } from './fixtures/operatorData'

const primaryLinks = [
  { href: '/admin/dashboard', target: '/admin/dashboard' },
  { href: '/admin/stats', target: '/admin/stats' },
  { href: '/admin/accounts', target: '/admin/accounts' },
  { href: '/admin/groups', target: '/admin/groups' },
  { href: '/admin/usage', target: '/admin/usage' },
  { href: '/admin/settings', target: '/admin/settings' },
]

const fidelityAdminRoutes = ['/admin/dashboard', '/admin/stats', '/admin/accounts', '/admin/groups', '/admin/usage', '/admin/settings']
const fidelityViewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
]

const operatorPopupPalette = {
  surface: 'rgb(23, 23, 23)',
  search: 'rgb(20, 20, 20)',
  hover: 'rgb(32, 32, 32)',
  selected: 'rgb(37, 37, 37)',
  border: 'rgb(58, 58, 58)',
  foreground: 'rgb(245, 245, 245)',
  muted: 'rgb(163, 163, 163)',
}

function expectDarkNeutralSurface(color: string, label: string) {
  expect(color, `${label} must resolve to an RGB color`).toMatch(/^rgba?\(/)
  const channels = color.match(/[\d.]+/g)?.map(Number) ?? []
  const [red, green, blue, alpha = 1] = channels
  expect(alpha, `${label} must be opaque`).toBe(1)
  expect(Math.max(red, green, blue), `${label} must stay dark`).toBeLessThan(70)
  expect(Math.max(red, green, blue) - Math.min(red, green, blue), `${label} must stay neutral`).toBeLessThanOrEqual(8)
}

async function expectNeutralOperatorMenu(
  page: Page,
  menu: Locator,
  emphasizedItem: Locator,
  state: 'hover' | 'selected' = 'hover',
) {
  await expect(menu).toBeVisible()
  await expect(emphasizedItem).toBeVisible()
  if (state === 'hover') await emphasizedItem.hover()
  await page.waitForTimeout(200)

  const menuColors = await menu.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, border: style.borderColor }
  })
  const itemColors = await emphasizedItem.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, color: style.color }
  })

  expect(menuColors).toEqual({
    background: operatorPopupPalette.surface,
    border: operatorPopupPalette.border,
  })
  expect(itemColors).toEqual({
    background: operatorPopupPalette[state],
    color: operatorPopupPalette.foreground,
  })
}

async function openNeutralOperatorSelect(page: Page, trigger: Locator) {
  await trigger.scrollIntoViewIfNeeded()
  await trigger.click()

  const popup = page.locator('body > .select-dropdown-portal.operator-select-menu.operator-menu')
  await expect(popup).toBeVisible()
  const layers = await popup.evaluate((element) => {
    const background = (selector: string) => {
      const layer = element.querySelector(selector)
      return layer ? getComputedStyle(layer).backgroundColor : null
    }
    return {
      root: getComputedStyle(element).backgroundColor,
      border: getComputedStyle(element).borderColor,
      search: background('.select-search'),
      searchInput: background('.select-search-input'),
      list: background('.select-options'),
      normal: background('.select-option:not(.select-option-selected)'),
      selected: background('.select-option-selected'),
    }
  })

  expect(layers.root).toBe(operatorPopupPalette.surface)
  expect(layers.border).toBe(operatorPopupPalette.border)
  if (layers.search !== null) expect(layers.search).toBe(operatorPopupPalette.search)
  if (layers.searchInput !== null) expect(layers.searchInput).toBe(operatorPopupPalette.search)
  expect(layers.list).toBe(operatorPopupPalette.surface)
  expect(layers.normal).toBe(operatorPopupPalette.surface)
  expect(layers.selected).toBe(operatorPopupPalette.selected)

  const normalOption = popup.locator('.select-option:not(.select-option-selected)').first()
  const selectedOption = popup.locator('.select-option-selected').first()
  await normalOption.hover()
  await expect.poll(() => normalOption.evaluate((element) => getComputedStyle(element).backgroundColor))
    .toBe(operatorPopupPalette.hover)
  expect(await selectedOption.evaluate((element) => getComputedStyle(element).backgroundColor))
    .toBe(operatorPopupPalette.selected)

  await trigger.click()
  await expect(popup).toHaveCount(0)
}

async function operatorTokens(page: Page) {
  return page.locator('.operator-console').evaluate((element) => {
    const style = getComputedStyle(element)
    return {
      background: style.getPropertyValue('--operator-background').trim(),
      card: style.getPropertyValue('--operator-card').trim(),
      border: style.getPropertyValue('--operator-border').trim(),
      borderSubtle: style.getPropertyValue('--operator-border-subtle').trim(),
      muted: style.getPropertyValue('--operator-muted').trim(),
      mutedForeground: style.getPropertyValue('--operator-muted-foreground').trim(),
      foreground: style.getPropertyValue('--operator-foreground').trim(),
      focus: style.getPropertyValue('--operator-focus').trim(),
      track: style.getPropertyValue('--operator-track').trim(),
    }
  })
}

async function expectNeutralOperatorDialog(page: Page, dialog: Locator) {
  const tokens = await operatorTokens(page)
  const shell = dialog.locator('.operator-dialog')
  const colors = await shell.evaluate((element) => {
    const header = element.querySelector('.operator-dialog-header') as HTMLElement
    const body = element.querySelector('.operator-dialog-body') as HTMLElement
    const footer = element.querySelector('.operator-dialog-footer') as HTMLElement | null
    return {
      shell: getComputedStyle(element).backgroundColor,
      border: getComputedStyle(element).borderColor,
      header: getComputedStyle(header).backgroundColor,
      headerBorder: getComputedStyle(header).borderBottomColor,
      body: getComputedStyle(body).backgroundColor,
      footer: footer ? getComputedStyle(footer).backgroundColor : null,
      footerBorder: footer ? getComputedStyle(footer).borderTopColor : null,
    }
  })

  expect(colors).toMatchObject({
    shell: tokens.card,
    border: tokens.border,
    header: tokens.card,
    headerBorder: tokens.border,
    body: tokens.card,
  })
  if (colors.footer !== null) {
    expect(colors.footer).toBe(tokens.card)
    expect(colors.footerBorder).toBe(tokens.border)
  }
}

test.describe('operator console navigation', () => {
  test.beforeEach(async ({ page }) => {
    await page.clock.setFixedTime(OPERATOR_FIXTURE_NOW)
    await seedSession(page)
    await installOperatorApiMock(page)
  })

  test('reaches all six areas and keeps history and deep-link state', async ({ page }) => {
    test.setTimeout(60_000)
    await page.goto('/admin/dashboard')
    await expect(page.locator('.operator-primary-nav a[href="/admin/dashboard"]')).toHaveClass(/operator-primary-link-active/)

    for (const link of primaryLinks.slice(1)) {
      const navLink = page.locator(`.operator-primary-nav a[href="${link.href}"]`)
      await expect(navLink).toBeVisible()
      await navLink.click()
      await expect(page).toHaveURL(new RegExp(`${link.target.replaceAll('/', '\\/')}(?:\\?.*)?$`), { timeout: 15_000 })
      await expect(navLink).toHaveClass(/operator-primary-link-active/)
      await expect(page.getByRole('progressbar', { name: 'Loading' })).toBeHidden()
    }

    await page.goto('/admin/channels/pricing')
    await expect(page.locator('.operator-primary-nav a[href="/admin/groups"]')).toHaveClass(/operator-primary-link-active/)
    await expect(page.locator('nav[aria-label] a[href="/admin/channels/pricing"]')).toHaveClass(/operator-context-link-active/)

    await page.goto('/admin/audit-logs')
    await expect(page.locator('.operator-primary-nav a[href="/admin/usage"]')).toHaveClass(/operator-primary-link-active/)
    await expect(page.locator('nav[aria-label] a[href="/admin/audit-logs"]')).toHaveClass(/operator-context-link-active/)

    await page.goto('/admin/accounts')
    await page.goto('/admin/groups')
    await page.goBack()
    await expect(page).toHaveURL(/\/admin\/accounts$/)
    await expect(page.locator('.operator-primary-nav a[href="/admin/accounts"]')).toHaveClass(/operator-primary-link-active/)
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

  test('keeps admin personal routes outside operator styling', async ({ page }) => {
    await page.goto('/keys')

    await expect(page.locator('.operator-console')).toHaveCount(0)
  })

  test('keeps teleported operator popup layers dark neutral without the global dark theme', async ({ page }) => {
    await page.addInitScript(() => localStorage.setItem('theme', 'light'))
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.goto('/admin/accounts')
    await expect(page.locator('html')).not.toHaveClass(/dark/)

    await page.locator('.operator-accounts-toolbar .select-trigger').first().click()
    const popup = page.locator('body > .select-dropdown-portal.operator-select-menu.operator-menu')
    await expect(popup).toBeVisible()

    const normalOption = popup.locator('.select-option:not(.select-option-selected)').first()
    const selectedOption = popup.locator('.select-option-selected')
    const initialColors = await popup.evaluate((element) => {
      const color = (selector: string) => getComputedStyle(element.querySelector(selector) as HTMLElement).backgroundColor
      return {
        root: getComputedStyle(element).backgroundColor,
        rootBorder: getComputedStyle(element).borderColor,
        search: color('.select-search'),
        searchBorder: getComputedStyle(element.querySelector('.select-search') as HTMLElement).borderBottomColor,
        searchInput: color('.select-search-input'),
        list: color('.select-options'),
        normal: color('.select-option:not(.select-option-selected)'),
        selected: color('.select-option-selected'),
      }
    })
    for (const [label, color] of Object.entries(initialColors)) {
      expectDarkNeutralSurface(color, label)
    }

    await normalOption.hover()
    expectDarkNeutralSurface(
      await normalOption.evaluate((element) => getComputedStyle(element).backgroundColor),
      'hovered option',
    )
    expectDarkNeutralSurface(
      await selectedOption.evaluate((element) => getComputedStyle(element).backgroundColor),
      'selected option after another option is highlighted',
    )

    await popup.locator('.select-search-input').fill('__no_platform_match__')
    const empty = popup.locator('.select-empty')
    await expect(empty).toBeVisible()
    expectDarkNeutralSurface(
      await empty.evaluate((element) => getComputedStyle(element).backgroundColor),
      'empty state',
    )
  })

  test('keeps operator popup colors stable through same-context route lifecycles', async ({ page }) => {
    test.setTimeout(90_000)
    await page.addInitScript(() => localStorage.setItem('theme', 'light'))
    await page.setViewportSize({ width: 1440, height: 900 })

    const navigate = async (path: string) => {
      await page.locator(`.operator-primary-nav a[href="${path}"]`).click()
      await expect(page).toHaveURL(new RegExp(`${path.replaceAll('/', '\\/')}$`))
      await expect(page.locator('body')).toHaveAttribute('data-operator-console', '')
      await expect(page.locator('.operator-console')).toBeVisible()
      await expect(page.locator('body > .select-dropdown-portal')).toHaveCount(0)
    }
    const openLocaleMenu = async () => {
      const trigger = page.locator('.operator-header-secondary-action > button').last()
      await trigger.click()
      const menu = page.locator('.operator-header .operator-menu')
      await expectNeutralOperatorMenu(page, menu, menu.locator('[aria-current="true"]'), 'selected')
      await trigger.click()
      await expect(menu).toHaveCount(0)
    }

    await page.goto('/admin/accounts')
    await expect(page.locator('html')).not.toHaveClass(/dark/)
    await expect(page.locator('body')).toHaveAttribute('data-operator-console', '')
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').first())

    await navigate('/admin/usage')
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').filter({ hasText: 'Hour' }))

    await navigate('/admin/accounts')
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').first())
    const moreActionsButton = page.getByRole('button', { name: 'More Actions' })
    await moreActionsButton.click()
    const moreActionsMenu = page.locator('.account-tools-menu')
    await expectNeutralOperatorMenu(page, moreActionsMenu, moreActionsMenu.locator('.operator-menu-item').first())
    await moreActionsButton.click()
    await expect(moreActionsMenu).toHaveCount(0)
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').nth(2))

    await navigate('/admin/stats')
    await openLocaleMenu()
    await navigate('/admin/groups')
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').first())
    await navigate('/admin/usage')
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').filter({ hasText: 'Hour' }))
    await navigate('/admin/settings')
    await page.getByRole('tab', { name: 'Gateway' }).click()
    await openNeutralOperatorSelect(page, page.locator('.select-trigger:visible').first())
    await navigate('/admin/accounts')
    await openNeutralOperatorSelect(page, page.locator('.select-trigger').first())

    for (let iteration = 0; iteration < 3; iteration += 1) {
      await navigate('/admin/groups')
      await openNeutralOperatorSelect(page, page.locator('.select-trigger').first())
      await navigate('/admin/accounts')
      await openNeutralOperatorSelect(page, page.locator('.select-trigger').first())
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

  test('keeps Overview compact and shows normalized global and provider capacity on Stats', async ({ page }) => {
    await page.goto('/admin/dashboard')

    const summary = page.getByTestId('account-pool-capacity')
    await expect(summary).toBeVisible()
    await expect(summary).toContainText('21%')
    await expect(summary.locator('[data-testid="capacity-account-segment"]')).toHaveCount(0)
    await expect(summary.getByRole('link', { name: 'View Stats' })).toHaveAttribute('href', '/admin/stats')

    await summary.getByRole('link', { name: 'View Stats' }).click()
    await expect(page).toHaveURL(/\/admin\/stats$/)

    const global = page.getByTestId('global-capacity-donut')
    await expect(global).toContainText('21%')
    await expect(global.locator('[data-testid="capacity-account-segment"]')).toHaveCount(4)
    await expect(global.locator('svg[role="img"]')).toHaveAttribute('aria-label', /21% available, 79% Used capacity/)
    await expect(global).not.toContainText('Gemini Recovery')

    const usedSegment = global.getByTestId('capacity-used-segment')
    const usedBounds = await usedSegment.boundingBox()
    expect(usedBounds).not.toBeNull()
    await page.mouse.move(
      (usedBounds?.x ?? 0) + (usedBounds?.width ?? 0) / 2,
      (usedBounds?.y ?? 0) + 2,
    )
    await expect(global.locator('.stats-capacity-tooltip')).toBeVisible()
    const firstSegment = global.locator('[data-testid="capacity-account-segment"]').first()
    await firstSegment.focus()
    await page.keyboard.press('Enter')
    await expect(firstSegment).toHaveAttribute('aria-pressed', 'true')

    const segmentSelect = global.getByRole('button', { name: 'Inspect segment' })
    await segmentSelect.focus()
    await page.keyboard.press('ArrowDown')
    await expect(page.getByRole('listbox')).toBeFocused()
    await page.keyboard.press('ArrowDown')
    await page.keyboard.press('Enter')
    await expect(segmentSelect).toBeFocused()
    await expect(global.getByTestId('capacity-selected-detail')).toContainText('Claude Primary')

    const openAI = page.getByTestId('provider-capacity-openai')
    await expect(openAI).toContainText('52%')
    const openAIWindows = openAI.locator('details.stats-window-section')
    await expect(openAIWindows).not.toHaveAttribute('open', '')
    await openAIWindows.locator('summary').click()
    await expect(openAIWindows).toHaveAttribute('open', '')
    await expect(openAI).toContainText('5h')
    await expect(openAI).toContainText('7d')
    await expect(openAI.getByTestId('provider-capacity-donut-openai').locator('[data-testid="capacity-account-segment"]')).toHaveCount(1)
    await expect(openAI.getByTestId('provider-capacity-donut-openai').locator('[data-testid="capacity-used-segment"]')).toBeVisible()

    const anthropic = page.getByTestId('provider-capacity-anthropic')
    await expect(anthropic).toContainText('4%')
    await expect(anthropic.getByTestId('provider-capacity-donut-anthropic').locator('[data-testid="capacity-account-segment"]')).toHaveCount(1)

    const antigravity = page.getByTestId('provider-capacity-antigravity')
    await expect(antigravity).toContainText('28%')
    await expect(antigravity.getByTestId('provider-capacity-donut-antigravity').locator('[data-testid="capacity-account-segment"]')).toHaveCount(1)

    const gemini = page.getByTestId('provider-capacity-gemini')
    await expect(gemini).toContainText('0%')
    await expect(gemini).toContainText('Gemini Quota Limited')
    await expect(gemini).not.toContainText('1 unknown')
    await expect(gemini.getByTestId('provider-capacity-donut-gemini').locator('[data-testid="capacity-account-segment"]')).toHaveCount(1)
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
    await expect(cooldownRow).toContainText('Provider cooldown')

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

  test('uses semantic green switches and neutral open-menu states across operator areas', async ({ page }) => {
    test.setTimeout(60_000)
    await page.addInitScript(() => localStorage.setItem('theme', 'dark'))
    await page.setViewportSize({ width: 1024, height: 900 })
    await page.goto('/admin/accounts')

    const inactiveContextBorder = await page.locator('.operator-context-link:not(.operator-context-link-active)').first()
      .evaluate((element) => getComputedStyle(element).borderBottomColor)
    expect(inactiveContextBorder).toBe('rgba(0, 0, 0, 0)')

    await page.getByRole('tab', { name: 'Technical' }).click()

    const enabledSwitch = page.locator('.operator-scheduling-switch[aria-checked="true"]').first()
    const disabledSwitch = page.locator('.operator-scheduling-switch[aria-checked="false"]').first()
    await expect(enabledSwitch).toBeVisible()
    await expect(disabledSwitch).toBeVisible()
    await expect(enabledSwitch).toHaveAttribute('aria-label', 'Scheduling enabled: Codex Team West')
    await expect(disabledSwitch).toHaveAttribute('aria-label', /Scheduling disabled: /)
    await expect(enabledSwitch).toHaveText('')
    await expect(disabledSwitch).toHaveText('')
    const switchColors = await page.evaluate(() => {
      const root = document.querySelector('.operator-console') as HTMLElement
      const enabled = document.querySelector('.operator-scheduling-switch[aria-checked="true"]') as HTMLElement
      const disabled = document.querySelector('.operator-scheduling-switch[aria-checked="false"]') as HTMLElement
      const rootStyle = getComputedStyle(root)
      return {
        success: rootStyle.getPropertyValue('--operator-success-fill').trim(),
        track: rootStyle.getPropertyValue('--operator-track').trim(),
        enabled: getComputedStyle(enabled).backgroundColor,
        disabled: getComputedStyle(disabled).backgroundColor,
      }
    })
    expect(switchColors.enabled).toBe(switchColors.success)
    expect(switchColors.disabled).toBe(switchColors.track)

    const secondTechnicalRow = page.locator('.operator-account-table .table-body tr[data-index="1"]')
    expect(await secondTechnicalRow.evaluate((element) => getComputedStyle(element).borderTopColor))
      .toBe((await operatorTokens(page)).borderSubtle)

    const routingGroup = page.locator('.operator-account-routing-groups .group-badge').first()
    const routingGroupColors = await routingGroup.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, color: style.color }
    })
    expect(routingGroupColors).toEqual({
      background: (await operatorTokens(page)).muted,
      color: (await operatorTokens(page)).mutedForeground,
    })

    const platformSelect = page.locator('.select-trigger').first()
    const selectTokens = await operatorTokens(page)
    const closedSelectColors = await platformSelect.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, border: style.borderColor, color: style.color }
    })
    expect(closedSelectColors).toEqual({
      background: selectTokens.card,
      border: selectTokens.border,
      color: selectTokens.foreground,
    })
    await platformSelect.click()
    await expect(page.locator('body > .select-dropdown-portal.operator-select-menu.operator-menu')).toBeVisible()
    await expect.poll(() => platformSelect.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, border: style.borderColor, color: style.color }
    })).toEqual({
      background: selectTokens.card,
      border: selectTokens.focus,
      color: selectTokens.foreground,
    })
    const selectedOption = page.locator('.select-dropdown-portal .select-option-selected')
    const popup = page.locator('.select-dropdown-portal')
    await expectNeutralOperatorMenu(page, popup, selectedOption, 'selected')
    const popupColors = await popup.evaluate((element) => {
      const search = element.querySelector('.select-search') as HTMLElement
      const options = element.querySelector('.select-options') as HTMLElement
      const normal = element.querySelector('.select-option:not(.select-option-selected)') as HTMLElement
      const selected = element.querySelector('.select-option-selected') as HTMLElement
      return {
        search: getComputedStyle(search).backgroundColor,
        searchBorder: getComputedStyle(search).borderBottomColor,
        options: getComputedStyle(options).backgroundColor,
        scrollbar: getComputedStyle(options).scrollbarColor,
        normal: getComputedStyle(normal).backgroundColor,
        selected: getComputedStyle(selected).backgroundColor,
      }
    })
    expect(popupColors).toEqual({
      search: operatorPopupPalette.search,
      searchBorder: operatorPopupPalette.border,
      options: operatorPopupPalette.surface,
      scrollbar: 'rgb(82, 82, 82) rgb(20, 20, 20)',
      normal: operatorPopupPalette.surface,
      selected: operatorPopupPalette.selected,
    })
    const checkColor = await selectedOption.locator('svg').evaluate((element) => getComputedStyle(element).color)
    expect(checkColor).toBe(operatorPopupPalette.foreground)
    await page.keyboard.press('Escape')

    const moreActionsButton = page.getByRole('button', { name: 'More Actions' })
    await moreActionsButton.click()
    const moreActionsMenu = page.locator('.account-tools-menu')
    await expectNeutralOperatorMenu(page, moreActionsMenu, moreActionsMenu.locator('.operator-menu-item').first())
    expect(await moreActionsMenu.locator('.operator-menu-divider').first().evaluate(
      (element) => getComputedStyle(element).borderTopColor,
    )).toBe(operatorPopupPalette.border)
    expect(await moreActionsMenu.locator('.text-xs.font-semibold.uppercase').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(operatorPopupPalette.muted)
    const [moreActionsBox, moreActionsStatusBarBox] = await Promise.all([
      moreActionsMenu.boundingBox(),
      page.locator('.operator-status-bar').boundingBox(),
    ])
    expect(moreActionsBox).not.toBeNull()
    expect(moreActionsStatusBarBox).not.toBeNull()
    expect(moreActionsBox!.y + moreActionsBox!.height).toBeLessThanOrEqual(moreActionsStatusBarBox!.y)
    await moreActionsButton.click()

    const moreButton = page.locator('.operator-account-table .operator-table-row-action').filter({ hasText: 'More' }).first()
    await moreButton.click()
    const menu = page.locator('.action-menu-content')
    await expect(menu).toBeVisible()
    const [menuBox, statusBarBox] = await Promise.all([
      menu.boundingBox(),
      page.locator('.operator-status-bar').boundingBox(),
    ])
    expect(menuBox).not.toBeNull()
    expect(statusBarBox).not.toBeNull()
    expect(menuBox!.y + menuBox!.height).toBeLessThanOrEqual(statusBarBox!.y)
    const firstAction = menu.locator('.operator-menu-item').first()
    await expectNeutralOperatorMenu(page, menu, firstAction)
    await page.keyboard.press('Escape')

    await page.goto('/admin/usage')
    const activityTokens = await operatorTokens(page)
    const dateRange = page.locator('.date-picker-trigger')
    await expect(dateRange).toBeVisible()
    expect(await dateRange.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, border: style.borderColor, color: style.color }
    })).toEqual({
      background: activityTokens.card,
      border: activityTokens.border,
      color: activityTokens.foreground,
    })
    await dateRange.click()
    await expect.poll(() => dateRange.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, border: style.borderColor, color: style.color }
    })).toEqual({
      background: activityTokens.card,
      border: activityTokens.focus,
      color: activityTokens.foreground,
    })
    await expectNeutralOperatorMenu(
      page,
      page.locator('.date-picker-dropdown'),
      page.locator('.date-picker-preset-active'),
      'selected',
    )
    expect(await page.locator('.date-picker-preset:not(.date-picker-preset-active)').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(operatorPopupPalette.foreground)
    expect(await page.locator('.date-picker-label').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(operatorPopupPalette.muted)
    await page.keyboard.press('Escape')

    await page.goto('/admin/ops')
    const activitySelect = page.locator('.select-trigger').first()
    await activitySelect.click()
    await page.keyboard.press('ArrowDown')
    await expectNeutralOperatorMenu(
      page,
      page.locator('.select-dropdown-portal'),
      page.locator('.select-dropdown-portal .select-option-focused'),
    )
    await page.keyboard.press('Escape')

    await page.goto('/admin/groups')
    const groupsTokens = await operatorTokens(page)
    const neutralBadge = page.locator('.badge-gray').first()
    expect(await neutralBadge.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, color: style.color }
    })).toEqual({ background: groupsTokens.muted, color: groupsTokens.mutedForeground })
    expect(await page.locator('.table-body > tr + tr').first().evaluate(
      (element) => getComputedStyle(element).borderTopColor,
    )).toBe(groupsTokens.border)
    await page.getByRole('button', { name: 'Column Settings' }).click()
    const groupsMenu = page.locator('.operator-menu:visible')
    await expectNeutralOperatorMenu(page, groupsMenu, groupsMenu.locator('[aria-pressed="true"]').first(), 'selected')

    await page.goto('/admin/settings')
    const settingsTokens = await operatorTokens(page)
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
      background: settingsTokens.muted,
      border: settingsTokens.border,
      activeBackground: settingsTokens.background,
      activeColor: settingsTokens.foreground,
    })
    await page.getByRole('tab', { name: 'Gateway' }).click()
    await page.locator('.select-trigger:visible').first().click()
    await page.keyboard.press('ArrowDown')
    await expectNeutralOperatorMenu(
      page,
      page.locator('.select-dropdown-portal'),
      page.locator('.select-dropdown-portal .select-option-focused'),
    )
  })

  test('uses neutral Create, Edit, and Bulk Edit dialog surfaces and callouts', async ({ page }) => {
    test.setTimeout(60_000)
    await page.addInitScript(() => localStorage.setItem('theme', 'dark'))
    await page.goto('/admin/accounts')

    const createTrigger = page.getByRole('button', { name: 'Create Account', exact: true }).first()
    await createTrigger.click()
    const createDialog = page.getByRole('dialog', { name: 'Create Account' })
    await expectNeutralOperatorDialog(page, createDialog)
    const tokens = await operatorTokens(page)
    expect(await createDialog.locator('.input-hint').first().evaluate(
      (element) => getComputedStyle(element).color,
    )).toBe(tokens.mutedForeground)
    await page.keyboard.press('Escape')

    await page.getByRole('tab', { name: 'Technical' }).click()
    await page.locator('.operator-account-table .operator-table-row-action').filter({ hasText: 'Edit' }).first().click()
    const editDialog = page.getByRole('dialog', { name: 'Edit Account' })
    await expectNeutralOperatorDialog(page, editDialog)
    await page.keyboard.press('Escape')

    await page.locator('.operator-account-table tbody input[type="checkbox"]').first().check()
    await page.getByRole('button', { name: 'Bulk Edit', exact: true }).click()
    const bulkDialog = page.getByRole('dialog', { name: 'Bulk Edit Accounts' })
    await expectNeutralOperatorDialog(page, bulkDialog)

    const callout = bulkDialog.locator('.operator-callout-info').first()
    const calloutColors = await callout.evaluate((element) => {
      const style = getComputedStyle(element)
      return { background: style.backgroundColor, borderLeft: style.borderLeftColor }
    })
    expect(calloutColors).toEqual({
      background: tokens.muted,
      borderLeft: tokens.mutedForeground,
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

    const header = await page.locator('.operator-header').boundingBox()
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
  await expect(page.locator('.operator-primary-nav a[href="/admin/groups"]')).toHaveCount(0)
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
