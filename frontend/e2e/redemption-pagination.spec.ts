import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`redemption history pages, retries and numeric jumps at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const requests: { page: number; size: number }[] = []
    let failNext = false
    await page.route('**/api/v1/redeem/history*', route => {
      const params = new URL(route.request().url()).searchParams
      const requestedPage = Number(params.get('page'))
      const size = Number(params.get('page_size'))
      requests.push({ page: requestedPage, size })
      if (failNext) {
        failNext = false
        return route.fulfill({ status: 503, json: { message: 'Fixture retry' } })
      }
      return route.fulfill({ json: { code: 0, data: {
        items: [{ id: requestedPage, code: `PAGE000${requestedPage}-CODE`, type: 'balance', value: requestedPage, status: 'used', notes: `History page ${requestedPage}`, created_at: '2026-09-01T00:00:00Z', used_at: '2026-09-01T00:00:00Z' }],
        total: 105, page: requestedPage, page_size: size, pages: Math.ceil(105 / size),
      } } })
    })
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/redeem')
    await expect(page.getByText('History page 1', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Next', exact: true }).click()
    await expect(page.getByText('History page 2', { exact: true })).toBeVisible()
    failNext = true
    await page.getByRole('button', { name: 'Next', exact: true }).click()
    await expect(page.getByText('Failed to load activity. Please try again.', { exact: true })).toBeVisible()
    await expect(page.getByText('History page 2', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Next', exact: true }).click()
    await expect(page.getByText('History page 3', { exact: true })).toBeVisible()
    await page.locator('select').selectOption('100')
    await expect(page.getByText('History page 1', { exact: true })).toBeVisible()
    expect(requests.at(-1)).toEqual({ page: 1, size: 100 })
    if (width === 1280) {
      const jump = page.locator('input[type="number"]')
      await jump.fill('2')
      await jump.press('Enter')
      await expect(page.getByText('History page 2', { exact: true })).toBeVisible()
      expect(requests.at(-1)).toEqual({ page: 2, size: 100 })
    } else {
      await page.getByRole('button', { name: 'Next', exact: true }).click()
      await expect(page.getByText('History page 2', { exact: true })).toBeVisible()
    }
    await expect(page.getByRole('button', { name: 'Previous', exact: true })).toBeEnabled()
    await page.screenshot({ path: testInfo.outputPath(`redemption-pagination-${width}.png`), animations: 'disabled' })
    expect(errors).toEqual([])
  })
}
