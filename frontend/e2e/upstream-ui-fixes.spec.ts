import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`idle reset countdown and group search lifecycle at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.route('**/api/v1/admin/users?*', route => route.fulfill({ json: { code: 0, data: {
      items: [{ id: 1, username: 'operator', email: 'operator@example.test', status: 'active' }],
      total: 1, page: 1, pages: 1, page_size: 10,
    } } }))
    await page.route('**/api/v1/admin/groups/*/rate-multipliers*', route => route.fulfill({ json: { code: 0, data: [] } }))
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => {
      if (message.type() === 'error') errors.push(message.text())
    })
    page.on('response', response => {
      if (response.status() >= 400) errors.push(`${response.status()} ${response.url()}`)
    })
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [operatorFixtureAccounts[0]], total: 1, page: 1, pages: 1, page_size: 20,
    } } }))
    const usage = {
      five_hour: { utilization: 0, resets_at: null },
      seven_day: { utilization: 0, resets_at: new Date(Date.now() + 90 * 60_000).toISOString() },
    }
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 101: usage }, errors: {} } } }))
    await page.route('**/api/v1/admin/accounts/101/usage*', route => route.fulfill({ json: { code: 0, data: usage } }))
    await page.goto('/admin/accounts?view=technical')
    const sevenDay = page.locator('.flex.items-center.gap-1').filter({ has: page.getByText('7d', { exact: true }) })
    await expect(sevenDay).toContainText(/1h (29|30)m/)
    await sevenDay.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`idle-reset-${width}.png`), animations: 'disabled' })

    await page.goto('/admin/groups')
    for (const name of ['RPM Overrides', 'Rate Multipliers']) {
      await page.getByRole('button', { name, exact: true }).first().click()
      const dialog = page.getByRole('dialog')
      const search = dialog.getByPlaceholder('Search user email...')
      await search.fill('operator')
      await expect(dialog.getByText('operator@example.test', { exact: true }).first()).toBeVisible()
      await expect(page.getByText('Failed to load groups', { exact: true })).toHaveCount(0)
      await page.screenshot({ path: testInfo.outputPath(`${name.replaceAll(' ', '-')}-${width}.png`), animations: 'disabled' })
      await page.keyboard.press('Escape')
      await expect(dialog).toBeHidden()
    }
    expect(errors).toEqual([])
  })
}
