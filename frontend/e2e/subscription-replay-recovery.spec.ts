import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [390, 1280]) {
  for (const outcome of ['replayed', 'expired'] as const) {
    test(`uncertain subscription ${outcome} recovery preserves changed targets at ${width}px`, async ({ page }, testInfo) => {
      await page.setViewportSize({ width, height: 900 })
      await seedSession(page)
      await installOperatorApiMock(page)
      const errors: string[] = []
      page.on('pageerror', error => errors.push(error.message))
      const group = { id: 1, name: 'Replay fixture group', platform: 'openai', subscription_type: 'subscription', rate_multiplier: 1 }
      const rows = [1, 2].map(id => ({
        id, user_id: id + 10, group_id: 1, status: 'active', starts_at: '2026-09-01T00:00:00Z',
        expires_at: '2027-01-01T00:00:00Z', created_at: '2026-09-01T00:00:00Z', updated_at: '2026-09-01T00:00:00Z',
        daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0,
        user: { id: id + 10, email: `replay-${id}@example.test` }, group
      }))
      await page.route('**/api/v1/admin/subscriptions?**', route => route.fulfill({ json: { code: 0, data: {
        items: rows, total: rows.length, page: 1, page_size: 20, pages: 1
      } } }))
      await page.route('**/api/v1/admin/groups*', route => route.fulfill({ json: { code: 0, data: { items: [group], total: 1, pages: 1 } } }))
      const attempts: { body: unknown; key: string; replayOnly?: string }[] = []
      await page.route('**/api/v1/admin/subscriptions/bulk-action', route => {
        const headers = route.request().headers()
        attempts.push({ body: route.request().postDataJSON(), key: headers['idempotency-key'], replayOnly: headers['idempotency-retry-only'] })
        if (attempts.length === 1) {
          rows[0].status = 'revoked'
          return route.fulfill({ status: 503, json: { code: 503, message: 'Response outcome unknown' } })
        }
        if (outcome === 'expired') {
          return route.fulfill({ status: 410, json: { code: 410, reason: 'IDEMPOTENCY_REPLAY_UNAVAILABLE', message: 'Original replay is unavailable' } })
        }
        return route.fulfill({ json: { code: 0, data: { success_count: 1, failed_count: 1, results: [
          { subscription_id: 1, success: true }, { subscription_id: 2, success: false, error: 'Original fixture failure' }
        ] } } })
      })
      await page.goto('/admin/subscriptions')
      for (const id of [1, 2]) await page.getByRole('checkbox', { name: `Select subscription #${id}`, exact: true }).check()
      await page.locator('[data-test="bulk-revoke"]').click()
      let dialog = page.getByRole('dialog', { name: 'Bulk Revoke', exact: true })
      await dialog.getByRole('button', { name: 'Confirm Action', exact: true }).click()
      await expect(dialog).toContainText('Response outcome unknown')
      await page.keyboard.press('Escape')
      await expect(dialog).not.toBeVisible()
      await expect(page.getByText('Unconfirmed Actions', { exact: true })).toBeVisible()
      await page.getByRole('button', { name: 'Refresh', exact: true }).click()
      await expect(page.locator('[data-test="bulk-revoke"]')).toHaveText('Bulk Revoke (1)')
      await page.locator('[data-test="subscription-pending-retry"]').click()
      dialog = page.getByRole('dialog', { name: 'Bulk Revoke', exact: true })
      await expect(dialog).toContainText('replay-1@example.test')
      await expect(dialog).toContainText('replay-2@example.test')
      await dialog.getByRole('button', { name: 'Retry Original Action', exact: true }).click()
      expect(attempts).toHaveLength(2)
      expect(attempts[1].body).toEqual(attempts[0].body)
      expect(attempts[1].key).toBe(attempts[0].key)
      expect(attempts[0].replayOnly).toBeUndefined()
      expect(attempts[1].replayOnly).toBe('true')
      if (outcome === 'expired') {
        await expect(dialog).toContainText('Original replay is unavailable')
        await expect(dialog.getByRole('button', { name: 'Retry Original Action', exact: true })).toBeDisabled()
      } else {
        await expect(dialog).toContainText('Original fixture failure')
      }
      await page.keyboard.press('Tab')
      expect(await dialog.evaluate(element => element.contains(document.activeElement))).toBe(true)
      await page.screenshot({ path: testInfo.outputPath(`subscription-${outcome}-${width}.png`), fullPage: true, animations: 'disabled' })
      await page.keyboard.press('Escape')
      await expect(dialog).not.toBeVisible()
      if (outcome === 'expired') {
        await page.getByRole('button', { name: 'Dismiss Saved Action', exact: true }).click()
        const confirmation = page.getByRole('dialog', { name: 'Dismiss Saved Action', exact: true })
        await confirmation.getByRole('button', { name: 'Cancel', exact: true }).click()
        await expect(confirmation).not.toBeVisible()
        await expect(page.locator('[data-test="subscription-pending-retry"]')).toBeVisible()
        await page.getByRole('button', { name: 'Dismiss Saved Action', exact: true }).click()
        await confirmation.getByRole('button', { name: 'Dismiss Saved Action', exact: true }).click()
        await expect(page.locator('[data-test="subscription-pending-retry"]')).not.toBeVisible()
        expect(attempts).toHaveLength(2)
      } else {
        await expect(page.locator('[data-test="subscription-pending-retry"]')).not.toBeVisible()
      }
      expect(errors).toEqual([])
    })
  }
}
