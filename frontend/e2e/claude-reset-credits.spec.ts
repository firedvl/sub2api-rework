import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  test(`native Claude query and confirmed reset at ${width}px`, async ({ page }, testInfo) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    let queries = 0
    const keys: string[] = []
    let consumed = false
    let verified = false
    await page.route('**/api/v1/user/totp/step-up', async route => {
      expect(route.request().postDataJSON()).toEqual({ code: '123456' })
      verified = true
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ code: 0, data: {} }) })
    })
    await page.route('**/api/v1/admin/accounts/102/claude/reset-credits**', async route => {
      const request = route.request()
      if (request.method() === 'POST') {
        keys.push(request.headers()['idempotency-key'])
        expect(request.postData()).toBeNull()
        if (!verified) {
          return route.fulfill({ status: 403, contentType: 'application/json', body: JSON.stringify({
            code: 403, reason: 'STEP_UP_REQUIRED', message: 'Verification required',
          }) })
        }
        if (keys.length === 3) {
          return route.fulfill({ status: 409, contentType: 'application/json', body: JSON.stringify({
            code: 409, reason: 'IDEMPOTENCY_IN_PROGRESS', message: 'Still processing',
          }) })
        }
        consumed = true
        return route.fulfill({ contentType: 'application/json', body: JSON.stringify({
          code: 0, data: { outcome: 'reset', cleared: ['five_hour', 'seven_day', 'seven_day_overage_included'], replayed: true },
        }) })
      }
      queries++
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ code: 0, data: {
        eligible: true, available_count: consumed ? 0 : 2, fetched_at: '2026-10-08T00:00:00Z',
        credits: consumed ? [] : [{
          label: 'Native reset', resets_left: 2, expires_at: '2099-10-22T00:00:00Z',
          clears: ['five_hour', 'seven_day', 'seven_day_overage_included'],
          percent_used: {}, blocking: [], use_requires_limit: false, redeemable: true,
        }],
      } }) })
    })
    await page.goto('/admin/accounts?view=technical', { waitUntil: 'domcontentloaded' })
    await page.getByRole('tab', { name: 'Technical' }).click()
    const row = width < 768 ? page : page.locator('.operator-account-table tr').filter({ hasText: 'Claude Primary' })
    const count = row.getByTestId('claude-reset-count')
    const reset = row.getByTestId('claude-reset-redeem')
    await count.scrollIntoViewIfNeeded()
    await expect(reset).toBeDisabled()
    expect(queries).toBe(0)
    await count.click()
    await expect(count).toHaveText('Resets2')
    await expect(reset).toBeEnabled()
    expect(keys).toHaveLength(0)
    await reset.click()
    const dialog = page.getByRole('dialog', { name: 'Confirm Claude Reset' })
    await expect(dialog).toContainText('5h, 7d, 7d overage')
    await expect(dialog).toContainText('1 remaining')
    await page.screenshot({ path: testInfo.outputPath(`claude-confirm-${width}.png`), animations: 'disabled' })
    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    expect(keys).toHaveLength(0)
    await reset.click()
    await dialog.getByRole('button', { name: 'Reset', exact: true }).click()
    const verification = page.locator('.fixed.inset-0.z-\\[60\\]').filter({ has: page.getByText('Two-Factor Verification Required', { exact: true }) })
    await expect(verification).toBeVisible()
    await verification.getByRole('button', { name: 'Cancel', exact: true }).click()
    await expect(verification).toBeHidden()
    expect(keys).toHaveLength(1)
    await reset.click()
    await dialog.getByRole('button', { name: 'Reset', exact: true }).click()
    await expect(verification).toBeVisible()
    const digits = verification.locator('input[maxlength="1"]')
    for (let index = 0; index < 6; index++) await digits.nth(index).fill(String(index + 1))
    await expect(verification).toBeHidden()
    await expect(row.getByTestId('claude-reset-feedback')).toContainText('still processing')
    await reset.click()
    await dialog.getByRole('button', { name: 'Reset', exact: true }).click()
    await expect(row.getByTestId('claude-reset-feedback')).toContainText('Reset applied')
    expect(keys).toHaveLength(4)
    expect(keys[1]).not.toBe(keys[0])
    expect(keys[2]).toBe(keys[1])
    expect(keys[3]).toBe(keys[1])
    await expect(count).toHaveText('Resets0')
    await expect(reset).toBeDisabled()
    await page.screenshot({ path: testInfo.outputPath(`claude-complete-${width}.png`), animations: 'disabled' })
  })
}
