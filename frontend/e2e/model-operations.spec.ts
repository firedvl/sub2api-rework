import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

test('shows catalog membership separately from current availability', async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)
  await page.goto('/admin/model-operations')

  await expect(page.locator('.app-header').getByRole('heading', { level: 1, name: 'Model Operations' })).toBeVisible()
  await expect(page.getByText('gpt-6-astra', { exact: true })).toBeVisible()
  await expect(page.getByText('unavailable', { exact: true })).toBeVisible()
  await expect(page.getByText('P50 1,280 ms', { exact: false })).toBeVisible()
  // Output speed and timing-sample counts now sit in the model's row drawer.
  await page.getByRole('row', { name: 'gpt-6-astra', exact: true }).click()
  const drawer = page.getByRole('dialog', { name: 'gpt-6-astra' })
  await expect(drawer.getByText('46.7 tok/s', { exact: true })).toBeVisible()
  await expect(drawer.locator('dt', { hasText: 'Timing samples' }).locator('xpath=following-sibling::dd[1]')).toHaveText('12')
  await page.keyboard.press('Escape')
  await expect(drawer).toBeHidden()
  await expect(page.getByText('Grok', { exact: true })).toHaveCount(0)

  await page.getByRole('searchbox', { name: 'Filter models' }).fill('missing-model')
  await expect(page.getByText('No provider-backed models are published for this group.')).toBeVisible()
})

test('keeps the diagnostic table contained on a narrow viewport', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await seedSession(page)
  await installOperatorApiMock(page)
  await page.goto('/admin/model-operations')

  const tableScroller = page.locator('table').locator('..')
  await expect(page.locator('.app-header').getByRole('heading', { level: 1, name: 'Model Operations' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Group' })).toBeVisible()
  await expect(page.getByRole('searchbox', { name: 'Filter models' })).toBeVisible()
  await expect(tableScroller).toHaveCSS('overflow-x', 'auto')
  await expect(page.getByText('gpt-6-astra', { exact: true })).toBeVisible()
})
