import { expect, test, type Page, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const users = [1, 2].map(id => ({
  id, username: `user-${id}`, email: `user-${id}@example.test`, role: 'user',
  balance: 1, concurrency: 1, status: 'active', allowed_groups: [7], group_rates: { 7: 0.5 },
  created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', notes: '',
}))
const fulfill = (route: Route, data: unknown) => route.fulfill({
  status: 200, contentType: 'application/json', body: JSON.stringify({ code: 0, message: 'ok', data }),
})

async function openUsers(page: Page) {
  await seedSession(page)
  await installOperatorApiMock(page)
  await page.route('**/api/v1/admin/user-attributes?**', route => fulfill(route, []))
  await page.route('**/api/v1/admin/dashboard/users-usage**', route => fulfill(route, { stats: {} }))
  await page.route('**/api/v1/admin/users?**', route => fulfill(route, {
    items: users, total: 2, page: 1, page_size: 20, pages: 1,
  }))
  await page.goto('/admin/users')
  await expect(page.getByText(users[0].email, { exact: true })).toBeVisible()
}

async function openAction(page: Page, id: number, action: string) {
  const row = page.locator('tr').filter({ hasText: users[id - 1].email })
  await row.getByRole('button', { name: 'More', exact: true }).click()
  await page.getByRole('button', { name: action, exact: true }).click()
}

test('history ignores a closed user request after another user opens', async ({ page }) => {
  await openUsers(page)
  let oldRequest: Route | undefined
  await page.route('**/api/v1/admin/users/*/balance-history?**', async route => {
    if (new URL(route.request().url()).pathname.includes('/1/')) {
      oldRequest = route
      return
    }
    await fulfill(route, {
      items: [{ id: 20, type: 'admin_balance', value: 20, notes: 'Current user history' }],
      total: 1, total_recharged: 20,
    })
  })
  await openAction(page, 1, 'Recharge History')
  const dialog = page.getByRole('dialog', { name: 'User Recharge & Concurrency History' })
  await expect(dialog).toBeVisible()
  await expect.poll(() => Boolean(oldRequest)).toBe(true)
  await page.keyboard.press('Escape')
  await expect(dialog).not.toBeVisible()
  await openAction(page, 2, 'Recharge History')
  await expect(dialog).toContainText('Current user history')
  await fulfill(oldRequest!, {
    items: [{ id: 10, type: 'admin_balance', value: 10, notes: 'Old user history' }],
    total: 1, total_recharged: 10,
  })
  await expect(dialog).toContainText('Current user history')
  await expect(dialog).not.toContainText('Old user history')
})

test('group configuration cannot save before the groups arrive', async ({ page }) => {
  await openUsers(page)
  let groupRequest: Route | undefined
  let update: unknown
  await page.route('**/api/v1/admin/groups?**', route => { groupRequest = route })
  await page.route('**/api/v1/admin/users/1', async route => {
    update = route.request().postDataJSON()
    await fulfill(route, users[0])
  })
  await openAction(page, 1, 'Groups')
  const dialog = page.getByRole('dialog', { name: 'User Group Configuration' })
  const save = dialog.getByRole('button', { name: 'Save', exact: true })
  await expect(save).toBeDisabled()
  expect(update).toBeUndefined()
  await expect.poll(() => Boolean(groupRequest)).toBe(true)
  await fulfill(groupRequest!, { items: [{
    id: 7, name: 'Exclusive', platform: 'openai', subscription_type: 'standard',
    status: 'active', is_exclusive: true, rate_multiplier: 1,
  }] })
  await expect(save).toBeEnabled()
  await save.click()
  await expect.poll(() => update).toEqual({
    allowed_groups: [7], restrict_public_groups: false, group_rates: { 7: 0.5 },
  })
  await expect(dialog).not.toBeVisible()
})
