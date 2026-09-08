import { expect, test, type Page, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

const resetAccounts = [
  {
    ...operatorFixtureAccounts[0],
    id: 101,
    name: 'Codex Team West',
    parent_account_id: null,
    extra: {
      auto_reset_credit_enabled: true,
      auto_reset_credit_5h_threshold: 0.9,
      auto_reset_credit_7d_threshold: 0.95,
    },
  },
  {
    ...operatorFixtureAccounts[0],
    id: 107,
    name: 'Codex Team East',
    parent_account_id: null,
    extra: {
      auto_reset_credit_enabled: false,
      auto_reset_credit_5h_threshold: 0.8,
      auto_reset_credit_7d_threshold: 0.9,
    },
  },
  {
    ...operatorFixtureAccounts[0],
    id: 108,
    name: 'Codex Model Retry',
    parent_account_id: null,
    extra: {
      auto_warmup_enabled: true,
      codex_5h_reset_at: '2099-01-01T00:00:00Z',
      codex_auto_warmup_state: {
        status: 'failed',
        attempted_at: '2098-12-31T23:00:00Z',
        reset_at: '2099-01-01T00:00:00Z',
        error_code: 'OPENAI_AUTO_WARMUP_MODEL_RESOLUTION_FAILED',
      },
    },
  },
  {
    ...operatorFixtureAccounts[0],
    id: 109,
    name: 'Codex Waiting Observation',
    parent_account_id: null,
    extra: {
      auto_warmup_enabled: true,
      codex_auto_warmup_evaluation: {
        reason: 'waiting_second_observation',
        observed_at: '2098-12-31T23:00:00Z',
        next_eligible_at: '2099-01-01T00:00:00Z',
      },
    },
  },
  {
    ...operatorFixtureAccounts[0],
    id: 110,
    name: 'Codex Current Complete',
    parent_account_id: null,
    extra: {
      auto_warmup_enabled: true,
      codex_5h_reset_at: '2099-01-01T00:00:00Z',
      codex_auto_warmup_state: {
        status: 'succeeded',
        attempted_at: '2098-12-31T23:00:00Z',
        completed_at: '2098-12-31T23:00:01Z',
        reset_at: '2099-01-01T00:00:00Z',
      },
    },
  },
]

const fulfill = (route: Route, data: unknown) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({ code: 0, message: 'ok', data }),
})

async function installResetCreditMock(page: Page) {
  const updates: unknown[] = []
  const bulkUpdates: unknown[] = []

  await page.route('**/api/v1/admin/settings**', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback()
    return fulfill(route, { openai_auto_warmup_enabled: true })
  })

  await page.route('**/api/v1/admin/accounts**', async (route) => {
    const request = route.request()
    const url = new URL(request.url())
    const accountID = Number(url.pathname.match(/\/admin\/accounts\/(\d+)$/)?.[1])
    if (request.method() === 'POST' && url.pathname === '/api/v1/admin/accounts/bulk-update') {
      bulkUpdates.push(request.postDataJSON())
      return fulfill(route, {
        success: 2,
        failed: 0,
        auto_reset_credit_updated_count: 2,
        auto_reset_credit_skipped_count: 0,
        results: [],
      })
    }
    if (request.method() === 'PUT' && Number.isFinite(accountID)) {
      updates.push(request.postDataJSON())
      return fulfill(route, resetAccounts.find((account) => account.id === accountID))
    }
    if (request.method() === 'GET' && Number.isFinite(accountID)) {
      return fulfill(route, resetAccounts.find((account) => account.id === accountID))
    }
    if (request.method() === 'GET' && url.pathname === '/api/v1/admin/accounts') {
      const pageNumber = Math.max(1, Number(url.searchParams.get('page') || 1))
      const pageSize = Math.max(1, Number(url.searchParams.get('page_size') || resetAccounts.length))
      const start = (pageNumber - 1) * pageSize
      return fulfill(route, {
        items: resetAccounts.slice(start, start + pageSize),
        total: resetAccounts.length,
        page: pageNumber,
        page_size: pageSize,
        pages: Math.ceil(resetAccounts.length / pageSize),
      })
    }
    return route.fallback()
  })

  return { updates, bulkUpdates }
}

