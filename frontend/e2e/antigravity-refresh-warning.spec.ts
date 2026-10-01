import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`partial token refresh updates the row and keeps its warning at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    const account = { ...operatorFixtureAccounts[2], name: 'Refresh warning fixture' }
    const message = 'Token refreshed, but project_id is temporarily unavailable'
    let refreshes = 0
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [account], total: 1, page: 1, page_size: 50, pages: 1,
    } } }))
    await page.route('**/api/v1/admin/accounts/103/refresh', route => {
      refreshes++
      return route.fulfill({ json: { code: 0, data: {
        account: { ...account, name: 'Refreshed fixture' }, message, warning: 'missing_project_id_temporary',
      } } })
    })
    await page.goto('/admin/accounts?view=technical')
    await page.getByRole('button', { name: 'More', exact: true }).first().click()
    await page.locator('.action-menu-content').getByRole('button', { name: 'Refresh Token', exact: true }).click()
    await expect(page.getByText(message, { exact: true })).toBeVisible()
    await expect(page.getByText('Refreshed fixture', { exact: true }).first()).toBeVisible()
    expect(refreshes).toBe(1)
    expect(errors).toEqual([])
    await page.screenshot({ path: testInfo.outputPath(`refresh-warning-${width}.png`), animations: 'disabled' })
  })
}
