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

for (const width of [390, 1280]) {
  test(`Claude Capacity excludes historical samples and keeps Query results across views at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    const now = new Date('2026-10-18T12:00:00Z')
    await page.clock.install({ time: now })
    await page.addInitScript(() => localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 })))
    await seedSession(page)
    await installOperatorApiMock(page)
    const account = { ...operatorFixtureAccounts[1], id: 102, name: 'Claude recovery', platform: 'anthropic', type: 'oauth' }
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [account], total: 1, page: 1, pages: 1, page_size: 20,
    } } }))
    let usage = {
      source: 'active', cached: true, stale: true, updated_at: '2026-10-17T10:00:00Z',
      five_hour: { utilization: 96, source: 'active', observed_at: '2026-10-17T10:00:00Z', stale: true, resets_at: '2026-10-17T15:00:00Z' },
      seven_day: { utilization: 37, source: 'active', observed_at: '2026-10-17T10:00:00Z', stale: true, resets_at: '2026-10-20T15:00:00Z' },
    }
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 102: usage }, errors: {} } } }))
    let queries = 0
    await page.route('**/api/v1/admin/accounts/102/usage*', route => {
      const url = new URL(route.request().url())
      if (url.searchParams.get('source') !== 'active') return route.fulfill({ json: { code: 0, data: usage } })
      expect(url.searchParams.get('force')).toBe('true')
      queries++
      if (queries === 1) return route.fulfill({ status: 503, json: { message: 'upstream unavailable' } })
      usage = {
        source: 'active', cached: false, stale: false, updated_at: now.toISOString(),
        five_hour: { utilization: 50, source: 'active', observed_at: now.toISOString(), stale: false, resets_at: '2026-10-18T17:00:00Z' },
        seven_day: { utilization: 46, source: 'active', observed_at: now.toISOString(), stale: false, resets_at: '2026-10-20T15:00:00Z' },
      }
      return route.fulfill({ json: { code: 0, data: usage } })
    })
    await page.goto('/admin/accounts?view=capacity')
    const panel = page.getByRole('tabpanel', { name: 'Capacity', exact: true })
    const row = panel.getByTestId('account-capacity-row')
    const provider = panel.getByTestId('provider-toggle-anthropic')
    await expect(provider).toContainText('0 with quota')
    await expect(provider).toContainText('1 unknown')
    await expect(row).toContainText('Stale — refresh needed')
    await expect(row).toHaveAttribute('data-status', 'active')
    const query = row.getByRole('button', { name: 'Query Claude recovery', exact: true })
    await query.focus()
    await page.keyboard.press('Enter')
    await expect(query).toBeEnabled()
    await row.getByRole('button', { name: 'Show details for Claude recovery' }).click()
    await expect(row).toContainText('upstream unavailable')
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    await expect(page.getByRole('tabpanel', { name: 'Technical', exact: true })).toContainText('upstream unavailable')
    await page.getByRole('tab', { name: 'Capacity', exact: true }).click()
    await query.click()
    await expect(provider).toContainText('1 with quota')
    await expect(row).toContainText('50%')
    await expect(row).not.toContainText('Stale — refresh needed')
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const technical = page.getByRole('tabpanel', { name: 'Technical', exact: true })
    await expect(technical).toContainText('50%')
    const technicalQuery = technical.getByRole('button', { name: 'Query', exact: true })
    await technicalQuery.click()
    await expect(technicalQuery).toBeEnabled()
    await page.clock.fastForward(10_000)
    await expect(technical).toContainText('50%')
    await expect(technical).not.toContainText('96%')
    await page.getByRole('tab', { name: 'Capacity', exact: true }).click()
    expect(queries).toBe(3)
    await page.clock.fastForward(180_000)
    await expect(provider).toContainText('0 with quota')
    await expect(row).toContainText('Stale — refresh needed')
    expect(queries).toBe(3)
    await expect(page.locator('body')).toHaveJSProperty('scrollWidth', width)
    await row.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`claude-capacity-${width}.png`), animations: 'disabled' })
  })
}
