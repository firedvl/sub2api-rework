import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  for (const type of ['oauth', 'apikey']) {
    test(`Claude ${type} separates usage tests from unavailable warm-up at ${width}px`, async ({ page }, testInfo) => {
      test.setTimeout(60_000)
      await page.setViewportSize({ width, height: 900 })
      await seedSession(page)
      await installOperatorApiMock(page)
      const account = { ...operatorFixtureAccounts[1], id: 102, platform: 'anthropic', type }
      await page.route(/\/api\/v1\/admin\/accounts\/102(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: account } }))
      await page.route(/\/api\/v1\/admin\/accounts\?/, route => route.fulfill({ json: { code: 0, data: {
        items: [account], total: 1, page: 1, pages: 1, page_size: 20,
      } } }))
      await page.route(/\/api\/v1\/admin\/accounts\/102\/models(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: [
        { id: 'claude-sonnet-4-6', display_name: 'Claude Sonnet' },
      ] } }))
      let inferenceRequests = 0
      await page.route(/\/api\/v1\/admin\/accounts\/102\/test(?:\?.*)?$/, route => {
        inferenceRequests++
        return route.fulfill({ contentType: 'text/event-stream', body: 'data: {"type":"test_complete","success":true}\n\n' })
      })
      await page.goto('/admin/accounts?view=technical')
      await page.getByRole('tab', { name: 'Technical', exact: true }).click()
      await expect(page.getByTestId('claude-auto-warmup-unavailable').first()).toContainText('Warm-up: Unavailable')
      await page.getByRole('button', { name: 'Edit', exact: true }).click()
      const editor = page.getByRole('dialog', { name: 'Edit Account' })
      const availability = editor.getByTestId('claude-auto-warmup-status')
      await availability.scrollIntoViewIfNeeded()
      await expect(availability).toContainText(type === 'apikey' ? 'API-key requests incur API charges' : 'Automatic subscription warm-up is unavailable')
      await expect(availability.getByRole('switch')).toHaveCount(0)
      await editor.getByRole('button', { name: 'Cancel', exact: true }).click()
      await page.getByRole('button', { name: 'More', exact: true }).click()
      await page.getByRole('button', { name: 'Test Connection', exact: true }).click()
      const tester = page.getByRole('dialog', { name: 'Test Account Connection' })
      await expect(tester.getByTestId('test-usage-warning')).toContainText('real inference request')
      await expect(tester.getByTestId('test-usage-warning')).toContainText('does not verify')
      expect(inferenceRequests).toBe(0)
      await page.screenshot({ path: testInfo.outputPath(`claude-${type}-${width}.png`) })
      const start = tester.getByRole('button', { name: 'Start Test', exact: true })
      await expect(start).toBeEnabled()
      await start.focus()
      await start.press('Enter')
      await expect(tester).toContainText('Test completed successfully!')
      expect(inferenceRequests).toBe(1)
      await expect(tester.getByTestId('test-usage-warning')).toContainText('does not verify')
      await tester.getByRole('button', { name: 'Close', exact: true }).click()
      await expect(page.getByTestId('claude-auto-warmup-unavailable').first()).toContainText('Warm-up: Unavailable')
    })
  }
}
