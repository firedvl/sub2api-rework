import { expect, test, type Locator, type Page } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const hints = {
  'Context Pool (ctx_pool)': 'The gateway gets and reuses upstream WS connections from a pool, with the pool limit determined by gateway configuration.',
  'Passthrough (passthrough)': 'The gateway opens a separate upstream WS connection for each client session, without using a connection pool.',
  'HTTP Bridge (http_bridge)': 'The gateway converts client WS requests to upstream HTTP requests, then converts SSE streaming responses back into WS messages.',
}

async function checkModeHints(page: Page, section: Locator) {
  for (const [label, hint] of Object.entries(hints)) {
    const trigger = section.locator('.select-trigger').first()
    await trigger.focus()
    await trigger.press('ArrowDown')
    await page.getByRole('option', { name: label, exact: true }).click()
    await expect(section.getByText(hint, { exact: true })).toBeVisible()
    for (const otherHint of Object.values(hints).filter(value => value !== hint)) {
      await expect(section.getByText(otherHint, { exact: true })).toHaveCount(0)
    }
  }
  await section.locator('.select-trigger').first().click()
  await page.getByRole('option', { name: 'Off (off)', exact: true }).click()
  await expect(page.getByRole('listbox')).toBeHidden()
  for (const hint of Object.values(hints)) {
    await expect(section.getByText(hint, { exact: true })).toHaveCount(0)
  }
}

for (const width of [390, 1280]) {
  test(`shows distinct WS mode hints in all account dialogs at ${width}px`, async ({ page }, testInfo) => {
    test.setTimeout(90_000)
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.goto('/admin/accounts?view=technical', { waitUntil: 'domcontentloaded' })

    await page.getByRole('button', { name: 'Create Account', exact: true }).first().click()
    const createDialog = page.getByRole('dialog', { name: 'Create Account' })
    await createDialog.getByRole('button', { name: 'OpenAI', exact: true }).click()
    await checkModeHints(page, createDialog.getByTestId('create-openai-ws-mode'))
    await page.keyboard.press('Escape')

    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const accountRow = page.locator(width < 640
      ? '.operator-account-table div.rounded-lg.border'
      : '.operator-account-table tr').filter({ hasText: 'Codex Team West' })
    await expect(accountRow).toHaveCount(1)
    await accountRow.locator('.operator-table-row-action').filter({ hasText: 'Edit' }).click()
    const editDialog = page.getByRole('dialog', { name: 'Edit Account' })
    await checkModeHints(page, editDialog.getByTestId('edit-openai-ws-mode-select').locator('xpath=../..'))
    await page.keyboard.press('Escape')

    await accountRow.locator('input[type="checkbox"]').check()
    await page.getByRole('button', { name: 'Bulk Edit', exact: true }).click()
    const bulkDialog = page.getByRole('dialog', { name: 'Bulk Edit Accounts' })
    await bulkDialog.locator('#bulk-edit-openai-ws-mode-enabled').check()
    await checkModeHints(page, bulkDialog.locator('#bulk-edit-openai-ws-mode'))
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`ws-hints-${width}.png`) })
  })
}
