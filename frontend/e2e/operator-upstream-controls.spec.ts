import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`whitelist mapping conflict preserves the draft at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.goto('/admin/accounts')
    await page.getByRole('button', { name: 'Create Account', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Create Account', exact: true })
    await dialog.getByRole('button', { name: 'Claude Console API Key', exact: true }).click()
    await dialog.getByRole('button', { name: 'Model Mapping', exact: true }).click()
    await dialog.getByRole('button', { name: 'Add Mapping', exact: true }).click()
    await dialog.getByPlaceholder('Request Model').fill('custom-alias')
    await dialog.getByPlaceholder('Actual Model').fill('custom-target')
    await dialog.getByRole('button', { name: 'Model Whitelist', exact: true }).click()
    const custom = dialog.getByPlaceholder('Enter custom model name')
    await custom.fill('custom-alias')
    await dialog.getByRole('button', { name: 'Add', exact: true }).click()
    await expect(page.getByText('Remove mapped models first.', { exact: true })).toBeVisible()
    await expect(custom).toHaveValue('custom-alias')
    await custom.fill('custom-valid')
    await dialog.getByRole('button', { name: 'Add', exact: true }).click()
    await expect(dialog.getByText('custom-valid', { exact: true })).toBeVisible()
    await page.screenshot({ path: testInfo.outputPath(`whitelist-${width}.png`) })
    await dialog.getByRole('button', { name: 'Model Mapping', exact: true }).click()
    await expect(dialog.getByPlaceholder('Request Model')).toHaveValue('custom-alias')
    await expect(dialog.getByPlaceholder('Actual Model')).toHaveValue('custom-target')
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  })

  test(`operator priority and Overview snapshot recovery at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const account = { ...operatorFixtureAccounts[0], priority: 10 }
    const writes: unknown[] = []
    await page.route('**/api/v1/admin/accounts/101', route => {
      if (route.request().method() === 'PUT') {
        const body = route.request().postDataJSON()
        writes.push(body)
        if (writes.length === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Fixture priority failure' } })
        account.priority = body.priority
      }
      return route.fulfill({ json: { code: 0, data: account } })
    })
    await page.goto('/admin/accounts?view=technical')
    const cell = page.getByTestId('account-priority-cell').first()
    await cell.scrollIntoViewIfNeeded()
    await cell.getByTestId('account-priority-value').click()
    await cell.getByTestId('account-priority-input').fill('12')
    await cell.getByTestId('account-priority-input').press('Enter')
    await expect(page.getByText('Fixture priority failure', { exact: true })).toBeVisible()
    await expect(cell.getByTestId('account-priority-value')).toHaveText('10')
    await cell.getByTestId('account-priority-value').click()
    await cell.getByTestId('account-priority-input').fill('12')
    await cell.getByTestId('account-priority-input').press('Enter')
    await expect(cell.getByTestId('account-priority-value')).toHaveText('12')
    expect(writes).toEqual([{ priority: 12 }, { priority: 12 }])
    await page.screenshot({ path: testInfo.outputPath(`priority-${width}.png`) })

    // A failed Overview snapshot replaces only the status card; the account cards still load.
    let snapshotAttempts = 0
    await page.route('**/api/v1/admin/dashboard/snapshot-v2*', route => {
      if (++snapshotAttempts === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Fixture snapshot failure' } })
      return route.fallback()
    })
    await page.goto('/admin/dashboard')
    const loadError = page.getByTestId('dashboard-load-error')
    const status = page.locator('section[aria-labelledby="overview-status"]')
    await expect(loadError).toBeVisible()
    await expect(loadError).toHaveAttribute('role', 'alert')
    await expect(status).toHaveCount(0)
    await expect(page.getByTestId('overview-state-limited')).toHaveText('2 Limited')
    await expect(page.locator('section[aria-labelledby="overview-recent"] .overview-table tbody tr')).toHaveCount(4)
    expect(snapshotAttempts).toBe(1)
    await loadError.getByRole('button', { name: 'Retry', exact: true }).click()
    await expect(loadError).toHaveCount(0)
    await expect(status.locator('.overview-today')).toContainText('4,286')
    const hourly = status.getByRole('img', { name: /^Requests per hour over the last 24 hours\./ })
    await expect(hourly).toBeVisible()
    expect(snapshotAttempts).toBe(2)
    await hourly.focus()
    await page.keyboard.press('ArrowLeft')
    await expect(status.locator('.trend-tip')).toContainText('Requests')
    await page.keyboard.press('Escape')
    await expect(status.locator('.trend-tip')).toHaveCount(0)
    await page.screenshot({ path: testInfo.outputPath(`overview-recovery-${width}.png`) })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  })
}
