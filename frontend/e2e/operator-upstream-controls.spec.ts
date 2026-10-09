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

  test(`operator priority and usage trend recovery at ${width}px`, async ({ page }, testInfo) => {
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

    let spendAttempts = 0
    await page.route('**/api/v1/admin/dashboard/users-trend*', route => {
      const spending = new URL(route.request().url()).searchParams.get('metric') === 'actual_cost'
      if (spending && ++spendAttempts === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Fixture trend failure' } })
      return route.fulfill({ json: { code: 0, data: { trend: spending ? [{ date: '2026-10-08', user_id: 1, username: 'Fixture user', tokens: 10, actual_cost: 5 }] : [] } } })
    })
    await page.goto('/admin/dashboard')
    const trend = page.locator('section[aria-labelledby="user-usage-trend-title"]')
    await expect(trend.getByText('No data available', { exact: true })).toBeVisible()
    await trend.getByRole('button', { name: 'Actual spending ($)', exact: true }).click()
    await expect(trend.getByRole('alert')).toBeVisible()
    await trend.getByRole('button', { name: 'Retry', exact: true }).click()
    await expect(trend.locator('canvas')).toBeVisible()
    await expect(trend.getByRole('button', { name: 'Actual spending ($)', exact: true })).toHaveAttribute('aria-pressed', 'true')
    expect(spendAttempts).toBe(2)
    let previousPixels = ''
    let stableFrames = 0
    await expect.poll(async () => {
      const pixels = await trend.locator('canvas').evaluate(canvas => (canvas as HTMLCanvasElement).toDataURL())
      stableFrames = pixels === previousPixels ? stableFrames + 1 : 0
      previousPixels = pixels
      return stableFrames
    }, { intervals: [100, 250, 500, 1000], timeout: 10000 }).toBeGreaterThanOrEqual(4)
    await page.screenshot({ path: testInfo.outputPath(`spending-trend-${width}.png`) })
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  })
}
