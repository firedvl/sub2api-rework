import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`weekly cost estimate stays a display projection at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [operatorFixtureAccounts[0]], total: 1, page: 1, pages: 1, page_size: 20,
    } } }))
    const usage = {
      five_hour: { utilization: 25, resets_at: null, window_stats: { requests: 1, tokens: 100, cost: 2 } },
      seven_day: { utilization: 40, resets_at: null, window_stats: { requests: 2, tokens: 200, cost: 12 } },
    }
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 101: usage }, errors: {} } } }))
    await page.route('**/api/v1/admin/accounts/101/usage*', route => route.fulfill({ json: { code: 0, data: usage } }))
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/admin/accounts?view=technical')
    const estimate = page.locator('[data-test="estimated-total-cost"]')
    await expect(estimate).toHaveCount(1)
    await expect(estimate).toHaveText('Est. total $30.00')
    await expect(estimate).toHaveAttribute('title', 'Estimated total cost at 100% utilization, based on current window cost and utilization')
    await estimate.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`weekly-estimate-${width}.png`), animations: 'disabled' })
    expect(errors).toEqual([])
  })
}
