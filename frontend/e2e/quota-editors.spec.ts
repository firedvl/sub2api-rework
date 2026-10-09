import { expect, test, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureUser } from './fixtures/operatorData'

const fulfill = (route: Route, data: unknown) => route.fulfill({ json: { code: 0, data } })
const platforms = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe']

for (const width of [390, 1280]) {
  test(`quota editor preserves all supported limits at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/admin/redeem-codes?**', route => fulfill(route, {
      items: [], total: 0, page: 1, page_size: 20, pages: 0,
    }))
    await page.route('**/api/v1/admin/user-attributes?**', route => fulfill(route, []))
    await page.route('**/api/v1/admin/dashboard/users-usage**', route => fulfill(route, { stats: {} }))
    const user = { ...operatorFixtureUser('user'), id: 99, email: 'quota@example.test', status: 'active' }
    await page.route('**/api/v1/admin/users?**', route => fulfill(route, {
      items: [user], total: 1, page: 1, page_size: 20, pages: 1,
    }))
    let saved: unknown
    await page.route('**/api/v1/admin/users/99/platform-quotas*', route => {
      if (route.request().method() === 'PUT') {
        saved = route.request().postDataJSON()
        return fulfill(route, {})
      }
      return fulfill(route, { platform_quotas: platforms.map(platform => ({
        platform, daily_limit_usd: 0, weekly_limit_usd: null, monthly_limit_usd: 50,
      })) })
    })
    await page.goto('/admin/users')
    await page.getByRole('button', { name: 'More', exact: true }).click()
    await page.getByRole('button', { name: 'Platform Quotas', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Platform Quotas' })
    await expect(dialog.locator('tbody tr')).toHaveCount(platforms.length)
    const deepseek = dialog.locator('tr').filter({ has: page.locator('td').filter({ hasText: /^deepseek$/ }) })
    await deepseek.locator('input').nth(1).fill('12.5')
    const typesafe = dialog.locator('tr').filter({ has: page.locator('td').filter({ hasText: /^typesafe$/ }) })
    await typesafe.locator('input').nth(0).fill('3.25')
    await page.screenshot({ path: `test-results/quota-platforms-${width}.png`, fullPage: true, animations: 'disabled' })
    await dialog.getByRole('button', { name: 'Save', exact: true }).click()
    await expect.poll(() => saved).toEqual({ quotas: platforms.map(platform => ({
      platform, daily_limit_usd: platform === 'typesafe' ? 3.25 : 0, weekly_limit_usd: platform === 'deepseek' ? 12.5 : null, monthly_limit_usd: 50,
    })) })
    await expect(dialog).not.toBeVisible()
    await page.goto('/admin/redeem')
    await page.getByRole('button', { name: 'Generate Codes', exact: true }).click()
    const generator = page.locator('.fixed.inset-0.z-50').filter({ hasText: 'Generate Redeem Codes' })
    await generator.locator('.select-trigger').first().click()
    await page.getByRole('option', { name: 'Subscription', exact: true }).click()
    const duration = generator.locator('input[max="36500"]')
    await duration.fill('36500')
    expect(await duration.evaluate(input => (input as HTMLInputElement).checkValidity())).toBe(true)
    await duration.fill('36501')
    expect(await duration.evaluate(input => (input as HTMLInputElement).checkValidity())).toBe(false)
    await duration.fill('366')
    expect(await duration.evaluate(input => (input as HTMLInputElement).checkValidity())).toBe(true)
    await page.screenshot({ path: `test-results/quota-duration-${width}.png`, fullPage: true, animations: 'disabled' })
    expect(errors).toEqual([])
  })

  test(`API key reset updates status without erasing draft name at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const key = { id: 1, user_id: 1, key: 'sk-fixture', name: 'Quota key', status: 'quota_exhausted',
      group_id: 1, quota: 10, quota_used: 10, ip_whitelist: [], ip_blacklist: [], expires_at: null,
      rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, last_used_at: null,
      created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z' }
    await page.route('**/api/v1/keys?**', route => fulfill(route, { items: [key], total: 1, page: 1, page_size: 20, pages: 1 }))
    await page.route('**/api/v1/groups/available*', route => fulfill(route, [
      { id: 1, name: 'Fixture', platform: 'openai', status: 'active', subscription_type: 'standard', rate_multiplier: 1 },
    ]))
    await page.route('**/api/v1/usage/dashboard/api-keys-usage', route => fulfill(route, { stats: {} }))
    const writes: Record<string, unknown>[] = []
    await page.route('**/api/v1/keys/1', route => {
      writes.push(route.request().postDataJSON())
      return fulfill(route, { ...key, status: 'active', quota_used: 0 })
    })
    await page.goto('/keys')
    await page.getByRole('button', { name: 'Edit', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Edit API Key' })
    await dialog.locator('#key-form input[type="text"]').fill('Draft name')
    await dialog.getByTitle('Reset used quota to 0', { exact: true }).click()
    await page.getByRole('dialog', { name: 'Confirm Reset Quota' }).getByRole('button', { name: 'Reset', exact: true }).click()
    await expect.poll(() => writes.length).toBe(1)
    expect(writes[0]).toEqual({ reset_quota: true })
    await expect(page.getByRole('dialog', { name: 'Confirm Reset Quota' })).not.toBeVisible()
    await expect(dialog.locator('#key-form input[type="text"]')).toHaveValue('Draft name')
    await expect(dialog.locator('.select-trigger').last()).toContainText('Active')
    await page.screenshot({ path: `test-results/quota-key-reset-${width}.png`, fullPage: true, animations: 'disabled' })
    await dialog.locator('button[form="key-form"]').click()
    await expect.poll(() => writes.length).toBe(2)
    expect(writes[1]).toMatchObject({ name: 'Draft name', status: 'active' })
    await expect(dialog).not.toBeVisible()
  })
}
