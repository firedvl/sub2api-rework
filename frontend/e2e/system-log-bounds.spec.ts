import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'

for (const width of [1440, 390]) {
  test(`database access-log persistence is opt-in and survives reload at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    let saved = {
      level: 'info', persist_access_logs: false, enable_sampling: false,
      sampling_initial: 100, sampling_thereafter: 100, caller: true,
      stacktrace_level: 'error', retention_days: 14, request_retention_days: 90,
    }
    let updates = 0
    await page.route('**/api/v1/admin/ops/runtime/logging**', async route => {
      if (route.request().method() === 'PUT') {
        saved = route.request().postDataJSON()
        updates++
      }
      await route.fulfill({
        contentType: 'application/json', body: JSON.stringify({ code: 0, data: saved }),
      })
    })
    await page.goto('/admin/ops')
    const checkbox = page.getByRole('checkbox', { name: 'Store access logs in database', exact: true })
    await expect(checkbox).not.toBeChecked()
    await checkbox.check()
    await page.getByRole('button', { name: 'Save and apply', exact: true }).click()
    await expect.poll(() => updates).toBe(1)
    expect(saved.persist_access_logs).toBe(true)
    expect(saved.retention_days).toBe(14)
    expect(saved.request_retention_days).toBe(90)
    await page.reload()
    await expect(checkbox).toBeChecked()
    await checkbox.uncheck()
    await page.getByRole('button', { name: 'Save and apply', exact: true }).click()
    await expect.poll(() => updates).toBe(2)
    expect(saved.persist_access_logs).toBe(false)
    await expect(page.locator('body')).toContainText('Warning, error, and audit logs are always stored.')
  })
}
