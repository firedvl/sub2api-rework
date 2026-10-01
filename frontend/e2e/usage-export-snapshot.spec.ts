import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`usage export retains its first filter snapshot at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    const fixture = getOperatorFixtureData('/api/v1/admin/usage') as { items: Record<string, unknown>[] }
    await page.route('**/api/v1/groups/available*', route => route.fulfill({ json: { code: 0, data: [] } }))
    await page.route('**/api/v1/usage/stats*', route => route.fulfill({ json: { code: 0, data: getOperatorFixtureData('/api/v1/admin/usage/stats') } }))
    await page.route('**/api/v1/usage/dashboard/*', route => route.fulfill({ json: { code: 0, data: { trend: [], groups: [], models: [] } } }))
    const exportQueries: URLSearchParams[] = []
    let release!: () => void
    const held = new Promise<void>(resolve => { release = resolve })
    await page.route(/\/api\/v1\/usage(?:\?.*)?$/, async route => {
      const params = new URL(route.request().url()).searchParams
      if (params.get('page_size') === '100') {
        exportQueries.push(params)
        if (params.get('page') === '1') await held
      }
      await route.fulfill({ json: { code: 0, data: { items: [fixture.items[0]], total: 101, pages: 2, page: Number(params.get('page')), page_size: Number(params.get('page_size')) } } })
    })
    await page.goto('/usage')
    await expect(page.getByRole('button', { name: 'Export CSV', exact: true })).toBeEnabled()
    await expect(page.getByText(String(fixture.items[0].model), { exact: true }).first()).toBeVisible()
    const downloadPromise = page.waitForEvent('download')
    await page.getByRole('button', { name: 'Export CSV', exact: true }).click()
    await expect.poll(() => exportQueries.length).toBe(1)
    await page.locator('label').filter({ hasText: /^Type$/ }).locator('..').getByRole('button').click()
    await page.getByRole('option', { name: 'Sync', exact: true }).click()
    release()
    const download = await downloadPromise
    expect(exportQueries).toHaveLength(2)
    const initial = Object.fromEntries(exportQueries[0])
    expect(Object.fromEntries(exportQueries[1])).toEqual({ ...initial, page: '2' })
    expect(exportQueries[1].get('request_type')).toBeNull()
    expect(download.suggestedFilename()).toBe(`usage_${initial.start_date}_to_${initial.end_date}.csv`)
    expect(errors).toEqual([])
  })
}
