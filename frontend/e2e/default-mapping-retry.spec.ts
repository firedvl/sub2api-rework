import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`default model mappings surface failure and retry at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page, 'admin')
    await installOperatorApiMock(page, 'admin')
    let requests = 0
    await page.route('**/api/v1/admin/accounts/antigravity/default-model-mapping*', route => {
      requests += 1
      return requests === 1
        ? route.fulfill({ status: 503, json: { code: 503, message: 'mapping service unavailable' } })
        : route.fulfill({ json: { code: 0, data: { 'gemini-custom': 'gemini-provider' } } })
    })
    await page.goto('/admin/accounts')
    await page.getByRole('button', { name: 'Create Account', exact: true }).click()
    let dialog = page.getByRole('dialog', { name: 'Create Account' })
    await dialog.getByRole('button', { name: 'Antigravity', exact: true }).click()
    await expect(page.getByText('mapping service unavailable', { exact: true })).toBeVisible()
    await dialog.getByRole('button', { name: 'OpenAI', exact: true }).click()
    await dialog.getByRole('button', { name: 'Antigravity', exact: true }).click()
    await expect(dialog.getByPlaceholder('Request Model')).toHaveValue('gemini-custom')
    await expect(dialog.getByPlaceholder('Actual Model')).toHaveValue('gemini-provider')
    await dialog.getByPlaceholder('Request Model').fill('gemini-edited')
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    expect(requests).toBe(2)
    await page.getByRole('button', { name: 'Create Account', exact: true }).click()
    dialog = page.getByRole('dialog', { name: 'Create Account' })
    await dialog.getByRole('button', { name: 'Antigravity', exact: true }).click()
    await expect(dialog.getByPlaceholder('Request Model')).toHaveValue('gemini-custom')
    expect(requests).toBe(2)
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  })
}
