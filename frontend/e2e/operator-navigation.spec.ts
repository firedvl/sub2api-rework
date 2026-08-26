import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const primaryLinks = [
  { href: '/admin/dashboard', target: '/admin/dashboard' },
  { href: '/admin/accounts', target: '/admin/accounts' },
  { href: '/admin/groups', target: '/admin/groups' },
  { href: '/admin/usage', target: '/admin/usage' },
  { href: '/admin/settings', target: '/admin/settings' },
]

const fidelityAdminRoutes = ['/admin/dashboard', '/admin/accounts', '/admin/groups', '/admin/usage', '/admin/settings']
const fidelityViewports = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1024, height: 768 },
  { width: 768, height: 900 },
]

test.describe('operator console navigation', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page)
    await installOperatorApiMock(page)
  })

  test('reaches all five areas and keeps history and deep-link state', async ({ page }) => {
    await page.goto('/admin/dashboard')
    await expect(page.locator('.operator-primary-nav a[href="/admin/dashboard"]')).toHaveClass(/operator-primary-link-active/)

    for (const link of primaryLinks.slice(1)) {
      const navLink = page.locator(`.operator-primary-nav a[href="${link.href}"]`)
      await expect(navLink).toBeVisible()
      await navLink.click()
      await expect(page).toHaveURL(new RegExp(`${link.target.replaceAll('/', '\\/')}(?:\\?.*)?$`))
      await expect(navLink).toHaveClass(/operator-primary-link-active/)
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

  test('keeps the Accounts technical table inside the desktop layout', async ({ page }) => {
    await page.goto('/admin/accounts')

    const layout = page.locator('.operator-accounts-layout')
    const details = page.locator('.operator-account-details')
    await expect(layout).toBeVisible()
    await details.locator('summary').click()

    const table = page.locator('.operator-account-table')
    await expect(table).toBeVisible()
    const bounds = await layout.evaluate((element) => {
      const layoutBounds = element.getBoundingClientRect()
      const tableBounds = element.querySelector('.operator-account-table')!.getBoundingClientRect()
      return {
        layoutLeft: layoutBounds.left,
        layoutRight: layoutBounds.right,
        tableLeft: tableBounds.left,
        tableRight: tableBounds.right,
        viewportWidth: window.innerWidth,
      }
    })
    expect(bounds.layoutRight).toBeLessThanOrEqual(bounds.viewportWidth + 1)
    expect(bounds.tableLeft).toBeGreaterThanOrEqual(bounds.layoutLeft - 1)
    expect(bounds.tableRight).toBeLessThanOrEqual(bounds.layoutRight + 1)
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
