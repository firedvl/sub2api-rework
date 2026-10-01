import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`key bulk edit preserves partial failures at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    await page.route('**/api/v1/groups/available*', route => route.fulfill({ json: { code: 0, data: [] } }))
    await page.route('**/api/v1/groups/rates*', route => route.fulfill({ json: { code: 0, data: {} } }))
    const keys = [1, 2].map(id => ({
      id, name: `Key ${id}`, user_id: 1, key: `sk-fixture-${id}`, group_id: null,
      status: 'active', quota: 100, quota_used: 12, rate_limit_5h: 10,
      rate_limit_1d: 20, rate_limit_7d: 30, usage_5h: 2, usage_1d: 3, usage_7d: 4,
      ip_whitelist: [], ip_blacklist: [], created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z', expires_at: null,
    }))
    let reads = 0
    const writes: Array<{ id: number; updates: unknown }> = []
    await page.route('**/api/v1/keys?**', route => {
      reads++
      return route.fulfill({ json: { code: 0, data: { items: keys, total: 2, page: 1, page_size: 10, pages: 1 } } })
    })
    await page.route('**/api/v1/usage/dashboard/api-keys*', route => route.fulfill({ json: { code: 0, data: { stats: {} } } }))
    await page.route('**/api/v1/keys/*', async route => {
      if (route.request().method() !== 'PUT') return route.fallback()
      const id = Number(new URL(route.request().url()).pathname.split('/').pop())
      const updates = route.request().postDataJSON()
      writes.push({ id, updates })
      if (id === 2 && writes.filter(write => write.id === 2).length === 1) {
        return route.fulfill({ status: 400, json: { code: 'INVALID_INPUT', message: 'Fixture key rejected' } })
      }
      Object.assign(keys.find(key => key.id === id)!, updates)
      return route.fulfill({ json: { code: 0, data: keys.find(key => key.id === id) } })
    })
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.goto('/keys')
    await page.getByRole('checkbox', { name: 'Select key Key 1', exact: true }).check()
    await page.getByRole('checkbox', { name: 'Select key Key 2', exact: true }).check()
    await page.locator('[data-test="bulk-edit-keys"]').click()
    const dialog = page.getByRole('dialog', { name: 'Bulk Edit', exact: true })
    await expect(dialog.getByRole('button', { name: 'Close modal' })).toBeFocused()
    await page.keyboard.press('Tab')
    await expect(dialog.locator('[data-test="enable-group"]')).toBeFocused()
    const submit = dialog.locator('[data-test="submit"]')
    await expect(submit).toBeDisabled()
    await dialog.locator('[data-test="enable-quota"]').check()
    await dialog.locator('[data-test="quota-input"]').fill('-1')
    await expect(submit).toBeDisabled()
    await dialog.locator('[data-test="quota-input"]').fill('0')
    await dialog.locator('[data-test="enable-rate_limit_1d"]').check()
    await dialog.locator('[data-test="rate_limit_1d-input"]').fill('0')
    await submit.click()
    await expect(dialog.getByText('#2 Key 2: Fixture key rejected', { exact: true })).toBeVisible()
    await expect(submit).toHaveText('Apply to 1 keys')
    expect(writes).toEqual([1, 2].map(id => ({ id, updates: { quota: 0, rate_limit_1d: 0 } })))
    expect(keys[0].quota_used).toBe(12)
    expect(keys[0].rate_limit_5h).toBe(10)
    expect(keys[0].usage_1d).toBe(3)
    await expect.poll(() => reads).toBeGreaterThan(1)
    expect(await dialog.evaluate(element => element.scrollWidth <= element.clientWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`key-bulk-partial-${width}.png`), animations: 'disabled' })
    await submit.click()
    await expect(dialog).not.toBeVisible()
    expect(writes.map(write => write.id)).toEqual([1, 2, 2])
    await expect(page.locator('[data-test="bulk-edit-keys"]')).toHaveCount(0)
    await expect.poll(() => reads).toBeGreaterThan(2)
    expect(errors).toEqual([])
  })
}
