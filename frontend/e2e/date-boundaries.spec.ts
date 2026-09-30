import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`admin date inputs reject unrepresentable years at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.route('**/api/v1/admin/announcements?**', route => route.fulfill({
      json: { code: 0, data: { items: [], total: 0, page: 1, page_size: 20, pages: 0 } },
    }))
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/admin/announcements')
    await page.getByRole('button', { name: 'Create Announcement', exact: true }).first().click()
    const announcement = page.getByRole('dialog', { name: 'Create Announcement' })
    const schedule = announcement.locator('input[type="datetime-local"]')
    await expect(schedule).toHaveCount(2)
    for (const input of await schedule.all()) {
      await input.fill('9999-12-31T23:59')
      expect(await input.evaluate(element => (element as HTMLInputElement).checkValidity())).toBe(true)
      await input.fill('10000-01-01T00:00')
      expect(await input.evaluate(element => (element as HTMLInputElement).checkValidity())).toBe(false)
    }
    await page.screenshot({ path: `test-results/announcement-date-${width}.png`, animations: 'disabled' })
    await page.goto('/admin/proxies')
    await page.getByRole('button', { name: 'Create Proxy', exact: true }).click()
    const expiry = page.getByRole('dialog', { name: 'Create Proxy' }).locator('input[type="date"]')
    await expiry.fill('9999-12-31')
    expect(await expiry.evaluate(element => (element as HTMLInputElement).checkValidity())).toBe(true)
    await expiry.fill('10000-01-01')
    expect(await expiry.evaluate(element => (element as HTMLInputElement).checkValidity())).toBe(false)
    await page.screenshot({ path: `test-results/proxy-date-${width}.png`, animations: 'disabled' })
    expect(errors).toEqual([])
  })
}
