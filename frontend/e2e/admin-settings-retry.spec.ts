import { expect, test } from '@playwright/test'
import { installOperatorApiMock, seedSession } from './fixtures/operatorApi'
import { getOperatorFixtureData } from './fixtures/operatorData'

for (const width of [390, 1280]) {
  test(`failed initial admin settings retry during navigation at ${width}px`, async ({ page }) => {
    await page.setViewportSize({ width, height: 900 })
    await seedSession(page)
    await installOperatorApiMock(page)
    await page.addInitScript(() => localStorage.setItem('ops_monitoring_enabled_cached', 'false'))
    let attempts = 0
    await page.route(/\/api\/v1\/admin\/settings(?:\?.*)?$/, route => {
      return route.fulfill({ json: { code: 0, data: {
        ...getOperatorFixtureData('/api/v1/admin/settings') as object,
        ops_monitoring_enabled: true
      } } })
    })
    await page.route(/\/api\/v1\/admin\/payment\/config(?:\?.*)?$/, route => {
      attempts++
      if (attempts === 1) return route.fulfill({ status: 503, json: { code: 503, message: 'Payment configuration unavailable' } })
      return route.fulfill({ json: { code: 0, data: getOperatorFixtureData('/api/v1/admin/payment/config') } })
    })
    await page.goto('/admin/accounts')
    await expect(page.getByRole('heading', { name: 'Accounts', exact: true })).toBeVisible()
    await expect.poll(() => attempts).toBeGreaterThanOrEqual(1)
    await page.getByRole('link', { name: 'Proxies', exact: true }).first().click()
    await expect(page).toHaveURL(/\/admin\/proxies$/)
    await expect(page.getByRole('heading', { name: 'Proxy Management', exact: true })).toBeVisible()
    await expect.poll(() => attempts).toBe(2)
    await expect.poll(() => page.evaluate(() => localStorage.getItem('ops_monitoring_enabled_cached'))).toBe('true')
    if (width < 768) await page.getByRole('button', { name: 'Toggle menu', exact: true }).click()
    await page.getByRole('link', { name: 'Accounts', exact: true }).first().click()
    await expect(page.getByRole('heading', { name: 'Accounts', exact: true })).toBeVisible()
    expect(attempts).toBe(2)
  })
}
