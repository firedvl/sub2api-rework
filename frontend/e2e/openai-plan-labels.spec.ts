import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`OpenAI plan labels and manual presets preserve canonical values at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    const account = { ...operatorFixtureAccounts[0], credentials: { plan_type: 'CHATGPT_PRO' } }
    const writes: unknown[] = []
    await page.route('**/api/v1/admin/accounts*', route => {
      const path = new URL(route.request().url()).pathname
      if (path !== '/api/v1/admin/accounts') return route.fallback()
      return route.fulfill({ json: { code: 0, data: { items: [account], total: 1, page: 1, page_size: 50, pages: 1 } } })
    })
    await page.route('**/api/v1/admin/accounts/101*', route => {
      if (new URL(route.request().url()).pathname !== '/api/v1/admin/accounts/101') return route.fallback()
      if (route.request().method() === 'GET') return route.fulfill({ json: { code: 0, data: account } })
      writes.push(route.request().postDataJSON())
      return route.fulfill({ json: { code: 0, data: account } })
    })
    await page.goto('/admin/accounts?view=technical')
    await expect(page.getByText('Pro 200', { exact: true }).first()).toBeVisible()
    await page.getByRole('button', { name: 'Edit', exact: true }).first().click()
    const dialog = page.getByRole('dialog', { name: 'Edit Account', exact: true })
    const plan = dialog.locator('.select-trigger').filter({ hasText: 'Pro 200' })
    await expect(plan).toBeVisible()
    await plan.click()
    await expect(page.getByRole('option', { name: 'Pro 200', exact: true })).toHaveCount(1)
    await expect(page.getByRole('option', { name: 'Pro 100', exact: true })).toBeVisible()
    await expect(page.getByRole('option', { name: 'Pro 500', exact: true })).toBeVisible()
    await expect(page.getByRole('option', { name: 'Business Premium', exact: true })).toBeVisible()
    await page.getByRole('option', { name: 'Pro 100', exact: true }).click()
    await expect(dialog.locator('.select-trigger').filter({ hasText: 'Pro 100' })).toBeVisible()
    await page.screenshot({ path: testInfo.outputPath(`openai-plans-${width}.png`), animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
    await expect(dialog).not.toBeVisible()
    expect(writes).toEqual([])
  })
}
