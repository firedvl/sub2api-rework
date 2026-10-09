import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixturePublicSettings } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`TypeSafe account and native key setup at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/admin/accounts?view=technical')
    await page.getByRole('button', { name: 'Create Account', exact: true }).first().click()
    const createDialog = page.getByRole('dialog', { name: 'Create Account' })
    await createDialog.getByRole('button', { name: 'TypeSafe / Jev', exact: true }).click()
    await expect(createDialog.getByPlaceholder('https://api.typesafe.ai', { exact: true })).toHaveValue('https://api.typesafe.ai')
    await expect(createDialog.getByText('jev-latest', { exact: true })).toBeVisible()
    await expect(createDialog.getByRole('button', { name: 'OAuth', exact: true })).toHaveCount(0)
    await expect(createDialog.getByText('Leave default for official Anthropic API', { exact: true })).toHaveCount(0)
    await expect(createDialog.getByText('Your Claude Console API Key', { exact: true })).toHaveCount(0)
    await page.screenshot({ path: testInfo.outputPath(`typesafe-account-${width}.png`), animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(createDialog).toBeHidden()

    const group = { id: 88, name: 'TypeSafe Fixture', platform: 'typesafe', status: 'active' }
    await page.route('**/api/v1/settings/public*', route => route.fulfill({ json: { code: 0, data: {
      ...operatorFixturePublicSettings, api_base_url: 'https://gateway.example.test/v1/'
    } } }))
    await page.route('**/api/v1/groups/available*', route => route.fulfill({ json: { code: 0, data: [group] } }))
    await page.route('**/api/v1/groups/rates*', route => route.fulfill({ json: { code: 0, data: {} } }))
    await page.route('**/api/v1/usage/dashboard/api-keys*', route => route.fulfill({ json: { code: 0, data: { stats: {} } } }))
    await page.route('**/api/v1/keys?**', route => route.fulfill({ json: { code: 0, data: {
      items: [{ id: 1, name: 'Native fixture key', key: 'sk-fixture', user_id: 2, group_id: 88, group,
        status: 'active', quota: 0, quota_used: 0, ip_whitelist: [], ip_blacklist: [],
        created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z', expires_at: null }],
      total: 1, page: 1, page_size: 10, pages: 1
    } } }))
    await page.goto('/keys')
    await page.getByRole('button', { name: 'Use Key', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Use API Key' })
    await expect(dialog.getByRole('button', { name: 'System One', exact: true })).toBeVisible()
    await expect(dialog.getByRole('button', { name: 'Codex CLI', exact: true })).toHaveCount(0)
    for (const shell of ['macOS / Linux', 'Windows CMD', 'PowerShell']) {
      await dialog.getByRole('button', { name: shell, exact: true }).click()
      await expect(dialog.locator('pre code')).toContainText('https://gateway.example.test/v1/systemone')
      await expect(dialog.locator('pre code')).toContainText('jev-latest')
      await expect(dialog.locator('pre code')).not.toContainText('/v1/v1/')
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`typesafe-key-${width}.png`), animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    expect(errors).toEqual([])
  })
}
