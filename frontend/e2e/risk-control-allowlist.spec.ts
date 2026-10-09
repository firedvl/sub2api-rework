import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`trusted risk users can be selected, saved, and removed at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    const writes: Record<string, unknown>[] = []
    let settings = { ...getOperatorFixtureData('/api/v1/admin/settings') as Record<string, unknown>, cyber_policy_user_allowlist: '12' }
    await page.route(/\/api\/v1\/admin\/settings(?:\?.*)?$/, route => {
      if (route.request().method() === 'PUT') {
        const payload = route.request().postDataJSON() as typeof settings
        writes.push(payload)
        settings = { ...settings, ...payload }
      }
      return route.fulfill({ json: { code: 0, data: settings } })
    })
    await page.route('**/api/v1/admin/settings/web-search-emulation*', route => route.fulfill({ json: { code: 0, data: { enabled: false } } }))
    await page.route('**/api/v1/admin/users/12*', route => route.fulfill({ json: { code: 0, data: { id: 12, email: 'trusted@example.test' } } }))
    await page.route('**/api/v1/admin/usage/search-users*', route => route.fulfill({ json: { code: 0, data: [{ id: 34, email: 'second-trusted@example.test' }] } }))
    await page.goto('/admin/settings')
    await page.getByRole('tab', { name: 'Feature Switches', exact: true }).click()
    const group = page.getByRole('group', { name: 'Risk control allowlist', exact: true })
    await expect(group).toContainText('trusted@example.test')
    const search = group.getByRole('textbox')
    await search.focus()
    await search.fill('second')
    await group.getByRole('button', { name: 'second-trusted@example.test #34', exact: true }).click()
    await expect(group.getByRole('button', { name: 'Remove user', exact: true })).toHaveCount(2)
    await page.getByTestId('settings-floating-save-button').click()
    await expect.poll(() => writes.length).toBe(1)
    expect(writes[0].cyber_policy_user_allowlist).toBe('12,34')
    await page.reload()
    await page.getByRole('tab', { name: 'Feature Switches', exact: true }).click()
    await expect(group.getByRole('button', { name: 'Remove user', exact: true })).toHaveCount(2)
    await group.getByRole('button', { name: 'Remove user', exact: true }).first().focus()
    await page.keyboard.press('Enter')
    await group.getByRole('button', { name: 'Remove user', exact: true }).click()
    await page.getByTestId('settings-floating-save-button').click()
    await expect.poll(() => writes.length).toBe(2)
    expect(writes[1].cyber_policy_user_allowlist).toBe('')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    expect(errors).toEqual([])
    await group.screenshot({ path: testInfo.outputPath(`risk-allowlist-${width}.png`) })
  })
}
