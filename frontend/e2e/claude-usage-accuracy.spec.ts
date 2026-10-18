import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Claude usage keeps observations and retries upstream refresh at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const account = { ...operatorFixtureAccounts[1], id: 102, platform: 'anthropic', type: 'oauth' }
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [account], total: 1, page: 1, pages: 1, page_size: 20,
    } } }))
    const refreshedAt = new Date().toISOString()
    const observedAt = '2026-10-01T10:00:00Z'
    const usage = {
      source: 'passive', cached: true, stale: true, updated_at: observedAt,
      five_hour: { utilization: 53, resets_at: '2026-10-01T15:00:00Z', source: 'passive', observed_at: observedAt, stale: true },
      seven_day: { utilization: 0.9, resets_at: null, source: 'estimated', observed_at: observedAt, estimated: true, stale: false },
      seven_day_sonnet: null,
    }
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 102: usage }, errors: {} } } }))
    let refreshes = 0
    await page.route('**/api/v1/admin/accounts/102/usage*', route => {
      expect(route.request().url()).toContain('source=active')
      expect(route.request().url()).toContain('force=true')
      refreshes++
      if (refreshes === 1) return route.fulfill({ status: 503, json: { message: 'upstream unavailable' } })
      return route.fulfill({ json: { code: 0, data: {
        source: 'active', cached: false, stale: false, updated_at: refreshedAt,
        five_hour: { utilization: 0.1, resets_at: null, source: 'active', observed_at: refreshedAt, stale: false },
      } } })
    })
    await page.goto('/admin/accounts?view=technical')
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const refresh = page.getByRole('button', { name: 'Query', exact: true })
    await refresh.scrollIntoViewIfNeeded()
    const cell = page.locator('[data-test="claude-usage-observed"]').locator('..')
    await expect(cell).toContainText('53%')
    await expect(cell).toContainText('0.9%')
    await expect(cell).toContainText('Sampled')
    await expect(cell).toContainText('Cached')
    await expect(cell).toContainText('Stale — refresh needed')
    await expect(cell).toContainText('Estimated')
    await expect(cell).not.toContainText('7d S')
    await expect(cell.locator('time').first()).toHaveAttribute('datetime', observedAt)
    await refresh.click()
    await expect(refresh).toBeEnabled()
    await expect(cell).toContainText('53%')
    await refresh.click()
    await expect(cell).toContainText('0.1%')
    await expect(cell).toContainText('Upstream')
    await expect(cell).not.toContainText('Stale — refresh needed')
    await cell.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`claude-usage-${width}.png`), animations: 'disabled' })
    expect(refreshes).toBe(2)
  })
}
