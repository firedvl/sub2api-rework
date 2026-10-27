import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Claude usage shows observation provenance without a Query action at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const account = { ...operatorFixtureAccounts[1], id: 102, platform: 'anthropic', type: 'oauth' }
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: {
      items: [account], total: 1, page: 1, pages: 1, page_size: 20,
    } } }))
    const observedAt = '2026-10-01T10:00:00Z'
    const usage = {
      source: 'passive', cached: true, stale: true, updated_at: observedAt,
      five_hour: { utilization: 53, resets_at: '2026-10-01T15:00:00Z', source: 'passive', observed_at: observedAt, stale: true },
      seven_day: { utilization: 0.9, resets_at: null, source: 'estimated', observed_at: observedAt, estimated: true, stale: false },
      seven_day_sonnet: null,
    }
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 102: usage }, errors: {} } } }))
    const activeQueries: string[] = []
    await page.route('**/api/v1/admin/accounts/102/usage*', route => {
      activeQueries.push(route.request().url())
      return route.fulfill({ json: { code: 0, data: usage } })
    })
    await page.goto('/admin/accounts?view=technical')
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const cell = page.locator('[data-test="claude-usage-observed"]').locator('..')
    await expect(cell).toContainText('53%')
    await expect(cell).toContainText('0.9%')
    await expect(cell).toContainText('Sampled')
    await expect(cell).toContainText('Cached')
    await expect(cell).toContainText('Stale — refresh needed')
    await expect(cell).toContainText('Estimated')
    await expect(cell).not.toContainText('7d S')
    await expect(cell.locator('time').first()).toHaveAttribute('datetime', observedAt)
    await expect(page.getByRole('button', { name: 'Query', exact: true })).toHaveCount(0)
    await cell.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`claude-usage-${width}.png`), animations: 'disabled' })
    expect(activeQueries.filter(url => url.includes('force=true'))).toEqual([])
  })
}

for (const width of [390, 1280]) {
  test(`Claude Capacity excludes historical samples without a Query action at ${width}px`, async ({ page }, testInfo) => {
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
    const usage = {
      source: 'active', cached: true, stale: true, updated_at: '2026-10-17T10:00:00Z',
      five_hour: { utilization: 96, source: 'active', observed_at: '2026-10-17T10:00:00Z', stale: true, resets_at: '2026-10-17T15:00:00Z' },
      seven_day: { utilization: 37, source: 'active', observed_at: '2026-10-17T10:00:00Z', stale: true, resets_at: '2026-10-20T15:00:00Z' },
    }
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 102: usage }, errors: {} } } }))
    let forcedQueries = 0
    await page.route('**/api/v1/admin/accounts/102/usage*', route => {
      if (new URL(route.request().url()).searchParams.get('force') === 'true') forcedQueries++
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
    await expect(row.getByRole('button', { name: 'Query Claude recovery', exact: true })).toHaveCount(0)
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const technical = page.getByRole('tabpanel', { name: 'Technical', exact: true })
    await expect(technical.getByRole('button', { name: 'Query', exact: true })).toHaveCount(0)
    await page.getByRole('tab', { name: 'Capacity', exact: true }).click()
    await page.clock.fastForward(180_000)
    await expect(provider).toContainText('0 with quota')
    await expect(row).toContainText('Stale — refresh needed')
    expect(forcedQueries).toBe(0)
    await expect(page.locator('body')).toHaveJSProperty('scrollWidth', width)
    await row.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`claude-capacity-${width}.png`), animations: 'disabled' })
  })
}

