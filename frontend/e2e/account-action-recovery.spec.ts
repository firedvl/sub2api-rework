import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts, operatorFixtureGroups } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`account refresh retry and inactive assignment removal at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    const inactive = { ...operatorFixtureGroups[0], id: 88, name: 'Inactive assignment', status: 'inactive' }
    const account = { ...operatorFixtureAccounts[0], group_ids: [11, 88], groups: [operatorFixtureGroups[0], inactive, { ...inactive, id: 89, name: 'Stale assignment' }] }
    const accounts = [account, { ...operatorFixtureAccounts[0], id: 102, name: 'Refresh retry target' }]
    const refreshes: number[][] = []
    const writes: Record<string, unknown>[] = []
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('dialog', dialog => dialog.accept())
    await page.route('**/api/v1/admin/accounts**', route => {
      const path = new URL(route.request().url()).pathname
      if (path === '/api/v1/admin/accounts' && route.request().method() === 'GET') {
        return route.fulfill({ json: { code: 0, data: { items: accounts, total: 2, page: 1, page_size: 50, pages: 1 } } })
      }
      if (path === '/api/v1/admin/accounts/batch-refresh') {
        refreshes.push(route.request().postDataJSON().account_ids)
        const result = refreshes.length === 1
          ? { total: 2, success: 1, failed: 1, errors: [{ account_id: 102, error: 'Fixture refresh failure' }] }
          : { total: 1, success: 1, failed: 0 }
        return route.fulfill({ json: { code: 0, data: result } })
      }
      if (path === '/api/v1/admin/accounts/101') {
        if (route.request().method() !== 'GET') writes.push(route.request().postDataJSON())
        return route.fulfill({ json: { code: 0, data: account } })
      }
      return route.fallback()
    })
    await page.goto('/admin/accounts?view=technical')
    await page.getByRole('button', { name: 'Select all results (2)', exact: true }).click()
    await expect(page.getByText('All 2 account(s) selected', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Refresh Token', exact: true }).click()
    await expect.poll(() => refreshes.length).toBe(1)
    await expect(page.getByText('1 account(s) selected', { exact: true })).toBeVisible()
    expect(refreshes).toEqual([[101, 102]])
    await page.getByRole('button', { name: 'Refresh Token', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Refresh Token', exact: true })).toHaveCount(0)
    expect(refreshes).toEqual([[101, 102], [102]])
    if (width === 390) await page.getByRole('button', { name: 'More', exact: true }).first().click()
    await page.getByRole('button', { name: 'Edit', exact: true }).first().click()
    const dialog = page.getByRole('dialog', { name: 'Edit Account', exact: true })
    const assignment = dialog.getByRole('checkbox', { name: /Inactive assignment/ })
    await expect(assignment).toBeChecked()
    await expect(dialog.getByText('Stale assignment', { exact: true })).toHaveCount(0)
    await assignment.uncheck()
    await dialog.getByRole('button', { name: 'Update', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    expect(writes).toHaveLength(1)
    expect(writes[0].group_ids).toEqual([11])
    await page.goto('/admin/accounts?view=capacity')
    await page.getByRole('button', { name: 'More Codex Team West', exact: true }).click()
    const menu = page.locator('.action-menu-content')
    await expect(menu).toBeVisible()
    await page.setViewportSize({ width, height: 360 })
    await expect.poll(() => page.locator('.operator-status-bar').evaluate(element => element.getBoundingClientRect().bottom)).toBeLessThanOrEqual(360)
    await expect.poll(async () => {
      const bounds = await menu.boundingBox()
      const statusBarTop = await page.locator('.operator-status-bar').evaluate(element => element.getBoundingClientRect().top)
      return bounds && bounds.x >= 8 && bounds.y >= 8 && bounds.x + bounds.width <= width - 8 && bounds.y + bounds.height <= Math.min(360, statusBarTop) - 8
        ? 'within viewport'
        : JSON.stringify({ bounds, statusBarTop, style: await menu.getAttribute('style') })
    }).toBe('within viewport')
    await menu.hover()
    await page.mouse.wheel(0, 400)
    await expect(menu.getByRole('button', { name: 'Delete Account', exact: true })).toBeInViewport({ ratio: 1 })
    await page.screenshot({ path: testInfo.outputPath(`account-menu-${width}.png`), animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(menu).not.toBeVisible()
    expect(errors).toEqual([])
  })
}
