import { expect, test, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const fulfill = (route: Route, data: unknown) => route.fulfill({ json: { code: 0, data } })

for (const width of [390, 1280]) {
  test(`subscription bulk adjustment validates, retries and preserves partial failures at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    const rows = ['active', 'expired', 'revoked'].map((status, index) => ({
      id: index + 1, user_id: index + 11, group_id: 1, status,
      starts_at: '2026-09-01T00:00:00Z', expires_at: '2027-01-01T00:00:00Z',
      created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z',
      daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0,
      user: { id: index + 11, email: `${status}@example.test` },
      group: { id: 1, name: 'Subscription group', platform: 'openai', subscription_type: 'subscription', rate_multiplier: 1 },
    }))
    let listRequests = 0
    await page.route('**/api/v1/admin/subscriptions?**', route => {
      listRequests++
      return fulfill(route, { items: rows, total: rows.length, page: 1, page_size: 20, pages: 1 })
    })
    await page.route('**/api/v1/admin/groups*', route => fulfill(route, { items: [rows[0].group], total: 1, pages: 1 }))
    const attempts: { body: unknown; key: string | undefined }[] = []
    await page.route('**/api/v1/admin/subscriptions/bulk-action', route => {
      attempts.push({ body: route.request().postDataJSON(), key: route.request().headers()['idempotency-key'] })
      if (attempts.length === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Temporary operation failure' } })
      return fulfill(route, { success_count: 1, failed_count: 1, results: [
        { subscription_id: 1, success: true },
        { subscription_id: 2, success: false, error: 'Subscription adjustment rejected' },
      ] })
    })
    await page.goto('/admin/subscriptions')
    for (const id of [1, 2, 3]) await page.getByRole('checkbox', { name: `Select subscription #${id}`, exact: true }).check()
    await page.locator('[data-test="bulk-extend"]').click()
    const dialog = page.getByRole('dialog', { name: 'Bulk Adjust Expiration', exact: true })
    await expect(dialog).toContainText('active@example.test')
    await expect(dialog).toContainText('expired@example.test')
    await expect(dialog).not.toContainText('revoked@example.test')
    const days = dialog.getByRole('spinbutton')
    await days.fill('0')
    await expect(dialog.getByRole('button', { name: 'Confirm Action', exact: true })).toBeDisabled()
    await days.fill('7')
    await dialog.getByRole('button', { name: 'Confirm Action', exact: true }).click()
    await expect(dialog).toContainText('Temporary operation failure')
    await expect(days).toBeDisabled()
    await page.keyboard.press('Tab')
    expect(await dialog.evaluate(element => element.contains(document.activeElement))).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`subscription-bulk-retry-${width}.png`), fullPage: true, animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Retry Original Action', exact: true }).click()
    await expect(dialog).toContainText('Subscription adjustment rejected')
    expect(attempts).toHaveLength(2)
    expect(attempts[0].key).toBeTruthy()
    expect(attempts[1]).toEqual(attempts[0])
    expect(attempts[0].body).toEqual({ subscription_ids: [1, 2], action: 'extend', days: 7 })
    await expect(page.getByRole('checkbox', { name: 'Select subscription #1', exact: true })).not.toBeChecked()
    await expect(page.getByRole('checkbox', { name: 'Select subscription #2', exact: true })).toBeChecked()
    await expect(page.getByRole('checkbox', { name: 'Select subscription #3', exact: true })).toBeChecked()
    expect(listRequests).toBeGreaterThanOrEqual(2)
    await page.screenshot({ path: testInfo.outputPath(`subscription-bulk-partial-${width}.png`), fullPage: true, animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Close', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    expect(errors).toEqual([])
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
  })
}