for (const width of [390, 1280]) {
  test(`Accounts refresh updates the filtered Claude fleet without Query at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    const now = new Date('2026-10-18T12:00:00Z')
    await page.clock.install({ time: now })
    await page.addInitScript(() => localStorage.setItem('account-auto-refresh', JSON.stringify({ enabled: true, interval_seconds: 5 })))
    await seedSession(page)
    await installOperatorApiMock(page)
    const accounts = [102, 103, 104, 105].map((id, index) => ({
      ...operatorFixtureAccounts[1], id, name: `Claude fleet ${id}`, platform: 'anthropic',
      type: index === 2 ? 'setup-token' : index === 3 ? 'apikey' : 'oauth',
      status: 'active', rate_limited_at: null, rate_limit_reset_at: null,
      overload_until: null, temp_unschedulable_until: null,
    }))
    await page.route('**/api/v1/admin/accounts?*', route => {
      const url = new URL(route.request().url())
      const fleet = url.searchParams.get('page_size') === '1000'
      const secondPage = url.searchParams.get('page') === '2'
      return route.fulfill({ json: { code: 0, data: {
        items: fleet ? secondPage ? accounts.slice(2) : accounts.slice(0, 2) : accounts,
        total: 4, page: secondPage ? 2 : 1, pages: fleet ? 2 : 1, page_size: fleet ? 1000 : 20,
      } } })
    })
    const makeUsage = (utilization: number, source = 'active') => ({
      source, cached: false, stale: false, updated_at: now.toISOString(),
      five_hour: { utilization, source, observed_at: now.toISOString(), stale: false, resets_at: '2026-10-18T17:00:00Z' },
      seven_day: null, seven_day_sonnet: null,
    })
    const requests: Array<{ account_ids: number[]; force: boolean }> = []
    let manualDone = false
    let automaticDone = false
    await page.route('**/api/v1/admin/accounts/usage/batch', route => {
      const body = route.request().postDataJSON()
      requests.push(body)
      expect(body.account_ids.slice().sort()).toEqual([102, 103, 104])
      if (body.force) {
        manualDone = true
        return route.fulfill({ json: { code: 0, data: {
          usage: { 102: makeUsage(25), 103: { ...makeUsage(60), cached: true }, 104: makeUsage(70, 'passive') },
          errors: { 103: 'Claude refresh temporarily unavailable' },
        } } })
      }
      if (manualDone) automaticDone = true
      return route.fulfill({ json: { code: 0, data: {
        usage: { 102: makeUsage(manualDone ? 35 : 20), 103: makeUsage(manualDone ? 45 : 60), 104: makeUsage(70, 'passive') },
        errors: {},
      } } })
    })
    let directQueries = 0
    await page.route(/\/api\/v1\/admin\/accounts\/\d+\/usage(?:\?|$)/, route => {
      directQueries++
      return route.fulfill({ status: 500, json: { message: 'Unexpected subscription query' } })
    })
    await page.goto('/admin/accounts?platform=anthropic&view=capacity')
    const capacity = page.getByRole('tabpanel', { name: 'Capacity', exact: true })
    const first = capacity.getByTestId('account-capacity-row').filter({ hasText: 'Claude fleet 102' })
    const second = capacity.getByTestId('account-capacity-row').filter({ hasText: 'Claude fleet 103' })
    await expect(first).toContainText('80%')
    await page.getByRole('button', { name: 'Refresh', exact: true }).click()
    await expect(first).toContainText('75%')
    await second.getByRole('button', { name: 'Show details for Claude fleet 103' }).click()
    await expect(second).toContainText('Claude refresh temporarily unavailable')
    expect(requests.at(-1)?.force).toBe(true)
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const technical = page.getByRole('tabpanel', { name: 'Technical', exact: true })
    await expect(technical).toContainText('25%')
    await expect(technical).toContainText('Claude refresh temporarily unavailable')
    await expect(page.getByRole('button', { name: 'Refresh', exact: true })).toBeEnabled()
    await page.clock.fastForward(5_000)
    await expect.poll(() => automaticDone).toBe(true)
    await expect(technical).toContainText('35%')
    await expect(technical).not.toContainText('Claude refresh temporarily unavailable')
    expect(requests.at(-1)?.force).toBe(false)
    await page.getByRole('tab', { name: 'Capacity', exact: true }).click()
    await expect(first).toContainText('65%')
    await expect(second).toContainText('55%')
    expect(directQueries).toBe(0)
    await expect(page.locator('body')).toHaveJSProperty('scrollWidth', width)
    await first.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`claude-fleet-refresh-${width}.png`), animations: 'disabled' })
  })
}
