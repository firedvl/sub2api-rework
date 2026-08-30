import { expect, test, type Page } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

const account = (index: number) => ({
  name: `Browser Account ${index}`,
  platform: 'anthropic',
  type: 'apikey',
  credentials: { api_key: `browser-secret-${index}` },
  concurrency: 1,
  priority: 0,
})

async function openImport(page: Page) {
  await page.getByRole('button', { name: 'More Actions' }).click()
  await page.locator('.account-tools-menu').getByRole('button', { name: 'Import', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: 'Bulk Account Import' })
  await expect(dialog).toBeVisible()
  return dialog
}

test.describe('bulk account import', () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.goto('/admin/accounts')
  })

  test('previews, imports partial results, retries safely, and clears secrets', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    let importRequests = 0
    page.on('request', (request) => {
      if (request.method() === 'POST' && new URL(request.url()).pathname === '/api/v1/admin/accounts/data') importRequests += 1
    })

    let dialog = await openImport(page)
    await expect(page.getByRole('button', { name: 'Close modal' })).toBeFocused()
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    await expect(page.getByRole('button', { name: 'More Actions' })).toBeFocused()

    dialog = await openImport(page)
    const rows = [
      { ...account(1), name: 'Existing Codex', platform: 'openai', type: 'oauth', credentials: { email: 'codex-west@example.test', access_token: 'browser-secret-existing' } },
      { ...account(2), name: 'Provider Reject' },
      { ...account(3), credentials: { wrong_field: 'browser-secret-invalid' } },
      { ...account(4), platform: 'unsupported' },
      ...Array.from({ length: 6 }, (_, index) => account(index + 5)),
    ]
    await dialog.getByRole('textbox', { name: 'Paste account data' }).fill(JSON.stringify(rows))
    await dialog.getByRole('button', { name: 'Review', exact: true }).click()

    await expect(dialog.getByText('Ready', { exact: true }).locator('..').locator('strong')).toHaveText('7')
    await expect(dialog.getByText('Duplicates', { exact: true }).locator('..').locator('strong')).toHaveText('1')
    await expect(dialog.getByText('Invalid', { exact: true }).locator('..').locator('strong')).toHaveText('2')
    await expect(dialog.locator('tbody tr')).toHaveCount(10)
    await expect(dialog).not.toContainText('browser-secret')

    await dialog.getByRole('button', { name: 'Start Import' }).click()
    await expect(dialog.locator('section').last().locator('div.grid strong')).toHaveText(['6', '0', '1', '3'])
    await expect(dialog.locator('tbody tr')).toHaveCount(10)
    await expect(dialog).not.toContainText('browser-secret')

    await dialog.getByRole('button', { name: 'Failed', exact: true }).click()
    await expect(dialog.locator('tbody tr')).toHaveCount(3)
    await dialog.getByRole('button', { name: 'Copy', exact: true }).click()
    const copied = await page.evaluate(() => navigator.clipboard.readText())
    expect(copied).toContain('Provider validation failed')
    expect(copied).not.toContain('browser-secret')

    await dialog.getByRole('button', { name: 'Retry Failed' }).click()
    await expect(dialog.locator('section').last().locator('div.grid strong')).toHaveText(['6', '0', '1', '3'])
    expect(importRequests).toBe(2)

    await dialog.getByRole('button', { name: 'Close', exact: true }).click()
    await expect(dialog).toBeHidden()
    await expect(page.getByRole('button', { name: 'More Actions' })).toBeFocused()
    expect(await page.evaluate(() => Object.values(localStorage).some((value) => value.includes('browser-secret')))).toBe(false)

    dialog = await openImport(page)
    await expect(dialog.getByRole('textbox', { name: 'Paste account data' })).toHaveValue('')
  })

  test('supports file picker and drag/drop with linked export proxies', async ({ page }) => {
    const body = JSON.stringify({
      type: 'sub2api-data',
      version: 1,
      exported_at: '2026-08-29T00:00:00Z',
      proxies: [{ proxy_key: 'http|127.0.0.1|8080||', name: 'Linked proxy', protocol: 'http', host: '127.0.0.1', port: 8080, status: 'active' }],
      accounts: [account(1)],
    })
    let dialog = await openImport(page)
    await dialog.locator('input[type=file]').setInputFiles({ name: 'accounts.json', mimeType: 'application/json', buffer: Buffer.from(body) })
    await dialog.getByRole('button', { name: 'Review', exact: true }).click()
    await expect(dialog.locator('tbody tr')).toHaveCount(1)
    await page.keyboard.press('Escape')

    dialog = await openImport(page)
    const dataTransfer = await page.evaluateHandle((contents) => {
      const transfer = new DataTransfer()
      transfer.items.add(new File([contents], 'accounts.json', { type: 'application/json' }))
      return transfer
    }, body)
    await dialog.getByText('Drop .json or .jsonl files here', { exact: true }).last().dispatchEvent('drop', { dataTransfer })
    await dialog.getByRole('button', { name: 'Review', exact: true }).click()
    await expect(dialog.locator('tbody tr')).toHaveCount(1)
  })

  test('keeps a 50-row review usable at tablet width', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 900 })
    const dialog = await openImport(page)
    await dialog.getByRole('textbox', { name: 'Paste account data' }).fill(JSON.stringify(Array.from({ length: 50 }, (_, index) => account(index + 1))))
    await dialog.getByRole('button', { name: 'Review', exact: true }).click()
    await expect(dialog.locator('tbody tr')).toHaveCount(50)
    const bounds = await dialog.locator('.operator-dialog').boundingBox()
    expect(bounds).not.toBeNull()
    expect(bounds!.x).toBeGreaterThanOrEqual(0)
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(768)
    await expect(dialog.getByRole('button', { name: 'Start Import' })).toBeVisible()
  })
})
