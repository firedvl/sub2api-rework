import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { operatorFixtureAccounts } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  for (const type of ['oauth', 'apikey']) {
    test(`Claude ${type} offers eligible warm-up and explicit connection tests at ${width}px`, async ({ page }, testInfo) => {
      test.setTimeout(60_000)
      await page.setViewportSize({ width, height: 900 })
      await seedSession(page)
      await installOperatorApiMock(page)
      const account = { ...operatorFixtureAccounts[1], id: 102, platform: 'anthropic', type }
      await page.route(/\/api\/v1\/admin\/accounts\/102(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: route.request().method() === 'PUT' ? { ...account, ...route.request().postDataJSON() } : account } }))
      await page.route(/\/api\/v1\/admin\/accounts\?/, route => route.fulfill({ json: { code: 0, data: {
        items: [account], total: 1, page: 1, pages: 1, page_size: 20,
      } } }))
      await page.route(/\/api\/v1\/admin\/accounts\/102\/models(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: [
        { id: 'claude-sonnet-4-6', display_name: 'Claude Sonnet' },
      ] } }))
      await page.route('**/api/v1/admin/accounts/check-mixed-channel', route => route.fulfill({ json: { code: 0, data: { has_risk: false } } }))
      let inferenceRequests = 0
      await page.route(/\/api\/v1\/admin\/accounts\/102\/test(?:\?.*)?$/, route => {
        inferenceRequests++
        return route.fulfill({ contentType: 'text/event-stream', body: 'data: {"type":"test_complete","success":true}\n\n' })
      })
      await page.goto('/admin/accounts?view=technical')
      await page.getByRole('tab', { name: 'Technical', exact: true }).click()
      await expect(page.getByTestId(type === 'oauth' ? 'account-auto-warmup-status' : 'claude-auto-warmup-unavailable').first()).toContainText(type === 'oauth' ? 'Warm-up: Off' : 'Warm-up: Unavailable')
      await page.getByRole('button', { name: 'Edit', exact: true }).click()
      const editor = page.getByRole('dialog', { name: 'Edit Account' })
      if (type === 'oauth') {
        const warmup = editor.getByTestId('auto-warmup-settings')
        await warmup.scrollIntoViewIfNeeded()
        await expect(warmup).toContainText('5-hour window is idle or expired')
        const toggle = warmup.getByRole('switch', { name: 'Automatic warm-up' })
        await expect(toggle).toHaveAttribute('aria-checked', 'false')
        await toggle.focus()
        await toggle.press('Space')
        await expect(toggle).toHaveAttribute('aria-checked', 'true')
        expect(inferenceRequests).toBe(0)
        await page.screenshot({ path: testInfo.outputPath(`claude-warmup-${width}.png`) })
        const saved = page.waitForRequest(request => request.method() === 'PUT' && /\/accounts\/102$/.test(new URL(request.url()).pathname))
        await editor.getByRole('button', { name: 'Update', exact: true }).click()
        expect((await saved).postDataJSON().extra.auto_warmup_enabled).toBe(true)
        await expect(editor).not.toBeVisible()
        await page.getByRole('button', { name: 'Edit', exact: true }).click()
      } else {
        const availability = editor.getByTestId('claude-auto-warmup-status')
        await availability.scrollIntoViewIfNeeded()
        await expect(availability).toContainText('API-key requests incur API charges')
        await expect(availability.getByRole('switch')).toHaveCount(0)
      }
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
      await expect(page.getByTestId(type === 'oauth' ? 'account-auto-warmup-status' : 'claude-auto-warmup-unavailable').first()).toContainText(type === 'oauth' ? 'Warm-up: On' : 'Warm-up: Unavailable')
    })
  }
}

for (const width of [390, 1280]) {
  test(`confirmed Claude warm-up shows a running provider countdown at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    const now = new Date('2026-10-30T12:00:00Z')
    const resetAt = '2026-10-30T16:32:30Z'
    await page.clock.install({ time: now })
    await seedSession(page)
    await installOperatorApiMock(page)
    const account = { ...operatorFixtureAccounts[1], id: 102, session_window_end: resetAt,
      extra: { auto_warmup_enabled: true, claude_auto_warmup_state: {
        status: 'succeeded', window_started: true, observed_5h_reset_at: resetAt,
        attempted_at: now.toISOString(), completed_at: now.toISOString()
      }, claude_auto_warmup_evaluation: {
        reason: 'current_window_complete', checked_at: now.toISOString(), next_eligible_at: resetAt
      } }
    }
    const usage = { source: 'active', cached: true, updated_at: now.toISOString(),
      five_hour: { utilization: 0.01, source: 'active', observed_at: now.toISOString(), resets_at: resetAt, stale: false }
    }
    await page.route('**/api/v1/admin/accounts?*', route => route.fulfill({ json: { code: 0, data: { items: [account], total: 1, page: 1, pages: 1, page_size: 20 } } }))
    await page.route(/\/api\/v1\/admin\/accounts\/102(?:\?.*)?$/, route => route.fulfill({ json: { code: 0, data: account } }))
    await page.route('**/api/v1/admin/accounts/usage/batch', route => route.fulfill({ json: { code: 0, data: { usage: { 102: usage }, errors: {} } } }))
    await page.route('**/api/v1/admin/accounts/102/usage*', route => route.fulfill({ json: { code: 0, data: usage } }))
    let sends = 0
    await page.route('**/api/v1/admin/accounts/102/test', route => {
      sends++
      return route.fulfill({ status: 500 })
    })
    await page.goto('/admin/accounts?view=technical')
    await page.getByRole('tab', { name: 'Technical', exact: true }).click()
    const cell = page.locator('[data-test="claude-usage-observed"]').locator('..')
    await expect(cell).toContainText('4h 32m')
    await expect(page.getByTestId('auto-warmup-reason').first()).toContainText('Warm-up complete for current window')
    await page.clock.fastForward(60_000)
    await expect(cell).toContainText('4h 31m')
    await page.getByRole('button', { name: 'Edit', exact: true }).click()
    const editor = page.getByRole('dialog', { name: 'Edit Account' })
    const confirmation = editor.getByTestId('claude-warmup-reset-at')
    await confirmation.scrollIntoViewIfNeeded()
    await expect(confirmation).toContainText('5-hour window resets at')
    await expect(editor.getByTestId('auto-warmup-settings')).not.toContainText('not yet verified')
    await page.screenshot({ path: testInfo.outputPath(`claude-confirmed-countdown-${width}.png`), animations: 'disabled' })
    expect(sends).toBe(0)
  })
}
