import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`Ops latency and error drill-down preserve their meanings at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    const errors: string[] = []
    page.on('pageerror', error => errors.push(error.message))
    await page.route('**/api/v1/admin/ops/advanced-settings*', route => route.fulfill({ json: {
      code: 0, data: { ...getOperatorFixtureData('/api/v1/admin/ops/advanced-settings') as object, display_openai_token_stats: true }
    } }))
    await page.route('**/api/v1/admin/ops/dashboard/openai-token-stats*', route => route.fulfill({ json: {
      code: 0, data: { items: [{ model: 'gemini-2.5-pro', request_count: 2 }], total: 1, page: 1, page_size: 20 }
    } }))
    const detailQueries: URLSearchParams[] = []
    await page.route('**/api/v1/admin/ops/requests?*', route => {
      detailQueries.push(new URL(route.request().url()).searchParams)
      return route.fulfill({ json: { code: 0, data: { total: 3, items: [800, 0, null].map((latency, index) => ({
        kind: 'success', created_at: '2026-09-30T00:00:00Z', model: `gemini-fixture-${index}`,
        platform: 'gemini', duration_ms: 12000, first_token_ms: latency
      })) } } })
    })
    await page.route('**/api/v1/admin/ops/request-errors?*', route => route.fulfill({ json: { code: 0, data: {
      total: 1, items: [{ id: 1, created_at: '2026-09-30T00:00:00Z', phase: 'upstream', severity: 'error',
        status_code: 502, platform: 'gemini', message: 'Provider fixture response', request_id: 'fixture-request', resolved: false }]
    } } }))
    await page.goto('/admin/ops')
    await expect(page.getByText('Token Request Stats', { exact: true })).toBeVisible()
    await expect(page.getByText('gemini-2.5-pro', { exact: true })).toBeVisible()
    await page.getByText('TTFT', { exact: true }).locator('../..').getByRole('button', { name: 'Details', exact: true }).click()
    const details = page.getByRole('dialog', { name: 'TTFT (first_token_ms)', exact: true })
    await expect(details).toContainText('800 ms')
    await expect(details).toContainText('0 ms')
    await expect(details).not.toContainText('12000 ms')
    expect(detailQueries.at(-1)?.get('sort')).toBe('ttft_desc')
    expect(detailQueries.at(-1)?.get('kind')).toBe('success')
    await page.keyboard.press('Tab')
    expect(await details.evaluate(element => element.contains(document.activeElement))).toBe(true)
    await page.screenshot({ path: testInfo.outputPath(`ops-ttft-${width}.png`), fullPage: true, animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(details).not.toBeVisible()
    await page.getByText('Request Errors', { exact: true }).locator('../..').getByRole('button', { name: 'Details', exact: true }).click()
    const errorDetails = page.getByRole('dialog', { name: 'Request Errors', exact: true })
    await expect(errorDetails).toContainText('Provider fixture response')
    if (width >= 768) {
      expect((await errorDetails.locator('thead th').allTextContents()).slice(0, 2).map(text => text.trim())).toEqual(['Time', 'Message'])
    } else {
      expect(await errorDetails.locator('[data-field]').evaluateAll(elements => elements.slice(0, 2).map(element => element.getAttribute('data-field')))).toEqual(['created_at', 'message'])
    }
    await page.keyboard.press('Escape')
    await expect(errorDetails).not.toBeVisible()
    expect(errors).toEqual([])
  })
}
