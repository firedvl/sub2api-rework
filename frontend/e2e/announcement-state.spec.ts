import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`announcement bulk read retains partial success and retries only failures at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const notices = [1, 2].map(id => ({
      id, title: `Notice ${id}`, content: 'Fixture content', notify_mode: 'silent',
      created_at: '2026-09-18T00:00:00Z', updated_at: '2026-09-18T00:00:00Z',
    }))
    const submitted: number[] = []
    let failed = false
    await page.route('**/api/v1/announcements*', route =>
      route.fulfill({ json: { code: 0, data: notices } }))
    await page.route('**/api/v1/announcements/*/read', route => {
      const id = Number(new URL(route.request().url()).pathname.split('/').at(-2))
      submitted.push(id)
      if (id === 2 && !failed) {
        failed = true
        return route.fulfill({ status: 503, json: { code: 503, message: 'read fixture failed' } })
      }
      return route.fulfill({ json: { code: 0, data: { message: 'ok' } } })
    })
    await page.goto('/dashboard')
    await page.getByRole('button', { name: 'Announcements', exact: true }).click()
    const markAll = page.getByRole('button', { name: 'Mark all as read', exact: true })
    await markAll.click()
    await expect(page.getByText('read fixture failed', { exact: true })).toBeVisible()
    await expect(page.locator('p').filter({ hasText: /^1\s+Unread$/ })).toBeVisible()
    expect(submitted.sort()).toEqual([1, 2])
    await markAll.click()
    await expect(page.getByText('All announcements marked as read', { exact: true })).toBeVisible()
    expect(submitted).toEqual([1, 2, 2])
    await expect(markAll).toBeHidden()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('heading', { name: 'Announcements', exact: true })).toBeHidden()
  })
}
