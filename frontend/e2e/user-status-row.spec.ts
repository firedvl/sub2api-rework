import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`user status changes retain the row and do not reload at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route(/\/api\/v1\/admin\/user-attributes(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: [] } }))
    const row = { id: 42, email: 'status-fixture@example.test', username: 'Status Fixture', role: 'user', status: 'active',
      current_concurrency: 3, concurrency: 5, balance: 1, allowed_groups: [], subscriptions: [], notes: '',
      balance_notify_enabled: false, balance_notify_threshold: null, balance_notify_extra_emails: [],
      created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' }
    let listRequests = 0
    await page.route(/\/api\/v1\/admin\/users(?:\?.*)?$/, route => {
      listRequests++
      return route.fulfill({ json: { code: 0, data: { items: [row], total: 1, pages: 1, page: 1, page_size: 20 } } })
    })
    let updates = 0
    await page.route('**/api/v1/admin/users/42', route => {
      updates++
      if (updates === 3) return route.fulfill({ status: 503, json: { code: 503, message: 'Status update unavailable' } })
      row.status = route.request().postDataJSON().status
      return route.fulfill({ json: { code: 0, data: { id: row.id, status: row.status, updated_at: '2026-10-01T00:00:00Z' } } })
    })
    await page.goto('/admin/users')
    await expect(page.getByText(row.email, { exact: true })).toBeVisible()
    const before = listRequests
    await page.getByRole('button', { name: 'Disable', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Enable', exact: true })).toBeVisible()
    expect(listRequests).toBe(before)
    await expect(page.getByText(row.email, { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Enable', exact: true }).click()
    await expect(page.getByRole('button', { name: 'Disable', exact: true })).toBeVisible()
    expect(listRequests).toBe(before)
    await page.getByRole('button', { name: 'Disable', exact: true }).click()
    await expect(page.getByText('Failed to update user status', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Disable', exact: true })).toBeVisible()
    expect(listRequests).toBe(before)
    await page.screenshot({ path: testInfo.outputPath(`user-status-${width}.png`), fullPage: true, animations: 'disabled' })
    expect(errors).toEqual([])
  })
}
