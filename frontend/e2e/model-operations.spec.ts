import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

test('shows catalog membership separately from current availability', async ({ page }) => {
  await seedSession(page)
  await installOperatorApiMock(page)
  await page.goto('/admin/model-operations')

  await expect(page.getByRole('main').getByRole('heading', { name: 'Model Operations' })).toBeVisible()
  await expect(page.getByText('gpt-6-astra', { exact: true })).toBeVisible()
  await expect(page.getByText('unavailable', { exact: true })).toBeVisible()
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
  await expect(page.getByRole('main').getByRole('heading', { name: 'Model Operations' })).toBeVisible()
  await expect(page.getByRole('combobox', { name: 'Group' })).toBeVisible()
  await expect(page.getByRole('searchbox', { name: 'Filter models' })).toBeVisible()
  await expect(tableScroller).toHaveCSS('overflow-x', 'auto')
  await expect(page.getByText('gpt-6-astra', { exact: true })).toBeVisible()
})
