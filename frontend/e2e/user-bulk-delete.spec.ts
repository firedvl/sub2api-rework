import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`selected-user deletion confirms and retains failures at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route(/\/api\/v1\/admin\/user-attributes(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: [] } }))
    let users = [42, 43].map(id => ({ id, email: `delete-${id}@example.test`, username: `User ${id}`, role: 'user', status: 'active',
      current_concurrency: 0, concurrency: 5, balance: 1, allowed_groups: [], subscriptions: [], notes: '',
      balance_notify_enabled: false, balance_notify_threshold: null, balance_notify_extra_emails: [],
      created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' }))
    await page.route(/\/api\/v1\/admin\/users(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: {
      items: users, total: users.length, pages: 1, page: 1, page_size: 20,
    } } }))
    const deleted: number[] = []
    await page.route(/\/api\/v1\/admin\/users\/(42|43)$/, route => {
      expect(route.request().method()).toBe('DELETE')
      const id = Number(new URL(route.request().url()).pathname.split('/').pop())
      deleted.push(id)
      if (id === 43 && deleted.filter(value => value === 43).length === 1) {
        return route.fulfill({ status: 503, json: { code: 503, message: 'Fixture deletion unavailable' } })
      }
      users = users.filter(user => user.id !== id)
      return route.fulfill({ json: { code: 0, data: null } })
    })
    await page.goto('/admin/users')
    await expect(page.getByText('delete-42@example.test', { exact: true })).toBeVisible()
    await page.getByRole('checkbox').first().check()
    await page.locator('[data-test="bulk-delete-users"]').click()
    const dialog = page.getByRole('dialog', { name: 'Delete selected users', exact: true })
    await expect(dialog).toBeVisible()
    expect(deleted).toEqual([])
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
    expect(deleted).toEqual([])
    await page.locator('[data-test="bulk-delete-users"]').click()
    await dialog.getByRole('button', { name: 'Delete', exact: true }).click()
    await expect(page.getByText('delete-42@example.test', { exact: true })).toHaveCount(0)
    await expect(page.getByText('delete-43@example.test', { exact: true })).toBeVisible()
    expect(deleted).toEqual([42, 43])
    await expect(page.locator('[data-test="bulk-delete-users"]')).toContainText('1')
    await page.screenshot({ path: testInfo.outputPath(`user-delete-partial-${width}.png`), animations: 'disabled' })
    await page.locator('[data-test="bulk-delete-users"]').click()
    await dialog.getByRole('button', { name: 'Delete', exact: true }).click()
    await expect(page.locator('[data-test="bulk-delete-users"]')).toHaveCount(0)
    expect(deleted).toEqual([42, 43, 43])
    expect(errors).toEqual([])
  })
}
