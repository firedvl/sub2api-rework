import { expect, test, type Page, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureUpdateStatus } from './fixtures/operatorData'

const readyStatus = {
  ...operatorFixtureUpdateStatus,
  latest_compatible_rework: '0.1.184-rework.1',
  state: 'update_ready',
  installable: true,
  updater: {
    ...operatorFixtureUpdateStatus.updater,
    state: 'prepared',
    prepared_version: '0.1.184-rework.1',
  },
}

async function fulfill(route: Route, data: unknown) {
  await route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ code: 0, message: 'ok', data }),
  })
}

async function mockUpdateApi(page: Page, status: typeof operatorFixtureUpdateStatus | typeof readyStatus, installs: unknown[] = []) {
  await page.route('**/api/v1/admin/system/**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    if (pathname === '/api/v1/admin/system/check-updates') return fulfill(route, status)
    if (pathname === '/api/v1/admin/system/install') {
      installs.push(request.postDataJSON())
      return fulfill(route, { operation_id: 'install-fixture', action: 'install', state: 'accepted' })
    }
    return fulfill(route, { operation_id: 'fixture', action: 'prepare', state: 'accepted' })
  })
}

test.beforeEach(async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)
})

test('shows compatibility pending distinctly at a narrow width and renders release text safely', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await mockUpdateApi(page, operatorFixtureUpdateStatus)
  await page.goto('/admin/settings')

  const card = page.getByRole('region', { name: 'Software Updates' })
  await expect(card).toContainText('Compatibility review pending')
  await expect(card).toContainText('Upstream baseline')
  await expect(card).toContainText('v0.1.183')
  await expect(card).toContainText('Latest upstream')
  await expect(card).toContainText('v0.1.184')
  await expect(card).toContainText('<strong>Untrusted release text stays text.</strong>')
  await expect(card.locator('strong')).toHaveCount(0)
  await expect(card.getByRole('button', { name: 'Prepare' })).toBeDisabled()
  await expect(card.getByRole('button', { name: 'Install' })).toBeDisabled()
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('update-pending-narrow.png'), fullPage: true })
})

test('confirms a ready install by keyboard at a wide width', async ({ page }, testInfo) => {
  const installs: unknown[] = []
  await page.setViewportSize({ width: 1280, height: 800 })
  await mockUpdateApi(page, readyStatus, installs)
  await page.goto('/admin/settings')

  const card = page.getByRole('region', { name: 'Software Updates' })
  await expect(card).toContainText('Approved update ready')
  const install = card.getByRole('button', { name: 'Install' })
  await expect(install).toBeEnabled()
  await install.focus()
  await page.keyboard.press('Enter')

  const dialog = page.getByRole('dialog', { name: 'Install' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Close modal' })).toBeFocused()
  await page.keyboard.press('Tab')
  const confirmation = dialog.getByLabel('Type INSTALL 0.1.184-rework.1 to confirm')
  await expect(confirmation).toBeFocused()
  await confirmation.pressSequentially('INSTALL 0.1.184-rework.1')
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Cancel' })).toBeFocused()
  await page.keyboard.press('Tab')
  await expect(dialog.getByRole('button', { name: 'Confirm' })).toBeFocused()
  await page.screenshot({ path: testInfo.outputPath('update-ready-confirmation-wide.png'), fullPage: true })
  await page.keyboard.press('Enter')

  await expect(dialog).toBeHidden()
  await expect.poll(() => installs).toEqual([{
    version: '0.1.184-rework.1',
    confirmation: 'INSTALL 0.1.184-rework.1',
  }])
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
})