async function openAccountEditor(page: Page) {
  await page.goto('/admin/accounts?view=technical', { waitUntil: 'domcontentloaded' })
  await page.getByRole('tab', { name: 'Technical' }).click()
  const row = page.locator('.operator-account-table tr').filter({ hasText: 'Codex Team West' })
  await row.locator('.operator-table-row-action').filter({ hasText: 'Edit' }).click()
  return page.getByRole('dialog', { name: 'Edit Account' })
}

async function selectAutoResetMode(dialog: ReturnType<Page['getByRole']>, mode: 'Enable' | 'Disable') {
  const select = dialog.getByTestId('bulk-edit-auto-reset-credit-select').getByRole('button')
  await select.click()
  await dialog.page().getByRole('option', { name: mode, exact: true }).click()
}

test.beforeEach(async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)
})

test('reviews a single enable or lower threshold, supports Escape, and sends only after confirmation', async ({ page }, testInfo) => {
  const { updates } = await installResetCreditMock(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  const dialog = await openAccountEditor(page)
  const toggle = dialog.getByTestId('auto-reset-credit-enabled')
  await toggle.scrollIntoViewIfNeeded()
  await dialog.getByTestId('auto-reset-credit-5h-threshold').fill('75')
  await dialog.getByTestId('auto-reset-credit-7d-threshold').fill('90')
  await dialog.getByRole('button', { name: 'Update' }).click()

  const review = page.getByRole('dialog', { name: 'Review automatic Reset Credit use' })
  await expect(review).toContainText('Use a credit at >= 75% USED')
  await expect(review).toContainText('Use a credit at >= 90% USED')
  await expect(review).toContainText('Either threshold')
  await expect(review).toContainText('Lowering this value causes Reset Credits to become eligible earlier.')
  await page.screenshot({ path: testInfo.outputPath('single-reset-credit-review-desktop.png'), animations: 'disabled' })
  await page.keyboard.press('Escape')
  await expect(review).toBeHidden()
  await expect(dialog.getByRole('button', { name: 'Update' })).toBeFocused()
  expect(updates).toHaveLength(0)

  await dialog.getByRole('button', { name: 'Update' }).click()
  await review.getByRole('button', { name: 'Confirm' }).click()
  await expect.poll(() => updates).toHaveLength(1)
  expect(updates[0]).toEqual(expect.objectContaining({
    extra: expect.objectContaining({
      auto_reset_credit_enabled: true,
      auto_reset_credit_5h_threshold: 0.75,
      auto_reset_credit_7d_threshold: 0.9,
    }),
  }))
})

test('does not interrupt a harmless single-account threshold increase', async ({ page }) => {
  const { updates } = await installResetCreditMock(page)
  const dialog = await openAccountEditor(page)
  await dialog.getByTestId('auto-reset-credit-5h-threshold').fill('95')
  await dialog.getByRole('button', { name: 'Update' }).click()

  await expect(page.getByRole('dialog', { name: 'Review automatic Reset Credit use' })).toBeHidden()
  await expect.poll(() => updates).toHaveLength(1)
})

test('reviews selected and filtered bulk changes with their exact scope at narrow and desktop widths', async ({ page }, testInfo) => {
  const { bulkUpdates } = await installResetCreditMock(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/admin/accounts?view=technical', { waitUntil: 'domcontentloaded' })
  await page.getByRole('tab', { name: 'Technical' }).click()
  const checkboxes = page.locator('.operator-account-table tbody input[type="checkbox"]')
  await expect(checkboxes).toHaveCount(5)
  await checkboxes.nth(0).check()
  await checkboxes.nth(1).check()
  await page.getByRole('button', { name: 'Bulk Edit', exact: true }).click()
  let dialog = page.getByRole('dialog', { name: 'Bulk Edit Accounts' })
  await page.setViewportSize({ width: 390, height: 844 })
  await selectAutoResetMode(dialog, 'Enable')
  await dialog.locator('#bulk-edit-auto-reset-credit-5h').fill('75')
  await dialog.locator('#bulk-edit-auto-reset-credit-7d').fill('90')
  await dialog.getByRole('button', { name: 'Update Accounts' }).click()

  let review = page.getByRole('dialog', { name: 'Review automatic Reset Credit use' })
  await expect(review).toContainText('Accounts affected')
  await expect(review).toContainText('2')
  await expect(review).toContainText('Use a credit at >= 75% USED')
  await expect(review).toContainText('Use a credit at >= 90% USED')
  await expect(review).toContainText('Either threshold')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('bulk-reset-credit-review-mobile.png'), animations: 'disabled' })
  await review.getByRole('button', { name: 'Confirm' }).click()
  await expect.poll(() => bulkUpdates).toHaveLength(1)
  expect(bulkUpdates[0]).toEqual(expect.objectContaining({
    account_ids: [101, 107],
    auto_reset_credit_enabled: true,
    auto_reset_credit_5h_threshold: 0.75,
    auto_reset_credit_7d_threshold: 0.9,
  }))

  await page.setViewportSize({ width: 1440, height: 900 })
  await page.goto('/admin/accounts?view=technical', { waitUntil: 'domcontentloaded' })
  await page.getByRole('tab', { name: 'Technical' }).click()
  await page.getByRole('button', { name: 'Update Accounts', exact: true }).click()
  dialog = page.getByRole('dialog', { name: 'Bulk Edit Accounts' })
  await dialog.locator('#bulk-edit-auto-reset-credit-5h').fill('75')
  await dialog.getByRole('button', { name: 'Update Accounts' }).click()
  review = page.getByRole('dialog', { name: 'Review automatic Reset Credit use' })
  await expect(review).toContainText('5')
  await page.screenshot({ path: testInfo.outputPath('bulk-reset-credit-review-desktop.png'), animations: 'disabled' })
  await review.getByRole('button', { name: 'Confirm' }).click()
  await expect.poll(() => bulkUpdates).toHaveLength(2)
  expect(bulkUpdates[1]).toEqual(expect.objectContaining({
    filters: expect.objectContaining({}),
    auto_reset_credit_5h_threshold: 0.75,
  }))
})

