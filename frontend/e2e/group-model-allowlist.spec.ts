import { expect, test, type Route } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts, operatorFixtureGroups } from './fixtures/operatorData'

const fulfill = (route: Route, data: unknown) => route.fulfill({
  status: 200,
  contentType: 'application/json',
  body: JSON.stringify({ code: 0, message: 'ok', data }),
})

test('edits the enforced allowlist without changing the display-only model list', async ({ page }, testInfo) => {
  await seedSession(page)
  await installOperatorApiMock(page)

  const updates: Record<string, unknown>[] = []
  await page.route('**/api/v1/admin/groups/**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    if (request.method() === 'GET' && pathname.endsWith('/model-allowlist-candidates')) {
      return fulfill(route, { models: ['gpt-5.5', 'gpt-5.4-mini'] })
    }
    if (request.method() === 'GET' && pathname.endsWith('/models-list-candidates')) {
      return fulfill(route, { models: ['catalog-only-model'] })
    }
    if (request.method() === 'PUT' && pathname === '/api/v1/admin/groups/11') {
      updates.push(request.postDataJSON() as Record<string, unknown>)
      return fulfill(route, {})
    }
    return route.fallback()
  })

  await page.goto('/admin/groups')
  const row = page.getByText('OpenAI Production', { exact: true }).locator('..').locator('..')
  await row.getByRole('button', { name: 'Edit' }).click()

  const dialog = page.getByRole('dialog', { name: 'Edit Group' })
  await expect(dialog).toBeVisible()
  const allowlistToggle = dialog.getByRole('switch', { name: 'Model Allowlist' })
  await allowlistToggle.click()
  await expect(dialog.getByText('gpt-5.5', { exact: true })).toBeVisible()
  await expect(dialog.getByText('catalog-only-model', { exact: true })).toHaveCount(0)
  await dialog.getByPlaceholder('Custom entry, e.g. claude-* or gpt-5.5-codex').fill('gpt-6-*')
  await dialog.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(dialog.getByText('gpt-6-*', { exact: false })).toBeVisible()
  await page.screenshot({ path: testInfo.outputPath('group-model-allowlist-desktop.png'), animations: 'disabled' })

  await dialog.getByRole('button', { name: 'Update', exact: true }).click()
  await expect.poll(() => updates).toHaveLength(1)
  expect(updates[0].model_allowlist).toEqual({
    enabled: true,
    models: ['gpt-5.5', 'gpt-5.4-mini', 'gpt-6-*'],
  })
  expect(updates[0].models_list_config).toEqual({ enabled: false, models: ['catalog-only-model'] })
})

test('persists ordered pinned manifest accounts and scheduler fallback through group edits', async ({ page }, testInfo) => {
  await seedSession(page)
  await installOperatorApiMock(page)

  let groups = operatorFixtureGroups.map(group => ({ ...group }))
  let update: Record<string, unknown> | undefined
  await page.route('**/api/v1/admin/groups**', async (route) => {
    const request = route.request()
    const pathname = new URL(request.url()).pathname
    if (request.method() === 'GET' && pathname === '/api/v1/admin/groups') {
      return fulfill(route, { items: groups, total: groups.length, page: 1, page_size: 20, pages: 1 })
    }
    if (request.method() === 'PUT' && pathname === '/api/v1/admin/groups/11') {
      update = request.postDataJSON() as Record<string, unknown>
      groups = groups.map(group => group.id === 11
        ? { ...group, codex_models_manifest_config: update!.codex_models_manifest_config }
        : group)
      return fulfill(route, groups.find(group => group.id === 11))
    }
    return route.fallback()
  })
  await page.route('**/api/v1/admin/accounts/101', route => fulfill(route, operatorFixtureAccounts.find(account => account.id === 101)))

  await page.goto('/admin/groups')
  const row = page.getByText('OpenAI Production', { exact: true }).locator('..').locator('..')
  await row.getByRole('button', { name: 'Edit' }).click()
  const dialog = page.getByRole('dialog', { name: 'Edit Group' })
  const manifestToggle = dialog.getByTestId('codex-manifest-toggle')
  await manifestToggle.click()
  await dialog.getByRole('button', { name: 'Update', exact: true }).click()
  await expect(dialog.getByTestId('codex-manifest-validation-error')).toBeVisible()

  const search = dialog.getByTestId('codex-manifest-search')
  await search.fill('Codex')
  await expect(dialog.getByTestId('codex-manifest-dropdown').getByRole('button', { name: /Codex Team West/ })).toBeVisible()
  await dialog.getByTestId('codex-manifest-dropdown').getByRole('button', { name: /Codex Team West/ }).click()
  await dialog.getByTestId('codex-manifest-fallback-toggle').click()
  await page.screenshot({ path: testInfo.outputPath('group-codex-manifest-desktop.png'), animations: 'disabled' })

  await dialog.getByRole('button', { name: 'Update', exact: true }).click()
  await expect.poll(() => update).toBeTruthy()
  expect(update?.codex_models_manifest_config).toEqual({
    enabled: true,
    account_ids: [101],
    fallback_to_scheduler: true,
  })

  await row.getByRole('button', { name: 'Edit' }).click()
  await expect(dialog.getByTestId('codex-manifest-selected-tags')).toContainText('Codex Team West')
  await expect(dialog.getByTestId('codex-manifest-fallback-toggle')).toHaveAttribute('aria-checked', 'true')
})
