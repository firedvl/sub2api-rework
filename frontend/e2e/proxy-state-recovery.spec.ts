import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`proxy filters, malformed responses and partial imports recover at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    let malformed = false
    const requests: URLSearchParams[] = []
    const row = { id: 1, name: 'Recovery Proxy', protocol: 'http', host: '192.0.2.1', port: 8080,
      status: 'active', expires_at: new Date(Date.now() - 1000).toISOString(), account_count: 0,
      created_at: '2026-09-30T00:00:00Z', updated_at: '2026-09-30T00:00:00Z' }
    await page.route('**/api/v1/admin/proxies?*', route => {
      requests.push(new URL(route.request().url()).searchParams)
      return route.fulfill({ json: { code: 0, data: malformed ? { items: null } : {
        items: [row], total: 60, pages: 3, page: Number(requests.at(-1)?.get('page')), page_size: 20
      } } })
    })
    await page.route('**/api/v1/admin/proxies/all*', route => route.fulfill({ json: { code: 0, data: [row] } }))
    await page.route('**/api/v1/admin/proxies/data*', route => route.fulfill({ json: { code: 0, data: {
      proxy_created: 1, proxy_reused: 0, proxy_failed: 1, account_created: 0, account_failed: 0,
      errors: [{ kind: 'proxy', name: 'invalid fixture', message: 'Invalid fixture port' }]
    } } }))
    await page.goto('/admin/proxies')
    await expect(page.getByText('Recovery Proxy', { exact: true })).toBeVisible()
    await expect(page.getByText('Expired', { exact: true })).toBeVisible()
    await page.getByRole('button', { name: 'Next', exact: true }).click()
    await expect.poll(() => requests.at(-1)?.get('page')).toBe('2')
    await page.getByRole('button').filter({ hasText: /^All Protocols$/ }).click()
    await page.getByRole('option', { name: 'SOCKS5', exact: true }).click()
    await expect.poll(() => requests.at(-1)?.get('page')).toBe('1')
    expect(requests.at(-1)?.get('protocol')).toBe('socks5')
    await page.getByRole('button', { name: 'Next', exact: true }).click()
    await expect.poll(() => requests.at(-1)?.get('page')).toBe('2')
    await page.getByRole('button').filter({ hasText: /^All Status$/ }).click()
    await page.getByRole('option', { name: 'Expired', exact: true }).click()
    await expect.poll(() => requests.at(-1)?.get('page')).toBe('1')
    expect(requests.at(-1)?.get('status')).toBe('expired')
    malformed = true
    await page.getByRole('button', { name: 'Refresh', exact: true }).click()
    await expect(page.getByText('Failed to load proxies', { exact: true })).toBeVisible()
    await expect(page.getByText('Recovery Proxy', { exact: true })).toBeVisible()
    malformed = false
    await page.getByRole('button', { name: 'Refresh', exact: true }).click()
    await page.getByRole('button', { name: 'Import', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: 'Import Proxies', exact: true })
    await dialog.locator('input[type="file"]').setInputFiles({ name: 'fixture.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify({
      exported_at: '2026-09-30T00:00:00Z', proxies: [], accounts: []
    })) })
    await dialog.getByRole('button', { name: 'Start Import', exact: true }).click()
    await expect(dialog).toContainText('Invalid fixture port')
    const beforeClose = requests.length
    await page.keyboard.press('Tab')
    expect(await dialog.evaluate(element => element.contains(document.activeElement))).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`proxy-partial-${width}.png`), fullPage: true, animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(dialog).not.toBeVisible()
    await expect.poll(() => requests.length).toBeGreaterThan(beforeClose)
    expect(errors).toEqual([])
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width)
  })
}
