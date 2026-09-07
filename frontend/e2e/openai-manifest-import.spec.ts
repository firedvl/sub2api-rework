import { expect, test, type Locator, type Page } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

async function openOpenAICodexImport(page: Page) {
  await page.getByRole('button', { name: 'Create Account' }).click()
  const dialog = page.getByRole('dialog', { name: 'Create Account' })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: 'OpenAI', exact: true }).click()
  await dialog.getByPlaceholder('Enter account name').fill('Fixture Codex account')
  return dialog
}

async function continueToCodexSessionImport(dialog: Locator) {
  await dialog.getByRole('button', { name: 'Next', exact: true }).click()
  await dialog.getByLabel('Codex OAuth auth.json / AT Import').check()
  await dialog.getByPlaceholder('Multiple lines supported, one token or auth.json object per line').fill('fixture-token')
  return dialog
}

async function captureCodexImport(page: Page) {
  let payload: Record<string, unknown> | undefined
  await page.route('**/api/v1/admin/accounts/import/codex-session', async (route) => {
    payload = route.request().postDataJSON() as Record<string, unknown>
    await route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({
        code: 0,
        message: 'ok',
        data: { created: 1, updated: 0, skipped: 0, failed: 0, errors: [], warnings: [] },
      }),
    })
  })
  return () => payload
}

test.describe('OpenAI manifest-driven Codex imports', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.goto('/admin/accounts')
  })

  test('keeps the default OpenAI OAuth import unrestricted', async ({ page }, testInfo) => {
    const getPayload = await captureCodexImport(page)
    const createDialog = await openOpenAICodexImport(page)

    await expect(createDialog.getByText('Supports all models')).toBeVisible()
    await createDialog.getByText('Supports all models').scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath('openai-codex-default-desktop.png') })
    const dialog = await continueToCodexSessionImport(createDialog)
    await dialog.getByRole('button', { name: 'Import & Create Account', exact: true }).click()

    expect(getPayload()).toBeDefined()
    expect(getPayload()).not.toHaveProperty('credential_extras')
  })

  test('keeps an intentional mapping usable at mobile width', async ({ page }, testInfo) => {
    await page.setViewportSize({ width: 390, height: 844 })
    const getPayload = await captureCodexImport(page)
    const dialog = await openOpenAICodexImport(page)

    await dialog.getByRole('button', { name: 'Model Mapping', exact: true }).click()
    await dialog.getByRole('button', { name: /Add Mapping/ }).first().click()
    await dialog.getByPlaceholder('Request model').fill('gpt-public')
    await dialog.getByPlaceholder('Actual model').fill('gpt-private')
    await expect(dialog).toBeVisible()
    await page.screenshot({ path: testInfo.outputPath('openai-codex-mapping-mobile.png') })

    await dialog.getByRole('button', { name: 'Next', exact: true }).click()
    await dialog.getByLabel('Codex OAuth auth.json / AT Import').check()
    await dialog.getByPlaceholder('Multiple lines supported, one token or auth.json object per line').fill('fixture-token')
    await dialog.getByRole('button', { name: 'Import & Create Account', exact: true }).click()

    expect(getPayload()?.credential_extras).toEqual({
      model_mapping: { 'gpt-public': 'gpt-private' },
    })
  })
})
