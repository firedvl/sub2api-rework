import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

test.use({ timezoneId: 'America/Los_Angeles', viewport: { width: 1440, height: 900 } })

for (const scenario of [
  { expires: '2026-09-23T01:00:00Z', label: 'Expires today' },
  { expires: '2026-09-24T01:00:00Z', label: 'Expires tomorrow' },
]) {
  test(`header uses calendar label ${scenario.label}`, async ({ page }) => {
    await page.clock.install({ time: new Date('2026-09-22T19:00:00Z') })
    await seedSession(page, 'user')
    await installOperatorApiMock(page, 'user')
    await page.route('**/api/v1/subscriptions/active**', (route) => route.fulfill({
      contentType: 'application/json',
      body: JSON.stringify({ code: 0, data: [{
        id: 1, user_id: 2, group_id: 1, status: 'active',
        starts_at: '2026-09-01T00:00:00Z', expires_at: scenario.expires,
        daily_usage_usd: 0, weekly_usage_usd: 0, monthly_usage_usd: 0,
        group: { id: 1, name: 'Calendar plan' },
      }] }),
    }))
    await page.goto('/dashboard')
    await page.getByTitle('View subscription details', { exact: true }).click()
    await expect(page.getByText('Calendar plan', { exact: true })).toBeVisible()
    await expect(page.getByText(scenario.label, { exact: true })).toBeVisible()
    await page.getByTitle('View subscription details', { exact: true }).click()
    await expect(page.getByText('Calendar plan', { exact: true })).not.toBeVisible()
  })
}