test('projects model retry, second observation, and current-window completion in technical and capacity views', async ({ page }, testInfo) => {
  await page.setViewportSize({ width: 1440, height: 900 })
  await installResetCreditMock(page)
  await page.goto('/admin/accounts?view=technical', { waitUntil: 'domcontentloaded' })
  await page.getByRole('tab', { name: 'Technical' }).click()
  const technical = page.locator('#account-technical-panel')
  await expect(technical.getByTestId('auto-warmup-reason').filter({ hasText: 'No eligible model available; bounded retry pending' })).toBeVisible()
  await expect(technical.getByTestId('auto-warmup-reason').filter({ hasText: 'Waiting for second quota observation' })).toBeVisible()
  await expect(technical.getByTestId('auto-warmup-reason').filter({ hasText: 'Warm-up complete for current window' })).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('warmup-status-technical-desktop.png'), fullPage: true, animations: 'disabled' })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.getByRole('tab', { name: 'Capacity' }).click()
  const row = page.getByTestId('account-capacity-row').filter({ hasText: 'Codex Model Retry' })
  await row.getByRole('button', { name: /Show details/ }).click()
  await expect(row.getByTestId('auto-warmup-reason')).toContainText('No eligible model available; bounded retry pending')
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
  await page.screenshot({ path: testInfo.outputPath('warmup-status-capacity-mobile.png'), fullPage: true, animations: 'disabled' })
})
