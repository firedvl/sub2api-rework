import { expect, test, type Page, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData, operatorFixtureAccounts } from './fixtures/operatorData.ts'

const eligibleAccounts = [
  { ...operatorFixtureAccounts[0], parent_account_id: null },
  {
    ...operatorFixtureAccounts[0],
    id: 107,
    name: 'Codex Team East',
    parent_account_id: null,
    extra: { auto_warmup_enabled: false },
  },
]
const allAccounts = [...operatorFixtureAccounts, eligibleAccounts[1]]

const fulfill = (route: Route, data: unknown) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({ code: 0, message: 'ok', data }),
})

async function installAutoWarmupMock(page: Page, globalEnabled = false) {
  const bulkRequests: unknown[] = []

  await page.route('**/api/v1/admin/settings**', async (route) => {
    const request = route.request()
    if (request.method() !== 'GET' || new URL(request.url()).pathname !== '/api/v1/admin/settings') {
      return route.fallback()
    }
    const settings = structuredClone(getOperatorFixtureData('/api/v1/admin/settings')) as Record<string, unknown>
    settings.openai_auto_warmup_enabled = globalEnabled
    await fulfill(route, settings)
  })

  await page.route('**/api/v1/admin/accounts**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    if (request.method() === 'POST' && url.pathname === '/api/v1/admin/accounts/bulk-update') {
      bulkRequests.push(request.postDataJSON())
      return fulfill(route, {
        success: 2,
        failed: 0,
        success_ids: [101, 107],
        failed_ids: [],
        auto_warmup_updated_count: 2,
        auto_warmup_skipped_count: 0,
        results: [],
      })
    }
    if (url.pathname !== '/api/v1/admin/accounts') return route.fallback()

    const filter = url.searchParams.get('auto_warmup')
    const accounts = filter === 'eligible'
      ? eligibleAccounts
      : filter === 'enabled'
        ? (globalEnabled ? [] : [eligibleAccounts[0]])
        : filter === 'disabled'
          ? [eligibleAccounts[1]]
          : allAccounts
    const pageNumber = Math.max(1, Number(url.searchParams.get('page') || 1))
    const pageSize = Math.max(1, Number(url.searchParams.get('page_size') || accounts.length))
    const start = (pageNumber - 1) * pageSize
    return fulfill(route, {
      items: accounts.slice(start, start + pageSize),
      total: accounts.length,
      page: pageNumber,
      page_size: pageSize,
      pages: Math.ceil(accounts.length / pageSize),
    })
  })

  return bulkRequests
}

test.beforeEach(async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)
})

for (const viewport of [
  { name: 'desktop-1440', width: 1440, height: 900 },
  { name: 'compact-1024', width: 1024, height: 768 },
  { name: 'mobile-390', width: 390, height: 844 },
]) {
  test(`configures the full eligible fleet at ${viewport.name}`, async ({ page }, testInfo) => {
    test.setTimeout(60_000)
    await page.setViewportSize(viewport)
    const bulkRequests = await installAutoWarmupMock(page)
    await page.goto('/admin/accounts?view=technical&auto_warmup=disabled', { waitUntil: 'domcontentloaded' })

    await expect(page.getByText('Warm-up: Off', { exact: true })).toBeVisible({ timeout: 15_000 })
    await page.getByTestId('edit-auto-warmup-eligible').click()

    const dialog = page.getByRole('dialog', { name: 'Bulk Edit Accounts' })
    await expect(dialog).toContainText('2')
    const select = dialog.getByTestId('bulk-edit-auto-warmup-select').getByRole('button')
    await select.focus()
    await select.press('ArrowDown')
    await page.getByRole('option', { name: 'Enable', exact: true }).click()
    await expect(dialog.getByTestId('bulk-edit-auto-warmup-global-off')).toBeVisible()
    await expect(dialog.getByRole('link', { name: 'Open Gateway settings' })).toHaveAttribute('href', '/admin/settings?tab=gateway')
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`${viewport.name}.png`) })

    await dialog.getByRole('button', { name: 'Update Accounts' }).click()
    await expect.poll(() => bulkRequests).toHaveLength(1)
    expect(bulkRequests[0]).toEqual(expect.objectContaining({
      filters: expect.objectContaining({ auto_warmup: 'eligible' }),
      auto_warmup_enabled: true,
    }))
  })
}

test('shows the global-on zero-account notice at mobile width', async ({ page }, testInfo) => {
  test.setTimeout(60_000)
  await page.setViewportSize({ width: 390, height: 844 })
  await installAutoWarmupMock(page, true)
  await page.goto('/admin/settings?tab=gateway', { waitUntil: 'domcontentloaded' })

  const notice = page.getByTestId('openai-auto-warmup-zero-accounts')
  await expect(notice).toBeVisible({ timeout: 20_000 })
  await expect(notice).toContainText('no eligible OpenAI OAuth accounts are enabled')
  await expect(notice.getByRole('link', { name: 'Manage accounts' })).toHaveAttribute('href', '/admin/accounts?view=technical&auto_warmup=disabled')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('settings-zero-account-mobile.png') })
})
